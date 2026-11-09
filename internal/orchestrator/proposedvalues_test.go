package orchestrator

// A VALUE A REQUEST PROPOSED, ON ITS WAY TO A CONFIRMATION.
//
// Two things this file is about, and they are the two that cannot be left to a surface. WHERE A
// VALUE CAME FROM is derived here by comparing what was offered with what came back, because a
// surface that prefilled a box and forgot to relabel it would tell somebody a model read their
// sentence correctly when they are the one who fixed it. And A PROPOSAL CANNOT OUTLIVE THE
// REVISION IT WAS MADE FOR, compared on the digest and never on the names, because a comparison of
// names passes exactly the case the rule exists to refuse.
//
// See [[ADR-277-a-request-may-propose-the-text-a-routine-types]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// proposing prepares an invocation for a routine, with what was proposed and what will run.
func proposing(t *testing.T, d Deps, rt routes.Route, values, proposed map[string]string,
	at string) Invocation {

	t.Helper()
	i, err := d.PrepareProposed(rt, values, proposed, at)
	if err != nil {
		t.Fatalf("PrepareProposed: %v", err)
	}
	return i
}

// digestNow is the routine's source as it stands, which is what a proposal would have been read
// from a moment ago.
func digestNow(t *testing.T, d Deps, rt routes.Route) string {
	t.Helper()
	i, err := d.PrepareInvocation(rt, nil)
	if err != nil {
		t.Fatalf("reading the play: %v", err)
	}
	return i.SourceAt
}

// WHERE EACH VALUE CAME FROM IS DERIVED, NOT TAKEN ON TRUST.
//
// The three things that can have happened to a prefilled box: nobody proposed anything for it, the
// proposal was left alone, or the person changed it.
func TestWhereAValueCameFromIsDerivedFromWhatWasProposed(t *testing.T) {
	d, rt := aRoutineWithAValue(t)
	at := digestNow(t, d, rt)

	// MISSING IS NOT EMPTY, here too. The cases are written with an explicit `offered`
	// flag rather than inferred from `was` being empty, because that inference is exactly
	// the defect: a proposal of the empty string is a proposal, and it has to be told
	// apart from no proposal at all. A mutation that tested the length instead of the
	// presence survived a version of this test that had no empty-proposal case in it.
	for _, c := range []struct {
		why     string
		runs    string
		was     string
		offered bool
		want    string
	}{
		{"a proposal left alone", "hello there", "hello there", true,
			routes.FromExtracted},
		{"a proposal corrected", "goodbye", "hello there", true, routes.FromEdited},
		{"a proposal corrected to nothing", "", "hello there", true, routes.FromEdited},
		{"an empty proposal left alone", "", "", true, routes.FromExtracted},
		{"an empty proposal filled in", "hello there", "", true, routes.FromEdited},
		{"a value nobody proposed", "hello there", "", false, routes.FromTyped},
		{"nothing proposed and nothing given", "", "", false, routes.FromTyped},
	} {
		proposed := map[string]string{}
		if c.offered {
			proposed["message"] = c.was
		}
		i := proposing(t, d, rt, map[string]string{"message": c.runs}, proposed, at)
		if i.Why != "" {
			t.Fatalf("%s: %s", c.why, i.Why)
		}
		if got := i.From["message"]; got != c.want {
			t.Errorf("%s is reported as %q, want %q", c.why, got, c.want)
		}
	}
}

// AND THE FOUR WORDS ARE FOUR DIFFERENT WORDS.
//
// `saidWhere` branches on all four, so the compiler already refuses two of them being the same
// string — a duplicate case in an expression switch. That is a stronger guarantee than a test and
// it is the compiler's rather than this file's, which is exactly why this is here: the guarantee
// lasts only as long as one function happens to branch on all four, and a word dropped from that
// switch would take the guarantee with it silently.
func TestTheFourProvenanceWordsAreFourDifferentWords(t *testing.T) {
	words := map[string]string{
		"typed":     routes.FromTyped,
		"parsed":    routes.FromParsed,
		"extracted": routes.FromExtracted,
		"edited":    routes.FromEdited,
	}
	seen := map[string]string{}
	for name, word := range words {
		if word == "" {
			t.Errorf("%s is the empty string, which reads as no provenance at all",
				name)
		}
		if also, clash := seen[word]; clash {
			t.Errorf("%s and %s are both %q", name, also, word)
		}
		seen[word] = name
	}
}

// AND A PROPOSAL CORRECTED TO THE EMPTY STRING IS STILL A CORRECTION.
//
// The case the derivation could get wrong by testing length instead of presence: somebody clearing
// a prefilled box has made a decision, and an empty value they supplied on purpose is a value —
// `routes.CheckArgs`'s own rule, reaching one layer further. It is theirs, not the model's.
func TestClearingAProposalIsACorrectionAndNotAnAbsence(t *testing.T) {
	d, rt := aRoutineWithAValue(t)
	at := digestNow(t, d, rt)
	i := proposing(t, d, rt, map[string]string{"message": ""},
		map[string]string{"message": "hello there"}, at)

	if i.Why != "" {
		t.Fatalf("clearing a proposal was refused: %s", i.Why)
	}
	if got := i.From["message"]; got != routes.FromEdited {
		t.Errorf("a cleared proposal is reported as %q", got)
	}
	if !i.Runnable() {
		t.Error("a play told to type nothing cannot be run")
	}
	// AND IT IS SAID AS WHAT IT IS, with the correction named.
	if len(i.Says) != 1 || !strings.Contains(i.Says[0], "nothing") ||
		!strings.Contains(i.Says[0], "as you corrected it") {
		t.Errorf("the confirmation reads %v", i.Says)
	}
}

// THE CONFIRMATION SAYS WHERE EACH VALUE CAME FROM, AND A TYPED ONE GETS NO CLAUSE.
//
// A person reading their own text back is not owed a line telling them they typed it, in the one
// place that has to be easy to check. The two that DO get a clause are the two they could mistake
// for their own.
func TestTheConfirmationSaysWhereAValueCameFrom(t *testing.T) {
	d, rt := aRoutineWithAValue(t)
	at := digestNow(t, d, rt)

	extracted := proposing(t, d, rt, map[string]string{"message": "hello there"},
		map[string]string{"message": "hello there"}, at)
	if len(extracted.Says) != 1 ||
		!strings.Contains(extracted.Says[0], "read out of what you said") {
		t.Errorf("an extracted value is confirmed as %v", extracted.Says)
	}
	if !strings.Contains(extracted.Says[0], `"hello there"`) {
		t.Errorf("the confirmation does not show the text: %v", extracted.Says)
	}

	edited := proposing(t, d, rt, map[string]string{"message": "goodbye"},
		map[string]string{"message": "hello there"}, at)
	if len(edited.Says) != 1 || !strings.Contains(edited.Says[0], "as you corrected it") {
		t.Errorf("a corrected value is confirmed as %v", edited.Says)
	}
	// AND THE PROPOSAL IT REPLACED IS NOT IN IT. What somebody is confirming is what will
	// be typed, and the words they rejected are not part of that.
	if strings.Contains(edited.Says[0], "hello there") {
		t.Errorf("the confirmation still shows the proposal: %v", edited.Says)
	}

	typed := proposing(t, d, rt, map[string]string{"message": "hello there"}, nil, "")
	if len(typed.Says) != 1 || strings.Contains(typed.Says[0], "(") {
		t.Errorf("a typed value is confirmed as %v, and should carry no clause",
			typed.Says)
	}
}

// A PROPOSAL CANNOT OUTLIVE THE REVISION IT WAS MADE FOR.
func TestAProposalForAnOlderRevisionIsRefused(t *testing.T) {
	d, rt := aRoutineWithAValue(t)
	was := digestNow(t, d, rt)

	// SAVED AGAIN, with the SAME argument name. The discriminating case: a check on the
	// names would pass this, and the same name in a rewritten routine may be typed
	// somewhere else entirely.
	src := "// Auto-generated by `marco learn \"write a note with message\"`.\n" +
		"use os.\n\nthe App is a script.\n\n" +
		"do OS's Key with \"ctrl+n\"...\n" +
		"    when ok?\n        do OS's Type with \"{{message}}\".\n" +
		"    or?\n        log that's error.\n"
	if err := d.Reg.Save(rt, src); err != nil {
		t.Fatal(err)
	}
	if now := digestNow(t, d, rt); now == was {
		t.Fatal("the play was not actually changed, so this proves nothing")
	}

	i := proposing(t, d, rt, map[string]string{"message": "hello there"},
		map[string]string{"message": "hello there"}, was)
	if i.Runnable() {
		t.Fatal("a proposal read against the old revision may still be run")
	}
	if !strings.Contains(i.Why, "saved again") {
		t.Errorf("the refusal reads %q", i.Why)
	}
	// AND IT NAMES THE ROUTINE AND NOT THE VALUE. A refusal is read in terminals and logs.
	if strings.Contains(i.Why, "hello there") {
		t.Errorf("the refusal repeats the value: %q", i.Why)
	}
	if !strings.Contains(i.Why, "write a note") {
		t.Errorf("the refusal does not name the play: %q", i.Why)
	}
	// AND THE SAME VALUES AGAINST THE REVISION AS IT NOW STANDS ARE FINE, so what was
	// refused was the revision and not anything about the value.
	again := proposing(t, d, rt, map[string]string{"message": "hello there"},
		map[string]string{"message": "hello there"}, digestNow(t, d, rt))
	if !again.Runnable() {
		t.Errorf("re-asked against the current revision it still refuses: %s", again.Why)
	}
}

// AND THE REVISION IS ASKED ABOUT BEFORE THE ARGUMENTS ARE.
//
// A proposal for an old revision is reported as a routine that was saved again, not as an argument
// nobody declared — the names may well have changed, and that refusal would send somebody looking
// for a typo in a word that was correct when they said it.
func TestAnOldRevisionIsSaidBeforeAnUnknownArgumentIs(t *testing.T) {
	d, rt := aRoutineWithAValue(t)
	was := digestNow(t, d, rt)

	// REWRITTEN TO TAKE A DIFFERENT NAME, so the old proposal's name is now unknown.
	src := "// Auto-generated by `marco learn \"write a note with body\"`.\n" +
		"use os.\n\nthe App is a script.\n\n" +
		"do OS's Type with \"{{body}}\"...\n" +
		"    when ok?\n        log \"done\".\n    or?\n        log that's error.\n"
	if err := d.Reg.Save(rt, src); err != nil {
		t.Fatal(err)
	}

	i := proposing(t, d, rt, map[string]string{"message": "hello there"},
		map[string]string{"message": "hello there"}, was)
	if i.Runnable() {
		t.Fatal("a proposal naming an argument this revision does not take may be run")
	}
	if !strings.Contains(i.Why, "saved again") {
		t.Errorf("the refusal reads %q, and should be about the revision", i.Why)
	}
}

// AND THE PROVENANCE IS SEALED WITH EVERYTHING ELSE.
//
// A label changed after the confirmation was read is the same class of thing as a value changed
// after it: what the struct says it showed is not what it now holds. It matters because the label
// is the only thing telling somebody whether a model or they themselves produced the text.
func TestTheProvenanceIsSealedLikeTheValues(t *testing.T) {
	d, rt := aRoutineWithAValue(t)
	at := digestNow(t, d, rt)
	i := proposing(t, d, rt, map[string]string{"message": "goodbye"},
		map[string]string{"message": "hello there"}, at)
	if why := d.ConfirmInvocation(i); why != "" {
		t.Fatalf("a freshly prepared invocation refuses: %s", why)
	}

	// RELABELLED: the person's own correction, credited to the model.
	tampered := i
	tampered.From = map[string]string{"message": routes.FromExtracted}
	if why := d.ConfirmInvocation(tampered); why == "" {
		t.Error("a relabelled invocation was confirmed")
	}
	// AND REMOVED ALTOGETHER.
	stripped := i
	stripped.From = nil
	if why := d.ConfirmInvocation(stripped); why == "" {
		t.Error("an invocation with its provenance stripped was confirmed")
	}
}

// A SECRET IS GIVEN NO PROVENANCE, BECAUSE IT IS GIVEN NO VALUE.
//
// Nothing about a secret is proposed, carried, sealed or shown: it comes from the credential store
// at run time, and the confirmation says so rather than naming where a value came from.
func TestASecretCarriesNoValueAndNoProvenance(t *testing.T) {
	d := Deps{Reg: routes.Registry{Dir: t.TempDir()}, Out: &strings.Builder{}}
	rt := routes.Route{App: "notepad", Slug: "sign-in"}
	src := "// Auto-generated by `marco learn \"sign in\"`.\n" +
		"use os.\n\nthe App is a script.\n\n" +
		"do OS's Secret with \"sign-in:token\"...\n" +
		"    when ok?\n        log \"done\".\n    or?\n        log that's error.\n"
	if err := d.Reg.Save(rt, src); err != nil {
		t.Fatal(err)
	}
	at := digestNow(t, d, rt)

	// A PROPOSAL FOR THE SECRET, ANSWERED ANYWAY. `resolveroute` never asks about one and
	// drops a span that named one; this is the layer that would carry it if either failed.
	i := proposing(t, d, rt, map[string]string{"token": "hunter2"},
		map[string]string{"token": "hunter2"}, at)

	if _, held := i.Values["token"]; held {
		t.Error("a secret's value was carried on the invocation")
	}
	if _, labelled := i.From["token"]; labelled {
		t.Error("a secret was given a provenance")
	}
	for _, line := range i.Says {
		if strings.Contains(line, "hunter2") {
			t.Errorf("the confirmation shows a secret: %q", line)
		}
		if strings.Contains(line, "read out of what you said") {
			t.Errorf("a secret is confirmed as extracted: %q", line)
		}
	}
	if i.Token == "" {
		t.Fatal("the invocation was not sealed")
	}
}

// AND AN INVOCATION WITH NOTHING PROPOSED IS WHAT IT ALWAYS WAS.
//
// `PrepareInvocation` is this function with no proposals and no revision, so every value is typed
// and nothing about the confirmation changes. The surfaces built in
// [[ADR-276-a-routine-may-be-told-what-to-type]] go on reading exactly what they read.
func TestWithNothingProposedEveryValueIsTyped(t *testing.T) {
	d, rt := aRoutineWithAValue(t)
	i, err := d.PrepareInvocation(rt, map[string]string{"message": "hello there"})
	if err != nil {
		t.Fatal(err)
	}
	if got := i.From["message"]; got != routes.FromTyped {
		t.Errorf("a typed value is reported as %q", got)
	}
	if why := d.ConfirmInvocation(i); why != "" {
		t.Errorf("it no longer confirms: %s", why)
	}
	if len(i.Says) != 1 || i.Says[0] != `message — "hello there"` {
		t.Errorf("the confirmation reads %v", i.Says)
	}
}
