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
	"github.com/chaynes-simpleclouds/marco/internal/simplify"
)

// THE SAME DEMONSTRATION, UNDER THE TWO TIMINGS A GLANCE CAN HAVE.
//
// Two clicks in a game with a look at Marco between them, and the Stop button at the end. No
// keystrokes anywhere: nothing shields a click beside an app switch from the rule that reads the
// step before a switch as the gesture that performed it.
//
// The poller samples the foreground every `appPollInterval` and emits a switch only when the
// sample CHANGES. That single fact makes these two different streams, and both are ordinary:
//
//	the poller saw both sides   a switch into Marco and a switch back out
//	the return was too fast     a switch into Marco and nothing after it, so the next
//	                            click carries Marco's stale name and the game's process
//
// The first is what a glance looks like whenever Marco is looked at for longer than one poll —
// which, for a window with anything to read on it, is every time. The second needs the click to
// land inside the one poll after returning. Neither is exotic and neither may lose a click.
//
// Because the poller emits only on change, a stream can never contain two switches naming the
// same app with nothing between them. A fixture that does is describing a desktop that cannot
// happen, which is how the first version of this hid the ordinary case behind the rare one.
func aGlanceThePollerSawBothSidesOf() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(105)},
		// ── A LOOK AT MARCO, mid-demonstration ──
		{Kind: recorder.EvAppSwitch, KeyName: "studio", T: at(1000)},
		// ── AND THE RETURN, SAMPLED. This is the event that costs a click:
		// with the Marco switch excluded it announces a switch to the window
		// that was in front all along, and the gesture rule reads the click
		// before it as the taskbar press that performed it.
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(1200)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 496, Y: 283,
			App: "arkascended", PID: gamePID, T: at(1500)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 496, Y: 283,
			App: "arkascended", PID: gamePID, T: at(1505)},
		// ── AND THE STOP BUTTON ──
		{Kind: recorder.EvAppSwitch, KeyName: "studio", T: at(3000)},
		{Kind: recorder.EvMove, X: 1500, Y: 900, T: at(3100)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: "studio", PID: studioPID, T: at(3200)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: "studio", PID: studioPID, T: at(3205)},
	}
}

// aGlanceTooFastForThePoller is the other timing: in and out of Marco inside one poll.
//
// The sample stays on Marco throughout, so there is no return switch and no second Marco switch
// either — a switch is emitted only on change, and nothing changed. The click made back in the
// game therefore carries Marco's name and the game's process, and the press of Stop is excluded
// by its process alone, with no switch anywhere to announce it.
func aGlanceTooFastForThePoller() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: gamePID, T: at(105)},
		// ── A LOOK AT MARCO. The poller samples it, and never samples again. ──
		{Kind: recorder.EvAppSwitch, KeyName: "studio", T: at(1000)},
		// ── BACK IN THE GAME WITHIN THE POLL. Stale name, true process. ──
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 496, Y: 283,
			App: "studio", PID: gamePID, T: at(1050)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 496, Y: 283,
			App: "studio", PID: gamePID, T: at(1055)},
		// ── AND BACK TO MARCO FOR STOP, still inside the same sample ──
		{Kind: recorder.EvMove, X: 1500, Y: 900, T: at(3100)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: "studio", PID: studioPID, T: at(3200)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: "studio", PID: studioPID, T: at(3205)},
	}
}

// theTwoTimings pairs them up, so every claim about a glance is made about both.
func theTwoTimings() []struct {
	what   string
	events []recorder.RecordedEvent
} {
	return []struct {
		what   string
		events []recorder.RecordedEvent
	}{
		{"the poller saw both sides", aGlanceThePollerSawBothSidesOf()},
		{"the return was too fast to sample", aGlanceTooFastForThePoller()},
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

// activatedIn reports the applications a draft would switch to, which is an executable action
// like any other and the one a glance at the recorder tends to leave behind.
func activatedIn(steps []macroir.Step) []string {
	var out []string
	for _, s := range steps {
		if s.Kind == macroir.StepActivate {
			out = append(out, s.Text)
		}
	}
	return out
}

// STOP ENDS THE CAPTURE AND SAVES NOTHING.
//
// The first claim, because everything else depends on it: if Stop wrote the routine there would
// be nothing to cancel.
func TestStopEndsTheCaptureAndWritesNothing(t *testing.T) {
	d, dr := recording(t, aGlanceThePollerSawBothSidesOf())

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

// AND AN UNSHIELDED CLICK-ONLY DEMONSTRATION KEEPS BOTH CLICKS AND ITS APP, EITHER WAY.
//
// The acceptance case, made about both timings. No keystrokes anywhere, so each click sits
// unshielded beside an app switch — which is exactly where `simplify.foldActivates` reads a step
// as the gesture that performed the switch.
//
// Where the two timings put the click at risk differs, and that is the point of running both:
//
//	poller saw both sides   the Marco switch follows the first click, and the sampled return
//	                        follows it too once Marco's is excluded — so the FIRST click is
//	                        the one beside an activate
//	return too fast         no return was sampled, so nothing follows either click and the
//	                        first click is safe for a different reason
//
// A fixture with only the second timing passes with the normalization deleted. That is how the
// ordinary case stayed hidden.
func TestAnUnshieldedClickOnlyDemonstrationSurvivesAGlanceAtMarco(t *testing.T) {
	for _, c := range theTwoTimings() {
		t.Run(c.what, func(t *testing.T) {
			_, dr := recording(t, c.events)

			got := clicksIn(dr.Steps)
			want := [][2]int{{842, 274}, {496, 283}}
			if len(got) != len(want) {
				t.Fatalf("the draft has clicks %v, want %v. Nothing "+
					"shields these clicks from the rule that reads "+
					"the step before an app switch as the gesture "+
					"that performed it", got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("click %d is at %v, demonstrated at %v",
						i, got[i], want[i])
				}
			}
			// AND MARCO CONTRIBUTES NO EXECUTABLE ACTION AT ALL — not the
			// click that pressed Stop, and not an Activate bringing Marco
			// forward. Named separately, because a draft that kept
			// everything would pass the count above, and because an
			// Activate is an action too: a routine that focuses the
			// recorder on its way through is a routine that lost its game.
			for _, p := range got {
				if p == [2]int{1511, 913} {
					t.Error("the click that pressed Stop is a step " +
						"in the draft")
				}
			}
			for _, app := range activatedIn(dr.Steps) {
				if app == "studio" {
					t.Error("the draft activates Marco; looking at " +
						"the recorder is not part of the " +
						"demonstration")
				}
			}
			// AND THE APP IS THE GAME, not the window Stop was pressed in.
			// This decides where the routine is filed and therefore whether
			// it is ever offered.
			if dr.App != "arkascended" {
				t.Errorf("the draft's application is %q; the "+
					"demonstration was made in arkascended and "+
					"Marco was only looked at", dr.App)
			}
			// AND SAVE FILES IT UNDER THE STATED GAME SCOPE, with the whole
			// demonstration in it. The draft surviving review is worth
			// nothing if what reaches disk is shorter than what was shown.
			d, dr := recording(t, c.events)
			rt, err := d.Save(dr, ScopeApp, "")
			if err != nil {
				t.Fatalf("saving under the game scope: %v", err)
			}
			if rt.App != "arkascended" {
				t.Errorf("saved under app %q, want arkascended", rt.App)
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
			if strings.Contains(src, "1511") || strings.Contains(src, "studio") {
				t.Errorf("Marco reached the saved routine:\n%s", src)
			}
		})
	}
}

// AND CANCEL IS DROPPING THE DRAFT — there is nothing to undo.
//
// Asserted as the absence it is: a cancelled recording leaves the store exactly as it was, and
// it does so because `Record` never wrote to it.
func TestCancelLeavesNothingBehind(t *testing.T) {
	d, dr := recording(t, aGlanceThePollerSawBothSidesOf())
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
			d, dr := recording(t, aGlanceThePollerSawBothSidesOf())
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
	d, dr := recording(t, aGlanceThePollerSawBothSidesOf())
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
	d, _ := recording(t, aGlanceThePollerSawBothSidesOf())
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
// switches by name because a switch carries no process. A click is paired first and decided as
// one act, so the release of a press it removes goes with it. Anything it cannot pair stays,
// because an event it cannot place is not shown to belong to anybody, and guessing the other way
// deletes demonstrations.
//
// # The case that changed, and why
//
// "A release elsewhere" used to be KEPT: the old rule took the release of an owner's press only
// when it was unattributed AND at the identical point, so a release that had moved was left
// behind. That is where the defect lived. A button cannot be released twice without being
// pressed again, so the release after a press IS that press's release — one that moved is a drag
// on the owner's window, not a different click — and leaving it behind is leaving half a click
// in the stream for `simplify` to pair with the next one. See withoutOwner.
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
		// THE RELEASE OF THAT PRESS, HAVING MOVED. Still that press's release:
		// the button was not pressed again in between, so there is nothing else
		// it could belong to. Kept, it is half a click left in the stream.
		{"a release elsewhere", unowned("left", false, 500, 500), 2},
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

// AND WHAT THE RETURN NORMALIZATION WILL NOT CLAIM.
//
// It removes a switch naming the app already in front, and nothing else. A real excursion into a
// third application ends in a switch back that is NOT redundant — something else was in front —
// and taking that one would leave the routine running the rest of the demonstration against
// whatever window the excursion left behind.
//
// Driven directly, because no draft fixture produces a three-app excursion: through `Record` the
// whole clause could be a constant and the acceptance tests above would still pass.
func TestTheReturnNormalizationKeepsARealSwitchBack(t *testing.T) {
	click := func(x, y, ms int) recorder.RecordedEvent {
		return recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left",
			Down: true, X: x, Y: y, PID: gamePID, T: at(ms)}
	}
	sw := func(app string, ms int) recorder.RecordedEvent {
		return recorder.RecordedEvent{Kind: recorder.EvAppSwitch, KeyName: app,
			T: at(ms)}
	}
	for _, c := range []struct {
		what string
		in   []recorder.RecordedEvent
		want []string
	}{
		{
			// THE CASE IT IS FOR: the owner's switch has already been
			// excluded, so the return names the app that never left.
			"a return to the app already in front",
			[]recorder.RecordedEvent{sw("arkascended", 0), click(842, 274, 100),
				sw("arkascended", 1200), click(496, 283, 1500)},
			[]string{"arkascended"},
		},
		{
			// AND THE CASE IT IS NOT FOR: a real trip through a third app.
			"a return from a real excursion",
			[]recorder.RecordedEvent{sw("arkascended", 0), click(842, 274, 100),
				sw("notepad", 1200), click(300, 300, 1300),
				sw("arkascended", 2000), click(496, 283, 2500)},
			[]string{"arkascended", "notepad", "arkascended"},
		},
		{
			// AND A RECORDING THAT OPENS WITH NO SWITCH AT ALL keeps the
			// first one it gets: there is nothing yet for it to repeat.
			"the first switch of a recording that opened without one",
			[]recorder.RecordedEvent{click(842, 274, 100), sw("arkascended", 1200)},
			[]string{"arkascended"},
		},
	} {
		t.Run(c.what, func(t *testing.T) {
			got := withoutRedundantReturns(c.in)
			var apps []string
			clicks := 0
			for _, e := range got {
				switch e.Kind {
				case recorder.EvAppSwitch:
					apps = append(apps, e.KeyName)
				case recorder.EvClick:
					clicks++
				}
			}
			if strings.Join(apps, ",") != strings.Join(c.want, ",") {
				t.Errorf("the switches left are %v, want %v", apps, c.want)
			}
			// AND NO CLICK WAS TOUCHED. This pass is about switches; a
			// demonstration lost here would be lost for the same reason the
			// gesture rule loses one, which is the thing it exists to stop.
			if clicks != countClicks(c.in) {
				t.Errorf("%d of %d click(s) survived a pass that is only "+
					"about app switches", clicks, countClicks(c.in))
			}
		})
	}
}

// countClicks is the arithmetic-free way to say how many clicks a stream has.
func countClicks(events []recorder.RecordedEvent) int {
	n := 0
	for _, e := range events {
		if e.Kind == recorder.EvClick {
			n++
		}
	}
	return n
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

// ── A CLICK'S TWO EDGES CAN BELONG TO TWO PROCESSES ───────────────────────────
//
// Which is how the owner's first mouse-driven trial produced `drag from -740, 348 to 50, 374`
// out of two ordinary clicks, and why every fixture above missed it: they all give both edges of
// a click the same process id, and a real desktop does not.
//
// The press that RAISES a window arrives while the previous window is still in front — clicking
// a window is what causes the activation. So a click on Marco from the application is attributed
// (press, release) = (application, Marco), and the click back is (Marco, application). Excluded
// edge by edge, each click lost one half, and the halves left behind were a press on Marco and a
// release in the application with no press between them: a drag, by every rule `simplify` has.
// A drag cannot be saved in a routine whose presses are checked, so looking at Marco during a
// recording made the whole recording unsaveable.

// clickAcross is one click whose press and release were attributed to different processes, which
// is what a click that changes the foreground looks like from inside the hook.
func clickAcross(button string, x, y int, pressPID, releasePID uint32,
	ms int) []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: button, Down: true, X: x, Y: y,
			PID: pressPID, T: at(ms)},
		{Kind: recorder.EvClick, Button: button, Down: false, X: x, Y: y,
			PID: releasePID, T: at(ms + 5)},
	}
}

// demonstratedClick is an ordinary click in the application, both edges its own.
func demonstratedClick(x, y int, ms int) []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: "left", Down: true, X: x, Y: y,
			App: "arkascended", PID: gamePID, T: at(ms)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: x, Y: y,
			App: "arkascended", PID: gamePID, T: at(ms + 5)},
	}
}

func clickEdges(events []recorder.RecordedEvent) []recorder.RecordedEvent {
	var out []recorder.RecordedEvent
	for _, e := range events {
		if e.Kind == recorder.EvClick {
			out = append(out, e)
		}
	}
	return out
}

// CLICKING INTO MARCO AND BACK LEAVES NO HALF OF EITHER CLICK, AND TAKES NEITHER DEMONSTRATED ONE.
//
// The live case, as the hooks see it: a demonstrated click, a click onto Marco's window whose
// release is Marco's, a click back whose press is Marco's, and a second demonstrated click. Both
// of the middle clicks cross the foreground boundary in opposite directions.
func TestAClickOntoTheRecorderAndBackLeavesNoHalfBehind(t *testing.T) {
	var stream []recorder.RecordedEvent
	stream = append(stream, recorder.RecordedEvent{Kind: recorder.EvAppSwitch,
		KeyName: "arkascended", T: at(0)})
	stream = append(stream, demonstratedClick(842, 274, 100)...)
	// ONTO MARCO: the press is still the game's, the release is Marco's.
	stream = append(stream, recorder.RecordedEvent{Kind: recorder.EvMove, X: -700,
		Y: 340, T: at(1000)})
	stream = append(stream, clickAcross("left", -740, 348, gamePID, studioPID, 1100)...)
	stream = append(stream, recorder.RecordedEvent{Kind: recorder.EvAppSwitch,
		KeyName: "studio", T: at(1200)})
	// AND BACK: the press is Marco's, the release is the game's.
	stream = append(stream, recorder.RecordedEvent{Kind: recorder.EvMove, X: 40,
		Y: 370, T: at(2000)})
	stream = append(stream, clickAcross("left", 50, 374, studioPID, gamePID, 2100)...)
	stream = append(stream, recorder.RecordedEvent{Kind: recorder.EvAppSwitch,
		KeyName: "arkascended", T: at(2200)})
	stream = append(stream, demonstratedClick(496, 283, 3000)...)

	kept := clickEdges(withoutOwner(stream, "studio", studioPID))
	if len(kept) != 4 {
		t.Fatalf("%d click edge(s) survived, want the 4 of the two demonstrated "+
			"clicks: %+v", len(kept), kept)
	}
	// BOTH DEMONSTRATED CLICKS, WHOLE.
	for i, want := range []struct {
		x, y int
		down bool
	}{{842, 274, true}, {842, 274, false}, {496, 283, true}, {496, 283, false}} {
		if kept[i].X != want.x || kept[i].Y != want.y || kept[i].Down != want.down {
			t.Errorf("edge %d is %+v, want %d,%d down=%v", i, kept[i],
				want.x, want.y, want.down)
		}
	}
	// AND NO PHANTOM GESTURE COMES OUT OF WHAT IS LEFT, which is the thing the
	// person actually saw. Through the production simplifier, because that is what
	// read the halves as a drag.
	for _, s := range simplify.Simplify(withoutOwner(stream, "studio", studioPID),
		simplify.Options{}) {
		if s.Kind == macroir.StepDrag {
			t.Fatalf("a drag was invented out of two clicks: from %d,%d to "+
				"%d,%d", s.X, s.Y, s.X2, s.Y2)
		}
	}
}

// AND THE STOP BUTTON'S OWN ACTIVATION CLICK REACHES NOTHING.
//
// Pressing Stop brings Marco forward and clicks it in one gesture, so the press is the
// application's and the release is Marco's. Neither half may survive: the press is at Marco's
// button, which is nowhere in the demonstration.
func TestTheStopButtonsActivationClickReachesNeitherHalf(t *testing.T) {
	var stream []recorder.RecordedEvent
	stream = append(stream, demonstratedClick(842, 274, 100)...)
	stream = append(stream, clickAcross("left", 1511, 913, gamePID, studioPID, 4200)...)

	kept := clickEdges(withoutOwner(stream, "studio", studioPID))
	if len(kept) != 2 {
		t.Fatalf("%d click edge(s) survived, want the 2 of the demonstrated "+
			"click: %+v", len(kept), kept)
	}
	for _, e := range kept {
		if e.X == 1511 {
			t.Errorf("half of the Stop click is in the demonstration: %+v", e)
		}
	}
}

// AND A CLICK THAT OPENS ANOTHER APPLICATION'S WINDOW IS KEPT.
//
// The same crossed attribution, and nothing to do with Marco: a press in the game whose release
// lands in a window the game itself raised. Neither edge is the owner's, so it is a demonstrated
// click and it stays. Excluding by "the edges disagree" rather than by "one of them is Marco's"
// would have taken it.
func TestAClickThatRaisesAnotherWindowIsKept(t *testing.T) {
	const launcherPID uint32 = 9191
	stream := clickAcross("left", 600, 500, gamePID, launcherPID, 100)
	kept := clickEdges(withoutOwner(stream, "studio", studioPID))
	if len(kept) != 2 {
		t.Fatalf("%d of 2 edge(s) kept: a demonstrated click was excluded because "+
			"its two edges were attributed to different programs, neither of "+
			"them Marco: %+v", len(kept), kept)
	}
}

// AND NOTHING UNMATCHED IS EVER PAIRED INTO A GESTURE THAT WAS NOT PERFORMED.
//
// Two rules, and the fixtures are what a lost event and interleaved buttons look like. A press
// whose release never came must not borrow the next click's release, and a left click's edges
// must not pair with a right click's.
func TestNothingUnmatchedIsPairedIntoAnInventedGesture(t *testing.T) {
	t.Run("a press whose release was lost", func(t *testing.T) {
		stream := []recorder.RecordedEvent{
			// THE LOST ONE, in the game.
			{Kind: recorder.EvClick, Button: "left", Down: true, X: 100, Y: 100,
				PID: gamePID, T: at(100)},
			// AND A WHOLE CLICK ON MARCO AFTERWARDS. Its release must not be
			// taken as the first press's, which would make that press Marco's
			// and delete a demonstrated event.
			{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511,
				Y: 913, PID: studioPID, T: at(200)},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511,
				Y: 913, PID: studioPID, T: at(205)},
		}
		kept := clickEdges(withoutOwner(stream, "studio", studioPID))
		if len(kept) != 1 || kept[0].X != 100 {
			t.Fatalf("the unmatched demonstrated press did not survive on its "+
				"own: %+v", kept)
		}
	})
	t.Run("interleaved buttons", func(t *testing.T) {
		// A LEFT CLICK ON MARCO, with a right click in the game threaded
		// through it. Pairing by order and not by button would marry the left
		// press to the right release and take the game's click with it.
		stream := []recorder.RecordedEvent{
			{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511,
				Y: 913, PID: studioPID, T: at(100)},
			{Kind: recorder.EvClick, Button: "right", Down: true, X: 300,
				Y: 300, PID: gamePID, T: at(110)},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511,
				Y: 913, PID: studioPID, T: at(120)},
			{Kind: recorder.EvClick, Button: "right", Down: false, X: 300,
				Y: 300, PID: gamePID, T: at(130)},
		}
		kept := clickEdges(withoutOwner(stream, "studio", studioPID))
		if len(kept) != 2 {
			t.Fatalf("%d of 2 edge(s) kept: %+v", len(kept), kept)
		}
		for _, e := range kept {
			if e.Button != "right" || e.X != 300 {
				t.Errorf("what survived is not the game's right click: %+v",
					e)
			}
		}
	})
	t.Run("a release whose press was lost", func(t *testing.T) {
		// AN ORPHAN RELEASE IN THE GAME, after a whole click on Marco. The
		// release is not Marco's and cannot be placed, so it stays — and it
		// must not have taken Marco's press as its own on the way through.
		stream := []recorder.RecordedEvent{
			{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511,
				Y: 913, PID: studioPID, T: at(100)},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511,
				Y: 913, PID: studioPID, T: at(105)},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 400,
				Y: 400, PID: gamePID, T: at(200)},
		}
		kept := clickEdges(withoutOwner(stream, "studio", studioPID))
		if len(kept) != 1 || kept[0].X != 400 {
			t.Fatalf("the orphan release did not survive on its own: %+v", kept)
		}
	})
}
