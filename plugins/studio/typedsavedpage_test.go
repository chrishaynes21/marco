package main

// THE SAVED ROUTINE'S PANEL DRAWS THE CONTROL THAT DECLARES TYPED TEXT, AND SENDS IT ALONE.
//
// RENDERED, because the claim is the page's and no backend test can take it: the declaration is
// the one operation that changes how many steps there are, so the contract refuses it beside
// anything else — and a page that sent it along with whatever somebody had already collected would
// turn that rule into a refusal met after a press. What has to be true is that the press CLEARS
// the collected changes and sends one operation.
//
// The backend stub is `stepeditpage_test.go`'s, which is the one that drives this panel; the driver
// below is this file's own.
//
// See [[ADR-282-a-run-of-steps-can-be-said-to-be-typed-text]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// A RUN IS OFFERED, CARRIES BOTH ITS ENDS, AND GOES ON ITS OWN.
func TestThePanelDeclaresATypedRunOnItsOwn(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, stepEditBackend+`
let box = 0, row = 0, opts = 0, named = 0, ranged = 0, costs = 0
let collected = 'x', clearedAsked = 'y', sent = '', only = 0, basisSent = 0

setTimeout(() => { pickId('notepad/focus/address') }, 1000)
setTimeout(() => { press('routineStepsBtn') }, 1200)
setTimeout(() => {
  const b = document.getElementById('routineTypedBox')
  const r = document.getElementById('routineTypedRow')
  const pick = document.getElementById('routineTypedAt')
  box = b.classList.contains('hide') ? 0 : 1
  row = r.classList.contains('hide') ? 0 : 1
  opts = pick.options.length
  named = Array.from(pick.options).map(o => o.textContent).join('/')
    .includes('types "stone"') ? 1 : 0
  ranged = Array.from(pick.options).map(o => o.value).join('/') === '1:3' ? 1 : 0
  costs = shown('routineTypedSays').includes('waits between them go') ? 1 : 0
  // SOMETHING IS COLLECTED FIRST, so that "sends it alone" is a claim with
  // something to be alone of.
  document.getElementById('routineStepMs-1').value = '800'
  document.getElementById('routineStepMs-1').dispatchEvent(new Event('input'))
}, 1500)
setTimeout(() => {
  collected = shown('routineStepAsked')
  document.getElementById('routineTypedAt').value = '1:3'
}, 1800)
let heldAsked = '', heldBox = '', heldWhy = '', sentWhileHeld = -1
setTimeout(() => {
  // AND WITH SOMETHING ELSE TYPED, THE DECLARATION WAITS -- IT DOES NOT CLEAR IT.
  //
  // It used to drop the collected changes and say so. With the boxes being the
  // changes, that press would throw away what somebody typed, so it sends nothing
  // and says what to do instead. READ IN THE SAME TICK AS THE CLICK: the handler
  // decides synchronously, and a later read would be a read of something else.
  const before = (window.reviewArgs || []).length
  const btns = Array.from(document.querySelectorAll('#routineTypedRow button'))
  btns[0].click()
  heldAsked = shown('routineStepAsked')
  heldBox = document.getElementById('routineStepMs-1').value
  heldWhy = document.getElementById('routineStepsWhy').textContent
  sentWhileHeld = (window.reviewArgs || []).length - before
}, 2000)
setTimeout(() => {
  // PUT THE WAIT BACK, AND NOW IT GOES -- ON ITS OWN.
  document.getElementById('routineStepReset-1').click()
  clearedAsked = shown('routineStepAsked')
  const btns = Array.from(document.querySelectorAll('#routineTypedRow button'))
  btns[0].click()
}, 2200)
setTimeout(() => {
  const args = (window.reviewArgs || [])
  sent = args.length ? args[args.length - 1] : ''
  only = (sent.includes('"op":"typed"') && !sent.includes('"op":"wait"')) ? 1 : 0
  basisSent = sent.startsWith('3:basis-one:') ? 1 : 0
}, 2500)

setTimeout(() => {
  document.title = 'box=' + box + ' row=' + row + ' opts=' + opts +
    ' named=' + named + ' ranged=' + ranged + ' costs=' + costs +
    ' collected=' + (collected ? 1 : 0) +
    ' held=' + (heldAsked !== '' && heldBox === '800' ? 1 : 0) +
    ' heldsays=' + (heldWhy.includes('Review or reset') ? 1 : 0) +
    ' heldsent=' + (sentWhileHeld === 0 ? 1 : 0) +
    ' cleared=' + (clearedAsked === '' ? 1 : 0) +
    ' only=' + only + ' basis=' + basisSent +
    ' upto=' + (sent.includes('"upto":3') ? 1 : 0)
}, 5200)`)
	if !strings.Contains(title, "box=") {
		t.Fatalf("the page never reported its counters: %q", title)
	}
	n := func(key string) int {
		m := regexp.MustCompile(`\b` + key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	if n("box") != 1 || n("row") != 1 {
		t.Fatalf("the control is not on the panel: %q", title)
	}
	if n("opts") != 1 || n("named") != 1 {
		t.Errorf("the chooser does not offer the run by what it would become: %q",
			title)
	}
	if n("ranged") != 1 || n("upto") != 1 {
		t.Errorf("the range did not survive the option and the call, so a press "+
			"would declare the wrong steps: %q", title)
	}
	if n("costs") != 1 {
		t.Errorf("the panel does not say what declaring costs: %q", title)
	}
	if n("collected") != 1 {
		t.Fatalf("nothing was collected before the press, so this cannot say the "+
			"declaration went alone: %q", title)
	}
	if n("held") != 1 || n("heldsent") != 1 {
		t.Errorf("declaring a typed run with another change typed threw the typing "+
			"away or sent the declaration anyway: %q", title)
	}
	if n("heldsays") != 1 {
		t.Errorf("a declaration that waited did not say what to do instead: %q", title)
	}
	if n("cleared") != 1 {
		t.Errorf("Reset did not take the wait back off the list: %q", title)
	}
	if n("only") != 1 {
		t.Errorf("the declaration was not sent on its own: %q", title)
	}
	if n("basis") != 1 {
		t.Errorf("the declaration did not carry the basis the list came with: %q",
			title)
	}
}
