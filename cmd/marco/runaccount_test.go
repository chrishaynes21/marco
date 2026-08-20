package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// The control centre must report what a run ACTUALLY did.
//
// Before Phase 4 it answered `{"ok":true}` the instant the child process existed and rendered
// "running: X" for ever. A play the authority door DECLINED, a play somebody STOPPED, a play that
// REFUSED because it could not recognise the screen, and a play that worked were reported
// identically — and only the last one made the message true.
//
// Nothing here runs `marco do`. The spawn seam is replaced with a process that prints a scripted
// transcript, so these assertions are about how the SURFACE reads the engine, never about input.

// scriptedRun replaces the spawn with a child that prints `lines` and exits with `code`.
func scriptedRun(t *testing.T, code int, lines ...string) {
	t.Helper()
	prev := runSpawn
	runSpawn = func([]string) (*exec.Cmd, io.Reader, error) {
		// `go run` would be slow and needs the network on a cold module cache. The test binary
		// itself is a program that already exists, and `-test.run` with a name matching nothing
		// makes it print a couple of lines and exit 0 — which is not the transcript we need.
		// So: the shell that ships with this repo's toolchain, echoing the script.
		var b strings.Builder
		for _, l := range lines {
			b.WriteString("echo " + l + "\n")
		}
		b.WriteString("exit " + itoaSmall(code) + "\n")
		cmd := exec.Command("sh", "-c", b.String())
		pipe, err := cmd.StdoutPipe()
		if err != nil {
			return nil, nil, err
		}
		cmd.Stderr = cmd.Stdout
		if err := cmd.Start(); err != nil {
			return nil, nil, err
		}
		return cmd, pipe, nil
	}
	t.Cleanup(func() { runSpawn = prev })
}

func itoaSmall(n int) string {
	if n == 0 {
		return "0"
	}
	return string(rune('0' + n))
}

// waitForRun polls the handler the page polls, until the run reports itself finished.
func waitForRun(t *testing.T, e *editor, id string) map[string]any {
	t.Helper()
	for range 200 {
		w := httptest.NewRecorder()
		e.handleRun(w, httptest.NewRequest(http.MethodGet, "/api/run?id="+id, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("GET /api/run = %d: %s", w.Code, w.Body.String())
		}
		got := decodeJSON(t, w.Body.String())
		if done, _ := got["done"].(bool); done {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the run never finished")
	return nil
}

// startRun presses Run on a real registered play and returns the run id.
func startRun(t *testing.T, e *editor, slug string) string {
	t.Helper()
	p := runRow(t, e, slug, true)
	w := postDo(t, e, map[string]string{"slug": p.Slug, "app": p.App, "scope": p.Scope})
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/do = %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON(t, w.Body.String())
	id, _ := got["run"].(string)
	if id == "" {
		t.Fatal("a clicked Run answered with no run id, so the page has nothing to ask about " +
			"and is back to reporting every run as a success")
	}
	return id
}

// TestAClickedRunReportsWhatTheEngineSaid is the claim: the SIX WORDS reach the browser, and the
// one the engine said is the one that arrives.
//
// Every outcome is walked from outcome.All rather than from a list written here — a word added to
// the vocabulary and not handled by this surface must fail HERE.
func TestAClickedRunReportsWhatTheEngineSaid(t *testing.T) {
	for _, want := range outcome.All {
		t.Run(string(want), func(t *testing.T) {
			e := newTestEditor(t)
			authored(t, e, "", false, "open-the-test")
			// Exit non-zero for everything but performed, exactly as the engine does: the
			// point is that the WIRE decides, not the exit code.
			code := 1
			if want == outcome.Performed {
				code = 0
			}
			scriptedRun(t, code, "[route] open-the-test", outcome.Line(want))

			got := waitForRun(t, e, startRun(t, e, "open-the-test"))
			if got["outcome"] != string(want) {
				t.Fatalf("the engine said %q and the control centre reported %q", want, got["outcome"])
			}
		})
	}
}

// A refusal is not a success, and this is the case the old surface got wrong every single time.
func TestARefusedRunIsNotReportedAsHavingRun(t *testing.T) {
	e := newTestEditor(t)
	authored(t, e, "", false, "open-the-test")
	scriptedRun(t, 5, "Marco could not check", outcome.Line(outcome.Refused))

	got := waitForRun(t, e, startRun(t, e, "open-the-test"))
	if got["outcome"] == string(outcome.Performed) {
		t.Fatal("a refused play was reported as having run")
	}
	if got["outcome"] != string(outcome.Refused) {
		t.Fatalf("a refused play reported %q", got["outcome"])
	}
	// And the sentence the engine printed survives, because "refused" alone tells a person
	// nothing they can act on.
	if d, _ := got["detail"].(string); !strings.Contains(d, "could not check") {
		t.Errorf("the refusal lost the engine's own sentence: %q", d)
	}
}

// A run is reported as still going until it is not. Without this the page cannot tell "started"
// from "finished" and is back to flashing one message for ever.
func TestARunIsUnfinishedUntilTheChildEnds(t *testing.T) {
	e := newTestEditor(t)
	authored(t, e, "", false, "open-the-test")
	scriptedRun(t, 0, "[route] open-the-test", "sleeping", outcome.Line(outcome.Performed))

	id := startRun(t, e, "open-the-test")
	// Asked immediately: it may already be finished on a fast machine, but it must never be
	// missing, and it must never claim an outcome it has not been told.
	w := httptest.NewRecorder()
	e.handleRun(w, httptest.NewRequest(http.MethodGet, "/api/run?id="+id, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("a run that was just started is not answerable: %d", w.Code)
	}
	first := decodeJSON(t, w.Body.String())
	if done, _ := first["done"].(bool); !done && first["outcome"] != "" {
		t.Errorf("an unfinished run already claimed the outcome %q", first["outcome"])
	}
	if got := waitForRun(t, e, id); got["outcome"] != string(outcome.Performed) {
		t.Fatalf("the finished run reported %q", got["outcome"])
	}
}

// A child that says nothing and dies is a failure, not a success.
//
// The intake ALWAYS announces, so silence from it means the process never got there.
func TestAChildThatSaysNothingAndFailsIsNotASuccess(t *testing.T) {
	e := newTestEditor(t)
	authored(t, e, "", false, "open-the-test")
	scriptedRun(t, 1, "something went badly wrong")

	got := waitForRun(t, e, startRun(t, e, "open-the-test"))
	if got["outcome"] != string(outcome.Failed) {
		t.Fatalf("a child that died saying nothing reported %q", got["outcome"])
	}
}

// Asking about a run nobody started is a 404, not an invented answer.
func TestAnUnknownRunIsNotAnswered(t *testing.T) {
	e := newTestEditor(t)
	w := httptest.NewRecorder()
	e.handleRun(w, httptest.NewRequest(http.MethodGet, "/api/run?id=nope", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("an unknown run answered %d: %s", w.Code, w.Body.String())
	}
}

// The account forgets. It is not a history — the Director owns what happened over time — and a
// control centre left open must not accumulate a second record of everything Marco has ever done.
func TestTheRunAccountForgetsFinishedRuns(t *testing.T) {
	var a runAccount
	old := a.begin("an old play")
	a.finish(old, outcome.Performed, "", "")
	a.mu.Lock()
	a.runs[old].Started = time.Now().Add(-2 * runMemory)
	a.mu.Unlock()

	// Any new run sweeps.
	a.begin("a new play")
	if _, ok := a.get(old); ok {
		t.Error("a finished run older than the memory window is still being kept")
	}
}

// An unfinished run is NEVER swept, however old. A play can legitimately run for a long time, and
// forgetting it would leave the page asking about an id that had ceased to exist.
func TestALongRunningPlayIsNotForgotten(t *testing.T) {
	var a runAccount
	id := a.begin("a slow play")
	a.mu.Lock()
	a.runs[id].Started = time.Now().Add(-10 * runMemory)
	a.mu.Unlock()

	a.begin("something else")
	if _, ok := a.get(id); !ok {
		t.Error("a run that is still going was forgotten because it started a long time ago")
	}
}

// The run account does not decide anything about the play.
//
// It spawns `marco do` with an explicit identity and reads the answer. A control centre that
// resolved, authorised or scoped anything itself would be the second intake this campaign exists
// to remove — so the argv is asserted to carry the identity and no loose words.
func TestTheControlCentreDecidesNothingAboutTheRun(t *testing.T) {
	e := newTestEditor(t)
	registerLearned(t, e, "settings", true, "open-mouse-settings")
	got := captureRun(t)
	p := runRow(t, e, "open-mouse-settings", true)
	if w := postDo(t, e, map[string]string{"slug": p.Slug, "app": p.App, "scope": p.Scope}); w.Code != http.StatusOK {
		t.Fatalf("POST /api/do = %d", w.Code)
	}
	if len(*got) == 0 || (*got)[0] != "do" {
		t.Fatalf("the control centre launched %q instead of `marco do`", *got)
	}
	for _, a := range (*got)[1:] {
		if !strings.HasPrefix(a, "--") {
			t.Errorf("the argv carries the loose word %q; the surface is describing the play "+
				"rather than naming it", a)
		}
	}
}

// A spawn that could not start is reported, not left pending for ever.
func TestARunThatCouldNotStartIsReported(t *testing.T) {
	e := newTestEditor(t)
	authored(t, e, "", false, "open-the-test")
	var a runAccount
	prev := runSpawn
	runSpawn = func([]string) (*exec.Cmd, io.Reader, error) { return nil, nil, errNoStart }
	t.Cleanup(func() { runSpawn = prev })

	id, err := a.start(routes.Route{Slug: "open-the-test"})
	if err == nil {
		t.Fatal("a spawn that failed was reported as having started")
	}
	rec, ok := a.get(id)
	if !ok || !rec.Done {
		t.Fatal("a run that never started was left unfinished, so the page would poll for ever")
	}
	if rec.Outcome != outcome.Unavailable {
		t.Errorf("a run that never started reported %q; nothing took it, which is what "+
			"unavailable means", rec.Outcome)
	}
	_ = e
}

var errNoStart = &startErr{}

type startErr struct{}

func (*startErr) Error() string { return "could not start marco" }

// decodeJSON reads a handler's answer the way the page does.
func decodeJSON(t *testing.T, body string) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("the handler answered something the page cannot read: %v\n%s", err, body)
	}
	return got
}
