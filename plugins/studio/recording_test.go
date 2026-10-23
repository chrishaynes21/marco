package main

// THE FOUR BUTTONS, THROUGH STUDIO'S OWN PATH.
//
// Everything here enters through `newActs` and the methods the page is bound to. The only thing
// substituted is the input recorder, and that is substituted for the reason `raising` and
// `listing` are: the real one installs low-level keyboard and mouse hooks and needs a person's
// hands on a real desktop. The claim this file exists for — that pressing Stop in this window
// does not become a step in somebody's macro — cannot otherwise be made to happen at all.
//
// What is NOT substituted is the part that was wrong four times: Studio's own process id and its
// own app name are whatever production computes, and the fixture's stopping click carries
// `os.Getpid()`. So a Studio that passed the wrong owner, or none, fails here.
//
// See [[ADR-249-ending-a-recording-is-not-part-of-the-recording]].

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// heldRecorder hands back a fixed stream, exactly as the orchestrator's own tests do.
//
// `Stop` returns the events whatever ended the capture, which is what a real recorder does: it
// hands over what it saw, and the backend decides what of that was the person demonstrating.
type heldRecorder struct {
	events  []recorder.RecordedEvent
	ch      chan recorder.RecordedEvent
	started bool
}

func (h *heldRecorder) Start() error {
	h.started = true
	h.ch = make(chan recorder.RecordedEvent)
	return nil
}
func (h *heldRecorder) Stop() []recorder.RecordedEvent        { return h.events }
func (h *heldRecorder) Events() <-chan recorder.RecordedEvent { return h.ch }

// at is a moment, in milliseconds from the start of the recording.
func at(ms int) time.Time {
	return time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC).
		Add(time.Duration(ms) * time.Millisecond)
}

// gamePID stands for the application being demonstrated in. Studio's own process id is NOT a
// constant here: it is `os.Getpid()`, because the thing under test is whether Studio tells the
// recorder who it is.
const gamePID uint32 = 4812

// anUnshieldedGlanceAtStudio is the acceptance case, with Studio brought forward to press Stop.
//
// Clicks only — no keystroke anywhere shields a click from `simplify.foldActivates`, which reads
// the step immediately before an app switch as the gesture that performed it. The person clicks
// in the game, looks at Studio mid-demonstration, goes back and clicks again, then brings Studio
// forward and presses the Stop button.
//
// The poller samples both sides of the glance, which is what a visit long enough to read
// something looks like. That makes the return a switch naming the window that never left the
// front once Studio's own switch is excluded — the case that cost the first click.
func anUnshieldedGlanceAtStudio(owner uint32, ownerApp string) []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: ownerApp, T: at(0)},
		// ── OFF TO THE GAME, AND THE FIRST CLICK ──
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(400)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(900)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(905)},
		// ── A LOOK AT STUDIO, AND THE RETURN, BOTH SAMPLED ──
		{Kind: recorder.EvAppSwitch, KeyName: ownerApp, T: at(2000)},
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(2600)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 496, Y: 283,
			App: "arkascended", PID: gamePID, T: at(3000)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 496, Y: 283,
			App: "arkascended", PID: gamePID, T: at(3005)},
		// ── STUDIO FORWARD, AND THE STOP BUTTON PRESSED IN IT ──
		//
		// Both edges carry Studio's real process id, as the production builder
		// stamps them. See internal/recorder/attribution_windows_test.go.
		{Kind: recorder.EvAppSwitch, KeyName: ownerApp, T: at(4000)},
		{Kind: recorder.EvMove, X: 1500, Y: 900, T: at(4100)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: ownerApp, PID: owner, T: at(4200)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: ownerApp, PID: owner, T: at(4205)},
	}
}

// aStudioRecording runs one capture through `StartRecording` and `StopRecording`, and hands back
// the surface and the routes directory.
//
// The events are built from what PRODUCTION will claim as its own — `os.Getpid` and
// `winctx.Self` — so the fixture poses the real question rather than agreeing with a constant.
func aStudioRecording(t *testing.T, name string) (*Acts, string) {
	t.Helper()
	t.Setenv("MARCO_CV", "off")
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	a := newActs(t.TempDir())
	owner, ownerApp := uint32(os.Getpid()), winctx.Self()
	if ownerApp == "" {
		t.Fatal("Studio cannot name its own application, so it cannot tell the " +
			"recorder which window to leave out")
	}
	held := &heldRecorder{events: anUnshieldedGlanceAtStudio(owner, ownerApp)}
	a.recording = func() recorder.Recorder { return held }

	if why := a.StartRecording(name); why != "" {
		t.Fatalf("StartRecording refused: %s", why)
	}
	if !held.started {
		t.Fatal("StartRecording never started the recorder")
	}
	if why := a.StopRecording(); why != "" {
		t.Fatalf("StopRecording: %s", why)
	}
	return a, dir
}

// pressesIn is what a recording would click, as the review lists it.
func pressesIn(r Recording) []string {
	var out []string
	for _, s := range r.Steps {
		if strings.Contains(s, "click") {
			out = append(out, s)
		}
	}
	return out
}

// STOP ENDS THE CAPTURE, OPENS REVIEW, AND WRITES NOTHING.
//
// The first claim, because the other three rest on it: if Stop had written the routine there
// would be nothing to cancel and no decision left to make about scope.
func TestStopOpensReviewAndWritesNothing(t *testing.T) {
	a, dir := aStudioRecording(t, "ark graphics")

	r := a.Recording()
	if r.Active {
		t.Error("the capture is still running after Stop")
	}
	if !r.Review {
		t.Fatalf("Stop did not open review: %+v", r)
	}
	if r.Failed {
		t.Fatalf("the recording failed: %s", r.Said)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("Stop wrote %d entr(ies) into the routes directory; Stop is for "+
			"looking at what was recorded and Save is what keeps it: %v",
			len(entries), entries)
	}
	// AND THE OTHER WAY OUT IS NAMED, because the button is not the only one and a
	// person who learns the shortcut should learn it from the window.
	if r.StopKey == "" {
		t.Error("review does not say which key also ends a recording")
	}
}

// AND THE CLICK THAT PRESSED STOP IS NOT IN THE MACRO, NOR IS THE GLANCE.
//
// The Architect's test, through Studio's own spawn: Studio is brought forward and the button is
// pressed in it, with no keystroke anywhere to shield the clicks beside the switches.
//
// Three things have to be true at once, and each was wrong at some point:
//
//	the stopping click is not a step     it was, until the tail came off
//	both game clicks are steps           the first went with the sampled return
//	the app is the game                  it was Studio, from the switch that pressed Stop
func TestPressingStopInStudioDoesNotBecomeAStep(t *testing.T) {
	a, _ := aStudioRecording(t, "ark graphics")
	r := a.Recording()

	got := pressesIn(r)
	if len(got) != 2 {
		t.Fatalf("review lists %d click(s): %v\nall steps: %v. Nothing shields "+
			"these clicks from the rule that reads the step before an app "+
			"switch as the gesture that performed it", len(got), got, r.Steps)
	}
	for _, want := range []string{"842", "496"} {
		found := false
		for _, s := range got {
			if strings.Contains(s, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("the click demonstrated at %s is not in the review: %v",
				want, got)
		}
	}
	// AND STUDIO CONTRIBUTES NO ACTION AT ALL — not the press of Stop, and not an
	// Activate bringing Studio forward. Both named, because an Activate is an action
	// too: a routine that focuses the recorder on its way through has lost its game.
	for _, s := range r.Steps {
		if strings.Contains(s, "1511") {
			t.Errorf("the click that pressed Stop is a step: %q", s)
		}
		if strings.Contains(s, winctx.Self()) {
			t.Errorf("a step names Studio itself: %q", s)
		}
	}
	if r.Application != "arkascended" {
		t.Errorf("the recording's application is %q; it was demonstrated in "+
			"arkascended and Studio was only looked at and then pressed",
			r.Application)
	}
	if r.StoppedBy != "the Stop button" {
		t.Errorf("review says it ended by %q, and the button is what was pressed",
			r.StoppedBy)
	}
}

// AND CANCEL LEAVES NOTHING BEHIND.
//
// Asserted as the absence it is, and then as the state: there is nothing on disk, and the window
// is ready to record again rather than stuck holding a draft nobody wants.
func TestCancelDiscardsTheDraft(t *testing.T) {
	a, dir := aStudioRecording(t, "ark graphics")
	if why := a.CancelRecording(); why != "" {
		t.Fatalf("Cancel: %s", why)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("a cancelled recording left %d entr(ies) behind", len(entries))
	}
	if r := a.Recording(); r.Review || r.Active || r.Name != "" {
		t.Errorf("after Cancel the window still has a recording: %+v", r)
	}
	// AND THE WINDOW WILL RECORD AGAIN. A Cancel that left the slot claimed would be a
	// Studio that can record exactly once.
	a.recording = func() recorder.Recorder {
		return &heldRecorder{events: anUnshieldedGlanceAtStudio(
			uint32(os.Getpid()), winctx.Self())}
	}
	if why := a.StartRecording("ark graphics again"); why != "" {
		t.Errorf("a second recording was refused after a cancelled one: %s", why)
	}
	_ = a.StopRecording()
}

// AND SAVE PERSISTS IT, UNDER THE SCOPE IT IS GIVEN.
//
// Each scope files the routine somewhere different and only one of them keeps the window switch.
// The window asks the question, so it has to be able to answer it rather than hope.
func TestSaveKeepsTheRecordingUnderTheStatedScope(t *testing.T) {
	for _, c := range []struct {
		scope string
		under string
	}{
		{"app", "arkascended"},
		{"app-anywhere", "arkascended"},
		{"everywhere", ""},
	} {
		t.Run(c.scope, func(t *testing.T) {
			a, dir := aStudioRecording(t, "ark graphics")
			if why := a.SaveDraft(c.scope, ""); why != "" {
				t.Fatalf("saving as %s: %s", c.scope, why)
			}
			r := a.Recording()
			if r.Saved == "" {
				t.Fatal("Save did not say where the routine went")
			}
			if r.Review {
				t.Error("review is still open over a routine already saved")
			}
			raw, err := os.ReadFile(r.Saved)
			if err != nil {
				t.Fatalf("the routine is not on disk: %v", err)
			}
			src := string(raw)
			for _, want := range []string{"842", "274", "496", "283"} {
				if !strings.Contains(src, want) {
					t.Errorf("the saved routine is missing %s:\n%s",
						want, src)
				}
			}
			if strings.Contains(src, "1511") {
				t.Errorf("the saved routine presses Stop:\n%s", src)
			}
			if strings.Contains(strings.ToLower(src), winctx.Self()) {
				t.Errorf("the saved routine names Studio:\n%s", src)
			}
			// AND IT IS FILED WHERE THE SCOPE SAYS — under the application,
			// or not under one at all. That directory is what decides
			// whether the routine is ever offered, which is the whole of
			// what a stated scope is for.
			rel, err := filepath.Rel(dir, r.Saved)
			if err != nil {
				t.Fatal(err)
			}
			parts := strings.Split(filepath.ToSlash(rel), "/")
			under := parts[0]
			if c.under == "" {
				if strings.EqualFold(under, "arkascended") {
					t.Errorf("a routine saved for everywhere is "+
						"filed under the application (%q), so "+
						"it is offered where it was recorded "+
						"and nowhere else", rel)
				}
			} else if !strings.EqualFold(under, c.under) {
				t.Errorf("the routine is at %q, and an %s scope files it "+
					"under %q", rel, c.scope, c.under)
			}
		})
	}
}

// AND THE WINDOW RECORDS AGAIN AFTER A SAVE, WITHOUT BEING CANCELLED.
//
// Found by reading rather than by a failing test, which is worth saying: every test above ends
// its recording and none of them started a second one after KEEPING the first. The guard that
// stops a second capture over an unsaved draft was asking only whether a draft existed — so a
// Studio that had saved one routine refused to record another for the rest of its life, unless
// somebody pressed Cancel over a routine they had just kept.
//
// And saving twice is refused rather than written again, because the second Save would use
// whichever scope happened to be selected then: a routine rescoped by a double-click.
func TestTheWindowRecordsAgainAfterSaving(t *testing.T) {
	a, _ := aStudioRecording(t, "ark graphics")
	if why := a.SaveDraft("app", ""); why != "" {
		t.Fatalf("saving: %s", why)
	}
	if why := a.SaveDraft("everywhere", ""); why == "" {
		t.Error("the same recording was saved twice, the second time under a " +
			"different scope")
	}
	a.recording = func() recorder.Recorder {
		return &heldRecorder{events: anUnshieldedGlanceAtStudio(
			uint32(os.Getpid()), winctx.Self())}
	}
	if why := a.StartRecording("ark audio"); why != "" {
		t.Fatalf("a second recording was refused after the first was SAVED, so "+
			"this Studio records once per launch: %s", why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatal(why)
	}
	if r := a.Recording(); !r.Review || r.Name != "ark audio" {
		t.Errorf("the second recording did not reach review: %+v", r)
	}
}

// AND A SCOPE NOBODY RECOGNISED IS REFUSED RATHER THAN DEFAULTED.
//
// A routine saved somewhere nobody chose works in places nobody was told about. Refusing is the
// smaller harm and it is the honest one — and `Save` is still available afterwards, because the
// draft was not consumed by the refusal.
func TestAnUnknownScopeIsRefusedAndTheDraftSurvives(t *testing.T) {
	a, dir := aStudioRecording(t, "ark graphics")

	if why := a.SaveDraft("", ""); why == "" {
		t.Fatal("an empty scope was accepted; the routine would work somewhere " +
			"nobody chose")
	}
	if why := a.SaveDraft("somewhere", ""); why == "" {
		t.Fatal("an unrecognised scope was accepted")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("a refused save wrote %d entr(ies)", len(entries))
	}
	if !a.Recording().Review {
		t.Error("a refused save closed review, so the recording is gone and " +
			"nobody got to choose again")
	}
	if why := a.SaveDraft("app", ""); why != "" {
		t.Errorf("the draft could not be saved after a refusal: %s", why)
	}
}

// AND REVIEW SHOWS WHAT THE PRESS MEASURED ABOUT ITS WINDOW.
//
// Not decoration. Those terms are what the saved route will carry and what a run will check
// against, so a review that listed only the coordinate would hide the thing that decides whether
// the coordinate is ever trusted. A person looking at a routine recorded in a window they have
// since resized is owed the chance to see that before they keep it.
func TestReviewSaysWhatThePressMeasured(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	owner, ownerApp := uint32(os.Getpid()), winctx.Self()
	a.recording = func() recorder.Recorder {
		return &heldRecorder{events: []recorder.RecordedEvent{
			{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
			{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
				App: "arkascended", PID: gamePID, T: at(100),
				WinRel: true, RelX: 722, RelY: 210,
				RelTo: macroir.RelToWindow, WinW: 1920, WinH: 1080,
				Dpi: 144, Process: "arkascended.exe"},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
				App: "arkascended", PID: gamePID, T: at(105)},
			{Kind: recorder.EvAppSwitch, KeyName: ownerApp, T: at(3000)},
			{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511,
				Y: 913, App: ownerApp, PID: owner, T: at(3200)},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511,
				Y: 913, App: ownerApp, PID: owner, T: at(3205)},
		}}
	}
	if why := a.StartRecording("ark graphics"); why != "" {
		t.Fatal(why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatal(why)
	}

	line := ""
	for _, s := range a.Recording().Steps {
		if strings.Contains(s, "click") {
			line = s
		}
	}
	if line == "" {
		t.Fatal("review lists no click at all")
	}
	for _, want := range []string{"722, 210 in the window", "1920×1080",
		"144 dpi", "arkascended.exe"} {
		if !strings.Contains(line, want) {
			t.Errorf("review says %q and does not mention %q", line, want)
		}
	}
}

// AND A RECORDING OF NOTHING SAYS SO IN REVIEW, RATHER THAN BEING SAVED.
//
// The case where somebody presses Record, changes their mind, and presses Stop: the only events
// in the stream are their trip to this window and back. Every one of them is excluded, so there
// is nothing left — and a routine saved from that would be offered to them later and then do
// nothing at all.
//
// Review is still OPENED for it, deliberately. The alternative is a Record button that appears
// to do nothing, and the sentence in review is where the reason goes.
func TestARecordingWithNothingInItIsNotOffered(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	a := newActs(t.TempDir())
	owner, ownerApp := uint32(os.Getpid()), winctx.Self()
	a.recording = func() recorder.Recorder {
		return &heldRecorder{events: []recorder.RecordedEvent{
			{Kind: recorder.EvAppSwitch, KeyName: ownerApp, T: at(0)},
			{Kind: recorder.EvMove, X: 1500, Y: 900, T: at(100)},
			{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511,
				Y: 913, App: ownerApp, PID: owner, T: at(200)},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511,
				Y: 913, App: ownerApp, PID: owner, T: at(205)},
		}}
	}
	if why := a.StartRecording("a change of mind"); why != "" {
		t.Fatal(why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatal(why)
	}

	r := a.Recording()
	if !r.Review {
		t.Error("review was not opened, so pressing Stop looked like nothing " +
			"happening")
	}
	if !r.Failed || r.Said == "" {
		t.Errorf("review does not say the recording is empty: %+v", r)
	}
	if r.Pressable != 0 {
		t.Errorf("review counts %d pressable step(s) in a recording that is "+
			"nothing but a visit to this window", r.Pressable)
	}
	if why := a.SaveDraft("everywhere", ""); why == "" {
		t.Error("a recording that presses nothing was saved; it would be offered " +
			"to somebody and then do nothing")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("%d entr(ies) were written for a recording of nothing", len(entries))
	}
}

// AND NEITHER RECORDER RUNS WHILE THE OTHER DOES, NOR A RUN WHILE EITHER DOES.
//
// One screen. A runner pressing on it while the hooks are watching would put its own clicks into
// somebody's draft — and these hooks are in THIS process, so there is no process boundary to
// notice it. Both directions, because the predicate they share is new and a predicate asked in
// one direction only is half a guard.
func TestARecordingAndARunExcludeEachOther(t *testing.T) {
	// A REAL ACT, so `Play` reaches the guard rather than stopping at "there is no
	// Act called that". A refusal for the wrong reason would pass a test that looked
	// only for a refusal.
	a := aPlayableAct(t, "audio settings")
	a.recording = func() recorder.Recorder {
		return &heldRecorder{events: anUnshieldedGlanceAtStudio(
			uint32(os.Getpid()), winctx.Self())}
	}
	if why := a.StartRecording("ark graphics"); why != "" {
		t.Fatal(why)
	}
	if why := a.StartRecording("ark graphics two"); why == "" {
		t.Error("two captures were started at once")
	}
	// A RUN IS REFUSED WHILE THE HOOKS WATCH. Through `Play`, the production entrance:
	// the refusal lives behind a predicate shared with three other callers, and asking
	// that predicate directly would prove nothing about the path a person takes.
	if why := a.Play("audio settings", false, false, ""); !strings.Contains(why,
		"recording") {
		t.Errorf("a run was answered with %q while the screen was being "+
			"recorded", why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatal(why)
	}
	// AND A DRAFT IN REVIEW IS NOT A RECORDING. The hooks are gone and the screen is
	// free, so a run is allowed again — the guard is a guard and not a door that
	// locked. It does still block a SECOND capture, because there is one draft and
	// taking another would drop it unasked.
	if why := a.Play("audio settings", false, false, ""); strings.Contains(why,
		"recording") {
		t.Errorf("a run is still refused as recording after Stop: %q", why)
	}
	if why := a.StartRecording("something else"); why == "" {
		t.Error("a second recording was started over a draft nobody had saved " +
			"or cancelled")
	}
}
