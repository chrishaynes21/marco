package observesession

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Which question gets asked, when only one may be.
//
// # The live failure this file exists for
//
// Somebody typed "Open Mouse Settings", pressed Start, demonstrated the route, pressed Stop, and
// read "I think I got it. Want me to try?" on the panel. There was no Try button that worked and
// no question to answer, and there never would be.
//
// The interruption budget is ONE open question. Every route with a stored candidate is reviewed
// in a single pass against the same ledger, so the first eligible route takes the slot and every
// route after it is refused for budget. The order was lexicographic by subject id — deterministic
// and meaningless. With five stored routes, the one just demonstrated sorted LAST:
//
//	subj_235d87abdfc2 -> subj_892a4cc30f41
//	subj_543793ccc326 -> subj_61ffd6bc8602
//	subj_892a4cc30f41 -> subj_61ffd6bc8602
//	subj_bef5e3d29af8 -> subj_543793ccc326
//	subj_bef5e3d29af8 -> subj_892a4cc30f41   <- demonstrated seconds earlier
//
// So the slot went to a route shown an hour before, the person in front of the panel was never
// asked about the thing they had just done, and no surface said why.

// route is one relationship, briefly.
func route(from, to string) observe.RelationshipRef {
	return observe.RelationshipRef{From: from, To: to}
}

// candidatesInStoreOrder is what the store hands back: append order, oldest first.
func candidatesInStoreOrder(routes ...observe.RelationshipRef) []observe.ProcedureCandidate {
	out := make([]observe.ProcedureCandidate, 0, len(routes))
	for _, r := range routes {
		out = append(out, observe.ProcedureCandidate{Relationship: r, Application: "settings"})
	}
	return out
}

// THE regression, with the exact routes from the live store.
//
// The route demonstrated last is reviewed first, so it is the one that gets the single question.
//
// Mutation: sort by subject id. The live ordering comes back and this fails naming the route that
// lost its slot.
func TestTheRouteJustDemonstratedGetsTheQuestion(t *testing.T) {
	// Store order — oldest first. The last entry is what somebody just showed Marco.
	justDemonstrated := route("subj_bef5e3d29af8", "subj_892a4cc30f41")
	_, routes := routesByRecency(candidatesInStoreOrder(
		route("subj_543793ccc326", "subj_61ffd6bc8602"),
		route("subj_235d87abdfc2", "subj_892a4cc30f41"),
		route("subj_892a4cc30f41", "subj_61ffd6bc8602"),
		route("subj_bef5e3d29af8", "subj_543793ccc326"),
		justDemonstrated,
	))

	if len(routes) != 5 {
		t.Fatalf("%d route(s) came back, want 5", len(routes))
	}
	if routes[0] != justDemonstrated {
		t.Fatalf("the first route reviewed is %v, want the one just demonstrated (%v).\n"+
			"Only one question may be open, so the first eligible route reviewed is the "+
			"only one anybody is asked about. Whatever somebody just showed Marco is what "+
			"they are sitting in front of the panel waiting for.",
			routes[0], justDemonstrated)
	}
	// And it is genuinely the LAST under the ordering that failed live, so this fixture
	// would not pass by accident.
	lowest := routes[0]
	for _, r := range routes {
		if r.From < lowest.From || (r.From == lowest.From && r.To < lowest.To) {
			lowest = r
		}
	}
	if lowest == justDemonstrated {
		t.Error("the just-demonstrated route also sorts first by id, so this fixture cannot " +
			"tell the new ordering from the old one")
	}
}

// Every route is still reviewed, and the order is total.
//
// Recency decides who goes first; it must not drop anybody. A route that lost the slot this time
// is refused for budget and asked next time, which only works if it was reviewed at all.
func TestEveryRouteIsStillReviewed(t *testing.T) {
	all := []observe.RelationshipRef{
		route("a", "b"), route("c", "d"), route("e", "f"),
	}
	byRoute, routes := routesByRecency(candidatesInStoreOrder(all...))
	if len(routes) != len(all) || len(byRoute) != len(all) {
		t.Fatalf("%d route(s) and %d group(s) from %d candidates",
			len(routes), len(byRoute), len(all))
	}
	seen := map[observe.RelationshipRef]bool{}
	for _, r := range routes {
		if seen[r] {
			t.Errorf("%v appears twice", r)
		}
		seen[r] = true
		if len(byRoute[r]) == 0 {
			t.Errorf("%v has no candidates behind it", r)
		}
	}
	for _, r := range all {
		if !seen[r] {
			t.Errorf("%v was dropped, so it can never be asked about", r)
		}
	}
}

// Several candidates for one route order by that route's NEWEST.
//
// A route demonstrated twice — once long ago, once just now — is a route somebody is working on.
// Ordering it by its oldest example would bury it under routes nobody has touched since.
func TestARouteIsRankedByItsNewestCandidate(t *testing.T) {
	old := route("subj_aaaa", "subj_bbbb")
	other := route("subj_cccc", "subj_dddd")
	_, routes := routesByRecency(candidatesInStoreOrder(old, other, old))
	if routes[0] != old {
		t.Errorf("the first route reviewed is %v, want %v — its second demonstration is the "+
			"most recent thing in the store", routes[0], old)
	}
}

// The ordering is deterministic when nothing separates two routes.
//
// Two runs must review in the same order or the same demonstration would sometimes earn a
// question and sometimes not, which is the worst possible failure mode: intermittent.
func TestTheReviewOrderIsDeterministic(t *testing.T) {
	in := candidatesInStoreOrder(
		route("subj_bbbb", "subj_cccc"),
		route("subj_aaaa", "subj_dddd"),
		route("subj_cccc", "subj_aaaa"),
	)
	_, first := routesByRecency(in)
	for range 20 {
		_, again := routesByRecency(in)
		for i := range first {
			if first[i] != again[i] {
				t.Fatalf("two reviews of the same store disagree at %d: %v vs %v",
					i, first[i], again[i])
			}
		}
	}
}

// Nothing in, nothing out.
func TestNoCandidatesIsNoRoutes(t *testing.T) {
	byRoute, routes := routesByRecency(nil)
	if len(routes) != 0 || len(byRoute) != 0 {
		t.Errorf("%d route(s) from no candidates", len(routes))
	}
}

// An invited rehearsal question is not blocked by an unrelated open question.
//
// # The live failure
//
// Three consecutive runs ended the same way: "I think I got it. Want me to try?", with no
// question behind it. The diagnostic, once it was finally surfaced, said `another_question_open`.
//
// The interruption budget is ONE open question and it counts every kind. Review order is
// understanding first and permission LAST, so an incidental question — "are these one set?" —
// took the slot and the rehearsal question was refused. Recency ordering could not help: it only
// decides which ROUTE goes first, and the budget was already gone to a different KIND.
//
// Under teaching the reasoning inverts. The person asked for this; the incidental question is the
// interruption. See Episode.PermissionExpected.
func TestAnInvitedRehearsalQuestionIsNotBlockedByAnotherQuestion(t *testing.T) {
	policy := observe.DefaultProposalThresholds()
	if policy.MaxOpen != 1 {
		t.Fatalf("the budget is %d open question(s); this test is about it being one",
			policy.MaxOpen)
	}

	// A ledger with the slot already taken by something the person never asked about.
	taken := func() *observe.ProposalLedger {
		l := &observe.ProposalLedger{Proposals: []observe.Proposal{{
			ID: "q_incidental", Ask: observe.AskSemantic, Status: observe.ProposalOpen,
			Question: "Are these one set?",
		}}}
		return l
	}

	j := observe.RehearsalJudgement{
		Relationship: route("subj_a", "subj_b"),
		Application:  "settings", Eligible: true,
	}

	// WITHOUT the invitation: refused for budget, which is the live behaviour and correct
	// for passive observation.
	passive := taken()
	got := passive.ReviewRehearsal(j, observe.Topology{}, false, policy)
	if got.Eligible {
		t.Error("a passive session asked anyway, so the interruption bound is gone")
	}
	if !hasRefusal(got, observe.RefusalQuestionOpen) {
		t.Errorf("refused with %v, want the budget refusal", got.Refusals)
	}

	// WITH it: one extra slot, and the question is asked.
	invited := taken()
	widened := policy
	widened.MaxOpen++
	got = invited.ReviewRehearsal(j, observe.Topology{}, false, widened)
	if !got.Eligible {
		t.Fatalf("the question the person is waiting for was refused with %v.\nThey typed "+
			"what they wanted, demonstrated it, and are sitting in front of the panel; "+
			"the slot went to a question about a group nobody asked about.", got.Refusals)
	}
	if _, ok := openRehearsal(invited); !ok {
		t.Error("nothing was actually put to the person")
	}
	// And the incidental question is still there — this buys a slot, it does not evict.
	if len(invited.Proposals) != 2 {
		t.Errorf("the ledger holds %d proposal(s), want the incidental one kept alongside",
			len(invited.Proposals))
	}
}

// The extra slot is ONE, and the next route still meets the bound.
//
// Otherwise "the person asked" would become "ask about everything", which is the queue of
// interruptions the budget exists to prevent.
func TestTheInvitationBuysExactlyOneSlot(t *testing.T) {
	policy := observe.DefaultProposalThresholds()
	policy.MaxOpen++
	l := &observe.ProposalLedger{}

	first := observe.RehearsalJudgement{
		Relationship: route("subj_a", "subj_b"), Application: "settings", Eligible: true,
	}
	second := observe.RehearsalJudgement{
		Relationship: route("subj_c", "subj_d"), Application: "settings", Eligible: true,
	}
	if got := l.ReviewRehearsal(first, observe.Topology{}, false, policy); !got.Eligible {
		t.Fatalf("the first route was refused: %v", got.Refusals)
	}
	if got := l.ReviewRehearsal(second, observe.Topology{}, false, policy); !got.Eligible {
		t.Fatalf("the second route was refused: %v", got.Refusals)
	}
	third := observe.RehearsalJudgement{
		Relationship: route("subj_e", "subj_f"), Application: "settings", Eligible: true,
	}
	if got := l.ReviewRehearsal(third, observe.Topology{}, false, policy); got.Eligible {
		t.Error("a third route was asked about too; the invitation bought more than one slot")
	}
}

func hasRefusal(j observe.RehearsalJudgement, want observe.RehearsalRefusal) bool {
	for _, r := range j.Refusals {
		if r == want {
			return true
		}
	}
	return false
}

func openRehearsal(l *observe.ProposalLedger) (observe.Proposal, bool) {
	for _, p := range l.Open() {
		if p.Ask == observe.AskRehearse {
			return p, true
		}
	}
	return observe.Proposal{}, false
}

// The runner ITSELF grants the extra slot when the episode expects permission.
//
// # Why this exists beside the two above
//
// Because those call ReviewRehearsal with an already-widened policy, which proves the ledger
// honours a wider budget and says nothing about whether anything widens it. That is the shape of
// bug this repository keeps finding: complete, tested logic that the production path never
// reaches. See the memory note "prove wiring by deleting it".
//
// Deleting the `policy.MaxOpen++` in reviewRehearsal must fail this.
func TestAnInvitedRehearsalQuestionIsNotRationed(t *testing.T) {
	from, to := "subj_from", "subj_to"
	mem := &recallingMemory{known: map[string]bool{from: true, to: true}}
	store := &fixedCandidates{candidates: candidatesInStoreOrder(route(from, to))}
	// A candidate that is genuinely ELIGIBLE, because the budget check sits after the
	// eligibility gate: an ineligible candidate never reaches it and could not tell the two
	// cases apart. Known endpoints, a real arrival, complete, no text entry.
	store.candidates[0].Complete = true
	store.candidates[0].Start = observe.Checkpoint{Subject: from}
	store.candidates[0].Steps = []observe.DemonstrationStep{{
		Arrived: observe.Checkpoint{Subject: to},
		Intents: []observe.NavIntent{observe.NavConfirm},
	}}

	build := func(ep Episode) []observe.RehearsalJudgement {
		r := &Runner{
			memory: mem, candidates: store,
			policy:        observe.DefaultProposalThresholds(),
			captureBounds: observe.DefaultCaptureBounds(),
			// THE SLOT TAKEN SEVERAL TIMES OVER, by questions nobody asked about.
			// Three, because that is what a clean sandbox actually produced
			// live: one extra slot was not enough, so the invitation exempts.
			proposals: observe.ProposalLedger{Proposals: []observe.Proposal{
				{ID: "q_a", Ask: observe.AskSemantic, Status: observe.ProposalOpen},
				{ID: "q_b", Ask: observe.AskSemantic, Status: observe.ProposalOpen},
				{ID: "q_c", Ask: observe.AskSemantic, Status: observe.ProposalOpen},
			}},
		}
		return r.reviewRehearsal("settings", ep, nil)
	}

	// PASSIVE: refused for budget, which is correct and is what shipped.
	passive := build(Episode{})
	if len(passive) == 0 {
		t.Fatal("nothing was judged at all, so this fixture proves nothing about budget")
	}
	if !hasRefusal(passive[0], observe.RefusalQuestionOpen) {
		t.Errorf("a passive session was not refused for budget (%v); the interruption "+
			"bound is gone", passive[0].Refusals)
	}

	// TEACHING: asked.
	invited := build(Episode{PermissionExpected: true})
	if len(invited) == 0 {
		t.Fatal("nothing was judged")
	}
	if !invited[0].Eligible {
		t.Fatalf("the runner refused the question the person is waiting for: %v.\nThe "+
			"widening exists and reviewRehearsal does not apply it, so every live "+
			"teach ends at \"Want me to try?\" with nothing behind it.",
			invited[0].Refusals)
	}
}

// recallingMemory knows the two subjects a route needs and nothing else.
type recallingMemory struct {
	observe.Memory
	known map[string]bool
}

func (m *recallingMemory) Topology(string) observe.Topology {
	top := observe.Topology{Subjects: map[string]observe.RememberedSubject{}}
	for id := range m.known {
		top.Subjects[id] = observe.RememberedSubject{ID: id}
	}
	return top
}

// fixedCandidates is a candidate store with a fixed list.
type fixedCandidates struct {
	observe.CandidateStore
	candidates []observe.ProcedureCandidate
}

func (s *fixedCandidates) Candidates(string) []observe.ProcedureCandidate { return s.candidates }

// The exemption is spent on ONE route, not handed to every route.
//
// "The person asked for this" licenses the question about what they just demonstrated. It does
// not license a queue of questions about every route in the store, which is the interruption the
// budget exists to prevent — and which teaching, running several passes, would otherwise produce
// in bulk.
//
// Deleting `spent = true` must fail this.
func TestTheInvitationIsSpentOnOneRoute(t *testing.T) {
	newest := route("subj_new", "subj_newer")
	older := route("subj_old", "subj_older")

	mem := &recallingMemory{known: map[string]bool{
		newest.From: true, newest.To: true, older.From: true, older.To: true,
	}}
	// Store order: older first, the just-demonstrated one last.
	store := &fixedCandidates{candidates: candidatesInStoreOrder(older, newest)}
	for i := range store.candidates {
		c := &store.candidates[i]
		c.Complete = true
		c.Start = observe.Checkpoint{Subject: c.Relationship.From}
		c.Steps = []observe.DemonstrationStep{{
			Arrived: observe.Checkpoint{Subject: c.Relationship.To},
			Intents: []observe.NavIntent{observe.NavConfirm},
		}}
	}

	r := &Runner{
		memory: mem, candidates: store,
		policy:        observe.DefaultProposalThresholds(),
		captureBounds: observe.DefaultCaptureBounds(),
		proposals: observe.ProposalLedger{Proposals: []observe.Proposal{
			{ID: "q_a", Ask: observe.AskSemantic, Status: observe.ProposalOpen},
			{ID: "q_b", Ask: observe.AskSemantic, Status: observe.ProposalOpen},
		}},
	}
	judged := r.reviewRehearsal("settings", Episode{PermissionExpected: true}, nil)
	if len(judged) != 2 {
		t.Fatalf("%d route(s) judged, want 2", len(judged))
	}

	// The just-demonstrated route is reviewed first and is asked about.
	if judged[0].Relationship != newest {
		t.Fatalf("the first route reviewed is %v, want the newest (%v)",
			judged[0].Relationship, newest)
	}
	if hasRefusal(judged[0], observe.RefusalQuestionOpen) {
		t.Errorf("the invited route was still refused for budget: %v", judged[0].Refusals)
	}
	// The older one meets the ordinary bound.
	if !hasRefusal(judged[1], observe.RefusalQuestionOpen) {
		t.Errorf("a second route was also asked about (%v).\nThe invitation licenses the "+
			"question about what somebody just demonstrated, not a queue of questions "+
			"about everything in the store.", judged[1].Refusals)
	}
}

// THE exemption survives a zero policy — which is the policy production actually uses.
//
// # The live failure
//
// Four consecutive runs ended at "I think I got it, but I can't ask you for permission right now",
// with the panel reporting `another_question_open` beside `questions open: 0`.
//
// ReviewRehearsal replaces a wholly-zero policy with the defaults at its top, and the registry
// never sets ProposalPolicy — so the policy reaching this code is the zero value in production.
// The exemption was `th.MaxOpen = th.MaxProposals`, which on a zero policy is `0 = 0`: still
// wholly zero, so ReviewRehearsal substituted the defaults and reinstated the budget of one.
//
// The exemption erased itself, and only in production: every test passed a real policy and
// exempted correctly. Green tests, four failed live runs, and a diagnostic that was telling the
// exact truth the whole time.
func TestTheExemptionSurvivesAZeroPolicy(t *testing.T) {
	from, to := "subj_from", "subj_to"
	mem := &recallingMemory{known: map[string]bool{from: true, to: true}}
	store := &fixedCandidates{candidates: candidatesInStoreOrder(route(from, to))}
	c := &store.candidates[0]
	c.Complete = true
	c.Start = observe.Checkpoint{Subject: from}
	c.Steps = []observe.DemonstrationStep{{
		Arrived: observe.Checkpoint{Subject: to},
		Intents: []observe.NavIntent{observe.NavConfirm},
	}}

	r := &Runner{
		memory: mem, candidates: store,
		// THE ZERO POLICY, exactly as the registry leaves it.
		policy:        observe.ProposalThresholds{},
		captureBounds: observe.DefaultCaptureBounds(),
		proposals: observe.ProposalLedger{Proposals: []observe.Proposal{
			{ID: "q_incidental", Ask: observe.AskSemantic, Status: observe.ProposalOpen},
		}},
	}

	judged := r.reviewRehearsal("settings", Episode{PermissionExpected: true}, nil)
	if len(judged) == 0 {
		t.Fatal("nothing was judged")
	}
	if hasRefusal(judged[0], observe.RefusalQuestionOpen) {
		t.Fatalf("the invited question was refused for budget under the zero policy: %v.\n"+
			"That is the policy production uses, so the exemption never applied where it "+
			"mattered — four live runs, and every test green.", judged[0].Refusals)
	}
	if _, ok := openRehearsal(&r.proposals); !ok {
		t.Error("no rehearsal question was actually put to the person")
	}
}

// And a passive session under the zero policy is still bounded.
//
// The normalisation must not become a way for ordinary observation to acquire a wider budget: the
// one-question bound is right for somebody being interrupted while they work.
func TestAZeroPolicyStillBoundsAPassiveSession(t *testing.T) {
	from, to := "subj_from", "subj_to"
	mem := &recallingMemory{known: map[string]bool{from: true, to: true}}
	store := &fixedCandidates{candidates: candidatesInStoreOrder(route(from, to))}
	c := &store.candidates[0]
	c.Complete = true
	c.Start = observe.Checkpoint{Subject: from}
	c.Steps = []observe.DemonstrationStep{{
		Arrived: observe.Checkpoint{Subject: to},
		Intents: []observe.NavIntent{observe.NavConfirm},
	}}

	r := &Runner{
		memory: mem, candidates: store,
		policy:        observe.ProposalThresholds{},
		captureBounds: observe.DefaultCaptureBounds(),
		proposals: observe.ProposalLedger{Proposals: []observe.Proposal{
			{ID: "q_incidental", Ask: observe.AskSemantic, Status: observe.ProposalOpen},
		}},
	}

	judged := r.reviewRehearsal("settings", Episode{}, nil)
	if len(judged) == 0 {
		t.Fatal("nothing was judged")
	}
	if !hasRefusal(judged[0], observe.RefusalQuestionOpen) {
		t.Errorf("a passive session was not bounded under the zero policy: %v", judged[0].Refusals)
	}
}

// THE live regression: the exemption follows the demonstration, not the store's ordering.
//
// # What happened
//
// A teach called "Open Mouse Settings" demonstrated `subj_543793ccc326 → subj_61ffd6bc8602`, one
// click, and the session's own conclusion named that candidate. The panel then sat on "I think I
// got it, but I can't ask you for permission right now" with ZERO questions open.
//
// The pass that watched the demonstration also recorded the incidental transitions the person
// passed through on the way, and one of them was appended to the store AFTER the demonstrated
// route. Recency therefore put an incidental route first, it took the only exempt slot, and the
// route the person had actually asked about was refused `another_question_open`.
//
// The fixture is that store, in that order: the demonstrated route is NOT last.
func TestTheExemptionFollowsTheDemonstratedRoute(t *testing.T) {
	demonstrated := route("subj_543793ccc326", "subj_61ffd6bc8602")
	incidental := route("subj_bef5e3d29af8", "subj_892a4cc30f41")

	mem := &recallingMemory{known: map[string]bool{
		"subj_543793ccc326": true, "subj_61ffd6bc8602": true,
		"subj_bef5e3d29af8": true, "subj_892a4cc30f41": true,
	}}
	// Store order, oldest first: the demonstrated route, then an incidental one seen on the
	// way. Recency puts the INCIDENTAL route first, which is the whole bug.
	store := &fixedCandidates{candidates: candidatesInStoreOrder(demonstrated, incidental)}
	for i := range store.candidates {
		c := &store.candidates[i]
		c.Complete = true
		c.Start = observe.Checkpoint{Subject: c.Relationship.From}
		c.Steps = []observe.DemonstrationStep{{
			Arrived: observe.Checkpoint{Subject: c.Relationship.To},
			Intents: []observe.NavIntent{observe.NavConfirm},
		}}
	}
	if _, order := routesByRecency(store.candidates); order[0] != incidental {
		t.Fatalf("the fixture does not reproduce the live ordering: recency puts %v first, "+
			"so this proves nothing about which route gets the slot", order[0])
	}

	r := &Runner{
		memory: mem, candidates: store,
		policy:        observe.DefaultProposalThresholds(),
		captureBounds: observe.DefaultCaptureBounds(),
		// The slot already taken by questions nobody asked for — the ordinary state of
		// a teach that watched an interesting screen.
		proposals: observe.ProposalLedger{Proposals: []observe.Proposal{
			{ID: "q_a", Ask: observe.AskSemantic, Status: observe.ProposalOpen},
		}},
	}
	// The session's own conclusion, which is what the runner has three lines above the call.
	watched := store.candidates[0]
	judged := r.reviewRehearsal("settings", Episode{PermissionExpected: true}, &watched)

	var asked *observe.RehearsalJudgement
	for i := range judged {
		if judged[i].Eligible {
			asked = &judged[i]
		}
	}
	if asked == nil {
		t.Fatalf("no route was asked about at all: %+v", judged)
	}
	if asked.Relationship != demonstrated {
		t.Errorf("the exempt slot went to %v, but the person demonstrated %v.\n"+
			"Recency is a proxy for \"the one just demonstrated\" and it is wrong "+
			"whenever a pass sees more than one transition, which on a real desktop is "+
			"the normal case. The panel then says \"I can't ask you for permission right "+
			"now\" with no questions open and no way forward.",
			asked.Relationship, demonstrated)
	}
}

// A session with no demonstration still reviews every route, in recency order.
//
// The exemption is a slot, not a way to review something that has nothing behind it, and passive
// observation has no demonstration to follow.
func TestWithNoDemonstrationTheOrderIsUnchanged(t *testing.T) {
	a := route("subj_a", "subj_b")
	b := route("subj_b", "subj_c")
	_, order := routesByRecency(candidatesInStoreOrder(a, b))
	if got := demonstratedFirst(order, route("subj_x", "subj_y")); got[0] != order[0] {
		t.Errorf("a route the store has no candidate for was hoisted to the front: %v", got)
	}
	if got := demonstratedFirst(order, a); got[0] != a || len(got) != 2 || got[1] != b {
		t.Errorf("hoisting lost or reordered the rest: %v", got)
	}
}

// A GRANT FOR ONE ROUTE DOES NOT SILENCE THE QUESTION ABOUT ANOTHER.
//
// # The live deadlock
//
// A sequential edge review walks Home → Bluetooth → Mouse leg by leg. The ordinary machinery had
// already asked about the LAST leg, the person said yes to the only question on screen, and the
// review was still on the first leg. That left:
//
//	an active grant for Bluetooth → Mouse, unspendable — Marco was standing at Mouse
//	step 1 of 2: Home → Bluetooth — trying, forever
//
// Asking about leg 1 was refused `already_authorized` because SOME authority existed, and nothing
// in the lifecycle could release an authority for a leg it was not on. The step that had to go
// first could never be asked about.
//
// A grant names its route. Authorised means authorised for the route being judged.
func TestAGrantForOneRouteDoesNotSilenceTheQuestionAboutAnother(t *testing.T) {
	held := route("subj_b", "subj_c")
	needed := route("subj_a", "subj_b")

	mem := &recallingMemory{known: map[string]bool{
		held.From: true, held.To: true, needed.From: true, needed.To: true,
	}}
	store := &fixedCandidates{candidates: candidatesInStoreOrder(needed, held)}
	for i := range store.candidates {
		c := &store.candidates[i]
		c.Complete = true
		c.Start = observe.Checkpoint{Subject: c.Relationship.From}
		c.Steps = []observe.DemonstrationStep{{
			Arrived: observe.Checkpoint{Subject: c.Relationship.To},
			Intents: []observe.NavIntent{observe.NavConfirm},
		}}
	}
	grant, err := observe.NewRehearsalGrant("test", observe.RehearsalJudgement{
		Eligible: true, Application: "settings", Relationship: held,
		Source: held.From, Destination: held.To, Digest: "e", Inputs: 4,
	}, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("building the held grant: %v", err)
	}

	r := &Runner{
		memory: mem, candidates: store,
		policy:        observe.DefaultProposalThresholds(),
		captureBounds: observe.DefaultCaptureBounds(),
		grant:         grant,
		clock:         grantClock{},
	}
	judged := r.reviewRehearsal("settings", Episode{PermissionExpected: true}, nil)

	var asked bool
	for _, j := range judged {
		if j.Relationship != needed {
			continue
		}
		asked = j.Eligible
		if !asked {
			t.Errorf("the leg that has to go first was refused %v while an authority for a "+
				"DIFFERENT leg was outstanding. Nothing can release that authority, so the "+
				"review waits on a question nobody will raise.", j.Refusals)
		}
	}
	if !asked {
		t.Fatal("no question was raised about the route under review")
	}
	// And the route that IS authorised is still not asked about again.
	for _, j := range judged {
		if j.Relationship == held && j.Eligible {
			t.Error("Marco asked again about a route the person has already authorised")
		}
	}
}

// A yes about another route supersedes an authority nobody can spend.
//
// Still exactly one authority, and it still came from an explicit yes about the thing it
// authorises. A second yes about the SAME route is still refused: it queues no second attempt.
func TestAYesAboutAnotherRouteSupersedesAnUnspendableGrant(t *testing.T) {
	held := route("subj_b", "subj_c")
	wanted := route("subj_a", "subj_b")

	mem := &recallingMemory{known: map[string]bool{
		held.From: true, held.To: true, wanted.From: true, wanted.To: true,
	}}
	store := &fixedCandidates{candidates: candidatesInStoreOrder(wanted)}
	for i := range store.candidates {
		c := &store.candidates[i]
		c.Complete = true
		c.Start = observe.Checkpoint{Subject: c.Relationship.From}
		c.Steps = []observe.DemonstrationStep{{
			Arrived: observe.Checkpoint{Subject: c.Relationship.To},
			Intents: []observe.NavIntent{observe.NavConfirm},
		}}
		// Events is what a grant bounds itself by: a candidate that cost nothing cannot be
		// authorised at all, which is a different refusal from the one under test.
		c.Events = 1
	}
	grant, err := observe.NewRehearsalGrant("test", observe.RehearsalJudgement{
		Eligible: true, Application: "settings", Relationship: held,
		Source: held.From, Destination: held.To, Digest: "e", Inputs: 4,
	}, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("building the held grant: %v", err)
	}

	r := &Runner{
		memory: mem, candidates: store,
		policy:        observe.DefaultProposalThresholds(),
		captureBounds: observe.DefaultCaptureBounds(),
		grant:         grant,
		clock:         grantClock{},
	}
	// The yes must carry the evidence the question was asked about, computed the way the
	// authorising path computes it — a digest from a different assembly reads as the
	// evidence having moved, which is a real refusal and not the one under test.
	var mine []observe.ProcedureCandidate
	for _, c := range store.Candidates("settings") {
		if c.Relationship == wanted {
			mine = append(mine, c)
		}
	}
	firstC, corrC := firstAndCorroboration(mine)
	a := observe.AssessCandidate(*firstC, mem.Topology("settings"),
		observe.DefaultCaptureBounds(), corrC)
	wantedJudgement := observe.JudgeRehearsal(*firstC, a, mem.Topology("settings"), "settings")

	ref := wanted
	r.authorizeRehearsal("settings", observe.Proposal{
		ID: "r_wanted", Ask: observe.AskRehearse, Relationship: &ref,
		Evidence: wantedJudgement.Digest,
	}, observe.ResponseConfirmed)

	got := r.Grant()
	if got == nil {
		t.Fatal("no authority at all after a yes")
	}
	if got.Relationship != wanted {
		t.Fatalf("the authority is for %v, want the route just said yes to (%v). An "+
			"unspendable permission for another leg cannot be allowed to refuse this one.",
			got.Relationship, wanted)
	}
}

// grantClock is time enough to stamp a grant. Internal tests cannot reach the external suite's
// fakeClock, and a grant only needs to be able to say when it was issued.
type grantClock struct{}

func (grantClock) Now() time.Time                       { return time.Unix(1, 0) }
func (grantClock) After(time.Duration) <-chan time.Time { return nil }

// ── answers that outlive the Runner that asked ────────────────────────────────

// rehearsalRunner is a runner with real evidence for one route and no question of its own.
//
// It stands for the ordinary shape of a teach episode: a session that started AFTER a question was
// raised, so the proposal being answered was never in its ledger.
func rehearsalRunner(t *testing.T, refs ...observe.RelationshipRef) (*Runner, *recallingMemory) {
	t.Helper()
	known := map[string]bool{}
	for _, r := range refs {
		known[r.From], known[r.To] = true, true
	}
	mem := &recallingMemory{known: known}
	store := &fixedCandidates{candidates: candidatesInStoreOrder(refs...)}
	for i := range store.candidates {
		c := &store.candidates[i]
		c.Complete = true
		c.Events = 1
		c.Start = observe.Checkpoint{Subject: c.Relationship.From}
		c.Steps = []observe.DemonstrationStep{{
			Arrived: observe.Checkpoint{Subject: c.Relationship.To},
			Intents: []observe.NavIntent{observe.NavConfirm},
		}}
	}
	return &Runner{
		memory: mem, candidates: store,
		policy:        observe.DefaultProposalThresholds(),
		captureBounds: observe.DefaultCaptureBounds(),
		clock:         grantClock{},
		// ApplyAnswer reads the application from the RUNNER.s session, so a fixture
		// without one judges against an empty application and reports evidence_moved.
		session: observe.Session{Application: "settings"},
	}, mem
}

// askedAbout builds the proposal a person is answering, with the evidence the question carried.
func askedAbout(t *testing.T, r *Runner, mem *recallingMemory,
	ref observe.RelationshipRef) observe.Proposal {

	t.Helper()
	var mine []observe.ProcedureCandidate
	for _, c := range r.candidates.Candidates("settings") {
		if c.Relationship == ref {
			mine = append(mine, c)
		}
	}
	first, corr := firstAndCorroboration(mine)
	if first == nil {
		t.Fatalf("no candidate for %v", ref)
	}
	top := mem.Topology("settings")
	a := observe.AssessCandidate(*first, top, observe.DefaultCaptureBounds(), corr)
	j := observe.JudgeRehearsal(*first, a, top, "settings")
	held := ref
	return observe.Proposal{
		ID: observe.RehearsalProposalIdentity(ref.From, ref.To), Ask: observe.AskRehearse,
		Relationship: &held, Evidence: j.Digest, Status: observe.ProposalOpen,
	}
}

// A YES REACHES ITS OWN PROPOSAL THROUGH A RUNNER THAT NEVER ASKED.
//
// The proposal identifies what is being answered. Requiring it to be in the applying runner's own
// ledger made the newest Runner an authority on what an older answer meant — and it is not one.
func TestApplyAnswerNeedsNoLedgerMembership(t *testing.T) {
	wanted := route("subj_a", "subj_b")
	r, mem := rehearsalRunner(t, wanted)
	p := askedAbout(t, r, mem, wanted)
	if len(r.Proposals().Proposals) != 0 {
		t.Fatal("the runner already holds the proposal, so this proves nothing")
	}

	r.ApplyAnswer("settings", p, observe.ResponseConfirmed)

	got := r.Grant()
	if got == nil {
		t.Fatalf("a yes produced no authority (refusal: %q). The proposal says what is being "+
			"answered; the runner never had to have asked.", r.AuthorizationRefused())
	}
	if got.Relationship != wanted {
		t.Errorf("the authority is for %v, want %v", got.Relationship, wanted)
	}
}

// ONE EDGE'S YES CANNOT AUTHORISE ANOTHER EDGE.
//
// The sequential review asks leg by leg, and each leg needs its own explicit permission. A grant
// carries the route it was given for, and nothing may widen it.
func TestOneEdgesYesCannotAuthoriseAnother(t *testing.T) {
	first, second := route("subj_a", "subj_b"), route("subj_b", "subj_c")
	r, mem := rehearsalRunner(t, first, second)

	r.ApplyAnswer("settings", askedAbout(t, r, mem, first), observe.ResponseConfirmed)
	got := r.Grant()
	if got == nil {
		t.Fatalf("no authority for the first leg (refusal: %q)", r.AuthorizationRefused())
	}
	if got.Relationship == second {
		t.Fatal("a yes about the first leg authorised the second")
	}
	if got.Relationship != first {
		t.Fatalf("the authority is for %v, want %v", got.Relationship, first)
	}
}

// THE SAME YES TWICE DOES NOT CREATE TWO AUTHORITIES.
//
// Surfaces retry. A repeat about the same route is refused `already_granted`, and the authority in
// hand is the one that was already there rather than a second one queued behind it.
func TestTheSameYesTwiceCreatesOneAuthority(t *testing.T) {
	wanted := route("subj_a", "subj_b")
	r, mem := rehearsalRunner(t, wanted)
	p := askedAbout(t, r, mem, wanted)

	r.ApplyAnswer("settings", p, observe.ResponseConfirmed)
	firstGrant := r.Grant()
	if firstGrant == nil {
		t.Fatalf("no authority at all (refusal: %q)", r.AuthorizationRefused())
	}
	r.ApplyAnswer("settings", p, observe.ResponseConfirmed)
	again := r.Grant()

	if again == nil || again.ID != firstGrant.ID {
		t.Errorf("a repeated yes replaced the authority (%v then %v); a retry must be "+
			"harmless, not a second permission", firstGrant, again)
	}
	if got := r.AuthorizationRefused(); got != AuthorizationAlreadyGranted {
		t.Errorf("the repeat recorded %q, want it said to be already granted", got)
	}
}

// A GRANT REFUSAL IS ABOUT THE ROUTE IT WAS RECORDED FOR.
//
// The review asks one leg at a time and reads "why did my yes create nothing" per leg. Reporting
// the last refusal whatever leg was asked about told a reader something confident and false — and
// a wrong reason is worse than none, because it is trusted.
func TestAGrantRefusalIsAboutTheRouteItWasRecordedFor(t *testing.T) {
	first, second := route("subj_a", "subj_b"), route("subj_b", "subj_c")
	r, mem := rehearsalRunner(t, first, second)

	// A yes about the SECOND leg, twice, so a refusal is on record about that leg.
	p := askedAbout(t, r, mem, second)
	r.ApplyAnswer("settings", p, observe.ResponseConfirmed)
	r.ApplyAnswer("settings", p, observe.ResponseConfirmed)
	if r.AuthorizationRefused() == "" {
		t.Fatal("no refusal was recorded, so there is nothing to misattribute")
	}

	if why := r.AuthorizationRefusedFor(second); why == "" {
		t.Error("the leg the refusal is actually about reports no reason")
	}
	if why := r.AuthorizationRefusedFor(first); why != "" {
		t.Errorf("the OTHER leg reports %q as its reason. Nothing has been answered about "+
			"it, and a review reading this can end that leg on somebody else's refusal.", why)
	}
}

// A yes with nowhere to go still records why it created nothing.
//
// The forbidden middle is `response: yes, consequence: none, refusal: none`.
func TestAYesWithNoStoreStillRecordsWhyItCreatedNothing(t *testing.T) {
	wanted := route("subj_a", "subj_b")
	r, mem := rehearsalRunner(t, wanted)
	p := askedAbout(t, r, mem, wanted)
	bare := &Runner{clock: grantClock{}} // no memory, no candidates

	bare.ApplyAnswer("settings", p, observe.ResponseConfirmed)

	if bare.Grant() != nil {
		t.Fatal("a runner with no store produced authority")
	}
	if why := bare.AuthorizationRefusedFor(wanted); why == "" {
		t.Error("a yes that could not possibly work explained nothing. Recorded yes, no " +
			"consequence, no reason is the one state this path may never be in.")
	}
	_ = mem
}

// A YES IS JUDGED AGAINST ITS OWN APPLICATION.
//
// # The live failure
//
// `g.last` — the runner an answer's meaning is applied through — is reassigned the moment a pass
// STARTS, before its first sample sets the session's application. A yes arriving in that window
// was judged against an empty application, found no candidate for the route, and created no
// authority. The review's answer clock then ran out and told the person
// "Alright — I won't try it": their yes, reported back to them as a refusal.
//
// Measured first. The proposal's stored evidence and the current judgement digest MATCHED
// (`e57d44945da6f671`), and every route judged `eligible=true` — so the yes was not stale and not
// ineligible. What was wrong was the application it was judged in.
//
// An answer belongs to the question that caused it. The application comes from there.
func TestAYesIsJudgedAgainstItsOwnApplication(t *testing.T) {
	wanted := route("subj_a", "subj_b")
	r, mem := rehearsalRunner(t, wanted)
	p := askedAbout(t, r, mem, wanted)

	// The answering runner has NOT sampled yet: no application of its own, exactly as a
	// pass that has only just started.
	r.mu.Lock()
	r.session.Application = ""
	r.mu.Unlock()

	r.ApplyAnswer("settings", p, observe.ResponseConfirmed)

	if got := r.Grant(); got == nil {
		t.Fatalf("a yes created no authority (%q) because the runner applying it had no "+
			"application yet. The question knows which application it was about.",
			r.AuthorizationRefused())
	}
	// And with nothing supplied it still falls back to the runner's own, which is right for
	// a question that runner raised.
	r2, mem2 := rehearsalRunner(t, wanted)
	r2.ApplyAnswer("", askedAbout(t, r2, mem2, wanted), observe.ResponseConfirmed)
	if r2.Grant() == nil {
		t.Errorf("a question the runner raised itself no longer authorises (%q)",
			r2.AuthorizationRefused())
	}
}
