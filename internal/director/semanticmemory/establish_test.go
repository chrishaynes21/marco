package semanticmemory_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// Persisting a place's IDENTITY without persisting a judgement about it.
//
// The store always could do this — a `not now` answer mints a subject and asserts nothing — but
// the only door to the canonical subject path ran through a person answering a semantic question.
// These tests hold the second door open and, more importantly, hold it NARROW: identity in,
// nothing else, and no route by which repeated observation could edit what somebody once settled.

// ── the claim, and the one it is not ──────────────────────────────────────────

// An established place is recognised after a restart and claims nothing.
func TestAnEstablishedPlaceIsRecognisedAndCarriesNoJudgement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")

	first := open(t, path)
	id, err := first.EstablishPlace("unknown-game", settingsSignature())
	if err != nil {
		t.Fatalf("EstablishPlace: %v", err)
	}
	if id == "" {
		t.Fatal("no subject id came back")
	}

	// A different process, as far as this store is concerned.
	second := open(t, path)
	rec := second.Recall("unknown-game", settingsSignature())
	if rec.Verdict != observe.MatchSame {
		t.Fatalf("verdict %q after reopening; a place established in one sitting is not "+
			"recognised in the next, which is the whole reason to make it durable", rec.Verdict)
	}
	if rec.Subject.ID != id {
		t.Errorf("recall resolved to %q, not the established subject %q", rec.Subject.ID, id)
	}
	if n := len(rec.Subject.Knowledge); n != 0 {
		t.Fatalf("the place carries %d interpretation(s): %+v.\nNobody was asked anything. "+
			"Persisting identity and persisting a judgement are separate claims and this is "+
			"the line between them", n, rec.Subject.Knowledge)
	}
	// And nothing downstream reads it as an answer.
	for _, kind := range []observe.HypothesisKind{
		observe.PossibleSettingsLikeState, observe.PossibleMenuLikeState,
	} {
		if _, ok := observe.RecalledValidation(rec, kind); ok {
			t.Errorf("an established place produced user validation for %s", kind)
		}
	}
}

// The application namespace still applies.
func TestAnEstablishedPlaceIsNotRecognisedInAnotherApplication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	s := open(t, path)
	if _, err := s.EstablishPlace("game-one", settingsSignature()); err != nil {
		t.Fatalf("EstablishPlace: %v", err)
	}
	if v := s.Recall("game-two", settingsSignature()).Verdict; v.Established() {
		t.Errorf("a place established in game-one is recognised in game-two (%q). Two "+
			"programs that happen to present the same shape do not mean the same thing by it", v)
	}
}

// ── what it must never do ─────────────────────────────────────────────────────

// Establishing a place a person has already settled leaves their answer exactly as it was.
//
// The worst available failure in this mechanism: a teach attempt walking over a judgement because
// the user happened to start teaching on a screen they had answered a question about last week.
func TestEstablishingAPlaceNeverTouchesAnExistingJudgement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")

	first := open(t, path)
	if err := first.Remember("unknown-game", settingsSignature(),
		confirmed(observe.PossibleSettingsLikeState)); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	before := first.Recall("unknown-game", settingsSignature()).Subject

	id, err := first.EstablishPlace("unknown-game", settingsSignature())
	if err != nil {
		t.Fatalf("EstablishPlace: %v", err)
	}
	if id != before.ID {
		t.Fatalf("establishing an already-known place minted a SECOND subject (%q beside %q). "+
			"Two records for one screen is how recall becomes ambiguous and stops recognising "+
			"either of them", id, before.ID)
	}

	after := open(t, path).Recall("unknown-game", settingsSignature()).Subject
	if len(after.Knowledge) != len(before.Knowledge) {
		t.Fatalf("the interpretation list went from %d to %d entries",
			len(before.Knowledge), len(after.Knowledge))
	}
	k, ok := after.Find(observe.PossibleSettingsLikeState)
	if !ok || k.Status != observe.KnowledgeConfirmed {
		t.Fatalf("the user's confirmation is now %+v. Establishing a place must not be a "+
			"route by which observation edits what a person settled", k)
	}
	if k.Answered != 1 {
		t.Errorf("Answered = %d, want 1; establishing a place counted as somebody answering",
			k.Answered)
	}
	if after.Sessions != before.Sessions {
		t.Errorf("Sessions went %d → %d", before.Sessions, after.Sessions)
	}
}

// A place with nothing to recognise it by is refused, in the same words Remember uses.
//
// Not an oversight and not a nicety: a record nothing can ever match is one more entry per teach
// attempt, forever, buying nothing. Refusing is honest — Marco could not recognise this place, and
// storing the fact would not change that.
func TestAPlaceWithNoDiscriminatorIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	s := open(t, path)

	bare := observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 5}, Members: 5,
	}
	id, err := s.EstablishPlace("unknown-game", bare)
	if err == nil {
		t.Fatalf("a place with no discriminator was stored as %q", id)
	}
	if !strings.Contains(err.Error(), "discriminator") {
		t.Errorf("the refusal does not say why: %v", err)
	}
	if s.Count() != 0 {
		t.Errorf("%d subject(s) stored anyway", s.Count())
	}
}

// The bound still refuses, and establishing places does not get its own allowance.
func TestEstablishingAPlaceHonoursTheSubjectBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	s := open(t, path)

	for i := 0; i < semanticmemory.MaxSubjects; i++ {
		if _, err := s.EstablishPlace("unknown-game", distinctPlace(i)); err != nil {
			t.Fatalf("EstablishPlace %d: %v", i, err)
		}
	}
	if _, err := s.EstablishPlace("unknown-game",
		distinctPlace(semanticmemory.MaxSubjects)); err == nil {
		t.Fatalf("the store went past its %d-subject bound", semanticmemory.MaxSubjects)
	}
	if s.Count() != semanticmemory.MaxSubjects {
		t.Errorf("%d subjects held, want %d", s.Count(), semanticmemory.MaxSubjects)
	}
}

// An unreadable store refuses to establish anything, and says so.
//
// The same rule Remember follows: a corrupt file is preserved for recovery, never silently
// replaced — and least of all by a mechanism whose whole job is to add records.
func TestAnUnreadableStoreEstablishesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.json")
	if err := os.WriteFile(path, []byte("{not json at all"), 0o600); err != nil {
		t.Fatalf("planting the corrupt fixture: %v", err)
	}
	s, why := semanticmemory.Open(path)
	if why == "" {
		t.Fatal("the corrupt fixture opened cleanly")
	}
	if _, err := s.EstablishPlace("unknown-game", settingsSignature()); err == nil {
		t.Fatal("a place was established over an unreadable store")
	}
}

// distinctPlace is a signature nothing else matches, keyed on geometry.
//
// Geometry rather than terms, because the term vocabulary is closed and small: a fixture that
// needed five hundred distinct term sets could not have one.
func distinctPlace(i int) observe.StructureSignature {
	x := float64(i%64) / 128
	y := float64(i/64) / 128
	return observe.StructureSignature{
		Subject: observe.SubjectState,
		Roles:   map[string]int{"button": 4},
		Members: 4,
		Envelope: &observe.Region{
			X: x, Y: y, Width: 0.05, Height: 0.05,
		},
	}
}
