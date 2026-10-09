// Package selection reads which of several candidate regions is the chosen one, from pixels.
//
// # The question, and why it is not "find the highlight"
//
// An interface that cannot be read through an accessibility API still says which entry is
// chosen — it says it in paint. The obvious approach is to look for a highlight, and it is
// wrong, because "highlight" is not one thing. Measured on a single Trackmania menu:
//
//   - eight entries are chosen by a white filled bar with inverted dark text
//   - EXIT is chosen by its text simply getting brighter, with no bar at all
//
// Two styles, one menu. A reader keyed on the bar reports nothing when EXIT is chosen —
// silently, and on every such frame.
//
// So this package does not know what a highlight looks like. It knows two things that are
// true of a chosen thing whatever the art direction:
//
//	SPATIALLY   among things that look alike, the chosen one does not
//	TEMPORALLY  the thing that just changed, while its neighbours did not, is where the
//	            choice went
//
// The first covers the bar. The second covers EXIT, which has no siblings to be unlike — it
// is alone at the bottom of the panel, so the only evidence available is that it is brighter
// than IT was a moment ago.
//
// # Three answers, and the third is not a failure
//
// Insufficient is a real answer and the most important one. A momentary absence of a
// recognisable cue is not evidence that nothing is chosen; it is evidence that this frame
// cannot be read. [[ADR-006-unknown-is-not-false]].
//
// NoneSelected therefore requires AFFIRMATIVE evidence and is deliberately hard to reach. The
// recording this was built from contains no frame where nothing is chosen, so that path is
// unvalidated and says so.
//
// # Why a change must be seen twice
//
// Every transition in the measured menu lasts exactly one frame, and that frame is a blend:
// the departing entry's text brightens while the arriving entry's bar is washed out and its
// text grey rather than dark. By mean luminance alone the blend is nearly indistinguishable
// from a settled selection — 204 against 206 — so a per-frame rule answers confidently and
// wrongly.
//
// The blend is distinguishable by one thing: it does not last. So a change of winner is not
// asserted until a second frame agrees. That costs one frame of latency on every real change
// and it refuses every one-frame artefact, including a measured case where the bar flickered
// to a neighbour and back with no input near it.
//
// Holding is BOUNDED. After enough unreadable frames the reader stops claiming the old
// selection, because "it was this a second ago" stops being evidence.
package selection

import (
	"image"
	"math"
	"sort"
)

// Row is one candidate region, as the caller found it.
//
// This package does not locate rows. Where they come from — a generic band finder, an
// accessibility tree, a learned detector — is the caller's business, and keeping it out means
// nothing here can quietly depend on one game's geometry.
type Row struct {
	Bounds image.Rectangle
	// Group ties rows that are siblings: alike enough that one being different is
	// evidence. Rows in different groups are never compared with each other.
	//
	// A row alone in its group has no spatial evidence available, which is the honest
	// description of EXIT and the reason the temporal half of this reader exists.
	Group int
}

// Outcome is what the reader can say about a frame.
type Outcome string

const (
	// Selected names the chosen row.
	Selected Outcome = "selected"
	// NoneSelected is a positive finding that nothing is chosen. It requires evidence,
	// never merely the absence of a cue this reader happens to recognise.
	NoneSelected Outcome = "none_selected"
	// Insufficient is what an unreadable frame gets. Not a failure: a transition really
	// is unreadable, and saying so is the correct answer.
	Insufficient Outcome = "insufficient_evidence"
)

// Result is one frame's reading.
type Result struct {
	Outcome Outcome
	// Row indexes the caller's slice when Outcome is Selected, and is -1 otherwise.
	Row int
	// Why is a short account for a person reading a trace.
	Why string
}

// Tuning. Deliberately few, and none of them a colour, a coordinate or a style.
const (
	// SpatialK is how many robust deviations from its siblings a row must stand out by.
	SpatialK = 4.0
	// SpatialRunnerUp is how much weaker the second candidate must be for the first to be
	// unique. Two rows deviating together is a blend, not a selection.
	SpatialRunnerUp = 0.5
	// TemporalK is how far a row must move from its own recent level, in the same robust
	// units measured across rows, to count as having changed.
	TemporalK = 4.0
	// BaselineSamples is how many unselected observations of a row are kept. Enough for a
	// stable median, few enough that a scene which genuinely changed is relearned.
	BaselineSamples = 16
	// MinBaseline is how many unselected observations a row needs before its own level
	// means anything. Below it the reader has no temporal evidence and says so.
	MinBaseline = 3
	// ConfirmFrames is how many consecutive frames must agree before a CHANGE of winner is
	// asserted. Two: the measured blends all last exactly one frame.
	ConfirmFrames = 2
	// HoldFrames bounds how long a settled selection survives unreadable frames. Past it
	// the reader stops claiming it, because an old observation is not a current one.
	HoldFrames = 6
	// MinSpread floors the robust scale so a perfectly uniform set of rows cannot make an
	// arbitrarily small difference look like an infinite deviation.
	MinSpread = 1.5
)

// Reader carries the little history the temporal half needs.
//
// What it keeps is one thing: for each row, a sample of that row's level FROM FRAMES IN WHICH
// IT WAS NOT THE CHOSEN ONE. That is the row's baseline, and comparing against it is what
// lets a selection be recognised for as long as it lasts rather than for as long as it is
// new.
//
// A sliding window over recent frames cannot do this, and the failure is quiet: hold a
// selection for a second and the window fills with selected levels, the row stops looking
// different from itself, and the reader drops a selection that is still plainly on screen.
// The samples here are therefore collected per row and never expire merely because time
// passed — only because newer unselected samples displaced them.
type Reader struct {
	baseline [][]desc // per row, recent descriptions from frames in which it was not claimed
	asserted int      // the row currently being claimed, or -1
	pending  int      // a candidate not yet confirmed, or -1
	pendingN int      // how many frames it has held
	unread   int      // consecutive frames with no usable winner
}

// New builds a reader with no history.
func New() *Reader { return &Reader{asserted: -1, pending: -1} }

// Reset forgets everything. Call it when the scene changed for a reason the reader cannot
// see — a different window, a different screen — so that stale levels cannot be compared
// against fresh ones.
func (r *Reader) Reset() {
	r.baseline = nil
	r.asserted, r.pending, r.pendingN, r.unread = -1, -1, 0, 0
}

// Read judges one frame against the candidate rows it is given.
func (r *Reader) Read(img image.Image, rows []Row) Result {
	if len(rows) == 0 {
		return Result{Outcome: Insufficient, Row: -1, Why: "no candidate rows"}
	}
	descs := make([]desc, len(rows))
	for i, row := range rows {
		descs[i] = describe(img, row.Bounds)
	}

	winner, why := r.candidate(rows, descs)
	// Remembered with the claim that was standing when the frame was read, so a row's
	// baseline is built only from frames in which it was not the chosen one.
	r.remember(descs, r.asserted)
	return r.settle(winner, why)
}

// candidate finds the row this FRAME points at, before any history of decisions is applied.
func (r *Reader) candidate(rows []Row, descs []desc) (int, string) {
	spatial, spatialWhy := r.spatialOutlier(rows, descs)
	temporal, temporalWhy := r.temporalOutlier(descs)

	switch {
	case spatial >= 0 && temporal >= 0 && spatial != temporal:
		// One row is unlike its siblings and a DIFFERENT one just moved. Something is
		// mid-change; neither claim is safe.
		return -1, "two different rows carry the two kinds of evidence"
	case spatial >= 0:
		return spatial, spatialWhy
	case temporal >= 0:
		return temporal, temporalWhy
	}
	return -1, "no row stands out from its siblings or from its own recent level"
}

// spatialOutlier finds the one row unlike the others in its group.
func (r *Reader) spatialOutlier(rows []Row, descs []desc) (int, string) {
	groups := map[int][]int{}
	for i, row := range rows {
		groups[row.Group] = append(groups[row.Group], i)
	}
	rowScore := make([]float64, len(descs))
	for _, members := range groups {
		if len(members) < 3 {
			// Two rows cannot say which of them is the odd one out, and one cannot say
			// anything at all. Not a defect: those rows are covered temporally.
			continue
		}
		// Each feature is scored on its own and a row keeps its strongest. A selection
		// that changes only what is DRAWN on a row barely moves that row's overall level,
		// so requiring both to agree would discard the evidence that matters.
		for f := range Features {
			vals := make([]float64, len(members))
			for i, m := range members {
				vals[i] = descs[m][f]
			}
			mid := median(vals)
			scale := math.Max(mad(vals, mid), MinSpread)
			for _, m := range members {
				if s := math.Abs(descs[m][f]-mid) / scale; s > rowScore[m] {
					rowScore[m] = s
				}
			}
		}
	}
	best, bestScore, runnerUp := -1, 0.0, 0.0
	for i, s := range rowScore {
		switch {
		case s > bestScore:
			best, runnerUp, bestScore = i, bestScore, s
		case s > runnerUp:
			runnerUp = s
		}
	}
	if best < 0 || bestScore < SpatialK {
		return -1, ""
	}
	if runnerUp > bestScore*SpatialRunnerUp {
		// A second row is nearly as unlike the rest. That is what a blend looks like
		// from here, and it is the whole reason this check exists.
		return -1, "two rows deviate together"
	}
	return best, "unlike its siblings"
}

// temporalOutlier finds the one row standing away from its OWN unselected level.
//
// The baseline for a row is taken only from frames in which that row was not being claimed,
// which is what lets a selection be recognised for as long as it lasts rather than for as
// long as it is new. It also means a row that has just LOST the selection falls back TO its
// baseline and stops deviating, so the departing row does not compete with the arriving one.
//
// The deviation is absolute, deliberately. Nothing here assumes a chosen thing gets brighter;
// a menu that marks its choice with a dark fill deviates downwards and reads the same way.
func (r *Reader) temporalOutlier(descs []desc) (int, string) {
	if len(r.baseline) != len(descs) {
		return -1, ""
	}
	for i := range descs {
		if len(r.baseline[i]) < MinBaseline {
			return -1, ""
		}
	}
	rowScore := make([]float64, len(descs))
	for f := range Features {
		devs := make([]float64, len(descs))
		for i := range descs {
			past := make([]float64, len(r.baseline[i]))
			for j, d := range r.baseline[i] {
				past[j] = d[f]
			}
			devs[i] = descs[i][f] - median(past)
		}
		mid := median(devs)
		scale := math.Max(mad(devs, mid), MinSpread)
		for i := range devs {
			if s := math.Abs(devs[i]-mid) / scale; s > rowScore[i] {
				rowScore[i] = s
			}
		}
	}
	best, bestScore, runnerUp := -1, 0.0, 0.0
	for i, s := range rowScore {
		switch {
		case s > bestScore:
			best, runnerUp, bestScore = i, bestScore, s
		case s > runnerUp:
			runnerUp = s
		}
	}
	if best < 0 || bestScore < TemporalK || runnerUp > bestScore*SpatialRunnerUp {
		return -1, ""
	}
	return best, "away from its own unselected level, while its neighbours are not"
}

// settle turns this frame's candidate into an answer, using what was decided before.
func (r *Reader) settle(winner int, why string) Result {
	if winner < 0 {
		r.pending, r.pendingN = -1, 0
		r.unread++
		if r.asserted >= 0 && r.unread <= HoldFrames {
			return Result{Outcome: Selected, Row: r.asserted,
				Why: "unreadable this frame; still claiming the settled selection"}
		}
		if r.asserted >= 0 {
			// Held as long as is honest. An old observation is not a current one.
			r.asserted = -1
			return Result{Outcome: Insufficient, Row: -1,
				Why: "unreadable for too long to keep claiming the previous selection"}
		}
		return Result{Outcome: Insufficient, Row: -1, Why: why}
	}
	r.unread = 0
	if winner == r.asserted {
		r.pending, r.pendingN = -1, 0
		return Result{Outcome: Selected, Row: winner, Why: why}
	}
	// A different row. Every measured transition blends for exactly one frame, so a
	// change is not asserted until a second frame agrees.
	if winner == r.pending {
		r.pendingN++
	} else {
		r.pending, r.pendingN = winner, 1
	}
	if r.pendingN >= ConfirmFrames {
		r.asserted, r.pending, r.pendingN = winner, -1, 0
		return Result{Outcome: Selected, Row: winner, Why: why}
	}
	return Result{Outcome: Insufficient, Row: -1,
		Why: "a different row this frame; waiting for it to be confirmed"}
}

// remember adds this frame's levels to the baseline of every row that was not being claimed.
//
// A changed row count means the caller is no longer describing the same thing, so the
// baselines are dropped rather than re-indexed onto rows they were never measured from.
func (r *Reader) remember(descs []desc, asserted int) {
	if len(r.baseline) != len(descs) {
		r.baseline = make([][]desc, len(descs))
	}
	for i, v := range descs {
		if i == asserted {
			continue
		}
		r.baseline[i] = append(r.baseline[i], v)
		if len(r.baseline[i]) > BaselineSamples {
			r.baseline[i] = r.baseline[i][len(r.baseline[i])-BaselineSamples:]
		}
	}
}

// Features is how many numbers describe a row.
const Features = 2

// desc describes one row: its overall level, and the level of its brightest quarter.
//
// Two numbers, and the second earned its place by a measured failure. The mean alone was
// enough for a filled bar, which repaints most of a row, and useless for a selection drawn by
// brightening the TEXT: text covers a small part of a wide row, so a change that is obvious
// to a person moves the mean by two or three units and never clears a threshold. On the
// measured menu this cost every frame of one of the two highlight styles — the reader kept
// claiming the previous entry instead, confidently and wrongly, for as long as it was allowed
// to hold.
//
// The brightest quarter is the general form of "what is drawn on this row", and it moves for
// all three shapes a selection takes: a bright fill raises it, brightened text raises it, and
// a dark fill lowers it. Neither number assumes which direction means chosen.
type desc [Features]float64

func describe(img image.Image, b image.Rectangle) desc {
	b = b.Intersect(img.Bounds())
	if b.Empty() {
		return desc{}
	}
	// Sampled rather than exhaustive: a row is thousands of pixels and these are summary
	// statistics. Every second pixel each way is four times cheaper and the same answer.
	var vals []float64
	var sum float64
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x += 2 {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			l := 0.299*float64(cr>>8) + 0.587*float64(cg>>8) + 0.114*float64(cb>>8)
			vals = append(vals, l)
			sum += l
		}
	}
	if len(vals) == 0 {
		return desc{}
	}
	sortFloats(vals)
	top := vals[len(vals)*3/4:]
	var tsum float64
	for _, v := range top {
		tsum += v
	}
	return desc{sum / float64(len(vals)), tsum / float64(len(top))}
}

func median(v []float64) float64 {
	c := append([]float64(nil), v...)
	insertionSort(c)
	n := len(c)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return c[n/2]
	}
	return (c[n/2-1] + c[n/2]) / 2
}

// mad is the median absolute deviation: a spread that a single large outlier cannot inflate,
// which matters here because the outlier is the thing being looked for.
func mad(v []float64, mid float64) float64 {
	d := make([]float64, len(v))
	for i, x := range v {
		d[i] = math.Abs(x - mid)
	}
	return median(d)
}

// sortFloats orders a large sample. The medians below run over a handful of rows and use
// insertionSort; this one runs over thousands of pixels, where that would be quadratic.
func sortFloats(a []float64) { sort.Float64s(a) }

func insertionSort(a []float64) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}
