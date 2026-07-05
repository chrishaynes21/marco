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
	// With no CV switch, the existing MARCO_ANCHORS default (on) and override (off) hold.
	t.Setenv("MARCO_CV", "")
	t.Setenv("MARCO_ANCHORS", "0")
	if anchorsEnabled() {
		t.Error("no CV + MARCO_ANCHORS=0 should be off")
	}
	t.Setenv("MARCO_ANCHORS", "")
	if !anchorsEnabled() {
		t.Error("no CV + no MARCO_ANCHORS should default on")
	}
}
