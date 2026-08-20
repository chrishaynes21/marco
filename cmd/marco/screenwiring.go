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
// The host is handed a `screenhost.Recognition`: three reads and nothing else — which application
// is in front, which screen is in front, and what the user has named. It cannot press a key, focus
// a window, run a route or write to memory, whatever it decides.

// newScreenHost builds the Screen act's implementation.
//
// A host with no recogniser behind it is a real answer. A Marco that cannot tell where it is makes
// a guarded play refuse rather than degrade into blind replay — see
// [[ADR-031-the-user-names-the-stage]].
func newScreenHost() *screenhost.Host {
	return screenhost.New(newScreenRecognition())
}

// newScreenRecognition opens the durable store this process reads screens out of.
//
// Split out of [newScreenHost] so a test can hold the injected backing itself: the host keeps its
// `Recognition` unexported (correctly — nothing may reach past it), which leaves no other way to
// assert that the composition root opened the right file. The two are one composition; a test that
// enters here enters the production wiring.
func newScreenRecognition() screenhost.Recognition {
	store, _ := semanticmemory.Open(memoryPath())
	if store == nil {
		return nil
	}
	return &liveScreens{store: store}
}

// memoryPath is where durable semantic memory lives: the Director's store, under `$MARCO_HOME`.
//
// ONE store, not one per process. Screen names are written by the Director while it teaches, and
// read here while a play runs; a second file beside the routes could only ever be empty, and an
// empty store made `Screen's Showing` refuse with "nothing in <app> is called <name>" — a sentence
// about a name that was in fact recorded, just somewhere else.
//
// `$MARCO_MEMORY` still wins, and `cmd/director` honours it identically. An override one process
// obeys and the other ignores is the same split under a different name.
func memoryPath() string {
	if p := strings.TrimSpace(os.Getenv("MARCO_MEMORY")); p != "" {
		return p
	}
	return filepath.Join(directorDir(), "semantic-memory.json")
}

// liveScreens is the read-only view the Screen host is given.
//
// Semantic memory answers "what is this place called"; the window tracker answers "what is in
// front". Recognising the CURRENT screen needs perception Director owns, and standalone Marco does
// not have it — so `CurrentSubject` reports `unavailable` and the guard refuses.
//
// On the product path nothing reaches that refusal: a learned play is performed by the Director
// (see [[ADR-078-a-learned-play-is-performed-by-the-director]]), which re-plans and verifies out of band and
// never runs the play's `do Screen's Showing` lines here. This host answers for a guarded source
// run locally — `marco run`, `marco do` on a saved file — and its job there is to refuse honestly,
// naming what it could not do rather than a name it could not find.
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
