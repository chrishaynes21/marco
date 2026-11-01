package main

// THE STEP EDITOR IS A LIST OF ROWS: NUMBER · ACTION · VALUE · CONTROLS.
//
// # Why this is a browser and not a source grep
//
// Two of the three claims here cannot be read out of the file.
//
// The first is the STRIKETHROUGH, which is a bug the source looks fine about. A removed row is
// struck out, and the sentence saying that removing it removes a SECOND step sat inside the
// struck element — `text-decoration` reaches every descendant and cannot be taken off one, so the
// row said, in the typeface of a thing already deleted, that something else was about to be
// deleted. Nothing in the markup says that; it is what the cascade does with it.
//
// The second is the COLUMN the verb lands in. The backend writes a step as one sentence and the
// page opens it on a closed table of prefixes; whether a given sentence lands in the action
// column or spills into the value column is a thing the page decides at render time.
//
// `journeypage_test.go` holds the other half — that every sentence `oneStep` writes is one the
// table opens, over a step of every kind. This holds that the row USES it.
//
// See [[ADR-284-the-step-editor-is-a-list-of-rows]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// THE ROWS ARE FOUR COLUMNS, AND A REMOVED ROW DOES NOT STRIKE OUT ITS OWN WARNING.
func TestTheStepRowsAreCompactAndTheWarningStaysReadable(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, stepEditBackend+`
// NOT NAMED acts: the page declares that at top level for its own backend
// binding, and a second top-level let of the same name in this script is a
// SyntaxError that takes the whole page with it. Measured: five rows rendered
// and every counter stayed at its initial value.
let four = 0, actCol = '', vals = '', whole = 0
let strikesSaid = 0, strikesWhat = 0, pairOutside = 0, pairText = 0
let rmName = '', rmKept = '', msName = '', noteGone = 1, linked = 0
let revFour = 0, revAct = ''

setTimeout(() => { pickId('notepad/focus/address') }, 1000)
setTimeout(() => { press('routineStepsBtn') }, 1200)
setTimeout(() => {
  const rows = Array.from(document.querySelectorAll('#routineStepRows .sr'))
  four = rows.length && rows.every(r => r.classList.contains('four')) ? 1 : 0
  actCol = rows.map(r => (r.querySelector('.act') || {}).textContent || '-')
    .join('/')
  vals = rows.map(r => (r.querySelector('.said') || {}).textContent || '-').join('/')
  // THE VERB IS NOT ALSO IN THE VALUE, which is what a split that did not happen
  // would look like.
  whole = vals.includes('bring ') || vals.includes('hold ') ? 1 : 0
  // THE CONTROLS SAY WHICH STEP THEY ARE ON.
  rmName = document.getElementById('routineStepCut-2').getAttribute('aria-label')
  msName = document.getElementById('routineStepMs-1').getAttribute('aria-label')
  // AND THE PARAGRAPH ABOVE THE LIST IS GONE, WITH A LINK IN ITS PLACE.
  noteGone = document.getElementById('routineStepsNote') ? 1 : 0
  linked = Array.from(document.querySelectorAll('#routineStepsRow .hint'))
    .length ? 1 : 0
  // NOW REMOVE THE HELD KEY, which is the row that carries a warning.
  press('routineStepCut-2')
}, 1500)
setTimeout(() => {
  const row = document.querySelector('#routineStepRows .sr.gone')
  if (!row) { return }
  const said = row.querySelector('.said')
  const what = row.querySelector('.what')
  const pair = row.querySelector('.pair')
  // WHAT IS STRUCK OUT: the value, and NOT the box the warning is in. A rule on
  // the .what class reaches the .pair class through the box tree and no style on the .pair class can
  // refuse it, so this is the only place the difference is visible.
  strikesSaid = getComputedStyle(said).textDecorationLine
    .includes('line-through') ? 1 : 0
  strikesWhat = getComputedStyle(what).textDecorationLine
    .includes('line-through') ? 1 : 0
  pairOutside = (pair && said && !said.contains(pair)) ? 1 : 0
  // AND THE BUTTON NOW SAYS WHAT IT WOULD UNDO, still naming the step.
  rmKept = document.getElementById('routineStepCut-2').getAttribute('aria-label')
  // AND THE REVIEW'S OWN LIST IS THE SAME SHAPE. Drawn directly, because reaching a
  // savable review through this stub takes three presses and the claim is about the
  // renderer: a review drawn differently from the list somebody read their changes
  // off is a second thing to parse at the moment they decide whether to save.
  stepReview = {id: 'x', token: 't', doing: ['x'], savable: true,
    rows: [{at: 0, says: 'left-click at 640, 480'}]}
  drawStepEdit()
  const rr = document.querySelector('#routineStepEditRows .sr')
  revFour = rr && rr.classList.contains('four') ? 1 : 0
  revAct = rr ? ((rr.querySelector('.act') || {}).textContent || '-') : '-'
  pairText = (pair && pair.textContent.includes('removes step 5')) ? 1 : 0
}, 1800)

setTimeout(() => {
  document.title = 'four=' + four + ' whole=' + whole +
    ' strikesSaid=' + strikesSaid + ' strikesWhat=' + strikesWhat +
    ' pairOutside=' + pairOutside + ' pairText=' + pairText +
    ' noteGone=' + noteGone + ' linked=' + linked +
    ' rmName=' + (rmName === 'Remove step 3' ? 1 : 0) +
    ' rmKept=' + (rmKept === 'Keep step 3' ? 1 : 0) +
    ' revFour=' + revFour + ' revAct=' + (revAct === 'Left-click' ? 1 : 0) +
    ' msName=' + (msName === 'Wait on step 2, in milliseconds' ? 1 : 0) +
    ' acts=[' + actCol + '] vals=[' + vals + ']'
}, 5200)`)
	if !strings.Contains(title, "four=") {
		t.Fatalf("the page never reported its counters: %q", title)
	}
	n := func(key string) int {
		m := regexp.MustCompile(`\b` + key + `=(-?\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	if n("four") != 1 {
		t.Fatalf("the step rows are not the four-column shape: %q", title)
	}
	// ── THE ACTION COLUMN HOLDS THE VERB, AND ONLY THE VERB ──
	acts := regexp.MustCompile(`acts=\[([^\]]*)\]`).FindStringSubmatch(title)
	if acts == nil {
		t.Fatalf("no action column in %q", title)
	}
	want := "Bring/Wait/Hold/Left-click/Let go"
	if acts[1] != want {
		t.Errorf("the action column reads %q, and the verbs for these five steps "+
			"are %q", acts[1], want)
	}
	if n("whole") != 0 {
		t.Errorf("a verb is in the value column as well as the action column, so "+
			"the sentence was not split at all: %q", title)
	}
	// ── AND THE WARNING SURVIVES THE REMOVAL IT IS ABOUT ──
	if n("pairText") != 1 {
		t.Fatalf("the removed row carries no warning about its other half, so this "+
			"cannot say whether the warning is readable: %q", title)
	}
	if n("strikesSaid") != 1 {
		t.Errorf("the removed step's value is not struck out, so nothing on the row "+
			"shows it is going: %q", title)
	}
	if n("strikesWhat") != 0 {
		t.Errorf("the strikethrough is on the box that CONTAINS the warning, so the "+
			"sentence saying a second step will be removed is struck out with "+
			"it — and no style on the sentence can refuse a decoration an "+
			"ancestor painted: %q", title)
	}
	if n("pairOutside") != 1 {
		t.Errorf("the warning is inside the struck-out element: %q", title)
	}
	if n("revFour") != 1 || n("revAct") != 1 {
		t.Errorf("the review's own list is not the shape of the list the changes "+
			"were read off: %q", title)
	}
	// ── AND THE PROSE MOVED, WITH A WAY BACK TO IT ──
	if n("noteGone") != 0 {
		t.Errorf("the paragraph above the step list is still on the panel: %q", title)
	}
	if n("linked") != 1 {
		t.Errorf("nothing beside the step list links to what can be changed, so the "+
			"explanation was removed rather than moved: %q", title)
	}
	// ── AND EACH CONTROL SAYS WHICH STEP IT IS ON ──
	if n("rmName") != 1 || n("rmKept") != 1 || n("msName") != 1 {
		t.Errorf("the remove button or the wait box has no name that identifies its "+
			"step, so a list of nine rows is nine controls called the same "+
			"thing: %q", title)
	}
}
