package search

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/wait/conditions"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Looking for something that is not in front of Marco.
//
// Every case here is NON-ACTUATING: the worlds are written down, the moves are counted, and
// nothing touches a desktop. That is the point of the shape — the searcher takes an Observer and
// a Performer, so the loop can be driven through every outcome it can reach without a screen.

// ── the surface under test ───────────────────────────────────────────────────

// scrollMove is the one looking move the gate admits today.
func scrollMove(where directorapi.ScrollWhere, phrase string) Move {
	return Move{
		Action: directorapi.SemanticAction{
			Kind:   directorapi.SemanticScroll,
			Scroll: where,
		},
		Phrase: phrase,
	}
}

func scrollPattern(budget int) Pattern {
	return Pattern{
		Step:     scrollMove(directorapi.ScrollPageDown, "scroll the list down one page"),
		Home:     scrollMove(directorapi.ScrollTop, "scroll the list back to the top"),
		Budget:   budget,
		Describe: "scrolling the list",
	}
}

// ── worlds, written down ─────────────────────────────────────────────────────

func row(id, label string, y int) *directorapi.Element {
	return &directorapi.Element{
		ID: directorapi.ElementID(id), Label: label, Role: directorapi.RoleListItem,
		WindowID: "hwnd:1", Enabled: true, Visible: true, Confidence: 1,
		Bounds: directorapi.Rect{X: 0, Y: y, Width: 200, Height: 20},
	}
}

// worldOf is a readable world holding the named rows. Confidence is high so nothing here is
// measuring perception quality.
func worldOf(labels ...string) directorapi.WorldState {
	m := map[directorapi.ElementID]*directorapi.Element{}
	for i, l := range labels {
		id := "e" + strings.ToLower(strings.ReplaceAll(l, " ", "-"))
		m[directorapi.ElementID(id)] = row(id, l, 20*i)
	}
	return directorapi.WorldState{
		Elements: m,
		Windows:  []directorapi.Window{{ID: "hwnd:1", Application: "fixture", Focused: true}},
		Confidence: directorapi.WorldConfidence{
			ObservationQuality: 1, Coverage: 1, Actionability: 1, Freshness: 1,
		},
	}
}

// unreadable is a world with nothing in it. The condition's Unknown gate reads this as "could
// not look", never as "the thing is absent" — see ElementQuery.find.
func unreadable() directorapi.WorldState {
	return directorapi.WorldState{
		Windows: []directorapi.Window{{ID: "hwnd:1", Application: "fixture"}},
	}
}

// rig drives the loop from a written-down sequence of places.
//
// Each place is read as many times as the searcher asks (a thin reading gets read twice), and a
// move advances to the next place. So the test says where the thing is, and the searcher's
// movement decides whether it gets there.
type rig struct {
	places  []directorapi.WorldState
	at      int
	reads   int
	moves   []string
	moveErr error
}

func (r *rig) observe(context.Context) (directorapi.WorldState, error) {
	r.reads++
	if r.at >= len(r.places) {
		return r.places[len(r.places)-1], nil
	}
	return r.places[r.at], nil
}

func (r *rig) perform(_ context.Context, m Move) error {
	r.moves = append(r.moves, m.Phrase)
	if r.moveErr != nil {
		return r.moveErr
	}
	// Home goes back to the start; a step goes on one place.
	if sem, ok := m.Action.(directorapi.SemanticAction); ok && sem.Scroll == directorapi.ScrollTop {
		r.at = 0
		return nil
	}
	if r.at < len(r.places)-1 {
		r.at++
	}
	return nil
}

func (r *rig) searcher() *Searcher { return New(r.observe, r.perform) }

func looking(label string) conditions.WorldCondition {
	return conditions.ElementExists{Query: conditions.ElementQuery{Label: label}}
}

// ── the four outcomes ────────────────────────────────────────────────────────

// TestWhatIsAlreadyOnScreenCostsNoMoves is the first look.
//
// A search that moved before looking would scroll past a thing that was in front of it, and
// would charge somebody's screen for the privilege.
func TestWhatIsAlreadyOnScreenCostsNoMoves(t *testing.T) {
	r := &rig{places: []directorapi.WorldState{worldOf("milk", "eggs", "bread")}}

	got, err := r.searcher().Find(context.Background(), looking("eggs"), scrollPattern(5))
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Outcome != Found {
		t.Fatalf("outcome = %s (%s)", got.Outcome, got.Reason)
	}
	if got.Moved != 0 {
		t.Errorf("moved %d time(s) to find something already on screen: %v", got.Moved, r.moves)
	}
	if got.Looked != 1 {
		t.Errorf("looked in %d places, want 1", got.Looked)
	}
}

// TestSomethingThreePlacesAwayIsFound is the capability.
func TestSomethingThreePlacesAwayIsFound(t *testing.T) {
	r := &rig{places: []directorapi.WorldState{
		worldOf("alpha", "beta"),
		worldOf("gamma", "delta"),
		worldOf("epsilon", "zeta"),
		worldOf("milk", "eggs"),
	}}

	got, err := r.searcher().Find(context.Background(), looking("milk"), scrollPattern(5))
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Outcome != Found {
		t.Fatalf("outcome = %s (%s); the thing was three moves away", got.Outcome, got.Reason)
	}
	if got.Moved != 3 {
		t.Errorf("moved %d time(s), want 3: %v", got.Moved, r.moves)
	}
	// AND THE WORLD IT WAS SEEN IN comes back. The caller resolves against the reading the
	// thing was in, not a fresher one that may have scrolled on.
	if got.World == nil {
		t.Fatal("no world came back from a successful search; the caller has nothing to resolve against")
	}
	if _, ok := got.World.Elements["emilk"]; !ok {
		t.Error("the world that came back does not contain what was found")
	}
}

// TestTheViewIsNotPutBackOnSuccess — the next step is about to act on the thing.
func TestTheViewIsNotPutBackOnSuccess(t *testing.T) {
	r := &rig{places: []directorapi.WorldState{worldOf("alpha"), worldOf("milk")}}

	got, _ := r.searcher().Find(context.Background(), looking("milk"), scrollPattern(5))
	if got.Restored {
		t.Error("the view was scrolled away from what was just found")
	}
	for _, m := range r.moves {
		if strings.Contains(m, "back to the top") {
			t.Errorf("a successful search performed %q", m)
		}
	}
}

// TestLookingEverywhereAndNotFindingItIsExhausted is the outcome the package exists for.
//
// "Not in this reading" is what a single failed look says. "Not anywhere I could get to" is a
// far stronger statement, and it is the only one that licenses telling somebody their text is
// not in the document.
func TestLookingEverywhereAndNotFindingItIsExhausted(t *testing.T) {
	r := &rig{places: []directorapi.WorldState{
		worldOf("alpha"), worldOf("beta"), worldOf("gamma"),
	}}

	got, err := r.searcher().Find(context.Background(), looking("milk"), scrollPattern(5))
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Outcome != Exhausted {
		t.Fatalf("outcome = %s (%s); every place was readable and none held it",
			got.Outcome, got.Reason)
	}
	if got.Read != got.Looked {
		t.Errorf("read %d of %d places; exhausted claims every one was read", got.Read, got.Looked)
	}
	if !strings.Contains(got.Reason, "milk") {
		t.Errorf("the reason does not name what was looked for: %q", got.Reason)
	}
}

// TestAPlaceThatCouldNotBeReadIsNotAPlaceThatLacksIt is the sharpest rule here.
//
// One place that could not be read is one place the thing could have been, so the search has not
// established absence. It established that it did not SEE it, which is a different sentence and
// has to be a different word. [[ADR-006-unknown-is-not-false]].
func TestAPlaceThatCouldNotBeReadIsNotAPlaceThatLacksIt(t *testing.T) {
	r := &rig{places: []directorapi.WorldState{
		worldOf("alpha"),
		unreadable(), // and it stays unreadable on the second look too
		unreadable(),
		worldOf("gamma"),
	}}

	got, err := r.searcher().Find(context.Background(), looking("milk"), scrollPattern(5))
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Outcome != Unreadable {
		t.Fatalf("outcome = %s (%s); a place Marco could not read was counted as a place "+
			"the thing is not in", got.Outcome, got.Reason)
	}
	if got.Read >= got.Looked {
		t.Errorf("read %d of %d, which is not consistent with Unreadable", got.Read, got.Looked)
	}
	if !strings.Contains(got.Reason, "could not be read") {
		t.Errorf("the reason does not say a place could not be read: %q", got.Reason)
	}
}

// TestAThinReadingIsReadAgainBeforeItCostsTheSearchItsAnswer
//
// Free, because looking has no effect — the same argument [[ADR-137-a-thin-reading-is-not-an-answer]]
// makes upstream. It matters MORE here: at the measured rate of two thin readings in twelve, a
// five-place search that believed the first reading would almost never be able to say a thing is
// absent, and `exhausted` is the whole point.
func TestAThinReadingIsReadAgainBeforeItCostsTheSearchItsAnswer(t *testing.T) {
	// The rig serves the place at r.at on every read, so a place that is thin ONCE needs the
	// reads themselves to differ. This observer gives a thin first reading of place 0 and a
	// good one after.
	var reads int
	places := []directorapi.WorldState{worldOf("alpha"), worldOf("milk")}
	at := 0
	observe := func(context.Context) (directorapi.WorldState, error) {
		reads++
		if reads == 1 {
			return unreadable(), nil
		}
		return places[at], nil
	}
	var moves int
	perform := func(_ context.Context, m Move) error {
		moves++
		if at < len(places)-1 {
			at++
		}
		return nil
	}

	got, err := New(observe, perform).Find(context.Background(), looking("milk"), scrollPattern(5))
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Outcome != Found {
		t.Fatalf("outcome = %s (%s)", got.Outcome, got.Reason)
	}
	if !got.Trail[0].Relooked {
		t.Error("the thin first reading was believed; nothing recorded a second look")
	}
	if got.Read != got.Looked {
		t.Errorf("read %d of %d; the re-look should have made the first place readable",
			got.Read, got.Looked)
	}
}

// ── the bounds ───────────────────────────────────────────────────────────────

// TestTheBudgetIsTheBound — a search stops looking when it says it will.
func TestTheBudgetIsTheBound(t *testing.T) {
	// Twenty distinct places, a budget of three.
	var places []directorapi.WorldState
	for _, l := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		places = append(places, worldOf(l))
	}
	r := &rig{places: places}

	got, _ := r.searcher().Find(context.Background(), looking("milk"), scrollPattern(3))
	if got.Moved != 3 {
		t.Errorf("moved %d times against a budget of 3", got.Moved)
	}
	if got.Looked != 4 {
		t.Errorf("looked in %d places; three moves from a starting place is four", got.Looked)
	}
	if got.Outcome != Exhausted {
		t.Errorf("outcome = %s (%s)", got.Outcome, got.Reason)
	}
}

// TestAnUnboundedSearchIsRefused — the Director does not get to decide how much of somebody's
// afternoon to spend looking.
func TestAnUnboundedSearchIsRefused(t *testing.T) {
	r := &rig{places: []directorapi.WorldState{worldOf("alpha")}}
	p := scrollPattern(5)
	p.Budget = 0

	if _, err := r.searcher().Find(context.Background(), looking("milk"), p); err == nil {
		t.Error("a search with no budget was accepted")
	}
	if len(r.moves) != 0 {
		t.Errorf("a refused search performed %v", r.moves)
	}
}

// TestAViewThatStoppedMovingIsNotMorePlaces is [[ADR-007-no-progress-no-repetition]] in a third
// disguise.
//
// A region already at its bottom accepts a page-down and reports success forever. Without this
// the search spends its whole budget re-reading one screen and then claims to have looked
// everywhere — which turns an honest `exhausted` into a lie told confidently.
func TestAViewThatStoppedMovingIsNotMorePlaces(t *testing.T) {
	// Two real places, then it is at the bottom: every further move shows the same thing.
	r := &rig{places: []directorapi.WorldState{worldOf("alpha"), worldOf("beta")}}

	got, _ := r.searcher().Find(context.Background(), looking("milk"), scrollPattern(10))
	if got.Moved > 2 {
		t.Errorf("moved %d times against a view that stopped changing after 1: %v",
			got.Moved, r.moves)
	}
	if got.Outcome != Exhausted {
		t.Errorf("outcome = %s (%s)", got.Outcome, got.Reason)
	}
	if !strings.Contains(got.Reason, "nothing new") {
		t.Errorf("the reason does not say the view stopped showing anything new: %q", got.Reason)
	}
}

// TestTheViewIsPutBackWhenNothingWasFound
//
// A search that found nothing has still moved somebody's screen. Leaving it at the bottom of a
// list with nothing to show for it is the Director changing what a person is looking at as a
// side effect of being wrong.
func TestTheViewIsPutBackWhenNothingWasFound(t *testing.T) {
	r := &rig{places: []directorapi.WorldState{worldOf("alpha"), worldOf("beta"), worldOf("gamma")}}

	got, _ := r.searcher().Find(context.Background(), looking("milk"), scrollPattern(5))
	if !got.Restored {
		t.Fatalf("the view was not put back (%s)", got.Reason)
	}
	if r.at != 0 {
		t.Errorf("the rig is at place %d, not back at the start", r.at)
	}
	if !strings.Contains(got.Reason, "put back") {
		t.Errorf("the reason does not say the view was put back: %q", got.Reason)
	}
}

// TestAPatternThatCannotRestoreSaysSo — the sentence must not imply nothing happened.
func TestAPatternThatCannotRestoreSaysSo(t *testing.T) {
	r := &rig{places: []directorapi.WorldState{worldOf("alpha"), worldOf("beta")}}
	p := scrollPattern(5)
	p.Home = Move{} // this pattern cannot put the view back

	got, _ := r.searcher().Find(context.Background(), looking("milk"), p)
	if got.Restored {
		t.Fatal("restored with no way to restore")
	}
	if strings.Contains(got.Reason, "put back") {
		t.Errorf("the reason claims the view was put back: %q", got.Reason)
	}
}

// TestAViewWithNowhereToGoCostsOneMove.
//
// A search cannot know a view is already at its end without moving it once — the accessibility
// layer reports that a region CAN scroll, never that it has anywhere left to go. So one move is
// spent learning, and then the search stops rather than spending the other four.
//
// Written as an exact count because both neighbouring numbers are defects: zero would mean the
// search gave up without looking, and two would mean it did not notice the first one taught it
// everything.
func TestAViewWithNowhereToGoCostsOneMove(t *testing.T) {
	r := &rig{places: []directorapi.WorldState{worldOf("alpha")}}

	got, _ := r.searcher().Find(context.Background(), looking("milk"), scrollPattern(5))
	if got.Moved != 1 {
		t.Errorf("moved %d time(s) against a view with nowhere to go: %v", got.Moved, r.moves)
	}
	if got.Looked != 2 {
		t.Errorf("looked in %d places; the same screen twice is one place plus the proof "+
			"that there is no second", got.Looked)
	}
	if got.Outcome != Exhausted {
		t.Errorf("outcome = %s (%s)", got.Outcome, got.Reason)
	}
}

// TestNothingIsRestoredWhenTheSearchNeverMoved — a search that found what it wanted in the
// first reading has nothing to undo, and a stray scroll-to-top would move a view the person
// had set themselves.
func TestNothingIsRestoredWhenTheSearchNeverMoved(t *testing.T) {
	r := &rig{places: []directorapi.WorldState{worldOf("milk")}}

	got, _ := r.searcher().Find(context.Background(), looking("milk"), scrollPattern(5))
	if len(r.moves) != 0 {
		t.Errorf("a search satisfied by the first reading performed %v", r.moves)
	}
	if got.Restored {
		t.Error("restored without having moved")
	}
}

// ── the gate ─────────────────────────────────────────────────────────────────

// TestASearchWillNotClickToLookIsTheLoadBearingRule.
//
// A search performs moves WITHOUT BEING ASKED. That is only acceptable while the moves change
// what can be seen and commit nothing. A click is the whole category of thing a person has to
// ask for, and a search that could click its way around somebody's computer looking for a label
// is not a search — it is the Director operating software on a hunch.
func TestASearchWillNotClickToLookIsTheLoadBearingRule(t *testing.T) {
	for _, c := range []struct {
		what string
		a    directorapi.Action
	}{
		{"a click", directorapi.ClickAction{
			Target: directorapi.ElementReference{Description: "the Next button"}}},
		{"typing", directorapi.TypeAction{Text: "milk"}},
		{"an edit", directorapi.EditAction{Operation: "set_text", Text: "milk"}},
		{"a key press", directorapi.KeyAction{Chord: "down"}},
		{"bringing a control into view", directorapi.SemanticAction{
			Kind: directorapi.SemanticScrollHere}},
	} {
		t.Run(c.what, func(t *testing.T) {
			if MaySearchWith(c.a) {
				t.Fatalf("%s is admitted as a move a search may make unasked", c.what)
			}
			r := &rig{places: []directorapi.WorldState{worldOf("alpha")}}
			p := scrollPattern(5)
			p.Step = Move{Action: c.a, Phrase: c.what}

			_, err := r.searcher().Find(context.Background(), looking("milk"), p)
			if !errors.Is(err, ErrNotALookingMove) {
				t.Fatalf("a pattern moving by %s was accepted: err=%v", c.what, err)
			}
			if len(r.moves) != 0 {
				t.Errorf("the refused pattern still performed %v", r.moves)
			}
		})
	}
}

// TestScrollingIsTheOneAdmittedMove records the closed list as a list of one.
//
// It is written as an assertion about the whole list rather than about scrolling, so a second
// entry cannot arrive without this test being read.
func TestScrollingIsTheOneAdmittedMove(t *testing.T) {
	admitted := 0
	for _, where := range directorapi.ScrollWheres {
		if MaySearchWith(directorapi.SemanticAction{
			Kind: directorapi.SemanticScroll, Scroll: where}) {
			admitted++
		}
	}
	if admitted != len(directorapi.ScrollWheres) {
		t.Errorf("%d of %d scroll destinations are admitted; the gate is on the verb, not the "+
			"destination", admitted, len(directorapi.ScrollWheres))
	}
	// And nothing else in the semantic vocabulary is.
	for _, kind := range directorapi.SemanticVocabulary {
		if kind == directorapi.SemanticScroll {
			continue
		}
		if MaySearchWith(directorapi.SemanticAction{Kind: kind}) {
			t.Errorf("%s is admitted as a looking move. If that is intended, the argument "+
				"belongs in MaySearchWith and in an ADR — a search uses these without being "+
				"asked", kind)
		}
	}
}

// TestTheHomeMoveIsGatedToo — the way back is a move the search makes unasked as well.
func TestTheHomeMoveIsGatedToo(t *testing.T) {
	r := &rig{places: []directorapi.WorldState{worldOf("alpha")}}
	p := scrollPattern(5)
	p.Home = Move{Action: directorapi.ClickAction{
		Target: directorapi.ElementReference{Description: "Back"}}, Phrase: "click Back"}

	if _, err := r.searcher().Find(context.Background(), looking("milk"), p); !errors.Is(err, ErrNotALookingMove) {
		t.Fatalf("a pattern that restores by clicking was accepted: %v", err)
	}
}

// ── stopping ─────────────────────────────────────────────────────────────────

// TestStopEndsTheSearchAndPutsTheViewBack.
//
// "Stop" is not a request to leave somebody's screen scrolled somewhere else, so the restore
// runs even though the search was cancelled.
func TestStopEndsTheSearchAndPutsTheViewBack(t *testing.T) {
	r := &rig{places: []directorapi.WorldState{
		worldOf("alpha"), worldOf("beta"), worldOf("gamma"), worldOf("delta"),
	}}
	looks := 0
	s := New(func(ctx context.Context) (directorapi.WorldState, error) {
		looks++
		return r.observe(ctx)
	}, r.perform).WithStopCheck(func() bool { return looks >= 2 })

	got, err := s.Find(context.Background(), looking("milk"), scrollPattern(10))
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Outcome != Stopped {
		t.Fatalf("outcome = %s (%s)", got.Outcome, got.Reason)
	}
	if !got.Restored {
		t.Errorf("a stopped search left the view where it looked (%s)", got.Reason)
	}
}

// TestAStoppedSearchClaimsNothingAboutWhereTheThingIs — it is not absence and must not read as
// absence.
func TestAStoppedSearchClaimsNothingAboutWhereTheThingIs(t *testing.T) {
	r := &rig{places: []directorapi.WorldState{worldOf("alpha"), worldOf("beta")}}
	s := New(r.observe, r.perform).WithStopCheck(func() bool { return true })

	got, _ := s.Find(context.Background(), looking("milk"), scrollPattern(5))
	if got.Outcome == Exhausted {
		t.Error("a search that never ran claimed to have looked everywhere")
	}
	if strings.Contains(got.Reason, "did not find") {
		t.Errorf("a stopped search reports a negative finding: %q", got.Reason)
	}
}

// TestAMoveThatFailedIsNotAnAnswer — the view did not move, so the place beyond it was never
// looked at, and `exhausted` would be a claim about somewhere nobody went.
func TestAMoveThatFailedIsNotAnAnswer(t *testing.T) {
	r := &rig{
		places:  []directorapi.WorldState{worldOf("alpha"), worldOf("milk")},
		moveErr: errors.New("the region does not expose scrolling"),
	}

	got, err := r.searcher().Find(context.Background(), looking("milk"), scrollPattern(5))
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Outcome == Exhausted || got.Outcome == Found {
		t.Fatalf("outcome = %s after the move failed (%s)", got.Outcome, got.Reason)
	}
	if !strings.Contains(got.Reason, "does not expose scrolling") {
		t.Errorf("the reason does not carry why the move failed: %q", got.Reason)
	}
}

// TestAnObservationThatFailedIsNotAnAnswer — the same rule for the look rather than the move.
func TestAnObservationThatFailedIsNotAnAnswer(t *testing.T) {
	s := New(func(context.Context) (directorapi.WorldState, error) {
		return directorapi.WorldState{}, errors.New("the accessibility host is not running")
	}, func(context.Context, Move) error { return nil })

	got, err := s.Find(context.Background(), looking("milk"), scrollPattern(5))
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Outcome != Stopped {
		t.Errorf("outcome = %s; a search that could not look has not looked anywhere", got.Outcome)
	}
	if !strings.Contains(got.Reason, "accessibility host") {
		t.Errorf("the reason does not carry why looking failed: %q", got.Reason)
	}
}

// ── the trail ────────────────────────────────────────────────────────────────

// TestTheTrailSaysWhereItLooked — a search that cannot say where it went cannot be argued with,
// and `exhausted` is a strong enough claim that somebody has to be able to check it.
func TestTheTrailSaysWhereItLooked(t *testing.T) {
	r := &rig{places: []directorapi.WorldState{worldOf("alpha"), worldOf("beta"), worldOf("gamma")}}

	got, _ := r.searcher().Find(context.Background(), looking("milk"), scrollPattern(5))
	if len(got.Trail) != got.Looked {
		t.Fatalf("%d places looked at, %d recorded", got.Looked, len(got.Trail))
	}
	if got.Trail[0].After != "" {
		t.Errorf("the first look records a move that got it there: %q", got.Trail[0].After)
	}
	for i, l := range got.Trail[1:] {
		if l.After == "" {
			t.Errorf("look %d records no move that got it there", i+2)
		}
	}
}
