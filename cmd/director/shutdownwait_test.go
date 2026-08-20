package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Shutdown must prove the process is gone before it says so.
//
// # The incident this is written from
//
// `director shutdown` sent the request, swallowed the connection error a service closing its
// socket produces, and printed "Director service stopped". That sentence is true of a service that
// exited and equally true of one whose socket went away while the process kept running — and the
// second is invisible. The operator believes Marco is gone, starts another, and two Directors now
// hold global low-level input hooks. It happened twice in one session and doubled the desktop's
// input latency for half an hour before anybody looked at a process list.
//
// A silent-but-alive service is the worst outcome available here, so it is the one that must be
// reported loudest.

// A process that is demonstrably alive is never reported as gone.
//
// Its own. Nothing in this test can be flaky about whether the process exists, because the test IS
// the process.
func TestShutdownDoesNotClaimSuccessWhileTheProcessLives(t *testing.T) {
	gone, why := waitForExit(os.Getpid(), 250*time.Millisecond)
	if gone {
		t.Fatal("reported a live process as exited. Every `director shutdown` would then " +
			"claim success while the service kept its hooks installed, which is precisely " +
			"how two Directors end up running")
	}
	if !strings.Contains(why, "still running") {
		t.Errorf("the reason reads %q; it must say the process is still there so somebody "+
			"can act on it", why)
	}
}

// A process that really has exited is reported as gone, so the wait is not simply always false.
//
// The control. Without it, `return false, "..."` passes the test above.
func TestShutdownConfirmsAProcessThatHasActuallyExited(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "exit")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot spawn a short-lived process here: %v", err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Wait()

	gone, why := waitForExit(pid, 3*time.Second)
	if !gone {
		t.Fatalf("a process that has exited was not confirmed gone: %s", why)
	}
}

// Not knowing which process to watch is its own answer, not success.
//
// A service too broken to say what it is cannot have its exit confirmed, and guessing by
// executable name would sweep up the Director the operator is deliberately running.
func TestShutdownWithNoProcessIdentityRefusesToConfirm(t *testing.T) {
	gone, why := waitForExit(0, 50*time.Millisecond)
	if gone {
		t.Fatal("claimed a process exited without knowing which process it was")
	}
	if why == "" {
		t.Error("no reason given")
	}
}
