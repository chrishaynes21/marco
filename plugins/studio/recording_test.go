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
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
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

	if why := a.StartRecording(name, ""); why != "" {
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
	if why := a.StartRecording("ark graphics again", ""); why != "" {
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
	if why := a.StartRecording("ark audio", ""); why != "" {
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
	if why := a.StartRecording("ark graphics", ""); why != "" {
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

// AND REVIEW EXPLAINS A MEASUREMENT THAT FAILED, AND WHETHER DOING IT AGAIN WOULD HELP.
//
// A press that declared the window it was made in and is missing a measurement will REFUSE at
// run time rather than press — which is the point, because the alternative was pressing a
// remembered pixel having checked nothing. The person has to find that out HERE, while they are
// still in front of the screen and before they keep it, and they have to be told enough to act:
// the name of what was not measured, and whether recording it again can change the answer.
//
// The case is the program lookup, because that one is unlike the others: it runs AFTER the
// capture, once the hooks are down, so it fails for a program that has exited between the press
// and the Stop button — the one measurement that can fail while the window itself answered
// everything.
func TestReviewExplainsAMeasurementThatFailed(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	a.recording = func() recorder.Recorder {
		return &heldRecorder{events: []recorder.RecordedEvent{
			{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
			// THE WINDOW ANSWERED EVERYTHING AND THE PROGRAM HAD GONE: no
			// `Process`, which is what the post-capture lookup leaves when
			// the program it would have opened is no longer running.
			{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
				App: "arkascended", PID: gamePID, T: at(100),
				WinRel: true, RelX: 722, RelY: 210,
				RelTo: macroir.RelToWindow, WinW: 1920, WinH: 1080,
				Dpi: 144},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
				App: "arkascended", PID: gamePID, T: at(105)},
		}}
	}
	if why := a.StartRecording("ark graphics", ""); why != "" {
		t.Fatal(why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatal(why)
	}

	r := a.Recording()
	line := ""
	for _, s := range r.Steps {
		if strings.Contains(s, "click") {
			line = s
		}
	}
	if line == "" {
		t.Fatal("review lists no click at all")
	}
	// ── BESIDE THE STEP: what it cannot be checked against ──
	for _, want := range []string{"cannot be checked", "which program owned it"} {
		if !strings.Contains(line, want) {
			t.Errorf("the step reads %q and does not mention %q", line, want)
		}
	}
	// AND WHAT IT DID MEASURE IS STILL THERE, because the person is deciding and the
	// measurement that arrived is half of what they are deciding about.
	if !strings.Contains(line, "1920×1080") || !strings.Contains(line, "144 dpi") {
		t.Errorf("the step reads %q and drops what WAS measured", line)
	}

	// ── AND THE DRAFT SAYS IT WILL NOT RUN, ONCE, IN WORDS ──
	if !r.Incomplete {
		t.Error("the draft does not say it is incomplete, so the page has " +
			"nothing to warn with")
	}
	// THE COUNT, THE MEASUREMENT AND THE REPAIR — and `REFUSE` is not one of them any
	// more. The page says it won't run in the heading directly above this sentence and
	// offers `Save anyway` directly below it, so the clause that said it a third time is
	// gone. That the SCREEN still says it is held by the page's own test
	// (TestTheReviewSaysItWillNotRunAboveTheReason); what this sentence has to carry is
	// which measurement did not arrive and whether another demonstration would help.
	for _, want := range []string{"1 press", "which program owned it",
		"Record it again"} {
		if !strings.Contains(r.Missing, want) {
			t.Errorf("the account reads %q and does not mention %q",
				r.Missing, want)
		}
	}

	// ── AND SAVING IT SAYS SO TOO, RATHER THAN "SAVED, RUN IT" ──
	//
	// Saving is allowed: the draft is a true record of what somebody did, and throwing
	// it away would lose the demonstration along with the measurement. What is not
	// allowed is saving it quietly as something that runs.
	if why := a.SaveDraft("app-anywhere", "arkascended"); why != "" {
		t.Fatalf("saving an incomplete recording was refused: %s", why)
	}
	after := a.Recording()
	if after.Saved == "" {
		t.Fatal("nothing was saved")
	}
	if !strings.Contains(after.Said, "refuse to press") {
		t.Errorf("the save message reads %q and does not say the routine "+
			"will refuse", after.Said)
	}
	if !after.Incomplete {
		t.Error("what was saved stopped being incomplete the moment it was saved")
	}
}

// AND A RECORDING THAT MEASURED EVERYTHING SAYS NONE OF THAT.
//
// The control, and it earns its place: a warning that appears on every recording is a warning
// nobody reads, and the whole value of the sentence above is that it is absent when the routine
// will actually run.
func TestReviewIsQuietWhenEverythingWasMeasured(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
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
		}}
	}
	if why := a.StartRecording("ark graphics", ""); why != "" {
		t.Fatal(why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatal(why)
	}

	r := a.Recording()
	if r.Incomplete || r.Missing != "" {
		t.Errorf("a fully measured recording is reported as incomplete: %q",
			r.Missing)
	}
	for _, s := range r.Steps {
		if strings.Contains(s, "cannot be checked") {
			t.Errorf("a fully measured press says %q", s)
		}
	}

	// AND A PRESS ABOUT NO WINDOW AT ALL IS NOT REPORTED AS INCOMPLETE EITHER.
	//
	// Incomplete means "it asked to be checked and cannot be". A press with no basis
	// asked for nothing and keeps the behaviour it was saved with, so flagging it would
	// put the warning on every legacy press there is — and a warning that appears on
	// everything is a warning nobody reads.
	legacy := reviewOf(t, func(e *recorder.RecordedEvent) {
		e.RelTo, e.WinW, e.WinH, e.Dpi, e.Process = "", 0, 0, 0, ""
	})
	if legacy.Incomplete || legacy.Missing != "" {
		t.Errorf("a press that asked to be checked against nothing is reported "+
			"as incomplete: %q", legacy.Missing)
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
	if why := a.StartRecording("a change of mind", ""); why != "" {
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
	if why := a.StartRecording("ark graphics", ""); why != "" {
		t.Fatal(why)
	}
	if why := a.StartRecording("ark graphics two", ""); why == "" {
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
	if why := a.StartRecording("something else", ""); why == "" {
		t.Error("a second recording was started over a draft nobody had saved " +
			"or cancelled")
	}
}

// AND THE SENTENCE AGREES WITH ITS OWN COUNT.
//
// # Why this is a test and not a reading
//
// `1 press lack` reached the screen in this file's previous copy pass, from a format string that
// counted presses and then named one in a word that did not bend with the count. The copy pass
// after it shortened the sentence again, which moved the verb — so this is the second edit in two
// changes to touch exactly the construction that broke, and neither edit could be checked by
// reading the format string, because both readings looked right.
//
// So the function is called over one unmeasured press and then two, and what is asserted is the
// agreement itself. The measurement names are CLAUSES ("which program owned it"), which is why
// they are introduced with a colon rather than hung off a verb; that is asserted too, since a
// later shortening that drops the colon produces "1 press is missing which program owned it".
func TestTheWontRunSentenceAgreesWithItsOwnCount(t *testing.T) {
	// A press that asked to be checked and whose program had gone: the one measurement
	// that fails after the hooks are down. See notMeasured.
	press := macroir.Step{Kind: macroir.StepClick, RelTo: macroir.RelToWindow,
		WinRel: true, RelX: 10, RelY: 20, WinW: 1920, WinH: 1080, Dpi: 144}
	for _, c := range []struct {
		presses int
		want    string
	}{
		{1, "1 press is missing a measurement: which program owned it."},
		{2, "2 presses are missing measurements: which program owned it."},
	} {
		steps := make([]macroir.Step, c.presses)
		for i := range steps {
			steps[i] = press
		}
		if said := whyItWillNotRun(steps); !strings.Contains(said, c.want) {
			t.Errorf("%d unmeasured press(es) reads %q, which does not open "+
				"%q", c.presses, said, c.want)
		}
	}
}

// AND EVERY MEASUREMENT THAT CAN FAIL IS NAMED, ONE AT A TIME.
//
// Four measurements, four reasons they fail on real desktops, and each one has to appear in the
// review by name — because "this press cannot be checked" tells somebody nothing about whether
// doing it again would help, and the name of the missing measurement does.
func TestReviewNamesWhicheverMeasurementFailed(t *testing.T) {
	for _, c := range []struct {
		what string
		lose func(*recorder.RecordedEvent)
		says string
	}{
		{"the window would not say how big it is",
			func(e *recorder.RecordedEvent) { e.WinW, e.WinH = 0, 0 },
			"how big the window was"},
		{"this Windows cannot report a scale",
			func(e *recorder.RecordedEvent) { e.Dpi = 0 },
			"what scale it was drawn at"},
		{"the program had gone before the lookup ran",
			func(e *recorder.RecordedEvent) { e.Process = "" },
			"which program owned it"},
		// THE PRESS COULD NOT BE PLACED INSIDE THE WINDOW. A click that CHANGED the
		// foreground looks like this from inside the hook: the attribution names the
		// window that was in front at that instant and the click landed in the one
		// it was about to raise.
		{"the press could not be placed inside the window",
			func(e *recorder.RecordedEvent) {
				e.WinRel, e.RelX, e.RelY = false, 0, 0
			},
			"where in the window the press went"},
	} {
		t.Run(c.what, func(t *testing.T) {
			r := reviewOf(t, c.lose)
			line := ""
			for _, s := range r.Steps {
				if strings.Contains(s, "click") {
					line = s
				}
			}
			if !strings.Contains(line, c.says) {
				t.Errorf("the step reads %q and does not name %q", line,
					c.says)
			}
			if !r.Incomplete || !strings.Contains(r.Missing, c.says) {
				t.Errorf("the account reads %q and does not name %q",
					r.Missing, c.says)
			}
		})
	}
}

// AND A SCALE THIS COMPUTER CANNOT REPORT IS NOT SOMETHING TO TRY AGAIN.
//
// The one case where "record it again" is the wrong advice. `GetDpiForWindow` arrived in Windows
// 10 1607; without it no press on this machine will ever carry a scale, so a routine that checks
// the window it was made in cannot be made here at all — and sending somebody back to
// re-demonstrate, repeatedly, for an answer that cannot arrive is the worse of the two mistakes.
//
// Reachable only through the seam: this machine CAN report a scale, which is why the branch
// needs a test rather than a reading.
func TestAScaleThisComputerCannotReportIsNotRepairable(t *testing.T) {
	was := canReadWindowScale
	t.Cleanup(func() { canReadWindowScale = was })
	canReadWindowScale = func() bool { return false }

	r := reviewOf(t, func(e *recorder.RecordedEvent) { e.Dpi = 0 })

	// THE THREE THINGS IT HAS TO SAY, and `too old` is not one of them.
	//
	// This asserted the phrase `too old` as well, and the copy pass dropped it: the age of
	// this computer's Windows is the CAUSE and not the thing a person can act on. What they
	// can act on is that it cannot be done here, which is asserted below and unchanged —
	// along with the two halves that matter just as much, that the scale is the reason and
	// that another demonstration is not the answer.
	if !strings.Contains(r.Missing, "scale") ||
		!strings.Contains(r.Missing, "cannot be made on this machine") {
		t.Errorf("the account reads %q, and on a computer that can never "+
			"report a scale it has to say so rather than ask for another "+
			"demonstration", r.Missing)
	}
	if strings.Contains(r.Missing, "Record it again") {
		t.Errorf("the account asks for another demonstration that cannot "+
			"change the answer: %q", r.Missing)
	}
}

// AND REVIEW SAYS THE WINDOW WAS LEFT.
//
// The card is what somebody looks at before deciding whether to keep a recording, and a draft
// with fewer steps than they performed needs the reason beside it — otherwise the only reading
// available is that the recorder missed something. See orchestrator.Draft.AwaySentence.
func TestReviewSaysTheWindowWasLeft(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	a.recording = func() recorder.Recorder {
		return &heldRecorder{events: []recorder.RecordedEvent{
			{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
			{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
				App: "arkascended", PID: gamePID, T: at(100)},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 842,
				Y: 274, App: "arkascended", PID: gamePID, T: at(105)},
			// AND HALF A MINUTE SOMEWHERE ELSE.
			{Kind: recorder.EvLeft, T: at(200)},
			{Kind: recorder.EvEntered, T: at(30200)},
			{Kind: recorder.EvClick, Button: "left", Down: true, X: 900, Y: 300,
				App: "arkascended", PID: gamePID, T: at(30300)},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 900,
				Y: 300, App: "arkascended", PID: gamePID, T: at(30305)},
		}}
	}
	if why := a.StartRecording("ark graphics", ""); why != "" {
		t.Fatal(why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatal(why)
	}
	r := a.Recording()
	if !strings.Contains(r.Away, "outside the window") {
		t.Errorf("the review card says %q about the absence", r.Away)
	}
	// AND THE STEPS ON BOTH SIDES ARE THERE, so this is a recording with a gap in it
	// rather than one that stopped when the person walked away.
	if r.Pressable < 2 {
		t.Errorf("the draft has %d pressable steps, so the clicks on either side "+
			"of the absence did not both survive", r.Pressable)
	}
}

// reviewOf records one fully measured press with `lose` applied to it, stops, and returns what
// review says.
func reviewOf(t *testing.T, lose func(*recorder.RecordedEvent)) Recording {
	t.Helper()
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	press := recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left",
		Down: true, X: 842, Y: 274, App: "arkascended", PID: gamePID,
		T: at(100), WinRel: true, RelX: 722, RelY: 210,
		RelTo: macroir.RelToWindow, WinW: 1920, WinH: 1080, Dpi: 144,
		Process: "arkascended.exe"}
	if lose != nil {
		lose(&press)
	}
	a.recording = func() recorder.Recorder {
		return &heldRecorder{events: []recorder.RecordedEvent{
			{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
			press,
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 842,
				Y: 274, App: "arkascended", PID: gamePID, T: at(105)},
		}}
	}
	if why := a.StartRecording("ark graphics", ""); why != "" {
		t.Fatal(why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatal(why)
	}
	return a.Recording()
}

// A GUARDED RECORDING KEEPS TWO SCOPES, AND THE THIRD IS UNAVAILABLE WITH ITS REASON (S9).
//
// "Only in" binds the window in front when the run is admitted, and "From anywhere → app" binds
// the one it brings forward, so both are the person's choice and the chooser opens on the ruled
// default. "From anywhere → current foreground" names no application, so there is no window for
// a checked press, and it is withdrawn with the backend's own sentence.
//
// Forcing a single scope again, or offering current foreground, must fail this.
func TestAGuardedRecordingKeepsTwoScopesAndSaysWhyNotTheThird(t *testing.T) {
	r := reviewOf(t, nil)
	if r.Scope != "" || r.Brings != "" {
		t.Errorf("the choice was withdrawn: scope %q, %q", r.Scope, r.Brings)
	}
	if r.DefaultScope != "app-anywhere" {
		t.Errorf("the chooser opens on %q, not the ruled default", r.DefaultScope)
	}
	if len(r.Unavailable) != 1 || !strings.Contains(r.Unavailable["everywhere"], "names no application") {
		t.Errorf("unavailable: %v", r.Unavailable)
	}
}

// AND A RECORDING THAT ASKED TO BE CHECKED AGAINST NOTHING KEEPS ALL THREE.
//
// The half that keeps this confined. A routine whose presses are the two numbers they always
// were is saved under the contract it always was, and where it works is still the person's to
// say.
func TestALegacyRecordingKeepsTheChoice(t *testing.T) {
	r := reviewOf(t, func(e *recorder.RecordedEvent) {
		e.RelTo, e.WinW, e.WinH, e.Dpi, e.Process = "", 0, 0, 0, ""
	})

	if r.Scope != "" || r.Brings != "" {
		t.Errorf("a recording that checks no window had its scope decided for "+
			"it: %q / %q", r.Scope, r.Brings)
	}
	// AND THE CHOOSER OPENS ON THE SHARED DEFAULT (R2), which the backend states rather
	// than the page typing its own.
	if r.DefaultScope != "app-anywhere" {
		t.Errorf("the chooser opens on %q, want the shared default app-anywhere",
			r.DefaultScope)
	}
}

// AND THE PAGE TAKES THE BACKEND'S DEFAULT, never a scope word of its own.
func TestThePageOpensTheScopeChooserOnTheBackendsDefault(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(raw)
	if !strings.Contains(page, "r.default_scope") {
		t.Error("the page does not read the backend's default scope")
	}
	if strings.Contains(page, "had || 'app'\n") {
		t.Error("the page still opens the chooser on its own 'app' (Only in) default")
	}
}

// AND SAVING ONE UNDER A SCOPE ITS PRESSES CANNOT BE PERFORMED UNDER IS REFUSED.
//
// Refused and explained, never silently rewritten: quietly saving it as something else would be
// this window deciding where somebody's routine works, and saving it as asked would write a
// routine that refuses every press. Nothing reaches disk either way.
func TestSavingAGuardedRecordingUnderAnImpossibleScopeIsRefused(t *testing.T) {
	// ONE SCOPE SINCE S9: Only in binds the window in front at admission, and is saved as asked
	// (TestAGuardedRecordingCanBeSavedOnlyIn).
	for _, scope := range []string{"everywhere"} {
		t.Run(scope, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("MARCO_CV", "off")
			t.Setenv("MARCO_ROUTES", dir)
			a := recorded(t, nil)

			why := a.SaveDraft(scope, "arkascended")
			if why == "" {
				t.Fatal("it was saved under a scope that brings no window " +
					"forward, so every press in it will refuse")
			}
			for _, want := range []string{"checks the window it was made in",
				"names no application", "Only in arkascended"} {
				if !strings.Contains(why, want) {
					t.Errorf("the refusal says %q and does not mention "+
						"%q", why, want)
				}
			}
			if a.Recording().Saved != "" {
				t.Error("something was written anyway")
			}
			// AND THE SCOPE IT NEEDS IS STILL ACCEPTED AFTERWARDS, because a
			// refusal is a question asked again and not a dead end.
			if why := a.SaveDraft("app-anywhere", "arkascended"); why != "" {
				t.Errorf("the scope it asked for was then refused: %s", why)
			}
		})
	}
}

// AND A ROUTINE ALREADY SAVED THAT CANNOT PRESS ANYTHING IS LISTED, AND SAVED AGAIN ON ASKING.
//
// The guarded contract arrived before the rule above it, so a routine saved in between is on disk
// now: it reads like a routine and refuses every press, and the only thing that says so at run
// time is the refusal itself, one press at a time. Its own demonstration is stored beside it, so
// saving it again needs nobody to re-demonstrate anything — and it is done on asking, by name,
// because a routine that rescoped itself quietly is the thing Save refuses to do.
func TestARoutineThatCannotPressAnythingIsListedAndSavedAgain(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", dir)
	a := recorded(t, nil)

	// SAVED THE ONE WAY STILL UNRUNNABLE SINCE S9: through the backend, under the current-
	// foreground scope, which names no application and so binds no window.
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	dr := a.capture.draft
	if _, err := d.Save(dr, orchestrator.ScopeEverywhere, ""); err != nil {
		t.Fatal(err)
	}

	listed := a.Unrunnable()
	if len(listed) != 1 || !strings.EqualFold(listed[0].Name, "ark graphics") {
		t.Fatalf("the routine is not listed as unable to press anything: %+v",
			listed)
	}
	if !listed[0].Resavable {
		t.Fatal("its demonstration was not kept, so it cannot be saved again")
	}

	if why := a.Resave(listed[0].ID); why != "" {
		t.Fatalf("saving it again was refused: %s", why)
	}
	if left := a.Unrunnable(); len(left) != 0 {
		t.Errorf("it is still listed as unable to press anything: %+v", left)
	}
	// AND WHAT IS ON DISK NOW BRINGS THE APPLICATION FORWARD, which is the whole
	// point: its presses have a window to be checked against.
	saved := ""
	for _, rt := range a.reg().List() {
		if rt.Slug == "ark-graphics" {
			raw, err := os.ReadFile(a.reg().Path(rt))
			if err != nil {
				t.Fatal(err)
			}
			saved = string(raw)
		}
	}
	if saved == "" {
		t.Fatal("nothing called ark-graphics is on disk")
	}
	if !strings.Contains(saved, `do OS's Activate with "arkascended"`) {
		t.Errorf("the routine saved again still brings no window forward:\n%s",
			saved)
	}
	if !strings.Contains(saved, `RelTo "window"`) {
		t.Errorf("it lost what its presses are checked against:\n%s", saved)
	}
	// AND THERE IS ONE OF IT. Two routines of the same name, one of which cannot press
	// anything, is worse than the one that could not.
	if n := strings.Count(strings.Join(slugsIn(a), "|"), "ark-graphics"); n != 1 {
		t.Errorf("there are %d copies of it on disk", n)
	}
}

// slugsIn is every route slug on disk.
func slugsIn(a *Acts) []string {
	var out []string
	for _, rt := range a.reg().List() {
		out = append(out, rt.Slug)
	}
	return out
}

// recorded is a window that has just stopped a recording of one fully measured press, with
// `lose` applied to it.
func recorded(t *testing.T, lose func(*recorder.RecordedEvent)) *Acts {
	t.Helper()
	a := newActs(t.TempDir())
	press := recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left",
		Down: true, X: 842, Y: 274, App: "arkascended", PID: gamePID,
		T: at(100), WinRel: true, RelX: 722, RelY: 210,
		RelTo: macroir.RelToWindow, WinW: 1920, WinH: 1080, Dpi: 144,
		Process: "arkascended.exe"}
	if lose != nil {
		lose(&press)
	}
	a.recording = func() recorder.Recorder {
		return &heldRecorder{events: []recorder.RecordedEvent{
			{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
			press,
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 842,
				Y: 274, App: "arkascended", PID: gamePID, T: at(105)},
		}}
	}
	if why := a.StartRecording("ark graphics", ""); why != "" {
		t.Fatal(why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatal(why)
	}
	return a
}

// A RIGHT OR MIDDLE PRESS IS SAVED, AND THE REVIEW SAYS WHICH BUTTON IT IS.
//
// THIS TEST USED TO ASSERT THE OPPOSITE, and what changed is the generator rather than the
// standard. A guarded routine has to stop when a press does not happen, and the chain that makes
// that true wraps ONE `do` sentence in arms that both end — so a press emitted as an aim and then
// a click at the cursor had nothing to wrap, and Studio refused the recording before Save with a
// sentence naming the button. The Point carries the button now: one sentence, one guard, one
// failure for the arm to stop at.
//
// What the window owes somebody is unchanged and is what is asserted here: the review says what
// each press IS, because "click at 722, 210 in the window" for a right-click would be a review of
// a routine they are not about to save. And the saved file carries the button beside the place
// with the window terms intact, which is what makes it the guarded contract rather than a legacy
// press that happens to name a button.
func TestARightOrMiddlePressIsSavedAndTheReviewSaysWhichButton(t *testing.T) {
	for _, button := range []string{"right", "middle"} {
		t.Run(button, func(t *testing.T) {
			t.Setenv("MARCO_CV", "off")
			t.Setenv("MARCO_ROUTES", t.TempDir())
			a := recorded(t, func(e *recorder.RecordedEvent) { e.Button = button })

			r := a.Recording()
			if r.Unsupported != "" {
				t.Errorf("the window says a %s press cannot be saved: %s",
					button, r.Unsupported)
			}
			if r.Incomplete {
				t.Errorf("the window says it will not run: %s", r.Missing)
			}
			// THE REVIEW NAMES THE BUTTON, in the line for that press.
			said := strings.Join(r.Steps, " | ")
			if !strings.Contains(said, button+"-click at 722, 210 in the window") {
				t.Errorf("the review reads %q, and does not say the press is "+
					"a %s one at the place it was measured", said, button)
			}
			// AND IT SAVES, and what was written is a guarded press of that
			// button: the button beside the place, with the window it was
			// measured in. Read off the file, because the file is the only
			// thing that runs.
			if why := a.SaveDraft("app-anywhere", "arkascended"); why != "" {
				t.Fatalf("a %s press was refused: %s", button, why)
			}
			at := a.Recording().Saved
			if at == "" {
				t.Fatal("nothing was written")
			}
			src, err := os.ReadFile(at)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{
				"Button \"" + button + "\"",
				"RelX 722", "RelTo \"window\"", "WinW", "Dpi", "Process",
			} {
				if !strings.Contains(string(src), want) {
					t.Errorf("the saved routine does not state %q:\n%s",
						want, src)
				}
			}
			// AND THE CONTRACT IT IS READ BACK AS IS THE GUARDED ONE, which is
			// the word Studio prints on the card and `marco do` records. A new
			// guarded shape that read back as legacy would be described to a
			// person as a press that checks nothing — which is what happened
			// to the first guarded drag.
			if got, _ := orchestrator.ContractOf(string(src)); got != orchestrator.Guarded {
				t.Errorf("a saved %s press reads back as %q", button, got)
			}
		})
	}
}

// AND A RECORDING OF ORDINARY LEFT-CLICKS SAYS NONE OF THAT.
func TestAnOrdinaryRecordingIsSavedWithoutComplaint(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := recorded(t, nil)

	if r := a.Recording(); r.Unsupported != "" {
		t.Errorf("an ordinary recording says %q", r.Unsupported)
	}
	if why := a.SaveDraft("app-anywhere", "arkascended"); why != "" {
		t.Fatalf("an ordinary recording was refused: %s", why)
	}
}

// AND A RIGHT-CLICK IN A RECORDING THAT CHECKS NO WINDOW IS SAVED AS IT ALWAYS WAS.
//
// The half that keeps the refusal confined. A routine whose presses are the two numbers they
// always were was saved under a contract that says a click is attempted and the run carries on —
// so a right-click in one is not this contract's business, and refusing it would refuse a kind of
// recording that has always worked.
func TestARightClickInALegacyRecordingIsStillSaved(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := recorded(t, func(e *recorder.RecordedEvent) {
		e.Button = "right"
		e.RelTo, e.WinW, e.WinH, e.Dpi, e.Process = "", 0, 0, 0, ""
	})

	if r := a.Recording(); r.Unsupported != "" {
		t.Errorf("a recording that checks no window says %q", r.Unsupported)
	}
	if why := a.SaveDraft("app", "arkascended"); why != "" {
		t.Fatalf("a right-click recorded before any of this was refused: %s", why)
	}
}

// ── A MID-RECORDING VISIT, AS A REAL DESKTOP ATTRIBUTES IT ────────────────────

// aVisitToStudioWithCrossedEdges is the owner's first mouse-driven trial, as the hooks saw it.
//
// Every other fixture in this file gives both edges of a click one process id. A real desktop
// does not: the press that RAISES a window arrives while the previous window is still in front,
// because clicking a window is what causes the activation. So the click onto Studio is attributed
// (press, release) = (game, Studio), and the click back is (Studio, game).
//
// Excluded edge by edge, each of those clicks lost one half, and what was left — a press on
// Studio and a release in the game with no press between them — came out of `simplify` as a
// DRAG. A drag cannot be saved in a routine whose presses are checked, so the whole recording
// was refused at Save: `drag from -740, 348 to 50, 374`, which is the press of one click joined
// to the release of the next.
func aVisitToStudioWithCrossedEdges(owner uint32, ownerApp string) []recorder.RecordedEvent {
	crossed := func(x, y int, press, release uint32, ms int) []recorder.RecordedEvent {
		return []recorder.RecordedEvent{
			{Kind: recorder.EvClick, Button: "left", Down: true, X: x, Y: y,
				PID: press, T: at(ms)},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: x, Y: y,
				PID: release, T: at(ms + 5)},
		}
	}
	measured := func(x, y, relX, relY int, ms int) []recorder.RecordedEvent {
		return []recorder.RecordedEvent{
			{Kind: recorder.EvClick, Button: "left", Down: true, X: x, Y: y,
				App: "arkascended", PID: gamePID, T: at(ms),
				WinRel: true, RelX: relX, RelY: relY,
				RelTo: macroir.RelToWindow, WinW: 1936, WinH: 1048, Dpi: 96,
				Process: "arkascended.exe"},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: x, Y: y,
				App: "arkascended", PID: gamePID, T: at(ms + 5)},
		}
	}
	var s []recorder.RecordedEvent
	s = append(s, recorder.RecordedEvent{Kind: recorder.EvAppSwitch,
		KeyName: "arkascended", T: at(0)})
	s = append(s, measured(900, 400, 57, 343, 500)...)
	// ── A LOOK AT STUDIO: pointer travels, then a click whose halves disagree ──
	s = append(s, recorder.RecordedEvent{Kind: recorder.EvMove, X: -700, Y: 340,
		T: at(2000)})
	s = append(s, crossed(-740, 348, gamePID, owner, 2400)...)
	s = append(s, recorder.RecordedEvent{Kind: recorder.EvAppSwitch,
		KeyName: ownerApp, T: at(2500)})
	// ── AND THE CLICK BACK, disagreeing the other way ──
	s = append(s, recorder.RecordedEvent{Kind: recorder.EvMove, X: 40, Y: 370,
		T: at(4000)})
	s = append(s, crossed(50, 374, owner, gamePID, 4400)...)
	s = append(s, recorder.RecordedEvent{Kind: recorder.EvAppSwitch,
		KeyName: "arkascended", T: at(4500)})
	s = append(s, measured(880, 510, 37, 450, 6000)...)
	// ── AND STOP, WHICH IS ALSO AN ACTIVATION CLICK ──
	s = append(s, recorder.RecordedEvent{Kind: recorder.EvMove, X: 1500, Y: 900,
		T: at(7000)})
	s = append(s, crossed(1511, 913, gamePID, owner, 7100)...)
	return s
}

// A VISIT TO STUDIO MID-RECORDING LEAVES THE RECORDING SAVEABLE, AND KEEPS BOTH PRESSES.
//
// The whole flow through Studio's own methods: Record, Stop, read the review, Save. What the
// owner got instead was a review containing a drag and a refusal to save it at all.
func TestAVisitToStudioLeavesARecordingThatStillSaves(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	a := newActs(t.TempDir())
	owner, ownerApp := uint32(os.Getpid()), winctx.Self()
	if ownerApp == "" {
		t.Fatal("Studio cannot name its own application")
	}
	a.recording = func() recorder.Recorder {
		return &heldRecorder{events: aVisitToStudioWithCrossedEdges(owner, ownerApp)}
	}
	if why := a.StartRecording("ark graphics", ""); why != "" {
		t.Fatal(why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatal(why)
	}
	r := a.Recording()

	// NO INVENTED GESTURE IN THE REVIEW, which is the thing the person reads.
	for _, step := range r.Steps {
		if strings.Contains(step, "drag") {
			t.Errorf("the review contains a gesture nobody performed: %q\n"+
				"all steps: %v", step, r.Steps)
		}
	}
	// AND IT CAN BE SAVED. A drag refuses generation outright, so this is the
	// sentence the owner actually hit.
	if r.Unsupported != "" {
		t.Fatalf("the recording cannot be saved: %s", r.Unsupported)
	}
	// BOTH DEMONSTRATED PRESSES, measured in the window they were made in — the one
	// before the visit and the one after it.
	presses := pressesIn(r)
	if len(presses) != 2 {
		t.Fatalf("%d press(es) in the review, want the 2 that were demonstrated: "+
			"%v", len(presses), r.Steps)
	}
	for i, want := range []string{"57, 343 in the window", "37, 450 in the window"} {
		if !strings.Contains(presses[i], want) {
			t.Errorf("press %d reads %q, want it to carry %q", i, presses[i],
				want)
		}
	}
	// AND NOTHING OF STUDIO'S OWN WINDOW, by either half of any of its clicks.
	for _, step := range r.Steps {
		for _, gone := range []string{"-740", "1511", "50, 374"} {
			if strings.Contains(step, gone) {
				t.Errorf("a step carries half a click on Studio's own "+
					"window: %q", step)
			}
		}
	}

	// AND IT SAVES, AND WHAT IS ON DISK PRESSES TWICE.
	if why := a.SaveDraft("app-anywhere", "arkascended"); why != "" {
		t.Fatalf("saving it: %s", why)
	}
	saved := ""
	for _, rt := range a.reg().List() {
		if rt.Slug == "ark-graphics" {
			raw, err := os.ReadFile(a.reg().Path(rt))
			if err != nil {
				t.Fatal(err)
			}
			saved = string(raw)
		}
	}
	if saved == "" {
		t.Fatal("nothing was written to the library")
	}
	if n := strings.Count(saved, "do OS's Click"); n != 2 {
		t.Errorf("the saved routine presses %d time(s), want 2:\n%s", n, saved)
	}
	if strings.Contains(saved, "Drag") {
		t.Errorf("the saved routine drags:\n%s", saved)
	}
}

// S9: a guarded recording saved "Only in" its application is saved there, and is not listed as a
// routine that cannot press anything.
func TestAGuardedRecordingCanBeSavedOnlyIn(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", dir)
	a := recorded(t, nil)
	if why := a.SaveDraft("app", "arkascended"); why != "" {
		t.Fatalf("Only in was refused: %s", why)
	}
	if a.Recording().Saved == "" {
		t.Fatal("nothing was saved")
	}
	if got := a.Unrunnable(); len(got) != 0 {
		t.Errorf("a guarded Only-in routine is listed as unable to press anything: %+v", got)
	}
}
