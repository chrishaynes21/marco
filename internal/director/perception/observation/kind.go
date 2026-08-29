package observation

import "github.com/chaynes-simpleclouds/marco/pkg/directorapi"

// WHO SAID WHAT THIS IS.
//
// # The question, and why provenance is the wrong shape for it
//
// `Provenance.OnlyDescribesPixels` answers "did anything but a camera report this object". That
// is the right question for actionability — nothing has claimed a mechanism — and the wrong one
// for identity, because an object can be reported by accessibility and still have its KIND
// supplied by a detector.
//
// Measured, on one Windows Settings page with the visual detector configured:
//
//	without the detector    115 elements   … unknown x1
//	with the detector       136 elements   … icon x21 (vision alone)
//	                                          icon x1  (accessibility + vision)
//
// Twenty-one of those are pixels and nothing else. The twenty-second is an element
// accessibility DID report, as `unknown` — which fusion treats as no claim at all, correctly,
// so the first specific claim wins and a detector named it `icon`. Both are "this thing's kind
// is known only from pixels", and only the first is `OnlyDescribesPixels`.
//
// That one element is not a rounding error. `unknown` is a layout role and `icon` is not, so it
// alone moves the identity role SET, and a rule that caught the twenty-one and missed it would
// leave two durable Places for one screen exactly as before.

// StructuralKindClaims is the set of observations that made a real claim about what something
// is, from a source that can describe more than pixels.
//
// Keyed by observation id so a fused element can be asked about its own provenance rather than
// re-matched by geometry — the ids are already on the element, put there by fusion, and
// re-deriving the association here would be a second answer to "which evidence is this made of".
//
// Not "which sources are structural": an accessibility node reporting `unknown` is a structural
// source that described the object and did not say what it was, and counting it as a claim would
// re-admit exactly the case above.
func StructuralKindClaims(obs []Observation) map[directorapi.ObservationID]bool {
	out := make(map[directorapi.ObservationID]bool, len(obs))
	for _, o := range obs {
		if !directorapi.ActuatingSource(o.Source()) {
			continue
		}
		el, ok := o.(Element)
		if !ok {
			continue
		}
		if el.Raw.Role.Generic() {
			continue
		}
		out[o.ID()] = true
	}
	return out
}

// KindOf says who accounted for what this element IS: nothing but pixels, a detector naming
// something structural evidence merely reported, or a structural source itself.
//
// Three answers rather than two, because two of them are not the same and the difference was
// measured. On one Settings page with the detector running there were twenty-one detections
// nothing structural had reported AND one element accessibility DID report, as `unknown`, which
// the detector then named `icon`. A rule with one bit keeps both — leaving a detector's own
// boxes in a screen's identity — or drops both, which deletes an element the application really
// has the moment a sensor names it.
//
// # Absence is not evidence
//
// An element with no provenance at all is `described`, deliberately, and for the same reason as
// `Provenance.OnlyDescribesPixels`. Elements are constructed as well as observed — a fixture, a
// capability pack's enrichment, a hand-built query — and treating "nobody wrote down where this
// came from" as "only a camera saw it" would quietly empty every composition built that way.
//
// # A generic role was nobody's claim
//
// An element whose fused role is generic has not been named by anything, so no detector overrode
// anybody. The first version of this asked only whether a structural source had made a claim,
// and therefore answered "named by pixels" for every accessibility text node on the screen —
// live, that removed `text x29` and `unknown x1` from one page. Accessibility described those
// elements and said they were text: a poorer claim than `button`, and not the same as no account
// at all.
func KindOf(role directorapi.ElementRole, p directorapi.Provenance,
	claims map[directorapi.ObservationID]bool) directorapi.KindEvidence {

	if p.Len() == 0 {
		return directorapi.KindDescribed
	}
	if p.OnlyDescribesPixels() {
		return directorapi.KindPixelOnly
	}
	if role.Generic() {
		return directorapi.KindDescribed
	}
	for _, ref := range p.Sources {
		if claims[ref.Observation] {
			return directorapi.KindDescribed
		}
	}
	return directorapi.KindPixelNamed
}
