package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/internal/resolveroute"
)

// A misheard routine name is PROPOSED from Director's resolver, and the screen is never read.
//
// Measured live: "cloud paul stone" went straight to Director's screen reading, was tried as a
// desktop task and failed, and the routine it named was never offered.
//
// Mutation: delete the proposal step in askDirector. This fails: the phrase is submitted.
func TestAMisheardNameIsProposedBeforeTheScreenIsRead(t *testing.T) {
	d := intakeWorld(t)
	noPendingQuestion(t)
	submitted := ""
	prevSubmit, prevPropose := submitPhrase, proposeRoutine
	submitPhrase = func(p string, _ bool) int { submitted = p; return exitOK }
	asked := ""
	proposeRoutine = func(p string) (string, string, bool) {
		asked = p
		return proposedRoutine(service.ResolveRouteResponse{Answer: resolveroute.Answer{
			Kind: "run", ID: "arkascended/focus/cloud-pull", App: "arkascended", Slug: "cloud-pull",
			Focus: true, Name: "cloud pull", Source: "advisor"}})
	}
	t.Cleanup(func() { submitPhrase, proposeRoutine = prevSubmit, prevPropose })

	var out Outcome
	printed, _ := captureStdout(t, func() error {
		out, _ = runInvocation(d, invoke.Request{Text: "cloud paul stone", Source: invoke.SourceSpoken})
		return nil
	})
	if asked != "cloud paul stone" {
		t.Fatalf("the resolver was not asked about the words (asked %q)", asked)
	}
	if submitted != "" {
		t.Fatal("a play was proposed and the words were STILL read against the screen")
	}
	if out != OutcomeClarify {
		t.Fatalf("a proposal ended %q; it is a question, nothing ran", out)
	}
	var line string
	for _, l := range strings.Split(printed, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimRight(l, "\r"), outcome.ProposedPrefix); ok {
			line = rest
		}
	}
	var back service.ResolveRouteResponse
	if err := json.Unmarshal([]byte(line), &back); err != nil || back.Answer.ID != "arkascended/focus/cloud-pull" {
		t.Fatalf("the proposal line does not carry the answer (%v): %q", err, printed)
	}
}

// Nothing proposed — a learn answer, a clarification, a mismatch, no Director — goes on to the
// screen as before.
func TestNoProposalLeavesTheScreenReadingAsItWas(t *testing.T) {
	for name, resp := range map[string]service.ResolveRouteResponse{
		"learn":    {Answer: resolveroute.Answer{Kind: "teach"}},
		"clarify":  {Answer: resolveroute.Answer{Kind: "clarify", ID: "a/b"}},
		"mismatch": {Answer: resolveroute.Answer{Kind: "run", ID: "a/b"}, Mismatch: "another library"},
		"no id":    {Answer: resolveroute.Answer{Kind: "run"}},
	} {
		if _, _, ok := proposedRoutine(resp); ok {
			t.Errorf("%s was proposed as a play", name)
		}
	}
	d := intakeWorld(t)
	noPendingQuestion(t)
	submitted := ""
	prevSubmit, prevPropose := submitPhrase, proposeRoutine
	submitPhrase = func(p string, _ bool) int { submitted = p; return exitOK }
	proposeRoutine = func(string) (string, string, bool) { return "", "", false }
	t.Cleanup(func() { submitPhrase, proposeRoutine = prevSubmit, prevPropose })
	if out, _ := runInvocation(d, invoke.Request{Text: "open the settings", Source: invoke.SourceTyped}); out != OutcomePerformed || submitted != "open the settings" {
		t.Fatalf("with nothing proposed the words did not reach Director (%q, %q)", out, submitted)
	}
}

// An ANSWER to a question Director is holding is not a request, and is not resolved.
func TestAnAnswerIsNotResolvedAsARoutine(t *testing.T) {
	d := intakeWorld(t)
	prevPending := pendingQuestion
	pendingQuestion = func() bool { return true }
	asked := false
	prevSubmit, prevPropose := submitPhrase, proposeRoutine
	submitPhrase = func(string, bool) int { return exitOK }
	proposeRoutine = func(string) (string, string, bool) { asked = true; return "{}", "x", true }
	t.Cleanup(func() { submitPhrase, proposeRoutine, pendingQuestion = prevSubmit, prevPropose, prevPending })
	if out, _ := runInvocation(d, invoke.Request{Text: "the second one", Source: invoke.SourceSpoken}); asked || out != OutcomePerformed {
		t.Fatalf("an answer to a pending question was resolved as a play (asked=%v, %q)", asked, out)
	}
}
