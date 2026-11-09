package demonstrated

// EVERY BINDING RESOLVED BEFORE ANY ACTION, OR NOTHING HAPPENS.
//
// # Why the whole play, not each sentence as it comes
//
// Because a play whose SECOND sentence names an unbound interaction would otherwise press the
// first control and then stop — one real press on somebody's computer, in service of a run that
// was never going to finish. "Refuses before input" has to mean before the first input, not
// before the one it could not resolve.
//
// So preflight answers every question that can be answered without looking at a screen:
//
//	the record       a version this build can vouch for, with at least one binding
//	the play         compiles, through the production checker
//	the pair         the record's digest is this play's digest — which is also what makes
//	                 the record COMPLETE for it, see bindings.go
//	one routine      pinned to one revision, because the adapter holds one coordinator
//	the routine      is saved, and is still the revision that was bound
//	every position   is an interaction that routine actually has
//
// What it deliberately does NOT answer: whether the screen is right, whether the control can be
// found, or whether anybody will agree to the press. Those are Director's, they need a look, and
// they are asked per action at the moment the action is about to happen. A preflight that tried
// would be a stale answer by the time it mattered.

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Routines is where saved demonstrations are found. `demo.Store` satisfies it.
//
// An interface rather than the store, so this package does not reach into a directory and a test
// does not need one. The production caller passes the real store.
type Routines interface {
	Sequence(name string) (workflowstore.Artifact, bool)
}

// Plan is a preflighted play: what to perform, and which interaction each name means.
//
// The ONLY way to get one is through Preflight, and the host takes one rather than taking a
// source and a record — so there is no path from an unchecked pair to a press.
type Plan struct {
	// Play is THE VALIDATED BYTES, and the only thing execution is allowed to run.
	//
	// # Why the plan carries the source rather than a path
	//
	// Because the first version ran the play by PATH: preflight read the file, checked it,
	// and then `RunFileWithHostsCtx` read it again. An edit or a replacement between the
	// two reached the runtime unchecked — it would be compiled, which establishes that it
	// is legal Marco and nothing more. Not that it is the play whose bindings were
	// resolved, and not that it still has the nested continuation that is this bridge's
	// only gate between one action and the next.
	//
	// So the bytes are read once, checked, and carried. There is no path on a Plan and
	// nothing downstream can open a file: "the saved source controls execution" is a claim
	// about the bytes that were saved, and this is the snapshot of them.
	Play string
	// Name labels the play in an error, and is never opened.
	Name string
	// Routine is the saved demonstration, as loaded.
	Routine workflowstore.Artifact
	// Revision is what it was pinned to and still is.
	Revision string
	// Steps is name-in-the-play to interaction position.
	Steps map[string]int
	// Order is the interaction names IN THE ORDER THE PLAY PERFORMS THEM.
	//
	// What makes "interaction 2 of 3" a true sentence about the Act rather than about the
	// demonstration underneath it. `Steps` cannot answer that: it is a map, and the
	// position it holds is the DEMONSTRATION's, which for an Act composed by picking is a
	// different number from the one a person is watching. See
	// cmd/director/interactionidentity.go for what that cost before it was carried.
	Order []string
	// Source is the play's own digest, for the record and the report.
	Source string
}

// Preflight checks a play and its bindings together, and refuses rather than repairing.
// Plays answers whether a play is legal Marco, WITHOUT running it.
//
// An interface because `internal/director` may not import the engine: Marco programs reach the
// machine through `directorapi.MarcoRunner`, wired in `cmd/director`, and
// `internal/director/boundary_test.go` walks every import to hold that. The first draft of this
// package called `driver.CheckSource` directly and the guard caught it on five files.
//
// The production implementation is one line over that same checker, so the question asked here
// is the question the runtime asks. See the two-lists-one-fact defect: a spec test was green
// while production refused the same play, because the two were asking different things about one
// source.
//
// # It used to have a second method, and that was the mistake
//
// `Interactions(src)` ran the play with a recording host and swept prefixes — answer ok to the
// first k asks, fail the rest — to discover which interactions it could ask for. Sound for the
// generated shape, and NOT analysis of arbitrary Marco: a play that asks A, asks B when A fails,
// and asks C when B SUCCEEDS is never shown C by any prefix, because k=0 fails B and k>=1 never
// enters the fallback. It would have called a record complete with C unbound.
//
// Completeness is now established by rebuilding the play from the record, which supports exactly
// the generated form and refuses everything else by name. See `Preflight`.
type Plays interface {
	Check(src string) error
}

func Preflight(src string, b Bindings, in Routines, check Plays) (Plan, error) {
	if b.Version != BindingsVersion {
		return Plan{}, fmt.Errorf("these bindings are version %d and this build writes %d",
			b.Version, BindingsVersion)
	}
	if len(b.Scenes) == 0 {
		return Plan{}, fmt.Errorf("this play has no bindings; there is no way to know " +
			"which demonstrated interaction any of its sentences means")
	}
	if check == nil {
		return Plan{}, fmt.Errorf("there is no way to check that this play is legal " +
			"Marco, and an unchecked play is not performed")
	}
	if err := check.Check(src); err != nil {
		return Plan{}, fmt.Errorf("this play does not compile, so nothing will be "+
			"performed from it: %w", err)
	}
	// THE PAIR. A mismatch is a play that has been changed since its bindings were
	// written — which is allowed, and means it has to be saved again before it is
	// replayed. Reordering works; reordering without a save does not.
	if got := routes.DigestOf(src); got != b.Source {
		return Plan{}, fmt.Errorf("this play has changed since its bindings were saved "+
			"(the file is %s, the bindings are for %s); save it again to bind the "+
			"interactions it names now", got, b.Source)
	}
	routine, revision, err := b.one()
	if err != nil {
		return Plan{}, err
	}
	if in == nil {
		return Plan{}, fmt.Errorf("there is nowhere to look for %q", routine)
	}
	art, ok := in.Sequence(routine)
	if !ok {
		return Plan{}, fmt.Errorf("%q is not a saved play on this machine, so the "+
			"interactions this play names cannot be found", routine)
	}
	now, err := RevisionOf(art)
	if err != nil {
		return Plan{}, err
	}
	if now != revision {
		return Plan{}, fmt.Errorf("%q has changed since this play was saved (it is now "+
			"%s, the play is bound to %s); the interactions may not be the same ones",
			routine, now, revision)
	}
	for _, s := range b.Scenes {
		// THE POSITION, SAID SPECIFICALLY. The rebuild below refuses this too, but a
		// person reading "interaction 7 of a routine that has 2" can act on it, and
		// "this play cannot be rebuilt from its bindings" is a sentence about a file.
		if s.Position < 0 || s.Position >= len(art.Steps) {
			return Plan{}, fmt.Errorf("%q names interaction %d of %q, which has %d",
				s.Name, s.Position, routine, len(art.Steps))
		}
	}

	// ── AND THE RECORD IS SHOWN TO BE COMPLETE FOR THIS PLAY, BY REBUILDING IT ──
	//
	// The digest above says the FILE has not changed. It says nothing about the record,
	// which is a separate file: remove one binding and the digest still matches, and a
	// preflight that validated only the bindings present accepted the pair — then
	// interaction one was grounded, authorised and PRESSED before interaction two was
	// refused as unbound.
	//
	// So the play is rebuilt from the record and compared, byte for byte. Equality
	// establishes, without inferring anything:
	//
	//	every name the play asks for is bound        the generator wrote it FROM these
	//	every binding is asked for                   the generator wrote nothing else
	//	the gate is there                            the generator writes the nested form
	//	the order is the one recorded                the rebuild renders that order
	//
	// And INEQUALITY is a refusal rather than a diagnosis. A play this bridge did not
	// generate is not supported — not analysed, not probed, not partially accepted. The
	// attempt before this one ran the play to discover what it asks for, which cannot be
	// exhaustive for arbitrary Marco: a play that asks A, asks B when A fails, and asks C
	// when B succeeds is never shown C by any prefix of answers.
	rebuilt, _, cerr := Compose(art, b.Act, b.Interactions(), b.SavedAt, check)
	if cerr != nil {
		return Plan{}, fmt.Errorf("this play cannot be rebuilt from its bindings, so "+
			"they cannot be shown to describe it: %w", cerr)
	}
	if rebuilt != src {
		return Plan{}, fmt.Errorf("this play is not the one these bindings describe: " +
			"rebuilding it from them gives different source. Only a play Marco wrote " +
			"from a demonstration runs here, and it runs only while its bindings still " +
			"match it — save it again to bind the interactions it names now")
	}
	// THE PLAN IS BUILT ONLY NOW, from a record shown to describe this play.
	//
	// After the rebuild rather than before it, so nothing is ever derived from a record that
	// has not been established. Two bindings of one name, for instance, would collapse into
	// one entry here and leave the run's own account of how many interactions it should
	// reach off by one — and `Compose` refuses that case above, which is the one place a
	// duplicate name is refused at all.
	steps := map[string]int{}
	// AND THE ORDER, which is the Act's own numbering and is not recoverable from the
	// map: `b.Scenes` is the order the generator wrote the sentences in, and the rebuild
	// above is what establishes that the play performs them in it.
	order := make([]string, 0, len(b.Scenes))
	for _, s := range b.Scenes {
		steps[s.Name] = s.Position
		order = append(order, s.Name)
	}
	return Plan{Routine: art, Revision: revision, Steps: steps, Order: order,
		Source: b.Source, Play: src}, nil
}

// PlayName labels this play in an error message. It is not a path and nothing opens it.
func (p Plan) PlayName() string {
	if p.Name != "" {
		return p.Name
	}
	if p.Routine.Name != "" {
		return p.Routine.Name
	}
	return "a demonstrated play"
}
