package main

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/outcome"
)

// THE INTAKE PROPOSED A SAVED ROUTINE, AND THE WIDGET OPENS IT FOR REVIEW (ADR-322). The engine
// asked Director's resolver before reading "cloud paul stone" against the screen, printed what it
// named on `[proposed] `, and ended with a question. The widget reads that line off a real child —
// the same reader every run goes through — opens the review on Cloud pull · resource: stone, does
// not ask Director a second time, and keeps the proposal's value out of the console log.
//
// Dropping the proposal read in streamChild, or ignoring childRun.proposed in dispatchIntake,
// must fail this.
func TestAProposedRoutineIsReviewedAndItsValueIsNotLogged(t *testing.T) {
	const heard = "cloud paul zq7stone9f3k"
	proposed := aCloudPull("cloud pull resource: zq7stone9f3k", "zq7stone9f3k")
	engineReads(t, map[string]lineReading{proposed.Line: proposed})
	was := askWhichRoutine
	askWhichRoutine = func(string) (resolved, string) {
		t.Error("Director was asked again after the intake had already proposed a play")
		return resolved{}, "asked twice"
	}
	t.Cleanup(func() { askWhichRoutine = was; promptAsk.Store(false); takePendingRoutine(); takePendingLearn() })

	var logged strings.Builder
	var mu sync.Mutex
	prevW, prevLvl := mlogW, mlogLvl
	mlogInit()
	mlogW, mlogLvl = lockedWriter{&mu, &logged}, mlogDebug
	t.Cleanup(func() { mlogW, mlogLvl = prevW, prevLvl })

	t.Setenv("MARCO_BIN", os.Args[0])
	t.Setenv("MARCO_OVERLAY_ECHO", outcome.ProposedPrefix+
		`{"answer":{"kind":"run","id":"game-b/context/cloud-pull","app":"game-b","slug":"cloud-pull",`+
		`"name":"cloud pull","source":"advisor","values":[{"name":"resource","value":"zq7stone9f3k"}],`+
		`"sourceAt":"v1"}}`+"\n"+
		"Marco has a saved play that may be what you meant: cloud pull. Nothing has run.\n"+
		outcome.ResultPrefix+string(outcome.Clarify)+"\n")
	restore := stubIntakeChild(t, func(h *model, name string, track bool, args ...string) childRun {
		return runRecordTracked(h, name, track, append([]string{"-test.run=TestOverlayEchoHelper"}, args...)...)
	})
	defer restore()
	voiceEnabled.Store(true)

	h := newModel()
	dispatch(h, request{Action: "RunVoice", Input: heard})
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && !h.snapshot().editing {
		time.Sleep(5 * time.Millisecond)
	}
	s := h.snapshot()
	// With the space a person types after the leader, so it reads "`m cloud pull …".
	if !s.editing || s.input != " "+proposed.Line || !s.hasReading {
		t.Fatalf("the proposal was not opened for review: editing %v, input %q, reading %v, status %q",
			s.editing, s.input, s.hasReading, s.status)
	}
	if takePendingLearn() != "" {
		t.Error("a proposal became an offer to record the phrase")
	}
	// WHAT IT BECAME, NOT WHAT WAS HEARD: the history row and the HUD's own lines name the routine.
	if n := len(s.history); n == 0 || s.history[n-1].cmd != "cloud pull" {
		t.Errorf("the history row does not name the proposed play: %+v", s.history)
	}
	for _, l := range s.logs {
		if strings.Contains(l, "cloud paul") {
			t.Errorf("the transcript is in the HUD's lines: %q", l)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Contains(logged.String(), "zq7stone9f3k") {
		t.Errorf("the proposed value reached the console log:\n%s", logged.String())
	}
}

// WHAT RAN IS WHAT THE HISTORY SAYS, values and all: the engine names the routine on `[route] `
// and what it was run with on `[ran] `, and the history row is the second — "cloud pull resource:
// stone", not "cloud pull" — while the console log names the routine and never the value.
//
// Ignoring childRun.ran in runRecordTracked, or echoing the line into the console, must fail this.
func TestTheHistoryShowsWhatRanWithItsValues(t *testing.T) {
	const value = "zq7stone9f3k"
	var logged strings.Builder
	var mu sync.Mutex
	prevW, prevLvl := mlogW, mlogLvl
	mlogInit()
	mlogW, mlogLvl = lockedWriter{&mu, &logged}, mlogDebug
	t.Cleanup(func() { mlogW, mlogLvl = prevW, prevLvl })

	t.Setenv("MARCO_BIN", os.Args[0])
	t.Setenv("MARCO_OVERLAY_ECHO", outcome.RoutePrefix+"cloud pull\n"+
		outcome.RanPrefix+"cloud pull resource: "+value+"\n"+
		outcome.ResultPrefix+string(outcome.Performed)+"\n")
	h := newModel()
	runRecordTracked(h, "cloud pull "+value, false, "-test.run=TestOverlayEchoHelper", "do", "cloud pull "+value)

	s := h.snapshot()
	if n := len(s.history); n == 0 || s.history[n-1].cmd != "cloud pull resource: "+value {
		t.Errorf("the history row is not what ran: %+v", s.history)
	}
	mu.Lock()
	defer mu.Unlock()
	if strings.Contains(logged.String(), value) {
		t.Errorf("the value reached the console log:\n%s", logged.String())
	}
	if !strings.Contains(logged.String(), "cloud pull") {
		t.Errorf("the console log does not name the play:\n%s", logged.String())
	}
}
