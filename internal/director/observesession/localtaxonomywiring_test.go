package observesession_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
)

// What kinds of local change become places, and what kinds do not.
//
// The model does not owe every visible change a state. A dropdown that is gone before anybody
// could name it is not somewhere to be; a panel drawn from the same kind of structure as what it
// covers is not something this signature can see at all. Both are legitimate answers, and the
// dangerous one is the third: a change that IS meaningful and is silently absorbed.
//
// This MEASURES and classifies rather than asserting a table. The classification is the output.
// One hard assertion guards the pair that must never swap: primary content replacement is a
// place, and ordinary churn is not.

type verdict string

const (
	detectable        verdict = "DETECTABLE_STATE"
	transientChange   verdict = "TRANSIENT_CHANGE"
	indistinguishable verdict = "INDISTINGUISHABLE_WITH_CURRENT_VOCABULARY"
)

// classify runs a session that alternates between a base surface and one variation of it, and
// reports what the model made of the difference.
//
// `persistent` is the question the taxonomy actually turns on: something shown twice, for long
// enough, has a chance to become a place; something shown once cannot and should not.
func classify(t *testing.T, to screenfixture.Surface, persistent bool) verdict {
	t.Helper()
	base := oneSurface()
	var frames []frame
	if persistent {
		for range 2 {
			frames = append(frames, stayOn(base.Regions(), 5)...)
			frames = append(frames, stayOn(to.Regions(), 5)...)
		}
	} else {
		frames = append(frames, stayOn(base.Regions(), 6)...)
		frames = append(frames, stayOn(to.Regions(), 1)...)
		frames = append(frames, stayOn(base.Regions(), 8)...)
	}
	sh := run(t, script{frames: frames}).Stats.Shadow

	if len(sh.States) > 1 {
		return detectable
	}
	// One state. Either the comparison could not see the difference at all, or it saw it and
	// persistence refused to promote it. The local measurement tells them apart, and this is
	// the reason the profile carries it.
	if sh.Match.LocalReplaced > 0 {
		return transientChange
	}
	return indistinguishable
}

// The taxonomy, measured.
func TestWhichLocalChangesBecomePlaces(t *testing.T) {
	base := oneSurface()
	cases := []struct {
		name       string
		to         screenfixture.Surface
		persistent bool
		// meaning says whether a person would call this going somewhere. It is the column
		// the verdict is read AGAINST: the same verdict is a success on one row and a
		// limit on the next, and a table without it invites the wrong conclusion from
		// both.
		meaning string
	}{
		{"A. the primary content region is replaced", base.ContentReplaced("checkbox"), true,
			"somewhere else — MUST be a place"},
		{"B. a modal of a different kind of structure", base.Overlaid("dialog", 10, 0.34), true,
			"somewhere else"},
		{"C. an overlay made of what it covers", base.Overlaid("list_item", 10, 0.34), true,
			"somewhere else — the recorded limit"},
		{"D. a small transient dropdown", base.Overlaid("menu_item", 4, 0.10), false,
			"not a place — correct to absorb"},
		{"E. a persistent sidebar", base.Beside("tree_item", 12), true,
			"arguably somewhere else"},
		{"F. ordinary churn in a list", base.Churned(4), true,
			"NOT somewhere else — must not be a place"},
		{"G. a viewport scrolling", base.Scrolled(40.5), true,
			"NOT somewhere else — must not be a place"},
	}

	got := map[string]verdict{}
	for _, c := range cases {
		v := classify(t, c.to, c.persistent)
		got[c.name[:1]] = v
		t.Logf("%-44s %-42s %s", c.name, v, c.meaning)
	}

	// The two that must never swap. Everything else above is a finding; these are the claim.
	if got["A"] != detectable {
		t.Errorf("replacing the primary content region is %s; going somewhere inside an "+
			"application would be invisible", got["A"])
	}
	for _, harmless := range []string{"F", "G"} {
		if got[harmless] == detectable {
			t.Errorf("case %s became a place; ordinary use of an application would mint one "+
				"every time somebody scrolled", harmless)
		}
	}
}

// Provider independence, at the level the architecture actually supports.
//
// The screen model reads `StructuralView`, which carries its composition AND where that came
// from. Two providers reporting the same structure must produce the same places — the provenance
// is for diagnostics, and a model that read it would be deciding what a screen IS from who
// happened to be looking at it.
//
// What this does NOT do is fuse detector output into the authoritative world. That separation is
// deliberate (ADR-036) and is preserved here: each source is played on its own, and the claim is
// that the ANSWER is the same, not that the evidence is pooled.
func TestTheSameStructureFromEitherProviderIsTheSamePlaces(t *testing.T) {
	base := oneSurface()
	other := base.ContentReplaced("checkbox")

	bySource := map[observe.StructuralSource][]observe.ScreenState{}
	for _, source := range []observe.StructuralSource{
		observe.StructureFused, observe.StructureDetector,
	} {
		var frames []frame
		for range 2 {
			frames = append(frames, stayOn(base.Regions(), 5)...)
			frames = append(frames, stayOn(other.Regions(), 5)...)
		}
		sh := run(t, script{frames: frames, source: source}).Stats.Shadow
		bySource[source] = sh.States
		if string(sh.Structure) != string(source) {
			t.Errorf("a session fed %s reported its structure as %s; provenance did not "+
				"survive to diagnostics", source, sh.Structure)
		}
	}

	fused, detected := bySource[observe.StructureFused], bySource[observe.StructureDetector]
	if len(fused) != len(detected) {
		t.Fatalf("the same structure produced %d place(s) from the fused world and %d from "+
			"the detector; what a screen IS would depend on who was looking",
			len(fused), len(detected))
	}
	for i := range fused {
		if len(fused[i].Roles) != len(detected[i].Roles) {
			t.Errorf("place %d differs by provider: %v vs %v",
				i, fused[i].Roles, detected[i].Roles)
		}
	}
	t.Logf("%d place(s) from either provider", len(fused))
}

// One provider being absent is not the other being wrong.
//
// `StructureUnobserved` is not an empty screen. A session that could not look must not conclude
// that everything went away, because "there is nothing here" and "I could not see" call for
// opposite responses and only one of them is a place.
func TestAnUnobservedFrameIsNotAnEmptyPlace(t *testing.T) {
	base := oneSurface()
	var frames []frame
	frames = append(frames, stayOn(base.Regions(), 6)...)
	frames = append(frames, blind(4)...)
	frames = append(frames, stayOn(base.Regions(), 6)...)

	sh := run(t, script{frames: frames}).Stats.Shadow
	if n := len(sh.States); n != 1 {
		t.Errorf("a gap in perception produced %d place(s); losing sight of an application "+
			"would be indistinguishable from going somewhere in it", n)
	}
}
