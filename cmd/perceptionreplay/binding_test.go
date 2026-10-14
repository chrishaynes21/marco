package main

// THE ONE PROPERTY THE BINDING EXPERIMENT DEPENDS ON.
//
// [[Experiment-029-binding-a-screen-across-two-independent-folds]] measures whether a screen
// learned by one observer can be recognised by another, and its result means nothing if the two
// folds were ever in contact. The owner was explicit:
//
//	Importantly, don't allow the two folds to share the registry or screen identities.
//
// Sharing would not look like a bug. It would look like an excellent binding rate — which is the
// shape of a measurement that has measured its own plumbing, and this subsystem has produced two
// of those already. So the independence is asserted rather than assumed.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// topLeft and bottomRight are two arrangements no similarity could confuse: the rectangles are in
// different cells of the grid the arrangement is counted over.
func topLeft() []observe.Region {
	var out []observe.Region
	for i := range 12 {
		out = append(out, observe.Region{
			X: 0.02, Y: 0.02 + float64(i)*0.015, Width: 0.12, Height: 0.01,
		})
	}
	return out
}

func bottomRight() []observe.Region {
	var out []observe.Region
	for i := range 12 {
		out = append(out, observe.Region{
			X: 0.80, Y: 0.70 + float64(i)*0.015, Width: 0.12, Height: 0.01,
		})
	}
	return out
}

// syntheticLooks builds a recording of `each` looks of one arrangement followed by `each` of the
// other, with the order reversed for the second half.
func syntheticLooks(each int) ([]frameRecord, [][]observe.Region) {
	var frames []frameRecord
	var regions [][]observe.Region
	add := func(r []observe.Region) {
		frames = append(frames, frameRecord{
			Index: len(frames), AtMS: int64(100 * len(frames)),
			Width: 1920, Height: 1080, WindowGeneration: 1,
		})
		regions = append(regions, r)
	}
	for range each {
		add(topLeft())
	}
	for range each {
		add(bottomRight())
	}
	// THE SECOND HALF MEETS THE SAME TWO SCREENS IN THE OPPOSITE ORDER. If anything were
	// shared, the second fold would reuse the first fold's name for the screen it meets
	// first; independent, it mints its own `visual_1` for the other one.
	for range each {
		add(bottomRight())
	}
	for range each {
		add(topLeft())
	}
	return frames, regions
}

func TestTheBindingFoldsShareNothing(t *testing.T) {
	const each = 8
	frames, regions := syntheticLooks(each)
	half := len(frames) / 2
	// Every synthetic look has pixels; the fixture is about screen identity,
	// not about capture.
	readable := make([]bool, len(frames))
	for i := range readable {
		readable[i] = true
	}

	a, err := foldRange(frames, regions, readable, 0, half)
	if err != nil {
		t.Fatal(err)
	}
	b, err := foldRange(frames, regions, readable, half, len(frames))
	if err != nil {
		t.Fatal(err)
	}

	// Each fold minted two screens and started counting at one, which is only true if
	// neither inherited anything.
	for name, f := range map[string]fold{"fold A": a, "fold B": b} {
		if len(f.arr) != 2 {
			t.Fatalf("%s saw %d screen(s), want 2 — the fixture is not exercising "+
				"two screens", name, len(f.arr))
		}
		if _, ok := f.arr["visual_1"]; !ok {
			t.Errorf("%s has no visual_1, so its counter did not start fresh: %v",
				name, f.arr)
		}
	}

	// AND `visual_1` MEANS A DIFFERENT SCREEN IN EACH, which is the whole point: the
	// name is a session's private counter and carries no claim about the world.
	// The LAST look of each fold's first run, because a screen has to hold for several
	// looks before it is one and the opening looks are deliberately unknown.
	first := a.at[each-1]
	other := b.at[half+each-1]
	if first != "visual_1" || other != "visual_1" {
		t.Fatalf("the folds settled on %q and %q first, want both visual_1", first, other)
	}
	if sim := observe.ArrangementSimilarity(a.arr[first], b.arr[other]); sim >= observe.VisualScreenMatch {
		t.Errorf("the two folds' visual_1 are alike at %.3f, so they are the same "+
			"screen and this fixture cannot show that the names are independent",
			sim)
	}
	// The screens themselves do correspond, under the other name — so the fixture is
	// testing independence of NAMES and not absence of evidence.
	var counterpart observe.VisualScreenID
	for id := range b.arr {
		if id != other {
			counterpart = id
		}
	}
	if sim := observe.ArrangementSimilarity(a.arr[first], b.arr[counterpart]); sim < observe.VisualScreenMatch {
		t.Errorf("fold A's first screen is alike fold B's %s at only %.3f; the two "+
			"folds did see the same pair of screens", counterpart, sim)
	}
}

// middle is alike neither of the corners.
func middle() []observe.Region {
	var out []observe.Region
	for i := range 12 {
		out = append(out, observe.Region{
			X: 0.44, Y: 0.44 + float64(i)*0.004, Width: 0.1, Height: 0.003,
		})
	}
	return out
}

// THE EXPERIMENT'S JUDGEMENT, AND THE THREE WAYS IT MUST NOT SAY "BOUND".
//
// `bind` is what Experiment-029 reports, so a defect here is a defect in the finding rather than
// in the tool. The cases are the ones the recording actually produced: a screen that is there, a
// screen that is NOT there and binds anyway, and two candidates nothing chooses between.
func TestBindingRefusesRatherThanGuessing(t *testing.T) {
	arr := observe.ArrangeLook
	to := []foldScreen{
		{id: "b_1", arr: arr(topLeft()), truth: "oracle_1"},
		{id: "b_2", arr: arr(bottomRight()), truth: "oracle_2"},
	}
	from := []foldScreen{
		{id: "a_1", arr: arr(topLeft()), truth: "oracle_1"},
		{id: "a_2", arr: arr(topLeft()), truth: "oracle_9"},
		{id: "a_3", arr: arr(middle()), truth: "oracle_8"},
	}
	want := []struct {
		result      string
		correct     bool
		counterpart bool
	}{
		// The screen is there and is found.
		{"bound", true, true},
		// THE EXPENSIVE ONE. `oracle_9` is not in the other fold at all, so the only
		// right answer was to refuse — and the relation binds it to the best thing
		// lying around, confidently. Both of Experiment-029's wrong bindings are this.
		{"bound", false, false},
		// Nothing is alike it, which is the honest silence.
		{"unobservable", false, false},
	}
	check(t, bind(from, to), want)

	// AND TWO CANDIDATES NOTHING CHOOSES BETWEEN. Production resolves this silently by
	// taking the better score; a durable binding may not.
	alike := []foldScreen{
		{id: "b_1", arr: arr(topLeft()), truth: "oracle_1"},
		{id: "b_2", arr: arr(topLeft()[:10]), truth: "oracle_2"},
	}
	check(t, bind([]foldScreen{{id: "a_4", arr: arr(topLeft()[:11]), truth: "oracle_1"}},
		alike), []struct {
		result      string
		correct     bool
		counterpart bool
	}{{"ambiguous", false, true}})
}

func check(t *testing.T, got []bindingOutcome, want []struct {
	result      string
	correct     bool
	counterpart bool
}) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d outcome(s), want %d", len(got), len(want))
	}
	for i, w := range want {
		g := got[i]
		if g.result != w.result {
			t.Errorf("%s: result %q (best %.3f, second %.3f), want %q",
				g.from.id, g.result, g.bestSim, g.secondSim, w.result)
		}
		if g.correct != w.correct {
			t.Errorf("%s: correct=%t, want %t", g.from.id, g.correct, w.correct)
		}
		if g.counterpart != w.counterpart {
			t.Errorf("%s: counterpart=%t, want %t — a screen the other fold never "+
				"saw is the case where binding anything at all is the error",
				g.from.id, g.counterpart, w.counterpart)
		}
	}
}
