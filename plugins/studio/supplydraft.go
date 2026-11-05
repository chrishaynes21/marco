package main

// MARKING A VALUE AS SUPPLIED, FROM THE CARD THAT REVIEWS THE RECORDING.
//
// A routine can be told to ASK for a piece of text instead of typing the text that was
// demonstrated, and the three ways in were all invisible from this window. Two of them need
// foreknowledge: name the recording `cloud pull stone with item` before you start, or press the
// reserved key (F9) at the moment in the demonstration where the value goes. The third — turning
// a typed step into a supplied one — needs none, and until now it was only on a SAVED routine,
// so a person who realised at review had to save, find the routine in the panel, edit it, and
// save again. Two writes and a hunt to say something they knew while they were demonstrating.
//
// The rules are the orchestrator’s: [orchestrator.Deps.SuppliedInDraft] asks `suppliedName`, which is
// the same function [orchestrator.EditArgument] asks of a saved routine’s list. This file is the
// window’s interaction with it and decides nothing about what may be asked.
//
// # One control under the list, rather than one beside every step
//
// The saved routine’s panel puts a box on each row, because that panel exists to operate on rows
// and its rows carry three different operations. This card exists to be READ — it is the
// demonstration, in order, with Save underneath — and a text box threaded through every typed step
// would turn the thing being approved into a form. So the steps stay a list and the control names
// its target: the chooser offers only the steps that type something, each labelled with the text
// it types, which is the same explicitness a per-row box buys.
//
// # Nothing is written, and re-deriving takes the marks off
//
// Like Simplify beside it: the draft under review is swapped for the marked one and Save persists
// whichever is being shown. And because Simplify re-derives the steps from the EVENTS, it throws
// the marks away — there is no honest way to carry a position across a list that folded. That is
// said rather than discovered, in the caption, naming the values that would go back to being
// typed.

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// SupplyChoice is one step whose text could be supplied instead of typed.
//
// `At` is a position in the step list this review is showing, from zero, and `Label` names it the
// way the list does so the chooser and the steps above it cannot be read as two different
// routines.
type SupplyChoice struct {
	At    int    `json:"at"`
	Label string `json:"label"`
}

// supplyChoices is every step of a draft whose text can be made something supplied.
//
// ONLY WHERE IT APPLIES, which is the same rule the saved routine’s panel follows: a chooser
// offering a click or a key press would be a control whose refusal arrives after somebody has
// typed a name into it. A step that is ALREADY supplied is left out for the same reason — it is
// in the list above with its placeholder on show, which is the answer to "can this change".
func supplyChoices(steps []macroir.Step) []SupplyChoice {
	var out []SupplyChoice
	for i, s := range steps {
		if s.Kind != macroir.StepType || s.Text == "" {
			continue
		}
		if routes.Arguments(s.Text) != nil {
			continue
		}
		out = append(out, SupplyChoice{At: i,
			Label: fmt.Sprintf("step %d — types %q", i+1, s.Text)})
	}
	return out
}

// perCharacterTyping reports whether this list types text one keystroke at a time.
//
// Which is a DIFFERENT question from whether anything can be supplied, and the difference is why
// the caption is built from sentences rather than chosen from a list. `isPrintableKey`’s shape: a
// `StepKey` carrying `Text` is one character somebody typed, and a run of them with waits in
// between is the recording as it was MADE.
func perCharacterTyping(steps []macroir.Step) bool {
	for _, s := range steps {
		if s.Kind == macroir.StepKey && s.Text != "" {
			return true
		}
	}
	return false
}

// supplyCaption is what the control says about itself, and it is built from up to three sentences
// rather than chosen between them.
//
// COMPOSED, BECAUSE THE STATES OVERLAP AND A CHOSEN SENTENCE GOT IT WRONG. The first version
// picked the advice below whenever there was nothing to pick, which is also true of a draft whose
// only typed step has just been MARKED — so marking the one value in a recording replaced the
// warning about losing it with a paragraph telling somebody to simplify a thing that is already
// folded. Found by `TestSimplifyingTakesTheSuppliedMarksOffAndSaidSoFirst`, which is the test that
// reads the caption at exactly that moment.
//
// # A VALUE IS ONE STEP, AND A FRESH RECORDING IS ONE STEP PER CHARACTER
//
// The part that would have made the control useless without a word of explanation. A recording as
// it was made holds each keystroke as its own step — `type "s"`, `wait 180ms`, `type "t"` —
// because `coalesceTyping` folds only what is adjacent, and the gaps between somebody’s
// keystrokes are not. The run becomes one `type "stone"` when the list is derived with a typing
// rhythm, which is what Simplify does.
//
// So on a fresh draft there is usually nothing to offer, and the reason is not a missing feature:
// "ask for this value" needs a value, and until the run is folded there are five of them. The
// card says to simplify first and names the button, rather than showing an empty chooser or no
// control at all — both of which read as the capability being absent, which is how this whole
// slice got reported.
//
// # AND WHAT RE-DERIVING WOULD COST
//
// The other honest half, and the reason this is a backend sentence rather than a line in the
// page: Simplify re-derives from the demonstration, so marks already made come off. Somebody who
// marked three values and then pressed Simplify would otherwise find out by reading the saved
// routine. They are NAMED, because "your marks" is not something anybody can check.
func supplyCaption(steps []macroir.Step, marked []string) string {
	var say []string
	if len(supplyChoices(steps)) > 0 {
		say = append(say, "Pick a typed step and name it, and Play will ask for "+
			"that value instead of typing it.")
	} else if perCharacterTyping(steps) {
		say = append(say, "This recording types one step per character, as you typed "+
			"it, and a value the routine asks for has to be one step. Press "+
			"Simplify — it folds a typed run into one step — or say below which "+
			"steps are typed text, and then you can ask for it.")
	}
	if len(marked) > 0 {
		say = append(say, "Simplify re-derives the steps from your demonstration, so "+
			"it would type "+strings.Join(marked, ", ")+" again.")
	}
	return strings.Join(say, " ")
}

// SupplyDraftText makes the text at `at` something the person supplies, and writes nothing.
//
// `at` is a position in the step list the card was drawing and `shown` is that list
// (`Recording.Basis`), which is REFUSED when it is no longer the draft’s.
//
// THAT TERM WAS NOT HERE AT FIRST, and the reasoning it replaced was wrong in a way worth keeping
// a record of: a draft is held in memory by the one window reviewing it, so there is no second
// PROCESS to race — which was read as there being no second writer at all. The window is the other
// writer. Simplify re-derives the list, going back to the original replaces it, this operation and
// the typed-text one beside it each change it, and every one of those moves or re-means the
// positions the page last drew. A press that arrived after any of them would have supplied a step
// nobody pointed at, and the result is a working routine that types a placeholder somewhere else.
// See orchestrator.Draft.Shown.
func (a *Acts) SupplyDraftText(shown string, at int, name string) string {
	dr, cp, why := a.draftToSave()
	if why != "" {
		return why
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	next, said, err := d.SuppliedInDraft(dr, shown, at, name)
	if err != nil {
		return err.Error()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	// THE DRAFT UNDER REVIEW IS SWAPPED, and the marks are remembered so the caption can
	// say what Simplify would cost. `wasDraft` is NOT touched: it is the way back to what
	// was DEMONSTRATED, and a mark is not a simplification.
	cp.draft = next
	cp.supplied = append(cp.supplied, strings.ToLower(strings.TrimSpace(name)))
	a.trial.recorded(cp.name, "Marked a value as supplied before saving: "+said,
		"", false, Pictured{})
	return ""
}
