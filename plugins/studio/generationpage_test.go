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
    CancelReview: (id) => { window.cancelled = (window.cancelled || 0) + 1 },
    ReplaceWithRebuilt: (id) => {
      window.replaced = (window.replaced || 0) + 1
      return Promise.resolve('')
    },
    ReviewRebuilt: (id) => {
      window.reviewed = (window.reviewed || 0) + 1
      if (id !== 'ark/focus/old-thing') {
        return Promise.resolve({id: id, why: 'this should never have been asked'})
      }
      return Promise.resolve({id: id, name: 'old thing',
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
let says = ['', '', '', ''], live = [0, 0, 0, 0]
const pick = (i) => {
  const cards = document.querySelectorAll('#routineList .card')
  if (cards.length > i) { cards[i].click() }
}
const look = (i) => {
  says[i] = document.getElementById('routineWrote').textContent
  live[i] = document.getElementById('routineReviewBtn').disabled ? 0 : 1
}
const reviewShown = () =>
  !document.getElementById('routineReview').classList.contains('hide')
let panes = 0, bothgens = 0, afterCancel = 1, replaceLive = 0
setTimeout(() => { pick(0) }, 1500)
setTimeout(() => { look(0); pick(1) }, 1800)
setTimeout(() => { look(1); pick(2) }, 2100)
setTimeout(() => { look(2); pick(3) }, 2400)
setTimeout(() => { look(3); pick(1); reviewRebuilt() }, 2700)
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
  cancelReview()
}, 3300)
setTimeout(() => {
  afterCancel = reviewShown() ? 1 : 0
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
    ' replaced=' + (window.replaced || 0) +
    ' reviewed=' + (window.reviewed || 0)
}, 4200)
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
	if n("reviewed") != 1 {
		t.Fatalf("the page asked for %d review(s), so this proves nothing about "+
			"what one shows: %q", n("reviewed"), title)
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
	if n("cancelled") != 1 {
		t.Errorf("cancelling told the window %d time(s) to let go of the version it "+
			"held, so a second press of a dismissed button could still replace "+
			"with it: %q", n("cancelled"), title)
	}
	// AND NOTHING WAS REPLACED. The whole claim: a person who reviewed and cancelled
	// has the routine they started with.
	if n("replaced") != 0 {
		t.Errorf("a review that was cancelled replaced the routine %d time(s): %q",
			n("replaced"), title)
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
