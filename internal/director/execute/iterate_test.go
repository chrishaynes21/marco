package execute

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Bounded iteration, driven through the real pipeline.
//
//	Every iteration re-establishes the current member against the current world.
//	Iteration advances only after the current member is verified.

// itemScene is a window of n list items, of which the first `sel` are selected.
// focusAfter is which item has keyboard focus, modelling what a real click does: focus
// lands on the control that was clicked, which is the evidence the verifier looks for.
// A fixture where nothing ever takes focus would report every click as unconfirmed and
// would be testing the failure path throughout.
func itemScene(at int, n, sel int) directorapi.WorldState {
	focusAfter := (at + 1) / 3
	items := []directorapi.Observation{
		obs("uia:1", directorapi.RoleWindow, "Files", rect(0, 0, 800, 600)),
		// A status line that changes on every observation — a clock, a byte count, the
		// sort of thing every real window has. Without SOME change between a member's
		// before and after, every click is correctly reported as having produced no
		// observable effect, and the fixture would be testing the failure path while
		// claiming to test the success path.
		obs("uia:99", directorapi.RoleText, "tick "+itoa(at), rect(0, 570, 800, 20)),
	}
	for i := 1; i <= n; i++ {
		o := obs(itemID(i), directorapi.RoleListItem,
			itemLabel(i), rect(20, 40*i, 300, 30))
		if i <= sel {
			yes := true
			o.Selected = &yes
		}
		if i == focusAfter {
			yes := true
			o.Focused = &yes
		}
		items = append(items, o)
	}
	return scene(t0.Add(time.Duration(at+1)*time.Second), nil, items...)
}

func itemID(i int) string    { return "uia:1" + itoa(i) }
func itemLabel(i int) string { return "item " + itoa(i) }

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

// itemScenes is a run of identical scenes with advancing clocks, which is what the
// verifier needs to compare a before against an after.
func itemScenes(count, n, sel int) []directorapi.WorldState {
	out := make([]directorapi.WorldState, count)
	for i := range out {
		out[i] = itemScene(i, n, sel)
	}
	return out
}

// shrinkingScenes model a set whose members DISAPPEAR as they are acted on — what
// "close every window" actually looks like. Each pair of observations drops one more.
func shrinkingScenes(count, start int) []directorapi.WorldState {
	out := make([]directorapi.WorldState, count)
	for i := range out {
		// Two observations per iteration (before and after), so the set shrinks once
		// per completed member rather than once per look.
		// The set shrinks between a member.s BEFORE and its AFTER, which is where the
		// evidence of progress has to appear.
		gone := (i + 1) / 3
		selectedLeft := start - gone
		if selectedLeft < 0 {
			selectedLeft = 0
		}
		// Two unselected items always remain, so the WINDOW stays observable after the
		// last member goes. A world emptied down to its containers is correctly reported
		// as unobservable, which would be a different test.
		out[i] = itemScene(i, selectedLeft+2, selectedLeft)
	}
	return out
}

func iterHarness(t *testing.T, worlds ...directorapi.WorldState) *harness {
	t.Helper()
	h := newHarness(worlds...)
	return h
}

func runIteration(t *testing.T, h *harness, phrase string) (Outcome, program.Context) {
	t.Helper()
	var pctx program.Context
	pctx.EnsureValues()
	pctx.EnsureCollections()
	out := h.pipeline.handleParsed(context.Background(), phrase, testIntent(phrase), pctx)
	return out, pctx
}

func TestAnIterationActsOnEveryMemberOnceAndVerifiesEach(t *testing.T) {
	// Three selected items, and the set does not change — so the processed-member
	// ledger is the only thing stopping the first member being clicked forever.
	h := iterHarness(t, itemScenes(20, 5, 3)...)

	out, _ := runIteration(t, h, "focus every selected item")
	if out.Collection == nil {
		t.Fatalf("no collection result: %s", out.Message)
	}
	res := out.Collection
	if res.State != collections.CollectionCompleted {
		t.Fatalf("state = %s (%s)", res.State, res.Reason)
	}
	if res.Completed != 3 {
		t.Fatalf("completed %d, want the 3 selected members", res.Completed)
	}
	if len(h.focuser.targets) != 3 {
		t.Fatalf("clicked %d times, want 3", len(h.focuser.targets))
	}
	// Every member is distinct: no member was processed twice.
	seen := map[string]bool{}
	for _, it := range res.Iterations {
		if seen[it.Member.Key] {
			t.Fatalf("member %q was processed twice", it.Member.Label)
		}
		seen[it.Member.Key] = true
		if it.State != collections.IterationVerified {
			t.Errorf("iteration %d = %s: %s", it.Index, it.State, it.Reason)
		}
	}
}

func TestEachIterationObservesFreshly(t *testing.T) {
	// The whole design. A stale member list would need one observation; re-resolving
	// every iteration needs one per member, plus each member's own before/after.
	h := iterHarness(t, itemScenes(30, 4, 3)...)
	runIteration(t, h, "focus every selected item")

	// 3 members × (collection observe + step observe before + observe after) at least.
	if h.observed < 9 {
		t.Fatalf("observed %d times for 3 members; each must re-resolve membership "+
			"and verify against a fresh world", h.observed)
	}
}

func TestAMemberThatDisappearsAfterItsActionIsNormal(t *testing.T) {
	// "Close every window" succeeds by making its own members vanish. The loop must
	// read that as progress rather than as members going missing.
	h := iterHarness(t, shrinkingScenes(30, 3)...)

	out, _ := runIteration(t, h, "focus every selected item")
	if out.Collection == nil {
		t.Fatalf("no collection result: %s", out.Message)
	}
	if out.Collection.State != collections.CollectionCompleted {
		t.Fatalf("state = %s (%s)", out.Collection.State, out.Collection.Reason)
	}
}

func TestAnEmptySetIsAnAnswerAndAnUnobservableOneIsNot(t *testing.T) {
	// Nothing selected: a fact, and zero iterations is the right response.
	h := iterHarness(t, itemScenes(6, 4, 0)...)
	out, _ := runIteration(t, h, "focus every selected item")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("an empty set failed: %s (%s)", out.Status, out.Message)
	}
	if out.Collection.State != collections.CollectionEmpty {
		t.Fatalf("state = %s, want empty", out.Collection.State)
	}
	if len(h.focuser.targets) != 0 {
		t.Fatal("an empty set clicked something")
	}

	// A world that could not be read is NOT an empty set.
	blind := scene(t0, nil)
	h2 := iterHarness(t, blind, blind, blind)
	out2, _ := runIteration(t, h2, "focus every selected item")
	if out2.Status == directorapi.ResultDone {
		t.Fatal("an unobservable world was reported as an empty set")
	}
	if !strings.Contains(out2.Message, "not evidence") {
		t.Fatalf("message = %q, want it to refuse the inference", out2.Message)
	}
}

func TestAFailedMemberStopsTheRestImmediately(t *testing.T) {
	// The conjunction rule one level down: member N+1 would execute against a world
	// member N was supposed to have produced.
	h := iterHarness(t, itemScenes(20, 5, 3)...)
	h.focuser.err = errors.New("the click went nowhere")

	out, _ := runIteration(t, h, "focus every selected item")
	if out.Status == directorapi.ResultDone {
		t.Fatal("a collection whose first member failed reported success")
	}
	res := out.Collection
	if res.Completed != 0 {
		t.Fatalf("completed = %d, want 0", res.Completed)
	}
	if len(res.Iterations) != 1 {
		t.Fatalf("%d iterations ran; the first failure must stop the rest",
			len(res.Iterations))
	}
	if !strings.Contains(res.Summarise(), "Stopped after 0 of") {
		t.Fatalf("summary = %q", res.Summarise())
	}
	// One attempted click, not three.
	if len(h.focuser.targets) > 1 {
		t.Fatalf("%d clicks after a failure", len(h.focuser.targets))
	}
}

func TestPartialSuccessIsNeverReportedAsSuccess(t *testing.T) {
	res := collections.Result{
		State: collections.CollectionStopped, Completed: 8, Matched: 10,
		Reason: "iteration 9 was unverified",
	}
	got := res.Summarise()
	if !strings.HasPrefix(got, "Stopped after 8 of 10") {
		t.Fatalf("summary = %q, want it to lead with the stop", got)
	}
	if strings.Contains(got, "verified") && !strings.Contains(got, "unverified") {
		t.Fatalf("summary = %q, want it not to read as success", got)
	}
}

func TestCancellationStopsBeforeTheNextMember(t *testing.T) {
	h := iterHarness(t, itemScenes(20, 5, 3)...)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var pctx program.Context
	pctx.EnsureValues()
	pctx.EnsureCollections()
	out := h.pipeline.handleParsed(ctx, "focus every selected item",
		testIntent("focus every selected item"), pctx)

	if len(h.focuser.targets) != 0 {
		t.Fatalf("%d members ran after cancellation", len(h.focuser.targets))
	}
	if out.Collection == nil || !strings.Contains(out.Collection.Reason, "Cancelled after 0") {
		t.Fatalf("result = %+v, want a cancellation report", out.Collection)
	}
}

func TestABoundedPrefixRunsExactlyThatMany(t *testing.T) {
	h := iterHarness(t, itemScenes(40, 6, 6)...)
	out, _ := runIteration(t, h, "focus the first three items")
	if out.Collection == nil {
		t.Fatalf("no result: %s", out.Message)
	}
	if out.Collection.Completed != 3 {
		t.Fatalf("completed %d, want exactly 3", out.Collection.Completed)
	}
	if len(h.focuser.targets) != 3 {
		t.Fatalf("%d clicks, want exactly 3", len(h.focuser.targets))
	}
}

func TestEachSuccessfulMemberGetsItsOwnActionGraphNode(t *testing.T) {
	// One node per member, not one node for the collection. A single node would make
	// the history unable to say which member failed.
	h := iterHarness(t, itemScenes(20, 5, 3)...)
	before := h.graph.Len()

	out, _ := runIteration(t, h, "focus every selected item")
	if got := h.graph.Len() - before; got != 3 {
		t.Fatalf("the collection produced %d nodes, want one per member", got)
	}
	for _, it := range out.Collection.Iterations {
		if it.ActionNode == "" {
			t.Errorf("iteration %d produced no node", it.Index)
		}
	}
}

func TestCapturingACollectionCreatesNoNodeAndBindsAQuery(t *testing.T) {
	h := iterHarness(t, itemScenes(10, 5, 3)...)
	before := h.graph.Len()

	out, pctx := runIteration(t, h, "remember every selected item as items")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	if h.graph.Len() != before {
		t.Fatal("a collection capture fabricated desktop-action history")
	}
	if out.Node != nil {
		t.Fatalf("a capture produced a node: %+v", out.Node)
	}

	coll, ok := pctx.Collections.Get("items")
	if !ok {
		t.Fatal("the collection was not bound")
	}
	// The QUERY is bound, and the members it saw are provenance rather than a list.
	if coll.Query.Selection != collections.SelectionSelected {
		t.Fatalf("query = %+v", coll.Query)
	}
	if coll.Provenance.MatchedAtCapture != 3 {
		t.Fatalf("matched at capture = %d, want 3", coll.Provenance.MatchedAtCapture)
	}
	raw, _ := json.Marshal(coll)
	for _, forbidden := range []string{"element_id", "hwnd", "native_id", "bounds"} {
		if strings.Contains(strings.ToLower(string(raw)), forbidden) {
			t.Fatalf("the bound collection carries %q:\n%s", forbidden, raw)
		}
	}
}

func TestIteratingANamedCollectionUsesItsStoredQuery(t *testing.T) {
	h := iterHarness(t, itemScenes(40, 5, 3)...)

	var pctx program.Context
	pctx.EnsureValues()
	pctx.EnsureCollections()

	capture := "remember every selected item as items"
	if out := h.pipeline.handleParsed(context.Background(), capture,
		testIntent(capture), pctx); out.Status != directorapi.ResultDone {
		t.Fatalf("capture: %s (%s)", out.Status, out.Message)
	}

	use := "focus each item in items"
	out := h.pipeline.handleParsed(context.Background(), use, testIntent(use), pctx)
	if out.Collection == nil {
		t.Fatalf("no result: %s", out.Message)
	}
	if out.Collection.State != collections.CollectionCompleted {
		t.Fatalf("state = %s (%s)", out.Collection.State, out.Collection.Reason)
	}
	if out.Collection.Completed != 3 {
		t.Fatalf("completed %d, want 3", out.Collection.Completed)
	}
}

func TestIteratingAnUncapturedCollectionFailsByName(t *testing.T) {
	h := iterHarness(t, itemScenes(10, 5, 3)...)
	out, _ := runIteration(t, h, "focus each item in nothing")
	if out.Status == directorapi.ResultDone {
		t.Fatal("an uncaptured collection iterated")
	}
	if !strings.Contains(out.Message, "Unknown program-local collection: nothing") {
		t.Fatalf("message = %q", out.Message)
	}
}

func TestCollectionsDisappearWhenTheProgramEnds(t *testing.T) {
	h := iterHarness(t, itemScenes(20, 5, 3)...)
	prog, err := program.Decompose("remember every selected item as items", testIntent)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	var pctx program.Context
	colls := pctx.EnsureCollections()
	h.pipeline.RunProgram(context.Background(), prog, pctx, 0)

	if !colls.Cleared() {
		t.Fatal("the collection environment outlived its program")
	}
	if colls.Has("items") {
		t.Fatal("a collection survived its program")
	}
}

// ── collection-level policy ───────────────────────────────────────────────────

func TestNoMemberRunsBeforeBulkPolicyPasses(t *testing.T) {
	// The rule the whole gate exists for. A bulk click is never silent, so NOTHING may
	// have happened by the time the refusal comes back.
	h := iterHarness(t, itemScenes(20, 5, 3)...)

	out, _ := runIteration(t, h, "click every selected item")
	if out.Status != directorapi.ResultBlocked {
		t.Fatalf("status = %s, want blocked (%s)", out.Status, out.Message)
	}
	if len(h.actuator.clicks) != 0 {
		t.Fatalf("%d members ran before the bulk decision", len(h.actuator.clicks))
	}
	if len(h.focuser.targets) != 0 {
		t.Fatal("a member was focused before the bulk decision")
	}
	if h.graph.Len() != 0 {
		t.Fatal("a refused collection recorded desktop history")
	}
	if out.Collection == nil || out.Collection.State != collections.CollectionRefused {
		t.Fatalf("result = %+v, want a refusal", out.Collection)
	}
	// The decision is kept, so a reader can see what was decided and why.
	if out.Collection.Policy == nil {
		t.Fatal("the refusal recorded no policy decision")
	}
	if out.Collection.Policy.Operation != "click" {
		t.Fatalf("policy operation = %q", out.Collection.Policy.Operation)
	}
}

func TestADestructiveBulkPhraseIsRefusedBeforeAnyMember(t *testing.T) {
	h := iterHarness(t, deleteScenes(20, 4)...)

	out, _ := runIteration(t, h, "focus every matching Delete button")
	if out.Status != directorapi.ResultBlocked {
		t.Fatalf("status = %s, want blocked (%s)", out.Status, out.Message)
	}
	if len(h.focuser.targets) != 0 {
		t.Fatalf("%d members ran despite a destructive target", len(h.focuser.targets))
	}
	if !strings.Contains(out.Message, "No items were changed") {
		t.Fatalf("message = %q, want it to state that nothing happened", out.Message)
	}
}

func TestPerMemberPolicyStillRunsAfterBulkApproval(t *testing.T) {
	// Collection-level approval is not a pre-authorisation of every future member: the
	// ordinary path still evaluates each one, which is what the observation count and
	// the per-member records show.
	h := iterHarness(t, itemScenes(30, 4, 3)...)
	out, _ := runIteration(t, h, "focus every selected item")
	if out.Collection == nil || out.Collection.State != collections.CollectionCompleted {
		t.Fatalf("result = %+v", out.Collection)
	}
	for _, it := range out.Collection.Iterations {
		if it.ActionNode == "" {
			t.Errorf("iteration %d produced no record, so no per-member path ran", it.Index)
		}
	}
}

// deleteScenes are buttons whose labels look destructive.
func deleteScenes(count, n int) []directorapi.WorldState {
	out := make([]directorapi.WorldState, count)
	for i := range out {
		items := []directorapi.Observation{
			obs("uia:1", directorapi.RoleWindow, "Files", rect(0, 0, 800, 600)),
			obs("uia:99", directorapi.RoleText, "tick "+itoa(i), rect(0, 570, 800, 20)),
		}
		for k := 1; k <= n; k++ {
			o := obs(itemID(k), directorapi.RoleButton, "Delete "+itoa(k),
				rect(20, 40*k, 300, 30))
			yes := true
			o.Selected = &yes
			items = append(items, o)
		}
		out[i] = scene(t0.Add(time.Duration(i+1)*time.Second), nil, items...)
	}
	return out
}

// ── Action Graph provenance ───────────────────────────────────────────────────

func TestEveryIterationNodeCarriesCollectionProvenance(t *testing.T) {
	h := iterHarness(t, itemScenes(30, 4, 3)...)
	out, _ := runIteration(t, h, "focus every selected item")
	if out.Collection == nil || out.Collection.Completed != 3 {
		t.Fatalf("result = %+v", out.Collection)
	}
	nodes, err := h.graph.Recent(50)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("%d nodes, want one per member", len(nodes))
	}

	digests := map[string]bool{}
	for i, n := range nodes {
		for _, key := range []string{
			MetaCollectionKind, MetaCollectionQuery, MetaCollectionOrdering,
			MetaIterationIndex, MetaIterationLimit, MetaMemberDigest,
		} {
			if _, has := n.Metadata[key]; !has {
				t.Errorf("node %d is missing %s", i, key)
			}
		}
		d, _ := n.Metadata[MetaMemberDigest].(string)
		if d == "" {
			t.Errorf("node %d has an empty member digest", i)
		}
		if digests[d] {
			t.Errorf("two nodes share a member digest: %s", d)
		}
		digests[d] = true
	}

	// The digest is a digest: it must not be the label itself, and must carry no
	// coordinates.
	raw, _ := json.Marshal(nodes)
	for _, forbidden := range []string{"member_list", "collection_snapshot"} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("the graph stored %q", forbidden)
		}
	}
}

func TestTheMemberDigestIsDeterministicAndOneWay(t *testing.T) {
	key := "some-semantic-key-with-Private Label"
	a := collections.Digest(key)
	b := collections.Digest(key)
	if a != b {
		t.Fatalf("digest is not deterministic: %q vs %q", a, b)
	}
	if a == "" {
		t.Fatal("digest is empty")
	}
	// Not reversible into the private text it came from.
	if strings.Contains(a, "Private") || strings.Contains(a, "Label") {
		t.Fatalf("the digest carries its input: %q", a)
	}
	if collections.Digest("other") == a {
		t.Fatal("two different keys share a digest")
	}
	if collections.Digest("") != "" {
		t.Fatal("an empty key produced a digest")
	}
}

func TestACollectionMemberIsNotReplayableOnItsOwn(t *testing.T) {
	h := iterHarness(t, itemScenes(30, 4, 3)...)
	runIteration(t, h, "focus every selected item")
	nodes, _ := h.graph.Recent(50)
	if len(nodes) == 0 {
		t.Fatal("no nodes to replay")
	}

	out := h.pipeline.Replay(context.Background(), ReplaySpec{Node: nodes[0], Count: 1})
	if out.Status != directorapi.ResultBlocked {
		t.Fatalf("status = %s, want blocked", out.Status)
	}
	if !strings.Contains(out.Message, "Whole-collection replay is not implemented") {
		t.Fatalf("message = %q", out.Message)
	}
	if out.Completed != 0 {
		t.Fatalf("%d iterations ran despite the refusal", out.Completed)
	}
}

func TestOrdinaryReplayIsUnaffectedByCollections(t *testing.T) {
	h := newHarness(menuFlow()...)
	first := h.pipeline.Handle(context.Background(), "click File")
	if first.Node == nil {
		t.Fatalf("no node: %s", first.Message)
	}
	if _, member := PartOfCollection(*first.Node); member {
		t.Fatal("an ordinary click was marked as a collection member")
	}
	out := h.pipeline.Replay(context.Background(), ReplaySpec{Node: *first.Node, Count: 1})
	if out.StoppedBecause == "not_replayable" {
		t.Fatalf("an ordinary action was refused: %s", out.Message)
	}
}

// ── lifecycle events ──────────────────────────────────────────────────────────

func TestCollectionLifecycleEventsAreEmittedSafely(t *testing.T) {
	h := iterHarness(t, itemScenes(30, 4, 3)...)
	tr := trace.New("cmd_1", "focus every selected item")
	h.pipeline.Trace = tr

	out, _ := runIteration(t, h, "focus every selected item")
	if out.Collection == nil || out.Collection.Completed != 3 {
		t.Fatalf("result = %+v", out.Collection)
	}

	for _, want := range []trace.ValueEventKind{
		trace.EventCollectionPolicyCompleted,
		trace.EventIterationStarted,
		trace.EventIterationCompleted,
		trace.EventCollectionCompleted,
	} {
		if tr.CountEvents(want) == 0 {
			t.Errorf("%s was never emitted", want)
		}
	}
	// One start and one completion per member.
	if n := tr.CountEvents(trace.EventIterationStarted); n != 3 {
		t.Errorf("iteration_started emitted %d times, want 3", n)
	}
	if n := tr.CountEvents(trace.EventIterationCompleted); n != 3 {
		t.Errorf("iteration_completed emitted %d times, want 3", n)
	}
	// The collection completes once.
	if n := tr.CountEvents(trace.EventCollectionCompleted); n != 1 {
		t.Errorf("collection_completed emitted %d times, want 1", n)
	}

	// No member LABEL reaches an event: the digest is what identifies a member.
	raw, err := json.Marshal(tr.ValueEvents())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for i := 1; i <= 3; i++ {
		if strings.Contains(string(raw), itemLabel(i)) {
			t.Fatalf("an event carried a member label (%q):\n%s", itemLabel(i), raw)
		}
	}
	if !strings.Contains(string(raw), "member_digest") {
		t.Fatal("no event identified its member at all")
	}
}

func TestARefusedCollectionRecordsItsPolicyDecisionAsAnEvent(t *testing.T) {
	h := iterHarness(t, itemScenes(20, 5, 3)...)
	tr := trace.New("cmd_1", "click every selected item")
	h.pipeline.Trace = tr

	runIteration(t, h, "click every selected item")

	if tr.CountEvents(trace.EventCollectionPolicyCompleted) != 1 {
		t.Fatal("a refused collection recorded no policy decision")
	}
	if tr.CountEvents(trace.EventIterationStarted) != 0 {
		t.Fatal("an iteration started despite the refusal")
	}
	for _, e := range tr.ValueEvents() {
		if e.Kind != trace.EventCollectionPolicyCompleted {
			continue
		}
		if e.Outcome != "needs_confirmation" && e.Outcome != "refused" {
			t.Errorf("policy event outcome = %q", e.Outcome)
		}
	}
}

func TestTheCollectionClearedEventIsEmittedOncePerProgram(t *testing.T) {
	h := iterHarness(t, itemScenes(20, 5, 3)...)
	tr := trace.New("cmd_1", "capture")
	h.pipeline.Trace = tr

	prog, err := program.Decompose("remember every selected item as items", testIntent)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	h.pipeline.RunProgram(context.Background(), prog, program.Context{}, 0)

	if n := tr.CountEvents(trace.EventCollectionCleared); n != 1 {
		t.Fatalf("collection_cleared emitted %d times, want exactly 1", n)
	}
}

// ── clarification pause and resume ────────────────────────────────────────────

// ambiguousMemberScenes are items of which the LAST is ambiguous: two controls share
// its label, so resolving that one member asks.
func ambiguousMemberScenes(count int) []directorapi.WorldState {
	out := make([]directorapi.WorldState, count)
	for i := range out {
		focusAfter := (i + 1) / 3
		items := []directorapi.Observation{
			obs("uia:1", directorapi.RoleWindow, "Files", rect(0, 0, 800, 600)),
			obs("uia:99", directorapi.RoleText, "tick "+itoa(i), rect(0, 570, 800, 20)),
		}
		for k := 1; k <= 2; k++ {
			o := obs(itemID(k), directorapi.RoleListItem, itemLabel(k), rect(20, 40*k, 300, 30))
			yes := true
			o.Selected = &yes
			if k == focusAfter {
				o.Focused = &yes
			}
			items = append(items, o)
		}
		// The third member's label is shared by two equally good controls, so its own
		// resolution is ambiguous while the collection's membership is not.
		yes := true
		a := obs("uia:31", directorapi.RoleListItem, "item 3", rect(20, 120, 300, 30))
		a.Selected = &yes
		b := obs("uia:32", directorapi.RoleListItem, "item 3", rect(400, 120, 300, 30))
		b.Selected = &yes
		items = append(items, a, b)
		out[i] = scene(t0.Add(time.Duration(i+1)*time.Second), nil, items...)
	}
	return out
}

func TestAnAmbiguousMemberPausesRatherThanStopping(t *testing.T) {
	h := iterHarness(t, ambiguousMemberScenes(40)...)
	tr := trace.New("cmd_1", "focus every selected item")
	h.pipeline.Trace = tr

	out, pctx := runIteration(t, h, "focus every selected item")
	if out.Status != directorapi.ResultNeedsClarification {
		t.Fatalf("status = %s, want needs_clarification (%s)", out.Status, out.Message)
	}
	res := out.Collection
	if res.State != collections.CollectionAwaitingClarification {
		t.Fatalf("state = %s, want awaiting_clarification", res.State)
	}
	// Two members verified before the pause, and the pause records its REAL position.
	if res.Completed != 2 {
		t.Fatalf("completed = %d, want the 2 that verified first", res.Completed)
	}
	if res.PausedAt != 3 {
		t.Fatalf("paused at %d, want the real position 3", res.PausedAt)
	}
	// The processed ledger survives, which is what stops members 1 and 2 rerunning.
	if n := pctx.Collections.ProcessedCount("inline:focus every selected item"); n != 2 {
		t.Fatalf("processed ledger holds %d, want 2", n)
	}
	// Authoritative contenders are offered.
	if out.Resolution == nil || len(out.Resolution.Contenders) < 2 {
		t.Fatalf("no contenders offered: %+v", out.Resolution)
	}
	if tr.CountEvents(trace.EventCollectionPaused) != 1 {
		t.Fatal("no collection_paused event")
	}
}

func TestResumingRunsOnlyTheAmbiguousMember(t *testing.T) {
	h := iterHarness(t, ambiguousMemberScenes(60)...)
	tr := trace.New("cmd_1", "focus every selected item")
	h.pipeline.Trace = tr

	phrase := "focus every selected item"
	var pctx program.Context
	pctx.EnsureValues()
	pctx.EnsureCollections()

	first := h.pipeline.handleParsed(context.Background(), phrase, testIntent(phrase), pctx)
	if first.Collection == nil || first.Collection.PausedAt != 3 {
		t.Fatalf("expected a pause at 3: %+v", first.Collection)
	}
	before := len(h.focuser.targets)
	if before != 2 {
		t.Fatalf("%d members ran before the pause, want 2", before)
	}

	// The answer arrives — from a different client in production, which is the same
	// thing as a second call here: the state it needs is on the retained context.
	pctx.CollectionResume = &program.CollectionResume{
		Ledger: "inline:" + phrase, Iteration: 3, Ordinal: 1,
	}
	second := h.pipeline.handleParsed(context.Background(), phrase, testIntent(phrase), pctx)

	if second.Collection == nil {
		t.Fatalf("no result on resume: %s", second.Message)
	}
	// Members 1 and 2 are NOT repeated: the ledger already holds their keys.
	ran := len(h.focuser.targets) - before
	if ran > 2 {
		t.Fatalf("%d members ran on resume; the completed ones were repeated", ran)
	}
	// The answer was actually APPLIED. Without it the member stays ambiguous and pauses
	// again — which is what a resume that only restored the ledger would do, and is why
	// this assertion is here rather than a count check alone.
	if second.Collection.State == collections.CollectionAwaitingClarification {
		t.Fatalf("the member paused again, so the clarification was never applied: %s",
			second.Collection.Reason)
	}
	if ran == 0 {
		t.Fatal("the resumed member never executed")
	}
	// The resume reports the real position rather than counting from one.
	if tr.CountEvents(trace.EventCollectionResumed) != 1 {
		t.Fatal("no collection_resumed event")
	}
	for _, e := range tr.ValueEvents() {
		if e.Kind == trace.EventCollectionResumed && e.Iteration != 3 {
			t.Errorf("resumed at iteration %d, want 3", e.Iteration)
		}
	}
}

func TestTheOrdinalNeverEntersTheStoredQuery(t *testing.T) {
	// The ordinal refers to the contender list of ONE event. Writing it into the
	// collection's query would make every later member pick the same contender.
	h := iterHarness(t, ambiguousMemberScenes(40)...)

	var pctx program.Context
	pctx.EnsureValues()
	pctx.EnsureCollections()
	capture := "remember every selected item as items"
	if o := h.pipeline.handleParsed(context.Background(), capture,
		testIntent(capture), pctx); o.Status != directorapi.ResultDone {
		t.Fatalf("capture: %s (%s)", o.Status, o.Message)
	}

	pctx.CollectionResume = &program.CollectionResume{
		Ledger: "items", Iteration: 1, Ordinal: 2,
	}
	use := "focus each item in items"
	h.pipeline.handleParsed(context.Background(), use, testIntent(use), pctx)

	stored, ok := pctx.Collections.Get("items")
	if !ok {
		t.Fatal("the collection vanished")
	}
	if stored.Query.Element.Ordinal != 0 {
		t.Fatalf("the ordinal was written into the stored query: %d",
			stored.Query.Element.Ordinal)
	}
	raw, _ := json.Marshal(stored)
	if strings.Contains(string(raw), `"ordinal"`) {
		t.Fatalf("the stored collection carries an ordinal:\n%s", raw)
	}
}

func TestAResumeAnswerIsConsumedByExactlyOneMember(t *testing.T) {
	// Narrowing every subsequent member by an answer that was about one of them would
	// silently redirect the rest of the collection.
	resume := &program.CollectionResume{Ledger: "items", Iteration: 3, Ordinal: 2}
	q := &directorapi.ElementQuery{Label: "item"}
	resume.Narrow(q)
	if q.Ordinal != 2 {
		t.Fatalf("the answer did not narrow the member: %+v", q)
	}
	if !resume.Applies("items") || resume.Applies("other") {
		t.Fatal("the answer applied to the wrong collection")
	}
	// A nil resume narrows nothing and claims nothing.
	var none *program.CollectionResume
	fresh := &directorapi.ElementQuery{Label: "item"}
	none.Narrow(fresh)
	if fresh.Ordinal != 0 || none.Applies("items") {
		t.Fatal("an absent answer narrowed something")
	}
}

// ── the no-progress guard ─────────────────────────────────────────────────────

// staticScenes never change: the operation runs and the world does not move. A loop
// without a no-progress guard applies the same ineffective action until the limit.
func staticScenes(count, n int) []directorapi.WorldState {
	out := make([]directorapi.WorldState, count)
	for i := range out {
		items := []directorapi.Observation{
			obs("uia:1", directorapi.RoleWindow, "Files", rect(0, 0, 800, 600)),
			obs("uia:99", directorapi.RoleText, "tick "+itoa(i), rect(0, 570, 800, 20)),
		}
		for k := 1; k <= n; k++ {
			o := obs(itemID(k), directorapi.RoleListItem, itemLabel(k), rect(20, 40*k, 300, 30))
			yes := true
			o.Selected = &yes
			items = append(items, o)
		}
		// No item EVER takes focus, so a focus operation can never be verified as
		// having moved anything.
		out[i] = scene(t0.Add(time.Duration(i+1)*time.Second), nil, items...)
	}
	return out
}

func TestAMemberThatMakesNoProgressStopsTheCollectionAfterOneAttempt(t *testing.T) {
	h := iterHarness(t, staticScenes(60, 3)...)

	out, pctx := runIteration(t, h, "focus every selected item")
	if out.Status == directorapi.ResultDone {
		t.Fatalf("a collection that made no progress reported success: %s", out.Message)
	}
	res := out.Collection
	if res.Completed != 0 {
		t.Fatalf("completed = %d, want 0", res.Completed)
	}
	// ONE attempt. The guard's entire purpose is that the same member is not retried.
	if len(h.focuser.targets) > 1 {
		t.Fatalf("the member was acted on %d times; a no-progress member must not repeat",
			len(h.focuser.targets))
	}
	if len(res.Iterations) != 1 {
		t.Fatalf("%d iterations ran, want 1", len(res.Iterations))
	}
	// It is NOT marked processed: claiming it would record work that did not happen.
	if n := pctx.Collections.ProcessedCount("inline:focus every selected item"); n != 0 {
		t.Fatalf("processed ledger holds %d after no progress", n)
	}
	if !strings.Contains(res.Reason, "no verified progress for the current member") {
		t.Fatalf("reason = %q", res.Reason)
	}
}

func TestVerifiedMembersCarryTheirProgressClassification(t *testing.T) {
	h := iterHarness(t, itemScenes(30, 4, 3)...)
	out, _ := runIteration(t, h, "focus every selected item")
	if out.Collection == nil || out.Collection.Completed != 3 {
		t.Fatalf("result = %+v", out.Collection)
	}
	for _, it := range out.Collection.Iterations {
		if it.Progress == "" {
			t.Errorf("iteration %d carries no progress classification", it.Index)
		}
		if !it.Progress.Advances() {
			t.Errorf("iteration %d verified but classified %s", it.Index, it.Progress)
		}
	}
	// The fixture models focus landing on the member, which is a state change.
	if got := out.Collection.Iterations[0].Progress; got != collections.ProgressMemberStateChanged {
		t.Fatalf("progress = %s, want member_state_changed", got)
	}
}

// ── membership drift across a pause ───────────────────────────────────────────

func TestAStaleOrdinalIsRefusedWhenTheChoicesChanged(t *testing.T) {
	// The governing case. The user is offered two contenders, thinks, and answers "the
	// first one" — while a new contender has appeared at the top. Applying the ordinal
	// would select a control they were never shown.
	h := iterHarness(t, ambiguousMemberScenes(40)...)
	phrase := "focus every selected item"

	var pctx program.Context
	pctx.EnsureValues()
	pctx.EnsureCollections()
	first := h.pipeline.handleParsed(context.Background(), phrase, testIntent(phrase), pctx)
	if first.Collection == nil || first.Collection.PausedAt != 3 {
		t.Fatalf("expected a pause: %+v", first.Collection)
	}
	if first.Collection.EventID == "" {
		t.Fatal("the pause recorded no clarification event id")
	}
	if len(first.Collection.Offered.OrderedKeyDigests) == 0 {
		t.Fatal("the pause recorded no offered fingerprint")
	}
	before := len(h.focuser.targets)

	// The answer arrives, but it refers to an offer that no longer describes the world:
	// the fingerprint is deliberately one the current membership cannot match.
	pctx.CollectionResume = &program.CollectionResume{
		Ledger: "inline:" + phrase, Iteration: 3, Ordinal: 1,
		EventID: first.Collection.EventID,
		// Same QUESTION, different ordered contenders: the world moved while the user
		// was thinking, which is exactly when an ordinal stops meaning what it meant.
		Offered: collections.MembershipFingerprint{
			QueryDigest:       first.Collection.Offered.QueryDigest,
			OrderedKeyDigests: []string{"a-contender-that-is-gone", "and-another"},
			MatchedCount:      2,
		},
	}
	second := h.pipeline.handleParsed(context.Background(), phrase, testIntent(phrase), pctx)

	if second.Status == directorapi.ResultDone {
		t.Fatalf("a stale answer was honoured: %s", second.Message)
	}
	if len(h.focuser.targets) != before {
		t.Fatalf("%d members ran under a stale answer", len(h.focuser.targets)-before)
	}
	// Completed members are NOT lost: this is a refusal to resume, not a restart.
	if second.Collection == nil {
		t.Fatal("no collection result")
	}
}
