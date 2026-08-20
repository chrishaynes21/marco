package goal

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Binding a deictic directive during expansion.
//
//	When a procedure directive is deictic … resolve it through
//	internal/director/binding. A deictic directive must not reach RunProgram with only
//	a generic focused-element target.
//
// Expansion used to answer "rename THIS file" with a query for whatever held focus. That
// is not a target: it is a description of a moment, and the moment it describes is the
// one before the user finished speaking. A folder, an unsaved tab, a text selection and
// an unrelated button all satisfy it, and each turns a rename into something else.
//
// So a deictic directive is now RESOLVED here, against the world, into a typed binding
// with evidence — or refused. There is no third answer and deliberately no fallback: a
// procedure that cannot say what its target is does not get to act on whatever is there.
//
// The goal package does not observe. It asks a Binder, which the pipeline supplies from
// the world it has already looked at, so expansion stays a pure function of the goal, the
// registry and one injected resolution.

// Binder resolves a deictic reference for an expanding procedure.
//
// One method, and it may refuse. The refusal is TYPED — a binding.Problem distinguishes
// "several things are equally plausible", which the user can settle, from "the focused
// thing is a folder", which they cannot.
type Binder interface {
	Bind(BindRequest) (*binding.Binding, *binding.Problem)
}

// BindRequest is everything the resolver needs to bind one directive's target.
type BindRequest struct {
	// Phrase is the user's own words for the object ("this file").
	Phrase string
	// Expected is the kind the procedure declared its target must be.
	Expected binding.ObjectKind
	// Application constrains resolution to the app the procedure was selected for,
	// empty for the active one.
	Application string
	// Semantic is the action the bound object will receive, when the directive is a
	// semantic one. A target constraint rather than a mechanism: "the object this
	// select will act on".
	Semantic directorapi.SemanticActionKind
	// Focus marks a directive that focuses rather than acts.
	Focus bool
	// Origin is the goal, procedure and step this request came from.
	Origin binding.Origin
}

// Describe renders a request for a diagnostic line.
func (r BindRequest) Describe() string {
	return fmt.Sprintf("%q must be %s", r.Phrase, r.Expected.Describe())
}

// BindingFailure is an expansion refused because a deictic target could not be bound.
//
// Separate from Refusal because the two are answered differently: a Refusal is missing
// INFORMATION and the front-end asks for it, while this is a mismatch between what the
// user pointed at and what the procedure needs. Some of these are clarifiable and some
// are not, which is what Problem.Clarifiable reports.
type BindingFailure struct {
	Goal      Kind             `json:"goal"`
	Procedure string           `json:"procedure"`
	StepIndex int              `json:"step_index"`
	Phrase    string           `json:"phrase"`
	Problem   *binding.Problem `json:"problem"`
}

func (f BindingFailure) Error() string {
	if f.Problem == nil {
		return fmt.Sprintf("%s could not resolve what %q points at", f.Goal.Describe(), f.Phrase)
	}
	return f.Problem.Message
}

// Clarifiable reports whether the user could settle this by answering.
func (f BindingFailure) Clarifiable() bool {
	return f.Problem != nil && f.Problem.Clarifiable()
}

// Question renders the failure as something to ask.
func (f BindingFailure) Question() string {
	if f.Problem == nil {
		return f.Error()
	}
	if f.Problem.Reason == binding.ReasonAmbiguous && len(f.Problem.Candidates) > 0 {
		names := make([]string, 0, len(f.Problem.Candidates))
		for _, c := range f.Problem.Candidates {
			label := c.Label
			if c.Resource != "" {
				label = c.Resource
			}
			names = append(names, label)
		}
		return fmt.Sprintf("%s — which one? %s", f.Goal.Describe(), joinOr(names))
	}
	return f.Problem.Message
}

func joinOr(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	out := ""
	for i, p := range parts {
		switch {
		case i == 0:
			out = p
		case i == len(parts)-1:
			out += " or " + p
		default:
			out += ", " + p
		}
	}
	return out
}
