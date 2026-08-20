package execute

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/variables"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Variable-aware target resolution.
//
//	A remembered target is a reusable query, not a trusted handle.
//
// This stage does exactly one thing: it swaps a `$name` reference for the QUERY the
// variable stored. Everything after it — observe, rank, clarify, policy, lower,
// execute, verify — is the ordinary path, unchanged and unaware that a variable was
// involved.
//
// That is deliberate and load-bearing. A variable-specific resolver would be a second
// definition of "which control did they mean", and the two would drift until one of
// them clicked the wrong thing. A remembered target gets no more trust than a phrase
// typed a moment ago, because it goes through the same scrutiny.

// VariableStore is the Director's semantic memory, as the pipeline needs it.
//
// An interface so the pipeline can be tested without a file on disk, and so the
// service owns the real store.
type VariableStore interface {
	Get(name string) (variables.Variable, bool)
	RecordResolution(name, label string)
	RecordFailure(name, reason string)
}

// varReference reports whether an intent targets a variable, and which.
func varReference(in directorapi.Intent) (string, bool) {
	if len(in.Targets) == 0 {
		return "", false
	}
	return intent.VariableTarget(in.Targets[0].Phrase)
}

// applyVariable replaces a `$name` target with the variable's stored query.
//
// Returns the variable name so the caller can record the outcome and label the action
// graph entry — the REQUEST was "click $save", and losing that leaves history saying
// only that something called Save was clicked.
func (p *Pipeline) applyVariable(in directorapi.Intent) (directorapi.Intent, string, error) {
	name, ok := varReference(in)
	if !ok {
		return in, "", nil
	}
	if p.Variables == nil {
		return in, name, fmt.Errorf("variables are not available in this Director")
	}

	v, found := p.Variables.Get(name)
	if !found {
		// Named and refused. Never reinterpreted as text: searching the desktop for
		// the literal string "$save" would either find nothing or, far worse, find
		// something.
		return in, name, &variables.ErrUnknown{Name: name}
	}

	q, err := variables.QueryFor(v)
	if err != nil {
		return in, name, err
	}

	// The stored query REPLACES the reference. The phrase is kept as the user's own
	// words so a clarification can quote it back and the trace can show what was asked.
	out := in
	out.Targets = []directorapi.ReferenceExpression{{
		Phrase: "$" + name,
		Kind:   directorapi.ReferenceAnaphoric,
		Query:  &q,
	}}
	return out, name, nil
}

// recordVariableOutcome files what happened, so a variable can explain itself later.
//
// Both outcomes are kept. A variable that resolved yesterday and fails today is a
// different problem from one that never worked, and only the record distinguishes them.
func (p *Pipeline) recordVariableOutcome(name string, res *directorapi.Resolution) {
	if p.Variables == nil || name == "" || res == nil {
		return
	}
	if res.Status == directorapi.ResolutionResolved && res.Target != nil {
		p.Variables.RecordResolution(name, res.Target.Label)
		return
	}
	p.Variables.RecordFailure(name, firstNonEmpty(res.Explanation, string(res.Status)))
}

// variableFailure turns a failed variable lookup into an honest outcome.
//
// The three cases stay distinct because the user's next move differs for each: an
// unknown variable needs remembering again, a missing target needs re-capturing, and an
// unobservable one needs nothing at all — the Director simply could not see.
func variableFailure(name string, v variables.Variable, res directorapi.Resolution) error {
	switch res.Status {
	case directorapi.ResolutionAbsent:
		return &variables.ErrStale{
			Name: name,
			Detail: fmt.Sprintf(
				"Nothing matching %s is present in the current observable interface.", v.Describe()),
		}
	case directorapi.ResolutionUnobservable:
		return fmt.Errorf(
			"Variable %q cannot be resolved because the Director cannot currently observe "+
				"this interface. That is not evidence the target is gone.", name)
	}
	return nil
}

// describeQuery renders a stored query for the trace, without transient identity.
func describeQuery(in directorapi.Intent) string {
	if len(in.Targets) == 0 || in.Targets[0].Query == nil {
		return "no query"
	}
	q := in.Targets[0].Query
	parts := []string{}
	if q.Label != "" {
		parts = append(parts, "label="+q.Label)
	}
	if q.Role != "" {
		parts = append(parts, "role="+string(q.Role))
	}
	if q.Application != "" {
		parts = append(parts, "app="+q.Application)
	}
	if len(parts) == 0 {
		return "an unconstrained query"
	}
	return joinWith(parts, " ")
}

func joinWith(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}
