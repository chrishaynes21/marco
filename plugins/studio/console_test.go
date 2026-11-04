package main

// A CHILD DIRECTOR DOES NOT GET A WINDOW.
//
// `director` is a console executable, so on Windows starting one creates a console window and
// that window takes the foreground. Studio starts three: the backend, a run, and a recording.
// The owner found it the obvious way — "every time I click record, a new shell window is also
// launched" — and the visible rectangle is the lesser half. A recording has just raised the
// person's game; a console appearing a moment later is in front of it, and the recorder then
// photographs a covered window, refuses presses it cannot attribute, and reads nothing on the
// screen about to be clicked.

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// EVERY PLACE THAT STARTS A CHILD HIDES ITS CONSOLE.
//
// Over the source, and by walking the package rather than naming the three files: the next
// spawn site is the one that will be written without this, and a test that listed today's
// would pass while it did.
func TestNoChildDirectorIsStartedWithAConsole(t *testing.T) {
	spawn := regexp.MustCompile(`exec\.Command(Context)?\(`)
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, f := range files {
		name := f.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		src := string(raw)
		starts := len(spawn.FindAllString(src, -1))
		if starts == 0 {
			continue
		}
		found += starts
		// ONE hideConsole PER SPAWN, in the same file. The call is three lines
		// after the command is built in every case, and a file that starts two
		// children and hides one is the shape this is looking for.
		if hid := strings.Count(src, "hideConsole(cmd)"); hid != starts {
			t.Errorf("%s starts %d child process(es) and hides %d console(s), "+
				"so one of them puts a window in front of whatever Studio "+
				"had just brought forward", name, starts, hid)
		}
	}
	if found == 0 {
		t.Fatal("no child process is started anywhere in this package, so this " +
			"test is watching nothing")
	}
}

// AND HIDING IT DOES NOT TAKE THE CHILD'S OUTPUT WITH IT.
//
// CREATE_NO_WINDOW suppresses the console, not the inherited handles — but that is a claim about
// a Windows constant made in a comment, and Studio reads the recorder's whole transcript off
// exactly these pipes. If it were wrong, every recording would report nothing and say the
// recorder had been silent.
func TestAHiddenConsoleStillHandsBackWhatTheChildSaid(t *testing.T) {
	cmd := exec.Command("go", "env", "GOOS")
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("no toolchain to ask: %v", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		t.Error("a child started with its console hidden handed back nothing, so " +
			"Studio would report every recording as silent")
	}
}
