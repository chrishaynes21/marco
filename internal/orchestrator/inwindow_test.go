package orchestrator

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// THE CLOCK OF A RECORDING THAT WAITED, AND WHAT A DRAFT SAYS ABOUT IT.
//
// Through `Finish`, which is the production entrance: Studio's Stop button, `Learn` and
// `LearnAuto` all end up there, and a test that read `inWindowClock` alone would pass for the
// whole time the call was missing from the function a person goes through. The pure function is
// asked directly too, for the arithmetic — but the wiring is asked here.

// ── THE ARITHMETIC ───────────────────────────────────────────────────────────

// in-window time is preserved and away time is not, which is the whole rule in one case.
func TestTheClockKeepsTimeInTheWindowAndDropsTimeAway(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvLeft, T: at(0)}, // pressing Record and walking over
		{Kind: recorder.EvEntered, T: at(4000)},
		{Kind: recorder.EvKey, KeyName: "a", Down: true, T: at(4000)},
		// A DELIBERATE PAUSE IN THE WINDOW: two seconds the person meant.
		{Kind: recorder.EvKey, KeyName: "b", Down: true, T: at(6000)},
		// AND THIRTY SECONDS SOMEWHERE ELSE.
		{Kind: recorder.EvLeft, T: at(6100)},
		{Kind: recorder.EvEntered, T: at(36100)},
		{Kind: recorder.EvKey, KeyName: "c", Down: true, T: at(36200)},
	}
	st := inWindowClock(events, 250*time.Millisecond)
	if len(st.Events) != 3 {
		t.Fatalf("the recording came back with %d events, not the three that happened "+
			"in the window: %+v", len(st.Events), st.Events)
	}
	// THE PAUSE INSIDE IS UNTOUCHED.
	if gap := st.Events[1].T.Sub(st.Events[0].T); gap != 2*time.Second {
		t.Errorf("the pause inside the window became %v, not the 2s it was", gap)
	}
	// AND THE ABSENCE COSTS THE SETTLE AND NOTHING MORE. The gap runs from "b" at 6000
	// to "c" at 36200 and is made of three things: the 100ms in the window before
	// leaving, the 250ms settle standing in for the 30s away, and the 100ms in the
	// window after coming back. Time inside is kept on BOTH sides of an absence, which
	// is what "preserve time spent inside" means when the absence is in the middle.
	if gap := st.Events[2].T.Sub(st.Events[1].T); gap != 450*time.Millisecond {
		t.Errorf("the gap across a 30s absence is %v, not the 100ms inside, the "+
			"250ms settle and the 100ms inside that it is made of", gap)
	}
	// AND THE LEADING ABSENCE IS NOT COUNTED AS ONE, because the person had not
	// arrived: they pressed Record and went to the window, which is how a recording
	// starts rather than an interruption of one.
	if st.Absences != 1 || st.Away != 30*time.Second {
		t.Errorf("review would say %v over %d absences, not 30s over 1",
			st.Away, st.Absences)
	}
}

// A SHORT ABSENCE IS NOT STRETCHED, which is the other direction of the same arithmetic: the
// settle is a ceiling on what the absence costs, not a floor under it.
func TestAnAbsenceShorterThanTheSettleIsLeftAlone(t *testing.T) {
	st := inWindowClock([]recorder.RecordedEvent{
		{Kind: recorder.EvKey, KeyName: "a", Down: true, T: at(0)},
		{Kind: recorder.EvLeft, T: at(10)},
		{Kind: recorder.EvEntered, T: at(60)},
		{Kind: recorder.EvKey, KeyName: "b", Down: true, T: at(100)},
	}, 250*time.Millisecond)
	if gap := st.Events[1].T.Sub(st.Events[0].T); gap != 100*time.Millisecond {
		t.Errorf("a 50ms absence turned a 100ms gap into %v", gap)
	}
}

// A TRAILING ABSENCE ADDS NOTHING. Somebody who walks away and stops the recording from
// somewhere else has not asked the routine to wait at the end.
func TestATrailingAbsenceAddsNoTimeAtAll(t *testing.T) {
	st := inWindowClock([]recorder.RecordedEvent{
		{Kind: recorder.EvKey, KeyName: "a", Down: true, T: at(0)},
		{Kind: recorder.EvKey, KeyName: "b", Down: true, T: at(100)},
		{Kind: recorder.EvLeft, T: at(200)},
	}, 250*time.Millisecond)
	if len(st.Events) != 2 {
		t.Fatalf("%d events came back, not 2", len(st.Events))
	}
	if gap := st.Events[1].T.Sub(st.Events[0].T); gap != 100*time.Millisecond {
		t.Errorf("the recording's last gap became %v", gap)
	}
	if st.Absences != 0 {
		t.Errorf("walking away at the end counted as %d absences", st.Absences)
	}
}

// AND NO MARKER COMES BACK, which is the compatibility answer: a stored recording holds only
// the kinds it always held, so there is nothing in it for an older build to interpret
// differently and no version for it to refuse.
func TestNoCaptureMarkerSurvivesIntoTheRecording(t *testing.T) {
	st := inWindowClock([]recorder.RecordedEvent{
		{Kind: recorder.EvLeft, T: at(0)},
		{Kind: recorder.EvEntered, T: at(100)},
		{Kind: recorder.EvKey, KeyName: "a", Down: true, T: at(200)},
		{Kind: recorder.EvCrossed, KeyName: "ctrl was held", T: at(300)},
		{Kind: recorder.EvTargetGone, T: at(400)},
	}, 250*time.Millisecond)
	for _, ev := range st.Events {
		if ev.Kind.Marker() {
			t.Errorf("a %v marker is in the recording that would be stored, where "+
				"an older build would ignore it and keep the raw gap", ev.Kind)
		}
	}
	if st.Crossing != "ctrl was held" || !st.Gone {
		t.Errorf("the markers were dropped without being read: %+v", st)
	}
}

// ── AND THE SAME THING THROUGH THE FUNCTION A PERSON GOES THROUGH ────────────

// confinedRecorder is a recorder that can be told which window to record, and says what it was
// told. It publishes nothing and closes its stream, which ends `waitForStop` at once — these
// cases are about what `Finish` makes of a recording, not about how it ended.
type confinedRecorder struct {
	events []recorder.RecordedEvent
	ch     chan recorder.RecordedEvent
	// told is what `Begin` stated, window is WHICH window it stated, and asked is
	// whether it stated anything at all.
	told   recorder.Watching
	window recorder.Target
	asked  bool
}

func (c *confinedRecorder) Start() error {
	c.ch = make(chan recorder.RecordedEvent, len(c.events))
	close(c.ch)
	return nil
}
func (c *confinedRecorder) Stop() []recorder.RecordedEvent        { return c.events }
func (c *confinedRecorder) Events() <-chan recorder.RecordedEvent { return c.ch }
func (c *confinedRecorder) Only(w recorder.Watching, t recorder.Target) {
	c.told, c.window, c.asked = w, t, true
}

// theGameWindow is a window somebody chose: a handle and the process that owns it, which is
// what a recording is confined to. Both, because either alone admits a window nobody picked.
var theGameWindow = recorder.Target{HWnd: 0x3333, PID: 7001}

// aDemonstrationWithAnAbsence is two clicks in a game with half a minute away in between.
func aDemonstrationWithAnAbsence() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvLeft, T: at(0)},
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(4000)},
		{Kind: recorder.EvEntered, T: at(4000)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", T: at(4100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", T: at(4150)},
		{Kind: recorder.EvLeft, T: at(5000)},
		{Kind: recorder.EvEntered, T: at(35000)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 900, Y: 300,
			App: "arkascended", T: at(35100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 900, Y: 300,
			App: "arkascended", T: at(35150)},
	}
}

// THE DEFAULT IS ONE WINDOW, AND IT IS STATED BEFORE THE RECORDER STARTS.
//
// The mutation this kills is the one that matters most in this slice: delete the `Only` call
// from `Begin` and every other test here still passes, because they all hand `Finish` a
// recording that was already gated. What the person would get is the whole desktop.
func TestTheRecordingIsConfinedToOneWindowUnlessTheCallerSaysOtherwise(t *testing.T) {
	rec := &confinedRecorder{}
	d := Deps{Rec: rec, Window: theGameWindow, OwnerPID: 4812}
	if _, err := d.Begin("ark graphics"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if !rec.asked {
		t.Fatal("the recorder was never told which window to record, so a " +
			"demonstration records the whole desktop")
	}
	if rec.told != recorder.OneWindow {
		t.Errorf("the recorder was told %v, not one window", rec.told)
	}
	// AND THE WINDOW SOMEBODY CHOSE REACHES IT, handle and owner both. Either one
	// dropped on the way admits a window nobody picked: the handle because Windows
	// reuses them, the owner because two windows of one application are two windows.
	if rec.window != theGameWindow {
		t.Errorf("the recorder was told to record %+v, not the window that was "+
			"chosen (%+v)", rec.window, theGameWindow)
	}
	// AND THE WHOLE DESKTOP IS STILL AVAILABLE, said explicitly — the control without
	// which "one window" could be a recorder that ignores what it is told.
	rec2 := &confinedRecorder{}
	d2 := Deps{Rec: rec2, Watch: WholeDesktop}
	if _, err := d2.Begin("across two apps"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if rec2.told != recorder.WholeDesktop {
		t.Errorf("a caller that asked for the whole desktop was told %v", rec2.told)
	}
}

// THE DRAFT A PERSON REVIEWS HAS THE IN-WINDOW CLOCK IN IT, and says how long they were away.
func TestADraftExcludesTheTimeSpentAwayAndSaysSo(t *testing.T) {
	rec := &confinedRecorder{events: aDemonstrationWithAnAbsence()}
	d := Deps{Rec: rec, Window: theGameWindow}
	c, err := d.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	dr := c.Finish()
	if dr.Absences != 1 || dr.Away != 30*time.Second {
		t.Errorf("the draft says %v away over %d absences, not 30s over 1",
			dr.Away, dr.Absences)
	}
	// NO WAIT LONGER THAN THE SETTLE SURVIVED INTO THE STEPS, which is the behaviour
	// the clock exists for: a routine that paused for half a minute on every run
	// because somebody looked something up while recording it.
	for _, s := range dr.Steps {
		if s.Kind == macroir.StepWait && time.Duration(s.Ms)*time.Millisecond > ResumeSettle {
			t.Errorf("the routine waits %dms, which is the time the person spent "+
				"outside the window", s.Ms)
		}
	}
	// AND THE MARKERS ARE NOT IN WHAT WOULD BE STORED.
	for _, ev := range dr.Events {
		if ev.Kind.Marker() {
			t.Errorf("a %v marker is in the draft's recording", ev.Kind)
		}
	}
	if dr.Unrepresentable != "" {
		t.Errorf("a recording with a plain absence in it cannot be saved: %q",
			dr.Unrepresentable)
	}
}

// A CROSSING REFUSES THE SAVE, AND THE DRAFT STILL EXISTS TO BE LOOKED AT.
func TestACrossingDraftIsKeptForReviewAndRefusedAtSave(t *testing.T) {
	events := aDemonstrationWithAnAbsence()
	events = append(events, recorder.RecordedEvent{Kind: recorder.EvCrossed,
		KeyName: "ctrl was held down from outside the window", T: at(35200)})
	rec := &confinedRecorder{events: events}
	// SOMEWHERE OF ITS OWN TO WRITE TO, because this calls `Save` and expects a
	// refusal: an empty `Registry.Dir` joins to a RELATIVE path, so a save that should
	// have been refused and was not lands in the package directory. That is not
	// hypothetical — the mutation run for this slice left a routine there.
	d := Deps{Rec: rec, Window: theGameWindow, Reg: routes.Registry{Dir: t.TempDir()}}
	c, err := d.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	dr := c.Finish()
	// THE STEPS ARE THERE: the person is the only one who can perform the gesture
	// again differently, and "nothing happened" would be a worse answer.
	if dr.Pressable() == 0 {
		t.Error("a draft refused for a crossing has no steps in it to look at")
	}
	if !strings.Contains(dr.Unrepresentable, "ctrl") {
		t.Errorf("the draft's refusal is %q, which does not say what crossed",
			dr.Unrepresentable)
	}
	// AND SAVE REFUSES IT, which is where the refusal has to bite.
	if _, err := d.Save(dr, ScopeApp, "arkascended"); err == nil {
		t.Error("a draft with half a gesture in it saved")
	} else if !strings.Contains(err.Error(), "ctrl") {
		t.Errorf("Save refused it for some other reason: %v", err)
	}
}

// AND THE WINDOW CLOSING ENDS THE RECORDING AND SAYS THAT IS WHAT HAPPENED.
func TestAClosedWindowEndsTheRecordingAndIsNamedOnTheDraft(t *testing.T) {
	events := append(aDemonstrationWithAnAbsence(),
		recorder.RecordedEvent{Kind: recorder.EvTargetGone, T: at(36000)})
	rec := &confinedRecorder{events: events}
	d := Deps{Rec: rec, Window: theGameWindow}
	c, err := d.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	dr := c.Finish()
	if dr.StoppedBy != "the window closing" {
		t.Errorf("the draft says it was stopped by %q", dr.StoppedBy)
	}
	// AND WHAT WAS RECORDED IS KEPT, because review opens on it.
	if dr.Pressable() == 0 {
		t.Error("the draft of a recording whose window closed has no steps in it")
	}
}

// A WINDOW THAT CLOSES MID-GESTURE REFUSES THE SAVE, which is the two rules meeting: the draft
// is kept for review, and a gesture the closing left unfinished is still unrepresentable.
func TestAWindowThatClosesMidGestureRefusesTheSave(t *testing.T) {
	events := aDemonstrationWithAnAbsence()
	events = append(events,
		recorder.RecordedEvent{Kind: recorder.EvCrossed,
			KeyName: "the left button was still held when the window was left",
			T:       at(35900)},
		recorder.RecordedEvent{Kind: recorder.EvTargetGone, T: at(36000)})
	rec := &confinedRecorder{events: events}
	d := Deps{Rec: rec, Window: theGameWindow, Reg: routes.Registry{Dir: t.TempDir()}}
	c, err := d.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	dr := c.Finish()
	if dr.StoppedBy != "the window closing" {
		t.Errorf("the draft says it was stopped by %q", dr.StoppedBy)
	}
	if _, err := d.Save(dr, ScopeApp, "arkascended"); err == nil {
		t.Error("a recording whose window closed mid-gesture saved anyway")
	}
}

// A CROSSING IS THE REASON GIVEN, EVEN WHEN SIMPLIFY HAS ONE OF ITS OWN.
//
// Both refuse the save, so a test with only one of them passes whichever order the two are
// asked in. The order is not arbitrary: a gesture whose other half happened in another window
// is not a recording with a problem in it, it is a recording of something that was not done
// here, and that is what the person has to be told to know what to do differently.
func TestTheCrossingIsNamedAheadOfSimplifysOwnRefusal(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", T: at(200)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", T: at(250)},
		// A MODIFIER RELEASED WITH NO PRESS, AFTER A CLICK, which is what
		// `simplify.modifierTrouble` refuses on its own account: it was held when
		// recording began and there is no instant to press it at.
		{Kind: recorder.EvKey, KeyName: "ctrl", Down: false, App: "arkascended",
			T: at(300)},
		{Kind: recorder.EvCrossed,
			KeyName: "shift was still held when the window was left", T: at(400)},
	}
	rec := &confinedRecorder{events: events}
	d := Deps{Rec: rec, Window: theGameWindow}
	c, err := d.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	dr := c.Finish()
	if !strings.Contains(dr.Unrepresentable, "shift") {
		t.Errorf("the draft is refused for %q, which is not the crossing",
			dr.Unrepresentable)
	}
	// AND THE CONTROL: without the crossing, simplify's own refusal is still the
	// answer — so this is an ordering and not a crossing that swallows everything.
	plain := &confinedRecorder{events: events[:len(events)-1]}
	c2, err := Deps{Rec: plain, Window: theGameWindow}.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if why := c2.Finish().Unrepresentable; why == "" {
		t.Error("simplify's own refusal was lost")
	} else if strings.Contains(why, "shift") {
		t.Errorf("a recording with no crossing in it was refused for one: %q", why)
	}
}

// AND THE WINDOW CLOSING ENDS A RECORDING NOBODY HAS STOPPED.
//
// `confinedRecorder` closes its stream, which ends `waitForStop` by itself — so the cases above
// could not ask this: a recording that stops because the events ran out is not a recording that
// was stopped by anything. `heldRecorder` keeps the stream open, the way a real recorder does
// while somebody is still demonstrating, so what ends this one is the marker.
func TestTheChosenWindowClosingEndsARecordingNobodyStopped(t *testing.T) {
	rec := &heldRecorder{events: []recorder.RecordedEvent{
		{Kind: recorder.EvTargetGone, T: at(9000)},
	}}
	d := Deps{Rec: rec, Window: theGameWindow, StopKey: recorder.DefaultStopKey}
	c, err := d.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	done := make(chan *Draft, 1)
	go func() { done <- c.Finish() }()
	select {
	case dr := <-done:
		if dr.StoppedBy != "the window closing" {
			t.Errorf("the draft says it was stopped by %q", dr.StoppedBy)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the recording never ended, so closing the window being recorded " +
			"leaves the capture running with nowhere to record into")
	}
}

// AND REVIEW SAYS WHY THE DRAFT IS SHORTER THAN THE DEMONSTRATION.
//
// Both surfaces, because a draft that has fewer steps than the person performed and a clock that
// no longer matches what happened are both invisible otherwise: the only available reading is
// that the recorder missed something. This is the terminal; `plugins/studio`
// `TestReviewSaysTheWindowWasLeft` is the window.
func TestTheTerminalSaysTheWindowWasLeft(t *testing.T) {
	// THE SENTENCE ITSELF: how often, how long, and that the wait and the input are both
	// left out of the routine.
	rec := &confinedRecorder{events: aDemonstrationWithAnAbsence()}
	c, err := Deps{Rec: rec, Window: theGameWindow}.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	said := c.Finish().AwaySentence()
	for _, want := range []string{"Left out: ", "outside the window", "once", "30s", "anything you did there"} {
		if !strings.Contains(said, want) {
			t.Errorf("the sentence does not mention %q: %q", want, said)
		}
	}
	// AND NOTHING IS SAID WHEN NOTHING HAPPENED, which is what keeps it from being a
	// line on every recording.
	if quiet := (&Draft{}).AwaySentence(); quiet != "" {
		t.Errorf("a recording with no absence in it said %q", quiet)
	}

	// AND THE TERMINAL PRINTS IT, through `Learn` — which is where a person sees it, and
	// the reason this is not a test of a string: the sentence existed and nothing said
	// it for as long as it took to write this.
	var out strings.Builder
	d := Deps{
		Reg:    routes.Registry{Dir: t.TempDir()},
		Rec:    &confinedRecorder{events: aDemonstrationWithAnAbsence()},
		Window: theGameWindow,
		In:     strings.NewReader("y\nc\n"), // save? yes; scope? only here
		Out:    &out,
		App:    func() string { return "arkascended" },
	}
	if err := d.Learn("ark graphics"); err != nil {
		t.Fatalf("Learn: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "outside the window (once)") {
		t.Errorf("the terminal never said it:\n%s", out.String())
	}
}

// AND A STOP FROM OUTSIDE THE WINDOW STILL ENDS THE RECORDING.
//
// The recorder publishes nothing but the stop key — see `recorder.StopOnly` — so what
// `waitForStop` has to work from is one key event for a key that was pressed somewhere else.
// `heldRecorder` keeps the stream open, so a recording that ended here ended because the stop
// was detected and not because the events ran out.
func TestAStopPressedOutsideTheWindowEndsTheRecording(t *testing.T) {
	// The ONLY thing on the channel, which is what a confined recording publishes when
	// the person walks away and presses the stop key.
	rec := &heldRecorder{events: []recorder.RecordedEvent{
		{Kind: recorder.EvKey, KeyName: recorder.DefaultStopKey, Down: true,
			T: at(9000)},
	}}
	d := Deps{Rec: rec, Window: theGameWindow, StopKey: recorder.DefaultStopKey}
	c, err := d.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	done := make(chan *Draft, 1)
	go func() { done <- c.Finish() }()
	select {
	case dr := <-done:
		if dr.StoppedBy != "the stop key" {
			t.Errorf("the draft says it was stopped by %q", dr.StoppedBy)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the recording never ended, so a person who walks out of the " +
			"window cannot stop it")
	}
}

// AND A RECORDING OF ONE WINDOW THAT DOES NOT KNOW WHICH IS REFUSED.
//
// This is the whole of why the target is given rather than inferred. The first version LATCHED
// it — the first foreground window that was not Marco's became the thing being recorded — so a
// terminal, a notification toast or an updater taking focus in the second after Record was
// pressed became the target, the demonstration that followed was admitted from a window nobody
// picked, and the person could not correct a choice they were never shown.
//
// The alternative to refusing is recording the desktop, which is the one outcome this slice
// exists to prevent. So it is refused BEFORE the hooks go on, because after them somebody is
// already demonstrating.
func TestAConfinedRecordingWithNoChosenWindowIsRefusedBeforeItStarts(t *testing.T) {
	rec := &confinedRecorder{events: aDemonstrationWithAnAbsence()}
	_, err := Deps{Rec: rec}.Begin("ark graphics")
	if err == nil {
		t.Fatal("a recording of one window started without knowing which window, " +
			"so it recorded the desktop")
	}
	if !strings.Contains(err.Error(), "no window was chosen") {
		t.Errorf("the refusal does not say what is missing: %v", err)
	}
	// AND IT DID NOT START. A refusal after `Start` is a person demonstrating into
	// nothing, and the hooks are already on the whole desktop by then.
	if rec.asked {
		t.Error("the recorder was told to confine itself to nothing")
	}
	if rec.ch != nil {
		t.Error("the recorder was started anyway")
	}
	// AND THE SAME DEPS ASKING FOR THE DESKTOP IS FINE, which is what makes the refusal
	// about the missing CHOICE rather than about confinement being broken.
	if _, err := (Deps{Rec: &confinedRecorder{}, Watch: WholeDesktop}).Begin("x"); err != nil {
		t.Errorf("a recording of the whole desktop was refused for having no "+
			"window: %v", err)
	}
}

// AND THE PERSON IS TOLD WHEN THE CAPTURE STOPS ADMITTING ANYTHING.
//
// Required, not optional. A confined recording goes silent the moment somebody leaves the
// window, and silence looks exactly like a recording that is still going: somebody who alt-tabs
// to check something and carries on demonstrating gets a routine missing half its steps with
// nothing having told them. The review sentence afterwards is not a substitute, because by then
// the demonstration they have to give again is over.
func TestLeavingTheWindowIsSaidWhileItIsHappening(t *testing.T) {
	var out strings.Builder
	rec := &heldRecorder{events: []recorder.RecordedEvent{
		{Kind: recorder.EvLeft, T: at(1000)},
		{Kind: recorder.EvEntered, T: at(4000)},
		{Kind: recorder.EvCrossed, KeyName: "ctrl was held down from outside it",
			T: at(4100)},
		{Kind: recorder.EvKey, KeyName: recorder.DefaultStopKey, Down: true,
			T: at(5000)},
	}}
	d := Deps{Rec: rec, Window: theGameWindow, Out: &out,
		StopKey: recorder.DefaultStopKey}
	c, err := d.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	done := make(chan *Draft, 1)
	go func() { done <- c.Finish() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the recording never ended")
	}
	said := out.String()
	for _, want := range []string{"paused", "left the window", "recording again",
		"crossed out of the window", "ctrl"} {
		if !strings.Contains(said, want) {
			t.Errorf("nothing said %q while it was happening:\n%s", want, said)
		}
	}
}

// AND A SURFACE THAT IS LOOKED AT CAN ASK, because Studio does not read the event channel.
func TestACaptureAnswersWhetherItIsPaused(t *testing.T) {
	rec := &pausedRecorder{}
	c, err := Deps{Rec: rec, Window: theGameWindow}.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	rec.away, rec.since = true, 7*time.Second
	paused, since := c.Paused()
	if !paused || since != 7*time.Second {
		t.Errorf("the capture says paused=%v since=%v", paused, since)
	}
	rec.away = false
	if paused, _ := c.Paused(); paused {
		t.Error("the capture still says it is paused after coming back")
	}
}

// pausedRecorder is a recorder that can be asked, which is what `Capture.Paused` goes through.
type pausedRecorder struct {
	confinedRecorder
	away  bool
	since time.Duration
}

func (p *pausedRecorder) Paused() (bool, time.Duration) { return p.away, p.since }
