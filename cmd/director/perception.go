package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/diagnostics"
)

// The perception diagnostics.
//
//	director observations   who observed, and what they produced
//	director fusion         what fusion made of it
//
// Both read the RUNNING service rather than observing afresh. That matters more than
// it looks: a diagnostic that took its own snapshot would report on a cycle nothing
// ever planned against, and — worse — would attach a second accessibility client to
// the desktop to do it, which is exactly the cold-tree problem the long-lived service
// exists to solve. What these show is the evidence the Director actually acted on.

// runObservations reports the providers and the recent observation cycles.
func runObservations(args []string) int {
	fs := flag.NewFlagSet("observations", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	start := fs.Bool("start", false, "start the service if it is not running")
	_ = fs.Parse(args)

	p, code := explanation(*start, *jsonOut)
	if code != 0 {
		return code
	}
	if *jsonOut {
		return printJSON(p)
	}
	fmt.Print(diagnostics.RenderObservations(p))
	return 0
}

// runFusion reports what fusion made of the most recent cycle.
func runFusion(args []string) int {
	fs := flag.NewFlagSet("fusion", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	start := fs.Bool("start", false, "start the service if it is not running")
	_ = fs.Parse(args)

	p, code := explanation(*start, *jsonOut)
	if code != 0 {
		return code
	}
	if *jsonOut {
		return printJSON(p.Fusion)
	}
	fmt.Print(diagnostics.RenderFusion(p))
	return 0
}

// perception fetches the diagnostic picture from the service.
func perception(start, jsonOut bool) (diagnostics.Perception, int) {
	c, err := connect(start)
	if err != nil {
		if jsonOut {
			fmt.Println(`{"running":false}`)
		} else {
			fmt.Println("Director: not running")
			fmt.Println("  start it with: director serve")
		}
		return diagnostics.Perception{}, 1
	}
	defer c.Close()

	p, err := c.Perception()
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return diagnostics.Perception{}, 1
	}
	if len(p.Cycles) == 0 && !jsonOut {
		// A service that has not observed yet is a real and confusing state: it is
		// running, it answers, and it has nothing to say. Saying so beats printing a
		// page of zeroes.
		fmt.Println("The Director has not observed anything yet.")
		fmt.Println("  run a command first, or: director status --start")
	}
	return p, 0
}
