package semanticmemory_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// The candidate ledger: bounded, deterministic, and never knowledge.
//
// See [[ADR-095-repeated-observation-may-become-knowledge]].

func watchedStore(t *testing.T) *semanticmemory.Store {
	t.Helper()
	s, why := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	if why != "" {
		t.Fatalf("opening: %s", why)
	}
	return s
}

func candidate(id string, traversals int, at time.Time) observe.WatchedEdge {
	sig := observe.StructureSignature{
		Subject: observe.SubjectState, Members: 4, TermsKnown: true,
		Roles: map[string]int{"button": 4}, Terms: []observe.InterfaceTerm{observe.TermSettings},
	}
	return observe.WatchedEdge{
		ID: id, Application: "settings",
		From: observe.WatchedEnd{Shape: &sig, Called: "Home"},
		To:   observe.WatchedEnd{Shape: &sig, Called: "Bluetooth & devices"},
		Kind: "activate", Target: "Bluetooth & devices", Role: "button",
		Seen: traversals, First: at, Last: at,
	}
}

// REPEATED EVIDENCE IS ONE RECORD, NOT A PILE.
//
// The property everything ambient rests on, on the durable side: storage tracks how many DIFFERENT
// relationships somebody has, never how often they do them or how long Marco watched. A ledger
// that appended would grow with observation time, which is the exact shape of the thing ADR-093
// promised watching would never be.
//
// Deleting the replace must fail this.
func TestRepeatedEvidenceIsOneRecordNotAPile(t *testing.T) {
	s := watchedStore(t)
	at := time.Now()
	for i := 1; i <= 50; i++ {
		e := candidate("watched_one", i, at.Add(time.Duration(i)*time.Minute))
		if err := s.RememberWatched(e); err != nil {
			t.Fatalf("remembering: %v", err)
		}
	}
	held := s.Watched("settings")
	if len(held) != 1 {
		t.Fatalf("%d records after fifty sightings of one relationship, want 1", len(held))
	}
	if held[0].Seen != 50 {
		t.Errorf("the record says %d occasions, want 50: it is one record and the numbers "+
			"on it are what grow", held[0].Seen)
	}
}

// AND IT SURVIVES A RESTART.
//
// Candidate evidence is the first thing in the ambient path that is durable, and the reason is the
// product claim: a candidate that has not qualified yet must not have to be re-earned because a
// Director stopped, and the provenance of one that did must outlive the session that saw it.
func TestCandidateEvidenceSurvivesAReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	first, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("opening: %s", why)
	}
	if err := first.RememberWatched(candidate("watched_one", 1, time.Now())); err != nil {
		t.Fatalf("remembering: %v", err)
	}
	second, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("reopening: %s", why)
	}
	held := second.Watched("settings")
	if len(held) != 1 {
		t.Fatalf("%d records survived the reopen, want 1", len(held))
	}
	if held[0].From.Shape == nil || held[0].From.Called != "Home" {
		t.Errorf("what the screen was seen as did not survive: %+v", held[0].From)
	}
}

// EVICTION FORGETS THE WEAKEST CANDIDATE FIRST.
//
// A person meets thousands of controls over a month and almost every one of them is a thing they
// did once. The ledger is bounded — and a bound that forgot by insertion order would drop a
// candidate one sighting from promotion in favour of something somebody did once this morning.
//
// Deleting the weakest-first rule must fail this.
func TestEvictionForgetsTheWeakestCandidateFirst(t *testing.T) {
	s := watchedStore(t)
	at := time.Now()

	// A strong one, written first, so insertion order would evict it.
	strong := candidate("watched_strong", 9, at)
	if err := s.RememberWatched(strong); err != nil {
		t.Fatalf("remembering: %v", err)
	}
	// Then fill the ledger with things seen once.
	for i := 0; i < observe.MaxWatchedEdges+20; i++ {
		e := candidate("watched_weak_"+time.Duration(i).String(), 1,
			at.Add(time.Duration(i)*time.Minute))
		if err := s.RememberWatched(e); err != nil {
			t.Fatalf("remembering: %v", err)
		}
	}

	held := s.Watched("settings")
	if len(held) > observe.MaxWatchedEdges {
		t.Fatalf("the ledger grew to %d, past its bound of %d",
			len(held), observe.MaxWatchedEdges)
	}
	found := false
	for _, w := range held {
		if w.ID == "watched_strong" {
			found = true
		}
	}
	if !found {
		t.Error("the candidate closest to promotion was evicted in favour of things seen " +
			"once. Eviction that forgets the most work first is a bound that punishes use.")
	}
	// AND THE STRONGEST COMES BACK FIRST, so a caller meets the best evidence first and two
	// reads of an unchanged ledger agree.
	if held[0].ID != "watched_strong" {
		t.Errorf("the ledger leads with %q rather than its strongest candidate", held[0].ID)
	}
}

// A PROMOTED CANDIDATE IS NEVER EVICTED TO MAKE ROOM.
//
// It is the provenance of something Marco now knows. Losing it would leave durable knowledge
// unable to say where it came from — which is the one question anybody would ask about a thing
// Marco learned without being told to.
func TestAPromotedCandidateIsNotEvicted(t *testing.T) {
	s := watchedStore(t)
	at := time.Now()
	promoted := candidate("watched_promoted", 2, at)
	promoted.Promoted = at
	if err := s.RememberWatched(promoted); err != nil {
		t.Fatalf("remembering: %v", err)
	}
	for i := 0; i < observe.MaxWatchedEdges+20; i++ {
		e := candidate("watched_other_"+time.Duration(i).String(), 3,
			at.Add(time.Duration(i)*time.Minute))
		if err := s.RememberWatched(e); err != nil {
			t.Fatalf("remembering: %v", err)
		}
	}
	for _, w := range s.Watched("settings") {
		if w.ID == "watched_promoted" {
			return
		}
	}
	t.Error("the record of where durable knowledge came from was evicted")
}

// A CANDIDATE IS NOT KNOWLEDGE.
//
// Nothing plans over these. The interface that writes them cannot write a subject, a relationship,
// a goal or a judgement — and a ledger full of evidence leaves the topology exactly as empty as it
// was.
func TestCandidateEvidenceIsNotPlannableKnowledge(t *testing.T) {
	s := watchedStore(t)
	at := time.Now()
	for i := 0; i < 20; i++ {
		e := candidate("watched_"+time.Duration(i).String(), 9,
			at.Add(time.Duration(i)*time.Minute))
		if err := s.RememberWatched(e); err != nil {
			t.Fatalf("remembering: %v", err)
		}
	}
	top := s.Topology("settings")
	if len(top.Relationships) != 0 {
		t.Errorf("%d relationship(s) appeared from candidate evidence alone. Seeing "+
			"something is not knowing it, and only the admission boundary may change that.",
			len(top.Relationships))
	}
	if len(top.Subjects) != 0 {
		t.Errorf("%d subject(s) appeared from candidate evidence alone", len(top.Subjects))
	}
}

// A CANDIDATE WITH NO HANDLE IS REFUSED.
//
// The id is how a second sighting finds the first. Without one every sighting would be a new
// record, which is the append-forever failure this store exists to refuse.
func TestACandidateWithNoHandleIsRefused(t *testing.T) {
	s := watchedStore(t)
	e := candidate("", 1, time.Now())
	if err := s.RememberWatched(e); err == nil {
		t.Fatal("a candidate with no handle was accepted, so nothing could ever find it again")
	}
	if n := len(s.Watched("settings")); n != 0 {
		t.Errorf("%d record(s) were written for a refused candidate", n)
	}
}

// EVICTION MAKES ROOM RATHER THAN TURNING NEW EVIDENCE AWAY.
//
// # Why the previous test could not see this
//
// It wrote the strong candidate FIRST and then filled the ledger, so a store that simply refused
// everything once it was full kept the strong one too — and the mutation that removes eviction
// entirely survived. The order is the whole test: the candidate worth keeping arrives LAST, into a
// ledger that is already full of things seen once.
//
// A store that only refused would be bounded and useless: whatever somebody happened to do first
// would occupy it forever, and nothing they did afterwards could ever be learned.
//
// Deleting the eviction must fail this.
func TestEvictionMakesRoomRatherThanRefusingNewEvidence(t *testing.T) {
	s := watchedStore(t)
	at := time.Now()
	for i := 0; i < observe.MaxWatchedEdges; i++ {
		e := candidate("watched_weak_"+time.Duration(i).String(), 1,
			at.Add(time.Duration(i)*time.Minute))
		if err := s.RememberWatched(e); err != nil {
			t.Fatalf("remembering: %v", err)
		}
	}
	if n := len(s.Watched("settings")); n != observe.MaxWatchedEdges {
		t.Fatalf("the ledger holds %d, want it full at %d", n, observe.MaxWatchedEdges)
	}

	// And now the thing that matters, arriving into a full ledger.
	strong := candidate("watched_strong", 9, at.Add(time.Hour))
	if err := s.RememberWatched(strong); err != nil {
		t.Fatalf("remembering: %v", err)
	}
	for _, w := range s.Watched("settings") {
		if w.ID == "watched_strong" {
			return
		}
	}
	t.Error("a full ledger turned away the strongest evidence it had been offered. A store " +
		"that only refuses is bounded and useless: whatever somebody did first would " +
		"occupy it forever and nothing later could ever be learned.")
}

// AND A LEDGER FULL OF PROVENANCE REFUSES RATHER THAN FORGETTING IT.
//
// The one case where refusing IS right. Every record is the provenance of something Marco now
// knows, so there is nothing in the ledger it would be honest to drop — and durable knowledge that
// cannot say where it came from is the one question anybody would ask about a thing Marco learned
// without being told to.
//
// Deleting the promoted guard must fail this.
func TestALedgerFullOfProvenanceRefusesRatherThanForgetting(t *testing.T) {
	s := watchedStore(t)
	at := time.Now()
	for i := 0; i < observe.MaxWatchedEdges; i++ {
		e := candidate("watched_known_"+time.Duration(i).String(), 2,
			at.Add(time.Duration(i)*time.Minute))
		e.Promoted = at
		if err := s.RememberWatched(e); err != nil {
			t.Fatalf("remembering: %v", err)
		}
	}
	held := len(s.Watched("settings"))
	if held != observe.MaxWatchedEdges {
		t.Fatalf("the ledger holds %d, want it full at %d", held, observe.MaxWatchedEdges)
	}

	// A brand new candidate, stronger by every measure except the one that counts.
	fresh := candidate("watched_fresh", 99, at.Add(time.Hour))
	if err := s.RememberWatched(fresh); err != nil {
		t.Fatalf("remembering: %v", err)
	}
	for _, w := range s.Watched("settings") {
		if w.ID == "watched_fresh" {
			t.Fatal("a new candidate displaced the record of where durable knowledge came " +
				"from. Nothing in a ledger of pure provenance is honest to forget.")
		}
	}
	if n := len(s.Watched("settings")); n != held {
		t.Errorf("the ledger changed size: %d -> %d", held, n)
	}
}
