package observesession_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// Asking to try, and the one yes that creates authority.
//
// The chain this file defends, end to end and through the real path:
//
//	ProcedureCandidate → CandidateAssessment → RehearsalJudgement → AskRehearse → YES
//	→ RehearsalGrant → STOP
//
// And the thing it defends about that chain: **nothing before the last arrow can act, and the
// last arrow does not act either.** A grant is authority data for one future experiment. This
// milestone performs no input at all, and the final assertion of the headline test is that the
// strongest state reachable is an inert grant.

// ── the production flow ───────────────────────────────────────────────────────

// rehearsable drives the whole real path to a corroborated, fully verifiable candidate.
//
// Nothing here calls JudgeRehearsal, AssessCandidate or a grant constructor. It seeds a
// relationship, answers the learning question, gives a demonstration, answers the follow-up
// question, gives the second — all through the machinery those milestones built — and returns the
// runner holding whatever Marco made of it.
func rehearsable(t *testing.T, dir string, script []demoFrame) (
	*semanticmemory.Store, *observesession.Runner, observesession.Result) {

	t.Helper()
	store, _, _ := firstDemonstration(t, dir, script)

	// ONE demonstration. This walk used to answer a follow-up and give a second, and it no
	// longer can: after one clean example the follow-up is not eligible, because an attempt
	// settles what a repetition only repeats.
	// [[ADR-051-one-demonstration-and-an-attempt]]
	runner, res := demonstrate(t, store, hold("", 6))
	return store, runner, res
}

func rehearsalQuestion(r *observesession.Runner) (observe.Proposal, bool) {
	for _, p := range r.Proposals().Open() {
		if p.Ask == observe.AskRehearse {
			return p, true
		}
	}
	return observe.Proposal{}, false
}

func rehearsalRefusals(res observesession.Result) map[observe.RehearsalRefusal]bool {
	out := map[observe.RehearsalRefusal]bool{}
	for _, j := range res.Rehearsals {
		for _, r := range j.Refusals {
			out[r] = true
		}
	}
	return out
}

// THE headline test. Two agreeing demonstrations of a fully verifiable route earn a question,
// and a yes earns exactly one inert grant.
//
// Deleting the judgement→proposal call or the yes→grant call must fail this.
func TestARehearsalIsProposedOnlyWhenTheEvidenceAllows(t *testing.T) {
	dir := t.TempDir()
	store, runner, res := rehearsable(t, dir, verifiableScript())

	p, ok := rehearsalQuestion(runner)
	if !ok {
		t.Fatalf("no rehearsal question after two agreeing demonstrations of a route whose "+
			"every screen Marco recognises. Refusals: %v", rehearsalRefusals(res))
	}
	if p.Ask != observe.AskRehearse {
		t.Fatalf("ask kind = %q", p.Ask)
	}
	// The wording says what it will do and that it will stop. No ids, no numbers.
	lower := strings.ToLower(p.Question)
	for _, want := range []string{"one step at a time", "stop"} {
		if !strings.Contains(lower, want) {
			t.Errorf("the question does not say %q: %q", want, p.Question)
		}
	}
	for _, bad := range []string{"subj_", "candidate", "confidence", "iou", "digest",
		"unverifiable", "directly_verifiable"} {
		if strings.Contains(lower, bad) {
			t.Errorf("the question exposes implementation language (%q): %q", bad, p.Question)
		}
	}

	// Asking is not authority.
	if runner.Grant() != nil {
		t.Fatal("a question created authority. Asking is not permission")
	}

	// ── the one yes that creates a grant ──
	if _, ok := runner.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}
	g := runner.Grant()
	if g == nil {
		t.Fatal("a yes to a rehearsal question created no authorization")
	}
	if !g.Active() {
		t.Fatalf("the grant is %q", g.State())
	}

	// SCOPE: one application, one source, one destination, bounded.
	if g.Application != "testgame" {
		t.Errorf("application scope = %q", g.Application)
	}
	if g.Source == "" || g.Destination == "" || g.Source == g.Destination {
		t.Errorf("the grant does not name where it starts and ends: %+v", g)
	}
	if g.MaxInputs <= 0 || g.MaxDuration <= 0 {
		t.Errorf("the grant carries no bounds: inputs=%d duration=%s",
			g.MaxInputs, g.MaxDuration)
	}
	if g.Evidence == "" {
		t.Error("the grant is not bound to the evidence it was given for")
	}

	// AND NOTHING HAPPENED. The strongest state this milestone can reach is an inert grant:
	// no route was generated, no capability exists, and the store holds what it held.
	// ONE, since one demonstration now earns the question.
	if got := len(store.Candidates("testgame")); got != 1 {
		t.Errorf("%d candidate(s); a grant changed the evidence", got)
	}
	for _, c := range store.Candidates("testgame") {
		if c.Verified {
			t.Fatal("authorising an experiment marked the candidate verified. Nothing has " +
				"been tried")
		}
	}
	raw := readStore(t, dir)
	for _, leak := range []string{"attempt_", "grant", "authoriz", "rehears"} {
		if strings.Contains(strings.ToLower(raw), leak) {
			t.Errorf("the durable store mentions %q; a grant must be ephemeral", leak)
		}
	}
}

// ── a yes to something else is not a yes to this ──────────────────────────────

// The load-bearing distinction: learning permission is not acting permission.
func TestOnlyARehearsalYesCreatesAuthority(t *testing.T) {
	dir := t.TempDir()
	store := memoryAt(t, dir)
	seedRelationshipIn(t, store, 3, strongEvidence())

	// Yes to "shall I learn this?" — the permission that authorises WATCHING.
	runner, _ := runOver(t, store)
	p, ok := learningQuestion(runner)
	if !ok {
		t.Fatal("no learning question")
	}
	if _, ok := runner.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}
	if g := runner.Grant(); g != nil {
		t.Fatalf("saying yes to LEARNING created authority to act: %+v", g)
	}

	// And yes to "shall I watch again?" is not either.
	store2, second, _ := func() (*semanticmemory.Store, *observesession.Runner, observesession.Result) {
		d := t.TempDir()
		return rehearsableFollowUpOnly(t, d)
	}()
	_ = store2
	if g := second.Grant(); g != nil {
		t.Fatalf("saying yes to a SECOND DEMONSTRATION created authority to act: %+v", g)
	}
}

// rehearsableFollowUpOnly answers the follow-up question and stops there.
func rehearsableFollowUpOnly(t *testing.T, dir string) (
	*semanticmemory.Store, *observesession.Runner, observesession.Result) {

	t.Helper()
	// A route THROUGH an unrecognisable screen, which is what earns a follow-up now: a clean
	// one is offered a rehearsal instead. [[ADR-051-one-demonstration-and-an-attempt]]
	store, _, _ := firstDemonstration(t, dir, happyScript())
	runner, res := demonstrate(t, store, hold("", 6))
	p, ok := followUpQuestion(runner)
	if !ok {
		t.Fatal("no follow-up question")
	}
	if _, ok := runner.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}
	return store, runner, res
}

// ── the refusal matrix ────────────────────────────────────────────────────────

// Every designed blocker refuses, and a contained unobservable step does not.
func TestTheRehearsalRefusalMatrix(t *testing.T) {
	full := knownTopology("subj_a", "subj_b", "subj_x")

	cases := []struct {
		name    string
		c       observe.ProcedureCandidate
		top     observe.Topology
		corr    observe.Corroboration
		app     string
		refusal observe.RehearsalRefusal
		want    bool // eligible?
	}{{
		name: "corroborated and fully verifiable", c: cleanCandidate(), top: full,
		corr: agreeing(), app: "testgame", want: true,
	}, {
		// ONE demonstration is eligible. It used to refuse, and the refusal was circular:
		// a rehearsal is how Marco finds out whether one example was enough, so withholding
		// it for lack of confidence withheld the only thing that could produce confidence.
		// The user still has to say yes; that is where the safety lives.
		// [[ADR-051-one-demonstration-and-an-attempt]]
		name: "one demonstration, cleanly observed", c: cleanCandidate(), top: full,
		app: "testgame", want: true,
	}, {
		name: "the two disagree", c: cleanCandidate(), top: full, app: "testgame",
		corr:    observe.Corroboration{Compared: true, Agreement: observe.AgreementDifferent},
		refusal: observe.RefusalDemonstrationsDisagree,
	}, {
		name: "a screen it cannot recognise at the end",
		c: candidate(
			step(checkpoint("subj_x"), observe.NavConfirm),
			step(transient(observe.TermHelp), observe.NavConfirm)),
		top: full, corr: agreeing(), app: "testgame",
		refusal: observe.RefusalUnverifiableCheckpoint,
	}, {
		name: "a screen the user typed on",
		c: func() observe.ProcedureCandidate {
			c := cleanCandidate()
			c.Steps[0].RequiresTextEntry = true
			return c
		}(),
		top: full, corr: agreeing(), app: "testgame",
		refusal: observe.RefusalRequiresTextEntry,
	}, {
		name: "a click with nothing behind it",
		c: candidate(
			step(checkpoint("subj_x"), observe.NavPoint),
			step(checkpoint("subj_b"), observe.NavConfirm)),
		top: full, corr: agreeing(), app: "testgame",
		refusal: observe.RefusalUnresolvedPointer,
	}, {
		name: "a start memory no longer holds", c: cleanCandidate(),
		top: knownTopology("subj_b", "subj_x"), corr: agreeing(), app: "testgame",
		refusal: observe.RefusalEndpointUnrecognised,
	}, {
		name: "a destination memory no longer holds", c: cleanCandidate(),
		top: knownTopology("subj_a", "subj_x"), corr: agreeing(), app: "testgame",
		refusal: observe.RefusalEndpointUnrecognised,
	}, {
		name: "another application's candidate", c: cleanCandidate(), top: full,
		corr: agreeing(), app: "someothergame",
		refusal: observe.RefusalApplicationMismatch,
	}, {
		name: "a run of unseeable moves that settles",
		c: candidate(
			step(staysPut(), observe.NavDown),
			step(staysPut(), observe.NavDown),
			step(checkpoint("subj_b"), observe.NavConfirm)),
		top: full, corr: agreeing(), app: "testgame", want: true,
	}, {
		name: "more unseeable moves than the design contains",
		c: candidate(
			step(staysPut(), observe.NavDown),
			step(staysPut(), observe.NavDown),
			step(staysPut(), observe.NavDown),
			step(checkpoint("subj_b"), observe.NavConfirm)),
		top: full, corr: agreeing(), app: "testgame",
		refusal: observe.RefusalUnverifiableCheckpoint,
	}, {
		// A screen Marco recognised WHEN IT WATCHED and does not recognise now. Nothing
		// about the demonstration is wrong, so the assessment has no complaint — the
		// per-step reading is the only thing that can catch it.
		name: "a screen in the middle that memory no longer holds",
		c: candidate(
			step(checkpoint("subj_gone"), observe.NavConfirm),
			step(checkpoint("subj_b"), observe.NavConfirm)),
		top: full, corr: agreeing(), app: "testgame",
		refusal: observe.RefusalUnverifiableCheckpoint,
	}, {
		name: "a rehearsal that would end on a step it can only contain",
		c: candidate(
			step(checkpoint("subj_b"), observe.NavConfirm),
			step(checkpoint("subj_b"), observe.NavDown)),
		top: full, corr: agreeing(), app: "testgame",
		refusal: observe.RefusalUnverifiableCheckpoint,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := observe.AssessCandidate(tc.c, tc.top, observe.DefaultCaptureBounds(), tc.corr)
			j := observe.JudgeRehearsal(tc.c, a, tc.top, tc.app)
			if j.Eligible != tc.want {
				t.Errorf("eligible = %v, want %v (refusals %v; assessment %v)",
					j.Eligible, tc.want, j.Refusals, a.Reasons)
			}
			if tc.refusal != "" {
				var found bool
				for _, r := range j.Refusals {
					if r == tc.refusal {
						found = true
					}
				}
				if !found {
					t.Errorf("refusals %v do not include %q", j.Refusals, tc.refusal)
				}
			}
			// A judgement never grants.
			if j.Eligible {
				if _, err := observe.NewRehearsalGrant("", j, time.Now()); err == nil &&
					j.Application == "" {
					t.Error("an unscoped judgement produced a grant")
				}
			}
		})
	}
}

// staysPut is a step that moved something inside the screen it started on — the contained case.
//
// `down` in a menu. The screen is the same remembered screen afterwards, so Marco can check it is
// still there, and it can see nothing at all about the selection that actually moved.
func staysPut() observe.Checkpoint { return checkpoint("subj_a") }

func agreeing() observe.Corroboration {
	return observe.Corroboration{Compared: true, Agreement: observe.AgreementSame}
}

// An unobservable step is never success.
func TestProgressUnobservableIsNeverSuccess(t *testing.T) {
	if !observe.ProgressUnobservable.Progressed() {
		t.Error("an unobservable step must permit continuing; it is contained, not failed")
	}
	if observe.ProgressUnobservable == observe.DirectlyVerifiable {
		t.Fatal("the two collapsed into one value")
	}
	// The judgement must report it as its own thing, not fold it into the verifiable count.
	c := candidate(
		step(staysPut(), observe.NavDown),
		step(checkpoint("subj_b"), observe.NavConfirm))
	top := knownTopology("subj_a", "subj_b")
	j := observe.JudgeRehearsal(c,
		observe.AssessCandidate(c, top, observe.DefaultCaptureBounds(), agreeing()),
		top, "testgame")
	if j.Unobservable != 1 {
		t.Errorf("unobservable = %d, want 1", j.Unobservable)
	}
	if len(j.Plan) != 2 || j.Plan[0].Verifiability != observe.ProgressUnobservable {
		t.Fatalf("the plan does not mark the unseeable step: %+v", j.Plan)
	}
	// What it expects is the screen it must REMAIN on — the containment check, and the only
	// thing an unobservable step entitles Marco to check.
	if j.Plan[0].Expect != "subj_a" {
		t.Errorf("an unobservable step expects %q, not the screen it must stay on",
			j.Plan[0].Expect)
	}
	if j.Plan[1].Verifiability != observe.DirectlyVerifiable || j.Plan[1].Expect != "subj_b" {
		t.Errorf("the settling step is not directly verifiable: %+v", j.Plan[1])
	}
	// And a person is told.
	joined := strings.Join(observe.DescribeRehearsal(j, top), "\n")
	if !strings.Contains(joined, "cannot see") {
		t.Errorf("the description does not say a step is unseeable:\n%s", joined)
	}
}

// ── the grant's lifecycle ─────────────────────────────────────────────────────

func issuedGrant(t *testing.T) *observe.RehearsalGrant {
	t.Helper()
	c := cleanCandidate()
	top := knownTopology("subj_a", "subj_b", "subj_x")
	j := observe.JudgeRehearsal(c,
		observe.AssessCandidate(c, top, observe.DefaultCaptureBounds(), agreeing()),
		top, "testgame")
	if !j.Eligible {
		t.Fatalf("the fixture is not eligible: %v", j.Refusals)
	}
	g, err := observe.NewRehearsalGrant("e1", j, time.Now())
	if err != nil {
		t.Fatalf("issuing: %v", err)
	}
	return g
}

// One grant, one attempt, and the claim happens when the attempt BEGINS.
func TestAGrantAuthorizesExactlyOneAttempt(t *testing.T) {
	g := issuedGrant(t)
	if err := g.BeginAttempt("testgame", g.Source, g.Evidence, time.Now()); err != nil {
		t.Fatalf("the first claim was refused: %v", err)
	}
	if g.State() != observe.GrantConsumed {
		t.Fatalf("state = %q after claiming", g.State())
	}
	if err := g.BeginAttempt("testgame", g.Source, g.Evidence, time.Now()); err == nil {
		t.Fatal("a grant authorised a second attempt. A crash midway through the first would " +
			"leave reusable authority behind")
	}
}

// Scope is checked when the grant is claimed, not merely when it is issued.
func TestAGrantRefusesOutOfScopeClaims(t *testing.T) {
	for _, tc := range []struct {
		name              string
		app, source, evid func(g *observe.RehearsalGrant) string
	}{{
		name: "another application",
		app:  func(*observe.RehearsalGrant) string { return "someothergame" },
	}, {
		name:   "another starting screen",
		source: func(*observe.RehearsalGrant) string { return "subj_elsewhere" },
	}, {
		name: "evidence that has moved since",
		evid: func(*observe.RehearsalGrant) string { return "different" },
	}} {
		t.Run(tc.name, func(t *testing.T) {
			g := issuedGrant(t)
			app, source, evid := "testgame", g.Source, g.Evidence
			if tc.app != nil {
				app = tc.app(g)
			}
			if tc.source != nil {
				source = tc.source(g)
			}
			if tc.evid != nil {
				evid = tc.evid(g)
			}
			if err := g.BeginAttempt(app, source, evid, time.Now()); err == nil {
				t.Fatal("an out-of-scope claim was authorised")
			}
			if g.State() != observe.GrantIssued {
				t.Errorf("a refused claim changed the state to %q", g.State())
			}
		})
	}
}

// A malformed grant is refused rather than defaulted.
func TestAGrantMustBeScopedAndBounded(t *testing.T) {
	base := func() observe.RehearsalJudgement {
		c := cleanCandidate()
		top := knownTopology("subj_a", "subj_b", "subj_x")
		return observe.JudgeRehearsal(c,
			observe.AssessCandidate(c, top, observe.DefaultCaptureBounds(), agreeing()),
			top, "testgame")
	}
	for _, tc := range []struct {
		name string
		mut  func(j *observe.RehearsalJudgement)
	}{
		{"not eligible", func(j *observe.RehearsalJudgement) { j.Eligible = false }},
		{"no application", func(j *observe.RehearsalJudgement) { j.Application = "" }},
		{"no source", func(j *observe.RehearsalJudgement) { j.Source = "" }},
		{"no destination", func(j *observe.RehearsalJudgement) { j.Destination = "" }},
		{"no input bound", func(j *observe.RehearsalJudgement) { j.Inputs = 0 }},
		{"no evidence binding", func(j *observe.RehearsalJudgement) { j.Digest = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j := base()
			tc.mut(&j)
			if g, err := observe.NewRehearsalGrant("e1", j, time.Now()); err == nil {
				t.Fatalf("a grant was issued with %s: %+v", tc.name, g)
			}
		})
	}
}

// Attempt ids are distinct back to back.
//
// A timestamp is not enough: `time.Now().UnixNano()` is nanosecond-typed and not
// nanosecond-resolved, and the Windows clock advances in ~15ms steps. This repository has
// already had one flaky test caused by exactly that.
func TestAttemptIdentityIsDistinctBackToBack(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		g := issuedGrant(t)
		if seen[g.ID] {
			t.Fatalf("two grants share the id %q", g.ID)
		}
		seen[g.ID] = true
	}
}

// Cancelling withdraws the authorization immediately.
func TestCancellingWithdrawsTheAuthorization(t *testing.T) {
	dir := t.TempDir()
	_, runner, _ := rehearsable(t, dir, verifiableScript())
	p, ok := rehearsalQuestion(runner)
	if !ok {
		t.Fatal("no rehearsal question")
	}
	if _, ok := runner.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}
	g := runner.Grant()
	if g == nil || !g.Active() {
		t.Fatal("no active grant to cancel")
	}
	runner.CancelCapture()
	if g.Active() {
		t.Fatal("cancelling left the authorization standing")
	}
	if err := g.BeginAttempt("testgame", g.Source, g.Evidence, time.Now()); err == nil {
		t.Fatal("a withdrawn grant authorised an attempt")
	}
}

// A new Director has no authority, whatever happened before.
func TestNoAuthoritySurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	store, runner, _ := rehearsable(t, dir, verifiableScript())
	p, ok := rehearsalQuestion(runner)
	if !ok {
		t.Fatal("no rehearsal question")
	}
	if _, ok := runner.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}
	if runner.Grant() == nil {
		t.Fatal("no grant to lose")
	}

	// A new Director over the same store.
	reopened := memoryAt(t, dir)
	fresh, _ := demonstrate(t, reopened, hold("", 6))
	if g := fresh.Grant(); g != nil {
		t.Fatalf("a new Director began with authority: %+v", g)
	}
	// And nothing durable could restore one.
	raw := readStore(t, dir)
	var f map[string]any
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		t.Fatalf("the store is unreadable: %v", err)
	}
	for key := range f {
		if strings.Contains(strings.ToLower(key), "grant") ||
			strings.Contains(strings.ToLower(key), "authoriz") {
			t.Errorf("the durable store has a %q section", key)
		}
	}
	_ = store
}

// One active authority at a time, and Marco stops asking while it stands.
func TestOnlyOneAuthorizationIsActiveAtATime(t *testing.T) {
	dir := t.TempDir()
	store, runner, _ := rehearsable(t, dir, verifiableScript())
	p, ok := rehearsalQuestion(runner)
	if !ok {
		t.Fatal("no rehearsal question")
	}
	if _, ok := runner.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}
	first := runner.Grant()
	if first == nil {
		t.Fatal("no grant")
	}
	// Answering the same question again cannot mint a second future authority.
	runner.Respond(p.ID, observe.ResponseConfirmed)
	if second := runner.Grant(); second != first {
		t.Fatalf("a second authorization appeared: %+v", second)
	}

	// And the next session over the SAME runner does not ask again — the outstanding
	// permission is reported as the reason, rather than a second one being offered.
	if _, err := runner.Run(context.Background(), config()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, ok := rehearsalQuestion(runner); ok {
		t.Fatal("Marco asked to try again while an unused permission was still standing")
	}
	if runner.Grant() != first {
		t.Fatal("a second session replaced the outstanding authorization")
	}
	_ = store
}

// ── the delayed answer ────────────────────────────────────────────────────────

// The answer belongs to the question that was put, not to whatever is newest.
func TestADelayedRehearsalAnswerBindsToItsOwnCandidate(t *testing.T) {
	dir := t.TempDir()
	store, runner, _ := rehearsable(t, dir, verifiableScript())
	p, ok := rehearsalQuestion(runner)
	if !ok {
		t.Fatal("no rehearsal question")
	}
	want := *p.Relationship

	// Another route acquires its own demonstration while the question is open, and it is a
	// route that sorts AHEAD of this one in the store. That ordering is the whole point: a
	// yes that reached for "a demonstration of this application" rather than "a demonstration
	// of THIS ROUTE" would find that one first, and the mistake would be invisible in a
	// fixture where the questioned route happens to come first anyway.
	seedEarlierRoute(t, store, want)
	if got := store.Candidates("testgame")[0].Relationship; got == want {
		t.Fatalf("the interfering route does not sort ahead of the questioned one (%+v); "+
			"this test cannot detect an answer binding to the wrong route", got)
	}

	if _, ok := runner.Respond(p.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}
	g := runner.Grant()
	if g == nil {
		t.Fatal("no grant: the answer did not find the evidence its own question was about")
	}
	if g.Relationship != want {
		t.Fatalf("the grant names %+v, not the route the question was about (%+v)",
			g.Relationship, want)
	}
}

// seedEarlierRoute gives the store one demonstration of a route that sorts before `after`.
//
// Interference, not the path under test — the equivalent of [[seedOtherRoute]] for a milestone
// whose question is which candidate an answer reaches for.
func seedEarlierRoute(t *testing.T, store *semanticmemory.Store, after observe.RelationshipRef) {
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
	ref := observe.RelationshipRef{From: third, To: after.From}
	if _, err := store.RememberRelationships("testgame",
		[]observe.RelationshipObservation{{
			From: ref.From, To: ref.To, Evidence: strongEvidence(),
		}}); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	if err := store.RememberCandidate("testgame", observe.ProcedureCandidate{
		Relationship: ref, Sequence: 1, Application: "testgame",
		Start: observe.Checkpoint{Subject: ref.From, Verdict: observe.MatchSame},
		Steps: []observe.DemonstrationStep{{
			Intents: []observe.NavIntent{observe.NavConfirm},
			Arrived: observe.Checkpoint{Subject: ref.To, Verdict: observe.MatchSame},
		}},
		Complete: true, Reason: observe.ReasonArrived, Checkpoints: 2, Events: 1,
	}); err != nil {
		t.Fatalf("seeding a candidate: %v", err)
	}
}

// Evidence that moved between the question and the answer refuses the grant.
//
// Two ways it can move, and they are caught by different halves of the same guard.
func TestAStaleQuestionCannotAuthorize(t *testing.T) {
	// The evidence got WORSE. A third demonstration disagrees with the first two, so the
	// judgement Marco would make now is not the one the user was shown.
	t.Run("no longer eligible", func(t *testing.T) {
		dir := t.TempDir()
		store, runner, _ := rehearsable(t, dir, verifiableScript())
		p, ok := rehearsalQuestion(runner)
		if !ok {
			t.Fatal("no rehearsal question")
		}
		if err := store.RememberFollowUp("testgame", *p.Relationship,
			observe.LearningRequest{Status: observe.LearningPending}); err != nil {
			t.Fatalf("re-arming: %v", err)
		}
		demonstrate(t, store, divergentScript())

		if _, ok := runner.Respond(p.ID, observe.ResponseConfirmed); !ok {
			t.Fatal("the answer was not recorded")
		}
		if g := runner.Grant(); g != nil {
			t.Fatalf("a yes given about evidence Marco no longer believes produced "+
				"authority: %+v", g)
		}
	})

	// The evidence CHANGED while staying good enough. This is the half `eligible` cannot
	// see: the same verdict over a different procedure. The user agreed to one attempt at
	// what they were shown, and this is no longer that.
	t.Run("still eligible but not what was asked", func(t *testing.T) {
		dir := t.TempDir()
		store, runner, _ := rehearsable(t, dir, verifiableScript())
		p, ok := rehearsalQuestion(runner)
		if !ok {
			t.Fatal("no rehearsal question")
		}
		var revised observe.ProcedureCandidate
		for _, c := range store.Candidates("testgame") {
			if c.Relationship == *p.Relationship && c.Sequence == 1 {
				revised = c
			}
		}
		if revised.Sequence != 1 {
			t.Fatal("the fixture has no first demonstration to revise")
		}
		// One more input than the user was told about. Still consistent, still agreeing,
		// still eligible — and a different thing to be given permission for, because the
		// input bound the grant would carry is derived from exactly this number.
		revised.Events++
		if err := store.RememberCandidate("testgame", revised); err != nil {
			t.Fatalf("revising: %v", err)
		}

		if _, ok := runner.Respond(p.ID, observe.ResponseConfirmed); !ok {
			t.Fatal("the answer was not recorded")
		}
		if g := runner.Grant(); g != nil {
			t.Fatalf("a yes given about evidence that has since changed produced "+
				"authority: %+v", g)
		}
	})
}

// ── authority and privacy ─────────────────────────────────────────────────────

// The grant cannot act, and cannot hold anything captured.
func TestAGrantIsInertAndHoldsNothingCaptured(t *testing.T) {
	rt := reflect.TypeOf(observe.RehearsalGrant{})
	for _, forbidden := range []string{
		"Execute", "Run", "Replay", "Perform", "Apply", "Invoke", "Compile",
		"Rehearse", "Send", "Press", "Click", "Type", "Lower", "Emit",
	} {
		if _, ok := rt.MethodByName(forbidden); ok {
			t.Errorf("RehearsalGrant has a %s method", forbidden)
		}
		if _, ok := reflect.PointerTo(rt).MethodByName(forbidden); ok {
			t.Errorf("*RehearsalGrant has a %s method", forbidden)
		}
	}
	forbiddenFields := []string{
		"keycode", "scancode", "rawkey", "rune", "character", "screenshot", "pixels",
		"image", "title", "label", "text", "password", "secret", "coordinate", "window",
	}
	for i := 0; i < rt.NumField(); i++ {
		name := strings.ToLower(rt.Field(i).Name)
		for _, bad := range forbiddenFields {
			if strings.Contains(name, bad) {
				t.Errorf("RehearsalGrant.%s could hold captured content", rt.Field(i).Name)
			}
		}
	}
	// The judgement too — it is the thing a report renders.
	jt := reflect.TypeOf(observe.RehearsalJudgement{})
	for i := 0; i < jt.NumField(); i++ {
		name := strings.ToLower(jt.Field(i).Name)
		for _, bad := range forbiddenFields {
			if strings.Contains(name, bad) {
				t.Errorf("RehearsalJudgement.%s could hold captured content", jt.Field(i).Name)
			}
		}
	}
}

// ── concurrency ───────────────────────────────────────────────────────────────

// Answering, cancelling, inspecting and claiming, all at once.
func TestTheAuthorizationIsRaceSafe(t *testing.T) {
	dir := t.TempDir()
	_, runner, _ := rehearsable(t, dir, verifiableScript())
	p, ok := rehearsalQuestion(runner)
	if !ok {
		t.Fatal("no rehearsal question")
	}

	var wg sync.WaitGroup
	claimed := make(chan struct{}, 64)
	for i := 0; i < 16; i++ {
		wg.Add(4)
		go func() { defer wg.Done(); runner.Respond(p.ID, observe.ResponseConfirmed) }()
		go func() { defer wg.Done(); runner.CancelCapture() }()
		go func() { defer wg.Done(); _ = runner.Grant() }()
		go func() {
			defer wg.Done()
			if g := runner.Grant(); g != nil {
				if err := g.BeginAttempt("testgame", g.Source, g.Evidence, time.Now()); err == nil {
					claimed <- struct{}{}
				}
			}
		}()
	}
	wg.Wait()
	close(claimed)

	// At most ONE claim succeeded across every goroutine. A second would be two attempts
	// authorised by one yes.
	//
	// Note what is NOT asserted: whether a grant exists at the end. Yes and cancel raced, so
	// either outcome is a correct one — the invariant is that no interleaving produces two
	// attempts, or a grant that outlives a cancellation that followed it.
	if n := len(claimed); n > 1 {
		t.Fatalf("%d concurrent claims succeeded on one authorization", n)
	}
	runner.CancelCapture()
	if g := runner.Grant(); g != nil && g.Active() {
		t.Error("an authorization survived cancellation")
	}
}
