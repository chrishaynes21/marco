package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// What a person's words turn out to mean.
//
// # The layer this is, and the three above and below it
//
//	LANGUAGE    the words somebody said            <- this file turns these
//	GOAL        the outcome they name              <- into this
//	DESTINATION a subject in the semantic graph
//	GRAPH       the transitions Marco knows
//	PLAN        the route chosen now, from here
//
// Resolution is where a phrase becomes a destination, and it is the ONLY thing this file does. It
// consults no topology, chooses no route, ranks no edge and reads no screen — a goal layer that
// knew about routes would be a second planner, and a planner that knew about words would be a
// second language.
//
// # It is pure
//
// A function of the goals a store holds, the applications to search and what was asked. No clock,
// no desktop, no I/O, nothing reachable from here that could move anything. That is what lets the
// diagnostic and the performer share it: `director reach` can say what a phrase would mean without
// the phrase doing anything, and the answer it gives is the answer `perform` would act on rather
// than a second opinion about it.
//
// See [[ADR-097-language-names-the-outcome-the-graph-decides-the-way]].

// resolution is what one phrase turned out to mean.
type resolution struct {
	// Goal is the outcome, when exactly one was found.
	Goal *observe.Goal
	// Application is where that outcome lives.
	Application string
	// Ambiguous is every goal the words could equally have meant, when more than one
	// application answered to them. Populated only when Goal is nil.
	Ambiguous []observe.Goal
}

// Found says the words resolved to exactly one outcome.
func (r resolution) Found() bool { return r.Goal != nil }

// resolveGoal turns what somebody asked for into the outcome they meant.
//
// # Identity first, then words
//
// A caller that supplies a subject id is naming an identity, and identities are matched exactly
// and case-sensitively: that is a registered play's own provenance speaking, and it can never be
// ambiguous. Only when there is no identity do the words decide.
//
// # And the words may not decide, which is a real answer
//
// The same phrase can name outcomes in two different applications. Somebody who taught "open
// settings" in Windows Settings and "open settings" in their mail client has two meanings for one
// sentence, and Marco has no way to know which afternoon they are having.
//
// It used to take the first, in whatever order the applications happened to sort. That is
// deterministic and it is not an answer: measured, two goals named "open settings" in `mail` and
// in `settings` resolved to `mail` because `m` sorts before `s` — so the person who taught it in
// Settings would have had their mail client brought forward and walked. A wrong answer nobody was
// told about is worse than a question.
//
// So more than one application answering is `Ambiguous`, and the caller says which ones. Narrowing
// with an application is the way through, and it is the caller's to offer.
//
// Deleting the ambiguity arm must fail TestOnePhraseInTwoApplicationsIsAQuestionNotAGuess.
func resolveGoal(goals observe.GoalStore, applications []string,
	q service.PerformQuery) resolution {

	if goals == nil {
		return resolution{}
	}
	subject := strings.TrimSpace(q.Subject)
	name := strings.TrimSpace(q.Name)
	if subject == "" && name == "" {
		return resolution{}
	}

	var found []observe.Goal
	var where []string
	for _, app := range applications {
		for _, have := range goals.Goals(app) {
			if !namesOutcome(have, q) {
				continue
			}
			// Goal.Application is the store's own — RememberGoal writes it and
			// Goals() hands it back — so there is nothing to copy in here. Taking it
			// from the loop variable instead would be a second answer to a question
			// the record already answers.
			found = append(found, have)
			where = append(where, app)
			// One name means one outcome WITHIN an application — the invariant
			// RememberGoal keeps by rebinding — so the first match in an application
			// is the only match in it.
			break
		}
	}
	switch len(found) {
	case 0:
		return resolution{}
	case 1:
		return resolution{Goal: &found[0], Application: where[0]}
	}
	// A SUBJECT ID THAT ANSWERED TWICE is a store that has lost its namespacing, not a
	// question for a person: subject ids are content-derived per application and two of them
	// matching across applications means something upstream is wrong. Reported as ambiguity
	// rather than picked, for the same reason as the words.
	sort.Slice(found, func(i, j int) bool {
		if found[i].Application != found[j].Application {
			return found[i].Application < found[j].Application
		}
		return found[i].Name < found[j].Name
	})
	return resolution{Ambiguous: found}
}

// sayAmbiguous is the question a person reads when their words meant two things.
//
// It NAMES them, because "that's ambiguous" is not something anybody can act on and "did you mean
// the one in Settings or the one in Mail" is. The way through is on the same line.
func sayAmbiguous(asked string, found []observe.Goal) string {
	apps := make([]string, 0, len(found))
	for _, g := range found {
		apps = append(apps, g.Application)
	}
	return fmt.Sprintf("%q means something in %s. Say which: --application %s",
		asked, joinWords(apps), apps[0])
}

// joinWords lists applications the way somebody would read them out.
func joinWords(in []string) string {
	switch len(in) {
	case 0:
		return ""
	case 1:
		return in[0]
	case 2:
		return in[0] + " and " + in[1]
	}
	return strings.Join(in[:len(in)-1], ", ") + " and " + in[len(in)-1]
}

// ambiguousWord is how a phrase that meant two things is named, everywhere.
//
// A refusal word in the same closed vocabulary the walker and the planner use, so a client can
// tell "I don't know that" from "I know two of those" without reading a sentence.
const ambiguousWord = "ambiguous_outcome"

// reboundFrom is the store-side reading of observe.ReboundFrom.
//
// The RULE lives in `observe`, beside the Goal it is about, so the live Learn path and this one
// cannot come to different answers about the same reuse. This is the two lines that fetch the
// list — a store read, which `observe` may not do.
func reboundFrom(goals observe.GoalStore, application, name, subject string) (string, bool) {
	if goals == nil {
		return "", false
	}
	return observe.ReboundFrom(goals.Goals(application), name, subject)
}

// sayRebound is what a person reads when a name they reused now means somewhere else.
//
// It says the old meaning is GONE, because that is the part they cannot see. A sentence that only
// announced the new binding would be true and would leave them believing they had two commands.
func sayRebound(name, was string, memory observe.Memory, application string) string {
	return fmt.Sprintf("%q used to mean %s and now means this instead.",
		name, describeSubject(memory, application, was))
}

// describeSubject names a screen the way a person would recognise it, falling back to its id.
func describeSubject(memory observe.Memory, application, subject string) string {
	type namer interface {
		Subject(id string) (observe.RememberedSubject, bool)
	}
	if s, ok := memory.(namer); ok {
		if found, held := s.Subject(subject); held {
			if n := strings.TrimSpace(found.Name()); n != "" {
				return n
			}
		}
	}
	// No name is an honest answer and the id is still an identity somebody can quote back.
	return "another screen (" + subject + ")"
}
