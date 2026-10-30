package main

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// ALT-TAB BACK TO STUDIO TO PRESS STOP, THROUGH STUDIO'S OWN METHODS.
//
// The keyboard half of the defect `TestAVisitToStudioLeavesARecordingThatStillSaves` closed for
// the mouse. Going back to Studio by CLICKING on it has been excused since the activation-click
// exception; going back by Alt-Tab stranded two keys at the boundary and refused the recording,
// which made the Stop button unreachable by the one gesture everybody uses to reach it.
//
// Record, Stop, read the review, Save — the whole flow, not `inWindowClock` on its own.
func TestComingBackByAltTabLeavesARecordingThatSaves(t *testing.T) {
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
				Dpi: 96, Process: "arkascended.exe"},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 842,
				Y: 274, App: "arkascended", PID: gamePID, T: at(105)},
			// ── AND THE WAY BACK HERE: alt and tab go down in the window,
			// the switcher takes the foreground while they are still down,
			// and their releases happen somewhere nothing is admitted.
			{Kind: recorder.EvKey, KeyName: "alt", Down: true,
				App: "arkascended", PID: gamePID, T: at(2000)},
			{Kind: recorder.EvKey, KeyName: "tab", Down: true,
				App: "arkascended", PID: gamePID, T: at(2100)},
			{Kind: recorder.EvLeft, T: at(2150)},
			{Kind: recorder.EvHeldAcross, KeyName: "alt", T: at(2150)},
			{Kind: recorder.EvHeldAcross, KeyName: "tab", T: at(2150)},
		}}
	}
	if why := a.StartRecording("ark graphics", ""); why != "" {
		t.Fatal(why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatal(why)
	}
	r := a.Recording()
	if !r.Review {
		t.Fatal("Stop did not open review")
	}
	// IT SAVES. The sentence the owner actually hit was a refusal naming a key they had
	// used to reach this window.
	if r.Failed {
		t.Fatalf("coming back to press Stop refused the recording: %s", r.Said)
	}
	// AND THE PRESS THEY CAME TO MAKE IS STILL THERE, with the two keys they left by
	// nowhere in it.
	if r.Pressable != 1 {
		t.Errorf("the draft has %d pressable steps, not the one click that was "+
			"demonstrated: %v", r.Pressable, r.Steps)
	}
	for _, step := range r.Steps {
		if strings.Contains(step, "alt") || strings.Contains(step, "tab") {
			t.Errorf("the way out became a step: %q", step)
		}
	}
	// AND REVIEW SAYS SO. This is the only thing the boundary takes out instead of
	// refusing, so the card has to name it: the person is the only one who can say that
	// the Alt they held was something they meant.
	if !strings.Contains(r.Exit, "alt and tab") {
		t.Errorf("the review card says %q about what it left out", r.Exit)
	}
}

// AND A HELD KEY THE DEMONSTRATION CARRIED ON PAST IS STILL REFUSED HERE.
//
// Both halves at the surface, because the surface is where the owner met this: a window that
// excused every stranded key would save a recording whose ctrl+clicks are written down as
// clicks, and a window that excused none is the one that would not let them stop.
func TestAStrandedKeyTheDemonstrationUsedIsStillRefusedInStudio(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	a.recording = func() recorder.Recorder {
		return &heldRecorder{events: []recorder.RecordedEvent{
			{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
			// CTRL GOES DOWN AND THE DEMONSTRATION HAPPENS UNDER IT.
			{Kind: recorder.EvKey, KeyName: "ctrl", Down: true,
				App: "arkascended", PID: gamePID, T: at(100)},
			{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
				App: "arkascended", PID: gamePID, T: at(200),
				WinRel: true, RelX: 722, RelY: 210,
				RelTo: macroir.RelToWindow, WinW: 1920, WinH: 1080,
				Dpi: 96, Process: "arkascended.exe"},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: 842,
				Y: 274, App: "arkascended", PID: gamePID, T: at(205)},
			{Kind: recorder.EvLeft, T: at(300)},
			{Kind: recorder.EvHeldAcross, KeyName: "ctrl", T: at(300)},
		}}
	}
	if why := a.StartRecording("ark graphics", ""); why != "" {
		t.Fatal(why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatal(why)
	}
	r := a.Recording()
	if !r.Failed {
		t.Fatal("a recording whose ctrl was let go in another window, with the click " +
			"it modified still in it, was offered for saving")
	}
	if !strings.Contains(r.Said, "ctrl") {
		t.Errorf("the refusal does not name what was held: %q", r.Said)
	}
	if r.Exit != "" {
		t.Errorf("the window says it left something out: %q", r.Exit)
	}
}

// AND THE CARD RENDERS IT, which the payload test above cannot establish.
//
// Rendered, because a sentence in a payload nobody draws is a sentence nobody reads — and that
// is not hypothetical here: `Recording.Away` has been in this payload since the absence rule
// landed and the page has no element for it, so the one thing review says about the time
// somebody spent away is invisible in this window. Reported separately; this is the half that
// does not repeat it.
func aRenderedExitPage(t *testing.T, recording string) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, againHarnessWith(recording)+`
setTimeout(() => {
  const el = document.getElementById('capExit')
  const shown = el.classList.contains('hide') ? '' : el.textContent
  // THE TEXT AND WHETHER THE BOX IS THERE AT ALL, for the reason againHarnessWith
  // gives: an element shown with nothing in it cannot be told from an absent one by
  // reading its text, and a mutation that showed it unconditionally survives a
  // claim-level assertion on its own.
  document.title = 'exitsaid=' + (shown.includes('alt and tab') ? 1 : 0) +
    ' exitbox=' + (el.classList.contains('hide') ? 0 : 1)
}, 5200)`)
	if !strings.Contains(title, "exitsaid=") {
		t.Fatalf("the page never reported its counters: %q", title)
	}
	return title, func(key string) int {
		m := regexp.MustCompile(key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
}

// WHAT WAS LEFT OUT IS ON THE SCREEN, above the steps it is about.
func TestTheReviewCardSaysWhatTheWayOutLeftOut(t *testing.T) {
	title, n := aRenderedExitPage(t, `{review: true, name: 'enter freeplay',
		steps: ['press the left button at 722, 210 in the window'], pressable: 1,
		exit: 'The alt and tab you used to leave the window and come back here ' +
			'are how the recording was ended, not part of it, so they were left out.'}`)
	if n("exitsaid") != 1 {
		t.Errorf("the card does not say what was left out: %q", title)
	}
}

// AND AN ORDINARY RECORDING CARRIES NO EMPTY BOX ABOVE ITS STEPS.
func TestAReviewThatLeftNothingOutShowsNoSuchBox(t *testing.T) {
	title, n := aRenderedExitPage(t, `{review: true, name: 'enter freeplay',
		steps: ['press the left button at 722, 210 in the window'], pressable: 1}`)
	if n("exitbox") != 0 {
		t.Errorf("a recording that left nothing out has the box anyway: %q", title)
	}
	if n("exitsaid") != 0 {
		t.Errorf("the box says something: %q", title)
	}
}
