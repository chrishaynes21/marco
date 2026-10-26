package oshost

// A CTRL-HELD CLICK, ALL THE WAY FROM A DEMONSTRATION TO A PRESS.
//
// # Why this exists beside the simplifier's own tests
//
// Those establish that a demonstrated Ctrl+click becomes `KeyDown ctrl`, the click, `KeyUp ctrl`.
// That is the half that was broken and it is not the whole claim: a modifier is a different kind
// of key from Q, and everything after the simplifier had only ever carried a letter. The
// generator has to make those two steps links in the chain, the resolver has to know `ctrl` as a
// key to press and hold rather than only as part of a chord string, and the Host has to track it
// as held so the route-end sweep answers for it.
//
// So every case here goes recording events → `simplify` → `codegen` → `routes` → the runtime →
// the production `Host`. Nothing is hand-written: the demonstration is the input and the
// assertions are about what the machine was asked for and what the run says afterwards.
//
// # The two failures that matter
//
//	the modifier will not go down     the click must not happen. A click with Ctrl held and
//	                                  the same click without it are different acts, and the
//	                                  window guard cannot tell them apart — the window is
//	                                  the same window
//	the modifier will not come up     whatever follows runs with Ctrl still down, which is
//	                                  worse for a modifier than for a letter: a stuck Ctrl
//	                                  changes every keystroke anybody types afterwards
//
// See [[ADR-258-a-chord-is-a-promise-to-let-go]] and [[Demonstrations]].

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// aCtrlHeldClickInTheGame is the demonstration: hold Ctrl, click something, let go.
//
// Guarded, like every other recording in this package that presses: the window facts are on the
// press, which is what asks for the window to be checked and what makes the hold a link in a
// chain rather than a step that is merely attempted.
//
// The trailing clicks in `studio` are the owner's own — pressing Stop — and the production
// recorder strips them. They are here because `savedFrom` goes through `Record`, and a
// demonstration that ended without them would be a shape the product never produces.
func aCtrlHeldClickInTheGame(guarded bool) []recorder.RecordedEvent {
	press := func(x, y, rx, ry, at int) []recorder.RecordedEvent {
		down := recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left",
			Down: true, X: x, Y: y, App: "arkascended", PID: 4812, T: when(at)}
		if guarded {
			down.WinRel, down.RelX, down.RelY = true, rx, ry
			down.RelTo, down.WinW, down.WinH = "window", 1920, 1080
			down.Dpi, down.Process = 144, "arkascended.exe"
		}
		return []recorder.RecordedEvent{down,
			{Kind: recorder.EvClick, Button: "left", Down: false, X: x, Y: y,
				App: "arkascended", PID: 4812, T: when(at + 40)}}
	}
	mod := func(down bool, at int) recorder.RecordedEvent {
		return recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: "ctrl", Down: down,
			App: "arkascended", PID: 4812, T: when(at)}
	}
	s := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: when(0)},
		mod(true, 100),
	}
	s = append(s, press(900, 300, 100, 200, 400)...)
	s = append(s, mod(false, 800))
	s = append(s,
		recorder.RecordedEvent{Kind: recorder.EvMove, X: 1500, Y: 900, T: when(1400)},
		recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left", Down: true,
			X: 1511, Y: 913, App: "studio", PID: 7001, T: when(1500)},
		recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left", Down: false,
			X: 1511, Y: 913, App: "studio", PID: 7001, T: when(1505)})
	return s
}

// aSavedCtrlClick is the demonstration saved, with the shape asserted before anything is run.
//
// THE SHAPE IS A PRECONDITION AND NOT A FINDING. A demonstration that came out as a plain click —
// which is exactly what it used to do — would make every assertion below true of something that
// is not a Ctrl+click, and the test would pass. So the hold is checked for here, by the words the
// generator writes, and so is the chain link.
func aSavedCtrlClick(t *testing.T, guarded bool) string {
	t.Helper()
	scope := orchestrator.ScopeAppAnywhere
	if !guarded {
		scope = orchestrator.ScopeApp
	}
	src, at := savedFrom(t, scope, aCtrlHeldClickInTheGame(guarded))
	for _, want := range []string{`do OS's KeyDown with "ctrl"`, `do OS's KeyUp with "ctrl"`} {
		if !strings.Contains(src, want) {
			t.Fatalf("the demonstrated Ctrl+click did not come out as a hold: no %s "+
				"in\n%s", want, src)
		}
	}
	// AND THE CLICK IS INSIDE IT, which is the whole gesture.
	down := strings.Index(src, `do OS's KeyDown with "ctrl"`)
	up := strings.Index(src, `do OS's KeyUp with "ctrl"`)
	if mid := strings.Index(src[down:up], "do OS's Click"); mid < 0 {
		t.Fatalf("the saved routine holds Ctrl and clicks nothing while it is down:"+
			"\n%s", src)
	}
	// AND NOT AS A CHORD. `ctrl+` in a Key step would mean the modifier was folded into
	// something, and there is nothing here for it to fold into — a chord would press and
	// release Ctrl itself and the click would land outside it.
	if strings.Contains(src, `do OS's Key with "ctrl+`) {
		t.Fatalf("the demonstration came out as a chord rather than a hold:\n%s", src)
	}
	if guarded {
		for _, want := range []string{`do OS's KeyDown with "ctrl"...`,
			`do OS's KeyUp with "ctrl"...`} {
			if !strings.Contains(src, want) {
				t.Errorf("the hold is not a link that can stop the routine: no "+
					"%s in\n%s", want, src)
			}
		}
	}
	return at
}

// A CTRL+CLICK REACHES THE MACHINE AS A HELD MODIFIER AND A CLICK INSIDE IT.
func TestADemonstratedCtrlClickIsPerformedAsOne(t *testing.T) {
	at := aSavedCtrlClick(t, true)
	rec := &recBackend{}
	h, out := playedBy(t, at, rec)

	if got := strings.Join(rec.keyDowns, ",") + "/" + strings.Join(rec.keyUps, ","); got !=
		"ctrl/ctrl" {
		t.Errorf("the machine was asked for %q, want Ctrl held and released once\n%s",
			got, out)
	}
	if len(rec.clicks) != 1 {
		t.Errorf("%d click(s) were sent, want the one inside the hold\n%s",
			len(rec.clicks), out)
	}
	// AND NO CHORD WAS SENT. `keys` is what `OS's Key` asks for; a chord here would mean
	// the modifier was pressed and released by something other than the hold.
	if len(rec.keys) != 0 {
		t.Errorf("the machine was asked for key presses %v as well as the hold, so the "+
			"modifier was sent twice\n%s", rec.keys, out)
	}
	if undone := h.Unperformed(); len(undone) != 0 {
		t.Errorf("a run where everything happened reports %v", undone)
	}
	if warn := h.NotReleased(); warn != "" {
		t.Errorf("a run that released its modifier reports %q", warn)
	}
}

// AND A MODIFIER THE MACHINE WILL NOT PRESS STOPS THE CLICK.
//
// The click is the point of the gesture, and a click with Ctrl held is a different act from the
// same click without it. Nothing in the window guard can tell, because it is the same window and
// the same pixel — so the hold failing has to stop the routine, exactly as a keystroke that was
// not delivered does.
func TestACtrlThatWillNotGoDownStopsTheClick(t *testing.T) {
	at := aSavedCtrlClick(t, true)
	rec := &recBackend{refuseHolds: map[string]bool{"ctrl": true}}
	h, out := playedBy(t, at, rec)

	if len(rec.clicks) != 0 {
		t.Errorf("%d click(s) were sent after Ctrl failed to go down, so the routine "+
			"performed a plain click where a Ctrl+click was demonstrated\n%s",
			len(rec.clicks), out)
	}
	undone := h.Unperformed()
	if len(undone) != 1 {
		t.Fatalf("the run reports %d thing(s) that did not happen: %v — a command "+
			"reading this would report success\n%s", len(undone), undone, out)
	}
	if !strings.Contains(undone[0], "ctrl hold") {
		t.Errorf("the account says %q and does not name the modifier", undone[0])
	}
}

// AND A MODIFIER THE MACHINE WILL NOT RELEASE IS REPORTED BY NAME.
//
// Worse for a modifier than for a letter. A stuck Q repeats itself into whatever has the
// keyboard; a stuck Ctrl means every keystroke anybody types on the machine arrives as a shortcut
// until somebody works out why. The run cannot put it back up — that is the standing limit — so
// what it must do is say so.
func TestACtrlThatWillNotComeUpIsNamedInTheAccount(t *testing.T) {
	at := aSavedCtrlClick(t, true)
	rec := &recBackend{refuseReleases: map[string]int{"ctrl": -1}}
	h, out := playedBy(t, at, rec)

	// THE CLICK HAPPENED, which is what makes this about the release.
	if len(rec.clicks) != 1 {
		t.Errorf("%d click(s) were sent; the click inside the hold should have "+
			"happened\n%s", len(rec.clicks), out)
	}
	undone := h.Unperformed()
	if len(undone) != 1 || !strings.Contains(undone[0], "ctrl release") {
		t.Errorf("the run reports %v and does not name the release that failed\n%s",
			undone, out)
	}
	if warn := h.NotReleased(); !strings.Contains(warn, "ctrl") {
		t.Errorf("the cleanup account says %q and does not name the modifier that may "+
			"still be down\n%s", warn, out)
	}
}

// AND A LEGACY ROUTINE KEEPS THE CONTRACT IT WAS SAVED UNDER.
//
// A route with no window facts on its press attempts its input and carries on, which is what it
// was saved under, and the account is recorded for both while being acted on for one. Here to
// keep the correction confined: nothing about reading a modifier as a hold may change what an
// existing routine does.
func TestALegacyCtrlClickWhoseModifierFailedCarriesOn(t *testing.T) {
	at := aSavedCtrlClick(t, false)
	rec := &recBackend{refuseHolds: map[string]bool{"ctrl": true}}
	h, out := playedBy(t, at, rec)

	if len(rec.clicks) != 1 {
		t.Errorf("%d click(s) were sent; a legacy route attempts its input and carries "+
			"on\n%s", len(rec.clicks), out)
	}
	if undone := h.Unperformed(); len(undone) != 1 {
		t.Errorf("a legacy run whose modifier hold failed reports %v", undone)
	}
}

// AND A RUN CANCELLED WITH CTRL DOWN STILL ATTEMPTS THE RELEASE.
//
// The modifier gets into the sweep's list by the new classification path and by nothing else, so
// this is an integration claim rather than a second test of `ReleaseHeld`: a modifier that the
// simplifier now turns into a hold has to be a key the Host is TRACKING, or a routine stopped
// between the press and its release leaves Ctrl down on somebody's desktop with nothing to say
// so.
//
// Driven through the route, not by calling the primitives: what is being checked is that the real
// run — generated from a real demonstration — hands the modifier to the thing that sweeps.
func TestARunCancelledHoldingCtrlStillLetsGo(t *testing.T) {
	at := aSavedCtrlClick(t, true)
	rec := &recBackend{}
	h, out := playedBy(t, at, rec)

	// THE HOLD IS NO LONGER TRACKED once its own release has run, which is what makes a
	// second sweep a no-op rather than a second release of a key nobody is holding.
	before := len(rec.keyUps)
	h.ReleaseHeld()
	if len(rec.keyUps) != before {
		t.Errorf("the sweep released %v again after the routine had already let go\n%s",
			rec.keyUps, out)
	}

	// AND A RUN THAT STOPPED BEFORE ITS RELEASE LEAVES THE SWEEP SOMETHING TO DO.
	//
	// Built on the production Host rather than the route, because stopping a run exactly
	// between the hold and its release is not a thing a route can be asked for: the
	// routine is what decides when the release happens. What this establishes is the half
	// that matters — a modifier pressed as a hold is in the sweep's list, in the order it
	// went down, and the sweep attempts it.
	stopped := &recBackend{}
	hs := &Host{b: stopped}
	ctx, cancel := context.WithCancel(context.Background())
	if status := holding(t, hs, ctx, "ctrl"); status != "ok" {
		t.Fatalf("the machine would not hold ctrl down: %s", status)
	}
	cancel()
	hs.ReleaseHeld()
	if got := strings.Join(stopped.keyUps, ","); got != "ctrl" {
		t.Errorf("the sweep of a cancelled run released %q, want the modifier it was "+
			"holding", got)
	}
	if warn := hs.NotReleased(); warn != "" {
		t.Errorf("a sweep that released the modifier reports %q", warn)
	}
}
