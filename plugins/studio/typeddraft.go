package main

// SAYING WHICH STEPS ARE TYPED TEXT, FROM THE CARD THAT REVIEWS THE RECORDING.
//
// # The defect this exists for
//
// A key is written down as a HOLD when another key goes down before it is released. That is
// exactly what typing rollover is, and it is not rare: measured on the owner's own recording, the
// presses of `stone` last 93–149ms and overlap the next one by 12–56ms, so the word was saved as
//
//	KeyDown s · KeyDown t · KeyUp s · KeyDown o · KeyUp t · KeyDown n · KeyUp o · Key e · KeyUp n
//
// — four held keys and one tap. Neither typing-fold pass can touch it, because both need a step
// carrying TEXT and a hold carries none. So Simplify left the word alone, and because a value the
// routine asks for has to be one step, the recording could not be turned into a routine that asks
// for the item name either.
//
// # And why the fix is a person saying so
//
// Because `s↓ t↓ s↑ t↑` and a brief two-key gameplay combination `q↓ e↓ q↑ e↑` are the SAME SHAPE.
// Not the key names — Q and E are printable. Not the durations — somebody can type slowly and
// press a game key briefly. Nothing in the events distinguishes them, so a rule that folded one
// would silently flatten the other, in a routine somebody is relying on.
//
// So nothing here reads the recording differently. `markHolds` is untouched; an ordinary save of
// that demonstration still produces the holds, and so does a rebuild under today's rules of a
// routine nobody has edited. What this does is EDIT the list, with the steps shown before and
// after, and the edit is written down beside the routine the way every other edit to a list is.
//
// The rules are the orchestrator's: [orchestrator.TypedRunsIn] proposes the runs and
// [orchestrator.Deps.TypedInDraft] applies one, both through the same validator, which is also
// what the saved routine's panel asks. This file is the window's interaction with it.

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
)

// TypedChoice is one run of steps that could be declared typed text.
//
// `From` and `To` are positions in the step list this review is showing, from zero and inclusive
// at both ends. `Label` names the run the way the list numbers it AND says what it would become,
// because that is the thing somebody is being asked to agree to — a range of positions is
// something they would have to trust.
type TypedChoice struct {
	From  int    `json:"from"`
	To    int    `json:"to"`
	Label string `json:"label"`
}

// typedChoices is every run of a draft's steps that could be declared typed text.
//
// Straight from the orchestrator, relabelled. The window does not work out which runs are
// eligible: that is the validator's reading, and a second one here would be free to offer a range
// the press then refuses.
func typedChoices(steps []macroir.Step) []TypedChoice {
	var out []TypedChoice
	for _, r := range orchestrator.TypedRunsIn(steps) {
		out = append(out, TypedChoice{From: r.From, To: r.To,
			Label: fmt.Sprintf("steps %d to %d — types %q", r.From+1, r.To+1,
				r.Text)})
	}
	return out
}

// typedCaption is what the control says about itself, before anything is pressed.
//
// THREE FACTS, AND THE SECOND ONE IS A REQUIREMENT. The ruling that allowed this operation to
// convert a long press as readily as a short one — because duration does not tell typing from a
// game, which is the whole premise — came with the condition that what the conversion COSTS is
// said in advance: the held keys inside the run stop being held, and the waits between them go.
// The waits outside the run do not, and that is worth saying too, because "the waits go" read on
// its own is a sentence about the whole routine.
//
// Said here rather than in the page for the reason every other sentence on this card is: a page
// with the words typed into it goes on promising whatever it said after the rules move.
func typedCaption(runs []TypedChoice) string {
	if len(runs) == 0 {
		return ""
	}
	return "Typing is recorded as held keys when the next key goes down before the last " +
		"one comes up, which is most typing. If a run of steps is text you typed, say " +
		"so and it becomes one step: the keys in it stop being held and the waits " +
		"between them go. Waits outside the run are left alone."
}

// TypedDraftText declares a run of the draft's steps to be typed text, and writes nothing.
//
// `shown` is the step list the card was drawing (`Recording.Basis`), and a stale one is REFUSED:
// Simplify, going back to the original and an earlier edit all move the positions, so a range of
// numbers is not consent on its own. The saved routine's panel has had that contract since step
// editing did; the draft had none until this operation needed one, and the operation beside this
// one takes it now too.
//
// The draft under review is swapped, like Simplify and the supply control beside it, and Save
// persists whichever version is on screen.
func (a *Acts) TypedDraftText(shown string, from, to int) string {
	dr, cp, why := a.draftToSave()
	if why != "" {
		return why
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	next, said, err := d.TypedInDraft(dr, shown, from, to)
	if err != nil {
		return err.Error()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	// AND THE SUPPLIED MARKS SURVIVE, because this does not re-derive anything: a
	// placeholder is refused inside the run, so a value already marked is still in the
	// list and still marked. `wasDraft` is not touched either — the way back is to what
	// was DEMONSTRATED, and this is an edit.
	cp.draft = next
	a.trial.recorded(cp.name, "Said which steps are typed text, before saving: "+said,
		"", false, Pictured{})
	return ""
}
