package execute

import (
	"context"
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Confirmation, at both levels.
//
//	Confirmation happens after goal selection and procedure expansion, and before any
//	executable step runs.
//	High-risk actions can request confirmation regardless of whether they came from
//	goal expansion, clause splitting, direct semantic actions, or replay.
//
// One gate expressed twice, at two scopes. The GOAL gate asks about a whole procedure
// before its first step, because a person cannot meaningfully agree to "delete this"
// without being told it means select, then invoke Delete, then answer a prompt. The
// ACTION gate asks about one concrete action, after its target is known and after the
// binding that names that target has been re-checked, because that is the first moment
// the question can be asked truthfully.
//
// Both go through one Confirmer and one closed outcome vocabulary. Two interfaces would
// have meant two ways to say no, and a front-end that implemented one of them.

// ConfirmationScope says what a question is about.
type ConfirmationScope string

const (
	// ScopeGoal: a whole expanded procedure, asked before its first step.
	ScopeGoal ConfirmationScope = "goal"
	// ScopeAction: one concrete action, asked after its target is bound and re-checked.
	ScopeAction ConfirmationScope = "action"
	// ScopeReplay: a recorded action about to be performed again.
	ScopeReplay ConfirmationScope = "replay"
)

// ConfirmationOutcome is what happened at a gate.
//
// A CLOSED vocabulary, because a caller and a log need to tell these apart and "nothing
// ran" is true of four of them for four different reasons: the user said no, there was no
// way to ask, the request was abandoned, or nothing needed asking at all.
type ConfirmationOutcome string

const (
	// ConfirmationNotRequired: nothing asked for one.
	ConfirmationNotRequired ConfirmationOutcome = "not_required"
	// ConfirmationAccepted: the user agreed and execution proceeded.
	ConfirmationAccepted ConfirmationOutcome = "accepted"
	// ConfirmationRejected: the user declined. A normal outcome, not a failure.
	ConfirmationRejected ConfirmationOutcome = "rejected"
	// ConfirmationUnavailable: one was required and there is no way to ask. Refused,
	// because proceeding would be doing the irreversible thing the confirmation exists
	// to gate, and refusing is the recoverable direction.
	ConfirmationUnavailable ConfirmationOutcome = "unavailable"
	// ConfirmationCancelled: the request was abandoned while the question was open.
	// Distinct from a rejection: the user did not decide, and reporting "you declined"
	// when they merely walked away is a lie about what they wanted.
	ConfirmationCancelled ConfirmationOutcome = "cancelled"
)

// Confirmed reports whether execution may proceed.
func (o ConfirmationOutcome) Confirmed() bool {
	return o == ConfirmationNotRequired || o == ConfirmationAccepted
}

// ConfirmationRequest is what the user is asked to agree to.
//
// One type for every scope, because the fields a person needs are the same: what will
// happen, to WHICH object, how bad it would be to get wrong, and why they are being asked.
// A goal-scoped request fills the Steps; an action-scoped one fills Effect and Resource.
type ConfirmationRequest struct {
	Scope ConfirmationScope `json:"scope"`

	// Goal and Procedure are set when this came from a goal expansion. They are also
	// PROVENANCE for an action-scoped question, so a prompt in the middle of a
	// procedure can say which procedure it belongs to.
	Goal      goal.Kind `json:"goal,omitempty"`
	Procedure string    `json:"procedure,omitempty"`
	StepID    string    `json:"step_id,omitempty"`
	StepIndex int       `json:"step_index,omitempty"`
	StepCount int       `json:"step_count,omitempty"`

	// Request is the user's own words.
	Request string `json:"request"`
	// Action is the semantic action about to be performed, for an action-scoped
	// question ("delete", "click", "edit").
	Action string `json:"action,omitempty"`
	// Target is the human-readable thing acted on.
	Target string `json:"target,omitempty"`
	// Resource is the bound backing identity — a file path — when there is one.
	//
	//	Do not construct destructive confirmation text from a label alone when a
	//	backing resource is available.
	//
	// A prompt saying "delete Budget.txt" is ambiguous between four folders. One saying
	// "delete C:\work\Budget.txt" is not, and the binding already knows which.
	Resource string `json:"resource,omitempty"`
	// Effect is what the action is expected to do, in one phrase.
	Effect string `json:"effect,omitempty"`

	Risk   directorapi.RiskLevel `json:"risk"`
	Safety goal.Safety           `json:"safety,omitzero"`
	// Steps are the phrases of the expanded program, for a goal-scoped question.
	Steps []string `json:"steps,omitempty"`
	// Reason is why confirmation is required.
	Reason string `json:"reason,omitempty"`

	// CoverableByGoal reports whether an accepted goal-level confirmation could have
	// answered this question. Informational for a front-end that wants to say "you
	// already agreed to the procedure, but this step is riskier than it declared".
	CoverableByGoal bool `json:"coverable_by_goal,omitempty"`
	// TargetChanged reports that the bound object was re-established under a changed
	// world since the last confirmation, and how.
	TargetChanged bool     `json:"target_changed,omitempty"`
	Changes       []string `json:"changes,omitempty"`

	// Replay is set when this action is being performed again from history.
	Replay *ReplayContext `json:"replay,omitempty"`
}

// ReplayContext is what a replayed action's confirmation must disclose.
type ReplayContext struct {
	// SourceNode is the recorded action being repeated.
	SourceNode string `json:"source_node"`
	// Iteration is which repeat this is, 1-based.
	Iteration int `json:"iteration,omitempty"`
	// Total is how many were asked for.
	Total int `json:"total,omitempty"`
	// StoredConfirmation is what the ORIGINAL action's confirmation concluded.
	//
	//	Stored confirmation history is audit metadata only. It is never reusable
	//	authorization.
	//
	// Disclosed so a person can see it, and deliberately not consulted by anything: a
	// yes given last Tuesday to a file that has since been replaced is not a yes to
	// this.
	StoredConfirmation string `json:"stored_confirmation,omitempty"`
}

// Describe renders the request as the sentence to put to a person.
func (r ConfirmationRequest) Describe() string {
	subject := r.Target
	if r.Resource != "" {
		// The resource wins. See the Resource field for why a label alone is not enough
		// to describe something destructive.
		subject = r.Resource
	}
	var b strings.Builder
	switch {
	case r.Scope == ScopeGoal:
		fmt.Fprintf(&b, "%s", r.Goal.Describe())
		if subject != "" {
			fmt.Fprintf(&b, " %s", subject)
		}
	case r.Action != "" && subject != "":
		fmt.Fprintf(&b, "%s %s", r.Action, subject)
	case r.Action != "":
		b.WriteString(r.Action)
	default:
		b.WriteString(r.Request)
	}
	if r.Effect != "" {
		fmt.Fprintf(&b, " — %s", r.Effect)
	}
	if r.Replay != nil {
		b.WriteString(" (repeating a recorded action)")
	}
	return b.String()
}

// Confirmer asks the user to agree to something before it happens.
//
// Optional, and its absence is a REFUSAL rather than a default-yes: see
// ConfirmationUnavailable. A Director wired without one can still run everything that
// needs no confirmation, which is most things.
type Confirmer interface {
	// Confirm returns true when the user agreed. An error means the question could not
	// be put — treated as unavailable, never as agreement.
	Confirm(ctx context.Context, req ConfirmationRequest) (bool, error)
}

// ask puts one question and maps everything that can happen onto the vocabulary.
//
// The single place a Confirmer is called, so the rules hold everywhere:
//
//	nil confirmer → unavailable, confirmer error → unavailable,
//	cancelled context → cancelled, refusal → rejected.
func (p *Pipeline) ask(ctx context.Context, req ConfirmationRequest) (ConfirmationOutcome, string) {
	if err := ctx.Err(); err != nil {
		return ConfirmationCancelled, "the request was abandoned before it could be confirmed, " +
			"so nothing was done"
	}
	if p.Confirmer == nil {
		return ConfirmationUnavailable, req.Describe() +
			" needs to be confirmed before it runs, and this Director has no way to ask. " +
			"Nothing was done."
	}
	ok, err := p.Confirmer.Confirm(ctx, req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ConfirmationCancelled, "the request was abandoned while the confirmation " +
				"was open, so nothing was done"
		}
		return ConfirmationUnavailable, "the confirmation could not be put to you (" +
			err.Error() + "), so nothing was done"
	}
	if !ok {
		return ConfirmationRejected, "cancelled — nothing was done"
	}
	return ConfirmationAccepted, ""
}

// confirmGoal puts the goal-level question, when the procedure asks for one.
func (p *Pipeline) confirmGoal(ctx context.Context, request string, ex goal.Expansion) (
	ConfirmationOutcome, string) {

	if !ex.Safety.RequiresConfirmation {
		return ConfirmationNotRequired, ""
	}
	steps := make([]string, 0, len(ex.Program.Steps))
	for _, s := range ex.Program.Steps {
		steps = append(steps, s.Phrase)
	}
	req := ConfirmationRequest{
		Scope: ScopeGoal, Goal: ex.Goal.Kind, Procedure: ex.Procedure, Request: request,
		Risk: ex.Safety.Risk, Safety: ex.Safety, Steps: steps, Reason: ex.Why,
		Target: ex.Goal.Context.Target, Effect: goalEffect(ex),
	}
	// The concrete object, when the goal pointed at one. "Delete this" is not a question
	// anybody can answer; "delete C:\work\Budget.txt" is.
	if b := firstBinding(ex); b != nil {
		req.Resource = b.Resource
		if req.Target == "" || b.Label != "" {
			req.Target = b.Label
		}
	}
	return p.ask(ctx, req)
}

// goalEffect is the one-phrase account of what a procedure will do.
func goalEffect(ex goal.Expansion) string {
	var flags []string
	if ex.Safety.Destructive {
		flags = append(flags, "removes or overwrites something")
	}
	if ex.Safety.Irreversible {
		flags = append(flags, "cannot be undone")
	}
	if ex.Safety.External {
		flags = append(flags, "is visible outside this machine")
	}
	if len(flags) == 0 {
		return fmt.Sprintf("%d step(s)", len(ex.Program.Steps))
	}
	return strings.Join(flags, ", ")
}

// firstBinding returns the expansion's first resolved deictic object.
func firstBinding(ex goal.Expansion) *binding.Binding {
	for _, b := range ex.Bindings {
		if b.Bound() {
			return b
		}
	}
	return nil
}

// ── coverage ──────────────────────────────────────────────────────────────────

// coverage is what an accepted goal-level confirmation agreed to.
//
//	Store coverage state in the request execution context, not on Pipeline, a
//	registry, or a reusable executor.
//
// On the CONTEXT because it belongs to a single run. A field would be shared by whatever
// ran next, and an accepted confirmation leaking into the following command is precisely
// the bug the whole gate exists to prevent.
type coverage struct {
	// Risk is the level the user agreed to.
	Risk directorapi.RiskLevel
	// Goal and Procedure are what they agreed to.
	Goal      goal.Kind
	Procedure string
	// Resource and Label are the concrete object the question named, when it named one.
	// A confirmation for file A never covers an action on file B.
	Resource string
	Label    string
	// Destructive records whether what was agreed to was itself destructive. A yes to
	// a non-destructive procedure does not become a yes to discarding work, however
	// the risk levels compare.
	Destructive bool
}

type coverageKey struct{}

// withCoverage marks a context as covered by an accepted goal-level confirmation.
func withCoverage(ctx context.Context, c coverage) context.Context {
	return context.WithValue(ctx, coverageKey{}, c)
}

// coverageFrom reports what the user has already agreed to in this request.
func coverageFrom(ctx context.Context) (coverage, bool) {
	if ctx == nil {
		return coverage{}, false
	}
	c, ok := ctx.Value(coverageKey{}).(coverage)
	return c, ok
}

// CoverageDecision is why an action-level question was or was not suppressed.
//
// Returned rather than a bare bool because "we did not ask" needs an account: a reader of
// the trace has to be able to tell "the user already agreed to exactly this" from "no
// goal confirmation was given", and a front-end has to be able to say which.
type CoverageDecision struct {
	Covered bool   `json:"covered"`
	Reason  string `json:"reason"`
}

// coveredByGoalConfirmation decides whether the goal-level yes answers this question.
//
//	A goal confirmation covers an action only when it was accepted in the current
//	request, described the same semantic effect, described the same concrete target
//	identity, its risk is at least the action's, no binding refresh materially changed
//	the target, and lowering did not introduce a stricter action.
//
// Every clause below is one of those, and each has a case behind it. "Continue with
// rename?" must not cover a deletion; a yes for file A must not cover file B; a
// medium-risk yes must not cover a high-risk step; and a yes given while the binding
// pointed at one file must not survive that binding being re-established somewhere else.
func coveredByGoalConfirmation(ctx context.Context, act actionConfirmation) CoverageDecision {
	c, ok := coverageFrom(ctx)
	if !ok {
		return CoverageDecision{Reason: "no confirmation has been given in this request"}
	}
	if !c.Risk.AtLeast(act.Risk) {
		return CoverageDecision{Reason: fmt.Sprintf(
			"the confirmation given was for %s risk and this step is %s", c.Risk, act.Risk)}
	}
	if act.Destructive && !c.Destructive {
		return CoverageDecision{Reason: "what you agreed to was not described as destructive, " +
			"and this step removes or overwrites something"}
	}
	// The concrete object. Compared only when BOTH sides name one — an unbound action
	// inside a confirmed procedure is covered by the procedure, and a bound one is
	// covered only if it is the same object.
	if c.Resource != "" && act.Resource != "" && !strings.EqualFold(c.Resource, act.Resource) {
		return CoverageDecision{Reason: fmt.Sprintf(
			"you confirmed %s and this step acts on %s", c.Resource, act.Resource)}
	}
	if c.Resource == "" && act.Resource != "" && c.Label != "" && act.Label != "" &&
		!strings.EqualFold(c.Label, act.Label) {
		return CoverageDecision{Reason: fmt.Sprintf(
			"you confirmed %q and this step acts on %q", c.Label, act.Label)}
	}
	// A binding that was re-established under a changed world may still be the same
	// object — but the description the user agreed to was written before that, so a
	// change that alters the description invalidates the agreement.
	if act.MaterialChange != "" {
		return CoverageDecision{Reason: "the target changed materially after you confirmed: " +
			act.MaterialChange}
	}
	return CoverageDecision{Covered: true, Reason: fmt.Sprintf(
		"covered by the confirmation already given for %s", c.Procedure)}
}

// actionConfirmation is one action's confirmation question, before it is asked.
//
// Assembled after lowering and after revalidation, which is what makes it truthful: the
// action is as specific as it is going to get, and the target is the one that will be
// acted on rather than the one that was resolved a second ago.
type actionConfirmation struct {
	Action      string
	Target      string
	Label       string
	Resource    string
	Effect      string
	Risk        directorapi.RiskLevel
	Destructive bool
	Reason      string
	// MaterialChange is non-empty when a binding refresh altered how the target would be
	// described. See coveredByGoalConfirmation.
	MaterialChange string
	Binding        *binding.Binding
	Replay         *ReplayContext
}

// confirmAction puts the action-level question, unless the goal-level one already
// answered it.
//
//	The action-level gate must run after binding revalidation, target resolution,
//	safety classification, and any lowering step that can make the action materially
//	more specific or risky. It must run before capability execution.
func (p *Pipeline) confirmAction(ctx context.Context, request string, act actionConfirmation) (
	ConfirmationOutcome, CoverageDecision, string) {

	decision := coveredByGoalConfirmation(ctx, act)
	if decision.Covered {
		return ConfirmationNotRequired, decision, decision.Reason
	}
	outcome, message := p.ask(ctx, p.confirmationRequestFor(ctx, request, act))
	return outcome, decision, message
}

// confirmationRequestFor assembles the question one action would be asked.
//
// Pure, so the DIAGNOSTICS can show exactly what was put to the user rather than a
// reconstruction of it — a trace that described a slightly different question than the one
// asked would be worse than no trace at all.
func (p *Pipeline) confirmationRequestFor(ctx context.Context, request string,
	act actionConfirmation) ConfirmationRequest {

	req := ConfirmationRequest{
		Scope: ScopeAction, Request: request,
		Action: act.Action, Target: act.Target, Resource: act.Resource,
		Effect: act.Effect, Risk: act.Risk, Reason: act.Reason,
		CoverableByGoal: true, Replay: act.Replay,
		StepID: p.stepID, StepIndex: p.stepIndex, StepCount: p.stepCount,
	}
	if act.Replay != nil {
		req.Scope = ScopeReplay
	}
	if prov := p.goalProvenance; prov != nil {
		req.Goal, req.Procedure = goal.Kind(prov.Goal), prov.Procedure
	} else if c, ok := coverageFrom(ctx); ok {
		req.Goal, req.Procedure = c.Goal, c.Procedure
	}
	if act.MaterialChange != "" {
		req.TargetChanged = true
		req.Changes = []string{act.MaterialChange}
	}
	return req
}

// materialChange reports how a refresh altered the way a target would be described.
//
// A rebuilt element id is NOT material: the object is the same file in the same window and
// a person reading "rename Budget.txt" would see no difference. A changed window or a
// changed resource IS material, because the sentence the user agreed to would now name
// something else.
func materialChange(r revalidation) string {
	if !r.Refreshed {
		return ""
	}
	for _, c := range r.Changes {
		if strings.Contains(c, "window") {
			return c
		}
	}
	return ""
}
