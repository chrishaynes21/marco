package observesession_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// Asking for a second demonstration, and — much more often — deciding not to.
//
// The governing rule this file defends:
//
//	Do not ask the user for more evidence merely because Marco is uncertain. Ask only when the
//	evidence model can explain what another example would resolve.
//
// The headline tests enter through the real path: a persisted relationship, an approved first
// demonstration, a real assessment, a real follow-up question, a real answer, a real second
// capture, and the comparison of the two. Nothing here calls `AssessCandidate`,
// `FollowUpFrom` or `CompareCandidates` directly.

// ── helpers ───────────────────────────────────────────────────────────────────

func followUpQuestion(r *observesession.Runner) (observe.Proposal, bool) {
	for _, p := range r.Proposals().Open() {
		if p.Ask == observe.AskSecondDemonstration {
			return p, true
		}
	}
	return observe.Proposal{}, false
}

func followUpRefusals(res observesession.Result) map[observe.FollowUpRefusal]bool {
	out := map[observe.FollowUpRefusal]bool{}
	for _, f := range res.FollowUps {
		for _, r := range f.Judgement.Refusals {
			out[r] = true
		}
	}
	return out
}

// firstDemonstration seeds an approved route and gives one complete demonstration of it.
func firstDemonstration(t *testing.T, dir string,
	script []demoFrame) (*semanticmemory.Store, string, string) {

	t.Helper()
	store, from, to := approvedRun(t, dir)
	if _, res := demonstrate(t, store, script); res.Demonstration == nil ||
		!res.Demonstration.Complete {
		t.Fatalf("the first demonstration did not complete: %+v", res.Demonstration)
	}
	return store, from, to
}

// verifiableScript is A → B with every screen recognisable, so the only gap is corroboration.
func verifiableScript() []demoFrame {
	var out []demoFrame
	out = append(out, hold("a", 4)...)
	out = append(out, press("b", observe.NavDown, observe.NavConfirm))
	out = append(out, hold("b", 5)...)
	return out
}

// divergentScript reaches the same destination by a materially different route.
func divergentScript() []demoFrame {
	var out []demoFrame
	out = append(out, hold("a", 4)...)
	out = append(out, press("b", observe.NavBack, observe.NavPause, observe.NavConfirm))
	out = append(out, hold("b", 5)...)
	return out
}

func candidatesFor(t *testing.T, dir string) []observe.ProcedureCandidate {
	t.Helper()
	return memoryAt(t, dir).Candidates("testgame")
}

// ── PART 20 / PART 27: THE production path ────────────────────────────────────

// A follow-up is requested, agreed to, captured, compared, and reassessed — end to end.
//
// Deleting the follow-up review, the second capture arming, or the comparison must fail this.
func TestASecondDemonstrationIsRequestedCapturedAndCompared(t *testing.T) {
	dir := t.TempDir()
	store, from, to := firstDemonstration(t, dir, happyScript())

	// ── the question ──
	asking, res := demonstrate(t, store, hold("", 6))
	p, ok := followUpQuestion(asking)
	if !ok {
		t.Fatalf("no second-demonstration question was put about a route Marco has seen "+
			"exactly once. Refusals: %v", followUpRefusals(res))
	}
	if p.Relationship == nil || p.Relationship.From != from || p.Relationship.To != to {
		t.Fatalf("the question refers to %+v, not to the demonstrated route", p.Relationship)
	}
	if p.Ask != observe.AskSecondDemonstration {
		t.Errorf("ask kind = %q; typed as something else, its answer would be interpreted "+
			"as something else", p.Ask)
	}
	// The wording says WHY, which is the whole product principle.
	lower := strings.ToLower(p.Question)
	if !strings.Contains(lower, "another example") {
		t.Errorf("the question does not explain what it wants: %q", p.Question)
	}
	if strings.TrimSpace(p.Question) == "Show me again?" {
		t.Errorf("the question is a bare demand: %q", p.Question)
	}
	for _, bad := range []string{"candidate 2", "confidence", "subj_"} {
		if strings.Contains(lower, bad) {
			t.Errorf("the question exposes implementation language (%q): %q", bad, p.Question)
		}
	}

	// ── yes, through the real response path ──
	if _, ok := asking.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}
	// It creates a REQUEST, not a verification.
	for _, c := range candidatesFor(t, dir) {
		if c.Verified {
			t.Fatal("agreeing to demonstrate again verified the candidate")
		}
	}

	// ── the second demonstration ──
	_, second := demonstrate(t, store, happyScript())
	if second.Demonstration == nil || !second.Demonstration.Complete {
		t.Fatalf("the second demonstration did not complete: %+v", second.Demonstration)
	}
	if second.Demonstration.Sequence != 2 {
		t.Errorf("the second demonstration was recorded as sequence %d; lineage is lost",
			second.Demonstration.Sequence)
	}
	kept := candidatesFor(t, dir)
	if len(kept) != 2 {
		t.Fatalf("%d candidate(s) kept; candidate 1 was overwritten rather than kept beside "+
			"candidate 2", len(kept))
	}

	// ── the comparison and the reassessment ──
	if second.Assessment == nil {
		t.Fatal("the second demonstration was not assessed")
	}
	if reasonsOf(*second.Assessment)[observe.ReasonSingleDemonstration] {
		t.Errorf("after two agreeing demonstrations the judgement still says it rests on "+
			"one: %v", second.Assessment.Reasons)
	}
	if reasonsOf(*second.Assessment)[observe.ReasonDemonstrationsDisagree] {
		t.Errorf("two equivalent demonstrations were read as disagreeing: %v",
			second.Assessment.Reasons)
	}
	// The route here passes THROUGH a screen with no durable identity — which is what makes
	// it earn a follow-up at all now, since a clean one would simply be offered a rehearsal.
	// So the verdict stays short of consistent, and the reason is that screen rather than the
	// number of examples.
	if !reasonsOf(*second.Assessment)[observe.ReasonTransientCheckpoint] {
		t.Errorf("verdict %q is not explained by the unrecognisable screen on the route: %v",
			second.Assessment.Verdict, second.Assessment.Reasons)
	}
	// AUTHORITY. The strongest outcome this whole system can reach still grants nothing.
	if second.Assessment.Verified {
		t.Fatal("two agreeing demonstrations granted verification")
	}
	joined := strings.Join(observe.DescribeAssessment(*second.Assessment), "\n")
	if !strings.Contains(joined, "verified: no") {
		t.Errorf("the description does not disclaim verification:\n%s", joined)
	}
}

// ── PART 19: the bound ────────────────────────────────────────────────────────

// Two demonstrations, and then Marco stops asking.
//
// "Show me again, show me again" is a collection loop. Whether a third example is worth asking
// for needs the evidence the first two produced, which is a later decision.
func TestMarcoStopsAfterTwoDemonstrations(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := firstDemonstration(t, dir, happyScript())

	asking, _ := demonstrate(t, store, hold("", 6))
	p, ok := followUpQuestion(asking)
	if !ok {
		t.Fatal("no first follow-up")
	}
	if _, ok := asking.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}
	demonstrate(t, store, happyScript())

	// A THIRD session must neither ask again nor arm another capture.
	third, res := demonstrate(t, store, happyScript())
	if p, ok := followUpQuestion(third); ok {
		t.Errorf("Marco asked for a third demonstration: %q", p.Question)
	}
	if !followUpRefusals(res)[observe.RefusalSecondAlreadyCaptured] {
		t.Errorf("the report does not explain the silence: %v", followUpRefusals(res))
	}
	if got := len(candidatesFor(t, dir)); got != 2 {
		t.Errorf("%d candidates exist; the bound of two demonstrations is not holding", got)
	}
}

// ── PART 21: they disagree ────────────────────────────────────────────────────

// Two materially different demonstrations are recorded as a disagreement, never averaged.
func TestTwoDisagreeingDemonstrationsAreNotReconciled(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := firstDemonstration(t, dir, happyScript())

	asking, _ := demonstrate(t, store, hold("", 6))
	p, ok := followUpQuestion(asking)
	if !ok {
		t.Fatal("no follow-up question")
	}
	if _, ok := asking.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}

	_, second := demonstrate(t, store, divergentScript())
	if second.Demonstration == nil || !second.Demonstration.Complete {
		t.Fatalf("the second demonstration did not complete: %+v", second.Demonstration)
	}
	if second.Assessment == nil {
		t.Fatal("no reassessment")
	}
	if !reasonsOf(*second.Assessment)[observe.ReasonDemonstrationsDisagree] {
		t.Fatalf("two materially different routes were not reported as disagreeing: %v",
			second.Assessment.Reasons)
	}
	if second.Assessment.Verdict != observe.CandidateAmbiguous {
		t.Errorf("verdict = %q, want ambiguous", second.Assessment.Verdict)
	}
	if second.Assessment.Verified {
		t.Error("a disagreement granted verification")
	}
	// BOTH are kept. Neither was chosen, averaged, or discarded for being older or shorter.
	kept := candidatesFor(t, dir)
	if len(kept) != 2 {
		t.Fatalf("%d candidate(s) kept after a disagreement", len(kept))
	}
	if kept[0].Sequence == kept[1].Sequence {
		t.Error("both candidates carry the same lineage sequence")
	}
}

// ── PART 22: when another example cannot help ─────────────────────────────────

// A demonstration through a screen the user types on does not earn a follow-up request.
//
// Another passive semantic example does not create text-parameter authority, so asking for one
// would be asking somebody to do work whose value Marco already knows is capped.
func TestNoFollowUpWhenAnotherExampleCannotHelp(t *testing.T) {
	dir := t.TempDir()
	var script []demoFrame
	script = append(script, hold("a", 4)...)
	script = append(script, demoFrame{screen: "x", editable: 2,
		inputs: []observe.NavIntent{observe.NavConfirm}})
	for i := 0; i < 3; i++ {
		script = append(script, demoFrame{screen: "x", editable: 2})
	}
	script = append(script, press("b", observe.NavConfirm))
	script = append(script, hold("b", 4)...)

	store, _, _ := firstDemonstration(t, dir, script)
	asking, res := demonstrate(t, store, hold("", 6))
	if p, ok := followUpQuestion(asking); ok {
		t.Fatalf("Marco asked for another example of a route it cannot use however many it "+
			"sees: %q", p.Question)
	}
	got := followUpRefusals(res)
	if !got[observe.RefusalNonResolvableBlocker] {
		t.Errorf("refused for %v; the blocker should be named", got)
	}
	// And the report says which gaps another example WOULD have closed, so the refusal is
	// legible rather than flat.
	var explained bool
	for _, f := range res.FollowUps {
		if len(f.Judgement.Blocking) > 0 && len(f.Judgement.Resolvable) > 0 {
			explained = true
			lines := strings.Join(observe.DescribeFollowUp(f.Assessment, f.Judgement), "\n")
			if !strings.Contains(lines, "would not help with") {
				t.Errorf("the explanation does not separate the two:\n%s", lines)
			}
		}
	}
	if !explained {
		t.Error("the report does not distinguish what another example could and could not fix")
	}
}

// ── PART 23: a transient checkpoint ───────────────────────────────────────────

// An unrecognisable middle screen IS worth another example, and repeated appearance is
// corroboration rather than recognition.
func TestATransientCheckpointEarnsAFollowUpButNotRecognition(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := firstDemonstration(t, dir, happyScript())
	before := len(memoryAt(t, dir).Subjects())

	asking, res := demonstrate(t, store, hold("", 6))
	p, ok := followUpQuestion(asking)
	if !ok {
		t.Fatalf("no follow-up for an unrecognisable middle screen: %v", followUpRefusals(res))
	}
	if !strings.Contains(strings.ToLower(p.Question), "middle") {
		t.Errorf("the question does not say what the gap is: %q", p.Question)
	}
	if _, ok := asking.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}

	_, second := demonstrate(t, store, happyScript())
	if second.Assessment == nil {
		t.Fatal("no reassessment")
	}
	// Corroborated, and STILL not recognisable. Two sightings of an unknown screen are
	// evidence about the route, not a durable identity.
	if reasonsOf(*second.Assessment)[observe.ReasonSingleDemonstration] {
		t.Error("the second demonstration did not corroborate the first")
	}
	if !reasonsOf(*second.Assessment)[observe.ReasonTransientCheckpoint] {
		t.Error("the middle screen became verifiable merely by being seen twice")
	}
	if after := len(memoryAt(t, dir).Subjects()); after != before {
		t.Errorf("memory grew from %d to %d subjects; a screen seen during a demonstration "+
			"was promoted without anybody agreeing", before, after)
	}
	if second.Assessment.Verdict == observe.CandidateConsistent {
		t.Error("a route with an unrecognisable checkpoint reached the best verdict")
	}
}

// ── PARTS 24/25: not now, and no ──────────────────────────────────────────────

// Not-now suppresses until the judgement changes shape, then asks again.
func TestADeclinedFollowUpReturnsOnlyWhenTheJudgementChanges(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := firstDemonstration(t, dir, happyScript())

	asking, _ := demonstrate(t, store, hold("", 6))
	p, ok := followUpQuestion(asking)
	if !ok {
		t.Fatal("no follow-up question")
	}
	if _, ok := asking.Respond(p.ID, observe.ResponseDeclined); !ok {
		t.Fatal("the decline was not recorded")
	}

	// Nothing changed. Running again must not re-ask.
	quiet, res := demonstrate(t, store, hold("", 6))
	if _, ok := followUpQuestion(quiet); ok {
		t.Error("a declined follow-up came straight back; that is the nagging a decline " +
			"exists to prevent")
	}
	if !followUpRefusals(res)[observe.RefusalFollowUpDeclined] {
		t.Errorf("refused for %v, not for the earlier decline", followUpRefusals(res))
	}

	// The JUDGEMENT changes: the user names the middle screen, so the reason set changes
	// shape and the question becomes worth asking again.
	if err := store.Remember("testgame", middleSignature(), observe.SemanticKnowledge{
		Kind: observe.PossibleMenuLikeState, Status: observe.KnowledgeConfirmed,
	}); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	//
	// What comes back is the REHEARSAL offer rather than another request to repeat, and that
	// is the point rather than a compromise: naming the middle screen made the route fully
	// verifiable, and a verifiable route is something Marco can try. The invariant under test
	// — a decline is not permanent — holds, and the question that returns is the better one.
	// [[ADR-051-one-demonstration-and-an-attempt]]
	changed, _ := demonstrate(t, store, hold("", 6))
	_, askedAgain := followUpQuestion(changed)
	_, offered := rehearsalQuestion(changed)
	if !askedAgain && !offered {
		t.Error("Marco's judgement changed shape and no question came back at all; a " +
			"decline has become permanent")
	}
}

// A refusal is a preference and touches nothing else.
func TestRefusingASecondDemonstrationChangesNothingElse(t *testing.T) {
	dir := t.TempDir()
	store, from, to := firstDemonstration(t, dir, happyScript())
	beforeCandidates := candidatesFor(t, dir)
	beforeSubjects := len(memoryAt(t, dir).Subjects())

	asking, _ := demonstrate(t, store, hold("", 6))
	p, ok := followUpQuestion(asking)
	if !ok {
		t.Fatal("no follow-up question")
	}
	if _, ok := asking.Respond(p.ID, observe.ResponseContradicted); !ok {
		t.Fatal("the refusal was not recorded")
	}

	// Candidate 1 survives untouched.
	after := candidatesFor(t, dir)
	if len(after) != len(beforeCandidates) || !after[0].Complete {
		t.Errorf("refusing a second demonstration disturbed the first: %+v", after)
	}
	// The relationship survives, and nothing semantic was contradicted.
	var found bool
	for _, rel := range memoryAt(t, dir).Relationships() {
		if rel.From == from && rel.To == to {
			found = true
			if rel.Observations == 0 {
				t.Error("the relationship's evidence was cleared")
			}
			if rel.Learning == nil {
				t.Error("the original learning request was removed")
			}
		}
	}
	if !found {
		t.Fatal("the relationship was deleted")
	}
	if got := len(memoryAt(t, dir).Subjects()); got != beforeSubjects {
		t.Errorf("subjects went from %d to %d", beforeSubjects, got)
	}
	for _, s := range memoryAt(t, dir).Subjects() {
		for _, k := range s.Knowledge {
			if k.Status == observe.KnowledgeContradicted {
				t.Errorf("declining to demonstrate again contradicted %s", k.Kind)
			}
		}
	}
	// And no second capture happens.
	_, res := demonstrate(t, store, verifiableScript())
	if got := len(candidatesFor(t, dir)); got != 1 {
		t.Errorf("%d candidates after a refusal", got)
	}
	if !followUpRefusals(res)[observe.RefusalFollowUpRefused] {
		t.Errorf("refused for %v", followUpRefusals(res))
	}
}

// ── PART 26: a delayed answer binds to the route it was asked about ───────────

// The answer follows the question, not whatever candidate is newest.
func TestADelayedFollowUpAnswerBindsToItsOwnRoute(t *testing.T) {
	dir := t.TempDir()
	store, from, to := firstDemonstration(t, dir, happyScript())

	asking, _ := demonstrate(t, store, hold("", 6))
	p, ok := followUpQuestion(asking)
	if !ok {
		t.Fatal("no follow-up question")
	}

	// A DIFFERENT route acquires its own demonstration while the question is open.
	other := seedOtherRoute(t, store, from)
	if _, ok := asking.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}

	// The pending follow-up belongs to the route that was asked about.
	var pending []observe.RelationshipRef
	for _, rel := range memoryAt(t, dir).Relationships() {
		if rel.FollowUp != nil && rel.FollowUp.Status == observe.LearningPending {
			pending = append(pending, observe.RelationshipRef{From: rel.From, To: rel.To})
		}
	}
	if len(pending) != 1 {
		t.Fatalf("%d pending follow-up(s): %+v", len(pending), pending)
	}
	if pending[0].From != from || pending[0].To != to {
		t.Fatalf("the answer landed on %+v, not on the route the question named (%s → %s)",
			pending[0], from, to)
	}
	if pending[0].To == other {
		t.Fatal("the answer bound to the other route")
	}
}

// seedOtherRoute adds a second remembered route out of the same start.
func seedOtherRoute(t *testing.T, store *semanticmemory.Store, from string) string {
	t.Helper()
	if err := store.Remember("testgame", cSignature(), observe.SemanticKnowledge{
		Kind: observe.PossibleMenuLikeState, Status: observe.KnowledgeConfirmed,
	}); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	var third string
	for _, s := range store.Subjects() {
		if len(s.Structure.Terms) > 0 && s.Structure.Terms[0] == observe.TermInvite {
			third = s.ID
		}
	}
	for i := 0; i < 3; i++ {
		if _, err := store.RememberRelationships("testgame",
			[]observe.RelationshipObservation{{
				From: from, To: third, Evidence: strongEvidence(),
			}}); err != nil {
			t.Fatalf("seeding: %v", err)
		}
	}
	return third
}

// middleSignature is the fixture's intermediate screen, as memory would hold it.
func middleSignature() observe.StructureSignature {
	return observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 4, "icon": 1},
		Terms:      []observe.InterfaceTerm{observe.TermHelp, observe.TermLanguage},
		TermsKnown: true,
	}
}

// ── PART 31/36: privacy and authority ─────────────────────────────────────────

// Nothing captured reaches the follow-up record, and nothing about it can be run.
func TestTheFollowUpRecordHoldsNothingCaptured(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := firstDemonstration(t, dir, happyScript())
	asking, _ := demonstrate(t, store, hold("", 6))
	p, ok := followUpQuestion(asking)
	if !ok {
		t.Fatal("no follow-up question")
	}
	if _, ok := asking.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}
	demonstrate(t, store, happyScript())

	raw := readStore(t, dir)
	for _, leak := range []string{
		"keycode", "scancode", "vk_", "hwnd", "generation", "process",
		"state_", "shadow_", "screenshot", "pixel", "\"verified\":true",
	} {
		if strings.Contains(strings.ToLower(raw), strings.ToLower(leak)) {
			t.Errorf("the store contains %q", leak)
		}
	}
	for _, want := range []string{"follow_up", "candidates", "\"sequence\": 2"} {
		if !strings.Contains(raw, want) {
			t.Errorf("the store does not contain %q", want)
		}
	}
}

// readStore returns the memory file's bytes.
func readStore(t *testing.T, dir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "memory.json"))
	if err != nil {
		t.Fatalf("reading the store: %v", err)
	}
	return string(raw)
}
