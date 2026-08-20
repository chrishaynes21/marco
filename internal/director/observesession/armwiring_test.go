package observesession_test

import (
	"context"
	"sync"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// A window chosen WITHOUT naming its application must still arm the approved demonstration.
//
// # What this is, and why the whole suite missed it
//
// The durable topology is keyed by application name. The arming used to read that name from
// `cfg.Selector.Application` — but a selector names a WINDOW, and only the `--application` form
// carries an application at all. `windowref.Selector{EphemeralID: …}` carries none, which is what
// `--window-id` produces and what resolving the foreground produces, and resolving the foreground
// is how a person actually teaches something.
//
// So `Topology("")` was consulted, held nothing, and no capture was ever created. Live, that
// surfaced as Marco saying "Something went wrong on my side — I wasn't watching for your example"
// to a user who had just demonstrated the route four times; the traversals were all in the store,
// with their navigation intact, and nothing had watched for them.
//
// Every fixture in this package builds `Selector{Application: "testgame"}`, so every test armed
// correctly and the defect could not be seen from inside. This one differs in exactly one field.

// foregroundConfig is `config()` with the window named the way a person's session names it: an
// adopted ephemeral id, and no application.
func foregroundConfig() observesession.Config {
	cfg := config()
	cfg.Selector = windowref.Selector{EphemeralID: "window_1"}
	return cfg
}

func TestAWindowChosenWithoutNamingItsApplicationStillArms(t *testing.T) {
	dir := t.TempDir()
	store, from, to := approvedRun(t, dir)

	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: happyScript()}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)

	res, err := r.Run(context.Background(), foregroundConfig())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Demonstration == nil {
		t.Fatalf("the approved demonstration %s → %s was never captured.\nThe session named its "+
			"window by ephemeral id and carried no application, so the arming looked up the "+
			"topology of \"\" and found nothing waiting. A person teaching something has always "+
			"reached this path: they do not run `director windows` first.", from, to)
	}
	if !res.Demonstration.Complete {
		t.Fatalf("captured, but incomplete: %s", res.Demonstration.Reason)
	}
	if res.Demonstration.Relationship.From != from || res.Demonstration.Relationship.To != to {
		t.Errorf("the candidate names %+v, not the approved edge %s → %s",
			res.Demonstration.Relationship, from, to)
	}
}

// The control: naming the application still works, so the fix did not simply move the hole.
func TestNamingTheApplicationStillArms(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := approvedRun(t, dir)

	_, res := demonstrate(t, store, happyScript())
	if res.Demonstration == nil {
		t.Fatal("a selector that DOES name its application armed nothing; the arming has moved " +
			"from one half-working placement to another")
	}
}

// The capture is armed before the first sample reaches it, which is the property that made the
// original placement "before the loop".
//
// A user already standing on the start when the session opens must be seen there. The script holds
// on A from its very first frame and never returns, so a capture armed even one sample late would
// have missed the start and could not complete.
func TestTheCaptureSeesTheVeryFirstSample(t *testing.T) {
	dir := t.TempDir()
	store, from, _ := approvedRun(t, dir)

	// A, then away, and never back: the only sighting of the start is at the top.
	script := []demoFrame{press("a", observe.NavDown, observe.NavDown, observe.NavConfirm)}
	script = append(script, hold("x", 3)...)
	script = append(script, press("b", observe.NavConfirm))
	script = append(script, hold("b", 4)...)

	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: script}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)

	res, err := r.Run(context.Background(), foregroundConfig())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Demonstration == nil {
		t.Fatal("nothing was captured at all")
	}
	if res.Demonstration.Start.Subject != from {
		t.Errorf("the start is %+v, want %s: the capture was armed after the first sample and "+
			"the only frame showing the start went past unwatched",
			res.Demonstration.Start, from)
	}
}

// The store is read once, not twice a second for the life of the session.
//
// The arming consults the durable topology, which takes the store's lock and may touch a file.
// Moving it into the sample path is only acceptable because it happens once — a session that finds
// no pending request must not keep asking.
func TestTheTopologyIsConsultedOncePerSession(t *testing.T) {
	dir := t.TempDir()
	inner := memoryAt(t, dir)
	counting := &countingTopology{Memory: inner}

	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: happyScript()}, &recordingEvents{}).
		WithMemory(counting)

	if _, err := r.Run(context.Background(), foregroundConfig()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// reviewLearning reads it too, once, at the end. Two is the ceiling; per-sample is what this
	// is guarding against, and the script is long enough that the difference is unmistakable.
	if counting.calls > 2 {
		t.Errorf("Topology was read %d times in one session; the arming is re-asking on every "+
			"sample, which puts a store lock and a possible file read on the sampling path",
			counting.calls)
	}
}

// A first sample that cannot say what application it is does not burn the one arming attempt.
//
// The window reference is resolved per sample, and nothing guarantees the first one carries a name
// — a process still starting up, or a shell surface that resolves late. Spending the single attempt
// on an empty name would leave the session permanently unarmed for a request that was waiting the
// whole time, and it would look exactly like the defect this file exists for.
func TestAnUnnamedFirstSampleDoesNotBurnTheArmingAttempt(t *testing.T) {
	dir := t.TempDir()
	store, from, to := approvedRun(t, dir)

	nameless := ref(1)
	nameless.Application = ""
	target := &latecomer{first: nameless, rest: ref(1)}

	r := observesession.New(newClock(), target,
		&demoSampler{script: happyScript()}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)

	res, err := r.Run(context.Background(), foregroundConfig())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Demonstration == nil {
		t.Fatalf("the approved demonstration %s → %s was never captured because the FIRST "+
			"window reference had no application name. The attempt was spent on \"\" and never "+
			"retried, so a slow-resolving window silently watches nothing.", from, to)
	}
}

// latecomer resolves without an application name once, then normally.
type latecomer struct {
	first, rest windowref.Ref
	calls       int
	mu          sync.Mutex
}

func (l *latecomer) Acquire(context.Context, windowref.Selector) (windowref.Ref, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	if l.calls == 1 {
		return l.first, nil
	}
	return l.rest, nil
}

// countingTopology counts topology reads and delegates everything else.
type countingTopology struct {
	observe.Memory
	calls int
}

func (c *countingTopology) Topology(application string) observe.Topology {
	c.calls++
	return c.Memory.Topology(application)
}
