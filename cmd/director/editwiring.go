package main

import (
	"context"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
	"github.com/chaynes-simpleclouds/marco/internal/director/edit/clipboard"
	editproviders "github.com/chaynes-simpleclouds/marco/internal/director/edit/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	waitengine "github.com/chaynes-simpleclouds/marco/internal/director/wait/engine"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// worldObserver answers "what does this element look like now?" from the live world.
//
// The editor verifies by LOOKING, never by remembering what it sent, and this is the
// looking. It goes through the same observe function everything else does, so an edit
// is verified against the world the planner reasons over rather than a private view.
type worldObserver struct {
	observe func(context.Context) (directorapi.WorldState, error)
}

func (o worldObserver) Element(ctx context.Context, window directorapi.WindowID, id directorapi.ElementID) (directorapi.Element, bool, error) {
	w, err := o.observe(ctx)
	if err != nil {
		return directorapi.Element{}, false, err
	}
	if el, ok := w.Elements[id]; ok && el != nil {
		return *el, true, nil
	}
	return directorapi.Element{}, false, nil
}

// editSettler waits for an edit to show up in the world, by CONDITION.
//
// It is the wait engine, not a sleep. "Do not insert arbitrary sleeps" is the rule;
// the difference in practice is that a value API write is verified in a single poll
// while a browser field that repaints slowly is given as long as it needs, and neither
// case costs the other anything.
type editSettler struct {
	waits *waitengine.Waiter
}

// editSettleTimeout bounds how long an edit is given to appear. Generous enough for a
// remote desktop or a busy Electron app to repaint, short enough that a control that
// silently ignored the edit is reported rather than waited on forever.
const editSettleTimeout = 4 * time.Second

func (s editSettler) Settle(ctx context.Context, want func(context.Context) bool) (bool, error) {
	deadline := time.Now().Add(editSettleTimeout)
	for {
		if want(ctx) {
			return true, nil
		}
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		// The poll interval matches the wait engine's, so an edit and a wait sample the
		// world at the same cadence rather than fighting over the accessibility bridge.
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(waitengine.Defaults().PollInterval):
		}
	}
}

// editHistory keeps the recent edit outcomes for `director edit`.
//
// Bounded, and diagnostics only — nothing plans from it. It exists so "why did it
// type instead of setting the value?" has an answer that comes from a record rather
// than from a guess.
type editHistory struct {
	mu      sync.Mutex
	entries []edit.Outcome
}

// editHistoryLimit is how many outcomes are kept. Enough to cover a session's worth of
// editing without holding text — and the text IS held, so the bound is also the bound
// on how long a typed value lingers in memory.
const editHistoryLimit = 50

func (h *editHistory) record(o edit.Outcome) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = append(h.entries, o)
	if len(h.entries) > editHistoryLimit {
		// Drop from the front: the newest edits are the ones anyone is asking about.
		h.entries = h.entries[len(h.entries)-editHistoryLimit:]
	}
}

func (h *editHistory) recent() []edit.Outcome {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]edit.Outcome, len(h.entries))
	copy(out, h.entries)
	return out
}

// guardedInput is the editor's keyboard, addressed to a window.
//
// It exists to carry the target window from the editor down to the executor, where the
// foreground guard can use it. That link was missing: the editor knew the window, the
// guard was wired, and nothing joined them — so `Confirm(ctx, "")` returned true for
// every keystroke and Ctrl+C went to whatever happened to be in front.
//
// A named type rather than the executor itself, so the guarded methods are the ones the
// editor can reach and the unaddressed ones are not.
type guardedInput struct{ exec *marcoexec.Executor }

func (g guardedInput) Type(ctx context.Context, window directorapi.WindowID, text string) error {
	return g.exec.TypeIn(ctx, window, text)
}

func (g guardedInput) Key(ctx context.Context, window directorapi.WindowID, chord string,
	hold time.Duration) error {
	return g.exec.KeyIn(ctx, window, chord, hold)
}

// newEditor assembles the editor with every rung of the ladder it can reach.
//
// Each dependency is passed only if it genuinely exists. Handing the editor a stub
// that pretended to be a value provider would collapse the ladder into a single rung
// while looking like it had four.
func newEditor(values editproviders.Values, focus editproviders.Focus, input editproviders.Input,
	board directorapi.Clipboard, observe func(context.Context) (directorapi.WorldState, error),
	waits *waitengine.Waiter) *edit.Editor {

	deps := edit.Deps{
		Values:   values,
		Focus:    focus,
		Input:    input,
		Observer: worldObserver{observe: observe},
		Settle:   editSettler{waits: waits},
	}
	if board != nil {
		deps.Clipboard = clipboard.New(board)
	}
	return edit.New(deps)
}

// Edits reports the recent edit outcomes.
func (r *Runtime) Edits() []edit.Outcome {
	if r.edits == nil {
		return nil
	}
	return r.edits.recent()
}
