package surface

// A QUESTION NOBODY COULD ANSWER IS NOT A QUESTION.
//
// Found live, on Windows Settings, under production observation. The accessibility tree routinely
// reports a control and its own wrapper at the same bounds, or two pixels apart, and the surface
// evaluator correctly says it cannot choose between them. Offering that as a clarification asks
// somebody to click in a place that does not exist, and their correct press comes back as
// `undiscriminating` — which reads as their mistake.
//
// Every rectangle in this file came off a real screen or a real fixture, so a change that breaks
// one of these breaks a case somebody saw.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The measured geometry, with where each one came from.
var (
	// The pixel-only Stage of Windows Settings: a caption inside a card, 0.1% of the
	// window against 1.6%. This is the case the whole feature exists for.
	theCaption = directorapi.Rect{X: 637, Y: 424, Width: 126, Height: 18}
	theCard    = directorapi.Rect{X: 613, Y: 400, Width: 492, Height: 66}
	// The Bluetooth page under production observation: a button and its own wrapper at
	// identical bounds, with an identical press point.
	theButton  = directorapi.Rect{X: 908, Y: 54, Width: 98, Height: 38}
	theWrapper = directorapi.Rect{X: 908, Y: 54, Width: 98, Height: 38}
	// The Windows Settings page of Experiment 059, recorded: a caption near the LEFT
	// edge of the row it labels, so the row's left slab is 12px wide and its right slab
	// is 674. A clip that kept the first part rather than the largest would find only
	// the 12.
	theNarrowCaption = directorapi.Rect{X: 348, Y: 746, Width: 160, Height: 34}
	theWideRow       = directorapi.Rect{X: 336, Y: 732, Width: 846, Height: 66}
	// The full-window container the live Stage also reported, refused because a region
	// running the window's width has the window's middle and not anything's position.
	theWholeWindow = directorapi.Rect{X: 0, Y: 32, Width: 1920, Height: 1000}
	// The Personalisation panel: three options, two of them two pixels apart.
	theIcon      = directorapi.Rect{X: 994, Y: 246, Width: 24, Height: 24}
	theTile      = directorapi.Rect{X: 910, Y: 242, Width: 112, Height: 90}
	theTilesEdge = directorapi.Rect{X: 908, Y: 240, Width: 116, Height: 94}
)

// opt is one option with the bounds and press point a Prop would offer.
func opt(id string, b directorapi.Rect) Option {
	return Option{ElementID: directorapi.ElementID(id), Bounds: b,
		Point: directorapi.Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2}}
}

// A PRESS HAS TO BE ABLE TO TELL THEM APART.
//
// The four cases that decided the rule.
func TestAPressCouldTellThemApart(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []Option
		want bool
	}{
		{"a caption inside a card",
			[]Option{opt("e28", theCaption), opt("e27", theCard)}, true},
		{
			// Two peer surfaces overlapping in the middle, told apart by a press
			// near either outer edge. Neither middle is in exactly one of them,
			// which is what killed the first version of this rule.
			"two peers that overlap in the middle",
			[]Option{
				opt("word", directorapi.Rect{X: 200, Y: 760, Width: 100, Height: 22}),
				opt("left", directorapi.Rect{X: 100, Y: 750, Width: 250, Height: 60}),
				opt("right", directorapi.Rect{X: 150, Y: 745, Width: 250, Height: 70}),
			}, true,
		},
		{"a caption near the left edge of its row",
			[]Option{opt("e68", theNarrowCaption), opt("e66", theWideRow)}, true},
		{"a control and its wrapper at the same bounds",
			[]Option{opt("e49", theButton), opt("e50", theWrapper)}, false},
		{
			// The same near-miss pair with nothing else on the menu, so the
			// widest strip left clear is 116x2. Wide enough and nowhere near
			// tall enough: a rule that looked at one direction would admit it.
			"two surfaces two pixels apart, on their own",
			[]Option{opt("e59", theTile), opt("e58", theTilesEdge)}, false,
		},
		{"two surfaces two pixels apart",
			[]Option{opt("e60", theIcon), opt("e59", theTile),
				opt("e58", theTilesEdge)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clear, got := aPressCouldTellThemApart(tc.opts)
			if got != tc.want {
				t.Fatalf("aPressCouldTellThemApart = %v (%+v), want %v", got,
					clear, tc.want)
			}
			if !got {
				return
			}
			// AND THE PLACE IT FOUND REALLY IS IN EXACTLY ONE, by the same
			// containment rule a real press is judged by. A rule that reported a
			// place no press could use would be worse than refusing.
			mid := directorapi.Point{X: clear.X + clear.Width/2,
				Y: clear.Y + clear.Height/2}
			inside := 0
			for _, o := range tc.opts {
				if covers(o.Bounds, mid.X, mid.Y) {
					inside++
				}
			}
			if inside != 1 {
				t.Errorf("the clear area %+v has its middle inside %d options",
					clear, inside)
			}
		})
	}
}

// AND THE QUESTION IS NOT OFFERED WHEN NO PRESS COULD ANSWER IT.
//
// Through `Evaluate`, not around it. The admission path is what changed, and a test that built a
// Hypothesis by hand would pass with the production rule deleted.
func TestAQuestionNoPressCouldAnswerIsNotAsked(t *testing.T) {
	// THE ANSWERABLE ONE FIRST, so the refusal below is not simply "it never asks".
	open := ask(stage(answerableStage()...), "installed apps")
	q, ok := open.Question()
	if !ok {
		t.Fatalf("a caption inside a card raised no question: %s — %s",
			open.Decision, open.Explanation)
	}
	// AND ONLY THE DEFENSIBLE SURFACES ARE ON THE MENU.
	//
	// The full-window container is on this Stage and was refused. Offering it as an option
	// would let a demonstration settle on the whole window, which is the press ADR-205's
	// band rule exists to refuse — and it would do so while looking like a successful
	// answer.
	var offered []string
	for _, o := range q.Options {
		offered = append(offered, string(o.ElementID))
	}
	if len(offered) != 2 {
		t.Errorf("the question offers %v, but only two surfaces were defensible", offered)
	}
	for _, o := range offered {
		if o == "e156" {
			t.Errorf("a refused surface is on the menu: %v", offered)
		}
	}

	shut := ask(stage(duplicatedWrapper()...), "installed apps")
	if shutQ, shutOK := shut.Question(); shutOK {
		var where []string
		for _, o := range shutQ.Options {
			where = append(where, string(o.ElementID))
		}
		t.Errorf("surfaces no press could separate were offered as a question "+
			"over %s", strings.Join(where, ", "))
	}
}

// AND THE REASON IS GIVEN, WITHOUT BLAMING ANYBODY.
//
// The owner's requirement, in their words:
//
//	If the candidates cannot be distinguished by a click, refuse with a clear explanation. Do
//	not blame the user for an impossible demonstration.
//
// Driven through `Evaluate`, so deleting the explanation from the evaluator fails this.
func TestAnUnanswerableAmbiguitySaysWhyRatherThanAsking(t *testing.T) {
	h := ask(stage(duplicatedWrapper()...), "installed apps")

	// THE AMBIGUITY IS STILL REAL. Nothing chose, and reporting that it did would be
	// worse than reporting nothing.
	if h.Decision != DecisionAmbiguous {
		t.Fatalf("the ambiguity was reported as %s: %s", h.Decision, h.Explanation)
	}
	for _, want := range []string{"No press could tell them apart",
		"cannot exist rather than one somebody failed to give",
		"better evidence about the surfaces"} {
		if !strings.Contains(h.Explanation, want) {
			t.Errorf("the explanation does not say %q:\n%s", want, h.Explanation)
		}
	}

	// AND AN ANSWERABLE ONE DOES NOT CARRY THE EXCUSE.
	open := ask(stage(answerableStage()...), "installed apps")
	if strings.Contains(open.Explanation, "No press could tell them apart") {
		t.Errorf("an answerable question was explained away:\n%s", open.Explanation)
	}
}

// duplicatedWrapper is the caption-in-a-card Stage with the card reported TWICE at one bounds.
//
// Both halves are measured, and from different runs. The rectangles are the pixel-only Stage of
// Windows Settings, where the card really does contain the caption. The duplication is the
// production-observation pattern from the same application: e49 {908 54 98 38} beside
// e50 {908 54 98 38}, a control and its own wrapper arriving at one rectangle. Put together they
// are the case this rule exists for, built from nothing invented.
func duplicatedWrapper() []*directorapi.Element {
	return []*directorapi.Element{
		prop("e28", "installed apps", theCaption,
			directorapi.SourceStructure, directorapi.SourceOCR),
		prop("e27", "", theCard, directorapi.SourceStructure),
		prop("e27b", "", theCard, directorapi.SourceStructure),
	}
}

// answerableStage is the caption-in-a-card Stage as the live run actually reported it.
//
// With the full-window container, which the live Stage carried and the evaluator refused. It is
// here because a question is built from the DEFENSIBLE surfaces only: admit the refused one and
// the card's clear part is covered by it, so an answerable question becomes an unanswerable one
// and nothing would be asked at all. The filter is load-bearing and this is what holds it.
func answerableStage() []*directorapi.Element {
	return []*directorapi.Element{
		prop("e28", "installed apps", theCaption,
			directorapi.SourceStructure, directorapi.SourceOCR),
		prop("e27", "", theCard, directorapi.SourceStructure),
		prop("e156", "", theWholeWindow, directorapi.SourceStructure),
	}
}

// CONTAINMENT IS HALF-OPEN, HERE AND IN THE ANSWER.
//
// One rule, used by the admission gate and by the judgement of a real press, so the two cannot
// disagree about what being inside means. Half-open because every rectangle in this package is:
// a surface's far edge belongs to whatever comes next, and two surfaces sharing an edge do not
// overlap along it.
func TestContainmentIsHalfOpen(t *testing.T) {
	b := directorapi.Rect{X: 10, Y: 20, Width: 100, Height: 50}
	for _, tc := range []struct {
		name string
		x, y int
		want bool
	}{
		{"the near corner", 10, 20, true},
		{"the last pixel inside", 109, 69, true},
		{"the far edge in x", 110, 40, false},
		{"the far edge in y", 50, 70, false},
		{"just outside the near corner", 9, 20, false},
	} {
		if got := covers(b, tc.x, tc.y); got != tc.want {
			t.Errorf("%s: covers(%+v, %d, %d) = %v, want %v", tc.name, b,
				tc.x, tc.y, got, tc.want)
		}
	}
}
