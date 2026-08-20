package service

import (
	"context"
	"sync"
	"testing"
)

// One word, three kinds of thing under way.
//
// A Learn EPISODE rides RequestObservation with no command behind it, so `Registry.Cancel` could
// never see one. Measured: a leader key, a spoken "stop" and `marco director stop` all answered
// "nothing is running" during a demonstration, while the demonstration went on capturing, went on
// holding the observation slot, and went on refusing the next Start with "a learn session is
// already running".

// learningRuntime is a Director with somebody demonstrating into it.
//
// Through the PRODUCTION surfaces: it implements Acquirer, which is what the server asserts for,
// and it records what arrives at `Observation` — which is where the ONE implementation of
// abandoning an episode lives. A fake that offered the server a bespoke "cancel learning" method
// would prove the opposite of what this test is for.
type learningRuntime struct {
	*fakeRuntime
	mu      sync.Mutex
	running bool
	// seen is every acquisition request the server made, in order.
	seen []ObserveLearn
}

func newLearningRuntime() *learningRuntime {
	return &learningRuntime{fakeRuntime: newFakeRuntime(), running: true}
}

func (l *learningRuntime) LearningNow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.running
}

func (l *learningRuntime) Observation(q ObserveQuery) (any, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if q.Learn == nil {
		return nil, nil
	}
	l.seen = append(l.seen, *q.Learn)
	if q.Learn.Cancel {
		l.running = false
	}
	return struct{}{}, nil
}

func (l *learningRuntime) requests() []ObserveLearn {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]ObserveLearn(nil), l.seen...)
}

// STOPPING DURING A DEMONSTRATION CANCELS THE EPISODE.
//
// # The mutations this kills
//
//   - delete the third arm of cancelActive: "stop" answers "nothing is running" while somebody
//     is demonstrating, which is what it did.
//   - route it to Finish instead of Cancel: the demonstration is KEPT and the pipeline runs on,
//     so the person who said the abort word gets a saved play. ADR-066 is explicit that routing
//     one of the two to the other "silently destroys a demonstration a person has just given —
//     and it would look like it worked"; this is the same hazard with the sign reversed.
//   - answer accepted without sending anything: the episode goes on capturing while the person
//     is told it stopped.
func TestStoppingDuringADemonstrationCancelsTheEpisode(t *testing.T) {
	rt := newLearningRuntime()
	_, dir := serve(t, rt)
	client := dial(t, dir)

	res, err := client.Cancel()
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if !res.Accepted {
		t.Fatalf("stopping during a demonstration was refused: %q. A Learn episode claims no "+
			"registry slot, so nothing in cancelActive could see it and the Audience was "+
			"told nothing was running while Marco went on watching them.", res.Message)
	}

	sent := rt.requests()
	if len(sent) != 1 {
		t.Fatalf("the stop made %d acquisition request(s), want exactly one", len(sent))
	}
	if !sent[0].Cancel {
		t.Error("the stop did not send Cancel. A bare \"stop\" is the abort word everywhere " +
			"in Marco and must never mean Finish.")
	}
	if sent[0].Finish || sent[0].Stop {
		t.Error("the stop asked to FINISH the demonstration. Finish keeps everything and runs " +
			"the pipeline — the opposite of what the person asked for, and it would look " +
			"like it had worked.")
	}
	if rt.LearningNow() {
		t.Error("the episode is still running after an accepted stop")
	}
}

// AND NOTHING IS INVENTED WHEN NOBODY IS DEMONSTRATING.
//
// The arm must be reached only when there is an episode. A cancelActive that sent the acquisition
// request unconditionally would turn every idle "stop" into an error from a Director that has
// nothing to learn — and would report `accepted` for having done nothing.
func TestStoppingWithNothingRunningStillSaysNothingIsRunning(t *testing.T) {
	rt := newLearningRuntime()
	rt.running = false
	_, dir := serve(t, rt)

	res, err := dial(t, dir).Cancel()
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if res.Accepted {
		t.Errorf("stopping with nothing running reported %q as accepted", res.Message)
	}
	if got := rt.requests(); len(got) != 0 {
		t.Errorf("an idle stop made %d acquisition request(s); it must make none", len(got))
	}
}

// registeringRuntime is a Director that accepts the command registry.
//
// It implements CommandRegistrar, which is what NewServer asserts for, and records what it was
// handed. Nothing else about it matters.
type registeringRuntime struct {
	*fakeRuntime
	mu  sync.Mutex
	reg *Registry
	ctx context.Context
}

func (r *registeringRuntime) UseCommands(serviceCtx context.Context, reg *Registry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reg, r.ctx = reg, serviceCtx
}

// THE SERVER HANDS ITS RUNTIME THE COMMAND REGISTRY.
//
// # Why this enters through NewServer
//
// Because the defect it guards is a WIRING defect, and a wiring test that builds the collaboration
// itself proves only that the collaboration can be built. This repository has three recorded cases
// of complete, correct code that nothing ever called.
//
// What depends on it: the one thing this Director does to the desktop without the server routing a
// request — a live rehearsal, reached from inside a Learn episode — claims its slot through the
// registry it is given here. A Runtime that never receives one runs the walk unpublished, which is
// exactly the state `director stop` could not see into.
//
// Deleting the UseCommands call in NewServer must fail this.
func TestTheServerHandsItsRuntimeTheCommandRegistry(t *testing.T) {
	rt := &registeringRuntime{fakeRuntime: newFakeRuntime()}
	srv := NewServer(Config{Dir: t.TempDir(), Runtime: rt})

	rt.mu.Lock()
	got, ctx := rt.reg, rt.ctx
	rt.mu.Unlock()

	if got == nil {
		t.Fatal("the server kept its command registry to itself. Work the Director begins " +
			"on its own — a live rehearsal inside a Learn episode — is then invisible to " +
			"`director status` and unreachable by CANCEL_ACTIVE while it types.")
	}
	if got != srv.registry {
		t.Error("the Director was handed a DIFFERENT registry from the one the server " +
			"cancels. Two registries is two lifecycles, and only one of them hears stop.")
	}
	if ctx == nil {
		t.Error("the Director was handed no service lifetime, so work it begins outlives " +
			"the shutdown that was supposed to end it")
	}
	if ctx != nil && ctx.Err() != nil {
		t.Error("the service lifetime handed over is already finished")
	}
}
