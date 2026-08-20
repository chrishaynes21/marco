package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
)

// `director edit` — how was the text actually changed?
//
//	director edit
//	director edit --json
//
// The question this answers is "why did it type instead of setting the value?", and
// the answer has to come from a record rather than from reasoning about the code. An
// autocomplete that fired, a field that truncated, a clipboard that could not be
// given back — all of them look like "the edit worked" from the outside, and all of
// them are visible here.
func runEdit(args []string) int {
	fs := flag.NewFlagSet("edit", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print the outcomes as JSON")
	start := fs.Bool("start", false, "start the service if it is not running")
	limit := fs.Int("n", 10, "how many recent edits to show")
	_ = fs.Parse(flagsFirst(args))

	c, err := connect(*start)
	if err != nil {
		if *jsonOut {
			fmt.Println("[]")
		} else {
			fmt.Println("Director: not running")
		}
		return 1
	}
	defer c.Close()

	outcomes, err := c.Edits()
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *limit > 0 && len(outcomes) > *limit {
		outcomes = outcomes[len(outcomes)-*limit:]
	}
	if *jsonOut {
		return printJSON(outcomes)
	}
	fmt.Print(renderEdits(outcomes))
	return 0
}

// renderEdits describes what each edit did and why.
func renderEdits(outcomes []edit.Outcome) string {
	var b strings.Builder
	if len(outcomes) == 0 {
		b.WriteString("No text has been edited yet.\n")
		return b.String()
	}

	for i, o := range outcomes {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s — %s\n", o.Operation, o.Description)
		fmt.Fprintf(&b, "  strategy   %s\n", o.Strategy)

		// Before and After are the whole basis of the verdict, so they are shown even
		// when they are empty — and "unreadable" is shown differently from empty,
		// because only one of them proves anything.
		fmt.Fprintf(&b, "  before     %s\n", showValue(o.Before, o.BeforeKnown))
		fmt.Fprintf(&b, "  after      %s\n", showValue(o.After, o.AfterKnown))

		verdict := "NOT VERIFIED"
		if o.Verified {
			verdict = "verified"
		}
		fmt.Fprintf(&b, "  %s: %s\n", verdict, o.Evidence)

		// Every rung that was skipped or refused, in the order it was considered.
		// This is the part that makes a fallback explainable rather than mysterious.
		if len(o.Attempts) > 0 {
			b.WriteString("  attempts\n")
			for _, a := range o.Attempts {
				mark := " "
				if a.Chosen {
					mark = "*"
				}
				line := fmt.Sprintf("    %s %-14s %s", mark, a.Strategy, a.Reason)
				if a.Err != "" {
					line += " — " + a.Err
				}
				b.WriteString(strings.TrimRight(line, " ") + "\n")
			}
		}
		if o.ClipboardBorrowed {
			if o.ClipboardRestored {
				b.WriteString("  clipboard  borrowed and restored\n")
			} else {
				b.WriteString("  clipboard  BORROWED AND NOT RESTORED — the user's clipboard is not what they left it as\n")
			}
		}
		if o.Error != "" {
			fmt.Fprintf(&b, "  error      %s\n", o.Error)
		}
	}
	return b.String()
}

// showValue renders a field's contents, keeping "empty" distinct from "unreadable".
func showValue(v string, known bool) string {
	if !known {
		return "(could not be read)"
	}
	if v == "" {
		return "(empty)"
	}
	if len(v) > 70 {
		return fmt.Sprintf("%q (%d characters)", v[:67]+"...", len(v))
	}
	return fmt.Sprintf("%q", v)
}
