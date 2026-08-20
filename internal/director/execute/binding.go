package execute

import (
	"context"
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/inline"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Bindings in the running pipeline.
//
//	A request such as "rename this file to Budget" cannot execute unless "this file"
//	has resolved to a typed, evidenced binding, and the binding is revalidated
//	immediately before the first externally observable action that depends on it.
//
// Two things happen here, at two different moments, and keeping them apart is the whole
// safety argument.
//
// RESOLUTION happens once, during expansion, against the world as it was when the user
// spoke. It is what turns "this file" into C:\work\Budget.txt with the evidence that says
// why.
//
// REVALIDATION happens per step, immediately before the policy gate and therefore before
// confirmation and before any capability runs. It asks only "is this still the same
// object?" — and when the answer is no it STOPS. It never re-reads "this" against whatever
// is focused now, because between speaking and acting the user clicks things, and the
// object under the cursor a second later is not the object they meant.

// ensureBindings gives a request its own binding store, reusing one already on the
// context.
//
// Reuse is what makes a program work: step 4 must act on the object step 1 bound, and a
// per-step store would give each step a different empty one. Requests do not share a
// context, so reuse within one never becomes reuse across two.
func ensureBindings(ctx context.Context) context.Context {
	if binding.StoreFrom(ctx) != nil {
		return ctx
	}
	return binding.WithStore(ctx, binding.NewStore())
}

// binder resolves a deictic directive against one observed world.
//
// Holds the world it was built with, deliberately. Expansion is a single moment and every
// directive in it must bind against the SAME screen: re-observing per directive would let
// step 1 bind a file and step 3 bind whatever replaced it.
type binder struct {
	world    *directorapi.WorldState
	resolver *binding.Resolver
	store    *binding.Store
	// bound records what was resolved, in order, for diagnostics.
	bound []*binding.Binding
}

var _ goal.Binder = (*binder)(nil)

// Bind resolves one deictic reference and files it in the request's store.
func (b *binder) Bind(req goal.BindRequest) (*binding.Binding, *binding.Problem) {
	res, prob := b.resolver.Resolve(req.Phrase, req.Expected, b.world)
	if prob != nil {
		return nil, prob
	}
	res.Origin = req.Origin
	if req.Application != "" && res.Application == "" {
		res.Application = req.Application
	}
	b.store.Put(res)
	b.bound = append(b.bound, res)
	return res, nil
}

// newBinder observes and returns a binder over what it saw.
func (p *Pipeline) newBinder(ctx context.Context) (*binder, error) {
	world, err := p.observeTraced(ctx)
	if err != nil {
		return nil, err
	}
	store := binding.StoreFrom(ctx)
	if store == nil {
		// A programming error rather than a runtime condition: every entry point calls
		// ensureBindings first, and binding into a store nothing can read back would
		// produce an action whose binding id resolves to nothing.
		return nil, fmt.Errorf("execute: this request has no binding store, so a deictic " +
			"target could not be resolved safely")
	}
	return &binder{world: &world, resolver: binding.NewResolver(), store: store}, nil
}

// ── revalidation ──────────────────────────────────────────────────────────────

// revalidation is what the pre-action re-check concluded, for the trace and the caller.
type revalidation struct {
	// ID is the binding that was checked, empty when the step had none.
	ID binding.ID
	// Binding is the binding to act on — the original, or the refreshed one.
	Binding *binding.Binding
	// Refreshed reports that the same object was re-established after a change.
	Refreshed bool
	Changes   []string
	// Problem is why the action must not proceed.
	Problem *binding.Problem
}

// OK reports whether the action may go ahead.
func (r revalidation) OK() bool { return r.Problem == nil }

// Describe renders the outcome for the trace.
func (r revalidation) Describe() string {
	switch {
	case r.ID == "":
		return "no binding to re-check"
	case r.Problem != nil:
		return r.Problem.Message
	case r.Refreshed:
		return fmt.Sprintf("%s — still the same object (%s)",
			r.Binding.Describe(), strings.Join(r.Changes, "; "))
	}
	return r.Binding.Describe() + " — unchanged"
}

// revalidateBinding re-checks a step's binding against the world it is about to act on.
//
// The point in the sequence is chosen and load-bearing:
//
//	resolve during expansion → validate the program → REVALIDATE → confirm →
//	execute → verify against the same identity
//
// It sits after the plan is built, because only then is the actionable target known and
// the confirmation description can name it; and before the policy gate, because a
// confirmation that described the stale object would be asking the user to agree to
// something other than what would happen.
//
// The world it checks against is the one this step observed a moment ago — no extra
// observation, and none of the focus disturbance one would cause.
func (p *Pipeline) revalidateBinding(ctx context.Context, world directorapi.WorldState,
	in directorapi.Intent) revalidation {

	ref, ok := deicticRef(in)
	if !ok {
		return revalidation{}
	}
	// From the CONTEXT, never from the Pipeline. A pipeline field would be there for
	// whatever ran next, and a binding is trustworthy for exactly one request.
	store := binding.StoreFrom(ctx) //nolint:staticcheck // one lookup, read below
	if store == nil {
		return revalidation{ID: binding.ID(ref.BindingID), Problem: &binding.Problem{
			Reason: binding.ReasonGone,
			Message: fmt.Sprintf("%q was resolved in a request this one cannot see, so it "+
				"could not be re-checked before acting", ref.Phrase),
		}}
	}
	id := binding.ID(ref.BindingID)
	b, found := store.Get(id)
	if !found {
		return revalidation{ID: id, Problem: &binding.Problem{
			Reason: binding.ReasonGone,
			Message: fmt.Sprintf("the binding for %q is no longer available, so the action "+
				"was not performed", ref.Phrase),
		}}
	}

	out := binding.NewResolver().Revalidate(b, &world)
	if out.Problem != nil {
		return revalidation{ID: id, Binding: b, Problem: out.Problem}
	}
	if out.Refreshed {
		// The single writer. Everything downstream — confirmation, execution,
		// verification, diagnostics, the graph — reads this ID and therefore sees the
		// refreshed object rather than the one resolved a second ago.
		store.Replace(id, out.Binding)
	}
	return revalidation{
		ID: id, Binding: out.Binding, Refreshed: out.Refreshed, Changes: out.Changes,
	}
}

// deicticRef returns the intent's bound reference, if it has one.
func deicticRef(in directorapi.Intent) (directorapi.ReferenceExpression, bool) {
	for _, ref := range in.Targets {
		if ref.RequiresBinding {
			return ref, true
		}
	}
	return directorapi.ReferenceExpression{}, false
}

// requireBinding refuses an action that declared it needs a binding and has none.
//
//	A deictic directive must not reach RunProgram with only a generic focused-element
//	target. The old untyped focus fallback is unreachable for deictic procedures.
//
// The last of three guards, and the one that catches everything the other two cannot: an
// intent assembled outside goal expansion, a step rebuilt from an old graph, a replay of a
// node whose binding could not be restored. Program validation runs before step 1 and this
// runs before every step, so there is no path from a deictic reference to an executor
// without a binding.
func requireBinding(in directorapi.Intent) error {
	ref, ok := deicticRef(in)
	if !ok {
		return nil
	}
	if ref.BindingID == "" {
		return fmt.Errorf("%q points at something that was never resolved to a concrete "+
			"object, so it was refused rather than aimed at whatever holds focus", ref.Phrase)
	}
	if ref.ExpectedKind != "" && !binding.ObjectKind(ref.ExpectedKind).Known() {
		return fmt.Errorf("%q asks for %q, which is not a kind of object this Director "+
			"knows how to check", ref.Phrase, ref.ExpectedKind)
	}
	return nil
}

// BindingDiagnostics is the account of what the binding layer did for one action.
//
//	For actual execution diagnostics, record: initial binding, revalidation result,
//	refresh details, action-level confirmation request, confirmation outcome,
//	goal-confirmation coverage decision and reason, capability execution result,
//	verification correlation.
//
// Every field is what HAPPENED rather than what was intended, and the whole is optional:
// a request with no deictic target and no confirmation produces an empty one, which
// renders as nothing rather than as a row of "none".
type BindingDiagnostics struct {
	// Initial is the binding as it stood when the action was planned.
	Initial *binding.Snapshot `json:"initial_binding,omitempty"`
	// Revalidated is the binding that was actually acted on. Equal to Initial when the
	// world had not moved; different when it was re-established.
	Revalidated *binding.Snapshot `json:"revalidated_binding,omitempty"`
	// Refreshed and Changes say whether and how the world moved under the binding.
	Refreshed bool     `json:"refreshed,omitempty"`
	Changes   []string `json:"changes,omitempty"`
	// Problem is why the action did not proceed, when the re-check refused it.
	Problem *binding.Problem `json:"problem,omitempty"`

	// Confirmation is what the action-level gate concluded, empty when it did not run.
	Confirmation ConfirmationOutcome `json:"confirmation,omitempty"`
	// Request is the question that was put, nil when none was.
	Request *ConfirmationRequest `json:"confirmation_request,omitempty"`
	// Coverage is whether a goal-level confirmation answered it, and why.
	Coverage *CoverageDecision `json:"goal_coverage,omitempty"`

	// Capability is the mechanism the ladder chose and actually ran — the "how" the
	// semantic vocabulary deliberately keeps out of the action, recorded here because a
	// fallback that was never a preference is exactly what a reader needs to see.
	Capability string `json:"capability,omitempty"`
	// Verification is the per-action verdict in one phrase.
	Verification string `json:"verification,omitempty"`
	// Correlation is whether the result belongs to the bound object, in one line.
	//
	// Read back out of the record's own evidence rather than threaded down from the
	// verification call, so there is one place the verdict lives and the diagnostics
	// cannot drift from what was recorded.
	Correlation string `json:"correlation,omitempty"`
	// Editor is what the inline-editor DERIVATION concluded, when the step acted on one.
	// Absent for every step that did not.
	Editor *inline.Verification `json:"editor,omitempty"`
	// EditorOutcome is what the editor was found to hold, or to have closed, AFTER the
	// step ran. Kept apart from Editor because the two answer different questions and
	// fail in different ways: a step can derive the right editor and still not have its
	// text land in it, which is the failure the whole model exists to catch.
	EditorOutcome *inline.Verification `json:"editor_outcome,omitempty"`
	// Node is the action-graph node this became, empty when nothing was recorded.
	Node string `json:"action_node,omitempty"`
	// Result is the request's headline outcome, so the whole account reads as one
	// thing rather than as a set of fragments a reader has to assemble.
	Result string `json:"result,omitempty"`
}

// Empty reports whether there is nothing to show.
func (d *BindingDiagnostics) Empty() bool {
	return d == nil || (d.Initial == nil && d.Confirmation == "" && d.Problem == nil &&
		d.Capability == "" && d.Verification == "" && d.Correlation == "" &&
		d.Editor == nil)
}

// Describe renders the diagnostics as stable, readable lines.
func (d *BindingDiagnostics) Describe() string {
	if d.Empty() {
		return ""
	}
	var b strings.Builder
	if d.Initial != nil {
		fmt.Fprintf(&b, "  bound        %s\n", d.Initial.Describe())
		for _, e := range d.Initial.Evidence {
			mark := " "
			if e.Decisive {
				mark = "*"
			}
			fmt.Fprintf(&b, "   %s %-14s %s\n", mark, e.Kind, e.Detail)
		}
	}
	switch {
	case d.Problem != nil:
		fmt.Fprintf(&b, "  re-checked   REFUSED (%s): %s\n", d.Problem.Reason, d.Problem.Message)
	case d.Refreshed:
		fmt.Fprintf(&b, "  re-checked   re-established: %s\n", strings.Join(d.Changes, "; "))
		if d.Revalidated != nil {
			fmt.Fprintf(&b, "  acted on     %s\n", d.Revalidated.Describe())
		}
	case d.Initial != nil:
		b.WriteString("  re-checked   unchanged\n")
	}
	if d.Confirmation != "" {
		fmt.Fprintf(&b, "  confirmation %s\n", d.Confirmation)
	}
	if d.Coverage != nil {
		covered := "not covered"
		if d.Coverage.Covered {
			covered = "covered"
		}
		fmt.Fprintf(&b, "  goal cover   %s — %s\n", covered, d.Coverage.Reason)
	}
	if d.Request != nil {
		fmt.Fprintf(&b, "  asked        %s\n", d.Request.Describe())
	}
	// The editor derivation, before the capability that ran in it: this is what the step
	// was aimed at, and a reader tracing a rename that went into the wrong box needs to
	// see the correlation and its evidence before anything else about the action.
	if d.Editor != nil {
		fmt.Fprintf(&b, "  editor       %s\n", d.Editor.Describe())
		for _, e := range d.Editor.Evidence {
			fmt.Fprintf(&b, "     %s\n", e)
		}
		for _, m := range d.Editor.Missing {
			fmt.Fprintf(&b, "   ! %s\n", m)
		}
		for _, c := range d.Editor.Candidates {
			fmt.Fprintf(&b, "   ? %s\n", c)
		}
	}
	if d.EditorOutcome != nil {
		fmt.Fprintf(&b, "  editor after %s\n", d.EditorOutcome.Describe())
	}
	if d.Capability != "" {
		fmt.Fprintf(&b, "  capability   %s\n", d.Capability)
	}
	if d.Verification != "" {
		fmt.Fprintf(&b, "  verified     %s\n", d.Verification)
	}
	if d.Correlation != "" {
		fmt.Fprintf(&b, "  correlated   %s\n", d.Correlation)
	}
	if d.Node != "" {
		fmt.Fprintf(&b, "  node         %s\n", d.Node)
	}
	if d.Result != "" {
		fmt.Fprintf(&b, "  result       %s\n", d.Result)
	}
	return b.String()
}

// finish completes the account once the action has run and been recorded.
func (d *BindingDiagnostics) finish(record directorapi.ActionRecord,
	node *actiongraph.ActionNode, result directorapi.ResultStatus) {

	if d == nil {
		return
	}
	d.Capability = record.Execution.Detail
	d.Verification = firstNonEmpty(record.Verification.Reason, record.FailureReason)
	for _, e := range record.Verification.Evidence {
		if e.Kind == "binding_correlation" {
			d.Correlation = e.Detail
		}
	}
	if node != nil {
		d.Node = string(node.ID)
	}
	d.Result = string(result)
}

// record fills the diagnostics from a revalidation.
func (d *BindingDiagnostics) record(r revalidation, initial *binding.Binding) {
	if d == nil || r.ID == "" {
		return
	}
	d.Initial = initial.Snapshot()
	d.Refreshed, d.Changes, d.Problem = r.Refreshed, r.Changes, r.Problem
	d.Revalidated = r.Binding.Snapshot()
}

// noteBinding stamps a node with the object its action was aimed at.
//
//	Old graphs without binding metadata still decode. Re-encoding old graphs does not
//	invent a binding.
//
// A nil or unbound binding leaves the field ABSENT rather than writing a zero value, so a
// non-deictic node written today and one written before this existed are byte-identical —
// which is what makes "this node had no binding" and "this node predates bindings"
// indistinguishable, correctly, because they mean the same thing.
func noteBinding(node *actiongraph.ActionNode, b *binding.Binding) {
	if node == nil || !b.Bound() {
		return
	}
	node.Binding = b.Snapshot()
}

// noteEditor stamps a node with the inline editor its action was carried out in.
//
//	Do not serialize ephemeral native handles as durable graph identity.
//
// The Snapshot is what makes that safe — it drops the element id and the native id and
// keeps what was edited, what the box held before and after, and why it was tied to the
// object. A node written for a step that acted on no editor is byte-identical to one
// written before this existed.
func noteEditor(node *actiongraph.ActionNode, d *BindingDiagnostics) {
	if node == nil || d == nil || d.Editor == nil {
		return
	}
	snap := d.Editor.Editor.Snapshot()
	if snap == nil {
		return
	}
	if d.EditorOutcome != nil {
		// What the box held when it was last looked at. Absent for a commit, where the
		// editor is gone by then and its final contents are not knowable from here — the
		// filesystem is what settles that, and it says so elsewhere.
		snap.FinalValue = d.EditorOutcome.Value
	}
	node.Editor = snap
}

// bindingFor returns the live binding an intent refers to, nil when it has none.
func bindingFor(ctx context.Context, in directorapi.Intent) *binding.Binding {
	ref, ok := deicticRef(in)
	if !ok {
		return nil
	}
	store := binding.StoreFrom(ctx)
	if store == nil {
		return nil
	}
	b, _ := store.Get(binding.ID(ref.BindingID))
	return b
}
