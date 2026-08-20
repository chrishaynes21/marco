package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/uiact"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// `director actions` — what can the Director do, and how would it do it?
//
//	director actions                 the whole semantic vocabulary
//	director actions expand          one verb's capability ladder
//	director actions --json
//
// The command exists because the capability ladder is the part of this system a user
// cannot otherwise see. "Why did it click instead of expanding?" is answerable after the
// fact by `director explain action`, but "what would it do?" has to be answerable
// BEFORE, or the ladder is folklore.
//
// It needs no service and touches no desktop: a ladder is a property of the verb, not of
// a session. Only the availability of each rung depends on a control, and that is what
// the explain command reports.

// actionsRow is one verb, for the JSON form.
type actionsRow struct {
	Kind        string   `json:"kind"`
	Phrase      string   `json:"phrase"`
	NeedsTarget bool     `json:"needs_target"`
	Risk        string   `json:"risk"`
	Reversible  bool     `json:"reversible"`
	Ladder      []string `json:"ladder"`
}

func runActions(args []string) int {
	fs := flag.NewFlagSet("actions", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	// A named verb prints its ladder in full; no argument lists the vocabulary.
	if name := strings.TrimSpace(fs.Arg(0)); name != "" {
		kind := directorapi.SemanticActionKind(strings.ToLower(strings.ReplaceAll(name, " ", "_")))
		if !kind.Known() {
			fmt.Fprintf(os.Stderr, "director: %q is not an action the Director knows.\n"+
				"Run `director actions` for the vocabulary.\n", name)
			return 1
		}
		if *jsonOut {
			return printJSON(rowFor(kind))
		}
		fmt.Print(renderAction(kind))
		return 0
	}

	if *jsonOut {
		rows := make([]actionsRow, 0, len(directorapi.SemanticVocabulary))
		for _, k := range directorapi.SemanticVocabulary {
			rows = append(rows, rowFor(k))
		}
		return printJSON(rows)
	}
	fmt.Print(renderActions())
	return 0
}

func rowFor(kind directorapi.SemanticActionKind) actionsRow {
	return actionsRow{
		Kind: string(kind), Phrase: kind.Describe(),
		NeedsTarget: kind.NeedsTarget(), Risk: string(kind.Risk()),
		Reversible: kind.Reversible(), Ladder: uiact.Describe(kind),
	}
}

// renderActions lists the vocabulary.
func renderActions() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d semantic actions. Each lowers to legal Marco; none replays a coordinate.\n\n",
		len(directorapi.SemanticVocabulary))
	fmt.Fprintf(&b, "  %-18s %-8s %-7s %s\n", "ACTION", "RISK", "TARGET", "PREFERRED IMPLEMENTATION")
	for _, k := range directorapi.SemanticVocabulary {
		target := "—"
		if k.NeedsTarget() {
			target = "needed"
		}
		// The FIRST rung, which is what it would use given a control that can do it.
		// Printing the whole ladder here would bury the vocabulary in detail; that is
		// what naming a verb is for.
		preferred := "—"
		if rungs := uiact.Ladder(k); len(rungs) > 0 {
			preferred = rungs[0].Detail
		}
		fmt.Fprintf(&b, "  %-18s %-8s %-7s %s\n", k, k.Risk(), target, preferred)
	}
	b.WriteString("\nRun `director actions <name>` for one action's full ladder.\n")
	return b.String()
}

// renderAction draws one verb's ladder.
func renderAction(kind directorapi.SemanticActionKind) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %q\n\n", kind, kind.Describe())
	fmt.Fprintf(&b, "  Risk         %s\n", kind.Risk())
	fmt.Fprintf(&b, "  Reversible   %v\n", kind.Reversible())
	if kind.NeedsTarget() {
		b.WriteString("  Target       required — this action is meaningless without one\n")
	} else {
		b.WriteString("  Target       optional — addressed to the focused context\n")
	}

	b.WriteString("\nImplementations, strongest first:\n")
	for _, line := range uiact.Describe(kind) {
		fmt.Fprintf(&b, "  %s\n", line)
	}
	// Stated rather than left implicit, because it is the load-bearing property: the
	// last rung is always a refusal, and a reader should see that the list is not
	// "keep trying until something happens".
	b.WriteString("\nThe last rung is a refusal. When nothing above it is available the action\n" +
		"is refused rather than approximated with a click.\n")
	return b.String()
}

// `director explain action` — why did it do it that way?
//
//	director explain action           the most recent semantic action
//	director explain action --json
//
// Answers the questions the milestone asks of it: why this capability, why not the
// stronger one, what the fallback was, and what the evidence was.
func runExplainAction(args []string) int {
	fs := flag.NewFlagSet("explain action", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	out, err := client.LastSemanticAction()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if out == nil {
		fmt.Println("No semantic action has run yet.")
		return 1
	}
	if *jsonOut {
		return printJSON(out)
	}
	fmt.Print(renderSemanticExplanation(*out))
	return 0
}

// renderSemanticExplanation draws one semantic action's full account.
func renderSemanticExplanation(out uiact.Outcome) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Action: %s\n", out.Kind)
	if out.Target.Label != "" {
		fmt.Fprintf(&b, "Target: %q (%s)\n", out.Target.Label, out.Target.Role)
	}

	switch {
	case out.Choice.Satisfied:
		// The outcome most in need of explaining, because nothing happened and that was
		// correct. Without this line it reads as a silent failure.
		fmt.Fprintf(&b, "\nNothing was performed: %s\n", out.Choice.Reason)
		b.WriteString("Acting would have changed the state AWAY from what was asked for.\n")
		return b.String()
	case out.Choice.Refused:
		fmt.Fprintf(&b, "\nRefused: %s\n", out.Choice.Reason)
	default:
		fmt.Fprintf(&b, "\nChosen: %s — %s\n", out.Choice.Mechanism, out.Choice.Detail)
		if out.Choice.Chord != "" {
			fmt.Fprintf(&b, "        %s\n", out.Choice.Chord)
		}
	}

	if len(out.Choice.Rejected) > 0 {
		b.WriteString("\nStronger implementations that were not available:\n")
		for _, r := range out.Choice.Rejected {
			fmt.Fprintf(&b, "  %-24s %s\n", r.Mechanism, r.Why)
		}
	} else if !out.Choice.Refused {
		b.WriteString("\nNothing stronger was available to reject: this is the preferred\n" +
			"implementation for this action.\n")
	}

	if len(out.Operations) > 0 {
		b.WriteString("\nLowered to:\n")
		for _, op := range out.Operations {
			fmt.Fprintf(&b, "  %s\n", op)
		}
	}
	if out.Choice.Mechanism.Geometric() {
		// Worth saying out loud. A geometric mechanism is the one that can miss, and a
		// reader deciding whether to trust an outcome should know which kind they got.
		b.WriteString("\nThis implementation aims at the control's on-screen rectangle, so it\n" +
			"depends on the control being where its bounds say it is.\n")
	}
	if out.Error != "" {
		fmt.Fprintf(&b, "\nError: %s\n", out.Error)
	}
	return b.String()
}
