package main

// WHAT A FAILED HANDOFF WROTE DOWN, AND WHETHER IT IS ENOUGH TO DIAGNOSE ONE.
//
// Three desktop scenarios failed with *Windows would not bring %s to the front*, and that
// sentence is a CONCLUSION. What `liftChosen` established was narrower: the target process was not
// leading after the bounded wait. It did not establish that the foreground lock rejected the
// request — the call may have been accepted and the foreground handed on elsewhere, the window may
// have been restored without being activated, another window of the same process may have been
// leading, or the window doing the asking may not have owned the foreground to give away.
//
// None of those is distinguishable from the others without the readings, and none of the readings
// was kept. So `raiseWindowTraced` records four facts and concludes nothing from them, and this
// holds the channel they travel on: the field names an investigation reads, and the file a
// windowed process can write to at all.
//
// WHAT THIS CANNOT DO is produce the readings. They come from the desktop, and a fixture that
// invented them would be evidence about nothing — which is why the stand-in says so in `Why`
// rather than filling them in. The readings are what the authorised desktop run is for.
//
// See [[Alpha-candidate-2026-10-29]].

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
)

// THE TRACE CARRIES THE FOUR FACTS, UNDER NAMES THAT DO NOT MOVE.
//
// Each one answers a different question about the same failure, and the sentence that was being
// believed answers none of them:
//
//	who was leading BEFORE the ask      — whether this window had a foreground to hand on
//	the window and owner asked for      — whether the right window was even addressed
//	what the call itself returned        — accepted, or declined, and the error code behind it
//	who was leading at each look         — whether it arrived late, or went somewhere else
func TestTheRaiseTraceCarriesWhatAFailedHandoffNeedsExplaining(t *testing.T) {
	raw, err := json.Marshal(raiseTrace{
		Asked: 0x4420, AskedPID: 7788,
		LeadingBefore: 0x1111, LeadingPIDBefore: 4242,
		Restored:              true,
		SetForegroundReturned: false,
		SetForegroundError:    "Access is denied.",
		Looks: []raiseLook{
			{At: 0, Hwnd: 0x1111, PID: 4242},
			{At: 1, Hwnd: 0x2222, PID: 9999},
		},
		LeadingAfter: 0x2222, LeadingPIDAfter: 9999,
	})
	if err != nil {
		t.Fatalf("the trace cannot be written down at all: %v", err)
	}
	said := string(raw)
	for _, must := range []string{
		"asked_hwnd", "asked_pid",
		"leading_before_hwnd", "leading_before_pid",
		"setforeground_returned", "setforeground_error",
		"looks", "leading_after_hwnd", "leading_after_pid",
	} {
		if !strings.Contains(said, must) {
			t.Errorf("the trace does not carry %q, so an investigation reading it "+
				"cannot answer the question that field is for:\n%s", must, said)
		}
	}
	// AND EVERY LOOK IS THERE, IN ORDER. One reading would say where the foreground
	// ended up and not whether it moved, which is the difference between "it never
	// arrived" and "it arrived and left".
	var back raiseTrace
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("the trace does not read back: %v", err)
	}
	if len(back.Looks) != 2 || back.Looks[0].At != 0 || back.Looks[1].At != 1 {
		t.Errorf("the looks came back as %+v, so the order of the wait is lost",
			back.Looks)
	}
	if back.Looks[0].PID == back.Looks[1].PID {
		t.Error("this fixture no longer describes a foreground that MOVED, which is " +
			"the case one reading cannot tell from a foreground that never came")
	}
}

// AND THE REFUSAL SAYS WHAT WAS OBSERVED, NOT WHICH MECHANISM CAUSED IT.
//
// It said "Windows would not bring %s to the front" and explained the foreground lock as the
// reason. The readings contradicted that everywhere anybody measured: `SetForegroundWindow`
// returned accepted, with *The operation completed successfully*, and the window doing the asking
// held the foreground it was handing on. What is established is that the handoff was not
// confirmed inside the wait — and a sentence naming a cause it cannot see sends whoever reads it
// to the wrong place.
//
// Held in both directions: it states the observation, and it does NOT assert the mechanism. The
// lock may still be mentioned as a possibility, which is where it belongs.
func TestTheRefusalSaysWhatWasObservedAndNotWhyItHappened(t *testing.T) {
	a := newActs(t.TempDir())
	a.lifting = func(uintptr, uint32) (bool, bool) { return false, true }

	why := a.liftChosen(orchestrator.Pickable{HWnd: 1, PID: 2, App: "fixture",
		Title: "Marco Fixture"})
	if why == "" {
		t.Fatal("a handoff that was not confirmed was not refused")
	}
	// THE OBSERVATION, AND WHAT TO DO ABOUT IT.
	for _, must := range []string{"couldn't confirm", "Nothing was started",
		"press Run again"} {
		if !strings.Contains(why, must) {
			t.Errorf("the refusal does not say %q, so somebody reading it cannot "+
				"tell what happened or what to do: %q", must, why)
		}
	}
	// AND THE WINDOW IT WAS ABOUT.
	if !strings.Contains(why, "Marco Fixture") {
		t.Errorf("the refusal does not name the window: %q", why)
	}
	// AND IT ASSERTS NO CAUSE. The lock is a POSSIBILITY and is marked as one; a
	// sentence that states it as the reason is the thing being corrected.
	if strings.Contains(why, "Windows would not bring") {
		t.Errorf("the refusal still asserts that Windows refused, which the readings "+
			"contradict wherever they have been taken: %q", why)
	}
	if strings.Contains(why, "foreground on") && !strings.Contains(why, "One possibility") {
		t.Errorf("the refusal explains the foreground lock without marking it as a "+
			"possibility: %q", why)
	}
}

// AND IT IS WRITTEN WHERE A WINDOWED PROCESS CAN BE READ, FOR EVERY OUTCOME.
//
// Studio has no stderr anybody sees, so the channel is the file a harness asked it to write into
// — the same one the page's own proof uses. Written on success as well as on refusal: a trace that
// appears only when something went wrong cannot say what a working handoff looks like, which is
// most of what a comparison needs.
func TestTheRaiseIsRecordedForEveryOutcome(t *testing.T) {
	for _, c := range []struct {
		what    string
		leading bool
	}{
		{"a refused handoff", false},
		{"a handoff that worked", true},
	} {
		t.Run(c.what, func(t *testing.T) {
			at := filepath.Join(t.TempDir(), "evidence.txt")
			a := newActs(t.TempDir())
			a.observing(at)
			a.lifting = func(uintptr, uint32) (bool, bool) { return c.leading, true }

			why := a.liftChosen(orchestrator.Pickable{HWnd: 0x4420, PID: 7788,
				App: "fixture", Title: "Marco Fixture"})
			if c.leading && why != "" {
				t.Errorf("a handoff that worked was refused: %s", why)
			}
			if !c.leading && why == "" {
				t.Error("a handoff that did not work was not refused")
			}
			said, err := os.ReadFile(at)
			if err != nil {
				t.Fatalf("nothing was written where the harness asked: %v", err)
			}
			if !strings.Contains(string(said), "raise") {
				t.Errorf("the raise left no reading behind:\n%s", said)
			}
			// AND THE WINDOW IT WAS ASKED ABOUT, so a trace can be matched to the
			// scenario that produced it.
			if !strings.Contains(string(said), "17440") { // 0x4420
				t.Errorf("the reading does not say which window was asked "+
					"for:\n%s", said)
			}
			// AND A STAND-IN SAYS IT IS ONE. A fixture's zeroes read exactly like a
			// desktop that answered zero, and the whole point of the trace is that
			// its readings came from somewhere.
			if !strings.Contains(string(said), "a test stood in for the desktop") {
				t.Errorf("a stand-in's trace does not say it is one, so its "+
					"empty readings read as a desktop's:\n%s", said)
			}
		})
	}
}

// AND A WINDOW NOBODY ASKED FOR DIAGNOSTICS FROM WRITES NONE.
//
// `-evidence` is how a harness asks. A person's window has no such file and must not acquire one:
// a product that writes a diagnostic trace into somebody's store on every Play is a product
// keeping a log nobody asked for.
func TestAWindowNobodyAskedForDiagnosticsFromWritesNone(t *testing.T) {
	home := t.TempDir()
	a := newActs(home)
	a.lifting = func(uintptr, uint32) (bool, bool) { return false, true }

	if why := a.liftChosen(orchestrator.Pickable{HWnd: 1, PID: 2}); why == "" {
		t.Fatal("the fixture did not refuse, so nothing was recorded either way")
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "evidence") || strings.Contains(e.Name(), "raise") {
			t.Errorf("a window nobody asked wrote %q into the store", e.Name())
		}
	}
}
