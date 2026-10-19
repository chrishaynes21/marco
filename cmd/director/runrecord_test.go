package main

// THE FOUR ANSWERS A RUN RECORD HAS TO KEEP APART.
//
//	The run evidence distinguishes cancellation, refusal, execution, and visual verification.
//
// Four and not two. "It did not work" covers a refusal before anything was pressed, a press
// whose outcome contradicted the expectation, and a press nobody could confirm either way — and
// those send somebody to three different places. Cancellation is the fourth, because a run
// somebody stopped is not a run that failed.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

func TestARunRecordKeepsTheFourAnswersApart(t *testing.T) {
	for _, c := range []struct {
		name      string
		rep       workflowrun.Report
		cancelled bool
		want      RunOutcome
	}{
		{"cancelled outranks everything, including a verified action",
			workflowrun.Report{Pressed: 1, VerifiedOK: 1, Progress: done(1)},
			true, RunCancelled},
		{"nothing pressed is a refusal",
			workflowrun.Report{Pressed: 0}, false, RunRefused},
		{"a verified action on a workflow that finished",
			workflowrun.Report{Pressed: 1, VerifiedOK: 1, Progress: done(1)},
			false, RunVerified},
		// ── THE DEFECT THIS CLASSIFIER WAS CHANGED FOR ──
		{"first verified, second refused before execution",
			workflowrun.Report{Pressed: 1, VerifiedOK: 1, Progress: stoppedAt(2, 1)},
			false, RunRefused},
		{"first verified, second still pending at the deadline",
			workflowrun.Report{Pressed: 2, VerifiedOK: 1, Progress: stoppedAt(2, 1)},
			false, RunUnconfirmed},
		{"evidence against the action",
			workflowrun.Report{Pressed: 1, Contradicted: 1, Progress: done(1)},
			false, RunContradicted},
		{"no evidence either way",
			workflowrun.Report{Pressed: 1, Unconfirmed: 1, Progress: done(1)},
			false, RunUnconfirmed},
		// PRESSED AND NEVER CHECKED is the case `--step` on a routine with no
		// outcome expectation lands in, and it must not read as a success.
		{"pressed and never checked is not verified",
			workflowrun.Report{Pressed: 1}, false, RunUnconfirmed},
		// AND A CONTRADICTION IS NOT HIDDEN BY A VERIFICATION beside it: a run that
		// verified one action and was contradicted on another did not go well.
		{"a contradiction outranks a verification",
			workflowrun.Report{Pressed: 2, VerifiedOK: 1, Contradicted: 1,
				Progress: done(2)}, false, RunContradicted},
	} {
		if got := outcomeOfRun(c.rep, c.cancelled); got != c.want {
			t.Errorf("%s: outcome %q, want %q", c.name, got, c.want)
		}
	}
}

// TestARunRecordIsWrittenWhereItsReferenceSaysItIs.
//
// The reference is printed before anything happens, so it names a path that does not exist yet.
// What has to hold is that the record lands exactly there — a citation that retrieves nothing
// is worse than no citation, because somebody will go looking.
func TestARunRecordIsWrittenWhereItsReferenceSaysItIs(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	id, _, rerr := reserveRun(time.Date(2026, 10, 18, 23, 7, 24, 0, time.UTC), "", nil)
	if rerr != nil {
		t.Fatalf("reserving: %v", rerr)
	}
	// THE TIMESTAMP IS THE HALF A PERSON READS, and the suffix is the half that
	// makes it an identity — two attempts in one second used to share an id and one
	// record replaced the other.
	if !strings.HasPrefix(id, "run-20261018-230724-") {
		t.Fatalf("run id %q is not the sortable timestamp a person reads", id)
	}
	want := runPath(id)

	got, err := saveRun(RunRecord{
		Version: 1, Run: id, Workflow: "ark graphics live",
		Application: "arkascended", Outcome: RunVerified,
		Sent: 1, Real: true, Verified: 1,
	})
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	if got != want {
		t.Errorf("written to %q, cited as %q", got, want)
	}
	b, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("the reference retrieves nothing: %v", err)
	}
	// AND THE THREE COUNTS ARE THREE FIELDS. The first version declared them on one
	// line with one tag, which names all three "verified" and keeps one number.
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("the record is not readable JSON: %v", err)
	}
	for _, k := range []string{"verified", "contradicted", "unconfirmed"} {
		if _, ok := back[k]; !ok {
			t.Errorf("the record has no %q field: %s", k, b)
		}
	}
	if !strings.Contains(string(b), "arkascended") {
		t.Errorf("the record does not say which application it ran against: %s", b)
	}
}

// TestACitationSaysSoWhenThereIsNothingToRetrieve.
func TestACitationSaysSoWhenThereIsNothingToRetrieve(t *testing.T) {
	if got := citeRun("run-x", ""); !strings.Contains(got, "could not be written") {
		t.Errorf("a citation with no record reads as though one exists: %q", got)
	}
	if got := citeRun("run-x", "C:/x/run-x.json"); !strings.Contains(got, "run-x.json") {
		t.Errorf("a citation does not name the file: %q", got)
	}
}

// done is a workflow that reached every step and stopped at none.
func done(steps int) workflowstore.Progress {
	return workflowstore.Progress{Steps: steps, Reached: steps, Stopped: -1, Acted: steps}
}

// stoppedAt is a workflow that got partway and stopped at a step.
func stoppedAt(steps, reached int) workflowstore.Progress {
	return workflowstore.Progress{Steps: steps, Reached: reached, Stopped: reached,
		Acted: reached}
}
