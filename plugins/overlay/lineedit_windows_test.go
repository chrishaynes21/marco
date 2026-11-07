package main

import (
	"testing"
	"time"
)

// THE EDITOR'S KEYS QUEUE THE CARET'S ACTIONS while the command line is open, Ctrl making them a
// word, and a letter with Ctrl held types nothing. Dropping a key from handleKey must fail this.
func TestTheEditorKeysMoveTheCaret(t *testing.T) {
	prev := editing.Load()
	editing.Store(true)
	t.Cleanup(func() { editing.Store(prev); ctrlDown.Store(false) })
	for len(actCh) > 0 {
		<-actCh
	}
	expect := func(vk uint16, kind int, r rune) {
		t.Helper()
		if !handleKey(vk, true) {
			t.Errorf("key %#x reached the application while the command line was open", vk)
		}
		select {
		case a := <-actCh:
			if a.kind != kind || a.r != r {
				t.Errorf("key %#x queued %d/%d, want %d/%d", vk, a.kind, a.r, kind, r)
			}
		default:
			t.Errorf("key %#x queued nothing", vk)
		}
	}
	expect(vkLeft, actCaretLeft, 0)
	expect(vkRight, actCaretRight, 0)
	expect(vkHome, actCaretHome, 0)
	expect(vkEnd, actCaretEnd, 0)
	expect(vkDelete, actDeleteFwd, 0)
	handleKey(vkLCtrl, true)
	expect(vkLeft, actCaretLeft, 1)
	expect(vkRight, actCaretRight, 1)
	expect(vkDelete, actDeleteFwd, 1)
	expect(vkBack, actBackspaceWord, 0)
	handleKey('C', true)
	select {
	case a := <-actCh:
		t.Errorf("Ctrl+C typed: %+v", a)
	default:
	}
	handleKey(vkLCtrl, false)

	// AND THE ACTION LOOP CARRIES THEM OUT.
	h := newModel()
	ch := make(chan action, 8)
	go processFrom(ch, h, func(event) {})
	t.Cleanup(func() { close(ch) })
	h.setEditing(true)
	h.setInput(" focus chat")
	ch <- action{kind: actCaretLeft, r: 1}
	ch <- action{kind: actAppend, r: 'x'}
	ch <- action{kind: actCaretHome}
	ch <- action{kind: actDeleteFwd, r: 1}
	ch <- action{kind: actCaretEnd}
	ch <- action{kind: actBackspaceWord}
	ch <- action{kind: actAppend, r: '!'}
	waitFor(t, func() bool { return h.snapshot().input == " !" }, func() string { return h.snapshot().input })
}

// waitFor waits for ok, and fails with what got says when it never comes.
func waitFor(t *testing.T, ok func() bool, got func() string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("never came; the line is %q", got())
}
