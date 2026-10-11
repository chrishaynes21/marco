package observe_test

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// THE MARK'S MOVEMENT BECOMING SOMETHING THE SESSION KEEPS.
//
// Everything before this could say "something on the screen changed after an input". A screen
// transition is keyed on two whole compositions, and navigating a menu changes neither of them
// until somebody activates an entry — so the thing a menu mostly does produced nothing a later
// layer could read.
//
// These enter through ShadowTotals.Observe, the call the live cycle makes, because the rule can
// be perfect while nothing ever hands it a look. Eight recorded cases of that in this
// repository.
//
// Every test here is also a statement about what must NOT be recorded: a move between two
// different lists, a move across a renumbering, and above all a move that no pixels showed.

// oneListSample is one look at a single five-row list, with its own observed navigation.
//
// A SINGLE list, unlike navattribution_test.go's pair: the question there is which of two lists
// answered, and the question here is what is retained once one of them has. The rows differ
// from each other for the reason that fixture's do — see the note there about the dwell defect,
// which is measured, real, and not what this file is about.
func oneListSample(seq int, at time.Time, mark int,
	intents []observe.NavIntent) observe.Sample {

	var shapes []observe.ShadowShape
	for row := range 5 {
		base := 50 + float64(row)*9
		shade := []float64{base, base + 30}
		if row == mark {
			shade = []float64{base + 150, base + 180}
		}
		shapes = append(shapes, observe.ShadowShape{
			Region: observe.Region{
				X: 0.05, Y: 0.10 + float64(row)*0.120, Width: 0.30, Height: 0.080,
			},
			Confidence: 0.3,
			Shade:      shade,
		})
	}
	var inputs []observe.InputEvent
	for i, in := range intents {
		inputs = append(inputs, observe.InputEvent{
			Intent: in, AtMS: int64(seq)*2000 + int64(i)*10,
		})
	}
	return observe.Sample{
		Sequence: seq, WindowGeneration: 1, Timestamp: at,
		Shadow: &observe.ShadowSample{
			Detector: "screenparser", Ran: true, TargetProven: true,
			Shapes: shapes, Inputs: inputs,
		},
	}
}

// walkStep is one look in a scripted walk: where the mark is, and what was pressed.
type walkStep struct {
	mark    int
	intents []observe.NavIntent
}

// foldWalk runs a scripted walk through the production accumulator at a 100ms cadence.
//
// 100ms rather than the 2s of navattribution_test.go, because at 2s intervals every look is
// its own attribution window and nothing here would test the window at all. A real menu is
// read faster than it is navigated.
func foldWalk(steps []walkStep) observe.ShadowTotals {
	var totals observe.ShadowTotals
	base := time.Unix(1760000000, 0)
	for i, s := range steps {
		totals.Observe(oneListSample(i+1,
			base.Add(time.Duration(i)*100*time.Millisecond), s.mark, s.intents))
	}
	return totals
}

// settledWalk is a list that sits still long enough to be judged, then is navigated.
//
// The quiet prefix is not padding: a row's unselected level is learned only from looks in
// which it was not marked, so nothing can be read as selected until every row has been quiet
// several times.
func settledWalk(moves int, pressed observe.NavIntent) []walkStep {
	var out []walkStep
	for range 12 {
		out = append(out, walkStep{mark: 0})
	}
	mark := 0
	for i := range moves {
		mark = (mark + 1) % 5
		out = append(out, walkStep{mark: mark, intents: []observe.NavIntent{pressed}})
		_ = i
		// Two quiet looks after each press, so the move is seen settled rather than
		// mid-repaint, and so the window has somewhere to decay to.
		out = append(out, walkStep{mark: mark}, walkStep{mark: mark})
	}
	return out
}

// A session keeps the moves the mark made, with the presses they followed.
//
// THE POINT OF THE WHOLE CHANGE, as one assertion: a menu being navigated now leaves something
// behind. Before this the only durable trace of a person walking down a list was a candidate's
// transfer COUNT — a number, with no way to ask which entry the mark went from or to.
//
// Deleting the groundSelectionMoves call in ShadowTotals.Observe must fail this.
func TestTheSessionRetainsTheMovesTheMarkMade(t *testing.T) {
	totals := foldWalk(settledWalk(40, observe.NavDown))

	if len(totals.SelectionTransitions) == 0 {
		t.Fatalf("a five-row list navigated forty times retained no moves at all. "+
			"belief=%s selected=%d because=%s", totals.Interface.Status,
			totals.Interface.Selected, totals.Interface.Because)
	}
	// AND THE ENTRIES ARE NAMED, which is the difference between this and a transfer count.
	seen, prompted := 0, 0
	for _, st := range totals.SelectionTransitions {
		if st.From < 0 || st.To < 0 || st.From == st.To {
			t.Errorf("retained a move %d->%d, which is not a move between two entries",
				st.From, st.To)
		}
		if st.Entries != 5 {
			t.Errorf("move %s names a list of %d entries; the fixture draws five",
				st, st.Entries)
		}
		if st.Count != st.Prompted+st.Unprompted {
			t.Errorf("move %s: %d seen but %d+%d attributed, so one of the counters is "+
				"not an itemisation of the other", st, st.Count, st.Prompted,
				st.Unprompted)
		}
		seen += st.Count
		prompted += st.Prompted
	}
	if prompted == 0 {
		t.Errorf("%d moves retained and not one of them attributed to a press, in a "+
			"session where every move followed one. The attribution window never "+
			"reached the ledger", seen)
	}
	t.Logf("%d distinct moves, %d observations, %d prompted",
		len(totals.SelectionTransitions), seen, prompted)
}

// THE INVARIANT. Navigation alone retains nothing.
//
// Sixty looks of somebody pressing down at a list whose mark never moves. An input must not be
// able to manufacture a visual outcome, which is ADR-158's rule, and this is the point at which
// breaking it would be most profitable: the ledger is the thing a route would be built from, and
// a ledger that filled up from the keyboard would describe a menu nobody was using.
func TestNavigationAloneRetainsNoMove(t *testing.T) {
	var steps []walkStep
	for i := range 60 {
		var intents []observe.NavIntent
		if i >= 8 {
			intents = []observe.NavIntent{observe.NavDown}
		}
		steps = append(steps, walkStep{mark: 2, intents: intents})
	}
	totals := foldWalk(steps)

	if n := len(totals.SelectionTransitions); n != 0 {
		t.Errorf("%d move(s) retained from a session in which the mark never moved: %v",
			n, totals.SelectionTransitions)
	}
}

// A press made during a slot the detector sat out still attributes the move it explains.
//
// # Why this is the test that matters live
//
// `interfaceLookFrom` reads navigation only on a look that was OBSERVED. Live, an inference
// starts about every two seconds against a session sampling twice a second, so three slots in
// four bring no look at all — and the press that opened a menu or moved a mark is as likely to
// land in one of those as in the one that saw it. Before this, such a press never opened the
// attribution window: the graph would have filled with moves nobody appeared to have caused,
// on a session where somebody was pressing a key every second.
//
// Deleting the `t.watch.Navigated(...)` call in ShadowTotals.Observe must fail this.
func TestAPressDuringASkippedSlotStillAttributesTheMoveItExplains(t *testing.T) {
	var totals observe.ShadowTotals
	base := time.Unix(1760000000, 0)
	seq := 0
	at := func() time.Time {
		return base.Add(time.Duration(seq) * 100 * time.Millisecond)
	}
	// Settle first, with looks that ran. A list is not identified until it has been seen
	// handing its mark around, which takes most of this prefix.
	for range 14 {
		seq++
		totals.Observe(oneListSample(seq, at(), 0, nil))
	}
	mark := 0
	for range 30 {
		// THE PRESS, ON A SLOT THE DETECTOR SAT OUT. The input producer drains every
		// cycle whether or not an inference runs, so this is the ordinary live shape
		// and not a contrived one.
		seq++
		skipped := oneListSample(seq, at(), mark, []observe.NavIntent{observe.NavDown})
		skipped.Shadow.Ran = false
		skipped.Shadow.Shapes = nil
		totals.Observe(skipped)
		// THE MOVE, on the next slot that did look.
		seq++
		mark = (mark + 1) % 5
		totals.Observe(oneListSample(seq, at(), mark, nil))
		seq++
		totals.Observe(oneListSample(seq, at(), mark, nil))
	}

	prompted, total := 0, 0
	for _, st := range totals.SelectionTransitions {
		prompted += st.Prompted
		total += st.Count
	}
	if total == 0 {
		t.Fatalf("no move was retained at all, so this test cannot say anything about "+
			"attribution. belief=%s because=%s", totals.Interface.Status,
			totals.Interface.Because)
	}
	if prompted == 0 {
		t.Errorf("%d moves retained and none attributed. Every one of them followed a "+
			"press made on a slot the detector sat out, and the window those presses "+
			"should have opened never opened", total)
	}
	t.Logf("%d moves, %d attributed to presses made on skipped slots", total, prompted)
}

// Every move of the mark is offered for grounding, INCLUDING the ones only a fast look saw.
//
// # Why the fast path has to be in this test
//
// Because live it is most of them. ADR-152's tracking path reads the mark several times per
// cycle over remembered geometry, and the discovery that found the list runs once every two
// seconds. A ledger fed only from the discovery path would miss the majority of a live
// session's moves while looking entirely healthy — the counters would all be non-zero.
//
// Deleting the `w.noteMove(before, next)` call in InterfaceWatch.settle must fail this.
func TestEveryMoveOfTheMarkIsOfferedForGrounding(t *testing.T) {
	watch := observe.NewInterfaceWatch()
	base := time.Unix(1760000000, 0)
	rows := make([]observe.Region, 5)
	for row := range 5 {
		rows[row] = observe.Region{
			X: 0.05, Y: 0.10 + float64(row)*0.120, Width: 0.30, Height: 0.080,
		}
	}
	look := func(seq int, mark int, navs int) observe.InterfaceLook {
		var shapes []observe.ShadowShape
		for row := range 5 {
			b := 50 + float64(row)*9
			shade := []float64{b, b + 30}
			if row == mark {
				shade = []float64{b + 150, b + 180}
			}
			shapes = append(shapes, observe.ShadowShape{
				Region: rows[row], Confidence: 0.3, Shade: shade,
			})
		}
		return observe.InterfaceLook{
			Sequence: seq, At: base.Add(time.Duration(seq) * 100 * time.Millisecond),
			Observed: true, Shapes: shapes, Navs: navs,
		}
	}
	// Settle, then move with discoveries so a list is identified at all.
	seq := 0
	for range 14 {
		seq++
		watch.Observe(look(seq, 0, 0))
	}
	mark := 0
	for range 30 {
		seq++
		mark = (mark + 1) % 5
		watch.Observe(look(seq, mark, 1))
		seq++
		watch.Observe(look(seq, mark, 0))
	}
	slow, _ := watch.TakeSelectionMoves()
	if len(slow) == 0 {
		t.Fatalf("the discovery path offered no moves at all: belief=%s %s",
			watch.Belief().Status, watch.Belief().Because)
	}

	// NOW THE FAST PATH, against the geometry the discovery just found.
	discoveredAt := base.Add(time.Duration(seq) * 100 * time.Millisecond)
	shadesFor := func(mark int) [][]float64 {
		out := make([][]float64, 5)
		for row := range 5 {
			b := 50 + float64(row)*9
			out[row] = []float64{b, b + 30}
			if row == mark {
				out[row] = []float64{b + 150, b + 180}
			}
		}
		return out
	}
	// FOUR MOVES, each held for three reads. Held, because the selection reader refuses a
	// row it has seen once — "a different row this frame; waiting for it to be confirmed" —
	// and a fixture that moved the mark on every read would be measuring that refusal
	// rather than this path.
	read := 0
	for range 4 {
		mark = (mark + 1) % 5
		for range 3 {
			read++
			watch.Track(observe.TrackLook{
				Discovery: seq, DiscoveredAt: discoveredAt,
				At:         discoveredAt.Add(time.Duration(read) * 20 * time.Millisecond),
				Generation: 1, Rows: rows, Shades: shadesFor(mark),
			})
		}
	}
	fast, dropped := watch.TakeSelectionMoves()
	if dropped != 0 {
		t.Errorf("%d moves dropped by the bound in a test that makes four", dropped)
	}
	tracked := 0
	for _, m := range fast {
		if m.Tracked {
			tracked++
		}
	}
	if tracked == 0 {
		t.Fatalf("%d move(s) from four fast reads, none marked as tracked. The fast path "+
			"is where a live session's moves come from, and a ledger fed only by "+
			"discoveries would lose most of them", len(fast))
	}
	t.Logf("%d moves from discoveries, %d from fast reads", len(slow), tracked)
}

// A fast read long after the press is not attributed to it.
//
// The other side of the bound, on the path that has no navigation of its own. A TrackLook
// carries no input — the producer drains per cycle and a fast read is not a cycle — so without
// ageing the window on this path the debt is only ever aged by slow looks, which live are
// seconds apart. A mark read well after somebody pressed down would be credited to that press,
// and every move a panel makes on its own schedule would belong to whoever last touched the
// controller.
//
// Two seconds: past MaxResponseLatency by five times over, and still inside TrackStaleness, so
// the read is admitted rather than refused for a different reason.
//
// Deleting the `w.navOpen(look.At)` call in InterfaceWatch.Track must fail this.
func TestAFastMoveLongAfterThePressIsNotPrompted(t *testing.T) {
	watch := observe.NewInterfaceWatch()
	base := time.Unix(1760000000, 0)
	rows := make([]observe.Region, 5)
	for row := range 5 {
		rows[row] = observe.Region{
			X: 0.05, Y: 0.10 + float64(row)*0.120, Width: 0.30, Height: 0.080,
		}
	}
	shadesFor := func(mark int) [][]float64 {
		out := make([][]float64, 5)
		for row := range 5 {
			b := 50 + float64(row)*9
			out[row] = []float64{b, b + 30}
			if row == mark {
				out[row] = []float64{b + 150, b + 180}
			}
		}
		return out
	}
	shapesFor := func(mark int) []observe.ShadowShape {
		var out []observe.ShadowShape
		for row, sh := range shadesFor(mark) {
			out = append(out, observe.ShadowShape{
				Region: rows[row], Confidence: 0.3, Shade: sh,
			})
		}
		return out
	}
	seq := 0
	at := func() time.Time {
		return base.Add(time.Duration(seq) * 100 * time.Millisecond)
	}
	for range 14 {
		seq++
		watch.Observe(observe.InterfaceLook{
			Sequence: seq, At: at(), Observed: true, Shapes: shapesFor(0),
		})
	}
	mark := 0
	for range 30 {
		seq++
		mark = (mark + 1) % 5
		watch.Observe(observe.InterfaceLook{
			Sequence: seq, At: at(), Observed: true, Shapes: shapesFor(mark), Navs: 1,
		})
		seq++
		watch.Observe(observe.InterfaceLook{
			Sequence: seq, At: at(), Observed: true, Shapes: shapesFor(mark),
		})
	}
	watch.TakeSelectionMoves()
	if watch.Belief().Status != observe.InterfaceProvisional {
		t.Fatalf("no list was identified, so the fast path is refused and this test "+
			"proves nothing: %s", watch.Belief().Because)
	}

	// THE PRESS, on a look of its own, and then nothing for two seconds.
	seq++
	discoveredAt := at()
	watch.Observe(observe.InterfaceLook{
		Sequence: seq, At: discoveredAt, Observed: true, Shapes: shapesFor(mark), Navs: 1,
	})
	watch.TakeSelectionMoves()

	mark = (mark + 1) % 5
	for i := range 3 {
		watch.Track(observe.TrackLook{
			Discovery: seq, DiscoveredAt: discoveredAt,
			At: discoveredAt.Add(2*time.Second +
				time.Duration(i)*20*time.Millisecond),
			Generation: 1, Rows: rows, Shades: shadesFor(mark),
		})
	}
	moves, _ := watch.TakeSelectionMoves()
	if len(moves) == 0 {
		t.Fatalf("the fast read moved the mark and produced no move, so there is nothing "+
			"to attribute and this test proves nothing. tracking=%+v", watch.Tracking())
	}
	for _, m := range moves {
		if m.Prompted {
			t.Errorf("a move read %s after the press was attributed to it; the bound "+
				"is %s", m.At.Sub(discoveredAt), observe.MaxResponseLatency)
		}
	}
}

// ── the move reaching the hypothesis layer ────────────────────────────────────

// A move of the mark becomes a hypothesis somebody could read.
//
// The retention above is the measurement; this is the sentence. Nothing downstream reads
// SelectionTransitions directly — the hypothesis layer is where a session's evidence becomes
// something a reader or a route can ask about — so a ledger nothing generated from would be
// another case of complete code with no consumer.
//
// Deleting the `selectionMoves(t, th)` line in Hypotheses must fail this.
func TestAMoveOfTheMarkBecomesAHypothesis(t *testing.T) {
	totals := foldWalk(settledWalk(40, observe.NavDown))
	hs := observe.Hypotheses(totals, observe.DefaultHypothesisThresholds())

	var moves []observe.Hypothesis
	for _, h := range hs {
		if h.Kind == observe.PossibleSelectionTransition {
			moves = append(moves, h)
		}
	}
	if len(moves) == 0 {
		t.Fatalf("%d hypotheses from a session with %d retained moves, and not one of "+
			"them is about the mark moving", len(hs), len(totals.SelectionTransitions))
	}
	for _, h := range moves {
		if h.Subject.Kind != observe.SubjectSelection {
			t.Errorf("a selection move is filed under subject kind %q", h.Subject.Kind)
		}
		if h.Subject.Selection == nil {
			t.Fatal("a selection hypothesis carries no entries, so a reader cannot tell " +
				"which rows it is about")
		}
		// THE DISTINCTION THAT MATTERS: the screen did not change. A reader must not be
		// able to mistake this for a state transition, and the subject says so by
		// naming the same screen on both sides.
		if h.Subject.Ref != h.Subject.To {
			t.Errorf("a selection move claims %q became %q. Moving a mark is not a "+
				"screen change", h.Subject.Ref, h.Subject.To)
		}
		sel := h.Subject.Selection
		if sel.From == sel.To || sel.Entries <= 0 {
			t.Errorf("a selection hypothesis about entry %d -> %d of %d entries",
				sel.From, sel.To, sel.Entries)
		}
		// AND IT SAYS WHAT WOULD SETTLE IT. A hypothesis with no test is an assertion.
		if h.Validation == "" {
			t.Error("a selection hypothesis offers nothing a person could do to settle it")
		}
	}
	t.Logf("%d selection hypotheses of %d, first: %s", len(moves), len(hs), moves[0].Observed)
}

// A hypothesis about the mark never says the press moved it.
//
// The one sentence this layer must not produce. `Observed` and the navigation support may say
// a press was seen before the move; neither may say it caused it, and the validation must
// still be an experiment rather than a conclusion. Causation needs an action performed
// deliberately and the outcome checked, which is a rehearsal and not a passive observation.
func TestASelectionHypothesisClaimsAssociationAndNotCause(t *testing.T) {
	totals := foldWalk(settledWalk(40, observe.NavDown))
	hs := observe.Hypotheses(totals, observe.DefaultHypothesisThresholds())

	banned := []string{"caused", "because the", "which moved", "will move", "moves the mark to"}
	for _, h := range hs {
		if h.Kind != observe.PossibleSelectionTransition {
			continue
		}
		texts := []string{h.Observed, h.Validation}
		for _, e := range h.Support {
			texts = append(texts, e.Statement)
		}
		for _, e := range h.Contradictions {
			texts = append(texts, e.Statement)
		}
		for _, text := range texts {
			for _, bad := range banned {
				if strings.Contains(strings.ToLower(text), bad) {
					t.Errorf("a selection hypothesis says %q, which claims the press "+
						"did it: %q", bad, text)
				}
			}
		}
		// AND THE COUNTER-EXAMPLE IS CARRIED. A move with unattributed observations
		// must say so where a reader is already looking.
		if h.Unattributed > 0 && len(h.Contradictions) == 0 {
			t.Errorf("a hypothesis reports %d unattributed moves and no contradiction, "+
				"so the counter-example has been deleted from the claim", h.Unattributed)
		}
	}
}

// ── the three guards on what counts as one list ───────────────────────────────

// A list that SCROLLED has not handed its mark from one of its entries to another.
//
// # Why the obvious fixtures do not pin this
//
// Two lists in different places was the first attempt, three times over. A cross-list move is
// recorded with the arriving list's envelope, so asserting on position proves nothing; a walk
// that wraps produces the same delta as a crossing, so asserting on deltas proves nothing; and
// making the first list vanish and a second appear leaves BOTH qualifying — the first one's
// counters do not expire — so the belief goes ambiguous and retains nothing at all.
//
// What isolates the envelope is the same list MOVING. Its rows shift up by one pitch while its
// entry count stays the same, so "entry 1" before and "entry 3" after are positions in two
// different arrangements. The candidate still matches itself, so the belief stays provisional
// throughout and the entry-count half of the guard never fires. Only the envelope says this is
// not one list handing its mark along.
//
// Deleting `next.Where != w.heldWhere` from the guard in noteMove must fail this.
func TestAListThatScrolledRetainsNoMoveAcrossTheScroll(t *testing.T) {
	var totals observe.ShadowTotals
	base := time.Unix(1760000000, 0)
	seq := 0
	feed := func(mark int, shift float64) {
		seq++
		sample := oneListSample(seq,
			base.Add(time.Duration(seq)*100*time.Millisecond), mark, nil)
		for i := range sample.Shadow.Shapes {
			sample.Shadow.Shapes[i].Region.Y -= shift
		}
		totals.Observe(sample)
	}
	// Identify the list, with the mark settling on entry 1.
	for range 14 {
		feed(0, 0)
	}
	for range 12 {
		for mark := range 5 {
			for range 3 {
				feed(mark, 0)
			}
		}
	}
	for range 4 {
		feed(1, 0)
	}
	if totals.Interface.Status != observe.InterfaceProvisional {
		t.Fatalf("no list was identified, so the guard is not what refuses anything "+
			"below: %s", totals.Interface.Because)
	}
	before := map[string]int{}
	for _, st := range totals.SelectionTransitions {
		before[st.String()] = st.Count
	}
	entries := totals.Interface.Entries

	// THE SCROLL: every row one pitch higher, the mark now on entry 3. Same count of
	// entries, different arrangement.
	for range 6 {
		feed(3, 0.120)
	}
	if totals.Interface.Status != observe.InterfaceProvisional {
		t.Fatalf("the scroll lost the list altogether (%s), so the envelope guard is not "+
			"what refused the move", totals.Interface.Because)
	}
	if got := totals.Interface.Entries; got != entries {
		t.Fatalf("the list went from %d entries to %d across the scroll, so the "+
			"entry-count guard would refuse this move too and the envelope half is "+
			"untested", entries, got)
	}
	for _, st := range totals.SelectionTransitions {
		if was, seen := before[st.String()]; !seen || was != st.Count {
			t.Errorf("retained %s across a scroll. The list moved, so entry 1 before it "+
				"and entry 3 after it are positions in two different arrangements", st)
		}
	}
	t.Logf("%d moves before the scroll, %d after, %d entries throughout",
		len(before), len(totals.SelectionTransitions), entries)
}

// A list that gains an entry has renumbered itself, and did not hand its mark along.
//
// # Why this needs its own fixture
//
// Because the obvious one — a row appearing above the list — moves the list's EXTENT as well as
// its numbering, so the envelope half of the guard catches it and the entry-count half is never
// consulted.
//
// A row appearing BETWEEN two entries changes nothing about where the list is. It changes what
// "entry 3" means, which is the whole claim. And it has to appear only AFTER the list has been
// identified: an interloper present throughout stops the list qualifying at all, and the test
// then passes because nothing was watching rather than because the guard held.
//
// Deleting `next.Entries != w.heldEntries` from the guard in noteMove must fail this.
func TestAListThatRenumbersRetainsNoMove(t *testing.T) {
	var totals observe.ShadowTotals
	base := time.Unix(1760000000, 0)
	seq := 0
	feed := func(mark int, interloper bool) {
		seq++
		sample := oneListSample(seq,
			base.Add(time.Duration(seq)*100*time.Millisecond), mark, nil)
		if interloper {
			sample.Shadow.Shapes = append(sample.Shadow.Shapes, observe.ShadowShape{
				Region: observe.Region{
					X: 0.05, Y: 0.10 + 0.090, Width: 0.20, Height: 0.022,
				},
				Confidence: 0.3, Shade: []float64{44, 74},
			})
		}
		totals.Observe(sample)
	}
	// Identify the list first, with nothing interleaved.
	for range 14 {
		feed(0, false)
	}
	for range 12 {
		for mark := range 5 {
			for range 3 {
				feed(mark, false)
			}
		}
	}
	if totals.Interface.Status != observe.InterfaceProvisional {
		t.Fatalf("no list was identified, so the guard is not what refuses anything "+
			"below: %s", totals.Interface.Because)
	}
	before := len(totals.SelectionTransitions)

	// NOW the numbering changes under a mark that never moves, each state held for
	// three looks.
	//
	// HELD, because the selection reader refuses a row it has seen once. Alternating
	// every look means the index is never confirmed, Selected stays unreadable, and
	// nothing is recorded whether the guard is there or not — which is how the first
	// version of this test survived its own gate.
	for i := range 60 {
		feed(3, (i/3)%2 == 0)
	}
	if got := len(totals.SelectionTransitions); got != before {
		var fresh []observe.SelectionTransition
		for _, st := range totals.SelectionTransitions {
			fresh = append(fresh, st)
		}
		t.Errorf("%d moves became %d while the mark sat on one row and only the "+
			"numbering changed around it: %v", before, got, fresh)
	}
	t.Logf("%d moves before the renumbering and %d after; entries seen: %d",
		before, len(totals.SelectionTransitions), totals.Interface.Entries)
}
