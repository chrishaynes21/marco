package actiongraph

import (
	"errors"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

var t0 = time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

func rect(x, y, w, h int) directorapi.Rect {
	return directorapi.Rect{X: x, Y: y, Width: w, Height: h}
}

// node builds a click node against a labelled target.
func node(id, app, label string, r directorapi.Rect) ActionNode {
	return ActionNode{
		ID:        NodeID(id),
		Timestamp: t0,
		Goal:      "click " + label,
		Intent: directorapi.Intent{
			Kind: directorapi.IntentAct, Verb: "click", Raw: "click " + label,
			Targets: []directorapi.ReferenceExpression{{Phrase: label}},
		},
		RequestedTarget: directorapi.ReferenceExpression{Phrase: label},
		Plan: PlanSnapshot{
			Goal: "click " + label,
			Risk: directorapi.RiskLow,
			Steps: []StepSnapshot{{
				Action: ActionSpec{
					Type:        directorapi.ActionClick,
					Query:       &directorapi.ElementQuery{Label: label},
					Description: label,
				},
			}},
		},
		ResolvedTarget: TargetSnapshot{
			App: app, Role: "menu_item", Label: label, Bounds: r,
			ElementID: "e1", WindowID: "hwnd:1",
			Identity: IdentitySnapshot{NativeID: "uia:1.1", LabelUnique: true, Durable: true},
		},
		Verification: directorapi.VerificationResult{Success: true, Confidence: 0.9},
		Outcome:      OutcomeSummary{Success: true, Status: directorapi.ActionSucceeded},
	}
}

// ── graph insertion ───────────────────────────────────────────────────────────

func TestInsertionAndLookup(t *testing.T) {
	g := NewMemory()
	for _, id := range []string{"a", "b", "c"} {
		if err := g.Add(node(id, "notepad", "File", rect(10, 10, 40, 24))); err != nil {
			t.Fatalf("Add(%s): %v", id, err)
		}
	}
	if g.Len() != 3 {
		t.Fatalf("want 3 nodes, got %d", g.Len())
	}

	got, err := g.Get("b")
	if err != nil || got.ID != "b" {
		t.Fatalf("Get(b) = %v, %v", got.ID, err)
	}
	last, err := g.Last()
	if err != nil || last.ID != "c" {
		t.Fatalf("Last() = %v, %v", last.ID, err)
	}
	recent, err := g.Recent(2)
	if err != nil || len(recent) != 2 || recent[0].ID != "c" || recent[1].ID != "b" {
		t.Fatalf("Recent(2) should be newest-first, got %v", ids(recent))
	}

	if _, err := g.Get("nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a missing node should report ErrNotFound, got %v", err)
	}
	if _, err := NewMemory().Last(); !errors.Is(err, ErrNotFound) {
		t.Errorf("an empty graph has no last node, got %v", err)
	}
}

func TestNodeNeedsAnID(t *testing.T) {
	n := node("", "notepad", "File", rect(0, 0, 10, 10))
	if err := NewMemory().Add(n); err == nil {
		t.Error("a node with no id must be rejected")
	}
}

// ── immutable history ─────────────────────────────────────────────────────────

// The append-only invariant. History is a record of what happened; rewriting it to
// match what happened next destroys exactly the information that makes a failure
// diagnosable.
func TestHistoryIsImmutable(t *testing.T) {
	g := NewMemory()
	original := node("a", "notepad", "File", rect(10, 10, 40, 24))
	if err := g.Add(original); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Re-adding the same id is refused rather than treated as an update.
	replacement := node("a", "notepad", "Something Else", rect(0, 0, 1, 1))
	if err := g.Add(replacement); !errors.Is(err, ErrImmutable) {
		t.Fatalf("replacing a node must be refused, got %v", err)
	}
	if got, _ := g.Get("a"); got.ResolvedTarget.Label != "File" {
		t.Errorf("the original node changed: %q", got.ResolvedTarget.Label)
	}

	// Mutating the value that was handed IN must not reach the graph.
	original.ResolvedTarget.Label = "tampered"
	original.Plan.Steps[0].Action.Query.Label = "tampered"
	if got, _ := g.Get("a"); got.ResolvedTarget.Label != "File" {
		t.Error("mutating the caller's copy changed the stored node")
	}
	if got, _ := g.Get("a"); got.Plan.Steps[0].Action.Query.Label != "File" {
		t.Error("the stored query shares memory with the caller's")
	}

	// Mutating a value handed OUT must not reach the graph either. Immutability that
	// depends on callers behaving is not immutability.
	handed, _ := g.Get("a")
	handed.ResolvedTarget.Label = "tampered"
	handed.Verification.Evidence = append(handed.Verification.Evidence,
		directorapi.Evidence{Kind: "fake"})
	if got, _ := g.Get("a"); got.ResolvedTarget.Label != "File" || len(got.Verification.Evidence) != 0 {
		t.Error("mutating a returned node changed the graph")
	}
}

// A failed replay appends a new node rather than editing the old one, so the history
// shows both what was expected and what happened.
func TestFailureAppendsRatherThanOverwrites(t *testing.T) {
	g := NewMemory()
	first := node("a", "notepad", "File", rect(10, 10, 40, 24))
	_ = g.Add(first)

	second := node("b", "notepad", "File", rect(10, 10, 40, 24))
	second.Parent = idPtr("a")
	second.Outcome = OutcomeSummary{Success: false, Status: directorapi.ActionFailed,
		FailureReason: "the menu did not open"}
	second.Verification = directorapi.VerificationResult{Success: false}
	if err := g.Add(second); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if g.Len() != 2 {
		t.Fatalf("both attempts should be recorded, got %d", g.Len())
	}
	if got, _ := g.Get("a"); !got.Outcome.Success {
		t.Error("the earlier success must not be rewritten by the later failure")
	}
	// And the successful one is still what "that" would refer to.
	if s, ok := g.LastSuccessful(); !ok || s.ID != "a" {
		t.Errorf("LastSuccessful = %v, want the one that worked", s.ID)
	}
}

// ── relationships ─────────────────────────────────────────────────────────────

func TestParentChildRelationships(t *testing.T) {
	g := NewMemory()
	_ = g.Add(node("a", "notepad", "File", rect(10, 10, 40, 24)))

	child := node("b", "notepad", "Save", rect(20, 60, 100, 24))
	child.Parent = idPtr("a")
	if err := g.Add(child); err != nil {
		t.Fatalf("Add child: %v", err)
	}

	grand := node("c", "notepad", "OK", rect(200, 200, 60, 24))
	grand.Parent = idPtr("b")
	_ = g.Add(grand)

	if kids := g.Children("a"); len(kids) != 1 || kids[0].ID != "b" {
		t.Errorf("Children(a) = %v", ids(kids))
	}
	chain := g.Chain("c")
	if len(chain) != 3 || chain[0].ID != "a" || chain[2].ID != "c" {
		t.Errorf("Chain(c) should run root-first a→b→c, got %v", ids(chain))
	}
}

// A parent that is not in the graph is a dangling edge, and accepting one would make
// chain-walking unreliable.
func TestUnknownParentIsRejected(t *testing.T) {
	g := NewMemory()
	orphan := node("b", "notepad", "Save", rect(0, 0, 10, 10))
	orphan.Parent = idPtr("nonexistent")
	if err := g.Add(orphan); err == nil {
		t.Error("a node whose parent is missing must be refused")
	}
}

// ── semantic identity ─────────────────────────────────────────────────────────

// The central claim of the package: the same action performed on different days is
// the same action, even though every coordinate, element id and window handle
// differs.
func TestSemanticIdentityIgnoresPosition(t *testing.T) {
	today := node("a", "notepad", "Save", rect(900, 700, 90, 30))
	tomorrow := node("b", "notepad", "Save", rect(120, 340, 90, 30))
	tomorrow.ResolvedTarget.ElementID = "e77"
	tomorrow.ResolvedTarget.WindowID = "hwnd:99999"
	tomorrow.ResolvedTarget.Identity.NativeID = "uia:9.9"
	tomorrow.Timestamp = t0.Add(24 * time.Hour)

	if !Equivalent(today, tomorrow) {
		t.Errorf("the same action a day later should be equivalent:\n  %s\n  %s",
			today.SemanticKey(), tomorrow.SemanticKey())
	}
}

func TestSemanticIdentityDistinguishesDifferentActions(t *testing.T) {
	base := node("a", "notepad", "Save", rect(10, 10, 40, 24))

	cases := map[string]ActionNode{
		"a different label":       node("b", "notepad", "Cancel", rect(10, 10, 40, 24)),
		"a different application": node("c", "chrome", "Save", rect(10, 10, 40, 24)),
	}
	for name, other := range cases {
		if Equivalent(base, other) {
			t.Errorf("%s should not be equivalent", name)
		}
	}

	// A different role is a different control even with the same name.
	differentRole := node("d", "notepad", "Save", rect(10, 10, 40, 24))
	differentRole.ResolvedTarget.Role = "button"
	if Equivalent(base, differentRole) {
		t.Error("a button and a menu item with the same label are different targets")
	}

	// Label decoration is not a difference: toolkits add accelerators and ellipses.
	decorated := node("e", "notepad", "&Save...", rect(10, 10, 40, 24))
	if !Equivalent(base, decorated) {
		t.Error("an accelerator and an ellipsis should not change the identity")
	}
}

// Moving a window left and moving it right are different actions, so placement is
// part of the identity.
func TestPlacementIsPartOfWindowActionIdentity(t *testing.T) {
	left := windowNode("a", "notepad", "left_half")
	right := windowNode("b", "notepad", "right_half")
	alsoLeft := windowNode("c", "notepad", "left_half")

	if Equivalent(left, right) {
		t.Error("moving left and moving right are different actions")
	}
	if !Equivalent(left, alsoLeft) {
		t.Error("two moves to the same place are the same action")
	}
}

func TestFindBySemanticKey(t *testing.T) {
	g := NewMemory()
	_ = g.Add(node("a", "notepad", "Save", rect(10, 10, 40, 24)))
	_ = g.Add(node("b", "notepad", "Cancel", rect(60, 10, 40, 24)))
	later := node("c", "notepad", "Save", rect(500, 500, 40, 24))
	_ = g.Add(later)

	got, ok := g.FindBySemanticKey(later.SemanticKey())
	if !ok || got.ID != "c" {
		t.Errorf("want the most recent match, got %v", got.ID)
	}
	if _, ok := g.FindBySemanticKey("nothing"); ok {
		t.Error("an unknown key should not match")
	}
}

// ── ids ───────────────────────────────────────────────────────────────────────

// Each CLI invocation is its own process; restarting the count would give two
// different actions the same name.
func TestIDsContinueFromWhatWasLoaded(t *testing.T) {
	g := NewMemory()
	_ = g.Add(node("action_7", "notepad", "File", rect(0, 0, 10, 10)))
	if got := g.NextID(); got != "action_8" {
		t.Errorf("NextID = %q, want action_8", got)
	}

	fresh := NewMemory()
	fresh.Seed(20)
	if got := fresh.NextID(); got != "action_21" {
		t.Errorf("after seeding, NextID = %q, want action_21", got)
	}
}

// ── helpers ──

func windowNode(id, app, placement string) ActionNode {
	n := node(id, app, "", directorapi.Rect{})
	n.Goal = "move the window " + placement
	n.ResolvedTarget.Role = ""
	n.ResolvedTarget.Label = ""
	n.Plan.Steps[0].Action = ActionSpec{
		Type:      directorapi.ActionMoveWindow,
		Window:    &WindowSpec{Application: app, Title: "Untitled", Handle: "hwnd:1"},
		Placement: &PlacementSpec{Named: placement},
	}
	return n
}

func idPtr(s string) *NodeID {
	id := NodeID(s)
	return &id
}

func ids(nodes []ActionNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = string(n.ID)
	}
	return out
}
