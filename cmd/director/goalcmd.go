package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
)

// `director goals` / `director procedures` / `director procedure <name>` /
// `director explain goal "<request>"`.
//
// The goal layer is the one a user cannot see at all from the outside: a request goes in
// and five steps come out, and without these commands the expansion is folklore. They
// need no service and touch no desktop — a procedure is a property of the goal, and an
// expansion is a pure function of the goal and the registry.
//
// `explain goal` in particular EXPANDS WITHOUT RUNNING. That is the point: the way to
// find out what "close without saving" will do is to ask, not to try it.

func runGoals(args []string) int {
	fs := flag.NewFlagSet("goals", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	r, code := registry()
	if code != 0 {
		return code
	}
	if *jsonOut {
		type row struct {
			Kind       string   `json:"kind"`
			Phrase     string   `json:"phrase"`
			Procedures []string `json:"procedures"`
		}
		rows := make([]row, 0, len(goal.Vocabulary))
		for _, k := range goal.Vocabulary {
			rows = append(rows, row{string(k), k.Describe(), proceduresFor(r, k)})
		}
		return printJSON(rows)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d goals. Each expands into an ordinary Director program.\n\n",
		len(goal.Vocabulary))
	fmt.Fprintf(&b, "  %-22s %-8s %s\n", "GOAL", "RISK", "PROCEDURES")
	for _, k := range goal.Vocabulary {
		risk := ""
		if p, ok := r.Select(goal.Goal{Kind: k}); ok {
			risk = string(p.Safety.Risk)
		}
		fmt.Fprintf(&b, "  %-22s %-8s %s\n", k, risk,
			strings.Join(proceduresFor(r, k), ", "))
	}
	b.WriteString("\nRun `director procedure <name>` for one procedure's steps.\n")
	fmt.Print(b.String())
	return 0
}

// proceduresFor lists the procedures serving a goal, generic first.
func proceduresFor(r *goal.Registry, k goal.Kind) []string {
	var out []string
	for _, p := range r.All() {
		if p.Goal == k {
			out = append(out, p.Name)
		}
	}
	return out
}

func runProcedures(args []string) int {
	fs := flag.NewFlagSet("procedures", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	r, code := registry()
	if code != 0 {
		return code
	}
	// A named procedure prints its full account.
	if name := strings.TrimSpace(strings.Join(fs.Args(), " ")); name != "" {
		p, ok := r.Find(name)
		if !ok {
			fmt.Fprintf(os.Stderr, "director: no procedure called %q.\n"+
				"Run `director procedures` for the list.\n", name)
			return 1
		}
		if *jsonOut {
			return printJSON(p)
		}
		fmt.Print(renderProcedure(r, p))
		return 0
	}

	all := r.All()
	if *jsonOut {
		return printJSON(all)
	}
	var b strings.Builder
	learned := 0
	for _, p := range all {
		if p.Learned {
			learned++
		}
	}
	if learned > 0 {
		fmt.Fprintf(&b, "%d procedures, %d of them learned from a demonstration.\n\n",
			len(all), learned)
	} else {
		fmt.Fprintf(&b, "%d procedures.\n\n", len(all))
	}
	fmt.Fprintf(&b, "  %-34s %-20s %-8s %s\n", "PROCEDURE", "GOAL", "RISK", "APPLIES TO")
	for _, p := range all {
		scope := "any application"
		if !p.Generic() {
			scope = strings.Join(p.Applications, ", ")
		}
		// Disclosed in the listing, not only in the name. A user about to let a procedure
		// act on their files is entitled to see where it came from without asking.
		name := p.Name
		if p.Learned {
			name += " *"
		}
		fmt.Fprintf(&b, "  %-34s %-20s %-8s %s\n", name, p.Goal, p.Safety.Risk, scope)
	}
	if learned > 0 {
		b.WriteString("\n  * learned from a demonstration — see: director explain procedure <name>\n")
	}
	fmt.Print(b.String())
	return 0
}

// renderProcedure draws one procedure in full.
func renderProcedure(r *goal.Registry, p goal.Procedure) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", p.Name)
	fmt.Fprintf(&b, "  Goal         %s\n", p.Goal)
	if p.Generic() {
		b.WriteString("  Applies to   any application with no more specific procedure\n")
	} else {
		fmt.Fprintf(&b, "  Applies to   %s\n", strings.Join(p.Applications, ", "))
	}
	if len(p.Requires) > 0 {
		reqs := make([]string, 0, len(p.Requires))
		for _, req := range p.Requires {
			reqs = append(reqs, string(req))
		}
		fmt.Fprintf(&b, "  Requires     %s\n", strings.Join(reqs, ", "))
	}

	b.WriteString("\nSafety:\n")
	fmt.Fprintf(&b, "  Risk         %s\n", p.Safety.Risk)
	fmt.Fprintf(&b, "  Mutations    %d\n", p.Safety.Mutations)
	fmt.Fprintf(&b, "  Destructive  %v\n", p.Safety.Destructive)
	fmt.Fprintf(&b, "  External     %v\n", p.Safety.External)
	fmt.Fprintf(&b, "  Irreversible %v\n", p.Safety.Irreversible)
	fmt.Fprintf(&b, "  Confirms     %v\n", p.Safety.RequiresConfirmation)

	if p.Why != "" {
		fmt.Fprintf(&b, "\nWhy these steps:\n  %s\n", p.Why)
	}

	// The steps, expanded against a filled-in example so the shape is visible. Stated
	// as an example rather than as the truth: the real expansion depends on the goal's
	// own parameters, and printing it as though it were fixed would mislead.
	example := exampleGoal(p)
	if ex, err := goal.Plan(r, example); err == nil {
		fmt.Fprintf(&b, "\nSteps, for %q:\n", example.Describe())
		for i, s := range ex.Program.Steps {
			fmt.Fprintf(&b, "  %d. %-42s %s\n", i+1, s.Phrase, s.Operation.Verb)
			for _, pre := range s.Preconditions {
				fmt.Fprintf(&b, "     waits until: %s\n", pre.Describe())
			}
		}
	}
	return b.String()
}

// exampleGoal builds a goal that satisfies a procedure's requirements, for display.
//
// Only what the procedure REQUIRES. Filling every parameter regardless produced
// "delete Report.txt to Reports to Downloads" — an example that reads as nonsense and
// teaches the reader the wrong shape of request.
func exampleGoal(p goal.Procedure) goal.Goal {
	g := goal.Goal{Kind: p.Goal, Parameters: map[string]string{}}
	for _, req := range p.Requires {
		switch req {
		case goal.RequiresTarget:
			g.Context.Target = "Report.txt"
		case goal.RequiresName:
			g.Parameters[goal.ParamName] = "Reports"
		case goal.RequiresDestination:
			g.Parameters[goal.ParamDestination] = "Downloads"
		}
	}
	if !p.Generic() {
		g.Context.Application = p.Applications[0]
	}
	return g
}

// runExplainGoal expands a request WITHOUT running it.
func runExplainGoal(args []string) int {
	fs := flag.NewFlagSet("explain goal", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	app := fs.String("app", "", "the application to expand for, for an override")
	_ = fs.Parse(flagsFirst(args))

	request := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if request == "" {
		fmt.Fprintln(os.Stderr,
			`explain goal needs a request — try: director explain goal "rename this file to Budget"`)
		return 2
	}

	g, ok := goal.Parse(request)
	if !ok {
		fmt.Printf("%q is not a goal the Director recognises.\n"+
			"It would be handled as an ordinary request instead.\n", request)
		return 1
	}
	g.Context.Application = *app

	r, code := registry()
	if code != 0 {
		return code
	}
	// Plan, not Expand: `explain goal` answers "what would this become?" without
	// observing the screen, so a deictic target is shown as what it must resolve to
	// rather than resolved. The program that comes back is unrunnable by construction.
	ex, err := goal.Plan(r, g)
	if err != nil {
		// A refusal is an ANSWER here, and the most useful one: it says what the request
		// is missing before anything has been done about it.
		if *jsonOut {
			return printJSON(map[string]any{"goal": g, "refused": true, "reason": err.Error()})
		}
		fmt.Printf("Goal:\n  %s\n\nRefused:\n  %s\n", g.Kind, err.Error())
		if refusal, isRefusal := err.(goal.Refusal); isRefusal && len(refusal.Missing) > 0 {
			fmt.Printf("\nThe Director would ask:\n  %s\n", refusal.Question())
		}
		return 1
	}
	if *jsonOut {
		return printJSON(ex)
	}
	fmt.Print(renderExpansion(ex))
	return 0
}

// renderExpansion draws the goal → procedure → program chain.
func renderExpansion(ex goal.Expansion) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Goal:\n  %s\n\n", ex.Goal.Kind)

	b.WriteString("Expanded because:\n")
	app := ex.Application
	if app == "" {
		app = "(the active application)"
	}
	fmt.Fprintf(&b, "  Application  %s\n", app)
	kind := "application override"
	if ex.Generic {
		kind = "generic"
	}
	fmt.Fprintf(&b, "  Procedure    %s (%s)\n", ex.Procedure, kind)

	b.WriteString("\nProduced:\n")
	for i, s := range ex.Program.Steps {
		fmt.Fprintf(&b, "  %d. %-42s %s\n", i+1, s.Phrase, s.Operation.Verb)
		for _, pre := range s.Preconditions {
			fmt.Fprintf(&b, "     waits until: %s\n", pre.Describe())
		}
		if s.Verification == "best_effort" {
			// Worth calling out: it is the difference between a step that must happen
			// and one that may not apply at all.
			fmt.Fprintf(&b, "     best effort: it may not apply, and the program continues either way\n")
		}
	}

	if ex.Deictic {
		// The most important line in this output for a request that points at something,
		// and the one a reader would otherwise assume the opposite of.
		b.WriteString("\nPoints at something:\n")
		for _, pb := range plannedBindings(ex) {
			fmt.Fprintf(&b, "  step %d  %q must be %s — NOT resolved here\n",
				pb.Step, pb.Reference, pb.ExpectedKind)
		}
		b.WriteString("  Resolution happens against the screen when the request runs, and " +
			"is re-checked\n  immediately before the step acts. A wrong kind is refused; " +
			"several candidates is a question.\n")
	}

	if ex.Why != "" {
		fmt.Fprintf(&b, "\nReason:\n  %s\n", ex.Why)
	}

	b.WriteString("\nSafety:\n")
	fmt.Fprintf(&b, "  Risk         %s\n", ex.Safety.Risk)
	if ex.Safety.RequiresConfirmation {
		b.WriteString("  Confirms     yes — this is asked about before anything runs\n")
	}
	if ex.Safety.Destructive || ex.Safety.Irreversible || ex.Safety.External {
		var flags []string
		if ex.Safety.Destructive {
			flags = append(flags, "destructive")
		}
		if ex.Safety.Irreversible {
			flags = append(flags, "irreversible")
		}
		if ex.Safety.External {
			flags = append(flags, "visible outside this machine")
		}
		fmt.Fprintf(&b, "  Effects      %s\n", strings.Join(flags, ", "))
	}
	b.WriteString("\nNothing was run: this expanded the request without performing it.\n")
	return b.String()
}

// registry builds the procedure library, reporting a validation failure the same way
// everywhere.
//
//	CLI commands using the registry must report validation failure consistently.
//
// One helper rather than four call sites deciding for themselves, so a broken library
// produces the same message and the same exit code from every command that touches it —
// and so no command can quietly carry on with a registry the service would refuse.
// registry is the procedure library as the running Director has it: the built-ins plus
// whatever the user has demonstrated and approved.
//
// The learned procedures are read from the STORE rather than asked of the daemon, so
// `director procedures` answers with the service stopped. A procedure approved in a
// running service is written to the store as it is approved, so the two cannot disagree
// about what exists — only about what is loaded, which the listing says.
func registry() (*goal.Registry, int) {
	r, err := goal.NewValidatedRegistry()
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return nil, 1
	}
	// The capability packs' procedures, so `director procedures` lists what the running
	// service would actually have. Built from the same registeredPacks() the daemon uses:
	// a listing that showed a different set from the one that runs would be worse than no
	// listing.
	packs, perr := newGameRegistry()
	if perr != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", perr)
		return nil, 1
	}
	packs.RegisterProcedures(r)

	store, serr := demo.Open(configDir())
	if serr != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", serr)
		return nil, 1
	}
	store.Register(r)
	return r, 0
}
