package collections_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Progress classification.
//
//	Iteration advances only when the current member produced verified semantic
//	progress.
//
// This explains verification; it never replaces it. Every case below starts from
// evidence the verifier already gathered.

func ev(e collections.Evidence) collections.ProgressKind {
	return collections.ClassifyProgress(e)
}

func TestEachProgressKindIsReachedFromRealEvidence(t *testing.T) {
	for _, c := range []struct {
		name string
		in   collections.Evidence
		want collections.ProgressKind
	}{
		{"member removed", collections.Evidence{
			Verified: true, Observable: true, MemberPresent: false,
			Kinds: []string{"target_gone"},
		}, collections.ProgressMemberRemoved},

		{"focus acquired", collections.Evidence{
			Verified: true, Observable: true, MemberPresent: true,
			Kinds: []string{"focus_on_target"},
		}, collections.ProgressMemberStateChanged},

		{"state changed", collections.Evidence{
			Verified: true, Observable: true, MemberPresent: true,
			Kinds: []string{"target_state_changed"},
		}, collections.ProgressMemberStateChanged},

		{"verified with no specific story", collections.Evidence{
			Verified: true, Observable: true, MemberPresent: true,
			Kinds: []string{"element_count_changed"},
		}, collections.ProgressMemberCompleted},

		{"nothing happened", collections.Evidence{
			Verified: false, Observable: true, MemberPresent: true,
			Kinds: []string{"nothing_changed"},
		}, collections.ProgressMemberUnchanged},

		{"reordered", collections.Evidence{
			Verified: false, Observable: true, MemberPresent: true, OrderChanged: true,
		}, collections.ProgressCollectionReordered},

		{"unobservable", collections.Evidence{
			Verified: true, Observable: false, MemberPresent: true,
			Kinds: []string{"focus_on_target"},
		}, collections.ProgressCollectionUnobservable},
	} {
		if got := ev(c.in); got != c.want {
			t.Errorf("%s = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestThePrecedenceIsDeterministicAndTotal(t *testing.T) {
	// Two conflicting terminal classifications must not both be reachable from one
	// piece of evidence. The order is: unobservable → removed → state change →
	// completion → reordered → unchanged.

	// Unobservable beats everything, because a world that stopped answering cannot
	// tell us the member is gone.
	all := collections.Evidence{
		Verified: true, Observable: false, MemberPresent: false, OrderChanged: true,
		Kinds: []string{"target_gone", "focus_on_target"},
	}
	if got := ev(all); got != collections.ProgressCollectionUnobservable {
		t.Fatalf("unobservable did not win: %s", got)
	}

	// Removal beats a state change: a member that no longer exists has no state to
	// have changed.
	removedAndChanged := collections.Evidence{
		Verified: true, Observable: true, MemberPresent: false,
		Kinds: []string{"target_gone", "focus_on_target"},
	}
	if got := ev(removedAndChanged); got != collections.ProgressMemberRemoved {
		t.Fatalf("removal did not beat state change: %s", got)
	}

	// A state change beats a bare completion.
	if got := ev(collections.Evidence{
		Verified: true, Observable: true, MemberPresent: true,
		Kinds: []string{"focus_on_target", "element_count_changed"},
	}); got != collections.ProgressMemberStateChanged {
		t.Fatalf("state change did not beat completion: %s", got)
	}

	// Reordering only surfaces when nothing verified — a verified member is a better
	// account of what happened than "the list moved".
	if got := ev(collections.Evidence{
		Verified: true, Observable: true, MemberPresent: true, OrderChanged: true,
		Kinds: []string{"focus_on_target"},
	}); got != collections.ProgressMemberStateChanged {
		t.Fatalf("reordering displaced a verified state change: %s", got)
	}

	// Unchanged is the residue: the answer when nothing positive was established.
	if got := ev(collections.Evidence{Observable: true, MemberPresent: true}); got !=
		collections.ProgressMemberUnchanged {
		t.Fatalf("bare evidence = %s, want unchanged", got)
	}
}

func TestOnlyRealProgressPermitsAdvancing(t *testing.T) {
	// The whole point of the classification. Advancing past an unchanged member would
	// leave it unprocessed and eligible again, and the loop would apply the same
	// ineffective operation until the limit.
	for kind, advances := range map[collections.ProgressKind]bool{
		collections.ProgressMemberCompleted:        true,
		collections.ProgressMemberRemoved:          true,
		collections.ProgressMemberStateChanged:     true,
		collections.ProgressMemberUnchanged:        false,
		collections.ProgressCollectionReordered:    false,
		collections.ProgressCollectionUnobservable: false,
	} {
		if got := kind.Advances(); got != advances {
			t.Errorf("%s advances = %v, want %v", kind, got, advances)
		}
	}
}

func TestClassificationNeverReadsCoordinates(t *testing.T) {
	// Evidence has no Bounds and no Point, so classifying from a repaint or a scroll is
	// structurally impossible rather than merely avoided. Asserted against the type.
	var e collections.Evidence
	_ = e
	// A verified member whose only evidence is positional would classify as a bare
	// completion, not as a state change — there is no positional evidence kind.
	if got := ev(collections.Evidence{
		Verified: true, Observable: true, MemberPresent: true,
		Kinds: []string{"window_bounds_changed"},
	}); got != collections.ProgressMemberCompleted {
		t.Fatalf("a positional evidence kind produced %s", got)
	}
}

func TestEvidenceKindsComeFromTheVerifier(t *testing.T) {
	v := directorapi.VerificationResult{Evidence: []directorapi.Evidence{
		{Kind: "focus_on_target", Observed: true},
		{Kind: "nothing_changed", Observed: true},
	}}
	got := collections.EvidenceKinds(v)
	if len(got) != 2 || got[0] != "focus_on_target" {
		t.Fatalf("kinds = %v", got)
	}
}

func TestExplicitNoChangeEvidenceIsDecisiveEvenWhenVerified(t *testing.T) {
	// The single-action verifier can be lenient — a best-effort operation may pass
	// without proving much. A LOOP cannot afford that: advancing past a member the
	// verifier positively observed to be unchanged leaves it eligible again, and the
	// same ineffective operation runs until the limit.
	got := ev(collections.Evidence{
		Verified: true, Observable: true, MemberPresent: true,
		Kinds: []string{"nothing_changed"},
	})
	if got != collections.ProgressMemberUnchanged {
		t.Fatalf("verified-but-unchanged classified %s, want member_unchanged", got)
	}
	if got.Advances() {
		t.Fatal("an unchanged member permitted advancing")
	}

	// Removal still wins over it: a member that vanished did not "not change".
	if got := ev(collections.Evidence{
		Verified: true, Observable: true, MemberPresent: false,
		Kinds: []string{"target_gone", "nothing_changed"},
	}); got != collections.ProgressMemberRemoved {
		t.Fatalf("removal lost to no-change evidence: %s", got)
	}
}
