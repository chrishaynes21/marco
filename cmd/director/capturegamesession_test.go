package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
)

// Non-actuating: nothing here captures a window or reads a controller. What is tested is the
// shape of the record and the guards that run before any of that starts.

func TestAGameSessionWillNotWriteIntoTheCorpus(t *testing.T) {
	// The one mistake that matters: unreviewed frames of somebody's game where the
	// benchmark reads from. Checked before a window is acquired, so this needs no desktop.
	for _, out := range []string{"fixtures", "fixtures/vision/v2/rocketleague", "docs/experiments"} {
		code := runCaptureGameSession([]string{
			"--application", "rocketleague", "--session", "s", "--out", out,
		})
		if code != 2 {
			t.Errorf("--out %s should be refused, got exit %d", out, code)
		}
	}
}

func TestAGameSessionNeedsToBeToldWhatToRecord(t *testing.T) {
	if code := runCaptureGameSession([]string{"--session", "s"}); code != 2 {
		t.Errorf("no --application should be refused, got %d", code)
	}
	if code := runCaptureGameSession([]string{"--application", "rocketleague"}); code != 2 {
		t.Errorf("no --session should be refused, got %d", code)
	}
}

func TestTheFrameRateIsBounded(t *testing.T) {
	// A session is a person playing a game for a minute. 0 fps records nothing and 600 fps
	// fills a disk with duplicate frames of a static menu; neither is a mistake worth
	// discovering afterwards.
	for _, fps := range []string{"0", "-1", "61", "1000"} {
		code := runCaptureGameSession([]string{
			"--application", "rocketleague", "--session", "s", "--fps", fps,
			"--out", t.TempDir(),
		})
		if code != 2 {
			t.Errorf("--fps %s should be refused, got exit %d", fps, code)
		}
	}
}

// TestTheRecordSaysNothingAboutWhatIsSelected is the load-bearing claim of this command.
//
// ADR-144's gap is filled by a person annotating frames, not by the recorder guessing. The
// brightness measurement that found the gap is a hypothesis about one game's menu; if the
// recorder wrote it down, the corpus's ground truth and the detector's cue would share an
// ancestor and the benchmark would agree with itself.
//
// So: whatever this command writes, no part of it may name selection.
func TestTheRecordSaysNothingAboutWhatIsSelected(t *testing.T) {
	dir := t.TempDir()
	err := writeSessionRecord(dir,
		gameSession{Session: "s", Application: "rocketleague", Frames: 2, Inputs: 1},
		[]gameFrame{{Index: 0, File: "frames/s-0000.png", AtMS: 0, Width: 1920, Height: 1080},
			{Index: 1, File: "frames/s-0001.png", AtMS: 103, Width: 1920, Height: 1080}},
		[]gameInput{{Intent: "down", AtMS: 51}})
	if err != nil {
		t.Fatalf("writing the session: %v", err)
	}

	forbidden := []string{"select", "highlight", "chosen", "focus", "active_entry", "brightness"}
	for _, name := range []string{"session.json", "frames.jsonl", "input.jsonl"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s was not written: %v", name, err)
		}
		body := strings.ToLower(string(b))
		for _, word := range forbidden {
			// session.json's own note explains that it states nothing about selection,
			// which is the one place the word is allowed to appear.
			if name == "session.json" {
				continue
			}
			if strings.Contains(body, word) {
				t.Errorf("%s mentions %q; recording must not annotate", name, word)
			}
		}
	}
}

func TestTheThreeLogsAreSeparateDocuments(t *testing.T) {
	// Frames, inputs and the session description are separate claims, and an annotation
	// pass will add a fourth file beside them rather than editing any of these. A single
	// merged log would make that impossible without rewriting the evidence.
	dir := t.TempDir()
	if err := writeSessionRecord(dir, gameSession{Session: "s"},
		[]gameFrame{{Index: 0, File: "frames/s-0000.png", AtMS: 7}},
		[]gameInput{{Intent: "confirm", AtMS: 9}}); err != nil {
		t.Fatalf("writing the session: %v", err)
	}
	for _, name := range []string{"session.json", "frames.jsonl", "input.jsonl"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s is missing: %v", name, err)
		}
	}

	// And the times survive exactly as given. They are measured from the clock during the
	// session; a log that rounded or re-derived them would destroy the only thing that
	// makes a press and a frame orderable.
	line, err := os.ReadFile(filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var f gameFrame
	if err := json.Unmarshal(line, &f); err != nil {
		t.Fatalf("a frames.jsonl line must be one JSON object: %v", err)
	}
	if f.AtMS != 7 {
		t.Errorf("frame time %d, want the 7 it was given", f.AtMS)
	}
}

// TestTheSessionReportsTheRateItAchievedNotTheOneItWasAsked is the third way a capture can
// look valid and mislead.
//
// Measured on a real Rocket League window: encoding a 1920x1080 frame at Go's default
// compression costs 476ms against about 130ms to capture, so the first version of this
// command ran at 1.6 frames per second when asked for 5. The times were honest — they are
// measured — but a reader skimming `requested_frame_interval_ms` would have concluded the
// recording could show a menu transition, and it could not.
func TestTheSessionReportsTheRateItAchievedNotTheOneItWasAsked(t *testing.T) {
	ten := []gameFrame{{Index: 0, AtMS: 0}, {Index: 1, AtMS: 100}, {Index: 2, AtMS: 200}}
	if got := achievedFPS(ten); got < 9.9 || got > 10.1 {
		t.Errorf("three frames 100ms apart is 10 fps, got %.2f", got)
	}
	slow := []gameFrame{{Index: 0, AtMS: 0}, {Index: 1, AtMS: 725}, {Index: 2, AtMS: 1400}}
	if got := achievedFPS(slow); got > 2 {
		t.Errorf("the rate the original defect produced must not read as fast, got %.2f", got)
	}
	// Degenerate inputs must not invent a rate. A session that captured one frame has no
	// measurable rate, and reporting one would be the same class of fiction.
	for _, c := range [][]gameFrame{nil, {{Index: 0, AtMS: 0}}, {{AtMS: 5}, {AtMS: 5}}} {
		if got := achievedFPS(c); got != 0 {
			t.Errorf("%v has no measurable rate, got %.2f", c, got)
		}
	}
}

// TestFrameTimesAreReadFromTheClockNotTheSchedule guards the defect this command was written
// to avoid repeating.
//
// `capture-vision-fixture` records offset_ms as index × interval — what the schedule intended.
// For frames alone that is harmless. Here it would mis-order the one pair that matters, since
// encoding a 1920x1080 PNG costs tens of milliseconds and the real gap is not the asked-for
// one. Structural, because reaching this behaviourally means capturing a real window.
func TestFrameTimesAreReadFromTheClockNotTheSchedule(t *testing.T) {
	src, err := os.ReadFile("capturegamesession.go")
	if err != nil {
		t.Fatalf("reading the command: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, "at := time.Since(start).Milliseconds()") {
		t.Error("frame times must be measured from the session clock")
	}
	if strings.Contains(s, "time.Duration(i) * *interval") {
		t.Error("a frame time derived from the schedule is a fiction; measure it")
	}
	// And encoding stays off the clock. Inline encoding is what held the recorder to 1.6
	// fps, and it is a one-line regression away: png.Encode in the capture loop.
	if !strings.Contains(s, "CompressionLevel: png.BestSpeed") {
		t.Error("default PNG compression costs 476ms a frame here and governs the rate")
	}
	if !strings.Contains(s, "queue <- p:") {
		t.Error("frames must be handed to encoder workers, not encoded on the capture clock")
	}
}

// TestAFullEncoderQueueDropsRatherThanStretchingTheTimeline is the choice behind the bound.
//
// Blocking on a full queue would keep every frame and silently lengthen the interval, so the
// record would still look regular while no longer describing when anything happened. A
// dropped frame is a gap somebody can count, and the session says how many.
func TestAFullEncoderQueueDropsRatherThanStretchingTheTimeline(t *testing.T) {
	src, err := os.ReadFile("capturegamesession.go")
	if err != nil {
		t.Fatalf("reading the command: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, "droppedFrames++") {
		t.Error("a frame that could not be queued must be counted")
	}
	if !strings.Contains(s, `DroppedFrames int `) {
		t.Error("the session record must carry the dropped count; an uncounted gap is a lie")
	}
	// The bound itself: an unbounded queue holds 8MB per pending frame and this machine
	// has killed processes for memory during a test run.
	if encodeQueue <= 0 || encodeQueue > 32 {
		t.Errorf("encodeQueue = %d; it bounds roughly %dMB of pending frames",
			encodeQueue, encodeQueue*8)
	}
}

// TestTheGameSessionCommandIsReachable is the wiring claim.
//
// Eight recorded cases in this repository of complete, correct code that production never
// invoked. A capture command nothing dispatches to would be the ninth, and it would look
// exactly like a working feature in every file listing.
func TestTheGameSessionCommandIsReachable(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading the dispatch: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, `case "capture-game-session":`) {
		t.Error("main.go dispatches no capture-game-session verb")
	}
	if !strings.Contains(s, "runCaptureGameSession(os.Args[2:])") {
		t.Error("the capture-game-session verb does not call the command")
	}
}

// TestAGameRecordingGivesThePointerAFrame is the defect that would have wasted a session.
//
// # What was broken
//
// `navsource` normalises a pointer press against the watched window's rectangle and REFUSES
// any press it cannot place — `windowBounds.fresh` is false until somebody pushes one. This
// command opened a nav subscription and never pushed a rectangle, so every mouse press it
// ever saw was counted `unplaceable_pointer` and dropped WHOLE. The recording said `0 inputs`.
// A session recorded to learn what a person clicked would have contained no clicks, and the
// failure is indistinguishable from "nobody pressed anything" in the output.
//
// The same defect, at the live sampler, is TestADefaultDirectorGivesThePointerAFrame. Two
// sites, one rule, and this one stayed broken because every session so far used a controller.
//
// Structural for the reason the other tests here are: reaching it behaviourally means
// capturing a real window and pressing a real mouse button.
//
// Deleting the SetWindowBounds call, or the Where field it exists to fill, must fail this.
func TestAGameRecordingGivesThePointerAFrame(t *testing.T) {
	src, err := os.ReadFile("capturegamesession.go")
	if err != nil {
		t.Fatalf("reading the command: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, "navs.SetWindowBounds(") {
		t.Error("the recorder pushes no window rectangle, so navsource can place no " +
			"pointer press and every click is refused as unplaceable_pointer. The " +
			"recording will say nobody touched the mouse")
	}
	// THE CAPTURE'S rectangle, not the reference's. A press is a fraction of the
	// rectangle it was placed against and a row is a fraction of the rectangle the frame
	// was captured from; if those differ by a window frame, the press lands a few percent
	// off, which is enough to name the neighbouring row and look like an answer.
	if !strings.Contains(s, "navs.SetWindowBounds(img.Bounds.X, img.Bounds.Y,") {
		t.Error("the rectangle pushed must be the one these pixels came from. A press " +
			"placed against the window rect and a row measured in the captured " +
			"surface are fractions of two different rectangles")
	}
	// And the position has to reach disk, which is the half that looks fine in a log:
	// `point` with no `where` is a record that somebody clicked and no record of what.
	if !strings.Contains(s, `Where observe.PointerAt `+"`"+`json:"where,omitzero"`+"`") {
		t.Error("gameInput carries no position, so a pointer press reaches disk as an " +
			"intent with no place")
	}
	if !strings.Contains(s, "Where: e.Where,") {
		t.Error("the drain does not copy the event's position onto the record")
	}
}

// TestEveryRecordedInputSaysItWasObserved is the one claim the record must never get wrong.
//
// `observed` is what keeps a DEMONSTRATION distinguishable from an ACTUATION in the file
// itself rather than in a note somebody has to read. The tail that drained the controller
// after the last frame wrote `gameInput{Intent, AtMS}` — no device, and `observed` false by
// omission, which in this format states that Marco produced the input. Nothing in this
// command can inject.
//
// Checked over the literals rather than behaviourally: a controller press arriving in the
// window between the last frame and the stop is not reachable from a test.
func TestEveryRecordedInputSaysItWasObserved(t *testing.T) {
	src, err := os.ReadFile("capturegamesession.go")
	if err != nil {
		t.Fatalf("reading the command: %v", err)
	}
	s := string(src)
	for i := 0; ; {
		j := strings.Index(s[i:], "gameInput{")
		if j < 0 {
			break
		}
		at := i + j
		// The literal runs to its closing brace; `Observed` must be inside it.
		end := strings.Index(s[at:], "})")
		if end < 0 {
			end = len(s) - at - 1
		}
		lit := s[at : at+end]
		if !strings.Contains(lit, "Observed: true") && !strings.Contains(lit, "Observed:\ttrue") {
			t.Errorf("a gameInput literal does not say it was observed:\n\t%s\n"+
				"An input recorded with observed=false states that MARCO pressed "+
				"it, and nothing here can inject", strings.TrimSpace(lit))
		}
		i = at + len("gameInput{")
	}
}

// TestTheSessionSaysWhatTheMouseHalfIsWorth keeps the two silences apart.
//
// "Nobody clicked" and "every click was discarded" produced an identical recording: no point
// intents, and a refusal count nothing wrote down. One is a fact about the person and the
// other is a defect in Marco, and a reader deciding whether to record again needs to know
// which they are holding.
func TestTheSessionSaysWhatTheMouseHalfIsWorth(t *testing.T) {
	src, err := os.ReadFile("capturegamesession.go")
	if err != nil {
		t.Fatalf("reading the command: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, `Pointer string `+"`"+`json:"pointer"`+"`") {
		t.Error("the session record says nothing about the mouse")
	}
	if !strings.Contains(s, "observe.IgnoreUnplaceablePointer") {
		t.Error("the note must read the refusal count. Without it a recorder that " +
			"placed nothing reports a quiet session")
	}
	if !strings.Contains(s, "Pointer:                   pointerNote(navs, inputs),") {
		t.Error("the note is computed and never stored, which is this repository's " +
			"eighth-recorded failure: correct code nothing invokes")
	}
}

// TestTheMouseNoteTellsAQuietSessionFromADiscardedOne is the behavioural half of the defect.
//
// The structural tests above say the frame is pushed and the position is written. This one
// drives the real producer through the state this command was actually in — a session open, no
// window rectangle ever pushed — and checks that the note the recording carries says which of
// the two silences it is.
//
// Both halves matter. "Nobody clicked" is a fact about the person and ends the investigation;
// "every click was refused" is a defect in Marco and must not be allowed to read as the first.
func TestTheMouseNoteTellsAQuietSessionFromADiscardedOne(t *testing.T) {
	s, press := navsource.NewSyntheticPointer()
	defer func() { _ = s.Close() }()
	sub := s.Open(time.Now())
	defer s.CloseSession(sub)

	// THE STATE THIS COMMAND SHIPPED IN: watching, with no idea where the window is.
	press(300, 350)
	waitReceived(t, s, 1)
	if n := s.Stats().Ignored[observe.IgnoreUnplaceablePointer]; n != 1 {
		t.Fatalf("a press with no window frame was refused %d time(s), want 1; "+
			"stats %+v", n, s.Stats())
	}
	if got := sub.Peek(); got != 0 {
		t.Fatalf("%d unplaceable press(es) reached the subscription", got)
	}
	note := pointerNote(s, nil)
	if !strings.Contains(note, "defect") {
		t.Errorf("a session that refused every click reports %q. A reader cannot tell "+
			"that from a session nobody clicked in, and the two have different "+
			"causes and different fixes", note)
	}

	// AND WITH A FRAME, the identical press is recorded with its position.
	s.SetWindowBounds(100, 200, 400, 300, time.Now())
	press(300, 350)
	waitReceived(t, s, 2)
	var inputs []gameInput
	for _, e := range sub.Drain() {
		inputs = append(inputs, gameInput{Intent: e.Intent, AtMS: e.AtMS,
			Device: deviceOr(e.Device, observe.DeviceKeyboard),
			Where:  e.Where, Observed: true})
	}
	if len(inputs) != 1 {
		t.Fatalf("a press inside a framed window produced %d recorded input(s), want 1; "+
			"stats %+v", len(inputs), s.Stats())
	}
	if inputs[0].Where.X != 0.5 || inputs[0].Where.Y != 0.5 {
		t.Errorf("a press at the centre of the window reached the record at %.3f,%.3f; "+
			"a click with no place is a record that somebody clicked and no "+
			"record of what they clicked",
			inputs[0].Where.X, inputs[0].Where.Y)
	}
	note = pointerNote(s, inputs)
	if !strings.Contains(note, "1 press") {
		t.Errorf("the note for a session with one recorded press reads %q", note)
	}
	// AND IT STILL REPORTS THE ONE IT COULD NOT PLACE. A recording that mentions only
	// its successes is how a partial loss becomes invisible.
	if !strings.Contains(note, "NOT in this recording") {
		t.Errorf("the note %q does not say that a press was lost. A session that "+
			"placed some clicks and dropped others must say so", note)
	}
}
