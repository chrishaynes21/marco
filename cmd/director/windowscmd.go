package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// `director windows` — which windows exist right now, and how to name one.
//
// The companion to explicit targeting. Every other diagnostic used to look at whatever was
// in front, which meant that running one from a terminal described the terminal; three live
// attempts at a game all reported VS Code. This lists what could be targeted and issues an
// id for each so the next command can say which.
//
// The ids are EPHEMERAL and the header says so. They are valid for the generation they were
// listed in and meaningless afterwards, which is the same rule the rest of this subsystem
// runs on: a reference a person could write down and reuse tomorrow would be durable
// identity by another name.

// Windows lists the current live windows, issuing an ephemeral id for each.
//
// Control plane: it enumerates the desktop and touches no command state, so it answers
// while something else is running.
func (r *Runtime) LiveWindows(ctx context.Context, application string) []windowref.Listing {
	if r.winDirectory == nil || r.winPlatform == nil {
		return nil
	}
	return r.winDirectory.List(ctx, r.winPlatform, application)
}

// runWindows is `director windows`.
func runWindows(args []string) int {
	fs := flag.NewFlagSet("windows", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	application := fs.String("application", "", "list only this application's windows")
	_ = fs.Parse(flagsFirst(args))

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	out, err := client.LiveWindows(*application)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(out)
	}
	fmt.Print(renderWindows(out))
	return 0
}

// renderWindows draws the listing.
func renderWindows(list []windowref.Listing) string {
	if len(list) == 0 {
		return "No capturable windows.\n" +
			"Minimized and off-screen windows are omitted: they have no pixels to read.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d window(s). The ids below are EPHEMERAL — valid now, meaningless\n"+
		"after the window changes generation. Re-run this to get current ones.\n\n", len(list))
	fmt.Fprintf(&b, "  %-11s %-18s %-34s %-8s %s\n",
		"ID", "APPLICATION", "TITLE", "PID", "STATE")
	for _, w := range list {
		fmt.Fprintf(&b, "  %-11s %-18s %-34s %-8d %s\n",
			w.ID, truncate(w.Application, 18), truncate(w.Title, 34), w.ProcessID, w.State)
	}
	b.WriteString("\nTarget one explicitly, and focus stops mattering:\n" +
		"  director vision --window-id <id>\n" +
		"  director vision --application <name>\n")
	return b.String()
}
