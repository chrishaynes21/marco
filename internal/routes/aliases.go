package routes

// WHAT ELSE A ROUTINE ANSWERS TO.
//
// A routine has exactly one name and that name is the file: `Slug` folds case, punctuation,
// quotes and runs of whitespace onto one durable identity, and the route IS its scope and that
// slug. So the only way to reach a routine by a second phrase was to save a second routine, which
// is two files, two recordings and two things to keep in step.
//
// This is the second phrase, kept beside the source like the demonstration, the provenance and the
// step account. One routine, one place in the library, several ways in.
//
// # It is a name for the IDENTITY, which is why it carries no digests
//
// The step account beside it records a rendering BASIS, so it is bound to the bytes it rendered
// and a moved source digest means something. A phrase is not about the bytes at all: editing a
// step, re-simplifying, regenerating or rebuilding all change the source and change nothing about
// what the routine is called. A file bound to the source would have dropped somebody's phrases
// every time they fixed a wait.
//
// What it IS bound to is the identity — `<app>/context/<slug>` and the rest — because that is the
// thing that moves. A rename or a rescope rewrites it as part of the move, and a file whose
// binding names some other identity is NOT believed: its phrases are not offered and the state
// says why. Fail-closed in that one direction, for the reason the provenance sidecar refuses an
// inheritance: the cost of believing an orphan is a phrase that runs a routine nobody attached it
// to.
//
// # The slug always wins
//
// [Registry.Resolve] answers on the slug exactly as it always did, and only a miss reaches the
// phrases. So a phrase that runs something today keeps running it, and no routine can lose its own
// name to somebody else's alias — the same ordering, for the same reason, as a demonstration
// arm sitting after the play arm in `invoke.Decide`.
//
// # And the phrases are stored as somebody typed them
//
// Verbatim. The folding happens when they are COMPARED, so the list read back is the list that was
// written — a person who added "enter free play" is not shown `enter-free-play`.
//
// See [[Command-aliases-proposal-2026-10-30]].

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// aliasExt is the suffix for a routine's other names, beside its source.
const aliasExt = ".aliases.json"

// AliasesVersion is the shape of the file written today.
//
// A file stating anything else is UNREADABLE rather than absent, for the reason the step account
// gives: a version from the future was written by a build that knew something this one does not,
// and reading it as "no phrases" would quietly drop them the next time anything wrote the file.
const AliasesVersion = 1

// StoredAliases is what a routine also answers to, and which routine that is.
type StoredAliases struct {
	Version int `json:"version"`
	// Route is the identity these phrases belong to — `global/<slug>`,
	// `<app>/context/<slug>`, `<app>/focus/<slug>`.
	//
	// THE BINDING, and the only one there is. See the note above for why it is the
	// identity and not a digest of anything.
	Route string `json:"route"`
	// Phrases is what the routine also answers to, exactly as they were typed.
	Phrases []string `json:"phrases"`
}

// AliasState is what a phrase file on disk says about the routine it sits beside.
type AliasState string

const (
	// AliasesNone is no file at all: the routine answers to its own name and nothing
	// else, which is what every routine already on disk is.
	AliasesNone AliasState = "none"
	// AliasesIntact is a file bound to the routine reading it. The only state whose
	// phrases are offered.
	AliasesIntact AliasState = "intact"
	// AliasesElsewhere is a file whose binding names another identity — one left behind
	// by a move whose rewrite did not land, or one sitting under a slug a different
	// routine has since been saved to.
	//
	// ITS PHRASES ARE NOT OFFERED. A phrase answering for a routine nobody attached it
	// to is the one outcome this file exists to prevent.
	AliasesElsewhere AliasState = "elsewhere"
	// AliasesOrphaned is a file with no source beside it: there is no routine here to
	// answer to anything.
	AliasesOrphaned AliasState = "orphaned"
	// AliasesUnreadable is a file that cannot be parsed, or states a version this build
	// does not know. SAID, never treated as absent.
	AliasesUnreadable AliasState = "unreadable"
)

// Offered is whether this state's phrases may answer a request. Only one state's may.
func (a AliasState) Offered() bool { return a == AliasesIntact }

// ID is a route's whole identity — its place in the library.
//
// Here rather than in a caller because it is what the phrase file binds itself to, and a second
// spelling of a route's identity is a second thing for a binding check to disagree with.
// `orchestrator.RouteID` is this function.
func (rt Route) ID() string {
	if rt.App == "" {
		return GlobalDir + "/" + rt.Slug
	}
	scope := ContextDir
	if rt.Focus {
		scope = FocusDir
	}
	return rt.App + "/" + scope + "/" + rt.Slug
}

// AliasesPath is where a routine's other names sit.
func (r Registry) AliasesPath(rt Route) string {
	return filepath.Join(r.locDir(rt), rt.Slug+aliasExt)
}

// HasAliases is whether a routine has a phrase file at all, whatever it says.
func (r Registry) HasAliases(rt Route) bool { return fileExists(r.AliasesPath(rt)) }

// readAliases is the ONE read of the file, and the one place "there is none" is told apart from
// "it could not be read" — the correction `readSteps` and `Collision` both carry.
func (r Registry) readAliases(rt Route) (raw []byte, found bool, err error) {
	raw, err = os.ReadFile(r.AliasesPath(rt))
	switch {
	case err == nil:
		return raw, true, nil
	case os.IsNotExist(err):
		return nil, false, nil
	}
	return nil, true, err
}

// Aliases reads what a routine also answers to, and whether it still belongs to it.
//
// The phrases are filled whenever the file parsed, whatever the state, because a surface refusing
// to offer them still wants to say what the file claims. Only [AliasesIntact] is offered.
func (r Registry) Aliases(rt Route) (StoredAliases, AliasState) {
	raw, found, rerr := r.readAliases(rt)
	switch {
	case rerr != nil:
		return StoredAliases{}, AliasesUnreadable
	case !found:
		return StoredAliases{}, AliasesNone
	}
	var s StoredAliases
	if err := json.Unmarshal(raw, &s); err != nil || s.Version != AliasesVersion ||
		s.Route == "" || len(s.Phrases) == 0 {
		// A FILE THAT CANNOT ANSWER IS NOT AN ABSENT FILE. One with no binding is
		// one nothing can check, which is the single thing this file exists to make
		// impossible, and one with no phrases claims nothing and should not be on
		// disk at all.
		return StoredAliases{}, AliasesUnreadable
	}
	if !fileExists(r.Path(rt)) {
		return s, AliasesOrphaned
	}
	if s.Route != rt.ID() {
		return s, AliasesElsewhere
	}
	return s, AliasesIntact
}

// SaveAliases writes what a routine answers to, bound to the routine it is written beside.
//
// The binding is set HERE rather than taken from a caller, so a file that says it belongs
// somewhere else cannot be produced by passing the wrong thing in — the only way to get one is a
// move that did not finish, which is what the state is for.
//
// Atomically, through the helper the other sidecars use: a reader never sees a half-written file.
// An empty list REMOVES the file rather than writing one that claims nothing.
func (r Registry) SaveAliases(rt Route, phrases []string) error {
	if rt.Slug == "" {
		return fmt.Errorf("routes: a routine needs a name")
	}
	kept := make([]string, 0, len(phrases))
	for _, p := range phrases {
		if p = strings.TrimSpace(p); p != "" {
			kept = append(kept, p)
		}
	}
	if len(kept) == 0 {
		return r.DeleteAliases(rt)
	}
	raw, err := json.MarshalIndent(StoredAliases{
		Version: AliasesVersion, Route: rt.ID(), Phrases: kept}, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(r.AliasesPath(rt), append(raw, '\n'))
}

// DeleteAliases removes what a routine answered to. Absent is not a failure.
func (r Registry) DeleteAliases(rt Route) error {
	err := os.Remove(r.AliasesPath(rt))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// AliasesDigest is a digest of the phrase file as it stands, with "" for no file at all, and
// `readable` false for a file that is there and could not be read.
//
// What a review binds itself to, beside the source, the demonstration and the step account: a
// phrase file that moved while somebody read a move means they were taught another phrase since,
// and a move that carried the list it read at preparation would silently drop it.
//
// THE THIRD ANSWER IS WHY THIS RETURNS TWO VALUES, exactly as [Registry.StepsDigest]: an
// unreadable file answering "" would compare equal to an absent one at the write and the move
// would go ahead having never read one of the files it is about.
func (r Registry) AliasesDigest(rt Route) (digest string, readable bool) {
	raw, found, err := r.readAliases(rt)
	switch {
	case err != nil:
		return "", false
	case !found:
		return "", true
	}
	return DigestOf(string(raw)), true
}

// AnsweringTo is every routine in the library whose phrases answer to this name.
//
// Through `Slug` on both sides, so a phrase is matched by the same folding a name is — which is
// what makes an alias EXACT rather than a guess, and is why it needs nothing running.
//
// More than one is a real answer and not an error here. Two routines answering to one phrase is a
// question for whoever asked, and this package's job is to say that it happened rather than to
// pick between them. `List` is sorted, so the order is stable.
func (r Registry) AnsweringTo(name string) []Route {
	want := Slug(name)
	if want == "" {
		return nil
	}
	var out []Route
	for _, rt := range r.List() {
		s, st := r.Aliases(rt)
		if !st.Offered() {
			continue
		}
		for _, p := range s.Phrases {
			if Slug(p) == want {
				out = append(out, rt)
				break
			}
		}
	}
	return out
}
