package main

// AN ADDED CLICK IS GIVEN ITS BASIS, BUTTON AND PLACE ON THE PAGE, AND REVIEW SENDS THEM.
// Rendered. See [[ADR-315-a-click-can-be-added]].

import (
	"encoding/json"
	"net/url"
	"regexp"
	"testing"
)

func clickHarness(clicking string) string {
	return stepEditBackend + `
window.go.main.Acts.StepEditing = (id) => Promise.resolve({id: id, name: 'address',
  editable: true, basis: 'basis-ck', ` + clicking + `,
  rows: [
    {at: 0, says: 'bring notepad to the front'},
    {at: 1, says: 'left-click at 100, 80 in the window', point: true, movable: true,
     in_window: true, x: 100, y: 80},
  ]})
window.go.main.Acts.ReviewStepEdit = (...args) => {
  window.sent = (window.sent || []).concat([JSON.stringify(args[2])])
  return Promise.resolve({id: args[0], why: 'a stub refusal', correct: true})
}
const set = (id, v) => {
  const e = document.getElementById(id)
  e.value = v
  e.dispatchEvent(new Event(e.tagName === 'SELECT' ? 'change' : 'input'))
}
const options = (id) => { const e = document.getElementById(id)
  return e ? Array.from(e.options).map(o => o.value).join('+') : '' }
let r = {}
setTimeout(() => { pickId('notepad/focus/address') }, 1000)
setTimeout(() => { press('routineStepsBtn') }, 1200)
setTimeout(() => { press('routineStepAdd-1') }, 1500)
setTimeout(() => {
  r.kinds = encodeURIComponent(options('routineStepNewKind-1'))
  if (options('routineStepNewKind-1').includes('click')) { set('routineStepNewKind-1', 'click') }
}, 1800)
setTimeout(() => {
  r.bases = encodeURIComponent(options('routineStepNewBasis-1'))
  const says = document.getElementById('routineStepNewClickSays-1')
  r.says = says ? encodeURIComponent(says.textContent) : ''
  if (there('routineStepNewX-1')) {
    set('routineStepNewButton-1', 'right')
    set('routineStepNewX-1', '200')
    set('routineStepNewY-1', '100')
  }
}, 2100)
setTimeout(() => {
  r.asked = encodeURIComponent(document.getElementById('routineStepAsked').textContent)
  press('routineStepReviewBtn')
}, 2400)
setTimeout(() => {
  document.title = 'err=0 ' + Object.keys(r).map(k => k + '=' + r[k]).join(' ') +
    ' sent=' + encodeURIComponent((window.sent || [])[0] || '')
}, 2800)`
}

func renderClick(t *testing.T, clicking string) map[string]string {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, clickHarness(clicking))
	out := map[string]string{}
	for _, k := range []string{"kinds", "bases", "says", "asked", "sent"} {
		m := regexp.MustCompile(`\b` + k + `=(\S*)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", k, title)
		}
		v, err := url.QueryUnescape(m[1])
		if err != nil {
			t.Fatal(err)
		}
		out[k] = v
	}
	return out
}

func sentChanges(t *testing.T, raw string) []map[string]any {
	t.Helper()
	if raw == "" {
		return nil
	}
	var sent []map[string]any
	if err := json.Unmarshal([]byte(raw), &sent); err != nil {
		t.Fatalf("what was sent is not a list of changes: %q", raw)
	}
	return sent
}

func TestAClickAddedInsideAWindowIsSentWithItsBasis(t *testing.T) {
	got := renderClick(t, `click_bases: [{at: 1, label: 'the window step 2 was measured in `+
		`(notepad.exe, 1280×720, 96 dpi)', w: 1280, h: 720}]`)
	if got["kinds"] != "wait+key+type+click" {
		t.Errorf("the kinds offered are %q", got["kinds"])
	}
	if got["bases"] != "1" {
		t.Errorf("the bases offered are %q: a checked routine must not be offered a screen "+
			"point", got["bases"])
	}
	if got["asked"] != "To review: a right-click at 200, 100 in the window of step 2 is "+
		"added after step 2." {
		t.Errorf("the summary reads %q", got["asked"])
	}
	sent := sentChanges(t, got["sent"])
	if len(sent) != 1 {
		t.Fatalf("sent %v", sent)
	}
	for k, v := range map[string]any{"op": "insert", "kind": "click", "at": float64(1),
		"basis_at": float64(1), "screen": false, "button": "right", "x": float64(200),
		"y": float64(100)} {
		if sent[0][k] != v {
			t.Errorf("the click was sent with %s=%v, want %v", k, sent[0][k], v)
		}
	}
}

func TestAScreenClickIsOfferedOnlyWhereTheRoutineChecksNothing(t *testing.T) {
	got := renderClick(t, `screen_clicks: true, click_says: 'No step here was measured inside `+
		`a window, so a click can be added only at a point on the screen.'`)
	if got["bases"] != "screen" {
		t.Errorf("the bases offered are %q", got["bases"])
	}
	if got["says"] == "" {
		t.Error("the page does not say why there is no window to choose")
	}
	sent := sentChanges(t, got["sent"])
	if len(sent) != 1 || sent[0]["screen"] != true || sent[0]["x"] != float64(200) {
		t.Errorf("the screen click was sent as %v", sent)
	}
}

func TestNoClickIsOfferedWhereNoneCanBeAdded(t *testing.T) {
	got := renderClick(t, `click_says: 'A click cannot be added here.'`)
	if got["kinds"] != "wait+key+type" {
		t.Errorf("the kinds offered are %q", got["kinds"])
	}
}
