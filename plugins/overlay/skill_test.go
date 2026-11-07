package main

// BUILT-IN DESKTOP SKILLS IN THE WIDGET (cmd/marco skill.go): read back like a routine with a
// value, run by their identity, recalled by their name, and a skill Director named is put on the
// command line for review.

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// aFocus is the engine's reading of "focus Discord".
func aFocus(line string) lineReading {
	return lineReading{Line: line, Kind: "skill", ID: "builtin/focus-app", Slug: "focus-app",
		Name: "Focus app", Needs: true, Values: map[string]string{"app": "Discord"},
		Parts: []linePart{{Label: "app", Value: "Discord"}}, Readback: "Focus app · app: Discord",
		Runnable: true}
}

// A SKILL RUNS ONLY FROM ITS READ-BACK, typed or spoken, and as its identity: never as words and
// never as a routine. Treating a skill as an ordinary line must fail this.
func TestASkillRunsOnlyFromItsReadBack(t *testing.T) {
	r := aFocus("focus Discord")
	engineReads(t, map[string]lineReading{r.Line: r})
	runs := runsRecorded(t)
	h := newModel()

	dispatch(h, request{Action: "RunVoice", Input: r.Line}) // heard, and not on screen yet
	if got := runs(); len(got) != 0 {
		t.Fatalf("a skill nobody had seen read back ran: %q", got)
	}
	if got := h.recalled(); !reflect.DeepEqual(got, []string{"Focus app"}) {
		t.Errorf("a spoken skill was not kept by its name: %q", got)
	}
	if s := h.snapshot(); !s.editing || s.input != " "+r.Line {
		t.Fatalf("the skill is not open for review: editing %v, input %q", s.editing, s.input)
	}
	onScreen(h, r)
	typedEnter(h, r.Line)
	want := "do --source=typed --skill=focus-app --arg=app=Discord"
	if got := runs(); len(got) != 1 || got[0] != want {
		t.Errorf("Enter on the read-back ran %q, want %q", got, want)
	}
	// AND IT IS RECALLED BY ITS NAME, whose reading asks for the app again.
	if got := h.recalled(); !reflect.DeepEqual(got, []string{"Focus app"}) {
		t.Errorf("kept %q", got)
	}
}

// A SKILL DIRECTOR NAMED IS REVIEWED ON THE COMMAND LINE AS ITS EXPLICIT LINE, and nothing runs
// until Enter on that read-back. One the line reads as something else is not put there at all.
func TestASkillDirectorNamedIsReviewedBeforeItRuns(t *testing.T) {
	r := aFocus("focus Discord")
	engineReads(t, map[string]lineReading{r.Line: r,
		"focus Steam": {Line: "focus Steam", Kind: "routine", Slug: "focus", Name: "focus", Needs: true}})
	runs := runsRecorded(t)
	promptAsk.Store(false) // an earlier test can leave a y/n up
	takePendingRoutine()
	h := newModel()
	t.Cleanup(func() { promptAsk.Store(false); takePendingRoutine() })

	var p resolved
	p.Answer.Kind, p.Answer.ID, p.Answer.Slug, p.Answer.Name = "run", "builtin/focus-app", "focus-app", "Focus app"
	p.Answer.Builtin, p.Answer.Line = true, "focus Discord"
	if d := offerResolved(h, "switch me back to discord", p); d != offeredRun {
		t.Fatalf("the skill was not offered: %v", d)
	}
	if promptAsk.Load() {
		t.Fatal("the skill was offered as a yes")
	}
	if s := h.snapshot(); !s.editing || s.input != " focus Discord" {
		t.Fatalf("the skill is not open for review: editing %v, input %q", s.editing, s.input)
	}
	if got := runs(); len(got) != 0 {
		t.Fatalf("a skill Director named ran before anybody looked at it: %q", got)
	}
	time.Sleep(readDwell + 50*time.Millisecond)
	h.takeInput()
	h.setEditing(false)
	typedEnter(h, "focus Discord")
	if got := runs(); len(got) != 1 || got[0] != "do --source=typed --skill=focus-app --arg=app=Discord" {
		t.Errorf("Enter on the reviewed skill ran %q", got)
	}

	for _, c := range []struct {
		line  string
		times int
	}{{"focus Steam", 0}, {"focus Discord", 3}, {"", 0}, {"focus nothing-reads-this", 0}} {
		h := newModel()
		p.Answer.Line, p.Times = c.line, c.times
		if d := offerResolved(h, "switch me back", p); d != saidWhyNot || h.snapshot().editing {
			t.Errorf("%q ×%d ended %v, line open %v", c.line, c.times, d, h.snapshot().editing)
		}
	}
	if got := runs(); len(got) != 1 {
		t.Errorf("a refused review ran: %q", got)
	}
}

// A READ-BACK STILL FINDING ITS PROGRAM SAYS SO AND RUNS NOTHING, and is replaced by the one that
// found it, which names the program Enter will bring forward.
func TestAReadBackShowsTheProgramOnceItIsFound(t *testing.T) {
	const line = "focus chat gp jpg"
	finding := lineReading{Line: line, Kind: "skill", Slug: "focus-app", Name: "Focus app", Needs: true,
		Resolving: true, Says: `finding which open program "chat gp jpg" is…`}
	found := lineReading{Line: line, Kind: "skill", Slug: "focus-app", Name: "Focus app", Needs: true,
		Values: map[string]string{"app": "chat gp jpg", "window": "15:7"}, Runnable: true,
		Parts: []linePart{{Label: "app", Value: "chat gp jpg"},
			{Label: "program", Value: "ChatGPT", Said: "(worked out by Marco's model)"}}}
	engineReads(t, map[string]lineReading{line: finding})
	release := make(chan struct{})
	prev := askResolved
	askResolved = func(l string) (lineReading, bool) { <-release; return found, l == line }
	t.Cleanup(func() { askResolved = prev })
	runs := runsRecorded(t)
	h := newModel()
	h.setEditing(true)
	h.setInput(" " + line)

	refreshReading(h, line)
	drawn := func() string {
		s := ""
		for _, r := range readingRuns(h.snapshot()) {
			s += r.s
		}
		return s
	}
	if got := drawn(); got != finding.Says {
		t.Errorf("while finding, drawn as %q", got)
	}
	h.takeInput()
	h.setEditing(false)
	typedEnter(h, line)
	if got := runs(); len(got) != 0 {
		t.Fatalf("Enter ran a skill whose program was not found yet: %q", got)
	}
	close(release)
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(drawn(), "program: ChatGPT") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := drawn(); !strings.Contains(got, "program: ChatGPT (worked out by Marco's model)") {
		t.Fatalf("once found, drawn as %q", got)
	}
	time.Sleep(readDwell + 50*time.Millisecond)
	h.takeInput()
	h.setEditing(false)
	typedEnter(h, line)
	if got := runs(); len(got) != 1 || !strings.Contains(got[0], "--arg=window=15:7") {
		t.Errorf("Enter on the found program ran %q", got)
	}
}
