package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"os"
	"path/filepath"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// The rest of what watching has to be true about: that leaving it on all day costs nothing, that
// it is not a thing Marco DID, that the Director says so when asked, and the whole of it end to
// end. See ADR-093.

// knownPlace is a screen Marco recognises, as the observer would read it.
func knownPlace(id string) observe.Place {
	return observe.Place{Placed: true, Reach: observe.ReachContent,
		Subject: id, Verdict: observe.MatchSame}
}

// WATCHING ALL DAY DOES NOT FILL THE DIRECTOR UP.
//
// The buffer's own test drives the buffer; this drives the SUPERVISOR, which is the thing that
// actually runs for eight hours. Ten thousand readings of a desk somebody left to go to a
// meeting, and the ordinary back-and-forth between two screens, must cost the same as five
// minutes of it.
//
// The number that must not move is what is HELD. Not the reading count, which is supposed to
// climb — a Director that stopped reading the screen after a while would pass a test that only
// looked at size, and be broken.
func TestWatchingAllDayDoesNotFillTheDirectorUp(t *testing.T) {
	rt := watchingRuntime(t)
	a := rt.ambient()
	now := time.Now()

	// Four hours of it, at the ceiling attention. Two screens, because a person who leaves
	// something running is usually flicking between a couple of things, and that is the case
	// where a buffer that was secretly a log would grow fastest.
	const readings = 10000
	for i := 0; i < readings; i++ {
		id := "subj_home"
		if i%2 == 1 {
			id = "subj_bt"
		}
		a.recordPlace("settings", knownPlace(id), now.Add(time.Duration(i)*ambientIdle))
	}

	places, edges, recent := a.buf.Size()
	if places != 2 {
		t.Errorf("%d places held after %d readings of two screens", places, readings)
	}
	if edges != 2 {
		t.Errorf("%d transitions held after %d readings of one back-and-forth", edges, readings)
	}
	if recent > ambient.MaxMoves {
		t.Errorf("the recent walk grew to %d, past its bound of %d", recent, ambient.MaxMoves)
	}

	// AND WHAT IT HOLDS IS STILL TRUE, not merely small. A buffer that had quietly stopped
	// recording would also be two places and two edges.
	view := a.buf.Look()
	total := 0
	for _, p := range view.Places {
		total += p.Seen
	}
	if total != readings {
		t.Errorf("%d sightings counted across %d readings: the counts are what make "+
			"repetition free, and they have to be right for that to mean anything",
			total, readings)
	}
}

// WATCHING IS NOT SOMETHING MARCO DID.
//
// Activity is the account of what MARCO has done — every row is an action it took, and a person
// reads it to see what happened on their behalf. Somebody's own navigation is not that. Writing
// ambient observation into it would turn the one surface that says "here is what I did for you"
// into a log of what its owner did all afternoon, which is a different product nobody asked for.
//
// So: the durable action graph, which is what Activity reads, must be untouched by a watching
// session however much it sees.
//
// Recording an ambient sighting anywhere near the graph must fail this.
func TestWatchingIsNotSomethingMarcoDid(t *testing.T) {
	g, store := watchedRegistry(t)
	_ = store
	graph, err := actiongraph.OpenFile(filepath.Join(t.TempDir(), "action-graph.json"))
	if err != nil {
		t.Fatalf("opening the action graph: %v", err)
	}
	rt := &Runtime{observations: g, graph: graph}
	t.Cleanup(func() { rt.DisableAmbient() })

	a := rt.ambient()
	now := time.Now()
	a.recordPlace("settings", knownPlace("subj_home"), now)
	a.recordPlace("settings", knownPlace("subj_bt"), now.Add(time.Second))
	a.recordPlace("settings", knownPlace("subj_home"), now.Add(2*time.Second))

	if got := a.buf.Look(); len(got.Edges) == 0 {
		t.Fatal("the fixture recorded nothing, so this proves nothing")
	}
	nodes, err := graph.Recent(0)
	if err != nil {
		t.Fatalf("reading the action graph: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("watching wrote %d rows into the account of what Marco has done. "+
			"Somebody's own navigation is not Marco's activity.", len(nodes))
	}
}

// A DIRECTOR SAYS WHETHER IT IS WATCHING, and says it either way.
//
// `marco observe status` answers this precisely, and somebody asking what the Director is doing
// should not have to know that command exists. Printed even when the answer is no, because a
// status report that mentioned watching only while it was happening would make its silence mean
// two things at once.
func TestADirectorSaysWhetherItIsWatching(t *testing.T) {
	off := captureStdout(t, func() {
		printStatus(service.StatusPayload{Running: true, PID: 1, UptimeStr: "1s"})
	})
	if !strings.Contains(off, "Watching: no") {
		t.Errorf("status does not say it is not watching:\n%s", off)
	}

	on := captureStdout(t, func() {
		printStatus(service.StatusPayload{Running: true, PID: 1, UptimeStr: "1s",
			Watching: service.AmbientView{Watching: true, Places: 3, Transitions: 2}})
	})
	if !strings.Contains(on, "Watching: yes") {
		t.Errorf("status does not say it is watching:\n%s", on)
	}
	if !strings.Contains(on, "3 screens") || !strings.Contains(on, "2 moves") {
		t.Errorf("status does not say what watching has noticed:\n%s", on)
	}
	// COUNTS ONLY. The rest of printStatus already holds this line and watching must not be
	// where it breaks: a status report is meant to be safe to paste into a bug report.
	if strings.Contains(on, "subj_") {
		t.Errorf("status printed a screen identity:\n%s", on)
	}
}

// AND THE WHOLE OF IT, once, in order.
//
// Off, then on, then something happens, then somebody asks, then off again and it is gone. Each
// step has its own test above; this is the one that holds them in sequence, because the failure
// this catches is not any single step being wrong — it is two of them disagreeing about what
// state the Director is in.
func TestWatchingFromOnToOffAndGone(t *testing.T) {
	rt := watchingRuntime(t)

	if before := rt.AmbientStatus(); before.Watching {
		t.Fatal("a Director nobody asked to watch says it is watching")
	}

	on := rt.EnableAmbient()
	if !on.Watching {
		t.Fatal("watching did not start")
	}

	a := rt.ambient()
	now := time.Now()
	a.recordPlace("settings", knownPlace("subj_home"), now)
	a.recordPlace("settings", knownPlace("subj_bt"), now.Add(time.Second))

	during := rt.AmbientStatus()
	if !during.Watching {
		t.Fatal("it stopped saying it was watching while it was")
	}
	if during.Places != 2 || during.Transitions != 1 {
		t.Errorf("status reports %d screens and %d moves, want 2 and 1",
			during.Places, during.Transitions)
	}
	// AND NOTHING A PERSON'S SCREEN SAID. Durable subject ids and counts are allowed here —
	// they mean nothing outside Marco, and `printObserving` never puts one in front of
	// anybody — but a label, a window title or a line of screen text is not. The way one of
	// those would arrive is a field somebody adds beside the ones below, so the SHAPE is held
	// closed rather than the values checked one at a time. See ADR-093.
	raw, err := json.Marshal(during)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	allowed := map[string]bool{
		"watching": true, "watching_for_ms": true, "application": true, "place": true,
		"perception_degraded": true, "places": true, "transitions": true, "recent": true,
		"samples": true, "sessions": true, "attention_ms": true,
	}
	for k := range fields {
		if !allowed[k] {
			t.Errorf("the ambient status grew a field %q. Watching reports counts, an "+
				"application and an opaque screen id; anything a person could read off "+
				"their own screen does not belong in it.", k)
		}
	}

	off := rt.DisableAmbient()
	if off.Watching {
		t.Fatal("it is still watching after being told to stop")
	}
	if off.Places != 0 || off.Transitions != 0 {
		t.Errorf("stopping left %d screens and %d moves behind. Ambient evidence is the "+
			"present tense, and keeping it would make `stop` a claim about the future only.",
			off.Places, off.Transitions)
	}
	if after := rt.AmbientStatus(); after.Watching {
		t.Fatal("it says it is watching after being stopped")
	}
}

// captureStdout runs fn with os.Stdout redirected, and returns what it printed.
//
// printStatus writes to stdout rather than to a writer it was handed, and it is not worth
// changing the shape of a printer to test one line of it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		done <- sb.String()
	}()
	fn()
	os.Stdout = saved
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}
