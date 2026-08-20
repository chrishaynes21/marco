package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
)

// `director goal --dry-run "<request>"` — everything that would happen, and nothing that
// would happen.
//
//	Dry-run must not focus windows or perform UI input.
//
// It is a pure function of the request and the registry: it parses, selects, expands and
// validates, and it constructs no accessibility bridge, opens no window and sends no
// input. That is enforceable by inspection here — nothing in this file reaches a
// provider — and it is what makes the command safe to run against a request you would
// not dare execute.

// DryRun is the full account, for --json.
type DryRun struct {
	Request    string           `json:"request"`
	Normalized string           `json:"normalized_request"`
	Goal       *goal.Goal       `json:"goal,omitempty"`
	Selection  goal.Selection   `json:"selection"`
	Procedure  string           `json:"procedure,omitempty"`
	Safety     *goal.Safety     `json:"safety,omitempty"`
	Missing    []string         `json:"unresolved_preconditions,omitempty"`
	Program    *program.Program `json:"program,omitempty"`
	Validation string           `json:"validation"`
	// Confirmations is what the user would be asked before anything ran.
	Confirmations []string `json:"expected_confirmations,omitempty"`
	// Deictic reports how each "this"/"that" would be bound.
	Deictic []string `json:"deictic_bindings,omitempty"`
	// Bindings is the per-step account of what would have to be resolved: the phrase,
	// the kind it must be, and what happens when it is not.
	Bindings []PlannedBinding `json:"bindings,omitempty"`
	// Revalidation says when each binding would be re-checked.
	Revalidation string `json:"revalidation_point,omitempty"`
	// ReplayPolicy says what repeating this would need.
	ReplayPolicy string `json:"replay_policy,omitempty"`
	// Provenance is what would be stamped on every node this program produced.
	Provenance *actiongraph.GoalProvenance `json:"provenance,omitempty"`
	Refused    string                      `json:"refused,omitempty"`
}

func runDryRun(args []string) int {
	fs := flag.NewFlagSet("goal", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	app := fs.String("app", "", "the application to expand for")
	_ = fs.Bool("dry-run", true, "expand without running (always true for this command)")
	_ = fs.Parse(flagsFirst(args))

	request := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if request == "" {
		fmt.Fprintln(os.Stderr,
			`goal needs a request — try: director goal --dry-run "rename this file to Budget"`)
		return 2
	}

	dry := DryRun{Request: request, Normalized: normalizeRequest(request)}
	r, code := registry()
	if code != 0 {
		return code
	}

	g, ok := goal.Parse(request)
	if !ok {
		dry.Refused = "not a goal; this would be handled as an ordinary request"
		if *jsonOut {
			return printJSON(dry)
		}
		fmt.Printf("Request:\n  %s\n\nNot a goal.\n  %s\n", request, dry.Refused)
		return 1
	}
	g.Context.Application = *app
	dry.Goal = &g

	proc, selection, chosen := r.SelectProcedure(g)
	dry.Selection = selection
	if !chosen {
		dry.Refused = selection.Reason
		if *jsonOut {
			return printJSON(dry)
		}
		fmt.Print(renderDryRun(dry))
		return 1
	}
	dry.Procedure = proc.Name
	safety := proc.Safety
	dry.Safety = &safety

	for _, m := range proc.Missing(g) {
		dry.Missing = append(dry.Missing, string(m)+" — "+m.Describe())
	}
	if safety.RequiresConfirmation {
		dry.Confirmations = append(dry.Confirmations, fmt.Sprintf(
			"%s (%s risk) would be confirmed before step 1", proc.Name, safety.Risk))
	}

	// Plan, not Expand: nothing is observed and nothing is bound, so the program it
	// produces is refused by ValidateBound and cannot be run by accident. That is the
	// whole safety property of this command, and it is structural rather than a promise.
	ex, err := goal.Plan(r, g)
	if err != nil {
		dry.Validation = "refused: " + err.Error()
		dry.Refused = err.Error()
		if *jsonOut {
			return printJSON(dry)
		}
		fmt.Print(renderDryRun(dry))
		return 1
	}
	prog := ex.Program
	dry.Program = &prog
	dry.Validation = "ok — the expansion is a well-formed program"
	if ex.Deictic {
		// Said plainly, because the difference matters: this program is NOT runnable as
		// it stands, and the reason is the safety property rather than a fault.
		dry.Validation = "ok as a plan — but this request points at something, so it " +
			"cannot run until that has been resolved against the screen. " +
			"A dry run does not observe, so nothing was resolved here."
		dry.Bindings = plannedBindings(ex)
		dry.Revalidation = "each binding would be re-checked immediately before its step " +
			"acts, after the plan is built and before any confirmation is put"
	}
	dry.ReplayPolicy = replayPolicyFor(*dry.Safety)
	dry.Provenance = &actiongraph.GoalProvenance{
		Goal: string(ex.Goal.Kind), Procedure: ex.Procedure, Request: request,
		ProcedureGeneric: ex.Generic, Application: ex.Application,
		StepCount: len(prog.Steps),
	}
	dry.Deictic = deicticBindings(ex)

	if *jsonOut {
		return printJSON(dry)
	}
	fmt.Print(renderDryRun(dry))
	return 0
}

// normalizeRequest shows what the parser actually reads.
//
// Worth printing because the difference between what was typed and what was matched is
// where a surprising expansion usually starts.
func normalizeRequest(s string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(s), " .!?,"))
}

// PlannedBinding is what one deictic reference would have to resolve to.
type PlannedBinding struct {
	Step         int    `json:"step"`
	Phrase       string `json:"phrase"`
	Reference    string `json:"reference"`
	ExpectedKind string `json:"expected_kind"`
	Resolved     bool   `json:"resolved"`
	Note         string `json:"note"`
}

// deicticBindings reports how each pointed-at target would resolve, in one line each.
//
// It says what the binding WOULD be, never what it is: resolving one for real needs a
// world, and observing one is exactly what a dry run must not do.
func deicticBindings(ex goal.Expansion) []string {
	var out []string
	for _, pb := range plannedBindings(ex) {
		out = append(out, fmt.Sprintf("step %d (%s): %q → %s. %s",
			pb.Step, pb.Phrase, pb.Reference, pb.ExpectedKind, pb.Note))
	}
	return out
}

// plannedBindings is the structured form.
func plannedBindings(ex goal.Expansion) []PlannedBinding {
	var out []PlannedBinding
	for i, s := range ex.Program.Steps {
		for _, ref := range s.Operation.Targets {
			if !ref.RequiresBinding {
				continue
			}
			pb := PlannedBinding{
				Step: i + 1, Phrase: s.Phrase, Reference: ref.Phrase,
				ExpectedKind: ref.ExpectedKind, Resolved: ref.BindingID != "",
			}
			pb.Note = "must resolve to exactly one such object, with a backing resource " +
				"that identifies it; a wrong kind or an unidentifiable object is refused, " +
				"and several candidates is a question"
			if !pb.Resolved {
				pb.Note = "NOT resolved here — a dry run does not observe the screen. " + pb.Note
			}
			out = append(out, pb)
		}
	}
	return out
}

// replayPolicyFor says what repeating this program would require.
func replayPolicyFor(s goal.Safety) string {
	if s.Destructive || s.Irreversible || s.External {
		return "a repeat would be confirmed again. The original run's confirmation is a " +
			"record of what was agreed to then, not permission to do it again, and a " +
			"repeat whose recorded object cannot be re-identified stops rather than " +
			"binding to whatever has focus."
	}
	return "a repeat needs no renewed confirmation beyond whatever policy asks at the " +
		"time. A repeat whose recorded object cannot be re-identified still stops."
}

// renderDryRun draws the whole account.
func renderDryRun(d DryRun) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Request:\n  %s\n", d.Request)
	if d.Normalized != d.Request {
		fmt.Fprintf(&b, "  normalized: %s\n", d.Normalized)
	}

	if d.Goal != nil {
		fmt.Fprintf(&b, "\nGoal:\n  %s\n", d.Goal.Kind)
		for name, value := range d.Goal.Parameters {
			fmt.Fprintf(&b, "  %-12s %s\n", name, value)
		}
		if d.Goal.Context.TargetIsImplicit {
			b.WriteString("  target       (pointed at, not named)\n")
		} else if d.Goal.Context.Target != "" {
			fmt.Fprintf(&b, "  target       %s\n", d.Goal.Context.Target)
		}
	}

	if len(d.Selection.Candidates) > 0 {
		b.WriteString("\nCompeting procedures:\n")
		for _, c := range d.Selection.Candidates {
			marker := " "
			if c.Procedure == d.Selection.Chosen {
				marker = "*"
			}
			fmt.Fprintf(&b, "  %s %-30s %s\n", marker, c.Procedure, c.Why)
		}
		if d.Selection.Ambiguous {
			fmt.Fprintf(&b, "\n  AMBIGUOUS: %s\n", d.Selection.Reason)
		}
	}

	if d.Safety != nil {
		fmt.Fprintf(&b, "\nDeclared safety:\n  risk %s, %d mutation(s)",
			d.Safety.Risk, d.Safety.Mutations)
		var flags []string
		if d.Safety.Destructive {
			flags = append(flags, "destructive")
		}
		if d.Safety.Irreversible {
			flags = append(flags, "irreversible")
		}
		if d.Safety.External {
			flags = append(flags, "external")
		}
		if len(flags) > 0 {
			fmt.Fprintf(&b, " — %s", strings.Join(flags, ", "))
		}
		b.WriteString("\n")
	}

	if len(d.Missing) > 0 {
		b.WriteString("\nUnresolved preconditions:\n")
		for _, m := range d.Missing {
			fmt.Fprintf(&b, "  %s\n", m)
		}
	}
	if len(d.Confirmations) > 0 {
		b.WriteString("\nExpected confirmations:\n")
		for _, c := range d.Confirmations {
			fmt.Fprintf(&b, "  %s\n", c)
		}
	}

	if d.Program != nil {
		b.WriteString("\nExpanded program:\n")
		for i, s := range d.Program.Steps {
			fmt.Fprintf(&b, "  %d. %-42s %-8s %s\n", i+1, s.Phrase,
				s.Operation.Verb, s.Verification)
			for _, pre := range s.Preconditions {
				fmt.Fprintf(&b, "     waits until: %s\n", pre.Describe())
			}
		}
	}
	fmt.Fprintf(&b, "\nValidation:\n  %s\n", firstNonBlank(d.Validation, "not reached"))

	if len(d.Bindings) > 0 {
		b.WriteString("\nDeictic bindings:\n")
		for _, pb := range d.Bindings {
			state := "NOT RESOLVED"
			if pb.Resolved {
				state = "resolved"
			}
			fmt.Fprintf(&b, "  step %d  %q must be %s [%s]\n", pb.Step, pb.Reference,
				pb.ExpectedKind, state)
			fmt.Fprintf(&b, "          %s\n", pb.Note)
		}
		if d.Revalidation != "" {
			fmt.Fprintf(&b, "\n  Re-checked: %s\n", d.Revalidation)
		}
	} else if len(d.Deictic) > 0 {
		b.WriteString("\nDeictic bindings:\n")
		for _, x := range d.Deictic {
			fmt.Fprintf(&b, "  %s\n", x)
		}
	}
	if d.ReplayPolicy != "" {
		fmt.Fprintf(&b, "\nIf repeated:\n  %s\n", d.ReplayPolicy)
	}
	if d.Provenance != nil {
		b.WriteString("\nProvenance that would be attached to each node:\n")
		fmt.Fprintf(&b, "  goal       %s\n", d.Provenance.Goal)
		fmt.Fprintf(&b, "  procedure  %s\n", d.Provenance.Procedure)
		fmt.Fprintf(&b, "  request    %s\n", d.Provenance.Request)
		fmt.Fprintf(&b, "  steps      %d\n", d.Provenance.StepCount)
	}
	if d.Refused != "" {
		fmt.Fprintf(&b, "\nRefused:\n  %s\n", d.Refused)
	}

	b.WriteString("\nNothing was run, focused, or observed: this is a dry run.\n")
	return b.String()
}
