package main

// THE PAGE SAYS WHAT A RECORDING CAPTURES, AND SAYS NOTHING WHEN THE FILE SAYS NOTHING.
//
// Rendered rather than grepped, for the reason `redraw_test.go` gives at length: a source guard
// passes as long as the field is NAMED in the file, so a page that printed the sentence
// unconditionally — or printed `undefined` for a routine that states no mode — would keep every
// such guard green. The branch is the thing under test and only something that runs the page can
// take it.
//
// Both directions, because a blank rendered as a sentence and a sentence rendered as a blank are
// two different defects and a single assertion catches one of them.
//
// See [[ADR-254-what-a-recording-captures-is-stated]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// modeHarness is a window with nothing recording, one routine that states its mode and one that
// does not, and each of them chosen in turn.
//
// The silent routine is the ordinary case: every routine in every library predates this, and so
// does one made by the overlay or by `marco learn`. It is listed SECOND so that choosing it has
// to actually change the panel — a page that rendered the first routine's answer for whatever was
// selected would pass if the stated one came last.
const modeHarness = `
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
    Applications: () => Promise.resolve([]),
    Pictures: () => Promise.resolve([]),
    Picture: () => Promise.resolve(''),
    Learning: () => Promise.resolve({active: false}),
    Watch: () => Promise.resolve({act: '', active: false, transcript: null}),
    History: () => Promise.resolve([]),
    Unrunnable: () => Promise.resolve([]),
    Library: () => Promise.resolve({dir: 'C:/x/routes'}),
    Recording: () => Promise.resolve({active: false, mode: 'coordinates',
      captures: 'Where you press and how long you pause, and nothing about how the ' +
                'screen looked. It does not take a picture of anything.'}),
    // THE WINDOW A RUN WOULD PRESS IN. Studio does not Play without one, so a
    // harness that does not answer this is a page whose Play button is dead -
    // which is the right failure, and not what these are about.
    PlaybackTarget: () => Promise.resolve({chosen: '4660:77',
      windows: [{id: '4660:77', label: 'a window',
        says: 'Play brings a window to the front and presses there.'}]}),
    // THE PHRASES PANEL'S READ, which the page makes for every routine it
    // draws. A stub without it is a page that throws mid-render -- and note
    // that no backtick may appear in this comment: the harness around it is a
    // Go raw string, and one closed ten of these files at once.
    RoutineAnswers: (id) => Promise.resolve({id: id, phrases: [],
      askable: true}),
    Routines: () => Promise.resolve([
      {id: 'ark/focus/ark-graphics', name: 'ark graphics', slug: 'ark-graphics',
       app: 'ark', scope: 'app-anywhere', contract: 'guarded', runnable: true,
       saved_at: '2026-10-24 16:05', source: 'use os.',
       checks: 'every press is checked against the window it was recorded in',
       mode: 'coordinates',
       captures: 'Where you press and how long you pause, and nothing about how the ' +
                 'screen looked. It does not take a picture of anything.'},
      {id: 'ark/focus/old-thing', name: 'old thing', slug: 'old-thing',
       app: 'ark', scope: 'app-anywhere', contract: 'legacy', runnable: true,
       saved_at: '2026-01-02 09:00', source: 'use os.',
       checks: 'its presses are not checked against anything'},
    ]),
  },
}}
let stated = '', silent = ''
const pick = (i) => {
  const cards = document.querySelectorAll('#routineList .card')
  if (cards.length > i) { cards[i].click() }
}
const panel = () => {
  const el = document.getElementById('routineCaptures')
  return el.classList.contains('hide') ? '' : el.textContent
}
setTimeout(() => { pick(0) }, 2000)
setTimeout(() => { stated = panel() }, 2600)
setTimeout(() => { pick(1) }, 3200)
setTimeout(() => {
  silent = panel()
  const beside = document.getElementById('capCaptures')
  const review = document.getElementById('capReviewCaptures')
  document.title = 'cards=' + document.querySelectorAll('#routineList .card').length +
    ' beside=' + (beside.textContent.includes('does not take a picture') ? 1 : 0) +
    ' besideshown=' + (beside.classList.contains('hide') ? 0 : 1) +
    ' reviewtext=' + (review.textContent.includes('does not take a picture') ? 1 : 0) +
    ' statedsays=' + (stated.includes('Recorded as coordinates') ? 1 : 0) +
    ' statedblank=' + (stated === '' ? 1 : 0) +
    ' silentblank=' + (silent === '' ? 1 : 0) +
    ' silentundefined=' + (silent.includes('undefined') ? 1 : 0) +
    ' silentsays=' + (silent.includes('Recorded as') ? 1 : 0)
}, 4500)
`

// WHAT A RECORDING WILL CAPTURE IS ON THE PAGE BEFORE ANYBODY PRESSES RECORD.
//
// Beside Record, where the choice being described is one somebody is about to make, and again at
// review. From the backend both times: a page with the sentence typed into it would go on
// promising coordinates after the window stopped stating one.
func TestThePageSaysWhatARecordingWillCapture(t *testing.T) {
	title, n := aRenderedModePage(t)
	if got := n("beside"); got != 1 {
		t.Errorf("the line beside Record does not say that nothing is pictured: %q",
			title)
	}
	if got := n("besideshown"); got != 1 {
		t.Errorf("the line beside Record is hidden, so the sentence exists and "+
			"nobody can read it: %q", title)
	}
	if got := n("reviewtext"); got != 1 {
		t.Errorf("the review does not say what was captured: %q", title)
	}
}

// AND A ROUTINE THAT STATES ITS MODE SAYS SO, WHILE ONE THAT DOES NOT STAYS BLANK.
//
// The blank is the point. A routine saved before this says nothing about how it was made, and
// what it captured depended on the environment of a process that has long since exited — so a
// panel that filled the gap in would be telling somebody how their routine was made on the
// strength of an average. `undefined` is the same defect from the other side: it is what a page
// prints when it reads a field that was never sent, and it reads as a fault rather than as
// "nobody wrote this down".
func TestARoutineThatStatesNoModeIsShownWithoutOne(t *testing.T) {
	title, n := aRenderedModePage(t)
	if got := n("cards"); got != 2 {
		t.Fatalf("the library drew %d of 2 routines: %q", got, title)
	}
	if n("statedsays") != 1 {
		t.Errorf("the routine whose file states coordinates does not say so: %q", title)
	}
	if n("statedblank") == 1 {
		t.Errorf("the stated routine's panel is empty, so the mode reached the page "+
			"and was never rendered: %q", title)
	}
	if n("silentblank") != 1 {
		t.Errorf("a routine that states no mode has a sentence anyway: %q", title)
	}
	if n("silentundefined") == 1 {
		t.Errorf("a routine that states no mode is shown as `undefined`: %q", title)
	}
	if n("silentsays") == 1 {
		t.Errorf("a routine that states no mode is described as recorded as "+
			"something: %q", title)
	}
}

// aRenderedModePage renders the harness and hands back the title with a reader for its counters.
func aRenderedModePage(t *testing.T) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, modeHarness)
	if !strings.Contains(title, "cards=") {
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
