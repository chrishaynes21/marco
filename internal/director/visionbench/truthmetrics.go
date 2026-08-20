package visionbench

import (
	"image"
	"sort"
)

// Measuring a detector against what is declared to be there.
//
// # The defect this replaces
//
// Version 1's temporal metric rewarded PERSISTENCE and called it trust. A rectangle that
// stayed put across frames was treated as probably real. On a corpus of unrelated crops that
// degenerated into "two rectangles fell in the same coarse ninth", and the score ended up
// anti-correlated with precision — measured, not suspected: filtering 75% of the false
// positives out made the score DROP 55.2 → 50.9.
//
// The distinction version 2 turns on:
//
//	temporal persistence    the same box keeps appearing
//	temporal CORRECTNESS    the same box keeps appearing AND is really there
//
// A false rectangle that persists for a hundred frames is still false, and TemporalPrecision
// is the number that says so.

// MatchIoU is the overlap at which a detection is credited to a ground-truth region.
//
// 0.35, and lower than an object-detection benchmark would use. The reason is what this
// corpus is for: a detector proposing a slightly loose box around a button has found the
// button — the Director will place a click inside it, and scoped OCR will read inside it —
// whereas an object-detection leaderboard cares about box quality for its own sake. Set once,
// documented, and NOT tuned per backend: a threshold chosen to favour an incumbent would make
// every comparison meaningless.
const MatchIoU = 0.35

// iou is intersection over union, 0 when either rectangle is empty.
func iou(a, b image.Rectangle) float64 {
	inter := a.Intersect(b)
	if inter.Empty() {
		return 0
	}
	ia := float64(inter.Dx() * inter.Dy())
	ua := float64(a.Dx()*a.Dy()) + float64(b.Dx()*b.Dy()) - ia
	if ua <= 0 {
		return 0
	}
	return ia / ua
}

// containment is how much of `inner` lies inside `outer`.
//
// Used for NEGATIVE regions rather than IoU. A detector that finds one small rectangle deep
// inside a large annotated patch of arena texture has made exactly the error being measured,
// and IoU against the big patch would be near zero and would forgive it.
func containment(inner, outer image.Rectangle) float64 {
	i := inner.Intersect(outer)
	if i.Empty() {
		return 0
	}
	area := float64(inner.Dx() * inner.Dy())
	if area <= 0 {
		return 0
	}
	return float64(i.Dx()*i.Dy()) / area
}

// NegativeContainment is how much of a detection must lie in a negative region to count
// as a false positive against it.
const NegativeContainment = 0.6

// Verdict is what became of one detection.
type Verdict string

const (
	// VerdictTrue matched a declared interface region.
	VerdictTrue Verdict = "true_positive"
	// VerdictFalse landed on declared scenery, or on a frame declared to hold no
	// interface at all.
	VerdictFalse Verdict = "false_positive"
	// VerdictUnmatched matched nothing either way.
	//
	// NOT counted as a false positive, deliberately. The annotation is partial by design,
	// so an unmatched box may be real structure nobody marked. Counting it as wrong would
	// punish a detector for the corpus's incompleteness; counting it as right would let
	// any detector claim anything. It is reported as its own number and left visible.
	VerdictUnmatched Verdict = "unmatched"
)

// TruthMetrics is a backend's measured agreement with declared ground truth.
type TruthMetrics struct {
	Frames       int `json:"frames"`
	Detections   int `json:"detections"`
	TruePos      int `json:"true_positives"`
	FalsePos     int `json:"false_positives"`
	Unmatched    int `json:"unmatched"`
	TruthRegions int `json:"truth_regions"`
	Matched      int `json:"matched_truth_regions"`

	// Precision and Recall over structure. Both, always: the classical size filter looked
	// excellent on precision while silently deleting real small controls, and a detector
	// cannot be allowed to earn a precision win by seeing almost nothing.
	Precision float64 `json:"structural_precision"`
	Recall    float64 `json:"structural_recall"`

	// Nameable precision closes the loophole coverage left open. A detector that calls
	// twenty rectangles buttons has high nameable COVERAGE and, if eight are really
	// nameable, 40% nameable PRECISION.
	NameableClaimed   int     `json:"nameable_claimed"`
	NameableCorrect   int     `json:"nameable_correct"`
	NameableTruth     int     `json:"nameable_truth_regions"`
	NameableFound     int     `json:"nameable_truth_found"`
	NameablePrecision float64 `json:"nameable_precision"`
	NameableRecall    float64 `json:"nameable_recall"`

	// OCR-region quality: did it propose reading in the places worth reading?
	OCRProposed  int     `json:"ocr_proposed"`
	OCRCorrect   int     `json:"ocr_correct"`
	OCRTruth     int     `json:"ocr_truth_regions"`
	OCRFound     int     `json:"ocr_truth_found"`
	OCRPrecision float64 `json:"ocr_region_precision"`
	OCRRecall    float64 `json:"ocr_region_recall"`

	// Temporal, static half. The pair that fixes the anti-correlation.
	PersistentTracks      int `json:"persistent_tracks"`
	PersistentTrue        int `json:"persistent_true"`
	PersistentTruthTracks int `json:"persistent_truth_tracks"`
	PersistentTruthFound  int `json:"persistent_truth_found"`

	// Temporal, transition half. Counted in FRAMES, because a transition's whole content is
	// timing and a per-track yes/no cannot express "found it, but one frame late".
	TransitionTracks int `json:"transition_tracks"`
	// OnTime is frames where a transitioning element was expected AND found.
	OnTime int `json:"transition_on_time"`
	// Mistimed is frames where it was found while NOT expected: claimed before it arrived,
	// or still claimed after it left. This is the stale-menu number.
	Mistimed int `json:"transition_mistimed"`
	// Expected is frames where a transitioning element should have been found.
	Expected int `json:"transition_expected"`

	TemporalPrecision float64 `json:"temporal_precision"`
	TemporalRecall    float64 `json:"temporal_recall"`
	// TemporalSequences is how many sequences contributed a defined temporal value.
	//
	// Reported because the aggregate is a mean over sequences, and a mean over an
	// unstated population is the shape of the mistake this metric already made once.
	TemporalPrecisionSequences int `json:"temporal_precision_sequences"`
	TemporalRecallSequences    int `json:"temporal_recall_sequences"`
}

// scoredDetection is one detection with its verdict, kept for temporal analysis.
type scoredDetection struct {
	bounds  image.Rectangle
	verdict Verdict
	truthID string
}

// EvaluateTruth scores one backend's per-frame detections against declared ground truth,
// treating every sequence as static.
//
// The static default is for corpora that declare no temporal mode. A v2 corpus loaded through
// LoadV2 always declares one, and cmd/director always passes them — see EvaluateTruthModes.
func EvaluateTruth(byFrame map[string][]Detection, bounds map[string]image.Rectangle,
	truths []FrameTruth) TruthMetrics {

	return EvaluateTruthModes(byFrame, bounds, truths, nil)
}

// EvaluateTruthModes scores a backend against declared ground truth, honouring each sequence's
// declared temporal semantics.
//
// frames must be in sequence order and aligned with truths by frame name; anything the truth
// set does not describe is skipped rather than guessed at.
//
// # How static and transition results combine
//
// MACRO-AVERAGE OVER SEQUENCES: every track yields a score in 0..1, a sequence is the mean of
// its tracks, and the corpus is the mean of its sequences. Chosen deliberately over a
// micro-average, for two reasons.
//
// The first is the one Part 20 of the brief names: micro-averaging would weight by frame and
// track count, so a nine-frame static sequence with six persistent identities would drown two
// six-frame transitions — and the transitions are the more interesting evidence, not the less.
// One sequence, one vote.
//
// The second is unit safety. Static tracks are counted per track and transition tracks per
// frame, because timing cannot be expressed as a yes/no. Those are different units, and adding
// them into one ratio would produce a number whose denominator meant two things at once. Both
// normalise to 0..1 at the track level first, so nothing incommensurable is ever summed.
//
// Sequences that offer no temporal evidence are EXCLUDED from the mean rather than counted as
// zero, and the surviving population is reported. Rendering "no opportunity" as 0% is the exact
// mistake that made a working detector look like it kept a closed menu alive.
func EvaluateTruthModes(byFrame map[string][]Detection, bounds map[string]image.Rectangle,
	truths []FrameTruth, modes map[string]SequenceTruth) TruthMetrics {

	var m TruthMetrics
	seqs, _ := Sequences(truths)

	// Per-sequence, so temporal metrics never span two unrelated runs.
	names := make([]string, 0, len(seqs))
	for n := range seqs {
		names = append(names, n)
	}
	sort.Strings(names)

	var precSum, recSum float64

	for _, name := range names {
		// tracks follow a detection identity across the sequence, and truthTracks follow
		// a declared identity. Both keyed the same way so the two can be compared.
		tracks := map[string]int{}
		trackTrue := map[string]int{}
		truthTracks := map[string]*truthTrack{}
		var seqFrames []evalFrame

		for _, ft := range seqs[name] {
			dets, ok := byFrame[ft.Key()]
			if !ok {
				continue
			}
			m.Frames++
			fb, ok := bounds[ft.Key()]
			if !ok || fb.Empty() {
				continue
			}
			seqFrames = append(seqFrames, evalFrame{truth: ft, bounds: fb, dets: dets})

			scored, matchedTruth := scoreFrame(dets, ft, fb)
			m.Detections += len(scored)
			m.TruthRegions += len(ft.Regions)

			for _, r := range ft.Regions {
				if r.IsNameable() {
					m.NameableTruth++
				}
				if r.Kind.OCRWorthy() {
					m.OCRTruth++
				}
				id := identityOf(r)
				tt := truthTracks[id]
				if tt == nil {
					tt = &truthTrack{ref: r.Bounds}
					truthTracks[id] = tt
				}
				tt.present = append(tt.present, ft.Index)
				if matchedTruth[id] {
					m.Matched++
					tt.found++
					if r.IsNameable() {
						m.NameableFound++
					}
					if r.Kind.OCRWorthy() {
						m.OCRFound++
					}
				}
			}

			for _, s := range scored {
				switch s.verdict {
				case VerdictTrue:
					m.TruePos++
				case VerdictFalse:
					m.FalsePos++
				default:
					m.Unmatched++
				}
				key := trackKey(s.bounds, fb)
				tracks[key]++
				if s.verdict == VerdictTrue {
					trackTrue[key]++
				}
			}

			// Role claims, scored against what the region really is.
			for _, d := range dets {
				claimed := TruthKind(d.Label)
				if claimed.Nameable() {
					m.NameableClaimed++
				}
				if claimed.OCRWorthy() {
					m.OCRProposed++
				}
			}
			m.NameableCorrect += countRoleAgreement(dets, ft, fb, TruthKind.Nameable)
			m.OCRCorrect += countRoleAgreement(dets, ft, fb, TruthKind.OCRWorthy)
		}

		// Every track that has something to say contributes one score in 0..1 to its
		// sequence. Static tracks score 0 or 1; transition tracks score by timing.
		var precScores, recScores []float64
		frames := len(seqFrames)
		declared := modes[name]

		// Persistent DETECTION structure, in every mode. A rectangle the detector holds
		// onto for most of a sequence had better be interface, whether or not the scene
		// changed underneath it — this is the measure a persistent arena artefact fails.
		need := (frames + 1) / 2
		for key, n := range tracks {
			if n < need || need == 0 {
				continue
			}
			m.PersistentTracks++
			real := trackTrue[key]*2 >= n
			if real {
				m.PersistentTrue++
			}
			precScores = append(precScores, boolScore(real))
		}

		ids := make([]string, 0, len(truthTracks))
		for id := range truthTracks {
			ids = append(ids, id)
		}
		sort.Strings(ids)

		for _, id := range ids {
			tt := truthTracks[id]
			track, isDeclared := declared.Track(id)

			// The static rule applies to an identity that is present throughout, and to
			// any identity whose absence was never DECLARED. The second half is the
			// important one: this corpus is deliberately partial, so "no annotation here"
			// does not mean "not here", and charging a detector for claiming an
			// unannotated element would punish it for the corpus's incompleteness.
			//
			// A HUD element also stays static however much changes around it — it does
			// not become a transition because a menu opened over it.
			if !isDeclared || len(tt.present) >= frames {
				if need == 0 || len(tt.present) < need {
					continue
				}
				m.PersistentTruthTracks++
				found := tt.found*2 >= len(tt.present)
				if found {
					m.PersistentTruthFound++
				}
				recScores = append(recScores, boolScore(found))
				continue
			}

			// An identity whose presence is declared to change. Two questions, both about
			// timing:
			//
			//	recall     of the frames it was there, in how many was it found
			//	precision  of every frame the detector claimed it, how many were frames
			//	           it was actually there
			//
			// Both are frame ratios, and neither refers to the sequence length or to
			// where a change falls in it. That is what makes an appearance and its
			// mirrored disappearance score identically, makes a perfect detector score
			// perfectly wherever a change falls, and lets one identity appear, leave and
			// return without any of it being a special case.
			mistimed := tt.mistimedFrames(seqFrames, track)
			m.TransitionTracks++
			m.OnTime += tt.found
			m.Mistimed += mistimed
			m.Expected += len(tt.present)

			if len(tt.present) > 0 {
				recScores = append(recScores, float64(tt.found)/float64(len(tt.present)))
			}
			// A detector that never proposed this identity at all, in either phase, has
			// no timing to judge — it is a recall failure, already counted above, and
			// scoring it as zero precision would punish the same miss twice.
			if claims := tt.found + mistimed; claims > 0 {
				precScores = append(precScores, float64(tt.found)/float64(claims))
			}
		}

		if len(precScores) > 0 {
			precSum += mean(precScores)
			m.TemporalPrecisionSequences++
		}
		if len(recScores) > 0 {
			recSum += mean(recScores)
			m.TemporalRecallSequences++
		}
	}

	if m.TemporalPrecisionSequences > 0 {
		m.TemporalPrecision = precSum / float64(m.TemporalPrecisionSequences)
	}
	if m.TemporalRecallSequences > 0 {
		m.TemporalRecall = recSum / float64(m.TemporalRecallSequences)
	}

	m.finalise()
	return m
}

// evalFrame is one frame a backend actually produced a result for.
//
// Kept for the whole sequence because a transition can only be judged by looking at frames
// where the element is NOT annotated — an element that has left is described by its absence,
// and absence is not something a per-frame loop can see.
type evalFrame struct {
	truth  FrameTruth
	bounds image.Rectangle
	dets   []Detection
}

// truthTrack follows one declared identity across a sequence.
type truthTrack struct {
	// ref is the identity's geometry, taken from a frame where it was declared present.
	//
	// Needed because the frames that matter most for a transition are the ones with no
	// annotation at all: to ask "is the detector still claiming the menu?" after the menu
	// has gone, the benchmark must remember where the menu was.
	ref     NormRect
	present []int
	found   int
}

// mistimedFrames counts frames where the detector claimed this identity but it was not there.
//
// Early, stale, or claimed during a gap between two spells on screen — one measurement, taken
// from whichever side the change happens to fall. That symmetry is what stops the metric caring
// where in a sequence something changes.
func (t *truthTrack) mistimedFrames(frames []evalFrame, track TrackTruth) int {
	n := 0
	for _, f := range frames {
		if track.PresentAt(f.truth.Index) {
			continue
		}
		want := t.ref.Pixels(f.bounds)
		for _, d := range f.dets {
			// A mask this project painted is not evidence of anything, on either side of
			// a transition.
			if inIgnore(d.Bounds, f.truth, f.bounds) {
				continue
			}
			if iou(d.Bounds, want) >= MatchIoU {
				n++
				break
			}
		}
	}
	return n
}

func boolScore(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// scoreFrame gives every detection a verdict and reports which truth regions were found.
func scoreFrame(dets []Detection, ft FrameTruth, fb image.Rectangle) (
	[]scoredDetection, map[string]bool) {

	matched := map[string]bool{}
	out := make([]scoredDetection, 0, len(dets))

	for _, d := range dets {
		// Inside a mask this project painted: not evidence either way.
		if inIgnore(d.Bounds, ft, fb) {
			continue
		}
		best, bestID := 0.0, ""
		for _, r := range ft.Regions {
			if got := iou(d.Bounds, r.Bounds.Pixels(fb)); got > best {
				best, bestID = got, identityOf(r)
			}
		}
		switch {
		case best >= MatchIoU:
			matched[bestID] = true
			out = append(out, scoredDetection{d.Bounds, VerdictTrue, bestID})
		case !ft.InterfacePresent:
			// The frame says there is nothing here. Everything found is wrong, with no
			// inference required — the strongest and cheapest annotation available.
			out = append(out, scoredDetection{d.Bounds, VerdictFalse, ""})
		case inNegative(d.Bounds, ft, fb):
			out = append(out, scoredDetection{d.Bounds, VerdictFalse, ""})
		default:
			out = append(out, scoredDetection{d.Bounds, VerdictUnmatched, ""})
		}
	}
	return out, matched
}

func inNegative(b image.Rectangle, ft FrameTruth, fb image.Rectangle) bool {
	for _, n := range ft.NegativeRegions {
		if containment(b, n.Bounds.Pixels(fb)) >= NegativeContainment {
			return true
		}
	}
	return false
}

// countRoleAgreement counts detections whose claimed role property matches the region they
// actually landed on. The predicate selects which property is being checked.
func countRoleAgreement(dets []Detection, ft FrameTruth, fb image.Rectangle,
	prop func(TruthKind) bool) int {

	n := 0
	for _, d := range dets {
		if !prop(TruthKind(d.Label)) {
			continue
		}
		for _, r := range ft.Regions {
			if iou(d.Bounds, r.Bounds.Pixels(fb)) >= MatchIoU && prop(r.Kind) {
				n++
				break
			}
		}
	}
	return n
}

func identityOf(r TruthRegion) string {
	if r.Identity != "" {
		return r.Identity
	}
	return string(r.Kind) + boundsKey(r.Bounds)
}

func boundsKey(r NormRect) string {
	return string(rune('a'+int(r.X*20))) + string(rune('a'+int(r.Y*20)))
}

// trackKey identifies a detection across frames by class-free coarse position.
//
// A twentieth of the frame per axis, which is far finer than version 1's ninths. The coarse
// key is what let unrelated rectangles merge into one apparently persistent identity; this
// one is tight enough that a HUD element tracks and a moving scene edge does not.
func trackKey(b, frame image.Rectangle) string {
	if frame.Dx() == 0 || frame.Dy() == 0 {
		return ""
	}
	cx := (b.Min.X + b.Max.X) / 2
	cy := (b.Min.Y + b.Max.Y) / 2
	gx := (cx - frame.Min.X) * 20 / frame.Dx()
	gy := (cy - frame.Min.Y) * 20 / frame.Dy()
	return string(rune('a'+clamp20(gx))) + string(rune('a'+clamp20(gy)))
}

func clamp20(v int) int {
	if v < 0 {
		return 0
	}
	if v > 19 {
		return 19
	}
	return v
}

func (m *TruthMetrics) finalise() {
	ratio := func(a, b int) float64 {
		if b == 0 {
			return 0
		}
		return float64(a) / float64(b)
	}
	// Precision is over JUDGED detections — true plus false. Unmatched ones are excluded
	// rather than counted either way, because the annotation is partial and an unmatched
	// box is genuinely unknown. The count stays in the report so nobody can forget it.
	m.Precision = ratio(m.TruePos, m.TruePos+m.FalsePos)
	m.Recall = ratio(m.Matched, m.TruthRegions)
	m.NameablePrecision = ratio(m.NameableCorrect, m.NameableClaimed)
	m.NameableRecall = ratio(m.NameableFound, m.NameableTruth)
	m.OCRPrecision = ratio(m.OCRCorrect, m.OCRProposed)
	m.OCRRecall = ratio(m.OCRFound, m.OCRTruth)
	// Temporal is NOT computed here. It is a mean over sequences, and finalise() only sees
	// corpus totals — rebuilding it from these counts is exactly how a transition would get
	// folded back into a persistence ratio.
}

// inIgnore reports whether a detection lies wholly enough inside a benchmark-created mask to
// be dropped before scoring.
//
// Uses the same containment threshold as a negative region, for the same reason: a small box
// deep inside a large masked band is entirely a product of the mask.
func inIgnore(b image.Rectangle, ft FrameTruth, fb image.Rectangle) bool {
	for _, ig := range ft.IgnoreRegions {
		if containment(b, ig.Pixels(fb)) >= NegativeContainment {
			return true
		}
	}
	return false
}
