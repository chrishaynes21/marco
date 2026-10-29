package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
)

// runDispatch is the machine seam a front-end (the overlay) uses to hold a
// conversation: `marco dispatch "<phrase>" [--json]` classifies the phrase into a
// single ROUTE-LEVEL decision (run / teach / chat / clarify) using the deterministic
// matcher plus, when configured, the local-LLM Advisor. It does NOT act — the caller
// decides whether to run/learn/speak — so it's safe to call speculatively.
//
// Without --json it prints a short human-readable line (for eyeballing at a shell);
// with --json it prints {"intent","route","name","reply"} for a program to parse.
func runDispatch(args []string) {
	jsonMode := false
	var rest []string
	for _, a := range args {
		if a == "--json" {
			jsonMode = true
		} else {
			rest = append(rest, a)
		}
	}
	phrase := strings.TrimSpace(strings.Join(rest, " "))
	if phrase == "" {
		fmt.Fprintln(os.Stderr, `usage: marco dispatch "<phrase>" [--json]`)
		os.Exit(2)
	}
	// ── THE RESOLUTION IS DIRECTOR'S, AND THIS IS A THIN ADAPTER OVER IT ──
	//
	// It used to run  here, against this process's own slug list and its own
	// idea of the foreground — which is a second answer to "which saved routine did they
	// mean", out of a library nobody checked. Director owns that now. What is kept is this
	// command's shape: one phrase in, one classification out, printing either prose or
	// JSON, acting on nothing.
	//
	// It needs Director, and says which of the three ways it could not be asked. There is
	// no local fallback: exact matching is part of the resolution and not a smaller
	// question this command may answer for itself.
	dec, why := dispatchThroughDirector(phrase)
	if why != "" {
		if jsonMode {
			_ = json.NewEncoder(os.Stdout).Encode(struct {
				Why string
			}{why})
		} else {
			fmt.Fprintln(os.Stderr, why)
		}
		os.Exit(resolveExit)
	}
	if jsonMode {
		enc := json.NewEncoder(os.Stdout)
		_ = enc.Encode(struct {
			Intent string `json:"intent"`
			Route  string `json:"route"`
			Name   string `json:"name"`
			Reply  string `json:"reply"`
		}{dec.Intent, dec.Route, dec.Name, dec.Reply})
		return
	}
	switch dec.Intent {
	case dispatch.IntentRun:
		fmt.Printf("run: %s\n", prettyRoute(dec.Route))
	case dispatch.IntentLearn:
		fmt.Printf("learn: %s\n", dec.Name)
	case dispatch.IntentChat, dispatch.IntentClarify:
		fmt.Println(dec.Reply)
	default:
		fmt.Println("(nothing to do)")
	}
}

// converseTurn handles one line in the interactive assistant using dispatch's
// Advisor (the local-LLM brain). It returns true when it fully handled the line
// (ran, learned, or replied), false when there's no brain wired or it had nothing
// confident — in which case the caller falls back to its own deterministic
// fuzzy-confirm flow (including any legacy $MARCO_RESOLVER). So the assistant
// behaves exactly as before unless a converse-capable brain is configured.
func converseTurn(d orchestrator.Deps, line string) bool {
	// THROUGH DIRECTOR, and still ONLY the resolver`s own suggestion.
	//
	// `Propose` meant "the Advisor`s validated proposal and nothing else", so that an
	// unconfigured machine left this REPL`s own flow intact. Director answers every
	// request — exact, advisor or fallback — so the same meaning is `source == advisor`
	// and nothing else: the arm that IS the Advisor. A thin adapter, and the semantics
	// are the ones that were here.
	proposal, ok := proposeThroughDirector(line)
	if !ok {
		return false
	}
	switch proposal.Intent {
	case dispatch.IntentRun:
		if proposal.Reply != "" {
			fmt.Println(proposal.Reply)
		}
		// THE MODEL PROPOSES; THE PERSON DECIDES.
		//
		// This used to run the play the model named, with no question asked — so an external
		// model chose which durable behaviour to perform and the one intake merely exact-
		// matched the slug it had already picked. That is the shape Phase 2 deleted from
		// `marco do`: a guess in front of the door, wearing the door's authority.
		//
		// The two arms beside this one in the REPL — the deterministic fuzzy match and the
		// legacy $MARCO_RESOLVER — both ask before they substitute anything. This one did
		// not, and it was the only one that could be an arbitrary program.
		//
		// Asking is enough. What a person confirms is their own invocation, and it goes
		// through the same intake as every other; what they decline is not performed.
		//
		// Deleting the confirmation must fail TestTheAdvisorProposesAndDoesNotPerform.
		if !askYes(fmt.Sprintf("Run %q? [y]es / [n]o: ", prettyRoute(proposal.Route))) {
			fmt.Printf("Left alone.\n")
			return true
		}
		runDo(d, proposal.Route)
		return true
	case dispatch.IntentLearn:
		// Offer to learn it, then hand off to the normal learn-by-demonstration flow
		// under the suggested name.
		if proposal.Reply != "" {
			fmt.Println(proposal.Reply)
		}
		if askYes(fmt.Sprintf("Learn %q now? [y]es / [n]o: ", proposal.Name)) {
			if err := d.Learn(proposal.Name); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
		return true
	case dispatch.IntentChat, dispatch.IntentClarify:
		fmt.Println(proposal.Reply)
		return true
	default:
		return false
	}
}

// dispatchThroughDirector asks Director and shapes the answer as this command has always printed
// it, or says why it could not be asked.
//
// A COMPATIBILITY ADAPTER and nothing more: the four intent words are dispatch's own and reach
// here unchanged, because Director answers with them.
func dispatchThroughDirector(phrase string) (dispatch.Decision, string) {
	c, err := directorConnect(false)
	if err != nil {
		return dispatch.Decision{}, whyNoDirectorHere(err)
	}
	defer c.Close()
	out, err := c.ResolveRoute(service.ResolveRoutePayload{
		Request: phrase, Library: marcopaths.Routes(),
	})
	if err != nil {
		return dispatch.Decision{}, whyRefusedHere(err)
	}
	if out.Mismatch != "" {
		return dispatch.Decision{}, out.Mismatch
	}
	a := out.Answer
	// THE SLUG IS WHAT THIS COMMAND HAS ALWAYS PRINTED, and the whole identity is in the
	// JSON for anything that needs it. prettyRoute takes a slug.
	return dispatch.Decision{Intent: a.Kind, Route: a.Slug, Name: a.Name,
		Reply: a.Reply}, ""
}

// proposeThroughDirector is Director's answer, kept only when the resolver itself produced it.
//
// False for everything else, which is what `Propose` answered and is what leaves this REPL's own
// deterministic flow alone: a machine with no resolver configured, or one whose resolver had
// nothing to say, behaves exactly as it did before any of this existed. A Director that cannot be
// asked is the same answer for the same reason — said in the log rather than silently, because
// this is a developer surface and a silent nothing is what it would otherwise look like.
func proposeThroughDirector(line string) (dispatch.Decision, bool) {
	c, err := directorConnect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, whyNoDirectorHere(err))
		return dispatch.Decision{}, false
	}
	defer c.Close()
	out, err := c.ResolveRoute(service.ResolveRoutePayload{
		Request: line, Library: marcopaths.Routes(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, whyRefusedHere(err))
		return dispatch.Decision{}, false
	}
	if out.Mismatch != "" {
		fmt.Fprintln(os.Stderr, out.Mismatch)
		return dispatch.Decision{}, false
	}
	a := out.Answer
	if a.Source != dispatch.SourceAdvisor {
		return dispatch.Decision{}, false
	}
	return dispatch.Decision{Intent: a.Kind, Route: a.Slug, Name: a.Name,
		Reply: a.Reply}, true
}
