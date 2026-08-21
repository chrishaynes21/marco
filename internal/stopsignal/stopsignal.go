// Package stopsignal is how one "stop" reaches a Play running in another process.
//
// # The problem it solves
//
// Marco drives the desktop from more than one process. The Director performs a learned Play
// inside its own service; `marco do` performs an authored or recorded Play inside a short-lived
// child that the overlay, the control centre, a hotkey or a terminal may each have started. The
// Audience has ONE word for making all of that stop, and before this package that word reached
// only the Director: `runControl` had a single arm. A Play typing into a text box could not be
// stopped by saying "stop", because nothing in the engine could name a sibling process.
//
// # Why a file, and not something cleverer
//
// Three constraints, and together they leave one honest answer.
//
//   - **The engine has no external dependencies and no service dependency.** `marco do` must work
//     with the Director switched off — that is the offline promise. So the rendezvous cannot be
//     the Director, a socket it owns, or anything that needs a daemon alive to broker it.
//   - **The stopper often cannot see the stopped.** A person typing `marco stop` in a terminal has
//     no handle on a child the overlay spawned, and the overlay has no handle on a Play started
//     from a terminal. Process handles are the wrong currency; the store they share is the right
//     one.
//   - **It must not be a kill.** Killing is what the overlay used to do, and `TerminateProcess`
//     runs no deferred function: held keys stayed held, the cursor stayed where the Play left it.
//     Stopping has to arrive as a CANCELLATION so the runtime unwinds through `finally` and the
//     host releases what it is holding.
//
// So: a counter in a file under the same `$MARCO_HOME` every part of Marco already shares. Raising
// it is one small write. Noticing is a poll of one small file. Both are far below the cost of the
// input they are interrupting, and both are off any hook thread.
//
// # The generation, not a flag
//
// The file holds a MONOTONIC NUMBER, not "stop yes/no". A flag would need clearing, and whoever
// cleared it would race the next Play into starting: a Play begun a millisecond after a stop would
// either die instantly on a flag nobody had reset yet, or survive a stop that was meant for it.
//
// A generation has no such moment. A run reads the number when it starts and stops when it sees a
// LARGER one. A stale file left by last week's stop reads as "already accounted for" and nothing
// has to tidy it up.
//
// # What it deliberately does not do
//
// It does not report who stopped, or how many are listening, or whether anybody heard. A Play that
// has already finished is not an error to have asked to stop. Stop is a broadcast, and broadcasts
// do not get receipts.
package stopsignal

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FileName is the generation file, beside the action graph and the semantic memory.
//
// Named for what it is rather than for the mechanism, because a person who finds it while looking
// through their own store should be able to tell that deleting it costs them nothing.
const FileName = "stop-generation"

// PollInterval is how often a running Play looks.
//
// 100ms is chosen against the thing being interrupted, not against a benchmark: it is far below
// the moment at which a person who pressed stop starts wondering whether it worked, and far above
// any cost worth counting for a stat of a file of at most twenty bytes. It is deliberately NOT on
// the low-level hook thread — see the hook invariants in CLAUDE.md.
const PollInterval = 100 * time.Millisecond

// Home is the store root: `$MARCO_HOME`, else the OS config directory, else the working directory.
//
// The same rule `cmd/marco`'s directorDir uses, and it has to stay the same rule: a stop raised in
// one directory and watched for in another is a stop that silently does nothing.
func Home() string {
	if dir := os.Getenv("MARCO_HOME"); dir != "" {
		return dir
	}
	if home, err := os.UserConfigDir(); err == nil {
		return filepath.Join(home, "marco")
	}
	return "."
}

// Path is the generation file inside home.
func Path(home string) string { return filepath.Join(home, FileName) }

// Generation reads the current generation. An absent, empty or unreadable file is generation 0.
//
// Unreadable counts as zero on purpose. This is consulted on the path that STARTS a Play, and a
// Play that refused to start because a scratch file was briefly locked would be a worse product
// than one that starts and can still be stopped by the next raise.
func Generation(home string) int64 {
	b, err := os.ReadFile(Path(home))
	if err != nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// Raise asks every Play running against this store to stop.
//
// It writes strictly more than it read, so two stops racing each other still leave a number larger
// than any watcher started with — which is the only property a watcher depends on. It does not
// need to be exactly one more, and deliberately is not: using the wall clock when the clock is
// ahead of the counter keeps the file meaningful to a human reading it.
func Raise(home string) error {
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	next := Generation(home) + 1
	if now := time.Now().UnixMilli(); now > next {
		next = now
	}
	// Write through a temporary file so a watcher never reads a half-written number. A truncating
	// write is not atomic, and the failure it produces — a parse error read as generation 0 — is
	// exactly the one that would make a stop go unheard.
	tmp, err := os.CreateTemp(home, FileName+".*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.WriteString(strconv.FormatInt(next, 10)); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, Path(home)); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// Watch returns a context that is cancelled when the generation rises above the value it reads
// now, and a function that releases the watcher.
//
// The returned context also inherits parent, so a caller keeps whatever cancellation it already
// had. The caller MUST call the release function; it stops the polling goroutine.
//
// # Read the generation before doing anything else
//
// The baseline is taken synchronously, before Watch returns — not inside the goroutine. If it were
// taken inside, a stop raised in the window between starting a Play and the goroutine's first read
// would be adopted as the baseline and the Play would never hear it. That window is small and it
// is exactly the window a person creates when they change their mind immediately.
//
// Moving it into the goroutine must fail TestAStopRaisedWhileWatchWasStartingIsStillHeard, which
// raises a stop from inside Watch itself rather than racing one against it from outside.
func Watch(parent context.Context, home string) (context.Context, func()) {
	return watch(parent, home, PollInterval)
}

// watch is Watch with the interval exposed, so a test does not have to wait in real time.
func watch(parent context.Context, home string, poll time.Duration) (context.Context, func()) {
	if parent == nil {
		parent = context.Background()
	}
	base := Generation(home)
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(poll)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-t.C:
				if Generation(home) > base {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, func() {
		close(done)
		cancel()
	}
}
