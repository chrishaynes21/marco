// Package meaning asks an optional external interpreter which observed control a request meant,
// and believes none of it.
//
// # Why this exists at all
//
// [[Experiment-069-rewording-closes-the-spelling-gap-and-not-the-meaning-gap]] measured the
// boundary of a deterministic reading precisely: wrapping words and spellings close at 11/11
// and 3/4 on applications nobody tuned against, and PARAPHRASE closes at one in thirteen.
// `themes` for Personalization, `bookmarks` for Favorites, `dark mode` for Theme, `wifi` for
// Network & internet. Every one fails for the same stated reason — rewording may only ask with
// words the screen is showing, and none of those words is on the screen.
//
// Those are knowledge questions. No amount of further lexical rules answers them, and the
// recommendation out of that experiment was not to keep trying.
//
// # Shadow mode, which is the whole of this experiment
//
// Nothing in resolution or execution asks this question. The only caller is the evaluation that
// measures it, and a tree walk holds that — cmd/director TestTheInterpreterIsNotWiredIntoAnything.
//
// That is not caution for its own sake. The question being asked is whether semantic
// interpretation identifies the right control when a person describes something naturally, AND
// recognises when it does not know enough. Both halves need measuring before anything is
// allowed to act on either, and a mechanism that is already acting cannot be measured against
// what it displaced.
//
// # What it may do, which is propose
//
// A Proposal is an element id and a sentence. It is not a resolution: it carries no confidence,
// no authority, no action and no claim that the control can be operated. Were it ever wired in,
// a proposal would enter the resolver as a candidate and face the same actionability rules, the
// same ambiguity refusal and the same policy as anything else — which is the arrangement
// [[ADR-218-a-request-reworded-in-the-screens-own-words]] already uses for rewording, and the
// reason this returns ids rather than targets.
//
// Four refusals, and three of them are structural rather than promised:
//
//  1. It cannot name a control that was not observed. The id must be in the list that was sent
//     AND still be in the world when the answer comes back.
//  2. It cannot make anything operable. Only controls that are ALREADY addressable are offered
//     to it, and what it returns is an id.
//  3. It cannot authorise. There is nowhere in a Proposal to say so.
//  4. It must be able to say it does not know. An abstention is a first-class answer, counted
//     separately in the evaluation, and an interpreter that never abstains is one whose
//     answers cannot be trusted when it does.
package meaning

import (
	"context"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/world"
	"github.com/chaynes-simpleclouds/marco/internal/resolver"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Proposal is one interpreter's reading of which observed control a request meant.
type Proposal struct {
	// ElementID is a Prop in the world that was asked about. Never anything else.
	ElementID directorapi.ElementID
	// Because is the interpreter's own sentence about why this control fits the request.
	// It is evidence for a person reading the account, not grounds for acting.
	Because string
}

// Available reports whether an interpreter is configured at all.
//
// Unset means the question is never asked and everything behaves exactly as it does without
// this package, which is what "degrades cleanly" has to mean: not a fallback path, but no path.
func Available() bool { return resolver.InterpreterConfigured() }

// HowManyControlsToOffer bounds what is sent.
//
// A screenful, not a treeful. Paint reports 106 elements and Explorer 140, of which the named
// operable ones are a third — and a list long enough to need paging is a list the question is
// too vague to be asked about. The bound is also the cost: this is the whole of what an
// interpreter is charged for.
const HowManyControlsToOffer = 80

// Candidates is the bounded projection of a Stage that an interpreter is told about.
//
// # What is left out, and why each one
//
// Only ADDRESSABLE controls: named, operable, enabled, on screen. Scenery is left out because a
// proposal naming it would be rejected anyway, and sending it would be sending more of a
// person's screen than the question needs.
//
// No values. A text field's contents are the person's own writing and have no bearing on which
// control they meant. No bounds, which an interpreter has no use for and could only use to
// point somewhere. No native ids, which are the application's own handles. No window title,
// which on a file manager is a path.
//
// What remains is an id, a role and a label — and the labels are the part that is genuinely the
// person's screen. There is no way to ask which control was meant without sending them, which
// is exactly why the question has its own switch and is off until somebody turns it on.
func Candidates(w *directorapi.WorldState) []resolver.Control {
	if w == nil {
		return nil
	}
	var out []resolver.Control
	for _, el := range world.Elements(w) {
		if !inTheActiveWindow(w, el) || !el.Addressable() {
			continue
		}
		out = append(out, resolver.Control{
			ID:    string(el.ID),
			Role:  string(el.Role),
			Label: strings.TrimSpace(el.Label),
		})
	}
	// Deterministic order, so two runs over one reading send the same question. Nothing
	// about the order is a hint: it is the id order the world already has.
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	if len(out) > HowManyControlsToOffer {
		out = out[:HowManyControlsToOffer]
	}
	return out
}

func inTheActiveWindow(w *directorapi.WorldState, el *directorapi.Element) bool {
	return w.ActiveWindow == nil || el.WindowID == *w.ActiveWindow
}

// Ask asks the configured interpreter which observed control the request meant.
//
// Returns no proposal — never an error — when nothing is configured, when there is nothing
// addressable to choose between, when the interpreter abstains, or when it names anything that
// is not a Prop of this world. The caller cannot tell those apart and does not need to: each of
// them means the same thing, which is that nothing interpreted the request.
func Ask(ctx context.Context, w *directorapi.WorldState, request string) (Proposal, bool) {
	if !Available() || w == nil || strings.TrimSpace(request) == "" {
		return Proposal{}, false
	}
	controls := Candidates(w)
	if len(controls) == 0 {
		return Proposal{}, false
	}
	choice := resolver.WhichControl(ctx, request, applicationOf(w), controls)
	if choice.Abstained() {
		return Proposal{}, false
	}
	// ASKED OF THE WORLD AGAIN, not merely of the list that was sent. The client already
	// refuses an id it did not offer; this refuses one the world no longer has, which is
	// the difference between a check on the reply and a check on reality.
	id := directorapi.ElementID(choice.Control)
	el, ok := w.Element(id)
	if !ok || !el.Addressable() {
		return Proposal{}, false
	}
	return Proposal{ElementID: id, Because: choice.Because}, true
}

func applicationOf(w *directorapi.WorldState) string {
	if w.ActiveWindow == nil {
		return ""
	}
	if win, ok := w.Window(*w.ActiveWindow); ok {
		return win.Application
	}
	return ""
}
