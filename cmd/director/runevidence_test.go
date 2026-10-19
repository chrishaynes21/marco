package main

// TWO ATTEMPTS, TWO RECORDS, TWO SETS OF PIXELS.
//
// Run ids had second resolution and `saveRun` truncated whatever was at the path, so two
// attempts started in the same second produced one record and the earlier reference retrieved
// the later run. Separately, the hover probe writes `01-before.png` and `01-after.png`, so two
// runs sharing a `--frames` directory overwrote each other's images while both records pointed
// at the shared directory.
//
// # Why randomness is not the subject here
//
// The first version of this file drew 64 ids and asserted no two were equal, which is a 3%
// chance of failing a correct implementation and — worse — tests the wrong thing. A suffix
// REDUCES collisions; it cannot prevent them. What has to hold is that a collision is survived,
// so every test below FORCES one.
//
// The one thing a reference must do is retrieve the attempt it names.

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestTwoAttemptsProposingOneIdGetTwoNamespaces forces the collision the suffix exists to make
// unlikely, and requires both attempts to come out of it with somewhere of their own.
func TestTwoAttemptsProposingOneIdGetTwoNamespaces(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	root := filepath.Join(t.TempDir(), "frames")
	at := time.Date(2026, 10, 19, 0, 15, 30, 0, time.UTC)

	// ── A PROPOSER THAT CANNOT GET IT RIGHT FIRST TIME ──
	//
	// It answers with the same candidate until that candidate is taken, and only then
	// moves on. So the first attempt gets `run-collide-0`, the second is handed
	// `run-collide-0` as well and has to notice; the sequence is the test.
	proposed := []string{}
	fixed := func(time.Time) string {
		id := "run-collide-" + string(rune('0'+len(proposed)/2))
		proposed = append(proposed, id)
		return id
	}

	first, firstFrames, err := reserveRun(at, root, fixed)
	if err != nil {
		t.Fatalf("reserving the first namespace: %v", err)
	}
	second, secondFrames, err := reserveRun(at, root, fixed)
	if err != nil {
		t.Fatalf("reserving the second namespace: %v", err)
	}
	if first == second {
		t.Fatalf("two attempts both reserved %q, so one record and one set of "+
			"images would replace the other's", first)
	}
	if firstFrames == secondFrames {
		t.Fatalf("both attempts write their pixels into %q", firstFrames)
	}
	// AND THE SECOND ONE TRIED THE FIRST ONE'S ID AND WAS TOLD NO, which is the
	// mechanism rather than the outcome: a reservation that happened to differ for
	// another reason would satisfy the two checks above.
	if len(proposed) < 3 || proposed[1] != proposed[0] {
		t.Fatalf("the second attempt did not propose the first attempt's id, so "+
			"no collision was forced: %v", proposed)
	}

	// AND BOTH RESERVATIONS EXIST, AND SAY SO.
	for _, id := range []string{first, second} {
		b, err := os.ReadFile(runPath(id))
		if err != nil {
			t.Fatalf("the reservation for %q is not retrievable: %v", id, err)
		}
		if !strings.Contains(string(b), string(RunStarted)) {
			t.Errorf("the provisional record for %q does not say it was only "+
				"reserved: %s", id, b)
		}
	}
	for _, d := range []string{firstFrames, secondFrames} {
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			t.Errorf("the frames namespace %q was not claimed: %v", d, err)
		}
	}
}

// TestNeitherAttemptCanWriteTheOthersEvidence is the same forced collision, carried through to
// the two things a reference retrieves: the record and the images.
func TestNeitherAttemptCanWriteTheOthersEvidence(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	root := filepath.Join(t.TempDir(), "frames")
	at := time.Date(2026, 10, 19, 0, 15, 30, 0, time.UTC)
	n := 0
	fixed := func(time.Time) string {
		n++
		if n <= 2 {
			return "run-same"
		}
		return "run-other"
	}
	first, firstFrames, err := reserveRun(at, root, fixed)
	if err != nil {
		t.Fatal(err)
	}
	second, secondFrames, err := reserveRun(at, root, fixed)
	if err != nil {
		t.Fatal(err)
	}

	// EACH WRITES ITS OWN RECORD AND ITS OWN PIXELS.
	if _, err := saveRun(RunRecord{Version: 1, Run: first, Workflow: "the first",
		Outcome: RunVerified}); err != nil {
		t.Fatalf("the first attempt could not write its own record: %v", err)
	}
	if _, err := saveRun(RunRecord{Version: 1, Run: second, Workflow: "the second",
		Outcome: RunFailed}); err != nil {
		t.Fatalf("the second attempt could not write its own record: %v", err)
	}
	for _, d := range []string{firstFrames, secondFrames} {
		probe := &hoverProbe{frames: d}
		probe.keep("01-before.png", onePixel())
		probe.keep("01-after.png", onePixel())
	}

	// AND EACH REFERENCE RETRIEVES ITS OWN ATTEMPT.
	b, err := os.ReadFile(runPath(first))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "the first") {
		t.Errorf("the first reference retrieves the second attempt: %s", b)
	}
	for _, d := range []string{firstFrames, secondFrames} {
		for _, name := range []string{"01-before.png", "01-after.png"} {
			if _, err := os.Stat(filepath.Join(d, name)); err != nil {
				t.Errorf("%s/%s is missing, so one run overwrote the "+
					"other's evidence: %v", d, name, err)
			}
		}
	}
}

// TestARecordCannotBeWrittenWhereNothingWasReserved is the guard from the other side.
//
// `saveRun` opens without O_CREATE, so the file EXISTING is the proof that this attempt holds
// this id. An id nobody reserved has nowhere to write, which is what makes the reservation the
// only way to get a reference at all.
func TestARecordCannotBeWrittenWhereNothingWasReserved(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	// ── A RESERVATION FIRST, SO THE DIRECTORY IS NOT THE THING REFUSING ──
	//
	// Without this the runs directory does not exist yet, so an unreserved write fails
	// for the wrong reason and the test passes with `saveRun` creating files again — a
	// mutation adding O_CREATE back survived exactly this.
	held, _, err := reserveRun(time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := saveRun(RunRecord{Version: 1, Run: held,
		Outcome: RunVerified}); err != nil {
		t.Fatalf("a reserved id could not be written, so this test is not about "+
			"reservation: %v", err)
	}
	if _, err := saveRun(RunRecord{Version: 1, Run: "run-never-reserved",
		Outcome: RunVerified}); err == nil {
		t.Fatal("a record was written for an id nobody reserved, in a directory " +
			"that exists, so the reservation is not what decides who owns a " +
			"reference")
	}
}

// TestAReservationThatCannotClaimThePixelsClaimsNothing.
//
// The two halves are one namespace. A candidate that gets the record and loses the frames
// directory must give the record back, or a later attempt proposing that id is refused a
// reference it could have had — and the abandoned provisional record stays forever.
func TestAReservationThatCannotClaimThePixelsClaimsNothing(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	root := filepath.Join(t.TempDir(), "frames")
	at := time.Date(2026, 10, 19, 0, 15, 30, 0, time.UTC)
	if err := os.MkdirAll(filepath.Join(root, "run-taken"), 0o755); err != nil {
		t.Fatal(err)
	}
	n := 0
	fixed := func(time.Time) string {
		n++
		if n == 1 {
			return "run-taken"
		}
		return "run-free"
	}
	id, _, err := reserveRun(at, root, fixed)
	if err != nil {
		t.Fatal(err)
	}
	if id != "run-free" {
		t.Fatalf("the reservation kept %q, whose frames directory was already "+
			"somebody else's", id)
	}
	if _, err := os.Stat(runPath("run-taken")); err == nil {
		t.Error("the abandoned candidate left its provisional record behind, so " +
			"that reference is now unusable by anybody")
	}
}

// onePixel is the smallest image that can be written, because what is being tested is WHERE.
func onePixel() *image.RGBA {
	return image.NewRGBA(image.Rect(0, 0, 1, 1))
}
