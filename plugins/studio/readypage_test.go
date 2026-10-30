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
  Backend: {
    Loaded: () => Promise.resolve(),
    // DIRECTOR ANSWERS THE ASK BOX NOW, so a stub without this is a page whose
    // request box throws — which is the right failure: a binding the page calls and
    // the window does not export is what these harnesses exist to catch.
    SuggestRoutine: () => Promise.resolve({answer: {kind: 'none'}}),
  },
  Acts: {
    // THE ASK BOX. Bound at startup, so a stub without it is a page that paints
    // nothing - which is the right failure: a binding the page calls and the window
    // does not export is exactly the defect these harnesses exist to catch.
    SuggestTargets: () => Promise.resolve([]),
    SuggestRoutine: () => Promise.resolve({kind: 'none'}),
    Demonstrations: () => Promise.resolve([]),
    Saved: () => Promise.resolve([]),
    // THE WINDOW A RUN WOULD PRESS IN. Studio does not Play without one, so a
    // harness that does not answer this is a page whose Play button is dead -
    // which is the right failure, and not what these are about.
    PlaybackTarget: () => Promise.resolve({chosen: '4660:77',
      windows: [{id: '4660:77', label: 'a window',
        says: 'Play brings a window to the front and presses there.'}]}),
    // THE PHRASES PANEL'S READ, which the page makes for every routine it
    // draws. A stub without it is a page that throws mid-render -- and note
    // that no backtick may appear in this comment: the harness around it is a
    // Go raw string, and one closed ten of these files at once.
    RoutineAnswers: (id) => Promise.resolve({id: id, phrases: [],
      askable: true}),
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
  // THE LIBRARY IS READ OFF THE ROUTINE LIST, not off the machine line.
  //
  // It was on both, which is why it moved: the line under every page carried the
  // library and both spawned binaries as four absolute paths. The claim is unchanged —
  // the window says which library it reads — and this is where it says it.
  const lib = document.getElementById('libWhere')
  document.title = 'cardshown=' + (card.classList.contains('hide') ? 0 : 1) +
    ' problems=' + card.querySelectorAll('div.no').length +
    ' blocks=' + (card.textContent.includes('This stops playing a routine') ? 1 : 0) +
    ' todo=' + (card.textContent.includes('build.cmd') ? 1 : 0) +
    ' build=' + (mach.textContent.includes('studio abc1234') ? 1 : 0) +
    ' library=' + (lib.textContent.includes('C:/x/routes') ? 1 : 0) +
    ' machinepaths=' + (mach.textContent.includes('C:/m/') ? 1 : 0) +
    ' limits=' + (document.getElementById('readyLimits').textContent.includes('a press will land') ? 1 : 0) +
    ' limitsmono=' + (document.getElementById('readyLimits').classList.contains('mach') ? 1 : 0)
}, 4500)
`
}

// aCleanMachine is what the panel is handed when there is nothing wrong.
const aCleanMachine = `{build: 'abc1234', marco: 'C:/m/marco.exe',
  director: 'C:/m/director.exe', library: 'C:/x/routes', problems: [],
  limits: 'Checked just now, without looking at a screen: this cannot say the ' +
          'input hooks will install, or that a press will land where you put it.'}`

// aMachineWithAProblem has the quiet one: a sibling missing, with the spawn still succeeding.
const aMachineWithAProblem = `{build: 'abc1234', marco: 'marco.exe',
  director: 'C:/m/director.exe', library: 'C:/x/routes',
  problems: [{says: 'there is no marco.exe beside this window',
              blocks: 'playing a routine',
              do: 'run plugins\\\\studio\\\\build.cmd with this window CLOSED'}],
  limits: 'Checked just now, without looking at a screen: this cannot say the ' +
          'input hooks will install, or that a press will land where you put it.'}`

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
	if n("build") != 1 {
		t.Errorf("the machine line does not name this build: %q", title)
	}
	if n("library") != 1 {
		t.Errorf("the window does not say which library it reads, anywhere: %q", title)
	}
	// AND IT DOES NOT CARRY THE SPAWN'S PATHS. Two absolute paths under every page,
	// for a comparison nobody can make by eye — and a spawn that would find the wrong
	// binary is a PROBLEM in the card above, with what it stops and what to do.
	if n("machinepaths") != 0 {
		t.Errorf("the machine line is naming the binaries it would spawn: %q", title)
	}
	if n("limits") != 1 {
		t.Errorf("with nothing wrong, the page stopped saying what it had not "+
			"established: %q", title)
	}
	// AND IT IS A SENTENCE, SET AS ONE. Mono marks a value — a revision, a path, a
	// coordinate — and never a sentence; this one shared an element with the build
	// line, so the only standing prose in the window was in the machine face.
	if n("limitsmono") != 0 {
		t.Errorf("the caveat is set in the monospace face: %q", title)
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
