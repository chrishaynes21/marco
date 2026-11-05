package orchestrator

// A STEP CAN BE ADDED: a wait, a key press or typed text, in a gap of the list that was shown.
//
// Through the same prepare/review/save contract as every other step edit. An insertion names the
// gap AFTER an original position, so it is collected with the other operations against the list
// the person was looking at and the new list is built once — which is what lets it share a review
// with a removal or a changed wait. See [[ADR-312-a-step-can-be-added]].

import (
	"strconv"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// ofKinds is a step list in words short enough to compare: kind and the one value that matters.
func ofKinds(steps []macroir.Step) string {
	var out []string
	for _, s := range steps {
		switch s.Kind {
		case macroir.StepWait:
			out = append(out, "wait:"+strconv.Itoa(s.Ms))
		case macroir.StepKey, macroir.StepKeyDown, macroir.StepKeyUp:
			out = append(out, string(s.Kind)+":"+s.Key)
		default:
			out = append(out, string(s.Kind)+":"+s.Text)
		}
	}
	return strings.Join(out, " ")
}

// EACH KIND GOES IN THE GAP IT WAS ASKED FOR, AND NOTHING ELSE MOVES.
func TestAStepIsAddedInTheGapAfterAPosition(t *testing.T) {
	for _, c := range []struct {
		edit StepEdit
		want string
		says string
	}{
		{StepEdit{Op: EditInsert, At: 1, Kind: "wait", To: 300},
			"activate:notepad type:hello there wait:300 key:enter wait:50", "a wait of 300ms"},
		{StepEdit{Op: EditInsert, At: 2, Kind: "key", Key: " F5 "},
			"activate:notepad type:hello there key:enter key:f5 wait:50", `"f5"`},
		{StepEdit{Op: EditInsert, At: 3, Kind: "type", Text: "and more "},
			"activate:notepad type:hello there key:enter wait:50 type:and more ", "and more"},
	} {
		out, says, why := editedFrom(typingSteps(), c.edit)
		if why != "" {
			t.Fatalf("%+v was refused: %s", c.edit, why)
		}
		if got := ofKinds(out); got != c.want {
			t.Errorf("%+v made\n  %s\nnot\n  %s", c.edit, got, c.want)
		}
		if len(says) != 1 || !strings.Contains(says[0], c.says) ||
			!strings.Contains(says[0], "after step "+strconv.Itoa(c.edit.At+1)) {
			t.Errorf("%+v: the review says %v", c.edit, says)
		}
	}
}

// AN ADDITION SHARES A REVIEW WITH OTHER CHANGES, every position meaning the list on screen.
//
// Remove step 3 (enter), change the wait at step 4, add a wait after step 2 and a key after the
// REMOVED step 3: the key goes where enter was, and the changed wait is still the one that was
// step 4 — nothing was renumbered under the person.
func TestAnAdditionSharesAReviewWithOtherChanges(t *testing.T) {
	out, says, why := editedFrom(typingSteps(),
		StepEdit{Op: EditRemove, At: 2},
		StepEdit{Op: EditWait, At: 3, To: 75},
		StepEdit{Op: EditInsert, At: 1, Kind: "wait", To: 200},
		StepEdit{Op: EditInsert, At: 2, Kind: "key", Key: "tab"},
		StepEdit{Op: EditInsert, At: 2, Kind: "key", Key: "esc"})
	if why != "" {
		t.Fatalf("it refused: %s", why)
	}
	want := "activate:notepad type:hello there wait:200 key:tab key:esc wait:75"
	if got := ofKinds(out); got != want {
		t.Errorf("the list is\n  %s\nnot\n  %s", got, want)
	}
	if len(says) != 5 {
		t.Errorf("the review does not say all five changes: %v", says)
	}
}

// NOTHING IS ADDED BEFORE THE WINDOW COMES FORWARD — and where nothing brings one forward, the
// start of the routine is a gap like any other.
func TestNothingIsAddedBeforeTheWindowComesForward(t *testing.T) {
	_, _, why := editedFrom(typingSteps(), StepEdit{Op: EditInsert, At: -1, Kind: "key", Key: "a"})
	if !strings.Contains(why, "before step 1, which brings notepad forward") {
		t.Errorf("a press before the window comes forward was accepted, or refused "+
			"saying %q", why)
	}
	steps := typingSteps()[1:] // no activate
	out, _, why := editedFrom(steps, StepEdit{Op: EditInsert, At: -1, Kind: "wait", To: 10})
	if why != "" || ofKinds(out)[:len("wait:10")] != "wait:10" {
		t.Errorf("adding at the start of a routine that brings nothing forward: refused %q, "+
			"made %s", why, ofKinds(out))
	}
}

// AND WHAT IS ADDED OBEYS THE RULES A CHANGE OF THAT KIND OBEYS.
func TestAnAdditionIsRefusedWhenAChangeOfItsKindWouldBe(t *testing.T) {
	for _, c := range []struct {
		what string
		edit StepEdit
		says string
	}{
		{"a key Marco cannot press", StepEdit{Op: EditInsert, At: 1, Kind: "key", Key: "hyper"},
			"not a key Marco can press"},
		{"no key", StepEdit{Op: EditInsert, At: 1, Kind: "key", Key: " "}, "enter the key"},
		{"braces", StepEdit{Op: EditInsert, At: 1, Kind: "type", Text: "{{x}}"},
			"asks for a value"},
		{"no text", StepEdit{Op: EditInsert, At: 1, Kind: "type", Text: ""}, "type the text"},
		{"a negative wait", StepEdit{Op: EditInsert, At: 1, Kind: "wait", To: -5},
			"cannot be -5ms"},
		{"a click", StepEdit{Op: EditInsert, At: 1, Kind: "click"},
			"not a step that can be added"},
		{"past the end", StepEdit{Op: EditInsert, At: 4, Kind: "wait", To: 5},
			"no step 5"},
	} {
		t.Run(c.what, func(t *testing.T) {
			out, _, why := editedFrom(typingSteps(), c.edit)
			if why == "" {
				t.Fatalf("it was accepted: %s", ofKinds(out))
			}
			if !strings.Contains(why, c.says) {
				t.Errorf("it was refused, saying %q", why)
			}
		})
	}
}

// ADDED, SAVED, RUN, AND EDITABLE AGAIN — through the production path.
func TestAnAddedStepIsWhatTheRoutineDoes(t *testing.T) {
	d, rt := aThreePressRoutine(t)
	before := whatItAsked(t, d, rt, 0)
	if hasCall(before, "OS's Key with f5") {
		t.Fatal("the fixture already presses f5")
	}
	editing(t, d, rt, StepEdit{Op: EditInsert, At: 0, Kind: "key", Key: "f5"},
		StepEdit{Op: EditInsert, At: 0, Kind: "wait", To: 250})
	calls := whatItAsked(t, d, rt, 0)
	if len(calls) < 3 || !strings.HasPrefix(calls[0], "OS's Activate") ||
		calls[1] != "OS's Key with f5" || calls[2] != "OS's Sleep with 250" {
		t.Errorf("the additions are not right after the window comes forward, in order: %v",
			calls)
	}
	if len(calls) != len(before)+2 {
		t.Errorf("the routine made %d calls, %d before and two added", len(calls),
			len(before))
	}
	e, err := d.PrepareEdit(rt, nil)
	if err != nil || e.Refused != "" {
		t.Fatalf("after the addition the routine cannot be edited again: %v %q", err,
			e.Refused)
	}
	// AND THE ADDED STEP IS A STEP LIKE ANY OTHER: it can be changed in the next edit.
	editing(t, d, rt, StepEdit{Op: EditKey, At: 1, Key: "f6"})
	if !hasCall(whatItAsked(t, d, rt, 0), "OS's Key with f6") {
		t.Error("the added key could not be changed afterwards")
	}
}
