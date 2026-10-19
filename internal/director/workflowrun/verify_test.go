package workflowrun_test

// VERIFIED, CONTRADICTED, UNCONFIRMED — three answers, through the real coordinator.
//
// The owner's requirement, and the third is the one that matters most:
//
//	Return VERIFIED, CONTRADICTED, or UNCONFIRMED/TIMEOUT as appropriate. Stop without
//	proceeding to another action if verification fails.
//
// A run that could not tell "the evidence disagrees" from "the evidence has not arrived yet"
// would report success whenever OCR was slow. So each verdict is its own test, and each asserts
// that the run STOPS on anything but the first.
//
// These drive `Advance` with a notebook executor: a program is lowered, compiled and run, and
// the last step appends a line instead of pressing a mouse button. What is being tested is the
// lifecycle, not the press.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// aTabStep is one step that presses a tab and changes the panel under it.
//
// The shape Ark's settings menu has, and the case the global fold cannot answer: the step's
// screens are the SAME label at both ends, so its destination can never be established and the
// only evidence the press worked is what appeared below the tab bar.
// [[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]].
func aTabStep() workflowstore.Artifact {
	return workflowstore.Artifact{
		Version: workflowstore.Version, Name: "ark graphics",
		SavedAt: time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC),
		Screens: []workflowstore.Screen{{Label: "screen_a"}},
		Controls: []workflowstore.Control{{Label: "control_1",
			Readings: []observe.Region{
				{X: 0.410, Y: 0.241, Width: 0.072, Height: 0.028}},
			Containers: []observe.Region{
				{X: 0.027, Y: 0.230, Width: 0.946, Height: 0.050}}}},
		Steps: []workflowstore.Step{{
			Position: 0, From: "screen_a", To: "screen_a", Control: "control_1",
			Cause: observe.CauseClick, Seen: 2,
			Outcome: []string{"bloom", "foliage", "presets", "shadows", "textures"},
		}},
	}
}

// panelWords is a reading of the panel BELOW the tab bar, which is where an outcome may be read.
func panelWords(words ...string) []observe.ReadWord {
	out := make([]observe.ReadWord, 0, len(words))
	for i, w := range words {
		out = append(out, observe.ReadWord{Text: w, Where: observe.Region{
			X: 0.10, Y: 0.35 + float64(i)*0.03, Width: 0.20, Height: 0.025}})
	}
	return out
}

// seeing is one look of the tab screen with the control findable and some words read on it.
func seeing(epoch uint64, at time.Time, words []observe.ReadWord) workflowrun.Sighting {
	return workflowrun.Sighting{
		Screen: "now_1", Epoch: epoch, Generation: 1,
		Window: "hwnd:1", Bounds: window, Front: true, At: at,
		Outcomes: []observe.Reacquisition{{
			Target: "control_1", Screen: "now_1",
			Result:   observe.ReacquireMatched,
			Where:    observe.Region{X: 0.410, Y: 0.241, Width: 0.072, Height: 0.028},
			Controls: 1, Because: "a fixture outcome",
		}},
		Words: words,
	}
}

// sameScreenGrounder establishes the step's one screen, which is both its ends.
type sameScreenGrounder struct{}

func (sameScreenGrounder) Establish(ctx context.Context, s *workflowstore.SessionBinding,
	at workflowrun.Sighting) error {
	err := s.Confirm("screen_a", at.Screen, at.Epoch, at.Generation,
		"SIMULATED fixture", at.At)
	if err != nil && strings.Contains(err.Error(), "already") {
		return nil
	}
	return err
}

// aTabRun is a coordinator over the tab step, with a clock a test can advance.
func aTabRun(t *testing.T, now *time.Time) (*workflowrun.Runner, *workflowstore.SessionBinding) {
	t.Helper()
	art := aTabStep()
	s := workflowstore.NewSessionBinding()
	if err := s.ConfirmStep(0, "SIMULATED fixture"); err != nil {
		t.Fatal(err)
	}
	run := workflowrun.New(art, nil, sameScreenGrounder{},
		&grants{epoch: 5, generation: 1}).
		WithExecutor(marcoexec.New(&notebook{}), false).
		WithClock(func() time.Time { return *now }).
		// THE WORLD RE-CHECK AT THE INSTANT OF THE PRESS, which the coordinator
		// refuses to click without: an unverifiable precondition on an operation
		// that clicks wherever the pointer is, is not a precondition.
		WithConfirm(func(context.Context, workflowrun.Sighting, observe.PointerAt) (bool, string) {
			return true, ""
		}).
		WithPatience(50)
	return run, s
}

// press drives looks until the action goes out, and returns the attempt that sent it.
func press(t *testing.T, run *workflowrun.Runner, s *workflowstore.SessionBinding,
	now *time.Time, before []observe.ReadWord) workflowrun.Attempt {

	t.Helper()
	for i := 0; i < 8; i++ {
		*now = (*now).Add(100 * time.Millisecond)
		a := run.Advance(context.Background(), s, seeing(5, *now, before))
		if a.Outcome == workflowrun.OutcomePressed {
			return a
		}
		if a.Outcome == workflowrun.OutcomeRefused ||
			a.Outcome == workflowrun.OutcomeFailed {
			t.Fatalf("the run refused before pressing: %s / %s", a.Outcome, a.Because)
		}
	}
	t.Fatal("the action never went out")
	return workflowrun.Attempt{}
}

// VERIFIED: the demonstrated panel arrives, and the step advances on post-action evidence.
//
// The only path in this subsystem that advances a step without an arrival, and the reason a
// pixel workflow can be performed at all.
func TestAnActionWhoseOutcomeArrivesIsVerifiedAndAdvances(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	run, s := aTabRun(t, &now)
	was := panelWords("resolution", "window", "mode", "upscaler", "vsync")

	sent := press(t, run, s, &now, was)
	if sent.Verified != nil {
		t.Errorf("a verdict was reached in the same look as the press: %+v",
			sent.Verified)
	}

	// THE PANEL THE DEMONSTRATION RECORDED, over the readings the check needs.
	is := panelWords("presets", "textures", "shadows", "bloom", "foliage")
	var got workflowrun.Attempt
	for i := 0; i < observe.OutcomeSettleLooks+2; i++ {
		now = now.Add(100 * time.Millisecond)
		got = run.Advance(context.Background(), s, seeing(5, now, is))
		if got.Verified != nil {
			break
		}
	}
	if got.Verified == nil {
		t.Fatalf("no verdict after %d reading(s) of the demonstrated panel: %s",
			observe.OutcomeSettleLooks+2, got.Because)
	}
	if got.Verified.Verdict != workflowrun.VerdictVerified {
		t.Fatalf("the verdict is %q: %s", got.Verified.Verdict, got.Verified.Because)
	}
	if got.Outcome != workflowrun.OutcomeVerified {
		t.Errorf("the attempt's outcome is %q, want %q", got.Outcome,
			workflowrun.OutcomeVerified)
	}
	// AND THE STEP IS DONE, which is what makes this a replay rather than a press.
	if run.Cursor() != 1 {
		t.Errorf("the cursor is %d after a verified outcome, want 1", run.Cursor())
	}
	// AND THE REPORT COUNTS IT, apart from the other two verdicts.
	rep := run.Report()
	if rep.VerifiedOK != 1 || rep.Contradicted != 0 || rep.Unconfirmed != 0 {
		t.Errorf("the report counts %d verified, %d contradicted, %d unconfirmed",
			rep.VerifiedOK, rep.Contradicted, rep.Unconfirmed)
	}
	// AND IT STILL REFUSES THE CAUSAL CLAIM, which a verified outcome is not.
	if ok, why := rep.Verified(); ok {
		t.Error("a verified outcome was reported as a verified ACTION")
	} else if !strings.Contains(why, "verified") {
		t.Errorf("the sentence no longer mentions the evidence collected: %s", why)
	}
	// AND THE TIME IS MEASURED, because the deadline was chosen from an offline rate.
	if got.Verified.Took <= 0 {
		t.Errorf("the verification reports %s from the action", got.Verified.Took)
	}
}

// CONTRADICTED: enough evidence arrived and it says something else is standing.
func TestAnActionWhoseOutcomeIsSomethingElseIsContradictedAndStops(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	run, s := aTabRun(t, &now)
	press(t, run, s, &now, panelWords("presets", "textures", "shadows", "bloom", "foliage"))

	// THE AUDIO TAB'S PANEL: persistent, plentiful, and not what was demonstrated.
	wrong := panelWords("master", "music", "voice", "ambient", "sfx", "volume")
	var got workflowrun.Attempt
	for i := 0; i < observe.OutcomeSettleLooks+2; i++ {
		now = now.Add(100 * time.Millisecond)
		got = run.Advance(context.Background(), s, seeing(5, now, wrong))
		if got.Verified != nil {
			break
		}
	}
	if got.Verified == nil {
		t.Fatalf("no verdict on a panel that was not the demonstrated one: %s",
			got.Because)
	}
	if got.Verified.Verdict != workflowrun.VerdictContradicted {
		t.Fatalf("the verdict is %q: %s", got.Verified.Verdict, got.Verified.Because)
	}
	if got.Outcome != workflowrun.OutcomeRefused {
		t.Errorf("a contradicted outcome is %q, want refused", got.Outcome)
	}
	// AND THE RUN DOES NOT GO ON. The cursor stays where it was, so nothing presses
	// the next step on evidence that the last one did not work.
	if run.Cursor() != 0 {
		t.Errorf("the cursor advanced to %d on a contradicted outcome", run.Cursor())
	}
	if rep := run.Report(); rep.Contradicted != 1 || rep.VerifiedOK != 0 {
		t.Errorf("the report counts %d contradicted and %d verified",
			rep.Contradicted, rep.VerifiedOK)
	}
}

// UNCONFIRMED: the deadline passes without enough readings, and that is not a failure.
//
// The verdict that stops a slow reader being read as a successful action. Driven by a clock
// rather than by real OCR, because what is being tested is that the deadline produces its own
// answer — and a test that waited for real OCR would be testing the OCR.
func TestAnActionWhoseEvidenceNeverArrivesIsUnconfirmedAndNotASuccess(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	run, s := aTabRun(t, &now)
	run.WithVerifyDeadline(2 * time.Second)
	press(t, run, s, &now, panelWords("resolution", "window", "mode", "upscaler", "vsync"))

	// LOOKS WITH NOTHING READ ON THEM, which is what a reader that cannot keep up
	// produces: the pixels arrive, the words do not.
	var got workflowrun.Attempt
	for i := 0; i < 40; i++ {
		now = now.Add(100 * time.Millisecond)
		got = run.Advance(context.Background(), s, seeing(5, now, nil))
		if got.Verified != nil {
			break
		}
	}
	if got.Verified == nil {
		t.Fatalf("the deadline passed and no verdict was produced: %s", got.Because)
	}
	if got.Verified.Verdict != workflowrun.VerdictUnconfirmed {
		t.Fatalf("the verdict is %q: %s", got.Verified.Verdict, got.Verified.Because)
	}
	if got.Verified.Readings != 0 {
		t.Errorf("%d reading(s) were counted on looks nobody read",
			got.Verified.Readings)
	}
	// IT IS NOT A SUCCESS.
	if run.Cursor() != 0 {
		t.Errorf("the cursor advanced to %d on an unconfirmed outcome", run.Cursor())
	}
	rep := run.Report()
	if rep.Unconfirmed != 1 || rep.VerifiedOK != 0 {
		t.Errorf("the report counts %d unconfirmed and %d verified",
			rep.Unconfirmed, rep.VerifiedOK)
	}
	// AND IT IS NOT A FAILURE EITHER, which the sentence has to say.
	if !strings.Contains(got.Verified.Because, "not the same as its having failed") {
		t.Errorf("the reason does not distinguish unknown from failed: %s",
			got.Verified.Because)
	}
	// AND THE DEADLINE WAS HONOURED rather than exceeded by a whole patience.
	if got.Verified.Took < 2*time.Second || got.Verified.Took > 4*time.Second {
		t.Errorf("the verdict came %s after the action, deadline 2s",
			got.Verified.Took)
	}
}

// AND A STEP DEMONSTRATED WITHOUT AN OUTCOME IS NOT VERIFIED AND NOT REFUSED.
//
// Every artifact saved before outcomes were recorded is in this case. A run that reported it as
// a contradiction would be blaming the application for the file.
func TestAStepWithNoOutcomeExpectationIsNeverVerifiedOrContradicted(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	art := aTabStep()
	art.Steps[0].Outcome = nil
	s := workflowstore.NewSessionBinding()
	if err := s.ConfirmStep(0, "SIMULATED fixture"); err != nil {
		t.Fatal(err)
	}
	run := workflowrun.New(art, nil, sameScreenGrounder{},
		&grants{epoch: 5, generation: 1}).
		WithExecutor(marcoexec.New(&notebook{}), false).
		WithClock(func() time.Time { return now }).
		WithConfirm(func(context.Context, workflowrun.Sighting, observe.PointerAt) (bool, string) {
			return true, ""
		}).
		WithPatience(6)

	press(t, run, s, &now, panelWords("resolution", "window"))
	for i := 0; i < 12; i++ {
		now = now.Add(100 * time.Millisecond)
		a := run.Advance(context.Background(), s,
			seeing(5, now, panelWords("presets", "textures", "shadows", "bloom",
				"foliage")))
		if a.Verified != nil {
			t.Fatalf("a step with no recorded outcome produced the verdict %q: %s",
				a.Verified.Verdict, a.Verified.Because)
		}
	}
	// IT RAN OUT OF PATIENCE INSTEAD, which is the behaviour this had before outcomes
	// existed and is the honest one for a file that cannot be checked.
	if rep := run.Report(); rep.VerifiedOK+rep.Contradicted+rep.Unconfirmed != 0 {
		t.Errorf("a step with no outcome produced %d verdict(s)",
			len(rep.Verifications))
	}
}

// THE READINGS TAKEN MID-RESPONSE ARE NOT EVIDENCE ON THE REPLAY SIDE EITHER.
//
// The same skip the demonstration applies, and the same measured reason: the persistence term
// wants a word on every reading of the settled window, and on the readings straight after a
// press the new panel is not drawn yet. A mutation run found this unguarded — every other test
// here feeds settled content from the first reading, so the skip was doing nothing it could see.
func TestTheReplayIgnoresTheReadingsTakenMidResponse(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	run, s := aTabRun(t, &now)
	press(t, run, s, &now, panelWords("resolution", "window", "mode"))

	// THE TRANSITION: the panel is not drawn, so these readings carry none of it.
	for i := 0; i < observe.OutcomeSkipReadings; i++ {
		now = now.Add(100 * time.Millisecond)
		if a := run.Advance(context.Background(), s,
			seeing(5, now, panelWords("loading"))); a.Verified != nil {
			t.Fatalf("a verdict was reached on the transition itself: %q %s",
				a.Verified.Verdict, a.Verified.Because)
		}
	}
	// AND THEN THE DEMONSTRATED PANEL, standing.
	is := panelWords("presets", "textures", "shadows", "bloom", "foliage")
	var got workflowrun.Attempt
	for i := 0; i < observe.OutcomeSettleLooks+2; i++ {
		now = now.Add(100 * time.Millisecond)
		got = run.Advance(context.Background(), s, seeing(5, now, is))
		if got.Verified != nil {
			break
		}
	}
	if got.Verified == nil {
		t.Fatalf("no verdict after the panel settled: %s", got.Because)
	}
	if got.Verified.Verdict != workflowrun.VerdictVerified {
		t.Fatalf("the verdict is %q: %s — the transition's own readings were "+
			"counted as evidence", got.Verified.Verdict, got.Verified.Because)
	}
}

// AND A SCREEN CHANGE AFTER THE ACTION IS STILL THE ACTION'S EVIDENCE.
//
// This test used to assert the opposite, and the opposite was measured wrong. The
// verification was scoped to the screen id the action was sent to, so a change big enough to
// mint a new screen discarded every reading that proved it had happened.
//
// Measured live on the controlled test menu, where the press WORKED: the run observed
// `visual_1 -> visual_2`, visual_2's words were the eighteen the step expected, and not one of
// them reached the verifier. The check exists for the case where the screen does NOT change
// ([[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]]); a change large enough to
// be named must not be the one thing that turns it off.
//
// What the old scoping was reaching for is kept below, one window at a time: the fear was that
// "any later reading of anywhere would fill in the after side", and ANYWHERE means another
// window. Inside one window, a reading saying the words this step SAVED is the outcome — that
// overlap is the test, not the label the fold happened to give the pixels.
func TestAScreenChangeAfterTheActionIsStillTheActionsEvidence(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	run, s := aTabRun(t, &now)
	run.WithVerifyDeadline(2 * time.Second)
	press(t, run, s, &now, panelWords("resolution", "window", "mode"))

	// THE SAME WINDOW, A NEW SCREEN NAME, and the words the step expected.
	is := panelWords("presets", "textures", "shadows", "bloom", "foliage")
	var got workflowrun.Attempt
	for i := 0; i < 40; i++ {
		now = now.Add(100 * time.Millisecond)
		at := seeing(5, now, is)
		at.WordsSeq = 10 + i
		at.Screen = "now_2"
		at.Outcomes[0].Screen = "now_2"
		got = run.Advance(context.Background(), s, at)
		if got.Verified != nil {
			break
		}
	}
	if got.Verified == nil {
		t.Fatalf("the deadline passed and no verdict was produced: %s", got.Because)
	}
	if got.Verified.Verdict != workflowrun.VerdictVerified {
		t.Fatalf("the verdict is %q: %s — the readings that prove the action worked "+
			"were discarded because the screen was renamed",
			got.Verified.Verdict, got.Verified.Because)
	}
}

// AND ANOTHER WINDOW'S READINGS ARE NOT THIS ACTION'S EVIDENCE.
//
// The guard the screen scoping was reaching for, kept at the level it belongs to. A reading
// taken under a different window generation is a reading of a different window — or of the same
// one after it was replaced — and crediting an action with it would credit the action with
// whatever Marco looked at next.
func TestAnotherWindowsReadingsDoNotVerifyTheAction(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	run, s := aTabRun(t, &now)
	run.WithVerifyDeadline(2 * time.Second)
	press(t, run, s, &now, panelWords("resolution", "window", "mode"))

	is := panelWords("presets", "textures", "shadows", "bloom", "foliage")
	var got workflowrun.Attempt
	for i := 0; i < 40; i++ {
		now = now.Add(100 * time.Millisecond)
		at := seeing(5, now, is)
		at.WordsSeq = 10 + i
		// A DIFFERENT WINDOW GENERATION, which is another window or a replaced one.
		at.Generation = 2
		at.Outcomes[0].Screen = "now_1"
		got = run.Advance(context.Background(), s, at)
		if got.Verified != nil {
			break
		}
	}
	if got.Verified == nil {
		t.Fatalf("the deadline passed and no verdict was produced: %s", got.Because)
	}
	if got.Verified.Verdict == workflowrun.VerdictVerified {
		t.Fatalf("another window's readings verified the action: %s",
			got.Verified.Because)
	}
	if got.Verified.Readings != 0 {
		t.Errorf("%d reading(s) of another window were counted as this action's "+
			"evidence", got.Verified.Readings)
	}
}
