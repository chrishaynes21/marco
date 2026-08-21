package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
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
//
// One of those three reads now goes over a socket to the Director, and the claim survives intact
// on both ends. The request it sends is `ObserveQuery{Showing: …}`, whose only field is an
// application name; the reply's only fields are a durable subject id and a reason. There is no
// field on either that names a key, a window, a route or a memory write, and the Director answers
// it out of `freshPlace`, which looks and resolves and does nothing else. Asking where you are is
// still not a way of doing something — it is just no longer a way of finding out nothing.

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
// ONE store, not one per process. Screen names are written by the Director while it learns, and
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
// front"; and for "which place is in front", this asks the Director, because the Director owns the
// only perception in the system. When no Director is reachable the answer is `unavailable` and the
// guard refuses — the same refusal that was here before, now reached by looking rather than by
// assuming.
//
// This host answers for a guarded source run LOCALLY — `marco run`, `marco do` on a saved file,
// and the case that matters most, an EDITED learned play. A learned play with intact provenance is
// performed by the Director (see [[ADR-078-a-learned-play-is-performed-by-the-director]]) and never
// runs its `do Screen's Showing` lines here. Edit it and it stops being that: `Resolved.Learned()`
// means learned AND provenance-verified, so an edited play takes the local runner, exactly as the
// authority seam promises the person it will. Which is why the stub that used to live below was
// not a harmless honesty — it made every edited play unrunnable at its own first line.
type liveScreens struct{ store *semanticmemory.Store }

func (l *liveScreens) Application() string { return winctx.Active() }

func (l *liveScreens) SubjectNamed(application, name string) (string, bool) {
	r, ok := l.store.SubjectNamed(application, name)
	if !ok {
		return "", false
	}
	return r.ID, true
}

// CurrentSubject asks the Director which place is in front, and refuses when it cannot ask.
//
// # Why it asks rather than answering
//
// Recognising a screen means taking a fresh look and resolving it against durable memory. There is
// exactly one thing in this tree that can do that — `Runtime.freshPlace` — and it lives in the
// Director. This used to return `unavailable` unconditionally, which was a true statement about
// this process and a false one about the system: the eyes exist, in a sibling process, one dial
// away. ADR-031 Decision 4 said so in its own words while the stub was being written: "Director
// figures out the play; Marco performs it; **Director still provides the eyes while it does**."
// This is that sentence, wired.
//
// # Why it does not start a Director
//
// Same reasoning as `pendingQuestion` in intake.go, and it is worth repeating because the cost is
// asymmetric. A Director that is not running cannot see, so the honest answer is already known
// before the dial: an outage costs ONE failed connection rather than a twenty-second service start
// charged to a play's first line. A read must never pay for a launch.
//
// # The one thing that may never be weakened
//
// Every path out of here that is not a positive identification is a refusal. No Director, a
// Director that could not look, a place it does not recognise, a malformed reply, an outcome word
// this build has never heard of, or `recognised` with no subject id — all of them refuse. There is
// no fallback, no nearest match, and no "assume it's fine, the person asked for it". Letting this
// guess is the single fastest way to destroy [[ADR-031-the-user-names-the-stage]] and "silence is
// never yes" together, and the tests below assert the refusals precisely so nobody can do it by
// accident while making a success case pass.
//
// Deleting the Director query must fail TestTheDirectorsAnswerSatisfiesTheEntryGuard; weakening
// any refusal must fail TestNoDirectorMeansMarcoCannotSeeAndSaysSo or
// TestADirectorThatDoesNotRecogniseTheScreenRefuses.
func (l *liveScreens) CurrentSubject(application string) (string, screenhost.Outcome) {
	// NO AUTO-START — see above. `false` is the whole of that decision.
	c, err := directorConnect(false)
	if err != nil {
		return "", screenhost.Unavailable
	}
	defer c.Close()

	raw, err := c.Observation(service.ObserveQuery{
		Showing: &service.ObserveShowing{Application: application},
	})
	if err != nil {
		// A Director that answered with an error did not answer the question. "I could not
		// check" is what happened, and it is not the same as "I checked and it was different".
		return "", screenhost.Unavailable
	}
	var view service.ShowingView
	if json.Unmarshal(raw, &view) != nil {
		return "", screenhost.Unavailable
	}

	// Back into the vocabulary's own type, by conversion rather than by a table of words.
	// `internal/director` may not import platform code, so the outcome crosses the wire as a
	// bare string; converting it here — at the other composition root, against `screenhost`'s
	// own constants — keeps that one vocabulary rather than making it two.
	switch screenhost.Outcome(view.Outcome) {
	case screenhost.Recognised:
		if strings.TrimSpace(view.Subject) == "" {
			// A recognition with nothing recognised. Structurally impossible from a Director
			// of this build, which is exactly why it is treated as a build disagreement
			// rather than trusted: an empty id compared against a named subject would
			// refuse anyway, and saying so as `unavailable` sends whoever reads the
			// diagnostics to the right place.
			return "", screenhost.Unavailable
		}
		return view.Subject, screenhost.Recognised
	case screenhost.Ambiguous, screenhost.Unobservable, screenhost.Unreadable, screenhost.Unknown:
		// Carried through as itself. All of these become `failed` at the language boundary
		// and stay apart in the diagnostics, because "I could not see" and "I saw a different
		// screen" call for opposite fixes.
		return "", screenhost.Outcome(view.Outcome)
	default:
		// Anything outside the vocabulary — the empty string an older Director's session
		// snapshot decodes to, or a word a newer one invented — is "I could not check".
		// Never a match, and never silently one of the others.
		return "", screenhost.Unavailable
	}
}
