package main

// THE EDITOR DRAWS WHERE A PRESS IS, IN THE BASIS IT WAS MEASURED IN, AND SENDS THE PAIR BACK.
//
// # Why this is a browser
//
// Because the claim is about which of two pairs a row offers, and a row that drew the wrong one
// would look right: two numbers in two boxes, labelled, beside a button that works. What makes it
// wrong is invisible until somebody plays the routine in a window that has moved.
//
// And because the backend never tells the page which pair to offer by asking it to work anything
// out — `in_window` comes from the same reading the validator applies against. A page that
// decided for itself would be free to offer a window to a press that was never measured in one,
// and the refusal would arrive after somebody had typed into the box.
//
// See [[ADR-286-a-press-can-be-moved]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// thePointBackend is the step-edit stub with three presses on it: one measured in a window, one
// measured on the desktop, and one found by its picture.
const thePointBackend = `
window.reviewArgs = []
window.go = {main: {
  Backend: {
    Loaded: () => Promise.resolve(),
    SuggestRoutine: () => Promise.resolve({answer: {kind: 'none'}}),
  },
  Acts: {
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
    Recording: () => Promise.resolve({active: false}),
    PlaybackTarget: () => Promise.resolve({chosen: '', windows: []}),
    Ready: () => Promise.resolve({build: 'abc1234', problems: []}),
    RoutineAnswers: (id) => Promise.resolve({id: id, phrases: [], askable: true}),
    PlayNeeds: (id) => Promise.resolve({id: id, fields: [], needs: false}),
    ReviewPlay: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    ConfirmPlay: () => Promise.resolve(''),
    CancelPlay: () => {},
    Library: () => Promise.resolve({dir: 'C:/x/routes'}),
    Features: () => Promise.resolve({acts: false, scenes: false,
      views: ['routines', 'history', 'help']}),
    // THE RAIL'S BUILD STAMP, which is its own read now rather than two fields of
    // the Scenes view's poll. See buildstatus.go.
    BuildStatus: () => Promise.resolve({build: 'abc1234'}),
    Help: () => Promise.resolve({sections: []}),
    Routines: () => Promise.resolve([{id: 'notepad/focus/address', name: 'address',
      slug: 'address', app: 'notepad', scope: 'app-focus', contract: 'legacy',
      runnable: true, offer: false, wrote: 'x', source: 'use os.'}]),
    StepEditing: (id) => Promise.resolve({id: id, name: 'address', editable: true,
      basis: 'basis-one', rows: [
        {at: 0, says: 'bring notepad to the front', x: 0, y: 0},
        {at: 1, says: 'left-click at 120, 90 in the window', point: true,
         movable: true, in_window: true, x: 120, y: 90,
         terms: 'left - of 1280x800, 96 dpi, notepad.exe'},
        {at: 2, says: 'left-click at 1620, 980', point: true, movable: true,
         in_window: false, x: 1620, y: 980, terms: 'left'},
        {at: 3, says: 'left-click at 880, 410 in the window', point: true,
         movable: false, in_window: true, x: 880, y: 410, terms: 'left',
         point_why: 'it is found by its picture, so the position is a place ' +
           'inside that picture rather than a place on the screen'},
      ]}),
    ReviewStepEdit: (...args) => {
      window.reviewArgs.push(JSON.stringify(args[2]))
      return Promise.resolve({id: args[0], name: 'address', token: 'tok',
        doing: ['the press at step 2 moves'], was: 'use os.', now: 'use os.',
        changes: true, savable: true, rows: []})
    },
    CancelStepEdit: () => {},
    SaveStepEdit: () => Promise.resolve({saved: true}),
  },
}}
const press = (id) => { document.getElementById(id).click() }
const there = (id) => !!document.getElementById(id)
const seen = (id) => {
  const el = document.getElementById(id)
  return el && !el.classList.contains('hide') ? 1 : 0
}
const pickRoutine = () => {
  const c = document.getElementById('routine-notepad-focus-address')
  if (c) { c.click() }
}
const rowAt = (n) => document.querySelectorAll('#routineStepRows .sr')[n]
`

// EACH PRESS SAYS WHICH PAIR IT IS OFFERING, AND THE ONE THAT CANNOT MOVE SAYS WHY.
func TestTheEditorOffersThePairAPressWasMeasuredIn(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, thePointBackend+`
let winBadge = '', scrBadge = '', winX = '', winY = '', scrX = ''
let winTerms = 0, noBoxes = 1, why = '', pinnedBadge = ''
let labelled = 0, named = 0, noPoint = 1
let sent = '', after = ''

setTimeout(() => { pickRoutine() }, 1000)
setTimeout(() => { press('routineStepsBtn') }, 1200)
setTimeout(() => {
  winBadge = (rowAt(1).querySelector('.basis') || {}).textContent || ''
  scrBadge = (rowAt(2).querySelector('.basis') || {}).textContent || ''
  pinnedBadge = (rowAt(3).querySelector('.basis') || {}).textContent || ''
  winX = document.getElementById('routineStepX-1').value
  winY = document.getElementById('routineStepY-1').value
  scrX = document.getElementById('routineStepX-2').value
  winTerms = rowAt(1).textContent.indexOf('1280x800') >= 0 ? 1 : 0
  // THE ONE FOUND BY ITS PICTURE HAS NO BOXES AND A REASON.
  noBoxes = there('routineStepX-3') ? 1 : 0
  why = rowAt(3).textContent
  // AND A STEP THAT PRESSES NOWHERE HAS NEITHER.
  noPoint = (there('routineStepX-0') || rowAt(0).querySelector('.basis')) ? 1 : 0
  // EVERY BOX IS LABELLED AND NAMED FOR ITS STEP AND ITS BASIS.
  const bx = document.getElementById('routineStepX-1')
  labelled = document.querySelector('label[for=routineStepX-1]') ? 1 : 0
  named = (bx.getAttribute('aria-label') || '').indexOf('inside the window') >= 0 ? 1 : 0
}, 1500)
setTimeout(() => {
  document.getElementById('routineStepX-1').value = '200'
  document.getElementById('routineStepY-1').value = '150'
  press('routineStepPlace-1')
}, 1800)
setTimeout(() => {
  after = shownAsked()
  press('routineStepReviewBtn')
}, 2100)
setTimeout(() => { sent = window.reviewArgs.join('|') }, 2400)

function shownAsked(){
  const el = document.getElementById('routineStepAsked')
  return el.classList.contains('hide') ? '' : el.textContent
}
setTimeout(() => {
  document.title = 'win=' + (winBadge === 'in the window' ? 1 : 0) +
    ' scr=' + (scrBadge === 'on the screen' ? 1 : 0) +
    ' pinned=' + (pinnedBadge === 'in the window' ? 1 : 0) +
    ' winX=' + (winX === '120' ? 1 : 0) + ' winY=' + (winY === '90' ? 1 : 0) +
    ' scrX=' + (scrX === '1620' ? 1 : 0) +
    ' terms=' + winTerms + ' noBoxes=' + noBoxes + ' noPoint=' + noPoint +
    ' why=' + (why.indexOf('found by its picture') >= 0 ? 1 : 0) +
    ' labelled=' + labelled + ' named=' + named +
    ' asked=' + (after.indexOf('presses at 200, 150 in the window') >= 0 ? 1 : 0) +
    ' sentOp=' + (sent.indexOf('"op":"point"') >= 0 ? 1 : 0) +
    ' sentX=' + (sent.indexOf('"x":200') >= 0 ? 1 : 0) +
    ' sentY=' + (sent.indexOf('"y":150') >= 0 ? 1 : 0) +
    ' sentAt=' + (sent.indexOf('"at":1') >= 0 ? 1 : 0)
}, 5200)`)
	if !strings.Contains(title, "win=") {
		t.Fatalf("the page never reported its counters: %q", title)
	}
	n := func(key string) int {
		m := regexp.MustCompile(`\b` + key + `=(-?\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	// ── WHICH PAIR, SAID ON THE ROW ──
	if n("win") != 1 || n("scr") != 1 {
		t.Fatalf("a press does not say which pair it is offering: %q", title)
	}
	if n("winX") != 1 || n("winY") != 1 {
		t.Errorf("the measured press does not offer its OFFSET: %q", title)
	}
	if n("scrX") != 1 {
		t.Errorf("the unmeasured press does not offer its screen point: %q", title)
	}
	if n("terms") != 1 {
		t.Errorf("the window the press is checked against is not shown beside the "+
			"numbers, which is what the edit keeps: %q", title)
	}
	// ── AND THE ONES THAT CANNOT BE MOVED ──
	if n("noBoxes") != 0 {
		t.Errorf("a press found by its picture was given boxes to move it: %q", title)
	}
	if n("why") != 1 {
		t.Errorf("a press that cannot be moved does not say why on its row, which "+
			"reads as a capability that is simply absent: %q", title)
	}
	if n("pinned") != 1 {
		t.Errorf("a press that cannot be moved does not say where it is: %q", title)
	}
	if n("noPoint") != 0 {
		t.Errorf("a step that presses nowhere was given a position: %q", title)
	}
	// ── AND EVERY BOX IS NAMED ──
	if n("labelled") != 1 || n("named") != 1 {
		t.Errorf("the boxes have no visible label, or no name saying which step and "+
			"which basis they are: %q", title)
	}
	// ── AND THE PAIR REACHES THE BACKEND, AGAINST THE ROW IT WAS READ OFF ──
	if n("asked") != 1 {
		t.Errorf("the asked line does not say where the press would be, or in "+
			"which basis — two numbers with no word beside them are the same "+
			"sentence for two different places: %q", title)
	}
	if n("sentOp") != 1 || n("sentAt") != 1 {
		t.Fatalf("the move was not sent against the step it was read off: %q", title)
	}
	if n("sentX") != 1 || n("sentY") != 1 {
		t.Errorf("the numbers in the boxes did not reach the backend: %q", title)
	}
}
