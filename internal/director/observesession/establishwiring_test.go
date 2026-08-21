package observesession_test

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// The place bootstrap, entered through the production session path.
//
// # The blocker this exists to close
//
// `learn "…"` refused at its first step. Establishing a START goes PlaceNow → SignatureOfState →
// Recall, and Recall could only ever succeed against a durable subject — which was written ONLY
// when a person answered a semantic proposal. Passive observation formed hypotheses and persisted
// nothing, so Marco could not learn anything until they had happened to answer an
// incidental "is this a menu?" about the right screen. Which question Marco raised was not theirs
// to choose: observed live, the same application asked about the screen in one session and about
// a group inside it in another, and only the first would have unblocked Learn.
//
// # What is being proved, and what must NOT be
//
// Proved: an explicitly licensed session makes the place the user is standing on durably
// recognisable, across a restart, with NOBODY having answered anything.
//
// Not proved and deliberately not true: that observation persists places. The negative control
// below runs the identical evidence through an unlicensed session and requires an empty store.
// If that ever passes vacuously the positive test would too, so they share one fixture.

// establishing runs one licensed pass over a store and returns its result.
func establishing(t *testing.T, m observe.Memory, p observe.PlaceStore,
	s observesession.Sampler) observesession.Result {

	t.Helper()
	return passOver(t, m, p, s, observesession.Episode{Licence: observesession.LearnLicence()})
}

// passOver runs one session, wiring exactly what the caller supplied.
//
// `p` is passed separately from `m` although production hands the same store to both, because the
// mutation this test exists to catch is somebody deleting one of the two wirings — and a helper
// that derived one from the other could not tell them apart.
func passOver(t *testing.T, m observe.Memory, p observe.PlaceStore, s observesession.Sampler,
	ep observesession.Episode) observesession.Result {

	t.Helper()
	cfg := config()
	cfg.Episode = ep
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)}, s, &recordingEvents{}).
		WithMemory(m)
	if p != nil {
		r = r.WithPlaces(p)
	}
	got, err := r.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// NOBODY answers anything, anywhere in this file. A single Respond would make every
	// assertion below meaningless, because that is the path that already worked.
	for _, p := range r.Proposals().Proposals {
		if p.Response.Known() {
			t.Fatalf("question %s was answered %q during a pass; this test can then say "+
				"nothing about establishing a place WITHOUT a semantic answer",
				p.ID, p.Response)
		}
	}
	return got
}

// screenSubjects is the durable screen subjects a store holds.
func screenSubjects(s *semanticmemory.Store) []observe.RememberedSubject {
	var out []observe.RememberedSubject
	for _, r := range s.Subjects() {
		if r.Structure.Subject == observe.SubjectState {
			out = append(out, r)
		}
	}
	return out
}

// ── the positive: Learn establishes a place nobody was asked about ────────────

// THE bootstrap test. A licensed pass makes where the user is standing durably recognisable.
//
// Deleting the establishPlace call from Runner.Run must fail this, and so must deleting the
// WithPlaces wiring.
func TestLearningEstablishesTheStartThroughTheProductionPath(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)

	got := establishing(t, store, store, &topologySampler{})

	if !got.Places.Licensed {
		t.Fatal("a pass configured to establish places reported itself unlicensed")
	}
	if !got.Places.Established() {
		t.Fatalf("no place was established (reason=%q). The user has explicitly asked Marco to "+
			"learn something and Marco still cannot remember where they are standing, which "+
			"is the whole blocker", got.Places.Reason)
	}

	// THE cold restart. A genuinely new process's view of the file.
	reopened := memoryAt(t, dir)
	subjects := screenSubjects(reopened)
	// EVERY place the pass settled on, and nothing else.
	//
	// This asserted "exactly one" until 2026-08-17, which was ADR-047's cap stated
	// directly. The cap made a route with a middle step unlearnable — the intermediate
	// place never became durable, so the edges either side of it had an unresolvable
	// endpoint — and it made ADR-056's multi-edge decomposition unreachable in practice.
	// See [[ADR-063-a-pass-remembers-every-place-it-settled-on]].
	//
	// What replaced it is not "more": it is the per-place GATES, which is what the cap was
	// standing in for. This fixture settles on one place, so it still establishes one —
	// and the count is asserted against what the pass actually settled on rather than
	// against a constant, so the test says the rule rather than the number.
	if len(subjects) != len(got.Places.Also)+1 {
		t.Fatalf("the store holds %d screen subject(s); the pass reported 1 current place "+
			"and %d others, so they disagree about what was established",
			len(subjects), len(got.Places.Also))
	}
	found := false
	for _, s := range subjects {
		if s.ID == got.Places.Subject {
			found = true
		}
	}
	if !found {
		t.Errorf("the session reported subject %q and the file does not hold it",
			got.Places.Subject)
	}

	// ZERO semantic judgements. This is the half that makes the mechanism honest: Marco
	// remembers the place and claims nothing whatever about what it means.
	if n := len(subjects[0].Knowledge); n != 0 {
		t.Errorf("the established place carries %d interpretation(s): %+v.\nNobody was asked "+
			"anything, so nothing may be recorded about what this screen is",
			n, subjects[0].Knowledge)
	}
	if subjects[0].Called != "" {
		t.Errorf("the established place is called %q; no person named it", subjects[0].Called)
	}

	// And it is RECOGNISED again, through the canonical identity path, from the reopened file.
	sig, ok := observe.SignatureOfState(got.Stats.Shadow, got.Stats.Shadow.CurrentState,
		observe.DefaultHypothesisThresholds())
	if !ok {
		t.Fatal("the pass's own current state has no signature; nothing above could be checked")
	}
	rec := reopened.Recall(got.Session.Application, sig)
	if !rec.Verdict.Established() {
		t.Fatalf("a place established in the previous session recalls as %q after a restart. "+
			"Cold-restart recognition is the entire point of making it durable", rec.Verdict)
	}
	if rec.Subject.ID != got.Places.Subject {
		t.Errorf("recall resolved to %q, not the subject that was established (%q)",
			rec.Subject.ID, got.Places.Subject)
	}
}

// A start established with no semantic answer is not mistaken for one somebody confirmed.
//
// The failure this guards is the worst available: a place remembered because the user asked
// Marco to learn, later read as a place the user agreed was a settings screen. Every consumer
// already handles an empty interpretation list; this holds them to it.
func TestAnEstablishedPlaceCarriesNoUserJudgement(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	establishing(t, store, store, &topologySampler{})

	reopened := memoryAt(t, dir)
	for _, s := range screenSubjects(reopened) {
		for _, k := range s.Knowledge {
			if k.Active() {
				t.Errorf("%s carries an active judgement %q that nobody gave", s.ID, k.Status)
			}
		}
		for _, kind := range []observe.HypothesisKind{
			observe.PossibleMenuLikeState,
			observe.PossibleSettingsLikeState,
			observe.PossibleTextEntryState,
		} {
			if _, ok := observe.RecalledValidation(
				observe.Recollection{Verdict: observe.MatchSame, Subject: s}, kind); ok {
				t.Errorf("%s produced user validation for %s. Marco would then present its "+
					"own bookkeeping back to the user as something they had settled", s.ID, kind)
			}
		}
	}
}

// ── the negative control: ordinary observation still persists nothing ─────────

// The identical evidence, unlicensed, writes nothing.
//
// Deleting the `cfg.EstablishPlaces` check in establishPlace must fail this. Without it the
// positive test above would be satisfied by a Director that remembered every screen anybody ever
// looked at, which is precisely what bounded memory forbids.
func TestAnOrdinarySessionEstablishesNoPlace(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)

	got := passOver(t, store, store, &topologySampler{}, observesession.Episode{})

	if got.Places.Licensed || got.Places.Established() {
		t.Fatalf("an ordinary observation session established %q. Watching somebody play is "+
			"not them asking Marco to learn", got.Places.Subject)
	}
	if got.Places.Reason != observe.PlaceNotLicensed {
		t.Errorf("reason %q, want %q — a reader has to be able to tell a missing licence "+
			"from a screen with nothing to recognise it by",
			got.Places.Reason, observe.PlaceNotLicensed)
	}
	if n := memoryAt(t, dir).Count(); n != 0 {
		t.Fatalf("%d subject(s) were written by a session nobody answered a question in. "+
			"Memory is bounded by what a person settled, and this would make it bounded by "+
			"how long Marco was left running", n)
	}
}

// A Director with no place store is unchanged, and says so.
func TestALicensedPassWithNowhereToWriteSaysSo(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)

	got := passOver(t, store, nil, &topologySampler{},
		observesession.Episode{Licence: observesession.LearnLicence()})

	if got.Places.Established() {
		t.Fatal("a place was established with no place store wired")
	}
	if got.Places.Reason != observe.PlaceNoMemory {
		t.Errorf("reason %q, want %q", got.Places.Reason, observe.PlaceNoMemory)
	}
}

// ── the destination, and the edge that only becomes writable because of it ────

// endOnTheDestination shifts the topology fixture so a pass STOPS on its second screen.
//
// A learn pass ends where the user is standing, and the destination is established from that.
// The fixture's cycle is 8 phases and a pass takes 51 samples, so the shift that leaves the
// second screen up on the last sample has to be stated rather than hoped for — a fixture that
// happened to end on the FIRST screen would establish nothing new and the route test below would
// fail for a reason that has nothing to do with the mechanism.
const endOnTheDestination = 6

// THE requirement, end to end: two places and a durable route between them, with no answers.
//
// This is the shape of a real learn attempt — a pass to establish the start, then a pass in which
// the user navigates — and it is the one that shows WHY the establishment happens where it does.
// A durable edge needs both endpoints resolvable at the moment the topology is folded, so a
// destination established after that fold would be in the store and still rejected, and the user
// would be told their destination was not recognised.
func TestATaughtRouteBecomesDurableWithoutASemanticAnswer(t *testing.T) {
	dir := t.TempDir()

	// Pass one: hold still. The start becomes recognisable.
	start := establishing(t, memoryAt(t, dir), memoryAt(t, dir), &topologySampler{})
	if !start.Places.Established() {
		t.Fatalf("no start was established (reason=%q)", start.Places.Reason)
	}

	// Pass two: the user navigates. The sampler is REVERSED, so this pass mints its
	// session-local identities in a different order — which is what a separate sitting does —
	// and shifted so it STOPS on the second screen, because the destination is established
	// from the place the user is standing on when the pass ends.
	store := memoryAt(t, dir)
	dest := establishing(t, store, store,
		&topologySampler{reversed: true, offset: endOnTheDestination})
	// The property is that BOTH endpoints end up durable, not which pass did it.
	//
	// Since 2026-08-17 a pass establishes every place it settled on, so the first pass
	// may already have made the destination recognisable and the second correctly reports
	// `already_known` — which is success, not refusal. Asserting the pass attribution
	// would be asserting the old cap through the back door.
	// See [[ADR-063-a-pass-remembers-every-place-it-settled-on]].
	if !dest.Places.Established() && dest.Places.Reason != observe.PlaceAlreadyKnown {
		t.Fatalf("no destination was established (reason=%q)", dest.Places.Reason)
	}

	reopened := memoryAt(t, dir)
	if n := len(screenSubjects(reopened)); n != 2 {
		t.Fatalf("the store holds %d screen subject(s), want 2", n)
	}
	for _, s := range screenSubjects(reopened) {
		if len(s.Knowledge) != 0 {
			t.Errorf("%s carries %d interpretation(s) and nobody was asked anything",
				s.ID, len(s.Knowledge))
		}
	}

	rels := reopened.Relationships()
	if len(rels) == 0 {
		t.Fatalf("no durable route was written although both of its endpoints are "+
			"recognisable (%d transitions were seen, %d stayed session-local). LearnSession "+
			"would refuse with destination_not_recognised",
			dest.Relationships.Durable+dest.Relationships.SessionLocal,
			dest.Relationships.SessionLocal)
	}
	// The endpoints really are the established places, not something else that happened to
	// resolve.
	//
	// Read from the STORE rather than from the passes' own reports: which pass established
	// which place is no longer fixed, and asserting it would be asserting the old
	// one-per-pass cap through the back door.
	known := map[string]bool{}
	for _, s := range screenSubjects(reopened) {
		known[s.ID] = true
	}
	if !known[start.Places.Subject] {
		t.Errorf("the start %q is not in the store", start.Places.Subject)
	}
	for _, r := range rels {
		if !known[r.From] || !known[r.To] {
			t.Errorf("edge %s → %s does not connect two established places", r.From, r.To)
		}
	}
}

// The same two passes, unlicensed, produce no route at all.
//
// The control for the test above. Without it, a fixture that had somehow started answering its
// own questions would make the positive result look like the mechanism working.
func TestWithoutTheLicenceNoRouteBecomesDurable(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	passOver(t, store, store, &topologySampler{}, observesession.Episode{})
	passOver(t, store, store, &topologySampler{reversed: true}, observesession.Episode{})

	reopened := memoryAt(t, dir)
	if n := reopened.Count(); n != 0 {
		t.Errorf("%d subject(s) written with no licence and no answers", n)
	}
	if n := len(reopened.Relationships()); n != 0 {
		t.Errorf("%d durable route(s) written with no licence and no answers", n)
	}
}

// EVERY place a licensed pass settled on becomes durable, not only the one it ended at.
//
// THE live blocker of 2026-08-17. A person demonstrated `Settings Home → Bluetooth & devices
// → Mouse` on a cold store. All three screens settled and all three carried terms, so all
// three passed every quality gate — and only Mouse was established, because establishment
// considered `CurrentState` alone. That left both edges with an unresolvable endpoint:
//
//	Home → Bluetooth    destination_unresolved
//	Bluetooth → Mouse   source_unresolved
//
// so a demonstration that was captured, attributed and semantically resolved end to end
// still could not be learned. It also made ADR-056's multi-edge decomposition unreachable:
// A → B → C yields two reusable edges only if B is durable.
//
// The intermediate place is the one this test is about; the ending place was never in doubt.
func TestEveryPlaceOnTheRouteBecomesDurable(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	// A walk that passes THROUGH one screen and stops on another.
	got := establishing(t, store, store, &topologySampler{})

	subjects := screenSubjects(memoryAt(t, dir))
	if len(subjects) < 2 {
		t.Fatalf("the store holds %d screen subject(s) after a walk through two settled "+
			"places, want at least 2.\nAn intermediate place that never becomes durable "+
			"leaves the edges either side of it with an endpoint nothing can resolve, and "+
			"the demonstration cannot be learned however well it was captured.",
			len(subjects))
	}
	if !got.Places.Established() {
		t.Errorf("the place the pass ended on was not established (reason=%q)",
			got.Places.Reason)
	}
	if len(got.Places.Also) == 0 {
		t.Error("the pass reported no other places; the walk went through one")
	}
	// And the report agrees with the file, so a reader is not told about a place that is
	// not there.
	held := map[string]bool{}
	for _, s := range subjects {
		held[s.ID] = true
	}
	for _, id := range got.Places.Also {
		if !held[id] {
			t.Errorf("the pass reported establishing %q and the store does not hold it", id)
		}
	}
	// STILL no judgements. Widening WHICH places are remembered does not widen what is
	// claimed about any of them.
	for _, s := range subjects {
		if len(s.Knowledge) != 0 {
			t.Errorf("%s carries %d interpretation(s) and nobody was asked anything",
				s.ID, len(s.Knowledge))
		}
	}
}

// An UNLICENSED pass still establishes nothing, however many places it settles on.
//
// The control, and the guarantee the widening must not touch: passive observation persists
// no subjects at all. Widening the count without this test would be widening the licence.
func TestAnUnlicensedPassEstablishesNoPlaceHowManyItSees(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)}, &topologySampler{},
		&recordingEvents{}).WithMemory(store).WithPlaces(store)

	res, err := r.Run(context.Background(), foregroundConfig()) // zero Episode
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Places.Licensed {
		t.Fatal("an ordinary session reported itself licensed to establish places")
	}
	if n := len(screenSubjects(memoryAt(t, dir))); n != 0 {
		t.Fatalf("an unlicensed pass established %d place(s); passive observation persists "+
			"no subjects", n)
	}
}
