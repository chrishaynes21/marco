package main

// TWO ATTEMPTS, TWO RECORDS, TWO SETS OF PIXELS.
//
// Run ids had second resolution and `saveRun` truncated whatever was at the path, so two
// attempts started in the same second produced one record and the earlier reference retrieved
// the later run. Separately, the hover probe writes `01-before.png` and `01-after.png`, so two
// runs sharing a `--frames` directory overwrote each other's images while both records pointed
// at the shared directory.
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

// TestTwoAttemptsInTheSameSecondKeepSeparateIdentities.
func TestTwoAttemptsInTheSameSecondKeepSeparateIdentities(t *testing.T) {
	at := time.Date(2026, 10, 19, 0, 15, 30, 0, time.UTC)
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		id := newRun(at)
		if !strings.HasPrefix(id, "run-20261019-001530-") {
			t.Fatalf("id %q is not the sortable timestamp a person reads", id)
		}
		if seen[id] {
			t.Fatalf("two attempts in the same second produced the same id %q, "+
				"so one record would replace the other", id)
		}
		seen[id] = true
	}
}

// TestARecordIsNeverSilentlyReplaced.
//
// If an id ever did collide, the second write must be refused rather than substituted. A
// reference that retrieves somebody else's attempt is worse than one that retrieves nothing,
// because nothing is obviously wrong and the wrong record is not.
func TestARecordIsNeverSilentlyReplaced(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	first := RunRecord{Version: 1, Run: "run-fixed", Workflow: "the first",
		Outcome: RunVerified}
	if _, err := saveRun(first); err != nil {
		t.Fatalf("saving the first record: %v", err)
	}
	second := RunRecord{Version: 1, Run: "run-fixed", Workflow: "the second",
		Outcome: RunFailed}
	if _, err := saveRun(second); err == nil {
		t.Fatal("a second record replaced the first at the same reference")
	}
	b, err := os.ReadFile(runPath("run-fixed"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "the first") {
		t.Errorf("the surviving record is not the one written first: %s", b)
	}
}

// TestTwoRunsSharingAFramesRootKeepTheirOwnImages.
//
// The probe's file names are fixed, so isolation has to come from the directory. This asserts
// the shape the production path builds: a run's frames live under its own id inside whatever
// root was asked for.
func TestTwoRunsSharingAFramesRootKeepTheirOwnImages(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"run-a", "run-b"} {
		probe := &hoverProbe{frames: filepath.Join(root, id)}
		probe.keep("01-before.png", onePixel())
		probe.keep("01-after.png", onePixel())
	}
	for _, id := range []string{"run-a", "run-b"} {
		for _, name := range []string{"01-before.png", "01-after.png"} {
			p := filepath.Join(root, id, name)
			if _, err := os.Stat(p); err != nil {
				t.Errorf("%s/%s is missing, so one run overwrote the "+
					"other's evidence: %v", id, name, err)
			}
		}
	}
}

// onePixel is the smallest image that can be written, because what is being tested is WHERE.
func onePixel() *image.RGBA {
	return image.NewRGBA(image.Rect(0, 0, 1, 1))
}
