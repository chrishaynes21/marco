package selection

import (
	"fmt"
	"image"
	"math"
)

// Which of the lists on a screen is the one being used.
//
// # The problem this exists for, stated from a measurement
//
// Discovery works. Offered the regions a detector proposed, grouping finds every row of a
// game's menu on every frame. The reader then reads the wrong list: it picks a deviant row
// from a rotating advertisement, or from a heads-up display whose numbers animate. Those are
// real, visible changes in genuinely regular lists. End to end that was 100% discovery and 0%
// reading.
//
// Nothing about appearance separates them. A menu is not the biggest list, the brightest
// list, or the list with the most going on — on the measured frame it was none of those.
//
// # What does separate them
//
// A list being navigated shows ONE member marked at a time, and the mark MOVES between
// members. A heads-up display changes several values at once, independently, and a rotating
// advertisement changes the same region over and over without the change ever transferring to
// a sibling.
//
// So the test is exclusivity plus transfer, over time:
//
//	EXCLUSIVE   in most frames, at most one member stands out
//	TRANSFERS   the member that stands out changes, more than once
//	HOLDERS     and more than one distinct member has held it
//
// None of those can be read from a single frame, which is why this is a tracker and not a
// predicate. It needs to watch somebody navigate.
//
// # Abstention is the normal early state
//
// Until a list has been seen handing its mark around, there is no evidence here, and the
// honest answer is that the active interface is not yet known. Two lists both passing is also
// an abstention: a tracker that picks one to avoid saying "I cannot tell" is guessing with
// extra steps.
//
// # Candidates have no identity between frames
//
// Grouping runs per frame and numbers its groups by where they fell in that frame's
// proposals. Those numbers are not stable, and neither is membership — a detector drops a row
// on one frame and finds it on the next. So candidates are matched across frames by WHERE
// THEY ARE, and a candidate whose extent stops matching anything is simply not updated.

const (
	// SharedPenalty is how far exclusive frames must outnumber frames in which several
	// members were marked at once.
	//
	// This is the exclusivity claim, and on real data it is the one that works. An
	// earlier version also demanded that a candidate carry a mark in most of the frames
	// it was seen in — "a menu nearly always has something selected". Measured, that
	// ranked the two candidates BACKWARDS: the heads-up display scored 0.44 and the real
	// menu 0.33, because the menu's last two entries were grouped separately and the
	// selection sat on them for most of the run, leaving the rest of the menu honestly
	// quiet.
	//
	// A list need not always carry a mark. What it must not do is carry several at once.
	SharedPenalty = 3
	// MinExclusive is how many marked frames are needed at all. A handful of coincidences
	// can satisfy any ratio.
	MinExclusive = 8
	// MinObservations is how long a candidate must have been watched. Baselines take a
	// few frames to exist, so a candidate seen briefly has not been tested.
	MinObservations = 10
	// MinTransfers is how many times the mark must move between members. Two, because one
	// move can be a coincidence of timing with something else changing, and because a
	// single transfer cannot distinguish a mark that moves from a mark that appeared.
	MinTransfers = 2
	// MinHolders is how many distinct members must have held the mark.
	MinHolders = 2
	// CandidateMatch is how much two extents must overlap to be the same candidate seen
	// again.
	CandidateMatch = 0.5
	// MemberMatch is how far apart two rows' centres may be, as a fraction of the
	// candidate's height, and still be the same member.
	MemberMatch = 0.25
)

// Evidence is what has been observed about one candidate list.
//
// Kept and reported rather than reduced to a score, because "why this one" is a question
// somebody will ask of a wrong answer, and a number cannot answer it.
type Evidence struct {
	// Bounds is where this candidate sits, which is also its identity over time.
	Bounds image.Rectangle
	Rows   int
	// Frames is how many observations this candidate has survived.
	Frames int
	// Exclusive counts frames with exactly one marked member; Shared, frames with more
	// than one; Quiet, frames with none.
	Exclusive, Shared, Quiet int
	// Unreadable counts frames in which too few members could be judged at all. Kept
	// apart from Quiet because one is a fact about the list and the other about the look.
	Unreadable int
	// Transfers counts the mark moving from one member to a different one.
	Transfers int
	// Holders is how many distinct members have held the mark.
	Holders int
	// Qualifies says this candidate behaves like a list somebody is choosing from.
	Qualifies bool
}

func (e Evidence) String() string {
	return fmt.Sprintf("rows %d, %d frames: %d exclusive, %d shared, %d quiet, "+
		"%d unreadable, %d transfers among %d holders", e.Rows, e.Frames, e.Exclusive,
		e.Shared, e.Quiet, e.Unreadable, e.Transfers, e.Holders)
}

// candidate is a tracked list.
type candidate struct {
	bounds     image.Rectangle
	rows       []image.Rectangle
	baseline   [][]desc
	seen       int
	excl       int
	shared     int
	quiet      int
	unreadable int
	last       int // the member marked on the last exclusive frame, or -1
	holders    map[int]bool
	moves      int
	stale      int
}

// Tracker watches candidate lists and decides which one is being used.
type Tracker struct {
	cands        []*candidate
	frames       int
	identifiedAt int
}

// NewTracker begins watching, knowing nothing.
func NewTracker() *Tracker { return &Tracker{identifiedAt: -1} }

// Observe takes one frame.s grouped rows and updates what is known about each list.
func (t *Tracker) Observe(img image.Image, rows []Row) {
	shades := make([]Shade, len(rows))
	for i, r := range rows {
		shades[i] = Measure(img, r.Bounds)
	}
	t.ObserveShades(rows, shades)
}

// ObserveShades is the same, for a caller that already measured.
//
// The live path measures where the frame is and carries numbers from there, so this is the
// door it comes in by. Order matches rows.
func (t *Tracker) ObserveShades(rows []Row, shades []Shade) {
	if len(shades) != len(rows) {
		return
	}
	t.frames++
	byGroup := map[int][]image.Rectangle{}
	shadeOf := map[int][]Shade{}
	order := []int{}
	for i, r := range rows {
		if _, ok := byGroup[r.Group]; !ok {
			order = append(order, r.Group)
		}
		byGroup[r.Group] = append(byGroup[r.Group], r.Bounds)
		shadeOf[r.Group] = append(shadeOf[r.Group], shades[i])
	}
	// One candidate per group per frame. Grouping emits several stacks whose extents
	// overlap — interleaved rows of different widths cover much the same rectangle — so
	// without this a single candidate absorbs two or three groups every frame and its
	// counts come out larger than the number of frames it was seen in. Measured: a
	// hundred-frame run produced candidates claiming a hundred and eighty observations.
	used := map[*candidate]bool{}
	for _, g := range order {
		members := byGroup[g]
		if len(members) < minStack {
			// A lone row cannot show a mark TRANSFERRING, so there is nothing here this
			// tracker can learn. The reader still judges such rows on their own.
			continue
		}
		if c := t.update(members, shadeOf[g], used); c != nil {
			used[c] = true
		}
	}
	for _, c := range t.cands {
		c.stale++
	}
	if t.identifiedAt < 0 {
		if g, _ := t.active(); g >= 0 {
			t.identifiedAt = t.frames
		}
	}
}

// update folds one frame's view of one list into whatever is already known about it.
func (t *Tracker) update(members []image.Rectangle, shades []Shade, used map[*candidate]bool) *candidate {
	ext := extent(members)
	c := t.match(ext, used)
	if c == nil {
		c = &candidate{bounds: ext, rows: members, last: -1, holders: map[int]bool{}}
		c.baseline = make([][]desc, len(members))
		t.cands = append(t.cands, c)
	}
	c.stale = 0
	c.bounds = ext
	// Members are matched by position, so a row the detector dropped this frame does not
	// shift every baseline onto a neighbour.
	mapped := make([]int, len(members))
	for i, m := range members {
		mapped[i] = c.memberAt(m)
	}

	descs := make([]desc, len(c.rows))
	have := make([]bool, len(c.rows))
	for i := range members {
		if mapped[i] < 0 {
			continue
		}
		descs[mapped[i]] = shades[i]
		have[mapped[i]] = true
	}

	marked, readable := c.marked(descs, have)
	switch {
	case !readable:
		// Too few of this list's members could be judged — the detector found some of
		// them this frame and not others, or their baselines do not exist yet. That is a
		// statement about the observation, not about the list, and counting it as "no
		// mark" would hold a list's own invisibility against it.
		//
		// Measured: counting these as quiet put the real menu at a 32% exclusivity ratio
		// and rejected it. [[ADR-006-unknown-is-not-false]].
		c.unreadable++
	case len(marked) == 0:
		c.quiet++
	case len(marked) == 1:
		c.excl++
		if c.last >= 0 && marked[0] != c.last {
			c.moves++
		}
		c.last = marked[0]
		c.holders[marked[0]] = true
	default:
		c.shared++
	}
	c.seen++

	// Baselines take only frames in which a member was NOT marked, the same rule the
	// reader uses and for the same reason.
	for i := range c.rows {
		if !have[i] || (len(marked) == 1 && marked[0] == i) {
			continue
		}
		c.baseline[i] = append(c.baseline[i], descs[i])
		if len(c.baseline[i]) > BaselineSamples {
			c.baseline[i] = c.baseline[i][len(c.baseline[i])-BaselineSamples:]
		}
	}
	return c
}

// marked lists the members standing away from their own unselected level this frame.
func (c *candidate) marked(descs []desc, have []bool) ([]int, bool) {
	var devs []float64
	idx := []int{}
	for i := range c.rows {
		if !have[i] || len(c.baseline[i]) < MinBaseline {
			continue
		}
		var worst float64
		for f := range Features {
			past := make([]float64, len(c.baseline[i]))
			for j, d := range c.baseline[i] {
				past[j] = d[f]
			}
			if v := math.Abs(descs[i][f] - median(past)); v > worst {
				worst = v
			}
		}
		devs = append(devs, worst)
		idx = append(idx, i)
	}
	if len(devs) < minStack {
		return nil, false
	}
	mid := median(devs)
	scale := math.Max(mad(devs, mid), MinSpread)
	var out []int
	for k, d := range devs {
		if d >= MinAbsolute && (d-mid)/scale >= SpatialK {
			out = append(out, idx[k])
		}
	}
	return out, true
}

// memberAt finds which of this candidate's known rows a region is, by position.
func (c *candidate) memberAt(r image.Rectangle) int {
	h := float64(c.bounds.Dy())
	if h <= 0 {
		return -1
	}
	best, bestD := -1, math.MaxFloat64
	for i, k := range c.rows {
		d := math.Abs(centreY(k) - centreY(r))
		if d < bestD {
			best, bestD = i, d
		}
	}
	if best < 0 || bestD > MemberMatch*h {
		return -1
	}
	return best
}

// match finds the candidate already being tracked at this extent.
func (t *Tracker) match(ext image.Rectangle, used map[*candidate]bool) *candidate {
	for _, c := range t.cands {
		if used[c] {
			continue
		}
		if overlap(c.bounds, ext) >= CandidateMatch {
			return c
		}
	}
	return nil
}

// Evidence reports what has been observed about every candidate, most convincing first.
func (t *Tracker) Evidence() []Evidence {
	out := make([]Evidence, 0, len(t.cands))
	for _, c := range t.cands {
		out = append(out, Evidence{
			Bounds: c.bounds, Rows: len(c.rows), Frames: c.seen,
			Exclusive: c.excl, Shared: c.shared, Quiet: c.quiet, Unreadable: c.unreadable,
			Transfers: c.moves, Holders: len(c.holders),
			Qualifies: c.qualifies(),
		})
	}
	return out
}

func (c *candidate) qualifies() bool {
	judged := c.seen - c.unreadable
	if judged <= 0 {
		return false
	}
	return judged >= MinObservations &&
		c.excl >= MinExclusive &&
		// Frames where several members stand out at once are evidence AGAINST: whatever
		// is happening there, it is not one thing being chosen.
		c.excl > SharedPenalty*c.shared &&
		c.moves >= MinTransfers && len(c.holders) >= MinHolders
}

// Active names the list being used, or abstains.
//
// The returned rectangle is where it is; callers restrict the reader to rows inside it. It is
// a place rather than a group number because group numbers do not survive a frame.
func (t *Tracker) Active() (image.Rectangle, string, bool) {
	i, why := t.active()
	if i < 0 {
		return image.Rectangle{}, why, false
	}
	return t.cands[i].bounds, why, true
}

func (t *Tracker) active() (int, string) {
	var passing []int
	for i, c := range t.cands {
		if c.qualifies() {
			passing = append(passing, i)
		}
	}
	switch len(passing) {
	case 0:
		return -1, fmt.Sprintf("no list has been seen handing a mark between its members "+
			"(%d candidates watched over %d frames)", len(t.cands), t.frames)
	case 1:
		c := t.cands[passing[0]]
		return passing[0], fmt.Sprintf("one list hands a mark around: %s",
			Evidence{Rows: len(c.rows), Frames: c.seen, Exclusive: c.excl,
				Shared: c.shared, Quiet: c.quiet, Unreadable: c.unreadable,
				Transfers: c.moves, Holders: len(c.holders)}.String())
	}
	// Two lists both behaving like menus is not a reason to pick one. It is a reason to
	// say so: picking would be guessing, and a guess that happens to be right teaches
	// nothing about when to trust the next one.
	return -1, fmt.Sprintf("%d lists each hand a mark between their members; "+
		"the active one is not determined by this evidence", len(passing))
}

// IdentifiedAt is the observation on which a single list first qualified, or -1.
func (t *Tracker) IdentifiedAt() int { return t.identifiedAt }

// extent is the rectangle a set of rows occupies.
func extent(rows []image.Rectangle) image.Rectangle {
	out := rows[0]
	for _, r := range rows[1:] {
		out = out.Union(r)
	}
	return out
}
