//go:build windows

package recorder

import (
	"strings"
	"testing"
	"time"
)

// THE BUFFER AND THE LIVE CHANNEL, GATED TOGETHER.
//
// `confine_test.go` holds the rules. This holds the wiring: that `observe` — the function both
// hook callbacks call — asks the boundary at all, and that an event the boundary drops reaches
// NEITHER the recording nor anybody listening to it.
//
// # Why the channel is a separate assertion from the buffer
//
// Because the first version of this gated only the buffer, on the reasoning that the live
// channel exists for the stop gesture and a consumer can filter. That is the same mistake one
// step along: a consumer of that channel holds whatever is published, so publishing every
// off-target keystroke exposes somebody's message to it however carefully the buffer is kept.
// What a stop needs is the stop keys (see StopOnly), and that is all an off-target event may
// say.
//
// These drive the production function with the one desktop read supplied, which is the pattern
// `statedmode_windows_test.go` set: `latchedTarget` is the seam that needs a real foreground
// window, and what `observe` does with its answer is what these are about.

// aConfinedRecording puts the package into the state a confined recording in flight is in, and
// hands back a reader of the live channel.
func aConfinedRecording(t *testing.T) <-chan RecordedEvent {
	t.Helper()
	aCleanPackage(t)
	recMu.Lock()
	recBuf = nil
	recPub = make(chan RecordedEvent, 64)
	bound = NewBoundary()
	ch := recPub
	recMu.Unlock()
	confined.Store(true)
	return ch
}

// recorded is what the buffer holds, by kind, and what reached the channel.
func recorded(t *testing.T, ch <-chan RecordedEvent) (buf, pub []RecordedEvent) {
	t.Helper()
	recMu.Lock()
	buf = append(buf, recBuf...)
	recMu.Unlock()
	for {
		select {
		case ev := <-ch:
			pub = append(pub, ev)
		default:
			return buf, pub
		}
	}
}

func keysIn(evs []RecordedEvent) string {
	var out []string
	for _, ev := range evs {
		switch ev.Kind {
		case EvKey:
			out = append(out, ev.KeyName)
		case EvClick:
			out = append(out, ev.Button)
		case EvMove:
			out = append(out, "move")
		}
	}
	return strings.Join(out, ",")
}

// AN OFF-TARGET KEYSTROKE IS IN NEITHER THE RECORDING NOR THE CHANNEL.
func TestOffTargetInputReachesNeitherTheBufferNorTheChannel(t *testing.T) {
	ch := aConfinedRecording(t)
	now := time.Now()
	// IN THE WINDOW.
	observe(RecordedEvent{Kind: EvKey, KeyName: "h", Down: true, T: now}, true)
	observe(RecordedEvent{Kind: EvKey, KeyName: "h", Down: false, T: now}, true)
	// AND AWAY — somebody's message, which is the whole reason for the rule.
	for _, k := range []string{"s", "e", "c", "r", "e", "t"} {
		observe(RecordedEvent{Kind: EvKey, KeyName: k, Down: true, T: now}, false)
		observe(RecordedEvent{Kind: EvKey, KeyName: k, Down: false, T: now}, false)
	}
	observe(RecordedEvent{Kind: EvMove, X: 40, Y: 40, T: now}, false)
	// A CLICK OUT THERE CARRIES COORDINATES AND A PICTURE, and must not be recorded
	// either. Its index is what `recordClickDown` reads to decide whether to ask for the
	// picture at all.
	if idx := observe(RecordedEvent{Kind: EvClick, Button: "left", Down: true,
		X: 900, Y: 400, T: now}, false); idx >= 0 {
		t.Errorf("an off-target press was recorded at index %d, so a screen capture "+
			"of a window nobody chose would be queued against it", idx)
	}
	observe(RecordedEvent{Kind: EvClick, Button: "left", Down: false, X: 900, Y: 400,
		T: now}, false)
	// AND BACK.
	observe(RecordedEvent{Kind: EvKey, KeyName: "i", Down: true, T: now}, true)

	buf, pub := recorded(t, ch)
	if got := keysIn(buf); got != "h,h,i" {
		t.Errorf("the recording holds %q, not just what happened in the window", got)
	}
	if got := keysIn(pub); got != "h,h,i" {
		t.Errorf("the live channel carried %q: a consumer of it holds input from "+
			"outside the window", got)
	}
}

// AND THE STOP KEY IS PUBLISHED FROM ANYWHERE WITHOUT BEING RECORDED.
//
// Both halves. Published, because `waitForStop` and `cmd/marco`'s panic stop detect the stop
// from this channel and a person who has walked away is entitled to stop the recording.
// NOT recorded, because it did not happen in the window.
func TestTheStopKeyFromElsewhereIsPublishedButNotRecorded(t *testing.T) {
	ch := aConfinedRecording(t)
	now := time.Now()
	observe(RecordedEvent{Kind: EvKey, KeyName: "a", Down: true, T: now}, true)
	observe(RecordedEvent{Kind: EvKey, KeyName: "a", Down: false, T: now}, true)
	// AWAY, AND THE STOP GESTURE.
	//
	// `stopKey` is package state read from $MARCO_STOP_KEY when the process starts, so
	// this STATES it for the duration rather than skipping when the environment holds
	// a chord. A skip would be worse than useless here: `cmd/gate` tolerates one only
	// when `gate-skips.txt` names it, and a declaration whose test did not skip fails
	// the gate too — so a conditional skip that fires on somebody's machine and not in
	// the gate cannot be declared either way.
	wasStop := stopKey
	t.Cleanup(func() { stopKey = wasStop })
	stopKey = ParseStopKey("")
	stop := DefaultStopKey
	observe(RecordedEvent{Kind: EvKey, KeyName: stop, Down: true, T: now}, false)

	buf, pub := recorded(t, ch)
	if got := keysIn(buf); got != "a,a" {
		t.Errorf("the recording holds %q: the stop key was pressed outside the "+
			"window and is in the demonstration", got)
	}
	if got := keysIn(pub); got != "a,a,"+stop {
		t.Errorf("the channel carried %q, so a stop from outside the window never "+
			"reaches the thing waiting for it", got)
	}
	// AND THE PRODUCTION DETECTOR AGREES, which is what makes the line above matter
	// rather than being a fact about a string.
	sk := ParseStopKey("")
	var triggered bool
	for _, ev := range pub {
		if sk.Triggered(ev) {
			triggered = true
		}
	}
	if !triggered {
		t.Error("StopKey.Triggered never fired on what was published, so the " +
			"recording cannot be stopped from outside the window")
	}
}

// A CROSSING IS AN EVENT IN THE RECORDING, which is how `Finish` learns of it and how a surface
// watching the channel learns of it at the instant it happened.
func TestACrossingIsRecordedAndPublishedOnce(t *testing.T) {
	ch := aConfinedRecording(t)
	now := time.Now()
	observe(RecordedEvent{Kind: EvKey, KeyName: "ctrl", Down: true, T: now}, false)
	observe(RecordedEvent{Kind: EvClick, Button: "left", Down: true, T: now}, true)
	observe(RecordedEvent{Kind: EvClick, Button: "left", Down: false, T: now}, true)

	buf, pub := recorded(t, ch)
	crossings := 0
	var why string
	for _, ev := range buf {
		if ev.Kind == EvCrossed {
			crossings++
			why = ev.KeyName
		}
	}
	if crossings != 1 {
		t.Fatalf("the recording holds %d crossing markers, not one", crossings)
	}
	if !strings.Contains(why, "ctrl") {
		t.Errorf("the crossing does not say what was held: %q", why)
	}
	// ONCE, not on every event after it: a marker per press would fill the recording
	// with the same sentence.
	published := 0
	for _, ev := range pub {
		if ev.Kind == EvCrossed {
			published++
		}
	}
	if published != 1 {
		t.Errorf("%d crossing markers reached the channel, not one", published)
	}
}

// THE BOUNDARY MARKERS ARE IN THE RECORDING AND ON THE CHANNEL, because the clock is made from
// them and because a surface showing capture state is entitled to know the recording is paused.
func TestLeavingAndReturningAreRecordedAsCaptureState(t *testing.T) {
	ch := aConfinedRecording(t)
	now := time.Now()
	// WHOLE GESTURES ON BOTH SIDES, so this case is about the markers alone. Writing it
	// with a key left down produced a crossing marker and a fourth publication, which is
	// the rule in `TestACrossingIsRecordedAndPublishedOnce` biting a test that did not
	// mean to ask about it.
	observe(RecordedEvent{Kind: EvKey, KeyName: "a", Down: true, T: now}, true)
	observe(RecordedEvent{Kind: EvKey, KeyName: "a", Down: false, T: now}, true)
	observe(RecordedEvent{Kind: EvKey, KeyName: "b", Down: true, T: now.Add(time.Second)}, false)
	observe(RecordedEvent{Kind: EvKey, KeyName: "b", Down: false, T: now.Add(time.Second)}, false)
	observe(RecordedEvent{Kind: EvKey, KeyName: "c", Down: true, T: now.Add(2 * time.Second)}, true)

	buf, pub := recorded(t, ch)
	var kinds []string
	for _, ev := range buf {
		switch ev.Kind {
		case EvLeft:
			kinds = append(kinds, "left")
		case EvEntered:
			kinds = append(kinds, "entered")
		case EvKey:
			kinds = append(kinds, ev.KeyName)
		}
	}
	// The first event is in the window, so `Arrived` reports entering before it.
	if got := strings.Join(kinds, ","); got != "entered,a,a,left,entered,c" {
		t.Errorf("the recording holds %q", got)
	}
	markers := 0
	for _, ev := range pub {
		if ev.Kind.Marker() {
			markers++
		}
	}
	if markers != 3 {
		t.Errorf("%d markers reached the channel, not 3: a surface watching it "+
			"cannot say whether the capture is paused", markers)
	}
}

// AND AN UNCONFINED RECORDING IS WHAT IT ALWAYS WAS, which is what "the whole desktop is still
// available" means in the code rather than in a sentence.
func TestAnUnconfinedRecordingAdmitsEverything(t *testing.T) {
	aCleanPackage(t)
	recMu.Lock()
	recBuf = nil
	recPub = make(chan RecordedEvent, 64)
	bound = nil
	ch := recPub
	recMu.Unlock()
	confined.Store(false)

	now := time.Now()
	// `inside` false, which is what `latchedTarget` answers when nothing is confined —
	// and it must make no difference at all.
	for _, k := range []string{"a", "b"} {
		if idx := observe(RecordedEvent{Kind: EvKey, KeyName: k, Down: true,
			T: now}, false); idx < 0 {
			t.Fatalf("%q was dropped by a recording of the whole desktop", k)
		}
	}
	buf, pub := recorded(t, ch)
	if got := keysIn(buf); got != "a,b" {
		t.Errorf("the recording holds %q", got)
	}
	if got := keysIn(pub); got != "a,b" {
		t.Errorf("the channel carried %q", got)
	}
}

// A KEY THE LEAVING STRANDED REACHES THE RECORDING AS A FACT, AND NOT AS A REFUSAL.
//
// The wiring between the boundary and the buffer, which no test of either alone can hold: the
// boundary reports the key, `inWindowClock` decides what it meant, and between them is this
// door. Delete the drain and the decision has nothing to read — every recording ended by
// Alt-Tab goes back to refusing, and every unit test of the boundary still passes.
//
// It is also the half that says NO CROSSING is written here. The boundary used to refuse at this
// instant, and at this instant there is nothing to refuse on: the only thing known is that a key
// was down when the foreground moved.
func TestAKeyStrandedByLeavingReachesTheRecordingAsAFact(t *testing.T) {
	ch := aConfinedRecording(t)
	now := time.Now()
	observe(RecordedEvent{Kind: EvKey, KeyName: "a", Down: true, T: now}, true)
	observe(RecordedEvent{Kind: EvKey, KeyName: "a", Down: false, T: now}, true)
	observe(RecordedEvent{Kind: EvKey, KeyName: "alt", Down: true, T: now}, true)
	// AND THE SWITCHER TAKES THE FOREGROUND WHILE ALT IS STILL DOWN.
	observe(RecordedEvent{Kind: EvKey, KeyName: "tab", Down: true, T: now}, false)

	buf, pub := recorded(t, ch)
	for _, ev := range buf {
		if ev.Kind == EvCrossed {
			t.Errorf("leaving the window refused the recording on the spot: %q",
				ev.KeyName)
		}
	}
	left, held := -1, -1
	for i, ev := range buf {
		switch {
		case ev.Kind == EvLeft:
			left = i
		case ev.Kind == EvHeldAcross && ev.KeyName == "alt":
			held = i
		}
	}
	if held < 0 {
		t.Fatalf("the key the leaving stranded is not in the recording: %v", buf)
	}
	// AFTER THE LEAVING IT BELONGS TO, because that is how a reader of the stream knows
	// which absence stranded it.
	if left < 0 || held < left {
		t.Errorf("the stranded key is at %d and the leaving at %d", held, left)
	}
	// AND A SURFACE WATCHING THE CHANNEL SEES IT TOO, like every other marker.
	for _, ev := range pub {
		if ev.Kind == EvHeldAcross {
			return
		}
	}
	t.Error("the stranded key never reached the channel")
}
