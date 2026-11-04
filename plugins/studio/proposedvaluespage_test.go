package main

// A PROPOSAL IN THE BOX, AND WHAT THE PAGE DOES WITH IT.
//
// Four things here belong to the page and cannot be taken from a backend test: a box FILLED from
// what the request said, the clause beside it changing the moment somebody types over it, what is
// SENT when Play is pressed — both what was offered and what came back, plus the revision — and a
// proposal being dropped before the next request is asked.
//
// The contract derives the same three answers from the same two facts and seals its own into the
// confirmation; `internal/orchestrator/proposedvalues_test.go` holds that. This is the field
// label, not the authority.
//
// See [[ADR-277-a-request-may-propose-the-text-a-routine-types]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// proposalHarness is a window where Director answers the first request with a proposal and the
// second with none.
//
// NO BACKTICK MAY APPEAR IN THIS FILE'S HARNESS COMMENTS — it is a Go raw string, and one closed
// ten of these files at once.
const proposalHarness = `
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
      // THE FIRST REQUEST PROPOSES A VALUE; THE SECOND NAMES THE SAME ROUTINE AND
      // PROPOSES NOTHING, which is what must clear the box.
      if (window.__asks === 1 || window.__asks === 3) {
        return Promise.resolve({answer: {kind: 'run',
          id: 'notepad/focus/write-a-note', app: 'notepad',
          slug: 'write-a-note', focus: true, name: 'write a note',
          scope: 'app-anywhere', reply: 'Writing it.', offered: 1,
          source: 'advisor', resolver: 'the resolver answered this',
          library: 'C:/x/routes', sourceAt: 'sha256:abc',
          values: [{name: 'message', value: 'hello there',
                    from: 'extracted'}],
          context: {app: 'notepad', basis: 'stated',
                    says: 'read against notepad, which you named'}}})
      }
      return Promise.resolve({answer: {kind: 'run',
        id: 'notepad/focus/write-a-note', app: 'notepad',
        slug: 'write-a-note', focus: true, name: 'write a note',
        scope: 'app-anywhere', reply: 'Writing it.', offered: 1,
        source: 'exact', resolver: 'the resolver was not asked',
        library: 'C:/x/routes',
        context: {app: 'notepad', basis: 'stated',
                  says: 'read against notepad, which you named'}}})
    },
  },
  Acts: {
    // THE WIDGET ACTION (S7), polled from start-up: the page draws whatever it says.
    WidgetStatus: () => Promise.resolve({state: 'ready', says: '', problems: []}),
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
    PlayNeeds: (id) => {
      if (id === 'notepad/focus/write-a-note') {
        return Promise.resolve({id: id, name: 'write a note', needs: true,
          fields: [{name: 'message', secret: false,
                    says: 'typed into the routine'},
                   // A FIELD NOTHING EVER PROPOSES FOR, filled in by hand. The
                   // discriminating half of the drop rule: clearing every box
                   // would pass the inheritance test and destroy somebody's
                   // own typing.
                   {name: 'subject', secret: false,
                    says: 'typed into the routine'},
                   {name: 'password', secret: true,
                    says: 'read from the credential store'}]})
      }
      // A SECOND ROUTINE THAT TAKES THE SAME ARGUMENT NAME. The proposal was
      // read for one routine's arguments, and however alike the names, it must
      // not appear beside another's.
      if (id === 'notepad/focus/also-types') {
        return Promise.resolve({id: id, name: 'also types', needs: true,
          fields: [{name: 'message', secret: false,
                    says: 'typed into the routine'}]})
      }
      return Promise.resolve({id: id, name: 'other', needs: false, fields: []})
    },
    ReviewPlay: (...args) => {
      // EVERY ARGUMENT, RECORDED. What was in the boxes, what was offered, and
      // the revision - because the page sending only the first is the defect
      // this exists to catch.
      window.__sent = (window.__sent || []).concat(JSON.stringify(
        {n: args.length, values: args[1], proposed: args[2], at: args[3]}))
      return Promise.resolve({id: args[0], name: 'write a note', token: 'tok',
        says: ['message \u2014 "hello there" (read out of what you said)']})
    },
    ConfirmPlay: () => Promise.resolve(''),
    CancelPlay: () => {},
    PlayRoutine: (id) => Promise.resolve(''),
    Routines: () => {
      const row = (id, slug) => ({id: id, name: slug.replace(/-/g, ' '), slug: slug,
        app: 'notepad', scope: 'app-anywhere', contract: 'legacy', runnable: true,
        source: 'use os.', generation: 'unstated', rebuildable: true,
        offer: true, wrote: 'x'})
      return Promise.resolve([row('notepad/focus/write-a-note', 'write-a-note'),
                              row('notepad/focus/also-types', 'also-types'),
                              row('notepad/focus/other', 'other')])
    },
  },
}}
const box = () => document.getElementById('routineArg-message')
const other = () => document.getElementById('routineArg-subject')
const clause = () => {
  const el = document.getElementById('routineArgFrom-message')
  return el ? el.textContent : ''
}
const typeInto = (id, said) => {
  const el = document.getElementById(id)
  el.value = said
  el.dispatchEvent(new Event('input'))
}
const pickId = (id) => {
  // A ROUTINE IS OPENED BY ITS DISCLOSURE, which is the only control that
  // toggles the row -- every other press inside it belongs to the routine.
  const c = document.getElementById('routineOpen-' + id.replace(/[^a-z0-9]+/gi, '-'))
  if (!c) { return }
  c.click()
  // AND LEAVING A ROUTINE SOMEBODY HAS TYPED INTO ASKS FIRST. These tests are
  // about what happens once that is agreed to, and the press that agrees is the
  // same press -- so this opens it and answers, which is what a person does.
  const warn = document.getElementById('routineLeaving')
  if (warn && !warn.classList.contains('hide')) { c.click() }
}
const ask = (said) => {
  document.getElementById('askBox').value = said
  document.getElementById('askBtn').click()
}
let pFilled = '', pClause = '', pDead = 1, pOtherAtFirst = 'unset'
let pAfterEdit = '', pEditClause = '', pOtherClause = 'unset'
let pSent = '', pSentAt = '', pSentN = 0, pSentProposed = '', pSentOther = ''
let pSecondFilled = 'unset', pSecondClause = 'unset', pOtherKept = 'unset'
let pElsewhere = 'unset', pElsewhereClause = 'unset'

setTimeout(() => { ask('write a note saying hello there') }, 900)
setTimeout(() => {
  // THE BOX IS FILLED FROM THE REQUEST, AND SAYS SO.
  pFilled = box() ? box().value : ''
  pClause = clause()
  // AND THE FIELD NOTHING PROPOSED FOR IS EMPTY AND UNLABELLED.
  pOtherAtFirst = other() ? other().value : 'nobox'
  pOtherClause = document.getElementById('routineArgFrom-subject')
    ? document.getElementById('routineArgFrom-subject').textContent : 'nomark'
  typeInto('routineArg-subject', 'by hand')
}, 1700)
setTimeout(() => {
  // AND PLAY IS LIVE, because a filled box is a value GIVEN - the proposal's and
  // the typed one together. A box that looked filled while nothing stood behind it
  // would leave the button dead with the text in front of somebody.
  pDead = document.getElementById('routinePlayBtn').disabled ? 1 : 0
}, 1950)
setTimeout(() => { typeInto('routineArg-message', 'goodbye instead') }, 2150)
setTimeout(() => {
  // CORRECTED, AND THE CLAUSE FOLLOWS IMMEDIATELY - repainted on the keystroke
  // rather than when the fields are next rebuilt.
  pAfterEdit = box() ? box().value : ''
  pEditClause = clause()
  document.getElementById('routinePlayBtn').click()
}, 2400)
setTimeout(() => {
  const sent = (window.__sent || [])
  const last = sent.length ? JSON.parse(sent[sent.length - 1]) : {}
  pSentN = last.n || 0
  pSent = (last.values && last.values.message) || ''
  pSentOther = (last.values && last.values.subject) || ''
  pSentProposed = (last.proposed && last.proposed.message) || ''
  pSentAt = last.at || ''
  // A SECOND REQUEST, NAMING THE SAME ROUTINE AND PROPOSING NOTHING.
  ask('write a note')
}, 2900)
setTimeout(() => {
  pSecondFilled = box() ? box().value : 'nobox'
  pSecondClause = clause()
  pOtherKept = other() ? other().value : 'nobox'
}, 3700)
// AND A THIRD REQUEST PROPOSES AGAIN, so the routine-keyed check below is made against a
// proposal that is actually standing rather than one already cleared.
setTimeout(() => { ask('write a note saying hello there') }, 4000)
setTimeout(() => { pickId('notepad/focus/also-types') }, 4600)
setTimeout(() => {
  // ANOTHER ROUTINE'S FIELD OF THE SAME NAME IS EMPTY AND UNLABELLED.
  pElsewhere = box() ? box().value : 'nobox'
  pElsewhereClause = clause()
  document.title = 'filled=' + (pFilled === 'hello there' ? 1 : 0) +
    ' elsewhereempty=' + (pElsewhere === '' ? 1 : 0) +
    ' elsewhereclause=' + (pElsewhereClause === '' ? 1 : 0) +
    ' otherempty=' + (pOtherAtFirst === '' ? 1 : 0) +
    ' otherclause=' + (pOtherClause === '' ? 1 : 0) +
    ' sentother=' + (pSentOther === 'by hand' ? 1 : 0) +
    ' otherkept=' + (pOtherKept === 'by hand' ? 1 : 0) +
    ' clause=' + (pClause === 'read out of what you said' ? 1 : 0) +
    ' live=' + (pDead ? 0 : 1) +
    ' edited=' + (pAfterEdit === 'goodbye instead' ? 1 : 0) +
    ' editclause=' + (pEditClause === 'as you corrected it' ? 1 : 0) +
    ' sentn=' + pSentN +
    ' sentvalue=' + (pSent === 'goodbye instead' ? 1 : 0) +
    ' sentproposed=' + (pSentProposed === 'hello there' ? 1 : 0) +
    ' sentat=' + (pSentAt === 'sha256:abc' ? 1 : 0) +
    ' secondempty=' + (pSecondFilled === '' ? 1 : 0) +
    ' secondclause=' + (pSecondClause === '' ? 1 : 0) +
    ' err=' + (window.bad ? 1 : 0)
}, 5200)
`

// aRenderedProposalPage renders the real page and reads its counters.
func aRenderedProposalPage(t *testing.T) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, proposalHarness)
	if !strings.Contains(title, "filled=") {
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

// A PROPOSED VALUE FILLS AN EDITABLE BOX, SAYS WHERE IT CAME FROM, AND TRAVELS BACK WHOLE.
func TestAProposalFillsTheBoxAndSaysWhereItCameFrom(t *testing.T) {
	title, n := aRenderedProposalPage(t)
	if n("err") != 0 {
		t.Fatalf("the page threw: %q", title)
	}
	if n("filled") != 1 {
		t.Errorf("the box was not filled from the request: %q", title)
	}
	if n("clause") != 1 {
		t.Errorf("the box does not say the value was read out of the request: %q", title)
	}
	// AND THE FILLED BOX IS A VALUE GIVEN, so Play is live.
	if n("live") != 1 {
		t.Errorf("Play is dead although the box is filled: %q", title)
	}
}

// AND CORRECTING IT IS A CORRECTION, SAID SO AT ONCE.
func TestCorrectingAProposalIsSaidImmediately(t *testing.T) {
	title, n := aRenderedProposalPage(t)
	if n("err") != 0 {
		t.Fatalf("the page threw: %q", title)
	}
	if n("edited") != 1 {
		t.Errorf("the box did not take the correction: %q", title)
	}
	if n("editclause") != 1 {
		t.Errorf("a corrected value is still credited to the resolver: %q", title)
	}
}

// AND PLAY SENDS WHAT WAS OFFERED, WHAT CAME BACK, AND THE REVISION.
//
// The discriminating one. A page that sent only the values would make every proposal look like
// text somebody typed, and the contract could not tell a reading that was kept from one that was
// fixed — nor refuse a routine that had been saved since.
func TestPlaySendsTheProposalAndTheRevisionBesideTheValues(t *testing.T) {
	title, n := aRenderedProposalPage(t)
	if n("err") != 0 {
		t.Fatalf("the page threw: %q", title)
	}
	if n("sentn") != 4 {
		t.Errorf("the review was called with %d argument(s), want four: %q",
			n("sentn"), title)
	}
	if n("sentvalue") != 1 {
		t.Errorf("what will run was not sent as it stands: %q", title)
	}
	if n("sentproposed") != 1 {
		t.Errorf("what was proposed was not sent, so a correction cannot be told "+
			"from a reading that was kept: %q", title)
	}
	if n("sentat") != 1 {
		t.Errorf("the revision the names were read from was not sent: %q", title)
	}
}

// AND A FIELD NOTHING PROPOSED FOR IS EMPTY, UNLABELLED, AND THE PERSON'S OWN.
//
// Three claims about the same box, and the third is the one that keeps the drop rule honest: a
// `forgetProposals` that cleared every box would pass the inheritance test below and destroy
// somebody's own typing on the way.
func TestAFieldNothingProposedForIsTheirsAlone(t *testing.T) {
	title, n := aRenderedProposalPage(t)
	if n("err") != 0 {
		t.Fatalf("the page threw: %q", title)
	}
	if n("otherempty") != 1 {
		t.Errorf("a field nothing proposed for was filled anyway: %q", title)
	}
	if n("otherclause") != 1 {
		t.Errorf("a field nothing proposed for carries a clause: %q", title)
	}
	if n("sentother") != 1 {
		t.Errorf("the hand-typed value did not reach the review: %q", title)
	}
	if n("otherkept") != 1 {
		t.Errorf("a new request destroyed text the person typed themselves: %q", title)
	}
}

// AND A PROPOSAL BELONGS TO THE ROUTINE IT WAS READ FOR.
//
// A reading is of ONE routine's arguments. Another routine that happens to declare the same name
// is a different routine, and prefilling its box would put text read for somewhere else in front
// of somebody as though their request had asked for it.
func TestAProposalDoesNotReachAnotherRoutinesFieldOfTheSameName(t *testing.T) {
	title, n := aRenderedProposalPage(t)
	if n("err") != 0 {
		t.Fatalf("the page threw: %q", title)
	}
	if n("elsewhereempty") != 1 {
		t.Errorf("another routine's field was filled from this one's reading: %q", title)
	}
	if n("elsewhereclause") != 1 {
		t.Errorf("another routine's field claims a value was read out of the "+
			"request: %q", title)
	}
}

// AND A PROPOSAL IS DROPPED BEFORE THE NEXT REQUEST IS ASKED.
//
// A proposal is a reading of ONE request. Left standing, a second request naming the same routine
// would arrive with the first one's text already in the box, and somebody would be confirming
// words they said a minute ago about something else.
func TestASecondRequestDoesNotInheritTheFirstsProposal(t *testing.T) {
	title, n := aRenderedProposalPage(t)
	if n("err") != 0 {
		t.Fatalf("the page threw: %q", title)
	}
	if n("secondempty") != 1 {
		t.Errorf("the second request inherited the first's text: %q", title)
	}
	if n("secondclause") != 1 {
		t.Errorf("the second request still claims a value was read out of it: %q", title)
	}
}
