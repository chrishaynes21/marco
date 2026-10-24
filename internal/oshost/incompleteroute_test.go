package oshost

// CAPTURE → SAVE → RELOAD → PLAY, WHEN A MEASUREMENT FAILED.
//
// The chain this file exists for is the one nothing else walks end to end. Each link was tested
// next door and the question is about the JOIN: a recording whose window could not be fully
// measured goes through the production simplifier and the production generator, onto disk as
// legal Marco, back off disk through the production compiler, and into the production host.
//
// # What used to happen here, and why it needed a test of its own
//
// A press whose program had exited before the hooks came down, or whose window would not report
// its size or its scale, was written out as a bare absolute point. Byte for byte a route from
// before any of this existed — so the host saw a legacy point, pressed the coordinate it was
// demonstrated at, and checked nothing. The guard was present, correct, and never consulted.
// Nothing between the recorder and the host could have noticed: every part of the chain was
// doing what it was told.
//
// So the claims are: the saved route SAYS it is about a window, it does not say what the window
// was, and running it sends NO INPUT AT ALL and falls back to nothing.
//
// The recorder and the desktop are stood in for; everything between them is production.

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// aHeldRecorder publishes its events and keeps the stream open, the way a real one does while
// somebody is still demonstrating.
type aHeldRecorder struct {
	events []recorder.RecordedEvent
	ch     chan recorder.RecordedEvent
}

func (h *aHeldRecorder) Start() error {
	h.ch = make(chan recorder.RecordedEvent, len(h.events)+1)
	for _, e := range h.events {
		h.ch <- e
	}
	return nil
}
func (h *aHeldRecorder) Stop() []recorder.RecordedEvent        { return h.events }
func (h *aHeldRecorder) Events() <-chan recorder.RecordedEvent { return h.ch }

// aPressThatFailedTo is one click in the game and then the trip to Marco's window to press Stop,
// with `lose` applied to the game's press — which is what a failed measurement leaves behind.
func aPressThatFailedTo(lose func(*recorder.RecordedEvent)) []recorder.RecordedEvent {
	press := recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left", Down: true,
		X: 842, Y: 274, App: "arkascended", PID: 4812, T: when(100),
		// EVERYTHING MEASURED, and then one thing taken away.
		WinRel: true, RelX: 722, RelY: 210, RelTo: "window",
		WinW: 1920, WinH: 1080, Dpi: 144, Process: "arkascended.exe"}
	lose(&press)
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: when(0)},
		press,
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: 4812, T: when(105)},
		// ── AND NOW THEY GO AND PRESS STOP ──
		{Kind: recorder.EvAppSwitch, KeyName: "studio", T: when(3000)},
		{Kind: recorder.EvMove, X: 1500, Y: 900, T: when(3100)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: "studio", PID: 7001, T: when(3200)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: "studio", PID: 7001, T: when(3205)},
	}
}

func when(ms int) time.Time {
	return time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC).
		Add(time.Duration(ms) * time.Millisecond)
}

// savedFrom records the events, saves the draft through the production path under `scope`, and
// returns the saved route's source and its file.
func savedFrom(t *testing.T, scope orchestrator.Scope, events []recorder.RecordedEvent) (string, string) {
	t.Helper()
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	press := make(chan struct{})
	d := orchestrator.Deps{
		Reg: reg, Rec: &aHeldRecorder{events: events}, Out: &strings.Builder{},
		Owner: "studio", OwnerPID: 7001, StopWhen: press,
	}
	go func() { time.Sleep(10 * time.Millisecond); close(press) }()
	dr, err := d.Record("ark graphics")
	if err != nil {
		t.Fatalf("recording: %v", err)
	}
	rt, err := d.Save(dr, scope, "arkascended")
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	at := reg.Path(rt)
	src, err := os.ReadFile(at)
	if err != nil {
		t.Fatalf("reading back what was saved: %v", err)
	}
	return string(src), at
}

// played loads the saved route from disk and runs it against a host whose desktop agrees with
// everything and whose input is counted. It returns the input that was sent and everything the
// run said, the route's own output and the host's log together.
//
// # Why the log is part of the answer
//
// Because a refused press does not fail the routine. `codegen` emits a bare `do OS's Click with
// p1.`, and a `do` whose status nobody reads does not abort the interaction — so the route runs
// to `this is ok!` and the play reports done having pressed nothing. That is the pre-existing
// behaviour of every failing host call in a generated route and it is not this increment's to
// change, but it is why the refusal has to be looked for where the refusal actually goes.
func played(t *testing.T, at string) (*recBackend, string) {
	t.Helper()
	onADesktopLike(t, steady(sameWindowMoved), "arkascended.exe")
	wasBind, wasLog := bindWindow, mlog.Default
	t.Cleanup(func() { bindWindow, mlog.Default = wasBind, wasLog })
	// ACTIVATE SUCCEEDS AND BINDS THE GAME'S WINDOW, so a press is refused for the
	// reason a test is about and not because nothing was ever prepared.
	bindWindow = func(string) (uintptr, error) { return theGameWindow, nil }
	var said bytes.Buffer
	mlog.Default = mlog.New(&said, mlog.LevelDebug)

	rec := &recBackend{}
	var out bytes.Buffer
	err := driver.RunFileWithHosts(at, &out, map[string]runtime.Host{"*": &Host{b: rec}})
	if err != nil {
		t.Fatalf("the saved route would not run at all: %v\n%s", err, out.String())
	}
	return rec, out.String() + "\n" + said.String()
}

// A SAVED RECORDING WITH A FAILED MEASUREMENT PRESSES NOTHING, AND FALLS BACK TO NOTHING.
//
// One case per measurement that can fail, because they fail for different reasons on real
// desktops and each one has to reach the same answer:
//
//   - the program could not be named — it had exited by the time the hooks came down, and the
//     lookup runs AFTER the capture, so it had nothing left to open
//   - the scale could not be read — GetDpiForWindow arrived in Windows 10 1607
//   - the window would not report its size
//   - the press could not be placed inside the window — the attribution named the window that
//     was in front at that instant and the click landed in the one it was about to raise
func TestASavedRecordingThatCouldNotBeMeasuredPressesNothing(t *testing.T) {
	for _, c := range []struct {
		what string
		lose func(*recorder.RecordedEvent)
		gone string
		says string
	}{
		{"the program had gone before the lookup ran",
			func(e *recorder.RecordedEvent) { e.Process = "" },
			"Process", "which program owned it"},
		{"this Windows cannot report a window's scale",
			func(e *recorder.RecordedEvent) { e.Dpi = 0 },
			"Dpi", "what scale it was drawn at"},
		{"the window would not say how big it is",
			func(e *recorder.RecordedEvent) { e.WinW, e.WinH = 0, 0 },
			"WinW", "how wide that window was"},
		{"the press could not be placed inside the window",
			func(e *recorder.RecordedEvent) {
				e.WinRel, e.RelX, e.RelY = false, 0, 0
			},
			"RelX", "where across that window the press went"},
	} {
		t.Run(c.what, func(t *testing.T) {
			src, at := savedFrom(t, orchestrator.ScopeAppAnywhere, aPressThatFailedTo(c.lose))

			// ── THE SAVED ROUTE SAYS IT IS ABOUT A WINDOW ──
			//
			// This is the whole of it. Without this line the route is a
			// legacy point and the host is right to press it.
			if !strings.Contains(src, `RelTo "window"`) {
				t.Fatalf("the saved route does not say it is about a "+
					"window, so it is indistinguishable from one "+
					"recorded before any of this and will be pressed "+
					"unchecked:\n%s", src)
			}
			// AND IT DOES NOT SAY WHAT IT NEVER MEASURED.
			if strings.Contains(src, c.gone) {
				t.Errorf("%q is in the saved route although nothing "+
					"measured it:\n%s", c.gone, src)
			}

			// ── AND PLAYING IT SENDS NOTHING ──
			rec, out := played(t, at)
			if len(rec.moves) != 0 || len(rec.clicks) != 0 {
				t.Errorf("the route sent input: moves=%v clicks=%v\n%s",
					rec.moves, rec.clicks, out)
			}
			// AND SPECIFICALLY NOT THE COORDINATE IT WAS DEMONSTRATED AT,
			// which is what the legacy fallback would have pressed.
			for _, m := range rec.moves {
				if m == [2]int{842, 274} {
					t.Error("the press fell back to its recorded " +
						"desktop coordinate")
				}
			}
			if !strings.Contains(out, c.says) {
				t.Errorf("the run does not say which measurement is "+
					"missing (%q):\n%s", c.says, out)
			}
		})
	}
}

// AND THE SAME RECORDING WITH EVERY MEASUREMENT PRESSES WHERE IT WAS DEMONSTRATED.
//
// The control. Without it the test above passes on a chain that refuses everything — including a
// recording that measured the lot, which is the one that has to work.
func TestASavedRecordingThatWasMeasuredPressesInsideTheWindow(t *testing.T) {
	_, at := savedFrom(t, orchestrator.ScopeAppAnywhere,
		aPressThatFailedTo(func(*recorder.RecordedEvent) {}))

	rec, out := played(t, at)
	want := [2]int{sameWindowMoved.Left + 722, sameWindowMoved.Top + 210}
	if len(rec.moves) != 1 || rec.moves[0] != want {
		t.Fatalf("pressed at %v, want %v — the window's origin plus the "+
			"demonstrated offset\n%s", rec.moves, want, out)
	}
}

// AND A CONTEXT-SCOPED RECORDING REFUSES EVERY PRESS, BECAUSE NOTHING IN IT BINDS A WINDOW.
//
// Found while wiring the chain above, and not caused by it. `Save` under the "app" scope —
// Studio's default in the review panel — strips the leading Activate and passes no application
// to `codegen`, because that scope's whole promise is that it switches nothing: it runs only
// while its application is already in front. So the saved route contains no `do OS's Activate`,
// nothing binds a window, and a press that declared the window it was made in has no window to
// be about.
//
// It is pinned rather than fixed. The resolution is a decision about what PREPARATION means for
// a scope with no preparation step — either such a routine binds the foreground once at the
// start of its run, which is that scope's own premise and would make the first press checked
// against whatever is in front, or recordings whose presses declared a window must be saved as
// "anywhere, bringing the application forward". Both change the guarded contract, which is not
// this increment's to change; what this test does is make sure the state is deliberate and is
// found by a suite rather than by somebody's live trial.
//
// The same applies to the "everywhere" scope, for the same reason and the same line of `Save`.
func TestAContextScopedRecordingHasNoWindowToCheckAgainst(t *testing.T) {
	for _, scope := range []orchestrator.Scope{
		orchestrator.ScopeApp, orchestrator.ScopeEverywhere,
	} {
		t.Run(string(scope), func(t *testing.T) {
			src, at := savedFrom(t, scope,
				aPressThatFailedTo(func(*recorder.RecordedEvent) {}))

			if strings.Contains(src, "OS's Activate") {
				t.Fatalf("this scope brings a window forward after all, so "+
					"this test is about nothing:\n%s", src)
			}
			rec, out := played(t, at)
			if len(rec.moves) != 0 || len(rec.clicks) != 0 {
				t.Errorf("input was sent with no window bound: moves=%v "+
					"clicks=%v\n%s", rec.moves, rec.clicks, out)
			}
			if !strings.Contains(out, "nothing to measure it against") {
				t.Errorf("the refusal does not say that nothing was "+
					"brought to the front:\n%s", out)
			}
		})
	}
}
