package observesession_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// From a realistic transition to what Director actually concludes.
//
// Parts 7, 8, 12 and 13: does a transition between two real-shaped screens reach the
// relationship machinery, is recurrence counted across REAL session boundaries, and what does
// the existing policy make of agreeing and disagreeing evidence.
//
// Nothing here creates a relationship, answers on the machinery's behalf, or tunes a threshold.
// The output that matters is what Director concluded, whatever that turns out to be.

// visit is one deterministic sitting: settle on A, do something, settle on B.
//
// The shape a human would produce in thirty seconds, and the shape tomorrow's live test asks
// for. `after` is what was observed before the change; nil means nothing was.
// TWO round trips, because a screen seen once is not yet a screen Marco will commit to.
// `MinEpisodes` is the policy — a transition frame and a screen are indistinguishable until one
// of them comes back — and a fixture that visited each screen once would be testing the policy's
// refusal rather than the substrate underneath it.
// The terms are the ones the live sessions actually reported — `search` and `notifications`
// in VS Code, `settings` in Chrome — and they are what makes each screen RECOGNISABLE later.
// A screen with no read text and no envelope carries no discriminator and can never become a
// durable subject; see TestAScreenWithNoDiscriminatorCannotBeRemembered for that half.
func visit(after ...observe.NavIntent) script {
	editor, settings := screenfixture.Editor(), screenfixture.Settings()
	var frames []frame
	for range 2 {
		frames = append(frames,
			reading(stayOn(editor, 4), observe.TermSearch, observe.TermNotifications)...)
		frames = append(frames,
			reading(pressThen(settings, 10, after...), observe.TermSettings, observe.TermAudio)...)
	}
	frames = append(frames,
		reading(stayOn(editor, 3), observe.TermSearch, observe.TermNotifications)...)
	return script{frames: frames}
}

// sitting runs ONE whole session against a store and answers every question it asked.
//
// Answering is what puts a screen into durable memory — a subject enters the store when a person
// says something about it — so a test about relationships has to model a user who engaged, or
// both endpoints stay unrecognisable and there is nothing to relate.
func sitting(t *testing.T, m observe.Memory, s script) observesession.Result {
	t.Helper()
	cfg := config()
	cfg.ProposalPolicy = observe.ProposalThresholds{MaxOpen: 12, MaxProposals: 64}
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&scripted{s: s}, &recordingEvents{}).WithMemory(m)

	got, err := r.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Every SEMANTIC question, answered yes. An empty Ask means AskSemantic — every record
	// written before that field existed reads correctly — so the emptiness has to be
	// admitted here or a helper that filtered on the constant would silently answer nothing.
	for _, p := range r.Proposals().Open() {
		if p.Ask == observe.AskSemantic || p.Ask == "" {
			_, _ = r.Respond(p.ID, observe.ResponseConfirmed)
		}
	}
	return got
}

// ── Part 7: does the transition reach the relationship machinery ──────────────

// A transition between two screens the user has confirmed becomes durable relationship evidence.
//
// The whole point of Part 7: prove the transition ARRIVES, not that learning succeeds. What the
// policy then does with it is the policy's business and is read rather than forced.
func TestARealShapedTransitionReachesTheRelationshipMachinery(t *testing.T) {
	dir := t.TempDir()

	// First sitting establishes the two screens in memory.
	first := sitting(t, memoryAt(t, dir), visit(observe.NavConfirm))
	if len(first.Stats.Shadow.Transitions) == 0 {
		t.Fatal("the fixture produced no transition; nothing below could be tested")
	}
	requireTwoScreens(t, dir)

	// Second sitting: both endpoints are now recognisable, so the edge can go durable.
	second := sitting(t, memoryAt(t, dir), visit(observe.NavConfirm))

	rels := relationshipsIn(t, dir)
	if len(rels) == 0 {
		t.Fatalf("a transition between two remembered screens produced no durable "+
			"relationship.\nsession reported: durable=%d session-local=%d created=%d "+
			"corroborated=%d rejected=%d unavailable=%q",
			second.Relationships.Durable, second.Relationships.SessionLocal,
			second.Relationships.Created, second.Relationships.Corroborated,
			second.Relationships.Rejected, second.Relationships.Unavailable)
	}

	// It carries the navigation as CORRELATION, with its control number beside it.
	var found bool
	for _, r := range rels {
		ev := r.Evidence()
		if ev.Preceded[observe.NavConfirm] > 0 {
			found = true
			if ev.Observations == 0 {
				t.Error("an edge with a correlation has no observation count to weigh it against")
			}
		}
		t.Logf("edge %s -> %s: observations=%d sessions=%d preceded=%v unattributed=%d",
			short(r.From), short(r.To), r.Observations, r.Sessions, ev.Preceded, ev.Unattributed)
	}
	if !found {
		t.Error("no durable edge carried the navigation observed before the change")
	}
}

// An unattributed transition still reaches memory.
//
// A change nobody caused is still a fact about how two screens are connected. Dropping it would
// also destroy the control evidence that keeps the attributed ones honest.
func TestAnUnattributedTransitionStillReachesMemory(t *testing.T) {
	dir := t.TempDir()
	sitting(t, memoryAt(t, dir), visit()) // no navigation at all
	requireTwoScreens(t, dir)
	sitting(t, memoryAt(t, dir), visit())

	rels := relationshipsIn(t, dir)
	if len(rels) == 0 {
		t.Fatal("a change with no observed cause produced no durable evidence")
	}
	var unattributed int
	for _, r := range rels {
		unattributed += r.Evidence().Unattributed
	}
	if unattributed == 0 {
		t.Error("the durable edge does not record that nothing was observed before the change")
	}
}

// ── Part 8: recurrence across REAL session boundaries ─────────────────────────

// Four sittings of the same interaction count as four sittings, not one long stream.
//
// The distinction learning rests on: twenty observations in one afternoon and the same edge on
// four separate days are different kinds of evidence, and only the second has survived a
// restart, a different window generation and whatever else changed in between.
func TestRecurrenceIsCountedAcrossSessionsNotWithinOne(t *testing.T) {
	dir := t.TempDir()
	const sittings = 4
	for range sittings {
		sitting(t, memoryAt(t, dir), visit(observe.NavConfirm))
	}

	rels := relationshipsIn(t, dir)
	if len(rels) == 0 {
		t.Fatal("four agreeing sittings produced no durable edge")
	}
	var best observe.RememberedRelationship
	for _, r := range rels {
		if r.Sessions > best.Sessions {
			best = r
		}
	}
	t.Logf("strongest edge: observations=%d sessions=%d", best.Observations, best.Sessions)

	if best.Sessions < 2 {
		t.Fatalf("%d separate sittings were counted as %d session(s). Recurrence would be "+
			"indistinguishable from one long event stream", sittings, best.Sessions)
	}
	if best.Sessions > sittings {
		t.Errorf("%d sittings were counted as %d", sittings, best.Sessions)
	}
	// Observations accumulate SEPARATELY from sittings, and both are kept.
	if best.Observations < best.Sessions {
		t.Errorf("observations=%d is fewer than sessions=%d", best.Observations, best.Sessions)
	}
}

// ── Part 12: the two-session synthetic, and what Director concluded ───────────

// Two realistic agreeing sittings. What does the EXISTING machinery make of them?
//
// Nothing is forced. The assertions are only that the evidence arrived and that no claim
// stronger than the evidence was made; the interesting output is the log.
func TestWhatDirectorConcludesFromTwoAgreeingSittings(t *testing.T) {
	dir := t.TempDir()
	var last observesession.Result
	for i := range 2 {
		last = sitting(t, memoryAt(t, dir), visit(observe.NavConfirm))
		t.Logf("── sitting %d ──", i+1)
		t.Logf("   screens=%d transitions=%d", len(last.Stats.Shadow.States),
			len(last.Stats.Shadow.Transitions))
		t.Logf("   relationships: durable=%d session-local=%d created=%d corroborated=%d",
			last.Relationships.Durable, last.Relationships.SessionLocal,
			last.Relationships.Created, last.Relationships.Corroborated)
	}

	for _, r := range relationshipsIn(t, dir) {
		ev := r.Evidence()
		t.Logf("remembered: %s -> %s  observations=%d sessions=%d preceded=%v "+
			"unattributed=%d conditional_only=%d sequences=%d",
			short(r.From), short(r.To), r.Observations, r.Sessions,
			ev.Preceded, ev.Unattributed, ev.ConditionalOnly, len(ev.Sequences))
	}

	// What Director decided about LEARNING, in its own closed reasons.
	if len(last.Learning) == 0 {
		t.Log("learning: nothing was judged (no remembered relationship reached the policy)")
	}
	for _, a := range last.Learning {
		t.Logf("learning: %s -> %s eligible=%v refusals=%v observations=%d sessions=%d",
			short(a.Relationship.From), short(a.Relationship.To),
			a.Eligible, a.Refusals, a.Observations, a.Sessions)
	}

	// The one thing that must hold: nothing claimed more than it saw.
	for _, r := range relationshipsIn(t, dir) {
		if r.Sessions > 2 {
			t.Errorf("two sittings produced an edge claiming %d", r.Sessions)
		}
		if ev := r.Evidence(); ev.Preceded[observe.NavConfirm] > ev.Observations {
			t.Errorf("an intent is credited to more observations than exist: %d of %d",
				ev.Preceded[observe.NavConfirm], ev.Observations)
		}
	}
}

// ── Part 13: the adversarial variant ──────────────────────────────────────────

// Four sittings that DISAGREE about what preceded the change.
//
// The same two screens every time, and a different navigation before each. A policy that
// learned a stable relationship from this would be learning that any key works, which is the
// most damaging thing this system could conclude — it would rehearse the wrong one and blame
// the application.
func TestFourDisagreeingSittingsDoNotProduceAConfidentCause(t *testing.T) {
	dir := t.TempDir()
	intents := []observe.NavIntent{
		observe.NavDown, observe.NavConfirm, observe.NavBack, observe.NavLeft,
	}
	var last observesession.Result
	for _, in := range intents {
		last = sitting(t, memoryAt(t, dir), visit(in))
	}

	rels := relationshipsIn(t, dir)
	if len(rels) == 0 {
		t.Skip("the disagreeing sittings produced no durable edge at all; there is nothing " +
			"to be over-confident about")
	}
	for _, r := range rels {
		ev := r.Evidence()
		t.Logf("remembered: %s -> %s observations=%d sessions=%d preceded=%v sequences=%d",
			short(r.From), short(r.To), r.Observations, r.Sessions, ev.Preceded,
			len(ev.Sequences))

		if ev.Attributed() == 0 {
			continue // the unattributed direction; nothing to be over-confident about
		}
		// EVERY intent survives beside the others. A representation that kept only the most
		// frequent would render several one-off observations as one confident cause.
		if len(ev.Preceded) < 2 {
			t.Errorf("sittings that each used a different key were reduced to %d intent(s): %v",
				len(ev.Preceded), ev.Preceded)
		}
		// And NONE of them dominates. Each sitting makes the same round trip twice, so a
		// key used in one sitting is credited twice — the test is whether any key accounts
		// for a majority of the attributed observations, which is what "a confident cause"
		// would look like.
		for intent, n := range ev.Preceded {
			if n*2 > ev.Attributed() {
				t.Errorf("%q accounts for %d of %d attributed observations across sittings "+
					"that each used a DIFFERENT key; that reads as a cause",
					intent, n, ev.Attributed())
			}
		}
	}

	for _, a := range last.Learning {
		t.Logf("learning: %s -> %s eligible=%v refusals=%v",
			short(a.Relationship.From), short(a.Relationship.To), a.Eligible, a.Refusals)
	}
}

// The control for the adversarial test: agreeing sittings DO concentrate the evidence.
//
// Without this, the assertion above would hold for a system that had simply stopped recording
// navigation at all.
func TestAgreeingSittingsConcentrateTheEvidence(t *testing.T) {
	dir := t.TempDir()
	for range 4 {
		sitting(t, memoryAt(t, dir), visit(observe.NavConfirm))
	}
	rels := relationshipsIn(t, dir)
	if len(rels) == 0 {
		t.Fatal("four agreeing sittings produced no durable edge")
	}
	var best int
	for _, r := range rels {
		if n := r.Evidence().Preceded[observe.NavConfirm]; n > best {
			best = n
		}
	}
	if best < 2 {
		t.Fatalf("four sittings that all pressed confirm credited it %d time(s); the "+
			"disagreeing test above would pass for a system that recorded nothing", best)
	}
	t.Logf("agreeing: confirm credited %d times", best)
}

// A screen with nothing distinctive about it cannot be remembered, so its transitions cannot
// go durable.
//
// THE second scale finding, recorded rather than repaired.
//
// `StructureSignature.Discriminating()` is `(terms were read AND there are some) || there is an
// envelope`. A whole SCREEN has no envelope — it fills the window, and a window-sized envelope
// discriminates nothing — so a screen's only possible discriminator is read text.
//
// That rule is correct at detector scale, where a screen is five boxes over two roles and two
// screens really are indistinguishable without words. At accessibility scale a screen carries a
// twelve-role histogram with forty list items, which is highly distinctive and is NOT counted.
//
// The consequence on real software: whenever a scoped reading does not happen — and it runs on
// roughly one sample in six, by design, because it costs 230–730 ms per control — the screens
// observed in that window can never become durable subjects, and every transition between them
// stays session-local forever.
//
// Not repaired here. Loosening what counts as a discriminator is exactly the change
// [[ADR-016-cross-session-identity-is-structural-and-conservative]] warns about — a wrong
// durable match survives every session that could have contradicted it — and the evidence for
// whether a real role histogram is distinctive enough is a measurement nobody has taken.
func TestAScreenWithNoDiscriminatorCannotBeRemembered(t *testing.T) {
	dir := t.TempDir()

	// The same session, with no scoped reading on any sample.
	silent := func() script {
		editor, settings := screenfixture.Editor(), screenfixture.Settings()
		var frames []frame
		for range 2 {
			frames = append(frames, stayOn(editor, 4)...)
			frames = append(frames, pressThen(settings, 10, observe.NavConfirm)...)
		}
		return script{frames: frames}
	}

	got := sitting(t, memoryAt(t, dir), silent())
	if len(got.Stats.Shadow.Transitions) == 0 {
		t.Fatal("the fixture produced no transition")
	}

	// The screens were observed, hypothesised about, and asked about.
	var screenQuestions int
	for _, h := range got.Hypotheses {
		if h.Subject.Kind != observe.SubjectState {
			continue
		}
		screenQuestions++
		if observe.SignatureOf(h).Discriminating() {
			t.Errorf("a screen with no read text reported a discriminator; this test's "+
				"premise has changed: %+v", observe.SignatureOf(h))
		}
	}
	if screenQuestions == 0 {
		t.Fatal("no screen hypothesis was raised, so nothing here is being tested")
	}

	// And none of them reached the store, so the transition can never go durable.
	sitting(t, memoryAt(t, dir), silent())
	if rels := relationshipsIn(t, dir); len(rels) != 0 {
		t.Errorf("a relationship was written between screens that carry no discriminator: %v",
			rels)
	}
	t.Logf("%d screen hypotheses, 0 durable subjects, 0 relationships — "+
		"the transitions stayed session-local", screenQuestions)
}

// short renders a durable subject id for a log line without pretending it means anything.
func short(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:12] + "…"
}

var _ = fmt.Sprintf
var _ = filepath.Join
var _ = semanticmemory.Store{}
