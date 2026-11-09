package orchestrator

// A MOVE CARRIES WHAT THE ROUTINE ANSWERS TO.
//
// Found as a gap rather than by a failing test, which is the part worth recording: §4 of the
// contract had a row saying a move writes the phrases at the destination and verifies them before
// removing the original, the ADR repeated it, and the code did neither. `Rescope` copied the
// recording and the step account and not these — and then `DeleteLeftovers` removed the original's
// file, so a move silently took away every other way of reaching a routine and left nothing to
// recover them from.
//
// See [[ADR-275-a-routine-may-answer-to-more-than-one-phrase]].

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aMovableRoutineAnsweringTo is the move fixture with phrases attached.
func aMovableRoutineAnsweringTo(t *testing.T, phrases ...string) (Deps, routes.Route) {
	t.Helper()
	d, rt := aMovableRoutine(t)
	for _, p := range phrases {
		if _, err := d.AddAlias(rt, p); err != nil {
			t.Fatalf("teaching the fixture %q: %v", p, err)
		}
	}
	return d, rt
}

// THE PHRASES ARRIVE AT THE DESTINATION, BOUND TO IT, AND THE ORIGINAL'S FILE IS GONE.
//
// A real move through `PrepareRescope` and `Rescope`, which is where the gap was: everything about
// the phrase file is right in `internal/routes` and none of it happened here.
func TestAMoveCarriesThePhrasesTheRoutineAnswersTo(t *testing.T) {
	d, rt := aMovableRoutineAnsweringTo(t, "write my address", "fill the address in")
	got, err := moved(t, d, rt, ScopeApp, Consent{})
	if err != nil {
		t.Fatalf("the move failed: %v", err)
	}
	if got.To == (routes.Route{}) {
		t.Fatalf("the move landed nowhere: %+v", got)
	}
	answers, why := d.AliasesOf(got.To)
	if why != "" {
		t.Fatalf("the moved play's phrases cannot be read: %s", why)
	}
	if strings.Join(answers, "|") != "write my address|fill the address in" {
		t.Errorf("the moved play answers to %q", answers)
	}
	// BOUND TO WHERE IT NOW LIVES, which the state is the only check for: a file carried
	// without its binding rewritten reads as belonging somewhere else and answers for
	// nothing.
	if s, st := d.Reg.Aliases(got.To); st != routes.AliasesIntact || s.Route != got.To.ID() {
		t.Errorf("the phrases arrived as %q, bound to %q", st, s.Route)
	}
	// AND NOTHING IS LEFT AT THE OLD IDENTITY for the next routine saved there.
	if d.Reg.HasAliases(rt) {
		t.Error("the original's phrase file outlived the move")
	}
	// AND THE LIBRARY ANSWERS FOR THE ROUTINE AT ITS NEW PLACE.
	if back := d.Reg.AnsweringTo("write my address"); len(back) != 1 || back[0] != got.To {
		t.Errorf("the phrase is answered by %v", back)
	}
}

// A ROUTINE THAT ANSWERS TO NOTHING CLEARS WHAT IS SITTING AT THE DESTINATION.
//
// The half that is easy to miss. A phrase file at the destination — left by a deletion that could
// not sweep it, or by the routine this move replaces — has a binding that names that identity, so
// the moment this move's source exists it matches again and the arriving routine answers to
// phrases nobody attached to it. The destination's file is made to match the original either way:
// written when there are phrases, removed when there are none.
func TestAMoveClearsPhrasesSittingAtTheDestination(t *testing.T) {
	d, rt := aMovableRoutine(t)
	m, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatalf("preparing the move: %v", err)
	}
	if m.Refused != "" {
		t.Fatalf("the move was refused: %s", m.Refused)
	}
	// RESIDUE AT THE DESTINATION, bound to the identity this move is about to occupy.
	// The scope folder does not exist until something is written there, so this makes it
	// the way a deletion that could not sweep its own litter would have left it.
	if err := os.MkdirAll(filepath.Dir(d.Reg.AliasesPath(m.To)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeFileFor(t, d.Reg.AliasesPath(m.To), `{"version":1,"route":"`+
		m.To.ID()+`","phrases":["somebody elses phrase"]}`); err != nil {
		t.Fatal(err)
	}
	got, err := d.Rescope(m, Consent{})
	if err != nil {
		t.Fatalf("the move failed: %v", err)
	}
	if answers, _ := d.AliasesOf(got.To); len(answers) != 0 {
		t.Errorf("the moved play inherited %q", answers)
	}
	if len(d.Reg.AnsweringTo("somebody elses phrase")) != 0 {
		t.Error("the leftover phrase still answers for something")
	}
}

// AND A PHRASE TAUGHT BETWEEN THE REVIEW AND THE WRITE REFUSES THE MOVE.
//
// The move carries the list it read at preparation, so going ahead would write the older list at
// the destination and then delete the newer one with the original — losing a phrase somebody added
// while they were deciding. The same revalidation the source, the demonstration and the step
// account already get.
func TestAPhraseTaughtSinceTheMoveWasReviewedRefusesIt(t *testing.T) {
	d, rt := aMovableRoutineAnsweringTo(t, "write my address")
	m, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatalf("preparing the move: %v", err)
	}
	if _, err := d.AddAlias(rt, "put the address in"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Rescope(m, Consent{}); err == nil {
		t.Fatal("the move went ahead and took the newer phrase away with the original")
	}
	// AND THE ORIGINAL IS WHERE IT WAS, answering to both.
	if answers, _ := d.AliasesOf(rt); len(answers) != 2 {
		t.Errorf("the original answers to %q", answers)
	}
}

// AND A PHRASE FILE THAT CANNOT BE READ REFUSES THE MOVE AT PREPARATION.
//
// A move is bound to the files beside the routine, and this is one of them: a file that could not
// be read is one a move cannot say what it would do to. The correction every other sidecar in this
// package already carries.
func TestAMoveRefusesAnUnreadablePhraseFile(t *testing.T) {
	d, rt := aMovableRoutine(t)
	if err := writeFileFor(t, d.Reg.AliasesPath(rt), "{not json"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.PrepareRescope(rt, ScopeApp, ""); err == nil {
		t.Fatal("a move was prepared against a phrase file nothing could read")
	}
}

// AND THE PHRASES ARE READ BACK AT THE DESTINATION BEFORE THE ORIGINAL IS REMOVED.
//
// A write that returned no error is not a file that is there. The original is about to be deleted
// and its phrase file goes with it, so this is the only other copy — and `afterDestination` is
// the one seam that can occupy the instant between the two, which is why it exists.
func TestAMoveVerifiesThePhrasesAtTheDestination(t *testing.T) {
	d, rt := aMovableRoutineAnsweringTo(t, "write my address")
	m, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatalf("preparing the move: %v", err)
	}
	// SOMETHING ELSE REWRITES WHAT LANDED, between the destination write and the
	// removal of the original.
	d.afterDestination = func() {
		_ = d.Reg.SaveAliases(m.To, []string{"not what the move carried"})
	}
	got, err := d.Rescope(m, Consent{})
	if err != nil {
		t.Fatalf("the move reported an error rather than a retained original: %v", err)
	}
	if !strings.Contains(got.Says, "phrases") {
		t.Errorf("the move does not say what it found at the destination: %q",
			got.Says)
	}
	// AND THE ORIGINAL IS STILL THERE, which is the whole promise of reading back first.
	if !d.Reg.Has(rt) {
		t.Error("the original was deleted on the strength of a destination nobody read")
	}
	if answers, _ := d.AliasesOf(rt); len(answers) != 1 {
		t.Errorf("the original answers to %q", answers)
	}
}

// AND A REPLACING MOVE CLEARS THE PHRASES OF THE ROUTINE IT REPLACES.
//
// WRITTEN FOR A SURVIVOR, and the survivor was instructive: on a move to a FREE identity the
// destination's residue is cleared by `routes.Create` on its way past, so deleting this branch
// changed nothing. A move onto an OCCUPIED one goes through `Replace`, which writes over the
// source and touches nothing beside it — so this is the only thing standing between the arriving
// routine and the phrases of the one it replaced.
//
// The arriving routine answers to nothing here, which is the case that matters: the phrases it
// brings would otherwise be written over the old ones, and NO phrases means nothing is written at
// all.
func TestAReplacingMoveClearsThePhrasesOfWhatItReplaces(t *testing.T) {
	d, rt := aMovableRoutine(t)
	to := routes.Route{App: "notepad", Slug: rt.Slug}
	other := d.DraftOf(NameOf(d.Reg, rt), aPlainDemonstration("b"), "notepad",
		"the stop key", Unstated)
	if _, err := d.Save(other, ScopeApp, "notepad"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.AddAlias(to, "the one being replaced"); err != nil {
		t.Fatal(err)
	}
	m, err := d.PrepareRescope(rt, ScopeApp, "")
	if err != nil {
		t.Fatal(err)
	}
	src, rec := d.versionAt(to)
	got, err := d.Rescope(m, Consent{ID: m.TakenID, Source: src, Recording: rec})
	if err != nil {
		t.Fatalf("a consented move was refused: %v", err)
	}
	if answers, _ := d.AliasesOf(got.To); len(answers) != 0 {
		t.Errorf("the arriving play answers to %q, which somebody attached to the "+
			"play it replaced", answers)
	}
	if len(d.Reg.AnsweringTo("the one being replaced")) != 0 {
		t.Error("the replaced play's phrase still answers for something")
	}
}
