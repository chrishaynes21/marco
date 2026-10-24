package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// runRoute runs a saved play, substituting named/positional args into its
// {{name}}/{{N}} placeholders. With no placeholders/args it's the same as running
// the file.
func runRoute(ctx context.Context, d orchestrator.Deps, rt routes.Route, named map[string]string, positional []string) error {
	path := d.Reg.Path(rt)
	mlog.Debug("run: reading route", "route", rt.Slug, "path", path)
	src, err := os.ReadFile(path)
	if err != nil {
		mlog.Error("run: read failed", "path", path, "err", err)
		return fmt.Errorf("read %s: %w", path, err)
	}
	// Expose the named args to the OS host so a secret-typed arg (password/pin/…) can
	// be provided inline and remembered (see oshost.doSecret).
	if h, ok := d.Hosts["*"].(*oshost.Host); ok {
		h.SetArgs(named)
	}
	filled := routes.ApplyArgs(string(src), named, positional)
	mlog.Info("run: executing", "route", rt.Slug, "scope", rt.App, "named_args", len(named), "positional_args", len(positional))
	if err := driver.RunSourceWithHostsCtx(ctx, filled, filepath.Dir(path), path, os.Stdout, d.Hosts); err != nil {
		return err
	}
	// ── AND A ROUTINE THAT WAS REFUSED IS NOT A ROUTINE THAT RAN ──
	//
	// A press that checks the window it was recorded in can refuse, and a guarded route
	// stops there: nothing after it is done. The program itself ends tidily — a root
	// handler is compulsory in Marco, so the failure is caught and logged — which means
	// without this, the command that performed it would exit successfully about a
	// routine that pressed nothing. The host keeps its own refusals so the answer is
	// data rather than a line of output somebody has to read.
	//
	// Only a GUARDED press records one, so a legacy route reports exactly what it
	// always did. See oshost.Host.Refused.
	// WHICH CONTRACT THIS ROUTE WAS SAVED UNDER, read from the route itself. A point
	// that states the window it was recorded in IS the guarded contract — it is what
	// makes the three saved contracts distinguishable at all (see ADR-251) — and a
	// route carrying none reports exactly what it always did.
	//
	// Read from the source rather than from a field, because there is no field: the
	// contract lives in the saved Marco, which is the only executable truth here. If
	// that ever needs to be asked a second way, the answer belongs on `routes.Route`
	// and not in a second copy of this line.
	if !strings.Contains(filled, `RelTo "`) {
		return nil
	}
	if h, ok := d.Hosts["*"].(*oshost.Host); ok {
		if refused := h.Refused(); len(refused) > 0 {
			mlog.Error("run: refused", "route", rt.Slug, "why", refused[0])
			return fmt.Errorf("%s stopped: %s", rt.Slug, refused[0])
		}
	}
	return nil
}
