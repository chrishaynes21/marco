package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/diagnostics"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/explain"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The explanation front-end.
//
//	director explain               every element of the latest cycle, one line each
//	director explain <element-id>  one element in full
//	director explain --last        the most recently added element, in full
//	director explain --json        the whole model
//
// Like the other perception diagnostics, this reads the RUNNING service. An explanation
// of a cycle nobody planned against would be an explanation of nothing, and observing
// afresh to produce one would attach a second accessibility client to the desktop.

func runExplain(args []string) int {
	// "explain value <name>" is a different question about a different kind of thing —
	// a live program-local value rather than a perceived element — so it routes away
	// before the element flags are parsed.
	if len(args) > 0 && args[0] == "value" {
		return runExplainValue(args[1:])
	}
	if len(args) > 0 && args[0] == "collection" {
		return runExplainCollection(args[1:])
	}
	// "explain action" asks about the last SEMANTIC action — which implementation the
	// capability ladder chose and what it rejected — rather than about a perceived
	// element, so it routes away before the element flags are parsed.
	if len(args) > 0 && args[0] == "action" {
		return runExplainAction(args[1:])
	}
	// "explain goal" expands a request WITHOUT running it — the way to find out what
	// "close without saving" will do is to ask, not to try it.
	if len(args) > 0 && args[0] == "goal" {
		return runExplainGoal(args[1:])
	}
	// "explain procedure" asks why a LEARNED procedure has the shape it has: why a value
	// became a parameter, why the rest stayed constant, why this outcome was inferred,
	// and why a demonstration was refused. A different question from every one above, so
	// it routes away before the element flags are parsed.
	if len(args) > 0 && args[0] == "procedure" {
		return runExplainProcedure(args[1:])
	}
	// "explain game" and "explain inventory" ask about the application in front rather
	// than about a perceived element: which capability pack serves it and why, and what
	// that pack lets the Director see of the player.s holdings.
	if len(args) > 0 && args[0] == "game" {
		return runExplainGame(args[1:])
	}
	if len(args) > 0 && args[0] == "inventory" {
		return runExplainInventory(args[1:])
	}
	// "explain vision" is the last detection pass, read rather than taken: what the
	// detector saw, what was refused and why. It captures nothing.
	if len(args) > 0 && args[0] == "vision" {
		return runExplainVision(args[1:])
	}
	fs := flag.NewFlagSet("explain", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print the full model as JSON")
	last := fs.Bool("last", false, "explain the most recently added element")
	chain := fs.Bool("chain", false, "render as a source-to-replay chain")
	start := fs.Bool("start", false, "start the service if it is not running")
	// Flags first, so `explain e246 --chain` works as well as `explain --chain e246`.
	// Go's flag package stops at the first non-flag argument, which would silently
	// treat a trailing --chain as an element id and print the wrong view without
	// complaining — the worst kind of CLI failure, since the output still looks right.
	_ = fs.Parse(flagsFirst(args))

	p, code := explanation(*start, *jsonOut)
	if code != 0 {
		return code
	}
	if *jsonOut {
		// The whole model, not a summary of it. A JSON mode that dropped fields would
		// make the human output the authoritative one, which is backwards.
		return printJSON(p)
	}
	if p.Explanation == nil {
		fmt.Println("The Director has not observed anything yet, so there is nothing to explain.")
		return 1
	}
	cx := *p.Explanation

	if len(cx.Elements) == 0 {
		fmt.Printf("Cycle %s produced no elements. Nothing to explain.\n", cx.Cycle)
		if p.Fusion.ObservationCount > 0 {
			fmt.Printf("  %d observations arrived and none became belief — see: director fusion\n",
				p.Fusion.ObservationCount)
		}
		return 1
	}

	// A specific element, by id or by --last.
	target := ""
	if rest := fs.Args(); len(rest) > 0 {
		target = rest[0]
	}
	if *last {
		// "The most recent" means the newest identity, which is the highest-numbered
		// element id — ids are minted in order and never reused.
		target = string(newestElement(cx))
	}

	if target != "" {
		e, ok := cx.Find(directorapi.ElementID(target))
		if !ok {
			fmt.Fprintf(os.Stderr, "director: no element %q in cycle %s\n", target, cx.Cycle)
			suggest(cx, target)
			return 1
		}
		if *chain {
			fmt.Print(explain.RenderChain(e, nil))
		} else {
			fmt.Print(explain.Render(e))
		}
		return 0
	}

	fmt.Print(explain.RenderSummary(cx))
	fmt.Printf("\n%s", explain.RenderRules(cx))
	fmt.Printf("\nFor one element in full: director explain <element-id>\n")
	return 0
}

// flagsFirst reorders arguments so every flag precedes every positional, preserving
// the relative order of each. A flag taking a separate value ("--window x") keeps its
// value adjacent, which is why the loop skips ahead rather than sorting.
// valued names the flags that take a VALUE in the next argument.
//
// This list is why the function needs updating when a non-boolean flag is added, and
// the cost of forgetting is not a parse error but silence: the flag gets no value and
// its argument is quietly treated as a positional. That happened with --window, whose
// handle became an operation argument and produced an unhelpful usage error.
var valued = map[string]bool{
	"-window": true, "--window": true, "-n": true, "--n": true,
	// --app names the application to expand a goal for. Added after `explain goal
	// "rename this file" --app explorer` reported that "explorer" was not a goal: the
	// value had been reordered into the positional list and became the request. The
	// silence this list's comment warns about, in practice.
	"-app": true, "--app": true,
	// The window selectors, added after `director vision --application chrome --json`
	// silently produced non-JSON: "chrome" was reordered behind "--json", so the flag
	// package read the application as "--json" and the boolean never got set. Exactly the
	// failure this table's comment above describes, a second time — a flag that takes a
	// value and is missing from here fails quietly and plausibly.
	"-application": true, "--application": true,
	"-window-id": true, "--window-id": true,
	"-window-title": true, "--window-title": true,
	"-process": true, "--process": true,
	"-region": true, "--region": true,
	"-interval": true, "--interval": true,
	"-duration": true, "--duration": true,
	// `show-me`'s two durations. Same story a third time: without these,
	// `--watch 10s --no-highlight` reordered into `--watch --no-highlight 10s` and the
	// duration flag tried to parse the boolean's name.
	"-watch": true, "--watch": true,
	"-hold": true, "--hold": true,
	"-subject": true, "--subject": true,
	// `reach --from <subject>` asks what Marco would do from somewhere it is not standing.
	// Caught by this table's own test before it ever ran, which is the fourth time.
	"-from": true, "--from": true,
	// `capture-desktop-sample --title Settings` — which window, when an application owns
	// several. Caught by this table's own test the first time the command ran, which is the
	// fifth time and the reason the test walks every FlagSet rather than a list.
	"-title": true, "--title": true,
	"-question": true, "--question": true,
	// `rehearse --step 2 --live`. A fourth instance of the same silence, found while
	// testing the command that could not be invoked at all: without this, the reorder
	// produced `--step --live 2` and the step flag tried to parse "--live" as a number.
	"-step": true, "--step": true,
	// And then the whole rest of them, found in one pass once the property was written
	// down as a test rather than as a warning in this comment.
	//
	// Twelve, none of which had failed for anybody YET. That is the shape of this defect:
	// it needs a value-taking flag and a later flag in the same command line, so each one
	// waits for the first person to type that combination and then misreads it in silence.
	// See TestEveryValueTakingFlagIsInTheReorderingTable, which now holds the whole
	// package to this rather than trusting anyone to remember.
	"-repeat": true, "--repeat": true,
	"-find": true, "--find": true,
	"-actor": true, "--actor": true,
	"-verb": true, "--verb": true,
	"-every": true, "--every": true,
	"-out": true, "--out": true,
	"-frames": true, "--frames": true,
	"-reason": true, "--reason": true,
	"-split": true, "--split": true,
	"-threshold": true, "--threshold": true,
	"-detections": true, "--detections": true,
	"-accessibility": true, "--accessibility": true,
	"-backend": true, "--backend": true,
	"-control": true, "--control": true,
	"-corpus": true, "--corpus": true,
	"-delay": true, "--delay": true,
	"-explain": true, "--explain": true,
	"-fixture": true, "--fixture": true,
	"-max-nodes": true, "--max-nodes": true,
	"-name": true, "--name": true,
	"-sequence": true, "--sequence": true,
	"-sequences": true, "--sequences": true,
	"-dir": true, "--dir": true,
	"-examples": true, "--examples": true,
	"-iou": true, "--iou": true,
	"-redact": true, "--redact": true,
}

// valuedFlags is the table above, for the test that holds every command's value-taking flags to
// it. Exported within the package rather than read directly so the test names one thing.
func valuedFlags() map[string]bool { return valued }

func flagsFirst(args []string) []string {
	var flags, rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			rest = append(rest, a)
			continue
		}
		flags = append(flags, a)
		// --flag=value carries its own value. A bare flag takes the next argument only
		// if it is one of the flags that wants one; booleans must not swallow the
		// positional that follows them.
		if !strings.Contains(a, "=") && valued[a] && i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return append(flags, rest...)
}

// newestElement is the highest-numbered element id in the cycle.
func newestElement(cx explain.CycleExplanation) directorapi.ElementID {
	best, bestN := directorapi.ElementID(""), -1
	for _, e := range cx.Elements {
		if n := elementNumber(e.ElementID); n > bestN {
			best, bestN = e.ElementID, n
		}
	}
	return best
}

// elementNumber parses the counter out of an element id ("e42" → 42), -1 if it does not
// look like one.
func elementNumber(id directorapi.ElementID) int {
	s := strings.TrimPrefix(string(id), "e")
	if s == string(id) || s == "" {
		return -1
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// suggest offers the nearest thing to a mistyped element id.
//
// A bare "no such element" is unhelpful when ids are opaque and the world has hundreds:
// the likely cause is that the caller is holding an id from an earlier cycle, and
// saying which ids DO exist is the fastest way to see that.
func suggest(cx explain.CycleExplanation, target string) {
	fmt.Fprintf(os.Stderr, "  the cycle holds %d elements", len(cx.Elements))
	shown := 0
	var ids []string
	for _, e := range cx.Elements {
		if shown >= 8 {
			break
		}
		ids = append(ids, string(e.ElementID))
		shown++
	}
	if len(ids) > 0 {
		fmt.Fprintf(os.Stderr, ", including %s", strings.Join(ids, " "))
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  ids are minted per session and do not survive a service restart")
}

// explanation fetches the diagnostic picture, with explanations, from the service.
func explanation(start, jsonOut bool) (diagnostics.Perception, int) {
	c, err := connect(start)
	if err != nil {
		if jsonOut {
			fmt.Println(`{"running":false}`)
		} else {
			fmt.Println("Director: not running")
			fmt.Println("  start it with: director serve")
		}
		return diagnostics.Perception{}, 1
	}
	defer c.Close()

	p, err := c.Explain()
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return diagnostics.Perception{}, 1
	}
	return p, 0
}
