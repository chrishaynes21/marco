package screen

import "testing"

// The CV find dial ($MARCO_CV_SENSITIVITY) also drives the screen matcher's default gates:
// MatchThreshold (pixel found-gate), edgeMatchThreshold (edge found-gate) and EdgeTolerance
// (edge detection for cropping + matching). Neutral 0.5 must reproduce the legacy defaults;
// looser (1.0) must lower every gate so a present-but-imperfect button still registers.
func TestCVSensitivityGates(t *testing.T) {
	nearF := func(name string, got, want float64) {
		if d := got - want; d > 1e-9 || d < -1e-9 {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	eqI := func(name string, got, want int) {
		if got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}

	// Neutral 0.5 → legacy fixed values.
	t.Setenv("MARCO_CV_SENSITIVITY", "0.5")
	nearF("MatchThreshold@0.5", matchThresholdFromEnv(), 0.75)
	nearF("edgeMatchThreshold@0.5", edgeMatchThresholdFromEnv(), 0.55)
	eqI("EdgeTolerance@0.5", edgeToleranceFromEnv(), 100)

	// Loose end (1.0) — every gate at its most permissive (lower).
	t.Setenv("MARCO_CV_SENSITIVITY", "1")
	nearF("MatchThreshold@1", matchThresholdFromEnv(), 0.60)
	nearF("edgeMatchThreshold@1", edgeMatchThresholdFromEnv(), 0.40)
	eqI("EdgeTolerance@1", edgeToleranceFromEnv(), 70)

	// Strict end (0.0).
	t.Setenv("MARCO_CV_SENSITIVITY", "0")
	nearF("MatchThreshold@0", matchThresholdFromEnv(), 0.90)
	nearF("edgeMatchThreshold@0", edgeMatchThresholdFromEnv(), 0.70)
	eqI("EdgeTolerance@0", edgeToleranceFromEnv(), 130)
}

// A knob's own env var still overrides the dial-derived default.
func TestGateEnvOverride(t *testing.T) {
	t.Setenv("MARCO_CV_SENSITIVITY", "1") // would loosen everything
	t.Setenv("MARCO_FIND_THRESHOLD", "0.82")
	t.Setenv("MARCO_EDGE_MATCH", "0.66")
	t.Setenv("MARCO_EDGE_TOLERANCE", "120")
	if got := matchThresholdFromEnv(); got != 0.82 {
		t.Errorf("MARCO_FIND_THRESHOLD override = %v, want 0.82", got)
	}
	if got := edgeMatchThresholdFromEnv(); got != 0.66 {
		t.Errorf("MARCO_EDGE_MATCH override = %v, want 0.66", got)
	}
	if got := edgeToleranceFromEnv(); got != 120 {
		t.Errorf("MARCO_EDGE_TOLERANCE override = %d, want 120", got)
	}
}
