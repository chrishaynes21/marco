package main

// SAY WHAT YOU WANT, AND DIRECTOR SAYS WHICH ROUTINE IT WAS.
//
// # Why this is a client call and not a decision
//
// Because it was a decision, here, and it should not have been. This window grew the whole of
// scope eligibility, candidate labelling and the resolver conversation in its own module — where
// the widget could not reach any of it, so wiring the widget would have meant a second scope
// policy and a second place deciding which routine a sentence named. Two answers to one question
// is the defect this repository keeps finding.
//
// So Director owns it. `internal/resolveroute` is the policy and Director gives it the three
// facts no surface can supply honestly about itself:
//
//	the library       Director's, and NAMED in the answer, so this window is told when it
//	                  asked about a different one rather than quietly served from elsewhere
//	the context       which application the request is read against, and HOW that was known —
//	                  a window has the foreground whenever somebody types into it, so what
//	                  this window states is an OVERRIDE and never an observation
//	the candidates    every saved routine, with whether it would run as it stands
//
// What is left here is the interaction: a request box, a target to state, a sentence, and a card
// that becomes selected. Selecting is all it does. The thing that performs a routine is the Play
// button that was already there, which hands the whole identity to the one intake.
//
// # And a Director that is not there is said accurately
//
// Three different things a person can act on differently: it is not running, it is running and
// speaks a protocol this build does not, or it answered and refused. Rendering all three as the
// first would send somebody to start a service that is already running. There is NO local
// fallback: exact matching is not a smaller question this window may answer for itself, it is
// part of the resolution Director owns.
//
// Cards and Play keep working with Director down, because neither ever asked it anything.
//
// See [[ADR-267-director-owns-route-resolution]].

import (
	"errors"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/resolveroute"
)

// Suggestion is what the page shows for one request.
type Suggestion struct {
	// Answer is Director's, forwarded whole. Reshaping it here would be a second opinion
	// about which routine a request named.
	Answer resolveroute.Answer `json:"answer"`
	// Why is why there is no answer at all, when there is not: a Director that could not
	// be reached, one that speaks a different protocol, or a request it refused. Empty
	// when `Answer` is the answer.
	Why string `json:"why,omitempty"`
	// Asked is the request these words were, echoed so a slow answer arriving after
	// somebody has typed on cannot be shown against the wrong words.
	Asked string `json:"asked"`
}

// SuggestRoutine asks Director which saved routine this request named.
//
// IT SELECTS AND DOES NOT RUN. `target` is the application the person chose in this window, and
// it is sent as the override it is — see resolveroute.Basis.
func (b *Backend) SuggestRoutine(request, target string) Suggestion {
	request = strings.TrimSpace(request)
	said := Suggestion{Asked: request}
	if request == "" {
		said.Why = "say what you want, in your own words"
		return said
	}
	c, err := b.dialOwn()
	if err != nil {
		said.Why = whyNoDirector(b.home, err)
		return said
	}
	defer c.Close()

	// THE LIBRARY THIS WINDOW BELIEVES IT IS TALKING ABOUT, so Director can say when it
	// is reading a different one. Both answer it through `marcopaths`, so they agree
	// unless the two processes were given different `$MARCO_ROUTES`.
	out, err := c.ResolveRoute(service.ResolveRoutePayload{
		Request: request, Target: strings.TrimSpace(target),
		Library: marcopaths.Routes(),
	})
	if err != nil {
		said.Why = whyRefused(err)
		return said
	}
	said.Answer = out.Answer
	return said
}

// whyNoDirector tells the three states apart.
//
// Not one sentence for all of them. "It is not running" sends somebody to start it; "it is
// running and this build cannot speak to it" sends them to the version they are running; "it was
// reached and refused" sends them to the reason. Rendering the second or third as the first sends
// them to start a service that is already there.
func whyNoDirector(home string, err error) string {
	if _, ok := service.ReadEndpoint(home); !ok {
		return "the Director is not running, so nothing can resolve this request — " +
			"cards and Run still work"
	}
	return "the Director is running and this window could not reach it, so nothing can " +
		"resolve this request: " + err.Error()
}

// whyRefused is a request the Director answered and would not satisfy.
func whyRefused(err error) string {
	var refused *service.RequestError
	if errors.As(err, &refused) {
		if refused.Incompatible() {
			return "the Director is running and speaks a different protocol from this " +
				"window, so nothing can resolve this request: " + refused.Message
		}
		return "the Director refused this request: " + refused.Error()
	}
	return "asking the Director which play this is failed: " + err.Error()
}
