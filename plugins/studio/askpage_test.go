package main

// THE ASK BOX, RENDERED.
//
// Three things a Go test of `SuggestRoutine` cannot see, because they are what the window DOES
// with the answer:
//
//   - the target application beside the request, read off the answer rather than off what the
//     page sent, so the two cannot disagree about which application it was read against;
//   - which arm answered and what part the configured resolver had, because two of the three
//     arms need no model and a window that never reached Ollama would look exactly like one that
//     did;
//   - that finding a routine SELECTS it and nothing else — the thing that performs one is the
//     Play button, pressed by a person.
//
// See [[ADR-265-say-what-you-want-and-studio-offers-the-routine]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// askHarnessWith is a window holding one routine, with the ask box answered as given.
//
// The routine is the same shape the listing produces, so the card this selects is a real card
// with a real identity rather than a div the harness drew.
func askHarnessWith(suggestion string) string {
	return `
window.__ran = 0
window.go = {main: {
  Backend: {Loaded: () => Promise.resolve()},
  Acts: {
    SuggestTargets: () => Promise.resolve([
      {app: 'arkascended', open: true, context: 2},
      {app: 'notepad', open: false, context: 1},
    ]),
    SuggestRoutine: (text, target) => {
      window.__asked = text + '|' + target
      return Promise.resolve(` + suggestion + `)
    },
    Demonstrations: () => Promise.resolve([]),
    Saved: () => Promise.resolve([]),
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
    Ready: () => Promise.resolve({build: 'abc1234', problems: [], limits: 'nothing'}),
    Recording: () => Promise.resolve({active: false}),
    // IF THE PAGE EVER RUNS ANYTHING OFF A SUGGESTION, THIS COUNTS IT.
    PlayRoutine: () => { window.__ran++; return Promise.resolve('') },
  },
}}
setTimeout(() => {
  // A TARGET IS CHOSEN, as a person would: the box offers one and this picks it.
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
    ' notrunning=' + (sel0().includes('not running') ? 1 : 0) +
    ' saidreply=' + (said.includes('Opening it') ? 1 : 0) +
    ' saidtarget=' + (who.includes('in arkascended') ? 1 : 0) +
    ' offered=' + (who.includes('3 routine(s) offered') ? 1 : 0) +
    ' advisor=' + (who.includes('advisor') ? 1 : 0) +
    ' fallback=' + (who.includes('fallback') ? 1 : 0) +
    ' unavailable=' + (who.includes('did not answer') ? 1 : 0) +
    ' unconfigured=' + (who.includes('no resolver is configured') ? 1 : 0) +
    ' selected=' + (card && card.classList.contains('on') ? 1 : 0) +
    ' ran=' + window.__ran +
    ' asked=' + (window.__asked === 'make the pictures nicer|arkascended' ? 1 : 0)
}, 4500)
function sel0(){
  const sel = document.getElementById('askTarget')
  return Array.from(sel.options).map(o => o.textContent).join(' ')
}
`
}

// anOllamaSuggestion is the resolver having answered, naming the routine's whole identity.
const anOllamaSuggestion = `{kind: 'run', id: 'arkascended/focus/ark-graphics',
  name: 'ark graphics', scope: 'app-anywhere', reply: 'Opening it.',
  target: 'arkascended', offered: 3, source: 'advisor',
  resolver: 'the configured resolver answered this: D:\\\\x\\\\llama.exe'}`

// aSilentResolver is the one that used to be invisible: configured, asked, and silent, which looks
// exactly like never having configured one.
const aSilentResolver = `{kind: 'clarify', id: '', reply: 'Did you mean ark graphics?',
  target: 'arkascended', offered: 3, source: 'fallback',
  resolver: 'the configured resolver was asked and did not answer, so this is a ` +
	`deterministic match: D:\\\\x\\\\llama.exe \u2014 check that the model server is running'}`

// noResolverAtAll is a configuration fact and not a silence, and is said as one.
const noResolverAtAll = `{kind: 'clarify', id: '', reply: 'Did you mean ark graphics?',
  target: 'arkascended', offered: 3, source: 'fallback',
  resolver: 'no resolver is configured, so only an exact name or a near match can be found'}`

// A RESOLVER'S ANSWER SELECTS THE ROUTINE AND RUNS NOTHING.
func TestTheAskBoxSelectsWhatTheResolverNamedAndRunsNothing(t *testing.T) {
	title, n := aRenderedAskPage(t, anOllamaSuggestion)

	// THE REQUEST AND THE TARGET REACHED THE BACKEND TOGETHER.
	if n("asked") != 1 {
		t.Errorf("the request and the chosen target did not arrive together: %q", title)
	}
	// AND BOTH TARGETS ARE OFFERED, with "not running" as a hint rather than as a filter:
	// an application that is not running is still a target somebody may choose, because
	// the choice is theirs and Play raises it.
	if n("targets") != 3 {
		t.Errorf("the target box offers %d option(s), want the two applications and "+
			"none: %q", n("targets"), title)
	}
	if n("notrunning") != 1 {
		t.Errorf("an application with no window is not marked, so somebody cannot see "+
			"that the one their in-place command needs is not there: %q", title)
	}
	// THE ROUTINE IS SELECTED.
	if n("selected") != 1 {
		t.Errorf("the routine the resolver named is not selected: %q", title)
	}
	// AND NOTHING RAN. The whole of what makes this safe to press.
	if n("ran") != 0 {
		t.Errorf("finding a routine performed it %d time(s), and the only thing that "+
			"may is a person pressing Play: %q", n("ran"), title)
	}
	// AND THE WINDOW SAYS WHAT ANSWERED.
	if n("saidreply") != 1 {
		t.Errorf("the resolver's own sentence is not shown: %q", title)
	}
	if n("saidtarget") != 1 {
		t.Errorf("the target the request was read against is not shown beside it: %q",
			title)
	}
	if n("offered") != 1 {
		t.Errorf("how many routines were eligible is not shown, which is the "+
			"denominator for the suggestion: %q", title)
	}
	if n("advisor") != 1 {
		t.Errorf("the window does not say the resolver answered this: %q", title)
	}
}

// AND A CONFIGURED RESOLVER THAT DID NOT ANSWER IS SAID SO, VISIBLY.
//
// The state that used to be indistinguishable from never having configured one — which is how a
// feature comes to look as though it reaches a model it has never once reached.
func TestTheAskBoxSaysWhenTheResolverWasSilent(t *testing.T) {
	title, n := aRenderedAskPage(t, aSilentResolver)
	if n("unavailable") != 1 {
		t.Errorf("a configured resolver that did not answer is not reported: %q", title)
	}
	if n("fallback") != 1 {
		t.Errorf("the deterministic arm is not named as what answered: %q", title)
	}
	if n("advisor") != 0 {
		t.Errorf("a silent resolver was credited with the answer: %q", title)
	}
	// AND A NEAR MISS SELECTS NOTHING. A question with the answer already filled in is
	// not a question.
	if n("selected") != 0 {
		t.Errorf("a near miss selected a routine rather than asking: %q", title)
	}
	if n("ran") != 0 {
		t.Errorf("a near miss ran something: %q", title)
	}
}

// AND NO RESOLVER AT ALL IS A CONFIGURATION FACT, NOT A FAULT.
func TestTheAskBoxSaysWhenNoResolverIsConfigured(t *testing.T) {
	title, n := aRenderedAskPage(t, noResolverAtAll)
	if n("unconfigured") != 1 {
		t.Errorf("a window with no resolver configured does not say so: %q", title)
	}
	if n("unavailable") != 0 {
		t.Errorf("having configured nothing is reported as a resolver that did not "+
			"answer, which would tell somebody their resolver is broken when they "+
			"have not got one: %q", title)
	}
}

// aRenderedAskPage renders the real page with the ask box answered as given.
func aRenderedAskPage(t *testing.T, suggestion string) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, askHarnessWith(suggestion))
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
