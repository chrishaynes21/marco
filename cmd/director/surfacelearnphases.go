package main

// The three phases of one live focused demonstration, kept apart.
//
// # Why they are three commands and not one
//
// The owner's third condition, and it is a safety property rather than a convenience:
//
//	Separate learning from execution. Human demonstration first, attributed transition and
//	disposable memory second, then one separately authorized guarded click. No automatic retries
//	or exploration.
//
// So `-learn` and `-reground` build a DRY RUN runtime — the only host that could reach the
// desktop is `runtime.DryRunHost{}`, which is a property of the object graph and not a promise —
// and `-execute` is the one phase that builds a real one, needs its own flag, and performs
// exactly one click.
//
// Nothing here retries. Every refusal below returns, says why, and stops; there is no loop that
// tries again with a different target, a wider margin or a second press.

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/surface"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// surfacePhase is the shared setup every phase needs: a sandbox, the pixel-only configuration,
// and a runtime.
//
// `wet` is the one difference between the phases that cannot press anything and the one that can,
// and it is a single boolean passed to `NewRuntime` — so a reader can see which phase is which by
// looking at one call.
func surfacePhase(out io.Writer, bridge string, maxNodes int, wet bool) (*Runtime, error) {
	if err := enablePixelOnly(os.Getenv("MARCO_HOME"), defaultHome()); err != nil {
		return nil, err
	}
	fmt.Fprintf(out, "%s\n", pixelOnlyNote())
	if wet {
		fmt.Fprintln(out, "runtime: LIVE — this phase can press exactly once")
	} else {
		fmt.Fprintln(out, "runtime: DRY RUN — nothing on this path can reach the desktop")
	}
	if _, err := os.Stat(bridge); err != nil {
		// Still required: the runtime constructs the bridge host even when the
		// accessibility provider is not registered, and a missing one fails late and
		// confusingly rather than here.
		return nil, fmt.Errorf("the accessibility bridge is not at %s; it is still "+
			"needed to construct the runtime even though this experiment does not "+
			"observe with it", bridge)
	}
	graph, err := openGraph()
	if err != nil {
		return nil, err
	}
	return NewRuntime(bridge, maxNodes, !wet, graph)
}

// liveStage collects one Stage and reports the foreground application with it.
func liveStage(rt *Runtime) (directorapi.WorldState, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	w, err := observeStage(ctx, rt)
	if err != nil {
		return directorapi.WorldState{}, "", err
	}
	app := ""
	if w.ActiveWindow != nil {
		if win, ok := w.Window(*w.ActiveWindow); ok {
			app = win.Application
		}
	}
	return w, app, nil
}

// theAmbiguityIsReallyThere is the owner's second condition, asked of the live screen.
//
//	Verify that the live Stage actually contains the nested caption/row candidates before asking
//	for a demonstration. Don't assume the recorded fixture predicts the live screen.
//
// And the consequence of its being false, which is to say so:
//
//	If the live Settings screen doesn't produce the expected ambiguity, the test should report
//	that result rather than manufacturing an ambiguous resolution.
func theAmbiguityIsReallyThere(out io.Writer, w directorapi.WorldState, instruction string) (
	surface.Question, bool) {

	words := instructionWords(instruction)
	h := surface.New().Evaluate(&w, instruction, "click",
		directorapi.ElementQuery{Text: words})
	fmt.Fprintf(out, "\nthe live Stage: %d Props; %q → %s\n", len(w.Elements), words,
		h.Decision)
	if h.Identity != nil {
		fmt.Fprintf(out, "  identity %s %q role=%s sources=%v rejected=%q\n",
			h.Identity.ElementID, h.Identity.Label, h.Identity.Role,
			h.Identity.Sources, h.Identity.Rejected)
	}
	for _, c := range h.Candidates {
		verdict := "DEFENSIBLE"
		if !c.Defensible() {
			verdict = "refused: " + c.Rejected
		}
		fmt.Fprintf(out, "  candidate %s %v  area %.1f%%  %s\n", c.ElementID, c.Bounds,
			100*c.Evidence.AreaShare, verdict)
	}
	fmt.Fprintf(out, "  %s\n", h.Explanation)

	q, ok := h.Question()
	if !ok {
		fmt.Fprintf(out, "\nSTOP: this screen does not produce a surface question, so "+
			"there is nothing to demonstrate. That is the result, not a failure — "+
			"and the answer is to find a screen where identity and interaction "+
			"surface really do come apart, never to loosen what counts as one.\n")
		return surface.Question{}, false
	}
	return q, true
}

// ── LEARN ─────────────────────────────────────────────────────────────────────

// runSurfaceLearn is phases A to E: ask, watch a person demonstrate, and keep what was confirmed.
//
// It cannot press anything: the runtime is a dry run. The only thing that touches the desktop in
// this phase is the person's own hand.
func runSurfaceLearn(out io.Writer, instruction, bridge string, maxNodes, seconds int) int {
	if instruction == "" {
		fmt.Fprintln(out, "give the instruction with -instruction")
		return 2
	}
	rt, err := surfacePhase(out, bridge, maxNodes, false)
	if err != nil {
		fmt.Fprintf(out, "STOP: %v\n", err)
		return 1
	}
	defer rt.Close()

	// AMBIENT WATCHING, AND AMBIENT LEARNING. The first opens the navigation subscription
	// that populates the input log a demonstration arrives through; the second is what
	// licenses establishing the screen as a durable Place, which a target needs in order to
	// have any identity at all.
	rt.EnableAmbient()
	rt.EnableAmbientLearning()
	defer rt.DisableAmbient()

	w, app, err := liveStage(rt)
	if err != nil {
		fmt.Fprintf(out, "STOP: could not observe: %v\n", err)
		return 1
	}
	fmt.Fprintf(out, "foreground application: %q\n", app)
	if app == "" {
		fmt.Fprintln(out, "STOP: no foreground application could be established.")
		return 1
	}

	// B. THE AMBIGUITY, VERIFIED ON THE LIVE SCREEN BEFORE ANYTHING IS ASKED.
	if _, ok := theAmbiguityIsReallyThere(out, w, instruction); !ok {
		return 1
	}

	// AND A DURABLE PLACE, which ambient learning establishes on its own cadence.
	place := waitForPlace(out, rt, app, seconds)
	if place == "" {
		fmt.Fprintln(out, "STOP: no durable place was established for this screen, so "+
			"a target on it has no durable identity and an answer could not be "+
			"kept. Leave the screen still for a few seconds and try again.")
		return 1
	}
	fmt.Fprintf(out, "durable place: %s\n", place)

	// THE REQUEST. A dry run, so a clarification is the only thing that can come back —
	// and if something else does, that is reported rather than worked around.
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	outcome := rt.Handle(ctx, instruction, nil)
	fmt.Fprintf(out, "\nrequest: %s — %s\n", outcome.Status, outcome.Message)
	if outcome.Status != directorapi.ResultNeedsClarification ||
		outcome.SurfaceQuestion == nil {
		fmt.Fprintln(out, "STOP: the request did not raise a surface question, so there "+
			"is nothing for a demonstration to answer.")
		return 1
	}

	// C AND D. THE PERSON DEMONSTRATES, and the existing correlator decides whether the
	// screen change that follows is attributable to exactly that action.
	fmt.Fprintf(out, "\n%s\n", outcome.Message)
	// AND ONLY NOW is there anything to answer. Everything above had to succeed first:
	// the Stage, the ambiguity, the durable place, and a request that actually raised the
	// question. Somebody told to click before this point would be pressing at a screen
	// with no open question, and the press would be dropped rather than banked.
	checkpoint(out, checkDemoNeeded)
	fmt.Fprintf(out, "Click the control, on a part of it the words do not cover. "+
		"Watching for %ds. Marco will not press anything.\n", seconds)
	return reportDemonstration(out, rt, app, seconds)
}

// waitForPlace polls until ambient watching has established a durable place for this screen.
func waitForPlace(out io.Writer, rt *Runtime, app string, seconds int) string {
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	for time.Now().Before(deadline) {
		place, _, _, got := rt.ambient().standing()
		if place != "" && sameApplication(got, app) {
			return place
		}
		time.Sleep(time.Second)
	}
	return ""
}

// reportDemonstration watches the learner and reports the three facts separately.
//
// The owner's requirement, and the reason they are three lines rather than one verdict:
//
//	Report observed press, attributed transition and intended outcome as separate facts.
func reportDemonstration(out io.Writer, rt *Runtime, app string, seconds int) int {
	l := rt.surfaceLearner()
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	seen := 0
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		// STOP ON A CHANGED FOREGROUND. A press made in another application is not an
		// answer to this question, and carrying on watching would be waiting for one.
		if _, _, _, now := rt.ambient().standing(); now != "" && !sameApplication(now, app) {
			fmt.Fprintf(out, "\nSTOP: the foreground moved to %q while the question "+
				"was open.\n", now)
			return 1
		}
		got := l.Lessons()
		for ; seen < len(got); seen++ {
			fmt.Fprintf(out, "  %s: %s\n", got[seen].Outcome, got[seen].Detail)
		}
		if l.Open() == nil && seen > 0 {
			break
		}
	}

	// THE THREE FACTS.
	fmt.Fprintln(out, "\nWHAT WAS ESTABLISHED, as three separate claims:")
	learned := ""
	for _, g := range l.Lessons() {
		switch g.Outcome {
		case "settled", "undiscriminating", "elsewhere", "learned", "unconfirmed":
			learned = g.Outcome
		}
	}
	press := learned == "settled" || learned == "learned" || learned == "unconfirmed"
	fmt.Fprintf(out, "  observed press ............ %v\n", press)
	fmt.Fprintf(out, "  attributed transition .... %v\n", learned == "learned")
	fmt.Fprintf(out, "  intended outcome verified  NOT IMPLEMENTED — an attributed "+
		"transition says the interaction did something, never that the right thing "+
		"happened\n")

	if learned != "learned" {
		fmt.Fprintf(out, "\nSTOP: nothing executable was learned (%q). Inspect the "+
			"store at %s and discard it with `director reset-test-state`.\n",
			learned, semanticMemoryPath())
		return 1
	}
	fmt.Fprintf(out, "\nE. the saved relation is in %s. Inspect it: it carries a "+
		"relation and a depth, normalised evidence, and the target by its durable "+
		"subject id — no absolute coordinate anywhere.\n", semanticMemoryPath())
	return 0
}

// ── REGROUND ──────────────────────────────────────────────────────────────────

// runSurfaceReground is phase F: a fresh Stage, and what the saved relation picks out of it.
//
// A dry run. Nothing is pressed and nothing is authorised; this reports the decision that the
// execute phase would act on, so the decision can be read before anything happens.
func runSurfaceReground(out io.Writer, instruction, bridge string, maxNodes int) int {
	rt, err := surfacePhase(out, bridge, maxNodes, false)
	if err != nil {
		fmt.Fprintf(out, "STOP: %v\n", err)
		return 1
	}
	defer rt.Close()
	rt.EnableAmbient()
	defer rt.DisableAmbient()

	w, app, err := liveStage(rt)
	if err != nil {
		fmt.Fprintf(out, "STOP: could not observe: %v\n", err)
		return 1
	}
	place := waitForPlace(out, rt, app, 30)
	fmt.Fprintf(out, "foreground %q; durable place %q\n", app, place)
	if place == "" {
		fmt.Fprintln(out, "STOP: this screen has no durable place, so nothing stored "+
			"could be about it.")
		return 1
	}
	g := rt.pipeline.Surfaces.Ground(&w, app, place, instruction, "click",
		directorapi.ElementQuery{Text: instructionWords(instruction)})
	fmt.Fprintf(out, "\n%s\n", g.Because)
	if !g.Shown.Given() {
		fmt.Fprintln(out, "\nSTOP: the saved demonstration does not ground this request "+
			"against the screen in front. That is the refusal working; it never "+
			"falls back to the caption.")
		return 1
	}
	fmt.Fprintf(out, "\nF. it would vouch for %s and press the surface %s.\n",
		g.Shown.ElementID, g.Shown.Surface)
	return 0
}

// ── EXECUTE ───────────────────────────────────────────────────────────────────

// runSurfaceExecute is phases G and H: one guarded click, and what the screen did.
//
// The ONE phase with a live runtime, and it needs its own flag. Everything between the
// instruction and the desktop is the production path: the resolver, ADR-202's licence, policy,
// the confirmation gate, the capability ladder and the Marco runtime, none of them changed for
// this experiment.
func runSurfaceExecute(out io.Writer, instruction, bridge string, maxNodes int,
	authorised bool) int {

	if !authorised {
		fmt.Fprintln(out, "STOP: this phase presses something. Re-run with "+
			"-authorise-one-click to authorise exactly one click.")
		return 2
	}
	rt, err := surfacePhase(out, bridge, maxNodes, true)
	if err != nil {
		fmt.Fprintf(out, "STOP: %v\n", err)
		return 1
	}
	defer rt.Close()
	rt.EnableAmbient()
	defer rt.DisableAmbient()

	before, app, err := liveStage(rt)
	if err != nil {
		fmt.Fprintf(out, "STOP: could not observe: %v\n", err)
		return 1
	}
	place := waitForPlace(out, rt, app, 30)
	fmt.Fprintf(out, "foreground %q; durable place %q; %d Props before\n", app, place,
		len(before.Elements))
	if place == "" {
		fmt.Fprintln(out, "STOP: no durable place for this screen.")
		return 1
	}

	// THE ONE CLICK, through the ordinary path. `Handle` observes afresh for itself, so the
	// Stage above is the before-state for the report rather than the one acted on.
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	outcome := rt.Handle(ctx, instruction, nil)
	reportExecution(out, outcome)

	// H. AND WHAT THE SCREEN DID, reported as a separate fact from the press.
	after, afterApp, aerr := liveStage(rt)
	fmt.Fprintln(out, "\nWHAT HAPPENED, as separate claims:")
	fmt.Fprintf(out, "  a press was sent ......... %v\n",
		outcome.Status == directorapi.ResultDone ||
			outcome.Status == directorapi.ResultPartial)
	if aerr != nil {
		fmt.Fprintf(out, "  the screen afterwards .... could not be observed: %v\n", aerr)
		return 1
	}
	fmt.Fprintf(out, "  the screen changed ....... %v (%d Props before, %d after, "+
		"foreground %q)\n", len(after.Elements) != len(before.Elements),
		len(before.Elements), len(after.Elements), afterApp)
	fmt.Fprintln(out, "  intended destination ..... READ IT YOURSELF AND SAY. Nothing "+
		"here can verify that the right page opened: that is the third claim, it "+
		"is not implemented, and a screen change is not it.")
	return 0
}

// reportExecution prints what each stage of the production path decided.
func reportExecution(out io.Writer, outcome execute.Outcome) {
	fmt.Fprintf(out, "\nrequest: %s — %s\n", outcome.Status, outcome.Message)
	for _, st := range outcome.Stages {
		mark := "  "
		if !st.OK {
			mark = " !"
		}
		fmt.Fprintf(out, "%s %-9s %s\n", mark, st.Name, st.Detail)
	}
	if outcome.Resolution != nil && outcome.Resolution.Target != nil {
		t := outcome.Resolution.Target
		fmt.Fprintf(out, "\n  resolved %s %q; surface %q; press at %v\n",
			t.ElementID, t.Label, t.Surface, t.Point)
	}
	if outcome.Policy != nil {
		fmt.Fprintf(out, "  policy: allowed=%v confirm=%v %s\n", outcome.Policy.Allowed,
			outcome.Policy.RequiresConfirmation, outcome.Policy.Reason)
	}
}
