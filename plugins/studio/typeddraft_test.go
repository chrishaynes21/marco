package main

// SAYING WHICH STEPS ARE TYPED TEXT, FROM THE CARD THAT REVIEWS THE RECORDING.
//
// The journey this closes, in the order the owner walked it: record a word in a game, find it
// written down as held keys, press Simplify and watch nothing happen, and have no way to ask the
// routine for the item name because a value has to be one step.
//
// EVERY FIXTURE HERE IS MEASURED OR IS THE COUNTEREXAMPLE. `theOwnersRollover` is ten key events
// reduced from the actual `cloud pull stone` recording; `aBriefGameplayCombination` is the gesture
// that is indistinguishable from it and must be left alone. The argument for why a person has to
// be the one who says which is which is in `typeddraft.go`.
//
// See [[ADR-282-a-run-of-steps-can-be-said-to-be-typed-text]].

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// theOwnersRollover is the typing from the owner's own `cloud pull stone` recording: its ten key
// events and their offsets, reduced from a recording of 688 events.
//
// MEASURED. `s` down at +9.294s, `t` down at +9.387, `s` up at +9.443, and so on, rebased to zero.
// Presses of 93-149ms overlapping the next by 12-56ms, which is why every key but the last was
// written down as a hold: a press would have to last 500ms to be one on duration alone.
func theOwnersRollover() []recorder.RecordedEvent {
	type press struct {
		key  string
		down bool
		ms   int
	}
	measured := []press{
		{"s", true, 0}, {"t", true, 93}, {"s", false, 149}, {"o", true, 178},
		{"t", false, 234}, {"n", true, 246}, {"o", false, 301}, {"e", true, 362},
		{"n", false, 374}, {"e", false, 455},
	}
	out := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
	}
	for _, m := range measured {
		out = append(out, recorder.RecordedEvent{Kind: recorder.EvKey,
			KeyName: m.key, Down: m.down, T: at(10 + m.ms), App: "notepad"})
	}
	return out
}

// aBriefGameplayCombination is `hold Q, press E, release Q` as a person actually produces it, in
// under a fifth of a second.
//
// THE COUNTEREXAMPLE, step-for-step identical in shape to the first two letters of the word above.
// It is offered here as readily as the typing is, because nothing can tell them apart — and it
// stays exactly as recorded unless somebody says otherwise.
func aBriefGameplayCombination() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
		{Kind: recorder.EvKey, KeyName: "q", Down: true, T: at(10), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "e", Down: true, T: at(70), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "q", Down: false, T: at(150), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "e", Down: false, T: at(210), App: "notepad"},
	}
}

// THE CARD OFFERS THE RUN THAT ROLLOVER SHREDDED, AND SAYS WHAT CONVERTING IT COSTS.
//
// The sentence is a requirement rather than a courtesy: this operation converts a long press as
// readily as a short one, because duration cannot tell typing from a game, and the condition that
// came with that is that what it REMOVES is said before anybody presses it.
func TestTheCardOffersTheRunRolloverShredded(t *testing.T) {
	a := aRecordingUnderReview(t, "cloud pull stone", theOwnersRollover())
	got := a.Recording()

	if len(got.TypedRuns) != 1 {
		t.Fatalf("%d runs are offered for one word: %+v (steps %v)",
			len(got.TypedRuns), got.TypedRuns, got.Steps)
	}
	if !strings.Contains(got.TypedRuns[0].Label, `types "stone"`) {
		t.Errorf("the option does not say what the run would become: %q",
			got.TypedRuns[0].Label)
	}
	// AND THE STEPS IT WOULD REPLACE ARE THE ONES ON SCREEN.
	if got.TypedRuns[0].From < 0 || got.TypedRuns[0].To >= len(got.Steps) {
		t.Errorf("the run points outside the list being shown: %+v of %d steps",
			got.TypedRuns[0], len(got.Steps))
	}
	for _, want := range []string{"held keys", "waits between them go",
		"Waits outside the run are left alone"} {
		if !strings.Contains(got.TypedSays, want) {
			t.Errorf("the caption does not say %q: %q", want, got.TypedSays)
		}
	}
	// AND THE CARD CARRIES A TERM FOR THE LIST, which every operation hands back.
	if got.Basis == "" {
		t.Error("the review states no basis, so a position is all an operation would " +
			"carry")
	}
}

// AND IT CONVERTS ONLY WHEN SOMEBODY SAYS SO, WITH NOTHING WRITTEN.
func TestTheOwnersWordConvertsOnlyWhenItIsDeclared(t *testing.T) {
	a := aRecordingUnderReview(t, "cloud pull stone", theOwnersRollover())
	dir := routesDirOf(t, a)

	was := a.Recording()
	steps := strings.Join(was.Steps, "|")
	if !strings.Contains(steps, "hold") && !strings.Contains(steps, "keydown") &&
		!strings.Contains(steps, "down") {
		t.Fatalf("the measured recording does not hold its keys, so this test cannot "+
			"reach the state it is about: %v", was.Steps)
	}
	if strings.Contains(steps, `"stone"`) {
		t.Fatalf("the word folded with nobody saying it was text: %v", was.Steps)
	}

	run := was.TypedRuns[0]
	if why := a.TypedDraftText(was.Basis, run.From, run.To); why != "" {
		t.Fatalf("the declaration was refused: %s", why)
	}
	now := a.Recording()
	if !strings.Contains(strings.Join(now.Steps, "|"), `stone`) {
		t.Fatalf("the run did not become one typed step: %v", now.Steps)
	}
	if len(now.Steps) >= len(was.Steps) {
		t.Errorf("the list did not get shorter: %d steps became %d", len(was.Steps),
			len(now.Steps))
	}
	if !wroteNothing(t, dir) {
		t.Error("declaring typed text wrote to the library")
	}
}

// AND THE GAMEPLAY GESTURE IS OFFERED AND, LEFT ALONE, IS STILL A HELD KEY.
//
// THE CONTROL THE WHOLE DESIGN HAS TO PASS, at the surface a person uses. It is offered — the
// window cannot tell it from typing and a chooser that hid it would have guessed — and the draft
// is unchanged until the button is pressed.
func TestAGameplayCombinationIsOfferedAndLeftAlone(t *testing.T) {
	a := aRecordingUnderReview(t, "pull the lever", aBriefGameplayCombination())
	got := a.Recording()

	if len(got.TypedRuns) != 1 {
		t.Fatalf("the gesture is not offered, so something is deciding for somebody: "+
			"%+v", got.TypedRuns)
	}
	steps := strings.Join(got.Steps, "|")
	if strings.Contains(steps, `"qe"`) {
		t.Fatalf("the gesture was folded into typed text by itself: %v", got.Steps)
	}
	// AND LOOKING AT IT AGAIN CHANGES NOTHING, however many polls the page makes.
	if again := strings.Join(a.Recording().Steps, "|"); again != steps {
		t.Errorf("the steps changed between two readings:\n%s\n%s", steps, again)
	}
}

// AND A DECLARED RUN CAN THEN BE ASKED FOR, WHICH IS THE JOURNEY THIS CLOSES.
func TestADeclaredRunCanThenBeAskedFor(t *testing.T) {
	a := aRecordingUnderReview(t, "cloud pull stone", theOwnersRollover())

	was := a.Recording()
	if len(was.Supply) != 0 {
		t.Fatalf("a recording of held keys offers a value to ask for already: %+v",
			was.Supply)
	}
	run := was.TypedRuns[0]
	if why := a.TypedDraftText(was.Basis, run.From, run.To); why != "" {
		t.Fatalf("the declaration was refused: %s", why)
	}
	now := a.Recording()
	if len(now.Supply) != 1 {
		t.Fatalf("the declared run cannot be asked for: %+v", now.Supply)
	}
	if why := a.SupplyDraftText(now.Basis, now.Supply[0].At, "item"); why != "" {
		t.Fatalf("asking for it was refused: %s", why)
	}
	if !strings.Contains(strings.Join(a.Recording().Steps, "|"), "{{item}}") {
		t.Errorf("the value is not something the play asks for: %v",
			a.Recording().Steps)
	}
}

// AND A DECLARATION SURVIVES THE SAVE, THE EDIT PANEL AND A REBUILD.
//
// The same lifecycle [[ADR-281-a-value-can-be-asked-for-from-the-review]] needed and for the same
// reason: a declared run is not what rendering this demonstration produces, so the save has to
// write the step list down beside the routine. `Draft.Changed` is what says so.
func TestADeclaredRunSurvivesSaveAndEditingAndARebuild(t *testing.T) {
	a := aRecordingUnderReview(t, "cloud pull stone", theOwnersRollover())
	was := a.Recording()
	run := was.TypedRuns[0]
	if why := a.TypedDraftText(was.Basis, run.From, run.To); why != "" {
		t.Fatalf("the declaration was refused: %s", why)
	}
	if why := a.SaveDraft("app", ""); why != "" {
		t.Fatalf("saving refused: %s", why)
	}
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d plays", len(rs))
	}
	saved := rs[0]
	if !strings.Contains(saved.Source, `do OS's Type with "stone"`) {
		t.Fatalf("the saved play does not type the word:\n%s", saved.Source)
	}
	if strings.Contains(saved.Source, "KeyDown") {
		t.Errorf("the saved play still holds the keys:\n%s", saved.Source)
	}
	// AND THE DEMONSTRATION IS THE ONE THAT WAS MADE.
	rec, err := os.ReadFile(strings.TrimSuffix(saved.Path, ".marco") + ".rec.json")
	if err != nil {
		t.Fatalf("the demonstration is not beside the play: %v", err)
	}

	// ── THE EDIT PANEL ──
	e := a.StepEditing(saved.ID)
	if e.Why != "" {
		t.Fatalf("the play that was just saved cannot be edited: %s", e.Why)
	}
	typed := 0
	for _, row := range e.Rows {
		if row.Types && row.Text == "stone" {
			typed++
		}
	}
	if typed != 1 {
		t.Errorf("the panel does not list one step typing the word: %+v", e.Rows)
	}

	// ── AND A REBUILD UNDER TODAY'S RULES KEEPS IT ──
	//
	// Which is the half that would otherwise undo the declaration quietly: with no
	// account beside it, a rebuild re-derives from the demonstration and the word goes
	// back to being held keys.
	v := a.ReviewRebuilt(saved.ID, "current")
	if v.Why != "" {
		t.Fatalf("the saved play cannot be rebuilt: %s", v.Why)
	}
	if !strings.Contains(v.Now, `do OS's Type with "stone"`) {
		t.Errorf("a rebuild under today's rules undid the declaration:\n%s", v.Now)
	}
	// AND NOTHING WAS WRITTEN BY LOOKING.
	again, rerr := os.ReadFile(strings.TrimSuffix(saved.Path, ".marco") + ".rec.json")
	if rerr != nil || string(again) != string(rec) {
		t.Errorf("the demonstration changed during a review that writes nothing (%v)",
			rerr)
	}
}

// AND A SELECTION COUNTED AGAINST A LIST THAT MOVED IS REFUSED, ON BOTH CONTROLS.
//
// Positions alone are not consent. Simplify re-derives the list, going back to the original
// replaces it, and each edit changes it — so the window is the other writer, and the term the card
// drew has to come back with the press.
func TestAnOperationAgainstAListThatMovedIsRefused(t *testing.T) {
	a := aRecordingUnderReview(t, "cloud pull stone", theOwnersRollover())
	was := a.Recording()
	run := was.TypedRuns[0]

	// THE INTERVENING CHANGE IS THE DECLARATION ITSELF, and not Simplify, which is the
	// obvious candidate and does nothing at all to THIS recording: every wait in it is
	// already under the cap and `markHolds` does not care about a typing rhythm, which is
	// the ADR-280 defect this whole slice exists for. A test that pressed Simplify here
	// would have asserted a basis change that never happened - and it did, which is how
	// this comment came to be written.
	if why := a.TypedDraftText(was.Basis, run.From, run.To); why != "" {
		t.Fatalf("the first declaration was refused: %s", why)
	}
	now := a.Recording()
	if now.Basis == was.Basis {
		t.Fatal("the term for the list did not change when the list did")
	}

	// AND THE SELECTION READ OFF THE OLD LIST IS REFUSED AGAINST THE NEW ONE.
	why := a.TypedDraftText(was.Basis, run.From, run.To)
	if why == "" {
		t.Fatal("a run read off the old list was declared against the new one")
	}
	if !strings.Contains(why, "changed since") {
		t.Errorf("the refusal does not say what happened: %q", why)
	}

	// AND THE SAME TERM GUARDS THE CONTROL BESIDE IT, which had no term at all until this
	// operation needed one.
	if len(now.Supply) == 0 {
		t.Fatal("the declared run cannot be asked for, so the second half of this " +
			"test cannot run")
	}
	if why := a.SupplyDraftText(was.Basis, now.Supply[0].At, "item"); why == "" {
		t.Error("a position read off the old list was supplied against the new one")
	}

	// AND STATING NO LIST AT ALL IS NOT CONSENT EITHER, because a caller that sends none
	// is one that has not been changed to send one.
	if why := a.TypedDraftText("", 0, 1); why == "" {
		t.Error("an operation naming no list was applied")
	} else if !strings.Contains(why, "changed since") {
		t.Errorf("an empty term was refused for the wrong reason: %q", why)
	}
	// AND NOTHING THAT WAS REFUSED CHANGED ANYTHING.
	if strings.Join(a.Recording().Steps, "|") != strings.Join(now.Steps, "|") {
		t.Errorf("a refused operation changed the draft: %v became %v",
			now.Steps,
			a.Recording().Steps)
	}
}
