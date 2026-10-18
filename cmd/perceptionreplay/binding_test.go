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
	"io"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
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

	a, err := foldRange(frames, regions, readable, nil, nil, 0, half)
	if err != nil {
		t.Fatal(err)
	}
	b, err := foldRange(frames, regions, readable, nil, nil, half, len(frames))
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

// EVERY BINDING DISCRIMINATOR IS REPORTED.
//
// [[Experiment-076-nothing-the-artifact-carries-says-which-screen-this-is]] is a NEGATIVE result
// about four things at once: the score floor, the words, the margin to the runner-up, and the
// number of candidates that qualified. A report that quietly stopped printing one of them would
// read as a result about the other three, and the conclusion drawn from it —
// [[ADR-229-proposed-a-single-qualifying-candidate-is-not-permission]] — would be standing on
// evidence nobody could see.
//
// So this holds the report's shape rather than its numbers. Deleting any of the four sections
// must fail it.
func TestEveryBindingDiscriminatorIsReported(t *testing.T) {
	// Two screens, each with a reading, bound against themselves and against each other:
	// enough that every section has something to say.
	out := []bindingOutcome{
		{
			from:   foldScreen{id: "visual_1", truth: "visual_1", said: read("play", "free")},
			best:   foldScreen{id: "visual_9", truth: "visual_1", said: read("play", "free")},
			result: "bound", correct: true, offered: 4,
			bestSim: 0.9, secondSim: 0.4,
			bothRead: true, textSim: 0.8,
		},
		{
			from:   foldScreen{id: "visual_2", truth: "visual_2", said: read("garage")},
			best:   foldScreen{id: "visual_7", truth: "visual_5", said: read("options")},
			result: "bound", correct: false, offered: 4,
			bestSim: 0.88, secondSim: 0.5,
			bothRead: true, textSim: 0.1, disagree: true, disWhy: "nothing in common",
		},
	}
	// THROUGH THE REPORT, not through the two functions. Written as two direct calls
	// first, and the mutation run found that deleting either one from `printBinding`
	// changed no test: the sections were being exercised and the REPORT was not.
	a := []foldScreen{out[0].from, out[1].from}
	b := []foldScreen{out[0].best, out[1].best}
	got := capture(func() { printBinding("fixture", 900, 450, a, b, out) })
	for _, want := range []string{
		// the words
		"would the WORDS have separated them",
		"wrong, and the words reject it",
		"correct, and the words reject it",
		// the margin
		"would the MARGIN to the runner-up",
		"correct bindings had gaps",
		// the qualifier count, which is the rule BindScreens already acts on
		"how many candidates qualified",
		"qualified of 4 offered",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report no longer says %q:\n%s", want, got)
		}
	}
	// AND THE COST LINE IS THE ONE A READER LOOKS FOR. A discriminator that rejects
	// correct bindings is worse than none, so that number is never implicit.
	if !strings.Contains(got, "<- the cost") {
		t.Errorf("the report does not mark what the words cost:\n%s", got)
	}
}

// read is a screen's reading, from OCR.
func read(words ...string) observe.ScreenText {
	return observe.TextOf(directorapi.SourceOCR, words)
}

// capture is what a print function wrote to stdout.
func capture(f func()) string {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return ""
	}
	os.Stdout = w
	f()
	_ = w.Close()
	os.Stdout = old
	var b strings.Builder
	_, _ = io.Copy(&b, r)
	return b.String()
}
