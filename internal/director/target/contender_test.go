package target_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The contender contract.
//
//	AMBIGUOUS is not merely a score verdict. It is a PROMISE that the Director has
//	at least two concrete, safe, ordered choices it can present to the user.
//
// These tests exist because that promise was once breakable: the resolver reported
// AMBIGUOUS with a contender COUNT, the clarification layer re-derived viability from
// the full candidate list with its own predicate, and the two could disagree. When
// they did, the user got an empty question — a bare ":" — and the answer they typed
// was swallowed as a new request.

func world(labels ...string) directorapi.WorldState {
	w := directorapi.WorldState{
		Elements: map[directorapi.ElementID]*directorapi.Element{},
		Windows: []directorapi.Window{{
			ID: "hwnd:1", Application: "notepad", Title: "Untitled - Notepad", Focused: true,
		}},
		Confidence: directorapi.WorldConfidence{
			ObservationQuality: 1, Coverage: 1, Actionability: 1, Freshness: 1,
		},
	}
	for i, l := range labels {
		id := directorapi.ElementID("e" + string(rune('1'+i)))
		w.Elements[id] = &directorapi.Element{
			ID: id, Label: l, Role: directorapi.RoleMenuItem, WindowID: "hwnd:1",
			Enabled: true, Visible: true,
			Bounds: directorapi.Rect{X: 10, Y: 20 * (i + 1), Width: 100, Height: 18},
		}
	}
	return w
}

func TestAmbiguityCarriesAtLeastTwoOrderedContenders(t *testing.T) {
	w := world("New Tab", "New Window")
	res := target.NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "new"})

	if res.Status != directorapi.ResolutionAmbiguous {
		t.Fatalf("status = %s, want ambiguous (%s)", res.Status, res.Explanation)
	}
	if err := res.Consistent(); err != nil {
		t.Fatalf("the resolution breaks its own promise: %v", err)
	}
	if got := res.ContenderCount(); got < directorapi.MinContenders {
		t.Fatalf("contenders = %d, want at least %d", got, directorapi.MinContenders)
	}
}

func TestRejectedCandidatesNeverReachTheContenderList(t *testing.T) {
	// A disabled control is part of the EXPLANATION, never part of the choice:
	// offering it invites the user to pick something that cannot be acted on.
	w := world("New Tab", "New Window")
	w.Elements["e2"].Enabled = false

	res := target.NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "new"})
	for _, c := range res.Contenders {
		if c.Rejected != "" {
			t.Fatalf("a rejected candidate (%s: %s) was offered as a choice", c.Label, c.Rejected)
		}
		if c.Score <= 0 {
			t.Fatalf("a zero-scoring candidate (%s) was offered as a choice", c.Label)
		}
	}
	if err := res.Consistent(); err != nil {
		t.Fatalf("inconsistent: %v", err)
	}
}

func TestContendersAreAPrefixOfTheRankedCandidates(t *testing.T) {
	// "The second one" must mean the same thing to the user and to the resolver, which
	// only holds if the offered list is a PREFIX — never a re-ordering, never a
	// de-duplication.
	w := world("New Tab", "New Window", "New File")
	res := target.NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "new"})
	if res.Status != directorapi.ResolutionAmbiguous {
		t.Skipf("not ambiguous: %s", res.Explanation)
	}
	for i, c := range res.Contenders {
		if res.Candidates[i].ElementID != c.ElementID {
			t.Fatalf("contender %d is %s but candidate %d is %s — the offer was re-ordered",
				i+1, c.ElementID, i+1, res.Candidates[i].ElementID)
		}
	}
}

func TestTheResolverCannotEmitAmbiguousWithTooFewContenders(t *testing.T) {
	// The invariant, checked across a spread of worlds rather than one hand-picked
	// case. Any AMBIGUOUS the resolver can produce must be answerable.
	r := target.NewResolver()
	for _, labels := range [][]string{
		{"Save"},
		{"Save", "Save As"},
		{"New Tab", "New Window", "New File"},
		{"a", "b", "c", "d", "e", "f"},
		{"Save", "Save", "Save"},
		{},
	} {
		w := world(labels...)
		for _, q := range []string{"save", "new", "a", "nothing like this"} {
			res := r.Resolve(&w, directorapi.ElementQuery{Label: q})
			if err := res.Consistent(); err != nil {
				t.Fatalf("labels=%v query=%q: %v", labels, q, err)
			}
		}
	}
}

func TestASingleViableMatchResolvesRatherThanAsking(t *testing.T) {
	// One choice is not ambiguity. Reporting it as such would ask a question with a
	// single option, which is not a question.
	w := world("Save", "Print")
	res := target.NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "save"})
	if res.Status != directorapi.ResolutionResolved {
		t.Fatalf("status = %s, want resolved (%s)", res.Status, res.Explanation)
	}
	if err := res.Consistent(); err != nil {
		t.Fatalf("inconsistent: %v", err)
	}
}

func TestAbsenceOffersNothing(t *testing.T) {
	w := world("Save", "Print")
	res := target.NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "reticulate"})
	if res.Status == directorapi.ResolutionAmbiguous {
		t.Fatalf("a missing target was reported as ambiguous")
	}
	if len(res.Contenders) != 0 {
		t.Fatalf("a non-match offered %d contenders", len(res.Contenders))
	}
	if err := res.Consistent(); err != nil {
		t.Fatalf("inconsistent: %v", err)
	}
}
