package observe

// The structural evidence a screen is segmented from, and where it came from.
//
// # Why this type exists
//
// It did not, and that was the defect. Screen segmentation read `Sample.Shadow.Regions` —
// the EXPERIMENTAL detector's output — and nothing else. So with the detector switched off,
// which is the default because it costs ~1.25 GB resident, an application producing six
// hundred fused accessibility elements per sample produced zero screens, and therefore no
// recognition, no relationships, no learning, no rehearsal and no plays.
//
// The evidence was there the whole time. It was in `Sample.Entities`, in the same
// window-relative normalised geometry and the same role vocabulary segmentation wants, and
// there was simply no path from it to the segmenter.
//
// # What a screen is segmented from
//
// A bounded set of (role, window-relative region) pairs. That is the whole contract —
// `ScreenSignature` reads nothing else. It does not want labels, text, identities, absolute
// coordinates or a provider's name, and it must not: a screen is a composition, and a
// composition is the same composition whichever admissible source described it.
//
// # Why provenance travels with it
//
// Because "observed and structurally empty" and "not observed" are different facts, and
// segmentation must treat them differently. An empty signature from a source that LOOKED is
// the sparse-gameplay screen, a real state that recurs. An empty signature from a source
// that did not look is unknown, and minting a state from it would invent a screen out of
// silence — see [[ADR-006-unknown-is-not-false]].
//
// The old code encoded exactly that rule as `Ran && TargetProven && Unavailable == ""` on
// the shadow sample. It is the same rule; it now lives where provenance is decided rather
// than where segmentation happens, so a second source cannot arrive without one.

// StructuralSource says which kind of evidence a screen composition was read from.
//
// A CLOSED vocabulary of two, and deliberately not a provider name. Segmentation does not
// branch on it and must not: it exists so a diagnostic can say where the structure came
// from and so `StructureOf` can express a preference between them once, in one place.
type StructuralSource string

const (
	// StructureUnobserved: nothing looked, so nothing is known about this frame's
	// composition. NOT the same as an empty screen.
	StructureUnobserved StructuralSource = ""
	// StructureFused: the authoritative world — every admissible provider, already merged
	// and de-duplicated by fusion, already through the privacy classifier.
	StructureFused StructuralSource = "fused"
	// StructureDetector: the structural detector alone, on a surface fusion could not see.
	StructureDetector StructuralSource = "detector"
)

// StructuralView is one inference's structural evidence, with its provenance.
type StructuralView struct {
	Source StructuralSource `json:"source,omitempty"`
	// Regions is the composition. Empty is legitimate when Source is not unobserved: an
	// application can genuinely present nothing structural, and that is a screen.
	Regions []ShadowRegion `json:"regions,omitempty"`
	// Why explains an unobserved view in words a person can act on, empty otherwise.
	//
	// Carried because the four reasons a frame has no structure send a reader to four
	// different places, and a segmenter that reported only "no screens" makes them guess.
	Why string `json:"why,omitempty"`
}

// Observed reports whether anything looked at this frame's composition.
//
// The gate segmentation gets to make a state from. False means unknown, and unknown mints
// nothing.
func (v StructuralView) Observed() bool { return v.Source != StructureUnobserved }

// StructureOf chooses the structural evidence one sample offers to segmentation.
//
// # The precedence, and why it is this way round
//
// FUSED FIRST. The fused world is the authoritative composition: every provider that could
// prove its target contributed to it, fusion merged the ones describing the same thing, and
// the privacy classifier has already run. Segmenting from it means a screen is a fact about
// what the Director BELIEVES is on screen.
//
// THE DETECTOR SECOND, and only when fusion saw no structure at all. That is precisely the
// surface the detector exists for — a game with no accessibility tree — and it is the case
// [[ADR-017-structure-earns-a-name-text-never-earns-structure]] was written about.
//
// # Why not the union
//
// Because the detector is an EXPERIMENT. Its evidence is deliberately kept out of fusion so
// it cannot influence belief, and a screen identity is belief-adjacent: it decides what a
// track is measured against, what a relationship connects, and what a play says it begins
// on. Merging unfused detector boxes into an authoritative composition would let the
// experiment move all of that — the exact influence the shadow boundary exists to prevent —
// and would double-count every control both sources found.
//
// So the two are alternatives, not addends. Corroboration between them is measured
// separately, by ShadowComparison, which is where a comparison belongs.
func StructureOf(s Sample) StructuralView {
	if v := s.Structure; v.Observed() {
		if len(v.Regions) > 0 || s.Shadow == nil {
			return v
		}
		// Fusion looked and found no structure, and the detector may have. Falling through
		// is not "preferring the detector" — it is the one case where the authoritative
		// answer is silence and something else has a real answer.
		if d := s.Shadow.Structure(); d.Observed() && len(d.Regions) > 0 {
			return d
		}
		return v
	}
	if s.Shadow != nil {
		return s.Shadow.Structure()
	}
	return StructuralView{Why: "nothing observed this frame's composition"}
}

// Structure is what the structural detector proved about this frame's composition.
//
// The gate is the one segmentation always applied, moved here unchanged: a slot the cadence
// declined, a failed pass, and evidence that cannot be shown to describe the pinned window
// are all UNKNOWN. In particular an unproven target is refused rather than downgraded —
// boxes from the window that was there before would segment the wrong screen, confidently.
// The three conditions are the ORIGINAL gate, unchanged and in the original order. The
// detector's NAME is deliberately not one of them: a record that ran and proved its target
// describes a real composition whether or not anything labelled it, and adding a fourth
// refusal here would be this milestone quietly narrowing the rule it set out to widen.
func (s *ShadowSample) Structure() StructuralView {
	switch {
	case s == nil:
		return StructuralView{Why: "no structural detector is configured"}
	case s.Unavailable != "":
		return StructuralView{Why: detectorName(s) + " could not run: " + s.Unavailable}
	case !s.Ran:
		if s.Detector == "" {
			return StructuralView{Why: "no structural detector is configured"}
		}
		return StructuralView{Why: s.Detector + " sat this frame out"}
	case !s.TargetProven:
		return StructuralView{
			Why: detectorName(s) + " could not prove it looked at the right window"}
	}
	return StructuralView{Source: StructureDetector, Regions: s.Regions}
}

func detectorName(s *ShadowSample) string {
	if s.Detector == "" {
		return "the structural detector"
	}
	return s.Detector
}
