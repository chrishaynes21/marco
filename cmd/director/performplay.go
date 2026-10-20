package main

// SELECTING WHAT TO PERFORM, WHEN IT MIGHT BE A PLAY.
//
// `director perform-workflow` has always chosen between two things: a saved demonstration by name
// and an artifact file. A third arrives here — a demonstrated ACT, a saved `.marco` play whose own
// sentences are the order ([[ADR-243-a-door-of-its-own-and-the-order-in-the-file]]).
//
// It is a flag on this command rather than a command of its own, and that is the point. Everything
// between choosing what to perform and pressing something is the same: the window is acquired the
// same way, the scope gate is the same gate, the screens are named by the same person, each press
// is authorised at the same desk, the pointer and hover probes ask the same questions, the stop
// key reaches the same runner, and the run is cited by the same reference. A second command would
// have been a second place where all of that was assembled — and a second place is how two
// answers to "may this be pressed" come to exist.
//
// What differs is one thing: WHERE THE ORDER COMES FROM. A workflow's order is the artifact's, and
// the coordinator's cursor walks it. A play's order is the play's, and the coordinator is told
// which interaction each sentence means.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// whatToPerform resolves the one thing this run performs, from whichever way it was named.
//
// Returns the artifact and, for a play, the preflighted plan. A nil plan means an ordinary
// workflow run and the cursor is the order.
func whatToPerform(file, named, play string) (workflowstore.Artifact,
	*demonstrated.Plan, int) {

	if play == "" {
		art, code := workflowToPerform(file, named)
		return art, nil, code
	}
	if file != "" || named != "" {
		fmt.Fprintln(os.Stderr, "director: --play names a saved play, and --name or "+
			"--workflow names what to perform directly; running whichever was\n"+
			"  meant is not a guess this command makes")
		return workflowstore.Artifact{}, nil, 2
	}
	plan, code := planToPerform(play)
	if code != 0 {
		return workflowstore.Artifact{}, nil, code
	}
	// THE ARTIFACT COMES FROM THE PLAN, not from a second lookup.
	//
	// `Preflight` has already found the routine, checked that it is still the revision
	// the play was bound to, and refused otherwise. Looking it up again here would be a
	// second answer to "which demonstration is this" — and the two could differ, because
	// nothing stops a routine being re-saved between two reads of the store.
	return plan.Routine, &plan, 0
}

// planToPerform loads a saved play with its bindings and preflights the pair.
//
// Everything that can be answered without looking at a screen is answered here, before the window
// is even acquired: the record's version, the digest, one routine pinned to one revision, the
// routine present and still that revision, every position real, and the play rebuilt from the
// record and compared byte for byte. See `demonstrated.Preflight`.
func planToPerform(name string) (demonstrated.Plan, int) {
	rt, ok := playNamed(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "director: nothing called %q has been saved as a play.\n"+
			"  director sequences    what you have demonstrated\n", name)
		return demonstrated.Plan{}, 1
	}
	src, binds, err := demonstrated.Load(routesHere(), rt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return demonstrated.Plan{}, 1
	}
	store, serr := demo.Open(configDir())
	if serr != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", serr)
		return demonstrated.Plan{}, 1
	}
	plan, perr := demonstrated.Preflight(src, binds, store, marcoChecks{})
	if perr != nil {
		// A REFUSAL BEFORE ANYTHING IS ACQUIRED, GROUNDED OR ASKED. Exit 1 rather than
		// 2: the command was used correctly and the thing it was pointed at cannot be
		// performed, which are different failures and send somebody to different
		// places.
		fmt.Fprintf(os.Stderr, "director: %v\n", perr)
		return demonstrated.Plan{}, 1
	}
	plan.Name = rt.Slug
	return plan, 0
}

// playNamed finds the route a play was saved under, by the name a person says.
//
// Case-insensitively over the registry's own listing, because somebody asking for "ark graphics
// live" is asking for the thing they called "Ark Graphics Live" — the same rule
// `demo.Store.Sequence` uses, for the same reason.
func playNamed(name string) (routes.Route, bool) {
	want := routes.Slug(name)
	for _, rt := range routesHere().List() {
		if strings.EqualFold(rt.Slug, want) {
			return rt, true
		}
	}
	return routes.Route{}, false
}

// routesHere is the route store this Director reads plays from.
//
// `learnedRegistry` already is that store — the same one ordinary learned and authored routes
// use, honouring $MARCO_ROUTES exactly as `marco` does. Named again here only so a reader of
// this file does not have to know that a play lives among the plays.
func routesHere() routes.Registry { return learnedRegistry() }

// looksPerPlay is the slack a play-driven run gets on top of its patience.
//
// `--patience` is how many observations ONE interaction may consume before the coordinator gives
// up on it, and a play's budget is that per interaction. The slack covers the looks spent getting
// anywhere at all: naming the screen, waiting for a person to answer, the readings a verdict needs
// after the last press. Generous on purpose — it is a backstop against a loop that would otherwise
// not end, not a timing policy. The run's real limit is `--seconds`, which is a wall clock.
const looksPerPlay = 240

// outcomeOfAct is how a run that performed a PLAY ended.
//
// # Why the artifact cursor cannot answer this
//
// `outcomeOfRun` takes "did it finish" from `rep.Progress.Complete()` — the cursor reaching the
// end of the artifact. That is right for a workflow, where the artifact IS the thing being
// performed, and wrong for an Act, which may legitimately be a subset of a longer demonstration
// or its interactions in another order.
//
// The defect: an Act composed of the first interaction of a three-step demonstration verified
// everything it asked for, and was recorded as REFUSED — because the cursor stopped at 1 of 3. A
// reordered Act had the same problem. The person's routine did exactly what it said and the record
// said Marco declined.
//
// So completion comes from the play's own account, which knows how many sentences it has and what
// each one answered. Every per-action number the record carries — presses sent, verified,
// contradicted, unconfirmed — still comes off the runner, because those are facts about what
// reached the machine and this changes none of them.
//
// A stop still outranks everything: somebody chose it.
func outcomeOfAct(acc *demonstrated.Account, stopped bool) RunOutcome {
	if stopped {
		return RunCancelled
	}
	switch out := acc.Outcome(); out {
	case runref.Verified, runref.Contradicted, runref.Unconfirmed,
		runref.Cancelled, runref.Failed, runref.Refused:
		// ONE VOCABULARY, so this is a pass-through rather than a translation:
		// `RunOutcome` IS `runref.Outcome` (runrecord.go:51), and the account already
		// speaks it. Enumerated rather than returned blindly so that a word added to
		// one side cannot arrive here unconsidered.
		return out
	default:
		return RunRefused
	}
}

// endedBecause separates a run that was STOPPED from one that ran out of time.
//
// `context.WithDeadline` says which: `Canceled` is the parent context ending — an interrupt, the
// cross-process stop signal — and `DeadlineExceeded` is the run's own `--seconds`. Both close
// every prompt and every wait, which is the point of putting the deadline in the context, and
// they mean opposite things in the record a person reads afterwards:
//
//	cancelled   somebody chose to end this
//	expired     Marco was given N seconds and used them
//
// A stop key outranks both, because that is somebody choosing too.
//
// Expiry deliberately does NOT become an outcome word of its own. The vocabulary both binaries
// share has seven and none of them is "expired" — and inventing an eighth for this would be a
// word every future reader pays for, to say something the per-action counts already say
// truthfully: an action that went out with nothing answering for it is `unconfirmed`, and a run
// that pressed nothing is `refused`. What expiry gets is the REASON, which travels in the
// record's `because` and in the run's own last line.
func endedBecause(ctx context.Context, stoppedByKey bool) (expired, stopped bool) {
	if stoppedByKey {
		return false, true
	}
	switch err := ctx.Err(); {
	case err == nil:
		return false, false
	case errors.Is(err, context.DeadlineExceeded):
		return true, false
	default:
		return false, true
	}
}

// verdictWord is an attempt's verdict as one word, for the structured account.
//
// The verdict and not a sentence: a front end deciding what to show needs "verified" against
// "contradicted" against "unconfirmed", and the sentence that explains it is in the transcript
// beside it. Empty while nothing has been decided.
func verdictWord(a workflowrun.Attempt) string {
	if a.Verified == nil {
		return ""
	}
	return string(a.Verified.Verdict)
}
