package resolveroute

// WHAT A ROUTINE ANSWERS TO, AS SOMETHING DIRECTOR CAN RESOLVE.
//
// A phrase is offered as another LABEL for the same candidate, which is the whole mechanism: the
// first arm of `dispatch.Decide` is an exact match over the offered labels, and it is
// deterministic, offline and free. So the claim these cases hold is that a library whose routines
// answer to several phrases resolves with no plugin configured and no model server running —
// stated as an assertion rather than left as a hope.
//
// See [[Command-aliases-proposal-2026-10-30]].

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/nlu"
)

// answering is a candidate with the phrases it also answers to.
func answering(c Candidate, phrases ...string) Candidate {
	c.Answers = phrases
	return c
}

// AN EXACT PHRASE RESOLVES WITH NOTHING RUNNING.
//
// No Advisor at all, which is the strongest form of the claim: not "a resolver was configured and
// happened not to be needed", but a library with nowhere to ask. The arm that answers is the
// deterministic matcher, and it says so — `exact`, and the resolver sentence says a phrase was
// answered without one.
func TestAPhraseResolvesOfflineWithNoResolverConfigured(t *testing.T) {
	r := Resolver{Advisor: nil}
	cs := []Candidate{answering(aCandidate("enter-freeplay", "trackmania", true),
		"go racing")}
	said := r.Resolve(context.Background(), "go racing", cs, at("trackmania"))
	if said.Kind != dispatch.IntentRun {
		t.Fatalf("the phrase came back as %q: %s", said.Kind, said.Reply)
	}
	if said.ID != "trackmania/focus/enter-freeplay" {
		t.Errorf("it selected %q", said.ID)
	}
	if said.Source != dispatch.SourceExact {
		t.Errorf("the phrase was answered by the %q arm, not the offline one",
			said.Source)
	}
}

// AND THE CANONICAL ROUTINE IS WHAT COMES BACK, never the phrase that found it.
//
// The identity and the NAME are the routine's own. A surface showing the words somebody typed in
// place of the routine's name is the failure ADR-086 named for the CLI's own alias — showing a
// person the alias as though it were canonical — and here it would cost more than a confusing
// sentence, because a person who cannot tell which of two namesakes is about to press something
// cannot consent to it.
func TestAPhraseResolvesToTheCanonicalRoutine(t *testing.T) {
	r := Resolver{}
	cs := []Candidate{answering(aCandidate("enter-freeplay", "trackmania", true),
		"go racing")}
	said := r.Resolve(context.Background(), "GO RACING", cs, at("trackmania"))
	if said.Name != "enter freeplay" {
		t.Errorf("the answer names %q rather than the play", said.Name)
	}
	if said.Slug != "enter-freeplay" || said.Scope == "" {
		t.Errorf("the answer carries %q at %q", said.Slug, said.Scope)
	}
}

// A PHRASE INHERITS ITS ROUTINE'S SCOPE AND CANNOT WIDEN IT.
//
// An in-place routine works only while its application is in front, and teaching it a phrase does
// not change that. The alternative is a routine pressing at coordinates in somebody else's
// window, reached by a phrase.
func TestAPhraseIsNotOfferedWhenItsRoutineIsNotEligible(t *testing.T) {
	r := Resolver{}
	cs := []Candidate{answering(aCandidate("trackmania-settings", "trackmania", false),
		"open the settings")}
	// NOTHING IS KNOWN TO BE IN FRONT, so an in-place routine is not eligible — and
	// neither is the phrase that reaches it.
	said := r.Resolve(context.Background(), "open the settings", cs,
		Context{Basis: Unknown})
	if said.Kind == dispatch.IntentRun {
		t.Fatalf("an in-place play was selected by phrase with nothing known to "+
			"be in front: %q", said.ID)
	}
	if said.Offered != 0 {
		t.Errorf("%d plays were offered", said.Offered)
	}
}

// AND A PHRASE REACHING A ROUTINE THAT WOULD REFUSE IS NOT OFFERED AT ALL.
func TestAPhraseForAnUnrunnableRoutineIsNotOffered(t *testing.T) {
	r := Resolver{}
	c := answering(aCandidate("enter-freeplay", "trackmania", true), "go racing")
	c.Runnable = false
	said := r.Resolve(context.Background(), "go racing", []Candidate{c},
		at("trackmania"))
	if said.Kind == dispatch.IntentRun {
		t.Errorf("a play that would refuse every press was selected by phrase")
	}
}

// A PHRASE TWO ROUTINES ANSWER TO SELECTS NEITHER AND ASKS WHICH.
//
// The rule the shared-name clarification already had, now counting phrases as names — because
// from the person's side they are the same claim on the same words. What must NOT happen is the
// phrase being handed to one of them as a label: that is an exact match, answered offline,
// selecting whichever the listing put first, with neither the resolver nor the person asked.
func TestAPhraseTwoRoutinesAnswerToAsksWhich(t *testing.T) {
	r := Resolver{}
	cs := []Candidate{
		answering(aCandidate("ark-settings", "arkascended", true), "settings"),
		answering(aCandidate("trackmania-settings", "trackmania", true), "settings"),
	}
	said := r.Resolve(context.Background(), "settings", cs, at("trackmania"))
	if said.Kind != dispatch.IntentClarify {
		t.Fatalf("a phrase two plays answer to came back as %q selecting %q",
			said.Kind, said.ID)
	}
	if said.ID != "" {
		t.Errorf("it selected %q anyway", said.ID)
	}
	if len(said.Candidates) != 2 {
		t.Errorf("the question narrows to %q", said.Candidates)
	}
	// AND IT SAYS WHICH ONES, naming the phrase they answer to rather than printing
	// one name twice.
	if !strings.Contains(said.Reply, "answers to") {
		t.Errorf("the question does not say how they answer to it: %q", said.Reply)
	}
}

// AND A ROUTINE KEEPS ITS OWN NAME WHATEVER ANYBODY ELSE ANSWERS TO.
//
// The labelling order read as a claim: own names are claimed first, phrases second, and a label
// this package invents last. So a phrase can never displace the routine that is actually called
// that — and a phrase nobody else wants still gets offered.
func TestAPhraseNeverDisplacesARoutinesOwnName(t *testing.T) {
	cs := []Candidate{
		aCandidate("settings", "", false),
		answering(aCandidate("trackmania-settings", "trackmania", true),
			"open the settings"),
	}
	labels, byLabel := labelled(cs)
	if byLabel["settings"].ID != "global/settings" {
		t.Errorf("`settings` labels %q", byLabel["settings"].ID)
	}
	if byLabel["open the settings"].ID != "trackmania/focus/trackmania-settings" {
		t.Errorf("the phrase labels %q", byLabel["open the settings"].ID)
	}
	// AND THE PHRASE IS AN EXTRA WAY IN RATHER THAN A REPLACEMENT: three labels for
	// two routines, because the one with a phrase still answers to its own name.
	if len(labels) != 3 {
		t.Errorf("%d labels were offered: %q", len(labels), labels)
	}
	if byLabel["trackmania-settings"].ID != "trackmania/focus/trackmania-settings" {
		t.Errorf("the play lost its own name to its phrase: %q",
			byLabel["trackmania-settings"].ID)
	}
}

// AND A SHARED PHRASE IS CLAIMED BY NOBODY.
//
// The load-bearing half of the rule above. Handed to one of the routines that answers to it, the
// phrase is an exact match on that label and the clarification above never runs.
func TestASharedPhraseIsOfferedAsNobodysLabel(t *testing.T) {
	cs := []Candidate{
		answering(aCandidate("ark-settings", "arkascended", true), "settings"),
		answering(aCandidate("trackmania-settings", "trackmania", true), "settings"),
	}
	_, byLabel := labelled(cs)
	for label, c := range byLabel {
		if nlu.Key(label) == nlu.Key("settings") {
			t.Errorf("the shared phrase was handed to %q as %q", c.ID, label)
		}
	}
}

// AND A ROUTINE ANSWERING TO TWO PHRASES THE MATCHER CANNOT TELL APART IS STILL ONE ROUTINE.
//
// The counting rule, which is easy to get wrong in the other direction: `settings in ark` and
// `settings-ark` reduce to one key, so counting them twice would make a single routine look like
// a collision with itself and clarify about one candidate.
func TestOneRoutineWithTwoPhrasesThatReduceAlikeIsNotACollision(t *testing.T) {
	r := Resolver{}
	cs := []Candidate{answering(aCandidate("ark-settings", "arkascended", true),
		"settings in ark", "settings ark")}
	said := r.Resolve(context.Background(), "settings in ark", cs, at("arkascended"))
	if said.Kind != dispatch.IntentRun {
		t.Fatalf("one play answering to two alike phrases came back as %q: %s",
			said.Kind, said.Reply)
	}
	if said.ID != "arkascended/focus/ark-settings" {
		t.Errorf("it selected %q", said.ID)
	}
}

// A ROUTINE'S OWN NAME IS NOT DRAGGED INTO AMBIGUITY BY SOMEBODY ELSE'S PHRASE.
//
// The precedence rule, and it was wrong here while `routes` held it everywhere else: phrases were
// counted as names, so a routine CALLED `settings` and a second taught to answer to "settings"
// made the key shared — the first lost its own name in the labelling pass and a request for
// `settings` became a question about the routine that is actually called that.
//
// A name wins. The phrase is shadowed by it, and the routine it belongs to is still reachable by
// its own name and by any other phrase it has.
func TestARoutinesOwnNameOutranksAnotherRoutinesPhrase(t *testing.T) {
	r := Resolver{}
	cs := []Candidate{
		aCandidate("settings", "", false),
		answering(aCandidate("trackmania-settings", "trackmania", true), "settings",
			"open the settings"),
	}
	said := r.Resolve(context.Background(), "settings", cs, at("trackmania"))
	if said.Kind != dispatch.IntentRun {
		t.Fatalf("a request for a play's own name came back as %q: %s",
			said.Kind, said.Reply)
	}
	if said.ID != "global/settings" {
		t.Errorf("it selected %q rather than the play called settings", said.ID)
	}
	// AND THE SHADOWED ROUTINE IS STILL REACHABLE, by its name and its other phrase.
	for _, said := range []string{"trackmania settings", "open the settings"} {
		got := r.Resolve(context.Background(), said, cs, at("trackmania"))
		if got.ID != "trackmania/focus/trackmania-settings" {
			t.Errorf("%q selected %q", said, got.ID)
		}
	}
}

// AND A SHADOWED PHRASE IS NOT OFFERED AS A LABEL AT ALL.
//
// The mechanism behind the rule above: a phrase reducing to a key some routine is CALLED is never
// handed out, so nothing can match it offline and select the wrong routine.
func TestAPhraseThatAnotherRoutineIsCalledIsNotOffered(t *testing.T) {
	cs := []Candidate{
		aCandidate("settings", "", false),
		answering(aCandidate("trackmania-settings", "trackmania", true), "settings"),
	}
	labels, byLabel := labelled(cs)
	if byLabel["settings"].ID != "global/settings" {
		t.Errorf("`settings` labels %q", byLabel["settings"].ID)
	}
	for _, l := range labels {
		if c := byLabel[l]; nlu.Key(l) == nlu.Key("settings") &&
			c.ID != "global/settings" {
			t.Errorf("the shadowed phrase was offered as %q for %q", l, c.ID)
		}
	}
}

// AND TWO ROUTINES CALLED THE SAME THING ARE STILL A QUESTION, naming only them.
//
// The other direction, and the half that keeps the precedence from becoming a hole: when routines
// are CALLED this, they are the question — and a third that merely answers to it is not part of
// it, because listing it would offer somebody a routine the name does not belong to.
func TestNamesakesAreTheQuestionAndAPhraseHolderIsNotPartOfIt(t *testing.T) {
	r := Resolver{}
	cs := []Candidate{
		aCandidate("settings", "arkascended", true),
		aCandidate("settings", "trackmania", true),
		answering(aCandidate("notepad-settings", "notepad", true), "settings"),
	}
	said := r.Resolve(context.Background(), "settings", cs, at("trackmania"))
	if said.Kind != dispatch.IntentClarify || said.ID != "" {
		t.Fatalf("two plays called settings answered %q with %q",
			said.Kind, said.ID)
	}
	if len(said.Candidates) != 2 {
		t.Errorf("the question narrows to %q, and only the two CALLED settings are "+
			"what the name is ambiguous between", said.Candidates)
	}
	if strings.Contains(said.Reply, "notepad") {
		t.Errorf("a play that merely answers to the name is in the question: %q",
			said.Reply)
	}
}

// A NAME WINS EVEN WHEN SEVERAL ROUTINES ANSWER TO IT BY PHRASE.
//
// WRITTEN FOR A SURVIVOR. With one phrase holder the counting reaches the same answer either way,
// so the rule that a name decides the question survived its own test. Two phrase holders separate
// them: counted together, the key is shared and a request for it becomes a question — about a
// routine that is actually CALLED that, with two others merely answering to it.
func TestANameDecidesTheQuestionWhateverAnswersToItByPhrase(t *testing.T) {
	r := Resolver{}
	cs := []Candidate{
		aCandidate("settings", "", false),
		answering(aCandidate("ark-settings", "arkascended", true), "settings"),
		answering(aCandidate("trackmania-settings", "trackmania", true), "settings"),
	}
	said := r.Resolve(context.Background(), "settings", cs, at("trackmania"))
	if said.Kind != dispatch.IntentRun {
		t.Fatalf("a play's own name came back as %q against two phrases: %s",
			said.Kind, said.Reply)
	}
	if said.ID != "global/settings" {
		t.Errorf("it selected %q", said.ID)
	}
}
