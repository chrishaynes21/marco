//go:build windows

package gate

// AND NOTHING IS ENDED BY PID ALONE.
//
// The last gap in this, and the one no table can hold: the pid was read out of a snapshot, and a
// snapshot is a photograph. Between the photograph and the kill the process can exit and Windows
// can hand its number to something else — during a gate that spawns hundreds of processes, that
// is not a remote possibility. A kill by pid would then end whatever now holds the number, and
// the report would say it had ended the survivor it wrote down.
//
// There is no way to make Windows reuse a particular pid on demand, so the condition is produced
// from the other side, which is the same condition from the point of view of the code: a pid
// whose process is NOT the instance that was written down. That is exactly what End has to
// notice, and it has to notice it on the handle it is about to kill.

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestTheProcessEndedIsTheOneThatWasWrittenDown(t *testing.T) {
	// A REAL PROCESS, because the whole question is what the kernel says about a handle.
	// A sleeping shell, because what is under test is the ending and not Marco.
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Start-Sleep -Seconds 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting something to end: %v", err)
	}
	pid := uint32(cmd.Process.Pid)
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	since, err := StartedAt(pid)
	if err != nil {
		t.Fatalf("a process this run just started could not be identified: %v", err)
	}
	if !running(t, pid) {
		t.Fatalf("pid %d was not running at the start, so nothing below this is a test",
			pid)
	}

	// ── THE TARGET IS NOT THE ONE THAT WAS WRITTEN DOWN ──
	stale := Identity{PID: pid, Since: since.Add(-time.Minute)}
	err = End(stale)
	if err == nil {
		t.Fatal("a pid whose process is not the instance written down was ended, " +
			"which is how a gate kills whatever happens to hold a number")
	}
	for _, says := range []string{"reused", "NOTHING HAS BEEN DONE"} {
		if !strings.Contains(err.Error(), says) {
			t.Errorf("the refusal does not say %q: %v", says, err)
		}
	}
	if !running(t, pid) {
		t.Fatalf("pid %d was ended although it is not the instance that was written "+
			"down", pid)
	}

	// ── AND A PID WITH NO IDENTITY AT ALL IS NOT ENDED EITHER ──
	//
	// Which is the bare pid the old End took, arriving as the only thing it now refuses.
	if err := End(Identity{PID: pid}); err == nil {
		t.Error("a bare pid with no start time was ended")
	} else if !strings.Contains(err.Error(), "not ended") {
		t.Errorf("the refusal does not say what it did not do: %v", err)
	}
	if !running(t, pid) {
		t.Fatalf("pid %d was ended on a bare pid", pid)
	}

	// ── AND THE ONE THAT WAS WRITTEN DOWN IS ENDED ──
	//
	// Without which every refusal above is satisfied by an End that ends nothing at all.
	if err := End(Identity{PID: pid, Since: since}); err != nil {
		t.Fatalf("the instance this run established could not be ended: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for running(t, pid) {
		if time.Now().After(deadline) {
			t.Fatalf("pid %d is still running after it was ended", pid)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// running reports whether a pid is in the process list.
//
// ASKED OF THE SNAPSHOT, not of a handle: this test holds a handle to the child through
// `exec.Cmd`, and a process whose handle is still open can be opened and asked when it started
// long after it has gone. The process list is what "running" means.
func running(t *testing.T, pid uint32) bool {
	t.Helper()
	now, err := Snapshot()
	if err != nil {
		t.Fatalf("asking what is running: %v", err)
	}
	for _, p := range now {
		if p.PID == pid {
			return true
		}
	}
	return false
}
