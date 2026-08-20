package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The vision diagnostics.
//
//	director vision          run one pass and report what it saw
//	director explain vision  what the last pass made of it, and what it refused
//	director frames          the recent frames, and what came of each
//	director observations    every perception source and what it contributed, vision among them
//
// `director vision` CAPTURES THE SCREEN, like `director ocr`, which is why it is a command
// a person runs rather than something any request triggers. The others read records.

// ReadVision performs one vision pass for diagnostics.
//
// It observes and looks; it never executes, and cannot — it returns evidence, and evidence
// has no path to an action.
func (r *Runtime) ReadVision(ctx context.Context, region *directorapi.Rect, target windowref.Selector) vision.Diagnostics {
	if r.vision == nil {
		return vision.Diagnostics{
			Backend: "vision", Available: false,
			Unavailable: firstNonEmptyStr(r.visionUnavailable, "no vision provider is wired"),
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	// An explicit target replaces "whatever is in front" for this pass, which is the
	// whole point: running this command gives the terminal focus, so without a selector a
	// diagnostic about a game reliably describes the terminal instead.
	if !target.Zero() && r.winTracker != nil && r.winPlatform != nil {
		v := r.winTracker.AcquireBy(ctx, r.winPlatform, r.winDirectory, target)
		if !v.State.OK() {
			return vision.Diagnostics{
				Backend: "vision", Available: false,
				Unavailable: fmt.Sprintf("%s: %s", target.Describe(), v.Reason),
			}
		}
		r.pinnedWindow = &v.Ref
		defer func() { r.pinnedWindow = nil }()
	}

	_, diag, err := r.vision.Look(ctx, observation.WithVision(region))
	if r.winTracker != nil {
		diag.WindowGeneration = r.winTracker.Generation()
	}
	if err != nil && diag.Error == "" {
		diag.Error = err.Error()
	}
	if !diag.Available && diag.Unavailable == "" {
		diag.Unavailable = r.visionUnavailable
	}
	return diag
}

// VisionUnavailable is why vision cannot run, empty when it can.
//
// Control plane: it reads a field written once at construction, so it answers while a
// command is running.
func (r *Runtime) VisionUnavailable() string { return r.visionUnavailable }

// LastVision is the most recent pass, without performing one.
func (r *Runtime) LastVision() vision.Diagnostics {
	if r.vision == nil {
		return vision.Diagnostics{
			Backend: "vision", Available: false,
			Unavailable: firstNonEmptyStr(r.visionUnavailable, "no vision provider is wired"),
		}
	}
	return r.vision.LastDiagnostics()
}

// Frames is the recent frame log, newest first.
func (r *Runtime) Frames() []vision.FrameRecord {
	if r.vision == nil {
		return nil
	}
	return r.vision.Frames()
}

// ── rendering ─────────────────────────────────────────────────────────────────

// renderVision draws one vision pass.
//
// It leads with what was REFUSED as prominently as what was accepted. A provider that
// reported "12 observations" and silently dropped forty is one whose output a reader
// cannot calibrate, and calibrating it is the entire purpose of this command.
func renderVision(d vision.Diagnostics) string {
	var b strings.Builder

	b.WriteString("Vision\n")
	fmt.Fprintf(&b, "  provider     %s\n", d.Backend)
	if d.Model != "" {
		fmt.Fprintf(&b, "  model        %s\n", d.Model)
	}
	if !d.Available {
		fmt.Fprintf(&b, "  available    no — %s\n", d.Unavailable)
		b.WriteString("\nThe Director can still see whatever the accessibility tree exposes.\n" +
			"A detector adds boxes for applications that expose nothing.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "  available    yes\n")
	// Application, window and frame are reported as SEPARATE facts.
	//
	// The incident collapsed them: a frame was attributed to Rocket League because the
	// window record said so, long after the window had ceased to exist. A reader has to
	// be able to see which of the three is being claimed — and the raw handle is not
	// shown, because a handle in a diagnostic invites exactly the comparison across time
	// that caused the trouble.
	if d.Application != "" {
		fmt.Fprintf(&b, "  application  %s\n", d.Application)
		fmt.Fprintf(&b, "  window       validated, generation %d\n", d.WindowGeneration)
	}
	if d.FrameID != "" {
		fmt.Fprintf(&b, "  frame        %s  %s\n", d.FrameID, d.ImageSize)
	}
	if d.Transform != "" {
		fmt.Fprintf(&b, "  transform    %s\n", d.Transform)
	}
	if d.Error != "" {
		fmt.Fprintf(&b, "\n  error        %s\n", d.Error)
	}

	c := d.Counters
	b.WriteString("\nObservations\n")
	fmt.Fprintf(&b, "  considered   %d\n", c.Total())
	fmt.Fprintf(&b, "  accepted     %d   (%d as controls, %d as text)\n",
		c.Accepted, c.AcceptedStructural, c.AcceptedText)
	rejected := c.Total() - c.Accepted
	fmt.Fprintf(&b, "  rejected     %d\n", rejected)
	reject := func(label string, n int) {
		if n > 0 {
			fmt.Fprintf(&b, "     %-22s %d\n", label, n)
		}
	}
	reject("unknown class", c.RejectedClass)
	reject("below confidence", c.RejectedConfidence)
	reject("implausible geometry", c.RejectedGeometry)
	reject("stale capture", c.RejectedStaleCapture)
	reject("over the ceiling", c.RejectedCeiling)

	// Names, and how many controls did not get one.
	//
	// Both numbers or neither. A pass reporting only what it read would suggest the rest
	// were never tried, and "this control has no name" and "this control was not looked
	// at" send a reader somewhere completely different.
	if c.LabelsRead > 0 || c.LabelsUnreadable > 0 {
		b.WriteString("\nNames\n")
		fmt.Fprintf(&b, "  read         %d   (by reading the words inside the box)\n", c.LabelsRead)
		fmt.Fprintf(&b, "  unreadable   %d   (left unnamed rather than guessed)\n", c.LabelsUnreadable)
	}

	if len(d.Classes) > 0 {
		b.WriteString("\nWhat the detector called things\n")
		names := make([]string, 0, len(d.Classes))
		for name := range d.Classes {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			mark := " "
			if _, known := vision.ClassOf(name); !known {
				// The ones a reader most needs to see: a model whose vocabulary this
				// build does not share produces nothing, and looks like a model that
				// found nothing.
				mark = "?"
			}
			fmt.Fprintf(&b, "  %s %-24s %d\n", mark, name, d.Classes[name])
		}
		fmt.Fprintf(&b, "\n  ? = a class this build has no word for, so it was refused\n")
	}

	if len(d.Grids) > 0 {
		b.WriteString("\nGrids\n")
		for _, g := range d.Grids {
			fmt.Fprintf(&b, "  %-18s %dx%d, %d cells of %s  (%.0f%% regular)\n",
				g.ID, g.Rows, g.Columns, g.Cells, g.CellSize, g.Confidence*100)
		}
	}

	b.WriteString("\nTime\n")
	fmt.Fprintf(&b, "  capture      %s\n", d.Timings.Capture.Round(1e6))
	fmt.Fprintf(&b, "  detect       %s\n", d.Timings.Detect.Round(1e6))
	fmt.Fprintf(&b, "  construct    %s\n", d.Timings.Construct.Round(1e6))
	fmt.Fprintf(&b, "  total        %s\n", d.Timings.Total.Round(1e6))

	if c.Accepted > 0 {
		b.WriteString("\nNothing here is a control the Director may act on by itself. " +
			"A detected\nbox is evidence of a shape; whether it is a usable control is " +
			"fusion's and\npolicy's question.\n")
	}
	return b.String()
}

// renderFrames draws the frame log.
//
// The picture is never kept, and the header says so — a reader looking at a list of frames
// is entitled to know the Director is not holding screenshots of their desktop.
func renderFrames(records []vision.FrameRecord) string {
	if len(records) == 0 {
		return "No frames have been read.\n" +
			"Vision is opt-in: run `director vision` to take one.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d frame(s), newest first. The pictures themselves are not kept.\n\n",
		len(records))
	fmt.Fprintf(&b, "  %-14s %-20s %-11s %-9s %s\n",
		"FRAME", "WINDOW", "SIZE", "ACCEPTED", "TOTAL")
	for _, f := range records {
		fmt.Fprintf(&b, "  %-14s %-20s %-11s %-9d %s\n",
			f.ID, truncate(f.Window, 20), f.Size, f.Counters.Accepted,
			f.Timings.Total.Round(1e6))
	}
	return b.String()
}
