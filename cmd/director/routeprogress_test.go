package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
)

// The panel says how much of the demonstrated route is verified.
//
// # Why a person needs this
//
// A demonstration of Home → Bluetooth → Mouse is two reusable edges, and BOTH have to be reviewed
// before the goal is reachable from where it starts. Live, the second leg was never offered and
// the episode ended without saying so, so a person was told the route was learned when one step
// of it had never been tried.
//
// "Verified 1 / 2" is the difference between that and the truth.
//
// Deleting routeProgress must fail this.
func TestThePanelSaysHowMuchOfTheRouteIsVerified(t *testing.T) {
	rt := &Runtime{}
	s := teach.Session{
		Application: "settings",
		Edges: []teach.EdgeReview{
			{Route: observe.RelationshipRef{From: "subj_home", To: "subj_bt"},
				Status: teach.EdgeVerified},
			{Route: observe.RelationshipRef{From: "subj_bt", To: "subj_mouse"},
				Status: teach.EdgeOffered},
		},
	}
	var v learnView
	rt.routeProgress(s, &v)

	if v.Verified != 1 || v.Required != 2 {
		t.Errorf("verified %d/%d, want 1/2", v.Verified, v.Required)
	}
	if v.RouteStatus != string(teach.RouteUnreviewed) {
		t.Errorf("route status %q with a leg still under review", v.RouteStatus)
	}
	if len(v.Steps) != 2 {
		t.Fatalf("%d step(s) shown for a two-leg route", len(v.Steps))
	}
	if v.Steps[0].Status != string(teach.EdgeVerified) {
		t.Errorf("step 1 shows %q", v.Steps[0].Status)
	}
	// THE WHOLE walk, not whichever leg is under review.
	if strings.Count(v.Route, "→") != 2 {
		t.Errorf("the route reads %q; a two-leg walk has three places in it", v.Route)
	}
}

// No subject id is ever put in front of a person.
//
// An id is an internal handle. Showing one is asking somebody to debug Marco rather than read it.
func TestTheRoutePanelShowsNoSubjectIds(t *testing.T) {
	rt := &Runtime{}
	s := teach.Session{
		Application: "settings",
		Edges: []teach.EdgeReview{
			{Route: observe.RelationshipRef{From: "subj_home", To: "subj_bt"},
				Status: teach.EdgeVerified},
		},
	}
	var v learnView
	rt.routeProgress(s, &v)

	shown := v.Route
	for _, st := range v.Steps {
		shown += " " + st.From + " " + st.To
	}
	if strings.Contains(shown, "subj_") {
		t.Errorf("a subject id reached the panel: %q", shown)
	}
}

// A session with no demonstrated route leaves the panel alone.
func TestNoRouteMeansNoProgressLine(t *testing.T) {
	rt := &Runtime{}
	var v learnView
	rt.routeProgress(teach.Session{Application: "settings"}, &v)
	if v.Required != 0 || len(v.Steps) != 0 || v.RouteStatus != "" {
		t.Errorf("a session with no route produced progress: %+v", v.Steps)
	}
}
