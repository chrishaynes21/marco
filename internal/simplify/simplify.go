// Package simplify turns a raw recorded input stream into a clean macro step
// list: it lowers events to steps, inserts rounded waits, coalesces repeated
// keys, folds exact repeated cycles into loops, and drops noise. It is pure Go
// and OS-agnostic, so it is fully unit-testable without any hooks.
package simplify

import (
	"errors"
	"fmt"
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
	// macros are preserved. Used by Tighten / the "simplify further" learn option.
	TypingRhythmMs int
	// ArgKey is a reserved key (e.g. "f9") you tap during a demonstration to drop an
	// argument placeholder — the Nth tap becomes a Type "{{name}}" step (named from
	// ArgNames, else positional "{{N}}") that codegen keeps and the run fills from
	// `<route> name:value`. So you mark "an argument goes here" without typing into
	// the app. "" / "off" disables it.
	ArgKey string
	// ArgNames are the declared argument names (from the learn phrase's "with name,
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

// SimplifiedMaxWaitMs is what an explicit simplification clamps every remaining wait to.
//
// # Why simplifying touches waits at all
//
// Because shortening the routine is what somebody pressing Simplify is asking for, and on a
// recording of CLICKS the waits ARE its length. A demonstration of a game menu carries the
// person's own reaction time between every press — a measured one ran 2610ms, 2180ms,
// 2180ms, 1160ms and on: 19.6 seconds of waiting across 17 waits, in a routine whose typing was
// five characters. Folding typed runs cannot touch any of that, so Simplify was a no-op on
// exactly the recordings somebody most wants shortened, and said so only by showing two
// identical panes.
//
// # Why 250ms, and why this is not the cap `normalize` applies
//
// `MaxWaitMs`'s default of 50ms belongs to the anchored world: with CV on, the wait-gate does the
// real waiting and the recorded gap is scaffolding. With CV off — the default, and the state
// every Studio recording is made in — there is nothing to wait ON, so
// [[ADR-254-what-a-recording-captures-is-stated]] disables the cap entirely for a routine that
// states `coordinates`. That rule stands: an ordinary save and `RebuildCurrent` still keep every
// recorded gap exactly.
//
// This is the other half of that decision rather than a reversal of it. ADR-254's defect was the
// ENVIRONMENT deciding to cap, at save time, invisibly, with nothing on screen to say so. This is
// a person pressing a button named for shortening a routine, being shown the before and the
// after, and choosing to replace. 250ms is several frames at any refresh rate a game runs at and
// still a quarter of a second rather than a pause — the reasoning
// `workflowrun.DefaultDwell` was chosen under.
//
// IT CAN BE TOO FAST, AND THAT IS THE PERSON'S TO JUDGE. Nothing here knows when a menu will
// appear; that is precisely what an anchor would have known, and anchors are off. So a routine
// whose menu takes longer than this to draw will press before it is there. The caution beside the
// button says so in those words, the review shows both versions, and nothing is written until
// Replace.
const SimplifiedMaxWaitMs = 250

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
//
// The error is a REFUSAL and not a failure: the recording contains a gesture that cannot be
// written down faithfully, and the sentence says which and what to do instead. Every caller has
// to answer for it, which is why it is a second return rather than something to ask about
// afterwards — a path that simplified and saved without asking would be the exact defect this
// exists to prevent, and it would look like a working save. See modifierTrouble.
//
// The steps come back anyway. A person reviewing a refused recording is entitled to see what was
// understood of it, and nothing downstream may save them while the reason is set.
func Simplify(events []recorder.RecordedEvent, opt Options) ([]macroir.Step, error) {
	opt = opt.normalize()
	timed, gesture := lower(events, opt)
	timed = foldActivates(timed)
	steps := SimplifySteps(insertWaits(timed, opt), opt)
	if why := modifierTrouble(events, strings.ToLower(strings.TrimSpace(opt.ArgKey))); why != "" {
		return steps, errors.New(why)
	}
	// AND A GESTURE THAT SPANS TWO WINDOWS, which `lower` noticed while it was pairing
	// the edges. Second, because a modifier nobody let go of is the more urgent sentence
	// of the two and only one error comes back.
	if gesture != "" {
		return steps, errors.New(gesture)
	}
	return steps, nil
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
func lower(events []recorder.RecordedEvent, opt Options) ([]timedStep, string) {
	var out []timedStep
	// trouble is the first gesture in this recording that cannot be written down
	// faithfully, and it comes out of HERE rather than from a second pass over the
	// events: the pairing of a press with its release, the move test and the distance
	// test are what decide that a gesture is a drag at all, and a separate pass asking
	// the same question again would be a second chance to answer it differently.
	// `modifierTrouble` is a pass of its own because it is about keys nothing pairs.
	var trouble string
	shift, ctrl, alt, win := false, false, false, false
	// heldMods are the modifiers currently down as an EXPLICIT hold, as opposed to the
	// four flags above, which are the ones being tracked for a chord. The two are
	// mutually exclusive on purpose and the distinction is the whole of this fix: a
	// modifier in here has already been pressed by a step of its own, so a key arriving
	// under it is an ordinary keystroke under a held key and not a chord to expand.
	heldMods := map[string]bool{}

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
	var downIn pressedIn
	// downRect is the window the PRESS happened in, by the evidence that says which window
	// rather than what it looked like. Compared against the release's at the end of a drag.
	var downRect inRect
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
				downIn = pressedIn{relTo: ev.RelTo, w: ev.WinW, h: ev.WinH,
					dpi: ev.Dpi, process: ev.Process}
				downRect = rectOf(ev)
				sawMove = false
			} else {
				if !downActive {
					continue
				}
				downActive = false
				dist := abs(ev.X-downX) > opt.DragThresholdPx || abs(ev.Y-downY) > opt.DragThresholdPx
				if sawMove && dist {
					// A DRAG CARRIES THE WINDOW ITS TWO EDGES AGREE ABOUT, or
					// it carries the basis alone and is refused at Save, or —
					// when the edges describe two different rectangles — the
					// whole recording is refused with a sentence saying so.
					// `dragIn` is the one place that decides; see its comment.
					step, why := dragIn(downRect, rectOf(ev), macroir.Step{
						Kind: macroir.StepDrag, X: downX, Y: downY,
						X2: ev.X, Y2: ev.Y, Button: downBtn})
					if why != "" && trouble == "" {
						trouble = why
					}
					out = append(out, timedStep{step: step, t: downT})
				} else {
					// The click carries its window-relative offset (default — follows the
					// active window across monitors/position) and, when anchors are on, a
					// captured patch (codegen turns it into an image find). Drags carry
					// neither.
					//
					// AND THE POINT IS THE PRESS, NOT THE RELEASE.
					//
					// It used to be the release's coordinate while RelX,RelY came from
					// the press — so a hand that drifted a pixel between the two left a
					// step whose absolute fallback and whose window-relative offset named
					// different pixels, with the fallback being the less accurate of the
					// two. Everything else on this step is already the press's: the
					// template, the colour, the window title, the offset into the
					// template. The press is also where somebody AIMED; the release is
					// where their hand had got to.
					out = append(out, timedStep{
						step: downIn.onto(macroir.Step{
							Kind: macroir.StepClick, X: downX, Y: downY, Button: downBtn,
							Template: downImg, Color: downColor, Window: downWindow,
							AnchorClickX: downClickX, AnchorClickY: downClickY,
							RelX: downRelX, RelY: downRelY, WinRel: downWinRel,
						}),
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
			// A HELD key (≥ holdThreshold, or spanning an action) becomes an explicit
			// press/release that brackets the steps in between — the hold persists.
			//
			// BEFORE THE MODIFIER BRANCH, AND THAT ORDER IS THE SECOND HALF OF THE FIX.
			// The branch below used to come first and `continue` past these two, so a
			// modifier marked as a hold by `markHolds` would still have been swallowed
			// by it: the classification would have been right and nothing would have
			// come out. Teaching `markHolds` about modifiers without moving this would
			// have changed nothing at all, which is the kind of fix that looks done.
			if holdDown[i] {
				out = append(out, timedStep{step: macroir.Step{Kind: macroir.StepKeyDown, Key: name}, t: ev.T})
				if isModifier(name) {
					heldMods[name] = true
				}
				continue
			}
			if holdUp[i] {
				out = append(out, timedStep{step: macroir.Step{Kind: macroir.StepKeyUp, Key: name}, t: ev.T})
				delete(heldMods, name)
				continue
			}
			// AND A MODIFIER THAT IS NOT A HOLD IS STATE, FOR THE CHORD BELOW.
			//
			// Deliberately not set for one that IS a hold: the hold already presses the
			// key, and a chord built from it as well would press Ctrl a second time and
			// release it inside its own expansion — which is how "Ctrl held across a
			// keypress and then a click" would come out releasing Ctrl before the click.
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
			if !ev.Down {
				continue
			}
			// UNDER AN EXPLICIT HOLD, A KEY CARRIES ONLY THE MODIFIERS THE HOLD IS
			// NOT ALREADY SUPPLYING.
			//
			// `Ctrl↓ Shift↓ A↓ A↑ Shift↑ click Ctrl↑` is the case this is for. Ctrl
			// spans the click so it is a hold; Shift spans only the A, so it is
			// still chord state — and a bare `Key "a"` here drops it, performing
			// Ctrl+A where Ctrl+Shift+A was demonstrated. That was the first version
			// of this branch and the defect was invisible: every step looked right.
			//
			// The flags carry only the modifiers that are NOT held, and they carry
			// that by construction rather than by being filtered here: the hold
			// checks above `continue` before the branch that sets them, so a
			// modifier written as a hold never becomes chord state in the first
			// place. That ordering is the whole of why a chord cannot name a key the
			// hold is already holding — which matters, because `sendChord` presses
			// and releases what it is given, so naming Ctrl here would press it a
			// second time and let go of it inside a hold that is still open, leaving
			// the click after it unmodified.
			//
			// An earlier version of this filtered the held classes out again, here.
			// It was dead code — the flags were already false — and dead code that
			// looks load-bearing is worse than none, because the next reader trusts
			// it instead of the ordering. The ordering is what is pinned, by
			// `TestTheHoldMarkersAreReadBeforeTheModifierState`, which reads it off
			// the source: there is no input that can tell the two mechanisms apart,
			// so a test over behaviour cannot hold this.
			//
			// And NO TEXT either way, which is the subtler half: Text is what lets a
			// run of keys coalesce into a Type step, and Type sends characters rather
			// than keystrokes — so `a` under a held Ctrl would arrive as the letter
			// and not as Ctrl+A, after a fold nothing here could see. `chord` does
			// not set it, and the bare step below must not either.
			if len(heldMods) > 0 {
				k := name
				if ctrl || alt || shift || win {
					// SHIFT ALONE IS A MODIFIER HERE AND A CAPITAL LETTER
					// ANYWHERE ELSE. Outside a hold, Shift+A is how a capital
					// is typed and comes out as a key carrying the text "A".
					// Inside one it is part of a shortcut — Ctrl+Shift+A — and
					// a capital letter is not what was asked for.
					k = chord(ctrl, alt, shift, win, name)
				}
				out = append(out, timedStep{
					step: macroir.Step{Kind: macroir.StepKey, Key: k, Count: 1},
					t:    ev.T,
				})
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
	return out, trouble
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
		if isArg(name) {
			continue
		}
		j := releaseOf(events, i, name)
		if j < 0 {
			continue // no release captured → leave it a tap
		}
		// A MODIFIER IS A HOLD WHEN IT SPANS A POINTER ACTION, AND ONLY THEN.
		//
		// `ctrl+shift+a` is one chord step and must stay one: the modifiers around a KEY
		// fold into it, and emitting them as holds as well would press Ctrl twice and
		// release it in the middle of its own chord. But a modifier around a CLICK has no
		// chord to fold into, and the branch below used to skip every modifier by name —
		// so Ctrl+click came out as a plain click, which is a different act in most
		// applications and looked like a faithful recording.
		//
		// The DURATION rule deliberately does not apply here. A modifier held a long
		// while with nothing in between modified nothing, and a person resting a finger
		// on Shift while they think should not acquire a stray hold.
		if isModifier(name) {
			if spansAPointerAction(events, i, j) {
				down[i], up[j] = true, true
			}
			continue
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
		// AND A PRESS THAT ASKED TO BE CHECKED IS NEVER FOLDED INTO A REPEAT.
		//
		// Each such press can REFUSE, and a refusal has to stop the routine — which
		// the generated route says by giving every one of them its own capability that
		// calls the next only when its own press went through. A `repeat` has no way
		// to say that: the body would run again after a press that did not happen.
		// Three identical presses written out three times is the honest reading of
		// three presses that each have to be allowed.
		if p > 0 && reps >= opt.MinLoopReps && !anyGuarded(in[i:i+p]) {
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

// pressedIn is what the window a press landed in was like, as the press itself measured it.
//
// One value rather than five more locals beside the eight already tracked across a press and its
// release, because these five are one measurement and travel together: the basis the offset is
// measured from, the window's outer size, its DPI, and the program that owned it. See
// [[macroir.Step]] for what each is for and why DPI is its own term.
type pressedIn struct {
	relTo     string
	w, h, dpi int
	process   string
}

// onto copies the measurement onto a click step.
//
// A method rather than five assignments at the call site, so a term added to the measurement
// reaches the step without anybody having to notice the call site — which is the shape of the
// defect that left demonstrated points reaching one of two loops that build a step.
func (p pressedIn) onto(s macroir.Step) macroir.Step {
	s.RelTo, s.WinW, s.WinH, s.Dpi, s.Process = p.relTo, p.w, p.h, p.dpi, p.process
	return s
}

// inRect is one edge of a gesture and the window it happened in, as that edge measured it.
//
// Everything `pressedIn` has, plus the three terms a saved route never sees: the handle, its
// owner, and where the rectangle WAS. Those are recording evidence — they answer "which window",
// which nothing else here can. A rectangle's size, scale and program name are satisfied by two
// windows of one executable at the same size on the same display, and those are two windows.
type inRect struct {
	hwnd      uintptr
	pid       uint32
	x, y      int // where the rectangle was, which is never emitted
	w, h, dpi int
	process   string
	relTo     string
	rel       bool // this edge resolved an offset inside that rectangle
	rx, ry    int
}

func rectOf(ev recorder.RecordedEvent) inRect {
	return inRect{hwnd: ev.HWnd, pid: ev.PID, x: ev.WinX, y: ev.WinY,
		w: ev.WinW, h: ev.WinH, dpi: ev.Dpi, process: ev.Process,
		relTo: ev.RelTo, rel: ev.WinRel, rx: ev.RelX, ry: ev.RelY}
}

// measured reports whether this edge has everything a guarded point needs.
func (r inRect) measured() bool {
	return r.relTo != "" && r.rel && r.w > 0 && r.h > 0 && r.dpi > 0 && r.process != ""
}

// dragIn decides what a drag's two edges entitle it to, and is the ONE place that decides.
//
// Three outcomes, and the middle one is the reason this is a function rather than a condition:
//
//   - NEITHER EDGE DECLARED A WINDOW. A recording from a build before any of this, or one made
//     where nothing could be measured at all. The step is what it always was — two absolute
//     coordinates — and runs the way every drag saved before this does. Untouched, deliberately.
//   - THE TWO EDGES ARE ABOUT DIFFERENT WINDOWS. Refused, with a sentence, because there is no
//     honest step to write. Two offsets into two rectangles describe a gesture that crossed from
//     one to the other, and replaying it in one window would be reproducing neither half. A
//     window that MOVED mid-drag is the ordinary case of this and reads as exactly that.
//   - ONE WINDOW, AND NOT EVERYTHING MEASURED. The basis alone, which is the third saved
//     contract: a step that asked to be checked and cannot be. Replay refuses it naming the
//     missing term, and `codegen.Unpropagatable` refuses to SAVE it, which is where somebody
//     finds out. What must not happen is it quietly becoming the first case and running
//     unchecked — that is the whole reason the basis is written whether or not the rest arrived.
//
// And the destination's offset is the RELEASE's own measurement. Deriving it from the press's
// rectangle was the first design and it is an unchecked inference: an offset into the window as it
// was, attributed to a point captured later, with nothing able to tell whether the window moved
// in between. The second measurement is what turns that into a question with an answer.
//
// # What the second measurement is actually for, which mutation testing had to say out loud
//
// Replacing `up.rx` below with `s.X2 - down.x` — the derivation — passes every test, and it
// SHOULD: once the two edges are required to report the same origin, the two expressions are
// arithmetically identical. The mutant is equivalent, and that is not a gap in the tests.
//
// So the second measurement earns its place as the CHECK and not as the arithmetic. What it buys
// is the ability to notice, and the mutation that proves it is the origin comparison above:
// remove that and a window which moved mid-drag is silently reconciled into a gesture nobody
// demonstrated. Worth writing down, because "we measure the release so the destination is right"
// is the wrong reason and would survive somebody deleting the comparison it depends on.
func dragIn(down, up inRect, s macroir.Step) (macroir.Step, string) {
	if down.relTo == "" && up.relTo == "" {
		return s, "" // a legacy drag, and it stays one
	}
	// WHICH WINDOW, BEFORE WHAT IT LOOKED LIKE. A handle and its owner name the window;
	// the geometry describes it. Asking the describing questions first would let two
	// windows of one program at one size pass as one window.
	switch {
	case down.hwnd == 0 || up.hwnd == 0 || down.pid == 0 || up.pid == 0:
		s.RelTo = firstOf(down.relTo, up.relTo)
		return s, ""
	case down.hwnd != up.hwnd || down.pid != up.pid:
		s.RelTo = firstOf(down.relTo, up.relTo)
		return s, "this drag starts in one window and ends in another, so there is " +
			"no single window to measure it against — demonstrate it again " +
			"inside one window"
	case down.x != up.x || down.y != up.y:
		s.RelTo = firstOf(down.relTo, up.relTo)
		return s, "the window moved while this drag was being demonstrated, so where " +
			"it started and where it ended are offsets into two different " +
			"rectangles — demonstrate it again without moving the window"
	case down.w != up.w || down.h != up.h || down.dpi != up.dpi:
		s.RelTo = firstOf(down.relTo, up.relTo)
		return s, "the window changed size or scale while this drag was being " +
			"demonstrated, so what is inside it moved between the start and " +
			"the end — demonstrate it again"
	case down.rel && !up.rel:
		// AND A DRAG THAT ENDED OUTSIDE THE WINDOW IT STARTED IN.
		//
		// A sentence rather than silence, because this is something the person DID
		// and can do differently. `newClick` declines an offset for an edge that
		// landed outside the rectangle it attributed — the subtraction would
		// produce a number that is not an offset into anything — so a drag out of
		// a window arrives here with a measured start and an unmeasured end.
		s.RelTo = firstOf(down.relTo, up.relTo)
		return s, "this drag ends outside the window it started in, so where it " +
			"finishes is not a point in that window — demonstrate it again " +
			"inside one window"
	case !down.measured() || !up.measured():
		// ASKED AND UNMEASURABLE. The basis and nothing else, which replay refuses
		// and Save refuses, and which is NOT the same as an ordinary drag.
		//
		// No sentence, and the distinction from the case above is the whole reason
		// they are two cases: a scale Windows would not report or a program that
		// had exited is not a thing the person did, so telling them to demonstrate
		// it differently would be telling them to do something that cannot help.
		// Save names the missing term instead.
		s.RelTo = firstOf(down.relTo, up.relTo)
		return s, ""
	}
	s = pressedIn{relTo: down.relTo, w: down.w, h: down.h, dpi: down.dpi,
		process: down.process}.onto(s)
	s.RelX, s.RelY, s.WinRel = down.rx, down.ry, true
	s.ToRelX, s.ToRelY = up.rx, up.ry
	return s, ""
}

func firstOf(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// anyGuarded reports whether any of these steps asked to be checked against the window it was
// recorded in, at any depth.
//
// The one question `foldLoops` asks before folding. Nested too, because a repeat inside a repeat
// is still a body that would run again after a press that did not happen.
func anyGuarded(steps []macroir.Step) bool {
	for _, s := range steps {
		if s.RelTo != "" {
			return true
		}
		if len(s.Steps) > 0 && anyGuarded(s.Steps) {
			return true
		}
	}
	return false
}

// releaseOf is the index of the key-UP that ends the press at index i, or -1 if the recording
// never captured one.
//
// Split out when modifiers started being candidates for a hold, because three things now need
// the same answer and a second copy of this loop would be a second chance to disagree about it.
func releaseOf(events []recorder.RecordedEvent, i int, name string) int {
	for k := i + 1; k < len(events); k++ {
		e := events[k]
		if e.Kind == recorder.EvKey && !e.Down && strings.ToLower(e.KeyName) == name {
			return k
		}
	}
	return -1
}

// spansAPointerAction reports whether a pointer action is COMPLETED between two indices.
//
// The emission point of a click, not its press: `lower` turns a press-and-release into one step
// when the release arrives, because that is when it knows whether the gesture was a click or a
// drag. So a modifier whose hold is to bracket a click has to still be down at that point, and
// asking about the press instead would mark a hold whose release the lowering then emits BEFORE
// the click it was supposed to modify.
//
// A bare move is not an action: `lower` discards hover movement, so a modifier held across one
// has modified nothing.
func spansAPointerAction(events []recorder.RecordedEvent, from, to int) bool {
	for k := from + 1; k < to; k++ {
		if events[k].Kind == recorder.EvClick && !events[k].Down {
			return true
		}
	}
	return false
}

// modifierTrouble is the gesture this recording contains that cannot be written down faithfully,
// or "" when there is none.
//
// # Why a refusal and not a best guess
//
// Everything else here degrades gracefully on purpose: an unreleased ordinary key becomes a tap,
// hover movement disappears, a wait gets rounded. None of those changes what the routine DOES.
// Dropping a modifier does: Ctrl+click and click are different acts in most applications — one
// adds to a selection, one replaces it — so a routine that quietly lost the Ctrl performs a
// different gesture while reading as a faithful recording of the one that was shown. That is the
// one failure this whole package is least able to make visible afterwards, so it is refused at
// the point where it is still obvious what happened.
//
// Three ways a modifier around a pointer action cannot be written down, and all three are about
// not knowing WHEN it was held rather than whether:
//
//	released mid-click     the modifier came up between the press and the release of a
//	                       click, so neither "held across it" nor "not held" is what happened
//	never released         the recording ends with it down, so a hold written from it would
//	                       have no release and would leave the key down on a real desktop
//	never pressed          it was already down when recording began, so there is no instant
//	                       to press it at and no way to know what else it was modifying
func modifierTrouble(events []recorder.RecordedEvent, argKey string) string {
	isArg := func(name string) bool { return argKey != "" && argKey != "off" && name == argKey }
	seen := map[string]bool{}
	for i := range events {
		ev := events[i]
		if ev.Kind != recorder.EvKey {
			continue
		}
		name := strings.ToLower(ev.KeyName)
		if !isModifier(name) || isArg(name) {
			continue
		}
		if !ev.Down {
			// A RELEASE OF SOMETHING THIS RECORDING NEVER SAW PRESSED. Only
			// trouble if something was pressed under it; a stray modifier release
			// before anything happened modified nothing.
			if !seen[name] && aPointerActionHappenedBefore(events, i) {
				return fmt.Sprintf("%s was released during this recording and "+
					"never pressed in it, so it was already held when "+
					"recording began — and a press was made while it was "+
					"down. There is no instant in the recording to press "+
					"it at, so what that press actually was cannot be "+
					"written down. Record it again, pressing %s yourself "+
					"after recording starts", name, name)
			}
			continue
		}
		seen[name] = true
		j := releaseOf(events, i, name)
		if j < 0 {
			if aPointerActionHappenedAfter(events, i) {
				return fmt.Sprintf("%s was pressed and never released in this "+
					"recording, and a press was made while it was down. A "+
					"hold written from that would have no release and would "+
					"leave %s down on the desktop after the routine "+
					"finished. Record it again, letting go of %s before you "+
					"stop", name, name, name)
			}
			continue
		}
		// EITHER EDGE LANDING INSIDE A PRESS IS THE SAME AMBIGUITY.
		//
		// Released mid-press: held across the whole thing and let go before it are
		// different gestures. Pressed mid-press: the application read the keyboard
		// when the button went down, so the press was unmodified — and writing a hold
		// around it would claim a gesture that did not happen. Neither is answerable
		// from the recording, and the pair of them is why spansAPointerAction's choice
		// of which click edge to test cannot change an outcome: every case where the
		// two edges disagree is refused here first.
		if insideAClick(events, i) || insideAClick(events, j) {
			return fmt.Sprintf("%s was let go of in the middle of a press — after the "+
				"mouse button went down and before it came up. Holding it across "+
				"the whole press and letting go before it are different gestures, "+
				"and this recording does not say which one was meant. Record it "+
				"again, keeping %s down until after the click", name, name)
		}
	}
	return ""
}

// aPointerActionHappenedBefore / After are the two halves of "was anything pressed while this
// modifier was down", for the cases where one end of the hold is missing.
//
// Asked at all because an unbalanced modifier is only a problem when something happened under it.
// A recording that begins with a stray Shift release and contains no press at all has lost
// nothing, and refusing it would be refusing a recording for a key that modified nothing.
func aPointerActionHappenedBefore(events []recorder.RecordedEvent, to int) bool {
	return spansAPointerAction(events, -1, to)
}

func aPointerActionHappenedAfter(events []recorder.RecordedEvent, from int) bool {
	return spansAPointerAction(events, from, len(events))
}

// insideAClick reports whether the event at index k falls between a mouse-down and its own
// mouse-up.
//
// The ambiguous instant, asked of both ends of a modifier hold. Everything else about a modifier
// is answerable from the ORDER of the events; this is not, because the application reads the
// keyboard when the button goes down while the recording's own step is emitted when it comes up.
// A modifier whose press or release lands in that gap was held for part of a press, and "part"
// is not a thing a routine can say.
//
// A mouse-down with no release does not contain anything: the lowering emits no step for it at
// all, so there is no gesture for a modifier to be ambiguous about.
func insideAClick(events []recorder.RecordedEvent, k int) bool {
	for d := 0; d < k; d++ {
		if events[d].Kind != recorder.EvClick || !events[d].Down {
			continue
		}
		for u := d + 1; u < len(events); u++ {
			if events[u].Kind != recorder.EvClick || events[u].Down {
				continue
			}
			if u > k {
				return true
			}
			break // that press closed before k; look for a later one
		}
	}
	return false
}
