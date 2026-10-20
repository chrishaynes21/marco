package main

// WHAT HAPPENED BEFORE, READ BACK FROM THE RECORDS THE RUNNER WROTE.
//
// Not from a log this window kept: the run record is the canonical account of a run, and a
// second account kept here would be a second answer to "what happened". So these tests write
// records the way `director` writes them — by their JSON names, into `runs/` under the store —
// and hold what a person is shown about them.

import (
	"os"
	"path/filepath"
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
	if err := a.Play("the audio page", false, ""); err != "" {
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
