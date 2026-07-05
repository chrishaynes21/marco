//go:build windows

package recorder

import "testing"

func TestCVMode(t *testing.T) {
	for in, want := range map[string]string{
		"max": "max", "1": "max", "on": "max", "yes": "max",
		"off": "off", "0": "off", "no": "off", "none": "off",
		"": "", "banana": "",
	} {
		t.Setenv("MARCO_CV", in)
		if got := cvMode(); got != want {
			t.Errorf("cvMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAnchorsEnabledHonorsCV(t *testing.T) {
	// CV=max forces anchors on even if MARCO_ANCHORS says off; CV=off forces them off.
	t.Setenv("MARCO_CV", "max")
	t.Setenv("MARCO_ANCHORS", "0")
	if !anchorsEnabled() {
		t.Error("CV=max should force anchors on despite MARCO_ANCHORS=0")
	}
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ANCHORS", "1")
	if anchorsEnabled() {
		t.Error("CV=off should force anchors off")
	}
	// CV is a feature flag that's OFF BY DEFAULT now: with no CV switch and no MARCO_ANCHORS,
	// anchors are off; MARCO_ANCHORS=1/on is the explicit opt-in.
	t.Setenv("MARCO_CV", "")
	t.Setenv("MARCO_ANCHORS", "")
	if anchorsEnabled() {
		t.Error("no CV + no MARCO_ANCHORS should default OFF (CV is a feature flag)")
	}
	t.Setenv("MARCO_ANCHORS", "on")
	if !anchorsEnabled() {
		t.Error("MARCO_ANCHORS=on should enable anchors even with no CV switch")
	}
}
