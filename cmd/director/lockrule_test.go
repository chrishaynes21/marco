package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// The same rule as the scan below, asserted by RUNNING it rather than by reading it.
//
// The textual scan proves no listed method mentions mu. This proves the property that
// mattered: with the command lock held — which is the state of the service for the
// entire duration of every desktop command — the diagnostics still answer.
//
// A zero Runtime is the whole fixture. Each of these paths ends in "nothing is running,
// nothing is paused", which is the case that used to reach for mu and block: a program
// holding collections set r.activeCollections and returned before the lock, so the
// ordinary command with no collections at all was the one that hung status.
func TestTheControlPlaneAnswersWhileACommandHoldsTheLock(t *testing.T) {
	r := &Runtime{}
	r.mu.Lock()
	defer r.mu.Unlock()

	answered := make(chan string, 3)
	go func() { r.ActiveValues(); answered <- "ActiveValues" }()
	go func() { r.ActiveCollections(); answered <- "ActiveCollections" }()
	go func() { r.AbandonProgram("cancelled"); answered <- "AbandonProgram" }()

	for i := 0; i < 3; i++ {
		select {
		case <-answered:
		case <-time.After(2 * time.Second):
			t.Fatal("a control-plane method blocked while the command lock was held: " +
				"the Director cannot report what it is doing while it is doing it, which " +
				"is the one moment anybody asks")
		}
	}
}

// The control-plane lock rule.
//
//	Control-plane operations must remain responsive even when desktop work is
//	blocked. Long-running work may own a context, but it may not own the service.
//
// Runtime.mu serialises DESKTOP WORK: Handle, HandleClarified, RunOperation, ReadText
// and ReadRegion all hold it for as long as their work takes, because the pipeline
// holds a stateful world builder whose element identity depends on snapshots arriving
// in order. That is correct and must stay.
//
// What must never happen is a CONTROL-PLANE method taking it. Status, cancellation,
// history and the read-only diagnostics are answered while a command is running, and a
// single r.mu in one of them would make every one of them block behind desktop work —
// silently, and only under load.
//
// This is currently true by construction: the status path goes through the tracker,
// the graph and the per-feature histories, each with its own lock. Nothing enforces it
// but this test, which is the point — the regression is one innocent-looking field
// read away.
var controlPlaneMethods = []string{
	"func (r *Runtime) Providers(",
	"func (r *Runtime) AttachedAt(",
	"func (r *Runtime) Graph(",
	"func (r *Runtime) ActiveWait(",
	"func (r *Runtime) Lowerings(",
	"func (r *Runtime) Edits(",
	"func (r *Runtime) Traces(",
	"func (r *Runtime) TraceFor(",
	"func (r *Runtime) OCRUnavailable(",
	// The diagnostics that answer from the running or paused program. Added after all
	// three were found taking mu through a helper: `director status` renders both of
	// the first two on every call, so the control plane went silent for the duration of
	// any desktop command — the regression this rule exists to prevent, reached through
	// a delegation the original scan did not follow.
	"func (r *Runtime) ActiveValues(",
	"func (r *Runtime) ActiveCollections(",
	// Cancellation. The one control-plane method that must answer while a command is in
	// flight, because being in flight is the reason it was called.
	"func (r *Runtime) AbandonProgram(",
}

func TestControlPlaneMethodsNeverTakeTheCommandLock(t *testing.T) {
	sources := map[string]string{}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		sources[e.Name()] = string(b)
	}

	checked := 0
	for _, sig := range controlPlaneMethods {
		found := false
		for name, src := range sources {
			i := strings.Index(src, sig)
			if i < 0 {
				continue
			}
			found = true
			checked++
			if via, takes := takesCommandLock(methodBody(src[i:]), sources, map[string]bool{}); takes {
				t.Errorf("%s in %s takes r.mu%s — that lock is held for the whole duration "+
					"of desktop work, so this method would block behind any running command. "+
					"Use a per-feature lock, or snapshot under that lock and return the copy.",
					strings.TrimPrefix(sig, "func (r *Runtime) "), name, via)
			}
		}
		if !found {
			t.Errorf("%s was not found; the rule cannot be checked against a method that "+
				"has been renamed or removed", sig)
		}
	}
	if checked == 0 {
		t.Fatal("no control-plane methods were checked — the scan is broken, not the rule")
	}
}

// takesCommandLock reports whether a body takes r.mu, directly or through any helper
// on the same receiver that it calls.
//
// The delegation is the whole point. `ActiveValues` and `ActiveCollections` each read
// three clean lines and take no lock at all; the mu was one call down, in the helper
// that reads the paused program. Checking only the method named in the list declared
// them compliant while `director status` blocked behind every running command, which is
// a guard reporting the answer it was written to disprove.
//
// Returns a description of the path, so a failure names the helper rather than leaving
// the reader to find it.
func takesCommandLock(body string, sources map[string]string, seen map[string]bool) (string, bool) {
	if strings.Contains(body, "r.mu.Lock") || strings.Contains(body, "r.mu.RLock") {
		return "", true
	}
	for _, helper := range receiverCalls(body) {
		if seen[helper] {
			continue
		}
		seen[helper] = true
		sig := "func (r *Runtime) " + helper + "("
		for _, src := range sources {
			i := strings.Index(src, sig)
			if i < 0 {
				continue
			}
			if via, takes := takesCommandLock(methodBody(src[i:]), sources, seen); takes {
				return " through " + helper + "()" + via, true
			}
		}
	}
	return "", false
}

// receiverCalls returns the names of methods a body calls on r.
//
// Textual, like methodBody and for the same reason: the rule is about which lock a call
// chain reaches, and every call in this package is written `r.name(`.
func receiverCalls(body string) []string {
	var out []string
	for i := 0; i+2 < len(body); i++ {
		if body[i] != 'r' || body[i+1] != '.' {
			continue
		}
		// Not a call on the receiver if what precedes it is part of a longer identifier
		// (a field named `other.r`, a variable `br`).
		if i > 0 && isIdentByte(body[i-1]) {
			continue
		}
		j := i + 2
		for j < len(body) && isIdentByte(body[j]) {
			j++
		}
		if j < len(body) && body[j] == '(' && j > i+2 {
			out = append(out, body[i+2:j])
		}
	}
	return out
}

func isIdentByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// methodBody returns the source from a function signature to its closing brace.
//
// Brace counting rather than go/ast: the rule is about a textual pattern inside one
// function, and a parser here would be more machinery for the same answer.
func methodBody(src string) string {
	depth := 0
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[:i+1]
			}
		}
	}
	return src
}
