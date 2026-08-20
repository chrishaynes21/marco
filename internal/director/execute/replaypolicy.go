package execute

import (
	"context"
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// What a replay may do without asking again.
//
//	Stored confirmation history is audit metadata only. It is never reusable
//	authorization.
//	A replayed action must not bind to a new object merely because it now has focus.
//
// Those two sentences are the whole policy, and they close the two ways a replay can
// quietly become a different action than the one recorded.
//
// The first: a node records that its original run was confirmed. That fact is history. The
// user agreed, on a particular day, to delete a particular file; they did not agree to
// delete whatever occupies that position now. So the stored outcome is DISCLOSED in the
// prompt and never consulted as permission.
//
// The second: a recorded deictic action carries a binding SNAPSHOT — a path, a native id,
// the evidence that chose it. Replay re-establishes that object from the snapshot and
// refuses when it cannot. What it never does is read the word "this" again, because
// "this" meant something at the moment it was said and means something else now.

// ReplayConsent is what the user explicitly agreed to when they asked for the repeat.
//
//	Explicit replay invocation consent may cover an action only when it is specific
//	enough to satisfy the same coverage rules as goal confirmation.
//
// So it is turned into exactly the same coverage record a goal confirmation produces and
// judged by exactly the same function. A vague "do that again" names no effect, no object
// and no risk, and therefore covers nothing — which is the correct reading of it.
type ReplayConsent struct {
	// Phrase is what the user said.
	Phrase string
	// Effect is the semantic effect they named, empty for a bare "again".
	Effect string
	// Resource is the concrete object they named, empty when they named none.
	Resource string
	// Label is the object's on-screen name, when that is all they named.
	Label string
	// Risk is the level they were told they were agreeing to.
	Risk directorapi.RiskLevel
	// Destructive records that they were told it removes or overwrites something.
	Destructive bool
}

// Specific reports whether this consent says enough to cover anything.
//
// A consent that names no risk cannot be compared against an action's risk, and one that
// names no effect is not agreement to a particular thing happening. Both are required, and
// "repeat that" supplies neither.
func (c *ReplayConsent) Specific() bool {
	return c != nil && c.Risk != "" && c.Effect != ""
}

// withReplayConsent installs explicit consent as request-scoped coverage.
//
// The same context key a goal confirmation uses, so there is one coverage rule rather than
// two that could disagree. Non-specific consent installs nothing, which means every
// question is asked afresh.
func withReplayConsent(ctx context.Context, c *ReplayConsent) context.Context {
	if !c.Specific() {
		return ctx
	}
	return withCoverage(ctx, coverage{
		Risk: c.Risk, Procedure: "the repeat you asked for",
		Resource: c.Resource, Label: c.Label, Destructive: c.Destructive,
	})
}

// replayBinding re-establishes a recorded action's deictic target in this request.
//
// The sequence, and each step is a refusal rather than a fallback:
//
//  1. The action does not point at anything → nothing to do.
//  2. It points at something and the node kept no binding → refuse. An old graph whose
//     deictic action predates bindings has no identity to re-establish, and guessing one
//     from focus is the exact failure this milestone removes.
//  3. The binding has no durable identity — a label and nothing else → refuse. Two files
//     in different folders share a name.
//  4. Re-identify by resource, then native id, against the world observed just now. Gone,
//     moved to another window, changed kind, or no longer the current object → refuse.
//  5. Otherwise file the re-established binding in THIS request's store and point the
//     intent at it.
func (p *Pipeline) replayBinding(ctx context.Context, source actiongraph.ActionNode,
	in directorapi.Intent, world directorapi.WorldState) (directorapi.Intent, error) {

	ref, ok := deicticRef(in)
	if !ok {
		return in, nil
	}
	if !source.Binding.Bound() {
		return in, fmt.Errorf(
			"this recorded action points at %q and the history does not say which object "+
				"that was, so it cannot be repeated safely", ref.Phrase)
	}
	if !source.Binding.Identified() {
		return in, fmt.Errorf(
			"this recorded action points at %s, which the history knows only by its name — "+
				"and a name is shared by files in different folders, so it cannot be "+
				"repeated safely", source.Binding.Describe())
	}

	store := binding.StoreFrom(ctx)
	if store == nil {
		return in, fmt.Errorf("this repeat has no binding store, so %q could not be "+
			"re-established", ref.Phrase)
	}

	restored := source.Binding.Restore()
	// The restored binding carries no stability token, so this ALWAYS re-identifies by
	// resource or native id rather than short-circuiting on an unchanged world. A replay
	// never gets to skip the check.
	out := binding.NewResolver().Revalidate(restored, &world)
	if out.Problem != nil {
		return in, fmt.Errorf("%s: %s", source.Binding.Describe(), out.Problem.Message)
	}
	// Belt and braces on the rule that matters most here: the object re-identified must
	// be the one the history names, by RESOURCE, and a mismatch is refused before any
	// confirmation is put.
	if same, known := binding.SameResource(source.Binding, out.Binding); known && !same {
		return in, fmt.Errorf(
			"the history recorded %s and what is there now is %s, so the repeat was refused "+
				"rather than performed on a different object",
			source.Binding.Resource, out.Binding.Resource)
	}

	id := store.Put(out.Binding)
	// The intent is rewritten to point at THIS request's binding. The recorded id belonged
	// to a request that is over, and leaving it would either resolve to nothing or, worse,
	// collide with a binding this request made for something else.
	rewritten := in
	rewritten.Targets = append([]directorapi.ReferenceExpression{}, in.Targets...)
	for i := range rewritten.Targets {
		if rewritten.Targets[i].RequiresBinding {
			rewritten.Targets[i].BindingID = string(id)
			rewritten.Targets[i].Query = queryForBinding(out.Binding)
		}
	}
	return rewritten, nil
}

// queryForBinding builds the element query that finds a re-established object.
//
// The same shape goal expansion uses, and for the same reason: a semantic description the
// resolver can search for, never a stored handle.
func queryForBinding(b *binding.Binding) *directorapi.ElementQuery {
	q := &directorapi.ElementQuery{Label: b.Label}
	if b.WindowID != "" {
		w := directorapi.WindowID(b.WindowID)
		q.Window = &w
	}
	if b.Application != "" {
		q.Application = b.Application
	}
	if b.Label == "" {
		focused := true
		q.Focused = &focused
	}
	return q
}

// replayConfirmation puts the fresh question a consequential repeat requires.
//
//	Non-destructive replay may proceed without renewed confirmation if current policy
//	says confirmation is unnecessary. Destructive or externally consequential replay
//	requires fresh confirmation.
//
// Two gates, not one. The ordinary policy gate inside prepare still runs and still asks
// whatever it would ask. This one runs FIRST and asks about the thing policy has no way to
// know: that this action is being performed again from history, that the original run's
// confirmation is not being reused, and which object it will land on now.
func (p *Pipeline) replayConfirmation(ctx context.Context, source actiongraph.ActionNode,
	in directorapi.Intent, index, total int) (ConfirmationOutcome, string) {

	action, ok := source.Action()
	if !ok {
		return ConfirmationNotRequired, ""
	}
	rebuilt, err := action.Rebuild()
	if err != nil {
		return ConfirmationNotRequired, ""
	}
	if !destructiveAction(rebuilt) {
		// Everything else is left to the ordinary policy gate, which is the same gate a
		// first-time request goes through. A repeat of a click is a click.
		return ConfirmationNotRequired, ""
	}

	act := actionConfirmation{
		Action:      rebuilt.Describe(),
		Risk:        directorapi.RiskHigh,
		Destructive: true,
		Effect:      "this repeats a recorded action that removes or overwrites something",
		Reason: "the original run's confirmation is a record of what you agreed to then, " +
			"not permission to do it again now",
		Replay: &ReplayContext{
			SourceNode: string(source.ID), Iteration: index + 1, Total: total,
			StoredConfirmation: source.Confirmation,
		},
	}
	if b := bindingFor(ctx, in); b.Bound() {
		act.Label, act.Resource, act.Binding = b.Label, b.Resource, b
		act.Target = b.Resource
	}
	if act.Target == "" {
		act.Target = source.ResolvedTarget.Label
	}

	outcome, _, message := p.confirmAction(ctx, source.Intent.Raw, act)
	return outcome, message
}
