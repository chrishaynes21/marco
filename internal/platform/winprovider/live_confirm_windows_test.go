//go:build windows

package winprovider_test

import (
	"context"
	"os"
	"os/exec"
	"sort"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
)

// What the target provenance guard costs, against the real operating system.
//
// The guard adds one read-only re-validation per targeted provider per cycle. That is the
// entire added cost, so it is the entire thing worth measuring — and it has to be measured
// against real syscalls, because the fake desktop the logic is tested on returns from a map
// and would report a cost of zero.
//
// The question being answered is narrow: is provenance correctness materially affecting
// responsiveness? Not "how fast is Confirm" in the abstract, but "how big is it beside the
// work it was added to".
//
//	MARCO_LIVE_WINDOW_TEST=1 go test ./internal/platform/winprovider/ -run LiveConfirmCost -v
func TestLiveConfirmCost(t *testing.T) {
	if os.Getenv("MARCO_LIVE_WINDOW_TEST") == "" {
		t.Skip("set MARCO_LIVE_WINDOW_TEST=1 to run: it launches and closes a program")
	}

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
	tracker := windowref.NewTracker(p)

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if v := tracker.Acquire(ctx, "mspaint"); v.State.OK() {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if _, held := tracker.Current(); !held {
		t.Skip("mspaint never presented a usable window")
	}

	// Warm: the first call through a syscall path pays for lazy resolution, and reporting
	// that as the steady-state cost would overstate it.
	for range 20 {
		tracker.Confirm(ctx)
	}

	const n = 400
	samples := make([]time.Duration, 0, n)
	for range n {
		start := time.Now()
		if _, ok := tracker.Confirm(ctx); !ok {
			t.Fatal("the window stopped confirming mid-measurement")
		}
		samples = append(samples, time.Since(start))
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })

	min, median := samples[0], samples[n/2]
	p95, max := samples[(n*95)/100], samples[n-1]
	var total time.Duration
	for _, s := range samples {
		total += s
	}
	t.Logf("Tracker.Confirm over %d live calls: min=%s median=%s p95=%s max=%s mean=%s",
		n, min, median, p95, max, total/time.Duration(n))

	// A budget, not a benchmark. The sampling interval floor is 200ms and a detection pass
	// alone runs 170-730ms; a re-validation costing a millisecond is free at that scale and
	// one costing tens of milliseconds is not. This fails only on a regression large enough
	// to matter, so it does not become a flaky performance test.
	if median > 5*time.Millisecond {
		t.Errorf("median Confirm is %s; the guard runs once per targeted provider per "+
			"cycle and this is no longer negligible against a 200ms floor", median)
	}
}

// Confirm must not enumerate. Acquire searches an application's windows when validation
// fails; Confirm answers a question about the reference already held, and an enumeration
// hidden inside it would scale with the number of open windows on the desktop.
func TestLiveConfirmDoesNotEnumerate(t *testing.T) {
	if os.Getenv("MARCO_LIVE_WINDOW_TEST") == "" {
		t.Skip("set MARCO_LIVE_WINDOW_TEST=1 to run: it launches and closes a program")
	}

	cmd := exec.Command("mspaint.exe")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot launch mspaint: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	counted := &countingPlatform{inner: winprovider.New()}
	tracker := windowref.NewTracker(counted)
	ctx := context.Background()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if v := tracker.Acquire(ctx, "mspaint"); v.State.OK() {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if _, held := tracker.Current(); !held {
		t.Skip("mspaint never presented a usable window")
	}

	before := counted.candidates
	for range 10 {
		tracker.Confirm(ctx)
	}
	if got := counted.candidates - before; got != 0 {
		t.Errorf("10 Confirm calls enumerated windows %d times; Confirm asks about the "+
			"reference it holds and must not search", got)
	}
}

// countingPlatform records how often the expensive enumeration is reached.
type countingPlatform struct {
	inner      windowref.Platform
	candidates int
}

func (c *countingPlatform) Live(ctx context.Context, h uintptr) (windowref.Candidate, bool) {
	return c.inner.Live(ctx, h)
}

func (c *countingPlatform) ProcessAlive(ctx context.Context, pid uint32) bool {
	return c.inner.ProcessAlive(ctx, pid)
}

func (c *countingPlatform) Candidates(ctx context.Context, app string) []windowref.Candidate {
	c.candidates++
	return c.inner.Candidates(ctx, app)
}
