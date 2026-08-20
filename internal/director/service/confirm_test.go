package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The transport, and the three ways it can fail to get an answer.
//
//	nil confirmer → unavailable, confirmer error → unavailable,
//	cancelled context → cancelled.
//
// None of them is a yes. These prove that, and that nothing in the wiring can turn a
// missing answer into agreement.

func deleteRequest() execute.ConfirmationRequest {
	return execute.ConfirmationRequest{
		Scope: execute.ScopeAction, Request: "delete this file",
		Action: "delete", Target: "Report.txt", Resource: `C:\tmp\Report.txt`,
		Effect: "removes it", Reason: "this cannot be undone",
		Risk: directorapi.RiskHigh,
	}
}

// watched returns a broker publishing to a channel, and the stop function.
func watched(b *ConfirmationBroker) (<-chan ConfirmationPayload, func()) {
	asked := make(chan ConfirmationPayload, 4)
	stop := b.Watch("cmd-1", func(p ConfirmationPayload) { asked <- p })
	return asked, stop
}

// TestAnAcceptedConfirmationReturnsYes — the ordinary path.
func TestAnAcceptedConfirmationReturnsYes(t *testing.T) {
	b := NewConfirmationBroker()
	asked, stop := watched(b)
	defer stop()

	go func() {
		p := <-asked
		if err := b.Answer(p.ID, true); err != nil {
			t.Errorf("answering: %v", err)
		}
	}()

	ok, err := b.Confirm(context.Background(), deleteRequest())
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if !ok {
		t.Fatal("an accepted confirmation did not report agreement")
	}
	if _, pending := b.Pending(); pending {
		t.Error("the answered question is still open")
	}
}

// TestTheQuestionCarriesWhatAPersonNeedsToAnswerIt.
func TestTheQuestionCarriesWhatAPersonNeedsToAnswerIt(t *testing.T) {
	b := NewConfirmationBroker()
	asked, stop := watched(b)
	defer stop()

	go func() {
		p := <-asked
		_ = b.Answer(p.ID, false)
	}()
	if _, err := b.Confirm(context.Background(), deleteRequest()); err != nil {
		t.Fatalf("confirm: %v", err)
	}

	// Re-ask so the payload can be inspected.
	b2 := NewConfirmationBroker()
	got := make(chan ConfirmationPayload, 1)
	stop2 := b2.Watch("cmd-2", func(p ConfirmationPayload) { got <- p })
	defer stop2()
	go func() {
		p := <-got
		_ = b2.Answer(p.ID, true)
		got <- p
	}()
	if _, err := b2.Confirm(context.Background(), deleteRequest()); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	p := <-got

	if p.ID == "" {
		t.Error("the question has no id, so an answer cannot be tied to it")
	}
	if p.CommandID != "cmd-2" {
		t.Errorf("command id = %q, want the command that asked", p.CommandID)
	}
	for name, value := range map[string]string{
		"action": p.Action, "risk": p.Risk, "effect": p.Effect, "reason": p.Reason,
		"resource": p.Resource, "scope": p.Scope,
	} {
		if value == "" {
			t.Errorf("the question carries no %s", name)
		}
	}
	if p.ExpiresAt.Before(p.AskedAt) {
		t.Error("the question expires before it was asked")
	}
	// The prompt names the FILE, not the caption: "Report.txt" is ambiguous between
	// four folders and a path is not.
	if !strings.Contains(p.Question(), `C:\tmp\Report.txt`) {
		t.Errorf("the prompt does not name the file: %s", p.Question())
	}
}

// TestARefusedConfirmationReturnsNo.
func TestARefusedConfirmationReturnsNo(t *testing.T) {
	b := NewConfirmationBroker()
	asked, stop := watched(b)
	defer stop()

	go func() {
		p := <-asked
		_ = b.Answer(p.ID, false)
	}()

	ok, err := b.Confirm(context.Background(), deleteRequest())
	if err != nil {
		t.Fatalf("a refusal is a normal outcome, not an error: %v", err)
	}
	if ok {
		t.Fatal("a refusal reported agreement")
	}
}

// TestNobodyListeningIsUnavailableNotYes.
func TestNobodyListeningIsUnavailableNotYes(t *testing.T) {
	b := NewConfirmationBroker()

	ok, err := b.Confirm(context.Background(), deleteRequest())
	if ok {
		t.Fatal("a question nobody could be asked was answered yes")
	}
	if err == nil {
		t.Fatal("no error, so the execution layer would read this as a plain refusal " +
			"rather than as a Director that cannot ask")
	}
	if !strings.Contains(err.Error(), "listening") {
		t.Errorf("the error does not say why: %v", err)
	}
}

// TestATimedOutConfirmationIsUnavailableNotYes.
func TestATimedOutConfirmationIsUnavailableNotYes(t *testing.T) {
	b := NewConfirmationBroker()
	b.Timeout = 20 * time.Millisecond
	_, stop := watched(b)
	defer stop()

	start := time.Now()
	ok, err := b.Confirm(context.Background(), deleteRequest())
	if ok {
		t.Fatal("an unanswered question timed out into agreement")
	}
	if err == nil {
		t.Fatal("a timeout produced no error")
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("the timeout took %s; it is meant to bound the wait", time.Since(start))
	}
	if _, pending := b.Pending(); pending {
		t.Error("the timed-out question is still open, so the next answer would apply to it")
	}
}

// TestACancelledRequestIsCancelledNotRefused.
func TestACancelledRequestIsCancelledNotRefused(t *testing.T) {
	b := NewConfirmationBroker()
	_, stop := watched(b)
	defer stop()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	ok, err := b.Confirm(ctx, deleteRequest())
	if ok {
		t.Fatal("a cancelled request was agreed to")
	}
	if err == nil {
		t.Fatal("a cancelled request produced no error")
	}
	// The context error specifically: the execution layer maps that to "cancelled",
	// which is what happened. Reporting "you declined" when they walked away is a lie
	// about what they wanted.
	if ctx.Err() == nil {
		t.Fatal("the fixture did not cancel")
	}
}

// TestAnAnswerForADifferentQuestionIsRefused.
func TestAnAnswerForADifferentQuestionIsRefused(t *testing.T) {
	b := NewConfirmationBroker()
	b.Timeout = 200 * time.Millisecond
	asked, stop := watched(b)
	defer stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		<-asked
		if err := b.Answer("confirm-999", true); err == nil {
			t.Error("an answer written for another question was applied to this one")
		}
		if _, pending := b.Pending(); !pending {
			t.Error("the stale answer closed the open question")
		}
		_ = b.Answer("", false)
	}()

	ok, err := b.Confirm(context.Background(), deleteRequest())
	<-done
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if ok {
		t.Fatal("the question ended in agreement")
	}
}

// TestAnAnswerWithNothingPendingIsRefused.
func TestAnAnswerWithNothingPendingIsRefused(t *testing.T) {
	b := NewConfirmationBroker()
	if err := b.Answer("confirm-1", true); err == nil {
		t.Fatal("an answer was accepted with no question open, which would let a stray " +
			"yes wait around for the next destructive action")
	}
}

// TestUnwatchingAbandonsAnOpenQuestion — a question belongs to the command that asked it.
func TestUnwatchingAbandonsAnOpenQuestion(t *testing.T) {
	b := NewConfirmationBroker()
	b.Timeout = time.Second
	asked, stop := watched(b)

	go func() {
		<-asked
		stop()
	}()
	if _, err := b.Confirm(context.Background(), deleteRequest()); err == nil {
		t.Fatal("a question whose command ended was still answerable")
	}
	if _, pending := b.Pending(); pending {
		t.Error("the abandoned question is still open, so the NEXT command's answer " +
			"would arrive for the PREVIOUS command's question")
	}
}

// TestTheBrokerIsAConfirmer — the compile-time claim, as a test.
func TestTheBrokerIsAConfirmer(t *testing.T) {
	var c execute.Confirmer = NewConfirmationBroker()
	if c == nil {
		t.Fatal("the broker does not satisfy execute.Confirmer")
	}
}

// TestServerRoutesAnAnswerBackToTheWaitingCommand — the whole round trip, over the wire.
func TestServerRoutesAnAnswerBackToTheWaitingCommand(t *testing.T) {
	rt := newFakeRuntime()
	broker := rt.Confirmations()
	broker.Timeout = 3 * time.Second

	// The fake command asks for confirmation from inside Handle, which is where a real
	// one asks from: mid-request, with the client watching.
	agreed := make(chan bool, 1)
	rt.handle = func(ctx context.Context, phrase string, _ func(ProgressPayload)) execute.Outcome {
		ok, err := broker.Confirm(ctx, deleteRequest())
		if err != nil {
			return execute.Outcome{Request: phrase, Status: directorapi.ResultBlocked,
				Message: err.Error()}
		}
		agreed <- ok
		if !ok {
			return execute.Outcome{Request: phrase, Status: directorapi.ResultCancelled,
				Message: "cancelled — nothing was done"}
		}
		return execute.Outcome{Request: phrase, Status: directorapi.ResultDone, Message: "done"}
	}

	_, endpoint := serve(t, rt)

	// The answering connection. Separate, because the connection that submitted the
	// command is blocked reading its events and the command cannot finish until the
	// answer arrives.
	answerer := dial(t, endpoint)
	defer answerer.Close()

	go func() {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			status, err := answerer.Status()
			if err == nil && status.Confirmation != nil {
				_, _ = answerer.Confirm(status.Confirmation.ID, true)
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	c := dial(t, endpoint)
	defer c.Close()
	var sawQuestion bool
	out, err := c.Execute("delete this file", false, func(ev ResponseEnvelope) {
		if ev.Type == ResponseConfirmationRequired {
			sawQuestion = true
		}
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !sawQuestion {
		t.Error("the client watching the command never saw the question")
	}
	select {
	case ok := <-agreed:
		if !ok {
			t.Fatal("the command was told the user declined")
		}
	default:
		t.Fatal("the command never received an answer")
	}
	if out.State == CommandBlocked {
		t.Fatalf("the command was blocked despite an answer: %s", out.Message)
	}
}
