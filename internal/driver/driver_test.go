package driver

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestGolden walks testdata/ and runs every subdirectory that contains a
// program.marco file. Each test compares stdout to expected.txt (or, if
// expected-error.txt exists, the run must fail and stderr-equivalent must
// match).
func TestGolden(t *testing.T) {
	root := filepath.Join("..", "..", "testdata")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		dir := filepath.Join(root, name)
		prog := filepath.Join(dir, "program.marco")
		if _, err := os.Stat(prog); err != nil {
			continue
		}
		t.Run(name, func(t *testing.T) {
			runGoldenCase(t, dir)
		})
	}
}

func runGoldenCase(t *testing.T, dir string) {
	t.Helper()
	prog := filepath.Join(dir, "program.marco")
	expectErrPath := filepath.Join(dir, "expected-error.txt")
	expectOutPath := filepath.Join(dir, "expected.txt")
	expectUnorderedPath := filepath.Join(dir, "expected-unordered.txt")

	var buf bytes.Buffer
	runErr := RunFile(prog, &buf)

	if data, err := os.ReadFile(expectErrPath); err == nil {
		if runErr == nil {
			t.Fatalf("expected error, got success with output:\n%s", buf.String())
		}
		want := strings.TrimRight(string(data), "\n")
		got := strings.TrimRight(runErr.Error(), "\n")
		if !strings.Contains(got, want) {
			t.Fatalf("error mismatch:\n--- got ---\n%s\n--- want substring ---\n%s", got, want)
		}
		return
	}

	if runErr != nil {
		t.Fatalf("run: %v\noutput so far:\n%s", runErr, buf.String())
	}

	// expected-unordered.txt: parallel-execution tests where line order is not
	// deterministic. Compare line multisets instead of byte streams.
	if data, err := os.ReadFile(expectUnorderedPath); err == nil {
		gotLines := splitLines(buf.String())
		wantLines := splitLines(string(data))
		sort.Strings(gotLines)
		sort.Strings(wantLines)
		if !equalLines(gotLines, wantLines) {
			t.Fatalf("unordered output mismatch:\n--- got (sorted) ---\n%s\n--- want (sorted) ---\n%s",
				strings.Join(gotLines, "\n"), strings.Join(wantLines, "\n"))
		}
		return
	}

	want, err := os.ReadFile(expectOutPath)
	if err != nil {
		t.Fatalf("read expected.txt: %v", err)
	}
	if got := buf.String(); got != string(want) {
		t.Fatalf("output mismatch:\n--- got ---\n%s--- want ---\n%s", got, string(want))
	}
}

// TestRunner_RunTests builds and runs the in-tree marco-test-mode sample to
// verify that `marco test` reports per-test pass/fail correctly.
func TestRunner_RunTests(t *testing.T) {
	dir := t.TempDir()
	src := `the Game is an actor.
this can Save.
this's Save does...
    this is ok with input!

this can Crash.
this's Crash does...
    this is failed with error "boom"!

test SaveOk...
    do Game's Save with "x".
    log "save returned".
    expect that is ok.

test CrashFail...
    do Game's Crash.

test ExpectFail...
    do Game's Save with "x".
    expect that is failed.
`
	path := filepath.Join(dir, "tests.marco")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err := RunTests(path, &buf)
	if err == nil {
		t.Fatalf("expected RunTests to report failures; got success:\n%s", buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "PASS SaveOk") {
		t.Errorf("expected PASS SaveOk; got:\n%s", out)
	}
	if !strings.Contains(out, "FAIL CrashFail") {
		t.Errorf("expected FAIL CrashFail; got:\n%s", out)
	}
	if !strings.Contains(out, "FAIL ExpectFail") {
		t.Errorf("expected FAIL ExpectFail; got:\n%s", out)
	}
	if !strings.Contains(out, "expectation failed") {
		t.Errorf("expected expectation diagnostic; got:\n%s", out)
	}
}

func TestCheck_JSONDiagnostic(t *testing.T) {
	dir := t.TempDir()
	src := `the Game is an actor.
this can Save.
this's Save does...
    this is failed!

the App is a script.
do Game's Save...
    when failed?
        log "fail".
    or when bogus?
        log "never".
    or?
`
	path := filepath.Join(dir, "p.marco")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err := Check(path, &buf, true)
	if err == nil {
		t.Fatalf("expected Check to fail on dead-arm; got success: %s", buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, `"code": "dead-arm"`) {
		t.Errorf("expected dead-arm code in JSON; got:\n%s", out)
	}
	if !strings.Contains(out, `"severity": "error"`) {
		t.Errorf("expected severity error; got:\n%s", out)
	}
	if !strings.Contains(out, `"line": 10`) {
		t.Errorf("expected line 10; got:\n%s", out)
	}
}

func TestRunTests_ScopedMockIsolation(t *testing.T) {
	dir := t.TempDir()
	src := `the Game is an actor.
this can Save.
this's Save does...
    this is ok!

test MockedFails...
    mock Game's Save is failed.
    do Game's Save...
        or?
    expect that is failed.

test RealOk...
    do Game's Save, when ok, log "ok".
    expect that is ok.
`
	path := filepath.Join(dir, "tests.marco")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := RunTests(path, &buf); err != nil {
		t.Fatalf("expected RunTests to pass; got %v\n%s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "PASS MockedFails") {
		t.Errorf("expected PASS MockedFails (mock applied); got:\n%s", out)
	}
	if !strings.Contains(out, "PASS RealOk") {
		t.Errorf("expected PASS RealOk (mock not leaked into next test); got:\n%s", out)
	}
}

// TestCheck_JSONMultipleDiagnostics verifies that Check emits ALL collected
// diagnostics in a single JSON array — not just the first. This exercises the
// cross-pass aggregation in compile.Compile and the per-pass collection in
// passes that iterate top-level nodes.
func TestCheck_JSONMultipleDiagnostics(t *testing.T) {
	dir := t.TempDir()
	src := `the Game is an actor.
this can Save.
this's Save does...
    this is failed!

this can Crash.
this's Crash does...
    this is failed!

the App is a script.
do Game's Save...
    when failed?
        log "fail".
    or when bogus?
        log "never".
    or?
do Game's Crash...
    when failed?
        log "boom".
    or when wat?
        log "never".
    or?
`
	path := filepath.Join(dir, "p.marco")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err := Check(path, &buf, true)
	if err == nil {
		t.Fatalf("expected Check to fail; got success: %s", buf.String())
	}
	out := buf.String()
	count := strings.Count(out, `"code": "dead-arm"`)
	if count < 2 {
		t.Errorf("expected at least 2 dead-arm diagnostics, got %d:\n%s", count, out)
	}
}

func TestCheck_OK(t *testing.T) {
	dir := t.TempDir()
	src := `the Game is an actor.
this can Save.
this's Save does...
    this is ok!

the App is a script.
do Game's Save, when ok, log "done".
`
	path := filepath.Join(dir, "ok.marco")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Check(path, &buf, true); err != nil {
		t.Fatalf("expected Check to succeed; got %v\noutput:\n%s", err, buf.String())
	}
	out := strings.TrimSpace(buf.String())
	if out != "[]" {
		t.Errorf("expected empty diagnostics array; got:\n%s", out)
	}
}

func splitLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
