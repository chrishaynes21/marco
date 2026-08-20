package demo

import (
	"fmt"
	"strings"
)

// Explaining an extraction.
//
//	Every extraction decision should be explainable.
//
// The five questions this has to answer, from the milestone:
//
//	Why is this parameter?
//	Why wasn't this parameterized?
//	Why is Rename inferred?
//	Why wasn't this demonstration accepted?
//	Why are these steps constants?
//
// All five are answered from the same list. That is the design: a Decision is recorded at
// the moment a rule fires, by the code that fires it, so an explanation cannot drift from
// what was actually decided. Nothing here re-derives anything — a renderer that recomputed
// the reasoning would be a second implementation of the extractor, and the two would
// disagree exactly when it mattered.

// Explanation is an extraction's decisions, grouped for reading.
type Explanation struct {
	Demonstration ID `json:"demonstration"`
	// Procedure is the candidate's name, empty when the extraction refused.
	Procedure string `json:"procedure,omitempty"`
	// Goal is why the outcome was inferred.
	Goal []Decision `json:"goal,omitempty"`
	// Parameters is why each value became one.
	Parameters []Decision `json:"parameters,omitempty"`
	// Constants is why the rest stayed as demonstrated.
	Constants []Decision `json:"constants,omitempty"`
	// Refusals is why nothing, or nothing more, came back.
	Refusals []Decision `json:"refusals,omitempty"`
}

// Explain groups an extraction's decisions by the question they answer.
func Explain(e Extraction) Explanation {
	out := Explanation{Demonstration: e.Demonstration}
	if e.Candidate != nil {
		out.Procedure = e.Candidate.Name
	}
	for _, d := range e.Decisions {
		switch {
		case d.Verdict == VerdictRefused:
			out.Refusals = append(out.Refusals, d)
		case d.Subject == "goal":
			out.Goal = append(out.Goal, d)
		case d.Verdict == VerdictParameter:
			out.Parameters = append(out.Parameters, d)
		case d.Verdict == VerdictConstant:
			out.Constants = append(out.Constants, d)
		default:
			out.Goal = append(out.Goal, d)
		}
	}
	return out
}

// Describe renders an explanation as the answers to the questions it exists for.
//
// Headed by the QUESTION rather than by the category. A reader arrives here with one of
// five questions in mind, and a list headed "constants" makes them work out that it is the
// answer to "why wasn't this parameterised?".
func (x Explanation) Describe() string {
	var b strings.Builder
	if x.Procedure != "" {
		fmt.Fprintf(&b, "%s — learned from %s\n", x.Procedure, x.Demonstration)
	} else {
		fmt.Fprintf(&b, "%s — nothing was learned from it\n", x.Demonstration)
	}

	section := func(question string, ds []Decision) {
		if len(ds) == 0 {
			return
		}
		fmt.Fprintf(&b, "\n%s\n", question)
		for _, d := range ds {
			fmt.Fprintf(&b, "  %-22s %s\n", d.Subject, d.Reason)
		}
	}
	section("Why this outcome?", x.Goal)
	section("Why is this a parameter?", x.Parameters)
	section("Why wasn't this parameterised?", x.Constants)
	section("Why was this refused?", x.Refusals)
	return b.String()
}

// Trace renders an extraction as the stages it went through, in order.
//
//	Observed semantic actions → recovered goal → recovered parameters →
//	generalized procedure → validation → registry
//
// The same decisions the Explanation groups by QUESTION, ordered instead by WHEN — which
// is the view for "what did the extractor do?" rather than "why is this a parameter?".
// A stage that produced nothing says so: an extraction with no parameters is a fact about
// the demonstration, and a trace that omitted the line would look like a missing stage.
func (e Extraction) Trace(d *Demonstration) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Extraction of %s\n\n", e.Demonstration)

	b.WriteString("Observed semantic actions\n")
	if d == nil || len(d.Steps) == 0 {
		b.WriteString("  (none)\n")
	} else {
		for _, s := range d.Steps {
			fmt.Fprintf(&b, "  %d %s\n", s.Index, s.Describe())
		}
	}

	x := Explain(e)
	stage := func(title string, ds []Decision, empty string) {
		fmt.Fprintf(&b, "\n%s\n", title)
		if len(ds) == 0 {
			fmt.Fprintf(&b, "  %s\n", empty)
			return
		}
		for _, dec := range ds {
			fmt.Fprintf(&b, "  %-22s %s\n", dec.Subject, dec.Reason)
		}
	}
	stage("Recovered goal", x.Goal, "no outcome was recovered")
	stage("Recovered parameters", x.Parameters, "nothing the user typed became a parameter")

	b.WriteString("\nGeneralized procedure\n")
	if e.Candidate == nil {
		b.WriteString("  (none)\n")
	} else {
		for _, s := range e.Candidate.Steps {
			fmt.Fprintf(&b, "  %d %s\n", s.Index, s.Describe())
		}
	}

	b.WriteString("\nValidation\n")
	if e.Refusal != "" {
		fmt.Fprintf(&b, "  REFUSED: %s\n", e.Refusal)
	} else {
		b.WriteString("  every step verified, ordered and durably targeted\n")
	}

	b.WriteString("\nRegistry\n")
	switch {
	case e.Candidate == nil:
		b.WriteString("  nothing was proposed, so nothing can be installed\n")
	default:
		fmt.Fprintf(&b, "  %q is PROPOSED and not installed. Approval is a separate step.\n",
			e.Candidate.Name)
	}
	return b.String()
}

// Answer finds the decisions about one subject, for a question about a specific step or
// parameter.
//
// Substring rather than exact, because a user asks about "new_name" and the decision's
// subject is "parameter new_name" — and making them type the internal form would be a way
// of saying the explanation is for the Director rather than for them.
func (x Explanation) Answer(subject string) []Decision {
	want := strings.ToLower(strings.TrimSpace(subject))
	var out []Decision
	for _, group := range [][]Decision{x.Goal, x.Parameters, x.Constants, x.Refusals} {
		for _, d := range group {
			if strings.Contains(strings.ToLower(d.Subject), want) {
				out = append(out, d)
			}
		}
	}
	return out
}
