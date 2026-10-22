package main

// WHAT HAPPENED BEFORE, READ BACK FROM THE RECORDS THE RUNNER WROTE.
//
// Not from a log this window kept: the run record is the canonical account of a run, and a
// second account kept here would be a second answer to "what happened". So these tests write
// records the way `director` writes them — by their JSON names, into `runs/` under the store —
// and hold what a person is shown about them.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// aRecordOf writes one run record into the store, as the runner would have left it.
func aRecordOf(t *testing.T, a *Acts, name, body string) {
	t.Helper()
	dir := filepath.Join(a.home, "runs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTheRunsThisMachineKeptAreListedNewestFirst(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)

	aRecordOf(t, a, "run-20261020-160545-4006", `{
	  "version": 1, "run": "run-20261020-160545-4006",
	  "workflow": "ark graphics live", "act": "ark graphics act",
	  "application": "arkascended",
	  "started": "2026-10-20T16:05:45-07:00", "ended": "2026-10-20T16:06:17-07:00",
	  "outcome": "refused", "because": "refused — 0 of 2 interaction(s) verified",
	  "actions_sent": 0, "live": true, "verified": 0,
	  "authority": "the person who pressed Play in Studio at 16:05:44"
	}`)
	aRecordOf(t, a, "run-20261019-090000-aaaa", `{
	  "version": 1, "run": "run-20261019-090000-aaaa",
	  "workflow": "ark graphics live", "act": "the audio page",
	  "started": "2026-10-19T09:00:00-07:00", "ended": "2026-10-19T09:00:30-07:00",
	  "outcome": "verified", "because": "", "actions_sent": 1, "verified": 1,
	  "live": true
	}`)
	// AND ONE THE RUNNER NEVER FINISHED WRITING. Reachable: the reference is reserved
	// and `started` written before anything else, so a child killed between the two
	// leaves exactly this.
	aRecordOf(t, a, "run-20261018-120000-bbbb", `{
	  "version": 1, "run": "run-20261018-120000-bbbb",
	  "workflow": "ark graphics live", "started": "2026-10-18T12:00:00-07:00",
	  "ended": "", "outcome": "started",
	  "because": "reserved; this attempt had not finished when the record was last written"
	}`)
	// AND ONE THAT WILL NOT PARSE, which is still a run that happened.
	aRecordOf(t, a, "run-20261017-120000-cccc", `{not json`)

	got := a.History()
	if len(got) != 4 {
		t.Fatalf("%d runs listed, and four records were written: %+v", len(got), got)
	}
	if got[0].Reference != "run-20261020-160545-4006" ||
		got[1].Reference != "run-20261019-090000-aaaa" {
		t.Errorf("the runs are not newest first: %s then %s", got[0].Reference,
			got[1].Reference)
	}

	first := got[0]
	if first.Act != "ark graphics act" || first.Routine != "ark graphics live" ||
		first.Application != "arkascended" || first.Outcome != "refused" ||
		first.Sent != 0 || !first.Live {
		t.Errorf("the record is read back as %+v", first)
	}
	// WHAT IT MEANS AND WHAT TO DO, from the same two functions the live run screen
	// uses, so a run reads the same way an hour later as it did in front of somebody.
	if !strings.Contains(first.Said, "nothing was pressed") {
		t.Errorf("the run does not say what its outcome meant: %q", first.Said)
	}
	if first.Next == "" {
		t.Error("a refused run says nothing about what to do about it")
	}
	// AND WHO AUTHORISED IT, which is the difference between a granted run and one
	// answered key by key.
	if !strings.Contains(first.Authority, "pressed Play") {
		t.Errorf("the authority is %q", first.Authority)
	}
	if first.When == "" {
		t.Error("the run is listed with no time anybody can read")
	}

	// THE ONE THAT NEVER ENDED IS MARKED, and not as a failure: nobody can speak for
	// what happened, which is a different thing from having failed.
	var unfinished *PastRun
	for i := range got {
		if got[i].Reference == "run-20261018-120000-bbbb" {
			unfinished = &got[i]
		}
	}
	if unfinished == nil {
		t.Fatal("the run that never finished is not listed at all")
	}
	if !unfinished.Unfinished {
		t.Error("a record with no end is not marked as unfinished")
	}
	// AND THE UNREADABLE ONE IS LISTED BY ITS NAME, because the alternative is a window
	// that silently has fewer runs in it than the machine does.
	found := false
	for _, h := range got {
		if h.Reference == "run-20261017-120000-cccc" {
			found, _ = true, h
			if h.Outcome != "unreadable" {
				t.Errorf("an unparseable record is listed as %q", h.Outcome)
			}
		}
	}
	if !found {
		t.Error("a record that will not parse is left out of the history")
	}
}

func TestAStoreWithNoRunsIsAnEmptyHistoryAndNotAFailure(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if got := a.History(); len(got) != 0 {
		t.Errorf("%d runs in a store that has never run anything", len(got))
	}
}

// REPORTING A PROBLEM GATHERS THE RUN, AND SENDS NOTHING.
func TestReportingAProblemWritesTheRunAndTheActIntoOneFile(t *testing.T) {
	a := aPlayableAct(t, "the audio page")
	aRecordOf(t, a, "run-20261020-160545-4006", `{
	  "version": 1, "run": "run-20261020-160545-4006",
	  "workflow": "ark graphics live", "act": "the audio page",
	  "started": "2026-10-20T16:05:45-07:00", "outcome": "refused",
	  "because": "refused — 0 of 1 interaction(s) verified", "actions_sent": 0
	}`)
	// A RUN THIS WINDOW WATCHED, so the transcript is part of what it gathers.
	if err := a.Play("the audio page", false, false, ""); err != "" {
		t.Fatalf("Play would not start: %s", err)
	}
	waitForTranscript(t, a)

	r := a.Report("run-20261020-160545-4006")
	if r.Error != "" {
		t.Fatalf("the report was not written: %s", r.Error)
	}
	raw, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatalf("the report it says it wrote cannot be read: %v", err)
	}
	report := string(raw)
	for _, want := range []struct{ text, why string }{
		{"run-20261020-160545-4006", "the report does not name the run, so " +
			"attaching it says nothing about which run it is about"},
		{"0 of 1 interaction(s) verified", "the run's own record is not in it"},
		{"WHAT IT SAID AS IT RAN", "what the run said is not in it"},
		{"THE ACT, AS IT STANDS NOW", "the Act is not in it, and a run refused " +
			"for a reason about the demonstration is explained by the Act"},
		{"the audio tab", "the Act's interactions are not in it"},
	} {
		if !strings.Contains(report, want.text) {
			t.Errorf("%s (looked for %q)", want.why, want.text)
		}
	}
	// AND IT SAYS NOTHING WAS SENT, because nothing was: this window reads the owner's
	// store and has no business posting their desktop anywhere.
	if !strings.Contains(r.Holds, "Nothing has been sent") {
		t.Errorf("the report does not say that nothing was sent: %q", r.Holds)
	}
}

func TestAProblemWithNoRunToReportIsRefused(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if r := a.Report("  "); r.Error == "" {
		t.Errorf("a report was written for no run at all: %+v", r)
	}
}

// aFewRuns writes three records and returns how many the directory then holds.
func aFewRuns(t *testing.T, a *Acts) int {
	t.Helper()
	for _, r := range []string{
		"run-20261020-160545-4006", "run-20261019-090000-aaaa",
		"run-20261018-120000-bbbb",
	} {
		aRecordOf(t, a, r, `{"version":1,"run":"`+r+`","outcome":"refused",
		  "started":"2026-10-20T16:05:45-07:00","ended":"2026-10-20T16:06:17-07:00"}`)
	}
	return 3
}

func runsLeft(t *testing.T, a *Acts) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(a.home, "runs"))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// DELETING A RUN ASKS FIRST, AND THE FIRST PRESS REMOVES NOTHING.
//
// A run record is the only account of what Marco did to somebody's machine and the one thing
// `Report` gathers from, so the thing that deletes must never be the thing somebody clicked
// first. The same two steps "That's it" uses for the other irreversible thing this window
// offers.
func TestDeletingARunAsksBeforeItDoesIt(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	aFewRuns(t, a)

	asked := a.Forget("run-20261019-090000-aaaa", false)
	if asked.Error != "" {
		t.Fatalf("asking was refused: %s", asked.Error)
	}
	if asked.Deleted != 0 || len(runsLeft(t, a)) != 3 {
		t.Fatalf("the first press deleted %d record(s) and %d are left; it must "+
			"remove nothing", asked.Deleted, len(runsLeft(t, a)))
	}
	if !strings.Contains(asked.Asks, "run-20261019-090000-aaaa") {
		t.Errorf("the question does not name what would go: %q", asked.Asks)
	}
	// AND IT SAYS A PROBLEM REPORT IS LEFT ALONE, because that file is the person's
	// and may be the thing they are about to send.
	if !strings.Contains(asked.Asks, "report") {
		t.Errorf("the question says nothing about a problem report already "+
			"written: %q", asked.Asks)
	}

	done := a.Forget("run-20261019-090000-aaaa", true)
	if done.Error != "" || done.Deleted != 1 {
		t.Fatalf("the second press deleted %d record(s): %s", done.Deleted, done.Error)
	}
	if done.Asks != "" {
		t.Error("a deletion that happened is still asking, so the window would " +
			"show a question about something that has gone")
	}
	left := runsLeft(t, a)
	if len(left) != 2 {
		t.Fatalf("%d record(s) left of three, want two: %v", len(left), left)
	}
	for _, n := range left {
		if strings.Contains(n, "aaaa") {
			t.Errorf("the record that was deleted is still there: %v", left)
		}
	}
}

// AND DELETING EVERY RUN IS A DIFFERENT QUESTION, counted over the DIRECTORY.
//
// The window lists the most recent `historyShown`; the directory keeps all of them. A count
// taken from the list would tell somebody they were deleting twenty-five when they were
// deleting four hundred.
func TestDeletingEveryRunCountsTheDirectoryAndNotTheList(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	for i := 0; i < historyShown+7; i++ {
		r := fmt.Sprintf("run-20261020-1605%02d-%04x", i%60, i)
		aRecordOf(t, a, r, `{"version":1,"run":"`+r+`","outcome":"refused",
		  "started":"2026-10-20T16:05:45-07:00","ended":"2026-10-20T16:06:17-07:00"}`)
	}
	if shown := len(a.History()); shown != historyShown {
		t.Fatalf("the window lists %d run(s); this test needs more kept than "+
			"listed to say anything", shown)
	}
	asked := a.Forget("", false)
	if !strings.Contains(asked.Asks, fmt.Sprint(historyShown+7)) {
		t.Errorf("the question counts %q, and the directory holds %d — a count "+
			"from the list would understate what is about to go",
			asked.Asks, historyShown+7)
	}
	if done := a.Forget("", true); done.Deleted != historyShown+7 || done.Kept != 0 {
		t.Fatalf("deleted %d and kept %d of %d", done.Deleted, done.Kept,
			historyShown+7)
	}
	if left := runsLeft(t, a); len(left) != 0 {
		t.Errorf("%d record(s) survived deleting all of them: %v", len(left), left)
	}
}

// AND A REFERENCE IS MATCHED, NEVER JOINED INTO A PATH.
//
// It arrives from the page. A path built from it is a path somebody can aim, and no amount of
// checking the string afterwards is as good as not building one: this reads the directory and
// compares names, so a reference naming something outside it matches nothing by construction.
func TestARunReferenceCannotReachOutsideTheRunsDirectory(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	aFewRuns(t, a)
	// SOMETHING REAL AND PRECIOUS, one directory up from the runs.
	beside := filepath.Join(a.home, "sequences", "keep-me.json")
	if err := os.MkdirAll(filepath.Dir(beside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(beside, []byte(`{"keep":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, aim := range []string{
		filepath.Join("..", "sequences", "keep-me"),
		"../sequences/keep-me",
		"..\\sequences\\keep-me",
		"run-20261019-090000-aaaa/../../sequences/keep-me",
	} {
		got := a.Forget(aim, true)
		if got.Deleted != 0 {
			t.Errorf("%q deleted %d file(s)", aim, got.Deleted)
		}
		if got.Error == "" {
			t.Errorf("%q was not refused", aim)
		}
	}
	if _, err := os.Stat(beside); err != nil {
		t.Fatalf("a file outside the runs directory was reached: %v", err)
	}
	if len(runsLeft(t, a)) != 3 {
		t.Errorf("the run records changed: %v", runsLeft(t, a))
	}
}

// AND A RUN IN FLIGHT IS NOT A PAST RUN.
//
// Its record is being written while this reads the directory, and the runner owns it until it
// has said how it ended. Deleting it would leave a live child writing to a path nothing lists.
func TestARunInFlightCannotBeDeleted(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	aFewRuns(t, a)

	a.mu.Lock()
	a.busy, a.reference = true, "run-20261020-160545-4006"
	a.mu.Unlock()

	if got := a.Forget("run-20261020-160545-4006", true); got.Deleted != 0 ||
		!strings.Contains(got.Error, "in flight") {
		t.Errorf("the run in flight was deleted (%d) or refused without saying "+
			"why: %q", got.Deleted, got.Error)
	}
	// AND NEITHER IS EVERY RUN, which would take it with them.
	if got := a.Forget("", true); got.Deleted != 0 {
		t.Errorf("deleting all removed %d record(s) while a run was in flight",
			got.Deleted)
	}
	if len(runsLeft(t, a)) != 3 {
		t.Errorf("records went while a run was in flight: %v", runsLeft(t, a))
	}
	// AND A DIFFERENT RUN IS STILL SOMEBODY'S TO DELETE.
	if got := a.Forget("run-20261018-120000-bbbb", true); got.Deleted != 1 {
		t.Errorf("a past run could not be deleted while another was in flight: "+
			"%d, %q", got.Deleted, got.Error)
	}
}

// AND A REFERENCE NOTHING WAS SAVED UNDER IS REFUSED, rather than reported as nothing deleted.
func TestDeletingARunThatIsNotThereSaysSo(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	t.Cleanup(a.Close)
	aFewRuns(t, a)
	got := a.Forget("run-19990101-000000-zzzz", false)
	if got.Error == "" {
		t.Error("a reference nothing is kept under was not refused")
	}
	if got.Asks != "" {
		t.Errorf("it asked about deleting something that is not there: %q", got.Asks)
	}
}
