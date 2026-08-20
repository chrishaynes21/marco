package actiongraph

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/internal/recorded"
	"github.com/chaynes-simpleclouds/marco/internal/director/policy"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Analysis executes nothing. Every test here asks only whether an action COULD be
// done now, which is why they all run against constructed worlds and never touch a
// desktop.

func obs(id string, role directorapi.ElementRole, label string, r directorapi.Rect) directorapi.Observation {
	enabled, visible, focus, sel := true, true, false, false
	return directorapi.Observation{
		ID: directorapi.ObservationID("acc:" + id), Source: directorapi.SourceAccessibility,
		WindowID: "hwnd:1", Role: role, Label: label, Bounds: r,
		Enabled: &enabled, Visible: &visible, Focused: &focus, Selected: &sel,
		Confidence: 1, NativeID: id,
	}
}

// scene builds a world for one application.
func scene(app string, obs ...directorapi.Observation) directorapi.WorldState {
	id := directorapi.WindowID("hwnd:1")
	return recorded.NewBuilder().Build(recorded.Perception{
		Timestamp:    t0,
		Observations: obs,
		Windows: []directorapi.Window{{
			ID: id, Application: app, Title: "Untitled - " + app,
			Bounds: rect(100, 100, 800, 600), Focused: true, Visible: true,
			MonitorID: "monitor:1",
		}},
		ActiveWindow: &id,
		ActiveApp:    &directorapi.Application{ID: app, Name: app},
		Monitors: []directorapi.Monitor{
			{ID: "monitor:1", Bounds: rect(0, 0, 1920, 1080), WorkArea: rect(0, 0, 1920, 1040), Primary: true},
		},
	})
}

// notepadScene is the shape a live Notepad presents.
func notepadScene() directorapi.WorldState {
	return scene("notepad",
		obs("uia:1", directorapi.RoleWindow, "Untitled - notepad", rect(100, 100, 800, 600)),
		obs("uia:2", directorapi.RoleMenuItem, "File", rect(110, 140, 40, 30)),
		obs("uia:3", directorapi.RoleMenuItem, "Edit", rect(160, 140, 40, 30)),
		obs("uia:4", directorapi.RoleTextField, "Text editor", rect(100, 180, 800, 520)),
	)
}

// fileNode is a recorded "click File in Notepad" whose target sits where
// notepadScene puts it.
func fileNode() ActionNode {
	n := node("action_1", "notepad", "File", rect(110, 140, 40, 30))
	return n
}

// ── READY ─────────────────────────────────────────────────────────────────────

func TestReadyWhenTheTargetIsWhereItWas(t *testing.T) {
	c := AnalyzeReplay(fileNode(), notepadScene())
	if c.Status != ReplayReady {
		t.Fatalf("status = %s (%s), want READY", c.Status, c.Reason)
	}
	if !c.Replayable {
		t.Error("READY must be replayable")
	}
	if c.CurrentTarget == nil {
		t.Error("a ready candidate should report where the target is now")
	}
	// The current target comes from a FRESH resolution, never from the snapshot.
	if c.CurrentTarget != nil && c.CurrentTarget.Label != "File" {
		t.Errorf("current target = %q", c.CurrentTarget.Label)
	}
}

// ── TARGET_MOVED ──────────────────────────────────────────────────────────────

// A moved target is still replayable — replay re-resolves rather than replaying
// coordinates — but the move is worth reporting.
func TestTargetMovedIsStillReplayable(t *testing.T) {
	moved := scene("notepad",
		obs("uia:1", directorapi.RoleWindow, "Untitled - notepad", rect(500, 400, 800, 600)),
		obs("uia:2", directorapi.RoleMenuItem, "File", rect(510, 440, 40, 30)),
		obs("uia:3", directorapi.RoleMenuItem, "Edit", rect(560, 440, 40, 30)),
		obs("uia:4", directorapi.RoleTextField, "Text editor", rect(500, 480, 800, 520)),
	)
	c := AnalyzeReplay(fileNode(), moved)
	if c.Status != ReplayTargetMoved {
		t.Fatalf("status = %s (%s), want TARGET_MOVED", c.Status, c.Reason)
	}
	if !c.Replayable {
		t.Error("a moved target is still replayable: replay re-resolves")
	}
	if c.Moved == nil || (c.Moved.DX == 0 && c.Moved.DY == 0) {
		t.Errorf("the displacement should be reported, got %+v", c.Moved)
	}
}

// A shift of a pixel or two is not "moving" in any sense a user would recognise.
func TestTinyShiftsAreStillReady(t *testing.T) {
	nudged := scene("notepad",
		obs("uia:1", directorapi.RoleWindow, "Untitled - notepad", rect(100, 100, 800, 600)),
		obs("uia:2", directorapi.RoleMenuItem, "File", rect(112, 141, 40, 30)),
		obs("uia:3", directorapi.RoleMenuItem, "Edit", rect(162, 141, 40, 30)),
		obs("uia:4", directorapi.RoleTextField, "Text editor", rect(100, 180, 800, 520)),
	)
	if c := AnalyzeReplay(fileNode(), nudged); c.Status != ReplayReady {
		t.Errorf("a two-pixel nudge should still be READY, got %s", c.Status)
	}
}

// ── TARGET_MISSING ────────────────────────────────────────────────────────────

func TestTargetMissingWhenTheWorldIsReadableAndItIsGone(t *testing.T) {
	gone := scene("notepad",
		obs("uia:1", directorapi.RoleWindow, "Untitled - notepad", rect(100, 100, 800, 600)),
		obs("uia:3", directorapi.RoleMenuItem, "Edit", rect(160, 140, 40, 30)),
		obs("uia:5", directorapi.RoleMenuItem, "View", rect(210, 140, 40, 30)),
		obs("uia:4", directorapi.RoleTextField, "Text editor", rect(100, 180, 800, 520)),
	)
	c := AnalyzeReplay(fileNode(), gone)
	if c.Status != ReplayTargetMissing {
		t.Fatalf("status = %s (%s), want TARGET_MISSING", c.Status, c.Reason)
	}
	if c.Replayable {
		t.Error("a missing target is not replayable")
	}
}

// ── TARGET_AMBIGUOUS ──────────────────────────────────────────────────────────

func TestTargetAmbiguousWhenSeveralNowMatch(t *testing.T) {
	twoFiles := scene("notepad",
		obs("uia:1", directorapi.RoleWindow, "Untitled - notepad", rect(100, 100, 800, 600)),
		obs("uia:2", directorapi.RoleMenuItem, "File", rect(110, 140, 40, 30)),
		obs("uia:9", directorapi.RoleMenuItem, "File", rect(400, 140, 40, 30)),
		obs("uia:4", directorapi.RoleTextField, "Text editor", rect(100, 180, 800, 520)),
	)
	c := AnalyzeReplay(fileNode(), twoFiles)
	if c.Status != ReplayTargetAmbiguous {
		t.Fatalf("status = %s (%s), want TARGET_AMBIGUOUS", c.Status, c.Reason)
	}
	if c.Replayable {
		t.Error("replaying a coin flip between two controls is not replay")
	}
	if len(c.Alternatives) == 0 {
		t.Error("the competing candidates should be reported")
	}
}

// ── APP_NOT_RUNNING ───────────────────────────────────────────────────────────

// "Notepad is closed" is a far more useful answer than "no menu item called File",
// so the application is checked before the target.
func TestAppNotRunning(t *testing.T) {
	elsewhere := scene("chrome",
		obs("uia:1", directorapi.RoleWindow, "Untitled - chrome", rect(100, 100, 800, 600)),
		obs("uia:2", directorapi.RoleButton, "Back", rect(110, 140, 40, 30)),
		obs("uia:3", directorapi.RoleButton, "Forward", rect(160, 140, 40, 30)),
	)
	c := AnalyzeReplay(fileNode(), elsewhere)
	if c.Status != ReplayAppNotRunning {
		t.Fatalf("status = %s (%s), want APP_NOT_RUNNING", c.Status, c.Reason)
	}
	if !strings.Contains(c.Reason, "notepad") {
		t.Errorf("the reason should name the application, got %q", c.Reason)
	}
}

// The live validation for this milestone closes Notepad and re-analyses. It must
// answer, not crash.
func TestAnalysisNeverPanicsOnAnEmptyWorld(t *testing.T) {
	for name, w := range map[string]directorapi.WorldState{
		"nothing at all": {},
		"no elements":    scene("notepad"),
	} {
		c := AnalyzeReplay(fileNode(), w)
		if c.Status == "" {
			t.Errorf("%s: analysis must always reach a verdict", name)
		}
		if c.Replayable {
			t.Errorf("%s: an empty world is not replayable", name)
		}
	}
}

// ── UNOBSERVABLE ──────────────────────────────────────────────────────────────

// The Discord shape. A world that could not be read must not be reported as the
// target being gone — that is the conflation the four-way resolution model exists to
// prevent, and re-introducing it here would undo it.
func TestUnobservableIsNotMissing(t *testing.T) {
	opaque := scene("notepad",
		obs("uia:1", directorapi.RoleWindow, "Untitled - notepad", rect(0, 0, 1200, 800)),
		obs("uia:2", directorapi.RolePane, "", rect(0, 0, 1200, 800)),
		obs("uia:3", directorapi.RolePane, "", rect(0, 40, 1200, 760)),
		obs("uia:4", directorapi.RolePane, "", rect(240, 40, 960, 760)),
	)
	c := AnalyzeReplay(fileNode(), opaque)
	if c.Status == ReplayTargetMissing {
		t.Fatal("a window that exposes nothing must not report the target as gone")
	}
	if c.Status != ReplayUnobservable {
		t.Fatalf("status = %s (%s), want UNOBSERVABLE", c.Status, c.Reason)
	}
	if !strings.Contains(c.Reason, "not evidence") {
		t.Errorf("the reason must say this is not evidence of absence, got %q", c.Reason)
	}
}

// ── UNSAFE ────────────────────────────────────────────────────────────────────

// Analysis that ignored policy would promise a replay the executor then declines.
func TestUnsafeWhenPolicyWouldRefuse(t *testing.T) {
	deleteNode := node("action_9", "mail", "Delete", rect(10, 10, 80, 24))
	deleteNode.ResolvedTarget.Role = "button"
	deleteNode.Plan.Risk = directorapi.RiskHigh

	w := scene("mail",
		obs("uia:1", directorapi.RoleWindow, "Untitled - mail", rect(0, 0, 800, 600)),
		obs("uia:2", directorapi.RoleButton, "Delete", rect(10, 10, 80, 24)),
		obs("uia:3", directorapi.RoleButton, "Cancel", rect(100, 10, 80, 24)),
		obs("uia:4", directorapi.RoleTextField, "Subject", rect(10, 50, 200, 24)),
	)

	engine := policy.New()
	engine.Now = func() time.Time { return t0 }
	a := NewAnalyzer()
	a.Policy = engine

	c := a.Analyze(deleteNode, w)
	if c.Status != ReplayUnsafe {
		t.Fatalf("status = %s (%s), want UNSAFE", c.Status, c.Reason)
	}
	if c.Replayable {
		t.Error("an action needing confirmation is not replayable unattended")
	}
}

// ── non-replayable actions ────────────────────────────────────────────────────

// A node with no query has nothing to re-resolve. Reporting it READY would promise a
// replay that could only proceed by guessing.
func TestActionWithNoQueryIsNotReplayable(t *testing.T) {
	n := fileNode()
	n.Plan.Steps[0].Action.Query = nil

	c := AnalyzeReplay(n, notepadScene())
	if c.Replayable {
		t.Fatal("an action with no target description cannot be replayed")
	}
	if c.Status != ReplayUnsafe {
		t.Errorf("status = %s, want UNSAFE", c.Status)
	}
	if !strings.Contains(c.Reason, "re-resolve") && !strings.Contains(c.Reason, "query") {
		t.Errorf("the reason should explain what is missing, got %q", c.Reason)
	}
}

func TestNodeWithNoActionIsNotReplayable(t *testing.T) {
	n := fileNode()
	n.Plan.Steps = nil
	if c := AnalyzeReplay(n, notepadScene()); c.Replayable {
		t.Error("a node recording no action cannot be replayed")
	}
}

// ── window actions ────────────────────────────────────────────────────────────

// A window move is re-found by APPLICATION and TITLE, never by the stored handle:
// handles are reissued on restart and would either miss or find a different window.
func TestWindowMoveReadyWhenTheWindowIsOpen(t *testing.T) {
	n := windowNode("action_2", "notepad", "left_half")
	n.Plan.Steps[0].Action.Window.Handle = "hwnd:99999" // stale on purpose

	c := AnalyzeReplay(n, notepadScene())
	if c.Status != ReplayReady {
		t.Fatalf("status = %s (%s), want READY despite the stale handle", c.Status, c.Reason)
	}
	if c.CurrentTarget == nil || c.CurrentTarget.WindowID != "hwnd:1" {
		t.Errorf("the window should be re-found by application, got %+v", c.CurrentTarget)
	}
}

func TestWindowMoveAppNotRunning(t *testing.T) {
	n := windowNode("action_2", "notepad", "left_half")
	elsewhere := scene("chrome",
		obs("uia:1", directorapi.RoleWindow, "Untitled - chrome", rect(0, 0, 800, 600)),
		obs("uia:2", directorapi.RoleButton, "Back", rect(10, 10, 40, 24)),
	)
	if c := AnalyzeReplay(n, elsewhere); c.Status != ReplayAppNotRunning {
		t.Errorf("status = %s (%s), want APP_NOT_RUNNING", c.Status, c.Reason)
	}
}

// A minimised window has nothing to act on until it is restored.
func TestWindowMoveMissingWhenMinimised(t *testing.T) {
	n := windowNode("action_2", "notepad", "left_half")
	id := directorapi.WindowID("hwnd:1")
	w := recorded.NewBuilder().Build(recorded.Perception{
		Timestamp: t0,
		Windows: []directorapi.Window{{
			ID: id, Application: "notepad", Title: "Untitled", Minimized: true,
		}},
		ActiveWindow: &id,
		ActiveApp:    &directorapi.Application{ID: "notepad", Name: "notepad"},
	})
	if c := AnalyzeReplay(n, w); c.Replayable {
		t.Errorf("a minimised window is not ready to be moved, got %s", c.Status)
	}
}

// Several windows of the same application, with nothing to tell them apart.
func TestWindowMoveAmbiguousAcrossWindows(t *testing.T) {
	n := windowNode("action_2", "notepad", "left_half")
	n.Plan.Steps[0].Action.Window.Title = "" // no title to narrow by

	one := directorapi.WindowID("hwnd:1")
	w := recorded.NewBuilder().Build(recorded.Perception{
		Timestamp: t0,
		Windows: []directorapi.Window{
			{ID: one, Application: "notepad", Title: "A", Visible: true},
			{ID: "hwnd:2", Application: "notepad", Title: "B", Visible: true},
		},
		ActiveWindow: &one,
		ActiveApp:    &directorapi.Application{ID: "notepad", Name: "notepad"},
	})
	if c := AnalyzeReplay(n, w); c.Status != ReplayTargetAmbiguous {
		t.Errorf("status = %s (%s), want TARGET_AMBIGUOUS", c.Status, c.Reason)
	}
}

// ── the analysis contract ─────────────────────────────────────────────────────

// Analysis must never act. Every path returns a verdict and a reason.
func TestEveryAnalysisExplainsItself(t *testing.T) {
	worlds := map[string]directorapi.WorldState{
		"ready":     notepadScene(),
		"empty":     {},
		"other app": scene("chrome", obs("uia:1", directorapi.RoleButton, "Back", rect(0, 0, 40, 24))),
	}
	for name, w := range worlds {
		c := AnalyzeReplay(fileNode(), w)
		if c.Status == "" {
			t.Errorf("%s: no status", name)
		}
		if c.Reason == "" {
			t.Errorf("%s: no reason given", name)
		}
		if c.Node.ID != "action_1" {
			t.Errorf("%s: the candidate should carry its node", name)
		}
	}
}

func TestReplayableStatuses(t *testing.T) {
	replayable := []ReplayStatus{ReplayReady, ReplayTargetMoved}
	blocked := []ReplayStatus{
		ReplayTargetMissing, ReplayTargetAmbiguous, ReplayAppNotRunning,
		ReplayUnsafe, ReplayUnobservable,
	}
	for _, s := range replayable {
		if !s.Replayable() {
			t.Errorf("%s should be replayable", s)
		}
	}
	for _, s := range blocked {
		if s.Replayable() {
			t.Errorf("%s must not be replayable", s)
		}
	}
}
