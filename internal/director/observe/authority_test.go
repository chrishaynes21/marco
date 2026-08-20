package observe_test

import (
	"go/build"
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Nothing Marco has learned can act.
//
// # Why this file is separate from the boundary test
//
// `TestTheObservationPackageCannotAct` proves that this PACKAGE cannot reach a desktop. That is
// the guarantee, and it holds for whatever happens to live here.
//
// What it does not prove is that the things Marco learns still live here. The risk the rehearsal
// design has to close is not "somebody adds an import" — it is "somebody moves ProcedureCandidate
// somewhere more convenient", into a package that plans, or executes, or lowers. The type would
// keep every field, keep every method, keep passing its own tests, and quietly acquire a
// neighbourhood that can act.
//
// So the learned types are NAMED here, and the naming is the point. Moving one out of the
// analysis core fails this file rather than passing silently.
//
// See [[ADR-023-rehearsal-is-attempt-scoped-authority]].

// learnedTypes are everything Marco believes as a result of watching somebody.
//
// Each one is a claim about the world that a person's behaviour put there, and not one of them
// is permission to do anything. The list is the whole chain, from "these two screens are
// connected" to "here is a demonstration of how".
var learnedTypes = []any{
	observe.RememberedSubject{},
	observe.RememberedRelationship{},
	observe.LearningRequest{},
	observe.ProcedureCandidate{},
	observe.CandidateAssessment{},
	observe.Capture{},
	observe.Proposal{},
	// The judgement is the last thing derived from watching: "is this evidence enough to
	// ASK?". Named here for the same reason as the rest — it is a reading of behaviour, and
	// a reading of behaviour must live where it cannot act on itself.
	observe.RehearsalJudgement{},
	// And the grant, which is the one type on this list that is NOT learned.
	//
	// It is here because it is the type most likely to be moved. A grant is authority, and
	// authority feels like it belongs next to the thing that consumes it — which is exactly
	// the package that can act. It stays here: the engine that will one day spend a grant must
	// be HANDED one, and this is what stops it acquiring the ability to mint its own.
	observe.RehearsalGrant{},
}

// TestEverythingMarcoLearnsLivesWhereItCannotAct is the authority proof.
//
// The invariants ADR-023 rests on, checked rather than asserted:
//
//	a remembered subject cannot execute
//	a remembered relationship cannot execute
//	a learning request cannot execute
//	a procedure candidate cannot execute
//	an assessment cannot execute
//	a demonstration cannot execute
//	a yes cannot execute
//
// All seven reduce to one fact — every one of those types is defined in a package with no path
// to anything that can affect the machine — and that fact is what this checks.
func TestEverythingMarcoLearnsLivesWhereItCannotAct(t *testing.T) {
	const core = "github.com/chaynes-simpleclouds/marco/internal/director/observe"
	for _, v := range learnedTypes {
		rt := reflect.TypeOf(v)
		if got := rt.PkgPath(); got != core {
			t.Errorf("%s is defined in %s, not in the analysis core.\n"+
				"  Everything Marco learns by watching lives where it CANNOT act. Moving one "+
				"out gives it a neighbourhood that can plan, lower or execute — and the type "+
				"would keep every field and pass every one of its own tests while doing it.\n"+
				"  See ADR-023.", rt.Name(), got)
		}
	}

	// And the core still cannot reach anything that acts. Restated here rather than assumed,
	// because the two halves are only a guarantee together: the types must be here, and here
	// must be powerless.
	reachable := map[string]bool{}
	if err := walkImports(core, reachable, 0); err != nil {
		t.Fatalf("walking imports: %v", err)
	}
	for _, f := range forbidden {
		for path := range reachable {
			if strings.Contains(path, f.fragment) {
				t.Errorf("the analysis core can reach %s (%s)", path, f.why)
			}
		}
	}
	_ = build.Default
}

// TestNoLearnedTypeOffersAWayToRunItself is the shape half of the same proof.
//
// A type cannot execute if it cannot reach an executor — but a method NAMED like an executor is
// how the next person finds out that it can. This is the same discipline the candidate already
// had, applied to the whole chain rather than to one type.
func TestNoLearnedTypeOffersAWayToRunItself(t *testing.T) {
	forbiddenMethods := []string{
		"Execute", "Run", "Replay", "Perform", "Apply", "Invoke", "Compile",
		"Rehearse", "Promote", "Lower", "Emit", "Send", "Press", "Click", "Type",
	}
	for _, v := range learnedTypes {
		rt := reflect.TypeOf(v)
		for _, name := range forbiddenMethods {
			if _, ok := rt.MethodByName(name); ok {
				t.Errorf("%s has a %s method", rt.Name(), name)
			}
			if _, ok := reflect.PointerTo(rt).MethodByName(name); ok {
				t.Errorf("*%s has a %s method", rt.Name(), name)
			}
		}
	}
}

// TestNothingLearnedClaimsToBeVerified holds the line the rehearsal design exists to draw.
//
// `Verified` is the one word that would mean "Marco tried this and it worked", and today nothing
// has tried anything. Every path through the learning loop — one demonstration, two agreeing
// demonstrations, the best possible assessment — must leave it false, because a rehearsal is the
// only thing that could set it and rehearsal does not exist yet.
//
// When rehearsal IS built, this test is what stops the flag being set by the wrong layer: the
// only thing entitled to change it is a rehearsal result, and a rehearsal result is a separate
// record rather than a mutation of the observation. See ADR-023.
func TestNothingLearnedClaimsToBeVerified(t *testing.T) {
	c := observe.ProcedureCandidate{
		Relationship: observe.RelationshipRef{From: "subj_a", To: "subj_b"},
		Start:        observe.Checkpoint{Subject: "subj_a", Verdict: observe.MatchSame},
		Steps: []observe.DemonstrationStep{{
			Intents: []observe.NavIntent{observe.NavConfirm},
			Arrived: observe.Checkpoint{Subject: "subj_b", Verdict: observe.MatchSame},
		}},
		Complete: true, Reason: observe.ReasonArrived, Checkpoints: 2, Events: 1,
	}
	if c.Verified {
		t.Error("a freshly captured demonstration claims to be verified")
	}

	top := observe.Topology{Subjects: map[string]observe.RememberedSubject{
		"subj_a": {ID: "subj_a"}, "subj_b": {ID: "subj_b"},
	}}
	// The strongest state the whole loop can reach: a consistent candidate corroborated by a
	// second agreeing demonstration.
	best := observe.AssessCandidate(c, top, observe.DefaultCaptureBounds(),
		observe.Corroboration{Compared: true, Agreement: observe.AgreementSame})
	if best.Verified {
		t.Fatal("two agreeing demonstrations produced a verified assessment. Nothing has been " +
			"tried; `verified` must mean Marco rehearsed it and watched it work")
	}
	// And no assessment reason claims a rehearsal happened.
	for _, r := range best.Reasons {
		// `start_unverifiable` and friends are about RECOGNITION and are fine. What must
		// not exist is a reason that reports on an experiment.
		if strings.Contains(string(r), "rehears") || string(r) == "verified" {
			t.Errorf("assessment reason %q implies an experiment that has not been built", r)
		}
	}

	// And now the strongest state the whole system can reach: an ELIGIBLE judgement and an
	// ISSUED grant. Neither may claim anything was tried.
	j := observe.JudgeRehearsal(c, best, top, "")
	if j.Eligible {
		// The empty application is a scope mismatch, so this fixture is not eligible; what
		// matters is that being judged changed nothing about the candidate.
		t.Fatal("a candidate for no application was judged eligible")
	}
	if c.Verified {
		t.Error("judging a candidate marked it verified")
	}
	rt := reflect.TypeOf(observe.RehearsalGrant{})
	if _, ok := rt.FieldByName("Verified"); ok {
		t.Error("a grant carries a Verified flag. A grant is permission to try; it is not a " +
			"result, and nothing that has not run may report one")
	}
}
