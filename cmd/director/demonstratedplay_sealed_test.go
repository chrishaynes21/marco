package main

// THE TWO HOLES THE ARCHITECT FOUND IN THE FAIL-CLOSED CLAIM.
//
// Both are about the gap between what was CHECKED and what is PERFORMED, and both let real input
// reach somebody's computer in service of a run that was never going to finish. They are kept
// together because they are one mistake made twice: trusting a digest to say more than it says.

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// A BINDING MISSING FOR A LATER INTERACTION REFUSES BEFORE ANY INPUT.
//
// `Preflight` compared the play's digest against the record and treated a match as a completeness
// proof, on the reasoning that the play is generated from the bindings. But the digest is over the
// SOURCE, and the record is a separate file — so removing the SECOND binding and leaving the
// digest alone produced a pair the old preflight accepted, because it validated only the bindings
// that were present.
//
// What followed is the thing this bridge exists to make impossible: interaction one grounded,
// authorised and PRESSED, and interaction two refused as unbound afterwards.
//
// The existing refusal cases covered an empty map and a missing FIRST name. Neither reaches this:
// a missing first name is refused at the first sentence and so presses nothing by accident.
func TestABindingMissingForALaterInteractionRefusesBeforeAnyInput(t *testing.T) {
	art := dpATwoTapRoutine()
	good := dpSave(t, art, dpTheNames())
	src, binds := good.dpReload(t)

	for _, c := range []struct {
		name string
		b    demonstrated.Bindings
		says string
	}{
		{
			name: "the second binding removed, digest untouched",
			b:    without(binds, "apply changes"),
			says: "not the one these bindings describe",
		},
		{
			name: "the second binding renamed, digest untouched",
			b:    renamed(binds, "apply changes", "apply the changes"),
			says: "not the one these bindings describe",
		},
		{
			name: "a binding the play never asks for",
			b:    plus(binds, "something else", 1),
			says: "not the one these bindings describe",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			// THE DIGEST STILL MATCHES. That is the whole point of the case: the
			// source is byte for byte what it was.
			if c.b.Source != binds.Source {
				t.Fatalf("this case changed the digest and so proves nothing")
			}
			now := dpLooked
			run, bind, grant, ground, book := dpARun(t, art, &now)
			st := dpAWorkingStage()
			st.book, st.now = book, &now
			_ = run
			_ = bind

			plan, err := demonstrated.Preflight(src, c.b,
				dpAStore{art.Name: art}, marcoChecks{})
			if err == nil {
				t.Fatalf("this pair was accepted, and a run from it would press "+
					"interaction one before refusing two: %+v", plan.Steps)
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("the refusal does not say why: %v", err)
			}
			// ZERO AUTHORIZATION REQUESTS AND ZERO EFFECTS, which is the claim.
			// Nothing was grounded either: preflight is reached before the
			// coordinator is asked anything at all.
			if len(grant.asked) != 0 {
				t.Errorf("somebody was asked about %+v", grant.asked)
			}
			if book.clicks() != 0 {
				t.Errorf("%d press(es) went out", book.clicks())
			}
			if ground.times != 0 {
				t.Errorf("the screen was grounded %d time(s)", ground.times)
			}
		})
	}
}

// REPLACING THE PLAY AFTER PREFLIGHT CHANGES NOTHING.
//
// Execution ran the play BY PATH, so the runtime reread the file. An edit or a replacement
// between preflight and execution reached it unchecked — it would be compiled, which establishes
// that it is legal Marco and nothing else. Not that it is the play whose bindings were resolved,
// and not that it still has the nested continuation that is the only gate between one action and
// the next.
//
// The replacement written here is the proof of why that matters: legal Marco, asking for the same
// two interactions, with NO GATE between them. Under it both presses would go out.
func TestReplacingThePlayAfterPreflightDoesNotChangeWhatRuns(t *testing.T) {
	art := dpATwoTapRoutine()
	now := dpLooked
	s := dpSave(t, art, dpTheNames())
	src, binds := s.dpReload(t)
	plan, err := demonstrated.Preflight(src, binds, dpAStore{art.Name: art}, marcoChecks{})
	if err != nil {
		t.Fatal(err)
	}
	if werr := os.WriteFile(s.play, []byte(aGatelessReplacement), 0o644); werr != nil {
		t.Fatal(werr)
	}

	run, bind, _, _, book := dpARun(t, art, &now)
	// THE OUTCOME IS WITHHELD, so the validated play's first interaction cannot verify and
	// its gate must stop the run after one press. The replacement has no gate, so if it
	// were what ran, two presses would go out.
	st := dpAWalk(nil, nil)
	st.limit = 120
	st.book, st.now = book, &now
	var said strings.Builder
	h, _ := performDemonstratedPlay(context.Background(), dpTheRun, plan,
		demonstrated.NewSupervised(run, bind, st.look, 120), &said)

	if strings.Contains(said.String(), "REPLACEMENT RAN") {
		t.Fatalf("the replacement was executed: %q", said.String())
	}
	if book.clicks() != 1 {
		t.Fatalf("%d press(es) went out; the validated play presses once when its "+
			"first interaction cannot be verified", book.clicks())
	}
	if len(h.Asked()) != 1 {
		t.Fatalf("the account of the run is %+v", h.Asked())
	}
}

// aGatelessReplacement is legal Marco that asks for the same interactions with nothing between
// them — what an unchecked reread could have been handed.
const aGatelessReplacement = `use demonstrated.

the Replaced is an actor.
this can Run.
this's Run does...
    do Demonstrated's Press with "the graphics tab".
    do Demonstrated's Press with "apply changes".
    this is ok!

the App is a script.

do Replaced's Run...
    when ok?
        log "REPLACEMENT RAN".
    or?
        log that's error.
`

// A PLAN WITH NO VALIDATED BYTES PERFORMS NOTHING.
//
// The structural half: there is no path on a Plan, so the only thing execution can run is what
// preflight put there. A plan assembled without it is refused rather than falling back to a file.
func TestAPlanWithNoValidatedBytesPerformsNothing(t *testing.T) {
	art := dpATwoTapRoutine()
	now := dpLooked
	run, bind, grant, _, book := dpARun(t, art, &now)
	st := dpAWorkingStage()
	st.book, st.now = book, &now
	h, err := performDemonstratedPlay(context.Background(), dpTheRun,
		demonstrated.Plan{Routine: art, Steps: map[string]int{"the graphics tab": 0}},
		demonstrated.NewSupervised(run, bind, st.look, 120), io.Discard)
	if err == nil {
		t.Fatal("a plan carrying no play was performed")
	}
	// AND IT SAYS WHY, which is what pins the guard rather than the runtime's own
	// refusal to compile an empty program: the two fail for different reasons and only
	// one of them is about this bridge.
	if !strings.Contains(err.Error(), "preflighted") {
		t.Errorf("the refusal is the runtime's, not this bridge's: %v", err)
	}
	if book.clicks() != 0 || len(grant.asked) != 0 {
		t.Fatalf("%d press(es) and %d authorisation(s)", book.clicks(),
			len(grant.asked))
	}
	if got := h.Outcome(); got == runref.Verified {
		t.Fatalf("it reported %q", got)
	}
}

// AND NO FILE BESIDE A PLAY CAN CHANGE WHAT ITS SENTENCES MEAN.
//
// A consequence of running the snapshot with an empty import directory, and worth its own
// assertion because the alternative is quiet: with a directory, `use demonstrated.` resolves a
// sibling `demonstrated.marco` AHEAD of the built-in act surface. A file dropped next to a play
// could redefine the act it is performed through — declaring `Press` as something else entirely,
// and the play would read exactly the same.
func TestASiblingActSurfaceCannotRedefineTheDemonstratedAct(t *testing.T) {
	art := dpATwoTapRoutine()
	now := dpLooked
	s := dpSave(t, art, dpTheNames())
	src, binds := s.dpReload(t)
	plan, err := demonstrated.Preflight(src, binds, dpAStore{art.Name: art}, marcoChecks{})
	if err != nil {
		t.Fatal(err)
	}
	// A SIBLING SURFACE declaring an act of the same name with a different capability.
	// If it were resolved, the play would not compile — `Press` would not exist — so the
	// run failing to start is the tell, and it must NOT happen.
	beside := `the Demonstrated is an act.
this exports SomethingElse.
`
	if werr := os.WriteFile(s.dir+string(os.PathSeparator)+"demonstrated.marco",
		[]byte(beside), 0o644); werr != nil {
		t.Fatal(werr)
	}
	run, bind, _, _, book := dpARun(t, art, &now)
	st := dpAWorkingStage()
	st.book, st.now = book, &now
	h, rerr := performDemonstratedPlay(context.Background(), dpTheRun, plan,
		demonstrated.NewSupervised(run, bind, st.look, 120), io.Discard)
	if rerr != nil {
		t.Fatalf("a file beside the play changed what it compiles to: %v", rerr)
	}
	if got := h.Outcome(); got != runref.Verified {
		t.Fatalf("the run ended %q: %+v", got, h.Asked())
	}
	if book.clicks() != 2 {
		t.Fatalf("%d press(es) went out", book.clicks())
	}
}

// without is the bindings with one name removed and the SOURCE DIGEST LEFT ALONE.
func without(b demonstrated.Bindings, name string) demonstrated.Bindings {
	out := copyBindings(b)
	keep := out.Scenes[:0]
	for _, s := range out.Scenes {
		if s.Name != name {
			keep = append(keep, s)
		}
	}
	out.Scenes = keep
	return out
}

// renamed is the bindings with one name changed, which leaves the play's name unbound and the
// record's name unused.
func renamed(b demonstrated.Bindings, from, to string) demonstrated.Bindings {
	out := copyBindings(b)
	for i := range out.Scenes {
		if out.Scenes[i].Name == from {
			out.Scenes[i].Name = to
		}
	}
	return out
}

// plus is the bindings with one name added that the play never asks for.
func plus(b demonstrated.Bindings, name string, at int) demonstrated.Bindings {
	out := copyBindings(b)
	if len(out.Scenes) == 0 {
		return out
	}
	one := out.Scenes[0]
	out.Scenes = append(out.Scenes, demonstrated.Binding{Name: name,
		Routine: one.Routine, Revision: one.Revision, Position: at})
	return out
}

// reordered is the bindings with the same interactions in the other order, which rebuilds to a
// different play and is therefore refused rather than performed.
func reordered(b demonstrated.Bindings) demonstrated.Bindings {
	out := copyBindings(b)
	for i, j := 0, len(out.Scenes)-1; i < j; i, j = i+1, j-1 {
		out.Scenes[i], out.Scenes[j] = out.Scenes[j], out.Scenes[i]
	}
	return out
}

func copyBindings(b demonstrated.Bindings) demonstrated.Bindings {
	return demonstrated.Bindings{Version: b.Version, Act: b.Act, Source: b.Source,
		Application: b.Application, SavedAt: b.SavedAt,
		Scenes: append([]demonstrated.Binding(nil), b.Scenes...)}
}

// THE STRUCTURE PROBING COULD NEVER SEE, REFUSED BY NAME.
//
// The Architect's counterexample, and the reason the checker is not a probe. A play that asks A,
// asks B when A FAILS, and asks C when B SUCCEEDS is never shown C by any prefix of answers:
// with k=0 both A and B fail, and with k>=1 A succeeds so the fallback is never entered. The old
// checker returned {A, B} and would have accepted a record with C unbound — and execution could
// then have pressed B before discovering it.
//
// Rebuilding the play from the record answers it without analysing anything. This play is not
// what the generator writes, so it is refused: no third interaction to miss, no partial
// acceptance, and nothing grounded, authorised or pressed.
func TestAPlayWithAFallbackThatLeadsOnIsRefusedRatherThanPartlyUnderstood(t *testing.T) {
	art := dpATwoTapRoutine()
	now := dpLooked
	// A RECORD THAT LOOKS COMPLETE FOR WHAT THE OLD CHECKER WOULD HAVE FOUND: the two
	// names a prefix sweep discovers, and not the third.
	src := theFallbackChain
	binds := demonstrated.Bindings{
		Version: demonstrated.BindingsVersion,
		Act:     "a chain",
		Source:  routes.DigestOf(src),
		Scenes: []demonstrated.Binding{
			{Name: "the graphics tab", Routine: art.Name, Position: 0},
			{Name: "apply changes", Routine: art.Name, Position: 1},
		},
	}
	rev, err := demonstrated.RevisionOf(art)
	if err != nil {
		t.Fatal(err)
	}
	for i := range binds.Scenes {
		binds.Scenes[i].Revision = rev
	}

	run, bind, grant, ground, book := dpARun(t, art, &now)
	st := dpAWorkingStage()
	st.book, st.now = book, &now
	_ = run
	_ = bind

	_, perr := demonstrated.Preflight(src, binds, dpAStore{art.Name: art}, marcoChecks{})
	if perr == nil {
		t.Fatal("a play with a fallback leading to a third interaction was accepted; " +
			"the third one is unbound and would have been reached after a press")
	}
	if !strings.Contains(perr.Error(), "not the one these bindings describe") {
		t.Errorf("the refusal does not say the play is unsupported: %v", perr)
	}
	// ZERO GROUNDING, ZERO AUTHORIZATION, ZERO INPUT.
	if ground.times != 0 || len(grant.asked) != 0 || book.clicks() != 0 {
		t.Fatalf("grounded %d, asked %d, pressed %d", ground.times,
			len(grant.asked), book.clicks())
	}
}

// theFallbackChain is legal Marco, nested, and NOT what the generator writes: the third
// interaction sits behind a successful fallback.
const theFallbackChain = `use demonstrated.

the Chain is an actor.
this can Run.
this's Run does...
    do Demonstrated's Press with "the graphics tab"...
        when ok?
            this is ok!
        or?
            do Demonstrated's Press with "apply changes"...
                when ok?
                    do Demonstrated's Press with "the third one"...
                        when ok?
                            this is ok!
                        or?
                            this is that!
                or?
                    this is that!

the App is a script.

do Chain's Run...
    when ok?
        log "done".
    or?
        log that's error.
`

// AND REORDERING THE RECORD WITHOUT SAVING THE PLAY IS REFUSED.
//
// The order in the record is the generator's input, not an execution order — so a record whose
// order disagrees with the file rebuilds to different source, and that is a refusal rather than a
// choice between two accounts of what to do. This is what stops the two from ever competing.
func TestAReorderedRecordIsRefusedRatherThanObeyed(t *testing.T) {
	art := dpATwoTapRoutine()
	now := dpLooked
	s := dpSave(t, art, dpTheNames())
	src, binds := s.dpReload(t)
	run, bind, grant, ground, book := dpARun(t, art, &now)
	st := dpAWorkingStage()
	st.book, st.now = book, &now
	_ = run
	_ = bind

	_, err := demonstrated.Preflight(src, reordered(binds),
		dpAStore{art.Name: art}, marcoChecks{})
	if err == nil {
		t.Fatal("a record whose order disagrees with the play was accepted")
	}
	if ground.times != 0 || len(grant.asked) != 0 || book.clicks() != 0 {
		t.Fatalf("grounded %d, asked %d, pressed %d", ground.times,
			len(grant.asked), book.clicks())
	}
}

// TWO BINDINGS OF ONE NAME ARE REFUSED WITH THEIR OWN SENTENCE.
//
// The rebuild would refuse them too — `Compose` will not write a play with two interactions of
// one name — but a plan built from them would also have collapsed the two into one entry, so the
// run's own account of how many interactions it should reach would be wrong. The specific
// refusal comes first because it is the one a person can act on.
func TestTwoBindingsOfOneNameAreRefused(t *testing.T) {
	art := dpATwoTapRoutine()
	s := dpSave(t, art, dpTheNames())
	src, binds := s.dpReload(t)
	doubled := copyBindings(binds)
	doubled.Scenes[1].Name = doubled.Scenes[0].Name

	_, err := demonstrated.Preflight(src, doubled, dpAStore{art.Name: art}, marcoChecks{})
	if err == nil {
		t.Fatal("two bindings of one name were accepted")
	}
	if !strings.Contains(err.Error(), "both called") {
		t.Errorf("the refusal does not say which: %v", err)
	}
}
