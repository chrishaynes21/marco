package macroir_test

import (
	"slices"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// DIRECTOR KNOWS THE BUTTONS THE HOST PRESSES. It keeps its own copy because it may not import
// this package, and a copy is a second list: a button added here and not there is one Director
// refuses to click with.
func TestDirectorKnowsTheButtonsTheHostPresses(t *testing.T) {
	if got, want := marcoexec.MouseButtons(), macroir.MouseButtons(); !slices.Equal(got, want) {
		t.Errorf("Director's buttons are %v and the host's are %v", got, want)
	}
}

// A COMBO IS SPELLED ONE WAY: modifiers first in the recorder's order, every key by the recorder's
// name. Two spellings of the same keys must come out as one text, or a hotkey typed with one
// would never match the keys pressed.
func TestACanonicalComboIsOneSpelling(t *testing.T) {
	for in, want := range map[string]string{
		"Shift + Ctrl + 1": "ctrl+shift+1", "control+pgup": "ctrl+pageup", "rwin+escape": "win+esc",
		"alt+shift+ctrl+win+f9": "ctrl+alt+shift+win+f9", "f9": "f9", "bs": "backspace",
	} {
		if got := macroir.CanonicalChord(in); got != want {
			t.Errorf("%q is spelled %q, want %q", in, got, want)
		}
	}
}
