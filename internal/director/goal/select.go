package goal

import (
	"fmt"
	"sort"
	"strings"
)

// Choosing between procedures.
//
//	A single strictly more-specific match should still win.
//	Two incomparable or equally specific matches must produce clarification or a
//	deterministic ambiguity error.
//
// The registry used to answer with the first procedure that named the application, so
// registration ORDER silently decided between two equally good candidates. That is the
// kind of bug that never shows up in a test and shows up once, in front of a user, as
// the wrong menu being opened.
//
// Specificity here is a documented SEMANTIC rule rather than a priority number: one
// procedure is more specific than another when it constrains strictly more. A procedure
// naming this application beats one naming none, because "for Explorer" is a superset of
// the conditions "for anything" imposes. Two procedures that both name the application
// constrain the same amount and neither wins — that is ambiguity, and it is reported
// rather than resolved by whichever was registered first.

// Candidate is one procedure considered for a goal, and why it matched.
type Candidate struct {
	Procedure string `json:"procedure"`
	// Specificity is how much this procedure constrains. Higher is more specific.
	Specificity int `json:"specificity"`
	// Why explains the match in one phrase, for diagnostics.
	Why string `json:"why"`
}

// Selection is the outcome of choosing, with everything that competed.
//
//	Diagnostics must show all competing goal/procedure candidates and why each
//	matched.
type Selection struct {
	// Chosen is the winner's name, empty when nothing matched or it was ambiguous.
	Chosen string `json:"chosen,omitempty"`
	// Candidates are every procedure that could have served, most specific first.
	Candidates []Candidate `json:"candidates,omitempty"`
	// Ambiguous is set when two candidates were equally specific.
	Ambiguous bool `json:"ambiguous,omitempty"`
	// Reason explains an ambiguity or a miss.
	Reason string `json:"reason,omitempty"`
}

// specificity scores how much a procedure constrains.
//
// Two levels today, and deliberately not a free integer a contributor can tune: an
// application-scoped procedure constrains strictly more than a generic one, and there is
// no third thing to say. When a procedure gains role or window constraints this becomes
// a comparison over those dimensions rather than a bigger number.
func specificity(p Procedure) int {
	if p.Generic() {
		return 0
	}
	return 1
}

// SelectProcedure picks the procedure for a goal, reporting everything that competed.
//
// Returns ok=false for both "nothing matched" and "two things matched equally". The
// caller distinguishes them by Selection.Ambiguous, because they need different
// answers: one is "the Director cannot do this", the other is a question.
func (r *Registry) SelectProcedure(g Goal) (Procedure, Selection, bool) {
	var matched []Procedure
	sel := Selection{}

	for _, p := range r.procedures {
		if p.Goal != g.Kind || !p.Serves(g.Context.Application) {
			continue
		}
		matched = append(matched, p)
		sel.Candidates = append(sel.Candidates, Candidate{
			Procedure: p.Name, Specificity: specificity(p), Why: whyMatched(p, g),
		})
	}
	if len(matched) == 0 {
		sel.Reason = fmt.Sprintf("no procedure serves %s", g.Kind.Describe())
		return Procedure{}, sel, false
	}

	// Most specific first, then by name so the listing is stable. The sort is for
	// DISPLAY; the decision below reads the scores rather than the order, so a stable
	// tie-break here cannot quietly become the tie-break for the choice.
	sort.SliceStable(sel.Candidates, func(i, j int) bool {
		if sel.Candidates[i].Specificity != sel.Candidates[j].Specificity {
			return sel.Candidates[i].Specificity > sel.Candidates[j].Specificity
		}
		return sel.Candidates[i].Procedure < sel.Candidates[j].Procedure
	})

	best, top := -1, 0
	for _, p := range matched {
		if s := specificity(p); s > best {
			best, top = s, 1
		} else if s == best {
			top++
		}
	}
	if top > 1 {
		// PROVENANCE breaks the tie, and only this tie. Two procedures constrain the same
		// amount and exactly one was demonstrated on this machine and approved by this
		// user: that is a stronger statement about this desktop than a procedure written
		// for desktops in general, and it is a stated rule rather than registration order.
		//
		// Deliberately not folded into specificity. A learned procedure does not constrain
		// MORE — it constrains the same and comes from somewhere better — and pretending
		// otherwise would make the specificity score mean two things.
		if p, ok := onlyLearned(matched, best); ok {
			sel.Chosen = p.Name
			sel.Reason = fmt.Sprintf(
				"%s was demonstrated here and approved, so it is preferred over the "+
					"built-in procedure that constrains the same amount", p.Name)
			return p, sel, true
		}
		var names []string
		for _, c := range sel.Candidates {
			if c.Specificity == best {
				names = append(names, c.Procedure)
			}
		}
		sel.Ambiguous = true
		sel.Reason = fmt.Sprintf(
			"%s could be carried out by %s, and nothing distinguishes them. "+
				"Refusing rather than letting registration order decide.",
			g.Kind.Describe(), strings.Join(names, " or "))
		return Procedure{}, sel, false
	}

	for _, p := range matched {
		if specificity(p) == best {
			sel.Chosen = p.Name
			return p, sel, true
		}
	}
	return Procedure{}, sel, false
}

// onlyLearned returns the single learned procedure among the top-scoring candidates.
//
// Returns false when none is learned, and false when SEVERAL are: two learned procedures
// for the same goal in the same application are two demonstrations of the same task, and
// which one the user meant is a question rather than an answer. The tie-break exists to
// prefer what the user showed the Director over what it was shipped with, not to make
// learned procedures compete.
func onlyLearned(matched []Procedure, best int) (Procedure, bool) {
	var found Procedure
	n := 0
	for _, p := range matched {
		if specificity(p) == best && p.Learned {
			found, n = p, n+1
		}
	}
	return found, n == 1
}

// whyMatched explains one candidate's match.
func whyMatched(p Procedure, g Goal) string {
	origin := ""
	if p.Learned {
		origin = "learned from a demonstration; "
	}
	if p.Generic() {
		return origin + "generic: serves any application"
	}
	app := g.Context.Application
	if app == "" {
		app = "(none named)"
	}
	return fmt.Sprintf("%snames %s, matching %q", origin,
		strings.Join(p.Applications, "/"), app)
}

// Select picks the procedure for a goal.
//
// Kept as the simple form for callers that do not need the competition, and defined in
// terms of SelectProcedure so there is one selection rule rather than two.
func (r *Registry) Select(g Goal) (Procedure, bool) {
	p, _, ok := r.SelectProcedure(g)
	return p, ok
}

// ShadowedProcedure is a registration that can never be chosen.
type ShadowedProcedure struct {
	Procedure string `json:"procedure"`
	By        string `json:"by"`
	Reason    string `json:"reason"`
}

// Validate reports registrations that are permanently unreachable or ambiguous.
//
//	Add registry validation that catches permanently shadowed procedures where
//	possible.
//
// Two faults are detectable without a request: a duplicate NAME, which makes `director
// procedure <name>` answer about whichever was registered first; and two procedures for
// the same goal naming the SAME application, which can never be told apart and would
// make every such request ambiguous at run time rather than at startup.
//
// A generic procedure is never reported as shadowed: it is the fallback for every
// application the specific ones do not name, which is most of them.
func (r *Registry) Validate() []ShadowedProcedure {
	var out []ShadowedProcedure

	seen := map[string]bool{}
	for _, p := range r.procedures {
		key := strings.ToLower(p.Name)
		if seen[key] {
			out = append(out, ShadowedProcedure{
				Procedure: p.Name, By: p.Name,
				Reason: "a procedure with this name is already registered, so this one " +
					"can never be reached by name",
			})
		}
		seen[key] = true
	}

	// Same goal, same application, both specific: permanently ambiguous.
	for i, a := range r.procedures {
		if a.Generic() {
			continue
		}
		for j := i + 1; j < len(r.procedures); j++ {
			b := r.procedures[j]
			if b.Generic() || a.Goal != b.Goal {
				continue
			}
			if a.Learned != b.Learned {
				// One was demonstrated here and one was shipped. That pair is decided by
				// the provenance rule in SelectProcedure rather than being ambiguous, and
				// reporting it as shadowed would refuse to start the service the moment a
				// user shows the Director something it already knew how to do.
				continue
			}
			if shared := sharedApplications(a, b); len(shared) > 0 {
				out = append(out, ShadowedProcedure{
					Procedure: b.Name, By: a.Name,
					Reason: fmt.Sprintf(
						"both serve %s for %s, so neither can be chosen over the other",
						a.Goal, strings.Join(shared, "/")),
				})
			}
		}
	}
	return out
}

// NewValidatedRegistry builds the built-in library and refuses to return one that is
// internally inconsistent.
//
//	Invalid built-in registrations fail startup before serving requests.
//	Validation must not be deferred until a matching request happens.
//
// The alternative — validating lazily — means a duplicate registration is discovered by
// the first user who happens to ask for that goal, mid-request, on a live desktop. A
// registry is fixed at build time, so its faults are build-time faults and belong at
// startup where they stop the process rather than a command.
//
// Tests that need a deliberately broken registry construct one directly and call
// Validate themselves, so this never gets in the way of testing the failure.
func NewValidatedRegistry() (*Registry, error) {
	r := NewRegistry()
	shadowed := r.Validate()
	if len(shadowed) == 0 {
		return r, nil
	}
	var b strings.Builder
	b.WriteString("the procedure registry is not usable:")
	for _, s := range shadowed {
		fmt.Fprintf(&b, "\n  %s (shadowed by %s): %s", s.Procedure, s.By, s.Reason)
	}
	b.WriteString("\nThis is a build-time fault in the built-in procedure library, not " +
		"something the current request caused.")
	return nil, fmt.Errorf("%s", b.String())
}

func sharedApplications(a, b Procedure) []string {
	var out []string
	for _, x := range a.Applications {
		for _, y := range b.Applications {
			if strings.EqualFold(x, y) {
				out = append(out, x)
			}
		}
	}
	return out
}
