package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// `director perform "<outcome>"` — carry out something Marco has learned.
//
// The only command in this binary whose ordinary outcome is real input driven by durable knowledge
// rather than by a request typed for one occasion. It is separate from `reach`, which plans and
// performs nothing, because a surface that could act while somebody was only asking a question is
// the failure ADR-029 exists to prevent.
func runPerform(args []string) int {
	name, rest := "", args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		name, rest = args[0], args[1:]
	}
	fs := flag.NewFlagSet("perform", flag.ExitOnError)
	application := fs.String("application", "", "which application's outcome to carry out")
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(rest))
	if strings.TrimSpace(name) == "" {
		fmt.Fprintln(os.Stderr, `director: say what to do, e.g. director perform "Open Mouse Settings"`)
		return 2
	}

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	raw, err := client.Observation(service.ObserveQuery{
		Perform: &service.PerformQuery{Name: name, Application: *application},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	var v service.PerformView
	if err := json.Unmarshal(raw, &v); err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		out, _ := json.MarshalIndent(v, "", "  ")
		fmt.Println(string(out))
		return 0
	}

	// WHAT THE WORDS COULD HAVE MEANT, when Marco refused to guess between them.
	//
	// Before the steps, because there are none: this refusal happens before anything moves,
	// and the person needs the choice rather than an explanation of a walk that did not
	// happen.
	for _, c := range v.Candidates {
		fmt.Printf("  %q in %s\n", c.Name, c.Application)
	}
	// WHAT HAPPENED, step by step, because a route that got half way is a different fact
	// from one that never started.
	for i, s := range v.Steps {
		mark := "verified"
		if !s.Verified {
			mark = s.Refusal
			if mark == "" {
				mark = "did not verify"
			}
		}
		fmt.Printf("  step %d: %s\n", i+1, mark)
		if s.Detail != "" {
			fmt.Printf("      %s\n", s.Detail)
		}
	}
	if v.Say != "" {
		fmt.Println(v.Say)
	}
	if v.Refusal != "" && v.Say == "" {
		fmt.Println(v.Refusal)
	}
	if !v.Arrived && v.Refusal != "" {
		return 1
	}
	return 0
}
