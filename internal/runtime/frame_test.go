package runtime

import (
	"sync"
	"testing"
)

func TestStatusFromNameRoundTrip(t *testing.T) {
	all := []Status{
		StatusCreated, StatusRunning, StatusWaiting, StatusOK,
		StatusFailed, StatusCanceled, StatusExited, StatusDied,
	}
	for _, s := range all {
		name := s.String()
		got, ok := StatusFromName(name)
		if !ok {
			t.Errorf("StatusFromName(%q) not found", name)
			continue
		}
		if got != s {
			t.Errorf("round trip for %q = %v, want %v", name, got, s)
		}
	}
}

func TestStatusFromNameUnknown(t *testing.T) {
	if _, ok := StatusFromName("bogus"); ok {
		t.Error("StatusFromName(bogus) reported ok")
	}
	if _, ok := StatusFromName(""); ok {
		t.Error("StatusFromName(empty) reported ok")
	}
}

func TestStatusStringUnknown(t *testing.T) {
	if got := Status(999).String(); got != "?" {
		t.Errorf("unknown Status.String() = %q, want ?", got)
	}
}

func TestFrameIsTerminal(t *testing.T) {
	terminal := []Status{StatusOK, StatusFailed, StatusCanceled, StatusExited, StatusDied}
	nonTerminal := []Status{StatusCreated, StatusRunning, StatusWaiting}
	for _, s := range terminal {
		f := &Frame{status: s}
		if !f.IsTerminal() {
			t.Errorf("Status %v should be terminal", s)
		}
	}
	for _, s := range nonTerminal {
		f := &Frame{status: s}
		if f.IsTerminal() {
			t.Errorf("Status %v should not be terminal", s)
		}
	}
}

func TestFrameLocals(t *testing.T) {
	f := &Frame{}
	if _, ok := f.localGet("x"); ok {
		t.Error("localGet on empty frame reported ok")
	}
	f.localSet("x", Number(1))
	v, ok := f.localGet("x")
	if !ok || v.String() != "1" {
		t.Errorf("localGet(x) = %v,%v want 1,true", v, ok)
	}
	f.localSet("x", Number(2)) // overwrite
	v, _ = f.localGet("x")
	if v.String() != "2" {
		t.Errorf("after overwrite localGet(x) = %q, want 2", v.String())
	}
	f.localDelete("x")
	if _, ok := f.localGet("x"); ok {
		t.Error("localGet after delete reported ok")
	}
	// Deleting a missing key is a no-op, not a panic.
	f.localDelete("never-existed")
}

func TestFrameBindAndLookup(t *testing.T) {
	parent := &Frame{ID: 1}
	child := &Frame{ID: 2}
	if parent.lookupFrame("save") != nil {
		t.Error("lookup before bind returned non-nil")
	}
	parent.bindFrame("save", child)
	if got := parent.lookupFrame("save"); got != child {
		t.Errorf("lookupFrame(save) = %v, want child", got)
	}
	if parent.lookupFrame("other") != nil {
		t.Error("lookup of unbound name returned non-nil")
	}
}

func TestFrameChildren(t *testing.T) {
	parent := &Frame{ID: 1}
	if len(parent.snapshotChildren()) != 0 {
		t.Error("new frame has children")
	}
	a, b := &Frame{ID: 2}, &Frame{ID: 3}
	parent.addChild(a)
	parent.addChild(b)
	kids := parent.snapshotChildren()
	if len(kids) != 2 || kids[0] != a || kids[1] != b {
		t.Errorf("snapshotChildren = %v, want [a b] in order", kids)
	}
	// Snapshot is a copy.
	kids[0] = nil
	if parent.snapshotChildren()[0] == nil {
		t.Error("snapshotChildren returned an aliased slice")
	}
	// addChild on a nil frame is a safe no-op.
	var nilFrame *Frame
	nilFrame.addChild(a)
}

// TestFrameConcurrentLocalsAndChildren exercises the mu-guarded maps/slices
// from multiple goroutines; run under -race to catch unsynchronized access.
func TestFrameConcurrentLocalsAndChildren(t *testing.T) {
	f := &Frame{ID: 1}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "k" + itoaLocal(i)
			f.localSet(key, Number(float64(i)))
			f.localGet(key)
			f.addChild(&Frame{ID: i})
			f.snapshotChildren()
			f.bindFrame(key, &Frame{ID: i})
			f.lookupFrame(key)
		}(i)
	}
	wg.Wait()
	if got := len(f.snapshotChildren()); got != 16 {
		t.Errorf("expected 16 children, got %d", got)
	}
}

func itoaLocal(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
