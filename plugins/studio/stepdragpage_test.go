package main

// THE STEP PANEL OFFERS A DRAG'S TWO ENDS, AND TURNS A CLICK INTO A DRAG, IN THE ROW'S BASIS.
// Rendered. See [[ADR-314-drags-can-be-edited]].

import (
	"encoding/json"
	"net/url"
	"regexp"
	"testing"
)

const dragHarness = stepEditBackend + `
window.go.main.Acts.StepEditing = (id) => Promise.resolve({id: id, name: 'address',
  editable: true, basis: 'basis-dg',
  rows: [
    {at: 0, says: 'bring notepad to the front'},
    {at: 1, says: 'left-click at 100, 80 in the window', point: true, movable: true,
     in_window: true, x: 100, y: 80, to_drag: true},
    {at: 2, says: 'drag from 10, 20 to 300, 40 in the window', drag: true,
     drag_movable: true, in_window: true, x: 10, y: 20, x2: 300, y2: 40},
    {at: 3, says: 'drag from 1, 2 to 3, 4', drag: true, x: 1, y: 2, x2: 3, y2: 4,
     drag_why: 'found by its picture: the position is inside that picture, not on the screen'},
  ]})
window.go.main.Acts.ReviewStepEdit = (...args) => {
  window.sent = (window.sent || []).concat([JSON.stringify(args[2])])
  return Promise.resolve({id: args[0], why: 'a stub refusal', correct: true})
}
const val = (id) => { const e = document.getElementById(id); return e ? e.value : null }
const set = (id, v) => {
  const e = document.getElementById(id)
  e.value = v
  e.dispatchEvent(new Event('input'))
}
let r = {}
setTimeout(() => { pickId('notepad/focus/address') }, 1000)
setTimeout(() => { press('routineStepsBtn') }, 1200)
setTimeout(() => {
  r.fourBoxes = val('routineStepX-2') === '10' && val('routineStepY-2') === '20' &&
    val('routineStepX2-2') === '300' && val('routineStepY2-2') === '40' ? 1 : 0
  r.noBoxesUnmovable = there('routineStepX-3') || there('routineStepX2-3') ? 0 : 1
  r.whyUnmovable = document.getElementById('routineStepRow-3').textContent
    .includes('found by its picture') ? 1 : 0
  r.noEndBeforeToggle = there('routineStepX2-1') ? 0 : 1
  r.noToggleOnDrag = there('routineStepToDrag-2') ? 0 : 1
  press('routineStepToDrag-1')
}, 1500)
setTimeout(() => {
  // THE CLICK'S DRAG STARTS WHERE THE CLICK IS.
  r.endStartsAtClick = val('routineStepX2-1') === '100' && val('routineStepY2-1') === '80'
    ? 1 : 0
  set('routineStepX2-1', '250')
  // AND ITS START MOVED TOO, which is where a drag and a move could both be sent.
  set('routineStepX-1', '110')
  set('routineStepY2-2', '90')
}, 1800)
setTimeout(() => {
  const asked = document.getElementById('routineStepAsked').textContent
  r.askedToDrag = asked.includes('step 2 becomes a drag from 110, 80 to 250, 80 in the window')
    ? 1 : 0
  r.askedDrag = asked.includes('step 3 drags from 10, 20 to 300, 90 in the window') ? 1 : 0
  press('routineStepReviewBtn')
}, 2100)
setTimeout(() => {
  // KEPT AS A CLICK, THE END BOXES GO.
  press('routineStepToDrag-1')
}, 2400)
setTimeout(() => {
  r.endGone = there('routineStepX2-1') ? 0 : 1
  document.title = 'err=0 ' + Object.keys(r).map(k => k + '=' + r[k]).join(' ') +
    ' sent=' + encodeURIComponent((window.sent || [])[0] || '')
}, 2700)`

func TestTheStepPanelOffersADragsEndsAndMakesAClickADrag(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, dragHarness)
	n := counters(t, title, "fourBoxes=")
	for _, c := range []struct{ key, why string }{
		{"fourBoxes", "a drag is not offered both ends with what they are now"},
		{"noBoxesUnmovable", "a drag that cannot be moved is offered boxes"},
		{"whyUnmovable", "a drag that cannot be moved does not say why"},
		{"noEndBeforeToggle", "a click shows a drag's end before it was asked to be a drag"},
		{"noToggleOnDrag", "a drag is offered to become a drag"},
		{"endStartsAtClick", "a click's drag does not start where the click is"},
		{"askedToDrag", "the click made a drag is not listed with its basis"},
		{"askedDrag", "the moved drag is not listed with its basis"},
		{"endGone", "keeping the click left the drag's end boxes"},
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
	want := map[string]map[string]float64{
		"todrag": {"at": 1, "x": 110, "y": 80, "x2": 250, "y2": 80},
		"drag":   {"at": 2, "x": 10, "y": 20, "x2": 300, "y2": 90},
	}
	for _, c := range sent {
		op, _ := c["op"].(string)
		if op == "point" {
			t.Errorf("a click being made a drag was also sent as a move: %s", raw)
		}
		w, ok := want[op]
		if !ok {
			continue
		}
		for k, v := range w {
			if c[k] != v {
				t.Errorf("the %s change sent %s=%v, want %v", op, k, c[k], v)
			}
		}
		delete(want, op)
	}
	for op := range want {
		t.Errorf("no %s change was sent: %s", op, raw)
	}
}
