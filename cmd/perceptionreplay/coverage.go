package main

// What the recording can and cannot say about each learned step.
//
// # Why a prefix is not an assessment
//
// The ordered walk answers one question — how far would a run get — and it answers it by
// stopping. Every step after the stop is unexamined, and "unexamined" has at least three
// different meanings that a single number hides:
//
//   - the control could not be resolved, which is a perception failure;
//   - the step's own screen never stood again, so there was nothing to try it against;
//   - the recording simply ended first, which is a fact about the demonstration and not
//     about Marco.
//
// Reporting the third as the first would invent a defect. Reporting the first as the third
// would hide one. So each step is classified, and the classification says which evidence was
// missing rather than only that something was.
//
// The owner asked for exactly this distinction:
//
//	Do not treat a missing recorded continuation as a successful transition.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
)

// walker is one in-order attempt at the draft, from a given starting step.
//
// Extracted so the SECOND demonstrated route can be walked from its own beginning rather than
// only as the tail of the first. A route that is never reached because an earlier one stopped
// has not been evaluated, and saying so needs a walk that starts there.
type walker struct {
	start  int
	cursor int
	// notBefore is the look this walk may begin at.
	//
	// Without it the second route's walk starts at look 1 and plans its opening
	// steps against the FIRST route's observations — steps 11-14 are the same
	// screens and controls as 0-3, so it would sail through them on evidence
	// recorded before the person had demonstrated that route at all. The answer
	// would be real and about the wrong thing.
	notBefore int
	reach     int
	stopAt    int
	stopWhy   string

	pending *workflow.Action

	arrived   map[int]int
	waiting   map[int]int
	elsewhere map[int]int

	// act is the first action this walk planned, kept so the lowering demonstration can
	// use a real one.
	act   *workflow.Action
	actAt int
}

func newWalker(start, notBefore int) *walker {
	return &walker{
		start: start, cursor: start, reach: start, stopAt: -1, notBefore: notBefore,
		arrived: map[int]int{}, waiting: map[int]int{}, elsewhere: map[int]int{},
	}
}

// Steps reports how many steps this walk completed from where it started.
func (w *walker) Steps() int { return w.reach - w.start }

// look advances the walk by one observation.
//
// Behaviour is the walk that was already here, moved rather than redesigned, with one dead
// guard removed: a refusal used to be followed by `if standing != step.From { stop }`, which
// could not be reached because the line above it returns in exactly that case. So a refusal
// never stops a walk — it waits, on the reasoning that a screen settles before its contents are
// proposed. Worth stating plainly, because it is why a stop is nearly always "the recording
// ended" rather than "the control was refused".
func (w *walker) look(d workflow.Draft, at int, standing observe.VisualScreenID,
	got []observe.Reacquisition) {

	if at < w.notBefore || w.stopAt >= 0 || w.cursor >= len(d.Steps) {
		return
	}
	step := d.Steps[w.cursor]
	if !step.Repeatable() {
		// A WAIT. The right response to a load completing is to watch for the
		// destination, and arriving is what advances.
		if standing == step.To {
			w.arrived[w.cursor]++
			w.cursor++
			w.reach = w.cursor
		} else {
			w.waiting[w.cursor]++
		}
		return
	}
	if w.pending != nil {
		switch workflow.Verify(*w.pending, step.From, standing) {
		case workflow.VerdictArrived:
			w.arrived[w.cursor]++
			w.pending = nil
			w.cursor++
			w.reach = w.cursor
		case workflow.VerdictElsewhere:
			// THE EXPECTED TRANSITION DID NOT HAPPEN. A run stops: it has lost
			// its place, and acting on a workflow that does not know where it is
			// presses something nobody asked for.
			w.elsewhere[w.cursor]++
			w.stopAt, w.stopWhy = w.cursor, fmt.Sprintf(
				"the step expected %s and the screen became %s",
				w.pending.Expect, standing)
			w.pending = nil
		case workflow.VerdictWaiting:
			w.waiting[w.cursor]++
		}
		return
	}
	if standing != step.From {
		return
	}
	act, ref := workflow.Plan(d, w.cursor, standing, got)
	if ref != nil {
		return
	}
	w.pending = act
	if w.act == nil {
		w.act, w.actAt = act, w.cursor
	}
}

// finish records the outcome of a walk that ended without stopping and without arriving.
func (w *walker) finish(d workflow.Draft) {
	if w.stopAt < 0 && w.cursor < len(d.Steps) {
		w.stopAt = w.cursor
		w.stopWhy = "the recording ended while this step was still waiting for its " +
			"control to be resolvable"
	}
}

// ── classification ────────────────────────────────────────────────────────────

// coverage is why one step is or is not evaluable from this recording.
type coverage string

const (
	// covVerified: the in-order walk planned it, acted, and the screen became what the
	// step expected. The only one of these that is a success.
	covVerified coverage = "verified"
	// covUnresolved: its screen stood and the control was never resolvable on it. A
	// perception failure, and the only one that is a defect.
	covUnresolved coverage = "unresolved"
	// covNotReached: resolvable, but the walk never arrived at it — an earlier step
	// stopped, or the recording ran out first. A fact about the demonstration.
	covNotReached coverage = "not-reached"
	// covNoEvidence: the screen this step starts from never stood again anywhere in the
	// recording, so there was nothing to evaluate it against at all.
	covNoEvidence coverage = "no-evidence"
)

// stepCoverage is one step's verdict and the counts behind it.
type stepCoverage struct {
	Step    int
	Verdict coverage
	// OwnScreenLooks is how many looks this step's starting screen was standing —
	// the opportunities it had, independent of whether the walk ever reached it.
	OwnScreenLooks int
	// Resolved is how many of those looks the control could be planned on.
	Resolved int
	Refusals map[string]int
}

// classify turns the two passes' evidence into one verdict per step.
//
// `ownScreen` counts looks where the step's starting screen stood; for a wait step that is its
// DESTINATION, because what a wait needs to observe is the arrival.
// A step counts as verified if ANY walk arrived through it. The second route is walked from
// its own start precisely so that its steps are not written off as unreachable merely because
// an earlier route ran out of recording first — crediting only the first walk would report a
// route that completes end to end as twelve unexamined steps.
func classify(d workflow.Draft, walks []*walker, ownScreen map[int]int,
	trials map[int]*stepTrial) []stepCoverage {

	out := make([]stepCoverage, 0, len(d.Steps))
	for i := range d.Steps {
		sc := stepCoverage{Step: i, OwnScreenLooks: ownScreen[i], Refusals: map[string]int{}}
		if t, ok := trials[i]; ok {
			sc.Resolved = t.planned
			for k, n := range t.refused {
				sc.Refusals[k] = n
			}
		}
		switch {
		case arrivedInAny(walks, i):
			sc.Verdict = covVerified
		case sc.OwnScreenLooks == 0:
			// NOTHING TO TRY IT ON. Not a failure of anything Marco did.
			sc.Verdict = covNoEvidence
		case d.Steps[i].Repeatable() && sc.Resolved == 0:
			sc.Verdict = covUnresolved
		default:
			sc.Verdict = covNotReached
		}
		out = append(out, sc)
	}
	return out
}

// arrivedInAny reports whether any walk completed step i.
func arrivedInAny(walks []*walker, i int) bool {
	for _, w := range walks {
		if w != nil && w.arrived[i] > 0 {
			return true
		}
	}
	return false
}

// printCoverage is the product-level assessment the prefix alone cannot give.
func printCoverage(d workflow.Draft, cov []stepCoverage, first, second *walker,
	secondStart int) {

	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  WHAT THIS RECORDING CAN AND CANNOT SAY ABOUT EACH STEP\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("  A prefix stops, and everything after the stop is unexamined. These are\n")
	fmt.Printf("  the reasons a step went unexamined, which are not the same reason.\n\n")
	fmt.Printf("  %-4s %-10s %-10s %-12s %7s %9s  %s\n",
		"#", "from", "target", "verdict", "looks", "resolved", "refusals")

	counts := map[coverage]int{}
	for _, c := range cov {
		counts[c.Verdict]++
		var refs []string
		for k, n := range c.Refusals {
			refs = append(refs, fmt.Sprintf("%s=%d", k, n))
		}
		sort.Strings(refs)
		target := string(d.Steps[c.Step].Target)
		if target == "" {
			target = "-"
		}
		fmt.Printf("  %-4d %-10s %-10s %-12s %7d %9d  %s\n",
			c.Step, d.Steps[c.Step].From, target, c.Verdict,
			c.OwnScreenLooks, c.Resolved, strings.Join(refs, " "))
	}

	fmt.Printf("\n      %d verified, %d unresolved, %d not reached, %d with no evidence\n",
		counts[covVerified], counts[covUnresolved],
		counts[covNotReached], counts[covNoEvidence])
	fmt.Printf("      Only `unresolved` is a perception defect. `not-reached` and\n")
	fmt.Printf("      `no-evidence` are limits of this recording, and counting them as\n")
	fmt.Printf("      failures would invent a problem; counting them as successes would\n")
	fmt.Printf("      hide one.\n")

	// ── THE SECOND ROUTE, FROM ITS OWN BEGINNING ──
	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  THE TWO DEMONSTRATED ROUTES, WALKED INDEPENDENTLY\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	if secondStart < 0 {
		fmt.Printf("      This demonstration does not repeat its opening step, so there is\n")
		fmt.Printf("      no second route to walk from its own start.\n")
		return
	}
	fmt.Printf("      The demonstration returns to its opening step at %d, so the walk\n",
		secondStart)
	fmt.Printf("      is run twice: once from step 0, and once starting at step %d with a\n",
		secondStart)
	fmt.Printf("      fresh cursor, and not before look %d — where the demonstration\n",
		second.notBefore)
	fmt.Printf("      arrived at the screen this route starts from. Starting it at look 1\n")
	fmt.Printf("      would have let it plan its opening steps against the FIRST route's\n")
	fmt.Printf("      observations, since they are the same screens and controls, and the\n")
	fmt.Printf("      answer would have been real and about the wrong thing.\n\n")
	for _, w := range []struct {
		name  string
		w     *walker
		total int
	}{
		{fmt.Sprintf("route 1 (steps 0-%d)", secondStart-1), first, secondStart},
		{fmt.Sprintf("route 2 (steps %d-%d)", secondStart, len(d.Steps)-1),
			second, len(d.Steps) - secondStart},
	} {
		fmt.Printf("      %-24s %d of %d step(s)", w.name, w.w.Steps(), w.total)
		if w.w.stopAt >= 0 {
			fmt.Printf(", stopped at %d\n        %s\n", w.w.stopAt,
				wrapAt(w.w.stopWhy, 60, "        "))
		} else {
			fmt.Printf(", completed\n")
		}
	}
}

// secondRouteStart finds where the demonstration repeats its own opening step.
//
// Structural rather than guessed: the same source screen, the same target and the same
// destination as step 0, met again. On `rl-freeplay-1` the person walked to Free Play, came
// back, and did it again — so the draft contains two routes end to end and the second one has
// never been evaluated from its own starting observation.
//
// Returns -1 when the demonstration never repeats itself, which is the ordinary case for a
// workflow demonstrated once.
func secondRouteStart(d workflow.Draft) int {
	if len(d.Steps) == 0 {
		return -1
	}
	first := d.Steps[0]
	if !first.Repeatable() {
		return -1
	}
	for i := 1; i < len(d.Steps); i++ {
		s := d.Steps[i]
		if s.From == first.From && s.To == first.To && s.Target == first.Target {
			return i
		}
	}
	return -1
}

// routeStartLook is the look at which a route BEGINS: when the demonstration arrived at the
// screen its first step starts from.
//
// The obvious choice — the look of the route's own first transition — is wrong, and measurably
// so. A `VisualTransition`'s look is when the change SETTLED, so gating on it starts the walk
// after that step has already happened: the walk then sits waiting for a source screen it has
// just left, catches the NEXT visit to it, and reports that the route failed at step one when
// what actually happened is that it was never given the look it needed. That produced `0 of 10`
// on a route the recording completes.
//
// So the gate is the transition INTO the route's first screen — the step before it. The route
// starts when the person was standing where the route starts.
//
// Mirrors `workflow.Learn`'s rule for which occurrence a step came from: steps are built in walk
// order and take the next unused occurrence of their screen pair, so the occurrence wanted here
// is the number of earlier steps sharing that pair.
//
// Returns 0 when the recording cannot say, which starts the walk at the beginning — the
// permissive answer, and the report prints the look it used so a reader can see which it got.
func routeStartLook(d workflow.Draft, trans []observe.VisualTransition, at int) int {
	arrival := at - 1
	if arrival < 0 || at >= len(d.Steps) {
		return 0
	}
	want := d.Steps[arrival]
	occurrence := 0
	for j := 0; j < arrival; j++ {
		if d.Steps[j].From == want.From && d.Steps[j].To == want.To {
			occurrence++
		}
	}
	for _, t := range trans {
		if t.From != want.From || t.To != want.To || len(t.Looks) == 0 {
			continue
		}
		if occurrence >= len(t.Looks) {
			occurrence = len(t.Looks) - 1
		}
		return t.Looks[occurrence]
	}
	return 0
}

// sameTargetAcrossVisits reports, for every target used by more than one step, whether each of
// those steps could be resolved.
//
// The owner's question: do the same learned targets resolve correctly on both visits? A target
// that resolves on its first step and not its second is not one control seen twice — it is
// evidence that the identity is carrying something about WHEN it was learned.
func sameTargetAcrossVisits(d workflow.Draft, cov []stepCoverage) map[observe.VisualTargetID][]stepCoverage {
	byTarget := map[observe.VisualTargetID][]stepCoverage{}
	for _, c := range cov {
		if t := d.Steps[c.Step].Target; t != "" {
			byTarget[t] = append(byTarget[t], c)
		}
	}
	for t, list := range byTarget {
		if len(list) < 2 {
			delete(byTarget, t)
		}
	}
	return byTarget
}

func printRepeatedTargets(d workflow.Draft, byTarget map[observe.VisualTargetID][]stepCoverage) {
	if len(byTarget) == 0 {
		return
	}
	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  THE SAME LEARNED TARGET, ON EACH STEP THAT USES IT\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("  One control pressed at two points in the walk. Both steps have to work,\n")
	fmt.Printf("  and a target that resolves at one and not the other is carrying\n")
	fmt.Printf("  something about WHEN it was learned rather than WHAT it is.\n\n")
	ids := make([]string, 0, len(byTarget))
	for t := range byTarget {
		ids = append(ids, string(t))
	}
	sort.Strings(ids)
	for _, id := range ids {
		list := byTarget[observe.VisualTargetID(id)]
		var parts []string
		agree := true
		for _, c := range list {
			parts = append(parts, fmt.Sprintf("step %d: %s (%d resolved of %d look(s))",
				c.Step, c.Verdict, c.Resolved, c.OwnScreenLooks))
			if (c.Resolved > 0) != (list[0].Resolved > 0) {
				agree = false
			}
		}
		mark := "  "
		if !agree {
			mark = "<-"
		}
		fmt.Printf("  %-10s %s %s\n", id, mark, strings.Join(parts, "; "))
	}
}
