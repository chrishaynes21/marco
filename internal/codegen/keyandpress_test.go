package codegen_test

// A KEYSTROKE AND A CHECKED PRESS IN ONE ROUTINE.
//
// The owner's first live routine was a keystroke and then a press: Escape to close a menu, then
// a click on the control behind it. Nothing in this tree combined the two. Every test of the
// chain used presses alone and every test of a key used a route with nothing guarded in it, so
// what the chain does with a keystroke was carried by a comment and by one live run.
//
// # A KEYSTROKE IS A LINK, AND NOT BECAUSE ANYTHING CHECKS IT
//
// Nothing measures a window for a key and nothing compares one. A key is a link because it can
// fail to be DELIVERED — `SendInput` is blocked by an elevated foreground window, by UIPI, by a
// game with raw input and a lower-integrity sender — and what follows it then acts on a screen
// the key never changed. An Escape that did not arrive leaves the menu open; the press after it
// lands on whatever is now under that coordinate; and the window guard cannot see the difference,
// because the window is the same window. Geometry cannot detect a menu that stayed open.
//
// That was this contract's gap and it was found by the mutation gate on the first version of this
// file, which pinned today's behaviour instead: a key promoted to a link of its own passed every
// test here, so the question became what production did when the KEY was the thing that could not
// be performed. It carried on and reported that it performed. The test that recorded that is gone
// and the cases below replace it.
//
// # Why these are behavioural
//
// Because the claim is which things HAPPENED and in what order, and a route can read correctly
// and still perform a keystroke on a path where the thing before it did not. So the host writes
// down what it was asked to do, in order, and refuses one nominated thing.
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
	refuseKey      int  // the 1-based keystroke to refuse; 0 refuses none
	refuseActivate bool // refuse the preparation, as an application that is not there does
	presses        int
	keys           int
}

func (w *watcher) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	switch c.Action {
	case "Activate":
		w.did = append(w.did, "Activate")
		if w.refuseActivate {
			return refusedBy("no window of that application is on the screen")
		}
	case "Key":
		w.keys++
		w.did = append(w.did, "Key "+strings.Trim(c.Input.String(), `"`))
		if w.keys == w.refuseKey {
			return refusedBy("SendInput was blocked")
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

// AND A KEYSTROKE THAT DID NOT HAPPEN STOPS EVERYTHING AFTER IT.
//
// Wherever it sits. Each case has controls around the key that the machine takes without
// complaint, so what stopped the routine is the keystroke and not the arrangement: before the
// first press there are two presses that must not be reached; between two presses the first must
// survive and the second must not happen; after the last press everything demonstrated before it
// must still be in the account.
//
// The settle wait is the tell in the last case. It is the only thing after the final keystroke,
// so its absence is what says the tail went with the key instead of running regardless.
func TestAKeyThatDidNotHappenStopsEverythingAfterIt(t *testing.T) {
	for _, c := range []struct {
		where string
		steps []macroir.Step
		want  []string
	}{
		{"before the first press",
			[]macroir.Step{aKey("escape"), aMeasuredPress(), elsewhereInTheSameWindow()},
			[]string{"Activate", "Key escape"}},
		{"between two presses",
			[]macroir.Step{aMeasuredPress(), aKey("escape"), elsewhereInTheSameWindow()},
			[]string{"Activate", "Click 1", "Key escape"}},
		{"after the last press",
			[]macroir.Step{aMeasuredPress(), elsewhereInTheSameWindow(), aKey("escape")},
			[]string{"Activate", "Click 1", "Click 2", "Key escape"}},
	} {
		t.Run(c.where, func(t *testing.T) {
			w := &watcher{refuseKey: 1}
			_, out := performedBy(t, w, c.steps...)

			mustHaveDone(t, w, c.want...)
			if strings.Contains(out, "ark graphics: done") {
				t.Errorf("the routine reports that it finished, having been "+
					"unable to press a key it was shown:\n%s", out)
			}
		})
	}
}

// AND A KEY PRESSED SEVERAL TIMES IS EACH OF ITS PRESSES.
//
// `simplify` folds a run of the same key into one step with a count, and the generator wrote that
// as a `repeat` block. A block cannot stop a routine — its body runs again after the thing that
// did not happen — so a repeated key becomes one link per keystroke. The second of three is
// refused here: the first happened, the third must not, and neither must the press after them.
func TestARepeatedKeyIsEachOfItsPresses(t *testing.T) {
	thrice := aKey("escape")
	thrice.Count = 3

	w := &watcher{refuseKey: 2}
	_, out := performedBy(t, w, thrice, aMeasuredPress())

	mustHaveDone(t, w, "Activate", "Key escape", "Key escape")
	if strings.Contains(out, "ark graphics: done") {
		t.Errorf("the routine reports that it finished:\n%s", out)
	}
}

// AND ALL THREE HAPPEN WHEN THE MACHINE TAKES THEM.
//
// The control for the case above, without which it passes on a routine that presses the key once,
// or never, and stops for some other reason.
func TestARepeatedKeyPressesOncePerRepetition(t *testing.T) {
	thrice := aKey("escape")
	thrice.Count = 3

	w := &watcher{}
	performedBy(t, w, thrice, aMeasuredPress())

	mustHaveDone(t, w, "Activate", "Key escape", "Key escape", "Key escape",
		"Click 1", "Sleep")
}
