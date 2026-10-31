package main

// THE TEXT A ROUTINE NEEDS, ASKED FOR BESIDE PLAY.
//
// These are not maintenance. A field somebody must fill in before the routine can do anything is a
// REQUIRED INPUT to running it, so it sits beside Play and the window it will press in — not below
// the step panel and the phrases panel, which are about changing a routine rather than performing
// one.
//
// # Play cannot go round it
//
// `PlayRoutine` refuses a routine that needs text. That is the whole reason this is not just a
// panel: a window that offered the fields AND left the plain button live would let somebody press
// the one that types nothing, which is what the product did before any of this existed.
//
// # And what is confirmed is what runs
//
// The invocation is prepared by the backend, held HERE by its token, and confirmed by that token
// alone — so the page cannot hand back values it was not given, and a value edited between
// reading the confirmation and pressing it refuses. `orchestrator.ConfirmInvocation` re-asks all
// three questions; this window asks none of them itself.
//
// See [[Text-arguments-proposal-2026-10-30]].

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// invocationsKept is how many prepared invocations this window holds at once.
//
// More than one because the page can prepare, read, and go back to the fields several times
// before pressing; bounded because each one holds text somebody typed and nothing should keep
// that for longer than the window it might be confirmed in.
const invocationsKept = 8

// PlayValues is what a routine needs before it can be run, as the Play row shows it.
type PlayValues struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Fields is one per argument, in the order the routine uses them.
	Fields []PlayField `json:"fields"`
	// Needs is whether anything has to be supplied before Play can do anything. False for
	// every routine saved before this existed, which is the common case and must read as
	// an ordinary routine rather than as a problem.
	Needs bool `json:"needs"`
	// Why is why none of this can be offered: no routine chosen, a recording in progress,
	// a routine this library does not hold.
	Why string `json:"why,omitempty"`
}

// PlayField is one value a routine needs.
type PlayField struct {
	Name string `json:"name"`
	// Secret is whether the credential store answers for it rather than the person.
	//
	// NO BOX IS OFFERED FOR ONE. `oshost.doSecret` reads it from the store, and a run that
	// cannot find it refuses saying how to set it — so what this window owes somebody is
	// where the value comes from, not a field that would make it travel on a command line.
	Secret bool `json:"secret"`
	// Says is the line beside it.
	Says string `json:"says"`
}

// PlayConfirm is what pressing Play with values comes back with: what it would type, and a token.
type PlayConfirm struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Token string `json:"token,omitempty"`
	// Says is one line per argument, naming it and the text. The one place a value is
	// repeated, and it is on a screen rather than in a record.
	Says []string `json:"says,omitempty"`
	// Why is why this cannot be run, in the contract's own words.
	Why string `json:"why,omitempty"`
}

// PlayNeeds is what the chosen routine needs supplied before it can run.
func (a *Acts) PlayNeeds(id string) PlayValues {
	out, _, _, ok := a.playFor(id)
	if !ok {
		return out
	}
	return out
}

// playFor is the read every method here starts from.
func (a *Acts) playFor(id string) (PlayValues, orchestrator.Deps, Routine, bool) {
	id = strings.TrimSpace(id)
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	if id == "" {
		return PlayValues{Fields: []PlayField{},
			Why: "choose a routine to run"}, d, Routine{}, false
	}
	a.mu.Lock()
	recording := a.recordingLocked()
	a.mu.Unlock()
	if recording {
		return PlayValues{ID: id, Fields: []PlayField{},
				Why: "Marco is recording a demonstration; one thing at a time"}, d,
			Routine{}, false
	}
	chosen, found := a.routineWithID(id)
	if !found {
		return PlayValues{ID: id, Fields: []PlayField{},
				Why: fmt.Sprintf("there is no routine %q in this library (%s) — press "+
					"Refresh and choose it again", id, a.routesDir)}, d,
			Routine{}, false
	}
	// WHAT IT NEEDS, FROM THE BACKEND THAT WILL REFUSE IT. No values offered, which is
	// the question "what does this routine want" rather than "may it run" — so the Why
	// below is about this window and never about a value nobody has typed yet.
	i, err := d.PrepareInvocation(chosen.route(), nil)
	if err != nil {
		return PlayValues{ID: id, Name: chosen.Name, Fields: []PlayField{},
			Why: err.Error()}, d, Routine{}, false
	}
	out := PlayValues{ID: id, Name: chosen.Name, Fields: []PlayField{}, Needs: i.Needs()}
	for _, arg := range i.Arguments {
		out.Fields = append(out.Fields, PlayField{Name: arg.Name, Secret: arg.Secret,
			Says: fieldSays(arg)})
	}
	return out, d, chosen, true
}

// fieldSays is the line beside one field, and for a secret it is the whole of what the window
// owes somebody: where the value comes from and what to do when it is not there.
func fieldSays(arg routes.Argument) string {
	if arg.Secret {
		return "read from the credential store — if it is not set, the run says so and " +
			"`marco secret set` is how to set it"
	}
	return "typed into the routine where you tapped during the demonstration"
}

// ReviewPlay prepares the run and says what it would type, without running anything.
//
// # What `proposed` and `proposedAt` are for
//
// A request can already have said what to type — *write a note saying hello there* — and Director
// answers which words those are. The page puts them in the boxes, where they can be corrected,
// and sends BOTH what it offered and what came back. The contract compares them, so a value the
// person changed is labelled as theirs rather than as a model's reading: a window that prefilled a
// box and forgot to relabel it would tell somebody the model read their sentence correctly when
// they are the one who fixed it.
//
// `proposedAt` is the revision those names were read from. A routine saved since REFUSES, even
// when it declares the same names, because the same name in a rewritten routine may be typed
// somewhere else entirely. Both are empty for an ordinary press, which is exactly
// `PrepareInvocation`.
//
// See [[ADR-277-a-request-may-propose-the-text-a-routine-types]].
func (a *Acts) ReviewPlay(id string, values, proposed map[string]string,
	proposedAt string) PlayConfirm {

	got, d, chosen, ok := a.playFor(id)
	if !ok {
		return PlayConfirm{ID: id, Why: got.Why}
	}
	i, err := d.PrepareProposed(chosen.route(), values, proposed, proposedAt)
	if err != nil {
		return PlayConfirm{ID: id, Name: got.Name, Why: err.Error()}
	}
	if !i.Runnable() {
		// THE CONTRACT'S OWN SENTENCE. A window with a copy of the rule is a window
		// that offers a press and is then told off for it.
		return PlayConfirm{ID: id, Name: i.Name, Why: i.Why}
	}
	a.keepInvocation(i)
	return PlayConfirm{ID: id, Name: i.Name, Token: i.Token, Says: i.Says}
}

func (a *Acts) keepInvocation(i orchestrator.Invocation) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.invocations == nil {
		a.invocations = map[string]orchestrator.Invocation{}
	}
	if _, already := a.invocations[i.Token]; !already {
		a.invocationOrder = append(a.invocationOrder, i.Token)
	}
	a.invocations[i.Token] = i
	for len(a.invocationOrder) > invocationsKept {
		delete(a.invocations, a.invocationOrder[0])
		a.invocationOrder = a.invocationOrder[1:]
	}
}

// play hands a run to the thing that performs it, which is `playChosen` in the product.
func (a *Acts) play(chosen Routine, window string, values map[string]string,
	expect string) string {

	if a.playing != nil {
		return a.playing(chosen, window, values, expect)
	}
	return a.playChosen(chosen, window, values, expect)
}

// CancelPlay drops one prepared invocation.
//
// It wrote nothing, so there is nothing to undo — what this lets go of is the TEXT. A window
// holding somebody's message after they have changed their mind is the one thing R6 is about, and
// cancelling has to be something the page DOES rather than something it omits.
func (a *Acts) CancelPlay(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.invocations, token)
	for n, t := range a.invocationOrder {
		if t == token {
			a.invocationOrder = append(a.invocationOrder[:n],
				a.invocationOrder[n+1:]...)
			break
		}
	}
}

// ConfirmPlay runs the invocation this token answers for, in the window it is given.
//
// BY TOKEN ALONE. The values are the backend's copy of what the confirmation showed; a page
// handing them back would be a page that could hand back different ones. And the invocation is
// spent either way, so a second press of a button somebody already pressed runs nothing.
func (a *Acts) ConfirmPlay(token, window string) string {
	token = strings.TrimSpace(token)
	a.mu.Lock()
	i, held := a.invocations[token]
	a.mu.Unlock()
	if !held {
		return "this is no longer the run this window prepared — press Play again and " +
			"read what it would type"
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	if why := d.ConfirmInvocation(i); why != "" {
		a.CancelPlay(token)
		return why
	}
	chosen, found := a.routineWithID(i.ID)
	if !found {
		a.CancelPlay(token)
		return fmt.Sprintf("there is no routine %q in this library (%s) any more",
			i.ID, a.routesDir)
	}
	a.CancelPlay(token)
	// THE VERSION IT WAS SEALED WITH, and never a fresh reading of the file: a reading
	// taken here would be a reading of whatever is on disk NOW, which is the very thing
	// the pin exists to catch. See invoke.Request.Expect.
	return a.play(chosen, window, i.Values, i.SourceAt)
}
