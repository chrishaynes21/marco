package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/uiaclient"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// WHY DID MARCO PAY FOR THIS TREE WALK?
//
// # What the audit found before this command existed
//
// The accessibility bridge holds no state between snapshots, subscribes to no UI Automation
// events, and walks the whole subtree from the window root on every call — one bulk
// CacheRequest over TreeScope.Subtree, which is the right shape for a walk and says nothing
// about how often one is needed. So the entire cost question is "how many times does the
// Director ask", and that had been reasoned about rather than counted.
//
// This counts. It repeats one window's reading N times through the PRODUCTION collector and
// fusion engine — not a copy of them — and reports what each walk cost. Pointed at Explorer it
// answers the question that motivated 37E: does watching an unchanged window rebuild the same
// 1.5-second tree over and over, and what would be saved by not doing that.
//
// It performs no input, starts no session, writes nothing, and carries no authority.

// runWalkAudit is `director walk-audit --application <app> --repeat N`.
func runWalkAudit(args []string) int {
	fs := flag.NewFlagSet("walk-audit", flag.ExitOnError)
	app := fs.String("application", "", "the application to read")
	title := fs.String("title", "", "or the window whose title contains this")
	repeat := fs.Int("repeat", 5, "how many readings to take")
	pause := fs.Duration("pause", 0, "wait between readings")
	bridgeFlag := fs.String("accessibility", defaultBridge(),
		"path to the accessibility bridge")
	_ = fs.Parse(flagsFirst(args))

	if *app == "" && *title == "" {
		fmt.Fprintln(os.Stderr,
			"director: one of --application or --title is required\n"+
				"  example: director walk-audit --title \"File Explorer\" --repeat 5")
		return 2
	}

	ctx := context.Background()
	windows := winprovider.New()
	tracker := windowref.NewTracker(windows)
	selector := windowref.Selector{Application: *app}
	if *title != "" {
		selector = windowref.Selector{Title: *title}
	}
	v := tracker.AcquireBy(ctx, windows, windowref.NewDirectory(), selector)
	if !v.State.OK() {
		fmt.Fprintf(os.Stderr, "director: %s (%s)\n", v.Reason, v.State)
		return 1
	}

	host := bridgehost.New(*bridgeFlag)
	defer host.Close()
	uia := uiaclient.New(host)
	if !uia.Available(ctx) {
		fmt.Fprintln(os.Stderr, "director: the accessibility provider reports it is unavailable")
		return 1
	}
	collector := providers.NewCollector(
		providers.NewAccessibility(uia).WithTargetResolver(newTargetResolver(tracker)),
		providers.NewWindowSystem(windows))
	engine := fusion.NewEngine()

	stop := uiaclient.CountWalks()
	defer stop()

	fmt.Printf("%-8s %10s %8s %10s %10s %s\n",
		"reading", "elements", "walk", "collect", "fuse", "sufficiency")

	var firstElements int
	same := true
	for i := 0; i < *repeat; i++ {
		if i > 0 && *pause > 0 {
			time.Sleep(*pause)
		}
		ref := tracker.AcquireBy(ctx, windows, windowref.NewDirectory(), selector)
		if !ref.State.OK() {
			fmt.Fprintf(os.Stderr, "  reading %d: %s\n", i+1, ref.Reason)
			continue
		}
		window := ref.Ref.ID
		req := observation.Request{Window: &window, Target: expectedTarget(ref.Ref)}

		before, _ := uiaclient.TotalWalks()
		restore := uiaclient.AttributeWalksTo(uiaclient.PurposeCommand)
		collectStart := time.Now()
		cycle := collector.Collect(ctx, req)
		collectFor := time.Since(collectStart)
		restore()
		after, walked := uiaclient.TotalWalks()

		fuseStart := time.Now()
		world, _, err := engine.Fuse(cycle)
		fuseFor := time.Since(fuseStart)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  reading %d: fusing: %v\n", i+1, err)
			continue
		}

		place := observe.PlaceNow(worldTotals(world, ref.Ref.Bounds), ref.Ref.Application,
			recallsNothing{}, observe.DefaultHypothesisThresholds())
		a := observe.SufficiencyOf(place)

		if i == 0 {
			firstElements = len(world.Elements)
		} else if len(world.Elements) != firstElements {
			same = false
		}
		fmt.Printf("%-8d %10d %7dx %9v %9v %s\n",
			i+1, len(world.Elements), after-before, collectFor.Round(time.Millisecond),
			fuseFor.Round(time.Millisecond), a.State)
		_ = walked
	}

	fmt.Println()
	total, spent := uiaclient.TotalWalks()
	fmt.Printf("%d full accessibility walks, %v altogether\n", total, spent.Round(time.Millisecond))
	for _, t := range uiaclient.WalksSoFar() {
		fmt.Printf("  %-14s %3d walks  %8v total  %8v longest  %8v mean\n",
			t.Purpose, t.Walks, t.Total.Round(time.Millisecond),
			t.Longest.Round(time.Millisecond),
			(t.Total / time.Duration(max(t.Walks, 1))).Round(time.Millisecond))
	}
	if *repeat > 1 {
		if same {
			fmt.Printf("\nevery reading found the same %d elements. That is %v spent "+
				"rebuilding a tree that did not change.\n", firstElements,
				spent.Round(time.Millisecond))
		} else {
			fmt.Println("\nthe readings differ, so this window was not unchanged and the " +
				"total above is not waste.")
		}
	}
	return 0
}

// worldTotals presents one fused world as a settled screen state.
//
// The same dull conversion assess-desktop-sample uses, and it carries the same limitation:
// enough for the sufficiency judgement, which reads only how many structures there are and
// where, and NOT enough for identity. See assessdesktop.go.
func worldTotals(w directorapi.WorldState, frame directorapi.Rect) observe.ShadowTotals {
	tracks := make([]observe.ShadowTrack, 0, len(w.Elements))
	for id, el := range w.Elements {
		if el == nil || el.Bounds.Empty() {
			continue
		}
		tracks = append(tracks, observe.ShadowTrack{
			ID: string(id), Role: string(el.Role), Present: true,
			Reference: observe.RelativeTo(el.Bounds, frame),
			Seen:      3, Eligible: 3,
			States: []observe.TrackState{{State: "state_1", Seen: 3, Eligible: 3}},
		})
	}
	return observe.ShadowTotals{
		CurrentState: "state_1", Tracks: tracks,
		States: []observe.ScreenState{{ID: "state_1", Inferences: 3, Settled: true}},
	}
}
