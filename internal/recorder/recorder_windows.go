//go:build windows

package recorder

import (
	"fmt"
	"image"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/screen"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// clickTemplateRadius is the half-size of the FALLBACK patch captured around a left-click
// — used only when button recognition can't isolate a control at the click (a flat panel,
// a freeform canvas). The normal path captures the whole desktop and crops to the detected
// button (screen.AutoCropAt), re-centring the anchor on it; the fallback patch is centred
// on the click, so locating it and clicking its centre reproduces the click. Tune with
// $MARCO_CLICK_RADIUS (pixels; the patch is 2× this).
var clickTemplateRadius = clickRadiusFromEnv()

// captureAnchors gates per-click anchor capture. Default OFF — the CV anchor stack (match a
// moving control by image/colour/edge) isn't reliable enough yet, so it's behind a feature
// flag: learning records plain coordinates + timings unless CV is explicitly turned on
// ($MARCO_CV=on/max, or $MARCO_ANCHORS=1/on). See anchorsEnabled.
var captureAnchors = anchorsEnabled()

// cvKitchenSink is the MARCO_CV=max mode — the one switch that throws EVERY signal at
// EVERY click for a maximally robust (and maximally A/B-testable) capture: anchor every
// button (not just left), and capture even a non-distinctive patch so the OCR/Vision
// resolvers can still label it. Run-time scoring stays conservative (it won't move off
// the recorded point without strong, corroborated evidence), so "kitchen sink" means more
// signals captured, never less-safe clicking.
var cvKitchenSink = cvMode() == "max"

// cvMode reads the MARCO_CV master switch: "max" → the kitchen sink (above); "off" →
// plain coordinates only; "" → leave the individual knobs (MARCO_ANCHORS, MARCO_OCR, …)
// at their defaults. setup.ps1 -CV / -NoCV flip it so the whole CV stack toggles at once.
func cvMode() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_CV"))) {
	case "max", "1", "on", "all", "true", "yes":
		return "max"
	case "0", "off", "false", "no", "none":
		return "off"
	default:
		return ""
	}
}

func anchorsEnabled() bool {
	switch cvMode() {
	case "max":
		return true // CV explicitly on → anchors on, regardless of MARCO_ANCHORS
	case "off":
		return false // CV explicitly off → plain coordinates
	}
	// CV is a feature flag that's OFF BY DEFAULT (the anchor stack isn't reliable yet). Anchors
	// turn on only when explicitly asked via MARCO_ANCHORS.
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_ANCHORS"))) {
	case "1", "on", "true", "yes":
		return true
	}
	return false
}

func clickRadiusFromEnv() int {
	if v := os.Getenv("MARCO_CLICK_RADIUS"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 8 && n <= 1200 {
			return n
		}
	}
	// Half-size of the fallback patch (radius 100 → ~200px) used when no button is cleanly
	// detected at the click; the normal path crops to the recognised button instead. Kept
	// generous so the click-centred fallback is still distinctive. Override $MARCO_CLICK_RADIUS.
	return 100
}

// New returns the Windows recorder (global low-level keyboard + mouse hooks).
func New() Recorder { return &winRecorder{} }

// Package-level capture state — the hook callbacks are C callbacks and can only
// reach package state. Only one recorder is active at a time.
var (
	// recMu guards the capture state below AND the session number that says whose it is.
	//
	// ONE LOCK FOR BOTH, because a worker asking whether the state is still its own and
	// then writing into it is two moments, and the state can be replaced in between. The
	// session was raised BESIDE this lock rather than under it, and every fence was read
	// BEFORE the lock was taken — so a worker could pass its check, be descheduled, and
	// publish into the recording that replaced it. That is what this lock closes: a
	// publication decides and writes under one acquisition, and `beginRecording` takes the
	// same lock to raise the session and empty the buffer it describes.
	recMu     sync.Mutex
	recBuf    []RecordedEvent
	recPub    chan RecordedEvent
	recActive atomic.Bool
	// recSession is which recording the global state above belongs to, raised by every
	// Start, under recMu.
	//
	// # Why a session number and not a flag
	//
	// Because the workers outlive a shutdown that could not complete. The app-switch
	// poller's one blocking call is a foreground-window read, which does not return while
	// the window's owning thread is not pumping; the capture worker is doing screen work
	// and image encoding. `Stop` is bounded, so it gives up and returns — and those two
	// goroutines are still running, still holding an index into a buffer, and still able
	// to publish through this package's state.
	//
	// So each of them takes the session it belongs to when it starts and compares before
	// writing. An old poller's late app switch is then dropped instead of appearing in a
	// later recording, and an old capture cannot write a picture into whatever event
	// happens to be at its index now.
	//
	// THE COMPARISON AND THE WRITE ARE ONE STEP, under recMu — see `mineUnderLock`, and
	// `beginRecording` for the half of that which is about the replacement. A fence read
	// before the lock is a fence a worker can be past while the recording is replaced.
	//
	// See [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].
	recSession atomic.Int64
	// recWedged is set when a shutdown gave up on hooks it could not remove, and no
	// further recording is allowed in this process.
	//
	// THE HOOKS CANNOT BE FENCED FROM HERE, which is why this is a refusal rather than
	// another session check. They are C callbacks reached by the operating system with no
	// context of their own; they gate on `recActive`, and the next Start would set that
	// true again — at which point an abandoned recording's hooks, still installed because
	// the thread that owns them never answered, would feed the NEW recording. A session
	// number in Go cannot reach them.
	//
	// So a recording that could not be stopped ends recording for the life of the
	// process, and says that a restart is what fixes it. That is the honest consequence
	// of hooks that are still installed, and it is what the shutdown's own message
	// already said: "They stay installed until this process exits."
	recWedged     atomic.Bool
	lastMoveNanos atomic.Int64
	// foreApp is the app poller's last sample, published so the HOOK can tag an event
	// with the application it landed in without making a process query of its own.
	// The hooks are synchronous and shared with the whole desktop; a lookup there is
	// how Windows comes to drop them. An empty value means no sample yet, which is a
	// real answer and is never guessed at.
	foreApp atomic.Value
)

// foreground returns the app poller's last sample, or "" before the first one.
func foreground() string {
	if v, ok := foreApp.Load().(string); ok {
		return v
	}
	return ""
}

const moveThrottle = 8 * time.Millisecond

// appPollInterval is how often the recorder samples the foreground app to detect
// switches (clicking a taskbar icon, Alt-Tab, etc.). Switches become EvAppSwitch
// events so simplify can turn "however you got to the app" into a robust
// Activate, instead of recording brittle navigation clicks.
const appPollInterval = 120 * time.Millisecond

type winRecorder struct {
	done    chan struct{}
	stopped chan struct{} // hook pump finished
	// ready closes once the pump publishes its thread id; a blocking pump is woken by a
	// posted WM_QUIT rather than by a closed channel.
	ready       chan struct{}
	threadID    atomic.Uint32
	pollStopped chan struct{} // app poller finished
	// session is which recording this is, so its workers can tell whether the global
	// state still belongs to them. See recSession.
	session        int64
	captureStopped chan struct{} // click-template capture worker finished
	// mode is what this recorder was TOLD to capture, or Inherited when nothing was.
	//
	// Per RECORDER rather than per package, because stating a mode is something a caller
	// does about one recording. `Start` publishes it to `plainCoordinates`, which is where
	// the hook callbacks can reach it — see the capture state below.
	mode Mode
	// watch is which windows this recording admits input from and target is WHICH
	// window, both from the caller that started it. Published by `Start` the same way and
	// for the same reason. See Confined.
	watch  Watching
	target Target
}

// Only states what the recording `Start` begins next admits. See recorder.Confined.
func (r *winRecorder) Only(w Watching, t Target) { r.watch, r.target = w, t }

// Capture states what this recorder captures, overriding what the process was launched with.
//
// See recorder.Mode. Called before Start; a recorder is reusable and the mode stays until it is
// stated again, which is the same lifetime the environment-read variables have.
func (r *winRecorder) Capture(m Mode) { r.mode = m }

// plainCoordinates is "the recording that is running was told to capture coordinates only".
//
// Package-level and atomic for the same reason every other piece of capture state here is: the
// hook callbacks are C callbacks and can reach nothing but package state. Written by `Start` from
// the recorder's own `mode`, so what a CALLER stated is what the hooks read — and read on every
// click rather than once at process start, which is the whole defect this closes.
var plainCoordinates atomic.Bool

// ── WHICH WINDOW THIS RECORDING IS OF ────────────────────────────────────────
//
// Atomics for the same reason `plainCoordinates` is one: the hook callbacks are C callbacks
// and can reach nothing but package state. All of it written by `Start`, before the hooks go
// on, from what the caller stated.
//
// # The target is GIVEN, and was once inferred
//
// The first version LATCHED it: the first foreground window that was neither the surface the
// recording was started from nor its process became the target. That is not a window somebody
// CHOSE — it is whichever window came forward first, so a terminal, a notification toast or
// an updater taking focus in the second after Record was pressed became the thing being
// recorded, and the demonstration that followed was admitted from nowhere. A person cannot
// correct a choice they were never shown. So the window is selected BEFORE capture, by the
// person, and arrives here as two numbers.
var (
	// confined is "this recording admits one window only".
	confined atomic.Bool
	// targetHWnd and targetPID are the chosen window. BOTH decide membership: a handle
	// Windows has handed back out is a different window, and a recording that asked only
	// about the handle would carry on admitting input from it.
	targetHWnd atomic.Uintptr
	targetPID  atomic.Uint32
	// targetApp is the chosen window's application name, resolved ONCE at Start and off
	// the hook thread, because reading it opens a process handle.
	//
	// It is what an admitted event is tagged with, which is the other half of a defect:
	// a key used to carry `foreground()`, the app-switch poller's last SAMPLE, so a key
	// pressed within the poll interval of a switch was labelled with the application the
	// person had left. The chosen window's name cannot be wrong about the chosen window.
	targetApp atomic.Value
	// gone is set once the chosen window has stopped existing, so the admission gate
	// stops asking about a handle Windows may already have given to somebody else.
	gone atomic.Bool
)

// alreadyHeld is every key and mouse button that is PHYSICALLY DOWN right now.
//
// # Why a recording has to ask
//
// Because a recording begins in the middle of somebody's day. Ctrl held when Record is pressed
// is a press this recording will never see, and the release — if it comes at all — arrives with
// nothing to pair it with. `Boundary` tracked only what it watched go down, so it started empty
// and believed nothing was held: a ctrl+click performed in the chosen window was admitted and
// written down as an ordinary `click`, which is a different demonstration. It is the same defect
// as entering the window with Ctrl down, one step earlier, and the boundary could not see it
// because the press happened before there was a boundary.
//
// So the physical state is read ONCE, at Start, and seeded as held-from-outside: the rule that
// already refuses an admitted action while something is held from outside then covers this too,
// and a key released without ever being pressed is dropped instead of recorded.
//
// # The whole keyboard, and why that is affordable
//
// `GetAsyncKeyState` is one read per virtual key and this asks about 254 of them, which is a
// burst of cheap reads ONCE, off the hook thread, before the hooks are installed. The
// alternative — a short list of modifiers — is the mistake this slice already corrected in
// `Boundary`: Q held across a boundary is a crossing, and Q held before Start is the same fact
// earlier. The high bit is "down now"; the low bit is "pressed since last asked", which is a
// different question and would report a key somebody finished pressing as one that is stuck.
//
// A variable, so a test can state a desktop where something is held. Production never replaces
// it.
// imageOfProcess is a process's executable name. A variable so a test can state one without a
// process to read; production never replaces it.
var imageOfProcess = winctx.ProcessImage

var alreadyHeld = func() []string {
	var held []string
	for vk := 1; vk <= 254; vk++ {
		// The mouse buttons are VK 1,2,4 and are named as the boundary names them,
		// because a button held across the start is the same fact as a key.
		//
		// THE HIGH BIT IS "DOWN NOW". The low bit is "pressed since last asked",
		// which is a different question and the wrong one: it would report a key
		// somebody finished pressing a moment ago as one that is stuck, and refuse
		// the recording for it.
		//
		// WHICH BIT THIS IS CANNOT BE EXERCISED HEADLESSLY and the mutation that
		// swaps it SURVIVES: establishing it needs a key physically held while the
		// scan runs, which is the same class of gap as physical capture and is
		// recorded as one rather than contrived against. `plugins/e2e`
		// `isHeldDown` reads the same bit for the same reason with the same note,
		// and the desktop harness there does hold keys down.
		state, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
		if state&0x8000 == 0 {
			continue
		}
		switch vk {
		case 1:
			held = append(held, "the left button")
		case 2:
			held = append(held, "the right button")
		case 4:
			held = append(held, "the middle button")
		default:
			if n := vkToName(uint16(vk)); n != "" {
				held = append(held, n)
			}
		}
	}
	return held
}

// chosenApp is the chosen window's application name, and "" when there is no chosen window or
// its image could not be read.
//
// SEPARATE FROM `appOfTarget` BECAUSE THE FALLBACK IS NOT ALWAYS RIGHT. Tagging an admitted
// event with a stale sample is a wrong label on an event that did happen in the window; writing
// an application switch from one is a step for an application nobody demonstrated in. So the
// poller asks THIS, which has no fallback: an established identity or none.
func chosenApp() string {
	if !confined.Load() {
		return ""
	}
	v, _ := targetApp.Load().(string)
	return v
}

// appOfTarget is the chosen window's application name, or the poller's sample when there is no
// chosen window (an unconfined recording, which is every recording there was).
func appOfTarget() string {
	if v := chosenApp(); v != "" {
		return v
	}
	return foreground()
}

// bound is the crossing state of the recording in flight, or nil when nothing is confined.
//
// NOT an atomic, because unlike everything above it is MUTATED by what it is asked: a press
// changes what is held, and the decision and the mutation are one step. It lives under `recMu`,
// which `observe` takes anyway to append — so the boundary costs the hook thread no lock it was
// not already taking.
var bound *Boundary

// whose is ONE reading of who owns this instant: the foreground window and the process that
// owns it, taken together.
//
// # One reading, because two readings are two instants
//
// A click used to ask `clickAttribution` for the window it landed in, and the admission gate
// asked the foreground AGAIN for itself. Between those two calls the foreground can change —
// that is what a click on another window DOES — so one event could be attributed to one window
// and admitted on the strength of another. A key was worse: it was tagged with `foreground()`,
// the app-switch poller's last sample, and admitted on a third reading. So there is ONE snapshot
// per event and everything about that event is decided from it.
//
// Both are window-data reads, which is the class of call that is safe on the low-level hook
// thread: no process handle is opened and no image path is queried. The application NAME is not
// read here for exactly that reason — see `appOfTarget`, which answers it from the chosen
// window instead of from a sample.
//
// `ok` false means the foreground or its owner could not be read. That is UNKNOWN, and unknown
// is never membership.
type whose struct {
	hwnd uintptr
	pid  uint32
	ok   bool
}

// attributionNow is that one reading.
//
// A variable so a test can drive the production gate against a known desktop — the pattern
// `clickAttribution` set in this file, and for the same reason: a rule about WHICH WINDOW cannot
// be exercised against the one window a test process has. Production never replaces it, and the
// hooks call it once per event.
var attributionNow = func() whose {
	hwnd, pid, ok := winctx.ForegroundProcess()
	return whose{hwnd: hwnd, pid: pid, ok: ok}
}

// admits answers whether one reading of the desktop belongs to the chosen window.
//
// THE HANDLE AND THE OWNER, both, out of the one snapshot: the handle because two windows of one
// application are two different windows, and the owner because Windows hands a handle back out
// when a window closes. Either one alone admits input from a window nobody chose.
func (w whose) admits() bool {
	if !confined.Load() {
		return true
	}
	// THE UNREADABLE ARM IS UNFALSIFIABLE AND STAYS ANYWAY.
	//
	// `Start` only sets `confined` when the target has both halves, so the comparison
	// below already refuses a reading with a zero in it: 0 cannot equal a handle or an
	// owner that exists. A mutation deleting `!w.ok || w.hwnd == 0 || w.pid == 0` changes
	// no behaviour reachable from production, and the gate run for it is recorded as
	// unfalsifiable rather than as a kill.
	//
	// It stays because it is the honest expression of "unknown is not membership" at the
	// point where the answer is read, and because the two guards protect different things
	// if either ever moves: this one is about the READING, and the comparison is about the
	// TARGET.
	if gone.Load() || !w.ok || w.hwnd == 0 || w.pid == 0 {
		return false
	}
	return w.hwnd == targetHWnd.Load() && w.pid == targetPID.Load()
}

// targetLost reports whether the chosen window has stopped being itself, and latches that
// answer so it is asked of the operating system once.
//
// Called from the app-switch poller and never from a hook: it opens no process handle, but it
// is two window reads for a question that only changes when a window closes, and the poller is
// already off the hook thread.
func targetLost() bool {
	if !confined.Load() || gone.Load() {
		return gone.Load()
	}
	h, p := targetHWnd.Load(), targetPID.Load()
	if h == 0 {
		return false
	}
	if winctx.WindowAlive(h, p) {
		return false
	}
	gone.Store(true)
	return true
}

// anchoring reports whether this recording captures a picture of what was pressed.
//
// `captureAnchors` is the environment, read at process start. `plainCoordinates` is a caller
// saying so about THIS recording, and it wins: a stated mode an inherited variable could
// overrule would not be a stated mode. Every point that would capture, crop, colour-sample or
// listen for the anchor key asks this one question, so a fourth such point cannot be added
// without answering it.
func anchoring() bool { return captureAnchors && !plainCoordinates.Load() }

// captureReq asks the off-thread worker to snapshot the patch around a click and
// write it back onto the already-recorded event at idx. The capture is done off
// the hook thread because BitBlt against the live screen can spike to tens of ms
// (worse while the GPU composites an overlay); doing it inline in the LL mouse
// hook would blow past Windows' LowLevelHooksTimeout, and Windows silently drops
// a hook that's too slow — taking the keyboard hook (and thus the F12 stop key)
// down with it.
type captureReq struct {
	idx   int
	x, y  int
	armed bool // explicit anchor (tap-then-click): capture the whole screen to crop
}

// captureReqs carries click-template work from the hook thread to the worker.
// Buffered so a burst of clicks never blocks the hook; over-full just drops the
// template (the click degrades to a plain coordinate), never the click itself.
var captureReqs chan captureReq

// anchorKey is the key you TAP to start an image anchor; the next left-click ends it
// — that click is captured as an image anchor. One-handed: tap, then click, no holding.
// "" disables it. Gated on captureAnchors: with CV off (the alpha default) we don't
// listen for it at all, so F12 stays a normal key that reaches the game.
var anchorKey = func() string {
	if !captureAnchors {
		return ""
	}
	return AnchorKey()
}()

// anchorArmed is set by a tap of the anchor key and consumed by the next click, which is then
// captured as a template.
//
// # Not "even when MARCO_ANCHORS is off", which is what this said
//
// It said the arming survives the anchor switch being off, and that was true when it was written
// and is not true now. A recording that STATES coordinate mode takes the picture away: the gate
// is `!plainCoordinates.Load() && anchorArmed.Load()`, so an armed click in a stated coordinate
// recording captures nothing.
//
// The distinction is the one [[ADR-254-what-a-recording-captures-is-stated]] turns on. The
// SWITCH is a setting of a process — `$MARCO_ANCHORS`, read once at start-up — and the arming
// does still survive that, which is what the sentence was about. The stated MODE is a claim about
// one recording, and it wins: a routine saved in coordinate mode may not come back carrying a
// picture, because a press found by matching one cannot be stopped at and would make the whole
// guarded routine unsaveable.
//
// Comment only; see `recordClickDown` for the gate itself and for why it reads
// `plainCoordinates` rather than `anchoring()`.
var anchorArmed atomic.Bool

// stopKey is the recording's stop gesture, read from $MARCO_STOP_KEY (default F12).
// When the anchor key is the same key, stopping wins — the key isn't treated as an
// anchor, so the demo can still be ended (see keyboardProc).
var stopKey = ParseStopKey(os.Getenv("MARCO_STOP_KEY"))

func (r *winRecorder) Start() error {
	// A SHUTDOWN THAT COULD NOT COMPLETE ENDS RECORDING FOR THIS PROCESS. See recWedged:
	// the abandoned recording's hooks are still installed and gate on `recActive`, which
	// the lines below are about to set true again — so a second recording would be fed by
	// the first one's hooks, and no amount of fencing in Go can reach a C callback.
	//
	// Asked FIRST, before anything is published or installed, so a refused Start leaves
	// the process exactly as it was.
	if recWedged.Load() {
		return fmt.Errorf("this Marco cannot record again: a previous recording's " +
			"keyboard and mouse hooks could not be taken off, so anything " +
			"recorded now would be fed by them as well. Close Marco and open it " +
			"again")
	}
	r.beginRecording()
	lastMoveNanos.Store(0)
	recActive.Store(true)
	// WHAT THIS RECORDING CAPTURES, FROM THE CALLER THAT STARTED IT. Published before
	// the hooks go on, because after them the first press can already have been seen.
	plainCoordinates.Store(r.mode == Coordinates)
	// AND WHICH WINDOW, from the same caller and for the same reason. The surface that
	// started this is in front right now, which is what makes it the one window the
	// target can never be — see latchedTarget.
	confined.Store(r.watch == OneWindow && r.target.Chosen())
	targetHWnd.Store(r.target.HWnd)
	targetPID.Store(r.target.PID)
	gone.Store(false)

	// AND NOTHING IS LEFT ARMED FROM A RECORDING BEFORE THIS ONE. The arm is a one-shot
	// consumed by the next click, so an unconsumed one outlives the recording it was
	// tapped in — which would put a picture in the first click of a recording that asked
	// for none.
	anchorArmed.Store(false)

	r.done = make(chan struct{})
	r.stopped = make(chan struct{})
	r.ready = make(chan struct{})
	r.pollStopped = make(chan struct{})
	r.captureStopped = make(chan struct{})
	captureReqs = make(chan captureReq, 256)

	// Capture worker: snapshots the click target off the hook thread and writes the
	// PNG back onto the recorded event, so the synchronous LL hooks return instantly
	// (see captureReq). An explicit (armed) anchor grabs the WHOLE screen — you crop
	// it down to the target afterwards (keep the target centered, since the match
	// clicks the patch centre). Global MARCO_ANCHORS mode grabs the small auto-patch.
	// Drains on Stop.
	go func() {
		defer close(r.captureStopped)
		for req := range captureReqs {
			var img []byte
			var color string
			clickLX, clickLY := 0, 0
			newX, newY, recenter := req.x, req.y, false
			if req.armed {
				// Explicit anchor: keep the whole frame to crop AND read the pixel under
				// the click, so the anchor gets both a template and a colour resolver from
				// the one capture. The frame is the primary screen at origin (0,0), so the
				// click's position within it is just its absolute coordinate.
				if frame := captureFullFrameRGBA(); frame != nil {
					if data, err := screen.EncodePNG(frame); err == nil {
						img = data
						clickLX, clickLY = req.x, req.y
					}
					if col, ok := screen.ColorAt(frame, req.x, req.y); ok {
						color = fmt.Sprintf("0x%06X", col)
					}
				}
			} else if frame, ox, oy, err := screen.CaptureVirtual(); err == nil && frame != nil {
				// Auto-anchor: capture the WHOLE desktop so a button bigger than a fixed
				// patch fits, then crop to the button containing the click and re-centre the
				// recorded click on that button — the template's centre is the click target
				// the matcher resolves. A flat / non-distinctive spot drops the anchor (the
				// click stays a plain coordinate). ox,oy map the absolute click into the frame.
				lx, ly := req.x-ox, req.y-oy
				tmpl, cgx, cgy, clx, cly := screen.AutoCropAt(frame, lx, ly, clickTemplateRadius)
				if tmpl != nil && (cvKitchenSink || screen.Distinctive(tmpl)) {
					if col, ok := screen.ColorAt(frame, cgx, cgy); ok {
						color = fmt.Sprintf("0x%06X", col)
					}
					if data, encErr := screen.EncodePNG(tmpl); encErr == nil {
						img = data
						clickLX, clickLY = clx, cly
					}
					if nx, ny := cgx+ox, cgy+oy; nx != req.x || ny != req.y {
						newX, newY, recenter = nx, ny, true
					}
				}
			}
			if img == nil {
				continue
			}
			// AND IS THIS STILL OUR RECORDING? Asked inside `attach`, under the one
			// lock that writes: a capture that came back after a shutdown gave up on
			// it would otherwise put a picture into whatever event is at its index in
			// a LATER recording's buffer.
			r.attach(captured{idx: req.idx, img: img, color: color,
				clickLX: clickLX, clickLY: clickLY,
				x: newX, y: newY, recenter: recenter})
		}
	}()

	// App-switch poller: sample the foreground app and emit EvAppSwitch on change
	// (including an initial one for the app the demo starts in). Runs off the hook
	// thread so it never stalls the synchronous LL hooks.
	go func() {
		defer close(r.pollStopped)
		last := ""
		// ONE DECISION, ASKED TWICE. The first sample and every later one differ
		// only in what `last` is — "" before anything has been seen — so a second
		// copy of the condition here is a second place for it to drift, and it did:
		// the session fence was added to one of them first. See sampled.
		//
		// AND THE TIME IS TAKEN AFTER THE SAMPLE, which is what the left-to-right
		// evaluation of these arguments means: `winctx.Active()` is the call that
		// blocks, so the instant worth recording is the one it came back in.
		last = r.sampled(winctx.Active(), last, time.Now())
		t := time.NewTicker(appPollInterval)
		defer t.Stop()
		for {
			select {
			case <-r.done:
				return
			case <-t.C:
				// THE CHOSEN WINDOW'S OWN DEATH, asked here because it is two
				// window reads for a question that changes once, and the hook
				// thread must not spend them on every keystroke. The recording
				// ENDS: `waitForStop` returns on this marker, the draft is kept
				// for review, and a gesture left unfinished by the closing has
				// already made it unsaveable.
				if targetLost() && r.recordTargetGone(time.Now()) {
					return
				}
				last = r.sampled(winctx.Active(), last, time.Now())
			}
		}
	}()

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(r.stopped)

		// Published before the hooks exist, so a Stop racing Start still has a thread to wake.
		tid, _, _ := procGetCurrentThreadId.Call()
		r.threadID.Store(uint32(tid))
		close(r.ready)

		kbCB := syscall.NewCallback(keyboardProc)
		msCB := syscall.NewCallback(mouseProc)
		kbHook, _, _ := procSetWindowsHookExW.Call(whKeyboardLL, kbCB, 0, 0)
		msHook, _, _ := procSetWindowsHookExW.Call(whMouseLL, msCB, 0, 0)

		// BLOCKING, for the reason set out in internal/platform/navsource: Windows can only
		// deliver a low-level hook callback while the installing thread waits on messages, and
		// PeekMessage with a Sleep between polls leaves it asleep instead — which puts up to a
		// scheduler quantum of latency on every keystroke and every mouse move on the whole
		// desktop. It is not a dropped hook and never looks like one; it just feels heavy.
		var m msg
		for {
			ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if ret == 0 || int32(ret) == -1 {
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
		if kbHook != 0 {
			procUnhookWindowsHookEx.Call(kbHook)
		}
		if msHook != 0 {
			procUnhookWindowsHookEx.Call(msHook)
		}
	}()
	return nil
}

func (r *winRecorder) Stop() []RecordedEvent {
	if r.done == nil {
		return nil
	}
	close(r.done)
	// WAKE THE BLOCKING PUMP, AND KEEP KNOCKING UNTIL IT ANSWERS.
	//
	// Only the installing thread may unhook what it installed, and it is asleep in
	// GetMessage until a message arrives — so shutdown is a message rather than a flag.
	//
	// THE POST CAN LAND NOWHERE. A Windows thread has no message queue until it first
	// calls a message function, and PostThreadMessage to a thread without one fails. The
	// id is published before the hooks exist so that a Stop racing a Start still has
	// somewhere to aim, and that is the window: a single unchecked post into it is lost,
	// GetMessage waits forever for a message that will never come, and so does this.
	//
	// It is not hypothetical and it was not cheap. `cmd/marco` hit its 600-second limit
	// twice on this, both times recorded as an unexplained timeout with contention as the
	// guess — and the same defect had already been found and fixed in
	// `internal/platform/navsource`, whose own suite ran 495s before being killed. This
	// is that fix carried to the site that still had it. Every real-input run goes through
	// it: `withPanicStop` starts a recording to watch for the stop key and stops it when
	// the play ends, so a short play is exactly the shape that posts into the window.
	//
	// Bounded, because a pump that has somehow already died must not wedge the caller
	// either, and because a shutdown that cannot be completed is a thing to SAY.
	// ── AND THE TWO WAITS EITHER SIDE OF IT ARE BOUNDED TOO ──
	//
	// The pump wake above has been bounded since the 600-second timeouts. These two were
	// not, and one of them is how a completed run came to report nothing at all.
	//
	// WHAT WAS SEEN, on 2026-10-27: `plugins/studio`'s own suite, under the load of its
	// headless-browser tests, intermittently left a played routine unfinished. The child
	// had run the program — its last log line was the routine's own — and then said
	// nothing for thirty seconds, with no outcome word and the window's one run slot still
	// claimed. Roughly one run in six. In isolation it finishes in half a second.
	//
	// WHAT THAT POINTS AT, and it is a pointer rather than a proof: `<-r.pollStopped`
	// waits for the app-switch poller, whose one blocking call is `winctx.Active()` — a
	// foreground-window read. A window whose owning thread is not pumping makes a
	// window-data read block, and a desktop full of browser windows coming and going is
	// exactly that desktop. The poller checks `r.done` between ticks, so it finishes
	// "either way" only once that call returns.
	//
	// So both receives get the same treatment the pump wake has, for the same reason given
	// there: a shutdown that cannot be completed is a thing to SAY. A run that finished
	// its program must report that it finished — leaving the caller wedged is the one
	// outcome that is wrong whatever the cause — and each bound has its OWN sentence, so
	// the next occurrence says which wait expired instead of needing this paragraph again.
	//
	// NOT A FIX CLAIMED. Nothing here establishes why the poller was late; it establishes
	// that a late poller no longer costs the run its report. See
	// [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].
	if !waitedFor(r.ready, hookShutdownBound) {
		fmt.Fprintf(os.Stderr, "\n[marco: this recording's hook thread never reported "+
			"that it had started, so its hooks could not be taken off. They stay "+
			"installed until this process exits. Nothing more is being recorded.]\n")
		return r.abandon()
	}
	woken := false
	deadline := time.Now().Add(5 * time.Second)
	for {
		procPostThreadMessageW.Call(uintptr(r.threadID.Load()), wmQuit, 0, 0)
		select {
		case <-r.stopped: // hook pump unhooked — no more mouseProc, so no more capture sends
			woken = true
		case <-time.After(20 * time.Millisecond):
		}
		// The deadline is checked HERE and not in the select's case, where a `break`
		// would leave the select and go round the loop again forever.
		if woken || time.Now().After(deadline) {
			break
		}
	}
	if !waitedFor(r.pollStopped, hookShutdownBound) {
		fmt.Fprintf(os.Stderr, "\n[marco: this recording's app-switch poller did not "+
			"come back — it is most likely inside a foreground-window read that has "+
			"not returned. The recording is stopped and what was captured is "+
			"complete; the poller stops when that read returns.]\n")
	}
	if !woken {
		// REPORTED, NOT RECOVERED. See abandon for what is and is not done, and why
		// nothing is forced.
		fmt.Fprintf(os.Stderr, "\n[marco: this recording's keyboard and mouse hooks could "+
			"not be taken off — the thread that installed them did not answer. They stay "+
			"installed until this process exits. Nothing more is being recorded.]\n")
		return r.abandon()
	}
	close(captureReqs) // safe now: nothing sends after the hooks are gone
	// AND THE TEMPLATE WRITER, BOUNDED LIKE THE OTHER TWO.
	//
	// It finishes when `captureReqs` closes, and what it does in between is screen capture
	// and image encoding — work that can be slow and whose one failure mode is the same as
	// the poller's: a call into the window system that has not come back. The same rule
	// applies for the same reason, and what is lost by giving up is the template of a press
	// whose picture was still being written, which is a thing to SAY rather than a thing to
	// wait on for ever. See hookShutdownBound.
	if !waitedFor(r.captureStopped, hookShutdownBound) {
		fmt.Fprintf(os.Stderr, "\n[marco: a picture of one press was still being "+
			"written when this recording stopped, and has not finished. The "+
			"demonstration is complete apart from that one picture.]\n")
	}
	recActive.Store(false)
	recMu.Lock()
	out := recBuf
	recBuf = nil
	recMu.Unlock()
	// AND THE PROCESS IMAGES, NOW THAT THERE IS NO HOOK LEFT TO SLOW DOWN.
	//
	// The hooks are unhooked by this point, so opening a process handle costs nobody
	// anything. Doing it here rather than in the callback is the whole reason the
	// process id is recorded at the press and the name is not. See resolveImages.
	resolveImages(out, winctx.ImageOf)
	r.done = nil
	return out
}

// Paused answers the live capture state. See recorder.Pausable.
//
// UNDER `recMu`, which is the lock the boundary is mutated by: a surface that read the two
// halves of this separately could draw "paused" with the time it came back, and a poll that
// landed mid-event could read a boundary halfway through being moved.
func (r *winRecorder) Paused() (bool, time.Duration) {
	recMu.Lock()
	defer recMu.Unlock()
	if bound == nil || !r.mineUnderLock() {
		return false, 0
	}
	outside, since := bound.Outside()
	if !outside || since.IsZero() {
		// OUTSIDE WITH NO INSTANT is the state before the person has ever arrived:
		// they have chosen a window and not gone to it yet, which is not a pause in a
		// recording and must not be shown as one.
		return false, 0
	}
	return true, time.Since(since)
}

func (r *winRecorder) Events() <-chan RecordedEvent {
	recMu.Lock()
	defer recMu.Unlock()
	return recPub
}

// record appends an event and forwards it to the live channel (non-blocking).
func record(ev RecordedEvent) { recordIndexed(ev) }

// recordIndexed is record that also returns the event's index in recBuf, so the
// capture worker can later fill in a click's Image in place.
//
// It is the UNCONFINED path and the markers' path: an event given to it is recorded and
// published, which is what every event did before there was a chosen window. Input from the
// hooks goes through `observe`.
func recordIndexed(ev RecordedEvent) int {
	recMu.Lock()
	idx := appendUnderLock(ev)
	pub := recPub
	recMu.Unlock()
	publish(pub, ev)
	return idx
}

// appendUnderLock appends to the buffer and returns the index. `recMu` MUST BE HELD.
func appendUnderLock(ev RecordedEvent) int {
	idx := len(recBuf)
	recBuf = append(recBuf, ev)
	return idx
}

// publish forwards an event to the live channel, never blocking: a consumer that is not
// reading loses events rather than stalling a hook callback.
func publish(pub chan RecordedEvent, ev RecordedEvent) {
	if pub == nil {
		return
	}
	select {
	case pub <- ev:
	default:
	}
}

// observe decides what becomes of one event the hooks saw, and does it.
//
// THE DECISION AND THE WRITES ARE ONE STEP, under `recMu`. The boundary is mutable state that
// every event changes, so asking it outside the lock and writing inside would be two instants
// with another hook callback's answer possible in between — the shape of defect this file
// already carries two long comments about. `inside` is read BEFORE the lock, because it is a
// syscall and `recMu` is held by workers doing screen work.
//
// Returns the event's index in the buffer, or -1 when it was not recorded — which is what stops
// a dropped click from queueing a screen capture of somebody else's window.
func observe(ev RecordedEvent, inside bool) int {
	recMu.Lock()
	b := bound
	if b == nil {
		idx := appendUnderLock(ev)
		pub := recPub
		recMu.Unlock()
		publish(pub, ev)
		return idx
	}
	was := b.Crossing()
	markers := make([]RecordedEvent, 0, 2)
	if k, changed := b.Arrived(inside, ev.T); changed {
		markers = append(markers, RecordedEvent{Kind: k, T: ev.T})
	}
	var adm Admission
	switch ev.Kind {
	case EvKey:
		adm = b.Pressed(ev.KeyName, ev.Down, inside, stopKey.Has(ev.KeyName))
	case EvClick:
		adm = b.Pressed("the "+ev.Button+" button", ev.Down, inside, false)
	case EvMove:
		adm = b.Moved(inside)
	default:
		adm = b.Admits(inside)
	}
	// THE CROSSING IS AN EVENT, not state read back afterwards. The buffer is what
	// `Stop` returns and the only thing a draft is made from, so a reason carried
	// anywhere else would be a second answer that could outlive its recording — and
	// a surface watching the channel learns at the instant it happened.
	if now := b.Crossing(); now != was {
		markers = append(markers, RecordedEvent{Kind: EvCrossed, KeyName: now, T: ev.T})
	}
	// AND A KEY STRANDED BY THE BOUNDARY, which is a fact and not yet a refusal. It goes
	// into the buffer AFTER the EvLeft above, so whoever reads the stream can see which
	// absence stranded it. See EvHeldAcross.
	for _, id := range b.HeldAcross() {
		markers = append(markers, RecordedEvent{Kind: EvHeldAcross, KeyName: id, T: ev.T})
	}
	idx := -1
	for _, m := range markers {
		appendUnderLock(m)
	}
	if adm == Admit {
		idx = appendUnderLock(ev)
	}
	pub := recPub
	recMu.Unlock()
	for _, m := range markers {
		publish(pub, m)
	}
	// AND OFF-TARGET INPUT REACHES THE CHANNEL ONLY AS A STOP. See recorder.StopOnly:
	// a consumer of this channel is as entitled not to hold somebody's message as the
	// buffer is, and the whole of what a stop needs is the stop keys themselves.
	if adm != Drop {
		publish(pub, ev)
	}
	return idx
}

// keyboardProc is the WH_KEYBOARD_LL callback. It records but never suppresses —
// the user's keystrokes must reach the app they're demonstrating in.
func keyboardProc(nCode int, wParam, lParam uintptr) uintptr {
	if nCode >= 0 && recActive.Load() {
		kb := (*kbdllHookStruct)(unsafe.Pointer(lParam))
		if kb.flags&llkhfInjected == 0 {
			down := wParam == wmKeyDown || wParam == wmSysKeyDown
			up := wParam == wmKeyUp || wParam == wmSysKeyUp
			if down || up {
				vk := uint16(kb.vkCode)
				name := numpadEnter(vkToName(vk), vk, kb.flags)
				// The anchor key is a recorder-only modifier: a TAP arms an anchor (the
				// next click is captured), so it's never recorded and is swallowed so the
				// app never sees it. Skip this when the anchor key is also the stop key —
				// then it must fall through to be recorded so the demo can be stopped.
				// AND THE ANCHOR KEY IS AN ORDINARY KEY IN A COORDINATE
				// RECORDING. Belt and braces, recorded as such: the gate
				// in `recordClickDown` already refuses to honour an arm in
				// this mode, so a mutation removing THIS one changes no
				// behaviour and the gate run for it is recorded as
				// unfalsifiable rather than as a kill. It stays because it
				// is the honest expression of the rule at the point where
				// the key is read, and because the two gates protect
				// different things if either ever moves.
				if anchoring() && anchorKey != "" && name == anchorKey && !stopKey.Has(name) {
					if down {
						anchorArmed.Store(true) // tap to start; the next click ends it
					}
					return 1 // suppress — never reaches the app
				}
				// WHOSE KEY IS THIS? One reading, taken before the lock
				// because it is a window read and `recMu` is held by
				// workers doing screen work, and used for BOTH the
				// admission and the tag. The key hook made no such read
				// before this, which is the cost of the rule and is
				// recorded as such.
				//
				// `appOfTarget` and not `foreground()`: the poller's sample
				// labelled a key with the application the person had just
				// LEFT whenever the press landed inside the poll interval.
				w := attributionNow()
				observe(RecordedEvent{Kind: EvKey, VK: vk, KeyName: name,
					Down: down, T: time.Now(), App: appOfTarget()},
					w.admits())
			}
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

// mouseProc is the WH_MOUSE_LL callback. Moves are throttled in-callback to keep
// the synchronous hook from stalling.
func mouseProc(nCode int, wParam, lParam uintptr) uintptr {
	if nCode >= 0 && recActive.Load() {
		ms := (*msllHookStruct)(unsafe.Pointer(lParam))
		if ms.flags&llmhfInjected == 0 {
			x, y := int(ms.pt.x), int(ms.pt.y)
			switch wParam {
			case wmMouseMove:
				now := time.Now().UnixNano()
				if now-lastMoveNanos.Load() >= int64(moveThrottle) {
					lastMoveNanos.Store(now)
					// THE THROTTLE FIRST, and the window read only for a
					// sample that would be recorded: the gate is a syscall
					// and a move arrives on every pixel of travel.
					observe(RecordedEvent{Kind: EvMove, X: x, Y: y,
						T: time.Now()}, attributionNow().admits())
				}
			case wmLButtonDown:
				recordClickDown("left", x, y)
			case wmLButtonUp:
				recordClickUp("left", x, y)
			case wmRButtonDown:
				recordClickDown("right", x, y)
			case wmRButtonUp:
				recordClickUp("right", x, y)
			case wmMButtonDown:
				recordClickDown("middle", x, y)
			case wmMButtonUp:
				recordClickUp("middle", x, y)
			case wmXButtonDown:
				if b := sideButton(ms.mouseData); b != "" {
					recordClickDown(b, x, y)
				}
			case wmXButtonUp:
				if b := sideButton(ms.mouseData); b != "" {
					recordClickUp(b, x, y)
				}
			case wmMouseWheel, wmMouseHWheel:
				recordScroll(x, y, int(int16(ms.mouseData>>16)), wParam == wmMouseHWheel)
			}
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

// recordClickDown records a button-press click (fast) and captures its template
// off-thread — never BitBlt inside the hook (see captureReq) — when anchors are on
// globally (left clicks, or EVERY button in kitchen-sink mode) OR the anchor key armed
// this click (tap-then-click, any button). An armed click ends the anchor and disarms. A
// full capture queue just drops the template; the click itself is already recorded.
func recordClickDown(button string, x, y int) {
	// ONE READING FOR THE ADMISSION AND THE EVENT. `newClick` used to take its own, so
	// the gate and the event could describe two different windows; see `whose`.
	w := attributionNow()
	idx := observe(newClick(button, true, x, y, w), w.admits())
	// A PRESS THAT WAS NOT RECORDED TAKES NO PICTURE OF WHAT IT LANDED ON. The index is
	// -1, and a capture queued against it would write a screenshot of a window nobody
	// chose onto whatever event is at index -1 — which is to say, it would panic or, worse
	// for this rule, land somewhere. The picture is the clearest case of the thing the
	// whole gate is about: off-target PIXELS are off-target content.
	if idx < 0 {
		return
	}
	// ── A STATED COORDINATE MODE TAKES THE ARM WITH IT, AND NOTHING ELSE CHANGES ──
	//
	// The arm is the one path that captures a picture of a click the global switch would
	// not have anchored — a tap of the anchor key makes the next click an anchor whatever
	// button it is. So a gate that asked only about `captureAnchors` would have left
	// exactly one way for a coordinate recording to come back carrying a picture.
	//
	// It is `plainCoordinates` here and not `anchoring()`, and the difference is a defect
	// this very mutation gate found: `anchoring()` is the conjunction, so using it would
	// have made an armed click do nothing whenever the switch was off, taking the
	// tap-then-click anchor away from every caller instead of from this recording. With
	// no mode stated, both lines below are what they were before, term for term.
	armed := !plainCoordinates.Load() && anchorArmed.Load()
	if armed || (anchoring() && (button == "left" || cvKitchenSink)) {
		select {
		case captureReqs <- captureReq{idx: idx, x: x, y: y, armed: armed}:
		default:
		}
	}
	if armed {
		anchorArmed.Store(false) // consume the one-shot arm
	}
}

// recordClickUp is the release half, and takes its own one reading for the same reason.
func recordClickUp(button string, x, y int) {
	w := attributionNow()
	observe(newClick(button, false, x, y, w), w.admits())
}

// clickShape answers WHAT THAT WINDOW WAS LIKE: its origin, its outer size and its DPI.
//
// Separate from the attribution above because the two are asked at different times — the
// attribution on both click edges, this only on the press — and replaceable for the same reason:
// a real answer needs a real window on a real desktop, and what the builder does with the answer
// is what these tests are about.
var clickShape = winctx.WindowShape

// newClick builds an EvClick event.
//
// # Attribution is on BOTH edges, geometry is on the press only
//
// They are separated deliberately, and the separation is the whole of this comment. The offset
// from the window's top-left is about WHERE the click goes, and only a press has a where worth
// keeping — a release lands wherever the press did. Attribution is about WHOSE the click is, and
// both edges have an owner.
//
// Stamping the press alone is what a first version did, and it reopened the defect it was added
// to close. A button is activated on RELEASE, so the tail of a recording stopped by one ends
// with an edge carrying no owner at all; the strip walking back from the end meets that
// unattributed release, cannot place it, and stops — never reaching the correctly stamped press
// one event behind it. The fixtures stamped both edges, so they described a stream the hook does
// not produce and the strip looked fine.
//
// GetForegroundWindow, GetWindowRect and GetWindowThreadProcessId are all window-data reads:
// cheap, and safe to call on the hook thread. None of them opens a process handle.
func newClick(button string, down bool, x, y int, w whose) RecordedEvent {
	hwnd, pid := w.hwnd, w.pid
	ev := RecordedEvent{Kind: EvClick, Button: button, Down: down, X: x, Y: y,
		T: time.Now(), App: appOfTarget(), PID: pid, HWnd: uintptr(hwnd)}
	// THE WINDOW AS IT WAS AT THIS EDGE — where it is, how big it is on the outside, and
	// what DPI it is drawn at, from the window the attribution above already identified.
	// At the edge itself, because the edge is the only moment it can be read: anything
	// later is a sample, and a sample cannot answer about an instant it missed.
	//
	// The process IMAGE is deliberately not read here. That opens a process handle, which
	// is the work that makes Windows drop the hooks, so it is resolved from `PID` once the
	// capture is over. See `resolveImages`.
	//
	// ── BOTH EDGES, AND THAT IS WHAT A DRAG NEEDS ──
	//
	// The release used to be measured not at all: a release lands wherever the press did,
	// so for a CLICK it is about no rectangle of its own. A DRAG ends somewhere else, and
	// its destination is a point in a window just as much as its start is — so deriving
	// the destination's offset from the PRESS's rectangle would subtract a rectangle
	// measured at one instant from a point captured at another, with nothing able to tell
	// whether the window moved in between. That is an offset into a window as it WAS,
	// attributed to a point from after, and it would pass every check at replay.
	//
	// So the release is measured the same way, `simplify` requires the two edges to agree
	// about the window, and a gesture that spans two different rectangles is refused
	// rather than reconciled. The cost is one more `clickShape` per mouse event in the
	// hook: `GetWindowRect` plus `GetDpiForWindow`, window data Windows already holds, no
	// process handle. The same class of call the attribution above already makes on both
	// edges.
	//
	// ── THE CONTRACT IS DECLARED FOR EVERY EDGE, BEFORE ANY OF IT IS READ ──
	//
	// `RelTo` names the rectangle this edge is ABOUT: the one its offset is measured from
	// and the one it is checked against immediately before input. It is a statement of
	// what the edge MEANS, not one of the measurements, so it goes in whether or not the
	// measurements arrive.
	//
	// That ordering is the whole of this comment, because the other one was wrong.
	// Writing the basis only when every term arrived meant a press whose window would not
	// report its size, or whose scale Windows will not report (no `GetDpiForWindow` before
	// Windows 10 1607), or whose program had exited before the hooks came down, was saved
	// as an ORDINARY point — and an ordinary point is pressed at its recorded coordinate
	// having checked nothing. A failed measurement cannot be what makes the safeguards
	// optional: it means guarded replay is unavailable for that recording, and the
	// recording has to say so rather than quietly become something that runs.
	//
	// So: the basis always, each measurement only when it was taken, missing left
	// missing, and replay refuses until they are all there. Routes already on disk carry
	// no basis and are untouched.
	ev.RelTo = macroir.RelToWindow
	if left, top, w, h, dpi, ok := clickShape(hwnd); ok {
		ev.Dpi = dpi // 0 when Windows would not say, and 0 stays 0
		if w > 0 && h > 0 {
			ev.WinW, ev.WinH = w, h
			// AND WHERE THAT RECTANGLE WAS, which is not a replay target and is
			// never emitted into a route. It is here so `simplify` can ask
			// whether the two edges of a drag are about the SAME rectangle —
			// a question an offset cannot answer, because an offset is what is
			// left after the origin has been subtracted away.
			ev.WinX, ev.WinY = left, top
			// AND THE OFFSET IS A MEASUREMENT OF THAT WINDOW ONLY IF THE EDGE
			// WAS IN IT. A click that CHANGED the foreground looks like this
			// from inside the hook: the attribution names the window that was
			// in front at that instant and the click landed in the one it was
			// about to raise, so the subtraction produces a number that is not
			// an offset into anything.
			if x >= left && y >= top && x-left < w && y-top < h {
				ev.RelX, ev.RelY, ev.WinRel = x-left, y-top, true
			}
		}
	}
	if down {
		ev.Window = winctx.ForegroundTitle() // context: which window was clicked
	}
	return ev
}

// captureFullFrameRGBA grabs the whole primary screen — the anchor image for an
// explicit (armed) anchor, which you then crop down to the target (until cropped it
// won't match the changed live screen, so the click falls back to its recorded
// coordinate). The raw frame is returned (not yet PNG) so the worker can also read
// the pixel under the click for the colour resolver. Best-effort: nil on any failure.
func captureFullFrameRGBA() *image.RGBA {
	w, h := screen.PrimarySize()
	if w <= 0 || h <= 0 {
		return nil
	}
	img, err := screen.CaptureRegion(0, 0, w, h)
	if err != nil || img == nil {
		return nil
	}
	return img
}

// Available says whether this platform has an input-capture backend at all.
//
// A PLATFORM fact and nothing more. It does not say the hooks will install: that can only be
// known by installing them, which takes the keyboard and the mouse of the whole desktop, so a
// readiness check claiming it would be claiming something it had not done. See Start, and the
// note on `anchorKey` for what Windows does to a hook it does not like.
func Available() bool { return true }

// hookShutdownBound is how long a shutdown waits for one of its own goroutines.
//
// Generous against the thing it is bounding and short against a person: a foreground-window read
// that has not returned in this long is not about to, and a run that reports nothing is worse
// than a run that reports what it did and says what it could not tidy up.
const hookShutdownBound = 5 * time.Second

// waitedFor reports whether a channel closed within a bound.
//
// One function, because there are two of these and a second copy of a select is how two bounds
// come to differ. It returns whether it arrived rather than how long it took: every caller's next
// question is "and what do I say".
func waitedFor(ch <-chan struct{}, bound time.Duration) bool {
	select {
	case <-ch:
		return true
	case <-time.After(bound):
		return false
	}
}

// abandon stops recording and hands back what was captured, having given up on the hooks.
//
// The tail of `Stop` for the case where the installing thread cannot be reached at all. Lifted
// out because there are two ways to reach it now — a pump that never answered and a thread that
// never reported starting — and the one thing that must be identical either way is what is left
// behind: recording off, the buffer handed over, and `done` cleared so a second Stop is a no-op.
//
// NOTHING IS FORCED. The hooks can only be removed by a thread that is not answering, and closing
// `captureReqs` while a callback can still send into it would panic inside a hook — a worse end
// to a run than a hook that outlives it.
func (r *winRecorder) abandon() []RecordedEvent {
	recActive.Store(false)
	// AND NO FURTHER RECORDING IN THIS PROCESS. The hooks are still installed and the
	// next Start would turn `recActive` back on for them; see recWedged.
	recWedged.Store(true)
	r.done = nil
	recMu.Lock()
	out := recBuf
	recBuf = nil
	recMu.Unlock()
	// AND THE PROCESS IMAGES, like the ordinary tail. A run whose hooks could not be
	// taken off still produced a demonstration, and a demonstration missing the program
	// behind each press is one `simplify` reads differently — so the give-up path may
	// not be the ordinary path minus a step.
	resolveImages(out, winctx.ImageOf)
	return out
}

// beginRecording makes the global capture state this recorder's.
//
// THE SESSION IS RAISED UNDER `recMu`, WITH THE BUFFER IT DESCRIBES. It used to be raised beside
// the lock — `recSession.Add(1)` and then `recMu.Lock()` on the next line — so the replacement
// was two steps and a worker could be let through between them: it read a session that had
// already moved on, or read the old one and wrote into a buffer that had already been emptied.
// Under one acquisition a worker's publication is entirely before this or entirely after it.
//
// Its own function so a test can arrange the replacement without installing hooks, which is a
// thing this package may not do in a test. See
// `TestARecordingIsNotReplacedWhileAWorkerIsPublishing`.
func (r *winRecorder) beginRecording() {
	recMu.Lock()
	r.session = recSession.Add(1)
	recBuf = nil
	recPub = make(chan RecordedEvent, 1024)
	// THE CROSSING STATE BELONGS TO THIS RECORDING, raised under the lock that empties
	// the buffer it describes — see recSession. A boundary left over from a shutdown that
	// could not complete would start this recording with somebody else's key held down.
	// AND THE CHOSEN WINDOW'S NAME, READ ONCE AND HERE.
	//
	// Here rather than in `Start` for two reasons. It opens a process handle, which is
	// the work that makes Windows drop the hooks, so it cannot be asked on the hook
	// thread and an admitted event has to be tagged with SOMETHING. And this is where
	// everything about whose recording this is gets raised, under the one lock that
	// writes it — a second place for "which recording is this" is a second place for it to
	// drift. Once, because the answer cannot change: a target whose owner changes is a
	// target that is GONE. See appOfTarget for what used to be read instead.
	targetApp.Store(imageOfProcess(r.target.PID))
	bound = nil
	if r.watch == OneWindow {
		bound = NewBoundary()
		// WHAT WAS ALREADY HELD WHEN THIS BEGAN, seeded as held-from-outside so
		// the rule that refuses an admitted action while something is held from
		// outside covers the keys this recording never saw go down. See
		// alreadyHeld.
		bound.HeldBefore(alreadyHeld())
		// AND THE RECORDING OPENS OUTSIDE THE WINDOW, said in the buffer rather
		// than implied. The person is in the Record window and has not gone
		// anywhere yet, so the interval from here to their arrival is time away —
		// and `orchestrator.inWindowClock` takes it out of the clock by the one
		// rule it applies to every other absence, instead of a second rule about
		// the beginning.
		recBuf = append(recBuf, RecordedEvent{Kind: EvLeft, T: time.Now()})
	}
	recMu.Unlock()
}

// mineUnderLock reports whether the global recording state still belongs to this recorder.
//
// `recMu` MUST BE HELD, and that is the point rather than a caller's convenience — which is why
// the name says so, since the compiler cannot. This is read to decide a publication, and a
// decision taken before the lock is one that can already be wrong by the time the write happens:
// the worker passes, is descheduled, a later `Start` empties the buffer, and the write lands in
// somebody else's recording.
//
// A shutdown that could not complete is what leaves a worker to ask this late. The poller is
// inside a foreground-window read that has not returned and the capture is inside screen work;
// `Stop` is bounded, so it gives up and returns while both are still running. See recSession.
func (r *winRecorder) mineUnderLock() bool { return recSession.Load() == r.session }

// anAppSwitchToRecord is whether a sampled foreground app is a switch worth recording at all.
//
// Two questions, not three. The sample has to have found an application, and it has to be a
// CHANGE from the last one recorded — which for the first sample means any application at all.
//
// THE SESSION FENCE USED TO BE THE THIRD QUESTION HERE, and that was the defect: this is asked
// where the sample comes back, and the write happened afterwards, so a recording could be
// replaced between the two. It moved to `recordAppSwitch`, where the write is, and is asked under
// the lock that write takes.
//
// Still its own function because the loop asks it twice, and a condition written twice is one
// that drifts: the fence went into one of the two first. See
// `TestAnAbandonedPollersSampleIsNotRecorded`.
func anAppSwitchToRecord(app, last string) bool { return app != "" && app != last }

// sampled records a foreground sample as an app switch when it is one, and answers what the
// poller's `last` becomes.
//
// THE WHOLE OF THE POLLER'S DECISION, in one function because the loop makes it twice — once for
// the application the demonstration began in and once per tick — and a decision written twice is
// one that drifts: the session fence went into one of the two copies first.
//
// `last` ADVANCES ONLY WHEN THE SWITCH WAS RECORDED. Publication can refuse it even when the
// sample was eligible, because the recording may have been replaced while the foreground read was
// blocked — and a sample remembered as recorded when it was dropped would silence the next real
// switch to that application.
func (r *winRecorder) sampled(app, last string, at time.Time) string {
	// ── THE BOUNDARY MOVES WHEN THE FOREGROUND DOES, NOT WHEN THE NEXT KEY ARRIVES ──
	//
	// This is the second door onto the boundary and it was missing. Markers were made by
	// `observe`, which only runs when the hooks see something, so somebody who left the
	// window, read a page and came back WITHOUT TOUCHING ANYTHING produced no EvLeft and
	// no EvEntered at all — and the absence then had no pair of markers around it, so
	// `inWindowClock` had nothing to take out of the clock and the routine waited the
	// whole interval on every run. Exactly the defect the clock exists to prevent,
	// arriving through the one path that generates no input.
	//
	// So the poller reports the transition it is already sampling for. `Arrived` answers
	// only on a CHANGE, so the two doors cannot double-count: whichever notices first
	// moves the boundary and the other sees no change.
	inside := attributionNow().admits()
	if confined.Load() {
		r.noteBoundary(inside, at)
	}
	// AND A SWITCH TO SOMEWHERE ELSE IS NOT THIS RECORDING'S. The poller writes straight
	// to the buffer, so without this an Activate for the application somebody went to
	// while they were away would be in the demonstration — and `simplify` turns an
	// EvAppSwitch into exactly that. `last` is left alone, so the switch BACK is still
	// recorded, and the decision is made from the SAME reading as the boundary above.
	if confined.Load() && !inside {
		return last
	}
	// ── AND THE APPLICATION RECORDED IS THE CHOSEN WINDOW'S, NOT THE SAMPLE ──
	//
	// The gate above closes the half of the race where the foreground is STILL elsewhere
	// when the snapshot is taken. This is the other half: the sample came back naming
	// Chrome, the person clicked back to the game, and by the time `attributionNow` reads
	// the desktop the foreground IS the game — so the reading admits, and the string
	// written beside it is still `chrome`. `simplify` turns that EvAppSwitch into an
	// Activate, so the routine begins by raising a browser nobody demonstrated in, and
	// `Keep` files it under that application.
	//
	// So membership and the recorded name come out of the same fact: the window that was
	// CHOSEN. Which also means a confined recording holds exactly one application switch,
	// written on the first sample that is admitted — every later one finds `last` already
	// equal to it. And a target whose image could not be read records NONE, because
	// `anAppSwitchToRecord` refuses "": unknown is not an application either.
	if confined.Load() {
		app = chosenApp()
	}
	if anAppSwitchToRecord(app, last) && r.recordAppSwitch(app, at) {
		return app
	}
	return last
}

// noteBoundary moves the boundary from a foreground sample and records whatever marker that
// produces, under the one lock that writes.
//
// THE SAME FENCE AS `recordAppSwitch`, and for the same reason: a poller abandoned by a shutdown
// that could not complete must not move a LATER recording's boundary, or leave an EvLeft in
// somebody else's demonstration that nothing will ever pair with.
func (r *winRecorder) noteBoundary(inside bool, at time.Time) {
	recMu.Lock()
	if bound == nil || !r.mineUnderLock() {
		recMu.Unlock()
		return
	}
	was := bound.Crossing()
	k, changed := bound.Arrived(inside, at)
	var markers []RecordedEvent
	if changed {
		markers = append(markers, RecordedEvent{Kind: k, T: at})
	}
	if now := bound.Crossing(); now != was {
		markers = append(markers, RecordedEvent{Kind: EvCrossed, KeyName: now, T: at})
	}
	// A LEAVING STRANDS WHAT WAS HELD, and this door is the one that usually sees it: the
	// foreground poller notices the switcher taking the foreground, not the key that was
	// still down when it did. The fact has to reach the buffer from here too.
	for _, id := range bound.HeldAcross() {
		markers = append(markers, RecordedEvent{Kind: EvHeldAcross, KeyName: id, T: at})
	}
	for _, m := range markers {
		appendUnderLock(m)
	}
	pub := recPub
	recMu.Unlock()
	for _, m := range markers {
		publish(pub, m)
	}
}

// recordTargetGone publishes the end of a recording whose window has stopped existing, and
// reports whether it did.
//
// The same one-step fence as `recordAppSwitch`: a poller abandoned by a shutdown that could not
// complete must not end a LATER recording on behalf of a window that closed during an earlier
// one.
func (r *winRecorder) recordTargetGone(at time.Time) bool {
	ev := RecordedEvent{Kind: EvTargetGone, T: at}
	recMu.Lock()
	if !r.mineUnderLock() {
		recMu.Unlock()
		return false
	}
	appendUnderLock(ev)
	pub := recPub
	recMu.Unlock()
	publish(pub, ev)
	return true
}

// recordAppSwitch publishes a foreground sample as this recording's app switch, and reports
// whether it did.
//
// THE FENCE AND THE WRITES ARE ONE STEP. Two things reach the global state here — the event
// itself, and the foreground the hooks tag every press with — and both were in the gap: the
// session was asked where the sample came back, so a poller whose read returned late could pass
// that question and then append a step nobody performed to somebody else's demonstration, and
// tag their presses with an application that was not in front of them.
//
// `winctx.Active()` is the call that blocks, which is why the instant that matters is this one
// rather than the one the sample was asked about.
func (r *winRecorder) recordAppSwitch(app string, at time.Time) bool {
	ev := RecordedEvent{Kind: EvAppSwitch, KeyName: app, T: at}
	recMu.Lock()
	if !r.mineUnderLock() {
		recMu.Unlock()
		return false
	}
	recBuf = append(recBuf, ev)
	pub := recPub
	// AND THE FOREGROUND THE HOOKS READ, under the same lock and for the same reason: an
	// abandoned poller that published it would have the next recording's presses tagged
	// with an application nobody was in. See foreApp.
	foreApp.Store(app)
	recMu.Unlock()
	if pub != nil {
		select {
		case pub <- ev:
		default:
		}
	}
	return true
}

// captured is what the capture worker came back with: a picture, and where the click really was.
//
// One value rather than eight parameters, because it travels out of the worker as one thing and
// either lands whole or is dropped whole.
type captured struct {
	idx              int
	img              []byte
	color            string
	clickLX, clickLY int
	// x, y and recenter are where the click moved to when the auto-crop re-centred it on
	// the button it landed in.
	x, y     int
	recenter bool
}

// attach writes a finished capture onto the event it was taken for, and reports whether it did.
//
// THE FENCE AND THE WRITE ARE ONE STEP, under the lock a later `Start` takes to raise the session
// and empty the buffer. Asked before the lock — which is how this was written — the answer could
// be true and the write still land in the recording that replaced it, putting a picture of
// somebody's old press onto whatever event is at that index now.
//
// The bounds check belongs to the same acquisition for the same reason: an index into a buffer is
// only an index into the buffer it was taken from.
func (r *winRecorder) attach(c captured) bool {
	recMu.Lock()
	defer recMu.Unlock()
	if !r.mineUnderLock() || c.idx < 0 || c.idx >= len(recBuf) {
		return false
	}
	ev := &recBuf[c.idx]
	ev.Image = c.img
	ev.ClickX, ev.ClickY = c.clickLX, c.clickLY
	if c.color != "" {
		ev.Color = c.color
	}
	// Re-centre the recorded click on the button centre, shifting the window-relative
	// offset by the same delta so both stay consistent.
	if c.recenter {
		dx, dy := c.x-ev.X, c.y-ev.Y
		ev.X, ev.Y = c.x, c.y
		if ev.WinRel {
			ev.RelX += dx
			ev.RelY += dy
		}
	}
	return true
}

// sideButton names the side button a WM_XBUTTON message is about, from the high word of its
// mouseData: XBUTTON1 is back and XBUTTON2 is forward, the names macroir.MouseButtons uses. A
// value Windows does not document is "", and the press is not recorded rather than recorded as
// some other button.
//
// Not reading these must fail TestTheSideButtonsAreRecordedByName.
func sideButton(mouseData uint32) string {
	switch mouseData >> 16 {
	case 1:
		return "back"
	case 2:
		return "forward"
	}
	return ""
}

// recordScroll records a turn of the wheel where the pointer is, measured the way a press is
// (newClick: the window under it, its offset into that window and the window's terms), since
// the window under the pointer is the one that scrolls. No picture is taken: nothing anchors a
// scroll. A zero delta is not a turn and is not recorded.
//
// Not reading the wheel must fail TestTheWheelIsRecordedWhereItTurned.
func recordScroll(x, y, delta int, horizontal bool) {
	if delta == 0 {
		return
	}
	w := attributionNow()
	ev := newClick("", true, x, y, w)
	ev.Kind, ev.Button, ev.Down = EvScroll, "", false
	ev.Delta, ev.Horizontal = delta, horizontal
	observe(ev, w.admits())
}
