package observesession_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// A durable habit BETWEEN two places inside one application.
//
// A relationship is the first thing in this system that survives a restart, and both its ends
// resolve through `Recall(application, SignatureOf(hypothesis))`. If that derivation could not
// separate two places inside one surface, the edge would either fail to resolve or resolve into
// a loop — "from this screen, press confirm, and arrive at this screen" — which is worse than no
// edge at all, because it reads as knowledge.
//
// Nothing here constructs a RelationshipObservation. Every edge below is produced by a session
// observing a surface change and resolving its own endpoints against the real store.

// runWith runs a session against durable memory, which is what makes an edge durable.
func runWith(t *testing.T, s script, m observe.Memory) observesession.Result {
	t.Helper()
	got, err := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&scripted{s: s}, &recordingEvents{}).WithMemory(m).
		Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return got
}

// placeIn is one state of the shared surface, and what a text pass read on it.
type placeIn struct {
	surface screenfixture.Surface
	terms   []observe.InterfaceTerm
}

func placeA() placeIn {
	return placeIn{surface: oneSurface(),
		terms: []observe.InterfaceTerm{observe.TermControls, observe.TermSettings}}
}

func placeB() placeIn {
	return placeIn{surface: oneSurface().ContentReplaced("checkbox"),
		terms: []observe.InterfaceTerm{observe.TermAudio, observe.TermDisplay}}
}

func placeC() placeIn {
	return placeIn{surface: oneSurface().ContentReplaced("menu_item"),
		terms: []observe.InterfaceTerm{observe.TermQuit, observe.TermResume}}
}

// journeyScript is one visit to `from`, a confirm, and a stay on `to`, twice over.
//
// Twice because a composition seen once is a transition frame and a composition seen twice is a
// place — the promotion rule this whole model rests on.
func journeyScript(from, to placeIn) script {
	var frames []frame
	for range 2 {
		frames = append(frames, reading(stayOn(from.surface.Regions(), 5), from.terms...)...)
		frames = append(frames, reading(
			pressThen(to.surface.Regions(), 5, observe.NavConfirm), to.terms...)...)
	}
	return script{frames: frames}
}

// learn observes one journey and stores whatever places it found, so a later session can resolve
// its own endpoints against them.
func learn(t *testing.T, store *semanticmemory.Store, s script) {
	t.Helper()
	for _, h := range run(t, s).Hypotheses {
		if h.Subject.Kind != observe.SubjectState {
			continue
		}
		sig := observe.SignatureOf(h)
		if !sig.Discriminating() {
			continue
		}
		_ = store.Remember("testgame", sig, observe.SemanticKnowledge{
			Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeConfirmed,
		})
	}
}

func newStore(t *testing.T) *semanticmemory.Store {
	t.Helper()
	s, _ := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	return s
}

// R (relationships). A→B inside one surface becomes a durable directed edge.
func TestAHabitBetweenTwoPlacesInOneSurfaceBecomesDurable(t *testing.T) {
	store := newStore(t)
	journey := journeyScript(placeA(), placeB())
	learn(t, store, journey)
	if n := len(store.Subjects()); n != 2 {
		t.Fatalf("the two places were stored as %d subject(s)", n)
	}

	got := runWith(t, journey, store)
	rels := store.Relationships()
	if len(rels) == 0 {
		t.Fatalf("a session that moved between two places inside one application left no "+
			"durable edge (%d session-local, %q)",
			got.Relationships.SessionLocal, got.Relationships.Unavailable)
	}
	for _, r := range rels {
		if r.From == r.To {
			t.Fatalf("the edge is a loop: %s → %s. Two places inside one surface collapsed "+
				"into one endpoint, so Marco believes pressing confirm leaves it where it "+
				"already was", r.From, r.To)
		}
	}
	t.Logf("%d durable edge(s) after one session", len(rels))
}

// Recurrence across sessions accumulates onto ONE edge, not onto a new one each time.
func TestTheSameHabitInAnotherSessionIsTheSameEdge(t *testing.T) {
	store := newStore(t)
	journey := journeyScript(placeA(), placeB())
	learn(t, store, journey)

	// The script goes there and comes back, so BOTH directions are habits. What must not
	// happen is a new edge appearing for the same pair on the second visit.
	runWith(t, journey, store)
	first := edgeSessions(store)
	runWith(t, journey, store)
	second := edgeSessions(store)

	if len(second) != len(first) {
		t.Fatalf("the same habit in a second session produced %d edges, up from %d; a "+
			"person doing the same thing twice would accumulate a new one every time",
			len(second), len(first))
	}
	for pair, sessions := range second {
		was, ok := first[pair]
		if !ok {
			t.Fatalf("a new edge %s appeared for a journey already seen", pair)
		}
		if sessions <= was {
			t.Errorf("edge %s did not count the second session: %d then %d",
				pair, was, sessions)
		}
	}
	t.Logf("%d edge(s), each seen in a second session: %v", len(second), second)
}

// Adversarial. A→B and A→C do not merge because B and C share a surface with each other.
//
// The failure this guards against is the plausible one: two destinations that look alike at the
// application level are exactly what a surface-keyed model would fuse, and the resulting single
// edge would say "confirm from here goes there" when it goes to one of two places.
func TestTwoDestinationsInOneSurfaceStayTwoEdges(t *testing.T) {
	store := newStore(t)
	toB, toC := journeyScript(placeA(), placeB()), journeyScript(placeA(), placeC())
	learn(t, store, toB)
	learn(t, store, toC)
	if n := len(store.Subjects()); n != 3 {
		t.Fatalf("three places inside one surface were stored as %d subject(s)", n)
	}

	runWith(t, toB, store)
	runWith(t, toC, store)

	rels := store.Relationships()
	dest := map[string]bool{}
	for _, r := range rels {
		dest[r.To] = true
	}
	if len(dest) < 2 {
		t.Fatalf("two different destinations inside one surface produced %d edge(s) to %d "+
			"place(s); Marco would believe one habit that actually has two outcomes",
			len(rels), len(dest))
	}
	t.Logf("%d edge(s) to %d distinct destination(s)", len(rels), len(dest))
}

// edgeSessions is every durable edge, as "from→to" against how many sessions saw it.
func edgeSessions(store *semanticmemory.Store) map[string]int {
	out := map[string]int{}
	for _, r := range store.Relationships() {
		out[r.From+"→"+r.To] = r.Sessions
	}
	return out
}
