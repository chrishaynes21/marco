package main

// THE ASK BOX, RENDERED, NOW THAT DIRECTOR ANSWERS IT.
//
// Four things a Go test of the client call cannot see, because they are what the window DOES with
// what comes back:
//
//   - the context it was read against AND HOW THAT WAS KNOWN, off the answer rather than off what
//     the page sent, so the two cannot disagree — and the basis is the part worth showing, since
//     a window has the foreground whenever somebody types into it;
//   - which arm answered and what part the configured resolver had, because two of the three need
//     no model and a window that never reached it would look exactly like one that did;
//   - a Director that could not answer, said as WHICH of the three it was and with no local
//     fallback;
//   - and that finding a routine SELECTS it and nothing else.
//
// See [[ADR-267-director-owns-route-resolution]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// askHarnessWith is a window holding one routine, with Director answering as given.
func askHarnessWith(answer string) string {
	return `
window.__ran = 0
window.go = {main: {
  Backend: {
    Loaded: () => Promise.resolve(),
    SuggestRoutine: (text, target) => {
      window.__asked = text + '|' + target
      return Promise.resolve(` + answer + `)
    },
  },
  Acts: {
    SuggestTargets: () => Promise.resolve([
      {app: 'arkascended', open: true, context: 2},
      {app: 'notepad', open: false, context: 1},
    ]),
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
    // THE PLAY ROW'S READ, which the page makes for every routine it draws.
    // A stub without it is a page that throws mid-render; no backtick may appear
    // in this comment, because the harness around it is a Go raw string.
    PlayNeeds: (id) => Promise.resolve({id: id, fields: [], needs: false}),
    ReviewPlay: (id) => Promise.resolve({id: id, why: 'not what this asks about'}),
    ConfirmPlay: () => Promise.resolve(''),
    CancelPlay: () => {},
    Routines: () => Promise.resolve([
      {id: 'arkascended/focus/ark-graphics', name: 'ark graphics',
       slug: 'ark-graphics', app: 'arkascended', scope: 'app-anywhere',
       contract: 'guarded', runnable: true, generation: 'current',
       saved_at: '2026-10-28 09:00', rebuildable: true, offer: true},
    ]),
    Applications: () => Promise.resolve([]),
    Pictures: () => Promise.resolve([]),
    Picture: () => Promise.resolve(''),
    Learning: () => Promise.resolve({active: false}),
    Watch: () => Promise.resolve({act: '', active: false, transcript: null}),
    History: () => Promise.resolve([]),
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
    Ready: () => Promise.resolve({build: 'abc1234', problems: [], limits: 'nothing'}),
    Recording: () => Promise.resolve({active: false}),
    // IF THE PAGE EVER RUNS ANYTHING OFF AN ANSWER, THIS COUNTS IT.
    PlayRoutine: () => { window.__ran++; return Promise.resolve('') },
  },
}}
setTimeout(() => {
  const sel = document.getElementById('askTarget')
  sel.value = 'arkascended'
  document.getElementById('askBox').value = 'make the pictures nicer'
  document.getElementById('askBtn').click()
}, 1200)
setTimeout(() => {
  const said = document.getElementById('askSaid').textContent
  const who = document.getElementById('askWho').textContent
  const card = document.getElementById('routine-arkascended-focus-ark-graphics')
  document.title =
    'targets=' + document.getElementById('askTarget').options.length +
    ' notrunning=' + (labels().includes('not running') ? 1 : 0) +
    ' saidreply=' + (said.includes('Opening it') ? 1 : 0) +
    ' saidbasis=' + (who.includes('in front now') ? 1 : 0) +
    ' saidstated=' + (who.includes('which you named') ? 1 : 0) +
    ' saidnocontext=' + (who.includes('nothing is known to be in front') ? 1 : 0) +
    ' offered=' + (who.includes('3 routine(s) offered') ? 1 : 0) +
    ' advisor=' + (who.includes('advisor') ? 1 : 0) +
    ' fallback=' + (who.includes('fallback') ? 1 : 0) +
    ' unavailable=' + (who.includes('did not answer') ? 1 : 0) +
    ' notrunningdirector=' + (said.includes('Director is not running') ? 1 : 0) +
    ' otherprotocol=' + (said.includes('different protocol') ? 1 : 0) +
    ' whoblank=' + (who === '' ? 1 : 0) +
    ' selected=' + (card && card.classList.contains('on') ? 1 : 0) +
    ' ran=' + window.__ran +
    ' asked=' + (window.__asked === 'make the pictures nicer|arkascended' ? 1 : 0)
}, 4500)
function labels(){
  const sel = document.getElementById('askTarget')
  return Array.from(sel.options).map(o => o.textContent).join(' ')
}
`
}

// aDirectorAnswer is Director having resolved it, with the context it read against.
const aDirectorAnswer = `{answer: {kind: 'run', id: 'arkascended/focus/ark-graphics',
  app: 'arkascended', slug: 'ark-graphics', focus: true, name: 'ark graphics',
  scope: 'app-anywhere', reply: 'Opening it.', offered: 3, source: 'advisor',
  resolver: 'the configured resolver answered this: D:\\\\x\\\\llama.exe',
  library: 'C:/x/routes',
  context: {app: 'arkascended', basis: 'stated',
            says: 'read against arkascended, which you named'}}}`

// aSilentResolver is configured, asked, and with nothing to say — the state that used to look
// exactly like never having configured one.
const aSilentResolver = `{answer: {kind: 'clarify', id: '',
  reply: 'Did you mean ark graphics?', offered: 3, source: 'fallback',
  resolver: 'the configured resolver was asked and did not answer, so this is a ` +
	`deterministic match: D:\\\\x\\\\llama.exe', library: 'C:/x/routes',
  context: {app: 'arkascended', basis: 'observed',
            says: 'read against arkascended, which is in front now'}}}`

// anUncorroboratedContext is a Marco window holding the foreground: nothing is known to be in
// front, so no in-place routine was offered at all.
const anUncorroboratedContext = `{answer: {kind: 'clarify', id: '',
  reply: 'Did you mean ark graphics?', offered: 2, source: 'fallback',
  resolver: 'a resolver is configured and was NOT asked', library: 'C:/x/routes',
  context: {basis: 'last-external',
            says: 'nothing is known to be in front, so no in-place routine is offered.'}}}`

// noDirectorAtAll and anotherProtocol are two of the three ways there is no answer, and they must
// not read as each other: one sends somebody to start a service, the other to the version they
// are running.
const noDirectorAtAll = `{why: 'the Director is not running, so nothing can resolve this ` +
	`request — cards and Play still work', asked: 'make the pictures nicer'}`

const anotherProtocol = `{why: 'the Director is running and speaks a different protocol from ` +
	`this window, so nothing can resolve this request: protocol 11 vs 10',
  asked: 'make the pictures nicer'}`

// A RESOLVED ANSWER SELECTS THE ROUTINE AND RUNS NOTHING.
func TestTheAskBoxSelectsWhatDirectorNamedAndRunsNothing(t *testing.T) {
	title, n := aRenderedAskPage(t, aDirectorAnswer)

	// THE REQUEST AND THE TARGET REACHED DIRECTOR TOGETHER.
	if n("asked") != 1 {
		t.Errorf("the request and the chosen target did not arrive together: %q", title)
	}
	if n("targets") != 3 {
		t.Errorf("the target box offers %d option(s), want the two applications and "+
			"none: %q", n("targets"), title)
	}
	if n("notrunning") != 1 {
		t.Errorf("an application with no window is not marked: %q", title)
	}
	if n("selected") != 1 {
		t.Errorf("the routine Director named is not selected: %q", title)
	}
	// AND NOTHING RAN. The whole of what makes this safe to press.
	if n("ran") != 0 {
		t.Errorf("resolving a request performed it %d time(s): %q", n("ran"), title)
	}
	if n("saidreply") != 1 {
		t.Errorf("Director's own sentence is not shown: %q", title)
	}
	// AND HOW THE CONTEXT WAS KNOWN, not just which application it was.
	if n("saidstated") != 1 {
		t.Errorf("the basis the request was read under is not shown: %q", title)
	}
	if n("offered") != 1 || n("advisor") != 1 {
		t.Errorf("the denominator and the arm that answered are not both shown: %q", title)
	}
}

// AND A CONFIGURED RESOLVER THAT DID NOT ANSWER IS SAID SO, with the basis beside it.
func TestTheAskBoxSaysWhenTheResolverWasSilent(t *testing.T) {
	title, n := aRenderedAskPage(t, aSilentResolver)
	if n("unavailable") != 1 || n("fallback") != 1 {
		t.Errorf("a silent resolver is not reported as one: %q", title)
	}
	if n("advisor") != 0 {
		t.Errorf("a silent resolver was credited with the answer: %q", title)
	}
	if n("saidbasis") != 1 {
		t.Errorf("a corroborated foreground is not shown as current: %q", title)
	}
	if n("selected") != 0 || n("ran") != 0 {
		t.Errorf("a near miss selected or ran something: %q", title)
	}
}

// AND A CONTEXT THAT COULD NOT BE CORROBORATED SAYS SO, which is the case a person needs to
// understand: their in-place routines are missing from the offer and this is why.
func TestTheAskBoxSaysWhenNothingIsKnownToBeInFront(t *testing.T) {
	title, n := aRenderedAskPage(t, anUncorroboratedContext)
	if n("saidnocontext") != 1 {
		t.Errorf("an uncorroborated context is not explained: %q", title)
	}
	if n("selected") != 0 {
		t.Errorf("something was selected from an uncorroborated context: %q", title)
	}
}

// ── AND A DIRECTOR THAT COULD NOT ANSWER, SAID AS WHICH ONE IT WAS ───────────

func TestTheAskBoxSaysDirectorIsNotRunning(t *testing.T) {
	title, n := aRenderedAskPage(t, noDirectorAtAll)
	if n("notrunningdirector") != 1 {
		t.Errorf("a missing Director is not reported as one: %q", title)
	}
	// AND NOTHING ELSE IS CLAIMED. No arm answered, no denominator, no resolver
	// sentence: there is no answer, and a page that drew one would be drawing a zero
	// answer as "nothing matched".
	if n("whoblank") != 1 {
		t.Errorf("the window described an answer it did not get: %q", title)
	}
	if n("selected") != 0 || n("ran") != 0 {
		t.Errorf("a missing Director selected or ran something: %q", title)
	}
}

func TestTheAskBoxTellsAProtocolMismatchFromAMissingService(t *testing.T) {
	title, n := aRenderedAskPage(t, anotherProtocol)
	if n("otherprotocol") != 1 {
		t.Errorf("a protocol mismatch is not reported as one: %q", title)
	}
	// AND IT IS NOT REPORTED AS A SERVICE THAT IS NOT RUNNING, which would send
	// somebody to start one that is already there.
	if n("notrunningdirector") != 0 {
		t.Errorf("a running Director was reported as not running: %q", title)
	}
	if n("selected") != 0 || n("ran") != 0 {
		t.Errorf("a protocol mismatch selected or ran something: %q", title)
	}
}

// aRenderedAskPage renders the real page with Director answering as given.
func aRenderedAskPage(t *testing.T, answer string) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, askHarnessWith(answer))
	if !strings.Contains(title, "targets=") {
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
