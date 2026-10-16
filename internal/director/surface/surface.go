// Package surface asks whether an explicitly requested click has a defensible place to land.
//
// # The question, and why it is Director's rather than perception's
//
// [[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]] put proposed rectangles on
// the Stage claiming nothing: `RoleUnknown`, no label, the shape proposer's own evidence. So a
// game menu went from zero Props to 228, and nothing became clickable — correctly, because a
// rectangle is not a control.
//
// [[Experiment-054-the-surface-a-named-prop-does-not-carry]] then measured what that leaves. OCR
// reads SETTINGS; fusion gives the word to the smallest region containing it; the 444x56 menu row
// is also a Prop and is also a candidate for where a person would press. Identity and interaction
// surface are different things that happen to be the same object when accessibility reports a
// button, and come apart when nothing does.
//
// Deciding between them is not a perception problem. The pixels are not ambiguous — both regions
// are really there. What is missing is a REASON to prefer one, and assembling reasons from
// separate pieces of evidence is Director's work.
//
// # What this package may and may not conclude
//
// It is READ-ONLY and it executes nothing. It produces a hypothesis: the identity, the candidate
// surfaces, the evidence for and against each, a decision, and a sentence. The hypothesis is
// request-scoped and disposable — no role is assigned, no Prop is modified, no screen identity or
// workflow is written, and nothing here is persisted or consulted again.
//
// Three decisions, and the middle one is not a failure:
//
//	one defensible surface   exactly one candidate survives every test
//	ambiguous                two or more do, and nothing here chooses between them
//	insufficient evidence    none does, or the request was never identified
//
// AMBIGUOUS is the answer that makes Marco teachable. "I found Settings and I cannot tell which
// of these two surfaces is the control" is a question a person answers in one click, and it is a
// far better thing to say than a confident press on the wrong rectangle.
//
// # The tests a candidate has to survive, and where each comes from
//
// Nothing here is a new threshold, and nothing is fitted to an example:
//
//   - IT MUST CONTAIN THE IDENTITY, by `fusion.Owners` — the same rule, the same constants and
//     the same ordering fusion used when it decided which Prop the word belonged to. A surface
//     Director nominates and fusion would have refused is a surface nobody can explain.
//   - SOMETHING MUST HAVE DESCRIBED THE OBJECT, by `Provenance.DescribesAnObject` —
//     [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]]'s own rule.
//     A read word locates a glyph run; it is not a claim that a thing is there.
//   - IT MUST LOCATE SOMETHING, by `observe.Region.Locates` — the workflow stack's rule that a
//     band across the window has a centre which is the window's centre and so carries no
//     information about where anything in it sits.
//   - IT MUST BE THE ONLY NAMED THING INSIDE IT. This is the test that does the most work and it
//     is a consequence of the others rather than a new idea: if a candidate contains Props the
//     request did not name, then pressing its middle is pressing a region that stands for several
//     things at once, and which of them responds is not something the evidence says. A toolbar
//     band contains every control on the toolbar. A 48%-of-window background contains most of the
//     screen. A menu row contains its own entry.
//
// And the rest is reported as EVIDENCE and decides nothing on its own — the proposer's `fill`,
// `parts` and shape class, the area share of the window, and how many same-shaped siblings the
// candidate has. [[Experiment-056-whether-a-row-is-one-of-a-set]] measured that sibling structure
// separates a menu row from a background cleanly and leaves the Explorer toolbar unresolved, which
// is exactly why it is written down and not acted upon.
package surface

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Decision is what the evidence supports.
type Decision string

const (
	// DecisionDefensible: exactly one candidate survived, and the hypothesis names it.
	DecisionDefensible Decision = "one defensible surface"
	// DecisionAmbiguous: more than one survived. Nothing here chooses, on purpose.
	DecisionAmbiguous Decision = "two or more defensible surfaces"
	// DecisionInsufficient: none survived, or the request was never identified at all.
	DecisionInsufficient Decision = "insufficient evidence"
)

// Identity is the Prop whose words matched the request, and how well.
type Identity struct {
	ElementID directorapi.ElementID `json:"element_id"`
	Label     string                `json:"label,omitempty"`
	Role      directorapi.ElementRole
	Bounds    directorapi.Rect `json:"bounds"`
	// Score and Signals are the resolver's own ranking of this match, carried rather than
	// recomputed: the question "which Prop did these words mean" already has an owner.
	Score   float64            `json:"score"`
	Signals map[string]float64 `json:"signals,omitempty"`
	// Sources is which sources accounted for it.
	Sources []directorapi.ObservationSource `json:"sources,omitempty"`
	// Rejected is why the resolver would not act on it, which for a pixel-only Prop with a
	// generic role is the whole reason this package exists.
	Rejected string `json:"rejected,omitempty"`
}

// Evidence is everything known about one candidate surface. Every field is reported; only the
// four named in the package note decide anything.
type Evidence struct {
	// Containment is how much of the identity's region lies inside this candidate, by
	// fusion's rule.
	Containment float64 `json:"containment"`
	// Ancestor is whether the identity's own region is plainly inside this one rather than
	// beside it.
	Ancestor bool `json:"ancestor"`
	// IsIdentity is whether this candidate IS the Prop the words named, rather than
	// something around it.
	IsIdentity bool `json:"is_identity"`

	// DescribesAnObject is whether any source other than read text accounted for it.
	DescribesAnObject bool `json:"describes_an_object"`
	// Sources is which ones.
	Sources []directorapi.ObservationSource `json:"sources,omitempty"`
	// StructureKind, Fill and Parts are the shape proposer's own evidence where it was the
	// source. Reported, never decisive: Experiment-054 measured fill and parts separating a
	// background cleanly and failing on a toolbar band.
	StructureKind string  `json:"structure_kind,omitempty"`
	Fill          float64 `json:"fill,omitempty"`
	Parts         int     `json:"parts,omitempty"`

	// Locates is whether the candidate's middle says where anything in the window is.
	Locates bool   `json:"locates"`
	WhyNot  string `json:"locates_why_not,omitempty"`
	// AreaShare is the fraction of the window this candidate covers.
	AreaShare float64 `json:"area_share"`

	// OtherNamed is the Props with their own names that lie wholly inside this candidate,
	// other than the identity. A surface standing for several named things is a surface whose
	// press lands on something the evidence cannot name.
	OtherNamed []Named `json:"other_named,omitempty"`

	// Siblings is how many Props share this candidate's shape and column, and DistinctTexts
	// how many of that set carry text differing from the others'. Reported as the structural
	// evidence Experiment-056 measured, and decisive nowhere.
	Siblings      int `json:"siblings"`
	DistinctTexts int `json:"distinct_texts"`

	// CompetingIdentities is the other Props whose words also match the request, with their
	// scores. More than one close match is an identity problem rather than a surface one.
	CompetingIdentities []Named `json:"competing_identities,omitempty"`
}

// Named is a Prop referred to by name in an explanation.
type Named struct {
	ElementID directorapi.ElementID `json:"element_id"`
	Label     string                `json:"label,omitempty"`
	Bounds    directorapi.Rect      `json:"bounds"`
	Score     float64               `json:"score,omitempty"`
}

// Candidate is one possible place to press.
type Candidate struct {
	ElementID directorapi.ElementID `json:"element_id"`
	Bounds    directorapi.Rect      `json:"bounds"`
	// Point is where a press would land IF this candidate were chosen. Always the
	// candidate's own click point: this package never invents a position inside a rectangle,
	// because the evidence that would choose one is the evidence that is missing.
	Point    directorapi.Point `json:"point"`
	Evidence Evidence          `json:"evidence"`
	// Rejected is why this candidate is not defensible, empty when it is.
	Rejected string `json:"rejected,omitempty"`
}

// Defensible reports whether this candidate survived every test.
func (c Candidate) Defensible() bool { return c.Rejected == "" }

// Hypothesis is one request's reasoning about where to press, start to finish.
//
// Disposable by construction: a value returned from one call, naming ids that belong to one
// reading of the screen. Nothing stores it and there is nowhere for it to be stored.
type Hypothesis struct {
	Request  string    `json:"request"`
	Verb     string    `json:"verb"`
	Identity *Identity `json:"identity,omitempty"`
	// Candidates is every surface considered, best container first, including the rejected
	// with the reason. The losers are the explanation.
	Candidates []Candidate `json:"candidates,omitempty"`
	// Window is the rectangle every candidate above is stated in.
	//
	// Carried because a surface means nothing without it: the area share is a fraction OF
	// this, the band rule is about running the width of THIS, and a press arrives
	// window-relative and has to be placed back into the space the candidates are in. A
	// caller that had to ask the world again for it could ask a different world.
	Window directorapi.Rect `json:"window"`
	// Chosen is the one defensible surface, set only when Decision says there is one.
	Chosen      *Candidate `json:"chosen,omitempty"`
	Decision    Decision   `json:"decision"`
	Explanation string     `json:"explanation"`
	// Unsettled is the question a demonstration would answer, when the evidence cannot.
	// Empty when nothing was left open.
	//
	// NOT CALLED "teachable", although that is the word it wants. TEACH is reserved for the
	// mirror image — Marco guiding a person through something they do themselves — and this
	// is the other direction entirely: a person acting while Marco watches, which is LEARN.
	// `cmd/marco`'s TestNoLiveAcquisitionCodeIsNamedTeach holds that line and caught this.
	Unsettled string `json:"unsettled,omitempty"`
}

// Evaluator asks the question. It holds no state between calls.
type Evaluator struct {
	// Resolver answers "which Prop did these words mean". Shared with execution on purpose:
	// an identity this package chose differently from the resolver would be an identity the
	// rest of the system does not agree about.
	Resolver *target.Resolver
}

// New returns an Evaluator using the production resolver and its thresholds.
func New() *Evaluator { return &Evaluator{Resolver: target.NewResolver()} }

// Evaluate reasons about one request against one reading of the screen, and acts on nothing.
//
// `verb` is the action the request named, as the parser normalised it. The licence is built here
// from the request exactly as `execute.Pipeline` builds it, and used for exactly one thing: to
// ask the resolver for its ranking of a Prop that only pixels describe. It authorises nothing,
// because nothing is done.
func (e *Evaluator) Evaluate(w *directorapi.WorldState, request, verb string,
	q directorapi.ElementQuery) Hypothesis {

	h := Hypothesis{Request: request, Verb: verb, Decision: DecisionInsufficient}
	if w == nil || len(w.Elements) == 0 {
		h.Explanation = "nothing was observed in this window, so there is nothing to " +
			"press and nothing to identify"
		return h
	}

	res := e.resolver().ResolveUnder(w, q,
		directorapi.OperateLicence{Instruction: request, Verb: verb}, verb)

	// ── WHO THE REQUEST MEANT ──
	//
	// Taken from the resolver's own candidate list rather than matched again here. A rejected
	// candidate is still an identity: the whole case this package exists for is a Prop the
	// resolver correctly matched and correctly refused to act on.
	matches := textMatches(res.Candidates, e.resolver().MinScore)
	if len(matches) == 0 {
		h.Explanation = fmt.Sprintf("nothing in this window is named like %q, so there "+
			"is no target to find a surface for", describe(q))
		return h
	}
	best := matches[0]
	el, ok := w.Element(best.ElementID)
	if !ok {
		h.Explanation = "the best match is not in this reading of the screen"
		return h
	}
	h.Identity = &Identity{
		ElementID: best.ElementID, Label: best.Label, Role: best.Role,
		Bounds: el.Bounds, Score: best.Score, Signals: best.Signals,
		Sources: best.Sources, Rejected: best.Rejected,
	}

	// AND WHETHER THAT WAS EVEN SETTLED. Two Props matching the words about equally well is
	// an IDENTITY problem, and no amount of surface reasoning repairs it — the resolver's own
	// margin is the authority, so the same request refuses here and there for one reason.
	competing := within(matches[1:], best.Score-e.resolver().AmbiguityMargin)
	if len(competing) > 0 {
		h.Decision = DecisionAmbiguous
		h.Explanation = fmt.Sprintf("%d Props are named like %q about equally well, so "+
			"which one was meant is not settled and no surface follows from it",
			len(competing)+1, describe(q))
		h.Unsettled = "which of these the request meant"
		for _, c := range competing {
			if cel, ok := w.Element(c.ElementID); ok {
				h.Candidates = append(h.Candidates, Candidate{
					ElementID: c.ElementID, Bounds: cel.Bounds,
					Point:    cel.ClickPoint(),
					Rejected: "a competing identity, not a surface",
					Evidence: Evidence{CompetingIdentities: []Named{{
						ElementID: best.ElementID, Label: best.Label,
						Bounds: el.Bounds, Score: best.Score}}},
				})
			}
		}
		return h
	}

	// ── AND WHERE A PRESS COULD LAND ──
	ordered := elementsOf(w)
	owners := fusion.Owners(ordered, el.Bounds)
	if len(owners) == 0 {
		// The identity's own region always contains itself, so this means the Prop has no
		// usable rectangle at all.
		h.Explanation = "the Prop named by this request has no rectangle that contains it"
		return h
	}

	// FUSION'S OWN AMBIGUITY RULE FIRST. Two comparably sized surfaces both containing the
	// identity are peers rather than nested, and fusion refuses to choose between them for
	// the same reason this does.
	peer, peered := fusion.Rivals(owners)

	window, _ := w.Window(el.WindowID)
	h.Window = window.Bounds
	for _, o := range owners {
		h.Candidates = append(h.Candidates, e.judge(w, ordered, o, el, window.Bounds, q))
	}

	var good []int
	for i, c := range h.Candidates {
		if c.Defensible() {
			good = append(good, i)
		}
	}

	switch {
	case peered && len(good) > 1:
		h.Decision = DecisionAmbiguous
		h.Explanation = fmt.Sprintf("%q sits inside %d surfaces of comparable size — "+
			"%s and %s — which fusion's own rule calls peers rather than one inside "+
			"the other, so nothing here chooses between them",
			best.Label, len(good), h.Candidates[good[0]].ElementID, peer.Element.ID)
		h.Unsettled = "which surface is the control"
	case len(good) == 0:
		h.Decision = DecisionInsufficient
		h.Explanation = fmt.Sprintf("%d surface(s) contain %q and none of them is "+
			"defensible: %s", len(h.Candidates), best.Label,
			h.Candidates[0].Rejected)
	case len(good) == 1:
		h.Decision = DecisionDefensible
		h.Chosen = &h.Candidates[good[0]]
		h.Explanation = fmt.Sprintf("%q is named by %s, and %s is the only surface "+
			"containing it that %s", best.Label, sourceList(h.Identity.Sources),
			h.Chosen.ElementID, whySurvived(h.Chosen.Evidence))
		// AND WHICH OF THE TWO THINGS HAPPENED, said rather than left to be assumed.
		//
		// When the survivor is the identity's own region, the press lands on the region
		// the words were read on. That may be the control — a toolbar button is about the
		// size of its label — or it may be a caption, and
		// [[ADR-201-a-demonstrated-click-remembers-what-it-clicked]] is a record of the
		// second: pressing the middle of a word because the word matched put the click
		// 17 px from the edge of the tile it labelled. The regrounder one tier down says
		// which of the two it did for the same reason.
		if h.Chosen.Evidence.IsIdentity {
			h.Explanation += ". The press would land on the region the words " +
				"themselves were read on, which may be the control or may be " +
				"its caption"
			if len(h.Candidates) > 1 {
				h.Explanation += fmt.Sprintf("; %d surface(s) around it were "+
					"considered and refused", len(h.Candidates)-1)
			}
			h.Unsettled = "whether the words are the control or its caption"
		}
	default:
		h.Decision = DecisionAmbiguous
		names := make([]string, 0, len(good))
		for _, i := range good {
			names = append(names, string(h.Candidates[i].ElementID))
		}
		h.Explanation = fmt.Sprintf("%d surfaces containing %q are equally defensible "+
			"on this evidence (%s), and nothing here chooses between them",
			len(good), best.Label, strings.Join(names, ", "))
		h.Unsettled = "which surface is the control"
	}
	return h
}

// judge assembles the evidence for one candidate and says whether it survives.
func (e *Evaluator) judge(w *directorapi.WorldState, ordered []*directorapi.Element,
	o fusion.Owner, identity *directorapi.Element, window directorapi.Rect,
	q directorapi.ElementQuery) Candidate {

	el := o.Element
	c := Candidate{ElementID: el.ID, Bounds: el.Bounds, Point: el.ClickPoint()}
	ev := Evidence{
		Containment: o.Containment, Ancestor: o.Ancestor,
		IsIdentity:        el.ID == identity.ID,
		DescribesAnObject: el.Provenance.DescribesAnObject(),
		Sources:           sourcesOf(el),
	}
	if s, ok := el.Attributes["structure_kind"].(string); ok {
		ev.StructureKind = s
	}
	if f, ok := el.Attributes["structure_fill"].(float64); ok {
		ev.Fill = f
	}
	if p, ok := el.Attributes["structure_parts"].(int); ok {
		ev.Parts = p
	}
	if window.Area() > 0 {
		ev.AreaShare = float64(el.Bounds.Area()) / float64(window.Area())
	}
	ev.Locates, ev.WhyNot = locates(el.Bounds, window)
	ev.OtherNamed = otherNamed(ordered, el, identity)
	ev.Siblings, ev.DistinctTexts = siblings(ordered, el)
	c.Evidence = ev

	// ── AND THE FOUR TESTS, in the order that explains best ──
	switch {
	case ev.IsIdentity && !o.Ancestor && onlyRegionIsTheWords(el):
		// The words' own region, and nothing else reported an object there. ADR-202's
		// sentence, because it is the same finding one tier down.
		c.Rejected = "only read text describes it, which locates a word and not the " +
			"control it is written on"
	case !ev.DescribesAnObject:
		c.Rejected = "nothing but read text accounted for this region"
	case !ev.Locates:
		c.Rejected = ev.WhyNot
	case len(ev.OtherNamed) > 0:
		c.Rejected = fmt.Sprintf("it also contains %s, so its middle stands for "+
			"several named things at once and which of them a press reaches is "+
			"not something this evidence says", nameList(ev.OtherNamed))
	}
	return c
}

// onlyRegionIsTheWords reports that read text is the only account of this region.
func onlyRegionIsTheWords(el *directorapi.Element) bool {
	if len(el.Provenance.Sources) == 0 {
		return false
	}
	for _, s := range el.Provenance.Sources {
		if s.Source != directorapi.SourceOCR {
			return false
		}
	}
	return true
}

// locates asks the workflow stack's own question of a DESKTOP rectangle.
//
// The conversion is the only thing here: `observe.Region` is normalised against the window, which
// is the space that subsystem speaks, and the rule itself is `Region.Locates` unchanged.
//
// # The origin, and why leaving it out could not fail on any fixture
//
// A Prop's bounds are desktop coordinates — `providers/structure` puts the proposer's
// frame-local rectangles through the capture's own transform, like every other source on the
// window. So normalising means subtracting the window's own origin and then dividing, which is
// exactly what `observe.relativeTo` does and what this did not.
//
// Dividing without subtracting inflates X and Y by the window's position, and `Region.Locates`
// asks where a rectangle's edges are: `X <= 0.005 && X+Width >= 0.995`. On a window at the
// desktop origin the two arithmetics agree exactly, and every recorded fixture states its window
// as `Rect{Width, Height}` — so the band rule was measured only where the bug cannot show. Off
// the origin the left edge no longer looks like the left edge and a band across the window stops
// being recognised as one, which is the rule the owner named as the test of readiness.
//
// Held by `TestABandIsStillABandWhenTheWindowIsNotAtTheOrigin`.
func locates(r, window directorapi.Rect) (bool, string) {
	if window.Width <= 0 || window.Height <= 0 {
		return true, ""
	}
	return observe.Region{
		X:      float64(r.X-window.X) / float64(window.Width),
		Y:      float64(r.Y-window.Y) / float64(window.Height),
		Width:  float64(r.Width) / float64(window.Width),
		Height: float64(r.Height) / float64(window.Height),
	}.Locates()
}

// otherNamed is the named Props wholly inside `el`, other than the identity and its ancestors.
//
// Wholly inside, and named: a Prop with no label stands for nothing a request could have meant,
// and a Prop only partly overlapping is beside this one rather than in it.
func otherNamed(ordered []*directorapi.Element, el, identity *directorapi.Element) []Named {
	var out []Named
	for _, other := range ordered {
		if other == nil || other.ID == el.ID || other.ID == identity.ID ||
			other.Label == "" {
			continue
		}
		if !el.Bounds.ContainsRect(other.Bounds) {
			continue
		}
		// AND NOT THE IDENTITY WEARING ANOTHER ID. A Prop containing the identity is an
		// ancestor of it, not a second thing inside this surface.
		if other.Bounds.ContainsRect(identity.Bounds) {
			continue
		}
		out = append(out, Named{ElementID: other.ID, Label: other.Label,
			Bounds: other.Bounds})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ElementID < out[j].ElementID })
	return out
}

// siblings counts the Props sharing this one's shape and column, and how many of that set carry
// text differing from the others'.
//
// Reported as evidence and decisive nowhere. The tolerances are one obvious reading of "the same
// row shape in the same column" and [[Experiment-056-whether-a-row-is-one-of-a-set]] measured
// both what they separate and what they do not; promoting them to a rule needs more than five
// recorded interfaces.
func siblings(ordered []*directorapi.Element, el *directorapi.Element) (int, int) {
	r := el.Bounds
	if r.Width <= 0 || r.Height <= 0 {
		return 0, 0
	}
	set := []*directorapi.Element{el}
	for _, other := range ordered {
		if other == nil || other.ID == el.ID {
			continue
		}
		b := other.Bounds
		switch {
		case b.Width <= 0 || b.Height <= 0:
		case abs(b.Height-r.Height)*10 > r.Height: // not the same height
		case abs(b.X-r.X)*10 > r.Width: // not the same left edge
		case abs(b.Width-r.Width)*4 > r.Width: // not a comparable width
		case b.Y < r.Y+r.Height && r.Y < b.Y+b.Height: // not a separate row
		default:
			set = append(set, other)
		}
	}
	// AND HOW MANY OF THE SET CARRY THEIR OWN CONTENT. A row's own label is usually empty —
	// fusion gives the words to the smaller region inside it — so the names INSIDE each
	// member are what says whether this is a set of distinct things or the same thing
	// repeated. Measured in [[Experiment-056-whether-a-row-is-one-of-a-set]]: Explorer has
	// four same-shaped regions at a perfect pitch carrying ONE distinct reading between
	// them, and Windows Settings has nine carrying nine.
	seen := map[string]bool{}
	for _, m := range set {
		var words []string
		if m.Label != "" {
			words = append(words, strings.ToLower(m.Label))
		}
		for _, inner := range ordered {
			if inner == nil || inner.ID == m.ID || inner.Label == "" {
				continue
			}
			if m.Bounds.ContainsRect(inner.Bounds) {
				words = append(words, strings.ToLower(inner.Label))
			}
		}
		if len(words) == 0 {
			continue
		}
		sort.Strings(words)
		seen[strings.Join(words, "|")] = true
	}
	return len(set) - 1, len(seen)
}

// ── small helpers ─────────────────────────────────────────────────────────────

func (e *Evaluator) resolver() *target.Resolver {
	if e.Resolver == nil {
		e.Resolver = target.NewResolver()
	}
	return e.Resolver
}

// textMatches is the resolver's candidates that matched on words well enough to be offered,
// best first.
//
// The resolver already dropped everything whose label does not match and recorded the rest with
// its reason, so this is a filter rather than a second matcher. `floor` is the resolver's own
// `MinScore` and not a new threshold, and it is load-bearing: the resolver keeps a sub-MinScore
// candidate when it has a REJECTION to report, which every pixel-only Prop does, so without this
// the list includes matches the resolver itself would never offer.
//
// Measured on the recorded Windows Settings page: "click pointer" matched a Prop OCR had labelled
// "te" — because `textScore` rates a label CONTAINED IN the request at 0.5, and "te" is inside
// "pointer" — scoring 0.174 against a MinScore of 0.25. It produced a confident "one defensible
// surface" naming a two-letter fragment. The resolver's threshold was the right answer and this
// was not asking it.
func textMatches(all []directorapi.TargetCandidate, floor float64) []directorapi.TargetCandidate {
	var out []directorapi.TargetCandidate
	for _, c := range all {
		if c.Signals["text"] > 0 && c.Score >= floor {
			out = append(out, c)
		}
	}
	return out
}

func within(cands []directorapi.TargetCandidate, floor float64) []directorapi.TargetCandidate {
	var out []directorapi.TargetCandidate
	for _, c := range cands {
		if c.Score > floor {
			out = append(out, c)
		}
	}
	return out
}

// elementsOf is the world's Props in a fixed order, because a map's iteration order would make
// the owner tie-break unstable and the tie-break is the difference between a row and its panel.
func elementsOf(w *directorapi.WorldState) []*directorapi.Element {
	out := make([]*directorapi.Element, 0, len(w.Elements))
	for _, el := range w.Elements {
		if el != nil {
			out = append(out, el)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func sourcesOf(el *directorapi.Element) []directorapi.ObservationSource {
	var out []directorapi.ObservationSource
	for _, s := range el.Provenance.Sources {
		out = append(out, s.Source)
	}
	return out
}

func sourceList(ss []directorapi.ObservationSource) string {
	if len(ss) == 0 {
		return "nothing that recorded where it came from"
	}
	// DISTINCT, in the order they first appear: an element's provenance can hold two OCR
	// references and "structure and ocr and ocr" is a sentence about the bookkeeping.
	seen := map[directorapi.ObservationSource]bool{}
	parts := make([]string, 0, len(ss))
	for _, s := range ss {
		if seen[s] {
			continue
		}
		seen[s] = true
		parts = append(parts, string(s))
	}
	return strings.Join(parts, " and ")
}

func nameList(ns []Named) string {
	parts := make([]string, 0, len(ns))
	for i, n := range ns {
		if i == 3 {
			parts = append(parts, fmt.Sprintf("and %d more", len(ns)-3))
			break
		}
		parts = append(parts, fmt.Sprintf("%q", n.Label))
	}
	return strings.Join(parts, ", ")
}

func whySurvived(ev Evidence) string {
	what := fmt.Sprintf("%s describes as an object, that stands for nothing else named, "+
		"and whose middle is a position in the window rather than the window's own",
		sourceList(ev.Sources))
	if ev.Siblings > 0 {
		what += fmt.Sprintf(" (and %d Prop(s) share its shape and column, %d of the set "+
			"carrying distinct text)", ev.Siblings, ev.DistinctTexts)
	}
	return what
}

func describe(q directorapi.ElementQuery) string {
	for _, s := range []string{q.Label, q.Text} {
		if s != "" {
			return s
		}
	}
	if len(q.AnyLabel) > 0 {
		return q.AnyLabel[0]
	}
	return "that"
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
