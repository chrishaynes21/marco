package execute

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/internal/recorded"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/plan"
	"github.com/chaynes-simpleclouds/marco/internal/director/policy"
	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/internal/director/verify"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The whole pipeline is exercised here without a desktop. Perception is a scripted
// sequence of world states, the actuator records calls instead of making them, and
// the clock is injected — so every one of these runs in CI, on any machine, with no
// possibility of clicking something real.
//
// That is not merely convenient. A test suite that needs a live desktop cannot cover
// failure cases at all: there is no way to make a real window refuse to move on
// demand, and those are exactly the paths that matter.

var t0 = time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

func rect(x, y, w, h int) directorapi.Rect {
	return directorapi.Rect{X: x, Y: y, Width: w, Height: h}
}

// obs builds a synthetic accessibility observation.
func obs(id string, role directorapi.ElementRole, label string, r directorapi.Rect) directorapi.Observation {
	enabled, visible, focused, selected := true, true, false, false
	return directorapi.Observation{
		ID: directorapi.ObservationID("acc:" + id), Source: directorapi.SourceAccessibility,
		WindowID: "hwnd:1", Role: role, Label: label, Bounds: r,
		Enabled: &enabled, Visible: &visible, Focused: &focused, Selected: &selected,
		Confidence: 1, NativeID: id,
	}
}

func focused(o directorapi.Observation) directorapi.Observation {
	yes := true
	o.Focused = &yes
	return o
}

// scene builds one world state at a given time.
func scene(at time.Time, windows []directorapi.Window, obs ...directorapi.Observation) directorapi.WorldState {
	if len(windows) == 0 {
		windows = []directorapi.Window{{
			ID: "hwnd:1", Application: "notepad", Title: "Untitled - Notepad",
			Bounds: rect(100, 100, 800, 600), Focused: true, Visible: true,
			MonitorID: "monitor:1",
		}}
	}
	id := windows[0].ID
	return recorded.NewBuilder().Build(recorded.Perception{
		Timestamp: at, Observations: obs, Windows: windows, ActiveWindow: &id,
		ActiveApp: &directorapi.Application{ID: windows[0].Application, Name: windows[0].Application},
		Monitors:  testMonitors,
	})
}

var testMonitors = []directorapi.Monitor{
	{ID: "monitor:1", Bounds: rect(0, 0, 1920, 1080), WorkArea: rect(0, 0, 1920, 1040), Primary: true},
	{ID: "monitor:2", Bounds: rect(-1920, 0, 1920, 1080), WorkArea: rect(-1920, 0, 1920, 1040)},
}

// menuBar is the shape a live Notepad presents: File/Edit/View and no Save.
func menuBar() []directorapi.Observation {
	return []directorapi.Observation{
		obs("uia:1", directorapi.RoleWindow, "Untitled - Notepad", rect(100, 100, 800, 600)),
		obs("uia:2", directorapi.RoleMenuItem, "File", rect(110, 140, 40, 30)),
		obs("uia:3", directorapi.RoleMenuItem, "Edit", rect(160, 140, 40, 30)),
		obs("uia:4", directorapi.RoleMenuItem, "View", rect(210, 140, 40, 30)),
		obs("uia:5", directorapi.RoleTextField, "Text editor", rect(100, 180, 800, 520)),
	}
}

// menuOpen is the same window with the File menu revealed.
func menuOpen() []directorapi.Observation {
	out := menuBar()
	return append(out,
		obs("uia:10", directorapi.RoleMenuItem, "New", rect(110, 175, 200, 28)),
		obs("uia:11", directorapi.RoleMenuItem, "Open", rect(110, 203, 200, 28)),
		obs("uia:12", directorapi.RoleMenuItem, "Save", rect(110, 231, 200, 28)),
		obs("uia:13", directorapi.RoleMenuItem, "Exit", rect(110, 259, 200, 28)),
	)
}

// fakeActuator records what it was asked to do rather than doing it.
type fakeActuator struct {
	clicks []directorapi.Point
	moves  []directorapi.Rect
	err    error
}

// fakeOperations stands in for marcoexec.Executor — the runner a semantic action's
// chosen mechanism lowers into.
//
// It records rather than performs, and it does NOT decide anything: the ladder has
// already chosen the mechanism by the time an operation arrives here, so a test that
// wants to check the choice looks at the operations recorded, not at this type.
type fakeOperations struct {
	ops []marcoexec.Operation
	err error
}

func (f *fakeOperations) Do(_ context.Context, op marcoexec.Operation) (marcoexec.Result, error) {
	f.ops = append(f.ops, op)
	if f.err != nil {
		return marcoexec.Result{Operation: op, Status: marcoexec.StatusRuntimeFailed}, f.err
	}
	return marcoexec.Result{Operation: op, Status: marcoexec.StatusCompleted}, nil
}

// kinds is what was run, for a test that wants to assert the lowering.
func (f *fakeOperations) kinds() []marcoexec.Kind {
	out := make([]marcoexec.Kind, 0, len(f.ops))
	for _, op := range f.ops {
		out = append(out, op.Kind)
	}
	return out
}

func (f *fakeActuator) Click(_ context.Context, at directorapi.Point, _ directorapi.MouseButton, _ int) error {
	if f.err != nil {
		return f.err
	}
	f.clicks = append(f.clicks, at)
	return nil
}
func (f *fakeActuator) Type(context.Context, string) error                         { return nil }
func (f *fakeActuator) TypeSecret(context.Context, string) error                   { return nil }
func (f *fakeActuator) Key(context.Context, string, time.Duration) error           { return nil }
func (f *fakeActuator) Move(context.Context, directorapi.Point) error              { return nil }
func (f *fakeActuator) Scroll(context.Context, directorapi.Point, int, bool) error { return nil }
func (f *fakeActuator) Activate(context.Context, string) error                     { return nil }
func (f *fakeActuator) Launch(context.Context, string) error                       { return nil }
func (f *fakeActuator) Drag(context.Context, directorapi.Point, directorapi.Point, directorapi.MouseButton) error {
	return nil
}
func (f *fakeActuator) MoveWindow(_ context.Context, _ directorapi.WindowID, to directorapi.Rect) error {
	if f.err != nil {
		return f.err
	}
	f.moves = append(f.moves, to)
	return nil
}
func (f *fakeActuator) SetWindowState(context.Context, directorapi.WindowID, directorapi.WindowState) error {
	return nil
}

// fakeFocuser records focus requests.
type fakeFocuser struct {
	targets []string
	err     error
}

func (f *fakeFocuser) Focus(_ context.Context, _ directorapi.WindowID, nativeID string) error {
	if f.err != nil {
		return f.err
	}
	f.targets = append(f.targets, nativeID)
	return nil
}

// harness wires a pipeline over a scripted sequence of worlds.
type harness struct {
	worlds     []directorapi.WorldState
	observed   int
	actuator   *fakeActuator
	focuser    *fakeFocuser
	operations *fakeOperations
	graph      *actiongraph.Memory
	pipeline   *Pipeline
}

// newHarness builds a pipeline that returns the given worlds in order, repeating the
// last one once exhausted.
func newHarness(worlds ...directorapi.WorldState) *harness {
	h := &harness{
		worlds:     worlds,
		actuator:   &fakeActuator{},
		focuser:    &fakeFocuser{},
		operations: &fakeOperations{},
		graph:      actiongraph.NewMemory(),
	}
	tick := 0
	h.pipeline = &Pipeline{
		Observe: func(context.Context) (directorapi.WorldState, error) {
			w := h.worlds[min(h.observed, len(h.worlds)-1)]
			h.observed++
			return w, nil
		},
		Intent:      intent.New().Parse,
		Resolver:    target.NewResolver(),
		Planner:     plan.New(),
		Policy:      testPolicy(),
		Verifier:    verify.New(),
		Graph:       h.graph,
		Monitors:    func(context.Context) ([]directorapi.Monitor, error) { return testMonitors, nil },
		SettleDelay: time.Millisecond,
		Now: func() time.Time {
			tick++
			return t0.Add(time.Duration(tick) * time.Second)
		},
	}
	h.pipeline.Executor = &Executor{
		Actuator:   h.actuator,
		Focus:      h.focuser,
		Operations: h.operations,
		Resolve: func(ref directorapi.ElementReference) (directorapi.ResolvedTarget, error) {
			w := h.worlds[min(h.observed-1, len(h.worlds)-1)]
			if el, ok := w.Element(ref.ID); ok {
				native, _ := el.Attributes["native_id"].(string)
				return directorapi.ResolvedTarget{
					ElementID: el.ID, WindowID: el.WindowID, Point: el.ClickPoint(),
					Role: el.Role, Label: el.Label, NativeID: native,
				}, nil
			}
			return directorapi.ResolvedTarget{}, errors.New("the target is no longer on screen")
		},
	}
	return h
}

// testPolicy uses a clock pinned near the scripted worlds so freshness never trips.
func testPolicy() *policy.Engine {
	e := policy.New()
	e.Now = func() time.Time { return t0 }
	return e
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── successful click ──────────────────────────────────────────────────────────

// Clicking File opens the File menu, and the verifier sees the menu items appear.
// This is the milestone's central claim, end to end.
func TestSuccessfulClick(t *testing.T) {
	h := newHarness(
		scene(t0, nil, menuBar()...),
		scene(t0.Add(time.Second), nil, menuOpen()...),
	)

	out := h.pipeline.Handle(context.Background(), "click File")

	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	if len(h.actuator.clicks) != 1 {
		t.Fatalf("want exactly one click, got %d", len(h.actuator.clicks))
	}
	// The click must land on the File menu item, not on a coordinate carried over
	// from planning.
	if got := h.actuator.clicks[0]; got != (directorapi.Point{X: 130, Y: 155}) {
		t.Errorf("click landed at %+v, want the centre of the File menu item", got)
	}
	if !out.Record.Verification.Success {
		t.Error("the menu opening should verify the click")
	}
	if !hasEvidence(out.Record.Verification, "menu_opened") {
		t.Errorf("want menu_opened evidence, got %v", kinds(out.Record.Verification))
	}
	if out.Retried {
		t.Error("a first-time success should not retry")
	}
	// And it must have become a durable semantic node.
	n, err := h.graph.Last()
	if err != nil {
		t.Fatalf("the action was not recorded: %v", err)
	}
	if n.Intent.Raw != "click File" || n.ResolvedTarget.Label != "File" {
		t.Errorf("the node is incomplete: %+v", n)
	}
	if n.Outcome.Before.Fingerprint == n.Outcome.After.Fingerprint {
		t.Error("before and after should fingerprint differently — the screen changed")
	}
	// The node's identity must be semantic, not positional.
	if spec, ok := n.Action(); !ok || spec.Query == nil {
		t.Error("the node must carry the query that found the target")
	}
	if n.SemanticKey() == "" {
		t.Error("the node needs a semantic key")
	}
	if out.Node == nil || out.Node.ID != n.ID {
		t.Error("the outcome should report the node it produced")
	}
}

// Chained actions link to what they followed on from — a parent is claimed only when
// the previous action left the screen in the state this one started from, which is
// what distinguishes a sequence from two things that merely happened in order.
func TestConsecutiveActionsChain(t *testing.T) {
	opened := scene(t0.Add(time.Second), nil, menuOpen()...)
	h := newHarness(
		scene(t0, nil, menuBar()...),
		opened,
		opened, // the second action starts from where the first left off
		scene(t0.Add(3*time.Second), nil, menuBar()...),
	)

	if out := h.pipeline.Handle(context.Background(), "click File"); out.Status != directorapi.ResultDone {
		t.Fatalf("first action: %s", out.Message)
	}
	if out := h.pipeline.Handle(context.Background(), "click Save"); out.Status != directorapi.ResultDone {
		t.Fatalf("second action: %s", out.Message)
	}

	nodes, _ := h.graph.Recent(0)
	if len(nodes) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(nodes))
	}
	newest, oldest := nodes[0], nodes[1]
	if newest.Parent == nil {
		t.Fatal("the second action should link to the first")
	}
	if *newest.Parent != oldest.ID {
		t.Errorf("parent = %s, want %s", *newest.Parent, oldest.ID)
	}
	if chain := h.graph.Chain(newest.ID); len(chain) != 2 {
		t.Errorf("the chain should hold both actions, got %d", len(chain))
	}
}

// ── successful focus ──────────────────────────────────────────────────────────

func TestSuccessfulFocus(t *testing.T) {
	before := scene(t0, nil,
		obs("uia:1", directorapi.RoleWindow, "Code", rect(0, 0, 1200, 800)),
		obs("uia:2", directorapi.RoleTab, "Explorer", rect(0, 35, 48, 48)),
		obs("uia:3", directorapi.RoleTab, "Search", rect(0, 90, 48, 48)),
	)
	after := scene(t0.Add(time.Second), nil,
		obs("uia:1", directorapi.RoleWindow, "Code", rect(0, 0, 1200, 800)),
		focused(obs("uia:2", directorapi.RoleTab, "Explorer", rect(0, 35, 48, 48))),
		obs("uia:3", directorapi.RoleTab, "Search", rect(0, 90, 48, 48)),
	)
	h := newHarness(before, after)

	out := h.pipeline.Handle(context.Background(), "focus Explorer")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	// Focus must go through the accessibility provider, never a click — clicking
	// would activate the control, which is a different action.
	if len(h.focuser.targets) != 1 {
		t.Fatalf("want one focus request, got %d", len(h.focuser.targets))
	}
	if h.focuser.targets[0] != "uia:2" {
		t.Errorf("focused %q, want the Explorer tab's native id", h.focuser.targets[0])
	}
	if len(h.actuator.clicks) != 0 {
		t.Error("focusing must not click anything")
	}
	if !hasEvidence(out.Record.Verification, "focus_on_target") {
		t.Errorf("want focus_on_target evidence, got %v", kinds(out.Record.Verification))
	}
}

// Without a provider that can set focus, the executor must fail rather than fall
// back to a click. A click would press the control.
func TestFocusWithoutAProviderRefusesRatherThanClicking(t *testing.T) {
	h := newHarness(scene(t0, nil,
		obs("uia:1", directorapi.RoleWindow, "Code", rect(0, 0, 1200, 800)),
		obs("uia:2", directorapi.RoleTab, "Explorer", rect(0, 35, 48, 48)),
	))
	h.pipeline.Executor.(*Executor).Focus = nil

	out := h.pipeline.Handle(context.Background(), "focus Explorer")
	if out.Status == directorapi.ResultDone {
		t.Fatal("focus should not succeed with no way to set focus")
	}
	if len(h.actuator.clicks) != 0 {
		t.Fatal("it must not substitute a click, which would activate the control")
	}
	if !strings.Contains(out.Record.Execution.Error, "activate") {
		t.Errorf("the error should explain why it refused, got %q", out.Record.Execution.Error)
	}
}

// ── successful window move ────────────────────────────────────────────────────

func TestSuccessfulWindowMove(t *testing.T) {
	start := directorapi.Window{
		ID: "hwnd:1", Application: "notepad", Title: "Untitled - Notepad",
		Bounds: rect(300, 200, 800, 600), Focused: true, Visible: true, MonitorID: "monitor:1",
	}
	moved := start
	moved.Bounds = rect(0, 0, 960, 1040) // the left half of monitor:1's work area

	h := newHarness(
		scene(t0, []directorapi.Window{start}, menuBar()...),
		scene(t0.Add(time.Second), []directorapi.Window{moved}, menuBar()...),
	)

	out := h.pipeline.Handle(context.Background(), "move window left")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	if len(h.actuator.moves) != 1 {
		t.Fatalf("want one move, got %d", len(h.actuator.moves))
	}
	// The destination is computed from the monitor's WORK area, so the window does
	// not end up half behind the taskbar.
	if got := h.actuator.moves[0]; got != rect(0, 0, 960, 1040) {
		t.Errorf("moved to %+v, want the left half of the work area", got)
	}
	if !hasEvidence(out.Record.Verification, "window_at_requested_bounds") {
		t.Errorf("want the bounds checked against the request, got %v", kinds(out.Record.Verification))
	}
	// A window move is the one action in this milestone that can be undone, and the
	// undo must restore the ORIGINAL rectangle.
	if !out.Record.Reversible {
		t.Fatal("a window move should be reversible")
	}
	undo, ok := out.Record.UndoAction.(directorapi.MoveWindowAction)
	if !ok || undo.Placement.Bounds == nil {
		t.Fatal("the undo action should be a move with explicit bounds")
	}
	if *undo.Placement.Bounds != start.Bounds {
		t.Errorf("undo targets %+v, want the original %+v", *undo.Placement.Bounds, start.Bounds)
	}
}

// ── verification failure ──────────────────────────────────────────────────────

// A click that changes nothing must be reported as a failure. This is the case a
// system without verification gets wrong: the click was sent, no error came back,
// and nothing happened.
func TestVerificationFailureWhenNothingChanges(t *testing.T) {
	still := menuBar()
	h := newHarness(
		scene(t0, nil, still...),
		scene(t0.Add(time.Second), nil, still...),
		scene(t0.Add(2*time.Second), nil, still...),
		scene(t0.Add(3*time.Second), nil, still...),
	)

	out := h.pipeline.Handle(context.Background(), "click File")
	if out.Status == directorapi.ResultDone {
		t.Fatal("a click that changed nothing must not be reported as success")
	}
	if out.Record.Execution.Performed != true {
		t.Error("the click WAS sent — that is precisely why verification is needed")
	}
	if out.Record.Verification.Success {
		t.Error("verification should have failed")
	}
	if !hasEvidence(out.Record.Verification, "nothing_changed") {
		t.Errorf("want nothing_changed evidence, got %v", kinds(out.Record.Verification))
	}
}

// ── retry succeeds ────────────────────────────────────────────────────────────

// The first click misses because the menu had not rendered; re-observing and trying
// once more works. Exactly one retry is permitted.
func TestRetrySucceeds(t *testing.T) {
	h := newHarness(
		scene(t0, nil, menuBar()...),                     // observe
		scene(t0.Add(1*time.Second), nil, menuBar()...),  // after attempt 1: unchanged
		scene(t0.Add(2*time.Second), nil, menuBar()...),  // re-observe for the retry
		scene(t0.Add(3*time.Second), nil, menuOpen()...), // after attempt 2: menu open
	)

	out := h.pipeline.Handle(context.Background(), "click File")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("the retry should have succeeded: %s (%s)", out.Status, out.Message)
	}
	if !out.Retried {
		t.Error("the outcome should record that it retried")
	}
	if len(h.actuator.clicks) != 2 {
		t.Errorf("want two clicks (original plus one retry), got %d", len(h.actuator.clicks))
	}
	if out.Record.Attempts != 2 {
		t.Errorf("the record should show 2 attempts, got %d", out.Record.Attempts)
	}
	// One record per REQUEST, not per attempt: history is a log of what was asked
	// for, with the retry visible in Attempts.
	if h.graph.Len() != 1 {
		t.Errorf("want one history record for one request, got %d", h.graph.Len())
	}
}

// ── retry fails ───────────────────────────────────────────────────────────────

// After one retry, stop. An agent that keeps trying is how a stuck UI turns into a
// hundred clicks.
func TestRetryFailsAndStops(t *testing.T) {
	still := menuBar()
	h := newHarness(
		scene(t0, nil, still...),
		scene(t0.Add(1*time.Second), nil, still...),
		scene(t0.Add(2*time.Second), nil, still...),
		scene(t0.Add(3*time.Second), nil, still...),
		scene(t0.Add(4*time.Second), nil, still...),
		scene(t0.Add(5*time.Second), nil, still...),
	)

	out := h.pipeline.Handle(context.Background(), "click File")
	if out.Status == directorapi.ResultDone {
		t.Fatal("nothing ever changed; this must not report success")
	}
	if len(h.actuator.clicks) != 2 {
		t.Fatalf("want exactly 2 clicks — one retry and then stop — got %d", len(h.actuator.clicks))
	}
	if out.Record.Attempts != 2 {
		t.Errorf("want 2 attempts recorded, got %d", out.Record.Attempts)
	}
	if out.Record.FailureReason == "" {
		t.Error("a failed action must say why")
	}
	// A failure is still recorded. History that only holds successes cannot explain
	// anything.
	if h.graph.Len() != 1 {
		t.Errorf("the failure should be recorded, got %d records", h.graph.Len())
	}
	if _, ok := h.graph.LastSuccessful(); ok {
		t.Error("a failed action must not be offered as the last successful one")
	}
}

// Regression, found running against a live Chrome. Clicking Back navigated the
// page, but the navigation had not finished within the settle delay, so
// verification failed and the retry clicked Back again — sending the browser two
// pages back when one was asked for.
//
// Most actions are not idempotent, and "I could not confirm it" is not the same as
// "it did not happen". A retry is permitted only when the screen is unchanged.
func TestNoRetryWhenTheScreenChangedButDidNotVerify(t *testing.T) {
	before := menuBar()
	// Something changed — an unrelated element appeared — but nothing that
	// constitutes evidence the click landed on its target.
	after := append(append([]directorapi.Observation(nil), before...),
		obs("uia:99", directorapi.RoleText, "loading", rect(400, 400, 60, 16)))

	h := newHarness(
		scene(t0, nil, before...),
		scene(t0.Add(time.Second), nil, after...),
		scene(t0.Add(2*time.Second), nil, after...),
		scene(t0.Add(3*time.Second), nil, after...),
	)

	out := h.pipeline.Handle(context.Background(), "click File")

	if len(h.actuator.clicks) != 1 {
		t.Fatalf("want exactly ONE click — the screen changed, so a retry could "+
			"double-apply — got %d", len(h.actuator.clicks))
	}
	if out.Retried {
		t.Error("it must not report having retried")
	}
	// And the honest verdict is "unconfirmed", not "failed": telling the user it
	// failed invites them to do it again, which is the same double-application.
	if out.Status != directorapi.ResultPartial {
		t.Errorf("status = %s, want partial (performed but unconfirmed)", out.Status)
	}
	if out.Record.Status != directorapi.ActionUnverified {
		t.Errorf("record status = %s, want unverified", out.Record.Status)
	}
}

// The complement: when NOTHING changed, the action provably did not land, so
// retrying cannot double-apply and is the right move.
func TestRetryStillHappensWhenNothingChanged(t *testing.T) {
	still := menuBar()
	h := newHarness(
		scene(t0, nil, still...),
		scene(t0.Add(1*time.Second), nil, still...),
		scene(t0.Add(2*time.Second), nil, still...),
		scene(t0.Add(3*time.Second), nil, menuOpen()...),
	)
	out := h.pipeline.Handle(context.Background(), "click File")
	if !out.Retried {
		t.Fatal("an unchanged screen means the action did not land; it should retry")
	}
	if out.Status != directorapi.ResultDone {
		t.Errorf("the retry should have succeeded: %s", out.Message)
	}
}

// ── wrong candidate rejected ──────────────────────────────────────────────────

// The live VS Code case: the exact label match is inert text and the real control is
// a tab matching on substring. The pipeline must act on the tab.
func TestWrongCandidateRejected(t *testing.T) {
	before := scene(t0, nil,
		obs("uia:1", directorapi.RoleWindow, "Code", rect(0, 0, 1200, 800)),
		obs("uia:2", directorapi.RoleText, "EXPLORER", rect(68, 35, 51, 35)),
		obs("uia:3", directorapi.RoleTab, "Explorer (Ctrl+Shift+E)", rect(0, 35, 48, 48)),
	)
	after := scene(t0.Add(time.Second), nil,
		obs("uia:1", directorapi.RoleWindow, "Code", rect(0, 0, 1200, 800)),
		obs("uia:2", directorapi.RoleText, "EXPLORER", rect(68, 35, 51, 35)),
		focused(obs("uia:3", directorapi.RoleTab, "Explorer (Ctrl+Shift+E)", rect(0, 35, 48, 48))),
	)
	h := newHarness(before, after)

	out := h.pipeline.Handle(context.Background(), "click Explorer")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	if out.Record.Target.Role != directorapi.RoleTab {
		t.Errorf("acted on a %s; the actionable tab must outrank the inert exact-label text",
			out.Record.Target.Role)
	}
	// The rejection has to be visible in the record, not merely implied by the
	// winner: the planner and the user both need to see what was considered.
	var sawRejected bool
	for _, c := range out.Resolution.Candidates {
		if c.Label == "EXPLORER" && c.Rejected != "" {
			sawRejected = true
		}
	}
	if !sawRejected {
		t.Error("the inert candidate should be recorded as rejected, with a reason")
	}
}

// ── stale target ──────────────────────────────────────────────────────────────

// The element vanishes between planning and executing. The executor must fail
// cleanly rather than clicking where it used to be.
func TestStaleTargetIsNotClickedAtItsOldPosition(t *testing.T) {
	h := newHarness(scene(t0, nil, menuBar()...))
	// Resolution at execution time finds nothing: the world moved on.
	h.pipeline.Executor.(*Executor).Resolve = func(directorapi.ElementReference) (directorapi.ResolvedTarget, error) {
		return directorapi.ResolvedTarget{}, errors.New("the target is no longer on screen")
	}

	out := h.pipeline.Handle(context.Background(), "click File")
	if out.Status == directorapi.ResultDone {
		t.Fatal("a vanished target must not be reported as clicked")
	}
	if len(h.actuator.clicks) != 0 {
		t.Fatal("nothing should have been clicked — coordinate replay is exactly the bug")
	}
	if !strings.Contains(out.Record.FailureReason, "no longer on screen") {
		t.Errorf("the failure should name the cause, got %q", out.Record.FailureReason)
	}
}

// A snapshot that is not newer than the one before it proves nothing. Verifying
// against it would "confirm" anything.
func TestStaleSnapshotCannotVerify(t *testing.T) {
	v := verify.New()
	w := scene(t0, nil, menuBar()...)
	res := v.Verify(
		directorapi.ClickAction{Target: directorapi.ElementReference{ID: "e1"}},
		directorapi.ResolvedTarget{ElementID: "e1"}, w, w)
	if res.Success {
		t.Fatal("comparing a snapshot with itself must not verify anything")
	}
	if !res.Inconclusive {
		t.Error("it is inconclusive, not a failure — nothing was actually checked")
	}
}

// ── policy and unresolvable requests ──────────────────────────────────────────

// A world the Director cannot see into must stop the pipeline before anything is
// executed.
func TestBlindWorldNeverExecutes(t *testing.T) {
	h := newHarness(scene(t0, nil,
		obs("uia:1", directorapi.RoleWindow, "Discord", rect(0, 0, 1200, 800)),
		obs("uia:2", directorapi.RolePane, "", rect(0, 0, 1200, 800)),
		obs("uia:3", directorapi.RolePane, "", rect(0, 40, 1200, 760)),
	))

	out := h.pipeline.Handle(context.Background(), "click Send")
	if out.Status == directorapi.ResultDone {
		t.Fatal("nothing should succeed in a window with nothing observable")
	}
	if len(h.actuator.clicks) != 0 {
		t.Fatal("nothing should have been clicked")
	}
	if h.graph.Len() != 0 {
		t.Error("no action was attempted, so nothing should be recorded")
	}
}

// A destructive-looking button needs confirmation, and a Director with no way to ask
// must stop rather than proceed.
//
//	nil confirmer → unavailable, and no action may execute after it.
//
// BLOCKED rather than "needs confirmation": the request was reasonable and the Director
// could not carry it out safely, which is a different problem with a different fix than
// the user declining. See TestADirectHighRiskActionIsConfirmedAndExecuted for the wired
// case.
func TestDestructiveClickStopsForConfirmation(t *testing.T) {
	h := newHarness(scene(t0, nil,
		obs("uia:1", directorapi.RoleWindow, "Mail", rect(0, 0, 800, 600)),
		obs("uia:2", directorapi.RoleButton, "Delete", rect(10, 10, 80, 24)),
		obs("uia:3", directorapi.RoleButton, "Cancel", rect(100, 10, 80, 24)),
		obs("uia:4", directorapi.RoleTextField, "Subject", rect(10, 50, 200, 24)),
	))

	out := h.pipeline.Handle(context.Background(), "click Delete")
	if out.Status != directorapi.ResultBlocked {
		t.Fatalf("status = %s; a destructive click with no way to ask must be blocked", out.Status)
	}
	if len(h.actuator.clicks) != 0 {
		t.Fatal("nothing should have been clicked without confirmation")
	}
}

func TestUnknownRequestAsksRatherThanGuessing(t *testing.T) {
	h := newHarness(scene(t0, nil, menuBar()...))
	out := h.pipeline.Handle(context.Background(), "frobnicate the widget")
	if out.Status != directorapi.ResultNeedsClarification {
		t.Fatalf("status = %s; an unparsed request must ask, not guess", out.Status)
	}
	if len(h.actuator.clicks) != 0 {
		t.Fatal("nothing should have been clicked")
	}
}

func TestStopIsImmediate(t *testing.T) {
	h := newHarness(scene(t0, nil, menuBar()...))
	out := h.pipeline.Handle(context.Background(), "stop")
	if out.Status != directorapi.ResultCancelled {
		t.Errorf("status = %s, want cancelled", out.Status)
	}
	if h.observed != 0 {
		t.Error("stop should not even observe")
	}
}

// ── history ───────────────────────────────────────────────────────────────────

// The record has to hold everything the later "do that again" / "undo" commands will
// need, even though none of them is implemented yet. History that was not captured
// cannot be reconstructed.
func TestRecordHoldsWhatLaterCommandsWillNeed(t *testing.T) {
	h := newHarness(
		scene(t0, nil, menuBar()...),
		scene(t0.Add(time.Second), nil, menuOpen()...),
	)
	out := h.pipeline.Handle(context.Background(), "click File")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("precondition: %s", out.Message)
	}
	r := out.Record

	if r.ID == "" {
		t.Error("every action needs an id")
	}
	if r.Action == nil {
		t.Error("the SEMANTIC action must be kept — a repeat re-resolves, it does not replay")
	}
	if r.Target.Query == nil {
		t.Error("the query must be kept so a repeat can find the target afresh")
	}
	if r.Before.Fingerprint == "" || r.After.Fingerprint == "" {
		t.Error("before and after summaries are needed to reason about what changed")
	}
	if r.CompletedAt == nil || r.Duration() <= 0 {
		t.Error("timing should be recorded")
	}
	if len(r.Verification.Evidence) == 0 {
		t.Error("the evidence behind the verdict must be kept")
	}
	if r.Execution.Point == nil {
		t.Error("where the action landed is evidence and should be recorded")
	}

	// A click cannot be un-clicked, and claiming otherwise would make "undo"
	// dangerous the moment it is built.
	if r.Reversible {
		t.Error("a click must not be marked reversible")
	}
}

// ── helpers ──

func hasEvidence(v directorapi.VerificationResult, kind string) bool {
	for _, e := range v.Evidence {
		if e.Kind == kind && e.Observed {
			return true
		}
	}
	return false
}

func kinds(v directorapi.VerificationResult) []string {
	out := make([]string, 0, len(v.Evidence))
	for _, e := range v.Evidence {
		out = append(out, e.Kind)
	}
	return out
}
