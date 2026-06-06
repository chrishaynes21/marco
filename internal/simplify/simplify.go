// Package simplify turns a raw recorded input stream into a clean macro step
// list: it lowers events to steps, inserts rounded waits, coalesces repeated
// keys, folds exact repeated cycles into loops, and drops noise. It is pure Go
// and OS-agnostic, so it is fully unit-testable without any hooks.
package simplify

import (
	"reflect"
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
}

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
	// FoldLoops defaults to true; callers that want it off must set it explicitly
	// via DefaultOptions then flip it.
	return o
}

// DefaultOptions returns the recommended defaults (loop folding on).
func DefaultOptions() Options {
	o := Options{FoldLoops: true}.normalize()
	return o
}

// Simplify lowers a recorded event stream into a clean step list.
func Simplify(events []recorder.RecordedEvent, opt Options) []macroir.Step {
	opt = opt.normalize()
	timed := lower(events, opt)
	steps := insertWaits(timed, opt)
	return SimplifySteps(steps, opt)
}

// SimplifySteps re-simplifies an existing step list (coalesce typing, coalesce
// keys, merge waits, trim ends, fold cycles). Safe to run on hand-written or
// previously recorded steps.
func SimplifySteps(in []macroir.Step, opt Options) []macroir.Step {
	opt = opt.normalize()
	out := coalesceTyping(in)
	out = coalesceKeys(out)
	out = mergeWaits(out)
	out = trimEdgeWaits(out)
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
	shift := false

	// mouse-down tracking for drag detection
	downActive := false
	var downX, downY int
	var downBtn string
	var downT time.Time
	sawMove := false

	for _, ev := range events {
		switch ev.Kind {
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
					out = append(out, timedStep{
						step: macroir.Step{Kind: macroir.StepClick, X: ev.X, Y: ev.Y, Button: downBtn},
						t:    downT,
					})
				}
			}
		case recorder.EvKey:
			name := strings.ToLower(ev.KeyName)
			if isModifier(name) {
				if strings.Contains(name, "shift") {
					shift = ev.Down
				}
				continue
			}
			if !ev.Down {
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

// insertWaits computes the gap before each anchor (relative to the previous
// anchor), rounds it, and emits a wait step when it clears the threshold.
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
		p, reps := bestRepeat(in, i, opt)
		if p > 0 && reps >= opt.MinLoopReps {
			body := foldLoops(append([]macroir.Step(nil), in[i:i+p]...), opt)
			out = append(out, macroir.Step{Kind: macroir.StepLoop, Count: reps, Steps: body})
			i += p * reps
			continue
		}
		out = append(out, in[i])
		i++
	}
	return out
}

// bestRepeat finds the smallest period p at index i whose block tiles forward
// at least MinLoopReps times, preferring the longest total coverage. Returns
// (0,0) if no qualifying repeat. A block made only of waits is never folded.
func bestRepeat(in []macroir.Step, i int, opt Options) (period, reps int) {
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
		if r >= opt.MinLoopReps && p*r > bestCover {
			bestCover = p * r
			period, reps = p, r
		}
	}
	return period, reps
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
