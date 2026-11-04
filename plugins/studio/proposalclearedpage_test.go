package main

// AN ANSWER THAT SELECTS NOTHING STILL CLEARS THE LAST READING.
//
// Its own harness, because the case is a SEQUENCE: a request that proposed something, then one
// Director answers with a question. Nothing is selected, so nothing takes a new reading — and the
// clear therefore cannot live where a new reading is taken. It runs before the asking.
//
// Found by a mutation. Removing the clear from before the ask survived every test in
// `proposedvaluespage_test.go`, because every answer there selects a routine and the taking of a
// new reading clears the old one on its way past.
//
// See [[ADR-277-a-request-may-propose-the-text-a-routine-types]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// clearedHarness proposes on the first request and answers the second with a question.
//
// NO BACKTICK MAY APPEAR IN THIS FILE'S HARNESS COMMENTS — it is a Go raw string.
const clearedHarness = `
window.addEventListener('error', (e) => {
  window.bad = (window.bad || '') + '[' + e.message + ']'
  document.title = 'the page threw: ' + window.bad
})
window.__asks = 0
window.go = {main: {
  Backend: {
    Loaded: () => Promise.resolve(),
    SuggestRoutine: (text, target) => {
      window.__asks = (window.__asks || 0) + 1
      if (window.__asks === 1) {
        return Promise.resolve({answer: {kind: 'run',
          id: 'notepad/focus/write-a-note', app: 'notepad',
          slug: 'write-a-note', focus: true, name: 'write a note',
          scope: 'app-anywhere', reply: 'Writing it.', offered: 2,
          source: 'advisor', resolver: 'the resolver answered this',
          library: 'C:/x/routes', sourceAt: 'sha256:abc',
          values: [{name: 'message', value: 'hello there',
                    from: 'extracted'}],
          context: {app: 'notepad', basis: 'stated',
                    says: 'read against notepad, which you named'}}})
      }
      // A QUESTION. Nothing is selected, so nothing takes a new reading - and the
      // last one must be gone all the same.
      return Promise.resolve({answer: {kind: 'clarify', id: '',
        reply: 'More than one routine answers to that.', offered: 2,
        source: 'fallback', resolver: 'the resolver was asked and did not answer',
        library: 'C:/x/routes', candidates: ['a', 'b'],
        context: {app: 'notepad', basis: 'stated',
                  says: 'read against notepad, which you named'}}})
    },
  },
  Acts: {
    // THE WIDGET ACTION (S7), polled from start-up: the page draws whatever it says.
    WidgetStatus: () => Promise.resolve({state: 'ready', says: '', problems: []}),
    Theme: () => Promise.resolve({asked: '', theme: 'default', light: false, accentFrom: 'name', tokens: {}}),
    SuggestTargets: () => Promise.resolve([{app: 'notepad', open: true, context: 1}]),
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
    Ready: () => Promise.resolve({build: 'abc1234', problems: [], limits: 'nothing'}),
    CancelReview: () => {},
    ReplaceWithRebuilt: () => Promise.resolve(''),
    ReviewRebuilt: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    ScopeChoices: () => Promise.resolve([]),
    CancelMove: () => {},
    StepEditing: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    ReviewStepEdit: () => Promise.resolve({why: 'not what this asks about'}),
    CancelStepEdit: () => {},
    SaveStepEdit: () => Promise.resolve({why: 'not what this asks about'}),
    RoutineAnswers: (id) => Promise.resolve({id: id, phrases: [], askable: true}),
    AddRoutineAnswer: (id) => Promise.resolve({id: id, phrases: []}),
    RemoveRoutineAnswer: (id) => Promise.resolve({id: id, phrases: []}),
    PlaybackTarget: () => Promise.resolve({chosen: '4660:77',
      windows: [{id: '4660:77', label: 'a window',
        says: 'Play brings a window to the front and presses there.'}]}),
    PlayNeeds: (id) => Promise.resolve({id: id, name: 'write a note', needs: true,
      fields: [{name: 'message', secret: false, says: 'typed into the routine'}]}),
    ReviewPlay: (...args) => {
      window.__sent = (window.__sent || []).concat(JSON.stringify(
        {values: args[1], proposed: args[2], at: args[3]}))
      return Promise.resolve({id: args[0], name: 'write a note', token: 'tok',
        says: ['message \u2014 "hello there"']})
    },
    ConfirmPlay: () => Promise.resolve(''),
    CancelPlay: () => {},
    PlayRoutine: () => Promise.resolve(''),
    Routines: () => Promise.resolve([{id: 'notepad/focus/write-a-note',
      name: 'write a note', slug: 'write-a-note', app: 'notepad',
      scope: 'app-anywhere', contract: 'legacy', runnable: true,
      source: 'use os.', generation: 'unstated', rebuildable: true,
      offer: true, wrote: 'x'}])
  },
}}
const box = () => document.getElementById('routineArg-message')
const clause = () => {
  const el = document.getElementById('routineArgFrom-message')
  return el ? el.textContent : ''
}
const ask = (said) => {
  document.getElementById('askBox').value = said
  document.getElementById('askBtn').click()
}
// THE CARD IS UP WHEN ITS CONTAINER IS SHOWN, which is what hides it. Asking the ROWS
// whether they are hidden answers about the wrong element: they are never given the class,
// their parent is, so a hidden card reads as a visible one holding its old text.
const cardUp = () => {
  const el = document.getElementById('routineWillType')
  return (el && !el.classList.contains('hide')) ? 1 : 0
}
const confirmDead = () => {
  const b = document.getElementById('routineConfirmBtn')
  return (!b || b.disabled) ? 1 : 0
}
let cFilled = 'unset', cClause = 'unset'
let cAfter = 'unset', cAfterClause = 'unset', cSaid = ''
let cSentProposed = 'unset', cSentAt = 'unset'
let cCardUp = 0, cCardDead = 1, cCardGone = 0, cCardStillLive = 1

setTimeout(() => { ask('write a note saying hello there') }, 900)
setTimeout(() => {
  cFilled = box() ? box().value : 'nobox'
  cClause = clause()
  // A QUESTION NOW.
  ask('settings')
}, 1800)
setTimeout(() => {
  cAfter = box() ? box().value : 'nobox'
  cAfterClause = clause()
  cSaid = document.getElementById('askSaid').textContent
  // AND PRESSING PLAY AFTER IT MUST CARRY NO PROPOSAL AND NO REVISION. The box is
  // empty, so Play is dead -- type something and press, which is the state somebody
  // is actually in after a question they then answered by hand.
  const el = box()
  if (el) { el.value = 'typed by hand'; el.dispatchEvent(new Event('input')) }
}, 2700)
setTimeout(() => { document.getElementById('routinePlayBtn').click() }, 3000)
setTimeout(() => {
  const sent = (window.__sent || [])
  const last = sent.length ? JSON.parse(sent[sent.length - 1]) : {}
  cSentProposed = JSON.stringify(last.proposed || {})
  cSentAt = last.at === undefined ? 'missing' : last.at
  // THE CONFIRMATION IS UP, WITH A LIVE BUTTON ON IT.
  cCardUp = cardUp()
  cCardDead = confirmDead()
  // AND A NEW REQUEST NOW.
  ask('something else entirely')
}, 3600)
setTimeout(() => {
  // THE CARD IS GONE AND THE BUTTON WITH IT. A confirmation prepared before this
  // request was asked is a confirmation of something else, and it carried a token
  // somebody could still have pressed.
  cCardGone = cardUp() ? 0 : 1
  cCardStillLive = confirmDead() ? 0 : 1
  document.title = 'filled=' + (cFilled === 'hello there' ? 1 : 0) +
    ' cardup=' + cCardUp + ' carddead=' + cCardDead +
    ' cardgone=' + cCardGone + ' cardlive=' + cCardStillLive +
    ' clause=' + (cClause === 'read out of what you said' ? 1 : 0) +
    ' cleared=' + (cAfter === '' ? 1 : 0) +
    ' clauseGone=' + (cAfterClause === '' ? 1 : 0) +
    ' asked=' + (cSaid.includes('More than one') ? 1 : 0) +
    ' noproposal=' + (cSentProposed === '{}' ? 1 : 0) +
    ' norevision=' + (cSentAt === '' ? 1 : 0) +
    ' err=' + (window.bad ? 1 : 0)
}, 4500)
`

// A QUESTION CLEARS THE LAST READING, AND WHAT IS PRESSED AFTERWARDS CARRIES NONE.
func TestAnAnswerThatSelectsNothingClearsTheLastProposal(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, clearedHarness)
	if !strings.Contains(title, "filled=") {
		t.Fatalf("the page never reported its counters: %q", title)
	}
	n := func(key string) int {
		m := regexp.MustCompile(key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	if n("err") != 0 {
		t.Fatalf("the page threw: %q", title)
	}
	// THE FIRST REQUEST PROPOSED, so there is something for the second to clear.
	if n("filled") != 1 || n("clause") != 1 {
		t.Fatalf("the first request did not propose anything: %q", title)
	}
	if n("asked") != 1 {
		t.Fatalf("the second answer was not the question it was given: %q", title)
	}
	if n("cleared") != 1 {
		t.Errorf("a question left the last request's text in the box: %q", title)
	}
	if n("clauseGone") != 1 {
		t.Errorf("a question left the box claiming its text was read out of the "+
			"request: %q", title)
	}
	// AND WHAT IS PRESSED AFTERWARDS IS THE PERSON'S OWN, CREDITED TO NOBODY.
	if n("noproposal") != 1 {
		t.Errorf("a press after the question still carried a proposal: %q", title)
	}
	if n("norevision") != 1 {
		t.Errorf("a press after the question still named a revision: %q", title)
	}
}

// AND A NEW REQUEST TAKES THE CONFIRMATION CARD WITH IT.
//
// The card carries a TOKEN, and a token is a press. One prepared before this request was asked is
// a confirmation of something else — of a routine the person may no longer have chosen, with text
// read out of words they are no longer saying — so leaving it up leaves a live button over a
// decision that has moved on.
//
// Written because a mutation found the claim only in a comment: making the clear a no-op survived
// every other test here.
func TestANewRequestTakesTheConfirmationCardWithIt(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, clearedHarness)
	if !strings.Contains(title, "cardup=") {
		t.Fatalf("the page never reported its counters: %q", title)
	}
	n := func(key string) int {
		m := regexp.MustCompile(key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	if n("err") != 0 {
		t.Fatalf("the page threw: %q", title)
	}
	// THE CARD WAS UP AND PRESSABLE, or there is nothing for the next request to clear.
	if n("cardup") != 1 || n("carddead") != 0 {
		t.Fatalf("no live confirmation was prepared, so this proves nothing: %q", title)
	}
	if n("cardgone") != 1 {
		t.Errorf("a new request left the old confirmation on screen: %q", title)
	}
	if n("cardlive") != 0 {
		t.Errorf("a new request left a live Run button over the old "+
			"confirmation: %q", title)
	}
}
