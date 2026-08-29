package semanticmemory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// A LEARNING EVENT MUST MEAN SOMETHING WAS LEARNED.
//
// The whole value of a feed that says "I learned this" is that a person can turn round and try it.
// One event for something that was not written, or was already known, and they stop believing the
// feed — and a feed nobody believes is worse than no feed, because it looked like a promise.
//
// So the announcement is made by the code that did the writing, after the write succeeded.

func openStore(t *testing.T) (*semanticmemory.Store, *[]semanticmemory.Learning) {
	t.Helper()
	store, why := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	if why != "" {
		t.Fatalf("opening: %s", why)
	}
	var seen []semanticmemory.Learning
	store.WhenLearned(func(e semanticmemory.Learning) { seen = append(seen, e) })
	return store, &seen
}

func aPlace(term observe.InterfaceTerm) observe.StructureSignature {
	return observe.StructureSignature{
		Subject: observe.SubjectState, TermsKnown: true,
		Terms: []observe.InterfaceTerm{term},
		Roles: map[string]int{"button": 4, "list_item": 7},
	}
}

// Establishing the same Place twice announces once.
//
// `EstablishPlace` is idempotent by signature: the second call returns the existing id and writes
// nothing. Announcing there would fire every time somebody walked a route they had already taught.
func TestRecognisingAPlaceAgainIsNotLearningIt(t *testing.T) {
	store, seen := openStore(t)
	first, err := store.EstablishPlace("settings", aPlace(observe.TermControls))
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}
	again, err := store.EstablishPlace("settings", aPlace(observe.TermControls))
	if err != nil {
		t.Fatalf("establishing again: %v", err)
	}
	if first != again {
		t.Fatalf("the same signature produced two subjects, %s and %s", first, again)
	}
	if len(*seen) != 1 {
		t.Fatalf("%d learning events for one Place established twice: %v", len(*seen), *seen)
	}
	if (*seen)[0].Change != semanticmemory.Learned || (*seen)[0].Subject != first {
		t.Errorf("the event does not describe the Place that was written: %+v", (*seen)[0])
	}
}

// A write that could not be saved is not learning.
//
// The feed's one unbreakable rule. A Place refused at a bound, a store that has gone unavailable,
// a disk that will not take it — all ordinary — must reach a person as silence.
func TestAWriteThatFailedAnnouncesNothing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	store, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("opening: %s", why)
	}
	var seen []semanticmemory.Learning
	store.WhenLearned(func(e semanticmemory.Learning) { seen = append(seen, e) })

	// Make the save fail: the store writes `path.tmp` and renames it, so a DIRECTORY where
	// the file belongs refuses the rename without touching anything else.
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Skipf("cannot stage an unwritable store on this platform: %v", err)
	}
	if _, err := store.EstablishPlace("settings", aPlace(observe.TermAudio)); err == nil {
		t.Skip("this platform accepted the write; the failure path cannot be staged here")
	}
	if len(seen) != 0 {
		t.Errorf("a Place that could not be written was announced as learned: %v\n"+
			"The feed is a claim about what Marco KNOWS, and the only thing that makes it "+
			"believable is that it follows the commit.", seen)
	}
}

// A new way between two screens is learned; the same way again is strengthened.
//
// Two words because they are two facts, and a feed that said "learned" for both would train a
// person to stop reading it.
func TestAKnownWayIsStrengthenedRatherThanLearnedAgain(t *testing.T) {
	store, seen := openStore(t)
	from, err := store.EstablishPlace("settings", aPlace(observe.TermControls))
	if err != nil {
		t.Fatal(err)
	}
	to, err := store.EstablishPlace("settings", aPlace(observe.TermAudio))
	if err != nil {
		t.Fatal(err)
	}
	edge := []observe.RelationshipObservation{{
		From: from, To: to,
		Evidence: observe.RelationshipEvidence{Observations: 1, Unattributed: 1},
	}}
	if _, err := store.RememberRelationships("settings", edge); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RememberRelationships("settings", edge); err != nil {
		t.Fatal(err)
	}

	var learned, strengthened int
	for _, e := range *seen {
		if e.Kind != semanticmemory.KindEdge {
			continue
		}
		if e.From != from || e.To != to {
			t.Errorf("an edge event names %s → %s, want %s → %s", e.From, e.To, from, to)
		}
		switch e.Change {
		case semanticmemory.Learned:
			learned++
		case semanticmemory.Strengthened:
			strengthened++
		}
	}
	if learned != 1 || strengthened != 1 {
		t.Errorf("one new way seen twice produced learned=%d strengthened=%d, want 1 and 1",
			learned, strengthened)
	}
}

// A destination bound to a word is announced, and rebinding it says so.
func TestBindingAWordToAPlaceIsAnnouncedAndRebindingSaysSo(t *testing.T) {
	store, seen := openStore(t)
	mouse, err := store.EstablishPlace("settings", aPlace(observe.TermControls))
	if err != nil {
		t.Fatal(err)
	}
	display, err := store.EstablishPlace("settings", aPlace(observe.TermDisplay))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RememberGoal("settings",
		observe.Goal{Name: "mouse", Subject: mouse}); err != nil {
		t.Fatal(err)
	}
	if err := store.RememberGoal("settings",
		observe.Goal{Name: "mouse", Subject: display}); err != nil {
		t.Fatal(err)
	}
	var changes []semanticmemory.LearningChange
	for _, e := range *seen {
		if e.Kind == semanticmemory.KindGoal {
			changes = append(changes, e.Change)
		}
	}
	if len(changes) != 2 || changes[0] != semanticmemory.Learned ||
		changes[1] != semanticmemory.Rebound {
		t.Errorf("binding then rebinding a word produced %v, want [learned rebound].\n"+
			"A word that now means somewhere else is exactly what a person needs told.",
			changes)
	}
}
