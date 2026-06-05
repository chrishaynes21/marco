package runtime

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/graph"
)

// Value is the universal data carrier in the runtime. Sets are themselves
// Values; primitives wrap their Go scalar.
type Value struct {
	tag valueTag
	s   string
	n   float64
	b   bool
	set *Set
	err *Err
	q   *Queue
	fd  *Feed
	ch  *Channel
	lst *List
}

type valueTag int

const (
	tagAbsent valueTag = iota
	tagText
	tagNumber
	tagBoolean
	tagSet
	tagError
	tagQueue
	tagFeed
	tagChannel
	tagList
)

// Constructors
func Absent() Value               { return Value{tag: tagAbsent} }
func Text(s string) Value         { return Value{tag: tagText, s: s} }
func Number(n float64) Value      { return Value{tag: tagNumber, n: n} }
func Bool(b bool) Value           { return Value{tag: tagBoolean, b: b} }
func SetVal(s *Set) Value         { return Value{tag: tagSet, set: s} }
func ErrVal(e *Err) Value         { return Value{tag: tagError, err: e} }
func QueueVal(q *Queue) Value     { return Value{tag: tagQueue, q: q} }
func FeedVal(fd *Feed) Value      { return Value{tag: tagFeed, fd: fd} }
func ChannelVal(c *Channel) Value { return Value{tag: tagChannel, ch: c} }
func ListVal(l *List) Value       { return Value{tag: tagList, lst: l} }

// Predicates
func (v Value) IsAbsent() bool { return v.tag == tagAbsent }
func (v Value) IsSet() bool    { return v.tag == tagSet }
func (v Value) IsError() bool  { return v.tag == tagError }

// Type returns the declared graph type of a typed Set, or nil if the value
// has no graph-level declared type (primitives, untyped sets, lists, etc.).
func (v Value) Type() *graph.Node {
	if v.tag == tagSet && v.set != nil {
		return v.set.Type
	}
	return nil
}

// Accessors
func (v Value) AsSet() *Set   { return v.set }
func (v Value) AsError() *Err { return v.err }
func (v Value) AsText() string {
	switch v.tag {
	case tagText:
		return v.s
	default:
		return ""
	}
}

// String renders a Value the way `log` would print it.
func (v Value) String() string {
	switch v.tag {
	case tagAbsent:
		return "<absent>"
	case tagText:
		return v.s
	case tagNumber:
		if v.n == float64(int64(v.n)) {
			return strconv.FormatInt(int64(v.n), 10)
		}
		return strconv.FormatFloat(v.n, 'g', -1, 64)
	case tagBoolean:
		if v.b {
			return "true"
		}
		return "false"
	case tagSet:
		return v.set.String()
	case tagError:
		return v.err.Message
	case tagQueue:
		return fmt.Sprintf("queue<%d>", v.q.Len())
	case tagFeed:
		return fmt.Sprintf("feed<%s>", v.fd.Name)
	case tagChannel:
		return fmt.Sprintf("channel<%s>", v.ch.Name)
	case tagList:
		return fmt.Sprintf("list<%d>", v.lst.Len())
	}
	return "<unknown>"
}

// Set is the universal structured data container. Concurrent reads and writes
// are guarded by mu — any caller using Fields directly outside this file must
// hold the lock or first call SnapshotFields.
//
// lockMu + cond + locked implement the advisory lock acquired by
// `lock <set>...` blocks. The state is observable so `wait until <set> unlocked?`
// can park externally.
type Set struct {
	Type   *graph.Node
	mu     sync.Mutex
	Fields map[string]Value

	lockMu sync.Mutex
	cond   *sync.Cond
	locked bool
}

func (s *Set) ensureCond() {
	if s.cond == nil {
		s.cond = sync.NewCond(&s.lockMu)
	}
}

// AcquireLock takes the set's advisory lock; pair with ReleaseLock.
func (s *Set) AcquireLock() {
	s.lockMu.Lock()
	s.ensureCond()
	for s.locked {
		s.cond.Wait()
	}
	s.locked = true
	s.lockMu.Unlock()
}

// ReleaseLock releases an advisory lock acquired with AcquireLock.
func (s *Set) ReleaseLock() {
	s.lockMu.Lock()
	s.locked = false
	if s.cond != nil {
		s.cond.Broadcast()
	}
	s.lockMu.Unlock()
}

// WaitUnlocked blocks until the advisory lock is observed unheld. The set
// may be re-locked by another goroutine immediately after — callers wanting
// exclusive access should use AcquireLock instead.
func (s *Set) WaitUnlocked() {
	s.lockMu.Lock()
	s.ensureCond()
	for s.locked {
		s.cond.Wait()
	}
	s.lockMu.Unlock()
}

// IsLocked reports the current advisory-lock state. The state is racy by
// nature; treat the result as advisory.
func (s *Set) IsLocked() bool {
	s.lockMu.Lock()
	defer s.lockMu.Unlock()
	return s.locked
}

func NewSet() *Set                   { return &Set{Fields: map[string]Value{}} }
func NewTypedSet(t *graph.Node) *Set { return &Set{Type: t, Fields: map[string]Value{}} }

func (s *Set) Get(name string) (Value, bool) {
	if s == nil {
		return Absent(), false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.Fields[name]
	return v, ok
}

func (s *Set) Put(name string, v Value) {
	s.mu.Lock()
	if s.Fields == nil {
		s.Fields = map[string]Value{}
	}
	s.Fields[name] = v
	s.mu.Unlock()
}

// SnapshotFields returns a copy of (sorted-key, value) pairs for safe iteration
// outside the lock.
func (s *Set) SnapshotFields() []SetField {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SetField, 0, len(s.Fields))
	keys := make([]string, 0, len(s.Fields))
	for k := range s.Fields {
		keys = append(keys, k)
	}
	sortStrings(keys)
	for _, k := range keys {
		out = append(out, SetField{Name: k, Value: s.Fields[k]})
	}
	return out
}

type SetField struct {
	Name  string
	Value Value
}

// sortStrings is a tiny inlined sort to avoid importing sort here.
func sortStrings(a []string) {
	// Insertion sort — Sets stay small in practice.
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}

func (s *Set) String() string {
	if s == nil {
		return "<nil set>"
	}
	fields := s.SnapshotFields()
	var b strings.Builder
	b.WriteString("{")
	for i, f := range fields {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s=%s", f.Name, f.Value)
	}
	b.WriteString("}")
	return b.String()
}

// Err is Marco's error value.
type Err struct {
	Type    *graph.Node
	Message string
	Fields  map[string]Value
}

func (e *Err) Get(name string) (Value, bool) {
	if e == nil {
		return Absent(), false
	}
	if name == "message" {
		if e.Message == "" {
			return Absent(), false
		}
		return Text(e.Message), true
	}
	if e.Fields == nil {
		return Absent(), false
	}
	v, ok := e.Fields[name]
	return v, ok
}

// List is an ordered, indexable container.
type List struct {
	mu    sync.Mutex
	Items []Value
}

func NewList() *List { return &List{} }

func (l *List) Append(v Value) {
	l.mu.Lock()
	l.Items = append(l.Items, v)
	l.mu.Unlock()
}

func (l *List) Get(i int) (Value, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if i < 0 || i >= len(l.Items) {
		return Absent(), false
	}
	return l.Items[i], true
}

func (l *List) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.Items)
}

// Snapshot returns a copy of the underlying items for lock-free iteration.
func (l *List) Snapshot() []Value {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Value, len(l.Items))
	copy(out, l.Items)
	return out
}

// Queue is a FIFO container.
type Queue struct {
	mu    sync.Mutex
	Items []Value
}

func NewQueue() *Queue { return &Queue{} }

func (q *Queue) Push(v Value) {
	q.mu.Lock()
	q.Items = append(q.Items, v)
	q.mu.Unlock()
}

func (q *Queue) Pop() (Value, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.Items) == 0 {
		return Absent(), false
	}
	v := q.Items[0]
	q.Items = q.Items[1:]
	return v, true
}

func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.Items)
}

// Snapshot returns a copy of pending items for iteration outside the lock.
func (q *Queue) Snapshot() []Value {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Value, len(q.Items))
	copy(out, q.Items)
	return out
}

// Listener is one registered handler on a channel or feed.
type Listener struct {
	Message    string
	FromSource string
	Body       *graph.Block
	Frame      *Frame
}

// Channel routes messages to registered listeners.
type Channel struct {
	Name      string
	mu        sync.Mutex
	Listeners []*Listener
}

func NewChannel(name string) *Channel { return &Channel{Name: name} }

func (c *Channel) AddListener(ln *Listener) {
	c.mu.Lock()
	c.Listeners = append(c.Listeners, ln)
	c.mu.Unlock()
}

func (c *Channel) SnapshotListeners() []*Listener {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*Listener, len(c.Listeners))
	copy(out, c.Listeners)
	return out
}

// Feed is a stream of messages; non-replayable feeds only deliver to listeners
// present at write-time. Replayable feeds buffer history for late readers.
type Feed struct {
	Name       string
	Replayable bool
	mu         sync.Mutex
	Listeners  []*Listener
	History    []FeedEvent
}

type FeedEvent struct {
	Message string
	Payload Value
}

func NewFeed(name string, replayable bool) *Feed {
	return &Feed{Name: name, Replayable: replayable}
}

func (f *Feed) AddListener(ln *Listener) {
	f.mu.Lock()
	f.Listeners = append(f.Listeners, ln)
	f.mu.Unlock()
}

func (f *Feed) SnapshotListeners() []*Listener {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*Listener, len(f.Listeners))
	copy(out, f.Listeners)
	return out
}

func (f *Feed) AppendHistory(ev FeedEvent) {
	f.mu.Lock()
	f.History = append(f.History, ev)
	f.mu.Unlock()
}

func (f *Feed) SnapshotHistory() []FeedEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]FeedEvent, len(f.History))
	copy(out, f.History)
	return out
}
