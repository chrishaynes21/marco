package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/values"
)

// Live value diagnostics.
//
//	The active environment belongs to the program, not the service globally.
//
// The runtime holds a reference to the running program's environment for exactly as
// long as that program lives. That is NOT a registry: there is one active mutating
// command at a time, the reference is handed over when the program starts and dropped
// when it ends, and nothing looks values up by any key other than "the program that is
// running now".
//
// The locking rule is the other half:
//
//	lock → copy safe metadata → unlock → render
//
// Diagnostics must never delay execution. Every function here takes a SNAPSHOT under
// the lock and returns immediately; the formatting, the previews and the JSON all
// happen on the caller's time with nothing held.

// setActiveValues records the running program's environment, or clears it.
//
// Called by the pipeline through Pipeline.OnEnvironment. Guarded by its own mutex
// rather than the command lock: a status request must be answerable while a command is
// mid-execution holding r.mu, which is the entire point of a separate control plane.
func (r *Runtime) setActiveValues(env *values.Environment) {
	r.valuesMu.Lock()
	r.activeValues = env
	r.valuesMu.Unlock()
}

// liveEnvironment returns the environment to answer diagnostics from.
//
// The RUNNING program first, then the PAUSED one. A paused program is still alive — it
// is waiting for a clarification that may arrive from an entirely different client
// process — and its values must stay inspectable until it either resumes or is
// abandoned.
func (r *Runtime) liveEnvironment() *values.Environment {
	r.valuesMu.RLock()
	active := r.activeValues
	r.valuesMu.RUnlock()
	if active != nil && !active.Cleared() {
		return active
	}
	// Under the paused program's OWN lock, not the command lock. How briefly this held
	// mu was never the hazard: Handle holds mu for the entire duration of desktop work,
	// so asking for it here meant a diagnostic waited on the command it was being run to
	// explain.
	r.pausedMu.RLock()
	defer r.pausedMu.RUnlock()
	if r.paused != nil && r.paused.Ctx.Values != nil && !r.paused.Ctx.Values.Cleared() {
		return r.paused.Ctx.Values
	}
	return nil
}

// ActiveValues returns a safe snapshot of the live program's values.
//
// Empty rather than an error when nothing is running: "no active program-local values"
// is a normal answer, not a fault.
func (r *Runtime) ActiveValues() values.EnvironmentSnapshot {
	env := r.liveEnvironment()
	if env == nil {
		return values.EnvironmentSnapshot{Cleared: true, Values: []values.ValueSnapshot{}}
	}
	// Snapshot copies under the environment's own lock and returns detached, so
	// everything after this line is free of every lock in the system.
	return env.Snapshot()
}

// renderActiveValues draws the values section of `director status`.
//
// Rendering happens on a SNAPSHOT, with no lock held. Formatting a preview or padding a
// column while the command lock was held would make a slow terminal into a slow
// Director.
func renderActiveValues(snap values.EnvironmentSnapshot) string {
	if len(snap.Values) == 0 {
		return "Active values: 0\n"
	}
	var b strings.Builder
	if snap.ProgramID != "" {
		fmt.Fprintf(&b, "Program: %s\n", snap.ProgramID)
	}
	b.WriteString("Values:\n")

	// Already sorted by the snapshot, but sorting here too costs nothing and means the
	// rendering does not silently depend on the producer's ordering.
	rows := append([]values.ValueSnapshot(nil), snap.Values...)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })

	for _, v := range rows {
		fmt.Fprintf(&b, "  %-10s %-14s %-10s %d bytes",
			v.Name, v.Kind, v.Visibility, v.ByteLength)
		// The preview is only ever shown for a PUBLIC value, and the snapshot has
		// already decided that — this reads what it was given rather than deciding
		// again, so there is one place the rule lives.
		if v.Visibility == values.VisibilityNormal {
			fmt.Fprintf(&b, "  %s", v.Preview)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// ExplainValue renders one live value's full account.
//
// Returns false when the value does not exist in the running or paused program —
// including after the program has ended, which is the case that proves the lifetime
// rule rather than merely asserting it.
//
// It deliberately does NOT consult the trace or the Action Graph. Reconstructing a
// finished value from history would resurrect exactly what the whole design spends its
// effort discarding, and would answer "what was customer?" long after the honest answer
// became "it is gone".
func (r *Runtime) ExplainValue(name string) (values.ValueSnapshot, bool) {
	return r.ActiveValues().Find(name)
}

// renderValueExplanation draws one value for a person.
func renderValueExplanation(programID string, v values.ValueSnapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Value: %s\n", v.Name)
	if programID != "" {
		fmt.Fprintf(&b, "Program: %s\n", programID)
	}
	fmt.Fprintf(&b, "Kind: %s\n", v.Kind)
	fmt.Fprintf(&b, "Visibility: %s\n", v.Visibility)

	p := v.Provenance
	if p.SourceKind != "" {
		fmt.Fprintf(&b, "Captured from: %s\n", p.SourceKind.Describe())
	}
	// Everything below is printed only where it was actually RECORDED. An explanation
	// that filled a gap with a plausible default would be believed, and would be wrong
	// precisely where the capture did something unusual.
	if p.Method != "" {
		fmt.Fprintf(&b, "Method: %s\n", p.Method)
	}
	if p.Provider != "" {
		fmt.Fprintf(&b, "Provider: %s\n", p.Provider)
	}
	if p.Application != "" {
		fmt.Fprintf(&b, "Application: %s\n", p.Application)
	}
	if p.Role != "" {
		fmt.Fprintf(&b, "Role: %s\n", p.Role)
	}
	if p.Confidence > 0 {
		fmt.Fprintf(&b, "Capture confidence: %.2f\n", p.Confidence)
	}
	if p.ClipboardRestored != nil {
		// Three states, and this prints the two that can happen. A nil means the
		// clipboard was never borrowed, and saying "clipboard restored: n/a" for a
		// window-title capture would imply it had been.
		if *p.ClipboardRestored {
			b.WriteString("Clipboard restored: verified\n")
		} else {
			b.WriteString("Clipboard restored: NO — the clipboard could not be put back\n")
		}
	}
	if p.StepIndex > 0 {
		fmt.Fprintf(&b, "Captured at step: %d\n", p.StepIndex)
	}
	fmt.Fprintf(&b, "Captured at: %s\n", v.CapturedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "Verified: %s\n", yesNo(v.Verified))
	fmt.Fprintf(&b, "Length: %d bytes\n", v.ByteLength)

	// The one line whose wording depends on visibility, and it reads the snapshot's
	// own preview rather than deciding again.
	if v.Visibility == values.VisibilityNormal {
		fmt.Fprintf(&b, "Preview: %s\n", v.Preview)
	} else {
		fmt.Fprintf(&b, "Value: %s\n", v.Preview)
	}

	b.WriteString("\nConsumed by:\n")
	if len(v.ConsumedBy) == 0 {
		b.WriteString("  nothing yet\n")
		return b.String()
	}
	for _, c := range v.ConsumedBy {
		fmt.Fprintf(&b, "  step %d %s", c.StepIndex, c.Operation)
		if c.Outcome != "" {
			fmt.Fprintf(&b, " (%s)", c.Outcome)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// unknownValueMessage is the answer for a value that is not in the live program.
//
// The same sentence whether the program ended, the name was mistyped, or nothing is
// running at all. That is deliberate: distinguishing them would tell a caller that a
// value with that name once existed, which is a small fact about a finished program
// that this layer has no business remembering.
func unknownValueMessage(name string) string {
	return fmt.Sprintf("Unknown program-local value: %s", name)
}

// runExplainValue is `director explain value <name>`.
//
// A subcommand of explain rather than a command of its own, because it answers the same
// question the rest of explain does — "why is this what it is?" — about a different kind
// of thing.
func runExplainValue(args []string) int {
	fs := flag.NewFlagSet("explain value", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print the explanation as JSON")
	_ = fs.Parse(flagsFirst(args))

	name := strings.TrimSpace(fs.Arg(0))
	if name == "" {
		fmt.Fprintln(os.Stderr, `explain value needs a name — try "director explain value customer"`)
		return 2
	}

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	out, err := client.ExplainValue(name)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *jsonOut {
		return printJSON(out)
	}
	if !out.Found {
		// The honest answer for a program that has ended, and the proof of the lifetime
		// rule: nothing is consulted to reconstruct it.
		fmt.Println(out.Message)
		return 1
	}
	fmt.Print(renderValueExplanation(out.ProgramID, *out.Value))
	return 0
}

// AbandonProgram discards a paused program and everything it captured.
//
// A cancelled clarification is a TERMINAL outcome for the program that asked, and the
// lifetime rule makes no exception for it. Without this, "stop" cleared the question and
// left the program alive: its values stayed bound, stayed visible to status, and stayed
// explainable — a data flow outliving the program that owned it, which is the one thing
// this whole design exists to prevent.
func (r *Runtime) AbandonProgram(reason string) {
	r.pausedMu.Lock()
	p := r.paused
	r.paused = nil
	r.pausedMu.Unlock()

	if p == nil || p.Ctx.Values == nil {
		return
	}
	// Cleared explicitly rather than left to become unreachable, for the same reason
	// every other terminal path clears rather than drops: a secret must stop being
	// readable now, not whenever the collector next runs.
	p.Ctx.Values.Clear()
	r.setActiveValues(nil)
}
