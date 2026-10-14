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
	"sync/atomic"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/platform/ocrclient"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// readingFrame is one frame handed to OCR, with the observation it came from.
type readingFrame struct {
	img        image.Image
	look       int
	epoch      uint64
	generation uint64
}

// readingEyes is OCR running on its own clock.
type readingEyes struct {
	eng  *ocrclient.Engine
	work chan readingFrame
	done chan observe.ScreenReading
	// asked and answered count frames offered and readings returned; dropped counts
	// frames skipped because a reading was still running, which is the ordinary case and
	// the number that says how far behind OCR is.
	asked, answered, dropped, words atomic.Int64
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
			got, err := e.eng.Recognize(ctx, ocr.ImageInput{Image: f.img})
			if err != nil {
				// NOT A SILENT SCREEN. A reading that failed is offered as
				// nothing at all, so the observer counts it as a source that
				// could not answer rather than as a screen with no words on
				// it — see ScreenText and ReadingsSilent.
				continue
			}
			e.answered.Add(1)
			var phrases []string
			for _, g := range got {
				if g.Text == "" || g.Bounds.Empty() {
					continue
				}
				phrases = append(phrases, g.Text)
			}
			r := observe.ScreenReading{
				Text:     observe.TextOf(directorapi.SourceOCR, phrases),
				FromLook: f.look,
				// THE CONTEXT THE PIXELS WERE READ IN, which is the whole of
				// what makes a late reading safe. The observer refuses one
				// whose window or observation generation has moved.
				Generation: f.generation,
				Epoch:      f.epoch,
			}
			e.words.Add(int64(len(r.Text.Words)))
			select {
			case e.done <- r:
			default: // nobody drained; the next look will have a fresher one anyway
			}
		}
	}
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
	return "offered " + itoa(e.asked.Load()) + ", read " + itoa(e.answered.Load()) +
		", dropped while busy " + itoa(e.dropped.Load()) + ", " +
		itoa(e.words.Load()) + " word(s)"
}
