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

// thePointBackend is the step-edit stub with four presses on it: one measured in a window, one
// measured on the desktop, one found by its picture, and one whose sentence the page's verb
// table does not open.
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
         movable: true, in_window: true, x: 120, y: 90},
        {at: 2, says: 'left-click at 1620, 980', point: true, movable: true,
         in_window: false, x: 1620, y: 980},
        {at: 3, says: 'left-click at 880, 410 in the window', point: true,
         movable: false, in_window: true, x: 880, y: 410,
         point_why: 'it is found by its picture, so the position is a place ' +
           'inside that picture rather than a place on the screen'},
        // AND ONE THE VERB TABLE CANNOT OPEN. The backend writes a verb for every
        // kind that presses and a Go test holds it to that; this is the page's half
        // of the same promise, because the value column is REPLACED on a press row
        // and a sentence with nowhere to go is a sentence thrown away.
        {at: 4, says: 'nudge 5, 6', point: true, movable: true,
         in_window: false, x: 5, y: 6},
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
  // A ROUTINE IS OPENED BY ITS DISCLOSURE, which is the only control that
  // toggles the row -- every other press inside it belongs to the routine.
  const c = document.getElementById('routineOpen-notepad-focus-address')
  if (!c) { return }
  c.click()
  // AND LEAVING A ROUTINE SOMEBODY HAS TYPED INTO ASKS FIRST. These tests are
  // about what happens once that is agreed to, and the press that agrees is the
  // same press -- so this opens it and answers, which is what a person does.
  const warn = document.getElementById('routineLeaving')
  if (warn && !warn.classList.contains('hide')) { c.click() }
}
const rowAt = (n) => document.querySelectorAll('#routineStepRows .sr')[n]
`

// A PRESS ROW IS ONE LINE AT AN ORDINARY WIDTH, AND WHEN IT STACKS THE PAIRS STAY WHOLE.
//
// # Why a browser, and why two of them
//
// Because both halves are about what a layout DOES at a width, and neither can be read off the
// markup. The row is a four-column grid whose value cell is whatever is left over, and what goes
// in that cell is a badge and two labelled number boxes. Nothing in the HTML says whether they
// fit.
//
// They did not. At a 1000px window the cell is 238 pixels; the row wanted 289 and was allowed to
// wrap, so `x` sat at the end of one line with the number it names on the next — two controls
// called nothing, under a badge saying which basis they are in. The Architect read it off a
// rendered page: *the coordinate row has no action verb, and X/Y split across lines*.
//
// So the row is `nowrap` and the stacking is a DECISION, taken in one place at one width. Which
// moves the question to: at what width? Measured at ten of them. The four columns cost about
// 302px before the value has anywhere to go, every hundred pixels off the window takes ninety-one
// off the cell, and the row needs 206 — so a little under 960 is where it stops fitting, and
// that is where the narrow block begins. At 760, where it used to begin, the cell is 35 pixels.
//
// The second render is the other half of the same decision: a stacked row is only better than a
// wrapped one if what comes down comes down WHOLE. `x` and the box it names travel together.
func TestAPressRowIsOneLineAtAnOrdinaryWidthAndStacksWhole(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	const look = `
let cols = 0, lines = 0, fits = 0, boxW = 0, inOrder = 0, pairLines = 0, pairs = 0
setTimeout(() => { pickRoutine() }, 1000)
setTimeout(() => { press('routineStepsBtn') }, 1200)
setTimeout(() => {
  const row = rowAt(1)
  const said = row.querySelector('.said.pt')
  const act = row.querySelector('.act')
  // THE CENTRES, NOT THE TOPS. A badge and a number box on one line are different
  // heights, so their tops differ by a few pixels while they sit on the same line.
  const mid = (e) => { const b = e.getBoundingClientRect()
    return Math.round(b.top + b.height / 2) }
  cols = getComputedStyle(row).gridTemplateColumns.split(' ').length
  lines = new Set(Array.from(said.children).map(mid)).size
  // AND NOTHING LEANS OUT OF THE CELL. Refusing to wrap buys one line by refusing
  // to break, which is an answer only while what is on it FITS; otherwise it is the
  // same defect with the numbers over the buttons instead of under the badge.
  // AND AGAINST THE CELL, NOT AGAINST ITSELF. An inline-flex box sizes to its own
  // content, so its scrollWidth is its clientWidth however far it leans over the
  // column beside it. What it has to fit inside is the grid cell holding it.
  fits = said.getBoundingClientRect().width <=
    row.querySelector('.what').getBoundingClientRect().width + 1 ? 1 : 0
  boxW = Math.round(document.getElementById('routineStepX-1').getBoundingClientRect().width)
  inOrder = act.getBoundingClientRect().right <= said.getBoundingClientRect().left ? 1 : 0
  // AND EVERY LABEL IS ON ITS OWN BOX'S LINE, at whatever width this is.
  const ps = row.querySelectorAll('.xyp')
  pairs = ps.length
  ps.forEach(p => { pairLines += new Set(Array.from(p.children).map(mid)).size })
}, 1600)
setTimeout(() => {
  document.title = 'cols=' + cols + ' lines=' + lines + ' fits=' + fits +
    ' boxW=' + boxW + ' inOrder=' + inOrder +
    ' pairs=' + pairs + ' pairLines=' + pairLines
}, 2000)`

	read := func(title string) func(string) int {
		if !strings.Contains(title, "cols=") {
			t.Fatalf("the page never reported its counters: %q", title)
		}
		return func(key string) int {
			m := regexp.MustCompile(`\b` + key + `=(-?\d+)`).FindStringSubmatch(title)
			if m == nil {
				t.Fatalf("no %s in %q", key, title)
			}
			v, _ := strconv.Atoi(m[1])
			return v
		}
	}

	// ── AN ORDINARY WINDOW: ACTION, WHERE IT PRESSES, CONTROLS, ON ONE LINE ──
	//
	// 1000 is the width the Architect read the defect off, and it is the narrow end of
	// ordinary: the cell left over is 238px and the row needs 206. Measured here rather
	// than somewhere roomier on purpose — at 1400 the row fits whatever is put in it,
	// so a test there would be a test of nothing.
	wideSaid := renderForCountersAt(t, browser, 1000, 1200, thePointBackend+look)
	wide := read(wideSaid)
	if wide("cols") != 4 {
		t.Fatalf("an ordinary window does not draw the four columns, so this is "+
			"measuring the stacked row: %q", wideSaid)
	}
	if wide("lines") != 1 {
		t.Errorf("the press row is drawn on %d lines in an ordinary window. A label "+
			"on one line and the number it names on the next are two controls "+
			"called nothing.", wide("lines"))
	}
	if wide("fits") != 1 {
		t.Error("the press row refuses to wrap and does not fit, so it leans out of " +
			"its cell and over the buttons beside it")
	}
	if wide("boxW") < 44 {
		t.Errorf("the boxes are %dpx wide, which is not a number somebody can read "+
			"or a target they can hit — one line was bought by squashing them",
			wide("boxW"))
	}
	if wide("inOrder") != 1 {
		t.Error("the row does not read left to right: what the press does is not " +
			"before where it presses")
	}
	if wide("pairs") != 2 || wide("pairLines") != 2 {
		t.Errorf("there are %d labelled boxes and they occupy %d lines between them, "+
			"and two boxes each on their own line is what this asks",
			wide("pairs"), wide("pairLines"))
	}

	// ── AND IT STACKS WHERE IT STOPS FITTING, NOT WHERE THE PAGE TURNS NARROW ──
	//
	// 900 is in the band this is about. The rest of the page becomes a narrow layout at
	// 760; this row ran out of room at about 960, and between the two it was drawing
	// four columns into a cell of fifty-odd pixels.
	betweenSaid := renderForCountersAt(t, browser, 900, 1200, thePointBackend+look)
	between := read(betweenSaid)
	if between("cols") != 2 {
		t.Errorf("at 900 the editor still draws four columns, where the value cell is "+
			"about fifty pixels wide. The width this row stacks at is the "+
			"width it stops fitting at, and it is not the width the rest of "+
			"the page turns narrow at: %q", betweenSaid)
	}

	// ── AND NARROW, WHERE IT STACKS ON PURPOSE AND THE PAIRS COME DOWN WHOLE ──
	thinSaid := renderForCountersAt(t, browser, 460, 1200, thePointBackend+look)
	thin := read(thinSaid)
	if thin("cols") != 2 {
		t.Fatalf("a narrow window still draws four columns, so the editor never "+
			"stacked and the rest of this is about the wrong layout: %q", thinSaid)
	}
	if thin("lines") < 2 {
		t.Error("the press row is still one line in a narrow window, where there is " +
			"no room for one — so it is leaning out of the window instead of " +
			"coming down")
	}
	if thin("fits") != 1 {
		t.Error("the press row leans out of its cell in a narrow window")
	}
	if thin("pairs") != 2 || thin("pairLines") != 2 {
		t.Errorf("the row stacked and took a label away from its box: %d boxes over "+
			"%d lines", thin("pairs"), thin("pairLines"))
	}
}

// EACH PRESS SAYS WHICH PAIR IT IS OFFERING, AND THE ONE THAT CANNOT MOVE SAYS WHY.
func TestTheEditorOffersThePairAPressWasMeasuredIn(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, thePointBackend+`
let winBadge = '', scrBadge = '', winX = '', winY = '', scrX = ''
let act = '', oddAct = '', twice = 1, noBoxes = 1, why = '', pinnedBadge = ''
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
  // WHAT THE PRESS DOES IS THE FIRST THING ON THE ROW, AND IT IS SAID ONCE.
  // The measurements came off these rows and then the button did too: it was
  // drawn after the numbers while the same word was already at the front of the
  // row, which is the column somebody scans.
  act = (rowAt(1).querySelector('.act') || {}).textContent || ''
  twice = (rowAt(1).querySelector('.said') || {}).textContent || ''
  // AND A SENTENCE THE VERB TABLE CANNOT OPEN STILL REACHES THE ROW.
  oddAct = (rowAt(4).querySelector('.act') || {}).textContent || ''
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
  document.title = 'win=' + (winBadge === 'window' ? 1 : 0) +
    ' scr=' + (scrBadge === 'screen' ? 1 : 0) +
    ' pinned=' + (pinnedBadge === 'window' ? 1 : 0) +
    ' winX=' + (winX === '120' ? 1 : 0) + ' winY=' + (winY === '90' ? 1 : 0) +
    ' scrX=' + (scrX === '1620' ? 1 : 0) +
    ' act=' + (act === 'Left-click' ? 1 : 0) +
    ' twice=' + (twice.indexOf('left') >= 0 || twice.indexOf('1280') >= 0 ? 1 : 0) +
    ' oddAct=' + (oddAct === 'nudge 5, 6' ? 1 : 0) +
    ' noBoxes=' + noBoxes + ' noPoint=' + noPoint +
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
	// ── AND WHAT THE PRESS DOES, ONCE, IN FRONT OF THEM ──
	if n("act") != 1 {
		t.Errorf("the press row does not open with what the press DOES. The editor "+
			"replaces everything the sentence said about where it is with the "+
			"boxes, so the verb is all that is left to say what happens "+
			"there: %q", title)
	}
	if n("twice") != 0 {
		t.Errorf("the button, or the window it is checked against, is written beside "+
			"the numbers as well as in front of them: %q", title)
	}
	if n("oddAct") != 1 {
		t.Errorf("a press whose sentence the verb table does not open lost that "+
			"sentence altogether, so the row is a badge and two numbers with "+
			"nothing saying what happens at them: %q", title)
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
