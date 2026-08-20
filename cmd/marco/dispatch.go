package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
)

// runDispatch is the machine seam a front-end (the overlay) uses to hold a
// conversation: `marco dispatch "<phrase>" [--json]` classifies the phrase into a
// single ROUTE-LEVEL decision (run / teach / chat / clarify) using the deterministic
// matcher plus, when configured, the local-LLM Advisor. It does NOT act — the caller
// decides whether to run/teach/speak — so it's safe to call speculatively.
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
	d := newDeps()
	dec := dispatch.New(dispatch.Default()).Decide(context.Background(), phrase, d.Reg.Slugs(), appOf(d))
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
	case dispatch.IntentTeach:
		fmt.Printf("teach: %s\n", dec.Name)
	case dispatch.IntentChat, dispatch.IntentClarify:
		fmt.Println(dec.Reply)
	default:
		fmt.Println("(nothing to do)")
	}
}

// converseTurn handles one line in the interactive assistant using dispatch's
// Advisor (the local-LLM brain). It returns true when it fully handled the line
// (ran, taught, or replied), false when there's no brain wired or it had nothing
// confident — in which case the caller falls back to its own deterministic
// fuzzy-confirm flow (including any legacy $MARCO_RESOLVER). So the assistant
// behaves exactly as before unless a converse-capable brain is configured.
func converseTurn(d orchestrator.Deps, line string) bool {
	proposal, ok := dispatch.New(dispatch.Default()).Propose(context.Background(), line, d.Reg.Slugs(), appOf(d))
	if !ok {
		return false
	}
	switch proposal.Intent {
	case dispatch.IntentRun:
		if proposal.Reply != "" {
			fmt.Println(proposal.Reply)
		}
		runDo(d, proposal.Route)
		return true
	case dispatch.IntentTeach:
		// Offer to learn it, then hand off to the normal teach-by-demonstration flow
		// under the suggested name.
		if proposal.Reply != "" {
			fmt.Println(proposal.Reply)
		}
		if askYes(fmt.Sprintf("Teach %q now? [y]es / [n]o: ", proposal.Name)) {
			if err := d.Teach(proposal.Name); err != nil {
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
