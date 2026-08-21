package stopsignal

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"
)

// TestAStopRaisedAfterAPlayStartedIsHeard is the whole point of the package: a Play running in one
// process stops because something in another process said so.
func TestAStopRaisedAfterAPlayStartedIsHeard(t *testing.T) {
	home := t.TempDir()
	ctx, release := watch(context.Background(), home, time.Millisecond)
	defer release()

	select {
	case <-ctx.Done():
		t.Fatal("a Play stopped before anybody asked it to")
	case <-time.After(20 * time.Millisecond):
	}

	if err := Raise(home); err != nil {
		t.Fatalf("Raise: %v", err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("a stop was raised and the running Play never heard it")
	}
}

// TestAStaleStopDoesNotStopTheNextPlay is why the file holds a generation and not a flag. A stop
// from an earlier run must not reach a Play that started afterwards.
func TestAStaleStopDoesNotStopTheNextPlay(t *testing.T) {
	home := t.TempDir()
	if err := Raise(home); err != nil {
		t.Fatalf("Raise: %v", err)
	}

	ctx, release := watch(context.Background(), home, time.Millisecond)
	defer release()
	select {
	case <-ctx.Done():
		t.Fatal("a Play was stopped by a stop raised before it started")
	case <-time.After(50 * time.Millisecond):
	}
}

// TestTwoStopsInARowAreBothHeard — Raise must always leave a larger number, including when it is
// called twice inside the same millisecond.
func TestTwoStopsInARowAreBothHeard(t *testing.T) {
	home := t.TempDir()
	for i := range 5 {
		before := Generation(home)
		if err := Raise(home); err != nil {
			t.Fatalf("Raise %d: %v", i, err)
		}
		if after := Generation(home); after <= before {
			t.Fatalf("raise %d did not increase the generation: %d -> %d", i, before, after)
		}
	}
}

// TestAnUnreadableGenerationDoesNotRefuseToStartAPlay — a scratch file is not a reason to refuse
// to run. Garbage reads as generation 0, and the next raise is still heard.
func TestAnUnreadableGenerationDoesNotRefuseToStartAPlay(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(Path(home), []byte("not a number"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Generation(home); got != 0 {
		t.Fatalf("garbage should read as 0, got %d", got)
	}
	ctx, release := watch(context.Background(), home, time.Millisecond)
	defer release()
	if err := Raise(home); err != nil {
		t.Fatalf("Raise: %v", err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("a stop after an unreadable generation was not heard")
	}
}

// TestReleasingTheWatcherStopsThePolling — the goroutine must not outlive the run it belongs to.
func TestReleasingTheWatcherStopsThePolling(t *testing.T) {
	home := t.TempDir()
	ctx, release := watch(context.Background(), home, time.Millisecond)
	release()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("releasing the watcher left its context live")
	}
}

// TestTheParentCancellationStillReaches — Watch must not take away the cancellation a caller
// already had.
func TestTheParentCancellationStillReaches(t *testing.T) {
	home := t.TempDir()
	parent, cancelParent := context.WithCancel(context.Background())
	ctx, release := watch(parent, home, time.Millisecond)
	defer release()
	cancelParent()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("cancelling the parent did not cancel the watched context")
	}
}

// TestHomeFollowsMarcoHome — a raise and a watch must land on the same file, or a stop silently
// does nothing. This is the one bug the package cannot detect at run time.
func TestHomeFollowsMarcoHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MARCO_HOME", dir)
	if got := Home(); got != dir {
		t.Fatalf("Home() = %q, want %q", got, dir)
	}
	if got, want := Path(Home()), filepath.Join(dir, FileName); got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

// TestARaiseIsReadableByAPersonLookingAtTheirOwnStore — the file is in a user's store, so it holds
// a plain number and nothing else.
func TestARaiseIsReadableByAPersonLookingAtTheirOwnStore(t *testing.T) {
	home := t.TempDir()
	if err := Raise(home); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(Path(home))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := strconv.ParseInt(string(b), 10, 64); err != nil {
		t.Fatalf("the generation file is not a plain number: %q", b)
	}
}

// TestRaiseLeavesNoTemporaryFilesBehind — the store is a place people look at.
func TestRaiseLeavesNoTemporaryFilesBehind(t *testing.T) {
	home := t.TempDir()
	for range 3 {
		if err := Raise(home); err != nil {
			t.Fatal(err)
		}
	}
	ents, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 || ents[0].Name() != FileName {
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Fatalf("raising a stop left %v behind; want only %q", names, FileName)
	}
}

// ── the baseline is read before Watch does anything else ─────────────────────

// raisingParent is a parent context that raises a stop the first time Watch consults it.
//
// # Why the parent, of all things
//
// The window this is about is microseconds wide: between a Play starting and its watcher's first
// look at the generation file. Racing a real `Raise` against it from the test's own goroutine
// measures the Go scheduler, not the code — the raise loses nearly every time, which is exactly
// why moving the baseline into the goroutine survived an independent mutation run with the whole
// package green.
//
// So the stop is raised from INSIDE Watch instead, at a moment Watch itself chooses.
// `context.WithCancel(parent)` consults its parent synchronously — `Done`, then `Value` — and
// that call sits after the baseline read and before the goroutine exists. Hooking it puts the
// stop in the window deterministically, with no timing assumption anywhere in the test.
type raisingParent struct {
	context.Context
	home string
	once sync.Once
	// err is whatever the raise reported, checked by the test rather than swallowed here.
	err error
	// hit records that Watch really did consult its parent; without it a Watch that stopped
	// doing so would make this test pass by never raising anything at all.
	hit bool
}

func (p *raisingParent) raise() {
	p.once.Do(func() {
		p.hit = true
		p.err = Raise(p.home)
	})
}

func (p *raisingParent) Done() <-chan struct{} {
	p.raise()
	return p.Context.Done()
}

func (p *raisingParent) Value(key any) any {
	p.raise()
	return p.Context.Value(key)
}

// A stop raised while Watch was still setting up is HEARD, not adopted as the baseline.
//
// # The defect this exists to catch, and what it feels like
//
// Move `base := Generation(home)` inside Watch's goroutine and the package stays green. The
// package comment explains at length why it must not be there: a stop raised in the window
// between starting a Play and the goroutine's first read is then read AS the baseline, the
// watcher waits for a generation larger than the one that was meant to stop it, and the Play runs
// on. That window is exactly the one a person creates when they change their mind immediately —
// they start something, see it is wrong, and say stop at once — and the failure is silent: the
// stop file is written, `marco stop` reports nothing wrong, and the Play keeps typing.
//
// Nothing held it. The existing tests all wait 20ms or more before raising, by which time the
// goroutine has long since taken its baseline either way.
func TestAStopRaisedWhileWatchWasStartingIsStillHeard(t *testing.T) {
	home := t.TempDir()
	parent := &raisingParent{Context: context.Background(), home: home}

	ctx, release := watch(parent, home, time.Millisecond)
	defer release()

	if !parent.hit {
		t.Fatal("Watch never consulted its parent context, so this test had no way to " +
			"place a stop inside it and is not measuring anything")
	}
	if parent.err != nil {
		t.Fatalf("raising the stop: %v", parent.err)
	}
	if Generation(home) == 0 {
		t.Fatal("the stop was never written, so there is nothing for the watcher to hear")
	}

	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("a stop raised while Watch was still setting up was never heard.\n" +
			"The baseline was taken after it — inside the goroutine, or otherwise not " +
			"first — so the watcher adopted the stop that was meant for this Play as " +
			"the number it waits to exceed, and the Play runs on with nothing on any " +
			"surface saying why.")
	}
}
