package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// `director clarify "<answer>"` — answer the question a running command is waiting on.
//
// # Why this did not exist, and what that cost
//
// `Client.Clarify` was written, and nothing called it. Marco could ask "create a folder —
// what should it be called?" and there was no way to answer from a terminal: an ordinary
// `director execute "Reports"` SUPERSEDES the question by design — a new request means the
// person moved on — so the answer became a request to find a control called Reports.
//
// Measured live while testing the missing-argument clarification this exists for. The
// question was right, the sentence was right, and the conversation had no second half.
//
// Its own command on its own connection, for the same reason `confirm` is: the connection
// that submitted the request is busy reading that request's events, and the request cannot
// finish until the answer arrives.
//
// With no argument it PRINTS the question and answers nothing. A clarification is less
// dangerous to answer by accident than a confirmation — the worst case is a folder with an
// odd name rather than a deletion — but the symmetry is worth more than the keystroke.

func runClarify(args []string) int {
	fs := flag.NewFlagSet("clarify", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	c, err := connect(false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	defer c.Close()

	status, err := c.Status()
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	ask := status.Clarification
	if ask == nil {
		if *jsonOut {
			return printJSON(map[string]any{"pending": false})
		}
		fmt.Println("No question is waiting for an answer.")
		return 1
	}

	answer := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if answer == "" {
		if *jsonOut {
			return printJSON(ask)
		}
		fmt.Print(renderClarification(*ask))
		fmt.Println("Answer with `director clarify \"<your answer>\"`.")
		return 0
	}

	res, err := c.Clarify(ask.CommandID, answer, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(res)
	}
	// The answer resumes the ORIGINAL request, so what comes back is that request's
	// outcome rather than an acknowledgement — printed the way `execute` prints it, so
	// the two read alike.
	renderOutcome(res)
	if res.State == "" {
		// A clarification that produced a further question rather than an outcome.
		if again, err := c.Status(); err == nil && again.Clarification != nil {
			fmt.Print(renderClarification(*again.Clarification))
		}
	}
	return 0
}
