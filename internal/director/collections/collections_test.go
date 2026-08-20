package collections_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The governing rules, as tests.
//
//	A collection is a bounded semantic QUERY over the current world.
//	Collections store semantic members, not resolved pixels.
//	Iteration is always bounded. Empty is not unobservable.

func query(role directorapi.ElementRole, label string) collections.Query {
	return collections.Query{
		Element:  directorapi.ElementQuery{Role: role, Label: label},
		Ordering: collections.OrderingVisual,
		Limit:    collections.MaximumItems,
	}
}

func TestACollectionStoresAQueryAndNeverResolvedIdentity(t *testing.T) {
	// The whole design in one assertion. A serialised collection must contain nothing
	// that could be replayed against a screen that has since moved.
	c := collections.Collection{
		Name: "items", Kind: collections.KindTarget, Query: query("", "result"),
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("a valid collection was refused: %v", err)
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{
		"element_id", "hwnd", "runtime_id", "native_id", "bounds", "point",
		"coordinates", "ordinal_index",
	} {
		if strings.Contains(strings.ToLower(string(raw)), forbidden) {
			t.Errorf("a collection carries %q — membership is a query, not a handle:\n%s",
				forbidden, raw)
		}
	}
}

func TestACollectionMustBeBoundedAndOrdered(t *testing.T) {
	// Unbounded iteration and unordered iteration are both refused at the model level,
	// so no caller can construct one and discover the problem at execution.
	unbounded := query("", "x")
	unbounded.Limit = 0
	if err := unbounded.Validate(); err == nil {
		t.Error("an unbounded collection was accepted")
	}

	unordered := query("", "x")
	unordered.Ordering = ""
	if err := unordered.Validate(); err == nil {
		t.Error("an unordered collection was accepted")
	}

	huge := query("", "x")
	huge.Limit = collections.MaximumItems + 1
	if err := huge.Validate(); err == nil {
		t.Error("a limit above the maximum was accepted")
	}

	// And a collection has to describe SOMETHING: "act on everything on screen" is
	// never a request worth guessing at.
	empty := collections.Query{Ordering: collections.OrderingVisual, Limit: 10}
	if err := empty.Validate(); err == nil {
		t.Error("an unconstrained collection was accepted")
	}
}

func TestABoundedPrefixIsNotATruncation(t *testing.T) {
	// "The first three" asks for three. A LIMIT being exceeded is a refusal. The two
	// look similar and behave oppositely, which is why they are separate fields.
	w := listWorld(t, 6)
	prefix := query("", "result")
	prefix.Take = 3

	res := collections.Resolve(&w, prefix, rank)
	if res.Status != collections.StatusResolved {
		t.Fatalf("status = %s (%s)", res.Status, res.Explanation)
	}
	if len(res.Members) != 3 {
		t.Fatalf("got %d members, want the 3 that were asked for", len(res.Members))
	}
	// Matched reports the truth about the world, not the size of the answer.
	if res.Matched != 6 {
		t.Fatalf("matched = %d, want the 6 that exist", res.Matched)
	}

	// Without a Take, exceeding the limit REJECTS rather than silently taking a prefix.
	limited := query("", "result")
	limited.Limit = 4
	over := collections.Resolve(&w, limited, rank)
	if over.Status != collections.StatusOverLimit {
		t.Fatalf("status = %s, want over_limit", over.Status)
	}
	if !strings.Contains(over.Explanation, "exceeding the limit of 4") {
		t.Fatalf("explanation = %q", over.Explanation)
	}
	if len(over.Members) != 0 {
		t.Fatal("an over-limit collection returned members anyway")
	}
}

func TestEmptyAndUnobservableAreDifferentAnswers(t *testing.T) {
	// "Nothing is selected" is evidence the user can act on. "I cannot see whether
	// anything is selected" is the absence of evidence. Collapsing them would report
	// an empty result for an application the Director could not read.
	readable := listWorld(t, 3)
	res := collections.Resolve(&readable, query("", "nothing-matches-this"), rank)
	if res.Status != collections.StatusEmpty {
		t.Fatalf("a readable world with no matches gave %s, want empty", res.Status)
	}

	blind := directorapi.WorldState{}
	blindRes := collections.Resolve(&blind, query("", "result"), rank)
	if blindRes.Status != collections.StatusUnobservable {
		t.Fatalf("an unreadable world gave %s, want unobservable", blindRes.Status)
	}
	if !strings.Contains(blindRes.Explanation, "not evidence") {
		t.Fatalf("explanation = %q, want it to refuse the inference", blindRes.Explanation)
	}
}

func TestOrderingIsDeterministicAcrossRuns(t *testing.T) {
	// A sort leaving equal elements in map order would iterate differently on
	// consecutive runs against an identical screen.
	w := listWorld(t, 8)
	var first []string
	for run := 0; run < 5; run++ {
		res := collections.Resolve(&w, query("", "result"), rank)
		var got []string
		for _, m := range res.Members {
			got = append(got, m.Label)
		}
		if run == 0 {
			first = got
			continue
		}
		if strings.Join(got, "|") != strings.Join(first, "|") {
			t.Fatalf("run %d ordered %v, run 0 ordered %v", run, got, first)
		}
	}
	// Visual order is top to bottom: the fixture stacks them, so the labels come back
	// in their created order.
	if first[0] != "result 1" || first[len(first)-1] != "result 8" {
		t.Fatalf("visual order = %v, want top to bottom", first)
	}
}

func TestVisualOrderReadsRowsLeftToRight(t *testing.T) {
	// Controls on the same row must be read left to right rather than by a one-pixel
	// difference in their tops — which is what a person means by "the first three".
	w := rowWorld(t)
	res := collections.Resolve(&w, query("", "cell"), rank)
	var got []string
	for _, m := range res.Members {
		got = append(got, m.Label)
	}
	want := "cell a|cell b|cell c|cell d"
	if strings.Join(got, "|") != want {
		t.Fatalf("order = %v, want %s", got, want)
	}
}

func TestASemanticKeySurvivesMovementButDistinguishesMembers(t *testing.T) {
	// The key must not rely on coordinates: a list that reflows after its first item
	// is deleted gives every remaining member a new position, and a positional key
	// would present them all as new.
	q := query("", "result")
	moved := collections.Member{
		Application: "explorer", Role: directorapi.RoleListItem,
		Label: "Report.docx", NativeID: "uia:7",
	}
	same := moved
	// Everything about where it is has changed; nothing about what it is has.
	if collections.SemanticKey(moved, q) != collections.SemanticKey(same, q) {
		t.Fatal("a member that only moved got a new identity")
	}

	other := moved
	other.Label = "Budget.xlsx"
	other.NativeID = "uia:9"
	if collections.SemanticKey(moved, q) == collections.SemanticKey(other, q) {
		t.Fatal("two different members share an identity")
	}

	// The same control reached through a DIFFERENT collection is not confused with
	// this one, because the query is part of the identity.
	if collections.SemanticKey(moved, q) == collections.SemanticKey(moved, query("", "other")) {
		t.Fatal("the same member has one identity across unrelated collections")
	}
	// Case and stray whitespace are not identity.
	spaced := moved
	spaced.Label = "  report.DOCX  "
	if collections.SemanticKey(moved, q) != collections.SemanticKey(spaced, q) {
		t.Fatal("whitespace and case changed a member's identity")
	}
}

func TestAMemberWithNoDurableIdentityIsRefused(t *testing.T) {
	// Without a native id or a label, a member is indistinguishable from its siblings,
	// and processing the set would either skip members or act on one twice. There is
	// no safe guess, so the caller stops.
	anonymous := collections.Member{Application: "app", Role: directorapi.RoleButton}
	if anonymous.Durable() {
		t.Fatal("a member with neither an id nor a label claimed durable identity")
	}
	labelled := anonymous
	labelled.Label = "Save"
	if !labelled.Durable() {
		t.Fatal("a labelled member was called undurable")
	}
}

// ── the environment ───────────────────────────────────────────────────────────

func TestACollectionCannotBeRebound(t *testing.T) {
	env := collections.NewEnvironment()
	c := collections.Collection{Name: "items", Kind: collections.KindTarget, Query: query("", "result")}
	if err := env.Bind(c); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := env.Bind(c); err == nil {
		t.Fatal("a collection was rebound")
	}
}

func TestProcessedMembersAreTrackedSemantically(t *testing.T) {
	env := collections.NewEnvironment()
	_ = env.Bind(collections.Collection{
		Name: "items", Kind: collections.KindTarget, Query: query("", "result"),
	})
	env.MarkProcessed("items", "key-a")
	env.MarkProcessed("items", "key-a") // idempotent

	if got := env.Processed("items"); len(got) != 1 {
		t.Fatalf("processed = %v, want one entry", got)
	}
	if !env.WasProcessed("items", "key-a") {
		t.Fatal("a processed member was not remembered")
	}
	if env.WasProcessed("items", "key-b") {
		t.Fatal("an unprocessed member was reported as done")
	}
}

func TestAnEnvironmentEndsWithItsProgram(t *testing.T) {
	env := collections.NewEnvironment()
	_ = env.Bind(collections.Collection{
		Name: "items", Kind: collections.KindTarget, Query: query("", "result"),
	})
	env.MarkProcessed("items", "key-a")

	if n := env.Clear(); n != 1 {
		t.Fatalf("Clear discarded %d, want 1", n)
	}
	if !env.Cleared() || env.Count() != 0 {
		t.Fatal("the environment did not report itself finished")
	}
	// The processed history goes too: keeping it would let a later program's identical
	// collection think its members had already been handled.
	if got := env.Processed("items"); len(got) != 0 {
		t.Fatalf("processed history survived: %v", got)
	}
	if _, err := env.Resolve("items"); err == nil {
		t.Fatal("a collection survived its program")
	}
	err := (&collections.ErrUnknownCollection{Name: "items"}).Error()
	if !strings.Contains(err, "Unknown program-local collection: items") {
		t.Fatalf("message = %q", err)
	}
	// A cleared environment is not quietly reusable.
	if err := env.Bind(collections.Collection{
		Name: "more", Kind: collections.KindTarget, Query: query("", "x"),
	}); err == nil {
		t.Fatal("a finished program accepted a new collection")
	}
}

func TestASnapshotCarriesNoMemberIdentity(t *testing.T) {
	env := collections.NewEnvironment()
	_ = env.Bind(collections.Collection{
		Name: "items", Kind: collections.KindTarget, Query: query("", "result"),
		Provenance: collections.Provenance{Application: "explorer", MatchedAtCapture: 7},
	})
	env.MarkProcessed("items", "deadbeefdeadbeef")

	snap := env.Describe()
	got, ok := snap.Find("items")
	if !ok {
		t.Fatal("the collection is missing from the snapshot")
	}
	if got.ProcessedCount != 1 {
		t.Fatalf("processed count = %d", got.ProcessedCount)
	}
	raw, _ := json.Marshal(snap)
	// The COUNT is reported, never the keys: a key is an opaque digest that means
	// nothing to a reader and would only invite something to try matching on it.
	if strings.Contains(string(raw), "deadbeefdeadbeef") {
		t.Fatalf("the snapshot exposed a member key:\n%s", raw)
	}
	if !strings.Contains(string(raw), "explorer") {
		t.Fatal("the snapshot dropped the application, which is safe and useful")
	}
}
