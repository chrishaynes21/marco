package orchestrator

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// GOING BACK TO THE WINDOW WITH THE STOP BUTTON IN IT.
//
// The defect these cases close was not subtle and was invisible to every test in this package:
// the boundary refused a recording the instant somebody left the window with a key still down,
// and Alt-Tab holds two keys down. So the one obvious way to finish a recording — come back to
// Studio and press Stop — refused every recording it ended, and the only exit that worked was a
// shortcut mentioned in six grey words beside the button.
//
// The rule now distinguishes the way OUT from a gesture the demonstration DEPENDS on, and the
// three cases below the first one are the bound on it. See `exitGesture`.

// anAltTabOutAfter is somebody leaving the chosen window by Alt-Tab at the end of a
// demonstration: both keys down and admitted, the window left while they were down, and both
// reported as stranded.
//
// Nothing follows them because nothing could: their releases happened in another window, where
// nothing is admitted. That is the shape the rule reads.
func anAltTabOutAfter(events []recorder.RecordedEvent) []recorder.RecordedEvent {
	return append(events,
		recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: "alt", Down: true,
			App: "arkascended", T: at(36000)},
		recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: "tab", Down: true,
			App: "arkascended", T: at(36100)},
		recorder.RecordedEvent{Kind: recorder.EvLeft, T: at(36150)},
		recorder.RecordedEvent{Kind: recorder.EvHeldAcross, KeyName: "alt",
			T: at(36150)},
		recorder.RecordedEvent{Kind: recorder.EvHeldAcross, KeyName: "tab",
			T: at(36150)})
}

// ALT-TAB BACK TO STOP, AND THE RECORDING SAVES.
//
// Through `Finish` and `Save`, which is the production path: Studio's Stop button, `Learn` and
// `LearnAuto` all arrive there. A test of `inWindowClock` alone would have passed throughout the
// time this was broken, because what was broken is what the boundary decided before the clock
// ever saw it.
func TestAltTabbingBackToStopTheRecordingSaves(t *testing.T) {
	rec := &confinedRecorder{events: anAltTabOutAfter(aDemonstrationWithAnAbsence())}
	d := Deps{Rec: rec, Window: theGameWindow, Reg: routes.Registry{Dir: t.TempDir()}}
	c, err := d.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	dr := c.Finish()
	if dr.Unrepresentable != "" {
		t.Fatalf("coming back to press Stop refused the recording: %q",
			dr.Unrepresentable)
	}
	if _, err := d.Save(dr, ScopeApp, "arkascended"); err != nil {
		t.Fatalf("a recording ended by coming back to the window would not save: %v",
			err)
	}
}

// AND THE WAY OUT IS NOT IN THE DEMONSTRATION, WHILE EVERYTHING BEFORE IT IS.
//
// Both halves in one case, because each is wrong without the other: a routine that presses Alt
// and Tab on every run is the way out written down as a step, and a routine that lost the clicks
// in front of it is the discard this boundary exists to refuse.
func TestTheKeysUsedToLeaveAreNotInTheDemonstrationAndWhatPrecededThemIs(t *testing.T) {
	rec := &confinedRecorder{events: anAltTabOutAfter(aDemonstrationWithAnAbsence())}
	d := Deps{Rec: rec, Window: theGameWindow, Reg: routes.Registry{Dir: t.TempDir()}}
	c, err := d.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	dr := c.Finish()
	for _, ev := range dr.Events {
		if ev.Kind == recorder.EvKey {
			t.Errorf("%q is in the stored demonstration", ev.KeyName)
		}
	}
	for _, s := range dr.Steps {
		switch s.Kind {
		case macroir.StepKey, macroir.StepKeyDown, macroir.StepKeyUp:
			t.Errorf("the way out became a step the play would press: %+v", s)
		}
	}
	// THE PRESSES THEY CAME TO MAKE ARE STILL THERE.
	if dr.Pressable() != 2 {
		t.Errorf("the demonstration kept %d presses, not the two clicks that were "+
			"made in the window", dr.Pressable())
	}
	// AND IT IS SAID. Leaving something out of somebody's demonstration silently is the
	// same discard, one layer along: the only person who can disagree is the one who
	// performed it, and they can only disagree if review tells them.
	if !strings.Contains(dr.ExitSentence(), "Left out: Alt+Tab") {
		t.Errorf("review does not say what was left out: %q", dr.ExitSentence())
	}
}

// A STRANDED KEY THE DEMONSTRATION CARRIED ON PAST STILL REFUSES.
//
// The case the boundary exists for, and the reason the excuse above is not simply "a held key at
// the end is fine": Ctrl down in the window, released somewhere else, with clicks in between
// that it changed the meaning of. Keeping those clicks without it writes down `click` where the
// person did `ctrl+click`.
func TestAHeldKeyTheDemonstrationCarriedOnPastStillRefuses(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvKey, KeyName: "ctrl", Down: true, App: "arkascended",
			T: at(100)},
		{Kind: recorder.EvLeft, T: at(200)},
		{Kind: recorder.EvHeldAcross, KeyName: "ctrl", T: at(200)},
		{Kind: recorder.EvEntered, T: at(300)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", T: at(400)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", T: at(450)},
	}
	rec := &confinedRecorder{events: events}
	d := Deps{Rec: rec, Window: theGameWindow, Reg: routes.Registry{Dir: t.TempDir()}}
	c, err := d.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	dr := c.Finish()
	if dr.Unrepresentable == "" {
		t.Fatal("a recording whose ctrl was released in another window, with the " +
			"clicks it modified still in it, saved anyway")
	}
	if !strings.Contains(dr.Unrepresentable, "ctrl") {
		t.Errorf("the refusal does not say what was held: %q", dr.Unrepresentable)
	}
	if len(dr.Exit) != 0 {
		t.Errorf("it was treated as the way out: %q", dr.Exit)
	}
}

// AND PRESSING STOP IS NOT PERMISSION TO DISCARD WHATEVER WAS HELD.
//
// The guard that makes the excuse narrow enough to be safe. Ctrl goes down, the demonstration
// happens UNDER it, and only then does somebody Alt-Tab away — so three keys are stranded by the
// one leaving and only two of them are the way out. There is no way to drop Ctrl's press without
// rewriting what the clicks meant, so nothing is dropped and the recording refuses.
//
// This is the mutation that matters most here: an `exitGesture` that excused every stranded key
// passes every other case in this file.
func TestAKeyHeldUnderTheDemonstrationIsNotExcusedByLeavingAfterwards(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvKey, KeyName: "ctrl", Down: true, App: "arkascended",
			T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", T: at(200)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", T: at(250)},
	}
	rec := &confinedRecorder{events: append(anAltTabOutAfter(events),
		recorder.RecordedEvent{Kind: recorder.EvHeldAcross, KeyName: "ctrl",
			T: at(36150)})}
	d := Deps{Rec: rec, Window: theGameWindow, Reg: routes.Registry{Dir: t.TempDir()}}
	c, err := d.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	dr := c.Finish()
	if dr.Unrepresentable == "" {
		t.Fatal("ctrl was held under the whole demonstration and leaving the window " +
			"afterwards excused it, so the clicks it modified were saved without it")
	}
	if len(dr.Exit) != 0 {
		t.Errorf("something was dropped from the demonstration: %q", dr.Exit)
	}
}

// A WINDOW THAT WENT AWAY IS NOT SOMEBODY WALKING TO A BUTTON.
//
// The shapes are identical in the buffer — a key down, the boundary crossed, nothing after it —
// and they mean opposite things. Nobody navigated anywhere: the window closed on a gesture and
// cut it in half, and there was no Stop button they were on their way to.
func TestAKeyHeldWhenTheWindowWentAwayIsStillACrossing(t *testing.T) {
	rec := &confinedRecorder{events: append(
		anAltTabOutAfter(aDemonstrationWithAnAbsence()),
		recorder.RecordedEvent{Kind: recorder.EvTargetGone, T: at(36200)})}
	d := Deps{Rec: rec, Window: theGameWindow, Reg: routes.Registry{Dir: t.TempDir()}}
	c, err := d.Begin("ark graphics")
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	dr := c.Finish()
	if dr.Unrepresentable == "" {
		t.Fatal("a window that closed on a held key was read as somebody leaving it")
	}
	if len(dr.Exit) != 0 {
		t.Errorf("the closing dropped %q from the demonstration", dr.Exit)
	}
}

// AND A MOUSE BUTTON STRANDED AT THE BOUNDARY IS A DRAG, NOT A WAY OUT.
//
// Leaving a window is a keyboard gesture. A button that went down in the window and came up
// somewhere else is the start of a drag whose end happened elsewhere, and dropping its press
// would turn a drag into nothing at all — so this one is never excused, whatever follows it.
func TestAMouseButtonStrandedAtTheBoundaryIsNeverTheWayOut(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", T: at(100)},
		{Kind: recorder.EvLeft, T: at(200)},
		{Kind: recorder.EvHeldAcross, KeyName: "the left button", T: at(200)},
	}
	st := inWindowClock(events, ResumeSettle)
	if st.Crossing == "" {
		t.Fatal("a drag that left the window was read as somebody leaving it")
	}
	if len(st.Exit) != 0 {
		t.Errorf("the drag's press was dropped as a way out: %q", st.Exit)
	}
}

// A KEY PRESSED AND RELEASED IN THE WINDOW IS A STEP, even when the same key is stranded later.
//
// Tab is the obvious one: a demonstration that tabs between fields and is then ended with
// Alt-Tab reports `tab` as stranded, and the tab presses the person MADE are finished gestures
// with a release of their own. Only the press still open at the end is the way out.
func TestAFinishedPressOfTheSameKeyIsKept(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvKey, KeyName: "tab", Down: true, App: "arkascended",
			T: at(100)},
		{Kind: recorder.EvKey, KeyName: "tab", Down: false, App: "arkascended",
			T: at(150)},
	}
	st := inWindowClock(anAltTabOutAfter(events), ResumeSettle)
	if st.Crossing != "" {
		t.Fatalf("the recording was refused: %q", st.Crossing)
	}
	// THE DEMONSTRATION'S OWN TAB SURVIVES, with its release.
	var down, up int
	for _, ev := range st.Events {
		if ev.Kind != recorder.EvKey || ev.KeyName != "tab" {
			continue
		}
		if ev.Down {
			down++
		} else {
			up++
		}
	}
	if down != 1 || up != 1 {
		t.Errorf("the tab the person pressed came back as %d presses and %d releases",
			down, up)
	}
}

// A DEMONSTRATION THAT CARRIED ON, AND THEN ENDED WITH ALT-TAB.
//
// WRITTEN FOR A SURVIVOR, and it is the one that mattered. Deleting the promotion of a stranded
// key to a crossing left every other case here passing, because the trailing-run check refuses
// the same recordings for its own reasons. The case the two do NOT agree on is this one: a key
// stranded by an EARLIER leaving, a demonstration that went on afterwards, and then a perfectly
// ordinary Alt-Tab out to press Stop.
//
// Without the promotion, all three keys arrive at `exitGesture` as one stranded set, every one
// of their presses is in the trailing run of that larger set, and the way out carries the
// earlier key away with it — a recording saved with a press silently removed from the middle of
// somebody's demonstration, which is the exact discard this boundary exists to refuse.
func TestAKeyStrandedBeforeTheDemonstrationCarriedOnIsNotTakenOutByTheWayOut(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		// Q GOES DOWN AND THE WINDOW IS LEFT WHILE IT IS HELD.
		{Kind: recorder.EvKey, KeyName: "q", Down: true, App: "arkascended",
			T: at(100)},
		{Kind: recorder.EvLeft, T: at(200)},
		{Kind: recorder.EvHeldAcross, KeyName: "q", T: at(200)},
		// AND THE DEMONSTRATION CARRIES ON AFTER IT.
		{Kind: recorder.EvEntered, T: at(300)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", T: at(400)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", T: at(450)},
	}
	st := inWindowClock(anAltTabOutAfter(events), ResumeSettle)
	if st.Crossing == "" {
		t.Fatal("q was left behind in another window with a click after it, and the " +
			"Alt-Tab at the end excused it")
	}
	if !strings.Contains(st.Crossing, "q") {
		t.Errorf("the refusal names %q", st.Crossing)
	}
	// AND NOTHING WAS TAKEN OUT, because nothing may be: the recording is refused whole.
	if len(st.Exit) != 0 {
		t.Errorf("%q was taken out of a demonstration that is being refused anyway",
			st.Exit)
	}
}

// AND `exitGesture` ITSELF KEEPS A FINISHED PRESS OUT OF THE WAY OUT.
//
// WRITTEN FOR A SURVIVOR. Through the whole clock, a finished press of a key that is stranded
// LATER is covered by the promotion above, so dropping the release-tracking here changed nothing
// anybody could see. This asks the function its own question, because the rule is the function's:
// only a press still open at the end can be the way out, and a key somebody pressed and released
// in the window is a step whatever else happens to that key afterwards.
func TestOnlyAnOpenPressCanBeTheWayOut(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvKey, KeyName: "tab", Down: true, T: at(100)},
		{Kind: recorder.EvKey, KeyName: "tab", Down: false, T: at(150)},
		{Kind: recorder.EvKey, KeyName: "alt", Down: true, T: at(200)},
	}
	exit, kept := exitGesture(events, []string{"alt", "tab"}, false)
	if len(exit) != 1 || exit[0] != "alt" {
		t.Fatalf("the way out is %q, and tab was pressed and let go in the window",
			exit)
	}
	if len(kept) != 2 {
		t.Errorf("%d events were kept, not the tab press and its release",
			len(kept))
	}
}

// AND THE TERMINAL SAYS IT TOO, through `Learn`.
//
// WRITTEN FOR A SURVIVOR: deleting the line that prints it changed no test in this repository.
// The same shape the absence sentence was in once — the sentence existed and nothing said it —
// and the same answer, because `marco learn` is where a person sees this and a payload field
// Studio reads is not that.
func TestTheTerminalSaysWhatTheWayOutLeftOut(t *testing.T) {
	var out strings.Builder
	d := Deps{
		Reg:    routes.Registry{Dir: t.TempDir()},
		Rec:    &confinedRecorder{events: anAltTabOutAfter(aDemonstrationWithAnAbsence())},
		Window: theGameWindow,
		In:     strings.NewReader("y\nc\n"), // save? yes; scope? only here
		Out:    &out,
		App:    func() string { return "arkascended" },
	}
	if err := d.Learn("ark graphics"); err != nil {
		t.Fatalf("Learn: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "Left out: Alt+Tab") {
		t.Errorf("the terminal never said what it left out:\n%s", out.String())
	}
}

// AND THE ONE CASE WHERE THE PROMOTION IS THE ONLY THING THAT ANSWERS.
//
// WRITTEN FOR A SURVIVOR, and the second attempt at it: the first put a click after the stranded
// key, which the trailing-run check refuses on its own account, so deleting the promotion
// changed nothing and the test passed either way.
//
// What separates them is a stranded key with nothing but KEYS after it. Q is held, the window is
// left, the person comes back, and they finish by Alt-Tabbing out to press Stop — so the whole
// of what follows Q's press is Alt and Tab, which is exactly the shape of a way out. Promote Q
// when the demonstration goes past it and the recording refuses, naming Q. Leave it in the
// stranded set and all three arrive at `exitGesture` together, every press in the trailing run
// belongs to the set, and the way out carries Q off with it — a recording saved with a press
// taken silently out of the middle of it.
func TestAStrandedKeyIsNotCarriedOffByALaterWayOut(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", T: at(150)},
		// Q IS HELD, AND THE WINDOW IS LEFT WHILE IT IS DOWN.
		{Kind: recorder.EvKey, KeyName: "q", Down: true, App: "arkascended",
			T: at(200)},
		{Kind: recorder.EvLeft, T: at(300)},
		{Kind: recorder.EvHeldAcross, KeyName: "q", T: at(300)},
		{Kind: recorder.EvEntered, T: at(1000)},
	}
	st := inWindowClock(anAltTabOutAfter(events), ResumeSettle)
	if st.Crossing == "" {
		t.Fatal("q was stranded in another window and the Alt-Tab that ended the " +
			"recording took it away as part of the way out")
	}
	if !strings.Contains(st.Crossing, "q") {
		t.Errorf("the refusal names %q, which is not the key the person can do "+
			"something about", st.Crossing)
	}
	if len(st.Exit) != 0 {
		t.Errorf("%q was taken out of a recording that is refused anyway", st.Exit)
	}
	// AND Q'S PRESS IS STILL THERE TO BE SEEN, because the person has to give this
	// demonstration again and all of what they did is what tells them how.
	var sawQ bool
	for _, ev := range st.Events {
		if ev.Kind == recorder.EvKey && ev.KeyName == "q" {
			sawQ = true
		}
	}
	if !sawQ {
		t.Error("q is not in the recording the person is shown")
	}
}

// AND IT HOLDS WHICHEVER WAY ROUND THE DESKTOP REPORTS IT.
//
// The fixtures above assume both keys were admitted before the leaving was noticed, which is one
// of four orderings the real desktop can produce — and which one happens depends on whether the
// switcher had taken the foreground when `tab` went down, and on whether the hook or the
// foreground poller noticed the change first. None of that is a choice this code makes, and a fix
// that worked for one ordering would look right here and fail on somebody's machine.
//
// So all four, in one table. The one thing every row must agree on: the recording saves, and
// nothing of the demonstration is lost.
func TestTheWayOutIsRecognisedWhicheverOrderTheDesktopReportsIt(t *testing.T) {
	click := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", T: at(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", T: at(150)},
	}
	alt := recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: "alt", Down: true,
		App: "arkascended", T: at(200)}
	tab := recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: "tab", Down: true,
		App: "arkascended", T: at(210)}
	left := recorder.RecordedEvent{Kind: recorder.EvLeft, T: at(220)}
	held := func(k string) recorder.RecordedEvent {
		return recorder.RecordedEvent{Kind: recorder.EvHeldAcross, KeyName: k,
			T: at(220)}
	}
	for _, c := range []struct {
		name string
		tail []recorder.RecordedEvent
		want []string
	}{
		// BOTH ADMITTED, then the leaving noticed.
		{"both keys were in the window", []recorder.RecordedEvent{
			alt, tab, left, held("alt"), held("tab")},
			[]string{"alt", "tab"}},
		// THE SWITCHER TOOK THE FOREGROUND AS TAB WENT DOWN, so tab was never
		// admitted and only alt was stranded.
		{"the switcher took the foreground first", []recorder.RecordedEvent{
			alt, left, held("alt")},
			[]string{"alt"}},
		// AND THE POLLER NOTICING FIRST PRODUCES THAT SAME BUFFER, which is
		// worth a row rather than a sentence: the two doors that move the
		// boundary converge, so a fix made at one of them is enough. The
		// recorder's own case holds that both doors report; this holds that
		// what they report means the same thing.
		{"the poller noticed first", []recorder.RecordedEvent{
			alt, left, held("alt")},
			[]string{"alt"}},
		// AND THE FOREGROUND HAD ALREADY MOVED, so nothing was stranded at all
		// and there is nothing to excuse.
		{"nothing was held in the window", []recorder.RecordedEvent{left}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			st := inWindowClock(append(append([]recorder.RecordedEvent{},
				click...), c.tail...), ResumeSettle)
			if st.Crossing != "" {
				t.Fatalf("the recording was refused: %q", st.Crossing)
			}
			if strings.Join(st.Exit, ",") != strings.Join(c.want, ",") {
				t.Errorf("the way out reads as %q, want %q", st.Exit, c.want)
			}
			// AND THE CLICK IS THERE IN EVERY ONE OF THEM.
			var clicks int
			for _, ev := range st.Events {
				if ev.Kind == recorder.EvClick {
					clicks++
				}
			}
			if clicks != 2 {
				t.Errorf("%d click events survived, not the press and its "+
					"release", clicks)
			}
		})
	}
}
