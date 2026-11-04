package main

// A ROUTINE IS A ROW THAT OPENS WHERE IT IS, AND PLAY NEVER LEAVES THAT ROW.
//
// The Architect, replacing the separate selected-routine card:
//
//	Replace the separate selected-routine card with an inline accordion. Put a chevron beside
//	each routine name; clicking the row or chevron expands it, clicking again collapses it.
//	Open one routine at a time. [...] Preserve unsaved inputs when clicking Rename, Play or
//	other controls—those clicks must not toggle the row. If switching routines would discard
//	pending edits, make that explicit.
//
// and, on the button:
//
//	keep Play on the collapsed row, with the chevron and routine name on the left, Play on the
//	right. [...] Clicking Play must not also toggle the row. Keep a single primary Play button
//	per routine—don't duplicate it inside the expanded section.
//
// # Why every claim here is rendered
//
// Because all of them are about WHERE THINGS ARE and WHAT A PRESS DOES, and neither survives
// reading the document. The panel is one set of elements CARRIED into whichever row is open, so
// its position is a fact about the running page and not about the markup; whether a press on
// Rename closes the row it is in depends on which element the handler is attached to and how the
// event travels; and whether Play is bigger than the controls beside it is computed by the
// cascade. A markup test would pass with the panel nailed to the bottom of the page.
//
// See [[ADR-290-a-routine-opens-where-it-is]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// theRowBackend is three routines: one that asks for a value, one that asks for nothing, and one
// that cannot run at all.
//
// THREE, BECAUSE THE RULING HAS THREE ANSWERS FOR PLAY. Ready, needing something, and refusing.
const theRowBackend = `
window.played = []
window.go = {main: {
  Backend: {Loaded: () => Promise.resolve(),
    // A REQUEST THAT SELECTS THE FIRST ROUTINE AND READS ITS VALUE OUT OF THE
    // WORDS. Nobody typed that value, and asking again produces it again.
    SuggestRoutine: () => Promise.resolve({answer: {kind: 'run',
      id: 'arkascended/focus/cloud-pull-stone', reply: 'Opening it.',
      offered: 1, source: 'exact', resolver: 'not-asked', context: {says: 'x'},
      values: [{name: 'item', value: 'granite'}], sourceAt: 'rev1'}})},
  Acts: {
    // THE WIDGET ACTION (S7), polled from start-up: the page draws whatever it says.
    WidgetStatus: () => Promise.resolve({state: 'ready', says: '', problems: []}),
    Theme: () => Promise.resolve({asked: '', theme: 'default', light: false, accentFrom: 'name', tokens: {}}),
    SuggestTargets: () => Promise.resolve(['arkascended']),
    SuggestRoutine: () => Promise.resolve({kind: 'none'}),
    Demonstrations: () => Promise.resolve([]), Saved: () => Promise.resolve([]),
    Learning: () => Promise.resolve({active: false}),
    Applications: () => Promise.resolve([]), Pictures: () => Promise.resolve([]),
    Picture: () => Promise.resolve(''),
    Watch: () => Promise.resolve({act: '', active: false, transcript: null}),
    History: () => Promise.resolve([]), Unrunnable: () => Promise.resolve([]),
    Recording: () => Promise.resolve({active: false}),
    PlaybackTarget: () => Promise.resolve({chosen: 'Untitled - Notepad',
      windows: [{term: 'Untitled - Notepad', label: 'Untitled - Notepad',
        says: 'Play brings Untitled - Notepad to the front and presses there.'}]}),
    Ready: () => Promise.resolve({build: 'abc1234', problems: []}),
    RoutineAnswers: (id) => Promise.resolve({id: id, phrases: [], askable: true}),
    PlayNeeds: (id) => Promise.resolve(id.indexOf('cloud') >= 0
      ? {id: id, needs: true,
         fields: [{name: 'item', says: 'what to type', secret: false}]}
      : {id: id, needs: false, fields: []}),
    ReviewPlay: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    ConfirmPlay: () => Promise.resolve(''), CancelPlay: () => {},
    PlayRoutine: (...a) => { window.played.push(JSON.stringify(a))
      return Promise.resolve('') },
    StepEditing: (id) => Promise.resolve({id: id, basis: 'b1', editable: true,
      rows: [
      {at: 0, says: 'type "hello"', kind: 'type', removable: true},
      {at: 1, says: 'wait 400ms', kind: 'wait', waitable: true, ms: 400}]}),
    ReviewStepEdit: () => Promise.resolve({why: 'not what this asks about'}),
    Library: () => Promise.resolve({dir: 'C:/x/routes'}),
    Features: () => Promise.resolve({acts: false, scenes: false,
      views: ['routines', 'history', 'help']}),
    BuildStatus: () => Promise.resolve({build: 'abc1234', recorder: 'abc1234'}),
    Help: () => Promise.resolve({sections: []}),
    Routines: () => Promise.resolve([
      {id: 'arkascended/focus/cloud-pull-stone', name: 'cloud pull stone',
       slug: 'cloud-pull-stone', app: 'arkascended', scope: 'app-focus',
       contract: 'guarded', runnable: true, clause: 'with item', wrote: 'x',
       saved_at: '31 Oct 2026 23:09', source: 'use os.\n'},
      {id: 'notepad/context/save-the-file', name: 'save the file',
       slug: 'save-the-file', app: 'notepad', scope: 'app-context',
       contract: 'guarded', runnable: true, wrote: 'x',
       saved_at: '30 Oct 2026 11:02', source: 'use os.\n'},
      {id: 'global/open-the-thing', name: 'open the thing', slug: 'open-the-thing',
       scope: 'everywhere', contract: 'legacy', runnable: false, wrote: 'x',
       why: 'its recording is missing how wide that window was, so every press ' +
         'refuses. Record it again to measure it.',
       saved_at: '29 Oct 2026 09:30', source: 'use os.\n'}]),
    CancelMove: () => {}, MoveRoutine: () => Promise.resolve({}),
    RenameRoutine: () => Promise.resolve({}),
    ReviewRename: (...a) => Promise.resolve({id: a[0], from: a[0],
      renaming: a[1], token: 't1', to: 'x', movable: true, was: '', now: ''}),
  },
}}
const K1 = 'arkascended-focus-cloud-pull-stone'
const K2 = 'notepad-context-save-the-file'
const K3 = 'global-open-the-thing'
const press = (id) => { const e = document.getElementById(id); if (e) { e.click() } }
const seen = (id) => { const e = document.getElementById(id)
  return e && !e.classList.contains('hide') ? 1 : 0 }
const where = () => document.getElementById('routineChosen').parentElement.id
const aria = (k) => document.getElementById('routineOpen-' + k)
  .getAttribute('aria-expanded')
const box = (id) => {
  const e = document.getElementById(id)
  if (!e) { return null }
  const b = e.getBoundingClientRect()
  return {t: Math.round(b.top), l: Math.round(b.left), h: e.offsetHeight,
          f: Math.round(parseFloat(getComputedStyle(e).fontSize) * 10)}
}
`

// counters pulls one integer out of what the page reported in its title.
func counters(t *testing.T, title, probe string) func(string) int {
	t.Helper()
	if !strings.Contains(title, probe) {
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

// ONE OPEN AT A TIME, IN PLACE, AND NOTHING ELSE TOGGLES IT.
func TestARoutineOpensWhereItIsAndPlayStaysOnItsRow(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, theRowBackend+`
let r = {}
setTimeout(() => {
  r.rows = document.querySelectorAll('#routineList > .card.pick').length
  r.discl = document.querySelectorAll('button.disclose').length
  r.plays = document.querySelectorAll('#routineList .go.big').length
  r.shut = aria(K1) === 'false' && aria(K2) === 'false' ? 1 : 0
  r.parked = where() === 'routinePark' ? 1 : 0
  r.isButton = document.getElementById('routineOpen-' + K1).tagName === 'BUTTON' ? 1 : 0
  r.controls = document.getElementById('routineOpen-' + K1)
    .getAttribute('aria-controls') === 'routineHere-' + K1 ? 1 : 0
  press('routineOpen-' + K1)
}, 1100)
setTimeout(() => {
  // IT OPENED WHERE IT IS: the panel is inside that row, not below the list.
  r.inRow = where() === 'routineHere-' + K1 ? 1 : 0
  r.open1 = aria(K1) === 'true' ? 1 : 0
  // AND PLAY IS ON THE ROW, NOT IN WHAT OPENED.
  r.playOnRow = document.getElementById('routinePlayBtn')
    .closest('.rowline') ? 1 : 0
  r.playInPanel = document.getElementById('routineChosen')
    .querySelector('.go.big') ? 1 : 0
  r.oneBig = document.querySelectorAll('#routineList .go.big').length
  // A CONTROL IN THE ROW DOES NOT TOGGLE THE ROW.
  press('routineRenameBtn')
}, 1700)
setTimeout(() => {
  r.afterRename = aria(K1) === 'true' ? 1 : 0
  r.formUp = seen('routineRenameForm')
  // NOR DOES ONE INSIDE WHAT OPENED.
  press('routineStepsBtn')
}, 2100)
setTimeout(() => {
  r.afterSteps = aria(K1) === 'true' ? 1 : 0
  r.stepsUp = seen('routineSteps')
  // ONE AT A TIME: opening another closes this one and takes the panel with it.
  press('routineOpen-' + K2)
}, 2600)
setTimeout(() => {
  r.moved = where() === 'routineHere-' + K2 ? 1 : 0
  r.open2 = aria(K2) === 'true' ? 1 : 0
  r.closed1 = aria(K1) === 'false' ? 1 : 0
  // AND THE RENAME BOX DID NOT COME WITH IT. It was opened on the routine above
  // with that routine's name in it; left open here it is an offer to call THIS
  // routine THAT, which is a different routine that already exists.
  r.renameClosed = seen('routineRenameForm') ? 0 : 1
  // AND PRESSING THE SAME ONE AGAIN COLLAPSES IT.
  press('routineOpen-' + K2)
}, 3100)
setTimeout(() => {
  r.collapsed = aria(K2) === 'false' ? 1 : 0
  r.backInPark = where() === 'routinePark' ? 1 : 0
  r.panelHidden = seen('routineChosen') ? 0 : 1
  // PLAY ON A SHUT ROW THAT NEEDS A VALUE OPENS IT AND ASKS FOR THE VALUE.
  press('routinePlay-' + K1)
}, 3500)
setTimeout(() => {
  r.openedToAsk = where() === 'routineHere-' + K1 ? 1 : 0
  r.focused = document.activeElement &&
    document.activeElement.id === 'routineArg-item' ? 1 : 0
  r.notRun = (window.played || []).length === 0 ? 1 : 0
  // AND PLAY ON A SHUT ROW THAT NEEDS NOTHING RUNS IT.
  //
  // LAST, because a run takes the screen: playRoutine moves the window to the run
  // view, and everything after it would be measured on a hidden page.
  press('routinePlay-' + K2)
}, 4500)
setTimeout(() => {
  r.ranShut = (window.played || []).length
  r.openedByPlay = where() === 'routineHere-' + K2 ? 1 : 0
}, 5300)
setTimeout(() => {
  // THE ONE THAT CANNOT RUN HAS A DARK PLAY AND ITS REASON BESIDE IT.
  r.deadPlay = document.getElementById('routinePlay-' + K3).disabled ? 1 : 0
  r.whyBeside = (document.getElementById('routineSays-' + K3).textContent || '')
    .indexOf('how wide that window was') >= 0 ? 1 : 0
  document.title = 'err=0 ' + Object.keys(r).map(k => k + '=' + r[k]).join(' ')
}, 5600)`)
	n := counters(t, title, "rows=")

	// ── the list, before anything is opened ──
	if n("rows") != 3 || n("discl") != 3 {
		t.Errorf("%d rows with %d disclosures, want 3 and 3: %s",
			n("rows"), n("discl"), title)
	}
	if n("plays") != 3 {
		t.Errorf("%d Play buttons for 3 routines. Each row carries its own, open or "+
			"shut: %s", n("plays"), title)
	}
	if n("shut") != 1 || n("parked") != 1 {
		t.Errorf("the list does not open with every row shut and the panel put "+
			"away: %s", title)
	}
	if n("isButton") != 1 || n("controls") != 1 {
		t.Errorf("the disclosure is not a button naming what it controls, so it "+
			"cannot be reached by keyboard or announced: %s", title)
	}

	// ── it opens where it is ──
	if n("inRow") != 1 || n("open1") != 1 {
		t.Errorf("opening a routine did not put its panel inside its own row with "+
			"aria-expanded set: %s", title)
	}

	// ── and Play is on the row, once ──
	if n("playOnRow") != 1 {
		t.Errorf("Play is not on the routine's row: %s", title)
	}
	if n("playInPanel") != 0 {
		t.Error("there is a second primary button inside the expanded section, " +
			"which is the duplicate Play the ruling names")
	}
	if n("oneBig") != 3 {
		t.Errorf("%d primary buttons across 3 routines, so one of them has two: %s",
			n("oneBig"), title)
	}

	// ── a press on a control is a press on that control ──
	if n("afterRename") != 1 || n("formUp") != 1 {
		t.Errorf("pressing Rename collapsed the row it is in: %s", title)
	}
	if n("afterSteps") != 1 || n("stepsUp") != 1 {
		t.Errorf("pressing a control inside the expanded section collapsed it: %s",
			title)
	}

	// ── one at a time, and press again to close ──
	if n("moved") != 1 || n("open2") != 1 || n("closed1") != 1 {
		t.Errorf("opening a second routine did not close the first and carry the "+
			"panel into the second: %s", title)
	}
	if n("renameClosed") != 1 {
		t.Error("the rename box came with the panel, holding the name of the " +
			"routine it was opened for. Under this one's name it is an offer " +
			"to call this routine that, which is a different routine")
	}
	if n("collapsed") != 1 || n("backInPark") != 1 || n("panelHidden") != 1 {
		t.Errorf("pressing the open routine again did not collapse it: %s", title)
	}

	// ── Play from a row that was not open ──
	if n("ranShut") != 1 || n("openedByPlay") != 1 {
		t.Errorf("Play on a shut row that needs nothing did not run it and open it: "+
			"%s", title)
	}
	if n("openedToAsk") != 1 || n("focused") != 1 {
		t.Errorf("Play on a shut row that needs a value did not open it and put the "+
			"cursor in the field it is waiting for: %s", title)
	}
	if n("notRun") != 1 {
		t.Errorf("a routine was performed with its value never supplied: %s", title)
	}

	// ── and the one that cannot run ──
	if n("deadPlay") != 1 || n("whyBeside") != 1 {
		t.Errorf("the routine that cannot run does not have a dark Play with its "+
			"short reason beside it: %s", title)
	}
}

// LEAVING A ROUTINE SOMEBODY HAS TYPED INTO SAYS WHAT IT WOULD THROW AWAY.
//
// And says it in their own terms. "Unsaved changes" is a sentence that could be about anything;
// what this has to name is the thing they would go looking for afterwards.
func TestLeavingARoutineSaysWhatItWouldDiscardAndKeepsItUntilAgreed(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, theRowBackend+`
let r = {}, says = ''
setTimeout(() => { press('routineOpen-' + K1) }, 1100)
setTimeout(() => {
  const b = document.getElementById('routineArg-item')
  b.value = 'granite'
  b.dispatchEvent(new Event('input'))
}, 1800)
setTimeout(() => { press('routineOpen-' + K2) }, 2200)
setTimeout(() => {
  r.asked = seen('routineLeaving')
  says = document.getElementById('routineLeaving').textContent
  r.stayed = where() === 'routineHere-' + K1 ? 1 : 0
  r.kept = document.getElementById('routineArg-item').value === 'granite' ? 1 : 0
}, 2600)
// AND IT IS STILL THERE AFTER THE LIST HAS BEEN POLLED. A redraw that wiped the
// warning two seconds after somebody read it would be a window asking a question
// and then taking it back.
setTimeout(() => {
  r.stillAsked = seen('routineLeaving')
  r.stillKept = document.getElementById('routineArg-item').value === 'granite' ? 1 : 0
  r.stillThere = where() === 'routineHere-' + K1 ? 1 : 0
  press('routineOpen-' + K2)
}, 5300)
setTimeout(() => {
  r.wentAfterAgreeing = where() === 'routineHere-' + K2 ? 1 : 0
  r.warningGone = seen('routineLeaving')
  document.title = 'err=0 ' + Object.keys(r).map(k => k + '=' + r[k]).join(' ') +
    ' named=' + (says.indexOf('\u201citem\u201d') >= 0 ? 1 : 0) +
    ' vague=' + (says.indexOf('unsaved') >= 0 ? 1 : 0)
}, 5900)`)
	n := counters(t, title, "asked=")

	if n("asked") != 1 {
		t.Errorf("leaving a routine with text typed into it discarded the text "+
			"without saying so: %s", title)
	}
	if n("stayed") != 1 || n("kept") != 1 {
		t.Errorf("the first press left the routine anyway: %s", title)
	}
	if n("named") != 1 {
		t.Errorf("the warning does not name the argument whose text would go. What " +
			"somebody needs is the thing they would go looking for afterwards")
	}
	if n("vague") != 0 {
		t.Error(`the warning says "unsaved", which is a sentence that could be ` +
			`about anything`)
	}
	// AND IT SURVIVES THE POLL. This failed when it was written: every caller that
	// changed the selection blanked the list signature and nothing set it again, so
	// the next poll redrew everything — moving the panel, taking focus out of
	// whatever was being typed in, and wiping this warning about two seconds after
	// it appeared. `drawRoutines` records what it drew now.
	if n("stillAsked") != 1 || n("stillKept") != 1 || n("stillThere") != 1 {
		t.Errorf("a poll took the warning, the text or the open routine away "+
			"underneath somebody: %s", title)
	}
	if n("wentAfterAgreeing") != 1 || n("warningGone") != 0 {
		t.Errorf("pressing again did not leave: %s", title)
	}
}

// AND A VALUE NOBODY TYPED IS NOT WORK IN PROGRESS.
//
// A request reads values out of its own words and puts them in the same boxes a person types
// into. Losing one costs nobody anything — asking again produces it again — so stopping to warn
// about it would be this window asking somebody to confirm its own work, which is how a warning
// stops being read at all.
//
// The distinction is by VALUE and not by name: a proposal somebody has since corrected is theirs.
func TestAValueARequestProposedIsNotSomethingSomebodyTyped(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, theRowBackend+`
let r = {}
// THE REQUEST SELECTS THE FIRST ROUTINE AND PROPOSES "granite" FOR IT.
setTimeout(() => {
  document.getElementById('askTarget').value = 'arkascended'
  document.getElementById('askBox').value = 'cloud pull stone with granite'
  document.getElementById('askBtn').click()
}, 1100)
setTimeout(() => {
  r.opened = where() === 'routineHere-' + K1 ? 1 : 0
  r.proposalIn = (document.getElementById('routineArg-item') || {}).value === 'granite'
    ? 1 : 0
  press('routineOpen-' + K2)
}, 2400)
setTimeout(() => {
  r.leftWithoutAsking = where() === 'routineHere-' + K2 ? 1 : 0
  r.asked = seen('routineLeaving')
  document.title = 'err=0 ' + Object.keys(r).map(k => k + '=' + r[k]).join(' ')
}, 3200)`)
	n := counters(t, title, "opened=")

	if n("opened") != 1 || n("proposalIn") != 1 {
		t.Fatalf("the request did not open the routine it chose with its value in "+
			"the box: %s", title)
	}
	if n("leftWithoutAsking") != 1 || n("asked") != 0 {
		t.Errorf("opening another routine stopped to warn about a value a request "+
			"proposed rather than one somebody typed: %s", title)
	}
}

// PLAY IS THE BIGGEST THING ON A ROW, AND THE ROW READS LEFT TO RIGHT.
//
// Chevron, name, then Play on the right — the Architect's layout — and Play bigger than the
// controls beside it, which is the owner's. Measured rather than read: `--ctl` is one height for
// every button on this page, and whether this one escapes it is decided by the cascade.
func TestPlayIsTheBiggestThingOnARowAndTheRowReadsLeftToRight(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	// AT A STATED WIDTH. Below 760 the row's controls WRAP under the name on
	// purpose, and the claim here is about the line they share when they do not.
	title := renderForCountersAt(t, browser, 1100, 1000, theRowBackend+`
let r = {}
setTimeout(() => { press('routineOpen-' + K1) }, 1100)
setTimeout(() => {
  const play = box('routinePlayBtn')
  const rename = box('routineRenameBtn')
  const chev = document.querySelector('#routineOpen-' + K1 + ' .chev')
    .getBoundingClientRect()
  const name = document.querySelector('#routineOpen-' + K1 + ' h3')
    .getBoundingClientRect()
  r.playTall = play.h
  r.renameTall = rename.h
  r.playFont = play.f
  r.renameFont = rename.f
  // LEFT TO RIGHT: chevron, name, then Play.
  r.chevFirst = Math.round(chev.left) < Math.round(name.left) ? 1 : 0
  r.playRight = rename.l < play.l && Math.round(name.left) < play.l ? 1 : 0
  // AND ALL THREE ON ONE LINE -- compared at their CENTRES, because they are
  // different heights and a line box centres them: a 46px Play and a 34px Rename
  // sharing a line have tops six pixels apart, which is the layout working.
  r.oneLine = Math.abs((play.t + play.h / 2) - (rename.t + rename.h / 2)) < 4 ? 1 : 0
  document.title = 'err=0 ' + Object.keys(r).map(k => k + '=' + r[k]).join(' ')
}, 1900)`)
	n := counters(t, title, "playTall=")

	if n("playTall") < 44 {
		t.Errorf("Play is %dpx tall, and the owner asked for a bigger button: %s",
			n("playTall"), title)
	}
	if n("playTall") <= n("renameTall") {
		t.Errorf("Play is %dpx and Rename beside it is %dpx, so Play is not the "+
			"bigger of the two", n("playTall"), n("renameTall"))
	}
	if n("playFont") <= n("renameFont") {
		t.Errorf("Play's label is %.1fpx and Rename's is %.1fpx: a bigger button "+
			"with the same lettering reads as a stretched button",
			float64(n("playFont"))/10, float64(n("renameFont"))/10)
	}
	if n("chevFirst") != 1 {
		t.Error("the chevron is not to the left of the name it opens")
	}
	if n("playRight") != 1 {
		t.Error("Play is not to the right of the name and of Rename")
	}
	if n("oneLine") != 1 {
		t.Error("Play and Rename are not on the same line of the row")
	}
}

// AND THE ROW HOLDS TOGETHER IN A NARROW WINDOW.
//
// A list is scanned by its names, and a name squeezed to two words by the buttons beside it is a
// list that cannot be scanned. At 460px the controls wrap under the name instead — and nothing
// pushes the page sideways, which is the failure a reader meets as a horizontal scrollbar under
// everything.
func TestTheRowsHoldTogetherInANarrowWindow(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCountersAt(t, browser, 460, 1100, theRowBackend+`
let r = {}
setTimeout(() => { press('routineOpen-' + K1) }, 1100)
setTimeout(() => { press('routineStepsBtn') }, 1700)
setTimeout(() => {
  const d = document.documentElement
  r.noSideways = d.scrollWidth <= d.clientWidth + 1 ? 1 : 0
  // THE NAME KEEPS ITS LINE. Wrapped to its own row, the disclosure is as wide as
  // the list rather than as wide as what is left over beside three controls.
  const disc = document.getElementById('routineOpen-' + K1).getBoundingClientRect()
  const card = document.getElementById('routine-' + K1).getBoundingClientRect()
  r.nameKeepsTheLine = disc.width > card.width * 0.8 ? 1 : 0
  // AND THE EDITOR, WHICH IS THE LONGEST THING THIS PANEL EVER SHOWS.
  r.editorOpen = seen('routineSteps')
  const rows = document.querySelectorAll('#routineStepRows .sr')
  r.stepRows = rows.length
  let over = 0
  rows.forEach(x => { if (x.scrollWidth > x.clientWidth + 1) { over++ } })
  r.stepsFit = over === 0 ? 1 : 0
  // AND THE VALUE HAS THE LINE TO ITSELF. Nothing overflows with four columns
  // either -- the text just wraps into a eighty-pixel ribbon between the verb
  // and the buttons, which is the failure this width is about.
  const sr = document.querySelector('#routineStepRows .sr.four')
  const cell = sr && sr.children[2]
  r.valueWide = (cell &&
    cell.getBoundingClientRect().width > sr.getBoundingClientRect().width * 0.6)
    ? 1 : 0
  document.title = 'err=0 ' + Object.keys(r).map(k => k + '=' + r[k]).join(' ')
}, 2400)`)
	n := counters(t, title, "noSideways=")

	if n("noSideways") != 1 {
		t.Errorf("the page scrolls sideways at 460px: %s", title)
	}
	if n("nameKeepsTheLine") != 1 {
		t.Errorf("the name is squeezed by the controls beside it rather than them "+
			"wrapping under it: %s", title)
	}
	if n("editorOpen") != 1 || n("stepRows") < 2 {
		t.Fatalf("the step editor did not open inside the narrow row: %s", title)
	}
	if n("stepsFit") != 1 {
		t.Errorf("a step row overflows its own width at 460px: %s", title)
	}
	// AND NOTHING OVERFLOWS WITH FOUR COLUMNS EITHER -- the text wraps instead, into
	// a ribbon between the verb and the buttons. Which is why the claim is that the
	// value HAS the line: that is what the three-line layout is for.
	if n("valueWide") != 1 {
		t.Errorf("what a step does is squeezed between the verb beside it and its "+
			"buttons rather than having the line: %s", title)
	}
}
