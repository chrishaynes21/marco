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
	// digits and F1–F24, which `vkToName` computes.
	for _, vk := range []uint16{0x41, 0x5A, 0x30, 0x39, 0x60, 0x69, 0x70, 0x7B, 0x7C, 0x87} {
		if name := vkToName(vk); !macroir.IsKeyName(name) {
			t.Errorf("the recorder writes %q for virtual key 0x%02X, and it is not a key "+
				"Marco can press", name, vk)
		}
	}
}

// THE KEYS A GAME BINDS APART ARE RECORDED APART: a numpad digit is not the top row's, numpad
// Enter is not Enter, and F13-F24 (which a gaming mouse's extra buttons often send) are named
// rather than recorded as "?".
//
// Recording any of them as the key it shares a code or a character with must fail this.
func TestTheNumpadsKeysAreRecordedApart(t *testing.T) {
	for vk, want := range map[uint16]string{
		0x61: "num1", 0x31: "1", 0x6B: "numplus", 0x6F: "numslash",
		0x7C: "f13", 0x87: "f24", 0x90: "numlock", 0x2C: "printscreen", 0x5D: "menu",
	} {
		if got := vkToName(vk); got != want {
			t.Errorf("virtual key 0x%02X is recorded as %q, want %q", vk, got, want)
		}
	}
	if got := numpadEnter(vkToName(0x0D), 0x0D, llkhfExtended); got != "numenter" {
		t.Errorf("numpad Enter is recorded as %q", got)
	}
	if got := numpadEnter(vkToName(0x0D), 0x0D, 0); got != "enter" {
		t.Errorf("the main Enter is recorded as %q", got)
	}
}
