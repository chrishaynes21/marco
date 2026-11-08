package main

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// ↑ STARTS AT THE NEWEST AND WALKS BACK, ↓ WALKS FORWARD AND THEN PUTS BACK THE LINE AS IT WAS.
func TestUpAndDownWalkTheCommandsLikeAShell(t *testing.T) {
	h := newModel()
	for _, c := range []string{"open social", "cloud pull", "discord mute"} {
		h.remember(c)
	}
	draft := &lineProposal{slug: "cloud-pull"}
	h.setEditing(true)
	h.setInput(" half typ")
	h.setProposal(draft)

	step := func(move func(), want string) {
		t.Helper()
		move()
		if got := h.snapshot().input; got != want {
			t.Fatalf("the line is %q, want %q", got, want)
		}
	}
	step(h.recallOlder, " discord mute")
	if h.lineProposal() != nil {
		t.Error("a recalled line kept the proposal of the line it replaced")
	}
	step(h.recallOlder, " cloud pull")
	step(h.recallOlder, " open social")
	step(h.recallOlder, " open social") // the oldest stays
	step(h.recallNewer, " cloud pull")
	step(h.recallNewer, " discord mute")
	step(h.recallNewer, " half typ") // past the newest: what was being typed
	if h.lineProposal() != draft {
		t.Error("the line put back did not get its proposal back")
	}
	step(h.recallNewer, " half typ")     // and ↓ again does nothing
	step(h.recallOlder, " discord mute") // a new walk starts at the newest again

	// AND A LINE OPENED AFRESH STARTS A NEW WALK, with nothing left of the last.
	h.endRecall()
	h.setInput("")
	step(h.recallNewer, "")
	step(h.recallOlder, " discord mute")

	// NOTHING HAPPENS WITH THE LINE CLOSED.
	h.setEditing(false)
	h.setInput("")
	step(h.recallOlder, "")
}

// THE SAME COMMAND TWICE RUNNING IS KEPT ONCE, and only so many are kept.
func TestTheRecallKeepsEachCommandOnceAndNotForever(t *testing.T) {
	h := newModel()
	h.remember("open social")
	h.remember(" open social ")
	h.remember("cloud pull")
	h.remember("open social")
	if got := h.recalled(); !reflect.DeepEqual(got, []string{"open social", "cloud pull", "open social"}) {
		t.Errorf("kept %q", got)
	}
	for i := 0; i < recallKeeps+10; i++ {
		h.remember("c" + strings.Repeat("x", i))
	}
	if got := h.recalled(); len(got) != recallKeeps || got[len(got)-1] != "c"+strings.Repeat("x", recallKeeps+9) {
		t.Errorf("kept %d, newest %q", len(got), got[len(got)-1])
	}
}

// A ROUTINE IS RECALLED AS IT RAN, values and all, typed or spoken, so ↑ then Enter runs it again;
// what the microphone heard in passing is still not kept.
//
// Keeping a routine by its name only must fail this.
func TestARecalledRoutineComesBackWithItsValues(t *testing.T) {
	r := aCloudPull("cloud pull stone", "stone")
	repeat := aSocialRepeat("repeat open social five times")
	spokenRoutine := aCloudPull("cloud pull wood", "wood")
	engineReads(t, map[string]lineReading{
		r.Line: r, repeat.Line: repeat, spokenRoutine.Line: spokenRoutine,
		"make the coffee": {Line: "make the coffee", Kind: "none"},
		"mumble mumble":   {Line: "mumble mumble", Kind: "none"},
	})
	runs := runsRecorded(t)
	h := newModel()

	onScreen(h, r)
	typedEnter(h, r.Line)
	onScreen(h, repeat)
	typedEnter(h, repeat.Line)
	typedEnter(h, "make the coffee")
	dispatch(h, request{Action: "RunVoice", Input: "mumble mumble"})
	dispatch(h, request{Action: "RunVoice", Input: spokenRoutine.Line})
	typedEnter(h, "help")
	typedEnter(h, "learn") // the widget's own word, answered before any reading
	_ = runs()

	// The spoken routine is cloud pull again, kept by its name; the spoken mumble is not kept.
	want := []string{"cloud pull stone", "repeat open social five times", "make the coffee", "cloud pull wood", "help", "learn"}
	got := h.recalled()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("kept %q, want %q", got, want)
	}
	for _, c := range got {
		if strings.Contains(c, "mumble") {
			t.Errorf("a passing transcript was kept: %q", c)
		}
	}

	// AND ON THE LINE IT IS THE COMMAND AS IT RAN.
	h.setEditing(true)
	h.setInput("")
	for h.snapshot().input != " cloud pull stone" {
		before := h.snapshot().input
		h.recallOlder()
		if h.snapshot().input == before {
			t.Fatalf("cloud pull was never recalled; kept %q", got)
		}
	}
}

// A LINE THAT HAD NO READING IS KEPT AS TYPED, and stays so once it has run as a routine: the
// values are part of the command. Putting the routine's bare name in its place must fail this.
func TestALineWithNoReadingIsKeptAsTyped(t *testing.T) {
	engineReads(t, map[string]lineReading{})
	done := make(chan struct{}, 1)
	t.Cleanup(stubIntakeChild(t, func(*model, string, bool, ...string) childRun {
		defer func() { done <- struct{}{} }()
		return childRun{result: "performed", route: "cloud pull"}
	}))
	h := newModel()
	typedEnter(h, "cloud pull stone")
	<-done
	time.Sleep(50 * time.Millisecond)
	if got := h.recalled(); !reflect.DeepEqual(got, []string{"cloud pull stone"}) {
		t.Errorf("kept %q", got)
	}
}
