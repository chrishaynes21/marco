package main

// RECORDING A ROUTINE AGAIN, AND THE ONE THING IT MUST NOT DO.
//
// The claim that matters here is a NEGATIVE one: *Record again* carries no authority to replace
// anything. A button that re-recorded a routine and then replaced it on an ordinary Save would
// have reintroduced, through a door marked convenience, exactly the loss R5 was built to prevent —
// and the person pressing it would have had more reason than anybody to expect it.
//
// So every test below that checks the convenience is paired with one that checks the safeguard,
// and the central one presses Save the ordinary way and requires the first routine to still be
// there, byte for byte, with its demonstration beside it.
//
// See [[ADR-256-recording-a-routine-again]].

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// aRoutineWorthRecordingAgain records and saves one routine, and hands back the window and it.
func aRoutineWorthRecordingAgain(t *testing.T) (*Acts, Routine, string) {
	t.Helper()
	a, dir := aStudioRecording(t, "ark graphics")
	if why := a.SaveDraft(string(orchestrator.ScopeAppAnywhere), ""); why != "" {
		t.Fatalf("the first save refused: %s", why)
	}
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d routines after one save", len(rs))
	}
	return a, rs[0], dir
}

// recordingAgain starts a second recording of one routine, in the application given.
//
// `observed` is the application the new demonstration happens in, which is the whole of what
// decides which routine a save would write — so a test that could not vary it could not pose the
// one question that is not about replacement.
func recordingAgain(t *testing.T, a *Acts, id, observed string) *heldRecorder {
	t.Helper()
	owner, ownerApp := uint32(os.Getpid()), winctx.Self()
	held := &heldRecorder{events: aDemonstrationIn(observed, owner, ownerApp)}
	a.recording = func() recorder.Recorder { return held }
	if why := a.RecordAgain(id); why != "" {
		t.Fatalf("RecordAgain refused: %s", why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatalf("StopRecording: %s", why)
	}
	return held
}

// aDemonstrationIn is two presses in one named application, ended by Studio's own Stop button.
//
// Built from `os.Getpid` and `winctx.Self` like every other fixture in this package, so Studio's
// own events are excluded by what production claims as its own rather than by a constant.
func aDemonstrationIn(app string, owner uint32, ownerApp string) []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: app, T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 640, Y: 480,
			App: app, PID: gamePID, RelTo: "window", RelX: 40, RelY: 80,
			WinW: 1920, WinH: 1080, Dpi: 96, Process: app + ".exe", T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 640, Y: 480,
			App: app, PID: gamePID, T: at(105)},
		{Kind: recorder.EvKey, KeyName: "escape", Down: true, App: app,
			PID: gamePID, T: at(800)},
		{Kind: recorder.EvKey, KeyName: "escape", Down: false, App: app,
			PID: gamePID, T: at(805)},
		{Kind: recorder.EvAppSwitch, KeyName: ownerApp, T: at(2000)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: ownerApp, PID: owner, T: at(2100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: ownerApp, PID: owner, T: at(2105)},
	}
}

// IT STARTS A RECORDING WITH THE ROUTINE'S NAME, ITS APPLICATION AND ITS SCOPE.
//
// The convenience, which is the point of the button: re-recording used to mean reading the name
// off the card and typing it back into the box exactly. Get it wrong by a space and you have two
// routines rather than a new version of one.
func TestRecordAgainStartsWithTheRoutinesOwnNameAndScope(t *testing.T) {
	a, r, _ := aRoutineWorthRecordingAgain(t)
	recordingAgain(t, a, r.ID, "arkascended")

	got := a.Recording()
	if got.Name != r.Name {
		t.Errorf("the second recording is called %q, not %q", got.Name, r.Name)
	}
	if got.Again != r.ID {
		t.Errorf("it does not say which routine it is a second recording of: %q",
			got.Again)
	}
	if got.AgainName != r.Name {
		t.Errorf("it does not name that routine: %q", got.AgainName)
	}
	if got.AgainScope != r.Scope {
		t.Errorf("it opens on scope %q, not the routine's own %q", got.AgainScope,
			r.Scope)
	}
	// AND IT SAYS WHAT SAVE WILL DO, which is the expectation this has to head off:
	// somebody who pressed Record again may reasonably think Save replaces.
	if !strings.Contains(got.AgainSays, "NOT replace") {
		t.Errorf("it does not say that saving will not replace that routine: %q",
			got.AgainSays)
	}
}

// AND AN ORDINARY SAVE DOES NOT REPLACE THE ROUTINE IT WAS RECORDED FROM.
//
// The claim the whole slice rests on. R5 exists because `Registry.Save` is a plain file write, so
// keeping a recording under a name already in the library replaced the source and the
// demonstration beside it with nothing asked and nothing said.
func TestRecordAgainCannotOverwriteThroughOrdinarySave(t *testing.T) {
	a, r, dir := aRoutineWorthRecordingAgain(t)
	// THE FILE AS IT STANDS, byte for byte, and its demonstration beside it.
	before, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	recording, recErr := os.ReadFile(strings.TrimSuffix(r.Path, ".marco") + ".rec.json")

	recordingAgain(t, a, r.ID, "arkascended")
	why := a.SaveDraft(string(orchestrator.ScopeAppAnywhere), "")
	if why == "" {
		t.Fatal("an ordinary Save after Record again replaced the routine without " +
			"being asked, which is the loss R5 exists to prevent")
	}
	// AND THE REFUSAL NAMES THE ROUTINE AND OFFERS THE WAY ON, rather than simply
	// failing: a window that refused without saying what is there has stopped somebody
	// from doing the thing they came to do.
	if !strings.Contains(why, r.Name) {
		t.Errorf("the refusal does not name what is already there: %q", why)
	}
	if !strings.Contains(strings.ToLower(why), "replace that routine on purpose") {
		t.Errorf("the refusal does not offer a deliberate replacement: %q", why)
	}

	after, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Error("the routine on disk changed, so something was written over it")
	}
	if recErr == nil {
		if _, err := os.ReadFile(strings.TrimSuffix(r.Path, ".marco") + ".rec.json"); err != nil {
			t.Error("the demonstration beside the routine is gone, which is the " +
				"copy a routine is ever saved again from")
		}
		_ = recording
	}
	// AND THE LIBRARY STILL HOLDS EXACTLY ONE. Nothing was written anywhere else
	// either, which a refusal that had quietly saved under another name would hide.
	if n := len(a.Routines()); n != 1 {
		t.Errorf("the library holds %d routines after a refused save", n)
	}
	_ = dir
}

// AND REPLACING IT IS STILL A THING SOMEBODY CHOOSES, BY NAME.
//
// The way on. `ReplaceDraft` takes the identity somebody agreed to replace and checks it against
// the identity this save would write — so agreeing to replace one routine cannot replace another
// that arrived at the target in the meantime.
func TestAfterRecordAgainReplacingIsStillAnExplicitChoice(t *testing.T) {
	a, r, _ := aRoutineWorthRecordingAgain(t)
	recordingAgain(t, a, r.ID, "arkascended")
	if why := a.SaveDraft(string(orchestrator.ScopeAppAnywhere), ""); why == "" {
		t.Fatal("the ordinary Save did not refuse, so there is no choice to make")
	}
	// THE COLLISION AS THE PAGE ASKS FOR IT, and it must name the routine this was
	// recorded from — not some other routine that happens to be in the library.
	c := a.Collision(string(orchestrator.ScopeAppAnywhere), "")
	if !c.Exists {
		t.Fatal("the window cannot say what is at the target, so it cannot offer " +
			"to replace it by name")
	}
	if c.ID != r.ID {
		t.Errorf("it offers to replace %q, and this was recorded from %q", c.ID, r.ID)
	}
	if why := a.ReplaceDraft(c.ID, string(orchestrator.ScopeAppAnywhere), ""); why != "" {
		t.Fatalf("the deliberate replacement refused: %s", why)
	}
	if n := len(a.Routines()); n != 1 {
		t.Errorf("replacing one routine left %d in the library", n)
	}
}

// AND CANCEL STILL THROWS IT AWAY WITH NOTHING WRITTEN.
//
// Which is the whole reason Stop and Save are separate buttons, and it has to keep being true for
// a recording started this way: a re-recording somebody looked at and thought better of must leave
// the routine it came from untouched.
func TestCancelAfterRecordAgainLeavesTheRoutineAlone(t *testing.T) {
	a, r, _ := aRoutineWorthRecordingAgain(t)
	before, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	recordingAgain(t, a, r.ID, "arkascended")
	if !a.Recording().Review {
		t.Fatal("Stop did not open review, so there is nothing to cancel")
	}
	if why := a.CancelRecording(); why != "" {
		t.Fatalf("Cancel refused: %s", why)
	}
	after, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Error("cancelling a re-recording changed the routine it came from")
	}
	if n := len(a.Routines()); n != 1 {
		t.Errorf("cancelling left %d routines in the library", n)
	}
}

// A RE-RECORDING MADE IN A DIFFERENT APPLICATION IS SAID, BECAUSE SAVE WOULD SUCCEED.
//
// This is the one case where nothing refuses. Which routine a save writes is decided by the
// application that was OBSERVED, so a re-recording made elsewhere is a different route — no
// collision, no refusal, and a second routine quietly kept beside the one somebody set out to
// replace. Nothing is lost and it is not what they asked for.
func TestARecordingMadeInAnotherApplicationSaysSoBeforeSave(t *testing.T) {
	a, r, _ := aRoutineWorthRecordingAgain(t)
	recordingAgain(t, a, r.ID, "notepad")

	got := a.Recording()
	if got.AgainElsewhere == "" {
		t.Fatal("a re-recording made in another application said nothing, so Save " +
			"would quietly make a second routine")
	}
	for _, must := range []string{"notepad", r.App, "SECOND"} {
		if !strings.Contains(got.AgainElsewhere, must) {
			t.Errorf("the sentence does not mention %q: %q", must,
				got.AgainElsewhere)
		}
	}
	// AND THE ORDINARY CASE SAYS NOTHING. A warning on every re-recording is a
	// warning nobody reads.
	if why := a.CancelRecording(); why != "" {
		t.Fatal(why)
	}
	recordingAgain(t, a, r.ID, "arkascended")
	if got := a.Recording(); got.AgainElsewhere != "" {
		t.Errorf("a re-recording made where the routine lives warned anyway: %q",
			got.AgainElsewhere)
	}
}

// AND A ROUTINE THAT HAS GONE SINCE THE PAGE WAS DRAWN IS NOT RE-RECORDED.
//
// A card can be on screen for a routine somebody has just deleted from a terminal. A recording
// started as a second version of something that is gone would be a window promising a
// replacement it could not offer.
func TestRecordAgainRefusesARoutineThatIsGone(t *testing.T) {
	a, r, _ := aRoutineWorthRecordingAgain(t)
	if err := os.Remove(r.Path); err != nil {
		t.Fatal(err)
	}
	why := a.RecordAgain(r.ID)
	if why == "" {
		t.Fatal("it started a second recording of a routine that is not there")
	}
	if !strings.Contains(why, r.ID) {
		t.Errorf("the refusal does not name what was asked for: %q", why)
	}
	// AND THE WINDOW CAN STILL RECORD SOMETHING ELSE.
	//
	// Asked as the thing it is rather than as `a.capture == nil`, which was this
	// assertion's first form and was wrong: the saved capture from the first recording
	// is still held on purpose, and it is the `saved` half of it that lets another
	// recording start. See StartRecording's fourth refusal.
	owner, ownerApp := uint32(os.Getpid()), winctx.Self()
	a.recording = func() recorder.Recorder {
		return &heldRecorder{events: aDemonstrationIn("notepad", owner, ownerApp)}
	}
	if why := a.StartRecording("something else"); why != "" {
		t.Errorf("a refused Record again left the window unable to record "+
			"anything else: %s", why)
	}
}

// AND AN ORDINARY RECORDING IS STILL AN ORDINARY RECORDING.
//
// Everything above travels on a field that is nil for every other recording. A change that set it
// always would put *this is a second recording of* over somebody's first one.
func TestAnOrdinaryRecordingSaysNothingAboutRecordingAgain(t *testing.T) {
	a, _ := aStudioRecording(t, "ark graphics")
	got := a.Recording()
	if got.Again != "" || got.AgainName != "" || got.AgainSays != "" ||
		got.AgainScope != "" || got.AgainElsewhere != "" {
		t.Errorf("an ordinary recording claims to be a second one: %+v", got)
	}
}

// AND RECORD AGAIN CANNOT START WHILE SOMETHING IS ALREADY HAPPENING.
//
// The same five refusals `StartRecording` has, which is why both go through one function. A second
// entrance that skipped them would let somebody start a recording while a run was in flight — and
// a runner pressing things while the hooks are watching records Marco's own presses as the
// person's.
func TestRecordAgainRefusesWhileSomethingElseIsHappening(t *testing.T) {
	a, r, _ := aRoutineWorthRecordingAgain(t)

	a.mu.Lock()
	a.busy = true
	a.mu.Unlock()
	if why := a.RecordAgain(r.ID); why == "" {
		t.Error("it started a recording while a run was in flight")
	}
	a.mu.Lock()
	a.busy = false
	a.mu.Unlock()

	// AND NOT WHILE SOMETHING IS BEING RECORDED EITHER.
	recordingAgain(t, a, r.ID, "arkascended")
	if why := a.RecordAgain(r.ID); why == "" {
		t.Error("it started a second recording over a draft waiting to be dealt with")
	}
	if why := a.CancelRecording(); why != "" {
		t.Fatal(why)
	}
}
