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
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/buildid"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/runs"
)

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
	// Build is the revision of the director that performed this run.
	//
	// Run evidence has to answer "what was running", and a file's timestamp does not:
	// the owner spent a day on recordings made by a director a build behind the window
	// that spawned it, and nothing anywhere said so. See internal/buildid.
	Build string `json:"build,omitempty"`
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
	// Layout says the run held the fixed-layout replay licence: it pressed where
	// somebody clicked, in a window of the size they clicked it in, and asked nothing
	// about the screen. The one variable a coordinate-replay trial is ABOUT, and two
	// runs were read back as evidence concerning it while no record said whether it
	// had been granted — so neither of them could distinguish a mode that was off
	// from a mode that was on and bypassed.
	Layout bool `json:"fixed_layout,omitempty"`
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

// runCandidate, the reservation discipline and the finalise-without-create guard are now
// `internal/runs`, because a routine's run has to claim a reference the same way and under the
// same roof — see that package for why the DISCIPLINE is shared while the RECORD is not.
//
// What stays here is this kind's record and this kind's words. The wrappers below keep their old
// names on purpose: this file is where a supervised run's record is written, and a reader
// following `saveRun` should not have to learn that it now means something general.

// reserveRun claims this attempt's namespace, and leaves a supervised record saying `started` in
// it.
//
// The provisional record is THIS KIND'S shape, which is the one thing the shared machinery does
// not decide: an attempt that dies before finalisation must be retrievable as a supervised
// attempt that had not finished, in the vocabulary a supervised run is read in.
func reserveRun(at time.Time, framesRoot string, propose func(time.Time) string) (
	id string, framesDir string, err error) {

	got, err := runs.Reserve(runs.Claim{
		At: at, FramesRoot: framesRoot, Propose: propose,
		Provisional: func(id string) any {
			return RunRecord{
				Version: 1, Run: id,
				Started: at.Format(time.RFC3339),
				Outcome: RunStarted,
				Because: "reserved; this attempt had not finished " +
					"when the record was last written",
			}
		},
	})
	return got.ID, got.Frames, err
}

// runPath is where a run's record is written.
func runPath(id string) string { return runs.Path(id) }

// saveRun writes the record into this run's OWN reservation and returns the path.
//
// A failure here is NOT a failure of the run. The run already happened; losing its record loses
// the ability to cite it, which is worth a line on stderr and nothing more.
func saveRun(r RunRecord) (string, error) {
	// WHICH DIRECTOR THIS WAS, stamped here rather than by each caller: there is one
	// place a supervised record is written and every one of them should answer the
	// question. See internal/buildid.
	//
	// Here and not in `runs.Finalise`, which is handed a record it may not look inside:
	// the field is this kind's, and the other kind stamps its own build on its own field.
	r.Build = buildid.Revision()
	return runs.Finalise(r.Run, r)
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
