package main

// OCR beside the fast loop, and never inside it.
//
// # Why a second slow source rather than a faster OCR
//
// [[ADR-194-a-screen-is-also-what-it-says]] proved offline that what a screen says can keep two
// visually identical menus apart: Rocket League's Training page and its Free Play submenu agree
// at 0.559 by arrangement against a threshold of 0.55, and the words refuse the merge. It also
// left one thing undone, and said so — `fastobserve.Look.Text` is look-local by construction,
// and OCR costs hundreds of milliseconds against an observation cadence of a hundred. No live
// source can ever fill it.
//
// So this is the same shape as [[ADR-192-a-second-source-in-the-live-loop]]: a source on its own
// clock, offering non-blocking with a queue one frame deep, returning results stamped with the
// context they were computed in. Nothing about lateness is re-solved here. The screen model
// already carries a refusal across the looks between readings, and `intakeReadings` already
// applies the same three rules the delayed detections go through.
//
// # What the measurement says about the latency
//
// Experiment-043 replays the recording with every reading delivered N looks after its own frame:
//
//	late by 1 look     APART     283 of 300 admitted
//	late by 3 looks    APART     260 admitted, 38 refused as stale
//	late by 5 looks    APART     245 admitted, 54 stale
//	late by 8 looks    APART     223 admitted, 75 stale
//	late by 12 looks   MERGED    200 admitted, 95 stale
//
// OCR at 300-600ms on a 1920x1080 frame is three to six looks behind, so the working range
// covers the real cost with margin — and past it the failure is the one it was before this
// existed, which is the right way for it to fail.

import (
	"context"
	"image"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/platform/ocrclient"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// readingFrame is one frame handed to OCR, with the observation it came from.
//
// `look` is the frame's identity and the only thing that ties a reading back to the pixels it
// describes. The loop numbers its looks from zero; the authorisation boundary takes a NEGATIVE
// number of its own from `nextBoundaryRead`, so a reading of one boundary capture can never be
// mistaken for a reading of the loop's last frame or of the other boundary in the same press.
type readingFrame struct {
	img        image.Image
	look       int
	epoch      uint64
	generation uint64
}

// readingEyes is OCR running on its own clock.
type readingEyes struct {
	// eng is the engine. An interface rather than the concrete bridge client because the
	// authorisation boundary now DEPENDS on when a reading arrives, and a test that
	// cannot decide when one arrives cannot describe that dependence.
	eng  ocr.Engine
	work chan readingFrame
	done chan observe.ScreenReading
	// asked and answered count frames offered and readings returned; dropped counts
	// frames skipped because a reading was still running, which is the ordinary case and
	// the number that says how far behind OCR is.
	asked, answered, dropped, words atomic.Int64
	// atBoundary counts readings asked for AT an authorisation boundary, and tooLate how
	// many of those did not arrive inside their budget. A run with tooLate above zero is
	// a run that refused presses for want of words, which is the correct refusal and
	// still the number somebody would want to see.
	atBoundary, tooLate atomic.Int64
	// readMS is the total milliseconds spent reading, and the count of readings is
	// `answered`, so the mean is one division.
	//
	// Measured rather than cited: Experiment-043 put OCR at 300-600ms on a 1920x1080
	// frame and derived the working range from it, and that was a replay on this
	// machine's CPU with no game running. A live session competing with a game for the
	// same machine is the number that decides whether the window holds.
	readMS, slowest atomic.Int64
	// why is why there is no reader, empty when there is one.
	why string
}

// newReadingEyes starts OCR, or returns one that explains why it cannot run.
func newReadingEyes(ctx context.Context) *readingEyes {
	e := &readingEyes{
		work: make(chan readingFrame, 1),
		done: make(chan observe.ScreenReading, 4),
	}
	bridge := defaultOCRBridge()
	if bridge == "" {
		e.why = "no OCR plugin found — build plugins/ocr, or set $DIRECTOR_OCR to ocr.exe"
		return e
	}
	if _, err := os.Stat(bridge); err != nil {
		e.why = "the OCR plugin is not at " + bridge
		return e
	}
	h := bridgehost.New(bridge)
	if t := defaultTesseract(); t != "" {
		h = h.WithEnv("MARCO_TESSERACT=" + t)
	}
	e.eng = ocrclient.New(h)
	go e.run(ctx)
	return e
}

// working reports whether there is a reader at all.
func (e *readingEyes) working() bool { return e != nil && e.eng != nil }

// offer hands OCR a frame, or drops it if a reading is still running.
//
// Non-blocking, and dropping is the ordinary case rather than an error: the loop produces ten
// frames a second and OCR answers about twice. What must never happen is the loop waiting, so
// the queue is one deep and the newest frame wins.
func (e *readingEyes) offer(f readingFrame) {
	if !e.working() {
		return
	}
	e.asked.Add(1)
	select {
	case e.work <- f:
	default:
		e.dropped.Add(1)
	}
}

// ready drains whatever OCR has finished, without waiting for it.
func (e *readingEyes) ready() []observe.ScreenReading {
	if !e.working() {
		return nil
	}
	var out []observe.ScreenReading
	for {
		select {
		case r := <-e.done:
			out = append(out, r)
		default:
			return out
		}
	}
}

// run is the worker. One frame at a time, for as long as the run lasts.
func (e *readingEyes) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case f := <-e.work:
			r, ok := e.recognise(ctx, f)
			if !ok {
				continue
			}
			select {
			case e.done <- r:
			default: // nobody drained; the next look will have a fresher one anyway
			}
		}
	}
}

// readNow reads ONE frame and waits for THAT frame's words, up to a budget.
//
// # Why the queue is not the way to ask
//
// `offer` and `ready` are a pipeline for the loop: the queue is one deep, the newest frame
// wins, and `ready` returns whatever has FINISHED — which is the reading of some earlier
// frame, or nothing. That is exactly right for a loop that looks ten times a second and will
// look again, and it is wrong for a question whose answer decides whether a click is sent.
//
// The authorisation boundary offered its capture and drained in the same breath, so the words
// it put on its look were the words of the frame BEFORE the one it had just taken. With the
// observation epoch unchanged — which it is, for as long as the stabiliser is damping the
// change — those stale words are admissible, agree with the standing screen, and CONFIRM the
// screen the plan was made on. A panel swapped for one of the same shape then had nothing left
// to refuse it. See freshFrom.
//
// So this is a different request with a different shape: this frame, this answer, bounded, and
// cancellable. It bypasses the queue rather than competing for it — the bridge serialises its
// own calls, so at worst this waits behind the one reading already in flight, and the loop's
// pipeline is left exactly as it was. The caller is a goroutine that is already waiting for a
// person, not the observation loop.
//
// `false` means no reading of THIS frame could be had inside the budget, which the boundary
// treats as missing evidence rather than as agreement.
func (e *readingEyes) readNow(ctx context.Context, f readingFrame,
	within time.Duration) (observe.ScreenReading, bool) {

	if !e.working() {
		return observe.ScreenReading{}, false
	}
	e.atBoundary.Add(1)
	ctx, cancel := context.WithTimeout(ctx, within)
	defer cancel()
	// BUFFERED, so the reading goroutine is never left blocked on a send nobody is
	// waiting for any more. A bridge call cannot be interrupted once it has been
	// written, so giving up here means giving up on the ANSWER and not on the work.
	out := make(chan observe.ScreenReading, 1)
	go func() {
		if r, ok := e.recognise(ctx, f); ok {
			out <- r
		}
	}()
	select {
	case r := <-out:
		return r, true
	case <-ctx.Done():
		e.tooLate.Add(1)
		return observe.ScreenReading{}, false
	}
}

// recognise reads one frame and returns it as a reading, or false if it could not be read.
//
// The one place a reading is made, for both the loop's pipeline and the boundary's bounded
// request. Shared rather than written twice because the SCALE is part of the evidence: the
// vocabulary every screen was recognised by was read at `readScale`, and a boundary reading
// taken at full size would come back with measurably different words — 181 distinct against
// 217 over the same frames — and disagree with the screen it is a reading of.
func (e *readingEyes) recognise(ctx context.Context, f readingFrame) (
	observe.ScreenReading, bool) {

	// HALF SIZE, BECAUSE THE DEADLINE IS REAL. See readScale.
	img := shrink(f.img, readScale)
	began := time.Now()
	got, err := e.eng.Recognize(ctx, ocr.ImageInput{Image: img})
	// TIMED WHETHER OR NOT IT WORKED, because a reading that takes two seconds and
	// fails is the same problem as one that takes two seconds and succeeds.
	ms := time.Since(began).Milliseconds()
	e.readMS.Add(ms)
	for {
		was := e.slowest.Load()
		if ms <= was || e.slowest.CompareAndSwap(was, ms) {
			break
		}
	}
	if err != nil {
		// NOT A SILENT SCREEN. A reading that failed is offered as nothing at all,
		// so the observer counts it as a source that could not answer rather than
		// as a screen with no words on it — see ScreenText and ReadingsSilent.
		return observe.ScreenReading{}, false
	}
	e.answered.Add(1)
	// THE FRAME THAT WAS ACTUALLY READ, which is the shrunken one, so every box comes
	// back in its coordinates and not the window's.
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	var phrases []string
	var placed []observe.ReadWord
	for _, g := range got {
		if g.Text == "" || g.Bounds.Empty() {
			continue
		}
		phrases = append(phrases, g.Text)
		if w <= 0 || h <= 0 {
			continue
		}
		// EACH WORD SEPARATELY, because a phrase's rectangle is the span of several
		// words and a span is not inside the row one of them sits in. The box is
		// shared by the words it holds: the engine reports a line, and splitting its
		// rectangle on character counts would invent positions nothing measured.
		for _, word := range strings.Fields(g.Text) {
			placed = append(placed, observe.ReadWord{
				Text: word,
				Where: observe.Region{
					X:      float64(g.Bounds.Min.X-b.Min.X) / w,
					Y:      float64(g.Bounds.Min.Y-b.Min.Y) / h,
					Width:  float64(g.Bounds.Dx()) / w,
					Height: float64(g.Bounds.Dy()) / h,
				},
			})
		}
	}
	r := observe.ScreenReading{
		Text:     observe.TextOf(directorapi.SourceOCR, phrases),
		Words:    placed,
		FromLook: f.look,
		// THE CONTEXT THE PIXELS WERE READ IN, which is the whole of what makes a
		// late reading safe. The observer refuses one whose window or observation
		// generation has moved.
		Generation: f.generation,
		Epoch:      f.epoch,
	}
	e.words.Add(int64(len(r.Text.Words)))
	return r, true
}

// readScale is how much the frame is reduced before it is read.
//
// # Why this is not an optimisation
//
// [[Experiment-043-a-reading-that-arrives-late]] measured the window: the two menus stay
// apart while a reading arrives within EIGHT looks of its own frame, and merge again past
// twelve. That was simulated delay on a recording. Measured for real, on the same frames
// through this plugin and this tesseract:
//
//	scale          per reading   looks behind   training/free   menu against itself
//	1/1 1920x1080        920ms       10  OUT    APART           one screen
//	1/2  960x540         516ms        6  in     APART           one screen
//	1/3  640x360         436ms        5  in     MERGED          one screen
//
// At full resolution the reading is TEN looks behind and the window is eight, so a live
// run would have had almost every reading refused as stale and the two menus would have
// merged exactly as they did before any of this existed. At a third the reading is quick
// and tesseract has started inventing: the Training page comes back as
// `aerial custom fire free goalie play players practice say sily striker` and the submenu
// as `create fire free online play say sily`, which share enough junk to agree -- kept
// 0.36 against a threshold of 0.35, and the discrimination is gone.
//
// Half is the one that is both in time and still reading the labels, with margin on both
// rules: kept 0.21 against 0.35, novel 0.70 against 0.55. It also reads MORE distinct
// words than full size over the same frames (217 against 181), because the gradients a
// game menu is painted on produce less spurious text once they are averaged down.
//
// Local to this source on purpose. `director ocr` reads a frame for a person to look at and
// has no deadline; this one has eight looks.
const readScale = 2

// shrink reduces an image by an integer factor, averaging each block.
//
// A box average rather than nearest-neighbour, and the difference is not cosmetic: dropping
// pixels deletes whole strokes from thin glyphs, while averaging leaves them as grey, which
// is what tesseract's own thresholding wants. Hand-written because the engine carries no
// image dependency and because this is the exact arithmetic the measurement above was taken
// with -- a resampler swapped in later would make those numbers describe something else.
func shrink(src image.Image, n int) image.Image {
	if n <= 1 || src == nil {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx()/n, b.Dy()/n
	if w <= 0 || h <= 0 {
		// SMALLER THAN THE FACTOR. Reading the original is right: it is cheap
		// precisely because there is almost nothing of it.
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	k := uint32(n * n)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, bl, a uint32
			for dy := 0; dy < n; dy++ {
				for dx := 0; dx < n; dx++ {
					cr, cg, cb, ca := src.At(
						b.Min.X+x*n+dx, b.Min.Y+y*n+dy).RGBA()
					r, g, bl, a = r+cr, g+cg, bl+cb, a+ca
				}
			}
			o := y*dst.Stride + x*4
			dst.Pix[o+0] = uint8(r / k >> 8)
			dst.Pix[o+1] = uint8(g / k >> 8)
			dst.Pix[o+2] = uint8(bl / k >> 8)
			dst.Pix[o+3] = uint8(a / k >> 8)
		}
	}
	return dst
}

// describe is what the report says about the reader.
func (e *readingEyes) describe() string {
	if e == nil {
		return "not running"
	}
	if e.why != "" {
		return "not running: " + e.why
	}
	if !e.working() {
		return "not running"
	}
	out := "offered " + itoa(e.asked.Load()) + ", read " + itoa(e.answered.Load()) +
		", dropped while busy " + itoa(e.dropped.Load()) + ", " +
		itoa(e.words.Load()) + " word(s)"
	// AND WHAT IT COST, which is what decides whether the delayed-arrival window holds.
	// A reading that lands more looks late than the observer will admit was taken and
	// thrown away, and the count of admitted readings cannot say which.
	//
	// Measured rather than cited. Experiment-043 put OCR at 300-600ms on a 1920x1080
	// frame and derived the working range from it — but that was a replay on an idle
	// machine, and a live session shares a CPU with the game it is watching.
	if n := e.answered.Load(); n > 0 {
		out += ", mean " + itoa(e.readMS.Load()/n) + "ms, slowest " +
			itoa(e.slowest.Load()) + "ms"
	}
	// AND WHAT THE BOUNDARY ASKED FOR, which is a different request from the loop's and
	// the only one anything waits on. `tooLate` above zero is a press refused for want of
	// the words that would have told it whether the panel under the pointer is still the
	// panel somebody authorised — see readNow.
	if n := e.atBoundary.Load(); n > 0 {
		out += ", at an authorisation boundary " + itoa(n) + " (" +
			itoa(e.tooLate.Load()) + " too late)"
	}
	return out
}
