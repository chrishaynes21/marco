package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// graphPath is where the action graph is stored.
//
// The path is chosen here, in the front-end, rather than inside the Director. Where
// history lives is a deployment concern — a process, a file, a database, nothing at
// all — and putting a path inside the Director would make that choice permanent.
func graphPath() string {
	dir := os.Getenv("MARCO_HOME")
	if dir == "" {
		dir = defaultHome()
	}
	return filepath.Join(dir, "action-graph.json")
}

// defaultHome is where Marco keeps everything when MARCO_HOME says nothing.
//
// THE REAL STORE. Extracted from the path helpers so `reset-test-state` can recognise it and
// refuse: a guard that computed the real location a second time could drift from the one the
// store actually uses, and then it would be guarding a directory nobody writes to.
//
// Deleting the use of this in resetGuard must fail TestAResetRefusesTheRealStore.
func defaultHome() string {
	if home, err := os.UserConfigDir(); err == nil {
		return filepath.Join(home, "marco")
	}
	return "."
}

// semanticMemoryPath is where cross-session semantic knowledge lives.
//
// Beside the action graph, under the same `$MARCO_HOME`, so a person who wants to inspect,
// back up, copy or delete what Marco has learned finds it in one place — and so a test can
// point an entire Director at a temporary directory with one environment variable.
func semanticMemoryPath() string {
	return filepath.Join(configDir(), "semantic-memory.json")
}

// legacyHistoryPath is where the previous milestone's flat history lived.
func legacyHistoryPath() string {
	return filepath.Join(filepath.Dir(graphPath()), "director-history.json")
}

// openGraph loads the persisted graph, migrating an older history if one is found.
//
// The migration runs once: the old file is read, upgraded, written out in the
// current schema under the new name, and the original left alone. Leaving it is
// deliberate — this is the first format change, the upgrade is not yet proven
// against every history in the wild, and deleting the only copy of something that
// cannot be regenerated is not a thing to do on a hunch.
func openGraph() (*actiongraph.FileGraph, error) {
	path := graphPath()
	if _, err := os.Stat(path); err == nil {
		return actiongraph.OpenFile(path)
	}

	legacy := legacyHistoryPath()
	if _, err := os.Stat(legacy); err != nil {
		return actiongraph.OpenFile(path) // nothing to migrate; a fresh graph
	}

	nodes, err := actiongraph.Load(legacy)
	if err != nil {
		// A history that cannot be read is worth reporting rather than silently
		// starting over: starting over looks identical to losing it.
		fmt.Fprintf(os.Stderr, "director: could not read the previous history at %s: %v\n", legacy, err)
		return actiongraph.OpenFile(path)
	}
	if err := actiongraph.Save(path, nodes); err != nil {
		return nil, err
	}
	fmt.Fprintf(os.Stderr, "director: migrated %d action(s) from %s to the action graph\n",
		len(nodes), filepath.Base(legacy))
	return actiongraph.OpenFile(path)
}

// resolveNode turns a CLI node reference into a node. "last" means the most recent.
func resolveNode(g *actiongraph.FileGraph, ref string) (actiongraph.ActionNode, error) {
	if ref == "" || ref == "last" {
		return g.Last()
	}
	return g.Get(actiongraph.NodeID(ref))
}

// ── director graph ────────────────────────────────────────────────────────────

func runGraph(args []string) int {
	fs := flag.NewFlagSet("graph", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	limit := fs.Int("n", 20, "how many nodes to show")
	_ = fs.Parse(args)

	g, err := openGraph()
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	nodes, err := g.Recent(*limit)
	if err != nil || len(nodes) == 0 {
		fmt.Println("the action graph is empty")
		return 0
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(nodes)
		return 0
	}

	fmt.Printf("%d nodes in %s\n\n", g.Len(), g.Path())
	for _, n := range nodes {
		mark := "✓"
		if !n.Outcome.Success {
			mark = "✗"
		}
		// The chain marker makes parent links visible at a glance: these are the
		// edges that future workflows will be built from.
		chain := " "
		if n.Parent != nil {
			chain = "└"
		}
		fmt.Printf("%s %s %-10s %-30s %s\n", chain, mark, n.ID,
			truncate(n.Describe(), 30), truncate(n.ResolvedTarget.App, 12))
		fmt.Printf("    %s  key=%s\n",
			n.Timestamp.Format("15:04:05"), n.SemanticKey())
	}
	return 0
}

// ── director show <id> ────────────────────────────────────────────────────────

func runShow(args []string) int {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(args)

	g, err := openGraph()
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	node, err := resolveNode(g, strings.Join(fs.Args(), " "))
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(node)
		return 0
	}
	printNode(g, node)
	return 0
}

func printNode(g *actiongraph.FileGraph, n actiongraph.ActionNode) {
	rule := strings.Repeat("─", 60)
	section := func(title string) { fmt.Printf("\n%s\n%s\n", title, rule) }

	fmt.Printf("%s   %s\n", n.ID, n.Timestamp.Format("2006-01-02 15:04:05"))

	section("Action")
	fmt.Printf("  %s\n", n.Describe())
	if n.Intent.Raw != "" {
		fmt.Printf("  requested: %q\n", n.Intent.Raw)
	}
	if spec, ok := n.Action(); ok {
		fmt.Printf("  type: %s\n", spec.Type)
		// How this action would be found again — the semantic identity, and what
		// makes it repeatable at all. An ELEMENT action is re-found by query; a
		// WINDOW action by application and placement. Reporting the absence of a
		// query for a window move would be a false negative: window actions have no
		// query and are perfectly re-resolvable.
		switch {
		case spec.Window != nil:
			fmt.Printf("  window: application=%q title=%q\n",
				spec.Window.Application, spec.Window.Title)
			if spec.Placement != nil {
				fmt.Printf("  placement: %s", spec.Placement.Named)
				if spec.Placement.Bounds != nil {
					fmt.Printf("   (was (%d,%d %dx%d) — historical only)",
						spec.Placement.Bounds.X, spec.Placement.Bounds.Y,
						spec.Placement.Bounds.Width, spec.Placement.Bounds.Height)
				}
				fmt.Println()
			}
		// A DERIVED target is reported before the query, because there is deliberately no
		// useful query to report: the inline editor this step acted in existed for one
		// edit transaction, and the graph stores no way to find it again. Replay derives
		// a fresh one, which is the whole design — see the Edited section below.
		case n.RequestedTarget.RequiresEditor:
			fmt.Printf("  target: derived at run time — %s\n", n.RequestedTarget.Phrase)
		case spec.Query != nil:
			fmt.Printf("  query: %s\n", describeQuery(*spec.Query))
		default:
			fmt.Printf("  query: (none — this action cannot be re-resolved)\n")
		}
	}

	if ed := n.Editor; ed != nil {
		section("Edited")
		fmt.Printf("  %s of %s\n", ed.Property.Describe(), firstNonEmptyStr(ed.Resource, "(unknown)"))
		if ed.ClassName != "" {
			fmt.Printf("  in a %s\n", ed.ClassName)
		}
		if ed.InitialValue != "" || ed.FinalValue != "" {
			fmt.Printf("  %q → %q\n", ed.InitialValue, ed.FinalValue)
		}
		for _, e := range ed.Evidence {
			fmt.Printf("  · %s\n", e)
		}
	}

	section("Application")
	fmt.Printf("  %s\n", firstNonEmptyStr(n.ResolvedTarget.App, n.Outcome.Before.Application, "(unknown)"))
	if n.Outcome.Before.WindowTitle != "" {
		fmt.Printf("  %s\n", n.Outcome.Before.WindowTitle)
	}

	section("Resolved Target")
	t := n.ResolvedTarget
	isWindowAction := false
	if spec, ok := n.Action(); ok && spec.Window != nil {
		isWindowAction = true
	}
	switch {
	case isWindowAction:
		fmt.Printf("  Window\n  Title: %s\n", firstNonEmptyStr(t.Label, n.Outcome.Before.WindowTitle))
	case t.Role == "" && t.Label == "":
		fmt.Printf("  (nothing recorded)\n")
	default:
		fmt.Printf("  %s\n  Label: %s\n", t.Role, t.Label)
	}
	if !t.Bounds.Empty() {
		// Coordinates appear here as HISTORY: where it was, not how to find it.
		fmt.Printf("  was at (%d,%d %dx%d) — historical only\n",
			t.Bounds.X, t.Bounds.Y, t.Bounds.Width, t.Bounds.Height)
	}
	// The identity summary describes how an ELEMENT would be recognised again, and
	// says nothing useful about a window — which is re-found by application.
	if !isWindowAction {
		fmt.Printf("  identity: durable=%v label-unique=%v", t.Identity.Durable, t.Identity.LabelUnique)
		if t.Identity.AutomationID != "" {
			fmt.Printf(" automation-id=%q", t.Identity.AutomationID)
		}
		if t.Identity.ParentLabel != "" {
			fmt.Printf(" in %q", t.Identity.ParentLabel)
		}
		fmt.Println()
	}
	if n.Reason != "" {
		fmt.Printf("  chosen because: %s\n", n.Reason)
	}

	section("Verification")
	for _, e := range n.Verification.Evidence {
		if e.Observed {
			fmt.Printf("  ✓ %-26s %s\n", e.Kind, e.Detail)
		}
	}
	if n.Outcome.Success {
		fmt.Printf("  SUCCESS (%.2f) — %s\n", n.Verification.Confidence, n.Verification.Reason)
	} else {
		fmt.Printf("  %s — %s\n", strings.ToUpper(string(n.Outcome.Status)), n.Outcome.FailureReason)
	}

	if n.Parent != nil {
		section("Follows")
		if parent, err := g.Get(*n.Parent); err == nil {
			fmt.Printf("  %s  %s\n", parent.ID, parent.Describe())
		} else {
			fmt.Printf("  %s (no longer in the graph)\n", *n.Parent)
		}
	}
	if kids := g.Children(n.ID); len(kids) > 0 {
		section("Followed by")
		for _, k := range kids {
			fmt.Printf("  %s  %s\n", k.ID, k.Describe())
		}
	}

	section("Identity")
	fmt.Printf("  semantic key: %s\n", n.SemanticKey())
	fmt.Printf("  (the same key means the same action, however the screen has moved)\n")

	if len(n.Metadata) > 0 {
		section("Execution metadata")
		fmt.Printf("  (evidence of what was done, never how to do it again)\n")
		for _, k := range sortedKeys(n.Metadata) {
			fmt.Printf("  %-22s %v\n", k, n.Metadata[k])
		}
	}
	fmt.Println()
}

// ── director analyze <id> ─────────────────────────────────────────────────────

func runAnalyze(args []string) int {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	bridgeFlag := fs.String("accessibility", defaultBridge(), "path to the accessibility bridge")
	maxNodes := fs.Int("max-nodes", 4000, "node ceiling per snapshot")
	_ = fs.Parse(args)

	g, err := openGraph()
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	node, err := resolveNode(g, strings.Join(fs.Args(), " "))
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}

	// Analysis needs to see the CURRENT world — that is the whole question. But it
	// executes nothing, so a failure to observe is an answer ("I cannot tell"), not
	// a crash.
	//
	// This builds its own read-only runtime rather than asking the service, which
	// means a second accessibility client for the duration. Acceptable because
	// analysis is a development command and observes only, but it does mean the
	// tree it sees may be colder than the service's. Worth folding into the protocol
	// if `analyze` becomes something users run routinely.
	rt, err := NewRuntime(*bridgeFlag, *maxNodes, true, g)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	defer rt.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	world, err := rt.pipeline.Observe(ctx)
	if err != nil {
		// Being unable to look is a legitimate verdict. It must never be reported as
		// the target being gone, and it must never crash.
		cand := actiongraph.ReplayCandidate{
			Node: node, Status: actiongraph.ReplayUnobservable, Replayable: false,
			Reason: "could not observe the screen: " + err.Error(),
		}
		printAnalysis(cand, *jsonOut)
		return 1
	}

	cand := actiongraph.NewAnalyzer().Analyze(node, world)
	printAnalysis(cand, *jsonOut)
	if cand.Replayable {
		return 0
	}
	return 1
}

func printAnalysis(c actiongraph.ReplayCandidate, asJSON bool) {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(c)
		return
	}
	rule := strings.Repeat("─", 60)
	fmt.Printf("\nAction\n%s\n  %s   (%s)\n", rule, c.Node.Describe(), c.Node.ID)
	if app := c.Node.ResolvedTarget.App; app != "" {
		fmt.Printf("  in %s\n", app)
	}

	fmt.Printf("\nReplay Status\n%s\n  %s\n", rule, c.Status)
	fmt.Printf("\nReason\n%s\n  %s\n", rule, c.Reason)

	if c.CurrentTarget != nil {
		fmt.Printf("\nCurrent Target\n%s\n", rule)
		fmt.Printf("  %s %q at (%d,%d)\n", c.CurrentTarget.Role, c.CurrentTarget.Label,
			c.CurrentTarget.Point.X, c.CurrentTarget.Point.Y)
	}
	if c.Moved != nil {
		fmt.Printf("  moved by (%+d,%+d) since the action was recorded\n", c.Moved.DX, c.Moved.DY)
	}
	if len(c.Alternatives) > 0 {
		fmt.Printf("\nCompeting Candidates\n%s\n", rule)
		for i, a := range c.Alternatives {
			if i >= 4 || a.Rejected != "" {
				continue
			}
			fmt.Printf("  %.2f  %-10s %s\n", a.Score, a.Role, truncate(a.Label, 30))
		}
	}
	fmt.Printf("\nreplayable: %v  (nothing was executed)\n\n", c.Replayable)
}

// ── helpers ──

func describeQuery(q directorapi.ElementQuery) string {
	var parts []string
	if q.Label != "" {
		parts = append(parts, fmt.Sprintf("label=%q", q.Label))
	}
	if q.Text != "" {
		parts = append(parts, fmt.Sprintf("text=%q", q.Text))
	}
	if q.Role != "" {
		parts = append(parts, "role="+string(q.Role))
	}
	if q.Ordinal > 0 {
		parts = append(parts, fmt.Sprintf("ordinal=%d", q.Ordinal))
	}
	if len(parts) == 0 {
		return "(unconstrained)"
	}
	return strings.Join(parts, " ")
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// graph is the durable action graph, opened once per process. In the service it
// lives for the whole run; in a client it backs the read-only commands that work
// without a service.
var graph *actiongraph.FileGraph

// configDir is where the Director keeps its files.
func configDir() string { return filepath.Dir(graphPath()) }

// resolveRef re-reads the current world to turn a plan's reference into a live
// target.
//
// It re-observes rather than trusting the plan's coordinates: between planning and
// executing the screen may have moved, and the plan names WHAT to act on, not where.
func resolveRef(observe execute.Observer, ref directorapi.ElementReference) (directorapi.ResolvedTarget, error) {
	w, err := observe(context.Background())
	if err != nil {
		return directorapi.ResolvedTarget{}, err
	}
	if el, ok := w.Element(ref.ID); ok {
		native, _ := el.Attributes["native_id"].(string)
		return directorapi.ResolvedTarget{
			ElementID: el.ID, WindowID: el.WindowID, Point: el.ClickPoint(),
			Role: el.Role, Label: el.Label, NativeID: native, Confidence: el.Confidence,
		}, nil
	}
	if ref.Query != nil {
		res := target.NewResolver().Resolve(&w, *ref.Query)
		if res.Status == directorapi.ResolutionResolved {
			return *res.Target, nil
		}
		return directorapi.ResolvedTarget{}, fmt.Errorf("the target could not be found again: %s", res.Explanation)
	}
	return directorapi.ResolvedTarget{}, fmt.Errorf("the target is no longer on screen")
}
