package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/platform/marcorunner"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// A production cannot put on itself.
//
// # The recursion, and why it is worth a file
//
// Roadmap 34E gave the Theater a Marco runner: an Actor no longer touches a host, it WRITES a
// small program and the Production boundary compiles and runs it. That is the compile gate, and
// it is the point. It also opens one door that direct calls never had — a cast program is
// ordinary Marco, and ordinary Marco may say `do Theater's Activate`.
//
// If it could, every level would be authorised by the one above it: the Director grants one
// permission at the top and an unbounded stack of productions spends it, each one looking
// perfectly legitimate from inside. No refusal in the production vocabulary describes that,
// because nothing was refused.
//
// # Two halves, and why neither is enough alone
//
// The TEXTUAL half lives in the Actor: a cast program names only the concrete act
// (TestACastProgramNamesOnlyTheConcreteAct in internal/platform/theaterhost). That covers the
// programs this Actor writes today and says nothing about the next Actor.
//
// The STRUCTURAL half is here: the act map a cast program can reach does not contain the Theater
// at all. It holds for a program nobody has written yet, and it needs no depth counter to tune —
// the capability is simply not on the stage.

// remembers is a host that records what it was asked and answers "ok".
type remembers struct {
	mu    sync.Mutex
	calls []runtime.HostCall
}

func (h *remembers) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	h.mu.Lock()
	h.calls = append(h.calls, c)
	h.mu.Unlock()
	return "ok", runtime.Absent(), nil
}

func (h *remembers) actions() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []string
	for _, c := range h.calls {
		out = append(out, c.Act+"'s "+c.Action)
	}
	return out
}

// A program run by the cast runner cannot reach the Theater act.
//
// The structural half of the recursion guarantee, tested by asking for the thing directly: a
// program that says `do Theater's Activate` runs, and the Theater host never hears about it.
//
// Deleting the exclusion in withoutTheater must fail this.
func TestACastProgramCannotReEnterTheTheater(t *testing.T) {
	theater := &remembers{}
	hosts := map[string]runtime.Host{
		"OS":            &remembers{},
		"Accessibility": &remembers{},
		"Theater":       theater,
	}

	runner := marcorunner.New(withoutTheater(hosts))
	// The program a malicious or careless Actor would write. It is legal Marco and it
	// compiles — which is exactly why the guarantee cannot be a matter of what gets written.
	program := strings.Join([]string{
		"use theater.",
		"",
		"the Cast is an actor.",
		"",
		"this can Run.",
		"this's Run does...",
		`    the target1 is a Target with Name "Mouse".`,
		"    do Theater's Activate with target1.",
		"    this is ok!",
		"",
		"the App is a script.",
		"",
		"do Cast's Run...",
		"    when ok?",
		`        log "cast: done".`,
		"    or?",
		"        log that's error.",
		"",
	}, "\n")

	// Whether the run succeeds or fails is not the claim; a Theater that was never reached is.
	if _, err := runner.Run(context.Background(), "cast", program); err != nil {
		t.Logf("the program failed, which is fine: %v", err)
	}
	if got := theater.actions(); len(got) != 0 {
		t.Fatalf("a cast program re-entered the Theater: %v\n"+
			"Every level of that recursion is authorised by the one above it, so the "+
			"authority check cannot see it and no refusal describes it.", got)
	}
}

// The cast runner does not share the live act map.
//
// A copy, not an alias. The caller in assistant.go adds "Theater" to its own map immediately
// after building this, and returning the same map would put the act back within reach one line
// later — a bypass introduced by ordinary wiring rather than by anything anyone wrote wrong.
//
// Mutating withoutTheater to `return hosts` must fail this.
func TestTheCastRunnerDoesNotShareTheLiveActMap(t *testing.T) {
	hosts := map[string]runtime.Host{"OS": &remembers{}}

	reachable := withoutTheater(hosts)
	// Exactly what assistant.go does next.
	hosts["Theater"] = &remembers{}

	if _, ok := reachable["Theater"]; ok {
		t.Error("adding the Theater to the live map put it back within a cast program's reach")
	}
	if _, ok := reachable["OS"]; !ok {
		t.Error("the cast runner lost the OS act; a cast program can reach every act but one")
	}
}

// Wiring: a production built the production way actually runs its cast program.
//
// # Why this is a wiring test and not a Theater test
//
// internal/platform/theaterhost proves the boundary refuses to act without a runner. That says
// nothing about whether THIS binary gives it one — and a Theater with no runner performs nothing
// at all, silently, on every machine. The seam is only real if the constructor the program uses
// installs it.
//
// Entered through newTheaterHost, so deleting `.WithRunner(...)` there must fail this.
func TestTheAssembledTheaterRunsItsCastProgram(t *testing.T) {
	accessibility := &remembers{}
	hosts := map[string]runtime.Host{"Accessibility": accessibility}
	hosts["Theater"] = newTheaterHost(hosts, accessibility)

	target := runtime.NewSet()
	target.Put("Name", runtime.Text("Mouse"))
	target.Put("Kind", runtime.Text("button"))

	status, _, err := hosts["Theater"].Invoke(runtime.HostCall{
		Act: "Theater", Action: "Activate",
		Input: runtime.SetVal(target),
		Ctx:   context.Background(),
	})
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}

	// The LOOK reaches the bridge either way; the ACT only reaches it through the runner,
	// because the Actor no longer calls a host to act.
	var acted bool
	for _, a := range accessibility.actions() {
		if a == "Accessibility's Invoke" {
			acted = true
		}
	}
	if !acted {
		t.Fatalf("the assembled Theater performed nothing (status %q, calls %v).\n"+
			"An Actor expresses its action as Marco and the Production boundary runs it, "+
			"so a Theater built without a runner cannot act at all.",
			status, accessibility.actions())
	}
}

// A LEARNED PLAY FINDS AN ACTOR WITHOUT BEING TOLD.
//
// # The live failure
//
// A play was learned, verified 2/2, saved, registered, and the taught phrase resolved. The
// Audience stood on Home, asked for it, and nothing happened.
//
// The Theater is wired unconditionally and casts whichever actor can play the part. Its only actor
// today is accessibility — and that was wired only when `$MARCO_UIA_BRIDGE` was set, which nothing
// tells anybody to do. So the Theater had nobody in it, and a play whose every action is
// `do Theater's Activate` reached a Theater that could not act.
//
// The Director beside it has always found the plugin by looking. `marco` now looks the same way.
//
// # Why this no longer waits for a real plugin
//
// It used to skip when plugins/uia/uia.exe was absent from the tree — which is every fresh
// clone, every CI run, and every machine where nobody had built the provider. A gate that
// silently stands down on the machines most likely to be missing the thing it guards is not a
// gate. It was green for the whole time the discovery it protects did not exist in any shipped
// build.
//
// The three tiers are exercised against a STUB file instead. The discovery asks `os.Stat`, so a
// zero-byte file at the right path is the whole question — and it lets the tier the INSTALLER
// depends on be tested at all: tier 2 looks beside the executable, and under `go test` the
// executable is the test binary, whose directory is a temp dir this test can write into.
//
// Deleting any tier of the discovery must fail this.
func TestALearnedPlayFindsAnActorWithoutBeingTold(t *testing.T) {
	t.Setenv("MARCO_UIA_BRIDGE", "")

	const rel = "plugins/uia/uia.exe"
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	besideExe := filepath.Join(filepath.Dir(exe), filepath.FromSlash(rel))

	// A working directory with nothing in it, so tier 3 cannot answer for tier 2.
	inTempCwd(t, t.TempDir())

	// Nothing anywhere: the honest answer is nothing, not a path that does not exist.
	// Returning a hopeful default here would make "this machine cannot act" arrive later,
	// as a bridge that fails to launch.
	if got := accessibilityBridge(); got != "" {
		t.Fatalf("with no plugin and no bridge named, discovery invented %q", got)
	}

	// TIER 2 — beside the executable. THE tier a packaged install depends on: the bundle
	// unpacks marco.exe into bin\ and uia.exe into bin\plugins\uia\, and nothing tells marco
	// where it is. This tier had never been exercised.
	stub(t, besideExe)
	if got := accessibilityBridge(); got != besideExe {
		t.Fatalf("a provider beside the executable was not found: got %q, want %q.\n"+
			"That is the layout the single-file installer produces, so the Theater "+
			"would have no actor on every installed machine and every learned play "+
			"would silently do nothing.", got, besideExe)
	}
	remove(t, besideExe)

	// TIER 3 — the working directory. The layout a build tree has.
	work := t.TempDir()
	stub(t, filepath.Join(work, filepath.FromSlash(rel)))
	inTempCwd(t, work)
	if got := accessibilityBridge(); got != filepath.FromSlash(rel) {
		t.Errorf("a provider in the working directory was not found: got %q, want %q",
			got, filepath.FromSlash(rel))
	}

	// An explicit choice still wins over both.
	t.Setenv("MARCO_UIA_BRIDGE", "chosen.exe")
	if got := accessibilityBridge(); got != "chosen.exe" {
		t.Errorf("an explicitly named bridge became %q", got)
	}
}

// inTempCwd moves into dir for the rest of the test and restores the old one after.
func inTempCwd(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

// stub writes an empty file at path, creating its directory.
//
// Empty on purpose: the discovery asks os.Stat and nothing else, so a real binary would only
// make the test need a build. What it must NOT be is absent.
func stub(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, nil, 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
}

func remove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove %s: %v", path, err)
	}
}
