package oshost

// HOW MANY TIMES THIS RUN ASKED THE MACHINE FOR SOMETHING.
//
// A routine's run record has to answer "did it touch my machine", and until now nothing counted.
// The assessment this work came from proposed counting inside `delivered`, on the stated basis
// that `oshost` "already wraps every send in `delivered`" — it does not, and part of what is held
// here is the measurement of that: four `delivered` call sites against eleven sending actions, so
// a routine that typed its password or dragged something would have been recorded as having asked
// for nothing at all.
//
// So the count is taken in `Invoke` against a closed classification, and the test that matters
// most is the one that reads the switch and refuses an action nobody has classified.

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// EVERY ACTION THE HOST OFFERS SAYS WHETHER IT SENDS INPUT.
//
// Read off the `Invoke` switch rather than from a list kept beside it, because a list beside it is
// the thing that goes stale. An action added without a classification fails here and has to be
// decided, which is the whole point: defaulting to "does not send" would silently under-count what
// reached somebody's machine, in the one field the record has for answering that.
func TestEveryHostActionSaysWhetherItSendsInput(t *testing.T) {
	raw, err := os.ReadFile("oshost.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	i := strings.Index(src, "func (h *Host) Invoke(")
	if i < 0 {
		t.Fatal("Invoke is gone; every host action used to be dispatched there")
	}
	body := src[i:]
	if end := strings.Index(body, "\n// doMoveWindow"); end > 0 {
		body = body[:end]
	}
	found := 0
	for _, part := range strings.Split(body, `case "`)[1:] {
		labels, _, _ := strings.Cut(part, ":")
		for _, label := range strings.Split(`"`+labels, ",") {
			name := strings.Trim(strings.TrimSpace(label), `"`)
			if name == "" {
				continue
			}
			found++
			if _, classified := sendsInput[name]; !classified {
				t.Errorf("the host action %q does not say whether it sends "+
					"input; add it to sendsInput either way", name)
			}
		}
	}
	// A PARSE THAT FINDS NOTHING CLASSIFIES NOTHING, and would pass silently.
	if found < 20 {
		t.Fatalf("only %d actions were read out of the switch; the parse is wrong", found)
	}
	// AND NOTHING IS CLASSIFIED THAT THE HOST DOES NOT OFFER. A stale entry is a
	// classification of an action nobody can call, which reads as coverage.
	for name := range sendsInput {
		if !strings.Contains(body, `"`+name+`"`) {
			t.Errorf("sendsInput classifies %q, which the Invoke switch no longer "+
				"offers", name)
		}
	}
}

// AND TWO OF THEM ARE THE ONES THAT WOULD HAVE BEEN MISSED.
//
// Typing and dragging did not go through `delivered`, so the count first proposed for this would
// have read zero for a routine that did both. These two are that measurement.
//
// A drag goes through `delivered` now, and the measurement is unchanged and still needed: the
// count is taken in `Invoke` against `sendsInput`, which is the whole point of where it is taken
// — a number that depended on each action remembering to report would start reading low again
// the next time one of them did not. Typing still does not.
func TestTypingAndDraggingAreAskedFor(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec}
	if st, _, _ := call(h, "Type", runtime.Text("hello")); st != "ok" {
		t.Fatalf("typing: %q", st)
	}
	s := runtime.NewSet()
	s.Put("FromX", runtime.Number(10))
	s.Put("FromY", runtime.Number(10))
	s.Put("ToX", runtime.Number(40))
	s.Put("ToY", runtime.Number(40))
	if st, _, _ := call(h, "Drag", runtime.SetVal(s)); st != "ok" {
		t.Fatalf("dragging: %q", st)
	}
	if got := h.InputActions(); got != 2 {
		t.Errorf("the run asked for %d input actions, want 2 — one for the typing "+
			"and one for the drag; the backend was told to type %v and drag %v",
			got, rec.typed, rec.drags)
	}
}

// AND LOOKING AT THE SCREEN IS NOT ASKING FOR INPUT.
//
// The number answers "did it touch my machine". A colour sampled off the screen and a question
// about which window is in front touch nothing, and counting them would make the answer useless
// for the one thing it is for.
func TestLookingAtTheScreenIsNotAskingForInput(t *testing.T) {
	h := &Host{b: &recBackend{exe: "notepad.exe"}}
	for _, action := range []string{"Focus", "Name"} {
		_, _, _ = call(h, action, runtime.Text(""))
	}
	if got := h.InputActions(); got != 0 {
		t.Errorf("looking at the screen asked for %d input actions, want none", got)
	}
}

// AND AN ACTION REFUSED BY ITS OWN GUARD STILL ASKED FOR IT.
//
// A guarded press that cannot check the window it was recorded in is a press somebody MEANT to
// happen. A record saying the run asked for nothing would be the wrong half of the story, and the
// other half is `Unperformed`, which names it. So the count is taken before the action is
// dispatched, and both halves are written down.
func TestAPressRefusedByItsGuardStillAskedForIt(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec} // nothing activated, so no window is bound
	if st, _, _ := call(h, "Click", runtime.SetVal(aGuardedPress())); st == "ok" {
		t.Fatal("an unbound guarded press was performed; this case is about a refusal")
	}
	if got := h.InputActions(); got != 1 {
		t.Errorf("a refused press asked for %d input actions, want 1", got)
	}
	if len(h.Unperformed()) == 0 {
		t.Error("a refused press asked for an action and left nothing in the " +
			"account; the count on its own would read as a press that happened")
	}
	if len(rec.moves) != 0 || len(rec.clicks) != 0 {
		t.Errorf("the refusal still reached the machine: moved %v, clicked %v",
			rec.moves, rec.clicks)
	}
}
