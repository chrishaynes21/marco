package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sort"
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

// Capturing one desktop moment, coherently.
//
// # Why the existing fixture capture is not enough
//
// `capture-vision-fixture` records SCREENSHOTS. That is the whole of what a detector benchmark
// needs, because a detector reads pixels and a human annotates pixels.
//
// The question Roadmap 37C asks is different: what does a visual detector see that Marco's
// CURRENT perception misses? Answering it needs the screenshot and what production believed at
// the same moment, and "the same moment" is the hard part — a screenshot of one state beside an
// accessibility reading of another is not evidence about a disagreement, it is evidence about
// latency.
//
// So this takes both in one pinned pass and records enough to prove they belong together.
//
// # What it is not
//
// Not approval. Everything lands in a scratch directory marked private, exactly as the frame
// capture does, and nothing here can write into a corpus. A person reviews before any of it
// becomes evidence.
//
// Not execution. It acquires a window, reads it, and writes files. No input, no lease, no
// authority, no graph.
//
// See [[Experiment-016-desktop-perception-corpus]].

// desktopSample is one coherent perception moment, as it lands on disk beside the frame.
type desktopSample struct {
	ID          string `json:"id"`
	Application string `json:"application"`
	// Window and Generation are what the reading was pinned to. A sample whose generation
	// changed between the screenshot and the reading is not one moment.
	Window     string `json:"window"`
	Generation uint64 `json:"generation"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	// Bounds is the window rectangle every region below is a proportion of.
	Bounds directorapi.Rect `json:"bounds"`
	// CapturedAt and ReadAt bracket the sensors. The gap is the coherence evidence: a
	// reader can see how far apart the two halves were rather than trusting that they were
	// simultaneous, which they never are.
	CapturedAt time.Time `json:"captured_at"`
	ReadAt     time.Time `json:"read_at"`
	GapMillis  int64     `json:"gap_millis"`
	// Coherent says the window and its generation were the same either side of the pass.
	Coherent   bool   `json:"coherent"`
	Incoherent string `json:"incoherent,omitempty"`
	// Privacy travels with the sample, as it does on a captured frame.
	Privacy string `json:"privacy"`
	// Elements is what production perception believed, normalised to the window frame.
	Elements []desktopElement `json:"elements"`
	// CollectMillis and FuseMillis are what production perception COST on this frame.
	CollectMillis int64 `json:"collect_millis"`
	FuseMillis    int64 `json:"fuse_millis"`
	// Redactions and OffscreenDropped are what was REMOVED, kept so a reader can tell a
	// sample that was cleaned from one that never carried anything. See redactdesktop.go.
	Redactions       []redaction `json:"redactions,omitempty"`
	OffscreenDropped int         `json:"offscreen_dropped,omitempty"`
	// Providers is which sensors contributed and which failed, so an empty reading can be
	// told from an unattempted one.
	Providers []string `json:"providers"`
	Failed    []string `json:"failed,omitempty"`
}

// desktopElement is one production-believed element, in the shape a benchmark can compare.
//
// Normalised bounds, because a corpus compared in pixels would be a corpus about one screen
// resolution. Provenance, because "which sensor said so" is the whole question. No handles, no
// native ids, no absolute coordinates.
type desktopElement struct {
	ID    string `json:"id"`
	Role  string `json:"role"`
	Label string `json:"label,omitempty"`
	// Bounds is a proportion of the window rectangle.
	Bounds observe.Region `json:"bounds"`
	// Sources is the provenance: which sensors reported this element.
	Sources []string `json:"sources"`
	// Actionable is production's own answer, carried so the comparison can see it rather
	// than recompute it differently. See ADR-101 — this is capability, not affordance.
	Actionable bool `json:"actionable"`
	Affords    bool `json:"affords"`
	Enabled    bool `json:"enabled"`
	Visible    bool `json:"visible"`
}

// runCaptureDesktopSample captures one coherent desktop moment.
func runCaptureDesktopSample(args []string) int {
	fs := flag.NewFlagSet("capture-desktop-sample", flag.ExitOnError)
	app := fs.String("application", "", "the application whose window to read")
	name := fs.String("name", "", "sample name, e.g. settings-mouse-wide")
	outDir := fs.String("out", ".tmp/desktop-corpus-review", "scratch directory (must not be a corpus)")
	// WHICH WINDOW, when an application owns several.
	//
	// Settings, XBOX and Realtek Audio Console are all `applicationframehost`, so selecting
	// by application alone is ambiguous and the tracker refuses rather than guessing — the
	// same refusal Phase 0 measured when asking for Mouse settings raised XBOX. A corpus
	// sample has to name the window it is about, and a Selector takes exactly ONE primary,
	// so naming a title replaces the application rather than narrowing it.
	title := fs.String("title", "", "which window, by caption, when an application owns several")
	delay := fs.Duration("delay", 2*time.Second, "wait before reading, to bring the window forward")
	bridgeFlag := fs.String("accessibility", defaultBridge(), "path to the accessibility bridge")
	_ = fs.Parse(flagsFirst(args))

	// ONE PRIMARY SELECTOR AND A NAME.
	//
	// Either is enough on its own: an application when it owns one window, a title when it
	// owns several or when the window belongs to a host process — a UWP application like
	// Settings is hosted by `applicationframehost` and its accessibility tree is walked
	// through a different window than the frame, so selecting by application there resolves
	// one window and reads another. See windowref.Selector: exactly one primary.
	if (*app == "" && *title == "") || *name == "" {
		fmt.Fprintln(os.Stderr,
			"director: --name, and one of --application or --title, are required\n"+
				"  example: director capture-desktop-sample --application explorer "+
				"--name explorer-details\n"+
				"  example: director capture-desktop-sample "+
				"--title \"Marco perception fixture\" --name browser-fixture-wide")
		return 2
	}
	// A corpus directory is not a capture target — the same rule the frame capture keeps,
	// for the same reason: captures are private until a person has reviewed them.
	if isCorpusPath(*outDir) {
		fmt.Fprintf(os.Stderr,
			"director: %s looks like a durable corpus. Captures are private until a person "+
				"has reviewed them; write them to a scratch directory instead.\n", *outDir)
		return 2
	}
	if _, err := os.Stat(*bridgeFlag); err != nil {
		fmt.Fprintf(os.Stderr, "director: accessibility bridge not found at %s\n", *bridgeFlag)
		return 1
	}

	windows := winprovider.New()
	tracker := windowref.NewTracker(windows)
	shots := newCapture(windows)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	selector := windowref.Selector{Application: *app}
	if *title != "" {
		selector = windowref.Selector{Title: *title}
	}
	v := tracker.AcquireBy(ctx, windows, windowref.NewDirectory(), selector)
	if !v.State.OK() {
		fmt.Fprintf(os.Stderr, "director: %s (%s)\n", v.Reason, v.State)
		return 1
	}
	fmt.Printf("Reading %s in %s\n", v.Ref.Describe(), *delay)
	time.Sleep(*delay)

	// THE PINNED PASS. Re-acquire, screenshot, read, re-acquire — and record both ends so a
	// reader can see whether the window was the same window throughout.
	before := tracker.AcquireBy(ctx, windows, windowref.NewDirectory(), selector)
	if !before.State.OK() {
		fmt.Fprintf(os.Stderr, "director: target lost before the read (%s)\n", before.State)
		return 1
	}
	capturedAt := time.Now()
	img, err := shots.CaptureWindow(ctx, windowRefToWindow(before.Ref))
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: capturing: %v\n", err)
		return 1
	}

	sample, code := readDesktopWorld(ctx, *bridgeFlag, windows, tracker, before.Ref)
	if code != 0 {
		return code
	}
	readAt := time.Now()

	after := tracker.AcquireBy(ctx, windows, windowref.NewDirectory(), selector)
	sample.ID = *name
	// THE APPLICATION THE WINDOW TURNED OUT TO BELONG TO, not the one that was asked for.
	// A sample selected by title has no application on the command line, and recording an
	// empty one would make the corpus unable to group by application at all.
	sample.Application = before.Ref.Application
	if sample.Application == "" {
		sample.Application = *app
	}
	sample.Window, sample.Generation = string(before.Ref.ID), before.Ref.Generation
	sample.Bounds = before.Ref.Bounds
	sample.CapturedAt, sample.ReadAt = capturedAt, readAt
	sample.GapMillis = readAt.Sub(capturedAt).Milliseconds()
	sample.Privacy = "captured_private"
	b := img.Image.Bounds()
	sample.Width, sample.Height = b.Dx(), b.Dy()

	// COHERENCE, decided rather than assumed.
	//
	// The same window and the same generation either side of the pass. A window replaced
	// mid-read is a sample describing two different things, and the honest response is to
	// say so on the sample rather than to drop it silently — a rejected sample is evidence
	// about the capture, and a missing one is evidence about nothing.
	switch {
	case !after.State.OK():
		sample.Incoherent = "the window was lost during the read"
	case after.Ref.ID != before.Ref.ID:
		sample.Incoherent = "the window changed during the read"
	case after.Ref.Generation != before.Ref.Generation:
		sample.Incoherent = fmt.Sprintf("the window was replaced during the read "+
			"(generation %d → %d)", before.Ref.Generation, after.Ref.Generation)
	default:
		sample.Coherent = true
	}

	dir := filepath.Join(*outDir, *name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	f, err := os.Create(filepath.Join(dir, *name+".png"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	err = png.Encode(f, img.Image)
	_ = f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: encoding: %v\n", err)
		return 1
	}
	out, err := json.MarshalIndent(sample, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if err := os.WriteFile(filepath.Join(dir, "production.json"), out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}

	fmt.Printf("  %dx%d, %d element(s), %dms between the picture and the reading\n",
		sample.Width, sample.Height, len(sample.Elements), sample.GapMillis)
	if !sample.Coherent {
		fmt.Printf("  NOT COHERENT: %s\n", sample.Incoherent)
	}
	fmt.Printf("  providers: %v", sample.Providers)
	if len(sample.Failed) > 0 {
		fmt.Printf("  failed: %v", sample.Failed)
	}
	fmt.Printf("\n  %s (captured_private — nothing is approved)\n", dir)
	return 0
}

// readDesktopWorld runs one collection and fusion pass over a pinned window.
//
// THE PRODUCTION PIPELINE, not a reimplementation of it: the same collector, the same providers
// and the same fusion engine an observation session uses. A corpus built from a second reading
// of the sensors would be a corpus about that second reading.
func readDesktopWorld(ctx context.Context, bridge string, windows *winprovider.Provider,
	tracker *windowref.Tracker, ref windowref.Ref) (desktopSample, int) {

	var sample desktopSample
	host := bridgehost.New(bridge)
	defer host.Close()
	uia := uiaclient.New(host)
	if !uia.Available(ctx) {
		fmt.Fprintln(os.Stderr, "director: the accessibility provider reports it is unavailable")
		return sample, 1
	}

	// THE TARGET RESOLVER, OVER THE TRACKER THAT ACQUIRED THE TARGET.
	//
	// The bridge must prove WHICH window it walked, or its evidence cannot be attributed and
	// fusion refuses it — the honest refusal this produced before the resolver was wired.
	//
	// And then once more WITH a resolver, built over a fresh tracker that had never acquired
	// anything: its idea of the current target was empty, so every read disagreed with it. A
	// resolver is only meaningful over the tracker that did the acquiring, which is why
	// production shares one tracker between the bridge and the capture rather than giving
	// each its own.
	collector := providers.NewCollector(
		providers.NewAccessibility(uia).WithTargetResolver(newTargetResolver(tracker)),
		providers.NewWindowSystem(windows))
	engine := fusion.NewEngine()

	window := ref.ID
	req := observation.Request{Window: &window, Target: expectedTarget(ref)}
	// THE PERFORMANCE GATE, taken here rather than promised again.
	//
	// 37A and 37B both listed production perception timings and both skipped them, so the
	// project has been carrying an unmeasured cost claim through two roadmaps. Collect is
	// the sensor walk — on Explorer it dominates everything else by an order of magnitude —
	// and Fuse is the merge. Both are recorded per sample so a reader can compare them
	// against a detector pass on the same frame without trusting either number secondhand.
	collectStart := time.Now()
	cycle := collector.Collect(ctx, req)
	sample.CollectMillis = time.Since(collectStart).Milliseconds()

	fuseStart := time.Now()
	world, _, err := engine.Fuse(cycle)
	sample.FuseMillis = time.Since(fuseStart).Milliseconds()
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: fusing: %v\n", err)
		return sample, 1
	}

	for _, o := range cycle.Observations {
		sample.Providers = append(sample.Providers, string(o.Source()))
	}
	sort.Strings(sample.Providers)
	sample.Providers = dedupeStrings(sample.Providers)
	for _, f := range world.Degraded {
		sample.Failed = append(sample.Failed, string(f.Source)+": "+f.Reason)
	}
	sort.Strings(sample.Failed)

	// DETERMINISTIC ORDER, so two readings of one scene produce comparable files. A world is
	// a map; a corpus sorted by iteration would replay differently every time.
	ids := make([]directorapi.ElementID, 0, len(world.Elements))
	for id := range world.Elements {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	frame := ref.Bounds
	for _, id := range ids {
		el := world.Elements[id]
		if el == nil || el.Bounds.Empty() {
			continue
		}
		if el.WindowID != "" && ref.ID != "" && el.WindowID != ref.ID {
			// Another window's element. A sample describes ONE window.
			continue
		}
		sources := make([]string, 0, el.Provenance.Len())
		for _, s := range el.Provenance.Sources {
			sources = append(sources, string(s.Source))
		}
		sort.Strings(sources)
		acts := el.Actions()
		sample.Elements = append(sample.Elements, desktopElement{
			ID: string(id), Role: string(el.Role), Label: el.Label,
			Bounds:     observe.RelativeTo(el.Bounds, frame),
			Sources:    dedupeStrings(sources),
			Actionable: acts.Targetable(), Affords: acts.Affords(),
			Enabled: el.Enabled, Visible: el.Visible,
		})
	}
	return sample, 0
}

// Geometry is normalised through observe.RelativeTo — THE conversion the production reading
// uses — for the reason [[ADR-091-a-place-is-not-its-presentation]] gives about identity: a
// measurement in pixels is a measurement about one screen, and a corpus with its own second
// conversion would be comparing two different notions of "where".

func dedupeStrings(in []string) []string {
	out := in[:0]
	var last string
	for i, s := range in {
		if i > 0 && s == last {
			continue
		}
		out = append(out, s)
		last = s
	}
	return out
}
