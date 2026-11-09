package orchestrator

// SAYING A RUN OF STEPS IS TYPED TEXT.
//
// # The defect, measured
//
// `markHolds` writes a key down as a HOLD when another key goes down before it is released, and
// that is what typing rollover is. `theOwnersRollover` below is REDUCED FROM THE ACTUAL RECORDING
// of `cloud pull stone` — its ten key events and their offsets, and nothing else from a file of
// 688 events — and under today's rules it renders as four KeyDown/KeyUp pairs and one tap. Neither
// typing-fold pass can reach that, because both need a step carrying text.
//
// # Why a person has to say it, and what that means for these tests
//
// `s↓ t↓ s↑ t↑` typed and `q↓ e↓ q↑ e↑` held briefly in a game are the SAME SHAPE. No key name
// tells them apart (Q and E are printable) and no duration does either (a person can type slowly
// and tap a game key). So nothing here reclassifies anything: `markHolds` is untouched, an
// ordinary draft of either still holds the keys, and the conversion is an EDIT that happens when
// somebody asks for it. `TestABriefGameplayCombinationIsOfferedAndNotTakenIsStillHolds` is the
// control that says so, and it is the test the whole design has to survive.
//
// See [[ADR-282-a-run-of-steps-can-be-said-to-be-typed-text]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// theOwnersRollover is the typing from the owner's own `cloud pull stone` recording: the ten key
// events and their offsets, reduced from a recording of 688.
//
// MEASURED, NOT INVENTED. `s` down at +9.294s, `t` down at +9.387, `s` up at +9.443, and so on;
// the offsets below are those, rebased to zero. Press durations are 93–149ms and each one overlaps
// the next by 12–56ms, which is why every key but the last was written down as a hold: the
// threshold a long press would have to cross is 500ms and nothing here is close to it.
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
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
	}
	for _, m := range measured {
		out = append(out, recorder.RecordedEvent{Kind: recorder.EvKey,
			KeyName: m.key, Down: m.down, T: at(10 + m.ms),
			App: "arkascended"})
	}
	return out
}

// aBriefGameplayCombination is two keys held together and let go in the order they were pressed,
// both for less than a fifth of a second.
//
// THE COUNTEREXAMPLE, AND IT IS SHAPE-IDENTICAL TO THE ROLLOVER ABOVE. `q` down, `e` down, `q` up,
// `e` up renders as `keydown q · key "e" · keyup q`, and the first two letters of the owner's
// typing render as `keydown s · keydown t · keyup s` for the identical reason: the first press
// spans the second, so the first is a hold and nothing in the events says why it was held. What
// comes out of this one is `hold Q, press E, release Q` — the canonical game gesture, the thing
// `internal/simplify`'s own `TestHoldKeySpansClick` exists to protect.
//
// Any rule that folded the owner's typing automatically would fold this too. It exists in these
// tests to be LEFT ALONE.
func aBriefGameplayCombination() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvKey, KeyName: "q", Down: true, T: at(10), App: "arkascended"},
		{Kind: recorder.EvKey, KeyName: "e", Down: true, T: at(70), App: "arkascended"},
		{Kind: recorder.EvKey, KeyName: "q", Down: false, T: at(150), App: "arkascended"},
		{Kind: recorder.EvKey, KeyName: "e", Down: false, T: at(210), App: "arkascended"},
	}
}

// aDraftOf is a recording made into a draft the ordinary way, under today's rules.
func aDraftOf(t *testing.T, name string, events []recorder.RecordedEvent) *Draft {
	t.Helper()
	d := Deps{Out: &strings.Builder{}}
	dr := d.DraftOf(name, events, "arkascended", "the stop key", Unstated)
	if dr == nil {
		t.Fatal("the fixture produced no draft")
	}
	return dr
}

// holdsIn is the keys a step list holds down, in order, so a test can say what it found.
func holdsIn(steps []macroir.Step) []string {
	var out []string
	for _, s := range steps {
		if s.Kind == macroir.StepKeyDown {
			out = append(out, s.Key)
		}
	}
	return out
}

// typedIn is the text every `type` step of a list says, joined.
func typedIn(steps []macroir.Step) string {
	var out []string
	for _, s := range steps {
		if s.Kind == macroir.StepType {
			out = append(out, s.Text)
		}
	}
	return strings.Join(out, "|")
}

// THE OWNER'S WORD IS FOUR HELD KEYS, AND ONE RUN SOMEBODY CAN DECLARE.
//
// Both halves in one test on purpose: the defect and the offer are the same list read twice, and
// a test that asserted only the offer would pass on a build that had folded the word
// automatically — which is the thing this design refuses to do.
func TestTheOwnersRolloverIsHeldKeysAndOneDeclarableRun(t *testing.T) {
	dr := aDraftOf(t, "cloud pull stone", theOwnersRollover())

	if got := holdsIn(dr.Steps); len(got) != 4 {
		t.Fatalf("the measured recording holds %v, not the four keys ADR-280 "+
			"recorded: %+v", got, dr.Steps)
	}
	if typedIn(dr.Steps) != "" {
		t.Errorf("the word folded without anybody saying it was text: %q",
			typedIn(dr.Steps))
	}

	runs := TypedRunsIn(dr.Steps)
	if len(runs) != 1 {
		t.Fatalf("%d runs are offered for one word: %+v", len(runs), runs)
	}
	if runs[0].Text != "stone" {
		t.Errorf("the run reads %q, so the characters are not being taken at the "+
			"press", runs[0].Text)
	}
}

// AND IT BECOMES ONE STEP ONLY WHEN SOMEBODY SAYS SO.
func TestTheOwnersRolloverConvertsWhenItIsDeclared(t *testing.T) {
	dr := aDraftOf(t, "cloud pull stone", theOwnersRollover())
	run := TypedRunsIn(dr.Steps)[0]

	next, said, err := Deps{}.TypedInDraft(dr, dr.Shown(), run.From, run.To)
	if err != nil {
		t.Fatalf("the declaration was refused: %v", err)
	}
	if typedIn(next.Steps) != "stone" {
		t.Fatalf("the run did not become one typed step: %+v", next.Steps)
	}
	if got := holdsIn(next.Steps); len(got) != 0 {
		t.Errorf("the keys are still held: %v", got)
	}
	// AND THE SENTENCE SAYS WHAT IT DID AND WHAT IT COST.
	for _, want := range []string{"stone", "waits"} {
		if !strings.Contains(said, want) {
			t.Errorf("the sentence does not mention %q: %q", want, said)
		}
	}
	// AND THE STEPS NEED WRITING DOWN, or the save leaves a routine nothing can explain.
	if !next.Changed {
		t.Error("the declaration did not mark the steps as changed, so a save would " +
			"leave no account beside the play")
	}
	// AND THE DRAFT IT CAME FROM IS UNTOUCHED.
	if len(holdsIn(dr.Steps)) != 4 {
		t.Error("the original draft was edited in place")
	}
	// AND THE DEMONSTRATION IS THE SAME DEMONSTRATION.
	if len(next.Events) != len(dr.Events) {
		t.Error("the declaration changed the recording it was made from")
	}
}

// AND A BRIEF GAMEPLAY COMBINATION IS OFFERED AND, UNTAKEN, IS STILL TWO HELD KEYS.
//
// THE CONTROL THIS WHOLE DESIGN EXISTS TO PASS. `q↓ e↓ q↑ e↑` is the same shape as the owner's
// typing, so it IS offered — the window cannot tell, and pretending otherwise would be the guess
// this feature refuses to make. What must be true is that nothing happens to it: the draft as
// recorded holds both keys, and it keeps holding them until somebody declares otherwise.
func TestABriefGameplayCombinationIsOfferedAndNotTakenIsStillHolds(t *testing.T) {
	dr := aDraftOf(t, "pull the lever", aBriefGameplayCombination())

	// HOLD Q, PRESS E, RELEASE Q - and the same reading, step for step, as the first two
	// letters of the owner's word. If this ever stops being a hold, whatever stopped it
	// has reclassified a gesture, and not reclassifying one is the whole promise here.
	if got := holdsIn(dr.Steps); len(got) != 1 || got[0] != "q" {
		t.Fatalf("the combination holds %v rather than Q across E: %v", got,
			inWordsFor(dr.Steps))
	}
	if typedIn(dr.Steps) != "" {
		t.Fatalf("a gameplay combination was folded into typed text: %v",
			inWordsFor(dr.Steps))
	}
	// OFFERED, AND THAT IS THE HONEST ANSWER: it is indistinguishable from typing, and a
	// chooser that left it out would be a chooser that had guessed.
	if runs := TypedRunsIn(dr.Steps); len(runs) != 1 || runs[0].Text != "qe" {
		t.Errorf("the combination is not offered as a run, which would mean something "+
			"here is deciding on somebody's behalf: %+v", runs)
	}
	// AND IT STAYS THAT WAY WHEN RE-DERIVED, which is what an ordinary save and a rebuild
	// under today's rules both do.
	again := aDraftOf(t, "pull the lever", aBriefGameplayCombination())
	if got := holdsIn(again.Steps); len(got) != 1 || got[0] != "q" {
		t.Errorf("a second reading of the same events gave a different answer: %v",
			inWordsFor(again.Steps))
	}
}

// inWordsFor is a step list as short strings, so a failure above reads as steps rather than as a
// page of zero-valued struct fields.
func inWordsFor(steps []macroir.Step) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		switch {
		case s.Text != "":
			out = append(out, string(s.Kind)+" "+s.Text)
		case s.Key != "":
			out = append(out, string(s.Kind)+" "+s.Key)
		default:
			out = append(out, string(s.Kind))
		}
	}
	return out
}

// AND A REPEATED CHARACTER SURVIVES, EXACTLY.
//
// `macroir` folds a key pressed several times into one step with a `Count`, and a declaration that
// dropped it would turn `aaa` into `a` — the quietest way to change what a routine types. The
// "two distinct characters" rule that keeps game key-spam out of AUTOMATIC coalescing has no
// business here: somebody has said what the steps are.
func TestARepeatedCharacterSurvivesBeingDeclaredTypedText(t *testing.T) {
	steps := []macroir.Step{
		{Kind: macroir.StepKey, Key: "a", Text: "a", Count: 3},
		{Kind: macroir.StepWait, Ms: 120},
		{Kind: macroir.StepKey, Key: "h", Text: "h"},
	}
	got, err := typedRun(steps, 0, 2)
	if err != nil {
		t.Fatalf("a repeated character was refused: %v", err)
	}
	if got != "aaah" {
		t.Errorf("the repeat was not kept: %q", got)
	}
	// AND A COUNT ON SOMETHING THIS CANNOT EXPAND IS REFUSED RATHER THAN IGNORED.
	odd := []macroir.Step{
		{Kind: macroir.StepKeyDown, Key: "a", Text: "a", Count: 2},
		{Kind: macroir.StepKeyUp, Key: "a"},
	}
	if _, err := typedRun(odd, 0, 1); err == nil {
		t.Error("a held key with a repeat count was silently flattened")
	}
}

// AND A KEY HELD ACROSS THE RUN REFUSES IT, NAMING THE KEY.
//
// The range looks clean from inside — both halves of that hold are outside it and untouched — and
// converting would still change the gesture, because the conversion removes the WAITS inside the
// range and a hold is as long as what it spans. A charge-hold would quietly become as long as
// typing two letters.
func TestAKeyHeldAcrossTheRunRefusesTheConversion(t *testing.T) {
	steps := []macroir.Step{
		{Kind: macroir.StepKeyDown, Key: "q", Text: "q"},
		{Kind: macroir.StepKey, Key: "s", Text: "s"},
		{Kind: macroir.StepWait, Ms: 300},
		{Kind: macroir.StepKey, Key: "t", Text: "t"},
		{Kind: macroir.StepKeyUp, Key: "q"},
	}
	_, err := typedRun(steps, 1, 3)
	if err == nil {
		t.Fatal("a run inside somebody's hold was converted, shortening the hold")
	}
	for _, want := range []string{"q", "held"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q: %v", want, err)
		}
	}
	// AND IT IS NOT OFFERED EITHER, so the refusal is never something somebody meets
	// after pressing a button.
	for _, r := range TypedRunsIn(steps) {
		if r.From >= 1 && r.To <= 3 {
			t.Errorf("a run inside the hold is offered: %+v", r)
		}
	}
}

// AND A SELECTION COUNTED AGAINST A LIST THAT HAS MOVED IS REFUSED.
//
// Positions alone are not consent. The saved routine's editor has refused on a moved list since
// step editing existed; a draft had nothing, on the reasoning that one window holds it and there
// is no second process — but the window is the other writer, through the controls beside the one
// being pressed.
func TestADeclarationAgainstAListThatMovedIsRefused(t *testing.T) {
	dr := aDraftOf(t, "cloud pull stone", theOwnersRollover())
	run := TypedRunsIn(dr.Steps)[0]
	was := dr.Shown()

	// SOMETHING ELSE HAPPENS TO THE LIST FIRST.
	moved, _, err := Deps{}.TypedInDraft(dr, was, run.From, run.To)
	if err != nil {
		t.Fatalf("the first declaration was refused: %v", err)
	}
	if moved.Shown() == was {
		t.Fatal("the term for the list did not change when the list did")
	}
	// ── A RANGE THAT IS VALID ON THE NEW LIST, SO ONLY THE TERM CAN REFUSE IT ──
	//
	// Isolated deliberately, and the first version of this was not: it asked for the OLD
	// range, which is out of bounds on the now-shorter list, so it passed with the basis
	// check DELETED — the refusal had been about the range all along. The mutation found
	// it, which is what the mutation is for.
	if _, _, err := (Deps{}).TypedInDraft(moved, moved.Shown(), 1, 1); err != nil {
		t.Fatalf("the new list cannot have its own step declared, so this test cannot "+
			"isolate the term: %v", err)
	}
	if _, _, err := (Deps{}).TypedInDraft(moved, was, 1, 1); err == nil {
		t.Error("a selection read off the old list was applied to the new one")
	} else if !strings.Contains(err.Error(), "changed since") {
		t.Errorf("the refusal is not about the list having moved: %v", err)
	}
	// AND AN EMPTY TERM IS NOT CONSENT EITHER, because a caller that sends none is one
	// that has not been changed to send one.
	if _, _, err := (Deps{}).TypedInDraft(moved, "", 1, 1); err == nil {
		t.Error("an operation stating no list at all was applied")
	} else if !strings.Contains(err.Error(), "changed since") {
		t.Errorf("an empty term was refused for the wrong reason: %v", err)
	}
}

// AND WHAT CANNOT BE TYPED TEXT IS REFUSED AND NAMED.
func TestWhatCannotBeDeclaredTypedTextIsSaidPlainly(t *testing.T) {
	for _, c := range []struct {
		what  string
		steps []macroir.Step
		from  int
		to    int
	}{
		{"a key that types nothing", []macroir.Step{
			{Kind: macroir.StepKey, Key: "s", Text: "s"},
			{Kind: macroir.StepKey, Key: "esc"},
		}, 0, 1},
		{"a click in the middle of it", []macroir.Step{
			{Kind: macroir.StepKey, Key: "s", Text: "s"},
			{Kind: macroir.StepClick, Button: "left", X: 10, Y: 10},
			{Kind: macroir.StepKey, Key: "t", Text: "t"},
		}, 0, 2},
		{"half of a held key", []macroir.Step{
			{Kind: macroir.StepKeyDown, Key: "s", Text: "s"},
			{Kind: macroir.StepKey, Key: "t", Text: "t"},
			{Kind: macroir.StepKeyUp, Key: "s"},
		}, 0, 1},
		{"a release of something never held", []macroir.Step{
			{Kind: macroir.StepKeyUp, Key: "s"},
			{Kind: macroir.StepKey, Key: "t", Text: "t"},
		}, 0, 1},
		{"text the play already asks for", []macroir.Step{
			{Kind: macroir.StepType, Text: "{{item}}"},
			{Kind: macroir.StepKey, Key: "t", Text: "t"},
		}, 0, 1},
		{"a range that is not a range", []macroir.Step{
			{Kind: macroir.StepKey, Key: "s", Text: "s"},
		}, 0, 4},
	} {
		if _, err := typedRun(c.steps, c.from, c.to); err == nil {
			t.Errorf("%s was accepted as typed text", c.what)
		}
	}
}
