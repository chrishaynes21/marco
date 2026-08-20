package verify

import (
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Evidence a source outside the Director can contribute.
//
//	Verification must remain semantic. Evidence: craft queue changed, inventory
//	changed, item count increased. Not: pixels moved.
//
// The Director's own evidence is everything it can judge without knowing what the
// application IS: focus moved, a window appeared, the title changed, an element's value is
// what was typed. That covers a great deal and stops exactly where application meaning
// begins — "the craft queue gained an entry" is a fact about a craft queue, and the
// Director has no idea what one is.
//
// So a capability pack contributes evidence SOURCES, and this is the whole of that seam.
// Three properties make it safe:
//
//   - Evidence is ADDITIVE. A source appends to what the Director found; it cannot remove
//     the Director's own evidence, and there is no return value that would let it try.
//   - Evidence is WEIGHED, not obeyed. A contributed weight goes into the same sum as
//     everything else and is capped, so a pack cannot declare its own action verified by
//     asserting a large enough number.
//   - A source sees the two WORLDS and nothing else. It cannot act, cannot observe, and
//     cannot reach a host — it reads what was already perceived and says what it makes of
//     it.

// EvidenceSource contributes verification evidence the Director could not produce itself.
//
// Called after the Director's own evidence has been gathered, with the same action, target
// and pair of worlds. Returning nothing is the ordinary case and means "this action is not
// one I know how to judge" — which must not be confused with "it failed".
type EvidenceSource interface {
	// Name identifies the source in diagnostics, so a reader can tell which pack
	// contributed a piece of evidence.
	Name() string
	// Evidence is what this source makes of the change. Never negative: a source that
	// believes an action did NOT happen says so by contributing nothing, because absence
	// of evidence is what the Director already weighs correctly.
	Evidence(action directorapi.Action, target directorapi.ResolvedTarget,
		before, after directorapi.WorldState) []directorapi.Evidence
}

// MaxContributedWeight caps what one contributed piece of evidence can be worth.
//
// Below the Director's own strongest signals (focus landing on the clicked element is
// 0.85) and comfortably above its weakest, so contributed evidence can carry a verdict
// when the Director has nothing — which is the case it exists for — without being able to
// carry one on its own against the Director's own reading.
const MaxContributedWeight = 0.7

// contributed gathers evidence from the registered sources, capped and attributed.
func (v *Verifier) contributed(action directorapi.Action, target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) []directorapi.Evidence {

	var out []directorapi.Evidence
	for _, src := range v.Sources {
		if src == nil {
			continue
		}
		for _, e := range src.Evidence(action, target, before, after) {
			// Capped here rather than trusted. A pack is contributed code with an
			// interest in its own actions succeeding, and the one number it could use to
			// override the Director's judgement is this one.
			if e.Weight > MaxContributedWeight {
				e.Weight = MaxContributedWeight
			}
			if e.Weight < 0 {
				e.Weight = 0
			}
			// Attribution, always. Evidence whose origin a reader cannot see is evidence
			// they cannot weigh, and "the craft queue changed" means something different
			// coming from a pack than from the Director.
			if e.Source == "" {
				e.Source = directorapi.ObservationSource(src.Name())
			}
			out = append(out, e)
		}
	}
	return out
}

// withContributed appends contributed evidence to a verdict and re-decides it.
//
// Re-deciding rather than adjusting: the verdict is a function of the evidence, and a
// result that kept its original Success while gaining evidence that changes the sum would
// be two answers to one question.
func (v *Verifier) withContributed(res directorapi.VerificationResult,
	action directorapi.Action, target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) directorapi.VerificationResult {

	extra := v.contributed(action, target, before, after)
	if len(extra) == 0 {
		return res
	}
	fail := res.Reason
	if res.Success {
		// The failure sentence to fall back on if the combined evidence somehow does not
		// carry. It cannot happen — evidence only adds — but a reason field that said
		// "verified" on a failed verdict would be worse than a slightly redundant one.
		fail = "the contributed evidence did not establish what happened"
	}
	out := conclude(append(res.Evidence, extra...), v.MinConfidence, fail)
	// An INCONCLUSIVE verdict that contributed evidence did not rescue stays
	// inconclusive. Letting it become a failure would be the worst possible use of this
	// seam: "I could not tell" would turn into "it did not happen" because a pack looked
	// and also could not tell, and the caller would retry an action that may have landed.
	if !out.Success && res.Inconclusive {
		out.Inconclusive = true
		out.Reason = res.Reason
	}
	return out
}
