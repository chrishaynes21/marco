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
