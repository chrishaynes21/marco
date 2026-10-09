package execute

import (
	"context"
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/search"
	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/conditions"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/evaluation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Looking for a target that is not in the reading.
//
// The pipeline's resolve stage has four answers and treats two of them as recoverable:
//
//	unobservable   the reading did not represent the window → LOOK AGAIN, same place
//	absent         the window was read and the thing is not in it → LOOK SOMEWHERE ELSE
//
// The first was already here, and its comment is careful about why a second look is free: "a
// resolution that failed sent no click and pressed no key; the desktop does not know Marco
// looked." The second is this file, and the difference between them is the difference between
// re-reading a page and turning one.
//
// Only `absent` earns a search. `unobservable` means Marco could not read the place it is in,
// and moving the view would be looking harder at something it cannot see — it would spend the
// budget producing more unreadable places and then report `unreadable`, which is what it
// already knew.

// searchConditionID is this caller's condition, named the way internal/director/game names its
// own: the ID set is open, and a condition that borrowed `element_exists` would misreport what
// was actually evaluated. Resolution is a stronger question than existence — it weighs
// candidates, rejects scenery and refuses a near-tie.
const searchConditionID conditions.ID = "target.resolves"

// resolvesHere asks the only question the pipeline needs of a place: would the request's target
// resolve against this reading?
//
// Not `element_exists`. A request naming nothing in particular ("the field") has no label to
// exist, and a label that exists on three controls does not resolve. Asking the resolver is the
// same question the pipeline is about to ask, so a search that stops cannot stop somewhere the
// pipeline would then refuse.
type resolvesHere struct {
	resolver *target.Resolver
	query    directorapi.ElementQuery
	phrase   string
}

func (c resolvesHere) ID() conditions.ID { return searchConditionID }

func (c resolvesHere) Description() string {
	if c.phrase != "" {
		return c.phrase
	}
	return "the target"
}

// Evaluate maps the resolver's four answers onto the three the loop understands.
//
//	resolved      satisfied — this is the place
//	ambiguous     SATISFIED, and deliberately. Several of the thing is here; that is a
//	              reason to stop and ask the person, not a reason to keep scrolling past it.
//	              A search that treated ambiguity as absence would scroll away from the
//	              answer and then report that it could not find it.
//	absent        denied — read, and not here
//	unobservable  unknown — says nothing about this place either way
func (c resolvesHere) Evaluate(w directorapi.WorldState) evaluation.Result {
	res := c.resolver.Resolve(&w, c.query)
	switch res.Status {
	case directorapi.ResolutionResolved:
		return evaluation.Satisfy(res.Target.Confidence,
			fmt.Sprintf("%s %q is here", res.Target.Role, res.Target.Label))
	case directorapi.ResolutionAmbiguous:
		return evaluation.Satisfy(0.5, res.Explanation)
	case directorapi.ResolutionUnobservable:
		return evaluation.Unknowable(res.Explanation)
	default:
		return evaluation.Deny(0.8, res.Explanation)
	}
}

// searchBudget is how many places one search may look in.
//
// Six. A page at a time through a list or a settings pane reaches the end of most of them, and
// the number is small enough that a wrong guess about where something is costs a second rather
// than a minute. It is a constant rather than a setting because nobody has asked for a different
// one, and a knob would invite somebody to set it to fifty.
const searchBudget = 6

// scrollPattern is the one pattern the Director searches with today.
//
// The move names no control. It carries a CAPABILITY — "the region that can scroll" — and the
// executor resolves it against the reading in front of it at the moment of the move, which is
// what ADR-140 built the Scrollable query for. Naming an element id here would pin the search to
// a region resolved before the first move, and a list that re-virtualises its rows would hand
// back a different id on the next look.
func scrollPattern() search.Pattern {
	region := func(d string) directorapi.ElementReference {
		yes := true
		return directorapi.ElementReference{
			Query:       &directorapi.ElementQuery{Scrollable: &yes, PreferFocused: true},
			Description: d,
		}
	}
	return search.Pattern{
		Step: search.Move{
			Action: directorapi.SemanticAction{
				Kind:   directorapi.SemanticScroll,
				Scroll: directorapi.ScrollPageDown,
				Target: region("the region that can scroll"),
			},
			Phrase: "scrolling down one page",
		},
		Home: search.Move{
			Action: directorapi.SemanticAction{
				Kind:   directorapi.SemanticScroll,
				Scroll: directorapi.ScrollTop,
				Target: region("the region that can scroll"),
			},
			Phrase: "scrolling back to the top",
		},
		Budget:   searchBudget,
		Describe: "scrolling",
	}
}

// lookElsewhere searches for the request's target and reports what came of it.
//
// Returns the reading the target was found in, or nil. The world is the SEARCH'S own — the one
// the thing was seen in — rather than a fresh observation afterwards, because the next thing the
// pipeline does is resolve against it, and a fresher reading is a different place.
func (p *Pipeline) lookElsewhere(ctx context.Context, in directorapi.Intent,
	add func(string, string, bool)) (*directorapi.WorldState, *search.Result) {

	if p.Searcher == nil || p.Resolver == nil {
		return nil, nil
	}
	if len(in.Targets) == 0 || in.Targets[0].Query == nil {
		// Nothing was named, so there is nothing to go looking for. The requests that
		// reach here are the ones whose target is a capability or the focused control,
		// and those are answered by the reading in front of Marco or not at all.
		return nil, nil
	}

	what := resolvesHere{
		resolver: p.Resolver,
		query:    *in.Targets[0].Query,
		phrase:   in.Targets[0].Phrase,
	}
	res, err := p.Searcher.Find(ctx, what, scrollPattern())
	if err != nil {
		// A refused pattern or a missing budget — a programming error here, not a
		// finding about the screen. Recorded and otherwise ignored, so the request
		// refuses with the resolver's own sentence rather than with this one.
		add("search", "could not search: "+err.Error(), false)
		return nil, nil
	}
	add("search", res.Reason, res.Outcome == search.Found)
	if res.Outcome != search.Found {
		return nil, &res
	}
	return res.World, &res
}

// searchRefusal is what a request says when a search did not find its target.
//
// AN EXHAUSTED SEARCH IS A STRONGER CLAIM THAN A FAILED LOOK, and the sentence has to be the
// stronger one — otherwise the search is work the person paid for and was never told about. The
// three negative outcomes say three different things and must not collapse:
//
//	exhausted    looked everywhere it could reach and read every place: it is not there
//	unreadable   some place could not be read, so it may still be there
//	stopped      nothing is claimed about where it is
func searchRefusal(phrase string, res *search.Result) string {
	if res == nil {
		return ""
	}
	switch res.Outcome {
	case search.Exhausted:
		return fmt.Sprintf("%s is not here, and not anywhere scrolling could reach — %s",
			describeWanted(phrase), res.Reason)
	case search.Unreadable:
		return fmt.Sprintf("%s was not found, and %s", describeWanted(phrase), res.Reason)
	}
	return ""
}

// describeWanted names the thing the person asked for. The phrase arrives already quoted from
// targetPhrase, so it is used as it comes.
func describeWanted(phrase string) string {
	if phrase == "" {
		return "what you asked for"
	}
	return phrase
}
