package main

// THE RECORD-AGAIN LINES, RENDERED.
//
// Two branches a source guard cannot take. The *this will not replace that routine* sentence must
// appear for a re-recording and not for an ordinary one — a page that printed it always would put
// it over somebody's first recording. And the *recorded somewhere else* warning must appear only
// when the applications differ, because that is the one case where Save succeeds and writes a
// second routine; a warning on every re-recording is a warning nobody reads.
//
// See [[ADR-256-recording-a-routine-again]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// againHarnessWith is a window holding one recording, in review, described by the given JSON.
func againHarnessWith(recording string) string {
	return `
window.go = {main: {
  Backend: {
    Loaded: () => Promise.resolve(),
    // DIRECTOR ANSWERS THE ASK BOX NOW, so a stub without this is a page whose
    // request box throws — which is the right failure: a binding the page calls and
    // the window does not export is what these harnesses exist to catch.
    SuggestRoutine: () => Promise.resolve({answer: {kind: 'none'}}),
  },
  Acts: {
    // THE ASK BOX. Bound at startup, so a stub without it is a page that paints
    // nothing - which is the right failure: a binding the page calls and the window
    // does not export is exactly the defect these harnesses exist to catch.
    SuggestTargets: () => Promise.resolve([]),
    SuggestRoutine: () => Promise.resolve({kind: 'none'}),
    Demonstrations: () => Promise.resolve([]),
    Saved: () => Promise.resolve([]),
    // THE WINDOW A RUN WOULD PRESS IN. Studio does not Play without one, so a
    // harness that does not answer this is a page whose Play button is dead -
    // which is the right failure, and not what these are about.
    PlaybackTarget: () => Promise.resolve({chosen: '4660:77',
      windows: [{id: '4660:77', label: 'a window',
        says: 'Play brings a window to the front and presses there.'}]}),
    Routines: () => Promise.resolve([]),
    Applications: () => Promise.resolve([]),
    Pictures: () => Promise.resolve([]),
    Picture: () => Promise.resolve(''),
    Learning: () => Promise.resolve({active: false}),
    Watch: () => Promise.resolve({act: '', active: false, transcript: null}),
    History: () => Promise.resolve([]),
    Unrunnable: () => Promise.resolve([]),
    Library: () => Promise.resolve({dir: 'C:/x/routes'}),
    Ready: () => Promise.resolve({build: 'abc1234', problems: [], limits: 'nothing'}),
    Recording: () => Promise.resolve(` + recording + `),
  },
}}
setTimeout(() => {
  const says = document.getElementById('capAgainReview')
  const away = document.getElementById('capAgainElsewhere')
  const shown = (el) => el.classList.contains('hide') ? '' : el.textContent
  // AND WHETHER THE BOX IS THERE AT ALL, separately from what it says.
  //
  // Both, because neither alone is enough. The TEXT is the claim and is what a reader
  // believes — but an element shown with no text in it cannot be told from an absent one
  // by reading its text, and a mutation that showed it unconditionally therefore survived
  // the claim-level assertion. It is harmless today only because the .no and .muted classes are
  // colour-only, which is a property of this page's CSS and not of its logic: give
  // either a border or a background and an empty box becomes a visible artifact above the
  // Save button on every ordinary recording.
  const boxed = (el) => el.classList.contains('hide') ? 0 : 1
  const scope = document.getElementById('capScope')
  document.title = 'inreview=' + (document.getElementById('capReview').classList.contains('hide') ? 0 : 1) +
    ' saysshown=' + (shown(says).includes('NOT replace') ? 1 : 0) +
    ' saysblank=' + (shown(says) === '' ? 1 : 0) +
    ' awayshown=' + (shown(away).includes('SECOND routine') ? 1 : 0) +
    ' awayblank=' + (shown(away) === '' ? 1 : 0) +
    ' saysbox=' + boxed(says) +
    ' awaybox=' + boxed(away) +
    ' scope=' + (scope.value === 'app-anywhere' ? 1 : 0)
}, 4500)
`
}

// aReRecordingInTheSamePlace is the ordinary re-recording: same application, nothing to warn about.
//
// `scope` is deliberately EMPTY and `again_scope` is set, which is the case the preference is for:
// a scope that is forced leaves no choice and withdraws the select, and only when there IS a
// choice does the routine's own scope decide what it opens on.
const aReRecordingInTheSamePlace = `{active: false, review: true, name: 'ark graphics',
  application: 'arkascended', stopped_by: 'the Stop button', pressable: 2,
  steps: ['click at 640, 480', 'press escape'], said: '2 step(s), 2 of which press something.',
  mode: 'coordinates', captures: 'Where you press and how long you pause.',
  again: 'arkascended/focus/ark-graphics', again_name: 'ark graphics',
  again_scope: 'app-anywhere',
  again_says: 'This is a second recording of "ark graphics". Saving it will NOT replace ' +
              'that routine: Save refuses to write over one, and then offers you Replace by name.'}`

// aReRecordingMadeElsewhere is the same recording made in another application.
const aReRecordingMadeElsewhere = `{active: false, review: true, name: 'ark graphics',
  application: 'notepad', stopped_by: 'the Stop button', pressable: 2,
  steps: ['click at 640, 480', 'press escape'], said: '2 step(s), 2 of which press something.',
  mode: 'coordinates', captures: 'Where you press and how long you pause.',
  again: 'arkascended/focus/ark-graphics', again_name: 'ark graphics',
  again_scope: 'app-anywhere',
  again_says: 'This is a second recording of "ark graphics". Saving it will NOT replace ' +
              'that routine: Save refuses to write over one, and then offers you Replace by name.',
  again_elsewhere: 'This was recorded in notepad, and "ark graphics" is saved for ' +
                   'arkascended. Saving it will make a SECOND routine rather than a new ' +
                   'version of that one.'}`

// anOrdinaryRecording is a first recording of something, which must carry neither line.
const anOrdinaryRecording = `{active: false, review: true, name: 'new thing',
  application: 'arkascended', stopped_by: 'the Stop button', pressable: 2,
  steps: ['click at 640, 480', 'press escape'], said: '2 step(s), 2 of which press something.',
  mode: 'coordinates', captures: 'Where you press and how long you pause.'}`

// A RE-RECORDING SAYS WHAT SAVE WILL DO, AND OPENS ON THE ROUTINE'S OWN SCOPE.
func TestAReRecordingSaysSavingWillNotReplace(t *testing.T) {
	title, n := aRenderedAgainPage(t, aReRecordingInTheSamePlace)
	if n("inreview") != 1 {
		t.Fatalf("the review card is not on screen: %q", title)
	}
	if n("saysshown") != 1 {
		t.Errorf("the review does not say that Save will not replace the routine "+
			"this was recorded from: %q", title)
	}
	// AND NO WARNING, because it was recorded where the routine lives. A warning on
	// every re-recording is a warning nobody reads.
	if n("awayblank") != 1 {
		t.Errorf("a re-recording made in the right application was warned about "+
			"anyway: %q", title)
	}
	// AND THE BOX IS NOT THERE AT ALL. See `boxed` for why this is asserted beside
	// the sentence above rather than instead of it.
	if n("awaybox") != 0 {
		t.Errorf("an empty warning box is on screen above Save: %q", title)
	}
	if n("scope") != 1 {
		t.Errorf("the review did not open on the routine's own scope: %q", title)
	}
}

// AND ONE MADE SOMEWHERE ELSE IS WARNED ABOUT, BECAUSE SAVE WOULD SUCCEED.
func TestAReRecordingMadeElsewhereIsWarnedAbout(t *testing.T) {
	title, n := aRenderedAgainPage(t, aReRecordingMadeElsewhere)
	if n("awayshown") != 1 {
		t.Errorf("a re-recording made in another application carries no warning, "+
			"so Save would quietly write a second routine: %q", title)
	}
	if n("saysshown") != 1 {
		t.Errorf("it stopped saying which routine it is a second recording of: %q",
			title)
	}
}

// AND AN ORDINARY RECORDING CARRIES NEITHER LINE.
func TestAnOrdinaryRecordingCarriesNeitherAgainLine(t *testing.T) {
	title, n := aRenderedAgainPage(t, anOrdinaryRecording)
	if n("inreview") != 1 {
		t.Fatalf("the review card is not on screen: %q", title)
	}
	if n("saysblank") != 1 {
		t.Errorf("a first recording is described as a second one: %q", title)
	}
	if n("awayblank") != 1 {
		t.Errorf("a first recording is warned about being made somewhere else: %q",
			title)
	}
	// AND NEITHER BOX IS THERE AT ALL. See `boxed`.
	if n("saysbox") != 0 || n("awaybox") != 0 {
		t.Errorf("an ordinary recording carries empty re-recording boxes: %q", title)
	}
}

// aRenderedAgainPage renders the page with one recording in review.
func aRenderedAgainPage(t *testing.T, recording string) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, againHarnessWith(recording))
	if !strings.Contains(title, "inreview=") {
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
