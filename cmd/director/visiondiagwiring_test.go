package main

import "testing"

// THE DIAGNOSTIC CAN ASK FOR THE PASS IT EXISTS TO REPORT ON.
//
// # Why this needs a test at all
//
// `director vision` could take a fresh pass and could report every candidate, and it could
// not buy the reading. So on an accessibility-blind application it printed 31 accepted
// structural controls and 43 refusals reading `nothing readable inside a non-structural
// class` — which looks exactly like a detector that finds shapes and no words, and is
// instead a detector that was never asked to read. The answer changed the diagnosis: with
// the budget bought, the same frame named eight controls including "Store", "Library" and
// "Buy Now".
//
// This subsystem has produced four separate capabilities that existed and were reachable by
// nothing — a null plugin build, a stale runtime version, a wrong input size, and a
// confidence floor above the model's whole operating range. A flag that silently did not
// arrive would be the fifth, and it would read as "the model cannot name Epic's controls".
//
// The behaviour either side of this seam is covered where it lives:
// TestTheDiagnosticSaysWhatTheReadingMadeOfEachBox and
// TestADetectOnlyPassLeavesEveryCandidateUnread in the vision package prove the provider
// honours the budget. This holds the two lines that carry the request across the service.
func TestTheReadLabelsFlagReachesThePixelPass(t *testing.T) {
	cmdSrc := mustReadSource(t, "visioncmd.go")
	if !containsAll(cmdSrc, `fs.Bool("read-labels"`, "ReadLabels: *readLabels") {
		t.Error("`director vision` no longer passes --read-labels into the payload.\n" +
			"The flag would parse, report nothing and change nothing, which reads as a " +
			"detector that cannot name anything.")
	}

	diagSrc := mustReadSource(t, "visiondiag.go")
	if !containsAll(diagSrc, "observation.WithPixels(region)") {
		t.Error("ReadVision never asks for the pixel pass, so --read-labels cannot buy a " +
			"reading however it is spelled at the CLI")
	}
	// AND THE CHEAP PASS IS STILL THE DEFAULT. A diagnostic that always read would report
	// a naming rate no ordinary sample pays for.
	if !containsAll(diagSrc, "observation.WithVision(region)") {
		t.Error("ReadVision lost its detect-only pass; every diagnostic would now buy " +
			"the expensive reading")
	}
	if !containsAll(diagSrc, "if readLabels {") {
		t.Error("the pixel pass is not gated on readLabels, so the reading is either " +
			"always or never bought")
	}
}
