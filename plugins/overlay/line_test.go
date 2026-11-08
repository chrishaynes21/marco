package main

// S6 IN THE WIDGET: what Enter does with a typed line, entered through dispatch (the Overlay act
// the command line's Enter reaches) with the engine's reading stated and the child that would run
// recorded. Nothing here starts marco.exe.

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
)

// aCloudPull is the engine's reading of a line naming game-b's `cloud pull` with a value.
func aCloudPull(line, value string) lineReading {
	return lineReading{Line: line, Kind: "routine", ID: "game-b/context/cloud-pull", App: "game-b",
		Slug: "cloud-pull", Name: "cloud pull", Needs: true, Values: map[string]string{"resource": value},
		Parts: []linePart{{Label: "resource", Value: value}}, Readback: "Cloud pull · resource: " + value,
		Runnable: true, SourceAt: "v1"}
}

// engineReads states the engine's reading for each line, and records every ask and its proposal.
func engineReads(t *testing.T, readings map[string]lineReading) func() []string {
	t.Helper()
	var mu sync.Mutex
	var asked []string
	prev := askLine
	askLine = func(line string, p *lineProposal) (lineReading, bool) {
		mu.Lock()
		asked = append(asked, strings.TrimSpace(line+" "+strings.Join(p.flags(), " ")))
		mu.Unlock()
		r, ok := readings[line]
		return r, ok
	}
	t.Cleanup(func() { askLine = prev })
	return func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), asked...) }
}

// runsRecorded records every child the door would start, by its argv.
func runsRecorded(t *testing.T) func() []string {
	t.Helper()
	var mu sync.Mutex
	var runs []string
	restore := stubIntakeChild(t, func(h *model, name string, track bool, args ...string) childRun {
		mu.Lock()
		runs = append(runs, strings.Join(args, " "))
		mu.Unlock()
		return childRun{result: "performed"}
	})
	t.Cleanup(restore)
	return func() []string {
		time.Sleep(50 * time.Millisecond) // the run is started on a goroutine
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), runs...)
	}
}

// onScreen puts r under the line as the poller would, and lets it be read.
func onScreen(h *model, r lineReading) {
	h.setInput(r.Line)
	h.setEditing(true)
	h.setReading(r.Line, r)
	h.mu.Lock()
	h.readingAt = time.Now().Add(-2 * readDwell)
	h.mu.Unlock()
	h.setEditing(false) // Enter closed the line
	h.setInput("")
}

func typedEnter(h *model, line string) response {
	return dispatch(h, request{Action: "Run", Input: line})
}

// A read-back on screen runs as its IDENTITY, its VERSION and its values, and the words are not
// sent at all.
//
// Reading the words again must fail this.
func TestAReadBackRunsAsItsIdentityAndVersion(t *testing.T) {
	r := aCloudPull("cloud pull stone", "stone")
	engineReads(t, map[string]lineReading{r.Line: r})
	runs := runsRecorded(t)
	h := newModel()
	onScreen(h, r)
	typedEnter(h, r.Line)
	got := runs()
	want := "do --source=typed --play=cloud-pull --app=game-b --expect=v1 --arg=resource=stone"
	if len(got) != 1 || got[0] != want {
		t.Errorf("Enter on the read-back ran %q, want %q", got, want)
	}
}

// A VALUE NOBODY TYPED IS READ BACK FIRST: a line carrying what Director proposed runs nothing on
// an Enter that comes before its read-back was on screen, and the next Enter runs it. (A spoken
// one is voice_test.go's "a value, said".)
//
// Letting a proposed value through on the first Enter must fail this.
func TestAValueNobodyTypedIsReadBackFirst(t *testing.T) {
	r := aCloudPull("cloud pull resource: stone", "stone")
	r.Parts[0].Said = "(read out of what you said)"
	engineReads(t, map[string]lineReading{r.Line: r})
	runs := runsRecorded(t)
	h := newModel()
	h.setProposal(&lineProposal{app: "game-b", slug: "cloud-pull",
		values: map[string]string{"resource": "stone"}, at: "v1"})

	typedEnter(h, r.Line) // fast: the review was not on screen
	if got := runs(); len(got) != 0 {
		t.Fatalf("a proposed value ran before its read-back was seen: %q", got)
	}
	s := h.snapshot()
	if !s.editing || s.input != " "+r.Line || !s.hasReading || s.status != "Enter runs it" {
		t.Fatalf("after the first Enter: editing %v, input %q, reading %v, status %q",
			s.editing, s.input, s.hasReading, s.status)
	}
	time.Sleep(readDwell + 50*time.Millisecond)
	h.takeInput()
	h.setEditing(false)
	typedEnter(h, r.Line)
	if got := runs(); len(got) != 1 || !strings.Contains(got[0], "--arg=resource=stone") {
		t.Errorf("the Enter on the read-back ran %q", got)
	}
}

// A LINE TYPED IN FULL RUNS ON THE FIRST ENTER: `cloud pull stone`, typed fast, runs cloud pull
// with `stone`, as its identity and version, with no read-back to confirm first. A line with a
// value missing still runs nothing and stays open. ADR-333 replaced ADR-301's "nothing with a
// value runs unread" for a typed line.
//
// Making a typed line wait for its read-back must fail this.
func TestATypedLineRunsOnTheFirstEnter(t *testing.T) {
	r := aCloudPull("cloud pull stone", "stone")
	missing := lineReading{Line: "cloud pull", Kind: "routine", Slug: "cloud-pull", App: "game-b",
		Needs: true, Pending: "‹resource›", Says: "Resource needed · cloud pull ‹resource›"}
	engineReads(t, map[string]lineReading{r.Line: r, missing.Line: missing})
	runs := runsRecorded(t)
	h := newModel()

	typedEnter(h, r.Line) // typed fast: nothing was on screen
	want := "do --source=typed --play=cloud-pull --app=game-b --expect=v1 --arg=resource=stone"
	if got := runs(); len(got) != 1 || got[0] != want {
		t.Fatalf("the first Enter on a typed line ran %q, want %q", got, want)
	}
	if s := h.snapshot(); s.editing || s.status == "Enter runs it" {
		t.Errorf("a typed line that ran is still open asking for another Enter: %q", s.status)
	}

	typedEnter(h, missing.Line)
	time.Sleep(readDwell + 50*time.Millisecond)
	h.takeInput()
	h.setEditing(false)
	typedEnter(h, missing.Line) // and again: still missing
	if got := runs(); len(got) != 1 {
		t.Errorf("a line with its value missing ran: %q", got)
	}
	if s := h.snapshot(); !s.editing || s.input != " cloud pull" || s.status != missing.Says {
		t.Errorf("a missing value left editing %v, input %q, status %q", s.editing, s.input, s.status)
	}
}

// Every line this does not take goes to the intake as words, as before: a routine with no values,
// a line naming nothing, an engine with no `line`, a control word, and a spoken one the engine
// could not read.
func TestOtherLinesGoToTheIntakeAsBefore(t *testing.T) {
	exact := lineReading{Line: "cloud pull stone", Kind: "routine", Slug: "cloud-pull-stone", Runnable: true}
	asked := engineReads(t, map[string]lineReading{exact.Line: exact,
		"make coffee": {Line: "make coffee", Kind: "none"}})
	runs := runsRecorded(t)
	h := newModel()
	for _, line := range []string{"cloud pull stone", "make coffee", "an engine with no line"} {
		typedEnter(h, line)
	}
	dispatch(h, request{Action: "Run", Input: "stop"})
	voiceEnabled.Store(true)
	dispatch(h, request{Action: "RunVoice", Input: "cloud pull granite"})
	got := runs()
	for _, want := range []string{"do --source=typed cloud pull stone", "do --source=typed make coffee",
		"do --source=typed an engine with no line", "do --source=typed stop",
		"do --source=" + string(invoke.SourceSpoken) + " cloud pull granite"} {
		found := false
		for _, g := range got {
			found = found || g == want
		}
		if !found {
			t.Errorf("%q was not sent to the intake as words: %q", want, got)
		}
	}
	// A spoken phrase IS read now, like a typed one (voice_test.go); one the engine could not read
	// still goes as words. A control word never waits for a reading.
	for _, a := range asked() {
		if a == "stop" {
			t.Errorf("a control word waited for a reading: %q", a)
		}
	}
}

// What Director proposed is opened on the command line, editable, and runs nothing until Enter on
// its read-back; the reading is asked with the identity, the proposals and their version.
//
// Forwarding the proposals without the review must fail this.
func TestAProposedValueIsReviewedBeforeItRuns(t *testing.T) {
	proposed := aCloudPull("cloud pull resource: stone", "stone")
	proposed.Parts[0].Said = "(read out of what you said)"
	asked := engineReads(t, map[string]lineReading{proposed.Line: proposed})
	runs := runsRecorded(t)
	prev := askWhichRoutine
	askWhichRoutine = func(string) (resolved, string) {
		var r resolved
		r.Answer.Kind, r.Answer.ID, r.Answer.App, r.Answer.Slug, r.Answer.Name =
			"run", "game-b/context/cloud-pull", "game-b", "cloud-pull", "cloud pull"
		r.Answer.Values = append(r.Answer.Values, struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}{"resource", "stone"})
		r.Answer.SourceAt = "v1"
		return r, ""
	}
	t.Cleanup(func() { askWhichRoutine = prev })
	h := newModel()
	if d := offerWhatMarcoHas(h, "pull some stone down from the cloud"); d != offeredRun {
		t.Fatalf("the proposal was not offered: %v", d)
	}
	if got := runs(); len(got) != 0 {
		t.Fatalf("a proposal ran before anybody looked at it: %q", got)
	}
	s := h.snapshot()
	if !s.editing || s.input != " "+proposed.Line || promptAsk.Load() {
		t.Fatalf("the proposal is not open for review: editing %v, input %q, y/n prompt %v",
			s.editing, s.input, promptAsk.Load())
	}
	if a := asked(); len(a) == 0 || a[0] != "cloud pull resource: stone --play=cloud-pull --app=game-b "+
		"--proposed=resource=stone --proposed-at=v1" {
		t.Errorf("the reading was asked as %q", a)
	}
	// Enter on it, once it has been read.
	time.Sleep(readDwell + 50*time.Millisecond)
	h.takeInput()
	h.setEditing(false)
	typedEnter(h, proposed.Line)
	if got := runs(); len(got) != 1 || got[0] != "do --source=typed --play=cloud-pull --app=game-b --expect=v1 --arg=resource=stone" {
		t.Errorf("Enter on the reviewed proposal ran %q", got)
	}
	if h.lineProposal() != nil {
		t.Error("the proposal outlived the run it was for")
	}
}

// A routine Director named that takes no values is still the plain offer.
func TestARoutineWithNoValuesIsStillOffered(t *testing.T) {
	engineReads(t, map[string]lineReading{"open settings": {Line: "open settings", Kind: "routine",
		Slug: "open-settings", Runnable: true}})
	prev := askWhichRoutine
	askWhichRoutine = func(string) (resolved, string) {
		var r resolved
		r.Answer.Kind, r.Answer.ID, r.Answer.Slug, r.Answer.Name = "run", "open-settings", "open-settings", "open settings"
		return r, ""
	}
	t.Cleanup(func() { askWhichRoutine = prev; promptAsk.Store(false); takePendingRoutine() })
	h := newModel()
	offerWhatMarcoHas(h, "the settings please")
	if !promptAsk.Load() || h.snapshot().editing {
		t.Error("a routine with no values was not offered as before")
	}
}

// The reading child is asked with the line after `--` and its answer is read; an answer for
// another line is not taken.
func TestTheLineIsAskedOfTheEngine(t *testing.T) {
	answer := `{"line":"cloud pull stone","kind":"routine","slug":"cloud-pull","app":"game-b","needs":true,` +
		`"values":{"resource":"stone"},"runnable":true,"source_at":"v1","readback":"Cloud pull · resource: stone"}`
	argv := marcoAnswers(t, answer, 0)
	r, ok := readLineChild("cloud pull stone", &lineProposal{app: "game-b", slug: "cloud-pull",
		values: map[string]string{"resource": "stone"}, at: "v1"})
	if !ok || !r.Runnable || r.Values["resource"] != "stone" || r.SourceAt != "v1" {
		t.Fatalf("read as %+v %v", r, ok)
	}
	if got := argv(); got != "line --json --play=cloud-pull --app=game-b --proposed=resource=stone "+
		"--proposed-at=v1 -- cloud pull stone" {
		t.Errorf("asked as %q", got)
	}
	if _, ok := readLineChild("something else", nil); ok {
		t.Error("an answer about another line was taken")
	}
}

// The poller's reading is shown only for the line it was read for.
func TestAReadingIsShownOnlyForItsLine(t *testing.T) {
	r := aCloudPull("cloud pull stone", "stone")
	engineReads(t, map[string]lineReading{r.Line: r})
	h := newModel()
	h.setEditing(true)
	h.setInput("cloud pull ston")
	refreshReading(h, "cloud pull stone") // arrived after the line moved on
	if h.snapshot().hasReading {
		t.Error("a reading for another line was shown")
	}
	h.setInput("cloud pull stone")
	refreshReading(h, "cloud pull stone")
	if s := h.snapshot(); !s.hasReading || s.reading.Readback != r.Readback {
		t.Errorf("the reading for the line typed was not shown: %+v", s.reading)
	}
	h.setInput("cloud pull stones")
	if h.snapshot().hasReading {
		t.Error("the reading stayed under a line it was not for")
	}
}

// A late reading for what the line used to be does not replace the reading of what it is.
func TestALateReadingDoesNotReplaceTheCurrentOne(t *testing.T) {
	now, old := aCloudPull("cloud pull stone", "stone"), aCloudPull("cloud pull ston", "ston")
	engineReads(t, map[string]lineReading{now.Line: now, old.Line: old})
	h := newModel()
	h.setEditing(true)
	h.setInput(now.Line)
	refreshReading(h, now.Line)
	refreshReading(h, old.Line) // asked before the last key, answered after
	if s := h.snapshot(); !s.hasReading || s.reading.Readback != now.Readback {
		t.Errorf("the line's reading is %+v", s.reading)
	}
}

// A review opens only on the routine Director named: if the line reads as another one (a namesake
// in a nearer scope), it is the plain offer of the named routine and no proposal is attached.
func TestAReviewIsOnlyOfTheNamedRoutine(t *testing.T) {
	namesake := aCloudPull("cloud pull resource: stone", "stone")
	namesake.Slug, namesake.App = "cloud-pull", "notepad"
	engineReads(t, map[string]lineReading{namesake.Line: namesake})
	h := newModel()
	t.Cleanup(func() { promptAsk.Store(false); takePendingRoutine() })
	if got := reviewProposal(h, "cloud pull", "game-b", "cloud-pull", false, map[string]string{"resource": "stone"}, "v1"); got != refusedReview {
		t.Errorf("a review on a routine Director did not name came to %v", got)
	}
	if h.lineProposal() != nil || h.snapshot().editing {
		t.Error("the proposal was attached to another routine's line")
	}
}

// THE TWO NAMESAKES: game-b's context `cloud pull` and its focus `cloud pull` share the
// application and the name. Director proposes a value for the FOCUS one, and the line reads as
// the context one. Nothing is put on the line, nothing is offered or run, and the HUD says why.
//
// Comparing the application and the name alone must fail this.
func TestAProposalForTheFocusRoutineIsNotPutOnItsNamesake(t *testing.T) {
	context := aCloudPull("cloud pull resource: stone", "stone") // Focus false: the context one
	engineReads(t, map[string]lineReading{context.Line: context})
	runs := runsRecorded(t)
	prev := askWhichRoutine
	askWhichRoutine = func(string) (resolved, string) {
		var r resolved
		r.Answer.Kind, r.Answer.ID, r.Answer.App, r.Answer.Slug, r.Answer.Name, r.Answer.Focus =
			"run", "game-b/focus/cloud-pull", "game-b", "cloud-pull", "cloud pull", true
		r.Answer.Values = append(r.Answer.Values, struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}{"resource", "stone"})
		r.Answer.SourceAt = "v1"
		return r, ""
	}
	t.Cleanup(func() { askWhichRoutine = prev; promptAsk.Store(false); takePendingRoutine() })
	h := newModel()
	if d := offerWhatMarcoHas(h, "pull some stone down from the cloud"); d != saidWhyNot {
		t.Errorf("the namesake came to %v", d)
	}
	s := h.snapshot()
	if s.editing || promptAsk.Load() || h.lineProposal() != nil {
		t.Errorf("something was offered: editing %v, prompt %v, proposal %+v", s.editing, promptAsk.Load(), h.lineProposal())
	}
	if got := runs(); len(got) != 0 {
		t.Errorf("something ran: %q", got)
	}
	if len(s.logs) == 0 || !strings.Contains(s.logs[len(s.logs)-1], "cloud-pull (from anywhere → game-b)") {
		t.Errorf("the HUD did not say why: %q", s.logs)
	}
}

// While Director's values are on the line, only their routine runs from it: edited to name the
// namesake, Enter runs nothing until the line is dismissed.
func TestALineEditedAwayFromItsProposalRunsNothing(t *testing.T) {
	context := aCloudPull("cloud pull resource: stone", "stone")
	engineReads(t, map[string]lineReading{context.Line: context})
	runs := runsRecorded(t)
	h := newModel()
	h.setProposal(&lineProposal{app: "game-b", slug: "cloud-pull", focus: true,
		values: map[string]string{"resource": "stone"}, at: "v1"})
	onScreen(h, context)
	typedEnter(h, context.Line)
	if got := runs(); len(got) != 0 {
		t.Fatalf("a line naming another routine ran with the proposal on it: %q", got)
	}
	if s := h.snapshot(); !s.editing || !strings.Contains(s.status, "Esc clears it") {
		t.Errorf("after Enter: editing %v, status %q", s.editing, s.status)
	}
	h.dismiss()
	if h.lineProposal() != nil {
		t.Error("dismissing the line kept the proposal")
	}
}

// A VALUE SOMEBODY TYPED NEVER REACHES THE LOG. The child cannot start (there is no such marco),
// so nothing announces a route and the tracked name is what the history, the status and the
// console log say. A unique value is in none of them.
//
// Tracking the read-back must fail this.
func TestASuppliedValueNeverReachesTheLog(t *testing.T) {
	const secretish = "zq7-value-9f3k"
	r := aCloudPull("cloud pull "+secretish, secretish)
	engineReads(t, map[string]lineReading{r.Line: r})
	t.Setenv("MARCO_BIN", t.TempDir()+`\no-such-marco.exe`)
	var logged strings.Builder
	var mu sync.Mutex
	prevW, prevLvl := mlogW, mlogLvl
	mlogInit()
	mlogW, mlogLvl = lockedWriter{&mu, &logged}, mlogDebug
	t.Cleanup(func() { mlogW, mlogLvl = prevW, prevLvl })
	done := make(chan childRun, 1)
	prev := spawnIntakeChild
	spawnIntakeChild = func(h *model, name string, track bool, args ...string) childRun {
		c := runRecordTracked(h, name, track, args...)
		done <- c
		return c
	}
	t.Cleanup(func() { spawnIntakeChild = prev })
	h := newModel()
	onScreen(h, r)
	typedEnter(h, r.Line)
	var c childRun
	select {
	case c = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the run never ended")
	}
	if c.err == nil {
		t.Fatal("the child started; this test needs one that cannot")
	}
	mu.Lock()
	out := logged.String()
	mu.Unlock()
	s := h.snapshot()
	if strings.Contains(out, secretish) {
		t.Errorf("the value reached the log:\n%s", out)
	}
	for _, e := range s.history {
		if strings.Contains(e.cmd, secretish) {
			t.Errorf("the value is in the history: %q", e.cmd)
		}
	}
	if strings.Contains(s.status, secretish) {
		t.Errorf("the value is in the status after the run: %q", s.status)
	}
	if !strings.Contains(out, "Cloud pull") {
		t.Errorf("the run was not logged under its routine's name:\n%s", out)
	}
}

type lockedWriter struct {
	mu *sync.Mutex
	b  *strings.Builder
}

func (w lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}
