package main

// WHICH APPLICATIONS A REQUEST CAN BE AIMED AT.
//
// What is left of this file after the resolution moved. It used to hold scope eligibility,
// candidate labelling and the resolver conversation; all of that is Director's now
// (`internal/resolveroute`, reached through `RESOLVE_ROUTE` — see resolve.go), because the widget
// could not reach a policy living in this module and two surfaces resolving separately would be
// two answers to one question.
//
// # Why the dropdown stayed here
//
// Because it is LIBRARY BROWSING and not resolution policy. It answers "which applications have
// something saved for them", which is the same listing the cards on this page come from, and
// offering it is an affordance rather than a decision: nothing is eligible or ineligible because
// of what this list says.
//
// What keeps that safe is that the resolution validates what is submitted. Director is told which
// library this window believes it is talking about and which application the person chose, and it
// names a mismatch and selects NOTHING rather than serving a routine of that name out of a
// different library. So a stale or wrong list costs somebody a sentence, never a selection.
//
// See [[ADR-267-director-owns-route-resolution]].

import (
	"sort"
	"strings"
)

// Target is one application a request may be aimed at, as the page offers it.
type Target struct {
	// App is the application as the library names it, and is the term the page hands
	// back — it reaches Director as the stated override. See resolveroute.Basis.
	App string `json:"app"`
	// Open is whether it has a window right now. A HINT AND NOT A FILTER: it is shown so
	// somebody can see that the application their in-place command needs is not running,
	// and it decides nothing, because the choice is theirs.
	Open bool `json:"open"`
	// Context is how many in-place routines it has, which is what choosing it unlocks.
	Context int `json:"context"`
}

// SuggestTargets is the applications a request can be aimed at.
//
// Every application in the library with at least one in-place routine, because those are the ones
// a target CHANGES anything for: a global routine and an anywhere-in-application one are eligible
// whatever is chosen. Sorted, so the page is stable between polls.
func (a *Acts) SuggestTargets() []Target {
	open := map[string]bool{}
	for _, c := range a.windowsNow() {
		if c.Application != "" {
			open[strings.ToLower(c.Application)] = true
		}
	}
	count := map[string]int{}
	for _, r := range a.Routines() {
		if r.App != "" && !r.Focus {
			count[r.App]++
		}
	}
	out := []Target{}
	for app, n := range count {
		out = append(out, Target{App: app, Open: open[strings.ToLower(app)], Context: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].App < out[j].App })
	return out
}
