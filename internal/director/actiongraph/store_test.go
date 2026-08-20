package actiongraph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ── round trip ────────────────────────────────────────────────────────────────

// Everything a node carries must survive storage. Notably the QUERY: a node whose
// query did not round-trip would look replayable and have nothing to re-resolve.
func TestRoundTripPreservesSemantics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.json")

	original := node("action_1", "notepad", "File", rect(110, 140, 40, 30))
	original.Metadata = map[string]any{"execution_detail": "left click sent"}

	if err := Save(path, []ActionNode{original}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("want 1 node, got %d", len(loaded))
	}
	got := loaded[0]

	if got.ID != original.ID || got.Goal != original.Goal {
		t.Errorf("identity lost: %+v", got)
	}
	spec, ok := got.Action()
	if !ok {
		t.Fatal("the action did not survive")
	}
	if spec.Query == nil || spec.Query.Label != "File" {
		t.Fatal("the target query did not survive — the node would look replayable and be unable to re-resolve")
	}
	if got.ResolvedTarget.Identity.NativeID != "uia:1.1" {
		t.Error("the identity snapshot did not survive")
	}
	// And it is still the SAME action, by meaning.
	if !Equivalent(original, got) {
		t.Error("a stored and reloaded node should be semantically identical to the original")
	}
}

// The spec's rule: no interface types on disk. An encoded Action interface cannot be
// decoded, so the stored form must be a closed, typed spec.
func TestStoredFormatHasNoInterfaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.json")
	if err := Save(path, []ActionNode{node("action_1", "notepad", "File", rect(0, 0, 10, 10))}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, _ := os.ReadFile(path)

	var doc struct {
		Schema int `json:"schema"`
		Nodes  []struct {
			Plan struct {
				Steps []struct {
					Action map[string]any `json:"action"`
				} `json:"steps"`
			} `json:"plan"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("the document should be plain JSON: %v", err)
	}
	if doc.Schema != SchemaVersion {
		t.Errorf("schema = %d, want %d", doc.Schema, SchemaVersion)
	}
	action := doc.Nodes[0].Plan.Steps[0].Action
	if _, ok := action["type"]; !ok {
		t.Error("a stored action must name its type so it can be rebuilt")
	}
	if _, ok := action["query"]; !ok {
		t.Error("a stored action must carry its query")
	}
}

// A missing file is an empty history, which is the normal state on first run.
func TestMissingFileIsEmptyNotAnError(t *testing.T) {
	nodes, err := Load(filepath.Join(t.TempDir(), "never-written.json"))
	if err != nil {
		t.Fatalf("a missing history must not be an error: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("want no nodes, got %d", len(nodes))
	}
}

// ── schema compatibility ──────────────────────────────────────────────────────

// The pre-graph format. Somebody's history exists in this shape, and the argument
// for an append-only log is undermined if a format change wipes it.
const legacyHistory = `[
  {
    "id": "action_1",
    "requested_intent": "click File",
    "action_type": "click",
    "action": "click \"File\"",
    "target": {"element_id":"e12","window_id":"hwnd:1","role":"menu_item","label":"File","confidence":0.97},
    "started_at": "2026-08-03 14:22:01",
    "before": {"window_title":"Untitled - Notepad","element_count":45,"application":"notepad","fingerprint":"abc123"},
    "after": {"window_title":"Untitled - Notepad","element_count":75,"fingerprint":"def456"},
    "verification": {"success":true,"confidence":0.93,"reason":"13 menu items appeared"},
    "success": true,
    "status": "succeeded",
    "attempts": 1,
    "reversible": false,
    "duration_ms": 812
  }
]`

func TestLegacyHistoryUpgrades(t *testing.T) {
	nodes, err := Decode([]byte(legacyHistory))
	if err != nil {
		t.Fatalf("the legacy format must still be readable: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("want 1 upgraded node, got %d", len(nodes))
	}
	n := nodes[0]

	if n.ID != "action_1" {
		t.Errorf("id = %q", n.ID)
	}
	if n.Goal != `click "File"` {
		t.Errorf("goal = %q", n.Goal)
	}
	if !n.Outcome.Success || n.Verification.Reason != "13 menu items appeared" {
		t.Errorf("the outcome did not survive: %+v", n.Outcome)
	}
	if n.ResolvedTarget.Label != "File" || n.ResolvedTarget.App != "notepad" {
		t.Errorf("the target did not survive: %+v", n.ResolvedTarget)
	}
	if n.Outcome.Before.Fingerprint != "abc123" {
		t.Error("the world summaries did not survive")
	}
	// The upgrade is honestly labelled.
	if n.Metadata["upgraded_from_schema"] == nil {
		t.Error("an upgraded node should say where it came from")
	}
}

// The legacy format never stored a query, so an upgraded node has nothing to
// re-resolve. It must be reported as non-replayable rather than given a fabricated
// query that would resolve to something plausible and wrong.
func TestUpgradedLegacyNodesAreNotReplayable(t *testing.T) {
	nodes, err := Decode([]byte(legacyHistory))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	c := AnalyzeReplay(nodes[0], notepadScene())
	if c.Replayable {
		t.Fatal("a legacy node has no target description; claiming it is replayable would be a guess")
	}
	if c.Status != ReplayUnsafe {
		t.Errorf("status = %s, want UNSAFE", c.Status)
	}
}

// After an upgrade, saving writes the CURRENT format — the migration is permanent
// rather than re-done on every read.
func TestUpgradeIsPersistedOnNextSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.json")
	if err := os.WriteFile(path, []byte(legacyHistory), 0o600); err != nil {
		t.Fatal(err)
	}

	g, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if g.Len() != 1 {
		t.Fatalf("the legacy record should have loaded, got %d", g.Len())
	}
	if err := g.Add(node("action_2", "notepad", "Edit", rect(0, 0, 10, 10))); err != nil {
		t.Fatalf("Add: %v", err)
	}

	data, _ := os.ReadFile(path)
	var doc struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal(data, &doc); err != nil || doc.Schema != SchemaVersion {
		t.Errorf("the file should have been rewritten in the current schema, got %d (%v)", doc.Schema, err)
	}
	// Both the upgraded and the new node are there — nothing was dropped.
	reloaded, err := Load(path)
	if err != nil || len(reloaded) != 2 {
		t.Errorf("want both nodes after the upgrade, got %d (%v)", len(reloaded), err)
	}
}

// Forward compatibility runs one way. A file from a newer build may hold fields this
// one would silently drop, and dropping them on the next save would destroy history.
func TestNewerSchemaIsRefusedRatherThanTruncated(t *testing.T) {
	future := []byte(`{"schema": 999, "nodes": []}`)
	_, err := Decode(future)
	if err == nil {
		t.Fatal("a newer schema must be refused, not silently downgraded")
	}
	if !strings.Contains(err.Error(), "newer") {
		t.Errorf("the error should explain the version problem, got %q", err)
	}
}

func TestCorruptFileIsAnErrorNotAnEmptyHistory(t *testing.T) {
	if _, err := Decode([]byte(`{"this": "is not a graph"`)); err == nil {
		t.Error("a corrupt file must be an error — silently returning an empty history would look like data loss")
	}
}

// ── file-backed graph ─────────────────────────────────────────────────────────

// Each CLI invocation is its own short-lived process, so a history that only
// persisted on a clean exit would lose exactly the actions that went wrong.
func TestFileGraphPersistsOnEveryAdd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graph.json")

	g, err := OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if err := g.Add(node("action_1", "notepad", "File", rect(0, 0, 10, 10))); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// A separate reader, as a separate process would be.
	reopened, err := OpenFile(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if reopened.Len() != 1 {
		t.Fatalf("the node should be on disk immediately, got %d", reopened.Len())
	}
	// And ids continue rather than restarting, or two actions would share a name.
	if got := reopened.NextID(); got != "action_2" {
		t.Errorf("NextID after reload = %q, want action_2", got)
	}
}

func TestFileGraphStoresOldestFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graph.json")
	g, _ := OpenFile(path)
	_ = g.Add(node("action_1", "notepad", "File", rect(0, 0, 10, 10)))
	_ = g.Add(node("action_2", "notepad", "Edit", rect(0, 0, 10, 10)))

	nodes, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(nodes) != 2 || nodes[0].ID != "action_1" {
		t.Errorf("the file should read chronologically, got %v", ids(nodes))
	}
	// ...while the API returns newest-first.
	recent, _ := g.Recent(0)
	if recent[0].ID != "action_2" {
		t.Errorf("Recent should be newest-first, got %v", ids(recent))
	}
}

// A dangling parent must not make the whole history unreadable: refusing to load
// over one bad edge would be worse than the edge.
func TestLoadToleratesADanglingParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graph.json")
	orphan := node("action_2", "notepad", "Save", rect(0, 0, 10, 10))
	orphan.Parent = idPtr("action_1") // never written
	if err := Save(path, []ActionNode{orphan}); err != nil {
		t.Fatal(err)
	}

	g, err := OpenFile(path)
	if err != nil {
		t.Fatalf("a dangling parent must not make the history unreadable: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("the node should still load, got %d", g.Len())
	}
	// And chain-walking must terminate rather than hang or panic.
	if chain := g.Chain("action_2"); len(chain) != 1 {
		t.Errorf("Chain should stop at the missing parent, got %v", ids(chain))
	}
}

// ── conversion from execution ─────────────────────────────────────────────────

// Coordinates may appear as evidence; they must never be the identity.
func TestConversionKeepsCoordinatesOutOfTheSemanticModel(t *testing.T) {
	point := directorapi.Point{X: 842, Y: 391}
	rec := directorapi.ActionRecord{
		ID:              "action_1",
		RequestedIntent: "click Back",
		Target: directorapi.ResolvedTarget{
			ElementID: "e5", WindowID: "hwnd:1", Role: directorapi.RoleButton,
			Label: "Back", Query: &directorapi.ElementQuery{Label: "Back"},
			Explanation: "exact label match, an operable button",
		},
		Before:       directorapi.WorldStateSummary{Application: "chrome", Fingerprint: "aaa"},
		After:        directorapi.WorldStateSummary{Fingerprint: "bbb"},
		Verification: directorapi.VerificationResult{Success: true, Confidence: 0.91},
		Success:      true,
		Status:       directorapi.ActionSucceeded,
		Execution:    directorapi.ExecutionOutcome{Performed: true, Point: &point, Detail: "left click sent"},
	}
	plan := directorapi.Plan{
		Goal: `click "Back"`, Risk: directorapi.RiskMedium,
		Steps: []directorapi.PlanStep{{
			Action: directorapi.ClickAction{Target: directorapi.ElementReference{
				Query: &directorapi.ElementQuery{Label: "Back"}, Description: "Back",
			}},
		}},
	}

	n := FromExecution("action_1", Source{
		Record: rec, Plan: plan,
		Intent: directorapi.Intent{Kind: directorapi.IntentAct, Verb: "click", Raw: "click Back"},
	})

	// The semantic identity must not depend on the point.
	key := n.SemanticKey()
	n.Metadata["execution_point"] = [2]int{1, 1}
	if n.SemanticKey() != key {
		t.Error("the semantic key changed with a coordinate — coordinates must not be identity")
	}
	// The point is recorded, as evidence.
	if _, ok := n.Metadata["execution_point"]; !ok {
		t.Error("where the action landed should be kept as metadata")
	}
	// The query — the thing that makes it repeatable — is in the plan.
	spec, _ := n.Action()
	if spec.Query == nil || spec.Query.Label != "Back" {
		t.Error("the query must be carried into the node")
	}
	if n.Goal != `click "Back"` {
		t.Errorf("goal = %q", n.Goal)
	}
	if n.Reason == "" {
		t.Error("the resolver's justification should be kept — it answers 'why that one?'")
	}
}

// A window move stores its NAMED placement, not just the rectangle: on a different
// monitor layout the rectangle would be wrong and the name still right.
func TestWindowMoveConversionKeepsTheNamedPlacement(t *testing.T) {
	dest := rect(0, 0, 960, 1040)
	plan := directorapi.Plan{
		Goal: "move the window left half",
		Steps: []directorapi.PlanStep{{
			Action: directorapi.MoveWindowAction{
				Window: directorapi.WindowReference{
					ID: "hwnd:1", Application: "notepad", Description: "Untitled - Notepad",
				},
				Placement: directorapi.WindowPlacement{Named: "left_half", Bounds: &dest},
			},
		}},
	}
	n := FromExecution("action_1", Source{
		Record: directorapi.ActionRecord{ID: "action_1", Success: true},
		Plan:   plan,
	})

	spec, _ := n.Action()
	if spec.Placement == nil || spec.Placement.Named != "left_half" {
		t.Fatalf("the named placement must be kept, got %+v", spec.Placement)
	}
	if spec.Window == nil || spec.Window.Application != "notepad" {
		t.Error("the window's application must be kept — the handle will not survive a restart")
	}

	// Rebuilding drops the rectangle in favour of the name.
	rebuilt, err := spec.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	mv := rebuilt.(directorapi.MoveWindowAction)
	if mv.Placement.Named != "left_half" {
		t.Error("a rebuilt move should use the named placement")
	}
	if mv.Placement.Bounds != nil {
		t.Error("a rebuilt move must not replay a stale rectangle")
	}
	if mv.Window.ID != "" {
		t.Error("a rebuilt move must not reuse the old window handle")
	}
}

// A rebuilt element action carries the query and no coordinates, so executing it
// re-resolves.
func TestRebuiltActionsReResolve(t *testing.T) {
	spec := ActionSpec{
		Type:  directorapi.ActionClick,
		Query: &directorapi.ElementQuery{Label: "Save"},
	}
	action, err := spec.Rebuild()
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	click := action.(directorapi.ClickAction)
	if click.Target.Query == nil || click.Target.Query.Label != "Save" {
		t.Error("a rebuilt click must carry its query")
	}
	if click.Target.Point != nil || click.Target.CoordinateLocked {
		t.Error("a rebuilt click must not carry coordinates")
	}
	if click.Target.ID != "" {
		t.Error("a rebuilt click must not reuse a stale element id")
	}
}

func TestRebuildRefusesWithoutAQuery(t *testing.T) {
	for name, spec := range map[string]ActionSpec{
		"click": {Type: directorapi.ActionClick},
		"focus": {Type: directorapi.ActionFocus},
		"move":  {Type: directorapi.ActionMoveWindow},
	} {
		if _, err := spec.Rebuild(); err == nil {
			t.Errorf("%s with no target must not rebuild", name)
		}
	}
}
