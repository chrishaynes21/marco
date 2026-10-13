package main

// WHAT WAS DEMONSTRATED, AND HOW OFTEN IT COULD BE FOUND AGAIN.
//
// [[ADR-180-finding-a-demonstrated-target-again]] measured reacquisition on one recording and five
// repeat presses: three verified, none contradicted, two declined. That is the right answer on the
// evidence that exists and it is not a rate. A rate needs a session that walks the same route
// twice on purpose — so the command has to be able to report one before such a session is worth
// asking anybody for.

import (
	"fmt"
	"io"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
)

// reportReacquire prints the registry and the reacquisition tally.
func reportReacquire(w io.Writer, obs *fastobserve.Observer, found reacquireTally) {
	reg := obs.Registry()
	if len(reg) == 0 {
		fmt.Fprintf(w, "\nnothing was clicked on a settled screen, so no target was "+
			"demonstrated\nand there was nothing to look for again.\n")
		return
	}
	repeated := 0
	for _, t := range reg {
		if t.Repeated() {
			repeated++
		}
	}
	fmt.Fprintf(w, "\n%d demonstrated target(s), %d clicked more than once "+
		"(session-local, never durable):\n", len(reg), repeated)
	fmt.Fprintf(w, "  a target clicked once has no independent evidence that it can be "+
		"found again.\n")

	fmt.Fprintf(w, "\n  %-10s %-10s %7s %6s %7s %6s %7s  %s\n",
		"target", "screen", "presses", "looks", "match", "ambig", "unobs", "notes")
	var looks, matched, ambiguous, unobservable int
	for _, id := range found.order {
		t := found.byTarget[id]
		looks += t.looks
		matched += t.matched
		ambiguous += t.ambiguous
		unobservable += t.unobservable
		presses := 0
		for _, r := range reg {
			if r.ID == id {
				presses = r.Presses
			}
		}
		notes := ""
		if t.uncertain {
			notes += "uncertain-identity "
		}
		if t.indistinct > 0 {
			notes += fmt.Sprintf("indistinct-from-container x%d ", t.indistinct)
		}
		fmt.Fprintf(w, "  %-10s %-10s %7d %6d %7d %6d %7d  %s\n",
			id, t.screen, presses, t.looks, t.matched, t.ambiguous,
			t.unobservable, notes)
	}
	if looks == 0 {
		// A SESSION THAT NEVER CAME BACK. Said rather than printed as 0%, because
		// "no screen with a target on it returned" and "the targets could not be
		// found" are different results and only one of them is about the matcher.
		fmt.Fprintf(w, "  no target's screen came back after it was clicked, so "+
			"nothing was looked for again.\n")
		return
	}
	pc := func(n int) float64 { return 100 * float64(n) / float64(looks) }
	fmt.Fprintf(w, "\n  over %d target-look(s), excluding the looks a target was "+
		"pressed on:\n", looks)
	fmt.Fprintf(w, "      matched      %6d  %5.1f%%   (a geometric claim about one "+
		"look, not permission)\n", matched, pc(matched))
	fmt.Fprintf(w, "      ambiguous    %6d  %5.1f%%   (abstention)\n",
		ambiguous, pc(ambiguous))
	fmt.Fprintf(w, "      unobservable %6d  %5.1f%%   (abstention)\n",
		unobservable, pc(unobservable))
}
