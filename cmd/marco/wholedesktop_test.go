package main

// THE WHOLE DESKTOP IS STILL AVAILABLE, AND A PERSON CAN ASK FOR IT.
//
// A recording is of one window now, which is the right default and a change of behaviour. The
// old behaviour is retained for a demonstration that genuinely spans applications, and retained
// means REACHABLE: a capability nothing can reach is the pattern this repository has eight
// recorded cases of.

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// twoWindows and oneWindow state a desktop, so choosing can be exercised without one.
func twoWindows(t *testing.T) {
	t.Helper()
	orchestrator.WindowsForTest(t, []winctx.LiveWindow{
		{Handle: 0x1111, ProcessID: 7001, Image: "arkascended", Title: "ARK",
			Visible: true, OnScreen: true},
		{Handle: 0x2222, ProcessID: 7002, Image: "chrome", Title: "Marco docs",
			Visible: true, OnScreen: true},
	})
}

func oneWindow(t *testing.T) {
	t.Helper()
	orchestrator.WindowsForTest(t, []winctx.LiveWindow{
		{Handle: 0x1111, ProcessID: 7001, Image: "arkascended", Title: "ARK",
			Visible: true, OnScreen: true},
	})
}

func TestTheWholeDesktopCanBeAskedForAndIsNotTheDefault(t *testing.T) {
	// THE DEFAULT, which is the half that matters most: say nothing and one window is
	// recorded.
	watch, said := howMuchToRecord(false)
	if watch != orchestrator.OneWindow {
		t.Errorf("recording with no flag records %v", watch)
	}
	if !strings.Contains(said, "one window") {
		t.Errorf("the sentence does not say what will be recorded: %q", said)
	}
	// AND THE OPT-OUT.
	watch, said = howMuchToRecord(true)
	if watch != orchestrator.WholeDesktop {
		t.Errorf("--whole-desktop records %v", watch)
	}
	if !strings.Contains(said, "whole desktop") {
		t.Errorf("the sentence does not say what will be recorded: %q", said)
	}
}

// AND THE FLAG IS PARSED AND ASSIGNED.
//
// A source check, for the reason `howMuchToRecord` is a separate function at all:
// `runAssistantLearn` ends in `os.Exit`, so the only other way to establish that the flag
// reaches `Deps.Watch` is to run the real recorder on somebody's desktop. What this cannot see
// is whether the flag WORKS, which is what the case above and
// `orchestrator.TestTheRecordingIsConfinedToOneWindowUnlessTheCallerSaysOtherwise` are for; what
// it can see is the wire being there, which is what was missing while the capability existed and
// nothing could ask for it.
func TestTheWholeDesktopFlagIsWiredToTheRecording(t *testing.T) {
	src, err := os.ReadFile("assistant.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`case "--whole-desktop":`,  // parsed
		"howMuchToRecord(desktop)", // and the flag is what decides
		"d.Watch = watch",          // and the decision reaches the recording
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("`marco learn --whole-desktop` is not wired: %s is missing",
				want)
		}
	}
}

// AND WHICH WINDOW IS NAMED, PARSED AND PASSED.
//
// `--window` is how a surface that already knows the answer says so: the overlay passes the
// handle of the window the person was in when they summoned the HUD, and a person at a terminal
// passes a number from the list `marco learn` prints. A source check for the same reason
// `howMuchToRecord` is a separate function: `runAssistantLearn` ends in `os.Exit`, so the only
// other way to establish that the flag reaches `Deps.Window` is to run the real recorder on
// somebody's desktop.
func TestTheChosenWindowFlagIsParsedAndReachesTheRecording(t *testing.T) {
	src, err := os.ReadFile("assistant.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`case a == "--window" && i+1 < len(args):`, // parsed, with its value
		"window = args[i+1]",                       // and the value kept
		"chooseTheWindow(d, window, auto)",         // which decides
		"d.Window = chosen.Target()",               // and reaches the recording
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("`marco learn --window` is not wired: %s is missing", want)
		}
	}
}

// AND AN UNATTENDED RECORDING IS TOLD RATHER THAN ASKED.
//
// `--auto` saves with nobody having looked, so there is nobody to answer a prompt: a question
// there would hang a HUD on something nothing can see. It is refused with the reason instead,
// and the reason says how to fix it.
func TestAnUnattendedRecordingWillNotBeAskedWhichWindow(t *testing.T) {
	d := orchestrator.Deps{}
	twoWindows(t)
	if _, err := chooseTheWindow(d, "", true); err == nil {
		t.Fatal("an unattended recording prompted for a window, so the HUD would " +
			"hang on a question nobody can see")
	} else {
		for _, want := range []string{"--window", "unattended"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal does not say %q: %v", want, err)
			}
		}
	}
	// AND IT IS NOT REFUSED WHEN IT WAS TOLD, which is what makes the rule about the
	// missing answer rather than about unattended recordings.
	got, err := chooseTheWindow(d, "chrome", true)
	if err != nil {
		t.Fatalf("an unattended recording that named its window was refused: %v", err)
	}
	if !strings.Contains(got.Label(), "chrome") {
		t.Errorf("it chose %q", got.Label())
	}
	// AND ONE CANDIDATE IS NOT A CHOICE AND IS TAKEN, attended or not: a machine with a
	// single recordable window has already answered.
	oneWindow(t)
	if _, err := chooseTheWindow(d, "", true); err != nil {
		t.Errorf("the only window on the desktop was refused: %v", err)
	}
}
