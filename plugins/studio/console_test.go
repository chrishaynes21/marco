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
	"path/filepath"
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

// AND THE DIRECTOR BESIDE STUDIO IS BUILT WITH IT.
//
// `directorBin` spawns `director.exe` from Studio's OWN directory, so the recorder a window
// uses is the one next to it and not the one next to `marco.exe`. With two copies of Studio in
// the tree — one here, one at the repo root — that makes a stale sibling invisible: every
// binary looks freshly built, the window is current, and the recordings come from yesterday.
//
// It cost the owner several recordings before the contradiction gave it away. Their stream said
// `start_known=1` with `screen_read=0`, which cannot happen — `Identifies` implies `Said` — and
// the only way to see both is a recorder that has never heard of the second field.
//
// So the build script makes all four, and this reads the script. It cannot check what is ON
// disk: these are build outputs, gitignored, and absent on a machine that has not built yet.
// What it can check is that the one command anybody is told to run still makes them.
func TestTheBuildScriptMakesTheDirectorStudioWillSpawn(t *testing.T) {
	raw, err := os.ReadFile("build.cmd")
	if err != nil {
		t.Fatal(err)
	}
	// THE COMMANDS, NOT THE PROSE. The comments in this script quote the commands
	// they explain, so a blob search finds `go build -o director.exe` inside the
	// paragraph about why it is there — and a mutation commenting the real line out
	// survived exactly that way.
	// THE COMMANDS, NOT THE PROSE. The comments in this script quote the
	// commands they explain, so searching the whole file finds
	// `go build -o director.exe` inside the paragraph about why it is there —
	// and a mutation commenting the real line out survived exactly that way.
	var script string
	for _, line := range strings.Split(string(raw), "\n") {
		if t := strings.TrimSpace(line); !strings.HasPrefix(
			strings.ToLower(t), "rem ") {
			script += line + "\n"
		}
	}
	for _, want := range []struct{ text, why string }{
		{`go build -o "%~dp0director.exe"`, "the script does not build a " +
			"director into this directory, so the one Studio spawns is " +
			"whatever was left there last"},
		// FROM THE ROOT MODULE, which is not a detail: `go build` stamps a VCS
		// revision only into a package in the MAIN module, so building the
		// director from in here produced one that could not say what it was —
		// and saying what it is is the whole point of internal/buildid.
		{`pushd "%~dp0..\.."`, "the director is not built from the root " +
			"module, so it carries no revision and cannot say which build " +
			"it is"},
		{`copy /y director.exe "%~dp0..\..\director.exe"`, "the director is not " +
			"put beside marco.exe, so the repo-root Studio spawns a stale one"},
		{`copy /y studio.exe "%~dp0..\..\studio.exe"`, "the window is not put " +
			"beside marco.exe"},
	} {
		if !strings.Contains(script, want.text) {
			t.Errorf("%s (looked for %q)", want.why, want.text)
		}
	}
	// AND IT IS THE ONE THE INSTRUCTIONS NAME. A script nobody is told to run is a
	// script that goes stale with the thing it builds.
	doc, err := os.ReadFile(filepath.Join("..", "..", "CLAUDE.md"))
	if err != nil {
		t.Skip("no CLAUDE.md to check against")
	}
	if !strings.Contains(string(doc), "plugins/studio/build.cmd") {
		t.Error("the build instructions do not name this script, so somebody " +
			"following them builds Studio with a plain `go build` and gets a " +
			"binary that opens no window")
	}
}
