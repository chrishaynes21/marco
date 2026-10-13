package main

// THE CLASSIFIER THE DIAGNOSIS RESTS ON.
//
// `-diagnose` decides nothing — it explains what the production matcher already decided, and the
// report prints both so a disagreement is visible rather than silent. But the explanation is only
// worth reading if its vocabulary is right, and
// [[ADR-182-two-controls-that-could-never-be-found-again]] separates two failures on the strength
// of these six categories. So the two measured geometries are in here as fixtures: get the
// classifier wrong and the ADR's conclusion changes.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// theLabelAndItsButton is `target_6` on `rl-freeplay-1`: a run of the EXIT TO MAIN MENU label and
// the button row around it, as the proposer actually read them.
func theLabelAndItsButton() (label, button observe.Region) {
	return observe.Region{X: 0.4521, Y: 0.5741, Width: 0.0885, Height: 0.0130},
		observe.Region{X: 0.4177, Y: 0.5648, Width: 0.1833, Height: 0.0315}
}

func TestTheLabelAndItsButtonAreTheSameRectangle(t *testing.T) {
	label, button := theLabelAndItsButton()
	if !sameRegion(label, button) {
		t.Fatal("the measured label and button do not agree, so the whole diagnosis of " +
			"target_6 rests on geometry this fixture does not reproduce")
	}
	// WITH THE BUTTON AS A CONTAINER, the button cannot support the label. That is the
	// refusal ADR-182 traced, stated here in the diagnostic's own vocabulary.
	rec := observe.TargetRecord{
		ID: "target_6", Screen: "visual_6",
		Where: []observe.Region{label}, Containers: []observe.Region{button},
	}
	if got := relate(rec, button); got != relMutualIndistinct {
		t.Errorf("the button relates to its own label as %q, want %q",
			got, relMutualIndistinct)
	}
	// AND WITHOUT IT — the state a second press leaves behind — it is support.
	rec.Containers = nil
	if got := relate(rec, button); got != relMutual {
		t.Errorf("with no container recorded the button relates as %q, want %q",
			got, relMutual)
	}
}

// A BACKGROUND SLAB IS NOT A CONTROL, AND THE RELATION CANNOT SAY SO.
//
// `target_2`'s reading is a 778x230 region of stadium; the closest any later look gets is a
// 1920x708 region that swallows it. The classifier's job is to name that shape, not to judge it:
// the judgement is in the pixels and the ADR.
func TestABlockThatSwallowsTheReadingIsNotSupport(t *testing.T) {
	slab := observe.Region{X: 0, Y: 0.4407, Width: 0.4052, Height: 0.2130}
	rec := observe.TargetRecord{
		ID: "target_2", Screen: "visual_2", Where: []observe.Region{slab},
	}
	bigger := observe.Region{X: 0, Y: 0, Width: 1, Height: 0.6556}
	if got := relate(rec, bigger); got != relSwallowsReading {
		t.Errorf("a region 7.6x the reading relates as %q, want %q",
			got, relSwallowsReading)
	}
}

func TestEveryRelationIsReachableAndOrdered(t *testing.T) {
	reading := observe.Region{X: 0.4, Y: 0.4, Width: 0.2, Height: 0.1}
	rec := observe.TargetRecord{Where: []observe.Region{reading}}
	cases := []struct {
		name string
		p    observe.Region
		want relation
	}{
		{"the reading itself", reading, relMutual},
		{"somewhere else entirely",
			observe.Region{X: 0.01, Y: 0.01, Width: 0.05, Height: 0.05}, relNone},
		{"a corner overlap with neither centre inside",
			observe.Region{X: 0.55, Y: 0.45, Width: 0.3, Height: 0.3}, relOverlap},
		{"a fragment in the reading's corner",
			observe.Region{X: 0.41, Y: 0.41, Width: 0.02, Height: 0.02},
			relInsideReading},
	}
	for _, c := range cases {
		if got := relate(rec, c.p); got != c.want {
			t.Errorf("%s relates as %q, want %q", c.name, got, c.want)
		}
	}
	// ORDERED WORST TO BEST, because the report takes the best relation any proposal
	// reaches as that look's whole diagnosis. If the order slipped, a look where the
	// control was found would be described by whatever else was lying around.
	order := []relation{relNone, relOverlap, relInsideReading, relSwallowsReading,
		relMutualIndistinct, relMutual}
	for i := 1; i < len(order); i++ {
		if !(order[i-1] < order[i]) {
			t.Fatalf("%q does not rank below %q", order[i-1], order[i])
		}
	}
}
