package shadow_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/shadow"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Comparison must not pick a winner, and must not manufacture information gain.

func at(x, y, w, h int, role string, nameable bool) shadow.Region {
	return shadow.Region{
		Role:     directorapi.ElementRole(role),
		Bounds:   directorapi.Rect{X: x, Y: y, Width: w, Height: h},
		Nameable: nameable,
	}
}

func TestOverlappingSameRoleRegionsAgree(t *testing.T) {
	s := []shadow.Region{at(100, 100, 200, 50, "button", true)}
	a := []shadow.Region{at(102, 101, 198, 49, "button", true)}
	_, sum := shadow.Compare(s, a, true)
	if sum.Agreed != 1 || sum.Total() != 1 {
		t.Fatalf("got %+v, want exactly one AGREED", sum)
	}
}

func TestStructureOnlyOneSideSawIsNotCalledTruth(t *testing.T) {
	s := []shadow.Region{
		at(100, 100, 200, 50, "button", true),
		at(400, 400, 100, 20, "bar", false),
	}
	a := []shadow.Region{at(700, 700, 80, 80, "image", false)}
	_, sum := shadow.Compare(s, a, true)

	if sum.ShadowOnly != 2 {
		t.Errorf("ShadowOnly = %d, want 2", sum.ShadowOnly)
	}
	if sum.AuthoritativeOnly != 1 {
		t.Errorf("AuthoritativeOnly = %d, want 1", sum.AuthoritativeOnly)
	}
	// The number the milestone actually asks for: safe role-bearing structure the
	// experiment offered and belief did not have. One of the two, not both.
	if sum.ShadowOnlyNameable != 1 {
		t.Errorf("ShadowOnlyNameable = %d, want 1", sum.ShadowOnlyNameable)
	}
	if sum.Agreed != 0 {
		t.Errorf("Agreed = %d; nothing overlapped", sum.Agreed)
	}
}

func TestSameRegionDifferentRoleIsADisagreementNotAMiss(t *testing.T) {
	s := []shadow.Region{at(100, 100, 200, 50, "button", true)}
	a := []shadow.Region{at(100, 100, 200, 50, "text", true)}
	_, sum := shadow.Compare(s, a, true)

	if sum.RoleDisagreement != 1 {
		t.Fatalf("got %+v, want one ROLE_DISAGREEMENT", sum)
	}
	// Crucially NOT counted as gain on either side. Both saw the control.
	if sum.ShadowOnly != 0 || sum.AuthoritativeOnly != 0 {
		t.Errorf("a role disagreement was also counted as structure one side missed: %+v", sum)
	}
}

func TestLooseButSameRoleIsGeometryDisagreement(t *testing.T) {
	s := []shadow.Region{at(100, 100, 200, 50, "button", true)}
	// Overlaps enough to be the same control, too loosely to agree about where it is.
	a := []shadow.Region{at(100, 100, 200, 110, "button", true)}
	_, sum := shadow.Compare(s, a, true)
	if sum.GeometryDisagreement != 1 {
		t.Fatalf("got %+v, want one GEOMETRY_DISAGREEMENT", sum)
	}
}

// The one that stops a race condition being reported as information gain.
func TestUncomparableEvidenceIsNeverGain(t *testing.T) {
	s := []shadow.Region{
		at(100, 100, 200, 50, "button", true),
		at(400, 400, 100, 20, "menu", true),
	}
	a := []shadow.Region{at(700, 700, 80, 80, "image", false)}

	_, sum := shadow.Compare(s, a, false) // observed a different window generation

	if sum.Uncomparable != 3 {
		t.Fatalf("Uncomparable = %d, want 3 — every region on both sides", sum.Uncomparable)
	}
	if sum.ShadowOnly != 0 || sum.ShadowOnlyNameable != 0 {
		t.Errorf("evidence about a different world was counted as new structure: %+v", sum)
	}
	if sum.Agreed != 0 || sum.AuthoritativeOnly != 0 || sum.RoleDisagreement != 0 {
		t.Errorf("uncomparable evidence produced UI verdicts: %+v", sum)
	}
}

// Every region on both sides is accounted for exactly once, whatever the inputs.
func TestEveryRegionIsCategorisedExactlyOnce(t *testing.T) {
	s := []shadow.Region{
		at(0, 0, 100, 100, "button", true),
		at(0, 0, 100, 100, "button", true), // near-duplicate
		at(500, 0, 50, 50, "icon", false),
	}
	a := []shadow.Region{
		at(5, 5, 95, 95, "button", true),
		at(900, 900, 40, 40, "text", true),
	}
	pairs, sum := shadow.Compare(s, a, true)

	seenS := map[int]int{}
	seenA := map[int]int{}
	for _, p := range pairs {
		if p.Shadow >= 0 {
			seenS[p.Shadow]++
		}
		if p.Authoritative >= 0 {
			seenA[p.Authoritative]++
		}
	}
	for i := range s {
		if seenS[i] != 1 {
			t.Errorf("shadow region %d appears in %d pairs, want 1", i, seenS[i])
		}
	}
	for i := range a {
		if seenA[i] != 1 {
			t.Errorf("authoritative region %d appears in %d pairs, want 1", i, seenA[i])
		}
	}
	if sum.Total() != len(pairs) {
		t.Errorf("summary counts %d, pairs %d", sum.Total(), len(pairs))
	}
}

// Deterministic: the same inputs must always produce the same counts, or a session report
// cannot be reproduced or compared with the next one.
func TestComparisonIsDeterministic(t *testing.T) {
	s := []shadow.Region{
		at(0, 0, 100, 100, "button", true),
		at(2, 2, 100, 100, "button", true),
		at(4, 4, 100, 100, "button", true),
	}
	a := []shadow.Region{
		at(1, 1, 100, 100, "button", true),
		at(3, 3, 100, 100, "button", true),
	}
	_, first := shadow.Compare(s, a, true)
	for i := 0; i < 50; i++ {
		if _, again := shadow.Compare(s, a, true); again != first {
			t.Fatalf("run %d produced %+v, first run %+v", i, again, first)
		}
	}
}
