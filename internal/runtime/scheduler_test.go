package runtime

import (
	"sync/atomic"
	"testing"
)

// TestSchedulerDriveWaitsForAll confirms drive() blocks until every spawned
// task has finished — the WaitGroup barrier the multi-active model relies on.
func TestSchedulerDriveWaitsForAll(t *testing.T) {
	s := newScheduler()
	const n = 50
	var done atomic.Int64
	for range n {
		s.spawn(nil, func() {
			done.Add(1)
		})
	}
	s.drive()
	if got := done.Load(); got != n {
		t.Errorf("after drive, %d/%d tasks finished — drive returned early", got, n)
	}
}

// TestSchedulerParkUnblocksOnClose confirms park returns once its done channel
// is closed (and only then).
func TestSchedulerParkUnblocksOnClose(t *testing.T) {
	s := newScheduler()
	done := make(chan struct{})
	parked := make(chan struct{})
	s.spawn(nil, func() {
		s.park(done)
		close(parked)
	})

	select {
	case <-parked:
		t.Fatal("park returned before done was closed")
	default:
	}

	close(done)
	s.drive() // the parked task should now complete
	select {
	case <-parked:
	default:
		t.Fatal("park did not return after done closed")
	}
}

// TestSchedulerNestedSpawn confirms a task may spawn further tasks and drive
// still waits for the whole transitive set.
func TestSchedulerNestedSpawn(t *testing.T) {
	s := newScheduler()
	var count atomic.Int64
	var release = make(chan struct{})
	s.spawn(nil, func() {
		count.Add(1)
		// Spawn a child before this task returns so the WaitGroup count is
		// incremented before drive() can observe zero.
		s.spawn(nil, func() {
			<-release
			count.Add(1)
		})
	})
	// Let the outer task register the child, then release it.
	close(release)
	s.drive()
	if got := count.Load(); got != 2 {
		t.Errorf("count = %d, want 2 (drive must wait for nested spawns)", got)
	}
}
