package main

import (
	"strings"
	"testing"
)

// WHAT MARCO THOUGHT, NOT WHAT A REPORT RECONSTRUCTED.
//
// # Why this is instrumentation rather than scaffolding
//
// "No pixel controls were on offer" has at least four causes with different remedies: the
// screen never settled, the repair for this epoch was already spent, the reading was judged
// unnecessary, or nothing was placed yet. From the World alone they are indistinguishable, and
// this project has now reconstructed that decision from indirect evidence several times and
// been wrong at least once — `0 landed on nothing` was literally true while the thing the
// person pressed had never been a candidate.
//
// So the gate says what it decided, at the moment it decides it. The value of that rests
// entirely on it being the SAME decision production acted on: a diagnostic that recomputed the
// verdict would be a second implementation, free to agree with the code while disagreeing with
// the run.

// The note is written by the gate itself, on every path it can take.
func TestTheEscalationNoteComesFromTheDecisionItself(t *testing.T) {
	// IGNORANCE. No registry at all: the gate buys a look and says so.
	rt := &Runtime{}
	got := rt.moreEvidenceIsWorthBuying()
	if !got.Buy {
		t.Fatal("a Director with nothing watching declined to spend; this test no longer " +
			"covers the path it names")
	}
	if note := rt.lastEscalation(); note == "" {
		t.Error("the gate decided to buy and recorded no reason.\nA silent decision is " +
			"exactly the case that has to be reconstructed afterwards, which is what this " +
			"diagnostic exists to stop.")
	}

	// AND THE NOTE FOLLOWS THE DECISION rather than being written once. A different
	// Runtime taking a different path must leave a different sentence.
	g, store := watchedRegistry(t)
	watchNow(t, g, store, "epicgameslauncher")
	placed := &Runtime{observations: g}

	first := placed.moreEvidenceIsWorthBuying()
	firstNote := placed.lastEscalation()
	if firstNote == "" {
		t.Fatal("a placed reading recorded no escalation note")
	}
	if firstNote == rt.lastEscalation() {
		t.Error("a placed reading and an ignorant one left the same note, so the note is " +
			"not describing the decision")
	}

	// THE SECOND ASK ON ONE EPOCH IS A DIFFERENT DECISION, and must read differently.
	// This is the bound working, and it used to be indistinguishable from a screen that
	// never settled — see the two refusals in moreEvidenceIsWorthBuying.
	second := placed.moreEvidenceIsWorthBuying()
	secondNote := placed.lastEscalation()
	if first.Buy && second.Buy && firstNote == secondNote {
		t.Error("two purchases on one settled screen left one note; the budget's refusal " +
			"is invisible")
	}
	if !second.Buy && !strings.Contains(secondNote, "spent") &&
		!strings.Contains(secondNote, "settled") {
		t.Errorf("the refusal says %q, which names neither the spent budget nor the "+
			"unsettled screen — the two refusals this note exists to separate", secondNote)
	}
}

// THE NOTE NAMES BOTH VERDICTS AND BOTH PURCHASES.
//
// Structural and semantic sufficiency are separate judgements answering separate questions, and
// a reading can be perfectly described and still say nothing about which state it is. Collapsing
// them in the report would re-create the confusion that made every Xbox game one Place.
func TestTheEscalationNoteSeparatesTheVerdictsAndThePurchases(t *testing.T) {
	g, store := watchedRegistry(t)
	watchNow(t, g, store, "epicgameslauncher")
	rt := &Runtime{observations: g}

	got := rt.moreEvidenceIsWorthBuying()
	note := rt.lastEscalation()
	if !strings.Contains(note, "structurally") || !strings.Contains(note, "semantically") {
		t.Errorf("the note %q does not report both verdicts. They answer different "+
			"questions — arrangement, and which state this is — and a reader who cannot "+
			"see both cannot tell a broken accessibility tree from a screen that simply "+
			"does not say where it is.", note)
	}
	if got.Buy && !strings.Contains(note, "buy=") {
		t.Errorf("the note %q does not say what was bought", note)
	}
	if got.Buy && !strings.Contains(note, "read=") {
		t.Errorf("the note %q does not say whether the reading was bought. A pass that "+
			"cannot read is the defect ADR-129 exists for, and it is invisible without "+
			"this.", note)
	}
}

// AND IT REACHES THE REPORT FROM THE RUNTIME, not from a second computation there.
func TestTheReportReadsTheGatesOwnNote(t *testing.T) {
	src := mustReadSource(t, "observewiring.go")
	if !containsAll(src, "st.Escalation = s.rt.lastEscalation()") {
		t.Error("the sample no longer carries the gate's own escalation note.\nA report " +
			"that recomputed the verdict would be a second implementation of the policy, " +
			"free to agree with the code while disagreeing with the run that produced it.")
	}
}
