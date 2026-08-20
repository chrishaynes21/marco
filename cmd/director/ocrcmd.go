package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// `director ocr` — read the active window and say what was seen.
//
//	director ocr
//	director ocr --json
//	director ocr --region x,y,width,height
//
// It captures the screen and reads it. It does NOT act, and structurally cannot: the
// service returns evidence, and there is no path from an observation to an action. That
// is what makes this safe to run against any window, including one showing something
// destructive.
//
// The region is in CANONICAL DESKTOP coordinates — the same space element bounds are
// in, negative values included. The capture layer converts; a caller never works in
// image pixels.
func runOCR(args []string) int {
	fs := flag.NewFlagSet("ocr", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print the full diagnostics as JSON")
	region := fs.String("region", "", "read only this rectangle: x,y,width,height in desktop coordinates")
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

	diag, err := c.ReadText(rect)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(diag)
	}
	fmt.Print(renderOCR(diag))
	if !diag.Available || diag.Error != "" {
		return 1
	}
	return 0
}

// parseRegion reads "x,y,width,height". Negative x and y are ordinary — a monitor to
// the left of the primary has them — so the parser accepts a leading minus rather than
// treating it as a malformed flag.
func parseRegion(s string) (directorapi.Rect, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return directorapi.Rect{}, fmt.Errorf(
			"--region needs x,y,width,height (got %q)", s)
	}
	nums := make([]int, 4)
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return directorapi.Rect{}, fmt.Errorf("--region: %q is not a number", p)
		}
		nums[i] = n
	}
	if nums[2] <= 0 || nums[3] <= 0 {
		return directorapi.Rect{}, fmt.Errorf("--region: width and height must be positive")
	}
	return directorapi.Rect{X: nums[0], Y: nums[1], Width: nums[2], Height: nums[3]}, nil
}

// renderOCR describes one OCR pass.
func renderOCR(d ocr.Diagnostics) string {
	var b strings.Builder

	if !d.Available {
		// The distinction this whole layer protects. "OCR is not installed" is a
		// capability gap the user can fix; "this window has no text" is a fact about
		// the screen. They must never print the same way.
		b.WriteString("OCR is unavailable.\n")
		if d.Unavailable != "" {
			fmt.Fprintf(&b, "  %s\n", d.Unavailable)
		}
		b.WriteString("\n  The Director works without it: accessibility perception is unaffected,\n")
		b.WriteString("  and no command depends on OCR.\n")
		b.WriteString("\n  To enable it: install tesseract, build plugins/ocr, and point\n")
		b.WriteString("  $DIRECTOR_OCR at ocr.exe (or $MARCO_TESSERACT at tesseract.exe).\n")
		return b.String()
	}

	fmt.Fprintf(&b, "OCR of %s", d.Application)
	if d.WindowID != "" {
		fmt.Fprintf(&b, " (%s)", d.WindowID)
	}
	if d.FromCache {
		fmt.Fprint(&b, "  [cached]")
	}
	b.WriteString("\n")

	if d.Region != nil {
		fmt.Fprintf(&b, "  region      %d,%d %dx%d (desktop coordinates)\n",
			d.Region.X, d.Region.Y, d.Region.Width, d.Region.Height)
	}
	if d.ImageSize != "" {
		fmt.Fprintf(&b, "  image       %s\n", d.ImageSize)
	}
	if d.Transform != "" {
		fmt.Fprintf(&b, "  transform   %s\n", d.Transform)
	}

	if d.Error != "" {
		fmt.Fprintf(&b, "\n  FAILED      %s\n", d.Error)
		if d.Counters.RejectedStaleCapture > 0 {
			b.WriteString("  Nothing was read. A capture whose window moved would place every\n")
			b.WriteString("  word confidently in the wrong place, so it is refused outright.\n")
		}
		return b.String()
	}

	c := d.Counters
	fmt.Fprintf(&b, "\n  accepted             %5d\n", c.Accepted)
	fmt.Fprintf(&b, "  rejected_empty       %5d\n", c.RejectedEmpty)
	fmt.Fprintf(&b, "  rejected_confidence  %5d\n", c.RejectedConfidence)
	fmt.Fprintf(&b, "  rejected_geometry    %5d\n", c.RejectedGeometry)
	fmt.Fprintf(&b, "  rejected_stale       %5d\n", c.RejectedStaleCapture)
	fmt.Fprintf(&b, "  (of %d engine results)\n", c.Total())

	t := d.Timings
	fmt.Fprintf(&b, "\n  capture     %s\n", dur(t.Capture))
	fmt.Fprintf(&b, "  recognise   %s\n", dur(t.Recognize))
	fmt.Fprintf(&b, "  construct   %s\n", dur(t.Construct))
	fmt.Fprintf(&b, "  total       %s\n", dur(t.Total))

	if c.Accepted == 0 {
		b.WriteString("\n  No text was accepted. That is a statement about this window, not\n")
		b.WriteString("  about OCR: the engine ran and the counters above say why.\n")
	} else {
		// Said explicitly because it is the safety property, and a reader should not
		// have to infer it from the absence of a claim.
		b.WriteString("\n  This is EVIDENCE, not belief. Text becomes part of the world only\n")
		b.WriteString("  where fusion attaches it to a structural element — see: director fusion\n")
	}
	return b.String()
}

func dur(d time.Duration) string {
	switch {
	case d == 0:
		return "-"
	case d < time.Millisecond:
		return fmt.Sprintf("%dµs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
