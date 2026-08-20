package collections

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Environment is one running program's collections.
//
// Program-local, exactly like captured values and for the same reason: a collection is
// a query written against a screen the user was looking at, and one that outlived its
// program would be re-run later against a screen nobody had seen. Target VARIABLES
// persist because a single query proved to name one object; a collection's query is
// bounded by a count and a moment, and neither survives.
//
// There is no Save, no path and no loader here. The absence is the design.
type Environment struct {
	mu          sync.RWMutex
	collections map[string]Collection
	// processed records the semantic keys already acted on, per collection. Held here
	// rather than in the iteration context because it must survive a clarification
	// pause: an answer arriving from another process resumes the same iteration, and a
	// forgotten processed-set would re-act on every earlier member.
	processed map[string][]string
	program   string
	cleared   bool
}

// NewEnvironment returns an empty environment.
func NewEnvironment() *Environment {
	return &Environment{
		collections: map[string]Collection{},
		processed:   map[string][]string{},
	}
}

// SetProgram names the run these collections belong to.
func (e *Environment) SetProgram(id string) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.program = id
}

// Program is the run these collections belong to.
func (e *Environment) Program() string {
	if e == nil {
		return ""
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.program
}

// Bind stores a collection under a name.
//
// Rebinding is REFUSED. A collection that could be re-pointed after capture would let a
// later step change what an earlier step named, so "iterate items" would mean different
// things depending on when it ran.
func (e *Environment) Bind(c Collection) error {
	if e == nil {
		return fmt.Errorf("collections: no environment is running")
	}
	if err := c.Validate(); err != nil {
		return err
	}
	name, err := NormalizeName(c.Name)
	if err != nil {
		return err
	}
	c.Name = name

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cleared {
		return fmt.Errorf("collections: this program has finished, so %q cannot be "+
			"captured into it", name)
	}
	if len(e.collections) >= MaximumCollections {
		return fmt.Errorf("collections: a program may capture at most %d collections",
			MaximumCollections)
	}
	if _, taken := e.collections[name]; taken {
		return fmt.Errorf("collections: %q is already captured in this program; "+
			"collections are immutable, so use a different name", name)
	}
	e.collections[name] = c
	return nil
}

// Get returns a collection by name.
func (e *Environment) Get(name string) (Collection, bool) {
	if e == nil {
		return Collection{}, false
	}
	normalised, err := NormalizeName(name)
	if err != nil {
		return Collection{}, false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	c, ok := e.collections[normalised]
	return c, ok
}

// Has reports whether a name is bound.
func (e *Environment) Has(name string) bool {
	_, ok := e.Get(name)
	return ok
}

// ErrUnknownCollection is a reference to a collection this program never captured.
//
// The message says "collection", not "value" or "variable": there are three namespaces
// now, and a user who reached into the wrong one needs to be told which.
type ErrUnknownCollection struct{ Name string }

func (e *ErrUnknownCollection) Error() string {
	return fmt.Sprintf("Unknown program-local collection: %s", e.Name)
}

// Resolve looks a collection up, failing honestly when it is not there.
func (e *Environment) Resolve(name string) (Collection, error) {
	c, ok := e.Get(name)
	if !ok {
		return Collection{}, &ErrUnknownCollection{Name: name}
	}
	return c, nil
}

// MarkProcessed records that a member has been acted on.
//
// Keyed by SEMANTIC identity rather than by position, so a list that reflows after its
// first item is deleted does not present every remaining member as new.
// The ledger is keyed by an INTERNAL string, not a user-facing name. Putting it
// through NormalizeName silently dropped every entry for an inline collection, whose
// ledger key is derived from the request and contains spaces — so every member was
// re-processed until the limit, which is exactly the runaway this ledger prevents.
func (e *Environment) MarkProcessed(collection, key string) {
	if e == nil || key == "" || collection == "" {
		return
	}
	name := collection
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cleared {
		return
	}
	for _, seen := range e.processed[name] {
		if seen == key {
			return
		}
	}
	e.processed[name] = append(e.processed[name], key)
}

// Processed reports the semantic keys already acted on, in order.
func (e *Environment) Processed(collection string) []string {
	if e == nil {
		return nil
	}
	name := collection
	e.mu.RLock()
	defer e.mu.RUnlock()
	return append([]string(nil), e.processed[name]...)
}

// WasProcessed reports whether this member has already been acted on.
func (e *Environment) WasProcessed(collection, key string) bool {
	for _, seen := range e.Processed(collection) {
		if seen == key {
			return true
		}
	}
	return false
}

// Clear discards every collection and marks the environment finished.
//
// The processed-key history goes with them. It is a record of what one program did to
// one screen, and keeping it would let a later program's identical collection think its
// members had already been handled.
func (e *Environment) Clear() int {
	if e == nil {
		return 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	n := len(e.collections)
	for name := range e.collections {
		delete(e.collections, name)
	}
	for name := range e.processed {
		delete(e.processed, name)
	}
	e.cleared = true
	return n
}

// Cleared reports whether the owning program has finished.
func (e *Environment) Cleared() bool {
	if e == nil {
		return true
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.cleared
}

// Count is how many collections are bound.
func (e *Environment) Count() int {
	if e == nil {
		return 0
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.collections)
}

// Snapshot describes the environment safely, as copies.
//
// Taken under the lock and returned detached, so a caller renders and serialises with
// nothing held — the same discipline the value environment follows.
type Snapshot struct {
	ProgramID   string              `json:"program_id,omitempty"`
	TakenAt     time.Time           `json:"taken_at"`
	Collections []CollectionSummary `json:"collections"`
	Cleared     bool                `json:"cleared"`
}

// CollectionSummary is one collection, described safely.
type CollectionSummary struct {
	Name       string     `json:"name"`
	Kind       Kind       `json:"kind"`
	Query      string     `json:"query"`
	Ordering   Ordering   `json:"ordering"`
	Limit      int        `json:"limit"`
	Provenance Provenance `json:"provenance"`
	// ProcessedCount is how many members have been acted on so far. A COUNT rather
	// than the keys, because a key is an opaque digest that means nothing to a reader
	// and would only invite something to try matching on it.
	ProcessedCount int `json:"processed_count"`
}

// Find returns one collection's summary by name.
func (s Snapshot) Find(name string) (CollectionSummary, bool) {
	normalised, err := NormalizeName(name)
	if err != nil {
		return CollectionSummary{}, false
	}
	for _, c := range s.Collections {
		if c.Name == normalised {
			return c, true
		}
	}
	return CollectionSummary{}, false
}

// Describe snapshots the environment safely.
func (e *Environment) Describe() Snapshot {
	if e == nil {
		return Snapshot{TakenAt: time.Now(), Cleared: true, Collections: []CollectionSummary{}}
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := Snapshot{
		ProgramID: e.program, TakenAt: time.Now(), Cleared: e.cleared,
		Collections: []CollectionSummary{},
	}
	names := make([]string, 0, len(e.collections))
	for n := range e.collections {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		c := e.collections[name]
		out.Collections = append(out.Collections, CollectionSummary{
			Name: name, Kind: c.Kind, Query: c.Query.Describe(),
			Ordering: c.Query.Ordering, Limit: c.Query.Limit,
			Provenance: c.Provenance, ProcessedCount: len(e.processed[name]),
		})
	}
	return out
}

// ProcessedCount is how many members have been acted on.
//
// Read on resume so a paused collection reports its real progress rather than counting
// from zero: the ledger is the only durable record of what was done, and it survived
// the pause precisely so this number can be trusted.
func (e *Environment) ProcessedCount(collection string) int {
	return len(e.Processed(collection))
}
