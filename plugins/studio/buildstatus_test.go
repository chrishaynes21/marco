package main

// THE RAIL'S BUILD STAMP HAS ITS OWN READ, AND IT ANSWERS WHATEVER IS OFFERED.
//
// It used to be two fields of `Learning`, the Scenes view's poll, so withholding Scenes was a
// choice between a poll running two and a half times a second for a surface nobody can reach and
// a rail that stops saying which build is recording. Studio spawns the `director.exe` beside
// ITSELF, so a stale sibling is invisible without the stamp; CLAUDE.md records that the invisible
// version of it cost the owner a day of recordings.
//
// See [[ADR-288-the-rail-keeps-its-build-stamp-when-scenes-is-withheld]].

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/learnstate"
)

// WITH NOTHING RECORDING, IT IS THIS WINDOW'S OWN REVISION AND NO ALARM.
//
// `buildid.Differ` and not `!Same`: an unstamped binary cannot vouch for a match AND cannot be
// seen to disagree, so a window with no recorder beside it must not redden.
func TestTheRailSaysWhichBuildThisWindowIs(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	t.Setenv("MARCO_STUDIO_SCENES", "")
	a := newActs(t.TempDir())
	a.revision = func() string { return "aaaaaaa" }

	// AND SCENES IS NOT OFFERED, which is the state this exists for.
	if a.Features().Scenes {
		t.Fatal("the fixture offers Scenes, so this says nothing about the rail " +
			"when it is withheld")
	}
	b := a.BuildStatus()
	if b.Build != "aaaaaaa" {
		t.Errorf("the rail reports this window as %q", b.Build)
	}
	if b.Recorder != "" {
		t.Errorf("the rail names a recorder when none is running: %q", b.Recorder)
	}
	if b.Mismatched {
		t.Error("the rail reports a build mismatch with no recorder to mismatch with")
	}
}

// AND IT NAMES A RECORDER OF ANOTHER BUILD, WITH BOTH REVISIONS.
//
// Both values, never a yes/no: "they differ" is what somebody acts on and the two revisions are
// what tell them which one to rebuild.
func TestTheRailNamesARecorderOfAnotherBuild(t *testing.T) {
	a := aWindowWatchingAStream(t, `{"version":`+strconv.Itoa(learnstate.Version)+
		`,"seq":9,"phase":"ready","build":"bbbbbbb",`+
		`"start_known":true,"screen_read":true}`+"\n")

	b := a.BuildStatus()
	if b.Recorder != "bbbbbbb" || b.Build != "aaaaaaa" {
		t.Fatalf("the rail reports recorder=%q window=%q", b.Recorder, b.Build)
	}
	if !b.Mismatched {
		t.Error("two builds that genuinely differ are not reported as a mismatch")
	}
}

// AND A RECORDER THIS BUILD CANNOT READ A WORD OF IS A MISMATCH.
//
// `Learning` reports this as a SENTENCE on the Scenes card and does not set `Mismatched`, so the
// rail stayed quiet about it — and a recorder whose stream this build cannot parse is the
// plainest kind of stale director there is. It is a mismatch here, in the one answer whose whole
// job is to say so.
//
// There is no second revision to show: the stream is the thing that would have carried it.
func TestTheRailMarksARecorderItCannotRead(t *testing.T) {
	a := aWindowWatchingAStream(t, "not a line this build can read\n")

	b := a.BuildStatus()
	if !b.Mismatched {
		t.Fatal("a recorder whose stream this build cannot read is not reported as " +
			"a mismatch, so the rail is quiet about the plainest stale " +
			"director there is")
	}
	if b.Recorder != "" {
		t.Errorf("the rail names a revision it cannot have read: %q", b.Recorder)
	}
	if b.Build != "aaaaaaa" {
		t.Errorf("the rail stopped reporting its own build: %q", b.Build)
	}
}

// aWindowWatchingAStream is a window whose recorder has written this progress stream.
//
// The learn session is put there directly: starting a real one spawns a director, and what this
// is about is the READING. `recordingNow` takes the path off `a.learn` and nothing else.
func aWindowWatchingAStream(t *testing.T, line string) *Acts {
	t.Helper()
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	a.revision = func() string { return "aaaaaaa" }

	path := filepath.Join(t.TempDir(), "progress.jsonl")
	if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.learn = &learning{name: "a scene", progress: path}
	a.mu.Unlock()
	return a
}
