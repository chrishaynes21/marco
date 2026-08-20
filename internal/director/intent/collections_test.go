package intent_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The parser's job here is almost entirely REFUSAL.
//
//	A phrase either says how many, or it does not parse as a collection.

func forEachOf(t *testing.T, phrase string) collections.ForEach {
	t.Helper()
	in := intent.New().Parse(phrase)
	if in.Kind != directorapi.IntentAct {
		t.Fatalf("%q was not understood: %s", phrase, in.Ambiguity)
	}
	if in.Verb != intent.VerbForEach {
		t.Fatalf("%q verb = %q, want %q", phrase, in.Verb, intent.VerbForEach)
	}
	f, ok := in.Parameters[intent.ParamForEach].(collections.ForEach)
	if !ok {
		t.Fatalf("%q carries no typed iteration: %#v", phrase, in.Parameters)
	}
	return f
}

func TestQuantifiedPhrasesBecomeBoundedIterations(t *testing.T) {
	for _, c := range []struct {
		phrase string
		verb   string
		limit  int
	}{
		{"click every selected item", "click", collections.MaximumIterations},
		{"click all matching Save buttons", "click", collections.MaximumIterations},
		{"open the first three results", "click", 3},
		{"focus every selected item", "focus", collections.MaximumIterations},
		{"click the last two tabs", "click", 2},
	} {
		f := forEachOf(t, c.phrase)
		if f.Operation.Verb != c.verb {
			t.Errorf("%q operation = %q, want %q", c.phrase, f.Operation.Verb, c.verb)
		}
		if f.Limit != c.limit {
			t.Errorf("%q limit = %d, want %d", c.phrase, f.Limit, c.limit)
		}
		if err := f.Validate(); err != nil {
			t.Errorf("%q produced an invalid iteration: %v", c.phrase, err)
		}
		// Every iteration is bounded, always.
		if f.Limit <= 0 || f.Limit > collections.MaximumIterations {
			t.Errorf("%q is not bounded: limit %d", c.phrase, f.Limit)
		}
	}
}

func TestAnUnquantifiedPluralIsNotACollection(t *testing.T) {
	// The load-bearing refusal. "Click the results" could mean one or many, and the
	// cost of guessing wrong is not one misplaced click but fifty. It stays SINGULAR,
	// so the ordinary resolver asks if it is ambiguous.
	for _, phrase := range []string{
		"click the results", "click the buttons", "click results", "click Save",
	} {
		in := intent.New().Parse(phrase)
		if in.Verb == intent.VerbForEach {
			t.Errorf("%q was silently read as a collection", phrase)
		}
	}
}

func TestVagueQuantitiesAreRefusedByName(t *testing.T) {
	// Refused by name rather than left to fall through: "I don't understand" invites
	// rephrasing the verb, while "say how many" names the actual problem. Defaulting
	// any of these to a number would act on a count nobody chose.
	for _, phrase := range []string{
		"click some items", "click several buttons", "click many results",
		"click a bunch of items", "click everything",
	} {
		in := intent.New().Parse(phrase)
		if in.Kind == directorapi.IntentAct && in.Verb == intent.VerbForEach {
			t.Errorf("%q was accepted as a bounded iteration", phrase)
		}
	}
	// And the message says what to do about it.
	in := intent.New().Parse("click some items")
	if in.Kind == directorapi.IntentAct {
		t.Fatalf("%q was understood as %q", "click some items", in.Verb)
	}
	if !strings.Contains(strings.ToLower(in.Ambiguity), "how many") {
		t.Fatalf("ambiguity = %q, want it to ask for a count", in.Ambiguity)
	}
}

func TestUnboundedIterationIsRejected(t *testing.T) {
	for _, phrase := range []string{
		"click every result forever", "click results until done",
		"click as many results as possible",
	} {
		in := intent.New().Parse(phrase)
		if in.Kind == directorapi.IntentAct && in.Verb == intent.VerbForEach {
			t.Errorf("%q produced an iteration", phrase)
		}
	}
}

func TestCapturingACollectionBindsAQueryNotMembers(t *testing.T) {
	for _, c := range []struct {
		phrase string
		name   string
		kind   collections.Kind
		take   int
	}{
		{"remember every selected item as items", "items", collections.KindTarget, 0},
		{"remember all Notepad windows as windows", "windows", collections.KindWindow, 0},
		{"remember the first three results as results", "results", collections.KindTarget, 3},
		{"remember every matching Save button as saves", "saves", collections.KindTarget, 0},
	} {
		in := intent.New().Parse(c.phrase)
		if in.Kind != directorapi.IntentAct {
			t.Errorf("%q was not understood: %s", c.phrase, in.Ambiguity)
			continue
		}
		if in.Verb != intent.VerbCaptureCollection {
			t.Errorf("%q verb = %q, want %q", c.phrase, in.Verb, intent.VerbCaptureCollection)
			continue
		}
		coll, ok := in.Parameters[intent.ParamCollection].(collections.Collection)
		if !ok {
			t.Errorf("%q carries no typed collection", c.phrase)
			continue
		}
		if coll.Name != c.name {
			t.Errorf("%q name = %q, want %q", c.phrase, coll.Name, c.name)
		}
		if coll.Kind != c.kind {
			t.Errorf("%q kind = %s, want %s", c.phrase, coll.Kind, c.kind)
		}
		if coll.Query.Take != c.take {
			t.Errorf("%q take = %d, want %d", c.phrase, coll.Query.Take, c.take)
		}
		if err := coll.Validate(); err != nil {
			t.Errorf("%q produced an invalid collection: %v", c.phrase, err)
		}
	}
}

func TestACollectionCaptureIsNotATargetVariableOrAValue(t *testing.T) {
	// Three memories now share the word "remember", and the shape of the phrase is what
	// tells them apart — not what happens to be bound already.
	set := intent.New().Parse("remember every selected item as items")
	object := intent.New().Parse("remember this button as save")
	value := intent.New().Parse("remember this field's value as email")

	if set.Verb == object.Verb || set.Verb == value.Verb {
		t.Fatalf("a set parsed as %q, the same as another memory", set.Verb)
	}
	if _, isSet := object.Parameters[intent.ParamCollection]; isSet {
		t.Error("remembering a control produced a collection")
	}
	if _, isSet := value.Parameters[intent.ParamCollection]; isSet {
		t.Error("remembering a value produced a collection")
	}
}

func TestIteratingANamedCollectionReferencesItRatherThanReparsingIt(t *testing.T) {
	f := forEachOf(t, "click each item in items")
	if f.Collection != "items" {
		t.Fatalf("collection = %q, want the named one", f.Collection)
	}
	if f.Inline != nil {
		t.Fatal("a named iteration also built an inline collection")
	}
	// An application scope is NOT a collection name.
	scoped := forEachOf(t, "click every selected item in explorer")
	if scoped.Collection != "" {
		t.Fatalf("collection = %q; \"in explorer\" scopes by application", scoped.Collection)
	}
	if scoped.Inline == nil || scoped.Inline.Query.Element.Application != "explorer" {
		t.Fatalf("inline query = %+v, want it scoped to explorer", scoped.Inline)
	}
}

func TestAWindowCollectionScopesByApplication(t *testing.T) {
	in := intent.New().Parse("remember all Notepad windows as windows")
	coll := in.Parameters[intent.ParamCollection].(collections.Collection)
	if coll.Query.Element.Application != "notepad" {
		t.Fatalf("application = %q, want notepad", coll.Query.Element.Application)
	}
	// "Notepad" names the APPLICATION, not a control label to search for.
	if coll.Query.Element.Label != "" {
		t.Fatalf("label = %q; the application name is not a control label",
			coll.Query.Element.Label)
	}
	if coll.Query.Ordering != collections.OrderingWindowZ {
		t.Fatalf("ordering = %s, want front to back", coll.Query.Ordering)
	}
	if !coll.Query.Element.AnyWindow {
		t.Fatal("a window collection did not search past the active window")
	}
}

func TestASelectedCollectionCarriesItsPredicate(t *testing.T) {
	f := forEachOf(t, "click every selected item")
	if f.Inline.Query.Selection != collections.SelectionSelected {
		t.Fatalf("selection = %q, want selected", f.Inline.Query.Selection)
	}
}

func TestTheOperationTemplateCarriesNoTarget(t *testing.T) {
	// The member is filled in per iteration. A template carrying a target would be a
	// resolved handle baked in at parse time — the exact thing collections exist to
	// avoid.
	f := forEachOf(t, "click every selected item")
	if len(f.Operation.Targets) != 0 {
		t.Fatalf("the template carries targets: %+v", f.Operation.Targets)
	}
}

func TestSingularRequestsAreUntouched(t *testing.T) {
	// The narrow rule. Everything that worked before this milestone still parses the
	// way it did.
	for _, c := range []struct{ phrase, verb string }{
		{"click Save", "click"},
		{"focus the search box", "focus"},
		{"click $save", "click"},
		{"type hello", "edit"},
		{"move window left", "move_window"},
	} {
		in := intent.New().Parse(c.phrase)
		if in.Verb != c.verb {
			t.Errorf("%q verb = %q, want %q", c.phrase, in.Verb, c.verb)
		}
	}
}
