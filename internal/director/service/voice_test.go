package service

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Voice routing and interactive control.
//
// Everything here is about the boundary between a front-end and the Director: what a
// spoken phrase becomes, and — more importantly — what it must NOT become. A front-end
// that resolved "the second one" itself, or that let a spoken "stop" reach the
// planner, would have crossed that boundary, and both are asserted against directly.

// ambiguous returns an outcome that asks a question between the given labels.
func ambiguous(question string, labels ...string) execute.Outcome {
	res := &directorapi.Resolution{Status: directorapi.ResolutionAmbiguous}
	for i, l := range labels {
		res.Candidates = append(res.Candidates, directorapi.TargetCandidate{
			ElementID: directorapi.ElementID("el_" + l),
			Role:      directorapi.RoleButton,
			Label:     l,
			Score:     0.8 - float64(i)*0.01,
		})
	}
	// Contenders is the authoritative offer list, so a hand-built ambiguous
	// Resolution has to carry it — exactly as the resolver does. A fixture that set
	// only Candidates would be testing a Resolution the resolver cannot produce.
	res.Contenders = append([]directorapi.TargetCandidate(nil), res.Candidates...)
	return execute.Outcome{
		Status:     directorapi.ResultNeedsClarification,
		Message:    question,
		Resolution: res,
	}
}

// asksOnce returns a runtime whose first phrase is ambiguous and whose second (the
// re-run with a refinement applied) succeeds.
func asksOnce(t *testing.T, question string, labels ...string) *fakeRuntime {
	t.Helper()
	rt := newFakeRuntime()
	var n int
	var mu sync.Mutex
	rt.handle = func(_ context.Context, phrase string, _ func(ProgressPayload)) execute.Outcome {
		mu.Lock()
		n++
		first := n == 1
		mu.Unlock()
		if first {
			return ambiguous(question, labels...)
		}
		return execute.Outcome{Status: directorapi.ResultDone, Message: "clicked " + phrase}
	}
	return rt
}

// ── 1. control phrases never reach the planner ────────────────────────────────

func TestControlPhraseRoutesToCancelAndNotToThePlanner(t *testing.T) {
	// The four the milestone names, plus the variants that mean the same thing.
	for _, phrase := range []string{
		"stop", "cancel", "stop that", "cancel that",
		"Stop.", "  CANCEL  ", "abort", "halt",
	} {
		route := RoutePhrase(phrase, StatusPayload{})
		if route.Kind != RouteCancel {
			t.Errorf("%q routed as %s, want %s — a spoken stop must never be planned",
				phrase, route.Kind, RouteCancel)
		}
	}
	// And the near-misses that are ordinary requests. "stop the music" is something a
	// person wants DONE, not a cancellation, and swallowing it would make a whole
	// class of phrases unsayable.
	for _, phrase := range []string{
		"stop the music", "cancel the meeting", "stop sharing", "click stop",
	} {
		route := RoutePhrase(phrase, StatusPayload{})
		if route.Kind != RouteExecute {
			t.Errorf("%q routed as %s, want %s — the control list must stay narrow",
				phrase, route.Kind, RouteExecute)
		}
	}
}

func TestControlPhraseCancelsWhileAQuestionIsPending(t *testing.T) {
	// Changing your mind mid-question is ordinary. If the pending question captured
	// "stop" as its answer, the only way out of a clarification would be to answer it.
	status := StatusPayload{Clarification: &ClarificationPayload{
		CommandID: "cmd_1", Question: "which one?",
	}}
	route := RoutePhrase("stop", status)
	if route.Kind != RouteCancel {
		t.Fatalf("routed as %s, want %s", route.Kind, RouteCancel)
	}
	if route.CommandID != "" {
		t.Errorf("a cancellation carried a command id %q — it is not an answer", route.CommandID)
	}
}

func TestSpokenStopCancelsARunningCommand(t *testing.T) {
	// The end-to-end property the long-lived service exists for: the phrase arrives on
	// a DIFFERENT connection while the first command is still running, and stops it.
	rt := newFakeRuntime()
	started := make(chan struct{})
	rt.handle = func(ctx context.Context, phrase string, _ func(ProgressPayload)) execute.Outcome {
		close(started)
		<-ctx.Done()
		return execute.Outcome{Status: directorapi.ResultCancelled, Message: "stopped"}
	}
	_, dir := serve(t, rt)

	runner := dial(t, dir)
	done := make(chan OutcomePayload, 1)
	go func() {
		out, err := runner.Execute("repeat that ten times", false, nil)
		if err != nil {
			t.Errorf("Execute: %v", err)
		}
		done <- out
	}()
	<-started

	// A second client — as a second `marco director` process would be.
	stopper := dial(t, dir)
	res, err := stopper.Submit("stop", nil)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if res.Route.Kind != RouteCancel {
		t.Fatalf("routed as %s, want %s", res.Route.Kind, RouteCancel)
	}
	if res.Cancel == nil || !res.Cancel.Accepted {
		t.Fatalf("the cancellation was not accepted: %+v", res.Cancel)
	}

	select {
	case out := <-done:
		if out.State != CommandCancelled {
			t.Errorf("state %s, want %s", out.State, CommandCancelled)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the running command was never cancelled")
	}

	// And the phrase never reached the planner as a request to click something.
	for _, p := range rt.phrases() {
		if p == "stop" {
			t.Fatal(`"stop" was planned — it must be a control phrase, not a target`)
		}
	}
}

// ── 2. the clarification exchange ─────────────────────────────────────────────

func TestAmbiguityBecomesAQuestionRatherThanAFailure(t *testing.T) {
	rt := asksOnce(t, "which Save did you mean?", "Save", "Save As", "Save All")
	_, dir := serve(t, rt)
	c := dial(t, dir)

	var seen []ResponseType
	out, err := c.Execute("click save", false, func(ev ResponseEnvelope) {
		seen = append(seen, ev.Type)
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.State != "" {
		t.Errorf("a question produced an outcome state %q — it is not a result", out.State)
	}

	// The event a front-end renders, with everything it needs to render it and
	// nothing it needs to decide with.
	ask := c.lastClarification
	if ask == nil {
		t.Fatal("no CLARIFICATION_REQUIRED was delivered")
	}
	if ask.CommandID == "" {
		t.Error("the question carried no command id — an answer could not be correlated")
	}
	if ask.Question != "which Save did you mean?" {
		t.Errorf("question %q", ask.Question)
	}
	if ask.Phrase != "click save" {
		t.Errorf("the original phrase was lost: %q", ask.Phrase)
	}
	if len(ask.Candidates) != 3 {
		t.Fatalf("%d candidates, want 3", len(ask.Candidates))
	}
	for i, cand := range ask.Candidates {
		if cand.Index != i+1 {
			t.Errorf("candidate %d has index %d — the numbering IS the answer", i, cand.Index)
		}
		if cand.Label == "" || cand.ID == "" {
			t.Errorf("candidate %d is missing a label or id: %+v", i, cand)
		}
	}
	if !containsType(seen, ResponseClarificationRequired) {
		t.Errorf("events were %v, missing %s", seen, ResponseClarificationRequired)
	}
}

func TestOnlyViableCandidatesAreOfferedAndTheListIsBounded(t *testing.T) {
	out := ambiguous("which one?", "One", "Two")
	// A disabled control and a piece of inert text: part of the explanation, never
	// part of the choice.
	out.Resolution.Candidates = append(out.Resolution.Candidates,
		directorapi.TargetCandidate{ElementID: "el_x", Label: "Three", Score: 0.7, Rejected: "disabled"},
		directorapi.TargetCandidate{ElementID: "el_y", Label: "Four", Score: 0},
	)
	for i := 0; i < 10; i++ {
		out.Resolution.Candidates = append(out.Resolution.Candidates,
			directorapi.TargetCandidate{ElementID: "el_extra", Label: "Extra", Score: 0.5})
	}

	ask, ok := clarificationFor("cmd_1", "click it", out)
	if !ok {
		t.Fatal("a viable choice was not offered")
	}
	if len(ask.Candidates) > maxClarificationCandidates {
		t.Errorf("%d candidates offered, cap is %d — a spoken choice must stay small",
			len(ask.Candidates), maxClarificationCandidates)
	}
	for _, c := range ask.Candidates {
		if c.Label == "Three" || c.Label == "Four" {
			t.Errorf("%q was offered but is not actionable", c.Label)
		}
	}

	// One viable candidate is not a choice, and asking about it would be theatre.
	lone := ambiguous("which one?", "Only")
	if _, ok := clarificationFor("cmd_2", "click it", lone); ok {
		t.Error("a question was asked with a single candidate")
	}
}

func TestOnlyTheCandidatesActuallyInContentionAreOffered(t *testing.T) {
	// Found live against VS Code: "click new" reported "3 controls match" and then
	// offered four, the fourth being an update notification that merely contained the
	// word. A user picking it would have picked something the Director never rated a
	// match — and the count they were told would have contradicted the list.
	out := ambiguous(`3 controls match "new" about equally well`,
		"New Terminal", "New Terminal", "New Text File", "Manage - New Code update available")
	// Only three were in contention; the fourth is an also-ran the resolver would not
	// have offered, so it is not in the contender list.
	out.Resolution.Contenders = out.Resolution.Candidates[:3]

	ask, ok := clarificationFor("cmd_1", "click new", out)
	if !ok {
		t.Fatal("no question was offered")
	}
	if len(ask.Candidates) != 3 {
		t.Fatalf("%d candidates offered, want the 3 in contention: %+v", len(ask.Candidates), ask.Candidates)
	}
	for _, c := range ask.Candidates {
		if strings.Contains(c.Label, "update available") {
			t.Errorf("an also-ran was offered as a choice: %q", c.Label)
		}
	}
	// The offer is a PREFIX, never re-ordered — the ordinal the user speaks indexes
	// the same list the resolver will index when it re-resolves.
	if ask.Candidates[0].Label != "New Terminal" || ask.Candidates[2].Label != "New Text File" {
		t.Errorf("the offer was re-ordered: %+v", ask.Candidates)
	}
}

func TestATrimmedOfferDoesNotContradictItsOwnQuestion(t *testing.T) {
	// More contenders than a person can hold in their head. The list is capped, and
	// the wording has to be capped with it — telling someone "8 controls match" over a
	// list of five is an invitation to say "the seventh one".
	labels := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
	out := ambiguous("8 controls match \"x\" about equally well.", labels...)
	out.Resolution.Contenders = out.Resolution.Candidates

	ask, ok := clarificationFor("cmd_1", "click x", out)
	if !ok {
		t.Fatal("no question was offered")
	}
	if len(ask.Candidates) != maxClarificationCandidates {
		t.Fatalf("%d candidates, want the cap of %d", len(ask.Candidates), maxClarificationCandidates)
	}
	if !strings.Contains(ask.Question, "closest 5") {
		t.Errorf("the question still claims a longer list than it shows: %q", ask.Question)
	}
}

func TestTheNextPhraseAnswersThePendingQuestionAsARefinement(t *testing.T) {
	rt := asksOnce(t, "which one?", "Save", "Save As")
	_, dir := serve(t, rt)
	c := dial(t, dir)

	if _, err := c.Submit("click save", nil); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if c.lastClarification == nil {
		t.Fatal("no question was asked")
	}
	asked := c.lastClarification.CommandID

	// A fresh client, because the overlay spawns a new process per phrase. The pending
	// question lives in the SERVICE, so the routing works anyway — which is the whole
	// reason it lives there.
	c2 := dial(t, dir)
	res, err := c2.Submit("the second one", nil)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if res.Route.Kind != RouteClarify {
		t.Fatalf("routed as %s, want %s", res.Route.Kind, RouteClarify)
	}
	if res.Route.CommandID != asked {
		t.Errorf("answered %s, but %s was asked", res.Route.CommandID, asked)
	}
	if res.Outcome == nil || res.Outcome.State != CommandCompleted {
		t.Fatalf("the answered request did not complete: %+v", res.Outcome)
	}

	// The answer reached the Director as a REFINEMENT of the original query, not as a
	// chosen element id. This is the architectural rule made testable: the front-end
	// submitted the words, and the Director turned them into a narrowing.
	refs := rt.refinements()
	if len(refs) != 1 {
		t.Fatalf("%d refinements, want 1", len(refs))
	}
	if refs[0].Ordinal != 2 {
		t.Errorf("ordinal %d, want 2", refs[0].Ordinal)
	}

	// And it re-ran the ORIGINAL phrase, resolved afresh — not "the second one".
	phrases := rt.phrases()
	if len(phrases) != 2 || phrases[1] != "click save" {
		t.Errorf("phrases were %v; the answer must re-run the original request", phrases)
	}
}

func TestAnUnparseableAnswerBecomesANewRequest(t *testing.T) {
	// Guessing here would act on a control the user never picked. Treating the phrase
	// as what it plainly is — a new request — is the only honest reading.
	rt := asksOnce(t, "which one?", "Save", "Save As")
	_, dir := serve(t, rt)
	c := dial(t, dir)

	if _, err := c.Submit("click save", nil); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	res, err := c.Submit("open the file menu", nil)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if res.Outcome == nil || res.Outcome.State != CommandCompleted {
		t.Fatalf("the new request did not run: %+v", res.Outcome)
	}
	if refs := rt.refinements(); len(refs) != 0 {
		t.Errorf("%d refinements applied — an unparseable answer narrowed nothing", len(refs))
	}
	phrases := rt.phrases()
	if len(phrases) != 2 || phrases[1] != "open the file menu" {
		t.Errorf("phrases were %v, want the new request to have run as itself", phrases)
	}
}

func TestAnAnswerToAQuestionThatIsNoLongerPendingIsRefused(t *testing.T) {
	rt := asksOnce(t, "which one?", "Save", "Save As")
	_, dir := serve(t, rt)
	c := dial(t, dir)

	if _, err := c.Submit("click save", nil); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	// An answer aimed at a different command: the user answered something that is no
	// longer on screen, and applying it to whatever is pending now would act on the
	// wrong thing entirely.
	_, err := c.Clarify("cmd_from_yesterday", "the first one", nil)
	if err == nil {
		t.Fatal("a stale answer was accepted")
	}
	if !strings.Contains(err.Error(), "stale_question") {
		t.Errorf("error was %v, want it to name the staleness", err)
	}
	if refs := rt.refinements(); len(refs) != 0 {
		t.Errorf("%d refinements applied from a stale answer", len(refs))
	}
}

func TestCancellingAQuestionAbandonsTheRequest(t *testing.T) {
	rt := asksOnce(t, "which one?", "Save", "Save As")
	_, dir := serve(t, rt)
	c := dial(t, dir)

	if _, err := c.Submit("click save", nil); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	// "never mind" is an answer to the question (it parses), so it goes as a CLARIFY
	// and abandons the request rather than cancelling a running command.
	res, err := c.Submit("never mind", nil)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if res.Outcome == nil || res.Outcome.State != CommandCancelled {
		t.Fatalf("state was %+v, want cancelled", res.Outcome)
	}
	if refs := rt.refinements(); len(refs) != 0 {
		t.Errorf("%d refinements — an abandoned question narrows nothing", len(refs))
	}
	if n := len(rt.phrases()); n != 1 {
		t.Errorf("%d phrases planned, want 1 — nothing should have re-run", n)
	}
}

func TestANewRequestSupersedesAnUnansweredQuestion(t *testing.T) {
	// Asked something, then asked something else. The old question must not linger and
	// capture a later phrase as its answer.
	rt := asksOnce(t, "which one?", "Save", "Save As")
	_, dir := serve(t, rt)
	c := dial(t, dir)

	if _, err := c.Submit("click save", nil); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if _, err := c.Submit("open the file menu", nil); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	st, err := c.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Clarification != nil {
		t.Errorf("a stale question is still pending: %+v", st.Clarification)
	}
	// So the NEXT phrase is a new request, not an answer.
	if route := RoutePhrase("the second one", st); route.Kind != RouteExecute {
		t.Errorf("routed as %s, want %s", route.Kind, RouteExecute)
	}
}

// ── 3. the event surface ──────────────────────────────────────────────────────

func TestAllSevenEventTypesAreDeliverable(t *testing.T) {
	// The seven a front-end must handle. Each is produced by the situation that
	// produces it, rather than asserted as a constant — a type that no path can emit
	// is not an event, it is a name.
	got := map[ResponseType]bool{}
	record := func(ev ResponseEnvelope) { got[ev.Type] = true }

	// ACKNOWLEDGED, PROGRESS, COMPLETED.
	rt := newFakeRuntime()
	rt.handle = func(_ context.Context, phrase string, progress func(ProgressPayload)) execute.Outcome {
		progress(ProgressPayload{Stage: "observe", Detail: "looking"})
		switch phrase {
		case "boom":
			return execute.Outcome{Status: directorapi.ResultFailed, Message: "no"}
		case "maybe":
			// "It happened but could not be confirmed" — partial, which the service
			// classifies as UNVERIFIED rather than as a failure.
			return execute.Outcome{Status: directorapi.ResultPartial, Message: "unsure"}
		case "ambiguous":
			return ambiguous("which one?", "A", "B")
		}
		return execute.Outcome{Status: directorapi.ResultDone, Message: "done"}
	}
	_, dir := serve(t, rt)
	c := dial(t, dir)

	for _, phrase := range []string{"click ok", "boom", "maybe", "ambiguous"} {
		if _, err := c.Execute(phrase, false, record); err != nil {
			t.Fatalf("Execute(%q): %v", phrase, err)
		}
	}

	// CANCELLED needs a command that is actually stopped mid-flight.
	rt2 := newFakeRuntime()
	started := make(chan struct{})
	rt2.handle = func(ctx context.Context, _ string, _ func(ProgressPayload)) execute.Outcome {
		close(started)
		<-ctx.Done()
		return execute.Outcome{Status: directorapi.ResultCancelled, Message: "stopped"}
	}
	_, dir2 := serve(t, rt2)
	runner := dial(t, dir2)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = runner.Execute("repeat that ten times", false, record)
	}()
	<-started
	if _, err := dial(t, dir2).Cancel(); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	<-done

	for _, want := range []ResponseType{
		ResponseAcknowledged, ResponseProgress, ResponseClarificationRequired,
		ResponseCompleted, ResponseUnverified, ResponseFailed, ResponseCancelled,
	} {
		if !got[want] {
			t.Errorf("%s was never emitted", want)
		}
	}
}

func TestClarificationIsTerminalForItsRequestButNotForTheCommand(t *testing.T) {
	// A question ends the REQUEST — the client stops reading — while the command stays
	// open awaiting an answer. If it were not terminal the client would block forever
	// on a service that is waiting for the user.
	if !ResponseClarificationRequired.Terminal() {
		t.Error("CLARIFICATION_REQUIRED is not terminal — a client would hang on it")
	}
	rt := asksOnce(t, "which one?", "Save", "Save As")
	_, dir := serve(t, rt)
	c := dial(t, dir)
	if _, err := c.Submit("click save", nil); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	st, err := c.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Clarification == nil {
		t.Fatal("the question is not visible in status — a new client could not route to it")
	}
	if st.Active != nil {
		t.Errorf("the command is still marked running while waiting for a person: %+v", st.Active)
	}
}

// ── 4. parsing an answer ──────────────────────────────────────────────────────

func TestClarificationAnswersParseIntoRefinements(t *testing.T) {
	cases := []struct {
		phrase string
		want   intent.Refinement
		ok     bool
	}{
		{"the first one", intent.Refinement{Ordinal: 1}, true},
		{"second", intent.Refinement{Ordinal: 2}, true},
		{"the third one please", intent.Refinement{Ordinal: 3}, true},
		{"number 4", intent.Refinement{Ordinal: 4}, true},
		{"2", intent.Refinement{Ordinal: 2}, true},
		{"the last one", intent.Refinement{Ordinal: -1}, true},
		{"the tab", intent.Refinement{Role: directorapi.RoleTab}, true},
		{"the second tab", intent.Refinement{Ordinal: 2, Role: directorapi.RoleTab}, true},
		{"cancel", intent.Refinement{Cancel: true}, true},
		{"never mind", intent.Refinement{Cancel: true}, true},

		// Not answers. These must NOT be forced into a choice.
		{"open the file menu", intent.Refinement{}, false},
		{"click the toolbar", intent.Refinement{}, false},
		{"", intent.Refinement{}, false},
	}
	for _, tc := range cases {
		got, ok := intent.ParseClarification(tc.phrase)
		if ok != tc.ok {
			t.Errorf("%q: understood=%v, want %v", tc.phrase, ok, tc.ok)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("%q: %+v, want %+v", tc.phrase, got, tc.want)
		}
	}
}

func TestARefinementNarrowsTheOriginalQueryRatherThanReplacingIt(t *testing.T) {
	// The point of a refinement: everything the user originally said survives, and the
	// answer only adds. Replacing the query would lose "save" from "click save".
	q := &directorapi.ElementQuery{Text: "save", Role: directorapi.RoleButton}
	intent.Refinement{Ordinal: 2}.Apply(q)
	if q.Text != "save" {
		t.Errorf("the original text was lost: %q", q.Text)
	}
	if q.Ordinal != 2 {
		t.Errorf("ordinal %d, want 2", q.Ordinal)
	}
	intent.Refinement{Role: directorapi.RoleTab}.Apply(q)
	if q.Role != directorapi.RoleTab {
		t.Errorf("role %q, want the answer to narrow it", q.Role)
	}
	// A nil query is left alone rather than panicking: a request with no element target
	// (a window move) can still be answered by a phrase that happens to parse.
	intent.Refinement{Ordinal: 1}.Apply(nil)
}

func containsType(types []ResponseType, want ResponseType) bool {
	for _, t := range types {
		if t == want {
			return true
		}
	}
	return false
}
