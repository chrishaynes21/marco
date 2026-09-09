package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// WHERE EVERY TALLIED CONTROL WENT ON THE WAY TO DURABLE MEMORY.
//
// A live store held ONE durable affordance against 512 established Places and tens of thousands
// of admitted controls. Between "a sweep would name this right now" and "this is durable" sit
// three gates, and from the outside working conservatism and a gate nothing reaches read exactly
// alike.
//
// So the account is produced where the decisions are made. These hold each gate to reporting its
// own outcome, and hold the two that mean different things apart.

// unsettled is the same screen still changing shape.
func unsettled(seen int, labels ...string) observe.ShadowTotals {
	t := offering(seen, labels...)
	t.States[0].Settled = false
	return t
}

func TestTheAffordanceFunnelNamesEachGate(t *testing.T) {
	known := recallStub{verdict: observe.MatchSame, subject: "subj_home"}
	th := observe.DefaultHypothesisThresholds()

	for _, tc := range []struct {
		what   string
		totals observe.ShadowTotals
		memory observe.Memory
		want   string
		n      int
	}{
		{
			"controls that keep being there",
			offering(3, "Bluetooth & devices", "Network & internet"), known,
			observe.AffordanceRecordable, 2,
		},
		{
			"a screen still changing shape",
			unsettled(3, "Bluetooth & devices", "Network & internet"), known,
			observe.AffordanceStateNotSettled, 2,
		},
		{
			"a settled screen that is no durable Place",
			offering(3, "Bluetooth & devices", "Network & internet"),
			recallStub{verdict: observe.MatchDifferent},
			observe.AffordanceStateHasNoPlace, 2,
		},
		{
			"a control seen only once",
			offering(1, "Update available"), known,
			observe.AffordanceNotRecurrent, 1,
		},
	} {
		_, funnel := observe.TargetsToRecordExplained(tc.totals, "settings", tc.memory, th)
		if funnel[tc.want] != tc.n {
			t.Errorf("%s recorded %v, want %d %q.\nA control that leaves by an unrecorded "+
				"gate is indistinguishable from one that was never tallied, which is the "+
				"silence this exists to end.", tc.what, funnel, tc.n, tc.want)
		}
		if len(funnel) != 1 {
			t.Errorf("%s recorded %d outcomes, want exactly 1: only the gate production "+
				"actually reached may be counted", tc.what, len(funnel))
		}
	}
}

// AND THE TWO REFUSALS THAT MEAN DIFFERENT THINGS STAY APART.
//
// "The screen never held still" and "the screen held still and is nowhere Marco knows" have
// opposite remedies — the first is a page that was still arriving, the second is an interface
// whose identity cannot be established. Collapsing them would put a number in the report that
// answers neither question.
func TestTheAffordanceFunnelKeepsUnsettledApartFromPlaceless(t *testing.T) {
	th := observe.DefaultHypothesisThresholds()

	_, stillMoving := observe.TargetsToRecordExplained(
		unsettled(3, "A", "B"), "settings",
		recallStub{verdict: observe.MatchSame, subject: "subj_home"}, th)
	_, nowhere := observe.TargetsToRecordExplained(
		offering(3, "A", "B"), "settings", recallStub{verdict: observe.MatchDifferent}, th)

	if stillMoving[observe.AffordanceStateNotSettled] == 0 {
		t.Error("a screen still changing shape was not reported as unsettled")
	}
	if nowhere[observe.AffordanceStateHasNoPlace] == 0 {
		t.Error("a settled screen with no durable Place was not reported as placeless")
	}
	if stillMoving[observe.AffordanceStateHasNoPlace] > 0 {
		t.Error("an unsettled screen was also reported as having no Place; the two " +
			"refusals have been collapsed and the report answers neither question")
	}
	if nowhere[observe.AffordanceStateNotSettled] > 0 {
		t.Error("a settled screen was reported as unsettled")
	}
}

// THE ACCOUNT DESCRIBES THE SAME TRAVERSAL THAT PRODUCED THE SIGNATURES.
//
// TargetsToRecord is TargetsToRecordExplained with the account dropped, so there is one set of
// gates. Two traversals would be two policies, and only one of them would be reviewed.
func TestTheAffordanceAccountDescribesTheWriteItCameFrom(t *testing.T) {
	totals := offering(3, "Bluetooth & devices", "Network & internet")
	m := recallStub{verdict: observe.MatchSame, subject: "subj_home"}
	th := observe.DefaultHypothesisThresholds()

	plain := observe.TargetsToRecord(totals, "settings", m, th)
	explained, funnel := observe.TargetsToRecordExplained(totals, "settings", m, th)

	if len(plain) != len(explained) {
		t.Fatalf("the explained traversal produced %d signatures and the plain one %d",
			len(explained), len(plain))
	}
	if funnel[observe.AffordanceRecordable] != len(explained) {
		t.Errorf("the account says %d recordable and the traversal produced %d signatures; "+
			"they are describing different runs",
			funnel[observe.AffordanceRecordable], len(explained))
	}
}
