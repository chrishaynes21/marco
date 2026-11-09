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

	"github.com/chaynes-simpleclouds/marco/internal/routes"
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
		first.Application != "arkascended" || first.Outcome != "refused" {
		t.Errorf("the record is read back as %+v", first)
	}
	// AND THE NUMBER AND THE MODE IT STATED, as it stated them. This record says no
	// presses went out and that input was enabled; both are read back rather than
	// defaulted, which is what makes a record that states NEITHER distinguishable. See
	// PastRun.Sent and PastRun.Live.
	if first.Sent == nil || *first.Sent != 0 {
		t.Errorf("the record says 0 presses went out and the row says %v", first.Sent)
	}
	if first.Live == nil || !*first.Live {
		t.Errorf("the record says input was enabled and the row says %v", first.Live)
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
	// A RUN THIS WINDOW WATCHED, so the transcript is part of what it gathers — and the
	// run cites the reference the record is under, because a transcript now belongs to a
	// reference and is withheld when it does not match. See
	// TestReportingOneRunDoesNotAttachAnotherRunsOutput.
	t.Setenv("STUDIO_STANDIN_REF", "run-20261020-160545-4006")
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

// REPORTING ONE RUN DOES NOT ATTACH ANOTHER RUN'S OUTPUT.
//
// The transcript is this window's memory of ONE child, cleared and refilled every time something
// runs, and `Report` copied whatever was in it into whatever run was being asked about. So: play
// A, play B, report A — and the file held A's reference and A's record with B's output underneath
// it, headed WHAT IT SAID AS IT RAN. The person attaching that to an email has no way to tell,
// and the people reading it have no reason to doubt it.
//
// A wrong transcript is worse than none. It is evidence about the wrong run, in the one file
// somebody sends precisely because they cannot explain what happened themselves.
func TestReportingOneRunDoesNotAttachAnotherRunsOutput(t *testing.T) {
	a := aPlayableAct(t, "the audio page")
	const older, newer = "run-20261024-100000-old1", "run-20261024-110000-new2"
	for _, ref := range []string{older, newer} {
		aRecordOf(t, a, ref, fmt.Sprintf(`{
		  "version": 1, "run": %q, "act": "the audio page",
		  "workflow": "ark graphics live",
		  "started": "2026-10-24T10:00:00-07:00", "outcome": "refused",
		  "because": "refused — 0 of 1 interaction(s) verified",
		  "actions_sent": 0, "live": true, "verified": 0
		}`, ref))
	}
	// TWO RUNS, EACH CITING ITS OWN REFERENCE. That citation is in the transcript, so
	// each run's output says which run it is — which is how a report holding the wrong
	// one can be caught at all.
	for _, ref := range []string{older, newer} {
		t.Setenv("STUDIO_STANDIN_REF", ref)
		if err := a.Play("the audio page", false, false, ""); err != "" {
			t.Fatalf("Play would not start: %s", err)
		}
		waitForTranscript(t, a)
	}

	// ── THE OLDER RUN: ITS OWN RECORD, AND NOBODY ELSE'S OUTPUT ──
	first := reportText(t, a, older)
	if strings.Contains(first, newer) {
		t.Errorf("the report of %s holds the output of %s:\n%s", older, newer, first)
	}
	// `ARGS perform-workflow` is the stand-in child's own first line, so it is in a
	// transcript and nowhere else in the file. Asserted rather than the heading, which is
	// written either way.
	if strings.Contains(first, "ARGS perform-workflow") {
		t.Errorf("the report of %s holds a transcript, and the only one this window "+
			"has belongs to a later run:\n%s", older, first)
	}
	if !strings.Contains(first, "not available") {
		t.Errorf("the report of %s does not say that what it said as it ran is "+
			"unavailable, so the section reads as a run that printed nothing:\n%s",
			older, first)
	}
	// ── AND THE RUN THIS WINDOW IS ACTUALLY HOLDING STILL GATHERS ITS OWN ──
	//
	// The other direction. A `Report` that withheld every transcript would pass the
	// assertions above and would have taken away the thing the file is for.
	second := reportText(t, a, newer)
	if !strings.Contains(second, "ARGS perform-workflow") {
		t.Errorf("the report of the run in front of somebody holds no transcript at "+
			"all:\n%s", second)
	}
}

// AND THE ACT IT DESCRIBES IS THE ONE THE RUN NAMES, NOT THE ONE ON SCREEN.
//
// The sibling of the transcript above, and the same shape: the Act section was found by matching
// whichever Act this window happened to have open, so a report about this morning's run, written
// after somebody had moved on to a different Act, printed that other Act's composition under
// THE ACT, AS IT STANDS NOW. The routine half of `Report` was already found by the record; this is
// the half that was not.
//
// Falsified with an Act the record names and the library does not hold: the old reading would
// print the loaded Act's interactions here, and the correct one says the named Act is gone.
func TestAReportDescribesTheActTheRunNamesAndNotTheOneOnScreen(t *testing.T) {
	a := aPlayableAct(t, "the audio page")
	const ref = "run-20261024-090000-else"
	aRecordOf(t, a, ref, `{
	  "version": 1, "run": "run-20261024-090000-else", "act": "a different act",
	  "started": "2026-10-24T09:00:00-07:00", "outcome": "refused",
	  "because": "refused — 0 of 1 interaction(s) verified",
	  "actions_sent": 0, "live": true, "verified": 0
	}`)
	// SOMETHING ELSE IS LOADED, which is the situation: a person runs an Act, then opens
	// another, then goes back to report the first.
	t.Setenv("STUDIO_STANDIN_REF", "run-20261024-120000-aact")
	if err := a.Play("the audio page", false, false, ""); err != "" {
		t.Fatalf("Play would not start: %s", err)
	}
	waitForTranscript(t, a)

	report := reportText(t, a, ref)
	// `the audio tab` is the composed Act's own interaction, so it is in that Act's
	// composition and nowhere else in the file.
	if strings.Contains(report, "the audio tab") {
		t.Errorf("the report of a run of %q describes the Act this window has open "+
			"instead:\n%s", "a different act", report)
	}
	if !strings.Contains(report, "a different act") {
		t.Errorf("the report does not name the Act the run was a run of:\n%s", report)
	}
	if !strings.Contains(report, "not composed") {
		t.Errorf("the report does not say that the Act the run names is no longer "+
			"in the library, so the section reads as one that was forgotten:\n%s",
			report)
	}
}

// reportText writes a problem report and reads it back.
func reportText(t *testing.T, a *Acts, reference string) string {
	t.Helper()
	r := a.Report(reference)
	if r.Error != "" {
		t.Fatalf("the report of %s was not written: %s", reference, r.Error)
	}
	raw, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatalf("the report it says it wrote cannot be read: %v", err)
	}
	return string(raw)
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

// aRoutinesRecord is one run of a saved routine, as `marco do` leaves it.
//
// Written by hand rather than by running the command, for the reason the Act records above are:
// what is under test is what a person is SHOWN about a record, and generating one would make
// these tests about the writer instead.
// `more` is whatever else the record carries, which for a routine is the count — and for an
// attempt nobody could count is NOTHING. A fixture that always wrote one could not express the
// record a killed `marco.exe` leaves, which is the record the page was inventing a zero for.
func aRoutinesRecord(reference, said, because, more string) string {
	return fmt.Sprintf(`{
	  "version": 1, "kind": "routine", "run": %q,
	  "build": "deadbeef", "routine": "ark-graphics", "application": "arkascended",
	  "scope": "focus", "source_digest": "f85ca17e96485e1f",
	  "library": "/home/somebody/.config/marco/routes",
	  "mode": "unstated", "contract": "guarded",
	  "started": "2026-10-24T16:05:45-07:00", "ended": "2026-10-24T16:05:47-07:00",
	  "outcome": %q, "because": %q%s
	}`, reference, said, because, more)
}

// askedFor is the `input_actions` a routine's record states, as aRoutinesRecord's tail.
func askedFor(n int) string { return fmt.Sprintf(`, "input_actions": %d`, n) }

// A ROUTINE'S RUN IS READ BACK IN A ROUTINE'S WORDS, AND NEVER IN AN ACT'S.
//
// The defect this closes. Every record used to go through `saidOf` and `nextOf` — the SUPERVISED
// seven — so a routine's `performed` fell to their default and the window described a run, and
// offered a remedy for it, in the language of evidence that run never gathered. Not a conversion
// into `verified`; the same mistake one step quieter.
func TestARoutinesRunIsListedInARoutinesWords(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	aRecordOf(t, a, "run-20261024-160545-ab12",
		aRoutinesRecord("run-20261024-160545-ab12", "performed", "", askedFor(4)))

	got := a.History()
	if len(got) != 1 {
		t.Fatalf("%d runs listed, want 1", len(got))
	}
	p := got[0]
	if p.Kind != "routine" {
		t.Errorf("the row does not say which kind of run it was: %q", p.Kind)
	}
	if p.Routine != "ark-graphics" {
		t.Errorf("the row calls the play %q", p.Routine)
	}
	// THE SENTENCE A ROUTINE GETS, which is careful about the one thing it cannot see.
	if !strings.Contains(p.Said, "Nothing here checked") {
		t.Errorf("a play's run is described as %q, which claims more than was "+
			"checked", p.Said)
	}
	// AND NOT AN ACT'S. These are `saidOf`'s own words for its own verdicts.
	for _, wrong := range []string{"It worked", "the screen changed", "could not tell",
		"ended as performed"} {
		if strings.Contains(p.Said, wrong) {
			t.Errorf("a play's run is described with an Act's words (%q): %q",
				wrong, p.Said)
		}
	}
	// AND `performed` IS GIVEN NOTHING TO DO. There is nothing to do about a run where
	// every press happened, and a next step would imply something was wrong with it.
	if p.Next != "" {
		t.Errorf("a play that performed is told to do something about it: %q", p.Next)
	}
	// AND THE NUMBER COMES OFF ITS OWN KEY. A routine writes `input_actions`; reading
	// `actions_sent` would show zero presses for a run that made four.
	if p.Sent == nil || *p.Sent != 4 {
		t.Errorf("the row says %v input actions, want 4", p.Sent)
	}
	// AND IT IS NOT GIVEN A MODE ITS RECORD NEVER STATED. `live` belongs to the
	// SUPERVISED record; a routine's states no liveness, because `marco do` has no
	// "write it down instead" mode to distinguish. Read as a plain bool it came back
	// false, and the page labelled a run that asked for four input actions `input
	// disabled`. See PastRun.Live and TestARowDoesNotSayInputWasDisabledWhenNothingSaidSo.
	if p.Live != nil {
		t.Errorf("a play's row was given a liveness of %v, and its record states "+
			"none", *p.Live)
	}
}

// AND A ROUTINE THAT REFUSED IS TOLD WHAT TO DO ABOUT IT, IN ITS OWN TERMS.
//
// `nextOf`'s remedies are about demonstrations and evidence — *demonstrate it again, resting
// after the click so the result is read.* A routine has neither, so every one of those sentences
// would be advice about machinery it does not have.
func TestARefusedRoutinesRemedyIsAboutTheWindowAndNotADemonstration(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	said := "ark-graphics stopped: the window this was recorded in was 1920x1080 and " +
		"is now 1280x720"
	aRecordOf(t, a, "run-20261024-160545-cd34",
		aRoutinesRecord("run-20261024-160545-cd34", "refused", said, askedFor(1)))

	got := a.History()
	if len(got) != 1 {
		t.Fatalf("%d runs listed, want 1", len(got))
	}
	p := got[0]
	if !strings.Contains(p.Said, said) {
		t.Errorf("the run's own sentence is not in what the person reads: %q", p.Said)
	}
	if p.Next == "" {
		t.Fatal("a refused play is given nothing to do about it")
	}
	for _, wrong := range []string{"demonstrate", "Demonstrate", "verified",
		"interaction"} {
		if strings.Contains(p.Next, wrong) {
			t.Errorf("the remedy offers an Act's machinery (%q): %q", wrong, p.Next)
		}
	}
}

// AND AN ACT'S RUN STILL READS EXACTLY AS IT DID.
//
// The regression direction, and the reason the kind is asked of the record rather than guessed:
// a supervised record carries no kind at all, so the absence has to keep meaning what it meant.
func TestAnActsRunStillReadsInTheSupervisedWords(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	aRecordOf(t, a, "run-20261024-150000-ee56", `{
	  "version": 1, "run": "run-20261024-150000-ee56",
	  "workflow": "ark graphics live", "act": "the audio page",
	  "application": "arkascended",
	  "started": "2026-10-24T15:00:00-07:00", "ended": "2026-10-24T15:00:30-07:00",
	  "outcome": "verified", "actions_sent": 2, "verified": 2, "live": true
	}`)
	got := a.History()
	if len(got) != 1 {
		t.Fatalf("%d runs listed, want 1", len(got))
	}
	p := got[0]
	if p.Kind != "" {
		t.Errorf("a supervised record was given the kind %q; absence is what means "+
			"supervised", p.Kind)
	}
	if !strings.Contains(p.Said, "It worked") {
		t.Errorf("a verified Act's run reads as %q", p.Said)
	}
	if p.Sent == nil || *p.Sent != 2 || p.Verified != 2 {
		t.Errorf("an Act's counts read as sent=%v verified=%d, want 2 and 2",
			p.Sent, p.Verified)
	}
	if p.Routine != "ark graphics live" {
		t.Errorf("the demonstration an Act's run came from reads as %q", p.Routine)
	}
}

// REPORTING A PROBLEM WITH A ROUTINE GATHERS THE ROUTINE, NOT AN ACT.
//
// `Report` ended with *THE ACT, AS IT STANDS NOW*, found by matching whichever Act this window
// had open. A routine has no Act; what it has is its SOURCE, which is the executable truth about
// it and the only thing a reader can judge the run against.
func TestReportingAProblemWithARoutineGathersItsSource(t *testing.T) {
	a := aSavedRoutine(t, "app-anywhere", nil)
	r, found := routineCalled(a.Routines(), "ark graphics")
	if !found {
		t.Fatalf("the fixture play is not in the library: %+v", a.Routines())
	}
	aRecordOf(t, a, "run-20261024-170000-ff78", fmt.Sprintf(`{
	  "version": 1, "kind": "routine", "run": "run-20261024-170000-ff78",
	  "build": "deadbeef", "routine": %q, "application": "arkascended",
	  "scope": "focus", "source_digest": %q, "mode": "unstated",
	  "contract": "guarded", "started": "2026-10-24T17:00:00-07:00",
	  "ended": "2026-10-24T17:00:02-07:00", "outcome": "failed",
	  "because": "ark-graphics stopped: the window was 1920x1080 and is now 1280x720",
	  "input_actions": 1
	}`, r.Slug, routes.DigestOf(r.Source)))

	got := a.Report("run-20261024-170000-ff78")
	if got.Error != "" {
		t.Fatalf("the report was not written: %s", got.Error)
	}
	raw, err := os.ReadFile(got.Path)
	if err != nil {
		t.Fatalf("the report it says it wrote cannot be read: %v", err)
	}
	report := string(raw)
	for _, want := range []struct{ text, why string }{
		{"run-20261024-170000-ff78", "the report does not name the run"},
		{"THE PLAY, AS IT STANDS NOW", "the play is not in it, and the " +
			"source is the only thing a reader can judge the run against"},
		{"is a script", "the play's own source is not in it"},
		{"the window was 1920x1080", "the run's own reason is not in it"},
		{"deadbeef", "the build that performed it is not in it"},
		{"guarded", "the contract it ran under is not in it"},
		{"unstated", "the mode is not in it"},
	} {
		if !strings.Contains(report, want.text) {
			t.Errorf("%s (looked for %q)", want.why, want.text)
		}
	}
	// AND IT DOES NOT CLAIM TO HOLD AN ACT. There is no Act here.
	if strings.Contains(report, "THE ACT, AS IT STANDS NOW") {
		t.Error("the report of a play's run holds a section about an Act")
	}
	// AND IT DOES NOT WARN THAT THE SOURCE CHANGED, because it has not.
	if strings.Contains(report, "THIS IS NOT THE SOURCE THAT RAN") {
		t.Error("the report says the source changed when the digests agree")
	}
}

// AND IT SAYS SO WHEN THE SOURCE ON DISK IS NO LONGER THE SOURCE THAT RAN.
//
// The one thing a routine's problem report can say that nothing else can — and the likely case
// rather than the unlikely one, because re-recording is exactly what somebody does between a
// failure and reporting it. Without this, a reader compares the run to a program that never ran.
func TestAReportSaysWhenTheRoutineHasBeenSavedAgainSince(t *testing.T) {
	a := aSavedRoutine(t, "app-anywhere", nil)
	r, found := routineCalled(a.Routines(), "ark graphics")
	if !found {
		t.Fatalf("the fixture play is not in the library: %+v", a.Routines())
	}
	// A DIGEST THAT IS NOT THIS FILE'S, which is what the record of a run from before
	// the routine was saved again holds.
	aRecordOf(t, a, "run-20261024-170000-0011", fmt.Sprintf(`{
	  "version": 1, "kind": "routine", "run": "run-20261024-170000-0011",
	  "routine": %q, "application": "arkascended", "scope": "focus",
	  "source_digest": "0000000000000000", "contract": "guarded",
	  "started": "2026-10-24T17:00:00-07:00", "ended": "2026-10-24T17:00:02-07:00",
	  "outcome": "failed", "input_actions": 1
	}`, r.Slug))

	got := a.Report("run-20261024-170000-0011")
	if got.Error != "" {
		t.Fatalf("the report was not written: %s", got.Error)
	}
	raw, err := os.ReadFile(got.Path)
	if err != nil {
		t.Fatal(err)
	}
	report := string(raw)
	if !strings.Contains(report, "THIS IS NOT THE SOURCE THAT RAN") {
		t.Errorf("the report prints a source that is not the one that ran and does "+
			"not say so:\n%s", report)
	}
	// AND IT NAMES BOTH DIGESTS, so a reader can tell which is which.
	for _, want := range []string{"0000000000000000", routes.DigestOf(r.Source)} {
		if !strings.Contains(report, want) {
			t.Errorf("the warning does not name the digest %q", want)
		}
	}
}

// AND A ROUTINE THAT HAS GONE SINCE IS SAID SO, NOT SILENTLY LEFT OUT.
//
// A run whose routine cannot be found is a different finding from one whose routine reads oddly,
// and a report with the section quietly missing looks like a report that forgot.
func TestAReportSaysWhenTheRoutineIsNotInTheLibraryAnyMore(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	aRecordOf(t, a, "run-20261024-170000-2233", `{
	  "version": 1, "kind": "routine", "run": "run-20261024-170000-2233",
	  "routine": "a-routine-nobody-has", "application": "arkascended",
	  "scope": "focus", "source_digest": "f85ca17e96485e1f", "contract": "guarded",
	  "started": "2026-10-24T17:00:00-07:00", "ended": "2026-10-24T17:00:02-07:00",
	  "outcome": "failed", "input_actions": 0
	}`)
	got := a.Report("run-20261024-170000-2233")
	if got.Error != "" {
		t.Fatalf("the report was not written: %s", got.Error)
	}
	raw, _ := os.ReadFile(got.Path)
	report := string(raw)
	if !strings.Contains(report, "THE PLAY, AS IT STANDS NOW") {
		t.Errorf("the section is missing entirely:\n%s", report)
	}
	if !strings.Contains(report, "is not in this library") {
		t.Errorf("the report does not say the play has gone:\n%s", report)
	}
}

// AND AN ATTEMPT THAT NEVER FINISHED SAYS SO, AND NEVER THAT IT WORKED.
//
// Reachable the moment a routine's run leaves a record at all: the reference is claimed and a
// provisional record written before anything is sent, so a `marco` killed between the two leaves
// exactly this. It is also the acceptance check — *a killed child leaves an unfinished attempt,
// never a completed or successful one.*
//
// Added because the gate found its absence: nothing covered an unfinished ROUTINE record, so the
// sentence for it could be deleted and the row fell through to "ended as started", which reads as
// a word somebody misspelled rather than as a run nobody can speak for.
func TestAnUnfinishedRoutineAttemptSaysSoAndNotThatItWorked(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	aRecordOf(t, a, "run-20261024-180000-4455", `{
	  "version": 1, "kind": "routine", "run": "run-20261024-180000-4455",
	  "routine": "ark-graphics", "application": "arkascended", "scope": "focus",
	  "mode": "unstated", "started": "2026-10-24T18:00:00-07:00", "outcome": "started",
	  "because": "reserved; this attempt had not finished when the record was last written"
	}`)
	got := a.History()
	if len(got) != 1 {
		t.Fatalf("%d runs listed, want 1", len(got))
	}
	p := got[0]
	if !p.Unfinished {
		t.Error("an attempt with no end is not marked unfinished")
	}
	if !strings.Contains(p.Said, "never said how it ended") {
		t.Errorf("an unfinished attempt reads as %q", p.Said)
	}
	if strings.Contains(p.Said, "ended as started") {
		t.Errorf("an unfinished attempt reads as a word somebody misspelled: %q",
			p.Said)
	}
	// AND IT IS NEVER READ AS A RUN THAT WORKED.
	for _, wrong := range []string{"every press happened", "It worked", "performed"} {
		if strings.Contains(p.Said, wrong) {
			t.Errorf("an unfinished attempt claims %q: %q", wrong, p.Said)
		}
	}
	if p.Next == "" {
		t.Error("an unfinished attempt is given nothing to do about it")
	}
	// AND IT IS GIVEN NEITHER A COUNT NOR A MODE, because it states neither.
	//
	// This is the record that matters most for both: an attempt killed mid-run may ALREADY
	// HAVE SENT INPUT, and nobody wrote the number down. Zero there is not a count, it is
	// the claim that nothing was pressed — offered about the one run a person is most
	// likely to be asking somebody else about.
	if p.Sent != nil {
		t.Errorf("an interrupted attempt is credited with %d input actions, and no "+
			"count was ever written down", *p.Sent)
	}
	if p.Live != nil {
		t.Errorf("an interrupted attempt is given a liveness of %v", *p.Live)
	}
}
