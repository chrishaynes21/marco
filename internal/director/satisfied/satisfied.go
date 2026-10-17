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
// # Where this is acted on
//
// In the request lifecycle, once: a satisfied request returns ResultNothingToDo, performs no
// input, builds no plan, takes no policy verdict and writes no action record. That was taken
// as a deliberate product decision — a request can now succeed WITHOUT performing its action
// — in [[ADR-221-a-question-the-person-already-answered]].
//
// A PERSON'S REQUEST, not a step of a procedure. A precondition already met means a procedure
// should continue, and the rename procedure found that the hard way: its first step selects the
// file the person pointed at, and in a window where that file was already selected, satisfaction
// ended the request before F2 was ever pressed.
//
// Enforced by cmd/director TestSatisfactionIsDecidedInExactlyOnePlace and, for the behaviour,
// internal/director/execute TestARequestForAPageThatIsOpenPerformsNothingAndSaysSo.
type Satisfaction struct {
	// Already is true when the request names a state the screen is in.
	Already bool
	// Because is the sentence to say instead of acting.
	Because string
	// ElementID and Source are the EVIDENCE: which Prop was read, and which sensor said
	// so. A no-op success is still an outcome somebody has to be able to audit, and
	// "nothing needed doing" without a receipt is indistinguishable from "I did not try".
	ElementID directorapi.ElementID
	Source    directorapi.ObservationSource
}

// Already reports whether a resolution's own answer is a state the screen is already in.
//
// # Why a resolution rather than an element
//
// Because the measured case is an AMBIGUOUS one. Asked to "open Apps" on a Settings window
// whose navigation already had Apps selected, Marco found two operable controls with that
// label and asked which was meant. Both answers were wrong in the same way, and the question
// itself was the wrong thing to say: Apps is open, and no control needs pressing.
//
// So the contenders are read as well as the target. A request is satisfied when ANY control
// this resolution would offer is already in the state asked for, because pressing any of the
// others would not change that — and asking somebody to choose between them is asking a
// question whose answers are all "nothing happens".
//
// Best first, and the first satisfied one wins, so the sentence names the control the resolver
// ranked highest rather than whichever the map happened to yield.
func Already(w *directorapi.WorldState, res directorapi.Resolution, verb string) Satisfaction {
	// The verb test is an EARLY-OUT, not the guarantee: AlreadyThere asks it again of every
	// control, which is where the rule lives. The mutation gate confirmed that deleting this
	// line changes no outcome. It is here so a reader of the function the lifecycle calls
	// can see what it will and will not answer, and so a screenful of controls is not
	// walked for a verb that could never be satisfied.
	if w == nil || !destinationVerbs[verb] {
		return Satisfaction{}
	}
	var ids []directorapi.ElementID
	if res.Target != nil {
		ids = append(ids, res.Target.ElementID)
	}
	for _, c := range res.Contenders {
		ids = append(ids, c.ElementID)
	}
	for _, id := range ids {
		el, ok := w.Element(id)
		if !ok {
			continue
		}
		if s := AlreadyThere(el, verb); s.Already {
			return s
		}
	}
	return Satisfaction{}
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
	// AND THE APPLICATION MUST HAVE SAID SO ITSELF.
	//
	// A no-op success is a claim that nothing needed doing, and it has to rest on observed
	// state rather than on a label or an appearance. Two things could set `Selected`
	// without the application reporting it: a value nobody wrote at all, and a visual
	// detector filling the gap from what a region LOOKS like — which fusion records
	// honestly as coming from vision, and which is exactly the kind of evidence ADR-202
	// says may describe a Prop and may not vouch for it.
	//
	// Appearance is good enough to say "this looks like the current tab" and not good
	// enough to tell somebody their request is done. So the state must carry a StateFact
	// from a STRUCTURED source: the application's own selection report.
	fact, told := el.StateEvidence[directorapi.StateSelected]
	if !told || !fact.Value || !fact.Source.Structured() {
		return Satisfaction{}
	}
	name := el.Label
	if name == "" {
		name = string(el.Role)
	}
	return Satisfaction{
		Already:   true,
		ElementID: el.ID,
		Source:    fact.Source,
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
