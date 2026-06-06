//go:build windows

package oshost

import "strings"

// Virtual key constants (subset needed for macro keys).
const (
	vkBack      = 0x08
	vkTab       = 0x09
	vkReturn    = 0x0D
	vkShift     = 0x10
	vkControl   = 0x11
	vkAlt       = 0x12
	vkEscape    = 0x1B
	vkSpace     = 0x20
	vkEnd       = 0x23
	vkHome      = 0x24
	vkLeft      = 0x25
	vkUp        = 0x26
	vkRight     = 0x27
	vkDown      = 0x28
	vkInsert    = 0x2D
	vkDelete    = 0x2E
	vkLWin      = 0x5B
	vkRWin      = 0x5C
	vkPrior     = 0x21
	vkNext      = 0x22
	vkF1        = 0x70
	vkOEM1      = 0xBA // ;:
	vkOEMPlus   = 0xBB // =+
	vkOEMComma  = 0xBC
	vkOEMMinus  = 0xBD
	vkOEMPeriod = 0xBE
	vkOEM2      = 0xBF // /?
	vkOEM3      = 0xC0 // `~
	vkOEM4      = 0xDB // [{
	vkOEM5      = 0xDC // \|
	vkOEM6      = 0xDD // ]}
	vkOEM7      = 0xDE // '"
	vkRControl  = 0xA3
	vkRAlt      = 0xA5
)

var extendedKeys = map[uint16]bool{
	vkInsert: true, vkDelete: true, vkHome: true, vkEnd: true,
	vkPrior: true, vkNext: true,
	vkLeft: true, vkUp: true, vkRight: true, vkDown: true,
	vkRControl: true, vkRAlt: true, vkLWin: true, vkRWin: true,
}

var keyMap = map[string]uint16{
	"enter": vkReturn, "return": vkReturn,
	"esc": vkEscape, "escape": vkEscape,
	"tab": vkTab, "space": vkSpace, " ": vkSpace,
	"backspace": vkBack, "bs": vkBack,
	"delete": vkDelete, "del": vkDelete, "insert": vkInsert, "ins": vkInsert,
	"home": vkHome, "end": vkEnd,
	"pageup": vkPrior, "pgup": vkPrior, "pagedown": vkNext, "pgdn": vkNext,
	"up": vkUp, "down": vkDown, "left": vkLeft, "right": vkRight,
	"shift": vkShift, "ctrl": vkControl, "control": vkControl, "alt": vkAlt,
	"win": vkLWin, "lwin": vkLWin, "rwin": vkRWin,
	"`": vkOEM3, "~": vkOEM3, "backtick": vkOEM3,
	"-": vkOEMMinus, "=": vkOEMPlus,
	"[": vkOEM4, "]": vkOEM6, "\\": vkOEM5,
	";": vkOEM1, "'": vkOEM7, ",": vkOEMComma, ".": vkOEMPeriod, "/": vkOEM2,
}

func init() {
	for i := 0; i < 26; i++ {
		keyMap[string(rune('a'+i))] = uint16(0x41 + i)
	}
	for i := 0; i <= 9; i++ {
		keyMap[string(rune('0'+i))] = uint16(0x30 + i)
	}
	for i := 0; i < 12; i++ {
		keyMap[fmtF(i+1)] = uint16(vkF1 + i)
	}
}

func fmtF(n int) string {
	if n < 10 {
		return "f" + string(rune('0'+n))
	}
	return "f1" + string(rune('0'+n-10))
}

// resolveKey returns the VK code, scan code, and extended flag for a key name.
func resolveKey(name string) (vk uint16, scan uint16, extended bool) {
	lower := strings.ToLower(strings.TrimSpace(name))
	vk, ok := keyMap[lower]
	if !ok {
		if len(lower) == 1 {
			ch := lower[0]
			if ch >= 'a' && ch <= 'z' {
				vk = uint16(ch - 'a' + 0x41)
			} else if ch >= '0' && ch <= '9' {
				vk = uint16(ch)
			}
		}
		if vk == 0 {
			return 0, 0, false
		}
	}
	scan = mapVirtualKey(vk)
	extended = extendedKeys[vk]
	return vk, scan, extended
}
