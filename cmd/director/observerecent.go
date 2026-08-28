package main

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// "Learn what I just did."
//
// # The sentence this makes true
//
//	marco observe
//	   ... the person uses their computer ...
//	marco learn "open mouse settings" --recent
//
// No demonstration mode, no repeat, no naming questions, no rehearsal, and no input from Marco at
// any point. The evidence already existed; what the person supplied is a NAME and a permission.
//
// # It is not a second Learn
//
// Everything after the promotion is the path a live Learn already takes. The places go into the
// place store, the transitions into the topology, the demonstrations into the candidate store,
// the controls into the Repertoire, the outcome onto a durable goal, and the play through
// `learnTail.Save`, which is `Runtime.LearnedPlay` with Save and Register — the one persistence
// path, with the one collision policy. There is no second router, no second resolver, no second
// performer, and nothing here that could emit input.
//
// # And it is not a fake demonstration either
//
// What it writes down is what was seen: `ProcedureCandidate.Verified` is false, the relationship
// carries the navigation that PRECEDED the change rather than a claim about what caused it, and
// no rehearsal is recorded because none happened. A person demonstrated this and Marco understood
// it — which is a different and weaker claim than Marco having performed it, and the store says
// which. See ADR-094.

// recentLearn is what a retrospective Learn concluded, for the surfaces that report it.
type recentLearn struct {
	// Outcome is selection's word, so a caller can tell "there was nothing" from "there were
	// two things" without parsing a sentence.
	Outcome ambient.Outcome `json:"outcome"`
	// Why is the shortfall when the evidence was insufficient.
	Why ambient.Shortfall `json:"why,omitempty"`
	// Said is what a person reads.
	Said string `json:"said"`
	// Application, Steps, Places and Targets describe what was promoted.
	Application string `json:"application,omitempty"`
	Steps       int    `json:"steps,omitempty"`
	// Established is how many screens this promotion made durable, out of Steps+1 endpoints.
	// Zero means every screen on the walk was already known, which is the ordinary case
	// after the first time.
	Established int `json:"places_established,omitempty"`
	Targets     int `json:"targets_remembered,omitempty"`
	// Route is the leg the play was written for.
	Route observe.RelationshipRef `json:"route,omitzero"`
	// Play is what was saved, when one was.
	Play *service.LearnedSaved `json:"play,omitempty"`
	// Considered is how much of the trail selection looked at, for the diagnostics.
	Considered int `json:"considered,omitempty"`
	// Rebound is the subject this name USED to mean, and ReboundSaid is that in words.
	//
	// Two fields because they answer different readers: a client that wants to show the old
	// destination needs the id, and a person needs the sentence.
	//
	// Empty in the ordinary case, which is a new name or the same name for the same outcome.
	// A name coming to mean somewhere else is rebinding — deliberate, and the right rule —
	// but it is invisible from the outside, so it travels here and gets said.
	Rebound     string `json:"rebound_from,omitempty"`
	ReboundSaid string `json:"rebound_said,omitempty"`
}

// LearnRecent promotes the demonstration somebody just gave, under the name they gave it.
//
// # The order, and why each step is where it is
//
//	select      pure, over transient evidence, writing nothing. A refusal costs nothing and
//	            leaves nothing behind, which is what makes it safe to ask on any phrase.
//	establish   the screens, first, because a transition needs both its endpoints to exist
//	            before the store will accept it.
//	relate      the transitions, because the candidate store refuses a candidate whose
//	            relationship it does not hold.
//	remember    the demonstrations, one per leg — the goal-centric decomposition, so every
//	            leg is reusable route evidence in its own right and none is welded to another.
//	name        the controls the selected walk actually used, and only those.
//	goal        the outcome, in the person's own words, bound to the destination.
//	save        the play, through the one persistence path.
//	watermark   last, so a promotion that failed part-way can be retried on the same evidence.
//
// # What happens if it fails half-way
//
// It is not one transaction and it does not claim to be. Every write below is its own atomic
// store operation and there is no rollback: a promotion that establishes two places and then
// cannot write a candidate leaves two places established. That is honest rather than tidy —
// establishing a Place asserts only that Marco can recognise a screen, which is true whatever
// happened afterwards, and the alternative would be forgetting something correct because
// something else went wrong.
//
// What it must never do is report success it did not have, and it does not: the play is the last
// write and the report carries what actually got written. The watermark moves only after the
// save, so a failed promotion can be retried on the same evidence rather than being consumed by
// the attempt.
//
// Deleting the watermark must fail TestTheSameAfternoonIsNotLearnedTwice.
func (r *Runtime) LearnRecent(q service.ObserveLearn) (recentLearn, error) {
	name := strings.TrimSpace(q.Name)
	if name == "" {
		return recentLearn{}, fmt.Errorf(
			"tell me what to call it: marco learn \"open mouse settings\" --recent")
	}
	// THE PLAY'S TWO NAMES, derived and validated FIRST — before anything is established,
	// related, remembered or saved. A phrase Marco could never write down should fail in the
	// first second rather than after a promotion that then has nothing to hand back.
	actor, verb, err := playNameFor(name, q.Actor, q.Verb)
	if err != nil {
		return recentLearn{}, err
	}
	if r.observations == nil {
		return recentLearn{}, fmt.Errorf("this Director has no observation registry")
	}
	a := r.ambient()

	// PURE, AND FIRST. Nothing durable has happened when this returns, whatever it returns.
	res := ambient.SelectDemonstration(a.buf.Look(), ambient.Request{Application: q.Target.Application})
	out := recentLearn{Outcome: res.Outcome, Why: res.Why, Considered: res.Considered,
		Application: res.Demonstration.Application}
	if res.Outcome != ambient.Selected {
		out.Said = sayRefusal(res, a.watching())
		return out, nil
	}

	d := res.Demonstration
	out.Application, out.Steps = d.Application, len(d.Steps)

	r.observations.mu.RLock()
	memory := r.observations.memory
	r.observations.mu.RUnlock()
	if memory == nil {
		return out, fmt.Errorf("this Director has no durable memory, so it cannot learn")
	}
	// THE SAME STORE, reached through the same three narrow interfaces a session reaches it
	// through. Probed rather than required, exactly as observationRegistry.start probes them,
	// so a Director wired without one degrades in the same way here as it does there.
	places, _ := memory.(observe.PlaceStore)
	candidates, _ := memory.(observe.CandidateStore)
	targets, _ := memory.(observe.TargetStore)

	// THE LICENCE, named out loud, and it is the SAME one a live Learn session declares.
	//
	// A person typed a name and asked for what they just did to be remembered. That is the
	// human semantic event the three permissions hang off — the identical event, arriving
	// through a different door — so it grants the identical set rather than a special
	// retrospective variant that would eventually drift from it.
	//
	// Replacing this with observesession.Episode{}.Licence must fail
	// TestObserveCannotMakeItsOwnEvidenceDurable.
	p := promotion{
		licence: observesession.LearnLicence(), application: d.Application,
		memory: memory, places: places, candidates: candidates, targets: targets,
	}

	steps, established, err := resolvePlaces(p, d)
	if err != nil {
		return out, err
	}
	out.Established = established
	if _, err := p.relate(steps); err != nil {
		return out, err
	}
	for _, s := range steps {
		c := candidateFor(d.Application, s, observe.MatchSame, len(d.Steps)+1)
		if err := p.remember(c); err != nil {
			return out, fmt.Errorf("keeping what you showed me: %w", err)
		}
		written, err := p.name(c)
		if err != nil {
			return out, err
		}
		out.Targets += written
	}

	// THE TERMINAL LEG is what the play is for. Goal-centric: a demonstration is evidence
	// about reaching a destination, every leg of it is durable route knowledge on its own,
	// and the outcome the person named is where they STOPPED. See ADR-056.
	terminal := steps[len(steps)-1]
	out.Route = observe.RelationshipRef{From: terminal.from, To: terminal.to}

	// THE GOAL, in the person's own words, bound to the destination subject — never to the
	// route and never to the start. Same write the live path makes, and the same refusal when
	// one name already means a different outcome.
	//
	// Deleting this must fail TestLearningRecentEvidenceRemembersTheGoal.
	if gs, ok := memory.(observe.GoalStore); ok {
		// WHAT IT USED TO MEAN, read BEFORE the write, because afterwards there is nothing
		// left to read. See reboundFrom: the store rebinds on purpose and cannot report it.
		//
		// Deleting this must fail TestTeachingANameAgainSaysWhatItUsedToMean.
		if was, changed := reboundFrom(gs, d.Application, name, terminal.to); changed {
			out.Rebound = was
			out.ReboundSaid = sayRebound(name, was, memory, d.Application)
		}
		if err := gs.RememberGoal(d.Application, observe.Goal{
			Name: name, Subject: terminal.to,
		}); err != nil {
			return out, err
		}
	}

	// AND THE PLAY, through the one persistence path — saved and registered in one act, with
	// whatever collision policy `LearnedPlay` already applies. There is no second saver here
	// and there must not be one.
	tail := &learnTail{
		rt:     r,
		app:    func() string { return d.Application },
		phrase: func() string { return name },
		live:   false,
	}
	saved, err := tail.Save(out.Route, actor, verb)
	if err != nil {
		return out, err
	}
	out.Play = &service.LearnedSaved{
		Name: saved.Name, Saved: saved.Saved, Registered: saved.Registered,
		Source: saved.Source,
	}

	// LAST. See the failure note above for why the watermark moves only once something was
	// written down, and why it moves at all.
	a.buf.Promoted(d.Through)
	out.Said = sayLearned(out, saved.Registered)
	return out, nil
}

// sayRefusal is what a person reads when there was nothing to promote.
//
// # Every one of these sends somebody somewhere different
//
// "I have nothing recent" and "I saw it and could not read what you clicked" are two completely
// different problems: the first means start watching, the second means the control has no name
// perception could admit. A single sentence for both is the shape of failure this repository has
// paid for repeatedly — a silence read as a finding.
func sayRefusal(res ambient.Result, watching bool) string {
	switch res.Outcome {
	case ambient.NotYours:
		return "the last thing that happened was something I did, not something you showed me"
	case ambient.Ambiguous:
		return "you got there two different ways just now, and I can't tell which one you " +
			"mean. Show me the one you want, or say `learn \"...\"` while you do it"
	case ambient.Insufficient:
		return sayShortfall(res.Why)
	}
	if !watching {
		return "I haven't been watching, so I don't know what you just did. " +
			"Turn it on with: marco observe"
	}
	return "I've been watching and haven't seen you go anywhere yet"
}

// sayShortfall names the one thing that was missing.
func sayShortfall(why ambient.Shortfall) string {
	switch why {
	case ambient.ShortUnknownPlace:
		return "I watched you move between screens I can't describe well enough to find " +
			"again, so I can't write down a way back to them"
	case ambient.ShortNoAction:
		return "the screen changed and I didn't see you do anything that caused it, so I " +
			"don't know how to make it happen again"
	case ambient.ShortUnnamedTarget:
		return "I saw you press something and couldn't read what it was called, so I can't " +
			"say what to press"
	}
	return "I don't have enough of what you just did to learn it"
}

// sayLearned is what a person reads when it worked.
//
// It says what was learned and does NOT offer to try it. A clean demonstration is admitted on the
// strength of what the person showed — see the Fast Learn note in learn.Coordinator.admitObserved
// — and the permission to act is asked later, of whoever invokes the play, by the ordinary
// authority door.
func sayLearned(out recentLearn, registered bool) string {
	var b strings.Builder
	b.WriteString("I saw what you did. ")
	if out.Play != nil && out.Play.Name != "" {
		b.WriteString("Learned it as " + out.Play.Name)
	} else {
		b.WriteString("Learned it")
	}
	if out.Steps > 1 {
		fmt.Fprintf(&b, ", from %d steps", out.Steps)
	}
	b.WriteString(".")
	if out.Established > 0 {
		fmt.Fprintf(&b, " %d screen(s) I hadn't seen before are now ones I know.",
			out.Established)
	}
	// AND WHAT THE NAME USED TO MEAN, when it used to mean something else.
	//
	// Before "you can ask me to do it", because the order is the point: somebody about to
	// use a command needs to know it stopped being the old one first.
	if out.ReboundSaid != "" {
		b.WriteString(" " + out.ReboundSaid)
	}
	if registered {
		b.WriteString(" You can ask me to do it.")
	}
	// WATCHING CONTINUES, and saying so matters: a person who has just been told something
	// was learned has no way to know whether the mode they turned on is still on.
	//
	// Deleting the continuation must fail TestWatchingSurvivesBeingLearnedFrom.
	b.WriteString(" Still watching.")
	return b.String()
}

// watching reports whether ambient observation is currently on, for the sentences that depend on
// telling "nothing happened" from "nothing was looking".
func (a *ambientObserver) watching() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.on
}
