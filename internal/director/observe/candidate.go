package observe

// A proposal and the evidence for it.
//
// # Why a rectangle needed to learn where it came from
//
// Until now this whole path dealt in bare rectangles. One source proposed them — the pixel
// proposer — so there was nothing to record and nowhere to record it. [[Experiment-030-asking-the-learned-detector-for-the-tile]] ended
// that: the vendored learned detector proposes Rocket League's TRAINING tile at IoU 0.928 where
// the proposer offers nothing at all, and it does so 600ms after the frame it describes.
//
// Two sources, arriving at two speeds, is the whole reason this type exists. A candidate now
// answers three questions a bare rectangle could not:
//
//	who says so          — From, and Class/Score when the source has them
//	about which look     — FromLook, which is NOT the look it arrives on
//	in which context     — Generation and Epoch, which decide whether it is still admissible
//
// # What it must not become
//
// A candidate is a PROPOSAL. It is not a control, not an identity and not a licence.
// `ClickTargets.Press` still takes the smallest rectangle containing a press, by area, with no
// reference to its source — a detector candidate wins or loses exactly as a pixel one does.
// Class is carried so a press can be explained afterwards and is read by NOTHING that decides;
// see TestNoDecisionReadsTheDetectorsClass, which walks for it rather than trusting discipline.
//
// [[ADR-184-two-speeds-of-looking]]

import (
	"sort"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Candidate is one proposed region, with the provenance that places it.
type Candidate struct {
	// Region is the rectangle, normalised to the window, exactly as before.
	Region
	// From is the cue that proposed it, in the Director's own vocabulary rather than a
	// second one invented here.
	From directorapi.ObservationSource `json:"from,omitempty"`
	// Class is the detector's word for this — "image", "button", "menu". CARRIED AND
	// NEVER READ. The tile that unblocked this work is classed `image`, which is correct
	// and is not a claim that it is a control; the control is decided by the press.
	Class string `json:"class,omitempty"`
	// Score is the source's own confidence, or 0 for a proposer that has none. Not a
	// threshold here: the source applied its own before emitting.
	Score float64 `json:"score,omitempty"`
	// FromLook is the look whose pixels produced this. For a pixel proposal it is the
	// look it arrives on; for a detection it is about ten looks earlier, which is the
	// fact every admission rule in fastobserve exists to handle.
	FromLook int `json:"from_look,omitempty"`
	// Generation is the window generation the originating frame was captured under.
	Generation uint64 `json:"generation,omitempty"`
	// Epoch is the observation generation that was current when the frame was captured —
	// a counter that moves whenever the standing screen changes. An arrangement can
	// recur; an epoch cannot, which is what makes a delayed result verifiable rather
	// than merely plausible.
	Epoch uint64 `json:"epoch,omitempty"`
}

// SourceStructure is the pixel proposer. Named here rather than in the shared ladder because
// this is the only path that distinguishes it from the learned detector, and a rung on the
// Director's priority ladder would be a claim about their relative strength that nothing has
// measured — Experiment-030 has the detector far ahead on a game tile, and
// `providers/vision/structureshapes.go` has the proposer ahead on a Trackmania list.
//
// Neither can confer actionability, which is the property that actually matters and which
// `directorapi.ActuatingSource` already denies to both.
const SourceStructure directorapi.ObservationSource = "structure"

// Proposed wraps a look's pixel proposals as candidates.
//
// The ordinary case, and the one that must stay cheap: one allocation and no decisions.
func Proposed(regions []Region, look int, generation, epoch uint64) []Candidate {
	if len(regions) == 0 {
		return nil
	}
	out := make([]Candidate, 0, len(regions))
	for _, r := range regions {
		out = append(out, Candidate{
			Region: r, From: SourceStructure,
			FromLook: look, Generation: generation, Epoch: epoch,
		})
	}
	return out
}

// Regions strips the provenance back off, for the parts of this subsystem that are PURELY
// geometric and should stay that way.
//
// Reacquisition is the important one. Finding a demonstrated control again is a question about
// rectangles, and it must give the same answer whichever cue proposed the rectangle — otherwise
// a target learned from one source could only ever be found again by that source, which is a
// dependency on inference timing dressed up as a match.
func Regions(cands []Candidate) []Region {
	if len(cands) == 0 {
		return nil
	}
	out := make([]Region, 0, len(cands))
	for _, c := range cands {
		out = append(out, c.Region)
	}
	return out
}

// Detected is how a slow source hands back what it found.
//
// The rectangles are normalised to the window already, because the source captured the frame
// and this package never sees pixels.
func Detected(regions []Region, classes []string, scores []float64,
	look int, generation, epoch uint64) []Candidate {

	out := make([]Candidate, 0, len(regions))
	for i, r := range regions {
		c := Candidate{
			Region: r, From: directorapi.SourceVision,
			FromLook: look, Generation: generation, Epoch: epoch,
		}
		if i < len(classes) {
			c.Class = classes[i]
		}
		if i < len(scores) {
			c.Score = scores[i]
		}
		out = append(out, c)
	}
	return out
}

// Admissible reports whether a delayed candidate is still evidence about the observation in
// front of us.
//
// THE ONE STATEMENT OF THE RULE. The observer applies it when folding a look; the offline
// replay applies it when deciding which candidates a reacquisition may search. Two copies would
// eventually disagree, and the shape of that disagreement is a measurement which says the
// integration rejects stale results while the thing being measured quietly accepted one.
//
// A pixel proposal passes trivially: it is stamped with the look it was made on, in the context
// it was made in.
func Admissible(c Candidate, generation, epoch uint64) bool {
	return c.Generation == generation && c.Epoch == epoch
}

// Merge combines a look's own proposals with admitted slow candidates, keeping one copy of a
// rectangle both sources proposed.
//
// # Why duplicates are collapsed at all
//
// Because `Press` counts candidates to decide ambiguity. Two sources agreeing about one
// rectangle is the BEST case available here, and keeping both would turn it into `Ambiguous` —
// an agreement reported as a doubt. Sameness is `sameRect`, mutual centre containment, the
// threshold-free relation the rest of this package already means by "two readings of one
// control".
//
// # Why the SMALLER survives, which is not the obvious choice
//
// The obvious rule is to keep the pixel copy, so a stored reading never depends on whether
// inference landed. A test written with the real rectangles from `rl-freeplay-1` found what
// that costs. `sameRect` is deliberately loose — it asks only that each rectangle contains the
// other's centre — so a 778x230 slab of stadium crowd and the 360x226 tile inside it can be
// "the same control" while differing 2.2x in area. On the measured frame they are not; move
// either by a few pixels and they are.
//
// Under a keep-the-pixel rule that near miss deletes the detector's tile as a duplicate of the
// slab, and the slab goes on to win the press — silently, with the admission counters all
// reporting success. The capability this whole integration exists to add would vanish on a
// rounding error.
//
// So the more specific reading survives. It is the one `Press` would have chosen anyway had
// both been offered, which makes this a tie-break and not a new ranking rule; and where the two
// genuinely agree the rectangles differ by a pixel and it does not matter which is kept.
func Merge(pixels, slow []Candidate) []Candidate {
	if len(slow) == 0 {
		return pixels
	}
	out := make([]Candidate, 0, len(pixels)+len(slow))
	out = append(out, pixels...)
	for _, s := range slow {
		dup := -1
		for i, p := range out {
			if sameRect(p.Region, s.Region) {
				dup = i
				break
			}
		}
		switch {
		case dup < 0:
			out = append(out, s)
		case s.Width*s.Height < out[dup].Width*out[dup].Height:
			out[dup] = s
		}
	}
	// Stable order regardless of arrival, so one recording replays the same way twice.
	sort.SliceStable(out, func(i, j int) bool {
		ai, aj := out[i].Width*out[i].Height, out[j].Width*out[j].Height
		if ai != aj {
			return ai < aj
		}
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].X < out[j].X
	})
	return out
}
