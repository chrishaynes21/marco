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
    Help: () => Promise.resolve({sections: [{key: 'editing',
      title: 'Editing what a routine does', lines: ['nothing is written yet']}]}),
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
const inClosedDisclosure = (el) => {
  for (let p = el; p; p = p.parentElement) {
    if (p.tagName === 'DETAILS' && !p.open) { return true }
  }
  return false
}
// onScreen is what an accessibility tree holds, which is neither textContent nor offsetParent
// alone. A hidden ancestor (display:none) takes offsetParent away; a CLOSED DISCLOSURE does
// not — Chromium keeps a layout box for the content of a closed <details> — and UI Automation
// leaves it out all the same. The provenance line is in one, so the first version of this read
// it as on screen while the real window did not offer it at all.
const onScreen = (id) => {
  const el = document.getElementById(id)
  if (!el || el.offsetParent === null) { return 0 }
  return inClosedDisclosure(el) ? 0 : 1
}
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
let wroteClosed = 1, wroteOpened = 0, summaryThere = 0
setTimeout(() => { pick(0) }, 1500)
setTimeout(() => { look(0); pick(1) }, 1800)
setTimeout(() => { look(1); pick(2) }, 2100)
setTimeout(() => { look(2); pick(3) }, 2400)
setTimeout(() => {
  look(3)
  // THE PROVENANCE LINE IS BEHIND A DISCLOSURE, and a scenario has to open it.
  wroteClosed = onScreen('routineWrote')
  summaryThere = document.getElementById('routineMaintainSummary') ? 1 : 0
  press('routineMaintainSummary')
}, 2700)
setTimeout(() => {
  wroteOpened = onScreen('routineWrote')
  pick(1); press('routineReviewBtn')
}, 2900)
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
    ' reviewed=' + (window.reviewed || 0) +
    ' wroteclosed=' + wroteClosed +
    ' wroteopened=' + wroteOpened +
    ' summarythere=' + summaryThere
}, 5400)
`

// AND THE PROVENANCE LINE IS BEHIND A DISCLOSURE SOMEBODY HAS TO OPEN.
//
// It lives in the collapsed `Rebuild and re-record` detail, which is where the maintenance facts
// were deliberately put. `textContent` reads it there perfectly well, which is why every test in
// this file was happy with it — and an accessibility tree does not hold it at all, so the desktop
// scenario that reads Studio's agreement before pressing Review failed in stage 6 of the packaged
// gate saying Studio did not say what it says one click away.
//
// So the disclosure has an id, and this asserts what opening it does. `plugins/e2e`
// `scenario_generation.go` presses the same id.
func TestTheProvenanceLineIsBehindADisclosureWithAnId(t *testing.T) {
	title, n := aRenderedGenerationPage(t)

	if n("summarythere") != 1 {
		t.Fatalf("the maintenance disclosure has no id, so nothing outside this window "+
			"can open it: %q", title)
	}
	if n("wroteclosed") != 0 {
		t.Errorf("the provenance line is on screen without the disclosure being "+
			"opened, so this test no longer describes the page: %q", title)
	}
	if n("wroteopened") != 1 {
		t.Errorf("opening the disclosure does not put the provenance line on screen, "+
			"so a scenario that presses it still cannot read Studio's agreement: %q",
			title)
	}
}

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

// THE RENDERED PAGE SAYS HOW TO MAKE A VALUE SOMETHING YOU SUPPLY.
//
// RENDERED, for the reason the disclosure above it is: the backend returning a sentence and the
// page having an element for it are two facts, and only something that runs the page joins them.
// A guard reading the source would pass on a page that wrote the text into a div it never shows,
// which is indistinguishable — from where a person is standing — from the silence this
// closes. `Recording.ArgSays` is held by `TestTheWindowSaysHowToMakeAValueSomethingYouSupply`;
// this holds that somebody can read it.
//
// AND THAT IT GOES AWAY. The second render answers the other half: with the arg key off the
// backend says nothing, and an element left on screen holding the previous poll's sentence would
// name a key that does nothing. Hidden rather than empty, because an empty muted div is a gap in
// a card that otherwise has none.
func TestTheRenderedPageSaysHowToMakeAValueSomethingYouSupply(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	harness := func(argSays string) string {
		return `
window.go = {main: {
  Backend: {
    Loaded: () => Promise.resolve(),
    SuggestRoutine: () => Promise.resolve({answer: {kind: 'none'}}),
  },
  Acts: {
    SuggestTargets: () => Promise.resolve([]),
    SuggestRoutine: () => Promise.resolve({kind: 'none'}),
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
    // WHICH PARTS OF THIS WINDOW ARE OFFERED, AND WHAT IT EXPLAINS.
    // Both are read once at startup; a stub without them is a page
    // whose rail never paints. See help.go.
    Features: () => Promise.resolve({acts: true, scenes: true,
      views: ['routines', 'acts', 'scenes', 'history', 'help']}),
    Help: () => Promise.resolve({sections: [{key: 'editing',
      title: 'Editing what a routine does', lines: ['nothing is written yet']}]}),
    PlaybackTarget: () => Promise.resolve({chosen: '4660:77',
      windows: [{id: '4660:77', label: 'a window',
        says: 'Play brings a window to the front and presses there.'}]}),
    Routines: () => Promise.resolve([]),
    Recording: () => Promise.resolve({active: false, stop_key: 'F12',
      captures: 'Where you press and how long you pause.',
      everywhere: 'Records actions in the window you choose.',
      arg_key: 'F9', arg_says: ` + "`" + argSays + "`" + `}),
  },
}}
setTimeout(() => {
  const el = document.getElementById('capArgSays')
  const hidden = !el || el.classList.contains('hide')
  const said = hidden ? '' : el.textContent
  document.title =
    ' shown=' + (hidden ? 0 : 1) +
    ' key=' + (said.includes('F9') ? 1 : 0) +
    ' asks=' + (said.includes('ask for a value') ? 1 : 0) +
    ' names=' + (said.includes('with item') ? 1 : 0) +
    ' undef=' + (said.includes('undefined') ? 1 : 0)
}, 2500)
`
	}
	n := func(title, key string) int {
		m := regexp.MustCompile(`\b` + key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}

	title := renderForCounters(t, browser, harness(
		"Press F9 where the routine should ask for a value instead of typing "+
			"what you typed. Call it “name with item” and the value "+
			"is called item."))
	if n(title, "shown") != 1 {
		t.Fatalf("nothing on the record card says a routine can ask for a value: %q", title)
	}
	for _, c := range []struct{ key, what string }{
		{"key", "which key marks a value"},
		{"asks", "what pressing it does"},
		{"names", "how the name declares the value"},
	} {
		if n(title, c.key) != 1 {
			t.Errorf("the rendered sentence does not say %s: %q", c.what, title)
		}
	}
	if n(title, "undef") != 0 {
		t.Errorf("the sentence rendered as `undefined`: %q", title)
	}

	// ── AND NOTHING WHEN THERE IS NO KEY ──
	if off := renderForCounters(t, browser, harness("")); n(off, "shown") != 0 {
		t.Errorf("the card still offers an arg key the backend did not name: %q", off)
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

// THE WINDOW SAYS WHAT A RECORDING TAKES IN AND HOW IT ENDS, BEFORE ANYBODY PRESSES RECORD.
//
// A release requirement rather than a nicety, and it has been both halves of one in turn.
//
// THIS TEST USED TO HOLD A FALSE CLAIM IN PLACE, which is why the inversion is recorded here
// rather than quietly rewritten. It asserted the disclosure said Marco watches the keyboard and
// mouse EVERYWHERE, that it is every application, and that "recording only a chosen window is not
// something Marco can do yet" — all true of the hooks when it was written, and none of them true
// after [[ADR-268-record-only-the-chosen-window]]. A disclosure asserting the opposite of what the
// recorder does is worse than none: the largest thing on the record card denied the window chooser
// two rows below it, and this test is what kept it there.
//
// So it asserts the two facts a person needs before pressing Record — that the window they pick is
// the bound, and that leaving it pauses rather than records — and that THE WAY OUT IS NAMED while
// they can still read it. The stop key lived only in the band, which is behind the application
// being demonstrated in for as long as it is on screen.
//
// RENDERED, because a sentence nobody draws is not a disclosure. The backend returning it and the
// page having an element for it are two facts, and only something that runs the page can join
// them: a guard reading the source would pass on a page that computed the text into a hidden div.
func TestTheWindowSaysWhatARecordingTakesInBeforeRecord(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	const harness = `
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
    Help: () => Promise.resolve({sections: [{key: 'editing',
      title: 'Editing what a routine does', lines: ['nothing is written yet']}]}),
    // THE WINDOW A RUN WOULD PRESS IN. Studio does not Play without one, so a
    // harness that does not answer this is a page whose Play button is dead.
    PlaybackTarget: () => Promise.resolve({chosen: '4660:77',
      windows: [{id: '4660:77', label: 'a window',
        says: 'Play brings a window to the front and presses there.'}]}),
    Routines: () => Promise.resolve([]),
    Recording: () => Promise.resolve({active: false, stop_key: 'F12',
      captures: 'Where you press and how long you pause.',
      everywhere: 'Records actions in the window you choose. Recording pauses ' +
                  'outside it. Return to Studio and press Stop, or press F12.'}),
  },
}}
setTimeout(() => {
  const el = document.getElementById('capEverywhere')
  const hidden = el.classList.contains('hide')
  const said = hidden ? '' : el.textContent
  document.title =
    ' shown=' + (hidden ? 0 : 1) +
    ' chosen=' + (said.includes('in the window you choose') ? 1 : 0) +
    ' pauses=' + (said.includes('pauses outside it') ? 1 : 0) +
    ' stop=' + (said.includes('press Stop') ? 1 : 0) +
    ' key=' + (said.includes('press F12') ? 1 : 0) +
    ' everywhere=' + (said.includes('EVERYWHERE') ? 1 : 0) +
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
		{"chosen", "that what is recorded is the window the person chooses"},
		{"pauses", "that leaving that window pauses the recording rather than recording elsewhere"},
		{"stop", "that Stop in this window is a way out"},
		{"key", "which key is the other way out"},
	} {
		if n(c.key) != 1 {
			t.Errorf("the disclosure does not say %s: %q", c.what, title)
		}
	}
	// ── AND NOT THE CLAIM IT USED TO ASSERT ──
	//
	// The two clauses that denied the chooser under this card. Named rather than left to
	// the positive assertions above, because the defect was a sentence that said both: a
	// disclosure can name the chosen window AND go on to say recording it is not possible,
	// and every assertion above would pass.
	for _, c := range []struct{ key, what string }{
		{"everywhere", "that it watches everywhere"},
		{"notyet", "that recording one chosen window cannot be done yet"},
	} {
		if n(c.key) != 0 {
			t.Errorf("the disclosure still claims %s, which is the opposite of "+
				"what the recorder does: %q", c.what, title)
		}
	}
	if n("undef") != 0 {
		t.Errorf("the disclosure rendered as `undefined`: %q", title)
	}
}
