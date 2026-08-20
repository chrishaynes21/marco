package execute

import (
	"context"
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Bounded iteration.
//
//	Iteration advances only after the current member is verified.
//	Director owns collection orchestration. Marco executes one deterministic
//	member action at a time.
//
// The loop below is those two sentences as code, and its shape is dictated by one fact:
// acting on a member CHANGES the set. Clicking the first search result may close the
// list, reorder it, open a window, or recreate every element's identity. So membership
// is re-resolved from a freshly observed world at the top of every iteration, and the
// next member is chosen from what is there NOW rather than from a list captured before
// anything happened.
//
// What this deliberately does not do:
//
//   - it does not precompile the iterations, because iteration N's target does not
//     exist in a form worth compiling until iteration N-1 has been verified;
//   - it does not run members concurrently, because two desktop mutations at once
//     have no defined result and no way to attribute a failure;
//   - it does not continue past a failure, because member N+1 would execute against a
//     world that member N was supposed to have produced.

// isForEach reports whether an intent is a bounded iteration.
func isForEach(in directorapi.Intent) (collections.ForEach, bool) {
	if in.Verb != intent.VerbForEach {
		return collections.ForEach{}, false
	}
	f, ok := in.Parameters[intent.ParamForEach].(collections.ForEach)
	return f, ok
}

// isCaptureCollection reports whether an intent binds a collection.
func isCaptureCollection(in directorapi.Intent) (collections.Collection, bool) {
	if in.Verb != intent.VerbCaptureCollection {
		return collections.Collection{}, false
	}
	c, ok := in.Parameters[intent.ParamCollection].(collections.Collection)
	return c, ok
}

// handleCaptureCollection binds a named collection.
//
// It observes and resolves ONCE, to prove the query describes something observable and
// bounded — a collection nobody could resolve is worth refusing now rather than at the
// first iteration. What it binds is the QUERY; the members it just saw are diagnostic
// evidence about a moment and are deliberately not kept.
//
// Creates no Action Graph node: nothing was touched.
func (p *Pipeline) handleCaptureCollection(ctx context.Context, request string,
	in directorapi.Intent, pctx program.Context, add func(string, string, bool)) (Outcome, bool) {

	c, ok := isCaptureCollection(in)
	if !ok {
		return Outcome{}, false
	}
	out := Outcome{Request: request, Intent: in, Status: directorapi.ResultFailed}

	if err := c.Validate(); err != nil {
		out.Message = err.Error()
		add("collection", out.Message, false)
		return out, true
	}
	env := pctx.Collections
	if env == nil || env.Cleared() {
		out.Message = fmt.Sprintf("cannot capture %q: there is no running program to hold it",
			c.Name)
		add("collection", out.Message, false)
		return out, true
	}
	if env.Has(c.Name) {
		out.Message = fmt.Sprintf("%q is already captured in this program; collections "+
			"are immutable, so use a different name", c.Name)
		add("collection", out.Message, false)
		return out, true
	}

	world, err := p.observeTraced(ctx)
	if err != nil {
		out.Error, out.Message = err.Error(), "could not observe the screen: "+err.Error()
		add("observe", out.Error, false)
		return out, true
	}
	out.World = &world
	add("observe", fmt.Sprintf("%d elements", len(world.Elements)), true)

	p.emitValue(trace.ValueEvent{
		Kind: trace.EventCollectionCaptureStarted, Collection: c.Name,
		CollectionKind: string(c.Kind), QuerySummary: c.Query.Describe(),
	})
	res := collections.Resolve(&world, c.Query, p.rank)
	switch res.Status {
	case collections.StatusResolved:
	case collections.StatusEmpty:
		// A verified empty set is a FACT and binds normally: "nothing is selected" is
		// something a later step can legitimately iterate zero times.
		add("collection", res.Explanation, true)
	case collections.StatusUnobservable:
		out.Message = fmt.Sprintf("could not capture %q: %s", c.Name, res.Explanation)
		add("collection", out.Message, false)
		return out, true
	default:
		out.Message = res.Explanation
		add("collection", out.Message, false)
		return out, true
	}

	c.Provenance = collections.Provenance{
		Application:      c.Query.Element.Application,
		CapturedAt:       p.now(),
		MatchedAtCapture: res.Matched,
		OrderingReason:   res.Ordering.Describe(),
		StepID:           p.stepID,
		StepIndex:        p.stepIndex,
	}
	if err := env.Bind(c); err != nil {
		out.Message = err.Error()
		add("collection", out.Message, false)
		return out, true
	}

	p.emitValue(trace.ValueEvent{
		Kind: trace.EventCollectionBound, Collection: c.Name,
		CollectionKind: string(c.Kind), QuerySummary: c.Query.Describe(),
		MatchedCount: res.Matched, Limit: c.Query.Limit, Outcome: "ok",
	})
	out.Status = directorapi.ResultDone
	out.Message = fmt.Sprintf("Captured collection %q: %s (%d member(s) at capture, %s).",
		c.Name, c.Query.Describe(), res.Matched, res.Ordering.Describe())
	add("collection", out.Message, true)
	return out, true
}

// handleForEach runs one bounded iteration.
func (p *Pipeline) handleForEach(ctx context.Context, request string, in directorapi.Intent,
	pctx program.Context, add func(string, string, bool)) (Outcome, bool) {

	f, ok := isForEach(in)
	if !ok {
		return Outcome{}, false
	}
	out := Outcome{Request: request, Intent: in, Status: directorapi.ResultFailed}

	if err := f.Validate(); err != nil {
		out.Message = err.Error()
		add("collection", out.Message, false)
		return out, true
	}

	// Which collection, and under what name. An inline collection has no name in the
	// program's namespace — it was never captured — but it still needs one for the
	// processed-member ledger, so it borrows the request itself.
	coll := collections.Collection{}
	ledger := "inline:" + request
	switch {
	case f.Inline != nil:
		coll = *f.Inline
	default:
		resolved, err := pctx.Collections.Resolve(f.Collection)
		if err != nil {
			out.Message = err.Error()
			add("collection", out.Message, false)
			return out, true
		}
		coll, ledger = resolved, resolved.Name
	}

	result := p.iterate(ctx, f, coll, ledger, pctx, add, &out)
	out.Collection = &result
	p.emitValue(trace.ValueEvent{
		Kind: trace.EventCollectionCompleted, Collection: coll.Name,
		QuerySummary: coll.Query.Describe(), MatchedCount: result.Matched,
		CompletedCount: result.Completed, Limit: f.Limit,
		Outcome: string(result.State), Reason: result.Reason,
	})

	switch result.State {
	case collections.CollectionCompleted:
		out.Status = directorapi.ResultDone
	case collections.CollectionEmpty:
		// Nothing to do, and that is a real answer rather than a failure. The set was
		// observed and it was empty.
		out.Status = directorapi.ResultDone
	case collections.CollectionAwaitingClarification:
		// A pause. The program stays alive and the answer resumes this member — see
		// resumeProgram, which routes the refinement to the iteration rather than to
		// the step.s targets.
		out.Status = directorapi.ResultNeedsClarification
	case collections.CollectionRefused:
		out.Status = directorapi.ResultBlocked
	default:
		out.Status = directorapi.ResultFailed
	}
	out.Message = result.Summarise()
	return out, true
}

// iterate is the loop.
func (p *Pipeline) iterate(ctx context.Context, f collections.ForEach,
	coll collections.Collection, ledger string, pctx program.Context,
	add func(string, string, bool), out *Outcome) collections.Result {

	result := collections.Result{
		CollectionName: coll.Name, Query: coll.Query.Describe(),
		State: collections.CollectionStopped, Limit: f.Limit,
	}
	env := pctx.Collections

	// approved is the collection-level decision. Nil until bulk policy has run, and
	// checked before the first member acts — see the guard below, which is what makes
	// "authorized before the first member" structural rather than a matter of the
	// statements happening to be in this order.
	var approved *collections.BulkDecision

	// A resumed iteration continues from its REAL position. Starting the count again
	// would report "[1/5]" for the third member and tell the reader the collection had
	// restarted, which is exactly what it must not do.
	start := 1
	resume := pctx.CollectionResume
	if resume.Applies(ledger) && resume.Iteration > 0 {
		start = resume.Iteration
		result.Completed = env.ProcessedCount(ledger)
		p.emitValue(trace.ValueEvent{
			Kind: trace.EventCollectionResumed, Collection: coll.Name,
			Iteration: start, CompletedCount: result.Completed,
			QuerySummary: coll.Query.Describe(),
		})
		add("collection", fmt.Sprintf("resumed at member %d; %d already verified",
			start, result.Completed), true)
	}

	for iteration := start; iteration < start+f.Limit; iteration++ {
		// Cancellation BEFORE observing, so a stop takes effect without first paying
		// for an accessibility walk — and, more importantly, before the next member is
		// touched.
		if err := ctx.Err(); err != nil {
			result.State = collections.CollectionStopped
			result.Reason = fmt.Sprintf("Cancelled after %d of %d members.",
				result.Completed, result.Matched)
			return result
		}

		world, err := p.observeTraced(ctx)
		if err != nil {
			result.Reason = "could not observe the screen: " + err.Error()
			return result
		}

		// Membership is re-resolved EVERY iteration. The previous member's action may
		// have removed it, reordered the rest, or recreated all of their identities.
		res := collections.Resolve(&world, coll.Query, p.rank)
		result.Matched = res.Matched

		switch res.Status {
		case collections.StatusResolved:
		case collections.StatusEmpty:
			if result.Completed == 0 {
				result.State = collections.CollectionEmpty
				result.Reason = res.Explanation
				return result
			}
			// Everything that matched has been processed and the set is now empty —
			// which is what "close every window" looks like when it succeeds.
			result.State = collections.CollectionCompleted
			return result
		case collections.StatusUnobservable:
			// Never treated as empty. Stopping here is the honest answer.
			result.Reason = res.Explanation
			return result
		default:
			result.Reason = res.Explanation
			return result
		}

		// ── Collection-level policy ───────────────────────────────────────────
		//
		// Before the first member, and RE-EVALUATED whenever the matched count moves.
		// The recheck is the "changed membership invalidates confirmation" rule: a
		// decision made about six items is not consent to act on nine, and a set that
		// grew while the user was reading the prompt is a different request.
		if approved == nil || approved.Stale(len(res.Members)) {
			previous := approved
			decision := collections.EvaluateBulk(collections.BulkRequest{
				Operation:      f.Operation.Verb,
				CollectionKind: coll.Kind,
				Query:          coll.Query,
				// The count that matters is what will be ACTED ON, not what matched.
				// "The first three results" touches three, however many results exist,
				// and judging it by the larger number would refuse a request that is
				// precisely bounded — which is the shape users should be encouraged into.
				MatchedCount: len(res.Members),
				MaximumCount: f.Limit,
				Application:  coll.Query.Element.Application,
				MemberRole:   string(coll.Query.Element.Role),
				MemberLabels: memberLabels(res.Members),
			})
			result.Policy = &decision

			p.emitValue(trace.ValueEvent{
				Kind: trace.EventCollectionPolicyCompleted, Collection: coll.Name,
				QuerySummary: coll.Query.Describe(), MatchedCount: len(res.Members),
				Operation: f.Operation.Verb, Outcome: policyOutcome(decision),
				Reason: decision.Reason,
			})

			if previous != nil {
				// A confirmation that has been overtaken by the set changing. Reported
				// as its own outcome rather than silently re-asking, because the user
				// needs to know their answer was about a different set.
				result.State = collections.CollectionRefused
				result.Reason = fmt.Sprintf(
					"The collection changed from %d to %d members. Confirmation is no "+
						"longer valid.", previous.MatchedCount, len(res.Members))
				return result
			}
			if !decision.Allowed {
				result.State = collections.CollectionRefused
				result.Reason = decision.Reason
				add("policy", "refused: "+decision.Reason, false)
				return result
			}
			if decision.RequiresConfirmation {
				// Confirmation is not implemented, so a bulk action needing one STOPS.
				// Proceeding would be exactly what the confirmation exists to prevent,
				// and "every" is not consent.
				result.State = collections.CollectionRefused
				result.Reason = decision.Prompt +
					"\n(confirmation is not supported yet, so nothing was changed)"
				add("policy", "needs confirmation: "+decision.Reason, false)
				return result
			}
			add("policy", "bulk allowed: "+decision.Reason, true)
			approved = &decision
		}

		member, found := nextUnprocessed(res.Members, env, ledger)
		if !found {
			// Every currently matching member has been processed. Bounded completion.
			result.State = collections.CollectionCompleted
			return result
		}
		// A member that cannot be told apart from its siblings would be processed twice
		// or skipped, and there is no safe guess between those.
		if !member.Durable() {
			result.Reason = "Iteration stopped: member identity is not durable enough " +
				"to continue safely."
			result.Iterations = append(result.Iterations, collections.IterationResult{
				Index: iteration, Member: member.Summarise(iteration),
				State: collections.IterationIdentityUncertain, Reason: result.Reason,
			})
			return result
		}

		p.emitValue(trace.ValueEvent{
			Kind: trace.EventIterationStarted, Collection: coll.Name,
			Iteration: iteration, Limit: f.Limit, MatchedCount: len(res.Members),
			CompletedCount: result.Completed, MemberDigest: collections.Digest(member.Key),
		})
		add("collection", fmt.Sprintf("[%d/%d] %s %q",
			iteration, len(res.Members), member.Role, member.Label), true)

		// One member, through the ORDINARY single-step path: observe, resolve, policy,
		// lower, compile, execute, verify. Not a shortcut round it — a per-member path
		// with its own simplified policy or verification is how the two would drift
		// until the bulk one was quietly less safe.
		mq := memberQuery(member)
		// ── Membership drift ──────────────────────────────────────────────────
		//
		//	An ordinal belongs to one offered list in one observed world.
		//	A fresh world may invalidate an old answer.
		//
		// Checked BEFORE the answer is applied. The user was offered a list, thought
		// about it, and answered; if the list moved in between, the position they chose
		// no longer refers to the control they were shown, and applying it would put a
		// choice in their mouth.
		if resume.Applies(ledger) && len(resume.Offered.OrderedKeyDigests) > 0 {
			current := collections.Fingerprint(coll.Query, res.Members)
			drift := collections.CompareMembership(resume.Offered, current,
				resume.Ordinal, res.Status != collections.StatusUnobservable)

			p.emitValue(trace.ValueEvent{
				Kind: trace.EventCollectionMembershipChanged, Collection: coll.Name,
				Iteration: iteration, CompletedCount: result.Completed,
				ChangeKind: string(drift), EventID: resume.EventID,
				OldCount: resume.Offered.MatchedCount, MatchedCount: current.MatchedCount,
				Reason: drift.Describe(),
			})

			if !drift.Resumable() {
				// The answer cannot be honoured. Reported in the user's own terms and
				// with the completed members intact — this is not a restart.
				result.State = collections.CollectionStopped
				result.Reason = drift.Describe()
				add("collection", "the choices changed: "+string(drift), false)
				entry := collections.IterationResult{
					Index: iteration, Member: member.Summarise(iteration),
					State: collections.IterationAmbiguous, Reason: drift.Describe(),
				}
				result.Iterations = append(result.Iterations, entry)
				return result
			}
			add("collection", "the offered choices are unchanged; applying the answer", true)
		}
		// The clarification answer narrows exactly ONE member and is then discarded.
		// It is applied here rather than to the collection.s stored query, because the
		// ordinal referred to the contender list of one event: writing it into the query
		// would make every later member pick the same contender.
		if resume.Applies(ledger) {
			resume.Narrow(mq)
			add("collection", "applying the clarification to this member only", true)
			pctx.CollectionResume, resume = nil, nil
		}

		stepIn := f.Operation
		stepIn.Targets = []directorapi.ReferenceExpression{{
			Phrase: memberPhrase(member),
			Kind:   directorapi.ReferenceLiteral,
			Query:  mq,
		}}
		p.emitValue(trace.ValueEvent{
			Kind: trace.EventIterationResolved, Collection: coll.Name,
			Iteration: iteration, MemberDigest: collections.Digest(member.Key),
			CompletedCount: result.Completed,
		})

		before := len(res.Members)
		// The member.s node records which collection it belonged to. Set around the
		// call and cleared after, so an ordinary action following an iteration cannot
		// inherit a lineage it was never part of.
		p.iteration = &iterationProvenance{
			Name: coll.Name, Kind: string(coll.Kind), Query: coll.Query.Describe(),
			Ordering: string(coll.Query.Ordering), Index: iteration, Limit: f.Limit,
			Digest:    collections.Digest(member.Key),
			ProgramID: env.Program(), StepID: p.stepID,
		}
		stepOut := p.handleParsed(ctx, memberPhrase(member), stepIn, pctx)
		p.iteration = nil
		result.Attempted++

		entry := collections.IterationResult{
			Index: iteration, Member: member.Summarise(iteration),
		}
		if stepOut.Node != nil {
			entry.ActionNode = string(stepOut.Node.ID)
		}

		switch stepOut.Status {
		case directorapi.ResultDone:
			entry.State = collections.IterationVerified
		case directorapi.ResultNeedsClarification:
			entry.State = collections.IterationAmbiguous
			entry.Reason = stepOut.Message
			result.Iterations = append(result.Iterations, entry)
			result.Reason = stepOut.Message
			// A PAUSE, not a stop. The completed members stay completed because their
			// keys are already in the ledger, and the ledger rides on the collection
			// environment the paused program keeps.
			//
			// The position is recorded so the answer resumes THIS member rather than
			// restarting the count.
			out.Resolution = stepOut.Resolution
			result.State = collections.CollectionAwaitingClarification
			result.PausedAt = iteration
			// The offered contenders are fingerprinted so a later answer can be checked
			// against what was actually shown, rather than against whatever is there
			// when it arrives.
			result.Offered = collections.Fingerprint(coll.Query, res.Members)
			result.EventID = string(collections.NewEventID(ledger, iteration, result.Offered))
			p.emitValue(trace.ValueEvent{
				Kind: trace.EventCollectionPaused, Collection: coll.Name,
				Iteration: iteration, CompletedCount: result.Completed,
				MatchedCount: len(res.Members), Limit: f.Limit,
				MemberDigest: collections.Digest(member.Key), Outcome: "awaiting_clarification",
			})
			return result
		case directorapi.ResultBlocked, directorapi.ResultNeedsConfirmation:
			entry.State = collections.IterationUnsafe
			entry.Reason = stepOut.Message
		case directorapi.ResultCancelled:
			entry.State = collections.IterationCancelled
			entry.Reason = fmt.Sprintf("Cancelled after %d of %d members.",
				result.Completed, result.Matched)
		case directorapi.ResultPartial:
			entry.State = collections.IterationUnverified
			entry.Reason = stepOut.Message
		default:
			entry.State = collections.IterationFailed
			entry.Reason = stepOut.Message
		}
		// ── Progress classification and the no-progress guard ─────────────────
		//
		//	Iteration advances only when the current member produced verified
		//	semantic progress.
		//
		// Classified for EVERY outcome, before the entry is recorded, so the result
		// explains what was seen whether the member succeeded or not. The verifier
		// stays authoritative — this only says which KIND of progress its evidence
		// amounts to, which is the question a loop needs and a single action does not.
		//
		// Derived from evidence the verifier already gathered; no extra observation
		// and no coordinates.
		entry.Progress = collections.ClassifyProgress(collections.Evidence{
			Verified:      stepOut.Status == directorapi.ResultDone,
			Inconclusive:  stepOut.Status == directorapi.ResultPartial,
			Kinds:         evidenceKindsOf(stepOut),
			Observable:    true,
			MemberPresent: memberStillPresent(stepOut, member),
		})

		// A member that is still there, unchanged, is the dangerous outcome: advancing
		// past it would leave it unprocessed and eligible again, and the loop would
		// apply the same ineffective operation until the limit. Named explicitly so the
		// reason says what happened rather than only that verification failed.
		if entry.State == collections.IterationVerified && !entry.Progress.Advances() {
			entry.State = collections.IterationNoProgress
		}
		if entry.Progress == collections.ProgressMemberUnchanged {
			entry.Reason = "Iteration stopped: no verified progress for the current member."
		}
		result.Iterations = append(result.Iterations, entry)

		if entry.State.Terminal() {
			p.emitValue(trace.ValueEvent{
				Kind: trace.EventIterationFailed, Collection: coll.Name,
				Iteration: iteration, CompletedCount: result.Completed,
				MemberDigest: collections.Digest(member.Key),
				Outcome:      string(entry.State), Progress: string(entry.Progress),
				Reason: entry.Reason,
			})
			// NOT marked processed, and NOT executed again. Both matter: marking it
			// would claim work that did not happen, and retrying it is the loop this
			// guard exists to prevent.
			if entry.Progress == collections.ProgressMemberUnchanged {
				result.Reason = entry.Reason
			} else {
				result.Reason = fmt.Sprintf("iteration %d was %s: %s",
					iteration, entry.State, entry.Reason)
			}
			return result
		}

		// The member is marked processed only AFTER it verified AND made progress.
		// Marking earlier would let a failed member be skipped on a retry that never
		// happens.
		env.MarkProcessed(ledger, member.Key)
		result.Completed++
		p.emitValue(trace.ValueEvent{
			Kind: trace.EventIterationCompleted, Collection: coll.Name,
			Iteration: iteration, CompletedCount: result.Completed,
			MemberDigest: collections.Digest(member.Key), Outcome: string(entry.State),
		})
		_ = before
	}

	// The limit was reached with members still outstanding. Reported as a stop rather
	// than a completion: acting on a prefix of what was asked for is not doing it.
	result.Reason = fmt.Sprintf(
		"Reached the limit of %d iterations with members still matching.", f.Limit)
	return result
}

// nextUnprocessed picks the first member this collection has not yet acted on.
//
// By SEMANTIC key rather than by position. A list that reflows after its first item is
// deleted gives every remaining member a new index, and a positional cursor would either
// skip one or process one twice.
func nextUnprocessed(members []collections.Member, env *collections.Environment,
	ledger string) (collections.Member, bool) {

	for _, m := range members {
		if !env.WasProcessed(ledger, m.Key) {
			return m, true
		}
	}
	return collections.Member{}, false
}

// memberPhrase names the current member for the trace and for a clarification.
func memberPhrase(m collections.Member) string {
	if m.Label != "" {
		return m.Label
	}
	return string(m.Role)
}

// memberQuery is how the member is re-resolved for its own iteration.
//
// By LABEL and role, not by the element id resolved a moment ago. The id is what the
// membership pass produced; re-resolving through the ordinary path means the single-
// step layer gets to apply its own scrutiny, and means a member that vanished between
// the membership pass and the action fails honestly rather than acting on a stale id.
func memberQuery(m collections.Member) *directorapi.ElementQuery {
	q := &directorapi.ElementQuery{Role: m.Role, Label: m.Label}
	if m.Window != "" {
		w := m.Window
		q.Window = &w
	}
	return q
}

// rank is the pipeline's ranker, handed to collection resolution.
//
// The SAME resolver the single-target path uses. A collection that ranked candidates
// differently could include something a singular request would consider out of scope,
// and the divergence would only show up when a bulk action hit the wrong control.
func (p *Pipeline) rank(w *directorapi.WorldState, q directorapi.ElementQuery) []directorapi.TargetCandidate {
	if p.Resolver == nil {
		return nil
	}
	return p.Resolver.Rank(w, q)
}

// memberLabels collects the labels a bulk decision will inspect.
//
// Used ONLY to detect a destructive-looking target. They are never rendered into a
// confirmation prompt: a label may carry private text, and a prompt shown to the user
// is the last place it should surface.
func memberLabels(members []collections.Member) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		out = append(out, m.Label)
	}
	return out
}

// policyOutcome names a bulk decision for an event, safely.
func policyOutcome(d collections.BulkDecision) string {
	switch {
	case d.RequiresConfirmation:
		return "needs_confirmation"
	case d.Allowed:
		return "permitted"
	}
	return "refused"
}

// evidenceKindsOf reads the verifier's evidence kinds out of a step's outcome.
func evidenceKindsOf(out Outcome) []string {
	if out.Record == nil {
		return nil
	}
	return collections.EvidenceKinds(out.Record.Verification)
}

// memberStillPresent reports whether the member survived its own action.
//
// Read from the AFTER world the step already observed, so no extra accessibility walk
// is paid for. A member that is genuinely gone — the normal outcome of closing a window
// — must not be mistaken for one that failed to change.
func memberStillPresent(out Outcome, m collections.Member) bool {
	if out.Record == nil {
		// No record means nothing ran; treating the member as present is the
		// conservative reading, since the alternative claims a removal never observed.
		return true
	}
	// The verifier records target_gone when it looked and the target had vanished.
	for _, e := range out.Record.Verification.Evidence {
		if e.Kind == "target_gone" && e.Observed {
			return false
		}
	}
	return true
}
