//go:build windows

package main

// SCENARIO: SIMPLIFYING A ROUTINE IN THE REAL STUDIO, AND THEN PLAYING WHAT CAME OUT.
//
// The other choice on the review surface, and the one with a behaviour change in it: folding each
// run of typing into one step removes the pauses inside that run. So this scenario has one more
// question than the generation one, and it is the one that matters most — **does the thing it
// wrote still work**. A review that produced a routine nobody could run would be a review of a
// file rather than of a routine.
//
//	the two choices differ      the saved routine types in two steps; the simplified one in one
//	the caution is on screen    before anything can be replaced with
//	Cancel changes nothing      measured against every file in the library
//	Replace writes the review   byte for byte, read off disk
//	and it still performs       played through `marco do`, asserted against the fixture's field
//
// # Why the pause in the demonstration is 540ms
//
// Because that is between the two thresholds. `simplify` folds a run of keys into one `Type`
// while the gaps read as typing rhythm, and the two choices draw that line at 300ms and at a
// second — so a 540ms pause is a deliberate wait to one rebuild and rhythm to the other. A
// demonstration without one produces the same routine twice, and every assertion below would
// stay true while establishing nothing.
//
// # The oracle for the playback half is the application's own field
//
// Same as the typing family's, and for the same reason: `SendInput` returning a count says the
// system took the events, so Marco's accounts agree on "submitted" and none of them can say what
// is in the box. A simplified routine that types faster than the demonstration did is exactly the
// case where that distinction has teeth.
//
// Physical recording capture is NOT EXERCISED, as everywhere: the demonstration behind the
// routine is seeded, which this harness says in its own report.
//
// See [[ADR-262-a-routine-states-which-generation-wrote-it]].

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// theTypedRuns are the two runs the seeded demonstration types, with a pause between them.
//
// Together they spell what the typing family's scenario types, so the fixture's field is read the
// same way at the end — and separately they are what the ordinary rebuild keeps as two steps.
var theTypedRuns = []string{"marco typed", " this 42"}

// thePauseBetweenRuns is between the two thresholds on purpose. See the note at the top.
const thePauseBetweenRuns = 540 * time.Millisecond

// aSimplifiedRoutineIsReviewedReplacedAndPlayed is the whole thing, in one run, because the last
// question is about what the first four produced.
func aSimplifiedRoutineIsReviewedReplacedAndPlayed(w *world) result {
	started := time.Now()
	st, err := setUpSeeded(w, func(f *fixtureRun) (seededRoutine, error) {
		return seedTypingRuns(f, w.art, "fixture simplify", "Name",
			thePauseBetweenRuns, theTypedRuns...)
	})
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	saved, err := readFileString(st.seed.path)
	if err != nil {
		return ending(started, err)
	}
	library := theWholeLibrary(st.store)

	// ── THE SIMPLIFIED CHOICE, REVIEWED ──
	if _, err := st.s.waitFor(10*time.Second, "routineSimplifyBtn", enabled); err != nil {
		st.fail()
		return fail(started, fmt.Sprintf("Studio will not offer to simplify %q: %v",
			st.seed.name, err))
	}
	if err := st.s.press("routineSimplifyBtn"); err != nil {
		return ending(started, err)
	}
	if err := st.s.waitForSimplifiedReview(); err != nil {
		st.fail()
		return fail(started, err.Error())
	}
	if why := theLibraryMoved(st.store, library, "while the review was on screen"); why != "" {
		st.fail()
		return fail(started, why)
	}
	w.art.step("the simplified review is up, with its caution, and nothing is written")

	// ── CANCEL: NOTHING WRITTEN, ANYWHERE ──
	if err := st.s.press("routineReviewCancelBtn"); err != nil {
		return ending(started, err)
	}
	if err := st.s.waitForNoReview(); err != nil {
		st.fail()
		return fail(started, err.Error())
	}
	if why := theLibraryMoved(st.store, library, "when the review was cancelled"); why != "" {
		st.fail()
		return fail(started, why)
	}
	w.art.step("the review was cancelled and the whole library is untouched")

	// ── REVIEW AGAIN, AND REPLACE ──
	if err := st.s.press("routineSimplifyBtn"); err != nil {
		return ending(started, err)
	}
	if err := st.s.waitForSimplifiedReview(); err != nil {
		st.fail()
		return fail(started, err.Error())
	}
	if _, err := st.s.waitFor(10*time.Second, "routineReplaceBtn", enabled); err != nil {
		st.fail()
		return fail(started, fmt.Sprintf("Studio will not offer to replace %q with the "+
			"simplified version: %v", st.seed.name, err))
	}
	if err := st.s.press("routineReplaceBtn"); err != nil {
		return ending(started, err)
	}
	now, err := theReplacedRoutine(st.seed.path, saved)
	if err != nil {
		st.fail()
		return fail(started, err.Error())
	}
	w.art.keep("simplified-"+st.seed.route.Slug+".marco", []byte(now))
	if why := theSimplifiedRoutineIsWrong(now, saved); why != "" {
		st.fail()
		return fail(started, why, fmt.Sprintf("the file now holds:\n%s", now))
	}
	w.art.step("the routine now types its text in one step")

	// ── AND IT STILL PERFORMS ──
	before, seq, records := st.f.mustRead(), st.f.lastSeq(), countRecords(st.store)
	if before.Name != "" {
		return skip(started, fmt.Sprintf("the fixture's Name field already holds %q, "+
			"so what it holds afterwards would not be this run's doing",
			before.Name))
	}
	if err := st.play(); err != nil {
		return ending(started, err)
	}
	rec, err := waitForRecord(60*time.Second, st.store, records)
	if err != nil {
		st.fail()
		return fail(started, fmt.Sprintf("Marco wrote no finished run record for the "+
			"simplified routine: %v. The application received %s", err,
			st.f.arrivedSoFar(seq)))
	}
	w.art.step("Marco recorded %s", rec)
	if err := st.f.drained(30 * time.Second); err != nil {
		st.fail()
		return skip(started, err.Error())
	}
	after := st.f.mustRead()
	w.art.step("the fixture's Name field holds %q", after.Name)

	want := strings.Join(theTypedRuns, "")
	why := firstOf(
		theTextDidNotArrive(after, want),
		itWentIntoAnotherField(before, after),
		theNextStepDidNotHappen(before, after),
		marcoSaidSomethingElse(rec, st.seed),
	)
	if why != "" {
		st.fail()
		return fail(started, why,
			fmt.Sprintf("the field holds %q and the simplified routine typed %q",
				after.Name, want),
			fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		"the review was up twice and cancelled once, and no file in the library "+
			"changed until Replace was pressed",
		"the replaced routine types its text in ONE step where the saved one typed "+
			"it in two, and is otherwise what the generator writes",
		fmt.Sprintf("and it still performs: the application's own Name field holds "+
			"%q, with Save pressed after it", after.Name),
		fmt.Sprintf("Marco's record: %s", rec))
}

// ── WHAT THIS FAMILY ADDS ─────────────────────────────────────────────────────

// waitForSimplifiedReview waits until the review is up AND is the simplified one.
//
// Which choice is on screen is the thing a person decides from, so a scenario that pressed one
// button and read whatever panel appeared would pass on a window wired to the other. Both halves
// are looked for: the words of the choice, and the caution — because the caution is the part that
// says a behaviour changed, and a review of this choice without it is a review that reads as a
// tidy-up.
func (s *studioRun) waitForSimplifiedReview() error {
	ok, saw := until(20*time.Second, func() (bool, string) {
		shot, err := s.look()
		if err != nil {
			return false, err.Error()
		}
		return theSimplifiedReviewIsNotUp(shot)
	})
	if !ok {
		return fmt.Errorf("Studio did not show a review of the simplified version: %s",
			saw)
	}
	return nil
}

// theWholeLibrary is every file under a store, digested.
//
// THE WHOLE OF IT, not the one routine. A review that wrote anywhere at all has written, and "it
// did not touch the file I was looking at" is a weaker claim than the one being made.
func theWholeLibrary(store string) map[string]string {
	out := map[string]string{}
	_ = filepath.WalkDir(store, func(path string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		rel, _ := filepath.Rel(store, path)
		out[rel] = routes.DigestOf(string(raw))
		return nil
	})
	return out
}

// theLibraryMoved names what changed under the store, or "" when nothing did.
func theLibraryMoved(store string, before map[string]string, when string) string {
	if len(before) == 0 {
		return "the library was empty when this run started, so nothing here could " +
			"detect a write"
	}
	after := theWholeLibrary(store)
	var said []string
	for path, was := range before {
		switch now, there := after[path]; {
		case !there:
			said = append(said, path+" is gone")
		case now != was:
			said = append(said, path+" was rewritten")
		}
	}
	for path := range after {
		if _, there := before[path]; !there {
			said = append(said, path+" was created")
		}
	}
	if len(said) == 0 {
		return ""
	}
	sort.Strings(said)
	return fmt.Sprintf("the library changed %s, and nothing had been replaced yet: %s",
		when, strings.Join(said, ", "))
}

// theSimplifiedRoutineIsWrong checks the one difference this choice is for.
//
// The saved routine types its text in TWO steps, because the pause between the runs is a
// deliberate wait by the ordinary rule. The simplified one types it in ONE. So the replaced file
// has to hold the whole text as a single `Type` and neither of the halves on its own — which is a
// far stronger claim than "it changed", and it is the claim the choice makes.
func theSimplifiedRoutineIsWrong(now, was string) string {
	whole := fmt.Sprintf("do OS's Type with %q", strings.Join(theTypedRuns, ""))
	if !strings.Contains(now, whole) {
		return fmt.Sprintf("the replaced routine does not type its text in one step "+
			"(no %s in it), which is the whole of what this choice does", whole)
	}
	for _, run := range theTypedRuns {
		part := fmt.Sprintf("do OS's Type with %q", run)
		if strings.Contains(now, part) {
			return fmt.Sprintf("the replaced routine still types %q as a step of its "+
				"own, so the runs were not folded together", run)
		}
		if !strings.Contains(was, part) {
			return fmt.Sprintf("the SAVED routine did not type %q as a step of its "+
				"own, so the two rebuilds would have been the same and this "+
				"establishes nothing", run)
		}
	}
	// AND IT IS STILL A CHECKED ROUTINE. Folding typing may not cost the window check:
	// the press before the typing and the press after it are both measured, and a
	// rebuild that lost that would be a routine that presses wherever the cursor is.
	if !strings.Contains(now, `RelTo "window"`) {
		return "the replaced routine states no window for its presses, so simplifying " +
			"the typing cost it the check it had"
	}
	return ""
}

// theSimplifiedReviewIsNotUp is what that wait looks for, in one place a test can reach.
//
// Lifted out because a wait is a PRECONDITION, and a precondition that stopped looking for
// anything would make every assertion after it true of a window in some other state — with the
// scenario still green. It runs only on a desktop, so the only way to know it still asks three
// questions is to ask it, headlessly, about a window that answers two of them.
func theSimplifiedReviewIsNotUp(shot snapshot) (bool, string) {
	if _, there := shot.byID("routineReplaceBtn"); !there {
		return false, "the review is not on screen"
	}
	switch {
	case !shot.says("Nothing has been written yet"):
		return false, "the review does not say that nothing has been written"
	case !shot.says("Simplify typing pauses"):
		return false, "the review does not say which choice produced it"
	case !shot.says("removes the pauses"):
		return false, "the review does not say that folding the typing removes the " +
			"pauses inside it"
	}
	return true, "showing"
}
