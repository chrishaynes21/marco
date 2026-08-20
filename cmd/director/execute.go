package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// runExecute submits a phrase to the Director service.
//
// The client's whole job: send intent, render what comes back. It builds no
// Director, holds no state, and cannot cancel anything by dying — which is what
// makes it safe for the overlay to spawn one per spoken phrase.
func runExecute(args []string) int {
	fs := flag.NewFlagSet("execute", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print the full result as JSON")
	dryRun := fs.Bool("dry-run", false, "plan and verify without performing real input")
	noStart := fs.Bool("no-start", false, "fail rather than starting the service")
	_ = fs.Parse(args)

	request := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if request == "" {
		fmt.Fprintln(os.Stderr, `usage: director execute "click save"`)
		return 2
	}

	// "Stop" cancels the running command rather than starting a new one.
	if isStopPhrase(request) {
		return runStop(nil)
	}

	c, err := connect(!*noStart)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	defer c.Close()

	var events []service.ResponseEnvelope
	onEvent := func(ev service.ResponseEnvelope) {
		events = append(events, ev)
		if *jsonOut {
			return
		}
		renderEvent(ev)
	}

	// Submit, not Execute. Submit applies the routing order — control phrase, then a
	// pending clarification response, then a new request — which is what makes "the
	// first one" answer the question the Director just asked instead of being read as
	// a fresh command. Execute always starts a new command, and calling it here is why
	// an ambiguous request printed a bare ":" and then swallowed the answer.
	//
	// A dry run is the exception: it is a single-shot preview that performs nothing,
	// so there is no pending clarification for it to answer and no reason to route it.
	if *dryRun {
		out, err := c.Execute(request, true, onEvent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", err)
			return 1
		}
		if *jsonOut {
			return printJSON(out)
		}
		renderOutcome(out)
		if out.State == service.CommandCompleted {
			return 0
		}
		return 1
	}

	in, err := c.Submit(request, onEvent)
	if err != nil {
		if *jsonOut {
			_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
		} else {
			fmt.Fprintf(os.Stderr, "director: %v\n", err)
		}
		return 1
	}
	if *jsonOut {
		return printJSON(in)
	}
	switch {
	case in.Clarification != nil:
		fmt.Print(renderClarification(*in.Clarification))
		return exitNeedsAnswer
	case in.Cancel != nil:
		fmt.Println(in.Cancel.Message)
		return 0
	case in.Outcome == nil:
		// Defensive: an interaction with nothing in it would render as an empty line,
		// which is the failure mode this whole path exists to remove.
		fmt.Println("director: the service returned no outcome and no question")
		return 1
	}
	renderOutcome(*in.Outcome)
	if in.Outcome.State == service.CommandCompleted {
		return 0
	}
	return 1
}

// exitNeedsAnswer reports that the Director asked a question rather than finishing.
//
// Distinct from failure: nothing went wrong, and the request is still live. A script
// that treated it as failure would abandon a command the user could still complete.
const exitNeedsAnswer = 2

// renderClarification draws the question, the numbered choices, and how to answer.
//
// Every line comes from the payload. Nothing here invents a distinguishing description
// the Director cannot support — if two candidates share a label and the payload offers
// no role to tell them apart, the ordinal is the distinction, and saying more would be
// making it up.
func renderClarification(q service.ClarificationPayload) string {
	var b strings.Builder

	if q.Program() {
		fmt.Fprintf(&b, "\nCLARIFICATION REQUIRED — step %d of %d\n", q.StepIndex, q.StepCount)
		if q.CompletedSteps > 0 {
			fmt.Fprintf(&b, "%d step(s) already verified — answering continues from step %d.\n",
				q.CompletedSteps, q.StepIndex)
		}
	} else {
		b.WriteString("\nCLARIFICATION REQUIRED\n")
	}
	fmt.Fprintf(&b, "\n%s\n\n", q.Question)

	for _, c := range q.Candidates {
		fmt.Fprintf(&b, "  %d. %s", c.Index, firstNonBlank(c.Label, "(unlabelled)"))
		if c.Role != "" {
			fmt.Fprintf(&b, " — %s", c.Role)
		}
		b.WriteString("\n")
	}
	b.WriteString("\nAnswer with \"the first one\", \"the second one\", a role such as " +
		"\"the button\" — or \"stop\" to cancel.\n")
	return b.String()
}

func firstNonBlank(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// isStopPhrase recognises the cancel forms without needing the parser, so `stop`
// works even when the service is unreachable.
func isStopPhrase(s string) bool {
	switch strings.ToLower(strings.TrimSpace(strings.Trim(s, ".!"))) {
	case "stop", "cancel", "abort":
		return true
	}
	return false
}

// renderEvent shows progress as it arrives, so a ten-iteration replay is visible
// while it happens rather than after it finishes.
func renderEvent(ev service.ResponseEnvelope) {
	switch ev.Type {
	case service.ResponseAccepted:
		var p service.AcceptedPayload
		if ev.Decode(&p) == nil {
			fmt.Printf("Accepted command %s\n", p.CommandID)
		}
	case service.ResponseProgress:
		var p service.ProgressPayload
		if ev.Decode(&p) != nil {
			return
		}
		switch {
		case p.Total > 0:
			fmt.Printf("  [%d/%d] %-9s %s\n", p.Iteration, p.Total, p.Stage, p.Detail)
		case p.Iteration > 0:
			fmt.Printf("  [%d] %-9s %s\n", p.Iteration, p.Stage, p.Detail)
		default:
			fmt.Printf("  %-9s %s\n", p.Stage, p.Detail)
		}
	case service.ResponseBusy:
		var p service.BusyPayload
		if ev.Decode(&p) == nil {
			fmt.Println(p.Message)
		}
	}
}

// renderOutcome prints the terminal result.
func renderOutcome(out service.OutcomePayload) {
	if r := out.Replay; r != nil {
		fmt.Printf("\nRepeating %s\n", r.SourceNode)
		c := r.Confidence
		fmt.Printf("  confidence  intent %.2f  target %.2f  context %.2f  →  overall %.2f\n",
			c.Intent, c.Target, c.Context, c.Overall)
		for _, n := range c.Notes {
			fmt.Printf("    · %s\n", n)
		}
		for _, it := range r.Iterations {
			mark := "✗"
			if it.Verified {
				mark = "✓"
			}
			fmt.Printf("  %s %d/%d  %-14s %s\n", mark, it.Index, max(1, r.Requested),
				it.Status, truncate(it.Reason, 44))
		}
		fmt.Printf("  completed %d of %d — %s\n", r.Completed, max(1, r.Requested), r.StoppedBecause)
	} else {
		for _, t := range out.Trace {
			mark := " "
			if !t.OK {
				mark = "!"
			}
			fmt.Printf("  %s %-9s %s\n", mark, t.Stage, t.Detail)
		}
	}

	fmt.Printf("\n%s: %s\n", strings.ToUpper(string(out.State)), out.Message)
	if out.CompletedActions > 0 {
		fmt.Printf("(%d action(s) performed)\n", out.CompletedActions)
	}
}

// runHistory lists what has been done, from the service when one is running and
// from the graph on disk when it is not.
//
// Falling back to the file matters: history is a read-only question about durable
// state, and needing a service running to answer it would be a regression from the
// previous milestone.
func runHistory(args []string) int {
	fs := flag.NewFlagSet("history", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	limit := fs.Int("n", 20, "how many records to show")
	_ = fs.Parse(args)

	if c, err := connect(false); err == nil {
		defer c.Close()
		if resp, err := c.History(*limit); err == nil {
			if *jsonOut {
				return printJSON(resp.Entries)
			}
			if len(resp.Entries) == 0 {
				fmt.Println("no actions recorded yet")
				return 0
			}
			for _, e := range resp.Entries {
				fmt.Printf("%-10s %-8s %-26s %s\n", e.ID, verdictOf(e.Success, e.Status),
					truncate(e.Phrase, 26), truncate(e.Reason, 44))
			}
			return 0
		}
	}
	return runHistoryLocal(*jsonOut, *limit)
}

// runHistoryLocal reads the action graph directly.
func runHistoryLocal(jsonOut bool, limit int) int {
	nodes, err := graph.Recent(limit)
	if err != nil || len(nodes) == 0 {
		if jsonOut {
			fmt.Println("[]")
		} else {
			fmt.Println("no actions recorded yet")
		}
		return 0
	}
	if jsonOut {
		return printJSON(nodes)
	}
	for _, n := range nodes {
		fmt.Printf("%-10s %-8s %-26s %s\n", n.ID,
			verdictOf(n.Outcome.Success, string(n.Outcome.Status)),
			truncate(n.Intent.Raw, 26),
			truncate(firstNonEmptyStr(n.Verification.Reason, n.Outcome.FailureReason), 44))
	}
	return 0
}

// runLast shows the most recent node in full, straight from the graph.
func runLast(args []string) int {
	return runShow(append([]string{}, append(args, "last")...))
}

func verdictOf(success bool, status string) string {
	switch {
	case success:
		return "ok"
	case status == "unverified":
		return "unverif"
	case status == "blocked":
		return "blocked"
	}
	return "failed"
}

func printJSON(v any) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	return 0
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return "(nothing reported)"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// renderEventTo is renderEvent with an explicit writer, used by `marco director`.
func renderEventTo(w *os.File, ev service.ResponseEnvelope) {
	old := os.Stdout
	os.Stdout = w
	renderEvent(ev)
	os.Stdout = old
}
