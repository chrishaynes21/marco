package main

import (
	"context"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"os/exec"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
	"github.com/chaynes-simpleclouds/marco/pkg/referent"
)

// Grounding is ephemeral presentation of one decision. These tests hold its lifecycle: a
// presentation belongs to the claim that raised it, and when the claim ends — the phase moves
// on, the learn fails, the session settles — the presentation ends with it. The surface's own
// hold timer is a backstop, never the lifecycle.

func groundedFixture(label string, current bool) groundedView {
	return groundedView{
		Label: label, Role: string(observe.ReferentTeachStart),
		Say:     label + " — 4 controls I recognise this screen by. This is what I mean.",
		Current: current,
		Boxes:   []referent.Box{{X: 10, Y: 10, Width: 100, Height: 40}},
	}
}

// A settled session owns no presentation, whatever its views still describe.
func TestFailedLearnDismissesOwnedPresentation(t *testing.T) {
	launched := 0
	g := &groundingShown{
		launch: func(groundedView, time.Duration) (*exec.Cmd, error) {
			launched++
			return &exec.Cmd{}, nil
		},
	}
	g.print(teachView{Phase: teach.ReadyForDemo,
		Grounding: []groundedView{groundedFixture("START", true)}})
	if launched != 1 || len(g.live) != 1 {
		t.Fatalf("launched %d, live %d — the current claim was not presented",
			launched, len(g.live))
	}
	// The learn fails. The session is settled, and the presentation it owned ends NOW —
	// not when a process timer happens to run out.
	g.print(teachView{Phase: teach.Refused, Settled: true,
		Grounding: []groundedView{groundedFixture("START", false)}})
	if len(g.live) != 0 {
		t.Fatalf("%d highlight(s) survive a failed learn", len(g.live))
	}
	if launched != 1 {
		t.Fatalf("a settled session launched %d more highlight(s)", launched-1)
	}
}

// A claim the phase has moved past is dismissed and never redrawn.
func TestAStalePresentationIsDismissedWhenItsClaimEnds(t *testing.T) {
	launched := 0
	g := &groundingShown{
		launch: func(groundedView, time.Duration) (*exec.Cmd, error) {
			launched++
			return &exec.Cmd{}, nil
		},
	}
	g.print(teachView{Phase: teach.ReadyForDemo,
		Grounding: []groundedView{groundedFixture("START", true)}})
	// The phase moves on; the START claim is history now.
	g.print(teachView{Phase: teach.Evaluating,
		Grounding: []groundedView{groundedFixture("START", false)}})
	if len(g.live) != 0 {
		t.Fatalf("%d highlight(s) outlived the phase that raised them", len(g.live))
	}
	if launched != 1 {
		t.Fatalf("%d launches; a stale claim was redrawn", launched)
	}
}

// A reader arriving after the moment gets the sentence and no box — the exact live failure:
// every bare `director teach` status read relaunched highlights from a phase long past.
func TestAStatusReadAfterTheMomentDrawsNothing(t *testing.T) {
	launched := 0
	g := &groundingShown{
		launch: func(groundedView, time.Duration) (*exec.Cmd, error) {
			launched++
			return &exec.Cmd{}, nil
		},
	}
	g.print(teachView{Phase: teach.Evaluating,
		Grounding: []groundedView{groundedFixture("START", false)}})
	if launched != 0 {
		t.Fatalf("a non-current claim was drawn %d time(s)", launched)
	}
	if len(g.live) != 0 {
		t.Fatalf("%d live highlight(s) for a claim that was never current", len(g.live))
	}
}

// The coordinator's own account: a settled session marks nothing current, so no surface —
// this follower or any other — has anything it may draw.
func TestEndedGroundingCannotLeaveHighlights(t *testing.T) {
	base := teach.Session{
		Start: "subj_a",
		Route: observe.RelationshipRef{From: "subj_a", To: "subj_b"},
	}
	for _, phase := range []teach.Phase{teach.Complete, teach.Refused, teach.Cancelled} {
		s := base
		s.Phase = phase
		for _, e := range s.Grounded() {
			if e.Current {
				t.Errorf("%s is still current in the settled phase %q — a surface reading "+
					"this session may draw a highlight nothing will ever dismiss",
					e.Label, phase)
			}
		}
	}
}

// And the positive half: each endpoint is current exactly while its claim is the session's.
func TestAGroundedEndpointIsCurrentOnlyDuringItsClaim(t *testing.T) {
	s := teach.Session{
		Start: "subj_a",
		Route: observe.RelationshipRef{From: "subj_a", To: "subj_b"},
	}
	currentOf := func(p teach.Phase, label string) bool {
		s.Phase = p
		for _, e := range s.Grounded() {
			if e.Label == label {
				return e.Current
			}
		}
		t.Fatalf("no %s endpoint in phase %q", label, p)
		return false
	}
	if !currentOf(teach.ReadyForDemo, "START") {
		t.Error("START is not current at the moment it was just decided")
	}
	if currentOf(teach.ReadyToRehearse, "START") {
		t.Error("START is still current long after the session's attention moved on")
	}
	if !currentOf(teach.NeedsAnotherExample, "DESTINATION") {
		t.Error("DESTINATION is not current at the moment it was just discovered")
	}
	if currentOf(teach.Rehearsing, "DESTINATION") {
		t.Error("DESTINATION is still current during the rehearsal")
	}
}

// The one line that carries Current from the coordinator to a surface is load-bearing:
// without it every reader sees zero values, no highlight is ever drawn, and the lifecycle
// above silently proves nothing.
func TestTheViewCarriesWhetherAnEndpointIsCurrent(t *testing.T) {
	s := teach.Session{
		Start: "subj_a", Phase: teach.ReadyForDemo,
		Route: observe.RelationshipRef{From: "subj_a", To: "subj_b"},
	}
	rt := &Runtime{}
	views := rt.groundingViews(context.Background(), s, windowref.Selector{})
	found := false
	for _, v := range views {
		if v.Label == "START" {
			found = v.Current
		}
	}
	if !found {
		t.Fatal("a current endpoint reached the view as history; the Current flag is not " +
			"being carried across")
	}
}
