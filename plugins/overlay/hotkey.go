package main

// A PRESSED SHORTCUT IS TRACKED LIKE ANY OTHER RUN (S4, finding F9).
//
// It used to go through runMarco, which streams and returns, so a hotkey left no status line and
// no history row: a key pressed over a game either did something or did not, and the HUD could
// not say which. Now it ends like every other run: the engine's `[result] ` word, the routine it
// announced, the time, in the status line and in the history.
//
// What it does NOT change: the press is `marco hotkey <key>`, so which routine runs (the
// application's own key first, then the global one), the recording hold-off and every refusal are
// still the engine's. It is still registered for cancellation, so the leader while it runs stops
// it exactly as before.

import (
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/outcome"
)

// pressShortcut runs `marco hotkey <key>` and records how it ended.
//
// Going back to runMarco must fail TestAPressedShortcutIsInTheHistory.
func pressShortcut(h *model, key string) childRun {
	shown := keyShown(key)
	h.setStatus("running: " + shown)
	args := []string{"hotkey", key}
	if app, ok := takeCarried(key); ok {
		args = append(args, armedFlag+app)
	}
	start := time.Now()
	r := streamChild(h, true, args...)
	r.dur = time.Since(start)
	word, row := hotkeyRow(shown, r)
	if word == "" {
		h.setStatus(row)
	} else {
		h.setStatus(statusLine(outcome.Outcome(word), row))
	}
	h.addHistory(row, word, r.dur)
	kv := []any{"key", key, "row", row, "outcome", word, "took", fmtDur(r.dur)}
	if r.err != nil {
		kv = append(kv, "err", r.err)
	}
	mlogI("overlay: shortcut finished", kv...)
	return r
}

// hotkeyRow is the history row for a press, and its outcome word ("" when nothing was invoked).
//
// A press that exited cleanly and announced nothing is a key with nothing on it: every press that
// reaches a routine announces a `[result] `, and every refusal does too. The engine says which
// key and where; an older engine says nothing, and the row then says it in this surface's words.
func hotkeyRow(shown string, r childRun) (word, row string) {
	if r.result == "" && r.err == nil && !r.killed {
		if r.said != "" {
			return "", r.said
		}
		return "", "nothing on " + shown
	}
	o := r.outcome()
	row = shown
	if r.route != "" {
		row += "  " + r.route
	}
	if o != outcome.Performed && r.said != "" {
		row += " — " + r.said
	}
	return string(o), row
}

// ── THE KEY CARRIES THE APPLICATION ITS LEADER WAS PRESSED IN ──
//
// The key does not come straight here: the action loop emits a Hotkeys event, overlay.marco's
// `when Hotkeys reads Key?` hands it to `Overlay Hotkey`, and that is pressShortcut. The event's
// data is the key alone, because a person's own overlay.marco may bind that feed and compare it
// with a letter. So the context goes beside it: the loop carries the session's application for
// that key (carryPress), and the press takes it (takeCarried), oldest first, so two quick presses
// of one key in two sessions each keep their own. A carried press the script never turned into a
// press expires rather than lending its context to a later one.

// carryFor is how long a carried context waits for its press. The hop through overlay.marco is
// milliseconds; this only bounds a press that never came.
const carryFor = 5 * time.Second

type carriedPress struct {
	key, app string
	at       time.Time
}

var carried struct {
	sync.Mutex
	q []carriedPress
}

// carryPress records that key was pressed after a leader pressed in app.
func carryPress(key, app string) {
	carried.Lock()
	defer carried.Unlock()
	carried.q = append(carried.q, carriedPress{key: key, app: app, at: time.Now()})
}

// takeCarried is the application the oldest waiting press of key was armed in. ok is false for a
// press no leader carried (a script calling `Overlay Hotkey` itself), which the engine runs in
// whatever is in front.
func takeCarried(key string) (app string, ok bool) {
	carried.Lock()
	defer carried.Unlock()
	now, kept := time.Now(), carried.q[:0]
	for _, c := range carried.q {
		switch {
		case now.Sub(c.at) > carryFor:
		case !ok && c.key == key:
			app, ok = c.app, true
		default:
			kept = append(kept, c)
		}
	}
	carried.q = kept
	return app, ok
}
