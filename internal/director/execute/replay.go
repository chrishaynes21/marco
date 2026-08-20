package execute

import (
	"context"
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Semantic replay: doing again what was done before, by MEANING rather than by
// repetition.
//
// The distinction is the whole feature. A macro recorder replays the events — the
// same coordinates, the same element handles, the same rectangle — and is wrong the
// moment anything has moved. Semantic replay throws all of that away and re-derives
// it: what did the user mean, what does that resolve to NOW, what plan does that
// need, did it work. The stored node contributes the INTENT and nothing else.
//
// Concretely, and enforced by tests:
//
//   - the stored plan is never executed; a fresh one is built each iteration;
//   - the stored element id is never reused; the target is re-resolved from its query;
//   - the stored coordinates are never sent; they exist only as historical evidence;
//   - every iteration is a full Observe → Resolve → Plan → Policy → Execute →
//     Observe → Verify cycle, exactly as if the user had typed the request again.
//
// What replay adds on top of that is the loop, and the loop is where an agent gets
// dangerous. So it is bounded three ways at once — a requested count, a hard
// ceiling, and progress detection — and it stops on the first of them to trip.

// ReplayConfidence rates how much this replay resembles the action it came from.
//
// It is diagnostic rather than a gate: AnalyzeReplay already decides whether replay
// may proceed. This answers the different question of whether the thing about to
// happen is really the same thing that happened before, which is what a user
// deserves to be told when the answer is "not very".
type ReplayConfidence struct {
	// Intent is whether the action being performed is the one that was recorded.
	Intent float64 `json:"intent"`
	// Target is how well the freshly resolved target matches the recorded one — role,
	// label, and whatever identity was durable enough to survive.
	Target float64 `json:"target"`
	// Context is whether the surroundings are the same: the same application, the
	// same window, a comparably observable world.
	Context float64 `json:"context"`
	// Overall is the weakest of the three. A replay is only as trustworthy as its
	// least convincing dimension, and averaging would let a perfect intent match
	// paper over acting in the wrong application.
	Overall float64 `json:"overall"`
	// Notes explain any dimension that scored below full marks.
	Notes []string `json:"notes,omitempty"`
}

// ReplaySpec is a request to repeat a recorded action.
type ReplaySpec struct {
	// Node is the action to repeat.
	Node actiongraph.ActionNode
	// Count is how many iterations the user asked for.
	Count int
	// MaxIterations is the hard ceiling, applied whatever was asked. A count that
	// came from a misheard word ("fifty" for "five") must not become fifty clicks.
	MaxIterations int
	// StopCheck reports whether the user has asked to stop. It is polled between
	// iterations, alongside context cancellation, because the two arrive by
	// different routes: a context is cancelled in-process, while "stop" spoken to a
	// front-end that spawns a new process each time cannot cancel anything directly.
	StopCheck func() bool
	// Consent is what the user explicitly agreed to when they asked for the repeat.
	// Optional, and a vague "do that again" is correctly represented by leaving it nil:
	// it covers nothing, and every question is asked afresh.
	Consent *ReplayConsent
}

// DefaultMaxIterations bounds any single replay request.
const DefaultMaxIterations = 25

// ReplayIteration is one pass of the loop.
type ReplayIteration struct {
	Index      int                         `json:"index"`
	Analysis   actiongraph.ReplayCandidate `json:"analysis"`
	Confidence ReplayConfidence            `json:"confidence"`
	Record     *directorapi.ActionRecord   `json:"record,omitempty"`
	Node       *actiongraph.ActionNode     `json:"node,omitempty"`
	Verified   bool                        `json:"verified"`
	Reason     string                      `json:"reason"`
}

// ReplayOutcome is the result of a whole replay request.
type ReplayOutcome struct {
	Source     actiongraph.ActionNode `json:"source"`
	Requested  int                    `json:"requested"`
	Completed  int                    `json:"completed"`
	Iterations []ReplayIteration      `json:"iterations,omitempty"`

	Status  directorapi.ResultStatus `json:"status"`
	Message string                   `json:"message"`
	// StoppedBecause names why the loop ended: "completed", "cancelled",
	// "not_replayable", "unverified", "no_progress", "ceiling".
	StoppedBecause string `json:"stopped_because"`
	// Confidence is the first iteration's, which is the one that decides whether
	// this is really the same action.
	Confidence ReplayConfidence `json:"confidence"`
}

// Replay repeats a recorded action, semantically.
func (p *Pipeline) Replay(ctx context.Context, spec ReplaySpec) ReplayOutcome {
	ctx = ensureBindings(ctx)
	// Explicit consent, when it was specific enough to mean something. Installed as the
	// same request-scoped coverage a goal confirmation produces, so one rule decides
	// whether a question has already been answered rather than two that could disagree.
	ctx = withReplayConsent(ctx, spec.Consent)
	out := ReplayOutcome{Source: spec.Node, Requested: spec.Count}

	// An action that consumed a program-local value cannot be replayed alone.
	//
	//	A program-local value can be replayed only by replaying the program that
	//	captures it — not by resurrecting its old plaintext.
	//
	// Every alternative is worse than refusing. Replaying the historical text would type
	// a value read from a screen that has since changed, into a world that has moved on;
	// re-reading it would need the control it came from, which the graph deliberately
	// does not keep; and turning it into a literal would silently convert "type what you
	// captured" into "type this exact string forever". The value is not stored anywhere,
	// which is what makes this refusal honest rather than a policy someone could relax.
	//
	// Checked FIRST, before the ceiling and before any iteration, because nothing about
	// the request can make it replayable.
	// An action performed as one member of a bounded set cannot be replayed from that
	// set's history.
	//
	//	Whole-collection replay is not implemented.
	//
	// Replaying the one member would be replaying it out of the sequence that gave it
	// meaning — "the third of six" is not a thing to do once — and the membership it
	// belonged to is gone: the collection was program-local and its program has ended.
	// Reconstructing the set from the iteration nodes would resurrect exactly the stale
	// member list the whole design refuses to keep.
	if name, member := PartOfCollection(spec.Node); member {
		out.Status = directorapi.ResultBlocked
		out.StoppedBecause = "not_replayable"
		out.Message = fmt.Sprintf(
			"Whole-collection replay is not implemented: this action was one member of "+
				"the collection %q, and its membership no longer exists.", name)
		return out
	}

	if name, dependent := DependedOnProgramValue(spec.Node); dependent {
		out.Status = directorapi.ResultBlocked
		out.StoppedBecause = "not_replayable"
		out.Message = fmt.Sprintf(
			"Replay unavailable: this action depended on program-local value %q.", name)
		return out
	}

	count := spec.Count
	if count <= 0 {
		count = 1
	}
	ceiling := spec.MaxIterations
	if ceiling <= 0 {
		ceiling = DefaultMaxIterations
	}
	if count > ceiling {
		out.Message = fmt.Sprintf("%d repeats is more than the limit of %d", count, ceiling)
		out.Status = directorapi.ResultBlocked
		out.StoppedBecause = "ceiling"
		return out
	}

	// Each iteration's node chains to the previous one, so the graph shows the
	// repeat as a sequence rather than as unrelated actions that happen to match.
	parent := spec.Node.ID

	for i := 0; i < count; i++ {
		// Cancellation is checked BEFORE each iteration, so "stop" takes effect
		// between iterations rather than after the loop has finished doing what it
		// was told to stop doing.
		if reason, stopped := p.cancelled(ctx, spec); stopped {
			out.StoppedBecause = "cancelled"
			out.Status = directorapi.ResultCancelled
			out.Message = fmt.Sprintf("%s after %d of %d", reason, out.Completed, count)
			return out
		}

		iter, node, fatal := p.replayOnce(ctx, spec.Node, i, count, parent)
		out.Iterations = append(out.Iterations, iter)
		if i == 0 {
			out.Confidence = iter.Confidence
		}

		if fatal != "" {
			out.StoppedBecause = fatal
			out.Status = statusForStop(fatal, out.Completed)
			out.Message = summarise(fatal, iter.Reason, out.Completed, count)
			return out
		}

		out.Completed++
		if node != nil {
			parent = node.ID
		}

		// Progress. An iteration that left the screen exactly as it found it did
		// nothing, and doing it again will do nothing too — but faster, and for as
		// many more iterations as were asked for. This is the bound that matters
		// when the other two have not tripped.
		if iter.Record != nil && !changed(*iter.Record) {
			out.StoppedBecause = "no_progress"
			out.Status = directorapi.ResultPartial
			out.Message = fmt.Sprintf(
				"stopped after %d of %d: the last repeat changed nothing, so further repeats would too",
				out.Completed, count)
			return out
		}
	}

	out.StoppedBecause = "completed"
	out.Status = directorapi.ResultDone
	out.Message = fmt.Sprintf("repeated %d time(s)", out.Completed)
	return out
}

// replayOnce performs one full cycle. It returns the iteration, the node it
// produced, and a non-empty stop reason when the loop must not continue.
//
// total is carried only so progress can say "4 of 10". Someone who dispatched a replay
// by voice cannot see a progress bar, and ten identical lines do not tell them how much
// is left — or whether it is worth saying "stop".
func (p *Pipeline) replayOnce(ctx context.Context, source actiongraph.ActionNode,
	index, total int, parent actiongraph.NodeID) (ReplayIteration, *actiongraph.ActionNode, string) {

	iter := ReplayIteration{Index: index}
	discard := func(string, string, bool) {}

	// Cancellation is checked at every boundary where stopping is SAFE: before
	// observing, after observing, before executing, and after verifying. Never in
	// the middle of a platform action — a half-sent click is worse than a completed
	// one, and the actuator's calls do not support interruption.
	stop := func() bool {
		select {
		case <-ctx.Done():
			return true
		default:
			return p.StopCheck != nil && p.StopCheck()
		}
	}

	// ── Observe ───────────────────────────────────────────────────────────────
	// Fresh, every iteration. Reusing the previous world would make this a replay of
	// a belief rather than of an action.
	p.progress("observe", index+1, total, "looking at the screen", false)
	world, err := p.observeTraced(ctx)
	if err != nil {
		iter.Reason = "could not observe the screen: " + err.Error()
		return iter, nil, "unobservable"
	}
	if stop() {
		iter.Reason = "stopped after observing"
		return iter, nil, "cancelled"
	}

	// ── Analyse ───────────────────────────────────────────────────────────────
	// The gate. Replay proceeds only when the recorded action still makes sense
	// here, and the analysis says exactly why when it does not.
	analyzer := actiongraph.NewAnalyzer()
	analyzer.Policy = p.Policy
	iter.Analysis = analyzer.Analyze(source, world)
	if !iter.Analysis.Replayable {
		iter.Reason = iter.Analysis.Reason
		return iter, nil, stopReasonFor(iter.Analysis.Status)
	}

	iter.Confidence = confidenceOf(source, iter.Analysis, world)

	// ── Re-establish the recorded target ──────────────────────────────────────
	// A recorded deictic action is aimed at the object the HISTORY names, re-identified
	// by its path or its native id against the world just observed. The word "this" is
	// never read again: it meant something when it was said, and what is focused now is
	// a different question with a different answer.
	in := replayIntent(source)
	in, berr := p.replayBinding(ctx, source, in, world)
	if berr != nil {
		iter.Reason = berr.Error()
		return iter, nil, "target_missing"
	}

	// ── Fresh confirmation for a consequential repeat ─────────────────────────
	// Before the ordinary policy gate, and about the thing policy cannot know: that
	// this is a repeat, that the original run's confirmation is not being reused, and
	// which object it will land on now.
	if outcome, message := p.replayConfirmation(ctx, source, in, index, total); !outcome.Confirmed() {
		iter.Reason = message
		return iter, nil, stopReasonForConfirmation(outcome)
	}
	if stop() {
		iter.Reason = "stopped after confirming"
		return iter, nil, "cancelled"
	}

	// ── Resolve + plan + policy ───────────────────────────────────────────────
	// The recorded INTENT is re-run through the ordinary pipeline. Nothing of the
	// stored plan is reused: it is rebuilt from what is on screen now.
	built, decision, res, status, msg := p.prepare(ctx, source.Intent.Raw, world, in, false, discard)
	if status != "" {
		iter.Reason = msg
		if status == directorapi.ResultNeedsClarification {
			return iter, nil, "ambiguous"
		}
		return iter, nil, "not_replayable"
	}

	// ── Execute → observe → verify ────────────────────────────────────────────
	// The last safe boundary. Past this point the action is in flight and stopping
	// would mean abandoning it half-done.
	if stop() {
		iter.Reason = "stopped before executing"
		return iter, nil, "cancelled"
	}
	p.progress("execute", index+1, total, built.Steps[0].Action.Describe(), false)

	request := fmt.Sprintf("(repeat %d) %s", index+1, source.Intent.Raw)
	record := p.attempt(ctx, request, in, *built, world, res, decision, nil, discard)
	iter.Record = &record
	iter.Verified = record.Success
	iter.Reason = firstNonEmpty(record.Verification.Reason, record.FailureReason)

	// ── Append a brand-new node ───────────────────────────────────────────────
	// Never an edit to the source. The graph gains a fact about this repeat, and the
	// original stays exactly as it was — including if this repeat failed, which is
	// the case where the comparison is most useful.
	node := actiongraph.FromExecution(actiongraph.NodeID(record.ID), actiongraph.Source{
		Record: record,
		Intent: in,
		Plan:   *built,
		World:  &world,
		Parent: &parent,
	})
	node.Metadata["replay_of"] = string(source.ID)
	node.Metadata["replay_iteration"] = index + 1
	node.Metadata["replay_confidence"] = iter.Confidence.Overall
	if err := p.Graph.Add(node); err == nil {
		iter.Node = &node
	}

	switch {
	case record.Success:
		return iter, iter.Node, ""

	case !changed(record):
		// The action was performed and the screen is exactly as it was. Whatever the
		// verification machinery called it, the useful description is that this
		// repeat did nothing — and so would the next one.
		return iter, iter.Node, "no_progress"

	case record.Status == directorapi.ActionUnverified || record.Verification.Inconclusive:
		return iter, iter.Node, "unverified"

	default:
		// Something changed, but not in a way that confirms the action. The same rule
		// the single-action path applies: "I could not confirm it" is not "it did not
		// happen", and repeating could apply it twice.
		return iter, iter.Node, "unverified"
	}
}

// replayIntent rebuilds the intent to re-run.
//
// It is the RECORDED intent, with one correction. A window move recorded as "move
// window left" resolves "the window" to whatever is focused; on replay that may be
// something else entirely, and moving the wrong window is a poor way to honour "do
// that again". So the stored application is pushed into the intent as the subject,
// which is the durable identifier the node kept for exactly this purpose.
func replayIntent(source actiongraph.ActionNode) directorapi.Intent {
	in := source.Intent
	spec, ok := source.Action()
	if !ok {
		return in
	}
	if spec.Window != nil && len(in.Targets) == 0 {
		subject := firstNonEmpty(spec.Window.Application, spec.Window.Title)
		if subject != "" {
			in.Targets = []directorapi.ReferenceExpression{{
				Phrase: subject,
				Kind:   directorapi.ReferenceLiteral,
			}}
		}
	}
	return in
}

// confidenceOf rates how much this replay resembles the original action.
func confidenceOf(source actiongraph.ActionNode, analysis actiongraph.ReplayCandidate,
	world directorapi.WorldState) ReplayConfidence {

	c := ReplayConfidence{Intent: 1, Target: 1, Context: 1}
	note := func(s string) { c.Notes = append(c.Notes, s) }

	// ── Intent ────────────────────────────────────────────────────────────────
	// Whether what is about to happen is the action that was recorded. Replay
	// re-runs the stored intent verbatim, so this is high by construction — and it
	// drops when the stored action could not be fully reconstructed.
	spec, ok := source.Action()
	if !ok {
		c.Intent = 0
		note("the source node records no action")
	} else if _, err := spec.Rebuild(); err != nil {
		c.Intent = 0.3
		note("the stored action is not fully reconstructable: " + err.Error())
	} else if spec.Query == nil && spec.Window == nil {
		c.Intent = 0.5
		note("the stored action has no semantic target")
	}

	// ── Target ────────────────────────────────────────────────────────────────
	// Whether the thing about to be acted on is the thing that was acted on.
	t := source.ResolvedTarget
	if cur := analysis.CurrentTarget; cur != nil {
		if t.Role != "" && string(cur.Role) != t.Role {
			c.Target -= 0.4
			note(fmt.Sprintf("it is now a %s where it was a %s", cur.Role, t.Role))
		}
		if t.Label != "" && !strings.EqualFold(cur.Label, t.Label) {
			c.Target -= 0.3
			note(fmt.Sprintf("the label is now %q where it was %q", cur.Label, t.Label))
		}
		// A target the graph already knew was not durably identifiable is one that
		// re-resolution may well have matched by luck.
		if !t.Identity.Durable {
			c.Target -= 0.2
			note("this control was never uniquely identifiable, so the match may not be the same one")
		}
		if analysis.Status == actiongraph.ReplayTargetMoved {
			c.Target -= 0.1
			note("the target has moved since it was recorded")
		}
	} else {
		c.Target = 0.4
		note("the target could not be re-resolved for comparison")
	}

	// ── Context ───────────────────────────────────────────────────────────────
	// Whether the surroundings are the same. Doing the right thing in the wrong
	// application is not doing the right thing.
	if app := t.App; app != "" {
		current := ""
		if world.ActiveApp != nil {
			current = world.ActiveApp.ID
		}
		if !strings.EqualFold(current, app) {
			c.Context -= 0.5
			note(fmt.Sprintf("the active application is %q where it was %q", current, app))
		}
	}
	if was := source.Outcome.Before.WindowTitle; was != "" {
		now := ""
		if win, ok := world.FocusedWindow(); ok {
			now = win.Title
		}
		if now != was {
			c.Context -= 0.2
			note(fmt.Sprintf("the window is now %q where it was %q", now, was))
		}
	}
	if world.Confidence.Coverage < 0.35 {
		c.Context -= 0.3
		note("this window is much less observable than when the action was recorded")
	}

	c.Intent = clamp01(c.Intent)
	c.Target = clamp01(c.Target)
	c.Context = clamp01(c.Context)
	// The weakest dimension, for the same reason world confidence uses a minimum: a
	// replay is only as trustworthy as its least convincing part.
	c.Overall = min3(c.Intent, c.Target, c.Context)
	return c
}

// cancelled reports whether the user has asked to stop.
//
// Two routes, because they arrive differently. A context cancel is what an
// in-process caller or a Ctrl-C does. StopCheck is what a front-end that spawns a
// process per command has instead: a spoken "stop" becomes a NEW process, which
// cannot cancel a context inside a different one, so it leaves a signal for the
// running loop to find.
func (p *Pipeline) cancelled(ctx context.Context, spec ReplaySpec) (string, bool) {
	select {
	case <-ctx.Done():
		return "cancelled", true
	default:
	}
	if spec.StopCheck != nil && spec.StopCheck() {
		return "stopped", true
	}
	return "", false
}

// stopReasonForConfirmation maps a refused confirmation onto a loop stop reason.
func stopReasonForConfirmation(o ConfirmationOutcome) string {
	switch o {
	case ConfirmationRejected, ConfirmationCancelled:
		return "cancelled"
	}
	return "unsafe"
}

// stopReasonFor maps an analysis verdict onto a loop stop reason.
func stopReasonFor(status actiongraph.ReplayStatus) string {
	switch status {
	case actiongraph.ReplayTargetAmbiguous:
		return "ambiguous"
	case actiongraph.ReplayAppNotRunning:
		return "app_not_running"
	case actiongraph.ReplayUnobservable:
		return "unobservable"
	case actiongraph.ReplayTargetMissing:
		return "target_missing"
	case actiongraph.ReplayUnsafe:
		return "unsafe"
	}
	return "not_replayable"
}

// statusForStop maps a stop reason onto the request's headline status.
func statusForStop(reason string, completed int) directorapi.ResultStatus {
	switch reason {
	case "ambiguous":
		return directorapi.ResultNeedsClarification
	case "unsafe":
		return directorapi.ResultBlocked
	}
	// Anything that stopped part-way through having done SOME iterations is a
	// partial result, not a failure: work was completed and the caller needs to know
	// how much.
	if completed > 0 {
		return directorapi.ResultPartial
	}
	return directorapi.ResultFailed
}

func summarise(reason, detail string, completed, requested int) string {
	prefix := ""
	if completed > 0 {
		prefix = fmt.Sprintf("stopped after %d of %d: ", completed, requested)
	}
	switch reason {
	case "ambiguous":
		return prefix + "several controls now match, so I need to know which one: " + detail
	case "app_not_running":
		return prefix + detail
	case "unobservable":
		return prefix + "I cannot see into that window any more, so I will not guess: " + detail
	case "target_missing":
		return prefix + "the target is no longer there: " + detail
	case "unsafe":
		return prefix + detail
	case "unverified":
		return prefix + "the repeat could not be confirmed, so I stopped rather than repeat it again: " + detail
	case "failed":
		return prefix + "the repeat did not work: " + detail
	}
	return prefix + detail
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func min3(a, b, c float64) float64 {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// Replay settles by CONDITION, not by delay.
//
// There is deliberately no per-iteration pause constant here. Each iteration runs the
// full Observe → Resolve → Plan → Execute → settle → Verify cycle, and its settle is the
// same semantic wait an ordinary action uses (see settleFor) — re-evaluated from scratch
// against the screen as it is now.
//
// That is what "replay preserves waits semantically" means concretely: a replay of an
// action that once took two looks to settle does not wait for two looks, and does not
// wait for however long it took last time. It waits until the region is quiet, which on
// a faster machine is sooner and on a slower one is later.

// handleRepeat resolves "that" to a node in the graph and replays it.
func (p *Pipeline) handleRepeat(ctx context.Context, out Outcome, add func(string, string, bool)) Outcome {
	// "That" means the last action that WORKED. Repeating something that failed, or
	// that could not be confirmed, is not what anybody means by "do that again" —
	// and silently repeating a failure would turn one wrong thing into several.
	source, ok := p.lastSuccessful()
	if !ok {
		out.Status = directorapi.ResultFailed
		out.Message = "there is no completed action to repeat yet"
		add("intent", out.Message, false)
		return out
	}
	add("intent", fmt.Sprintf("repeat %s x%d", source.Describe(), max(1, out.Intent.Count)), true)

	spec := ReplaySpec{
		Node:          source,
		Count:         out.Intent.Count,
		MaxIterations: p.maxReplay(),
		StopCheck:     p.StopCheck,
	}
	replay := p.Replay(ctx, spec)

	out.Replay = &replay
	out.Status = replay.Status
	out.Message = replay.Message
	if n := len(replay.Iterations); n > 0 {
		last := replay.Iterations[n-1]
		out.Record = last.Record
		out.Node = last.Node
	}
	add("replay", fmt.Sprintf("%d of %d completed (%s), confidence %.2f",
		replay.Completed, max(1, out.Intent.Count), replay.StoppedBecause,
		replay.Confidence.Overall), replay.Status == directorapi.ResultDone)
	return out
}

// lastSuccessful finds the most recent verified action in the graph.
func (p *Pipeline) lastSuccessful() (actiongraph.ActionNode, bool) {
	type finder interface {
		LastSuccessful() (actiongraph.ActionNode, bool)
	}
	if f, ok := p.Graph.(finder); ok {
		return f.LastSuccessful()
	}
	// Fall back to walking recent nodes, so any Graph implementation works.
	nodes, err := p.Graph.Recent(50)
	if err != nil {
		return actiongraph.ActionNode{}, false
	}
	for _, n := range nodes {
		if n.Outcome.Success && n.Verification.Success {
			return n, true
		}
	}
	return actiongraph.ActionNode{}, false
}

func (p *Pipeline) maxReplay() int {
	if p.MaxReplayIterations > 0 {
		return p.MaxReplayIterations
	}
	return DefaultMaxIterations
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// progress emits a progress event, if anyone is listening.
//
// A nil hook is the normal case for a direct caller; the service supplies one so a
// client can watch a ten-iteration replay happen rather than waiting for it.
func (p *Pipeline) progress(stage string, iteration, total int, detail string, verified bool) {
	if p.OnProgress == nil {
		return
	}
	p.OnProgress(Progress{
		Stage: stage, Iteration: iteration, Total: total,
		Detail: detail, Verified: verified,
	})
}

// Progress is one step of a long-running command.
type Progress struct {
	Stage     string
	Iteration int
	Total     int
	Detail    string
	Verified  bool
}
