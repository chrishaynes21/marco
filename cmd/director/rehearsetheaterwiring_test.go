package main

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// A demonstrated click, rehearsed through the Theater, from the composition root.
//
// # Why this test is here
//
// Because `rehearse` proves it hands a production over and `theaterhost` proves it puts one on,
// and neither of them proves that THIS binary connects the two. A Theater nobody wires is a
// Director that refuses every press with "this Director has no Theater" — silently, on every
// machine, while every unit test passes.
//
// That is not hypothetical in this repository: three separate pieces of complete, tested code have
// been found never invoked. So this enters through `Runtime.Observation`, the same request the CLI
// makes, and checks what the notebook at the bottom actually recorded.

// bridgeSpy is an Accessibility host that answers a look and records everything.
type bridgeSpy struct {
	mu    sync.Mutex
	calls []string
}

func (b *bridgeSpy) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	b.mu.Lock()
	b.calls = append(b.calls, c.Act+"'s "+c.Action)
	b.mu.Unlock()
	return "ok", runtime.Absent(), nil
}

func (b *bridgeSpy) seen() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.calls...)
}

// clickOn is a frame in which the person pressed a named control.
func clickOn(screen, role, label string) dryFrame {
	return dryFrame{screen: screen, inputs: []observe.NavIntent{observe.NavPoint},
		target: &observe.SemanticTarget{Role: role, Label: label}}
}

// aToBByClicking is the same route as aToB, demonstrated with a click rather than a key.
func aToBByClicking() []dryFrame {
	var out []dryFrame
	out = append(out, dryHold("a", 4)...)
	out = append(out, clickOn("b", "button", "Mouse"))
	out = append(out, dryHold("b", 5)...)
	return out
}

// clickAuthorized drives the whole chain with a demonstrated CLICK and stops holding one grant.
func clickAuthorized(t *testing.T) *observationRegistry {
	t.Helper()
	restore := sessionClock
	sessionClock = newDryClock()
	t.Cleanup(func() { sessionClock = restore })

	store, _ := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	g := newObservationRegistry()
	g.memory = store
	seedDryRoute(t, store)

	id := observeOnce(t, g, dryHold("", 6))
	sayYes(t, g, id, observe.AskLearnRelationship)
	observeOnce(t, g, aToBByClicking())
	id = observeOnce(t, g, dryHold("a", 8))
	sayYes(t, g, id, observe.AskRehearse)

	if g.last.Grant() == nil {
		t.Fatal("the chain produced no authorization, so there is nothing to rehearse")
	}
	return g
}

// The Theater is wired, reached, and its production lands in the dry run's notebook.
//
// # What each half of this catches
//
// Not wired at all: the press refuses `cannot_express` and nothing is recorded — which is what a
// deleted `WithTheater` looks like from outside, and it looks exactly like a broken demonstration.
//
// Wired to the LIVE runner in a dry run: the production would be performed on the real bridge
// instead of recorded, which the last assertion here catches.
//
// Deleting the WithTheater call must fail this.
func TestARehearsalPressGoesThroughTheTheater(t *testing.T) {
	g := clickAuthorized(t)
	spy := &bridgeSpy{}
	rt := &Runtime{observations: g, bridge: spy}

	out, err := rt.Observation(service.ObserveQuery{
		Rehearse: &service.ObserveRehearse{Step: 1},
	})
	if err != nil {
		t.Fatalf("the rehearsal request failed: %v", err)
	}
	view, ok := out.(service.RehearsalView)
	if !ok {
		t.Fatalf("the rehearsal request returned %T", out)
	}
	if !view.Attempted {
		t.Fatalf("nothing was produced: %s — %s", view.Refusal, view.Detail)
	}
	if len(view.Steps) == 0 {
		t.Fatalf("the attempt recorded no steps: terminal=%q", view.Terminal)
	}
	step := view.Steps[0]

	// THE ACTOR LOOKED. Finding a control is a read, and a dry run may look all it likes.
	var looked bool
	for _, c := range spy.seen() {
		if c == "Accessibility's Snapshot" {
			looked = true
		}
	}
	if !looked {
		t.Errorf("the Theater never looked for the control; calls were %v.\n"+
			"A Director with no Theater refuses every press, silently, on every machine.",
			spy.seen())
	}
	// AND THE PRODUCTION WAS RECORDED, not performed. The notebook is the whole point of a
	// dry run, and the program came back over the production boundary to be shown.
	if !strings.Contains(strings.Join(step.Emitted, "\n"), "Accessibility's Invoke") {
		t.Errorf("the notebook does not hold the production: %v\n"+
			"A cast program that reached the default host instead would still work and "+
			"would stop being recorded.", step.Emitted)
	}
	if !strings.Contains(step.Program, "use accessibility.") {
		t.Errorf("the step does not carry the Marco the Actor wrote:\n%s", step.Program)
	}
	// And nothing real happened: a dry run's productions go to the notebook, not the bridge.
	for _, c := range spy.seen() {
		if c == "Accessibility's Invoke" {
			t.Error("a DRY rehearsal invoked a control on the real bridge")
		}
	}
}

// A dry rehearsal draws no conclusion from a production either.
//
// The verifier travels with the request and a dry caller brings none, so the Theater reports the
// production unverified rather than claiming a route it never took.
func TestADryProductionCompletesNothing(t *testing.T) {
	g := clickAuthorized(t)
	rt := &Runtime{observations: g, bridge: &bridgeSpy{}}

	out, err := rt.Observation(service.ObserveQuery{
		Rehearse: &service.ObserveRehearse{Step: 1},
	})
	if err != nil {
		t.Fatalf("the rehearsal request failed: %v", err)
	}
	view := out.(service.RehearsalView)
	if view.Completed {
		t.Fatal("a dry production completed a route it never touched")
	}
	if view.Live {
		t.Fatal("a request without --live installed a real host")
	}
}
