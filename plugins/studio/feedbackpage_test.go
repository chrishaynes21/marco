package main

// FEEDBACK THE WINDOW HAD AND DID NOT GIVE.
//
// Four repairs from [[Studio-coherence-audit-2026-11-02]], each a sentence or a state the
// backend already had or the page already knew, and a person could not see:
//
//   - `Recording.Away` was sent and never drawn, so a routine came back shorter than the
//     demonstration with nothing saying why (finding 5).
//   - The run confirmation's lines were dropped into a three-column grid and broke over two
//     lines in its narrowest column (finding 9).
//   - The ask box's application selector had no options at all when nothing was running
//     (finding 17).
//   - Every Play stayed live while a recording ran, and the backend refused it (finding 15).
//
// And the line that names what Play is waiting for says it without a direction in it.

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// feedbackBackend is a window with two routines, one of which asks for `item`, and the
// recording state given.
func feedbackBackend(recording string) string {
	return `
window.go = {main: {
  Backend: {
    Loaded: () => Promise.resolve(),
    SuggestRoutine: () => Promise.resolve({answer: {kind: 'none'}}),
  },
  Acts: {
    // THE WIDGET ACTION (S7), polled from start-up: the page draws whatever it says.
    WidgetStatus: () => Promise.resolve({state: 'ready', says: '', problems: []}),
    Theme: () => Promise.resolve({asked: '', theme: 'default', light: false, accentFrom: 'name', tokens: {}}),
    // NOTHING RUNNING, so the selector is built from an empty answer.
    SuggestTargets: () => Promise.resolve([]),
    Demonstrations: () => Promise.resolve([]),
    Saved: () => Promise.resolve([]),
    Applications: () => Promise.resolve([]),
    Windows: () => Promise.resolve([{id: '4660', label: 'notepad \u2014 Untitled - Notepad'}]),
    Pictures: () => Promise.resolve([]),
    Picture: () => Promise.resolve(''),
    Learning: () => Promise.resolve({active: false}),
    Watch: () => Promise.resolve({act: '', active: false, transcript: null}),
    History: () => Promise.resolve([]),
    Unrunnable: () => Promise.resolve([]),
    Library: () => Promise.resolve({dir: 'C:/x/routes'}),
    Features: () => Promise.resolve({acts: false, scenes: false,
      views: ['routines', 'history', 'help']}),
    BuildStatus: () => Promise.resolve({build: 'abc1234'}),
    Help: () => Promise.resolve({sections: []}),
    Ready: () => Promise.resolve({build: 'abc1234', problems: [], limits: ''}),
    Recording: () => Promise.resolve(` + recording + `),
    PlaybackTarget: () => Promise.resolve({chosen: '4660:77', windows: [{id: '4660:77',
      label: 'notepad \u2014 Untitled - Notepad', says: 'Play brings it to the front.'}]}),
    RoutineAnswers: (id) => Promise.resolve({id: id, phrases: [], askable: true}),
    PlayNeeds: (id) => Promise.resolve(id.includes('stone')
      ? {id: id, name: 'pull stone', needs: true, fields: [{name: 'item', secret: false,
         says: 'typed into the routine where you tapped during the demonstration'}]}
      : {id: id, name: 'save', needs: false, fields: []}),
    ReviewPlay: (id) => Promise.resolve({id: id, name: 'pull stone', token: 'tok',
      says: ['item \u2014 "granite"']}),
    ConfirmPlay: () => Promise.resolve(''),
    CancelPlay: () => {},
    PlayRoutine: () => { window.played = (window.played || 0) + 1; return Promise.resolve('') },
    Routines: () => Promise.resolve([
      {id: 'notepad/focus/pull-stone', name: 'pull stone', slug: 'pull-stone',
       app: 'notepad', scope: 'app-anywhere', contract: 'guarded', runnable: true,
       clause: 'with item', saved_at: '2026-10-31 23:09', source: 'use os.',
       generation: 'current', rebuildable: true, offer: true, wrote: 'x'},
      {id: 'notepad/focus/save', name: 'save', slug: 'save', app: 'notepad',
       scope: 'app-anywhere', contract: 'guarded', runnable: true,
       saved_at: '2026-10-30 11:02', source: 'use os.', generation: 'current',
       rebuildable: true, offer: true, wrote: 'x'}]),
  },
}}
const shown = (id) => {
  const el = document.getElementById(id)
  return (el && !el.classList.contains('hide')) ? el.textContent : ''
}
`
}

func feedbackCounters(t *testing.T, recording, driver string) func(string) int {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, feedbackBackend(recording)+driver)
	if !strings.Contains(title, "ran=1") {
		t.Fatalf("the page never reported its counters: %q", title)
	}
	return func(key string) int {
		m := regexp.MustCompile(`\b` + key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
}

func TestAReviewSaysHowLongTheRecordingWasLeft(t *testing.T) {
	n := feedbackCounters(t, `{active: false, review: true, name: 'save',
  application: 'notepad', stopped_by: 'the stop key', pressable: 1,
  steps: ['press s'], said: '1 step(s), 1 of which press something.',
  away: 'You left the window once, for 8s in all. Nothing from out there was ' +
        'recorded, and the waiting is not in the routine.'}`, `
setTimeout(() => {
  document.title = 'ran=1 away=' + (shown('capAway').includes('for 8s in all') ? 1 : 0)
}, 2500)`)
	if n("away") != 1 {
		t.Error("the review does not say the recording was left, though the backend said so")
	}
}

func TestAnOrdinaryReviewHasNoAwayLine(t *testing.T) {
	n := feedbackCounters(t, `{active: false, review: true, name: 'save',
  application: 'notepad', stopped_by: 'the stop key', pressable: 1,
  steps: ['press s'], said: '1 step(s), 1 of which press something.'}`, `
setTimeout(() => {
  const el = document.getElementById('capAway')
  document.title = 'ran=1 boxed=' + (el.classList.contains('hide') ? 0 : 1)
}, 2500)`)
	if n("boxed") != 0 {
		t.Error("a review with nothing left out carries an away line anyway")
	}
}

func TestEveryPlayIsDarkAndSaysWhyWhileRecording(t *testing.T) {
	n := feedbackCounters(t, `{active: true, name: 'new one', stop_key: 'F12',
  window: 'notepad \u2014 Untitled - Notepad'}`, `
let dark = 0, says = '', pressed = 0
setTimeout(() => {
  dark = ['routinePlay-notepad-focus-pull-stone', 'routinePlay-notepad-focus-save']
    .every(id => document.getElementById(id).disabled) ? 1 : 0
  says = shown('routineSays-notepad-focus-save')
  // AND A PRESS ON IT ANYWAY OPENS NOTHING AND RUNS NOTHING.
  const b = document.getElementById('routinePlay-notepad-focus-save')
  b.disabled = false
  b.click()
  pressed = (window.played || 0) +
    (document.getElementById('routineOpen-notepad-focus-save')
      .getAttribute('aria-expanded') === 'true' ? 1 : 0)
}, 2500)
setTimeout(() => {
  document.title = 'ran=1 dark=' + dark +
    ' says=' + (says.includes('Finish the recording') ? 1 : 0) + ' pressed=' + pressed
}, 3000)`)
	if n("dark") != 1 {
		t.Error("a Play is live while something is being recorded")
	}
	if n("says") != 1 {
		t.Error("a dark Play during a recording does not say why")
	}
	if n("pressed") != 0 {
		t.Error("pressing Play during a recording opened or ran something")
	}
}

func TestPlaySaysWhatItIsWaitingForWithoutADirectionAndTheConfirmationIsOneLine(t *testing.T) {
	n := feedbackCounters(t, `{active: false}`, `
let wants = '', opts = '', plays = 0, line = 0
setTimeout(() => {
  opts = Array.from(document.getElementById('askTarget').options)
    .map(o => o.textContent).join('/')
  document.getElementById('routineOpen-notepad-focus-pull-stone').click()
}, 1200)
setTimeout(() => {
  wants = shown('routineWants')
  plays = document.getElementById('routinePlay-notepad-focus-save').disabled ? 0 : 1
  const box = document.getElementById('routineArg-item')
  box.value = 'granite'
  box.dispatchEvent(new Event('input'))
  document.getElementById('routinePlayBtn').click()
}, 2200)
setTimeout(() => {
  // ONE LINE: the confirmation's text spans the row rather than sitting in the
  // verb column, so it is no taller than one line of its own type.
  const li = document.querySelector('#routineWillTypeRows li')
  const whole = li && li.querySelector('.whole')
  const h = li ? li.getBoundingClientRect().height : 999
  const lh = whole ? parseFloat(getComputedStyle(whole).lineHeight) || 24 : 24
  line = (whole && h < lh * 1.6) ? 1 : 0
  document.title = 'ran=1 enter=' + (wants === 'Enter item to play.' ? 1 : 0) +
    ' nodirection=' + (/below|above/.test(wants) ? 0 : 1) +
    ' noapp=' + (opts === 'no application' ? 1 : 0) + ' plays=' + plays +
    ' line=' + line
}, 3000)`)
	if n("enter") != 1 || n("nodirection") != 1 {
		t.Error("the open row does not say `Enter item to play.`, or gives a direction")
	}
	if n("noapp") != 1 {
		t.Error("with nothing running the application selector is not `no application`")
	}
	if n("plays") != 1 {
		t.Error("a Play is dark with nothing being recorded")
	}
	if n("line") != 1 {
		t.Error("the confirmation's line does not span the row")
	}
}
