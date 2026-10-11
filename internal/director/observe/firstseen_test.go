package observe_test

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// When a move was MADE, against when it was believed.
//
// # The defect, measured
//
// The selection reader refuses a row it has seen once, so a move is published a look or more
// after the mark arrived. Everything downstream was dated from the publication: the move's own
// timestamp, whether a press was attributable to it, and which direction was in the window
// behind it. At 66ms a look that is one look late is 66ms late and nothing notices. At a live
// 2s cadence it is 2s late, which is five times the window — so the press that caused the move
// had stopped counting by the time the move existed.
//
// The repair is `selection.Result.Provisional`: the reader says which row this frame pointed at
// without publishing it, and the watch banks the moment. The move is still only recorded on
// CONFIRMATION — nothing here makes a provisional sighting into a reading — and what the
// sighting contributes is a time and a yes-or-no about the window as it stood then.
//
// Three claims, and the middle one is the one that would be easy to lose:
//
//	THE MOVE IS DATED EARLIER       FirstSeen is before At, and the gap is stated
//	A LATER PRESS IS NOW REFUSED    a press after the pixels moved cannot have moved them
//	AN EARLIER PRESS IS NOW KEPT    a press inside the window of the SIGHTING is attributed
//
// The second is a narrowing. It has to be asserted as hard as the third, because a change that
// only ever increases attribution is a change that cannot tell a repair from a loosening.

// slowWatch drives a five-row list at whatever cadence the caller asks for.
//
// The cadence is the argument because this file is about the gap between two looks: at 100ms
// the confirmation delay is inside the response window and nothing here can go wrong, and at
// 500ms it is outside it, which is where a live session lives.
type slowWatch struct {
	watch *observe.InterfaceWatch
	rows  []observe.Region
	base  time.Time
	seq   int
	at    time.Time
}

func newSlowWatch() *slowWatch {
	rows := make([]observe.Region, 5)
	for i := range rows {
		rows[i] = observe.Region{
			X: 0.05, Y: 0.10 + float64(i)*0.120, Width: 0.30, Height: 0.080,
		}
	}
	base := time.Unix(1760000000, 0)
	return &slowWatch{watch: observe.NewInterfaceWatch(), rows: rows, base: base, at: base}
}

func (s *slowWatch) shapes(mark int) []observe.ShadowShape {
	var out []observe.ShadowShape
	for i, r := range s.rows {
		b := 50 + float64(i)*9
		shade := []float64{b, b + 30}
		if i == mark {
			shade = []float64{b + 150, b + 180}
		}
		out = append(out, observe.ShadowShape{Region: r, Confidence: 0.3, Shade: shade})
	}
	return out
}

// look advances the clock by gap and folds one look, with navs directional intents on it.
func (s *slowWatch) look(gap time.Duration, mark, navs int) {
	s.seq++
	s.at = s.at.Add(gap)
	s.watch.Observe(observe.InterfaceLook{
		Sequence: s.seq, At: s.at, Observed: true, Shapes: s.shapes(mark), Navs: navs,
	})
}

// identify walks the list until it is the one being used, then drains what that produced.
//
// The quiet prefix is not padding: a row's unselected level is learned only from looks in which
// it was not marked, and a list is not named the active one until it has been seen handing its
// mark between members several times.
func (s *slowWatch) identify(t *testing.T, gap time.Duration) {
	t.Helper()
	for range 14 {
		s.look(gap, 0, 0)
	}
	mark := 0
	for range 30 {
		mark = (mark + 1) % 5
		s.look(gap, mark, 1)
		s.look(gap, mark, 0)
	}
	s.watch.TakeSelectionMoves()
	if got := s.watch.Belief().Status; got != observe.InterfaceProvisional {
		t.Fatalf("no list was identified (%s), so nothing below is about a move in one: %s",
			got, s.watch.Belief().Because)
	}
}

// A move says when the pixels moved, and not only when it was believed.
//
// Deleting the noteSettling call in InterfaceWatch.Observe must fail this.
func TestAMoveSaysWhenThePixelsMovedAndNotOnlyWhenItWasBelieved(t *testing.T) {
	s := newSlowWatch()
	s.identify(t, 100*time.Millisecond)

	// One move, seen over two looks: the first points at the new row, the second agrees.
	s.look(100*time.Millisecond, 2, 1)
	firstSighting := s.at
	s.look(100*time.Millisecond, 2, 0)

	moves, _ := s.watch.TakeSelectionMoves()
	if len(moves) == 0 {
		t.Fatal("the mark moved and no move was produced, so there is nothing to date")
	}
	var dated int
	for _, m := range moves {
		if m.FirstSeen.IsZero() {
			continue
		}
		dated++
		if !m.FirstSeen.Before(m.At) {
			t.Errorf("a move was first seen at %s and confirmed at %s. The reader "+
				"refuses a row it has seen once, so the confirmation is always "+
				"the later of the two", m.FirstSeen, m.At)
		}
		if m.ConfirmedAfterMS <= 0 {
			t.Errorf("the move reports a confirmation delay of %dms. Stated rather "+
				"than left to be subtracted is the whole point of the field",
				m.ConfirmedAfterMS)
		}
		if want := m.At.Sub(m.FirstSeen).Milliseconds(); m.ConfirmedAfterMS != want {
			t.Errorf("the move reports a %dms delay between two times %dms apart",
				m.ConfirmedAfterMS, want)
		}
		if !m.FirstSeen.Equal(firstSighting) {
			t.Errorf("the move is dated from %s; the pixels first pointed at its row "+
				"at %s", m.FirstSeen, firstSighting)
		}
	}
	if dated == 0 {
		t.Fatalf("not one of %d move(s) could say when the pixels moved, so the field "+
			"is never set and nothing above was checked", len(moves))
	}
}

// A press that landed AFTER the pixels moved is not what moved them.
//
// THE NARROWING, and it is the half of this change that must not be lost. Dating a move from
// its confirmation did not only lose attributions — it invented them. A press made after the
// mark had already arrived opened a window that was still open a look later, when the reader
// agreed, so the move came out prompted and the press was credited with an outcome it cannot
// have caused. That is the input predicting the outcome, which is the one thing ADR-158 exists
// to prevent, and it was reachable through the clock rather than through the data path.
//
// Deleting the refusal in InterfaceWatch.settlingOf — by or-ing the banked answer with the
// window as it stands now — must fail this.
func TestAPressAfterThePixelsMovedIsNotWhatMovedThem(t *testing.T) {
	s := newSlowWatch()
	s.identify(t, 100*time.Millisecond)

	// LONG ENOUGH FOR THE WINDOW TO CLOSE, so the sighting below happens with nothing
	// pressed and nothing recently pressed.
	for range 8 {
		s.look(200*time.Millisecond, 0, 0)
	}
	// The pixels move, with no press anywhere near them.
	s.look(100*time.Millisecond, 3, 0)
	// THEN somebody presses something, and the reader agrees on the same look.
	s.look(100*time.Millisecond, 3, 1)

	moves, _ := s.watch.TakeSelectionMoves()
	if len(moves) == 0 {
		t.Fatal("the mark moved and no move was produced, so nothing was attributed " +
			"either way and this test proves nothing")
	}
	for _, m := range moves {
		if m.FirstSeen.IsZero() {
			t.Errorf("the move could not say when the pixels moved, so it fell back "+
				"to its confirmation and this case was never reached: %+v", m)
			continue
		}
		if m.Prompted {
			t.Errorf("a move the pixels made at %s was attributed to a press observed "+
				"at %s, after it. A press cannot have caused a change that had "+
				"already happened, and crediting it is the input predicting the "+
				"outcome", m.FirstSeen, m.At)
		}
	}
}

// A press before the pixels moved is kept, though the reader agreed too late to say so.
//
// THE REPAIR, in the arrangement that produced it: looks far enough apart that the
// confirmation lands outside the window the press opened. Before this the move came out
// unprompted — and an unprompted move can never name a direction, so the whole binding chain
// went unnamed at exactly the cadence a live session runs at.
//
// 500ms between looks, against a 400ms response window: the press is 500ms before the
// confirmation and 0ms before the sighting.
func TestAPressBeforeThePixelsMovedSurvivesALateConfirmation(t *testing.T) {
	s := newSlowWatch()
	s.identify(t, 100*time.Millisecond)

	// The press and the repaint on one look, and the reader's agreement half a second
	// later — which is past MaxResponseLatency from the press.
	s.look(100*time.Millisecond, 4, 1)
	pressedAt := s.at
	s.look(500*time.Millisecond, 4, 0)

	moves, _ := s.watch.TakeSelectionMoves()
	if len(moves) == 0 {
		t.Fatal("the mark moved and no move was produced")
	}
	var prompted, dated int
	for _, m := range moves {
		if !m.FirstSeen.IsZero() {
			dated++
		}
		if m.Prompted {
			prompted++
		}
		if m.ConfirmedAfterMS > 0 &&
			m.ConfirmedAfterMS <= observe.MaxResponseLatency.Milliseconds() {
			t.Errorf("the reader agreed %dms after the pixels moved, which is inside "+
				"the %s window — so this fixture does not reach the case it is "+
				"for", m.ConfirmedAfterMS, observe.MaxResponseLatency)
		}
	}
	if dated == 0 {
		t.Fatalf("no move could say when the pixels moved, so the fallback decided this "+
			"and the repair was never exercised: %+v", moves[0])
	}
	if prompted == 0 {
		t.Errorf("a move the pixels made at the same moment as the press, confirmed "+
			"%dms later, is unprompted. The press is what this move is evidence "+
			"about, and an unprompted move can never name a direction",
			moves[0].ConfirmedAfterMS)
	}
	t.Logf("%d move(s), %d dated, %d prompted; the reader agreed %dms late",
		len(moves), dated, prompted, moves[0].ConfirmedAfterMS)
	_ = pressedAt
}

// A fast read dates a move from when the pixels moved.
//
// The path that matters live: most of a live session's moves are seen by the fast reads, which
// run several times per cycle over geometry a discovery found up to a cycle earlier. A move
// dated only by the slow looks is dated to the wrong second, so the banking has to happen on
// both paths or the measurement is about the quarter of moves the discoveries happen to catch.
//
// Deleting the noteSettling call in InterfaceWatch.Track must fail this.
func TestAFastReadDatesAMoveFromWhenThePixelsMoved(t *testing.T) {
	s := newSlowWatch()
	s.identify(t, 100*time.Millisecond)

	discovery, discoveredAt := s.seq, s.at
	shades := func(mark int) [][]float64 {
		out := make([][]float64, 5)
		for i := range out {
			b := 50 + float64(i)*9
			out[i] = []float64{b, b + 30}
			if i == mark {
				out[i] = []float64{b + 150, b + 180}
			}
		}
		return out
	}
	// Three fast reads on a new row, 20ms apart. The first points at it, the second
	// agrees — so the move exists on the second and is dated from the first.
	var sightingAt time.Time
	for i := range 3 {
		at := discoveredAt.Add(time.Duration(i+1) * 20 * time.Millisecond)
		if i == 0 {
			sightingAt = at
		}
		s.watch.Track(observe.TrackLook{
			Discovery: discovery, DiscoveredAt: discoveredAt, At: at,
			Generation: 1, Rows: s.rows, Shades: shades(1),
		})
	}
	moves, _ := s.watch.TakeSelectionMoves()
	if len(moves) == 0 {
		t.Fatalf("three fast reads moved the mark and produced no move: tracking=%+v",
			s.watch.Tracking())
	}
	var dated int
	for _, m := range moves {
		if !m.Tracked {
			continue
		}
		if m.FirstSeen.IsZero() {
			t.Errorf("a move seen by a fast read could not say when the pixels moved. "+
				"Live, these are most of the moves there are: %+v", m)
			continue
		}
		dated++
		if !m.FirstSeen.Equal(sightingAt) {
			t.Errorf("a tracked move is dated from %s; the first fast read to point "+
				"at its row was at %s", m.FirstSeen, sightingAt)
		}
	}
	if dated == 0 {
		t.Fatalf("none of %d move(s) came from a fast read, so the path this test is "+
			"about was never entered: tracking=%+v", len(moves), s.watch.Tracking())
	}
}

// A sighting in one list does not date a move in another.
//
// A settling row is an INDEX, and the same index in the next list is a different entry. The
// held sighting is already dropped when the identified list changes, for exactly this reason;
// the banked moment has to go with it, or a move in the new list would be dated from a sighting
// in the old one — and judged against the window as it stood then.
//
// Deleting the forgetSettling call in InterfaceWatch.noteMove's list-change branch must fail
// this.
func TestASightingInOneListDoesNotDateAMoveInAnother(t *testing.T) {
	s := newSlowWatch()
	s.identify(t, 100*time.Millisecond)

	// Point at a new row without confirming it, so a moment is banked.
	s.look(100*time.Millisecond, 2, 1)
	s.watch.TakeSelectionMoves()
	staleSighting := s.at

	// THE LIST CHANGES UNDER IT: one more row, so every index below it is a different
	// entry. Walked until the new shape is the one being read.
	s.rows = append(s.rows, observe.Region{X: 0.05, Y: 0.70, Width: 0.30, Height: 0.080})
	for range 20 {
		s.look(100*time.Millisecond, 0, 0)
	}
	mark := 0
	for range 20 {
		mark = (mark + 1) % len(s.rows)
		s.look(100*time.Millisecond, mark, 1)
		s.look(100*time.Millisecond, mark, 0)
	}
	moves, _ := s.watch.TakeSelectionMoves()
	for _, m := range moves {
		if !m.FirstSeen.IsZero() && !m.FirstSeen.After(staleSighting) {
			t.Errorf("a move in the new list is dated %s, at or before a sighting "+
				"made in the old one at %s. Entry 2 of a five-row list and "+
				"entry 2 of a six-row list are different entries",
				m.FirstSeen, staleSighting)
		}
	}
	if len(moves) == 0 {
		t.Skip("the renumbered list produced no move, so nothing was dated either way")
	}
	t.Logf("%d move(s) in the renumbered list, none dated from the old one", len(moves))
}

// A move is not dated from a sighting of a DIFFERENT row.
//
// The banked moment is read only for the row that was eventually confirmed, which is the
// arithmetic the whole arrangement rests on: a blend names the row the selection is LEAVING,
// so a move that took the blend's moment would be dated to whatever was happening when the
// selection started to go, and judged against the window as it stood then.
//
// Deleting the `row != w.settlingRow` test in InterfaceWatch.settlingOf must fail this.
func TestAMoveIsNotDatedFromASightingOfADifferentRow(t *testing.T) {
	s := newSlowWatch()
	s.identify(t, 100*time.Millisecond)
	s.watch.TakeSelectionMoves()

	// ONE LOOK AT ROW 1, which is not confirmed and goes nowhere.
	s.look(100*time.Millisecond, 1, 1)
	blendAt := s.at
	// Then the real arrival, two looks on row 3.
	s.look(100*time.Millisecond, 3, 0)
	arrivalAt := s.at
	s.look(100*time.Millisecond, 3, 0)

	moves, _ := s.watch.TakeSelectionMoves()
	if len(moves) == 0 {
		t.Fatal("the mark moved and no move was produced, so nothing was dated")
	}
	var checked int
	for _, m := range moves {
		if m.To != 3 || m.FirstSeen.IsZero() {
			continue
		}
		checked++
		if m.FirstSeen.Equal(blendAt) {
			t.Errorf("a move to entry 3 is dated %s, which is the look that pointed "+
				"at entry 1. A sighting of a different row is not this move's "+
				"moment", m.FirstSeen)
		}
		if !m.FirstSeen.Equal(arrivalAt) {
			t.Errorf("a move to entry 3 is dated %s; the pixels first pointed at "+
				"entry 3 at %s", m.FirstSeen, arrivalAt)
		}
	}
	if checked == 0 {
		t.Fatalf("no dated move to entry 3 among %d move(s), so this test asserts "+
			"nothing: %+v", len(moves), moves)
	}
}

// A sighting the reader gave up on is not the moment of a later arrival.
//
// The reader clears its pending candidate on any frame it cannot read, so a row pointed at,
// lost, and pointed at again is TWO sightings to the reader and says so by starting its count
// over. The watch has to follow that: keeping the first moment would date an arrival from a
// sighting the reader had already abandoned, which at a coarse cadence is seconds earlier.
//
// Deleting the ProvisionalN test in InterfaceWatch.noteSettling must fail this.
func TestALapsedSightingIsNotTheMomentOfALaterArrival(t *testing.T) {
	s := newSlowWatch()
	s.identify(t, 100*time.Millisecond)
	s.watch.TakeSelectionMoves()

	// One look at row 1, then looks the reader cannot read at all, then row 1 for real.
	s.look(100*time.Millisecond, 1, 0)
	lapsedAt := s.at
	for range 3 {
		s.seq++
		s.at = s.at.Add(400 * time.Millisecond)
		flat := make([]observe.ShadowShape, len(s.rows))
		for i, r := range s.rows {
			flat[i] = observe.ShadowShape{
				Region: r, Confidence: 0.3, Shade: []float64{90, 120},
			}
		}
		s.watch.Observe(observe.InterfaceLook{
			Sequence: s.seq, At: s.at, Observed: true, Shapes: flat,
		})
	}
	s.look(100*time.Millisecond, 1, 0)
	freshAt := s.at
	s.look(100*time.Millisecond, 1, 0)

	moves, _ := s.watch.TakeSelectionMoves()
	var checked int
	for _, m := range moves {
		if m.To != 1 || m.FirstSeen.IsZero() {
			continue
		}
		checked++
		if m.FirstSeen.Equal(lapsedAt) {
			t.Errorf("the move is dated %s, the sighting the reader gave up on "+
				"%s before it read entry 1 again. A lapsed sighting is not "+
				"an arrival", m.FirstSeen, freshAt.Sub(lapsedAt))
		}
		if m.FirstSeen.Before(freshAt) {
			t.Errorf("the move is dated %s, before the fresh run of agreement "+
				"started at %s", m.FirstSeen, freshAt)
		}
	}
	if checked == 0 {
		t.Skipf("no dated move to entry 1 among %d move(s); the unreadable looks "+
			"may have aged the held sighting past MaxUnreadBetweenSightings",
			len(moves))
	}
}

// A retained move reports how late the reader was, in the session's own terms.
//
// No labels involved, which is what makes it the number a LIVE session can report: the
// ground-truth measurement in the replay needs an annotation, and a number that only exists
// offline cannot tell anybody that a live cadence is costing the attribution.
//
// Deleting the Dated/ConfirmLateMS fold in selectionLedger.fold must fail this.
func TestARetainedMoveReportsHowLateTheReaderWas(t *testing.T) {
	var totals observe.ShadowTotals
	base := time.Unix(1760000000, 0)
	seq := 0
	look := func(mark int, intents ...observe.NavIntent) {
		seq++
		totals.Observe(oneListSample(seq,
			base.Add(time.Duration(seq)*100*time.Millisecond), mark, intents))
	}
	for range 12 {
		look(0)
	}
	mark := 0
	for range 40 {
		mark = (mark + 1) % 5
		look(mark, observe.NavDown)
		look(mark)
		look(mark)
	}
	if len(totals.SelectionTransitions) == 0 {
		t.Fatalf("no move was retained: belief=%s because=%s",
			totals.Interface.Status, totals.Interface.Because)
	}
	var dated, count int
	var late int64
	for _, st := range totals.SelectionTransitions {
		dated += st.Dated
		count += st.Count
		late += st.ConfirmLateMS
	}
	if dated == 0 {
		t.Fatalf("none of %d observation(s) could say when the pixels moved, so a "+
			"session cannot report its own confirmation delay: %s",
			count, totals.SelectionTransitions[0])
	}
	if late <= 0 {
		t.Errorf("%d dated observation(s) report %dms of total delay. The reader "+
			"refuses a row it has seen once, so every one of them was confirmed "+
			"at least a look after the pixels moved", dated, late)
	}
	if dated > count {
		t.Errorf("%d observation(s) are dated out of %d seen, which is more sightings "+
			"than there were moves", dated, count)
	}
	t.Logf("%d of %d observation(s) dated; %dms total delay, mean %dms",
		dated, count, late, late/int64(dated))
}

// A direction survives a confirmation slower than the ledger's own ceiling.
//
// THE REASON THE DIRECTION JOIN USES THE SAME MOMENT. The press ledger refuses a slot more than
// TrackStaleness old, which is the ceiling the response window clamps to. Dated from its
// confirmation, a move seen at a cadence slower than that ceiling has no slot left to attribute
// — the press that caused it is already too old by the time the reader agrees — so the move
// arrives prompted with no direction, and the three counters stop partitioning Prompted.
//
// 3.5s between looks, against a 3s ceiling: a cadence a loaded detector really reaches.
//
// Deleting the `when = m.FirstSeen` line in pressLedger.attribute must fail this.
func TestADirectionSurvivesAConfirmationSlowerThanTheCeiling(t *testing.T) {
	var totals observe.ShadowTotals
	base := time.Unix(1760000000, 0)
	seq := 0
	look := func(gap time.Duration, mark int, intents ...observe.NavIntent) {
		seq++
		totals.Observe(oneListSample(seq,
			base.Add(time.Duration(seq)*gap), mark, intents))
	}
	// Identified at a workable cadence first, because a list is not named the active one
	// until it has been seen handing its mark around several times.
	for range 12 {
		look(100*time.Millisecond, 0)
	}
	mark := 0
	for range 30 {
		mark = (mark + 1) % 5
		look(100*time.Millisecond, mark, observe.NavDown)
		look(100*time.Millisecond, mark)
		look(100*time.Millisecond, mark)
	}
	if totals.Interface.Status != observe.InterfaceProvisional {
		t.Fatalf("no list was identified, so nothing below is about a move in one: %s",
			totals.Interface.Because)
	}
	// THE DELTA AND NOT THE TOTAL, which is the difference between this test asserting
	// something and asserting nothing: the fast phase above produces plenty of named
	// directions, and a sum over the whole session is satisfied by those whatever the slow
	// phase does. Measured — the mutation gate put the confirmation time back and this
	// test passed until it counted only what the slow run added.
	count := func() (prompted, named int) {
		for _, st := range totals.SelectionTransitions {
			prompted += st.Prompted
			for _, d := range st.Directions {
				named += d.Count
			}
		}
		return
	}
	promptedBefore, namedBefore := count()

	// THEN THE CADENCE COLLAPSES. The press and the repaint land on one look; the reader
	// agrees on the next, 3.5s later — past the ledger's own ceiling.
	slow := 3500 * time.Millisecond
	for range 8 {
		mark = (mark + 1) % 5
		look(slow, mark, observe.NavDown)
		look(slow, mark)
	}
	promptedAfter, namedAfter := count()
	prompted, named := promptedAfter-promptedBefore, namedAfter-namedBefore

	if prompted == 0 {
		t.Fatalf("the slow run added no prompted observation, so there was nothing for "+
			"a direction to be refused from (%d prompted before, %d after)",
			promptedBefore, promptedAfter)
	}
	if named == 0 {
		t.Errorf("the slow run added %d prompted observation(s) and not one named a "+
			"direction. Dated from its confirmation a move at this cadence has no "+
			"slot left to attribute — the press is already past %s by the time the "+
			"reader agrees — so the whole binding chain goes unnamed at exactly the "+
			"cadence a live session runs at", prompted, observe.TrackStaleness)
	}
	t.Logf("the slow run added %d prompted observation(s), %d of which named a direction",
		prompted, named)
}
