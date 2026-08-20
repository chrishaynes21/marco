package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// The synthetic Marco Moment.
//
// One deterministic lifecycle, driven through the PRODUCTION registry, watched through
// the PRODUCTION visibility read at every step. What it proves is not that the sentences
// are nice — it is that the human-visible story corresponds to the same underlying states
// the product uses, and that the story changes when and only when the Director's state
// does.
//
// Nothing here fakes a stage to make the demo prettier. Where the lifecycle cannot be
// observed without an invasive hook, the test says so and the report says so — see
// TestTheStagesThisSurfaceCannotSeeAreNamed at the bottom, which is deliberately a list
// of what is MISSING.

// moment is one step of the lifecycle: what happened, and what a person then saw.
type moment struct {
	step string
	view playbill.View
	text string
}

func (m moment) String() string { return m.step + "\n" + m.text }

// TestTheSyntheticMarcoMoment walks the lifecycle and checks the story at each stage.
func TestTheSyntheticMarcoMoment(t *testing.T) {
	restore := sessionClock
	sessionClock = newDryClock()
	t.Cleanup(func() { sessionClock = restore })

	store, _ := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	g := newObservationRegistry()
	g.memory = store
	seedDryRoute(t, store)

	rt := testRuntime(t)
	rt.observations = g
	look := func(step string) moment {
		t.Helper()
		v := rt.Playbill(service.PlaybillPayload{}).Normalise()
		if err := v.Admit(); err != nil {
			t.Fatalf("%s: the account failed its own guard: %v", step, err)
		}
		return moment{step: step, view: v, text: renderWatch(v.Watch())}
	}

	var story []moment

	// ── 1. observing, and recognising a screen it has seen before ──
	id := observeOnce(t, g, dryHold("a", 8))
	story = append(story, look("observed a familiar screen"))
	if story[0].view.Current.Recognition == playbill.Unobservable {
		t.Fatalf("a session that saw a remembered screen reported it unobservable:\n%s",
			story[0].text)
	}

	// ── 2. Marco asks whether to learn a habit it has watched ──
	asked := look("a question is open")
	if asked.view.Question == nil {
		t.Fatalf("no question reached the surface. Marco asked the user something and "+
			"a person watching would not know:\n%s", asked.text)
	}
	if asked.view.Question.Via != playbill.ViaProposal {
		t.Fatalf("the question named the response path %q", asked.view.Question.Via)
	}
	if !asked.view.Normal().Attention {
		t.Error("an open question did not pull the consumer surface forward")
	}
	story = append(story, asked)

	// ── 3. the answer, through the ordinary path ──
	sayYes(t, g, id, observe.AskLearnRelationship)
	story = append(story, look("the user said yes"))

	// ── 4. the demonstration ──
	observeOnce(t, g, aToB())
	story = append(story, look("the user demonstrated it"))

	// ── 5. the one example is enough to be worth trying ──
	//
	// The story used to have a second demonstration here, answered through
	// `provide_second_demonstration`. It does not any more: after one clean example the
	// follow-up is not eligible, because Marco trying the thing settles what the person
	// performing it twice only repeats. [[ADR-051-one-demonstration-and-an-attempt]]
	counted := look("the surface knows what it has seen")
	if counted.view.Learning.Examples != 1 {
		t.Errorf("the surface reports %d example(s), want the one that was given",
			counted.view.Learning.Examples)
	}
	story = append(story, counted)

	// ── 6. Marco asks to try it, and is authorised ──
	id = observeOnce(t, g, dryHold("a", 8))
	offered := look("Marco asked to try it")
	story = append(story, offered)

	sayYes(t, g, id, observe.AskRehearse)
	if g.last.Grant() == nil {
		t.Fatal("the lifecycle produced no authorization, so there is nothing to rehearse")
	}
	authorised := look("permission was given")
	if authorised.view.Learning.Stage != playbill.RehearsalOffered {
		t.Fatalf("an issued grant did not reach the surface: %q",
			authorised.view.Learning.Stage)
	}
	if !strings.Contains(authorised.text, "I can try this once") {
		t.Errorf("the authorization did not read as one:\n%s", authorised.text)
	}
	story = append(story, authorised)

	// ── 7. the rehearsal completed, and the play can be written down ──
	grant := g.last.Grant()
	j, ok := g.judgeNow("testgame", grant.Relationship)
	if !ok {
		t.Fatal("no judgement for the authorized route")
	}
	g.rememberRehearsal("testgame", j, rehearse.RehearsalResult{
		Relationship: grant.Relationship, Source: grant.Source,
		Destination: grant.Destination, Evidence: j.Digest, Live: true,
		Terminal: rehearse.CompletedRoute, StepsTaken: 1, Inputs: 1,
		Steps: []rehearse.StepRecord{{Outcome: rehearse.DirectlyVerified}},
	})
	// The grant is spent. A rehearsal is single-use, and the account must stop offering
	// one the moment it has been taken up.
	g.last.RevokeRehearsal()
	verified := look("the rehearsal reached the screen it expected")
	story = append(story, verified)

	// ── what the story has to be ──
	//
	// Every step must have SAID something, and the story must have moved. A surface that
	// rendered the same thing through a whole lifecycle would be a surface nobody could
	// use to debug the lifecycle.
	seen := map[string]bool{}
	for _, m := range story {
		if strings.TrimSpace(m.text) == "" {
			t.Fatalf("%s: the surface said nothing at all", m.step)
		}
		seen[m.view.WithDigest().Digest] = true
	}
	if len(seen) < 4 {
		t.Fatalf("the whole lifecycle produced only %d distinct accounts. A person "+
			"watching would see a panel that barely moves while Marco learns", len(seen))
	}

	// The end of the lifecycle in THIS fixture is the honest one: the route was tried and
	// it worked, and Marco still cannot write it down because nobody has said what the
	// screen is called. That is not a shortcoming of the fixture — it is ADR-031 working,
	// and it is exactly the kind of thing this surface exists to make visible.
	//
	// The test asserts the state AND its explanation, because the state alone would leave
	// a person staring at "I tried it" wondering why no play appeared.
	if verified.view.Learning.Stage != playbill.Rehearsed {
		t.Errorf("a rehearsed route did not read as rehearsed: %q\n%s",
			verified.view.Learning.Stage, verified.text)
	}
	if !strings.Contains(verified.text, "I don't know what you call the screen") {
		t.Errorf("the surface did not say WHY no play appeared. A person would see a "+
			"lifecycle that finished and produced nothing:\n%s", verified.text)
	}

	// And once the screens have names, the play becomes available — through the same
	// read, with no other change.
	nameEverySubject(t, g, "the pause menu")
	writable := look("the user named the screens")
	if writable.view.Learning.Stage != playbill.PlayAvailable {
		t.Errorf("naming the screens did not unblock the play: %q\n%s",
			writable.view.Learning.Stage, writable.text)
	}
	if !strings.Contains(writable.text, "write this down") {
		t.Errorf("the end of the lifecycle did not read as one:\n%s", writable.text)
	}
	story = append(story, writable)

	// And nowhere in the whole story did an implementation identifier reach a person.
	for _, m := range story {
		for _, bad := range []string{"state_", "subj_", "q_", "attempt_", "shadow_",
			"possible_", "screen_state", "_like_state"} {
			if strings.Contains(m.text, bad) {
				t.Errorf("%s leaked %q to a person:\n%s", m.step, bad, m.text)
			}
		}
	}

	if testing.Verbose() {
		for _, m := range story {
			t.Logf("\n══ %s ══\n%s", m.step, m.text)
		}
	}
}

// The rehearsal ATTEMPT itself is not observable from here, and saying so is the point.
//
// A grant moves issued → consumed the moment an attempt begins, and the attempt runs
// inside `director rehearse` rather than inside anything the visibility read can see. So
// the account can say "you said yes — I can try this once" and "I tried it", and it cannot
// currently say "performing step 1 of 2" for a rehearsal in flight.
//
// This test exists so that gap is written down as a fact rather than discovered by
// somebody watching a panel that never showed them the interesting part. It will start
// failing the day a rehearsal publishes progress — which is the correct time to delete it.
func TestTheStagesThisSurfaceCannotSeeAreNamed(t *testing.T) {
	g := authorizedRegistry(t)
	rt := testRuntime(t)
	rt.observations = g

	v := rt.Playbill(service.PlaybillPayload{}).Normalise()
	if v.Learning.Stage == playbill.Rehearsing && v.Doing.Step > 0 {
		t.Fatal("a rehearsal in flight now reports its step position. Delete this test " +
			"and the 'what remains invisible' note that goes with it")
	}
	if v.Learning.Stage != playbill.RehearsalOffered {
		t.Fatalf("an issued grant should read as an offer taken up: %q", v.Learning.Stage)
	}
}
