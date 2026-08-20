package semanticmemory

import (
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Two different targets are two different subjects.
//
// # The live collision
//
// A real store held five subject records under one id, describing "Bluetooth & devices" on Home,
// "Mouse" on two different pages, and "Bluetooth & devices" on another. A target carries no roles
// and no terms — a screen is recognised by what it is MADE OF, a target by what it is CALLED — so
// every target in one application hashed to the same value and each write appended another record
// under it.
//
// Deleting the target fields from subjectID must fail this.
func TestTwoDifferentTargetsAreNotTheSameSubject(t *testing.T) {
	cases := []struct {
		name string
		a, b observe.StructureSignature
	}{
		{"different labels in the same place",
			observe.TargetSignature("subj_home", "Bluetooth & devices", "item"),
			observe.TargetSignature("subj_home", "Mouse", "item")},
		{"the same label in different places",
			observe.TargetSignature("subj_home", "Mouse", "item"),
			observe.TargetSignature("subj_bluetooth", "Mouse", "item")},
		{"the same label of a different kind",
			observe.TargetSignature("subj_home", "Mouse", "item"),
			observe.TargetSignature("subj_home", "Mouse", "button")},
	}
	for _, c := range cases {
		if got, want := subjectID("settings", c.a), subjectID("settings", c.b); got == want {
			t.Errorf("%s: both are %s.\nThe store cannot tell them apart, so every write "+
				"appends another record under one id and a play aimed at one may be "+
				"answered by the other.", c.name, got)
		}
	}
}

// The same target is the same subject, every time.
//
// The other half: an id that varied would orphan every route pointing at it.
func TestTheSameTargetIsAlwaysTheSameSubject(t *testing.T) {
	a := observe.TargetSignature("subj_home", "Mouse", "item")
	b := observe.TargetSignature("subj_home", "  Mouse  ", "item") // TargetSignature trims
	if subjectID("settings", a) != subjectID("settings", b) {
		t.Error("the same target produced two ids; every route pointing at it is orphaned")
	}
	if subjectID("settings", a) != subjectID("settings", a) {
		t.Error("the id is not stable for one signature")
	}
}

// A target and a place with the same application do not collide either.
func TestATargetIsNotAPlace(t *testing.T) {
	place := observe.StructureSignature{Subject: observe.SubjectState}
	target := observe.TargetSignature("subj_home", "", "")
	if subjectID("settings", place) == subjectID("settings", target) {
		t.Error("an empty place and an empty target are the same subject")
	}
}

// Writing four different targets leaves four records, not four copies of one.
//
// Entered through the store's own remembering path, so it fails if the id derivation regresses
// however the signature is built.
func TestFourTargetsLeaveFourSubjects(t *testing.T) {
	store, unavailable := Open(filepath.Join(t.TempDir(), "memory.json"))
	if unavailable != "" {
		t.Fatalf("open: %s", unavailable)
	}
	targets := []observe.StructureSignature{
		observe.TargetSignature("subj_home", "Bluetooth & devices", "item"),
		observe.TargetSignature("subj_home", "Mouse", "item"),
		observe.TargetSignature("subj_bt", "Mouse", "item"),
		observe.TargetSignature("subj_bt", "Bluetooth & devices", "item"),
	}
	ids := map[string]bool{}
	for _, sig := range targets {
		id, err := store.EstablishPlace("settings", sig)
		if err != nil {
			t.Fatalf("establishing %q: %v", sig.Label, err)
		}
		ids[id] = true
	}
	if len(ids) != len(targets) {
		t.Errorf("%d target(s) produced %d subject id(s); the store cannot tell them apart",
			len(targets), len(ids))
	}
	seen := map[string]int{}
	for _, s := range store.Subjects() {
		seen[s.ID]++
	}
	for id, n := range seen {
		if n > 1 {
			t.Errorf("subject %s has %d records; one identity is one record", id, n)
		}
	}
}
