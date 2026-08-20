package execute

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/variables"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Variable management.
//
//	First make semantic memory real and reachable.
//
// These operations manage the Director's KNOWLEDGE, not the desktop. Nothing here
// lowers to Marco, and nothing here produces an action-graph desktop node — a capture
// that fabricated one would claim the computer was touched when it was not, and every
// later count of "what did the Director do" would be wrong.
//
// `remember` is the exception that proves the rule: it must observe and resolve a real
// target first, because a variable is only worth storing if it named something. It
// uses the ordinary observe → resolve → clarify path to do that, then stores the
// QUERY and discards the answer.

// VariableWriter is the management half of the store.
//
// Separate from VariableStore (the read half the resolver needs) so a component that
// only looks things up cannot delete them.
type VariableWriter interface {
	VariableStore
	All() []variables.Variable
	Put(v variables.Variable) error
	Forget(name string) error
	Rename(from, to string) error
}

// isVariableCommand reports whether an intent manages variables rather than the
// desktop.
func isVariableCommand(verb string) bool {
	switch verb {
	case intent.VerbRemember, intent.VerbForget, intent.VerbRenameVariable,
		intent.VerbListVariables, intent.VerbExplainVar:
		return true
	}
	return false
}

// handleVariableCommand runs a management operation.
//
// Returns the outcome and true when it handled the intent. Management commands other
// than `remember` need no world at all, which is why they are answered before the
// observe stage: asking a user to wait for an accessibility walk to be told the name
// of a variable would be absurd.
func (p *Pipeline) handleVariableCommand(ctx context.Context, request string,
	in directorapi.Intent, add func(string, string, bool)) (Outcome, bool) {

	if !isVariableCommand(in.Verb) {
		return Outcome{}, false
	}
	out := Outcome{Request: request, Intent: in, Status: directorapi.ResultFailed}

	store := p.VariableWriter()
	if store == nil {
		out.Message = "this Director was built without variable storage"
		add("variable", out.Message, false)
		return out, true
	}
	name, _ := in.Parameters[intent.VariableName].(string)

	switch in.Verb {
	case intent.VerbListVariables:
		out.Status, out.Message = directorapi.ResultDone, renderVariableList(store.All())
		add("variable", fmt.Sprintf("%d remembered", len(store.All())), true)
		return out, true

	case intent.VerbExplainVar:
		v, ok := store.Get(name)
		if !ok {
			out.Message = (&variables.ErrUnknown{Name: name}).Error()
			add("variable", out.Message, false)
			return out, true
		}
		out.Status, out.Message = directorapi.ResultDone, ExplainVariable(v)
		return out, true

	case intent.VerbForget:
		if err := store.Forget(name); err != nil {
			out.Message = err.Error()
			add("variable", out.Message, false)
			return out, true
		}
		out.Status = directorapi.ResultDone
		out.Message = fmt.Sprintf("Forgot variable %q.", name)
		add("variable", out.Message, true)
		return out, true

	case intent.VerbRenameVariable:
		to, _ := in.Parameters[intent.VariableTo].(string)
		if err := store.Rename(name, to); err != nil {
			out.Message = err.Error()
			add("variable", out.Message, false)
			return out, true
		}
		out.Status = directorapi.ResultDone
		out.Message = fmt.Sprintf("Renamed variable %q to %q.", name, to)
		add("variable", out.Message, true)
		return out, true

	case intent.VerbRemember:
		return p.rememberTarget(ctx, request, in, name, store, add), true
	}
	return out, true
}

// rememberTarget observes, resolves and stores the QUERY.
//
// The resolution is the input to capture, never the output of it. Nothing is persisted
// unless a single real target was found: an ambiguous or absent capture that stored
// something would create a variable guaranteed to misbehave later, at a moment far
// from the mistake.
func (p *Pipeline) rememberTarget(ctx context.Context, request string, in directorapi.Intent,
	name string, store VariableWriter, add func(string, string, bool)) Outcome {

	out := Outcome{Request: request, Intent: in, Status: directorapi.ResultFailed}

	// Refuse a duplicate BEFORE doing any work. A variable is knowledge the user
	// built, and replacing it silently loses something retyping cannot recover.
	if existing, taken := store.Get(name); taken {
		out.Message = fmt.Sprintf("Variable %q already exists (%s). "+
			"Say \"forget %s\" first if you want to replace it.",
			name, existing.Describe(), name)
		add("variable", out.Message, false)
		return out
	}

	world, err := p.observeTraced(ctx)
	if err != nil {
		out.Error, out.Message = err.Error(), "could not observe the screen: "+err.Error()
		add("observe", out.Error, false)
		return out
	}
	out.World = &world
	add("observe", fmt.Sprintf("%d elements", len(world.Elements)), true)

	query := directorapi.ElementQuery{}
	if len(in.Targets) > 0 && in.Targets[0].Query != nil {
		query = *in.Targets[0].Query
	}
	res := p.Resolver.Resolve(&world, query)
	out.Resolution = &res
	add("resolve", string(res.Status)+": "+res.Explanation, res.Status == directorapi.ResolutionResolved)

	switch res.Status {
	case directorapi.ResolutionResolved:
	case directorapi.ResolutionAmbiguous:
		// The ordinary clarification path. The answer narrows THIS capture and must
		// not become part of the stored query — see Capture, which strips the ordinal.
		out.Status = directorapi.ResultNeedsClarification
		out.Message = res.Explanation
		return out
	case directorapi.ResolutionUnobservable:
		out.Message = fmt.Sprintf(
			"Could not remember %q: the Director cannot currently observe this interface. "+
				"That is not evidence the target is absent.", name)
		return out
	default:
		out.Message = fmt.Sprintf("Could not remember %q: the requested target is not present.", name)
		return out
	}

	v, err := variables.Capture(name, variables.KindTarget, res, &world, request)
	if err != nil {
		out.Message = err.Error()
		add("variable", out.Message, false)
		return out
	}
	if err := store.Put(v); err != nil {
		out.Message = err.Error()
		add("variable", out.Message, false)
		return out
	}

	out.Status = directorapi.ResultDone
	out.Message = fmt.Sprintf("Remembered variable %q as %s %q in %s.",
		name, firstNonEmpty(string(v.Role), "control"),
		firstNonEmpty(v.Label, "(unlabelled)"), firstNonEmpty(v.Application, "this application"))
	add("variable", out.Message, true)
	return out
}

// VariableWriter returns the store's management half when one is wired.
func (p *Pipeline) VariableWriter() VariableWriter {
	w, _ := p.Variables.(VariableWriter)
	return w
}

// renderVariableList draws what the Director remembers.
//
// Semantic facts only. A handle or a coordinate here would be the very thing the
// storage layer spends its design avoiding, leaking back out through a diagnostic.
func renderVariableList(vars []variables.Variable) string {
	if len(vars) == 0 {
		return "Nothing is remembered yet. Say \"remember this button as save\" to tell me one."
	}
	sort.Slice(vars, func(i, j int) bool { return vars[i].Name < vars[j].Name })

	var b strings.Builder
	fmt.Fprintf(&b, "%d remembered:\n", len(vars))
	for _, v := range vars {
		fmt.Fprintf(&b, "  %-16s %-8s %-10s %q", v.Name, v.Kind,
			firstNonEmpty(string(v.Role), "-"), firstNonEmpty(v.Label, "-"))
		if v.Application != "" {
			fmt.Fprintf(&b, " in %s", v.Application)
		}
		switch {
		case v.History.LastFailure != "":
			fmt.Fprintf(&b, "  — last use FAILED")
		case v.History.LastResolvedAt != nil:
			fmt.Fprintf(&b, "  — last resolved %s", v.History.LastResolvedAt.Format("15:04:05"))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// ExplainVariable renders a variable's full justification.
//
// Every line comes from stored provenance and history. A variable must justify itself
// exactly as a perception element does — and an explanation that invented a reason
// would be worse than none, because it would be believed.
func ExplainVariable(v variables.Variable) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Variable: %s\n", v.Name)
	fmt.Fprintf(&b, "Kind: %s\n", v.Kind)
	if v.Application != "" {
		fmt.Fprintf(&b, "Application: %s\n", v.Application)
	}
	if v.Role != "" {
		fmt.Fprintf(&b, "Role: %s\n", v.Role)
	}
	if v.Label != "" {
		fmt.Fprintf(&b, "Label: %s\n", v.Label)
	}

	b.WriteString("\nStored semantic query:\n")
	if v.Query != nil {
		if v.Query.Application != "" {
			fmt.Fprintf(&b, "  application=%s\n", v.Query.Application)
		}
		if v.Query.Role != "" {
			fmt.Fprintf(&b, "  role=%s\n", v.Query.Role)
		}
		if v.Query.Label != "" {
			fmt.Fprintf(&b, "  label=%s\n", v.Query.Label)
		}
	}

	// Stated explicitly, because "what was NOT stored" is the design and a reader
	// cannot see an absent field.
	b.WriteString("\nStripped during capture:\n")
	b.WriteString("  window handle — transient; a window is not the same window after a restart\n")
	b.WriteString("  ordinal — describes a contender list that will differ next time\n")
	b.WriteString("  element id, native id, coordinates — answers, not questions\n")

	b.WriteString("\nCapture:\n")
	fmt.Fprintf(&b, "  at %s\n", v.Provenance.CapturedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "  confidence %.2f\n", v.Provenance.Confidence)
	if v.Provenance.Phrase != "" {
		fmt.Fprintf(&b, "  from %q\n", v.Provenance.Phrase)
	}
	if v.Provenance.Explanation != "" {
		fmt.Fprintf(&b, "  because %s\n", v.Provenance.Explanation)
	}

	b.WriteString("\nHistory:\n")
	fmt.Fprintf(&b, "  used %d time(s)\n", v.History.Uses)
	if v.History.LastResolvedAt != nil {
		fmt.Fprintf(&b, "  last resolved %s to %q\n",
			v.History.LastResolvedAt.Format("2006-01-02 15:04:05"), v.History.LastResolvedLabel)
	}
	if v.History.LastFailedAt != nil {
		fmt.Fprintf(&b, "  last failed %s: %s\n",
			v.History.LastFailedAt.Format("2006-01-02 15:04:05"), v.History.LastFailure)
	}
	if v.History.Uses == 0 {
		b.WriteString("  never used since capture\n")
	}
	return b.String()
}
