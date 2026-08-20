package actiongraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Persistence for the action graph.
//
// Two rules shape the format.
//
// NO INTERFACES ON DISK. An encoded Action interface cannot be decoded: the decoder
// has no way to know which concrete type to build. Every stored action is an
// ActionSpec — a typed, closed description — so a node written by one build is
// readable by another, including one that has since gained action types this one
// never heard of.
//
// EXPLICIT VERSION. The document carries a schema number, and reading is written to
// handle the versions that exist rather than assuming the current one. History is
// the one thing in the system that cannot be regenerated, so a format change must
// never be a reason to discard it.

// SchemaVersion is the current on-disk format.
const SchemaVersion = 1

// document is the on-disk form.
type document struct {
	Schema int          `json:"schema"`
	Nodes  []ActionNode `json:"nodes"`
	// Written is when the file was last saved, for diagnostics.
	Written time.Time `json:"written,omitempty"`
}

// Save writes the graph's nodes to path, atomically.
//
// Atomically because history is append-only and a truncated file is worse than a
// stale one: a crash mid-write would otherwise turn a complete history into a
// corrupt one, which is the single failure this store cannot recover from.
func Save(path string, nodes []ActionNode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("actiongraph: preparing %s: %w", filepath.Dir(path), err)
	}
	doc := document{Schema: SchemaVersion, Nodes: nodes, Written: now()}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("actiongraph: encoding: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("actiongraph: writing %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("actiongraph: replacing %s: %w", path, err)
	}
	return nil
}

// Load reads nodes from path. A missing file is not an error — it is an empty
// history, which is the normal state on first run.
func Load(path string) ([]ActionNode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("actiongraph: reading %s: %w", path, err)
	}
	return Decode(data)
}

// Decode reads a stored document, upgrading older formats.
func Decode(data []byte) ([]ActionNode, error) {
	var probe struct {
		Schema int `json:"schema"`
	}
	// A document with a schema number is the current format. Anything else is either
	// the legacy flat array or corrupt, and the two must be told apart before either
	// is assumed.
	if err := json.Unmarshal(data, &probe); err == nil && probe.Schema > 0 {
		if probe.Schema > SchemaVersion {
			// Forward compatibility runs one way. A file from a newer build may hold
			// fields this one would silently drop, and dropping them on the next save
			// would quietly destroy history. Refusing is the safe direction.
			return nil, fmt.Errorf(
				"actiongraph: this history was written by a newer version (schema %d, this build understands %d)",
				probe.Schema, SchemaVersion)
		}
		var doc document
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("actiongraph: decoding schema %d: %w", probe.Schema, err)
		}
		return doc.Nodes, nil
	}

	return decodeLegacy(data)
}

// legacyRecord is the pre-graph format: a bare JSON array of flattened action
// records, written by the previous milestone's CLI store.
//
// It is read, not discarded. Somebody's history exists in this shape, and the whole
// argument for an append-only log is undermined if a format change wipes it.
type legacyRecord struct {
	ID              string                         `json:"id"`
	RequestedIntent string                         `json:"requested_intent"`
	ActionType      directorapi.ActionType         `json:"action_type"`
	ActionDescribed string                         `json:"action"`
	Target          directorapi.ResolvedTarget     `json:"target"`
	StartedAt       string                         `json:"started_at"`
	Before          directorapi.WorldStateSummary  `json:"before"`
	After           directorapi.WorldStateSummary  `json:"after"`
	Verification    directorapi.VerificationResult `json:"verification"`
	Success         bool                           `json:"success"`
	FailureReason   string                         `json:"failure_reason,omitempty"`
	Status          directorapi.ActionStatus       `json:"status"`
	Attempts        int                            `json:"attempts"`
	Reversible      bool                           `json:"reversible"`
	Execution       directorapi.ExecutionOutcome   `json:"execution"`
	DurationMS      int64                          `json:"duration_ms"`
}

// decodeLegacy upgrades the pre-graph format to nodes.
//
// The upgrade is lossy in one direction that matters and is worth being explicit
// about: legacy records stored the action as a DESCRIPTION, not a query, so an
// upgraded node cannot be replayed — there is nothing to re-resolve. They are
// marked as such rather than being given a fabricated query that would resolve to
// something plausible and wrong.
func decodeLegacy(data []byte) ([]ActionNode, error) {
	var records []legacyRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("actiongraph: this file is neither a versioned graph nor a legacy history: %w", err)
	}

	nodes := make([]ActionNode, 0, len(records))
	for i, r := range records {
		id := NodeID(r.ID)
		if id == "" {
			id = NodeID(fmt.Sprintf("legacy_%d", i+1))
		}
		ts, _ := time.Parse("2006-01-02 15:04:05", r.StartedAt)

		node := ActionNode{
			ID:        id,
			Timestamp: ts,
			Goal:      r.ActionDescribed,
			Reason:    r.Target.Explanation,
			Intent: directorapi.Intent{
				Kind: directorapi.IntentAct, Raw: r.RequestedIntent,
			},
			RequestedTarget: directorapi.ReferenceExpression{
				Phrase: r.RequestedIntent, Kind: directorapi.ReferenceLiteral,
			},
			ResolvedTarget: TargetSnapshot{
				ElementID:  string(r.Target.ElementID),
				WindowID:   string(r.Target.WindowID),
				Role:       string(r.Target.Role),
				Label:      r.Target.Label,
				App:        r.Before.Application,
				Confidence: r.Target.Confidence,
				Identity: IdentitySnapshot{
					NativeID: r.Target.NativeID,
				},
			},
			Verification: r.Verification,
			Outcome: OutcomeSummary{
				Success: r.Success, Status: r.Status,
				Reason: r.Verification.Reason, FailureReason: r.FailureReason,
				Before: r.Before, After: r.After,
				Attempts: r.Attempts, DurationMS: r.DurationMS,
				Reversible: r.Reversible,
			},
			Plan: PlanSnapshot{
				Goal: r.ActionDescribed,
				Steps: []StepSnapshot{{
					Index: 0,
					Action: ActionSpec{
						Type:        r.ActionType,
						Description: r.ActionDescribed,
						// Deliberately no Query. The legacy format never stored one,
						// and inventing one from the label would produce a node that
						// LOOKS replayable and would re-resolve by guesswork.
						Query: r.Target.Query,
					},
				}},
			},
			Metadata: map[string]any{
				"upgraded_from_schema": 0,
				"execution_detail":     r.Execution.Detail,
			},
		}
		if r.Execution.Point != nil {
			node.Metadata["execution_point"] = fmt.Sprintf("(%d,%d)",
				r.Execution.Point.X, r.Execution.Point.Y)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// FileGraph is a Memory graph backed by a file.
//
// It loads on open and saves on every Add. Saving eagerly rather than at shutdown is
// deliberate: each CLI invocation is its own short-lived process, and a history that
// only persisted on a clean exit would lose exactly the actions that went wrong.
type FileGraph struct {
	*Memory
	path string
}

// OpenFile loads the graph at path, creating an empty one if it does not exist.
func OpenFile(path string) (*FileGraph, error) {
	nodes, err := Load(path)
	if err != nil {
		return nil, err
	}
	g := &FileGraph{Memory: NewMemory(), path: path}
	for _, n := range nodes {
		// Loaded nodes bypass the parent check: a truncated or partially migrated
		// history may reference a parent that is no longer present, and refusing to
		// load the whole file over one dangling edge would be worse than the edge.
		g.Memory.nodes = append(g.Memory.nodes, n)
		g.Memory.byID[n.ID] = len(g.Memory.nodes) - 1
		if s := seqOf(n.ID); s > g.Memory.seq {
			g.Memory.seq = s
		}
	}
	return g, nil
}

// Add appends a node and persists the graph.
func (f *FileGraph) Add(node ActionNode) error {
	if err := f.Memory.Add(node); err != nil {
		return err
	}
	return f.Save()
}

// Save writes the whole graph out.
func (f *FileGraph) Save() error {
	nodes, err := f.Memory.Recent(0)
	if err != nil {
		return err
	}
	// Recent is newest-first; the file is oldest-first so it reads chronologically.
	for i, j := 0, len(nodes)-1; i < j; i, j = i+1, j-1 {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}
	return Save(f.path, nodes)
}

// Path is where this graph is stored.
func (f *FileGraph) Path() string { return f.path }
