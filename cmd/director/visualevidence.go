package main

// WHAT A DEMONSTRATION KEEPS ABOUT THE CONTROL SOMEBODY PRESSED.
//
// # Why this is here and not in the look loop
//
// Because calibrating a control is expensive and a look is 480 milliseconds long. Measuring what
// a template scores against everything within a band of itself is about a billion operations,
// and a recorder that did that while the person was still demonstrating would drop looks — on
// the one path where a dropped look is a reading nobody takes and an outcome nobody can check.
//
// So the LOOKS are kept, cheaply, and the measuring happens once at the end when nothing is
// waiting. What is kept is a band of grey levels around the control rather than the frame it
// came from: a 1920x1080 frame is eight megabytes and a band around a button is a couple of
// hundred kilobytes, and the band is all the calibration ever looks at. See
// internal/director/controlevidence.
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

// evidenceLooks is how many looks of a pressed control are kept to calibrate it.
//
// Five, which is `observe.OutcomeSettleLooks`: the stretch the recorder already watches after a
// press to decide whether anything stayed. Matching it is not tidiness — it is the same claim
// from the other side. A control whose own appearance will not hold still for the five looks its
// outcome is measured over is a control whose outcome was measured over something moving.
const evidenceLooks = 5

// evidenceBand is how far around the control a kept band reaches, in window pixels.
//
// `controlevidence.Band` is what the calibration asks "what else is around here" over, so the
// band kept has to cover it: the control's region plus that much on every side, or the
// calibration would be asking about pixels nobody kept.
const evidenceBand = controlevidence.Band

// pressEvidence is the looks kept for one pressed control.
type pressEvidence struct {
	// at is the press, normalised to the window, and box the control's own bounds in
	// window pixels — the proposal that was under the press at the look it arrived on.
	at  observe.PointerAt
	box image.Rectangle
	// client is the window's client size, which every coordinate here is relative to.
	client image.Point
	// band is the rectangle kept, and looks the grey levels of it, oldest first.
	band  image.Rectangle
	looks []*image.Gray
}

// wants says whether this still needs looks.
func (p *pressEvidence) wants() bool { return len(p.looks) < evidenceLooks }

// watchPressed starts a calibration for every press on this look, and folds this look into the
// ones already running.
//
// Called on every look the session keeps a picture of, press or not: the looks AFTER a press are
// the ones that say whether the control stays the same, and a control seen once has never been
// seen to stay anything.
func (r *visualResult) watchPressed(look int, frame image.Image, presses []observe.PointerAt,
	regions []observe.Region) {

	if frame == nil {
		return
	}
	client := frame.Bounds().Size()
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
		r.evidence[look] = &pressEvidence{at: at, box: box, client: client,
			band: box.Inset(-evidenceBand).Intersect(frame.Bounds())}
	}
	// AND EVERY CALIBRATION STILL WANTING LOOKS TAKES THIS ONE.
	for _, p := range r.evidence {
		if !p.wants() || p.client != client {
			continue
		}
		p.looks = append(p.looks, grayBand(frame, p.band))
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

// grayBand is the band's grey levels, keeping the band's own origin.
//
// Grey because that is what the matching works on, and a third of the memory of the colour it
// came from. The origin is kept because a band is a piece of a window and a coordinate in it has
// to still be a coordinate in that window. See match.Gray.
func grayBand(frame image.Image, band image.Rectangle) *image.Gray {
	return match.Gray(frame, band)
}

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
		// because that is how a press is recorded everywhere else; turned
		// into pixels here, beside the client size it is relative to,
		// because that is what a guard needs and what the record says it is.
		at := image.Pt(int(p.at.X*float64(p.client.X)),
			int(p.at.Y*float64(p.client.Y)))
		c := controlevidence.Watch(at, p.box, p.client, 1)
		for _, look := range p.looks {
			c.Look(look, p.client)
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
