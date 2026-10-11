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
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
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
	// Front says whether the TARGET was the window in front when this frame was taken:
	// "target", "other", or "" when nobody could say.
	//
	// A window capture is not occlusion-safe - it can return the pixels of whatever is
	// covering the target, under the target's own name - and no identity guard sees it.
	// Experiment-025 captured the Windows taskbar on one look of 107 with provenance
	// clean and target_losses 0. A recording without this field cannot say which of its
	// frames are evidence about the application, and a replay of it would reprocess
	// somebody else's window as though it were the game.
	Front string `json:"front,omitempty"`
	// Scale is the DPI scaling of the captured surface, and BoundsX/Y where the window
	// was. Carried because geometry read from these pixels is only placeable with them,
	// and a replay that assumed 1.0 on a scaled display would misplace every row.
	//
	// Position only, never size: Width and Height above are the frame's own. A window
	// that MOVED between frames is visible here, which is the case that silently
	// invalidates a comparison of two frames' geometry.
	Scale   float64 `json:"scale,omitempty"`
	BoundsX int     `json:"bounds_x,omitempty"`
	BoundsY int     `json:"bounds_y,omitempty"`
}

// gameInput is one navigation intent, as observed.
//
// Intent and time, which is the whole record. Which button produced it never reaches this
// process — see internal/platform/padsource.
type gameInput struct {
	Intent observe.NavIntent `json:"intent"`
	AtMS   int64             `json:"at_ms"`
	// Device is which hardware the intent was observed from, from the closed vocabulary
	// in observe.NavDevice: keyboard, mouse or controller. Never WHICH key - the privacy
	// boundary is unchanged.
	//
	// It was dropped on the way to disk, and a recording that cannot say where an input
	// came from cannot answer the question a mixed session raises: whether the keyboard
	// half was observed at all, or whether quiet means nobody pressed anything.
	Device observe.NavDevice `json:"device,omitempty"`
	// Observed is always true and is written anyway.
	//
	// It is the field that keeps a DEMONSTRATION distinguishable from an ACTUATION in
	// the record itself rather than in a note somebody has to read. Nothing in this
	// command can inject; if Marco ever records a session in which it acted, those
	// events must be a different shape, and a reader of an old recording must not have
	// to infer which kind it is holding.
	Observed bool `json:"observed"`
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
	// Keyboard says what the keyboard and mouse half of this session is worth, the same
	// way Controller does. A session recorded without one cannot answer what followed
	// what for somebody who played with the keyboard, and must not look like one that
	// could.
	Keyboard string `json:"keyboard"`
	// CaptureFailures is captures that ERRORED, kept apart from DroppedFrames, which is
	// captures that succeeded and were never written because the encoders were behind.
	// One is a fact about the window and the other about this process, and they were
	// summed into a single number that could not distinguish them.
	CaptureFailures int `json:"capture_failures"`
	// FramesBehindAnotherWindow is frames taken while something else was in front. They
	// are KEPT, because a person reviewing the recording should see them, and they are
	// counted so a replay can refuse them without having to discover them.
	FramesBehindAnotherWindow int `json:"frames_behind_another_window"`
	// Actuation states, in the record, that nothing here injected anything. See
	// gameInput.Observed: the distinction between what the person did and what Marco
	// did must be readable from an old recording without inferring it.
	Actuation string `json:"actuation"`
	// Retention is the bound this recording was taken under and what happens at it.
	// A capture that silently filled a disk is a capture nobody will authorise twice.
	Retention string `json:"retention"`
	// StoppedBecause is why recording ended: the clock, the window, or the bound.
	StoppedBecause string `json:"stopped_because"`
	// Components is what produced this recording. A replay comparing two sessions needs
	// to know whether it is comparing screens or builds.
	Components map[string]string `json:"components"`
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

// How much a recording may take, and what happens at the bound.
//
// A 1920x1080 PNG at BestSpeed is roughly 2MB, so 864 frames of a real session is about 1.4GB
// on this machine. That is a lot to produce by accident and more to produce twice, and a
// capture that silently filled a disk is a capture nobody will authorise again.
//
// So the bound is explicit, it is written into the session record, and reaching it STOPS the
// recording rather than overwriting or thinning it: a recording that quietly dropped its middle
// would be a timeline with a hole nobody could see. The frames already written stay.
const (
	// maxRecordedFrames bounds a session at roughly ten minutes at 15fps.
	maxRecordedFrames = 9000
	// maxRecordedBytes bounds it at 6GB, whichever comes first.
	maxRecordedBytes = 6 << 30
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

	// THE KEYBOARD AND MOUSE HALF, which this command did not have.
	//
	// A recording of somebody playing with the keyboard had no input half at all, and
	// said so only in a note about controllers. Device provenance travels with every
	// event, so a replay can tell a quiet keyboard from an unwatched one.
	//
	// navsource installs low-level hooks and owns its own blocking message pump; this
	// only opens a subscription and drains it, exactly as the observation sampler does.
	// Nothing here touches the hook thread.
	navs, navWhy := navsource.New()
	var navSub *navsource.Subscription
	if navWhy != "" {
		fmt.Printf("WARNING: %s\n", navWhy)
		fmt.Printf("         Keyboard and mouse input will NOT be recorded.\n\n")
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
	if navs != nil {
		navSub = navs.Open(time.Now())
		defer navs.CloseSession(navSub)
		defer func() { _ = navs.Close() }()
	}
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
	captured, droppedFrames, captureFailures := 0, 0, 0
	behind, bytesWritten := 0, int64(0)
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
				inputs = append(inputs, gameInput{Intent: e.Intent, AtMS: e.AtMS,
					Device:   deviceOr(e.Device, observe.DeviceController),
					Observed: true})
			default:
				draining = false
			}
		}

		// AND THE KEYBOARD AND MOUSE, drained on the same thread as the controller so
		// the two logs are written by one goroutine and their order needs no reasoning.
		if navSub != nil {
			for _, e := range navSub.Drain() {
				inputs = append(inputs, gameInput{Intent: e.Intent, AtMS: e.AtMS,
					Device: deviceOr(e.Device, observe.DeviceKeyboard), Observed: true})
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
			captureFailures++
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
				// WHICH WINDOW THESE PIXELS ARE OF, asked per frame. A capture by
				// window can return the covering window's pixels under the target's
				// own name, and no identity guard sees it.
				Front:   string(frontnessOf(frontCandidate(ctx, windows), v.Ref)),
				Scale:   img.Scale,
				BoundsX: img.Bounds.X, BoundsY: img.Bounds.Y,
			},
			img: img.Image,
		}
		if p.frame.Front == string(observe.FrontOther) {
			behind++
		}
		select {
		case queue <- p:
			captured++
			// A FRAME'S WORTH OF BYTES, estimated rather than measured, because the
			// encoders finish out of order and a bound that waited for them would let
			// the queue overrun it. Four bits a pixel is about what PNG BestSpeed
			// gives on a game frame; the bound is a safety limit, not an accountant.
			bytesWritten += int64(b.Dx()) * int64(b.Dy()) / 2
		default:
			// The encoders are behind. A dropped frame is an honest gap in a record
			// whose times are measured; blocking here would stretch the interval
			// instead, which hides the problem inside data that still looks regular.
			droppedFrames++
		}
		// THE RETENTION BOUND. Stops rather than thinning or overwriting: a recording
		// that quietly dropped its middle would be a timeline with a hole nobody could
		// see, and the frames already written are still evidence.
		if captured >= maxRecordedFrames || bytesWritten >= maxRecordedBytes {
			lost = fmt.Sprintf("the retention bound (%d frames or %dGB)",
				maxRecordedFrames, maxRecordedBytes>>30)
			break
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
		DroppedIn: pads.Dropped(),
		// Encoder failures count as dropped frames; CAPTURE failures do not, and are
		// their own number. One is a fact about this process and the other about the
		// window, and summing them produced a figure that could not say which.
		DroppedFrames:             droppedFrames + len(encodeErrs),
		CaptureFailures:           captureFailures,
		FramesBehindAnotherWindow: behind,
		AchievedFPS:               achievedFPS(written),
		Controller:                controllerNote(pads, len(inputs)),
		Keyboard:                  keyboardNote(navWhy, inputs),
		Actuation: "none; every input in this recording was OBSERVED from hardware. " +
			"Nothing here can inject, and the controller reader does not bind the " +
			"call that could. See gameInput.observed.",
		Retention: fmt.Sprintf("bounded at %d frames or %dGB, whichever comes first; "+
			"reaching either STOPS the recording and keeps what was written. Delete "+
			"the directory to discard it; nothing here copies it anywhere.",
			maxRecordedFrames, maxRecordedBytes>>30),
		StoppedBecause: stoppedBecause(lost),
		Components: map[string]string{
			"go":        runtime.Version(),
			"os_arch":   runtime.GOOS + "/" + runtime.GOARCH,
			"recorder":  "director capture-game-session",
			"input_pad": "padsource",
			"input_nav": navComponent(navWhy),
		},
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
	if captureFailures > 0 {
		fmt.Printf("%d capture(s) FAILED, which is a gap in the record and not an "+
			"encoder falling behind.\n   Those two were one number before.\n",
			captureFailures)
	}
	if behind > 0 {
		fmt.Printf("%d frame(s) were taken while ANOTHER window was in front. They are "+
			"kept and counted;\n   a window capture can return the covering window's "+
			"pixels.\n", behind)
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

// deviceOr fills in a producer's device when it did not say.
//
// A fallback and not an invention: each source here watches exactly one kind of hardware, so
// the default is a fact about which subscription the event came out of rather than a guess about
// the event. An empty device on disk would be indistinguishable from an unwatched one.
func deviceOr(d, fallback observe.NavDevice) observe.NavDevice {
	if d.Known() {
		return d
	}
	return fallback
}

// frontCandidate is the window in front right now, or the zero candidate.
//
// Asked per frame rather than once, like everything in the window layer: a cached answer
// describes where the person was looking a moment ago, and the whole value of this is that it is
// true at the instant the frame it describes was taken.
func frontCandidate(ctx context.Context, p windowref.Platform) windowref.Candidate {
	c, res, _ := windowref.Foreground(ctx, p)
	if res != windowref.Resolved {
		return windowref.Candidate{}
	}
	return c
}

// keyboardNote says what the keyboard half of this session is worth.
//
// The same shape as controllerNote and for the same reason: a session recorded without one
// cannot answer what followed what, and must not look like one that could.
func keyboardNote(unavailable string, inputs []gameInput) string {
	if unavailable != "" {
		return unavailable
	}
	for _, in := range inputs {
		if in.Device == observe.DeviceKeyboard || in.Device == observe.DeviceMouse {
			return "observed"
		}
	}
	return "watched, and nothing navigational was pressed on it"
}

// navComponent names the keyboard producer, or says there was none.
func navComponent(unavailable string) string {
	if unavailable != "" {
		return "none (" + unavailable + ")"
	}
	return "navsource"
}

// stoppedBecause says why recording ended, in the record rather than only on the terminal.
func stoppedBecause(lost string) string {
	if lost == "" {
		return "the requested duration elapsed"
	}
	return lost
}
