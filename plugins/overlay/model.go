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

	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
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
	mu            sync.Mutex
	visible       bool
	editing       bool
	status        string // raw status text from the engine
	state         string // idle | listen | run | ok | error
	input         string
	logs          []string
	lastRun       string
	app           string    // foreground app (polled via `marco active`)
	lastActive    time.Time // drives the wake/idle fade + auto-revert
	helpOn        bool
	help          []string
	leaderEcho    string      // the leader sequence being typed, e.g. "`h"
	heard         string      // live voice transcript (Google-style preview)
	history       []histEntry // recent commands with their outcome + time
	cpu, ram      float64     // system metrics (%), for the optional widget
	curX, curY    int         // live cursor position (virtual-desktop coords)
	curHasPos     bool        // whether curX/curY have been sampled yet
	learnSession  bool        // a learn recording is in progress (keeps the HUD awake)
	learnStart    time.Time   // when the current learn recording began (for the live timer)
	configOn      bool        // the config editor is open
	configSel     int         // selected config row
	config        []string    // rendered config rows
	learnLog      []string    // answered prompts + their selections (terminal transcript)
	prompt        string      // the active y/n/s prompt ("" = none), shown verbatim with its options
	promptPending string      // the answer typed but not yet submitted (type y/n/s, then Enter)
	insightOn     bool        // the frozen perception snapshot panel is open (`explain`)
	inspectorOn   bool        // mouse passthrough OFF, full detail
	insightFrozen bool        // showing a deep snapshot; the live poll is paused
	insight       []string    // rendered insight rows (see insight.go)
	// wmode is which reading of the Director's playbill is on screen, and watch is the
	// account itself. ONE value drives all three readings — see watch.go.
	wmode watchLevel
	watch playbill.View
	// headline is the NORMAL reading: one word and one sentence, kept current in the
	// background so a pending question is never invisible behind a closed panel.
	headline   playbill.Headline
	argHints   []string // arg labels to auto-pop for the route being typed ("name:" …)
	routeNames []string // known route display names, for Tab autocomplete
	quit       bool
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

// setRouteNames stores the known route display names for Tab autocomplete.
func (h *model) setRouteNames(names []string) { h.mu.Lock(); h.routeNames = names; h.mu.Unlock() }

// completeRoute extends the typed command to the longest common prefix of the route
// names it prefixes — Tab autocomplete for the route phrase. It only acts while
// you're still typing the phrase (before a " with " args clause) and only when there
// is more to add, so once the route is fully typed Tab falls through to the arg-slot
// advance. Returns true if it changed the input.
func (h *model) completeRoute() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if strings.Contains(strings.ToLower(h.input), " with ") {
		return false // in the args clause, not the route name
	}
	typed := strings.TrimSpace(h.input)
	if typed == "" {
		return false
	}
	low := strings.ToLower(typed)
	var matches []string
	for _, r := range h.routeNames {
		if strings.HasPrefix(strings.ToLower(r), low) {
			matches = append(matches, r)
		}
	}
	if len(matches) == 0 {
		return false
	}
	lcp := longestCommonPrefix(matches)
	if len(lcp) <= len(typed) {
		return false // already at the common prefix (or fully typed)
	}
	lead := h.input[:len(h.input)-len(strings.TrimLeft(h.input, " "))] // keep typed spaces
	h.input = lead + lcp
	h.lastActive = time.Now()
	return true
}

// longestCommonPrefix returns the longest string that prefixes every input.
func longestCommonPrefix(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	p := ss[0]
	for _, s := range ss[1:] {
		for !strings.HasPrefix(s, p) {
			p = p[:len(p)-1]
			if p == "" {
				return ""
			}
		}
	}
	return p
}

// acceptArgHint advances to the next argument slot (Tab): inserts " with " before
// the first arg, ", " before each subsequent one. The arg labels stay visible as a
// colored guide but are never typed — values go in positionally after "with". The
// label list (h.argHints) is the route's full arg set, refilled by the poller, so we
// stop once every slot is filled rather than consuming the list.
func (h *model) acceptArgHint() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.argHints) == 0 {
		return false
	}
	lower := strings.ToLower(h.input)
	i := strings.Index(lower, " with ")
	if i < 0 {
		if h.input != "" && !strings.HasSuffix(h.input, " ") {
			h.input += " "
		}
		h.input += "with "
		h.lastActive = time.Now()
		return true
	}
	// Already in the clause: advance only if there's an unfilled slot AND the current
	// one has a value (no point adding a comma after an empty slot).
	segs := strings.Split(h.input[i+len(" with "):], ",")
	filled := 0
	for _, s := range segs {
		if strings.TrimSpace(s) != "" {
			filled++
		}
	}
	if filled >= len(h.argHints) || strings.TrimSpace(segs[len(segs)-1]) == "" {
		return false
	}
	h.input = strings.TrimRight(h.input, " ") + ", "
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
	// A question is Marco waiting on the person, which is the same state the panel
	// already has a colour for. It is emphatically not an error: nothing went wrong.
	//
	// The second prefix is a CONSTANT, not a copy of the sentence. It used to be the
	// literal "director asked", written here and produced in director.go, and the two
	// agreed only by both containing a backstage word nobody was supposed to see. Renaming
	// the line in the product's own register would have left this arm matching nothing and
	// coloured a pending question as if nothing were happening.
	case strings.HasPrefix(s, "asked about"), strings.HasPrefix(s, questionStatus):
		h.state = "listen"
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

// ---- the Director's playbill: Normal, Watch, Diagnostics ----

// watchLevel is which reading of the ONE account is on screen.
//
// Levels of a single surface rather than three panels. They render the same value, they
// share a poller, and there is deliberately no state that one has and another does not —
// which is what makes "the same underlying state drives all of them" a property rather
// than an intention.
type watchLevel int

const (
	// watchOff: no panel. The NORMAL reading still runs in the background.
	watchOff watchLevel = iota
	// watchOn: WATCH — what Marco sees, believes, is learning and needs.
	watchOn
	// watchDeep: DIAGNOSTICS — the evidence underneath what Watch said.
	watchDeep
)

// openWatch shows the human-readable reading.
//
// Opening always leaves the mouse alone: Watch is click-through, because a person is
// meant to keep using another application while it is up. Only Diagnostics captures.
func (h *model) openWatch() {
	h.mu.Lock()
	h.wmode, h.helpOn, h.editing = watchOn, false, false
	h.insightOn, h.insightFrozen, h.inspectorOn = false, false, false
	h.lastActive = time.Now()
	h.mu.Unlock()
}

// openDiagnostics shows the evidence underneath, and captures the mouse.
//
// Capturing is entered BY NAME and announced on screen, exactly as the old inspector was.
// An overlay that quietly swallowed a click meant for the game would be worse than
// useless, so the mode that does it has to be asked for and has to say so.
func (h *model) openDiagnostics() {
	h.mu.Lock()
	h.wmode, h.inspectorOn = watchDeep, true
	h.helpOn, h.editing = false, false
	h.insightOn, h.insightFrozen = false, false
	h.lastActive = time.Now()
	h.mu.Unlock()
}

func (h *model) closeWatch() {
	h.mu.Lock()
	h.wmode, h.inspectorOn = watchOff, false
	h.lastActive = time.Now().Add(-2 * idleAfter) // fade out now
	h.mu.Unlock()
}

// watchMode reports which reading is on screen, for the poller.
func (h *model) watchMode() watchLevel {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.wmode
}

// setWatch replaces the account.
//
// It does NOT wake the panel: a refresh is the poller talking, not the user, and waking
// on it would keep the HUD lit for as long as anybody left it open.
func (h *model) setWatch(v playbill.View) {
	h.mu.Lock()
	h.watch = v
	// The headline comes from the same value, always. A separate fetch for it would be a
	// second opinion about whether Marco has a question.
	h.headline = v.Normal()
	h.mu.Unlock()
}

// setHeadline stores the NORMAL reading from the background poll.
//
// It wakes the panel only for something that is WAITING ON A PERSON. That is the whole
// of the consumer surface's claim on attention, and everything else Marco does — watching,
// learning, thinking — deliberately does not earn one.
func (h *model) setHeadline(hd playbill.Headline) {
	h.mu.Lock()
	if hd.Attention && !h.headline.Attention {
		h.lastActive = time.Now()
	}
	h.headline = hd
	h.mu.Unlock()
}

// ---- the frozen perception snapshot (`perception` / `explain`) ----
//
// The one surface the playbill does NOT replace. It is the per-element account, it is
// quadratic in the observation count, and it is the only thing that produces the
// anonymous-label share — so it stays a point-in-time snapshot somebody asks for by name
// rather than something a live panel pays for forever. See [[Visibility]]'s known gaps.

func (h *model) closeInsight() {
	h.mu.Lock()
	h.insightOn, h.inspectorOn, h.insightFrozen = false, false, false
	h.lastActive = time.Now().Add(-2 * idleAfter) // fade out now
	h.mu.Unlock()
}

// inspectorOpen reports whether the mouse-capturing mode is active.
func (h *model) inspectorOpen() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.inspectorOn
}

// insightPolls reports whether the live poller should refresh.
//
// False while frozen. The deep view is an expensive point-in-time sample, and letting the
// cheap poll overwrite it two seconds later would mean the thing the user asked for
// flickered past and vanished.
func (h *model) insightPolls() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.insightOn && !h.insightFrozen
}

// setInsight replaces the rendered rows. It does NOT wake the panel: a refresh is the
// poller talking, not the user, and waking on it would keep the HUD lit indefinitely.
func (h *model) setInsight(lines []string) {
	h.mu.Lock()
	h.insight = lines
	h.mu.Unlock()
}

// freezeInsight opens the panel and stops the live poll, for the deep snapshot.
//
// It closes the playbill panel: two surfaces sharing one slot would render on top of each
// other, and the one underneath would be the one nobody could see was still polling.
func (h *model) freezeInsight() {
	h.mu.Lock()
	h.wmode, h.inspectorOn = watchOff, false
	h.insightOn, h.insightFrozen, h.helpOn, h.editing = true, true, false, false
	h.insight = []string{"director  reading (deep)…"}
	h.lastActive = time.Now()
	h.mu.Unlock()
}

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
	cmd string
	// outcome is one of the engine's six words (see outcome.go), stored as a string
	// because this row is only rendered. It used to be one of three, two of which said
	// "ok" about something that had not happened.
	outcome string
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

// setCursor updates the live cursor position for the coords tooltip. Like metrics,
// it does NOT wake the panel — a background poll shouldn't keep the HUD lit.
func (h *model) setCursor(x, y int, ok bool) {
	h.mu.Lock()
	h.curX, h.curY, h.curHasPos = x, y, ok
	h.mu.Unlock()
}

// setLearnSession marks a learn recording active, keeping the HUD awake (like editing)
// for the whole demonstration so the "recording — F12 to save" guidance stays up.
func (h *model) setLearnSession(v bool) {
	h.mu.Lock()
	h.learnSession = v
	if v {
		h.learnStart = time.Now()
		h.learnLog, h.prompt, h.promptPending = nil, "", "" // fresh session
	}
	h.lastActive = time.Now()
	h.mu.Unlock()
}

// ---- interactive prompts (the save / scope / simplify questions) ----

// setPrompt shows a y/n/s prompt verbatim (with its [y]es/[n]o/[s] options).
// It clears any pending keystrokes so the new question starts empty.
func (h *model) setPrompt(s string) {
	h.mu.Lock()
	h.prompt, h.promptPending = s, ""
	h.lastActive = time.Now()
	h.mu.Unlock()
}

// setPromptPending shows the answer typed so far (you type y/n/s, see it, then press
// Enter to submit). "" clears it (backspace). Ignored if no prompt is up.
func (h *model) setPromptPending(s string) {
	h.mu.Lock()
	if h.prompt != "" {
		h.promptPending = s
		h.lastActive = time.Now()
	}
	h.mu.Unlock()
}

// submitPromptPending commits the pending answer: it appends the question and the
// chosen answer to the transcript and clears the active prompt. Returns the answer
// to send to the child ("" means the default — bare Enter). No-op (returns
// "" with ok=false) if no prompt is up.
func (h *model) submitPromptPending() (answer string, ok bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.prompt == "" {
		return "", false
	}
	answer = h.promptPending
	shown := answer
	if shown == "" {
		shown = "↵" // bare Enter = the default
	}
	h.learnLog = append(h.learnLog, h.prompt, "› "+shown)
	h.prompt, h.promptPending = "", ""
	h.lastActive = time.Now()
	return answer, true
}

func (h *model) clearPrompt() {
	h.mu.Lock()
	h.learnLog, h.prompt, h.promptPending = nil, "", ""
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
	inspectorOn                        bool
	insightOn                          bool
	insight                            []string
	status, state, input, lastRun, app string
	leaderEcho, heard                  string
	cpu, ram                           float64
	curX, curY                         int
	curHasPos                          bool
	learnSession                       bool
	learnStart                         time.Time
	configSel                          int
	prompt, promptPending              string
	learnLog                           []string
	argHints                           []string
	logs, help, config                 []string
	history                            []histEntry
	// wmode, watch and headline are the three readings of one account.
	wmode    watchLevel
	watch    playbill.View
	headline playbill.Headline
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
	tlog := make([]string, len(h.learnLog))
	copy(tlog, h.learnLog)
	hints := make([]string, len(h.argHints))
	copy(hints, h.argHints)
	ins := make([]string, len(h.insight))
	copy(ins, h.insight)
	return snapshot{
		visible: h.visible, editing: h.editing, helpOn: h.helpOn, configOn: h.configOn,
		status: h.status, state: h.state, input: h.input, lastRun: h.lastRun, app: h.app,
		leaderEcho: h.leaderEcho, heard: h.heard, cpu: h.cpu, ram: h.ram,
		curX: h.curX, curY: h.curY, curHasPos: h.curHasPos, learnSession: h.learnSession,
		learnStart: h.learnStart, configSel: h.configSel,
		prompt: h.prompt, promptPending: h.promptPending, learnLog: tlog,
		argHints:  hints,
		insightOn: h.insightOn, inspectorOn: h.inspectorOn, insight: ins,
		wmode: h.wmode, watch: h.watch, headline: h.headline,
		logs: logs, help: help, config: conf, history: hist,
	}
}
