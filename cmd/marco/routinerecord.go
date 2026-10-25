package main

// A ROUTINE'S RUN LEAVES A RECORD, AND THE EXECUTOR IS WHAT WRITES IT.
//
// # What was missing
//
// A supervised Act's run has been citable for a while: `cmd/director` claims a reference, prints
// it, and writes a record somebody can paste into a bug report. A saved routine's run announced
// `[result] <word>` and nothing else, so the terminal scrollback was the whole account — and
// Studio's run view said *a routine — no run record*. `Report` and `Inspect` already existed and
// were unreachable for exactly the routines the coordinate work is about.
//
// # Why the executor owns the reference and not the front end
//
// Because the front end is not what runs it, and two front ends run the same routine. Studio
// spawns `marco do` and so does the overlay; a reference minted by whichever surface happened to
// press Play would be a different reference for the same attempt depending on who asked, and a
// run somebody started from a terminal would have none at all. The thing that sends the input is
// the only thing present for every run of it.
//
// So the reservation is made here, before the first input, and the citation is printed. A surface
// READS that line (`runref.Parse`, which Studio already does for an Act) and reports what it was
// told rather than deciding anything.
//
// # And the record is this kind's own
//
// `internal/runs` owns the discipline — the place, the exclusive claim, the provisional record,
// the finalisation that cannot create — and is handed a record it may not look inside. This file
// is the record: a routine's words, a routine's fields, and not one number about evidence.
// See [[ADR-253-the-claim-is-shared-and-the-record-is-not]].

import (
	"fmt"
	"os"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/buildid"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runs"
)

// kindRoutine is what this kind of record calls itself.
//
// EXPLICIT, and the reason is the whole of ADR-253: a reader that cannot tell the two kinds apart
// would run a routine's `performed` through the supervised vocabulary and render it as an Act's
// verdict. A supervised record has no kind — it was written before there were two — so a record
// without one is that kind, and this is the discriminator that says otherwise.
//
// The WORD is `internal/runs`', because Studio reads it and there is no compiler between the two.
const kindRoutine = runs.KindRoutine

// modeUnstated is what a routine's record says about its mode when the source states none.
//
// Older source says nothing about how it was recorded, and guessing from an environment variable
// nobody can see afterwards would be a claim rather than a reading. It was the truth for every
// routine on disk until Studio could state a mode, and it stays the truth for every one of them:
// `orchestrator.ModeOf` reads the word the source carries, and a file that carries none reads
// back as this rather than as a default.
const modeUnstated = "unstated"

// modeOf is the word this record uses for what the source said it was recorded as.
//
// One place, because the record's vocabulary is the record's: an empty reading becomes the word
// `unstated` HERE rather than being written as an empty field that a reader would have to know to
// interpret. See `ran.Mode`.
func modeOf(did ran) string {
	if did.Mode == "" {
		return modeUnstated
	}
	return did.Mode
}

// routineRun is one attempt at a saved routine, as somebody would cite it in a bug report.
//
// The JSON names are deliberately the ones `plugins/studio/history.go` already reads — `run`,
// `routine`, `application`, `started`, `ended`, `outcome`, `because`, `build` — so the window
// lists this without being taught a second shape. `PastRun` even declares a `routine` field that
// nothing has ever written.
//
// What is NOT here is as deliberate: no `verified`, no `contradicted`, no `unconfirmed`. A routine
// presses what it was shown and looks at nothing afterwards, so those questions were never asked,
// and three zeroes would read as a run that asked them and found nothing. Absent is the truth.
type routineRun struct {
	Version int    `json:"version"`
	Kind    string `json:"kind"`
	Run     string `json:"run"`
	// Build is the revision of the `marco` that performed this run.
	//
	// The field that would have named the stale recorder a day of the owner's recordings
	// went into. Stamped where the record is written, not by each caller. See
	// internal/buildid.
	Build string `json:"build,omitempty"`
	// Routine, Application and Scope are the exact route — the identity Studio selects by.
	//
	// Three fields rather than one joined `<app>/<scope>/<slug>` string, because an
	// application's name is a file name and may hold anything a file name may hold. A
	// reader that had to split one string would be guessing where the parts were.
	Routine     string `json:"routine,omitempty"`
	Application string `json:"application,omitempty"`
	Scope       string `json:"scope,omitempty"`
	// Source is the digest of the saved source that actually ran, and Library the place it
	// was read from. Together they say WHICH routine this was, in a way that survives the
	// routine being saved again afterwards.
	Source  string `json:"source_digest,omitempty"`
	Library string `json:"library,omitempty"`
	// Mode and Contract are what the run was performed under: see modeUnstated and ran.
	Mode     string `json:"mode,omitempty"`
	Contract string `json:"contract,omitempty"`
	Started  string `json:"started"`
	// Ended is absent while the attempt has not finished, which is how a reader knows.
	Ended string `json:"ended,omitempty"`
	// Outcome is the routine's OWN word, unconverted, and Because the sentence the run
	// itself gave for it.
	Outcome string `json:"outcome"`
	Because string `json:"because,omitempty"`
	// Asked is how many input actions the run asked the machine for, or ABSENT when
	// nothing could say.
	//
	// A pointer, and that is the point: zero is a real answer — a run that refused before
	// the first press asked for nothing, and saying so is true. A host that cannot be asked
	// at all is a different fact, and writing zero for it would turn "nobody knows" into
	// "nothing happened". See oshost.Host.InputActions for what the number counts.
	Asked *int `json:"input_actions,omitempty"`
}

// aRoutineRun is the record's identity half: everything knowable before the run starts.
//
// Made once and filled in afterwards, so the provisional record left in the reservation and the
// finished record describe the same attempt. Two separately assembled records would be two
// answers to "which routine was this", and the one a person reads is whichever the run got to.
//
// # And the identity is ALL of it, including before the run
//
// `build`, `source_digest` and `contract` used to be added after the performance returned, which
// is the half of the run that may never happen. Kill the child and the reference held a record
// naming the routine and the hour and nothing about WHAT was running — no revision of `marco`, no
// digest of the source, no contract — which is the information an interrupted attempt is the one
// that most needs. They are knowable before the first input, so they are written before it: the
// revision is this binary's, and the digest and contract come off the one read `take` took. See
// `snapshot`.
func aRoutineRun(id string, at time.Time, rt routes.Route, did ran) routineRun {
	return routineRun{
		Version: 1, Kind: kindRoutine, Run: id,
		Build:   buildid.Revision(),
		Routine: rt.Slug, Application: rt.App, Scope: scopeOf(rt),
		Library: marcopaths.Routes(),
		Mode:    modeOf(did),
		Source:  did.Digest, Contract: did.Contract,
		Started: at.Format(time.RFC3339),
	}
}

// citeRoutineRun claims this attempt's reference and prints the line a person copies.
//
// # A record that could not be claimed is not a reason not to run
//
// The person asked for their routine, not for a file. A failure here costs the ability to cite
// the run, which is worth a line on stderr and nothing else — and the caller carries on with an
// empty reservation, which every step below treats as "there is no record".
//
// The citation is printed BEFORE the first input, because the thing it is most needed for is a run
// that then went wrong in a way that left nothing behind.
func citeRoutineRun(at time.Time, rt routes.Route, did ran) runs.Reservation {
	got, err := runs.Reserve(runs.Claim{At: at, Provisional: func(id string) any {
		r := aRoutineRun(id, at, rt, did)
		// NOT AN OUTCOME. See runs.Unfinished: the attempt has not finished, and
		// `ended` is absent, which is the pair a reader reads.
		r.Outcome = runs.Unfinished
		r.Because = "reserved; this attempt had not finished when the record was " +
			"last written"
		return r
	}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "marco: this run has no record: %v\n", err)
		return runs.Reservation{}
	}
	fmt.Println(runref.Cite(got.ID, runs.Path(got.ID)))
	return got
}

// keepRoutineRun writes what became of the attempt into its own reservation.
//
// # Why the outcome is not put on the citation line
//
// `runref.Ended` appends the word in brackets and `runref.Parse` strips it only when the word is
// one of the SEVEN SUPERVISED ones. `performed` is not, so a closing citation would print a word
// the parser leaves in the path, and a front end would show a record path with "(performed)" on
// the end of it. Teaching that parser a routine's words would make one parser answer for two
// vocabularies, which is the conversion ADR-253 forbids wearing a parser's clothes.
//
// It is also unnecessary. `[result] <word>` is already the wire line a front end reads for a
// routine, through the package that owns the seven words. One opening citation and one result
// line, each from the thing that owns it.
func keepRoutineRun(res runs.Reservation, at time.Time, rt routes.Route, did ran,
	out Outcome, because string, asked *int) {

	if res.ID == "" {
		return
	}
	// THE SAME IDENTITY THE RESERVATION WAS GIVEN, from the same `did`. Nothing is
	// re-derived here: a second reading of the file would be a second answer, and a save
	// between the two would make the finished record cite source the run never saw.
	r := aRoutineRun(res.ID, at, rt, did)
	r.Ended = time.Now().Format(time.RFC3339)
	// THE RUN'S OWN WORD, UNCONVERTED. Nothing here turns `performed` into `verified`:
	// this run pressed what it was shown and looked at nothing afterwards.
	r.Outcome, r.Because = string(out), because
	r.Asked = asked
	if _, err := runs.Finalise(res.ID, r); err != nil {
		// A FAILURE HERE IS NOT A FAILURE OF THE RUN. The run already happened.
		fmt.Fprintf(os.Stderr, "marco: the record of %s could not be written: %v\n",
			res.ID, err)
	}
}

// inputCounter is a host that can say how many input actions a run asked it for.
//
// An interface rather than `*oshost.Host`, for the same reason `unperformer` is one: the rule has
// to be exercisable with a host that is not the real one, whose backend sends real input.
type inputCounter interface {
	// InputActions is how many input actions this run asked the machine for. See
	// oshost.Host.InputActions.
	InputActions() int
}

// askedOf is how many input actions this run asked for, or nil when nothing can say.
//
// Nil and not zero. A dry-run host and a host from before this existed cannot answer, and writing
// zero for them would say the run touched nothing — which is the one thing the number is for.
func askedOf(d orchestrator.Deps) *int {
	h, ok := d.Hosts["*"].(inputCounter)
	if !ok {
		return nil
	}
	n := h.InputActions()
	return &n
}
