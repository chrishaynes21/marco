// Package plan turns a resolved intent into a typed, validated plan.
//
// The planner emits STRUCTS, never text and never commands. There is no code path
// from a plan to a shell, and no plan step that means "do whatever this string
// says". That is not a safety feature bolted on afterwards — it is the reason the
// type exists. A planner whose output had to be interpreted could be talked into
// anything by whatever produced the output.
//
// Every plan carries its own expectations. A step without a success condition
// degrades the whole pipeline to fire-and-hope, so the planner attaches them and
// validation rejects a plan that lacks them.
package plan

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Planner builds typed plans.
type Planner struct {
	// MaxAttempts is how many times the pipeline may execute a plan's step,
	// including the single retry after a failed verification.
	MaxAttempts int
}

// New returns a Planner with the milestone's bounds.
func New() *Planner { return &Planner{MaxAttempts: 2} }

// Input is everything the planner is given.
//
// It receives the chosen candidate AND the rejected ones AND the explanation, not
// just the winner. A planner that only sees the winner cannot tell a confident
// choice from a coin-flip between two near-identical controls, and it is the planner
// that decides whether to ask.
type Input struct {
	Intent     directorapi.Intent
	World      *directorapi.WorldState
	Resolution directorapi.Resolution
	// Placement is the requested window placement for a move.
	Placement string
	// TargetWindow is the window a move applies to.
	TargetWindow *directorapi.Window
	// Monitors is the display layout, needed to turn "left half" into a rectangle.
	Monitors []directorapi.Monitor
}

// Build produces a plan, or an error explaining why no plan is possible.
func (p *Planner) Build(in Input) (directorapi.Plan, error) {
	switch in.Intent.Verb {
	case "click":
		return p.clickPlan(in)
	case "focus":
		return p.focusPlan(in)
	case "move_window":
		return p.movePlan(in)
	case "edit":
		return p.editPlan(in)
	case intent.SemanticVerb:
		return p.semanticPlan(in)
	}
	return directorapi.Plan{}, fmt.Errorf("no planner for %q", in.Intent.Verb)
}

// clickPlan builds a single-step click.
//
// The step's expectations are what makes this verifiable rather than hopeful. A
// click's effect is not knowable in advance — it might open a menu, move focus,
// close a dialog — so the conditions are alternatives, and verification looks for
// ANY of them rather than requiring a specific one.
func (p *Planner) clickPlan(in Input) (directorapi.Plan, error) {
	target, err := elementTarget(in)
	if err != nil {
		return directorapi.Plan{}, err
	}
	el, _ := in.World.Element(in.Resolution.Target.ElementID)

	plan := directorapi.Plan{
		Goal:            "click " + describeTarget(in),
		RequestedIntent: in.Intent.Raw,
		Risk:            clickRisk(el),
		MaxAttempts:     p.MaxAttempts,
		Assumptions: []string{
			fmt.Sprintf("%s is on screen and enabled", describeTarget(in)),
		},
		Steps: []directorapi.PlanStep{{
			Index:     0,
			Action:    directorapi.ClickAction{Target: target, Button: directorapi.ButtonLeft},
			Rationale: in.Resolution.Explanation,
			Expect: []directorapi.Condition{
				{Type: directorapi.ConditionScreenChanged,
					Description: "clicking " + describeTarget(in) + " changes something on screen"},
			},
		}},
	}
	return plan, nil
}

// focusPlan builds a single-step focus.
//
// Focus is the one action with a precise, checkable expectation: afterwards, THIS
// element holds keyboard focus. Everything else is inference.
func (p *Planner) focusPlan(in Input) (directorapi.Plan, error) {
	target, err := elementTarget(in)
	if err != nil {
		return directorapi.Plan{}, err
	}
	return directorapi.Plan{
		Goal:            "focus " + describeTarget(in),
		RequestedIntent: in.Intent.Raw,
		// Focusing changes nothing but where the keyboard points, and is undone by
		// focusing something else.
		Risk:        directorapi.RiskLow,
		MaxAttempts: p.MaxAttempts,
		Steps: []directorapi.PlanStep{{
			Index:     0,
			Action:    directorapi.FocusAction{Target: target},
			Rationale: in.Resolution.Explanation,
			Expect: []directorapi.Condition{
				{Type: directorapi.ConditionElementFocused,
					Query:       in.Resolution.Target.Query,
					Description: describeTarget(in) + " holds keyboard focus"},
			},
		}},
	}, nil
}

// movePlan builds a window move.
//
// The destination rectangle is computed HERE, from the monitor layout, rather than
// left to the executor. The plan then states exactly where the window should end up,
// which is what lets verification check the outcome against the request instead of
// merely noticing that something moved.
func (p *Planner) movePlan(in Input) (directorapi.Plan, error) {
	if in.TargetWindow == nil {
		return directorapi.Plan{}, fmt.Errorf("no window to move")
	}
	win := *in.TargetWindow
	dest, err := Destination(win, in.Monitors, in.Placement)
	if err != nil {
		return directorapi.Plan{}, err
	}
	if dest == win.Bounds {
		return directorapi.Plan{}, fmt.Errorf("the window is already there")
	}

	return directorapi.Plan{
		Goal:            fmt.Sprintf("move %q %s", win.Title, strings.ReplaceAll(in.Placement, "_", " ")),
		RequestedIntent: in.Intent.Raw,
		Risk:            directorapi.RiskLow,
		MaxAttempts:     p.MaxAttempts,
		Assumptions: []string{
			fmt.Sprintf("%q is the window to move", win.Title),
		},
		Steps: []directorapi.PlanStep{{
			Index: 0,
			Action: directorapi.MoveWindowAction{
				// Both the handle and the durable identifiers. The handle is what
				// executes NOW; the application and title are what would find this
				// window again after it has been closed and reopened, when the handle
				// has been reissued and points at nothing (or at something else).
				Window: directorapi.WindowReference{
					ID:            win.ID,
					Application:   win.Application,
					TitleContains: win.Title,
					Description:   win.Title,
				},
				// Both the NAME and the rectangle, for the same reason. The rectangle
				// is where this window goes on today's monitor layout; the name is
				// what the user asked for, and is the only part that still means
				// something on a different layout. Storing only the rectangle would
				// reduce "move it left" to a coordinate, which is precisely what the
				// action graph exists to avoid.
				Placement: directorapi.WindowPlacement{
					Named:  in.Placement,
					Bounds: &dest,
				},
			},
			Rationale: fmt.Sprintf("%s of %s", strings.ReplaceAll(in.Placement, "_", " "), monitorName(win, in.Monitors)),
			Expect: []directorapi.Condition{
				{Type: directorapi.ConditionWindowBounds, Bounds: &dest,
					Window:      &directorapi.WindowReference{ID: win.ID},
					Description: "the window occupies the requested rectangle"},
			},
		}},
	}, nil
}

// Destination computes where a window should end up for a symbolic placement.
//
// Placements are relative to the WORK AREA rather than the full monitor rectangle,
// so "left half" does not put half the window behind the taskbar.
func Destination(win directorapi.Window, monitors []directorapi.Monitor, placement string) (directorapi.Rect, error) {
	mon, ok := monitorFor(win, monitors)
	if !ok {
		return directorapi.Rect{}, fmt.Errorf("cannot tell which monitor that window is on")
	}
	area := mon.WorkArea
	if area.Empty() {
		area = mon.Bounds
	}

	switch placement {
	case "left_half":
		return directorapi.Rect{X: area.X, Y: area.Y, Width: area.Width / 2, Height: area.Height}, nil
	case "right_half":
		return directorapi.Rect{X: area.X + area.Width/2, Y: area.Y,
			Width: area.Width - area.Width/2, Height: area.Height}, nil
	case "top_half":
		return directorapi.Rect{X: area.X, Y: area.Y, Width: area.Width, Height: area.Height / 2}, nil
	case "bottom_half":
		return directorapi.Rect{X: area.X, Y: area.Y + area.Height/2,
			Width: area.Width, Height: area.Height - area.Height/2}, nil
	case "center":
		w, h := win.Bounds.Width, win.Bounds.Height
		if w <= 0 || w > area.Width {
			w = area.Width * 2 / 3
		}
		if h <= 0 || h > area.Height {
			h = area.Height * 2 / 3
		}
		return directorapi.Rect{
			X: area.X + (area.Width-w)/2, Y: area.Y + (area.Height-h)/2,
			Width: w, Height: h,
		}, nil
	case "other_monitor":
		other, ok := otherMonitor(mon, monitors)
		if !ok {
			return directorapi.Rect{}, fmt.Errorf("there is only one monitor")
		}
		dest := other.WorkArea
		if dest.Empty() {
			dest = other.Bounds
		}
		// Keep the window's size, placed at the same RELATIVE position on the new
		// screen. Moving a window to another monitor should not also resize it, and
		// preserving the offset keeps it somewhere the user expects to find it.
		w := clamp(win.Bounds.Width, 1, dest.Width)
		h := clamp(win.Bounds.Height, 1, dest.Height)
		relX, relY := 0.0, 0.0
		if area.Width > 0 {
			relX = float64(win.Bounds.X-area.X) / float64(area.Width)
		}
		if area.Height > 0 {
			relY = float64(win.Bounds.Y-area.Y) / float64(area.Height)
		}
		x := dest.X + int(relX*float64(dest.Width))
		y := dest.Y + int(relY*float64(dest.Height))
		// Keep it fully on screen.
		x = clamp(x, dest.X, dest.X+dest.Width-w)
		y = clamp(y, dest.Y, dest.Y+dest.Height-h)
		return directorapi.Rect{X: x, Y: y, Width: w, Height: h}, nil
	case "maximized":
		return area, nil
	}
	return directorapi.Rect{}, fmt.Errorf("unknown placement %q", placement)
}

func monitorFor(win directorapi.Window, monitors []directorapi.Monitor) (directorapi.Monitor, bool) {
	for _, m := range monitors {
		if m.ID == win.MonitorID {
			return m, true
		}
	}
	// Fall back to whichever monitor contains the window's centre.
	centre := win.Bounds.Center()
	for _, m := range monitors {
		if m.Bounds.Contains(centre) {
			return m, true
		}
	}
	for _, m := range monitors {
		if m.Primary {
			return m, true
		}
	}
	return directorapi.Monitor{}, false
}

// otherMonitor picks the next monitor after the current one, in a stable order, so
// "the other monitor" means the same screen every time on a two-screen desktop and
// cycles predictably on more.
func otherMonitor(current directorapi.Monitor, monitors []directorapi.Monitor) (directorapi.Monitor, bool) {
	if len(monitors) < 2 {
		return directorapi.Monitor{}, false
	}
	idx := -1
	for i, m := range monitors {
		if m.ID == current.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return monitors[0], true
	}
	return monitors[(idx+1)%len(monitors)], true
}

func monitorName(win directorapi.Window, monitors []directorapi.Monitor) string {
	if m, ok := monitorFor(win, monitors); ok {
		if m.Primary {
			return "the primary monitor"
		}
		return "its monitor"
	}
	return "the screen"
}

// elementTarget builds the reference an element action points at.
//
// It carries the QUERY alongside the id, deliberately. The id identifies the element
// in the snapshot the plan was built from; the query is what lets it be found again
// after a retry re-observes, when the id may have changed.
func elementTarget(in Input) (directorapi.ElementReference, error) {
	if in.Resolution.Status != directorapi.ResolutionResolved || in.Resolution.Target == nil {
		return directorapi.ElementReference{}, fmt.Errorf("no resolved target to act on")
	}
	t := in.Resolution.Target
	return directorapi.ElementReference{
		ID:          t.ElementID,
		Query:       t.Query,
		Description: describeTarget(in),
	}, nil
}

func describeTarget(in Input) string {
	if in.Resolution.Target != nil && in.Resolution.Target.Label != "" {
		return fmt.Sprintf("%q", in.Resolution.Target.Label)
	}
	if len(in.Intent.Targets) > 0 {
		return fmt.Sprintf("%q", in.Intent.Targets[0].Phrase)
	}
	return "the target"
}

// clickRisk classifies a click by what it appears to do.
//
// Conservative and label-driven, which is crude but honest about being crude: the
// Director cannot know what a button does, so it treats the words users put on
// destructive buttons as a reason to ask. A missed classification errs toward
// confirming something harmless, not toward silently doing something irreversible.
func clickRisk(el *directorapi.Element) directorapi.RiskLevel {
	if el == nil {
		return directorapi.RiskMedium
	}
	label := strings.ToLower(el.Label)
	for _, word := range destructiveWords {
		if strings.Contains(label, word) {
			return directorapi.RiskHigh
		}
	}
	switch el.Role {
	case directorapi.RoleMenuItem, directorapi.RoleMenu, directorapi.RoleTab, directorapi.RoleTreeItem:
		// Opening a menu or switching a tab shows you something; it does not commit
		// anything.
		return directorapi.RiskLow
	}
	return directorapi.RiskMedium
}

var destructiveWords = []string{
	"delete", "remove", "erase", "discard", "destroy", "wipe",
	"send", "submit", "post", "publish", "buy", "purchase", "pay", "order",
	"uninstall", "format", "reset", "revoke", "deactivate", "close account",
	"sign out", "log out", "shut down", "restart",
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
