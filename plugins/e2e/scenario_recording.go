//go:build windows

package main

// LAYER TWO: THE RECORDING PATH, THROUGH A SCRIPTED RECORDER AND NOT THROUGH A HAND.
//
// # What this layer is and what it is not
//
// It is the half of Studio's flow that no desktop scenario can reach: a demonstration becoming a
// draft, a draft becoming a review, a review becoming a saved routine, and the three refusals
// that stand between a second recording and the loss of the first. Those run against the real
// backend, the real `simplify`, the real `codegen` and the real registry, with the RECORDER
// substituted — which is why they can run at all, and exactly why they prove nothing about the
// hooks.
//
//	THIS LAYER IS NOT EVIDENCE THAT WINDOWS CAPTURED A PHYSICAL GESTURE.
//
// Marco's hooks discard injected input by design. Nothing in this layer touches them, and the
// hook-attribution tests that do — `internal/recorder`'s own — are deliberately left where they
// are rather than pulled in here, because a harness that listed them beside these would be
// building exactly the impression this sentence exists to prevent.
//
// # Why it runs the tests that exist rather than reimplementing them
//
// Because they are already the right tests, written against the real backend, and a second copy
// in a harness would be a second opinion about what Save does. What this adds is the thing they
// did not have: a place in the report. A person asking "is the recording path covered" could not
// previously answer it without reading a package.
//
// THE LIST IS THE CONTRACT. Every name is run explicitly and every one has to have RUN — a
// pattern that matched nothing would otherwise be the quietest possible green, and renaming a
// test would silently remove it from this layer. That is not hypothetical: `-run` with a typo
// passes.

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// recordingTests are the scripted-recorder tests this layer accounts for.
//
// Grouped as the Architect's brief names them: draft → review → Save, exclusion, Record again,
// and replacement protection. Each is named rather than matched by pattern, so that a test which
// disappears takes this layer red with it.
var recordingTests = []struct{ name, about string }{
	// ── DRAFT → REVIEW → SAVE ──
	{"TestStopOpensReviewAndWritesNothing",
		"Stop ends the capture, opens review and writes nothing"},
	{"TestCancelDiscardsTheDraft", "Cancel throws it away with nothing written"},
	{"TestSaveKeepsTheRecordingUnderTheStatedScope",
		"Save writes the play under the scope it was given"},
	{"TestTheWindowRecordsAgainAfterSaving",
		"a saved draft does not leave the window unable to record"},
	{"TestAnUnknownScopeIsRefusedAndTheDraftSurvives",
		"a scope nobody offered is refused and the draft is still there"},
	{"TestReviewSaysWhatThePressMeasured", "review says what the press measured"},
	{"TestReviewExplainsAMeasurementThatFailed",
		"and what it could not measure, in the words a person would use"},
	{"TestARecordingWithNothingInItIsNotOffered",
		"a recording of nothing is not offered for saving"},
	{"TestAnOrdinaryRecordingIsSavedWithoutComplaint",
		"the ordinary case saves without a word"},
	// ── WHAT IS LEFT OUT OF A RECORDING ──
	{"TestPressingStopInStudioDoesNotBecomeAStep",
		"the press that stopped the recording is not in it"},
	{"TestAVisitToStudioLeavesARecordingThatStillSaves",
		"a mid-demonstration look at Studio does not break the recording"},
	{"TestARecordingAndARunExcludeEachOther",
		"one screen: a recording and a run cannot both be in flight"},
	// ── RECORD AGAIN ──
	{"TestRecordAgainStartsWithTheRoutinesOwnNameAndScope",
		"Record again starts with the play's own name, identity and scope"},
	{"TestRecordAgainCannotOverwriteThroughOrdinarySave",
		"and an ordinary Save afterwards does not replace what it came from"},
	{"TestARecordingMadeInAnotherApplicationSaysSoBeforeSave",
		"a re-recording made elsewhere says so, because Save would succeed"},
	{"TestRecordAgainKeepsTheRoutineItMatchedWhileTheLibraryChanges",
		"the play it matched cannot turn into a namesake"},
	// ── REPLACEMENT PROTECTION ──
	{"TestSavingOverARoutineIsRefusedAndLeavesItAlone",
		"keeping a recording over an existing play is refused"},
	{"TestSavingUnderAnotherNameKeepsBoth", "Save as keeps both"},
	{"TestReplacingChangesOnlyTheOneChosen",
		"a deliberate replacement changes only the one chosen"},
	{"TestReplacingSomethingElseIsRefused",
		"and agreeing to replace one cannot replace another"},
	// ── WHAT A RECORDING SAYS IT CAPTURED ──
	{"TestStudioStatesCoordinateModeRatherThanInheritingIt",
		"Studio states what it captures rather than inheriting it from a shell"},
	{"TestTheModeIsStatedBeforeAnythingIsRecorded",
		"and states it before the hooks start, not after"},
	{"TestASavedRoutineSaysHowItWasRecorded",
		"the saved play says how it was recorded, and reads back as that"},
	{"TestARoutineThatSaysNothingAboutItsModeIsShownAsSayingNothing",
		"and one that says nothing is shown as saying nothing"},
}

// theRecordingPath runs that list and reports each one.
func theRecordingPath(w *world) result {
	started := time.Now()
	names := make([]string, 0, len(recordingTests))
	for _, t := range recordingTests {
		names = append(names, t.name)
	}
	pattern := "^(" + strings.Join(names, "|") + ")$"
	dir := filepath.Join(w.machine.Root, "plugins", "studio")
	w.art.step("running %d scripted-recorder tests in %s", len(names),
		rel(w.machine.Root, dir))

	cmd := exec.Command("go", "test", "-json", "-count=1", "-run", pattern, ".")
	cmd.Dir = dir
	out, runErr := cmd.CombinedOutput()
	w.art.keep("recording-tests.json", out)

	ran, failed := map[string]bool{}, map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var e struct {
			Action, Test, Output string
		}
		if err := json.Unmarshal([]byte(line), &e); err != nil || e.Test == "" {
			continue
		}
		switch e.Action {
		case "pass":
			ran[e.Test] = true
		case "fail":
			ran[e.Test] = true
			failed[e.Test] = "failed"
		case "skip":
			// A SKIP IS NOT A PASS. A test that skipped itself established
			// nothing, and counting it would make a machine without a browser
			// look like a machine where the page was checked.
			failed[e.Test] = "skipped itself: " + strings.TrimSpace(e.Output)
		}
	}
	var missing, broke []string
	for _, t := range recordingTests {
		switch {
		case failed[t.name] != "":
			broke = append(broke, fmt.Sprintf("%s (%s)", t.name, failed[t.name]))
		case !ran[t.name]:
			missing = append(missing, t.name)
		}
	}
	switch {
	case len(missing) > 0:
		return fail(started, fmt.Sprintf("%d of the %d tests this layer accounts for "+
			"did not run at all: %s. A name that matches nothing is the quietest "+
			"possible green", len(missing), len(recordingTests),
			strings.Join(missing, ", ")))
	case len(broke) > 0:
		return fail(started, fmt.Sprintf("%s", strings.Join(broke, "; ")))
	case runErr != nil:
		return fail(started, fmt.Sprintf("every named test passed and `go test` still "+
			"reported a failure (%v), so something else in the package did not "+
			"build or did not pass", runErr))
	}
	// THE NUMBER IS THE NAMED LIST'S, not the count of things that passed.
	//
	// `ran` also holds subtests, so reporting its length would quietly inflate the figure
	// above the number of claims this layer actually accounts for — which is the kind of
	// arithmetic a report should not be doing to itself.
	return pass(started,
		fmt.Sprintf("%d named scripted-recorder tests ran and passed: draft → review → "+
			"Save, what a recording leaves out, Record again, and replacement "+
			"protection", len(recordingTests)),
		"the recorder is substituted in all of them, so none of this says the hooks "+
			"captured anything")
}
