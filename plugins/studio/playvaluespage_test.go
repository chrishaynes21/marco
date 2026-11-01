package main

// THE FIELDS BESIDE PLAY, RENDERED.
//
// Three of these cannot be taken from a backend test, because the branch belongs to the page:
// **Play is dead until every text field has been GIVEN a value** — given, not non-empty, which is
// the difference between a box somebody cleared on purpose and one they never touched; the
// confirmation is a card that appears before anything presses; and the text is dropped when the
// chosen routine changes rather than carried to another card.
//
// The backend is stubbed, as next door: `playvaluesacts_test.go` asks the same questions of the
// real `Acts` against a real library, and the contract is `internal/orchestrator`'s.
//
// See [[Text-arguments-proposal-2026-10-30]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// playValuesHarness is a window with one routine that needs text and one that needs none.
//
// The review refuses the FIRST time on purpose — a value this substitution cannot write down —
// because what the page does with that refusal is a branch no backend test can take: the card
// stays, the reason is on it, and the button beside it is dead.
const playValuesHarness = `
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
    // WHICH PARTS OF THIS WINDOW ARE OFFERED, AND WHAT IT EXPLAINS.
    // Both are read once at startup; a stub without them is a page
    // whose rail never paints. See help.go.
    Features: () => Promise.resolve({acts: true, scenes: true,
      views: ['routines', 'acts', 'scenes', 'history', 'help']}),
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
    // ONE WINDOW TO PRESS IN, so Play is dead for the one reason this file is about.
    PlaybackTarget: () => Promise.resolve({chosen: '4660:77',
      windows: [{id: '4660:77', label: 'a window',
        says: 'Play brings a window to the front and presses there.'}]}),
    PlayNeeds: (id) => {
      if (id === 'notepad/focus/write-a-note') {
        return Promise.resolve({id: id, name: 'write a note', needs: true,
          fields: [{name: 'message', secret: false,
                    says: 'typed into the routine'},
                   {name: 'password', secret: true,
                    says: 'read from the credential store'}]})
      }
      return Promise.resolve({id: id, name: 'other', needs: false, fields: []})
    },
    ReviewPlay: (...args) => {
      window.pvReviewed = (window.pvReviewed || 0) + 1
      // THE WHOLE CALL, not its arity. A count of arguments was what this
      // recorded when the review took two, and a count cannot say that an
      // ordinary press proposes nothing and names no revision -- which is the
      // claim worth holding now that it can carry both.
      window.pvSent = (window.pvSent || []).concat(
        args.length + ':' + JSON.stringify(args[1]) + ':' +
        JSON.stringify(args[2]) + ':' + JSON.stringify(args[3]))
      if (window.pvReviewed === 1) {
        return Promise.resolve({id: args[0], name: 'write a note',
          why: 'the value given for "message" runs over more than one line, and ' +
            'a routine cannot type that yet'})
      }
      return Promise.resolve({id: args[0], name: 'write a note', token: 'tok-run',
        says: ['message \u2014 "hello there"',
               'password \u2014 from the credential store']})
    },
    ConfirmPlay: (...args) => {
      window.pvConfirmed = (window.pvConfirmed || []).concat(
        args.length + ':' + args[0] + ':' + args[1])
      return Promise.resolve('')
    },
    CancelPlay: (token) => { window.pvCancelled = token },
    PlayRoutine: (id) => { window.pvPlayed = id; return Promise.resolve('') },
    Routines: () => {
      const row = (id, slug) => ({id: id, name: slug.replace(/-/g, ' '), slug: slug,
        app: 'notepad', scope: 'app-anywhere', contract: 'legacy', runnable: true,
        source: 'use os.', generation: 'unstated', rebuildable: true,
        offer: true, wrote: 'x'})
      return Promise.resolve([row('notepad/focus/write-a-note', 'write-a-note'),
                              row('notepad/focus/other', 'other')])
    },
  },
}}
const press = (id) => { document.getElementById(id).click() }
const there = (id) => !!document.getElementById(id)
const shown = (id) => {
  const el = document.getElementById(id)
  if (!el) { return '' }
  return el.classList.contains('hide') ? '' : el.textContent
}
const pickId = (id) => {
  const c = document.getElementById('routine-' + id.replace(/[^a-z0-9]+/gi, '-'))
  if (c) { c.click() }
}
const typeInto = (id, said) => {
  const el = document.getElementById(id)
  el.value = said
  el.dispatchEvent(new Event('input'))
}
const playDead = () => document.getElementById('routinePlayBtn').disabled ? 1 : 0
let pvBox = 0, pvSecretBox = 1, pvRows = 0
let pvDeadAtFirst = 0, pvDeadWhenCleared = 1, pvLiveWhenGiven = 0
let pvCardAfterRefusal = 0, pvRefusalSaid = '', pvRunDead = 0
let pvCardAfterOk = 0, pvSaid = '', pvRunLive = 0
let pvOtherBox = 1, pvOtherFields = 1
let pvBackBox = 'x', pvBackDead = 0

setTimeout(() => { pickId('notepad/focus/write-a-note') }, 1000)
setTimeout(() => {
  // A BOX FOR THE TEXT AND NONE FOR THE SECRET.
  pvRows = document.getElementById('routineArgsList').children.length
  pvBox = there('routineArg-message') ? 1 : 0
  pvSecretBox = there('routineArg-password') ? 1 : 0
  // AND PLAY IS DEAD UNTIL THE TEXT HAS BEEN GIVEN.
  pvDeadAtFirst = playDead()
  typeInto('routineArg-message', 'hello there')
}, 1400)
setTimeout(() => {
  pvLiveWhenGiven = playDead() ? 0 : 1
  // AND A BOX SOMEBODY CLEARED IS STILL A VALUE -- given, not non-empty. Typing and
  // clearing is how a person says "type nothing here", and refusing it would refuse
  // the one thing they meant.
  typeInto('routineArg-message', '')
}, 1700)
setTimeout(() => {
  pvDeadWhenCleared = playDead()
  typeInto('routineArg-message', 'hello there')
  press('routinePlayBtn')
}, 2000)
setTimeout(() => {
  // THE FIRST REVIEW REFUSES, AND THE CARD STAYS WITH THE REASON AND A DEAD BUTTON.
  pvCardAfterRefusal = shown('routineWillTypeRows') !== '' ||
    shown('routineWillTypeWhy') !== '' ? 1 : 0
  pvRefusalSaid = shown('routineWillTypeWhy')
  pvRunDead = document.getElementById('routineConfirmBtn').disabled ? 1 : 0
  press('routinePlayBtn')
}, 2400)
setTimeout(() => {
  pvCardAfterOk = shown('routineWillTypeRows') !== '' ? 1 : 0
  pvSaid = shown('routineWillTypeRows')
  pvRunLive = document.getElementById('routineConfirmBtn').disabled ? 0 : 1
  press('routineConfirmBtn')
}, 2800)
setTimeout(() => { pickId('notepad/focus/other') }, 3200)
setTimeout(() => {
  // THE FIELDS BELONG TO THE ROUTINE THEY ARE ABOUT.
  pvOtherFields = shown('routineArgsBox') === '' ? 0 : 1
  pvOtherBox = there('routineArg-message') ? 1 : 0
  // AND GOING BACK FINDS THE TEXT GONE, which is the half that discriminates: the
  // panel being hidden is drawArgs' own doing, and DROPPING what was typed is the
  // redraw hook's. Left behind, it would be a box that looks empty and counts as
  // given -- Play live, and a confirmation for text somebody typed for something else.
  pickId('notepad/focus/write-a-note')
}, 3600)
setTimeout(() => {
  // READ LATE ENOUGH FOR THE REBUILD. The fields are drawn from the backend's answer,
  // which arrives on a poll -- so this waits for the box to exist rather than reading
  // the moment the card changed and calling an absent box an empty one.
  pvBackBox = there('routineArg-message') ?
    document.getElementById('routineArg-message').value : 'gone'
  pvBackDead = playDead()
}, 4600)
setTimeout(() => {
  document.title = 'err=' + (window.bad ? 1 : 0) +
    ' rows=' + pvRows + ' box=' + pvBox + ' secretbox=' + pvSecretBox +
    ' deadfirst=' + pvDeadAtFirst + ' livegiven=' + pvLiveWhenGiven +
    ' deadcleared=' + pvDeadWhenCleared +
    ' cardrefused=' + pvCardAfterRefusal +
    ' refusedsaid=' + (pvRefusalSaid.includes('more than one line') ? 1 : 0) +
    ' rundead=' + pvRunDead +
    ' cardok=' + pvCardAfterOk +
    ' saidtext=' + (pvSaid.includes('hello there') ? 1 : 0) +
    ' saidsecret=' + (pvSaid.includes('credential store') ? 1 : 0) +
    ' runlive=' + pvRunLive +
    ' sent=' + ((window.pvSent || []).join(',') ===
      '4:{"message":"hello there"}:{}:"",' +
      '4:{"message":"hello there"}:{}:""' ? 1 : 0) +
    ' confirmed=' + ((window.pvConfirmed || []).join(',') ===
      '2:tok-run:4660:77' ? 1 : 0) +
    ' played=' + (window.pvPlayed ? 1 : 0) +
    ' otherfields=' + pvOtherFields + ' otherbox=' + pvOtherBox +
    ' backempty=' + (pvBackBox === '' ? 1 : 0) + ' backdead=' + pvBackDead
}, 5000)`

// aRenderedPlayValuesPage renders the window and reads the counters off its title.
func aRenderedPlayValuesPage(t *testing.T) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, playValuesHarness)
	if !strings.Contains(title, "err=") {
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

// THE PAGE DOES NOT THROW, which every counter below depends on and none of them can say.
func TestThePlayValuesPageDoesNotThrow(t *testing.T) {
	title, n := aRenderedPlayValuesPage(t)
	if n("err") != 0 {
		t.Fatalf("the page threw while this was being rendered: %q", title)
	}
}

// A BOX FOR THE TEXT, AND NONE FOR THE SECRET.
//
// The credential store answers for a secret and a field here would put it on a command line, so
// what the row owes somebody is where the value comes from — not somewhere to type it.
func TestThePlayRowOffersABoxForTextAndNotForASecret(t *testing.T) {
	title, n := aRenderedPlayValuesPage(t)
	if n("rows") != 2 {
		t.Errorf("the row lists %d fields: %q", n("rows"), title)
	}
	if n("box") != 1 {
		t.Errorf("there is no box for the text: %q", title)
	}
	if n("secretbox") != 0 {
		t.Errorf("there is a box for the secret: %q", title)
	}
}

// PLAY IS DEAD UNTIL THE TEXT HAS BEEN GIVEN, AND A CLEARED BOX IS STILL GIVEN.
//
// The branch only the page can take, and the distinction the whole slice turns on: a box nobody
// has touched is MISSING, and a box somebody typed into and cleared is an explicit "type nothing
// here". Refusing the second would refuse the one thing they meant.
func TestPlayWaitsForTheTextToBeGivenAndACountsAsGivenWhenCleared(t *testing.T) {
	title, n := aRenderedPlayValuesPage(t)
	if n("deadfirst") != 1 {
		t.Errorf("Play was live before the text was given: %q", title)
	}
	if n("livegiven") != 1 {
		t.Errorf("Play is dead after the text was given: %q", title)
	}
	if n("deadcleared") != 0 {
		t.Errorf("a box somebody cleared on purpose reads as missing: %q", title)
	}
}

// PRESSING PLAY SHOWS WHAT IT WOULD TYPE, AND PRESSES NOTHING UNTIL THAT IS CONFIRMED.
func TestPressingPlayShowsWhatItWouldTypeFirst(t *testing.T) {
	title, n := aRenderedPlayValuesPage(t)
	if n("cardok") != 1 || n("saidtext") != 1 {
		t.Errorf("the confirmation does not show the text: %q", title)
	}
	if n("saidsecret") != 1 {
		t.Errorf("the confirmation does not say where the secret comes from: %q", title)
	}
	if n("runlive") != 1 {
		t.Errorf("the button beside a confirmable run is dead: %q", title)
	}
	// AND THE PLAIN PLAY PATH WAS NEVER TAKEN for a routine that needs text.
	if n("played") != 0 {
		t.Errorf("the window pressed without confirming: %q", title)
	}
}

// AND A REFUSED REVIEW LEAVES THE CARD WITH THE REASON AND A DEAD BUTTON.
func TestARefusedPlayReviewIsShownAndCannotBeRun(t *testing.T) {
	title, n := aRenderedPlayValuesPage(t)
	if n("cardrefused") != 1 || n("refusedsaid") != 1 {
		t.Errorf("the refusal is not on the screen: %q", title)
	}
	if n("rundead") != 1 {
		t.Errorf("the run button is live beside a refusal: %q", title)
	}
}

// THE PAGE SENDS THE VALUES IT WAS GIVEN, AND CONFIRMS BY TOKEN AND WINDOW ALONE.
//
// The values go up once and what comes back down is a token: a page that handed the values back
// at confirm time would be a page that could hand back different ones. Two arguments to the
// confirm, and FOUR to the review — the values, what a request proposed, and the revision those
// proposals were read from.
//
// AND AN ORDINARY PRESS PROPOSES NOTHING AND NAMES NO REVISION, which is what the empty third and
// fourth say here. Nobody asked for this routine in words; the text in the box is the person's,
// and a window that sent a proposal it never made would have the contract label their own typing
// as a model's reading of a sentence they never said.
func TestThePageSendsValuesOnceAndConfirmsByToken(t *testing.T) {
	title, n := aRenderedPlayValuesPage(t)
	if n("sent") != 1 {
		t.Errorf("the review was not sent the values as typed: %q", title)
	}
	if n("confirmed") != 1 {
		t.Errorf("the confirm did not send the token and the window alone: %q", title)
	}
}

// AND THE FIELDS BELONG TO THE ROUTINE THEY ARE ABOUT.
//
// Choosing another routine takes the fields away AND the text with them: values belong to one run
// of one routine, and carrying them would offer somebody a confirmation for text they typed for
// something else.
func TestThePlayFieldsBelongToTheRoutineTheyAreAbout(t *testing.T) {
	title, n := aRenderedPlayValuesPage(t)
	if n("otherfields") != 0 {
		t.Errorf("a routine that needs nothing shows the fields: %q", title)
	}
	if n("otherbox") != 0 {
		t.Errorf("the other routine's card still holds the first one's box: %q", title)
	}
	// AND THE TEXT WENT WITH THEM. Coming back finds an empty box and a dead Play,
	// which is what "nothing is prefilled, here or ever" means in practice.
	if n("backempty") != 1 {
		t.Errorf("the box came back with text in it: %q", title)
	}
	if n("backdead") != 1 {
		t.Errorf("Play is live on a routine whose text was dropped, so an empty box "+
			"counts as given: %q", title)
	}
}
