package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/resolveroute"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
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
		// AND THE IDENTITY IS ACTUALLY IN IT NOW.
		//
		// The comment here said the whole identity was in the JSON for anything that
		// needed it, and it was not: only the slug was. A front end reading this and
		// building an invocation from `route` alone would have been building one that
		// gets resolved again. `route` stays, because something may be reading it.
		_ = enc.Encode(struct {
			Intent string `json:"intent"`
			Route  string `json:"route"`
			Name   string `json:"name"`
			Reply  string `json:"reply"`
			ID     string `json:"id,omitempty"`
			App    string `json:"app,omitempty"`
			Slug   string `json:"slug,omitempty"`
			Focus  bool   `json:"focus,omitempty"`
		}{dec.Intent, dec.slug(), dec.Name, dec.Reply,
			dec.ID, dec.Route.App, dec.Route.Slug, dec.Route.Focus})
		return
	}
	switch dec.Intent {
	case dispatch.IntentRun:
		fmt.Printf("run: %s\n", prettyRoute(dec.slug()))
		fmt.Printf("identity: %s\n", dec.ID)
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
	p, ok := proposeThroughDirector(line)
	if !ok {
		return false
	}
	switch p.Intent {
	case dispatch.IntentRun:
		if p.Reply != "" {
			fmt.Println(p.Reply)
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
		// AND THE QUESTION NAMES THE WHOLE IDENTITY, because a name is not unique:
		// "Run \"settings\"?" is not a question somebody can answer when two
		// routines answer to it.
		if !askYes(fmt.Sprintf("Run %q (%s)? [y]es / [N]o: ",
			prettyRoute(p.slug()), p.ID)) {
			fmt.Printf("Left alone.\n")
			return true
		}
		// THE IDENTITY, NOT THE WORDS. `runDo` sends a phrase through the door,
		// which resolves it AGAIN by the registry's priority order — so with two
		// routines of that name the one performed is whichever the second lookup
		// lands on and not the one that was just agreed to. This is the intake's
		// explicit-identity arm, where nothing is looked up.
		runChosen(d, p)
		return true
	case dispatch.IntentLearn:
		// Offer to learn it, then hand off to the normal learn-by-demonstration flow
		// under the suggested name.
		if p.Reply != "" {
			fmt.Println(p.Reply)
		}
		if askYes(fmt.Sprintf("Learn %q now? [y]es / [N]o: ", p.Name)) {
			if err := d.Learn(p.Name); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
		return true
	case dispatch.IntentChat, dispatch.IntentClarify:
		fmt.Println(p.Reply)
		return true
	default:
		return false
	}
}

// proposal is Director's answer as this command acts on it: a classification AND an identity.
//
// # Why the identity and not just the slug
//
// Because a slug is not a route. The first version of this adapter reduced Director's answer to
// `a.Slug`, the REPL confirmed that slug, and `runDo` sent it through the door as WORDS — which
// resolves it again, by the registry's own priority order. With two routines of that name in two
// scopes, or in two applications, the one performed is whichever the second lookup lands on and
// not the one Director proposed and the person agreed to.
//
// So the whole identity travels: the route this is, in the three parts an invocation is built
// from, used through the intake's EXPLICIT-IDENTITY arm where no phrase is looked up at all.
type proposal struct {
	Intent string
	Name   string
	Reply  string
	// Route is the whole identity, and ID is it as a person reads it. Zero for anything
	// but a run.
	Route routes.Route
	ID    string
}

// slug is what this command has always PRINTED, which is a display concern and never the thing
// that is run.
func (p proposal) slug() string { return p.Route.Slug }

// dispatchThroughDirector asks Director and shapes the answer as this command has always printed
// it, or says why it could not be asked.
//
// A COMPATIBILITY ADAPTER and nothing more: the four intent words are dispatch's own and reach
// here unchanged, because Director answers with them.
func dispatchThroughDirector(phrase string) (proposal, string) {
	c, err := directorConnect(false)
	if err != nil {
		return proposal{}, whyNoDirectorHere(err)
	}
	defer c.Close()
	out, err := c.ResolveRoute(service.ResolveRoutePayload{
		Request: phrase, Library: marcopaths.Routes(),
	})
	if err != nil {
		return proposal{}, whyRefusedHere(err)
	}
	if out.Mismatch != "" {
		return proposal{}, out.Mismatch
	}
	return proposalOf(out.Answer), ""
}

// proposalOf is one answer as this command acts on it.
//
// Every part of the identity, carried rather than rebuilt: `app` and `focus` are what tell two
// routines of one name apart, and dropping them is how a confirmation comes to mean a different
// routine from the one it described.
func proposalOf(a resolveroute.Answer) proposal {
	return proposal{Intent: a.Kind, Name: a.Name, Reply: a.Reply, ID: a.ID,
		Route: routes.Route{App: a.App, Focus: a.Focus, Slug: a.Slug}}
}

// proposeThroughDirector is Director's answer, kept only when the resolver itself produced it.
//
// False for everything else, which is what `Propose` answered and is what leaves this REPL's own
// deterministic flow alone: a machine with no resolver configured, or one whose resolver had
// nothing to say, behaves exactly as it did before any of this existed. A Director that cannot be
// asked is the same answer for the same reason — said in the log rather than silently, because
// this is a developer surface and a silent nothing is what it would otherwise look like.
func proposeThroughDirector(line string) (proposal, bool) {
	c, err := directorConnect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, whyNoDirectorHere(err))
		return proposal{}, false
	}
	defer c.Close()
	out, err := c.ResolveRoute(service.ResolveRoutePayload{
		Request: line, Library: marcopaths.Routes(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, whyRefusedHere(err))
		return proposal{}, false
	}
	if out.Mismatch != "" {
		fmt.Fprintln(os.Stderr, out.Mismatch)
		return proposal{}, false
	}
	if out.Answer.Source != dispatch.SourceAdvisor {
		return proposal{}, false
	}
	return proposalOf(out.Answer), true
}

// runChosen performs the routine somebody just confirmed, by its identity.
//
// Through the one intake, as everything else is, and through the arm that takes an identity
// rather than words: see internal/invoke's explicit-identity arm, which exists because a display
// name is derived from a slug and a slug is not a general inverse of the name, so a second lookup
// can land somewhere else entirely.
func runChosen(d orchestrator.Deps, p proposal) {
	out, err := runInvocation(d, requestFor(p))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	announce(out)
}

// requestFor is the invocation a confirmed proposal becomes.
//
// Its own function so it can be asserted without performing anything:  presses for
// real, so the one thing worth getting right here — that what travels is the ROUTE and not the
// words — cannot be checked by running it. The words would be resolved again by the door, which
// with two routines of one name is how a confirmation comes to perform a different routine from
// the one it described.
func requestFor(p proposal) invoke.Request {
	return invoke.Request{Play: &p.Route, Source: invoke.SourceCLI}
}
