package service

// THE QUESTION REACHES THE RUNTIME, AND A REFUSAL COMES BACK AS ONE.
//
// The transport half of route resolution. The policy is held in `internal/resolveroute` and
// Director's side of it in `cmd/director`; what is held here is that the request gets there at
// all, that what it carries survives the round trip, and that a refusal is not read as an answer
// with nothing in it.
//
// The last of those was a real defect found while wiring the first client: the service answers a
// bad request with an `ErrorPayload` in a perfectly good envelope, so unmarshalling it into the
// response type SUCCEEDED and produced a zero answer — which reads as "nothing matched". A
// protocol the Director does not speak would have been indistinguishable from a request it
// understood and could not satisfy.

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/resolveroute"
)

// AND THE RUNTIME IS THE THING THAT ANSWERS IT.
//
// Removing the runtime call leaves every surface asking and never resolving anything, with no
// error anywhere — the failure the wiring-test rule in this repository exists for. This is the
// mutation that catches it.
func TestRemovingTheRuntimeCallResolvesNothing(t *testing.T) {
	rt := &fakeRuntime{}
	rt.resolve = ResolveRouteResponse{Answer: resolveroute.Answer{
		Kind: "run", ID: "arkascended/focus/ark-graphics", App: "arkascended",
		Slug: "ark-graphics", Focus: true, Name: "ark graphics",
		Source: "advisor", Library: "C:/x/routes",
		Context: resolveroute.Context{App: "arkascended", Basis: resolveroute.Stated},
	}}
	_, dir := serve(t, rt)
	c := dial(t, dir)

	out, err := c.ResolveRoute(ResolveRoutePayload{
		Request: "adjust ark video quality", Target: "arkascended",
		Library: "C:/x/routes",
	})
	if err != nil {
		t.Fatalf("asking: %v", err)
	}
	if rt.resolveCalls != 1 {
		t.Fatalf("the runtime was asked %d time(s), so this is not what answered",
			rt.resolveCalls)
	}
	// WHAT THE REQUEST CARRIED ARRIVED WHOLE. The phrase, the stated target and the
	// library the caller believes it is talking about: all three are facts Director
	// needs, and a transport that dropped any of them would answer a different question.
	if rt.resolveAsked.Request != "adjust ark video quality" ||
		rt.resolveAsked.Target != "arkascended" ||
		rt.resolveAsked.Library != "C:/x/routes" {
		t.Errorf("the runtime was asked %+v", rt.resolveAsked)
	}
	// AND THE ANSWER CAME BACK WHOLE, including every part of the identity an invocation
	// is built from and the basis the context was granted under.
	a := out.Answer
	if a.ID != "arkascended/focus/ark-graphics" || a.App != "arkascended" ||
		a.Slug != "ark-graphics" || !a.Focus {
		t.Errorf("the identity did not survive the round trip: %+v", a)
	}
	if a.Context.Basis != resolveroute.Stated || a.Library != "C:/x/routes" {
		t.Errorf("the context and the library did not survive: %+v", a)
	}
}

// AND A REFUSAL IS A REFUSAL AND NOT AN EMPTY ANSWER.
//
// Two halves, because the one way a client can provoke this is a payload the service cannot
// decode, and the branch that matters is what the client does with the answer.
func TestARefusedResolutionIsNotReadAsNothingMatching(t *testing.T) {
	rt := &fakeRuntime{}
	_, dir := serve(t, rt)
	c := dial(t, dir)

	// THE SERVICE REFUSES A PAYLOAD IT CANNOT READ, with a code, and does not ask the
	// runtime about it. Sent through the client's own writer with a payload that is not
	// an object, which is the only shape of this a client can produce.
	id, err := c.send(RequestResolveRoute, "not an object")
	if err != nil {
		t.Fatalf("sending: %v", err)
	}
	resp, err := c.receive(id, 5*time.Second)
	if err != nil {
		t.Fatalf("receiving: %v", err)
	}
	if resp.Type != ResponseError {
		t.Fatalf("the service answered %q to a payload it could not read", resp.Type)
	}
	if rt.resolveCalls != 0 {
		t.Errorf("the runtime was asked %d time(s) about a request nobody could read",
			rt.resolveCalls)
	}

	// AND THE CLIENT READS THAT AS A REFUSAL rather than as an answer with nothing in it.
	//
	// This is the defect: an ErrorPayload in a perfectly good envelope unmarshals into
	// the response type successfully and produces a ZERO answer, which reads as "nothing
	// matched". A protocol the Director does not speak would have been indistinguishable
	// from a request it understood and could not satisfy.
	out, err := resolveAnswerOf(resp)
	if err == nil {
		t.Fatalf("a refusal was read as an answer: %+v", out)
	}
	var refused *RequestError
	if !errors.As(err, &refused) {
		t.Fatalf("the refusal is %T (%v), want a RequestError carrying its code",
			err, err)
	}
	if refused.Code == "" || strings.TrimSpace(refused.Message) == "" {
		t.Errorf("the refusal does not say what it was: %+v", refused)
	}
	if out.Answer.Kind != "" || out.Answer.ID != "" {
		t.Errorf("a refusal carried an answer: %+v", out.Answer)
	}
}

// AND THE PROTOCOL REFUSAL IS TOLD APART FROM EVERY OTHER ONE.
//
// Three states a person acts on differently, and this is the one that must not read as "the
// Director is not running": it IS running, and the client cannot speak to it.
func TestAProtocolRefusalSaysSo(t *testing.T) {
	e := &RequestError{Code: "protocol_version", Message: "11 vs 10"}
	if !e.Incompatible() {
		t.Error("a protocol refusal does not report itself as one")
	}
	if other := (&RequestError{Code: "malformed"}); other.Incompatible() {
		t.Error("an ordinary refusal reports itself as a protocol mismatch")
	}
	if !strings.Contains(e.Error(), "protocol_version") {
		t.Errorf("the refusal does not carry its code: %q", e.Error())
	}
}
