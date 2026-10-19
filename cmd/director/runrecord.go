package main

// A RUN REFERENCE, AND THE RECORD IT RETRIEVES.
//
// The owner's requirement, from the supervised-trial brief:
//
//	A run reference associated with the diagnostic evidence, easy to copy into a bug report.
//	[…] The run evidence distinguishes cancellation, refusal, execution, and visual
//	verification.
//
// # Why a record and not just an id
//
// An id on its own is a label for a thing nobody kept. Until now a supervised run left behind
// exactly one artefact — the terminal scrollback — and `--frames` writes hover measurements and
// is off by default. So "what happened on the attempt at 23:07" was answerable only by whoever
// still had the window open, which is why three of tonight's reports were reconstructed from
// pasted output.
//
// # What it deliberately does NOT contain
//
// No screenshots, and nothing that is not already durable. The outcome sentence can carry the
// words a verification rested on — but those words are the step's saved EXPECTATION, which is
// already in the artifact in the same store. A record that introduced a new class of durable
// screen content would be a privacy decision wearing a usability hat, and this is not one.
//
// `--frames` still writes pixels, still only when asked for, and now writes them under the run
// id so that the reference retrieves THAT attempt's evidence rather than the last one's.

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
)

// runsDir is where run records are kept, beside the routines they were run from.
const runsDir = "runs"

// RunOutcome is how a supervised run ended, in the four ways a person asks about.
//
// Four and not two. "It did not work" covers a refusal before anything was pressed, a press
// whose outcome contradicted the expectation, and a press nobody could confirm either way —
// and those send somebody to three different places. Cancellation is the fourth because a run
// somebody stopped is not a run that failed.
type RunOutcome string

const (
	// RunCancelled: a person stopped it. Nothing is implied about the application.
	RunCancelled RunOutcome = "cancelled"
	// RunRefused: Marco declined before sending input, or stopped without arriving.
	RunRefused RunOutcome = "refused"
	// RunVerified: an action was performed and its outcome was verified.
	RunVerified RunOutcome = "verified"
	// RunUnconfirmed: an action was performed and its outcome could not be decided.
	RunUnconfirmed RunOutcome = "unconfirmed"
	// RunContradicted: an action was performed and the evidence disagreed with it.
	RunContradicted RunOutcome = "contradicted"
	// RunFailed: the run could not start — no window, no artifact, a refused scope.
	RunFailed RunOutcome = "failed"
)

// RunRecord is one supervised attempt, as a person would cite it in a bug report.
type RunRecord struct {
	Version int    `json:"version"`
	Run     string `json:"run"`
	// Workflow is the routine's name and Application the scope it was run against.
	Workflow    string `json:"workflow,omitempty"`
	Application string `json:"application,omitempty"`
	Step        int    `json:"step,omitempty"`
	Started     string `json:"started"`
	Ended       string `json:"ended"`
	// Outcome is the one word, and Because the sentence the run itself gave.
	Outcome RunOutcome `json:"outcome"`
	Because string     `json:"because,omitempty"`
	// Sent is how many programs reached a computer, and Real whether this was --live.
	Sent int  `json:"actions_sent"`
	Real bool `json:"live"`
	// The three verification counts, kept apart for the reason the report keeps them
	// apart: an unconfirmed action is neither a success nor a failure.
	//
	// ONE TAG PER FIELD, and the first version of this had one tag for all three —
	// `json:"verified,contradicted,unconfirmed"` on a single declaration, which names
	// every one of them "verified" and treats the rest as encoding options. The record
	// would have carried one number three times under one key.
	Verified     int `json:"verified"`
	Contradicted int `json:"contradicted"`
	Unconfirmed  int `json:"unconfirmed"`
	// Frames is where the pixel evidence went, when any was asked for.
	Frames string `json:"frames,omitempty"`
}

// newRun mints a reference for this attempt.
//
// Sortable, second-resolution, and local time because the person reading it is in it. Two runs
// in one second would collide; two supervised runs in one second is not a thing a person does,
// and a reference nobody can type is worse than one that could theoretically collide.
func newRun(at time.Time) string {
	// AND A SUFFIX, BECAUSE A SECOND IS NOT UNIQUE.
	//
	// The timestamp is the sortable half and a person reads it; it is not an identity.
	// Two attempts started in the same second — a script, a retry, two terminals —
	// produced the same id, and `saveRun` overwrote, so one attempt's record silently
	// became the other's. The one thing a reference must do is retrieve the attempt it
	// names.
	//
	// Four hex characters from the system's randomness, which is short enough to type
	// and read aloud. If it cannot be read, the nanosecond stands in: a worse suffix is
	// better than a colliding one, and this must never fail a run.
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("run-%s-%04x", at.Format("20060102-150405"),
			at.Nanosecond()&0xffff)
	}
	return fmt.Sprintf("run-%s-%02x%02x", at.Format("20060102-150405"), b[0], b[1])
}

// runPath is where a run's record is written.
func runPath(id string) string { return filepath.Join(configDir(), runsDir, id+".json") }

// saveRun writes the record and returns the path, or the reason it could not.
//
// A failure here is NOT a failure of the run. The run already happened; losing its record
// loses the ability to cite it, which is worth a line on stderr and nothing more.
func saveRun(r RunRecord) (string, error) {
	if err := os.MkdirAll(filepath.Join(configDir(), runsDir), 0o755); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	// WRITTEN ONCE, NEVER OVER.
	//
	// `os.WriteFile` truncates whatever is there, so a colliding id replaced an earlier
	// attempt's record and the earlier reference then retrieved somebody else's run.
	// O_EXCL makes that a refusal rather than a silent substitution: the record that
	// exists is the one that was written first, and the caller is told.
	p := runPath(r.Run)
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return p, nil
}

// outcomeOfRun reads the four-way answer off the report the run produced.
//
// In the order a reader needs them: what a person did, then what Marco refused, then what the
// evidence said. A run that was cancelled is cancelled whatever else is true of it.
func outcomeOfRun(rep workflowrun.Report, cancelled bool) RunOutcome {
	// A VERDICT IS NOT A WHOLE RUN. Counted apart, because the overall answer depends on
	// how far the workflow got as well as on what each action's evidence said.
	decided := rep.VerifiedOK + rep.Contradicted + rep.Unconfirmed
	switch {
	case cancelled:
		return RunCancelled
	case rep.Contradicted > 0:
		return RunContradicted
	case rep.Unconfirmed > 0:
		return RunUnconfirmed
	case rep.Pressed > decided:
		// AN ACTION WENT OUT AND NOTHING EVER ANSWERED FOR IT. A later action
		// whose verification was still gathering when the run's own deadline
		// arrived lands here, and so does `--step` on a routine with no outcome
		// expectation. Neither is a verified run.
		return RunUnconfirmed
	case rep.VerifiedOK > 0 && rep.Progress.Complete():
		return RunVerified
	case rep.VerifiedOK > 0:
		// VERIFIED SOMETHING AND DID NOT FINISH.
		//
		// The defect this closes: the classifier returned `verified` on any
		// positive count, so a workflow whose first step verified and whose second
		// refused before execution was recorded, as a whole, as verified. The
		// per-action counts are kept — that first action did verify — and they do
		// not promote an incomplete run to success.
		return RunRefused
	case rep.Pressed == 0:
		// NOTHING WAS SENT. Whether it stopped early or never started, from the
		// person's side this is Marco declining to act.
		return RunRefused
	}
	return RunUnconfirmed
}

// citeRun is the line a person copies into a bug report.
func citeRun(id, path string) string {
	if path == "" {
		return fmt.Sprintf("  run %s   (its record could not be written)", id)
	}
	return fmt.Sprintf("  run %s   %s", id, path)
}
