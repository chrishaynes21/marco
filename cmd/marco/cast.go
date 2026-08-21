package main

import (
	"net/http"
)

// The CAST table, on the Advanced page: who could perform a play on this machine right now.
//
// # Why it is called Cast and not Actors
//
// Because `director learn --actor` already means the play's SUBJECT — the application being
// demonstrated — and `actor` is also a Marco language keyword with a third meaning again. Putting
// a table headed "Actors" beside a Learn tab whose flag means something else would ADD a
// confusion rather than remove one. Cast is the group, it is the word the Theater already uses for
// the act of choosing one, and it collides with nothing a person can type.
//
// The column heading inside the table is still "Actor", singular, because each ROW is one — that
// is the sense that is unambiguous once the table has a name.
//
// # Why this is Advanced
//
// A play names nobody. Who performs it is decided at run time and a person never has to know. But
// when nothing happens at all, one of the explanations is not about the screen or the phrase: this
// machine has nothing that can act, and every learned play will refuse `no_actor_available`
// forever. That is the question this table answers, and it is the reason it must never fabricate
// a row — a plausible-looking cast for a question Marco could not ask would send somebody looking
// at the wrong thing.
//
// # It is the same account `marco director diagnose --json` prints
//
// `theaterDiagnostics` in director.go, called unchanged. Not a second roster built here: the
// Roster asks each Actor the same availability question casting itself asks, and a surface that
// worked it out its own way would agree with the product right up until the moment somebody
// needed it to disagree.

// castRow is one Actor as the Advanced page shows it: Actor | Provider | Where | Available | Why not.
type castRow struct {
	// Actor is the capability, which is what casting selects on — "accessibility", not a product.
	Actor string `json:"actor"`
	// Provider is the installation behind it, and Where is that installation on disk. Kept
	// apart because "no provider" and "a provider somewhere you did not expect" are different
	// problems that look identical from a play that did nothing.
	Provider string `json:"provider,omitempty"`
	Where    string `json:"where,omitempty"`
	// Available is the answer to the question casting asks, at the moment it was asked.
	Available bool `json:"available"`
	// Why is the sentence that came back when it cannot act — usually the operating system's
	// own refusal, naming the binary it could not run. This is the last place it could be
	// dropped before it reaches the person who has to fix it.
	Why string `json:"why,omitempty"`
}

// castReport asks this machine who could act.
//
// A package variable for the same reason `runSpawn` and `pendingQuestion` are: a test must be able
// to render this table without touching the desktop, discovering a bridge, or depending on what
// happens to be installed on the machine running the suite. Production never reassigns it.
var castReport = theaterDiagnostics

// handleCast renders the cast, or says honestly that it has none.
//
// Three different answers, and they must stay three: a provider was found and the actors can act;
// a provider was found and they cannot, with the reason; and Marco could not ask at all. The third
// is the one that invites invention, so it is the one written down explicitly.
//
// Deleting the empty-roster arm must fail TestTheCastRendersAnHonestEmptyState.
func handleCast(w http.ResponseWriter, _ *http.Request) {
	r := castReport()
	rows := []castRow{}
	for _, p := range r.Roster {
		rows = append(rows, castRow{
			Actor:     p.Name,
			Provider:  p.Provider,
			Where:     p.Path,
			Available: p.Available,
			Why:       p.Reason,
		})
	}
	why := ""
	if len(rows) == 0 {
		// NOT "nobody is available" — that would be a claim about the machine made from the
		// absence of an answer. An empty roster means nothing was asked, and the sentence says
		// so, followed by what it costs the person: every learned play refuses.
		why = "Marco could not ask who is available on this machine, so it has nothing to show. " +
			"Until something can act, a learned play will do nothing at all."
	}
	writeJSON(w, map[string]any{
		"provider": r.Bridge,
		"cast":     rows,
		"last":     r.Last,
		"why":      why,
	})
}
