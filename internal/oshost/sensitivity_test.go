package oshost

import "testing"

// The CV find dial (cvSensitivity) drives findConfidence, locateFloor and moveMargin.
// The neutral 0.5 must reproduce the legacy fixed thresholds exactly, and moving the dial
// toward "loose" (1.0) must relax every knob (lower threshold, lower floor, lower margin).
func TestCVSensitivityMapping(t *testing.T) {
	approx := func(name string, got, want float64) {
		if d := got - want; d > 1e-9 || d < -1e-9 {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}

	// Neutral 0.5 → legacy values.
	t.Setenv("MARCO_CV_SENSITIVITY", "0.5")
	approx("findConfidence@0.5", findConfidence(), 0.6)
	approx("locateFloor@0.5", locateFloor(), 0.90)
	approx("moveMargin@0.5", moveMargin(), 0.12)

	// Loose end (1.0) — every knob at its most permissive.
	t.Setenv("MARCO_CV_SENSITIVITY", "1")
	approx("findConfidence@1", findConfidence(), 0.25)
	approx("locateFloor@1", locateFloor(), 0.80)
	approx("moveMargin@1", moveMargin(), 0.02)

	// Strict end (0.0) — every knob at its tightest.
	t.Setenv("MARCO_CV_SENSITIVITY", "0")
	approx("findConfidence@0", findConfidence(), 0.95)
	approx("locateFloor@0", locateFloor(), 1.0)
	approx("moveMargin@0", moveMargin(), 0.22)
}

// The find-gate timeout stretches only when the dial is loosened past neutral: 1× at/below
// 0.5 (legacy), scaling up to 4× at full-loose, so a slower-appearing target gets more time.
func TestFindTimeoutScale(t *testing.T) {
	cases := []struct {
		s    string
		want float64
	}{
		{"0", 1.0}, {"0.5", 1.0}, {"0.75", 2.5}, {"1", 4.0},
	}
	for _, c := range cases {
		t.Setenv("MARCO_CV_SENSITIVITY", c.s)
		if got := findTimeoutScale(); got-c.want > 1e-9 || got-c.want < -1e-9 {
			t.Errorf("findTimeoutScale(%s) = %v, want %v", c.s, got, c.want)
		}
	}
}

// An unset or out-of-range dial defaults to 0.5 (legacy behaviour), and an explicit
// $MARCO_FIND_CONFIDENCE still overrides the derived confidence.
func TestCVSensitivityDefaultsAndOverride(t *testing.T) {
	t.Setenv("MARCO_CV_SENSITIVITY", "")
	if got := cvSensitivity(); got != 0.5 {
		t.Fatalf("unset sensitivity = %v, want 0.5", got)
	}
	t.Setenv("MARCO_CV_SENSITIVITY", "3") // out of [0,1]
	if got := cvSensitivity(); got != 0.5 {
		t.Fatalf("out-of-range sensitivity = %v, want 0.5", got)
	}
	t.Setenv("MARCO_CV_SENSITIVITY", "1") // would give 0.3
	t.Setenv("MARCO_FIND_CONFIDENCE", "0.42")
	if got := findConfidence(); got != 0.42 {
		t.Fatalf("explicit MARCO_FIND_CONFIDENCE override = %v, want 0.42", got)
	}
}
