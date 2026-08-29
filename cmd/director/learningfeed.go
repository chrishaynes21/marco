package main

import (
	"fmt"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// TELL ME WHEN YOU LEARN SOMETHING, SO I CAN TRY IT.
//
// # What this is for
//
// Watching Marco watch is not interesting. What is interesting is the moment durable knowledge
// changes, because that is the moment a person can turn round and ask Marco to use it. Everything
// else — samples, walks, candidates, confidences — is Marco perceiving, and a feed that reported
// perception would bury the six lines a person actually wants under thousands they do not.
//
// # Why it cannot lie
//
// The events come from `semanticmemory.Store` itself, after its own write has committed. Nothing
// here predicts, and nothing upstream announces: a Place refused at a bound, a signature that
// turned out to match an existing record, a file that could not be written — all ordinary — reach
// this buffer as silence, which is the truth.
//
// # Why names are resolved on READ
//
// Because a Place is established on one pass and named on a later one, and a value that carried
// the name it had at the instant of the write would say `[unnamed]` forever about a Place that is
// now perfectly well named. So the store hands over ids and this renders them against the store as
// it is when somebody looks.

// MaxLearningEvents bounds the feed.
//
// Enough for a long session at the rate durable knowledge actually changes — which is a handful of
// events per application, not per sample — and small enough that a Director watching all day
// cannot grow. The oldest are dropped, and `learningSince` says when that has happened so a reader
// is never quietly shown a gap.
const MaxLearningEvents = 256

// learningFeed is the Director.s ring of committed changes.
type learningFeed struct {
	mu sync.Mutex
	// store is the memory these events came from, kept so rendering asks the SAME store
	// that announced them. Going back through the observation registry to find one was the
	// first version, and it rendered every Place as a bare subject id whenever the registry
	// was not the thing holding the memory — which is exactly the shape of "the mechanism is
	// right and it is reading the wrong object".
	store  *semanticmemory.Store
	events []semanticmemory.Learning
	// first is the sequence number of events[0]. Sequence numbers never restart, so a poller
	// that was away can tell "nothing happened" from "more happened than I can be shown".
	first uint64
	next  uint64
}

// record keeps one committed change. Called from the store's own announcement.
func (f *learningFeed) record(e semanticmemory.Learning) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, e)
	f.next++
	if len(f.events) > MaxLearningEvents {
		drop := len(f.events) - MaxLearningEvents
		f.events = append([]semanticmemory.Learning{}, f.events[drop:]...)
		f.first += uint64(drop)
	}
}

// since returns the events after a cursor, with the sequence a caller should ask from next.
//
// A cursor older than the ring reports what was missed rather than silently starting late.
func (f *learningFeed) since(cursor uint64) (out []semanticmemory.Learning, newest, missed uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if cursor < f.first {
		missed = f.first - cursor
		cursor = f.first
	}
	for i := cursor; i < f.next; i++ {
		out = append(out, f.events[i-f.first])
	}
	return out, f.next, missed
}

// watchLearning connects the store's committed changes to the feed.
//
// THE one wiring point. A second consumer of the store's announcement would be a second answer to
// "what has Marco learned", and the two would disagree the first time one of them was restarted.
//
// Deleting this must fail TestTheFeedIsFedByTheStoresOwnCommits.
func (r *Runtime) watchLearning(store *semanticmemory.Store) {
	if r == nil || store == nil {
		return
	}
	r.learned.mu.Lock()
	r.learned.store = store
	r.learned.mu.Unlock()
	store.WhenLearned(func(e semanticmemory.Learning) { r.learned.record(e) })
}

// LearningSince is the protocol read: what has been learned since a cursor, rendered.
func (r *Runtime) LearningSince(q service.ObserveLearning) service.LearningView {
	out := service.LearningView{}
	if r == nil {
		return out
	}
	events, newest, missed := r.learned.since(q.After)
	out.Newest, out.Missed = newest, missed
	for _, e := range events {
		out.Events = append(out.Events, service.LearningEvent{
			Change: string(e.Change), Kind: string(e.Kind), Application: e.Application,
			Description: r.describeLearning(e),
		})
	}
	return out
}

// describeLearning renders one change in the words a person reads.
//
// Names come from the store NOW. An unnamed Place says so and shows its subject, because
// [[ADR-110-a-navigation-rail-is-a-list-of-places-you-could-go]] leaves that a real outcome and
// hiding it would hide the thing this session exists to find out about.
func (r *Runtime) describeLearning(e semanticmemory.Learning) string {
	switch e.Kind {
	case semanticmemory.KindPlace:
		// A naming event says the word and nothing else. The description resolves names at
		// READ time, so by the time anybody sees this the Place already renders as its name —
		// and "Mouse — Mouse now calls itself this" is a sentence nobody needs twice.
		if e.Change == semanticmemory.Named {
			return e.Name
		}
		return r.placeWord(e.Subject)
	case semanticmemory.KindEdge:
		return fmt.Sprintf("%s -> %s", r.placeWord(e.From), r.placeWord(e.To))
	case semanticmemory.KindGoal:
		return fmt.Sprintf("%q -> %s", e.Name, r.placeWord(e.Subject))
	}
	return ""
}

// placeWord is what to call a subject, or an honest admission that Marco cannot.
//
// The Audience's own word first, then what the interface appears to call itself — the same
// precedence every other surface uses, so a Place is not called one thing here and another in the
// Learn panel. Never a goal name and never a play name: those are what somebody wants, not what
// the screen says it is.
func (r *Runtime) placeWord(subject string) string {
	if subject == "" {
		return "[unknown]"
	}
	store, ok := r.placeStore()
	if !ok {
		return shortSubject(subject)
	}
	rec, found := store.Subject(subject)
	if !found {
		return shortSubject(subject)
	}
	if rec.Called != "" {
		return rec.Called
	}
	if rec.Semantic != "" {
		return rec.Semantic
	}
	return "[unnamed " + shortSubject(subject) + "]"
}

func shortSubject(subject string) string {
	if len(subject) > 13 {
		return subject[:13]
	}
	return subject
}

// placeStore is the memory the feed was wired to, when there is one.
func (r *Runtime) placeStore() (*semanticmemory.Store, bool) {
	if r == nil {
		return nil, false
	}
	r.learned.mu.Lock()
	store := r.learned.store
	r.learned.mu.Unlock()
	return store, store != nil
}

var _ observe.Memory = (*semanticmemory.Store)(nil)
