package fusion

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/internal/fixtures"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

var t0 = time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

// accObs builds a synthetic accessibility observation with a native id, which is what
// the identity matcher keys on.
func accObs(nativeID string, role directorapi.ElementRole, label string, r directorapi.Rect) directorapi.Observation {
	enabled, visible := true, true
	return directorapi.Observation{
		ID:         directorapi.ObservationID("acc:" + nativeID),
		Source:     directorapi.SourceAccessibility,
		WindowID:   "w1",
		Role:       role,
		Label:      label,
		Bounds:     r,
		Enabled:    &enabled,
		Visible:    &visible,
		Confidence: 1,
		NativeID:   nativeID,
	}
}

// assign fuses and assigns in one step, returning the elements by label.
func assign(t *testing.T, tr *Tracker, obs []directorapi.Observation) map[string]*directorapi.Element {
	t.Helper()
	f := Fuse(obs)
	tr.Assign(f, t0)
	out := map[string]*directorapi.Element{}
	for _, e := range f {
		out[e.Element.Label] = e.Element
	}
	return out
}

// The whole point of the package: an element the platform still calls the same
// thing must keep its Director identity. "Click that again" depends on it.
func TestNativeIDCarriesIdentityForward(t *testing.T) {
	tr := New()
	first := assign(t, tr, []directorapi.Observation{
		accObs("uia:1.100", directorapi.RoleButton, "Save", rect(900, 700, 90, 30)),
		accObs("uia:1.200", directorapi.RoleButton, "Cancel", rect(1000, 700, 90, 30)),
	})
	second := assign(t, tr, []directorapi.Observation{
		accObs("uia:1.100", directorapi.RoleButton, "Save", rect(900, 700, 90, 30)),
		accObs("uia:1.200", directorapi.RoleButton, "Cancel", rect(1000, 700, 90, 30)),
	})

	if first["Save"].ID != second["Save"].ID {
		t.Errorf("Save changed identity: %q → %q", first["Save"].ID, second["Save"].ID)
	}
	if first["Cancel"].ID != second["Cancel"].ID {
		t.Errorf("Cancel changed identity: %q → %q", first["Cancel"].ID, second["Cancel"].ID)
	}
	if second["Save"].FirstSeen != t0 {
		t.Error("FirstSeen should carry forward, not be restamped each snapshot")
	}
}

// A dragged window moves every control. Identity has to survive that or "put it
// back" and "click that again" break the moment anyone moves a window.
func TestIdentitySurvivesAWindowMove(t *testing.T) {
	tr := New()
	first := assign(t, tr, []directorapi.Observation{
		accObs("uia:1.100", directorapi.RoleButton, "Save", rect(900, 700, 90, 30)),
	})
	// Same control, new position, and the platform reissued its runtime id (which
	// happens when a window is recreated rather than moved).
	second := assign(t, tr, []directorapi.Observation{
		accObs("uia:9.999", directorapi.RoleButton, "Save", rect(1400, 300, 90, 30)),
	})

	if first["Save"].ID != second["Save"].ID {
		t.Errorf("a moved button lost its identity: %q → %q", first["Save"].ID, second["Save"].ID)
	}
}

// The dangerous case. A wrong carry-forward makes "do that again" act on a
// DIFFERENT control while reporting full confidence — far worse than minting a new
// ID, which merely costs the user a re-reference.
func TestIdenticalSiblingsDoNotSwapIdentities(t *testing.T) {
	tr := New()
	left := rect(60, 140, 90, 30)
	right := rect(320, 140, 90, 30)

	build := func(leftNative, rightNative string) []directorapi.Observation {
		l := accObs(leftNative, directorapi.RoleButton, "Apply", left)
		l.ParentNativeID = "grp:audio"
		r := accObs(rightNative, directorapi.RoleButton, "Apply", right)
		r.ParentNativeID = "grp:video"
		return []directorapi.Observation{
			accObs("grp:audio", directorapi.RoleGroup, "Audio", rect(16, 16, 220, 200)),
			accObs("grp:video", directorapi.RoleGroup, "Video", rect(260, 16, 220, 200)),
			l, r,
		}
	}

	f1 := Fuse(build("uia:1.10", "uia:1.20"))
	tr.Assign(f1, t0)
	idByParent := map[string]directorapi.ElementID{}
	for _, e := range f1 {
		if e.Element.Label == "Apply" {
			idByParent[e.ParentNativeID] = e.Element.ID
		}
	}
	if len(idByParent) != 2 {
		t.Fatalf("expected two Apply buttons under distinct parents, got %d", len(idByParent))
	}

	// Both runtime ids change (the dialog was rebuilt), so only structure can tell
	// them apart.
	f2 := Fuse(build("uia:2.10", "uia:2.20"))
	tr.Assign(f2, t0.Add(time.Second))
	for _, e := range f2 {
		if e.Element.Label != "Apply" {
			continue
		}
		want := idByParent[e.ParentNativeID]
		if e.Element.ID != want {
			t.Errorf("the Apply button under %q got identity %q, want %q — identities were swapped",
				e.ParentNativeID, e.Element.ID, want)
		}
	}
}

// An identity may be inherited by at most one element. Without that, a row of
// identical controls would all claim the first one's ID.
func TestIdentityAssignmentIsOneToOne(t *testing.T) {
	tr := New()
	assign(t, tr, []directorapi.Observation{
		accObs("uia:1.1", directorapi.RoleListItem, "Row", rect(0, 0, 200, 20)),
	})

	f := Fuse([]directorapi.Observation{
		accObs("uia:2.1", directorapi.RoleListItem, "Row", rect(0, 0, 200, 20)),
		accObs("uia:2.2", directorapi.RoleListItem, "Row", rect(0, 20, 200, 20)),
		accObs("uia:2.3", directorapi.RoleListItem, "Row", rect(0, 40, 200, 20)),
	})
	tr.Assign(f, t0)

	seen := map[directorapi.ElementID]int{}
	for _, e := range f {
		seen[e.Element.ID]++
	}
	if len(seen) != 3 {
		t.Fatalf("three rows must have three distinct identities, got %d", len(seen))
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("identity %q was assigned %d times", id, n)
		}
	}
}

// An AutomationId is application-authored and survives recreation, where a runtime
// id does not. But it is only usable when it identifies exactly one element.
func TestAutomationIDMatchesAcrossRecreation(t *testing.T) {
	withAutoID := func(nativeID, autoID string) directorapi.Observation {
		o := accObs(nativeID, directorapi.RoleButton, "", rect(10, 10, 80, 24))
		o.Attributes = map[string]any{"automation_id": autoID}
		return o
	}

	tr := New()
	f1 := Fuse([]directorapi.Observation{withAutoID("uia:1.1", "btnSubmit")})
	tr.Assign(f1, t0)
	want := f1[0].Element.ID

	// Dialog closed and reopened: new runtime id, same authored id, moved.
	f2 := Fuse([]directorapi.Observation{withAutoID("uia:7.7", "btnSubmit")})
	tr.Assign(f2, t0)
	if f2[0].Element.ID != want {
		t.Errorf("AutomationId should carry identity across recreation: %q → %q", want, f2[0].Element.ID)
	}
}

// Toolkits reuse automation ids across list rows. An ambiguous id is worse than no
// id: it would confidently assign the wrong identity.
func TestDuplicateAutomationIDsAreNotUsed(t *testing.T) {
	row := func(nativeID string, y int) directorapi.Observation {
		o := accObs(nativeID, directorapi.RoleListItem, "", rect(0, y, 200, 20))
		o.Attributes = map[string]any{"automation_id": "item"} // reused by the toolkit
		return o
	}

	tr := New()
	f1 := Fuse([]directorapi.Observation{row("uia:1.1", 0), row("uia:1.2", 20)})
	tr.Assign(f1, t0)

	f2 := Fuse([]directorapi.Observation{row("uia:2.1", 0), row("uia:2.2", 20)})
	tr.Assign(f2, t0)

	ids := map[directorapi.ElementID]bool{}
	for _, e := range f2 {
		ids[e.Element.ID] = true
	}
	if len(ids) != 2 {
		t.Errorf("ambiguous automation ids must not collapse two rows into one identity, got %v", ids)
	}
}

// An object does not change what kind of thing it is, or which window it is in.
func TestRoleAndWindowAreHardGates(t *testing.T) {
	tr := New()
	f1 := Fuse([]directorapi.Observation{
		accObs("uia:1.1", directorapi.RoleButton, "Go", rect(10, 10, 80, 24)),
	})
	tr.Assign(f1, t0)
	buttonID := f1[0].Element.ID

	// Same label and place, different role and different runtime id.
	f2 := Fuse([]directorapi.Observation{
		accObs("uia:2.1", directorapi.RoleCheckbox, "Go", rect(10, 10, 80, 24)),
	})
	tr.Assign(f2, t0)
	if f2[0].Element.ID == buttonID {
		t.Error("a checkbox must not inherit a button's identity")
	}

	other := accObs("uia:3.1", directorapi.RoleButton, "Go", rect(10, 10, 80, 24))
	other.WindowID = "w2"
	f3 := Fuse([]directorapi.Observation{other})
	tr.Assign(f3, t0)
	if f3[0].Element.ID == buttonID {
		t.Error("an element in another window must not inherit the identity")
	}
}

// A new element must get a genuinely new ID, never one recycled from something that
// has gone away.
func TestNewElementsGetFreshIDs(t *testing.T) {
	tr := New()
	f1 := Fuse([]directorapi.Observation{
		accObs("uia:1.1", directorapi.RoleButton, "Save", rect(0, 0, 80, 24)),
	})
	tr.Assign(f1, t0)
	old := f1[0].Element.ID

	// The Save button is gone; a completely different control appears.
	f2 := Fuse([]directorapi.Observation{
		accObs("uia:2.1", directorapi.RoleTextField, "Search", rect(500, 500, 200, 24)),
	})
	tr.Assign(f2, t0)

	if f2[0].Element.ID == old {
		t.Error("a new element must not recycle a departed element's identity")
	}
	if f2[0].Element.FirstSeen != t0 {
		t.Error("a new element should be stamped with the time it appeared")
	}
}

// Parent links must be resolved to real element IDs, or "the Apply in the Video
// group" has nothing to stand on.
func TestParentLinksAreResolved(t *testing.T) {
	child := accObs("uia:1.2", directorapi.RoleButton, "Apply", rect(60, 140, 90, 30))
	child.ParentNativeID = "uia:1.1"

	f := Fuse([]directorapi.Observation{
		accObs("uia:1.1", directorapi.RoleGroup, "Audio", rect(16, 16, 220, 200)),
		child,
	})
	New().Assign(f, t0)

	var group, button *directorapi.Element
	for _, e := range f {
		switch e.Element.Label {
		case "Audio":
			group = e.Element
		case "Apply":
			button = e.Element
		}
	}
	if group == nil || button == nil {
		t.Fatal("expected both elements")
	}
	if button.ParentID == nil {
		t.Fatal("the button's parent link was not resolved")
	}
	if *button.ParentID != group.ID {
		t.Errorf("parent = %q, want the group's id %q", *button.ParentID, group.ID)
	}
	if group.ParentID != nil {
		t.Error("the root element should have no parent")
	}
}

// The real dialog, observed twice: every element must keep its identity.
func TestFixtureIdentityIsStableAcrossSnapshots(t *testing.T) {
	d := fixtures.Load(t, "save-dialog")
	tr := New()

	f1 := Fuse(d.Observations)
	tr.Assign(f1, t0)
	f2 := Fuse(d.Observations)
	tr.Assign(f2, t0.Add(time.Second))

	if len(f1) != len(f2) {
		t.Fatalf("element counts differ: %d vs %d", len(f1), len(f2))
	}
	changed := 0
	for i := range f1 {
		if f1[i].Element.ID != f2[i].Element.ID {
			changed++
			t.Logf("element %q (%s) changed identity: %s → %s",
				f1[i].Element.Label, f1[i].Element.Role, f1[i].Element.ID, f2[i].Element.ID)
		}
	}
	if changed != 0 {
		t.Errorf("%d of %d elements changed identity across identical snapshots", changed, len(f1))
	}
}

// Identity must not depend on how many times the tracker has run.
func TestRepeatedSnapshotsDoNotDriftIDs(t *testing.T) {
	d := fixtures.Load(t, "duplicate-labels")
	tr := New()

	f := Fuse(d.Observations)
	tr.Assign(f, t0)
	want := make([]directorapi.ElementID, len(f))
	for i := range f {
		want[i] = f[i].Element.ID
	}

	for round := range 10 {
		f = Fuse(d.Observations)
		tr.Assign(f, t0.Add(time.Duration(round)*time.Second))
		for i := range f {
			if f[i].Element.ID != want[i] {
				t.Fatalf("round %d: element %d drifted %q → %q", round, i, want[i], f[i].Element.ID)
			}
		}
	}
}

// The first snapshot has nothing to match against; it must simply mint.
func TestFirstSnapshotMintsEverything(t *testing.T) {
	d := fixtures.Load(t, "disabled-button")
	f := Fuse(d.Observations)
	New().Assign(f, t0)

	seen := map[directorapi.ElementID]bool{}
	for _, e := range f {
		if e.Element.ID == "" {
			t.Fatalf("element %q got no id", e.Element.Label)
		}
		if seen[e.Element.ID] {
			t.Fatalf("duplicate id %q", e.Element.ID)
		}
		seen[e.Element.ID] = true
	}
}

func TestEmptySnapshotIsHarmless(t *testing.T) {
	tr := New()
	tr.Assign(nil, t0)
	f := Fuse([]directorapi.Observation{
		accObs("uia:1.1", directorapi.RoleButton, "Save", rect(0, 0, 80, 24)),
	})
	tr.Assign(f, t0)
	if f[0].Element.ID == "" {
		t.Error("assignment should still work after an empty snapshot")
	}
}
