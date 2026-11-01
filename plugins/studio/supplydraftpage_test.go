package main

// THE REVIEW CARD DRAWS THE CONTROL THAT ASKS FOR A VALUE.
//
// RENDERED, because the backend returning a chooser and the page having somewhere to put it are
// two facts, and only something that runs the page joins them. That is not hypothetical in this
// window: `Recording.Away` has been in the payload since the absence rule landed and the page has
// no element for it, so the one thing review says about the time somebody spent away is invisible.
// A guard reading the source would pass on a page that built the options into a div it never
// shows, which is indistinguishable — from where a person is standing — from the silence this
// whole slice exists to close.
//
// Three states, because the control has three and two of them are "nothing to pick":
//
//	a folded typed step  → the caption, the chooser, and an option naming the text
//	one step per character → the caption telling somebody to simplify, and NO chooser
//	a recording of clicks  → no box at all
//
// The harness is `againpage_test.go`'s, which is the one that drives the review card.
//
// See [[ADR-281-a-value-can-be-asked-for-from-the-review]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// aRenderedSupplyPage renders the review card over a stated `Recording` and reports what the
// supply control looks like.
//
// THE BOX AND THE ROW ARE COUNTED SEPARATELY, because they answer to different facts: the box
// follows the sentence and the row follows the options, and a mutation that showed either
// unconditionally survives an assertion on the other. The option text is counted too — a chooser
// with the right number of options and the wrong labels is a control pointed at a step nobody can
// identify.
func aRenderedSupplyPage(t *testing.T, recording string) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, againHarnessWith(recording)+`
setTimeout(() => {
  const box = document.getElementById('capSupplyBox')
  const row = document.getElementById('capSupplyRow')
  const says = document.getElementById('capSupplySays')
  const pick = document.getElementById('capSupplyAt')
  const said = box.classList.contains('hide') ? '' : says.textContent
  const opts = Array.from(pick.options).map(o => o.textContent).join('/')
  document.title = 'box=' + (box.classList.contains('hide') ? 0 : 1) +
    ' row=' + (row.classList.contains('hide') ? 0 : 1) +
    ' opts=' + pick.options.length +
    ' named=' + (opts.includes('types "stone"') ? 1 : 0) +
    ' asks=' + (said.includes('ask for it when you play it') ? 1 : 0) +
    ' simplifyfirst=' + (said.includes('Press Simplify first') ? 1 : 0) +
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

// A STEP THAT TYPES SOMETHING CAN BE ASKED FOR, AND THE OPTION SAYS WHICH STEP.
func TestTheReviewCardDrawsTheControlThatAsksForAValue(t *testing.T) {
	title, n := aRenderedSupplyPage(t, `{review: true, name: 'cloud pull stone',
		steps: ['left-click at 722, 210 in the window', 'type "stone"'],
		pressable: 2,
		supply: [{at: 1, label: 'step 2 — types "stone"'}],
		supply_says: 'Ask for a value instead of typing it: pick a step, name the ' +
			'value, and the routine will ask for it when you play it.'}`)
	if n("box") != 1 || n("row") != 1 {
		t.Fatalf("the control is not on screen: %q", title)
	}
	if n("asks") != 1 {
		t.Errorf("the caption does not say what happens: %q", title)
	}
	if n("opts") != 1 {
		t.Errorf("the chooser offers %d options for one typed step: %q", n("opts"), title)
	}
	if n("named") != 1 {
		t.Errorf("the option does not name the text that step types: %q", title)
	}
	if n("undef") != 0 {
		t.Errorf("the caption rendered as `undefined`: %q", title)
	}
}

// AND A RECORDING WITH NOTHING TO PICK IS TOLD SO, WITH THE CHOOSER WITHDRAWN.
//
// The state a fresh recording of typing is actually in, and the one that would otherwise read as
// the feature not existing: the sentence stays and names the button, the row goes. A chooser with
// no options is worse than no chooser — it looks like a control that does nothing.
func TestAReviewWithNothingToAskForSaysToSimplifyFirst(t *testing.T) {
	title, n := aRenderedSupplyPage(t, `{review: true, name: 'cloud pull stone',
		steps: ['type "s"', 'wait 180ms', 'type "t"'], pressable: 2,
		supply_says: 'This recording types one step per character, as you typed it, ' +
			'and a value the routine asks for has to be one step. Press ' +
			'Simplify first — it folds a typed run into one step — and then ' +
			'you can ask for it.'}`)
	if n("box") != 1 {
		t.Fatalf("the card says nothing, which reads as a routine that cannot ask "+
			"for a value: %q", title)
	}
	if n("simplifyfirst") != 1 {
		t.Errorf("the sentence does not name the button that would help: %q", title)
	}
	if n("row") != 0 {
		t.Errorf("an empty chooser is on screen: %q", title)
	}
	if n("opts") != 0 {
		t.Errorf("the chooser holds %d options with nothing offered: %q",
			n("opts"), title)
	}
}

// AND A RECORDING OF CLICKS CARRIES NO BOX AT ALL.
//
// Which is most recordings of a game. The advice about simplifying is for a recording that types
// something; on one that does not it would be a paragraph about a thing nobody is trying to do.
func TestAReviewOfClicksHasNoSupplyBox(t *testing.T) {
	title, n := aRenderedSupplyPage(t, `{review: true, name: 'cloud pull stone',
		steps: ['left-click at 722, 210 in the window'], pressable: 1}`)
	if n("box") != 0 {
		t.Errorf("a recording that types nothing offers a value to supply: %q", title)
	}
	if n("row") != 0 {
		t.Errorf("the chooser is on screen: %q", title)
	}
}
