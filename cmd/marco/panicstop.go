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
// play mid-run (in-flight host calls stop and `finally` blocks run). It returns
// to the caller rather than killing the process, so an `assistant` session
// survives an abort.
//
// realInput should be false for the dryrun host (no need to grab global hooks).
// If global hooks aren't available (non-Windows, or install fails), it runs
// without a panic-stop.
func withPanicStop(realInput bool, fn func(context.Context) error) error {
	// A front-end that already owns global hotkeys (e.g. the AHK overlay) sets
	// MARCO_NO_PANIC_STOP so this child doesn't install its own competing
	// WH_KEYBOARD_LL/WH_MOUSE_LL hooks — dueling low-level hooks plus the play's
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
	return driver.RunSourceWithHostsCtx(ctx, filled, filepath.Dir(path), path, os.Stdout, d.Hosts)
}
