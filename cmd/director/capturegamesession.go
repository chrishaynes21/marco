package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/padsource"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
)

// `director capture-game-session` — frames and controller inputs on one clock, and nothing else.
//
// # What this is for
//
// `capture-vision-fixture` collects frames. This collects frames BESIDE what the player did,
// on a shared clock, because the question it feeds cannot be answered by either alone: does a
// selection change follow an input, and can a selection that merely moved be told from one
// that was held?
//
// # Recording is not annotation, and this records
//
// Nothing here says which entry is selected. Not a guess, not a heuristic, not a convenience
// field somebody would come to rely on. The brightness measurement that found the gap in the
// first place is a HYPOTHESIS about one game's menu, and baking it into the recorder would
// mean the corpus's ground truth and the detector's cue had a common ancestor — at which point
// the benchmark stops being independent evidence and starts agreeing with itself.
//
// So the output is frames, timestamps, and intents. Annotation is a separate act by a person,
// afterwards, as [[Vision-Corpus-Workflow]] already requires for every other kind of frame.
//
// # Why the timestamps are measured rather than planned
//
// `capture-vision-fixture` records `offset_ms` as index × interval — what the schedule
// intended. For frames alone that is harmless. Here it would be a fiction: PNG encoding of a
// 1920x1080 frame takes tens of milliseconds and the gap between frames is not the gap the
// clock asked for. Correlating a press with "the frame after it" against planned times would
// mis-order the pair that matters. Every time here is read from the clock.
//
// # Read-only, like everything that observes
//
// It captures a window and polls a controller. It injects nothing, and the controller reader
// does not bind the call that could. ADR-010 is untouched: an observation path is what that
// decision protects.

// gameFrame is one captured frame's safe metadata.
//
// Deliberately the same privacy shape as captureEntry: no handle, no title, no desktop
// coordinates. A game window's title is the commonest place an account name appears.
type gameFrame struct {
	Index            int    `json:"index"`
	File             string `json:"file"`
	AtMS             int64  `json:"at_ms"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	WindowGeneration uint64 `json:"window_generation"`
}

// gameInput is one navigation intent, as observed.
//
// Intent and time, which is the whole record. Which button produced it never reaches this
// process — see internal/platform/padsource.
type gameInput struct {
	Intent observe.NavIntent `json:"intent"`
	AtMS   int64             `json:"at_ms"`
}

// gameSession is what the directory is, for whoever opens it later.
type gameSession struct {
	Session     string `json:"session"`
	Application string `json:"application"`
	Privacy     string `json:"privacy"`
	Note        string `json:"note"`
	StartedUTC  string `json:"started_utc"`
	RequestedMS int    `json:"requested_frame_interval_ms"`
	Frames      int    `json:"frames"`
	Inputs      int    `json:"inputs"`
	DroppedIn   int    `json:"dropped_inputs"`
	// DroppedFrames is captures that were never written, because the encoders were behind
	// or a write failed. A gap somebody can count, rather than a timeline that stretched.
	DroppedFrames int `json:"dropped_frames"`
	// AchievedFPS is what the recording actually managed, from its own timestamps.
	//
	// Separate from the requested rate and printed beside it, because the two have been
	// different by a factor of three and the requested one is an intention. A reader
	// deciding whether this session can show a transition needs the measured number.
	AchievedFPS float64 `json:"achieved_fps"`
	Controller  string  `json:"controller"`
}

// achievedFPS is the rate the recording actually ran at, from the frames' own times.
func achievedFPS(frames []gameFrame) float64 {
	if len(frames) < 2 {
		return 0
	}
	span := frames[len(frames)-1].AtMS - frames[0].AtMS
	if span <= 0 {
		return 0
	}
	return float64(len(frames)-1) * 1000 / float64(span)
}

// How the encoders are bounded.
//
// Three workers against an 89ms encode is about 34 frames per second of throughput, well
// clear of any rate the capture side can feed them. The queue is small on purpose: each
// pending frame holds a full 1920x1080 image, about 8MB, so eight of them plus the three in
// flight is roughly 90MB — a bound worth having on a machine that has already killed
// processes for memory during a test run.
const (
	encodeWorkers = 3
	encodeQueue   = 8
)

// encodeFrame writes one frame, closing the file whatever happens.
func encodeFrame(enc png.Encoder, path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := enc.Encode(f, img); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func runCaptureGameSession(args []string) int {
	fs := flag.NewFlagSet("capture-game-session", flag.ExitOnError)
	app := fs.String("application", "", "the application whose window to capture")
	session := fs.String("session", "", "session name, e.g. rocketleague-menu-steps")
	seconds := fs.Int("seconds", 45, "how long to record")
	fps := fs.Int("fps", 10, "frames per second to attempt")
	outDir := fs.String("out", ".tmp/game-capture-review", "scratch directory (must not be a corpus)")
	delay := fs.Duration("delay", 5*time.Second, "wait before recording, to switch to the game")
	_ = fs.Parse(flagsFirst(args))

	if *app == "" || *session == "" {
		fmt.Fprintln(os.Stderr,
			"director: --application and --session are both required\n"+
				"  example: director capture-game-session --application rocketleague "+
				"--session rocketleague-menu-steps --seconds 45")
		return 2
	}
	if *fps < 1 || *fps > 60 {
		fmt.Fprintln(os.Stderr, "director: --fps must be between 1 and 60")
		return 2
	}
	// The same guard capture-vision-fixture carries, for the same reason: a capture is
	// private until a person has looked at it, and the one mistake that matters is writing
	// unreviewed frames where the benchmark reads from.
	if isCorpusPath(*outDir) {
		fmt.Fprintf(os.Stderr,
			"director: %s looks like a durable corpus. Captures are private until a person "+
				"has reviewed them; write them to a scratch directory instead.\n", *outDir)
		return 2
	}

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

	dir := filepath.Join(*outDir, *session)
	if err := os.MkdirAll(filepath.Join(dir, "frames"), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}

	pads := padsource.New()
	if why := pads.Unavailable(); why != "" {
		// Said plainly and not fatally: frames alone are still evidence, and somebody may
		// want them. But a session recorded without the input half cannot answer the
		// question this command exists for, so it must not look like one that did.
		fmt.Printf("WARNING: %s\n", why)
		fmt.Printf("         Frames will be recorded; controller inputs will not.\n\n")
	}

	interval := time.Second / time.Duration(*fps)
	fmt.Printf("Recording %s → %s\n", v.Ref.Describe(), dir)
	fmt.Printf("  %ds at about %d frames per second, starting in %s\n", *seconds, *fps, *delay)
	fmt.Printf("  Open the menu and walk the highlight through it. Nothing is injected,\n")
	fmt.Printf("  nothing is approved, and nothing here decides what is selected.\n\n")
	time.Sleep(*delay)

	pads.Start(ctx)
	start := time.Now()
	deadline := start.Add(time.Duration(*seconds) * time.Second)

	// Encoding runs OFF the capture clock, which is the difference between a usable
	// recording and a useless one.
	//
	// Measured on this machine against a real 1920x1080 Rocket League frame: 476ms to
	// encode at Go's default compression, about 130ms to capture. A recorder that encodes
	// inline therefore runs at about 1.6 frames per second whatever it was asked for — and
	// it reports that honestly, because the times are measured, which is how this was
	// caught. But a menu selection changes faster than 600ms, so every transition would
	// fall between two frames and the recording could not answer the question it exists
	// for.
	//
	// png.BestSpeed costs 89ms for about 20% more bytes, and moving it onto workers takes
	// it off the clock altogether. What is left is the capture cost.
	type pending struct {
		frame gameFrame
		img   image.Image
	}
	queue := make(chan pending, encodeQueue)
	var encMu sync.Mutex
	var written []gameFrame
	var encodeErrs []error
	var wg sync.WaitGroup
	for range encodeWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			enc := png.Encoder{CompressionLevel: png.BestSpeed}
			for p := range queue {
				err := encodeFrame(enc, filepath.Join(dir, filepath.FromSlash(p.frame.File)), p.img)
				encMu.Lock()
				if err != nil {
					encodeErrs = append(encodeErrs, err)
				} else {
					written = append(written, p.frame)
				}
				encMu.Unlock()
			}
		}()
	}

	var inputs []gameInput
	captured, droppedFrames := 0, 0
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lost := ""
	for time.Now().Before(deadline) {
		// Drain whatever the controller reported since the last frame. Done here rather
		// than on another goroutine so the two logs are written by one thread and their
		// order needs no reasoning about.
		for draining := true; draining; {
			select {
			case e, ok := <-pads.Events():
				if !ok {
					draining = false
					break
				}
				inputs = append(inputs, gameInput{Intent: e.Intent, AtMS: e.AtMS})
			default:
				draining = false
			}
		}

		// Re-validate every frame. A window that closed mid-session must stop the capture
		// rather than quietly produce frames of whatever inherited the handle.
		v := tracker.Acquire(ctx, *app)
		if !v.State.OK() {
			lost = fmt.Sprintf("target lost (%s)", v.State)
			break
		}
		at := time.Since(start).Milliseconds()
		img, err := shots.CaptureWindow(ctx, windowRefToWindow(v.Ref))
		if err != nil {
			// A failed capture is a gap in the record, not a reason to stop: the session
			// is a person playing a game, and it cannot be re-run identically.
			fmt.Printf("  %6dms: %v\n", at, err)
			<-ticker.C
			continue
		}
		index := captured
		rel := filepath.ToSlash(filepath.Join("frames", fmt.Sprintf("%s-%04d.png", *session, index)))
		b := img.Image.Bounds()
		p := pending{
			frame: gameFrame{
				Index: index, File: rel, AtMS: at,
				Width: b.Dx(), Height: b.Dy(),
				WindowGeneration: v.Ref.Generation,
			},
			img: img.Image,
		}
		select {
		case queue <- p:
			captured++
		default:
			// The encoders are behind. A dropped frame is an honest gap in a record
			// whose times are measured; blocking here would stretch the interval
			// instead, which hides the problem inside data that still looks regular.
			droppedFrames++
		}
		if index%*fps == 0 {
			fmt.Printf("  %6dms: %d frames, %d inputs\n", at, captured, len(inputs))
		}
		<-ticker.C
	}
	close(queue)
	wg.Wait()
	// Written in capture order. The workers finish out of order and that is fine for
	// files, but a log whose lines were not in time order would invite somebody to read
	// adjacency as sequence.
	sort.Slice(written, func(i, j int) bool { return written[i].Index < written[j].Index })

	pads.Stop()
	// Whatever the controller reported after the last frame still belongs to the session.
	for e := range pads.Events() {
		inputs = append(inputs, gameInput{Intent: e.Intent, AtMS: e.AtMS})
	}

	meta := gameSession{
		Session: *session, Application: *app,
		Privacy: "captured_private",
		Note: "Frames and observed navigation intents on one clock. NOTHING here states " +
			"what is selected; that is annotation, and it is a separate act by a person. " +
			"Times are milliseconds from the first frame, measured rather than scheduled.",
		StartedUTC:  start.UTC().Format(time.RFC3339),
		RequestedMS: int(interval.Milliseconds()),
		Frames:      len(written), Inputs: len(inputs),
		DroppedIn:     pads.Dropped(),
		DroppedFrames: droppedFrames + len(encodeErrs),
		AchievedFPS:   achievedFPS(written),
		Controller:    controllerNote(pads, len(inputs)),
	}
	if err := writeSessionRecord(dir, meta, written, inputs); err != nil {
		fmt.Fprintf(os.Stderr, "director: writing the session: %v\n", err)
		return 1
	}

	if lost != "" {
		fmt.Printf("\nStopped early: %s\n", lost)
	}
	fmt.Printf("\n%d frame(s) at %.1f fps measured, %d input(s), all marked captured_private.\n",
		len(written), meta.AchievedFPS, len(inputs))
	if meta.DroppedFrames > 0 {
		fmt.Printf("%d frame(s) were dropped because the encoders fell behind.\n", meta.DroppedFrames)
	}
	if pads.Dropped() > 0 {
		fmt.Printf("%d input(s) were dropped because the recorder fell behind.\n", pads.Dropped())
	}
	if len(inputs) == 0 {
		fmt.Printf("NO INPUTS WERE OBSERVED. %s\n", meta.Controller)
	}
	fmt.Printf("NOTHING is annotated and NOTHING is approved:\n  %s\n", dir)
	return 0
}

// controllerNote says what the input half of this session is worth.
func controllerNote(pads *padsource.Source, inputs int) string {
	switch {
	case pads.Unavailable() != "":
		return pads.Unavailable()
	case inputs > 0:
		return "observed"
	case pads.Connected():
		return "a controller was connected but nothing navigational was pressed"
	}
	return "no controller was connected; if you played with the keyboard, this session has " +
		"no input half and cannot answer what followed what"
}

// writeSessionRecord writes the three files, kept apart on purpose.
//
// Frames, inputs and the session's own description are separate documents because they are
// separate claims, and because an annotation pass will add a fourth beside them rather than
// editing any of these.
func writeSessionRecord(dir string, meta gameSession, frames []gameFrame, inputs []gameInput) error {
	if err := writeJSON(filepath.Join(dir, "session.json"), meta); err != nil {
		return err
	}
	if err := writeJSONL(filepath.Join(dir, "frames.jsonl"), len(frames), func(i int) any {
		return frames[i]
	}); err != nil {
		return err
	}
	return writeJSONL(filepath.Join(dir, "input.jsonl"), len(inputs), func(i int) any {
		return inputs[i]
	})
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// writeJSONL writes one record per line, which is what an append-shaped log should be even
// when it is written all at once: a reader can take the first n lines of a truncated file.
func writeJSONL(path string, n int, at func(int) any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	for i := range n {
		if err := enc.Encode(at(i)); err != nil {
			_ = f.Close()
			return err
		}
	}
	return f.Close()
}
