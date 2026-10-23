package orchestrator

// RECORD → STOP → REVIEW → SAVE, and CANCEL as a real alternative.
//
// The contract a window needs and `LearnAuto` cannot give it: Stop ends the capture and shows
// what it got, Cancel throws it away, Save keeps it. Routing a Stop button through `LearnAuto`
// would make Stop and Save one event, because the routine would already be on disk before
// anybody looked at it.
//
// The acceptance case here is CLICK-ONLY and UNSHIELDED. The strip tests next door shield a
// click from `simplify.foldActivates` with a keystroke, which isolates what they are about and
// proves nothing about a demonstration made of clicks alone — which is the ordinary kind. A
// window a person can glance at while recording has to survive being glanced at.

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aClickOnlyGlanceAtMarco is the whole acceptance case, and every event in it is ordinary.
//
// Two clicks in a game with a look at Marco between them, and the Stop button at the end. No
// keystrokes anywhere: nothing shields the click before the glance from the rule that reads the
// step before an app switch as the gesture that performed the switch.
//
// The poller sees the glance, so there is a switch into Marco in the middle. It then samples
// Marco while the person is already back in the game, so the second click carries the game's
// process and Marco's stale name — the precedence case, arriving here for free because this is
// what the sequence actually produces.
func aClickOnlyGlanceAtMarco() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(105)},
		// ── A LOOK AT MARCO, mid-demonstration. The poller sees it. ──
		{Kind: recorder.EvAppSwitch, KeyName: "studio", T: at(1000)},
		// ── BACK IN THE GAME. The name is stale; the process is not. ──
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 496, Y: 283,
			App: "studio", PID: gamePID, T: at(1500)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 496, Y: 283,
			App: "studio", PID: gamePID, T: at(1505)},
		// ── AND THE STOP BUTTON ──
		{Kind: recorder.EvAppSwitch, KeyName: "studio", T: at(3000)},
		{Kind: recorder.EvMove, X: 1500, Y: 900, T: at(3100)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: "studio", PID: studioPID, T: at(3200)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: "studio", PID: studioPID, T: at(3205)},
	}
}

// recording drives `Record` with the Stop button and returns the draft, saving nothing.
func recording(t *testing.T, events []recorder.RecordedEvent) (Deps, *Draft) {
	t.Helper()
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	press := make(chan struct{})
	d := Deps{
		Reg: reg, Rec: &heldRecorder{events: events}, Out: &strings.Builder{},
		Owner: "studio", OwnerPID: studioPID, StopWhen: press,
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		close(press)
	}()
	dr, err := d.Record("ark graphics")
	if err != nil {
		t.Fatalf("recording: %v", err)
	}
	return d, dr
}

// clicksIn reports the click steps of a draft, which is what review is mostly about.
func clicksIn(steps []macroir.Step) [][2]int {
	var out [][2]int
	for _, s := range steps {
		if s.Kind == macroir.StepClick {
			out = append(out, [2]int{s.X, s.Y})
		}
	}
	return out
}

// STOP ENDS THE CAPTURE AND SAVES NOTHING.
//
// The first claim, because everything else depends on it: if Stop wrote the routine there would
// be nothing to cancel.
func TestStopEndsTheCaptureAndWritesNothing(t *testing.T) {
	d, dr := recording(t, aClickOnlyGlanceAtMarco())

	if dr == nil || len(dr.Steps) == 0 {
		t.Fatal("Stop produced no draft to review")
	}
	entries, err := os.ReadDir(d.Reg.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("Stop wrote %d entr(ies) into the store; Stop is for looking at "+
			"what was recorded, and Save is what keeps it: %v",
			len(entries), entries)
	}
}

// AND AN UNSHIELDED CLICK-ONLY DEMONSTRATION KEEPS BOTH CLICKS AND ITS APP.
//
// The acceptance case. No keystrokes: the click at 842,274 sits immediately before the glance at
// Marco, which is exactly where `simplify.foldActivates` reads a step as the gesture that
// performed the switch. It survives because the owner's switch is not in the stream by the time
// simplify sees it.
func TestAnUnshieldedClickOnlyDemonstrationSurvivesAGlanceAtMarco(t *testing.T) {
	_, dr := recording(t, aClickOnlyGlanceAtMarco())

	got := clicksIn(dr.Steps)
	want := [][2]int{{842, 274}, {496, 283}}
	if len(got) != len(want) {
		t.Fatalf("the draft has clicks %v, want %v. Nothing shields the first "+
			"click from the rule that reads the step before an app switch "+
			"as the gesture that performed it", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("click %d is at %v, demonstrated at %v", i, got[i], want[i])
		}
	}
	// AND THE PRESS OF STOP IS NOT ONE OF THEM, named separately so a draft that
	// kept everything cannot pass the count above.
	for _, c := range got {
		if c == [2]int{1511, 913} {
			t.Error("the click that pressed Stop is a step in the draft")
		}
	}
	// AND THE APP IS THE GAME, not the window Stop was pressed in. This is what
	// decides where the routine is filed and therefore whether it is ever offered.
	if dr.App != "arkascended" {
		t.Errorf("the draft's application is %q; the demonstration was made in "+
			"arkascended and Marco was only looked at", dr.App)
	}
}

// AND CANCEL IS DROPPING THE DRAFT — there is nothing to undo.
//
// Asserted as the absence it is: a cancelled recording leaves the store exactly as it was, and
// it does so because `Record` never wrote to it.
func TestCancelLeavesNothingBehind(t *testing.T) {
	d, dr := recording(t, aClickOnlyGlanceAtMarco())
	dr = nil // what Cancel does
	_ = dr

	entries, err := os.ReadDir(d.Reg.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("a cancelled recording left %d entr(ies) behind", len(entries))
	}
}

// AND SAVE PERSISTS IT, UNDER THE SCOPE IT IS GIVEN.
//
// The scope is stated, not inferred. Each one files the routine somewhere different and only one
// of them keeps the window switch, so a window that asks the question has to be able to answer
// it rather than hope.
func TestSavePersistsTheDraftUnderTheScopeItIsGiven(t *testing.T) {
	for _, c := range []struct {
		scope Scope
		app   string
		focus bool
	}{
		{ScopeApp, "arkascended", false},
		{ScopeAppAnywhere, "arkascended", true},
		{ScopeEverywhere, "", false},
	} {
		t.Run(string(c.scope), func(t *testing.T) {
			d, dr := recording(t, aClickOnlyGlanceAtMarco())
			rt, err := d.Save(dr, c.scope, "")
			if err != nil {
				t.Fatalf("saving as %s: %v", c.scope, err)
			}
			if rt.App != c.app || rt.Focus != c.focus {
				t.Errorf("saved as app=%q focus=%v, want app=%q focus=%v",
					rt.App, rt.Focus, c.app, c.focus)
			}
			raw, err := os.ReadFile(d.Reg.Path(rt))
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
			// AND THE RAW DEMONSTRATION IS BESIDE IT, as review and
			// re-simplification material and nothing else.
			if !d.Reg.HasRecording(rt) {
				t.Error("the raw demonstration was not kept beside the " +
					"routine, so it cannot be reviewed or re-simplified")
			}
		})
	}
}

// AND SAVE REFUSES AN APP SCOPE WITH NO APP, RATHER THAN QUIETLY WIDENING IT.
//
// A routine saved as global when an app scope was asked for works in places the caller was told
// it would not. Refusing is the smaller harm and the honest one.
func TestSaveRefusesAnAppScopeWithNoApp(t *testing.T) {
	d, dr := recording(t, aClickOnlyGlanceAtMarco())
	dr.App = "" // nothing was observed

	if _, err := d.Save(dr, ScopeApp, ""); err == nil {
		t.Fatal("an app-scoped save with no application succeeded; the routine " +
			"would work somewhere the caller was not told about")
	}
	if _, err := d.Save(dr, ScopeEverywhere, ""); err != nil {
		t.Errorf("a global save needs no application and was refused: %v", err)
	}
}

// AND AN EMPTY DRAFT IS NOT SAVED.
//
// A recording nobody made is not a routine. Said rather than written as a file with no steps,
// which would be offered to a person and then do nothing.
func TestSaveRefusesADraftWithNoSteps(t *testing.T) {
	d, _ := recording(t, aClickOnlyGlanceAtMarco())
	if _, err := d.Save(&Draft{Name: "empty"}, ScopeEverywhere, ""); err == nil {
		t.Fatal("a draft with no steps was saved")
	}
	if _, err := d.Save(nil, ScopeEverywhere, ""); err == nil {
		t.Fatal("a nil draft was saved")
	}
}

// WHAT THE EXCLUSION WILL AND WILL NOT CLAIM.
//
// `withoutOwner` removes the owner by PROCESS, read at the instant of each event, plus app
// switches by name because a switch carries no process. The one unattributed event it will take
// is the release of a press it has just removed — same button, same point. Anything else unowned
// stays, because an event it cannot place is not shown to belong to anybody, and guessing the
// other way deletes demonstrations.
func TestWhatTheExclusionWillNotClaim(t *testing.T) {
	// MARCO'S PRESS, whose release is the event under test.
	base := []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(105)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1200, Y: 400,
			App: "arkascended", PID: studioPID, T: at(3000)},
	}
	unowned := func(button string, down bool, x, y int) recorder.RecordedEvent {
		return recorder.RecordedEvent{Kind: recorder.EvClick, Button: button,
			Down: down, X: x, Y: y, App: "arkascended", T: at(3005)}
	}
	for _, c := range []struct {
		what string
		ev   recorder.RecordedEvent
		// kept counts what survives of the four. Marco's own press always goes,
		// so 2 means the trailing event went with it and 3 means it stayed.
		kept int
	}{
		{"the release of that very press", unowned("left", false, 1200, 400), 2},
		{"a release elsewhere", unowned("left", false, 500, 500), 3},
		{"a release of another button", unowned("right", false, 1200, 400), 3},
		{"another press entirely", unowned("left", true, 1200, 400), 3},
	} {
		t.Run(c.what, func(t *testing.T) {
			stream := append(append([]recorder.RecordedEvent(nil), base...), c.ev)
			got := withoutOwner(stream, "studio", studioPID)
			if len(got) != c.kept {
				t.Fatalf("%d of %d event(s) kept, want %d", len(got),
					len(stream), c.kept)
			}
			// AND THE DEMONSTRATION IS WHAT SURVIVED, in every case.
			if got[0].X != 842 || got[1].X != 842 {
				t.Errorf("the demonstrated click is not what was kept: %+v",
					got[:2])
			}
		})
	}
}

// AND A STALE NAME NEVER EXCLUDES ANYTHING.
//
// The exclusion is by process alone for everything but an app switch. A click made in the game
// shortly after looking at Marco carries Marco's name and the game's process, and it is exactly
// the click this must not take.
func TestTheExclusionIgnoresAStaleName(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 496, Y: 283,
			App: "studio", PID: gamePID, T: at(1500)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 496, Y: 283,
			App: "studio", PID: gamePID, T: at(1505)},
	}
	if got := withoutOwner(events, "studio", studioPID); len(got) != 2 {
		t.Errorf("%d of 2 event(s) kept: a click the event itself places in the "+
			"game was excluded because a stale sample named Marco", len(got))
	}
}
