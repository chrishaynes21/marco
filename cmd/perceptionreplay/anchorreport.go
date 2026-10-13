package main

// Reporting what one Yes bought, and what it did not.

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

func printAnchoring(dir string, a workflowstore.Artifact, half, total int,
	events []anchorEvent, s *workflowstore.SessionBinding) {

	fmt.Printf("\n%s\n", repeat("=", 78))
	fmt.Printf("HOW OFTEN WOULD MARCO HAVE TO ASK? — %s\n", a.Name)
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("  fold A learned %d step(s) over %d screen(s); fold B walks looks %d-%d\n",
		len(a.Steps), len(a.Screens), half+1, total)
	fmt.Printf("  The oracle answers the questions. It is NOT a simulated person: it is\n")
	fmt.Printf("  right every time and instantly, which bounds the COUNT and says nothing\n")
	fmt.Printf("  about whether somebody looking at a monitor could answer at all.\n")

	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  EVERY MOMENT THE WORKFLOW HAD TO ESTABLISH WHERE IT WAS\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("  %-6s %-9s %-10s %-10s %s\n", "look", "epoch", "screen", "resolved", "how")
	var asked, confirmed, inferred, lost int
	maxDepth := 0
	for _, e := range events {
		how := "unresolved"
		switch {
		case e.ground == workflowstore.GroundInferred:
			how = fmt.Sprintf("inferred, depth %d", e.depth)
			inferred++
			if e.depth > maxDepth {
				maxDepth = e.depth
			}
		case e.answered:
			how = "ASKED -> yes"
			confirmed++
		case e.asked:
			how = "ASKED -> no"
		}
		if e.asked {
			asked++
		}
		if e.label == "" {
			lost++
		}
		label := e.label
		if label == "" {
			label = "-"
		}
		fmt.Printf("  %-6d %-9d %-10s %-10s %s\n", e.look, e.epoch, e.screen, label, how)
		if e.because != "" && e.label == "" {
			fmt.Printf("         %s\n", wrapAt(e.because, 62, "         "))
		}
	}

	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  WHAT THAT COST\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("      %d moment(s) needed a screen established\n", len(events))
	fmt.Printf("      %d question(s) put to a person\n", asked)
	fmt.Printf("        %d answered yes, %d answered no\n", confirmed, asked-confirmed)
	fmt.Printf("      %d established WITHOUT asking, by following a demonstrated\n", inferred)
	fmt.Printf("        transition the arrangement then agreed with; deepest chain %d\n", maxDepth)
	fmt.Printf("      %d moment(s) left unresolved\n", lost)
	fmt.Printf("\n      the session's own count: %d asked, %d confirmed, %d rejected, %d unsure\n",
		s.Asked(), s.Confirmed(), s.Denied(), s.Unsures())

	// ── WHAT IS ACTUALLY DRIVING THE COUNT ──
	//
	// A question per screen of the route would be the honest price. A question per
	// observation generation is what the rule costs, and the two differ by however often
	// the observer's screen identity churns — a screen lost for a look and found again
	// is a new observation under this design, and buys a new question.
	repeats, screens := 0, map[string]bool{}
	for i, e := range events {
		if e.label == "" {
			continue
		}
		screens[e.label] = true
		if i > 0 && events[i-1].label == e.label {
			repeats++
		}
	}
	reasked := 0
	seen := map[string]bool{}
	for _, e := range events {
		if !e.asked || e.label == "" {
			continue
		}
		if seen[e.label] {
			reasked++
		}
		seen[e.label] = true
	}
	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  WHAT IS DRIVING THE COUNT\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("      %d distinct saved screen(s) were established at all\n", len(screens))
	fmt.Printf("      %d of the %d question(s) were about a screen already established\n",
		reasked, asked)
	fmt.Printf("        earlier in this same walk — a re-ask, not a new place\n")
	fmt.Printf("      %d moment(s) resolved to the SAME screen as the moment before\n", repeats)
	fmt.Printf("\n      A question per screen of the route would be the honest price. A\n")
	fmt.Printf("      question per OBSERVATION is what this rule costs, and the gap is\n")
	fmt.Printf("      the observer losing a screen for a look and finding it again: that\n")
	fmt.Printf("      is a new observation by this design, and it buys a new question.\n")
	fmt.Printf("      Whether a transient loss should move the observation generation is\n")
	fmt.Printf("      a real question and is NOT settled here. A->unobserved->A and\n")
	fmt.Printf("      A->B->A are different, and only the second is the case the rule was\n")
	fmt.Printf("      written for.\n")

	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  EXACTLY WHAT ONE YES GRANTED\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("  One saved screen, as one current screen, in ONE observation generation.\n")
	fmt.Printf("  It did not bind any other screen of the workflow, did not survive the\n")
	fmt.Printf("  screen changing, did not survive the screen changing back, and did not\n")
	fmt.Printf("  confirm the workflow, re-acquire a control or authorise a press.\n\n")
	fmt.Printf("  What invalidated it: the observation generation moving, which happens on\n")
	fmt.Printf("  every screen change and every window replacement. A revisit is a new\n")
	fmt.Printf("  observation and needs a new answer or a new inference.\n\n")
	fmt.Printf("  What was grounded WITHOUT another answer: %d of %d moment(s), each by a\n",
		inferred, len(events))
	fmt.Printf("  transition the workflow itself demonstrates AND an arrangement that\n")
	fmt.Printf("  agreed. Neither alone would do: without the transition, similarity\n")
	fmt.Printf("  nominates a screen that is not there (Experiment-029's two wrong\n")
	fmt.Printf("  bindings); without the agreement, arriving somewhere would be taken as\n")
	fmt.Printf("  proof of arriving where the workflow expected.\n")
}
