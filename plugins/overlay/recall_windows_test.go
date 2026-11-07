package main

import (
	"testing"
	"time"
)

// ↑ AND ↓ WHILE THE COMMAND LINE IS OPEN ARE THE RECALL, swallowed from the application, and the
// action loop walks the commands with them; opening the line afresh starts a new walk.
//
// Dropping either key from handleKey, or either action from processFrom, must fail this.
func TestTheArrowKeysRecallOnTheCommandLine(t *testing.T) {
	prev := editing.Load()
	editing.Store(true)
	t.Cleanup(func() { editing.Store(prev) })
	for len(actCh) > 0 {
		<-actCh
	}
	for vk, want := range map[uint16]int{vkUp: actRecallOlder, vkDown: actRecallNewer} {
		if !handleKey(vk, true) {
			t.Errorf("key %#x reached the application while the command line was open", vk)
		}
		select {
		case a := <-actCh:
			if a.kind != want {
				t.Errorf("key %#x queued action %d, want %d", vk, a.kind, want)
			}
		default:
			t.Errorf("key %#x queued nothing", vk)
		}
	}

	h := newModel()
	h.remember("open social")
	h.remember("cloud pull")
	ch := make(chan action, 8)
	go processFrom(ch, h, func(event) {})
	t.Cleanup(func() { close(ch) })
	waitInput := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if h.snapshot().input == want {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("the line is %q, want %q", h.snapshot().input, want)
	}
	ch <- action{kind: actBeginEdit}
	ch <- action{kind: actRecallOlder}
	ch <- action{kind: actRecallOlder}
	waitInput(" open social")
	ch <- action{kind: actRecallNewer}
	waitInput(" cloud pull")
	ch <- action{kind: actBeginEdit} // a new line: the walk starts again at the newest
	ch <- action{kind: actRecallOlder}
	ch <- action{kind: actAppend, r: '!'} // handled in order, so the line says where the walk was
	waitInput(" cloud pull!")
}
