package demo

import (
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A learned procedure is an ordinary procedure.
//
//	Approved procedures enter the procedure registry. Exactly the same registry used by
//	built-in procedures. No separate runtime.
//
// So this file is an ADAPTER and nothing more. A Learned is data — steps, parameters, a
// goal, an application — and AsProcedure turns it into the goal.Procedure the registry
// already holds. From that point on nothing downstream can tell the difference, which is
// the whole claim: the expansion, the validation, the binding, the confirmation, the
// lowering into Marco and the verification are the built-in path because they ARE the
// built-in path.
//
// What a learned procedure is NOT allowed to be is code. Its Steps function below reads
// stored data and emits directives; it never evaluates anything the user wrote, because
// "user-written procedure code" is a non-goal of this milestone and a very different
// security position.

// Learned is an approved procedure, as it is stored.
//
// The same shape as a Candidate plus the approval. Kept as its own type rather than a
// flag on Candidate so that "proposed" and "approved" cannot be confused by a caller
// holding one — the type is the difference.
type Learned struct {
	Candidate
	// ApprovedAt is when the user accepted it. A learned procedure with no approval
	// cannot exist: Approve is the only constructor.
	ApprovedAt time.Time `json:"approved_at"`
	// ApprovedBy is how the approval arrived, for the audit trail.
	ApprovedBy string `json:"approved_by,omitempty"`
	// Decisions is the extraction's own account of why this procedure has the shape it
	// has. Stored WITH the procedure, so `director explain procedure` can answer months
	// later without the demonstration having to be re-read — and so the answer is the one
	// the user approved rather than one a newer extractor would give now.
	Decisions []Decision `json:"decisions,omitempty"`
}

// Approve turns a proposal into a learned procedure.
//
//	The extractor proposes. The user approves. The Director does not silently install
//	learned procedures.
//
// The gate is this function's existence: nothing else constructs a Learned, extraction
// returns an Extraction, and an Extraction is not something the registry accepts.
//
// It takes the whole extraction rather than the candidate alone so the decisions travel
// with the procedure. What the user approved is the proposal AND its reasoning, and an
// explanation reconstructed later by a newer extractor would be a different one.
func Approve(e Extraction, by string, at time.Time) (*Learned, error) {
	c := e.Candidate
	switch {
	case c == nil && e.Refusal != "":
		return nil, fmt.Errorf("there is nothing to approve: %s", e.Refusal)
	case c == nil:
		return nil, fmt.Errorf("there is no proposal to approve")
	case len(c.Steps) == 0:
		return nil, fmt.Errorf("%q has no steps", c.Name)
	case !c.Goal.Known():
		return nil, fmt.Errorf("%q claims to achieve %q, which is not an outcome this "+
			"Director knows", c.Name, c.Goal)
	}
	return &Learned{
		Candidate: *c, ApprovedAt: at, ApprovedBy: by,
		Decisions: append([]Decision{}, e.Decisions...),
	}, nil
}

// AsProcedure adapts a learned procedure into the registry's own type.
func (l *Learned) AsProcedure() goal.Procedure {
	steps := append([]CandidateStep{}, l.Steps...)
	params := append([]Parameter{}, l.Parameters...)

	p := goal.Procedure{
		Name:         l.Name,
		Goal:         l.Goal,
		Applications: applicationsOf(l.Application),
		Requires:     requirementsOf(l),
		Expect:       binding.ObjectKind(l.Expect),
		Safety:       l.Safety,
		Learned:      true,
		Why: fmt.Sprintf("learned from a demonstration (%s): %s",
			l.Source, l.Why),
		Steps: func(g goal.Goal) ([]goal.Directive, error) {
			return directivesFor(steps, params, g)
		},
	}
	return p
}

// applicationsOf scopes a learned procedure to where it was demonstrated.
//
// A learned procedure is NEVER generic. It was demonstrated once, in one application, and
// claiming it works anywhere is a claim nobody made — the built-in generic procedures are
// written to work anywhere, and this was not written at all.
func applicationsOf(app string) []string {
	if strings.TrimSpace(app) == "" {
		return nil
	}
	return []string{app}
}

// requirementsOf is what a learned procedure needs before it can expand.
//
// Derived from its own shape rather than declared: it needs a target when one of its steps
// points at one, and it needs each parameter that fills a goal role. A requirement it does
// not declare is a step that discovers mid-run that it has nothing to type.
func requirementsOf(l *Learned) []goal.Requirement {
	var out []goal.Requirement
	for _, s := range l.Steps {
		if s.Deictic {
			out = append(out, goal.RequiresTarget)
			break
		}
	}
	seen := map[goal.Requirement]bool{}
	for _, p := range l.Parameters {
		var req goal.Requirement
		switch p.Role {
		case goal.ParamName:
			req = goal.RequiresName
		case goal.ParamDestination:
			req = goal.RequiresDestination
		default:
			continue
		}
		if !seen[req] {
			seen[req] = true
			out = append(out, req)
		}
	}
	return out
}

// directivesFor expands stored steps into the typed directives the goal layer executes.
//
// The parameter substitution happens HERE, at expansion, against the goal the user
// actually asked for — never against the demonstrated example. A learned rename asked for
// with no new name is refused by the requirement above; one asked for with a new name types
// THAT name. There is no path by which the demonstrated value is typed into anything.
func directivesFor(steps []CandidateStep, params []Parameter, g goal.Goal) (
	[]goal.Directive, error) {

	byStep := map[int]Parameter{}
	for _, p := range params {
		byStep[p.Step] = p
	}

	out := make([]goal.Directive, 0, len(steps))
	for _, s := range steps {
		d := goal.Directive{
			Phrase:        s.Phrase,
			Preconditions: append([]directorapi.Condition{}, s.Preconditions...),
		}
		switch s.Kind {
		case StepEdit:
			d.SetText = true
			if p, ok := byStep[s.Index]; ok {
				text, err := valueFor(p, g)
				if err != nil {
					return nil, err
				}
				d.Text = text
			} else {
				d.Text = s.Text
			}
		case StepFocus:
			d.Focus = true
		default:
			if !s.Semantic.Known() {
				return nil, fmt.Errorf("step %d asks for %q, which is not an action this "+
					"Director can perform", s.Index, s.Semantic)
			}
			d.Semantic = s.Semantic
		}

		switch {
		case s.DerivedEditor:
			d.TargetEditor = true
		case s.Deictic:
			d.TargetDeictic = true
			d.Target = g.Context.Target
		case s.Role != "":
			d.Role = s.Role
		}
		if d.Phrase == "" {
			d.Phrase = s.Describe()
		}
		out = append(out, d)
	}
	return out, nil
}

// valueFor reads a parameter's value out of the goal the user asked for.
func valueFor(p Parameter, g goal.Goal) (string, error) {
	if p.Role != "" {
		if v := strings.TrimSpace(g.Param(p.Role)); v != "" {
			return v, nil
		}
	}
	if v := strings.TrimSpace(g.Param(p.Name)); v != "" {
		return v, nil
	}
	// Refused rather than falling back to the demonstrated example. A procedure that
	// typed the example would rename the user's second file to the first one's name —
	// silently, and with every step verifying.
	q := p.Prompt
	if q == "" {
		q = fmt.Sprintf("what should %s be?", p.Name)
	}
	return "", goal.Refusal{
		Goal:   g.Kind,
		Reason: fmt.Sprintf("%s — %s", g.Kind.Describe(), q),
	}
}

// Describe renders a learned procedure for review, in the form the proposal is shown in.
func (c *Candidate) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", c.Name)
	fmt.Fprintf(&b, "Goal\n  %s\n", c.Goal.Describe())
	if c.Application != "" {
		fmt.Fprintf(&b, "\nApplication\n  %s\n", c.Application)
	}
	if len(c.Parameters) > 0 {
		b.WriteString("\nParameters\n")
		for _, p := range c.Parameters {
			line := "  " + p.Name
			if p.Example != "" {
				line += fmt.Sprintf("   (you demonstrated it with %q)", p.Example)
			}
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\nSteps\n")
	for _, s := range c.Steps {
		fmt.Fprintf(&b, "  %d %s\n", s.Index, s.Describe())
		for _, pre := range s.Preconditions {
			fmt.Fprintf(&b, "       waits until: %s\n", pre.Describe())
		}
	}
	return b.String()
}
