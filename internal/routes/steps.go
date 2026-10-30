package routes

// A ROUTINE'S ACCOUNT OF ITSELF, AND WHY IT IS BOUND TO THE TWO FILES IT DESCRIBES.
//
// The `.marco` is the only executable truth about a routine. What it does NOT record is the step
// list it was rendered from, and until there was somewhere to put one, nothing could tell a routine
// whose source came from an edited step list from one that came straight off its demonstration: a
// demonstrated routine gets no provenance sidecar, so the only available answer was to render the
// demonstration again and compare — which says "hand-written" and "written by an older generator"
// in the same voice.
//
// So a routine whose source is NOT the render of its demonstration writes its step list down
// beside itself, and that file is the account of what the saved source is the render OF. Four
// fields and no history: what the routine does now is necessary, and a record of how it got there
// is not.
//
// # It is a stored RENDERING BASIS and not a claim that anybody edited anything
//
// Two things produce one: a step edit, and a re-simplification — which renders under rules that are
// not the default, so its source is not what the demonstration produces today and never will be,
// and an account of what was written is the only thing that can ever explain that file. So the
// presence of an account means "the basis this source was rendered from is stored here", and
// nothing in this package says who stored it or why. A surface that wants to say "you edited this"
// needs evidence this file does not carry.
//
// # It is believed only while it still describes what is on disk
//
// An account carries a digest of the source it rendered and a digest of the demonstration it
// descends from, and [Registry.Steps] answers with one of six states rather than with the file. A
// digest that no longer matches means the account is not about the routine in front of it, and
// NOTHING may be rendered from it — which is the whole of "a stale account must never silently
// replace the executable source's meaning".
//
// A mismatch is reported as what it is and not as what it suggests. A source digest that moved is
// consistent with somebody editing the file by hand AND with an account write that failed after
// the source landed, and nothing on disk distinguishes them.
//
// # Execution never reads this
//
// A run reads the `.marco` and nothing else, so an account that is missing, old or unreadable
// cannot change what a routine does. What it blocks is further EDITING, which is reported.
//
// See [[Step-editing-proposal-2026-10-30]].

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// stepsExt is the suffix for a routine's step list, beside its source.
const stepsExt = ".steps.json"

// StepsVersion is the shape of the file written today.
//
// A file stating anything else is UNREADABLE rather than absent: a version from the future was
// written by a build that knew something this one does not, and treating it as "no account" would
// quietly discard it on the next edit.
const StepsVersion = 1

// StoredSteps is a routine's step list and what it is bound to.
type StoredSteps struct {
	Version int `json:"version"`
	// Source is the digest of the `.marco` this step list renders to.
	Source string `json:"source"`
	// Demonstration is the digest of the `.rec.json` this step list descends from.
	//
	// SEPARATE FROM THE SOURCE, because the two move for different reasons and the
	// sentence a person reads differs: a source that moved means the file and these steps
	// no longer agree, and a demonstration that moved means somebody recorded the routine
	// again.
	Demonstration string `json:"demonstration"`
	// Steps is the step list itself, carried as written.
	//
	// RAW ON PURPOSE. This package stores a routine's files and digests them; what a step
	// IS belongs to `internal/macroir`, and a second definition of it here would be a
	// second thing to disagree with the first. The demonstration beside it is kept the
	// same way.
	Steps json.RawMessage `json:"steps"`
}

// StepsState is what an account on disk says about the routine it sits beside.
type StepsState string

const (
	// StepsNone is no file at all: nothing has stored a basis, so the demonstration is
	// the basis.
	//
	// EVERY ROUTINE ALREADY ON DISK IS THIS, which is why absence may never read as a
	// claim about anything. The inference is one-way.
	StepsNone StepsState = "none"
	// StepsIntact is an account that still describes both files beside it. The only
	// state anything may be rendered from.
	StepsIntact StepsState = "intact"
	// StepsMismatched is an account whose source digest no longer matches the `.marco`.
	//
	// Consistent with a hand edit AND with an account write that failed after the source
	// landed, so it is never reported as either.
	StepsMismatched StepsState = "mismatched"
	// StepsAdrift is an account bound to a demonstration that is no longer there — the
	// routine was recorded again, or its recording has gone.
	StepsAdrift StepsState = "adrift"
	// StepsOrphaned is an account with no source beside it.
	StepsOrphaned StepsState = "orphaned"
	// StepsUnreadable is a file that cannot be parsed, or states a version this build
	// does not know. SAID, never treated as absent.
	StepsUnreadable StepsState = "unreadable"
)

// Usable is whether a step list may be rendered from. Only one state is.
func (s StepsState) Usable() bool { return s == StepsIntact }

// StepsPath is where a routine's account sits.
func (r Registry) StepsPath(rt Route) string {
	return filepath.Join(r.locDir(rt), rt.Slug+stepsExt)
}

// HasSteps is whether a routine has an account file at all, whatever it says.
func (r Registry) HasSteps(rt Route) bool { return fileExists(r.StepsPath(rt)) }

// readSteps is the ONE read of the account file, and the one place "there is none" is told apart
// from "it could not be read".
//
// EVERY OTHER ERROR IS NOT ABSENCE, and the first version of this answered absence to all of them.
// A permission, a directory in the file's place, a name too long, a handle something else is
// holding: each came back as `StepsNone`, which is the single answer that licenses rendering the
// demonstration instead and discarding whatever the file says. The honest answer to a question
// that could not be asked is that it could not be asked — `Collision` carries the same correction,
// for the same reason and after the same defect.
func (r Registry) readSteps(rt Route) (raw []byte, found bool, err error) {
	raw, err = os.ReadFile(r.StepsPath(rt))
	switch {
	case err == nil:
		return raw, true, nil
	case os.IsNotExist(err):
		return nil, false, nil
	}
	return nil, true, err
}

// StepsDigest is a digest of the account file as it stands, with "" for no file at all, and
// `readable` false for a file that is there and could not be read.
//
// What a review binds itself to, beside the source and the demonstration: an account that moved
// while somebody read a review means something else wrote this routine's step list.
//
// THE THIRD ANSWER IS WHY THIS RETURNS TWO VALUES. An unreadable file answering "" would be an
// unreadable file answering the same as an absent one, and a review bound to "" would compare ""
// against "" at the write and go ahead — having never read one of the three files it is about.
// Every caller refuses instead.
func (r Registry) StepsDigest(rt Route) (digest string, readable bool) {
	raw, found, err := r.readSteps(rt)
	switch {
	case err != nil:
		return "", false
	case !found:
		return "", true
	}
	return DigestOf(string(raw)), true
}

// Steps reads a routine's account and what it still describes.
//
// The returned account is filled whenever the file parsed, whatever the state — a caller refusing
// an edit still wants to say what the file claims. Only [StepsIntact] licenses rendering.
func (r Registry) Steps(rt Route) (StoredSteps, StepsState) {
	raw, found, rerr := r.readSteps(rt)
	switch {
	case rerr != nil:
		// THERE AND UNREADABLE, which is not the same as not being there. See
		// readSteps.
		return StoredSteps{}, StepsUnreadable
	case !found:
		return StoredSteps{}, StepsNone
	}
	var s StoredSteps
	if err := json.Unmarshal(raw, &s); err != nil || s.Version != StepsVersion ||
		len(s.Steps) == 0 || s.Source == "" || s.Demonstration == "" {
		// A FILE THAT CANNOT ANSWER IS NOT AN ABSENT FILE. An account missing
		// either digest is one nothing can check, which is the single thing this
		// file exists to make impossible — so it is unreadable rather than
		// believed.
		return StoredSteps{}, StepsUnreadable
	}
	src, err := os.ReadFile(r.Path(rt))
	if err != nil {
		return s, StepsOrphaned
	}
	if DigestOf(string(src)) != s.Source {
		return s, StepsMismatched
	}
	rec, ok := r.LoadRecording(rt)
	if !ok || DigestOf(string(rec)) != s.Demonstration {
		return s, StepsAdrift
	}
	return s, StepsIntact
}

// SaveSteps writes a routine's account, bound to the two files it describes.
//
// Written by whatever rendered the source — a step edit, a rebuild of an edited routine, a move of
// one, a repair of one, or a re-simplification, which is the one with no edit in it at all. The
// file records a basis and not an author.
//
// # It is written LAST, and it cannot be written first
//
// The digest it records is of the source it describes, so there is nothing to record until that
// source is on disk. A caller therefore writes the assets, then the source, then this — and a
// failure here leaves the source in force with an account that no longer matches it, which blocks
// further editing and is reported. Execution is unaffected: a run reads only the `.marco`.
//
// Atomically, through the same helper the provenance sidecar uses: a reader never sees a
// half-written file, so "partially updated" is not a state this can be in. Only old, which the
// digests catch.
func (r Registry) SaveSteps(rt Route, s StoredSteps) error {
	switch {
	case rt.Slug == "":
		return fmt.Errorf("routes: a routine needs a name")
	case len(s.Steps) == 0:
		return fmt.Errorf("routes: %s's account has no steps in it", rt.Slug)
	case s.Source == "" || s.Demonstration == "":
		// AN UNBOUND ACCOUNT IS THE HAZARD ITSELF. A file with no digests is one
		// nothing can check, and writing one would be writing the thing every
		// rule above exists to prevent.
		return fmt.Errorf("routes: %s's account would not say which source and "+
			"demonstration it describes, so it was not written", rt.Slug)
	}
	s.Version = StepsVersion
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(r.StepsPath(rt), append(raw, '\n'))
}

// DeleteSteps removes a routine's account. Absent is not a failure.
func (r Registry) DeleteSteps(rt Route) error {
	err := os.Remove(r.StepsPath(rt))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
