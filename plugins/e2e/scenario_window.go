//go:build windows

package main

// SCENARIOS ABOUT THE WINDOW ITSELF: WHAT IT SAYS IT CANNOT DO, AND WHAT IT NOTICES APPEARING.
//
// Neither of these presses anything in an application. They are about the window being honest
// about its own situation, which is the half of a product that decides whether somebody trusts
// the other half.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// readinessNoticesAMissingSibling is the stale-binary trap, caught before it costs a day.
//
// # The defect this is about
//
// Studio spawns the `marco.exe` beside ITSELF. With no sibling at all, the bare name resolves on
// PATH — so playback runs a binary of another age, successfully, and nothing says so. That is the
// one readiness case that never failed loudly: the other four were each found by hitting them at
// Stop, at Play, at Save or by comparing file timestamps, and this one produced a working run
// against yesterday's code.
//
// So the panel has to say it while everything still looks fine. The sibling is renamed INSIDE
// THIS RUN'S BUNDLE — never in the tree, which is where the owner's own binaries are and where a
// rename would break whatever they were doing — and put back afterwards whatever happens.
func readinessNoticesAMissingSibling(w *world) result {
	started := time.Now()
	store := w.freshStore()
	s, err := startStudio(w.b, w.art, store)
	if err != nil {
		return ending(started, err)
	}
	defer s.close()
	// THE RECORDING SURFACE, PROVED BY A CONTROL RATHER THAN BY THE CARD.
	//
	// `readyCard` is a div with no role, so it is correctly absent from the accessibility
	// tree even when it is on screen — only its TEXT is there, as text. What the panel says
	// is asserted below with `says`; what is waited for here has to be something the tree
	// actually holds.
	// AND IT IS `routines` PROVED BY `askBtn`: `capBtn` moved when Studio gave the
	// demonstrate-and-watch tools their own place, AND it is hidden until the recording
	// form is revealed. This scenario reads the readiness panel and records nothing. See
	// stage.go.
	if err := s.surface("routines", routinesProof); err != nil {
		return fail(started, err.Error())
	}
	// NOTHING WRONG, FIRST, AND THE RIGHT PHRASE TO LOOK FOR.
	//
	// A panel that said the same thing either way would pass this scenario without
	// noticing anything, which is the vacuous half of every before-and-after assertion.
	// `marco.exe` alone is NOT that phrase — measured: the panel names WHERE the two
	// children are whether or not there is a problem with them, which is the point of it,
	// so the string is on screen in the healthy case too. The absence is a sentence:
	// `there is no marco.exe beside this window`.
	const absent = "there is no marco.exe"
	quiet, err := s.look()
	if err != nil {
		return fail(started, err.Error())
	}
	if quiet.says(absent) {
		return fail(started, fmt.Sprintf("the readiness panel already says %q with the "+
			"sibling in place, so what it says when the sibling is gone would "+
			"establish nothing", absent))
	}
	if !quiet.says("marco.exe") {
		return fail(started, "the readiness panel does not say where the marco it would "+
			"spawn is, which is the fact the missing-sibling case is about")
	}
	// AND THE LIMITS ARE THERE ON A CLEAN MACHINE, which is when a panel gets believed.
	if !quiet.says("Nothing here looks at a screen") {
		return fail(started, "the readiness panel does not state its limits when it "+
			"has no problems to report, and a clean panel is exactly when somebody "+
			"believes it")
	}

	aside := w.b.marco + ".away"
	if err := os.Rename(w.b.marco, aside); err != nil {
		return skip(started, fmt.Sprintf("this run's own marco.exe could not be moved "+
			"aside (%v), so the missing-sibling case cannot be posed", err))
	}
	defer func() {
		if err := os.Rename(aside, w.b.marco); err != nil {
			w.art.note("restore", "marco.exe was not put back: %v", err)
		}
	}()
	w.art.step("moved %s aside", filepath.Base(w.b.marco))

	// THE PANEL POLLS, so this waits for it rather than pressing anything. Bounded well
	// past the cadence: a window that takes longer than this to notice is a window somebody
	// would have recorded against in the meantime.
	noticed, saw := until(40*time.Second, func() (bool, string) {
		shot, err := s.look()
		if err != nil {
			return false, err.Error()
		}
		e, found := shot.labelled(absent)
		if !found {
			return false, "the panel still says nothing is wrong"
		}
		return true, e.Label
	})
	if !noticed {
		st := strings.TrimSpace(saw)
		s.keepRunView()
		return fail(started, fmt.Sprintf("the marco this window would spawn is gone "+
			"and the readiness panel never said so (%s). This is the one case that "+
			"never fails loudly: the bare name resolves on PATH, playback runs a "+
			"binary of another age, and the run succeeds", st))
	}
	w.art.step("the panel says: %s", saw)
	// AND IT SAYS WHAT TO DO ABOUT IT, which is the half somebody can act on.
	shot, err := s.look()
	if err != nil {
		return fail(started, err.Error())
	}
	if !shot.says("build.cmd") {
		s.keepRunView()
		return fail(started, fmt.Sprintf("the panel reports the missing marco (%q) and "+
			"does not say what to do about it", saw))
	}
	return pass(started,
		fmt.Sprintf("with its own marco.exe moved aside the panel says: %s", saw),
		"and it names build.cmd as the way out")
}

// aRoutineSavedWhileStudioIsOpenAppears is the library being read rather than remembered.
//
// A routine can arrive from somewhere else: a terminal, another window, this one. A list drawn
// once at launch would leave somebody looking at a library that is not the library, and the
// failure is silent — they conclude the routine they just saved is gone.
func aRoutineSavedWhileStudioIsOpenAppears(w *world) result {
	started := time.Now()
	st, err := setUp(w, "fixture save", 0, "Save")
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	// A SECOND ROUTINE, SAVED BEHIND THE WINDOW'S BACK. Through the same production save
	// the first one went through, into the library this Studio is reading.
	second, err := seedPresses(st.f, w.art, "fixture cancel", 0, "Cancel")
	if err != nil {
		return ending(started, err)
	}
	id := cardID(routeIdentity(second))
	if _, err := st.s.waitFor(30*time.Second, id, present); err != nil {
		st.fail()
		return fail(started, fmt.Sprintf("a routine saved into this window's library "+
			"while it was open never appeared in it: %v", err))
	}
	// AND THE FIRST ONE IS STILL THERE. A list that replaced itself with the newcomer
	// would pass the assertion above and have lost the routine somebody was working on.
	if _, err := st.s.waitFor(10*time.Second, cardID(routeIdentity(st.seed)), present); err != nil {
		st.fail()
		return fail(started, fmt.Sprintf("the new routine appeared and the one that "+
			"was already there is gone: %v", err))
	}
	return pass(started,
		fmt.Sprintf("%q was saved while the window was open and appeared in it, "+
			"beside %q", second.name, st.seed.name))
}
