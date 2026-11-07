package main

// BUILT-IN DESKTOP SKILLS IN THE WIDGET (cmd/marco skill.go): read back like a routine with a
// value, run by their identity, recalled by their name, and a skill Director named is put on the
// command line for review.

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// aFocus is the engine's reading of "focus Discord".
func aFocus(line string) lineReading {
	return lineReading{Line: line, Kind: "skill", ID: "builtin/focus-app", Slug: "focus-app",
		Name: "Focus app", Needs: true, Values: map[string]string{"app": "Discord"},
		Parts: []linePart{{Label: "program", Value: "Discord"}}, Readback: "Focus app · program: Discord",
		Runnable: true}
}

// SEARCH RUNS ONLY FROM ITS READ-BACK, typed or spoken, and as its identity: never as words and
// never as a routine. Treating a skill as an ordinary line must fail this.
func TestASkillRunsOnlyFromItsReadBack(t *testing.T) {
	r := aSearch("search the web for ark wiki")
	engineReads(t, map[string]lineReading{r.Line: r})
	runs := runsRecorded(t)
	h := newModel()

	dispatch(h, request{Action: "RunVoice", Input: r.Line}) // heard, and not on screen yet
	if got := runs(); len(got) != 0 {
		t.Fatalf("a skill nobody had seen read back ran: %q", got)
	}
	if got := h.recalled(); !reflect.DeepEqual(got, []string{"Search the web"}) {
		t.Errorf("a spoken skill was not kept by its name: %q", got)
	}
	if s := h.snapshot(); !s.editing || s.input != " "+r.Line {
		t.Fatalf("the skill is not open for review: editing %v, input %q", s.editing, s.input)
	}
	onScreen(h, r)
	typedEnter(h, r.Line)
	want := "do --source=typed --skill=search-the-web --arg=query=ark wiki"
	if got := runs(); len(got) != 1 || got[0] != want {
		t.Errorf("Enter on the read-back ran %q, want %q", got, want)
	}
	// AND IT IS RECALLED BY ITS NAME, whose reading asks for the query again.
	if got := h.recalled(); !reflect.DeepEqual(got, []string{"Search the web"}) {
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
		Resolving: true, Says: "finding the program…"}
	found := lineReading{Line: line, Kind: "skill", Slug: "focus-app", Name: "Focus app", Needs: true,
		Values: map[string]string{"app": "chat gp jpg", "window": "15:7"}, Runnable: true,
		Parts: []linePart{{Label: "program", Value: "ChatGPT"}}}
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
	close(release)
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(drawn(), "program: ChatGPT") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := drawn(); !strings.Contains(got, "Focus app · program: ChatGPT") {
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

// aSearch is the engine's reading of a search.
func aSearch(line string) lineReading {
	return lineReading{Line: line, Kind: "skill", ID: "builtin/search-the-web", Slug: "search-the-web",
		Name: "Search the web", Needs: true, Values: map[string]string{"query": "ark wiki"},
		Parts:    []linePart{{Label: "query", Value: "ark wiki"}, {Label: "provider", Value: "google"}},
		Runnable: true}
}

// FOCUS RUNS ON THE FIRST ENTER, typed or spoken: with the window it found, or, while it is still
// finding it, with the words for the run to work out. One that cannot run stays open.
func TestFocusRunsOnTheFirstEnter(t *testing.T) {
	found := aFocus("focus discord")
	found.Values = map[string]string{"app": "discord", "window": "12:3"}
	finding := lineReading{Line: "focus chatpgt", Kind: "skill", Slug: "focus-app", Name: "Focus app",
		Needs: true, Resolving: true, Values: map[string]string{"app": "chatpgt"}, Says: "finding the program…"}
	missing := lineReading{Line: "focus", Kind: "skill", Slug: "focus-app", Name: "Focus app",
		Needs: true, Says: "App needed · Focus app ‹app›"}
	engineReads(t, map[string]lineReading{found.Line: found, finding.Line: finding, missing.Line: missing})
	runs := runsRecorded(t)
	h := newModel()

	typedEnter(h, found.Line) // not on screen yet, and runs anyway
	dispatch(h, request{Action: "RunVoice", Input: finding.Line})
	got := runs()
	sort.Strings(got) // two runs, started apart and finishing in either order
	if len(got) != 2 || !strings.Contains(got[1], "--skill=focus-app") || !strings.Contains(got[1], "--arg=window=12:3") ||
		!strings.Contains(got[0], "--arg=app=chatpgt") || strings.Contains(got[0], "window") {
		t.Fatalf("ran %q", got)
	}
	typedEnter(h, missing.Line)
	if got := runs(); len(got) != 2 {
		t.Errorf("a focus with no app ran: %q", got)
	}
	if s := h.snapshot(); !s.editing {
		t.Error("a focus with no app did not stay open to say so")
	}
}

// FOCUS IS RECALLED WITH THE PROGRAM IT WENT TO, "focus ChatGPT", never what was heard: from the
// read-back when it had found the program, and from the run when it was still finding it.
func TestFocusIsRecalledWithTheProgramItWentTo(t *testing.T) {
	found := aFocus("focus discord")
	found.Parts = []linePart{{Label: "program", Value: "Discord"}}
	finding := lineReading{Line: "focus church tpg", Kind: "skill", Slug: "focus-app", Name: "Focus app",
		Needs: true, Resolving: true, Values: map[string]string{"app": "church tpg"}}
	engineReads(t, map[string]lineReading{found.Line: found, finding.Line: finding})
	done := make(chan struct{}, 2)
	restore := stubIntakeChild(t, func(_ *model, _ string, _ bool, args ...string) childRun {
		defer func() { done <- struct{}{} }()
		if strings.Contains(strings.Join(args, " "), "church tpg") {
			return childRun{result: "performed", ran: "Focus app · program: ChatGPT"}
		}
		// NO PROGRAM ON THE RUN'S LINE: what is kept comes from the read-back.
		return childRun{result: "performed"}
	})
	t.Cleanup(restore)
	h := newModel()
	typedEnter(h, found.Line)
	<-done
	dispatch(h, request{Action: "RunVoice", Input: finding.Line})
	<-done
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !reflect.DeepEqual(h.recalled(), []string{"focus Discord", "focus ChatGPT"}) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := h.recalled(); !reflect.DeepEqual(got, []string{"focus Discord", "focus ChatGPT"}) {
		t.Errorf("kept %q", got)
	}
}
