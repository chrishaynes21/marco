package main

// REPEAT IN THE WIDGET (cmd/marco repeat.go): a line asking for a routine N times is reviewed as
// "Open social ×5" and runs only as its identity with --times, and a count Director's resolver
// came back with is put on the command line for the same review.

import (
	"testing"
	"time"
)

// aSocialRepeat is the engine's reading of "repeat open social five times".
func aSocialRepeat(line string) lineReading {
	return lineReading{Line: line, Kind: "routine", ID: "open-social", Slug: "open-social",
		Name: "open social", Readback: "Open social ×5", Runnable: true, SourceAt: "v1", Times: 5}
}

// A REPEAT IS NEVER RUN UNREAD, even of a routine that takes no values, and runs with its count.
//
// Treating a repeat of a no-value routine as an ordinary line (to the intake as words), or
// dropping --times, must fail this.
func TestARepeatRunsOnlyFromItsReadBackWithItsCount(t *testing.T) {
	r := aSocialRepeat("repeat open social five times")
	engineReads(t, map[string]lineReading{r.Line: r})
	runs := runsRecorded(t)
	h := newModel()

	typedEnter(h, r.Line) // the read-back was not on screen yet
	if got := runs(); len(got) != 0 {
		t.Fatalf("a repeat nobody had seen read back ran: %q", got)
	}
	if s := h.snapshot(); !s.editing || s.input != " "+r.Line {
		t.Fatalf("the repeat is not open for review: editing %v, input %q", s.editing, s.input)
	}

	onScreen(h, r)
	typedEnter(h, r.Line)
	want := "do --source=typed --play=open-social --expect=v1 --times=5"
	if got := runs(); len(got) != 1 || got[0] != want {
		t.Errorf("Enter on the read-back ran %q, want %q", got, want)
	}
}

// THE READ-BACK UNDER THE LINE SAYS THE COUNT.
func TestARepeatIsDrawnWithItsCount(t *testing.T) {
	r := aSocialRepeat("repeat open social five times")
	h := newModel()
	h.setInput(r.Line)
	h.setEditing(true)
	h.setReading(r.Line, r)
	text := ""
	for _, run := range readingRuns(h.snapshot()) {
		text += run.s
	}
	if text != "Open social ×5   Enter runs it" {
		t.Errorf("drawn as %q", text)
	}
}

// A COUNT BESIDE A RESOLVER'S ANSWER IS REVIEWED ON THE LINE AS THE EXPLICIT REPEAT, and is not
// offered as a yes that would run it once.
func TestAProposedRepeatIsReviewedWithItsCount(t *testing.T) {
	line := "repeat open social 5 times"
	r := aSocialRepeat(line)
	asked := engineReads(t, map[string]lineReading{line: r})
	runs := runsRecorded(t)
	h := newModel()
	t.Cleanup(func() { promptAsk.Store(false); takePendingRoutine() })

	var p resolved
	p.Answer.Kind, p.Answer.ID, p.Answer.Slug, p.Answer.Name = "run", "open-social", "open-social", "open social"
	p.Answer.SourceAt = "v1"
	p.Times = 5
	if d := offerResolved(h, "open the social menu five times", p); d != offeredRun {
		t.Fatalf("the repeat was not offered: %v", d)
	}
	if promptAsk.Load() {
		t.Fatal("the repeat was offered as a yes, which runs the play once")
	}
	s := h.snapshot()
	if !s.editing || s.input != " "+line {
		t.Fatalf("the repeat is not open for review: editing %v, input %q (asked %q)", s.editing,
			s.input, asked())
	}
	if got := runs(); len(got) != 0 {
		t.Fatalf("a proposed repeat ran before anybody looked at it: %q", got)
	}
	time.Sleep(readDwell + 50*time.Millisecond)
	h.takeInput()
	h.setEditing(false)
	typedEnter(h, line)
	if got := runs(); len(got) != 1 || got[0] != "do --source=typed --play=open-social --expect=v1 --times=5" {
		t.Errorf("Enter on the reviewed repeat ran %q", got)
	}

	// AND ONE THAT CANNOT BE READ BACK IS NOT OFFERED AT ALL.
	h2 := newModel()
	engineReads(t, map[string]lineReading{})
	if d := offerResolved(h2, "open the social menu five times", p); d != saidWhyNot || promptAsk.Load() {
		t.Errorf("a repeat with no reading ended %v, y/n prompt %v", d, promptAsk.Load())
	}
}

// THE STATUS FOLLOWS THE REPEAT.
func TestARepeatsProgressIsTheStatus(t *testing.T) {
	if s, ok := directorStatusLine("[repeat 2/5] Open social"); !ok || s != "running 2 of 5: Open social" {
		t.Errorf("the progress line read as %q, %v", s, ok)
	}
}
