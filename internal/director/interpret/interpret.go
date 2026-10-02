// Package interpret turns a sentence the deterministic parser could not read into one of the
// Director's existing typed operations — and into nothing else.
//
// # What this is for
//
// The Director can already do a great deal: thirty-three semantic verbs, each lowered against a
// control's real capabilities by a ladder at execution time, each verified by that control's own
// state, all of it reaching the desktop only as legal Marco. What it could not do was understand a
// request phrased in any way other than the few the offline parser knows. So "click Save" worked
// and "hit the save button would you" did not, and the gap between those two sentences was the
// whole product.
//
// # Where it sits, and why that is the only place it could sit
//
//	natural language
//	    │
//	    ├── intent.Parser.Parse          deterministic, offline, authoritative where it answers
//	    └── THIS, only where that said IntentUnknown
//	    ▼
//	directorapi.Intent                   a typed operation, nothing more
//	    ▼
//	target.Resolver                      grounded against a world Marco observed ITSELF
//	    ▼
//	plan → policy → confirmation → legal Marco → execute → verify
//
// Everything below the second line is untouched. This package produces an Intent and has no
// access to anything that could act: its imports are the vocabulary, the plugin client and the one
// Intent constructor, and a boundary test walks the tree to keep it that way.
//
// # What it structurally cannot do
//
// Not rules — shapes. There is no field, no parameter and no import by which any of these could
// happen:
//
//	CALL THE DESKTOP          nothing here reaches a host, an executor or an input path.
//	GRANT AUTHORITY           nothing here mints, reads or carries a grant.
//	INVENT A RESOLVED TARGET  a target leaves here as the user's PHRASE and an ElementQuery on
//	                          it. There is no element id, no window, no coordinate and no
//	                          durable subject in anything this package can produce.
//	SEE THE SCREEN            the plugin is sent the user's words and a list of verbs. Not a
//	                          label, not a title, not a world, not a memory.
//	BYPASS THE PARSER         it is consulted only after the deterministic reading returned
//	                          IntentUnknown, and a deterministic reading always wins.
//	INVENT A VERB             the answer is checked against directorapi.SemanticVocabulary and
//	                          dropped whole if it is not in it.
//	DECLARE SUCCESS           it produces an intention. Whether anything happened is decided
//	                          three layers down, by a fresh look.
//
// # And what it is not
//
// Not an agent and not a loop. One sentence in, one typed operation out, no tools, no memory, no
// second call. A model that could ask for another turn would be a second decider in a system whose
// whole shape is that there is one.
package interpret

import (
	"context"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/uiact"
	"github.com/chaynes-simpleclouds/marco/internal/resolver"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ModelConfidence is what an interpreted reading claims.
//
// Below [intent.DeterministicConfidence], because it is a weaker kind of evidence about what
// somebody meant: the offline parser recognised a form, and this recognised an intention. Nothing
// in the pipeline gates on it — it exists so a trace can say which reading produced a step.
const ModelConfidence = 0.6

// Asker is the question this package puts, and the only thing it needs from the outside world.
//
// An interface of one function, so the plugin boundary is substitutable in a test without a
// subprocess, a model, a network or an API key — and so that this package's own tests exercise
// the VALIDATION, which is the part that matters, rather than exercising `exec`.
//
// It is handed the user's words and the verb vocabulary. There is no parameter for anything else,
// which is how "the interpreter never sees the screen" is a property of the type rather than a
// claim in a comment.
type Asker func(ctx context.Context, input string, verbs []string) (resolver.Semantic, bool)

// Interpreter turns a sentence into a typed operation by asking something that can read English.
type Interpreter struct {
	ask Asker
}

// New returns an Interpreter over the configured resolver plugin.
//
// Nil when no plugin is configured, so the caller wires nothing rather than wiring something that
// always declines — a field that is set and never answers is indistinguishable from a broken one,
// and the Director should be able to say "no model is configured" rather than "it did not
// understand".
func New() *Interpreter {
	if !resolver.Configured() {
		return nil
	}
	return &Interpreter{ask: resolver.Interpret}
}

// With returns an Interpreter over a supplied asker, for tests and for an alternative boundary.
func With(ask Asker) *Interpreter {
	if ask == nil {
		return nil
	}
	return &Interpreter{ask: ask}
}

// Intent interprets one request, reporting false when nothing could.
//
// # The signature is the contract
//
// It takes a context, because a model call is the one part of parsing that can block and `stop`
// has to reach it. It returns `(Intent, bool)` rather than an Intent with an error, because
// "nothing interpreted this" is not a failure — it is the ordinary answer when no plugin is
// configured, and the caller's response is the same either way: keep the deterministic reading and
// the sentence it refuses with.
//
// # Why the vocabulary is read here rather than passed in
//
// Because `directorapi.SemanticVocabulary` is the one list of what the Director can actually
// perform, and a caller that supplied its own could offer a verb no ladder implements. The same
// argument `intent.parseSemantic` makes for delegating to `uiact`.
func (i *Interpreter) Intent(ctx context.Context, request string) (directorapi.Intent, bool) {
	if i == nil || i.ask == nil {
		return directorapi.Intent{}, false
	}
	verbs := make([]string, 0, len(directorapi.SemanticVocabulary))
	for _, k := range directorapi.SemanticVocabulary {
		verbs = append(verbs, string(k))
	}

	answer, ok := i.ask(ctx, request, verbs)
	if !ok {
		return directorapi.Intent{}, false
	}

	// THE VERB IS CHECKED AGAINST THE VOCABULARY, and a verb outside it is refused rather
	// than approximated.
	//
	// This is the same rule the deterministic path relies on — `SemanticKindOf` refuses a kind
	// no ladder implements — asked here so an unperformable verb never becomes an Intent at
	// all. A kind that got through would refuse at execution time, after the person had been
	// told Marco understood them.
	//
	// It also closes the one thing a model answer could otherwise do that nothing else in this
	// system can: name an operation that does not exist. There are thirty-three words it may
	// say and it may not say anything else.
	//
	// Deleting this check must fail TestAnInventedVerbIsRefused.
	kind := directorapi.SemanticActionKind(answer.Kind)
	if !kind.Known() {
		return directorapi.Intent{}, false
	}

	// AND A VERB THAT NEEDS SOMETHING TO ACT ON MUST HAVE BEEN GIVEN IT.
	//
	// `NeedsTarget` is the Director's own division: "expand" means nothing until something is
	// named, while "undo" means the thing the person is in. An interpretation that returned
	// `expand` with no target and no pointing describes no request, and passing it on would
	// reach the resolver with nothing to resolve and refuse there instead — one layer further
	// from the sentence that caused it.
	//
	// Deleting this check must fail TestAVerbThatNeedsATargetIsRefusedWithoutOne.
	if kind.NeedsTarget() && answer.Target == "" && !answer.Deictic {
		return directorapi.Intent{}, false
	}

	// THE ONE CONSTRUCTION SITE. Everything above produced a `uiact.Request`, which is the
	// same value `uiact.Parse` produces for a phrase it recognises — so what is built here is
	// what the deterministic parser would have built had it understood the sentence.
	//
	// There is deliberately no second path: an Intent assembled field-by-field in this package
	// could carry a shape the parser cannot, and the shapes the parser cannot carry are the
	// ones nothing downstream has been tested against.
	req := uiact.Request{
		Kind: kind, Target: answer.Target, Ordinal: answer.Ordinal, Deictic: answer.Deictic,
	}
	in := directorapi.Intent{Raw: request}
	return intent.SemanticIntent(in, req, ModelConfidence), true
}
