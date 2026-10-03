package execute

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A thin reading is not an answer.
//
// # Measured
//
// Twelve identical requests to one window, nothing changing between them:
//
//	focus the Name field   ×12  →  9 performed · 2 refused · 1 unverified
//
// The two refusals were:
//
//	FAILED: the application exposed only containers and no content, so its interior was
//	        never visible — this is not evidence the target is absent
//
// The window was the same window that answered nine other times, so the thin reading was
// transient: UIA had not finished populating when the snapshot was taken. Marco refused one
// request in six because of a timing artefact, and told the person so in a sentence that
// correctly declines to blame the target and then offers them nothing to do about it.
//
// # Why a second look is free here, and was not free for the action
//
// This is the opposite side of the retry argument that narrowed the ACTION retry. That one
// risked applying a real effect twice, so it was restricted. A resolution that failed has
// touched nothing: no click was sent, no key was pressed, the desktop does not know Marco
// looked. Looking again costs one observation.
//
// So the asymmetry is the whole point, and the two live a few lines apart:
//
//	before acting   a failed look is worth repeating; nothing can be double-applied
//	after acting    a failed confirmation is NOT, because the action may have landed
//
// # Why only the unobservable verdict
//
// Because the resolver already separates the two cases and says which it is.
// `ResolutionUnobservable` means "the world was never capable of answering"; the alternative
// is a world that was good enough and whose answer is no, which target.go notes is the only
// point at which "there is no Save button" is an honest thing to say. Re-observing the second
// kind would be asking a question that was already answered.
func TestAThinReadingIsLookedAtAgainBeforeRefusing(t *testing.T) {
	// A window whose first reading arrived with its interior missing, and whose second has
	// the control that was there all along.
	// Reading 1 is thin and the request is refused on it. Reading 2 is the window as it always
	// was. Reading 3 is after the click, with the menu open — the ordinary consequence, so the
	// action verifies and the whole request succeeds rather than merely resolving.
	thin := shallowScene(t0)
	good := scene(t0.Add(time.Second), nil, menuBar()...)
	opened := scene(t0.Add(2*time.Second), nil, menuOpen()...)

	h := newHarness(thin, good, opened, opened)
	out := h.pipeline.Handle(context.Background(), "click File")

	if out.Status != directorapi.ResultDone {
		t.Fatalf("a request refused on a thin reading was not looked at again: %s (%q)\n"+
			"observations: %d", out.Status, out.Message, h.observed)
	}
	if h.observed < 2 {
		t.Errorf("only %d observation(s); the second look is the whole fix", h.observed)
	}
	if len(h.actuator.clicks) != 1 {
		t.Errorf("%d clicks; looking again must not act twice", len(h.actuator.clicks))
	}
}

// TestAWindowThatStaysThinIsStillRefused keeps the honesty.
//
// Looking again is one more look, not a loop. A window that genuinely exposes nothing — Discord
// sits near zero coverage, which is where the threshold came from — is refused, and refused with
// the OBSERVATION reason rather than with "the target is absent", because those send a person to
// two different places.
func TestAWindowThatStaysThinIsStillRefused(t *testing.T) {
	thin := shallowScene(t0)
	h := newHarness(thin, shallowScene(t0.Add(time.Second)), shallowScene(t0.Add(2*time.Second)))

	out := h.pipeline.Handle(context.Background(), "click File")

	if out.Status == directorapi.ResultDone {
		t.Fatal("a window that exposed nothing reported success")
	}
	if len(h.actuator.clicks) != 0 {
		t.Errorf("%d clicks against a window nobody could read", len(h.actuator.clicks))
	}
	if h.observed > 3 {
		t.Errorf("%d observations; a second look is not a loop", h.observed)
	}
	// AND IT STILL SAYS IT COULD NOT SEE, not that the thing is missing.
	if out.Resolution == nil || out.Resolution.Status != directorapi.ResolutionUnobservable {
		t.Errorf("the refusal lost the reason it was refused: %+v", out.Resolution)
	}
}

// TestAnAbsentTargetIsNotLookedForTwice is the other guard: a world that could see perfectly
// well and found nothing is answered, not re-asked.
//
// Without this, every honest "there is no such control" would cost a second observation and the
// product would get slower at exactly the moment it is telling you something useful.
func TestAnAbsentTargetIsNotLookedForTwice(t *testing.T) {
	good := scene(t0, nil, menuBar()...)
	h := newHarness(good, scene(t0.Add(time.Second), nil, menuBar()...))

	out := h.pipeline.Handle(context.Background(), "click Nonexistent")

	if out.Status == directorapi.ResultDone {
		t.Fatal("clicking a control nobody has reported success")
	}
	if h.observed != 1 {
		t.Errorf("%d observations for a target that was simply not there; a readable world's "+
			"answer of no is an answer", h.observed)
	}
}

// shallowScene is a window read with its interior missing: trustworthy observation quality — the
// sensors worked — and coverage under the shallow threshold. That combination is exactly what
// WorldConfidence.Shallow() names, and what a UIA tree caught mid-population looks like.
func shallowScene(at time.Time) directorapi.WorldState {
	w := scene(at, nil, obs("uia:1", directorapi.RoleWindow, "App", rect(0, 0, 800, 600)))
	w.Confidence.Coverage = 0.1
	w.Confidence.ObservationQuality = 0.9
	return w
}
