//go:build windows

package gate

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

// An exit is waited for on the instance that was identified, and a pid that names another
// instance is refused rather than waited on.
func TestAnExitIsWaitedForOnTheProcessThatWasIdentified(t *testing.T) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Start-Sleep -Seconds 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting something to wait for: %v", err)
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

	// ANOTHER INSTANCE under the same number: what a reused pid looks like from here.
	if _, err := Exited(Identity{PID: pid, Since: since.Add(-time.Second)}); err == nil ||
		!strings.Contains(err.Error(), "no longer") {
		t.Fatalf("a pid naming another instance was waited on: err=%v", err)
	}
	if _, err := Exited(Identity{PID: pid}); err == nil {
		t.Fatal("a pid with no start time was waited on")
	}

	gone, err := Exited(Identity{PID: pid, Since: since})
	if err != nil {
		t.Fatalf("the identified process could not be waited for: %v", err)
	}
	select {
	case <-gone:
		t.Fatal("reported exited while it was still running")
	case <-time.After(300 * time.Millisecond):
	}
	_ = cmd.Process.Kill()
	select {
	case <-gone:
	case <-time.After(10 * time.Second):
		t.Fatal("its exit was never reported")
	}
}
