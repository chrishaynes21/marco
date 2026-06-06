package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// withPanicStop runs fn with an Esc kill-switch when realInput is true: a global
// Esc press cancels the context, aborting the route mid-run (in-flight host
// calls stop and `finally` blocks run). It returns to the caller rather than
// killing the process, so an `assistant` session survives an abort.
//
// realInput should be false for the dryrun host (no need to grab global hooks).
// If global hooks aren't available (non-Windows, or install fails), it runs
// without a panic-stop.
func withPanicStop(realInput bool, fn func(context.Context) error) error {
	if !realInput {
		return fn(context.Background())
	}
	rec := recorder.New()
	if err := rec.Start(); err != nil {
		return fn(context.Background())
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for ev := range rec.Events() {
			if ev.Kind == recorder.EvKey && ev.Down && strings.EqualFold(ev.KeyName, "esc") {
				fmt.Fprintln(os.Stderr, "\n[aborted — Esc]")
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

// dispatchDo resolves the command to a route in the current app context and runs
// it under the Esc panic-stop; an unknown command is taught (teaching drives the
// recorder itself, so it isn't wrapped).
func dispatchDo(d orchestrator.Deps, name string) error {
	if rt, ok := d.Reg.Resolve(appOf(d), name); ok {
		return withPanicStop(true, func(ctx context.Context) error {
			return driver.RunFileWithHostsCtx(ctx, d.Reg.Path(rt), os.Stdout, d.Hosts)
		})
	}
	return d.Do(name)
}
