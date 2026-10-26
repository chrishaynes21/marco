package oshost

// A HOLD IS A KEY SOMEBODY IS STILL PRESSING, AND THE SAME PROMISE AS A CHORD.
//
// `hold Q, click, release Q` is the pattern, and a recording produces it whenever a key stays
// down across something else (`simplify`'s threshold is 500ms, or any other action in between).
// So a hold is a key held down on somebody's computer DELIBERATELY, for as long as the routine
// says — and the release is the same promise a chord makes, with one difference that makes it
// worse: the thing that cleans up after a hold is the end of a run, so a hold whose release goes
// wrong goes wrong at the moment nobody is watching any more.
//
// Five defects, four of them the same shapes the chord work found one layer down:
//
//   - `doKeyUp` forgot the key BEFORE finding out whether the release worked, so a refused
//     release left a key down that the route-end sweep had been told was already up. The one
//     thing that could have tried again did not know it existed.
//   - `ReleaseHeld` discarded every answer — `_ = h.b.keyUp(...)` — which made the one path whose
//     entire purpose is "nothing is left down" the one path that could not say otherwise.
//   - The keys were held in a MAP, so the sweep released them in whatever order Go felt like.
//     An application reading modifier state while a pile of holds comes apart sees a different
//     thing depending on which goes first, and nothing could pin it.
//   - `keyDown` returned `ctx.Err()` after a successful send, so a run cancelled at that instant
//     pressed the key down AND reported the hold as failed — after which nothing wrote it down as
//     held, and nothing released it. The worst of the five: a key down for ever, untracked.
//   - And nothing stopped a stopped run from starting a new hold.
//
// See [[ADR-258-a-chord-is-a-promise-to-let-go]], whose rules these are.

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// holding presses and holds a key through the production host.
func holding(t *testing.T, h *Host, ctx context.Context, name string) string {
	t.Helper()
	status, _, err := h.Invoke(runtime.HostCall{Act: "OS", Action: "keydown",
		Input: runtime.Text(name), Ctx: ctx})
	if err != nil {
		t.Fatalf("holding %q: %v", name, err)
	}
	return status
}

// releasing lets one go through the production host.
func releasingKey(t *testing.T, h *Host, ctx context.Context, name string) string {
	t.Helper()
	status, _, err := h.Invoke(runtime.HostCall{Act: "OS", Action: "keyup",
		Input: runtime.Text(name), Ctx: ctx})
	if err != nil {
		t.Fatalf("releasing %q: %v", name, err)
	}
	return status
}

// A HOLD AND ITS RELEASE LEAVE NOTHING FOR THE SWEEP TO DO.
//
// The control everything else is read against: without it, a host that never tracked a hold at
// all would satisfy every assertion below about what the sweep released.
func TestAHoldThatWasReleasedLeavesNothingHeld(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec}
	ctx := context.Background()

	holding(t, h, ctx, "q")
	releasingKey(t, h, ctx, "q")
	h.ReleaseHeld()

	if got := strings.Join(rec.keyUps, ","); got != "q" {
		t.Errorf("the machine was asked to release %q; the hold was released once and "+
			"the sweep had nothing left to do", got)
	}
	if warn := h.NotReleased(); warn != "" {
		t.Errorf("a hold that came up cleanly reported %q", warn)
	}
}

// AND A HOLD NOBODY RELEASED IS RELEASED AT THE END OF THE RUN.
//
// Which is what `ReleaseHeld` is for, and the reason a per-command process can be trusted with a
// hold at all.
func TestAHoldNobodyReleasedIsReleasedAtTheEnd(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec}

	holding(t, h, context.Background(), "q")
	h.ReleaseHeld()

	if got := strings.Join(rec.keyUps, ","); got != "q" {
		t.Errorf("the run ended holding q and the machine was asked to release %q", got)
	}
	if warn := h.NotReleased(); warn != "" {
		t.Errorf("a hold the sweep released reported %q", warn)
	}
}

// HOLDS COME UP IN THE REVERSE OF THE ORDER THEY WENT DOWN.
//
// They were held in a map, so the sweep released them in whatever order Go produced — which is
// not an order, and an application reading the keyboard while three holds come apart sees a
// different thing depending on which goes first. Reverse is what a hand does and what
// applications expect, and it is the same rule `sendChord` follows one layer down.
func TestHoldsAreReleasedInTheReverseOfTheOrderTheyWentDown(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec}
	ctx := context.Background()

	for _, k := range []string{"ctrl", "shift", "q"} {
		holding(t, h, ctx, k)
	}
	h.ReleaseHeld()

	if got, want := strings.Join(rec.keyUps, ","), "q,shift,ctrl"; got != want {
		t.Errorf("the sweep released %q, want %q — a pile of holds coming apart in "+
			"map order is a different keyboard state every run", got, want)
	}
}

// A RELEASE THE MACHINE REFUSES KEEPS THE KEY ON THE LIST, SO THE SWEEP TRIES AGAIN.
//
// The first defect: the key was forgotten before the machine was asked, so a refused release left
// it down with nothing tracking it. One refusal and then success is the case worth having — it is
// the ordinary transient, and the sweep existing is what makes it recoverable.
func TestARefusedReleaseIsTriedAgainAtTheEndOfTheRun(t *testing.T) {
	rec := &recBackend{refuseReleases: map[string]int{"q": 1}}
	h := &Host{b: rec}
	ctx := context.Background()

	holding(t, h, ctx, "q")
	if status := releasingKey(t, h, ctx, "q"); status == "ok" {
		t.Error("a release the machine refused reported ok, so a guarded routine " +
			"would carry on as though the key had come up")
	}
	h.ReleaseHeld()

	if got, want := strings.Join(rec.keyUps, ","), "q,q"; got != want {
		t.Errorf("the machine was asked to release %q, want %q — the key was "+
			"forgotten before the machine answered, so nothing tried again", got,
			want)
	}
	// AND NOTHING IS REPORTED, because by the end of the run the key was up. A warning
	// here would tell somebody their keyboard is stuck when it is not.
	if warn := h.NotReleased(); warn != "" {
		t.Errorf("a hold that came up on the second attempt reported %q", warn)
	}
}

// AND ONE THE MACHINE WILL NOT RELEASE AT ALL IS REPORTED, BY NAME.
//
// The second defect, and the quietest of the five: the sweep discarded every answer, so the one
// path whose entire purpose is "nothing is left down" was the one path that could not say
// otherwise. Nothing in the record, nothing on the card, and a key held down.
func TestAHoldTheMachineWillNotReleaseIsReported(t *testing.T) {
	rec := &recBackend{refuseReleases: map[string]int{"q": -1}}
	h := &Host{b: rec}

	holding(t, h, context.Background(), "q")
	h.ReleaseHeld()

	warn := h.NotReleased()
	switch {
	case warn == "":
		t.Fatal("the run ended with q held down and reported success, which is the " +
			"defect: the routine is over, the process is about to exit, and " +
			"every keystroke the person types next arrives with q held")
	case !strings.Contains(warn, "q"):
		t.Errorf("the report is %q and never names the key", warn)
	case !strings.Contains(warn, "still be held down"):
		t.Errorf("the report is %q and does not say the computer is still holding "+
			"it", warn)
	}
}

// AND EVERY OTHER HOLD IS STILL LET GO OF WHEN ONE OF THEM STICKS.
//
// Reporting the first failure by leaving the rest down would be the same defect with a sentence
// attached.
func TestOneStuckHoldDoesNotStrandTheOthers(t *testing.T) {
	rec := &recBackend{refuseReleases: map[string]int{"shift": -1}}
	h := &Host{b: rec}
	ctx := context.Background()

	for _, k := range []string{"ctrl", "shift", "q"} {
		holding(t, h, ctx, k)
	}
	h.ReleaseHeld()

	if got, want := strings.Join(rec.keyUps, ","), "q,shift,ctrl"; got != want {
		t.Errorf("the sweep asked for %q, want %q — every key has to be asked, "+
			"including after one of them refuses", got, want)
	}
	if warn := h.NotReleased(); !strings.Contains(warn, "shift") ||
		strings.Contains(warn, "ctrl") {
		t.Errorf("the report is %q; it should name the one that stuck and not the "+
			"ones that came up", warn)
	}
}

// A RUN THAT HAS ALREADY STOPPED DOES NOT START A HOLD.
//
// The fifth defect. A hold is released by the end of a run, so a hold started after the run was
// stopped is a key pressed down by a routine that is over — and the thing that would have
// released it has already happened.
func TestAStoppedRunDoesNotStartAHold(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if status := holding(t, h, ctx, "q"); status == "ok" {
		t.Error("a stopped run was told its hold succeeded")
	}
	if len(rec.keyDowns) != 0 {
		t.Errorf("a stopped run pressed %v down", rec.keyDowns)
	}
	h.ReleaseHeld()
	if len(rec.keyUps) != 0 {
		t.Errorf("the sweep released %v, and nothing was ever pressed", rec.keyUps)
	}
}

// AND A HOLD THAT WENT DOWN IS TRACKED EVEN IF THE RUN STOPS IN THAT INSTANT.
//
// The fourth defect, and the worst of them. `keyDown` ended with `return ctx.Err()` after the
// send, so a run cancelled between the press and the return reported the hold as FAILED — and
// `doKeyDown` then did not write it down. The key was down, nothing was tracking it, and the
// sweep released nothing. A key held for ever, invisible to every account.
//
// A send the machine took is reported as having happened, because it did. Whether the run goes
// on is decided where the run is.
func TestAHoldThatWentDownIsTrackedEvenIfTheRunStopsThatInstant(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec}
	// THE RUN IS STOPPED AFTER THE PRESS REACHED THE MACHINE — the instant the old code
	// turned into "the hold failed", which is the one case where the key really is down.
	ctx, cancel := context.WithCancel(context.Background())
	holding(t, h, ctx, "q")
	cancel()

	h.ReleaseHeld()
	if got := strings.Join(rec.keyUps, ","); got != "q" {
		t.Errorf("the sweep released %q; the key went down, so it was this run's to "+
			"release however the run ended", got)
	}
}

// AND THE RULE THAT CANNOT BE TESTED ANY OTHER WAY, READ OFF THE SOURCE.
//
// # Why this one is a source check
//
// Because the thing to prove is that a send the machine TOOK is not reported as a stop — and
// from above the backend those two are the same thing. The host sees an error either way, so no
// stand-in can tell them apart; and the only way to exercise the real one is to send real input
// into somebody's desktop, which nothing in this suite may do.
//
// The consequence is the worst in this file. `keyDown` ended with `return ctx.Err()` after the
// press, so a run cancelled in that instant pressed the key, reported failure, and was therefore
// never written down as holding anything. A key down for ever, invisible to the sweep, the
// record, and the person.
//
// So the rule is held where it can be: the hold primitives do not consult the run's context at
// all. Pressing is gated in `doKeyDown`, one layer up, where every backend is covered and a test
// can arrange it. RELEASING is gated nowhere, on purpose — a stopped run is exactly when a key
// most needs to come up.
func TestTheHoldPrimitivesDoNotReportAStop(t *testing.T) {
	src, err := os.ReadFile("backend_windows.go")
	if err != nil {
		t.Fatalf("reading the Windows backend: %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "backend_windows.go", src, 0)
	if err != nil {
		t.Fatalf("parsing the Windows backend: %v", err)
	}
	seen := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || (fn.Name.Name != "keyDown" && fn.Name.Name != "keyUp") {
			continue
		}
		seen++
		var found bool
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if ok && sel.Sel.Name == "Err" {
				found = true
			}
			return true
		})
		if found {
			t.Errorf("%s consults the run's context. A send the machine took that "+
				"reports a stop is a key pressed down and not written down as "+
				"held — nothing releases it, nothing reports it, and no test "+
				"above this layer can tell that from a key that was never "+
				"pressed. Press is gated in doKeyDown; release is gated nowhere",
				fn.Name.Name)
		}
	}
	if seen != 2 {
		t.Errorf("found %d of the two hold primitives in backend_windows.go, so this "+
			"rule is being read off something that has moved", seen)
	}
}
