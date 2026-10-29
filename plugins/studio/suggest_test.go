package main

// WHICH APPLICATIONS A REQUEST CAN BE AIMED AT, WHICH IS ALL THIS FILE IS NOW ABOUT.
//
// Everything else that was here moved to `internal/resolveroute` with the policy it was about:
// scope eligibility, label distinguishability, namesakes, which arm answered, and that an answer
// nobody offered selects nothing. They are the same claims, held where the decision now lives and
// where both surfaces reach them. See resolve.go for what this window does instead.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// aLibraryWithAnInPlaceRoutine saves one that works only while its application is in front.
//
// Its press states no window, which is what lets an in-place routine be saved and still run.
func aLibraryWithAnInPlaceRoutine(t *testing.T) *Acts {
	t.Helper()
	return aSavedRoutine(t, string(orchestrator.ScopeApp), func(p *recorder.RecordedEvent) {
		p.WinRel, p.RelTo = false, ""
	})
}

// AND THE TARGETS ARE THE APPLICATIONS A TARGET CHANGES SOMETHING FOR.
func TestTheTargetsAreTheApplicationsWithInPlaceRoutines(t *testing.T) {
	a := aLibraryWithAnInPlaceRoutine(t)
	// NO WINDOWS AT ALL, so `Open` is false everywhere — and the application is still
	// offered as a target, because whether it is running is a hint and not a filter.
	a.listing = func() []windowref.Candidate { return nil }
	ts := a.SuggestTargets()
	if len(ts) != 1 || ts[0].App == "" || ts[0].Context != 1 {
		t.Fatalf("the targets are %+v, want the one application with an in-place "+
			"routine", ts)
	}
	if ts[0].Open {
		t.Errorf("%s is reported open with no windows at all", ts[0].App)
	}
}
