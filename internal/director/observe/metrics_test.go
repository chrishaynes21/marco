package observe_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Measuring worth rather than volume.
//
// The number that looks obvious — detections per frame — is close to useless. A live
// three-minute session produced 52 samples, one stable entity and 139 anonymous ones that
// flickered; counting boxes would have scored that well.

func stab(role string, label observe.SafeLabel, presence float64, seen, flickers int) observe.Stability {
	return observe.Stability{
		Identity: observe.Digest(role + label.Text), Role: role, Label: label,
		PresenceRatio: presence, SamplesSeen: seen, SamplesTotal: 50, Flickers: flickers,
	}
}

func TestVolumeDoesNotBeatUsefulness(t *testing.T) {
	// The comparison the whole harness exists to make: many anonymous boxes versus few
	// named controls. The second must win.
	noisy := observe.Findings{Samples: 50}
	for i := 0; i < 200; i++ {
		noisy.Unstable = append(noisy.Unstable,
			stab("icon", observe.SafeLabel{}, 0.3, 15, 12))
	}
	useful := observe.Findings{Samples: 50}
	for i := 0; i < 6; i++ {
		useful.Stable = append(useful.Stable,
			stab("button", observe.SafeLabel{Text: "OK", Digest: "d"}, 0.98, 49, 0))
	}

	a := observe.Measure("noisy", "", noisy)
	b := observe.Measure("useful", "", useful)

	if observe.Assess(a, observe.DefaultSelectionThresholds()).Suitable {
		t.Error("200 anonymous flickering boxes were judged suitable")
	}
	if v := observe.Assess(b, observe.DefaultSelectionThresholds()); !v.Suitable {
		t.Errorf("six stable named controls were rejected: %v", v.Failures)
	}
	if a.AnonymousRatio <= b.AnonymousRatio {
		t.Error("the anonymous ratio does not separate them")
	}
}

func TestEveryFailureIsReportedNotJustTheFirst(t *testing.T) {
	// A backend missing one number by a little is a different proposition from one
	// missing six, and a reader choosing what to try next needs to see which.
	f := observe.Findings{Samples: 50}
	for i := 0; i < 100; i++ {
		f.Unstable = append(f.Unstable, stab("icon", observe.SafeLabel{}, 0.2, 10, 9))
	}
	v := observe.Assess(observe.Measure("bad", "", f), observe.DefaultSelectionThresholds())
	if len(v.Failures) < 4 {
		t.Fatalf("only %d failures reported for a backend failing on every axis: %v",
			len(v.Failures), v.Failures)
	}
	for _, msg := range v.Failures {
		if !strings.Contains(msg, "want") {
			t.Errorf("the failure %q does not say what was wanted", msg)
		}
	}
}

func TestAnEmptySessionScoresZeroRatherThanCrashing(t *testing.T) {
	m := observe.Measure("empty", "", observe.Findings{})
	if m.AnonymousRatio != 0 || m.FlickerRate != 0 {
		t.Fatalf("an empty session produced %+v", m)
	}
	if observe.Assess(m, observe.DefaultSelectionThresholds()).Suitable {
		t.Error("a backend that produced nothing was judged suitable")
	}
}

func TestTheComparisonIsDeterministic(t *testing.T) {
	f := observe.Findings{Samples: 20}
	for _, role := range []string{"button", "icon", "pane", "menu_item"} {
		f.Stable = append(f.Stable, stab(role, observe.SafeLabel{Text: role, Digest: "d"}, 0.9, 18, 1))
	}
	m := observe.Measure("x", "", f)
	first := observe.Compare([]observe.Metrics{m}, observe.DefaultSelectionThresholds())
	for i := 0; i < 20; i++ {
		if again := observe.Compare([]observe.Metrics{m}, observe.DefaultSelectionThresholds()); again != first {
			t.Fatal("the comparison table is not deterministic across runs")
		}
	}
}

func TestGridRolesAreCountedAsGrids(t *testing.T) {
	// The analyser stores a grid's role as "grid 2x3"; the vocabulary should read "grid"
	// rather than one entry per shape.
	f := observe.Findings{Samples: 10}
	f.Stable = append(f.Stable,
		stab("grid 2x3", observe.SafeLabel{}, 1, 10, 0),
		stab("grid 5x8", observe.SafeLabel{}, 1, 10, 0))
	m := observe.Measure("x", "", f)
	if m.RolesSeen["grid"] != 2 {
		t.Fatalf("roles seen = %v, want grid=2", m.RolesSeen)
	}
}
