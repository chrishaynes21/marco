package main

// WHICH WINDOW THIS RUN ACTS IN, CHOSEN BEFORE PLAY AND CARRIED TO THE RUN.
//
// # The third question, and it is not either of the other two
//
// Where a routine was RECORDED is settled before the hooks go on and is a fact about the
// demonstration. Where a routine is AVAILABLE is its folder, and is durable. Which window THIS RUN
// acts in is neither: it is a decision somebody makes at the instant they press Play, about a
// window that exists now, and it must never become durable — a handle written into a route is an
// identity that stops existing when the window closes, and the next run would inherit it.
//
// # The defect this closes
//
// Studio raised an application's window and the run bound one. Two steps, two choices, and
// nothing made them the same choice: `theWindowToRaise` here prefers the biggest window that is
// up, and `winctx.Bind` in the child takes the first visible window `EnumWindows` reaches. With
// two Explorer windows, two copies of a game or two browser profiles those are different windows.
//
// NOTHING DOWNSTREAM COULD TELL. `oshost.confirmWindow` compares the program, the owning process,
// the size and the scale of the window that was bound — and two windows of one program agree on
// program, size class and scale, while the PID check compares the bound window against itself. So
// the press landed in a window nobody chose, having passed every check there is.
//
// And for a routine with no application at all, nothing was raised: the run began with Studio in
// front, so an unguarded press went into this window.
//
// # What this does instead
//
// The window is chosen HERE, from the same list `marco learn` chooses a recording's window out of
// — `orchestrator.Pickable`, which excludes this process and the surface that owns it, so Studio
// is never a candidate. One candidate is selected automatically and SHOWN beside Play. More than
// one is a visible choice. None means Play is unavailable with a reason.
//
// Then that window travels: Studio raises THAT handle, and the child is given `--window=<handle>:
// <owner>`, which it verifies and binds and never substitutes.
//
// # And it is a HANDOFF, which is said plainly
//
// The selected window receives the foreground at the start of the run. It is not confinement of
// every subsequent keystroke: Marco does not own the desktop, and a window that takes the
// foreground afterwards gets what is typed next. A guarded routine REFUSES at that point, because
// its presses are checked against the window they were recorded in; an unguarded one does not,
// and the sentence this surface shows says which.
//
// See [[ADR-272-the-window-a-run-acts-in-is-chosen-and-carried]].

import (
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
)

// PlayInto is which window a routine would be played into, as the panel beside Play shows it.
//
// NOT `Target`, which the ask box already uses for an APPLICATION somebody states. Two different
// questions and two different terms: one names a program, this names one window of one.
type PlayInto struct {
	// Windows is every window this routine could be played into, each `ID` the term the
	// page hands back. Empty when there is nowhere to play it.
	Windows []PlayWindow `json:"windows"`
	// Chosen is the one selected, or empty when the person has to choose. Set
	// automatically ONLY when there is exactly one candidate: picking one out of several
	// would be this window deciding which of somebody's two browser profiles a routine
	// presses into.
	Chosen string `json:"chosen,omitempty"`
	// Choose is the sentence asking for a choice, set only while there is one to make.
	// What pressing Play would hand over belongs to a WINDOW and is on the row, not here
	// — see PlayWindow.Says.
	Choose string `json:"choose,omitempty"`
	// Why is why there is nowhere to play this routine, and Play is unavailable while it
	// is set. A sentence rather than a dead button: the remedy is to open the
	// application, and nothing else in the window would say so.
	Why string `json:"why,omitempty"`
}

// PlayWindow is one window a routine could be played into, with what choosing it hands over.
//
// # Why the sentence is on the ROW and not on the list
//
// Because it names the window. It was one sentence on the list, composed for whichever window the
// backend had selected — and when somebody then chose the OTHER window, the page went on showing
// the first one's sentence: the right words about the wrong window, which is worse than no words.
// A sentence that belongs to a window travels with it.
type PlayWindow struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Says is what Play hands over IF THIS WINDOW is the one chosen: the window by name,
	// and what does and does not hold afterwards. See handoffSays.
	Says string `json:"says"`
}

// PlaybackTarget lists the windows one routine could be played into, now.
//
// ASKED FRESH EVERY TIME, like the recording chooser and for the same reason: a window list is the
// one thing that cannot be cached, because the person is about to open the application they mean
// to press in.
//
// It writes nothing, decides nothing durable, and is not a grant: `PlayRoutine` is still the
// press, and it resolves the window again at that instant.
func (a *Acts) PlaybackTarget(id string) PlayInto {
	chosen, ok := a.routineWithID(strings.TrimSpace(id))
	if !ok {
		return PlayInto{Why: "choose a play first"}
	}
	cands := a.playbackCandidates(chosen.App)
	out := PlayInto{Windows: []PlayWindow{}}
	for _, p := range cands {
		out.Windows = append(out.Windows, PlayWindow{ID: p.Term(),
			Label: p.Label(), Says: handoffSays(chosen, p.Label())})
	}
	switch {
	case len(cands) == 0:
		out.Why = noWindowFor(chosen)
	case len(cands) == 1:
		// ONE CANDIDATE, SELECTED, AND STILL SHOWN. The person is told which
		// window it is rather than discovering it from where the presses landed.
		out.Chosen = cands[0].Term()
	default:
		out.Choose = fmt.Sprintf("%s has %d windows open. Choose the one to "+
			"press in.", nameOr(chosen.App, "This application"), len(cands))
	}
	return out
}

// playbackCandidates is every window this routine could be played into.
//
// For a routine that names an application, the windows OF that application: a routine recorded in
// Notepad has nothing to say to a browser window, and offering one would be offering a press that
// its own guard refuses. For a routine that names none, every window — because that is exactly
// what "works anywhere, switching nothing" means, and the question of which one it acts in is then
// the whole of the choice.
func (a *Acts) playbackCandidates(app string) []orchestrator.Pickable {
	d := orchestrator.Deps{OwnerPID: uint32(os.Getpid())}
	all := d.Pickable()
	if app == "" {
		return all
	}
	var out []orchestrator.Pickable
	for _, p := range all {
		if sameProgramAs(p.App, app) {
			out = append(out, p)
		}
	}
	return out
}

// sameProgramAs says whether a window's executable is the application a routine is about.
//
// The same containment comparison `winctx.Bind` makes, in both directions, because a routine says
// `notepad` and a window's image is `notepad.exe` — and a stricter rule here would hide the window
// a routine was recorded in from the chooser for that routine.
func sameProgramAs(image, app string) bool {
	image, app = strings.ToLower(strings.TrimSpace(image)), strings.ToLower(strings.TrimSpace(app))
	if image == "" || app == "" {
		return false
	}
	return strings.Contains(image, app) || strings.Contains(app, image)
}

// noWindowFor is why a routine cannot be played, in words that say what would fix it.
func noWindowFor(r Routine) string {
	if r.App != "" {
		return fmt.Sprintf("%s has no window open on this screen, so there is nothing "+
			"for this play to press in. Start it and press Refresh.", r.App)
	}
	// NO APPLICATION AND NO WINDOW. The routine presses into whatever is in front, and
	// the only thing in front is Marco — which is never what was meant, and is what this
	// surface used to do silently.
	return "there is no window on this screen other than Marco's own, and this play " +
		"presses into whatever is in front. Open the window you want it to press in."
}

// handoffSays is what choosing ONE window hands over, as the row beside it says.
//
// # Why it says what it does NOT promise
//
// Because the chosen window gets the foreground at the START of the run and nothing confines what
// happens after. Marco does not own the desktop: a window that takes the foreground mid-run gets
// what is typed next. A sentence reading "it will press in that window" would be a promise about
// the whole run, which only a guarded routine's own boundary can keep — so the guarded case says
// the presses are checked and stop, and the unguarded case says they are not.
func handoffSays(r Routine, which string) string {
	says := fmt.Sprintf("Run brings %s to the front and presses there.", which)
	if r.Contract == string(orchestrator.Guarded) {
		return says + " Its presses are checked against that window and stop if " +
			"something else comes forward."
	}
	return says + " Nothing after that is confined to it: its presses are not checked, " +
		"so whatever is in front at the time receives them."
}

// theChosenWindow is the window a Play names, verified against this routine, now.
//
// RESOLVED AGAIN AT THE PRESS and never taken on trust from the page. The list the page was
// showing is as old as the last time it asked, and between then and the click a window can close
// or its handle be handed back out — so the term is looked up in the list as it is at this
// instant, and a term naming nothing there is a refusal rather than a handle passed on to a child.
func (a *Acts) theChosenWindow(r Routine, window string) (orchestrator.Pickable, string) {
	window = strings.TrimSpace(window)
	cands := a.playbackCandidates(r.App)
	if window == "" {
		// NO WINDOW MEANS NO PLAY. Not "press into whatever is in front", which is
		// what this surface did and is the defect: with Studio in front, that is
		// Studio.
		if len(cands) == 1 {
			// EXCEPT WHERE THERE IS NOTHING TO CHOOSE. The page selects the
			// single candidate and shows it; a Play that arrived without it is
			// the same decision reached by a page that had not repainted yet,
			// and refusing it would be refusing the only answer there is.
			return cands[0], ""
		}
		if len(cands) == 0 {
			return orchestrator.Pickable{}, noWindowFor(r)
		}
		return orchestrator.Pickable{}, fmt.Sprintf("there are %d windows this "+
			"play could press in — choose which one before playing it",
			len(cands))
	}
	for _, p := range cands {
		if p.Term() == window {
			return p, ""
		}
	}
	return orchestrator.Pickable{}, "the window chosen for this play is no longer " +
		"open, or is no longer that window — press Refresh, choose it again, and " +
		"press Run"
}
