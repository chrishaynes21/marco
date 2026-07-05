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
	nearF("MatchThreshold@1", matchThresholdFromEnv(), 0.50)
	nearF("edgeMatchThreshold@1", edgeMatchThresholdFromEnv(), 0.30)
	eqI("EdgeTolerance@1", edgeToleranceFromEnv(), 50)

	// Strict end (0.0).
	t.Setenv("MARCO_CV_SENSITIVITY", "0")
	nearF("MatchThreshold@0", matchThresholdFromEnv(), 1.00)
	nearF("edgeMatchThreshold@0", edgeMatchThresholdFromEnv(), 0.80)
	eqI("EdgeTolerance@0", edgeToleranceFromEnv(), 150)
}

// The capture-time crop-size gates grow only when the dial is loosened past neutral: 1× at/
// below 0.5 (legacy), up to 4× at full-loose — so a huge game button is captured whole
// instead of rejected into the small fallback patch.
func TestCropScaleGates(t *testing.T) {
	t.Setenv("MARCO_CV_SENSITIVITY", "0.5")
	if got := cropScale(); got != 1.0 {
		t.Fatalf("cropScale@0.5 = %v, want 1.0", got)
	}
	if got := maxButtonDim(); got != 600 {
		t.Errorf("maxButtonDim@0.5 = %d, want 600 (legacy)", got)
	}
	if got := maxRecenterPx(); got != 140 {
		t.Errorf("maxRecenterPx@0.5 = %d, want 140 (legacy)", got)
	}
	if got := buttonSearchWin(); got != 480 {
		t.Errorf("buttonSearchWin@0.5 = %d, want 480 (legacy)", got)
	}

	t.Setenv("MARCO_CV_SENSITIVITY", "0") // stricter dial must NOT shrink crops below legacy
	if got := cropScale(); got != 1.0 {
		t.Errorf("cropScale@0 = %v, want 1.0 (floored at legacy)", got)
	}

	t.Setenv("MARCO_CV_SENSITIVITY", "1") // full-loose → 4×
	if got := cropScale(); got != 4.0 {
		t.Fatalf("cropScale@1 = %v, want 4.0", got)
	}
	if got := maxButtonDim(); got != 2400 {
		t.Errorf("maxButtonDim@1 = %d, want 2400", got)
	}
	if got := maxRecenterPx(); got != 560 {
		t.Errorf("maxRecenterPx@1 = %d, want 560", got)
	}
	if got := buttonSearchWin(); got != 1920 {
		t.Errorf("buttonSearchWin@1 = %d, want 1920", got)
	}
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
