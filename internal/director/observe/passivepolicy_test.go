package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Making explicit Learn one-shot must not make PASSIVE learning one-shot.
//
// # Why this file exists
//
// Two things called "corroboration" live in this package and they answer different questions:
//
//	LearningThresholds.MinSessions   may Marco ASK to learn a habit it noticed?
//	single_demonstration_only        must a person repeat a demonstration they were asked for?
//
// The second stopped blocking — a rehearsal answers it better than a repetition does. The first
// is untouched, and must stay untouched: a habit Marco merely noticed has no explicit request
// behind it and no rehearsal coming, so cross-session corroboration is the only evidence there is.
// [[ADR-051-one-demonstration-and-an-attempt]]
//
// There was no test over MinSessions at all when this was written, which is how a change to the
// other kind of corroboration could have reached it unnoticed.

func rememberedEdge(sessions, observations int) observe.RememberedRelationship {
	return observe.RememberedRelationship{
		From: "subj_a", To: "subj_b", Application: "testgame",
		Observations: observations, Sessions: sessions,
		Preceded: map[observe.NavIntent]int{observe.NavConfirm: observations},
		Sequences: []observe.NavSequence{
			{Intents: []observe.NavIntent{observe.NavConfirm}, Count: observations},
		},
	}
}

func topologyWith(ids ...string) observe.Topology {
	top := observe.Topology{Subjects: map[string]observe.RememberedSubject{}}
	for _, id := range ids {
		top.Subjects[id] = observe.RememberedSubject{ID: id, Application: "testgame"}
	}
	return top
}

func refused(a observe.LearningAssessment, why observe.LearningRefusal) bool {
	for _, r := range a.Refusals {
		if r == why {
			return true
		}
	}
	return false
}

// A habit seen in ONE session is still not proposed, however many times it happened there.
func TestPassiveLearningStillRequiresCrossSessionCorroboration(t *testing.T) {
	top := topologyWith("subj_a", "subj_b")
	a := observe.AssessLearning(rememberedEdge(1, 20), top, observe.DefaultLearningThresholds())
	if !refused(a, observe.RefusalInsufficientSessions) {
		t.Fatalf("an edge seen twenty times in ONE session was not refused for want of "+
			"sessions: %+v.\nTwenty observations of one sitting is one sitting. Explicit Learn "+
			"became one-shot because a person asked for it and a rehearsal confirms it; a habit "+
			"Marco merely noticed has neither.", a.Refusals)
	}
}

// And the threshold itself is still two.
func TestTheCrossSessionThresholdIsUnchanged(t *testing.T) {
	if got := observe.DefaultLearningThresholds().MinSessions; got != 2 {
		t.Errorf("MinSessions = %d, want 2; the passive corroboration policy was changed", got)
	}
}

// Two sessions with enough evidence still clears that particular gate, so the test above is
// measuring the sessions rule rather than something else refusing everything.
func TestTwoSessionsClearsTheSessionsGate(t *testing.T) {
	top := topologyWith("subj_a", "subj_b")
	a := observe.AssessLearning(rememberedEdge(2, 6), top, observe.DefaultLearningThresholds())
	if refused(a, observe.RefusalInsufficientSessions) {
		t.Errorf("two sessions were still refused for want of sessions: %+v", a.Refusals)
	}
}

// `single_demonstration_only` is still REPORTED. It stopped blocking; it did not stop being true.
//
// A reader of an assessment must still be able to see that there has been one example. Deleting
// the reason instead of reclassifying it would have hidden that.
func TestOneExampleIsStillReportedAsOneExample(t *testing.T) {
	if !observe.ReasonSingleDemonstration.ResolvableByDemonstration() {
		t.Error("single_demonstration_only no longer counts as resolvable by another " +
			"demonstration; a second example IS still available as a recovery path and the " +
			"reason has to stay honest about that")
	}
	if !observe.ReasonSingleDemonstration.ConfirmableByRehearsal() {
		t.Error("single_demonstration_only is not confirmable by rehearsal, so one clean " +
			"demonstration will still ask the person to repeat themselves")
	}
}

// Exactly one reason is answered by an attempt. Widening this is how one-shot would quietly
// become "accept anything".
func TestOnlyTheCountOfExamplesIsAnsweredByTrying(t *testing.T) {
	confirmable := []observe.AssessmentReason{}
	for _, r := range []observe.AssessmentReason{
		observe.ReasonSingleDemonstration, observe.ReasonIncompleteDemonstration,
		observe.ReasonStartUnverifiable, observe.ReasonEndUnverifiable,
		observe.ReasonTransientCheckpoint, observe.ReasonRequiresTextEntry,
		observe.ReasonUnresolvedPointer, observe.ReasonAmbiguousRun,
		observe.ReasonBacktracking, observe.ReasonNearCaptureBound,
		observe.ReasonNoSteps, observe.ReasonDemonstrationsDisagree,
	} {
		if r.ConfirmableByRehearsal() {
			confirmable = append(confirmable, r)
		}
	}
	if len(confirmable) != 1 || confirmable[0] != observe.ReasonSingleDemonstration {
		t.Errorf("these reasons claim to be settled by Marco trying it: %v.\nOnly the COUNT of "+
			"examples is. Every other reason names something Marco could not READ, and an "+
			"attempt acts on a reading rather than clarifying one.", confirmable)
	}
}

// A one-example assessment that is otherwise clean does not block the offer to try.
func TestACleanFirstAssessmentDoesNotBlock(t *testing.T) {
	a := observe.CandidateAssessment{
		Verdict: observe.CandidateConsistent,
		Reasons: []observe.AssessmentReason{observe.ReasonSingleDemonstration},
	}
	if a.BlocksRehearsal() {
		t.Errorf("a clean first demonstration is blocked by %v", a.Blocking())
	}
	// But an unreadable one does, and names itself.
	b := observe.CandidateAssessment{
		Verdict: observe.CandidateInsufficient,
		Reasons: []observe.AssessmentReason{
			observe.ReasonSingleDemonstration, observe.ReasonTransientCheckpoint,
		},
	}
	if !b.BlocksRehearsal() {
		t.Fatal("a route with an unrecognisable screen on it reached the offer to try")
	}
	if got := b.Blocking(); len(got) != 1 || got[0] != observe.ReasonTransientCheckpoint {
		t.Errorf("blocking = %v, want only the transient checkpoint; asking a person to fix "+
			"the number of examples they have given is asking for nothing they can do", got)
	}
}

// Recognition did not become permissive to make one screen work.
//
// The Settings drift was fixed in the PRODUCER — the composition a signature is built from — and
// the temptation throughout was to widen the matcher instead. Widening it would have made every
// application's screens easier to confuse in exchange for one page's noise, and it would have
// looked like it worked.
func TestRecognitionDidNotBecomeMorePermissive(t *testing.T) {
	if observe.RoleCountTolerance != 1 {
		t.Errorf("RoleCountTolerance = %d, want 1. The durable-identity drift was a producer "+
			"defect; loosening the matcher makes every screen in every application easier to "+
			"confuse in exchange.", observe.RoleCountTolerance)
	}
	if observe.MemberTolerance != 1 {
		t.Errorf("MemberTolerance = %d, want 1", observe.MemberTolerance)
	}
	// And it still bites: two screens that differ by more than the tolerance are different,
	// however much else they share.
	base := observe.StructureSignature{
		Subject: "screen_state",
		Roles:   map[string]int{"button": 15, "list": 2, "text": 32},
		Members: 49, Terms: []observe.InterfaceTerm{observe.TermSettings}, TermsKnown: true,
	}
	wider := observe.StructureSignature{
		Subject: "screen_state",
		Roles:   map[string]int{"button": 21, "list": 2, "text": 32},
		Members: 55, Terms: []observe.InterfaceTerm{observe.TermSettings}, TermsKnown: true,
	}
	if got := observe.CompareStructure(wider, base); got != observe.MatchDifferent {
		t.Errorf("a six-button difference compared as %q, want %q. The matcher has been "+
			"widened; the producer is where the drift was.", got, observe.MatchDifferent)
	}
}
