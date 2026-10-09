package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Non-actuating: nothing here captures a window or reads a controller. What is tested is the
// shape of the record and the guards that run before any of that starts.

func TestAGameSessionWillNotWriteIntoTheCorpus(t *testing.T) {
	// The one mistake that matters: unreviewed frames of somebody's game where the
	// benchmark reads from. Checked before a window is acquired, so this needs no desktop.
	for _, out := range []string{"fixtures", "fixtures/vision/v2/rocketleague", "docs/experiments"} {
		code := runCaptureGameSession([]string{
			"--application", "rocketleague", "--session", "s", "--out", out,
		})
		if code != 2 {
			t.Errorf("--out %s should be refused, got exit %d", out, code)
		}
	}
}

func TestAGameSessionNeedsToBeToldWhatToRecord(t *testing.T) {
	if code := runCaptureGameSession([]string{"--session", "s"}); code != 2 {
		t.Errorf("no --application should be refused, got %d", code)
	}
	if code := runCaptureGameSession([]string{"--application", "rocketleague"}); code != 2 {
		t.Errorf("no --session should be refused, got %d", code)
	}
}

func TestTheFrameRateIsBounded(t *testing.T) {
	// A session is a person playing a game for a minute. 0 fps records nothing and 600 fps
	// fills a disk with duplicate frames of a static menu; neither is a mistake worth
	// discovering afterwards.
	for _, fps := range []string{"0", "-1", "61", "1000"} {
		code := runCaptureGameSession([]string{
			"--application", "rocketleague", "--session", "s", "--fps", fps,
			"--out", t.TempDir(),
		})
		if code != 2 {
			t.Errorf("--fps %s should be refused, got exit %d", fps, code)
		}
	}
}

// TestTheRecordSaysNothingAboutWhatIsSelected is the load-bearing claim of this command.
//
// ADR-144's gap is filled by a person annotating frames, not by the recorder guessing. The
// brightness measurement that found the gap is a hypothesis about one game's menu; if the
// recorder wrote it down, the corpus's ground truth and the detector's cue would share an
// ancestor and the benchmark would agree with itself.
//
// So: whatever this command writes, no part of it may name selection.
func TestTheRecordSaysNothingAboutWhatIsSelected(t *testing.T) {
	dir := t.TempDir()
	err := writeSessionRecord(dir,
		gameSession{Session: "s", Application: "rocketleague", Frames: 2, Inputs: 1},
		[]gameFrame{{Index: 0, File: "frames/s-0000.png", AtMS: 0, Width: 1920, Height: 1080},
			{Index: 1, File: "frames/s-0001.png", AtMS: 103, Width: 1920, Height: 1080}},
		[]gameInput{{Intent: "down", AtMS: 51}})
	if err != nil {
		t.Fatalf("writing the session: %v", err)
	}

	forbidden := []string{"select", "highlight", "chosen", "focus", "active_entry", "brightness"}
	for _, name := range []string{"session.json", "frames.jsonl", "input.jsonl"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s was not written: %v", name, err)
		}
		body := strings.ToLower(string(b))
		for _, word := range forbidden {
			// session.json's own note explains that it states nothing about selection,
			// which is the one place the word is allowed to appear.
			if name == "session.json" {
				continue
			}
			if strings.Contains(body, word) {
				t.Errorf("%s mentions %q; recording must not annotate", name, word)
			}
		}
	}
}

func TestTheThreeLogsAreSeparateDocuments(t *testing.T) {
	// Frames, inputs and the session description are separate claims, and an annotation
	// pass will add a fourth file beside them rather than editing any of these. A single
	// merged log would make that impossible without rewriting the evidence.
	dir := t.TempDir()
	if err := writeSessionRecord(dir, gameSession{Session: "s"},
		[]gameFrame{{Index: 0, File: "frames/s-0000.png", AtMS: 7}},
		[]gameInput{{Intent: "confirm", AtMS: 9}}); err != nil {
		t.Fatalf("writing the session: %v", err)
	}
	for _, name := range []string{"session.json", "frames.jsonl", "input.jsonl"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s is missing: %v", name, err)
		}
	}

	// And the times survive exactly as given. They are measured from the clock during the
	// session; a log that rounded or re-derived them would destroy the only thing that
	// makes a press and a frame orderable.
	line, err := os.ReadFile(filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var f gameFrame
	if err := json.Unmarshal(line, &f); err != nil {
		t.Fatalf("a frames.jsonl line must be one JSON object: %v", err)
	}
	if f.AtMS != 7 {
		t.Errorf("frame time %d, want the 7 it was given", f.AtMS)
	}
}

// TestFrameTimesAreReadFromTheClockNotTheSchedule guards the defect this command was written
// to avoid repeating.
//
// `capture-vision-fixture` records offset_ms as index × interval — what the schedule intended.
// For frames alone that is harmless. Here it would mis-order the one pair that matters, since
// encoding a 1920x1080 PNG costs tens of milliseconds and the real gap is not the asked-for
// one. Structural, because reaching this behaviourally means capturing a real window.
func TestFrameTimesAreReadFromTheClockNotTheSchedule(t *testing.T) {
	src, err := os.ReadFile("capturegamesession.go")
	if err != nil {
		t.Fatalf("reading the command: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, "at := time.Since(start).Milliseconds()") {
		t.Error("frame times must be measured from the session clock")
	}
	if strings.Contains(s, "time.Duration(i) * *interval") {
		t.Error("a frame time derived from the schedule is a fiction; measure it")
	}
}

// TestTheGameSessionCommandIsReachable is the wiring claim.
//
// Eight recorded cases in this repository of complete, correct code that production never
// invoked. A capture command nothing dispatches to would be the ninth, and it would look
// exactly like a working feature in every file listing.
func TestTheGameSessionCommandIsReachable(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading the dispatch: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, `case "capture-game-session":`) {
		t.Error("main.go dispatches no capture-game-session verb")
	}
	if !strings.Contains(s, "runCaptureGameSession(os.Args[2:])") {
		t.Error("the capture-game-session verb does not call the command")
	}
}
