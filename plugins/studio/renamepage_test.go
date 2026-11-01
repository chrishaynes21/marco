package main

// THE WINDOW RENAMES A ROUTINE, AND NEVER OFFERS TO REPLACE WHAT IS IN THE WAY.
//
// # Why this is a browser
//
// Because `renameacts_test.go` proves what the BACKEND does with a real library and nothing
// about whether the window reaches it. Those are the two halves this repository has eight
// recorded cases of getting separately right and never joining.
//
// And because two of the claims are about what the page SENDS. A rename has no second yes to
// give — a collision is refused at preparation — so the call must be the one that takes the token
// alone. A page that called the move with `false` would work until somebody later made that
// argument come from a tick-box.
//
// See [[ADR-285-a-routine-can-be-called-something-else]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// theRenameBackend is a stub with one routine that asks for a value, and a rename that refuses
// the second time it is asked.
const theRenameBackend = `
window.renameArgs = []; window.ranArgs = []; window.movedArgs = []
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
    Learning: () => Promise.resolve({active: false}),
    Applications: () => Promise.resolve([]),
    Pictures: () => Promise.resolve([]),
    Picture: () => Promise.resolve(''),
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
    StepEditing: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    Library: () => Promise.resolve({dir: 'C:/x/routes'}),
    Features: () => Promise.resolve({acts: false, scenes: false,
      views: ['routines', 'history', 'help']}),
    // THE RAIL'S BUILD STAMP, which is its own read now rather than two fields of
    // the Scenes view's poll. See buildstatus.go.
    BuildStatus: () => Promise.resolve({build: 'abc1234'}),
    Help: () => Promise.resolve({sections: []}),
    Routines: () => Promise.resolve([{id: 'arkascended/focus/cloud-pull-stone',
      name: 'cloud pull stone', slug: 'cloud-pull-stone', app: 'arkascended',
      scope: 'app-focus', contract: 'legacy', runnable: true, offer: false,
      clause: 'with item', wrote: 'x',
      source: 'use os.\nthe CloudPullStone is a script.\n'}]),
    CancelMove: (token) => { window.cancelled = token },
    MoveRoutine: (...args) => {
      window.movedArgs.push(args.length + ':' + JSON.stringify(args))
      return Promise.resolve({why: 'a rename must not come through here'})
    },
    RenameRoutine: (...args) => {
      window.ranArgs.push(args.length + ':' + JSON.stringify(args))
      return Promise.resolve({moved: true, to: 'arkascended/focus/pull-a-stone',
        says: 'Renamed it.'})
    },
    ReviewRename: (...args) => {
      window.renameArgs.push(JSON.stringify(args))
      if (args[1] === 'cloud pull stone') {
        return Promise.resolve({id: args[0], why: '"cloud pull stone" is already ' +
          'called that, so there is nothing to rename'})
      }
      if (args[1] === 'cloud pull wood') {
        return Promise.resolve({id: args[0], from: args[0], name: 'cloud pull stone',
          renaming: 'cloud pull wood', clause: 'with item', token: 'tok-taken',
          to: 'arkascended/focus/cloud-pull-wood', movable: false,
          refused: 'there is already a routine at arkascended/focus/cloud-pull-wood. ' +
            'Nothing has been renamed - choose another name, or delete that one first',
          was: 'use os.', now: ''})
      }
      return Promise.resolve({id: args[0], from: args[0], name: 'cloud pull stone',
        renaming: args[1], clause: 'with item', token: 'tok-rename',
        to: 'arkascended/focus/pull-a-stone', movable: true, changes: true,
        was: 'use os.\nthe CloudPullStone is a script.\n',
        now: 'use os.\nthe PullAStone is a script.\n'})
    },
  },
}}
const press = (id) => { document.getElementById(id).click() }
const seen = (id) => {
  const el = document.getElementById(id)
  return el && !el.classList.contains('hide') ? 1 : 0
}
const said = (id) => {
  const el = document.getElementById(id)
  if (!el) { return '' }
  return el.classList.contains('hide') ? '' : el.textContent
}
const pick = () => {
  // A ROUTINE IS OPENED BY ITS DISCLOSURE, which is the only control that
  // toggles the row -- every other press inside it belongs to the routine.
  const c = document.getElementById('routineOpen-arkascended-focus-cloud-pull-stone')
  if (!c) { return }
  c.click()
  // AND LEAVING A ROUTINE SOMEBODY HAS TYPED INTO ASKS FIRST. These tests are
  // about what happens once that is agreed to, and the press that agrees is the
  // same press -- so this opens it and answers, which is what a person does.
  const warn = document.getElementById('routineLeaving')
  if (warn && !warn.classList.contains('hide')) { c.click() }
}
`

// THE BOX EDITS THE NAME, THE CHIP SHOWS WHAT IT KEEPS, AND ONE PRESS RENAMES IT.
func TestTheWindowRenamesARoutineAndKeepsWhatItAsksFor(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, theRenameBackend+`
let formHidden = 1, formShown = 0, boxHas = '', chip = ''
let panel = 0, means = '', btn = '', consent = 1
let sentName = '', ran = '', moved = 0, took = 0

setTimeout(() => { pick() }, 1000)
setTimeout(() => {
  // THE FORM IS REVEALED, NOT SITTING OPEN. A text box under every routine with
  // its name already in it reads as something to edit.
  formHidden = seen('routineRenameForm')
  press('routineRenameBtn')
}, 1300)
setTimeout(() => {
  formShown = seen('routineRenameForm')
  boxHas = document.getElementById('routineRenameBox').value
  chip = said('routineRenameClause')
  document.getElementById('routineRenameBox').value = 'pull a stone'
  press('routineRenameGo')
}, 1600)
setTimeout(() => {
  panel = seen('routineMove')
  means = said('routineMoveMeans')
  btn = document.getElementById('routineMoveBtn').textContent
  // AND NOTHING TO AGREE TO: a rename never offers to replace anything.
  consent = seen('routineMoveConsentRow')
  sentName = (window.renameArgs[window.renameArgs.length - 1] || '')
  press('routineMoveBtn')
}, 1900)
setTimeout(() => {
  ran = window.ranArgs.join('|')
  moved = window.movedArgs.length
  took = seen('routineMove')
}, 2300)

setTimeout(() => {
  document.title = 'formHidden=' + formHidden + ' formShown=' + formShown +
    ' boxHas=' + (boxHas === 'cloud pull stone' ? 1 : 0) +
    ' chip=' + (chip === 'with item' ? 1 : 0) +
    ' panel=' + panel + ' consent=' + consent +
    ' btn=' + (btn === 'Rename it' ? 1 : 0) +
    ' named=' + (means.indexOf('pull a stone') >= 0 ? 1 : 0) +
    ' keeps=' + (means.indexOf('asks for') >= 0 ? 1 : 0) +
    ' sent=' + (sentName.indexOf('pull a stone') >= 0 ? 1 : 0) +
    ' alone=' + (ran === '1:["tok-rename"]' ? 1 : 0) +
    ' moved=' + moved + ' closed=' + took
}, 5200)`)
	if !strings.Contains(title, "formShown=") {
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
	if n("formHidden") != 0 || n("formShown") != 1 {
		t.Fatalf("the rename box is not revealed by its button: %q", title)
	}
	if n("boxHas") != 1 {
		t.Errorf("the box does not open with the routine's name in it: %q", title)
	}
	if n("chip") != 1 {
		t.Errorf("what the routine asks for is not shown beside the box, so there "+
			"is nothing to tell somebody the values are being kept: %q", title)
	}
	if n("panel") != 1 || n("named") != 1 || n("keeps") != 1 {
		t.Errorf("the review does not say what it would be called and what comes "+
			"with it: %q", title)
	}
	if n("consent") != 0 {
		t.Errorf("a rename offers a box agreeing to replace something: %q", title)
	}
	if n("btn") != 1 {
		t.Errorf("the button under a rename review does not say Rename it: %q", title)
	}
	if n("sent") != 1 {
		t.Errorf("the name in the box did not reach the backend: %q", title)
	}
	// ── AND THE CALL IS THE ONE THAT TAKES THE TOKEN ALONE ──
	if n("alone") != 1 {
		t.Errorf("the rename was not sent as one argument — the token of the review "+
			"on the screen and nothing else: %q", title)
	}
	if n("moved") != 0 {
		t.Errorf("the rename went through the move call, which takes a second "+
			"argument agreeing to replace what is in the way: %q", title)
	}
	if n("closed") != 0 {
		t.Errorf("the review is still open after the rename: %q", title)
	}
}

// AND A REFUSAL LANDS WHERE THE THING TO CHANGE IS.
//
// TWO PLACES, AND THEY ARE NOT ONE. A refusal with no review — the name it already has — belongs
// beside the box, which is what somebody would retype. A refusal WITH a review — something is
// already called that — belongs in the panel beside the two sources, because there is a review to
// read and the answer is a different name rather than a press.
func TestARefusedRenameSaysSoWhereTheAnswerIs(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, theRenameBackend+`
let sameWhy = '', samePanel = 1
let takenWhy = 0, takenRefused = '', takenBtn = 1, takenConsent = 1

setTimeout(() => { pick() }, 1000)
setTimeout(() => { press('routineRenameBtn') }, 1300)
setTimeout(() => {
  // THE NAME IT ALREADY HAS: no review at all, so the reason is beside the box.
  document.getElementById('routineRenameBox').value = 'cloud pull stone'
  press('routineRenameGo')
}, 1600)
setTimeout(() => {
  sameWhy = said('routineRenameWhy')
  samePanel = seen('routineMove')
  // AND A NAME SOMETHING ELSE ALREADY HAS: a review, refused.
  document.getElementById('routineRenameBox').value = 'cloud pull wood'
  press('routineRenameGo')
}, 1900)
setTimeout(() => {
  takenWhy = seen('routineRenameWhy')
  takenRefused = said('routineMoveRefused')
  takenBtn = document.getElementById('routineMoveBtn').disabled ? 0 : 1
  takenConsent = seen('routineMoveConsentRow')
}, 2300)

setTimeout(() => {
  document.title = 'sameWhy=' + (sameWhy.indexOf('already called that') >= 0 ? 1 : 0) +
    ' samePanel=' + samePanel +
    ' takenWhy=' + takenWhy +
    ' taken=' + (takenRefused.indexOf('already a routine') >= 0 ? 1 : 0) +
    ' takenBtn=' + takenBtn + ' takenConsent=' + takenConsent +
    ' ran=' + window.ranArgs.length + ' moved=' + window.movedArgs.length
}, 5200)`)
	n := func(key string) int {
		m := regexp.MustCompile(`\b` + key + `=(-?\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	if n("sameWhy") != 1 {
		t.Errorf("a rename to the name it already has says nothing beside the box: %q",
			title)
	}
	if n("samePanel") != 0 {
		t.Errorf("a refusal with no review opened the review panel anyway: %q", title)
	}
	if n("takenWhy") != 0 {
		t.Errorf("the refusal from the earlier attempt is still beside the box: %q",
			title)
	}
	if n("taken") != 1 {
		t.Errorf("a name something else already has is not refused in the review: %q",
			title)
	}
	if n("takenBtn") != 0 {
		t.Errorf("the button under a refused rename is live: %q", title)
	}
	if n("takenConsent") != 0 {
		t.Errorf("the window offered to replace the routine that is in the way: %q",
			title)
	}
	if n("ran") != 0 || n("moved") != 0 {
		t.Errorf("a refused rename wrote something: %q", title)
	}
}
