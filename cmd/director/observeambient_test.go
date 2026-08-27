package main

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// Marco paying attention, and the four things that must stay true while it does.

// watchingRuntime is a Director with a desktop it can watch and nothing it can act on.
func watchingRuntime(t *testing.T) *Runtime {
	t.Helper()
	g, store := watchedRegistry(t)
	_ = store
	rt := &Runtime{observations: g}

	// The desktop, replaced: a real one would make this test depend on whatever window
	// happened to be in front of the developer.
	app, sel := winctxActive, currentWindowSelector
	winctxActive = func() string { return "" } // nothing in front: no session is started
	currentWindowSelector = func(a string) windowref.Selector {
		return windowref.Selector{Application: a}
	}
	t.Cleanup(func() { winctxActive, currentWindowSelector = app, sel })
	t.Cleanup(func() { rt.DisableAmbient() })
	return rt
}

// WATCHING TWICE IS STILL WATCHING ONCE.
//
// `marco observe` is the sort of thing somebody types twice — once because they meant to, and
// once because they were not sure the first one took. A second supervisor would be a second
// observer by another name, and it would compete with the first for the one substrate.
//
// Deleting the already-on check must fail this.
func TestWatchingTwiceIsStillWatchingOnce(t *testing.T) {
	rt := watchingRuntime(t)

	first := rt.EnableAmbient()
	if !first.Watching {
		t.Fatal("watching did not start")
	}
	supervisor := rt.watching

	second := rt.EnableAmbient()
	if !second.Watching {
		t.Fatal("the second request reported not watching")
	}
	if rt.watching != supervisor {
		t.Fatal("a second `observe` built a second supervisor")
	}
	// AND ONE LOOP, which is the claim that matters. The supervisor OBJECT being shared is
	// guaranteed by a sync.Once and proves nothing about how many goroutines are sampling:
	// the first version of this test asserted only the object, and a mutation that started a
	// second loop against the same object survived it.
	if got := supervisor.running(); got != 1 {
		t.Fatalf("%d supervisor loops are running after three requests to watch. Two of "+
			"them would compete for the one observation substrate, and neither could "+
			"attribute what it saw.", got)
	}
	third := rt.EnableAmbient()
	if !third.Watching {
		t.Fatal("the third request reported not watching")
	}

	// AND STOPPING IS ONE STOP. Not one per request.
	if off := rt.DisableAmbient(); off.Watching {
		t.Fatal("it is still watching after being told to stop")
	}
	if off := rt.DisableAmbient(); off.Watching {
		t.Fatal("stopping something already stopped reported watching")
	}
}

// WATCHING WAITS FOR WHOEVER ELSE IS LOOKING.
//
// One observation session runs at a time — the registry refuses a second, because two would
// contend for the screen. Learn, Here and a performance's verification all own the substrate
// ahead of ambient watching, and ambient is the one that gives way.
//
// The proof is that a session belonging to somebody else is STILL THEIRS after watching has been
// running: if the supervisor competed, it would either have failed to start (and given up) or
// displaced them.
func TestWatchingWaitsForWhoeverElseIsLooking(t *testing.T) {
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}

	app, sel := winctxActive, currentWindowSelector
	winctxActive = func() string { return "settings" }
	currentWindowSelector = func(a string) windowref.Selector {
		return windowref.Selector{Application: a}
	}
	t.Cleanup(func() { winctxActive, currentWindowSelector = app, sel })

	// SOMEBODY ELSE IS LOOKING — a Learn session, a Here, a verification. The registry
	// does not care which; it cares that one is running.
	watchNow(t, g, store, "settings")

	// STOPPED BEFORE THE OTHER SESSION IS, and the order is not arrangeable: cleanups run
	// last-registered-first, `watchNow` waits for ITS session to retire, and a supervisor
	// still running would start its own the moment the registry went quiet — so the wait
	// would see something active and time out. Registered after `watchNow` so it runs
	// before it.
	t.Cleanup(func() { rt.DisableAmbient() })
	theirs := g.ActiveID()
	if theirs == "" {
		t.Fatal("nothing is watching; this test is about giving way to somebody who is")
	}

	rt.EnableAmbient()
	// Long enough for the supervisor to have tried several times.
	time.Sleep(4 * ambientYield)

	if got := g.ActiveID(); got != theirs {
		t.Fatalf("the running session is now %q and was %q. Ambient watching displaced "+
			"another consumer of the one substrate instead of waiting for it.",
			got, theirs)
	}
	if !rt.AmbientStatus().Watching {
		t.Error("ambient watching gave up entirely rather than waiting; it must resume " +
			"when the substrate is free")
	}
}

// SHUTTING DOWN STOPS WATCHING.
//
// A supervisor goroutine that outlived its Runtime would go on sampling a desktop for a home
// nobody owns any more — and since ADR-092 the home's claim is released the moment the process
// ends, so the next Director could be watching the same screen while this one's orphan still was.
func TestShuttingDownStopsWatching(t *testing.T) {
	rt := watchingRuntime(t)
	rt.EnableAmbient()
	if !rt.AmbientStatus().Watching {
		t.Fatal("watching did not start")
	}
	rt.Close()
	if rt.AmbientStatus().Watching {
		t.Fatal("the Runtime closed and ambient watching is still running")
	}
}

// WATCHING KEEPS NOTHING ABOUT A SCREEN IT CANNOT READ, AND NAMES NOTHING IT DOES NOT KNOW.
//
// Two refusals in one place, because they are the two ways a reading can fail to be a Place and
// both would corrupt the buffer:
//
//	degraded perception   the window is there and its content is not being read (ADR-090).
//	                      Inventing a Place from that would put the frame every page of an
//	                      application shares into the buffer as a screen.
//	unknown screen        read perfectly well, and not one Marco knows. Ambient watching holds
//	                      no licence to establish it, and asking somebody to name it would turn
//	                      paying attention into an interactive acquisition episode.
//
// Held at `sample`, which is where the decision is, because the supervisor loop cannot be driven
// deterministically from a test without a desktop.
func TestWatchingRecordsNeitherDegradedNorUnknownScreens(t *testing.T) {
	rt := watchingRuntime(t)
	a := rt.ambient()

	// A reading that got no further than the window frame.
	before, _, _ := a.buf.Size()
	a.recordPlace("settings", observe.Place{Placed: true, Reach: observe.ReachShell},
		time.Now())
	if got, _, _ := a.buf.Size(); got != before {
		t.Error("a window whose content could not be read became a Place in the buffer")
	}
	if !rt.AmbientStatus().PerceptionDegraded {
		t.Error("the degraded reading was not reported; a person cannot tell an " +
			"unreadable window from a quiet one")
	}

	// A screen read perfectly well and not recognised.
	a.recordPlace("settings", observe.Place{Placed: true, Reach: observe.ReachContent},
		time.Now())
	if got, _, _ := a.buf.Size(); got != before {
		t.Error("an unrecognised screen was recorded as a Place. Ambient watching holds no " +
			"licence to establish anything, and this buffer keys on durable ids.")
	}

	// AND A SCREEN IT KNOWS IS RECORDED. Without this the assertions above pass for a
	// function that records nothing at all.
	a.recordPlace("settings", observe.Place{
		Placed: true, Reach: observe.ReachContent, Subject: "subj_home",
		Verdict: observe.MatchSame,
	}, time.Now())
	if got, _, _ := a.buf.Size(); got != before+1 {
		t.Fatal("a screen Marco knows was not recorded, so watching notices nothing at all")
	}
}

// ATTENTION FALLS WHILE NOTHING HAPPENS, AND SNAPS BACK WHEN SOMETHING DOES.
//
// # Why the asymmetry is the point
//
// Something that read the screen at full rate forever would be a background process a person can
// feel, and "affordable enough to leave on" is the whole product claim. So attention decays while
// the desktop is unchanged.
//
// It returns to busy IMMEDIATELY, not gradually. A gentle ramp would mean the first thing somebody
// did after a quiet afternoon was the thing Marco was slowest to notice — exactly backwards,
// because a change after a long pause is the most informative event there is.
func TestAttentionFallsWhenNothingHappensAndSnapsBackWhenItDoes(t *testing.T) {
	// Nothing happening, repeatedly.
	at := ambientBusy
	for i := 0; i < 10; i++ {
		at = nextAttention(at, false)
	}
	if at != ambientIdle {
		t.Errorf("attention settled at %v after ten quiet rounds, want %v — something "+
			"that never backs off is a background process a person can feel", at,
			ambientIdle)
	}
	if at <= ambientBusy {
		t.Fatal("attention never fell at all")
	}

	// AND THE MOMENT SOMETHING DOES.
	if got := nextAttention(at, true); got != ambientBusy {
		t.Fatalf("after a change, attention is %v and want %v. A gradual return makes the "+
			"first thing somebody does after a pause the thing Marco notices slowest.",
			got, ambientBusy)
	}
}

// A SCREEN MARCO DOES NOT KNOW DOES NOT BREAK THE WALK THROUGH IT.
//
// # The requirement this holds
//
// Real navigation passes through frames that are neither endpoint: a page part-way through
// arriving, a screen Marco has never seen. Ambient watching records none of them — it holds no
// licence to establish anything — and it must still notice that somebody went from Home to
// Bluetooth & devices.
//
// So an unrecognised reading is SKIPPED rather than treated as a place Marco moved to. The
// difference is in one line, and without it the previous place is forgotten and the transition
// across the gap is never seen. Measured: the mutation that removes it survives every other test
// in this file.
func TestAnUnknownScreenDoesNotBreakTheWalkThroughIt(t *testing.T) {
	rt := watchingRuntime(t)
	a := rt.ambient()
	now := time.Now()

	known := func(id string) observe.Place {
		return observe.Place{Placed: true, Reach: observe.ReachContent,
			Subject: id, Verdict: observe.MatchSame}
	}
	unknown := observe.Place{Placed: true, Reach: observe.ReachContent}

	a.recordPlace("settings", known("subj_home"), now)
	a.recordPlace("settings", unknown, now.Add(time.Second))
	a.recordPlace("settings", known("subj_bt"), now.Add(2*time.Second))

	view := a.buf.Look()
	if len(view.Edges) != 1 {
		t.Fatalf("%d transitions recorded across an unrecognised frame, want one.\n"+
			"Marco went from Home to Bluetooth & devices through a screen it does "+
			"not know, and that is what ordinary navigation looks like.", len(view.Edges))
	}
	if view.Edges[0].From != "subj_home" || view.Edges[0].To != "subj_bt" {
		t.Errorf("the transition reads %s -> %s", view.Edges[0].From, view.Edges[0].To)
	}

	// AND THE UNKNOWN SCREEN IS STILL NOT A PLACE.
	for _, p := range view.Places {
		if p.Subject == "" {
			t.Error("the unrecognised screen was recorded as a Place")
		}
	}
	if len(view.Places) != 2 {
		t.Errorf("%d places recorded, want the two Marco knows", len(view.Places))
	}
}

// AND WHAT WATCHING RECORDS IS THE HUMAN'S DOING.
//
// Ambient watching is what somebody using their own computer looks like. A performance's
// transitions are recorded by the walk that made them, and conflating the two would eventually
// teach Marco its own behaviour back from itself — and tell somebody in Activity that they did
// something Marco did.
func TestWatchingRecordsWhatThePersonDid(t *testing.T) {
	rt := watchingRuntime(t)
	a := rt.ambient()
	now := time.Now()

	known := func(id string) observe.Place {
		return observe.Place{Placed: true, Reach: observe.ReachContent,
			Subject: id, Verdict: observe.MatchSame}
	}
	a.recordPlace("settings", known("subj_home"), now)
	a.recordPlace("settings", known("subj_bt"), now.Add(time.Second))

	edges := a.buf.Look().Edges
	if len(edges) != 1 {
		t.Fatalf("%d transitions", len(edges))
	}
	if edges[0].By[ambient.ByHuman] != 1 {
		t.Fatalf("the transition is attributed %v, and ambient watching is what a person "+
			"doing their own work looks like", edges[0].By)
	}
	if edges[0].By[ambient.ByMarco] != 0 {
		t.Error("watching attributed somebody's own navigation to Marco")
	}
}
