//go:build windows

package winprovider_test

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
)

// The Windows implementation of windowref.Platform, against a real process.
//
// windowref's own tests use a fake desktop, which proves the LOGIC and nothing about the
// syscalls. This proves the syscalls: that a real window is found, that closing it is
// noticed, and that the tracker then refuses rather than handing back the rectangle the
// window used to occupy. That last step is the incident, reproduced against the real
// operating system rather than a model of it.
//
// It launches and closes a program, so it is opt-in:
//
//	MARCO_LIVE_WINDOW_TEST=1 go test ./internal/platform/winprovider/ -run Live -v
func TestLiveWindowLifecycle(t *testing.T) {
	if os.Getenv("MARCO_LIVE_WINDOW_TEST") == "" {
		t.Skip("set MARCO_LIVE_WINDOW_TEST=1 to run: it launches and closes a program")
	}

	// A program with a real top-level window that exits cleanly. mspaint ships with
	// Windows and does not shim through a store wrapper the way notepad now does.
	cmd := exec.Command("mspaint.exe")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot launch mspaint: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	p := winprovider.New()
	ctx := context.Background()

	// Wait for the window to exist. A process is running before it has drawn anything.
	var found windowref.Candidate
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		for _, c := range p.Candidates(ctx, "mspaint") {
			if c.OnScreen && c.Bounds.Width > 0 {
				found = c
				break
			}
		}
		if found.Handle != 0 {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if found.Handle == 0 {
		t.Skip("mspaint never presented a capturable window in this environment")
	}
	t.Logf("live window: pid=%d bounds=%v title=%q", found.ProcessID, found.Bounds, found.Title)

	if !p.ProcessAlive(ctx, found.ProcessID) {
		t.Fatal("the process owning a live window is reported as not alive")
	}
	if live, ok := p.Live(ctx, found.Handle); !ok {
		t.Fatal("a window that was just enumerated does not look up")
	} else if live.ProcessID != found.ProcessID {
		t.Fatalf("process = %d on lookup, %d on enumeration", live.ProcessID, found.ProcessID)
	}

	tr := windowref.NewTracker(p)
	tr.Propose(windowref.Ref{
		ID: found.ID, Handle: found.Handle, Application: "mspaint", Bounds: found.Bounds,
	})
	first := tr.Acquire(ctx, "mspaint")
	if !first.State.OK() {
		t.Fatalf("a live window would not validate: %v (%s)", first.State, first.Reason)
	}
	if first.Ref.Generation == 0 {
		t.Error("a validated live window has no epoch")
	}
	staleBounds := first.Ref.Bounds
	t.Logf("validated: %s", first.Ref.Describe())

	// Close it, and wait for the window to actually go.
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
	gone := time.Now().Add(15 * time.Second)
	for time.Now().Before(gone) {
		if _, ok := p.Live(ctx, found.Handle); !ok {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if _, stillThere := p.Live(ctx, found.Handle); stillThere {
		t.Fatal("the handle still looks up after the process was killed; " +
			"IsWindow is not detecting the destroyed window")
	}

	// The incident, against the real desktop.
	after := tr.Acquire(ctx, "mspaint")
	if after.State.OK() {
		t.Fatalf("a destroyed window validated, at %v — this is the stale-capture defect",
			after.Ref.Bounds)
	}
	if after.Ref.Bounds == staleBounds && !after.Ref.Zero() {
		t.Fatal("the old rectangle came back; capture would read whatever now occupies it")
	}
	if held, ok := tr.Current(); ok {
		t.Fatalf("the tracker still holds %+v after the window was destroyed", held)
	}
	t.Logf("refused, correctly: %v — %s", after.State, after.Reason)
}
