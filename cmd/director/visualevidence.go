package main

// WHAT A DEMONSTRATION KEEPS ABOUT THE CONTROL SOMEBODY PRESSED.
//
// # Why the looks BEFORE the press, and not after
//
// Because what a guard needs to know is whether the control holds still while it is sitting
// there waiting to be pressed. The looks AFTER a press are the demonstration's other question
// entirely: did anything change, which is the outcome's business and is measured by the outcome.
//
// Folding those in confuses the two, and in the worst possible direction. A press that WORKS
// closes the menu it was in, or opens a panel over it — so the control legitimately stops being
// there, its template stops matching, and a calibration built on that reports the control as
// unusable BECAUSE THE CLICK SUCCEEDED. Every well-behaved button in a navigation menu would
// measure as unstable and nothing would ever be guardable.
//
// So this keeps a short ring of recent looks and calibrates from the ones up to AND INCLUDING
// the press. Pre-action stability, measured on the pre-action screen.
//
// # Why the measuring is not done here
//
// Because calibrating a control is about a billion operations and a look is 480 milliseconds. A
// recorder that measured while the person was still demonstrating would drop looks — on the one
// path where a dropped look is a reading nobody takes and an outcome nobody can check. So the
// LOOKS are kept, cheaply, and the measuring happens at the end when nothing is waiting.
//
// What is kept is grey levels, which is what the matching works on and a quarter of the memory
// of the colour it came from. The ring is bounded and constant; what each press keeps out of it
// is a band around its own control.
//
// # And it is kept at CAPTURE RESOLUTION
//
// The screen previews beside it are thumbnails, because a person reviewing a demonstration looks
// at a picture. This is not looked at: it is measured, and the measurement is exactly what a
// thumbnail destroys. The same control at a third of its resolution matches itself just as well
// — 1.0000 — while the thing next to it rises from 0.5962 to 0.7111, so the margin the guard
// actually has falls by 28% and nothing one-sided can see it happen. See
// [[Experiment-097-a-translucent-menu-is-not-what-defeats-a-local-template]].

import (
	"fmt"
	"image"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/controlevidence"
	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/match"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// evidenceLooks is how many looks of a control are kept to calibrate it.
//
// Five, which is `observe.OutcomeSettleLooks`: the stretch the recorder already watches after a
// press to decide whether anything stayed. Matching it is not tidiness — it is the same claim
// from the other side. A control whose own appearance will not hold still for five looks is one
// whose outcome would be measured over something moving.
//
// These are the five looks ENDING at the press, not beginning there. See the note above.
const evidenceLooks = 5

// recentLook is one look held in the ring, as grey levels and the size they are of.
type recentLook struct {
	grey   *image.Gray
	window image.Point
}

// pressEvidence is the looks kept for one pressed control.
type pressEvidence struct {
	// at is the press, normalised to the window, and box the control's own bounds in
	// window pixels — the proposal that was under the press at the look it arrived on.
	at  observe.PointerAt
	box image.Rectangle
	// window is the captured window's size, which every coordinate here is in.
	//
	// THE WINDOW AND NOT THE CLIENT AREA. What is captured includes the frame, so
	// these are whole-window pixels; saying so is the difference between a consumer
	// converting correctly and a consumer being wrong by the width of a border.
	window image.Point
	// looks is the band around the control, oldest first, out of the ring.
	looks []*image.Gray
}

// watchPressed keeps this look, and takes a band out of the ring for every press on it.
//
// Called on every look the session keeps a picture of, press or not: the ring is what makes the
// looks BEFORE a press available at the moment the press arrives, and those are the ones that
// say whether the control was holding still.
//
// # KNOWN LIMITATION — the press-associated look may already be after the action
//
// A P2, recorded here because this is where somebody would otherwise re-derive it, and NOT to be
// relied on for execution until it is closed.
//
// The loop drains the input subscription and THEN acquires the frame, so the look a press is
// attributed to is captured after that press has already happened. On a fast menu transition
// that frame can hold post-action pixels, or carry no proposal under the click at all — and this
// function folds it in and takes the control's bounds from it.
//
// The loop already names the principle for the SCREEN: "the inputs arrived while the previous
// screen was up, and attributing them to the screen they may have just caused would be this
// layer deciding the causality it exists to record." The same is true of the pixels and of the
// proposals, and this does not yet honour it.
//
// What it costs today is bounded: the other looks in the ring are genuinely pre-action, so one
// late frame moves the worst-case self-match rather than defining it, and a control whose bounds
// came from a transitioned frame is one whose region is wrong rather than one that is silently
// confident. What closes it is selecting frames demonstrably PRECEDING the press and taking the
// target bounds from that same context.
//
// `TestAButtonThatDisappearsBecauseItsClickWorkedIsStillUsable` does not catch this: it supplies
// the intact button on the press-associated look and removes it only afterwards, which is the
// ordering that already works. The test that would catch it removes the control BEFORE the first
// capture after the press.
func (r *visualResult) watchPressed(look int, frame image.Image, presses []observe.PointerAt,
	regions []observe.Region) {

	if frame == nil {
		return
	}
	window := frame.Bounds().Size()
	// THIS LOOK JOINS THE RING FIRST, so a press on it is calibrated with the look
	// it happened on as well as the ones before it.
	r.recent = append(r.recent, recentLook{
		grey: match.Gray(frame, frame.Bounds()), window: window})
	if len(r.recent) > evidenceLooks {
		r.recent = r.recent[len(r.recent)-evidenceLooks:]
	}

	for _, at := range presses {
		if len(r.evidence) >= previewPresses {
			break
		}
		// THE CONTROL'S OWN BOUNDS, from the proposal that was under the press
		// on the look it arrived on. Not a constant around the click: a region
		// in pixels means something different at every capture scale, and what
		// makes this control distinguishable is the size it actually is.
		box, ok := boxUnder(frame.Bounds(), regions, at)
		if !ok {
			// NOTHING WAS PROPOSED UNDER IT. The same press that can never
			// become a step, for the same reason — there is no control
			// there to keep a picture of.
			continue
		}
		if r.evidence == nil {
			r.evidence = map[int]*pressEvidence{}
		}
		if _, had := r.evidence[look]; had {
			continue
		}
		p := &pressEvidence{at: at, box: box, window: window}
		// THE PIXELS A CALIBRATION OF THIS CONTROL WILL LOOK AT, out of every
		// look in the ring that is of the same window.
		//
		// Derived from the TEMPLATE and the positions it will be searched at,
		// not from the control's own box: the search moves a template, so the
		// pixels touched run a band past the region on each side rather than
		// past the control. Keeping the smaller rectangle does not fail — it
		// silently shortens the displacement a rival can be found at, and the
		// gap then reads better than it is.
		keep := controlevidence.Needed(
			controlevidence.RegionOf(box, window))
		for _, look := range r.recent {
			if look.window != window {
				// A LOOK AT A WINDOW OF ANOTHER SIZE is a look at
				// another layout. Dropped rather than compared.
				continue
			}
			band := keep.Intersect(look.grey.Bounds())
			if band.Empty() {
				continue
			}
			p.looks = append(p.looks, match.Gray(look.grey, band))
		}
		r.evidence[look] = p
	}
}

// boxUnder is the smallest proposed region containing a press, in window pixels.
//
// The smallest, because proposals nest: a button sits inside a bar which sits inside a panel,
// and what somebody pressed is the button. Taking the first or the largest would keep a picture
// of the thing around the control and call it the control.
func boxUnder(bounds image.Rectangle, regions []observe.Region, at observe.PointerAt) (
	image.Rectangle, bool) {

	best, found := image.Rectangle{}, false
	for _, g := range regions {
		if g.Width <= 0 || g.Height <= 0 {
			continue
		}
		if at.X < g.X || at.X > g.X+g.Width || at.Y < g.Y || at.Y > g.Y+g.Height {
			continue
		}
		box := image.Rect(
			bounds.Min.X+int(g.X*float64(bounds.Dx())),
			bounds.Min.Y+int(g.Y*float64(bounds.Dy())),
			bounds.Min.X+int((g.X+g.Width)*float64(bounds.Dx())),
			bounds.Min.Y+int((g.Y+g.Height)*float64(bounds.Dy())),
		)
		if box.Empty() {
			continue
		}
		if !found || area(box) < area(best) {
			best, found = box, true
		}
	}
	return best, found
}

func area(r image.Rectangle) int { return r.Dx() * r.Dy() }

// calibrate measures every control that was pressed, once the demonstration is over.
//
// Here rather than in the loop because this is where there is time: the person has finished, the
// window is no longer being watched, and a second of arithmetic costs nobody a look. Returns one
// record per press look, which is the same key `keepPreviews` already resolves a control to.
func (r *visualResult) calibrate() map[int]controlevidence.Record {
	if len(r.evidence) == 0 {
		return nil
	}
	out := make(map[int]controlevidence.Record, len(r.evidence))
	for look, p := range r.evidence {
		if len(p.looks) == 0 {
			continue
		}
		// THE CLICK IN WINDOW PIXELS. Kept normalised while it travelled,
		// because that is how a press is recorded everywhere else; turned into
		// pixels here, beside the window size it is relative to, because that
		// is what a guard needs and what the record says it is.
		//
		// AND THE DISPLAY SCALE IS NOT CLAIMED. Nothing in this path observes
		// it, and writing 1 would be inventing a measurement that a consumer
		// would then compare against a window captured at 150%.
		at := image.Pt(int(p.at.X*float64(p.window.X)),
			int(p.at.Y*float64(p.window.Y)))
		c := controlevidence.Watch(at, p.box, p.window,
			controlevidence.ScaleUnknown)
		for _, look := range p.looks {
			c.Look(look, p.window)
		}
		out[look] = c.Record()
	}
	return out
}

// keepControls measures every pressed control and writes what it is worth beside the routine.
//
// The label mapping is the one `keepPreviews` uses and is explained there: `a.Steps[i]` is
// `draft.Steps[i]`, so the artifact's label and the draft's target sit at the same index, and
// `ev.Observations` says which look each target's press arrived on. That look is the key the
// evidence was filed under while it was being collected.
//
// Returns how many were written, a sentence for each one worth saying out loud, and the first
// reason one was not.
func keepControls(store *demo.Store, a workflowstore.Artifact, draft workflow.Draft,
	res visualResult, ev observe.ClickEvidence) (int, []string, string) {

	records := res.calibrate()
	if len(records) == 0 {
		return 0, nil, ""
	}
	at := map[observe.VisualTargetID]int{}
	for _, p := range ev.Observations {
		if p.Target == "" {
			continue
		}
		if _, had := at[p.Target]; !had {
			at[p.Target] = p.Look
		}
	}
	kept, why := 0, ""
	var said []string
	done := map[string]bool{}
	for i, st := range draft.Steps {
		if i >= len(a.Steps) || st.Target == "" {
			continue
		}
		label := a.Steps[i].Control
		if label == "" || done[label] {
			continue
		}
		look, ok := at[st.Target]
		if !ok {
			continue
		}
		r, ok := records[look]
		if !ok {
			continue
		}
		done[label] = true
		if err := store.SaveControl(a, label, r); err != nil {
			if why == "" {
				why = err.Error()
			}
			continue
		}
		kept++
		// THE ONE SENTENCE WORTH A PERSON'S ATTENTION, and only when there is
		// one. A control that measured well says nothing: a recording that
		// narrated every success would bury the one line that matters.
		if r.Why != "" {
			said = append(said, saysOfControl(a, i, r))
		}
	}
	return kept, said, why
}

// saysOfControl names the interaction a reader would recognise, and what was found about it.
//
// By its position in the demonstration and by the WORDS READ ON THE CONTROL rather than by
// `control_1`, which is a name this machine gave itself and has never meant anything to anybody.
// The same rule the rest of the surface follows: machine names do not reach a person.
func saysOfControl(a workflowstore.Artifact, i int, r controlevidence.Record) string {
	where := fmt.Sprintf("the %s press", ordinal(i+1))
	if i < len(a.Steps) {
		for _, c := range a.Controls {
			if c.Label == a.Steps[i].Control && len(c.Text) > 0 {
				where = fmt.Sprintf("%s (%s)", where,
					strings.Join(c.Text, " "))
				break
			}
		}
	}
	return where + ": " + r.Why
}

// ordinal is 1 as "first", for a sentence somebody reads.
func ordinal(n int) string {
	switch n {
	case 1:
		return "first"
	case 2:
		return "second"
	case 3:
		return "third"
	}
	return fmt.Sprintf("%dth", n)
}
