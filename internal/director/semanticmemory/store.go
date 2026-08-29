// Package semanticmemory is where what the user told Marco survives a restart.
//
// # Why this is not in observe
//
// `internal/director/observe` is the pure analysis core and has no os import. A package that
// could open a file could eventually write a screenshot into one, and the privacy guarantees of
// the observation path rest on that being structurally impossible rather than merely unintended.
// So the analysis core declares `observe.Memory` and this package implements it.
//
// # What lives here
//
// One JSON file, holding structure signatures and what is known about each. No screenshots, no
// raw OCR text, no key identities, no window generations, no process ids, no absolute
// coordinates, no session or proposal identifiers — see observe/remember.go for the full list and
// the recursive shape guard that holds it.
//
// # Corruption is reported, never swallowed
//
// A store that silently returns "empty" when its file is unreadable tells the user their answers
// were forgotten by pretending they were never given. This follows `demo.Store`, which learned
// the same lesson about learned procedures: the first a person hears of a silently dropped file
// is the Director doing something else.
//
// Unreadable memory therefore degrades to INERT-AND-VISIBLE. The Director keeps perceiving,
// keeps executing, keeps asking — it simply cannot remember, and it says so.
package semanticmemory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Version is the on-disk format version.
//
// An unknown version is REFUSED rather than reinterpreted. Guessing at a future format is how a
// migration silently corrupts the thing it was meant to preserve.
const Version = 1

// MaxSubjects bounds the store.
//
// Growth is per SUBJECT, not per observation: a screen seen ten thousand times is one record,
// because Remember matches an existing signature and updates it. This bound exists for the
// pathological case of an application that presents endless structurally distinct screens.
const MaxSubjects = 512

// MaxRelationships bounds the durable topology.
//
// Growth is per EDGE, not per observation: the same change seen ten thousand times is one
// record, because a session's evidence is folded into the edge it already matched. This bound
// exists for the pathological case of an application whose screens connect to everything.
//
// Deliberately larger than MaxSubjects rather than squared: a real interface is sparse, and a
// bound that allowed every subject to connect to every other would be no bound at all.
const MaxRelationships = 2048

// file is the on-disk shape.
type file struct {
	Version  int                         `json:"version"`
	Subjects []observe.RememberedSubject `json:"subjects"`
	// Relationships is the durable topology: which remembered subjects have been observed
	// becoming which others. Beside the subjects rather than in a second file, because an
	// edge is meaningless without its endpoints and two files could disagree about whether
	// an endpoint exists.
	Relationships []observe.RememberedRelationship `json:"relationships,omitempty"`
	// Candidates are the demonstrations the user approved and gave. Beside the topology for
	// the same reason the topology is beside the subjects: a candidate is meaningless without
	// the edge it demonstrates.
	Candidates []observe.ProcedureCandidate `json:"candidates,omitempty"`
	// Rehearsals is what whole attempts proved. Bounded evidence, nothing executable.
	Rehearsals []observe.RehearsalEvidence `json:"rehearsals,omitempty"`
	// Goals are the destinations people asked Marco to learn, in their own words. Beside
	// the subjects for the same reason everything here is: a goal names a subject, and a
	// second file could disagree about whether that subject exists.
	Goals []observe.Goal `json:"goals,omitempty"`
	// Watched is what ambient observation has SEEN repeatedly and Marco has not yet decided
	// to know. Candidate evidence, not knowledge.
	//
	// # Why it is in this file and not a second one
	//
	// Because it is written by the same process, under the same home, and it has to survive
	// the same restart — and because a second store would be a second durability
	// implementation to keep atomic, a second thing to bound, and a second place for a
	// MARCO_HOME to be got wrong. One file, one atomic rename, one answer.
	//
	// # And why it is nevertheless a separate FIELD, read through its own interface
	//
	// Because a candidate is emphatically not a relationship. Nothing plans over these, and
	// observe.WatchedStore — the only way in — cannot write a subject, a relationship, a goal
	// or a judgement. Being in one file makes them durable together; being separate types
	// behind separate interfaces is what keeps "I have seen this" from becoming "I know this"
	// by accident. See [[ADR-095-repeated-observation-may-become-knowledge]].
	Watched []observe.WatchedEdge `json:"watched,omitempty"`
}

// Store is durable semantic memory.
type Store struct {
	path string

	// learnedObserver is who to tell when a write COMMITS. Embedded so the notification
	// carries its own lock: an observer renders names, which means it reads this store, and
	// a shared mutex would deadlock the Director at the moment it learned something.
	learnedObserver

	mu            sync.RWMutex
	subjects      []observe.RememberedSubject
	relationships []observe.RememberedRelationship
	candidates    []observe.ProcedureCandidate
	// rehearsals is what whole attempts proved. Evidence, never a procedure: see
	// observe.RehearsalEvidence for what is deliberately absent.
	rehearsals []observe.RehearsalEvidence
	// goals are remembered destinations. A name and a subject — never a start, never a
	// route; see observe.Goal for why the type has nowhere to put either.
	goals []observe.Goal
	// watched is candidate evidence: what repeated observation has seen and Marco has not
	// yet decided to know. Never plannable; see observe.WatchedEdge.
	watched []observe.WatchedEdge
	// orphaned counts relationships dropped at load because an endpoint was gone.
	orphaned int
	// unavailable explains why memory cannot be used, empty when it can.
	//
	// Carried rather than returned-and-forgotten, so every report can say WHY nothing was
	// recognised. "Marco did not recognise this" and "Marco could not read its memory" are
	// different sentences and only one of them is about the screen.
	unavailable string
	// dropped counts records refused at the bound.
	dropped int
}

var (
	_ observe.Memory       = (*Store)(nil)
	_ observe.PlaceStore   = (*Store)(nil)
	_ observe.GoalStore    = (*Store)(nil)
	_ observe.ScreenAuthor = (*Store)(nil)
)

// Open reads the store, or returns an inert one that explains why it could not.
//
// Never returns nil. A Director whose memory failed to load must still run, so the failure is a
// property of the store rather than an error the caller has to decide what to do with.
func Open(path string) (*Store, string) {
	s := &Store{path: path}
	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		// No memory yet is not a failure. It is the first run.
		return s, ""
	case err != nil:
		s.unavailable = fmt.Sprintf("semantic memory at %s could not be read: %v", path, err)
		return s, s.unavailable
	}

	var f file
	if jerr := json.Unmarshal(raw, &f); jerr != nil {
		// CORRUPTION. The file is not discarded and not overwritten: a person may want to
		// recover it, and a store that repaired itself by deleting the evidence would make
		// "your answers are gone" indistinguishable from "you never answered".
		s.unavailable = fmt.Sprintf(
			"semantic memory at %s is not readable JSON (%v); nothing has been forgotten "+
				"and nothing will be written until it is moved or repaired", path, jerr)
		return s, s.unavailable
	}
	if f.Version != Version {
		s.unavailable = fmt.Sprintf(
			"semantic memory at %s is version %d and this Director understands version %d; "+
				"refusing to reinterpret it", path, f.Version, Version)
		return s, s.unavailable
	}
	s.subjects = f.Subjects
	// REFERENTIAL INTEGRITY, at load.
	//
	// An edge whose endpoint is gone is not repairable and not interpretable: it says two
	// subjects were connected and can no longer say which. Dropping it is deterministic and
	// the count is kept, so a store that lost half its topology says so rather than quietly
	// presenting the other half as the whole.
	//
	// This is also what defines what happens when a subject is removed or the file is edited
	// by hand: its edges go with it, at the next load, without a migration.
	known := map[string]bool{}
	for _, r := range f.Subjects {
		known[r.ID] = true
	}
	for _, rel := range f.Relationships {
		if !known[rel.From] || !known[rel.To] || rel.From == rel.To {
			s.orphaned++
			continue
		}
		s.relationships = append(s.relationships, rel)
	}
	for _, c := range f.Candidates {
		// Same referential rule: a demonstration of an edge whose endpoints are gone
		// describes nothing anybody can read.
		if !known[c.Relationship.From] || !known[c.Relationship.To] {
			s.orphaned++
			continue
		}
		s.candidates = append(s.candidates, c)
	}
	for _, e := range f.Rehearsals {
		// The same referential rule again. A rehearsal of a route whose endpoints are gone
		// proves nothing about anything Marco can currently find — and keeping it would let
		// a forgotten screen go on vouching for a procedure.
		if !known[e.Relationship.From] || !known[e.Relationship.To] {
			s.orphaned++
			continue
		}
		s.rehearsals = append(s.rehearsals, e)
	}
	for _, w := range f.Watched {
		// THE SAME REFERENTIAL RULE, and it applies to only HALF of a candidate.
		//
		// An endpoint Marco recognises is named by a subject id, and an id whose subject
		// is gone describes nothing — so that candidate goes, like every other record
		// here. But an endpoint Marco does NOT recognise carries a structure instead of an
		// id, and there is nothing for it to dangle from; those are the whole point of
		// candidate evidence and must survive.
		if (w.From.Recognised() && !known[w.From.Subject]) ||
			(w.To.Recognised() && !known[w.To.Subject]) {
			s.orphaned++
			continue
		}
		s.watched = append(s.watched, w)
	}
	for _, g := range f.Goals {
		// And once more for goals: a goal whose subject is gone names nothing anybody can
		// reach or plan toward.
		if !known[g.Subject] {
			s.orphaned++
			continue
		}
		s.goals = append(s.goals, g)
	}
	return s, ""
}

// Relationships returns a copy of the durable topology, for diagnostics.
func (s *Store) Relationships() []observe.RememberedRelationship {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]observe.RememberedRelationship{}, s.relationships...)
}

// Orphaned is how many relationships were dropped at load for a missing endpoint.
func (s *Store) Orphaned() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.orphaned
}

// Subject returns one remembered subject by id.
func (s *Store) Subject(id string) (observe.RememberedSubject, bool) {
	if s == nil {
		return observe.RememberedSubject{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.subjects {
		if r.ID == id {
			return r, true
		}
	}
	return observe.RememberedSubject{}, false
}

// RememberRelationships folds one session's observed transitions into the durable topology.
//
// # Why this is one call per session
//
// Because a session is when the evidence for an edge is complete. Writing per transition would
// mean writing per sample — the transition tally grows as the session runs — and the store's
// whole boundedness argument is that it is written on semantic EVENTS rather than on
// observations. A finished session is such an event, and it is also the unit `Sessions` counts.
//
// # What is checked here and what is not
//
// Checked: that both endpoints exist, in this application's namespace, and are not the same
// subject. Not checked: whether the endpoints are the RIGHT subjects — the caller resolved them
// through Recall, which is the identity layer, and re-deriving that here would be a second
// matcher with its own tolerances.
func (s *Store) RememberRelationships(application string,
	obs []observe.RelationshipObservation) (observe.RelationshipUpdate, error) {

	var update observe.RelationshipUpdate
	// Collected under the lock and announced after the save, so an edge nobody could write
	// is an edge nobody is told about.
	var pending []Learning
	if s == nil {
		return update, fmt.Errorf("semanticmemory: no store")
	}
	if len(obs) == 0 {
		return update, nil
	}
	s.mu.Lock()
	if s.unavailable != "" {
		reason := s.unavailable
		s.mu.Unlock()
		return update, fmt.Errorf("semanticmemory: not writing: %s", reason)
	}

	known := map[string]bool{}
	for _, r := range s.subjects {
		if strings.EqualFold(r.Application, application) {
			known[r.ID] = true
		}
	}

	for _, o := range obs {
		if o.Evidence.Empty() || o.From == "" || o.To == "" || o.From == o.To {
			update.Rejected++
			continue
		}
		// Both ends must be subjects this application actually remembers. An edge to a
		// subject that is not in the store is unreadable the moment it is written: nothing
		// can ever resolve the other end, and nothing can explain it to a person.
		if !known[o.From] || !known[o.To] {
			update.Rejected++
			continue
		}
		idx := -1
		for i, rel := range s.relationships {
			// DIRECTED. from/to are compared in order, never as a pair, because A→B and
			// B→A are different observations — a menu opened by `confirm` and closed by
			// `back` would otherwise become one edge that appears to answer to both.
			if rel.From == o.From && rel.To == o.To &&
				strings.EqualFold(rel.Application, application) {
				idx = i
				break
			}
		}
		if idx < 0 {
			if len(s.relationships) >= MaxRelationships {
				update.Rejected++
				continue
			}
			s.relationships = append(s.relationships, observe.RememberedRelationship{
				From: o.From, To: o.To, Application: application,
			})
			idx = len(s.relationships) - 1
			update.Created++
			pending = append(pending, Learning{
				Change: Learned, Kind: KindEdge, Application: application,
				From: o.From, To: o.To,
			})
		} else {
			update.Corroborated++
			pending = append(pending, Learning{
				Change: Strengthened, Kind: KindEdge, Application: application,
				From: o.From, To: o.To,
			})
		}
		s.relationships[idx].Fold(o.Evidence)
		// One increment per session per edge, which is what makes this a count of
		// independent corroborations rather than a second observation counter.
		//
		// A session that declares itself part of an EPISODE already counted folds its
		// evidence and claims no further corroboration. Learn is the only thing that
		// says so: it runs several bounded passes in one sitting, and three passes of one
		// learn attempt are not three independent sightings of a habit.
		if !o.SameEpisode {
			s.relationships[idx].Sessions++
		}
	}
	if update.Created == 0 && update.Corroborated == 0 {
		s.mu.Unlock()
		return update, nil
	}
	sort.Slice(s.relationships, func(i, j int) bool {
		if s.relationships[i].From != s.relationships[j].From {
			return s.relationships[i].From < s.relationships[j].From
		}
		return s.relationships[i].To < s.relationships[j].To
	})
	snapshot := s.snapshotLocked()
	s.mu.Unlock()

	if err := save(s.path, snapshot); err != nil {
		return update, err
	}
	s.announce(pending...)
	return update, nil
}

// Topology is the durable relationship graph for one application, with its endpoints.
//
// One read under one lock, so the edges and the subjects describe the same moment. A caller that
// fetched them separately could judge an edge against a subject that had been rewritten in
// between — rare, and the kind of rare that produces an inexplicable question.
func (s *Store) Topology(application string) observe.Topology {
	out := observe.Topology{Subjects: map[string]observe.RememberedSubject{}}
	if s == nil {
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.unavailable != "" {
		return out
	}
	for _, rel := range s.relationships {
		if strings.EqualFold(rel.Application, application) {
			out.Relationships = append(out.Relationships, rel)
		}
	}
	for _, r := range s.subjects {
		if strings.EqualFold(r.Application, application) {
			out.Subjects[r.ID] = r
		}
	}
	return out
}

// RememberLearning records what the user said about learning one relationship.
//
// # What this writes, and what it deliberately leaves alone
//
// It sets ONE field on the edge. It does not touch Observations, Sessions, Preceded,
// Unattributed, ConditionalOnly or Sequences — a person saying "don't learn that" has expressed
// a preference and has not claimed the transition never happened. Deleting the evidence on a
// refusal would be the most destructive possible reading of `no`, and it is the reading a
// straightforward implementation reaches for.
//
// It also creates nothing executable. A pending request is a note that the user is willing, and
// there is no registry it can be promoted into from here.
func (s *Store) RememberLearning(application string, ref observe.RelationshipRef,
	req observe.LearningRequest) error {

	if s == nil {
		return fmt.Errorf("semanticmemory: no store")
	}
	s.mu.Lock()
	if s.unavailable != "" {
		reason := s.unavailable
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: not writing: %s", reason)
	}
	idx := -1
	for i, rel := range s.relationships {
		if rel.From == ref.From && rel.To == ref.To &&
			strings.EqualFold(rel.Application, application) {
			idx = i
			break
		}
	}
	if idx < 0 {
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: no remembered relationship %s → %s in %s",
			ref.From, ref.To, application)
	}
	stored := req
	// The counts are taken from the record rather than from the caller, so the snapshot
	// describes what was actually true when the answer landed. A caller-supplied number would
	// be whatever was true when the question was ASKED, which may be minutes and several
	// transitions ago.
	stored.Observations = s.relationships[idx].Observations
	stored.Sessions = s.relationships[idx].Sessions
	s.relationships[idx].Learning = &stored
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return save(s.path, snapshot)
}

// snapshotLocked copies the whole store for a write. Caller holds the lock.
func (s *Store) snapshotLocked() file {
	return file{
		Version:       Version,
		Subjects:      append([]observe.RememberedSubject{}, s.subjects...),
		Relationships: append([]observe.RememberedRelationship{}, s.relationships...),
		Candidates:    append([]observe.ProcedureCandidate{}, s.candidates...),
		Rehearsals:    append([]observe.RehearsalEvidence{}, s.rehearsals...),
		Goals:         append([]observe.Goal{}, s.goals...),
		Watched:       append([]observe.WatchedEdge{}, s.watched...),
	}
}

// Unavailable explains why memory cannot be used, empty when it can.
func (s *Store) Unavailable() string {
	if s == nil {
		return "no semantic memory is wired"
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.unavailable
}

// Count is how many subjects are remembered.
func (s *Store) Count() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.subjects)
}

// Subjects returns a copy of what is remembered, for diagnostics.
func (s *Store) Subjects() []observe.RememberedSubject {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]observe.RememberedSubject{}, s.subjects...)
}

// Recall looks up what is known about evidence like this.
//
// Application-namespaced. Two applications that happen to present structurally identical screens
// are not assumed to mean the same thing by them — see the note on the namespace boundary in the
// subsystem documentation. That is the conservative choice and it is deliberate: recognising
// common structures ACROSS applications is a genuinely harder claim and belongs to a milestone
// that can measure whether it holds.
func (s *Store) Recall(application string, sig observe.StructureSignature) observe.Recollection {
	if s == nil {
		return observe.Recollection{Verdict: observe.MatchInsufficient}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.unavailable != "" {
		// Memory that could not be read is not memory that says "new".
		return observe.Recollection{Verdict: observe.MatchInsufficient}
	}

	scoped := make([]observe.RememberedSubject, 0, len(s.subjects))
	for _, r := range s.subjects {
		if strings.EqualFold(r.Application, application) {
			scoped = append(scoped, r)
		}
	}
	subject, verdict := observe.Recall(sig, scoped)
	return observe.Recollection{Verdict: verdict, Subject: subject}
}

// Remember records a settled interpretation, and writes.
//
// Called on semantic EVENTS — an answer, or evidence that has changed shape — never per sample.
// The write is atomic; see save.
func (s *Store) Remember(application string, sig observe.StructureSignature,
	k observe.SemanticKnowledge) error {

	if s == nil {
		return fmt.Errorf("semanticmemory: no store")
	}
	// A subject with no discriminator can never be matched again, so a record for it could
	// never be read — it would be one more entry per answer, forever, buying nothing.
	// Refusing is honest: Marco cannot recognise this subject, and storing the fact would
	// not change that.
	if !sig.Discriminating() {
		return fmt.Errorf("semanticmemory: this subject carries no discriminator " +
			"(no interface concepts were read and it has no envelope), so it could never " +
			"be recognised again; not stored")
	}
	s.mu.Lock()
	if s.unavailable != "" {
		reason := s.unavailable
		s.mu.Unlock()
		// Refusing to write over a file that could not be read is the whole point: a
		// corrupt store is preserved for recovery, not silently replaced.
		return fmt.Errorf("semanticmemory: not writing: %s", reason)
	}

	idx, err := s.subjectLocked(application, sig)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	s.subjects[idx].Knowledge = upsert(s.subjects[idx].Knowledge, k)
	// The structure is refreshed to the most recent evidence, so a subject whose detector
	// output drifts slightly over months stays matchable rather than aging out.
	s.subjects[idx].Structure = sig
	snapshot := s.snapshotLocked()
	s.mu.Unlock()

	return save(s.path, snapshot)
}

// subjectLocked finds the subject this signature describes, minting one if there is none.
//
// THE canonical subject path, and the reason it is one function: `Remember` and `EstablishPlace`
// must agree exactly about what counts as the same subject, what a new record looks like and when
// the bound refuses one. Two copies of this would eventually disagree, and the disagreement would
// look like a screen Marco recognised on Tuesday and not on Wednesday. Caller holds the lock.
func (s *Store) subjectLocked(application string, sig observe.StructureSignature) (int, error) {
	// Match an EXISTING subject rather than appending. This is what makes storage bounded
	// by subjects instead of by observations: the same screen recognised on a hundred days
	// updates one record.
	for i, r := range s.subjects {
		if !strings.EqualFold(r.Application, application) {
			continue
		}
		if observe.CompareStructure(sig, r.Structure) == observe.MatchSame {
			return i, nil
		}
	}
	if len(s.subjects) >= MaxSubjects {
		s.dropped++
		return -1, fmt.Errorf("semanticmemory: at the %d-subject bound; not stored", MaxSubjects)
	}
	s.subjects = append(s.subjects, observe.RememberedSubject{
		ID:          subjectID(application, sig),
		Application: application, Structure: sig, Sessions: 1,
	})
	return len(s.subjects) - 1, nil
}

// EstablishPlace persists a place's IDENTITY, and asserts nothing about what it means.
//
// # What separates this from Remember
//
// One line: it writes no SemanticKnowledge. The subject, its structure, its bound and its id come
// from the same canonical path a confirmed answer uses — so a place established this way is
// recognised by exactly the same matcher, across a restart, with no second identity mechanism
// anywhere. What it does not do is claim the user said anything, because they did not.
//
// A subject the store already holds is returned untouched. That is deliberate rather than
// convenient: this must never be a route by which repeated observation edits, refreshes or erases
// what a person once settled. See [[ADR-021-a-judgement-is-recomputed-not-recorded]] for why the
// reverse direction is equally forbidden.
//
// Refuses a signature with no discriminator for the same reason Remember does: a record nothing
// can ever match is unbounded growth in exchange for nothing.
func (s *Store) EstablishPlace(application string,
	sig observe.StructureSignature) (string, error) {

	if s == nil {
		return "", fmt.Errorf("semanticmemory: no store")
	}
	if !sig.Discriminating() {
		return "", fmt.Errorf("semanticmemory: this place carries no discriminator " +
			"(no interface concepts were read and it has no envelope), so it could never " +
			"be recognised again; not stored")
	}
	s.mu.Lock()
	if s.unavailable != "" {
		reason := s.unavailable
		s.mu.Unlock()
		return "", fmt.Errorf("semanticmemory: not writing: %s", reason)
	}
	// Held BEFORE the write, so an existing subject costs no disk at all. A learn attempt runs
	// several passes in a sitting and most of them are standing somewhere already known.
	for _, r := range s.subjects {
		if strings.EqualFold(r.Application, application) &&
			observe.CompareStructure(sig, r.Structure) == observe.MatchSame {
			id := r.ID
			s.mu.Unlock()
			return id, nil
		}
	}
	idx, err := s.subjectLocked(application, sig)
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	id := s.subjects[idx].ID
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	// AFTER THE WRITE, AND ONLY IF IT WORKED. A Place that could not be saved is a Place
	// Marco has not learned, whatever the in-memory slice now says. The early return above
	// announces nothing on purpose: coming back with an existing id is recognition, not
	// learning, and a feed that said otherwise would fire every time somebody walked a route
	// they had already taught.
	if err := save(s.path, snapshot); err != nil {
		return id, err
	}
	s.announce(Learning{
		Change: Learned, Kind: KindPlace, Application: application, Subject: id,
	})
	return id, nil
}

// NoteSession records that a subject was seen again, without changing what is known.
//
// Bounded by construction: it updates a counter on an existing record and never creates one, so
// merely observing something familiar can never grow the store.
func (s *Store) NoteSession(application string, sig observe.StructureSignature) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unavailable != "" {
		return
	}
	for i, r := range s.subjects {
		if !strings.EqualFold(r.Application, application) {
			continue
		}
		if observe.CompareStructure(sig, r.Structure) == observe.MatchSame {
			s.subjects[i].Sessions++
			return
		}
	}
}

// upsert replaces the knowledge for a kind, or adds it.
func upsert(in []observe.SemanticKnowledge, k observe.SemanticKnowledge) []observe.SemanticKnowledge {
	for i := range in {
		if in[i].Kind != k.Kind {
			continue
		}
		// Answers accumulate rather than reset: a subject confirmed on three separate days
		// is a stronger record than one confirmed once, and the count is the only place
		// that shows.
		k.Answered += in[i].Answered
		in[i] = k
		return in
	}
	out := append(in, k)
	sort.Slice(out, func(a, b int) bool { return out[a].Kind < out[b].Kind })
	return out
}

// save writes the store atomically.
//
// Temp file then rename, the same pattern actiongraph.Save uses. A partial write that replaced
// the real file would lose every answer the user has ever given, and it would do so at exactly
// the moment they gave one more.
func save(path string, f file) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("semanticmemory: encoding: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("semanticmemory: preparing %s: %w", filepath.Dir(path), err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("semanticmemory: writing: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("semanticmemory: replacing %s: %w", path, err)
	}
	return nil
}

// subjectID is a stable handle for diagnostics.
//
// Content-derived and NOT the identity test — CompareStructure is. Two records whose ids differ
// may still be the same subject under the matcher's tolerances, which is exactly why the id is
// never consulted when matching.
func subjectID(application string, sig observe.StructureSignature) string {
	h := sha256.New()
	h.Write([]byte(strings.ToLower(application)))
	h.Write([]byte{0})
	h.Write([]byte(sig.Subject))
	roles := make([]string, 0, len(sig.Roles))
	for r, n := range sig.Roles {
		roles = append(roles, fmt.Sprintf("%s:%d", r, n))
	}
	sort.Strings(roles)
	h.Write([]byte(strings.Join(roles, ",")))
	terms := make([]string, 0, len(sig.Terms))
	for _, t := range sig.Terms {
		terms = append(terms, string(t))
	}
	sort.Strings(terms)
	h.Write([]byte(strings.Join(terms, ",")))
	// A TARGET IS IDENTIFIED BY WHAT IT IS CALLED, IN A PLACE — so that has to be in here.
	//
	// # The collision, measured
	//
	// A live store held FIVE subject records under one id, `subj_c7a9438815d7`, describing
	// four different things:
	//
	//	"Bluetooth & devices" in subj_bef5e3d29af8 (Home)
	//	"Mouse"               in subj_543793ccc326
	//	"Bluetooth & devices" in subj_235d87abdfc2
	//	"Mouse"               in subj_892a4cc30f41
	//
	// A target is not a composition: it carries no roles and no terms, because a screen is
	// recognised by what it is MADE OF and a target by what it is CALLED. Everything hashed
	// above is therefore empty for every target, so every target in one application hashed
	// to `application + "target"` and collapsed onto a single id — and each write appended
	// another record under it.
	//
	// It is not duplicate persistence and no caller disagreed about ownership. The id simply
	// was not a function of the identity, so `Mouse` and `Bluetooth & devices` were the same
	// subject as far as the store could tell.
	//
	// Deleting these three writes must fail TestTwoDifferentTargetsAreNotTheSameSubject.
	if sig.Subject == observe.SubjectTarget {
		h.Write([]byte{0})
		h.Write([]byte(strings.ToLower(sig.Label)))
		h.Write([]byte{0})
		h.Write([]byte(sig.Kind))
		h.Write([]byte{0})
		h.Write([]byte(sig.Place))
	}
	return "subj_" + hex.EncodeToString(h.Sum(nil))[:12]
}

// MaxCandidates bounds the demonstrations one store keeps.
//
// One per relationship — a second demonstration of the same edge replaces the first, because the
// question this evidence answers is "what does a clean example look like", not "how many times
// has the user shown me". This bound exists for the pathological case of an application with an
// enormous approved topology.
const MaxCandidates = 256

// RememberCandidate keeps one completed demonstration.
//
// # What this is allowed to be, and what it is not
//
// It is evidence: a person approved one demonstration, gave it, and this is what was observed.
// It is not a procedure and it is not registered anywhere that could run one — the type has no
// Execute, lives in the analysis core, and reaches this store through an interface whose only
// other method is a read.
//
// Refused when the relationship is not held, for the same reason a relationship is refused when
// its endpoints are not: a demonstration of an edge nothing remembers is unreadable the moment
// it is written.
func (s *Store) RememberCandidate(application string, c observe.ProcedureCandidate) error {
	if s == nil {
		return fmt.Errorf("semanticmemory: no store")
	}
	s.mu.Lock()
	if s.unavailable != "" {
		reason := s.unavailable
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: not writing: %s", reason)
	}
	held := false
	for _, rel := range s.relationships {
		if rel.From == c.Relationship.From && rel.To == c.Relationship.To &&
			strings.EqualFold(rel.Application, application) {
			held = true
			break
		}
	}
	if !held {
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: no remembered relationship %s → %s in %s",
			c.Relationship.From, c.Relationship.To, application)
	}
	c.Application = application
	// One per relationship PER SEQUENCE. A follow-up is a second observation, not a
	// correction of the first: candidate 1 stays exactly as it was captured, because a
	// comparison between two examples is worthless if one of them has been edited.
	if c.Sequence == 0 {
		c.Sequence = 1
	}
	replaced := false
	for i, existing := range s.candidates {
		if existing.Relationship == c.Relationship && existing.Sequence == c.Sequence &&
			strings.EqualFold(existing.Application, application) {
			s.candidates[i] = c
			replaced = true
			break
		}
	}
	if !replaced {
		if len(s.candidates) >= MaxCandidates {
			s.mu.Unlock()
			return fmt.Errorf("semanticmemory: at the %d-candidate bound; not stored",
				MaxCandidates)
		}
		s.candidates = append(s.candidates, c)
	}
	sort.Slice(s.candidates, func(i, j int) bool {
		if s.candidates[i].Relationship.From != s.candidates[j].Relationship.From {
			return s.candidates[i].Relationship.From < s.candidates[j].Relationship.From
		}
		if s.candidates[i].Relationship.To != s.candidates[j].Relationship.To {
			return s.candidates[i].Relationship.To < s.candidates[j].Relationship.To
		}
		return s.candidates[i].Sequence < s.candidates[j].Sequence
	})
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return save(s.path, snapshot)
}

// MaxGoals bounds how many learned goals one store holds.
//
// Goals are one per outcome a person asked Marco to learn, so the realistic count is tens;
// the bound exists so a runaway caller cannot grow the file without limit.
const MaxGoals = 256

// RememberGoal records one goal, or folds a repeat demonstration into an existing one.
//
// # The rules, and why each
//
//   - The subject must be held. A goal naming a place memory cannot find is a promise about
//     nothing, and would be dropped at the next load anyway.
//   - One name, one outcome, per application — the invariant `reach` depends on, and it is
//     kept by REBINDING rather than by refusing. A person asking for the same name again is
//     not creating a second meaning; they are saying what they mean by it now, usually
//     because the last attempt did not work. That is the same rule `NameSubject` follows
//     for screens: "renaming the same screen is ordinary, and drops the old name".
//     Refusing was tried first and it was wrong in the only way that matters — measured
//     live on 2026-08-17, a goal left behind by a FAILED learn made the name unusable, so
//     the person was punished for Marco's earlier failure and the name could not be reused at all.
//   - The same name for the same subject is a repeat demonstration and folds — lineage,
//     never a second record.
//
// What is deliberately NOT here: a start, a route, a step list, or anything the first
// demonstration could bake into the goal's identity. The type has no field to put one in.
func (s *Store) RememberGoal(application string, g observe.Goal) error {
	if s == nil {
		return fmt.Errorf("semanticmemory: no store")
	}
	name := strings.TrimSpace(g.Name)
	if name == "" || g.Subject == "" {
		return fmt.Errorf("semanticmemory: a goal is a name and a subject; refusing %q → %q",
			g.Name, g.Subject)
	}
	s.mu.Lock()
	if s.unavailable != "" {
		reason := s.unavailable
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: not writing: %s", reason)
	}
	held := false
	for _, r := range s.subjects {
		if r.ID == g.Subject && strings.EqualFold(r.Application, application) {
			held = true
			break
		}
	}
	if !held {
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: no remembered subject %s in %s", g.Subject,
			application)
	}
	folded := false
	change := Learned
	for i, have := range s.goals {
		if !strings.EqualFold(have.Application, application) ||
			!strings.EqualFold(have.Name, name) {
			continue
		}
		if have.Subject != g.Subject {
			// REBOUND. The lineage of the old binding does not carry over — it was about
			// reaching somewhere else — so the count starts again at this demonstration.
			s.goals[i].Subject, s.goals[i].Demonstrations = g.Subject, 1
			folded, change = true, Rebound
			break
		}
		s.goals[i].Demonstrations++
		folded, change = true, Strengthened
		break
	}
	if !folded {
		if len(s.goals) >= MaxGoals {
			s.mu.Unlock()
			return fmt.Errorf("semanticmemory: at the %d-goal bound; not stored", MaxGoals)
		}
		s.goals = append(s.goals, observe.Goal{
			Name: name, Application: application, Subject: g.Subject, Demonstrations: 1,
		})
		sort.Slice(s.goals, func(i, j int) bool {
			if s.goals[i].Application != s.goals[j].Application {
				return s.goals[i].Application < s.goals[j].Application
			}
			return s.goals[i].Name < s.goals[j].Name
		})
	}
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	if err := save(s.path, snapshot); err != nil {
		return err
	}
	s.announce(Learning{
		Change: change, Kind: KindGoal, Application: application,
		Subject: g.Subject, Name: name,
	})
	return nil
}

// Goals returns the learned goals for one application, in a stable order.
func (s *Store) Goals(application string) []observe.Goal {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []observe.Goal
	for _, g := range s.goals {
		if strings.EqualFold(g.Application, application) {
			out = append(out, g)
		}
	}
	return out
}

// Candidates returns the demonstrations recorded for one application.
func (s *Store) Candidates(application string) []observe.ProcedureCandidate {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []observe.ProcedureCandidate
	for _, c := range s.candidates {
		if strings.EqualFold(c.Application, application) {
			out = append(out, c)
		}
	}
	return out
}

// RememberFollowUp records what the user said when asked for a SECOND demonstration.
//
// A separate field from the first request, deliberately. The first one was agreed to and
// fulfilled, and that is part of the lineage; overwriting it would make "asked once" and "asked
// twice" the same record, and a later session could not tell whether it had already asked.
//
// Touches nothing else. Neither candidate, neither endpoint, and none of the relationship's
// evidence — a person declining to demonstrate something again has said nothing about whether
// the transition happens.
func (s *Store) RememberFollowUp(application string, ref observe.RelationshipRef,
	req observe.LearningRequest) error {

	if s == nil {
		return fmt.Errorf("semanticmemory: no store")
	}
	s.mu.Lock()
	if s.unavailable != "" {
		reason := s.unavailable
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: not writing: %s", reason)
	}
	idx := -1
	for i, rel := range s.relationships {
		if rel.From == ref.From && rel.To == ref.To &&
			strings.EqualFold(rel.Application, application) {
			idx = i
			break
		}
	}
	if idx < 0 {
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: no remembered relationship %s → %s in %s",
			ref.From, ref.To, application)
	}
	stored := req
	s.relationships[idx].FollowUp = &stored
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return save(s.path, snapshot)
}

// FulfilLearning marks a demonstration request as satisfied.
//
// THE thing that stops the collection loop. A pending request arms a capture, so a request that
// stayed pending after its demonstration completed would make every later session watch the same
// route again — forever, without asking, which is exactly the permission the user did not give.
//
// Which request is fulfilled depends on the lineage: sequence 1 settles the original, sequence 2
// settles the follow-up.
func (s *Store) FulfilLearning(application string, ref observe.RelationshipRef,
	sequence int) error {

	if s == nil {
		return fmt.Errorf("semanticmemory: no store")
	}
	s.mu.Lock()
	if s.unavailable != "" {
		reason := s.unavailable
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: not writing: %s", reason)
	}
	idx := -1
	for i, rel := range s.relationships {
		if rel.From == ref.From && rel.To == ref.To &&
			strings.EqualFold(rel.Application, application) {
			idx = i
			break
		}
	}
	if idx < 0 {
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: no remembered relationship %s → %s in %s",
			ref.From, ref.To, application)
	}
	target := s.relationships[idx].Learning
	if sequence >= 2 {
		target = s.relationships[idx].FollowUp
	}
	if target == nil || target.Status != observe.LearningPending {
		s.mu.Unlock()
		return nil
	}
	fulfilled := *target
	fulfilled.Status = observe.LearningFulfilled
	if sequence >= 2 {
		s.relationships[idx].FollowUp = &fulfilled
	} else {
		s.relationships[idx].Learning = &fulfilled
	}
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return save(s.path, snapshot)
}

// MaxRehearsals bounds the rehearsal evidence one store keeps.
//
// One per route per demonstration, replaced in place, so this is a ceiling on distinct routes
// rather than on attempts. A store that grew by one record per rehearsal would turn a feature
// somebody uses often into a file that never stops getting bigger.
const MaxRehearsals = 128

// RememberRehearsal keeps what one whole attempt proved about one route.
//
// Bounded semantic evidence and nothing executable: which candidate, where it ran, which meanings
// were sent, and how each step came out. Reproducing any of it means lowering the candidate again
// through the boundary under a fresh authorization — see [[ADR-026-verification-is-derived-from-a-completed-rehearsal]].
//
// One record per (route, sequence): a later attempt REPLACES an earlier one, because the question
// a reader asks is "does this route work now", not "how many times has it been tried".
func (s *Store) RememberRehearsal(application string, e observe.RehearsalEvidence) error {
	if s == nil {
		return fmt.Errorf("semanticmemory: no store")
	}
	if e.Relationship.From == "" || e.Relationship.To == "" {
		return fmt.Errorf("semanticmemory: rehearsal evidence must name a route")
	}
	s.mu.Lock()
	if s.unavailable != "" {
		reason := s.unavailable
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: not writing: %s", reason)
	}
	e.Application = application
	replaced := false
	for i, existing := range s.rehearsals {
		if existing.Relationship == e.Relationship && existing.Sequence == e.Sequence &&
			strings.EqualFold(existing.Application, application) {
			s.rehearsals[i] = e
			replaced = true
			break
		}
	}
	if !replaced {
		if len(s.rehearsals) >= MaxRehearsals {
			s.mu.Unlock()
			return fmt.Errorf("semanticmemory: at the %d-rehearsal bound; not stored",
				MaxRehearsals)
		}
		s.rehearsals = append(s.rehearsals, e)
	}
	sort.Slice(s.rehearsals, func(i, j int) bool {
		if s.rehearsals[i].Relationship.From != s.rehearsals[j].Relationship.From {
			return s.rehearsals[i].Relationship.From < s.rehearsals[j].Relationship.From
		}
		if s.rehearsals[i].Relationship.To != s.rehearsals[j].Relationship.To {
			return s.rehearsals[i].Relationship.To < s.rehearsals[j].Relationship.To
		}
		return s.rehearsals[i].Sequence < s.rehearsals[j].Sequence
	})
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return save(s.path, snapshot)
}

// Rehearsals returns the rehearsal evidence stored for one application.
func (s *Store) Rehearsals(application string) []observe.RehearsalEvidence {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []observe.RehearsalEvidence
	for _, e := range s.rehearsals {
		if strings.EqualFold(e.Application, application) {
			out = append(out, e)
		}
	}
	return out
}

// NameSubject records what the user calls one remembered screen.
//
// # Why this refuses a duplicate rather than resolving it
//
// Because a name is how a PLAY refers to a place, and two places answering to one name means a
// play whose first line is ambiguous. First-match-wins would make which one it meant depend on
// file order, which is not something a person could predict or debug.
//
// Renaming the same subject is fine and is the ordinary case. Giving a second subject a name that
// is already taken is refused, and the caller is told which one has it.
//
// The name arrives as an `observe.ScreenName`, which has one constructor and no other way in — so
// observed text cannot become a screen name by being assigned to the right variable.
func (s *Store) NameSubject(application, id string, called observe.ScreenName) error {
	name := called.String()
	if s == nil {
		return fmt.Errorf("semanticmemory: no store")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("semanticmemory: a screen needs a name")
	}
	s.mu.Lock()
	if s.unavailable != "" {
		reason := s.unavailable
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: not writing: %s", reason)
	}
	found := -1
	for i, r := range s.subjects {
		if !strings.EqualFold(r.Application, application) {
			continue
		}
		if r.ID == id {
			found = i
			continue
		}
		if strings.EqualFold(r.Called, name) {
			s.mu.Unlock()
			return fmt.Errorf(
				"semanticmemory: another screen in %s is already called %q", application, name)
		}
	}
	if found < 0 {
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: no remembered screen %s in %s", id, application)
	}
	s.subjects[found].Called = name
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return save(s.path, snapshot)
}

// Path is the file this store reads and writes.
//
// A read, for callers that need to prove something reached disk rather than a map. It hands back
// a location, never contents.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// SubjectNamed resolves a user-given screen name to the durable subject it refers to.
//
// # Exact, scoped, and fail-closed
//
// One application, one exact name (case-insensitively), one subject. No substring, no nearest
// match, no fuzzy score, and no cross-application fallback: `settings` in one program is not
// `settings` in another, and a play that asked for the wrong one would press keys in the wrong
// place.
//
// A duplicate that got into the file some other way is AMBIGUOUS, not first-match-wins. Legacy or
// corrupt data must not silently decide what a play meant.
func (s *Store) SubjectNamed(application, name string) (observe.RememberedSubject, bool) {
	if s == nil {
		return observe.RememberedSubject{}, false
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return observe.RememberedSubject{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var found observe.RememberedSubject
	n := 0
	// BY WHATEVER IT IS CALLED — the Audience.s word or Marco.s, through the one accessor.
	//
	// A play says `do Screen.s Showing with "Home"`, and that sentence has to find the screen
	// whether the word came from a person or from grounded inference. Matching only `Called`
	// would let a play be written down naming a screen and then fail to resolve it at run
	// time — worse than refusing to write it, because it fails at a keyboard.
	//
	// Still exactly one match or none: two screens answering to one word is ambiguous, and
	// ambiguity here would pick a screen for somebody.
	//
	// Deleting the Name() call must fail TestAPlayResolvesTheNameMarcoWorkedOut.
	for _, r := range s.subjects {
		if strings.EqualFold(r.Application, application) && strings.EqualFold(r.Name(), name) {
			found, n = r, n+1
		}
	}
	if n != 1 {
		return observe.RememberedSubject{}, false
	}
	return found, true
}

// ── the Repertoire: durable semantic targets ──────────────────────────────────
//
// Theater's durable half, implemented by the store that already holds places and routes. There is
// no second target database and no second matcher: a target is a subject, matched by the one
// CompareStructure, written by the one match-or-append, bounded by the one MaxSubjects.
//
// See [[ADR-068-the-theater-is-the-durable-semantic-world]].

// RememberTarget makes a semantic target durable, asserting nothing about what it means.
//
// The same discipline EstablishPlace follows, for the same reasons: idempotent, no judgement
// written, an existing record returned untouched, and a signature that could never be matched
// again refused outright rather than stored as clutter nothing can resolve.
//
// `learned` is PROVENANCE. It records which way of knowing produced the evidence and is read by
// nothing that decides how to act — see EvidenceSource.Authoritative for the one thing it governs.
func (s *Store) RememberTarget(application string, sig observe.StructureSignature,
	learned observe.EvidenceSource) (string, error) {

	if s == nil {
		return "", fmt.Errorf("semanticmemory: no store")
	}
	if sig.Subject != observe.SubjectTarget {
		return "", fmt.Errorf("semanticmemory: %q is not a target", sig.Subject)
	}
	if !sig.Discriminating() {
		return "", fmt.Errorf("semanticmemory: a target with no name and no place could " +
			"never be found again; not stored")
	}
	s.mu.Lock()
	if s.unavailable != "" {
		reason := s.unavailable
		s.mu.Unlock()
		return "", fmt.Errorf("semanticmemory: not writing: %s", reason)
	}
	// Already known costs no disk, and leaves everything the Audience has said about it —
	// its name, its corrections — exactly as it was.
	for _, r := range s.subjects {
		if strings.EqualFold(r.Application, application) &&
			observe.CompareStructure(sig, r.Structure) == observe.MatchSame {
			id := r.ID
			s.mu.Unlock()
			return id, nil
		}
	}
	idx, err := s.subjectLocked(application, sig)
	if err != nil {
		s.mu.Unlock()
		return "", err
	}
	s.subjects[idx].Learned = learned
	id := s.subjects[idx].ID
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return id, save(s.path, snapshot)
}

// ResolveTarget is which durable target this description is, if Marco knows one.
//
// The canonical Recall, so a target is recognised by the same machinery a place is. A caller gets
// a verdict rather than a boolean, because "several fit equally" is a different answer from "I
// have never seen this" and they call for different responses.
func (s *Store) ResolveTarget(application string,
	sig observe.StructureSignature) observe.Recollection {

	return s.Recall(application, sig)
}

// TargetsIn is every target known to be grounded in a place.
func (s *Store) TargetsIn(application, place string) []observe.RememberedSubject {
	return observe.TargetsGroundedIn(s.Subjects(), application, place)
}

// UnnameSubject takes back what the Audience called a place.
//
// # What survives, and why that is the whole point
//
// Everything except the word. The subject keeps its id, its structure, its judgements and every
// route, goal and target that refers to it — because a person changing their mind about a NAME
// has said nothing about the place. Deleting the subject would orphan the routes; minting a new
// one would do the same by a longer path.
//
// And the name is RELEASED. Uniqueness is enforced against what places are called NOW, so the
// word is immediately available to whichever place the person actually meant. Reserving every
// string ever used would mean a single mistyped answer permanently burned that word.
//
// Idempotent: a place nobody has named is already unnamed, and saying so again is not an error.
func (s *Store) UnnameSubject(application, id string) error {
	if s == nil {
		return fmt.Errorf("semanticmemory: no store")
	}
	s.mu.Lock()
	if s.unavailable != "" {
		reason := s.unavailable
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: not writing: %s", reason)
	}
	found := -1
	for i, r := range s.subjects {
		if strings.EqualFold(r.Application, application) && r.ID == id {
			found = i
			break
		}
	}
	if found < 0 {
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: no remembered place %s in %s", id, application)
	}
	if s.subjects[found].Called == "" {
		s.mu.Unlock()
		return nil
	}
	s.subjects[found].Called = ""
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return save(s.path, snapshot)
}

// ObserveSemanticName records what a Place APPEARS to be called, from an Actor's evidence.
//
// # Why this is not NameSubject
//
// `NameSubject` writes `Called`, which means "a person offered this word". This writes `Semantic`,
// which means "Marco worked this out", and the two must never be the same field — see
// [[ADR-076-a-place-may-say-what-it-appears-to-be-called]]. A reader deciding whether a play may
// say a name out loud has to be able to tell them apart, and it can only do that while exactly one
// of them is on the human side of the boundary.
//
// Consequences of the difference, all deliberate:
//
//   - no uniqueness check. Two Places may well appear to be called the same thing, and refusing
//     the second would be forcing uniqueness on an observation rather than on an authored name.
//   - idempotent. The same word again is not a change and does not rewrite the file.
//   - it never touches `Called`, so an Audience name is untouched by anything Marco later infers.
//
// Deleting the `Called` exclusion must fail TestAnInferredNameNeverOverwritesTheAudiences.
func (s *Store) ObserveSemanticName(application, id, name string, from observe.EvidenceSource) error {
	if s == nil {
		return fmt.Errorf("semanticmemory: no store")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	s.mu.Lock()
	if s.unavailable != "" {
		reason := s.unavailable
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: not writing: %s", reason)
	}
	found := -1
	for i, r := range s.subjects {
		if strings.EqualFold(r.Application, application) && r.ID == id {
			found = i
			break
		}
	}
	if found < 0 {
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: no remembered screen %s in %s", id, application)
	}
	if s.subjects[found].Semantic == name && s.subjects[found].SemanticFrom == from {
		s.mu.Unlock()
		return nil
	}
	s.subjects[found].Semantic, s.subjects[found].SemanticFrom = name, from
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	if err := save(s.path, snapshot); err != nil {
		return err
	}
	// A Place Marco could already recognise can now be spoken about. Worth telling somebody,
	// and NOT a new Place: the idempotent return above means the same word again says nothing.
	s.announce(Learning{
		Change: Named, Kind: KindPlace, Application: application, Subject: id, Name: name,
	})
	return nil
}
