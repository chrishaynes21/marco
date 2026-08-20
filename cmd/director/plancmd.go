package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// `director plan` — what would this request actually do?
//
//	director plan "focus the search box, clear it, type Director and press enter"
//	director plan --json
//
// It plans, validates and reports — and executes NOTHING. That is the whole point: a
// request that becomes four desktop operations deserves to be readable before it runs,
// and a preview that ran the thing it was previewing would be useless at the one
// moment you most want it.
//
// It also does not observe or resolve. Resolution needs a live world and would make
// the answer depend on whatever happened to be in front; this command answers the
// question that does not depend on that — what the Director understood, in what order,
// and whether it will accept it at all.
type planReport struct {
	Goal string `json:"goal"`
	// Steps are what the request decomposed into, in order.
	Steps []planStep `json:"steps"`
	// Executable is whether the whole program passed validation. False means the
	// request is REJECTED — no part of it would run.
	Executable bool `json:"executable"`
	// Reason explains a rejection.
	Reason string `json:"reason,omitempty"`
}

type planStep struct {
	Index int    `json:"index"`
	ID    string `json:"id"`
	// Phrase is the user's own words for this step.
	Phrase string `json:"phrase"`
	Verb   string `json:"verb"`
	Target string `json:"target,omitempty"`
	// Capability is the Marco capability this step will lower to, when it is known
	// without resolving a target.
	Capability string `json:"capability,omitempty"`
	// Verification is what the step must prove.
	Verification string `json:"verification"`
	// OnFailure is always "stop" — stated rather than assumed, because a reader
	// deciding whether to run a four-step program needs to know it will not carry on
	// past a failure.
	OnFailure string `json:"on_failure"`
}

func runPlan(args []string) int {
	fs := flag.NewFlagSet("plan", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print the plan as JSON")
	_ = fs.Parse(flagsFirst(args))

	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, `director plan: needs a request, e.g.`)
		fmt.Fprintln(os.Stderr, `  director plan "type Marco and press enter"`)
		return 2
	}
	request := strings.Join(rest, " ")

	report := buildPlan(request)
	if *jsonOut {
		return printJSON(report)
	}
	fmt.Print(renderPlan(report))
	if !report.Executable {
		return 1
	}
	return 0
}

// buildPlan decomposes and validates, without touching the desktop.
func buildPlan(request string) planReport {
	parse := intent.New().Parse
	prog, err := program.Decompose(request, parse)

	report := planReport{Goal: request, Executable: err == nil}
	if err != nil {
		report.Reason = err.Error()
	}
	for i, s := range prog.Steps {
		report.Steps = append(report.Steps, planStep{
			Index:        i + 1,
			ID:           string(s.ID),
			Phrase:       s.Phrase,
			Verb:         s.Operation.Verb,
			Target:       targetOf(s.Operation),
			Capability:   capabilityFor(s.Operation),
			Verification: string(s.Verification),
			OnFailure:    string(s.FailurePolicy),
		})
	}
	// A rejected request that produced no steps at all still needs its reason shown,
	// so the caller learns WHY rather than seeing an empty plan.
	if len(report.Steps) == 0 && report.Reason == "" {
		report.Reason = "the request produced no steps"
		report.Executable = false
	}
	return report
}

func targetOf(in directorapi.Intent) string {
	if len(in.Targets) == 0 {
		return ""
	}
	return in.Targets[0].Phrase
}

// capabilityFor names the Marco capability a step will use, where that is knowable
// without a live world.
//
// Empty where it is not. A click's capability is `OS's Click`, but an edit's depends
// on what the control turns out to support — the strategy ladder decides at execution
// against a real control, and printing a guess here would be inventing an answer the
// Director does not have yet.
func capabilityFor(in directorapi.Intent) string {
	switch in.Verb {
	case "click":
		return "OS's Click"
	case "focus":
		return "Accessibility's Focus"
	case "move_window":
		return "OS's MoveWindow"
	case "edit":
		op, _ := in.Parameters[intent.EditOperation].(string)
		switch op {
		case "press_enter", "select_all", "copy_selection", "paste_clipboard", "undo", "redo":
			return "OS's Key"
		case "set_text", "clear_text", "append_text":
			return "Accessibility's SetValue, or OS's Type if the control refuses"
		}
		return "chosen at execution from the control's capabilities"
	case "wait":
		return "none — waits are evaluated by the Director, not by Marco"
	}
	return ""
}

// renderPlan draws the plan for a person.
func renderPlan(r planReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Goal: %s\n\n", r.Goal)

	if len(r.Steps) == 0 {
		fmt.Fprintf(&b, "REJECTED: %s\n", r.Reason)
		return b.String()
	}
	for _, s := range r.Steps {
		fmt.Fprintf(&b, "[%d/%d] %s", s.Index, len(r.Steps), s.Verb)
		if s.Target != "" {
			fmt.Fprintf(&b, " %q", s.Target)
		}
		b.WriteString("\n")
		fmt.Fprintf(&b, "        phrase        %s\n", s.Phrase)
		if s.Capability != "" {
			fmt.Fprintf(&b, "        marco         %s\n", s.Capability)
		}
		fmt.Fprintf(&b, "        verification  %s\n", s.Verification)
		fmt.Fprintf(&b, "        on failure    %s\n", s.OnFailure)
	}
	b.WriteString("\n")
	if r.Executable {
		fmt.Fprintf(&b, "Executable: yes (%d steps, each independently verified)\n", len(r.Steps))
	} else {
		fmt.Fprintf(&b, "Executable: NO — the whole request is rejected and none of it would run\n")
		fmt.Fprintf(&b, "  %s\n", r.Reason)
	}
	return b.String()
}
