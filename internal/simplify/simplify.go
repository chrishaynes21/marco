// Package simplify turns a raw recorded input stream into a clean macro step
// list: it lowers events to steps, inserts rounded waits, coalesces repeated
// keys, folds exact repeated cycles into loops, and drops noise. It is pure Go
// and OS-agnostic, so it is fully unit-testable without any hooks.
package simplify

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// Options tune the simplification. Zero value is filled with sane defaults by
// normalize().
type Options struct {
	WaitGranularityMs int  // round waits to this multiple (default 10)
	MinWaitMs         int  // drop gaps below this (default 30)
	DragThresholdPx   int  // click-down→up move beyond this is a drag (default 6)
	FoldLoops         bool // fold exact repeated cycles into loops (default true)
	MinLoopReps       int  // minimum repetitions to fold (default 2)
	// TypingRhythmMs, when > 0, merges a varied run of printable keys into one
	// Type step even when separated by waits up to this many ms (the natural
	// rhythm of human typing), dropping those interior waits. 0 (the default)
	// keeps recording faithful — only no-gap runs coalesce — so game-timing
	// macros are preserved. Used by Tighten / the "simplify further" teach option.
	TypingRhythmMs int
	// ArgKey is a reserved key (e.g. "f9") you tap during a demonstration to drop an
	// argument placeholder — the Nth tap becomes a Type "{{name}}" step (named from
	// ArgNames, else positional "{{N}}") that codegen keeps and the run fills from
	// `<route> name:value`. So you mark "an argument goes here" without typing into
	// the app. "" / "off" disables it.
	ArgKey string
	// ArgNames are the declared argument names (from the teach phrase's "with name,
	// …" clause), used to name the placeholders the ArgKey drops, in order.
	ArgNames []string
	// MaxWaitMs caps every wait step to this many milliseconds after all other
	// passes, preventing recorded human-paced delays from slowing replay. The
	// zero value is filled to 50 by normalize. Set to a negative value to preserve
	// exact recorded timing (e.g. game macros that require specific inter-step gaps).
	MaxWaitMs int
}

// DefaultArgKey is the reserved demonstration key for "an argument goes here" when
// $MARCO_ARG_KEY isn't set. F9 is on every keyboard and rarely part of a macro.
const DefaultArgKey = "f9"

// DefaultTypingRhythmMs is the inter-key gap up to which spacing is treated as
// typing rhythm (and folded away). Above this, a wait is a deliberate pause and
// is kept.
const DefaultTypingRhythmMs = 300

// AggressiveTypingRhythmMs folds a typed run into one Type step even across
// pauses up to a full second between keys — used by the explicit "simplify
// further" / `marco simplify` paths, where the user has asked for the cleanest
// possible Type steps and ordinary inter-key timing (even a pause between words)
// no longer matters. A gap longer than this still reads as a deliberate pause
// and is kept. The faithful default (0) preserves every gap, so game-timing
// macros are untouched.
const AggressiveTypingRhythmMs = 1000

func (o Options) normalize() Options {
	if o.WaitGranularityMs <= 0 {
		o.WaitGranularityMs = 10
	}
	if o.MinWaitMs <= 0 {
		o.MinWaitMs = 30
	}
	if o.DragThresholdPx <= 0 {
		o.DragThresholdPx = 6
	}
	if o.MinLoopReps <= 0 {
		o.MinLoopReps = 2
	}
	if o.MaxWaitMs == 0 {
		o.MaxWaitMs = 50
	}
	// FoldLoops defaults to true; callers that want it off must set it explicitly
	// via DefaultOptions then flip it.
	return o
}

// DefaultOptions returns the recommended defaults (loop folding on, the F9 arg key).
func DefaultOptions() Options {
	o := Options{FoldLoops: true, ArgKey: DefaultArgKey}.normalize()
	return o
}

// Simplify lowers a recorded event stream into a clean step list.
func Simplify(events []recorder.RecordedEvent, opt Options) []macroir.Step {
	opt = opt.normalize()
	timed := lower(events, opt)
	timed = foldActivates(timed)
	steps := insertWaits(timed, opt)
	return SimplifySteps(steps, opt)
}

// SimplifySteps re-simplifies an existing step list (coalesce typing, coalesce
// keys, merge waits, trim ends, fold cycles). Safe to run on hand-written or
// previously recorded steps.
func SimplifySteps(in []macroir.Step, opt Options) []macroir.Step {
	opt = opt.normalize()
	out := coalesceTypingRhythm(in, opt.TypingRhythmMs)
	out = coalesceTyping(out)
	out = coalesceKeys(out)
	out = mergeWaits(out)
	out = trimEdgeWaits(out)
	out = capWaits(out, opt.MaxWaitMs)
	if opt.FoldLoops {
		out = foldLoops(out, opt)
	}
	return out
}

// timedStep is a step plus the wall-clock time of the event that produced it,
// used to compute inter-step waits.
type timedStep struct {
	step macroir.Step
	t    time.Time
}

// lower converts raw events into anchor steps (clicks/drags/keys/type), with no
// waits yet. Hover moves are discarded; printable keystrokes are accumulated
// into type runs; held-button drags are detected.
func lower(events []recorder.RecordedEvent, opt Options) []timedStep {
	var out []timedStep
	shift, ctrl, alt, win := false, false, false, false

	// mouse-down tracking for drag detection
	downActive := false
	var downX, downY int
	var downBtn string
	var downT time.Time
	var downImg []byte
	var downColor string
	var downWindow string
	var downClickX, downClickY int
	var downRelX, downRelY int
	var downWinRel bool
	sawMove := false
	argN := 0
	argKey := strings.ToLower(strings.TrimSpace(opt.ArgKey))
	// Which key-presses are HOLDS (≥ holdThreshold or spanning an action) → emitted as
	// explicit KeyDown/KeyUp instead of a tap, so a held key persists across the clicks
	// in between (hold Q, click, release Q). Decided with full lookahead.
	holdDown, holdUp := markHolds(events, argKey)

	for i := range events {
		ev := events[i]
		switch ev.Kind {
		case recorder.EvAppSwitch:
			if ev.KeyName != "" {
				out = append(out, timedStep{
					step: macroir.Step{Kind: macroir.StepActivate, Text: ev.KeyName},
					t:    ev.T,
				})
			}
		case recorder.EvMove:
			if downActive {
				if abs(ev.X-downX) > opt.DragThresholdPx || abs(ev.Y-downY) > opt.DragThresholdPx {
					sawMove = true
				}
			}
		case recorder.EvClick:
			if ev.Down {
				downActive = true
				downX, downY, downBtn, downT = ev.X, ev.Y, ev.Button, ev.T
				downImg = ev.Image
				downColor = ev.Color
				downWindow = ev.Window
				downClickX, downClickY = ev.ClickX, ev.ClickY
				downRelX, downRelY, downWinRel = ev.RelX, ev.RelY, ev.WinRel
				sawMove = false
			} else {
				if !downActive {
					continue
				}
				downActive = false
				dist := abs(ev.X-downX) > opt.DragThresholdPx || abs(ev.Y-downY) > opt.DragThresholdPx
				if sawMove && dist {
					out = append(out, timedStep{
						step: macroir.Step{Kind: macroir.StepDrag, X: downX, Y: downY, X2: ev.X, Y2: ev.Y, Button: downBtn},
						t:    downT,
					})
				} else {
					// The click carries its window-relative offset (default — follows the
					// active window across monitors/position) and, when anchors are on, a
					// captured patch (codegen turns it into an image find). Drags carry
					// neither.
					out = append(out, timedStep{
						step: macroir.Step{
							Kind: macroir.StepClick, X: ev.X, Y: ev.Y, Button: downBtn,
							Template: downImg, Color: downColor, Window: downWindow,
							AnchorClickX: downClickX, AnchorClickY: downClickY,
							RelX: downRelX, RelY: downRelY, WinRel: downWinRel,
						},
						t: downT,
					})
				}
			}
		case recorder.EvKey:
			name := strings.ToLower(ev.KeyName)
			// The reserved arg key (default F9) drops a positional placeholder: the Nth
			// tap during the demo becomes a Type "{{N}}" step, filled at run time from
			// `<route> with a, b`. So you mark "an argument goes here" without typing
			// {{1}} into the app. Suppressed from the recording itself (it never types).
			if argKey != "" && argKey != "off" && name == argKey {
				if ev.Down {
					argN++
					ph := strconv.Itoa(argN) // positional fallback
					if argN-1 < len(opt.ArgNames) {
						ph = opt.ArgNames[argN-1] // named, from the "with …" clause
					}
					out = append(out, timedStep{
						step: macroir.Step{Kind: macroir.StepType, Text: "{{" + ph + "}}"},
						t:    ev.T,
					})
				}
				continue
			}
			if isModifier(name) {
				switch {
				case strings.Contains(name, "shift"):
					shift = ev.Down
				case strings.Contains(name, "ctrl"), name == "control":
					ctrl = ev.Down
				case strings.Contains(name, "alt"):
					alt = ev.Down
				case strings.Contains(name, "win"):
					win = ev.Down
				}
				continue
			}
			// A HELD key (≥ holdThreshold or spanning an action) becomes an explicit
			// press/release that brackets the steps in between — the hold persists.
			if holdDown[i] {
				out = append(out, timedStep{step: macroir.Step{Kind: macroir.StepKeyDown, Key: name}, t: ev.T})
				continue
			}
			if holdUp[i] {
				out = append(out, timedStep{step: macroir.Step{Kind: macroir.StepKeyUp, Key: name}, t: ev.T})
				continue
			}
			if !ev.Down {
				continue
			}
			// With a command modifier (Ctrl/Alt/Win) held, this is a shortcut, not
			// typing — emit a chord key (e.g. "ctrl+c", "ctrl+shift+esc") that the OS
			// Key capability presses as a held combo. No Text, so it won't fold into
			// a Type run.
			if ctrl || alt || win {
				out = append(out, timedStep{
					step: macroir.Step{Kind: macroir.StepKey, Key: chord(ctrl, alt, shift, win, name), Count: 1},
					t:    ev.T,
				})
				continue
			}
			// Emit one key step per press. Printable keys carry the typed char in
			// Text so a varied run can later be coalesced into typing; uniform
			// repeats (e.g. game-style "e" spam) stay key presses.
			step := macroir.Step{Kind: macroir.StepKey, Key: name, Count: 1}
			if ch, ok := printable(name, shift); ok {
				step.Text = ch
			}
			out = append(out, timedStep{step: step, t: ev.T})
		}
	}
	return out
}

// holdThreshold is the press duration at/above which a key counts as a HOLD rather than a
// tap even with nothing else happening (a charge-hold). A SHORTER press is still a hold if
// another action (a click, another key) happens before it's released.
const holdThreshold = 500 * time.Millisecond

// markHolds decides, with full lookahead, which key-DOWN/UP event indices are HOLDS: a
// non-modifier key whose press lasts ≥ holdThreshold OR spans another action (a click or
// another key press) before its release. Those become explicit KeyDown/KeyUp so the hold
// persists across the steps in between; everything else stays a tap. A press with no
// captured release (recording stopped mid-hold) is left a tap — it releases itself.
func markHolds(events []recorder.RecordedEvent, argKey string) (down, up map[int]bool) {
	down, up = map[int]bool{}, map[int]bool{}
	isArg := func(name string) bool { return argKey != "" && argKey != "off" && name == argKey }
	for i := range events {
		ev := events[i]
		if ev.Kind != recorder.EvKey || !ev.Down {
			continue
		}
		name := strings.ToLower(ev.KeyName)
		if isModifier(name) || isArg(name) {
			continue
		}
		j := -1
		for k := i + 1; k < len(events); k++ {
			e := events[k]
			if e.Kind == recorder.EvKey && !e.Down && strings.ToLower(e.KeyName) == name {
				j = k
				break
			}
		}
		if j < 0 {
			continue // no release captured → leave it a tap
		}
		held := events[j].T.Sub(ev.T) >= holdThreshold
		for k := i + 1; k < j && !held; k++ {
			e := events[k]
			ekn := strings.ToLower(e.KeyName)
			if (e.Kind == recorder.EvClick && e.Down) ||
				(e.Kind == recorder.EvKey && e.Down && !isModifier(ekn) && !isArg(ekn)) {
				held = true
			}
		}
		if held {
			down[i], up[j] = true, true
		}
	}
	return down, up
}

// isPrintableKey reports whether a step is a single printable keystroke (carries
// the typed character in Text).
func isPrintableKey(s macroir.Step) bool {
	return s.Kind == macroir.StepKey && s.Text != "" && s.Count <= 1
}

// coalesceTypingRhythm merges a varied run of printable keys into one Type step
// even when small waits (≤ maxGapMs — the rhythm of human typing) sit between
// them, dropping those interior waits. A uniform run (the same key repeated,
// e.g. game "e" spam) is left untouched so its timing is preserved. maxGapMs ≤ 0
// disables the pass (the faithful default).
func coalesceTypingRhythm(in []macroir.Step, maxGapMs int) []macroir.Step {
	if maxGapMs <= 0 {
		return in
	}
	var out []macroir.Step
	i := 0
	for i < len(in) {
		if !isPrintableKey(in[i]) {
			out = append(out, in[i])
			i++
			continue
		}
		// Scan a run of printable keys, allowing a single small wait between two
		// keys. lastKey marks the final key, so a trailing wait is left behind.
		var b strings.Builder
		distinct := map[string]bool{}
		j, lastKey := i, i
		for j < len(in) {
			if isPrintableKey(in[j]) {
				b.WriteString(in[j].Text)
				distinct[in[j].Text] = true
				lastKey = j
				j++
				continue
			}
			if in[j].Kind == macroir.StepWait && in[j].Ms <= maxGapMs &&
				j+1 < len(in) && isPrintableKey(in[j+1]) {
				j++
				continue
			}
			break
		}
		if len(distinct) >= 2 {
			out = append(out, macroir.Step{Kind: macroir.StepType, Text: b.String()})
		} else {
			out = append(out, in[i:lastKey+1]...)
		}
		i = lastKey + 1
	}
	return out
}

// coalesceTyping merges a maximal run of adjacent printable key steps (no wait
// between them) into a single type step — but only when the run is *varied*
// (≥2 distinct characters). A uniform run (the same key repeated, e.g. "e"
// spam in a game) is left as key presses for coalesceKeys to count.
func coalesceTyping(in []macroir.Step) []macroir.Step {
	var out []macroir.Step
	i := 0
	for i < len(in) {
		if in[i].Kind != macroir.StepKey || in[i].Text == "" {
			out = append(out, in[i])
			i++
			continue
		}
		j := i
		distinct := map[string]bool{}
		var b strings.Builder
		for j < len(in) && in[j].Kind == macroir.StepKey && in[j].Text != "" {
			b.WriteString(in[j].Text)
			distinct[in[j].Text] = true
			j++
		}
		if len(distinct) >= 2 {
			out = append(out, macroir.Step{Kind: macroir.StepType, Text: b.String()})
		} else {
			out = append(out, in[i:j]...)
		}
		i = j
	}
	return out
}

// foldActivates rewrites recorded app switches into clean Activate steps. An app
// switch (EvAppSwitch → StepActivate) means the user navigated to another app —
// by a taskbar click, Alt-Tab, etc. The gesture that performed the switch is
// brittle (fixed pixel coordinates that may hit a different app later), so we
// drop it and keep only the robust Activate, which focuses the app and launches
// it if it isn't running. Then redundant/empty activates are collapsed.
func foldActivates(in []timedStep) []timedStep {
	isActivate := func(ts timedStep) bool { return ts.step.Kind == macroir.StepActivate }
	isNav := func(ts timedStep) bool {
		return ts.step.Kind == macroir.StepClick || ts.step.Kind == macroir.StepMove
	}

	// 1. Drop the navigation gesture (a click/move) immediately before a switch.
	var a []timedStep
	for i := range in {
		if i+1 < len(in) && isActivate(in[i+1]) && isNav(in[i]) {
			continue
		}
		a = append(a, in[i])
	}

	// 2. Collapse a run of consecutive activates to its last (e.g. you started in
	// one app and immediately switched away), and drop a trailing activate that
	// has no steps after it (switching away at the very end means nothing).
	var b []timedStep
	for i := 0; i < len(a); i++ {
		if isActivate(a[i]) {
			if i+1 < len(a) && isActivate(a[i+1]) {
				continue
			}
			if i == len(a)-1 {
				continue
			}
		}
		b = append(b, a[i])
	}

	// 3. Drop an activate that re-focuses the app already in effect.
	var c []timedStep
	lastApp := ""
	for _, ts := range b {
		if isActivate(ts) {
			if ts.step.Text == lastApp {
				continue
			}
			lastApp = ts.step.Text
		}
		c = append(c, ts)
	}
	return c
}

// insertWaits keeps each step's REAL recorded gap (rounded), dropping gaps below
// MinWaitMs so fast actions stay back-to-back. It does NOT force a uniform delay
// between every action — opt.MaxWaitMs is only a CAP, applied later by capWaits, so
// a long human pause doesn't slow replay while genuinely fast actions add nothing.
// The window-activation latency that used to need a forced gap is now handled by the
// host (Activate waits for the window to be focused and stable before returning).
func insertWaits(timed []timedStep, opt Options) []macroir.Step {
	var out []macroir.Step
	for i, ts := range timed {
		if i > 0 {
			gap := ts.t.Sub(timed[i-1].t)
			ms := roundMs(int(gap/time.Millisecond), opt.WaitGranularityMs)
			if ms >= opt.MinWaitMs {
				out = append(out, macroir.Step{Kind: macroir.StepWait, Ms: ms})
			}
		}
		out = append(out, ts.step)
	}
	return out
}

// coalesceKeys folds runs of the same key (not separated by a wait) into a
// single step with Count.
func coalesceKeys(in []macroir.Step) []macroir.Step {
	var out []macroir.Step
	for _, s := range in {
		if s.Kind == macroir.StepKey && len(out) > 0 {
			last := &out[len(out)-1]
			if last.Kind == macroir.StepKey && last.Key == s.Key {
				if last.Count == 0 {
					last.Count = 1
				}
				last.Count += max(s.Count, 1)
				continue
			}
		}
		out = append(out, s)
	}
	return out
}

// mergeWaits collapses adjacent wait steps into one summed wait.
func mergeWaits(in []macroir.Step) []macroir.Step {
	var out []macroir.Step
	for _, s := range in {
		if s.Kind == macroir.StepWait && len(out) > 0 && out[len(out)-1].Kind == macroir.StepWait {
			out[len(out)-1].Ms += s.Ms
			continue
		}
		out = append(out, s)
	}
	return out
}

// capWaits clamps every wait step to max milliseconds, replacing the recorded
// human-paced delays with a uniform small gap. max < 0 is a no-op (exact
// timing preserved). Applied before loop folding so identical cycles that
// differ only in their recorded gap still fold into a single loop.
func capWaits(in []macroir.Step, max int) []macroir.Step {
	if max < 0 {
		return in
	}
	out := make([]macroir.Step, len(in))
	copy(out, in)
	for i := range out {
		if out[i].Kind == macroir.StepWait && out[i].Ms > max {
			out[i].Ms = max
		}
	}
	return out
}

// trimEdgeWaits drops waits at the very start and end of the sequence.
func trimEdgeWaits(in []macroir.Step) []macroir.Step {
	start := 0
	for start < len(in) && in[start].Kind == macroir.StepWait {
		start++
	}
	end := len(in)
	for end > start && in[end-1].Kind == macroir.StepWait {
		end--
	}
	return append([]macroir.Step(nil), in[start:end]...)
}

// foldLoops replaces exact contiguous repeats with loop steps, recursing into
// nested bodies. Conservative: only exact repeats are folded.
func foldLoops(in []macroir.Step, opt Options) []macroir.Step {
	var out []macroir.Step
	i := 0
	for i < len(in) {
		p, reps, covered := bestRepeat(in, i, opt)
		if p > 0 && reps >= opt.MinLoopReps {
			body := foldLoops(append([]macroir.Step(nil), in[i:i+p]...), opt)
			out = append(out, macroir.Step{Kind: macroir.StepLoop, Count: reps, Steps: body})
			i += covered
			continue
		}
		out = append(out, in[i])
		i++
	}
	return out
}

// bestRepeat finds the smallest period p at index i whose block tiles forward
// at least MinLoopReps times, preferring the longest total coverage. Returns
// (0,0,0) if no qualifying repeat. A block made only of waits is never folded.
// covered is how many input steps the reps span — usually period*reps, but a block
// that ends in a wait may have a final rep that omits that trailing wait (the
// inter-iteration pacing gap, absent at the very end of the recording), so the loop
// still folds cleanly; the body keeps the wait.
func bestRepeat(in []macroir.Step, i int, opt Options) (period, reps, covered int) {
	bestCover := 0
	for p := 1; i+p*opt.MinLoopReps <= len(in); p++ {
		block := in[i : i+p]
		if allWaits(block) {
			continue
		}
		r := 1
		for i+(r+1)*p <= len(in) && blocksEqual(in[i:i+p], in[i+r*p:i+(r+1)*p]) {
			r++
		}
		cov := r * p
		// Allow one more rep whose trailing wait is missing (trimmed end-of-recording).
		if p > 1 && block[p-1].Kind == macroir.StepWait {
			if tail := i + r*p; tail+p-1 <= len(in) && blocksEqual(block[:p-1], in[tail:tail+p-1]) {
				r++
				cov += p - 1
			}
		}
		if r >= opt.MinLoopReps && cov > bestCover {
			bestCover = cov
			period, reps, covered = p, r, cov
		}
	}
	return period, reps, covered
}

func blocksEqual(a, b []macroir.Step) bool { return reflect.DeepEqual(a, b) }

func allWaits(b []macroir.Step) bool {
	for _, s := range b {
		if s.Kind != macroir.StepWait {
			return false
		}
	}
	return true
}

// ── small helpers ──

func roundMs(ms, granularity int) int {
	if granularity <= 1 {
		return ms
	}
	return ((ms + granularity/2) / granularity) * granularity
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// chord builds a key-combo name like "ctrl+c" or "ctrl+shift+esc" from the held
// modifiers (in a stable order) plus the base key.
func chord(ctrl, alt, shift, win bool, key string) string {
	var parts []string
	if ctrl {
		parts = append(parts, "ctrl")
	}
	if alt {
		parts = append(parts, "alt")
	}
	if shift {
		parts = append(parts, "shift")
	}
	if win {
		parts = append(parts, "win")
	}
	parts = append(parts, key)
	return strings.Join(parts, "+")
}

func isModifier(name string) bool {
	switch name {
	case "shift", "lshift", "rshift", "ctrl", "control", "lctrl", "rctrl",
		"alt", "lalt", "ralt", "win", "lwin", "rwin":
		return true
	}
	return false
}

// usShift maps an unshifted symbol/digit key to its shifted character on a US
// layout, so typed text (and {{secret}} placeholders) record faithfully.
var usShift = map[byte]string{
	'1': "!", '2': "@", '3': "#", '4': "$", '5': "%", '6': "^", '7': "&",
	'8': "*", '9': "(", '0': ")", '-': "_", '=': "+", '[': "{", ']': "}",
	'\\': "|", ';': ":", '\'': "\"", ',': "<", '.': ">", '/': "?", '`': "~",
}

// printable returns the character a key types and true for a printable key with
// no control modifier. Shift uppercases letters and maps symbols/digits (US
// layout).
func printable(name string, shift bool) (string, bool) {
	if name == "space" {
		return " ", true
	}
	if len(name) == 1 {
		c := name[0]
		switch {
		case c >= 'a' && c <= 'z':
			if shift {
				return strings.ToUpper(name), true
			}
			return name, true
		case c >= '0' && c <= '9':
			if shift {
				if s, ok := usShift[c]; ok {
					return s, true
				}
			}
			return name, true
		case strings.ContainsRune("`-=[]\\;',./", rune(c)):
			if shift {
				if s, ok := usShift[c]; ok {
					return s, true
				}
			}
			return name, true
		}
	}
	return "", false
}
