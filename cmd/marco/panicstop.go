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
)

// withPanicStop runs fn with a kill-switch when realInput is true: a global
// press of the stop key (see stopKeySpec) cancels the context, aborting the
// route mid-run (in-flight host calls stop and `finally` blocks run). It returns
// to the caller rather than killing the process, so an `assistant` session
// survives an abort.
//
// realInput should be false for the dryrun host (no need to grab global hooks).
// If global hooks aren't available (non-Windows, or install fails), it runs
// without a panic-stop.
func withPanicStop(realInput bool, fn func(context.Context) error) error {
	// A front-end that already owns global hotkeys (e.g. the AHK overlay) sets
	// MARCO_NO_PANIC_STOP so this child doesn't install its own competing
	// WH_KEYBOARD_LL/WH_MOUSE_LL hooks — dueling low-level hooks plus the route's
	// own injected input is a deadlock/zombie risk. The front-end owns aborting.
	if !realInput || os.Getenv("MARCO_NO_PANIC_STOP") != "" {
		return fn(context.Background())
	}
	rec := recorder.New()
	if err := rec.Start(); err != nil {
		return fn(context.Background())
	}
	stop := recorder.ParseStopKey(stopKeySpec())
	ctx, cancel := context.WithCancel(context.Background())
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

// appOf returns the current foreground app for a Deps (used for scoped route
// resolution), or "" when unavailable.
func appOf(d orchestrator.Deps) string {
	if d.App == nil {
		return ""
	}
	return d.App()
}

// dispatchDo resolves the command to a route in the current app context and runs it
// under the stop-key panic-stop (see stopKeySpec), filling any args (named
// "name:value" or positional "… with a, b") into the route's {{name}}/{{N}}
// placeholders; an unknown command is taught (teaching drives the recorder itself,
// so it isn't wrapped).
func dispatchDo(d orchestrator.Deps, name string, named map[string]string, positional []string) error {
	app := appOf(d)
	mlog.Debug("dispatch: resolving", "name", name, "app", app, "named_count", len(named), "positional_count", len(positional))
	if rt, ok := d.Reg.Resolve(app, name); ok {
		mlog.Info("dispatch: route found", "input", name, "route", rt.Slug, "scope", rt.App)
		// THE door. Resolving which play the user means is not permission to perform it. Every
		// invocation passes through the authority seam here, exactly as orchestrator.Deps.Do
		// does on the teach-miss path — authored and taught plays run as they always have, and a
		// learned play Marco composed by watching is asked about once before it runs. See
		// [[ADR-029-resolution-is-not-permission]].
		resolved := orchestrator.Classify(d.Reg, rt, name)
		decision := orchestrator.Authorize(resolved, d.Authority)
		mlog.Info("dispatch: authority", "route", rt.Slug, "verdict", string(decision.Verdict), "reason", decision.Reason)
		if !decision.Allow() {
			if decision.Sentence != "" {
				fmt.Fprintln(os.Stdout, decision.Sentence)
			}
			// Not an error: "you said no" and "something went wrong" are different, and only
			// one deserves a non-zero exit.
			return nil
		}
		// Announce the canonical route that's about to run, so a front-end (the
		// overlay) can show what a loose phrase actually resolved to.
		fmt.Printf("[route] %s\n", prettyRoute(rt.Slug))
		// THE FORK. A play MARCO wrote by watching is performed by the Director; everything
		// else runs here exactly as it always has.
		//
		// The split is on provenance, not on a flag: `Learned()` is true only for a play
		// whose origin sidecar still describes the file beside it. An authored play, a
		// taught play, and a learned play somebody has since EDITED are all ordinary plays
		// and take the local path untouched — an edited one deliberately so, because the
		// file is the user's writing now and the Director has no record of what it says.
		//
		// It sits AFTER the door on purpose. Delegation is performance, and
		// [[ADR-029-resolution-is-not-permission]] puts the authority seam in front of
		// performance wherever it happens.
		//
		// # Why not simply run it here
		//
		// Because `marco` cannot see. A learned play's first line asks the Screen whether the
		// place it begins on is showing; this process has no perception and answers "I could
		// not check", so the play refuses at line one — and the generated play CATCHES that
		// refusal and logs it, so the process still exits 0 and a front-end reads success.
		// That is the failure this fork exists to end.
		//
		// Deleting this branch must fail TestALearnedPlayIsPerformedByTheDirector.
		if resolved.Learned() {
			return performLearned(d, resolved, named, positional)
		}
		return withPanicStop(true, func(ctx context.Context) error {
			return runRoute(ctx, d, rt, named, positional)
		})
	}
	mlog.Info("dispatch: no route found", "name", name, "app", app)
	// MARCO_NO_TEACH (set by the overlay): an unknown command errors instead of
	// dropping into the interactive teach flow, which would block on stdin.
	if os.Getenv("MARCO_NO_TEACH") != "" {
		return fmt.Errorf("no route matches %q (teach it: marco teach %q, or `m teach … in the overlay)", name, name)
	}
	return d.Do(name)
}

// runRoute runs a saved route, substituting named/positional args into its
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
	return driver.RunSourceWithHostsCtx(ctx, filled, filepath.Dir(path), path, os.Stdout, d.Hosts)
}
