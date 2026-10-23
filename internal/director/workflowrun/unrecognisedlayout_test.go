package workflowrun_test

// A LICENSED RUN IS NOT HELD AT THE DOOR BY A SCREEN IT NEVER ASKS ABOUT.
//
// This is where geometry replay actually died, and it was the third place the same mistake was
// made in one feature. The step plan was taught not to ask about the screen. Then the press
// boundary was. The run coordinator in front of both still refused to proceed until something
// could name what was on screen — so the mode's two inner layers were vision-free and the mode
// was never reached.
//
// Measured on the owner's Ark pause menu: two live runs, agreement with the best saved screen
// at 0.500 and 0.442 against a 0.55 threshold, both refused with ZERO actions sent. That is
// precisely the case the licence exists for — a menu over a live 3D world, filed differently
// depending on what moves behind it — so the gate was turning down the one run it was meant to
// let through.
//
// The pair of tests here is the point. Without the licence the gate must still hold, or this
// stopped being a narrow exception and became the removal of a guard.
//
// See [[ADR-248-replaying-a-demonstrated-layout]].

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// theFold is a grounder that can never name what it is looking at, which is what the owner's
// pause menu does to the recogniser: every look disagrees with every saved screen.
type theFold struct{ asked int }

func (f *theFold) Establish(ctx context.Context, s *workflowstore.SessionBinding,
	at workflowrun.Sighting) error {

	f.asked++
	return errors.New("what is on screen agrees with screen_b at 0.442, below 0.55 — " +
		"this does not look like the screen \"ark graphics 2\" starts on")
}

// aDemonstratedPanelPress is the owner's GRAPHICS press: a tab that changes the panel under it,
// with the window it was demonstrated in and the point it was pressed at.
func aDemonstratedPanelPress() workflowstore.Artifact {
	at := observe.PointerAt{X: 0.4322916666666667, Y: 0.2611111111111111}
	a := aTabStep()
	a.Name = "ark graphics 2"
	a.Window = &workflowstore.WindowGeometry{
		Width: window.Width, Height: window.Height}
	a.Steps[0].At = &at
	return a
}

// blind is a look with NO screen anything can name and NO proposal under the point — the state
// of the world this mode is for.
//
// `words` is what OCR could read, and it is deliberately NOT withheld. The licence waives
// RECOGNITION, not reading: a step finished by its own outcome is checked by comparing the words
// before the press against the words after it, and those words are read off the same look whose
// arrangement matches no saved screen. That is the owner's own separation — localization and
// outcome verification are different questions — and it is why the trial's panel still has to be
// readable while its screen does not have to be nameable.
func blind(epoch uint64, at time.Time, words []observe.ReadWord) workflowrun.Sighting {
	return workflowrun.Sighting{
		Screen: "now_9", Epoch: epoch, Generation: 1,
		Window: "hwnd:1", Bounds: window, Front: true, At: at,
		Words: words,
	}
}

// anUnrecognisableRun is the coordinator over that press, against a grounder that always fails.
func anUnrecognisableRun(t *testing.T, now *time.Time, licensed bool) (
	*workflowrun.Runner, *workflowstore.SessionBinding, *theFold) {

	t.Helper()
	s := workflowstore.NewSessionBinding()
	if err := s.ConfirmStep(0, "SIMULATED fixture"); err != nil {
		t.Fatal(err)
	}
	fold := &theFold{}
	run := workflowrun.New(aDemonstratedPanelPress(), nil, fold,
		&grants{epoch: 5, generation: 1}).
		WithExecutor(marcoexec.New(&notebook{}), false).
		WithClock(func() time.Time { return *now }).
		WithConfirm(func(context.Context, workflowrun.Sighting,
			observe.PointerAt) (bool, string) {
			return true, ""
		}).
		WithFixedLayout(licensed).
		WithPatience(12)
	return run, s, fold
}

// THE LICENSED RUN PRESSES, THOUGH NOTHING COULD NAME THE SCREEN.
func TestALicensedRunPressesThroughAnUnrecognisableScreen(t *testing.T) {
	now := looked
	run, s, fold := anUnrecognisableRun(t, &now, true)

	// THE PANEL AS IT WAS, read but naming nothing. A verifiable step needs a
	// before-reading whatever route it takes, which this supplies and does not waive.
	was := panelWords("resolution", "window", "mode", "upscaler", "vsync")
	var sent workflowrun.Attempt
	for i := 0; i < 8; i++ {
		now = now.Add(100 * time.Millisecond)
		a := run.Advance(context.Background(), s, blind(5, now, was))
		if a.Outcome == workflowrun.OutcomePressed {
			sent = a
			break
		}
		if a.Outcome == workflowrun.OutcomeRefused ||
			a.Outcome == workflowrun.OutcomeFailed {
			t.Fatalf("a licensed run refused at %q: %s", a.Stage, a.Because)
		}
	}
	if sent.Outcome != workflowrun.OutcomePressed {
		t.Fatal("a licensed run sent nothing against a screen nothing could " +
			"name, which is the only case the licence is for")
	}
	// AND IT PRESSED ON THE DEMONSTRATED LAYOUT, not on something a reading it never
	// took put there. The exact point is held one layer down, where it can be
	// compared without desktop arithmetic:
	// workflowstore TestAPanelPressIsReplayedAtItsDemonstratedPoint.
	if sent.Intent == nil {
		t.Fatal("the press was sent with nobody asked what it was for")
	}
	if sent.Intent.Grounding != workflowstore.GroundingDemonstratedLayout {
		t.Fatalf("the press rests on %q rather than the demonstrated layout, so "+
			"something recognised a screen nothing could name",
			sent.Intent.Grounding)
	}
	// AND THE GROUNDER WAS STILL ASKED. Not asking would be a different claim —
	// that the mode stops perceiving — and the screen is worth knowing when it can
	// be known. What changed is only that failing to know it is no longer a reason
	// to hold the run.
	if fold.asked == 0 {
		t.Error("the grounder was never consulted; the licence suppresses the " +
			"REFUSAL, not the attempt to recognise")
	}
}

// AND WITHOUT THE LICENCE THE SAME RUN STILL SENDS NOTHING.
//
// The half that keeps this narrow. Identical world, identical artifact, licence off: the gate
// must hold, or a guard was deleted rather than an exception granted.
func TestWithoutTheLicenceAnUnrecognisableScreenStillSendsNothing(t *testing.T) {
	now := looked
	run, s, _ := anUnrecognisableRun(t, &now, false)

	was := panelWords("resolution", "window", "mode", "upscaler", "vsync")
	var last, waited workflowrun.Attempt
	for i := 0; i < 20; i++ {
		now = now.Add(100 * time.Millisecond)
		last = run.Advance(context.Background(), s, blind(5, now, was))
		if last.Outcome == workflowrun.OutcomePressed {
			t.Fatal("an unlicensed run pressed against a screen nothing " +
				"could name")
		}
		if last.Outcome == workflowrun.OutcomeWaiting && waited.Because == "" {
			waited = last
		}
		if last.Outcome == workflowrun.OutcomeRefused {
			break
		}
	}
	// ── AND IT WAS THIS GATE THAT HELD IT, not a later one ──
	//
	// Asserted because the step plan refuses an unestablished screen too, so an
	// outcome-and-stage assertion alone passes with this gate deleted outright —
	// measured, by deleting it. Safety survives that, which is worth knowing; the
	// claim here is narrower and is the one being made: the coordinator does not
	// proceed, and it says so in its own sentence, carrying the recogniser's reason
	// so the person who pressed Play learns what was on screen instead of being told
	// a screen is not established in an observation they cannot see.
	if !strings.Contains(waited.Because, "the screen could not be established") {
		t.Errorf("the run was held by %q, which is not this gate", waited.Because)
	}
	if !strings.Contains(waited.Because, "0.442") {
		t.Errorf("the reason is %q and does not carry what the recogniser "+
			"measured", waited.Because)
	}
	if last.Outcome != workflowrun.OutcomeRefused {
		t.Fatalf("an unlicensed run ended %q rather than refusing: %s",
			last.Outcome, last.Because)
	}
	if last.Stage != workflowstore.StepScreenUnestablished {
		t.Errorf("the refusal is staged %q, and the screen is what could not be "+
			"established", last.Stage)
	}
}

// AND THE LICENCE WAIVES RECOGNITION, NOT READING.
//
// A step finished by its own outcome is checked by comparing what was read before the press
// against what was read after. With nothing readable at all that comparison has one side and no
// verdict is possible — so the press must NOT go out, licence or no licence. The alternative is
// a click sent into a window nobody can say anything about, which is the thing this subsystem
// exists to refuse.
//
// This is also the one remaining vision dependency on the route, and it is deliberate: the
// owner's instruction is to keep localization and outcome verification separate, and this is the
// verification half asking for its own evidence rather than the localization half asking for a
// screen identity.
func TestALicensedRunWithAnUnreadablePanelStillSendsNothing(t *testing.T) {
	now := looked
	run, s, _ := anUnrecognisableRun(t, &now, true)

	var last workflowrun.Attempt
	for i := 0; i < 20; i++ {
		now = now.Add(100 * time.Millisecond)
		last = run.Advance(context.Background(), s, blind(5, now, nil))
		if last.Outcome == workflowrun.OutcomePressed {
			t.Fatal("a press went out for a step whose outcome could never be " +
				"checked, because nothing on the window could be read")
		}
		if last.Outcome == workflowrun.OutcomeRefused {
			break
		}
	}
	if last.Outcome != workflowrun.OutcomeRefused {
		t.Fatalf("the run ended %q rather than refusing: %s", last.Outcome,
			last.Because)
	}
	if run.Cursor() != 0 {
		t.Errorf("the cursor is %d over a step nothing was pressed for",
			run.Cursor())
	}
}

// AND WITH ITS OUTCOME READ, THE LICENSED PRESS IS VERIFIED AND THE STEP ADVANCES.
//
// The whole trial in one test: a screen nothing can name, a press at the demonstrated point, and
// a verdict from the words that appeared under the tab afterwards.
func TestALicensedPressIsVerifiedByTheWordsThatAppeared(t *testing.T) {
	now := looked
	run, s, _ := anUnrecognisableRun(t, &now, true)

	// BEFORE: the panel as it was, readable but not the demonstrated one.
	was := panelWords("resolution", "window", "mode", "upscaler", "vsync")
	var sent workflowrun.Attempt
	for i := 0; i < 8; i++ {
		now = now.Add(100 * time.Millisecond)
		sent = run.Advance(context.Background(), s, blind(5, now, was))
		if sent.Outcome == workflowrun.OutcomePressed {
			break
		}
	}
	if sent.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("the press never went out: %s / %s", sent.Outcome, sent.Because)
	}

	// AFTER: the panel the demonstration recorded.
	is := panelWords("presets", "textures", "shadows", "bloom", "foliage")
	var got workflowrun.Attempt
	for i := 0; i < observe.OutcomeSettleLooks+4; i++ {
		now = now.Add(100 * time.Millisecond)
		got = run.Advance(context.Background(), s, blind(5, now, is))
		if got.Verified != nil {
			break
		}
	}
	if got.Verified == nil {
		t.Fatalf("no verdict on a licensed press whose panel arrived: %s", got.Because)
	}
	if got.Verified.Verdict != workflowrun.VerdictVerified {
		t.Fatalf("the verdict is %q: %s", got.Verified.Verdict,
			got.Verified.Because)
	}
	if run.Cursor() != 1 {
		t.Errorf("the cursor is %d after a verified licensed press, want 1",
			run.Cursor())
	}
}
