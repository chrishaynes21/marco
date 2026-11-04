package main

// A RECORDING THAT WON'T RUN OFFERS TO RECORD IT AGAIN, AND SIMPLIFY SAYS WHAT IT COSTS.
//
// A review whose presses were not all measured offered `Save anyway` as its filled button, and no
// way to record again from the review at all: the emphasised next step was saving a routine that
// refuses every press. The Architect's ruling:
//
//   Approve Record again as primary for a correctable recording refusal, with Save anyway
//   secondary where saving is supported. Record again must disclose that it discards the current
//   draft, preserve setup choices, and revalidate the chosen window. It must not imply
//   re-recording fixes an unsupported machine capability.
//
// and, on Simplify: don't hide its consequences in a tooltip -- explain the timing change and the
// supplied-text marks it takes off in the review where the person approves them.
//
// See [[Studio-coherence-audit-2026-11-02]], finding 4.

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// againReviewBackend is a window holding the given review until it is discarded, and the given
// window lists for the first and every later ask.
func againReviewBackend(review, firstWindows, laterWindows string) string {
	return `
window.calls = []
window.discarded = false
let windowsAsked = 0
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
    Demonstrations: () => Promise.resolve([]),
    Saved: () => Promise.resolve([]),
    Applications: () => Promise.resolve([]),
    Windows: () => { windowsAsked++
      return Promise.resolve(windowsAsked === 1 ? ` + firstWindows + ` : ` + laterWindows + `) },
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
    Recording: () => Promise.resolve(window.discarded
      ? {active: false, stop_key: 'F12'} : (` + review + `)),
    CancelRecording: () => { window.calls.push('cancel'); window.discarded = true
      return Promise.resolve('') },
    StartRecording: (name, win) => { window.calls.push('start:' + name + ':' + win)
      return Promise.resolve('') },
    RecordAgain: (id, win) => { window.calls.push('again:' + id + ':' + win)
      return Promise.resolve('') },
    PlaybackTarget: () => Promise.resolve({windows: []}),
    RoutineAnswers: (id) => Promise.resolve({id: id, phrases: [], askable: true}),
    PlayNeeds: (id) => Promise.resolve({id: id, fields: [], needs: false}),
    Routines: () => Promise.resolve([]),
  },
}}
const shown = (id) => {
  const el = document.getElementById(id)
  return (el && !el.classList.contains('hide')) ? el.textContent : ''
}
`
}

const aWontRunReview = `{active: false, review: true, name: 'graphics low',
  application: 'arkascended', stopped_by: 'the Stop button', pressable: 2,
  steps: ['left-click at 960, 540'], said: '1 step(s), 1 of which press something.',
  incomplete: true,
  missing: '1 press is missing a measurement: what scale it was drawn at. Record it ' +
    'again with the program in front of you.'}`

const twoWindows = `[{id: '4660', label: 'arkascended — ARK'}, {id: '7712', label: 'notepad — x'}]`

func againCounters(t *testing.T, stub, driver string) func(string) int {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, stub+driver)
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

func TestARecordingThatWontRunOffersRecordAgainFirstAndSaysItDiscards(t *testing.T) {
	n := againCounters(t, againReviewBackend(aWontRunReview, twoWindows, twoWindows), `
setTimeout(() => {
  const again = document.getElementById('capAgainBtn')
  const save = document.getElementById('capSaveBtn')
  document.title = 'ran=1' +
    ' again=' + (shown('capAgainRow') && again.classList.contains('go') ? 1 : 0) +
    ' savequiet=' + (save.classList.contains('go') ? 0 : 1) +
    ' saveanyway=' + (save.textContent.trim() === 'Save anyway' ? 1 : 0) +
    ' discloses=' + (shown('capAgainSays').includes('Discards this recording') ? 1 : 0)
}, 2500)`)
	if n("again") != 1 {
		t.Error("a recording that won't run does not offer Record again as the filled button")
	}
	if n("savequiet") != 1 || n("saveanyway") != 1 {
		t.Error("Save anyway is still the emphasised button, or is gone")
	}
	if n("discloses") != 1 {
		t.Error("Record again does not say it discards this recording")
	}
}

func TestAnOrdinaryReviewKeepsSaveFirstAndOffersNoRecordAgain(t *testing.T) {
	n := againCounters(t, againReviewBackend(`{active: false, review: true, name: 'ok',
  application: 'notepad', stopped_by: 'the stop key', pressable: 1, steps: ['press s'],
  said: '1 step(s), 1 of which press something.'}`, twoWindows, twoWindows), `
setTimeout(() => {
  document.title = 'ran=1' +
    ' savego=' + (document.getElementById('capSaveBtn').classList.contains('go') ? 1 : 0) +
    ' noagain=' + (shown('capWontRunBox') === '' ? 1 : 0)
}, 2500)`)
	if n("savego") != 1 || n("noagain") != 1 {
		t.Error("an ordinary review lost Save as its filled button, or offers Record again")
	}
}

func TestRecordAgainIsNotOfferedForWhatThisVersionCannotSave(t *testing.T) {
	n := againCounters(t, againReviewBackend(`{active: false, review: true, name: 'x',
  application: 'notepad', stopped_by: 'the stop key', pressable: 1, steps: ['press s'],
  said: '1 step(s).', incomplete: true, missing: '1 press is missing a measurement.',
  unsupported: 'This recording contains a drag. ... so it cannot be saved yet.'}`,
		twoWindows, twoWindows), `
setTimeout(() => {
  document.title = 'ran=1 noagain=' + (shown('capAgainRow') === '' ? 1 : 0) +
    ' refusal=' + (shown('capUnsupported').includes('cannot be saved yet') ? 1 : 0)
}, 2500)`)
	if n("noagain") != 1 {
		t.Error("Record again is offered for a recording this version cannot save, which " +
			"implies re-recording would fix it")
	}
	if n("refusal") != 1 {
		t.Error("the unsupported refusal is not shown")
	}
}

func TestRecordAgainDiscardsKeepsTheSetupAndStartsInTheSameWindow(t *testing.T) {
	n := againCounters(t, againReviewBackend(aWontRunReview, twoWindows, twoWindows), `
setTimeout(() => {
  // THE SETUP AS IT WAS WHEN THIS WAS RECORDED: the window that was chosen.
  loadWindows().then(() => { document.getElementById('capWindow').value = '4660' })
}, 1500)
setTimeout(() => { document.getElementById('capAgainBtn').click() }, 2000)
setTimeout(() => {
  const c = window.calls.join('|')
  document.title = 'ran=1 order=' + (c === 'cancel|start:graphics low:4660' ? 1 : 0) +
    ' calls=' + c.replace(/[^a-z0-9:|]/gi, '_')
}, 3000)`)
	if n("order") != 1 {
		t.Error("Record again did not discard first and then start with the same name in " +
			"the same window")
	}
}

func TestRecordAgainAsksForAWindowWhenTheOneItUsedHasGone(t *testing.T) {
	n := againCounters(t, againReviewBackend(aWontRunReview, twoWindows,
		`[{id: '7712', label: 'notepad — x'}]`), `
setTimeout(() => {
  loadWindows().then(() => { document.getElementById('capWindow').value = '4660' })
}, 1500)
setTimeout(() => { document.getElementById('capAgainBtn').click() }, 2000)
setTimeout(() => {
  const c = window.calls.join('|')
  document.title = 'ran=1 nostart=' + (c === 'cancel' ? 1 : 0) +
    ' form=' + (shown('captureCard') !== '' ? 1 : 0) +
    ' says=' + (shown('capNote').includes('no longer open') ? 1 : 0) +
    ' name=' + (document.getElementById('capName').value === 'graphics low' ? 1 : 0)
}, 3400)`)
	if n("nostart") != 1 {
		t.Error("Record again started in a window that is not the one it was recorded in")
	}
	if n("form") != 1 || n("says") != 1 {
		t.Error("with its window gone, Record again did not open the form and say why")
	}
	if n("name") != 1 {
		t.Error("Record again did not keep the name")
	}
}

func TestRecordAgainOfASecondRecordingKeepsItASecondRecording(t *testing.T) {
	review := strings.Replace(aWontRunReview, `incomplete: true,`,
		`incomplete: true, again: 'arkascended/focus/graphics-low',`, 1)
	n := againCounters(t, againReviewBackend(review, twoWindows, twoWindows), `
setTimeout(() => {
  loadWindows().then(() => { document.getElementById('capWindow').value = '4660' })
}, 1500)
setTimeout(() => { document.getElementById('capAgainBtn').click() }, 2000)
setTimeout(() => {
  document.title = 'ran=1 again=' +
    (window.calls.join('|') === 'cancel|again:arkascended/focus/graphics-low:4660' ? 1 : 0)
}, 3000)`)
	if n("again") != 1 {
		t.Error("recording a second recording again lost which routine it is a second " +
			"recording of")
	}
}

func TestSimplifySaysWhatItCostsBeforeAndAfter(t *testing.T) {
	before := againCounters(t, againReviewBackend(`{active: false, review: true, name: 'say',
  application: 'notepad', stopped_by: 'the stop key', pressable: 1,
  steps: ['type "{{item}}"'], said: '1 step(s).', marked: ['item']}`, twoWindows, twoWindows), `
setTimeout(() => {
  const s = shown('capSimplifySays')
  document.title = 'ran=1 timing=' + (s.includes('quarter of a second') ? 1 : 0) +
    ' marks=' + (s.includes('takes off “item”') ? 1 : 0)
}, 2500)`)
	if before("timing") != 1 || before("marks") != 1 {
		t.Error("before simplifying, the review does not say what happens to the timing " +
			"and which marked value it takes off")
	}
	after := againCounters(t, againReviewBackend(`{active: false, review: true, name: 'say',
  application: 'notepad', stopped_by: 'the stop key', pressable: 1,
  steps: ['type "stone"'], said: '1 step(s).', simplified: true, original: true,
  unmarked: ['item']}`, twoWindows, twoWindows), `
setTimeout(() => {
  const s = shown('capSimplifySays')
  document.title = 'ran=1 timing=' + (s.includes('quarter of a') ? 1 : 0) +
    ' back=' + (s.includes('use the original') ? 1 : 0) +
    ' marks=' + (s.includes('took off “item”') ? 1 : 0) +
    ' notip=' + (document.getElementById('capSimplifyBtn').title ? 0 : 1)
}, 2500)`)
	if after("timing") != 1 || after("back") != 1 || after("marks") != 1 {
		t.Error("beside Save, the simplified review does not say what changed, the way " +
			"back, and which marked value was taken off")
	}
	if after("notip") != 1 {
		t.Error("the consequence is carried by a tooltip")
	}
}
