package main

// `marco rename` IS THE REVIEWED RENAME (R7 of docs/Surfaces-alignment-proposal-2026-11-02.md).
//
// It used to move the files itself (`RenameInPlace`): no read-back, no revalidation against what
// was shown, and a routine whose name declares its arguments could lose the declaration. Studio's
// Rename has gone through `PrepareRename` → `Rescope` since ADR-285. That move carries the
// `with …` clause, refuses a collision and a declared secret, writes the destination and reads it
// back before removing the original, and carries the recording, the step account, the phrases
// and the shortcuts. Now the CLI goes through it too, with the consent given by the person who
// typed it: a question naming the routine, where it works, what it will be called and what moves
// with it, and ENTER MEANS NO (ADR-266).
//
// Going back to RenameInPlace must fail TestTheCLIRenameIsTheReviewedOne.

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
)

func runRename(args []string) {
	phrase := strings.TrimSpace(strings.Join(args, " "))
	before, after, found := strings.Cut(phrase, " to ")
	oldName, newName := strings.TrimSpace(before), strings.TrimSpace(after)
	if !found || oldName == "" || newName == "" {
		fmt.Fprintln(os.Stderr, `usage: marco rename "old name" to "new name"`)
		os.Exit(2)
	}
	d := newDeps()
	rt, ok := d.Reg.Resolve(appOf(d), oldName)
	if !ok {
		fmt.Fprintf(os.Stderr, "No play named %q.\n", oldName)
		os.Exit(1)
	}
	os.Exit(renameReviewed(d, rt, newName, os.Stdout))
}

// renameReviewed is the whole of `marco rename` once the routine is found, returning the exit
// code: 0 renamed or declined, 1 refused or only partly done.
func renameReviewed(d orchestrator.Deps, rt routes.Route, newName string, out io.Writer) int {
	m, err := d.PrepareRename(rt, newName)
	if err != nil {
		fmt.Fprintln(out, err)
		return 1
	}
	if m.Refused != "" {
		fmt.Fprintln(out, m.Refused)
		return 1
	}
	fmt.Fprintf(out, "Rename %q (%s) to %q.\n", m.Name, rt.ID(), m.Renaming)
	if clause := clauseOfSource(m.Was); clause != "" {
		fmt.Fprintf(out, "  It keeps asking for its values: %s.\n", clause)
	}
	if keys := shortcutsNaming(d, rt); len(keys) > 0 {
		fmt.Fprintf(out, "  Its shortcuts move with it: %s.\n", strings.Join(keys, ", "))
	}
	if m.Was != m.Now {
		fmt.Fprintln(out, "  Its saved file is written again under the new name.")
	}
	if !confirmNo(out, "Rename it? [y]es / [N]o (Enter means no): ") {
		fmt.Fprintln(out, "Nothing was renamed.")
		return 0
	}
	done, err := d.Rescope(m, orchestrator.Consent{})
	if err != nil {
		fmt.Fprintln(out, err)
		return 1
	}
	fmt.Fprintln(out, done.Says)
	if done.Left != "" {
		fmt.Fprintln(out, done.Left)
	}
	if !done.Removed {
		return 1 // two routines, and that is said above
	}
	return 0
}

// clauseOfSource is what a routine asks for, `with item, note`, or "".
func clauseOfSource(src string) string {
	var names []string
	for _, a := range routes.Arguments(src) {
		names = append(names, a.Name)
	}
	if len(names) == 0 {
		return ""
	}
	return "with " + strings.Join(names, ", ")
}

// shortcutsNaming is the keys that run rt, as they are pressed.
func shortcutsNaming(d orchestrator.Deps, rt routes.Route) []string {
	var keys []string
	for _, b := range d.Reg.Bindings() {
		for _, id := range b.Routes {
			if id == rt.ID() {
				keys = append(keys, widgetconfig.Pressed(configuredLeader(), b.Key)+" "+where(b.App))
				break
			}
		}
	}
	return keys
}
