package main

// PRESSING PLAY PERFORMS THE ACT, AND WHAT IT HANDS OVER IS A DECISION.
//
// The surface half of the authorisation contract. The runner's half — what a grant licenses,
// what it refuses, and that a granted run asks nobody anything — is held in
// cmd/director/actgrant_test.go; this is about what the window actually does when somebody
// presses the button:
//
//	it refuses, in front of them, anything it can answer before raising a window
//	it writes the authorisation, naming the Act and the interactions they were shown
//	and it hands the runner that file, with --live only when they asked for it
//
// Driven through the stand-in runner in TestMain, which echoes the arguments it was given — so
// what is asserted is what the child was really asked to do.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
)

// aPlayableAct is a store with one demonstration and an Act over its verifiable interaction.
//
// Interaction 1 is the one with an outcome recorded, which is the one a person would pick: the
// other two cannot be checked afterwards and the surface says so.
func aPlayableAct(t *testing.T, name string) *Acts {
	t.Helper()
	a, _ := aStoreWithADemonstration(t)
	if r := a.Compose("ark graphics live", name,
		[]string{"1=the audio tab"}); r.Error != "" {
		t.Fatal(r.Error)
	}
	// THE DESKTOP'S ANSWER, STOOD IN FOR. Raising a window needs a window; what is
	// under test here is everything else. The production path is held below, by
	// TestAnApplicationThatIsNotRunningIsSaidSoBeforeAnythingStarts.
	a.raising = func(string) string { return "" }
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")
	t.Setenv("STUDIO_STANDIN_QUICK", "1")
	return a
}

// grantedIn is the authorisation file the child was handed, read back.
func grantedIn(t *testing.T, said string) grant {
	t.Helper()
	fields := strings.Fields(said)
	for i, f := range fields {
		if f != "--granted" || i+1 >= len(fields) {
			continue
		}
		raw, err := os.ReadFile(fields[i+1])
		if err != nil {
			t.Fatalf("the authorisation the child was given cannot be read: %v", err)
		}
		var g grant
		if err := json.Unmarshal(raw, &g); err != nil {
			t.Fatalf("the authorisation is not readable JSON: %v", err)
		}
		return g
	}
	t.Fatalf("the run was started with no authorisation at all:\n%s", said)
	return grant{}
}

// PLAY HANDS THE RUNNER AN AUTHORISATION FOR THE ACT THAT WAS CHOSEN.
//
// Every field in it is a term the runner checks before it performs anything, so every field is
// asserted here: a surface that wrote the wrong revision, or the interactions in the wrong
// order, would have its runs refused for a reason that is its own fault and reads like the Act's.
func TestPlayHandsTheRunnerAnAuthorisationForTheChosenAct(t *testing.T) {
	a := aPlayableAct(t, "the audio page")

	if err := a.Play("the audio page", false); err != "" {
		t.Fatalf("Play would not start: %s", err)
	}
	said := waitForTranscript(t, a)
	for _, always := range []string{"--play the audio page", "--hotkeys", "--granted"} {
		if !strings.Contains(said, always) {
			t.Errorf("the run was started without %q:\n%s", always, said)
		}
	}
	g := grantedIn(t, said)
	if g.Version != actGrantVersion {
		t.Errorf("the authorisation is version %d", g.Version)
	}
	if g.Act != "the audio page" {
		t.Errorf("the authorisation is for %q", g.Act)
	}
	if g.Application != "ArkAscended.exe" {
		t.Errorf("the authorisation names the application %q", g.Application)
	}
	if g.Revision == "" {
		t.Error("the authorisation names no revision, so an Act composed again " +
			"between the press and the run would be performed anyway")
	}
	if len(g.Interactions) != 1 || g.Interactions[0] != "the audio tab" {
		t.Errorf("the authorisation is over %v", g.Interactions)
	}
	if !strings.Contains(g.By, "pressed Play") {
		t.Errorf("the authorisation does not say who gave it: %q", g.By)
	}
	if g.At.IsZero() {
		t.Error("the authorisation does not say when it was given")
	}
}

// AND --live ONLY WHEN THEY PRESSED THE BUTTON THAT SAYS SO.
//
// The most dangerous thing this surface can get wrong, asserted again for the new gesture:
// `Play` touches the machine and `Try it without clicking` does not, and both go through one
// path that takes the decision as an argument.
func TestTryingItWithoutClickingDoesNotPassLive(t *testing.T) {
	for _, live := range []bool{false, true} {
		a := aPlayableAct(t, "the audio page")
		if err := a.Play("the audio page", live); err != "" {
			t.Fatalf("live=%v: Play would not start: %s", live, err)
		}
		said := waitForTranscript(t, a)
		if got := strings.Contains(said, "--live"); got != live {
			t.Errorf("live=%v: --live present=%v\n%s", live, got, said)
		}
		// AND READING IS ALWAYS ON. An Act's interactions are verified against what
		// the demonstration saw, and a run that cannot read the screen cannot
		// verify one — which is the difference between an Act and a macro.
		if !strings.Contains(said, "--reading") {
			t.Errorf("live=%v: the run cannot read the screen, so nothing it "+
				"performs can be verified:\n%s", live, said)
		}
		_ = a.Cancel()
	}
}

// AND WHAT PLAY CAN ANSWER NOW, IT ANSWERS NOW.
//
// Before a window is raised and before a child exists. Each of these is a sentence a person can
// act on, and each used to be either a refusal halfway through a run or nothing at all.
func TestPlayRefusesWhatItCanAnswerBeforeStartingAnything(t *testing.T) {
	a := aPlayableAct(t, "the audio page")

	// ── AN ACT THAT IS NOT THERE.
	if err := a.Play("something nobody composed", false); !strings.Contains(err,
		"no Act called") {
		t.Errorf("playing an Act that does not exist said %q", err)
	}
	// ── AND NOTHING WAS STARTED BY EITHER OF THEM.
	if w := a.Watch(); w.Active {
		t.Error("a refused Play started a run anyway")
	}
}

// AND AN ACT WHOSE DEMONSTRATION HAS MOVED UNDER IT IS REFUSED IN ITS OWN WORDS.
//
// The preflight has already asked whether the play still matches its bindings and whether the
// demonstration is still the revision it was pinned to. A person needs that sentence: "this Act
// cannot be performed as it stands" plus the reason is actionable, and a run that started and
// then refused is not.
func TestAnActWhoseDemonstrationMovedIsRefusedWithTheReason(t *testing.T) {
	a, art := aStoreWithADemonstration(t)
	if r := a.Compose("ark graphics live", "moved",
		[]string{"1=the audio tab"}); r.Error != "" {
		t.Fatal(r.Error)
	}
	a.raising = func(string) string { return "" }
	// THE DEMONSTRATION, SAVED AGAIN WITH A CHANGE. Its revision moves, and the Act is
	// pinned to the one it was composed from.
	art.Steps[1].Outcome = append(art.Steps[1].Outcome, "balance")
	store, err := demo.Open(a.home)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSequence(art); err != nil {
		t.Fatal(err)
	}

	why := a.Play("moved", false)
	if why == "" {
		t.Fatal("an Act pinned to a revision that no longer exists was performed")
	}
	if !strings.Contains(why, "cannot be performed as it stands") {
		t.Errorf("the refusal does not say what is wrong: %q", why)
	}
	if w := a.Watch(); w.Active {
		t.Error("a refused Play started a run anyway")
	}
}

// AND AN APPLICATION THAT IS NOT RUNNING IS SAID SO, BY THE PRODUCTION PATH.
//
// No stand-in: this is `bringForward` itself, against a real desktop that is certainly not
// running this. The sentence matters as much as the refusal — "start it, bring it up, and press
// Play again" is the whole of what a person has to do, and the alternative is a run that pauses
// for its whole budget about a window that was never open.
func TestAnApplicationThatIsNotRunningIsSaidSoBeforeAnythingStarts(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)

	why := a.bringForward("a-program-nobody-has-dot-exe")
	if why == "" {
		t.Fatal("an application that is not running was treated as ready")
	}
	for _, says := range []string{"does not seem to be running", "press Play again"} {
		if !strings.Contains(why, says) {
			t.Errorf("the sentence does not say %q: %s", says, why)
		}
	}
	// AND AN ACT PINNED TO NOTHING IS NOT REFUSED FOR IT. There is no window to
	// raise and nothing to check against; the run grounds against what leads the
	// desktop, as it always did.
	if why := a.bringForward(""); why != "" {
		t.Errorf("an Act with no application recorded was refused: %s", why)
	}
}

// ── AND HOW IT WENT, IN WORDS ────────────────────────────────────────────────

// EVERY OUTCOME IS SAID IN WORDS, AND THE TWO THAT MATTER MOST SAY WHETHER ANYTHING WAS PRESSED.
//
// All eight cases, because the point of this function is that no outcome falls through it. The
// pressed/not-pressed half is the one a person needs first: something on their machine either
// moved or it did not, and `refused` and `contradicted` are the two words that hide it.
func TestEveryOutcomeIsSaidInWords(t *testing.T) {
	for _, c := range []struct {
		outcome runref.Outcome
		says    string
	}{
		{runref.Verified, "It worked"},
		{runref.Contradicted, "did NOT change"},
		{runref.Unconfirmed, "could not tell whether it worked"},
		{runref.Refused, "nothing was pressed"},
		{runref.Cancelled, "You stopped it"},
		{runref.Failed, "could not finish"},
		{runref.Started, "never said how it ended"},
		{runref.Outcome("something new"), "something new"},
	} {
		got := saidOf("the audio tab", c.outcome, "")
		if !strings.Contains(got, c.says) {
			t.Errorf("%q is said as %q, which does not say %q", c.outcome, got,
				c.says)
		}
		if !strings.Contains(got, "the audio tab") &&
			c.outcome != runref.Cancelled {
			t.Errorf("%q is said without naming the Act: %q", c.outcome, got)
		}
	}
	// ── AND A RUN WITH NO OUTCOME YET SAYS NOTHING, rather than inventing a word
	//    for a run that is still going.
	if got := saidOf("the audio tab", "", ""); got != "" {
		t.Errorf("a run with no outcome is described as %q", got)
	}
	// ── AND THE RUN'S OWN REASON IS CARRIED, not paraphrased.
	got := saidOf("the audio tab", runref.Refused,
		"the look taken at the instant of the press did not agree")
	if !strings.Contains(got, "did not agree") {
		t.Errorf("the run's own reason was dropped: %q", got)
	}
}

// AND THE RUN'S OWN STATE CARRIES THE SENTENCE, not just the word.
//
// Through `Watch`, where the page gets it. The outcome word is kept beside it: this is a sentence
// ADDED for a person, not a replacement for the vocabulary the record uses.
func TestWhatTheWindowShowsSaysWhatHappened(t *testing.T) {
	ended := aStream(t, runstate.Event{Phase: runstate.Ended, Final: true,
		Act: "the audio page", Outcome: runref.Unconfirmed,
		Reason: "no words were read on the panel, so there was no evidence either way"})

	w := ended.Watch()
	if !strings.Contains(w.Said, "could not tell whether it worked") {
		t.Errorf("the window shows %q, which does not say what happened", w.Said)
	}
	if !strings.Contains(w.Said, "an act") {
		t.Errorf("the sentence does not name the Act: %q", w.Said)
	}
	// AND THE RUN'S OWN REASON IS IN IT, from the thing that decided.
	if !strings.Contains(w.Said, "no evidence either way") {
		t.Errorf("the sentence drops the run's own reason: %q", w.Said)
	}
	// AND THE WORD IS STILL THERE.
	if w.Outcome != "unconfirmed" {
		t.Errorf("the outcome word is %q", w.Outcome)
	}
}

// ── AND THE PAGE OFFERS WHAT THIS BUILT ─────────────────────────────────────

// THE PAGE'S OWN HALF OF THE FLOW.
//
// Source guards, because the rendering is JavaScript and nothing here can run it. Each one is a
// thing that would pass every test above while leaving the person with the surface they were
// handed before: a Run button that asks by key, an outcome word with no sentence, a paused run
// that looks like a failure, and `control_2` as the name of a thing on their screen.
func TestThePageIsTheFlowThatWasAskedFor(t *testing.T) {
	raw, err := os.ReadFile("frontend/index.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	for _, want := range []struct{ text, why string }{
		{"acts.Play(act, live)", "the page does not perform an Act by pressing " +
			"Play, so the authorisation contract is not reachable from it"},
		{">Stop<", "the stop control is not called Stop"},
		{"Try it without clicking", "there is no way to perform an Act without " +
			"touching the machine"},
		{"w.said", "the page shows the outcome word and not what it means"},
		{"'paused'", "the page does not show a paused run, so a run waiting for " +
			"the screen it expects looks like a run doing nothing"},
		{"s.reads", "the page labels an interaction by its machine name rather " +
			"than by the words that are on the person's own screen"},
		{"s.outcome_words", "the page does not say whether an interaction can be " +
			"checked afterwards, which is the most surprising thing about one"},
	} {
		if !strings.Contains(page, want.text) {
			t.Errorf("%s (looked for %q)", want.why, want.text)
		}
	}
}
