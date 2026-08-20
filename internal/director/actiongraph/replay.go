package actiongraph

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ReplayStatus is what analysis concluded about doing a stored action again.
//
// Nothing here executes. The question is whether it COULD be done right now, asked
// of the current world — which is a useful thing to be able to answer before
// offering "do that again" to a user, and a necessary thing to be able to answer
// before doing it.
type ReplayStatus string

const (
	// ReplayReady means the target re-resolves cleanly, in the same place.
	ReplayReady ReplayStatus = "READY"
	// ReplayTargetMoved means it re-resolves, somewhere else. Still replayable —
	// replay re-resolves rather than replaying coordinates, so a move is expected
	// and harmless. It is reported because a user asking "can you do that again"
	// deserves to know the screen is not as it was.
	ReplayTargetMoved ReplayStatus = "TARGET_MOVED"
	// ReplayTargetMissing means the world was readable and the target is not in it.
	ReplayTargetMissing ReplayStatus = "TARGET_MISSING"
	// ReplayTargetAmbiguous means several things now match. Replaying would be a
	// coin flip between them, so it is not replayable without asking.
	ReplayTargetAmbiguous ReplayStatus = "TARGET_AMBIGUOUS"
	// ReplayAppNotRunning means the application is not there at all.
	ReplayAppNotRunning ReplayStatus = "APP_NOT_RUNNING"
	// ReplayUnsafe means policy would not permit it now without confirmation.
	ReplayUnsafe ReplayStatus = "UNSAFE"

	// ReplayUnobservable means the question cannot be answered: the application
	// exposes nothing to look at, or a source that was needed did not report.
	//
	// This is NOT in the milestone's list of statuses, and it is here deliberately.
	// Without it the only honest home for "I cannot see" would be TARGET_MISSING,
	// which asserts absence — the exact conflation that made the Director
	// confidently wrong about Discord two milestones ago, and that the four-way
	// resolution model exists to prevent. Re-introducing it here to fit a shorter
	// enum would undo that.
	ReplayUnobservable ReplayStatus = "UNOBSERVABLE"
)

// Replayable reports whether this status permits replay without asking first.
func (s ReplayStatus) Replayable() bool {
	return s == ReplayReady || s == ReplayTargetMoved
}

// ReplayCandidate is the result of analysing one node against the current world.
type ReplayCandidate struct {
	Node ActionNode `json:"node"`

	Status ReplayStatus `json:"status"`
	// Replayable is whether it could be done now without asking.
	Replayable bool `json:"replayable"`
	// Reason explains the verdict in one sentence.
	Reason string `json:"reason"`

	// CurrentTarget is where the target is NOW, when it re-resolved. Nil otherwise.
	// This is a fresh resolution, not the stored snapshot — the stored one is
	// history and is never used to locate anything.
	CurrentTarget *directorapi.ResolvedTarget `json:"current_target,omitempty"`

	// Moved is how far the target has shifted since the action was recorded, when
	// both positions are known.
	Moved *Displacement `json:"moved,omitempty"`

	// Alternatives are the competing candidates when the status is ambiguous.
	Alternatives []directorapi.TargetCandidate `json:"alternatives,omitempty"`
}

// Displacement is how far a target moved between two observations.
type Displacement struct {
	From directorapi.Rect `json:"from"`
	To   directorapi.Rect `json:"to"`
	DX   int              `json:"dx"`
	DY   int              `json:"dy"`
}

// Analyzer answers whether a stored action could be performed again.
type Analyzer struct {
	Resolver *target.Resolver
	// Policy, when set, is consulted so an action that would now be refused or
	// need confirmation is reported UNSAFE rather than READY. Analysis that ignores
	// policy would promise a replay the executor then declines.
	Policy directorapi.PolicyEngine
	// MoveTolerance is how many pixels a target may shift and still count as being
	// where it was. Windows nudge things by a pixel or two for reasons no user would
	// call "moving".
	MoveTolerance int
}

// NewAnalyzer returns an Analyzer with the default resolver and tolerance.
func NewAnalyzer() *Analyzer {
	return &Analyzer{Resolver: target.NewResolver(), MoveTolerance: 4}
}

// AnalyzeReplay reports whether a node's action could be performed again in the
// given world. It executes nothing.
func AnalyzeReplay(node ActionNode, world directorapi.WorldState) ReplayCandidate {
	return NewAnalyzer().Analyze(node, world)
}

// Analyze is AnalyzeReplay with an explicit Analyzer.
func (a *Analyzer) Analyze(node ActionNode, world directorapi.WorldState) ReplayCandidate {
	out := ReplayCandidate{Node: node}

	spec, ok := node.Action()
	if !ok {
		return out.conclude(ReplayUnsafe, "this node records no action to replay")
	}

	// A stored action that cannot be rebuilt is not replayable at all, whatever the
	// world looks like — most often because it was never given a semantic target.
	if _, err := spec.Rebuild(); err != nil {
		return out.conclude(ReplayUnsafe, err.Error())
	}

	// The world is examined BEFORE policy, and the order matters.
	//
	// Policy refuses an unreadable window too — but its reason is derived from the
	// same unobservability, and reporting UNSAFE would bury the more specific and
	// more actionable finding under a generic refusal. "I cannot see into this
	// window" tells a caller to reach for another source; "policy refused" does not.
	//
	// Policy still runs afterwards, so an action that IS resolvable but would now be
	// declined is reported UNSAFE rather than READY. Analysis that ignored policy
	// would promise a replay the executor then refuses.
	var verdict ReplayCandidate
	if spec.Type == directorapi.ActionMoveWindow {
		verdict = a.analyzeWindow(out, spec, world)
	} else {
		verdict = a.analyzeElement(out, node, spec, world)
	}
	if !verdict.Replayable {
		return verdict
	}

	if a.Policy != nil {
		if status, reason := a.policyVerdict(node, world); status != "" {
			return verdict.conclude(status, reason)
		}
	}
	return verdict
}

// analyzeElement re-resolves an element target in the current world.
func (a *Analyzer) analyzeElement(out ReplayCandidate, node ActionNode,
	spec ActionSpec, world directorapi.WorldState) ReplayCandidate {

	// Is the application even there? Checked before resolution, because "Notepad is
	// closed" is a far more useful answer than "no button called Save".
	if app := node.ResolvedTarget.App; app != "" && !appPresent(world, app) {
		return out.conclude(ReplayAppNotRunning,
			fmt.Sprintf("%s is not running, or is not the application in front", app))
	}

	// A DERIVED target — the inline editor a previous step opened on the bound object.
	// There is nothing durable to re-resolve, on purpose: the control existed for one
	// edit transaction, and the graph deliberately stores no way to find it again.
	//
	// Whether an editor is open is decided at run time by the derivation, immediately
	// before the step acts and against its own observation — and it REFUSES when there is
	// none. So the honest verdict here is that this action names its target rather than
	// storing it, not that the target is missing: reporting TARGET_MISSING would make
	// every such action unreplayable on the strength of a handle nobody should be keeping.
	if node.RequestedTarget.RequiresEditor {
		return out.conclude(ReplayReady, "this action acts on an editor it derives when it "+
			"runs, so there is no stored target to re-resolve")
	}

	if spec.Query == nil {
		return out.conclude(ReplayUnsafe, "the stored action has no target description to re-resolve")
	}

	res := a.resolver().Resolve(&world, *spec.Query)
	switch res.Status {
	case directorapi.ResolutionResolved:
		out.CurrentTarget = res.Target
		was := node.ResolvedTarget.Bounds
		now := res.Target.Point
		if was.Empty() {
			return out.conclude(ReplayReady, "the target resolves in the current window")
		}
		centre := was.Center()
		dx, dy := now.X-centre.X, now.Y-centre.Y
		if abs(dx) <= a.tolerance() && abs(dy) <= a.tolerance() {
			return out.conclude(ReplayReady, "the target resolves where it was")
		}
		out.Moved = &Displacement{From: was, DX: dx, DY: dy}
		if el, ok := world.Element(res.Target.ElementID); ok {
			out.Moved.To = el.Bounds
		}
		// Moved is still replayable: replay re-resolves, so it will find the target
		// wherever it now is. Saying so is information, not a warning.
		return out.conclude(ReplayTargetMoved,
			fmt.Sprintf("the target is still there but has moved by (%+d,%+d)", dx, dy))

	case directorapi.ResolutionAmbiguous:
		out.Alternatives = res.Candidates
		return out.conclude(ReplayTargetAmbiguous, res.Explanation)

	case directorapi.ResolutionUnobservable:
		return out.conclude(ReplayUnobservable,
			res.Explanation+" — this is not evidence the target is gone")

	default:
		return out.conclude(ReplayTargetMissing, res.Explanation)
	}
}

// analyzeWindow checks whether the window a move applied to still exists.
//
// It looks the window up by APPLICATION and TITLE, never by the stored handle.
// Handles are reissued when an application restarts, so the old one would either
// miss or find a different window — and a window move that lands on the wrong window
// is a particularly unpleasant surprise.
func (a *Analyzer) analyzeWindow(out ReplayCandidate, spec ActionSpec,
	world directorapi.WorldState) ReplayCandidate {

	if spec.Window == nil {
		return out.conclude(ReplayUnsafe, "the stored move names no window")
	}
	app := spec.Window.Application

	var matches []directorapi.Window
	for _, w := range world.Windows {
		if app != "" && !strings.EqualFold(w.Application, app) {
			continue
		}
		if spec.Window.Title != "" && app == "" &&
			!strings.Contains(strings.ToLower(w.Title), strings.ToLower(spec.Window.Title)) {
			continue
		}
		matches = append(matches, w)
	}

	switch len(matches) {
	case 0:
		if app != "" {
			return out.conclude(ReplayAppNotRunning, fmt.Sprintf("%s has no window open", app))
		}
		return out.conclude(ReplayTargetMissing, "that window is not open")
	case 1:
		// Fall through.
	default:
		// Several windows of the same application. Without a way to tell which one
		// the user meant, replaying would pick arbitrarily.
		narrowed := 0
		var chosen directorapi.Window
		for _, w := range matches {
			if spec.Window.Title != "" && strings.EqualFold(w.Title, spec.Window.Title) {
				narrowed++
				chosen = w
			}
		}
		if narrowed != 1 {
			return out.conclude(ReplayTargetAmbiguous,
				fmt.Sprintf("%s has %d windows open and the title does not single one out",
					app, len(matches)))
		}
		matches = []directorapi.Window{chosen}
	}

	win := matches[0]
	out.CurrentTarget = &directorapi.ResolvedTarget{
		WindowID: win.ID, Label: win.Title, Confidence: 1,
	}
	if win.Minimized {
		return out.conclude(ReplayTargetMissing, fmt.Sprintf("%q is minimised", win.Title))
	}
	return out.conclude(ReplayReady, fmt.Sprintf("%q is open and can be moved", win.Title))
}

// policyVerdict asks policy whether this action would still be permitted, returning
// an empty status when it would.
func (a *Analyzer) policyVerdict(node ActionNode, world directorapi.WorldState) (ReplayStatus, string) {
	spec, _ := node.Action()
	action, err := spec.Rebuild()
	if err != nil {
		return ReplayUnsafe, err.Error()
	}
	step := directorapi.PlanStep{Action: action, Risk: node.Plan.Risk}
	d := a.Policy.EvaluateStep(nil, step, nil, world)
	switch {
	case !d.Allowed:
		return ReplayUnsafe, "policy would refuse this now: " + d.Reason
	case d.RequiresConfirmation:
		return ReplayUnsafe, "this would need confirmation: " + d.Reason
	}
	return "", ""
}

// appPresent reports whether an application has a window in this world.
func appPresent(world directorapi.WorldState, app string) bool {
	if world.ActiveApp != nil && strings.EqualFold(world.ActiveApp.ID, app) {
		return true
	}
	for _, w := range world.Windows {
		if strings.EqualFold(w.Application, app) {
			return true
		}
	}
	return false
}

func (a *Analyzer) resolver() *target.Resolver {
	if a.Resolver != nil {
		return a.Resolver
	}
	return target.NewResolver()
}

func (a *Analyzer) tolerance() int {
	if a.MoveTolerance > 0 {
		return a.MoveTolerance
	}
	return 4
}

func (c ReplayCandidate) conclude(status ReplayStatus, reason string) ReplayCandidate {
	c.Status = status
	c.Replayable = status.Replayable()
	c.Reason = reason
	return c
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
