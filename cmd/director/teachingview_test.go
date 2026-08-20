package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// The teaching session reaching every surface, through the production account.
//
// One value is published — `pkg/playbill.View` — and Normal, Watch and Diagnostics are three
// readings of it. The overlay imports that type and nothing else, so a fact that does not reach
// the account reaches no screen at all. This is where the wire is held.

// ── the wire ──────────────────────────────────────────────────────────────────

// TestTheTeachingSectionReachesEverySurface enters at Runtime.Playbill, which is the only path
// from the Director to a presentation. Deleting the `v.Teaching = r.teachingNow()` line must
// fail this.
func TestTheTeachingSectionReachesEverySurface(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry(), teach: &teaching{}}

	// A teach session, established through the coordinator rather than assembled here.
	c := teach.New("open target", &idlePasses{}, &stubTeachMemory{}, teach.DefaultBounds())
	rt.teach.coord, rt.teach.session, rt.teach.active = c, c.Session(), true

	v := rt.Playbill(service.PlaybillPayload{})
	if !v.Teaching.Active {
		t.Fatal("a running teach session does not reach the account; every surface would " +
			"show a Director watching for no reason")
	}
	if v.Teaching.Asked != "open target" {
		t.Errorf("the account says the user asked for %q", v.Teaching.Asked)
	}
	if len(v.Teaching.Progress) == 0 {
		t.Error("the account carries no checklist, so no surface can draw one")
	}
	// And it survives the admission guard, which is what a presentation actually receives.
	if err := v.Normalise().Admit(); err != nil {
		t.Fatalf("the teaching section was refused by the privacy guard: %v", err)
	}
	// All three readings see it.
	if h := v.Normalise().Normal(); h.Word != "Teaching" {
		t.Errorf("Normal reads %q, want Teaching", h.Word)
	}
	if !strings.Contains(watchTextOf(v), "TEACHING") {
		t.Error("Watch shows no teaching section")
	}
}

// TestAnIdleDirectorPublishesNoTeachingSection holds the other side: the wire must not invent
// a session where none exists.
func TestAnIdleDirectorPublishesNoTeachingSection(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry(), teach: &teaching{}}
	v := rt.Playbill(service.PlaybillPayload{})
	if v.Teaching.Active || v.Teaching.Asked != "" {
		t.Fatalf("an idle Director publishes a teaching session: %+v", v.Teaching)
	}
	if strings.Contains(watchTextOf(v), "TEACHING") {
		t.Error("an idle Director shows a teaching section")
	}
}

// ── armed is read from the coordinator, never decided here ────────────────────

// The cue must come from the phase the coordinator is actually in. A surface that decided for
// itself when to say SHOW ME would be a second opinion about whether Marco is watching.
func TestTheCueIsReadFromTheCoordinatorsOwnPhase(t *testing.T) {
	armedIn := map[teach.Phase]bool{
		teach.EstablishingStart:       false,
		teach.ReadyForDemo:            true,
		teach.Capturing:               true,
		teach.EstablishingDestination: false,
		teach.Evaluating:              false,
		teach.NeedsAnotherExample:     false,
		teach.ReadyToRehearse:         false,
		teach.Rehearsing:              false,
		teach.Naming:                  false,
		teach.Lowering:                false,
		teach.Complete:                false,
		teach.Refused:                 false,
		teach.Cancelled:               false,
	}
	for phase, want := range armedIn {
		rt := &Runtime{observations: newObservationRegistry(), teach: &teaching{}}
		rt.teach.coord = teach.New("open target", &idlePasses{}, &stubTeachMemory{},
			teach.DefaultBounds())
		rt.teach.session = teach.Session{Name: "open target", Phase: phase}
		rt.teach.active = !phase.Settled()

		got := rt.Playbill(service.PlaybillPayload{}).Teaching
		if got.Armed != want {
			t.Errorf("phase %q publishes armed=%v, want %v", phase, got.Armed, want)
		}
		// Every phase must produce an account the guard admits. A phase nobody can render
		// is a phase a person is stranded in.
		v := playbill.View{Reach: playbill.Present, Teaching: got}
		if err := v.Normalise().Admit(); err != nil {
			t.Errorf("phase %q produced an inadmissible account: %v", phase, err)
		}
	}
}

// ── completion comes from the artifact ────────────────────────────────────────

func TestThePublishedAccountSaysLearnedOnlyWhenAPlayExists(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry(), teach: &teaching{}}
	rt.teach.coord = teach.New("open target", &idlePasses{}, &stubTeachMemory{},
		teach.DefaultBounds())

	// Complete, and nothing written. This is the state a bug produces, and the account
	// must not dress it up.
	rt.teach.session = teach.Session{Name: "open target", Phase: teach.Complete}
	if got := rt.Playbill(service.PlaybillPayload{}).Teaching; got.Learned != "" {
		t.Fatalf("the account names a play %q that was never saved", got.Learned)
	}

	rt.teach.session.Saved = &teach.Saved{Name: "open target", Saved: true}
	got := rt.Playbill(service.PlaybillPayload{}).Teaching
	if got.Learned != "open target" {
		t.Fatalf("a saved play is not published: %+v", got)
	}
	if got.Registered {
		t.Error("the account reports a saved play as registered")
	}
}

// ── what you did comes from the demonstration ─────────────────────────────────

func TestWhatYouDidIsReadFromTheDemonstrationAndNeverFromTiming(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry(), teach: &teaching{}}
	rt.teach.coord = teach.New("open target", &idlePasses{}, &stubTeachMemory{},
		teach.DefaultBounds())

	// A completed demonstration whose steps carry no navigation. The screen changed and
	// nothing was attributed — the honest failure, and it must be published AS one.
	rt.teach.session = teach.Session{
		Name: "open target", Phase: teach.Evaluating,
		Demonstration: &observe.ProcedureCandidate{
			Complete: true,
			Steps: []observe.DemonstrationStep{{
				Arrived: observe.Checkpoint{Subject: "subj_end"},
			}},
		},
	}
	got := rt.Playbill(service.PlaybillPayload{}).Teaching
	if len(got.Did) != 0 || !got.Unattributed {
		t.Fatalf("an unattributed change published %+v; it must say it could not tell",
			got)
	}

	// And with navigation, the ordered meanings — never a key.
	rt.teach.session.Demonstration.Steps[0].Intents = []observe.NavIntent{
		observe.NavDown, observe.NavConfirm}
	got = rt.Playbill(service.PlaybillPayload{}).Teaching
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
	rt := &Runtime{observations: newObservationRegistry(), teach: &teaching{}}
	c := teach.New("open target", &idlePasses{}, &stubTeachMemory{}, teach.DefaultBounds())
	rt.teach.coord, rt.teach.session, rt.teach.active = c, c.Session(), true

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
		t.Fatalf("reading the account moved the teach session from %+v to %+v", before, after)
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
	waitingIn := map[teach.Phase]bool{
		teach.EstablishingStart: false,
		teach.ReadyForDemo:      false,
		teach.Capturing:         false,
		teach.Evaluating:        false,
		teach.ReadyToRehearse:   true,
		teach.Rehearsing:        false,
		teach.Naming:            true,
		teach.Lowering:          false,
		teach.Complete:          false,
	}
	for phase, want := range waitingIn {
		rt := &Runtime{observations: newObservationRegistry(), teach: &teaching{}}
		rt.teach.coord = teach.New("open target", &idlePasses{}, &stubTeachMemory{},
			teach.DefaultBounds())
		rt.teach.session = teach.Session{Name: "open target", Phase: phase}

		got := rt.Playbill(service.PlaybillPayload{}).Teaching
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
		applyControls(&v, teach.Session{})
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
