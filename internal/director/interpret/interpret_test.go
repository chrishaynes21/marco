package interpret_test

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/interpret"
	"github.com/chaynes-simpleclouds/marco/internal/resolver"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// What a model is allowed to turn a sentence into, and the seven things it cannot.
//
// Every test here drives the production `Interpreter.Intent` and substitutes only the plugin
// boundary, because the validation is the whole of what this package is. A test that substituted
// the validation would be testing nothing.

// answering is an Asker that returns one fixed reading, and records what it was told.
func answering(s resolver.Semantic, ok bool) (interpret.Asker, *[]string) {
	var sawInput []string
	return func(_ context.Context, input string, verbs []string) (resolver.Semantic, bool) {
		sawInput = append(sawInput, input)
		_ = verbs
		return s, ok
	}, &sawInput
}

// ── the affirmative case ──────────────────────────────────────────────────────

// A SENTENCE THE PARSER CANNOT READ BECOMES AN ORDINARY TYPED OPERATION.
//
// The headline. "Hit the save button would you" is not a phrasing `uiact.Parse` knows, and what
// comes out of here is byte-for-byte the shape "press Save" would have produced: one semantic
// verb, one target carrying the user's own phrase, and a query for the ordinary resolver.
func TestASentenceBecomesAnOrdinaryTypedOperation(t *testing.T) {
	ask, _ := answering(resolver.Semantic{Kind: "invoke", Target: "Save"}, true)
	in, ok := interpret.With(ask).Intent(context.Background(), "hit the save button would you")
	if !ok {
		t.Fatal("nothing was interpreted")
	}
	if in.Kind != directorapi.IntentAct {
		t.Errorf("Kind = %q, want act", in.Kind)
	}
	if in.Verb != intent.SemanticVerb {
		t.Errorf("Verb = %q, want the one semantic verb", in.Verb)
	}
	kind, known := intent.SemanticKindOf(in)
	if !known || kind != directorapi.SemanticInvoke {
		t.Errorf("the semantic kind read back as %q (known=%v), want invoke", kind, known)
	}
	if len(in.Targets) != 1 {
		t.Fatalf("%d target(s), want 1", len(in.Targets))
	}
	ref := in.Targets[0]
	if ref.Phrase != "Save" {
		t.Errorf("the target phrase is %q, want the user's own words", ref.Phrase)
	}
	if ref.Query == nil || ref.Query.Label != "Save" {
		t.Errorf("the target carries no label query: %+v", ref.Query)
	}
	// AND THE RAW REQUEST TRAVELS, because a clarification has to quote the words back.
	if in.Raw != "hit the save button would you" {
		t.Errorf("Raw = %q, want the sentence as typed", in.Raw)
	}
}

// AND IT IS THE SAME SHAPE THE DETERMINISTIC PARSER PRODUCES.
//
// Not "similar" — identical. Both go through `intent.SemanticIntent`, so there is one construction
// and the only difference between a parsed reading and an interpreted one is the confidence, which
// nothing downstream gates on.
//
// This is the test that makes "a model cannot produce a shape the parser could not" mean something.
func TestAnInterpretedIntentHasTheSameShapeAsAParsedOne(t *testing.T) {
	parsed := intent.New().Parse("press Save")
	if parsed.Kind != directorapi.IntentAct {
		t.Fatalf("the deterministic parser did not read `press Save`: %+v", parsed)
	}

	ask, _ := answering(resolver.Semantic{Kind: "invoke", Target: "Save"}, true)
	interpreted, ok := interpret.With(ask).Intent(context.Background(), "press Save")
	if !ok {
		t.Fatal("nothing was interpreted")
	}

	if parsed.Verb != interpreted.Verb {
		t.Errorf("verbs differ: parsed %q, interpreted %q", parsed.Verb, interpreted.Verb)
	}
	pk, _ := intent.SemanticKindOf(parsed)
	ik, _ := intent.SemanticKindOf(interpreted)
	if pk != ik {
		t.Errorf("semantic kinds differ: parsed %q, interpreted %q", pk, ik)
	}
	if len(parsed.Targets) != len(interpreted.Targets) {
		t.Fatalf("target counts differ: parsed %d, interpreted %d",
			len(parsed.Targets), len(interpreted.Targets))
	}
	if parsed.Targets[0].Kind != interpreted.Targets[0].Kind {
		t.Errorf("reference kinds differ: parsed %q, interpreted %q",
			parsed.Targets[0].Kind, interpreted.Targets[0].Kind)
	}
	// AND THE CONFIDENCE IS LOWER, which is the one field that should differ: it is a weaker
	// kind of evidence about what somebody meant.
	if interpreted.Confidence >= parsed.Confidence {
		t.Errorf("an interpreted reading claims %v against the parser's %v; it is the "+
			"weaker evidence and must say so", interpreted.Confidence, parsed.Confidence)
	}
}

// ── what it cannot do ─────────────────────────────────────────────────────────

// AN INVENTED VERB IS REFUSED.
//
// The one thing a model answer could otherwise do that nothing else in this system can: name an
// operation that does not exist. There are thirty-three words it may say.
//
// A verb that got through would be refused at execution time instead — after the person had been
// told Marco understood them, and after a plan had been built around a ladder that is not there.
//
// Deleting the Known() check must fail this.
func TestAnInventedVerbIsRefused(t *testing.T) {
	for _, invented := range []string{
		"delete_everything", // plausible and absent
		"scroll_page",       // the gap a reader would most want to paper over
		"click",             // a real English verb that is not a semantic kind
		"INVOKE",            // the right verb, wrong case: snapping it is the plugin's job
		"",                  // nothing at all
	} {
		ask, _ := answering(resolver.Semantic{Kind: invented, Target: "Save"}, true)
		if _, ok := interpret.With(ask).Intent(context.Background(), "do the thing"); ok {
			t.Errorf("the verb %q was accepted. Only directorapi.SemanticVocabulary may "+
				"reach an Intent — a verb with no ladder behind it refuses at execution "+
				"time, which is one layer further from the sentence that caused it.",
				invented)
		}
	}
}

// A VERB THAT NEEDS SOMETHING TO ACT ON IS REFUSED WITHOUT IT.
//
// `NeedsTarget` is the Director's own division: "expand" means nothing until something is named,
// while "undo" means the thing the person is in. An interpretation returning `expand` with neither
// a target nor a pointing describes no request anybody made.
//
// Deleting the NeedsTarget check must fail this.
func TestAVerbThatNeedsATargetIsRefusedWithoutOne(t *testing.T) {
	needy := directorapi.SemanticExpand
	if !needy.NeedsTarget() {
		t.Fatalf("%s no longer needs a target; pick another verb for this test", needy)
	}
	ask, _ := answering(resolver.Semantic{Kind: string(needy)}, true)
	if _, ok := interpret.With(ask).Intent(context.Background(), "open it up"); ok {
		t.Error("expand was accepted with nothing to expand")
	}

	// AND A POINTING IS ENOUGH, because "expand this" is a real request and the resolver
	// answers it from what holds focus.
	ask, _ = answering(resolver.Semantic{Kind: string(needy), Deictic: true}, true)
	in, ok := interpret.With(ask).Intent(context.Background(), "open this up")
	if !ok {
		t.Fatal("expand was refused with a pointing, which is a request a person makes")
	}
	if len(in.Targets) != 1 || in.Targets[0].Kind != directorapi.ReferenceDeictic {
		t.Errorf("a pointing did not become a deictic reference: %+v", in.Targets)
	}

	// AND A VERB THAT NEEDS NOTHING IS FINE ALONE — "go back" addresses the view.
	free := directorapi.SemanticBack
	if free.NeedsTarget() {
		t.Fatalf("%s now needs a target; pick another verb", free)
	}
	ask, _ = answering(resolver.Semantic{Kind: string(free)}, true)
	if _, ok := interpret.With(ask).Intent(context.Background(), "take me back"); !ok {
		t.Error("back was refused with no target, and it addresses the whole view")
	}
}

// NOTHING INTERPRETED IT IS AN ANSWER, NOT A FAILURE.
//
// No plugin configured, a plugin that only picks routes, a plugin that declined, a timeout: every
// one of them lands here, and the caller's response is the same — keep the deterministic reading.
func TestADeclinedInterpretationIsReportedAsNoReading(t *testing.T) {
	ask, _ := answering(resolver.Semantic{}, false)
	if _, ok := interpret.With(ask).Intent(context.Background(), "whatever"); ok {
		t.Error("a declining plugin produced a reading")
	}
	// A NIL INTERPRETER IS THE DEFAULT AND MUST BE SAFE TO CALL. `interpret.New()` returns nil
	// when no plugin is configured, and the pipeline holds the result in a field.
	var none *interpret.Interpreter
	if _, ok := none.Intent(context.Background(), "whatever"); ok {
		t.Error("a nil interpreter produced a reading")
	}
	if interpret.With(nil) != nil {
		t.Error("With(nil) built an interpreter over nothing")
	}
}

// THE INTERPRETER IS NEVER TOLD WHAT IS ON THE SCREEN.
//
// # Why this is tested rather than asserted
//
// Because it is the claim that decides whether this boundary is acceptable at all. Marco's whole
// posture is that what is on somebody's screen does not leave the machine, and a model that was
// handed control labels in order to "help it choose" would cross that line while looking like an
// improvement.
//
// The Asker's SIGNATURE is the guarantee — input and verbs, and no parameter for anything else — so
// this test asserts the two things that are passed and that the verbs are the closed vocabulary
// rather than anything read off a desktop.
//
// Widening Asker must fail this.
func TestTheInterpreterIsNeverToldWhatIsOnTheScreen(t *testing.T) {
	var gotInput string
	var gotVerbs []string
	ask := func(_ context.Context, input string, verbs []string) (resolver.Semantic, bool) {
		gotInput, gotVerbs = input, verbs
		return resolver.Semantic{Kind: "refresh"}, true
	}
	if _, ok := interpret.With(ask).Intent(context.Background(), "reload the page"); !ok {
		t.Fatal("nothing was interpreted")
	}

	if gotInput != "reload the page" {
		t.Errorf("the plugin was sent %q", gotInput)
	}
	// THE VERBS ARE THE VOCABULARY, exactly, and nothing else is sent at all.
	if len(gotVerbs) != len(directorapi.SemanticVocabulary) {
		t.Fatalf("the plugin was sent %d verbs against a vocabulary of %d",
			len(gotVerbs), len(directorapi.SemanticVocabulary))
	}
	for i, k := range directorapi.SemanticVocabulary {
		if gotVerbs[i] != string(k) {
			t.Errorf("verb %d is %q, want %q", i, gotVerbs[i], k)
		}
	}
}

// THE VOCABULARY IS READ FROM THE ONE LIST, so a verb with no ladder cannot be offered.
//
// `directorapi.SemanticVocabulary` is what `director actions` prints and what the completeness
// tests check against. An interpreter that carried its own list could offer a word the Director
// cannot perform, and the person would be told Marco understood them.
func TestTheOfferedVerbsAreTheOnesMarcoCanPerform(t *testing.T) {
	var offered []string
	ask := func(_ context.Context, _ string, verbs []string) (resolver.Semantic, bool) {
		offered = verbs
		return resolver.Semantic{}, false
	}
	_, _ = interpret.With(ask).Intent(context.Background(), "anything")
	for _, v := range offered {
		if !directorapi.SemanticActionKind(v).Known() {
			t.Errorf("the interpreter offered %q, which is not in the vocabulary", v)
		}
	}
	if len(offered) == 0 {
		t.Fatal("no verbs were offered, so this proves nothing")
	}
}

// AND A CANCELLED REQUEST REACHES THE PLUGIN'S CONTEXT.
//
// Interpretation is the one part of parsing that can block, which is why it is the one part that
// carries a context. A parse that could not be cancelled would hold a Director command open with
// nothing to show for it, and `stop` would report success while a subprocess waited on a network.
func TestTheCallersContextReachesTheInterpreter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var saw error
	ask := func(c context.Context, _ string, _ []string) (resolver.Semantic, bool) {
		saw = c.Err()
		return resolver.Semantic{}, false
	}
	_, _ = interpret.With(ask).Intent(ctx, "anything")
	if saw == nil {
		t.Error("the plugin was handed a context that had not been cancelled; a stop " +
			"cannot reach an interpretation that does not carry the caller's context")
	}
}

// ── what this package can reach ───────────────────────────────────────────────

// THE INTERPRETER CANNOT ACT.
//
// A source-level walk of what this package imports. The claim is not that it does not drive the
// desktop — it is that nothing it can reach knows how, which is the same structural argument
// `internal/director/observe`'s boundary test makes.
//
// Kept as an import check rather than a dependency walk because the package is three imports deep
// and the list is the whole of the surface: a model-backed reading gets the vocabulary, the plugin
// client and the one Intent constructor, and that is all.
func TestTheInterpreterCannotAct(t *testing.T) {
	source := read(t, "interpret.go")
	for _, forbidden := range []string{
		"platform/", // every OS surface
		"oshost",    // input
		"runtime",   // the Marco engine
		"rehearse",  // the one walker
		"theater",   // the actor layer
		"winctx",    // windows and the foreground
		"execute",   // the pipeline, which would be a cycle anyway
		"os/exec",   // a subprocess started from here rather than at the boundary
	} {
		if strings.Contains(importBlock(source), forbidden) {
			t.Errorf("interpret imports something matching %q. Nothing reachable from an "+
				"interpretation may be able to affect the machine: a model proposes "+
				"meaning, and every effect is decided and performed somewhere else.",
				forbidden)
		}
	}
}
