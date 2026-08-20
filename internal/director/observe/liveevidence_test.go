package observe_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The real Windows Settings evidence, replayed.
//
// # Why this is a fixture and not a live run
//
// Two `director teach` attempts against Windows Settings, cold store, vision and OCR off, both
// refused with "2 transition(s) were seen and none had two recognisable endpoints" — while both
// endpoints sat in the durable store as recognisable subjects. The sessions existed only in a
// running Director's memory, so they were captured before the process was restarted.
//
// They are byte-identical in shape despite one visit lasting 67 observations and the other 9,
// which is what rules out timing. Kept because this defect was invisible to every synthetic
// fixture in the suite: the topology tests all script screens that recur, and a screen that
// recurs is never separated from its neighbour by a frame nobody could place.

// liveSettings loads one captured session's accumulated evidence.
func liveSettings(t *testing.T, name string) observe.ShadowTotals {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading the captured session: %v", err)
	}
	var totals observe.ShadowTotals
	if err := json.Unmarshal(raw, &totals); err != nil {
		t.Fatalf("decoding the captured session: %v", err)
	}
	if len(totals.Crossings) == 0 {
		totals.Crossings = reconstructWalk(t, totals)
	}
	return totals
}

// reconstructWalk recovers the ORDER of a session recorded before the segmenter kept it.
//
// These captures predate `ShadowTotals.Crossings` and cannot be re-recorded: they are two real
// teach attempts against Windows Settings, taken out of a running Director's memory before the
// process was restarted, and reproducing them needs a person at the machine.
//
// It is a reconstruction and not a fabrication, and the difference is the guard below: a walk is
// only recovered when the aggregate leaves exactly ONE possible ordering — one entry into the
// unplaced state and one exit out of it. With two of either the order is genuinely unrecoverable,
// which is the whole reason the producer now records it, and this refuses rather than guesses.
func reconstructWalk(t *testing.T, totals observe.ShadowTotals) []observe.Crossing {
	t.Helper()
	var entry, exit *observe.ScreenTransition
	entries, exits, direct := 0, 0, 0
	for i := range totals.Transitions {
		tr := &totals.Transitions[i]
		switch {
		case tr.To == observe.ScreenStateUnknown && tr.From != observe.ScreenStateUnknown:
			entries++
			entry = tr
		case tr.From == observe.ScreenStateUnknown && tr.To != observe.ScreenStateUnknown:
			exits++
			exit = tr
		default:
			direct++
		}
	}
	if entries != 1 || exits != 1 || direct != 0 {
		t.Fatalf("this fixture has %d entries, %d exits and %d direct change(s), so its "+
			"order is not uniquely determined and may not be reconstructed. Re-record it "+
			"against a producer that keeps the walk.", entries, exits, direct)
	}
	return []observe.Crossing{
		{From: entry.From, To: observe.ScreenStateUnknown},
		// The gap length is the aggregate's, which is exact here: one crossing of one
		// edge, so the longest run that edge ever crossed IS this run.
		{From: observe.ScreenStateUnknown, To: exit.To, Run: exit.UnsettledRun},
	}
}

// settingsMemory is the durable store as it stood when those sessions ran: both endpoints
// established, and nothing known about either.
//
// Built from the two states' own signatures rather than from copied JSON, so it cannot drift out
// of step with the fixture — and it holds ZERO semantic knowledge, exactly as the real file did.
func settingsMemory(t *testing.T, totals observe.ShadowTotals) *recallingMemory {
	t.Helper()
	th := observe.DefaultHypothesisThresholds()
	m := &recallingMemory{}
	for _, st := range totals.States {
		sig, ok := observe.SignatureOfState(totals, st.ID, th)
		if !ok {
			t.Fatalf("state %s produced no signature", st.ID)
		}
		if !sig.Discriminating() {
			t.Fatalf("state %s carries no discriminator, so it could not have been stored",
				st.ID)
		}
		m.subjects = append(m.subjects, observe.RememberedSubject{
			ID: "subj_" + string(st.ID), Application: "applicationframehost",
			Structure: sig, Sessions: 1,
		})
	}
	if len(m.subjects) != 2 {
		t.Fatalf("the fixture holds %d states, want 2", len(m.subjects))
	}
	return m
}

// recallingMemory resolves through the REAL matcher. Nothing here decides identity.
type recallingMemory struct {
	subjects []observe.RememberedSubject
}

func (m *recallingMemory) Recall(_ string, sig observe.StructureSignature) observe.Recollection {
	s, v := observe.Recall(sig, m.subjects)
	return observe.Recollection{Verdict: v, Subject: s}
}

func (m *recallingMemory) Remember(string, observe.StructureSignature, observe.SemanticKnowledge) error {
	return nil
}
func (m *recallingMemory) RememberRelationships(string, []observe.RelationshipObservation) (
	observe.RelationshipUpdate, error) {
	return observe.RelationshipUpdate{}, nil
}
func (m *recallingMemory) Topology(string) observe.Topology { return observe.Topology{} }
func (m *recallingMemory) RememberLearning(string, observe.RelationshipRef, observe.LearningRequest) error {
	return nil
}
func (m *recallingMemory) RememberFollowUp(string, observe.RelationshipRef, observe.LearningRequest) error {
	return nil
}
func (m *recallingMemory) FulfilLearning(string, observe.RelationshipRef, int) error { return nil }

// ── the measurement ───────────────────────────────────────────────────────────

// THE proof. The two real transitions' causes, named rather than inferred.
//
// Before the bridge existed this is the whole story: `state_1 → state_unknown` fails because its
// DESTINATION is not a durable subject, `state_unknown → state_2` because its SOURCE is not, and
// the user's `confirm` is attached to the first of them.
func TestTheLiveSettingsTransitionsNameTheirCauses(t *testing.T) {
	for _, name := range []string{"live_settings_7.json", "live_settings_11.json"} {
		t.Run(name, func(t *testing.T) {
			totals := liveSettings(t, name)

			// The shape, asserted so a re-capture that changed it cannot pass quietly.
			if len(totals.Transitions) != 2 {
				t.Fatalf("%d transition(s), want 2", len(totals.Transitions))
			}
			var entry, exit observe.ScreenTransition
			for _, tr := range totals.Transitions {
				switch {
				case tr.To == observe.ScreenStateUnknown:
					entry = tr
				case tr.From == observe.ScreenStateUnknown:
					exit = tr
				}
			}
			if entry.From == "" || exit.To == "" {
				t.Fatalf("the captured transitions do not cross the unknown state: %+v",
					totals.Transitions)
			}
			if entry.Preceded[observe.NavConfirm] == 0 {
				t.Errorf("the user's confirm is not attached to the entry leg: %+v", entry)
			}

			// Both endpoints ARE durable and recognisable. Stated first, because every
			// cause below would be uninteresting if they were not.
			m := settingsMemory(t, totals)
			for _, id := range []observe.ScreenStateID{entry.From, exit.To} {
				sig, _ := observe.SignatureOfState(totals, id,
					observe.DefaultHypothesisThresholds())
				if v := m.Recall("applicationframehost", sig).Verdict; !v.Established() {
					t.Fatalf("%s recalls as %q; the premise of this whole measurement is "+
						"that both ends are recognisable", id, v)
				}
			}

			// WITHOUT continuity evidence the bridge must not fire, so this reads the
			// pre-bridge behaviour: the two causes, named.
			_, report := observe.RelationshipsFrom(totals,
				observe.Hypotheses(totals, observe.DefaultHypothesisThresholds()),
				"applicationframehost", m, observe.Continuity{TargetLosses: 1})

			if report.Durable != 0 {
				t.Fatalf("%d durable edge(s) without continuity evidence", report.Durable)
			}
			if report.SessionLocal != 2 {
				t.Fatalf("session_local = %d, want 2", report.SessionLocal)
			}
			want := map[observe.SessionLocalCause]int{
				observe.DestinationUnresolved: 1, // state_1 → state_unknown
				observe.SourceUnresolved:      1, // state_unknown → state_2
			}
			for cause, n := range want {
				if report.SessionLocalCauses[cause] != n {
					t.Errorf("cause %q = %d, want %d\nall causes: %v",
						cause, report.SessionLocalCauses[cause], n,
						report.SessionLocalCauses)
				}
			}
			if len(report.SessionLocalCauses) != len(want) {
				t.Errorf("unexpected causes present: %v", report.SessionLocalCauses)
			}
		})
	}
}

// And with the interval proven continuous, the adjacency the user demonstrated comes back.
func TestTheLiveSettingsRouteIsRecoveredByBridging(t *testing.T) {
	for _, name := range []string{"live_settings_7.json", "live_settings_11.json"} {
		t.Run(name, func(t *testing.T) {
			totals := liveSettings(t, name)
			m := settingsMemory(t, totals)

			obs, report := observe.RelationshipsFrom(totals,
				observe.Hypotheses(totals, observe.DefaultHypothesisThresholds()),
				"applicationframehost", m, observe.Continuity{Generations: 1})

			if report.Bridged != 1 || len(obs) != 1 {
				t.Fatalf("bridged=%d observations=%d; the route the user demonstrated was "+
					"not recovered.\ncauses: %v\nrefusals: %v",
					report.Bridged, len(obs), report.SessionLocalCauses,
					report.BridgeRefusals)
			}
			got := obs[0]
			if got.From == got.To {
				t.Fatalf("the recovered edge is a self-loop: %s → %s", got.From, got.To)
			}
			// The navigation survives the bridge. Without it the demonstration would be
			// refused `action_not_attributed` even though the user pressed a key.
			if got.Evidence.Preceded[observe.NavConfirm] == 0 {
				t.Errorf("the recovered edge lost the user's confirm: %+v", got.Evidence)
			}
			// And it says what it is.
			if got.Evidence.Bridged == 0 {
				t.Error("the recovered edge does not record that it was bridged, so nothing " +
					"downstream can weigh it differently from one seen whole")
			}
		})
	}
}
