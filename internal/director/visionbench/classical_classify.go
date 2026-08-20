package visionbench

// Classifying a rectangle, and saying why one was refused.
//
// Split out from candidate generation deliberately (see the package note on the pipeline):
// generation answers "where is there a uniform rectangle", classification answers "is that
// a control", and conflating them is what made every rejection unexplainable. A benchmark
// that cannot say WHY it dropped 65 rectangles cannot be tuned by anybody.

// Rejection is the closed vocabulary of reasons a candidate did not become a detection.
//
// Closed on purpose, and small. Each value exists because it was measured on the corpus, and
// a reason nobody can point at a frame for is a reason nobody can act on.
type Rejection string

const (
	// RejectNone: the candidate became a detection.
	RejectNone Rejection = ""
	// RejectTooSmall: below the normalised minimum useful size. The ONLY rejection rule
	// that survived ablation — see below.
	RejectTooSmall Rejection = "too_small"
	// RejectSceneSized: a single region covering most of the frame. That is the
	// background, not a control.
	RejectSceneSized Rejection = "scene_sized"
)

// # The rules that were built, measured, and removed
//
// Three other rejection rules were implemented and ablated on the reference corpus, scored
// against the manifest's declared ground truth (a detection on a frame marked
// `contains_no_ui_structure` is a false positive with no inference required):
//
//	configuration           false positives   true structure kept
//	baseline (no rules)          65                  27
//	size filter only             16                  27      ← kept
//	size + alignment             12                  23
//	size + border evidence        2                   3
//	all three                     6                  12
//
//   - BORDER CONTINUITY was the rule this milestone expected to carry the work, on the
//     theory that a control is bounded on all four sides and a patch of arena texture is
//     bounded only where the scan stopped. Measured, the two populations do not separate:
//     the corpus's two genuine menu buttons score 0.27 and 0.54, while kept panels range
//     0.15 to 0.75. Enabling it costs 24 of 27 true detections to remove 14 false ones.
//     Removed.
//
//   - ALIGNMENT as a REJECTION rule removes 4 false positives at a cost of 4 true ones.
//     That is not an improvement, it is a shuffle. Removed as a rejection; retained as a
//     FEATURE, because a future learned classifier is exactly the consumer that could use
//     it (see the hybrid note on CandidateReport).
//
//   - SCENE-SIZED rejection never fired once on the corpus — the largest candidate there
//     covers 7.9% of its frame — and was removed on that basis. It was then REINSTATED,
//     because the corpus turned out to be the wrong evidence for it: every frame in it is a
//     CROP, and a crop of arena never contains the uniform full-frame background that a real
//     screen does. A blank 640x480 frame produces exactly one candidate covering 100% of
//     itself, labelled `panel`. The rule is kept, justified by
//     TestAnEmptyFrameDoesNotBecomeOneEnormousPanel rather than by the corpus.
//
// Two rules survive. The first does the measurable work; the second exists because a corpus
// of crops cannot show what a whole screen does, which is worth remembering the next time a
// rule is judged by that corpus alone.

// Ablations turn individual heuristics off, so each one's contribution can be measured
// rather than asserted.
//
// Every field is "disable", not "enable", so the zero value is the full detector and a new
// heuristic cannot be silently omitted by a caller that predates it.

type Ablations struct {
	// NoSizeFilter disables the one surviving rejection rule, which is how the "baseline
	// (no heuristics)" row of the table above is produced. Keeping this switch is what
	// makes the claim re-checkable rather than a note in a document.
	NoSizeFilter bool
	// NoAlignmentEvidence stops alignment influencing the ROLE a kept candidate is given.
	// It no longer rejects anything.
	NoAlignmentEvidence bool
	// NoSceneSizeFilter disables the whole-frame rejection.
	NoSceneSizeFilter bool
}

// maxSceneArea is the fraction of a frame a candidate may cover and still be interface.
//
// 0.55. A pause menu or an inventory screen can legitimately be large, so this is not a
// "big things are not interface" rule — it is a "a single region covering most of the screen
// is the screen" rule. The background of any real frame is one such region; a menu is not,
// because a menu has controls drawn on it that break it up.
const maxSceneArea = 0.55

// barMaxNormH is how tall a long thin region may be and still read as a bar.
//
// 0.15, not 0.08. The tighter value was a boundary bug rather than a judgement: a 24px bar
// in a 300px-tall crop is exactly 0.08, and `< 0.08` refused it. Since the corpus is made of
// crops, a bar occupying an eighth of one is ordinary. Aspect ratio is doing the real work
// here; this only stops a tall slab from being called a meter.
const barMaxNormH = 0.15

// minNormSide is the smallest useful edge as a fraction of the frame.
//
// Normalised rather than fixed pixels so the rule survives a resolution change — the corpus
// frames range from 240x240 to 960x540, and a 24px floor means something different in each.
// 0.045 keeps a 24px control in a 540px-tall frame and a 48px one at 1080p, which is about
// the size of a checkbox.
//
// # Why this is 0.015 and not the 0.045 the corpus asks for
//
// Because the corpus and a real screen demand opposite values, and only one of them is the
// thing being built for. Measured both ways — the corpus by declared ground truth, a live
// 1920x1080 capture by shadowing the detector over it:
//
//	minNormSide   live screen kept   corpus false positives
//	0.045              4 of 83            16  (71% → 37%)
//	0.030              7 of 83            —
//	0.020             28 of 83            65  (no improvement)
//	0.015             42 of 83            65  (no improvement)
//
// 0.045 is a 86x49px floor at 1080p. A real button is around 30px tall — 0.028 — so that
// value rejects essentially every control on a real screen, which is exactly what the live
// column shows. It looks excellent on the corpus only because the corpus is made of CROPS:
// a 24px region is 10% of a 240px crop and 2% of a real screen, so "fraction of frame" is
// not the same quantity in the two places.
//
// So the rule is set where it is safe on the thing that matters — 0.015 keeps a 16px
// checkbox at 1080p — and the corpus consequently shows NO improvement from it. That is the
// honest outcome: this corpus cannot tune a normalised size rule, and a value that scores
// well on it is over-fitted by construction. See the experiment record.
const minNormSide = 0.015

// classify decides what a candidate is, or why it is not structure.
func classify(c candidate, ab Ablations) (string, Rejection) {
	if !ab.NoSizeFilter {
		if c.feat.NormW < minNormSide || c.feat.NormH < minNormSide {
			return "", RejectTooSmall
		}
	}
	if !ab.NoSceneSizeFilter && c.feat.AreaRatio > maxSceneArea {
		return "", RejectSceneSized
	}
	return roleFor(c, ab), RejectNone
}

// roleFor names the shape, as conservatively as the evidence allows.
//
// The asymmetry from the original detector is preserved: calling a panel a button invites
// the Director to believe it can be clicked, while calling a button a panel merely
// under-describes it. So `button` requires positive evidence and everything else falls back.
//
// This is also where nameable coverage is won or lost, and the temptation is to relabel
// uncertainty upward. The rule is that `button` needs BOTH a control-like shape and a peer
// group — a lone rectangle of button-ish proportions in scenery stays a panel.
func roleFor(c candidate, ab Ablations) string {
	f := c.feat
	switch {
	case f.Aspect >= 4 && f.NormH < barMaxNormH:
		// Long, thin, horizontal: a meter or progress bar.
		return "bar"
	case f.Aspect >= 1.6 && f.AreaRatio < 0.06 &&
		(ab.NoAlignmentEvidence || f.AlignedPeers >= 1):
		// Control-shaped AND part of a group. The menu rows in the corpus's pause frames
		// are exactly this: 80x24, same x, regular vertical spacing.
		return "button"
	default:
		return "panel"
	}
}
