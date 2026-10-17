package verify

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/world"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// DID THE PERSON GET WHAT THEY ASKED FOR, as distinct from did the steps run.
//
// # Why this is a different question from verifying a step
//
// Measured live. "Make a new folder called Reports" ran Explorer's procedure: invoke the New
// menu, choose Folder, type the name, confirm. Marco created the folder and stopped at step
// two, and had it finished, the sentence it would have reported was
//
//	4 of 4 steps verified
//
// — which is a claim about Marco's own mechanism, not about the person's folder. A procedure
// whose last step typed into the wrong control and pressed Enter completes four steps too.
//
// So a goal declares the OUTCOME it exists for, in the condition vocabulary
// `directorapi.Plan.SuccessConditions` has always described as "must ALL hold after the plan
// for it to count as done" — a sentence that was true of nothing, because nothing set those
// conditions and nothing read them. This reads them.
//
// # The one rule that makes it safe
//
// A CONDITION THIS CANNOT EVALUATE IS NOT SATISFIED. It is reported as unchecked, and an
// unchecked outcome is not a completed request. The alternative — treating "I do not know how
// to look at that" as a pass — would make every future condition type a silent success, which
// is the exact shape of the defect this is fixing one tier down.

// Outcome is what became of a goal's declared outcome conditions.
type Outcome struct {
	// Met and Unmet are the conditions that held and did not.
	Met, Unmet []string
	// Unchecked are the conditions this could not evaluate at all. Never a pass.
	Unchecked []string
}

// Satisfied reports whether the person got what they asked for: every condition met, none
// unmet, and none left unchecked.
func (o Outcome) Satisfied() bool {
	return len(o.Met) > 0 && len(o.Unmet) == 0 && len(o.Unchecked) == 0
}

// Describe is the sentence a person reads instead of a step count.
func (o Outcome) Describe() string {
	switch {
	case o.Satisfied():
		return strings.Join(o.Met, ", and ")
	case len(o.Unmet) > 0:
		return strings.Join(o.Unmet, ", and ")
	case len(o.Unchecked) > 0:
		return "could not check " + strings.Join(o.Unchecked, ", or ")
	default:
		return "nothing was declared to check"
	}
}

// CheckOutcome evaluates a goal's outcome conditions against the world as it now is.
//
// `touched` are the controls the program's own steps acted on, and they are EXCLUDED from the
// evidence. That exclusion is not a refinement; it is the whole difference between checking the
// outcome and checking the mechanism again, and it was measured as a false success.
//
// # What happened without it
//
// Live, on a disposable Explorer folder. "Make a new folder called Reports" ran all four steps:
// the New menu, the Folder entry, type "Reports" into the inline editor, confirm. The
// confirmation lowered to a CLICK, which does not commit an Explorer rename, so the folder
// stayed "New folder" — and Marco reported
//
//	COMPLETED: text_field "Reports" is there
//
// It was. Marco had typed it there. The outcome condition — something on screen called
// "Reports" — was satisfied by the edit box holding Marco's own keystrokes, which is evidence
// about Marco and not about the person's disk.
//
// So the rule: a control Marco wrote to cannot be the proof that the application accepted what
// was written. The application has to say it somewhere Marco did not reach.
func CheckOutcome(conds []directorapi.Condition, w *directorapi.WorldState,
	touched ...directorapi.ElementID) Outcome {

	ignore := make(map[directorapi.ElementID]bool, len(touched))
	for _, id := range touched {
		ignore[id] = true
	}
	var out Outcome
	for _, c := range conds {
		held, known, detail := holds(c, w, ignore)
		switch {
		case !known:
			out.Unchecked = append(out.Unchecked, detail)
		case held:
			out.Met = append(out.Met, detail)
		default:
			out.Unmet = append(out.Unmet, detail)
		}
	}
	return out
}

// holds evaluates one condition, and says whether it could be evaluated at all.
//
// The supported set is deliberately small: the conditions a GOAL can state about its own
// outcome. Everything else returns unknown rather than false, because "not supported" and "not
// true" call for different sentences — one is a gap in Marco and the other is a failed request.
func holds(c directorapi.Condition, w *directorapi.WorldState,
	ignore map[directorapi.ElementID]bool) (held, known bool, detail string) {
	switch c.Type {
	case directorapi.ConditionTextVisible:
		want := strings.TrimSpace(strings.ToLower(wantedText(c)))
		if want == "" {
			return false, false, "a text condition with no text to look for"
		}
		if el := visibleBearing(w, want, ignore); el != nil {
			return true, true, fmt.Sprintf("%s is there", el.ForAccount())
		}
		return false, true, fmt.Sprintf("nothing on screen is called %s", named(wantedText(c)))

	case directorapi.ConditionNot:
		if len(c.Sub) != 1 {
			return false, false, "a negation of " + fmt.Sprint(len(c.Sub)) + " conditions"
		}
		inner, innerKnown, innerDetail := holds(c.Sub[0], w, ignore)
		if !innerKnown {
			return false, false, innerDetail
		}
		return !inner, true, "not: " + innerDetail

	case directorapi.ConditionAll, directorapi.ConditionAny:
		if len(c.Sub) == 0 {
			return false, false, string(c.Type) + " with nothing in it"
		}
		met, unmet, unknown := 0, "", ""
		for _, sub := range c.Sub {
			subHeld, subKnown, subDetail := holds(sub, w, ignore)
			switch {
			case !subKnown:
				unknown = subDetail
			case subHeld:
				met++
			default:
				unmet = subDetail
			}
		}
		if unknown != "" {
			return false, false, unknown
		}
		if c.Type == directorapi.ConditionAny {
			if met > 0 {
				return true, true, "at least one of the alternatives held"
			}
			return false, true, unmet
		}
		if met == len(c.Sub) {
			return true, true, "all of them held"
		}
		return false, true, unmet
	}
	return false, false, c.Describe()
}

// wantedText is the text a condition is about, from Value or from its query.
func wantedText(c directorapi.Condition) string {
	if c.Value != "" {
		return c.Value
	}
	if c.Query != nil {
		if c.Query.Label != "" {
			return c.Query.Label
		}
		return c.Query.Text
	}
	return ""
}

// visibleBearing finds a visible element whose label or value is the wanted text.
//
// EXACT on the normalised text rather than a substring, because an outcome check is the last
// thing standing between a procedure and a claim of success: "Reports" must not be satisfied by
// "Reports (1)" or by "Quarterly Reports Draft", which are different folders.
func visibleBearing(w *directorapi.WorldState, want string,
	ignore map[directorapi.ElementID]bool) *directorapi.Element {

	for _, el := range world.Elements(w) {
		if !el.Visible || ignore[el.ID] {
			continue
		}
		for _, text := range []string{el.Label, el.Value} {
			if strings.TrimSpace(strings.ToLower(text)) == want {
				return el
			}
		}
	}
	return nil
}
