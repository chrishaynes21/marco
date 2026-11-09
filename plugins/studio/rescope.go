package main

// CHANGING WHERE A ROUTINE WORKS, FROM THE PANEL THAT SHOWS IT.
//
// Where a routine works is its folder, so this is a MOVE between two identities and not a save
// under another word: `routes/<app>/context/<slug>.marco`, `routes/<app>/focus/<slug>.marco` and
// `routes/global/<slug>.marco` are three files. The whole contract is in
// `internal/orchestrator`'s `PrepareRescope` and `Rescope` — both locks, both revalidations, the
// exclusive create, the destination read back, the removal that takes the source first — and what
// is here is this window's interaction with it, in four steps of which the first three write
// nothing:
//
//	the panel lists the three availability choices, and says why an unavailable one is
//	Review renders the routine for the chosen scope and shows both sources
//	Cancel drops it — the absence of an operation, because nothing was written
//	Move writes the destination, removes the original, and says which of those happened
//
// # Why the prepared move is held here and the page gets a token
//
// The same reason `generation.go` holds a review: the promise is that what gets written is what
// was on the screen, and that needs the four digests the review was bound to — the original's
// source and demonstration, and whatever was at the destination. A page that handed the rendered
// source back could hand back something else, and a check against whatever arrived is not a
// check.
//
// AND THE CONSENT TO REPLACE AN OCCUPIED DESTINATION IS BUILT HERE, NOT THERE. The page says one
// thing — whether to replace what it was shown — and the version that consent is bound to is the
// one this recorded when it prepared the review. Studio's older `ReplaceDraft` takes digests from
// the page, which works because it compares them against disk at the write; it still leaves the
// page as the author of a value whose whole job is to say what somebody saw. Here the page cannot
// author it at all.
//
// # It is not a run, and nothing here spawns anything
//
// Two files are read and a draft is rendered in process. It holds off while a demonstration is
// being recorded, for the reason every other write here does: the recorder owns the library's
// attention.
//
// See [[ADR-271-a-rescope-is-a-reviewed-move-between-identities]].

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
)

// ScopeChoice is one of the three places a routine can be made available, as the panel offers it.
//
// ALL THREE ARE ALWAYS LISTED, including the ones that cannot be chosen. A chooser that showed
// only what is possible would answer the question somebody is actually asking — "why can this not
// work from anywhere?" — by removing the row that would have carried the answer.
type ScopeChoice struct {
	// Scope is the word, and what comes back when somebody picks this.
	Scope string `json:"scope"`
	// Where is what this choice means for a run, and Asked where the routine could be
	// asked for by name under it. Two facts, because they are two: `resolveroute.Eligible`
	// offers an in-place routine only while its application is in front, which is the half
	// somebody discovers a week later with a library that looks like it has lost
	// something.
	Where string `json:"where"`
	Asked string `json:"asked"`
	// Current is the scope it already has, which is not a choice: selecting it is nothing
	// to do, and `PrepareRescope` refuses it rather than performing one lock taken twice
	// and a removal of what was just written.
	Current bool `json:"current,omitempty"`
	// Available is whether a move there can be prepared at all, and Why the reason when it
	// cannot — in the words of whatever refused it.
	//
	// The reasons are real and they are specific: a routine whose presses check the window
	// they were made in cannot go to a scope that brings no window forward, and a routine
	// that is about no application cannot go to one that needs one until somebody names
	// it. Neither is rephrased here. See orchestrator.WhyNotThisScope and rescopeTarget.
	Available bool   `json:"available"`
	Why       string `json:"why,omitempty"`
	// Taken says something is already saved where this choice would put it, and TakenSays
	// is that in a sentence. Shown with the choice rather than discovered after it: moving
	// this routine is not agreeing to lose that one.
	Taken     bool   `json:"taken,omitempty"`
	TakenSays string `json:"taken_says,omitempty"`
}

// Move is one prepared rescope, as the page shows it before anything is written.
//
// NOTHING ON DISK HAS CHANGED when this is returned. If `Why` is set, nothing was even rendered.
type Move struct {
	// ID is the routine this is about, From its identity now and To where it would go.
	// All three, because the move is between two identities and the panel shows both.
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to,omitempty"`
	// Token identifies THIS preparation and is the term `MoveRoutine` and `CancelMove` are
	// given back. Not the routine's identity, which two preparations of one routine share:
	// a window can hold two and the page can be showing either. Empty when `Why` is set.
	Token string `json:"token,omitempty"`
	Name  string `json:"name"`
	// Renaming is what the routine would be CALLED, for a move that is a rename, and
	// Clause the `with …` part of the name it has now — which a rename KEEPS, and which
	// the page shows BESIDE the box rather than in it. The name is where an argument is
	// declared, so a box holding the whole name is a way to drop one by retyping.
	//
	// Both empty for a move between scopes, which changes neither. See rename.go.
	Renaming string `json:"renaming,omitempty"`
	Clause   string `json:"clause,omitempty"`
	// Scope is the word asked for, Where what it means for a run, and Asked where the
	// routine could then be asked for by name. The same three fields a `ScopeChoice`
	// carries, and the same sentences, because the review is of the choice that was made.
	Scope string `json:"scope"`
	Where string `json:"where,omitempty"`
	Asked string `json:"asked,omitempty"`
	// Why is why there is no review at all: no such routine, a scope word nobody
	// recognises, the scope it already has, a demonstration that is gone. Everything else
	// is empty when this is set.
	Why string `json:"why,omitempty"`
	// Refused is why this move cannot be made, with the review still worth showing: a
	// scope these presses cannot be performed under, a rendering today's rules will not
	// write, a destination that needs an application nobody named.
	Refused string `json:"refused,omitempty"`
	// Was is the source as it stands — hand edits and all — and Now the source a move
	// would write. Both in full, and `Was` is the half that says what a hand edit costs:
	// a move re-renders from the demonstration, so approving it is approving the loss of
	// the edit, in front of them rather than afterwards.
	Was string `json:"was"`
	Now string `json:"now,omitempty"`
	// Changes is whether the two differ at all. A routine whose rendering is identical
	// still MOVES — the file goes to another folder, which is the whole point — so this is
	// about the source and never about whether the move is worth making.
	Changes bool `json:"changes"`
	// Movable is whether there is something a move could write.
	Movable bool `json:"movable"`
	// Taken says something is already at the destination, TakenID its whole identity and
	// TakenSays the sentence. A move onto it needs a separate yes, and that yes is bound
	// to the version this preparation saw — which is why the page sends a boolean and not
	// a digest. See MoveRoutine.
	Taken     bool   `json:"taken,omitempty"`
	TakenID   string `json:"taken_id,omitempty"`
	TakenSays string `json:"taken_says,omitempty"`
}

// MoveDone is what a move did, as the page says it.
//
// THREE OUTCOMES AND THEY ARE NOT ONE. Nothing written is a refusal; a destination written with
// the original gone is a move; a destination written with the original still there is two
// routines, and calling that a move would send somebody looking for one of them.
type MoveDone struct {
	// Why is why nothing was written. Everything else is empty when this is set.
	Why string `json:"why,omitempty"`
	// Moved is whether the original is gone, and Copied its complement: the destination
	// exists in both, which is the fact that makes the distinction worth drawing.
	Moved  bool `json:"moved,omitempty"`
	Copied bool `json:"copied,omitempty"`
	// To is where the routine now is, which the page needs in order to select it again:
	// the identity it was showing no longer exists after a move.
	To string `json:"to,omitempty"`
	// Says is the outcome in a sentence, and Left what could not be swept from the old
	// place. Two sentences, kept apart, because they are two things to be told — see
	// orchestrator.Rescoped.Left.
	Says string `json:"says,omitempty"`
	Left string `json:"left,omitempty"`
}

// movesKept is how many preparations one window holds at once, for the reason `reviewsKept` is
// that many: every press of Review makes one, nothing obliges anybody to act on it, and an
// evicted one is a REFUSAL with a reason rather than a wrong write.
const movesKept = 16

// ScopeChoices is the three availability choices for one saved routine, each with whether it can
// be chosen and why not.
//
// IT WRITES NOTHING, and it keeps nothing: the availability of a choice is asked again when
// somebody reviews it, under the locks, by the same code. This is the sentence beside the control.
//
// `app` is used only by a routine that is about no application and is being given one. A routine
// that already has one keeps it: a move changes where a routine works and does not get to change
// which application it is about on the way past.
func (a *Acts) ScopeChoices(id, app string) []ScopeChoice {
	out := []ScopeChoice{}
	chosen, ok := a.routineWithID(strings.TrimSpace(id))
	if !ok {
		return out
	}
	rt := chosen.route()
	already := string(orchestrator.ScopeAlreadyOf(rt))
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	for _, scope := range []orchestrator.Scope{orchestrator.ScopeApp,
		orchestrator.ScopeAppAnywhere, orchestrator.ScopeEverywhere} {
		c := ScopeChoice{Scope: string(scope)}
		if string(scope) == already {
			// THE ONE IT HAS. Said as the current state rather than offered as a
			// choice, and not asked of the backend at all: `PrepareRescope`
			// refuses it, and rendering that refusal as a reason this scope is
			// unavailable would read as a fault in the routine.
			c.Current = true
			c.Where, c.Asked = scopeMeans(scope, chosen.App)
			out = append(out, c)
			continue
		}
		// ASKED OF THE BACKEND, ONE CHOICE AT A TIME, rather than worked out here.
		// Whether a scope is possible for this routine is `WhyNotThisScope` and
		// `rescopeTarget`, and a page offering what the backend then declines is a
		// window telling somebody off for pressing what it offered them.
		m, err := d.PrepareRescope(rt, scope, strings.TrimSpace(app))
		which := chosen.App
		if m.To.App != "" {
			which = m.To.App
		}
		c.Where, c.Asked = scopeMeans(scope, which)
		switch {
		case err != nil:
			c.Why = err.Error()
		case m.Refused != "":
			c.Why = m.Refused
		default:
			c.Available = true
			c.Taken, c.TakenSays = m.Taken, m.TakenSays
		}
		out = append(out, c)
	}
	return out
}

// scopeMeans is what one scope means for a run, and where the routine could be asked for by name
// under it.
//
// THE WORDS ARE HERE AND NOT IN THE PAGE, for the reason every other sentence in this window is
// the backend's: a page with them typed into it goes on promising whatever it said after the rule
// moves. They say the same thing as `whereItWorks` and `askedFor` next door, because two
// phrasings of one scope is how somebody comes to believe they are looking at two routines.
func scopeMeans(scope orchestrator.Scope, app string) (where, asked string) {
	if app == "" {
		app = "the application you name"
	}
	switch scope {
	case orchestrator.ScopeAppAnywhere:
		return "brings " + app + " forward, then presses",
			"asked for by name from anywhere"
	case orchestrator.ScopeEverywhere:
		return "presses into whatever is in front, switching nothing",
			"asked for by name from anywhere"
	default:
		return "presses only while " + app + " is in front, switching nothing",
			"asked for by name only while " + app + " is in front"
	}
}

// ReviewRescope renders one routine for another scope, for review.
//
// IT WRITES NOTHING, and it is the only way to reach `MoveRoutine`: the preparation it holds is
// what that call moves with, and a move nobody reviewed is the thing this flow exists to prevent.
func (a *Acts) ReviewRescope(id, scope, app string) Move {
	id = strings.TrimSpace(id)
	if id == "" {
		return Move{Why: "choose a play to move"}
	}
	a.mu.Lock()
	recording := a.recordingLocked()
	a.mu.Unlock()
	if recording {
		return Move{ID: id, Why: "Marco is recording a demonstration; one thing at a " +
			"time"}
	}
	chosen, ok := a.routineWithID(id)
	if !ok {
		return Move{ID: id, Why: fmt.Sprintf("there is no play %q in this library "+
			"(%s) — press Refresh and choose it again", id, a.routesDir)}
	}
	// WHICH SCOPE, STATED BY THE PAGE AND CHECKED BELOW. `PrepareRescope` refuses a word
	// it cannot name, so an unknown one arrives as a review that says why rather than as a
	// review of something else.
	want := orchestrator.Scope(strings.TrimSpace(scope))
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	m, err := d.PrepareRescope(chosen.route(), want, strings.TrimSpace(app))
	if err != nil {
		return Move{ID: id, Name: chosen.Name, From: chosen.ID, Scope: string(want),
			Was: chosen.Source, Why: err.Error()}
	}
	// THE PREPARED MOVE STAYS HERE, UNDER ITS OWN TOKEN. See the note at the top of this
	// file and `orchestrator.Rescoping.Token`.
	a.keepMove(m)

	which := chosen.App
	if m.To.App != "" {
		which = m.To.App
	}
	where, asked := scopeMeans(want, which)
	out := Move{ID: id, From: chosen.ID, To: orchestrator.RouteID(m.To), Token: m.Token,
		Name: m.Name, Scope: string(want), Where: where, Asked: asked,
		Refused: m.Refused, Was: m.Was, Now: m.Now, Changes: m.Was != m.Now,
		Movable: m.Movable(), Taken: m.Taken, TakenID: m.TakenID,
		TakenSays: m.TakenSays}
	if m.Refused != "" {
		// NOWHERE TO GO, SO NOTHING IS CLAIMED ABOUT A DESTINATION. A refused
		// preparation can have no target at all — a global routine with no
		// application to give it — and `RouteID` of a zero route is a sentence
		// about a place that does not exist.
		if m.To.Slug == "" {
			out.To = ""
		}
		out.Movable = false
	}
	return out
}

// keepMove holds one preparation, oldest out first.
func (a *Acts) keepMove(m orchestrator.Rescoping) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.moves == nil {
		a.moves = map[string]orchestrator.Rescoping{}
	}
	if _, already := a.moves[m.Token]; !already {
		a.moveOrder = append(a.moveOrder, m.Token)
	}
	a.moves[m.Token] = m
	for len(a.moveOrder) > movesKept {
		delete(a.moves, a.moveOrder[0])
		a.moveOrder = a.moveOrder[1:]
	}
}

// CancelMove drops one prepared move without writing anything.
//
// There is nothing to undo — preparation wrote nothing — so this only lets go of what the window
// holds. It exists so that cancelling is something the page DOES rather than something it omits:
// a preparation left in memory would be moved with later, by a second press of a button the
// person had already dismissed.
func (a *Acts) CancelMove(token string) {
	token = strings.TrimSpace(token)
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.moves, token)
	for i, t := range a.moveOrder {
		if t == token {
			a.moveOrder = append(a.moveOrder[:i], a.moveOrder[i+1:]...)
			break
		}
	}
}

// MoveRoutine performs one reviewed move.
//
// It takes the REVIEW's token and one boolean, and that is the whole of what the page contributes.
//
// `replace` is the separate yes an occupied destination needs, and the version it is bound to is
// the one this window recorded when it prepared the review — never one the page hands back. The
// page was shown `TakenSays`, which names the destination and what is there; what it sends is
// agreement with what it was shown. If something else has been saved there since, the
// revalidation under the lock refuses with nothing written.
//
// Without that yes the destination is created EXCLUSIVELY, so a routine appearing between the
// review and the write is the failure rather than something written over.
func (a *Acts) MoveRoutine(token string, replace bool) MoveDone {
	token = strings.TrimSpace(token)
	a.mu.Lock()
	recording := a.recordingLocked()
	m, prepared := a.moves[token]
	a.mu.Unlock()
	switch {
	case recording:
		return MoveDone{Why: "Marco is recording a demonstration; one thing at a time"}
	case !prepared:
		// NOTHING IS MOVED WITHOUT THAT PREPARATION IN HAND. Not the routine's
		// most recent one, and not one made here as a convenience: the source that
		// lands at the destination is the one that was on the screen.
		return MoveDone{Why: "the move you are looking at is no longer the one this " +
			"window holds — review it again and the comparison will be against " +
			"the play as it now stands"}
	}
	// ── THE CONSENT, BUILT FROM WHAT WAS SHOWN ──
	//
	// Version-bound and assembled here. `orchestrator.Consent` names the destination's
	// identity and digests of the source and demonstration somebody was shown; all three
	// are in the preparation, which is what the page was rendered from. A consent the page
	// could author would be a check against whatever arrived.
	var consent orchestrator.Consent
	if replace && m.Taken {
		consent = m.ConsentToReplace()
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	done, err := d.Rescope(m, consent)
	if err != nil {
		// THE PREPARATION IS KEPT. Every refusal here names something that moved or
		// is in the way, and the way forward is to look again — which needs the
		// review still on the screen to press the button beside.
		return MoveDone{Why: err.Error()}
	}
	// SPENT. The destination has been written, so a second press is not a second move.
	a.CancelMove(token)
	a.trial.recorded(m.Name, fmt.Sprintf("%s %s", done.Says, func() string {
		if done.Removed {
			return "It is no longer at " + orchestrator.RouteID(m.From) + "."
		}
		return "It is still at " + orchestrator.RouteID(m.From) + " as well."
	}()), "Run it where it is now.", false, Pictured{})
	return MoveDone{Moved: done.Removed, Copied: !done.Removed,
		To: orchestrator.RouteID(done.To), Says: done.Says, Left: done.Left}
}
