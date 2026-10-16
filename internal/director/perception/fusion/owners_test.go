package fusion

// ONE RULE, HELD IN AGREEMENT.
//
// Fusion asks which element a word was printed on. Director asks which Prop a request's words
// sit inside. They go through the same `rankOwners` and `rivalOf` now, and these tests are what
// stops the two wrappers drifting: a surface Director nominates and fusion would have refused is
// a surface nobody can explain.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// nested is the shape the question is really about: a word, the row it sits on, the panel behind
// the row, and the window behind everything.
func nested() []*directorapi.Element {
	mk := func(id string, x, y, w, h int) *directorapi.Element {
		return &directorapi.Element{ID: directorapi.ElementID(id), WindowID: "hwnd:1",
			Bounds:  directorapi.Rect{X: x, Y: y, Width: w, Height: h},
			Enabled: true, Visible: true}
	}
	return []*directorapi.Element{
		mk("word", 388, 764, 162, 22),
		mk("row", 98, 750, 444, 56),
		mk("panel", 80, 600, 500, 300),
		mk("window", 0, 0, 1920, 1080),
		mk("elsewhere", 1200, 200, 300, 40),
	}
}

func TestTheExportedRuleAgreesWithFusionsOwn(t *testing.T) {
	els := nested()
	// WHOLLY INSIDE THE WORD'S OWN REGION, which is what makes this the nesting case and
	// not something else. The rule ranks containment FIRST and size only among equals, so a
	// box that clips its tightest container is genuinely better owned by the ancestor that
	// holds all of it — correct, and a different question from this one.
	box := directorapi.Rect{X: 390, Y: 766, Width: 158, Height: 18}

	fused := make([]Fused, len(els))
	for i, el := range els {
		fused[i] = Fused{Element: el}
	}
	text := observation.Text{Box: box, WindowID: "hwnd:1",
		Content: observation.NewText("SETTINGS")}

	mine, scoped := textCandidates(fused, text)
	theirs := Owners(els, box)
	if !scoped {
		t.Fatal("the fixture is not even in scope, so this proves nothing")
	}
	if len(mine) != len(theirs) {
		t.Fatalf("fusion found %d owner(s) and the exported rule found %d",
			len(mine), len(theirs))
	}
	if len(mine) == 0 {
		t.Fatal("neither found an owner for a word plainly inside four of them")
	}
	for i := range mine {
		if mine[i].fused.Element.ID != theirs[i].Element.ID {
			t.Errorf("position %d: fusion says %q, the exported rule says %q",
				i, mine[i].fused.Element.ID, theirs[i].Element.ID)
		}
		if mine[i].containment != theirs[i].Containment {
			t.Errorf("position %d: containment %v vs %v",
				i, mine[i].containment, theirs[i].Containment)
		}
	}
	// AND THE SMALLEST CONTAINER IS FIRST, which is the half of the rule a reordering
	// would silently break while keeping the set identical.
	if theirs[0].Element.ID != "word" {
		t.Errorf("the smallest container did not win: %q", theirs[0].Element.ID)
	}

	// AND THE ANCESTORS ARE MARKED AS SUCH rather than dropped, because an ancestor is not
	// a rival for the word and IS a candidate surface for whatever sits in it.
	for _, o := range theirs[1:] {
		if !o.Ancestor {
			t.Errorf("%q contains the word and the word's own region, and is not "+
				"marked an ancestor", o.Element.ID)
		}
	}

	// AND NEITHER CALLS NESTING AMBIGUOUS.
	_, fusionSays := ambiguousRival(mine)
	_, mineSays := Rivals(theirs)
	if fusionSays != mineSays {
		t.Errorf("fusion says rival=%v and the exported rule says %v", fusionSays, mineSays)
	}
	if mineSays {
		t.Error("a word inside its own region, a row, a panel and a window was called " +
			"ambiguous; every ancestor contains it and none of them is a peer")
	}
}

// AND TWO PEERS ARE AMBIGUOUS TO BOTH.
func TestBothCallTwoComparableContainersAmbiguous(t *testing.T) {
	// Two boxes of nearly the same size, both containing the word.
	els := []*directorapi.Element{
		{ID: "a", WindowID: "hwnd:1", Bounds: directorapi.Rect{X: 380, Y: 760, Width: 180, Height: 30}},
		{ID: "b", WindowID: "hwnd:1", Bounds: directorapi.Rect{X: 378, Y: 758, Width: 190, Height: 32}},
	}
	box := directorapi.Rect{X: 390, Y: 766, Width: 158, Height: 18}

	fused := []Fused{{Element: els[0]}, {Element: els[1]}}
	mine, _ := textCandidates(fused, observation.Text{Box: box, WindowID: "hwnd:1",
		Content: observation.NewText("SETTINGS")})
	theirs := Owners(els, box)

	_, fusionSays := ambiguousRival(mine)
	_, mineSays := Rivals(theirs)
	if !fusionSays || !mineSays {
		t.Errorf("two comparably sized containers: fusion says rival=%v, "+
			"the exported rule says %v; both should", fusionSays, mineSays)
	}
}

// AND A REGION NOTHING CONTAINS HAS NO OWNERS.
func TestNothingOwnsAnUncontainedRegion(t *testing.T) {
	// A box hanging off the bottom-right corner: the window overlaps 0.02 of it, far under
	// the containment rule, and nothing else comes near.
	if got := Owners(nested(), directorapi.Rect{X: 1900, Y: 1070, Width: 100, Height: 100}); len(got) != 0 {
		for _, o := range got {
			t.Logf("  %q contains it at %.3f", o.Element.ID, o.Containment)
		}
		t.Errorf("%d element(s) claim a region almost entirely outside all of them", len(got))
	}
	if got := Owners(nested(), directorapi.Rect{}); got != nil {
		t.Error("an empty region has owners")
	}
	if got := Owners(nil, directorapi.Rect{X: 1, Y: 1, Width: 2, Height: 2}); got != nil {
		t.Error("an empty world has owners")
	}
}
