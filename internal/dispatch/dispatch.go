// Package dispatch is Marco's ROUTE-LEVEL decision-maker. Given a line of natural
// language it decides WHAT the user wants — run a saved route, learn a new command,
// answer a question, or ask for clarification — and hands a front-end (the
// interactive assistant, the overlay) a single, verified Decision to act on.
//
// This is deliberately a small, closed decision: which of the routes you already
// have (if any) does this phrase mean? It is NOT the Director (internal/director),
// which builds a world model of the desktop and plans arbitrary UI actions against
// it. Dispatch picks a saved macro; the Director decides what to do to the screen.
// They are separate systems and neither imports the other.
//
// Dispatch is deliberately INDEPENDENT OF ANY LANGUAGE MODEL. Its baseline is the
// deterministic, offline matcher (internal/nlu): an exact route name runs, a near
// match asks to confirm, anything else is a new command to learn. That alone makes
// Marco usable with no model at all.
//
// Smarter understanding is layered in through the Advisor interface — an optional
// intelligence source consulted for the cases plain matching can't settle. A local
// LLM plugin is one Advisor (see llm.go / plugins/llama), but so could be a rules
// engine, a remote service, or a test double. Dispatch owns the policy and always
// re-verifies the Advisor's answer (a suggested route must be one the user actually
// has), so no backend — model or otherwise — can make it run something that doesn't
// exist. The Director reuses this trust-but-verify discipline for its planner.
package dispatch

import (
	"context"
	"slices"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/nlu"
)

// Intent is dispatch's verdict for one line of input.
const (
	IntentRun     = "run"     // run an existing route (Route is a verified slug)
	IntentLearn   = "teach"   // create a new command (Name is its suggested name)
	IntentChat    = "chat"    // conversational answer (Reply)
	IntentClarify = "clarify" // ambiguous — confirm/ask a follow-up (Reply; Route may hint)
	IntentNone    = "none"    // nothing to do (only when input is empty)
)

// Decision is a validated instruction for the caller. Intent is always one of the
// constants above.
type Decision struct {
	Intent string
	Route  string // IntentRun: a slug guaranteed to be in the offered routes
	Name   string // IntentLearn: the new command's name
	Reply  string // IntentChat/IntentClarify: something to say to the user
}

// Advisor is an optional intelligence source consulted for input the
// deterministic layer can't confidently resolve. It returns a proposed Decision and
// whether it's offering one at all (false = "I have nothing, use your defaults").
// Dispatch re-validates every proposal, so an Advisor may be as loose as it
// likes. Implementations must not assume any particular model — this interface is
// the whole contract, which is what keeps dispatch model-independent.
type Advisor interface {
	Advise(ctx context.Context, input string, routes []string, app string) (Decision, bool)
}

// Dispatcher turns input into a Decision. The zero value works (deterministic only);
// set Advisor to add a smarter backend.
type Dispatcher struct {
	// Advisor is consulted between the exact-match fast path and the deterministic
	// fallback. nil = deterministic only.
	Advisor Advisor
	// ClarifyMin is the fuzzy score at/above which an unmatched phrase is treated as
	// "did you mean <route>?" rather than a brand-new command. 0 uses defaultClarify.
	ClarifyMin float64
}

const defaultClarify = 0.6

// New builds a Dispatcher with the given Advisor (which may be nil).
func New(a Advisor) Dispatcher { return Dispatcher{Advisor: a} }

// Decide classifies input given the user's route slugs and the foreground app (used
// only as context for an Advisor). It always returns a usable Decision.
func (d Dispatcher) Decide(ctx context.Context, input string, routes []string, app string) Decision {
	input = strings.TrimSpace(input)
	if input == "" {
		return Decision{Intent: IntentNone}
	}

	// 1. Deterministic, offline, free: an exact route name just runs — no model call
	//    for the common case.
	m := nlu.Resolve(input, routes)
	if m.Exact {
		return Decision{Intent: IntentRun, Route: m.Route}
	}

	// 2. Optional intelligence. Trust-but-verify: we re-check the proposal
	//    so a bad Advisor can never widen what runs.
	if dec, ok := d.Propose(ctx, input, routes, app); ok {
		return dec
	}

	// 3. Deterministic fallback: a near match asks to confirm; anything else is a new
	//    command to learn (Marco learns what it is shown).
	thr := d.ClarifyMin
	if thr == 0 {
		thr = defaultClarify
	}
	if m.Route != "" && m.Score >= thr {
		return Decision{Intent: IntentClarify, Route: m.Route,
			Reply: "Did you mean " + prettify(m.Route) + "?"}
	}
	return Decision{Intent: IntentLearn, Name: input}
}

// Propose returns ONLY the Advisor's validated suggestion — no deterministic
// fallback. ok is false when there is no Advisor, the input is empty, or the Advisor
// declined or proposed something unusable (e.g. a route the user doesn't have). Use
// Propose to layer a smart backend on top of your own default handling (the
// interactive assistant does this so an unconfigured brain leaves its flow intact);
// use Decide when you want a complete decision every time (the overlay seam).
func (d Dispatcher) Propose(ctx context.Context, input string, routes []string, app string) (Decision, bool) {
	if d.Advisor == nil || strings.TrimSpace(input) == "" {
		return Decision{}, false
	}
	proposal, ok := d.Advisor.Advise(ctx, input, routes, app)
	if !ok {
		return Decision{}, false
	}
	return validate(proposal, routes)
}

// validate turns a raw proposal into a trustworthy Decision, or reports it unusable.
// A `run` is accepted only when its route is one we actually offered; `teach` needs
// a name; `chat`/`clarify` need a reply.
func validate(dec Decision, routes []string) (Decision, bool) {
	dec.Intent = strings.ToLower(strings.TrimSpace(dec.Intent))
	dec.Route = strings.TrimSpace(dec.Route)
	dec.Name = strings.TrimSpace(dec.Name)
	dec.Reply = strings.TrimSpace(dec.Reply)
	switch dec.Intent {
	case IntentRun:
		if slices.Contains(routes, dec.Route) {
			return dec, true
		}
		return Decision{}, false
	case IntentLearn:
		if dec.Name == "" {
			return Decision{}, false
		}
		return dec, true
	case IntentChat, IntentClarify:
		if dec.Reply == "" {
			return Decision{}, false
		}
		return dec, true
	}
	return Decision{}, false
}

func prettify(slug string) string { return strings.ReplaceAll(slug, "-", " ") }
