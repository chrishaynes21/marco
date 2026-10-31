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
	// ID is the routine this is about — the same whole identity the card carries.
	ID string `json:"id"`
	// Token identifies THIS review, and is the term `ReplaceWithRebuilt` and
	// `CancelReview` are given back.
	//
	// NOT THE ROUTINE'S IDENTITY, which two reviews of one routine share. A window can
	// have two preparations in flight and the page can be showing one while the window
	// holds the other; replacing by routine wrote whichever arrived last, which is bytes
	// the person never saw under the name of the routine they were looking at. Empty when
	// `Why` is set, because then there is no review to replace with.
	// See `orchestrator.Prepared.Token`.
	Token string `json:"token,omitempty"`
	Name  string `json:"name"`
	// Why is why there is no review at all: no such routine, no demonstration beside it,
	// a recording that could not be read. Everything else is empty when this is set.
	Why string `json:"why,omitempty"`
	// Rebuild is which of the two sets of rules produced this review, Choice is the words
	// on the control that asked for it, and Caution is what that choice CHANGES about the
	// routine beyond the generator's own rules.
	//
	// ALL THREE SHOWN, because a review of two sources is not self-explanatory: "a version
	// rebuilt from its recording" means two different things now, and a person comparing
	// two panes has to know which rules made the right-hand one. The caution is the half
	// that cannot be left to the diff — folding a typed run removes the pauses inside it,
	// which is a behaviour change and reads as a tidy-up.
	//
	// The words are the backend's for the same reason every other sentence here is: a page
	// with them typed into it would go on promising whatever it said after the rules moved.
	Rebuild string `json:"rebuild,omitempty"`
	Choice  string `json:"choice,omitempty"`
	Caution string `json:"caution,omitempty"`
	// Discards says a replacement would throw away the routine's STORED STEPS, and
	// StepsSays is the contract's sentence about it.
	//
	// THE CONTRACT WROTE THAT SENTENCE AND THIS WINDOW NEVER ASKED FOR IT.
	// `orchestrator.Prepared.StepsWarning` has existed since step editing did, and the
	// review here carried `Caution` — which is about WHICH REBUILD was chosen — and
	// nothing about the steps. So a routine whose text had been made into a supplied
	// value, rebuilt with typing folded harder, would come back as the recording with
	// the fixed text in it and the argument gone, having said nothing beforehand.
	//
	// Found by a regression test about something else: that edits and rebuilds never
	// render a credential read. They do not — and one of the two arms quietly undoes the
	// edit, which a person is owed before they press Replace rather than after.
	Discards  bool   `json:"discards,omitempty"`
	StepsSays string `json:"steps_says,omitempty"`
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
func (a *Acts) ReviewRebuilt(id, kind string) Review {
	id = strings.TrimSpace(id)
	if id == "" {
		return Review{Why: "choose a routine to review"}
	}
	// WHICH RULES, STATED BY THE PAGE AND CHECKED HERE. There are two choices and
	// `orchestrator.Prepare` refuses a word it cannot name, so an unknown one arrives
	// as a review that says why rather than as a review of something else. Studio
	// never consults `SimplifySaves`: a surface that can show a comparison is not
	// excused from showing it by an environment variable it inherited.
	want := orchestrator.Rebuild(strings.TrimSpace(kind))
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
	p, err := d.Prepare(rt, want)
	if err != nil {
		return Review{ID: id, Name: chosen.Name, Was: chosen.Source, Why: err.Error()}
	}
	// THE PREPARED VERSION STAYS HERE, UNDER ITS OWN TOKEN AND NOT UNDER THE ROUTINE'S.
	//
	// Keyed by the routine, the second review of one routine overwrote the first — and
	// these calls are asynchronous, so the page can be showing A's two sources while the
	// slot holds B. Press Replace and B is written: every check in `Deps.Replace` passes,
	// the success names the routine the person was looking at, and the bytes are ones they
	// never saw. The token identifies the review, so the page hands back the exact one it
	// displayed. See `orchestrator.Prepared.Token`.
	a.keepReview(p)

	r := Review{ID: id, Token: p.Token, Name: p.Name, Was: p.Was, Now: p.Now,
		Rebuild: string(p.Rebuild), Choice: p.Rebuild.Choice(),
		Caution:  p.Rebuild.Caution(),
		Discards: p.DiscardsEdits(), StepsSays: p.StepsWarning(),
		Refused: p.Refused, Changes: p.Changes(), Replaceable: p.Replaceable(),
		WasGeneration: string(p.WasGeneration.Reading),
		NowGeneration: string(p.NowGeneration.Reading)}
	for _, dd := range p.Differences {
		r.Differences = append(r.Differences, differenceLine(dd))
	}
	return r
}

// reviewsKept is how many preparations one window holds at once.
//
// There has to be a bound, because every press of Review makes one and nothing obliges anybody to
// act on it. Generous enough that no real sequence of looking at things loses one, and an evicted
// review is a REFUSAL with a reason rather than a wrong write — `ReplaceWithRebuilt` cannot find
// it and says to review again.
const reviewsKept = 16

// keepReview holds one preparation, oldest out first.
func (a *Acts) keepReview(p orchestrator.Prepared) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.reviews == nil {
		a.reviews = map[string]orchestrator.Prepared{}
	}
	if _, already := a.reviews[p.Token]; !already {
		a.reviewOrder = append(a.reviewOrder, p.Token)
	}
	a.reviews[p.Token] = p
	for len(a.reviewOrder) > reviewsKept {
		delete(a.reviews, a.reviewOrder[0])
		a.reviewOrder = a.reviewOrder[1:]
	}
}

// CancelReview drops one review without writing anything.
//
// There is nothing to undo — preparation wrote nothing — so this only lets go of the version held
// in the window. It exists so that cancelling is a thing the page DOES rather than a thing it
// omits: a review left in memory would be replaced with later, by a second press of a button the
// person had already dismissed.
//
// BY TOKEN, so dismissing one review of a routine does not throw away another that is still on
// the screen.
func (a *Acts) CancelReview(token string) {
	token = strings.TrimSpace(token)
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.reviews, token)
	for i, t := range a.reviewOrder {
		if t == token {
			a.reviewOrder = append(a.reviewOrder[:i], a.reviewOrder[i+1:]...)
			break
		}
	}
}

// ReplaceWithRebuilt writes one reviewed version over the routine it was built from.
//
// It takes the REVIEW's token and nothing else. What gets written is the source that review held,
// which is the source that was on the screen — and it refuses if either the routine or its
// demonstration has changed since, naming which, because the review was of a particular pair of
// files.
func (a *Acts) ReplaceWithRebuilt(token string) string {
	token = strings.TrimSpace(token)
	a.mu.Lock()
	recording := a.recordingLocked()
	p, reviewed := a.reviews[token]
	a.mu.Unlock()
	switch {
	case recording:
		return "Marco is recording a demonstration; one thing at a time"
	case !reviewed:
		// NOTHING IS WRITTEN WITHOUT THAT REVIEW IN HAND. Not the routine's most
		// recent review, and not one built here as a convenience: the whole point
		// of the flow is that the source which replaces somebody's routine is the
		// one they were looking at.
		return "the version you are looking at is no longer the one this window " +
			"holds — review it again and the comparison will be against the " +
			"routine as it now stands"
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	if err := d.Replace(p); err != nil {
		return err.Error()
	}
	// SPENT. The version has been written, so a second press of the same button is not a
	// second replacement — it is a refusal saying to review again, which is true.
	a.CancelReview(token)
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
