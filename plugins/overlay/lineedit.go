package main

// THE COMMAND LINE EDITS LIKE AN EDITOR'S: a caret that moves.
//
//	← →            one character
//	Ctrl+← Ctrl+→  one word (to the start of the word before; to the start of the next)
//	Home End       the start of the text (after the space the badge leaves) and the end
//	Backspace Del  the character before / after the caret
//	Ctrl+Bksp/Del  the word before / after the caret
//
// Typing inserts at the caret. The caret is kept as how many characters it is from the END
// (cursorBack), so everything that rewrites the line — ↑ and ↓, Tab completion, a read-back
// reopening the line — leaves it at the end, where it has always been, by setting the line.
// Characters, not bytes: "é" is one press of ←.
//
// No selection yet: Shift with an arrow moves like the arrow.

import (
	"time"
	"unicode"
)

// caretAt is the caret as an index into the line's runes. Called with h.mu held.
func (h *model) caretAt(rs []rune) int {
	i := len(rs) - h.cursorBack
	if i < 0 {
		i = 0
	}
	return i
}

// setCaret puts the caret at rune index i of rs. Called with h.mu held.
func (h *model) setCaret(rs []rune, i int) {
	if i < 0 {
		i = 0
	}
	if i > len(rs) {
		i = len(rs)
	}
	h.cursorBack = len(rs) - i
}

// insertRune types r at the caret.
func (h *model) insertRune(r rune) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rs := []rune(h.input)
	i := h.caretAt(rs)
	out := append(append(append([]rune{}, rs[:i]...), r), rs[i:]...)
	h.input = string(out)
	h.setCaret(out, i+1)
	h.lastActive = time.Now()
}

// deleteBack is Backspace: the character (or, with word, the word) before the caret.
func (h *model) deleteBack(word bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rs := []rune(h.input)
	i := h.caretAt(rs)
	from := i - 1
	if word {
		from = wordStartBefore(rs, i)
	}
	if from < 0 || i == 0 {
		return
	}
	out := append(append([]rune{}, rs[:from]...), rs[i:]...)
	h.input = string(out)
	h.setCaret(out, from)
	h.lastActive = time.Now()
}

// deleteForward is Delete: the character (or the word) after the caret.
func (h *model) deleteForward(word bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rs := []rune(h.input)
	i := h.caretAt(rs)
	if i >= len(rs) {
		return
	}
	to := i + 1
	if word {
		to = wordStartAfter(rs, i)
	}
	out := append(append([]rune{}, rs[:i]...), rs[to:]...)
	h.input = string(out)
	h.setCaret(out, i)
	h.lastActive = time.Now()
}

// moveCaret is ← (delta -1) and → (+1), a word at a time with word.
func (h *model) moveCaret(delta int, word bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rs := []rune(h.input)
	i := h.caretAt(rs)
	switch {
	case word && delta < 0:
		i = wordStartBefore(rs, i)
	case word:
		i = wordStartAfter(rs, i)
	default:
		i += delta
	}
	// NOT INTO THE BADGE'S SPACE: the line written by the widget starts with the space after
	// the leader, and the caret before it would type in front of the separator.
	if lead := leadingSpaces(rs); i < lead {
		i = lead
	}
	h.setCaret(rs, i)
}

// caretHome is Home (the start of the text) and caretEnd is End.
func (h *model) caretHome() {
	h.mu.Lock()
	defer h.mu.Unlock()
	rs := []rune(h.input)
	h.setCaret(rs, leadingSpaces(rs))
}

func (h *model) caretEnd() { h.mu.Lock(); h.cursorBack = 0; h.mu.Unlock() }

// wordStartBefore is where Ctrl+← lands from i: back over spaces, then back over the word.
func wordStartBefore(rs []rune, i int) int {
	for i > 0 && unicode.IsSpace(rs[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(rs[i-1]) {
		i--
	}
	return i
}

// wordStartAfter is where Ctrl+→ lands from i: over the rest of this word, then over spaces.
func wordStartAfter(rs []rune, i int) int {
	for i < len(rs) && !unicode.IsSpace(rs[i]) {
		i++
	}
	for i < len(rs) && unicode.IsSpace(rs[i]) {
		i++
	}
	return i
}

func leadingSpaces(rs []rune) int {
	n := 0
	for n < len(rs) && rs[n] == ' ' {
		n++
	}
	return n
}
