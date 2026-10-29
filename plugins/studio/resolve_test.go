package main

// A DIRECTOR THAT COULD NOT ANSWER, SAID AS WHICH ONE IT WAS.
//
// Three states a person acts on differently: it is not running, it is running and speaks a
// protocol this build does not, or it answered and refused. Rendering all three as the first
// sends somebody to start a service that is already running — and rendering the first as the
// third sends them looking for a reason that does not exist.
//
// Held here rather than only in the page harness, which shows these sentences and does not decide
// them.

import (
	"errors"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// AND A MISSING DIRECTOR IS TOLD FROM AN UNREACHABLE ONE.
//
// What distinguishes them is whether an endpoint was ever published, which is a fact on disk
// rather than a guess from the error: a dial that failed says nothing about whether anything is
// listening.
func TestAMissingDirectorIsToldFromAnUnreachableOne(t *testing.T) {
	empty := t.TempDir()
	said := whyNoDirector(empty, errors.New("dial tcp: connection refused"))
	if !strings.Contains(said, "not running") {
		t.Errorf("with no endpoint published the answer is %q", said)
	}
	if strings.Contains(said, "could not reach") {
		t.Errorf("a service that was never there is reported as unreachable: %q", said)
	}
	// AND IT SAYS WHAT STILL WORKS, because that is the part somebody needs: the cards
	// and Play never asked Director anything.
	if !strings.Contains(said, "cards and Play still work") {
		t.Errorf("the answer does not say what is unaffected: %q", said)
	}

	// AND WITH AN ENDPOINT PUBLISHED, the honest answer is that it is there and this
	// window could not reach it — which is a different thing to go and look at.
	withOne := t.TempDir()
	// PUBLISHED THROUGH THE SERVICE'S OWN WRITER, so this test cannot pass against a
	// file shape the reader would not accept.
	if err := service.WriteEndpoint(withOne, service.Endpoint{ProtocolVersion: service.ProtocolVersion,
		Address: "127.0.0.1:1", Token: "x",
		PID: 1}); err != nil {
		t.Fatal(err)
	}
	if _, ok := service.ReadEndpoint(withOne); !ok {
		t.Fatal("the endpoint this test published is not one the reader accepts")
	}
	said = whyNoDirector(withOne, errors.New("dial tcp: connection refused"))
	if strings.Contains(said, "not running") {
		t.Errorf("a published endpoint was reported as nothing running: %q", said)
	}
	if !strings.Contains(said, "could not reach") {
		t.Errorf("the answer does not say what happened: %q", said)
	}
}

// AND A PROTOCOL MISMATCH IS NOT AN ORDINARY REFUSAL.
//
// It IS running and it DID answer; what it cannot do is speak to this build. Somebody acts on
// that by looking at the versions, not at the request.
func TestAProtocolMismatchIsSaidAsOne(t *testing.T) {
	said := whyRefused(&service.RequestError{Code: "protocol_version",
		Message: "this client speaks 11 and the service speaks 10"})
	if !strings.Contains(said, "different protocol") {
		t.Errorf("a protocol refusal is reported as %q", said)
	}
	if strings.Contains(said, "not running") {
		t.Errorf("a running Director was reported as not running: %q", said)
	}
	// AND THE SERVICE'S OWN WORDS ARE CARRIED, because the versions are in them.
	if !strings.Contains(said, "speaks 11") {
		t.Errorf("the answer drops what the service said: %q", said)
	}

	// AND AN ORDINARY REFUSAL IS AN ORDINARY REFUSAL, with its code.
	ordinary := whyRefused(&service.RequestError{Code: "malformed",
		Message: "that is not an object"})
	if strings.Contains(ordinary, "different protocol") {
		t.Errorf("an ordinary refusal is reported as a protocol mismatch: %q", ordinary)
	}
	if !strings.Contains(ordinary, "malformed") {
		t.Errorf("the refusal does not carry its code: %q", ordinary)
	}

	// AND A TRANSPORT FAILURE IS NEITHER: nothing answered, so nothing refused.
	broke := whyRefused(errors.New("read tcp: connection reset by peer"))
	for _, never := range []string{"different protocol", "refused this request"} {
		if strings.Contains(broke, never) {
			t.Errorf("a transport failure claims %q: %s", never, broke)
		}
	}
	if !strings.Contains(broke, "failed") {
		t.Errorf("a transport failure does not say so: %q", broke)
	}
}

// AND AN EMPTY REQUEST IS NOT A QUESTION, so nothing is asked of anybody.
func TestAnEmptyRequestAsksDirectorNothing(t *testing.T) {
	b := &Backend{home: t.TempDir()}
	said := b.SuggestRoutine("   ", "arkascended")
	if said.Why == "" {
		t.Errorf("an empty request produced %+v", said)
	}
	if said.Answer.Kind != "" || said.Answer.ID != "" {
		t.Errorf("an empty request produced an answer: %+v", said.Answer)
	}
	// AND THE WORDS ARE ECHOED, so a slow answer arriving after somebody has typed on
	// cannot be shown against the wrong request.
	if said.Asked != "" {
		t.Errorf("the echoed request is %q", said.Asked)
	}
}
