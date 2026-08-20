package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/visualstate"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// `director visual` — look at a region and say what it looks like and whether it moved.
//
//	director visual
//	director visual --region x,y,width,height
//	director visual --json
//
// Like `director ocr` it captures the screen and cannot act: the service returns
// evidence, and evidence has no path to an action.
//
// Unlike `director ocr` it captures TWICE, with a settle between. That is not an
// implementation detail — a single frame can report appearance and can say nothing at
// all about change, and change is what this exists for.
func runVisual(args []string) int {
	fs := flag.NewFlagSet("visual", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print the full diagnostics as JSON")
	region := fs.String("region", "", "look at this rectangle: x,y,width,height in desktop coordinates")
	start := fs.Bool("start", false, "start the service if it is not running")
	_ = fs.Parse(flagsFirst(args))

	var rect *directorapi.Rect
	if strings.TrimSpace(*region) != "" {
		r, err := parseRegion(*region)
		if err != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", err)
			return 2
		}
		rect = &r
	}

	c, err := connect(*start)
	if err != nil {
		if *jsonOut {
			fmt.Println(`{"running":false}`)
		} else {
			fmt.Println("Director: not running")
			fmt.Println("  start it with: director serve")
		}
		return 1
	}
	defer c.Close()

	diag, err := c.ReadRegion(rect)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(diag)
	}
	fmt.Print(renderVisual(diag))
	if !diag.Available || diag.Error != "" {
		return 1
	}
	return 0
}

// renderVisual describes one visual pass.
func renderVisual(d visualstate.Diagnostics) string {
	var b strings.Builder

	if !d.Available {
		b.WriteString("Visual observation is unavailable.\n")
		if d.Error != "" {
			fmt.Fprintf(&b, "  %s\n", d.Error)
		}
		b.WriteString("\n  The Director works without it: accessibility perception and every\n")
		b.WriteString("  existing command are unaffected. What is lost is the ability to tell\n")
		b.WriteString("  \"nothing happened\" from \"it is happening right now\".\n")
		return b.String()
	}

	fmt.Fprintf(&b, "Visual observation of %s\n", d.WindowID)
	if r, ok := d.Detail["region"].(string); ok {
		fmt.Fprintf(&b, "  region      %s\n", r)
	}
	fmt.Fprintf(&b, "  captures    %d\n", d.Regions)

	if d.Error != "" {
		fmt.Fprintf(&b, "\n  FAILED      %s\n", d.Error)
		return b.String()
	}

	if d.Change != nil {
		ch := *d.Change
		fmt.Fprintf(&b, "\n  change      %s\n", ch.Kind)
		fmt.Fprintf(&b, "  cells       %.1f%% differ (largest single-cell delta %.2f)\n",
			ch.ChangedCells*100, ch.MaxCellDelta)
		fmt.Fprintf(&b, "  because     %s\n", ch.Reason)

		// The retry consequence, spelled out. It is the reason this command exists,
		// and leaving a reader to infer it from a change classification would waste
		// the whole point.
		b.WriteString("\n  For a failed action in this region, that means:\n")
		switch ch.Kind {
		case visualstate.ChangeIdentical:
			b.WriteString("    a retry is SAFE — nothing happened, so repeating cannot double-apply\n")
		case visualstate.ChangeMinor:
			b.WriteString("    a retry is safe as far as the pixels are concerned; the structural\n")
			b.WriteString("    guard still decides\n")
		case visualstate.ChangeMeaningful:
			b.WriteString("    a retry is REFUSED — something happened, and repeating might do it twice\n")
		case visualstate.ChangeStillChanging:
			b.WriteString("    a retry is REFUSED and the Director waits — something is happening now\n")
		}
	}

	for _, k := range []string{"before_digest", "after_digest"} {
		if v, ok := d.Detail[k].(string); ok {
			fmt.Fprintf(&b, "  %-12s%s\n", k, v)
		}
	}
	if v, ok := d.Detail["overlay"].(string); ok {
		fmt.Fprintf(&b, "\n  overlay     %s\n", v)
	}

	t := d.Timings
	if t.Capture > 0 || t.Total > 0 {
		fmt.Fprintf(&b, "\n  capture     %s\n", dur(t.Capture))
		fmt.Fprintf(&b, "  fingerprint %s\n", dur(t.Fingerprint))
		fmt.Fprintf(&b, "  analyse     %s\n", dur(t.Analyse))
		fmt.Fprintf(&b, "  total       %s\n", dur(t.Total))
	}

	b.WriteString("\n  This is EVIDENCE about appearance and change. It can strengthen a\n")
	b.WriteString("  verification and block an unsafe retry. It cannot make anything a control.\n")
	return b.String()
}
