package macroir

// THE MOUSE BUTTONS MARCO CAN PRESS, BY NAME.
//
// A click or a drag carries one of these in `Step.Button` (empty means left). The recorder writes
// them, the host presses them, and Studio and Director check a button somebody chose against
// them, so this is the one list and every one of those reads it.
//
// `back` and `forward` are the side buttons (Windows' XBUTTON1 and XBUTTON2), which games bind
// as often as the wheel button. They were missing from all four places: the recorder never read
// them, so a ctrl+shift+forward demonstrated as nothing at all (the modifiers had no click to
// hold across and were dropped as having modified nothing).
var mouseButtons = []string{"left", "right", "middle", "back", "forward"}

// MouseButtons is every button name Marco can press, in the order a person would list them.
func MouseButtons() []string { return append([]string(nil), mouseButtons...) }

// IsMouseButton says whether name is one of them, as `Step.Button` spells it: lowercased.
func IsMouseButton(name string) bool {
	for _, b := range mouseButtons {
		if b == name {
			return true
		}
	}
	return false
}

// MouseButtonList is the buttons as a sentence lists them: "left, right, middle, back and
// forward".
func MouseButtonList() string {
	out := ""
	for i, b := range mouseButtons {
		switch {
		case i == 0:
		case i == len(mouseButtons)-1:
			out += " and "
		default:
			out += ", "
		}
		out += b
	}
	return out
}
