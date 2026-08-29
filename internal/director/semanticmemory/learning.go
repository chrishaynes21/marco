package semanticmemory

import "sync"

// WHAT MARCO ACTUALLY LEARNED, SAID OUT LOUD.
//
// # Why this lives on the store and nowhere else
//
// Because the one thing a learning feed must never do is claim something Marco did not learn. A
// feed built anywhere upstream — in the observer, in promotion, in the session — is reporting an
// INTENTION: it says "this is about to be written" and is right only until a bound is hit, a
// signature turns out to match an existing record, the file cannot be written, or the store is
// unavailable. Every one of those is an ordinary outcome here.
//
// So the announcement is made by the code that did the writing, after the write succeeded. If
// `save` returns an error nothing is announced. If a Place turns out to be one the store already
// held, nothing is announced — `EstablishPlace` is idempotent by signature and returning an
// existing id is not learning.
//
// # It is a notification, not a log
//
// Nothing here is persisted, counted, or read back. It exists so a person watching can be told
// what changed; the canonical record is the store itself, and a consumer that wants the truth
// reads that.

// LearningChange is what happened to a piece of durable knowledge.
type LearningChange string

const (
	// Learned: it did not exist and now does.
	Learned LearningChange = "learned"
	// Strengthened: it existed and gained independent evidence. A separate word on purpose —
	// a feed that said "learned" every time somebody walked a familiar route would train a
	// person to stop reading it.
	Strengthened LearningChange = "strengthened"
	// Named: something Marco could already recognise can now be spoken about.
	Named LearningChange = "named"
	// Rebound: a name that pointed at one place now points at another.
	Rebound LearningChange = "rebound"
)

// LearningKind is what sort of knowledge changed.
type LearningKind string

const (
	KindPlace LearningKind = "place"
	KindEdge  LearningKind = "edge"
	KindGoal  LearningKind = "goal"
)

// Learning is one committed change, in ids rather than words.
//
// Deliberately unrendered. A subject's name is a fact about the store NOW, and it can arrive
// after the Place does — the same Place is established on one pass and named on a later one. A
// value that carried the name it had at the instant of the write would show `[unnamed]` forever
// for a Place that is now perfectly well named. So the consumer resolves names when it renders.
type Learning struct {
	Change      LearningChange
	Kind        LearningKind
	Application string
	// Subject is the place this is about, for KindPlace and KindGoal.
	Subject string
	// From and To are the endpoints, for KindEdge.
	From, To string
	// Name is the word, for KindGoal and KindPlace naming. Never a rendered description.
	Name string
}

// WhenLearned registers the one place committed changes are announced to.
//
// One observer, not a list: this exists for a person watching, there is exactly one Director, and
// a fan-out nobody needs is a lifetime question nobody has asked. A second call replaces the
// first, which is what makes it safe to set up after the store is already open.
func (s *Store) WhenLearned(fn func(Learning)) {
	if s == nil {
		return
	}
	s.learnedMu.Lock()
	s.learned = fn
	s.learnedMu.Unlock()
}

// announce reports a committed change, if anybody is listening.
//
// Called with NO store lock held, always. The observer is arbitrary code — it renders names,
// which means it reads the store — and calling it under the write lock would deadlock the
// Director at the moment it learned something, which is the worst possible time.
func (s *Store) announce(events ...Learning) {
	if s == nil || len(events) == 0 {
		return
	}
	s.learnedMu.RLock()
	fn := s.learned
	s.learnedMu.RUnlock()
	if fn == nil {
		return
	}
	for _, e := range events {
		fn(e)
	}
}

// learnedObserver is the store's half of the notification, kept in its own type so the field can
// carry its own mutex without widening the store's.
type learnedObserver struct {
	learnedMu sync.RWMutex
	learned   func(Learning)
}
