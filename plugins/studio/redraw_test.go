package main

// WHAT THE WINDOW REDRAWS WHILE NOTHING IS CHANGING.
//
// # Why this is a browser and not a source guard
//
// Every other page test in this package greps the source, which is the right shape for "does
// the page ask the backend for X" — a string either appears or it does not. It is the wrong
// shape for THIS, and a mutation run proved it: inverting `if (sig === shownReview) { return }`
// to `if (false)` leaves every one of those guards passing, because the variable is still named
// in the file. The behaviour is a count over time, and only something that runs the page can
// count it.
//
// # What was wrong
//
// Both polls run two and a half times a second for as long as the window is open, and both used
// to rebuild their whole subtree every time. For text that is invisible. For a PICTURE it is
// not: the finished review cleared its `.shots` boxes, asked the backend for three thumbnails
// per step again, and filled them in when they came back — so the images went away and returned
// twice a second and the page jumped as they did. The owner:
//
//	Also the images blink in the ui
//
// Measured on this harness, over about ten polls: 66 picture fetches before, 6 after. The run
// screen was worse — 108 against 9 — and its pictures were landing in the Act preview's boxes,
// because both lists gave their picture boxes the same element ids and both are in the document
// at once.
//
// # What it does NOT prove
//
// That the page looks right. It renders in Edge or Chrome rather than in WebView2, with the Go
// bindings stubbed, so what it holds is the arithmetic of redrawing and nothing about pixels.
// `studio.exe -evidence` remains the gate that the real webview comes up.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// aBrowser is a headless Chromium to render in, or "" when there is none.
//
// Edge first because it is what WebView2 is, and Chrome as the fallback. A machine with
// neither skips: this is a gate on a desktop, not a dependency of the package.
func aBrowser() string {
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		candidates = []string{
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		}
	default:
		candidates = []string{"microsoft-edge", "google-chrome", "chromium"}
	}
	for _, c := range candidates {
		if filepath.IsAbs(c) {
			if _, err := os.Stat(c); err == nil {
				return c
			}
			continue
		}
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

// harness is the stubbed backend the page polls, and the counters it reports back.
//
// The shapes are the real ones — `Learning`, `Demonstrations`, `Picture`, `Watch` — because the
// page reads named fields off them and a loose fixture would pass while the page rendered
// nothing. The title is the report: a `-dump-dom` run has no other way back.
const harness = `
window.__pic = 0; window.__demos = 0
const STEP = (i) => ({name: 'control_' + i, position: i, pressable: true, verifiable: true,
  recognisable: true, reads: ['video'], on: ['controls', 'display', 'video'],
  outcome_words: 23, routine: 'trackmania video',
  from_label: 'screen_a', control_label: 'control_' + i, to_label: 'screen_b'})
const D = {name: 'trackmania video', application: 'trackmania',
  saved_at: '2026-10-21 12:17', steps: [STEP(1), STEP(2)]}
const PNG = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCA' +
  'AAAC0lEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=='
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
    Demonstrations: () => { window.__demos++; return Promise.resolve([D]) },
    Saved: () => Promise.resolve([]),
    History: () => Promise.resolve([]),
    Applications: () => Promise.resolve(['trackmania']),
    Pictures: () => Promise.resolve([]),
    Picture: () => { window.__pic++; return Promise.resolve(PNG) },
    Watch: () => Promise.resolve({act: '', active: false, transcript: null}),
    // AND THE RECORDER-FIRST FLOW, STUBBED WHETHER OR NOT THIS TEST IS ABOUT IT.
    // The page polls it from its own startup, and an undefined method throws there —
    // which would take the rest of the page setup with it. See recording.go.
    Recording: () => Promise.resolve({active: false}),
    Unrunnable: () => Promise.resolve([]),
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
    // THE PLAY ROW'S READ, which the page makes for every routine it draws.
    // A stub without it is a page that throws mid-render; no backtick may appear
    // in this comment, because the harness around it is a Go raw string.
    PlayNeeds: (id) => Promise.resolve({id: id, fields: [], needs: false}),
    ReviewPlay: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    ConfirmPlay: () => Promise.resolve(''),
    CancelPlay: () => {},
    Routines: () => Promise.resolve([]),
    Library: () => Promise.resolve({dir: '/tmp/routes'}),
    // WHICH PARTS OF THIS WINDOW ARE OFFERED, AND WHAT IT EXPLAINS.
    // Both are read once at startup; a stub without them is a page
    // whose rail never paints. See help.go.
    Features: () => Promise.resolve({acts: true, scenes: true,
      views: ['routines', 'acts', 'scenes', 'history', 'help']}),
    // THE RAIL'S BUILD STAMP, which is its own read now rather than two
    // fields of the Scenes view's poll. See buildstatus.go.
    BuildStatus: () => Promise.resolve({build: 'abc1234'}),
    Help: () => Promise.resolve({sections: [{key: 'editing',
      title: 'Editing what a routine does', lines: ['nothing is written yet']}]}),
    Learning: () => Promise.resolve({name: 'trackmania video', application: 'trackmania',
      active: false, over: true, phase: 'ended', learned: D,
      said: 'Saved "trackmania video".', next: 'Build an Act from it.'}),
  },
}}
setTimeout(() => { show('scenes') }, 200)
setTimeout(() => {
  document.title = 'pic=' + window.__pic + ' demos=' + window.__demos +
    ' figs=' + document.querySelectorAll('#learnedWhat figure').length
}, 4200)
`

// A FINISHED REVIEW SITTING OPEN ASKS FOR ITS PICTURES ONCE.
func TestTheWindowDoesNotFetchTheSamePicturesOverAndOver(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	page, err := os.ReadFile(filepath.Join("frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	// THE STUB GOES IN FRONT OF THE PAGE'S OWN SCRIPT, because the page waits for
	// `window.go` and paints nothing until it exists.
	src := strings.Replace(string(page), "<script>",
		"<script>\n"+harness+"\n</script>\n<script>", 1)
	dir := t.TempDir()
	at := filepath.Join(dir, "page.html")
	if err := os.WriteFile(at, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	run := exec.Command(browser, "--headless=new", "--disable-gpu",
		"--no-first-run", "--no-default-browser-check",
		"--user-data-dir="+filepath.Join(dir, "profile"),
		"--virtual-time-budget=6000", "--dump-dom", at)
	done := make(chan struct{})
	var out []byte
	var runErr error
	go func() { out, runErr = run.Output(); close(done) }()
	select {
	case <-done:
	case <-time.After(90 * time.Second):
		_ = run.Process.Kill()
		t.Skip("the browser did not finish in time")
	}
	if runErr != nil && len(out) == 0 {
		t.Skipf("the browser would not render: %v", runErr)
	}

	title := regexp.MustCompile(`<title>([^<]*)</title>`).FindStringSubmatch(string(out))
	if title == nil {
		t.Fatalf("the page never reported its counters, so it did not run at all")
	}
	n := func(key string) int {
		m := regexp.MustCompile(key + `=(\d+)`).FindStringSubmatch(title[1])
		if m == nil {
			t.Fatalf("no %s in %q", key, title[1])
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	pics, demos, figs := n("pic"), n("demos"), n("figs")

	// TWO STEPS, THREE LABELS EACH. One pass is six; the poll ran about ten times.
	const wantPics = 6
	if figs != wantPics {
		t.Errorf("the review shows %d picture(s) and the fixture has %d, so this "+
			"says nothing about how often they were fetched", figs, wantPics)
	}
	if pics > wantPics {
		t.Errorf("the review asked for %d picture(s) where %d would do, so it is "+
			"clearing and refetching them while nothing changes \u2014 which is "+
			"the blinking", pics, wantPics)
	}
	// AND THE SCENE LIBRARY IS NOT REBUILT ON EVERY POLL EITHER. One at startup and
	// one when the review appeared; a third means it is being redrawn on a timer.
	if demos > 2 {
		t.Errorf("the Scene library was rebuilt %d time(s) while nothing changed",
			demos)
	}
}

// actHarness is a chosen Act with a run in flight, and the two things that go wrong there.
//
// `removed` counts nodes taken OUT of a picture box, which is the blink itself: a box that is
// cleared and refilled shows nothing in between. `runFigs` counts the pictures the RUN screen
// is showing, which is zero when it is painting into the preview's boxes instead of its own.
const actHarness = `
window.__pic = 0; window.__removed = 0
const STEP = (i) => ({name: 'control_' + i, position: i, reads: ['video'],
  on: ['controls', 'display'], expect: 9, expected: ['a', 'b'],
  routine: 'trackmania video',
  from_label: 'screen_a', control_label: 'control_' + i, to_label: 'screen_b'})
const ACT = {name: 'video act', application: 'trackmania', from: 'trackmania video',
  revision: '20261021T121748.000000000', runnable: true, waits: '',
  starts_on: ['controls', 'display', 'video'], interactions: ['one', 'two'],
  steps: [STEP(1), STEP(2)], play: 'use os.', identity: 'id'}
const PNG = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCA' +
  'AAAC0lEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=='
let tick = 0
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
    Saved: () => Promise.resolve([ACT]),
    History: () => Promise.resolve([]),
    Applications: () => Promise.resolve(['trackmania']),
    Pictures: () => Promise.resolve([]),
    Picture: () => { window.__pic++; return Promise.resolve(PNG) },
    Learning: () => Promise.resolve({active: false}),
    Recording: () => Promise.resolve({active: false}),
    Unrunnable: () => Promise.resolve([]),
    // THE WINDOW A RUN WOULD PRESS IN. Studio does not Play without one, so a
    // harness that does not answer this is a page whose Play button is dead.
    PlaybackTarget: () => Promise.resolve({chosen: '4660:77',
      windows: [{id: '4660:77', label: 'a window',
        says: 'Play brings a window to the front and presses there.'}]}),
    Routines: () => Promise.resolve([]),
    Library: () => Promise.resolve({dir: '/tmp/routes'}),
    // WHICH PARTS OF THIS WINDOW ARE OFFERED, AND WHAT IT EXPLAINS.
    // Both are read once at startup; a stub without them is a page
    // whose rail never paints. See help.go.
    Features: () => Promise.resolve({acts: true, scenes: true,
      views: ['routines', 'acts', 'scenes', 'history', 'help']}),
    // THE RAIL'S BUILD STAMP, which is its own read now rather than two
    // fields of the Scenes view's poll. See buildstatus.go.
    BuildStatus: () => Promise.resolve({build: 'abc1234'}),
    Help: () => Promise.resolve({sections: [{key: 'editing',
      title: 'Editing what a routine does', lines: ['nothing is written yet']}]}),
    Watch: () => { tick++
      return Promise.resolve({act: 'video act', live: true, reference: 'run-x',
        active: true, transcript: ['authorised by a press of Play'],
        state: {phase: 'acting',
          interaction: {index: Math.min(2, 1 + Math.floor(tick / 5)), of: 2, step: 1}}})
    },
  },
}}
// WATCH THE PREVIEW'S OWN BOXES for anything being taken out of them.
setTimeout(() => {
  const obs = new MutationObserver(ms => ms.forEach(m => {
    // FIGURES ONLY. A step row legitimately changes when the Act does; a picture
    // being taken away from a list that is still showing the same Act is the blink.
    m.removedNodes.forEach(n => {
      if (n.querySelectorAll) { window.__removed += n.querySelectorAll('figure').length }
      if (n.tagName === 'FIGURE') { window.__removed++ }
    })
  }))
  // THE CONTAINERS, not the boxes: a box destroyed with its parent is a box the
  // observer stops hearing from, and that is the case this is looking for.
  ;['adSteps', 'runSteps'].forEach(id =>
    obs.observe(document.getElementById(id), {childList: true, subtree: true}))
}, 900)
// AND CHOOSE THE ACT AGAIN AND AGAIN, which is what clicking between Acts does.
setTimeout(() => { chosen = 'video act'; loadActs() }, 300)
for (let i = 2; i < 10; i++) { setTimeout(() => { loadActs() }, 300 * i) }
setTimeout(() => {
  document.title = 'pic=' + window.__pic + ' removed=' + window.__removed +
    ' adFigs=' + document.querySelectorAll('#adSteps figure').length +
    ' runFigs=' + document.querySelectorAll('#runSteps figure').length
}, 4200)
`

// CHOOSING THE SAME ACT AGAIN DOES NOT TAKE ITS PICTURES DOWN AND PUT THEM BACK.
//
// And the run's step list paints its OWN picture boxes. Both lists are built by `stepsHTML` and
// both are in the document at once, so one set of element ids for two lists meant the run
// repainted the preview's boxes and left its own empty.
func TestTheWindowKeepsPicturesItIsAlreadyShowing(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, actHarness)
	n := func(key string) int {
		m := regexp.MustCompile(key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	// TWO STEPS, THREE LABELS EACH, IN TWO LISTS: the Act's preview and the run's.
	if ad, run := n("adFigs"), n("runFigs"); ad != 6 || run != 6 {
		t.Errorf("the preview shows %d picture(s) and the run shows %d, want 6 "+
			"each — a list showing none is painting into the other one's "+
			"boxes, which is what one set of element ids for both did", ad, run)
	}
	if r := n("removed"); r != 0 {
		t.Errorf("%d picture(s) were taken out of a box that was already showing "+
			"them, which is the blink", r)
	}
	if p := n("pic"); p > 12 {
		t.Errorf("%d picture fetches for twelve pictures, so choosing the same "+
			"Act again goes back to the backend for bytes it already had", p)
	}
}

// renderForCounters runs the page with a stubbed backend and returns what it put in its title.
func renderForCounters(t *testing.T, browser, stub string) string {
	t.Helper()
	return renderForCountersAt(t, browser, 0, 0, stub)
}

// AND THE SAME AT A STATED WIDTH, for a claim about a narrow window.
//
// A layout rule is only a rule at the size it fires at: the row's controls wrap under the name
// below 760px and squeeze it above, so a harness that only ever renders at the browser's default
// width cannot tell a working `@media` block from a missing one. Zero is that default, which is
// what every test that does not care about width gets.
func renderForCountersAt(t *testing.T, browser string, w, h int, stub string) string {
	t.Helper()
	page, err := os.ReadFile(filepath.Join("frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	src := strings.Replace(string(page), "<script>",
		"<script>\n"+stub+"\n</script>\n<script>", 1)
	dir := t.TempDir()
	at := filepath.Join(dir, "page.html")
	if err := os.WriteFile(at, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"--headless=new", "--disable-gpu",
		"--no-first-run", "--no-default-browser-check",
		"--user-data-dir=" + filepath.Join(dir, "profile"),
		"--virtual-time-budget=6000"}
	if w > 0 && h > 0 {
		args = append(args, fmt.Sprintf("--window-size=%d,%d", w, h))
	}
	run := exec.Command(browser, append(args, "--dump-dom", at)...)
	done := make(chan struct{})
	var out []byte
	var runErr error
	go func() { out, runErr = run.Output(); close(done) }()
	select {
	case <-done:
	case <-time.After(90 * time.Second):
		_ = run.Process.Kill()
		t.Skip("the browser did not finish in time")
	}
	if runErr != nil && len(out) == 0 {
		t.Skipf("the browser would not render: %v", runErr)
	}
	m := regexp.MustCompile(`<title>([^<]*)</title>`).FindStringSubmatch(string(out))
	if m == nil {
		t.Fatal("the page never reported its counters, so it did not run at all")
	}
	return m[1]
}

// THE REVIEW A STOP BUTTON OPENS, RENDERED.
//
// # Why this is a browser too
//
// Because `recording_test.go` proves what the BACKEND hands over and nothing about whether the
// window shows it. Those are the two halves this repository has eight recorded cases of getting
// separately right and never joining: a backend that returns a reviewed draft and a page that
// never draws one look identical from either side.
//
// So this renders the real page over a stubbed draft and counts what appears: the steps as list
// items, the three scope choices, and the Save button. And it counts them over about ten polls,
// which is the other claim — review is settled the moment the capture ends, so a page that
// rebuilds it on a timer would take the scope somebody had just chosen and put it back.
const reviewHarness = `
window.__scopeRedraws = 0
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
    History: () => Promise.resolve([]),
    Applications: () => Promise.resolve(['arkascended']),
    Pictures: () => Promise.resolve([]),
    Picture: () => Promise.resolve(''),
    Learning: () => Promise.resolve({active: false}),
    Watch: () => Promise.resolve({act: '', active: false, transcript: null}),
    // A DRAFT IN REVIEW: stopped by the button, nothing on disk, two clicks and a wait.
    Unrunnable: () => Promise.resolve([]),
    // THE WINDOW A RUN WOULD PRESS IN. Studio does not Play without one, so a
    // harness that does not answer this is a page whose Play button is dead.
    PlaybackTarget: () => Promise.resolve({chosen: '4660:77',
      windows: [{id: '4660:77', label: 'a window',
        says: 'Play brings a window to the front and presses there.'}]}),
    Routines: () => Promise.resolve([]),
    Library: () => Promise.resolve({dir: '/tmp/routes'}),
    // WHICH PARTS OF THIS WINDOW ARE OFFERED, AND WHAT IT EXPLAINS.
    // Both are read once at startup; a stub without them is a page
    // whose rail never paints. See help.go.
    Features: () => Promise.resolve({acts: true, scenes: true,
      views: ['routines', 'acts', 'scenes', 'history', 'help']}),
    // THE RAIL'S BUILD STAMP, which is its own read now rather than two
    // fields of the Scenes view's poll. See buildstatus.go.
    BuildStatus: () => Promise.resolve({build: 'abc1234'}),
    Help: () => Promise.resolve({sections: [{key: 'editing',
      title: 'Editing what a routine does', lines: ['nothing is written yet']}]}),
    Recording: () => Promise.resolve({name: 'ark graphics', active: false, review: true,
      application: 'arkascended', stopped_by: 'the Stop button', pressable: 2,
      stop_key: 'F12',
      steps: ['bring arkascended to the front', 'left-click at 842, 274',
              'wait 2095ms', 'left-click at 496, 283'],
      said: '4 step(s), 2 of which press something.'}),
  },
}}
// EVERY REBUILD OF THE SCOPE LIST, COUNTED. It is the control a person touches, so it is
// the one whose redrawing actually costs them something.
setTimeout(() => {
  new MutationObserver(ms => ms.forEach(m => {
    if (m.addedNodes.length) { window.__scopeRedraws++ }
  })).observe(document.getElementById('capScope'), {childList: true})
}, 300)
setTimeout(() => {
  const vis = (id) => {
    const e = document.getElementById(id)
    return e && !e.classList.contains('hide') ? 1 : 0
  }
  document.title = 'steps=' + document.querySelectorAll('#capSteps li').length +
    ' scopes=' + document.querySelectorAll('#capScope option').length +
    ' redraws=' + window.__scopeRedraws +
    ' review=' + vis('capReview') + ' card=' + vis('captureCard') +
    ' saveoff=' + (document.getElementById('capSaveBtn').disabled ? 1 : 0)
}, 4200)
`

// THE STOP BUTTON'S REVIEW APPEARS, WHOLE, AND IS DRAWN ONCE.
func TestTheReviewAStopButtonOpensIsRenderedOnce(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, reviewHarness)
	n := func(key string) int {
		m := regexp.MustCompile(key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	// THE REVIEW IS WHAT IS ON SCREEN, and the card that starts a recording is not:
	// a page showing both would be offering to record over a draft nobody has saved.
	if n("review") != 1 {
		t.Errorf("the review card is not showing over a draft in review: %q", title)
	}
	if n("card") != 0 {
		t.Errorf("the Record card is still showing beside a draft in review: %q",
			title)
	}
	// AND EVERY STEP IS LISTED. Four in the fixture: a review that showed only the
	// presses would be a review of a routine that does less than the one being saved.
	if got := n("steps"); got != 4 {
		t.Errorf("the review lists %d of 4 step(s): %q", got, title)
	}
	// AND ALL THREE PLACES IT COULD WORK, so the question can actually be answered.
	if got := n("scopes"); got != 3 {
		t.Errorf("the review offers %d of 3 scope(s), so somebody cannot say "+
			"where their routine should work: %q", got, title)
	}
	// AND SAVE IS OFFERED, because this draft presses something.
	if n("saveoff") != 0 {
		t.Errorf("Save is disabled over a draft with two presses in it: %q", title)
	}
	// AND THE SCOPE LIST WAS BUILT ONCE. The poll runs about ten times in the budget;
	// rebuilding the control resets it, so a person who chose "anywhere" and then
	// pressed Save would have saved something else.
	if got := n("redraws"); got > 1 {
		t.Errorf("the scope list was rebuilt %d time(s) while nothing changed, "+
			"which discards the choice somebody just made: %q", got, title)
	}
}

// ── THE LIBRARY, AND PRESSING PLAY ON SOMETHING IN IT ─────────────────────────

// THE SAVED ROUTINES ARE DRAWN, AND PLAY IS OFFERED FOR THE ONE THAT CAN RUN.
//
// # Why this is a browser too
//
// For the reason the review above is: `routines_test.go` proves what the BACKEND lists and
// nothing about whether the window shows it. A backend that answers with a library and a page
// that never draws one look identical from either side, and that is the shape of defect this
// repository has eight recorded cases of.
//
// What it counts: the cards, that choosing one opens its detail, that Play is DISABLED for a
// routine that cannot press anything — a button that is live and answers a press with "this
// routine cannot be performed" is a window telling somebody off for pressing what it offered
// them — and that a library sitting somewhere else is named rather than passed over in silence.
const libraryHarness = `
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
    History: () => Promise.resolve([]),
    Applications: () => Promise.resolve(['arkascended']),
    Pictures: () => Promise.resolve([]),
    Picture: () => Promise.resolve(''),
    Learning: () => Promise.resolve({active: false}),
    Watch: () => Promise.resolve({act: '', active: false, transcript: null}),
    Recording: () => Promise.resolve({active: false}),
    Unrunnable: () => Promise.resolve([]),
    // TWO ROUTINES OF THE SAME NAME, in two applications — one that checks every
    // press and can run, and one saved under a scope that brings no window forward
    // so every press in it would refuse.
    //
    // The same name deliberately. This page kept r.name and every lookup from it
    // answered with the FIRST row that matched, so clicking the second card showed
    // the first one's source, its sentence and its Play button. Rows that differed
    // by name could not ask that question, and the version of this harness that had
    // two different names did not.
    // THE WINDOW A RUN WOULD PRESS IN. Studio does not Play without one, so a
    // harness that does not answer this is a page whose Play button is dead.
    PlaybackTarget: () => Promise.resolve({chosen: '4660:77',
      windows: [{id: '4660:77', label: 'a window',
        says: 'Play brings a window to the front and presses there.'}]}),
    Routines: () => Promise.resolve([
      {id: 'arkascended/focus/settings', name: 'settings', slug: 'settings',
       app: 'arkascended', scope: 'app-anywhere', contract: 'guarded',
       runnable: true, saved_at: '2026-10-24 04:00',
       checks: 'Every press is checked against the window it was recorded in.',
       source: 'the Ark is a script. // ark'},
      {id: 'discord/context/settings', name: 'settings', slug: 'settings',
       app: 'discord', scope: 'app', contract: 'guarded', runnable: false,
       why: 'every press in it checks the window it was made in, and it was saved ' +
            'to switch nothing',
       checks: 'Every press is checked against the window it was recorded in.',
       source: 'the Discord is a script. // discord'},
    ]),
    Library: () => Promise.resolve({dir: 'C:/Users/x/AppData/Roaming/marco/routes',
      elsewhere: 'D:/Macros/marco/routes',
      says: '3 routine(s) are in D:/Macros/marco/routes, which is not the library above.'}),
    // WHICH PARTS OF THIS WINDOW ARE OFFERED, AND WHAT IT EXPLAINS.
    // Both are read once at startup; a stub without them is a page
    // whose rail never paints. See help.go.
    Features: () => Promise.resolve({acts: true, scenes: true,
      views: ['routines', 'acts', 'scenes', 'history', 'help']}),
    // THE RAIL'S BUILD STAMP, which is its own read now rather than two
    // fields of the Scenes view's poll. See buildstatus.go.
    BuildStatus: () => Promise.resolve({build: 'abc1234'}),
    Help: () => Promise.resolve({sections: [{key: 'editing',
      title: 'Editing what a routine does', lines: ['nothing is written yet']}]}),
    PlayRoutine: (id) => { window.__played = id; return Promise.resolve('') },
  },
}}
// CHOOSE THE SECOND ONE — the namesake that cannot run — by clicking its card the way a
// person would, and then press Play.
// OPENED BY ITS DISCLOSURE. The row is no longer the control: the routine's own
// panel is inside it, so only the chevron and the name toggle it.
const open = (n) => {
  const cards = document.querySelectorAll('#routineList > .card.pick')
  if (cards.length <= n) { return }
  const d = cards[n].querySelector('.disclose')
  d.click()
  // AND LEAVING A ROUTINE SOMEBODY HAS TYPED INTO ASKS FIRST. These tests are
  // about what happens once that is agreed to, and the press that agrees is the
  // same press -- so this opens it and answers, which is what a person does.
  const warn = document.getElementById('routineLeaving')
  if (warn && !warn.classList.contains('hide')) { d.click() }
}
setTimeout(() => { open(1) }, 2000)
// AND THEN THE FIRST ONE, AND PRESS PLAY ON IT. Which routine the button performs is the
// half of this that no amount of correct rendering covers.
setTimeout(() => { open(0) }, 3000)
// AND PLAY IS PRESSED ON A LATER TICK, because choosing a routine asks the backend which
// windows it could be played into and Play is dead until that answers. Clicking both in one
// tick pressed a disabled button and performed nothing — which is the behaviour working,
// and was this harness asking the wrong question. See target.go.
setTimeout(() => {
  const b = document.getElementById('routinePlayBtn')
  if (b && !b.disabled) { b.click() }
}, 3400)
setTimeout(() => {
  const vis = (id) => {
    const e = document.getElementById(id)
    return e && !e.classList.contains('hide') ? 1 : 0
  }
  document.title = 'cards=' + document.querySelectorAll('#routineList > .card.pick').length +
    ' chosen=' + vis('routineChosen') +
    ' elsewhere=' + vis('libElsewhere') +
    ' where=' + (document.getElementById('libWhere').textContent.includes('marco/routes')
                   ? 1 : 0) +
    ' played=' + (window.__played === 'arkascended/focus/settings' ? 1
                    : window.__played === undefined ? 0 : 9) +
    ' shown=' + (document.getElementById('routineSource').textContent.includes('// ark')
                   ? 1 : 0) +
    ' secondshown=' + window.__secondShown +
    ' secondwhy=' + window.__secondWhy +
    ' secondoff=' + window.__secondOff
}, 4600)
// WHAT THE SECOND CARD SHOWED WHILE IT WAS CHOSEN, read before the first is clicked:
// its own source, its own reason, and its own disabled Play.
setTimeout(() => {
  window.__secondShown =
    document.getElementById('routineSource').textContent.includes('// discord') ? 1 : 0
  window.__secondWhy =
    document.getElementById('routineWhy').textContent.includes('switch nothing') ? 1 : 0
  window.__secondOff = document.getElementById('routinePlayBtn').disabled ? 1 : 0
}, 2600)
`

func TestTheSavedRoutinesAreDrawnAndPlayIsOfferedForTheOneThatCanRun(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, libraryHarness)
	n := func(key string) int {
		m := regexp.MustCompile(key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	// BOTH ROUTINES ARE THERE. A library that drew only the runnable ones would hide the
	// one somebody needs to do something about.
	if got := n("cards"); got != 2 {
		t.Errorf("the library drew %d of 2 routine(s): %q", got, title)
	}
	// AND CHOOSING ONE OPENS IT.
	if n("chosen") != 1 {
		t.Errorf("clicking a routine did not open it: %q", title)
	}
	// ── AND IT IS THE ONE THEY CLICKED, which is the whole of this ──
	//
	// Both rows are called `settings`. The page kept the NAME of the chosen routine and
	// every lookup from it answered with the first row that matched, so clicking the
	// second card showed the first one's source, the first one's sentence and the first
	// one's Play button — and pressing Play performed the first one's route. Handing the
	// child a full identity was already right and was happening after the wrong routine
	// had been chosen.
	if n("secondshown") != 1 {
		t.Errorf("clicking the second of two routines of one name showed the "+
			"other one's saved Marco: %q", title)
	}
	// AND PLAY IS NOT OFFERED FOR THE ONE THAT CANNOT PRESS ANYTHING, with ITS reason
	// beside it. A live button here answers a press with a refusal, which is a window
	// telling somebody off for pressing what it offered them.
	if n("secondoff") != 1 {
		t.Errorf("Play is offered for a routine where every press would refuse: %q",
			title)
	}
	if n("secondwhy") != 1 {
		t.Errorf("nothing says why it cannot be played, or it says the other "+
			"routine's reason: %q", title)
	}
	// AND PRESSING PLAY PERFORMS EXACTLY THE ONE ON SCREEN, by identity. `9` is the
	// wrong routine, `0` is none at all.
	if got := n("played"); got != 1 {
		t.Errorf("Play performed %s: %q", map[int]string{
			0: "nothing", 9: "a different routine from the one chosen",
		}[got], title)
	}
	if n("shown") != 1 {
		t.Errorf("choosing the first routine afterwards did not show its own "+
			"saved Marco: %q", title)
	}
	// AND THE LIBRARY SAYS WHERE IT IS, and names the one somebody's older routines are
	// sitting in. Silence there is a person reading an empty list and concluding their
	// work is gone.
	if n("where") != 1 {
		t.Errorf("the window does not say which library it is reading: %q", title)
	}
	if n("elsewhere") != 1 {
		t.Errorf("a library full of routines beside this window is passed over in "+
			"silence: %q", title)
	}
}

// ── PRESSING SAVE OVER A NAME THAT IS TAKEN ───────────────────────────────────

// ORDINARY SAVE CANNOT REPLACE ANYTHING, AND THE CHOICE IS ONE SOMEBODY PRESSES.
//
// # Why this is a browser too
//
// Because the backend refusing a collision is half of it. The other half is that the window's
// ordinary Save button asks for a save and NOTHING else: a page that looked first and then called
// the replacement for you would pass every test in `replace_test.go` while quietly doing the one
// thing those tests exist to prevent. The two halves look identical from either side alone.
//
// So this renders the real page over a backend whose Save refuses, and counts what the page
// called. Save is pressed first — `ReplaceDraft` must not have been reached — and then the Replace
// button a person would have to find, which must carry the identity the backend named.
const takenHarness = `
window.__saved = 0
window.__replaced = ''
window.__as = ''
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
    History: () => Promise.resolve([]),
    Applications: () => Promise.resolve(['arkascended']),
    Pictures: () => Promise.resolve([]),
    Picture: () => Promise.resolve(''),
    Learning: () => Promise.resolve({active: false}),
    Watch: () => Promise.resolve({act: '', active: false, transcript: null}),
    Unrunnable: () => Promise.resolve([]),
    // THE WINDOW A RUN WOULD PRESS IN. Studio does not Play without one, so a
    // harness that does not answer this is a page whose Play button is dead.
    PlaybackTarget: () => Promise.resolve({chosen: '4660:77',
      windows: [{id: '4660:77', label: 'a window',
        says: 'Play brings a window to the front and presses there.'}]}),
    Routines: () => Promise.resolve([]),
    Library: () => Promise.resolve({dir: '/tmp/routes'}),
    // WHICH PARTS OF THIS WINDOW ARE OFFERED, AND WHAT IT EXPLAINS.
    // Both are read once at startup; a stub without them is a page
    // whose rail never paints. See help.go.
    Features: () => Promise.resolve({acts: true, scenes: true,
      views: ['routines', 'acts', 'scenes', 'history', 'help']}),
    // THE RAIL'S BUILD STAMP, which is its own read now rather than two
    // fields of the Scenes view's poll. See buildstatus.go.
    BuildStatus: () => Promise.resolve({build: 'abc1234'}),
    Help: () => Promise.resolve({sections: [{key: 'editing',
      title: 'Editing what a routine does', lines: ['nothing is written yet']}]}),
    Recording: () => Promise.resolve({name: 'ark graphics', active: false, review: true,
      application: 'arkascended', stopped_by: 'the Stop button', pressable: 1,
      stop_key: 'F12', steps: ['bring arkascended to the front',
      'left-click at 842, 274'], said: '2 step(s), 1 of which presses something.'}),
    // A BACKEND THAT REFUSES, which is what a taken name does.
    SaveDraft: () => { window.__saved++
      return Promise.resolve('there is already a routine saved there: "ark graphics", ' +
        'kept on 2026-10-24 04:00. Give this one another name, or replace that ' +
        'routine on purpose.') },
    Collision: () => Promise.resolve({exists: true, id: 'arkascended/focus/ark-graphics',
      name: 'ark graphics', saved_at: '2026-10-24 04:00',
      says: 'there is already a routine saved there'}),
    SaveDraftAs: (n) => { window.__as = n; return Promise.resolve('') },
    ReplaceDraft: (id) => { window.__replaced = id; return Promise.resolve('') },
  },
}}
// PRESS SAVE, THE WAY A PERSON WOULD.
setTimeout(() => {
  const b = document.getElementById('capSaveBtn')
  if (b && !b.disabled) { b.click() }
}, 1500)
// AND THEN, ONLY AFTER THE WINDOW HAS OFFERED IT, PRESS REPLACE.
setTimeout(() => {
  window.__takenShown = document.getElementById('capTaken') &&
    !document.getElementById('capTaken').classList.contains('hide') ? 1 : 0
  window.__replacedBySave = window.__replaced
  const r = document.getElementById('capReplaceBtn')
  if (r) { r.click() }
}, 3000)
setTimeout(() => {
  document.title = 'saved=' + window.__saved +
    ' taken=' + (window.__takenShown || 0) +
    ' bysave=' + (window.__replacedBySave === '' ? 'none' : window.__replacedBySave) +
    ' replaced=' + (window.__replaced === '' ? 'none' : window.__replaced) +
    ' as=' + (window.__as === '' ? 'none' : window.__as)
}, 4500)
`

func TestOrdinarySaveNeverReplacesAndTheChoiceIsPressed(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, takenHarness)
	field := func(key string) string {
		m := regexp.MustCompile(key + `=(\S+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		return m[1]
	}
	// SAVE WAS ASKED FOR, so the button is wired to something.
	if field("saved") == "0" {
		t.Fatalf("pressing Save called nothing: %q", title)
	}
	// AND IT DID NOT REPLACE ANYTHING. The whole of this test.
	if got := field("bysave"); got != "none" {
		t.Errorf("pressing the ordinary Save button replaced %q — a replacement "+
			"must be something a person chooses: %q", got, title)
	}
	// AND SAVE AS WAS NOT CALLED FOR THEM EITHER.
	if got := field("as"); got != "none" {
		t.Errorf("pressing Save renamed the routine to %q on its own: %q", got, title)
	}
	// THE CHOICE IS SHOWN, because a refusal nobody can act on is a dead end.
	if field("taken") != "1" {
		t.Errorf("the window refused the save and offered neither way on: %q", title)
	}
	// AND PRESSING REPLACE REPLACES THE ROUTINE THE BACKEND NAMED, not the name the
	// page happens to be showing.
	if got := field("replaced"); got != "arkascended/focus/ark-graphics" {
		t.Errorf("Replace was called with %q, want the identity the backend "+
			"named: %q", got, title)
	}
}

// historyHarness is one run of each kind in the history, so the page's two row shapes can be
// compared side by side — which is the whole point: the rows sit next to each other.
const historyHarness = `
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
    // harness that does not answer this is a page whose Play button is dead.
    PlaybackTarget: () => Promise.resolve({chosen: '4660:77',
      windows: [{id: '4660:77', label: 'a window',
        says: 'Play brings a window to the front and presses there.'}]}),
    Routines: () => Promise.resolve([]),
    Applications: () => Promise.resolve([]),
    Pictures: () => Promise.resolve([]),
    Picture: () => Promise.resolve(''),
    Learning: () => Promise.resolve({active: false}),
    Watch: () => Promise.resolve({act: '', active: false, transcript: null}),
    Recording: () => Promise.resolve({active: false}),
    Unrunnable: () => Promise.resolve([]),
    Library: () => Promise.resolve({dir: 'C:/x/routes'}),
    // WHICH PARTS OF THIS WINDOW ARE OFFERED, AND WHAT IT EXPLAINS.
    // Both are read once at startup; a stub without them is a page
    // whose rail never paints. See help.go.
    Features: () => Promise.resolve({acts: true, scenes: true,
      views: ['routines', 'acts', 'scenes', 'history', 'help']}),
    // THE RAIL'S BUILD STAMP, which is its own read now rather than two
    // fields of the Scenes view's poll. See buildstatus.go.
    BuildStatus: () => Promise.resolve({build: 'abc1234'}),
    Help: () => Promise.resolve({sections: [{key: 'editing',
      title: 'Editing what a routine does', lines: ['nothing is written yet']}]}),
    History: () => Promise.resolve([
      {reference: 'run-a', kind: 'routine', routine: 'ark-graphics',
       outcome: 'performed', when: '2026-10-24 16:05', sent: 4, verified: 0,
       said: 'Marco performed "ark graphics" — every press happened. Nothing here ' +
             'checked that the application did what you wanted.'},
      {reference: 'run-b', act: 'the audio page', outcome: 'verified',
       when: '2026-10-24 15:00', live: true, sent: 2, verified: 2,
       said: 'It worked. Marco performed "the audio page" and the screen changed.'},
      {reference: 'run-c', kind: 'routine', routine: 'ark-graphics',
       outcome: 'started', when: '2026-10-24 14:00', unfinished: true,
       said: 'Marco started "ark graphics" and never said how it ended.'},
      {reference: 'run-d', act: 'the audio page', outcome: 'verified',
       when: '2026-10-24 13:00', live: false, sent: 0, verified: 0,
       said: 'It worked. Marco performed "the audio page" and the screen changed.'},
    ]),
  },
}}
setTimeout(() => {
  const cards = document.querySelectorAll('#histList .card')
  const textOf = (i) => cards.length > i ? cards[i].textContent : ''
  document.title = 'cards=' + cards.length +
    ' routineasked=' + (textOf(0).includes('4 input actions asked for') ? 1 : 0) +
    ' routinenochecks=' + (textOf(0).includes('nothing checked afterwards') ? 1 : 0) +
    ' routinezero=' + (textOf(0).includes('0 checked') ? 1 : 0) +
    ' routinepresses=' + (textOf(0).includes('presses sent') ? 1 : 0) +
    ' routinedisabled=' + (textOf(0).includes('input disabled') ? 1 : 0) +
    ' routinetag=' + (textOf(0).includes('ROUTINE') ? 1 : 0) +
    ' actchecked=' + (textOf(1).includes('2 checked') ? 1 : 0) +
    ' actpresses=' + (textOf(1).includes('2 presses sent') ? 1 : 0) +
    ' actasked=' + (textOf(1).includes('asked for') ? 1 : 0) +
    ' actlive=' + (textOf(1).includes('LIVE') ? 1 : 0) +
    ' killedzero=' + (textOf(2).includes('0 input action') ? 1 : 0) +
    ' killeddisabled=' + (textOf(2).includes('input disabled') ? 1 : 0) +
    ' killedunknown=' + (textOf(2).includes('never written down') ? 1 : 0) +
    ' dryrundisabled=' + (textOf(3).includes('input disabled') ? 1 : 0)
}, 4500)
`

// A ROUTINE'S ROW DOES NOT SAY IT CHECKED NOTHING, AND AN ACT'S STILL SAYS WHAT IT CHECKED.
//
// Rendered, because this is the one place the rule ADR-253 is about could be broken on screen
// while every backend test passed. The row printed `<n> presses sent · <v> checked` for every
// record; a routine writes no verification counts at all, so `v` was zero and the row read
// "0 checked" — not a count but the claim that it checked and found nothing, sitting directly
// beside an Act that really did check two things. Three zeroes in the file was the thing the
// record was careful to avoid; one zero on the screen says the same thing to the person.
//
// Both rows are asserted, in both directions, because the fix is a branch and a branch can be
// wrong either way round.
func TestARoutinesRowDoesNotClaimToHaveCheckedAnything(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, historyHarness)
	n := func(key string) int {
		m := regexp.MustCompile(key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	if got := n("cards"); got != 4 {
		t.Fatalf("the history drew %d of 4 runs: %q", got, title)
	}
	// ── THE ROUTINE'S ROW ──
	if n("routinezero") == 1 {
		t.Errorf("a routine's row says `0 checked`, which is a claim that it "+
			"checked: %q", title)
	}
	if n("routinenochecks") != 1 {
		t.Errorf("a routine's row does not say that nothing was checked "+
			"afterwards: %q", title)
	}
	if n("routineasked") != 1 {
		t.Errorf("a routine's row does not say how many input actions it asked "+
			"for: %q", title)
	}
	if n("routinepresses") == 1 {
		t.Errorf("a routine's row counts `presses sent`, which is the supervised "+
			"record's measure and not this one's: %q", title)
	}
	// ── AND THE ACT'S, UNCHANGED ──
	if n("actchecked") != 1 || n("actpresses") != 1 {
		t.Errorf("an Act's row no longer says what it sent and what it checked: %q",
			title)
	}
	if n("actasked") == 1 {
		t.Errorf("an Act's row was given a routine's wording: %q", title)
	}
}

// A ROW SAYS NOTHING ABOUT TOUCHING THE MACHINE THAT ITS RECORD DID NOT SAY.
//
// Rendered, because both halves of this were invisible to every backend test and plain on screen.
//
//	`live` is a field only a SUPERVISED record carries. A routine's record states no
//	liveness — `marco do` has no "write it down instead" mode to distinguish — so the
//	field arrived absent, read as a falsy bool, and the row labelled a routine that had
//	just typed somebody's password `input disabled`.
//
//	`sent` is absent in the provisional record a killed `marco.exe` leaves, and that
//	attempt may already have sent input. Read as an int it rendered `0 input actions
//	asked for`: the same false claim, about the one run a person is most likely to be
//	asking somebody else about.
//
// Four rows, because each claim can be wrong in both directions: a routine that really pressed
// things, an Act with input enabled, an interrupted routine that counted nothing, and an Act that
// genuinely had input disabled — which must still say so.
func TestARowDoesNotSayInputWasDisabledWhenNothingSaidSo(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, historyHarness)
	n := func(key string) int {
		m := regexp.MustCompile(key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	if got := n("cards"); got != 4 {
		t.Fatalf("the history drew %d of 4 runs: %q", got, title)
	}
	// ── A ROUTINE THAT PRESSED FOUR THINGS ──
	if n("routinedisabled") == 1 {
		t.Errorf("a routine's row says `input disabled` about a run that asked "+
			"for four input actions: %q", title)
	}
	if n("routinetag") != 1 {
		t.Errorf("a routine's row says nothing about which kind of run it was, so "+
			"its numbers read as an Act's with a field missing: %q", title)
	}
	// ── AN INTERRUPTED ROUTINE, WHICH MAY ALREADY HAVE SENT INPUT ──
	if n("killedzero") == 1 {
		t.Errorf("an interrupted attempt's row counts `0 input actions`, and no "+
			"count was ever written down: %q", title)
	}
	if n("killeddisabled") == 1 {
		t.Errorf("an interrupted attempt's row says `input disabled`: %q", title)
	}
	if n("killedunknown") != 1 {
		t.Errorf("an interrupted attempt's row does not say that the count was "+
			"never written down: %q", title)
	}
	// ── AND THE TWO ACTS, WHICH DO STATE IT, IN BOTH DIRECTIONS ──
	if n("actlive") != 1 {
		t.Errorf("an Act's run with input enabled no longer says LIVE: %q", title)
	}
	if n("dryrundisabled") != 1 {
		t.Errorf("an Act's run that really had input disabled no longer says so, "+
			"which is the claim being made absent rather than false: %q", title)
	}
}
