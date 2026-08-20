package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/game"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/diagnostics"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/visualstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/timeline"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
	"github.com/chaynes-simpleclouds/marco/internal/director/uiact"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	waitengine "github.com/chaynes-simpleclouds/marco/internal/director/wait/engine"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// Everything here runs against a fake Director. No desktop, no accessibility
// bridge, no real input — which is what makes the interesting cases (a client
// disappearing mid-command, two clients racing to start the service, a version
// mismatch) testable at all.

// fakeRuntime is a Director that does whatever the test tells it to.
type fakeRuntime struct {
	mu sync.Mutex

	graph *actiongraph.Memory
	// activeValues is what ActiveValues reports, so a test can drive the live-value
	// diagnostics without running a real program.
	activeValues values.EnvironmentSnapshot
	// activeCollections is what ActiveCollections reports.
	activeCollections collections.Snapshot
	// abandoned counts AbandonProgram calls, so a test can prove a cancelled
	// clarification ends the program rather than only the question.
	abandoned int

	// demos is the demonstration recorder and store, built on first use over dir.
	demos *demoFake
	// dir is where the demonstration store lives, set by a test that cares.
	dir string

	// handle, when set, replaces the default behaviour.
	handle func(ctx context.Context, phrase string, progress func(ProgressPayload)) execute.Outcome

	calls        []string
	refined      []intent.Refinement
	perception   diagnostics.Perception
	world        WorldResponse
	liveAnalysis LiveAnalysisResponse
	obsEvents    ObservationEventsResponse
	// playbill is the Director half of the visibility account, and playbillCalls counts
	// how often the server asked for it — so a test can prove the server READS the
	// runtime rather than composing an account of its own.
	playbill      playbill.View
	playbillCalls int
	events        EventsResponse
	providers     []ProviderStatus
	attached      time.Time
	// windows is what the fake reports it can see.
	windows []directorapi.Window
	// confirmations is the broker the server publishes questions through.
	confirmations *ConfirmationBroker
}

func newFakeRuntime() *fakeRuntime {
	return &fakeRuntime{graph: actiongraph.NewMemory(), attached: time.Now()}
}

func (f *fakeRuntime) Handle(ctx context.Context, phrase string, progress func(ProgressPayload)) execute.Outcome {
	f.mu.Lock()
	f.calls = append(f.calls, phrase)
	handler := f.handle
	f.mu.Unlock()

	if handler != nil {
		return handler(ctx, phrase, progress)
	}
	return execute.Outcome{Status: directorapi.ResultDone, Message: "did " + phrase}
}

// HandleClarified records the refinement that was applied, so a test can assert the
// answer reached the Director as a narrowing of the ORIGINAL request rather than as a
// choice already made somewhere else.
func (f *fakeRuntime) HandleClarified(ctx context.Context, phrase string,
	refinement intent.Refinement, progress func(ProgressPayload)) execute.Outcome {

	f.mu.Lock()
	f.refined = append(f.refined, refinement)
	f.mu.Unlock()
	return f.Handle(ctx, phrase, progress)
}

func (f *fakeRuntime) refinements() []intent.Refinement {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]intent.Refinement(nil), f.refined...)
}

// Perception is the diagnostic picture. The fake has no providers, which is exactly
// what a Runtime with nothing wired is entitled to report.
func (f *fakeRuntime) Perception() diagnostics.Perception {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.perception
}

func (f *fakeRuntime) Explanation() diagnostics.Perception { return f.Perception() }

// World and Events are the HUD contracts. The fake serves whatever a test set, so a
// round-trip test asserts the TRANSPORT rather than the Director's own derivation.
func (f *fakeRuntime) LiveAnalysis(LiveAnalysisPayload) LiveAnalysisResponse {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.liveAnalysis
}

// Playbill is the Director half of the one visibility account. The fake serves whatever
// a test set, so a round-trip asserts the TRANSPORT and the server's own contribution
// rather than the Director's derivation, which has its own tests in cmd/director.
func (f *fakeRuntime) Playbill(PlaybillPayload) playbill.View {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.playbillCalls++
	return f.playbill
}

func (f *fakeRuntime) ObservationEvents(p ObservationEventsPayload) ObservationEventsResponse {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.obsEvents
	var kept []observe.LiveEvent
	for _, e := range out.Events {
		if e.Sequence > p.After {
			kept = append(kept, e)
		}
	}
	out.Events = kept
	return out
}

func (f *fakeRuntime) World(WorldPayload) WorldResponse {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.world
}

func (f *fakeRuntime) Events(p EventsPayload) EventsResponse {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.events
	// Honour the cursor, so a test can assert the client's gap arithmetic against a
	// server that actually filters.
	var kept []timeline.Event
	for _, e := range out.Events {
		if e.Seq > p.Cursor {
			kept = append(kept, e)
		}
	}
	out.Events = kept
	return out
}

func (f *fakeRuntime) ReadText(context.Context, *directorapi.Rect) ocr.Diagnostics {
	return ocr.Diagnostics{Engine: "fake", Available: false, Unavailable: "no OCR in tests"}
}

func (f *fakeRuntime) ReadRegion(context.Context, *directorapi.Rect) visualstate.Diagnostics {
	return visualstate.Diagnostics{Provider: "fake"}
}

func (f *fakeRuntime) ActiveWait() waitengine.Snapshot { return waitengine.Snapshot{} }

func (f *fakeRuntime) Edits() []edit.Outcome { return nil }

func (f *fakeRuntime) SemanticActions() []uiact.Outcome { return nil }

func (f *fakeRuntime) Lowerings() []marcoexec.Result { return nil }

func (f *fakeRuntime) TraceFor(string) *trace.Trace { return nil }

func (f *fakeRuntime) RunOperation(context.Context, marcoexec.Operation) marcoexec.Result {
	return marcoexec.Result{}
}

func (f *fakeRuntime) OCRUnavailable() string { return "no OCR in tests" }

func (f *fakeRuntime) Graph() actiongraph.Graph    { return f.graph }
func (f *fakeRuntime) Providers() []ProviderStatus { return f.providers }
func (f *fakeRuntime) AttachedAt() time.Time       { return f.attached }

// Confirmations returns the fake's broker, built lazily so a test that does not care
// about confirmations gets a real one anyway — which is the production shape, and stops a
// nil broker (the "cannot ask" case) from being the accidental default under test.
func (f *fakeRuntime) Confirmations() *ConfirmationBroker {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.confirmations == nil {
		f.confirmations = NewConfirmationBroker()
	}
	return f.confirmations
}

func (f *fakeRuntime) phrases() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// serve starts a service on a temp dir and returns it plus a connect helper.
func serve(t *testing.T, rt Runtime) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	srv := NewServer(Config{Dir: dir, Runtime: rt})
	if _, err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	go func() { _ = srv.Serve() }()
	t.Cleanup(srv.Shutdown)
	return srv, dir
}

// dial connects a client to a served directory.
func dial(t *testing.T, dir string) *Client {
	t.Helper()
	ep, ok := ReadEndpoint(dir)
	if !ok {
		t.Fatal("no endpoint was published")
	}
	c, err := Dial(ep, 2*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// ── service startup ───────────────────────────────────────────────────────────

func TestServiceStartupPublishesAReachableEndpoint(t *testing.T) {
	_, dir := serve(t, newFakeRuntime())

	ep, ok := ReadEndpoint(dir)
	if !ok {
		t.Fatal("the endpoint file should exist")
	}
	// Loopback only. There is no configuration that binds this anywhere else, and a
	// service reachable off the machine would be a remote desktop-control API.
	if !strings.HasPrefix(ep.Address, "127.0.0.1:") {
		t.Errorf("address = %q, want a loopback address", ep.Address)
	}
	if len(ep.Token) < 32 {
		t.Errorf("token looks too short: %d chars", len(ep.Token))
	}
	if ep.ProtocolVersion != ProtocolVersion {
		t.Errorf("endpoint version = %d", ep.ProtocolVersion)
	}
	if !Reachable(ep, 2*time.Second) {
		t.Error("the published endpoint should answer")
	}
}

// A connection that does not present the right token must never have a request
// read, let alone executed.
func TestUnauthenticatedConnectionCannotExecute(t *testing.T) {
	rt := newFakeRuntime()
	_, dir := serve(t, rt)

	ep, _ := ReadEndpoint(dir)
	bad := ep
	bad.Token = strings.Repeat("0", 64)

	if _, err := Dial(bad, 2*time.Second); err == nil {
		t.Fatal("a bad token must be rejected")
	}
	if len(rt.phrases()) != 0 {
		t.Fatal("nothing should have been executed")
	}
}

// ── protocol version ──────────────────────────────────────────────────────────

func TestProtocolVersionMismatchFailsExplicitly(t *testing.T) {
	if err := CheckVersion(ProtocolVersion); err != nil {
		t.Fatalf("the current version should be accepted: %v", err)
	}
	err := CheckVersion(ProtocolVersion + 1)
	if err == nil {
		t.Fatal("a mismatched version must be an error, not negotiated away")
	}
	if !strings.Contains(err.Error(), "mismatch") {
		t.Errorf("the error should explain, got %q", err)
	}
}

// A request from a different protocol version must be refused rather than
// interpreted — a disagreement about the shape of a command would otherwise surface
// as the wrong thing happening on the desktop.
func TestServerRefusesMismatchedRequests(t *testing.T) {
	rt := newFakeRuntime()
	_, dir := serve(t, rt)
	c := dial(t, dir)

	env := RequestEnvelope{
		ProtocolVersion: ProtocolVersion + 99,
		RequestID:       "x1",
		Type:            RequestExecutePhrase,
		Payload:         json.RawMessage(`{"phrase":"click File"}`),
	}
	_ = c.encoder.Encode(env)

	resp, err := c.receive("x1", 3*time.Second)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if resp.Type != ResponseError {
		t.Fatalf("type = %s, want ERROR", resp.Type)
	}
	if len(rt.phrases()) != 0 {
		t.Error("a mismatched request must not be executed")
	}
}

// ── request correlation ───────────────────────────────────────────────────────

func TestRequestResponseCorrelation(t *testing.T) {
	_, dir := serve(t, newFakeRuntime())
	c := dial(t, dir)

	id1, err := c.send(RequestStatus, nil)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	resp, err := c.receive(id1, 3*time.Second)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if resp.RequestID != id1 {
		t.Errorf("response correlated to %q, want %q", resp.RequestID, id1)
	}

	// Ids are unique, so two commands can never be confused for one another.
	id2, _ := c.send(RequestStatus, nil)
	if id1 == id2 {
		t.Error("request ids must be unique")
	}
}

// ── one mutating command ──────────────────────────────────────────────────────

func TestOnlyOneMutatingCommandAtATime(t *testing.T) {
	rt := newFakeRuntime()
	release := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once

	rt.handle = func(ctx context.Context, phrase string, _ func(ProgressPayload)) execute.Outcome {
		once.Do(func() { close(started) })
		<-release
		return execute.Outcome{Status: directorapi.ResultDone, Message: "done"}
	}

	_, dir := serve(t, rt)
	first := dial(t, dir)
	second := dial(t, dir)

	go func() { _, _ = first.Execute("click File", false, nil) }()
	<-started

	// A second mutating command gets a structured refusal rather than running.
	id, _ := second.send(RequestExecutePhrase, ExecutePayload{Phrase: "click Edit"})
	resp, err := second.receive(id, 3*time.Second)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if resp.Type != ResponseBusy {
		t.Fatalf("type = %s, want BUSY", resp.Type)
	}
	var busy BusyPayload
	if resp.Decode(&busy) == nil && busy.ActivePhrase != "click File" {
		t.Errorf("busy names %q, want the running command", busy.ActivePhrase)
	}

	close(release)
}

// Status must stay answerable while a command runs — it is most wanted precisely
// then.
func TestStatusRemainsAvailableDuringExecution(t *testing.T) {
	rt := newFakeRuntime()
	release := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once

	rt.handle = func(ctx context.Context, phrase string, progress func(ProgressPayload)) execute.Outcome {
		once.Do(func() { close(started) })
		progress(ProgressPayload{Stage: "iteration", Iteration: 4, Total: 10})
		<-release
		return execute.Outcome{Status: directorapi.ResultDone}
	}

	_, dir := serve(t, rt)
	runner := dial(t, dir)
	watcher := dial(t, dir)

	go func() { _, _ = runner.Execute("repeat that ten times", false, nil) }()
	<-started
	time.Sleep(50 * time.Millisecond)

	st, err := watcher.Status()
	if err != nil {
		t.Fatalf("status during execution: %v", err)
	}
	if st.Active == nil {
		t.Fatal("status should report the running command")
	}
	if st.Active.Phrase != "repeat that ten times" {
		t.Errorf("active phrase = %q", st.Active.Phrase)
	}
	if st.Active.Iteration != 4 || st.Active.Total != 10 {
		t.Errorf("progress = %d/%d, want 4/10", st.Active.Iteration, st.Active.Total)
	}
	close(release)
}

// ── cancellation ──────────────────────────────────────────────────────────────

// A second client cancels a running command. This is the whole reason the service
// exists: under spawn-per-command a spoken "stop" could not reach the loop.
func TestSecondClientCancelsActiveCommand(t *testing.T) {
	rt := newFakeRuntime()
	started := make(chan struct{})
	var once sync.Once
	var cancelled bool

	rt.handle = func(ctx context.Context, phrase string, _ func(ProgressPayload)) execute.Outcome {
		once.Do(func() { close(started) })
		select {
		case <-ctx.Done():
			cancelled = true
			return execute.Outcome{Status: directorapi.ResultCancelled, Message: "stopped after 3 of 10"}
		case <-time.After(5 * time.Second):
			return execute.Outcome{Status: directorapi.ResultDone}
		}
	}

	_, dir := serve(t, rt)
	runner := dial(t, dir)
	stopper := dial(t, dir)

	outcomes := make(chan OutcomePayload, 1)
	go func() {
		out, _ := runner.Execute("repeat that ten times", false, nil)
		outcomes <- out
	}()
	<-started

	res, err := stopper.Cancel()
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if !res.Accepted {
		t.Fatalf("cancellation should have been accepted: %s", res.Message)
	}

	select {
	case out := <-outcomes:
		if !cancelled {
			t.Error("the command's context should have been cancelled")
		}
		if out.State != CommandCancelled {
			t.Errorf("state = %s, want cancelled", out.State)
		}
		// A cancelled command reports how far it got, because "cancelled" alone
		// leaves the user unsure what state their machine is in.
		if !strings.Contains(out.Message, "3 of 10") {
			t.Errorf("the message should report progress, got %q", out.Message)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the command did not stop")
	}
}

// Cancelling when nothing is running is a reasonable thing to say, and answering
// "there was nothing to stop" beats an error.
func TestCancelWithNothingRunning(t *testing.T) {
	_, dir := serve(t, newFakeRuntime())
	c := dial(t, dir)

	res, err := c.Cancel()
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if res.Accepted {
		t.Error("nothing was running, so nothing was cancelled")
	}
	if res.Message == "" {
		t.Error("it should still say so")
	}
}

// Cancellation before execution begins must stop it without acting.
func TestCancellationBeforeExecution(t *testing.T) {
	rt := newFakeRuntime()
	rt.handle = func(ctx context.Context, phrase string, _ func(ProgressPayload)) execute.Outcome {
		// A command whose context is already dead must not act.
		select {
		case <-ctx.Done():
			return execute.Outcome{Status: directorapi.ResultCancelled, Message: "cancelled before starting"}
		default:
			return execute.Outcome{Status: directorapi.ResultDone, Message: "acted"}
		}
	}

	reg := NewRegistry()
	cmd, ctx, err := reg.Begin(context.Background(), "r1", "click File")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	reg.Cancel()

	out := rt.Handle(ctx, cmd.Phrase, nil)
	if out.Status != directorapi.ResultCancelled {
		t.Errorf("status = %s, want cancelled", out.Status)
	}
}

// ── client disconnect ─────────────────────────────────────────────────────────

// A dropped connection is a client that stopped listening, not a user who changed
// their mind. Treating the two the same would make every network hiccup an
// interruption of work already under way.
func TestClientDisconnectDoesNotCancel(t *testing.T) {
	rt := newFakeRuntime()
	started := make(chan struct{})
	finished := make(chan bool, 1)
	var once sync.Once

	rt.handle = func(ctx context.Context, phrase string, _ func(ProgressPayload)) execute.Outcome {
		once.Do(func() { close(started) })
		select {
		case <-ctx.Done():
			finished <- false
		case <-time.After(300 * time.Millisecond):
			finished <- true
		}
		return execute.Outcome{Status: directorapi.ResultDone}
	}

	srv, dir := serve(t, rt)
	c := dial(t, dir)

	go func() { _, _ = c.Execute("repeat that ten times", false, nil) }()
	<-started

	// The client vanishes mid-command.
	_ = c.Close()

	select {
	case completed := <-finished:
		if !completed {
			t.Fatal("the command was cancelled when the client hung up")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the command never finished")
	}

	// And the service is still healthy, with the outcome retained for whoever asks.
	watcher := dial(t, dir)
	st, err := watcher.Status()
	if err != nil {
		t.Fatalf("the service should still answer: %v", err)
	}
	if len(st.Recent) == 0 {
		t.Error("the finished command should be retained for later inspection")
	}
	_ = srv
}

// ── graph access ──────────────────────────────────────────────────────────────

func TestGraphAccessThroughTheService(t *testing.T) {
	rt := newFakeRuntime()
	node := actiongraph.ActionNode{
		ID: "action_1", Timestamp: time.Now(),
		Goal:   `click "File"`,
		Intent: directorapi.Intent{Raw: "click File"},
		ResolvedTarget: actiongraph.TargetSnapshot{
			App: "notepad", Role: "menu_item", Label: "File",
		},
		Verification: directorapi.VerificationResult{Success: true, Reason: "menu opened"},
		Outcome:      actiongraph.OutcomeSummary{Success: true, Status: directorapi.ActionSucceeded},
	}
	if err := rt.graph.Add(node); err != nil {
		t.Fatalf("seeding the graph: %v", err)
	}

	_, dir := serve(t, rt)
	c := dial(t, dir)

	resp, err := c.History(10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(resp.Entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(resp.Entries))
	}
	e := resp.Entries[0]
	if e.ID != "action_1" || e.Label != "File" || !e.Success {
		t.Errorf("the node did not survive the wire: %+v", e)
	}
	if e.SemanticKey == "" {
		t.Error("the semantic key should be reported")
	}
}

// ── conversation state ────────────────────────────────────────────────────────

// The point of service-owned conversation state: two SEPARATE client processes,
// and the second still knows what "that" refers to.
func TestConversationSurvivesSeparateClients(t *testing.T) {
	rt := newFakeRuntime()
	rt.handle = func(ctx context.Context, phrase string, _ func(ProgressPayload)) execute.Outcome {
		return execute.Outcome{Status: directorapi.ResultDone, Message: "did " + phrase}
	}
	_, dir := serve(t, rt)

	// First client: runs a command, then goes away entirely.
	first := dial(t, dir)
	if _, err := first.Execute("click File", false, nil); err != nil {
		t.Fatalf("first command: %v", err)
	}
	_ = first.Close()

	// Second client, a different connection: the service still remembers.
	second := dial(t, dir)
	st, err := second.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st.Conversation.LastPhrase != "click File" {
		t.Errorf("last phrase = %q, want the first client's", st.Conversation.LastPhrase)
	}
	if st.Conversation.UpdatedAt.IsZero() {
		t.Error("conversation state should be timestamped")
	}

	// And a follow-up reaches the same runtime.
	if _, err := second.Execute("do that again", false, nil); err != nil {
		t.Fatalf("follow-up: %v", err)
	}
	phrases := rt.phrases()
	if len(phrases) != 2 || phrases[0] != "click File" || phrases[1] != "do that again" {
		t.Errorf("both phrases should have reached one runtime, got %v", phrases)
	}
}

// ── command results vs action nodes ───────────────────────────────────────────

// A repeat is ONE command session and several action nodes. Collapsing them would
// make "how did that go?" unanswerable without counting.
func TestCommandResultsAreSeparateFromActionNodes(t *testing.T) {
	reg := NewRegistry()
	cmd, _, err := reg.Begin(context.Background(), "r1", "repeat that 3 times")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	result := reg.Finish(cmd.ID, CommandCompleted, 3, "repeated 3 times")

	if result.CompletedActions != 3 {
		t.Errorf("completed actions = %d, want 3", result.CompletedActions)
	}
	if result.Phrase != "repeat that 3 times" {
		t.Errorf("phrase = %q", result.Phrase)
	}
	recent := reg.Recent(5)
	if len(recent) != 1 {
		t.Fatalf("one command session, got %d", len(recent))
	}
}

// ── provider lifecycle ────────────────────────────────────────────────────────

// The milestone's central win, measured: providers accumulate observations across
// commands instead of starting from cold each time.
func TestProviderReuseAcrossRequests(t *testing.T) {
	tracker := NewProviderTracker()

	// Chrome, cold: the shape a per-command process always saw.
	tracker.Observe(fakeWorld("chrome", 65, 30, 20))
	// ...then hydrated, as it does after sustained client presence.
	for range 4 {
		tracker.Observe(fakeWorld("chrome", 2248, 1200, 115))
	}
	tracker.Observe(fakeWorld("code", 195, 110, 74))

	byApp := map[string]ProviderStatus{}
	for _, p := range tracker.Status() {
		byApp[p.App] = p
	}

	chrome, ok := byApp["chrome"]
	if !ok {
		t.Fatal("chrome should be tracked")
	}
	if chrome.Elements != 2248 {
		t.Errorf("chrome elements = %d, want the hydrated count", chrome.Elements)
	}
	if chrome.Observations != 5 {
		t.Errorf("observations = %d, want 5 — the point is that they accumulate", chrome.Observations)
	}
	if chrome.Status != "ready" {
		t.Errorf("chrome status = %q", chrome.Status)
	}
	if _, ok := byApp["code"]; !ok {
		t.Error("VS Code should be tracked alongside Chrome, not instead of it")
	}
}

// A window exposing only containers is reported as shallow, not as ready — the
// Electron signature, which a status command must not paper over.
func TestProviderStatusDistinguishesShallowFromReady(t *testing.T) {
	tracker := NewProviderTracker()
	tracker.Observe(fakeWorld("discord", 8, 0, 0))

	all := tracker.Status()
	if len(all) != 1 {
		t.Fatalf("want 1 provider, got %d", len(all))
	}
	if all[0].Status == "ready" {
		t.Errorf("a window with nothing operable must not report ready, got %q", all[0].Status)
	}
}

// A tree that is still growing has not finished hydrating, and the tracker must be
// able to say which.
func TestProviderStabilityTracksStructuralChange(t *testing.T) {
	tracker := NewProviderTracker()
	tracker.Observe(fakeWorld("chrome", 65, 30, 20))
	time.Sleep(20 * time.Millisecond)
	tracker.Observe(fakeWorld("chrome", 65, 30, 20)) // unchanged
	stable := tracker.Status()[0].StableFor

	tracker.Observe(fakeWorld("chrome", 2248, 1200, 115)) // changed
	after := tracker.Status()[0].StableFor

	if after > stable {
		t.Error("a changed element count should reset the stability clock")
	}
}

// ── stop-file compatibility ───────────────────────────────────────────────────

// The legacy mechanism is retained behind one adapter so exactly one place has to
// be deleted later. It must still honour only requests newer than the loop.
func TestDeprecatedStopFileAdapter(t *testing.T) {
	dir := t.TempDir()
	start := time.Now()

	check := DeprecatedStopCheck(dir, start)
	if check() {
		t.Fatal("no stop file exists, so nothing should be reported")
	}

	time.Sleep(10 * time.Millisecond)
	if err := writeFile(DeprecatedStopPath(dir), "stop"); err != nil {
		t.Fatal(err)
	}
	if !check() {
		t.Error("a stop written after the loop began should be honoured")
	}

	// A stale one — written BEFORE this loop started — must not cancel it.
	future := DeprecatedStopCheck(dir, time.Now().Add(time.Hour))
	if future() {
		t.Error("a stop older than the loop must be ignored")
	}
}

// ── shutdown ──────────────────────────────────────────────────────────────────

// A restarted service must not claim a command is running. Service state is not
// durable, and fabricating an active command would leave a user waiting for
// something that is not happening.
func TestRestartedServiceHasNoActiveCommand(t *testing.T) {
	rt := newFakeRuntime()
	srv, dir := serve(t, rt)

	c := dial(t, dir)
	if _, err := c.Execute("click File", false, nil); err != nil {
		t.Fatalf("execute: %v", err)
	}
	srv.Shutdown()
	time.Sleep(50 * time.Millisecond)

	// A fresh service over the same directory.
	fresh := NewServer(Config{Dir: dir, Runtime: rt})
	if _, err := fresh.Listen(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	go func() { _ = fresh.Serve() }()
	defer fresh.Shutdown()

	c2 := dial(t, dir)
	st, err := c2.Status()
	if err != nil {
		t.Fatalf("status after restart: %v", err)
	}
	if st.Active != nil {
		t.Error("a restarted service must not report an active command")
	}
	// The action graph, being durable, does survive.
	if st.GraphNodes != rt.graph.Len() {
		t.Error("the graph should still be reachable after a restart")
	}
}

func TestShutdownRemovesTheEndpoint(t *testing.T) {
	srv, dir := serve(t, newFakeRuntime())
	if _, ok := ReadEndpoint(dir); !ok {
		t.Fatal("the endpoint should exist while serving")
	}
	srv.Shutdown()
	time.Sleep(50 * time.Millisecond)
	if _, ok := ReadEndpoint(dir); ok {
		t.Error("a stopped service should not leave its endpoint behind")
	}
}

// ── startup races and stale metadata ──────────────────────────────────────────

// Two clients arriving at once is entirely ordinary — a user typing in one shell
// while the overlay dispatches a spoken phrase in another. Only one service may
// result, or two Directors would fight over the same desktop.
func TestDuplicateStartupRaceProducesOneLock(t *testing.T) {
	dir := t.TempDir()

	first, err := acquireStartupLock(dir, time.Minute)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	if _, err := acquireStartupLock(dir, time.Minute); err == nil {
		t.Fatal("a second starter must not also get the lock")
	}
	first.release()

	// Released, so the next one may proceed.
	second, err := acquireStartupLock(dir, time.Minute)
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	second.release()
}

// A process that died mid-start leaves a lock behind. It must not block startup
// forever.
func TestStaleStartupLockIsBroken(t *testing.T) {
	dir := t.TempDir()
	held, err := acquireStartupLock(dir, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	_ = held

	// Treat anything older than a moment as abandoned.
	time.Sleep(20 * time.Millisecond)
	stolen, err := acquireStartupLock(dir, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("a stale lock must be breakable: %v", err)
	}
	stolen.release()
}

// A service that was killed leaves its endpoint file behind. The file is evidence,
// not authority: a client validates it by connecting.
func TestStaleEndpointIsNotTrusted(t *testing.T) {
	dir := t.TempDir()
	stale := Endpoint{
		ProtocolVersion: ProtocolVersion,
		// A port nothing is listening on.
		Address: "127.0.0.1:1", Token: strings.Repeat("a", 64),
		PID: 999999, StartedAt: time.Now(),
	}
	if err := WriteEndpoint(dir, stale); err != nil {
		t.Fatal(err)
	}

	if Reachable(stale, 200*time.Millisecond) {
		t.Fatal("nothing is listening; this must not report reachable")
	}
	// And a client that is not allowed to start one says so rather than hanging.
	_, err := Connect(ConnectOptions{Dir: dir, AutoStart: false, DialTimeout: 200 * time.Millisecond})
	if err == nil {
		t.Fatal("connecting to a dead endpoint should fail")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("the error should explain, got %q", err)
	}
}

// ── event streaming ───────────────────────────────────────────────────────────

// Progress must arrive as it happens, not be buffered until the command finishes.
func TestProgressEventsStreamBeforeCompletion(t *testing.T) {
	rt := newFakeRuntime()
	rt.handle = func(ctx context.Context, phrase string, progress func(ProgressPayload)) execute.Outcome {
		for i := 1; i <= 3; i++ {
			progress(ProgressPayload{
				Stage: "iteration", Iteration: i, Total: 3,
				Detail: "verified", Verified: true,
			})
		}
		return execute.Outcome{Status: directorapi.ResultDone, Message: "repeated 3 times"}
	}
	_, dir := serve(t, rt)
	c := dial(t, dir)

	var seen []ResponseType
	var progressCount int
	out, err := c.Execute("repeat that 3 times", false, func(ev ResponseEnvelope) {
		seen = append(seen, ev.Type)
		if ev.Type == ResponseProgress {
			progressCount++
		}
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if progressCount != 3 {
		t.Errorf("want 3 progress events, got %d (%v)", progressCount, seen)
	}
	// ACKNOWLEDGED first, and immediately: a person who has just spoken needs to know
	// they were heard long before the command finishes.
	if len(seen) < 2 || seen[0] != ResponseAcknowledged {
		t.Errorf("the first response should be ACKNOWLEDGED, got %v", seen)
	}
	if seen[len(seen)-1] != ResponseCompleted {
		t.Errorf("the last response should be terminal, got %v", seen)
	}
	if out.Message != "repeated 3 times" {
		t.Errorf("outcome message = %q", out.Message)
	}
}

// UNVERIFIED stays its own outcome. "I did it but could not confirm it" is
// different information from "it did not work", and the difference decides whether
// a user should try again.
func TestUnverifiedIsNotFailed(t *testing.T) {
	rt := newFakeRuntime()
	rt.handle = func(context.Context, string, func(ProgressPayload)) execute.Outcome {
		return execute.Outcome{Status: directorapi.ResultPartial, Message: "could not confirm"}
	}
	_, dir := serve(t, rt)
	c := dial(t, dir)

	var terminal ResponseType
	out, err := c.Execute("click File", false, func(ev ResponseEnvelope) {
		if ev.Type.Terminal() {
			terminal = ev.Type
		}
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if terminal != ResponseUnverified {
		t.Errorf("response type = %s, want UNVERIFIED", terminal)
	}
	if out.State != CommandUnverified {
		t.Errorf("state = %s, want unverified", out.State)
	}
}

// ── helpers ──

func fakeWorld(app string, elements, content, actionable int) directorapi.WorldState {
	w := directorapi.WorldState{
		Timestamp: time.Now(),
		ActiveApp: &directorapi.Application{ID: app, Name: app},
		Elements:  map[directorapi.ElementID]*directorapi.Element{},
	}
	for i := 0; i < elements; i++ {
		id := directorapi.ElementID(itoa(i))
		el := &directorapi.Element{ID: id, Role: directorapi.RolePane}
		if i < content {
			el.Role = directorapi.RoleText
		}
		if i < actionable {
			el.Role = directorapi.RoleButton
			el.Label = "control"
			el.Enabled, el.Visible = true, true
			el.Bounds = directorapi.Rect{X: 0, Y: i, Width: 10, Height: 10}
		}
		w.Elements[id] = el
	}
	if elements > 0 {
		w.Confidence = directorapi.WorldConfidence{
			ObservationQuality: 0.9,
			Coverage:           float64(content) / float64(elements),
			Freshness:          1,
		}
		if actionable > 0 {
			w.Confidence.Actionability = 0.9
		}
	}
	return w
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

// ActiveValues lets a test drive the live-value diagnostics without a real program.
func (f *fakeRuntime) ActiveValues() values.EnvironmentSnapshot { return f.activeValues }

// AbandonProgram records that the paused program was discarded.
func (f *fakeRuntime) AbandonProgram(string) {
	f.mu.Lock()
	f.abandoned++
	f.activeValues = values.EnvironmentSnapshot{Cleared: true}
	f.mu.Unlock()
}

// ActiveCollections lets a test drive the collection diagnostics without a program.
func (f *fakeRuntime) ActiveCollections() collections.Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.activeCollections
}

// Windows is what the fake can see. Empty by default: a test that cares sets it.
func (f *fakeRuntime) Windows() []directorapi.Window {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.windows
}

// The demonstration surface, on the fake.
//
// A real recorder and a real store in a temp directory, because the interesting service
// tests are about ROUTING — that an action reaches the right method and comes back in the
// right field — and a stub that returned canned values would test the stub.
func (f *fakeRuntime) demoRuntime() *demoFake {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.demos == nil {
		f.demos = newDemoFake(f.dir)
	}
	return f.demos
}

func (f *fakeRuntime) StartDemonstration() (*demo.Demonstration, error) {
	return f.demoRuntime().recorder.Start(demo.NewID(time.Now()))
}
func (f *fakeRuntime) StopDemonstration() (*demo.Demonstration, error) {
	return f.demoRuntime().recorder.Stop()
}
func (f *fakeRuntime) AbandonDemonstration(reason string) (*demo.Demonstration, error) {
	return f.demoRuntime().recorder.Abandon(reason)
}
func (f *fakeRuntime) ActiveDemonstration() *demo.Demonstration {
	return f.demoRuntime().recorder.Active()
}
func (f *fakeRuntime) Demonstrations() ([]*demo.Demonstration, error) {
	return f.demoRuntime().store.Demonstrations()
}
func (f *fakeRuntime) Demonstration(id demo.ID) (*demo.Demonstration, error) {
	return f.demoRuntime().store.Demonstration(id)
}
func (f *fakeRuntime) ExtractProcedure(id demo.ID) (demo.Extraction, error) {
	d, err := f.Demonstration(id)
	if err != nil {
		return demo.Extraction{}, err
	}
	return demo.Extract(d), nil
}
func (f *fakeRuntime) ApproveProcedure(id demo.ID, by string) (*demo.Learned, error) {
	out, err := f.ExtractProcedure(id)
	if err != nil {
		return nil, err
	}
	l, err := demo.Approve(out, by, time.Now())
	if err != nil {
		return nil, err
	}
	return l, f.demoRuntime().store.SaveLearned(l)
}
func (f *fakeRuntime) ForgetProcedure(name string) error {
	return f.demoRuntime().store.Forget(name)
}
func (f *fakeRuntime) LearnedProcedures() []*demo.Learned {
	return f.demoRuntime().store.Learned()
}

// demoFake is a recorder and a store over a temp directory.
type demoFake struct {
	recorder *demo.Recorder
	store    *demo.Store
}

func newDemoFake(dir string) *demoFake {
	if dir == "" {
		dir = os.TempDir()
	}
	store, err := demo.Open(dir)
	if err != nil {
		panic("service test: opening the demonstration store: " + err.Error())
	}
	r := demo.NewRecorder()
	r.OnComplete = func(d *demo.Demonstration) { _ = store.SaveDemonstration(d) }
	return &demoFake{recorder: r, store: store}
}

// The capability-pack surface, on the fake.
//
// An EMPTY registry, which is the Director's ordinary state and the one worth covering
// here: the service must transport "no pack serves this" as an answer rather than as a
// missing field.
func (f *fakeRuntime) DetectedGame() game.Active { return game.Active{} }

func (f *fakeRuntime) GameCapabilities() game.Report {
	return game.NewRegistry().Report(game.Active{})
}

func (f *fakeRuntime) GameInventory(string) game.InventoryReport {
	return game.InventoryReport{Unavailable: "no capability packs in tests"}
}

// The vision surface, on the fake.
//
// No detector, which is the ordinary state of a Director: the service must transport
// "vision is unavailable" as an ANSWER rather than as an empty result, because "no model
// is installed" and "this window is empty" are different findings.
func (f *fakeRuntime) ReadVision(context.Context, *directorapi.Rect, windowref.Selector) vision.Diagnostics {
	return vision.Diagnostics{
		Backend: "fake", Available: false, Unavailable: "no detector in tests",
	}
}

func (f *fakeRuntime) LastVision() vision.Diagnostics {
	return f.ReadVision(nil, nil, windowref.Selector{})
}

func (f *fakeRuntime) Frames() []vision.FrameRecord { return nil }

func (f *fakeRuntime) VisionUnavailable() string { return "no detector in tests" }

// LiveWindows, on the fake. No desktop in tests, so nothing is capturable.
func (f *fakeRuntime) LiveWindows(context.Context, string) []windowref.Listing { return nil }

// The observation surface, on the fake: no registry, so nothing to observe.
func (f *fakeRuntime) StartObservation(ObservePayload) (ObserveStarted, error) {
	return ObserveStarted{}, errors.New("no observation registry in tests")
}
func (f *fakeRuntime) Observation(ObserveQuery) (any, error) {
	return nil, errors.New("no observation registry in tests")
}

func (f *fakeRuntime) LearnedPlay(LearnedQuery) (LearnedView, error) {
	return LearnedView{}, nil
}

// The fake reports a working Accessibility Actor: these tests are about the service's routing,
// not about a machine with a missing binary. The degraded case has its own test in cmd/director.
func (f *fakeRuntime) AccessibilityUnavailable() string { return "" }
