package main

// THE STEP PANEL OFFERS A STEP'S TEXT AND ITS KEY AS BOXES, AND SENDS WHAT WAS TYPED.
//
// Rendered, because what is offered on which row, what a box starts with, and what Review sends
// are facts about the running page. The backend is the step-edit stub with its two step bindings
// replaced: a list with a typing step, a press, and a hold around a click. See
// [[ADR-311-a-steps-text-and-key-can-be-changed]].

import (
	"encoding/json"
	"net/url"
	"regexp"
	"testing"
)

const textKeyHarness = stepEditBackend + `
window.go.main.Acts.StepEditing = (id) => Promise.resolve({id: id, name: 'address',
  editable: true, basis: 'basis-tk',
  rows: [
    {at: 0, says: 'bring notepad to the front'},
    {at: 1, says: 'type "hello"', types: true, text: 'hello'},
    {at: 2, says: 'press enter', keyed: true, key: 'enter'},
    {at: 3, says: 'hold shift', keyed: true, key: 'shift', holds: true, pairs: true,
     pairs_at: 5, pair_says: 'removes step 6 too: they are the two halves of holding "shift"'},
    {at: 4, says: 'left-click at 640, 480'},
    {at: 5, says: 'let go of shift', keyed: true, key: 'shift', holds: true, pairs: true,
     pairs_at: 3, pair_says: 'removes step 4 too: they are the two halves of holding "shift"'},
  ]})
window.go.main.Acts.ReviewStepEdit = (...args) => {
  window.sent = (window.sent || []).concat([JSON.stringify(args[2])])
  return Promise.resolve({id: args[0], why: 'a stub that prepares nothing'})
}
const val = (id) => { const e = document.getElementById(id); return e ? e.value : null }
const type = (id, v) => {
  const e = document.getElementById(id)
  e.value = v
  e.dispatchEvent(new Event('input'))
}
let r = {}
setTimeout(() => { pickId('notepad/focus/address') }, 1000)
setTimeout(() => { press('routineStepsBtn') }, 1200)
setTimeout(() => {
  // WHAT EACH ROW OFFERS, AND WHAT ITS BOX STARTS WITH.
  r.textBox = val('routineStepText-1') === 'hello' ? 1 : 0
  r.keyBox = val('routineStepKey-2') === 'enter' ? 1 : 0
  r.holdBoxes = val('routineStepKey-3') === 'shift' && val('routineStepKey-5') === 'shift'
    ? 1 : 0
  r.noKeyOnText = there('routineStepKey-1') ? 0 : 1
  r.noTextOnKey = there('routineStepText-2') ? 0 : 1
  r.noneOnClick = there('routineStepKey-4') || there('routineStepText-4') ? 0 : 1
  r.deadBefore = document.getElementById('routineStepReviewBtn').disabled ? 1 : 0
  // AND WHAT A PERSON TYPES IS THE CHANGE: a trailing space kept, a key as typed.
  type('routineStepText-1', 'hi there ')
  type('routineStepKey-2', 'F5')
}, 1500)
setTimeout(() => {
  const asked = document.getElementById('routineStepAsked').textContent
  r.asked = asked.includes('step 2 types “hi there ”') && asked.includes('step 3 uses F5')
    ? 1 : 0
  r.resetShown = document.getElementById('routineStepReset-1').classList.contains('hide')
    ? 0 : 1
  press('routineStepReviewBtn')
}, 1800)
setTimeout(() => {
  // AN EMPTY BOX IS A CORRECTION ON ITS ROW, NEVER SENT.
  type('routineStepKey-2', '')
  const err = document.getElementById('routineStepErr-2')
  r.emptyError = err && !err.classList.contains('hide') ? 0 : 1
}, 2300)
setTimeout(() => {
  press('routineStepReviewBtn')
}, 2500)
setTimeout(() => {
  const err = document.getElementById('routineStepErr-2')
  r.emptyError = err && !err.classList.contains('hide') &&
    err.textContent.includes('Enter a key') ? 1 : 0
  r.reviews = (window.sent || []).length
  document.title = 'err=0 ' + Object.keys(r).map(k => k + '=' + r[k]).join(' ') +
    ' sent=' + encodeURIComponent((window.sent || [])[0] || '')
}, 2900)`

func TestTheStepPanelOffersTextAndKeyBoxesAndSendsWhatWasTyped(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, textKeyHarness)
	n := counters(t, title, "textBox=")

	if n("textBox") != 1 || n("keyBox") != 1 || n("holdBoxes") != 1 {
		t.Errorf("a typing, press or hold row is not offered its box with what it holds "+
			"now: %s", title)
	}
	if n("noKeyOnText") != 1 || n("noTextOnKey") != 1 || n("noneOnClick") != 1 {
		t.Errorf("a box is offered on a row it does not belong to: %s", title)
	}
	if n("deadBefore") != 1 {
		t.Errorf("Review is live before anything was typed: %s", title)
	}
	if n("asked") != 1 || n("resetShown") != 1 {
		t.Errorf("what was typed is not listed as the change, or the row cannot be put "+
			"back: %s", title)
	}
	// WHAT REVIEW SENT, EXACTLY.
	m := regexp.MustCompile(`sent=(\S*)`).FindStringSubmatch(title)
	if m == nil {
		t.Fatalf("nothing was sent: %s", title)
	}
	raw, uerr := url.QueryUnescape(m[1])
	if uerr != nil {
		t.Fatalf("what was sent could not be read back: %v", uerr)
	}
	var sent []map[string]any
	if err := json.Unmarshal([]byte(raw), &sent); err != nil {
		t.Fatalf("what was sent is not a list of changes: %q (%v)", raw, err)
	}
	want := map[string]map[string]any{
		"text": {"op": "text", "at": float64(1), "text": "hi there "},
		"key":  {"op": "key", "at": float64(2), "key": "F5"},
	}
	for _, c := range sent {
		w, ok := want[c["op"].(string)]
		if !ok {
			t.Errorf("an unexpected change was sent: %v", c)
			continue
		}
		for k, v := range w {
			if c[k] != v {
				t.Errorf("the %s change sent %s=%v, want %v", c["op"], k, c[k], v)
			}
		}
		delete(want, c["op"].(string))
	}
	for op := range want {
		t.Errorf("no %s change was sent: %s", op, raw)
	}
	if n("emptyError") != 1 || n("reviews") != 1 {
		t.Errorf("an empty key box was sent, or not said on its row: %s", title)
	}
}
