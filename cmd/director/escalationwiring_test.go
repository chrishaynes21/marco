package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// THE WIRING, NOT THE RULES.
//
// # Why this file exists
//
// The escalation policy and the ambient cadence both have thorough unit tests, and a mutation
// run walked straight past three of them: reverting the production cadence to a flat interval,
// making the sensor gate decline when it has no idea, and having the gate reach its own verdict
// instead of asking the policy. Every one of those changes production behaviour and every one
// left the unit tests green, because the units were never the thing at risk.
//
// The rules were tested. What nobody had tested was that production asks them.

// A quiet desktop is watched at the quiet cadence, and the payload proves it.
//
// The supervisor's backoff already grew from a second to eight while nothing changed; what it
// governed was the gap BETWEEN twenty-second sessions, and the sessions themselves sampled at a
// flat one-second interval regardless. Measured against a File Explorer window whose single
// accessibility walk is 1.67s, that is about seven walks a session of a tree that did not
// change once.
//
// This reads the interval the supervisor actually asks for.
func TestTheAmbientSessionAsksForTheQuietCadence(t *testing.T) {
	a := &ambientObserver{attention: ambientIdle}

	var asked time.Duration
	open := ambientObserveNow
	ambientObserveNow = func(_ *Runtime, p service.ObservePayload) (service.ObserveStarted, error) {
		asked = p.Interval
		return service.ObserveStarted{}, errNoMemory
	}
	t.Cleanup(func() { ambientObserveNow = open })

	front := winctxActive
	winctxActive = func() string { return "explorer" }
	t.Cleanup(func() { winctxActive = front })

	a.watchOnce(t.Context())

	if asked != ambientIdle {
		t.Errorf("a settled supervisor opened its session at %v, want %v.\n"+
			"The attention that already decides how long to wait between sessions must "+
			"also decide how often to look inside one, or a quiet desktop still pays "+
			"the busy price for twenty seconds at a time.", asked, ambientIdle)
	}

	// And a busy one is not slowed down.
	a.attention = ambientBusy
	a.watchOnce(t.Context())
	if asked != ambientBusy {
		t.Errorf("a busy supervisor opened its session at %v, want %v — the cadence is "+
			"not tracking attention, it is pinned", asked, ambientBusy)
	}
}

// The sensor gate spends when it does not know, and declines only when it does.
//
// The dangerous failure is the quiet one: a gate that reads "no session, no memory, nothing
// settled yet" as "no more evidence needed" turns off the experiment it gates and looks like an
// optimisation while doing it. Deny on evidence, never on absence — the same rule as
// Provenance.OnlyDescribesPixels.
func TestTheSensorGateSpendsWhenItDoesNotKnow(t *testing.T) {
	// A Runtime with no observation registry at all knows nothing.
	if !(&Runtime{}).moreEvidenceIsWorthBuying() {
		t.Error("a Director with nothing watching declined to spend on extra evidence.\n" +
			"That is ignorance about the reading, not a reading that says it has " +
			"enough — and a gate that cannot tell them apart silently ends the " +
			"experiment it gates.")
	}
	if !(*Runtime)(nil).moreEvidenceIsWorthBuying() {
		t.Error("a nil Runtime declined to spend")
	}

	// AND THE CASE THE FIRST TWO CANNOT REACH: a registry that exists and has settled on
	// nothing. This is the ordinary state during startup and between sessions, it returns an
	// unplaced Place, and it is the branch a mutation slipped past until this line existed.
	if !(&Runtime{observations: &observationRegistry{}}).moreEvidenceIsWorthBuying() {
		t.Error("a Director whose registry has settled on nothing declined to spend.\n" +
			"An unplaced reading is Marco not knowing where it is, not a reading that " +
			"says it has seen enough.")
	}
}

// The gate asks the policy rather than reaching its own verdict.
//
// It would be easy, and wrong, to write `sufficiency != Sufficient` here. That answers a
// different question: it spends on the first incomplete frame of every navigation, and it
// spends forever on a game window that will be incomplete for as long as it is in front.
// The policy knows both of those and this must not have a second opinion about either.
func TestTheSensorGateRoutesThroughThePolicy(t *testing.T) {
	incomplete := observe.Sufficiency{State: observe.Incomplete,
		Reason: observe.ReasonClientAreaUnpopulated}

	// The two cases where "not sufficient" and "worth buying" disagree. If the gate ever
	// answers these the same way, it has stopped asking the policy.
	if observe.EscalationOf(observe.NeedAnswer, incomplete, 0).Worth() {
		t.Error("a page that has only just come back incomplete bought an inference; " +
			"the settle is not being applied")
	}
	if observe.EscalationOf(observe.NeedWatching, incomplete, time.Hour).Worth() {
		t.Error("background watching of a standing condition bought an inference")
	}
	if !observe.EscalationOf(observe.NeedAnswer, incomplete, time.Hour).Worth() {
		t.Fatal("a caller that waited and still cannot answer was refused; this test " +
			"cannot distinguish anything if the policy never says yes")
	}

	// And the wiring reaches it. Named explicitly: a gate that stopped calling
	// EscalationOf would pass every assertion above while spending on both cases.
	src := mustReadSource(t, "escalationwiring.go")
	if !containsAll(src, "observe.EscalationOf(", ".Worth()", "r.incompleteFor(p)") {
		t.Error("escalationwiring.go no longer decides through observe.EscalationOf.\n" +
			"A call site with its own rule spends on the first incomplete frame of " +
			"every navigation and forever on a game window.")
	}
}

func mustReadSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return string(b)
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
