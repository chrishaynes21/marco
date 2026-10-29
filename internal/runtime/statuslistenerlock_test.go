package runtime

import (
	"io"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/graph"
)

// A STATUS LISTENER INSTALLED ON A FRAME THAT HAS ALREADY FINISHED MUST NOT DEADLOCK.
//
// # The defect this is about
//
// `tryInstallStatusListener` decides under `child.mu` whether the child has already
// terminated, and the version that asked `statusMatches` for the answer went straight back into
// `Frame.statusPair`, which takes the same mutex. A `sync.Mutex` is not reentrant, so the
// goroutine blocked on ITSELF, the scheduler went on waiting on its WaitGroup, and the program
// never finished. A Marco program whose observation body attaches just after its target
// finishes hung forever.
//
// # Why nothing caught it
//
// It needs the child to have terminated BEFORE the listener installs, which is a race the child
// usually loses: win it the ordinary way and the listener is appended to `StatusListeners` and
// nothing locks twice. So it was INTERMITTENT and it was never about the program being run — it
// hung `TestGolden/lock_mutex` for the whole ten-minute package timeout once, and
// `TestGolden/saveapp_full` on the twelfth attempt of a hunt that ran the same five-count forty
// times. Two different fixtures, neither of them the subject.
//
// This states the moment instead of waiting for it, which is the difference between a tested
// mechanism and a tested moment.
//
// # Why it answers through a channel
//
// A regression here is a HANG, and a hang in a test is a ten-minute package timeout that says
// `panic: test timed out` and names whichever test was unlucky. The deadline makes a regression
// fail in five seconds, as itself, with a sentence about what happened.
func TestAListenerOnAFrameThatAlreadyFinishedDoesNotDeadlock(t *testing.T) {
	r := &runner{g: &graph.Graph{}, out: io.Discard, sched: newScheduler()}
	parent := &Frame{}
	child := &Frame{}
	child.setStatus(StatusOK, "")
	// ALREADY DRAINED, which is the state `markDone` leaves behind and the branch the
	// deadlock is in. Set directly: the point is to be in that state before the install,
	// and arranging it by racing a real child is what failed to catch this for so long.
	child.mu.Lock()
	child.terminated = true
	child.mu.Unlock()

	head := graph.Edge{Pred: graph.StatusPred{Ref: []string{"the child"}, Status: "ok"}}

	installed := make(chan bool, 1)
	go func() {
		installed <- r.tryInstallStatusListener(parent, child, "the child", head,
			&graph.Block{})
	}()

	select {
	case ok := <-installed:
		if !ok {
			t.Error("the listener was not installed for a frame that had already " +
				"finished, so an observation body that attached a moment late " +
				"would never run")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("installing a status listener on a frame that had already " +
			"terminated did not return: the install decision is taken under " +
			"`child.mu` and something inside it is taking that mutex again")
	}
}

// AND THE COMPARISON ITSELF STILL ANSWERS THE SAME WAY, locked or not.
//
// The fix splits the comparison from the read, so the two have to agree: a
// `statusNameMatches` that answered differently from `statusMatches` would turn the deadlock
// into a wrong answer, which is worse.
func TestTheStatusComparisonAgreesWhetherOrNotItTakesTheLock(t *testing.T) {
	for _, c := range []struct {
		why    string
		status Status
		custom string
		asked  string
		want   bool
	}{
		{"a named status that matches", StatusOK, "", "ok", true},
		{"a named status that does not", StatusOK, "", "failed", false},
		{"a custom status that matches", StatusRunning, "halfway", "halfway", true},
		{"a custom status that does not", StatusRunning, "halfway", "elsewhere", false},
		{"a word that is no status at all", StatusOK, "", "biscuit", false},
		{"a custom status beside a named one that would match", StatusOK, "nearly",
			"ok", true},
	} {
		f := &Frame{}
		f.setStatus(c.status, c.custom)
		if got := statusMatches(f, c.asked); got != c.want {
			t.Errorf("%s: statusMatches answered %v", c.why, got)
		}
		if got := statusNameMatches(c.status, c.custom, c.asked); got != c.want {
			t.Errorf("%s: statusNameMatches answered %v, which disagrees with "+
				"the locking one", c.why, got)
		}
	}
}
