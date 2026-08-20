package actiongraph_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The action graph's half of the semantic action milestone:
//
//	Persist semantic intent. Replay should re-resolve and choose the best
//	lowering again.
//
// What must NOT survive is as important as what must. A node that recorded the chosen
// mechanism would replay a decision made against a control that has since changed —
// clicking a disclosure arrow in an application that now exposes ExpandCollapse.

func semanticAction() directorapi.SemanticAction {
	return directorapi.SemanticAction{
		Kind: directorapi.SemanticExpand,
		Target: directorapi.ElementReference{
			// The id is what was true at the time; the query is what survives.
			ID:          "e246",
			Query:       &directorapi.ElementQuery{Label: "Explorer", Role: directorapi.RoleTreeItem},
			Description: "the Explorer folder",
		},
	}
}

func TestASemanticActionIsStoredAsItsVerbAndItsQuery(t *testing.T) {
	spec := actiongraph.SpecOf(semanticAction())

	if spec.Type != directorapi.ActionSemantic {
		t.Fatalf("type = %s, want %s", spec.Type, directorapi.ActionSemantic)
	}
	if spec.SemanticKind != directorapi.SemanticExpand {
		t.Errorf("kind = %q, want expand — the VERB is what the node is about",
			spec.SemanticKind)
	}
	if spec.Query == nil || spec.Query.Label != "Explorer" {
		t.Fatalf("query = %+v, want the query that found the target", spec.Query)
	}
}

// TestNoMechanismAndNoCoordinateIsStored is the rule, checked over the whole
// serialised node rather than field by field — a new field carrying the mechanism
// would slip past a targeted assertion.
func TestNoMechanismAndNoCoordinateIsStored(t *testing.T) {
	raw, err := json.Marshal(actiongraph.SpecOf(semanticAction()))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	stored := strings.ToLower(string(raw))

	for _, forbidden := range []string{
		"accessibility_pattern", "accessibility_invoke", "expandcollapse",
		"mechanism", "click", "chord", "point", "\"x\"", "\"y\"",
	} {
		if strings.Contains(stored, forbidden) {
			t.Errorf("the stored action carries %q:\n%s\n\n"+
				"A node records WHAT was meant. How it was carried out was chosen from "+
				"the control's capabilities at the time, and a replay must choose again.",
				forbidden, raw)
		}
	}
}

func TestReplayRebuildsTheVerbAndReResolvesTheTarget(t *testing.T) {
	spec := actiongraph.SpecOf(semanticAction())
	rebuilt, err := spec.Rebuild()
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	act, ok := rebuilt.(directorapi.SemanticAction)
	if !ok {
		t.Fatalf("rebuilt a %T, want a SemanticAction", rebuilt)
	}
	if act.Kind != directorapi.SemanticExpand {
		t.Errorf("kind = %s, want expand", act.Kind)
	}
	if act.Target.Query == nil || act.Target.Query.Label != "Explorer" {
		t.Errorf("the rebuilt action lost its query: %+v", act.Target)
	}
	// The element id must NOT come back. Replay re-resolves; an id from a previous
	// snapshot either names nothing or names something else.
	if act.Target.ID != "" {
		t.Errorf("the rebuilt action carries element id %q — a replay must re-resolve",
			act.Target.ID)
	}
}

func TestEveryVerbSurvivesAStoreAndRebuildRoundTrip(t *testing.T) {
	for _, kind := range directorapi.SemanticVocabulary {
		t.Run(string(kind), func(t *testing.T) {
			action := directorapi.SemanticAction{Kind: kind}
			if kind.NeedsTarget() {
				action.Target = directorapi.ElementReference{
					Query: &directorapi.ElementQuery{Label: "Thing"}, Description: "the thing",
				}
			}
			if kind.WindowLevel() {
				action.Window = &directorapi.WindowReference{
					Application: "notepad", TitleContains: "Untitled",
				}
			}

			raw, err := json.Marshal(actiongraph.SpecOf(action))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var spec actiongraph.ActionSpec
			if err := json.Unmarshal(raw, &spec); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			rebuilt, err := spec.Rebuild()
			if err != nil {
				t.Fatalf("rebuild after a round trip: %v", err)
			}
			got, ok := rebuilt.(directorapi.SemanticAction)
			if !ok {
				t.Fatalf("rebuilt a %T", rebuilt)
			}
			if got.Kind != kind {
				t.Fatalf("kind = %s, want %s", got.Kind, kind)
			}
		})
	}
}

// TestAStoredVerbThisBuildDoesNotKnowIsRefused: a graph file outlives the build that
// wrote it, and a verb removed from the vocabulary must not silently become something
// else at replay time.
func TestAStoredVerbThisBuildDoesNotKnowIsRefused(t *testing.T) {
	spec := actiongraph.ActionSpec{
		Type: directorapi.ActionSemantic, SemanticKind: "teleport",
		Query: &directorapi.ElementQuery{Label: "Thing"},
	}
	if _, err := spec.Rebuild(); err == nil {
		t.Fatal("an unknown stored verb rebuilt into something runnable")
	}
}

func TestAStoredTargetedVerbWithNoQueryIsRefused(t *testing.T) {
	spec := actiongraph.ActionSpec{
		Type: directorapi.ActionSemantic, SemanticKind: directorapi.SemanticExpand,
	}
	if _, err := spec.Rebuild(); err == nil {
		t.Fatal("an expand with nothing to re-resolve rebuilt anyway")
	}
}
