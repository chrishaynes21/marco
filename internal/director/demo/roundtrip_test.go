package demo_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
	editproviders "github.com/chaynes-simpleclouds/marco/internal/director/edit/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	dintent "github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/internal/recorded"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/plan"
	"github.com/chaynes-simpleclouds/marco/internal/director/policy"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/internal/director/verify"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The whole loop, without a desktop.
//
//	A user performs a task once. The Director observes. Instead of recording
//	coordinates, timing, windows and keystrokes, it extracts the semantic procedure.
//
// This drives the REAL pipeline over scripted worlds: observe, resolve, plan, policy,
// execute, re-observe, verify, record. The recorder subscribes to what comes out, exactly
// as the daemon does, and what it produces goes through extraction, approval and the
// registry — and is then EXPANDED again for a different file.
//
// It is the closest thing to the live validation that can run in CI, and it is what makes
// "learned procedures execute through the existing pipeline" a fact rather than a claim
// about the shape of a struct.

// ── the scripted desktop ──────────────────────────────────────────────────────

func rect(x, y, w, h int) directorapi.Rect {
	return directorapi.Rect{X: x, Y: y, Width: w, Height: h}
}

// obs builds one observation, with the attributes the bridge reports.
func obs(id string, role directorapi.ElementRole, label, value, class string,
	r directorapi.Rect) directorapi.Observation {

	enabled, visible, focused, selected := true, true, false, false
	return directorapi.Observation{
		ID: directorapi.ObservationID("acc:" + id), Source: directorapi.SourceAccessibility,
		WindowID: "hwnd:1", Role: role, Label: label, Value: value, Bounds: r,
		Enabled: &enabled, Visible: &visible, Focused: &focused, Selected: &selected,
		Confidence: 1, NativeID: id,
		Attributes: map[string]any{"class_name": class},
	}
}

func chosen(o directorapi.Observation) directorapi.Observation {
	yes := true
	o.Selected, o.Focused = &yes, &yes
	return o
}

// file is a shell item with the path behind it, which is what a binding needs.
func file(id, name, path string) directorapi.Observation {
	o := obs(id, directorapi.RoleListItem, name, name, "UIItem", rect(10, 40, 200, 24))
	o.Resource = &directorapi.ResourceIdentity{
		Kind: directorapi.ResourceFile, Path: path, DisplayName: name,
		Source: "shell_folder_view", Confidence: 1,
	}
	return o
}

var explorerWindow = []directorapi.Window{{
	ID: "hwnd:1", Application: "explorer", Title: "work",
	Bounds: rect(0, 0, 900, 700), Focused: true, Visible: true, MonitorID: "monitor:1",
}}

// desktop builds one world from a set of observations.
func desktop(at time.Time, obs ...directorapi.Observation) directorapi.WorldState {
	id := explorerWindow[0].ID
	return recorded.NewBuilder().Build(recorded.Perception{
		Timestamp: at, Observations: obs, Windows: explorerWindow, ActiveWindow: &id,
		ActiveApp: &directorapi.Application{ID: "explorer", Name: "explorer"},
		Monitors: []directorapi.Monitor{{
			ID: "monitor:1", Bounds: rect(0, 0, 1920, 1080),
			WorkArea: rect(0, 0, 1920, 1040), Primary: true,
		}},
	})
}

// idle is Explorer with one file selected and the command bar showing.
func idle(at time.Time, selected, path string) directorapi.WorldState {
	return desktop(at,
		obs("uia:win", directorapi.RoleWindow, "work", "", "CabinetWClass", rect(0, 0, 900, 700)),
		chosen(file("uia:item", selected, path)),
		obs("uia:rename", directorapi.RoleButton, "Rename", "", "AppBarButton", rect(400, 8, 90, 30)),
	)
}

// renaming is the same window with the in-place editor open on the item.
//
// The item stays SELECTED and loses FOCUS, which is what Explorer does: the editor takes
// keyboard focus, and the row underneath it is still the selected one. It matters here
// because the focus move is the evidence that verifies the invoke.
func renaming(at time.Time, selected, path, editorValue string) directorapi.WorldState {
	editor := obs("uia:editor", directorapi.RoleTextField, editorValue, editorValue,
		"UIRenameTextElement", rect(10, 40, 200, 24))
	yes := true
	editor.Focused = &yes

	item := file("uia:item", selected, path)
	item.Selected = &yes

	return desktop(at,
		obs("uia:win", directorapi.RoleWindow, "work", "", "CabinetWClass", rect(0, 0, 900, 700)),
		item,
		obs("uia:rename", directorapi.RoleButton, "Rename", "", "AppBarButton", rect(400, 8, 90, 30)),
		editor,
	)
}

// ── the harness ───────────────────────────────────────────────────────────────

// stubActuator performs nothing and records nothing. Every action in this flow is
// carried out through the operations runner or the editor.
type stubActuator struct{}

func (stubActuator) Click(context.Context, directorapi.Point, directorapi.MouseButton, int) error {
	return nil
}
func (stubActuator) Type(context.Context, string) error                         { return nil }
func (stubActuator) TypeSecret(context.Context, string) error                   { return nil }
func (stubActuator) Key(context.Context, string, time.Duration) error           { return nil }
func (stubActuator) Move(context.Context, directorapi.Point) error              { return nil }
func (stubActuator) Scroll(context.Context, directorapi.Point, int, bool) error { return nil }
func (stubActuator) Activate(context.Context, string) error                     { return nil }
func (stubActuator) Launch(context.Context, string) error                       { return nil }
func (stubActuator) Drag(context.Context, directorapi.Point, directorapi.Point, directorapi.MouseButton) error {
	return nil
}
func (stubActuator) MoveWindow(context.Context, directorapi.WindowID, directorapi.Rect) error {
	return nil
}
func (stubActuator) SetWindowState(context.Context, directorapi.WindowID, directorapi.WindowState) error {
	return nil
}

type stubFocus struct{}

func (stubFocus) Focus(context.Context, directorapi.WindowID, string) error { return nil }

type stubOperations struct{ ops []marcoexec.Operation }

func (s *stubOperations) Do(_ context.Context, op marcoexec.Operation) (marcoexec.Result, error) {
	s.ops = append(s.ops, op)
	return marcoexec.Result{Operation: op, Status: marcoexec.StatusCompleted}, nil
}

// stubTextEditor reports the edit as having landed, so the WORLD decides whether it did.
type stubTextEditor struct{ wrote []string }

func (s *stubTextEditor) Apply(_ context.Context, _ editproviders.Target, op edit.Operation) (
	edit.Outcome, error) {

	text := ""
	if set, ok := op.(edit.SetText); ok {
		text = set.Value
	}
	s.wrote = append(s.wrote, text)
	return edit.Outcome{
		Before: "before", BeforeKnown: true, After: text, AfterKnown: true,
		Strategy: edit.StrategyValueAPI, Operation: op.ID(),
	}, nil
}

// director wires a pipeline over a scripted sequence of worlds.
type director struct {
	worlds   []directorapi.WorldState
	observed int
	pipeline *execute.Pipeline
	graph    *actiongraph.Memory
	ops      *stubOperations
	editor   *stubTextEditor
	registry *goal.Registry
}

func newDirector(t *testing.T, worlds ...directorapi.WorldState) *director {
	t.Helper()
	d := &director{
		worlds: worlds, graph: actiongraph.NewMemory(),
		ops: &stubOperations{}, editor: &stubTextEditor{},
		registry: goal.NewRegistry(),
	}
	observe := func(context.Context) (directorapi.WorldState, error) {
		i := d.observed
		if i >= len(d.worlds) {
			i = len(d.worlds) - 1
		}
		d.observed++
		return d.worlds[i], nil
	}
	pol := policy.New()
	pol.Now = func() time.Time { return t1 }
	tick := 0
	d.pipeline = &execute.Pipeline{
		Observe:  observe,
		Intent:   dintent.New().Parse,
		Resolver: target.NewResolver(),
		Planner:  plan.New(),
		Policy:   pol,
		Verifier: verify.New(),
		Graph:    d.graph,
		Goals: &execute.Goals{
			Registry:    d.registry,
			Application: func() string { return "explorer" },
		},
		SettleDelay: time.Millisecond,
		Now: func() time.Time {
			tick++
			return t1.Add(time.Duration(tick) * time.Second)
		},
		Executor: &execute.Executor{
			Actuator: stubActuator{}, Focus: stubFocus{},
			Operations: d.ops, Editor: d.editor,
			Resolve: func(ref directorapi.ElementReference) (directorapi.ResolvedTarget, error) {
				w := d.worlds[min(d.observed-1, len(d.worlds)-1)]
				if el, ok := w.Element(ref.ID); ok {
					native, _ := el.Attributes["native_id"].(string)
					return directorapi.ResolvedTarget{
						ElementID: el.ID, WindowID: el.WindowID, Point: el.ClickPoint(),
						Role: el.Role, Label: el.Label, NativeID: native,
					}, nil
				}
				return directorapi.ResolvedTarget{}, errNotOnScreen
			},
		},
	}
	return d
}

var t1 = time.Date(2026, 8, 5, 15, 0, 0, 0, time.UTC)

var errNotOnScreen = errStr("the target is no longer on screen")

type errStr string

func (e errStr) Error() string { return string(e) }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── the loop ──────────────────────────────────────────────────────────────────

// TestADemonstratedRenameBecomesAProcedureThatRunsOnAnotherFile.
//
// The milestone's Definition of Done, in one test:
//
//	a user demonstrates once → the Director records SEMANTICS → a goal is recovered →
//	the typed value becomes a parameter → the proposal is approved → the procedure
//	enters the ordinary registry → it expands and runs for a DIFFERENT file
func TestADemonstratedRenameBecomesAProcedureThatRunsOnAnotherFile(t *testing.T) {
	const alpha = `C:\work\Alpha.txt`
	// Two worlds per request: the one it plans against and the one it verifies against.
	d := newDirector(t,
		// select Alpha.txt
		idle(t1, "Alpha.txt", alpha),
		idle(t1.Add(1*time.Second), "Alpha.txt", alpha),
		// click Rename → the editor opens and takes focus
		idle(t1.Add(2*time.Second), "Alpha.txt", alpha),
		renaming(t1.Add(3*time.Second), "Alpha.txt", alpha, "Alpha.txt"),
		// type Budget → the editor holds the new name
		renaming(t1.Add(4*time.Second), "Alpha.txt", alpha, "Alpha.txt"),
		renaming(t1.Add(5*time.Second), "Alpha.txt", alpha, "Budget"),
	)

	rec := demo.NewRecorder()
	if _, err := rec.Start("demo-live"); err != nil {
		t.Fatalf("start: %v", err)
	}

	// The user performs the task once, driving the Director the way a user does — by
	// asking for each thing. Every request is observed, planned, executed, re-observed
	// and VERIFIED before the recorder ever sees it.
	for _, phrase := range []string{
		"select Alpha.txt", "click Rename", `type "Budget"`,
	} {
		out := d.pipeline.Handle(context.Background(), phrase)
		if out.Status != directorapi.ResultDone {
			t.Fatalf("%q: %s — %s", phrase, out.Status, out.Message)
		}
		rec.Observe(out)
	}

	full, err := rec.Stop()
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if full.Status != demo.Completed {
		t.Fatalf("the session was not usable: %s — %s", full.Status, full.Refusal)
	}
	if len(full.Steps) != 3 {
		t.Fatalf("%d steps recorded: %+v", len(full.Steps), full.Steps)
	}
	// It recorded SEMANTICS. The rename command was recovered as a role, which is what
	// makes the learned procedure work on a machine in another language.
	if !full.HasRole(goal.RoleRenameCommand) {
		t.Fatalf("the rename command was not recognised: %+v", full.Steps)
	}

	// ── extract ───────────────────────────────────────────────────────────────
	ex := demo.Extract(full)
	if !ex.OK() {
		t.Fatalf("nothing was extracted from a clean demonstration: %s", ex.Refusal)
	}
	c := ex.Candidate
	if c.Goal != goal.Rename {
		t.Fatalf("recovered %q, want rename", c.Goal)
	}
	if len(c.Parameters) != 1 || c.Parameters[0].Name != "new_name" {
		t.Fatalf("parameters = %+v, want one called new_name", c.Parameters)
	}
	if c.Parameters[0].Example != "Budget" {
		t.Errorf("the demonstrated value was not kept for review: %q", c.Parameters[0].Example)
	}

	// ── approve ───────────────────────────────────────────────────────────────
	l, err := demo.Approve(ex, "the user", t1)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	d.registry.Register(l.AsProcedure())

	// ── run it, on a DIFFERENT file, with a DIFFERENT name ─────────────────────
	g := goal.Goal{
		Kind:       goal.Rename,
		Parameters: map[string]string{goal.ParamName: "Q4"},
		Context: goal.Context{
			Application: "explorer", Target: "this file", TargetIsImplicit: true,
		},
		Phrase: "rename this file to Q4",
	}
	learnedEx, err := goal.Expand(d.registry, g, fakeBinder{})
	if err != nil {
		t.Fatalf("the learned procedure did not expand: %v", err)
	}
	if learnedEx.Procedure != l.Name {
		t.Fatalf("procedure = %q, want the learned one", learnedEx.Procedure)
	}
	if err := program.Validate(learnedEx.Program); err != nil {
		t.Fatalf("the learned program is not an ordinary valid program: %v", err)
	}
	var typed string
	for _, s := range learnedEx.Program.Steps {
		if s.Operation.Verb == "edit" {
			typed = s.Operation.Text
		}
	}
	if typed != "Q4" {
		t.Fatalf("the learned procedure would type %q — the demonstrated value must "+
			"never be reused", typed)
	}
}

// TestTheRecordedStepsCarryNoMechanics — over a REAL pipeline, where every mechanism the
// Director uses is present and could have leaked.
func TestTheRecordedStepsCarryNoMechanics(t *testing.T) {
	const alpha = `C:\work\Alpha.txt`
	d := newDirector(t,
		idle(t1, "Alpha.txt", alpha), idle(t1.Add(time.Second), "Alpha.txt", alpha),
		idle(t1.Add(2*time.Second), "Alpha.txt", alpha),
		renaming(t1.Add(3*time.Second), "Alpha.txt", alpha, "Alpha.txt"),
	)
	rec := demo.NewRecorder()
	_, _ = rec.Start("demo-mechanics")
	for _, phrase := range []string{"select Alpha.txt", "click Rename"} {
		rec.Observe(d.pipeline.Handle(context.Background(), phrase))
	}
	session, _ := rec.Stop()

	raw := mustJSON(t, session)
	for _, forbidden := range []string{"hwnd:1", "uia:item", "uia:rename", `"point"`, `"bounds"`} {
		if strings.Contains(strings.ToLower(raw), strings.ToLower(forbidden)) {
			t.Errorf("the demonstration kept %q:\n%s", forbidden, raw)
		}
	}
	// And it DID keep the semantics.
	if !strings.Contains(raw, "rename_command") {
		t.Errorf("the demonstration kept no semantic role:\n%s", raw)
	}
	if !strings.Contains(raw, "action_") {
		t.Errorf("the demonstration references no action-graph node:\n%s", raw)
	}
}
