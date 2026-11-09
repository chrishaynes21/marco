package main

// THE WINDOW PAINTS THE RAIL IT WAS GIVEN, AND ASKS FOR NOTHING A WITHHELD VIEW WOULD WANT.
//
// # Why this is a browser and not a source grep
//
// Because every claim here is about what the page DOES over time, and the source holds the names
// either way. `paintViews` used to walk a hardcoded list and look each rail button up by id, so
// the first and most likely version of this change — delete the two buttons — throws mid-paint
// and stops the window rendering ALTOGETHER, with no console to say so in a `-H windowsgui`
// binary. A grep for `rail-acts` passes on a page that never paints.
//
// And the second claim is a count: a view nobody can reach must not keep reading for itself. A
// source guard cannot tell a poll that stopped from one whose variable is still spelled in the
// file; only something that runs the page can count the calls.
//
// See [[ADR-283-the-alpha-offers-one-journey-and-the-window-explains-itself]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// theAlphaBackend is a stub that offers what the alpha offers: Routines, Run history and Help.
//
// It COUNTS the two reads that belong to the withheld views, and the one that is shared — the
// recorder's state, which also paints the rail's build stamp and therefore does not stop.
const theAlphaBackend = `
window.__saved = 0; window.__demos = 0; window.__learning = 0
window.__help = 0; window.__build = 0
window.go = {main: {
  Backend: {
    Loaded: () => Promise.resolve(),
    SuggestRoutine: () => Promise.resolve({answer: {kind: 'none'}}),
  },
  Acts: {
    // THE WIDGET ACTION (S7), polled from start-up: the page draws whatever it says.
    WidgetStatus: () => Promise.resolve({state: 'ready', says: '', problems: []}),
    Theme: () => Promise.resolve({asked: '', theme: 'default', light: false, accentFrom: 'name', tokens: {}}),
    SuggestTargets: () => Promise.resolve([]),
    SuggestRoutine: () => Promise.resolve({kind: 'none'}),
    Demonstrations: () => { window.__demos++; return Promise.resolve([]) },
    Saved: () => { window.__saved++; return Promise.resolve([]) },
    Learning: () => { window.__learning++
      return Promise.resolve({active: false}) },
    // THE RAIL'S OWN READ. It answers whatever is offered, and it is NOT the
    // Scenes view's poll with two fields taken off it.
    BuildStatus: () => { window.__build++
      return Promise.resolve({build: 'abc1234', recorder: 'def5678',
        mismatched: true}) },
    Applications: () => Promise.resolve([]),
    Pictures: () => Promise.resolve([]),
    Picture: () => Promise.resolve(''),
    Watch: () => Promise.resolve({act: '', active: false, transcript: null}),
    History: () => Promise.resolve([]),
    Unrunnable: () => Promise.resolve([]),
    Recording: () => Promise.resolve({active: false}),
    PlaybackTarget: () => Promise.resolve({chosen: '', windows: []}),
    Ready: () => Promise.resolve({build: 'abc1234', problems: []}),
    Routines: () => Promise.resolve([]),
    Library: () => Promise.resolve({dir: 'C:/x/routes'}),
    // THE ALPHA RAIL. Acts and Scenes are not in it and are not claimed.
    Features: () => Promise.resolve({acts: false, scenes: false,
      views: ['routines', 'history', 'help']}),
    Help: () => { window.__help++
      return Promise.resolve({sections: [
        {key: 'recording', title: 'Recording a routine',
         lines: ['Press Record, and choose the window to record in.']},
        {key: 'arguments', title: 'Asking for a value each time',
         lines: ['Press ESC where the value goes.']},
        {key: 'editing', title: 'Editing what a routine does',
         lines: ['Nothing is written until you review it.']},
      ]}) },
  },
}}
const seen = (id) => {
  const el = document.getElementById(id)
  return el && !el.classList.contains('hide') ? 1 : 0
}
`

// THE RAIL IS THE THREE IT WAS GIVEN, THE OTHER TWO CANNOT BE REACHED, AND NOTHING READS FOR THEM.
func TestTheAlphaWindowOffersOneJourney(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, theAlphaBackend+`
let railR = 0, railA = 1, railS = 1, railH = 0, painted = 0
let wentA = 'x', viewA = 1, helpSecs = 0, helpOpen = 0, helpAt = ''

setTimeout(() => {
  // THE PAGE IS PAINTING AT ALL, which is the first thing a deleted rail button
  // takes away.
  painted = seen('view-routines')
  railR = seen('rail-routines'); railA = seen('rail-acts')
  railS = seen('rail-scenes'); railH = seen('rail-help')
}, 1200)
setTimeout(() => {
  // AND A WITHHELD VIEW IS NOT ARRIVED AT, however it is asked for. A hidden
  // button is not a closed door.
  show('acts')
  viewA = seen('view-acts')
  wentA = view
}, 1500)
setTimeout(() => {
  // AND HELP IS DRAWN, AND A LINK OPENS THE SECTION IT NAMES rather than the top
  // of a page of everything.
  helpOn('editing')
  helpAt = view
  helpSecs = document.getElementById('helpSections').children.length
  const d = document.getElementById('help-editing')
  helpOpen = d && d.open ? 1 : 0
}, 1800)

setTimeout(() => {
  document.title = 'painted=' + painted + ' railR=' + railR + ' railA=' + railA +
    ' railS=' + railS + ' railH=' + railH +
    ' viewA=' + viewA + ' wentA=' + (wentA === 'routines' ? 1 : 0) +
    ' helpSecs=' + helpSecs + ' helpOpen=' + helpOpen +
    ' helpAt=' + (helpAt === 'help' ? 1 : 0) +
    ' saved=' + window.__saved + ' demos=' + window.__demos +
    ' learning=' + window.__learning + ' help=' + window.__help +
    ' build=' + window.__build +
    ' stamp=' + (document.getElementById('build').textContent
      .indexOf('recorder def5678') >= 0 ? 1 : 0) +
    ' red=' + (document.getElementById('build').className.indexOf('no') >= 0 ? 1 : 0)
}, 5200)`)
	if !strings.Contains(title, "painted=") {
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
	if n("painted") != 1 {
		t.Fatalf("the Plays view is not on screen, so withholding a rail entry "+
			"stopped the window rendering: %q", title)
	}
	if n("railR") != 1 || n("railH") != 1 {
		t.Errorf("the rail does not offer Plays and Help: %q", title)
	}
	if n("railA") != 0 || n("railS") != 0 {
		t.Errorf("Acts or Scenes is still in the rail: %q", title)
	}
	if n("viewA") != 0 || n("wentA") != 1 {
		t.Errorf("asking for a withheld view arrived at it: %q", title)
	}
	if n("helpSecs") != 3 || n("helpOpen") != 1 || n("helpAt") != 1 {
		t.Errorf("Help is not drawn, or a link beside a control does not open the "+
			"section it names: %q", title)
	}
	// ── AND NOTHING READS FOR A VIEW NOBODY CAN REACH ──
	if n("saved") != 0 {
		t.Errorf("%d reads of the Acts list for a view that is not offered: %q",
			n("saved"), title)
	}
	if n("demos") != 0 {
		t.Errorf("%d reads of the demonstrations for a view that is not offered: %q",
			n("demos"), title)
	}
	// AND THE SCENES VIEW'S OWN READ IS NOT MADE AT ALL.
	//
	// It used to paint the rail's build stamp as well as the Scenes card, so
	// withholding Scenes was a choice between a poll running for a surface nobody
	// can reach and a rail that stops saying which build is recording. The stamp has
	// its own read now.
	if l := n("learning"); l != 0 {
		t.Errorf("%d read(s) of the Scenes view's state for a view that is not "+
			"offered: %q", l, title)
	}
	// ── AND THE RAIL STILL SAYS WHICH BUILD IS RECORDING ──
	//
	// Studio spawns the director beside ITSELF, so a stale one is invisible without
	// this; CLAUDE.md records that it cost the owner a day of recordings.
	if n("build") == 0 {
		t.Fatalf("the rail's build stamp is never read, so a stale director is "+
			"invisible: %q", title)
	}
	if n("stamp") != 1 {
		t.Errorf("the rail does not name the recorder's build beside its own: %q",
			title)
	}
	if n("red") != 1 {
		t.Errorf("the rail does not mark a build mismatch: %q", title)
	}
	// AND HELP IS ASKED FOR ONCE. None of it changes while the window is open.
	if h := n("help"); h != 1 {
		t.Errorf("Help was asked for %d times: %q", h, title)
	}
}

// AND WITH BOTH FLAGS ON, THE RAIL IS THE ONE THAT SHIPPED — PLUS HELP.
//
// The other half of the claim, and the one that says nothing was REMOVED. Acts and Scenes keep
// every capability they had; what the alpha withholds is the way in.
func TestTurningBothFlagsOnGivesBackTheWholeRail(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	stub := strings.Replace(theAlphaBackend,
		`Features: () => Promise.resolve({acts: false, scenes: false,
      views: ['routines', 'history', 'help']}),`,
		`Features: () => Promise.resolve({acts: true, scenes: true,
      views: ['routines', 'acts', 'scenes', 'history', 'help']}),`, 1)
	if stub == theAlphaBackend {
		t.Fatal("the stub's rail could not be replaced, so this test asserts nothing")
	}
	title := renderForCounters(t, browser, stub+`
let railA = 0, railS = 0, viewA = 0, viewS = 0

setTimeout(() => { railA = seen('rail-acts'); railS = seen('rail-scenes') }, 1200)
setTimeout(() => { show('acts'); viewA = seen('view-acts') }, 1500)
setTimeout(() => { show('scenes'); viewS = seen('view-scenes') }, 1800)
setTimeout(() => {
  document.title = 'railA=' + railA + ' railS=' + railS + ' viewA=' + viewA +
    ' viewS=' + viewS + ' saved=' + window.__saved + ' demos=' + window.__demos +
    ' learning=' + window.__learning + ' build=' + window.__build
}, 5200)`)
	n := func(key string) int {
		m := regexp.MustCompile(`\b` + key + `=(-?\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	if n("railA") != 1 || n("railS") != 1 {
		t.Fatalf("a flag that is on does not put its entry back in the rail: %q", title)
	}
	if n("viewA") != 1 || n("viewS") != 1 {
		t.Errorf("the views behind the flags cannot be reached: %q", title)
	}
	if n("saved") == 0 || n("demos") == 0 {
		t.Errorf("the views are offered and nothing fills them: %q", title)
	}
	// AND THE SCENES VIEW'S OWN READ IS RUNNING, which is the converse of the alpha
	// test's claim: withholding Scenes suppresses it, and offering Scenes must start
	// it. Fast, because it paints a recording in flight.
	if l := n("learning"); l < 3 {
		t.Errorf("%d read(s) of the Scenes view's state in five seconds with Scenes "+
			"offered, so a recording in flight would be narrated from a stale "+
			"answer: %q", l, title)
	}
	// AND THE RAIL'S OWN READ IS STILL ITS OWN, whichever views are offered.
	if n("build") == 0 {
		t.Errorf("the rail's build stamp is never read: %q", title)
	}
}
