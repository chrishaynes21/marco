package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// A SCROLL THAT DECLARED ITS WINDOW AND COULD NOT BE MEASURED IN IT IS SAID AT REVIEW, as a press
// is: it would be refused at replay, and the person decides knowing that. ADR-336.
//
// Leaving a scroll out of notMeasured must fail this.
func TestAnUnmeasuredScrollIsSaidAtReview(t *testing.T) {
	got := notMeasured(macroir.Step{Kind: macroir.StepScroll, X: 1, Y: 2, Wheel: "down",
		Count: 1, RelTo: "window"})
	if len(got) == 0 || got[0] != "where in the window the wheel turned" {
		t.Errorf("an unmeasured scroll is reviewed as missing %q", strings.Join(got, ", "))
	}
}
