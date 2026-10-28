package main

// THE ACT SURFACE, EXERCISED WITHOUT A WINDOW.
//
// `studio -acts <store>` drives every read-only verb the page calls and prints what it got. It
// opens nothing, presses nothing, and spawns no runner — so it can be pointed at a copy of
// somebody's store, or at a temporary one, and tell you whether the surface would have anything
// to show before a window is ever put on their desktop.
//
// It deliberately does NOT exercise `Run`: that spawns the supervised runner, which asks a person
// to press keys and may touch the machine. A check that did would be a check nobody could run
// safely.

import (
	"encoding/json"
	"fmt"
	"github.com/chaynes-simpleclouds/marco/internal/buildid"
	"os"
)

// runActsCheck prints what the Act surface would show. Returns an exit code.
func runActsCheck(home string) int {
	if home == "" {
		fmt.Println("FAIL  -acts needs a store to read: -acts <dir>")
		return 2
	}
	// THE SAME ENVIRONMENT THE WINDOW WOULD HAVE. `newActs` reads $MARCO_ROUTES for the
	// routes tree exactly as marco and director do, so the check has to set it the same
	// way or it would be looking somewhere else.
	if err := os.Setenv("MARCO_HOME", home); err != nil {
		fmt.Printf("FAIL  %v\n", err)
		return 1
	}
	// AND THE ROUTES TREE IS LEFT TO RESOLVE AS THE WINDOW RESOLVES IT.
	//
	// The first version defaulted it to `<home>/routes`, which is tidier and WRONG: the
	// window uses director's rule — $MARCO_ROUTES, else `routes` beside wherever Studio
	// was launched from — so a check that invented a different answer would report Acts
	// out of a directory the window never looks at. A check has to look where the thing
	// it is checking looks, even when that is the less tidy of the two.
	a := newActs(home)

	// WHICH BUILD THIS IS, and which director it would spawn. First, because a
	// diagnostic that does not say what it is reporting ON is one somebody reads
	// about the wrong binary — which is what happened: a window and a recorder a
	// day apart, and the only way to tell was comparing file timestamps.
	fmt.Printf("ACT SURFACE\n  build      %s\n  director   %s\n"+
		"  store      %s\n  routes     %s\n",
		buildid.Revision(), directorBin(), home, a.routesDir)

	// AND WHAT THE WHOLE INSTALLATION IS MADE OF, WITHOUT OPENING A WINDOW.
	//
	// `build` above is THIS binary and an installed Marco is five of them. A person
	// comparing a window with a recorder needs every revision, and the one piece that is
	// not Go has none to give — so the manifest's hashes are the only identity it has.
	//
	// Printed HERE because this is the diagnostic somebody can run, and a window is not
	// always available to the person who needs the answer: a bundle that will not start is
	// exactly the case where the report has to be reachable without starting it. The
	// problems are printed too, so a replaced file says so rather than waiting to be
	// noticed. See whatThisInstallationIsMadeOf.
	ready := a.Ready()
	if ready.Installation != "" {
		fmt.Printf("  made of    %s\n", ready.Installation)
	}
	for _, p := range ready.Problems {
		fmt.Printf("  PROBLEM    %s\n", p.Says)
		if p.Do != "" {
			fmt.Printf("             %s\n", p.Do)
		}
	}
	fmt.Println()

	demos := a.Demonstrations()
	fmt.Printf("DEMONSTRATIONS  %d\n", len(demos))
	for _, d := range demos {
		fmt.Printf("  %-28s %-18s %s\n", d.Name, d.Application, d.SavedAt)
		if d.Problem != "" {
			fmt.Printf("      cannot be read: %s\n", d.Problem)
		}
		for _, s := range d.Steps {
			fmt.Printf("      %d  %-46s %s%s\n", s.Position, s.Says,
				mark(s.Pressable, "presses", "presses nothing"),
				mark(s.Verifiable, ", can verify", ", no outcome recorded"))
		}
	}

	saved := a.Saved()
	fmt.Printf("\nACTS  %d\n", len(saved))
	for _, s := range saved {
		state := "ready"
		if !s.Runnable {
			state = "NOT READY: " + s.Why
		}
		// AND READY IS NOT THE WHOLE ANSWER. An Act whose starting screen was
		// never read preflights perfectly and can only wait for it; the window
		// says both, and a check that said only the first would be the reason
		// somebody trusted it. See waitsBecause.
		if s.Waits != "" {
			state += "\n      WILL WAIT, NOT PRESS: " + s.Waits
		}
		// AND WHETHER THE DEMONSTRATED LAYOUT IS AVAILABLE, which is the one
		// thing an Act that WILL WAIT can do instead — and the one thing a
		// person cannot work out from anything else printed here. Said both
		// ways: available is an offer nobody has taken, and unavailable names
		// what would fix it.
		if s.OnLayout {
			state += "\n      ON ITS DEMONSTRATED LAYOUT, if you ask for it: " +
				s.LayoutSays
		} else if s.LayoutSays != "" {
			state += "\n      NOT ON ITS DEMONSTRATED LAYOUT: " + s.LayoutSays
		}
		fmt.Printf("  %-28s from %-24s %v\n      %s\n", s.Name, s.From,
			s.Interactions, state)
	}

	// AND WHAT HAPPENED BEFORE, which is the third place in the window.
	past := a.History()
	fmt.Printf("\nRUNS  %d kept\n", len(past))
	for i, h := range past {
		if i >= 5 {
			fmt.Printf("  \u2026and %d more\n", len(past)-i)
			break
		}
		fmt.Printf("  %-26s %-13s %s\n      %s\n", h.Reference,
			h.Outcome, h.When, h.Said)
		if h.Next != "" {
			fmt.Printf("      WHAT TO DO: %s\n", h.Next)
		}
	}

	// AND WHAT THE PAGE WOULD POLL, so a reader can see the shape it renders.
	w, _ := json.Marshal(a.Watch())
	fmt.Printf("\nWATCH  %s\n", w)
	fmt.Printf("\n  Nothing was run, nothing was pressed, and no window was opened.\n")
	return 0
}

func mark(yes bool, when, otherwise string) string {
	if yes {
		return when
	}
	return otherwise
}
