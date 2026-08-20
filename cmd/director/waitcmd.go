package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	waitengine "github.com/chaynes-simpleclouds/marco/internal/director/wait/engine"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/evaluation"
)

// `director wait` — what is the Director waiting FOR?
//
//	director wait
//	director wait --json
//	director wait --follow
//
// A wait can run for seconds during which the Director looks busy and says nothing.
// That is exactly when someone wants to know whether it is making progress. "Waiting
// for the dialog, 14 looks, still unsatisfied" is a considered pause; "14 looks, all
// unknown" is a perception problem wearing the same clothes.
func runWait(args []string) int {
	fs := flag.NewFlagSet("wait", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print the wait as JSON")
	follow := fs.Bool("follow", false, "poll until the wait ends")
	start := fs.Bool("start", false, "start the service if it is not running")
	_ = fs.Parse(flagsFirst(args))

	c, err := connect(*start)
	if err != nil {
		if *jsonOut {
			fmt.Println(`{"waiting":false,"running":false}`)
		} else {
			fmt.Println("Director: not running")
		}
		return 1
	}
	defer c.Close()

	for {
		snap, err := c.ActiveWait()
		if err != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", err)
			return 1
		}
		if *jsonOut {
			return printJSON(snap)
		}
		fmt.Print(renderWait(snap))
		if !*follow || !snap.Waiting {
			if snap.Waiting {
				return 0
			}
			return 0
		}
		// Poll the DIAGNOSTIC, which is a different thing from the wait itself: this
		// interval says how often to redraw a status line, and nothing concludes
		// anything because it elapsed.
		time.Sleep(300 * time.Millisecond)
	}
}

// renderWait describes the wait in flight.
func renderWait(s waitengine.Snapshot) string {
	var b strings.Builder

	if !s.Waiting {
		b.WriteString("The Director is not waiting for anything.\n")
		b.WriteString("\n  Waits happen inside a command — after acting, while the screen\n")
		b.WriteString("  settles. Run one in another terminal and ask again.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "Waiting %s\n", s.Description)
	fmt.Fprintf(&b, "  condition   %s\n", s.Condition)
	fmt.Fprintf(&b, "  elapsed     %s of %s\n",
		s.Elapsed.Round(time.Millisecond), s.Timeout)
	fmt.Fprintf(&b, "  looks       %d\n", s.Iterations)

	if s.Latest != nil {
		fmt.Fprintf(&b, "\n  state       %s (confidence %.2f)\n",
			s.Latest.Result.State, s.Latest.Result.Confidence)
		fmt.Fprintf(&b, "  because     %s\n", s.Latest.Result.Explanation)
		for _, ev := range s.Latest.Result.Evidence {
			fmt.Fprintf(&b, "  evidence    %s: %s\n", ev.Kind, ev.Detail)
		}
	}

	if len(s.Counts) > 0 {
		b.WriteString("\n  so far      ")
		var parts []string
		for _, st := range []evaluation.State{
			evaluation.Satisfied, evaluation.Unsatisfied, evaluation.Unknown,
		} {
			if n := s.Counts[st]; n > 0 {
				parts = append(parts, fmt.Sprintf("%d %s", n, st))
			}
		}
		b.WriteString(strings.Join(parts, ", ") + "\n")

		// The diagnosis that matters. A wait that keeps returning UNKNOWN is not slow —
		// it is blind, and no amount of further waiting will help.
		if s.Counts[evaluation.Unknown] > 0 && s.Counts[evaluation.Unsatisfied] == 0 &&
			s.Counts[evaluation.Satisfied] == 0 {
			b.WriteString("\n  Every look has been UNKNOWN: the Director cannot observe the thing\n")
			b.WriteString("  this condition asks about. That is blindness, not slowness — waiting\n")
			b.WriteString("  longer will not answer it.\n")
		}
	}

	b.WriteString("\n  The Director is not waiting because time has passed. It is waiting\n")
	b.WriteString("  because the world has not yet provided evidence that the condition holds.\n")
	return b.String()
}
