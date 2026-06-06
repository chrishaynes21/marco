package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/graph"
)

// Status is the lifecycle state of a Frame.
type Status int

const (
	StatusCreated Status = iota
	StatusRunning
	StatusWaiting
	StatusOK
	StatusFailed
	StatusCanceled
	StatusExited
	StatusDied
)

// FromName maps a status word from source to a Status value. Returns false
// for unknown words.
func StatusFromName(name string) (Status, bool) {
	switch name {
	case "created":
		return StatusCreated, true
	case "running":
		return StatusRunning, true
	case "waiting":
		return StatusWaiting, true
	case "ok":
		return StatusOK, true
	case "failed":
		return StatusFailed, true
	case "canceled":
		return StatusCanceled, true
	case "exited":
		return StatusExited, true
	case "died":
		return StatusDied, true
	}
	return 0, false
}

func (s Status) String() string {
	switch s {
	case StatusCreated:
		return "created"
	case StatusRunning:
		return "running"
	case StatusWaiting:
		return "waiting"
	case StatusOK:
		return "ok"
	case StatusFailed:
		return "failed"
	case StatusCanceled:
		return "canceled"
	case StatusExited:
		return "exited"
	case StatusDied:
		return "died"
	}
	return "?"
}

// Frame is the runtime traversal of one block. Frames are themselves built-in
// actors: they own status, input, result, and error slots, plus parent / that
// references that drive process-mode resolution.
type Frame struct {
	ID     int
	Action *graph.Node
	Owner  *graph.Node

	// Status / result fields. status + customStatus are read and written across
	// goroutines (a spawned child resolves its own status while the parent's
	// goroutine evaluates `when that is <Status>?` predicates, fires status
	// listeners, or cancels the tree), so both are guarded by mu and reached
	// only through the accessor methods below — never touched directly.
	status       Status
	customStatus string
	Input        Value
	Result       Value
	Error        *Err

	Parent *Frame
	That   *Frame

	OwnerState *Set

	// CreatedAt is the wall-clock time the frame was instantiated. Used by
	// `it's been N seconds?` time predicates.
	CreatedAt time.Time

	// mu guards Locals, NamedFrames, StatusListeners, Children, and the
	// terminated flag. These can be touched from goroutines other than the
	// frame's own (listener installation by parents, listener firing on
	// terminal transitions, looked-up frame references inside listener bodies,
	// cancel propagation walking the tree).
	mu              sync.Mutex
	Locals          map[string]Value
	NamedFrames     map[string]*Frame
	StatusListeners []*StatusListener
	Children        []*Frame

	// Done is closed once the frame has reached a terminal status. Use
	// `terminated` (under mu) for the install-vs-fire decision; Done is just
	// the broadcast for parked waiters.
	Done chan struct{}

	// terminated flips to true under mu inside markDone, before listeners run.
	// Listener installers that observe terminated == true must fire the body
	// themselves — markDone has already drained the list.
	terminated bool

	// goctx is canceled when this frame is canceled or reaches a terminal
	// status. It is chained from the parent frame's context, so canceling a
	// frame cancels its whole subtree — matching cancelTree. Foreign host calls
	// receive it via HostCall.Ctx so a long-running host can abort on cancel.
	goctx     context.Context
	cancelCtx context.CancelFunc
}

// ctx returns the frame's cancellation context (never nil). Handed to foreign
// hosts so they can abort blocking work when the frame's tree is canceled.
func (f *Frame) ctx() context.Context {
	if f.goctx == nil {
		return context.Background()
	}
	return f.goctx
}

// StatusListener is a reactive listener for a Frame's status transition. It
// matches when the frame reaches Status (canonical or custom) and runs Body.
type StatusListener struct {
	Status string
	Body   *graph.Block
	Owner  *Frame
}

// addChild records target as a child of f. Used so cancel propagates down the
// frame tree. Idempotent for already-recorded children is not enforced — the
// caller is expected to call once per spawn.
func (f *Frame) addChild(target *Frame) {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.Children = append(f.Children, target)
	f.mu.Unlock()
}

// snapshotChildren returns a copy of f.Children; safe to walk outside the lock.
func (f *Frame) snapshotChildren() []*Frame {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*Frame, len(f.Children))
	copy(out, f.Children)
	return out
}

func (f *Frame) bindFrame(name string, target *Frame) {
	f.mu.Lock()
	if f.NamedFrames == nil {
		f.NamedFrames = map[string]*Frame{}
	}
	f.NamedFrames[name] = target
	f.mu.Unlock()
}

func (f *Frame) lookupFrame(name string) *Frame {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.NamedFrames == nil {
		return nil
	}
	return f.NamedFrames[name]
}

// isTerminalStatus reports whether s is a terminal lifecycle state.
func isTerminalStatus(s Status) bool {
	switch s {
	case StatusOK, StatusFailed, StatusCanceled, StatusExited, StatusDied:
		return true
	}
	return false
}

// IsTerminal reports whether the frame has reached a terminal lifecycle state.
func (f *Frame) IsTerminal() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return isTerminalStatus(f.status)
}

// Status returns the frame's canonical status. Safe from any goroutine.
func (f *Frame) Status() Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status
}

// CustomStatus returns the frame's custom status name (empty for canonical
// statuses). Safe from any goroutine.
func (f *Frame) CustomStatus() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.customStatus
}

// statusPair returns the (canonical, custom) status atomically so callers that
// need both never observe a torn write.
func (f *Frame) statusPair() (Status, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status, f.customStatus
}

// setStatus unconditionally sets the status pair. Used by return edges, which
// run on the frame's own goroutine and may overwrite an earlier non-terminal
// status (e.g. StatusRunning).
func (f *Frame) setStatus(s Status, custom string) {
	f.mu.Lock()
	f.status = s
	f.customStatus = custom
	f.mu.Unlock()
}

// resolveIfRunning sets the status pair only if the frame is not already
// terminal, atomically. This is the safe form of the
// `if !f.IsTerminal() { f.status = ... }` idiom: a concurrent cancel that
// terminated the frame between the check and the set is not clobbered.
func (f *Frame) resolveIfRunning(s Status, custom string) {
	f.mu.Lock()
	if !isTerminalStatus(f.status) {
		f.status = s
		f.customStatus = custom
	}
	f.mu.Unlock()
}

// cancelIfRunning marks the frame canceled unless it is already terminal,
// atomically. Returns true if it performed the cancel (so the caller knows to
// recurse into children), false if the frame was already terminal.
func (f *Frame) cancelIfRunning() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if isTerminalStatus(f.status) {
		return false
	}
	f.status = StatusCanceled
	f.customStatus = ""
	if f.cancelCtx != nil {
		f.cancelCtx()
	}
	return true
}

func (f *Frame) localGet(name string) (Value, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Locals == nil {
		return Absent(), false
	}
	v, ok := f.Locals[name]
	return v, ok
}

func (f *Frame) localSet(name string, v Value) {
	f.mu.Lock()
	if f.Locals == nil {
		f.Locals = map[string]Value{}
	}
	f.Locals[name] = v
	f.mu.Unlock()
}

func (f *Frame) localDelete(name string) {
	f.mu.Lock()
	if f.Locals != nil {
		delete(f.Locals, name)
	}
	f.mu.Unlock()
}
