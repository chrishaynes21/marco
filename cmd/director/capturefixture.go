package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
)

// `director capture-vision-fixture` — collect candidate benchmark frames, privately.
//
// # Why this writes somewhere unusable
//
// Because a corpus of game screenshots is exactly the kind of thing that ends up in a
// repository without anyone deciding it should. Frames land in a scratch directory, marked
// `captured_private`, and NOTHING here can promote them. Promotion is a separate act by a
// person who has looked at the pictures — see docs/Vision-Corpus-Workflow.md.
//
// The tool deliberately has no --approve flag. A flag would make approval something one could
// do by habit, and the whole reason previous corpus attempts stalled here is that this
// judgement is not mechanical.
//
// # What it captures
//
// One validated window, at full resolution, repeatedly, into an ordered sequence. Not the
// desktop: a desktop capture would contain whatever else is on screen, which is the largest
// single source of private content in a game session.

// runCaptureFixture is `director capture-vision-fixture --application X --sequence Y`.
func runCaptureFixture(args []string) int {
	fs := flag.NewFlagSet("capture-vision-fixture", flag.ExitOnError)
	app := fs.String("application", "", "the application whose window to capture")
	sequence := fs.String("sequence", "", "sequence name, e.g. rocketleague-freeplay-static")
	frames := fs.Int("frames", 15, "how many frames to capture")
	interval := fs.Duration("interval", 400*time.Millisecond, "gap between frames")
	outDir := fs.String("out", ".tmp/vision-corpus-review", "scratch directory (must not be a corpus)")
	delay := fs.Duration("delay", 3*time.Second, "wait before the first frame, to switch to the game")
	_ = fs.Parse(flagsFirst(args))

	if *app == "" || *sequence == "" {
		fmt.Fprintln(os.Stderr,
			"director: --application and --sequence are both required\n"+
				"  example: director capture-vision-fixture --application rocketleague "+
				"--sequence rocketleague-freeplay-static --frames 15")
		return 2
	}
	// A corpus directory is not a capture target. Cheap, and it is the one mistake that
	// would put unreviewed frames where the benchmark reads from.
	if isCorpusPath(*outDir) {
		fmt.Fprintf(os.Stderr,
			"director: %s looks like a durable corpus. Captures are private until a person "+
				"has reviewed them; write them to a scratch directory instead.\n", *outDir)
		return 2
	}

	// Built directly rather than through the full Runtime: capturing needs a window
	// platform, a tracker and a capture, and nothing else. A command that spun up the whole
	// Director would also start its perception providers and its action graph, which have
	// no business running while somebody is playing a game.
	windows := winprovider.New()
	tracker := windowref.NewTracker(windows)
	shots := newCapture(windows)

	ctx := context.Background()
	v := tracker.AcquireBy(ctx, windows, windowref.NewDirectory(),
		windowref.Selector{Application: *app})
	if !v.State.OK() {
		fmt.Fprintf(os.Stderr, "director: %s (%s)\n", v.Reason, v.State)
		return 1
	}
	ref := v.Ref

	dir := filepath.Join(*outDir, *sequence)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}

	fmt.Printf("Capturing %s → %s\n", ref.Describe(), dir)
	fmt.Printf("  %d frames every %s, starting in %s\n", *frames, *interval, *delay)
	fmt.Printf("  Play normally. Nothing is sent anywhere and nothing is approved.\n\n")
	time.Sleep(*delay)

	index := newCaptureIndex(*sequence, *app)
	captured := 0
	for i := range *frames {
		// Re-validate every frame. A window that closed mid-session must stop the capture
		// rather than quietly produce frames of whatever inherited the handle.
		v := tracker.Acquire(ctx, *app)
		if !v.State.OK() {
			fmt.Printf("  frame %d: target lost (%s) — stopping\n", i, v.State)
			break
		}
		img, err := shots.CaptureWindow(ctx, windowRefToWindow(v.Ref))
		if err != nil {
			fmt.Printf("  frame %d: %v\n", i, err)
			time.Sleep(*interval)
			continue
		}
		name := fmt.Sprintf("%s-%03d", *sequence, i)
		path := filepath.Join(dir, name+".png")
		f, err := os.Create(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", err)
			return 1
		}
		err = png.Encode(f, img.Image)
		_ = f.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "director: encoding %s: %v\n", name, err)
			return 1
		}
		b := img.Image.Bounds()
		index.add(captureEntry{
			ID: name, Sequence: *sequence, Index: captured,
			Width: b.Dx(), Height: b.Dy(),
			WindowGeneration: v.Ref.Generation,
			OffsetMillis:     int((time.Duration(i) * *interval).Milliseconds()),
			Privacy:          "captured_private",
		})
		captured++
		fmt.Printf("  frame %d: %dx%d generation %d\n", i, b.Dx(), b.Dy(), v.Ref.Generation)
		time.Sleep(*interval)
	}

	if err := index.write(dir); err != nil {
		fmt.Fprintf(os.Stderr, "director: writing the index: %v\n", err)
		return 1
	}
	fmt.Printf("\n%d frame(s) captured, all marked captured_private.\n", captured)
	fmt.Printf("NOTHING is approved. Review them before any becomes benchmark evidence:\n")
	fmt.Printf("  %s\n", dir)
	return 0
}

// isCorpusPath reports whether a path looks like durable benchmark evidence.
func isCorpusPath(p string) bool {
	clean := filepath.ToSlash(filepath.Clean(p))
	for _, durable := range []string{"fixtures", "docs/experiments"} {
		if len(clean) >= len(durable) && clean[:len(durable)] == durable {
			return true
		}
	}
	return false
}

// captureEntry is one frame's SAFE metadata.
//
// No handle, no desktop coordinates, no window title. A title is the commonest place a game
// puts an account name, and a handle is meaningless the moment the window changes generation.
// Application identity and the generation are what survive and what a benchmark needs.
type captureEntry struct {
	ID               string `json:"id"`
	Sequence         string `json:"sequence"`
	Index            int    `json:"index"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	WindowGeneration uint64 `json:"window_generation"`
	OffsetMillis     int    `json:"offset_ms"`
	Privacy          string `json:"privacy"`
}

type captureIndex struct {
	Sequence    string         `json:"sequence"`
	Application string         `json:"application"`
	Corpus      string         `json:"corpus"`
	Privacy     string         `json:"privacy"`
	Note        string         `json:"note"`
	Frames      []captureEntry `json:"frames"`
}

func newCaptureIndex(sequence, app string) *captureIndex {
	return &captureIndex{
		Sequence: sequence, Application: app,
		Corpus:  "candidate",
		Privacy: "captured_private",
		Note: "Unreviewed capture. Not benchmark evidence. No frame here may enter a " +
			"corpus until a person has looked at it and approved it individually.",
	}
}

func (c *captureIndex) add(e captureEntry) { c.Frames = append(c.Frames, e) }

func (c *captureIndex) write(dir string) error {
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "capture-index.json"), append(raw, '\n'), 0o644)
}
