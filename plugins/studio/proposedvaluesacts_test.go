package main

// THE SAME QUESTIONS OF THE REAL ACTS, AGAINST A REAL LIBRARY.
//
// Next door is the page, with the backend stubbed. This is the window's own method against a
// routine on disk: what it does with a proposal it was given, and the two refusals that are the
// contract's rather than the page's.
//
// See [[ADR-277-a-request-may-propose-the-text-a-routine-types]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
)

// revisionOf is the routine's source as it stands, digested — which is what a proposal read a
// moment ago would have carried.
//
// Through the same contract the window uses, so the digest is the one `ReviewPlay` will compare
// against rather than a second opinion about what the file says.
func revisionOf(t *testing.T, a *Acts, r Routine) string {
	t.Helper()
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	i, err := d.PrepareInvocation(r.route(), nil)
	if err != nil {
		t.Fatalf("reading the routine: %v", err)
	}
	if i.SourceAt == "" {
		t.Fatal("the contract reports no revision for a routine on disk")
	}
	return i.SourceAt
}

// A PROPOSAL LEFT ALONE IS CONFIRMED AS A READING OF THE REQUEST.
func TestTheWindowConfirmsAProposalAsARecordOfWhatWasSaid(t *testing.T) {
	a, r := aRoutineThatNeedsText(t)
	at := revisionOf(t, a, r)

	got := a.ReviewPlay(r.ID, map[string]string{"message": "hello there"},
		map[string]string{"message": "hello there"}, at)
	if got.Token == "" {
		t.Fatalf("a proposal was refused: %q", got.Why)
	}
	if len(got.Says) == 0 || !strings.Contains(got.Says[0], "read out of what you said") {
		t.Errorf("the confirmation reads %v", got.Says)
	}
	if !strings.Contains(got.Says[0], "hello there") {
		t.Errorf("the confirmation does not show the text: %v", got.Says)
	}
}

// AND ONE THE PERSON CORRECTED IS CONFIRMED AS THEIRS.
func TestTheWindowConfirmsACorrectionAsTheirOwn(t *testing.T) {
	a, r := aRoutineThatNeedsText(t)
	at := revisionOf(t, a, r)

	got := a.ReviewPlay(r.ID, map[string]string{"message": "goodbye instead"},
		map[string]string{"message": "hello there"}, at)
	if got.Token == "" {
		t.Fatalf("a corrected proposal was refused: %q", got.Why)
	}
	if len(got.Says) == 0 || !strings.Contains(got.Says[0], "as you corrected it") {
		t.Errorf("a correction is confirmed as %v", got.Says)
	}
	// AND THE WORDS THEY REJECTED ARE NOT IN IT. What is being confirmed is what will be
	// typed, and the reading they threw away is not part of that.
	if strings.Contains(got.Says[0], "hello there") {
		t.Errorf("the confirmation still shows the proposal: %v", got.Says)
	}
}

// AND A PROPOSAL FOR A REVISION THAT HAS BEEN SAVED OVER IS REFUSED BY THE WINDOW.
//
// The contract's refusal, arriving at the surface somebody would meet it on. Not a copy of the
// rule here: a window with its own copy is a window that offers a press and is then told off for
// it.
func TestTheWindowRefusesAProposalForAnOlderRevision(t *testing.T) {
	a, r := aRoutineThatNeedsText(t)

	got := a.ReviewPlay(r.ID, map[string]string{"message": "hello there"},
		map[string]string{"message": "hello there"}, "sha256:something-else")
	if got.Token != "" {
		t.Fatal("a proposal read against another revision was offered for running")
	}
	if !strings.Contains(got.Why, "saved again") {
		t.Errorf("the refusal reads %q", got.Why)
	}
	// AND IT NAMES THE ROUTINE AND NOT THE VALUE.
	if strings.Contains(got.Why, "hello there") {
		t.Errorf("the refusal repeats the value: %q", got.Why)
	}
}

// AND AN ORDINARY PRESS IS WHAT IT ALWAYS WAS.
//
// No proposal and no revision is exactly the plain path, so every routine saved before any of this
// existed presses the way it did — and the confirmation gains no clause, because the person is
// reading their own text back.
func TestAnOrdinaryPressCarriesNoProposalAndNoClause(t *testing.T) {
	a, r := aRoutineThatNeedsText(t)

	got := a.ReviewPlay(r.ID, map[string]string{"message": "hello there"}, nil, "")
	if got.Token == "" {
		t.Fatalf("an ordinary press was refused: %q", got.Why)
	}
	if len(got.Says) == 0 {
		t.Fatal("the confirmation says nothing")
	}
	if strings.Contains(got.Says[0], "read out of what you said") ||
		strings.Contains(got.Says[0], "as you corrected it") {
		t.Errorf("a typed value is confirmed as something else: %v", got.Says)
	}
}
