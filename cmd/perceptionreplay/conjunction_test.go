package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// THE CONJUNCTION IS MEASURED AGAINST ITS HALVES, and the harness does not answer a different
// question than the one it claims.
//
// [[Experiment-078-the-conjunction-is-two-terms-wearing-three-hats]] produced a plausible wrong
// answer twice before it produced a right one, and both times the scaffolding was at fault
// rather than the rule. So the properties the measurement rests on are asserted here.
func TestTheConjunctionIsMeasuredAgainstItsHalves(t *testing.T) {
	src, err := os.ReadFile("conjunction.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)

	// FOUR ARMS. A rule reported alone is a number with nothing to be better than, and
	// the whole finding is a comparison: 50.5% against 52.5%, 53.0% and 3.2%.
	// Asserted against the CONSTRUCTION list, not against the file: every arm's name
	// appears in the print block too, so a mutation that dropped one from the list it is
	// built from survived a plain Contains.
	built := s[strings.Index(s, "for _, n := range []string{"):]
	built = built[:strings.Index(built, "}")]
	for _, arm := range []string{
		`"control only"`, `"agreement only"`, `"conjunction"`, `"acted, then left"`,
	} {
		if !strings.Contains(built, arm) {
			t.Errorf("the measurement no longer BUILDS %s, so its conclusion is a "+
				"number with nothing to compare against", arm)
		}
	}

	// EACH ARM WALKS AT ITS OWN PACE. One shared step index would score every rule
	// against the conjunction's step list under its own name.
	// Read AND written by its own name: a mutation that changed only the READ survived an
	// assertion that any `at[name]` appeared, because the write still did.
	if !strings.Contains(s, "at[name] = i + 1") {
		t.Error("the arms do not advance their own step index")
	}
	if !strings.Contains(s, "i := at[name]") {
		t.Error("the arms share a step index, so three rules are being scored at one " +
			"rule's position")
	}

	// THE ADVANCE IS OUTSIDE THE READY BRANCH.
	//
	// The second defect: a destination is established on a LATER look, after the press,
	// when the source screen is gone and the rule is no longer ready for that step —
	// exactly the look a ready-gated advance skips. With it inside, no arm advanced past
	// step one over 449 looks and the expectation term was granted and never earned.
	ready := strings.Index(s, "arm.readyWrong++")
	advance := strings.Index(s, "arm.advanced++")
	if ready < 0 || advance < 0 {
		t.Fatal("the measurement no longer counts both readiness and advancement")
	}
	if !strings.Contains(s, "THE ADVANCE, ON EVERY LOOK AND NOT ONLY WHEN READY") {
		t.Error("the advance is no longer documented as unconditional, which is the " +
			"defect that made the expectation term unmeasurable")
	}

	// THE LEARNING FOLD IS FED THE PRESSES.
	//
	// The first defect: without them a fold identifies no control, every change comes
	// back `unknown`, and the control term cannot hold on any look — which reads as a
	// finding about the rule.
	if !strings.Contains(s, "pressesByLook(dir, frames)") {
		t.Error("the folds are not fed the recorded presses, so the learning fold " +
			"demonstrates no control and the control term can never hold")
	}

	// TRUTH COMES FROM THE ORACLE, not from either fold. A fold scoring itself is a
	// measurement of its own memory.
	if !strings.Contains(s, "oracle.at[look]") {
		t.Error("the measurement no longer takes truth from the whole-recording oracle")
	}
	// AND IT CANNOT PRESS. Nothing in a measurement may reach an actuator.
	for _, forbidden := range []string{"marcoexec", "oshost", "SendInput", "Actionable()"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("the measurement names %q", forbidden)
		}
	}
}

// AND THE REPORT NAMES THE MISSING EVIDENCE RATHER THAN BLAMING THE RULE.
//
//	If it doesn't improve discrimination, identify the missing evidence instead of adding
//	another arbitrary score term.
//
// So two sections are load-bearing: the one that says WHY the conjunction fails — its expectation
// is an arrangement comparison — and the one that says which link of the chain is missing when a
// term could never hold at all.
func TestTheConjunctionReportNamesTheMissingEvidence(t *testing.T) {
	src, err := os.ReadFile("conjunction.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	for _, want := range []string{
		"WHY, AND IT IS NOT THE RULE'S SHAPE",
		"same evidence wearing two hats",
		"TAKING THE EXPECTATION FROM MARCO'S OWN ACT",
		"WHY A TERM COULD NOT BE EVALUATED",
		"AND IT BARELY PROGRESSES",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("the report no longer says %q", want)
		}
	}
	// AND NO THRESHOLD OF ITS OWN. Every number is production's: a measurement that
	// invented one would be the thing the owner ruled out.
	for _, forbidden := range []string{"0.5 ", "0.6 ", "0.75", "threshold :="} {
		if strings.Contains(s, forbidden) {
			t.Errorf("the measurement appears to define its own threshold (%q)", forbidden)
		}
	}
	if !strings.Contains(s, "observe.VisualScreenMatch") {
		t.Error("the measurement does not use production's own screen relation")
	}
}

// ONLY A POINT IS A PRESS, and a press belongs to the look it happened before.
//
// Behavioural, because the two text assertions above cannot see this and a mutation proved it: a
// reader that took every input event as a press would attribute a screen change to a rectangle
// nobody clicked, and a key or a controller button carries no position at all.
func TestOnlyPointingCountsAsAPress(t *testing.T) {
	dir := t.TempDir()
	// Three looks, 100ms apart. Five events: a key, two points, a controller button and a
	// point after the last frame.
	lines := []string{
		`{"intent":"confirm","at_ms":50,"device":"keyboard"}`,
		`{"intent":"point","at_ms":60,"device":"mouse","where":{"x":0.25,"y":0.5}}`,
		`{"intent":"down","at_ms":120,"device":"pad"}`,
		`{"intent":"point","at_ms":150,"device":"mouse","where":{"x":0.75,"y":0.5}}`,
		`{"intent":"point","at_ms":9999,"device":"mouse","where":{"x":0.1,"y":0.1}}`,
	}
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "input.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	frames := []frameRecord{
		{Index: 0, AtMS: 100}, {Index: 1, AtMS: 200}, {Index: 2, AtMS: 300},
	}

	got := pressesByLook(dir, frames)
	if len(got) != len(frames) {
		t.Fatalf("%d look(s) of presses for %d frames", len(got), len(frames))
	}
	// THE KEY AND THE PAD BUTTON ARE NOT PRESSES. Total must be the two points that
	// happened before the last frame.
	total := 0
	for _, p := range got {
		total += len(p)
	}
	if total != 2 {
		t.Errorf("%d press(es) in total, want the 2 pointing events: %+v", total, got)
	}
	// AND EACH BELONGS TO THE LOOK IT HAPPENED BEFORE. A press attributed to a later
	// look is a press the correlator sees after the change it caused.
	if len(got[0]) != 1 || got[0][0].X != 0.25 {
		t.Errorf("look 0 got %+v, want the press at 60ms", got[0])
	}
	if len(got[1]) != 1 || got[1][0].X != 0.75 {
		t.Errorf("look 1 got %+v, want the press at 150ms", got[1])
	}
	if len(got[2]) != 0 {
		t.Errorf("look 2 got %+v; the press at 9999ms is after every frame", got[2])
	}
}
