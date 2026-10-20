package main

// WHICH INTERACTION A SNAPSHOT IS ABOUT, AND WHETHER IT IS THE ACT'S NUMBER OR THE ROUTINE'S.
//
// An Act is composed by picking, so those are two different numbers, and a window that shows the
// second under the name of the first is telling somebody something false about the press they are
// being asked to authorise. Every test here is about keeping them apart.

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// ── 1. THE NUMBERING ──────────────────────────────────────────────────────────

// AN ACT'S POSITIONS ARE THE ACT'S, AND THE STEP IS CARRIED BESIDE THEM.
func TestAnActPublishesItsOwnNumberingAndNotTheDemonstrationsStep(t *testing.T) {
	art := dpATwoTapRoutine()
	// AIMED AT THE ROUTINE'S SECOND STEP AS THE ACT'S FIRST, which is what a backwards
	// Act does and is the case the two numberings disagree in.
	scenes := actInteractions(art, 2)

	scenes.aim(1, "apply changes", 1)
	at := scenes.at(1, "")
	if at.Index != 1 || at.Of != 2 {
		t.Errorf("the Act's first interaction is %d of %d", at.Index, at.Of)
	}
	if at.Name != "apply changes" {
		t.Errorf("it is called %q", at.Name)
	}
	if at.Step != 1 {
		t.Errorf("it carries step %d, and the demonstration's step is evidence "+
			"that must survive", at.Step)
	}
	if at.Control != art.Steps[1].Control {
		t.Errorf("it carries control %q, want %q", at.Control, art.Steps[1].Control)
	}
	if at.Verdict != "" {
		t.Errorf("nothing has been decided and it says %q", at.Verdict)
	}

	// AND THE VERDICT IS BESIDE THE NAME, NEVER IN PLACE OF IT.
	with := scenes.at(1, "verified")
	if with.Verdict != "verified" || with.Name != "apply changes" {
		t.Errorf("with a verdict it reads %+v", with)
	}
	// A COPY EACH TIME, because an event is a snapshot: two of them sharing one struct
	// would be a window watching a field change under it.
	if with == at {
		t.Error("two snapshots were handed the same interaction")
	}
	if at.Verdict != "" {
		t.Error("asking with a verdict changed the earlier snapshot")
	}

	// AND AN AIM THAT HAS OUTLIVED ITS INTERACTION NAMES NOTHING.
	//
	// Index 0 is "nobody could say", which is a true answer; naming the other Scene
	// would not be, and neither would numbering this one from its step.
	other := scenes.at(0, "")
	if other.Name != "" {
		t.Errorf("a step nothing aimed was called %q", other.Name)
	}
	if other.Index != 0 {
		t.Errorf("a step nothing aimed was numbered %d of the Act", other.Index)
	}
	if other.Of != 2 || other.Control != art.Steps[0].Control {
		t.Errorf("it lost what IS known about the step: %+v", other)
	}

	scenes.over()
	if after := scenes.at(1, ""); after.Name != "" || after.Index != 0 {
		t.Errorf("after the sentence finished, step 1 still reads %+v", after)
	}
}

// AND A WALK OF AN ARTIFACT NUMBERS ITS STEPS, because its steps are its interactions.
//
// The case where the two numberings agree, said rather than relied on: a workflow run has no
// Scenes and nobody aims anything, and reporting index 0 for all of it would lose a number a
// person can use.
func TestAWorkflowWalkNumbersTheStepsItWalks(t *testing.T) {
	art := dpATwoTapRoutine()
	scenes := workflowInteractions(art)
	at := scenes.at(1, "")
	if at.Index != 2 || at.Of != 2 {
		t.Errorf("step 1 of a two-step workflow is %d of %d", at.Index, at.Of)
	}
	if at.Name != "" {
		t.Errorf("a workflow step was given the name %q, and nobody named it",
			at.Name)
	}
	if at.Control != art.Steps[1].Control {
		t.Errorf("it carries control %q", at.Control)
	}
}

// ── 2. THE THREE STATES OF AN ATTEMPT ─────────────────────────────────────────

// A PRESS, THE WAIT AFTER IT, AND THE VERDICT ARE THREE SNAPSHOTS AND NOT TWO.
//
// The wait is the one that was missing. It is the longest part of an interaction — the press has
// gone out and the evidence has not arrived — and with nothing emitted for it a front end
// rendering the latest snapshot showed "acting" for the whole of it.
func TestTheCoordinatorsThreeStatesAreEachSaid(t *testing.T) {
	art := dpATwoTapRoutine()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	say, err := runstate.NewWriter(path, "r_1_1", "an act", art.Name, false)
	if err != nil {
		t.Fatal(err)
	}
	scenes := actInteractions(art, 2)
	scenes.aim(1, "apply changes", 1)

	sayWhatTheCoordinatorDecided(say, scenes, workflowrun.Attempt{Step: 1,
		Result: &marcoexec.Result{}, Awaiting: true})
	sayWhatTheCoordinatorDecided(say, scenes, workflowrun.Attempt{Step: 1,
		Outcome: workflowrun.OutcomeWaiting, Awaiting: true})
	sayWhatTheCoordinatorDecided(say, scenes, workflowrun.Attempt{Step: 1,
		Verified: &workflowrun.Verification{Step: 1,
			Verdict: workflowrun.VerdictVerified}})
	// AND AN ATTEMPT THAT IS NOT ABOUT AN ACTION AT ALL SAYS NOTHING. A look nobody
	// could take, a screen that cannot be established: there is no new state to
	// publish, and publishing one would withdraw a question that is still open.
	sayWhatTheCoordinatorDecided(say, scenes, workflowrun.Attempt{Step: 1,
		Outcome: workflowrun.OutcomeWaiting})
	say.Close()

	events, rerr := runstate.Read(path)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if len(events) != 3 {
		t.Fatalf("%d event(s) for three states and a silence: %v", len(events),
			phasesIn(events))
	}
	if events[0].Phase != runstate.Acting {
		t.Errorf("the press was published as %q", events[0].Phase)
	}
	if events[1].Phase != runstate.Verifying || events[1].Interaction.Verdict != "" {
		t.Errorf("the wait after the press reads phase=%q verdict=%q",
			events[1].Phase, events[1].Interaction.Verdict)
	}
	if events[2].Phase != runstate.Verifying ||
		events[2].Interaction.Verdict != string(workflowrun.VerdictVerified) {
		t.Errorf("the verdict reads phase=%q verdict=%q", events[2].Phase,
			events[2].Interaction.Verdict)
	}
	for i, e := range events {
		if e.Interaction == nil || e.Interaction.Name != "apply changes" ||
			e.Interaction.Index != 1 {
			t.Errorf("event %d names %+v", i, e.Interaction)
		}
	}
}

// AND THE LOOP IS WHAT CALLS IT.
//
// The whole emission is one call in the between-looks closure. A function nobody calls publishes
// nothing, and what a window renders would silently be whatever the run last said.
func TestTheLoopSaysWhatTheCoordinatorDecided(t *testing.T) {
	if !strings.Contains(theAssembly(t), "sayWhatTheCoordinatorDecided(say, scenes, a)") {
		t.Error("the observation loop no longer publishes what the coordinator " +
			"decided, so a front end would show `acting` through verification " +
			"and a verdict nobody announced")
	}
}

// ── 3. THE WHOLE SEQUENCE, THROUGH THE PRODUCTION PATH ────────────────────────

// dpPerformSaying is dpPerform with the run's structured account turned on.
//
// The production identity, the production emitter as the between-looks watch, and the stream on
// disk — so what is asserted below is the sequence a window would really read.
func dpPerformSaying(t *testing.T, s dpSaved, art workflowstore.Artifact, st *dpStage,
	now *time.Time, path string) (*demonstrated.Account, []runstate.Event) {

	t.Helper()
	src, binds := s.dpReload(t)
	plan, err := demonstrated.Preflight(src, binds, dpAStore{art.Name: art}, marcoChecks{})
	if err != nil {
		t.Fatalf("the saved pair did not preflight: %v", err)
	}
	run, bind, _, _, book := dpARun(t, art, now)
	st.book = book
	st.now = now
	say, werr := runstate.NewWriter(path, dpTheRun, "an act", art.Name, false)
	if werr != nil {
		t.Fatal(werr)
	}
	scenes := actInteractions(art, len(plan.Order))
	sup := demonstrated.NewSupervised(run, bind, st.look, 120).
		WithWatch(func(c context.Context, a workflowrun.Attempt) (bool, string) {
			sayWhatTheCoordinatorDecided(say, scenes, a)
			return true, ""
		})
	acc, _ := performDemonstratedPlay(context.Background(), dpTheRun, plan, sup,
		scenes, io.Discard)
	say.Close()
	events, rerr := runstate.Read(path)
	if rerr != nil {
		t.Fatal(rerr)
	}
	return acc, events
}

// A REORDERED ACT'S SNAPSHOTS FOLLOW THE ACT, in its own numbering and under its own names.
//
// The same routine and the same two interactions, saved backwards — so every number a window
// shows is one the play decided and not one the artifact did. This is the case the first draft
// got wrong in both directions at once: it published the demonstration's step as the Act's
// position, and the control's identifier as the interaction's name.
//
// The ORDER is asserted too, because it is ADR-242 made visible: interaction 1's verdict arrives
// before interaction 0 is mentioned at all.
func TestAReorderedActsSnapshotsAreNumberedAndNamedByThePlay(t *testing.T) {
	art := dpATwoTapPanel()
	now := dpLooked
	backwards := []demonstrated.Named{
		{Position: 1, Name: "apply changes"},
		{Position: 0, Name: "the graphics tab"},
	}
	st := dpAPanel(dpPanelWords(dpSecondOutcome()...), dpPanelWords(dpFirstOutcome()...))
	acc, events := dpPerformSaying(t, dpSave(t, art, backwards), art, st, &now,
		filepath.Join(t.TempDir(), "events.jsonl"))
	if got := acc.Outcome(); got != runref.Verified {
		t.Fatalf("the reordered play ended %q: %+v", got, acc.Asked())
	}

	var said []runstate.Event
	for _, e := range events {
		if e.Interaction != nil {
			said = append(said, e)
		}
	}
	if len(said) < 4 {
		t.Fatalf("%d event(s) named an interaction across two presses and two "+
			"verdicts: %v", len(said), phasesIn(events))
	}
	// EVERY ONE OF THEM KNOWS HOW MANY THE ACT HAS, which is the field that used to be
	// left unset.
	for _, e := range said {
		if e.Interaction.Of != 2 {
			t.Fatalf("an interaction says it is one of %d: %+v",
				e.Interaction.Of, e.Interaction)
		}
	}
	// THE PLAY'S ORDER: the first thing mentioned is the Act's FIRST interaction, which
	// is the demonstration's SECOND step.
	first := said[0].Interaction
	if first.Index != 1 || first.Name != "apply changes" || first.Step != 1 {
		t.Fatalf("the first interaction published is %+v", first)
	}
	// AND THE SECOND IS NUMBERED 2 WHILE STANDING ON STEP 0.
	var second *runstate.Interaction
	for _, e := range said {
		if e.Interaction.Step == 0 {
			second = e.Interaction
			break
		}
	}
	if second == nil {
		t.Fatal("the second interaction was never named in the stream")
	}
	if second.Index != 2 || second.Name != "the graphics tab" {
		t.Fatalf("the second interaction published is %+v", second)
	}
	// NOBODY WAS SHOWN A CONTROL IDENTIFIER OR A VERDICT AS A NAME.
	for _, e := range said {
		n := e.Interaction.Name
		if n != "apply changes" && n != "the graphics tab" {
			t.Errorf("an interaction was named %q", n)
		}
	}
	// AND EACH ONE WAS WATCHED BEFORE IT WAS ANSWERED: a verifying snapshot with no
	// verdict, and then one with it.
	for _, step := range []int{1, 0} {
		watched, answered := false, false
		for _, e := range said {
			if e.Interaction.Step != step || e.Phase != runstate.Verifying {
				continue
			}
			if e.Interaction.Verdict == "" {
				watched = true
			} else if e.Interaction.Verdict ==
				string(workflowrun.VerdictVerified) {
				answered = true
			}
		}
		if !watched {
			t.Errorf("step %d went from pressed to answered with nothing said "+
				"about the interval between, which a window shows as "+
				"`acting`", step)
		}
		if !answered {
			t.Errorf("step %d's verdict was never published", step)
		}
	}
	// ADR-242, VISIBLE IN THE STREAM: the first interaction is answered before the
	// second is so much as mentioned.
	answeredFirst, mentionedSecond := -1, -1
	for i, e := range said {
		if e.Interaction.Step == 1 && e.Interaction.Verdict != "" && answeredFirst < 0 {
			answeredFirst = i
		}
		if e.Interaction.Step == 0 && mentionedSecond < 0 {
			mentionedSecond = i
		}
	}
	if answeredFirst < 0 || mentionedSecond < 0 || answeredFirst > mentionedSecond {
		t.Errorf("the second interaction appears at %d and the first was answered "+
			"at %d; the stream does not show the gate", mentionedSecond,
			answeredFirst)
	}
}

// AND THE HOST RELEASES THE SCENE WHEN ITS SENTENCE IS OVER.
//
// A source guard, honestly, because nothing observable depends on it today: every reader of the
// identity — the consent desk, the between-looks report — runs INSIDE a sentence, so there is no
// moment at which a stale aim is read. What does hold at runtime is the step check in `at`, which
// refuses to name a Scene the aim is no longer about, and that is tested above.
//
// The release is here for the moment that changes. A question asked between two sentences would
// otherwise carry the previous one's name and number, which is the exact class of wrong thing
// this whole file exists to stop a window saying.
func TestTheHostReleasesTheSceneItAimed(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(".", "demonstratedplay.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"h.scenes.aim(h.position(name), name, step)",
		"defer h.scenes.over()",
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the Demonstrated host no longer calls %q, so the run's "+
				"account of which interaction it is performing comes from "+
				"a step number again", want)
		}
	}
}
