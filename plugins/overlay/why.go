package main

// "WHY DIDN'T THAT WORK?" — answered from what the intake wrote down.
//
// The engine keeps one record of the latest attempt (internal/lastattempt, written by cmd/marco's
// runInvocation): what it became, how it ended, the stage it ended at, why, and what to do next.
// `marco last --json` reads it back with the sentences composed, and this shows them. Nothing here
// interprets anything: the widget is the only place a person asks, and the engine is the only
// place the answer is known. No model is asked; the phrasings are a fixed list
// (lastattempt.Asks). See [[ADR-320-why-didnt-that-work]].

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// lastWithin bounds one reading: one `marco` start and one small file.
var lastWithin = 3 * time.Second

// askLast is how the widget reads the record. A seam: a test answers for the engine.
var askLast = readLastChild

// lastAnswer is what `marco last --json` prints.
type lastAnswer struct {
	Known bool     `json:"known"`
	Says  []string `json:"says"`
}

func readLastChild() (lastAnswer, string) {
	ctx, cancel := context.WithTimeout(context.Background(), lastWithin)
	defer cancel()
	out, err := exec.CommandContext(ctx, marcoBin(), "last", "--json").Output()
	if ctx.Err() != nil {
		return lastAnswer{}, "Marco did not say what happened within " + lastWithin.String()
	}
	var a lastAnswer
	if jerr := json.Unmarshal(out, &a); jerr != nil || len(a.Says) == 0 {
		why := errText(err)
		if why == "" && jerr != nil {
			why = jerr.Error()
		}
		return lastAnswer{}, "Marco could not say what happened last (" + why + ")"
	}
	return a, ""
}

// explainLast shows the answer: the first sentence on the status line, all of them in the log.
func explainLast(h *model) {
	go func() {
		a, failed := askLast()
		if failed != "" {
			h.setStatus(failed)
			h.log(failed)
			return
		}
		// THE LINES FIRST, THE STATUS LAST: the status is what says the answer has arrived, so
		// anything that sees it sees the whole answer. The other order was a race, caught by
		// TestWhyDidntThatWorkIsAnsweredFromTheRecord reading the log between the two.
		for _, s := range a.Says {
			if s = strings.TrimSpace(s); s != "" {
				h.log(s)
			}
		}
		h.setStatus(a.Says[0])
	}()
}
