package theaterhost

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/activate"
)

// moody is an Actor whose availability is whatever the test says it is right now.
type moody struct {
	name     string
	ready    bool
	whyNot   string
	provider string
	path     string
}

func (m *moody) Name() string { return m.name }

// Availability answers the way a real Actor does: by SAYING WHY when it cannot act. A fake that
// only returned a bool would let the roster be tested without the field the roster exists for.
func (m *moody) Availability(context.Context) Availability {
	if m.ready {
		return Ready(m.provider, m.path)
	}
	return Unavailable(m.provider, m.path, m.whyNot)
}
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

// ── a Player cannot say both things at once ───────────────────────────────────

// contradictory is an Actor that answers ready AND hands over a reason it cannot act.
//
// Not reachable through [Ready], which is the point of that constructor — but an Actor is an
// interface anybody may implement, and the Availability it returns is a plain struct. The
// contradiction has one place it can be stopped before it reaches a person, and this is a fake
// that makes it.
type contradictory struct{ name string }

func (c *contradictory) Name() string { return c.name }
func (c *contradictory) Availability(context.Context) Availability {
	return Availability{
		Ready: true, Provider: "uia", Path: `plugins\uia\uia.exe`,
		Reason: "the bridge is not installed",
	}
}
func (c *contradictory) Find(context.Context, Target) ([]Candidate, error) { return nil, nil }
func (c *contradictory) Cast(Candidate, activate.Way) (string, bool)       { return "", false }

// A ready Player carries no reason, and an unavailable one carries the one it was given.
//
// Both halves, because the two mutations point opposite ways: dropping the guard renders
// "accessibility: ready — the bridge is not installed" in front of somebody, and dropping the
// reason entirely renders "accessibility: not available" with nothing to act on. The Cast page is
// the answer to "what can Marco do" for a person who never learns the word Theater, and it is the
// last place either could be caught.
func TestAReadyPlayerNeverCarriesAReason(t *testing.T) {
	ctx := context.Background()

	roster := New(&contradictory{name: "accessibility"}).Roster(ctx)
	if len(roster) != 1 {
		t.Fatalf("the roster reported %d players, want 1: %+v", len(roster), roster)
	}
	if p := roster[0]; p.Available && p.Reason != "" {
		t.Errorf("a player says it can act AND why it cannot: %+v.\nThe two are a "+
			"contradiction, and a surface rendering both faithfully prints "+
			"\"%s: ready — %s\".", p, p.Name, p.Reason)
	}
	// The installation is still reported: a Player that can act still says what is behind it.
	if p := roster[0]; p.Provider == "" {
		t.Errorf("the ready player lost the provider behind it: %+v", p)
	}

	// And the reason survives where there IS something to explain — the guard must be about
	// the contradiction, not about dropping the field.
	const why = `the bridge is not at plugins\uia\uia.exe`
	roster = New(&moody{name: "accessibility", whyNot: why}).Roster(ctx)
	if len(roster) != 1 || roster[0].Available {
		t.Fatalf("the unavailable actor is not in the roster as unavailable: %+v", roster)
	}
	if roster[0].Reason != why {
		t.Errorf("an unavailable player reports %q, want %q. Without it the Cast page says "+
			"something is wrong and nothing about what", roster[0].Reason, why)
	}
}

// ── a refusal carries the sentence somebody can act on ────────────────────────

// A Theater where nobody can act says WHY, per Actor.
//
// "Nothing here can act" is true and useless. The reasons were known one line up — every Actor was
// asked, and every one of them answered with a sentence — and used to be discarded on the way to
// the refusal, which is the same defect this subsystem has now had several times: the diagnosis
// existing one layer down and nothing carrying it up.
func TestAnEmptyStageSaysWhyItIsEmpty(t *testing.T) {
	a := &moody{name: "accessibility", whyNot: `the bridge is not at plugins\uia\uia.exe`}
	b := &moody{name: "vision", whyNot: "no model is installed"}

	got := New(a, b).Activate(context.Background(), Target{Name: "Mouse"})
	if got.Refused != NoActorAvailable {
		t.Fatalf("refusal %q, want %q", got.Refused, NoActorAvailable)
	}
	for _, actor := range []*moody{a, b} {
		if !strings.Contains(got.Detail, actor.name) {
			t.Errorf("the refusal never names the %s actor, so a person cannot tell which "+
				"of the two is missing:\n%s", actor.name, got.Detail)
		}
		if !strings.Contains(got.Detail, actor.whyNot) {
			t.Errorf("the refusal drops what %s said about itself (%q). The sentence with "+
				"the fix in it was known when the actor was asked and thrown away one "+
				"line later:\n%s", actor.name, actor.whyNot, got.Detail)
		}
	}
	// The refusal still reads as one: the reasons are added to the sentence, not instead of it.
	if !strings.Contains(got.Detail, "cannot go on here") {
		t.Errorf("the refusal no longer says the play cannot go on:\n%s", got.Detail)
	}
}
