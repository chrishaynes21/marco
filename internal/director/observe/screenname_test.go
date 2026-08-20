package observe_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The one string a person is allowed to write into durable memory.
//
// Everything else this repository persists is a closed vocabulary or a derived id. A screen name is
// the deliberate exception, and `UserSuppliedScreenName` is the only door — so the door is where
// the shape of an answer is checked. See [[ADR-031-the-user-names-the-stage]].
//
// The checks are not about taste. Each one is a way captured text differs from something a person
// types when asked what a place is called.

func TestUserSuppliedScreenNameTakesWhatAPersonWouldType(t *testing.T) {
	for _, in := range []string{
		"the pause menu",
		"Settings",
		"audio options (advanced)",
		"  the pause menu  ", // trimmed, not refused
	} {
		got, err := observe.UserSuppliedScreenName(in)
		if err != nil {
			t.Errorf("%q was refused: %v", in, err)
			continue
		}
		if got.String() != strings.TrimSpace(in) {
			t.Errorf("%q became %q", in, got)
		}
	}
}

// Every way an answer can fail to be a name.
//
// Deleting any one of these checks must fail this test — that is the whole reason the constructor
// exists rather than a bare conversion.
func TestUserSuppliedScreenNameRefusesWhatAPersonWouldNotType(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
	}{
		{"nothing at all", ""},
		{"only spaces", "   \t  "},
		// A PARAGRAPH. This is what an OCR dump or an accessibility subtree looks like when
		// somebody routes it here, and length is the cheapest thing that tells them apart.
		{"longer than a name", strings.Repeat("a", observe.MaxScreenNameLength+1)},
		{"exactly one over", strings.Repeat("b", observe.MaxScreenNameLength+1)},
		// CONTROL CHARACTERS. Nobody types a newline into "what is this screen called?";
		// captured text arrives full of them.
		{"a newline", "the pause\nmenu"},
		{"a tab inside", "the pause\tmenu"},
		{"a NUL", "the pause\x00menu"},
		{"a delete", "the pause\x7fmenu"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := observe.UserSuppliedScreenName(tc.in)
			if err == nil {
				t.Fatalf("%q was accepted as a screen name (became %q)", tc.in, got)
			}
			if got != "" {
				t.Errorf("a refused name still handed back %q", got)
			}
		})
	}
}

// The longest allowed name is allowed, so the bound is a bound and not an off-by-one.
func TestTheLongestAllowedScreenNameIsAllowed(t *testing.T) {
	in := strings.Repeat("a", observe.MaxScreenNameLength)
	if _, err := observe.UserSuppliedScreenName(in); err != nil {
		t.Errorf("a name of exactly %d characters was refused: %v",
			observe.MaxScreenNameLength, err)
	}
}
