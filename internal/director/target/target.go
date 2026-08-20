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
}

// NewResolver returns a Resolver with the default thresholds and discovery enabled.
func NewResolver() *Resolver {
	return &Resolver{
		AmbiguityMargin: 0.12,
		MinScore:        0.25,
		Discovery:       NewDiscoverer(),
	}
}

// Resolve finds what the query refers to in the given world.
//
// The order of checks matters and is the point of the package: whether the world
// could answer the question at all is decided BEFORE whether it contains an answer.
// Asking them the other way round is what turns "I cannot see into this application"
// into "that button does not exist".
func (r *Resolver) Resolve(w *directorapi.WorldState, q directorapi.ElementQuery) directorapi.Resolution {
	if !q.Constrained() {
		return directorapi.Resolution{
			Status:      directorapi.ResolutionUnobservable,
			Blocker:     "unconstrained_query",
			Explanation: "the request did not describe anything specific enough to look for",
		}
	}

	candidates := r.rank(w, q)

	// A match found despite a poor world is still a match: the world being weak
	// overall does not unfind a control we can plainly see and use.
	if best := firstUsable(candidates); best != nil {
		return r.decide(w, q, candidates)
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
	res := directorapi.Resolution{
		Status:      directorapi.ResolutionAbsent,
		Candidates:  candidates,
		Explanation: fmt.Sprintf("nothing matching %s is present in the observed window", describe(q)),
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
	candidates []directorapi.TargetCandidate) directorapi.Resolution {

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
			fmt.Sprintf("match %d of %d for %s", q.Ordinal, len(usable), describe(q)))
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
			return resolved(w, q, best, candidates, explain(best))
		}
		return directorapi.Resolution{
			Status:     directorapi.ResolutionAmbiguous,
			Candidates: candidates,
			Contenders: contenders,
			Explanation: fmt.Sprintf("%d controls match %s about equally well",
				len(contenders), describe(q)),
		}
	}
	return resolved(w, q, best, candidates, explain(best))
}

func resolved(w *directorapi.WorldState, q directorapi.ElementQuery, best directorapi.TargetCandidate,
	all []directorapi.TargetCandidate, why string) directorapi.Resolution {

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
		target.Alternatives = all[1:]
		if n, ok := el.Attributes["native_id"].(string); ok {
			target.NativeID = n
		}
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
	var out []directorapi.TargetCandidate

	for _, el := range world.Elements(w) {
		if !inScope(w, el, q) {
			continue
		}
		c := score(el, q)
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
func score(el *directorapi.Element, q directorapi.ElementQuery) directorapi.TargetCandidate {
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
	switch {
	case !acts.Interactive:
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
	if !acts.Targetable() {
		// Recorded, not dropped: an unusable match is a real finding, and it is what
		// lets the Director say "Save is disabled" instead of "Save does not exist".
		switch {
		case !acts.Interactive:
			c.Rejected = "not an interactive control"
		case !acts.Enabled:
			c.Rejected = "disabled"
		default:
			c.Rejected = "not visible on screen"
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
