package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
)

// Teaching, entered through the PRODUCTION request path.
//
// This repository has now shipped four mechanisms that nothing invoked, so the rule is that a
// wiring test enters through the production constructor and every new call site gets a mutation
// run. Teach has three call sites worth attacking — the request route, the pass adapter, and the
// arming — and the first two are held here. The third is held in the coordinator's own tests,
// where the fake store refuses a route it does not hold.

// ── the request route ─────────────────────────────────────────────────────────

// TestTeachIsReachableThroughTheProductionRequestPath enters at Runtime.Observation, which is
// where the service delivers a decoded request. Deleting the `q.Teach != nil` branch must fail
// this.
func TestTeachIsReachableThroughTheProductionRequestPath(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry(), teach: &teaching{}}

	// Nothing is being taught yet, and the read says so rather than inventing a session.
	if _, err := rt.Observation(service.ObserveQuery{
		Teach: &service.ObserveTeach{}}); err == nil {
		t.Fatal("reading a teach session before one exists returned a session")
	}

	// A start request with no window named resolves the foreground — and this Runtime has no
	// desktop to resolve it against, so it is refused at the boundary, not deeper.
	out, err := rt.Observation(service.ObserveQuery{
		Teach: &service.ObserveTeach{Name: "open downloads"}})
	if err == nil {
		t.Fatalf("teaching started with no window available: %+v", out)
	}
	if !strings.Contains(err.Error(), "window") && !strings.Contains(err.Error(), "desktop") {
		t.Errorf("the refusal was %q; it should say which window could not be found", err)
	}
}

// TestTeachingWithNoWindowNamedTeachesTheForegroundWindow is the whole point of not asking.
//
// A person saying "teach yourself to open downloads" is looking at the thing they mean. Requiring
// them to run `director windows` first and paste an ephemeral id is a CLI that has to be operated
// rather than used, and the id is meaningless to them either way.
//
// The property is BOTH halves: the foreground is chosen, and it is chosen ONCE. A session handed a
// live "whatever is in front" selector would follow focus onto whatever the user clicked during
// their own demonstration, which is precisely what windowref's ephemeral ids exist to prevent.
//
// Deleting the currentContext call in Teaching must fail this.
func TestTeachingWithNoWindowNamedTeachesTheForegroundWindow(t *testing.T) {
	plat := &focusDesktop{windows: []windowref.Candidate{
		{ID: "hwnd:11", Handle: 11, ProcessID: 3, Application: "code.exe", Title: "marco",
			Bounds: rect(0, 0, 1600, 900), Visible: true, OnScreen: true, Foreground: true},
	}}
	dir := windowref.NewDirectory()
	rt := &Runtime{
		observations: newObservationRegistry(), teach: &teaching{},
		winPlatform: plat, winDirectory: dir,
	}
	rt.observations.memory = &stubTeachMemory{}

	if _, err := rt.Observation(service.ObserveQuery{Teach: &service.ObserveTeach{
		Name: "open downloads", Actor: "Downloads", Verb: "Open",
	}}); err != nil {
		t.Fatalf("teaching the window in front was refused: %v", err)
	}
	defer func() { _, _ = rt.teach.stop() }()

	rt.teach.mu.RLock()
	sel := rt.teach.selector
	rt.teach.mu.RUnlock()

	if sel.Zero() {
		t.Fatal("no window was selected at all; the session would observe nothing")
	}
	if sel.EphemeralID == "" {
		t.Fatalf("the session holds %+v rather than a pinned id. A selector resolved afresh "+
			"every pass follows focus, and the user's next click retargets the lesson", sel)
	}
}

// TestTeachingRefusesWithoutDurableMemory holds the honest failure. A Director that cannot
// recognise anything cannot be taught, and saying so is better than watching for a minute and
// then reporting that the starting place was not recognised.
func TestTeachingRefusesWithoutDurableMemory(t *testing.T) {
	// NO memory: newObservationRegistry() with nothing installed.
	rt := &Runtime{observations: newObservationRegistry(), teach: &teaching{}}
	_, err := rt.Observation(service.ObserveQuery{Teach: &service.ObserveTeach{
		Name: "open downloads", Target: windowref.Selector{EphemeralID: "window_1"},
	}})
	if err == nil {
		t.Fatal("teaching started against a Director with no durable memory")
	}
	if !strings.Contains(err.Error(), "cannot be taught") {
		t.Errorf("the refusal was %q; it should say plainly that this Director cannot be "+
			"taught", err)
	}
}

// TestTeachingIsOneAtATime holds the same rule observation holds, for the same reason: two
// teaching sessions would contend for the screen and neither could attribute what it saw.
func TestTeachingIsOneAtATime(t *testing.T) {
	tc := &teaching{}
	m := &stubTeachMemory{}
	p := &stubTeachPasses{block: make(chan struct{})}
	defer close(p.block)

	if _, err := tc.start("first", p, m, nil, nil,
		windowref.Selector{}, "Downloads", "Open"); err != nil {
		t.Fatalf("starting the first teach session: %v", err)
	}
	waitFor(t, func() bool { return p.started() })

	if _, err := tc.start("second", p, m, nil, nil,
		windowref.Selector{}, "Downloads", "Open"); err == nil {
		t.Fatal("a second teaching session started while the first was running")
	}
}

// TestCancellingTeachingStopsEverything holds Part 28: no capture left running, nothing partial
// kept, and no pass started afterwards.
func TestCancellingTeachingStopsEverything(t *testing.T) {
	tc := &teaching{}
	m := &stubTeachMemory{}
	p := &stubTeachPasses{block: make(chan struct{})}

	if _, err := tc.start("open downloads", p, m, nil, nil,
		windowref.Selector{}, "Downloads", "Open"); err != nil {
		t.Fatalf("starting: %v", err)
	}
	waitFor(t, func() bool { return p.started() })

	s, err := tc.stop()
	if err != nil {
		t.Fatalf("stopping: %v", err)
	}
	if s.Phase != teach.Cancelled {
		t.Fatalf("after stopping the phase is %q, want %q", s.Phase, teach.Cancelled)
	}
	if tc.running() {
		t.Error("the teaching session is still running after being stopped")
	}
	// Release the in-flight pass; nothing may run after it.
	close(p.block)
	before := p.calls()
	time.Sleep(50 * time.Millisecond)
	if got := p.calls(); got != before {
		t.Errorf("%d passes ran after the session was stopped", got-before)
	}
	if m.pending() {
		t.Error("a pending demonstration request survived the cancel; an ordinary session " +
			"later would arm a capture for something nobody is teaching")
	}
}

// TestAnUnusableNameNeverReachesTheDesktop holds Part 21's cheap half: a name Marco cannot write
// down is refused before anybody is asked to demonstrate anything.
func TestAnUnusableNameNeverReachesTheDesktop(t *testing.T) {
	tc := &teaching{}
	p := &stubTeachPasses{}
	s, err := tc.start("   ", p, &stubTeachMemory{}, nil, nil,
		windowref.Selector{}, "Downloads", "Open")
	if err != nil {
		t.Fatalf("starting: %v", err)
	}
	if s.Phase != teach.Refused || s.Refusal != teach.NameNotUsable {
		t.Fatalf("got %q/%q, want refused/%s", s.Phase, s.Refusal, teach.NameNotUsable)
	}
	time.Sleep(20 * time.Millisecond)
	if p.calls() != 0 {
		t.Errorf("%d observation passes ran for an unusable name", p.calls())
	}
	if tc.running() {
		t.Error("a refused teach session is still running")
	}
}

// ── the pass adapter ──────────────────────────────────────────────────────────

// TestATeachPassIsAnOrdinaryObservationSession holds the second call site. The adapter must go
// through the ordinary registry, with the selector it was given and the duration it was asked
// for; a pass that quietly widened either would be a teaching session watching something the
// user did not choose.
func TestATeachPassIsAnOrdinaryObservationSession(t *testing.T) {
	g := newObservationRegistry()
	rt := &Runtime{observations: g}
	selector := windowref.Selector{EphemeralID: "window_7"}
	p := &teachPasses{rt: rt, selector: selector}

	// A registry with no way to acquire the window fails the pass rather than substituting
	// one. That is the property under test: the adapter has no fallback.
	res, err := p.Observe(t.Context(), 2*time.Second)
	if err == nil && res.Session.State == observe.Completed {
		t.Fatal("a teach pass completed against a window that could not be acquired")
	}
}

// TestRunPassReturnsTheSessionsOwnRecord holds the registry addition. A caller that got the
// WRONG session's result would teach from evidence about something else entirely.
func TestRunPassReturnsTheSessionsOwnRecord(t *testing.T) {
	g := newObservationRegistry()
	first, err := g.RunPass(t.Context(), failingTarget{}, nil, nil,
		windowref.Selector{EphemeralID: "window_1"}, briefBounds(), observesession.Episode{})
	if err != nil {
		t.Fatalf("running a pass: %v", err)
	}
	second, err := g.RunPass(t.Context(), failingTarget{}, nil, nil,
		windowref.Selector{EphemeralID: "window_2"}, briefBounds(), observesession.Episode{})
	if err != nil {
		t.Fatalf("running a second pass: %v", err)
	}
	if first.Session.ID == second.Session.ID {
		t.Fatalf("both passes reported session %s; each must get its own record",
			first.Session.ID)
	}
	if first.Session.Selector.EphemeralID != "window_1" ||
		second.Session.Selector.EphemeralID != "window_2" {
		t.Errorf("the records describe %q and %q, not the windows they were asked for",
			first.Session.Selector.EphemeralID, second.Session.Selector.EphemeralID)
	}
	// And the registry is idle again, or the next pass could never start.
	if g.ActiveID() != "" {
		t.Errorf("session %s is still active after RunPass returned", g.ActiveID())
	}
}

// ── the doubles ───────────────────────────────────────────────────────────────

// errNoWindow is the failure both doubles report: the window is not there.
var errNoWindow = errors.New("no such window")

// failingTarget cannot acquire anything, which ends a session quickly and honestly.
type failingTarget struct{}

func (failingTarget) Acquire(context.Context, windowref.Selector) (windowref.Ref, error) {
	return windowref.Ref{}, errNoWindow
}

// stubTeachPasses blocks in Observe until released, so a test can act while a pass is running.
type stubTeachPasses struct {
	block chan struct{}
	mu    sync.Mutex
	n     int
	begun bool
}

func (p *stubTeachPasses) Observe(ctx context.Context, _ time.Duration) (
	observesession.Result, error) {

	p.mu.Lock()
	p.n++
	p.begun = true
	p.mu.Unlock()
	if p.block != nil {
		select {
		case <-p.block:
		case <-ctx.Done():
		}
	}
	return observesession.Result{}, errNoWindow
}

func (p *stubTeachPasses) calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.n
}

func (p *stubTeachPasses) started() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.begun
}

// stubTeachMemory records whether a pending demonstration request was left behind.
type stubTeachMemory struct {
	observe.Memory
	mu   sync.Mutex
	last observe.LearningStatus
}

func (m *stubTeachMemory) Recall(string, observe.StructureSignature) observe.Recollection {
	return observe.Recollection{}
}

func (m *stubTeachMemory) Topology(string) observe.Topology { return observe.Topology{} }

func (m *stubTeachMemory) RememberLearning(_ string, _ observe.RelationshipRef,
	req observe.LearningRequest) error {

	m.mu.Lock()
	defer m.mu.Unlock()
	m.last = req.Status
	return nil
}

func (m *stubTeachMemory) pending() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.last == observe.LearningPending
}

// waitFor spins until cond or the test times out. Cheap, and it keeps the tests free of sleeps
// chosen by guesswork.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the teaching session to reach its first pass")
}

// briefBounds is a session that gives up on a missing window quickly.
//
// The default reacquire window is forty seconds, which is right for somebody who alt-tabbed away
// and wrong for a test whose target never existed.
func briefBounds() observe.Bounds {
	b := observe.DefaultBounds()
	b.Duration = 5 * time.Second
	b.ReacquireWindow = 100 * time.Millisecond
	return b
}

// A phrase Marco could never write down is refused in the FIRST second, not the last.
//
// Validating at save time alone would be correct and cruel: the user demonstrates twice, answers
// a rehearsal question, names two screens, and only then hears that their phrase is not a
// sentence Marco can make a play out of. The names are derived and checked at the request
// boundary; they are bound to nothing until the save.
//
// The example is a SINGLE word since 2026-08-17. A longer phrase now derives its actor from the
// rest of the words and says so out loud; one word still cannot be a sentence of two. The
// property under test — refused here, not at the end — is unchanged, and is the reason this test
// names a phrase rather than asserting a parser rule.
func TestAPhraseThatCannotBecomeAPlayIsRefusedBeforeAnybodyDemonstratesAnything(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry(), teach: &teaching{}}

	_, err := rt.Observation(service.ObserveQuery{Teach: &service.ObserveTeach{
		Name:   "downloads",
		Target: windowref.Selector{EphemeralID: "window_1"},
	}})
	if err == nil {
		t.Fatal("a phrase that cannot become a play started a teaching session")
	}
	if !strings.Contains(err.Error(), "sentence of two") {
		t.Errorf("the refusal reads %q; it should explain what a play name is", err)
	}
	// And it refused for THAT reason rather than for the missing memory, which is the
	// refusal that would otherwise fire first.
	if strings.Contains(err.Error(), "cannot be taught") {
		t.Error("the name was never checked; the refusal came from somewhere else entirely")
	}
	if s, ok := rt.teach.read(); ok {
		t.Fatalf("a teaching session was created for an unusable name: %+v", s)
	}
}

// The outward view may not say more than the session does.
//
// Two claims travel out of here that a person acts on: "I learned it", and "here is the question
// to answer". The first must come from the artifact and the second from the ledger — a view that
// derived either from the phase would be a second source of truth, and the one people read.
func TestTheTeachViewReportsTheArtifactAndTheQuestion(t *testing.T) {
	// A session that reached completion with nothing written must not claim it learned.
	unsaved := viewTeach(teach.Session{Name: "open downloads", Phase: teach.Complete}, false, false)
	if unsaved.Learned {
		t.Error("the view reports a learned play for a session that saved nothing")
	}
	if unsaved.Play != "" {
		t.Errorf("the view names a play %q that does not exist", unsaved.Play)
	}

	saved := viewTeach(teach.Session{
		Name: "open downloads", Phase: teach.Complete,
		Saved: &teach.Saved{Name: "downloads-open", Saved: true},
	}, false, false)
	if !saved.Learned || saved.Play != "downloads-open" {
		t.Errorf("a saved play is not reported: %+v", saved)
	}
	// Saved is not registered, and the view keeps them apart.
	if saved.Registered {
		t.Error("the view reports a saved play as registered")
	}

	// An open question must reach the caller with the id it is answered by. Without it the
	// user is told a question exists and has no way to answer it.
	asking := viewTeach(teach.Session{
		Phase:     teach.Naming,
		SessionID: "observe_3",
		Question:  &teach.Question{ID: "q_name", SessionID: "observe_3"},
	}, true, false)
	if asking.QuestionID != "q_name" {
		t.Errorf("the view carries question id %q, want q_name; a question nobody can "+
			"address is a question nobody can answer", asking.QuestionID)
	}
	if !asking.Waiting {
		t.Error("a naming phase is not reported as waiting")
	}
}

// idlePasses never returns; a coordinator built on it stays wherever it was put.
//
// Used by the view tests, which are about what an account SAYS about a session rather than about
// driving one.
type idlePasses struct{}

func (idlePasses) Observe(ctx context.Context, _ time.Duration) (
	observesession.Result, error) {

	<-ctx.Done()
	return observesession.Result{}, ctx.Err()
}

// A name derived from the person's words is SAID OUT LOUD before it is used.
//
// The phrase is the outcome's name and stays theirs verbatim; the actor and verb are a
// separate artifact derived from it. Deriving silently would put a developer identifier
// wearing their words into a file they later have to read — so the view carries the sentence
// the play would become from the first read, before anybody demonstrates anything, and the
// CLI prints it. See [[ADR-061-a-derived-name-is-said-out-loud]].
func TestTheViewSaysWhatThePlayWillBeCalledBeforeAnythingIsDemonstrated(t *testing.T) {
	v := viewTeach(teach.Session{
		Name: "open mouse settings", Phase: teach.EstablishingStart,
		Actor: "MouseSettings", Verb: "Open",
	}, true, false)
	if v.WillBeCalled != "MouseSettings's Open" {
		t.Fatalf("the view says the play will be called %q, want %q.\nA name welded out of "+
			"somebody's words in silence is one they meet for the first time in a file",
			v.WillBeCalled, "MouseSettings's Open")
	}
	// And it is present BEFORE any demonstration: the phase above is the first one.
	if v.Phase != teach.EstablishingStart {
		t.Fatalf("fixture phase drifted to %q", v.Phase)
	}
	// A session with no names yet claims none.
	if bare := viewTeach(teach.Session{Name: "open downloads"}, true, false); bare.WillBeCalled != "" {
		t.Errorf("a session with no derived name announced %q", bare.WillBeCalled)
	}
}

func (idlePasses) Finish() {}

func (p *stubTeachPasses) Finish() {}

func (idlePasses) AwaitSubject(context.Context) error         { return nil }
func (p *stubTeachPasses) AwaitSubject(context.Context) error { return nil }
