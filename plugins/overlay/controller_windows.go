//go:build windows

package main

import (
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// leaderVK is the leader key's virtual-key code, derived from cfg.Leader
// ("`", "capslock", "tab", a letter, "f8" …). The config editor re-derives it
// live via applyLeaderHook.
var leaderVK = parseLeader(cfgLeader())

func init() { applyLeaderHook = func() { leaderVK = parseLeader(cfgLeader()) } }

func parseLeader(spec string) uint16 {
	switch s := strings.ToLower(spec); s {
	case "`", "backtick", "grave", "tilde":
		return 0xC0
	case "capslock", "caps":
		return 0x14
	case "tab":
		return 0x09
	case "\\", "backslash":
		return 0xDC
	case "/", "slash":
		return 0xBF
	case "[", "lbracket":
		return 0xDB
	default:
		if len(s) == 1 {
			c := s[0]
			if c >= 'a' && c <= 'z' {
				return uint16(c - 'a' + 'A')
			}
			if c >= '0' && c <= '9' {
				return uint16(c)
			}
		}
		if len(s) >= 2 && s[0] == 'f' {
			if n, err := strconv.Atoi(s[1:]); err == nil && n >= 1 && n <= 12 {
				return uint16(0x70 + n - 1)
			}
		}
		return 0xC0
	}
}

// The overlay's input layer is a global low-level keyboard hook (WH_KEYBOARD_LL),
// the same mechanism internal/recorder uses, so it captures the leader key and
// the typed command even while a game has focus (the HUD itself is never
// focused). Keys it consumes are SUPPRESSED so the game never sees them: the
// leader, the function hotkeys, and — while the command line is open — every key.
//
// The hook callback is a C callback and can only reach package state, so the
// editing/shift flags are package-level atomics it reads/writes synchronously
// (the suppression decision must be immediate); slower work (mutating the HUD
// buffer, emitting events) is handed to a processor goroutine over actCh.

var (
	user32 = syscall.NewLazyDLL("user32.dll")

	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
)

// point mirrors the Win32 POINT for GetCursorPos.
type point struct{ x, y int32 }

// cursorPos returns the cursor's virtual-desktop coordinates — the same space the
// recorder logs clicks in, so it's correct on a second monitor (coords can be
// negative left of / above the primary). ok=false if the call fails.
func cursorPos() (x, y int, ok bool) {
	var p point
	r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	if r == 0 {
		return 0, 0, false
	}
	return int(p.x), int(p.y), true
}

const (
	whKeyboardLL = 13

	wmKeyDown    = 0x0100
	wmKeyUp      = 0x0101
	wmSysKeyDown = 0x0104
	wmSysKeyUp   = 0x0105

	llkhfInjected = 0x00000010

	vkBack   = 0x08
	vkTab    = 0x09
	vkReturn = 0x0D
	vkShift  = 0x10
	vkEscape = 0x1B
	vkSpace  = 0x20
	vkLShift = 0xA0
	vkRShift = 0xA1
	vkOEMMin = 0xBD // '-' / '_'
	vkLeft   = 0x25
	vkUp     = 0x26
	vkRight  = 0x27
	vkDown   = 0x28
)

type kbdllHookStruct struct {
	vkCode      uint32
	scanCode    uint32
	flags       uint32
	time        uint32
	dwExtraInfo uintptr
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

// action is a high-level command from the hook to the processor goroutine.
type action struct {
	kind int
	r    rune
	hot  string
}

const (
	actArm       = iota // leader pressed, waiting for the next key
	actBeginEdit        // `m — open the marco command line
	actAppend
	actBackspace
	actSubmit
	actCancel    // Esc while typing — cancel the command
	actDismiss   // Esc otherwise — unfocus the panel
	actHotkey    // `<key> — run the route bound to it
	actHelp      // `h or `m help — show the help menu
	actCancelRun // Esc — stop the route that's running
	actCfgUp     // config editor: move selection up
	actCfgDown   // config editor: move selection down
	actCfgLeft   // config editor: change selected setting down
	actCfgRight
	actCfgSave     // config editor: persist to disk
	actCfgClose    // config editor: close
	actTeachType   // teach prompt: a typed y/n/s (or "" to clear) — shown, not yet sent
	actTeachSubmit // teach prompt: Enter — send the pending answer to the teach child
	actAcceptHint  // Tab — append the next auto-popped "name:" arg label to the command
)

// leaderTimeout disarms the leader if no key follows the backtick.
const leaderTimeout = 2 * time.Second

var (
	editing   atomic.Bool // marco command line open
	armed     atomic.Bool // backtick pressed, awaiting the command key
	inConfig  atomic.Bool // config editor open
	shiftDown atomic.Bool
	leaderGen atomic.Int64 // cancels a stale disarm timer
	actCh     = make(chan action, 256)
)

// startInput installs the keyboard hook and starts the processor. It returns
// immediately; the pump runs on its own locked OS thread.
func startInput(h *model, emit func(event)) error {
	go processActions(h, emit)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		cb := syscall.NewCallback(keyboardProc)
		hook, _, _ := procSetWindowsHookExW.Call(whKeyboardLL, cb, 0, 0)
		defer func() {
			if hook != 0 {
				procUnhookWindowsHookEx.Call(hook)
			}
		}()
		var m msg
		for {
			r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int(r) <= 0 { // WM_QUIT or error
				return
			}
		}
	}()
	return nil
}

// processActions applies hook actions to the HUD and emits feed events.
func processActions(h *model, emit func(event)) {
	for a := range actCh {
		switch a.kind {
		case actArm:
			h.closeHelp()
			h.setLeaderEcho("`")   // show the leader in the terminal
			h.setStatus("command") // wakes; listen colour
		case actBeginEdit:
			h.closeHelp()
			h.setLeaderEcho("")
			h.setEditing(true)
			h.setInput("")
			h.show(true)
			h.setStatus("command:")
		case actAppend:
			h.appendRune(a.r)
		case actBackspace:
			h.backspace()
		case actAcceptHint:
			// Tab autocompletes the route name first; once it's fully typed, Tab
			// advances to the next argument slot.
			if !h.completeRoute() {
				h.acceptArgHint()
			}
		case actSubmit:
			cmd := h.takeInput()
			h.setEditing(false)
			switch cmd {
			case "":
				h.dismiss()
			case "help", "?", "h":
				startUI(h, "help") // open the Help screen in the visual editor
			case "config", "cfg", "settings":
				inConfig.Store(true)
				h.openConfig(configLines())
			case "watch", "director", "insight":
				// Opens the panel rather than submitting a phrase. "director" on its
				// own is not something to DO to the screen, and sending it to the
				// Director would make it hunt for a control called "director".
				//
				// WATCH: what Marco sees, believes, is learning and needs. Click-through,
				// because a person is meant to keep using another application while it
				// is up.
				h.openWatch()
			case "diagnostics", "diagnose", "inspector", "inspect":
				// DIAGNOSTICS: the evidence underneath what Watch said, and the one mode
				// that captures the mouse. Entered by name, never implicitly, and Esc
				// releases it — see actCancel/actDismiss below.
				h.openDiagnostics()
			case "perception", "explain":
				// The frozen per-element perception snapshot. Kept as its own word: it
				// is expensive, it is a point-in-time sample rather than a live view,
				// and it answers a narrower question than the playbill does.
				refreshInsightDeep(h)
			default:
				h.setStatus("running: " + cmd)
				emit(event{Feed: "Commands", Event: "Run", Data: cmd})
			}
		case actCancel:
			// Esc always releases the mouse and closes whatever is up. Visibility must
			// never be able to trap a person: the way out of a panel is the same key it
			// has always been, and it is checked before anything else.
			h.closeWatch()
			h.closeInsight()
			h.dismiss()
		case actDismiss:
			h.closeHelp()
			h.closeWatch()
			h.closeInsight()
			h.dismiss()
		case actHelp:
			h.setLeaderEcho("`" + string(a.r)) // show "`h" in the terminal
			startUI(h, "help")                 // open the Help screen in the visual editor
		case actCfgUp:
			h.configMove(-1, configFieldCount)
		case actCfgDown:
			h.configMove(1, configFieldCount)
		case actCfgLeft:
			configChange(h.configSel_(), -1)
			h.setConfigLines(configLines())
		case actCfgRight:
			configChange(h.configSel_(), 1)
			h.setConfigLines(configLines())
		case actCfgSave:
			if err := saveConfig(); err != nil {
				h.log("save failed: " + err.Error())
			} else {
				h.log("config saved")
			}
		case actCfgClose:
			h.closeConfig()
		case actTeachType:
			// Show the answer as you type it (y/n/s), not yet submitted.
			h.setTeachPending(a.hot)
		case actTeachSubmit:
			// Enter: commit the pending answer to the transcript. If this is the
			// unknown-command teach OFFER (not a child prompt), y/Enter starts a
			// demonstration teach and anything else declines; otherwise send the
			// answer to the live teach child. Off the hook thread, so the write is safe.
			if ans, ok := h.submitTeachPending(); ok {
				teachAsk.Store(false)
				if name := takePendingTeach(); name != "" {
					// Unknown-command teach OFFER (no child runs on decline), so wipe the
					// prompt/transcript ourselves — otherwise it lingers in the HUD.
					h.clearTeachPrompt()
					if ans == "" || ans == "y" || ans == "yes" {
						startTeach(h, name)
					}
				} else {
					writeTeachAnswer(ans)
				}
			}
		case actCancelRun:
			cancelRun()
		case actHotkey:
			h.closeHelp()
			if a.r != 0 {
				h.setLeaderEcho("`" + string(a.r))
			}
			// `<key> → run whatever route is bound to that key in the current app
			// (overlay.marco: when Hotkeys reads Key? → Overlay Hotkey → marco hotkey).
			emit(event{Feed: "Hotkeys", Event: "Key", Data: string(a.r)})
		}
	}
}

// arm enters leader mode and schedules a disarm if no key follows.
func arm() {
	g := leaderGen.Add(1)
	armed.Store(true)
	push(action{kind: actArm})
	time.AfterFunc(leaderTimeout, func() {
		if leaderGen.Load() == g && armed.Load() {
			armed.Store(false)
			push(action{kind: actDismiss})
		}
	})
}

func disarm() { leaderGen.Add(1); armed.Store(false) }

// isTeachAnswer reports whether r is a single-key answer to a teach prompt: y/n/s
// (save / discard / simplify) or c/f/g (scope: context / focus / global).
func isTeachAnswer(r rune) bool {
	switch r {
	case 'y', 'n', 's', 'c', 'f', 'g':
		return true
	}
	return false
}

func push(a action) {
	select {
	case actCh <- a:
	default: // drop under flood rather than stall the hook thread
	}
}

func keyboardProc(nCode int, wParam, lParam uintptr) uintptr {
	if nCode >= 0 {
		kb := (*kbdllHookStruct)(unsafe.Pointer(lParam))
		if kb.flags&llkhfInjected == 0 { // ignore macro-injected input
			down := wParam == wmKeyDown || wParam == wmSysKeyDown
			up := wParam == wmKeyUp || wParam == wmSysKeyUp
			if (down || up) && handleKey(uint16(kb.vkCode), down) {
				return 1 // consume: do not pass to the focused app
			}
		}
	}
	ret, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wParam, lParam)
	return ret
}

// handleKey updates synchronous state and queues actions; it returns true when
// the key should be suppressed from the foreground app. The model mirrors the
// AHK overlay: backtick is the leader, then `m opens the marco command line and
// `<key> fires a quick game macro. Esc cancels a command or unfocuses.
func handleKey(vk uint16, down bool) bool {
	switch vk {
	case vkShift, vkLShift, vkRShift:
		shiftDown.Store(down)
		return editing.Load() // hide shift from the game only while typing
	}

	// A teach save/scope prompt is waiting: capture the y/n/s answer (Enter = yes)
	// and hand it to the processor goroutine, which echoes it in the HUD and writes
	// it to the teach child's stdin (off this hook thread). Swallow all keys so the
	// demonstration is over and nothing leaks to the app; disarm immediately so a
	// stray repeat isn't sent twice before the next prompt arrives.
	if teachAsk.Load() {
		if down {
			switch {
			case vk == vkReturn:
				push(action{kind: actTeachSubmit}) // Enter = the default answer
			default:
				// Single keypress answers and submits: y/n/s (save) and c/f/g (scope:
				// context / focus / global). No Enter needed — leader-then-n discards.
				if r, ok := vkToRune(vk, false); ok && isTeachAnswer(r) {
					push(action{kind: actTeachType, hot: string(r)})
					push(action{kind: actTeachSubmit})
				}
			}
		}
		return true
	}

	// A demonstration is recording: let every key reach the recorder child and the
	// app being demonstrated. The leader is the recorder's stop key, so pressing it
	// ends the demo here instead of opening the overlay command line.
	if recording.Load() {
		return false
	}

	// Config editor open: arrows pick/change, S saves, Esc closes. Swallow all.
	if inConfig.Load() {
		if down {
			switch vk {
			case vkUp:
				push(action{kind: actCfgUp})
			case vkDown:
				push(action{kind: actCfgDown})
			case vkLeft:
				push(action{kind: actCfgLeft})
			case vkRight:
				push(action{kind: actCfgRight})
			case 'S':
				push(action{kind: actCfgSave})
			case vkEscape, leaderVK:
				inConfig.Store(false)
				push(action{kind: actCfgClose})
			}
		}
		return true
	}

	// While the command line is open, swallow everything and edit.
	if editing.Load() {
		if down {
			switch vk {
			case leaderVK, vkEscape:
				editing.Store(false)
				push(action{kind: actCancel})
			case vkReturn:
				editing.Store(false)
				push(action{kind: actSubmit})
			case vkTab:
				push(action{kind: actAcceptHint}) // accept the next auto-popped "name:" label
			case vkBack:
				push(action{kind: actBackspace})
			default:
				if r, ok := vkToRune(vk, shiftDown.Load()); ok {
					push(action{kind: actAppend, r: r})
				}
			}
		}
		return true
	}

	// Leader armed: the next key chooses what to do (echoed as `<key>).
	if armed.Load() {
		if down {
			disarm()
			ch, _ := vkToRune(vk, false) // lowercase, for the echo
			switch {
			case vk == 'M':
				editing.Store(true)
				push(action{kind: actBeginEdit})
			case vk == 'H':
				push(action{kind: actHelp, r: ch})
			case vk == vkEscape || vk == leaderVK:
				push(action{kind: actDismiss})
			default:
				if ch != 0 { // `<key> — run the route bound to it in the current app
					push(action{kind: actHotkey, r: ch})
				} else {
					push(action{kind: actDismiss})
				}
			}
		}
		return true // consume the key after the leader
	}

	// Idle. The leader is also the panic/stop key: while a route is running, it
	// cancels it (canceled, not failed); otherwise it arms the command line. Either
	// way it's consumed.
	if vk == leaderVK {
		if down {
			if isRunning() {
				push(action{kind: actCancelRun})
			} else {
				arm()
			}
		}
		return true
	}
	// Esc is unbound — the only overlay hotkeys are the leader (start/stop/cancel)
	// and the anchor key (F12). Esc passes straight through to the app/game.
	return false
}

// vkToRune maps a virtual-key to a command-line rune. Route names are lowercase
// slugs (letters, digits, spaces, hyphens), so the mapping stays minimal.
func vkToRune(vk uint16, shift bool) (rune, bool) {
	switch {
	case vk >= 'A' && vk <= 'Z':
		if shift {
			return rune(vk), true
		}
		return rune(vk - 'A' + 'a'), true
	case vk >= '0' && vk <= '9':
		return rune(vk), true
	case vk == vkSpace:
		return ' ', true
	case vk == vkOEMMin:
		return '-', true
	}
	return 0, false
}
