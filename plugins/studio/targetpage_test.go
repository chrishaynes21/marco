package main

// THE PAGE SHOWS WHICH WINDOW A RUN WILL PRESS IN, AND WILL NOT PLAY WITHOUT ONE.
//
// Rendered rather than grepped, for the reason `redraw_test.go` gives at length: a source guard
// passes as long as a field is NAMED somewhere in the file, so a page that left Play live with no
// window selected, or filled the choice in for somebody, or submitted a term it was not showing,
// would keep every such guard green.
//
// Three routines, because the three cases are different answers and not three renderings of one:
// one window is selected and said, two is a choice with Play dead until it is made, and none is
// Play unavailable with the reason that says what to start.
//
// The backend is stubbed because what is under test is the page. `routines_test.go` asks the same
// questions of the real `Acts` against a stated desktop, `oshost` holds what the receiving end
// does with the window, and `TestARunToldToActInAWindowThatIsNotThereIsRefused` is the two
// processes meeting for real.
//
// See [[ADR-272-the-window-a-run-acts-in-is-chosen-and-carried]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const targetHarness = `
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
    CancelReview: () => {},
    ReviewRebuilt: (id) => Promise.resolve({id: id, why: 'not what this asks'}),
    ScopeChoices: () => Promise.resolve([]),
    PlaybackTarget: (id) => {
      window.targetsAsked = (window.targetsAsked || []).concat(id)
      if (id === 'notepad/focus/one') {
        return Promise.resolve({
          chosen: '4660:77',
          windows: [{id: '4660:77', label: 'notepad — notes',
            says: 'Play brings notepad — notes to the front and presses there. ' +
                  'Its presses are checked against that window and stop if ' +
                  'something else comes forward.'}]})
      }
      if (id === 'notepad/focus/two') {
        return Promise.resolve({
          windows: [
            {id: '1:11', label: 'notepad — first',
             says: 'Play brings notepad — first to the front and presses there. ' +
                   'Its presses are checked against that window and stop if ' +
                   'something else comes forward.'},
            {id: '2:22', label: 'notepad — second',
             says: 'Play brings notepad — second to the front and presses there. ' +
                   'Its presses are checked against that window and stop if ' +
                   'something else comes forward.'}],
          choose: 'notepad has 2 windows open. Choose the one to press in.'})
      }
      return Promise.resolve({windows: [],
        why: 'ark has no window open on this screen, so there is nothing for this ' +
             'routine to press in. Start it and press Refresh.'})
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
    Routines: () => Promise.resolve([
      {id: 'notepad/focus/one', name: 'one window', slug: 'one', app: 'notepad',
       scope: 'app-anywhere', contract: 'guarded', runnable: true,
       source: 'use os.', generation: 'current', rebuildable: true, offer: true,
       wrote: 'x'},
      {id: 'notepad/focus/two', name: 'two windows', slug: 'two', app: 'notepad',
       scope: 'app-anywhere', contract: 'guarded', runnable: true,
       source: 'use os.', generation: 'current', rebuildable: true, offer: true,
       wrote: 'x'},
      {id: 'ark/focus/none', name: 'no window', slug: 'none', app: 'ark',
       scope: 'app-anywhere', contract: 'legacy', runnable: true,
       source: 'use os.', generation: 'current', rebuildable: true, offer: true,
       wrote: 'x'},
    ]),
  },
}}
// WHAT THE PAGE SUBMITTED, recorded as the pair it was: a Play that sent no window, or one
// that sent a term it was not showing, is the defect this is about.
window.go.main.Acts.PlayRoutine = (id, into) => {
  window.plays = (window.plays || []).concat(id + '|' + (into === undefined ? 'MISSING' : into))
  return Promise.resolve('')
}
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
const dead = () => document.getElementById('routinePlayBtn').disabled ? 1 : 0
const chosenValue = () => document.getElementById('routineInto').value
const options = () => Array.from(document.getElementById('routineInto').options)
  .map(o => o.value).join('+')
let oneRow = '', oneSays = '', oneDead = 1, oneValue = ''
let twoRow = '', twoSays = '', twoDead = 0, twoOptions = '', twoValue = 'x'
let twoDeadAfter = 1, twoSaysAfter = ''
let noneRow = 'x', noneWhy = '', noneDead = 0

setTimeout(() => { pickId('notepad/focus/one') }, 1100)
setTimeout(() => {
  // ONE CANDIDATE: SELECTED, AND STILL SHOWN.
  oneRow = shown('routineIntoRow')
  // THE LINE IS SHORT AND THE WHOLE SENTENCE IS ON THE CHOOSER: the backend's words,
  // about the window chosen, are the select's title. Coherence audit, finding 3.
  oneSays = shown('routineIntoSays') + ' || ' + document.getElementById('routineInto').title
  oneDead = dead()
  oneValue = chosenValue()
  press('routinePlayBtn')
}, 1500)
setTimeout(() => { pickId('notepad/focus/two') }, 1900)
setTimeout(() => {
  // TWO CANDIDATES: A CHOICE, AND NOTHING CHOSEN FOR THEM.
  twoRow = shown('routineIntoRow')
  twoSays = shown('routineIntoSays')
  twoDead = dead()
  twoOptions = options()
  twoValue = chosenValue()
  // AND PRESSING PLAY WITH NOTHING CHOSEN SENDS NOTHING: the button is dead, so this
  // click must not reach the backend at all.
  press('routinePlayBtn')
}, 2300)
setTimeout(() => {
  // NOW CHOOSE THE SECOND WINDOW, the way a person does.
  const sel = document.getElementById('routineInto')
  sel.value = '2:22'
  sel.dispatchEvent(new Event('change'))
}, 2600)
setTimeout(() => {
  twoDeadAfter = dead()
  twoSaysAfter = shown('routineIntoSays') + ' || ' +
    document.getElementById('routineInto').title
  press('routinePlayBtn')
}, 2900)
setTimeout(() => { pickId('ark/focus/none') }, 3200)
setTimeout(() => {
  // NO CANDIDATE: PLAY UNAVAILABLE, WITH THE REASON.
  noneRow = shown('routineIntoRow')
  noneWhy = shown('routineIntoWhy')
  noneDead = dead()
  press('routinePlayBtn')
}, 3600)
setTimeout(() => {
  document.title =
    ' onerow=' + (oneRow !== '' ? 1 : 0) +
    ' onevalue=' + (oneValue === '4660:77' ? 1 : 0) +
    ' onelabel=' + (oneRow.includes('notes') ? 1 : 0) +
    ' onesays=' + (oneSays.includes('checks it before each press') &&
        oneSays.includes('notes to the front') ? 1 : 0) +
    ' onedead=' + oneDead +
    ' tworow=' + (twoRow !== '' ? 1 : 0) +
    ' twooptions=' + (twoOptions === '+1:11+2:22' ? 1 : 0) +
    ' twovalue=' + (twoValue === '' ? 1 : 0) +
    ' twodead=' + twoDead +
    ' twosays=' + (twoSays.includes('2 windows open') ? 1 : 0) +
    ' twodeadafter=' + twoDeadAfter +
    ' twosaysafter=' + (twoSaysAfter.includes('second to the front') &&
        twoSaysAfter.includes('checks it before each press') ? 1 : 0) +
    ' nonerow=' + (noneRow === '' ? 1 : 0) +
    ' nonewhy=' + (noneWhy.includes('no window open') ? 1 : 0) +
    ' nonedead=' + noneDead +
    ' asked=' + (window.targetsAsked || []).join('~') +
    ' plays=' + (window.plays || []).join('~')
}, 4400)
`

func aRenderedTargetPage(t *testing.T) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, targetHarness)
	if !strings.Contains(title, "onerow=") {
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

// ONE WINDOW IS SELECTED FOR THEM, AND STILL SHOWN.
//
// Selected because there is nothing to choose; shown because the alternative is somebody
// discovering which window it was from where the presses landed.
func TestThePageNamesTheOneWindowARunWillPressIn(t *testing.T) {
	title, n := aRenderedTargetPage(t)

	for _, c := range []struct{ key, what string }{
		{"onerow", "the window a run will press in is not shown at all"},
		{"onevalue", "the one candidate is not selected"},
		{"onelabel", "the window is not named in a way anybody could recognise"},
		{"onesays", "nothing says what pressing Play hands over"},
	} {
		if n(c.key) != 1 {
			t.Errorf("%s: %q", c.what, title)
		}
	}
	if n("onedead") != 0 {
		t.Errorf("Play is dead although there is exactly one window to press in: %q",
			title)
	}
}

// TWO WINDOWS IS A CHOICE, AND PLAY IS DEAD UNTIL IT IS MADE.
//
// Nothing is selected for them: one of two windows of one program is not this window's decision,
// and a page that filled it in would press into whichever of somebody's two profiles it preferred.
// Dead rather than live-and-then-refused, because the refusal arrives after the decision.
func TestThePageWillNotPlayUntilOneOfSeveralWindowsIsChosen(t *testing.T) {
	title, n := aRenderedTargetPage(t)

	if n("tworow") != 1 || n("twooptions") != 1 {
		t.Errorf("both windows are not offered: %q", title)
	}
	if n("twovalue") != 1 {
		t.Errorf("a window was chosen for them out of two: %q", title)
	}
	if n("twodead") != 1 {
		t.Errorf("Play could be pressed with two windows and neither chosen: %q",
			title)
	}
	// AND NOTHING PROMISES ANYTHING UNTIL ONE IS CHOSEN. The handoff sentence names a
	// window; with none chosen there is no window to name, so what is shown is the ask.
	if n("twosays") != 1 {
		t.Errorf("the page promises a handoff before a window is chosen, or says "+
			"nothing at all: %q", title)
	}
	// AND CHOOSING ONE MAKES IT LIVE, with the sentence beside it.
	if n("twodeadafter") != 0 {
		t.Errorf("Play is still dead after a window was chosen: %q", title)
	}
	// AND THE SENTENCE IS ABOUT THE WINDOW THEY CHOSE. It was one sentence for the
	// whole list, composed for whichever window the backend had selected — so
	// choosing the other of two left the first one's sentence on screen, which is the
	// right words about the wrong window.
	if n("twosaysafter") != 1 {
		t.Errorf("after choosing the second window the page says nothing about what "+
			"Play hands over, or says it about the first: %q", title)
	}
}

// AND NO WINDOW MEANS PLAY IS UNAVAILABLE, WITH THE REASON AND NOT A DEAD BUTTON.
//
// The half that makes the rest safe: choosing a window is only an improvement on pressing into
// whatever is in front if NO window is a refusal rather than a fallback. The reason names what to
// start, because nothing else in the window would.
func TestThePageWillNotPlayWithNoWindowAndSaysWhatToStart(t *testing.T) {
	title, n := aRenderedTargetPage(t)

	if n("nonedead") != 1 {
		t.Errorf("Play could be pressed with nowhere to press: %q", title)
	}
	if n("nonewhy") != 1 {
		t.Errorf("nothing says why, or what to start: %q", title)
	}
	if n("nonerow") != 1 {
		t.Errorf("an empty chooser is shown beside the reason, which is a control "+
			"with nothing in it: %q", title)
	}
}

// AND EVERY PLAY CARRIES THE TERM THE PAGE WAS SHOWING, OR DOES NOT HAPPEN.
//
// The whole point, recorded call by call. Three presses of Play were made: with one window
// selected, with two and neither chosen, and with the second of the two chosen. The middle one
// must not have reached the backend at all.
func TestThePagePlaysWithTheWindowItWasShowing(t *testing.T) {
	title, _ := aRenderedTargetPage(t)

	got := listOf(t, title, "plays")
	want := "notepad/focus/one|4660:77~notepad/focus/two|2:22"
	if got != want {
		t.Errorf("the page played %q, want %q — one Play per window actually "+
			"chosen, each carrying the term that was on the screen: %q", got,
			want, title)
	}
	// AND THE WINDOW LIST IS ASKED FOR PER ROUTINE, because the candidates depend on the
	// routine's application and a list kept from the last one is a list of the wrong
	// windows.
	asked := listOf(t, title, "asked")
	if asked != "notepad/focus/one~notepad/focus/two~ark/focus/none" {
		t.Errorf("the page asked for %q: %q", asked, title)
	}
}

// listOf is one tilde-joined list out of the counters.
func listOf(t *testing.T, title, key string) string {
	t.Helper()
	m := regexp.MustCompile(`\b` + key + `=(\S*)`).FindStringSubmatch(title)
	if m == nil {
		t.Fatalf("no %s in %q", key, title)
	}
	return m[1]
}
