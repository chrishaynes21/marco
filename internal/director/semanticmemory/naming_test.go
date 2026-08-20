package semanticmemory_test

import (
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// What the Audience calls a place, and their right to change their mind about it.
//
// # The live failure this page exists for
//
// A person was asked to name a screen. The question could not show them WHICH screen, so they
// answered with the word they had in mind for a different one — "Mouse Settings", given to the
// Bluetooth page. Then:
//
//   - withdrawing the answer removed the judgement and left the name;
//   - uniqueness reserved that word against every other place, forever;
//   - so the screen they had actually meant could never receive it.
//
// The only repair was editing semantic-memory.json by hand, with the Director stopped. That is
// not an acceptable thing to ask of somebody who mistyped an answer to a question Marco asked at
// the moment it was least sure.
//
// Every test here is one clause of the contract that replaces it.

// place is a distinct, recognisable screen.
func place(term observe.InterfaceTerm) observe.StructureSignature {
	return observe.StructureSignature{
		Subject:    observe.SubjectState,
		Roles:      map[string]int{"button": 5},
		Terms:      []observe.InterfaceTerm{term},
		TermsKnown: true,
	}
}

// twoPlaces establishes two distinct places and returns the store and their ids.
func twoPlaces(t *testing.T, dir string) (*semanticmemory.Store, string, string) {
	t.Helper()
	store, note := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if note != "" {
		t.Fatalf("opening: %s", note)
	}
	a, err := store.EstablishPlace("settings", place(observe.TermAudio))
	if err != nil {
		t.Fatalf("establishing A: %v", err)
	}
	b, err := store.EstablishPlace("settings", place(observe.TermDisplay))
	if err != nil {
		t.Fatalf("establishing B: %v", err)
	}
	if a == b {
		t.Fatal("the fixture built one place twice; nothing below would mean anything")
	}
	return store, a, b
}

func named(t *testing.T, s *semanticmemory.Store, id, name string) {
	t.Helper()
	n, err := observe.UserSuppliedScreenName(name)
	if err != nil {
		t.Fatalf("the name %q is not usable: %v", name, err)
	}
	if err := s.NameSubject("settings", id, n); err != nil {
		t.Fatalf("naming %s %q: %v", id, name, err)
	}
}

func calledNow(s *semanticmemory.Store, id string) string {
	for _, r := range s.Subjects() {
		if r.ID == id {
			return r.Called
		}
	}
	return "<gone>"
}

// ── THE regression ────────────────────────────────────────────────────────────

// The exact wrong-screen naming failure, corrected without touching the file.
//
// This is the acceptance requirement stated as a test: no store surgery, no deleting
// semantic-memory.json, no cold reset. A person realises they named the wrong screen and fixes it
// with the operations the product offers.
func TestNamingTheWrongScreenIsCorrectableWithoutStoreSurgery(t *testing.T) {
	dir := t.TempDir()
	store, a, b := twoPlaces(t, dir)

	// Marco asks about A. The person means B, and answers anyway.
	named(t, store, a, "Mouse Settings")

	// They realise. A is actually the Bluetooth page.
	named(t, store, a, "Bluetooth Settings")
	if got := calledNow(store, a); got != "Bluetooth Settings" {
		t.Fatalf("after the correction A is called %q", got)
	}

	// AND THE OLD WORD IS FREE. This is the clause that failed live: uniqueness held
	// "Mouse Settings" against every other place even after nothing was called it.
	if err := func() error {
		n, _ := observe.UserSuppliedScreenName("Mouse Settings")
		return store.NameSubject("settings", b, n)
	}(); err != nil {
		t.Fatalf("the screen the person actually meant could not take the name they meant "+
			"for it: %v.\nA word nothing is called must be available. Reserving every "+
			"string ever typed means one mistaken answer burns it forever, and the only "+
			"repair is editing the file by hand — which is what happened.", err)
	}
	if got := calledNow(store, b); got != "Mouse Settings" {
		t.Fatalf("B is called %q", got)
	}
	// And A kept its correction rather than losing it to B's naming.
	if got := calledNow(store, a); got != "Bluetooth Settings" {
		t.Errorf("A is called %q after B was named", got)
	}
}

// ── the three transitions ─────────────────────────────────────────────────────

// Renaming keeps the SAME place. It does not mint a new one.
//
// Mutation: implement rename by establishing a new subject and copying. Every route, goal and
// target pointing at the old id would be orphaned, and nothing would say so.
func TestRenamingKeepsTheSamePlace(t *testing.T) {
	dir := t.TempDir()
	store, a, _ := twoPlaces(t, dir)
	before := len(store.Subjects())

	named(t, store, a, "Mouse Settings")
	named(t, store, a, "Bluetooth Settings")

	if got := len(store.Subjects()); got != before {
		t.Fatalf("the store holds %d subject(s) after a rename, was %d. Renaming a place "+
			"must not create one — everything that points at it would be pointing at "+
			"the old one.", got, before)
	}
	if calledNow(store, a) != "Bluetooth Settings" {
		t.Errorf("A is called %q", calledNow(store, a))
	}
}

// Unnaming keeps the place and releases the word.
//
// Mutation: delete the subject. The place stops being recognisable at all, and every route
// through it dies — because somebody took back a name.
func TestUnnamingKeepsThePlaceAndReleasesTheWord(t *testing.T) {
	dir := t.TempDir()
	store, a, b := twoPlaces(t, dir)
	named(t, store, a, "Mouse Settings")
	before := len(store.Subjects())

	if err := store.UnnameSubject("settings", a); err != nil {
		t.Fatalf("unnaming: %v", err)
	}
	if got := len(store.Subjects()); got != before {
		t.Fatalf("the store holds %d subject(s) after a name was taken back, was %d. "+
			"Removing what somebody CALLS a place says nothing about the place.",
			got, before)
	}
	if got := calledNow(store, a); got != "" {
		t.Errorf("A is still called %q", got)
	}
	// The word is free.
	n, _ := observe.UserSuppliedScreenName("Mouse Settings")
	if err := store.NameSubject("settings", b, n); err != nil {
		t.Errorf("the released word could not be reused: %v", err)
	}
}

// Unnaming something nobody named is not an error.
func TestUnnamingAnUnnamedPlaceIsFine(t *testing.T) {
	dir := t.TempDir()
	store, a, _ := twoPlaces(t, dir)
	if err := store.UnnameSubject("settings", a); err != nil {
		t.Errorf("unnaming an unnamed place: %v", err)
	}
}

// Two places may not share a name at the same time.
//
// The rule uniqueness is actually for, kept while the over-reach is removed.
func TestTwoPlacesMayNotShareANameAtOnce(t *testing.T) {
	dir := t.TempDir()
	store, a, b := twoPlaces(t, dir)
	named(t, store, a, "Mouse Settings")

	n, _ := observe.UserSuppliedScreenName("Mouse Settings")
	if err := store.NameSubject("settings", b, n); err == nil {
		t.Error("two different places are both called Mouse Settings; a name has to mean " +
			"one place or a play cannot say where it begins")
	}
}

// ── durability ────────────────────────────────────────────────────────────────

// A correction survives a restart, and so does a retraction.
func TestNamesAndTheirRemovalSurviveARestart(t *testing.T) {
	dir := t.TempDir()
	store, a, b := twoPlaces(t, dir)
	named(t, store, a, "Mouse Settings")
	named(t, store, a, "Bluetooth Settings")
	named(t, store, b, "Mouse Settings")
	if err := store.UnnameSubject("settings", b); err != nil {
		t.Fatalf("unnaming B: %v", err)
	}

	// THE RESTART.
	reopened, note := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if note != "" {
		t.Fatalf("reopening: %s", note)
	}
	if got := calledNow(reopened, a); got != "Bluetooth Settings" {
		t.Errorf("after the restart A is called %q, want the correction", got)
	}
	if got := calledNow(reopened, b); got != "" {
		t.Errorf("after the restart B is called %q, want nothing — the name was taken back",
			got)
	}
	// And the released word is still free on the other side of the restart.
	n, _ := observe.UserSuppliedScreenName("Mouse Settings")
	if err := reopened.NameSubject("settings", b, n); err != nil {
		t.Errorf("a word released before the restart is reserved after it: %v", err)
	}
}

// ── the boundary 34B drew ─────────────────────────────────────────────────────

// What perception observed never becomes what the Audience called something.
//
// A durable target carries the word on the control as EVIDENCE. That is not somebody naming it,
// and the two must never be stored as the same kind of claim — otherwise an observed label
// outranks a person's correction, which is the one thing authorship must never lose.
func TestAnObservedLabelIsNotAnAudienceName(t *testing.T) {
	dir := t.TempDir()
	store, a, _ := twoPlaces(t, dir)

	sig := observe.TargetSignature(a, "Mouse", observe.KindButton)
	id, err := store.RememberTarget("settings", sig, observe.FromAccessible)
	if err != nil {
		t.Fatalf("remembering the target: %v", err)
	}
	for _, r := range store.Subjects() {
		if r.ID != id {
			continue
		}
		if r.Called != "" {
			t.Fatalf("a target Marco merely OBSERVED is Called %q. Nobody named it; the "+
				"word came off the screen, and recording it as the Audience's own would "+
				"let perception outrank a correction.", r.Called)
		}
		if r.Structure.Label != "Mouse" {
			t.Errorf("the observed label is %q, want it kept as evidence",
				r.Structure.Label)
		}
	}
	// And the observed label does not reserve the word against a place.
	n, _ := observe.UserSuppliedScreenName("Mouse")
	if err := store.NameSubject("settings", a, n); err != nil {
		t.Errorf("a word Marco merely observed on a control blocked somebody from using "+
			"it as a name: %v", err)
	}
}
