package selection_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// Telling the list somebody is using from the lists that merely move.
//
// All drawn, and the negative cases are the point: a heads-up display and a rotating
// advertisement are both busier than a menu, and neither is being chosen from.

const (
	sceneW, sceneH = 600, 400
	colLeft        = 40
	colRight       = 340
	bandH          = 30
	bandGap        = 40
)

type scene struct{ img *image.RGBA }

func newScene() *scene {
	img := image.NewRGBA(image.Rect(0, 0, sceneW, sceneH))
	for y := range sceneH {
		for x := range sceneW {
			img.SetRGBA(x, y, color.RGBA{20, 20, 20, 255})
		}
	}
	return &scene{img}
}

// column draws n bands at x, each at the level given, and returns their rows in one group.
func (s *scene) column(x, group, n int, levels []int) []selection.Row {
	var rows []selection.Row
	for i := range n {
		top := 30 + i*bandGap
		r := image.Rect(x, top, x+220, top+bandH)
		fill(s.img, r, levels[i])
		rows = append(rows, selection.Row{Bounds: r, Group: group})
	}
	return rows
}

// levels returns a base set with one member raised, or none when marked is negative.
func levels(n, marked, base, high int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = base
	}
	if marked >= 0 && marked < n {
		out[marked] = high
	}
	return out
}

func TestAListThatHandsAMarkAroundIsTheActiveOne(t *testing.T) {
	tr := selection.NewTracker()
	// A menu on the left; on the right, four bands that never change.
	for _, mark := range []int{-1, -1, -1, -1, 0, 0, 0, 1, 1, 1, 2, 2, 2, 1, 1, 1} {
		s := newScene()
		rows := s.column(colLeft, 0, 4, levels(4, mark, 60, 210))
		rows = append(rows, s.column(colRight, 1, 4, levels(4, -1, 90, 90))...)
		tr.Observe(s.img, rows)
	}
	box, why, ok := tr.Active()
	if !ok {
		t.Fatalf("the menu should have been identified: %s", why)
	}
	if box.Min.X != colLeft {
		t.Errorf("the wrong list was named: %v (%s)", box, why)
	}
	if tr.IdentifiedAt() <= 0 {
		t.Error("identification should record when it happened")
	}
}

func TestAHeadsUpDisplayIsNotAListSomebodyIsChoosingFrom(t *testing.T) {
	// Several values changing at once, independently. Busier than a menu, and nothing is
	// being chosen: no frame has exactly one member standing out.
	tr := selection.NewTracker()
	for i := range 20 {
		s := newScene()
		l := []int{60, 60, 60, 60}
		l[0] = 60 + (i*37)%140
		l[2] = 60 + (i*53)%140
		rows := s.column(colLeft, 0, 4, l)
		tr.Observe(s.img, rows)
	}
	if _, why, ok := tr.Active(); ok {
		t.Fatalf("a heads-up display is not the active interface (%s)", why)
	}
}

func TestARotatingAdvertisementIsNotAListEither(t *testing.T) {
	// One member changes over and over and the change never transfers to a sibling. This
	// is the case that defeated "the list with the most activity", and it is why the test
	// is TRANSFER rather than movement.
	tr := selection.NewTracker()
	for i := range 24 {
		s := newScene()
		l := []int{60, 60, 60, 60}
		if i%3 != 0 {
			l[1] = 215 // the same member, repeatedly
		}
		rows := s.column(colLeft, 0, 4, l)
		tr.Observe(s.img, rows)
	}
	if _, why, ok := tr.Active(); ok {
		t.Fatalf("a region that changes without transferring is not a menu (%s)", why)
	}
}

func TestTwoListsBothHandingMarksAroundIsAnAbstention(t *testing.T) {
	// Picking one would be guessing, and a guess that happens to be right teaches nothing
	// about when to trust the next one.
	tr := selection.NewTracker()
	marks := []int{-1, -1, -1, 0, 0, 0, 1, 1, 1, 2, 2, 2, 1, 1, 1, 0, 0, 0}
	for _, m := range marks {
		s := newScene()
		rows := s.column(colLeft, 0, 4, levels(4, m, 60, 210))
		rows = append(rows, s.column(colRight, 1, 4, levels(4, m, 60, 210))...)
		tr.Observe(s.img, rows)
	}
	box, why, ok := tr.Active()
	if ok {
		t.Fatalf("two lists behaving alike must abstain, named %v (%s)", box, why)
	}
	if why == "" {
		t.Error("an abstention must say why")
	}
}

func TestIdentificationNeedsTheMarkToMoveMoreThanOnce(t *testing.T) {
	// A single transfer cannot tell a mark that MOVED from a mark that merely appeared
	// somewhere new, so one is not enough.
	tr := selection.NewTracker()
	for _, m := range []int{-1, -1, -1, -1, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1} {
		s := newScene()
		tr.Observe(s.img, s.column(colLeft, 0, 4, levels(4, m, 60, 210)))
	}
	if _, _, ok := tr.Active(); ok {
		t.Fatal("one transfer is not enough to identify an active list")
	}
	for _, m := range []int{2, 2, 2, 2, 2} {
		s := newScene()
		tr.Observe(s.img, s.column(colLeft, 0, 4, levels(4, m, 60, 210)))
	}
	if _, why, ok := tr.Active(); !ok {
		t.Fatalf("a second transfer should settle it: %s", why)
	}
}

func TestTheEvidenceIsKeptSoAWrongAnswerCanBeArguedWith(t *testing.T) {
	tr := selection.NewTracker()
	for _, m := range []int{-1, -1, -1, 0, 0, 0, 1, 1, 1, 2, 2, 2, 0, 0, 0} {
		s := newScene()
		tr.Observe(s.img, s.column(colLeft, 0, 4, levels(4, m, 60, 210)))
	}
	ev := tr.Evidence()
	if len(ev) == 0 {
		t.Fatal("no evidence was kept")
	}
	if ev[0].Transfers == 0 || ev[0].Holders == 0 || ev[0].Frames == 0 {
		t.Errorf("the evidence is empty: %s", ev[0])
	}
	if ev[0].String() == "" {
		t.Error("evidence must be able to say what it saw")
	}
}

func TestAListThatIsOftenQuietIsStillAList(t *testing.T) {
	// The mistake this guards, found on real data rather than reasoned about.
	//
	// An earlier rule also demanded that a candidate carry a mark in most of the frames
	// it was seen in — "a menu nearly always has something selected". It ranked the two
	// real candidates BACKWARDS: the heads-up display scored 0.44 and the actual menu
	// 0.33, because the menu's last two entries had been grouped separately and the
	// selection sat on them for most of the run. The rest of the menu was honestly quiet
	// and was punished for it.
	//
	// A list need not always carry a mark. What it must not do is carry several at once.
	tr := selection.NewTracker()
	marks := []int{-1, -1, -1, 0, 0, 0, -1, -1, -1, -1, -1, 1, 1, 1, -1, -1, -1, -1,
		-1, -1, 2, 2, 2, -1, -1, -1, -1, -1, -1, -1}
	for _, m := range marks {
		s := newScene()
		tr.Observe(s.img, s.column(colLeft, 0, 4, levels(4, m, 60, 210)))
	}
	if _, why, ok := tr.Active(); !ok {
		t.Fatalf("a mostly-quiet list that hands a mark around is still the active one: %s", why)
	}
}

func TestAFrameTooThinToJudgeIsNotHeldAgainstAList(t *testing.T) {
	// A detector finds some of a list's rows on one frame and not others. That is a
	// statement about the observation, not about the list, and counting it as "no mark"
	// holds a list's own invisibility against it. [[ADR-006-unknown-is-not-false]].
	tr := selection.NewTracker()
	marks := []int{-1, -1, -1, -1, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2,
		2, 2, 0, 0, 0, 0, 0, 0}
	for i, m := range marks {
		s := newScene()
		rows := s.column(colLeft, 0, 4, levels(4, m, 60, 210))
		if i%2 == 1 {
			rows = rows[:2] // the detector found only two of the four
		}
		tr.Observe(s.img, rows)
	}
	ev := tr.Evidence()
	if len(ev) == 0 {
		t.Fatal("nothing was tracked")
	}
	if ev[0].Unreadable == 0 {
		t.Error("thin frames should be counted as unreadable, not as quiet")
	}
	if _, why, ok := tr.Active(); !ok {
		t.Fatalf("thin frames must not disqualify a list that hands a mark around: %s", why)
	}
}

func TestABusyListThatMarksSeveralAtOnceIsStillRejected(t *testing.T) {
	// The real heads-up display, in miniature. It is not rejected for being quiet — it
	// had 52 exclusive frames and 21 transfers, more of both than the actual menu. It is
	// rejected because 26 of its frames marked several members at once, and whatever that
	// is, it is not one thing being chosen.
	tr := selection.NewTracker()
	// Six rows, so two marked at once still leaves a clear majority at rest and both
	// genuinely read as marked. With four, two marks drag the middle and neither
	// stands out. The quiet opening matters too: a row that is ALWAYS different never
	// establishes a baseline, so it can never be seen to differ from one.
	for range 10 {
		s := newScene()
		tr.Observe(s.img, s.column(colLeft, 0, 6, levels(6, -1, 60, 215)))
	}
	for i := range 30 {
		s := newScene()
		l := levels(6, -1, 60, 215)
		switch i % 3 {
		case 0:
			l[0] = 215
		case 1:
			l[3] = 215
		default:
			l[0], l[3] = 215, 215 // two at once, a third of the time
		}
		tr.Observe(s.img, s.column(colLeft, 0, 6, l))
	}
	ev := tr.Evidence()
	if len(ev) == 0 || ev[0].Shared == 0 {
		t.Fatalf("this case must produce shared frames to be testing anything: %v", ev)
	}
	if _, why, ok := tr.Active(); ok {
		t.Fatalf("a list marking several members at once is not being chosen from (%s)", why)
	}
}
