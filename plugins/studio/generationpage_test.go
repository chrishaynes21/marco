package main

// THE PAGE SAYS WHICH GENERATION WROTE A ROUTINE, AND OFFERS A REBUILT VERSION ONLY WHERE ONE CAN
// BE EXPLAINED.
//
// Rendered rather than grepped, for the reason `redraw_test.go` gives at length: a source guard
// passes as long as the field is NAMED somewhere in the file, so a page that printed every
// sentence unconditionally, or printed `undefined`, or left the Review button live for a routine
// nothing can be rebuilt for, would keep every such guard green. The branch is the thing under
// test and only something that runs the page can take it.
//
// Four things are checked and three of them are refusals — which is the shape of this whole
// slice. The one positive claim is that a review shows BOTH sources, because a review of one is
// not a comparison.
//
// See [[ADR-262-a-routine-states-which-generation-wrote-it]].

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// generationHarness is a window with four routines: one written by today's generator, one that
// states nothing, one from a later build, and one whose demonstration is gone.
//
// Each is CHOSEN in turn and the panel read after each, so a page that rendered the first card's
// answer for whatever was selected fails rather than passing on the first assertion.
//
// `ReviewRebuilt` is stubbed because what is under test here is the page: the backend's own
// refusals are held in `internal/orchestrator`, and the two meeting for real is the desktop
// scenario in `plugins/e2e`. The stub answers like the backend does — a review for the unstated
// routine, and a `why` for anything it should never have been asked about.
const generationHarness = `
window.go = {main: {
  Backend: {Loaded: () => Promise.resolve()},
  Acts: {
    Demonstrations: () => Promise.resolve([]),
    Saved: () => Promise.resolve([]),
    Applications: () => Promise.resolve([]),
    Pictures: () => Promise.resolve([]),
    Picture: () => Promise.resolve(''),
    Learning: () => Promise.resolve({active: false}),
    Watch: () => Promise.resolve({act: '', active: false, transcript: null}),
    History: () => Promise.resolve([]),
    Unrunnable: () => Promise.resolve([]),
    Library: () => Promise.resolve({dir: 'C:/x/routes'}),
    Recording: () => Promise.resolve({active: false}),
    CancelReview: (token) => {
      window.cancelledWith = token
      window.cancelled = (window.cancelled || 0) + 1
    },
    ReplaceWithRebuilt: (token) => {
      window.replacedWith = token
      window.replaced = (window.replaced || 0) + 1
      return Promise.resolve('')
    },
    ReviewRebuilt: (id, kind) => {
      window.reviewed = (window.reviewed || 0) + 1
      window.askedKinds = (window.askedKinds || []).concat(kind)
      if (id !== 'ark/focus/old-thing') {
        return Promise.resolve({id: id, why: 'this should never have been asked'})
      }
      if (kind === 'simplify-typing') {
        return Promise.resolve({id: id, token: 'tok-old-thing-folded',
          name: 'old thing', rebuild: kind, choice: 'Simplify typing pauses',
          caution: 'This folds each run of typing into one step, which removes the ' +
                   'pauses between those keystrokes.',
          was: 'use os.\nthe OldThing is an actor.\n',
          now: '// Generation: 1.\nuse os.\ndo OS\u0027s Type with "hello".\n',
          changes: true, replaceable: true,
          was_generation: 'unstated', now_generation: 'current'})
      }
      return Promise.resolve({id: id, token: 'tok-old-thing-1', name: 'old thing',
        rebuild: 'current', choice: 'Rebuild with current rules', caution: '',
        was: 'use os.\nthe OldThing is an actor.\n',
        now: '// Generation: 1.\nuse os.\nthe OldThing is an actor.\n',
        changes: true, replaceable: true,
        was_generation: 'unstated', now_generation: 'current'})
    },
    Routines: () => Promise.resolve([
      {id: 'ark/focus/ark-graphics', name: 'ark graphics', slug: 'ark-graphics',
       app: 'ark', scope: 'app-anywhere', contract: 'guarded', runnable: true,
       source: 'use os.', generation: 'current', declared_generation: '1',
       rebuildable: true, offer: true,
       wrote: 'This routine states that generation 1 of Marco\'s rules wrote it, ' +
              'which is the generation this build writes.'},
      {id: 'ark/focus/old-thing', name: 'old thing', slug: 'old-thing',
       app: 'ark', scope: 'app-anywhere', contract: 'legacy', runnable: true,
       source: 'use os.', generation: 'unstated',
       rebuildable: true, offer: true,
       wrote: 'This routine does not state which generation rules wrote it. You can ' +
              'review a version rebuilt from its stored recording.'},
      {id: 'ark/focus/from-later', name: 'from later', slug: 'from-later',
       app: 'ark', scope: 'app-anywhere', contract: 'guarded', runnable: true,
       source: 'use os.', generation: 'unknown', declared_generation: '4',
       rebuildable: true, offer: false,
       wrote: 'This routine states a generation this build does not recognise: it ' +
              'states generation 4 and this build writes generation 1. It is left ' +
              'exactly as it is, and runs under the same rules it always has.'},
      {id: 'ark/focus/no-recording', name: 'no recording', slug: 'no-recording',
       app: 'ark', scope: 'app-anywhere', contract: 'legacy', runnable: true,
       source: 'use os.', generation: 'unstated',
       rebuildable: false, offer: false,
       wrote: 'This routine does not state which generation rules wrote it. Its ' +
              'demonstration is no longer stored beside it, so there is nothing to ' +
              'rebuild a version from.'},
    ]),
  },
}}
let says = ['', '', '', ''], live = [0, 0, 0, 0], fold = [0, 0, 0, 0]
const pick = (i) => {
  const cards = document.querySelectorAll('#routineList .card')
  if (cards.length > i) { cards[i].click() }
}
// press clicks a control by id, which is what a person does.
//
// NOT the function behind it. Calling the handler directly proves the backend is asked for a
// choice and says nothing about which button asks for it — and a mutation swapping the second
// button's argument for the first's survived exactly that. The ids are the surface a person and
// an assistive technology both reach.
const press = (id) => { document.getElementById(id).click() }
const look = (i) => {
  says[i] = document.getElementById('routineWrote').textContent
  live[i] = document.getElementById('routineReviewBtn').disabled ? 0 : 1
  fold[i] = document.getElementById('routineSimplifyBtn').disabled ? 0 : 1
}
const reviewShown = () =>
  !document.getElementById('routineReview').classList.contains('hide')
const shown = (id) => {
  const el = document.getElementById(id)
  return el.classList.contains('hide') ? '' : el.textContent
}
let panes = 0, bothgens = 0, afterCancel = 1, replaceLive = 0
let choiceSaid = '', cautionSaid = '', foldedNow = '', plainCaution = 'x'
setTimeout(() => { pick(0) }, 1500)
setTimeout(() => { look(0); pick(1) }, 1800)
setTimeout(() => { look(1); pick(2) }, 2100)
setTimeout(() => { look(2); pick(3) }, 2400)
setTimeout(() => { look(3); pick(1); press('routineReviewBtn') }, 2700)
setTimeout(() => {
  // THE REVIEW: both sources on screen, each labelled with its own reading.
  const was = document.getElementById('routineReviewWas').textContent
  const now = document.getElementById('routineReviewNow').textContent
  panes = (reviewShown() && was.includes('the OldThing is an actor') &&
           now.includes('Generation: 1') && was !== now) ? 1 : 0
  bothgens = (document.getElementById('routineReviewWasGen').textContent === 'unstated' &&
              document.getElementById('routineReviewNowGen').textContent === 'current')
             ? 1 : 0
  replaceLive = document.getElementById('routineReplaceBtn').disabled ? 0 : 1
  // THE ORDINARY CHOICE CARRIES NO CAUTION, which is the control for the one below.
  plainCaution = shown('routineReviewCaution')
  press('routineReviewCancelBtn')
}, 3300)
setTimeout(() => {
  afterCancel = reviewShown() ? 1 : 0
  // AND NOW THE OTHER CHOICE, which has to reach the backend as its own word and come
  // back saying which rules made it and what they change.
  press('routineSimplifyBtn')
}, 3600)
setTimeout(() => {
  choiceSaid = document.getElementById('routineReviewChoice').textContent
  cautionSaid = shown('routineReviewCaution')
  foldedNow = document.getElementById('routineReviewNow').textContent
  press('routineReplaceBtn')
}, 4200)
setTimeout(() => {
  document.title = 'cards=' + document.querySelectorAll('#routineList .card').length +
    ' current=' + (says[0].includes('which is the generation this build writes') ? 1 : 0) +
    ' currentlive=' + live[0] +
    ' unstated=' + (says[1].includes('does not state which generation') ? 1 : 0) +
    ' unstatedlive=' + live[1] +
    ' unknown=' + (says[2].includes('does not recognise') ? 1 : 0) +
    ' unknowndeclared=' + (says[2].includes('generation 4') ? 1 : 0) +
    ' unknownlive=' + live[2] +
    ' norec=' + (says[3].includes('no longer stored beside it') ? 1 : 0) +
    ' noreclive=' + live[3] +
    ' undef=' + (says.some(s => s.includes('undefined')) ? 1 : 0) +
    ' panes=' + panes + ' bothgens=' + bothgens + ' replacelive=' + replaceLive +
    ' aftercancel=' + afterCancel +
    ' cancelled=' + (window.cancelled || 0) +
    ' cancelledtoken=' + (window.cancelledWith === 'tok-old-thing-1' ? 1 : 0) +
    ' replaced=' + (window.replaced || 0) +
    ' replacedtoken=' + (window.replacedWith === 'tok-old-thing-folded' ? 1 : 0) +
    ' foldlive=' + fold[1] + ' foldoff=' + fold[2] +
    ' kinds=' + (window.askedKinds || []).join('+') +
    ' choice=' + (choiceSaid.includes('Simplify typing pauses') ? 1 : 0) +
    ' caution=' + (cautionSaid.includes('removes the') ? 1 : 0) +
    ' plaincaution=' + (plainCaution === '' ? 1 : 0) +
    ' foldednow=' + (foldedNow.includes('OS\u0027s Type') ? 1 : 0) +
    ' reviewed=' + (window.reviewed || 0)
}, 5400)
`

// EACH OF THE FOUR READINGS IS ON THE PAGE IN ITS OWN WORDS.
//
// Four sentences and not one, because the wording is where this goes wrong: an unstated routine
// told it was "written before versioning" is a claim its file does not support, and a later
// build's routine called current would be this build claiming to have written it.
func TestThePageSaysWhichGenerationWroteEachRoutine(t *testing.T) {
	title, n := aRenderedGenerationPage(t)
	for _, c := range []struct {
		key  string
		what string
	}{
		{"current", "a routine this build wrote is not said to be current"},
		{"unstated", "a routine that states nothing is not said to state nothing"},
		{"unknown", "a routine from a later build is not said to be unrecognised"},
		{"norec", "a routine with no demonstration does not say so"},
	} {
		if n(c.key) != 1 {
			t.Errorf("%s: %q", c.what, title)
		}
	}
	// AND THE DECLARED VALUE IS SHOWN FOR THE ONE THIS BUILD CANNOT EXPLAIN, which is
	// the only fact about it that identifies the routine to whoever can.
	if n("unknowndeclared") != 1 {
		t.Errorf("the unrecognised routine's own stated generation is not on the "+
			"page: %q", title)
	}
	if n("undef") != 0 {
		t.Errorf("a routine's generation rendered as `undefined`: %q", title)
	}
}

// AND A REBUILT VERSION IS OFFERED ONLY WHERE ONE CAN BE EXPLAINED AND BUILT.
//
// The two refusals are different facts with one button between them: a routine whose
// demonstration is gone has nothing to rebuild FROM, and one from a later build has nothing this
// can say about what rebuilding would CHANGE. Both read `offer` false, and the sentence beside
// the button says which — a greyed button with no explanation is a dead end.
func TestThePageOffersARebuiltVersionOnlyWhereThereIsOneToOffer(t *testing.T) {
	title, n := aRenderedGenerationPage(t)
	if n("unstatedlive") != 1 {
		t.Errorf("the routine this flow exists for cannot be reviewed: %q", title)
	}
	// A CURRENT ROUTINE MAY STILL BE COMPARED. It rebuilds to itself, which the review
	// then says — withholding the comparison would be withholding an answer.
	if n("currentlive") != 1 {
		t.Errorf("a routine of the current generation cannot be compared with its "+
			"own recording: %q", title)
	}
	if n("unknownlive") != 0 {
		t.Errorf("a routine whose generation this build cannot explain is offered a "+
			"rebuilt version anyway: %q", title)
	}
	if n("noreclive") != 0 {
		t.Errorf("a routine with no stored demonstration is offered a rebuilt "+
			"version, which is an offer that fails after somebody takes it: %q",
			title)
	}
}

// A REVIEW SHOWS BOTH SOURCES, AND CANCELLING PUTS IT AWAY WITHOUT WRITING ANYTHING.
//
// Both panes, because a review of one source is not a comparison — and each labelled with its own
// reading, so the two are told apart by more than their position. Cancelling is the absence of an
// operation: the panel goes, the window is told to let go of the version, and nothing was ever
// written to let go of.
func TestAReviewShowsBothSourcesAndCancellingWritesNothing(t *testing.T) {
	title, n := aRenderedGenerationPage(t)
	if n("reviewed") != 2 {
		t.Fatalf("the page asked for %d review(s), want the two this sequence makes "+
			"— one to show and cancel, one to replace with: %q", n("reviewed"),
			title)
	}
	if n("panes") != 1 {
		t.Errorf("the review does not show both the saved source and the rebuilt "+
			"one: %q", title)
	}
	if n("bothgens") != 1 {
		t.Errorf("the two panes are not each labelled with their own generation: %q",
			title)
	}
	if n("replacelive") != 1 {
		t.Errorf("a review with a different version in it does not offer to replace: "+
			"%q", title)
	}
	if n("aftercancel") != 0 {
		t.Errorf("cancelling left the review on the screen: %q", title)
	}
	// AND THE TOKEN OF THE REVIEW ON THE SCREEN IS WHAT TRAVELS, not the routine's
	// identity: two reviews of one routine share the latter, so dismissing by it would
	// let go of whichever the window happened to hold.
	if n("cancelledtoken") != 1 {
		t.Errorf("cancelling did not send the token of the review it dismissed: %q",
			title)
	}
	if n("cancelled") != 1 {
		t.Errorf("cancelling told the window %d time(s) to let go of the version it "+
			"held, so a second press of a dismissed button could still replace "+
			"with it: %q", n("cancelled"), title)
	}
	// AND NOTHING WAS REPLACED. The whole claim: a person who reviewed and cancelled
	// has the routine they started with.
	// AND THE CANCEL DID NOT REPLACE. The page reviews again and replaces afterwards,
	// so this is one rather than zero — what matters is that the cancel itself wrote
	// nothing, which the ORDER establishes: `cancelled` was recorded before any replace
	// was asked for.
	if n("replaced") != 1 {
		t.Errorf("the page asked for %d replacement(s) across the whole run, want "+
			"the one it makes after reviewing again: %q", n("replaced"), title)
	}
	// AND THE TOKEN OF THE REVIEW ON SCREEN IS WHAT IT REPLACED WITH, not the routine's
	// identity. Two reviews of one routine share the latter, so handing it back writes
	// whichever preparation the window held last — bytes the person never saw, under the
	// name of the routine they were looking at.
	if n("replacedtoken") != 1 {
		t.Errorf("replacing did not send the token of the review it was showing: %q",
			title)
	}
}

// aRenderedGenerationPage renders the page against the harness above and reads its counters.
func aRenderedGenerationPage(t *testing.T) (string, func(string) int) {
	t.Helper()
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, generationHarness)
	if !strings.Contains(title, "cards=") {
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

// THE PAGE OFFERS BOTH CHOICES, HANDS EACH ONE ON BY NAME, AND SAYS WHICH MADE THE REVIEW.
//
// "A version rebuilt from its recording" means two different things now. A review that did not
// say which rules made its right-hand pane would be two sources and a Replace button, with the
// person left to work out from a diff whether the pauses in their typing had just been removed.
//
// The caution is the half a diff cannot carry, and it is asserted in both directions: shown for
// the choice that changes timing and ABSENT for the one that does not, because a warning attached
// to everything is a warning nobody reads.
func TestThePageOffersBothRebuildChoicesAndSaysWhichMadeTheReview(t *testing.T) {
	title, n := aRenderedGenerationPage(t)

	if got := kindsAsked(t, title); got != "current+simplify-typing" {
		t.Errorf("the page asked the backend for %q, want each choice handed on by "+
			"its own name: %q", got, title)
	}
	if n("choice") != 1 {
		t.Errorf("the review does not say which choice produced it: %q", title)
	}
	if n("caution") != 1 {
		t.Errorf("the review of the choice that removes pauses inside typing does not "+
			"say so: %q", title)
	}
	if n("plaincaution") != 1 {
		t.Errorf("the ordinary rebuild shows a caution it has nothing to warn about, "+
			"so a warning is attached to everything: %q", title)
	}
	// AND THE RIGHT-HAND PANE IS THE SIMPLIFIED SOURCE, not the other choice's. Without
	// this the page could ask for one and render the other.
	if n("foldednow") != 1 {
		t.Errorf("the review of the simplified choice does not show the simplified "+
			"source: %q", title)
	}
	// AND THE SECOND BUTTON IS OFFERED AND WITHHELD WITH THE FIRST, from one answer.
	// Two buttons each with their own condition is two chances to offer what the
	// backend declines.
	if n("foldlive") != 1 {
		t.Errorf("the routine this flow exists for cannot be simplified: %q", title)
	}
	if n("foldoff") != 0 {
		t.Errorf("a routine whose generation this build cannot explain is offered a "+
			"simplified rebuild anyway: %q", title)
	}
}

// kindsAsked is the choices the page handed to the backend, in order.
func kindsAsked(t *testing.T, title string) string {
	t.Helper()
	m := regexp.MustCompile(`\bkinds=(\S*)`).FindStringSubmatch(title)
	if m == nil {
		t.Fatalf("the page never reported which choices it asked for: %q", title)
	}
	return m[1]
}

// THE WINDOW SAYS THAT RECORDING WATCHES EVERYTHING, BEFORE ANYBODY PRESSES RECORD.
//
// A release requirement rather than a nicety. The recorder's hooks are global: every press and
// key anywhere on the desktop enters the demonstration, and the only thing left out is Studio's
// own interactions. A person demonstrating is demonstrating the whole desktop — answer a message
// halfway through and the reply is in the routine — and the product said so nowhere.
//
// RENDERED, because a sentence nobody draws is not a disclosure. The backend returning it and the
// page having an element for it are two facts, and only something that runs the page can join
// them: a guard reading the source would pass on a page that computed the text into a hidden div.
//
// And it must not claim the opposite either. Recording only a chosen window does not exist, so
// the sentence says that, and this asserts it says that — a disclosure that implied a bound would
// be worse than none.
func TestTheWindowSaysRecordingWatchesEverythingBeforeRecord(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	const harness = `
window.go = {main: {
  Backend: {Loaded: () => Promise.resolve()},
  Acts: {
    Demonstrations: () => Promise.resolve([]),
    Saved: () => Promise.resolve([]),
    Applications: () => Promise.resolve([]),
    Pictures: () => Promise.resolve([]),
    Picture: () => Promise.resolve(''),
    Learning: () => Promise.resolve({active: false}),
    Watch: () => Promise.resolve({act: '', active: false, transcript: null}),
    History: () => Promise.resolve([]),
    Unrunnable: () => Promise.resolve([]),
    Library: () => Promise.resolve({dir: 'C:/x/routes'}),
    Routines: () => Promise.resolve([]),
    Recording: () => Promise.resolve({active: false,
      captures: 'Where you press and how long you pause.',
      everywhere: 'While this is recording, Marco watches your keyboard and mouse ' +
                  'EVERYWHERE — every application, not just the one you are ' +
                  'demonstrating in. Studio\u0027s own window is the one exception ' +
                  'and is left out. Recording only a chosen window is not something ' +
                  'Marco can do yet.'}),
  },
}}
setTimeout(() => {
  const el = document.getElementById('capEverywhere')
  const hidden = el.classList.contains('hide')
  const said = hidden ? '' : el.textContent
  document.title =
    ' shown=' + (hidden ? 0 : 1) +
    ' everywhere=' + (said.includes('EVERYWHERE') ? 1 : 0) +
    ' apps=' + (said.includes('every application') ? 1 : 0) +
    ' exception=' + (said.includes('own window is the one exception') ? 1 : 0) +
    ' notyet=' + (said.includes('not something Marco can do yet') ? 1 : 0) +
    ' undef=' + (said.includes('undefined') ? 1 : 0)
}, 2500)
`
	title := renderForCounters(t, browser, harness)
	n := func(key string) int {
		m := regexp.MustCompile(`\b` + key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	if n("shown") != 1 {
		t.Fatalf("the disclosure is not on screen before Record: %q", title)
	}
	for _, c := range []struct{ key, what string }{
		{"everywhere", "that recording watches everywhere"},
		{"apps", "that it is every application and not only the one being demonstrated in"},
		{"exception", "that Studio's own window is the exception"},
		{"notyet", "that recording only a chosen window is not something Marco can do"},
	} {
		if n(c.key) != 1 {
			t.Errorf("the disclosure does not say %s: %q", c.what, title)
		}
	}
	if n("undef") != 0 {
		t.Errorf("the disclosure rendered as `undefined`: %q", title)
	}
}
