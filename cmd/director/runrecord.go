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

	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
)

// runsDir is where run records are kept, beside the routines they were run from.
const runsDir = "runs"

// RunOutcome and its seven words live in internal/director/runref, because the reference that
// carries them is printed by this command and READ by whatever asked for the run.
//
// Named here as they always were: this file is where the record is written, and a reader of
// `RunVerified` should not have to learn a second spelling to follow it. One definition, two
// spellings of the same constant, and no second vocabulary — see runref.Outcome for what each
// word means and why there are seven.
type RunOutcome = runref.Outcome

const (
	RunCancelled    = runref.Cancelled
	RunRefused      = runref.Refused
	RunVerified     = runref.Verified
	RunUnconfirmed  = runref.Unconfirmed
	RunContradicted = runref.Contradicted
	RunFailed       = runref.Failed
	RunStarted      = runref.Started
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
	// Act and ActSource are the PLAY this run performed, when it performed one: the name
	// it was selected by and the digest of its own source.
	//
	// Beside `Workflow` rather than instead of it, because they answer different
	// questions and an audit needs both: `Workflow` is the demonstration the interactions
	// came from, and these two are the composition that chose which of them to perform
	// and in what order. A record with only the first could not tell two Acts over one
	// demonstration apart, and a record with only the second could not say which
	// demonstration was pressed.
	//
	// `ActSource` is the digest rather than the source: the play is on disk under its own
	// name, and a record that embedded it would be a second copy to drift.
	Act       string `json:"act,omitempty"`
	ActSource string `json:"act_source,omitempty"`
	// Authority is what permitted this run's presses, when it was not somebody
	// answering at the keyboard press by press.
	//
	// The sentence a grant describes itself with: who gave it, when, and over which
	// interactions. Empty for a run under the keyed contract, where every press was
	// permitted by a key and the attempt records say so one at a time.
	//
	// Here because the record is what a bug report cites, and "what authorised this" is
	// the first question anybody asks about a press that should not have happened. A
	// record that only said `live: true` could not answer it.
	Authority string `json:"authority,omitempty"`
	// Frames is where the pixel evidence went, when any was asked for.
	Frames string `json:"frames,omitempty"`
}

// runCandidate proposes a reference for this attempt.
//
// Sortable, second-resolution, and local time because the person reading it is in it. A
// PROPOSAL and not an identity: `reserveRun` decides whether it is free, because randomness
// reduces collisions and does not prevent them.
func runCandidate(at time.Time) string {
	// AND A SUFFIX, BECAUSE A SECOND IS NOT UNIQUE.
	//
	// The timestamp is the sortable half and a person reads it. Two attempts started in
	// the same second — a script, a retry, two terminals — produced the same id, and the
	// one thing a reference must do is retrieve the attempt it names.
	//
	// Four hex characters from the system's randomness, which is short enough to type and
	// read aloud. If it cannot be read, the nanosecond stands in: a worse suffix is
	// better than none, and this must never fail a run.
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("run-%s-%04x", at.Format("20060102-150405"),
			at.Nanosecond()&0xffff)
	}
	return fmt.Sprintf("run-%s-%02x%02x", at.Format("20060102-150405"), b[0], b[1])
}

// runReserveTries is how many candidates are tried before giving up.
//
// Ten, which is not a probability argument. Each try is a fresh draw against a namespace
// nobody is competing for in earnest, so ten failures in a row means something other than
// bad luck — a read-only directory, a full disk, a clock that does not move — and that is
// worth reporting rather than retrying forever.
const runReserveTries = 10

// reserveRun claims a whole evidence namespace for this attempt, before anything is written
// into it and before the reference is printed.
//
// # Why reservation and not a check at the end
//
// The record used to be the only thing guarded: `saveRun` wrote with `O_EXCL`, so a second
// attempt with the same id was refused its JSON. By then both attempts had been printing the
// same reference for a minute and the hover probe had been writing `01-before.png` into the
// same frames directory — so the refusal protected the record and not the evidence it points
// at, and the attempt that was refused had already overwritten the other's images.
//
// So the id is claimed FIRST, and claimed across both places it means something: a provisional
// record under `runs/`, created exclusively, and this run's own frames directory, created
// exclusively when frames were asked for. A candidate that loses either is abandoned whole —
// the record it already made is removed — and the next one is tried. Two attempts that propose
// the same id therefore end up with two namespaces, and neither can reach the other's.
//
// The provisional record is a real record saying `started`. An attempt that dies before
// finalisation is then retrievable as what it was, rather than retrieving nothing, which is
// how the exits in the next section were found.
func reserveRun(at time.Time, framesRoot string, propose func(time.Time) string) (
	id string, framesDir string, err error) {

	if propose == nil {
		propose = runCandidate
	}
	if e := os.MkdirAll(filepath.Join(configDir(), runsDir), 0o755); e != nil {
		return "", "", e
	}
	for try := 0; try < runReserveTries; try++ {
		id = propose(at)
		p := runPath(id)
		f, e := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if e != nil {
			if os.IsExist(e) {
				continue
			}
			return "", "", e
		}
		b, _ := json.MarshalIndent(RunRecord{
			Version: 1, Run: id,
			Started: at.Format(time.RFC3339),
			Outcome: RunStarted,
			Because: "reserved; this attempt had not finished when the " +
				"record was last written",
		}, "", "  ")
		_, werr := f.Write(b)
		cerr := f.Close()
		if werr != nil || cerr != nil {
			_ = os.Remove(p)
			if werr != nil {
				return "", "", werr
			}
			return "", "", cerr
		}
		if framesRoot == "" {
			return id, "", nil
		}
		// AND THE PIXELS' OWN NAMESPACE, claimed by the same id.
		//
		// `MkdirAll` on the parent and a bare `Mkdir` on this run's own
		// element: the first is shared and must succeed if it is missing,
		// the second must FAIL if somebody else has it. MkdirAll on the
		// whole path would succeed on an existing directory and claim
		// nothing, which is the hole this closes.
		if e := os.MkdirAll(framesRoot, 0o755); e != nil {
			_ = os.Remove(p)
			return "", "", e
		}
		d := filepath.Join(framesRoot, id)
		if e := os.Mkdir(d, 0o755); e != nil {
			_ = os.Remove(p)
			if os.IsExist(e) {
				continue
			}
			return "", "", e
		}
		return id, d, nil
	}
	return "", "", fmt.Errorf("no free run reference after %d tries", runReserveTries)
}

// runPath is where a run's record is written.
func runPath(id string) string { return filepath.Join(configDir(), runsDir, id+".json") }

// saveRun writes the record into this run's OWN reservation and returns the path.
//
// A failure here is NOT a failure of the run. The run already happened; losing its record
// loses the ability to cite it, which is worth a line on stderr and nothing more.
//
// # It may only write where a reservation already is
//
// No O_CREATE, which is the whole guard. `reserveRun` created this file exclusively, so the
// file EXISTING is the proof that this id belongs to this attempt; opening without O_CREATE
// means a record for an id nobody reserved cannot be written at all, and a record for an id
// somebody else holds was never reachable because they hold it.
//
// The earlier version guarded the other way round — O_CREATE|O_EXCL at save time — which
// refused the second writer after both had been printing the same reference and writing into
// the same frames directory for the length of a run.
func saveRun(r RunRecord) (string, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	p := runPath(r.Run)
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_TRUNC, 0o644)
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

// citeRun is the line a person copies into a bug report, and the line a front end that spawned
// this run reads back. One shape, in runref, for exactly that reason.
func citeRun(id, path string) string { return runref.Cite(id, path) }
