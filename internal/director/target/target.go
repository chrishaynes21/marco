// Package target turns a description of what the user meant into a concrete element,
// or into an honest account of why it could not.
//
// Two findings from live inspection shaped this package.
//
// ACTIONABILITY OUTRANKS EXACTNESS. Searching a real VS Code window for "Explorer"
// returns seven matches. The one whose label matches EXACTLY is a static section
// header; the control the user means is a tab that matches only on substring. Any
// ranker that leads with textual similarity picks the header, clicks nothing, and
// reports success. So a candidate's score is its textual match SCALED by what can
// actually be done to it, and an inert element is penalised heavily enough that no
// amount of label precision lifts it above a usable control.
//
// NOT FINDING SOMETHING IS TWO DIFFERENT ANSWERS. Searching a Discord window returns
// nothing because Discord exposes nothing; searching a fully-readable dialog returns
// nothing because the thing is not there. The first is a statement about the
// Director and the second about the application, they call for opposite responses,
// and collapsing them is what makes an agent confidently wrong. Resolve reports
// unobservable and absent separately, and only the second is evidence of absence.
package target

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/rephrase"
	"github.com/chaynes-simpleclouds/marco/internal/director/world"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Resolver finds the element a query refers to.
type Resolver struct {
	// AmbiguityMargin is how far the best candidate must lead the runner-up to be
	// accepted without asking. Two controls that score within this of each other are
	// a genuine ambiguity, and picking the top of a near-tie is how an agent clicks
	// the wrong "Apply" and reports success.
	AmbiguityMargin float64
	// MinScore is the score below which a candidate is not worth offering at all.
	MinScore float64
	// Discovery, when set, proposes bounded searches for targets that are hidden
	// behind something that must be opened first.
	Discovery *Discoverer
	// Rephrasing, when set, re-asks a request that matched NOTHING in words the screen
	// itself uses — "the bluetooth panel" against "Bluetooth & devices".
	//
	// Reached on ABSENT and on nothing else, which is the whole of why it is safe to put
	// here: every request this package already answers is answered by exactly the code
	// that answered it before, bit for bit, and rewording only ever speaks where the
	// answer was that nothing matched.
	// See [[ADR-218-a-request-reworded-in-the-screens-own-words]].
	Rephrasing *rephrase.Rephraser
}

// NewResolver returns a Resolver with the default thresholds and discovery enabled.
func NewResolver() *Resolver {
	return &Resolver{
		AmbiguityMargin: 0.12,
		MinScore:        0.25,
		Discovery:       NewDiscoverer(),
		Rephrasing:      rephrase.New(),
	}
}

// Resolve finds what the query refers to in the given world.
//
// The order of checks matters and is the point of the package: whether the world
// could answer the question at all is decided BEFORE whether it contains an answer.
// Asking them the other way round is what turns "I cannot see into this application"
// into "that button does not exist".
func (r *Resolver) Resolve(w *directorapi.WorldState, q directorapi.ElementQuery) directorapi.Resolution {
	return r.ResolveUnder(w, q, directorapi.OperateLicence{}, "")
}

// ResolveUnder is Resolve for a request that carries an explicit action licence.
//
// The licence reaches exactly one decision: whether a Prop that ONLY PIXELS DESCRIBE may be
// offered as a target at all. Everything else about resolution is untouched — the same ranking,
// the same ambiguity refusal, the same geometry and visibility requirements, the same
// explanations. A request without a licence resolves exactly as it always did, which is what
// makes this safe to add to a shared path.
//
// See directorapi.OperateLicence for what a licence is and the four things it cannot do.
func (r *Resolver) ResolveUnder(w *directorapi.WorldState, q directorapi.ElementQuery,
	lic directorapi.OperateLicence, verb string) directorapi.Resolution {

	return r.resolve(w, q, lic, verb, Demonstration{})
}

// resolve is the whole of resolution, told everything the request carries.
//
// One body, so the licensed, unlicensed and demonstrated entry points cannot diverge: the
// difference between them is entirely in what they pass, and there is no second ranking, second
// ambiguity rule or second explanation anywhere in this package.
//
// # Where rewording goes, and why it is only here
//
// Underneath, `literally` is resolution exactly as it has always been. This function adds one
// thing: when the literal reading found NOTHING, the request is asked again in words the screen
// uses, and each re-asking goes back through `literally`.
//
// So the first property of [[ADR-218-a-request-reworded-in-the-screens-own-words]] is not a
// promise, it is the shape of the code. There is one branch on ABSENT, in one function, and no
// other path into rewording — which is what makes "every request the resolver already answered
// is answered identically" checkable by reading rather than by testing every caller.
func (r *Resolver) resolve(w *directorapi.WorldState, q directorapi.ElementQuery,
	lic directorapi.OperateLicence, verb string, shown Demonstration) directorapi.Resolution {

	res := r.literally(w, q, lic, verb, shown)
	// A NIL RESOLVER IS A REAL CALLER, and it was tolerated here by accident before this
	// function had a second half.
	//
	// `literally` refuses a query that describes nothing before it reads any of this
	// Resolver's fields, so a Pipeline assembled without one could always reach that
	// refusal — and a test that asks the Director to interpret a sentence naming no control
	// does exactly that. Reading `r.Rephrasing` unconditionally turned that honest refusal
	// into a crash. The guard is explicit now rather than incidental.
	if r == nil || r.Rephrasing == nil {
		return res
	}
	switch res.Status {
	case directorapi.ResolutionAbsent:
		if reworded, ok := r.reword(w, q, lic, verb, shown, anyAnswer); ok {
			return reworded
		}
	case directorapi.ResolutionResolved:
		if better, ok := r.reconsider(w, q, lic, verb, shown, res); ok {
			return better
		}
	case directorapi.ResolutionAmbiguous:
		if settled, ok := r.settle(w, q, lic, verb, shown, res); ok {
			return settled
		}
	}
	return res
}

// settle answers a question the person's own sentence already answered.
//
// # The measured case
//
// Paint shows a `Zoom` slider and a `Zoom` combo box. Asked for "the zoom slider", resolution
// matched both at 0.5 — the word "zoom" is in both labels and in the request — and asked which
// was meant. The person had already said which: they said slider.
//
// [[ADR-218-a-request-reworded-in-the-screens-own-words]] permits exactly this and only this:
// a tie may be broken by evidence that DISTINGUISHES the candidates, never by a preference. A
// kind noun in the request is that evidence when it fits one contender and not the others, and
// it is nothing at all when it fits several — which is why the re-asking has to come back
// RESOLVED rather than merely shorter.
//
// # The two conditions, and what each refuses
//
// THE RE-ASKING MUST RESOLVE. Role-constrained and still ambiguous means the kind did not
// distinguish them — two sliders both called Zoom are still two sliders — and the question
// stands exactly as it was.
//
// AND IT MUST LAND ON ONE OF THE CONTENDERS. The question was about those controls, and
// answering it with a different one would be changing the subject: a lower-scoring control
// that only wins once the contenders are role-rejected was never what the person was being
// asked to choose between.
//
// That second clause is defence in depth rather than a measured need, and the mutation gate is
// the reason it is described that way: no world in either evaluation corpus produces a
// re-asking that resolves to a control the question was not about, because the reduced query is
// an exact match on the contenders' own label and nothing outscores that. Deleting it changes
// no outcome today. It stays because the day something does outscore it, the failure would be
// Marco answering a question nobody asked.
//
// Together those mean this can only ever turn one of the offered choices into the answer. It
// cannot invent a third, cannot refuse, and cannot resolve a tie that the person's words left
// genuinely tied.
func (r *Resolver) settle(w *directorapi.WorldState, q directorapi.ElementQuery,
	lic directorapi.OperateLicence, verb string, shown Demonstration,
	res directorapi.Resolution) (directorapi.Resolution, bool) {

	kinds := rephrase.KindNamed(firstNonEmpty(q.Label, q.Text))
	if len(kinds) == 0 || len(res.Contenders) < directorapi.MinContenders {
		return directorapi.Resolution{}, false
	}
	// A kind that fits every contender distinguishes nothing, and asking again would be
	// asking the same question with extra steps.
	//
	// AN EARLY-OUT RATHER THAN THE GUARANTEE, and the mutation gate said so: deleting this
	// changes no outcome, because the accept predicate below is what actually refuses to
	// settle a genuine tie. Two buttons both called Close re-resolve to an AMBIGUOUS
	// answer, and `accept` takes only a RESOLVED one. Kept because it says what the rule
	// is in the place somebody looks for it, and because it saves the work — not because
	// the rule depends on it.
	fitting := 0
	offered := map[directorapi.ElementID]bool{}
	for _, c := range res.Contenders {
		offered[c.ElementID] = true
		if roleFits(c.Role, kinds) {
			fitting++
		}
	}
	if fitting != 1 {
		return directorapi.Resolution{}, false
	}
	return r.reword(w, q, lic, verb, shown, func(got directorapi.Resolution) bool {
		return got.Status == directorapi.ResolutionResolved &&
			offered[got.Target.ElementID] && roleFits(got.Target.Role, kinds)
	})
}

// reconsider asks again when the words matched something the person's own sentence says is the
// wrong SORT of control.
//
// # The defect this exists for: a match is not an answer
//
// Measured on the Settings Bluetooth page. Asked to open "bluetooth settings", resolution
// returned the Bluetooth ON/OFF SWITCH — confidently, as a single uncontested candidate at
// 0.485 — because `textScore` scores a label CONTAINED IN the request at 0.5 and "Bluetooth" is
// contained in "bluetooth settings". Nothing was wrong with the match. What was wrong is that a
// plausible match terminated resolution, so the reading that would have answered correctly
// never ran: asked again with "settings" set aside and destinations tried first, the navigation
// item resolves and the switch is rejected as the wrong kind of control.
//
// The failure is therefore not a missing interpretation. It is that Marco could mistake a match
// for an answer, and a confident wrong answer reaches the person before anything better does.
//
// # Why the person's own noun, and nothing else
//
// Because it is evidence rather than preference, which is the line
// [[ADR-218-a-request-reworded-in-the-screens-own-words]] draws and this must not cross. "A
// switch is a poor answer to open settings" is only knowable here because the person SAID
// settings; a rule that preferred navigation items generally would be Marco's taste, and two
// controls the person did not distinguish must stay undistinguished.
//
// So this fires only when the request named a kind of control and the chosen control's role
// conflicts with it. A request that named no kind reaches none of this.
//
// # And it can only ever swap one answer for another
//
// Four things it cannot do, which together are why it is safe to put in front of every resolved
// request:
//
//   - It cannot refuse. A reconsideration that finds nothing better leaves the literal answer
//     exactly as it was, word for word and number for number.
//   - It cannot raise a new question. Only a RESOLVED re-asking is accepted.
//   - It cannot choose the same control twice. An answer that re-resolves to the control the
//     words already chose is the words' own answer, and is returned untouched rather than
//     relabelled as reworded — which is what keeps "the Storage section" (a LINK, where the
//     person said "section") bit-identical.
//   - It cannot accept a control whose role conflicts too. The replacement must FIT the kind
//     that was named, or there was no reason to prefer it.
//
// Nothing about scoring changes. MinScore is untouched and substring matching is untouched:
// what changes is whether a particular kind of match is allowed to end the search.
func (r *Resolver) reconsider(w *directorapi.WorldState, q directorapi.ElementQuery,
	lic directorapi.OperateLicence, verb string, shown Demonstration,
	res directorapi.Resolution) (directorapi.Resolution, bool) {

	if res.Target == nil {
		return directorapi.Resolution{}, false
	}
	kinds := rephrase.KindNamed(firstNonEmpty(q.Label, q.Text))
	if len(kinds) == 0 || roleFits(res.Target.Role, kinds) {
		return directorapi.Resolution{}, false
	}
	chosen := res.Target.ElementID
	return r.reword(w, q, lic, verb, shown, func(got directorapi.Resolution) bool {
		return got.Status == directorapi.ResolutionResolved &&
			got.Target.ElementID != chosen && roleFits(got.Target.Role, kinds)
	})
}

// RoleFits reports whether a control's role is one of the kinds a request named.
//
// Exported for the evaluation, which has to be able to say precisely which literal answers were
// ALLOWED to change: one whose role conflicted with the kind the person named, replaced by one
// that fits. A second copy of the rule in the harness would let the measurement and the
// behaviour drift, and the measurement is the only thing holding the behaviour.
func RoleFits(got directorapi.ElementRole, kinds []directorapi.ElementRole) bool {
	return roleFits(got, kinds)
}

// roleFits reports whether a control's role is one of the kinds the request named, allowing the
// resolver's own compatibility between near-equivalent kinds — a navigation list member IS the
// tab of an application whose navigation is a list.
func roleFits(got directorapi.ElementRole, kinds []directorapi.ElementRole) bool {
	for _, want := range kinds {
		if got == want || compatibleRole(got, want) {
			return true
		}
	}
	return false
}

// anyAnswer accepts whatever a re-asking resolved or asked, which is what an ABSENT request
// wants: anything at all is better than nothing matched.
func anyAnswer(got directorapi.Resolution) bool {
	return got.Status == directorapi.ResolutionResolved ||
		got.Status == directorapi.ResolutionAmbiguous
}

// reword re-asks the request in the screen's own words, and reports whether that answered it.
//
// # The order the attempts are taken in, which is the whole safety argument
//
// The FIRST attempt that resolves wins, and the attempts arrive strictest first — an attempt
// carrying the role the person's own kind noun named comes before the same words with no role
// at all. So the looser reading only ever answers a question the stricter one could not, and a
// kind Marco read wrongly costs a tie-break rather than a control.
//
// When nothing resolves, the first AMBIGUOUS answer is returned instead. That ordering is
// deliberate and it is the one that pays for itself: an unnecessary question is the cost we are
// trying to reduce, and acting on the wrong control is the cost we must never buy with it. Two
// operable controls that a reworded request cannot tell apart are still two controls, and the
// person is still the one who knows which.
// `accept` says which answers the caller will take, and is the only difference between the two
// callers: a request nothing matched will take anything, and a request whose match was the wrong
// SORT of control will take only a resolved, role-fitting, different one.
func (r *Resolver) reword(w *directorapi.WorldState, q directorapi.ElementQuery,
	lic directorapi.OperateLicence, verb string, shown Demonstration,
	accept func(directorapi.Resolution) bool) (directorapi.Resolution, bool) {

	var ambiguous *directorapi.Resolution
	for _, attempt := range r.Rephrasing.Attempts(w, q) {
		got := r.literally(w, attempt.Query, lic, verb, shown)
		if !accept(got) {
			continue
		}
		switch got.Status {
		case directorapi.ResolutionResolved:
			return asReworded(got, q, attempt), true
		case directorapi.ResolutionAmbiguous:
			if ambiguous == nil {
				settled := asReworded(got, q, attempt)
				ambiguous = &settled
			}
		}
	}
	if ambiguous != nil {
		return *ambiguous, true
	}
	return directorapi.Resolution{}, false
}

// asReworded marks an answer as reached by rewording and says what was set aside to reach it.
//
// Three things it is responsible for, all of them about not overstating:
//
// THE QUERY IS THE PERSON'S OWN AGAIN. A resolved target carries its query so the control can be
// found again in the next snapshot, and the query that goes with it must be the request as the
// person made it — not Marco's reading of it. Otherwise the reading becomes the record, and the
// next observation re-finds a control by words nobody said.
//
// THE CONFIDENCE IS DISCOUNTED. A reworded match is a weaker claim than a lexical one and must
// not be able to read as the same thing.
//
// AND IT IS DISCOUNTED RATHER THAN FLOORED. ADR-217 proposed that a reworded confidence be
// "lower than any lexical match", which cannot be implemented as written: the lowest lexical
// match a usable candidate can have is MinScore, and anything under that is not offered at all,
// so the literal rule would refuse every reworded answer there is. What the proposal was
// guarding against — an interpretation beating an exact match — cannot arise, because rewording
// is only ever reached where the literal reading found nothing to beat.
func asReworded(res directorapi.Resolution, asked directorapi.ElementQuery,
	attempt rephrase.Attempt) directorapi.Resolution {

	res.Reworded = attempt.Because
	if res.Reworded == "" {
		res.Reworded = "reading the request against the words on screen"
	}
	if res.Target != nil {
		query := asked
		res.Target.Query = &query
		res.Target.Confidence *= rewordedDiscount
		res.Target.Explanation += ", " + res.Reworded
	}
	res.Explanation += ", " + res.Reworded
	return res
}

// rewordedDiscount is what a reworded match's confidence is multiplied by.
//
// Enough to be visible beside a lexical score and not enough to push a real match under the
// floor: an exact label reached by setting aside the word "panel" lands at 0.78 where the same
// label said exactly reaches 0.97, and the account prints both the number and the sentence that
// produced it.
const rewordedDiscount = 0.8

// literally is resolution by the words the person used, and nothing else.
func (r *Resolver) literally(w *directorapi.WorldState, q directorapi.ElementQuery,
	lic directorapi.OperateLicence, verb string, shown Demonstration) directorapi.Resolution {

	if !q.Constrained() {
		return directorapi.Resolution{
			Status:      directorapi.ResolutionUnobservable,
			Blocker:     "unconstrained_query",
			Explanation: "the request did not describe anything specific enough to look for",
		}
	}

	candidates := r.rankUnder(w, q, lic, verb, shown)

	// A match found despite a poor world is still a match: the world being weak
	// overall does not unfind a control we can plainly see and use.
	if best := firstUsable(candidates); best != nil {
		return r.decide(w, q, candidates, shown)
	}

	// Nothing usable found. Now the question is whether this world was ever capable
	// of answering, and that is what separates "not here" from "cannot see".
	if blocker, why := unobservable(w); blocker != "" {
		return directorapi.Resolution{
			Status:      directorapi.ResolutionUnobservable,
			Candidates:  candidates,
			Blocker:     blocker,
			Explanation: why,
		}
	}

	// The world was good enough to answer, and the answer is no. Only now is a
	// discovery attempt justified, and only now would "there is no Save button" be
	// an honest thing to say.
	//
	// EXCEPT WHEN THE THING WAS FOUND AND REFUSED, which is a different sentence.
	//
	// `disabledPenalty`'s own note already says what is owed here — "a disabled Save IS the
	// thing the user meant, and reporting 'Save is greyed out' requires finding it first" — and
	// the explanation below said the opposite: it reported a control the resolver had plainly
	// matched, scored and rejected as not being there at all.
	//
	// Measured by real-desktop acceptance against a named, visible, disabled button:
	//
	//	Resolving "Delete" → ABSENT
	//	  nothing matching "Delete" is present in the observed window
	//	  considered 26:
	//	    0.68  button  Delete  REJECTED: disabled
	//
	// Both halves of that are produced here, and they contradict each other. A person told the
	// control is absent goes looking for a missing control; a person told it is disabled goes
	// looking at why their application will not let them, which is the true answer and a
	// completely different place to look.
	//
	// It stays ABSENT as a STATUS. Nothing usable was found, every caller that branches on the
	// status is right to treat it as nothing-to-act-on, and a new status would be a vocabulary
	// change for a sentence. What changes is that the sentence describes what was seen.
	res := directorapi.Resolution{
		Status:      directorapi.ResolutionAbsent,
		Candidates:  candidates,
		Explanation: fmt.Sprintf("nothing matching %s is present in the observed window", describe(q)),
	}
	if why := rejectedMatch(candidates); why != "" {
		res.Explanation = fmt.Sprintf("%s is %s, so it cannot be acted on", describe(q), why)
		return res
	}
	if r.Discovery != nil {
		if plan := r.Discovery.Plan(w, q); !plan.Empty() {
			res.Discovery = plan
			res.Explanation += "; it may be inside a menu that is not open"
		}
	}
	return res
}

// decide turns a ranked candidate list into a resolved or ambiguous outcome.
func (r *Resolver) decide(w *directorapi.WorldState, q directorapi.ElementQuery,
	candidates []directorapi.TargetCandidate,
	shown Demonstration) directorapi.Resolution {

	usable := usableOnly(candidates)
	best := usable[0]

	// An explicit ordinal ("the second tab") is a direct instruction about WHICH
	// match to take, so it overrides the ambiguity check — the user has already
	// resolved it.
	if q.Ordinal > 0 {
		if q.Ordinal > len(usable) {
			return directorapi.Resolution{
				Status:     directorapi.ResolutionAbsent,
				Candidates: candidates,
				Explanation: fmt.Sprintf("only %d matches for %s, so there is no number %d",
					len(usable), describe(q), q.Ordinal),
			}
		}
		best = usable[q.Ordinal-1]
		return resolved(w, q, best, candidates,
			fmt.Sprintf("match %d of %d for %s", q.Ordinal, len(usable), describe(q)),
			shown)
	}

	if len(usable) > 1 && usable[1].Score > best.Score-r.AmbiguityMargin {
		// The contenders are the ORDERED SET, computed once, here. Every consumer
		// offers exactly this and never re-derives it — see Resolution.Contenders for
		// what the duplicate predicate used to cost.
		contenders := within(usable, best.Score-r.AmbiguityMargin)
		if len(contenders) < directorapi.MinContenders {
			// Cannot honestly call this ambiguous: there are not two things to offer.
			// Resolving to the best is the truthful reading — nothing was close enough
			// to contest it after all.
			return resolved(w, q, best, candidates, explain(best), shown)
		}
		return directorapi.Resolution{
			Status:     directorapi.ResolutionAmbiguous,
			Candidates: candidates,
			Contenders: contenders,
			Explanation: fmt.Sprintf("%d controls match %s about equally well",
				len(contenders), describe(q)),
		}
	}
	return resolved(w, q, best, candidates, explain(best), shown)
}

func resolved(w *directorapi.WorldState, q directorapi.ElementQuery, best directorapi.TargetCandidate,
	all []directorapi.TargetCandidate, why string, shown Demonstration) directorapi.Resolution {

	el, _ := w.Element(best.ElementID)
	query := q
	target := &directorapi.ResolvedTarget{
		ElementID:   best.ElementID,
		Confidence:  best.Score,
		Explanation: why,
		Role:        best.Role,
		Label:       best.Label,
		// The query is kept, not just the id. An id identifies the element in THIS
		// snapshot; the query is what lets it be found again in the next one, which
		// is the whole difference between repeating an action semantically and
		// replaying a coordinate.
		Query: &query,
	}
	if el != nil {
		target.WindowID = el.WindowID
		target.Point = el.ClickPoint()
		// AND WHERE THE PRESS GOES, WHEN A DEMONSTRATION SAID SOMEWHERE ELSE.
		//
		// `ResolvedTarget` has always separated identity from position, and this is the
		// first thing to use the separation: the words IDENTIFY the control and the
		// surface a person demonstrated LOCATES it. The regrounder one tier down says
		// the same sentence about the same problem.
		//
		// A PROP'S OWN MIDDLE, never a remembered number. The surface is named by id
		// and looked up in THIS world, so the press is computed from a live observation
		// of the thing that is there now — and a surface the demonstration named that is
		// no longer on screen contributes nothing rather than a stale coordinate.
		if shown.Covers(best.ElementID) && shown.Surface != "" &&
			shown.Surface != best.ElementID {
			if sel, ok := w.Element(shown.Surface); ok && !sel.Bounds.Empty() {
				target.Surface = shown.Surface
				target.Point = sel.ClickPoint()
			}
		}
		target.Alternatives = all[1:]
		if n, ok := el.Attributes["native_id"].(string); ok {
			target.NativeID = n
		}
		// WHAT THE CONTROL WAS LIKE, carried forward with the choice. The capability
		// ladder needs the patterns and the state, and this is the one place that has
		// them — a second lookup later would be a second observation of a world that
		// had already moved. See directorapi.ControlFacts for what went wrong without
		// them.
		target.Control = directorapi.FactsOf(el)
	}
	return directorapi.Resolution{
		Status:      directorapi.ResolutionResolved,
		Target:      target,
		Candidates:  all,
		Explanation: why,
	}
}

// unobservable reports whether the world was incapable of answering, and why.
//
// Each condition is a distinct way of not being able to see, and they are named
// separately because the right recovery differs: a shallow world needs another
// source, a stale one needs re-observing, a degraded one needs the failed provider.
func unobservable(w *directorapi.WorldState) (blocker, why string) {
	c := w.Confidence
	switch {
	case len(w.Elements) == 0 && len(w.Degraded) > 0:
		return "degraded_source", "no source reported, so nothing could be looked for"
	case c.Shallow():
		return "coverage", "the application exposed only containers and no content, " +
			"so its interior was never visible — this is not evidence the target is absent"
	case c.Blind():
		return "actionability", "nothing in this window can be operated, " +
			"so no control could be matched"
	case len(w.Degraded) > 0:
		return "degraded_source", "a source that was expected did not report, " +
			"so the window was only partly observed"
	}
	return "", ""
}

// rank scores every element against the query, best first.
func (r *Resolver) rank(w *directorapi.WorldState, q directorapi.ElementQuery) []directorapi.TargetCandidate {
	return r.rankUnder(w, q, directorapi.OperateLicence{}, "", Demonstration{})
}

// rankUnder is rank, told what the request is licensed to operate and what was demonstrated.
func (r *Resolver) rankUnder(w *directorapi.WorldState, q directorapi.ElementQuery,
	lic directorapi.OperateLicence, verb string,
	shown Demonstration) []directorapi.TargetCandidate {

	var out []directorapi.TargetCandidate

	for _, el := range world.Elements(w) {
		if !inScope(w, el, q) {
			continue
		}
		c := score(el, q, lic, verb, shown)
		if c.Score < r.MinScore && c.Rejected == "" {
			continue
		}
		out = append(out, c)
	}

	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Score != out[b].Score {
			return out[a].Score > out[b].Score
		}
		return out[a].ElementID < out[b].ElementID
	})
	return out
}

// inScope applies the query's hard filters — the constraints that are not evidence
// to be weighed but conditions to be met.
func inScope(w *directorapi.WorldState, el *directorapi.Element, q directorapi.ElementQuery) bool {
	if q.Window != nil && el.WindowID != *q.Window {
		return false
	}
	// Default scope is the active window: the user is almost always talking about
	// what is in front of them.
	if q.Window == nil && !q.AnyWindow && w.ActiveWindow != nil && el.WindowID != *w.ActiveWindow {
		return false
	}
	if q.Application != "" {
		if win, ok := w.Window(el.WindowID); !ok || !strings.EqualFold(win.Application, q.Application) {
			return false
		}
	}
	if q.Within != nil && !q.Within.ContainsRect(el.Bounds) {
		return false
	}
	if q.Enabled != nil && el.Enabled != *q.Enabled {
		return false
	}
	if q.Visible != nil && el.Visible != *q.Visible {
		return false
	}
	if q.Focused != nil && el.Focused != *q.Focused {
		return false
	}
	if q.Scrollable != nil && el.Actions().Scrollable != *q.Scrollable {
		return false
	}
	if q.Editable != nil && el.Actions().Settable != *q.Editable {
		return false
	}
	if q.NativeID != "" && nativeIDOf(el) != q.NativeID {
		// Exact, and the only query field that is. A derived control — an inline editor
		// the previous step opened — cannot be named any other way: two controls in the
		// window contain the same text, and only one of them is the editor.
		return false
	}
	return true
}

// nativeIDOf reads a source's own identifier off an element.
func nativeIDOf(el *directorapi.Element) string {
	v, _ := el.Attributes["native_id"].(string)
	return v
}

// Scoring weights. Text match is the largest single signal, but it is a factor in a
// product rather than a term in a sum — which is what stops a perfect label match on
// an inert element from outscoring a good match on a usable one.
const (
	// inertPenalty is what a non-interactive element's score is multiplied by.
	//
	// Sized from the VS Code case: an exact-match inert text (text 1.0) must land
	// below a substring-match usable control (text ~0.6). 0.35 puts the header at
	// 0.35 and the tab at ~0.6, which is a clear and stable ordering rather than a
	// coin flip. Inert matches are kept as candidates rather than dropped, because
	// "the only thing called Save here is a label" is a useful thing to be able to
	// say.
	inertPenalty = 0.35
	// unreachablePenalty applies to an interactive element with nowhere to aim —
	// scrolled out of view, or reported without bounds.
	unreachablePenalty = 0.5
	// disabledPenalty applies to a control that exists but will not respond. It is
	// mild on purpose: a disabled Save IS the thing the user meant, and reporting
	// "Save is greyed out" requires finding it first.
	disabledPenalty = 0.7
)

// score rates one element against the query and records why.
func score(el *directorapi.Element, q directorapi.ElementQuery,
	lic directorapi.OperateLicence, verb string,
	shown Demonstration) directorapi.TargetCandidate {
	c := directorapi.TargetCandidate{
		ElementID: el.ID,
		Role:      el.Role,
		Label:     el.Label,
		Sources:   el.Sources,
		Signals:   map[string]float64{},
	}

	text := textScore(el, q)
	if text <= 0 {
		c.Rejected = "label does not match"
		return c
	}
	c.Signals["text"] = text

	s := text

	// THE ONE YOU ARE IN, preferred rather than required. See ElementQuery.PreferFocused.
	if q.PreferFocused && el.Focused {
		c.Signals["focused"] = 1
		s *= 1.5
	}

	// Role, when asked for. A request that names a kind of control means it.
	if q.Role != "" {
		if el.Role == q.Role {
			c.Signals["role"] = 1
		} else if compatibleRole(el.Role, q.Role) {
			c.Signals["role"] = 0.6
			s *= 0.85
		} else {
			c.Rejected = "wrong kind of control"
			c.Score = 0
			return c
		}
	}

	// Actionability, as a MULTIPLIER. This is the ordering rule the live findings
	// forced: what can be done to a candidate gates how good a match it is, rather
	// than merely adding to it.
	acts := el.Actions()
	// ASKED-FOR CAPABILITY IS NOT INERTNESS.
	//
	// inertPenalty is for scenery: a heading or a static label that matches the words and
	// cannot be operated. A scrollable CONTAINER is inert by role for the same reason a list
	// is — you operate its items, not it — and that is the right answer for a click and the
	// wrong one for a scroll, which operates the container itself.
	//
	// Measured: the WinForms fixture's 200-row list scored 0.6 on a capability query, took the
	// penalty for being a container, and fell under MinScore — so a request to scroll the only
	// scrollable thing in the window resolved to nothing at all, with no rejected candidate to
	// explain it.
	//
	// Narrow on purpose: the exemption applies only when the query ASKED for the capability
	// this element has. A scrollable pane that merely matches somebody's words is still
	// scenery for a click.
	asked := q.Scrollable != nil && *q.Scrollable && acts.Scrollable
	switch {
	case !acts.Interactive && !asked:
		s *= inertPenalty
		c.Signals["inert"] = inertPenalty
	case !acts.ClickableByBounds:
		s *= unreachablePenalty
		c.Signals["unreachable"] = unreachablePenalty
	}
	if acts.Interactive && !acts.Enabled {
		s *= disabledPenalty
		c.Signals["disabled"] = disabledPenalty
	}

	// Evidence quality: a well-corroborated element is a safer target than a guess.
	s *= 0.7 + 0.3*el.Confidence
	c.Signals["evidence"] = el.Confidence

	// Proximity, when the request implies a place ("the one on the left", or near
	// where the user is pointing).
	if q.Near != nil {
		p := proximity(el.Bounds, *q.Near)
		c.Signals["near"] = p
		s *= 0.8 + 0.2*p
	}

	c.Score = s
	// GROUNDING AND PERMISSION, ASKED APART.
	//
	// `Targetable()` is the conjunction of both, and reading it as one question is what
	// made a pixel-only control report "not visible on screen" — a control that was
	// plainly visible, correctly matched, and merely unvouched-for. A person told that
	// goes looking for a missing button.
	//
	// So grounding is asked first and answers for itself, and the permission question —
	// did anything say this may be OPERATED — is asked second, where an explicit
	// instruction may answer it. See directorapi.OperateLicence.
	operable, why := el.OperableUnder(lic, verb)
	// AND WHETHER THE INTERFACE PRESENTS ANYTHING OPERABLE HERE, which is the third
	// question and the one a proposed rectangle cannot answer for itself.
	//
	// `Any()` is derived from ROLE, so a Prop the shape proposer found answers no however
	// well it is observed and however exactly the words match. Experiment-057 measured that
	// as the only thing refusing a correctly matched, correctly licensed menu entry.
	//
	// A person pressing that rectangle, and the screen then changing, is evidence about
	// exactly this question — the interface itself answering it, which is better evidence
	// than a role string. So a demonstration may answer this clause and NO OTHER: enabled,
	// reachable and ADR-202's licence are all still required, and all still asked above.
	//
	// It cannot be inferred from inside here; see demonstrated.go. Deleting this must fail
	// TestADemonstratedSurfaceResolvesAndNothingElseDoes.
	affords := acts.Any()
	if !affords && shown.Covers(el.ID) {
		affords = true
		c.Signals["demonstrated"] = 1
		c.Demonstrated = shown.Because
	}
	if !(acts.Enabled && acts.ClickableByBounds && operable && affords) {
		// Recorded, not dropped: an unusable match is a real finding, and it is
		// what lets the Director say "Save is disabled" instead of "Save does not
		// exist".
		switch {
		case !acts.Interactive:
			c.Rejected = "not an interactive control"
		case !acts.Enabled:
			c.Rejected = "disabled"
		case !acts.ClickableByBounds:
			c.Rejected = "not visible on screen"
		default:
			c.Rejected = why
		}
	}
	return c
}

// textScore rates how well an element's text matches what was asked for, 0..1.
//
// A query naming several acceptable labels — the localized aliases of one semantic role
// — scores as the BEST of them rather than as a conjunction: they are alternative names
// for the same control, and only one of them is on this machine's screen.
func textScore(el *directorapi.Element, q directorapi.ElementQuery) float64 {
	if len(q.AnyLabel) > 0 {
		best := 0.0
		for _, alias := range q.AnyLabel {
			single := q
			single.AnyLabel, single.Label, single.Text = nil, alias, ""
			if s := textScore(el, single); s > best {
				best = s
			}
		}
		return best
	}

	if q.ExactLabel {
		// All or nothing. A near match here is not a weaker answer to the same
		// question, it is a confident answer to a different one.
		want := normalise(firstNonEmpty(q.Label, q.Text))
		if want != "" && normalise(el.Label) == want {
			return 1.0
		}
		return 0
	}

	want := normalise(firstNonEmpty(q.Label, q.Text))
	if want == "" {
		// A query with no text at all (role plus ordinal, say) does not discriminate
		// on text, so every element passes this signal neutrally.
		return 0.6
	}

	best := 0.0
	for _, field := range []struct {
		value  string
		weight float64
	}{
		{el.Label, 1.0},
		{el.Value, 0.75},
		// Description is a weak signal: it is often a tooltip repeating the label,
		// and occasionally the only text a bare icon has.
		{el.Description, 0.6},
	} {
		got := normalise(field.value)
		if got == "" {
			continue
		}
		var s float64
		switch {
		case got == want:
			s = 1.0
		case strings.HasPrefix(got, want):
			// "Explorer (Ctrl+Shift+E)" for "Explorer": the accelerator hint that
			// toolkits append is not a worse match, it is the same control.
			s = 0.85
		case strings.Contains(got, want):
			s = 0.6
		case strings.Contains(want, got):
			s = 0.5
		default:
			s = wordOverlap(got, want)
		}
		if s *= field.weight; s > best {
			best = s
		}
	}
	return best
}

// wordOverlap is the share of the wanted words present in the candidate — the
// fallback for a multi-word request phrased differently ("save file" vs "save the
// file as").
func wordOverlap(got, want string) float64 {
	wantWords := strings.Fields(want)
	if len(wantWords) < 2 {
		return 0
	}
	gotWords := map[string]bool{}
	for _, w := range strings.Fields(got) {
		gotWords[w] = true
	}
	hits := 0
	for _, w := range wantWords {
		if gotWords[w] {
			hits++
		}
	}
	share := float64(hits) / float64(len(wantWords))
	if share < 0.6 {
		return 0
	}
	return 0.4 * share
}

// proximity is 1 at the point and falls to 0 at proximityRange.
func proximity(r directorapi.Rect, p directorapi.Point) float64 {
	if r.Empty() {
		return 0
	}
	c := r.Center()
	dx, dy := float64(c.X-p.X), float64(c.Y-p.Y)
	d := dx*dx + dy*dy
	const proximityRange = 600.0
	if d >= proximityRange*proximityRange {
		return 0
	}
	return 1 - sqrt(d)/proximityRange
}

func sqrt(f float64) float64 {
	if f <= 0 {
		return 0
	}
	// Newton's method: enough for a ranking prior, and keeps this package free of
	// any dependency beyond the standard library's string handling.
	x := f
	for range 20 {
		x = 0.5 * (x + f/x)
	}
	return x
}

// compatibleRole reports whether an element's role is close enough to a requested
// one. Users do not use a toolkit's vocabulary: "the Save button" is a reasonable
// way to refer to a menu item that saves.
func compatibleRole(got, want directorapi.ElementRole) bool {
	if got == want {
		return true
	}
	pairs := [][2]directorapi.ElementRole{
		{directorapi.RoleButton, directorapi.RoleMenuItem},
		{directorapi.RoleButton, directorapi.RoleIcon},
		{directorapi.RoleButton, directorapi.RoleToggle},
		{directorapi.RoleButton, directorapi.RoleLink},
		{directorapi.RoleTab, directorapi.RoleListItem},
		{directorapi.RoleListItem, directorapi.RoleRow},
		{directorapi.RoleCheckbox, directorapi.RoleToggle},
	}
	for _, p := range pairs {
		if (got == p[0] && want == p[1]) || (got == p[1] && want == p[0]) {
			return true
		}
	}
	return false
}

// firstUsable returns the best candidate that can actually be acted on.
func firstUsable(candidates []directorapi.TargetCandidate) *directorapi.TargetCandidate {
	for i := range candidates {
		if candidates[i].Rejected == "" && candidates[i].Score > 0 {
			return &candidates[i]
		}
	}
	return nil
}

func usableOnly(candidates []directorapi.TargetCandidate) []directorapi.TargetCandidate {
	out := make([]directorapi.TargetCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.Rejected == "" && c.Score > 0 {
			out = append(out, c)
		}
	}
	return out
}

// within returns the usable candidates that scored above the floor, in rank order.
//
// A PREFIX of the ranked list, never a re-ordering or a de-duplication. That is what
// keeps "the second one" meaning the same thing to the user and to the resolver.
func within(candidates []directorapi.TargetCandidate, floor float64) []directorapi.TargetCandidate {
	out := make([]directorapi.TargetCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.Score > floor {
			out = append(out, c)
		}
	}
	return out
}

func explain(c directorapi.TargetCandidate) string {
	var reasons []string
	if s := c.Signals["text"]; s >= 1 {
		reasons = append(reasons, "exact label match")
	} else if s >= 0.8 {
		reasons = append(reasons, "label matches allowing for a keyboard hint")
	} else if s > 0 {
		reasons = append(reasons, "label contains the requested text")
	}
	if _, inert := c.Signals["inert"]; !inert {
		reasons = append(reasons, "an operable "+string(c.Role))
	}
	if len(c.Sources) > 1 {
		reasons = append(reasons, "corroborated by several sources")
	}
	return fmt.Sprintf("%q: %s", c.Label, strings.Join(reasons, ", "))
}

func describe(q directorapi.ElementQuery) string {
	if t := firstNonEmpty(q.Label, q.Text); t != "" {
		return fmt.Sprintf("%q", t)
	}
	if q.Role != "" {
		return "a " + string(q.Role)
	}
	return "that"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// normalise matches the normalisation fusion and identity use, so a label decorated
// by one toolkit and not another still compares equal.
func normalise(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "&", "")
	s = strings.TrimSuffix(s, "...")
	s = strings.TrimSuffix(s, "…")
	s = strings.TrimSuffix(s, ":")
	return strings.TrimSpace(s)
}

// Rank exposes candidate scoring for callers that need EVERY match rather than one.
//
// A collection wants the whole ranked set: its question is "which of these qualify?"
// rather than "which one did they mean?". Sharing the scoring is what guarantees a
// collection can never include something a singular request would consider out of
// scope — a second implementation would drift, and the divergence would only surface
// when a bulk action hit the wrong control.
func (r *Resolver) Rank(w *directorapi.WorldState, q directorapi.ElementQuery) []directorapi.TargetCandidate {
	return r.rank(w, q)
}

// rejectedMatch names why a control the resolver FOUND cannot be acted on, or "".
//
// # Which rejections count, and why not all of them
//
// Only the ones that are facts about the CONTROL: it is disabled, it is not on screen, it is not
// interactive. Each of those means "the thing you named is there and will not respond", which is a
// true and useful sentence.
//
// "label does not match" is excluded, and that exclusion is the whole correctness of this function:
// every element in the window carries it, so counting it would turn "nothing matching Save is
// present" into "Save's label does not match" about a window with no Save in it at all.
//
// "wrong kind of control" is excluded for a narrower reason: it fires when a query asked for a role
// and something else wore the label, and the honest answer there is still that what was asked for
// is not present.
//
// The BEST-SCORING such candidate wins, because the resolver has already ranked them and the one it
// ranked first is the one it would have acted on. Ties are broken by the order the ranking produced,
// which is deterministic.
func rejectedMatch(candidates []directorapi.TargetCandidate) string {
	// A fact about the control rather than about the match.
	aboutTheControl := map[string]bool{
		"disabled":                   true,
		"not visible on screen":      true,
		"not an interactive control": true,
	}
	for _, c := range candidates {
		if c.Rejected == "" || !aboutTheControl[c.Rejected] {
			continue
		}
		return c.Rejected
	}
	return ""
}
