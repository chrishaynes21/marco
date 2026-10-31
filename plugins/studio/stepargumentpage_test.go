package main

// THE CONTROL THAT MAKES A STEP ASK FOR ITS TEXT, RENDERED.
//
// Three things belong to the page here: the control appears only on a row that TYPES text, an
// empty name is not sent (the box says what is wanted better than a refusal would), and the
// summary line NAMES this operation rather than borrowing another's sentence — which it did, and
// would have told somebody their step was being deleted.
//
// The rules about what a name may be, and what the edit renders to, are the contract's;
// `stepargumentacts_test.go` asks those of the real `Acts`.
//
// See [[ADR-276-a-routine-may-be-told-what-to-type]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// argStepHarness is a window whose routine has one step that types text and three that do not,
// plus one whose text is already supplied.
//
// NO BACKTICK MAY APPEAR IN THIS FILE'S HARNESS COMMENTS — it is a Go raw string.
const argStepHarness = `
window.addEventListener('error', (e) => {
  window.bad = (window.bad || '') + '[' + e.message + ']'
  document.title = 'the page threw: ' + window.bad
})
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
    Library: () => Promise.resolve({dir: 'C:/x/routes'}),
    Recording: () => Promise.resolve({active: false}),
    Ready: () => Promise.resolve({build: 'abc1234', problems: [], limits: 'nothing'}),
    CancelReview: () => {},
    ReplaceWithRebuilt: () => Promise.resolve(''),
    ReviewRebuilt: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    ScopeChoices: () => Promise.resolve([]),
    CancelMove: () => {},
    PlaybackTarget: () => Promise.resolve({chosen: '', windows: []}),
    RoutineAnswers: (id) => Promise.resolve({id: id, phrases: [], askable: true}),
    AddRoutineAnswer: (id) => Promise.resolve({id: id, phrases: []}),
    RemoveRoutineAnswer: (id) => Promise.resolve({id: id, phrases: []}),
    PlayNeeds: (id) => Promise.resolve({id: id, fields: [], needs: false}),
    ReviewPlay: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    ConfirmPlay: () => Promise.resolve(''),
    CancelPlay: () => {},
    PlayRoutine: () => Promise.resolve(''),
    StepEditing: (id) => Promise.resolve({id: id, name: 'say hello', editable: true,
      basis: 'basis-one',
      rows: [
        {at: 0, says: 'bring notepad to the front'},
        {at: 1, says: 'type "hello"', types: true, text: 'hello'},
        {at: 2, says: 'wait 250ms', wait: true, ms: 250},
        {at: 3, says: 'type what you supply', supplied: true, named: 'subject'},
      ]}),
    ReviewStepEdit: (id, basis, changes) => {
      window.sent = JSON.stringify(changes)
      return Promise.resolve({id: id, name: 'say hello', token: 'tok',
        was: 'old', now: 'new', doing: ['the text at step 2 becomes something ' +
          'you supply, named "message" (it was "hello")']})
    },
    CancelStepEdit: () => {},
    SaveStepEdit: () => Promise.resolve({why: 'not what this asks about'}),
    Routines: () => Promise.resolve([{id: 'notepad/focus/say-hello',
      name: 'say hello', slug: 'say-hello', app: 'notepad', scope: 'app-anywhere',
      contract: 'legacy', runnable: true, source: 'use os.',
      generation: 'unstated', rebuildable: true, offer: true, wrote: 'x'}])
  },
}}
const there = (id) => !!document.getElementById(id)
const press = (id) => { document.getElementById(id).click() }
const asked = () => {
  const el = document.getElementById('routineStepAsked')
  return (el && !el.classList.contains('hide')) ? el.textContent : ''
}
let aBox = 0, aBtn = 0, noBoxElsewhere = 1, alreadySaid = ''
let emptySent = 'unset', emptyAsked = 'unset'
let namedSent = 'unset', namedAsked = ''
let undone = 'unset'

setTimeout(() => {
  const card = document.getElementById('routine-notepad-focus-say-hello')
  if (card) { card.click() }
}, 900)
setTimeout(() => { press('routineStepsBtn') }, 1400)
setTimeout(() => {
  // THE CONTROL IS ON THE ROW THAT TYPES TEXT, AND ON NO OTHER.
  aBox = there('routineStepArgName-1') ? 1 : 0
  aBtn = there('routineStepArg-1') ? 1 : 0
  noBoxElsewhere = (there('routineStepArgName-0') || there('routineStepArgName-2') ||
    there('routineStepArgName-3')) ? 0 : 1
  // AND A STEP WHOSE TEXT IS ALREADY SUPPLIED SAYS SO INSTEAD.
  const already = document.getElementById('routineStepArgAlready-3')
  alreadySaid = already ? already.textContent : 'nomark'
  // AN EMPTY NAME IS NOT SENT.
  press('routineStepArg-1')
}, 1900)
setTimeout(() => {
  emptySent = window.sent === undefined ? 'nothing' : window.sent
  emptyAsked = asked()
  const box = document.getElementById('routineStepArgName-1')
  box.value = 'message'
  press('routineStepArg-1')
}, 2300)
setTimeout(() => {
  // THE SUMMARY NAMES THIS OPERATION, rather than saying the step is removed.
  namedAsked = asked()
  press('routineStepReviewBtn')
}, 2700)
setTimeout(() => {
  namedSent = window.sent === undefined ? 'nothing' : window.sent
  // AND IT CAN BE TAKEN BACK.
  press('routineStepArg-1')
}, 3100)
setTimeout(() => {
  undone = asked()
  document.title = 'box=' + aBox + ' btn=' + aBtn +
    ' noboxelsewhere=' + noBoxElsewhere +
    ' already=' + (alreadySaid.indexOf('subject') >= 0 ? 1 : 0) +
    ' emptynotsent=' + (emptySent === 'nothing' ? 1 : 0) +
    ' emptynotasked=' + (emptyAsked === '' ? 1 : 0) +
    ' saysargument=' + (namedAsked.indexOf('asks you for its text') >= 0 ? 1 : 0) +
    ' saysname=' + (namedAsked.indexOf('message') >= 0 ? 1 : 0) +
    ' notremoved=' + (namedAsked.indexOf('removed') < 0 ? 1 : 0) +
    ' sentop=' + (namedSent.indexOf('"op":"argument"') >= 0 ? 1 : 0) +
    ' sentname=' + (namedSent.indexOf('"name":"message"') >= 0 ? 1 : 0) +
    ' undone=' + (undone === '' ? 1 : 0) +
    ' err=' + (window.bad ? 1 : 0)
}, 3500)
`

func aRenderedArgStepPage(t *testing.T) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, argStepHarness)
	if !strings.Contains(title, "box=") {
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

// THE CONTROL IS OFFERED ONLY WHERE THERE IS TEXT TO SUPPLY.
func TestOnlyAStepThatTypesTextOffersToBeSupplied(t *testing.T) {
	title, n := aRenderedArgStepPage(t)
	if n("err") != 0 {
		t.Fatalf("the page threw: %q", title)
	}
	if n("box") != 1 || n("btn") != 1 {
		t.Errorf("the typing step has no control: %q", title)
	}
	// A BOX BESIDE A CLICK WOULD BE A CONTROL THAT REFUSES, and the refusal would arrive
	// after somebody had typed a name into it.
	if n("noboxelsewhere") != 1 {
		t.Errorf("a step that types nothing offers the control: %q", title)
	}
	if n("already") != 1 {
		t.Errorf("a step whose text is already supplied does not say so: %q", title)
	}
}

// AND AN EMPTY NAME IS NOT SENT.
//
// The box says what is wanted better than a refusal about nothing would.
func TestAnEmptyNameIsNotSent(t *testing.T) {
	title, n := aRenderedArgStepPage(t)
	if n("err") != 0 {
		t.Fatalf("the page threw: %q", title)
	}
	if n("emptynotsent") != 1 {
		t.Errorf("an empty name was sent to the contract: %q", title)
	}
	if n("emptynotasked") != 1 {
		t.Errorf("an empty name was recorded as a change: %q", title)
	}
}

// AND THE SUMMARY NAMES THIS OPERATION RATHER THAN BORROWING ANOTHER'S SENTENCE.
//
// It borrowed one: the line was a ternary that read "step N is removed" for anything that was not
// a wait, so this operation would have told somebody their step was being DELETED — a false
// sentence about their own routine, in the one place that exists to say what is about to happen.
func TestTheSummaryNamesTheSuppliedStepAndNotARemoval(t *testing.T) {
	title, n := aRenderedArgStepPage(t)
	if n("err") != 0 {
		t.Fatalf("the page threw: %q", title)
	}
	if n("saysargument") != 1 || n("saysname") != 1 {
		t.Errorf("the summary does not say what was asked: %q", title)
	}
	if n("notremoved") != 1 {
		t.Errorf("the summary says the step is removed: %q", title)
	}
	// AND WHAT IS SENT CARRIES THE OPERATION AND THE NAME.
	if n("sentop") != 1 || n("sentname") != 1 {
		t.Errorf("the change sent to the contract is wrong: %q", title)
	}
	// AND IT CAN BE TAKEN BACK BEFORE ANY REVIEW.
	if n("undone") != 1 {
		t.Errorf("the change could not be undone: %q", title)
	}
}
