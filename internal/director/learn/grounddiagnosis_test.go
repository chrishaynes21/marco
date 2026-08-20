package learn_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/learn"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Watch mode has to say WHICH kind of grounding failure happened.
//
// The Normal sentence is coarse on purpose — a person is told Marco knows where they are and
// cannot point at it, which is all they can act on. But `coordinate_mapping_unreliable` reaches
// that sentence from two opposite places, and a live Explorer learn attempt refused with it while
// no surface could say which. One is ours to fix; the other is a provider limitation that must
// stay refused. See observe.ReferentDiagnosis.

// refusedGrounding is a learn session whose start was settled and could not be pointed at.
func refusedGrounding(d observe.ReferentDiagnosis) learn.Session {
	return learn.Session{
		Name: "open target", Phase: learn.ReadyForDemo, Application: "explorer",
		Start: "subj_start", StartState: observe.ScreenStateID("state_1"),
		StartReferent: &observe.VisualReferent{
			Role:        observe.ReferentLearnStart,
			Unavailable: observe.ReferentCoordinatesUnreliable,
			Diagnosis:   d,
		},
	}
}

// TestTheWatchPanelDistinguishesTheTwoUnreliableCauses is the mutation gate on the diagnosis line.
//
// Deleting the describeGrounding call from Watch must fail this. Without it the panel shows one
// string for both causes, which is the state the live run was debugged from and could not be.
func TestTheWatchPanelDistinguishesTheTwoUnreliableCauses(t *testing.T) {
	// Cause A — no trustworthy frame. The resolver stopped before the members.
	noFrame := strings.Join(refusedGrounding(observe.ReferentDiagnosis{
		Watching: true, FrameAvailable: true, FrameReliable: false,
		Refusal: observe.ReferentCoordinatesUnreliable,
	}).Watch(), "\n")

	// Cause B — the group resolved and every member sits outside its own window.
	unplaceable := strings.Join(refusedGrounding(observe.ReferentDiagnosis{
		Watching: true, FrameAvailable: true, FrameReliable: true,
		StateSettled: true, StandsForGroup: true, SubjectResolved: true,
		MembersTotal: 19, MembersPresent: 19, MembersWithRegion: 19,
		MembersOutsideWindow: 19,
		Refusal:              observe.ReferentCoordinatesUnreliable,
	}).Watch(), "\n")

	// Both reach the SAME product refusal. That is correct and is not what this test is about.
	for _, panel := range []string{noFrame, unplaceable} {
		if !strings.Contains(panel, "unavailable=coordinate_mapping_unreliable") {
			t.Fatalf("the coarse reason is missing:\n%s", panel)
		}
	}
	if noFrame == unplaceable {
		t.Fatal("the Watch panel is identical for the two causes of " +
			"coordinate_mapping_unreliable. One is a wiring defect and the other is a " +
			"provider limitation, and this is the surface that has to tell them apart")
	}

	// Cause A reads: the frame was there and unusable, and the funnel never ran.
	for _, want := range []string{"frame=yes/no", "subject=no", "members=0",
		"outside_window=0"} {
		if !strings.Contains(noFrame, want) {
			t.Errorf("the no-frame panel does not show %q:\n%s", want, noFrame)
		}
	}
	// Cause B reads: the frame was fine, the group resolved, and every member was off-window.
	for _, want := range []string{"frame=yes/yes", "subject=yes", "members=19", "present=19",
		"outside_window=19", "placeable=0", "regions=0"} {
		if !strings.Contains(unplaceable, want) {
			t.Errorf("the unplaceable-members panel does not show %q:\n%s", want, unplaceable)
		}
	}
}

// The diagnosis is developer-facing and never reaches the person.
//
// The Normal line is a sentence about what Marco knows and cannot show. A member tally there would
// be backstage leaking into the play — the same rule Diagnostics already follows.
func TestTheGroundingDiagnosisNeverReachesNormalMode(t *testing.T) {
	s := refusedGrounding(observe.ReferentDiagnosis{
		Watching: true, FrameAvailable: true, FrameReliable: true, SubjectResolved: true,
		MembersTotal: 19, MembersOutsideWindow: 19,
		Refusal: observe.ReferentCoordinatesUnreliable,
	})
	for _, g := range s.Grounded() {
		for _, leak := range []string{"members", "outside_window", "frame=", "subject=",
			"placeable", "state_1", "subj_start"} {
			if strings.Contains(g.Say, leak) {
				t.Errorf("the Normal grounding line leaks %q: %s", leak, g.Say)
			}
		}
	}
	assertNoBackstageLeak(t, s.Say())
}

// Grounding failure changes NOTHING about what Learn decided.
//
// Stated as a test because it is the invariant most at risk while a grounding defect is being
// chased: the temptation is to treat "I cannot point at the start" as "I do not have a start".
func TestAGroundingRefusalDoesNotDisturbTheEstablishedPlace(t *testing.T) {
	s := refusedGrounding(observe.ReferentDiagnosis{
		Watching: true, FrameAvailable: true, FrameReliable: true, SubjectResolved: true,
		MembersTotal: 19, MembersOutsideWindow: 19,
		Refusal: observe.ReferentCoordinatesUnreliable,
	})
	if s.Start != "subj_start" {
		t.Error("the established start was disturbed by a grounding refusal")
	}
	if s.Phase != learn.ReadyForDemo {
		t.Errorf("phase %q; a grounding refusal must not refuse the session", s.Phase)
	}
	if s.Refusal != "" {
		t.Errorf("refusal %q from a session whose only problem is that it cannot point",
			s.Refusal)
	}
	// And the endpoint still reports itself as settled, because it is.
	found := false
	for _, g := range s.Grounded() {
		if g.Label != "START" {
			continue
		}
		found = true
		if !strings.Contains(g.Say, "settled") {
			t.Errorf("the START line does not say it is settled: %s", g.Say)
		}
	}
	if !found {
		t.Error("an established start produced no grounded endpoint at all")
	}
}
