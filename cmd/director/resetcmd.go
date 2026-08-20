package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// `director reset-test-state` — empty an acceptance sandbox between live runs.
//
// # Why this exists
//
// A failed or mis-targeted Learn attempt does not fail cleanly. It leaves durable subjects for
// the wrong foreground window, candidates for routes nobody wants, proposals that hold the single
// interruption slot, and goals pointing at screens that were a mistake. Observed across one
// afternoon of live acceptance: five candidate routes, fifteen subjects, and several near-
// identical Settings pages minted by passes that had already gone wrong.
//
// The next run then proves nothing. It interacts with the wreckage of the last one, and every
// diagnosis has to begin by working out which leftovers are load-bearing.
//
// # Why it is not a product feature
//
// Because deleting what somebody taught Marco is not a convenience. Durable memory is the whole
// point of the system, and the ONLY supported way to change it is the naming and correction path
// — see [[ADR-069-a-name-is-authored-and-can-be-taken-back]]. This is a harness operation for a
// sandbox that a test created, and the guard below is what keeps it one.

// resetGuard is why a reset was refused, empty when it may proceed.
//
// # The whole safety property, in one function
//
// A reset may only ever touch a home that was DECLARED as a sandbox. Two conditions, both
// required, neither inferable from anything the command was given:
//
//   - MARCO_HOME is set. Unset means the default location, which is the real store.
//   - MARCO_HOME is not the default location. Setting it TO the default is not a declaration of
//     anything; it is the same directory reached by a longer route.
//
// Deleting either check must fail TestAResetRefusesTheRealStore.
func resetGuard(home, defaultHome string) string {
	if strings.TrimSpace(home) == "" {
		return "MARCO_HOME is not set, so this would empty the real store. " +
			"A reset only ever runs against a sandbox: set MARCO_HOME to a " +
			"throwaway directory first."
	}
	if sameDir(home, defaultHome) {
		return fmt.Sprintf(
			"MARCO_HOME is %s, which is the real store. Pointing MARCO_HOME at the "+
				"default location is not a sandbox — it is the same directory by a "+
				"longer route.", home)
	}
	return ""
}

// sameDir reports whether two paths name the same directory.
//
// Compared after cleaning and case-folded, because this runs on Windows: `C:\Users\x\AppData` and
// `c:/users/x/appdata/` are the same store, and a guard that could be stepped around by changing
// a slash would not be a guard.
func sameDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	clean := func(p string) string {
		p = filepath.Clean(p)
		if abs, err := filepath.Abs(p); err == nil {
			p = filepath.Clean(abs)
		}
		return strings.ToLower(filepath.ToSlash(p))
	}
	return clean(a) == clean(b)
}

// resettable is everything one acceptance run can leave behind.
//
// Named explicitly rather than emptying the directory, so a file this list does not know about
// survives and is REPORTED. A reset that deleted whatever it found would quietly grow into
// "remove the directory", which is the operation this is careful not to be.
var resettable = []string{
	"semantic-memory.json", // durable subjects, relationships, candidates, goals, targets
	"action-graph.json",    // what was done
	"director-history.json",
	"variables.json",
	"director-stop",
	"demonstrations", // directory
	"learned",        // directory
}

// runReset is `director reset-test-state`.
func runReset(args []string) int {
	fs := flag.NewFlagSet("reset-test-state", flag.ExitOnError)
	dry := fs.Bool("dry-run", false, "say what would be removed and remove nothing")
	_ = fs.Parse(flagsFirst(args))

	home := os.Getenv("MARCO_HOME")
	if why := resetGuard(home, defaultHome()); why != "" {
		fmt.Fprintln(os.Stderr, "director: "+why)
		return 1
	}

	if why := resetBlockedByDirector(home); why != "" {
		fmt.Fprintln(os.Stderr, "director: "+why)
		return 1
	}

	removed, kept := resetHome(home, *dry)
	verb := "removed"
	if *dry {
		verb = "would remove"
	}
	if len(removed) == 0 {
		fmt.Printf("nothing to reset in %s\n", home)
	} else {
		fmt.Printf("%s in %s:\n", verb, home)
		for _, n := range removed {
			fmt.Println("  " + n)
		}
	}
	// WHAT WAS LEFT, always. A sandbox that still holds something is a sandbox whose next
	// run is not clean, and silence about it is how "reset" comes to mean "mostly reset".
	if len(kept) > 0 {
		fmt.Println("left in place (not a known acceptance artifact):")
		for _, n := range kept {
			fmt.Println("  " + n)
		}
	}
	return 0
}

// resetHome removes the known artifacts and reports what it left.
//
// Split from runReset so the whole decision can be tested without a process, a flag set or a
// terminal. Returns what was removed and what was found-but-unknown, both sorted, so two runs of
// the same sandbox read identically.
func resetHome(home string, dry bool) (removed, kept []string) {
	known := map[string]bool{}
	for _, n := range resettable {
		known[n] = true
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		return nil, nil
	}
	for _, e := range entries {
		name := e.Name()
		switch {
		case known[name]:
			if !dry {
				if rmErr := os.RemoveAll(filepath.Join(home, name)); rmErr != nil {
					kept = append(kept, name+" (could not remove: "+rmErr.Error()+")")
					continue
				}
			}
			removed = append(removed, name)
		case strings.HasPrefix(name, "semantic-memory."):
			// Backups and hand-edits from earlier debugging. Same family, same run,
			// and leaving them is how a "clean" sandbox gets reopened from a .bak.
			if !dry {
				_ = os.RemoveAll(filepath.Join(home, name))
			}
			removed = append(removed, name)
		case name == "director-service.json":
			// The endpoint file. Not state, and a running Director is refused above —
			// but a crashed one leaves this behind and the next start would find a
			// stale endpoint.
			if !dry {
				_ = os.RemoveAll(filepath.Join(home, name))
			}
			removed = append(removed, name)
		default:
			kept = append(kept, name)
		}
	}
	sort.Strings(removed)
	sort.Strings(kept)
	return removed, kept
}

// resetDialTimeout is how long a reset waits to find out whether a Director is answering.
//
// Short. This is a local socket and the only question is "is anybody there"; a slow answer is a
// no for these purposes, because a Director too wedged to reply in a second is also not going to
// write anything back.
const resetDialTimeout = 1 * time.Second

// resetBlockedByDirector is why a reset must not run yet, empty when it may.
//
// # Reachable, not merely on-disk
//
// A live Director holds session state, grants and proposals IN MEMORY and would write its own
// copy back over anything removed here — so it is refused rather than raced.
//
// But the endpoint FILE is not the question. A Director that crashed leaves the file behind, and
// refusing forever because of it would make a sandbox unusable for the exact reason it most needs
// clearing. resetHome even deletes that file, so trusting it here would make the two disagree.
// The only question that matters is whether something will answer.
//
// Deleting this must fail TestAResetRefusesWhileADirectorIsAnswering.
func resetBlockedByDirector(home string) string {
	return resetBlockedBy(home, func(ep service.Endpoint) bool {
		return service.Reachable(ep, resetDialTimeout)
	})
}

// resetBlockedBy is the decision, with the reachability question injected.
//
// A seam, because service.Reachable connects, handshakes and pings — it is asking whether a real
// Director is there, which is the right question and one a test cannot answer with a socket. Both
// branches matter and both are load-bearing, so both are driven directly.
func resetBlockedBy(home string, reachable func(service.Endpoint) bool) string {
	ep, found := service.ReadEndpoint(home)
	if !found {
		return ""
	}
	if !reachable(ep) {
		// A stale endpoint from a crashed run. Not a blocker — and resetHome removes it.
		return ""
	}
	return "a Director is answering against this home. Stop it first (`director shutdown`) — " +
		"it holds proposals, grants and session state in memory, and would write them " +
		"back over anything removed here."
}
