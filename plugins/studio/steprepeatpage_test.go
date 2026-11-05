package main

// THE STEP PANEL OFFERS A COUNT WHERE A STEP REPEATS, AND A BLOCK WHERE ONE CAN EXIST.
//
// Rendered, because which rows carry a count box, what Review sends, and whether the block
// chooser is offered or explained are facts about the running page. See
// [[ADR-313-steps-can-be-repeated]].

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func repeatHarness(blocks bool) string {
	b := "blocks: true"
	if !blocks {
		b = "blocks_says: 'This routine checks each press against its window, so steps " +
			"cannot repeat as a block. A key press can still repeat by its count.'"
	}
	return stepEditBackend + `
window.go.main.Acts.StepEditing = (id) => Promise.resolve({id: id, name: 'address',
  editable: true, basis: 'basis-rp', ` + b + `,
  rows: [
    {at: 0, says: 'bring notepad to the front'},
    {at: 1, says: 'hold shift', keyed: true, key: 'shift', holds: true, pairs: true, pairs_at: 3,
     pair_says: 'removes step 4 too'},
    {at: 2, says: 'press a', keyed: true, key: 'a', repeats: true, count: 1},
    {at: 3, says: 'let go of shift', keyed: true, key: 'shift', holds: true, pairs: true,
     pairs_at: 1, pair_says: 'removes step 2 too'},
    {at: 4, says: 'repeat 2 times: press b', repeats: true, count: 2},
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
const seen = (id) => { const e = document.getElementById(id)
  return e && !e.classList.contains('hide') ? 1 : 0 }
let r = {}
setTimeout(() => { pickId('notepad/focus/address') }, 1000)
setTimeout(() => { press('routineStepsBtn') }, 1200)
setTimeout(() => {
  r.countOnPress = document.getElementById('routineStepCount-2').value === '1' ? 1 : 0
  r.countOnBlock = document.getElementById('routineStepCount-4').value === '2' ? 1 : 0
  r.noCountOnHold = there('routineStepCount-1') || there('routineStepCount-3') ? 0 : 1
  r.blockRow = seen('routineLoopRow')
  r.blockSays = seen('routineLoopSays')
  set('routineStepCount-2', '4')
}, 1500)
setTimeout(() => {
  r.asked = document.getElementById('routineStepAsked').textContent
    .includes('step 3 happens 4 times') ? 1 : 0
  // A BLOCK WAITS FOR THE OTHER CHANGES rather than throwing them away.
  set('routineLoopFrom', '1')
  set('routineLoopTo', '3')
  set('routineLoopCount', '3')
  press('routineLoopBtn')
  r.waited = document.getElementById('routineStepsWhy').textContent
    .includes('Review or reset the other changes first') && !(window.sent || []).length ? 1 : 0
  press('routineStepReviewBtn')
}, 1800)
setTimeout(() => {
  press('routineStepReset-2')
}, 2200)
setTimeout(() => {
  if (there('routineLoopBtn') && seen('routineLoopRow')) { press('routineLoopBtn') }
}, 2400)
setTimeout(() => {
  document.title = 'err=0 ' + Object.keys(r).map(k => k + '=' + r[k]).join(' ') +
    ' sent=' + encodeURIComponent(JSON.stringify(window.sent || []))
}, 2800)`
}

func renderRepeat(t *testing.T, blocks bool) (string, func(string) int, []string) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, repeatHarness(blocks))
	n := counters(t, title, "countOnPress=")
	m := regexp.MustCompile(`sent=(\S*)`).FindStringSubmatch(title)
	if m == nil {
		t.Fatalf("no record of what was sent: %s", title)
	}
	raw, err := url.QueryUnescape(m[1])
	if err != nil {
		t.Fatal(err)
	}
	var sent []string
	if err := json.Unmarshal([]byte(raw), &sent); err != nil {
		t.Fatalf("what was sent could not be read: %q", raw)
	}
	return title, n, sent
}

func TestTheStepPanelOffersACountAndABlockAndSendsThem(t *testing.T) {
	title, n, sent := renderRepeat(t, true)
	for _, c := range []struct{ key, why string }{
		{"countOnPress", "a key press is not offered its count"},
		{"countOnBlock", "a block is not offered its count"},
		{"noCountOnHold", "half of a hold is offered a count"},
		{"blockRow", "a routine that can repeat a block is not offered the chooser"},
		{"asked", "the changed count is not listed as the change"},
		{"waited", "repeating a block did not wait for the other changes, or threw them away"},
	} {
		if n(c.key) != 1 {
			t.Errorf("%s: %s", c.why, title)
		}
	}
	if n("blockSays") != 0 {
		t.Errorf("a routine that can repeat a block is told it cannot: %s", title)
	}
	if len(sent) != 2 {
		t.Fatalf("sent %d reviews, want the count and then the block: %v", len(sent), sent)
	}
	if !strings.Contains(sent[0], `"op":"repeat"`) || !strings.Contains(sent[0], `"count":4`) ||
		!strings.Contains(sent[0], `"at":2`) {
		t.Errorf("the count was sent as %s", sent[0])
	}
	if !strings.Contains(sent[1], `"op":"loop"`) || !strings.Contains(sent[1], `"at":1`) ||
		!strings.Contains(sent[1], `"upto":3`) || !strings.Contains(sent[1], `"count":3`) {
		t.Errorf("the block was sent as %s", sent[1])
	}
}

func TestAGuardedRoutineIsToldWhyItHasNoBlock(t *testing.T) {
	title, n, sent := renderRepeat(t, false)
	if n("blockRow") != 0 || n("blockSays") != 1 {
		t.Errorf("a routine that cannot repeat a block is offered the chooser, or not told "+
			"why: %s", title)
	}
	for _, s := range sent {
		if strings.Contains(s, `"op":"loop"`) {
			t.Errorf("a block was sent for a routine that cannot have one: %s", s)
		}
	}
}
