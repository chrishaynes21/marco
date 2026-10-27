// Package oshost implements a native, in-process Host (see spec/Hosts.md) that
// performs real OS effects — keystrokes, mouse, sleep, pixel reads — for foreign
// acts. The platform primitives live behind a small `backend` interface with a
// Windows implementation (SendInput) and a stub for other platforms, so the
// package builds everywhere; only Windows does real work.
package oshost

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/screen"
	"github.com/chaynes-simpleclouds/marco/internal/secrets"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// backend is the platform-specific primitive surface. Implemented by the
// Windows SendInput backend and a no-op stub elsewhere.
type backend interface {
	key(ctx context.Context, name string, hold time.Duration) error // hold<=0 → default linger
	keyDown(ctx context.Context, name string) error                 // press and hold
	keyUp(ctx context.Context, name string) error                   // release
	typeText(ctx context.Context, text string) error
	click(ctx context.Context, button string) error
	clickAt(ctx context.Context, button string, x, y int) error
	drag(ctx context.Context, button string, x1, y1, x2, y2 int) error
	move(ctx context.Context, x, y int) error
	color(ctx context.Context, x, y int) (uint32, error)
	activeExe(ctx context.Context) (string, error)
}

// Host adapts a backend to runtime.Host, dispatching foreign action names to
// platform primitives and marshalling the call's Input/Output Values.
//
// It also owns the background spam loop: like the original MacroMarco engine,
// at most one spam runs at a time (Spam replaces it, StopSpam ends it). The spam
// goroutine is host-owned — not tied to a Marco frame — so it survives across
// hotkey events until StopSpam.
type Host struct {
	b          backend
	scr        screen.Screen
	sec        secrets.Store
	mu         sync.Mutex
	spamCancel context.CancelFunc
	args       map[string]string // current invocation's named args (for inline secrets)
	// originL/originT cache the foreground window origin for window-relative clicks,
	// snapshotted once and reused so a momentary focus flicker mid-route can't send a
	// later click to the wrong origin. Activate clears it so the next click re-reads
	// against the newly-focused window. originName is which window that origin came
	// from (for diagnostics). Guarded by mu.
	originL, originT int
	originName       string
	originSet        bool
	// asked is how many input actions this run asked the machine for. Guarded by mu.
	// See Host.InputActions for what the number is and is not.
	asked int
	// held is every key the machine would not release, which is a state this run LEFT
	// rather than a thing it did not do — and the one account that has to survive a
	// cancelled run discarding its error. Guarded by mu. See noteHeld and NotReleased.
	//
	// More than one, because there is more than one way to be left holding something: a
	// chord whose modifier would not come up, and a hold the route-end sweep could not
	// release. A single field let the second quietly replace the first.
	held []string
	// heldKeys are keys pressed by a KeyDown without a matching KeyUp yet, IN THE ORDER
	// THEY WENT DOWN. Tracked so a route end (success, error, or Esc) can release any still
	// down — a per-command process exiting with a key held would leave it STUCK down in the
	// target app — and ordered because they come up in reverse. Guarded by mu.
	heldKeys []string
	// activated is set once this run brings a specific window to the front (Activate).
	// Window-relative clicks resolve their origin from "the foreground window", which is
	// only trustworthy AFTER an Activate (then foreground really is the target app).
	// Without one, foreground may be the overlay (it holds focus when you type a
	// command), so a click uses its ABSOLUTE recorded coordinate instead — exact for a
	// fullscreen/in-place app, and immune to the focus race. Guarded by mu.
	activated bool
	// bound is THE WINDOW Activate brought to the front, and boundApp what was asked
	// for. Zero until something has been activated. Guarded by mu.
	//
	// # Why a handle and not "the foreground window"
	//
	// Because "activate the app, then press inside it" is two instants, and the desktop
	// belongs to somebody else in between. Re-asking which window is in front at the
	// press answers a different question from the one the route prepared — a dialog, an
	// overlay or the person can have changed the answer — and an offset added to that
	// window has been added to a window nobody checked.
	//
	// With the handle kept, a press can ask about THAT window: still a window, still in
	// front, still the size and scale and program it was. See `bindAt` and `resolve`.
	//
	// # And the process, because a handle is not an identity
	//
	// boundPID is who owned that handle at the instant it was bound. Windows reuses
	// window handles, and the program NAME does not separate two copies of the same
	// program: a second instance of the game, a second Explorer window, a second
	// browser profile each answer the image-name check correctly while being a window
	// the route never prepared. The process id is what tells them apart, and it has to
	// be read at preparation time, because afterwards there is nothing left to compare.
	bound    uintptr
	boundApp string
	boundPID uint32
	// undone is the sentence of everything this run did not do, in order. See
	// `Unperformed`. Guarded by mu.
	undone []string
}

// SetArgs records the named arguments of the invocation about to run, so a
// secret-typed arg (password/pin/…) can be provided inline — used and remembered —
// instead of pre-set with `marco secret set`.
func (h *Host) SetArgs(a map[string]string) {
	if len(a) > 0 {
		keys := make([]string, 0, len(a))
		for k := range a {
			keys = append(keys, k)
		}
		mlog.Debug("oshost: args set", "keys", keys)
	}
	h.mu.Lock()
	h.args = a
	h.mu.Unlock()
}

// New returns the native OS host for the current platform.
func New() runtime.Host {
	return &Host{b: newBackend(), scr: screen.New(), sec: secrets.New()}
}

// CursorSnapshot records the current cursor position and returns a function that
// puts it back — so a route's clicks/moves don't leave the pointer parked on the
// last target (a menu macro should return you to where you were). The driver calls
// it around a real run; the dryrun host doesn't implement it, so dry runs never
// touch the cursor. No-op off Windows (winctx.SetCursorPos is a stub there).
func (h *Host) CursorSnapshot() func() {
	x, y := winctx.CursorPos()
	return func() { winctx.SetCursorPos(x, y) }
}

// sendsInput says whether a host action puts input into an application.
//
// A CLOSED CLASSIFICATION WITH ITS FALSE ANSWERS WRITTEN OUT, because the half that matters is
// neither answer — it is "nobody has said". An action absent from here is not classified, and
// `TestEveryHostActionSaysWhetherItSendsInput` reads the switch below and fails until somebody
// decides, so a new action cannot quietly join either side. Defaulting to false would silently
// under-count what touched a person's machine, which is the one thing the count exists to answer;
// defaulting to true would over-count a reader.
//
// `secret` is here as a send on purpose: it types the password. `repeat` and `spam` each count
// once, as the one action that was asked for, though both put out keystrokes until something
// stops them.
var sendsInput = map[string]bool{
	"key": true, "keydown": true, "keyup": true, "navigate": true, "type": true,
	"click": true, "move": true, "drag": true, "repeat": true, "spam": true,
	"secret": true,

	"sleep": false, "clipboardget": false, "clipboardset": false, "color": false,
	"focus": false, "stopspam": false, "stop": false, "roll": false, "eightball": false,
	"name": false, "find": false, "activate": false, "launch": false, "restore": false,
	"movewindow": false, "windowstate": false,
}

// InputActions is how many input actions this run asked the machine for.
//
// # What the number is, and the two things it is not
//
// ASKED FOR, not delivered. `SendInput` returning success says the queue took the press, not that
// the application acted on it, and nothing on this path looks at the screen afterwards — so
// "confirmed delivery" is not a number this host can produce and is not written anywhere. What
// did NOT happen is [Host.Unperformed], and the two together are the honest pair: this says how
// many times Marco asked, that says which of them it could not.
//
// ACTIONS, not primitives and not steps. One smoothed Move puts out dozens of small moves and is
// one action; `Spam` puts out keystrokes until somebody stops it and is one action. A routine's
// step count is a different question and lives with the routine.
//
// Counted in [Host.Invoke] against [sendsInput], which is one place and a closed list. The
// earlier plan for this was to count inside `delivered` — the assessment this work came from says
// `oshost` "already wraps every send in `delivered`", and it does not: four call sites against
// eleven sending actions, so a routine that typed or dragged would have been counted as having
// asked for nothing.
func (h *Host) InputActions() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.asked
}

func (h *Host) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	// WHAT THIS RUN ASKED FOR, counted before the action is dispatched.
	//
	// Before, because an action refused by its own guard still asked: a `Move` that could
	// not check the window it was recorded in is a press somebody meant to happen, and a
	// record saying the run asked for nothing would be the wrong half of the story. The
	// other half is `Unperformed`, which names it.
	if sendsInput[strings.ToLower(c.Action)] {
		h.mu.Lock()
		h.asked++
		h.mu.Unlock()
	}
	switch strings.ToLower(c.Action) {
	case "key":
		return h.doKey(c)
	case "keydown":
		return h.doKeyDown(c)
	case "keyup":
		return h.doKeyUp(c)
	case "navigate":
		return h.doNavigate(c)
	case "type":
		return ok(h.b.typeText(c.Ctx, c.Input.AsText()))
	case "click":
		return h.doClick(c)
	case "move":
		// RESOLVED THE SAME WAY A CLICK IS, guard included. A right or middle
		// click is a Move followed by a Click, so a guard that covered only the
		// click would leave every press that is not left-handed unchecked.
		x, y, present, why := h.clickTarget(c.Input)
		if why != "" {
			return fail(why)
		}
		if !present {
			return fail("move needs a Point with X and Y")
		}
		mlog.Debug("move: to point", "x", x, "y", y)
		return h.delivered(fmt.Sprintf("aiming at %d,%d", x, y),
			h.b.move(c.Ctx, x, y))
	case "drag":
		return h.doDrag(c)
	case "sleep":
		return h.doSleep(c)
	case "clipboardget":
		return h.doClipboardGet(c)
	case "clipboardset":
		return h.doClipboardSet(c)
	case "color":
		return h.doColor(c)
	case "focus":
		return h.doFocus(c)
	case "repeat":
		return h.doRepeat(c)
	case "spam":
		return h.doSpam(c)
	case "stopspam", "stop":
		return h.doStopSpam()
	case "roll":
		return h.doRoll(c)
	case "eightball":
		return h.doEightBall()
	case "name":
		return h.doName()
	case "find":
		return h.doFind(c)
	case "secret":
		return h.doSecret(c)
	case "activate":
		app := c.Input.AsText()
		// AND THIS IS THE PREPARATION HALF OF THE GUARDED CONTRACT: the window
		// Activate settled on is BOUND, so a later press can ask about that same
		// window rather than about whatever is in front by then. See `bindAt`.
		hwnd, err := bindWindow(app)
		h.bindAt(hwnd, app)
		if err != nil {
			// PREPARATION THAT DID NOT HAPPEN IS KEPT BESIDE THE REFUSALS.
			//
			// Activate is what binds the window every guarded press is then
			// checked against, so a routine whose application never came
			// forward has nothing to press in. Recorded for every route and
			// acted on only by one: the caller decides, from the contract the
			// route was saved under, whether this makes the run unsuccessful.
			h.didNotDo(fmt.Sprintf("the %s window could not be brought to the "+
				"front: %v", app, err))
		}
		return ok(err)
	case "launch":
		return ok(winctx.Launch(c.Input.AsText()))
	case "restore":
		return ok(winctx.RestorePrevious())
	case "movewindow":
		return h.doMoveWindow(c)
	case "windowstate":
		return h.doWindowState(c)
	default:
		return fail(fmt.Sprintf("OS host has no action %q", c.Action))
	}
}

// doMoveWindow repositions a window: `{ Window: "hwnd:1234", X, Y, W, H }`.
//
// The window is named by HANDLE rather than by app name, unlike Activate. A caller
// that has already decided WHICH window it means — the Director, from its World
// Model — must be able to say so exactly; resolving by name again here could pick a
// different window of a multi-window application than the one that was chosen.
func (h *Host) doMoveWindow(c runtime.HostCall) (string, runtime.Value, error) {
	set := c.Input.AsSet()
	if set == nil {
		return fail("movewindow needs a set with Window, X, Y, W and H")
	}
	hwnd, err := parseHandle(textOf(set, "Window"))
	if err != nil {
		return fail(err.Error())
	}
	x, y := setInt(set, "X"), setInt(set, "Y")
	w, ht := setInt(set, "W"), setInt(set, "H")
	if w <= 0 || ht <= 0 {
		return fail("movewindow needs a positive W and H")
	}
	mlog.Debug("movewindow", "window", hwnd, "to", fmt.Sprintf("(%d,%d %dx%d)", x, y, w, ht))
	return ok(winctx.MoveWindow(hwnd, x, y, w, ht))
}

// doWindowState applies a symbolic window state: `{ Window: "hwnd:1234", State: "maximized" }`.
func (h *Host) doWindowState(c runtime.HostCall) (string, runtime.Value, error) {
	set := c.Input.AsSet()
	if set == nil {
		return fail("windowstate needs a set with Window and State")
	}
	hwnd, err := parseHandle(textOf(set, "Window"))
	if err != nil {
		return fail(err.Error())
	}
	return ok(winctx.SetWindowState(hwnd, strings.ToLower(textOf(set, "State"))))
}

// parseHandle reads the "hwnd:<n>" form the Director uses for a WindowID. An
// unparseable handle is an error rather than a fall-back to the foreground window:
// silently acting on a different window than the one requested is the failure mode
// this format exists to prevent.
func parseHandle(s string) (uintptr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("no Window given")
	}
	if !strings.HasPrefix(strings.ToLower(s), "hwnd:") {
		return 0, fmt.Errorf("Window must look like \"hwnd:<handle>\", got %q", s)
	}
	n, err := strconv.ParseUint(s[5:], 10, 64)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("Window %q is not a usable handle", s)
	}
	return uintptr(n), nil
}

// doSecret types the value of a named secret from the OS credential store. The
// value is never logged or returned — only typed. Resolves failed if the secret
// isn't set, so the route surfaces a clear error instead of typing nothing.
func (h *Host) doSecret(c runtime.HostCall) (string, runtime.Value, error) {
	name := c.Input.AsText()
	if name == "" {
		return fail("secret needs a name")
	}
	// The arg name is the part after the last ":" (route-qualified keys look like
	// "login-to-facebook:password"). If the invocation provided that arg inline, use
	// it and remember it — so a secret named arg is entered once, then reused.
	argname := name
	if i := strings.LastIndex(name, ":"); i >= 0 {
		argname = name[i+1:]
	}
	mlog.Debug("secret: lookup", "key", name, "argname", argname)
	h.mu.Lock()
	provided, has := h.args[argname]
	h.mu.Unlock()
	if has && provided != "" {
		mlog.Debug("secret: using inline arg, remembering", "argname", argname)
		_ = h.sec.Set(name, provided) // remember for next time
		return ok(h.b.typeText(c.Ctx, provided))
	}
	val, found, err := h.sec.Get(name)
	if err != nil {
		mlog.Error("secret: store error", "key", name, "err", err)
		return fail(fmt.Sprintf("secret %q: %v", name, err))
	}
	if !found {
		mlog.Warn("secret: not set", "key", name)
		return fail(fmt.Sprintf("secret %q is not set — pass %s:<value> once, or run: marco secret set %s", name, argname, name))
	}
	mlog.Debug("secret: store hit", "key", name)
	return ok(h.b.typeText(c.Ctx, val))
}

// doKey presses a key or chord. Input is either the key text ("alt+tab") or a set
// { Key "alt+tab", Hold 250 } whose Hold (ms) is how long the key/combo is held before
// release — the knob that makes Alt+Tab commit against a fullscreen app without the
// user having to spell out KeyDown/Sleep/KeyUp. Hold<=0 uses the default linger.
func (h *Host) doKey(c runtime.HostCall) (string, runtime.Value, error) {
	var name string
	var hold time.Duration
	if set := c.Input.AsSet(); set != nil {
		if v, has := set.Get("Key"); has {
			name = v.AsText()
		}
		if v, has := set.Get("Hold"); has {
			if n, okN := v.AsNumber(); okN && n > 0 {
				hold = time.Duration(n) * time.Millisecond
			}
		}
	} else {
		name = c.Input.AsText()
	}
	if name == "" {
		return fail("key needs a key name (text, or a set with Key)")
	}
	// AND A KEY THE MACHINE WOULD NOT TAKE IS KEPT, like a press the machine would not
	// take. It is the same outcome from where a person is standing — the keystroke did
	// not happen — and the screen the steps after it act on is not the screen they were
	// recorded against. Without this the status stopped the routine and the account held
	// nothing, so the command reported success about a routine that did not run.
	err := h.b.key(c.Ctx, name, hold)
	// AND A KEY THE MACHINE WOULD NOT LET GO OF IS KEPT SEPARATELY, because it is not an
	// account of something that did not happen. See noteHeld.
	h.noteHeld("the "+name+" key", err)
	return h.delivered("the "+name+" key", err)
}

// doKeyDown presses and HOLDS a key, tracking it so a route end always releases it. The
// hold persists across the steps that follow (a click, a move) until a matching KeyUp —
// that's the "hold Q, click, release Q" pattern.
func (h *Host) doKeyDown(c runtime.HostCall) (string, runtime.Value, error) {
	name := c.Input.AsText()
	if name == "" {
		return fail("keydown needs a key")
	}
	// NOTHING IS PRESSED BY A RUN THAT HAS ALREADY STOPPED, and a HOLD is the worst one
	// to get wrong: the thing that releases it is the end of a run that is already over.
	// Here rather than in the backend so it holds for every backend and can be checked
	// without a desktop — the same place and the same reason as `sendChord`.
	if err := c.Ctx.Err(); err != nil {
		return fail(err.Error())
	}
	// AND A HOLD THE MACHINE WOULD NOT TAKE IS KEPT, like a press and like a keystroke.
	//
	// It was reported as a failed step and recorded nowhere, so the command read an empty
	// account and reported success about a routine whose hold never went down — and
	// everything after that hold ran as though a key were held when it was not.
	if err := h.b.keyDown(c.Ctx, name); err != nil {
		return h.delivered("the "+name+" hold", err)
	}
	// AND A KEY THAT WENT DOWN IS WRITTEN DOWN, IN THE ORDER IT WENT DOWN.
	h.holding(name)
	mlog.Debug("key: hold", "key", name)
	return ok(nil)
}

// doKeyUp releases a held key.
//
// THE RELEASE IS ATTEMPTED FIRST AND THE KEY IS FORGOTTEN ONLY IF IT WORKED. Forgetting it
// first was the defect: the route-end sweep is the one thing that could try again, and it was
// being told the key had come up. A key the machine refuses stays on the list, so the sweep
// tries once more and reports it if that fails too.
func (h *Host) doKeyUp(c runtime.HostCall) (string, runtime.Value, error) {
	name := c.Input.AsText()
	if name == "" {
		return fail("keyup needs a key")
	}
	if err := h.b.keyUp(c.Ctx, name); err != nil {
		mlog.Error("key: the machine would not release a held key", "key", name,
			"err", err)
		// KEPT IN THE ACCOUNT TOO. Everything after a release that did not happen runs
		// with the key still down — modified clicks, different typing — and the
		// route-end sweep recovering the key later is not the same as the routine
		// having been right to carry on.
		return h.delivered("the "+name+" release", err)
	}
	h.released(name)
	mlog.Debug("key: release", "key", name)
	return ok(nil)
}

// holding and released are the list of keys this run is holding down, in the order they went
// down.
//
// A LIST AND NOT A SET, because the order is the answer. Releasing holds in the order a map
// happened to iterate was releasing them in no order at all: an application reading modifier
// state while a chord of holds comes apart sees a different thing depending on which goes
// first, and "whichever Go felt like" is not a thing a test can pin or a person can reason
// about. The reverse of the pressing is what a hand does and what applications expect.
func (h *Host) holding(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, k := range h.heldKeys {
		if k == name {
			return // already down; holding it twice is still one key
		}
	}
	h.heldKeys = append(h.heldKeys, name)
}

func (h *Host) released(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, k := range h.heldKeys {
		if k == name {
			h.heldKeys = append(h.heldKeys[:i], h.heldKeys[i+1:]...)
			return
		}
	}
}

// ReleaseHeld releases any key still down from a KeyDown without a matching KeyUp. The
// driver calls it at route end (success, error, or Esc), so a hold can never leave a key
// stuck down after the per-command process exits. No-op when nothing is held.
//
// # What it guarantees, in the words that survive review
//
// Every key this run is holding gets a release ATTEMPT, in the reverse of the order it was
// pressed, and a key the machine refuses is REPORTED — through the same account a chord's
// refused release goes into, so `marco` puts it in the record and Studio shows it. It is not a
// recovery: nothing here can put a key up that the machine will not take, and a process killed
// before this runs does not run it at all.
//
// It used to discard every answer (`_ = h.b.keyUp(...)`), which made the one path whose entire
// purpose is "nothing is left down" the one path that could not say otherwise.
//
// The context is `Background` on purpose and not the run's: a stopped run is exactly when a key
// most needs to come up.
func (h *Host) ReleaseHeld() {
	h.mu.Lock()
	held := append([]string(nil), h.heldKeys...)
	h.heldKeys = nil
	h.mu.Unlock()

	var stuck []string
	for i := len(held) - 1; i >= 0; i-- {
		mlog.Debug("key: releasing still-held key at route end", "key", held[i])
		if err := h.b.keyUp(context.Background(), held[i]); err != nil {
			// EVERY ONE IS STILL ATTEMPTED. Stopping at the first failure would
			// leave the rest down in order to report it.
			stuck = append(stuck, held[i])
		}
	}
	if len(stuck) > 0 {
		mlog.Error("key: a held key would not come up at route end", "keys", stuck)
		h.noteHeld("the run", KeyNotReleased(stuck...))
	}
}

// doFind resolves an Anchor — an on-screen target — into the point to click, by SCORING
// candidate locations and fusing the in-engine signals into a CONFIDENCE, rather than a
// first-hit chain. Input is the Anchor set:
//
//	{ Image: template path, X1..Y2?: search region, Color: "0xRRGGBB",
//	  X,Y: recorded click (+ RelX,RelY: window-relative), Tolerance?, Timeout? }
//
// Signals (each in-engine, OS-agnostic): IMAGE (how well the captured crop matches at a
// location), COLOUR (how close the pixel there is to the recorded colour), POSITION (how
// near the location is to the recorded click — the prior). Candidates are the recorded
// point P (resolved window-relative) and the image's best match Lb; each is scored across
// the present signals (weighted, normalised) and the best wins. Clears the confidence
// threshold → click THERE (so a moved target is followed). Below it → resolve failed and
// log a repair hint; the route falls back to its recorded coordinate. Text (OCR) is a
// separate host, tried by the route's `or?` branch when this is unsure.
func (h *Host) doFind(c runtime.HostCall) (string, runtime.Value, error) {
	set := c.Input.AsSet()
	if set == nil {
		return fail("find needs an Anchor set")
	}
	imgPath := textOf(set, "Image")
	colStr := textOf(set, "Color")
	if imgPath == "" && colStr == "" {
		// A text-only anchor belongs to the Text (OCR) host; with none wired this falls
		// through to us as the "*" default — decline quietly so the route uses its
		// recorded coordinate instead of erroring on a request that was never ours.
		if textOf(set, "Text") != "" {
			mlog.Debug("find: text-only anchor, no OCR host wired — declining")
			return "failed", runtime.Absent(), nil
		}
		return fail("find needs an Image or a Color")
	}
	region := screen.Region{X1: setInt(set, "X1"), Y1: setInt(set, "Y1"), X2: setInt(set, "X2"), Y2: setInt(set, "Y2")}
	tol := 24
	if v, ok := set.Get("Tolerance"); ok {
		if n, ok := v.AsNumber(); ok {
			tol = int(n)
		}
	}
	// $MARCO_FIND_TOLERANCE overrides the per-channel match tolerance for every anchor —
	// a quick way to test whether a stubborn match is just a colour/brightness shift.
	if v := strings.TrimSpace(os.Getenv("MARCO_FIND_TOLERANCE")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			tol = n
		}
	}
	wantCol, hasCol := parseHexColor(colStr)
	recWindow := textOf(set, "Window") // context: the window title this anchor was recorded in
	px, py := h.anchorPoint(set)       // recorded click, window-relative when this run activated
	_, hasX := set.Get("X")
	_, hasY := set.Get("Y")
	// Alpha "no CV" mode ($MARCO_CV=off / $MARCO_ANCHORS=off): skip the anchor entirely and
	// resolve to the recorded coordinate NOW — no hover, no matching, no polling. The route's
	// recorded Sleep timings drive pacing, so nothing "clicks randomly" waiting for a match.
	if cvOff() && (hasX || hasY) {
		// THROUGH THE SAME RESOLVER A CLICK USES, guard included.
		//
		// This is the point that becomes input one step later: the route does
		// `do OS's Find` and then clicks what it got back. `anchorPoint` above is
		// the unguarded form and it stays that way, because its other job is to
		// centre a SEARCH BOX — refusing there would refuse to go looking. Here
		// there is nothing to look for and this answer is the press.
		ax, ay := setInt(set, "X"), setInt(set, "Y")
		rx, ry, _, why := h.resolve(set, ax, ay)
		if why != "" {
			mlog.Info("find: refused", "why", why)
			return fail(why)
		}
		mlog.Debug("find: CV off — clicking the recorded coordinate",
			"at", fmt.Sprintf("(%d,%d)", rx, ry))
		return okPoint(rx, ry)
	}
	// Restrict the image search to a box around the recorded point when no explicit region
	// is given AND there IS a recorded point. The target is AT/near the click, so scanning
	// the whole (multi-monitor) desktop — and re-capturing it every poll — is what makes a
	// Find "take forever"; it also invites far-away false matches. A tight box is far faster
	// and safer. (A "wait for this screen" anchor has no X,Y, so it keeps the full-screen
	// search.) Misses only a target that moved very far — the text/OCR locator's job.
	// Last-known: where this anchor resolved on a previous run. The search box covers BOTH
	// the recorded point and last-known (so a drifted target is searched where it actually
	// is, while the recorded point stays a safety net), and last-known also boosts the
	// position prior there — together letting the search follow a UI that drifts across runs.
	lastKnown, hasLast := anchorLastKnown(imgPath)
	// Ignore a last-known that's implausibly far from the recorded point — a stale or
	// poisoned cache entry. Trusting it would balloon the search box and risk chasing a
	// wrong target; real drift between runs is small.
	if hasLast && (iabs(lastKnown[0]-px) > maxDriftPx || iabs(lastKnown[1]-py) > maxDriftPx) {
		mlog.Debug("find: ignoring an implausibly far last-known location", "lastKnown", fmt.Sprintf("(%d,%d)", lastKnown[0], lastKnown[1]))
		hasLast = false
	}
	if region.Empty() && imgPath != "" && (hasX || hasY) {
		x1, y1, x2, y2 := px-searchMargin, py-searchMargin, px+searchMargin, py+searchMargin
		if hasLast {
			x1, y1 = min(x1, lastKnown[0]-searchMargin), min(y1, lastKnown[1]-searchMargin)
			x2, y2 = max(x2, lastKnown[0]+searchMargin), max(y2, lastKnown[1]+searchMargin)
		}
		region = screen.Region{X1: x1, Y1: y1, X2: x2, Y2: y2}
		mlog.Debug("find: search box", "x1", region.X1, "y1", region.Y1, "x2", region.X2, "y2", region.Y2)
	}
	timeout := time.Duration(setInt(set, "Timeout")) * time.Millisecond
	// CV find dial: when loosened, be more PATIENT on the short click-gate (≤5s) so a
	// slower-appearing target gets more time before the route clicks anyway. Long
	// wait-for-screen barriers (already generous) are left alone.
	if timeout > 0 && timeout <= 5*time.Second {
		timeout = time.Duration(float64(timeout) * findTimeoutScale())
	}
	mlog.Debug("find: anchor", "image", filepath.Base(imgPath), "color", colStr,
		"recorded", fmt.Sprintf("(%d,%d)", px, py), "tol", tol)

	// Hover settle: many UIs draw a control differently while the pointer is OVER it
	// (highlight, glow, a fill that fades in). The template was captured at learn time with
	// the cursor on the target — i.e. hovered — so the live, un-hovered button never matches
	// it and the anchor falls back to its coordinate. Put the cursor on the recorded point
	// first so the live UI enters the SAME hover state the template captured, then match.
	// Only for a positioned anchor (a "wait for this screen" gate has no point to hover);
	// the recorded point is where we'd click anyway, and the driver restores the cursor at
	// route end. Toggle with $MARCO_FIND_HOVER=0.
	hovering := (hasX || hasY) && findHoverEnabled()
	if hovering {
		if err := h.glide(c.Ctx, px, py); err == nil {
			mlog.Debug("find: gliding the cursor onto the recorded point to settle the UI before matching", "at", fmt.Sprintf("(%d,%d)", px, py))
			select {
			case <-time.After(hoverSettle()):
			case <-c.Ctx.Done():
				return "failed", runtime.Absent(), nil
			}
		}
	}

	deadline := time.Now().Add(timeout)
	wiggle := 0
	for {
		// Keep the highlight ALIVE while we look: some UIs only render (or refresh) a hover
		// state on pointer MOVEMENT, so a stationary cursor lets it fade between polls. Nudge
		// a pixel or two around the recorded point each round so the live control keeps showing
		// the same hovered look the template captured. ±2px is well within the matcher's slack.
		if hovering && findWiggleEnabled() {
			dx, dy := wiggleStep(wiggle)
			_ = h.b.move(c.Ctx, px+dx, py+dy)
			wiggle++
		}
		best, conf := h.scoreAnchor(imgPath, region, tol, wantCol, hasCol, px, py, lastKnown, hasLast)
		conf *= h.windowFactor(recWindow) // a match in the wrong window can't be confident
		if conf >= findConfidence() {
			mlog.Debug("find: resolved", "at", fmt.Sprintf("(%d,%d)", best[0], best[1]), "confidence", round2(conf))
			rememberLocation(imgPath, best[0], best[1]) // follow drift on the next run
			return okPoint(best[0], best[1])
		}
		if timeout <= 0 || time.Now().After(deadline) {
			mlog.Warn("find: low confidence — falling back to the recorded coordinate; record it again to repair this anchor",
				"best", fmt.Sprintf("(%d,%d)", best[0], best[1]), "confidence", round2(conf), "threshold", findConfidence())
			return "failed", runtime.Absent(), nil
		}
		select {
		case <-c.Ctx.Done(): // Esc / abort stops the wait
			return "failed", runtime.Absent(), nil
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// locator is one LOCALISING signal's result: where it best matched, how well, and its
// evidence weight. Image matches raw pixels; edge matches the button's outline (so it
// survives a recolour/theme change).
type locator struct {
	name      string
	weight    float64
	found     bool
	ambiguous bool // matched in several places: confirms the recorded point, never moves the click
	x, y      int
	score     float64
	hist      float64 // colour-histogram (palette) similarity at the match, if available
	hasHist   bool
}

// scoreAnchor evaluates the anchor's candidate locations once and returns the best
// location and its CONFIDENCE (0..1). It fuses LOCATING signals (image, edge) with the
// CONFIRMING colour signal, using position only as a selection prior.
//
// Candidates: the recorded point P (the default target), plus any locator's match that's
// FAR from P AND colour-corroborated — a real move, not a false/ambiguous match that must
// never drag the click. Each candidate's CONFIDENCE is the weighted, normalised evidence
// of the signals that actually point at it (a locator credits a candidate only when its
// best match is there; colour is probed at the candidate). POSITION is NOT evidence (P is
// always distance 0 from itself) — it gently favours the nearer candidate so a far match
// must clearly out-evidence the recorded point to win.
func (h *Host) scoreAnchor(imgPath string, region screen.Region, tol int, wantCol uint32, hasCol bool, px, py int, lastKnown [2]int, hasLast bool) (loc [2]int, conf float64) {
	// Hard safety net: the demonstrated point is the default, and a panic anywhere in the
	// matchers (a malformed template, a bad capture) degrades to it instead of crashing the
	// route mid-run.
	loc = [2]int{px, py}
	defer func() {
		if r := recover(); r != nil {
			mlog.Error("find: scorer panicked — falling back to the recorded point", "err", fmt.Sprintf("%v", r))
			loc, conf = [2]int{px, py}, 0
		}
	}()
	locate := func(name string, weight float64, find func() (screen.Match, error)) locator {
		l := locator{name: name, weight: weight}
		if imgPath == "" {
			return l
		}
		m, err := find()
		switch {
		case err != nil || !m.Found:
		case m.Ambiguous:
			// Matches in MULTIPLE places (the same button across menus): we can't trust it to
			// LOCATE a moved target, but if its best match sits at the recorded point it still
			// CONFIRMS the template is (still) there. So keep it as evidence and flag it — the
			// move-candidate pass excludes ambiguous locators, the scoring pass still lets it
			// corroborate the demonstrated point. Without this a row of identical buttons would
			// zero out the recorded point's evidence and collapse confidence on a static menu.
			mlog.Debug("find: " + name + " ambiguous (matches multiple places) — confirming only, not moving by it")
			l.found, l.ambiguous, l.x, l.y, l.score = true, true, m.X, m.Y, m.Score
			l.hist, l.hasHist = m.ColorScore, m.ColorScore > 0
		default:
			l.found, l.x, l.y, l.score = true, m.X, m.Y, m.Score
			l.hist, l.hasHist = m.ColorScore, m.ColorScore > 0 // palette confirm (image only)
		}
		return l
	}
	locs := []locator{locate("image", wImage, func() (screen.Match, error) { return h.scr.Find(imgPath, region, tol) })}
	// Edge matching runs a distance transform over the region — heavier than a pixel
	// match — so only do it on a region-restricted search (a click anchor near its point),
	// not a full-screen wait-for-this-screen anchor.
	if !region.Empty() {
		locs = append(locs, locate("edge", wEdge, func() (screen.Match, error) { return h.scr.FindEdge(imgPath, region, tol) }))
	}

	type cand struct{ x, y int }
	cands := []cand{{px, py}}
	near := func(ax, ay, bx, by int) bool { return iabs(ax-bx) <= matchEpsilon && iabs(ay-by) <= matchEpsilon }
	for _, l := range locs {
		if !l.found || l.ambiguous || near(l.x, l.y, px, py) || l.score < locateFloor() {
			continue
		}
		if !h.colorCorroborates(l.x, l.y, wantCol, hasCol, tol) {
			continue
		}
		dup := false
		for _, c := range cands {
			if near(c.x, c.y, l.x, l.y) {
				dup = true
			}
		}
		if !dup {
			cands = append(cands, cand{l.x, l.y})
			mlog.Debug("find: "+l.name+" matched away from the recorded point and colour agrees — following the move",
				"to", fmt.Sprintf("(%d,%d)", l.x, l.y), "score", round2(l.score))
		}
	}

	best, bestSel, bestConf := [2]int{px, py}, -1.0, 0.0
	pSel := 0.0 // the recorded point's selection score (cands[0]); a move must clearly beat it
	for ci, c := range cands {
		var sum, wsum float64
		var parts []string
		for _, l := range locs {
			if l.found && near(l.x, l.y, c.x, c.y) {
				sum += l.weight * l.score
				wsum += l.weight
				parts = append(parts, fmt.Sprintf("%s=%.2f", l.name, l.score))
				if l.hasHist { // palette confirms this locator's match
					sum += wColorHist * l.hist
					wsum += wColorHist
					parts = append(parts, fmt.Sprintf("palette=%.2f", l.hist))
				}
			}
		}
		colS := -1.0
		if hasCol {
			colS = 0
			if s := h.colorScoreAround(c.x, c.y, wantCol, tol); s > 0 {
				colS = s
			}
			sum += wColor * colS
			wsum += wColor
		}
		evidence := 0.0
		if wsum > 0 {
			evidence = sum / wsum
		}
		// Position is a selection prior measured from the recorded point AND, if known, the
		// place this anchor last resolved — so a target that has DRIFTED toward last-known is
		// favoured there, letting the search follow it across runs instead of snapping back.
		pos := positionScore(c.x, c.y, px, py)
		if hasLast {
			if lp := positionScore(c.x, c.y, lastKnown[0], lastKnown[1]); lp > pos {
				pos = lp
			}
		}
		sel := evidence * (positionFloor + (1-positionFloor)*pos)
		// INFO: the per-candidate breakdown — the "why" behind where it clicks.
		mlog.Debug("find: candidate", "at", fmt.Sprintf("(%d,%d)", c.x, c.y),
			"signals", strings.Join(parts, ","), "colour", sigStr(hasCol, colS),
			"evidence", round2(evidence), "score", round2(sel))
		if ci == 0 {
			pSel = sel // the recorded point, always evaluated first
		} else if sel < pSel+moveMargin() {
			// A moved candidate that doesn't CLEARLY beat the demonstrated point is not worth
			// abandoning it for — stay where the user actually clicked.
			mlog.Debug("find: candidate too close to the recorded point's score — keeping the recorded point",
				"candidate", round2(sel), "recorded", round2(pSel))
			continue
		}
		if sel > bestSel {
			bestSel, best, bestConf = sel, [2]int{c.x, c.y}, evidence
		}
	}
	return best, bestConf
}

// windowFactor scales an anchor's confidence by how well the CURRENT foreground window's
// title matches the one recorded for the anchor — so a strong pixel/edge match in the
// WRONG window of a multi-window app (Steam's library vs friends vs store) can't produce a
// confident click. Only applied once a window was ACTIVATED this run (then the foreground
// is reliably the target, not the overlay that took focus to type the command). 1.0 when
// no window was recorded, none was activated, the titles agree, or the title is unreadable;
// a hard penalty on a clear mismatch, with partial credit for a shared significant word so
// a window whose title drifts (a game's level name) still passes.
func (h *Host) windowFactor(recorded string) float64 {
	if strings.TrimSpace(recorded) == "" || !h.didActivate() {
		return 1.0
	}
	cur := winctx.ForegroundTitle()
	f := windowMatchFactor(recorded, cur)
	if f < 1.0 {
		mlog.Debug("find: foreground window mismatch — penalising confidence",
			"recorded", recorded, "current", cur, "factor", round2(f))
	}
	return f
}

// windowMatchFactor scores how well current matches recorded (case-insensitive): 1.0 when
// either title contains the other or current is unreadable; windowPartialFactor on a shared
// significant word (a title that drifts — a game level name); windowMismatchFactor on a
// clear mismatch. Pure, so it's unit-testable without a live foreground window.
func windowMatchFactor(recorded, current string) float64 {
	r := strings.ToLower(strings.TrimSpace(recorded))
	c := strings.ToLower(strings.TrimSpace(current))
	if r == "" || c == "" || strings.Contains(c, r) || strings.Contains(r, c) {
		return 1.0
	}
	if sharesSignificantWord(r, c) {
		return windowPartialFactor
	}
	return windowMismatchFactor
}

// sharesSignificantWord reports whether a and b share a word of ≥4 letters (skipping short
// connectors), so a title that only partly changed still counts as the same window.
func sharesSignificantWord(a, b string) bool {
	bw := strings.Fields(b)
	for wa := range strings.FieldsSeq(a) {
		if len(wa) < 4 {
			continue
		}
		if slices.Contains(bw, wa) {
			return true
		}
	}
	return false
}

// Window-context confidence multipliers: a clear wrong-window match is knocked well below
// the find threshold (fail-safe), while a partial title overlap keeps most of the score.
const (
	windowPartialFactor  = 0.7
	windowMismatchFactor = 0.35
)

// sigStr formats a signal's score for logging, or "-" when it wasn't available.
func sigStr(has bool, s float64) string {
	if !has || s < 0 {
		return "-"
	}
	return fmt.Sprintf("%.2f", s)
}

// matchEpsilon (px): a match this close to the recorded point counts as "didn't move".
const matchEpsilon = 4

// searchMargin (px): how far around the recorded point the image is searched when no
// explicit region is set — a box ~5× the captured patch, so a slightly-shifted target is
// still found without scanning (and re-capturing) the whole desktop every poll.
const searchMargin = 250

// maxDriftPx (px): the farthest a last-known location may sit from the recorded point and
// still be trusted — beyond this the cache entry is treated as stale/poisoned and ignored.
const maxDriftPx = 600

// Evidence weights and the position selection floor. Image (raw pixels) and edge (the
// outline — robust to recolour) are the locators; colour is the confirm.
const (
	wImage        = 0.5
	wEdge         = 0.3
	wColor        = 0.2
	wColorHist    = 0.15 // palette confirm on an image match
	positionFloor = 0.7
)

// Thresholds for FOLLOWING the image to a new location (vs. clicking the recorded point).
// The demonstrated point is the safe default: a move is trusted only when a locator is a
// STRONG match there, colour agrees, AND the moved candidate clearly out-scores the
// recorded point (by moveMargin) — so a degraded crop, a lenient brightness match, or a
// look-alike button in a busy menu can't drag the click off where you actually clicked.
const corroborationFloor = 0.5

// locateFloor is the minimum locator score to consider FOLLOWING the image to a new
// location. Derived from cvSensitivity ($MARCO_CV_SENSITIVITY); s=0.5 → 0.90 (legacy).
func locateFloor() float64 { return cvLerp(1.0, 0.80) }

// moveMargin is how much a moved candidate must out-score the recorded point before the
// click follows it there. Derived from cvSensitivity; s=0.5 → 0.12 (legacy).
func moveMargin() float64 { return cvLerp(0.22, 0.02) }

// colorCorroborates reports whether the colour at a candidate location supports the
// target being there — required before the scorer follows the image to a new location.
// With no colour signal recorded, we don't follow on image alone (too risky).
func (h *Host) colorCorroborates(x, y int, wantCol uint32, hasCol bool, tol int) bool {
	if !hasCol {
		return false
	}
	return h.colorScoreAround(x, y, wantCol, tol) >= corroborationFloor
}

// hoverSettle is how long to wait after gliding the cursor onto the recorded point before
// matching (AND before the click fires), so a menu transition / hover highlight / fade-in
// finishes rendering — the "let the screen settle before clicking" beat. The wait loop
// (250ms polls) covers a slower animation; this sets the FIRST attempt's patience. Tunable
// via $MARCO_FIND_SETTLE_MS (default 150ms; 0 disables).
func hoverSettle() time.Duration {
	if v := strings.TrimSpace(os.Getenv("MARCO_FIND_SETTLE_MS")); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms >= 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return 150 * time.Millisecond
}

// findHoverEnabled gates the hover-settle move (default on). $MARCO_FIND_HOVER=0/off
// disables it for a UI where moving the pointer first would be undesirable.
func findHoverEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_FIND_HOVER"))) {
	case "0", "off", "false", "no":
		return false
	}
	return true
}

// findWiggleEnabled gates the per-poll hover wiggle (default on, only happens when hover is
// on too). $MARCO_FIND_WIGGLE=0/off keeps the cursor still after the initial hover.
func findWiggleEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_FIND_WIGGLE"))) {
	case "0", "off", "false", "no":
		return false
	}
	return true
}

// wiggleStep returns a tiny ±1-2px offset cycling around the hover point, so each poll nudges
// the cursor and the UI keeps firing the pointer-move events that sustain a hover highlight.
func wiggleStep(n int) (int, int) {
	steps := [...][2]int{{0, 0}, {1, 0}, {2, 1}, {1, 2}, {0, 1}, {-1, 1}, {-2, 0}, {-1, -1}, {0, -2}, {1, -1}}
	s := steps[n%len(steps)]
	return s[0], s[1]
}

// Cursor GLIDE (pull-to-target) for the hover-settle. Injecting ONE absolute MOVE teleports
// the pointer straight to the target; a game UI that lights a control only when the cursor
// physically TRAVELS onto it (RL's menu tiles) never sees the entry, so the hovered template
// never matches. Gliding steps the move — a short trail of injected MOVE events from where the
// cursor is now to the target — so the UI registers the pointer arriving. $MARCO_FIND_GLIDE=0
// reverts to a single jump.
const (
	glideStepPx    = 22 // ~pixels between interpolation steps
	glideMaxSteps  = 48 // cap so a cross-screen pull still finishes quickly
	glideStepDelay = 6 * time.Millisecond
)

func findGlideEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_FIND_GLIDE"))) {
	case "0", "off", "false", "no":
		return false
	}
	return true
}

// glide pulls the cursor from its current position to (x,y) as a trail of injected MOVE
// events. Falls back to a single move when gliding is off, the start point is unknown (the
// off-Windows stub reports (0,0)), or the target is within one step — so behaviour is
// unchanged where a real trail isn't possible or isn't needed. Honours ctx cancellation
// between steps.
func (h *Host) glide(ctx context.Context, x, y int) error {
	if !findGlideEnabled() {
		return h.b.move(ctx, x, y)
	}
	sx, sy := winctx.CursorPos()
	dx, dy := x-sx, y-sy
	dist := iabs(dx)
	if a := iabs(dy); a > dist {
		dist = a // Chebyshev distance — steps scale with the longer axis
	}
	steps := dist / glideStepPx
	if (sx == 0 && sy == 0) || steps <= 1 {
		return h.b.move(ctx, x, y)
	}
	if steps > glideMaxSteps {
		steps = glideMaxSteps
	}
	for i := 1; i < steps; i++ {
		t := float64(i) / float64(steps)
		if err := h.b.move(ctx, sx+int(float64(dx)*t), sy+int(float64(dy)*t)); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(glideStepDelay):
		}
	}
	return h.b.move(ctx, x, y) // land exactly on the target
}

// doDrag performs a press-hold-release drag from (FromX,FromY) to (ToX,ToY) — the real drag
// primitive behind a route's `do OS's Drag with …`. Absolute coordinates; Button defaults left.
func (h *Host) doDrag(c runtime.HostCall) (string, runtime.Value, error) {
	if !c.Input.IsSet() {
		return fail("drag needs a Drag set with FromX, FromY, ToX, ToY")
	}
	s := c.Input.AsSet()
	fx, fy := setInt(s, "FromX"), setInt(s, "FromY")
	tx, ty := setInt(s, "ToX"), setInt(s, "ToY")
	btn := strings.TrimSpace(textOf(s, "Button"))
	if btn == "" {
		btn = "left"
	}
	// NOTHING IS DRAGGED BY A RUN THAT HAS ALREADY STOPPED. Here as well as inside the
	// gesture, and for the reason `doKeyDown` gives: here it holds for EVERY backend,
	// including the ones a test can drive, so "a stopped run presses nothing" is a claim
	// about the host rather than about Windows. The gesture keeps its own check because
	// the press is the event that must not happen and the gesture is what sends it.
	if err := c.Ctx.Err(); err != nil {
		return fail(err.Error())
	}
	mlog.Debug("drag", "from", fmt.Sprintf("(%d,%d)", fx, fy), "to", fmt.Sprintf("(%d,%d)", tx, ty), "button", btn)
	// AND A DRAG THE MACHINE WOULD NOT TAKE IS KEPT, like a press, a hold and a keystroke.
	//
	// It used to go out through `ok`, which turns an error into a failed step and records it
	// nowhere — so a drag the machine refused was absent from the account the command reads
	// and the run reported success about a gesture that never happened. Worse, underneath it
	// the backend was discarding what `SendInput` returned, so there was usually no error to
	// report in the first place; both halves had to go.
	what := fmt.Sprintf("dragging from %d,%d to %d,%d", fx, fy, tx, ty)
	err := h.b.drag(c.Ctx, btn, fx, fy, tx, ty)
	// AND A BUTTON THE MACHINE WOULD NOT LET GO OF IS KEPT SEPARATELY, the same two calls a
	// keystroke gets and for the same reason.
	//
	// `delivered` alone was not enough and the gap was exactly the one the chord slice
	// closed for keys: a run STOPPED mid-drag discards its error on purpose, so a stuck
	// button reported only through the returned error reached nobody — the record said
	// `cancelled` with nothing beside it while the button was down, dragging whatever the
	// pointer passed over. This account is the one a cancellation cannot discard.
	h.noteHeld(what, err)
	return h.delivered(what, err)
}

// cvOff reports whether computer vision / anchors are OFF — the DEFAULT now. The CV anchor
// stack isn't reliable yet, so it's behind a feature flag: off unless explicitly enabled with
// $MARCO_CV=on/max (or $MARCO_ANCHORS=1/on). When off, doFind resolves straight to the recorded
// coordinate (no hover/matching) and learn preserves the recorded timings. Kept consistent with
// recorder.anchorsEnabled and orchestrator.cvOff.
func cvOff() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_CV"))) {
	case "max", "1", "on", "all", "true", "yes":
		return false // CV explicitly on
	case "0", "off", "false", "no", "none":
		return true // CV explicitly off
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_ANCHORS"))) {
	case "1", "on", "true", "yes":
		return false // anchors explicitly on
	}
	return true // default: CV off
}

// cvSensitivity is the master CV looseness dial (0 = strict, 1 = loose), default 0.5.
// The overlay's "cv find" slider writes $MARCO_CV_SENSITIVITY; findConfidence, locateFloor
// and moveMargin all derive from it (via cvLerp), so one control tunes how eagerly an
// anchor is trusted and followed. A specific per-knob env override (e.g.
// $MARCO_FIND_CONFIDENCE) still wins. Default 0.5 reproduces the legacy fixed values
// exactly (conf 0.6, locateFloor 0.90, moveMargin 0.12).
func cvSensitivity() float64 {
	if v := strings.TrimSpace(os.Getenv("MARCO_CV_SENSITIVITY")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			return f
		}
	}
	return 0.5
}

// cvLerp interpolates a knob between its strict (sensitivity 0) and loose (sensitivity 1)
// ends by the current cvSensitivity.
func cvLerp(strict, loose float64) float64 { return strict + (loose-strict)*cvSensitivity() }

// findTimeoutScale stretches the SHORT click-gate timeout when the dial is loosened past
// neutral, so a slower-appearing button gets more time before the route gives up and clicks
// its recorded coordinate anyway ("it went too fast"). 1× at/below 0.5 (legacy), up to 4× at
// full-loose. Long wait-for-screen barriers are left untouched (they're already generous).
func findTimeoutScale() float64 {
	s := cvSensitivity()
	if s <= 0.5 {
		return 1.0
	}
	return 1.0 + (s-0.5)*6.0 // 0.5 → 1×, 1.0 → 4×
}

// findConfidence is the score an anchor must reach to be clicked at its located point.
// Derived from cvSensitivity ($MARCO_CV_SENSITIVITY); s=0.5 → 0.6 (legacy default).
// $MARCO_FIND_CONFIDENCE (0..1) overrides outright.
func findConfidence() float64 {
	if v := strings.TrimSpace(os.Getenv("MARCO_FIND_CONFIDENCE")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && f <= 1 {
			return f
		}
	}
	return cvLerp(0.95, 0.25)
}

// anchorPoint resolves the anchor's recorded click point — window-relative against the
// activated window (so colour/position probe the RIGHT pixel after an Activate), else the
// absolute recorded coordinate (the same rule clicks use).
func (h *Host) anchorPoint(set *runtime.Set) (x, y int) {
	x, y = setInt(set, "X"), setInt(set, "Y")
	rxv, okx := set.Get("RelX")
	ryv, oky := set.Get("RelY")
	rx, okrx := rxv.AsNumber()
	ry, okry := ryv.AsNumber()
	if okx && oky && okrx && okry && h.didActivate() {
		if left, top, _, ok := h.windowOrigin(); ok {
			return left + int(rx), top + int(ry)
		}
	}
	return x, y
}

// colorSampleRadius is how far around a probe point colorScoreAround looks for the
// recorded colour — a few pixels, enough to absorb a 1px anti-aliasing / rounding shift
// (or the cursor sitting on the exact pixel) without blending in neighbouring controls.
const colorSampleRadius = 2

// colorScoreAround returns the BEST colorScore for the recorded colour within a small
// neighbourhood of (x,y) — "does the recorded colour appear right here", robust to a tiny
// positional shift in a way a single GetPixel is not (the static-menu false-low-colour
// bug). Taking the max (not an average) avoids blending a button edge into its background.
// Returns -1 when no pixel could be read.
func (h *Host) colorScoreAround(x, y int, want uint32, tol int) float64 {
	best := -1.0
	for dy := -colorSampleRadius; dy <= colorSampleRadius; dy++ {
		for dx := -colorSampleRadius; dx <= colorSampleRadius; dx++ {
			if got, err := h.scr.Pixel(x+dx, y+dy); err == nil {
				if s := colorScore(got, want, tol); s > best {
					best = s
				}
			}
		}
	}
	return best
}

// colorScore rates how close a probed colour is to the recorded one, tolerance-aware:
// within tol it's strong (1.0 at an exact match, 0.8 at the tolerance edge); beyond tol
// it falls to 0 by 3×tol. So a match scores high and a clear mismatch scores low,
// rather than the linear-to-128 ramp that left moderately-off colours ambiguous.
func colorScore(got, want uint32, tol int) float64 {
	if tol <= 0 {
		tol = 1
	}
	d := maxChannelDiff(got, want)
	if d <= tol {
		return 1 - 0.2*float64(d)/float64(tol)
	}
	if s := 0.8 - 0.8*float64(d-tol)/float64(2*tol); s > 0 {
		return s
	}
	return 0
}

func maxChannelDiff(a, b uint32) int {
	d := iabs(int(a>>16&0xff) - int(b>>16&0xff))
	if g := iabs(int(a>>8&0xff) - int(b>>8&0xff)); g > d {
		d = g
	}
	if bl := iabs(int(a&0xff) - int(b&0xff)); bl > d {
		d = bl
	}
	return d
}

// positionScore is 1 at the recorded point and falls linearly to 0 at posMaxDist.
func positionScore(x, y, px, py int) float64 {
	const posMaxDist = 160.0
	dx, dy := float64(x-px), float64(y-py)
	if s := 1 - math.Hypot(dx, dy)/posMaxDist; s > 0 {
		return s
	}
	return 0
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

func textOf(set *runtime.Set, key string) string {
	v, _ := set.Get(key)
	return v.AsText()
}

// okPoint returns an ok status carrying a Point Value — the coordinate a resolver
// settled on (image match centre or the colour-probed click point).
func okPoint(x, y int) (string, runtime.Value, error) {
	pt := runtime.NewSet()
	pt.Put("X", runtime.Number(float64(x)))
	pt.Put("Y", runtime.Number(float64(y)))
	return "ok", runtime.SetVal(pt), nil
}

// parseHexColor reads a "0xRRGGBB" string (the format OS's Color emits) into a
// packed RGB value. ok=false for an empty or malformed string.
func parseHexColor(s string) (uint32, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}

func iabs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// normAbsolute maps an absolute desktop pixel (x,y) to SendInput's ABSOLUTE
// coordinate space (0..65535) over the virtual desktop, given the virtual screen's
// origin (vsLeft,vsTop — negative when a monitor sits left/above the primary) and
// size (vsW,vsH). MOUSEEVENTF_ABSOLUTE|MOUSEEVENTF_VIRTUALDESK then lands the cursor
// on the exact pixel on ANY monitor — unlike SetCursorPos, the click can carry these
// coords itself, so move+down+up are one atomic, race-free batch. Pure, so it's
// unit-tested without a live desktop.
func normAbsolute(x, y, vsLeft, vsTop, vsW, vsH int) (nx, ny int32) {
	dw, dh := vsW-1, vsH-1
	if dw < 1 {
		dw = 1
	}
	if dh < 1 {
		dh = 1
	}
	ax := ((x-vsLeft)*65535 + dw/2) / dw // +half-step → round to nearest
	ay := ((y-vsTop)*65535 + dh/2) / dh
	return int32(clampInt(ax, 0, 65535)), int32(clampInt(ay, 0, 65535))
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func setInt(set *runtime.Set, field string) int {
	if v, ok := set.Get(field); ok {
		if n, ok2 := v.AsNumber(); ok2 {
			return int(n)
		}
	}
	return 0
}

// doSpam starts (or replaces) the single background spam loop. Input is a set
// { Key: text, Every: ms } to spam a key, or { Button: text, Every: ms } to spam
// a mouse click. Returns ok immediately; the loop runs until StopSpam (or a new
// Spam) cancels it. The loop's context is independent of the calling frame so it
// outlives the `do OS's Spam` that started it.
func (h *Host) doSpam(c runtime.HostCall) (string, runtime.Value, error) {
	set := c.Input.AsSet()
	if set == nil {
		return fail("spam needs a set with Key or Button, and Every")
	}
	keyVal, _ := set.Get("Key")
	btnVal, _ := set.Get("Button")
	everyVal, _ := set.Get("Every")
	key := keyVal.AsText()
	button := btnVal.AsText()
	if key == "" && button == "" {
		return fail("spam needs a Key or a Button")
	}
	every, okN := everyVal.AsNumber()
	if !okN || every <= 0 {
		every = 50
	}
	interval := time.Duration(every) * time.Millisecond

	h.mu.Lock()
	if h.spamCancel != nil {
		h.spamCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.spamCancel = cancel
	h.mu.Unlock()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if key != "" {
				_ = h.b.key(ctx, key, 0)
			} else {
				_ = h.b.click(ctx, button)
			}
			select {
			case <-time.After(interval):
			case <-ctx.Done():
				return
			}
		}
	}()
	return "ok", runtime.Absent(), nil
}

// doStopSpam cancels the background spam loop if one is running.
func (h *Host) doStopSpam() (string, runtime.Value, error) {
	h.mu.Lock()
	if h.spamCancel != nil {
		h.spamCancel()
		h.spamCancel = nil
	}
	h.mu.Unlock()
	return "ok", runtime.Absent(), nil
}

// doRoll returns a random integer in [Min, Max] (defaults 1..100). Input may be
// a set { Min, Max }, or a single number used as Max.
func (h *Host) doRoll(c runtime.HostCall) (string, runtime.Value, error) {
	lo, hi := 1, 100
	if n, okN := c.Input.AsNumber(); okN {
		hi = int(n)
	} else if set := c.Input.AsSet(); set != nil {
		if v, ok2 := set.Get("Min"); ok2 {
			if n, ok3 := v.AsNumber(); ok3 {
				lo = int(n)
			}
		}
		if v, ok2 := set.Get("Max"); ok2 {
			if n, ok3 := v.AsNumber(); ok3 {
				hi = int(n)
			}
		}
	}
	if lo > hi {
		lo, hi = hi, lo
	}
	n := lo + rand.Intn(hi-lo+1)
	return "ok", runtime.Number(float64(n)), nil
}

var eightBallResponses = []string{
	"It is certain.", "It is decidedly so.", "Without a doubt.",
	"Yes, definitely.", "You may rely on it.", "As I see it, yes.",
	"Most likely.", "Outlook good.", "Yes.", "Signs point to yes.",
	"Reply hazy, try again.", "Ask again later.", "Better not tell you now.",
	"Cannot predict now.", "Concentrate and ask again.",
	"Don't count on it.", "My reply is no.", "My sources say no.",
	"Outlook not so good.", "Very doubtful.",
}

func (h *Host) doEightBall() (string, runtime.Value, error) {
	return "ok", runtime.Text(eightBallResponses[rand.Intn(len(eightBallResponses))]), nil
}

var (
	nameStarts = []string{"Ael", "Bran", "Cas", "Dor", "El", "Fae", "Gar", "Har",
		"Il", "Jas", "Kel", "Lor", "Mir", "Nyx", "Or", "Phe", "Quel", "Rae",
		"Syl", "Tal", "Ul", "Val", "Wren", "Xan", "Yor", "Zel"}
	nameMids = []string{"a", "an", "el", "en", "ia", "iel", "in", "ion", "ir",
		"is", "or", "oth", "un", "wyn"}
	nameEnds = []string{"ath", "dor", "en", "eth", "ion", "ir", "is", "ith",
		"or", "oth", "rath", "rin", "thar", "wyn"}
)

func (h *Host) doName() (string, runtime.Value, error) {
	name := nameStarts[rand.Intn(len(nameStarts))]
	if rand.Intn(2) == 1 {
		name += nameMids[rand.Intn(len(nameMids))]
	}
	name += nameEnds[rand.Intn(len(nameEnds))]
	return "ok", runtime.Text(name), nil
}

// doClick: a Point input moves there first; a text input names the button;
// absent input clicks left at the current position.
func (h *Host) doClick(c runtime.HostCall) (string, runtime.Value, error) {
	x, y, present, why := h.clickTarget(c.Input)
	// THE REFUSAL IS THE OUTCOME, and it says which term disagreed. A guarded press
	// that could not be checked does not quietly become an unguarded one.
	if why != "" {
		mlog.Info("click: refused", "why", why)
		return fail(why)
	}
	if present {
		mlog.Debug("click: at point", "x", x, "y", y, "button", "left")
		// Position and click in ONE atomic SendInput batch (move+down+up, all carrying
		// the absolute target). A separate SetCursorPos-then-click races: the injected
		// down/up can register before the move settles (a game's raw-input loop, an
		// overlay LL mouse hook, DPI virtualisation), landing the click at a STALE
		// position — the "resolved coords are right but it clicked the wrong thing" bug.
		// AND A DELIVERY FAILURE IS KEPT. The guard passing and the machine then
		// refusing the input are the same thing from where a person is standing:
		// the press did not happen. See `Unperformed`.
		return h.delivered(fmt.Sprintf("a press at %d,%d", x, y),
			h.b.clickAt(c.Ctx, "left", x, y))
	}
	button := "left"
	if t := c.Input.AsText(); t != "" {
		button = t
	}
	mlog.Debug("click: at cursor", "button", button)
	return h.delivered("a "+button+" press where the cursor is",
		h.b.click(c.Ctx, button))
}

func (h *Host) doSleep(c runtime.HostCall) (string, runtime.Value, error) {
	ms, present := c.Input.AsNumber()
	if !present {
		return fail("sleep needs a number of milliseconds")
	}
	select {
	case <-time.After(time.Duration(ms) * time.Millisecond):
		return "ok", runtime.Absent(), nil
	case <-c.Ctx.Done():
		return "ok", runtime.Absent(), nil
	}
}

func (h *Host) doColor(c runtime.HostCall) (string, runtime.Value, error) {
	x, y, present := point(c.Input)
	if !present {
		return fail("color needs a Point with X and Y")
	}
	col, err := h.b.color(c.Ctx, x, y)
	if err != nil {
		return fail(err.Error())
	}
	return "ok", runtime.Text(fmt.Sprintf("0x%06X", col)), nil
}

// doFocus resolves ok when the active window's exe matches the given substring
// (case-insensitive), failed otherwise — used to gate window-scoped macros.
func (h *Host) doFocus(c runtime.HostCall) (string, runtime.Value, error) {
	spec := c.Input.AsText()
	exe, err := h.b.activeExe(c.Ctx)
	if err != nil {
		return fail(err.Error())
	}
	if spec == "" || strings.Contains(strings.ToLower(exe), strings.ToLower(spec)) {
		return "ok", runtime.Text(exe), nil
	}
	return "failed", runtime.Absent(), nil
}

// doRepeat presses a key on an interval until the frame is canceled. Input is a
// set { Key: text, Every: number-of-ms }. This mirrors the original MacroMarco
// Runners.Repeat continuous-spam pattern; cancellation (Esc/stop) ends it ok.
func (h *Host) doRepeat(c runtime.HostCall) (string, runtime.Value, error) {
	set := c.Input.AsSet()
	if set == nil {
		return fail("repeat needs a set with Key and Every")
	}
	keyVal, _ := set.Get("Key")
	everyVal, _ := set.Get("Every")
	key := keyVal.AsText()
	if key == "" {
		return fail("repeat needs a Key")
	}
	every, ok2 := everyVal.AsNumber()
	if !ok2 || every <= 0 {
		every = 50
	}
	interval := time.Duration(every) * time.Millisecond
	for {
		select {
		case <-c.Ctx.Done():
			return "ok", runtime.Absent(), nil
		default:
		}
		if err := h.b.key(c.Ctx, key, 0); err != nil {
			return fail(err.Error())
		}
		select {
		case <-time.After(interval):
		case <-c.Ctx.Done():
			return "ok", runtime.Absent(), nil
		}
	}
}

// ── helpers ──

// ok turns a backend error into a (status, data) pair: ok on nil, failed
// (with the message) otherwise.
func ok(err error) (string, runtime.Value, error) {
	if err != nil {
		return "failed", runtime.ErrVal(&runtime.Err{Message: err.Error()}), nil
	}
	return "ok", runtime.Absent(), nil
}

func fail(msg string) (string, runtime.Value, error) {
	return "failed", runtime.ErrVal(&runtime.Err{Message: msg}), nil
}

// clickTarget reads a click's target pixel from its Point. The DEFAULT is the
// recorded ABSOLUTE coordinate (X,Y) — exact for a fullscreen or in-place app and
// immune to focus games. A Point's RelX/RelY (offset inside the app window) is only
// resolved against the live window origin when this run has Activate'd a specific
// window: only then is "the foreground window" reliably the target app. Without an
// Activate, foreground may be the overlay (it holds focus when you type a command),
// whose origin would shift every click — the random-misclick cause — so we trust the
// recorded coordinate instead. Emits an INFO line breaking down every click.
func (h *Host) clickTarget(v runtime.Value) (x, y int, present bool, why string) {
	x, y, present = point(v)
	if !present {
		return x, y, false, ""
	}
	return h.resolve(v.AsSet(), x, y)
}

// resolve answers where a recorded point goes, from the set that describes it.
//
// Shared by a click, a Move, and the recorded point a Find hands back when there is no matching
// to do, because those are three ways into the same question and a guard that covered one of
// them would be a guard with two bypasses. `Anchor` and `Point` carry the same field names for
// exactly this reason.
func (h *Host) resolve(set *runtime.Set, x, y int) (px, py int, present bool, why string) {
	if set == nil {
		return x, y, true, ""
	}
	rxv, okx := set.Get("RelX")
	ryv, oky := set.Get("RelY")
	rx, okrx := rxv.AsNumber()
	ry, okry := ryv.AsNumber()
	hasRel := okx && oky && okrx && okry
	// ── THE GUARDED PATH, WHEN THE POINT ASKS FOR IT ──
	//
	// A point that says what it was measured from is asking to be checked against that
	// window immediately before input, rather than resolved against whatever is in
	// front. The ASKING is the opt-in: nothing else turns this on, and a point that
	// says nothing falls through to everything below, unchanged.
	//
	// The basis ALONE is the opt-in, and not the basis together with a usable offset.
	// Requiring both here read as caution and was a bypass: a point that stated a
	// window and carried no offset to resolve inside it missed the condition and went
	// down the legacy path, which presses the recorded coordinate without checking
	// anything. Whether the rest of the terms are there is the guard's question to
	// answer — and its answer is a refusal, not a fallback.
	if textOf(set, "RelTo") != "" {
		gx, gy, gwhy := h.guarded(set, x, y)
		return gx, gy, gwhy == "", gwhy
	}
	if !hasRel {
		mlog.Debug("click: absolute", "point", fmt.Sprintf("(%d,%d)", x, y), "why", "no window-relative offset recorded")
		return x, y, true, ""
	}
	if !h.didActivate() {
		mlog.Debug("click: absolute", "point", fmt.Sprintf("(%d,%d)", x, y),
			"why", "no Activate this run — trusting the recorded point, not the foreground origin")
		return x, y, true, ""
	}
	left, top, name, ok := h.windowOrigin()
	if !ok {
		mlog.Debug("click: absolute", "point", fmt.Sprintf("(%d,%d)", x, y), "why", "no foreground window")
		return x, y, true, ""
	}
	rxi, ryi := int(rx), int(ry)
	// The full breakdown: what was recorded, the relative offset it wanted, which
	// window the origin came from (the "why"), and where it actually landed.
	mlog.Debug("click: window-relative",
		"recorded", fmt.Sprintf("(%d,%d)", x, y),
		"wantRel", fmt.Sprintf("(%d,%d)", rxi, ryi),
		"originWindow", name,
		"origin", fmt.Sprintf("(%d,%d)", left, top),
		"clicked", fmt.Sprintf("(%d,%d)", left+rxi, top+ryi))
	return left + rxi, top + ryi, true, ""
}

// didActivate reports whether this run brought a window to the front (Activate),
// which is what makes window-relative origin resolution trustworthy.
func (h *Host) didActivate() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.activated
}

// windowOrigin returns the foreground window's origin and app name, snapshotted on
// first use and cached for the rest of the route so window-relative clicks all resolve
// against the same window. The activate case clears the cache so clicks after a focus
// change re-read against the new window. Logs the snapshot (which window it locked on).
func (h *Host) windowOrigin() (left, top int, name string, ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.originSet {
		return h.originL, h.originT, h.originName, true
	}
	left, top, name, ok = winctx.ForegroundInfo()
	if ok {
		h.originL, h.originT, h.originName, h.originSet = left, top, name, true
		mlog.Debug("origin: snapshot", "window", name, "origin", fmt.Sprintf("(%d,%d)", left, top))
	}
	return left, top, name, ok
}

// point reads X and Y number fields from a set Value. Returns present=false if
// the value isn't a set with both fields.
func point(v runtime.Value) (x, y int, present bool) {
	set := v.AsSet()
	if set == nil {
		return 0, 0, false
	}
	xv, okx := set.Get("X")
	yv, oky := set.Get("Y")
	if !okx || !oky {
		return 0, 0, false
	}
	xn, okxn := xv.AsNumber()
	yn, okyn := yv.AsNumber()
	if !okxn || !okyn {
		return 0, 0, false
	}
	return int(xn), int(yn), true
}

// doClipboardGet reads the clipboard as text.
//
// Reports whether it HELD text separately from what that text was. A clipboard holding
// an image is not a clipboard holding "", and a caller saving the contents to restore
// them later must not overwrite an image with an empty string.
func (h *Host) doClipboardGet(c runtime.HostCall) (string, runtime.Value, error) {
	text, isText, empty, err := clipboardText()
	if err != nil {
		return fail(err.Error())
	}
	set := runtime.NewSet()
	set.Put("Text", runtime.Text(text))
	set.Put("IsText", runtime.Bool(isText))
	// Empty separates "nothing is on the clipboard" from "something that is not text".
	// Only the first can be borrowed and restored faithfully.
	set.Put("Empty", runtime.Bool(empty))
	return "ok", runtime.SetVal(set), nil
}

// doClipboardSet replaces the clipboard with text.
func (h *Host) doClipboardSet(c runtime.HostCall) (string, runtime.Value, error) {
	text := c.Input.AsText()
	if s := c.Input.AsSet(); s != nil {
		if v, ok := s.Get("Text"); ok {
			text = v.AsText()
		}
	}
	if err := setClipboardText(text); err != nil {
		return fail(err.Error())
	}
	return "ok", runtime.Absent(), nil
}

// bindAt remembers the window Activate settled on, and is the preparation half of the guarded
// contract.
//
// It clears the legacy origin cache for the same reason it always did — a new window means a new
// origin — and it clears the binding too when Activate did not find one, because a stale handle
// is worse than none: it names a window that may since have closed, or whose handle has been
// reused by something else entirely.
//
// # And it reads who owns the handle, now, while there is still something to read
//
// The process id of the bound window is taken here and nowhere else, because this is the only
// instant at which the answer is about the window the route prepared. Asked again at the press it
// would be the identity of whatever holds the handle by then, compared against itself — a check
// that cannot fail, which is the shape every one of these findings has had.
//
// A window that will not say which process owns it leaves `boundPID` zero, and a guarded press
// then refuses: nothing was prepared that a press could be about.
func (h *Host) bindAt(hwnd uintptr, app string) {
	pid := uint32(0)
	if hwnd != 0 {
		pid = windowFacts(hwnd).PID
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.activated = true
	h.originSet = false
	h.bound, h.boundApp, h.boundPID = hwnd, app, pid
	if hwnd != 0 {
		mlog.Debug("bound: the window a press will be checked against",
			"app", app, "window", fmt.Sprintf("hwnd:%#x", hwnd), "pid", pid)
	}
}

// binding is the window this run bound, what was asked for, and who owned it then.
func (h *Host) binding() (uintptr, string, uint32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.bound, h.boundApp, h.boundPID
}

// supportedBasis is the only thing a press can state it was measured from.
//
// A named constant rather than a comparison against "not empty", because a point claiming some
// other basis is a point this does not know how to check, and resolving it as origin-plus-offset
// in a window would be answering a question it did not ask. A basis relative to a control, or to
// a screen, or to a monitor, is different arithmetic — when one exists it will be a term here and
// a case in `guarded`, not a string that happened to be non-empty.
const supportedBasis = macroir.RelToWindow

// farthestPixel is further than any window dimension or offset on any desktop, and is the upper
// bound on a term being believed at all. A recorded size of four billion is not a big window; it
// is a number that was not a measurement by the time it got here.
const farthestPixel = 1 << 20

// measuredWindow is the demonstration's whole side of the comparison: what the window was like,
// and where inside it the press went.
type measuredWindow struct {
	w, h, dpi int
	process   string
	rx, ry    int
}

// measuredFrom reads and validates everything a guarded point says about its window, or returns
// the sentence explaining why it cannot be checked.
//
// It is one function and runs before any question is put to the desktop, so that no check in
// `guarded` can be reached holding a term nobody validated — which is the shape the bypass had:
// each term was read at the point of use, and each one was read as "if present and sane", so an
// absent or malformed term meant that check silently did not happen.
func measuredFrom(set *runtime.Set) (measuredWindow, string) {
	var m measuredWindow
	// THE BASIS, AND IT MUST BE ONE THIS KNOWS.
	if basis := textOf(set, "RelTo"); !strings.EqualFold(basis, supportedBasis) {
		return m, fmt.Sprintf("this press says it was measured from %q, and the "+
			"only thing a press can be checked against is the %s it was "+
			"recorded in", basis, supportedBasis)
	}
	// THE WINDOW'S OWN TERMS, EVERY ONE REQUIRED.
	for _, need := range []struct {
		field, says string
		into        *int
	}{
		{"WinW", "how wide that window was", &m.w},
		{"WinH", "how tall that window was", &m.h},
		{"Dpi", "what scale it was drawn at", &m.dpi},
		{"RelX", "where across that window the press went", &m.rx},
		{"RelY", "where down that window the press went", &m.ry},
	} {
		// AN OFFSET MAY BE ZERO — the window's own top or left edge — and a size
		// may not. Whether an offset is INSIDE the window is a different
		// complaint, asked once both of them have been read.
		least := 1.0
		if need.field == "RelX" || need.field == "RelY" {
			least = -farthestPixel
		}
		n, stated, usable := wholeTerm(set, need.field, least)
		switch {
		case !stated:
			return m, fmt.Sprintf("this press was recorded inside a window and "+
				"does not say %s, so there is nothing to check it "+
				"against — demonstrate it again", need.says)
		case !usable:
			return m, fmt.Sprintf("this press says %s in a way that is not a "+
				"measurement, so it cannot be checked — demonstrate it "+
				"again", need.says)
		}
		*need.into = n
	}
	if m.process = textOf(set, "Process"); m.process == "" {
		return m, "this press was recorded inside a window and does not say which " +
			"program owned it, so a window of the right size and scale " +
			"belonging to something else would pass — demonstrate it again"
	}
	// ── AND THE PRESS MUST BE INSIDE THE WINDOW IT IS AN OFFSET INTO ──
	//
	// Every check after this one is about whether the window still matches, and all of
	// them can pass while the arithmetic sends the cursor out of the window entirely:
	// origin plus an offset is a point on the desktop, not a point in an app, and the
	// window's size is the only thing that says which. An offset past its edge is input
	// aimed at whatever happens to be next to the target — the neighbouring app, the
	// desktop, another monitor — under a guard that has just confirmed the target is
	// exactly as demonstrated.
	//
	// The RECORDED rectangle is the one it must be inside, because that is what the
	// offset was measured in; by the time the press happens the live one has been shown
	// to be the same size, so there is no second rectangle to choose between.
	if m.rx < 0 || m.ry < 0 || m.rx >= m.w || m.ry >= m.h {
		return m, fmt.Sprintf("this press is recorded %d,%d inside a %dx%d window, "+
			"which is outside it, so resolving it would aim at whatever is "+
			"beside that window rather than at anything in it",
			m.rx, m.ry, m.w, m.h)
	}
	return m, ""
}

// wholeTerm reads one recorded number, telling ABSENT from MALFORMED from usable.
//
// Three outcomes and not two, because they are three different recordings and the person reading
// the refusal is the one who has to tell them apart: a term the route never states is a
// recording that was truncated or made by an older build, and a term stated as something that is
// not a measurement is one that was edited by hand.
//
// `least` is the smallest honest value for that term. Fractions are refused as well as values out
// of range: half a pixel of window width is not a window that was measured.
func wholeTerm(set *runtime.Set, field string, least float64) (n int, stated, usable bool) {
	v, has := set.Get(field)
	if !has || v.IsAbsent() {
		return 0, false, false
	}
	f, numeric := v.AsNumber()
	if !numeric || f != math.Trunc(f) || f < least || f > farthestPixel {
		return 0, true, false
	}
	return int(f), true, true
}

// guarded resolves a press against the window it was demonstrated in, having checked that window
// is still that window — or refuses, saying which term disagreed.
//
// # Why it refuses instead of falling back
//
// The legacy path falls back to the recorded absolute coordinate whenever it cannot trust the
// origin, and that is right for a route that never measured anything: the coordinate is the only
// thing it has, it is exact for a fullscreen app, and the alternative is doing nothing. It is
// wrong here. A point that carries a window asked to be checked against it, and falling back
// would press a remembered pixel on a desktop that has just been shown not to match — which is
// the thing the owner's own instruction forbids: refuse rather than search elsewhere or guess.
//
// # Why every term is exact
//
// Because a tolerance would be a number nobody measured. A window one pixel narrower than it was
// demonstrated in has had something relayout inside it, or has not, and this cannot tell which;
// the honest answer is that the demonstration no longer describes the window. See
// [[ADR-248-replaying-a-demonstrated-layout]], where the same decision was made for the same
// reason.
//
// # Why a missing term is a refusal and not a skipped check
//
// This read the other way round first: a term the demonstration did not carry was not checked,
// because there was nothing to compare it against. That is the right rule for a route that
// measured nothing, and it is the wrong one here, because it makes the guard's strength depend on
// how much the recording happened to capture — and nothing in the route or the review says which
// terms were skipped. A press with no recorded DPI passed on a desktop redrawn at another scale;
// a press with no recorded program passed in a window belonging to something else entirely. The
// check was still there, still named in the log, and answering about less than it appeared to.
//
// So a stated basis now requires every term, and the recorder writes them together or not at all
// (see `newClick`): a press whose window would not report its size or its scale, or whose program
// had gone by the time the recording closed, is saved as an ordinary point and pressed by the
// rule every route before this was saved with. What cannot happen is a point that CLAIMS the
// window and omits what the window was — the one case where a fallback would be pressing a
// remembered pixel under cover of a guard.
//
// A term the DESKTOP will not report is a refusal for the same reason: this ran out of ways to
// tell, and "could not tell" is not "matches".
func (h *Host) guarded(set *runtime.Set, x, y int) (px, py int, why string) {
	px, py, why = h.checkGuarded(set, x, y)
	if why != "" {
		// ── THE RUN KEEPS ITS OWN REFUSALS ──
		//
		// Because a log sentence is not a result. A routine whose press was
		// refused did not do what it was asked, and whatever started it has to be
		// able to say so without reading output: `marco do` turns a non-empty list
		// into a failed command, and director sees the same refusal through the
		// capability status its runner already records.
		h.didNotDo(why)
	}
	return px, py, why
}

// didNotDo keeps one sentence about something this run did not do.
func (h *Host) didNotDo(why string) {
	h.mu.Lock()
	h.undone = append(h.undone, why)
	h.mu.Unlock()
}

// stillHolding is something the machine would not let go of: a key, or a mouse button.
//
// AN INTERFACE AND NOT A LIST OF TYPES, so a third device cannot be added without being noticed
// here. What the account needs from one is only its sentence — it says what the computer is now
// LIKE — and what it needs from the type is the undertaking that this is that kind of thing and
// not a failure. `errors.As` finds it through however many wrappings a stop put around it.
type stillHolding interface {
	error
	stillHolding()
}

// noteHeld keeps a key or a button the machine would not release, which is an account of its own.
//
// # Why it is not in `Unperformed`
//
// Because it is not a thing that did not happen. Every entry in that list is something the run
// was going to do and did not, and a caller reads it to decide that the routine stopped. This is
// the opposite shape: something that DID happen and has not finished — a modifier down on this
// computer, modifying every keystroke typed after it.
//
// And it has to survive a cancellation. A stopped run discards its error on purpose, because a
// stop is not a failure; the half of that error which is still happening must outlive the
// discarding, and an account the host keeps is what outlives it. The identity comes from the
// error rather than from its wording, so improving the sentence cannot quietly disconnect this.
//
// # And a KEY is not the only thing the machine can refuse to let go of
//
// A mouse button is the other one, and it is worse: a held left button drags whatever the pointer
// passes over. This looked for `notReleased` specifically, so the drag's stuck button went out
// through `delivered` — the list of things that did NOT happen — and never reached the channel a
// cancelled run cannot discard. A stop in the middle of a drag therefore read as `cancelled` with
// nothing beside it while the button was down, which is the exact defect the chord slice fixed for
// keys. It asks for `stillHolding` now: the shape of the thing, not the device.
func (h *Host) noteHeld(what string, err error) {
	var stuck stillHolding
	if !errors.As(err, &stuck) {
		return
	}
	said := fmt.Sprintf("after %s, %s", what, stuck.Error())
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, already := range h.held {
		if already == said {
			return
		}
	}
	h.held = append(h.held, said)
}

// NotReleased is the key this run may have left held down, or empty.
//
// Read by `marco` after the run, including — especially — after a run that was stopped. See
// `noteHeld`, and `cmd/marco`'s performOnePlay for what it does with it.
func (h *Host) NotReleased() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return strings.Join(h.held, " — and ")
}

// delivered reports a host call that sent input, keeping the sentence when the machine would not
// take it.
//
// Wrapping every input send rather than only the guarded ones, because the host cannot tell which
// contract the route it is serving was saved under and must not guess: it records what did not
// happen and the caller decides what that means. See `Unperformed`.
func (h *Host) delivered(what string, err error) (string, runtime.Value, error) {
	if err != nil {
		h.didNotDo(fmt.Sprintf("%s did not happen: %v", what, err))
	}
	return ok(err)
}

// Unperformed is everything this run did not do, in order, each with the sentence that said why.
//
// Three kinds, and the point of one list is that the caller does not have to care which:
//
//   - a press that could not be CHECKED against the window it was recorded in
//   - a press the machine would not DELIVER, having passed every check
//   - an application that never came FORWARD for one
//
// The second is the one this nearly missed. A guard that passes and a backend that then rejects
// the input are the same outcome from where a person is standing — the press did not happen —
// and an account holding only refusals would have let the command report success about it. A
// delivery failure is not a refusal, so a list named for refusals was the wrong list.
//
// Recorded for every route and acted on by one. The caller reads the contract its route was saved
// under and decides, so a legacy route reports exactly what it always did: it was saved under a
// contract which says a click is attempted and the run carries on, and changing that is not this
// contract's business.
func (h *Host) Unperformed() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.undone...)
}

// checkGuarded is the comparison itself; `guarded` is what remembers its answer.
func (h *Host) checkGuarded(set *runtime.Set, x, y int) (px, py int, why string) {
	m, why := measuredFrom(set)
	if why != "" {
		return 0, 0, why
	}
	wantW, wantH, wantDpi, wantProc := m.w, m.h, m.dpi, m.process
	rx, ry := m.rx, m.ry
	hwnd, app, boundPID := h.binding()
	if hwnd == 0 {
		// NOTHING WAS BOUND. A guarded point has no window to be about: either no
		// Activate ran, or the one that ran could not find its window. Refusing is
		// the whole of the opt-in — the alternative is the legacy fallback, which is
		// what this route asked not to have.
		return 0, 0, "this press was recorded inside a window and no window was " +
			"brought to the front for it, so there is nothing to measure it " +
			"against"
	}
	if boundPID == 0 {
		// THE WINDOW WAS FOUND AND ITS OWNER WAS NOT. Without the process that
		// held the handle at preparation time there is no way to tell this window
		// from a second copy of the same program that inherited the handle, and
		// the image-name check below would agree with both.
		return 0, 0, fmt.Sprintf("the %s window brought to the front would not "+
			"say which program it belongs to, so a press in it could not be "+
			"told from a press in another copy of the same program", app)
	}
	// ── AND THE WINDOW AS IT IS, NOW, IMMEDIATELY BEFORE THE INPUT ──
	//
	// Read fresh, never from the legacy origin cache. That cache exists to survive a
	// momentary focus flicker the legacy path cannot otherwise detect, and it is the
	// wrong instrument here for two reasons: it answers about the FIRST press of the
	// route, so a window moved halfway through sends every later press to the old
	// origin; and this path detects the flicker directly, through `Front`, so it has no
	// need to paper over one.
	f := windowFacts(hwnd)
	switch {
	case !f.Alive:
		return 0, 0, fmt.Sprintf("the %s window this was recorded in has closed", app)
	case !f.Front:
		return 0, 0, fmt.Sprintf("the %s window is no longer in front, so a press "+
			"would land in whatever is", app)
	case f.W == 0 || f.H == 0:
		return 0, 0, fmt.Sprintf("the %s window would not say how big it is", app)
	case f.Dpi == 0:
		// UNREADABLE IS NOT MATCHING. This press was recorded at a scale, and a
		// desktop that will not report the current one cannot be compared with it
		// — so the honest answer is that nobody can say whether the controls in
		// that window are where they were drawn.
		return 0, 0, fmt.Sprintf("the %s window would not say what scale it is "+
			"drawn at, and this press was recorded at %d dpi", app, wantDpi)
	case f.PID != boundPID:
		// AND THE HANDLE'S OWNER HAS CHANGED SINCE PREPARATION. The image-name
		// check below cannot see this: two copies of one program have the same
		// executable, so the window answering here would pass it while being a
		// window the route never brought to the front.
		return 0, 0, fmt.Sprintf("the window prepared for this press was %s "+
			"process %d and that handle now belongs to process %d, which is "+
			"another copy of the program and not the one activated",
			app, boundPID, f.PID)
	}
	// ── THE PROGRAM, THROUGH THE HANDLE'S CURRENT OWNER ──
	//
	// Asked of the owner now rather than the one bound, because window handles are
	// reused: the window that answered all of the above can belong to something else by
	// now, and everything it said would be true of that something else. A desktop that
	// will not name the program refuses, for the same reason the DPI does above.
	is := imageOf(f.PID)
	switch {
	case is == "":
		return 0, 0, fmt.Sprintf("the program that owns the %s window will not "+
			"say what it is, and this press was recorded in %s", app, wantProc)
	case !strings.EqualFold(is, wantProc):
		return 0, 0, fmt.Sprintf("this was recorded in %s and that window "+
			"now belongs to %s", wantProc, is)
	}
	// ── THE SHAPE, EXACTLY ──
	if f.W != wantW || f.H != wantH {
		return 0, 0, fmt.Sprintf("this was recorded in a %s window of "+
			"%dx%d and it is %dx%d now, so what is inside it is not "+
			"where it was", app, wantW, wantH, f.W, f.H)
	}
	// ── AND THE SCALE, AS ITS OWN TERM ──
	//
	// Separately from the size, because it is a separate fact. An application that
	// handles a DPI change relayouts every control inside a window whose outer
	// rectangle never moved, so the check above passes and everything in the window is
	// somewhere else.
	if f.Dpi != wantDpi {
		return 0, 0, fmt.Sprintf("this was recorded at %d dpi and the %s window "+
			"is at %d now, so what is inside it has been redrawn at another "+
			"scale", wantDpi, app, f.Dpi)
	}
	px, py = f.Left+rx, f.Top+ry
	mlog.Debug("click: guarded",
		"app", app, "window", fmt.Sprintf("hwnd:%#x", hwnd),
		"recorded", fmt.Sprintf("(%d,%d)", x, y),
		"wantRel", fmt.Sprintf("(%d,%d)", rx, ry),
		"origin", fmt.Sprintf("(%d,%d)", f.Left, f.Top),
		"shape", fmt.Sprintf("%dx%d at %d dpi", f.W, f.H, f.Dpi),
		"clicked", fmt.Sprintf("(%d,%d)", px, py))
	return px, py, ""
}

// windowFacts and imageOf are the two desktop questions the guarded press boundary asks.
//
// Variables so a test can stand a window in front of it. Nothing else replaces them, and the
// claim they exist for — that a press is REFUSED when the window it was recorded in is no longer
// that window — cannot be made to happen on a real desktop on purpose: it needs a window that
// closes, moves, resizes, loses focus and changes owner between two instants, one case at a
// time. See guardedpress_test.go.
// `bindWindow` is the third: the preparation half, so a test can run a SAVED route end to end —
// Activate, then the press — without a real window of somebody's game on the desktop.
var (
	windowFacts = winctx.Facts
	imageOf     = winctx.ImageOf
	bindWindow  = winctx.Bind
)
