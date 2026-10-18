package main

import (
	"os"
	"strings"
	"testing"
)

// THE SCREEN-FREE TALLY IS REPORTED BESIDE THE SCOPED ONE.
//
// [[Experiment-077-the-screen-was-not-supporting-context]] is a comparison, and a comparison
// with one side missing is a result about the other side presented as a result about both. Its
// conclusion — that dropping the screen bought no match rate and cost 3323 collisions and 2470
// off-screen matches — is only readable because both tallies print from one pass.
//
// So this holds the report's shape. Deleting the call, or any of the three numbers the
// conclusion rests on, must fail it.
func TestTheScreenFreeTallyIsReportedBesideTheScopedOne(t *testing.T) {
	src, err := os.ReadFile("reacquire.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	// BOTH PRINTS, FROM ONE PASS. Two passes could differ in the fold, and then a
	// difference between the tallies would not be the screen.
	for _, want := range []string{
		"printReacquisition(dir, obs, scores,", "printScreenFreeReacquisition(free,",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("the report no longer calls %s", want)
		}
	}
	if strings.Count(s, "obs.Observe(lk)") != 1 {
		t.Error("the recording is folded more than once, so a difference between the " +
			"two tallies need not be the screen")
	}
	// THE THREE NUMBERS THE CONCLUSION RESTS ON.
	for _, want := range []string{
		// matches made where the control was never demonstrated — the one that takes
		// a single control to be wrong.
		"sc.elsewhere++", "matched on a look of a screen it was NOT demonstrated on",
		// collisions across the whole registry, which is what the screen was buying.
		"collidingPairs(freeSeen)",
		// and the ground truth, which is the only check nobody labelled.
		"sc.verified++",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("the screen-free measurement no longer computes %q", want)
		}
	}
	// AND THE FREE TALLY SCOPES BY NOTHING. A registry filtered by the standing screen
	// would make the two tallies the same measurement printed twice.
	free := s[strings.Index(s, "AND THE SAME LOOK, WITH NO SCREEN"):]
	free = free[:strings.Index(free, "TWO TARGETS, ONE RECTANGLE. At least one is wrong")]
	if strings.Contains(free, "rec.Screen != obs.Screen() {\n\t\t\t\t\tcontinue") {
		t.Error("the screen-free tally skips targets off the standing screen, so it " +
			"is the scoped measurement under another name")
	}
	if !strings.Contains(free, "observe.ReacquireStored(st, lk.Regions)") {
		t.Error("the screen-free tally does not use the screen-independent matcher")
	}
}
