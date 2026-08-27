package ambient_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
)

// What Marco holds after watching, and why it does not grow.

// TEN THOUSAND SIGHTINGS OF ONE SCREEN ARE ONE ENTRY.
//
// # The property the ambient product rests on
//
// Somebody leaves Marco watching for eight hours on one page. The desktop is read thousands of
// times and nothing new happens. What Marco holds afterwards must be about the size of what it
// held after five minutes — otherwise memory tracks how long somebody watched rather than what
// they did, and "always watching" becomes "always growing".
//
// Deleting the tally — appending instead of counting — must fail this.
func TestWatchingLongerDoesNotMeanRememberingMore(t *testing.T) {
	b := ambient.New()
	start := time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC)

	for i := 0; i < 10_000; i++ {
		b.Saw("settings", "subj_home", start.Add(time.Duration(i)*time.Second))
	}
	places, edges, recent := b.Size()
	if places != 1 {
		t.Fatalf("%d places after ten thousand sightings of one screen. Memory is tracking "+
			"observation time rather than semantic novelty, which is what makes an "+
			"always-on observer unacceptable.", places)
	}
	if edges != 0 || recent != 0 {
		t.Errorf("standing still produced %d edge(s) and %d move(s)", edges, recent)
	}

	// AND THE COUNT IS THE FACT. Collapsing must not lose how often.
	view := b.Look()
	if view.Places[0].Seen != 10_000 {
		t.Errorf("it says %d sightings, want ten thousand — the tally is what makes "+
			"collapsing honest rather than lossy", view.Places[0].Seen)
	}
	if !view.Places[0].First.Equal(start) {
		t.Errorf("the first sighting reads %v", view.Places[0].First)
	}
	if view.Places[0].Last.Equal(start) {
		t.Error("the last sighting was never updated, so the buffer cannot tell a screen " +
			"somebody is on now from one they left this morning")
	}
}

// AND A ROUTE WALKED REPEATEDLY IS ONE EDGE.
func TestARouteWalkedRepeatedlyIsOneEdge(t *testing.T) {
	b := ambient.New()
	at := time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 500; i++ {
		b.Moved("settings", "subj_home", "subj_bt", ambient.ByHuman,
			at.Add(time.Duration(i)*time.Minute))
	}
	_, edges, _ := b.Size()
	if edges != 1 {
		t.Fatalf("%d edges for one route walked five hundred times", edges)
	}
	if got := b.Look().Edges[0].Seen; got != 500 {
		t.Errorf("it says %d walks, want 500", got)
	}
}

// GROWTH TRACKS NOVELTY, AND STOPS AT THE BOUND.
//
// The negative control for the test above: a buffer that never grew at all would pass it, and
// would also be useless. Distinct screens ARE remembered — up to a limit, past which the least
// recently seen is forgotten and the forgetting is counted rather than silent.
func TestDistinctScreensAreRememberedUpToTheBound(t *testing.T) {
	b := ambient.New()
	at := time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		b.Saw("settings", fmt.Sprintf("subj_%d", i), at.Add(time.Duration(i)*time.Second))
	}
	if places, _, _ := b.Size(); places != 10 {
		t.Fatalf("%d places after ten different screens; novelty must still be remembered",
			places)
	}

	for i := 0; i < ambient.MaxPlaces*2; i++ {
		b.Saw("settings", fmt.Sprintf("subj_many_%d", i),
			at.Add(time.Duration(100+i)*time.Second))
	}
	places, _, _ := b.Size()
	if places > ambient.MaxPlaces {
		t.Fatalf("%d places held, bound is %d. Unbounded is not acceptable however novel "+
			"the desktop is.", places, ambient.MaxPlaces)
	}
	if b.Look().Dropped == 0 {
		t.Error("the bound discarded nothing and reported nothing; a full buffer must not " +
			"read as a quiet one")
	}
}

// THE RECENT WALK KEEPS THE NEWEST, NOT THE OLDEST.
//
// It answers one question — what just happened — and a tail that dropped its newest entries
// would answer it with the afternoon.
func TestTheRecentWalkKeepsWhatJustHappened(t *testing.T) {
	b := ambient.New()
	at := time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC)
	for i := 0; i < ambient.MaxMoves*3; i++ {
		b.Moved("settings", fmt.Sprintf("subj_%d", i), fmt.Sprintf("subj_%d", i+1),
			ambient.ByHuman, at.Add(time.Duration(i)*time.Second))
	}
	view := b.Look()
	if len(view.Recent) > ambient.MaxMoves {
		t.Fatalf("%d moves held, bound is %d", len(view.Recent), ambient.MaxMoves)
	}
	last := view.Recent[len(view.Recent)-1]
	want := fmt.Sprintf("subj_%d", ambient.MaxMoves*3-1)
	if last.From != want {
		t.Fatalf("the newest move is from %q, want %q. The tail dropped the wrong end and "+
			"now describes what happened a while ago.", last.From, want)
	}
}

// WHO DID IT IS KEPT, AND THE TWO ARE NOT INTERCHANGEABLE.
//
// A transition somebody walked and one Marco performed are different facts. A future promotion
// policy that confused them would learn Marco's own behaviour back from itself, and Activity
// would tell somebody they did something Marco did.
func TestHumanAndMarcoAreNotTheSameEvidence(t *testing.T) {
	b := ambient.New()
	at := time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC)
	b.Moved("settings", "subj_home", "subj_bt", ambient.ByHuman, at)
	b.Moved("settings", "subj_home", "subj_bt", ambient.ByMarco, at.Add(time.Minute))
	b.Moved("settings", "subj_home", "subj_bt", ambient.ByHuman, at.Add(2*time.Minute))

	e := b.Look().Edges[0]
	if e.Seen != 3 {
		t.Fatalf("%d walks recorded, want three", e.Seen)
	}
	if e.By[ambient.ByHuman] != 2 || e.By[ambient.ByMarco] != 1 {
		t.Fatalf("provenance reads %v. Keeping only the latest would make one route's "+
			"evidence indistinguishable from the other's.", e.By)
	}
}

// NOTHING IN HERE NAMES ANYTHING.
//
// A transient buffer with weaker rules than the durable store would be exactly where a privacy
// boundary quietly stops applying. Ids, counts, times and a closed provenance word — no labels,
// no titles, no screen text, no coordinates.
func TestTheBufferHoldsNoWordsAnybodyCouldRead(t *testing.T) {
	b := ambient.New()
	at := time.Now()
	b.Saw("settings", "subj_home", at)
	b.Moved("settings", "subj_home", "subj_bt", ambient.ByHuman, at)

	view := b.Look()
	rendered := fmt.Sprintf("%+v", view)
	for _, leak := range []string{"Label", "Title", "Text", "Bounds", "Region", "Screenshot"} {
		if contains(rendered, leak) {
			t.Errorf("the buffer carries %q: %s", leak, rendered)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	}()
}
