package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// `director confirm [yes|no]` — answer the question a running command is waiting on.
//
//	nil confirmer → unavailable, and no action may execute after it.
//
// A command that needs agreement BLOCKS until someone answers, and until this existed
// nobody could. It is deliberately its own command on its own connection: the connection
// that submitted the request is busy reading that request's events, and the request cannot
// finish until the answer arrives.
//
// With no argument it PRINTS the question and answers nothing, which is the safe default
// for a command whose accidental invocation would otherwise agree to a deletion.

func runConfirm(args []string) int {
	fs := flag.NewFlagSet("confirm", flag.ExitOnError)
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
	ask := status.Confirmation
	if ask == nil {
		if *jsonOut {
			return printJSON(map[string]any{"pending": false})
		}
		fmt.Println("Nothing is waiting to be confirmed.")
		return 1
	}

	answer := strings.ToLower(strings.TrimSpace(strings.Join(fs.Args(), " ")))
	if answer == "" {
		// Show, do not decide. An invocation with no answer must never become a yes.
		if *jsonOut {
			return printJSON(ask)
		}
		fmt.Print(renderConfirmation(*ask))
		fmt.Println("\nAnswer with `director confirm yes` or `director confirm no`.")
		return 0
	}

	approved, ok := readAnswer(answer)
	if !ok {
		fmt.Fprintf(os.Stderr, "director: %q is not an answer. Use yes or no.\n", answer)
		return 2
	}
	res, err := c.Confirm(ask.ID, approved)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(res)
	}
	if !res.Accepted {
		fmt.Fprintf(os.Stderr, "director: %s\n", res.Message)
		return 1
	}
	fmt.Printf("%s — %s\n", res.Message, ask.Question())
	return 0
}

// readAnswer maps a word onto a decision.
//
// A CLOSED list, and anything outside it is refused rather than guessed at. "y" is a yes
// and "yeah" is not, because the cost of over-reading a word here is agreeing to something
// irreversible on the user's behalf.
func readAnswer(s string) (bool, bool) {
	switch s {
	case "yes", "y", "ok", "confirm", "approve":
		return true, true
	case "no", "n", "cancel", "deny", "reject":
		return false, true
	}
	return false, false
}

// renderConfirmation draws the question in full.
func renderConfirmation(c service.ConfirmationPayload) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Waiting for your answer (%s):\n\n  %s\n\n", c.ID, c.Question())
	if c.Request != "" {
		fmt.Fprintf(&b, "  you asked    %s\n", c.Request)
	}
	if c.Resource != "" {
		// The backing identity, not the caption. "Budget.txt" is ambiguous between four
		// folders; a path is not.
		fmt.Fprintf(&b, "  object       %s\n", c.Resource)
	} else if c.Target != "" {
		fmt.Fprintf(&b, "  object       %s\n", c.Target)
	}
	if c.Risk != "" {
		fmt.Fprintf(&b, "  risk         %s\n", c.Risk)
	}
	if c.Reason != "" {
		fmt.Fprintf(&b, "  because      %s\n", c.Reason)
	}
	if c.Goal != "" || c.Procedure != "" {
		fmt.Fprintf(&b, "  from         %s", c.Goal)
		if c.Procedure != "" {
			fmt.Fprintf(&b, " via %s", c.Procedure)
		}
		if c.StepCount > 0 {
			fmt.Fprintf(&b, " (step %d of %d)", c.StepIndex, c.StepCount)
		}
		b.WriteString("\n")
	}
	for i, s := range c.Steps {
		fmt.Fprintf(&b, "  %d. %s\n", i+1, s)
	}
	if c.TargetChanged {
		fmt.Fprintf(&b, "  CHANGED      %s\n", strings.Join(c.Changes, "; "))
	}
	if c.ReplayOf != "" {
		fmt.Fprintf(&b, "  repeat of    %s", c.ReplayOf)
		if c.StoredConfirmation != "" {
			// Disclosed, and explicitly not treated as permission.
			fmt.Fprintf(&b, " (which was %s at the time — that is a record, not consent)",
				c.StoredConfirmation)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "  expires      %s\n", c.ExpiresAt.Format("15:04:05"))
	return b.String()
}
