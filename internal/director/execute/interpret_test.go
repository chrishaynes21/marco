package execute

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/internal/director/uiact"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Where a model's reading is allowed to enter the pipeline, and where it is not.
//
// The integration is one optional field consulted in one condition. These hold that condition, and
// the claim that everything after it is the path it always was.

// interprets is an Interpret function that answers one fixed semantic reading.
func interprets(kind directorapi.SemanticActionKind, label string) func(context.Context,
	string) (directorapi.Intent, bool) {

	return func(_ context.Context, request string) (directorapi.Intent, bool) {
		req := uiact.Request{Kind: kind, Target: label}
		return intent.SemanticIntent(directorapi.Intent{Raw: request}, req, 0.6), true
	}
}

// ── the condition ─────────────────────────────────────────────────────────────

// A DETERMINISTIC READING IS NEVER REINTERPRETED.
//
// The offline parser is authoritative where it answers: deterministic, offline, instant, and right
// about "click File". A model consulted first — or consulted as well — would make a settled request
// depend on a subprocess and a network, and would occasionally read it differently.
//
// So `IntentUnknown` is the only state in which anything else is asked. Every other kind is a
// reading the parser is entitled to make.
//
// Deleting the IntentUnknown condition must fail this.
func TestADeterministicReadingIsNeverReinterpreted(t *testing.T) {
	h := newHarness(
		scene(t0, nil, menuBar()...),
		scene(t0.Add(time.Second), nil, menuOpen()...),
	)
	asked := 0
	// An interpreter that would answer DIFFERENTLY if it were consulted, so a reading that
	// reached it is visible in the outcome rather than merely counted.
	h.pipeline.Interpret = func(_ context.Context, request string) (directorapi.Intent, bool) {
		asked++
		req := uiact.Request{Kind: directorapi.SemanticRefresh}
		return intent.SemanticIntent(directorapi.Intent{Raw: request}, req, 0.6), true
	}

	out := h.pipeline.Handle(context.Background(), "click File")

	if asked != 0 {
		t.Errorf("the interpreter was consulted %d time(s) about a request the parser read "+
			"perfectly well. A settled phrasing must not become dependent on a model.",
			asked)
	}
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	if len(h.actuator.clicks) != 1 {
		t.Fatalf("want one click, got %d", len(h.actuator.clicks))
	}
}

// AND A PHRASE THE PARSER CANNOT READ GOES ALL THE WAY THROUGH.
//
// The headline, end to end over the ordinary harness: a sentence with no deterministic reading
// becomes a typed operation, resolves against the observed world, is planned, actuated and
// VERIFIED — and lands on the control the user described rather than on a coordinate.
//
// Nothing in the pipeline below the parse was changed for this, which is the whole point of
// integrating at `Pipeline.Intent`.
func TestAnInterpretedPhraseIsPlannedAndVerifiedLikeAnyOther(t *testing.T) {
	// The sentence has to be one the deterministic parser genuinely declines, or this test
	// would pass without the interpreter being involved at all.
	if parsed := intent.New().Parse(unparseable); parsed.Kind != directorapi.IntentUnknown {
		t.Fatalf("%q is now read deterministically as %+v; this test needs a phrase the "+
			"offline parser declines", unparseable, parsed)
	}

	h := newHarness(
		scene(t0, nil, menuBar()...),
		scene(t0.Add(time.Second), nil, menuOpen()...),
	)
	h.pipeline.Interpret = interprets(directorapi.SemanticInvoke, "File")

	out := h.pipeline.Handle(context.Background(), unparseable)

	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	// ONE EFFECT, THROUGH THE LADDER. An interpreted `invoke` is not a click: the ladder picks
	// the implementation from the control's real capabilities at execution time, so the effect
	// arrives as a Marco operation rather than as a pointer going somewhere. That it does is
	// the point — the model named an intention and the mechanism was chosen without it.
	if got := h.operations.kinds(); len(got) != 1 {
		t.Fatalf("want exactly one lowered operation, got %v (and %d clicks)",
			got, len(h.actuator.clicks))
	}
	if !out.Record.Verification.Success {
		t.Error("an interpreted request must be verified like any other; it was not")
	}
	// AND IT IS IN THE GRAPH AS AN ORDINARY NODE, carrying the user's own sentence.
	n, err := h.graph.Last()
	if err != nil {
		t.Fatalf("the action was not recorded: %v", err)
	}
	if n.Intent.Raw != unparseable {
		t.Errorf("the node records %q rather than what the person typed", n.Intent.Raw)
	}
	// THE TARGET WAS RESOLVED BY MARCO, not supplied by the interpretation. The model said
	// the word "File"; which element that is, in which window, at which position, all came
	// from the world this pipeline observed for itself.
	if n.ResolvedTarget.Label != "File" {
		t.Errorf("the resolved target is %q, want the control the description named",
			n.ResolvedTarget.Label)
	}
	if spec, ok := n.Action(); !ok || spec.Query == nil {
		t.Error("the node carries no query, so nothing re-resolves on a replay")
	}
}

// AND WHEN NOTHING INTERPRETS IT, THE PARSER'S OWN REFUSAL STANDS.
//
// A declining interpreter must leave the deterministic Intent exactly as it was — including its
// `Ambiguity` sentence, which is what the person is shown. A message invented at this boundary
// would claim a model had an opinion about the request when none did.
func TestADeclinedInterpretationLeavesTheParsersRefusal(t *testing.T) {
	h := newHarness(scene(t0, nil, menuBar()...))
	h.pipeline.Interpret = func(context.Context, string) (directorapi.Intent, bool) {
		return directorapi.Intent{}, false
	}

	out := h.pipeline.Handle(context.Background(), unparseable)

	if out.Status != directorapi.ResultNeedsClarification {
		t.Fatalf("status = %s (%s), want a clarification", out.Status, out.Message)
	}
	want := intent.New().Parse(unparseable).Ambiguity
	if want == "" {
		t.Fatal("the deterministic parser offers no sentence, so there is nothing to keep")
	}
	if out.Message != want {
		t.Errorf("the refusal reads %q, want the parser's own %q", out.Message, want)
	}
	// AND NOTHING WAS DONE.
	if len(h.actuator.clicks) != 0 {
		t.Errorf("%d effect(s) reached the desktop for a request nothing understood",
			len(h.actuator.clicks))
	}
}

// AN ABSENT INTERPRETER CHANGES NOTHING.
//
// The default: no plugin configured means a nil field. A Director without a model must behave
// exactly as it did before this existed, including in its refusal.
func TestWithNoInterpreterTheParserIsTheWholeReading(t *testing.T) {
	h := newHarness(scene(t0, nil, menuBar()...))
	if h.pipeline.Interpret != nil {
		t.Fatal("the harness wired an interpreter; this test is about not having one")
	}
	out := h.pipeline.Handle(context.Background(), unparseable)
	if out.Status != directorapi.ResultNeedsClarification {
		t.Errorf("status = %s (%s)", out.Status, out.Message)
	}
	if len(h.actuator.clicks) != 0 {
		t.Errorf("%d effect(s) with no reading at all", len(h.actuator.clicks))
	}
}

// ── composition ───────────────────────────────────────────────────────────────

// EVERY CLAUSE OF A MULTI-STEP REQUEST GETS THE SAME READING.
//
// `program.Decompose` parses one clause at a time, and it is handed the SAME composed reader the
// single-step path uses — so a request whose second clause needs interpreting gets it, and a
// request whose first clause is deterministic does not pay for a model call.
//
// This is why the integration is a closure rather than a change to Decompose's signature: the
// context is in scope at both call sites, and nothing else had to move.
func TestEveryClauseOfAProgramGetsTheSameReading(t *testing.T) {
	asked := []string{}
	parse := func(request string) directorapi.Intent {
		return (&Pipeline{
			Intent: intent.New().Parse,
			Interpret: func(_ context.Context, r string) (directorapi.Intent, bool) {
				asked = append(asked, r)
				req := uiact.Request{Kind: directorapi.SemanticInvoke, Target: "Save"}
				return intent.SemanticIntent(directorapi.Intent{Raw: r}, req, 0.6), true
			},
		}).parseWith(context.Background())(request)
	}

	// `program.Split` decides where the clauses are, from a fixed list of instruction-starting
	// words, and the interpreter does not change that. "hit" is on the list and is NOT a verb
	// the parser reads — which is exactly the shape this needs: two clauses, the first
	// deterministic and the second not.
	//
	// It is also the honest limit of this integration: a second clause beginning with a word
	// Split does not recognise ("…and would you hit Save") is rejoined into the first, and no
	// amount of interpretation per clause recovers a clause boundary that was never found.
	prog, err := program.Decompose("click File and hit the save button for me", parse)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	if len(prog.Steps) != 2 {
		t.Fatalf("%d step(s), want 2: %+v", len(prog.Steps), prog.Steps)
	}
	// THE FIRST CLAUSE COST NOTHING. Only the clause the parser declined was asked about.
	if len(asked) != 1 {
		t.Fatalf("the interpreter was asked about %d clause(s): %q. A deterministic clause "+
			"must not reach it.", len(asked), asked)
	}
	for i, step := range prog.Steps {
		if step.Operation.Kind != directorapi.IntentAct {
			t.Errorf("step %d is %s, want an action: %+v", i+1,
				step.Operation.Kind, step.Operation)
		}
	}
}

// unparseable is a sentence the deterministic parser declines, and a person would plainly mean.
//
// Asserted rather than assumed in the tests that use it: if the offline vocabulary ever grows to
// cover this phrasing, the tests that rely on it being declined must fail loudly rather than
// quietly stop testing anything.
const unparseable = "would you hit the save button for me"

// A STOPPED REQUEST IS CANCELLED, NOT MISUNDERSTOOD.
//
// # The defect, measured end to end
//
// `marco stop` while a model was thinking produced:
//
//	FAILED: I only understand click, focus, move, repeat and text editing so far
//	[result] failed
//
// The interpretation was genuinely cancelled — the plugin's subprocess was killed and the request
// returned early — so the mechanism worked. What was wrong was the SENTENCE. A person who pressed
// stop was told to say it differently, about a phrase that was never the problem.
//
// The cause is structural rather than careless: when the interpreter declines, `parseWith` keeps
// the deterministic reading and its refusal, which is right for every other way an interpretation
// can decline — no plugin, a malformed answer, an unknown verb — and wrong for exactly one.
// Cancellation is not a property of the request.
//
// The product already has the word. `handleParsed` turns `IntentStop` into `ResultCancelled`, so
// this adds no vocabulary and no concept.
//
// Deleting the ctx.Err() check must fail this.
func TestAStoppedRequestIsCancelledAndNotMisunderstood(t *testing.T) {
	h := newHarness(scene(t0, nil, menuBar()...))
	ctx, cancel := context.WithCancel(context.Background())

	// An interpreter that declines BECAUSE it was cancelled, which is what a killed plugin
	// subprocess produces.
	h.pipeline.Interpret = func(c context.Context, _ string) (directorapi.Intent, bool) {
		<-c.Done()
		return directorapi.Intent{}, false
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	out := h.pipeline.Handle(ctx, unparseable)

	if out.Status != directorapi.ResultCancelled {
		t.Errorf("a stopped request came back as %s (%q), want cancelled.\n\n"+
			"The deterministic parser's refusal is a true sentence about a phrase nobody "+
			"read, and the wrong one for somebody who pressed stop — it sends them to "+
			"rephrase a request that was never the problem.", out.Status, out.Message)
	}
	if strings.Contains(out.Message, "I only understand") {
		t.Errorf("the message is the parser's vocabulary list: %q", out.Message)
	}
	if len(h.actuator.clicks) != 0 || len(h.operations.ops) != 0 {
		t.Errorf("a stopped request still acted: %d clicks, %d operations",
			len(h.actuator.clicks), len(h.operations.ops))
	}
}

// AND AN ORDINARY DECLINE IS STILL A FAILURE TO UNDERSTAND.
//
// The other side, and the reason the check is on the context rather than on the decline: a plugin
// that simply had no reading, or none configured at all, leaves the parser's own refusal exactly
// where it was. Only a cancelled request is reported as cancelled.
func TestAnUncancelledDeclineKeepsTheParsersRefusal(t *testing.T) {
	h := newHarness(scene(t0, nil, menuBar()...))
	h.pipeline.Interpret = func(context.Context, string) (directorapi.Intent, bool) {
		return directorapi.Intent{}, false
	}
	out := h.pipeline.Handle(context.Background(), unparseable)
	if out.Status == directorapi.ResultCancelled {
		t.Error("a decline with a live context was reported as a cancellation")
	}
	if out.Status != directorapi.ResultNeedsClarification {
		t.Errorf("status = %s (%q)", out.Status, out.Message)
	}
}
