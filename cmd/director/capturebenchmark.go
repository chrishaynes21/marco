package main

// `director capture-benchmark` — what an acquisition costs, stage by stage, and nothing else.
//
// # Why this exists
//
// [[ADR-176-the-time-is-in-the-acquisition-and-a-smaller-observation-keeps-the-clicks]] priced
// every stage of an observation but one. Marco's own share of a capture is 2ms; the proposer is
// 4.6–31.9ms depending on resolution; a recorded PNG's decode is 51ms and a live observer pays
// none of it. What is left is GDI acquisition, and the only figure anybody had for it was a
// comment saying "about 130ms" that the recording's own measured 10.0 fps contradicts.
//
// So by elimination acquisition is the bottleneck, and nobody has measured it. This measures it.
//
// # What it does and does not do
//
// It takes pictures of one window and times the syscalls. It injects nothing, clicks nothing,
// activates nothing, and writes no pixels — not to disk, not to a log, not to the terminal. The
// only thing read out of a captured buffer is one bit per frame: whether it held any non-zero
// byte at all, because a blit that fails silently returns black and a configuration that was
// faster because it returned nothing would otherwise read as an improvement.
//
// The owner authorised exactly this, and the boundary is theirs:
//
//	Timing aggregates only; no captured pixels, screenshots, window content, or input events
//	written to disk. […] Not authorised: automated clicking, gameplay, persistent visual
//	observation, or changing durable graph behavior.
//
// # Why every configuration blits the same rectangle
//
//	One technical detail I'd insist on: benchmark the same source window dimensions and
//	capture region across configurations. Otherwise a reduced capture could appear faster
//	simply because it sampled less of the interface.
//
// The source rectangle is taken once, from the validated window reference, and every
// configuration blits from it. Only the destination differs, and both are printed on every row.

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/internal/screen"
)

// maxBenchmarkCaptures bounds one configuration.
//
// A few hundred, as authorised. Enough for a p95 to mean something and far short of anything
// that could be called persistent observation.
const maxBenchmarkCaptures = 400

// maxBenchmarkWall is the hard stop, whatever the counts come to.
//
// A bounded benchmark that a slow machine turns unbounded is not a bounded benchmark. Thirty
// seconds covers four configurations of a few hundred captures each with room to spare, and
// stops dead if it does not.
const maxBenchmarkWall = 30 * time.Second

func runCaptureBenchmark(args []string) int {
	fs := flag.NewFlagSet("capture-benchmark", flag.ExitOnError)
	app := fs.String("application", "", "the application whose window to measure")
	captures := fs.Int("captures", 200, "captures per configuration")
	delay := fs.Duration("delay", 5*time.Second, "wait before starting, to switch to the game")
	_ = fs.Parse(flagsFirst(args))

	if *app == "" {
		fmt.Fprintln(os.Stderr,
			"director: --application is required\n"+
				"  example: director capture-benchmark --application rocketleague")
		return 2
	}
	if *captures < 1 || *captures > maxBenchmarkCaptures {
		fmt.Fprintf(os.Stderr, "director: --captures must be between 1 and %d\n",
			maxBenchmarkCaptures)
		return 2
	}

	windows := winprovider.New()
	tracker := windowref.NewTracker(windows)
	ctx := context.Background()
	v := tracker.AcquireBy(ctx, windows, windowref.NewDirectory(),
		windowref.Selector{Application: *app})
	if !v.State.OK() {
		fmt.Fprintf(os.Stderr, "director: %s (%s)\n", v.Reason, v.State)
		return 1
	}
	b := v.Ref.Bounds
	if b.Width <= 0 || b.Height <= 0 {
		fmt.Fprintf(os.Stderr, "director: %s reports a %dx%d window, which cannot be "+
			"measured\n", v.Ref.Describe(), b.Width, b.Height)
		return 1
	}

	cfgs := screen.DefaultAcquireConfigs()
	fmt.Printf("Measuring %s — %dx%d at (%d,%d)\n",
		v.Ref.Describe(), b.Width, b.Height, b.X, b.Y)
	fmt.Printf("  %d configuration(s) x %d capture(s), hard stop at %s\n",
		len(cfgs), *captures, maxBenchmarkWall)
	fmt.Printf("  Timings only. No pixels are kept, nothing is injected, nothing is\n")
	fmt.Printf("  written to disk. Starting in %s.\n\n", *delay)
	time.Sleep(*delay)

	// RE-VALIDATED after the delay. The window may have closed while somebody switched
	// to it, and measuring whatever inherited its handle would be a benchmark of
	// something nobody asked about.
	v = tracker.Acquire(ctx, *app)
	if !v.State.OK() {
		fmt.Fprintf(os.Stderr, "director: %s (%s)\n", v.Reason, v.State)
		return 1
	}
	b = v.Ref.Bounds

	res, err := screen.BenchmarkAcquire(b.X, b.Y, b.Width, b.Height, cfgs,
		*captures, maxBenchmarkWall)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	reportAcquire(res)
	return 0
}

// reportAcquire prints the table. Timings and counts; nothing else exists to print.
func reportAcquire(res []screen.AcquireResult) {
	for _, r := range res {
		fmt.Printf("── %s ──\n", r.Config.Name)
		fmt.Printf("    source %dx%d  →  destination %dx%d  (%.1fMB per capture)\n",
			r.SrcW, r.SrcH, r.DstW, r.DstH, float64(r.Bytes)/(1<<20))
		for _, s := range r.Stages {
			fmt.Println(s.Line())
		}
		fmt.Printf("    %d capture(s), %d failure(s), %.1f per second at the median\n",
			r.Frames, r.Failures, r.Throughput())
		// THE VALIDITY LINE. A configuration whose captures were empty is not faster,
		// and this is the only thing read out of the pixels.
		switch {
		case r.Frames == 0:
			fmt.Printf("    NO CAPTURE SUCCEEDED, so none of the above is a measurement\n")
		case r.NonBlank < r.Frames:
			fmt.Printf("    ONLY %d of %d capture(s) held any pixels at all — the rest "+
				"were blank,\n    so this configuration is not a faster capture, "+
				"it is a failed one\n", r.NonBlank, r.Frames)
		default:
			fmt.Printf("    all %d capture(s) held pixels\n", r.Frames)
		}
		// AND THE LEAK LINE. A configuration that reuses handles must not accumulate
		// them, and one that builds them per frame must give every one back.
		switch {
		case r.GDIAfter > r.GDIBefore:
			fmt.Printf("    GDI objects %d → %d: THIS LEAKED %d handle(s)\n",
				r.GDIBefore, r.GDIAfter, r.GDIAfter-r.GDIBefore)
		default:
			fmt.Printf("    GDI objects %d → %d, nothing leaked\n",
				r.GDIBefore, r.GDIAfter)
		}
		fmt.Println()
	}
	fmt.Printf("%s\n", "The budget at 10 looks a second is 100ms. ADR-176 measured Marco's")
	fmt.Printf("own work at 33.8ms full-size and 7.2ms at a quarter, so compare the total\n")
	fmt.Printf("acquisition above against 66ms and 93ms respectively. Route correspondence\n")
	fmt.Printf("falls from 5 steps to 2 across that range, so the fastest row is not\n")
	fmt.Printf("automatically the right one.\n")
}
