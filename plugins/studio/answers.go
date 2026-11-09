package main

// WHAT ELSE A ROUTINE ANSWERS TO, FROM THE CARD THAT SHOWS THE ROUTINE.
//
// The surface over `orchestrator.AddAlias` / `RemoveAlias`, and it is deliberately the smallest
// one that can be used: the phrases listed, one added, one removed by name, and a collision NAMED
// rather than resolved. No reordering (a list of names has no order worth keeping), no editing a
// phrase in place (that is a remove and an add, and doing it as one would have to decide what
// happens when the add is refused and the remove already landed), and nothing that renames the
// routine itself.
//
// # It asks the backend and does not judge
//
// Every refusal on this panel is a sentence the contract produced. A window with its own copy of
// the collision rule is a window that can offer what the backend then declines, which is this
// product telling somebody off for pressing what it offered them — and the collision rule in
// particular is about the whole library, which a card about one routine is the worst possible
// place to re-derive.
//
// # And a removal names the phrase
//
// Not its position in the list. The step panel needed a basis token because a row number means
// nothing without the list it was read off; a phrase is its own identity, so the hazard does not
// exist here rather than being guarded against. See [[Command-aliases-proposal-2026-10-30]].

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
)

// Answers is what a routine answers to, as the panel shows it.
type Answers struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Phrases is what it also answers to, besides its own name, exactly as they were
	// typed.
	Phrases []string `json:"phrases"`
	// Why is why none of this can be offered, when it cannot — no routine chosen, a
	// recording in progress, a phrase file left by a half-finished move, one that cannot
	// be read. Empty when `Phrases` is the answer, INCLUDING when it is empty: a routine
	// answering only to its own name is an ordinary routine and not a problem.
	Why string `json:"why,omitempty"`
	// Askable is whether a phrase may be added, which is false for every `Why` above.
	Askable bool `json:"askable"`
	// Said is what the last add or remove on this routine came to, kept so the panel can
	// show it beside the list it changed.
	Said string `json:"said,omitempty"`
	// Failed is whether `Said` is a refusal rather than an outcome.
	Failed bool `json:"failed,omitempty"`
}

// RoutineAnswers is what the chosen routine also answers to.
func (a *Acts) RoutineAnswers(id string) Answers {
	out, _, _ := a.answersFor(id)
	return out
}

// answersFor is the read every method here starts from: the routine, its phrases, and whether
// anything may be done.
func (a *Acts) answersFor(id string) (Answers, orchestrator.Deps, bool) {
	id = strings.TrimSpace(id)
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	if id == "" {
		return Answers{Phrases: []string{},
			Why: "choose a play to see what it answers to"}, d, false
	}
	a.mu.Lock()
	recording := a.recordingLocked()
	a.mu.Unlock()
	if recording {
		// THE SAME HOLD-OFF THE STEP PANEL TAKES. Nothing here would corrupt a
		// recording, but a window that offers one thing while it is doing another is
		// how somebody comes to save a phrase onto the routine they are replacing.
		return Answers{ID: id, Phrases: []string{},
			Why: "Marco is recording a demonstration; one thing at a time"}, d, false
	}
	chosen, found := a.routineWithID(id)
	if !found {
		return Answers{ID: id, Phrases: []string{},
			Why: fmt.Sprintf("there is no play %q in this library (%s) — press "+
				"Refresh and choose it again", id, a.routesDir)}, d, false
	}
	phrases, why := d.AliasesOf(chosen.route())
	if why != "" {
		return Answers{ID: id, Name: chosen.Name, Phrases: []string{},
			Why: why}, d, false
	}
	if phrases == nil {
		phrases = []string{}
	}
	return Answers{ID: id, Name: chosen.Name, Phrases: phrases, Askable: true},
		d, true
}

// AddRoutineAnswer teaches the chosen routine one more phrase.
//
// The phrase is sent as the person typed it and stored that way. What comes back is the list READ
// BACK from disk, so what the panel shows is what the next request will be read against.
func (a *Acts) AddRoutineAnswer(id, phrase string) Answers {
	out, d, ok := a.answersFor(id)
	if !ok {
		return out
	}
	chosen, _ := a.routineWithID(id)
	said, err := d.AddAlias(chosen.route(), phrase)
	if err != nil {
		// THE REFUSAL IS THE CONTRACT'S OWN SENTENCE, which for a collision names
		// the routine it collides with and says what to do. Reworded here it would
		// be a second vocabulary for one rule.
		out.Failed, out.Said = true, err.Error()
		return out
	}
	out.Phrases, out.Said = said.Answers, said.Says
	if out.Phrases == nil {
		out.Phrases = []string{}
	}
	return out
}

// RemoveRoutineAnswer takes one phrase away, by naming it.
func (a *Acts) RemoveRoutineAnswer(id, phrase string) Answers {
	out, d, ok := a.answersFor(id)
	if !ok {
		return out
	}
	chosen, _ := a.routineWithID(id)
	said, err := d.RemoveAlias(chosen.route(), phrase)
	if err != nil {
		out.Failed, out.Said = true, err.Error()
		return out
	}
	out.Phrases, out.Said = said.Answers, said.Says
	if out.Phrases == nil {
		out.Phrases = []string{}
	}
	return out
}
