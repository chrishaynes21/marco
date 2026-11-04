package main

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// A DIRECTOR STARTED FOR A PROCESS STOPS WHEN IT HAS GONE AND NOTHING USES IT, and at no
// other time: not while the process lives, however idle, and not while anything is using it,
// however long the process has been gone.
func TestADirectorStartedForAProcessStopsWhenItHasGoneAndNothingUsesIt(t *testing.T) {
	gone := make(chan struct{})
	done := make(chan struct{})
	var mu sync.Mutex
	using := ""
	asked := 0
	stopped := make(chan struct{})
	tryStop := func() (string, bool) {
		mu.Lock()
		defer mu.Unlock()
		asked++
		if using != "" {
			return using, false
		}
		close(stopped)
		return "", true
	}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		stopWhenUnused(gone, done, tryStop, 10*time.Millisecond, func(string) {})
	}()

	// IDLE, BUT ITS PROCESS IS ALIVE: it is not even asked.
	time.Sleep(150 * time.Millisecond)
	mu.Lock()
	if asked != 0 {
		t.Fatalf("asked to stop %d time(s) while its process was still running", asked)
	}
	// GONE, BUT SOMETHING IS USING IT: it is asked, refuses, and keeps running.
	using = `running "open the settings"`
	mu.Unlock()
	close(gone)
	select {
	case <-stopped:
		t.Fatal("stopped while a command was still running")
	case <-time.After(150 * time.Millisecond):
	}
	mu.Lock()
	if asked < 2 {
		t.Fatalf("asked %d time(s) while in use; it should keep asking", asked)
	}
	// GONE AND UNUSED: it stops, and the loop ends.
	using = ""
	mu.Unlock()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("never stopped once its process had gone and nothing was using it")
	}
	<-finished
}

// AN OWNER THAT CANNOT BE FOUND COUNTS AS GONE, so a Director started for nobody findable does
// not run on for ever on their account. And a live one is waited for. (Windows only: elsewhere
// no process can be identified, which is the first case.)
func TestAnOwnerIsWaitedForUntilItExits(t *testing.T) {
	gone, err := ownerExit(owner{pid: 0})
	if err == nil {
		t.Fatal("pid 0 was accepted as an owner")
	}
	select {
	case <-gone:
	default:
		t.Fatal("an owner that cannot be found does not count as gone")
	}

	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Start-Sleep -Seconds 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting an owner: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	gone, err = ownerExit(owner{pid: uint32(cmd.Process.Pid)})
	if err != nil {
		t.Skipf("this platform cannot identify a process: %v", err)
	}
	select {
	case <-gone:
		t.Fatal("a live owner was reported gone")
	case <-time.After(300 * time.Millisecond):
	}
	_ = cmd.Process.Kill()
	select {
	case <-gone:
	case <-time.After(10 * time.Second):
		t.Fatal("the owner's exit was never noticed")
	}
}

// AND runServe WIRES IT: `--for` reaches stopWhenUnused, and the stop it is given is the server's
// ShutdownIfUnused with the runtime's account, so the reading and the decision are one act. Held at the source,
// because runServe binds a listener and blocks, and a behavioural test would have to start a
// real Director.
func TestServeForWiresTheStopToTheServiceAndTheRuntime(t *testing.T) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "serve.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var body *ast.BlockStmt
	for _, d := range parsed.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "runServe" {
			body = fn.Body
		}
	}
	if body == nil {
		t.Fatal("runServe is not in serve.go any more; this guard needs rewriting")
	}
	src := func(n ast.Node) string {
		var b strings.Builder
		_ = printer.Fprint(&b, fset, n)
		return b.String()
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		g, ok := n.(*ast.GoStmt)
		if !ok {
			return true
		}
		if id, ok := g.Call.Fun.(*ast.Ident); !ok || id.Name != "stopWhenUnused" {
			return true
		}
		found = true
		call := src(g.Call)
		for _, want := range []string{"srv.Done()", "tryStop"} {
			if !strings.Contains(call, want) {
				t.Errorf("stopWhenUnused is not given %s: %s", want, call)
			}
		}
		return false
	})
	if !found {
		t.Fatal("runServe never starts stopWhenUnused, so `--for` does nothing")
	}
	whole := src(body)
	for _, want := range []string{"ownerExit(", "srv.ShutdownIfUnused(rt.runtimeInUse)"} {
		if !strings.Contains(whole, want) {
			t.Errorf("runServe does not call %s", want)
		}
	}
}
