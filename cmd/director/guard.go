package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// foregroundGuard refuses input operations when the intended window is not the one
// in front.
//
// This exists because of a real incident. An edit was resolved against Notepad and
// executed while Discord held the foreground; the text went into a message box —
// successfully, verifiably, and into the wrong application. Nothing detected it,
// because input does not fail when it lands somewhere else. It succeeds.
//
// A previous focus operation proves nothing at the instant of execution. The user
// alt-tabs, a notification steals focus, an installer pops up. So the check is made
// per operation, immediately before lowering, and it RE-OBSERVES rather than trusting
// a remembered state.
//
// It observes through the same pipeline everything else does — collector, fusion,
// identity — so the guard is checking the world the planner reasoned over, not a
// private view of the desktop that could disagree with it.
type foregroundGuard struct {
	observe func(context.Context) (directorapi.WorldState, error)
	// activate re-establishes the intended context when it can be. Optional: without
	// it, a mismatch is simply refused.
	activate func(ctx context.Context, app string) error
}

// Confirm reports whether the intended window is still the right one to act on.
//
// An empty window means "whatever has focus" — the deliberate meaning of "type hello"
// with no named control — and there is nothing to compare it against, so it is
// allowed. That is not a hole: the operation named no target, so there is no mismatch
// to detect. Operations that DO name a window are the ones that can land wrong.
func (g foregroundGuard) Confirm(ctx context.Context, window string) (bool, string, error) {
	if strings.TrimSpace(window) == "" {
		return true, "", nil
	}
	if g.observe == nil {
		// No way to check. Refuse rather than assume: an unverifiable precondition on
		// an operation that can type into the wrong application is not a precondition.
		return false, "the foreground cannot be observed, so the intended target cannot be confirmed", nil
	}

	world, err := g.observe(ctx)
	if err != nil {
		return false, "", err
	}
	front, ok := world.FocusedWindow()
	if !ok {
		return false, "nothing holds the foreground, so the intended target cannot be confirmed", nil
	}
	if string(front.ID) == window {
		return true, "", nil
	}

	// One attempt to put it right. This is the "establish focus or activation when
	// semantically required" step, and it is followed by a RE-OBSERVATION, because
	// activating is a request rather than a result.
	if g.activate != nil {
		if app := applicationOf(world, window); app != "" {
			if err := g.activate(ctx, app); err == nil {
				if again, err := g.observe(ctx); err == nil {
					if f, ok := again.FocusedWindow(); ok && string(f.ID) == window {
						return true, "", nil
					}
				}
			}
		}
	}
	return false, fmt.Sprintf(
		"the intended window %s is not in front — %q (%s) is, so nothing was sent",
		window, front.Title, front.Application), nil
}

// applicationOf finds the application owning a window, "" if the world has no such
// window. OS's Activate names an APP, which is the vocabulary Marco actually has.
func applicationOf(w directorapi.WorldState, window string) string {
	for _, win := range w.Windows {
		if string(win.ID) == window {
			return win.Application
		}
	}
	return ""
}
