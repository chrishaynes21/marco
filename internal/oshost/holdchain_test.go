package oshost

// A HOLD THAT DID NOT HAPPEN HAS TO STOP WHAT COMES AFTER IT.
//
// `hold Q, click, release Q` is ONE GESTURE. The click is a different gesture without the Q — a
// modified click is not a click — and the window guard cannot tell the difference, because the
// window is the same window. So a hold that the machine would not take must stop the routine, for
// exactly the reason a keystroke that was not delivered does: what follows was demonstrated
// against a screen that the hold was supposed to change the meaning of.
//
// The release is the half that looks like it does not matter. It does: everything after a release
// the machine refused runs with the key STILL DOWN, so the clicks that follow are modified clicks
// and the typing that follows is different typing. The route-end sweep may recover the key
// afterwards, and that is not the same as the routine having been right to carry on.
//
// Two defects, found in review of the hold slice:
//
//   - the GENERATOR propagated `Key` and not `KeyDown` or `KeyUp`, so a guarded routine's hold
//     was written as an ordinary step and its failure stopped nothing.
//   - the HOST reported a failed hold or release as a failed step and recorded it NOWHERE, so the
//     command read an empty account and reported success about a routine that ran with a key in
//     the wrong state.
//
// Both halves are needed and neither is sufficient: a propagated step whose failure is not in the
// account lets the command report success, and an account without propagation lets the routine go
// on pressing. So every case here goes through generation, the runtime and the production host.
//
// See [[ADR-258-a-chord-is-a-promise-to-let-go]] and
// [[ADR-251-a-press-checks-the-window-it-was-recorded-in]].

import (
	"bytes"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// aHoldAcrossAPressInTheGame is the demonstration: hold Q, press something, release Q, press
// something else.
//
// The second press is the point of the shape. Without something after the RELEASE there is
// nothing for a refused release to wrongly allow, and the case would only ever be about the hold.
//
// `simplify` makes this a hold rather than two keystrokes because the key stays down across
// another action — that is its rule, and depending on it silently is how it gets changed.
func aHoldAcrossAPressInTheGame(guarded bool) []recorder.RecordedEvent {
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
	key := func(down bool, at int) recorder.RecordedEvent {
		return recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: "q", Down: down,
			App: "arkascended", PID: 4812, T: when(at)}
	}
	s := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: when(0)},
		key(true, 100),
	}
	s = append(s, press(900, 300, 100, 200, 300)...)
	s = append(s, key(false, 600))
	s = append(s, press(950, 350, 150, 250, 900)...)
	s = append(s,
		recorder.RecordedEvent{Kind: recorder.EvMove, X: 1500, Y: 900, T: when(1400)},
		recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left", Down: true,
			X: 1511, Y: 913, App: "studio", PID: 7001, T: when(1500)},
		recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left", Down: false,
			X: 1511, Y: 913, App: "studio", PID: 7001, T: when(1505)})
	return s
}

// playedBy performs a saved routine against the production host with the machine arranged by the
// caller.
func playedBy(t *testing.T, at string, rec *recBackend) (*Host, string) {
	t.Helper()
	onADesktopLike(t, steady(sameWindowMoved), "arkascended.exe")
	wasBind, wasLog := bindWindow, mlog.Default
	t.Cleanup(func() { bindWindow, mlog.Default = wasBind, wasLog })
	bindWindow = func(string) (uintptr, error) { return theGameWindow, nil }
	var said bytes.Buffer
	mlog.Default = mlog.New(&said, mlog.LevelDebug)

	h := &Host{b: rec}
	var out bytes.Buffer
	if err := driver.RunFileWithHosts(at, &out, map[string]runtime.Host{"*": h}); err != nil {
		t.Fatalf("the saved route would not run at all: %v\n%s", err, out.String())
	}
	return h, out.String() + "\n" + said.String()
}

// aSavedHold is the demonstration saved, with the shape it has to have asserted before anything
// is run against it.
func aSavedHold(t *testing.T, guarded bool) string {
	t.Helper()
	scope := orchestrator.ScopeAppAnywhere
	if !guarded {
		scope = orchestrator.ScopeApp
	}
	src, at := savedFrom(t, scope, aHoldAcrossAPressInTheGame(guarded))
	// THE HOLD IS IN THE SAVED ROUTINE AT ALL. `simplify` is what turns a key held across
	// another action into a hold, and a demonstration that came out as two keystrokes
	// would make every assertion below true of something that is not a hold.
	for _, want := range []string{`do OS's KeyDown with "q"`, `do OS's KeyUp with "q"`} {
		if !strings.Contains(src, want) {
			t.Fatalf("the demonstration did not come out as a hold: no %s in\n%s",
				want, src)
		}
	}
	if guarded {
		// AND IT IS A LINK IN THE CHAIN, which is the generator half of the fix.
		for _, want := range []string{`do OS's KeyDown with "q"...`,
			`do OS's KeyUp with "q"...`} {
			if !strings.Contains(src, want) {
				t.Errorf("the hold is not a link that can stop the routine: no "+
					"%s in\n%s", want, src)
			}
		}
	}
	return at
}

// A HOLD THE MACHINE WILL NOT TAKE STOPS THE ROUTINE, AND THE RUN SAYS WHY.
func TestAHoldTheMachineWillNotTakeStopsTheRoutine(t *testing.T) {
	at := aSavedHold(t, true)
	rec := &recBackend{refuseHolds: map[string]bool{"q": true}}
	h, out := playedBy(t, at, rec)

	if len(rec.keyDowns) != 1 {
		t.Errorf("the machine was asked for %v holds, want the one it refused\n%s",
			rec.keyDowns, out)
	}
	// NOTHING AFTER IT. The press was demonstrated with Q held, and without the Q it is
	// a different gesture aimed at the same pixel.
	if len(rec.clicks) != 0 {
		t.Errorf("%d press(es) were sent after the hold did not go down\n%s",
			len(rec.clicks), out)
	}
	undone := h.Unperformed()
	if len(undone) != 1 {
		t.Fatalf("the run reports %d thing(s) that did not happen: %v — a command "+
			"reading this would report success\n%s", len(undone), undone, out)
	}
	if !strings.Contains(undone[0], "q hold") {
		t.Errorf("the account says %q and does not name the hold", undone[0])
	}
}

// AND A RELEASE THE MACHINE WILL NOT TAKE STOPS WHAT COMES AFTER IT TOO.
//
// The half that looks harmless. Everything after a release that did not happen runs with the key
// still down, so the press that follows is a modified press — and the route-end sweep recovering
// the key later does not make that press the one that was demonstrated.
func TestAReleaseTheMachineWillNotTakeStopsWhatFollows(t *testing.T) {
	at := aSavedHold(t, true)
	rec := &recBackend{refuseReleases: map[string]int{"q": -1}}
	h, out := playedBy(t, at, rec)

	// THE FIRST PRESS HAPPENED, which is what makes this about the release.
	if len(rec.clicks) != 1 {
		t.Errorf("%d press(es) were sent; the hold and the press inside it should "+
			"have happened and nothing after the release\n%s", len(rec.clicks), out)
	}
	undone := h.Unperformed()
	if len(undone) != 1 {
		t.Fatalf("the run reports %d thing(s) that did not happen: %v\n%s",
			len(undone), undone, out)
	}
	if !strings.Contains(undone[0], "q release") {
		t.Errorf("the account says %q and does not name the release", undone[0])
	}
}

// AND THE SAME ROUTINE DOES ALL OF IT WHEN THE MACHINE TAKES BOTH.
//
// The control. Without it both cases above pass against a routine that refuses everything, or one
// whose hold never reached the host at all.
func TestAHoldTheMachineTakesLetsTheRoutineGoOn(t *testing.T) {
	at := aSavedHold(t, true)
	rec := &recBackend{}
	h, out := playedBy(t, at, rec)

	if len(rec.clicks) != 2 {
		t.Errorf("%d press(es) were sent, want both\n%s", len(rec.clicks), out)
	}
	if got := strings.Join(rec.keyDowns, ",") + "/" + strings.Join(rec.keyUps, ","); got !=
		"q/q" {
		t.Errorf("the machine was asked for %q, want one hold and one release\n%s",
			got, out)
	}
	if undone := h.Unperformed(); len(undone) != 0 {
		t.Errorf("a run where everything happened reports %v", undone)
	}
	if warn := h.NotReleased(); warn != "" {
		t.Errorf("a run that released its hold reports %q", warn)
	}
}

// AND A LEGACY ROUTINE WHOSE HOLD FAILED REPORTS WHAT IT ALWAYS DID.
//
// The same demonstration, the same refusal, the other contract. A route saved before any of this
// was saved under a rule that says the input is attempted and the run carries on — so its presses
// still happen, and the account is recorded for both while being acted on for one. This is the
// test that keeps the change confined to what it was made for.
func TestALegacyRoutineWhoseHoldFailedCarriesOn(t *testing.T) {
	at := aSavedHold(t, false)
	rec := &recBackend{refuseHolds: map[string]bool{"q": true}}
	h, out := playedBy(t, at, rec)

	if len(rec.clicks) != 2 {
		t.Errorf("%d press(es) were sent; a legacy route attempts its input and "+
			"carries on, which is the contract it was saved under\n%s",
			len(rec.clicks), out)
	}
	// AND IT IS STILL RECORDED, because the account is about what happened and not about
	// which contract decided what to do with it.
	if undone := h.Unperformed(); len(undone) != 1 {
		t.Errorf("a legacy run whose hold failed reports %v", undone)
	}
}
