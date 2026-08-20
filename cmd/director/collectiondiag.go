package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
)

// Live collection diagnostics.
//
//	Diagnostics explain the query and iteration state without exposing the
//	interface's private contents.
//
// Same discipline as the value diagnostics, and for the same reasons: the reference is
// held only while the program lives, the snapshot is copied under the lock and rendered
// after it is released, and nothing here consults the Action Graph. A completed
// collection is gone, and reconstructing one from its iteration nodes would resurrect
// exactly the stale membership the design refuses to keep.

// setActiveCollections records the running program's collection environment, or clears
// it. Called by the pipeline through Pipeline.OnCollections.
func (r *Runtime) setActiveCollections(env *collections.Environment) {
	r.valuesMu.Lock()
	r.activeCollections = env
	r.valuesMu.Unlock()
}

// liveCollections returns the environment to answer diagnostics from.
//
// The RUNNING program first, then the PAUSED one — a paused collection is still alive,
// waiting for an answer that may arrive from another process, and its progress must
// stay inspectable until it resumes or is abandoned.
func (r *Runtime) liveCollections() *collections.Environment {
	r.valuesMu.RLock()
	active := r.activeCollections
	r.valuesMu.RUnlock()
	if active != nil && !active.Cleared() {
		return active
	}
	r.pausedMu.RLock()
	defer r.pausedMu.RUnlock()
	if r.paused != nil && r.paused.Ctx.Collections != nil &&
		!r.paused.Ctx.Collections.Cleared() {
		return r.paused.Ctx.Collections
	}
	return nil
}

// ActiveCollections returns a safe snapshot of the live program's collections.
func (r *Runtime) ActiveCollections() collections.Snapshot {
	env := r.liveCollections()
	if env == nil {
		return collections.Snapshot{Cleared: true, Collections: []collections.CollectionSummary{}}
	}
	// Describe copies under the environment's own lock and returns detached, so
	// everything after this line holds nothing.
	return env.Describe()
}

// renderCollections draws `director collections`.
func renderCollections(snap collections.Snapshot) string {
	if len(snap.Collections) == 0 {
		return "No active program-local collections.\n"
	}
	var b strings.Builder
	b.WriteString("Collections:\n")
	for _, c := range snap.Collections {
		fmt.Fprintf(&b, "\n%s\n", c.Name)
		fmt.Fprintf(&b, "  Kind: %s\n", c.Kind)
		fmt.Fprintf(&b, "  Query: %s\n", c.Query)
		fmt.Fprintf(&b, "  Ordering: %s\n", c.Ordering)
		fmt.Fprintf(&b, "  Limit: %d\n", c.Limit)
		fmt.Fprintf(&b, "  Completed: %d\n", c.ProcessedCount)
		if c.Provenance.MatchedAtCapture > 0 {
			fmt.Fprintf(&b, "  Matched at capture: %d\n", c.Provenance.MatchedAtCapture)
		}
		if c.Provenance.Application != "" {
			fmt.Fprintf(&b, "  Application: %s\n", c.Provenance.Application)
		}
	}
	return b.String()
}

// renderCollectionExplanation draws one collection's full account.
//
// Every line comes from recorded evidence. A match mode or a member description this
// could not establish is omitted rather than invented — an explanation that filled a
// gap plausibly would be believed, and would be wrong exactly where the capture did
// something unusual.
func renderCollectionExplanation(programID string, c collections.CollectionSummary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Collection: %s\n", c.Name)
	if programID != "" {
		fmt.Fprintf(&b, "Program: %s\n", programID)
	}
	fmt.Fprintf(&b, "Kind: %s\n", c.Kind)
	if c.Provenance.Application != "" {
		fmt.Fprintf(&b, "Application: %s\n", c.Provenance.Application)
	}
	fmt.Fprintf(&b, "Ordering: %s\n", c.Ordering)
	fmt.Fprintf(&b, "Limit: %d\n", c.Limit)

	b.WriteString("\nQuery:\n")
	fmt.Fprintf(&b, "  %s\n", c.Query)
	if c.Provenance.OrderingReason != "" {
		fmt.Fprintf(&b, "  ordered by %s\n", c.Provenance.OrderingReason)
	}

	b.WriteString("\nProgress:\n")
	fmt.Fprintf(&b, "  Matched at capture: %d\n", c.Provenance.MatchedAtCapture)
	fmt.Fprintf(&b, "  Completed: %d\n", c.ProcessedCount)
	if c.Provenance.StepIndex > 0 {
		fmt.Fprintf(&b, "  Captured at step: %d\n", c.Provenance.StepIndex)
	}

	// Stated explicitly, because what is NOT kept is the design and a reader cannot see
	// an absent field.
	b.WriteString("\nProcessed members:\n")
	fmt.Fprintf(&b, "  %d semantic key(s), digests only\n", c.ProcessedCount)
	b.WriteString("  no element ids, handles or coordinates are retained\n")
	return b.String()
}

// runCollections is `director collections`.
func runCollections(args []string) int {
	fs := flag.NewFlagSet("collections", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	snap, err := client.Collections()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *jsonOut {
		return printJSON(snap)
	}
	fmt.Print(renderCollections(snap))
	return 0
}

// runExplainCollection is `director explain collection <name>`.
func runExplainCollection(args []string) int {
	fs := flag.NewFlagSet("explain collection", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	name := strings.TrimSpace(fs.Arg(0))
	if name == "" {
		fmt.Fprintln(os.Stderr,
			`explain collection needs a name — try "director explain collection items"`)
		return 2
	}
	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	snap, err := client.Collections()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	c, found := snap.Find(name)
	if !found {
		// The honest answer for a program that has ended, and the proof of the
		// lifetime rule: nothing is consulted to reconstruct it.
		msg := fmt.Sprintf("Unknown program-local collection: %s", name)
		if *jsonOut {
			return printJSON(map[string]any{"found": false, "message": msg})
		}
		fmt.Println(msg)
		return 1
	}
	if *jsonOut {
		return printJSON(map[string]any{
			"found": true, "program_id": snap.ProgramID, "collection": c,
		})
	}
	fmt.Print(renderCollectionExplanation(snap.ProgramID, c))
	return 0
}

// renderActiveCollections draws the collection section of `director status`.
//
// Progress and state only. No member labels, no coordinates, no raw semantic keys — a
// status command must be safe to paste into a bug report, and the user's screen is
// their business.
func renderActiveCollections(snap collections.Snapshot) string {
	var b strings.Builder
	for _, c := range snap.Collections {
		fmt.Fprintf(&b, "Collection: %s\n", c.Name)
		fmt.Fprintf(&b, "  Progress: %d of %d verified\n",
			c.ProcessedCount, c.Provenance.MatchedAtCapture)
		fmt.Fprintf(&b, "  Ordering: %s\n", c.Ordering)
		fmt.Fprintf(&b, "  Limit: %d\n", c.Limit)
	}
	return b.String()
}
