package recorder

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// EVERY KEY THE RECORDER WRITES IS ONE MARCO CAN PRESS.
//
// A recorded name that is not in `macroir`'s list is a step written into somebody's routine that
// the host cannot press, and that the editor would refuse to leave as it is. See macroir/keys.go.
func TestEveryKeyTheRecorderWritesIsOneMarcoCanPress(t *testing.T) {
	for vk, name := range vkNames {
		if !macroir.IsKeyName(name) {
			t.Errorf("the recorder writes %q for virtual key 0x%02X, and it is not a key "+
				"Marco can press", name, vk)
		}
	}
	// AND THE KEYS IT WORKS OUT RATHER THAN LOOKS UP: letters, digits, the numpad's
	// digits and F1–F12, which `vkToName` computes.
	for _, vk := range []uint16{0x41, 0x5A, 0x30, 0x39, 0x60, 0x69, 0x70, 0x7B} {
		if name := vkToName(vk); !macroir.IsKeyName(name) {
			t.Errorf("the recorder writes %q for virtual key 0x%02X, and it is not a key "+
				"Marco can press", name, vk)
		}
	}
}
