package oshost

import (
	"sort"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// THE HOST PRESSES EXACTLY THE KEYS MARCO NAMES.
//
// `macroir`'s list is what a person's typing is checked against when a step's key is edited.
// A name there that the host cannot resolve is an edit accepted at review and refused at run
// time; a name the host presses that is not there is a key somebody cannot type into the
// editor. Both directions, because either drift is one of the three lists telling a different
// story (see macroir/keys.go).
func TestTheHostPressesExactlyTheKeysMarcoNames(t *testing.T) {
	named := map[string]bool{}
	for _, k := range macroir.KeyNames() {
		named[k] = true
		if vk, scan, _ := resolveKey(k); vk == 0 && scan == 0 {
			t.Errorf("macroir names %q as a key Marco can press, and this host cannot "+
				"press it", k)
		}
	}
	var extra []string
	for k := range keyMap {
		// A TABLE ENTRY THE LOOKUP CAN NEVER REACH is not a key this host presses:
		// `resolveKey` trims the name, so " " is unreachable and is not asked about.
		if vk, scan, _ := resolveKey(k); vk == 0 && scan == 0 {
			continue
		}
		if !named[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	for _, k := range extra {
		t.Errorf("this host presses %q, and macroir does not name it, so it cannot be "+
			"typed into a step", k)
	}
}

// AND THEY ARE PRESSED APART: numpad Enter is Enter sent as an extended key, the numpad's digits
// and operators are their own virtual keys, and F13-F24 resolve.
//
// Pressing numpad Enter as Enter, or a numpad digit as the top row's, must fail this.
func TestTheNumpadsKeysArePressedApart(t *testing.T) {
	for name, want := range map[string]struct {
		vk       uint16
		extended bool
	}{
		"numenter": {0x0D, true}, "enter": {0x0D, false}, "num1": {0x61, false},
		"1": {0x31, false}, "numslash": {0x6F, true}, "numplus": {0x6B, false},
		"f13": {0x7C, false}, "f24": {0x87, false}, "numlock": {0x90, true},
	} {
		vk, _, ext := resolveKey(name)
		if vk != want.vk || ext != want.extended {
			t.Errorf("%q is pressed as virtual key 0x%02X extended=%v, want 0x%02X extended=%v",
				name, vk, ext, want.vk, want.extended)
		}
	}
}
