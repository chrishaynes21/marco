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
)

func TestARunRecordKeepsTheFourAnswersApart(t *testing.T) {
	for _, c := range []struct {
		name      string
		rep       workflowrun.Report
		cancelled bool
		want      RunOutcome
	}{
		{"cancelled outranks everything, including a verified action",
			workflowrun.Report{Pressed: 1, VerifiedOK: 1}, true, RunCancelled},
		{"nothing pressed is a refusal",
			workflowrun.Report{Pressed: 0}, false, RunRefused},
		{"a verified action",
			workflowrun.Report{Pressed: 1, VerifiedOK: 1}, false, RunVerified},
		{"evidence against the action",
			workflowrun.Report{Pressed: 1, Contradicted: 1}, false, RunContradicted},
		{"no evidence either way",
			workflowrun.Report{Pressed: 1, Unconfirmed: 1}, false, RunUnconfirmed},
		// PRESSED AND NEVER CHECKED is the case `--step` on a routine with no
		// outcome expectation lands in, and it must not read as a success.
		{"pressed and never checked is not verified",
			workflowrun.Report{Pressed: 1}, false, RunUnconfirmed},
		// AND A CONTRADICTION IS NOT HIDDEN BY A VERIFICATION beside it: a run that
		// verified one action and was contradicted on another did not go well.
		{"a contradiction outranks a verification",
			workflowrun.Report{Pressed: 2, VerifiedOK: 1, Contradicted: 1}, false,
			RunContradicted},
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
	id := newRun(time.Date(2026, 10, 18, 23, 7, 24, 0, time.UTC))
	if id != "run-20261018-230724" {
		t.Fatalf("run id %q is not sortable and second-resolution", id)
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
