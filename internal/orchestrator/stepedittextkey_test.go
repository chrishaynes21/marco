package orchestrator

// A STEP'S TEXT AND A STEP'S KEY CAN BE CHANGED.
//
// The fourth and fifth step-edit operations, through the same prepare/review/save path as every
// other: nothing is written until a review is saved, the stored step account is written beside the
// source, and the demonstration is never touched. They are the first of the control centre's
// editor capabilities brought over (see [[ADR-311-a-steps-text-and-key-can-be-changed]]). The
// control centre's own save rewrote source lines and left the account stale; these do not.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// heldSteps is a hold of shift around a press, then a press of esc.
func heldSteps() []macroir.Step {
	return []macroir.Step{
		{Kind: macroir.StepActivate, Text: "notepad"},
		{Kind: macroir.StepKeyDown, Key: "shift"},
		{Kind: macroir.StepKey, Key: "a"},
		{Kind: macroir.StepKeyUp, Key: "shift"},
		{Kind: macroir.StepKey, Key: "esc"},
	}
}

func editedFrom(steps []macroir.Step, ops ...StepEdit) ([]macroir.Step, []string, string) {
	out, says, why, _, err := applyEdits(steps, ops)
	if err != nil {
		return nil, says, err.Error()
	}
	return out, says, why
}

// ── TEXT ──────────────────────────────────────────────────────────────────────

// THE TEXT BECOMES OTHER FIXED TEXT, AND NOTHING ELSE MOVES.
func TestAStepsTextCanBeChangedToOtherFixedText(t *testing.T) {
	out, says, why := editedFrom(typingSteps(), StepEdit{Op: EditText, At: 1,
		Text: "see you, \"friend\" "})
	if why != "" {
		t.Fatalf("it refused: %s", why)
	}
	if len(out) != 4 || out[1].Kind != macroir.StepType {
		t.Fatalf("the list is now %+v", out)
	}
	// EXACTLY AS GIVEN: the quotes and the trailing space are part of what it types.
	if out[1].Text != "see you, \"friend\" " {
		t.Errorf("the step types %q", out[1].Text)
	}
	if out[0].Text != "notepad" || out[2].Key != "enter" || out[3].Ms != 50 {
		t.Errorf("the rest of the play moved: %+v", out)
	}
	if len(says) != 1 || !strings.Contains(says[0], "hello there") ||
		!strings.Contains(says[0], "friend") {
		t.Errorf("the review does not say what it typed and what it will type: %v", says)
	}
}

// AND NEW TEXT THAT WOULD CHANGE WHAT THE ROUTINE DOES IS REFUSED, EACH WITH ITS OWN REASON.
//
// A brace is the one that matters most: `{{name}}` undeclared renders as a read of the
// credential store, so a person's sentence would become a secret lookup.
func TestNewTextIsRefusedWhenItIsNotFixedText(t *testing.T) {
	for _, c := range []struct {
		what  string
		steps []macroir.Step
		edit  StepEdit
		says  string
	}{
		{"a placeholder", typingSteps(), StepEdit{Op: EditText, At: 1, Text: "hi {{name}}"},
			"asks for a value"},
		{"a closing brace alone", typingSteps(), StepEdit{Op: EditText, At: 1, Text: "a }} b"},
			"asks for a value"},
		{"nothing", typingSteps(), StepEdit{Op: EditText, At: 1, Text: ""},
			"remove the step instead"},
		{"the same text", typingSteps(), StepEdit{Op: EditText, At: 1, Text: "hello there"},
			"already types"},
		{"a step that types nothing", typingSteps(), StepEdit{Op: EditText, At: 2, Text: "x"},
			"does not type anything"},
		{"a step that asks for its text", []macroir.Step{
			{Kind: macroir.StepType, Text: "{{item}}"}},
			StepEdit{Op: EditText, At: 0, Text: "stone"}, "no fixed text"},
	} {
		t.Run(c.what, func(t *testing.T) {
			out, _, why := editedFrom(c.steps, c.edit)
			if why == "" {
				t.Fatalf("it was accepted: %+v", out)
			}
			if !strings.Contains(why, c.says) {
				t.Errorf("it was refused, saying %q", why)
			}
		})
	}
}

// ── KEYS ──────────────────────────────────────────────────────────────────────

// A PRESS TAKES ANOTHER KEY MARCO CAN PRESS, A CHORD INCLUDED, SPELT AS A STEP SPELLS ONE.
func TestAPressCanUseAnotherKeyMarcoCanPress(t *testing.T) {
	for _, c := range []struct{ typed, want string }{
		{"F5", "f5"}, {" ctrl+S ", "ctrl+s"}, {"pagedown", "pagedown"},
	} {
		out, says, why := editedFrom(typingSteps(), StepEdit{Op: EditKey, At: 2, Key: c.typed})
		if why != "" {
			t.Fatalf("%q was refused: %s", c.typed, why)
		}
		if out[2].Key != c.want || out[2].Kind != macroir.StepKey {
			t.Errorf("%q became %+v", c.typed, out[2])
		}
		if len(says) != 1 || !strings.Contains(says[0], "enter") ||
			!strings.Contains(says[0], c.want) {
			t.Errorf("the review says %v", says)
		}
	}
}

// AND ONE MARCO CANNOT PRESS IS REFUSED NOW, NOT AT RUN TIME AS `unknown key`.
func TestAKeyMarcoCannotPressIsRefusedAtReview(t *testing.T) {
	for _, c := range []struct {
		what  string
		steps []macroir.Step
		edit  StepEdit
		says  string
	}{
		{"an unknown name", typingSteps(), StepEdit{Op: EditKey, At: 2, Key: "hyper"},
			"not a key Marco can press"},
		{"a chord with an unknown part", typingSteps(), StepEdit{Op: EditKey, At: 2,
			Key: "ctrl+hyper"}, "not a key Marco can press"},
		{"a trailing plus", typingSteps(), StepEdit{Op: EditKey, At: 2, Key: "ctrl+"},
			"not a key Marco can press"},
		{"nothing", typingSteps(), StepEdit{Op: EditKey, At: 2, Key: "  "},
			"enter the key"},
		{"the same key", typingSteps(), StepEdit{Op: EditKey, At: 2, Key: "Enter"},
			"already uses"},
		{"a step that presses no key", typingSteps(), StepEdit{Op: EditKey, At: 1,
			Key: "a"}, "does not press a key"},
		{"a hold with an unknown part", heldSteps(), StepEdit{Op: EditKey, At: 1,
			Key: "shift+hyper"}, "not a key Marco can hold"},
	} {
		t.Run(c.what, func(t *testing.T) {
			out, _, why := editedFrom(c.steps, c.edit)
			if why == "" {
				t.Fatalf("it was accepted: %+v", out)
			}
			if !strings.Contains(why, c.says) {
				t.Errorf("it was refused, saying %q", why)
			}
		})
	}
}

// A HELD KEY CHANGES WHOLE, from either half, and the review says both steps.
func TestAHeldKeyChangesWholeFromEitherHalf(t *testing.T) {
	for _, at := range []int{1, 3} {
		out, says, why := editedFrom(heldSteps(), StepEdit{Op: EditKey, At: at, Key: "ctrl"})
		if why != "" {
			t.Fatalf("changing step %d was refused: %s", at+1, why)
		}
		if out[1].Key != "ctrl" || out[3].Key != "ctrl" {
			t.Errorf("changing step %d left the hold as %q / %q", at+1, out[1].Key,
				out[3].Key)
		}
		if out[2].Key != "a" || out[4].Key != "esc" {
			t.Errorf("changing step %d moved a press it does not hold: %+v", at+1, out)
		}
		if len(says) != 1 || !strings.Contains(says[0], "steps 2 and 4") ||
			!strings.Contains(says[0], "shift") {
			t.Errorf("the review says %v", says)
		}
	}
	// THE SAME KEY ASKED OF BOTH HALVES is one change, said once.
	_, says, why := editedFrom(heldSteps(), StepEdit{Op: EditKey, At: 1, Key: "ctrl"},
		StepEdit{Op: EditKey, At: 3, Key: "ctrl"})
	if why != "" || len(says) != 1 {
		t.Errorf("both halves asked the same key: refused %q, said %v", why, says)
	}
	// DIFFERENT KEYS ASKED OF THE TWO HALVES would hold one and release another.
	if _, _, why := editedFrom(heldSteps(), StepEdit{Op: EditKey, At: 1, Key: "ctrl"},
		StepEdit{Op: EditKey, At: 3, Key: "alt"}); !strings.Contains(why, "two different keys") {
		t.Errorf("two keys for one hold were accepted, or refused saying %q", why)
	}
}

// ── TWO THINGS ASKED OF ONE STEP ─────────────────────────────────────────────

// REFUSED IN EITHER ORDER, because the order a page sends its changes in is not a decision.
func TestTwoThingsAskedOfOneStepAreRefusedInEitherOrder(t *testing.T) {
	for _, c := range []struct {
		what  string
		steps []macroir.Step
		a, b  StepEdit
	}{
		{"new text and removed", typingSteps(),
			StepEdit{Op: EditText, At: 1, Text: "bye"}, StepEdit{Op: EditRemove, At: 1}},
		{"new text and asked for", typingSteps(),
			StepEdit{Op: EditText, At: 1, Text: "bye"},
			StepEdit{Op: EditArgument, At: 1, Name: "message"}},
		{"two texts", typingSteps(),
			StepEdit{Op: EditText, At: 1, Text: "bye"}, StepEdit{Op: EditText, At: 1, Text: "ciao"}},
		{"another key and removed", typingSteps(),
			StepEdit{Op: EditKey, At: 2, Key: "tab"}, StepEdit{Op: EditRemove, At: 2}},
		{"a hold rekeyed and its other half removed", heldSteps(),
			StepEdit{Op: EditKey, At: 1, Key: "ctrl"}, StepEdit{Op: EditRemove, At: 3}},
	} {
		for _, order := range [][]StepEdit{{c.a, c.b}, {c.b, c.a}} {
			t.Run(c.what+"/"+string(order[0].Op)+" first", func(t *testing.T) {
				out, _, why := editedFrom(c.steps, order...)
				if why == "" {
					t.Fatalf("both were accepted: %+v", out)
				}
				if !strings.Contains(why, "nothing was changed") {
					t.Errorf("it was refused, saying %q", why)
				}
			})
		}
	}
}

// ── THROUGH THE PRODUCTION PATH ──────────────────────────────────────────────

// THE CHANGED TEXT IS WHAT THE SAVED ROUTINE TYPES, prepared, saved and run.
func TestAChangedTextIsWhatTheRoutineTypes(t *testing.T) {
	d, rt := aTypedRoutine(t, Coordinates)
	// THE FIXTURE TYPES ONE KEY STEP PER LETTER, as an unsimplified recording does, so the
	// run is declared typed text first — itself an edit, which makes the one under test an
	// edit of an already-edited routine.
	editing(t, d, rt, StepEdit{Op: EditTyped, At: 2, Upto: 10})
	e, err := d.PrepareEdit(rt, nil)
	if err != nil {
		t.Fatal(err)
	}
	at := -1
	for i, s := range e.Steps() {
		if s.Kind == macroir.StepType {
			at = i
		}
	}
	if at < 0 {
		t.Fatalf("the typed fixture has no typing step: %+v", e.Steps())
	}
	before := whatItAsked(t, d, rt, 0)
	if !hasCall(before, "OS's Type with marco") {
		t.Fatalf("the fixture never typed what is being changed: %v", before)
	}
	editing(t, d, rt, StepEdit{Op: EditText, At: at, Text: "polo"})
	calls := whatItAsked(t, d, rt, 0)
	if !hasCall(calls, "OS's Type with polo") || hasCall(calls, "OS's Type with marco") {
		t.Errorf("the edited play typed: %v", calls)
	}
	// AND THE NEXT EDIT IS OFFERED, which is the account having been written beside it.
	if again, err := d.PrepareEdit(rt, nil); err != nil || again.Refused != "" {
		t.Errorf("after the edit the play cannot be edited again: %v %q", err,
			again.Refused)
	}
}

// THE CHANGED HOLD IS WHAT THE SAVED ROUTINE HOLDS AND RELEASES.
func TestAChangedHoldIsWhatTheRoutineHolds(t *testing.T) {
	d, rt := anEditableRoutine(t, "shift click", aHeldKeyDemonstration())
	e, err := d.PrepareEdit(rt, nil)
	if err != nil {
		t.Fatal(err)
	}
	at := -1
	for i, s := range e.Steps() {
		if s.Kind == macroir.StepKeyDown && s.Key == "shift" {
			at = i
		}
	}
	if at < 0 {
		t.Fatalf("the held fixture holds no shift: %+v", e.Steps())
	}
	before := strings.Join(whatItAsked(t, d, rt, 0), "\n")
	if !strings.Contains(before, "shift") {
		t.Fatalf("the fixture never held shift:\n%s", before)
	}
	editing(t, d, rt, StepEdit{Op: EditKey, At: at, Key: "ctrl"})
	after := strings.Join(whatItAsked(t, d, rt, 0), "\n")
	if strings.Contains(after, "shift") {
		t.Errorf("the edited play still holds or releases shift:\n%s", after)
	}
	if strings.Count(after, "ctrl") < 2 {
		t.Errorf("the edited play does not both hold and release ctrl:\n%s", after)
	}
}

// ── A CHANGED KEY TYPES WHAT IT NOW PRESSES, IN EVERY LATER EDIT ──
//
// A recorded tap carries the character it produced (`Text`) beside its key, and the typed-text
// conversion trusts that character first. So a key change that kept it was a routine that pressed
// `q` and, once a later edit declared the run typed text, typed `a` again. These go through save
// and then a SECOND edit, because the defect was invisible to playback straight after the first.

// THE CHARACTER IS GONE FROM THE STEP, from both halves of a hold too.
func TestAChangedKeyKeepsNoCharacterFromTheOldOne(t *testing.T) {
	steps := heldSteps()
	steps[1].Text, steps[2].Text, steps[3].Text = "s", "a", "s"
	out, _, why := editedFrom(steps, StepEdit{Op: EditKey, At: 2, Key: "q"},
		StepEdit{Op: EditKey, At: 1, Key: "ctrl"})
	if why != "" {
		t.Fatalf("it refused: %s", why)
	}
	for _, at := range []int{1, 2, 3} {
		if out[at].Text != "" {
			t.Errorf("step %d now %q still carries %q from the key it had", at+1,
				out[at].Key, out[at].Text)
		}
	}
}

// a → q, SAVED, THEN DECLARED TYPED TEXT: it types q.
func TestAKeyChangedToALetterIsThatLetterWhenLaterDeclaredTyped(t *testing.T) {
	d, rt := aTypedRoutine(t, Coordinates)
	at := keyStepAt(t, d, rt, "a")
	editing(t, d, rt, StepEdit{Op: EditKey, At: at, Key: "q"})
	editing(t, d, rt, StepEdit{Op: EditTyped, At: 2, Upto: 10})
	calls := whatItAsked(t, d, rt, 0)
	if !hasCall(calls, "OS's Type with mqrco") {
		t.Errorf("the typed run is not what the keys now press: %v", calls)
	}
}

// a → ctrl+s, SAVED, THEN DECLARED TYPED TEXT: refused, because a chord types nothing.
func TestAKeyChangedToAChordIsNotTypedTextLater(t *testing.T) {
	d, rt := aTypedRoutine(t, Coordinates)
	at := keyStepAt(t, d, rt, "a")
	editing(t, d, rt, StepEdit{Op: EditKey, At: at, Key: "ctrl+s"})
	e, err := d.PrepareEdit(rt, []StepEdit{{Op: EditTyped, At: 2, Upto: 10}})
	if err == nil && e.Refused == "" {
		t.Fatalf("a run holding ctrl+s was declared typed text:\n%s", e.Now)
	}
	why := e.Refused
	if err != nil {
		why = err.Error()
	}
	if !strings.Contains(why, "ctrl+s") {
		t.Errorf("it was refused without naming the chord: %q", why)
	}
}

// keyStepAt is the position of the routine's one press of `key`.
func keyStepAt(t *testing.T, d Deps, rt routes.Route, key string) int {
	t.Helper()
	e, err := d.PrepareEdit(rt, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range e.Steps() {
		if s.Kind == macroir.StepKey && s.Key == key {
			return i
		}
	}
	t.Fatalf("the play presses no %q: %+v", key, e.Steps())
	return -1
}

// A HOLD MAY BE A COMBO, WRITTEN AS PEOPLE WRITE ONE: "Shift + W" holds `shift+w`, and both halves
// of the hold change, so what is held is what is let go of. See ADR-340.
func TestAHoldCanBeChangedToACombo(t *testing.T) {
	out, _, why := editedFrom(heldSteps(), StepEdit{Op: EditKey, At: 1, Key: "Shift + W"})
	if why != "" {
		t.Fatalf("it was refused: %s", why)
	}
	if out[1].Key != "shift+w" || out[3].Key != "shift+w" {
		t.Fatalf("the hold is %q and its release %q", out[1].Key, out[3].Key)
	}
}

// AND A PRESS IS WRITTEN THE SAME WAY: "Ctrl + Shift" is the combo `ctrl+shift`.
func TestAPressWrittenWithSpacesIsTheCombo(t *testing.T) {
	out, _, why := editedFrom(typingSteps(), StepEdit{Op: EditKey, At: 2, Key: "Ctrl + Shift"})
	if why != "" {
		t.Fatalf("it was refused: %s", why)
	}
	if out[2].Key != "ctrl+shift" {
		t.Fatalf("the press is %q", out[2].Key)
	}
}

// AND AN ADDED PRESS IS WRITTEN THE SAME WAY.
func TestAnAddedPressWrittenWithSpacesIsTheCombo(t *testing.T) {
	out, _, why := editedFrom(typingSteps(), StepEdit{Op: EditInsert, At: 1, Kind: "key", Key: "Ctrl + Shift"})
	if why != "" {
		t.Fatalf("it was refused: %s", why)
	}
	if out[2].Key != "ctrl+shift" {
		t.Fatalf("the added press is %+v", out[2])
	}
}
