package demonstrated

// SAVING A PLAY AND ITS BINDINGS INTO ROUTE STORAGE, WITHOUT OVERWRITING ANYBODY.
//
// Route storage is where a play with a past already lives: the source at `<slug>.marco`, its
// provenance beside it at `<slug>.origin.json`, and the write order chosen so that a crash leaves
// an ordinary untrusted play rather than a trusted wrong one
// ([[ADR-028-a-learned-play-is-a-file-with-a-past]]). The bindings join that convention at
// `<slug>.scenes.json`, findable by name alone and carried by whatever moves the play.
//
// # Three writes, and the order is the whole of the care
//
//	1. the play           an ordinary play if the session dies here, claiming nothing
//	2. the bindings       now it is runnable through this bridge
//	3. the provenance     now it says where it came from
//
// Backwards, a crash would leave bindings describing a play that does not exist, or provenance
// vouching for one that cannot run. Forwards, every surviving state is either "not there yet" or
// "an ordinary file somebody could read", which is the only direction that is safe.
//
// `SaveWithOrigin` writes 1 and 3 together in that same order internally, so this writes 1+3 and
// then 2 — which inverts the last pair and is why the provenance is written LAST by writing the
// bindings between them is not possible. So the bindings go first instead, before either: a
// `.scenes.json` with no play beside it is meaningless and `Load` refuses it, exactly as an
// orphaned origin record is refused.
//
// # Nothing is overwritten and nothing is shadowed
//
// The Architect's disposition, and the precedent is `demo.Store.SaveSequence`, which refuses when
// two names would land in one file rather than replacing somebody's routine with a
// differently-capitalised one. Here the rule is stricter because a play is executable: a name
// already taken is REFUSED, and Studio asks the person for another. Replacing a play somebody has
// is a decision only they can make, and this is not the place it gets made silently.

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// BindingsPath is where a play's bindings live: beside its source, under the same slug.
func BindingsPath(r routes.Registry, rt routes.Route) string {
	return strings.TrimSuffix(r.Path(rt), ".marco") + bindingsExt
}

// Save writes a composed play, its bindings and its provenance, refusing a name already in use.
//
// The provenance is `routes.KindDemonstrated` — a fourth kind, truthful rather than borrowed:
// `learned` means rehearsed and verified, and this was neither. Nothing here has been performed
// yet, and the first time anybody finds out is the first time somebody runs it under supervision.
func Save(r routes.Registry, rt routes.Route, src string, b Bindings, at time.Time) error {
	if r.Has(rt) {
		return fmt.Errorf("there is already a play called %q; give this one a different "+
			"name rather than replacing it", rt.Slug)
	}
	if _, err := os.Stat(BindingsPath(r, rt)); err == nil {
		// Bindings with no play. Left by a crash between two writes, or by a delete
		// that took the source and not the companion. Refused rather than reused: they
		// describe sentences this play may not have.
		return fmt.Errorf("there are already bindings filed under %q with no play; "+
			"remove them before saving a play there", rt.Slug)
	}
	b.Version = BindingsVersion
	if b.Source != routes.DigestOf(src) {
		// THE PAIR IS CHECKED HERE TOO. `Compose` returns them together, and a caller
		// that assembled one from somewhere else could otherwise save a pair that
		// passes preflight and performs something nobody composed.
		return fmt.Errorf("these bindings are for a different play (%s, not %s); "+
			"compose the two together", b.Source, routes.DigestOf(src))
	}
	raw, err := b.Marshal()
	if err != nil {
		return err
	}
	// THE BINDINGS FIRST. Alone they are meaningless and `Load` refuses them, so this is
	// the write whose surviving state claims the least.
	if err := os.MkdirAll(r.ScopeDir(rt), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(BindingsPath(r, rt), raw, 0o644); err != nil {
		return err
	}
	return r.SaveWithOrigin(rt, src, routes.Origin{
		Kind: routes.KindDemonstrated, Application: b.Application,
		SavedAt: at.UTC().Format(time.RFC3339),
	})
}

// Load reads a saved play and its bindings back, with neither trusted without the other.
func Load(r routes.Registry, rt routes.Route) (string, Bindings, error) {
	src, ok := sourceOf(r, rt)
	if !ok {
		return "", Bindings{}, fmt.Errorf("there is no play called %q", rt.Slug)
	}
	raw, err := os.ReadFile(BindingsPath(r, rt))
	if err != nil {
		// A PLAY WITH NO BINDINGS IS NOT RUNNABLE HERE, and that is the whole reason
		// bindings are not kept in the provenance sidecar: `routes.Origin` is allowed
		// to be absent, because a play with no provenance is an ordinary authored play
		// and claims nothing. A play with no bindings would claim to be an ordinary
		// play too — and this bridge would have nothing to say about which control each
		// of its sentences presses.
		return "", Bindings{}, fmt.Errorf("%q has no bindings, so there is no way to "+
			"know which demonstrated interaction any of its sentences means", rt.Slug)
	}
	b, err := UnmarshalBindings(raw)
	if err != nil {
		return "", Bindings{}, err
	}
	return src, b, nil
}

// sourceOf reads a play's source.
func sourceOf(r routes.Registry, rt routes.Route) (string, bool) {
	if !r.Has(rt) {
		return "", false
	}
	raw, err := os.ReadFile(r.Path(rt))
	if err != nil {
		return "", false
	}
	return string(raw), true
}
