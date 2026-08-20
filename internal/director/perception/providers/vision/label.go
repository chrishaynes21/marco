package vision

import (
	"context"
	"image"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"sort"
	"strings"
	"unicode"
)

// Reading the words inside a detected control.
//
// A detector answers "there is a control here" and nothing else. What the control SAYS is
// the one thing that makes it addressable — "click Settings" needs a control named
// Settings — and a box with no name is a shape nobody can ask for.
//
// # Why the reading is scoped to the box
//
// This is not an optimisation. It is the only arrangement that works, and the alternatives
// were measured against a live Rocket League pause menu rather than reasoned about:
//
//	whole frame, native size      1 string of the ~12 a person reads
//	whole frame, 2× and 3×        2 strings
//	tiled 4×3 and 6×4, 2×       151 and 127 strings, essentially all garbage
//	scoped to detected boxes      4 of 4 button labels, exactly
//
// The whole-frame reads fail because a global binarisation threshold is dominated by a
// bright 3D scene, and the translucent panel text falls on the wrong side of it. Tiling
// fixes the threshold and introduces a worse problem: an OCR engine pointed at arena
// texture HALLUCINATES. It returned "Sty 4;", "Feasts)", "itirne" — confident nonsense from
// regions containing no text at all.
//
// So the structure is what makes the reading trustworthy. Vision says where a control is;
// only there is text worth believing. That is also why a label read this way is attached to
// the detection rather than emitted beside it — see the ordering note in
// Provider.observations. A name is a property OF a control, not a second thing at the same
// place, and this package has already paid for that lesson once with grid positions.
//
// # What is still refused, and by which of two filters
//
// Even inside a real control an engine returns garbage. Two different things reject it, and
// they are not interchangeable:
//
//   - SHAPE. Symbol soup — "{= =", "»)  (ee i", "~~ A" — contains characters no label has.
//     plausibleLabel refuses on the first one.
//   - CONFIDENCE. The same live frame read a stylised player-name plate as "Qovisivre ys",
//     which is eleven letters and a space and looks exactly like a product name nobody here
//     has heard of. No shape rule will ever reject that. What rejects it is the engine
//     admitting it was unsure, which is why MinConfidence is applied to every span before
//     the shape filter sees the joined text.
//
// An unnamed control is a fair description of one whose name could not be read. A control
// named "Qovisivre ys" is a lie, and an addressable one.

// readLabel is one control's name, as read.
type readLabel struct {
	text       string
	confidence float64
}

// labelsOver reads the words inside each accepted NAMEABLE detection.
//
// Keyed by the accepted index so the caller can attach a name to the observation it is
// already building. Boxes that read as nothing are simply absent: an unnamed control is the
// honest description of one whose name could not be read.
//
// Nothing here can fail the pass. A reader that errors leaves the detections unnamed, which
// is what they were before a reader existed.
//
// # Why nameability decides what is READ, not only what is kept
//
// This used to read every structural box and let the privacy classifier withhold the ones it
// disapproved of. That is the same answer at several times the cost and one avoidable risk: a
// round trip to an OCR engine is 200–700ms, so reading a panel, a progress bar and forty icons
// to discard all of it spends the budget that the buttons needed — and it puts the text of an
// icon (a document title, a contact) into this process's memory for no purpose at all.
//
// So the budget is spent where a reading can survive. The allowlist is the canonical one,
// `directorapi.ElementRole.NameablePlaintext`, which is also what the classifier will apply
// downstream; the two cannot drift, because there is only one of them.
func (p *Provider) labelsOver(ctx context.Context, accepted []placed,
	source image.Image, crop image.Rectangle) map[int]readLabel {

	if p.Reader == nil || len(accepted) == 0 || source == nil {
		return nil
	}
	t := p.Labels
	if t.Upscale <= 0 {
		t = DefaultLabelThresholds()
	}

	// A crop shifts image-local coordinates: a detection's box is relative to the FRAME,
	// and source is relative to the crop. Getting this wrong reads the wrong rectangle
	// and names a control after its neighbour.
	origin := image.Point{}
	if !crop.Empty() {
		origin = crop.Min
	}

	// Largest first. A ceiling has to drop something, and the biggest controls are both
	// the likeliest to carry a name and the likeliest to be what a person meant. Dropping
	// in detector order would instead be arbitrary.
	order := make([]int, 0, len(accepted))
	for i, a := range accepted {
		if !a.class.Nameable() {
			// Structural and unsayable. Not read at all — see the note above.
			p.counters.LabelsUnsayable++
			continue
		}
		if a.image.Dx() < t.MinWidth || a.image.Dy() < t.MinHeight {
			p.counters.LabelsSkipped++
			continue
		}
		if contested(i, accepted) {
			// Two nameable controls overlap and neither contains the other, so a word
			// found in the overlap belongs to both readings equally. Reported as
			// ambiguous rather than resolved: assigning it to whichever came first in
			// detector order, or to the nearer centre, is a coin toss that produces a
			// confidently misnamed control.
			p.counters.LabelsAmbiguous++
			continue
		}
		order = append(order, i)
	}
	sort.SliceStable(order, func(x, y int) bool {
		bx, by := accepted[order[x]].image, accepted[order[y]].image
		return bx.Dx()*bx.Dy() > by.Dx()*by.Dy()
	})
	if t.MaxLabels > 0 && len(order) > t.MaxLabels {
		p.counters.LabelsSkipped += len(order) - t.MaxLabels
		order = order[:t.MaxLabels]
	}

	out := make(map[int]readLabel, len(order))
	for _, i := range order {
		a := accepted[i]
		box := a.image.Sub(origin).Intersect(source.Bounds())
		if box.Empty() {
			continue
		}
		img := capture.Scale(capture.Crop(source, box), t.Upscale)
		spans, err := p.Reader.ReadLabel(ctx, img)
		if err != nil {
			// One unreadable control does not spoil the pass, and the reason belongs in
			// the counters rather than as a failure of the whole observation.
			p.counters.LabelsUnreadable++
			continue
		}
		inside, outside := spansWithin(spans, img.Bounds())
		if outside > 0 {
			// The engine placed a reading outside the rectangle it was handed. Whatever
			// that text belongs to, it is not provably this control.
			p.counters.LabelsAmbiguous += outside
		}
		text, confidence, ok := labelFrom(inside, t)
		if !ok {
			p.counters.LabelsUnreadable++
			continue
		}
		p.counters.LabelsRead++
		out[i] = readLabel{text: text, confidence: confidence}
	}
	return out
}

// contested reports whether another NAMEABLE accepted detection overlaps this one without
// either containing the other.
//
// # Why containment is not a contest
//
// Nesting is the ordinary case and is not ambiguous. A `button` inside a `menu`, or inside a
// `panel`, has a name that belongs to the button — the innermost thing a word sits in is the
// thing the word names, which is how interfaces are drawn and how people read them. Treating
// nesting as ambiguity would refuse every menu item in every menu.
//
// What IS ambiguous is two boxes that merely overlap: a word in the intersection is inside
// both and inside neither exclusively, and nothing about the geometry says which one it
// names. Unsayable neighbours are not considered at all, because their text is not going to
// be kept whatever it says, so they cannot compete for it.
func contested(i int, accepted []placed) bool {
	a := accepted[i].image
	for j, b := range accepted {
		if j == i || !b.class.Nameable() {
			continue
		}
		o := a.Intersect(b.image)
		if o.Empty() {
			continue
		}
		if a.In(b.image) || b.image.In(a) {
			continue
		}
		// Touching is not competing. The four buttons of the live pause menu overlap by
		// three pixels each, because a detector's boxes are not drawn by the interface —
		// and refusing to name all four because of a 3px band would be the strict reading
		// of "ambiguous" producing exactly the wrong answer. What makes an overlap a
		// contest is that a WORD could sit in it, which is a question about how much of
		// the smaller control the other one covers.
		if pixelArea(o) >= AmbiguousOverlap*float64(smallerArea(a, b.image)) {
			return true
		}
	}
	return false
}

// AmbiguousOverlap is how much of the smaller of two nameable controls the other must cover
// before a reading cannot be attributed to either.
//
// A quarter. Calibrated against the two cases that must come out differently: stacked menu
// buttons share about 6% of their area through detector jitter and must both be named, while
// two boxes genuinely competing for the same text share most of one of them. Nothing in
// between has been measured, and a quarter is comfortably clear of the case that has.
const AmbiguousOverlap = 0.25

func pixelArea(r image.Rectangle) float64 { return float64(r.Dx()) * float64(r.Dy()) }

func smallerArea(a, b image.Rectangle) int {
	x, y := a.Dx()*a.Dy(), b.Dx()*b.Dy()
	if y < x {
		return y
	}
	return x
}

// within splits spans into those the engine placed inside the rectangle it was reading and
// those it did not, so a reading that wandered out of its region can be counted rather than
// silently attached.
//
// A span with no bounds at all is kept: an engine that reports text without geometry has said
// nothing about where it was, and the crop is already the answer to that question.
func spansWithin(spans []TextSpan, bounds image.Rectangle) (inside []TextSpan, outside int) {
	inside = make([]TextSpan, 0, len(spans))
	for _, s := range spans {
		if !s.Bounds.Empty() && !s.Bounds.In(bounds) {
			outside++
			continue
		}
		inside = append(inside, s)
	}
	return inside, outside
}

// screenTextOver reads the words inside accepted TEXT regions.
//
// # Why a text region is read at all, and why it can never become a control
//
// A detector that finds a heading has found something genuinely useful on a surface with no
// accessibility: the word CONTROLLER SETTINGS above four boxes is what makes the four boxes
// mean something. But a heading is not a control, and the rule this package exists to enforce
// is that reading a word never creates a thing — so this returns TEXT, at the text region's
// own coordinates, and the caller emits it as text. Nothing here produces an element, gives
// anything a role, or names anything.
//
// The reading is otherwise identical to a control's: same reader, same thresholds, same
// shape and confidence filters, same budget. What differs downstream is that a text region's
// words are never released in the clear — they are classified into generic interface concepts
// and discarded — because unlike a button's name, what a text region says is as likely to be
// about the person as about the interface.
func (p *Provider) screenTextOver(ctx context.Context, candidates []placed,
	source image.Image, crop image.Rectangle) map[int]readLabel {

	if p.Reader == nil || len(candidates) == 0 || source == nil {
		return nil
	}
	t := p.Labels
	if t.Upscale <= 0 {
		t = DefaultLabelThresholds()
	}
	origin := image.Point{}
	if !crop.Empty() {
		origin = crop.Min
	}

	order := make([]int, 0, len(candidates))
	for i, c := range candidates {
		if c.image.Dx() < t.MinWidth || c.image.Dy() < t.MinHeight {
			p.counters.LabelsSkipped++
			continue
		}
		order = append(order, i)
	}
	sort.SliceStable(order, func(x, y int) bool {
		bx, by := candidates[order[x]].image, candidates[order[y]].image
		return bx.Dx()*bx.Dy() > by.Dx()*by.Dy()
	})
	if t.MaxScreenTexts > 0 && len(order) > t.MaxScreenTexts {
		p.counters.LabelsSkipped += len(order) - t.MaxScreenTexts
		order = order[:t.MaxScreenTexts]
	}

	out := make(map[int]readLabel, len(order))
	for _, i := range order {
		box := candidates[i].image.Sub(origin).Intersect(source.Bounds())
		if box.Empty() {
			continue
		}
		img := capture.Scale(capture.Crop(source, box), t.Upscale)
		spans, err := p.Reader.ReadLabel(ctx, img)
		if err != nil {
			p.counters.LabelsUnreadable++
			continue
		}
		inside, _ := spansWithin(spans, img.Bounds())
		text, confidence, ok := labelFrom(inside, t)
		if !ok {
			p.counters.LabelsUnreadable++
			continue
		}
		p.counters.ScreenTextsRead++
		out[i] = readLabel{text: text, confidence: confidence}
	}
	return out
}

// TextSpan is one piece of text an engine read, with how sure it was.
type TextSpan struct {
	Text       string
	Confidence float64
	// Bounds is where it sat in the crop, in crop-local pixels. Used only for reading
	// order; nothing downstream places anything from it.
	Bounds image.Rectangle
}

// LabelReader reads the text inside an image.
//
// Deliberately not ocr.Engine, though an adapter over one is three lines at the
// composition root. The vision provider must not depend on the OCR provider: they are
// siblings, they are separately optional, and a build in which one exists and the other
// does not is the ordinary case.
type LabelReader interface {
	ReadLabel(ctx context.Context, img image.Image) ([]TextSpan, error)
}

// LabelThresholds decide when a reading is worth keeping.
type LabelThresholds struct {
	// MinConfidence is the floor for one span. Below it the span is dropped.
	MinConfidence float64
	// Upscale enlarges the crop before reading. A button's glyphs are around twenty
	// pixels tall at native size and tesseract wants roughly double that.
	Upscale int
	// MinLetters is the fewest letters or digits a label must have. One stray mark is
	// not a name.
	MinLetters int
	// MaxLength bounds a label. Anything longer is a paragraph that happened to fall
	// inside a box, not the name of a control.
	MaxLength int
	// MinAlphaFraction is the share of non-space characters that must be letters or
	// digits. A name is mostly words; a string that is nearly all punctuation is not one,
	// however permitted each character is on its own.
	MinAlphaFraction float64
	// MaxLabels bounds how many controls one pass will read.
	//
	// Each reading is a separate round trip to an OCR engine — encode, recognise, decode —
	// and they are serial because the bridge is. Measured live: 39 boxes cost 9.0
	// seconds, about 230ms each. Unbounded, a busy window would make one observation
	// cycle take longer than anything waiting on it is prepared to wait.
	MaxLabels int
	// MaxScreenTexts bounds how many TEXT regions one pass will read.
	//
	// A separate, smaller budget rather than a share of MaxLabels, because the two compete
	// and controls should win. A screen's heading is one region and worth reading; a
	// document full of paragraphs is fifty and worth none of them, and the difference is
	// visible only as a count.
	MaxScreenTexts int
	// MinWidth and MinHeight skip boxes too small to hold a readable word. A 12×12 icon
	// costs the same round trip as a button and cannot contain a name.
	MinWidth  int
	MinHeight int
}

// DefaultLabelThresholds are the starting points, calibrated against one live frame and
// therefore provisional — see docs/director-vision.md, Known gaps.
func DefaultLabelThresholds() LabelThresholds {
	return LabelThresholds{
		MinConfidence:    0.45,
		Upscale:          3,
		MinLetters:       2,
		MaxLength:        64,
		MinAlphaFraction: 0.6,
		MaxLabels:        24,
		MaxScreenTexts:   4,
		MinWidth:         24,
		MinHeight:        10,
	}
}

// labelFrom assembles spans into one label, or reports that there is none.
//
// Returns the text, the weakest span's confidence, and whether it is worth keeping. The
// WEAKEST rather than the mean: a label is read correctly only if every part of it was, and
// averaging lets one confident word carry two illegible ones.
func labelFrom(spans []TextSpan, t LabelThresholds) (string, float64, bool) {
	kept := make([]TextSpan, 0, len(spans))
	for _, s := range spans {
		if strings.TrimSpace(s.Text) == "" || s.Confidence < t.MinConfidence {
			continue
		}
		kept = append(kept, s)
	}
	if len(kept) == 0 {
		return "", 0, false
	}

	// Reading order: top to bottom, then left to right. An engine's own order is its
	// business, and a label assembled out of order is a different label.
	sort.SliceStable(kept, func(i, j int) bool {
		a, b := kept[i].Bounds, kept[j].Bounds
		if a.Empty() || b.Empty() {
			return false
		}
		// Same line when the vertical overlap is most of the shorter box.
		if overlapsVertically(a, b) {
			return a.Min.X < b.Min.X
		}
		return a.Min.Y < b.Min.Y
	})

	weakest := kept[0].Confidence
	words := make([]string, 0, len(kept))
	for _, s := range kept {
		words = append(words, strings.TrimSpace(s.Text))
		if s.Confidence < weakest {
			weakest = s.Confidence
		}
	}
	text := normaliseSpace(strings.Join(words, " "))
	if !plausibleLabel(text, t) {
		return "", 0, false
	}
	return text, weakest, true
}

// # Why there is no edge-trimming here
//
// A control's border does become a character: reading the live pause menu's first button
// through tesseract's single-line mode returned "| RESUME GAME ," — panel edge one side,
// highlight the other. Trimming the ends looks like the obvious fix and was written, and
// then its own test refused it: stripping edges rescues "»)  (ee i" as "ee i" and "\\ Sea"
// as "Sea", which is precisely the garbage the shape filter exists to stop.
//
// The fix was unnecessary anyway. An engine reports WORDS, each with its own confidence,
// and this package joins the ones it believes — so in the real path "|" and "," arrive as
// separate low-confidence spans and MinConfidence drops them before the joining. The
// combined string that looked like a problem was an artefact of asking tesseract for one
// line of stdout, which is not how the Director asks.
//
// Left as a note rather than deleted quietly, because the next person to read "|" in a
// label will reach for the same fix.

// overlapsVertically reports whether two boxes share most of the shorter one's height.
func overlapsVertically(a, b image.Rectangle) bool {
	shorter := a.Dy()
	if b.Dy() < shorter {
		shorter = b.Dy()
	}
	if shorter <= 0 {
		return false
	}
	top := a.Min.Y
	if b.Min.Y > top {
		top = b.Min.Y
	}
	bottom := a.Max.Y
	if b.Max.Y < bottom {
		bottom = b.Max.Y
	}
	return float64(bottom-top) >= 0.5*float64(shorter)
}

// labelPunctuation is what a real control's name may contain besides letters and digits:
// "CHANGE MODE/MATCH", "Save & Close", "Don't Save", "50%".
const labelPunctuation = "/&'’-_.,:%+()#!? "

// plausibleLabel reports whether text has the SHAPE of a control's name.
//
// # What this catches, and what it cannot
//
// It catches symbol soup, which is what an engine returns when pointed at texture. From the
// live tiling run: "{= =", "»)  (ee i", "~~ A", "Sty 4;". Every one contains a character no
// label has, and that is the test — one disallowed character is enough, because a real
// label made of ordinary words does not acquire a "»".
//
// It CANNOT catch letter-shaped nonsense. The same live frame read a stylised player-name
// plate as "Qovisivre ys" — eleven letters and a space, indistinguishable by shape from a
// product name this build has never heard of. Nothing here will ever reject that, and
// pretending otherwise by bolting on a dictionary would refuse every application whose
// vocabulary was not anticipated.
//
// What rejects it is CONFIDENCE. An engine that is guessing says so, and labelFrom drops
// spans below LabelThresholds.MinConfidence before this is ever called. The two filters
// answer different questions — "is this shaped like a name" and "did the engine believe
// it" — and a label needs both.
func plausibleLabel(s string, t LabelThresholds) bool {
	if s == "" || len([]rune(s)) > t.MaxLength {
		return false
	}
	letters, meaningful := 0, 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		meaningful++
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			letters++
		case strings.ContainsRune(labelPunctuation, r):
			// Allowed, but not evidence of a name: a string of pure punctuation is not
			// one, so it counts toward neither side.
		default:
			// One character no label has is enough. This is the whole defence against
			// an engine reading arena texture.
			return false
		}
	}
	if meaningful == 0 || letters < t.MinLetters {
		return false
	}
	return float64(letters)/float64(meaningful) >= t.MinAlphaFraction
}

// normaliseSpace collapses runs of whitespace.
func normaliseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
