//go:build windows

package routes

// ONE ROUTE, ONE WRITER AT A TIME, ACROSS PROCESSES.
//
// # Why atomic writing was not enough
//
// A replacement is two operations with a decision between them: read what is there, compare it to
// the version somebody agreed to lose, and only then write. `Registry.Replace` makes the WRITE
// atomic, so nobody ever reads a half-written routine — and that says nothing about the gap
// before it. Another save landing between the comparison and the write has its version
// overwritten by a caller whose comparison was true when it was made and is not true any more.
//
// Making the comparison atomic is not a thing a file write can do. What closes it is holding the
// route for the whole of it: validate, write the source, write the demonstration, release.
//
// # Why the operating system's lock and not a lock file of our own
//
// Because a lock file has to be cleaned up by whoever took it, and the one case a lock is for is
// the one where that does not happen. A process killed mid-save — `TerminateProcess`, a power
// cut, the gate ending a survivor — leaves its file behind, and every later save then waits for a
// holder that does not exist. The remedies for that are all worse than the problem: a timeout
// long enough to be safe is long enough to be a hang, and a stale-lock breaker is a second race
// with a second set of rules.
//
// `LockFileEx` is held by a HANDLE. Windows drops it when the handle closes, which includes the
// process exiting for any reason at all, so there is no stale state to recover and no deadline to
// tune. The lock file itself is left on disk on purpose: it is a rendezvous point and holds
// nothing, so its presence means nothing and nobody has to tidy it.
//
// See [[ADR-266-one-set-of-rules-for-keeping-a-recording]].

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

const (
	lockExclusive     = 0x00000002
	lockFailImmediate = 0x00000001
)

// overlapped is the OVERLAPPED the lock calls want. Zeroed: the range starts at nought.
type overlapped struct {
	internal     uintptr
	internalHigh uintptr
	offset       uint32
	offsetHigh   uint32
	event        syscall.Handle
}

// Hold is one route, claimed. Release it or the process must end.
type Hold struct {
	f    *os.File
	path string
}

// LockPath is the rendezvous file for a route, which is beside the route and is not the route.
//
// Named for the route's whole identity rather than its slug, because two routines of the same
// name in two scopes are two routines and locking one must not stop the other being saved.
func (r Registry) LockPath(rt Route) string {
	return filepath.Join(r.locDir(rt), rt.Slug+".marco.lock")
}

// Lock claims a route for one writer, waiting up to `within` for whoever has it.
//
// The wait is BOUNDED and the failure is an error rather than a longer wait, because the only
// thing that holds this is another save of the same routine — which is milliseconds of rendering
// and two file writes. Anything that takes longer than the bound is not a save queued ahead of
// this one, and waiting out a problem is how a window comes to look wedged.
func (r Registry) Lock(rt Route, within time.Duration) (*Hold, error) {
	if rt.Slug == "" {
		return nil, fmt.Errorf("routes: a play needs a name")
	}
	if err := os.MkdirAll(r.locDir(rt), 0o755); err != nil {
		return nil, err
	}
	path := r.LockPath(rt)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(within)
	for {
		var o overlapped
		// FAIL IMMEDIATELY AND RETRY, rather than blocking in the syscall: a
		// blocking LockFileEx cannot be bounded, and a wait nobody can end is the
		// thing this is supposed to prevent rather than cause.
		r1, _, callErr := procLockFileEx.Call(f.Fd(),
			lockExclusive|lockFailImmediate, 0, 1, 0,
			uintptr(unsafe.Pointer(&o)))
		if r1 != 0 {
			return &Hold{f: f, path: path}, nil
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, fmt.Errorf("another save of %q is still going after %s, so "+
				"this one has not been written: %v", rt.Slug, within, callErr)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// Release gives the route back. Safe on a nil Hold, so a caller can defer it beside its error
// check without a second branch.
func (h *Hold) Release() {
	if h == nil || h.f == nil {
		return
	}
	// AND THE UNLOCK IS SAID THOUGH THE CLOSE BELOW WOULD DO IT.
	//
	// Windows releases every lock on a handle when the handle closes, so removing this
	// call changes no behaviour and its gate run is recorded as redundant rather than as
	// a kill — the mutation that bites is on the CLAIM. It stays because it is the honest
	// expression of giving the route back at the point where that happens, and because
	// the day this holds a handle it does not own the close and the unlock stop being the
	// same act. Same standing as Watching.has's empty-path check.
	var o overlapped
	procUnlockFileEx.Call(h.f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&o)))
	_ = h.f.Close()
	h.f = nil
}
