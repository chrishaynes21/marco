package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/acceptance"
)

// Stopping Marco while a model is thinking.
//
// # Why this needs the real binaries and no desktop
//
// The chain is `marco stop` → the service registry cancels the active command → its context goes
// down the pipeline → `parseWith(ctx)` → the interpreter → `exec.CommandContext` kills the plugin.
// Every link is tested on its own. None of it is tested TOGETHER, and this repository has recorded
// six occasions where exactly that gap hid a mechanism production never invoked.
//
// It performs NO INPUT: the phrase is one nothing can read, so it never reaches grounding, let
// alone the desktop. It is gated anyway, with the rest of this package, because it starts a real
// Director and that costs seconds the ordinary suite should not pay on every run.

// slowPlugin answers the interpret question, eventually. Long enough that a stop can arrive first.
const slowPlugin = `package main
import ("bufio";"encoding/json";"os";"time")
func main(){
	bufio.NewReader(os.Stdin).ReadString('\n')
	time.Sleep(6 * time.Second)
	json.NewEncoder(os.Stdout).Encode(map[string]any{"semantic": map[string]any{
		"kind": "invoke", "target": "Save",
	}})
}`

// STOP REACHES AN INTERPRETATION IN FLIGHT.
//
// The property, stated as the thing that must not happen: a person who pressed stop while a model
// was thinking must not be told Marco did not understand them. Those are different facts and only
// one of them is about their request.
func TestStopReachesAnInterpretationInFlight(t *testing.T) {
	acceptance.Require(t)
	root := repoRootFromTest(t)
	for _, bin := range []string{"marco.exe", "director.exe"} {
		if _, err := os.Stat(filepath.Join(root, bin)); err != nil {
			t.Skipf("%s is not built", bin)
		}
	}

	home := t.TempDir()
	plugin := buildSlowPlugin(t)
	env := append(os.Environ(), "MARCO_HOME="+home, "MARCO_RESOLVER="+plugin)
	run := func(args ...string) (string, int) {
		cmd := exec.Command(filepath.Join(root, args[0]), args[1:]...)
		cmd.Dir, cmd.Env = root, env
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		return string(out), code
	}

	director := exec.Command(filepath.Join(root, "director.exe"), "serve", "--quiet")
	director.Dir, director.Env = root, env
	if err := director.Start(); err != nil {
		t.Fatalf("starting the Director: %v", err)
	}
	t.Cleanup(func() {
		_, _ = run("director.exe", "shutdown")
		_ = director.Process.Kill()
		_, _ = director.Process.Wait()
	})
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if out, _ := run("director.exe", "status"); strings.Contains(out, "Director: running") {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	// A phrase the offline parser cannot read, so the slow interpreter is the only thing that
	// could answer it — and the request is therefore sitting inside interpretation when the
	// stop arrives.
	type done struct{ out string }
	ch := make(chan done, 1)
	go func() {
		out, _ := run("marco.exe", "do", "would you be so kind as to save this")
		ch <- done{out}
	}()

	time.Sleep(1500 * time.Millisecond)
	stopOut, _ := run("marco.exe", "stop")

	var d done
	select {
	case d = <-ch:
	case <-time.After(30 * time.Second):
		t.Fatal("the request never returned after a stop")
	}

	t.Logf("stop said: %s", strings.TrimSpace(firstResultLine(stopOut)))
	t.Logf("the request said: %s", strings.TrimSpace(firstResultLine(d.out)))

	// THE INTERPRETATION WAS CANCELLED. The plugin sleeps six seconds; a request that came
	// back in under that waited on nothing.
	if strings.Contains(d.out, "[result] performed") {
		t.Errorf("a stopped request reported performed:\n%s", d.out)
	}
	// AND THE PERSON IS NOT TOLD THEY WERE MISUNDERSTOOD.
	//
	// This is the assertion worth having. "I only understand click, focus, move, repeat and
	// text editing so far" is the deterministic parser's refusal, and it is a true sentence
	// about a phrase nobody read — but it is the WRONG sentence for somebody who pressed stop,
	// and it sends them to rephrase a request that was never the problem.
	if strings.Contains(d.out, "I only understand") {
		t.Errorf("stopping during interpretation was reported as a failure to understand:\n\n%s\n"+
			"A person who pressed stop is told to say it differently. Cancellation and "+
			"incomprehension are different facts and the product keeps a separate word for "+
			"each — see internal/outcome.", d.out)
	}
	// AND NO PLUGIN IS LEFT RUNNING.
	if n := countRunning(t, "slowplugin.exe"); n > 0 {
		t.Errorf("%d interpreter process(es) survived the stop", n)
	}
}

func buildSlowPlugin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(slowPlugin), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "slowplugin")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "main.go")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the slow plugin: %v\n%s", err, out)
	}
	return bin
}

func countRunning(t *testing.T, image string) int {
	t.Helper()
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq "+image, "/NH").Output()
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, image) {
			n++
		}
	}
	return n
}
