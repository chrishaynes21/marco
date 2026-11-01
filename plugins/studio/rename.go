package main

// A ROUTINE CAN BE CALLED SOMETHING ELSE, FROM ITS OWN CARD.
//
// # It is the move flow, and almost none of it is new
//
// A rename is a move whose destination differs by SLUG instead of by scope, so it reuses this
// window's move machinery whole: `keepMove` holds the preparation under its own token,
// `CancelMove` lets go of it, and `MoveRoutine` performs it with the one engine that takes both
// locks in a fixed order, revalidates both ends against the digests the review was built from,
// reads the destination back BEFORE removing the original, and reports a destination written with
// the original still there as TWO ROUTINES rather than as a move.
//
// # The three things that are its own
//
// THE ARGUMENTS ARE A CHIP AND NOT PART OF THE BOX. The name is where an argument is declared —
// `cloud pull stone with item` — so a box holding the whole name is a way to drop one by
// retyping, and the routine would then type the literal text of the demonstration the next time
// it ran. The box holds the base name; `orchestrator.PrepareRename` takes the clause off the
// routine's current name and puts it back.
//
// A COLLISION IS REFUSED AND THERE IS NOTHING TO AGREE TO. `MoveRoutine` can replace what is at
// the destination under a version-bound consent, and a rename offers no such press: two routines
// of one name in one place IS the collision, and the answer to it is a different name. So
// `RenameRoutine` takes the token alone — the page has no way to say "replace" — and the
// preparation has already turned an occupied destination into a refusal.
//
// AND A ROUTINE THAT READS A CREDENTIAL UNDER ITS OWN NAME CANNOT BE RENAMED AT ALL. That is
// decided in the orchestrator, where the two sources are; this surface shows the sentence.
//
// See [[ADR-285-a-routine-can-be-called-something-else]].

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// ReviewRename renders a saved routine under another name, for review.
//
// IT WRITES NOTHING. `name` is the base name only — what the box holds — and the clause that
// declares the routine's arguments is carried by the backend rather than by the page.
//
// The result is a `Move`, because it IS one: the panel shows the same two sources, the same
// destination identity and the same refusals, and `RenameRoutine` performs it through the same
// engine. What it never carries is `Taken`: an occupied destination arrives here as `Refused`.
func (a *Acts) ReviewRename(id, name string) Move {
	id = strings.TrimSpace(id)
	if id == "" {
		return Move{Why: "choose a routine to rename"}
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
		return Move{ID: id, Why: fmt.Sprintf("there is no routine %q in this library "+
			"(%s) — press Refresh and choose it again", id, a.routesDir)}
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	m, err := d.PrepareRename(chosen.route(), strings.TrimSpace(name))
	if err != nil {
		return Move{ID: id, Name: chosen.Name, From: chosen.ID, Was: chosen.Source,
			Clause: clauseOf(chosen.Source), Why: err.Error()}
	}
	// THE PREPARATION STAYS HERE, UNDER ITS OWN TOKEN — see rescope.go.
	a.keepMove(m)

	out := Move{ID: id, From: chosen.ID, To: orchestrator.RouteID(m.To), Token: m.Token,
		Name: m.Name, Renaming: m.Renaming, Clause: clauseOf(m.Was),
		Refused: m.Refused, Was: m.Was, Now: m.Now, Changes: m.Was != m.Now,
		Movable: m.Movable()}
	if m.Refused != "" {
		// NOWHERE TO GO, SO NOTHING IS CLAIMED ABOUT A DESTINATION — the same
		// reason `ReviewRescope` clears it: `RouteID` of a zero route is a sentence
		// about a place that does not exist.
		if m.To.Slug == "" {
			out.To = ""
		}
		out.Movable = false
	}
	return out
}

// RenameRoutine performs one reviewed rename.
//
// THE TOKEN ALONE. `MoveRoutine` takes a second argument — the separate yes an occupied
// destination needs — and a rename has no such yes to give: a collision is refused at
// preparation, so there is no press here that could take one. Passing `false` is not a default
// chosen at this call site; it is the absence of an option, and writing it as its own function
// is what stops a later caller supplying one.
func (a *Acts) RenameRoutine(token string) MoveDone {
	return a.MoveRoutine(strings.TrimSpace(token), false)
}

// clauseOf is what a routine asks for, written the way a name declares it — `with item` — or ""
// when it asks for nothing.
//
// FROM THE SOURCE AND NOT FROM THE NAME. A saved routine's header does NOT carry the `with …`
// clause: the declaration lives in the steps, as `{{item}}` in the text of a step that types, and
// `routes.Arguments` is what reads it. A chip built out of the name would be empty for every
// routine in the library, which is a chip that says a routine asks for nothing whenever it does.
//
// SHOWN BESIDE THE BOX AND NOT IN IT. It is what a rename KEEPS, and somebody about to retype a
// name is owed the part they are not being asked to retype — otherwise the only evidence that
// the values survived is the routine still working afterwards.
func clauseOf(src string) string {
	var names []string
	for _, a := range routes.Arguments(src) {
		names = append(names, a.Name)
	}
	if len(names) == 0 {
		return ""
	}
	return "with " + strings.Join(names, ", ")
}
