package main

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// When reality diverges from the graph.
//
// # Three different things, and confusing any two of them is the whole risk
//
//	GRAPH KNOWLEDGE    activating X from A has been observed to reach B
//	EXECUTION ATTEMPT  Marco tried to activate X from A, now
//	ATTEMPT OUTCOME    this attempt succeeded, failed, was refused, or could not be checked
//
// A failed attempt is a fact about NOW. It is not, by itself, evidence that the graph fact is
// false: a button that moved, a window that changed, a stale accessibility handle and a control
// that was briefly disabled all produce failures and none of them means the control leads
// somewhere else. Contradiction has its own definition and its own evidence — the same semantic
// action from the same semantic source POSITIVELY OBSERVED reaching a materially different
// destination — and nothing here may reach it.
//
// So failure evidence is ATTEMPT-SCOPED. It lives as long as one delegated goal and then it is
// gone, and it never touches the durable graph.
//
// # What recovery is, and what it is not
//
// It is fulfilment of the goal the person already asked for: look again, work out where Marco
// actually is, avoid what just failed, and ask the SAME planner for another way. It is not
// exploration, not retrying a demonstration, not learning, and not permission to do more than was
// delegated.
//
// See [[ADR-099-a-failed-attempt-is-not-a-false-edge]].

// ── what a failure means ─────────────────────────────────────────────────────

// verdict is what recovery may do about one failure.
type verdict int

const (
	// stopHere means end the attempt. Something happened that recovery must not work around.
	stopHere verdict = iota
	// replanFrom means the goal is still worth pursuing from wherever Marco now is.
	replanFrom
	// retryMechanics means the semantic edge is fine and only its mechanics went stale, so
	// resolving the target again and repeating the SAME edge once is the honest response.
	retryMechanics
)

// classify says what recovery may do about one failed edge.
//
// # Over the vocabulary that already exists
//
// Nothing new is invented here. `rehearse` already names every way a step can end — the walker's
// `Outcome`, the attempt's `Terminal`, and the door's `Refusal` — and inventing a parallel
// taxonomy would give the product two words for one event and let them drift.
//
// What was missing is the READING: which of those words describe a world that moved, and which
// describe a boundary Marco must not work around.
//
// # The three answers
//
//	stopHere         cancellation, panic, lost authority, an unreadable screen, a bound
//	                 spent, anything about permission rather than about the interface
//	retryMechanics   the semantic target is still there and its handle went stale
//	replanFrom       the interface is not what the graph expected, and another way may exist
//
// Everything unrecognised falls to `stopHere`. A word this function has never heard of is a word
// whose meaning nobody has decided, and guessing that an unknown failure is safe to work around is
// exactly the guess that turns a bug into Marco pressing things.
//
// Deleting the stopHere default must fail TestAnUnknownFailureStopsRatherThanRecovering.
func classify(step service.PerformStep) verdict {
	switch step.Refusal {
	case cancelledWord:
		// THE AUDIENCE ENDED IT. Never a failure, never a reason to look for another way.
		return stopHere
	case string(rehearse.RefusalNoGrant), string(rehearse.RefusalGrantSpent),
		string(rehearse.RefusalGrantRevoked), string(rehearse.RefusalGrantExpired),
		"no_authority", "no_actuator":
		// PERMISSION, not the interface. Recovery is not permission escalation and a
		// revoked grant is not something to plan around.
		return stopHere
	case string(rehearse.RefusalInputBound), string(rehearse.RefusalUnobservableBound),
		string(rehearse.BoundsExceeded):
		// A BOUND ALREADY SPENT. Working around it is how a bound stops being one.
		return stopHere
	}
	switch rehearse.Outcome(step.Outcome) {
	case rehearse.TargetMoved:
		// THE CONTROL IS STILL THERE and its handle is not. Nothing semantic has changed,
		// so re-resolving it and repeating the same edge once is the honest response —
		// and the graph says "press the control called X", never "press x=742 y=391".
		return retryMechanics
	case rehearse.TargetUnavailable, rehearse.Unrecognised, rehearse.WrongState,
		rehearse.Ambiguous, rehearse.InputFailed, rehearse.WindowBehind:
		// THE INTERFACE IS NOT WHAT THE GRAPH EXPECTED. Another way may exist and asking
		// is the point of this file.
		return replanFrom
	case rehearse.Unobservable, rehearse.ProgressUnobservable:
		// MARCO CANNOT SEE. 35C's rule: an unreadable screen is not a source to plan from,
		// and a best guess about where you are is the one thing recovery must never make.
		return stopHere
	}
	switch step.Terminal {
	case string(rehearse.EndedUnverified), string(rehearse.StoppedAtStep):
		// THE ACTION RAN AND THE DESTINATION DID NOT APPEAR. That is the motivating case:
		// something happened, and where it happened is a question for a fresh look.
		return replanFrom
	case string(rehearse.NothingSent):
		// NOTHING REACHED THE DESKTOP, so nothing about the world has been learned. There
		// is no new information to replan on and the same plan would do the same thing.
		return stopHere
	}
	// AND ANYTHING ELSE STOPS. See above: an unknown word is one nobody has decided about.
	return stopHere
}

// ── what one attempt remembers ───────────────────────────────────────────────

// Bounds on one delegated goal. Small, deliberate, and stated where they are read.
const (
	// maxReplans is how many times one goal may be re-planned after a failure.
	//
	// Three. A person asked for one thing: two alternate routes is a real second and third
	// chance at it, and a fourth would be Marco insisting. The number matters less than the
	// bound existing — an unbounded `while not arrived: replan` in front of a broken
	// interface is a loop somebody has to watch.
	maxReplans = 3
	// maxAttemptSteps is the total semantic actions one delegation may spend.
	//
	// Recovery makes routes longer, and lengths compound: a three-edge plan that replans
	// into five and then eight is how a request to open a settings page becomes a minute of
	// windows opening. This is the ceiling on the WHOLE attempt, replans included.
	maxAttemptSteps = 12
	// maxEdgeAttempts is how many times one semantic edge may be tried within one goal.
	//
	// Two, and only for a failure whose class earns the second — see retryMechanics. An edge
	// that failed twice has told Marco what it has to tell.
	maxEdgeAttempts = 2
)

// attempt is what one delegated goal remembers while it is being carried out.
//
// # Attempt-scoped, and semantic
//
// It lives on the stack of one `PerformGoal` call and is gone when that returns. Nothing here is
// written to the store, nothing survives a restart, and nothing narrows what a LATER invocation
// may plan — see [[ADR-099-a-failed-attempt-is-not-a-false-edge]].
//
// It holds subject ids, edge references and failure classes. No screenshots, no coordinates, no
// handles, no typed text: the identity of a failed edge is the same identity the planner uses, so
// suppressing one thing cannot accidentally suppress another.
type attempt struct {
	// failed is every edge that failed during this attempt, with how many times.
	failed map[observe.RelationshipRef]int
	// replans is how many alternate routes have been asked for.
	replans int
	// steps is every semantic action spent, across the original plan and every replan.
	steps int
	// visited is every Place recognised during this attempt, in order.
	//
	// For loop detection. Revisiting a Place is NOT wrong on its own — walking back to try
	// another way out of it is exactly what recovery is for — so this is read alongside the
	// replan count rather than as a refusal of its own.
	visited []string
}

func newAttempt() *attempt {
	return &attempt{failed: map[observe.RelationshipRef]int{}}
}

// recordFailure remembers that one edge did not work, and how many times.
func (a *attempt) recordFailure(edge observe.RelationshipRef) int {
	a.failed[edge]++
	return a.failed[edge]
}

// mayRetry says whether an edge that just failed may be tried once more.
func (a *attempt) mayRetry(edge observe.RelationshipRef) bool {
	return a.failed[edge] < maxEdgeAttempts
}

// arrivedAt records where Marco was recognised, for loop detection.
func (a *attempt) arrivedAt(subject string) {
	if subject == "" {
		return
	}
	a.visited = append(a.visited, subject)
}

// stuck reports whether this attempt is going round without getting anywhere.
//
// # Why a repeated Place is not enough on its own
//
// Coming back to a screen is ordinary: an edge out of it failed, Marco is standing there again,
// and another way out may exist. What is NOT ordinary is coming back to a screen having already
// tried and failed everything the planner would offer from it — and the replan count is what makes
// that measurable without keeping a second model of the graph.
//
// So: a Place seen three times in one attempt, with a replan spent each time, is going round.
func (a *attempt) stuck(subject string) bool {
	if subject == "" {
		return false
	}
	seen := 0
	for _, v := range a.visited {
		if v == subject {
			seen++
		}
	}
	return seen >= 3
}

// exhausted says the attempt has spent one of its bounds, and which.
func (a *attempt) exhausted(planned int) (string, bool) {
	if a.replans >= maxReplans {
		return "replans_exhausted", true
	}
	if a.steps+planned > maxAttemptSteps {
		return "step_budget_exhausted", true
	}
	return "", false
}

// avoiding layers this attempt's failures over the durable grade.
//
// # Two kinds of preference, kept apart
//
//	DURABLE     what the graph's evidence says about an edge — 36E's ranking
//	ATTEMPT     what just happened to it, in this execution, minutes ago
//
// The second wins for the current attempt and is invisible to the next one. That is the whole
// point of the motivating case: 36E prefers a verified edge, the verified edge is broken today, and
// Marco should take the observed alternative NOW without forgetting that the verified one worked
// yesterday.
//
// It refuses ELIGIBILITY rather than lowering a rank, so a failed edge cannot creep back in by
// being the best of a bad set. The durable grade still decides everything else, so a recovery route
// is ranked by exactly the same rules the first one was — there is no weaker fallback mode.
//
// Deleting this wrapper must fail TestAFailedEdgeIsNotChosenAgainInTheSameAttempt.
func (a *attempt) avoiding(grade observe.EdgeGrade) observe.EdgeGrade {
	return func(ref observe.RelationshipRef) (observe.EdgeRank, bool) {
		if a.failed[ref] > 0 {
			return observe.EdgeRank{}, false
		}
		if grade == nil {
			return observe.EdgeRank{Class: observe.ClassObservedOnce}, true
		}
		return grade(ref)
	}
}

// ── what a person is told ────────────────────────────────────────────────────

// recoveryWords describes one recovery for a person, from the semantic facts.
//
// Not host logs. Somebody who asked for a settings page and watched two windows open is owed the
// shape of what happened: what was tried, what went wrong with it, where Marco actually was, and
// what it did instead.
func recoveryWords(from string, failed observe.RelationshipRef, why string,
	top observe.Topology, steps int) string {

	var b strings.Builder
	fmt.Fprintf(&b, "%s didn't work (%s), ", placeWordsIn(top, failed.To), sayFailure(why))
	fmt.Fprintf(&b, "so from %s I went another way", placeWordsIn(top, from))
	if steps > 0 {
		fmt.Fprintf(&b, " — %d more step(s)", steps)
	}
	b.WriteString(".")
	return b.String()
}

// sayFailure renders one failure class in words rather than in the vocabulary.
func sayFailure(why string) string {
	switch rehearse.Outcome(why) {
	case rehearse.TargetMoved:
		return "the control had moved"
	case rehearse.TargetUnavailable:
		return "the control wasn't there"
	case rehearse.Unrecognised:
		return "I didn't recognise where it left me"
	case rehearse.WrongState:
		return "it went somewhere else"
	case rehearse.Ambiguous:
		return "I couldn't tell which control it meant"
	case rehearse.WindowBehind:
		return "the window wasn't in front"
	case rehearse.InputFailed:
		return "the press didn't land"
	}
	switch why {
	case string(rehearse.EndedUnverified):
		return "I couldn't confirm it arrived"
	case string(rehearse.StoppedAtStep):
		return "it stopped part way"
	}
	if why == "" {
		return "it didn't work"
	}
	return why
}
