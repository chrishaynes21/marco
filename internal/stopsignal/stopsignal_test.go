package stopsignal

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
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
