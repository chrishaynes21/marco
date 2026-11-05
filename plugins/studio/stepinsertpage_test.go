package main

// A STEP IS ADDED ON THE PAGE: the `+` on a row opens a row under it, and Review sends it.
//
// Rendered, because where the new row appears, what it sends, and what happens to an empty one
// are facts about the running page. See [[ADR-312-a-step-can-be-added]].

import (
	"encoding/json"
	"net/url"
	"regexp"
	"testing"
)

const insertHarness = stepEditBackend + `
window.go.main.Acts.StepEditing = (id) => Promise.resolve({id: id, name: 'address',
  editable: true, basis: 'basis-in',
  rows: [
    {at: 0, says: 'bring notepad to the front'},
    {at: 1, says: 'wait 250ms', wait: true, ms: 250},
    {at: 2, says: 'press enter', keyed: true, key: 'enter'},
    {at: 3, says: 'type "hello"', types: true, text: 'hello'},
  ]})
window.go.main.Acts.ReviewStepEdit = (...args) => {
  window.sent = (window.sent || []).concat([JSON.stringify(args[2])])
  return Promise.resolve({id: args[0], why: 'a stub refusal of a change asked for',
    correct: true})
}
const set = (id, v) => {
  const e = document.getElementById(id)
  e.value = v
  e.dispatchEvent(new Event(e.tagName === 'SELECT' ? 'change' : 'input'))
}
let r = {}
setTimeout(() => { pickId('notepad/focus/address') }, 1000)
setTimeout(() => { press('routineStepsBtn') }, 1200)
setTimeout(() => {
  r.plusOnEvery = [0, 1, 2, 3].every(i => there('routineStepAdd-' + i)) ? 1 : 0
  press('routineStepAdd-2')
}, 1500)
setTimeout(() => {
  // THE NEW ROW IS UNDER THE ROW IT WAS ADDED AFTER, and has the cursor.
  const added = document.getElementById('routineStepNew-1')
  r.underIt = added && added.previousElementSibling &&
    added.previousElementSibling.id === 'routineStepRow-2' ? 1 : 0
  r.focused = document.activeElement && document.activeElement.id === 'routineStepNewVal-1'
    ? 1 : 0
  set('routineStepNewKind-1', 'key')
  set('routineStepNewVal-1', 'Tab')
  press('routineStepAdd-2')
}, 1800)
setTimeout(() => {
  set('routineStepNewVal-2', '150')
  press('routineStepAdd-3')
}, 2100)
setTimeout(() => {
  // THE THIRD IS LEFT EMPTY, AS TYPE: a correction on its own row, nothing sent.
  set('routineStepNewKind-3', 'type')
  press('routineStepReviewBtn')
}, 2400)
setTimeout(() => {
  const err = document.getElementById('routineStepNewErr-3')
  r.emptyError = err && !err.classList.contains('hide') &&
    err.textContent.includes('Type the text') ? 1 : 0
  r.namedWhy = document.getElementById('routineStepsWhy').textContent
    .includes('step added after step 4') ? 1 : 0
  r.nothingSent = (window.sent || []).length === 0 ? 1 : 0
  set('routineStepNewVal-3', ' and more')
  press('routineStepReviewBtn')
}, 2700)
setTimeout(() => {
  // AND A REFUSAL OF SOMETHING ASKED KEEPS THE ADDED ROWS, typing and all.
  r.kept = there('routineStepNew-1') && there('routineStepNew-3') &&
    document.getElementById('routineStepNewVal-3').value === ' and more' ? 1 : 0
  // TAKEN BACK, IT IS GONE.
  press('routineStepNewCut-2')
}, 3100)
setTimeout(() => {
  r.removed = there('routineStepNew-2') ? 0 : 1
  document.title = 'err=0 ' + Object.keys(r).map(k => k + '=' + r[k]).join(' ') +
    ' sent=' + encodeURIComponent((window.sent || [])[0] || '')
}, 3400)`

func TestAStepIsAddedOnThePageAndReviewSendsIt(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, insertHarness)
	n := counters(t, title, "plusOnEvery=")
	for _, c := range []struct{ key, why string }{
		{"plusOnEvery", "not every row offers to add a step after it"},
		{"underIt", "the added row is not under the row it was added after"},
		{"focused", "the added row does not get the cursor"},
		{"emptyError", "an empty added step is not a correction on its own row"},
		{"namedWhy", "the correction does not say which added step it is"},
		{"nothingSent", "a list with an empty added step was sent"},
		{"kept", "a refusal of something asked threw the added rows away"},
		{"removed", "an added row that was taken back is still there"},
	} {
		if n(c.key) != 1 {
			t.Errorf("%s: %s", c.why, title)
		}
	}
	m := regexp.MustCompile(`sent=(\S*)`).FindStringSubmatch(title)
	if m == nil {
		t.Fatalf("nothing was sent: %s", title)
	}
	raw, err := url.QueryUnescape(m[1])
	if err != nil {
		t.Fatal(err)
	}
	var sent []map[string]any
	if err := json.Unmarshal([]byte(raw), &sent); err != nil {
		t.Fatalf("what was sent is not a list of changes: %q", raw)
	}
	want := []map[string]any{
		{"op": "insert", "at": float64(2), "kind": "key", "key": "Tab"},
		{"op": "insert", "at": float64(2), "kind": "wait", "to": float64(150)},
		{"op": "insert", "at": float64(3), "kind": "type", "text": " and more"},
	}
	if len(sent) != len(want) {
		t.Fatalf("sent %d changes, want %d, in the order added: %s", len(sent), len(want), raw)
	}
	for i, w := range want {
		for k, v := range w {
			if sent[i][k] != v {
				t.Errorf("change %d sent %s=%v, want %v: %s", i+1, k, sent[i][k], v, raw)
			}
		}
	}
}
