package main

import "testing"

// type types s at the caret.
func typeIn(h *model, s string) {
	for _, r := range s {
		h.appendRune(r)
	}
}

// at is the line with the caret drawn as "|".
func at(h *model) string {
	s := h.snapshot()
	rs := []rune(s.input)
	i := len(rs) - s.cursorBack
	return string(rs[:i]) + "|" + string(rs[i:])
}

// THE COMMAND LINE EDITS LIKE AN EDITOR'S: the caret moves by character and by word, typing and
// deleting happen at it, and Home stops after the badge's space.
func TestTheCommandLineEditsAtTheCaret(t *testing.T) {
	h := newModel()
	h.setInput(" focus chat gpt")
	step := func(do func(), want string) {
		t.Helper()
		do()
		if got := at(h); got != want {
			t.Fatalf("%q, want %q", got, want)
		}
	}
	step(func() {}, " focus chat gpt|")
	step(func() { h.moveCaret(-1, false) }, " focus chat gp|t")
	step(func() { h.moveCaret(-1, true) }, " focus chat |gpt")
	step(func() { h.moveCaret(-1, true) }, " focus |chat gpt")
	step(func() { h.deleteBack(false) }, " focus|chat gpt")
	step(func() { typeIn(h, " ") }, " focus |chat gpt")
	step(func() { h.deleteForward(true) }, " focus |gpt")
	step(func() { typeIn(h, "chat") }, " focus chat|gpt")
	step(func() { h.moveCaret(1, true) }, " focus chatgpt|")
	step(func() { h.caretHome() }, " |focus chatgpt")
	step(func() { h.moveCaret(-1, false) }, " |focus chatgpt") // not into the badge's space
	step(func() { h.moveCaret(1, true) }, " focus |chatgpt")
	step(func() { h.deleteBack(true) }, " |chatgpt")
	step(func() { h.caretEnd() }, " chatgpt|")
	step(func() { h.moveCaret(1, false) }, " chatgpt|")
	step(func() { h.deleteForward(false) }, " chatgpt|")

	// CHARACTERS, NOT BYTES, through Backspace itself.
	h.setInput(" café")
	step(func() { h.backspace() }, " caf|")
	h.setInput(" café")
	step(func() { h.moveCaret(-1, false) }, " caf|é")
	step(func() { h.deleteForward(false) }, " caf|")
	step(func() { h.deleteBack(false) }, " ca|")

	// AND ANYTHING THAT REWRITES THE LINE PUTS THE CARET AT THE END.
	h.remember("open social")
	h.setEditing(true)
	h.moveCaret(-1, true)
	h.recallOlder()
	if got := at(h); got != " open social|" {
		t.Errorf("a recalled line: %q", got)
	}
	h.moveCaret(-1, true)
	h.setInput(" x")
	if got := at(h); got != " x|" {
		t.Errorf("a line set: %q", got)
	}
	h.moveCaret(-1, false)
	if got := h.takeInput(); got != "x" || h.snapshot().cursorBack != 0 {
		t.Errorf("taken %q, caret %d", got, h.snapshot().cursorBack)
	}
}
