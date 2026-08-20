package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/platform/screenhost"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// Wiring the Screen act into the ordinary host map.
//
// # Why it is here and not in the invocation path
//
// Because a compiled play ASKS for `Screen's Showing`, and the ordinary runtime dispatches it like
// any other foreign capability. There is no `if learnedPlay { checkScreen() }` anywhere — the
// requirement lives in the `.marco`, and this file only decides who answers it.
//
// # Read-only by construction
//
// The host is handed a `screenhost.Recognition`: four reads and nothing else. It cannot press a
// key, focus a window, run a route or write to memory, whatever it decides.

// newScreenHost builds the Screen act's implementation, or nil when nothing can recognise a screen.
//
// Nil is a real answer. A Marco with no semantic memory cannot tell where it is, so a guarded play
// refuses rather than degrading into blind replay — see [[ADR-031-the-user-names-the-stage]].
func newScreenHost() *screenhost.Host {
	store, _ := semanticmemory.Open(memoryPath())
	if store == nil {
		return screenhost.New(nil)
	}
	return screenhost.New(&liveScreens{store: store})
}

// memoryPath is where durable semantic memory lives, beside the routes it describes.
func memoryPath() string {
	if p := strings.TrimSpace(os.Getenv("MARCO_MEMORY")); p != "" {
		return p
	}
	return filepath.Join(routesDir(), "memory.json")
}

// liveScreens is the read-only view the Screen host is given.
//
// Semantic memory answers "what is this place called"; the window tracker answers "what is in
// front". Recognising the CURRENT screen needs perception Director owns, and standalone Marco does
// not have it — so `CurrentSubject` reports `unavailable` and the guard refuses. That is the
// honest architecture rather than a contorted one: Director figures out the play, Marco performs
// it, and Director still provides the eyes while it does.
type liveScreens struct{ store *semanticmemory.Store }

func (l *liveScreens) Application() string { return winctx.Active() }

func (l *liveScreens) SubjectNamed(application, name string) (string, bool) {
	r, ok := l.store.SubjectNamed(application, name)
	if !ok {
		return "", false
	}
	return r.ID, true
}

// CurrentSubject reports that standalone Marco cannot see.
//
// Deliberately `unavailable` rather than a guess. Recognising a screen means running the detector
// stack Director owns; a Marco started on its own has no such thing, and answering anything else
// would be answering a question it did not ask.
func (l *liveScreens) CurrentSubject(string) (string, screenhost.Outcome) {
	return "", screenhost.Unavailable
}
