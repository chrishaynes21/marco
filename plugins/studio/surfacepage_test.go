package main

// WHAT IS ON SCREEN AFTER THE NAVIGATION THE DESKTOP HARNESS PERFORMS.
//
// # The defect this closes, which took two attempts
//
// `plugins/e2e` navigates Studio by accessibility: it presses a rail button and waits for a
// control to appear. It pressed `rail-scenes` and waited for `capBtn`, which worked while the
// recording card and the routine list were one surface. Studio gave the three
// demonstrate-and-watch tools their own place, so every registered desktop scenario failed in
// stage 6 of 6 — twenty minutes into a gate run, with the screen — saying *the scenes surface did
// not come up*, about a window that was working.
//
// The first fix moved the pair to `rail-routines`, where `capBtn` now lives. It was still wrong:
// `capBtn` is inside `captureCard`, which carries `hide` until somebody presses **Record
// something new**. AN ID IN THE DOCUMENT IS NOT AN ELEMENT IN THE ACCESSIBILITY TREE, and a
// containment check cannot tell the two apart — it passed completely on a harness that would
// still have hung.
//
// So this renders the real page and presses the real controls, in the order a scenario does, and
// asks what is actually on screen. `plugins/e2e`'s own guard keeps the narrower claim it can make
// without a browser: that the control is in the right section.
//
// Visibility is `offsetParent !== null`, which is the honest question for `.hide` —
// `display:none!important` — and is true of an element only when it and every ancestor are
// displayed. A `classList.contains('hide')` check would miss a hidden PARENT, which is exactly
// what `captureCard` is to `capBtn`.
//
// See [[Alpha-candidate-2026-10-29]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const surfaceHarness = `
window.go = {main: {
  Backend: {
    Loaded: () => Promise.resolve(),
    SuggestRoutine: () => Promise.resolve({answer: {kind: 'none'}}),
  },
  Acts: {
    // THE WIDGET ACTION (S7), polled from start-up: the page draws whatever it says.
    WidgetStatus: () => Promise.resolve({state: 'ready', says: '', problems: []}),
    Theme: () => Promise.resolve({asked: '', theme: 'default', light: false, accentFrom: 'name', tokens: {}}),
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
    // WHICH PARTS OF THIS WINDOW ARE OFFERED, AND WHAT IT EXPLAINS.
    // Both are read once at startup; a stub without them is a page
    // whose rail never paints. See help.go.
    Features: () => Promise.resolve({acts: true, scenes: true,
      views: ['routines', 'acts', 'scenes', 'history', 'help']}),
    // THE RAIL'S BUILD STAMP, which is its own read now rather than two
    // fields of the Scenes view's poll. See buildstatus.go.
    BuildStatus: () => Promise.resolve({build: 'abc1234'}),
    Help: () => Promise.resolve({sections: [{key: 'editing',
      title: 'Editing what a routine does', lines: ['nothing is written yet']}]}),
    Recording: () => Promise.resolve({active: false}),
    CancelReview: () => {},
    ReviewRebuilt: (id) => Promise.resolve({id: id, why: 'not what this asks'}),
    ScopeChoices: () => Promise.resolve([]),
    PlaybackTarget: () => Promise.resolve({chosen: '4660:77',
      windows: [{id: '4660:77', label: 'a window',
        says: 'Play brings a window to the front and presses there.'}]}),
    Windows: () => Promise.resolve([{id: '1', label: 'a window'}]),
    // THE PHRASES PANEL'S READ, which the page makes for every routine it
    // draws. A stub without it is a page that throws mid-render -- and note
    // that no backtick may appear in this comment: the harness around it is a
    // Go raw string, and one closed ten of these files at once.
    RoutineAnswers: (id) => Promise.resolve({id: id, phrases: [],
      askable: true}),
    // THE PLAY ROW'S READ, which the page makes for every routine it draws.
    // A stub without it is a page that throws mid-render; no backtick may appear
    // in this comment, because the harness around it is a Go raw string.
    PlayNeeds: (id) => Promise.resolve({id: id, fields: [], needs: false}),
    ReviewPlay: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    ConfirmPlay: () => Promise.resolve(''),
    CancelPlay: () => {},
    Routines: () => Promise.resolve([]),
  },
}}
const inClosedDisclosure = (el) => {
  for (let p = el; p; p = p.parentElement) {
    if (p.tagName === 'DETAILS' && !p.open) { return true }
  }
  return false
}
// onScreen is what an accessibility tree holds, which is neither textContent nor offsetParent
// alone. A hidden ancestor (display:none) takes offsetParent away; a CLOSED DISCLOSURE does
// not — Chromium keeps a layout box for the content of a closed <details> — and UI Automation
// leaves it out all the same. The provenance line is in one, so the first version of this read
// it as on screen while the real window did not offer it at all.
const onScreen = (id) => {
  const el = document.getElementById(id)
  if (!el || el.offsetParent === null) { return 0 }
  return inClosedDisclosure(el) ? 0 : 1
}
const press = (id) => {
  const el = document.getElementById(id)
  if (el) { el.click() }
}
let arrivedAsk = 0, arrivedCap = 0, arrivedReveal = 0
let afterAsk = 0, afterCap = 0, afterReveal = 0
let backAsk = 0, railThere = 0
let atOnceCap = 0, atOnceReveal = 1

// THE SEQUENCE A SCENARIO PERFORMS: press the rail button, then look.
setTimeout(() => {
  railThere = document.getElementById('rail-routines') ? 1 : 0
  press('rail-routines')
}, 1200)
setTimeout(() => {
  arrivedAsk = onScreen('askBtn')
  arrivedCap = onScreen('capBtn')
  arrivedReveal = onScreen('recordRevealBtn')
  // AND THE RECORDING FORM, WHICH HAS TO BE REVEALED.
  press('recordRevealBtn')
  // READ IN THE SAME TICK, WHICH IS THE WHOLE POINT.
  //
  // The window also repaints this from its poll, every 400ms, off recordOpen — so a
  // read on a later timer cannot tell the reveal from the repaint, and mutations that
  // broke the reveal survived because the poll quietly fixed it up. A press somebody
  // makes has to answer AT ONCE: half a second of a button that did nothing is the
  // thing this card was rearranged to stop.
  atOnceCap = onScreen('capBtn')
  atOnceReveal = onScreen('recordRevealBtn')
}, 1600)
setTimeout(() => {
  afterAsk = onScreen('askBtn')
  afterCap = onScreen('capBtn')
  afterReveal = onScreen('recordRevealBtn')
  // AND BACK, the way leaveRun returns from a run.
  press('rail-routines')
}, 2000)
setTimeout(() => { backAsk = onScreen('askBtn') }, 2400)
setTimeout(() => {
  document.title =
    ' railthere=' + railThere +
    ' arrivedask=' + arrivedAsk +
    ' arrivedcap=' + arrivedCap +
    ' arrivedreveal=' + arrivedReveal +
    ' afterask=' + afterAsk +
    ' aftercap=' + afterCap +
    ' afterreveal=' + afterReveal +
    ' backask=' + backAsk +
    ' atoncecap=' + atOnceCap +
    ' atoncereveal=' + atOnceReveal
}, 2900)
`

func aRenderedSurfacePage(t *testing.T) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, surfaceHarness)
	if !strings.Contains(title, "railthere=") {
		t.Fatalf("the page never reported its counters: %q", title)
	}
	return title, func(key string) int {
		m := regexp.MustCompile(`\b` + key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
}

// THE CONTROL THE HARNESS PROVES ARRIVAL WITH IS ON SCREEN WHEN IT ARRIVES.
//
// The claim every desktop scenario rests on, and the one that cannot be made from the file: it is
// about what is DISPLAYED after pressing the rail button, which needs the page to have run.
func TestTheRoutinesSurfaceIsProvedByAControlThatIsOnIt(t *testing.T) {
	title, n := aRenderedSurfacePage(t)

	if n("railthere") != 1 {
		t.Fatalf("the page has no rail-routines button, so the harness cannot "+
			"navigate at all: %q", title)
	}
	if n("arrivedask") != 1 {
		t.Errorf("askBtn is not on screen after pressing rail-routines, so every "+
			"scenario would report that the plays surface did not come up: %q",
			title)
	}
	// AND IT IS STILL THERE AFTER THE FORM IS REVEALED, which is what makes it a proof
	// of the SURFACE rather than of a moment in it: `recordRevealBtn` disappears when
	// pressed, so it could not do this job.
	if n("afterask") != 1 {
		t.Errorf("askBtn leaves the screen once the recording form is revealed: %q",
			title)
	}
	if n("afterreveal") != 0 {
		t.Errorf("recordRevealBtn is still on screen after being pressed, so this "+
			"test is not exercising the reveal: %q", title)
	}
	// AND AFTER COMING BACK, which is what `leaveRun` waits for.
	if n("backask") != 1 {
		t.Errorf("askBtn is not on screen after returning to the plays surface, "+
			"so leaveRun would never see the window come back: %q", title)
	}
}

// AND THE RECORDING FORM IS REVEALED RATHER THAN WAITED FOR.
//
// The half the containment guard cannot see, and the one the first fix got wrong. `capBtn` is in
// the document on arrival and NOT on screen; it appears when `recordRevealBtn` is pressed. A
// harness waiting for it on arrival waits for something nobody revealed — and a check that it sits
// in the right `<section>` agrees with that harness completely.
func TestTheRecordingFormIsRevealedRatherThanWaitedFor(t *testing.T) {
	title, n := aRenderedSurfacePage(t)

	if n("arrivedcap") != 0 {
		t.Errorf("capBtn is on screen as soon as the plays surface comes up, so "+
			"the recording form is no longer revealed and this test no longer "+
			"describes the page: %q", title)
	}
	if n("arrivedreveal") != 1 {
		t.Errorf("recordRevealBtn is not on screen on arrival, so there is no way to "+
			"reach the recording form: %q", title)
	}
	if n("aftercap") != 1 {
		t.Errorf("capBtn is still not on screen after pressing recordRevealBtn, so a "+
			"scenario that needs the recording form has no sequence that reaches "+
			"it: %q", title)
	}
	// AND AT ONCE, NOT AT THE NEXT POLL.
	//
	// The window repaints this from its poll every 400ms as well, so a read on a later
	// timer passes whether or not the press did anything — which it did: two mutations
	// that broke the reveal outright survived until this was asked. A press has to
	// answer immediately.
	if n("atoncecap") != 1 {
		t.Errorf("capBtn is not on screen in the same tick as the press, so the form "+
			"appears up to a poll later and the button looks dead: %q", title)
	}
	if n("atoncereveal") != 0 {
		t.Errorf("recordRevealBtn is still on screen in the same tick as its own "+
			"press: %q", title)
	}
}
