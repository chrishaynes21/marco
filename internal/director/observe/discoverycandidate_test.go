package observe_test

import (
	"reflect"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The pass that watched the demonstration IS the demonstration.
//
// Learn used to run the route twice: once to discover where it goes, once for an armed capture
// to watch it properly. The second performance is the one that kept failing live, because it is
// the one that has to independently re-confirm where the person is standing before it will record
// anything — and it failed with the correct route already in the store.
//
// These cover the construction and, more importantly, every case where it must NOT construct.

// statesMemory resolves each screen to its own subject, by the role map in its signature.
type statesMemory struct {
	observe.Memory
	byButtons map[int]string
}

func (m statesMemory) Recall(_ string, sig observe.StructureSignature) observe.Recollection {
	id, ok := m.byButtons[sig.Roles["button"]]
	if !ok {
		return observe.Recollection{Verdict: observe.MatchDifferent}
	}
	return observe.Recollection{
		Verdict: observe.MatchSame,
		Subject: observe.RememberedSubject{ID: id, Application: "testgame"},
	}
}

func twoPlaces() observe.ShadowTotals {
	return observe.ShadowTotals{
		CurrentState: "state_2",
		States: []observe.ScreenState{
			{ID: "state_1", Episodes: 3, TermObservations: 2,
				Roles: map[string]int{"button": 4, "list": 1}},
			{ID: "state_2", Episodes: 3, TermObservations: 2,
				Roles: map[string]int{"button": 7, "list": 1}},
		},
	}
}

func moved(t observe.ShadowTotals, from, to observe.ScreenStateID,
	seqs ...observe.TargetedSequence) observe.ShadowTotals {

	t.Transitions = append(t.Transitions, observe.ScreenTransition{
		From: from, To: to, Count: 1,
		Preceded:  map[observe.NavIntent]int{observe.NavConfirm: 1},
		Sequences: seqs,
	})
	// The WALK, kept in step with the aggregate. A fixture that recorded a change and not the
	// order it happened in describes a session the producer cannot produce — `note` writes
	// both, at one call site. See observe.ShadowTotals.Crossings.
	t.Crossings = append(t.Crossings, observe.Crossing{From: from, To: to})
	return t
}

// arrived is the second half of a change that crossed a sample nobody could place: the return to
// a placed screen, carrying no navigation of its own and the length of the gap it crossed.
func arrived(t observe.ShadowTotals, to observe.ScreenStateID, gap int) observe.ShadowTotals {
	t.Transitions = append(t.Transitions, observe.ScreenTransition{
		From: observe.ScreenStateUnknown, To: to, Count: 1,
		Unattributed: 1, UnsettledRun: gap,
	})
	t.Crossings = append(t.Crossings, observe.Crossing{
		From: observe.ScreenStateUnknown, To: to, Run: gap,
	})
	return t
}

func run(intents ...observe.NavIntent) observe.TargetedSequence {
	return observe.TargetedSequence{Intents: intents, Count: 1}
}

func memoryOfTwo() statesMemory {
	return statesMemory{byButtons: map[int]string{4: "subj_a", 7: "subj_b"}}
}

var abRoute = observe.RelationshipRef{From: "subj_a", To: "subj_b"}

func fromPass(t observe.ShadowTotals) observe.DiscoveredDemonstration {
	return observe.CandidateFromDiscovery(t, abRoute, "testgame",
		memoryOfTwo(), observe.DefaultHypothesisThresholds())
}

// THE one that removes the second performance.
func TestTheDiscoveryPassAloneProducesTheDemonstration(t *testing.T) {
	got := fromPass(moved(twoPlaces(), "state_1", "state_2",
		run(observe.NavDown, observe.NavDown, observe.NavConfirm)))

	if !got.Built {
		t.Fatalf("no candidate was built from a clean observed change: %s.\nEverything a "+
			"candidate is made of was recorded by the pass that watched it, and asking the "+
			"person to perform the route a second time is asking them to repeat work that "+
			"already succeeded.", got.Refusal)
	}
	c := got.Candidate
	if c.Start.Subject != "subj_a" || c.Relationship != abRoute {
		t.Errorf("candidate starts at %+v on route %+v", c.Start, c.Relationship)
	}
	if !c.Complete || c.Reason != observe.ReasonArrived {
		t.Errorf("complete=%v reason=%q, want an arrival", c.Complete, c.Reason)
	}
	if len(c.Steps) != 1 {
		t.Fatalf("%d steps, want 1", len(c.Steps))
	}
	// ORDER survives. That is the whole reason Sequences exists.
	want := []observe.NavIntent{observe.NavDown, observe.NavDown, observe.NavConfirm}
	if !reflect.DeepEqual(c.Steps[0].Intents, want) {
		t.Errorf("intents = %v, want %v — down, down, confirm and confirm, down, down are two "+
			"different interactions", c.Steps[0].Intents, want)
	}
	if c.Steps[0].Arrived.Subject != "subj_b" || c.Steps[0].Arrived.Transient {
		t.Errorf("arrived at %+v, want the durable destination", c.Steps[0].Arrived)
	}
}

// A change that crossed an unplaceable sample is still one change, and the intents ride on the
// entry leg — the same rule the relationship layer uses, not a second one.
func TestAChangeAcrossAnUnplaceableSampleStillBuilds(t *testing.T) {
	tot := twoPlaces()
	tot = moved(tot, "state_1", observe.ScreenStateUnknown,
		run(observe.NavDown, observe.NavConfirm))
	tot = arrived(tot, "state_2", 1)

	got := fromPass(tot)
	if !got.Built {
		t.Fatalf("the live shape did not build: %s", got.Refusal)
	}
	if !got.Bridged {
		t.Error("the candidate does not record that it crossed a sample nobody could place")
	}
	want := []observe.NavIntent{observe.NavDown, observe.NavConfirm}
	if !reflect.DeepEqual(got.Candidate.Steps[0].Intents, want) {
		t.Errorf("intents = %v, want %v from the ENTRY leg, which is where the navigation was "+
			"seen; the exit leg carried no intent at all",
			got.Candidate.Steps[0].Intents, want)
	}
}

// An interval long enough to have hidden a screen is not bridged here either.
func TestALongBlackoutDoesNotBuildACandidate(t *testing.T) {
	tot := twoPlaces()
	tot = moved(tot, "state_1", observe.ScreenStateUnknown, run(observe.NavConfirm))
	tot.Transitions = append(tot.Transitions, observe.ScreenTransition{
		From: observe.ScreenStateUnknown, To: "state_2", Count: 1,
		UnsettledRun: observe.StatePromotionCount,
	})
	if got := fromPass(tot); got.Built {
		t.Errorf("a blackout long enough to have been a screen produced a candidate anyway; "+
			"something recognisable may have happened inside it: %+v", got.Candidate)
	}
}

// It refuses rather than guessing, and each refusal is its own.
func TestTheDiscoveryCandidateRefusesRatherThanGuessing(t *testing.T) {
	for _, tc := range []struct {
		name string
		tot  observe.ShadowTotals
		want observe.DiscoveryRefusal
	}{{
		name: "the screen never changed",
		tot:  twoPlaces(),
		want: observe.DiscoveryNoTransition,
	}, {
		name: "the change had no navigation before it",
		tot:  moved(twoPlaces(), "state_1", "state_2"),
		want: observe.DiscoveryNoNavigation,
	}, {
		// TWO orders for one change. A capture watches one performance and records one
		// order; this reads an aggregate, so the aggregate has to say one thing.
		name: "the same change was made two different ways",
		tot: moved(twoPlaces(), "state_1", "state_2",
			run(observe.NavDown, observe.NavConfirm), run(observe.NavConfirm)),
		want: observe.DiscoveryOrderAmbiguous,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got := fromPass(tc.tot)
			if got.Built {
				t.Fatalf("built a candidate anyway: %+v", got.Candidate)
			}
			if got.Refusal != tc.want {
				t.Errorf("refusal = %q, want %q", got.Refusal, tc.want)
			}
		})
	}
}

// Recognisability is NOT relaxed on this path.
//
// The route names two durable subjects; if the screens in front of Marco right now do not resolve
// to those, there is no candidate. A path that skipped this would build a demonstration of a route
// between places it cannot find again.
func TestTheOneShotPathDoesNotRelaxRecognisability(t *testing.T) {
	tot := moved(twoPlaces(), "state_1", "state_2", run(observe.NavConfirm))

	// Memory that recognises the start and nothing else.
	half := statesMemory{byButtons: map[int]string{4: "subj_a"}}
	got := observe.CandidateFromDiscovery(tot, abRoute, "testgame",
		half, observe.DefaultHypothesisThresholds())
	if got.Built {
		t.Fatal("a candidate was built with an unrecognisable destination")
	}
	if got.Refusal != observe.DiscoveryEndpointUnresolved {
		t.Errorf("refusal = %q, want %q", got.Refusal, observe.DiscoveryEndpointUnresolved)
	}

	// And a start that resolves to somewhere OTHER than the route's start is not the start.
	wrong := statesMemory{byButtons: map[int]string{4: "subj_z", 7: "subj_b"}}
	if got := observe.CandidateFromDiscovery(tot, abRoute, "testgame",
		wrong, observe.DefaultHypothesisThresholds()); got.Built {
		t.Error("the demonstration began somewhere that is not the route's start")
	}
}

// The typed-text boundary is the SAME rule the capture uses, including its destination exemption.
//
// One function decides it for both paths, so the two cannot come to different answers about the
// same screen — which is exactly what would happen if this path kept its own copy and only one of
// them was corrected. See TestASearchBoxOnTheDestinationDoesNotBlockLearning for the rule itself.
func TestTheOneShotPathUsesTheSameTypedTextRule(t *testing.T) {
	tot := twoPlaces()
	tot.States[1].EditableFields = 1 // the DESTINATION has a search box
	tot = moved(tot, "state_1", "state_2", run(observe.NavConfirm))

	got := fromPass(tot)
	if !got.Built {
		t.Fatalf("did not build: %s", got.Refusal)
	}
	if got.Candidate.Steps[0].RequiresTextEntry {
		t.Error("a search box on the destination made an otherwise reproducible navigation " +
			"route unlowerable on the one-shot path; the capture path exempts it and the two " +
			"must not disagree")
	}
}

// No memory, no candidate.
func TestWithoutMemoryNothingIsBuilt(t *testing.T) {
	got := observe.CandidateFromDiscovery(
		moved(twoPlaces(), "state_1", "state_2", run(observe.NavConfirm)),
		abRoute, "testgame", nil,
		observe.DefaultHypothesisThresholds())
	if got.Built || got.Refusal != observe.DiscoveryNoMemory {
		t.Errorf("got built=%v refusal=%q", got.Built, got.Refusal)
	}
}
