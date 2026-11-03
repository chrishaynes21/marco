//go:build !windows

package routes

// ONE ROUTE, ONE WRITER AT A TIME — WITHIN THIS PROCESS, AND IT SAYS SO.
//
// The Windows side holds the route with `LockFileEx`, which the operating system drops when the
// handle closes and therefore when the process ends, however it ends. There is no portable
// equivalent in the standard library, and the engine has no third-party dependencies to reach
// for.
//
// So this is a real lock between goroutines and NOT a lock between processes, which is a weaker
// guarantee than the contract asks for and is stated rather than implied. What it costs: two
// Marcos saving the same routine at the same moment on a platform that is not Windows can still
// have one of them lose a version it had validated. What it keeps: everything else in the save
// contract, including the exclusive creation of a fresh routine, which needs no lock at all
// because `O_CREATE|O_EXCL` is itself the critical section.
//
// It is a stub for a platform the product does not ship on. Said plainly instead of quietly,
// because a lock that does not lock is worse than one that refuses — and the honest shape of
// "this platform cannot do that" is the one `gate.Snapshot` takes.
//
// See [[ADR-266-one-set-of-rules-for-keeping-a-recording]].

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

var (
	locksMu sync.Mutex
	locks   = map[string]*sync.Mutex{}
)

// Hold is one route, claimed.
type Hold struct {
	mu *sync.Mutex
}

// LockPath is where the Windows side keeps its rendezvous file, and is the key here.
func (r Registry) LockPath(rt Route) string {
	return filepath.Join(r.locDir(rt), rt.Slug+".marco.lock")
}

// Lock claims a route for one writer within this process.
func (r Registry) Lock(rt Route, within time.Duration) (*Hold, error) {
	if rt.Slug == "" {
		return nil, fmt.Errorf("routes: a play needs a name")
	}
	return lockFile("", r.LockPath(rt), "another save of "+fmt.Sprintf("%q", rt.Slug), within)
}

// lockFile is the one claim, keyed by its rendezvous path. See the Windows side.
func lockFile(_, key, what string, within time.Duration) (*Hold, error) {
	locksMu.Lock()
	mu, ok := locks[key]
	if !ok {
		mu = &sync.Mutex{}
		locks[key] = mu
	}
	locksMu.Unlock()
	// BOUNDED, like the other side: a wait nobody can end is the thing this prevents
	// rather than causes.
	done := make(chan struct{})
	go func() { mu.Lock(); close(done) }()
	select {
	case <-done:
		return &Hold{mu: mu}, nil
	case <-time.After(within):
		return nil, fmt.Errorf("%s is still going after %s, so this "+
			"one has not been written", what, within)
	}
}

// Release gives the route back. Safe on a nil Hold.
func (h *Hold) Release() {
	if h == nil || h.mu == nil {
		return
	}
	h.mu.Unlock()
	h.mu = nil
}
