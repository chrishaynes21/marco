package codegen_test

// A KEY AND A CHECKED PRESS IN ONE ROUTINE.
//
// The owner's first live routine was a keystroke and then a press: Escape to close a menu, then
// a click on the control behind it. Nothing in this tree combined the two. Every test of the
// chain used presses alone and every test of a key used a route with nothing guarded in it, so
// the one thing the chain does with an ordinary step — put it inside the SUCCESS of the press
// before it — was carried by a comment and by one live run.
//
// # Why these are behavioural
//
// A key cannot refuse, so it cannot stop a routine, so it is not a link: it is written into the
// body of the capability that holds the NEXT press. Read as generated text that is a detail
// about where a line sits. Read as behaviour it is the difference between Escape reaching the
// application and Escape reaching whatever was in front when the application never came
// forward — and the difference between a routine that stopped at a press that did not happen
// and one that carried on pressing keys into whatever was there instead.
//
// So the host here writes down what it was asked to do, in order, and refuses one nominated
// thing. What is asserted is which things happened.
//
// WHAT THIS DOES NOT PIN, and deliberately: WHICH capability the key is written into. A key in
// the previous link's success and a key in the next link's `before` are the same instant — both
// after the earlier press, both skipped when it refuses — so a test of that choice would hold a
// shape with no consequence. The mutation gate established it: moving every key from one to the
// other changed nothing any of these tests could see.
//
// See [[ADR-251-a-press-checks-the-window-it-was-recorded-in]].

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// aKey is one demonstrated keystroke.
func aKey(name string) macroir.Step {
	return macroir.Step{Kind: macroir.StepKey, Key: name}
}

// elsewhereInTheSameWindow is a second measured press, a different place in the same window the
// first one was measured against.
func elsewhereInTheSameWindow() macroir.Step {
	s := aMeasuredPress()
	s.X, s.Y, s.RelX, s.RelY = 904, 512, 784, 448
	return s
}

// watcher is an OS host that writes down what it was asked to do and can refuse one thing.
//
// Presses are numbered as they arrive so a refusal can name one of them, and so a failure
// message says which press was reached rather than how many were.
//
// The settle wait the generator puts at the end of every routine is written down with the rest.
// It sits at the tail, so it is inside the last press's success like anything else there, and a
// list leaving it out would describe a routine that is not the one that ran.
type watcher struct {
	did            []string
	refusePress    int  // the 1-based press to refuse; 0 refuses none
	refuseActivate bool // refuse the preparation, as an application that is not there does
	refuseKey      bool // refuse the keystroke, as an undeliverable key does
	presses        int
}

func (w *watcher) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	switch c.Action {
	case "Activate":
		w.did = append(w.did, "Activate")
		if w.refuseActivate {
			return refusedBy("no window of that application is on the screen")
		}
	case "Key":
		w.did = append(w.did, "Key "+strings.Trim(c.Input.String(), `"`))
		if w.refuseKey {
			return refusedBy("that key cannot be delivered")
		}
	case "Click":
		w.presses++
		w.did = append(w.did, fmt.Sprintf("Click %d", w.presses))
		if w.presses == w.refusePress {
			return refusedBy("the window this was recorded in was 1920x1080 " +
				"and is now 1280x720")
		}
	default:
		w.did = append(w.did, c.Action)
	}
	return "ok", runtime.Absent(), nil
}

// refusedBy is a refusal in the shape the real host has: the outcome IS the refusal and it
// carries the reason. See internal/oshost's `fail`.
func refusedBy(why string) (string, runtime.Value, error) {
	return "failed", runtime.ErrVal(&runtime.Err{Message: why}), nil
}

// performedBy generates the route the way Save does, checks it, and performs it through w.
//
// Through the real generator and the real runtime, because the claim is about a SAVED routine:
// a hand-written route with the lines in the right places would be a test of this file.
func performedBy(t *testing.T, w *watcher, steps ...macroir.Step) (src, out string) {
	t.Helper()
	dir := t.TempDir()
	source, _, err := codegen.Route("ark graphics", "arkascended", steps, dir)
	if err != nil {
		t.Fatal(err)
	}
	at := filepath.Join(dir, "route.marco")
	if err := os.WriteFile(at, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := driver.Check(at, &bytes.Buffer{}, false); err != nil {
		t.Fatalf("the route does not compile: %v\n--- source ---\n%s", err, source)
	}
	var b bytes.Buffer
	if err := driver.RunFileWithHosts(at, &b, map[string]runtime.Host{"*": w}); err != nil {
		t.Fatalf("the route does not run: %v\n--- source ---\n%s\n--- output ---\n%s",
			err, source, b.String())
	}
	return source, b.String()
}

// mustHaveDone fails unless the host was asked to do exactly these things, in this order.
func mustHaveDone(t *testing.T, w *watcher, want ...string) {
	t.Helper()
	if strings.Join(w.did, " → ") == strings.Join(want, " → ") {
		return
	}
	t.Errorf("the routine did not do what it was shown.\n  did:  %s\n  want: %s",
		strings.Join(w.did, " → "), strings.Join(want, " → "))
}

// THE OWNER'S OWN ROUTINE: A KEY, AND THEN A PRESS THAT IS CHECKED.
//
// Escape to close the menu, then the click on what was behind it. It generates, it compiles, it
// runs, and it does the two things in the order they were demonstrated.
func TestAKeyAndACheckedPressAreOneRoutine(t *testing.T) {
	w := &watcher{}
	_, out := performedBy(t, w, aKey("escape"), aMeasuredPress())

	mustHaveDone(t, w, "Activate", "Key escape", "Click 1", "Sleep")
	if !strings.Contains(out, "ark graphics: done") {
		t.Errorf("the routine did not report that it finished:\n%s", out)
	}
}

// AND THE KEY WAITS FOR THE APPLICATION, so it cannot be typed into something else.
//
// The press is checked against the window it was recorded in, which is what brings the
// application forward in the first place. A key written before that preparation would be sent
// to whichever window happened to be in front — including the one that recorded it.
func TestAKeyBeforeAPressWaitsForTheApplication(t *testing.T) {
	w := &watcher{refuseActivate: true}
	_, out := performedBy(t, w, aKey("escape"), aMeasuredPress())

	mustHaveDone(t, w, "Activate")
	if strings.Contains(out, "ark graphics: done") {
		t.Errorf("the routine reported that it finished, having pressed nothing:\n%s", out)
	}
}

// A KEY BETWEEN TWO PRESSES HAPPENS BETWEEN THEM.
func TestAKeyBetweenTwoPressesHappensBetweenThem(t *testing.T) {
	w := &watcher{}
	performedBy(t, w, aMeasuredPress(), aKey("escape"), elsewhereInTheSameWindow())

	mustHaveDone(t, w, "Activate", "Click 1", "Key escape", "Click 2", "Sleep")
}

// AND NOT WHEN THE PRESS BEFORE IT DID NOT HAPPEN.
//
// This is the one the chain exists for. The first press refuses; the key after it is in that
// press's success, so it never happens, and neither does the press after that. A routine that
// typed Escape here would be typing into a window it had just been told it could not press.
func TestAKeyBetweenTwoPressesIsInsideTheFirstOnesSuccess(t *testing.T) {
	w := &watcher{refusePress: 1}
	_, out := performedBy(t, w, aMeasuredPress(), aKey("escape"), elsewhereInTheSameWindow())

	mustHaveDone(t, w, "Activate", "Click 1")
	if strings.Contains(out, "ark graphics: done") {
		t.Errorf("the routine reported that it finished, having stopped at the first "+
			"press:\n%s", out)
	}
}

// AND A KEY AFTER THE LAST PRESS IS INSIDE ITS SUCCESS TOO.
//
// Whatever follows the last link is written into that link's success rather than after the
// chain, which is the half a tail could quietly lose: there is no further press to carry it.
func TestAKeyAfterTheLastPressIsInsideItsSuccess(t *testing.T) {
	for _, c := range []struct {
		what   string
		refuse int
		want   []string
	}{
		{"the press happened", 0, []string{"Activate", "Click 1", "Key escape", "Sleep"}},
		{"the press refused", 1, []string{"Activate", "Click 1"}},
	} {
		t.Run(c.what, func(t *testing.T) {
			w := &watcher{refusePress: c.refuse}
			performedBy(t, w, aMeasuredPress(), aKey("escape"))
			mustHaveDone(t, w, c.want...)
		})
	}
}

// AND A KEY THAT DID NOT HAPPEN DOES NOT STOP THE ROUTINE.
//
// THIS RECORDS WHAT IS, AND IT IS A GAP. The chain stops at presses, because a press is what
// asked to be checked; a key is an ordinary step, so an Escape the machine would not deliver is
// passed over and the press after it goes ahead — into a menu that never closed, which the
// window guard cannot see, because the window is the same window. The routine then reports that
// it performed.
//
// It is written down rather than left implicit for two reasons. A test that asserts the gap is
// how anybody finds out it is there, and this is the routine the owner actually made: Escape and
// then a press. Whether a key should propagate is a question about the contract, not about this
// generator — it would put keys in `Unpropagatable` and make every key a link — so it is scoped
// separately and this test is expected to be the one that fails when that is decided.
//
// Found by the mutation gate on this file: a key promoted to a link of its own passed every
// other test here.
func TestAKeyThatDidNotHappenDoesNotStopTheRoutine(t *testing.T) {
	w := &watcher{refuseKey: true}
	_, out := performedBy(t, w, aMeasuredPress(), aKey("escape"), elsewhereInTheSameWindow())

	mustHaveDone(t, w, "Activate", "Click 1", "Key escape", "Click 2", "Sleep")
	if !strings.Contains(out, "ark graphics: done") {
		t.Errorf("the routine stopped at the key after all, which is a better answer "+
			"than this test describes — read the comment above and fix it here:\n%s",
			out)
	}
}
