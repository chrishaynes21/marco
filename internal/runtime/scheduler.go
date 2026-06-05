package runtime

import "sync"

// scheduler is the goroutine pool for the runtime. Every spawned task gets
// its own goroutine and runs in true parallel; per-resource locks (on Frame,
// Set, Queue, Channel, Feed, and the runner's resource maps) provide the
// synchronization that the previous single-active model gave for free.
type scheduler struct {
	wg sync.WaitGroup
}

func newScheduler() *scheduler { return &scheduler{} }

// spawn runs fn in a new goroutine and tracks it on the WaitGroup. The frame
// argument is no longer used by the scheduler itself but is kept for callers
// that may want it for diagnostics.
func (s *scheduler) spawn(_ *Frame, fn func()) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		fn()
	}()
}

// park blocks the caller until done is closed.
func (s *scheduler) park(done <-chan struct{}) { <-done }

// drive blocks until all spawned tasks have finished.
func (s *scheduler) drive() { s.wg.Wait() }
