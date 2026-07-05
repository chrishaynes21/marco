package runtime

import (
	"sync"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/graph"
)

func TestValueConstructorsAndPredicates(t *testing.T) {
	if !Absent().IsAbsent() {
		t.Error("Absent().IsAbsent() = false")
	}
	if Text("x").IsAbsent() {
		t.Error("Text is not absent")
	}
	if !SetVal(NewSet()).IsSet() {
		t.Error("SetVal().IsSet() = false")
	}
	if !ErrVal(&Err{Message: "boom"}).IsError() {
		t.Error("ErrVal().IsError() = false")
	}
	if Text("x").IsSet() || Text("x").IsError() {
		t.Error("Text mis-reports as set/error")
	}
}

func TestValueStringFormatting(t *testing.T) {
	cases := []struct {
		name string
		v    Value
		want string
	}{
		{"absent", Absent(), "<command absent>"},
		{"text", Text("hello"), "hello"},
		{"empty text", Text(""), ""},
		{"int-valued number", Number(42), "42"},
		{"negative int", Number(-7), "-7"},
		{"zero", Number(0), "0"},
		{"float", Number(3.5), "3.5"},
		{"true", Bool(true), "true"},
		{"false", Bool(false), "false"},
		{"error", ErrVal(&Err{Message: "no state"}), "no state"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.v.String(); got != c.want {
				t.Errorf("String() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestNumberStringWholeFloatRendersAsInt(t *testing.T) {
	// A float64 that holds a whole number must print without a decimal point.
	if got := Number(100).String(); got != "100" {
		t.Errorf("Number(100).String() = %q, want 100", got)
	}
	// Large whole value still renders integral.
	if got := Number(1e6).String(); got != "1000000" {
		t.Errorf("Number(1e6).String() = %q, want 1000000", got)
	}
}

func TestContainerValueStrings(t *testing.T) {
	q := NewQueue()
	q.Push(Text("a"))
	q.Push(Text("b"))
	if got := QueueVal(q).String(); got != "queue<2>" {
		t.Errorf("queue string = %q, want queue<2>", got)
	}

	l := NewList()
	l.Append(Number(1))
	if got := ListVal(l).String(); got != "list<1>" {
		t.Errorf("list string = %q, want list<1>", got)
	}

	if got := FeedVal(NewFeed("Events", false)).String(); got != "feed<Events>" {
		t.Errorf("feed string = %q, want feed<Events>", got)
	}
	if got := ChannelVal(NewChannel("Bus")).String(); got != "channel<Bus>" {
		t.Errorf("channel string = %q, want channel<Bus>", got)
	}
}

func TestValueAccessors(t *testing.T) {
	if got := Text("hi").AsText(); got != "hi" {
		t.Errorf("AsText() = %q, want hi", got)
	}
	// AsText on a non-text returns "".
	if got := Number(5).AsText(); got != "" {
		t.Errorf("Number.AsText() = %q, want empty", got)
	}
	s := NewSet()
	if SetVal(s).AsSet() != s {
		t.Error("AsSet did not round-trip the set pointer")
	}
	e := &Err{Message: "x"}
	if ErrVal(e).AsError() != e {
		t.Error("AsError did not round-trip the error pointer")
	}
}

func TestValueTypeOnlyForTypedSet(t *testing.T) {
	if Text("x").Type() != nil {
		t.Error("primitive Type() should be nil")
	}
	if SetVal(NewSet()).Type() != nil {
		t.Error("untyped set Type() should be nil")
	}
	// A typed set reports its declared graph node.
	node := &graph.Node{Name: "SaveData", Kind: graph.KindSet}
	ts := NewTypedSet(node)
	if ts.Type != node {
		t.Fatal("NewTypedSet did not store the type node")
	}
	if got := SetVal(ts).Type(); got != node {
		t.Errorf("typed set Type() = %v, want node %q", got, node.Name)
	}
}

func TestValueStringUnknownTag(t *testing.T) {
	// Defensive default arm: a Value with an out-of-range tag renders <unknown>.
	var v Value
	v = Value{} // tagAbsent
	if v.String() != "<command absent>" {
		t.Fatalf("zero Value should be absent, got %q", v.String())
	}
	bogus := Value{tag: valueTag(99)}
	if got := bogus.String(); got != "<unknown>" {
		t.Errorf("out-of-range tag String() = %q, want <unknown>", got)
	}
}

func TestPutOnZeroValueSetInitializesFields(t *testing.T) {
	// A Set built without NewSet (zero value, nil Fields) must lazily allocate
	// on first Put rather than panic.
	s := &Set{}
	s.Put("X", Number(1))
	if v, ok := s.Get("X"); !ok || v.String() != "1" {
		t.Errorf("Put on zero-value Set failed: %v,%v", v, ok)
	}
}

func TestSetGetPut(t *testing.T) {
	s := NewSet()
	if _, ok := s.Get("missing"); ok {
		t.Error("Get on empty set reported ok")
	}
	s.Put("Path", Text("/tmp"))
	v, ok := s.Get("Path")
	if !ok || v.AsText() != "/tmp" {
		t.Errorf("Get(Path) = %v,%v want /tmp,true", v, ok)
	}
	// Overwrite.
	s.Put("Path", Text("/var"))
	v, _ = s.Get("Path")
	if v.AsText() != "/var" {
		t.Errorf("overwrite failed: %q", v.AsText())
	}
}

func TestNilSetGetIsSafe(t *testing.T) {
	var s *Set
	if v, ok := s.Get("x"); ok || !v.IsAbsent() {
		t.Errorf("nil Set.Get = %v,%v want absent,false", v, ok)
	}
	if got := s.String(); got != "<nil set>" {
		t.Errorf("nil Set.String() = %q, want <nil set>", got)
	}
	if s.SnapshotFields() != nil {
		t.Error("nil Set.SnapshotFields should be nil")
	}
}

func TestSetSnapshotFieldsSorted(t *testing.T) {
	s := NewSet()
	s.Put("Zebra", Number(1))
	s.Put("Apple", Number(2))
	s.Put("Mango", Number(3))
	got := s.SnapshotFields()
	want := []string{"Apple", "Mango", "Zebra"}
	if len(got) != len(want) {
		t.Fatalf("got %d fields, want %d", len(got), len(want))
	}
	for i, f := range got {
		if f.Name != want[i] {
			t.Errorf("field[%d] = %q, want %q (snapshot must be key-sorted)", i, f.Name, want[i])
		}
	}
}

func TestSetStringSortedAndFormatted(t *testing.T) {
	s := NewSet()
	s.Put("B", Text("two"))
	s.Put("A", Text("one"))
	if got := s.String(); got != "{A=one, B=two}" {
		t.Errorf("Set.String() = %q, want {A=one, B=two}", got)
	}
	if got := NewSet().String(); got != "{}" {
		t.Errorf("empty Set.String() = %q, want {}", got)
	}
}

func TestErrGet(t *testing.T) {
	e := &Err{Message: "boom", Fields: map[string]Value{"code": Number(42)}}
	// "message" is synthesized from the Message field.
	if v, ok := e.Get("message"); !ok || v.AsText() != "boom" {
		t.Errorf("Err.Get(message) = %v,%v want boom,true", v, ok)
	}
	// Regular field.
	if v, ok := e.Get("code"); !ok || v.String() != "42" {
		t.Errorf("Err.Get(code) = %v,%v want 42,true", v, ok)
	}
	// Missing field.
	if _, ok := e.Get("nope"); ok {
		t.Error("Err.Get(nope) reported ok")
	}
	// Non-message field lookup with nil Fields map must not panic.
	noFields := &Err{Message: "boom"}
	if _, ok := noFields.Get("code"); ok {
		t.Error("Err.Get(code) on nil Fields reported ok")
	}
}

func TestErrGetEmptyMessageIsAbsent(t *testing.T) {
	e := &Err{} // no message, no fields
	if v, ok := e.Get("message"); ok || !v.IsAbsent() {
		t.Errorf("empty-message Err.Get(message) = %v,%v want absent,false", v, ok)
	}
	var ne *Err
	if v, ok := ne.Get("anything"); ok || !v.IsAbsent() {
		t.Errorf("nil Err.Get = %v,%v want absent,false", v, ok)
	}
}

func TestListAppendGetBounds(t *testing.T) {
	l := NewList()
	l.Append(Text("a"))
	l.Append(Text("b"))
	if l.Len() != 2 {
		t.Fatalf("Len = %d, want 2", l.Len())
	}
	v, ok := l.Get(1)
	if !ok || v.AsText() != "b" {
		t.Errorf("Get(1) = %v,%v want b,true", v, ok)
	}
	// Out of bounds, both ends.
	if _, ok := l.Get(2); ok {
		t.Error("Get(2) on len-2 list reported ok")
	}
	if _, ok := l.Get(-1); ok {
		t.Error("Get(-1) reported ok")
	}
}

func TestListSnapshotIsCopy(t *testing.T) {
	l := NewList()
	l.Append(Number(1))
	snap := l.Snapshot()
	l.Append(Number(2)) // mutate after snapshot
	if len(snap) != 1 {
		t.Errorf("snapshot should be a copy frozen at len 1, got %d", len(snap))
	}
}

func TestQueueFIFO(t *testing.T) {
	q := NewQueue()
	if _, ok := q.Pop(); ok {
		t.Error("Pop on empty queue reported ok")
	}
	q.Push(Text("first"))
	q.Push(Text("second"))
	if q.Len() != 2 {
		t.Fatalf("Len = %d, want 2", q.Len())
	}
	v, ok := q.Pop()
	if !ok || v.AsText() != "first" {
		t.Errorf("first Pop = %v,%v want first,true (FIFO)", v, ok)
	}
	v, _ = q.Pop()
	if v.AsText() != "second" {
		t.Errorf("second Pop = %q, want second", v.AsText())
	}
	if q.Len() != 0 {
		t.Errorf("Len after draining = %d, want 0", q.Len())
	}
}

func TestQueueSnapshotIsCopy(t *testing.T) {
	q := NewQueue()
	q.Push(Number(1))
	snap := q.Snapshot()
	q.Pop()
	if len(snap) != 1 {
		t.Errorf("snapshot should be frozen at len 1, got %d", len(snap))
	}
}

func TestChannelListeners(t *testing.T) {
	c := NewChannel("Bus")
	if len(c.SnapshotListeners()) != 0 {
		t.Error("new channel has listeners")
	}
	ln := &Listener{Message: "Ping"}
	c.AddListener(ln)
	snap := c.SnapshotListeners()
	if len(snap) != 1 || snap[0] != ln {
		t.Errorf("AddListener did not register listener")
	}
	// Snapshot is a copy: mutating it does not affect the channel.
	snap[0] = nil
	if c.SnapshotListeners()[0] == nil {
		t.Error("SnapshotListeners returned an aliased slice")
	}
}

func TestFeedHistoryAndListeners(t *testing.T) {
	f := NewFeed("Events", true)
	if !f.Replayable {
		t.Error("Replayable flag not set")
	}
	f.AppendHistory(FeedEvent{Message: "A", Payload: Number(1)})
	f.AppendHistory(FeedEvent{Message: "B", Payload: Number(2)})
	hist := f.SnapshotHistory()
	if len(hist) != 2 || hist[0].Message != "A" || hist[1].Message != "B" {
		t.Errorf("history = %+v, want A then B", hist)
	}
	f.AddListener(&Listener{Message: "A"})
	if len(f.SnapshotListeners()) != 1 {
		t.Error("feed listener not registered")
	}
}

func TestSetLockReleaseAndIsLocked(t *testing.T) {
	s := NewSet()
	if s.IsLocked() {
		t.Error("new set reports locked")
	}
	s.AcquireLock()
	if !s.IsLocked() {
		t.Error("after AcquireLock IsLocked = false")
	}
	s.ReleaseLock()
	if s.IsLocked() {
		t.Error("after ReleaseLock IsLocked = true")
	}
}

// TestSetLockMutualExclusion confirms AcquireLock serializes concurrent
// holders: with the lock held, a competing goroutine cannot enter until
// ReleaseLock, and the critical section never overlaps.
func TestSetLockMutualExclusion(t *testing.T) {
	s := NewSet()
	const goroutines = 8
	const incsPer = 1000
	var plainCounter int // guarded only by s's advisory lock
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range incsPer {
				s.AcquireLock()
				plainCounter++ // racy iff the lock fails to exclude
				s.ReleaseLock()
			}
		})
	}
	wg.Wait()
	if want := goroutines * incsPer; plainCounter != want {
		t.Errorf("counter = %d, want %d (advisory lock failed to exclude)", plainCounter, want)
	}
}

// TestWaitUnlockedReturnsAfterRelease confirms WaitUnlocked blocks while the
// lock is held and returns once it is released.
func TestWaitUnlockedReturnsAfterRelease(t *testing.T) {
	s := NewSet()
	s.AcquireLock()

	released := make(chan struct{})
	returned := make(chan struct{})
	go func() {
		s.WaitUnlocked()
		close(returned)
	}()

	// Give the waiter a chance to park, then release.
	select {
	case <-returned:
		t.Fatal("WaitUnlocked returned while lock still held")
	default:
	}

	go func() {
		s.ReleaseLock()
		close(released)
	}()
	<-released
	<-returned // must return once unlocked; test hangs (and fails via -timeout) otherwise
}
