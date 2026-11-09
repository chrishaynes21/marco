package main

// WHAT SOMEBODY TYPES INTO THE STEP EDITOR IS NOT LOST.
//
// The editor had a Set button on every row, and a value typed and not Set was silently
// left out when Review was pressed: the review then showed the other changes, or none,
// and the number was gone. The boxes ARE the change now. These hold the four ways that
// could go wrong again: a value with no confirming press is sent, an invalid one is kept
// exactly as typed with its reason on its row and nothing sent, a redraw of the list does
// not take the typing with it, and a review made before a later edit is withdrawn rather
// than left on screen with a live Save. And leaving the routine counts the typing as work.
//
// See [[Studio-coherence-audit-2026-11-02]], finding 6.

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestTypedStepValuesAreReviewedWithoutASetAndAreNeverDropped(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, stepEditBackend+`
// PAST THE STUB'S TWO REFUSALS, which other tests are about.
window.reviewed = 5
const typeIn = (id, v) => {
  const el = document.getElementById(id)
  el.value = v
  el.dispatchEvent(new Event('input'))
}
let noSet = 0, marked = 0, resetShown = 0, badSent = -1, badErr = '', badKept = ''
let badFocus = 0, keptRedraw = '', errRedraw = '', errCleared = 0, sentWait = 0
let sentPoint = 0, reviewShown = 0, withdrawn = 0, cancelled = '', leaveSays = ''
let keptLeave = ''

setTimeout(() => { pickId('notepad/focus/address') }, 1000)
setTimeout(() => { press('routineStepsBtn') }, 1200)
setTimeout(() => {
  noSet = document.querySelector('[id^=routineStepSet-],[id^=routineStepPlace-]') ? 0 : 1
  typeIn('routineStepMs-1', 'abc')
  marked = document.getElementById('routineStepRow-1').classList.contains('changed') ? 1 : 0
  resetShown = shown('routineStepReset-1') ? 1 : 0
  press('routineStepReviewBtn')
  badSent = (window.reviewArgs || []).length
  badErr = shown('routineStepErr-1')
  badKept = document.getElementById('routineStepMs-1').value
  badFocus = document.activeElement && document.activeElement.id === 'routineStepMs-1' ? 1 : 0
}, 1500)
setTimeout(() => {
  // A REDRAW OF THE LIST, caused by a removal toggled on another row.
  press('routineStepCut-3')
  keptRedraw = document.getElementById('routineStepMs-1').value
  errRedraw = shown('routineStepErr-1')
  press('routineStepCut-3')
  typeIn('routineStepMs-1', '800')
  errCleared = shown('routineStepErr-1') === '' ? 1 : 0
}, 1800)
setTimeout(() => {
  press('routineStepReviewBtn')
  const sent = (window.reviewArgs || []).join('|')
  sentWait = sent.includes('"op":"wait","at":1,"to":800') ? 1 : 0
  sentPoint = sent.includes('"op":"point"') ? 1 : 0
}, 2100)
setTimeout(() => {
  reviewShown = editShown() ? 1 : 0
  typeIn('routineStepMs-1', '900')
  withdrawn = editShown() ? 0 : 1
  cancelled = window.cancelledEdit || ''
}, 2400)
setTimeout(() => {
  // AND LEAVING ASKS, naming the typing, and the box still holds it.
  document.getElementById('routineOpen-notepad-focus-other').click()
  leaveSays = shown('routineLeaving')
  keptLeave = document.getElementById('routineStepMs-1').value
}, 2700)
setTimeout(() => {
  document.title = 'noset=' + noSet + ' marked=' + marked + ' reset=' + resetShown +
    ' badnotsent=' + (badSent === 0 ? 1 : 0) +
    ' baderr=' + (badErr.includes('not a whole number') ? 1 : 0) +
    ' badkept=' + (badKept === 'abc' ? 1 : 0) + ' badfocus=' + badFocus +
    ' keptredraw=' + (keptRedraw === 'abc' ? 1 : 0) +
    ' errredraw=' + (errRedraw !== '' ? 1 : 0) + ' errcleared=' + errCleared +
    ' sentwait=' + sentWait + ' nopoint=' + (sentPoint ? 0 : 1) +
    ' review=' + reviewShown + ' withdrawn=' + withdrawn +
    ' cancelled=' + (cancelled === 'tok-edit' ? 1 : 0) +
    ' leave=' + (leaveSays.includes('changes you made to its steps') ||
                 leaveSays.includes('change you made to its steps') ? 1 : 0) +
    ' keptleave=' + (keptLeave === '900' ? 1 : 0)
}, 3600)`)
	if !strings.Contains(title, "noset=") {
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
	if n("noset") != 1 {
		t.Errorf("a row still carries a Set button: %q", title)
	}
	if n("marked") != 1 || n("reset") != 1 {
		t.Errorf("a row that differs from the play is not marked or cannot be put back: %q", title)
	}
	if n("badnotsent") != 1 {
		t.Errorf("an entry that is not a number was sent to be reviewed: %q", title)
	}
	if n("baderr") != 1 || n("badkept") != 1 || n("badfocus") != 1 {
		t.Errorf("an invalid entry was not kept as typed with its reason on its row and the cursor in it: %q", title)
	}
	if n("keptredraw") != 1 || n("errredraw") != 1 {
		t.Errorf("redrawing the list lost what was typed, or its reason: %q", title)
	}
	if n("errcleared") != 1 {
		t.Errorf("correcting the entry did not take its reason away: %q", title)
	}
	if n("sentwait") != 1 || n("nopoint") != 1 {
		t.Errorf("Review did not send the typed value, or sent a change nobody made: %q", title)
	}
	if n("review") != 1 || n("withdrawn") != 1 || n("cancelled") != 1 {
		t.Errorf("a review made before a later edit stayed on screen, or its token was kept: %q", title)
	}
	if n("leave") != 1 || n("keptleave") != 1 {
		t.Errorf("leaving the play did not count the typing as work, or lost it: %q", title)
	}
}
