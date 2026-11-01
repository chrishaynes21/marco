package main

// THE PAGE OFFERS TWO OPERATIONS OVER THE STEPS IT WAS SHOWN, AND SAVES ONLY ON PURPOSE.
//
// Rendered rather than grepped, for the reason `redraw_test.go` gives at length: a source guard
// passes as long as a field is NAMED somewhere in the file, so a page that offered a duration
// control on a press, left the Save button live beside a refused review, sent the routine's
// identity instead of the review's token, or applied row numbers from a list it had reloaded since
// would keep every such guard green. The branch is the thing under test and only something that
// runs the page can take it.
//
// The claim this file exists for, above the others: THE POSITIONS GO BACK WITH THE BASIS THEY WERE
// READ OFF, and a refusal about that basis leaves the list reloaded with nothing asked against it.
// No backend test can take that branch, because it is the page that holds the list.
//
// The backend is stubbed because what is under test is the page: `stepeditacts_test.go` next door
// asks the same questions of the real `Acts` against a real library, and the contract itself is
// held in `internal/orchestrator`.
//
// See [[ADR-273-a-routine-writes-down-what-it-does]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// stepEditHarness is a window with one routine and a backend that answers like the real one.
//
// It refuses twice on purpose — once at Review with a moved basis, once at Save with a routine
// saved since — because both refusals have to leave the panel on the screen with the way forward
// beside it, and neither is reachable from a backend test.
// stepEditBackend is the stub and the little helpers, WITHOUT a driver.
//
// Split out because `stepEditHarness` below is this file's stub AND this file's sequence of
// presses, and a second test that appended its own driver to it ran both at once: two reviews, a
// re-listed basis, and counters that made no sense. Found by writing that second test. A harness
// that carries a script is not a harness.
const stepEditBackend = `
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
    // THE RAIL'S BUILD STAMP, which is its own read now rather than two
    // fields of the Scenes view's poll. See buildstatus.go.
    BuildStatus: () => Promise.resolve({build: 'abc1234'}),
    Help: () => Promise.resolve({sections: [{key: 'editing',
      title: 'Editing what a routine does', lines: ['nothing is written yet']}]}),
    Recording: () => Promise.resolve({active: false}),
    CancelReview: () => {},
    ReplaceWithRebuilt: () => Promise.resolve(''),
    ReviewRebuilt: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    ScopeChoices: () => Promise.resolve([]),
    CancelMove: () => {},
    PlaybackTarget: () => Promise.resolve({chosen: '', windows: []}),
    // THE LIST, AND A DIFFERENT BASIS THE SECOND TIME. The page must send back the
    // one that came with the list it is showing, so a second listing has to be
    // distinguishable from the first.
    StepEditing: (id) => {
      window.listed = (window.listed || 0) + 1
      return Promise.resolve({id: id, name: 'address', editable: true,
        basis: window.listed === 1 ? 'basis-one' : 'basis-two',
        typed_runs: [{from: 1, to: 3, label: 'steps 2 to 4 - types \"stone\"'}],
        typed_says: 'If a run of steps is text you typed, say so and it becomes ' +
          'one step: the keys in it stop being held and the waits between ' +
          'them go. Waits outside the run are left alone.',
        rows: [
          {at: 0, says: 'bring notepad to the front'},
          {at: 1, says: 'wait 2500ms', wait: true, ms: 2500},
          {at: 2, says: 'hold shift', pairs: true, pairs_at: 4,
           pair_says: 'removing this removes step 5 as well, because they are ' +
             'the two halves of holding "shift"'},
          {at: 3, says: 'left-click at 640, 480'},
          {at: 4, says: 'let go of shift', pairs: true, pairs_at: 2,
           pair_says: 'removing this removes step 3 as well, because they are ' +
             'the two halves of holding "shift"'},
        ]})
    },
    ReviewStepEdit: (...args) => {
      // EXACTLY WHAT THE PAGE SENT, recorded whole: three arguments and no more,
      // and the basis among them.
      window.reviewArgs = (window.reviewArgs || []).concat(
        args.length + ':' + args[1] + ':' + JSON.stringify(args[2]))
      window.reviewed = (window.reviewed || 0) + 1
      if (window.reviewed === 1) {
        return Promise.resolve({id: args[0], why: '"address" has been written ' +
          'since its steps were listed, so the changes you asked for refer to ' +
          'steps it no longer has — look at it again and ask once more'})
      }
      if (window.reviewed === 2) {
        // A REVIEW WITH NOTHING TO SAVE. Worth showing -- the reason is the
        // point -- and the button beside it has to be dead, which is a branch
        // no savable review can take.
        return Promise.resolve({id: args[0], name: 'address', token: 'tok-no',
          doing: ['step 2 is removed'],
          refused: 'removing that would leave the routine with nothing to do. ' +
            'Deleting the routine is how to get rid of it',
          was: 'use os.\ndo OS\'s Sleep with 2500.\n',
          changes: false, savable: false, rows: []})
      }
      return Promise.resolve({id: args[0], name: 'address', token: 'tok-edit',
        doing: ['the wait at step 2 becomes 800ms (it was 2500ms)'],
        was: 'use os.\ndo OS\'s Sleep with 2500.\n',
        now: '// Generation: 1.\nuse os.\ndo OS\'s Sleep with 800.\n',
        changes: true, savable: true,
        rows: [{at: 0, says: 'bring notepad to the front'},
               {at: 1, says: 'wait 800ms', wait: true, ms: 800}]})
    },
    CancelStepEdit: (token) => {
      window.cancelledEdit = token
      window.cancelledEdits = (window.cancelledEdits || 0) + 1
    },
    SaveStepEdit: (...args) => {
      window.saveArgs = (window.saveArgs || []).concat(
        args.length + ':' + args[0])
      window.saved = (window.saved || 0) + 1
      if (window.saved === 1) {
        return Promise.resolve({why: '"address" has been saved again since the ' +
          'edit you reviewed was prepared, so it is left as it is'})
      }
      window.savedDone = 1
      return Promise.resolve({saved: true, partial: true,
        says: '"address": the routine was saved with the edit and its steps were ' +
          'not written down beside it, so it cannot be edited again until it has ' +
          'been rebuilt from its demonstration'})
    },
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
    Routines: () => {
      const row = (id, slug) => ({id: id, name: slug, slug: slug, app: 'notepad',
        scope: 'app-anywhere', contract: 'legacy', runnable: true,
        source: 'use os.', generation: 'unstated', rebuildable: true,
        offer: true, wrote: 'x'})
      return Promise.resolve([row('notepad/focus/address', 'address'),
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
const listShown = () =>
  !document.getElementById('routineSteps').classList.contains('hide')
const editShown = () =>
  !document.getElementById('routineStepEdit').classList.contains('hide')
`

// stepEditHarness is that stub with THIS file's driver on it.
const stepEditHarness = stepEditBackend + `
let rows = 0, rowText = '', waitBox = 0, pressBox = 1, cutEvery = 0
let addBtn = 1, deadBtn = 1, askedText = '', liveBtn = 0
let staleWhy = '', staleList = 0, staleAsked = 'x', staleRows = 0
let doing = '', panes = 0, resultRows = 0, saveLive = 0
let afterCancel = 1, savedAtCancel = 0
let staleNote = '', staleCard = 0, staleSaveLive = 0
let refusedSaid = '', refusedDead = 0
let partialSaid = '', partialBad = 0, partialGone = 'x'

setTimeout(() => { pickId('notepad/focus/address') }, 1000)
setTimeout(() => { press('routineStepsBtn') }, 1200)
setTimeout(() => {
  // EVERY STEP, WITH THE CONTROL ITS KIND ALLOWS AND NO OTHER.
  const box = document.getElementById('routineStepRows')
  rows = box.children.length
  rowText = box.textContent
  waitBox = there('routineStepMs-1') ? 1 : 0
  pressBox = there('routineStepMs-3') ? 1 : 0
  cutEvery = [0, 1, 2, 3, 4].every(i => there('routineStepCut-' + i)) ? 1 : 0
  // AND NOTHING THIS PASS DOES NOT OFFER. No adding, no reordering, no editing a
  // coordinate or a key: a control that is not there cannot be pressed, and a
  // backend that refuses one is not the same as a page that never offered it.
  addBtn = (there('routineStepAddBtn') || there('routineStepUp-1') ||
            there('routineStepDown-1')) ? 1 : 0
  deadBtn = document.getElementById('routineStepReviewBtn').disabled ? 1 : 0
  document.getElementById('routineStepMs-1').value = '800'
  press('routineStepSet-1')
}, 1500)
setTimeout(() => {
  askedText = shown('routineStepAsked')
  liveBtn = document.getElementById('routineStepReviewBtn').disabled ? 0 : 1
  press('routineStepReviewBtn')
}, 1800)
setTimeout(() => {
  // THE BASIS MOVED, SO THE CHANGES ARE REFUSED AND THE LIST IS READ AGAIN.
  //
  // The sentence stays where the button is, the panel is still listing, and nothing
  // is asked against the new list: row numbers from a list that has been replaced
  // are somebody's intention applied to a different step.
  staleWhy = document.getElementById('routineStepsWhy').textContent
  staleList = listShown() ? 1 : 0
  staleAsked = shown('routineStepAsked')
  staleRows = document.getElementById('routineStepRows').children.length
  document.getElementById('routineStepMs-1').value = '800'
  press('routineStepSet-1')
}, 2200)
setTimeout(() => { press('routineStepReviewBtn') }, 2500)
setTimeout(() => {
  // A REFUSED REVIEW IS SHOWN, AND THE BUTTON BESIDE IT IS DEAD. The reason is the
  // point -- an edit that would leave the routine with nothing to do -- and a live
  // button next to it would be this window offering a press it refuses.
  refusedSaid = shown('routineStepEditRefused')
  refusedDead = document.getElementById('routineStepSaveBtn').disabled ? 1 : 0
  press('routineStepCancelBtn')
}, 2800)
setTimeout(() => {
  afterCancel = editShown() ? 1 : 0
  savedAtCancel = window.saved || 0
  press('routineStepReviewBtn')
}, 3100)
setTimeout(() => {
  doing = document.getElementById('routineStepEditDoing').textContent
  const was = document.getElementById('routineStepEditWas').textContent
  const now = document.getElementById('routineStepEditNow').textContent
  panes = (editShown() && was.includes('Sleep with 2500') &&
           now.includes('Sleep with 800') && was !== now) ? 1 : 0
  resultRows = document.getElementById('routineStepEditRows').children.length
  saveLive = document.getElementById('routineStepSaveBtn').disabled ? 0 : 1
  press('routineStepSaveBtn')
}, 3400)
setTimeout(() => {
  // THE REFUSAL STAYS ON THE SCREEN WITH THE REVIEW, and the button is live again:
  // the way forward is to look at what is there now, which needs the panel still
  // there to press the button beside.
  staleNote = document.getElementById('routineStepEditNote').textContent
  staleCard = editShown() ? 1 : 0
  staleSaveLive = document.getElementById('routineStepSaveBtn').disabled ? 0 : 1
  press('routineStepSaveBtn')
}, 3700)
setTimeout(() => {
  // A SOURCE THAT LANDED WITHOUT ITS STEP LIST IS A PROBLEM CARD. The routine runs
  // as edited and the next edit of it refuses, which somebody has to be told.
  partialSaid = shown('routineStepOutcome')
  partialBad = document.getElementById('routineStepOutcome')
    .classList.contains('bad') ? 1 : 0
  pickId('notepad/focus/other')
}, 4100)
setTimeout(() => {
  // AND A SENTENCE ABOUT ONE ROUTINE IS NOT LEFT UNDER ANOTHER'S NAME.
  partialGone = shown('routineStepOutcome')
}, 4400)
setTimeout(() => {
  document.title = 'rows=' + rows +
    ' waitbox=' + waitBox +
    ' nopressbox=' + (pressBox ? 0 : 1) +
    ' cutevery=' + cutEvery +
    ' noadd=' + (addBtn ? 0 : 1) +
    ' pairsaid=' + (rowText.includes('two halves of holding') ? 1 : 0) +
    ' pairboth=' + ((rowText.match(/two halves of holding/g) || []).length) +
    ' undef=' + (rowText.includes('undefined') ? 1 : 0) +
    ' deadbtn=' + deadBtn +
    ' asked=' + (askedText.includes('step 2 waits 800ms') ? 1 : 0) +
    ' livebtn=' + liveBtn +
    ' stalewhy=' + (staleWhy.includes('no longer has') ? 1 : 0) +
    ' stalelist=' + staleList +
    ' stalecleared=' + (staleAsked === '' ? 1 : 0) +
    ' stalerows=' + staleRows +
    ' relisted=' + (window.listed || 0) +
    ' doing=' + (doing.includes('becomes 800ms') ? 1 : 0) +
    ' panes=' + panes +
    ' resultrows=' + resultRows +
    ' savelive=' + saveLive +
    ' refusedsaid=' + (refusedSaid.includes('nothing to do') ? 1 : 0) +
    ' refuseddead=' + refusedDead +
    ' aftercancel=' + afterCancel +
    ' cancelled=' + (window.cancelledEdits || 0) +
    ' cancelledtoken=' + (window.cancelledEdit === 'tok-no' ? 1 : 0) +
    ' savedatcancel=' + savedAtCancel +
    ' stalenote=' + (staleNote.includes('saved again since') ? 1 : 0) +
    ' stalecard=' + staleCard +
    ' stalesavelive=' + staleSaveLive +
    ' partialsaid=' + (partialSaid.includes('not written down beside it') ? 1 : 0) +
    ' partialbad=' + partialBad +
    ' partialgone=' + (partialGone === '' ? 1 : 0) +
    ' reviewargs=' + (window.reviewArgs || []).join('~') +
    ' saveargs=' + (window.saveArgs || []).join('~')
}, 4800)
`

// aRenderedStepEditPage renders the window against that backend and returns its counters.
func aRenderedStepEditPage(t *testing.T) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, stepEditHarness)
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

// EVERY STEP IS LISTED, EACH WITH THE CONTROL ITS KIND ALLOWS AND NO OTHER.
//
// A duration belongs to a wait and nowhere else; removal belongs to every row. And the two
// operations this pass offers are the only two on the page: a control that is not there cannot be
// pressed, which is a different and stronger thing than a backend that refuses it.
func TestTheStepPanelOffersTwoOperationsAndNoOthers(t *testing.T) {
	title, n := aRenderedStepEditPage(t)
	if n("rows") != 5 {
		t.Errorf("the panel shows %d of the 5 steps it was given: %q", n("rows"), title)
	}
	for _, c := range []struct{ key, why string }{
		{"waitbox", "a wait offers no duration to change"},
		{"nopressbox", "a press offers a duration, which it has none of"},
		{"cutevery", "some row cannot be removed"},
		{"noadd", "the page offers adding or reordering, which this pass does not"},
	} {
		if n(c.key) != 1 {
			t.Errorf("%s: %q", c.why, title)
		}
	}
	if n("undef") != 0 {
		t.Errorf("a row renders `undefined`: %q", title)
	}
}

// AND A REMOVAL THAT TAKES TWO STEPS SAYS SO ON BOTH ROWS, BEFORE ANYTHING IS PRESSED.
func TestTheStepPanelSaysOnTheRowWhenARemovalTakesTwo(t *testing.T) {
	title, n := aRenderedStepEditPage(t)
	if n("pairsaid") != 1 {
		t.Errorf("no row says a removal takes its other half: %q", title)
	}
	if n("pairboth") != 2 {
		t.Errorf("%d rows say it; a held key is two steps and both are owed the "+
			"sentence: %q", n("pairboth"), title)
	}
}

// NOTHING CAN BE REVIEWED UNTIL SOMETHING IS ASKED, AND WHAT WAS ASKED IS READABLE FIRST.
func TestTheStepPanelWillNotReviewNothing(t *testing.T) {
	title, n := aRenderedStepEditPage(t)
	if n("deadbtn") != 1 {
		t.Errorf("the Review button is live with nothing asked: %q", title)
	}
	if n("asked") != 1 || n("livebtn") != 1 {
		t.Errorf("asking a change neither showed it nor enabled the review: %q", title)
	}
}

// THE POSITIONS GO BACK WITH THE BASIS THEY WERE READ OFF.
//
// Three arguments and no more: the routine, the basis the backend minted with the list, and the
// changes. A page that sent only the changes would be asking for row numbers to be applied to
// whatever the routine is by then.
func TestThePageSendsTheBasisItWasGivenWithTheChanges(t *testing.T) {
	title, _ := aRenderedStepEditPage(t)
	m := regexp.MustCompile(`reviewargs=(\S*)`).FindStringSubmatch(title)
	if m == nil {
		t.Fatalf("the page reported no review arguments: %q", title)
	}
	sent := strings.Split(m[1], "~")
	if len(sent) != 3 {
		t.Fatalf("the page asked for %d reviews, want the three this harness drives: %q",
			len(sent), m[1])
	}
	// THE FIRST AGAINST THE LIST IT HAD, THE OTHER TWO AGAINST THE ONE IT RELOADED. The
	// second is what the first refusal sent it back for; the third is the review that is
	// then saved.
	for i, want := range []string{"3:basis-one:", "3:basis-two:", "3:basis-two:"} {
		if !strings.HasPrefix(sent[i], want) {
			t.Errorf("review %d was sent as %q, want %q and the changes after it",
				i+1, sent[i], want)
		}
	}
	// AND THE CHANGES ARE THE OPERATIONS, in the vocabulary the backend knows.
	if !strings.Contains(sent[2], `"op":"wait"`) || !strings.Contains(sent[2], `"to":800`) {
		t.Errorf("the changes were not sent as operations: %q", sent[2])
	}
}

// AND A REFUSAL ABOUT THAT BASIS LEAVES THE LIST READ AGAIN WITH NOTHING ASKED AGAINST IT.
//
// The sentence stays where the button is, the panel keeps listing, and the pending changes go:
// they were counted against a list that has been replaced, and keeping them would leave somebody
// one press away from applying them to steps they never saw.
func TestAMovedBasisLeavesTheListReloadedAndNothingAsked(t *testing.T) {
	title, n := aRenderedStepEditPage(t)
	for _, c := range []struct{ key, why string }{
		{"stalewhy", "the refusal is not shown where the button is"},
		{"stalelist", "the panel stopped listing the steps"},
		{"stalecleared", "the changes counted against the old list are still asked"},
	} {
		if n(c.key) != 1 {
			t.Errorf("%s: %q", c.why, title)
		}
	}
	if n("stalerows") != 5 {
		t.Errorf("the reloaded list has %d rows: %q", n("stalerows"), title)
	}
	if n("relisted") < 2 {
		t.Errorf("the page asked for the steps %d times, so it never read them "+
			"again: %q", n("relisted"), title)
	}
}

// THE REVIEW SHOWS WHAT IT WOULD DO, BOTH SOURCES IN FULL, AND THE STEPS IT WOULD LEAVE.
func TestTheStepReviewShowsBothSourcesAndTheResult(t *testing.T) {
	title, n := aRenderedStepEditPage(t)
	for _, c := range []struct{ key, why string }{
		{"doing", "the review does not say what it would do"},
		{"panes", "the two sources are not both there in full"},
		{"savelive", "the Save button is not live for a savable review"},
	} {
		if n(c.key) != 1 {
			t.Errorf("%s: %q", c.why, title)
		}
	}
	if n("resultrows") != 2 {
		t.Errorf("the resulting steps show %d rows: %q", n("resultrows"), title)
	}
}

// AND A REFUSED REVIEW IS SHOWN WITH THE BUTTON BESIDE IT DEAD.
//
// The reason is the point: an edit that would leave the routine with nothing to do is worth
// reading, and a live Save button next to it would be this window offering a press it refuses.
// Only an unsavable review can take that branch, which is why the harness produces one.
func TestARefusedStepEditIsShownAndCannotBeSaved(t *testing.T) {
	title, n := aRenderedStepEditPage(t)
	if n("refusedsaid") != 1 {
		t.Errorf("the refusal is not on the screen: %q", title)
	}
	if n("refuseddead") != 1 {
		t.Errorf("the Save button is live beside a review that cannot be saved: %q",
			title)
	}
}

// CANCEL WRITES NOTHING, AND LETS GO OF THE REVIEW BY ITS OWN TOKEN.
func TestCancellingAStepEditOnThePageWritesNothing(t *testing.T) {
	title, n := aRenderedStepEditPage(t)
	if n("aftercancel") != 0 {
		t.Errorf("the review is still on the screen after Cancel: %q", title)
	}
	if n("cancelled") < 1 || n("cancelledtoken") != 1 {
		t.Errorf("Cancel did not let go of the review it was showing: %q", title)
	}
	if n("savedatcancel") != 0 {
		t.Errorf("Cancel saved something: %q", title)
	}
}

// SAVING SENDS THE REVIEW'S TOKEN AND NOTHING ELSE, AND A REFUSAL KEEPS THE PANEL.
func TestSavingAStepEditSendsTheTokenAloneAndKeepsARefusalOnScreen(t *testing.T) {
	title, n := aRenderedStepEditPage(t)
	m := regexp.MustCompile(`saveargs=(\S*)`).FindStringSubmatch(title)
	if m == nil {
		t.Fatalf("the page reported no save arguments: %q", title)
	}
	for _, sent := range strings.Split(m[1], "~") {
		if sent != "1:tok-edit" {
			t.Errorf("a save was sent as %q, want the token alone", sent)
		}
	}
	for _, c := range []struct{ key, why string }{
		{"stalenote", "the refusal is not shown beside the button"},
		{"stalecard", "the review was taken off the screen by a refusal"},
		{"stalesavelive", "the button is dead after a refusal that says to look again"},
	} {
		if n(c.key) != 1 {
			t.Errorf("%s: %q", c.why, title)
		}
	}
}

// AND A SOURCE THAT LANDED WITHOUT ITS STEP LIST IS RENDERED AS THE PROBLEM IT IS.
//
// Not a success, because the next edit of that routine will refuse until it has been rebuilt; not
// a refusal, because the routine on disk is the edited one and it runs.
func TestAPartialStepSaveIsRenderedAsAProblem(t *testing.T) {
	title, n := aRenderedStepEditPage(t)
	if n("partialsaid") != 1 {
		t.Errorf("the outcome does not say what did not land: %q", title)
	}
	if n("partialbad") != 1 {
		t.Errorf("it is rendered as an ordinary success: %q", title)
	}
	if n("partialgone") != 1 {
		t.Errorf("the sentence stayed under another routine's name: %q", title)
	}
}
