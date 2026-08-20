package observe_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A session that produced no durable edge says WHY, and says it the same way twice.
//
// The counts were computed and tallied in closed vocabulary from the moment the causes existed.
// What was missing was any surface that read them out: a live learn refused with "4 transition(s)
// were seen and none had two recognisable endpoints", which is a count of a silence with four
// different explanations behind it — and the four call for four different responses.

func TestTheReportNamesEveryCauseItRecorded(t *testing.T) {
	r := observe.RelationshipReport{
		SessionLocal: 4,
		SessionLocalCauses: map[observe.SessionLocalCause]int{
			observe.DestinationUnresolved: 3,
			observe.SourceUnresolved:      1,
		},
		BridgeRefusals: map[observe.BridgeRefusal]int{
			observe.BridgeAmbiguousInterval: 1,
		},
	}
	got := r.Why()
	for _, want := range []string{
		string(observe.DestinationUnresolved) + "=3",
		string(observe.SourceUnresolved) + "=1",
		"bridge:" + string(observe.BridgeAmbiguousInterval) + "=1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report says %q, which does not mention %q.\nA cause that was counted "+
				"and then not said is the same silence the count already was.", got, want)
		}
	}
}

// Two readings of one report produce one sentence.
//
// Map iteration order is random in Go, so an unsorted rendering would make the same refusal read
// differently every time it was displayed — and a reader comparing two runs would see a difference
// that is not there.
func TestTheCausesAreRenderedInAStableOrder(t *testing.T) {
	r := observe.RelationshipReport{
		SessionLocalCauses: map[observe.SessionLocalCause]int{
			observe.DestinationUnresolved: 1,
			observe.SourceUnresolved:      1,
			observe.SameSubject:           1,
			observe.BothUnresolved:        1,
		},
	}
	first := r.Why()
	for range 24 {
		if got := r.Why(); got != first {
			t.Fatalf("two readings of one report differ:\n  %s\n  %s", first, got)
		}
	}
}

// Silence is reported as silence, not as an empty string.
//
// A refusal that ends in ": " tells a reader the surface is broken rather than that nothing was
// recorded, and those are different problems.
func TestAReportWithNoCausesStillSaysSomething(t *testing.T) {
	if got := (observe.RelationshipReport{SessionLocal: 2}).Why(); strings.TrimSpace(got) == "" {
		t.Error("a report with a session-local count and no recorded cause rendered nothing at " +
			"all; the refusal it is appended to would trail off")
	}
}

// A recovered adjacency is visible, not just the failures.
func TestABridgedAdjacencyIsReported(t *testing.T) {
	got := (observe.RelationshipReport{Bridged: 1}).Why()
	if !strings.Contains(got, "bridged=1") {
		t.Errorf("a bridged adjacency is not mentioned in %q; a reader cannot tell a route that "+
			"was recovered from one that never needed recovering", got)
	}
}
