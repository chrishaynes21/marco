package plan

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

func rect(x, y, w, h int) directorapi.Rect {
	return directorapi.Rect{X: x, Y: y, Width: w, Height: h}
}

var monitors = []directorapi.Monitor{
	{ID: "monitor:1", Bounds: rect(0, 0, 1920, 1080), WorkArea: rect(0, 0, 1920, 1040), Primary: true},
	{ID: "monitor:2", Bounds: rect(-1920, 0, 1920, 1080), WorkArea: rect(-1920, 0, 1920, 1040)},
}

func window(x, y, w, h int, monitor string) directorapi.Window {
	return directorapi.Window{ID: "hwnd:1", Title: "App", Bounds: rect(x, y, w, h), MonitorID: monitor}
}

// ── intent parsing ────────────────────────────────────────────────────────────

func TestIntentParsing(t *testing.T) {
	p := intent.New()
	cases := []struct {
		input     string
		kind      directorapi.IntentKind
		verb      string
		target    string
		placement string
	}{
		{"click save", directorapi.IntentAct, "click", "save", ""},
		{"click the Save button", directorapi.IntentAct, "click", "Save button", ""},
		// "Press File" is a SEMANTIC invoke, not a click. The semantic action milestone
		// changed this deliberately: pressing a control and clicking at its coordinates
		// differ in what they can be verified by and in what a replay re-derives, and
		// only the first is what the user said. A literal "click X" stays a click —
		// that request names the gesture.
		{"press File", directorapi.IntentAct, intent.SemanticVerb, "File", ""},
		{"focus explorer", directorapi.IntentAct, "focus", "explorer", ""},
		{"move window left", directorapi.IntentAct, "move_window", "", "left_half"},
		{"move window to the other monitor", directorapi.IntentAct, "move_window", "", "other_monitor"},
		{"stop", directorapi.IntentStop, "stop", "", ""},
		{"frobnicate things", directorapi.IntentUnknown, "", "", ""},
		{"click", directorapi.IntentUnknown, "", "", ""},
		{"", directorapi.IntentUnknown, "", "", ""},
	}
	for _, c := range cases {
		got := p.Parse(c.input)
		if got.Kind != c.kind {
			t.Errorf("%q: kind = %s, want %s", c.input, got.Kind, c.kind)
			continue
		}
		if c.verb != "" && got.Verb != c.verb {
			t.Errorf("%q: verb = %q, want %q", c.input, got.Verb, c.verb)
		}
		if c.target != "" {
			if len(got.Targets) == 0 || got.Targets[0].Phrase != c.target {
				t.Errorf("%q: target = %v, want %q", c.input, got.Targets, c.target)
			}
		}
		if c.placement != "" {
			if p, _ := got.Parameters["placement"].(string); p != c.placement {
				t.Errorf("%q: placement = %q, want %q", c.input, p, c.placement)
			}
		}
	}
}

// An unparsed request must produce a question, not a guess. Guessing turns "close
// the account" into a click on whatever happens to be nearest.
func TestUnknownIntentExplainsItself(t *testing.T) {
	got := intent.New().Parse("frobnicate the widget")
	if got.Ambiguity == "" {
		t.Error("an unrecognised request should say what it did not understand")
	}
}

// "left half" must beat "left" — longest match wins, or every two-word placement
// collapses to its first word.
func TestLongestPlacementWins(t *testing.T) {
	got := intent.New().Parse("move window left half")
	if p, _ := got.Parameters["placement"].(string); p != "left_half" {
		t.Errorf("placement = %q, want left_half", p)
	}
}

// ── placement geometry ────────────────────────────────────────────────────────

// Placements are relative to the WORK area, so a window does not end up half behind
// the taskbar.
func TestDestinationUsesTheWorkArea(t *testing.T) {
	win := window(300, 200, 800, 600, "monitor:1")

	got, err := Destination(win, monitors, "left_half")
	if err != nil {
		t.Fatalf("left_half: %v", err)
	}
	if got != rect(0, 0, 960, 1040) {
		t.Errorf("left half = %+v, want the work area's left half (height 1040, not 1080)", got)
	}

	if got, _ := Destination(win, monitors, "right_half"); got != rect(960, 0, 960, 1040) {
		t.Errorf("right half = %+v", got)
	}
	if got, _ := Destination(win, monitors, "maximized"); got != rect(0, 0, 1920, 1040) {
		t.Errorf("maximized = %+v", got)
	}
}

// Moving to another monitor keeps the window's SIZE. Resizing as a side effect of
// moving is not what anyone means.
func TestOtherMonitorPreservesSize(t *testing.T) {
	win := window(300, 200, 800, 600, "monitor:1")
	got, err := Destination(win, monitors, "other_monitor")
	if err != nil {
		t.Fatalf("other_monitor: %v", err)
	}
	if got.Width != 800 || got.Height != 600 {
		t.Errorf("size changed: %+v", got)
	}
	// And it must land on the other monitor, which here has a NEGATIVE origin.
	if got.X >= 0 {
		t.Errorf("x = %d, want a position on the left-hand monitor", got.X)
	}
	if !monitors[1].WorkArea.ContainsRect(got) {
		t.Errorf("%+v is not fully inside the destination monitor", got)
	}
}

// A window bigger than the destination must be shrunk to fit rather than placed
// partly off screen.
func TestOtherMonitorClampsAnOversizeWindow(t *testing.T) {
	huge := window(0, 0, 4000, 3000, "monitor:1")
	got, err := Destination(huge, monitors, "other_monitor")
	if err != nil {
		t.Fatalf("other_monitor: %v", err)
	}
	if !monitors[1].WorkArea.ContainsRect(got) {
		t.Errorf("%+v should be clamped into the destination monitor", got)
	}
}

func TestSingleMonitorHasNoOtherMonitor(t *testing.T) {
	one := monitors[:1]
	if _, err := Destination(window(0, 0, 800, 600, "monitor:1"), one, "other_monitor"); err == nil {
		t.Error("with one monitor there is nowhere else to go, and that must be an error")
	}
}

func TestUnknownPlacementIsAnError(t *testing.T) {
	if _, err := Destination(window(0, 0, 800, 600, "monitor:1"), monitors, "sideways"); err == nil {
		t.Error("an unknown placement must not silently produce a rectangle")
	}
}

// ── plan shape ────────────────────────────────────────────────────────────────

// A step with no expectation cannot be verified, which would quietly turn the whole
// pipeline back into fire-and-hope.
func TestEveryPlanStepCarriesAnExpectation(t *testing.T) {
	p := New()
	in := Input{
		Intent: intent.New().Parse("click Save"),
		World:  &directorapi.WorldState{},
		Resolution: directorapi.Resolution{
			Status: directorapi.ResolutionResolved,
			Target: &directorapi.ResolvedTarget{
				ElementID: "e1", Label: "Save", Role: directorapi.RoleButton,
				Query: &directorapi.ElementQuery{Label: "Save"},
			},
		},
	}
	built, err := p.Build(in)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(built.Steps) != 1 {
		t.Fatalf("want one step, got %d", len(built.Steps))
	}
	if len(built.Steps[0].Expect) == 0 {
		t.Error("a step with no expectation cannot be verified")
	}
	if built.MaxAttempts <= 0 {
		t.Error("a plan must bound its own attempts")
	}
	// The target must carry the query, not just the id: a retry re-observes, and the
	// id may not survive.
	ref := built.Steps[0].Action.(directorapi.ClickAction).Target
	if ref.Query == nil {
		t.Error("the action's target must keep the query so a retry can re-resolve")
	}
	if ref.CoordinateLocked {
		t.Error("a semantic click must never be coordinate-locked")
	}
}

// The Director cannot know what a button does, so it treats the words people put on
// destructive buttons as a reason to ask. Erring toward confirming something
// harmless is much cheaper than the reverse.
func TestDestructiveLabelsRaiseRisk(t *testing.T) {
	risky := []string{"Delete", "Delete account", "Send", "Submit order", "Uninstall"}
	for _, label := range risky {
		el := &directorapi.Element{Role: directorapi.RoleButton, Label: label}
		if got := clickRisk(el); got != directorapi.RiskHigh {
			t.Errorf("%q should be high risk, got %s", label, got)
		}
	}
	// Opening a menu shows you something; it does not commit anything.
	menu := &directorapi.Element{Role: directorapi.RoleMenuItem, Label: "File"}
	if got := clickRisk(menu); got != directorapi.RiskLow {
		t.Errorf("opening a menu should be low risk, got %s", got)
	}
	ordinary := &directorapi.Element{Role: directorapi.RoleButton, Label: "Next"}
	if got := clickRisk(ordinary); got != directorapi.RiskMedium {
		t.Errorf("an ordinary button should be medium risk, got %s", got)
	}
}

func TestMovePlanStatesItsDestination(t *testing.T) {
	win := window(300, 200, 800, 600, "monitor:1")
	built, err := New().Build(Input{
		Intent:       intent.New().Parse("move window left"),
		World:        &directorapi.WorldState{},
		TargetWindow: &win,
		Monitors:     monitors,
		Placement:    "left_half",
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	mv := built.Steps[0].Action.(directorapi.MoveWindowAction)
	if mv.Placement.Bounds == nil {
		t.Fatal("the plan must state exactly where the window should end up")
	}
	if *mv.Placement.Bounds != rect(0, 0, 960, 1040) {
		t.Errorf("destination = %+v", *mv.Placement.Bounds)
	}
	// Verification checks against the requested rectangle, so the expectation must
	// carry it too.
	if built.Steps[0].Expect[0].Bounds == nil {
		t.Error("the expectation must state the requested bounds")
	}
}

// Regression, found by inspecting a real node in the action graph: the plan stored
// the computed rectangle and threw away the placement the user actually asked for.
// A rectangle is meaningless on a different monitor layout; "left half" is not.
//
// The same applies to the window itself. A handle executes now and is reissued the
// moment the application restarts, so the durable identifiers have to travel with it
// or a stored action can never find its window again.
func TestMovePlanKeepsTheSemanticPlacementAndWindow(t *testing.T) {
	win := window(300, 200, 800, 600, "monitor:1")
	win.Application = "notepad"
	win.Title = "Untitled - Notepad"

	built, err := New().Build(Input{
		Intent:       intent.New().Parse("move window left"),
		World:        &directorapi.WorldState{},
		TargetWindow: &win,
		Monitors:     monitors,
		Placement:    "left_half",
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	mv := built.Steps[0].Action.(directorapi.MoveWindowAction)

	if mv.Placement.Named != "left_half" {
		t.Errorf("the named placement was lost: %+v", mv.Placement)
	}
	if mv.Placement.Bounds == nil {
		t.Error("the computed rectangle is still needed to execute now")
	}
	if mv.Window.Application != "notepad" {
		t.Errorf("the application was lost: %+v", mv.Window)
	}
	if mv.Window.TitleContains == "" {
		t.Error("the title was lost — with no durable identifier the window cannot be re-found")
	}
	if mv.Window.ID == "" {
		t.Error("the handle is still needed to execute now")
	}
}

// Moving a window that is already where it was asked to go is not work.
func TestMoveToWhereItAlreadyIsIsRefused(t *testing.T) {
	win := window(0, 0, 960, 1040, "monitor:1")
	_, err := New().Build(Input{
		Intent:       intent.New().Parse("move window left"),
		World:        &directorapi.WorldState{},
		TargetWindow: &win,
		Monitors:     monitors,
		Placement:    "left_half",
	})
	if err == nil {
		t.Error("a move to the current position should be refused rather than executed")
	}
}

func TestUnresolvedTargetProducesNoPlan(t *testing.T) {
	_, err := New().Build(Input{
		Intent:     intent.New().Parse("click Save"),
		World:      &directorapi.WorldState{},
		Resolution: directorapi.Resolution{Status: directorapi.ResolutionAbsent},
	})
	if err == nil {
		t.Error("there is nothing to act on, so there should be no plan")
	}
}
