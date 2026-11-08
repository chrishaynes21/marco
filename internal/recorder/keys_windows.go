//go:build windows

package recorder

// vkToName maps a virtual key code to the lowercase key name used by the OS act
// (matching internal/oshost keyMap) so recorded keys replay correctly.
func vkToName(vk uint16) string {
	if n, ok := vkNames[vk]; ok {
		return n
	}
	switch {
	case vk >= 0x41 && vk <= 0x5A: // A-Z
		return string(rune(vk - 0x41 + 'a'))
	case vk >= 0x30 && vk <= 0x39: // 0-9
		return string(rune(vk))
	case vk >= 0x60 && vk <= 0x69: // numpad 0-9, which games bind apart from the top row
		return "num" + string(rune(vk-0x60+'0'))
	case vk >= 0x70 && vk <= 0x87: // F1-F24: F13-F24 are what a gaming mouse's extra buttons often send
		return "f" + itoa(int(vk-0x70+1))
	}
	return "?"
}

var vkNames = map[uint16]string{
	0x08: "backspace", 0x09: "tab", 0x0D: "enter", 0x1B: "esc",
	0x14: "capslock",
	0x20: "space", 0x21: "pageup", 0x22: "pagedown", 0x23: "end", 0x24: "home",
	0x25: "left", 0x26: "up", 0x27: "right", 0x28: "down",
	0x2D: "insert", 0x2E: "delete",
	0x10: "shift", 0xA0: "shift", 0xA1: "shift",
	0x11: "ctrl", 0xA2: "ctrl", 0xA3: "ctrl",
	0x12: "alt", 0xA4: "alt", 0xA5: "alt",
	0x5B: "win", 0x5C: "win",
	0xBA: ";", 0xBB: "=", 0xBC: ",", 0xBD: "-", 0xBE: ".", 0xBF: "/",
	0xC0: "`", 0xDB: "[", 0xDC: "\\", 0xDD: "]", 0xDE: "'",
	// THE REST OF A FULL KEYBOARD, which games bind: the numpad's operators (spelt out, because
	// `+` joins a chord), and the lock, pause, print-screen and menu keys. Numpad Enter is VK_RETURN
	// with the extended flag and is told apart in keyboardProc (numpadEnter).
	0x6A: "numstar", 0x6B: "numplus", 0x6D: "numminus", 0x6E: "numdot", 0x6F: "numslash",
	0x90: "numlock", 0x91: "scrolllock", 0x13: "pause", 0x2C: "printscreen", 0x5D: "menu",
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [4]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// numpadEnter is the name for a key whose virtual key alone does not say which key it was:
// Enter, which is VK_RETURN on the main block and on the numpad, the numpad's carrying the
// extended flag. A game can bind the two apart, so they are recorded apart.
//
// Recording numpad Enter as enter must fail TestTheNumpadsKeysAreRecordedApart.
func numpadEnter(name string, vk uint16, flags uint32) string {
	if vk == 0x0D && flags&llkhfExtended != 0 {
		return "numenter"
	}
	return name
}
