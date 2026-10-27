package main

// REVIEWING A REBUILT VERSION OF A SAVED ROUTINE, AND REPLACING IT ONLY IF SOMEBODY SAYS SO.
//
// A saved routine keeps the rules it was generated with, and the way to give it today's is to
// render its own stored demonstration again. That replaces a file somebody is relying on, so this
// surface is four steps and the first three of them write nothing:
//
//	the card says which generation wrote it, and what that routine was written without
//	Review builds a version and shows both sources side by side, with any refusal
//	Cancel drops it — which is the absence of an operation, because nothing was written
//	Replace writes the reviewed source, and refuses if either file moved while it was open
//
// # Why the prepared version is held here and not handed to the page
//
// Because the promise is that what was reviewed is what gets written, and that needs the digests
// of the two files it was built from. A page that handed the source back on the way to replacing
// it would be a page that could hand back something else — and the check would then be against
// whatever arrived, which is not a check. So the window keeps one review per routine, the page
// keeps an identity, and `Replace` is handed nothing but that identity.
//
// # It is not a run, and it does not touch the run slot
//
// Nothing here spawns anything. Studio must not acquire an execution loop of its own and this is
// not one: it reads two files and renders a draft in process. It does hold off while a
// DEMONSTRATION is being recorded, for the reason `Resave` does — the recorder owns the library's
// attention, and a routine replaced underneath a recording in progress is a confusion nobody
// asked for.
//
// See [[ADR-262-a-routine-states-which-generation-wrote-it]].

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// differenceLine is one generator difference as a card prints it.
//
// The attribution and whether it bears on this routine are both in the line, because they are
// what the sentence means: "generation 2 — this routine types text, so this applies to it" is a
// claim, and "generation 2 — a difference between generators" is a comparison. A list that
// printed only the sentences would read as a list of faults.
func differenceLine(d orchestrator.Difference) string {
	if d.Bears {
		return fmt.Sprintf("generation %d: %s — this routine's own contents bear on "+
			"this one", d.Generation, d.Says)
	}
	return fmt.Sprintf("generation %d: %s — nothing in this routine shows whether it "+
		"applies", d.Generation, d.Says)
}

// Review is a rebuilt version of one routine, as the page shows it before anything is written.
//
// NOTHING ON DISK HAS CHANGED when this is returned. If `Why` is set, nothing was even built.
type Review struct {
	// ID is the routine this is about — the same whole identity the card carries, and the
	// term `ReplaceWithRebuilt` is given back.
	ID   string `json:"id"`
	Name string `json:"name"`
	// Why is why there is no review at all: no such routine, no demonstration beside it,
	// a recording that could not be read. Everything else is empty when this is set.
	Why string `json:"why,omitempty"`
	// Was is the saved source and Now the one a replacement would write, both in full.
	//
	// THE WHOLE OF EACH, because the review is of the whole of each. A list of changed
	// lines is an interpretation, and the file is the thing that will run — and for a
	// routine somebody edited by hand, `Was` is their own text, which is what replacing
	// it costs them.
	Was string `json:"was"`
	Now string `json:"now,omitempty"`
	// Refused is why today's rules produce no routine from this recording, in the
	// generator's own words. A reviewable answer and not an error: a recording an older
	// build saved happily can contain a press that cannot be stopped at.
	Refused string `json:"refused,omitempty"`
	// Changes is whether replacing would change the file at all, and Replaceable whether
	// there is anything to write. A routine already written by today's generator rebuilds
	// byte for byte, and saying so beats leaving somebody to compare two identical panes.
	Changes     bool `json:"changes"`
	Replaceable bool `json:"replaceable"`
	// WasGeneration and NowGeneration are the two readings, for the two panes.
	WasGeneration string `json:"was_generation,omitempty"`
	NowGeneration string `json:"now_generation,omitempty"`
	// Differences is what the saved routine was written without, each line saying whether
	// this routine's contents bear on it. See differenceLine.
	Differences []string `json:"differences,omitempty"`
}

// ReviewRebuilt builds a version of one routine from its own stored demonstration, for review.
//
// IT WRITES NOTHING, and it is the only way to reach `ReplaceWithRebuilt`: the review is what
// that call replaces with, and a replacement nobody reviewed is the thing this flow exists to
// prevent.
func (a *Acts) ReviewRebuilt(id string) Review {
	id = strings.TrimSpace(id)
	if id == "" {
		return Review{Why: "choose a routine to review"}
	}
	a.mu.Lock()
	recording := a.recordingLocked()
	a.mu.Unlock()
	if recording {
		return Review{ID: id, Why: "Marco is recording a demonstration; one thing at " +
			"a time"}
	}
	chosen, ok := a.routineWithID(id)
	if !ok {
		return Review{ID: id, Why: fmt.Sprintf("there is no routine %q in this "+
			"library (%s) — press Refresh and choose it again", id, a.routesDir)}
	}
	// AND NOTHING IS OFFERED FOR A GENERATION THIS BUILD CANNOT EXPLAIN.
	//
	// The same answer the button is greyed out by — `Routine.Offer`, which reads
	// `orchestrator.StatedGeneration.Offerable` — asked again here so the rule holds
	// however this is reached. A routine written by a later build is left exactly as it
	// is: it stays readable and playable, and what is withheld is only the offer.
	if !chosen.Offer {
		return Review{ID: id, Name: chosen.Name, Was: chosen.Source,
			WasGeneration: chosen.Generation, Why: chosen.Wrote}
	}
	rt := chosen.route()
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	p, err := d.Prepare(rt)
	if err != nil {
		return Review{ID: id, Why: err.Error()}
	}
	// THE PREPARED VERSION STAYS HERE. See the note at the top of this file: the digests
	// in it are what makes the replacement a check rather than a gesture.
	a.mu.Lock()
	if a.reviews == nil {
		a.reviews = map[string]orchestrator.Prepared{}
	}
	a.reviews[id] = p
	a.mu.Unlock()

	r := Review{ID: id, Name: p.Name, Was: p.Was, Now: p.Now, Refused: p.Refused,
		Changes: p.Changes(), Replaceable: p.Replaceable(),
		WasGeneration: string(p.WasGeneration.Reading),
		NowGeneration: string(p.NowGeneration.Reading)}
	for _, dd := range p.Differences {
		r.Differences = append(r.Differences, differenceLine(dd))
	}
	return r
}

// CancelReview drops a review without writing anything.
//
// There is nothing to undo — preparation wrote nothing — so this only lets go of the version held
// in the window. It exists so that cancelling is a thing the page DOES rather than a thing it
// omits: a review left in memory would be replaced with later, by a second press of a button the
// person had already dismissed.
func (a *Acts) CancelReview(id string) {
	a.mu.Lock()
	delete(a.reviews, strings.TrimSpace(id))
	a.mu.Unlock()
}

// ReplaceWithRebuilt writes the reviewed version over the routine it was built from.
//
// It takes only the identity. What gets written is the source the review held, which is the
// source that was on the screen — and it refuses if either the routine or its demonstration has
// changed since, naming which, because the review was of a particular pair of files.
func (a *Acts) ReplaceWithRebuilt(id string) string {
	id = strings.TrimSpace(id)
	a.mu.Lock()
	recording := a.recordingLocked()
	p, reviewed := a.reviews[id]
	a.mu.Unlock()
	switch {
	case recording:
		return "Marco is recording a demonstration; one thing at a time"
	case !reviewed:
		// NOTHING IS WRITTEN WITHOUT A REVIEW IN HAND. Not built-and-written here
		// as a convenience: the whole point of the flow is that the source which
		// replaces somebody's routine is one they looked at.
		return fmt.Sprintf("there is no reviewed version of %q to replace it with — "+
			"review it first", id)
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	if err := d.Replace(p); err != nil {
		return err.Error()
	}
	a.mu.Lock()
	delete(a.reviews, id)
	a.mu.Unlock()
	a.trial.recorded(p.Name, fmt.Sprintf("Replaced %q with the version rebuilt from its "+
		"own recording, which you reviewed first: it now states generation %d of "+
		"Marco's rules — %s", p.Name, orchestrator.GenerationNow(),
		a.reg().Path(p.Route)), "Run it.", false, Pictured{})
	return ""
}

// routineWithID is the one routine whose whole identity is this, or not found.
//
// By the WHOLE identity and never by name or slug, for the reason `PlayRoutine` is: a name is not
// unique, two routines called `settings` in two applications are two routines, and a lookup that
// took the first that answered would review one and replace the other. The listing is the one
// this surface already produces, so a routine this cannot reach is one no card offered.
func (a *Acts) routineWithID(id string) (Routine, bool) {
	for _, r := range a.Routines() {
		if r.ID == id {
			return r, true
		}
	}
	return Routine{}, false
}

// route is one listing row as the registry's own identity.
//
// One place, because a `Routine` is a card and a `routes.Route` is a file, and a second
// hand-written conversion between them is a second chance to drop `Focus` — which would be a
// review of `<app>/focus/settings` and a replacement of `<app>/context/settings`, the exact
// confusion `Routine.ID` exists to prevent.
func (r Routine) route() routes.Route {
	return routes.Route{Slug: r.Slug, App: r.App, Focus: r.Focus}
}
