package collections_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Collection-level policy.
//
//	Repeating a safe action many times can create an unsafe outcome.
//	Bulk intent must be authorized before the first member.

func bulk(op string, count int) collections.BulkRequest {
	return collections.BulkRequest{
		Operation: op, CollectionKind: collections.KindTarget,
		Query: query(directorapi.RoleListItem, ""), MatchedCount: count,
		MaximumCount: collections.MaximumItems, Application: "explorer",
	}
}

func TestASmallLowRiskBulkActionRunsWithoutAsking(t *testing.T) {
	d := collections.EvaluateBulk(bulk("focus", 3))
	if !d.Allowed || d.RequiresConfirmation {
		t.Fatalf("decision = %+v, want silently allowed", d)
	}
	if d.Risk != directorapi.RiskLow {
		t.Fatalf("risk = %s, want low", d.Risk)
	}
}

func TestTheSameOperationAskesAboveTheAutomaticThreshold(t *testing.T) {
	// The governing rule made concrete: focus is reversible and harmless, and focusing
	// thirty controls is still a lot of focus.
	d := collections.EvaluateBulk(bulk("focus", collections.BulkAutoApproveLimit+1))
	if !d.Allowed {
		t.Fatalf("decision = %+v, want allowed with confirmation", d)
	}
	if !d.RequiresConfirmation {
		t.Fatal("a bulk action above the automatic threshold ran silently")
	}
	// The prompt states the operation, the count and the application — and nothing
	// about the members themselves.
	for _, want := range []string{"focus", "6", "explorer", "No action has run yet"} {
		if !strings.Contains(d.Prompt, want) {
			t.Errorf("prompt is missing %q:\n%s", want, d.Prompt)
		}
	}
}

func TestClickIsNeverSilentInBulk(t *testing.T) {
	// A click's effect is whatever the control does: one may toggle a checkbox and the
	// next may be "Delete all". One click passing policy says nothing about fifty.
	d := collections.EvaluateBulk(bulk("click", 2))
	if d.Allowed && !d.RequiresConfirmation {
		t.Fatalf("a bulk click ran without asking: %+v", d)
	}
	if d.Risk == directorapi.RiskLow {
		t.Fatalf("a bulk click was classified %s", d.Risk)
	}
	// And it does not claim a reversibility it has not earned.
	if strings.Contains(d.Prompt, "The effect is reversible") {
		t.Fatalf("the prompt promised reversibility it cannot know:\n%s", d.Prompt)
	}
	if !strings.Contains(d.Prompt, "not known") {
		t.Fatalf("the prompt does not admit the unknown:\n%s", d.Prompt)
	}
}

func TestAnUnknownOperationIsRefusedRatherThanAssumedSafe(t *testing.T) {
	// The important refusal. An unrecognised operation is not low risk — it is
	// unclassified, and applying it fifty times is the request that must not be guessed.
	for _, op := range []string{"delete", "submit", "send", "drag", "purchase", ""} {
		d := collections.EvaluateBulk(bulk(op, 3))
		if d.Allowed {
			t.Errorf("%q was permitted in bulk: %+v", op, d)
		}
		if !strings.Contains(d.Reason, "No items were changed") &&
			!strings.Contains(d.Reason, "nothing was changed") {
			t.Errorf("%q reason = %q, want it to say nothing happened", op, d.Reason)
		}
	}
}

func TestADestructiveLookingTargetRefusesEvenAPermittedOperation(t *testing.T) {
	// The operation may be harmless; the controls may not be. "Focus every Delete
	// button" is a request whose next step is obvious.
	r := bulk("focus", 3)
	r.Query.Element.Label = "Delete"
	d := collections.EvaluateBulk(r)
	if d.Allowed {
		t.Fatalf("a destructive-looking collection was permitted: %+v", d)
	}
	if d.Risk != directorapi.RiskHigh {
		t.Fatalf("risk = %s, want high", d.Risk)
	}

	// Also caught when the query is generic and the MEMBERS are what look destructive.
	r2 := bulk("focus", 2)
	r2.MemberLabels = []string{"Rename", "Remove account"}
	if collections.EvaluateBulk(r2).Allowed {
		t.Fatal("destructive member labels were permitted")
	}
}

func TestACountAboveTheHardLimitIsRefusedBeforeAnythingElse(t *testing.T) {
	// Refused before classification, so an enormous set cannot be talked past by being
	// made of harmless members.
	d := collections.EvaluateBulk(bulk("focus", collections.BulkAbsoluteLimit+1))
	if d.Allowed {
		t.Fatalf("a set beyond the ceiling was permitted: %+v", d)
	}
	if !strings.Contains(d.Reason, "exceeding the bulk limit") {
		t.Fatalf("reason = %q", d.Reason)
	}

	// And above the confirmation ceiling but below the absolute one: still refused,
	// with a different reason.
	d2 := collections.EvaluateBulk(bulk("focus", collections.BulkConfirmationLimit+1))
	if d2.Allowed {
		t.Fatalf("a set above the confirmation ceiling was permitted: %+v", d2)
	}
}

func TestAChangedCountInvalidatesAConfirmation(t *testing.T) {
	// A confirmation is consent to act on what the user was TOLD about. If the set grew
	// between the prompt and the answer, re-using their answer puts words in their
	// mouth.
	d := collections.EvaluateBulk(bulk("focus", 6))
	if !d.RequiresConfirmation {
		t.Fatalf("expected a confirmation: %+v", d)
	}
	if d.Stale(6) {
		t.Fatal("an unchanged set invalidated its own confirmation")
	}
	if !d.Stale(9) {
		t.Fatal("a set that grew from 6 to 9 kept its confirmation")
	}
	if !d.Stale(4) {
		t.Fatal("a set that shrank kept its confirmation")
	}

	// A silent approval has nothing to go stale: it was never a promise about a count
	// the user read.
	silent := collections.EvaluateBulk(bulk("focus", 2))
	if silent.Stale(3) {
		t.Fatal("an auto-approved decision was treated as a stale confirmation")
	}
}

func TestPolicyReasonsCarryTheCountAndOperationButNoMemberText(t *testing.T) {
	// A label may carry private text, and a policy diagnostic is the last place it
	// should surface.
	const private = "Invoice 4471 for Jane Okonkwo"
	r := bulk("focus", 8)
	r.MemberLabels = []string{private}

	d := collections.EvaluateBulk(r)
	for _, s := range []string{d.Reason, d.Prompt} {
		if strings.Contains(s, private) {
			t.Fatalf("a policy message leaked a member label: %q", s)
		}
		if strings.Contains(s, "Okonkwo") {
			t.Fatalf("a policy message leaked part of a member label: %q", s)
		}
	}
	if !strings.Contains(d.Reason, "focus") || !strings.Contains(d.Reason, "8") {
		t.Fatalf("reason = %q, want the operation and the count", d.Reason)
	}
}

func TestAnEmptySetIsNotSilentlyApproved(t *testing.T) {
	if collections.EvaluateBulk(bulk("focus", 0)).Allowed {
		t.Fatal("a zero-member bulk request was approved")
	}
}
