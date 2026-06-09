package main

// model.go — the MODEL in the overlay's Model/View/Controller split.
//
// The model is the single source of truth for what the HUD shows. It is driven
// by Marco (the brain): the engine pushes semantic state through the Overlay act
// (acts.go → setStatus/log/showHelp/…), and the controller (controller_*.go)
// writes the live input it must own synchronously (the command buffer, leader
// echo, capture mode). The view (view.go) only reads a snapshot and renders it.

import (
	"strings"
	"sync"
	"time"
)

// idleAfter is how long after the last activity the HUD reverts to idle (the
// green "done" fades, the panel goes transparent).
const idleAfter = 5 * time.Second

// Wake vs idle opacity, mirroring the AHK overlay (statusOpacity 250 / idleOpacity
// 120 ≈ 0.47). Idle keeps a faint-but-clearly-dark panel — too low and the
// background bleeds through (coral over a coral game). $MARCO_OVERLAY_IDLE
// overrides the idle fill.
const (
	bgWake   = 0.97
	textWake = 1.0
	textIdle = 0.85
)

// model is the overlay's shared display state. Marco (via acts.go), the
// controller, and the clock/app pollers mutate it; the view's Draw reads it. All
// access goes through the mutex.
type model struct {
	mu           sync.Mutex
	visible      bool
	editing      bool
	status       string // raw status text from the engine
	state        string // idle | listen | run | ok | error
	input        string
	logs         []string
	lastRun      string
	app          string    // foreground app (polled via `marco active`)
	lastActive   time.Time // drives the wake/idle fade + auto-revert
	helpOn       bool
	help         []string
	leaderEcho   string      // the leader sequence being typed, e.g. "`h"
	heard        string      // live voice transcript (Google-style preview)
	history      []histEntry // recent commands with their outcome + time
	cpu, ram     float64     // system metrics (%), for the optional widget
	teaching     bool        // a teach recording is in progress (keeps the HUD awake)
	teachStart   time.Time   // when the current teach recording began (for the live timer)
	configOn     bool        // the config editor is open
	configSel    int         // selected config row
	config       []string    // rendered config rows
	teachLog     []string    // answered teach prompts + their selections (terminal transcript)
	teachPrompt  string      // the active teach y/n/s prompt ("" = none), shown verbatim with its options
	teachPending string      // the answer typed but not yet submitted (type y/n/s, then Enter)
	argHints     []string    // arg labels to auto-pop for the route being typed ("name:" …)
	quit         bool
}

// inputForHint returns the current command text and whether the line is open, so the
// arg-hint poller knows what (and when) to query.
func (h *model) inputForHint() (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.input, h.editing
}

// setArgHints stores the arg labels the engine suggested for the typed route.
func (h *model) setArgHints(a []string) { h.mu.Lock(); h.argHints = a; h.mu.Unlock() }

// acceptArgHint appends the first suggested "name:" label to the command line (Tab),
// and returns whether there was one to accept.
func (h *model) acceptArgHint() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.argHints) == 0 {
		return false
	}
	if h.input != "" && !strings.HasSuffix(h.input, " ") {
		h.input += " "
	}
	h.input += h.argHints[0] + ":"
	h.argHints = h.argHints[1:]
	h.lastActive = time.Now()
	return true
}

func newModel() *model {
	return &model{visible: true, status: "ready", state: "idle", app: "desktop", lastActive: time.Now()}
}

func (h *model) setStatus(s string) {
	h.mu.Lock()
	h.status = s
	switch {
	case strings.HasPrefix(s, "running"):
		h.state = "run"
	case strings.HasPrefix(s, "ran"):
		h.state = "ok"
	case strings.HasPrefix(s, "failed"):
		h.state = "error"
	case strings.HasPrefix(s, "command"):
		h.state = "listen"
	default:
		h.state = "idle"
	}
	if h.state != "idle" {
		h.lastActive = time.Now() // engine activity wakes the panel
	}
	h.mu.Unlock()
}

func (h *model) sinceActive() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	return time.Since(h.lastActive)
}

// idle reverts to the resting state without counting as activity (so the green
// "done" fades to "ready" instead of re-waking the panel).
func (h *model) idle() { h.mu.Lock(); h.state, h.status = "idle", "ready"; h.mu.Unlock() }

// dismiss cancels any command/help/echo and fades the panel out now (Esc).
func (h *model) dismiss() {
	h.mu.Lock()
	h.editing, h.helpOn, h.input, h.leaderEcho = false, false, "", ""
	h.state, h.status = "idle", "ready"
	h.lastActive = time.Now().Add(-2 * idleAfter) // fade immediately, no 5s wait
	h.mu.Unlock()
}

func (h *model) showHelp(lines []string) {
	h.mu.Lock()
	h.helpOn, h.help, h.editing = true, lines, false
	h.lastActive = time.Now() // wake
	h.mu.Unlock()
}

func (h *model) closeHelp() { h.mu.Lock(); h.helpOn = false; h.mu.Unlock() }

// ---- config editor ----

func (h *model) openConfig(lines []string) {
	h.mu.Lock()
	h.configOn, h.config, h.configSel, h.helpOn, h.editing = true, lines, 0, false, false
	h.lastActive = time.Now()
	h.mu.Unlock()
}

func (h *model) closeConfig() {
	h.mu.Lock()
	h.configOn = false
	h.lastActive = time.Now().Add(-2 * idleAfter) // fade out now
	h.mu.Unlock()
}

func (h *model) configSel_() int { h.mu.Lock(); defer h.mu.Unlock(); return h.configSel }

func (h *model) configMove(delta, n int) {
	h.mu.Lock()
	h.configSel = (h.configSel + delta + n) % n
	h.lastActive = time.Now()
	h.mu.Unlock()
}

func (h *model) setConfigLines(lines []string) {
	h.mu.Lock()
	h.config = lines
	h.lastActive = time.Now()
	h.mu.Unlock()
}

func (h *model) setLeaderEcho(s string) {
	h.mu.Lock()
	h.leaderEcho = s
	h.lastActive = time.Now()
	h.mu.Unlock()
}

// histEntry is one finished command in the history: what ran, how it ended, and
// how long it took.
type histEntry struct {
	cmd     string
	outcome string // ok | canceled | failed
	dur     time.Duration
}

// addHistory records a finished run in the command history (capped to the
// configured max lines), newest last.
func (h *model) addHistory(cmd, outcome string, d time.Duration) {
	max := cfgMaxLines()
	h.mu.Lock()
	h.lastRun = cmd
	h.history = append(h.history, histEntry{cmd: cmd, outcome: outcome, dur: d})
	if len(h.history) > max {
		h.history = h.history[len(h.history)-max:]
	}
	h.lastActive = time.Now()
	h.mu.Unlock()
}

// setMetrics updates the system metrics (does NOT wake the panel — a background
// poll shouldn't keep the HUD lit).
func (h *model) setMetrics(cpu, ram float64) {
	h.mu.Lock()
	h.cpu, h.ram = cpu, ram
	h.mu.Unlock()
}

// setTeaching marks a teach recording active, keeping the HUD awake (like editing)
// for the whole demonstration so the "recording — F12 to save" guidance stays up.
func (h *model) setTeaching(v bool) {
	h.mu.Lock()
	h.teaching = v
	if v {
		h.teachStart = time.Now()
		h.teachLog, h.teachPrompt, h.teachPending = nil, "", "" // fresh session
	}
	h.lastActive = time.Now()
	h.mu.Unlock()
}

// ---- teach prompts (the interactive save / scope / simplify questions) ----

// setTeachPrompt shows a y/n/s prompt verbatim (with its [y]es/[n]o/[s] options).
// It clears any pending keystrokes so the new question starts empty.
func (h *model) setTeachPrompt(s string) {
	h.mu.Lock()
	h.teachPrompt, h.teachPending = s, ""
	h.lastActive = time.Now()
	h.mu.Unlock()
}

// setTeachPending shows the answer typed so far (you type y/n/s, see it, then press
// Enter to submit). "" clears it (backspace). Ignored if no prompt is up.
func (h *model) setTeachPending(s string) {
	h.mu.Lock()
	if h.teachPrompt != "" {
		h.teachPending = s
		h.lastActive = time.Now()
	}
	h.mu.Unlock()
}

// submitTeachPending commits the pending answer: it appends the question and the
// chosen answer to the transcript and clears the active prompt. Returns the answer
// to send to the teach child ("" means the default — bare Enter). No-op (returns
// "" with ok=false) if no prompt is up.
func (h *model) submitTeachPending() (answer string, ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.teachPrompt == "" {
		return "", false
	}
	answer = h.teachPending
	shown := answer
	if shown == "" {
		shown = "↵" // bare Enter = the default
	}
	h.teachLog = append(h.teachLog, h.teachPrompt, "› "+shown)
	h.teachPrompt, h.teachPending = "", ""
	h.lastActive = time.Now()
	return answer, true
}

func (h *model) clearTeachPrompt() {
	h.mu.Lock()
	h.teachLog, h.teachPrompt, h.teachPending = nil, "", ""
	h.mu.Unlock()
}

// setHeard shows the live voice transcript on the command line (wakes the panel).
func (h *model) setHeard(s string) {
	h.mu.Lock()
	h.heard = s
	h.lastActive = time.Now()
	h.mu.Unlock()
}

func (h *model) setInput(s string) { h.mu.Lock(); h.input = s; h.mu.Unlock() }
func (h *model) show(v bool)       { h.mu.Lock(); h.visible = v; h.mu.Unlock() }
func (h *model) setEditing(v bool) {
	h.mu.Lock()
	h.editing = v
	if v {
		h.lastActive = time.Now()
	}
	h.mu.Unlock()
}
func (h *model) setRoutes([]string) {}
func (h *model) clear()             { h.mu.Lock(); h.logs = nil; h.mu.Unlock() }
func (h *model) requestQuit()       { h.mu.Lock(); h.quit = true; h.mu.Unlock() }
func (h *model) setApp(a string)    { h.mu.Lock(); h.app = a; h.mu.Unlock() }
func (h *model) appendRune(r rune) {
	h.mu.Lock()
	h.input += string(r)
	h.lastActive = time.Now()
	h.mu.Unlock()
}

func (h *model) backspace() {
	h.mu.Lock()
	if n := len(h.input); n > 0 {
		h.input = h.input[:n-1]
	}
	h.lastActive = time.Now()
	h.mu.Unlock()
}

func (h *model) takeInput() string {
	h.mu.Lock()
	s := strings.TrimSpace(h.input) // the typed `m + space separator isn't part of the command
	h.input, h.lastRun = "", s
	h.mu.Unlock()
	return s
}

func (h *model) log(line string) {
	max := cfgMaxLines()
	h.mu.Lock()
	h.logs = append(h.logs, line)
	if len(h.logs) > max {
		h.logs = h.logs[len(h.logs)-max:]
	}
	h.lastActive = time.Now()
	h.mu.Unlock()
}

// clearText drops the last-command echo and log tail (called after a minute idle).
// It does not count as activity, so the panel stays faded.
func (h *model) clearText() {
	h.mu.Lock()
	h.lastRun, h.logs, h.heard, h.history = "", nil, "", nil
	h.mu.Unlock()
}

func (h *model) shouldQuit() bool { h.mu.Lock(); defer h.mu.Unlock(); return h.quit }

// snapshot is an immutable copy the view renders without holding the lock.
type snapshot struct {
	visible, editing, helpOn, configOn bool
	status, state, input, lastRun, app string
	leaderEcho, heard                  string
	cpu, ram                           float64
	teaching                           bool
	teachStart                         time.Time
	configSel                          int
	teachPrompt, teachPending          string
	teachLog                           []string
	argHints                           []string
	logs, help, config                 []string
	history                            []histEntry
}

func (h *model) snapshot() snapshot {
	h.mu.Lock()
	defer h.mu.Unlock()
	logs := make([]string, len(h.logs))
	copy(logs, h.logs)
	help := make([]string, len(h.help))
	copy(help, h.help)
	conf := make([]string, len(h.config))
	copy(conf, h.config)
	hist := make([]histEntry, len(h.history))
	copy(hist, h.history)
	tlog := make([]string, len(h.teachLog))
	copy(tlog, h.teachLog)
	hints := make([]string, len(h.argHints))
	copy(hints, h.argHints)
	return snapshot{
		visible: h.visible, editing: h.editing, helpOn: h.helpOn, configOn: h.configOn,
		status: h.status, state: h.state, input: h.input, lastRun: h.lastRun, app: h.app,
		leaderEcho: h.leaderEcho, heard: h.heard, cpu: h.cpu, ram: h.ram, teaching: h.teaching,
		teachStart: h.teachStart, configSel: h.configSel,
		teachPrompt: h.teachPrompt, teachPending: h.teachPending, teachLog: tlog,
		argHints: hints,
		logs:     logs, help: help, config: conf, history: hist,
	}
}
