package main

// THE PAGE OFFERS THE THREE PLACES A ROUTINE CAN BE AVAILABLE, AND MOVES IT ONLY ON PURPOSE.
//
// Rendered rather than grepped, for the reason `redraw_test.go` gives at length: a source guard
// passes as long as a field is NAMED somewhere in the file, so a page that printed every sentence
// unconditionally, or left the Move button live beside an unticked agreement, or submitted the
// routine's identity instead of the review's token, would keep every such guard green. The branch
// is the thing under test and only something that runs the page can take it.
//
// Nine claims, and seven of them are about what does NOT happen — which is the shape of this whole
// contract. The two positive ones are that all three choices appear with their reasons, and that a
// move that is agreed to is carried out and the library read again.
//
// The backend is stubbed because what is under test is the page: `rescopeacts_test.go` next door
// asks the same questions of the real `Acts` against a real library, and the contract itself —
// both locks, both revalidations, the exclusive create, the destination read back, the removal
// that takes the source first — is held in `internal/orchestrator`.
//
// See [[ADR-271-a-rescope-is-a-reviewed-move-between-identities]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// rescopeHarness is a window with two routines and a backend that answers like the real one.
//
// The first can be moved with nothing in the way. The second has a routine already at the place it
// would go, and the window's first attempt at moving it is refused the way a stale review is
// refused — because that refusal has to leave the review on the screen with the way forward beside
// it, which is a branch no backend test can take.
const rescopeHarness = `
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
    CancelReview: () => {},
    ReplaceWithRebuilt: () => Promise.resolve(''),
    ReviewRebuilt: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    ScopeChoices: (id, app) => {
      window.choicesAsked = (window.choicesAsked || []).concat(id + '|' + (app || ''))
      const taken = id === 'notepad/focus/settings'
      return Promise.resolve([
        {scope: 'app',
         where: 'presses only while notepad is in front, switching nothing',
         asked: 'asked for by name only while notepad is in front',
         available: true, taken: taken,
         taken_says: taken ? 'there is already a routine at ' +
           'notepad/context/settings. Moving "settings" there would replace it, ' +
           'which is a second thing to agree to' : ''},
        {scope: 'app-anywhere', where: 'brings notepad forward, then presses',
         asked: 'asked for by name from anywhere', current: true},
        {scope: 'everywhere',
         where: 'presses into whatever is in front, switching nothing',
         asked: 'asked for by name from anywhere', available: false,
         why: 'every press in this recording checks the window it was made in, ' +
              'and "everywhere" brings no window forward'},
      ])
    },
    ReviewRescope: (id, scope, app) => {
      window.reviewsAsked = (window.reviewsAsked || []).concat(id + '|' + scope)
      const taken = id === 'notepad/focus/settings'
      const slug = taken ? 'settings' : 'address'
      return Promise.resolve({id: id, from: id,
        to: 'notepad/context/' + slug,
        token: 'tok-' + slug, name: slug, scope: scope,
        where: 'presses only while notepad is in front, switching nothing',
        asked: 'asked for by name only while notepad is in front',
        was: 'use os.\nthe ' + slug + ' is a script.\n  bring notepad to the front.\n',
        now: '// Generation: 1.\nuse os.\nthe ' + slug + ' is a script.\n',
        changes: true, movable: true, taken: taken,
        taken_id: taken ? 'notepad/context/settings' : '',
        taken_says: taken ? 'there is already a routine at ' +
          'notepad/context/settings. Moving "settings" there would replace it, ' +
          'which is a second thing to agree to' : ''})
    },
    CancelMove: (token) => {
      window.cancelledMove = token
      window.cancelledMoves = (window.cancelledMoves || 0) + 1
    },
    MoveRoutine: (...args) => {
      // EXACTLY WHAT THE PAGE SENT, recorded whole. Two arguments and no more:
      // a third would be the page authoring a version, which is the thing the
      // token exists to stop.
      window.moveArgs = (window.moveArgs || []).concat(
        args.length + ':' + args[0] + ':' + args[1])
      window.movedTimes = (window.movedTimes || 0) + 1
      if (window.movedTimes === 1) {
        return Promise.resolve({why: 'the routine at notepad/context/settings ' +
          'has been saved again since you were shown it, so nothing was written'})
      }
      if (window.movedTimes === 2) {
        window.copiedDone = 1
        return Promise.resolve({copied: true, to: 'notepad/context/settings',
          says: 'Copied "settings" to notepad/context/settings; original ' +
            'retained: the original could not be removed.',
          left: 'What was beside it at notepad/focus/settings could not be ' +
            'removed: its demonstration.'})
      }
      window.movedDone = 1
      return Promise.resolve({moved: true, to: 'notepad/context/address',
        says: 'Moved "address" to notepad/context/address.'})
    },
    // THE LIBRARY CHANGES WHEN A MOVE CHANGES IT, because the page is asked to read
    // the affected identities again: the identity its panel was showing does not
    // exist any more after a move, and the routine is the one at the destination.
    PlaybackTarget: () => Promise.resolve({chosen: '4660:77',
      windows: [{id: '4660:77', label: 'a window',
        says: 'Play brings a window to the front and presses there.'}]}),
    // THE PHRASES PANEL'S READ, which the page makes for every routine it
    // draws. A stub without it is a page that throws mid-render -- and note
    // that no backtick may appear in this comment: the harness around it is a
    // Go raw string, and one closed ten of these files at once.
    RoutineAnswers: (id) => Promise.resolve({id: id, phrases: [],
      askable: true}),
    Routines: () => {
      const row = (id, slug) => ({id: id, name: slug, slug: slug, app: 'notepad',
        scope: id.indexOf('/focus/') > 0 ? 'app-anywhere' : 'app',
        contract: 'legacy', runnable: true, source: 'use os.',
        generation: 'unstated', rebuildable: true, offer: true, wrote: 'x'})
      const out = []
      out.push(window.movedDone ? row('notepad/context/address', 'address')
                                : row('notepad/focus/address', 'address'))
      out.push(row('notepad/focus/settings', 'settings'))
      if (window.copiedDone) { out.push(row('notepad/context/settings', 'settings')) }
      return Promise.resolve(out)
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
const moveShown = () =>
  !document.getElementById('routineMove').classList.contains('hide')
let rows = 0, pills = '', choiceText = '', nowBtn = 1, badBtn = 1, goodBtn = 0
let means = '', panes = 0, consentFirst = 'x', liveFirst = 0
let afterCancel = 1, movedAtCancel = 0
let takenSaid = '', consentSaid = '', deadBeforeTick = 0, liveAfterTick = 0
let staleCard = 0, staleNote = '', staleLive = 0
let copiedSaid = '', copiedBad = 0, copiedLeft = '', copiedGone = 'x'
let movedSaid = '', movedBad = 1, movedLeft = 'x', selected = ''

setTimeout(() => { pickId('notepad/focus/address') }, 1100)
setTimeout(() => { press('routineWhereBtn') }, 1300)
setTimeout(() => {
  // ALL THREE ANSWERS ON THE PAGE, each as the kind of answer it is.
  const box = document.getElementById('routineWhereChoices')
  rows = box.children.length
  pills = Array.from(box.querySelectorAll('.pill')).map(p => p.textContent).join('+')
  choiceText = box.textContent
  nowBtn = there('routineMoveTo-app-anywhere') ? 1 : 0
  badBtn = there('routineMoveTo-everywhere') ? 1 : 0
  goodBtn = there('routineMoveTo-app') ? 1 : 0
  press('routineMoveTo-app')
}, 1600)
setTimeout(() => {
  means = document.getElementById('routineMoveMeans').textContent
  const was = document.getElementById('routineMoveWas').textContent
  const now = document.getElementById('routineMoveNow').textContent
  panes = (moveShown() && was.includes('bring notepad to the front') &&
           now.includes('Generation: 1') && was !== now) ? 1 : 0
  // NOTHING IS IN THE WAY HERE, so there is no second thing to agree to and no
  // tick-box: an agreement attached to every move is one nobody reads.
  consentFirst = shown('routineMoveConsentRow')
  liveFirst = document.getElementById('routineMoveBtn').disabled ? 0 : 1
  press('routineMoveCancelBtn')
}, 1900)
setTimeout(() => {
  afterCancel = moveShown() ? 1 : 0
  movedAtCancel = window.movedTimes || 0
  pickId('notepad/focus/settings')
}, 2200)
setTimeout(() => { press('routineWhereBtn') }, 2400)
setTimeout(() => { press('routineMoveTo-app') }, 2600)
setTimeout(() => {
  takenSaid = shown('routineMoveTaken')
  consentSaid = shown('routineMoveConsentRow')
  deadBeforeTick = document.getElementById('routineMoveBtn').disabled ? 1 : 0
  const box = document.getElementById('routineMoveConsent')
  box.checked = true
  box.dispatchEvent(new Event('change'))
}, 2900)
setTimeout(() => {
  liveAfterTick = document.getElementById('routineMoveBtn').disabled ? 0 : 1
  press('routineMoveBtn')
}, 3200)
setTimeout(() => {
  // THE REFUSAL STAYS ON THE SCREEN WITH THE REVIEW, and the button is live again:
  // the way forward is to look at what is there now and choose once more, which
  // needs the panel still there to press the button beside.
  staleCard = moveShown() ? 1 : 0
  staleNote = document.getElementById('routineMoveNote').textContent
  staleLive = document.getElementById('routineMoveBtn').disabled ? 0 : 1
  press('routineMoveBtn')
}, 3600)
setTimeout(() => {
  copiedSaid = shown('routineMoveOutcome')
  copiedBad = document.getElementById('routineMoveOutcome').classList.contains('bad')
    ? 1 : 0
  copiedLeft = shown('routineMoveLeft')
  pickId('notepad/focus/address')
}, 4000)
setTimeout(() => {
  // AND A SENTENCE ABOUT ONE ROUTINE IS NOT LEFT UNDER ANOTHER'S NAME.
  copiedGone = shown('routineMoveOutcome')
  press('routineWhereBtn')
}, 4300)
setTimeout(() => { press('routineMoveTo-app') }, 4500)
setTimeout(() => { press('routineMoveBtn') }, 4800)
setTimeout(() => {
  movedSaid = shown('routineMoveOutcome')
  movedBad = document.getElementById('routineMoveOutcome').classList.contains('bad')
    ? 1 : 0
  movedLeft = shown('routineMoveLeft')
  selected = document.getElementById('routineIdentityWhere').textContent
}, 5100)
setTimeout(() => {
  document.title = 'rows=' + rows +
    ' pills=' + pills +
    ' nowpill=' + (pills.includes('Now') ? 1 : 0) +
    ' nobtnfornow=' + (nowBtn ? 0 : 1) +
    ' nobtnforbad=' + (badBtn ? 0 : 1) +
    ' btnforgood=' + goodBtn +
    ' whysaid=' + (choiceText.includes('brings no window forward') ? 1 : 0) +
    ' bothfacts=' + (choiceText.includes('only while notepad is in front, ' +
      'switching nothing') && choiceText.includes('asked for by name only ' +
      'while notepad is in front') ? 1 : 0) +
    ' undef=' + (choiceText.includes('undefined') ? 1 : 0) +
    ' meansfrom=' + (means.includes('notepad/focus/address') ? 1 : 0) +
    ' meansto=' + (means.includes('notepad/context/address') ? 1 : 0) +
    ' panes=' + panes +
    ' consentfirst=' + (consentFirst === '' ? 1 : 0) +
    ' livefirst=' + liveFirst +
    ' aftercancel=' + afterCancel +
    ' cancelled=' + (window.cancelledMoves || 0) +
    ' cancelledtoken=' + (window.cancelledMove === 'tok-address' ? 1 : 0) +
    ' movedatcancel=' + movedAtCancel +
    ' takensaid=' + (takenSaid.includes('second thing to agree to') ? 1 : 0) +
    ' consentshown=' + (consentSaid.includes('notepad/context/settings') ? 1 : 0) +
    ' deadbeforetick=' + deadBeforeTick +
    ' liveaftertick=' + liveAfterTick +
    ' stalecard=' + staleCard +
    ' stalenote=' + (staleNote.includes('saved again since') ? 1 : 0) +
    ' stalelive=' + staleLive +
    ' copiedsaid=' + (copiedSaid.includes('original retained') ? 1 : 0) +
    ' copiednotmoved=' + (copiedSaid.includes('Moved') ? 1 : 0) +
    ' copiedbad=' + copiedBad +
    ' copiedleft=' + (copiedLeft.includes('could not be removed: its ' +
      'demonstration') ? 1 : 0) +
    ' copiedgone=' + (copiedGone === '' ? 1 : 0) +
    ' movedsaid=' + (movedSaid.includes('Moved') ? 1 : 0) +
    ' movedbad=' + movedBad +
    ' movedleft=' + (movedLeft === '' ? 1 : 0) +
    ' selected=' + (selected.includes('only while notepad is in front') ? 1 : 0) +
    ' args=' + (window.moveArgs || []).join('~') +
    ' asked=' + (window.reviewsAsked || []).join('~')
}, 5500)
`

// aRenderedRescopePage renders the window against that backend and returns its counters.
func aRenderedRescopePage(t *testing.T) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, rescopeHarness)
	if !strings.Contains(title, "rows=") {
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

// ALL THREE CHOICES ARE ON THE PAGE, AND AN UNAVAILABLE ONE CARRIES ITS REASON.
//
// A chooser showing only what is possible answers the question somebody is actually asking — why
// can this not work from anywhere? — by removing the row that would have said. And the row that
// cannot be chosen carries no button, because a control that is live and answers a press with a
// refusal is a window telling somebody off for pressing what it offered them.
func TestThePageOffersEveryScopeAndSaysWhyOneIsUnavailable(t *testing.T) {
	title, n := aRenderedRescopePage(t)

	if n("rows") != 3 {
		t.Errorf("the chooser drew %d rows, want all three choices: %q", n("rows"),
			title)
	}
	for _, c := range []struct{ key, what string }{
		{"nowpill", "the scope the routine already has is not marked as the one it has"},
		{"nobtnfornow", "the scope it already has is offered as a move"},
		{"nobtnforbad", "a scope the routine's presses could not run under is offered"},
		{"btnforgood", "the one scope it could move to is not offered"},
		{"whysaid", "the unavailable choice does not say why it is unavailable"},
		{"bothfacts", "a choice does not say both where it works and where it can " +
			"be asked for"},
	} {
		if n(c.key) != 1 {
			t.Errorf("%s: %q", c.what, title)
		}
	}
	if n("undef") != 0 {
		t.Errorf("the chooser printed undefined: %q", title)
	}
}

// AND REVIEWING ONE SHOWS BOTH IDENTITIES AND BOTH SOURCES, WITH NOTHING AGREED TO YET.
//
// Both sources because a move RE-RENDERS from the demonstration: for a routine somebody edited by
// hand the left-hand pane is their own text, and approving the move is approving the loss of it.
// Both identities because the move is between two, and a card naming one is a card about a save.
func TestThePageShowsBothEndsOfAMoveBeforeAnythingIsWritten(t *testing.T) {
	title, n := aRenderedRescopePage(t)

	for _, c := range []struct{ key, what string }{
		{"meansfrom", "the review does not name where the routine is now"},
		{"meansto", "the review does not name where it would go"},
		{"panes", "the review does not show both sources"},
		{"consentfirst", "a move with nothing in the way asks for a second " +
			"agreement anyway"},
		{"livefirst", "a move with nothing in the way cannot be pressed"},
	} {
		if n(c.key) != 1 {
			t.Errorf("%s: %q", c.what, title)
		}
	}
	// AND CANCEL WRITES NOTHING, which is the absence of an operation rather than an
	// undo: the preparation wrote nothing, and letting go of it is what stops a second
	// press of a dismissed button moving with it.
	if n("aftercancel") != 0 {
		t.Errorf("cancelling left the review on the screen: %q", title)
	}
	if n("cancelled") != 1 || n("cancelledtoken") != 1 {
		t.Errorf("cancelling did not let go of the review it was showing, by its own "+
			"token: %q", title)
	}
	if n("movedatcancel") != 0 {
		t.Errorf("cancelling moved something: %q", title)
	}
}

// AN OCCUPIED DESTINATION NEEDS ITS OWN AGREEMENT, AND THE BUTTON IS DEAD UNTIL IT HAS ONE.
//
// Agreeing to move this routine is not agreeing to lose that one, and the page is where that
// second question is asked. Dead rather than live-and-then-refused, because the refusal would
// arrive after somebody had already decided.
func TestThePageWillNotMoveOntoAnOccupiedIdentityWithoutAgreement(t *testing.T) {
	title, n := aRenderedRescopePage(t)

	if n("takensaid") != 1 {
		t.Errorf("the review does not say what is in the way: %q", title)
	}
	if n("consentshown") != 1 {
		t.Errorf("the agreement does not name the routine it would replace: %q", title)
	}
	if n("deadbeforetick") != 1 {
		t.Errorf("the move could be pressed with nothing agreed to: %q", title)
	}
	if n("liveaftertick") != 1 {
		t.Errorf("the move cannot be pressed after agreeing to it: %q", title)
	}
}

// THE PAGE SUBMITS THE BACKEND'S TOKEN AND ONE ANSWER, AND NEVER A VERSION OF ITS OWN.
//
// The whole of what makes this safe: the four digests a move is checked against — the original's
// source and demonstration, and whatever was at the destination — are the backend's, and a page
// that handed any of them back would be handing back something that could be anything. What it
// sends is the token of the review it displayed and whether the person agreed to what they were
// shown.
func TestThePageMovesWithTheReviewsOwnTokenAndNothingElse(t *testing.T) {
	title, _ := aRenderedRescopePage(t)

	got := submittedMoves(t, title)
	want := "2:tok-settings:true~2:tok-settings:true~2:tok-address:false"
	if got != want {
		t.Errorf("the page submitted %q, want %q — the token of the review it was "+
			"showing, one agreement, and no third value: %q", got, want, title)
	}
	// AND THE REVIEW IT ASKED FOR WAS THE CHOICE THAT WAS PRESSED, by routine and by
	// scope. A page that asked for whichever button came first would offer one move and
	// prepare another.
	asked := submitted(t, title, "asked")
	if asked != "notepad/focus/address|app~notepad/focus/settings|app~"+
		"notepad/focus/address|app" {
		t.Errorf("the page asked for %q: %q", asked, title)
	}
}

// AND A REFUSAL LEAVES THE REVIEW ON THE SCREEN WITH THE WAY FORWARD BESIDE IT.
//
// A stale review is the commonest refusal this flow has, and the answer to it is to look at what
// is there now and choose once more — which needs the panel still there to press the button
// beside. A window that cleared the card on a refusal would leave somebody with a sentence and no
// control.
func TestARefusedMoveKeepsTheReviewAndSaysWhat(t *testing.T) {
	title, n := aRenderedRescopePage(t)

	if n("stalecard") != 1 {
		t.Errorf("a refused move cleared the review it was refused from: %q", title)
	}
	if n("stalenote") != 1 {
		t.Errorf("a refused move does not say what was wrong: %q", title)
	}
	if n("stalelive") != 1 {
		t.Errorf("a refused move leaves its own button dead, so there is nothing to "+
			"press after fixing it: %q", title)
	}
}

// MOVED AND COPIED ARE RENDERED AS THE DIFFERENT THINGS THEY ARE, AND RESIDUE AS ITS OWN.
//
// A destination written with the original gone is a move. One written with the original still
// there is TWO routines, and somebody told "Moved" would go looking for one of them — so it is a
// problem card and not a note. Residue is a third sentence either way: the move happened AND
// something is still lying at the old place.
func TestThePageRendersMovedCopiedAndResidueDistinctly(t *testing.T) {
	title, n := aRenderedRescopePage(t)

	if n("copiedsaid") != 1 {
		t.Errorf("a copy with the original retained does not say so: %q", title)
	}
	if n("copiednotmoved") != 0 {
		t.Errorf("a copy with the original retained is called a move: %q", title)
	}
	if n("copiedbad") != 1 {
		t.Errorf("two routines where one was expected is not rendered as something "+
			"to look at: %q", title)
	}
	if n("copiedleft") != 1 {
		t.Errorf("what could not be swept from the old place is not said: %q", title)
	}
	// AND THE ORDINARY MOVE IS NEITHER OF THOSE.
	if n("movedsaid") != 1 {
		t.Errorf("an ordinary move does not say where the routine went: %q", title)
	}
	if n("movedbad") != 0 {
		t.Errorf("an ordinary move is rendered as a problem: %q", title)
	}
	if n("movedleft") != 1 {
		t.Errorf("an ordinary move reports litter it did not leave: %q", title)
	}
}

// AND THE LIBRARY IS READ AGAIN AFTERWARDS, WITH THE ROUTINE FOUND WHERE IT NOW IS.
//
// The identity the panel was showing does not exist after a move. A window that waited for its
// next poll would spend those seconds offering Play on a routine that is not there, and one that
// did not re-select would drop somebody back to a library with no selection and no account of what
// just happened.
func TestThePageReadsTheMovedIdentityBackAndSelectsIt(t *testing.T) {
	title, n := aRenderedRescopePage(t)

	if n("selected") != 1 {
		t.Errorf("after a move the panel does not show the routine at its new "+
			"scope: %q", title)
	}
	// AND THE SENTENCE BELONGS TO ONE ROUTINE. Choosing another card puts it away
	// rather than leaving an account of one move under another routine's name.
	if n("copiedgone") != 1 {
		t.Errorf("the account of one move is still on screen under another "+
			"routine: %q", title)
	}
}

// submittedMoves is what the page handed `MoveRoutine`, call by call.
func submittedMoves(t *testing.T, title string) string {
	t.Helper()
	return submitted(t, title, "args")
}

// submitted is one tilde-joined list out of the counters.
func submitted(t *testing.T, title, key string) string {
	t.Helper()
	m := regexp.MustCompile(`\b` + key + `=(\S*)`).FindStringSubmatch(title)
	if m == nil {
		t.Fatalf("no %s in %q", key, title)
	}
	return m[1]
}
