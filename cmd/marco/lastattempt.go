package main

// WHAT BECAME OF THE LATEST COMMAND, so "why didn't that work?" has an answer.
//
// The intake (runInvocation) is the one place every entrance reaches, so it is the one place the
// record is written: once per attempt, replacing the last, at the end — whichever arm took the
// request and however it ended. The arms only NOTE what they know as they go (which routine it
// became, where it was refused and why, the run it claimed); nothing below the intake writes the
// file, so there is one writer and one moment. See internal/lastattempt and
// [[ADR-320-why-didnt-that-work]].
//
// # What is not in it
//
// The words. The record has how many there were and from which entrance, and every sentence in it
// is scrubbed of the request, of each named and positional value it carried, and of each value a
// routine step was given — so a refusal that quoted what somebody supplied does not reach the disk.
// A control phrase ("stop") is not an attempt worth explaining and does not replace the record: the
// command somebody stopped is the one they will ask about.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/lastattempt"
	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// attemptNote is what the arms of one invocation noted about it.
type attemptNote struct {
	routine       *routes.Route
	stage         string
	because, next string
	run           string
	said          []string
}

var (
	attemptMu sync.Mutex
	attempt   attemptNote
)

// beginAttempt starts a fresh note. One invocation per process in production; a test runs many.
func beginAttempt() {
	attemptMu.Lock()
	attempt = attemptNote{}
	attemptMu.Unlock()
}

// noteStep is a routine the request became, and the values it is about to be given.
func noteStep(p playInvocation) {
	attemptMu.Lock()
	defer attemptMu.Unlock()
	rt := p.rt
	attempt.routine = &rt
	for _, v := range p.named {
		attempt.said = append(attempt.said, v)
	}
	attempt.said = append(attempt.said, p.positional...)
}

// noteEnd is where the attempt ended and why, in Marco's words, with the next step when one is
// known. The deepest arm to say so wins: it is the one that knows.
func noteEnd(stage, because, next string) {
	attemptMu.Lock()
	attempt.stage, attempt.because, attempt.next = stage, because, next
	attemptMu.Unlock()
}

// noteRun is the run record this attempt claimed.
func noteRun(id string) {
	attemptMu.Lock()
	attempt.run = id
	attemptMu.Unlock()
}

// attemptAt is where the record is kept; a variable so a test can keep it elsewhere.
var attemptAt = marcopaths.Home

// keepLastAttempt writes what became of this invocation. A failure to write is a line in the log
// and nothing else: the person asked for their routine, not for a file.
func keepLastAttempt(dec invoke.Decision, req invoke.Request, out Outcome, err error) {
	if dec.Kind == invoke.KindControl {
		return
	}
	attemptMu.Lock()
	n := attempt
	attemptMu.Unlock()

	a := lastattempt.Attempt{
		At: time.Now().Format(time.RFC3339), Source: string(req.Source),
		Words: len(strings.Fields(req.Text)), Outcome: string(out),
		Stage: n.stage, Because: n.because, Next: n.next, Run: n.run,
	}
	switch {
	case n.routine != nil:
		a.Routine, a.Application = plays.Pretty(n.routine.Slug), n.routine.App
	case dec.Kind == invoke.KindPlay:
		a.Routine, a.Application = plays.Pretty(dec.Play.Slug), dec.Play.App
	case dec.Kind == invoke.KindDemonstration:
		a.Routine = dec.Demonstration.Name
	}
	if a.Stage == "" {
		a.Stage, a.Because, a.Next = unnoted(dec, out, err, a.Routine != "")
	} else if a.Because == "" && err != nil {
		a.Because = err.Error()
	}

	said := append([]string{req.Text, dec.Phrase}, n.said...)
	said = append(said, req.Positional...)
	for _, v := range req.Named {
		said = append(said, v)
	}
	// BUT NOT THE ROUTINE'S OWN NAME: a request that IS a routine's name supplied nothing, and the
	// explanation has to be able to say which routine it is about.
	said = notTheName(said, a.Routine, n.routine)
	a.Because = lastattempt.Scrub(a.Because, said)
	a.Next = lastattempt.Scrub(a.Next, said)
	if werr := lastattempt.Write(attemptAt(), a); werr != nil {
		mlog.Info("intake: the latest attempt could not be written down", "err", werr)
	}
}

// unnoted is the stage, reason and next step of an attempt no arm said anything about: nothing
// answered, Director took it, or a run ended with an error and no note.
func unnoted(dec invoke.Decision, out Outcome, err error, routine bool) (stage, because, next string) {
	switch {
	case out == OutcomeUnavailable && !routine:
		return lastattempt.StageMatching,
			"no saved play answers to those words, and Director could not take them",
			"say a play's name as it is saved (plays lists what you can ask for here), " +
				"or learn it: learn <name>"
	case !routine:
		because = fmt.Sprintf("Director took the request, and it ended %s", out)
		if err != nil {
			because += ": " + err.Error()
		}
		next = "Studio's History has Director's own account of it"
		if out == OutcomeClarify {
			next = "answer Director's question, or ask again in other words"
		}
		return lastattempt.StageDirector, because, next
	case err != nil:
		return lastattempt.StageRunning, err.Error(), ""
	}
	return lastattempt.StageRunning, "", ""
}

// valuesNext is what to do about a routine that was refused for its values.
func valuesNext(rt routes.Route, why error) string {
	var needed *routes.ValuesNeeded
	if errors.As(why, &needed) && len(needed.Names) > 0 {
		return fmt.Sprintf("say it again with the value — for example “%s %s:…” — "+
			"or open it in Studio and run it from there", plays.Pretty(rt.Slug), needed.Names[0])
	}
	return "say it again with the values it takes, or open it in Studio"
}

// runLast is `marco last`: the latest attempt in a person's words, or `--json` the record and the
// same sentences beside it.
func runLast(args []string) {
	asJSON := false
	for _, a := range args {
		if a == "--json" {
			asJSON = true
		}
	}
	a, ok, err := lastattempt.Read(attemptAt())
	says := []string{"Nothing has been asked for yet, so there is nothing to explain."}
	if err != nil {
		says = []string{"What happened last cannot be read: " + err.Error()}
	} else if ok {
		says = a.Says(time.Now())
	}
	if asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(struct {
			Known   bool                 `json:"known"`
			Attempt *lastattempt.Attempt `json:"attempt,omitempty"`
			Says    []string             `json:"says"`
		}{ok, map[bool]*lastattempt.Attempt{true: &a}[ok], says})
		return
	}
	for _, s := range says {
		fmt.Println(s)
	}
}

// firstOf is the first non-empty sentence.
func firstOf(s ...string) string {
	for _, v := range s {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// notTheName is what was supplied, without anything that is only the routine's own name.
func notTheName(said []string, pretty string, rt *routes.Route) []string {
	out := said[:0:0]
	for _, s := range said {
		t := strings.ToLower(strings.TrimSpace(s))
		if t == strings.ToLower(pretty) || (rt != nil && (t == strings.ToLower(rt.Slug) ||
			t == strings.ToLower(plays.Pretty(rt.Slug)))) {
			continue
		}
		out = append(out, s)
	}
	return out
}
