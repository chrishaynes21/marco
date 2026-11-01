package main

// THE REVIEW CARD DRAWS THE CONTROL THAT SAYS A RUN IS TYPED TEXT.
//
// RENDERED, because the backend proposing a run and the page having somewhere to put it are two
// facts and only something that runs the page joins them. `Recording.Away` is the standing proof
// that they can come apart: it has been in the payload since the absence rule landed and the page
// has no element for it, so the one thing review says about time spent away is invisible.
//
// Two states, because the control has two:
//
//	a run to declare   → the caption, the chooser, and an option naming the text
//	nothing to declare → no box at all
//
// The harness is `againpage_test.go`'s, which is the one that drives the review card.
//
// See [[ADR-282-a-run-of-steps-can-be-said-to-be-typed-text]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// aRenderedTypedPage renders the review card over a stated `Recording` and reports what the
// typed-text control looks like.
//
// THE OPTION'S VALUE IS COUNTED, NOT ONLY ITS TEXT. A run is a RANGE, so the value carries both
// ends (`from:to`) and the press splits it — an option whose label is right and whose value is one
// number would send a declaration about the wrong steps, and no assertion on the visible text
// would notice.
func aRenderedTypedPage(t *testing.T, recording string) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, againHarnessWith(recording)+`
setTimeout(() => {
  const box = document.getElementById('capTypedBox')
  const row = document.getElementById('capTypedRow')
  const says = document.getElementById('capTypedSays')
  const pick = document.getElementById('capTypedAt')
  const said = box.classList.contains('hide') ? '' : says.textContent
  const labels = Array.from(pick.options).map(o => o.textContent).join('/')
  const values = Array.from(pick.options).map(o => o.value).join('/')
  document.title = 'box=' + (box.classList.contains('hide') ? 0 : 1) +
    ' row=' + (row.classList.contains('hide') ? 0 : 1) +
    ' opts=' + pick.options.length +
    ' named=' + (labels.includes('types "stone"') ? 1 : 0) +
    ' ranged=' + (values === '1:15' ? 1 : 0) +
    ' costs=' + (said.includes('waits between them go') ? 1 : 0) +
    ' outside=' + (said.includes('Waits outside the run are left alone') ? 1 : 0) +
    ' undef=' + (said.includes('undefined') ? 1 : 0)
}, 5200)`)
	if !strings.Contains(title, "box=") {
		t.Fatalf("the page never reported its counters: %q", title)
	}
	return title, func(key string) int {
		m := regexp.MustCompile(`\b` + key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
}

// A RUN OF HELD KEYS CAN BE DECLARED, AND THE OPTION CARRIES BOTH ENDS OF IT.
func TestTheReviewCardDrawsTheControlThatDeclaresTypedText(t *testing.T) {
	title, n := aRenderedTypedPage(t, `{review: true, name: 'cloud pull stone',
		steps: ['activate notepad', 'hold "s" down', 'wait 50ms', 'hold "t" down',
			'wait 50ms', 'let go of "s"', 'wait 30ms', 'hold "o" down',
			'wait 50ms', 'let go of "t"', 'hold "n" down', 'wait 50ms',
			'let go of "o"', 'wait 50ms', 'type "e"', 'let go of "n"'],
		pressable: 2, basis: 'b1',
		typed_runs: [{from: 1, to: 15, label: 'steps 2 to 16 — types "stone"'}],
		typed_says: 'Typing is recorded as held keys when the next key goes down ' +
			'before the last one comes up, which is most typing. If a run of ' +
			'steps is text you typed, say so and it becomes one step: the keys ' +
			'in it stop being held and the waits between them go. Waits ' +
			'outside the run are left alone.'}`)
	if n("box") != 1 || n("row") != 1 {
		t.Fatalf("the control is not on screen: %q", title)
	}
	if n("opts") != 1 {
		t.Errorf("the chooser offers %d options for one run: %q", n("opts"), title)
	}
	if n("named") != 1 {
		t.Errorf("the option does not name the text the run would become: %q", title)
	}
	if n("ranged") != 1 {
		t.Errorf("the option does not carry both ends of the range, so a press would "+
			"declare the wrong steps: %q", title)
	}
	if n("costs") != 1 || n("outside") != 1 {
		t.Errorf("the caption does not say what the conversion removes and what it "+
			"leaves: %q", title)
	}
	if n("undef") != 0 {
		t.Errorf("the caption rendered as `undefined`: %q", title)
	}
}

// AND A RECORDING WITH NOTHING TO DECLARE CARRIES NO BOX.
//
// Which is every recording whose typing already folded, and every recording of clicks. A control
// whose only answer is a refusal is worse than no control.
func TestAReviewWithNothingToDeclareHasNoTypedBox(t *testing.T) {
	title, n := aRenderedTypedPage(t, `{review: true, name: 'cloud pull stone',
		steps: ['left-click at 722, 210 in the window'], pressable: 1,
		basis: 'b1'}`)
	if n("box") != 0 {
		t.Errorf("a recording with nothing to declare offers the control: %q", title)
	}
	if n("row") != 0 {
		t.Errorf("the chooser is on screen: %q", title)
	}
	if n("opts") != 0 {
		t.Errorf("the chooser holds %d options with nothing offered: %q", n("opts"),
			title)
	}
}
