package binding

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// The request-scoped carrier.
//
//	Choose one canonical location for executable actions to carry a deictic binding.
//	Avoid storing the same mutable binding in multiple places.
//
// There is exactly ONE mutable copy of a binding, and it lives here: in a Store owned by
// a single request. An action refers to it by ID — see directorapi.ReferenceExpression's
// BindingID — and everything that needs the binding (confirmation, execution,
// verification, diagnostics) reads it back through that ID.
//
// The alternative, embedding the Binding in the action, looks simpler and is not: the
// action is copied into a plan, into a record and into a graph node, and revalidation
// REFRESHES a binding. Five copies of a mutable fact is five chances for the confirmation
// prompt to describe one object while the executor acts on another.
//
// What the graph keeps is a SNAPSHOT (see snapshot.go), which is immutable history rather
// than a second live copy.
//
// The store is request-scoped by construction: it is created per request, put on that
// request's context, and goes when the context does. A confirmation accepted for one
// command cannot be found by the next because the binding it referred to cannot be.

// ID names a binding within one request.
//
// Deliberately not a global identifier and deliberately not derived from the object: it
// means nothing outside the request that minted it, which is exactly the lifetime a
// binding may be trusted for.
type ID string

// Store holds the live bindings of one request.
type Store struct {
	mu   sync.Mutex
	seq  int
	byID map[ID]*Binding
	// order preserves mint order, so diagnostics list bindings as they were made.
	order []ID
}

// NewStore returns an empty store for one request.
func NewStore() *Store { return &Store{byID: map[ID]*Binding{}} }

// Put files a binding and returns the ID an action refers to it by.
func (s *Store) Put(b *Binding) ID {
	if s == nil || b == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byID == nil {
		s.byID = map[ID]*Binding{}
	}
	s.seq++
	id := ID(fmt.Sprintf("b%d", s.seq))
	b.ID = id
	s.byID[id] = b
	s.order = append(s.order, id)
	return id
}

// Get returns the binding an action refers to.
//
// A COPY, so a caller inspecting a binding cannot quietly mutate the one the executor
// will act on. Refreshing goes through Replace, which is the only writer.
func (s *Store) Get(id ID) (*Binding, bool) {
	if s == nil || id == "" {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.byID[id]
	if !ok {
		return nil, false
	}
	clone := *b
	return &clone, true
}

// Replace installs a refreshed binding under the same ID.
//
//	Update the request-scoped binding. Use the refreshed binding in confirmation,
//	execution, verification, diagnostics and graph provenance.
//
// One writer and one ID, so "the refreshed one" is not a thing a caller can forget to
// use: after this returns, every reader of that ID sees the refreshed binding.
func (s *Store) Replace(id ID, b *Binding) {
	if s == nil || id == "" || b == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := *b
	clone.ID = id
	s.byID[id] = &clone
}

// All returns every binding in the store, in mint order, for diagnostics.
func (s *Store) All() []*Binding {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Binding, 0, len(s.order))
	for _, id := range s.order {
		if b, ok := s.byID[id]; ok {
			clone := *b
			out = append(out, &clone)
		}
	}
	return out
}

// Len reports how many bindings this request made.
func (s *Store) Len() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.byID)
}

// IDs returns the stored IDs in a stable order, for tests and diagnostics.
func (s *Store) IDs() []ID {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]ID{}, s.order...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ── request scope ─────────────────────────────────────────────────────────────

type storeKey struct{}

// WithStore attaches a fresh store to a request's context.
//
// Called once per request. Nesting is harmless and deliberate — a program step derives
// its context from the request's, so it finds the request's store rather than making its
// own, which is what lets step 4 act on the binding step 1 resolved.
func WithStore(ctx context.Context, s *Store) context.Context {
	return context.WithValue(ctx, storeKey{}, s)
}

// StoreFrom returns the request's store, nil when there is none.
//
// Nil is a REFUSAL upstream rather than a licence to bind late: a deictic action whose
// store is missing cannot prove what it points at, and the execution guard stops it.
func StoreFrom(ctx context.Context) *Store {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(storeKey{}).(*Store)
	return s
}
