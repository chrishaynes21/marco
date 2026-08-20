package actiongraph

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ErrNotFound is returned when a node is not in the graph.
var ErrNotFound = errors.New("actiongraph: no such node")

// ErrImmutable is returned when something tries to add a node whose id already
// exists. Nodes are facts about the past; a second fact with the same name is a bug,
// not an update.
var ErrImmutable = errors.New("actiongraph: nodes are append-only and cannot be replaced")

// Graph is the Director's append-only store of semantic actions.
//
// There is no Update and no Delete, and that is the design rather than an omission.
// If an action is later replayed and fails, that is a NEW node with the old one as
// its parent — the history then shows both what was expected and what happened,
// which is precisely the information an edit would have destroyed.
type Graph interface {
	// Add appends a node. It fails if the id is already present.
	Add(node ActionNode) error
	// Get returns a node by id.
	Get(id NodeID) (ActionNode, error)
	// Last returns the most recently added node.
	Last() (ActionNode, error)
	// Recent returns up to limit nodes, newest first. limit <= 0 means all.
	Recent(limit int) ([]ActionNode, error)
}

// Memory is an in-memory Graph, safe for concurrent use.
type Memory struct {
	mu    sync.RWMutex
	nodes []ActionNode
	byID  map[NodeID]int
	seq   int
}

// NewMemory returns an empty in-memory graph.
func NewMemory() *Memory {
	return &Memory{byID: map[NodeID]int{}}
}

var _ Graph = (*Memory)(nil)

// Add appends a node.
//
// The stored node is a deep-enough copy that a caller holding the original cannot
// reach in and change what the graph reports later. Immutability that depends on
// callers behaving is not immutability.
func (m *Memory) Add(node ActionNode) error {
	if node.ID == "" {
		return fmt.Errorf("actiongraph: a node needs an id")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.byID[node.ID]; exists {
		return fmt.Errorf("%w: %s", ErrImmutable, node.ID)
	}
	if node.Parent != nil {
		if _, ok := m.byID[*node.Parent]; !ok {
			return fmt.Errorf("actiongraph: parent %s is not in the graph", *node.Parent)
		}
	}
	m.byID[node.ID] = len(m.nodes)
	m.nodes = append(m.nodes, copyNode(node))
	if n := seqOf(node.ID); n > m.seq {
		m.seq = n
	}
	return nil
}

// Get returns a copy of the node with this id.
func (m *Memory) Get(id NodeID) (ActionNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	i, ok := m.byID[id]
	if !ok {
		return ActionNode{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return copyNode(m.nodes[i]), nil
}

// Last returns the most recently added node.
func (m *Memory) Last() (ActionNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.nodes) == 0 {
		return ActionNode{}, ErrNotFound
	}
	return copyNode(m.nodes[len(m.nodes)-1]), nil
}

// Recent returns up to limit nodes, newest first.
func (m *Memory) Recent(limit int) ([]ActionNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.nodes) {
		limit = len(m.nodes)
	}
	out := make([]ActionNode, 0, limit)
	for i := len(m.nodes) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, copyNode(m.nodes[i]))
	}
	return out, nil
}

// Len reports how many nodes are held.
func (m *Memory) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.nodes)
}

// Children returns the nodes whose parent is id, oldest first — the forward edges
// that future workflow chains will walk.
func (m *Memory) Children(id NodeID) []ActionNode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []ActionNode
	for _, n := range m.nodes {
		if n.Parent != nil && *n.Parent == id {
			out = append(out, copyNode(n))
		}
	}
	return out
}

// Chain walks parent links from a node back to its root, oldest first. This is what
// answers "how did we get here" for a node that was one step of a longer sequence.
func (m *Memory) Chain(id NodeID) []ActionNode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var back []ActionNode
	seen := map[NodeID]bool{}
	cur := id
	for {
		i, ok := m.byID[cur]
		if !ok || seen[cur] {
			break // missing, or a cycle that should be impossible but must not hang
		}
		seen[cur] = true
		node := m.nodes[i]
		back = append(back, copyNode(node))
		if node.Parent == nil {
			break
		}
		cur = *node.Parent
	}
	for i, j := 0, len(back)-1; i < j; i, j = i+1, j-1 {
		back[i], back[j] = back[j], back[i]
	}
	return back
}

// NextID mints the next node id.
//
// Sequential rather than random so history reads in an obvious order and a test can
// predict it. Seeded from whatever was loaded, so ids stay unique across processes —
// each CLI invocation is its own process, and restarting the count would give two
// different actions the same name.
func (m *Memory) NextID() NodeID {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	return NodeID(fmt.Sprintf("action_%d", m.seq))
}

// Seed advances the id counter so a graph loaded from storage continues where it
// left off.
func (m *Memory) Seed(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n > m.seq {
		m.seq = n
	}
}

// FindBySemanticKey returns the most recent node with the given semantic key —
// "have we done this before?", asked by meaning rather than by id.
func (m *Memory) FindBySemanticKey(key string) (ActionNode, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := len(m.nodes) - 1; i >= 0; i-- {
		if m.nodes[i].SemanticKey() == key {
			return copyNode(m.nodes[i]), true
		}
	}
	return ActionNode{}, false
}

// LastSuccessful returns the most recent node that succeeded AND verified.
//
// Verification is part of the test on purpose: "do that again" means repeating
// something that worked, and an action that was performed but could not be confirmed
// is not something to repeat confidently.
func (m *Memory) LastSuccessful() (ActionNode, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := len(m.nodes) - 1; i >= 0; i-- {
		if m.nodes[i].Outcome.Success && m.nodes[i].Verification.Success {
			return copyNode(m.nodes[i]), true
		}
	}
	return ActionNode{}, false
}

// seqOf extracts the number from an "action_N" id, or 0.
func seqOf(id NodeID) int {
	const prefix = "action_"
	s := string(id)
	if len(s) <= len(prefix) || s[:len(prefix)] != prefix {
		return 0
	}
	n := 0
	for _, c := range s[len(prefix):] {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// copyNode deep-copies the parts of a node that a caller could otherwise mutate.
//
// Slices and maps are shared by assignment in Go, so a shallow copy would leave the
// graph's contents reachable — and editable — through any node ever returned. For an
// append-only store that is not a small leak; it is the invariant.
func copyNode(n ActionNode) ActionNode {
	out := n

	out.Preconditions = append([]directorapi.Condition(nil), n.Preconditions...)
	out.SuccessConditions = append([]directorapi.Condition(nil), n.SuccessConditions...)
	out.Verification.Evidence = append([]directorapi.Evidence(nil), n.Verification.Evidence...)

	out.Plan.Assumptions = append([]string(nil), n.Plan.Assumptions...)
	out.Plan.Steps = make([]StepSnapshot, len(n.Plan.Steps))
	for i, s := range n.Plan.Steps {
		step := s
		step.Expect = append([]directorapi.Condition(nil), s.Expect...)
		step.Action = copySpec(s.Action)
		out.Plan.Steps[i] = step
	}

	out.Intent.Targets = append([]directorapi.ReferenceExpression(nil), n.Intent.Targets...)
	out.Intent.Parameters = copyAnyMap(n.Intent.Parameters)
	out.Metadata = copyAnyMap(n.Metadata)

	if n.Parent != nil {
		parent := *n.Parent
		out.Parent = &parent
	}
	return out
}

func copySpec(s ActionSpec) ActionSpec {
	out := s
	if s.Query != nil {
		q := *s.Query
		out.Query = &q
	}
	if s.Window != nil {
		w := *s.Window
		out.Window = &w
	}
	if s.Placement != nil {
		p := *s.Placement
		if s.Placement.Bounds != nil {
			b := *s.Placement.Bounds
			p.Bounds = &b
		}
		out.Placement = &p
	}
	return out
}

func copyAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// now is indirected so tests can pin timestamps.
var now = time.Now
