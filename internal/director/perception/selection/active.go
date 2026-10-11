package selection

import (
	"fmt"
	"image"
	"math"
	"sort"
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
	// NavMargin is how far the leading candidate must out-evidence the next one on
	// input-attributed transfers before it is named the active list. Twice as many,
	// because a factor of two is a difference a coincidence does not easily produce
	// while two lists that both genuinely respond will not clear it - and two lists that
	// both respond is a case that must stay ambiguous.
	NavMargin = 2
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
	// Prompted and Unprompted split Transfers by whether a navigation input was
	// observed within NavWindow observations before the move was SEEN.
	//
	// THE INPUT LABELS A TRANSFER; IT NEVER CREATES ONE. Both counters are increments
	// of a move that was already measured in the pixels, and a candidate with no
	// visually observed transfer has nothing for an input to be attributed to however
	// much the person presses. That is the whole of what keeps this perception rather
	// than inference from intent: an input cannot manufacture a visual outcome here,
	// because it is only ever consulted about moves the shading already proved.
	Prompted, Unprompted int
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

// Judged is the frames this candidate could actually be assessed on.
//
// The threshold MinObservations is compared against THIS and not against Frames, and the
// difference is the whole of [[ADR-006-unknown-is-not-false]]: a list the detector kept
// finding and nobody could measure has been looked at many times and tested zero times. A
// report that showed Frames alone would make a candidate look well-observed at the exact
// moment it was not observed at all.
func (e Evidence) Judged() int { return e.Frames - e.Unreadable }

// Shortfall is one requirement a candidate does not meet, and by how much.
type Shortfall struct {
	// Requirement is a stable key, for a reader that is counting rather than reading.
	Requirement string `json:"requirement"`
	// Have and Want are the comparison that failed, in the units of that requirement.
	Have int `json:"have"`
	Want int `json:"want"`
	// Why is the same thing for a person, in one clause.
	Why string `json:"why"`
}

// Unmet lists every requirement this candidate falls short of, and is empty when it
// qualifies.
//
// # Why this exists rather than a bool
//
// Experiment-025 watched a player make ninety-two directional presses at a menu and ended
// with "no list has been seen handing a mark between its members (8 candidates watched over
// 106 frames)". That sentence is true and it does not say which of five different things
// went wrong, and the five call for completely different responses:
//
//	a candidate nobody could measure      -> a detector or capture finding
//	a candidate watched for three frames  -> candidate identity is churning
//	a candidate with no marked frames     -> no mark is visible in the shading at all
//	a candidate with several marked at once -> this is a heads-up display, correctly refused
//	marks but no transfer                 -> the mark is not moving, or members are not matched
//
// Reading them off the counters by hand is what had to be done after that run, from geometry
// regrouped by somebody else's script. So the tracker says it.
//
// # Why the predicate lives here
//
// `candidate.qualifies` delegates to this, so there is exactly one statement of the rule.
// The alternative — a reporting copy beside the deciding copy — is a rule that drifts, and
// a report that disagrees with the decision it describes is worse than no report.
func (e Evidence) Unmet() []Shortfall {
	var out []Shortfall
	judged := e.Judged()
	if judged <= 0 {
		return []Shortfall{{
			Requirement: "judged_frames", Have: 0, Want: MinObservations,
			Why: "no frame of this candidate could be judged at all",
		}}
	}
	if judged < MinObservations {
		out = append(out, Shortfall{
			Requirement: "judged_frames", Have: judged, Want: MinObservations,
			Why: "has not been watched long enough to have been tested",
		})
	}
	if e.Exclusive < MinExclusive {
		out = append(out, Shortfall{
			Requirement: "exclusive_marks", Have: e.Exclusive, Want: MinExclusive,
			Why: "too few frames had exactly one member standing out",
		})
	}
	if e.Exclusive <= SharedPenalty*e.Shared {
		out = append(out, Shortfall{
			Requirement: "exclusive_over_shared", Have: e.Exclusive,
			Want: SharedPenalty*e.Shared + 1,
			Why: "several members stood out at once too often for one thing to be " +
				"being chosen here",
		})
	}
	if e.Transfers < MinTransfers {
		out = append(out, Shortfall{
			Requirement: "transfers", Have: e.Transfers, Want: MinTransfers,
			Why: "the mark was never seen moving from one member to a different one",
		})
	}
	if e.Holders < MinHolders {
		out = append(out, Shortfall{
			Requirement: "holders", Have: e.Holders, Want: MinHolders,
			Why: "only one member has ever held the mark",
		})
	}
	return out
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
	prompted   int
	unprompted int
	stale      int
}

// Tracker watches candidate lists and decides which one is being used.
type Tracker struct {
	cands        []*candidate
	frames       int
	identifiedAt int
	// navRecent says a navigation input is close enough in TIME to this observation
	// for a transfer seen now to be attributable to it.
	//
	// Decided by the caller and not here, because the bound is a duration and this
	// package has no clock. It held the decay itself and counted in OBSERVATIONS, which
	// looked cadence-independent and is the opposite: two observations is 132ms on a
	// 66ms recording and 4.6s on a 2.3s live cycle, against a menu repaint that takes a
	// fixed couple of hundred milliseconds whatever the sampling rate. The short end
	// credited 4 of a real menu's 31 transfers; the long end would credit an unrelated
	// list's.
	navRecent bool
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
//
// Equivalent to ObserveNavigated with no input observed, which is the honest default: a
// caller that cannot say what the person did must not have its silence read as "nobody
// pressed anything". With no input evidence anywhere, two competing lists stay ambiguous,
// which is what they were before this existed.
func (t *Tracker) ObserveShades(rows []Row, shades []Shade) {
	t.ObserveNavigated(rows, shades, false)
}

// ObserveNavigated takes one frame's rows and whether the person was navigating recently
// enough for a transfer seen now to be attributable to it.
//
// `navRecent` is ONE BIT and nothing more: no key identity, no device, no direction, not even
// a count. Direction is deliberately absent — knowing that "down" was pressed and that the
// mark moved down would let the input predict the outcome, and an outcome that was predicted
// is not an outcome that was observed. All this asks is whether the person was navigating at
// all while a list handed its mark along.
//
// WHAT "RECENTLY ENOUGH" MEANS IS THE CALLER'S, and deliberately so. It is a statement about
// how long a screen takes to repaint after a press, which is a duration; this package sees
// frames and has no clock. See observe.MaxResponseLatency.
func (t *Tracker) ObserveNavigated(rows []Row, shades []Shade, navRecent bool) {
	if len(shades) != len(rows) {
		return
	}
	// BEFORE the frame is folded, so a transfer seen on THIS observation can be
	// attributed to an input the caller judged recent enough to have caused it.
	t.navRecent = navRecent
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
			// Labelled, not caused. See Evidence.Prompted.
			if t.navRecent {
				c.prompted++
			} else {
				c.unprompted++
			}
		}
		c.last = marked[0]
		c.holders[marked[0]] = true
	default:
		c.shared++
	}
	c.seen++

	// Baselines take only frames in which a member was NOT marked, the same rule the
	// reader uses and for the same reason.
	//
	// KNOWN GAP, MEASURED AND LEFT ALONE. On a frame where exclusivity was NOT
	// established - quiet, shared, or too few members judged - this teaches every present
	// member that whatever it looked like is its UNSELECTED level, including the member
	// that was in fact selected. On a list of near-identical rows that poisons the
	// selected row's baseline during the opening frames, and from then on it reads as
	// marked whenever the mark LEAVES it, so every later frame has two marked members and
	// is refused as shared. Measured on a synthetic five-row list, by how long the mark
	// dwells on a row:
	//
	//	dwell 1 or 3    218 exclusive of 240, 217 transfers
	//	dwell 10          0 exclusive, 198 shared, 0 transfers
	//	dwell 30          0 exclusive,  63 shared, 0 transfers
	//
	// THE OBVIOUS FIX WAS TRIED AND REJECTED ON EVIDENCE. Declining to teach the frame's
	// most conspicuous member on a non-exclusive frame repaired all of that -- 0 shared
	// at every dwell -- and cost 88% of the selection reads on 864 real frames: 155
	// frames naming an entry fell to 19, and identification slipped from look 28 to 294.
	// Narrowing it to members with no baseline yet did not recover them.
	//
	// The reason is the difference between the fixtures and a screen. A drawn list has
	// five identical rows and one bright one, so the brightest member of an unreadable
	// frame IS the selected one. A real list's rows differ in weight, colour and content,
	// so the brightest row is usually just the brightest row -- and withholding its
	// samples starves a baseline that was needed, on a game where most frames are
	// unreadable anyway.
	//
	// So the defect is real and its only known repair is worse. It needs a discriminator
	// that is not frame-local brightness. Recorded in Passive-Observation.md.
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
//
// Most convincing first means: qualifying candidates, then the ones tested on the most
// frames. Sorted here because this comment claimed an order the code did not produce, and a
// report that shows the first few of a long list has to be showing the right few. No caller's
// decision depends on the order — `active` scans all of them — so this is a reporting
// guarantee only.
func (t *Tracker) Evidence() []Evidence {
	out := make([]Evidence, 0, len(t.cands))
	for _, c := range t.cands {
		out = append(out, c.evidence())
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Qualifies != out[j].Qualifies {
			return out[i].Qualifies
		}
		return out[i].Judged() > out[j].Judged()
	})
	return out
}

// evidence is this candidate's record, verdict included.
//
// One builder for the decision and for the report, so the two cannot disagree. Frames where
// several members stand out at once are evidence AGAINST — whatever is happening there, it is
// not one thing being chosen — and Evidence.Unmet states that along with the rest.
func (c *candidate) evidence() Evidence {
	e := Evidence{
		Bounds: c.bounds, Rows: len(c.rows), Frames: c.seen,
		Exclusive: c.excl, Shared: c.shared, Quiet: c.quiet, Unreadable: c.unreadable,
		Transfers: c.moves, Holders: len(c.holders),
		Prompted: c.prompted, Unprompted: c.unprompted,
	}
	e.Qualifies = len(e.Unmet()) == 0
	return e
}

func (c *candidate) qualifies() bool { return c.evidence().Qualifies }

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
	// SEVERAL LISTS QUALIFY, SO ASK WHICH ONE RESPONDS.
	//
	// Measured, on 864 real frames of a game's pause menu: the menu qualifies and so does
	// the track-information panel beside it, because both genuinely hand a mark between
	// their rows over time. Geometry cannot separate them and never will — both are real
	// lists, and one of them is simply not the one being used.
	//
	// What separates them is that one of them responds. If the mark in a list moves while
	// the person is navigating, and the mark in another moves on its own schedule, the
	// first is the list the navigation is reaching.
	//
	// THE INPUT DOES NOT PROVE THE MOVE. Every transfer counted here was already measured
	// in the pixels by the exclusivity rule above; the input only says which of several
	// independently observed moves happened while somebody was pressing something. A list
	// with no visually observed transfer cannot be chosen here at any amount of input,
	// and an input with no visually observed transfer chooses nothing.
	if i, why := t.responding(passing); i >= 0 {
		return i, why
	}
	// Two lists both behaving like menus is not a reason to pick one. It is a reason to
	// say so: picking would be guessing, and a guess that happens to be right teaches
	// nothing about when to trust the next one.
	return -1, fmt.Sprintf("%d lists each hand a mark between their members, and what "+
		"the person pressed does not single one of them out; the active one is "+
		"not determined by this evidence", len(passing))
}

// responding names the candidate whose transfers followed observed navigation, or abstains.
//
// Abstains in every case where the evidence is not decisive, and the cases are named rather
// than collapsed into one silence, because they call for different responses: nobody pressed
// anything, nobody's presses landed anywhere, or two lists both answered.
func (t *Tracker) responding(passing []int) (int, string) {
	best, second, bestAt := 0, 0, -1
	for _, i := range passing {
		n := t.cands[i].prompted
		switch {
		case n > best:
			best, second, bestAt = n, best, i
		case n > second:
			second = n
		}
	}
	switch {
	case best < MinTransfers:
		// Nothing was attributable. Either no navigation was observed at all — a
		// session with no input source, or a person who pressed nothing — or the
		// presses never coincided with a mark moving anywhere.
		return -1, ""
	case best < NavMargin*second || (second > 0 && best == second):
		// More than one list answered. This is the case that must stay ambiguous: it
		// is exactly when picking would be guessing.
		return -1, ""
	}
	c := t.cands[bestAt]
	return bestAt, fmt.Sprintf("one of %d qualifying lists responds to navigation: "+
		"%d of its %d transfers followed an observed input, against %d for the next "+
		"(%s)", len(passing), c.prompted, c.moves, second,
		Evidence{Rows: len(c.rows), Frames: c.seen, Exclusive: c.excl,
			Shared: c.shared, Quiet: c.quiet, Unreadable: c.unreadable,
			Transfers: c.moves, Holders: len(c.holders)}.String())
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
