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
	// ID is which ENTRY of a tracked list this row is, or zero when nothing can say.
	//
	// # Why the reader needs this and could not have it before
	//
	// A reader's only evidence about a row is how it compares with its own past, and its
	// past has to be filed under something. It was filed under the row's position in the
	// slice, which is not a thing that survives a frame: a detector that finds six rows
	// and then seven has renumbered every row below the one it gained, so a position-filed
	// history has to be thrown away whenever the count changes. On 864 real frames of one
	// menu the count changed on most of them, and the reader said "no row stands out from
	// its own unselected level" on 279 of the 312 frames where it was handed the right
	// list.
	//
	// Zero means NO IDENTITY and is not an entry. A reader handed rows with no identities
	// at all behaves exactly as it did, position-filed and dropped on a count change,
	// because without an identity that is the only honest thing to do.
	// [[ADR-165-an-entry-is-the-same-entry-when-nothing-says-otherwise]].
	ID int
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

	// ProvisionalEntry is the identity of the entry Provisional names, or zero. Same
	// reason as Entry: a position does not survive a frame, and the caller that banks a
	// provisional sighting looks it up again a frame or more later.
	ProvisionalEntry int
	// Entry is the identity of the entry Row names, or zero when the rows carried none.
	//
	// Row is a position in the slice the caller passed, and positions do not survive a
	// frame. A caller that wants to know whether this is the SAME entry it was told about
	// last time has to be told something that does; see Row.ID.
	Entry int
	// Provisional names the row THIS FRAME pointed at, when that row is not the one being
	// published. -1 when the frame pointed at nothing, and -1 on the frame a row is
	// confirmed, because a confirmed row is published rather than provisional.
	//
	// # What it is for, and the one thing it must never be used for
	//
	// A change of winner is not published until a second frame agrees, which is correct
	// and costs a frame of latency on every real move. The cost is not in the answer — the
	// answer is right either way — it is in the TIME the answer carries: a consumer that
	// records when it was told has recorded the confirmation, not the change, and anything
	// that joins a move to a cause over a window is then systematically late by the
	// confirmation delay.
	//
	// So this field says when the pixels moved, for a consumer that needs the timing. It
	// is NOT a selection. Reading it as one re-admits every failure ConfirmFrames exists
	// to refuse: the measured blend lasts exactly one frame and names the row the selection
	// is LEAVING, so a provisional sighting is wrong about as often as it is right.
	//
	// The safe use is the narrow one, and it is narrow by arithmetic rather than by
	// convention: a consumer keeps the moment a provisional row APPEARED, and reads it only
	// once that same row is published. A provisional sighting that is never confirmed is
	// then never read by anything — and a blend, which names a different row, cannot
	// contribute its moment to the row that follows it, because a different row resets the
	// moment.
	//
	// Enforced by TestAProvisionalSightingNeverBecomesAReading and
	// TestABlendDoesNotLendItsMomentToTheRowThatFollowsIt.
	Provisional int
	// ProvisionalN is how many consecutive frames have pointed at Provisional. 0 when
	// Provisional is -1.
	ProvisionalN int
}

// reading is the one place a Result is built.
//
// Because Provisional is an INDEX, and the zero value of an index is a valid row. A Result
// written as a literal anywhere else would say "row 0 is settling" by omission, on every frame
// that said nothing at all — and row 0 is a real row with a real entry on it.
func reading(outcome Outcome, row int, why string) Result {
	return Result{Outcome: outcome, Row: row, Why: why, Provisional: -1}
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
	// MinAbsolute is how far a row must differ in plain luminance, not just in robust
	// units, before the difference counts as a cue.
	//
	// Normalising by a spread alone is not enough, and the failure is sharp: a tight
	// group of near-identical rows has almost no spread, so the floor does the dividing
	// and a difference of two units scores higher than a menu bar of a hundred and
	// twenty. Measured, that handed the answer to a heads-up display whose rows happen to
	// be evenly spaced. A selection is something a person can SEE.
	MinAbsolute = 8.0
	// MinSpread floors the robust scale so a perfectly uniform set of rows cannot make an
	// arbitrarily small difference look like an infinite deviation.
	MinSpread = 1.5
	// FadeRatio is how much of its previous distance from baseline a row must keep to be
	// treated as still carrying a cue. Below it the cue is dying, not arriving.
	FadeRatio = 0.7
	// FadeFloor is the distance below which fading is not worth testing, so ordinary
	// flicker in a row that is barely deviating cannot read as a fade.
	FadeFloor = 4.0
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
	// hist is recent descriptions of each row from frames in which it was not claimed,
	// filed under the row's KEY: its entry identity where it has one, and otherwise its
	// position in the slice, which is all there is to file under. See Row.ID.
	hist map[int][]desc
	// keyed says the rows the history was built from carried identities. A reader handed
	// identified rows and then unidentified ones is being told about two different things,
	// so the history is dropped rather than re-filed under positions it was never measured
	// at.
	keyed bool
	// seen is how many rows the last frame had, used only in the unkeyed case, where a
	// changed count means the caller is no longer describing the same rows and nothing
	// can say which of them moved.
	seen int
	// lastDev is each row's distance from its own baseline on the previous frame, by key.
	//
	// Kept because magnitude alone cannot tell an arriving cue from a departing one, and
	// the measured failures were all departing ones. When an entry loses the selection
	// its text stays brighter than its neighbours' for a frame: that is unlike its
	// siblings AND away from its own baseline, so both kinds of evidence name the entry
	// the selection has just left. What separates them is that one is growing and the
	// other is dying.
	lastDev  map[int]float64
	asserted int // the KEY of the row being claimed, or 0
	pending  int // the key of a candidate not yet confirmed, or 0
	pendingN int // how many frames it has held
	unread   int // consecutive frames with no usable winner
}

// New builds a reader with no history.
func New() *Reader { return &Reader{hist: map[int][]desc{}} }

// Reset forgets everything. Call it when the scene changed for a reason the reader cannot
// see — a different window, a different screen — so that stale levels cannot be compared
// against fresh ones.
func (r *Reader) Reset() {
	r.hist, r.lastDev = map[int][]desc{}, nil
	r.keyed, r.seen = false, 0
	r.asserted, r.pending, r.pendingN, r.unread = 0, 0, 0, 0
}

// keysOf names each row for the reader's own bookkeeping.
//
// An entry identity where there is one, and otherwise MINUS the position plus one, so that zero
// always means "nothing names this row" and a history filed under a position can never be
// mistaken for one filed under an identity. The sign is the cheap half of a guarantee that
// would otherwise need a reset every time a caller changed its mind about naming rows.
//
// Mixed frames take the identities and leave the rest unnamed: a row nothing can identify has
// no history to compare against and contributes no temporal evidence, which is the honest
// answer rather than a guess at which row it is.
func keysOf(rows []Row) ([]int, bool) {
	var keyed bool
	for _, row := range rows {
		if row.ID != 0 {
			keyed = true
			break
		}
	}
	out := make([]int, len(rows))
	for i, row := range rows {
		switch {
		case keyed:
			out[i] = row.ID
		default:
			out[i] = -(i + 1)
		}
	}
	return out, keyed
}

// Read judges one frame against the candidate rows it is given.
func (r *Reader) Read(img image.Image, rows []Row) Result {
	descs := make([]Shade, len(rows))
	for i, row := range rows {
		descs[i] = Measure(img, row.Bounds)
	}
	return r.ReadShades(rows, descs)
}

// ReadShades is the same, for a caller that already measured.
//
// The live observation path measures where the frame is and carries numbers onward, so this
// is the door it comes in by. Order matches rows.
func (r *Reader) ReadShades(rows []Row, descs []Shade) Result {
	if len(rows) == 0 || len(descs) != len(rows) {
		return reading(Insufficient, -1, "no candidate rows")
	}

	keys, keyed := keysOf(rows)
	// A HISTORY MEASURED UNDER POSITIONS IS NOT THIS FRAME'S HISTORY once the count has
	// changed: the rows below the one that went missing are all at different indices, and
	// nothing says which of them is which.
	//
	// Only in the unnamed case. A reader whose rows carry identities keeps each row's
	// history under its own name, which is the whole decision. And a reader that is given
	// identified rows after unidentified ones needs no reset either, because the two kinds
	// of key cannot collide — see keysOf.
	if !keyed && r.seen != len(rows) {
		r.hist, r.lastDev = map[int][]desc{}, nil
		r.asserted, r.pending, r.pendingN = 0, 0, 0
	}
	r.keyed, r.seen = keyed, len(rows)

	winner, why, departing := r.candidate(rows, descs, keys)
	// Recorded AFTER the judgement, so this frame is compared with the one before it.
	devs := make(map[int]float64, len(descs))
	for i := range descs {
		if keys[i] != 0 {
			devs[keys[i]] = r.deviation(descs, keys, i)
		}
	}
	// Remembered with the claim that was standing when the frame was read, so a row's
	// baseline is built only from frames in which it was not the chosen one.
	r.remember(descs, keys)
	r.lastDev = devs
	return r.settle(winner, keys, why, departing)
}

// hasHistory says whether this row has enough of its own unselected level to be judged
// against it.
func (r *Reader) hasHistory(keys []int, i int) bool {
	if i < 0 || i >= len(keys) || keys[i] == 0 {
		return false
	}
	return len(r.hist[keys[i]]) >= MinBaseline
}

// deviation is how far a row stands from its own unselected baseline, on whichever of its
// features says so most loudly. Zero when there is not yet a baseline to compare against.
func (r *Reader) deviation(descs []desc, keys []int, i int) float64 {
	if i < 0 || i >= len(descs) || keys[i] == 0 ||
		len(r.hist[keys[i]]) < MinBaseline {
		return 0
	}
	hist := r.hist[keys[i]]
	var worst float64
	for f := range Features {
		past := make([]float64, len(hist))
		for j, d := range hist {
			past[j] = d[f]
		}
		if v := math.Abs(descs[i][f] - median(past)); v > worst {
			worst = v
		}
	}
	return worst
}

// candidate finds the row this FRAME points at, before any history of decisions is applied.
func (r *Reader) candidate(rows []Row, descs []desc, keys []int) (int, string, bool) {
	spatial, spatialWhy := r.spatialOutlier(rows, descs)
	temporal, temporalWhy, temporalReady := r.temporalOutlier(descs, keys)

	// A candidate whose cue is DYING is the row the selection is leaving, not the row it
	// has arrived at. Refusing it is what turns a confident wrong answer into honest
	// uncertainty, and it costs nothing on a settled selection, whose distance from
	// baseline holds steady rather than shrinking.
	fading := func(i int) bool {
		if i < 0 || keys[i] == 0 {
			return false
		}
		before, ok := r.lastDev[keys[i]]
		if !ok {
			return false
		}
		now := r.deviation(descs, keys, i)
		return before > FadeFloor && now < before*FadeRatio
	}

	// Before there is any baseline, being unlike one's siblings is the only evidence
	// there is.
	//
	// PER ROW AS WELL AS OVERALL, and the second half of that is a measured repair. The
	// test used to be global: no temporal evidence anywhere, so take the spatial answer.
	// Temporal evidence is now gathered from the rows that HAVE a history rather than
	// refused unless every row does, which is most of what let the reader read at all —
	// and it took this bypass away from the one row that needs it most.
	//
	// A row marked from the first look the reader ever gets of it has no unselected level
	// and cannot acquire one, because the only frames it could learn from are frames in
	// which it is marked. Without the bypass it is never published; not being published,
	// it is not the standing claim; not being the standing claim, its selected level is
	// filed as its unselected one; and from then on it and the row the mark moves to BOTH
	// look deviant, so the temporal half abstains and the spatial half alone is refused as
	// residual. Measured on a drawn fixture: a selection that was read in four looks was
	// never read at all.
	//
	// So the bypass asks about the row in question rather than about the frame.
	if !temporalReady || (spatial >= 0 && !r.hasHistory(keys, spatial)) {
		if spatial >= 0 {
			return spatial, spatialWhy, false
		}
		return -1, "no row stands out from its siblings, and there is no history yet", false
	}

	switch {
	case spatial >= 0 && spatial == temporal && fading(spatial):
		return -1, "its cue is fading, so this is the row the selection is leaving", true
	case temporal >= 0 && spatial < 0 && fading(temporal):
		return -1, "its cue is fading, so this is the row the selection is leaving", true
	case spatial >= 0 && temporal >= 0 && spatial == temporal:
		return spatial, spatialWhy + ", and away from its own unselected level", false
	case spatial >= 0 && temporal >= 0:
		// One row is unlike its siblings and a DIFFERENT one has moved. Something is
		// mid-change; neither claim is safe.
		return -1, "two different rows carry the two kinds of evidence", false
	case spatial >= 0:
		// Unlike its siblings, but NOT away from its own unselected level — so it looks
		// like this because of what it is, or because of what it just stopped being.
		//
		// This is the measured EXIT failure. Leaving an entry, its text stays brighter
		// than its neighbours' for a frame while the chosen one's cue has not yet
		// established, and spatial evidence alone then names the entry the selection has
		// just LEFT. Six confident wrong answers, all this, all in the same place.
		// Residual appearance is not current selection.
		return -1, "unlike its siblings but not away from its own level; residual, not current", false
	case temporal >= 0:
		// Away from its own level with no siblings to compare against: a row standing
		// apart from the stack, which is the only evidence it can ever offer.
		return temporal, temporalWhy, false
	}
	return -1, "no row stands out from its siblings or from its own unselected level", false
}

// spatialOutlier finds the one row unlike the others in its group.
func (r *Reader) spatialOutlier(rows []Row, descs []desc) (int, string) {
	groups := map[int][]int{}
	for i, row := range rows {
		groups[row.Group] = append(groups[row.Group], i)
	}
	rowScore := make([]float64, len(descs))
	rowAbs := make([]float64, len(descs))
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
				d := math.Abs(descs[m][f] - mid)
				if s := d / scale; s > rowScore[m] {
					rowScore[m] = s
				}
				if d > rowAbs[m] {
					rowAbs[m] = d
				}
			}
		}
	}
	best, bestScore, runnerUp := -1, 0.0, 0.0
	for i, s := range rowScore {
		// A difference nobody could see is not a cue, however unusual it is.
		if rowAbs[i] < MinAbsolute {
			continue
		}
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
func (r *Reader) temporalOutlier(descs []desc, keys []int) (int, string, bool) {
	// THE ROWS THAT HAVE A HISTORY, and only those.
	//
	// It used to be every row or none: one row without MinBaseline samples and the whole
	// frame offered no temporal evidence at all. On a real list that is the common case
	// rather than the exception — a detector finds a row it had not found before, or a
	// list gains one — and the cost is not a cautious answer but no answer. The tracker
	// beside this reader has always compared the members it could judge and said how many
	// they were; measured on the same 864 frames it found a mark on 261 of them while this
	// said nothing on 279.
	//
	// minStack of them, which is the same floor the tracker uses and for the same reason:
	// two rows have no middle for a third to stand out from.
	judged := make([]int, 0, len(descs))
	for i := range descs {
		if keys[i] != 0 && len(r.hist[keys[i]]) >= MinBaseline {
			judged = append(judged, i)
		}
	}
	if len(judged) < minStack {
		return -1, "", false
	}
	rowScore := make([]float64, len(descs))
	rowAbs := make([]float64, len(descs))
	for f := range Features {
		devs := make([]float64, len(judged))
		for k, i := range judged {
			hist := r.hist[keys[i]]
			past := make([]float64, len(hist))
			for j, d := range hist {
				past[j] = d[f]
			}
			devs[k] = descs[i][f] - median(past)
		}
		mid := median(devs)
		scale := math.Max(mad(devs, mid), MinSpread)
		for k, i := range judged {
			d := math.Abs(devs[k] - mid)
			if sc := d / scale; sc > rowScore[i] {
				rowScore[i] = sc
			}
			if d > rowAbs[i] {
				rowAbs[i] = d
			}
		}
	}
	best, bestScore, runnerUp := -1, 0.0, 0.0
	for _, i := range judged {
		s := rowScore[i]
		if rowAbs[i] < MinAbsolute {
			continue
		}
		switch {
		case s > bestScore:
			best, runnerUp, bestScore = i, bestScore, s
		case s > runnerUp:
			runnerUp = s
		}
	}
	if best < 0 || bestScore < TemporalK || runnerUp > bestScore*SpatialRunnerUp {
		return -1, "", true
	}
	return best, "away from its own unselected level, while its neighbours are not", true
}

// settle turns this frame's candidate into an answer, using what was decided before.
func (r *Reader) settle(winner int, keys []int, why string, departing bool) Result {
	// THE CLAIM IS A KEY AND THE ANSWER IS A POSITION, and translating between them is
	// this function's other job. A key names an entry and survives a frame; a position
	// names a slot in the slice the caller passed and does not.
	at := func(key int) int {
		if key == 0 {
			return -1
		}
		for i, k := range keys {
			if k == key {
				return i
			}
		}
		return -1
	}
	key := 0
	if winner >= 0 {
		key = keys[winner]
	}
	named := func(row int) Result {
		out := reading(Selected, row, why)
		if row >= 0 && row < len(keys) && r.keyed {
			out.Entry = keys[row]
		}
		return out
	}
	if winner < 0 {
		r.pending, r.pendingN = 0, 0
		r.unread++
		if departing {
			// Holding is for a frame that could not be READ. This frame was read, and
			// what it says is that the selection is leaving the row being claimed.
			// Continuing to claim it would be the reader insisting on a thing it has
			// just watched end — which is precisely the measured failure, six confident
			// wrong answers all naming the entry the selection had moved off.
			r.asserted = 0
			return reading(Insufficient, -1, why)
		}
		held := at(r.asserted)
		if held >= 0 && r.unread <= HoldFrames {
			why = "unreadable this frame; still claiming the settled selection"
			return named(held)
		}
		if r.asserted != 0 && held < 0 {
			// THE ENTRY IT WAS CLAIMING IS NOT IN THIS FRAME. There is no row to
			// point at, and pointing at the position it used to be at would name
			// whichever row has moved into that slot. An old observation is not a
			// current one, and a position is not an entry.
			r.asserted = 0
			return reading(Insufficient, -1,
				"the entry it was claiming is not among this frame's rows")
		}
		if r.asserted != 0 {
			// Held as long as is honest. An old observation is not a current one.
			r.asserted = 0
			return reading(Insufficient, -1,
				"unreadable for too long to keep claiming the previous selection")
		}
		return reading(Insufficient, -1, why)
	}
	r.unread = 0
	if key == r.asserted {
		r.pending, r.pendingN = 0, 0
		return named(winner)
	}
	// A different row. Every measured transition blends for exactly one frame, so a
	// change is not asserted until a second frame agrees.
	if key == r.pending {
		r.pendingN++
	} else {
		r.pending, r.pendingN = key, 1
	}
	if r.pendingN >= ConfirmFrames {
		r.asserted, r.pending, r.pendingN = key, 0, 0
		return named(winner)
	}
	// THE ONE FRAME THAT REPORTS A PROVISIONAL ROW, and it reports it alongside an answer
	// that still says nothing. Outcome and Row are untouched by the field beside them: a
	// caller that ignores Provisional sees exactly what it saw before this field existed,
	// which is what makes the addition impossible to regress coverage or accuracy with.
	out := reading(Insufficient, -1,
		"a different row this frame; waiting for it to be confirmed")
	out.Provisional, out.ProvisionalN = winner, r.pendingN
	if r.keyed {
		out.ProvisionalEntry = key
	}
	return out
}

// remember adds this frame's levels to the history of every row that was not being claimed.
//
// Filed under the row's key, so a frame in which some other row was missing does not disturb
// it. A row nothing can name contributes nothing: there is no honest place to file it.
func (r *Reader) remember(descs []desc, keys []int) {
	for i, v := range descs {
		k := keys[i]
		// NOR THE ROW BEING CONFIRMED. A row pending confirmation is one the reader
		// has pointed at and not yet published, and filing its level as that row's
		// UNSELECTED level is filing a selected sample. Measured: a drawn list's
		// settled selection was asserted on one look and dropped on the next as a
		// fading cue, because its own distance from its own baseline had shrunk by
		// exactly the sample taken while it was pending.
		//
		// Bounded: a row is pending for at most ConfirmFrames looks. Excluding it for
		// longer than that was tried — remembering AFTER settling, so the row is
		// skipped on the look it is first pointed at as well — and it bought one right
		// answer for ten NEIGHBOUR confusions on 864 real frames.
		if k == 0 || k == r.asserted || k == r.pending {
			continue
		}
		r.hist[k] = append(r.hist[k], v)
		if len(r.hist[k]) > BaselineSamples {
			r.hist[k] = r.hist[k][len(r.hist[k])-BaselineSamples:]
		}
	}
	// AND THE NAMES THAT ARE NO LONGER USED ARE FORGOTTEN, so a session on a changing
	// screen does not accumulate a history for every row it has ever seen. Bounded by what
	// is on the screen rather than by what has been, which is the same thing candidate
	// retirement does one level up.
	if len(r.hist) > len(keys)*maxHistoryLists {
		live := make(map[int]bool, len(keys))
		for _, k := range keys {
			live[k] = true
		}
		for k := range r.hist {
			if !live[k] && k != r.asserted && k != r.pending {
				delete(r.hist, k)
			}
		}
	}
}

// maxHistoryLists is how many lists' worth of rows a reader may keep a history for before it
// forgets the ones that are not on the screen.
//
// Not one, because a reader is handed the rows of one list at a time and a list whose rows the
// detector is finding intermittently would otherwise lose the history of whichever rows were
// missing from the last frame — which is the defect this filing exists to repair. Not unbounded,
// because a session on a changing screen would keep every row it had ever seen.
const maxHistoryLists = 4

// Features is how many numbers describe a row.
const Features = 2

// Shade describes one row: its overall level, and the level of its brightest quarter.
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
type Shade [Features]float64

// desc is the internal spelling, kept so the reasoning above reads as it was written.
type desc = Shade

// Measure computes a row.s shade from pixels.
//
// Exported because the live observation path cannot carry frames: a detector runs where the
// image is, and what reaches a session is geometry. Two numbers per region ride along with
// it — no pixels, no text, nothing a label could hide in — and the tracker downstream works
// from those. One definition, used by both, so a reading taken live and a reading taken from
// a recording are the same measurement.
func Measure(img image.Image, b image.Rectangle) Shade { return describe(img, b) }

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
