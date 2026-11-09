package main

// A SAVED ROUTINE, PROPOSED BEFORE THE WORDS ARE READ AGAINST THE SCREEN.
//
// The intake's Director arm is reached by words no routine answers to exactly. It used to hand
// them straight to Director's screen reading, so a misheard routine name was tried as a desktop
// task and failed, and the routine it named was looked for only afterwards, if at all. Now
// Director's resolver (RESOLVE_ROUTE, the same read `marco resolve` makes) is asked first. When it
// names a saved routine, that answer is PROPOSED on `[proposed] ` and the intake stops: a
// resolution selects, and running is still the person's decision.
//
// Read-only, and it starts nothing: a Director that is not running has no resolver to ask, and
// the request then goes on exactly as it did before. See [[ADR-322-resolve-before-reading-the-screen]].

import (
	"encoding/json"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
)

// proposeRoutine is a package variable for the same reason submitPhrase is: the intake must be
// exercisable without reaching the Director on the developer's desktop. Production never
// reassigns it.
//
// It returns the answer as one JSON line, the routine's name, and whether a routine was named.
var proposeRoutine = directorPropose

func directorPropose(phrase string) (string, string, bool) {
	c, err := directorConnect(false)
	if err != nil {
		return "", "", false
	}
	defer c.Close()
	out, err := c.ResolveRoute(service.ResolveRoutePayload{Request: phrase, Library: marcopaths.Routes(),
		// THE INTAKE CAN PERFORM A SKILL, and its proposal is reviewed on the widget's line.
		Skills: true})
	if err != nil {
		mlog.Info("intake: the resolver could not be asked", "err", err)
		return "", "", false
	}
	return proposedRoutine(out)
}

// proposedRoutine is whether an answer proposes a routine, and the line that carries it.
//
// Only a run naming a routine, against the library this process means. A clarification, a learn
// answer, a chat reply or a library mismatch proposes nothing, and the words go on to Director's
// screen reading as before.
func proposedRoutine(out service.ResolveRouteResponse) (string, string, bool) {
	if out.Mismatch != "" || out.Answer.Kind != dispatch.IntentRun || out.Answer.ID == "" {
		return "", "", false
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", "", false
	}
	mlog.Info("intake: the resolver proposed a saved play", "routine", out.Answer.ID,
		"source", out.Answer.Source)
	return string(b), out.Answer.Name, true
}

// isClarificationAnswer is the intake's own reading of whether words answer a pending question:
// the same one invoke.Decide uses.
func isClarificationAnswer(phrase string) bool {
	_, ok := intent.ParseClarification(phrase)
	return ok
}
