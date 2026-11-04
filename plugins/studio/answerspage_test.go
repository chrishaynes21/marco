package main

// THE PANEL THAT SHOWS WHAT A ROUTINE ANSWERS TO, RENDERED.
//
// Three of the claims here cannot be taken from a backend test, because the branch belongs to the
// page: a refused phrase has to stay in the box the person typed it into, a removal has to send
// the PHRASE and not the position it was drawn at, and the panel has to belong to the routine it
// is about. The last one is the defect this window has shipped twice in other panels — a card
// showing one routine's state under another's name.
//
// The backend is stubbed, as next door: `answersacts_test.go` asks the same questions of the real
// `Acts` against a real library, and the rules themselves are `internal/orchestrator`'s.
//
// See [[Command-aliases-proposal-2026-10-30]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// answersHarness is a window with two routines and a backend that refuses the first phrase.
//
// It refuses once on purpose: a collision is the one answer this panel cannot produce itself, and
// what the page does with it — keep the words, say why — is only reachable by running the page.
const answersHarness = `
// EVERY ERROR THE PAGE THROWS, KEPT AND PUT IN THE TITLE.
//
// Two of them were in this harness: a missing Ready binding, which the page calls at
// startup, and a window global whose name the page already uses. Both produced a window
// in which nothing was drawn and every counter read zero -- which is indistinguishable
// from a panel that does not work, and is what cost three runs to find. The counter
// below makes it one assertion; this title write makes the MESSAGE readable when the
// error lands after the counters are written.
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
    // THE WIDGET ACTION (S7), polled from start-up: the page draws whatever it says.
    WidgetStatus: () => Promise.resolve({state: 'ready', says: '', problems: []}),
    Theme: () => Promise.resolve({asked: '', theme: 'default', light: false, accentFrom: 'name', tokens: {}}),
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
    // THE PAGE ASKS THIS AT STARTUP, so a harness without it throws before anything
    // is drawn. Found by the error counter below, which is the whole reason it is here.
    Ready: () => Promise.resolve({build: 'abc1234', problems: [], limits: 'nothing'}),
    CancelReview: () => {},
    ReplaceWithRebuilt: () => Promise.resolve(''),
    ReviewRebuilt: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    ScopeChoices: () => Promise.resolve([]),
    CancelMove: () => {},
    PlaybackTarget: () => Promise.resolve({chosen: '', windows: []}),
    StepEditing: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    // THE STEP PANEL'S BINDINGS, PRESENT AND UNUSED. A binding the page calls at
    // startup and the window does not export is a page that paints nothing, which is
    // what these harnesses exist to catch -- and it is not what this file is about.
    ReviewStepEdit: () => Promise.resolve({why: 'not what this asks about'}),
    CancelStepEdit: () => {},
    SaveStepEdit: () => Promise.resolve({why: 'not what this asks about'}),
    // WHAT EACH ROUTINE ANSWERS TO, so that choosing the other card cannot show the
    // first one's phrases.
    RoutineAnswers: (id) => {
      window.ansAsked = (window.ansAsked || []).concat(id)
      if (id === 'notepad/focus/address') {
        return Promise.resolve({id: id, name: 'address', askable: true,
          phrases: ['write my address', 'fill in the address']})
      }
      // SLOWLY, on purpose. The guard that keeps one routine's phrases off
      // another's card only matters between choosing and the answer arriving, so
      // an answer that is already there cannot test it.
      return new Promise((res) => setTimeout(() => res(
        {id: id, name: 'other', askable: true, phrases: []}), 600))
    },
    AddRoutineAnswer: (...args) => {
      // EXACTLY WHAT THE PAGE SENT: two arguments, and the phrase as typed.
      window.ansAddArgs = (window.ansAddArgs || []).concat(
        args.length + ':' + args[1])
      window.ansAdded = (window.ansAdded || 0) + 1
      if (window.ansAdded === 1) {
        return Promise.resolve({id: args[0], name: 'address', askable: true,
          phrases: ['write my address', 'fill in the address'], failed: true,
          said: '"settings" is already how "settings" at global/settings is ' +
            'reached, and a phrase two routines answer to selects neither of ' +
            'them — it asks which. Choose another phrase, or rename one of them'})
      }
      return Promise.resolve({id: args[0], name: 'address', askable: true,
        phrases: ['write my address', 'fill in the address', args[1].trim()],
        said: '"address" now also answers to "' + args[1].trim() + '".'})
    },
    RemoveRoutineAnswer: (...args) => {
      // BY THE PHRASE. A page sending a row number would record a number here.
      window.ansDropArgs = (window.ansDropArgs || []).concat(
        args.length + ':' + args[1])
      return Promise.resolve({id: args[0], name: 'address', askable: true,
        phrases: ['fill in the address'],
        said: '"address" no longer answers to "' + args[1] + '".'})
    },
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
const ansType = (said) => {
  document.getElementById('routineAnswerPhrase').value = said
}
let ansCount = 0, ansText = '', ansDrops = 0
let ansRefused = '', ansRefusedBad = 0, ansKept = ''
let ansOk = '', ansCleared = 'x'
let ansOtherCount = 9, ansOtherText = 'x'
let ansFlightCount = 9, ansFlightText = 'x'

setTimeout(() => { pickId('notepad/focus/address') }, 1000)
setTimeout(() => {
  // EVERY PHRASE, EACH WITH THE ONE THING THAT CAN BE DONE TO IT.
  const box = document.getElementById('routineAnswersList')
  ansCount = box.children.length
  ansText = box.textContent
  ansDrops = box.querySelectorAll('button').length
  ansType('settings')
  press('routineAnswerAddBtn')
}, 1300)
setTimeout(() => {
  // A REFUSED PHRASE STAYS WHERE IT WAS TYPED, and the reason is beside it.
  //
  // The next thing somebody does is change those words, and a collision refusal
  // names what to avoid — which is no use with the box emptied.
  ansRefused = shown('routineAnswersSaid')
  ansRefusedBad = document.getElementById('routineAnswersSaid')
    .classList.contains('no') ? 1 : 0
  ansKept = document.getElementById('routineAnswerPhrase').value
  ansType('  put my address in  ')
  press('routineAnswerAddBtn')
}, 1700)
setTimeout(() => {
  ansOk = shown('routineAnswersSaid')
  ansCleared = document.getElementById('routineAnswerPhrase').value
  // AND A REMOVAL NAMES THE PHRASE.
  document.getElementById('routineAnswersList')
    .querySelectorAll('button')[0].click()
}, 2100)
setTimeout(() => { pickId('notepad/focus/other') }, 2500)
setTimeout(() => {
  // THE PANEL BELONGS TO THE ROUTINE IT IS ABOUT, READ WHILE THE ANSWER IS STILL
  // IN FLIGHT. This is the whole window the guard exists for: the card on screen is
  // the other routine and the only phrases this page holds are the first one's.
  const box = document.getElementById('routineAnswersList')
  ansFlightCount = box.children.length
  ansFlightText = box.textContent
}, 2600)
setTimeout(() => {
  const box = document.getElementById('routineAnswersList')
  ansOtherCount = box.children.length
  ansOtherText = box.textContent
}, 3200)
setTimeout(() => {
  // AND WHETHER THE PAGE THREW AT ALL.
  //
  // WORTH ITS OWN COUNTER, and it is the reason this file took three runs to write: a
  // harness that declares a name the page already declares makes the page's whole script
  // a SyntaxError, so every function on it is undefined and every counter below reads
  // zero -- which looks exactly like a panel that does not work. A duplicate
  // let listed did it.
  document.title = 'err=' + (window.bad ? 1 : 0) +
    ' ansCount=' + ansCount +
    ' listsays=' + (ansText.includes('write my address') &&
                    ansText.includes('fill in the address') ? 1 : 0) +
    ' ansDrops=' + ansDrops +
    ' refusedsaid=' + (ansRefused.includes('asks which') ? 1 : 0) +
    ' refusedbad=' + ansRefusedBad +
    ' kept=' + (ansKept === 'settings' ? 1 : 0) +
    ' oksaid=' + (ansOk.includes('now also answers to') ? 1 : 0) +
    ' cleared=' + (ansCleared === '' ? 1 : 0) +
    ' addargs=' + ((window.ansAddArgs || []).join(',') ===
      '2:settings,2:  put my address in  ' ? 1 : 0) +
    ' dropargs=' + ((window.ansDropArgs || []).join(',') ===
      '2:write my address' ? 1 : 0) +
    ' otherlist=' + ansOtherCount +
    ' otherclean=' + (ansOtherText.includes('address') ? 0 : 1) +
    ' flightcount=' + ansFlightCount +
    ' flightclean=' + (ansFlightText.includes('address') ? 0 : 1) +
    ' asked=' + ((window.ansAsked || []).includes('notepad/focus/other') ? 1 : 0)
}, 3600)`

// aRenderedAnswersPage renders the window and reads the counters off its title.
func aRenderedAnswersPage(t *testing.T) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, answersHarness)
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
//
// First, because it is the failure that looks like all the others: a page whose script died
// reports zeros, and zeros read as a panel that does not work.
func TestTheAnswersPageDoesNotThrow(t *testing.T) {
	title, n := aRenderedAnswersPage(t)
	if n("err") != 0 {
		t.Fatalf("the page threw while this was being rendered: %q", title)
	}
}

// EVERY PHRASE IS LISTED, EACH WITH A REMOVE BESIDE IT.
func TestThePanelListsEveryPhraseWithARemove(t *testing.T) {
	title, n := aRenderedAnswersPage(t)
	if n("ansCount") != 2 || n("listsays") != 1 {
		t.Errorf("the panel lists %d phrases: %q", n("ansCount"), title)
	}
	if n("ansDrops") != 2 {
		t.Errorf("%d phrases offer a Remove, not both: %q", n("ansDrops"), title)
	}
}

// AND A REFUSED PHRASE STAYS IN THE BOX, WITH THE REASON BESIDE IT.
//
// The branch only the page can take. A collision refusal names the phrase to avoid, which is no
// use to somebody whose words have been cleared — and the next thing they do is change them.
func TestARefusedPhraseKeepsTheWordsAndSaysWhy(t *testing.T) {
	title, n := aRenderedAnswersPage(t)
	if n("refusedsaid") != 1 {
		t.Errorf("the refusal is not on the screen: %q", title)
	}
	if n("refusedbad") != 1 {
		t.Errorf("the refusal is not rendered as one: %q", title)
	}
	if n("kept") != 1 {
		t.Errorf("the refused words were cleared out of the box: %q", title)
	}
}

// AND A PHRASE THAT LANDED CLEARS THE BOX AND SAYS SO.
func TestAnAddedPhraseClearsTheBoxAndSaysSo(t *testing.T) {
	title, n := aRenderedAnswersPage(t)
	if n("oksaid") != 1 {
		t.Errorf("nothing was said about the phrase that landed: %q", title)
	}
	if n("cleared") != 1 {
		t.Errorf("the box still holds the phrase that was added: %q", title)
	}
}

// THE PAGE SENDS THE PHRASE AS TYPED, AND REMOVES BY NAMING ONE.
//
// Two arguments and no more, both times. A removal that sent the position it was drawn at would
// record a number here — which is the whole reason this panel needs no basis token: there is no
// position for a changed list to make mean something else.
func TestThePageSendsPhrasesAndNeverPositions(t *testing.T) {
	title, n := aRenderedAnswersPage(t)
	if n("addargs") != 1 {
		t.Errorf("the add did not send exactly the routine and the phrase as "+
			"typed: %q", title)
	}
	if n("dropargs") != 1 {
		t.Errorf("the removal did not name the phrase: %q", title)
	}
}

// AND THE PANEL BELONGS TO THE ROUTINE IT IS ABOUT.
//
// The defect this window has shipped in other panels: one routine's state under another's name.
// Choosing the other card asks again and shows nothing of the first.
func TestThePhrasePanelBelongsToTheRoutineItIsAbout(t *testing.T) {
	title, n := aRenderedAnswersPage(t)
	if n("asked") != 1 {
		t.Errorf("choosing another routine did not ask what IT answers to: %q", title)
	}
	if n("otherlist") != 0 || n("otherclean") != 1 {
		t.Errorf("the other routine's panel shows %d of the first one's phrases: %q",
			n("otherlist"), title)
	}
	// AND WHILE THE ANSWER IS STILL IN FLIGHT, which is the only window the guard
	// exists for: the card has changed and the phrases this page holds have not.
	if n("flightcount") != 0 || n("flightclean") != 1 {
		t.Errorf("a routine's phrases were drawn under another routine's name while "+
			"its own answer was still coming: %q", title)
	}
}

// A PHRASES PANEL THAT THROWS DOES NOT TAKE PLAY WITH IT.
//
// The defect this is about was real and cost five tests: the panel's read was the FIRST thing
// `renderRoutine` did, and every page harness in this package stubbed the routine list without
// it — so the page threw there and everything after it never ran, including the block that decides
// which window Play presses in. Play went dead on ten harnesses over a panel that has nothing to
// do with running anything.
//
// This reproduces it exactly: the binding is missing, so the page DOES throw where the phrases are
// read. What must survive is Play. A secondary panel does not belong upstream of the one control
// that acts, and nothing but the order of two blocks holds that.
func TestAThrowingPhrasePanelLeavesPlayAlone(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	stub := strings.Replace(answersHarness,
		`    RoutineAnswers: (id) => {`, `    NotRoutineAnswers: (id) => {`, 1)
	if stub == answersHarness {
		t.Fatal("the binding was not removed, so this renders the ordinary page")
	}
	stub = strings.Replace(stub,
		"PlaybackTarget: () => Promise.resolve({chosen: '', windows: []}),",
		"PlaybackTarget: () => Promise.resolve({chosen: '4660:77', windows: "+
			"[{id: '4660:77', label: 'a window', says: 'Play brings a window "+
			"to the front and presses there.'}]}),", 1)
	stub += `
setTimeout(() => {
  document.title = 'threw=' + (window.bad ? 1 : 0) +
    ' playlive=' + (document.getElementById('routinePlayBtn').disabled ? 0 : 1)
}, 4200)`
	title := renderForCounters(t, browser, stub)
	n := func(key string) int {
		m := regexp.MustCompile(key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	if n("threw") != 1 {
		t.Fatalf("the page did not throw, so this is not the case it is about: %q",
			title)
	}
	if n("playlive") != 1 {
		t.Errorf("Play is dead because the phrases panel threw: %q", title)
	}
}
