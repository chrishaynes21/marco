package theaterhost

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/activate"
)

// moody is an Actor whose availability is whatever the test says it is right now.
type moody struct {
	name  string
	ready bool
}

func (m *moody) Name() string                                      { return m.name }
func (m *moody) Available(context.Context) bool                    { return m.ready }
func (m *moody) Find(context.Context, Target) ([]Candidate, error) { return nil, nil }
func (m *moody) Cast(Candidate, activate.Way) (string, bool)       { return "", false }

// THE ROSTER IS NOT A SECOND OPINION.
//
// A diagnostic that computes readiness its own way agrees with the product exactly until the
// moment somebody needs it to disagree — which is the one moment it is ever consulted. So the
// claim is not "Roster reports something", it is "Roster reports what CASTING would find", tested
// by moving the answer and checking both surfaces move together.
//
// Mutations this kills: hardcoding Available true (or false); reading a cached field instead of
// asking the Actor; filtering unavailable actors out of the roster, which would make "the
// accessibility actor is here and cannot act" indistinguishable from "there is no accessibility
// actor at all".
func TestTheRosterReadsTheSamePredicateCastingReads(t *testing.T) {
	a := &moody{name: "accessibility", ready: false}
	theater := New(a)
	ctx := context.Background()

	roster := theater.Roster(ctx)
	if len(roster) != 1 || roster[0].Name != "accessibility" {
		t.Fatalf("roster did not report the one actor in the theater: %+v", roster)
	}
	if roster[0].Available {
		t.Error("the roster called an unavailable actor available")
	}
	// And casting agrees: with nobody available the Theater refuses no_actor_available.
	if p := theater.Activate(ctx, Target{Name: "Mouse"}); p.Refused != NoActorAvailable {
		t.Errorf("with an unavailable actor, casting refused %q, not %q",
			p.Refused, NoActorAvailable)
	}

	// Move the answer. Both surfaces must move.
	a.ready = true
	roster = theater.Roster(ctx)
	if len(roster) != 1 || !roster[0].Available {
		t.Fatalf("the roster still calls the actor unavailable after it became "+
			"available: %+v.\nIt is then a second opinion, and it will be wrong on "+
			"exactly the machine somebody runs it on.", roster)
	}
	if p := theater.Activate(ctx, Target{Name: "Mouse"}); p.Refused == NoActorAvailable {
		t.Error("casting still says no actor is available; the roster and casting " +
			"disagree about the same predicate")
	}
}

// Casting order is the roster's order.
//
// The order is the caller's statement about which Actor is cheapest to ask, and a roster that
// reordered it would describe a Theater nobody has.
func TestTheRosterIsInCastingOrder(t *testing.T) {
	got := New(&moody{name: "first"}, &moody{name: "second"}).Roster(context.Background())
	if len(got) != 2 || got[0].Name != "first" || got[1].Name != "second" {
		t.Errorf("the roster is not in casting order: %+v", got)
	}
}

// An empty stage and no stage at all are different answers.
func TestARosterDistinguishesAnEmptyTheaterFromNoTheater(t *testing.T) {
	ctx := context.Background()
	if got := NewHost(New()).Roster(ctx); got == nil || len(got) != 0 {
		t.Errorf("a Theater with nobody in it reported %v, want an empty roster", got)
	}
	if got := NewHost(nil).Roster(ctx); got != nil {
		t.Errorf("a host with no Theater reported %v, want nothing", got)
	}
}
