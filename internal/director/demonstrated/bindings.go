// Package demonstrated makes a saved demonstration executable as one complete legal Marco play.
//
// # What moves, and what does not
//
// Before this, a demonstrated routine's ORDER lived in Go: `workflowrun.Runner` walked a cursor
// from the front of the artifact, and each individual effect was lowered to legal Marco and
// compiled on its way to the machine. The order moves here, into a saved `.marco` play whose two
// sentences are the two interactions and whose `when ok?` nesting is the gate between them.
// Nothing else moves. Fresh grounding, per-action authorisation, bounded verification and the
// evidence stay exactly where they are, which is why this package drives the production
// coordinator rather than containing a second one.
//
// # The three files, and which of them is allowed to be missing
//
//	<slug>.marco          the play. The order, and the continuation.
//	<slug>.scenes.json    the BINDINGS. Which demonstrated interaction each name in the play
//	                      means. REQUIRED: a play with no bindings does not run here at all.
//	<slug>.origin.json    provenance. OPTIONAL, and it is the existing sidecar.
//
// Keeping those last two apart is the whole of this file's design. `routes.Origin` is allowed to
// be absent — a `.marco` with no sidecar is an ordinary authored play, which is the right
// fail-closed direction for PROVENANCE, because the surviving state claims nothing. Bindings are
// not provenance. They decide which control a sentence presses, and a missing or stale one must
// not degrade into "an ordinary play somebody wrote": it must stop the run. The same record
// written into `Origin` would have inherited exactly the fallback that must not happen here.
//
// # How a record is shown to be COMPLETE for a play
//
// By rebuilding the play from it and comparing bytes. `Preflight` hands the loaded routine and
// this record's interactions, in order, back to `Compose` — the one generator — and requires the
// result to equal the file. Equality establishes four things at once, none of them inferred:
//
//   - every name the play asks for is bound, because the generator wrote the play FROM the
//     bindings;
//   - every binding is asked for, for the same reason;
//   - the play is the supported nested form, with the `when ok?` gate between each interaction
//     and the next — a hand-written or replaced play is refused whatever it says;
//   - the order the file performs is the order this record describes.
//
// # Two things that were tried first, and why they are not here
//
// A DIGEST MATCH ALONE. `Source` pins the source, so a changed file is caught — but the record is
// a separate file and can lose or rename a binding while the source stays byte for byte what it
// was. Remove the second binding, leave the digest alone, and preflight accepted the pair,
// because it validated only the bindings that were present. Interaction one would then be
// grounded, authorised and PRESSED and interaction two refused as unbound afterwards.
//
// PROBING THE COMPILED PLAY. The next attempt ran the play with a host that recorded names and
// performed nothing, sweeping prefixes — answer ok to the first k asks, fail the rest. It is
// sound for the generated shape and it is NOT analysis: a play that asks A, asks B when A fails,
// and asks C when B succeeds is never shown C by any prefix, because k=0 fails B and k>=1 never
// enters the fallback. The checker would call the record complete with C unbound, and execution
// could press B before finding out. A finite set of simulated runs is not an exhaustive account
// of arbitrary legal Marco, and treating it as one was the mistake.
//
// So the supported form is ENFORCED rather than analysed. Anything else is refused by name,
// before grounding, before authorisation, before input.
//
// # Reordering
//
// Editing the play changes its bytes, which makes the rebuild disagree, which refuses the run. A
// reordered play REPLAYS only after an explicit save that writes the record again. Reordering
// works; reordering behind Marco's back does not.
package demonstrated

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// BindingsVersion is the binding record's format.
//
// Its own version, independent of the play, the artifact and the provenance sidecar. A record
// this build cannot vouch for is REFUSED rather than half-read: a reader that ignored a field it
// did not know would press a control chosen by a rule it had never heard of.
const BindingsVersion = 1

// bindingsExt is the suffix for a play's bindings, beside its source.
const bindingsExt = ".scenes.json"

// Binding is which demonstrated interaction one name in a play means.
//
// Pinned to a revision, always. [[ADR-183-remembering-recognising-and-being-allowed-to-act]] is
// why `Position` alone would not do: a saved routine can be corrected, and a corrected routine's
// third interaction is not the same interaction as the third one in the file somebody bound.
type Binding struct {
	// Name is what the play calls this interaction — the name a person gave it.
	Name string `json:"name"`
	// Routine is the saved demonstration this interaction belongs to, by the name a person
	// gave it.
	Routine string `json:"routine"`
	// Revision is the digest of that routine's saved artifact at the moment of binding.
	//
	// NOT its SavedAt. `SavedAt` deliberately does not move when a person corrects a
	// procedure, because it is the link to the raw demonstration beside it, and
	// `CorrectedAt` moves instead — so a timestamp-based revision would call a corrected
	// procedure the same revision as the one it replaced.
	Revision string `json:"revision"`
	// Position is which interaction of that routine, as `workflowstore.Step.Position`.
	Position int `json:"position"`
	// Evidence is the digest of the raw demonstration behind the routine, when there is
	// one. Provenance about the binding, never consulted to decide anything.
	Evidence string `json:"evidence,omitempty"`
}

// Bindings is the required execution record for one play.
type Bindings struct {
	Version int `json:"version"`
	// Source is the digest of the play these bindings were written with.
	//
	// Not the completeness proof — see the package comment. It is the cheap first answer
	// and the one that gives a person the right sentence: "the file changed" and "the
	// record changed" are different problems with the same remedy, and only this field can
	// tell them apart.
	Source string `json:"source"`
	// Application is the executable this play may run against, carried from the routine's
	// own scope. Empty is valid for nothing, exactly as `workflowstore.Scope` says.
	Application string `json:"application,omitempty"`
	// Act is the name this play was composed under, which the rebuild needs.
	//
	// Stored because `Preflight` establishes the record by REBUILDING the play from it,
	// and the play carries the Act's name in its header, its actor and its final line. A
	// record without it could not reproduce the file it describes.
	Act string `json:"act"`
	// Scenes is the interactions this play performs, IN THE ORDER THE GENERATOR WROTE THEM.
	//
	// # Why an order lives here, when the play's order is the play's
	//
	// Because this order is not an execution order. It is the input the generator needs to
	// rebuild the play, and rebuilding it is how the record is shown to be complete for the
	// file. Nothing at run time consults it: the host resolves a name, and which sentence
	// comes next is decided by the file's own nesting.
	//
	// So it cannot compete with the play's order. A record whose order disagrees with the
	// file rebuilds to different bytes, and different bytes are a REFUSAL rather than a
	// choice between two accounts of what to do.
	//
	// An earlier version kept a map, on the reasoning that a JSON object has no order to
	// disagree with. It also had no order to rebuild from, which is why its completeness
	// check had to be inferred — and the inference was wrong.
	Scenes []Binding `json:"scenes"`
	// SavedAt is when the pair was written.
	SavedAt time.Time `json:"saved_at"`
}

// Marshal writes the record.
func (b Bindings) Marshal() ([]byte, error) {
	b.Version = BindingsVersion
	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

// UnmarshalBindings reads one back, refusing a version this build cannot vouch for.
func UnmarshalBindings(raw []byte) (Bindings, error) {
	var b Bindings
	if err := json.Unmarshal(raw, &b); err != nil {
		return Bindings{}, fmt.Errorf("these bindings are not readable: %w", err)
	}
	if b.Version != BindingsVersion {
		return Bindings{}, fmt.Errorf("these bindings are version %d and this build writes "+
			"%d; it will not guess what changed", b.Version, BindingsVersion)
	}
	return b, nil
}

// RevisionOf is the pinned identity of a saved routine: the digest of its own saved bytes.
func RevisionOf(a workflowstore.Artifact) (string, error) {
	raw, err := a.Marshal()
	if err != nil {
		return "", err
	}
	return routes.DigestOf(string(raw)), nil
}

// Interactions is what the generator needs to rebuild the play, in the order it wrote them.
func (b Bindings) Interactions() []Named {
	out := make([]Named, 0, len(b.Scenes))
	for _, s := range b.Scenes {
		out = append(out, Named{Position: s.Position, Name: s.Name})
	}
	return out
}

// Find is the binding for one name, and whether there is one.
func (b Bindings) Find(name string) (Binding, bool) {
	for _, s := range b.Scenes {
		if s.Name == name {
			return s, true
		}
	}
	return Binding{}, false
}

// names is the bound names, sorted, for a sentence a person reads.
func (b Bindings) names() []string {
	out := make([]string, 0, len(b.Scenes))
	for _, s := range b.Scenes {
		out = append(out, s.Name)
	}
	sort.Strings(out)
	return out
}

// one is the single routine and revision this record refers to, or a refusal.
//
// Scene sharing across routines is DELIBERATELY not supported yet: an explicit reference pinned
// to a revision is allowed, and composing interactions from two different demonstrations is not
// advertised until the adapter performs it, because the adapter holds one coordinator over one
// artifact. A record that asked for two is refused with the sentence rather than performing the
// first and ignoring the second.
func (b Bindings) one() (routine, revision string, err error) {
	seen := map[string]bool{}
	for _, s := range b.Scenes {
		seen[s.Routine+"@"+s.Revision] = true
		routine, revision = s.Routine, s.Revision
	}
	if len(seen) > 1 {
		keys := make([]string, 0, len(seen))
		for k := range seen {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return "", "", fmt.Errorf("this play draws on %d demonstrations (%s); performing "+
			"interactions from more than one is not supported yet",
			len(seen), strings.Join(keys, ", "))
	}
	return routine, revision, nil
}
