package observesession_test

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// One watched pass produces a candidate, stores it, and ends with a question a person can answer.
//
// # The defect this holds closed
//
// The one-shot candidate was built in the teaching COORDINATOR, which runs after the session has
// ended. Everything downstream of a demonstration reads the candidate STORE, and the proposal that
// grants rehearsal authority is raised from it during the session — so the candidate arrived after
// the only stage that could have asked about it. Live, teaching reached "I think I got it. Want me
// to try?" and waited for a grant that could never be created, because no `AskRehearse` proposal
// existed to answer.
//
// Authority is unchanged and must stay unchanged: a rehearsal grant comes from answering that
// question and from nothing else. What moved is WHERE the candidate is made, so the question gets
// asked at all.
//
// This enters through `Runner.Run`, deliberately. A helper-level test would prove the construction
// works and say nothing about the thing that was actually broken.

// licensed is a teach pass: the episode that may establish places.
func licensed() observesession.Config {
	cfg := foregroundConfig()
	cfg.Episode = observesession.Episode{SameEpisode: true, EstablishPlaces: true}
	return cfg
}

// oneMove is what a person actually shows Marco: they are on A, they press, they are on B.
//
// Deliberately NOT happyScript, which routes through an intermediate screen with no durable
// identity — that produces two session-local edges and no durable route, so there is nothing for a
// one-shot candidate to be about. A single direct change is the shape this path exists for.
func oneMove() []demoFrame {
	var out []demoFrame
	out = append(out, hold("a", 4)...)
	out = append(out, press("b", observe.NavDown, observe.NavDown, observe.NavConfirm))
	out = append(out, hold("b", 4)...)
	return out
}

// TestOneWatchedPassProducesACandidateAndAsksToTry is the production-path gate.
//
// Deleting the runner's construction, its store call, or moving it after the session must fail it.
func TestOneWatchedPassProducesACandidateAndAsksToTry(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	from, to := seedRelationshipIn(t, store, 3, strongEvidence())

	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: oneMove()}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)

	res, err := r.Run(context.Background(), licensed())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if res.Demonstration == nil {
		t.Fatal("a licensed pass watched a clean route and produced no demonstration.\n" +
			"Everything a candidate is made of was in what the pass recorded, and the person " +
			"would be asked to perform the identical route a second time for a capture to see " +
			"it properly.")
	}
	if !res.Demonstration.Complete {
		t.Fatalf("the candidate is incomplete: %s", res.Demonstration.Reason)
	}
	if got := res.Demonstration.Relationship; got.From != from || got.To != to {
		t.Errorf("the candidate names %+v, not the route the session made durable %s → %s",
			got, from, to)
	}

	// STORED, which is the half that was missing. Everything after a demonstration reads this.
	stored := store.Candidates(res.Session.Application)
	if len(stored) == 0 {
		t.Fatal("the candidate was built and never stored.\nThe assessment, the review and the " +
			"question that grants authority all read the candidate store; one that is not in " +
			"it is invisible to every stage that matters.")
	}

	// And the question exists, so a person has something to answer.
	if !asks(res, observe.AskRehearse) {
		t.Fatalf("no rehearsal question was raised.\nTeaching would reach \"want me to "+
			"try?\" and wait for a grant nobody could give, because authority comes from "+
			"answering this proposal and from nothing else.\nrehearsals: %+v\nproposals: %+v",
			res.Rehearsals, res.Proposals.Proposals)
	}
}

// An UNLICENSED session watching the same thing produces nothing.
//
// The licence is the difference between being shown something and being present while it happened.
// Without it, ordinary passive observation would start manufacturing demonstrations of whatever it
// saw — and each one would raise a question asking to act.
func TestAnUnlicensedPassProducesNoDemonstration(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	seedRelationshipIn(t, store, 3, strongEvidence())

	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: oneMove()}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)

	res, err := r.Run(context.Background(), foregroundConfig()) // zero Episode
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Demonstration != nil {
		t.Fatalf("an unlicensed session produced a demonstration: %+v.\nWatching somebody is "+
			"not the same as being shown something, and a candidate nobody asked for still "+
			"raises a question asking to act.", res.Demonstration)
	}
	if len(store.Candidates(res.Session.Application)) != 0 {
		t.Error("an unlicensed session wrote a candidate to the store")
	}
}

// The candidate alone is not authority.
//
// Storing one raises the QUESTION; it does not answer it. A grant exists only after somebody says
// yes, and this is the test that fails if the runner ever starts producing one.
func TestAStoredCandidateGrantsNothing(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	seedRelationshipIn(t, store, 3, strongEvidence())

	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: oneMove()}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)

	if _, err := r.Run(context.Background(), licensed()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if g := r.Grant(); g != nil && g.Active() {
		t.Fatalf("a session that stored a candidate created rehearsal authority by itself: "+
			"%+v.\nAuthority comes from a person answering AskRehearse and from nowhere else.",
			g)
	}
}

// asks reports whether the result carries an unanswered proposal of this kind.
func asks(res observesession.Result, kind observe.AskKind) bool {
	for _, p := range res.Proposals.Proposals {
		if p.Ask == kind && p.Response == observe.ResponseNone {
			return true
		}
	}
	return false
}

// When the runner declines to build a candidate it SAYS SO.
//
// Silence here cost a live run. The runner declined, Teach fell back to the armed capture, that
// failed too, and the person was told "that example didn't finish where I expected" — about an
// example nobody had ever tried to build. The reason existed and stopped at the function boundary,
// which is the third time in this subsystem a diagnosis has been lost exactly that way.
func TestADeclinedWatchedPassSaysWhy(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	seedRelationshipIn(t, store, 3, strongEvidence())

	// The change is seen, the navigation is not — so there is nothing to build a procedure
	// from, and the refusal names that rather than leaving a blank.
	silent := oneMove()
	for i := range silent {
		silent[i].inputs = nil
	}

	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: silent}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)

	res, err := r.Run(context.Background(), licensed())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Demonstration != nil {
		t.Fatalf("the fixture was supposed to produce no candidate: %+v", res.Demonstration)
	}
	if res.Watched == "" {
		t.Fatal("the runner declined to build a demonstration and reported no reason.\n" +
			"Teach then falls back silently, and a person is told the example did not finish " +
			"when nothing ever tried to build one.")
	}
}

// A session that WAS licensed and DID build one reports no refusal.
func TestASuccessfulWatchedPassReportsNoRefusal(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	seedRelationshipIn(t, store, 3, strongEvidence())

	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: oneMove()}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)

	res, _ := r.Run(context.Background(), licensed())
	if res.Demonstration == nil {
		t.Fatalf("no candidate was built: %s", res.Watched)
	}
	if res.Watched != "" {
		t.Errorf("a successful build reported the refusal %q", res.Watched)
	}
}

// A LICENSED PASS WATCHES THE WHOLE WALK.
//
// # What this replaces, and why
//
// A licensed pass used to end itself once one settled screen differed from the one it opened on
// and input had been quiet for about two seconds. The test here asserted exactly that, and the
// behaviour was wrong for the thing Learn now exists to do.
//
// A pause is not an ending. The pause between legs of a walk is precisely when somebody is
// looking for the next thing to click. Measured on the run that could not learn
// Home → Bluetooth & devices → Mouse: the demonstration pass ended 3.6 seconds into a 45 second
// budget after 8 samples, and no session in the whole episode ever contained the Mouse
// composition. The second edge was never lost — its destination was never observed.
//
// An earlier attempt had already raised this from "the first settled screen" to two seconds,
// carrying a comment describing this very failure. Raising it again is guessing at how long a
// person takes to find a row.
//
// The panel says "Press Stop Learning when you are done." That is the person's own statement of
// intent, and it is the only trustworthy one.
//
// Reinstating an arrival-based end must fail this.
func TestALicensedPassWatchesTheWholeWalk(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	seedRelationshipIn(t, store, 3, strongEvidence())

	// A → B, a pause long enough to read as "stopped", then B → C. The pause is the whole
	// point: under the old rule the pass ended inside it and C was never seen.
	var walk []demoFrame
	walk = append(walk, hold("a", 4)...)
	walk = append(walk, press("b", observe.NavDown, observe.NavConfirm))
	walk = append(walk, hold("b", 12)...)
	walk = append(walk, press("c", observe.NavDown, observe.NavConfirm))
	walk = append(walk, hold("c", 6)...)

	s := &demoSampler{script: walk}
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)}, s,
		&recordingEvents{}).WithMemory(store).WithCandidates(store)
	res, err := r.Run(context.Background(), licensed())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// THE TERMINAL SCREEN WAS OBSERVED. Not "a route was found" — the specific screen the
	// walk ended on has to exist in the session's own model, or nothing downstream can
	// establish it, cross to it or build an edge into it.
	seen := 0
	for _, st := range res.Stats.Shadow.States {
		if st.Inferences > 0 {
			seen++
		}
	}
	if seen < 3 {
		t.Fatalf("the pass observed %d screen(s) of a three-screen walk in %d samples of a "+
			"%d-frame script. It stopped watching while the person was still walking, so the "+
			"screen they ended on was never observed — and every edge into it is unlearnable.",
			seen, res.Stats.SamplesTaken, len(walk))
	}
}

// A pass where nobody moves still runs to its bound and refuses honestly.
//
// The control. Early exit must be about evidence arriving, not about finishing sooner: a session
// that saw nothing has learned nothing, and cutting it short would turn "you did not show me
// anything" into "I stopped watching".
func TestAPassWhereNothingHappensStillRunsItsCourse(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	seedRelationshipIn(t, store, 3, strongEvidence())

	still := hold("a", 30)
	sampler := &demoSampler{script: still}
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)}, sampler, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)

	res, err := r.Run(context.Background(), licensed())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Demonstration != nil {
		t.Fatal("a session in which nobody moved produced a demonstration")
	}
	if res.Stats.SamplesTaken < len(still) {
		t.Errorf("the pass stopped after %d of %d samples without anything having been "+
			"shown; settling on the STARTING screen is not a demonstration",
			res.Stats.SamplesTaken, len(still))
	}
}

// An UNLICENSED pass is never cut short. Ordinary observation is not waiting for anything.
func TestAnUnlicensedPassIsNotEndedEarly(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	seedRelationshipIn(t, store, 3, strongEvidence())

	script := oneMove()
	script = append(script, hold("b", 40)...)
	sampler := &demoSampler{script: script}
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)}, sampler, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)

	res, err := r.Run(context.Background(), foregroundConfig()) // zero Episode
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stats.SamplesTaken < len(script) {
		t.Errorf("an ordinary observation session stopped after %d of %d samples because the "+
			"screen changed; passive watching has no goal to satisfy and cutting it short "+
			"would silently shorten every session", res.Stats.SamplesTaken, len(script))
	}
}
