package satisfied

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Satisfaction says the screen is already in the state the request asked for.
//
// # Why this is not rewording, and is not a resolution either
//
// Because it is a different question from "which control did they mean", and the measurement
// that produced it said so plainly.
// Asked to "open Apps" on a Settings window whose navigation list ALREADY had Apps
// selected, Marco found two operable controls labelled "Apps" and asked which one was meant.
// Both answers it could have given were wrong in the same way: the honest answer is that
// Apps is open, and that no control needs pressing at all.
//
// So this must never be counted as a resolution success. A resolution is "here is the
// control you meant"; this is "nothing needs doing", and the two are told apart because they
// are acted on differently — one presses something and one says so and stops. A system that
// scored them together would be rewarded for pressing a control it did not need to press,
// which is the opposite of what we want to measure.
//
// # What it reads, and why that is the only honest signal
//
// `Element.Selected` — the interface's OWN report that this is the current choice, carried
// from the accessibility source's selection pattern through fusion onto the Prop. Not a
// claim derived about the Place, not a breadcrumb label read out of the content area, not a
// geometry heuristic about which list member looks highlighted. The application is the only
// thing that knows which of its destinations is open, and this is the one place it says.
//
// That narrowness is the point. A rule that inferred "already open" from a page header
// repeating a navigation label would be a rule about applications that happen to print their
// page name, and the first application that did not would be told its request was satisfied
// when it was not — a worse failure than asking an unnecessary question, because it does
// nothing and reports success.
//
// # Not wired into execution
//
// Deliberately, and this is a boundary rather than an omission. Acting on satisfaction means
// a request can now succeed WITHOUT performing its action, which is a change to what
// "performed" means — it touches the plan, the policy verdict, the confirmation and the
// record a person reads afterwards. That is a decision about product behaviour, not a
// resolution detail, and it is taken separately from rewording.
//
// Enforced by cmd/director TestSatisfactionIsNotWiredIntoExecution.
type Satisfaction struct {
	// Already is true when the request names a state the screen is in.
	Already bool
	// Because is the sentence to say instead of acting.
	Because string
}

// AlreadyThere reports whether el is already in the state verb would put it in.
//
// Only the verbs that name a DESTINATION are answered. "Click" is mechanical — a person who
// says click wants a press, and a selected control can be pressed again — and refresh, next
// and submit have no state to be in. The verbs here all mean "be at this place", which is
// the only kind of request a selection can already satisfy.
func AlreadyThere(el *directorapi.Element, verb string) Satisfaction {
	if el == nil || !destinationVerbs[verb] {
		return Satisfaction{}
	}
	if !el.Selected {
		return Satisfaction{}
	}
	name := el.Label
	if name == "" {
		name = string(el.Role)
	}
	return Satisfaction{
		Already: true,
		Because: fmt.Sprintf("%q is already %s, so nothing needs pressing", name,
			selectedWord(el.Role)),
	}
}

// destinationVerbs are the semantic actions that mean "be at this place".
var destinationVerbs = map[string]bool{
	string(directorapi.SemanticOpen):   true,
	string(directorapi.SemanticSelect): true,
	string(directorapi.SemanticChoose): true,
}

// selectedWord says what being selected means for a kind of control, because "already
// selected" is the wrong sentence about a page.
func selectedWord(role directorapi.ElementRole) string {
	switch role {
	case directorapi.RoleTab, directorapi.RoleListItem, directorapi.RoleTreeItem:
		return "the one you are on"
	default:
		return "selected"
	}
}
