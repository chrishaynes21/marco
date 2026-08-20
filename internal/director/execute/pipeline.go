package execute

import (
	"context"
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/internal/director/memory"
	"github.com/chaynes-simpleclouds/marco/internal/director/plan"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/internal/director/verify"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/engine"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Observer produces a fresh world snapshot. It is a function rather than an
// interface because the pipeline needs exactly one thing from perception, and a
// one-method interface here would only obscure that.
type Observer func(ctx context.Context) (directorapi.WorldState, error)

// Pipeline runs the closed loop:
//
//	observe → build world → parse intent → resolve target → plan → policy →
//	execute one step → observe again → verify → record
//
// Every stage is an injected collaborator so each can be tested alone and the whole
// can be tested with fakes — no live desktop is needed to exercise any of it.
//
// The loop is CLOSED on purpose. Executing without re-observing would leave the
// Director believing its own intentions, which is precisely the failure mode this
// milestone exists to rule out.
type Pipeline struct {
	Observe  Observer
	Intent   func(string) directorapi.Intent
	Resolver *target.Resolver
	Planner  *plan.Planner
	// Goals expands a high-level goal into a program. Optional: without it the
	// pipeline behaves exactly as it did before, which is how a caller that has not
	// opted in stays unaffected.
	Goals *Goals
	// Confirmer puts a goal-level confirmation to the user before its first step
	// runs. Optional, and its absence REFUSES a goal that needs one rather than
	// defaulting to yes — see ConfirmationUnavailable.
	Confirmer Confirmer
	Policy    directorapi.PolicyEngine
	Executor  directorapi.DirectorExecutor
	Verifier  *verify.Verifier
	// Resources reads the backing identity of a bound object — whether a file is
	// there, and what is inside it. Optional, and its absence is reported honestly
	// rather than papered over: a rename whose result cannot be read back is
	// INCONCLUSIVE, which is a different claim from "it worked".
	//
	// Injected rather than implemented here so verification can be tested against
	// controlled fixtures, and so the Director can be looking at a remote or virtual
	// location where a stat would be both slow and occasionally wrong.
	Resources verify.Inspector
	// Graph is the durable, append-only record of what the Director has done. The
	// pipeline appends to it; nothing ever edits what is already there.
	Graph actiongraph.Graph

	// Monitors supplies the display layout for window placement. Optional; without
	// it, window moves that need geometry fail explicitly.
	Monitors func(ctx context.Context) ([]directorapi.Monitor, error)

	// SettleDelay is how long to wait after acting before observing again.
	//
	// Not padding: user interfaces animate. A menu takes a moment to render, a
	// window manager takes a moment to finish a move, and observing into the middle
	// of that reports a half-finished screen. Too long and the pipeline is sluggish;
	// too short and it verifies against a transition.
	SettleDelay time.Duration

	// OnProgram reports a step boundary during a program, so a client can draw
	// "[2/4] verified" as it happens rather than only at the end.
	OnProgram func(ProgramProgress)

	// OnProgress, when set, receives an event at each meaningful step of a long
	// command. Optional: without it the pipeline behaves exactly as before.
	OnProgress func(Progress)

	// Watcher, when set, observes whether a bounded region of the SCREEN changed —
	// evidence the accessibility tree cannot give. Optional: without it every path
	// below falls back to the structural guard exactly as before, which is what keeps
	// this an addition rather than a change.
	Watcher RegionWatcher

	// Waiter waits for SEMANTIC conditions instead of durations. Optional: without it
	// the pipeline keeps its fixed settle delay, which is the only remaining sleep and
	// exists solely for the case where no condition can be evaluated.
	Waiter SettleWaiter

	// StopCheck reports whether the user has asked to stop. Polled between replay
	// iterations, alongside context cancellation.
	StopCheck func() bool
	// MaxReplayIterations is the hard ceiling on any one replay request.
	MaxReplayIterations int

	// stepIndex, stepCount and stepID are the current program step, zero outside a
	// program. Held on the pipeline rather than threaded through every signature
	// because every phase helper needs them and nothing else does.
	stepIndex int
	stepCount int
	stepID    string
	// goalProvenance is which goal and procedure produced the running program, zero
	// outside one. Copied onto every node the program produces, as DIAGNOSTIC metadata
	// — replay never reads it.
	goalProvenance *actiongraph.GoalProvenance

	// iteration is the collection lineage of the member currently executing, nil
	// outside an iteration. On the pipeline for the same reason the step position is:
	// the node-building path needs it and threading it through every signature between
	// here and there would widen a dozen of them for one field.
	iteration *iterationProvenance

	// Variables is the Director.s semantic memory. Optional: without it a "$name"
	// reference fails honestly rather than being reinterpreted as text.
	Variables VariableStore

	// ControlValues reads a control's own value, for capturing one. Optional: without
	// it a control-value capture falls back to what perception observed, and then to an
	// honest Unknown. READ-ONLY by type — see ControlReader.
	ControlValues ControlReader
	// OnEnvironment reports the running program's value environment, and nil when it
	// ends. Optional: without it, values are still captured and consumed exactly as
	// before — only the diagnostics that inspect a LIVE program go quiet.
	//
	// A callback rather than a field the service reads, because the environment belongs
	// to the program: the service is told about it for as long as the program lives and
	// told to forget it the moment it does not. A field would outlive its owner.
	OnEnvironment func(*values.Environment)
	// OnCollections reports the running program.s collection environment, and nil when
	// it ends. Same contract and same reason as OnEnvironment: the service is told
	// about it for as long as the program lives and told to forget it the moment it
	// does not.
	OnCollections func(*collections.Environment)

	// Clipboard reads and writes the system clipboard, for capturing it and for
	// reading a selection through it. Optional: without it both fail explicitly rather
	// than degrading into something less honest.
	//
	// Writing is needed only to BORROW: a selection is read by leaving a probe on the
	// clipboard and taking it back afterwards. See captureSelectedText.
	Clipboard directorapi.Clipboard

	// Trace records what each phase of a command did and how long it took. Optional:
	// without one the pipeline behaves exactly as before, which is what keeps this an
	// addition rather than a change.
	Trace *trace.Trace
	// Deadlines bound the phases that can block on something outside the Director.
	Deadlines trace.Deadlines

	// Now is injectable so records are deterministic under test.
	Now func() time.Time
}

// DefaultSettleDelay is long enough for a menu to render and a window manager to
// finish a move, and short enough not to be felt.
const DefaultSettleDelay = 350 * time.Millisecond

// Outcome is the full result of one request, including everything needed to explain
// it. It is returned even when the request failed — a partial, inspectable failure
// is worth far more than an error string.
type Outcome struct {
	Request string                  `json:"request"`
	Intent  directorapi.Intent      `json:"intent"`
	World   *directorapi.WorldState `json:"-"`
	// Resolution is the target search, including rejected candidates.
	Resolution *directorapi.Resolution     `json:"resolution,omitempty"`
	Plan       *directorapi.Plan           `json:"plan,omitempty"`
	Policy     *directorapi.PolicyDecision `json:"policy,omitempty"`
	Record     *directorapi.ActionRecord   `json:"record,omitempty"`
	// Node is the durable semantic node this action became.
	Node *actiongraph.ActionNode `json:"node,omitempty"`
	// Replay is set when the request was a repeat.
	Replay *ReplayOutcome `json:"replay,omitempty"`
	// Collection is set when the request iterated a bounded set. Its Iterations hold
	// each member.s verified turn.
	Collection *collections.Result `json:"collection,omitempty"`
	// Program is set when the request decomposed into a sequence. Its Steps hold the
	// per-step outcomes; this Outcome is the whole thing.
	Program *ProgramOutcome `json:"program,omitempty"`
	// Binding is the account of what the binding and confirmation layers did: what was
	// bound, what the re-check concluded, what was asked and what was answered. Absent
	// for a request that pointed at nothing and needed no confirmation.
	Binding *BindingDiagnostics `json:"binding,omitempty"`

	// Status is the headline result.
	Status directorapi.ResultStatus `json:"status"`
	// Message is one sentence for a person.
	Message string `json:"message"`
	// Retried reports whether the single permitted retry was used.
	Retried bool `json:"retried"`
	// Stages records what happened at each step, in order, for the trace.
	Stages []Stage `json:"stages"`
	// Error is set when the pipeline could not complete.
	Error string `json:"error,omitempty"`
}

// Stage is one step of the pipeline, for the explanation trace.
type Stage struct {
	Name   string `json:"name"`
	Detail string `json:"detail"`
	OK     bool   `json:"ok"`
}

// Handle runs one request end to end.
//
// A single request gets a one-step value environment, created here and discarded when
// it returns. That is not a convenience: `remember the clipboard as clip` on its own is
// a legal request, and it must be able to bind somewhere — but nothing it binds may
// survive the request, or a value captured in one command would silently be readable by
// the next.
func (p *Pipeline) Handle(ctx context.Context, request string) Outcome {
	// And its own binding store, for the same reason and with the same lifetime: a
	// binding proves what "this" meant during ONE request, and one that outlived the
	// request would let the next command act on an object nobody pointed at.
	ctx = ensureBindings(ctx)
	var pctx program.Context
	env := pctx.EnsureValues()
	defer env.Clear()
	colls := pctx.EnsureCollections()
	defer colls.Clear()
	return p.handleParsed(ctx, request, p.Intent(request), pctx)
}

// handleParsed is the single-step pipeline, entered with an already-parsed intent.
//
// Split out of Handle so a program step can reuse it VERBATIM. A program step gets
// exactly the verification, the retry guard, the settle-by-condition and the Marco
// lowering that a single request gets, because it is the same code — a separate
// "sequence executor" with its own simplified copy is how two paths drift until one
// of them is quietly less safe.
//
// pctx is what the program running this step has established: the previous step's
// target, for a back-reference; the previous step's node, so a program reads as a chain
// in the action graph rather than as unrelated events; and the value environment, which
// is the ONE thing here that a step may both read from and write to.
//
// It is passed rather than held on the pipeline because it belongs to the PROGRAM. A
// pipeline field would belong to the Director, and would still be there for whatever
// ran next.
func (p *Pipeline) handleParsed(ctx context.Context, request string, parsed directorapi.Intent,
	pctx program.Context) Outcome {

	parent := parentForStep(pctx)
	out := Outcome{Request: request, Status: directorapi.ResultFailed}
	add := func(name, detail string, ok bool) {
		out.Stages = append(out.Stages, Stage{Name: name, Detail: detail, OK: ok})
	}

	// ── Intent ────────────────────────────────────────────────────────────────
	out.Intent = parsed
	if out.Intent.Kind == directorapi.IntentStop {
		out.Status = directorapi.ResultCancelled
		out.Message = "stopped"
		add("intent", "stop", true)
		return out
	}
	// A repeat refers to something already in the graph rather than to something on
	// screen, so it takes a different route: find the action, then re-run it
	// semantically. Everything downstream of that is the ordinary pipeline.
	if out.Intent.Kind == directorapi.IntentRepeat {
		return p.handleRepeat(ctx, out, add)
	}
	if out.Intent.Kind != directorapi.IntentAct {
		out.Status = directorapi.ResultNeedsClarification
		out.Message = firstNonEmpty(out.Intent.Ambiguity, "I did not understand that")
		add("intent", out.Message, false)
		return out
	}
	add("intent", fmt.Sprintf("%s %s", out.Intent.Verb, targetPhrase(out.Intent)), true)

	// The per-step binding guard. Program validation already refused an unbound deictic
	// program before step 1; this catches everything validation cannot see — an intent
	// assembled outside goal expansion, a step rebuilt from an old graph, a replay whose
	// stored binding could not be restored. There is no path from a reference that says
	// it needs a binding to an executor that does not have one.
	if err := requireBinding(out.Intent); err != nil {
		out.Status = directorapi.ResultFailed
		out.Message = err.Error()
		add("binding", err.Error(), false)
		return out
	}

	// Variable MANAGEMENT is Director knowledge, not a desktop effect. Answered here,
	// before observation, because nothing but `remember` needs a world at all.
	if vout, handled := p.handleVariableCommand(ctx, request, out.Intent, add); handled {
		vout.Stages = append(out.Stages, vout.Stages...)
		return vout
	}

	// Capturing a VALUE is also not a desktop effect. Answered here for a second
	// reason: there is nothing to plan. No action will be executed, so there is no plan
	// for policy to evaluate and no Marco to lower — a capture that fell through to the
	// ordinary path would have to invent an action to carry it.
	if cout, handled := p.handleCapture(ctx, request, out.Intent, pctx, add); handled {
		cout.Stages = append(out.Stages, cout.Stages...)
		return cout
	}

	// Capturing a COLLECTION binds a query and touches nothing, so it is answered here
	// for the same reason a value capture is: there is no action to plan.
	if cout, handled := p.handleCaptureCollection(ctx, request, out.Intent, pctx, add); handled {
		cout.Stages = append(out.Stages, cout.Stages...)
		return cout
	}

	// An ITERATION is answered here too, and for a different reason: it is not one
	// operation but many, each of which re-enters this same function. Letting it fall
	// through to the singular path would try to resolve "every selected item" as one
	// control.
	if fout, handled := p.handleForEach(ctx, request, out.Intent, pctx, add); handled {
		fout.Stages = append(out.Stages, fout.Stages...)
		return fout
	}

	// A ${name} reference becomes the text the program captured, read NOW rather than
	// when the request was parsed. Everything after this line receives concrete text and
	// does not know a value was involved — which is what keeps the planner, the policy
	// engine, the lowering and the verifier from each needing their own answer to "what
	// if the value is missing?".
	var used *valueUse
	if v, ok, err := p.applyValue(&out.Intent, pctx.Values); err != nil {
		// A failed lookup is a real event and worth seeing, but it is NOT a
		// consumption: nothing received the value, and recording it as one would make
		// "which steps used this?" answer with a step that never did.
		if ref, wanted := program.Reads(out.Intent); wanted {
			p.emitValue(trace.ValueEvent{
				Kind: trace.EventValueRead, Name: ref.Name,
				Outcome: "failed", Reason: err.Error(),
			})
		}
		out.Status = directorapi.ResultFailed
		out.Message = err.Error()
		add("value", err.Error(), false)
		return out
	} else if ok {
		ref, _ := program.Reads(out.Intent)
		used = &valueUse{Ref: ref, Val: v}
		p.emitValue(trace.ValueEvent{
			Kind: trace.EventValueRead, Name: ref.Name,
			ValueKind: string(v.Kind()), Visibility: string(v.Visibility()),
			ByteLength: v.Len(), ExpectedInputType: "text",
			Compatibility: "compatible", Outcome: "ok",
		})
		add("value", describeValueUse(ref, v), true)

		// Tell the executor not to keep this text in its editing diagnostics. Those are
		// the most detailed record in the system — value before, value after, which rung
		// of the ladder proved what — and are written by the editor itself, out of reach
		// of the redaction that runs later over the plan and the record.
		if r, canRedact := p.Executor.(interface{ RedactNext(string) }); canRedact &&
			v.Visibility() != values.VisibilityNormal {
			r.RedactNext(v.Plaintext())
			defer r.RedactNext("")
		}
	}

	// A "$name" reference becomes the query the variable stored. Everything after this
	// line is the ordinary path — the variable buys no shortcuts.
	varName := ""
	if swapped, name, err := p.applyVariable(out.Intent); err != nil {
		out.Status = directorapi.ResultFailed
		out.Message = err.Error()
		add("variable", err.Error(), false)
		return out
	} else if name != "" {
		out.Intent, varName = swapped, name
		add("variable", fmt.Sprintf("$%s → %s", name, describeQuery(swapped)), true)
	}

	// ── Observe ───────────────────────────────────────────────────────────────
	world, err := p.observeTraced(ctx)
	if err != nil {
		out.Error = err.Error()
		out.Message = "could not observe the screen: " + err.Error()
		add("observe", out.Error, false)
		return out
	}
	out.World = &world
	add("observe", fmt.Sprintf("%d elements, coverage %.2f, actionable %.2f",
		len(world.Elements), world.Confidence.Coverage, world.Confidence.Actionability), true)

	// ── Resolve + plan + policy ───────────────────────────────────────────────
	built, decision, res, status, msg, diag := p.prepareTraced(ctx, request, world,
		out.Intent, false, add)
	out.Binding = diag
	out.Resolution = res
	out.Policy = decision
	// The variable, if any, learns whether its query still finds its object.
	p.recordVariableOutcome(varName, res)
	if status != "" {
		out.Status = status
		out.Message = msg
		return out
	}
	out.Plan = built

	// ── Execute → observe → verify, with one retry ────────────────────────────
	record := p.attempt(ctx, request, out.Intent, *built, world, res, decision, diag, add)

	// The single permitted recovery: verification failed, so look again, re-resolve
	// the target from a fresh world, and try once more.
	//
	// Deliberately not a loop, and deliberately not "retry the same coordinates".
	// The overwhelmingly common cause of a failed verification is that the target
	// moved or the screen changed under us, which a fresh observation fixes; the
	// second most common is that the action genuinely does not work, which repeating
	// will not fix and which repeating makes worse.
	//
	// The unchanged-screen guard is the important part, and it came from watching
	// this run against a live Chrome. Clicking Back navigated the page, but the
	// navigation had not finished within the settle delay, so verification failed and
	// the retry clicked Back a SECOND time — sending the browser two pages back when
	// the user asked for one. Most actions are not idempotent, and "I could not
	// confirm it" is not the same as "it did not happen".
	//
	// So a retry is permitted only when the screen is byte-for-byte what it was
	// before: nothing happened at all, and doing it again cannot double-apply. If
	// anything changed, the honest report is that the action landed but could not be
	// confirmed.
	//
	// The pixels now get a say, and they are the only source that can distinguish
	// "nothing happened" from "it is happening right now" — a page mid-navigation has
	// an unchanged accessibility tree, because the new one does not exist yet.
	verdict := visualRetryVerdict(visualChangeOf(record), changed(record))
	if verdict.Wait {
		// Still settling. Not a retry and not a failure: the honest report is that the
		// action may be taking effect and could not be confirmed in time.
		add("recover", "not retrying: "+verdict.Reason, false)
		record.Status = directorapi.ActionUnverified
		record.Verification.Inconclusive = true
		record.Verification.Reason = verdict.Reason
		record.FailureReason = verdict.Reason
	}
	// An edit is never retried on an INCONCLUSIVE verification, whatever the pixels
	// say. The unchanged-screen guard below reasons about whether the action landed;
	// for an edit that reasoning is unavailable, because an edit that landed perfectly
	// can leave the screen identical (selecting text, copying) and an edit that landed
	// can be unreadable. Repeating an append that already worked appends twice, and
	// "I could not confirm it" is not "it did not happen".
	if isEditAction(*built) && record.Verification.Inconclusive {
		verdict.Allow = false
		verdict.Reason = "an edit that could not be confirmed is not repeated, because repeating it may apply it twice"
	}
	if !record.Success && record.Execution.Performed && verdict.Allow {
		add("recover", "verification failed — re-observing and re-resolving once", true)
		out.Retried = true

		retryWorld, rerr := p.observeTraced(ctx)
		if rerr != nil {
			add("recover", "could not re-observe: "+rerr.Error(), false)
		} else {
			retryBuilt, retryDecision, retryRes, retryStatus, retryMsg :=
				// Already confirmed: the retry is only reached when the first attempt
				// EXECUTED, which it could not have done without passing the gate.
				p.prepare(ctx, request, retryWorld, out.Intent, true, add)
			if retryStatus != "" {
				add("recover", "the target could not be resolved again: "+retryMsg, false)
			} else {
				retryRecord := p.attempt(ctx, request, out.Intent, *retryBuilt,
					retryWorld, retryRes, retryDecision, diag, add)
				retryRecord.Attempts = record.Attempts + retryRecord.Attempts
				record = retryRecord
				out.Plan = retryBuilt
				out.Resolution = retryRes
			}
		}
	}

	// An action that visibly changed something but could not be confirmed is
	// UNVERIFIED, not failed. Reporting it as a failure invites the caller to do it
	// again, which is exactly the double-application the retry guard prevents.
	if !record.Success && record.Execution.Performed && changed(record) &&
		record.Status == directorapi.ActionFailed {
		record.Status = directorapi.ActionUnverified
		record.Verification.Inconclusive = true
		record.FailureReason = "the screen changed but not in a way that confirms the action; " +
			"not retrying, because repeating it could apply it twice"
		add("verify", record.FailureReason, false)
	}

	// The execution becomes a durable semantic node. This is the point where a
	// transient result turns into knowledge: the record describes what happened, the
	// node describes what was MEANT and what came of it, in terms that will still be
	// true after the window has moved and the element ids have all been reissued.
	//
	// Failures are recorded too. History that only holds successes cannot explain
	// anything, and a later replay attempt appends a further node rather than editing
	// this one.
	// The plaintext goes no further than this line.
	//
	// After execution and after verification — both genuinely needed it — and before
	// the node exists, because the node is durable the moment it is added. This is the
	// only point at which both conditions hold.
	// out.Plan, not `built`. A retry re-plans, and out.Plan is then the SECOND plan
	// while `built` is the one that was superseded — redacting the original would clean
	// a plan nobody keeps and leave the retained one carrying the value. The audit test
	// caught exactly that, because the scripted world makes the edit fail verification
	// and so takes the retry path.
	redactHistory(used, &out.Intent, &record, built, out.Plan)
	redactStages(used, out.Stages)

	// The value REACHED execution. Recorded now rather than at lookup time, because a
	// value that was read and then refused by policy was never consumed by anything —
	// and "which steps consumed it" has to mean what it says.
	if used != nil {
		op, _ := out.Intent.Parameters["operation"].(string)
		p.emitValue(trace.ValueEvent{
			Kind: trace.EventValueConsumed, Name: used.Ref.Name,
			Visibility: string(used.Val.Visibility()), Operation: op,
			DestinationRole:        string(record.Target.Role),
			DestinationApplication: applicationOf(world, record.Target.WindowID),
			Outcome:                string(record.Status),
		})
		pctx.Values.RecordConsumption(used.Ref.Name, values.Consumption{
			StepID: p.stepID, StepIndex: p.stepIndex, Operation: op,
			ConsumedAt: p.now(), Outcome: string(record.Status),
		})
	}

	node := actiongraph.FromExecution(actiongraph.NodeID(record.ID), actiongraph.Source{
		Record: record,
		Intent: out.Intent,
		Plan:   *built,
		World:  &world,
		Parent: firstParent(parent, p.parentFor(record)),
	})
	// Every consumed value is noted, public or not: replay must refuse all of them,
	// because the program that captured them is over either way.
	noteValueUse(&node, used)
	// Collection lineage, when this action was one member of a bounded set.
	noteIteration(&node, p.iteration)
	// Which goal and procedure this action came from, when it came from one.
	noteGoalProvenance(&node, p.goalProvenance, p.stepIndex, p.stepCount, p.stepID)
	// And WHICH concrete object it was aimed at, when the request pointed at one. The
	// snapshot is taken AFTER revalidation, so a node records the object that was
	// actually acted on rather than the one resolved a second earlier.
	noteBinding(&node, bindingFor(ctx, out.Intent))
	// And the inline editor it was carried out in, when it was carried out in one. A
	// snapshot, so history says what was edited without offering a replay a handle to
	// a control that has since closed.
	noteEditor(&node, diag)
	// And the handle it was pinned to goes no further than the request that derived it.
	unpinEditor(&node)
	// And what the confirmation gate concluded, as a RECORD. Nothing reads it back as
	// permission — see the replay policy, which discloses it and asks again anyway.
	if diag != nil && diag.Confirmation != "" {
		node.Confirmation = string(diag.Confirmation)
	}
	if err := p.Graph.Add(node); err != nil {
		add("record", "could not record the action: "+err.Error(), false)
	} else {
		add("record", fmt.Sprintf("%s (%s)", node.ID, node.SemanticKey()), true)
	}
	out.Node = &node
	out.Record = &record

	switch {
	case record.Success:
		out.Status = directorapi.ResultDone
		out.Message = record.Verification.Reason
	case record.Status == directorapi.ActionUnverified:
		out.Status = directorapi.ResultPartial
		out.Message = record.FailureReason
	default:
		out.Status = directorapi.ResultFailed
		out.Message = record.FailureReason
	}
	// The account is completed LAST, when everything in it is known: the capability the
	// ladder ran, what verification made of it, which node it became, and how the whole
	// request ended.
	diag.finish(record, out.Node, out.Status)
	return out
}

// changed reports whether the screen differs at all between an action's before and
// after states. The fingerprint covers element identity, labels, geometry and the
// state flags a user would notice — so "unchanged" means genuinely nothing happened,
// not merely that nothing recognisable happened.
func changed(record directorapi.ActionRecord) bool {
	if record.Before.Fingerprint == "" || record.After.Fingerprint == "" {
		// Unknown. Treated as CHANGED, because the guard's job is to prevent
		// double-application and the safe assumption is that the action landed.
		return true
	}
	return record.Before.Fingerprint != record.After.Fingerprint
}

// prepare resolves the target, builds a plan, re-checks the binding and evaluates policy
// against one world. Returning a non-empty status means the request stops here.
//
// The order inside is the safety argument, and it is the order the milestone requires:
//
//	resolve → plan (lowering makes the action as specific as it will get) →
//	REVALIDATE the binding → policy → confirmation → (caller) execute
//
// Revalidation before policy and therefore before confirmation, because a prompt written
// against a stale binding asks the user to agree to something other than what would
// happen. Confirmation after lowering, because before it the action is not yet specific
// enough for the question to be honest.
//
// confirmed is set on the internal retry, where the user has already agreed to this exact
// action a moment ago. Re-asking there would put the same question twice for one request
// and teach the user that the prompts are noise — but it is honoured only while the target
// is unchanged, because a retry whose binding was re-established somewhere material asks
// again.
func (p *Pipeline) prepare(ctx context.Context, request string, world directorapi.WorldState,
	in directorapi.Intent, confirmed bool, add func(string, string, bool)) (
	*directorapi.Plan, *directorapi.PolicyDecision, *directorapi.Resolution,
	directorapi.ResultStatus, string) {

	built, decision, res, status, msg, _ := p.prepareTraced(ctx, request, world, in, confirmed, add)
	return built, decision, res, status, msg
}

// prepareTraced is prepare with the DIAGNOSTIC account it produces along the way.
//
// Split out rather than widening every caller, because only the single-step path records
// the account on its Outcome — a replay iteration and a retry have their own reporting and
// would discard it.
func (p *Pipeline) prepareTraced(ctx context.Context, request string, world directorapi.WorldState,
	in directorapi.Intent, confirmed bool, add func(string, string, bool)) (
	*directorapi.Plan, *directorapi.PolicyDecision, *directorapi.Resolution,
	directorapi.ResultStatus, string, *BindingDiagnostics) {

	diag := &BindingDiagnostics{}

	// The DERIVED target, before anything else looks at the intent. A step acting on an
	// editor a previous step opened must be pinned to that editor now, from this step's
	// own observation — and must stop if the editor is not there, is for something else,
	// or is one of several. Nothing falls back to "whatever holds focus": that is what
	// typed a filename into a details-view cell and renamed nothing.
	if pinned, v, wanted := p.deriveEditor(ctx, world, in); wanted {
		diag.Editor = &v
		add("editor", v.Describe(), v.Result.OK())
		if !v.Result.OK() {
			return nil, nil, nil, statusForEditor(v.Result), v.Reason, diag
		}
		in = pinned
	}

	input := plan.Input{Intent: in, World: &world}

	if in.Verb == "move_window" {
		win, ok := p.windowFor(world, in)
		if !ok {
			add("resolve", "no window to move", false)
			return nil, nil, nil, directorapi.ResultFailed, "I could not tell which window to move", diag
		}
		input.TargetWindow = &win
		input.Placement, _ = in.Parameters["placement"].(string)
		if p.Monitors != nil {
			mons, err := p.Monitors(ctx)
			if err != nil {
				add("resolve", "could not read the display layout: "+err.Error(), false)
				return nil, nil, nil, directorapi.ResultFailed, "I could not read the display layout", diag
			}
			input.Monitors = mons
		}
		add("resolve", fmt.Sprintf("window %q on %s", win.Title, win.MonitorID), true)
	} else {
		query := directorapi.ElementQuery{}
		if len(in.Targets) > 0 && in.Targets[0].Query != nil {
			query = *in.Targets[0].Query
		}
		// Resolution is pure computation over an already-observed world, so its
		// deadline exists to catch a pathological candidate set rather than a hang.
		var res directorapi.Resolution
		_ = trace.Do(ctx, p.Trace, trace.PhaseResolve, p.stepMeta(trace.PhaseResolve),
			func(context.Context) error {
				res = p.Resolver.Resolve(&world, query)
				return nil
			})
		input.Resolution = res

		if res.Status != directorapi.ResolutionResolved {
			add("resolve", string(res.Status)+": "+res.Explanation, false)
			return nil, nil, &res, statusFor(res), res.Explanation, diag
		}
		add("resolve", fmt.Sprintf("%s %q (%.2f)", res.Target.Role, res.Target.Label,
			res.Target.Confidence), true)
	}

	built, err := p.Planner.Build(input)
	if err != nil {
		add("plan", err.Error(), false)
		return nil, nil, resPtr(input.Resolution), directorapi.ResultFailed, err.Error(), diag
	}
	planLine := fmt.Sprintf("%s (%s risk)", built.Steps[0].Action.Describe(), built.Risk)
	for _, a := range built.Assumptions {
		// Assumptions are written down so a failure can be explained by naming the one
		// that broke — which only works if the user saw them.
		planLine += "\n            note: " + a
	}
	add("plan", planLine, true)

	// ── Revalidate the binding ────────────────────────────────────────────────
	// The latest safe point: the actionable target is known and NOTHING externally
	// observable has happened. A stale or re-focused target stops here, before the
	// confirmation is written and long before the executor is reached.
	initial := bindingFor(ctx, in)
	reval := p.revalidateBinding(ctx, world, in)
	diag.record(reval, initial)
	if reval.ID != "" {
		add("binding", reval.Describe(), reval.OK())
	}
	if !reval.OK() {
		status := directorapi.ResultFailed
		if reval.Problem.Clarifiable() {
			status = directorapi.ResultNeedsClarification
		}
		return nil, nil, resPtr(input.Resolution), status, reval.Problem.Message, diag
	}

	decision := p.Policy.EvaluatePlan(ctx, built, world)
	if !decision.Allowed {
		add("policy", "refused: "+decision.Reason, false)
		return nil, &decision, resPtr(input.Resolution), directorapi.ResultBlocked, decision.Reason, diag
	}
	if decision.RequiresConfirmation && confirmed && materialChange(reval) == "" {
		add("confirm", "already agreed to a moment ago, and the target has not changed", true)
	} else if decision.RequiresConfirmation {
		// The action-level gate. An accepted GOAL-level confirmation may already answer
		// it — the user agreed to the whole procedure, and asking again for each of its
		// steps would train them to click through prompts — but only when it covers this
		// action's effect, target, risk and destructiveness. See
		// coveredByGoalConfirmation for each clause and the case behind it.
		act := actionConfirmationFor(in, built, reval, decision)
		outcome, cover, message := p.confirmAction(ctx, request, act)
		diag.Confirmation, diag.Coverage = outcome, &cover
		if !cover.Covered {
			asked := p.confirmationRequestFor(ctx, request, act)
			diag.Request = &asked
		}
		switch outcome {
		case ConfirmationNotRequired:
			add("policy", "allowed: "+cover.Reason, true)
		case ConfirmationAccepted:
			add("confirm", "you agreed to "+built.Steps[0].Action.Describe(), true)
		default:
			// Rejected, unavailable or cancelled: nothing executes. The three are
			// reported differently because they are different — one is the user saying
			// no, one is a Director that cannot ask, one is a request abandoned.
			add("confirm", string(outcome)+": "+message, false)
			return nil, &decision, resPtr(input.Resolution), statusForConfirmation(outcome), message, diag
		}
	} else {
		add("policy", "allowed", true)
	}

	return &built, &decision, resPtr(input.Resolution), "", "", diag
}

// statusForConfirmation maps a refused confirmation onto the request's headline status.
//
// A rejection is CANCELLED: the user was asked and said no, which is the system working
// rather than failing. Unavailable is BLOCKED — the request was reasonable and the
// Director could not carry it out safely, a different problem with a different fix. A
// cancellation is cancelled for the same reason a rejection is: nothing happened and
// nobody was let down.
func statusForConfirmation(o ConfirmationOutcome) directorapi.ResultStatus {
	switch o {
	case ConfirmationRejected, ConfirmationCancelled:
		return directorapi.ResultCancelled
	}
	return directorapi.ResultBlocked
}

// actionConfirmationFor assembles the question for one lowered, revalidated action.
func actionConfirmationFor(in directorapi.Intent, built directorapi.Plan, reval revalidation,
	decision directorapi.PolicyDecision) actionConfirmation {

	act := actionConfirmation{
		Action: built.Steps[0].Action.Describe(),
		Risk:   built.Risk,
		Reason: decision.Reason,
		Effect: built.Goal,
		// Read off the LOWERED action, not off the procedure's declaration about
		// itself: lowering is exactly where an innocuous-sounding verb becomes an
		// irreversible one, and the coverage rule needs to know that here rather than
		// where the goal described its own intentions.
		Destructive:    destructiveAction(built.Steps[0].Action),
		MaterialChange: materialChange(reval),
		Binding:        reval.Binding,
	}
	if len(in.Targets) > 0 {
		act.Target = in.Targets[0].Phrase
	}
	if b := reval.Binding; b.Bound() {
		act.Label, act.Resource = b.Label, b.Resource
		if b.Resource != "" {
			act.Target = b.Resource
		}
	}
	return act
}

// destructiveAction reports whether a lowered action removes or overwrites something.
//
// Answered from the SEMANTIC vocabulary's own declaration rather than from a keyword
// search on the description: "delete" is irreversible because the vocabulary says the
// verb is, and a plan whose description happens to contain the word is not.
func destructiveAction(a directorapi.Action) bool {
	sa, ok := a.(directorapi.SemanticAction)
	if !ok {
		return false
	}
	return !sa.Kind.Reversible()
}

// attempt executes one step, re-observes and verifies, producing a record.
//
// diag is the account being assembled for this step, and may be nil: a replay iteration
// and a retry keep their own reporting. What attempt adds to it is what only running the
// action can establish — what the inline editor held afterwards.
func (p *Pipeline) attempt(ctx context.Context, request string, in directorapi.Intent,
	built directorapi.Plan, before directorapi.WorldState, res *directorapi.Resolution,
	decision *directorapi.PolicyDecision, diag *BindingDiagnostics,
	add func(string, string, bool)) directorapi.ActionRecord {

	step := built.Steps[0]
	resolved := directorapi.ResolvedTarget{}
	if res != nil && res.Target != nil {
		resolved = *res.Target
	}
	if in.Verb == "move_window" {
		if mv, ok := step.Action.(directorapi.MoveWindowAction); ok {
			resolved.WindowID = mv.Window.ID
			resolved.Label = mv.Window.Description
		}
	}

	record := directorapi.ActionRecord{
		ID:              directorapi.ActionID(p.nextID()),
		RequestedIntent: request,
		Action:          step.Action,
		Target:          resolved,
		StartedAt:       p.now(),
		Before:          memory.Summarise(before),
		Status:          directorapi.ActionRunning,
		Attempts:        1,
		Policy:          decision,
	}
	record.Reversible, record.UndoAction = undoFor(step.Action, before)

	// Watch the region the effect is expected in, BEFORE acting. Cheap and bounded:
	// one small rectangle, not the desktop, and only when a watcher is wired.
	var watch RegionSnapshot
	if p.Watcher != nil {
		if region, ok := watchRegion(resolved, before); ok {
			watch, _ = p.Watcher.Before(ctx, region)
		}
	}

	// The generous one. This is a real application receiving real input, and a timeout
	// here does NOT mean the action did not happen — see trace.ErrPhaseTimeout.
	var outcome directorapi.ExecutionOutcome
	err := trace.Do(ctx, p.Trace, trace.PhaseRuntimeExecute,
		p.stepMeta(trace.PhaseRuntimeExecute), func(ctx context.Context) error {
			var e error
			outcome, e = p.Executor.Execute(ctx, step.Action)
			return e
		})
	record.Execution = outcome
	if err != nil {
		add("execute", "failed: "+err.Error(), false)
		if line := loweringTrace(outcome); line != "" {
			// Most valuable exactly here: a compile failure means the desktop was
			// never touched, and a reader needs to know which it was.
			add("marco", line, false)
		}
		record.Status = directorapi.ActionFailed
		record.FailureReason = err.Error()
		record.After = record.Before
		record.CompletedAt = timePtr(p.now())
		return record
	}
	add("execute", outcome.Detail, true)
	if line := loweringTrace(outcome); line != "" {
		// Which Marco capability ran, whether it compiled, and whether it ran. The
		// trace is where "the Director executes legal Marco" stops being a claim in a
		// document and becomes something a reader can see happening.
		add("marco", line, true)
	}

	// Let the interface settle before looking again — by WATCHING it settle, not by
	// guessing how long that takes. See settleFor.
	var settled *engine.Result
	_ = trace.Do(ctx, p.Trace, trace.PhaseSettle, p.stepMeta(trace.PhaseSettle),
		func(ctx context.Context) error {
			settled = p.settleFor(ctx, step.Action, resolved, before, add)
			return nil
		})
	if settled != nil {
		record.SettleWait = settled.Summary()
	}

	// What the pixels did. This follows a still-changing region for a bounded number
	// of rounds, which is the whole reason a slow navigation no longer invites a
	// second click.
	var change RegionChange
	if p.Watcher != nil && watch.Valid {
		change, _ = p.Watcher.After(ctx, watch)
		if change.Valid {
			add("verify", "region "+describeChange(change), !change.StillChanging)
		}
	}
	record.VisualChange = visualChangeSummary(change)

	after, oerr := p.observeTraced(ctx)
	if oerr != nil {
		add("verify", "could not re-observe: "+oerr.Error(), false)
		record.Status = directorapi.ActionUnverified
		record.Verification = directorapi.VerificationResult{
			Inconclusive: true,
			Reason:       "could not observe the screen after acting: " + oerr.Error(),
		}
		record.FailureReason = record.Verification.Reason
		record.After = record.Before
		record.CompletedAt = timePtr(p.now())
		return record
	}
	record.After = memory.Summarise(after)

	_ = trace.Do(ctx, p.Trace, trace.PhaseVerify, p.stepMeta(trace.PhaseVerify),
		func(context.Context) error {
			record.Verification = p.Verifier.Verify(step.Action, resolved, before, after)
			return nil
		})

	// An interface still in flight is not an interface where the action failed. Rather
	// than retry — which double-applies — or declare Unverified immediately, wait for
	// the region to settle and verify AGAIN. The action is never repeated; only the
	// verification is.
	if !record.Verification.Success && change.Valid && change.StillChanging {
		if settled, w2, ok := p.awaitStable(ctx, resolved, before, add); ok {
			record.SettleWait = settled
			after = w2
			record.After = memory.Summarise(after)
			record.Verification = p.Verifier.Verify(step.Action, resolved, before, after)
			add("verify", "re-verified after the region settled: "+record.Verification.Reason,
				record.Verification.Success)
		}
	}
	// Visual evidence is APPENDED to the verdict, never substituted for it. A region
	// changing after a click is consistent with the click working and equally consistent
	// with an unrelated repaint, so it may strengthen a conclusion and may not reach one.
	if ev, ok := visualEvidence(change); ok {
		record.Verification.Evidence = append(record.Verification.Evidence, ev)
	}

	// Did the action land on the object that was BOUND? A verdict of "something changed
	// in the right way" is satisfied by the right thing happening to the wrong object,
	// which is the failure the whole binding layer exists to prevent — so it is checked
	// here, against identity, after execution and against the observed target.
	//
	// A demonstrated mismatch OVERRIDES a successful verdict. An inconclusive
	// correlation does not: plenty of actions leave nothing to compare, and failing
	// those would make the check noise people learn to ignore.
	// Did the EDIT land in the editor it was aimed at? A capability that returned
	// success says a call returned; it does not say the box now holds the new name. The
	// previous live attempt had exactly that: a successful set_text into a details-view
	// cell that renames nothing.
	if v, ok := p.verifyEditorValue(ctx, after, in, editedText(in)); ok {
		record.Verification.Evidence = append(record.Verification.Evidence,
			directorapi.Evidence{Kind: "inline_editor_value", Observed: v.Result.OK(),
				Detail: v.Describe(), Weight: 1})
		if diag != nil {
			diag.EditorOutcome = &v
		}
		if !v.Result.OK() {
			record.Verification.Success = false
			record.Verification.Inconclusive = false
			record.Verification.Reason = v.Reason
			add("verify", "the editor does not hold the new name: "+v.Reason, false)
		}
	}
	// And did a COMMIT end the edit mode? Half the answer — the filesystem correlation
	// is the other half, and says so.
	if isCommit(in) {
		if v, ok := p.verifyEditorClosed(ctx, after, in); ok {
			record.Verification.Evidence = append(record.Verification.Evidence,
				directorapi.Evidence{Kind: "inline_editor_closed", Observed: v.Result.OK(),
					Detail: v.Describe(), Weight: 0.5})
			if diag != nil {
				diag.EditorOutcome = &v
			}
			if !v.Result.OK() {
				record.Verification.Success = false
				record.Verification.Inconclusive = false
				record.Verification.Reason = v.Reason
				add("verify", "the edit mode did not end: "+v.Reason, false)
			} else {
				// An action whose only observable effect is a box closing verifies on
				// that, because the generic comparison cannot see a rename.
				record.Verification.Success = true
				record.Verification.Reason = v.Reason
				add("verify", v.Reason, true)
			}
		}
	}

	if corr, ok := p.correlateTarget(ctx, in, resolved, after); ok {
		record.Verification.Evidence = append(record.Verification.Evidence, corr.AsEvidence())
		if corr.Result == verify.Uncorrelated {
			record.Verification.Success = false
			record.Verification.Inconclusive = false
			record.Verification.Reason = corr.Reason
			add("verify", "wrong object: "+corr.Reason, false)
		}
	}
	record.Success = record.Verification.Success
	switch {
	case record.Verification.Success:
		record.Status = directorapi.ActionSucceeded
		add("verify", record.Verification.Reason, true)
	case record.Verification.Inconclusive:
		record.Status = directorapi.ActionUnverified
		record.FailureReason = record.Verification.Reason
		add("verify", "inconclusive: "+record.Verification.Reason, false)
	default:
		record.Status = directorapi.ActionFailed
		record.FailureReason = record.Verification.Reason
		add("verify", "failed: "+record.Verification.Reason, false)
	}
	record.CompletedAt = timePtr(p.now())
	return record
}

// windowFor picks the window a move applies to: the named one, else the active one.
func (p *Pipeline) windowFor(w directorapi.WorldState, in directorapi.Intent) (directorapi.Window, bool) {
	if len(in.Targets) > 0 {
		want := lower(in.Targets[0].Phrase)
		for _, win := range w.Windows {
			if contains(lower(win.Title), want) || contains(lower(win.Application), want) {
				return win, true
			}
		}
		return directorapi.Window{}, false
	}
	if win, ok := w.FocusedWindow(); ok {
		return *win, true
	}
	if len(w.Windows) == 1 {
		return w.Windows[0], true
	}
	return directorapi.Window{}, false
}

// undoFor builds the action that would reverse this one, where one exists.
//
// Only window moves are reversible today, because only they have a captured prior
// state that can be restored. A click cannot be un-clicked, and saying otherwise
// would make "undo the last action" dangerous the moment it is implemented — which
// is exactly why the flag is set honestly now rather than later.
func undoFor(action directorapi.Action, before directorapi.WorldState) (bool, directorapi.Action) {
	mv, ok := action.(directorapi.MoveWindowAction)
	if !ok {
		return false, nil
	}
	win, found := before.Window(mv.Window.ID)
	if !found || win.Bounds.Empty() {
		return false, nil
	}
	original := win.Bounds
	return true, directorapi.MoveWindowAction{
		Window:    mv.Window,
		Placement: directorapi.WindowPlacement{Bounds: &original},
	}
}

// statusFor maps a resolution outcome onto the request's headline status.
//
// Ambiguity is the only one that asks the user, because it is the only one they can
// resolve. "Unobservable" and "absent" both stop, but they stop for different
// reasons and their explanations must not be interchangeable — one means the
// Director could not see, the other that there was nothing to see.
func statusFor(res directorapi.Resolution) directorapi.ResultStatus {
	if res.Status == directorapi.ResolutionAmbiguous {
		return directorapi.ResultNeedsClarification
	}
	return directorapi.ResultFailed
}

func resPtr(res directorapi.Resolution) *directorapi.Resolution {
	if res.Status == "" {
		return nil
	}
	return &res
}

func (p *Pipeline) settle() time.Duration {
	if p.SettleDelay > 0 {
		return p.SettleDelay
	}
	return DefaultSettleDelay
}

func (p *Pipeline) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

func timePtr(t time.Time) *time.Time { return &t }

func targetPhrase(in directorapi.Intent) string {
	if len(in.Targets) > 0 {
		return fmt.Sprintf("%q", in.Targets[0].Phrase)
	}
	if p, ok := in.Parameters["placement"].(string); ok {
		return p
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func contains(s, sub string) bool {
	if sub == "" {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// nextID mints the next action id from the graph, so ids stay unique across the
// whole history rather than restarting each run.
func (p *Pipeline) nextID() string {
	type minter interface{ NextID() actiongraph.NodeID }
	if m, ok := p.Graph.(minter); ok {
		return string(m.NextID())
	}
	return fmt.Sprintf("action_%d", time.Now().UnixNano())
}

// parentFor links an action to the one it followed on from.
//
// A parent is claimed only when the previous action left the screen in the state
// this one started from — matching fingerprints. That is what distinguishes "click
// Save, having just opened the File menu" from two unrelated actions that merely
// happened in sequence, and it is what will make workflow chains meaningful rather
// than merely chronological.
func (p *Pipeline) parentFor(record directorapi.ActionRecord) *actiongraph.NodeID {
	last, err := p.Graph.Last()
	if err != nil {
		return nil
	}
	if last.Outcome.After.Fingerprint == "" || record.Before.Fingerprint == "" {
		return nil
	}
	if last.Outcome.After.Fingerprint != record.Before.Fingerprint {
		return nil
	}
	id := last.ID
	return &id
}

// HandleRefined runs a phrase with a clarification answer applied.
//
// The answer narrows the QUERY and the whole request is then resolved again from
// scratch. It deliberately does not act on the element ids that were offered: the
// user was thinking while the screen carried on, and an id captured before the
// question was asked may by then point at something else.
func (p *Pipeline) HandleRefined(ctx context.Context, phrase string, refine func(*directorapi.ElementQuery)) Outcome {
	prior := p.Intent
	p.Intent = func(s string) directorapi.Intent {
		in := prior(s)
		if refine != nil && len(in.Targets) > 0 && in.Targets[0].Query != nil {
			refine(in.Targets[0].Query)
		}
		return in
	}
	defer func() { p.Intent = prior }()
	return p.Handle(ctx, phrase)
}

// firstParent prefers an explicit parent over the inferred one.
//
// A program step's parent is the step before it — a fact the pipeline cannot infer,
// because the inferred link looks at what the action targeted rather than at what came
// before it in a sequence.
func firstParent(explicit, inferred *actiongraph.NodeID) *actiongraph.NodeID {
	if explicit != nil {
		return explicit
	}
	return inferred
}
