package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/learn"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// The learn session reaching every surface, through the production account.
//
// One value is published — `pkg/playbill.View` — and Normal, Watch and Diagnostics are three
// readings of it. The overlay imports that type and nothing else, so a fact that does not reach
// the account reaches no screen at all. This is where the wire is held.

// ── the wire ──────────────────────────────────────────────────────────────────

// TestTheLearningSectionReachesEverySurface enters at Runtime.Playbill, which is the only path
// from the Director to a presentation. Deleting the `v.LearnSession = r.learnSessionNow()` line must
// fail this.
func TestTheLearningSectionReachesEverySurface(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry(), learn: &learnSession{}}

	// A learn session, established through the coordinator rather than assembled here.
	c := learn.New("open target", &idlePasses{}, &stubTeachMemory{}, learn.DefaultBounds())
	rt.learn.coord, rt.learn.session, rt.learn.active = c, c.Session(), true

	v := rt.Playbill(service.PlaybillPayload{})
	if !v.LearnSession.Active {
		t.Fatal("a running learn session does not reach the account; every surface would " +
			"show a Director watching for no reason")
	}
	if v.LearnSession.Asked != "open target" {
		t.Errorf("the account says the user asked for %q", v.LearnSession.Asked)
	}
	if len(v.LearnSession.Progress) == 0 {
		t.Error("the account carries no checklist, so no surface can draw one")
	}
	// And it survives the admission guard, which is what a presentation actually receives.
	if err := v.Normalise().Admit(); err != nil {
		t.Fatalf("the Learn section was refused by the privacy guard: %v", err)
	}
	// All three readings see it.
	if h := v.Normalise().Normal(); h.Word != "Learning from you" {
		t.Errorf("Normal reads %q, want Learning from you", h.Word)
	}
	if !strings.Contains(watchTextOf(v), "LEARN SESSION") {
		t.Error("Watch shows no Learn section")
	}
}

// TestAnIdleDirectorPublishesNoLearningSection holds the other side: the wire must not invent
// a session where none exists.
func TestAnIdleDirectorPublishesNoLearningSection(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry(), learn: &learnSession{}}
	v := rt.Playbill(service.PlaybillPayload{})
	if v.LearnSession.Active || v.LearnSession.Asked != "" {
		t.Fatalf("an idle Director publishes a learn session: %+v", v.LearnSession)
	}
	if strings.Contains(watchTextOf(v), "LEARN SESSION") {
		t.Error("an idle Director shows a Learn section")
	}
}

// ── armed is read from the coordinator, never decided here ────────────────────

// The cue must come from the phase the coordinator is actually in. A surface that decided for
// itself when to say SHOW ME would be a second opinion about whether Marco is watching.
func TestTheCueIsReadFromTheCoordinatorsOwnPhase(t *testing.T) {
	armedIn := map[learn.Phase]bool{
		learn.EstablishingStart:       false,
		learn.ReadyForDemo:            true,
		learn.Capturing:               true,
		learn.EstablishingDestination: false,
		learn.Evaluating:              false,
		learn.NeedsAnotherExample:     false,
		learn.ReadyToRehearse:         false,
		learn.Rehearsing:              false,
		learn.Naming:                  false,
		learn.Lowering:                false,
		learn.Complete:                false,
		learn.Refused:                 false,
		learn.Cancelled:               false,
	}
	for phase, want := range armedIn {
		rt := &Runtime{observations: newObservationRegistry(), learn: &learnSession{}}
		rt.learn.coord = learn.New("open target", &idlePasses{}, &stubTeachMemory{},
			learn.DefaultBounds())
		rt.learn.session = learn.Session{Name: "open target", Phase: phase}
		rt.learn.active = !phase.Settled()

		got := rt.Playbill(service.PlaybillPayload{}).LearnSession
		if got.Armed != want {
			t.Errorf("phase %q publishes armed=%v, want %v", phase, got.Armed, want)
		}
		// Every phase must produce an account the guard admits. A phase nobody can render
		// is a phase a person is stranded in.
		v := playbill.View{Reach: playbill.Present, LearnSession: got}
		if err := v.Normalise().Admit(); err != nil {
			t.Errorf("phase %q produced an inadmissible account: %v", phase, err)
		}
	}
}

// ── completion comes from the artifact ────────────────────────────────────────

func TestThePublishedAccountSaysLearnedOnlyWhenAPlayExists(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry(), learn: &learnSession{}}
	rt.learn.coord = learn.New("open target", &idlePasses{}, &stubTeachMemory{},
		learn.DefaultBounds())

	// Complete, and nothing written. This is the state a bug produces, and the account
	// must not dress it up.
	rt.learn.session = learn.Session{Name: "open target", Phase: learn.Complete}
	if got := rt.Playbill(service.PlaybillPayload{}).LearnSession; got.Learned != "" {
		t.Fatalf("the account names a play %q that was never saved", got.Learned)
	}

	rt.learn.session.Saved = &learn.Saved{Name: "open target", Saved: true}
	got := rt.Playbill(service.PlaybillPayload{}).LearnSession
	if got.Learned != "open target" {
		t.Fatalf("a saved play is not published: %+v", got)
	}
	if got.Registered {
		t.Error("the account reports a saved play as registered")
	}
}

// ── what you did comes from the demonstration ─────────────────────────────────

func TestWhatYouDidIsReadFromTheDemonstrationAndNeverFromTiming(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry(), learn: &learnSession{}}
	rt.learn.coord = learn.New("open target", &idlePasses{}, &stubTeachMemory{},
		learn.DefaultBounds())

	// A completed demonstration whose steps carry no navigation. The screen changed and
	// nothing was attributed — the honest failure, and it must be published AS one.
	rt.learn.session = learn.Session{
		Name: "open target", Phase: learn.Evaluating,
		Demonstration: &observe.ProcedureCandidate{
			Complete: true,
			Steps: []observe.DemonstrationStep{{
				Arrived: observe.Checkpoint{Subject: "subj_end"},
			}},
		},
	}
	got := rt.Playbill(service.PlaybillPayload{}).LearnSession
	if len(got.Did) != 0 || !got.Unattributed {
		t.Fatalf("an unattributed change published %+v; it must say it could not tell",
			got)
	}

	// And with navigation, the ordered meanings — never a key.
	rt.learn.session.Demonstration.Steps[0].Intents = []observe.NavIntent{
		observe.NavDown, observe.NavConfirm}
	got = rt.Playbill(service.PlaybillPayload{}).LearnSession
	if strings.Join(got.Did, ",") != "down,confirm" {
		t.Fatalf("published %v, want the ordered meanings", got.Did)
	}
	if got.Unattributed {
		t.Error("an attributed demonstration is flagged unattributed")
	}
}

// ── the view is a read ────────────────────────────────────────────────────────

// Part 29 — reading the account, at any depth, changes nothing.
//
// Diagnostics is the interesting one: it is the densest read this system offers, and an
// observability surface that perturbed the thing it describes would be worthless precisely when
// somebody most needed it.
func TestReadingTheAccountChangesNoDirectorState(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry(), learn: &learnSession{}}
	c := learn.New("open target", &idlePasses{}, &stubTeachMemory{}, learn.DefaultBounds())
	rt.learn.coord, rt.learn.session, rt.learn.active = c, c.Session(), true

	before := c.Session()
	activeBefore := rt.observations.ActiveID()

	// Normal, Watch and Debug are three READS of one value; here they are all taken.
	for i := 0; i < 5; i++ {
		v := rt.Playbill(service.PlaybillPayload{})
		_ = v.Normalise().Normal()
		_ = v.Normalise().Watch()
		deep := rt.Playbill(service.PlaybillPayload{Diagnostics: true})
		_ = deep.Normalise().Deep()
	}

	after := c.Session()
	if after.Phase != before.Phase || after.Examples != before.Examples ||
		after.Named != before.Named {
		t.Fatalf("reading the account moved the learn session from %+v to %+v", before, after)
	}
	if rt.observations.ActiveID() != activeBefore {
		t.Fatal("reading the account started or stopped an observation session")
	}
	if len(rt.observations.finished) != 0 {
		t.Fatalf("reading the account produced %d session records",
			len(rt.observations.finished))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func watchTextOf(v playbill.View) string {
	var b strings.Builder
	for _, l := range v.Normalise().Watch() {
		b.WriteString(l.Text)
		b.WriteString("\n")
	}
	return b.String()
}

// U15's gap: "this is your turn" must reach the account, from the coordinator's own word.
//
// Found by mutation: the field existed, the narration used it, every view test passed — and the
// production wire never set it. A surface can only be as honest as what it is handed.
func TestWhoseTurnItIsReachesTheAccount(t *testing.T) {
	waitingIn := map[learn.Phase]bool{
		learn.EstablishingStart: false,
		learn.ReadyForDemo:      false,
		learn.Capturing:         false,
		learn.Evaluating:        false,
		learn.ReadyToRehearse:   true,
		learn.Rehearsing:        false,
		learn.Naming:            true,
		learn.Lowering:          false,
		learn.Complete:          false,
	}
	for phase, want := range waitingIn {
		rt := &Runtime{observations: newObservationRegistry(), learn: &learnSession{}}
		rt.learn.coord = learn.New("open target", &idlePasses{}, &stubTeachMemory{},
			learn.DefaultBounds())
		rt.learn.session = learn.Session{Name: "open target", Phase: phase}

		got := rt.Playbill(service.PlaybillPayload{}).LearnSession
		if got.Waiting != want {
			t.Errorf("phase %q publishes waiting=%v, want %v", phase, got.Waiting, want)
		}
		// And it agrees with the coordinator, which is the only owner of the distinction.
		if got.Waiting != phase.Waiting() {
			t.Errorf("phase %q: the account says waiting=%v and the coordinator says %v",
				phase, got.Waiting, phase.Waiting())
		}
	}
}

// A REVIEW CAN ALWAYS BE ENDED BY THE PERSON.
//
// # The live failure
//
// The route review walks legs one at a time and waits for an answer to each. A leg whose question
// was retracted left the panel reading:
//
//	step 1 of 2 … — waiting for your answer
//	questions open: 0   asking: NONE   can_stop=false  can_try=false  can_name=false
//
// Waiting for an answer to a question that did not exist, with nothing to press. The only control
// offered was Cancel, which throws the whole episode away — including the leg that had verified.
//
// Stop means here what it means during the demonstration: the person says they are done.
//
// Deleting CanStop from the reviewing stages must fail this.
func TestAReviewCanAlwaysBeEndedByThePerson(t *testing.T) {
	for _, stage := range []LearnStage{LearnFinishing, LearnTrying, LearnWaitingToTry} {
		v := learnView{Stage: stage}
		applyControls(&v, learn.Session{})
		if !v.CanStop {
			t.Errorf("at stage %q the person cannot say they are done. A review waiting on "+
				"a question nobody will raise leaves them with Cancel or nothing, and "+
				"Cancel discards the legs that worked.", stage)
		}
		if !v.CanCancel {
			t.Errorf("at stage %q there is no way out at all", stage)
		}
	}
}

// ── the stuck account: sentences one way, particulars the other ───────────────

// The projection reads the account through Notes, never off Diagnostics.
//
// This is the far end of the split the coordinator makes. Diagnostics is the RENDERED line, with
// the particulars glued onto the sentence — and gluing them is exactly what put a durable subject
// id in front of a person, in the red block of the Learn panel, in a sentence every surface was
// rendering faithfully. Notes hands over the two halves already separated, and the projection's
// job is to pass both on without re-joining them.
//
// The fixture carries BOTH lists, the way a real session does, so that a projection reading the
// wrong one still produces detail — and fails on what that detail contains rather than on there
// being none.
func TestTheStuckAccountShowsSentencesAndKeepsFactsApart(t *testing.T) {
	const (
		say = "I didn't recognise where this started, so I can't write down a way back to it"
		id  = "subj_a1b2c3"
	)
	note := learn.Note{Say: say, Facts: []learn.Fact{
		{Name: "verdict", Value: "different"},
		{Name: "established", Value: id},
	}}
	s := learn.Session{
		Phase:       learn.Refused,
		Refusal:     learn.DestinationNotRecognised,
		Account:     []learn.Note{note},
		Diagnostics: []string{note.Line()},
	}

	v := learnViewOf(s, true, false)

	if len(v.Detail) == 0 {
		t.Fatal("a refused session shows nothing about why it is stuck")
	}
	for _, d := range v.Detail {
		if strings.Contains(d, id) || strings.Contains(d, "established=") {
			t.Errorf("the detail a person reads carries a durable subject id: %q.\n"+
				"That is Diagnostics — the rendered line — being passed through instead "+
				"of the sentence half of the account.", d)
		}
	}
	if v.Detail[0] != say {
		t.Errorf("the sentence reached the panel as %q, want %q", v.Detail[0], say)
	}

	// THE FACTS ARE NOT DROPPED. They are how this failure is told apart from the others that
	// look like it, and an Advanced surface shows them under the line they explain — which is
	// why each one travels with the sentence it belongs to.
	want := map[string]string{"verdict": "different", "established": id}
	got := map[string]string{}
	for _, f := range v.Facts {
		got[f.Name] = f.Value
		if f.Say != say {
			t.Errorf("the fact %s=%s does not say which sentence it belongs to (%q); a "+
				"flat list with no owner leaves the surface guessing, or pushes the "+
				"grouping back into the projection", f.Name, f.Value, f.Say)
		}
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("the particular %s=%s never reached an Advanced surface: %+v",
				name, value, v.Facts)
		}
	}
}
