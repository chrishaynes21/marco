package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The ACTIVITY surface, driven through the real handler with both of its sources faked at their
// seams. Nothing here dials a service or starts a process.

// readActivity reads the surface exactly as the page does.
func readActivity(t *testing.T, e *editor) ([]activityRow, string) {
	t.Helper()
	w := httptest.NewRecorder()
	e.handleActivity(w, httptest.NewRequest(http.MethodGet, "/api/activity", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/activity = %d: %s", w.Code, w.Body.String())
	}
	var got struct {
		Entries []activityRow `json:"entries"`
		Why     string        `json:"why"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode /api/activity: %v (%s)", err, w.Body.String())
	}
	return got.Entries, got.Why
}

// noHistory stands in for a Director that is not running: the read fails and nothing starts.
func noHistory(t *testing.T) {
	t.Helper()
	old := directorHistoryEntries
	directorHistoryEntries = func(int) ([]service.HistoryEntry, error) {
		return nil, errors.New("not running")
	}
	t.Cleanup(func() { directorHistoryEntries = old })
}

// ACTIVITY SPEAKS THE SIX WORDS AND NO OTHERS.
//
// The point of a closed vocabulary is that a surface renders all of it. This walks every ActionStatus
// the Director can record, checks the word it becomes is one of the six, and checks each of the six
// has a rendering in the page — so a seventh word cannot appear and one of the six cannot go missing.
//
// Mutation: delete the ActionBlocked arm. `blocked` then reads as `failed`, and the assertion that
// a refusal is not a failure goes red.
func TestActivityRendersTheSixOutcomeWords(t *testing.T) {
	for status, want := range map[directorapi.ActionStatus]outcome.Outcome{
		directorapi.ActionSucceeded:  outcome.Performed,
		directorapi.ActionFailed:     outcome.Failed,
		directorapi.ActionUnverified: outcome.Failed,
		directorapi.ActionCancelled:  outcome.Cancelled,
		directorapi.ActionBlocked:    outcome.Refused,
		directorapi.ActionPending:    outcome.Unavailable,
		directorapi.ActionRunning:    outcome.Unavailable,
	} {
		got := activityOutcome(service.HistoryEntry{Status: string(status)})
		if got != want {
			t.Errorf("%s became %q, want %q", status, got, want)
		}
		if !outcome.Valid(string(got)) {
			t.Errorf("%s became %q, which is not one of the six", status, got)
		}
	}
	// A status nobody here knows still lands inside the vocabulary rather than blank.
	if got := activityOutcome(service.HistoryEntry{Status: "invented_upstream"}); got != outcome.Failed {
		t.Errorf("an unknown status became %q, want failed", got)
	}
	if got := activityOutcome(service.HistoryEntry{Status: "invented_upstream", Success: true}); got != outcome.Performed {
		t.Errorf("an unknown status that succeeded became %q, want performed", got)
	}
	// And the page can draw every one of them.
	for _, o := range outcome.All {
		if !strings.Contains(editPage, string(o)+":'"+string(o)+"'") {
			t.Errorf("the Activity view has no word for %q", o)
		}
		if !strings.Contains(editPage, ".out."+string(o)+"{") {
			t.Errorf("the Activity view has no rendering for %q", o)
		}
	}
}

// ACTIVITY SHOWS THE RUNS THIS PAGE STARTED, even with no service at all.
//
// The half a durable store would not have. `runAccount` is what became of a clicked Run, and on a
// machine where the Director is not running it is the ONLY account of anything Marco did — so a
// surface that read the graph and stopped would show an empty list to somebody who had just
// watched a play run.
//
// Mutation: delete the runAccount loop in handleActivity. This goes red.
func TestActivityShowsRunsStartedFromThisPage(t *testing.T) {
	noHistory(t)
	e := newTestEditor(t)
	id := e.runs.begin("open bluetooth")
	e.runs.finish(id, outcome.Refused, "", "Marco did not recognise the screen")

	rows, why := readActivity(t, e)
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want the one run this page started", rows)
	}
	if rows[0].What != "open bluetooth" || rows[0].Outcome != string(outcome.Refused) {
		t.Errorf("row = %+v, want the run and the word the engine gave it", rows[0])
	}
	if rows[0].From != "here" {
		t.Errorf("row.From = %q — a person cannot tell this row from one Marco recorded", rows[0].From)
	}
	if rows[0].Detail == "" {
		t.Error("the reason Marco gave was dropped")
	}
	if why != "" {
		t.Errorf("why = %q with rows present; the empty state is being shown over a list", why)
	}
}

// A RUN STILL GOING IS NOT AN ENDING.
//
// The defect runaccount.go exists to have fixed, in a new place: a row rendered before the engine
// said anything would carry an outcome nobody had decided.
func TestActivityDoesNotReportARunThatHasNotFinished(t *testing.T) {
	noHistory(t)
	e := newTestEditor(t)
	e.runs.begin("open bluetooth")
	rows, why := readActivity(t, e)
	if len(rows) != 0 {
		t.Fatalf("rows = %+v, want nothing until the run has an ending", rows)
	}
	if why == "" {
		t.Error("an empty Activity list says nothing about why it is empty")
	}
}

// THE TWO EMPTIES ARE DIFFERENT FACTS.
//
// "Marco has not done anything yet" and "Marco is not watching, so it cannot tell you about
// earlier" are different, and a person who sees the wrong one goes looking in the wrong place.
func TestActivitySaysWhichEmptyItIs(t *testing.T) {
	e := newTestEditor(t)

	noHistory(t)
	_, down := readActivity(t, e)

	old := directorHistoryEntries
	directorHistoryEntries = func(int) ([]service.HistoryEntry, error) { return nil, nil }
	t.Cleanup(func() { directorHistoryEntries = old })
	_, quiet := readActivity(t, e)

	if down == "" || quiet == "" {
		t.Fatalf("an empty list with no explanation: down=%q quiet=%q", down, quiet)
	}
	if down == quiet {
		t.Errorf("both empties say %q; a person cannot tell which one they are looking at", down)
	}
}

// THE DIRECTOR'S OWN RECORD IS SHOWN, NEWEST FIRST, IN ITS OWN WORDS.
func TestActivityShowsWhatTheDirectorRecorded(t *testing.T) {
	old := directorHistoryEntries
	now := time.Now()
	directorHistoryEntries = func(int) ([]service.HistoryEntry, error) {
		return []service.HistoryEntry{
			{Timestamp: now.Add(-time.Hour), Phrase: "open settings",
				Status: string(directorapi.ActionSucceeded), Success: true},
			{Timestamp: now, Goal: "open bluetooth", Reason: "the control was not on screen",
				Status: string(directorapi.ActionFailed)},
		}, nil
	}
	t.Cleanup(func() { directorHistoryEntries = old })

	rows, _ := readActivity(t, newTestEditor(t))
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want both entries", rows)
	}
	if rows[0].What != "open bluetooth" {
		t.Errorf("first row is %q; the list is not newest first", rows[0].What)
	}
	if rows[0].Outcome != string(outcome.Failed) || rows[0].Detail == "" {
		t.Errorf("row = %+v, want failed with the Director's own reason", rows[0])
	}
	if rows[1].What != "open settings" || rows[1].Outcome != string(outcome.Performed) {
		t.Errorf("row = %+v, want the person's own phrase and performed", rows[1])
	}
	for _, r := range rows {
		if r.From != "marco" {
			t.Errorf("row.From = %q, want marco", r.From)
		}
		if r.When == "" {
			t.Error("a row with no time on it")
		}
	}
}

// ACTIVITY IS A READ, AND A READ STARTS NOTHING.
//
// The same rule the rest of this surface follows: opening a tab may not pay for a service start.
// Checked at the source rather than by observing a start, because observing one would mean
// starting one.
func TestActivityNeverStartsTheService(t *testing.T) {
	src := readRepoFile(t, "cmd/marco/activity.go")
	if strings.Contains(src, "directorConnect(true)") || strings.Contains(src, "directorReach()") {
		t.Error("the Activity read starts the service; opening a tab must not cost a service start")
	}
	if !strings.Contains(src, "directorConnect(false)") {
		t.Error("the Activity read no longer goes through the non-starting door")
	}
}
