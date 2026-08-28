package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// `director reach "open mouse settings"` — the goal-centric question, asked out loud.
//
//	director reach                      # what outcomes has Marco learned here
//	director reach "open mouse settings"  # how would it get there from where I am
//
// A read. The plan is over VERIFIED edges — each one earned by its own rehearsal under its
// own yes — and performing any of it still goes through a saved play's ordinary
// resolve → authorize → run. Nothing here acts, and nothing here can.
func runReach(args []string) int {
	name, rest := "", args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, rest = args[0], args[1:]
	}
	fs := flag.NewFlagSet("reach", flag.ExitOnError)
	application := fs.String("application", "", "which application's outcomes to ask about")
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(rest))

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	raw, err := client.Observation(service.ObserveQuery{
		Reach: &service.ObserveReach{Name: name, Application: *application},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	var v service.ReachView
	if err := json.Unmarshal(raw, &v); err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(v)
	}

	if name == "" {
		if len(v.Outcomes) == 0 {
			fmt.Println(v.Say)
			return 0
		}
		fmt.Printf("Learned outcomes in %s:\n", v.Application)
		for _, o := range v.Outcomes {
			fmt.Printf("  %q — shown %d time(s)\n", o.Name, o.Demonstrations)
		}
		return 0
	}

	fmt.Println(v.Say)
	// WHAT THE WORDS COULD EQUALLY HAVE MEANT, when they meant more than one thing.
	//
	// The list, not just the sentence: somebody choosing between two outcomes needs to see
	// both, and the whole reason `perform` refuses here is that nobody should be guessing.
	for _, c := range v.Candidates {
		fmt.Printf("  %q in %s\n", c.Name, c.Application)
	}
	// AND THE WHOLE CHAIN, from words to route, because that is what this command is for.
	//
	// A person debugging "why did it go there" needs each hop named: which phrase matched,
	// which outcome that is, which screen the outcome IS, and where Marco thinks they are
	// standing. Any one of those being wrong looks identical from the outside.
	if v.Subject != "" {
		fmt.Printf("\n  %q → %s\n", v.Name, v.Subject)
		if v.Current != "" {
			fmt.Printf("  from %s\n", v.Current)
		} else {
			fmt.Println("  from nowhere Marco has resolved yet")
		}
	}
	if len(v.Steps) > 0 {
		for i, s := range v.Steps {
			fmt.Printf("  step %d: %s → %s\n", i+1, s.From, s.To)
		}
		fmt.Println("\nPerforming a step still goes through its saved play — reach shows, " +
			"it never does.")
	}
	if v.AsOf != "" {
		fmt.Printf("\n(from where you were last seen standing, session %s)\n", v.AsOf)
	}
	return 0
}
