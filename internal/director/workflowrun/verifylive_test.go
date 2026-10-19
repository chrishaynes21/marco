package workflowrun_test

// THE READING IDENTITY, AND WHY EVERY TEST IN verify_test.go MISSES IT.
//
// `seeing` leaves `WordsSeq` zero, which means "the caller does not distinguish readings" and
// takes the path that existed before it did. So the whole of the live production path — one
// reading standing for six looks, a late reading describing an older frame, a transition
// landing inside the settled window — was untested, and a mutation run said so: six survivors
// of nine over logic added to fix six defects found on a live desktop.
//
// Each test here names the live measurement it came from. The defects are recorded in
// [[Experiment-090-the-first-guarded-pixel-action]].

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// seeingSeq is one look whose reading is identified, as the live caller identifies them.
func seeingSeq(epoch uint64, at time.Time, words []observe.ReadWord,
	seq int) workflowrun.Sighting {

	s := seeing(epoch, at, words)
	s.WordsSeq = seq
	return s
}

// pressSeq drives looks with a fresh reading each time until the action goes out.
func pressSeq(t *testing.T, run *workflowrun.Runner, s *workflowstore.SessionBinding,
	now *time.Time, before []observe.ReadWord, firstSeq int) (workflowrun.Attempt, int) {

	t.Helper()
	seq := firstSeq
	for i := 0; i < 8; i++ {
		*now = (*now).Add(100 * time.Millisecond)
		a := run.Advance(context.Background(), s, seeingSeq(5, *now, before, seq))
		if a.Outcome == workflowrun.OutcomePressed {
			return a, seq
		}
		if a.Outcome == workflowrun.OutcomeRefused || a.Outcome == workflowrun.OutcomeFailed {
			t.Fatalf("the run refused before pressing: %s / %s", a.Outcome, a.Because)
		}
		seq++
	}
	t.Fatal("the action never went out")
	return workflowrun.Attempt{}, seq
}

// TestOneReadingStandingForManyLooksIsOnePieceOfEvidence.
//
// OCR answers about four times a second against a loop that looks ten times, so the last
// reading stands for the looks that follow it. Counting it once per look filled a five-reading
// window from ONE reading, faster than any application could respond: measured live, a press
// was CONTRADICTED 572ms after it went out, on "5 fresh reading(s)" that were five copies of
// the reading taken before it.
func TestOneReadingStandingForManyLooksIsOnePieceOfEvidence(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	run, s := aTabRun(t, &now)
	run.WithVerifyDeadline(2 * time.Second)
	_, seq := pressSeq(t, run, s, &now, panelWords("resolution", "window", "mode"), 1)

	// ONE READING OF THE NEW PANEL, standing for forty looks.
	is := panelWords("presets", "textures", "shadows", "bloom", "foliage")
	var got workflowrun.Attempt
	for i := 0; i < 40; i++ {
		now = now.Add(100 * time.Millisecond)
		got = run.Advance(context.Background(), s, seeingSeq(5, now, is, seq+1))
		if got.Verified != nil {
			break
		}
	}
	if got.Verified == nil {
		t.Fatalf("the deadline passed and no verdict was produced: %s", got.Because)
	}
	if got.Verified.Verdict == workflowrun.VerdictVerified {
		t.Fatalf("one reading repeated for forty looks verified the action: %s",
			got.Verified.Because)
	}
	if got.Verified.Readings > 1 {
		t.Errorf("%d reading(s) were counted from one reading standing for forty "+
			"looks", got.Verified.Readings)
	}
}

// TestALateReadingOfAnOlderFrameIsNotAfterEvidence.
//
// The reading standing when the action went out describes the screen the press LANDED on, and
// so does anything older than it. Admitting one as after-evidence compares a reading against
// itself, and a reading can arrive out of order: OCR runs beside the loop and answers late.
func TestALateReadingOfAnOlderFrameIsNotAfterEvidence(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	run, s := aTabRun(t, &now)
	run.WithVerifyDeadline(2 * time.Second)
	// The readings before the press run 5, 6, ... so 3 and 4 are older than all of them.
	_, seq := pressSeq(t, run, s, &now, panelWords("resolution", "window", "mode"), 5)

	// LATE READINGS, numbered BELOW the one standing at the press, carrying exactly the
	// words the step expects. They describe older frames and are not evidence of what the
	// press did — whatever they happen to say. Two different numbers, alternating, so it
	// is the ordering that refuses them and not the repeat-reading guard.
	is := panelWords("presets", "textures", "shadows", "bloom", "foliage")
	var got workflowrun.Attempt
	for i := 0; i < 40; i++ {
		now = now.Add(100 * time.Millisecond)
		older := 4
		if i%2 == 1 {
			older = 3
		}
		got = run.Advance(context.Background(), s, seeingSeq(5, now, is, older))
		if got.Verified != nil {
			break
		}
	}
	if got.Verified == nil {
		t.Fatalf("the deadline passed and no verdict was produced: %s", got.Because)
	}
	if got.Verified.Verdict == workflowrun.VerdictVerified {
		t.Fatalf("a reading older than the action verified it (seq %d was standing "+
			"when it went out): %s", seq, got.Verified.Because)
	}
	if got.Verified.Readings != 0 {
		t.Errorf("%d reading(s) older than the action were counted as its evidence",
			got.Verified.Readings)
	}
}

// TestATransitionLandingInsideTheWindowRestartsIt.
//
// `OutcomeSkipReadings` is two, measured offline on a recording read at every look. Live, an
// application took longer than two readings to redraw, the five-reading window straddled the
// change, and the persistence term correctly called every word of the new panel TRANSIENT —
// measured on the controlled test menu, on a press that had worked.
func TestATransitionLandingInsideTheWindowRestartsIt(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	run, s := aTabRun(t, &now)
	run.WithVerifyDeadline(4 * time.Second)
	was := panelWords("resolution", "window", "mode")
	_, seq := pressSeq(t, run, s, &now, was, 1)

	// THE OLD PANEL IS STILL THERE for several readings past the skip, and then the new
	// one arrives. Without the restart the window holds both and nothing persists.
	is := panelWords("presets", "textures", "shadows", "bloom", "foliage")
	var got workflowrun.Attempt
	for i := 0; i < 40; i++ {
		now = now.Add(100 * time.Millisecond)
		seq++
		words := is
		if i < 5 {
			words = was
		}
		got = run.Advance(context.Background(), s, seeingSeq(5, now, words, seq))
		if got.Verified != nil {
			break
		}
	}
	if got.Verified == nil {
		t.Fatalf("the deadline passed and no verdict was produced: %s", got.Because)
	}
	if got.Verified.Verdict != workflowrun.VerdictVerified {
		t.Fatalf("the verdict is %q: %s — the window straddled the transition and "+
			"did not start again from it", got.Verified.Verdict, got.Verified.Because)
	}
	if got.Verified.Restarts == 0 {
		t.Error("the verdict was reached without the window ever restarting, so the " +
			"transition was not detected and this passed for another reason")
	}
}

// TestAScreenThatKeepsChangingDoesNotRestartForever.
//
// The restart is bounded, because a screen that changes substantially on every reading would
// start the window again on every reading and never answer. Past the bound the readings are
// taken as they come — and on a screen like that the expectation does not persist, so the
// answer is a refusal rather than silence.
func TestAScreenThatKeepsChangingDoesNotRestartForever(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	run, s := aTabRun(t, &now)
	run.WithVerifyDeadline(30 * time.Second)
	_, seq := pressSeq(t, run, s, &now, panelWords("resolution", "window", "mode"), 1)

	var got workflowrun.Attempt
	for i := 0; i < 60; i++ {
		now = now.Add(100 * time.Millisecond)
		seq++
		// EVERY READING BRINGS FIVE WORDS THE LAST ONE DID NOT HAVE.
		churn := panelWords(
			fmt.Sprintf("alpha%d", i), fmt.Sprintf("bravo%d", i),
			fmt.Sprintf("delta%d", i), fmt.Sprintf("gamma%d", i),
			fmt.Sprintf("kilo%d", i))
		got = run.Advance(context.Background(), s, seeingSeq(5, now, churn, seq))
		if got.Verified != nil {
			break
		}
	}
	if got.Verified == nil {
		t.Fatal("a screen that changes on every reading never produced a verdict, so " +
			"the window restarted forever")
	}
	if got.Verified.Verdict == workflowrun.VerdictVerified {
		t.Fatalf("a screen of churn verified the action: %s", got.Verified.Because)
	}
}

// TestAStepWithAnOutcomeIsNotPressedBeforeItsScreenHasBeenRead.
//
// An action whose outcome is verified by comparing before against after cannot be sent before
// the before exists: the verdict is then decided the moment the press goes out, whatever the
// application does. Measured live: `--from` established the screen on observation 1, the press
// went out on observation 2, the first reading arrived after it, and the run reported a working
// press as contradicted.
func TestAStepWithAnOutcomeIsNotPressedBeforeItsScreenHasBeenRead(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	run, s := aTabRun(t, &now)

	// FIVE LOOKS WITH NOTHING READ ON THEM. The control is found, the screen is
	// established and the step is plannable — and it must not be pressed.
	for i := 0; i < 5; i++ {
		now = now.Add(100 * time.Millisecond)
		a := run.Advance(context.Background(), s, seeingSeq(5, now, nil, 0))
		if a.Outcome == workflowrun.OutcomePressed {
			t.Fatalf("look %d pressed before the screen had been read once, so the "+
				"outcome could not have been verified either way", i)
		}
		if !strings.Contains(a.Because, "has not been read yet") {
			t.Fatalf("look %d: outcome %q, %q — want the wait that says the screen "+
				"has not been read", i, a.Outcome, a.Because)
		}
	}
	// AND ONE READING IS ENOUGH TO PROCEED.
	for i := 0; i < 3; i++ {
		now = now.Add(100 * time.Millisecond)
		a := run.Advance(context.Background(), s,
			seeingSeq(5, now, panelWords("resolution", "window", "mode"), i+1))
		if a.Outcome == workflowrun.OutcomePressed {
			return
		}
	}
	t.Fatal("the screen was read and the step still never went out")
}

// TestOCRNoiseDoesNotRestartTheWindowForever.
//
// The restart needs `OutcomeWords` words the previous reading lacked, not one. OCR misreads a
// word or two on most readings of a live screen, so a restart on ANY new word restarts on every
// reading, the window never fills, and nothing is ever verified — the fix for one live defect
// turned into a slower version of it.
func TestOCRNoiseDoesNotRestartTheWindowForever(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	run, s := aTabRun(t, &now)
	run.WithVerifyDeadline(4 * time.Second)
	_, seq := pressSeq(t, run, s, &now, panelWords("resolution", "window", "mode"), 1)

	var got workflowrun.Attempt
	for i := 0; i < 40; i++ {
		now = now.Add(100 * time.Millisecond)
		seq++
		// THE PANEL, STANDING — plus one misread word that is new every time.
		words := panelWords("presets", "textures", "shadows", "bloom", "foliage",
			fmt.Sprintf("sm%d", i))
		got = run.Advance(context.Background(), s, seeingSeq(5, now, words, seq))
		if got.Verified != nil {
			break
		}
	}
	if got.Verified == nil {
		t.Fatal("a panel standing under one misread word per reading never produced " +
			"a verdict, so the window restarted on the noise")
	}
	if got.Verified.Verdict != workflowrun.VerdictVerified {
		t.Fatalf("the verdict is %q: %s — one new word per reading is noise, not a "+
			"transition", got.Verified.Verdict, got.Verified.Because)
	}
	// AND IT MUST NOT HAVE RESTARTED AT ALL.
	//
	// The bound hides the threshold otherwise: a restart on any new word restarts five
	// times, stops because it is bounded, and then fills the window and verifies — the
	// right verdict, five readings late, with the threshold doing nothing. The restart
	// COUNT is where the two differ, so that is what this asks.
	if got.Verified.Restarts != 0 {
		t.Errorf("the window restarted %d time(s) on one misread word per reading; "+
			"noise is not a transition", got.Verified.Restarts)
	}
}

// TestTheWaitForAReadingIsBoundedByPatience.
//
// Every other not-ready-yet case in `Advance` stops after `patience` observations rather than
// waiting indefinitely, and this one must too: a screen that is never read is a run that never
// ends, and `--seconds` expiring is a timeout rather than an answer.
func TestTheWaitForAReadingIsBoundedByPatience(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	run, s := aTabRun(t, &now)

	var got workflowrun.Attempt
	for i := 0; i < 200; i++ {
		now = now.Add(100 * time.Millisecond)
		got = run.Advance(context.Background(), s, seeingSeq(5, now, nil, 0))
		if got.Outcome == workflowrun.OutcomeRefused {
			break
		}
		if got.Outcome == workflowrun.OutcomePressed {
			t.Fatal("the step was pressed on a screen nobody ever read")
		}
	}
	if got.Outcome != workflowrun.OutcomeRefused {
		t.Fatal("a screen that is never read waited forever; the wait is not bounded " +
			"by patience")
	}
	if !strings.Contains(got.Because, "rather than waiting indefinitely") {
		t.Errorf("%q, want the refusal that says the run stopped rather than waiting",
			got.Because)
	}
	if got.Stage != workflowstore.StepScreenUnread {
		t.Errorf("stage %q, want the one that names the missing fact as the screen "+
			"not having been read", got.Stage)
	}
}

// losesTheScreen establishes until a press has gone out, and then cannot name anything.
//
// Which is the ordinary case and not a corner: a press that changes a panel changes what the
// screen LOOKS like, and that is exactly when the fold stops recognising it.
type losesTheScreen struct{ lost *bool }

func (g losesTheScreen) Establish(ctx context.Context, s *workflowstore.SessionBinding,
	at workflowrun.Sighting) error {

	if *g.lost {
		return errors.New("nobody could say which saved screen visual_9 is")
	}
	err := s.Confirm("screen_a", at.Screen, at.Epoch, at.Generation,
		"SIMULATED fixture", at.At)
	if err != nil && strings.Contains(err.Error(), "already") {
		return nil
	}
	return err
}

// TestAnActedStepIsAnsweredEvenWhenNobodyCanNameTheScreen.
//
// Measured live on the owner's Ark session, on the first human-demonstrated action this project
// executed: the press went out under per-action authorisation, 25 readings were admitted after
// it, nobody could name the screen, and the run reported
//
//	1 action(s) were performed and no post-action evidence was collected about any of them
//
// while holding the evidence. Section 2 returns at the first look whose screen nobody can name,
// and the post-action verdict was two hundred lines below it — so a press whose result could
// not be IDENTIFIED was also a press whose result was never READ.
func TestAnActedStepIsAnsweredEvenWhenNobodyCanNameTheScreen(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	lost := false
	art := aTabStep()
	s := workflowstore.NewSessionBinding()
	if err := s.ConfirmStep(0, "SIMULATED fixture"); err != nil {
		t.Fatal(err)
	}
	run := workflowrun.New(art, nil, losesTheScreen{lost: &lost},
		&grants{epoch: 5, generation: 1}).
		WithExecutor(marcoexec.New(&notebook{}), false).
		WithClock(func() time.Time { return now }).
		WithConfirm(func(context.Context, workflowrun.Sighting, observe.PointerAt) (bool, string) {
			return true, ""
		}).
		WithPatience(50).
		WithVerifyDeadline(4 * time.Second)

	_, seq := pressSeq(t, run, s, &now, panelWords("resolution", "window", "mode"), 1)
	// FROM HERE NOBODY CAN NAME THE SCREEN, and the panel the press produced is
	// standing in front of the observer the whole time.
	lost = true
	is := panelWords("presets", "textures", "shadows", "bloom", "foliage")
	var got workflowrun.Attempt
	for i := 0; i < 40; i++ {
		now = now.Add(100 * time.Millisecond)
		seq++
		got = run.Advance(context.Background(), s, seeingSeq(5, now, is, seq))
		if got.Verified != nil {
			break
		}
	}
	if got.Verified == nil {
		t.Fatalf("the press was never answered: %s / %s — the evidence was "+
			"collected and the verdict was never asked", got.Outcome, got.Because)
	}
	if got.Verified.Verdict != workflowrun.VerdictVerified {
		t.Errorf("the verdict is %q: %s", got.Verified.Verdict, got.Verified.Because)
	}
}
