package main

// THE READINESS PANEL, RENDERED.
//
// Grepping the source proves the field is named in the file. It cannot prove the branch: a page
// that showed the card unconditionally would leave an empty red box above Record on every clean
// machine, and one that dropped the machine line when nothing was wrong would withhold the caveat
// at exactly the moment somebody believes a panel. Both of those pass a source guard.
//
// See [[ADR-254-what-a-recording-captures-is-stated]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// readyHarnessWith is a window whose readiness answer is the given JSON literal.
func readyHarnessWith(answer string) string {
	return `
window.go = {main: {
  Backend: {Loaded: () => Promise.resolve()},
  Acts: {
    // THE ASK BOX. Bound at startup, so a stub without it is a page that paints
    // nothing - which is the right failure: a binding the page calls and the window
    // does not export is exactly the defect these harnesses exist to catch.
    SuggestTargets: () => Promise.resolve([]),
    SuggestRoutine: () => Promise.resolve({kind: 'none'}),
    Demonstrations: () => Promise.resolve([]),
    Saved: () => Promise.resolve([]),
    Routines: () => Promise.resolve([]),
    Applications: () => Promise.resolve([]),
    Pictures: () => Promise.resolve([]),
    Picture: () => Promise.resolve(''),
    Learning: () => Promise.resolve({active: false}),
    Watch: () => Promise.resolve({act: '', active: false, transcript: null}),
    Recording: () => Promise.resolve({active: false}),
    History: () => Promise.resolve([]),
    Unrunnable: () => Promise.resolve([]),
    Library: () => Promise.resolve({dir: 'C:/x/routes'}),
    Ready: () => Promise.resolve(` + answer + `),
  },
}}
setTimeout(() => {
  const card = document.getElementById('readyCard')
  const mach = document.getElementById('readyMachine')
  document.title = 'cardshown=' + (card.classList.contains('hide') ? 0 : 1) +
    ' problems=' + card.querySelectorAll('div.no').length +
    ' blocks=' + (card.textContent.includes('This stops playing a routine') ? 1 : 0) +
    ' todo=' + (card.textContent.includes('build.cmd') ? 1 : 0) +
    ' build=' + (mach.textContent.includes('studio abc1234') ? 1 : 0) +
    ' library=' + (mach.textContent.includes('C:/x/routes') ? 1 : 0) +
    ' limits=' + (mach.textContent.includes('does not say the input hooks') ? 1 : 0)
}, 4500)
`
}

// aCleanMachine is what the panel is handed when there is nothing wrong.
const aCleanMachine = `{build: 'abc1234', marco: 'C:/m/marco.exe',
  director: 'C:/m/director.exe', library: 'C:/x/routes', problems: [],
  limits: 'This is what Marco has checked. It does not say the input hooks will ' +
          'install, that a press will land, or that anything looks at a screen.'}`

// aMachineWithAProblem has the quiet one: a sibling missing, with the spawn still succeeding.
const aMachineWithAProblem = `{build: 'abc1234', marco: 'marco.exe',
  director: 'C:/m/director.exe', library: 'C:/x/routes',
  problems: [{says: 'there is no marco.exe beside this window',
              blocks: 'playing a routine',
              do: 'run plugins\\\\studio\\\\build.cmd with this window CLOSED'}],
  limits: 'This is what Marco has checked. It does not say the input hooks will ' +
          'install, that a press will land, or that anything looks at a screen.'}`

// NOTHING WRONG: NO CARD, AND THE CAVEAT IS STILL THERE.
//
// The second half is the one that matters. A clean machine is when somebody reads a readiness
// panel as a promise, so that is when it has to say what it did not establish.
func TestACleanMachineShowsNoProblemCardAndStillSaysWhatItDidNotCheck(t *testing.T) {
	title, n := aRenderedReadyPage(t, aCleanMachine)
	if n("cardshown") != 0 {
		t.Errorf("an empty problem card is on screen above Record: %q", title)
	}
	if n("problems") != 0 {
		t.Errorf("the page drew %d problems from an empty list: %q",
			n("problems"), title)
	}
	if n("build") != 1 || n("library") != 1 {
		t.Errorf("the machine line does not name this build and its library: %q", title)
	}
	if n("limits") != 1 {
		t.Errorf("with nothing wrong, the page stopped saying what it had not "+
			"established: %q", title)
	}
}

// A PROBLEM IS SHOWN WITH WHAT IT STOPS AND WHAT TO DO.
//
// Three parts, and the third is the one a diagnostic usually withholds. Every problem this panel
// can report has exactly one answer, so leaving somebody to work it out makes a report into a
// riddle.
func TestAProblemIsShownWithWhatItStopsAndWhatToDo(t *testing.T) {
	title, n := aRenderedReadyPage(t, aMachineWithAProblem)
	if n("cardshown") != 1 {
		t.Fatalf("a problem was reported and no card appeared: %q", title)
	}
	if n("problems") != 1 {
		t.Errorf("the page drew %d of 1 problem: %q", n("problems"), title)
	}
	if n("blocks") != 1 {
		t.Errorf("the page does not say what the problem stops: %q", title)
	}
	if n("todo") != 1 {
		t.Errorf("the page says what is wrong and not what to do about it: %q", title)
	}
	if n("limits") != 1 {
		t.Errorf("the caveat is missing with a problem on screen: %q", title)
	}
}

// aRenderedReadyPage renders the page with one readiness answer.
func aRenderedReadyPage(t *testing.T, answer string) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, readyHarnessWith(answer))
	if !strings.Contains(title, "cardshown=") {
		t.Fatalf("the page never reported its counters: %q", title)
	}
	return title, func(key string) int {
		m := regexp.MustCompile(key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
}
