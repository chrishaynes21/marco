package main

// Saving what the recording taught, and showing what comes back.
//
// The offline half of [[ADR-183-remembering-recognising-and-being-allowed-to-act]]: a workflow
// learned from a demonstration is written to a file, read back, and the report says what the
// reloaded one can and cannot do. It is a diagnostic, not a product path — `director` does not
// call this, nothing is written unless `-save` names a file, and the file is inert.
//
// What it is FOR is the thing a round-trip test cannot show: the artifact itself, in a person's
// hands, so the owner can read what Marco thinks it is keeping.

import (
	"fmt"
	"os"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// stamp is the artifact's SavedAt.
//
// Taken from the caller rather than the clock so that replaying one recording twice produces one
// artifact twice. A diagnostic whose output changes every run cannot be diffed, and this one
// exists to be read.
func stamp(now func() int64) time.Time {
	return time.Unix(now(), 0).UTC()
}

// saveAndReload writes the draft as a durable artifact and reports what reloading it gives.
func saveAndReload(path string, d workflow.Draft, obs *fastobserve.Observer, now func() int64) error {
	// THE EVIDENCE FOR EACH SCREEN THE DRAFT NAMES. An arrangement, not a name — the
	// whole point of the artifact is that `visual_3` means nothing tomorrow.
	arr := map[observe.VisualScreenID]observe.VisualArrangement{}
	for _, s := range obs.Screens() {
		arr[s.ID] = obs.Arrangement(s.ID)
	}

	a, err := workflowstore.Save(d, arr, stamp(now))
	if err != nil {
		return err
	}
	body, err := a.Marshal()
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return err
	}

	back, err := workflowstore.Unmarshal(body)
	if err != nil {
		return fmt.Errorf("the artifact just written could not be read back: %w", err)
	}

	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  SAVED, AND READ BACK\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("      %s — %d byte(s), version %d\n", path, len(body), back.Version)
	fmt.Printf("      %d step(s), %d screen(s), %d control(s)\n",
		len(back.Steps), len(back.Screens), len(back.Controls))

	// WHAT IT KEPT.
	var readings, containers, arrangements int
	for _, c := range back.Controls {
		readings += len(c.Readings)
		containers += len(c.Containers)
	}
	for _, s := range back.Screens {
		if s.Arrangement.Total > 0 {
			arrangements++
		}
	}
	fmt.Printf("      evidence: %d reading(s), %d container(s), %d screen arrangement(s)\n",
		readings, containers, arrangements)
	if n := len(back.Audit()); n > 0 {
		fmt.Printf("      audit: %d step(s) record who confirmed them\n", n)
	}

	// WHAT IT CANNOT DO, which is the point.
	reloaded := back.Draft()
	fmt.Printf("\n      reloaded, it is NOT executable: %t\n", !reloaded.Executable())
	fmt.Printf("      TWO INDEPENDENT REASONS, in the order a planner meets them.\n")

	// ONE: nobody has agreed. The file records who confirmed what, and that record is
	// audit — it is never read back as agreement.
	fmt.Printf("\n      1. nobody has agreed to it, however much audit it carries:\n")
	if _, ref := workflow.Plan(reloaded, 0, reloaded.Steps[0].From, nil); ref != nil {
		fmt.Printf("        %s\n", wrapAt(ref.Because, 64, "        "))
	}

	// TWO: and even confirmed, its screens are the artifact's own labels, which no
	// session ever mints. This is the reason that survives somebody deleting the first.
	confirmed := back.Draft()
	for i := range confirmed.Steps {
		_ = confirmed.Confirm(i, "this diagnostic, to show the second refusal")
	}
	fmt.Printf("\n      2. and confirmed anyway, its screens are the artifact's own\n")
	fmt.Printf("         labels, which no session mints:\n")
	if _, ref := workflow.Plan(confirmed, 0, obs.Screen(), nil); ref != nil {
		fmt.Printf("        %s\n", wrapAt(ref.Because, 64, "        "))
	} else {
		fmt.Printf("        *** IT PLANNED A STEP. A saved workflow has become\n")
		fmt.Printf("        *** executable authority, which it must never be.\n")
	}

	// AND WHAT BINDING WOULD TAKE.
	_, berr := back.Bind(workflowstore.Binding{})
	fmt.Printf("\n      binding it needs somebody to say what its labels are now:\n")
	fmt.Printf("        %s\n", wrapAt(fmt.Sprint(berr), 64, "        "))
	fmt.Printf("      Nothing here compares an arrangement or picks a best match.\n")
	fmt.Printf("      Experiment-029 measured that relation binding the wrong screen at\n")
	fmt.Printf("      0.886 while binding right ones at 0.556, with no floor between\n")
	fmt.Printf("      them, so until something can abstain the only honest binder is\n")
	fmt.Printf("      somebody who knows.\n")
	return nil
}
