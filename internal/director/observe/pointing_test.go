package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Choosing what to point at.
//
// The property every test here defends: a highlight must come from something Marco SAID. The
// tempting implementation — outline the biggest group on the current screen — passes every
// "does a box appear" check and means nothing, so these tests are written to fail it specifically.

// THE selection test. Marco points at a subject it made a claim about, not at the largest thing.
func TestMarcoPointsAtSomethingItSaidAndNotAtWhateverIsLargest(t *testing.T) {
	live, spoken, silent := watching(t)

	// Marco asked about ONE of the two groups. Nothing was ever said about the other.
	got := observe.WhatToPointAt([]observe.Proposal{asking(spoken.ID)}, nil, live)
	if got.Refusal != observe.PointingReady {
		t.Fatalf("refused with %q despite an open question", got.Refusal)
	}
	if !got.Referent.CanPoint() {
		t.Fatalf("nothing to point at: %q", got.Referent.Unavailable)
	}
	if got.Basis != observe.BasisOpenQuestion {
		t.Errorf("basis %q, want an open question", got.Basis)
	}

	allDrawnFrom(t, got.Referent.Regions, regionsFor(t, spoken, live), spoken.ID)
	// And it is not the group nobody said anything about — the answer a "pick a convenient
	// group" implementation would give whenever that group happened to sort first.
	if sharesAnything(got.Referent.Regions, regionsFor(t, silent, live)) {
		t.Error("pointed at the group Marco never mentioned")
	}
}

// Nothing said means nothing shown. Not "the first group", not an empty box.
func TestMarcoPointsAtNothingWhenItIsReferringToNothing(t *testing.T) {
	live, _, _ := watching(t)

	got := observe.WhatToPointAt(nil, nil, live)
	if got.Referent.CanPoint() {
		t.Fatalf("drew %d region(s) with nothing to refer to. A highlight is a sentence, "+
			"and this one would be Marco claiming to mean something it never said",
			len(got.Referent.Regions))
	}
	if got.Refusal != observe.NothingMeant {
		t.Errorf("refused with %q, want %q", got.Refusal, observe.NothingMeant)
	}
	if got.Say() == "" {
		t.Error("the refusal has no sentence a person could read")
	}
}

// An open question outranks a settled one, and both outrank an unasked interpretation.
func TestMarcoPointsAtWhatItIsAskingAboutFirst(t *testing.T) {
	live, first, second := watching(t)

	settled := asking(second.ID)
	settled.Status = observe.ProposalAnswered
	open := asking(first.ID)

	// Settled one FIRST in the slice, so an implementation that simply took the first
	// group-shaped proposal would choose it.
	got := observe.WhatToPointAt([]observe.Proposal{settled, open}, nil, live)
	if got.Basis != observe.BasisOpenQuestion {
		t.Fatalf("basis %q; the question Marco is asking right now is what \"this\" means",
			got.Basis)
	}
	want := regionsFor(t, first, live)
	if len(got.Referent.Regions) == 0 || len(want) == 0 ||
		got.Referent.Regions[0] != want[0] {
		t.Error("pointed at the settled question's subject rather than the open one's")
	}
}

// With no question at all, an interpretation Marco formed is still something it said.
func TestAnInterpretationIsEnoughToPointAt(t *testing.T) {
	live, group, _ := watching(t)

	h := observe.Hypothesis{
		Kind:     observe.PossibleChoiceGroup,
		Subject:  observe.Subject{Kind: observe.SubjectGroup, Ref: group.ID},
		Observed: "several controls appeared and disappeared together",
	}
	got := observe.WhatToPointAt(nil, []observe.Hypothesis{h}, live)
	if !got.Referent.CanPoint() {
		t.Fatalf("nothing to point at: %q / %q", got.Refusal, got.Referent.Unavailable)
	}
	if got.Basis != observe.BasisInterpretation {
		t.Errorf("basis %q, want an interpretation", got.Basis)
	}
	if got.Interpretation != h.Observed {
		t.Error("the observation sentence did not travel with the referent")
	}
}

// A screen-shaped subject does not consume the choice.
//
// A whole screen cannot be pointed at — outlining the window says nothing — so a proposal about
// one must not become the answer while a perfectly pointable group is sitting behind it.
func TestAWholeScreenSubjectDoesNotBlockAPointableOne(t *testing.T) {
	live, group, _ := watching(t)

	screen := observe.Proposal{
		ID: "q_screen", Status: observe.ProposalOpen,
		Subject: observe.Subject{Kind: observe.SubjectState, Ref: "state_1"},
	}
	got := observe.WhatToPointAt([]observe.Proposal{screen, asking(group.ID)}, nil, live)
	if !got.Referent.CanPoint() {
		t.Fatalf("the screen question swallowed the choice: %q / %q",
			got.Refusal, got.Referent.Unavailable)
	}
}

// Nothing watched is a different sentence from nothing meant.
func TestNothingWatchedAndNothingMeantAreDifferentAnswers(t *testing.T) {
	empty := observe.WhatToPointAt(nil, nil, observe.LiveGeometry{})
	if empty.Refusal != observe.NothingObserved {
		t.Errorf("refused with %q, want %q", empty.Refusal, observe.NothingObserved)
	}
	if empty.Say() == observe.NothingMeant.Say() {
		t.Error("\"I haven't looked\" and \"I have nothing to say\" read the same. They send " +
			"a person to different places")
	}
	for _, r := range []observe.PointingRefusal{observe.NothingObserved, observe.NothingMeant} {
		if r.Say() == "" {
			t.Errorf("%q has no reading", r)
		}
	}
	if observe.PointingReady.Say() != "" {
		t.Error("a successful pointing produced an apology")
	}
}

// Showing what a durable judgement refers to goes through RECOGNITION, never stored geometry.
//
// The whole reason "What Marco Knows" exists is a record whose stored envelope sits mostly above
// its own window and can therefore never be recognised again. A "show me" button that consulted
// that envelope would confidently outline a region of empty screen — and it would do it most
// eagerly for exactly the records that are most broken.
func TestShowingWhatAJudgementRefersToNeverUsesRememberedGeometry(t *testing.T) {
	live, group, _ := watching(t)

	// A session that recognised this live group as the remembered subject.
	recognised := asking(group.ID)
	recognised.Status = observe.ProposalAnswered
	recognised.Recognised = true
	recognised.RecognisedAs = "subj_remembered"

	got := observe.WhatRemembersThis("subj_remembered", []observe.Proposal{recognised}, live)
	if !got.Referent.CanPoint() {
		t.Fatalf("a recognised subject could not be pointed at: %q / %q",
			got.Refusal, got.Referent.Unavailable)
	}
	allDrawnFrom(t, got.Referent.Regions, regionsFor(t, group, live), group.ID)

	// The same subject, with NOTHING recognising it. This is the Explorer case, and the only
	// acceptable answer is a sentence.
	none := observe.WhatRemembersThis("subj_remembered", nil, live)
	if none.Referent.CanPoint() {
		t.Fatal("pointed at a subject nothing on screen was recognised as. The only geometry " +
			"available for that is the stored envelope, which is exactly what must never be " +
			"used to point")
	}
	if none.Refusal != observe.NotRecognisedHere {
		t.Errorf("refused with %q, want %q", none.Refusal, observe.NotRecognisedHere)
	}
	if none.Say() != "I remember this, but I can't currently point to it." {
		t.Errorf("the refusal reads %q; it must say Marco still remembers", none.Say())
	}
}

// A judgement recognised as some OTHER subject is not this one.
//
// The failure a loose match would produce: click "show me" on one remembered answer and get a
// highlight belonging to a different one, with no way to tell.
func TestShowingAJudgementDoesNotDriftToAnotherSubject(t *testing.T) {
	live, group, _ := watching(t)

	other := asking(group.ID)
	other.Recognised = true
	other.RecognisedAs = "subj_something_else"

	got := observe.WhatRemembersThis("subj_wanted", []observe.Proposal{other}, live)
	if got.Referent.CanPoint() {
		t.Fatal("pointed at another subject's referent")
	}
	if got.Refusal != observe.NotRecognisedHere {
		t.Errorf("refused with %q", got.Refusal)
	}
}

// An unrecognised proposal does not count as recognition.
func TestAnUnrecognisedProposalIsNotABridgeToARememberedSubject(t *testing.T) {
	live, group, _ := watching(t)

	p := asking(group.ID)
	p.RecognisedAs = "subj_remembered" // named, but Recognised was never set
	if got := observe.WhatRemembersThis("subj_remembered",
		[]observe.Proposal{p}, live); got.Referent.CanPoint() {
		t.Fatal("a proposal that never recognised anything was used as the bridge")
	}
}

// A question about a screen never borrows another subject's highlight.
//
// # The live failure this is written from
//
// Windows Settings. Marco asked "I keep seeing a screen that looks like a menu — is that what it
// is?", whose subject is a screen state, and screens are deliberately unpointable. A "show me what
// this refers to" button that asked the GENERAL question got back an interpretation about a
// seventeen-control group — because WhatToPointAt is supposed to skip a screen subject so it does
// not consume the choice — and would have drawn that group under the screen question's sentence.
//
// Two subjects, one box, and nothing on the surface able to tell them apart.
func TestAQuestionAboutAScreenNeverBorrowsAnotherSubjectsHighlight(t *testing.T) {
	live, group, _ := watching(t)

	screen := observe.Proposal{
		ID: "q_screen", Status: observe.ProposalOpen,
		Question: "I keep seeing a screen that looks like a menu. Is that what it is?",
		Subject:  observe.Subject{Kind: observe.SubjectState, Ref: "state_1"},
	}
	// A perfectly pointable group question sits right beside it, which is what the general
	// path would fall through to.
	alongside := asking(group.ID)

	props := []observe.Proposal{screen, alongside}
	got := observe.WhatThisQuestionMeans("q_screen", props, live)

	if got.Referent.CanPoint() {
		t.Fatalf("drew %d region(s) for a question about a whole screen. The only geometry "+
			"available for that belongs to a different subject", len(got.Referent.Regions))
	}
	if got.Referent.Unavailable != observe.ReferentNotAPart {
		t.Errorf("refused with %q, want %q — the honest answer is that a screen has no part "+
			"to single out, not that Marco lost sight of it",
			got.Referent.Unavailable, observe.ReferentNotAPart)
	}
	if got.Question != screen.Question {
		t.Errorf("the sentence beside the refusal is %q; it must be the question that was "+
			"asked about", got.Question)
	}
	// And the control: the general path DOES fall through to the other subject, which is
	// correct for "what are you referring to" and is exactly why the two cannot share a call.
	general := observe.WhatToPointAt(props, nil, live)
	if !general.Referent.CanPoint() {
		t.Fatal("the fixture no longer distinguishes the two paths; the general path must " +
			"still find the pointable group, or this test proves nothing")
	}
	if sharesAnything(got.Referent.Regions, general.Referent.Regions) {
		t.Error("the screen question ended up pointing where the general path points")
	}
}

// A named question that this session does not hold is a refusal, not a substitution.
//
// The stale-browser-tab case: a page holding a question id from a session that has since been
// replaced. Answering it with whatever Marco is talking about now would be silently wrong.
func TestPointingAtAQuestionNobodyHoldsRefuses(t *testing.T) {
	live, group, _ := watching(t)

	got := observe.WhatThisQuestionMeans("q_gone", []observe.Proposal{asking(group.ID)}, live)
	if got.Referent.CanPoint() {
		t.Fatal("a question that is not in this session was answered with another one's subject")
	}
	if got.Refusal != observe.NothingMeant {
		t.Errorf("refused with %q", got.Refusal)
	}
}

// A named question about a group still points, and says it is a question rather than a guess.
func TestPointingAtANamedGroupQuestionWorks(t *testing.T) {
	live, group, _ := watching(t)

	p := asking(group.ID)
	got := observe.WhatThisQuestionMeans(p.ID, []observe.Proposal{p}, live)
	if !got.Referent.CanPoint() {
		t.Fatalf("could not point at a named group question: %q / %q",
			got.Refusal, got.Referent.Unavailable)
	}
	if got.Basis != observe.BasisOpenQuestion {
		t.Errorf("basis %q, want an open question", got.Basis)
	}
	if got.Say() != "This is what I'm asking about." {
		t.Errorf("says %q beside the highlight", got.Say())
	}
	allDrawnFrom(t, got.Referent.Regions, regionsFor(t, group, live), group.ID)
}
