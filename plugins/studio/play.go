package main

// PRESS PLAY, AND MARCO PERFORMS THE ACT.
//
// # What this replaces
//
// A person who wanted to run something they had demonstrated had to: tick a box that said what
// live meant, type the executable name, tick a second box about reading the screen, press Run,
// find their game, bring it forward without clicking anything in it, race a five-second
// countdown, press F7 to say they were ready, answer a question about which saved screen they
// were looking at by agreeing with an arrangement score, and then answer two more questions per
// interaction. The owner's verdict on operating that: an engineering acceptance checklist handed
// over as a product.
//
// Every one of those except the first is now Marco's to do or to have already answered. What a
// person does is choose an Act and press Play.
//
// # What Play actually does, in order
//
//	1. finds the Act, and refuses now if it would not run — before anything is raised
//	2. writes the AUTHORISATION: this Act, this revision, these interactions, by them, now
//	3. brings the application the Act is pinned to forward
//	4. starts the supervised runner under that authorisation
//
// # Why the raising happens HERE and not in the runner
//
// Because Windows refuses it anywhere else. `SetForegroundWindow` is rejected for a process that
// does not already own the foreground — the foreground lock — and it was measured on this machine
// from a harness trying to raise a window from outside: the call returned false and the
// foreground did not move. The supervised runner is a child process nobody is looking at, so it
// cannot raise anything.
//
// Studio can, and only at this instant: the person just clicked a button in it, so it owns the
// foreground and may hand it on. That is the same split `plugins/fixture` already draws and says
// so in its own words — the surface may put the window in front, and Marco's own foreground guard
// is what CHECKS that it is there at the instant of execution. Nothing here presses anything, and
// nothing here simulates a keypress to get around the lock.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
)

// actGrantVersion is the shape the runner reads. See cmd/director/actgrant.go, which is the only
// other place this number appears: a surface writing a version the runner does not perform is
// refused rather than guessed at.
const actGrantVersion = 1

// grant is the authorisation a person gives by pressing Play, as the runner reads it.
//
// Written here and never remembered: one file per run, in a temporary directory, naming exactly
// what the person was looking at when they pressed. It is not a session flag and not a
// preference — the next run writes another one, because the next run is another decision.
type grant struct {
	Version      int      `json:"version"`
	Act          string   `json:"act"`
	Revision     string   `json:"revision,omitempty"`
	Application  string   `json:"application,omitempty"`
	Interactions []string `json:"interactions"`
	// Source is the digest of the play these interactions were bound to, and Bound is
	// what each name MEANS. Both because neither the names nor the revision pin it: the
	// same Act can be composed again over the same demonstration with the same names in
	// the same order and different positions, which changes every control that would be
	// pressed and nothing a person can see. See cmd/director/actgrant.go.
	Source string         `json:"source,omitempty"`
	Bound  []BoundNameFor `json:"bound,omitempty"`
	By     string         `json:"by"`
	At     time.Time      `json:"at"`
}

// Performance is one press of Play, with everything the surface knows about it.
type Performance struct {
	// Act is the one to perform, by the name the person gave it.
	Act string
	// Live is the whole of the decision to touch the machine, and it is still the
	// person's: without it the entire path runs and each press is written down.
	Live bool
	// Grant is the Act to perform under the authorisation of this press — the product
	// contract — and it is the SNAPSHOT `Play` read and checked, not a name for the
	// other end to look up again. Nil is the engineering path: a run that asks for every
	// press by key, which is what `Run` keeps for the acceptance checklist.
	//
	// A pointer to the Act itself rather than a flag beside its name, because a flag left
	// the reading to `start`, which looked the name up again and granted whatever it
	// found then. One read, one decision, one authorisation.
	Grant *Act
	// Application and Reading are overrides the person does not normally supply. Empty
	// and true respectively under Play: the Act in `Grant` carries which executable it
	// was demonstrated against, and a run that cannot read the screen cannot verify an
	// outcome, which is the thing that makes an Act more than a macro.
	Application string
	Reading     bool
}

// Play performs an Act under the authorisation of this press.
//
// Returns "" when a run started, or the sentence to show the person. Every refusal here happens
// BEFORE anything is raised or started, because a window that came forward and then said no is
// worse than a sentence in the surface they were already looking at.
// `shown` is the Act's identity as the page was displaying it when the button was pressed — its
// digest, revision, application and what each interaction name means, as one string (see
// identityOf) — or "" when the page has none, because somebody typed a name rather than clicking
// a listed Act. It closes the window between selecting an Act and performing it: Studio reads the
// Act here, once, refuses it if it is no longer the one they read, and hands that same snapshot
// to the run. An Act composed again in between is a different set of controls under the same
// name, and it must not inherit a decision made about the old one.
func (a *Acts) Play(act string, live bool, shown string) string {
	chosen, found := a.chosen(act)
	switch {
	case !found:
		return fmt.Sprintf("there is no Act called %q on this machine", act)
	case shown != "" && shown != chosen.Identity:
		// COMPOSED AGAIN SINCE IT WAS LISTED. Not performed under the decision
		// somebody made about the old one: the names can be identical and mean
		// different interactions, so what they were looking at when they pressed is
		// not what this would press. Refused, and sent back to look at it.
		//
		// The term compared is the Act's whole identity and was once only the play
		// digest, which a recompose of the same names onto different positions
		// leaves untouched — so the check passed in exactly the case it was written
		// for. See identityOf.
		return "this Act has been composed again since it was listed, so it may " +
			"perform different interactions than the ones you were looking at — " +
			"press Refresh and read it again before performing it"
	case !chosen.Runnable:
		// THE PREFLIGHT'S OWN WORDS. It has already asked whether the play still
		// matches its bindings, whether the demonstration is still the revision it
		// was pinned to, and whether every interaction is real. A person needs that
		// sentence, not "it would not run".
		return "this Act cannot be performed as it stands: " + chosen.Why
	case len(chosen.Interactions) == 0:
		return "this Act performs no interactions"
	}
	// AND THE APPLICATION HAS TO BE THERE, which is answerable now and otherwise
	// becomes a run paused about a window that was never open.
	if why := a.raise(chosen.Application); why != "" {
		return why
	}
	// AND THE ACT ITSELF TRAVELS, not just its name. Everything from here on performs
	// the version read and checked above.
	return a.start(Performance{Act: chosen.Name, Live: live, Grant: &chosen,
		Reading: true})
}

// chosen is the Act by name, as the surface lists it.
func (a *Acts) chosen(act string) (Act, bool) {
	for _, c := range a.Saved() {
		if strings.EqualFold(c.Name, act) {
			return c, true
		}
	}
	return Act{}, false
}

// granted writes the authorisation for one performance and returns its path.
//
// A failure to write it is a refusal of the run and not something to proceed without: the
// authorisation IS the permission, and a run started without one would be a run asking questions
// nobody is there to answer.
func (a *Acts) granted(c Act) (string, error) {
	f, err := os.CreateTemp("", "studio-granted-*.json")
	if err != nil {
		return "", err
	}
	defer f.Close()
	raw, err := json.Marshal(grant{
		Version: actGrantVersion, Act: c.Name, Revision: c.Revision,
		Application: c.Application, Interactions: c.Interactions,
		Source: c.Source, Bound: c.Bound,
		// WHAT IT WAS, in words, because this sentence ends up in the run record
		// and in the binding that establishes the screen. "The person who pressed
		// Play in Studio" and "the person at the keyboard (F7)" are two different
		// claims about how a press came to be allowed, and the record keeps them
		// apart.
		By: "the person who pressed Play in Studio", At: time.Now(),
	})
	if err != nil {
		return "", err
	}
	if _, err := f.Write(raw); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// raise brings the application forward, through the desktop or through a test's stand-in.
//
// A seam of the same kind as `afterReserve`: what is under test in this package is the surface's
// own bookkeeping, and raising a real window needs a real application running — so a test says
// what the desktop would have said instead of needing one. Nil everywhere in production, where
// the next function is what happens.
func (a *Acts) raise(app string) string {
	if a.raising != nil {
		return a.raising(app)
	}
	return a.bringForward(app)
}

// bringForward finds the application an Act is pinned to and raises it.
//
// Returns "" when the run may proceed, or the sentence to show the person instead.
//
// # Why a failure to RAISE is not a refusal
//
// Because the runner checks it anyway, and its check is the one that matters. Raising is best
// effort — the foreground lock can refuse it, a full-screen game can decline to come forward,
// another window can be above it — and the honest division is that this tries and the run's own
// occlusion and foreground guards decide. A run that cannot see the window pauses saying so,
// which is a sentence about what is actually wrong rather than a guess made here.
//
// Not being able to FIND it is different, and is refused: nothing can be performed against an
// application that is not running, and saying so now costs the person nothing.
func (a *Acts) bringForward(app string) string {
	if app == "" {
		// NO APPLICATION RECORDED. The run then grounds against whatever leads the
		// desktop, which is the old `--foreground` behaviour and is still honest —
		// there is nothing to raise and nothing to check against.
		return ""
	}
	c, found := theWindowToRaise(a.windowsNow(), app)
	if !found {
		return fmt.Sprintf("%s does not seem to be running — start it and press "+
			"Play again", app)
	}
	// AND IF IT IS NOT IN FRONT AFTERWARDS, SAY SO NOW.
	//
	// This used to be best effort in silence — `_ = a.lift(...)`, on the grounds that
	// the runner's own guards decide. They do, and what that looked like to the owner
	// was this: Play, nothing visible, thirty seconds, "refused". The run had paused
	// correctly and repeatedly about a screen it could not see, in a window they were
	// looking at instead of the game, because the game was never brought forward.
	//
	// A raise that did not work is therefore answered here, where the sentence can say
	// what to do about it, and the run is not started: it could only pause for its whole
	// deadline. `checked` keeps this honest — a platform that cannot tell where the
	// foreground is says nothing and starts the run, exactly as before.
	leading, checked := a.lift(c.Handle, c.ProcessID)
	if checked && !leading {
		return fmt.Sprintf("Windows would not bring %s to the front, and Marco "+
			"will not press anything it cannot see — click %s's own window "+
			"yourself, then press Play again. (Windows only lets the window "+
			"you last clicked hand the foreground on, so pressing Play while "+
			"a full-screen game is behind it can be refused.)", app, app)
	}
	return ""
}

// theWindowToRaise picks which of an application's windows to bring forward.
//
// # Why a minimized window is a candidate
//
// Because it is running, and somebody who pressed Play on an Act for it meant that window. The
// first version of this skipped minimized windows and then told them the application "does not
// seem to be running, or its window is minimised — bring it up and press Play again", which is
// the surface asking for a thing it can do itself: `raiseWindow` restores before it activates,
// and that branch was unreachable for exactly the windows it was written for.
//
// # What it prefers
//
// A window already on screen, and then the biggest. An executable owns several windows — a
// launcher, a splash, the main one — and the one somebody is looking at is the one with the
// pixels. Minimized bounds are not a reliable size on Windows (an iconic window reports an
// off-screen position and whatever size it will restore to), so a minimized window loses to any
// window that is up, and among themselves they are compared on the same arithmetic because there
// is nothing better to compare them on.
func theWindowToRaise(cands []windowref.Candidate, app string) (windowref.Candidate, bool) {
	var best windowref.Candidate
	found := false
	for _, c := range cands {
		if !strings.EqualFold(c.Application, app) || c.Handle == 0 {
			continue
		}
		if !found || betterToRaise(c, best) {
			best, found = c, true
		}
	}
	return best, found
}

// betterToRaise says whether one window is a better thing to bring forward than another.
func betterToRaise(c, than windowref.Candidate) bool {
	if c.Minimized != than.Minimized {
		// THE ONE THAT IS UP, whichever of the two that is.
		return than.Minimized
	}
	return onScreenArea(c) > onScreenArea(than)
}

// onScreenArea is a window's area, never negative.
func onScreenArea(c windowref.Candidate) int {
	w, h := c.Bounds.Width, c.Bounds.Height
	if w < 0 || h < 0 {
		return 0
	}
	return w * h
}

// windowsNow is every top-level window, or what a test says there is.
func (a *Acts) windowsNow() []windowref.Candidate {
	if a.listing != nil {
		return a.listing()
	}
	return winprovider.New().AllCandidates(context.Background())
}

// lift raises one window, through the desktop or through a test's stand-in.
//
// A seam of its own rather than part of `raising`, so a test can let the real choosing happen and
// still see WHICH window the choice reached. The restore branch for a minimized window is behind
// a Windows syscall; what a test can hold is that the decision gets there with the right handle.
func (a *Acts) lift(h uintptr, pid uint32) (leading, checked bool) {
	if a.lifting != nil {
		return a.lifting(h, pid)
	}
	return raiseWindow(h, pid)
}
