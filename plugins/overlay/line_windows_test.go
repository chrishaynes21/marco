//go:build windows

package main

import "testing"

// The command line types what a value needs, and the key after the leader is not widened: a
// punctuation key there is still no shortcut.
func TestTheCommandLineTypesWhatAValueNeeds(t *testing.T) {
	for _, c := range []struct {
		vk    uint16
		shift bool
		want  rune
	}{{0xBA, true, ':'}, {0xBA, false, ';'}, {0xBC, false, ','}, {0xBE, false, '.'}, {0xBF, false, '/'},
		{0xDE, false, '\''}, {0xDE, true, '"'}, {'8', true, '*'}, {'0', true, ')'}, {vkOEMMin, true, '_'},
		{'A', true, 'A'}, {'7', false, '7'}, {vkSpace, false, ' '}} {
		if got, ok := vkToText(c.vk, c.shift); !ok || got != c.want {
			t.Errorf("key %#x shift=%v types %q (%v), want %q", c.vk, c.shift, got, ok, c.want)
		}
	}
	for _, vk := range []uint16{0xBA, 0xBC, 0xBE, 0xDE} {
		if _, ok := vkToRune(vk, false); ok {
			t.Errorf("key %#x became a key after the leader", vk)
		}
	}
}
