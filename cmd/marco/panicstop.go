package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
)

// withPanicStop runs fn under everything that is allowed to abort it.
//
// # It used to ask one question and answer two
//
// There was a single `if` here, and `MARCO_NO_PANIC_STOP` decided both halves of it: whether this
// process installs global keyboard hooks, and whether the run it wraps can be cancelled at all.
//
// The variable exists for the FIRST half only. A front end that already owns a low-level keyboard
// hook — the overlay does — must not have a second, competing WH_KEYBOARD_LL installed in the
// child it spawns; duelling low-level hooks, plus the play's own injected input, is a
// deadlock/zombie risk, and the hook invariants in CLAUDE.md are about exactly that thread. The
// front end owns the abort GESTURE. That is all it was ever saying.
//
// It was never meant to say "and this play may not be stopped". But that is what it said: the
// early return handed `fn` a `context.Background()`, and nothing can cancel one. So every play the
// overlay spawned — which is most plays a person runs — was uncancellable from the instant it
// started, and the only lever the overlay had left was to KILL the child. `TerminateProcess` runs
// no deferred function, so `finally` never ran: a held key stayed held and the cursor stayed
// wherever the play had dragged it.
//
// # So the two decisions are now two decisions
//
// Hook ownership is still `MARCO_NO_PANIC_STOP`'s, and the stop-key behaviour when hooks ARE
// installed is unchanged, down to the message it prints. Cancellability belongs to nobody's
// environment variable: EVERY run — hooks or not, real input or not, dryrun or SendInput — is
// watched by [[internal/stopsignal]], so one "stop" from any entrance and any process reaches it
// as a CANCELLATION and the runtime unwinds through `finally` on the way out.
//
// A dryrun run is watched too, deliberately. It costs one stat of a twenty-byte file every 100ms,
// and it means "stop" behaves the same way while somebody is trying a play out as it will when
// they run it for real — which is the only way a person ever comes to trust the word.
//
// Deleting the stopsignal.Watch must fail TestASpawnedPlayIsCancellableWithoutHooks.
func withPanicStop(realInput bool, fn func(context.Context) error) error {
	// THE ONE STOP, arm (a), RECEIVED here. Taken before anything else, because Watch reads its
	// baseline generation synchronously: a stop raised a millisecond after this process started
	// is a stop somebody meant for this play, and it must not be adopted as the baseline.
	ctx, release := stopsignal.Watch(context.Background(), stopsignal.Home())
	defer release()

	if !realInput || os.Getenv("MARCO_NO_PANIC_STOP") != "" {
		return fn(ctx)
	}
	rec := recorder.New()
	if err := rec.Start(); err != nil {
		return fn(ctx)
	}
	stop := recorder.ParseStopKey(stopKeySpec())
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		for ev := range rec.Events() {
			if stop.Triggered(ev) {
				fmt.Fprintf(os.Stderr, "\n[aborted — %s]\n", stop.Label())
				cancel()
				return
			}
		}
	}()
	err := fn(ctx)
	cancel()
	rec.Stop()
	return err
}

// appOf returns the current foreground app for a Deps (used for scoped play
// resolution), or "" when unavailable.
func appOf(d orchestrator.Deps) string {
	if d.App == nil {
		return ""
	}
	return d.App()
}

// ran is what a performance actually executed, for its record to cite.
//
// Returned rather than worked out a second time by the caller. The digest and the contract both
// come off the bytes this function read, in the one place that reads them: deriving them again
// around the call would be a second answer about one file, and the day the two disagreed the
// record would cite source that did not run.
type ran struct {
	// Digest is `routes.DigestOf` over the SAVED source — the file, not the text with
	// arguments substituted into it. Somebody holding this record and the library can look
	// at exactly what ran; the arguments are a separate fact and are not in it, which also
	// keeps anything a person typed out of a durable file.
	Digest string
	// Contract is which contract the saved source STATES, in the three words
	// [[ADR-251-a-press-checks-the-window-it-was-recorded-in]] gives them: `legacy` when no
	// press states a window, `guarded` when one does and carries every term the check
	// needs, and `incomplete` when it states a window and lacks a term — which refuses
	// rather than presses, naming what it lacks.
	//
	// Read by `orchestrator.ContractOf`, which is the ONE reader of this and whose list of
	// terms is held against the guard's own requirement by test. The first version of this
	// change asked the source itself — `strings.Contains(src, "RelTo \"")` — which is a
	// second answer to the question, and a weaker one: it cannot see `incomplete` at all,
	// so a record would have called an incomplete route guarded.
	Contract string
}

// contractUnstated is what the record says when the contract could not be read at all.
//
// The source itself always states one of the three. This is for a route that could not be opened,
// where saying `legacy` would be a reading of something nobody read.
const contractUnstated = "unstated"

// snapshot is one read of a saved routine: the bytes, what they become, and what they are.
//
// # Why it is taken before the reference is claimed
//
// Because the record left in that reference has to say WHAT was running, and the moment that
// matters most is the moment the run never reaches its end — a killed child, a machine that went
// down, a stop that did not unwind. The identity used to be worked out from what the performance
// returned and written only at the end, so the one attempt a person has to ask somebody about was
// the one whose record said nothing but that something started.
//
// # And it is the same bytes that execute
//
// One read, not two. Reading the file to describe it and reading it again to perform it is two
// answers about one file, and a save can land between them — after which the record cites the
// digest of source that did not run. That is exactly the confusion `describeRoutine` prints a
// warning about, except arrived at by Marco's own hand rather than by somebody re-recording.
//
// A snapshot that could not be read carries its reason and runs nothing. It is still an attempt
// and still gets a reference, because "your routine is not where the library says it is" is
// something a person needs to be able to cite.
type snapshot struct {
	rt    routes.Route
	path  string
	named map[string]string
	// filled is the source with arguments substituted — what actually executes. The
	// arguments themselves are never in `did`, and so never in the record: somebody's
	// password arriving as an argument is not a thing to write into a durable file.
	filled string
	// did is the identity of what is about to run, and the identity the finished record
	// carries. One value, written twice, so the provisional record and the final one
	// cannot describe two different programs.
	did ran
	// contract is the same reading `did.Contract` holds, kept in its own type because the
	// rule at the end of the performance acts on it. The record wants a word and the rule
	// wants the thing; one read answers both.
	contract orchestrator.Contract
	// err is why there is nothing to perform.
	err error
}

// take reads the routine once and says what is about to run.
func take(d orchestrator.Deps, rt routes.Route, named map[string]string, positional []string) snapshot {
	s := snapshot{rt: rt, path: d.Reg.Path(rt), named: named}
	mlog.Debug("run: reading route", "route", rt.Slug, "path", s.path)
	src, err := os.ReadFile(s.path)
	if err != nil {
		mlog.Error("run: read failed", "path", s.path, "err", err)
		s.did = ran{Contract: contractUnstated}
		s.err = fmt.Errorf("read %s: %w", s.path, err)
		return s
	}
	s.filled = routes.ApplyArgs(string(src), named, positional)
	// WHAT IS ABOUT TO RUN, DESCRIBED ONCE. See `ran`.
	s.contract, _ = orchestrator.ContractOf(s.filled)
	s.did = ran{Digest: routes.DigestOf(string(src)), Contract: string(s.contract)}
	return s
}

// runRoute performs a snapshot: the source already read, with its args already substituted into
// its {{name}}/{{N}} placeholders.
func runRoute(ctx context.Context, d orchestrator.Deps, s snapshot) error {
	if s.err != nil {
		return s.err
	}
	// Expose the named args to the OS host so a secret-typed arg (password/pin/…) can
	// be provided inline and remembered (see oshost.doSecret).
	if h, ok := d.Hosts["*"].(*oshost.Host); ok {
		h.SetArgs(s.named)
	}
	rt, path := s.rt, s.path
	mlog.Info("run: executing", "route", rt.Slug, "scope", rt.App, "named_args", len(s.named))
	if err := driver.RunSourceWithHostsCtx(ctx, s.filled, filepath.Dir(path), path, os.Stdout, d.Hosts); err != nil {
		return err
	}
	// ── AND A ROUTINE THAT DID NOT HAPPEN IS NOT A ROUTINE THAT RAN ──
	//
	// A guarded route stops at the first thing that did not happen: a press it could not
	// check, a press the machine would not deliver, or an application that never came
	// forward. The program itself ends tidily either way — a root handler is compulsory
	// in Marco, so the failure is caught and logged — which means without this, the
	// command would exit successfully about a routine that pressed nothing.
	//
	// The host keeps the account so the answer is data rather than a line of output
	// somebody has to read. See oshost.Host.Unperformed.
	//
	// WHICH CONTRACT THIS ROUTE WAS SAVED UNDER decides whether that account stops the
	// run, and it is read ONCE, by `contractOf` above. A point that states the window it
	// was recorded in IS the guarded contract — it is what makes the three saved contracts
	// distinguishable at all (see ADR-251) — and a route carrying none reports exactly what
	// it always did.
	//
	// Read from the source rather than from a field, because there is no field: the
	// contract lives in the saved Marco, which is the only executable truth here. The
	// record cites the same answer this rule acts on, which is why `ran` carries it rather
	// than the caller asking the file a second time.
	//
	// LEGACY IS THE ONE THAT LEAVES, and the test it is written against matters: `!= Guarded`
	// is the plausible-looking form and it is wrong, because an INCOMPLETE route states a
	// window and refuses the press naming what it lacks. Skipping the account for it would
	// let exactly that refusal pass as a successful run — the case ADR-251 made a third
	// contract for. Held by TestAnIncompleteRoutinesRefusalStillFailsTheCommand.
	if s.contract == orchestrator.Legacy {
		return nil
	}
	if h, ok := d.Hosts["*"].(unperformer); ok {
		if undone := h.Unperformed(); len(undone) > 0 {
			mlog.Error("run: stopped", "route", rt.Slug, "why", undone[0])
			return fmt.Errorf("%s stopped: %s", rt.Slug, undone[0])
		}
	}
	return nil
}

// unperformer is a host that can say what it did not do.
//
// An interface rather than `*oshost.Host` so the rule above can be exercised with a host that
// reports a DELIVERY failure — a guard that passed and a machine that then rejected the input.
// The concrete assertion could only be satisfied by the real host, whose backend sends real
// input, so the only command-level case that could be written was an absent application: the
// cheapest failure and the one furthest from the press.
type unperformer interface {
	// Unperformed is everything the run did not do, in order, each with the sentence
	// that said why. See oshost.Host.Unperformed.
	Unperformed() []string
}
