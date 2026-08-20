package main

import (
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/director/uiact"
)

// The semantic action history, for `director explain action`.
//
// Bounded, diagnostics only, and mirrors editHistory deliberately — the two answer the
// same shape of question about different vocabularies ("why did it type instead of
// setting the value?" / "why did it click instead of expanding?"), and giving them
// different shapes would be two things to learn instead of one.
//
// It records REFUSALS as well as successes. A refused action is the one most in need of
// explaining: nothing happened, and the reason lives entirely in which rungs were
// unavailable.
type actionHistory struct {
	mu      sync.Mutex
	entries []uiact.Outcome
}

// actionHistoryLimit is how many outcomes are kept.
//
// Unlike the edit history this holds no user text — a semantic outcome carries a verb, a
// mechanism and a control's label — so the bound is about memory rather than about how
// long content lingers.
const actionHistoryLimit = 50

func (h *actionHistory) record(o uiact.Outcome) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = append(h.entries, o)
	if len(h.entries) > actionHistoryLimit {
		// Drop from the front: the newest is the one anyone is asking about.
		h.entries = h.entries[len(h.entries)-actionHistoryLimit:]
	}
}

func (h *actionHistory) recent() []uiact.Outcome {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]uiact.Outcome, len(h.entries))
	copy(out, h.entries)
	return out
}

// activeApplication reports the app the front window belongs to, for choosing an
// application-specific procedure.
//
// From the LAST OBSERVED world rather than by observing afresh: a goal is expanded
// immediately before its first step runs, so the newest world is a moment old, and
// taking a snapshot here would attach a second observation cycle to every request.
//
// Empty when nothing has been observed yet, which selects the generic procedure — a
// correct answer rather than a degraded one, since the generic procedures are written
// to work anywhere.
func (r *Runtime) activeApplication() string {
	r.diagMu.RLock()
	defer r.diagMu.RUnlock()
	if r.lastWorld == nil || r.lastWorld.ActiveApp == nil {
		return ""
	}
	return r.lastWorld.ActiveApp.ID
}

// SemanticActions reports the recent semantic action outcomes.
//
// A control-plane method: it takes only this history's own lock, never the command lock,
// so `director explain action` answers while a command is still running. See
// lockrule_test.go.
func (r *Runtime) SemanticActions() []uiact.Outcome {
	if r.actions == nil {
		return nil
	}
	return r.actions.recent()
}
