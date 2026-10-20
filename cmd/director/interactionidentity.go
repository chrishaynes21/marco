package main

// WHICH INTERACTION A SNAPSHOT IS ABOUT, SAID BY THE THING THAT SELECTED IT.
//
// # The defect this closes
//
// The first structured events built an identity out of the only number at hand: the step of the
// underlying DEMONSTRATION. An Act is composed by PICKING, though — a subset of a routine's
// interactions, in an order of its own — so an Act naming interaction 2 of a three-step routine
// published `index 2` about its FIRST interaction, said nothing about how many it has, and put
// the control identifier (or, between looks, the word of a verdict) where the name the person
// gave it belongs.
//
// None of that is a rounding error in a report. A window showing "2" about the first of one, over
// the machine's word for a thing somebody had given a word of their own, is telling a person
// something false about a press they are about to authorise with a key.
//
// # Why it has to be aimed rather than computed
//
// Because the two parties cannot both work it out. The `Demonstrated` host resolved the name out
// of the play's own sentence, so it is the only thing that knows WHICH Scene is in hand; the
// consent desk and the between-looks report are handed a step by the coordinator, and a step does
// not identify a Scene — an Act may name one position twice under two names.
//
// So the selector aims this at the interaction it is about to perform, and the emitters read it.
// What they read back is a copy: an event is a snapshot, and two snapshots sharing one struct
// would be a window watching a field change under it.
//
// A walk of an artifact aims nothing, because there is nothing to aim: its steps ARE its
// interactions, in order, and `stepIsPosition` says so.

import (
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// performedInteractions is the identity of whatever interaction is in hand.
type performedInteractions struct {
	mu sync.Mutex
	// of is how many interactions the performance has, and now the one in hand — nil
	// before the first and between each pair.
	of  int
	now *runstate.Interaction
	// stepIsPosition says a step number IS a position in this performance, which is true
	// of an artifact walk and false of an Act. False is why an unaimed Act interaction
	// reports index 0 rather than a number: "nobody could say" is a true answer and
	// "interaction 3" would not be.
	stepIsPosition bool
	// controls is step to the control the demonstration recorded, so a step is named with
	// its own evidence even when nothing aimed it.
	controls map[int]string
}

// workflowInteractions numbers an artifact walk: its steps are its interactions, in order.
func workflowInteractions(art workflowstore.Artifact) *performedInteractions {
	return &performedInteractions{of: len(art.Steps), stepIsPosition: true,
		controls: controlsOf(art)}
}

// actInteractions numbers an Act: `of` sentences, each aimed as it is performed.
func actInteractions(art workflowstore.Artifact, of int) *performedInteractions {
	return &performedInteractions{of: of, controls: controlsOf(art)}
}

// controlsOf is each step's recorded control, by step.
func controlsOf(art workflowstore.Artifact) map[int]string {
	out := make(map[int]string, len(art.Steps))
	for i, s := range art.Steps {
		out[i] = s.Control
	}
	return out
}

// aim is the selector saying which interaction it is about to perform.
//
// `index` counts from 1 and is the position in the ACT, which only the selector knows.
func (p *performedInteractions) aim(index int, name string, step int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.now = &runstate.Interaction{Index: index, Of: p.of, Name: name, Step: step,
		Control: p.controls[step]}
}

// over is the selector saying it has finished with it, so no later snapshot claims one is in
// hand. A question asked between two interactions is about neither.
func (p *performedInteractions) over() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.now = nil
}

// at is the identity of the interaction at `step`, with a verdict when one has been reached.
//
// The aimed one when it is the same step — and the step is checked rather than assumed, because
// an aim that has outlived its interaction would otherwise name the wrong Scene. Otherwise what
// can be said about the step alone, which for an artifact walk is everything.
func (p *performedInteractions) at(step int, verdict string) *runstate.Interaction {
	if p == nil {
		return &runstate.Interaction{Step: step, Verdict: verdict}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.now != nil && p.now.Step == step {
		in := *p.now
		in.Verdict = verdict
		return &in
	}
	in := &runstate.Interaction{Of: p.of, Step: step, Control: p.controls[step],
		Verdict: verdict}
	if p.stepIsPosition {
		in.Index = step + 1
	}
	return in
}

// sayWhatTheCoordinatorDecided is one attempt, in the run's structured account of itself.
//
// # Three things to say, and the first draft said one
//
// It emitted on the press and on the verdict, and nothing in between — so the whole interval
// between them went unmentioned, and a front end rendering the latest snapshot showed `acting`
// for all of it. That interval is not a gap: it is the longest part of an interaction and the
// part a person is actually waiting through, and "Marco is pressing" is the wrong thing to say
// about several seconds of Marco watching.
//
// `Awaiting` is what made the third case possible. It is the coordinator's own answer to whether
// this step's press has gone out, and no reader of an attempt can derive it: an attempt waiting
// for evidence and an attempt waiting for a screen to be established are both `waiting` with no
// result and no verdict.
//
// A function rather than four lines inside the loop's closure so it can be driven with attempts
// that are hard to produce on demand — a run that reaches a verdict needs a window, a control and
// a press — and because the identity of the interaction is the thing being asserted. The loop
// calls this and does nothing else with the attempt; deleting the call must fail
// TestTheLoopSaysWhatTheCoordinatorDecided.
func sayWhatTheCoordinatorDecided(say *runstate.Writer, scenes *performedInteractions,
	a workflowrun.Attempt) {

	switch {
	case a.Verified != nil:
		// THE VERDICT, which is what gates the next action: the second interaction
		// is not asked about until the first one is verified (ADR-242), and a front
		// end reading the stream can see that ordering rather than take it on faith.
		// Beside the interaction's name and not INSTEAD of it, which is where it was.
		say.Say(runstate.Event{Phase: runstate.Verifying,
			Interaction: scenes.at(a.Step, verdictWord(a))})
	case a.Result != nil:
		// THE PRESS ITSELF, in the look it went out in.
		say.Say(runstate.Event{Phase: runstate.Acting,
			Interaction: scenes.at(a.Step, "")})
	case a.Awaiting:
		// AND THE WAIT AFTER IT: evidence is being collected, nothing is being
		// pressed and nothing is being asked.
		say.Say(runstate.Event{Phase: runstate.Verifying,
			Interaction: scenes.at(a.Step, "")})
	}
}
