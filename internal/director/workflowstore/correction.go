package workflowstore

// A PERSON CHANGING WHAT MARCO LEARNED, WITHOUT CHANGING WHAT THEY DID.
//
// # The owner's requirement
//
//	And if you say "ignore that T," Marco should update the learned procedure without
//	falsifying the raw recording.
//
//	This is how we give the user authority over what they taught while keeping the runtime
//	strict about what it can execute.
//
// Which needs exactly two things to be true at once, and they pull in opposite directions: the
// procedure must stop doing the thing, and the record of the demonstration must still say the
// person did it. So a correction REMOVES A STEP and KEEPS IT — out of `Steps`, where it would
// be performed, and into `Corrections`, where it is provenance. The raw demonstration is not
// touched at all; it is a separate file and this package cannot reach it.
//
// # Why this does not bump the artifact version
//
// Version 3's own rule is that a reader which ignored a new field would perform something
// different from what the file describes — that is the argument the version refuses an unknown
// file on. It does not apply here, and the reason is the shape of the correction rather than a
// judgement about how important it is: the step is GONE from `Steps`, so a reader that knows
// nothing about `corrections` performs exactly the corrected procedure. The field it ignores is
// the explanation, not the instruction.
//
// Asserted by TestACorrectedProcedureIsCorrectForAReaderThatIgnoresCorrections, which is the
// test that would fail if somebody ever made a correction something a performer has to consult.
//
// # And why a correction keeps the artifact's own instant
//
// `SavedAt` is when the demonstration was saved, and it is the LINK to the raw record beside it
// (see evidence.go). A correction that restamped it would orphan the demonstration it is a
// correction OF, which is the one thing this file exists to avoid. `CorrectedAt` is separate
// and says when the person changed their mind.

import (
	"fmt"
	"strings"
	"time"
)

// MaxCorrectionWords bounds what a person's own words may be stored as.
//
// A sentence, not a paragraph. The same reasoning as MaxTargetLabelLength: a free-text field
// with no bound is where captured content eventually arrives, and this one is written by a
// person at a prompt rather than read off a screen.
const MaxCorrectionWords = 200

// Correction is one change a person made to the learned procedure.
type Correction struct {
	// At is when they said it, in UTC.
	At time.Time `json:"at"`
	// Said is their own words, when they gave any. Evidence of WHY, and never a rule:
	// nothing parses this, matches on it, or reaches another workflow with it.
	Said string `json:"said,omitempty"`
	// Dropped is the step that was removed, as it was numbered when they saw it.
	//
	// One-based, because that is how `director sequences` prints the steps and how a
	// person refers to one. A correction that numbered them differently from the report
	// the person read would remove the wrong step and look like a working feature.
	Dropped int `json:"dropped"`
	// Step is the removed step ITSELF, so the artifact still says what was taken out.
	//
	// The whole step and not a summary of it. A person who corrects a mistake and then
	// wants it back has everything here; a person auditing the routine can see that a
	// click on that control was demonstrated and deliberately not kept.
	Step Step `json:"step"`
}

// Dropping removes one step from the learned procedure and records why.
//
// Returns a NEW artifact. The original is untouched, which matters because the caller holds the
// thing that is on disk: a correction that failed halfway through would otherwise leave a
// procedure nobody demonstrated.
//
// `step` is one-based, as the reports print it. A step nobody has is refused with its own
// sentence rather than clamped — "ignore step 4" about a three-step routine is a person
// describing something Marco should go and look at, not an index to round down.
func (a Artifact) Dropping(step int, said string, at time.Time) (Artifact, error) {
	if step < 1 || step > len(a.Steps) {
		return a, fmt.Errorf("there is no step %d: %q has %d",
			step, a.Name, len(a.Steps))
	}
	said = strings.TrimSpace(said)
	if len([]rune(said)) > MaxCorrectionWords {
		said = string([]rune(said)[:MaxCorrectionWords])
	}
	out := a
	// COPIED, both lists. Appending to a slice this artifact shares with its caller is
	// how one correction silently becomes two.
	out.Steps = make([]Step, 0, len(a.Steps)-1)
	out.Steps = append(out.Steps, a.Steps[:step-1]...)
	out.Steps = append(out.Steps, a.Steps[step:]...)
	out.Corrections = make([]Correction, len(a.Corrections), len(a.Corrections)+1)
	copy(out.Corrections, a.Corrections)
	out.Corrections = append(out.Corrections, Correction{
		At: at.UTC(), Said: said, Dropped: step, Step: a.Steps[step-1],
	})
	out.CorrectedAt = at.UTC()
	return out, nil
}

// Corrected reports whether a person has changed this procedure since it was demonstrated.
func (a Artifact) Corrected() bool { return len(a.Corrections) > 0 }
