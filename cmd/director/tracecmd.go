package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
)

// `director trace` — where did the time go?
//
//	director trace last
//	director trace <command-id>
//	director trace last --json
//
// The command that exists because a three-minute command could not be explained. A
// total duration says a command was slow; a phase breakdown says WHICH part was, and
// a phase still RUNNING says the command is stuck rather than working.
func runTrace(args []string) int {
	fs := flag.NewFlagSet("trace", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print the trace as JSON")
	start := fs.Bool("start", false, "start the service if it is not running")
	_ = fs.Parse(flagsFirst(args))

	id := "last"
	if rest := fs.Args(); len(rest) > 0 {
		id = rest[0]
	}

	c, err := connect(*start)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Director: not running")
		return 3
	}
	defer c.Close()

	tr, err := c.Trace(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if tr == nil {
		fmt.Println("no trace recorded yet")
		return 1
	}
	if *jsonOut {
		return printJSON(tr)
	}
	fmt.Print(renderTrace(tr))
	return 0
}

// renderTrace draws the phase breakdown.
//
// By pointer: a Trace carries the mutex guarding its phases, so rendering a COPY would
// read the records through a lock nobody else holds — the accessors would still return,
// and would stop being the synchronised view they are written to be.
func renderTrace(t *trace.Trace) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %q\n", t.CommandID, t.Phrase)
	if t.ParentCommand != "" {
		// A clarification answer is its own request over the wire, so it gets its own
		// trace. This line is what stops the pair reading as two unrelated commands.
		fmt.Fprintf(&b, "continues %s\n", t.ParentCommand)
	}
	if t.ProgramID != "" {
		fmt.Fprintf(&b, "program %s\n", t.ProgramID)
	}
	b.WriteString("\n")

	for _, p := range t.Phases() {
		name := string(p.Phase)
		if p.StepCount > 0 {
			name = fmt.Sprintf("%s [%d/%d]", name, p.StepIndex, p.StepCount)
		}
		fmt.Fprintf(&b, "  %-28s %10s", name, phaseDur(p.Elapsed()))

		// COMPLETED is the common case and adding it to every line would bury the
		// three states worth noticing.
		switch p.State {
		case trace.StateCompleted:
		case trace.StateRunning:
			fmt.Fprintf(&b, "  RUNNING")
			if p.Deadline > 0 {
				fmt.Fprintf(&b, " (deadline %s)", phaseDur(p.Deadline))
			}
		default:
			fmt.Fprintf(&b, "  %s", p.State)
		}
		b.WriteString("\n")
		if p.Reason != "" && p.State != trace.StateCompleted {
			fmt.Fprintf(&b, "  %-28s %s\n", "", p.Reason)
		}
	}

	b.WriteString("\n")
	terminal := t.Terminal
	if terminal == "" {
		terminal = "running"
	}
	fmt.Fprintf(&b, "  %-28s %10s  %s\n", "total", phaseDur(t.Total()), terminal)
	return b.String()
}

// dur renders a duration at a readable precision.
//
// Sub-millisecond phases are shown as "<1ms" rather than "412µs": the point of this
// table is finding the slow phase, and exact microseconds on a fast one is noise that
// makes the slow one harder to spot.
func phaseDur(d time.Duration) string {
	switch {
	case d >= time.Second:
		return fmt.Sprintf("%.3fs", d.Seconds())
	case d >= time.Millisecond:
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return "<1ms"
}
