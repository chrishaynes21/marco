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
	//
	// NO LONGER CONSULTED FOR IDENTITY. Kept because it is the published bound of a rule
	// this package used from the start, and because the measurement that retired it is
	// worth a sentence where a reader will look for it: matching by extent made every
	// candidate that ever qualified a bucket. See Tracker.match.
	CandidateMatch = 0.5
	// RowCorrespondence is what share of a frame's rows must be rows this candidate
	// ALREADY KNOWS, counted as distinct members, for the frame to be the same list seen
	// again. Half, because a detector routinely finds only part of a list.
	RowCorrespondence = 0.5
	// RowMatch is how far two rows' centres may be apart, as a fraction of the SHORTER of
	// the two heights, and still be the same row found again.
	//
	// The shorter height is what makes this scale-free, and that is the whole point. A
	// small group nested inside a tracked extent used to match it completely; measured
	// against the smaller row it is a different row, because a sliver's rows are a
	// fraction of a member's height from each other and most of a member's height from
	// most members.
	RowMatch = 0.5
	// RowSideMatch is how much of the narrower row's width the two must share. A list
	// beside another at the same heights is not the same list, and a rule that reads only
	// centre heights cannot say so.
	RowSideMatch = 0.5
	// MaxUnmatched is how many observations a candidate may go without being found again
	// before it is retired.
	//
	// IN OBSERVATIONS AND NOT IN TIME, unlike the response window, and the difference is
	// real: a response latency is a fact about how long a screen takes to repaint, where
	// this is a fact about the matching process - how many looks in a row failed to find
	// this thing. Nothing about a candidate changes between looks that are not taken.
	//
	// It exists because the population was unbounded. Matching by extent kept it down by
	// absorbing everything into a few buckets, which is the defect; matching by rows
	// refuses far more, and an earlier experiment with a stricter rule accumulated
	// candidates without bound and slowed the replay several times over. Retiring is the
	// half of that trade this rule has to pay for itself.
	MaxUnmatched = 30
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
	// ID names this candidate for as long as it is tracked, and is never reused.
	ID int
	// Bounds is where this candidate sits. It is NOT its identity: see candidate.id.
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
	// QualifiedAt is the observation on which this candidate first met every
	// requirement, or -1. Tracker.IdentifiedAt says WHEN a list was named; this says
	// WHICH ONE got there first, which is a different question and the one that matters
	// when the wrong list was named.
	//
	// It matters because `active` publishes a sole qualifier with no comparison to
	// anything. First past the post wins, so the order candidates cross the bar in IS
	// the decision, and a report that cannot show that order cannot explain the outcome.
	QualifiedAt int
	// Hull is the union of every extent this candidate has ever had, and Rehomed how
	// many times its extent changed area by more than double or less than half.
	//
	// THE TEMPORAL-IDENTITY DIAGNOSTIC, and it cannot be recovered from Bounds. A
	// candidate's bounds are overwritten by every frame's extent, so the final table
	// shows wherever it ended up — and candidates are matched by overlap normalised on
	// the SMALLER area, which means a small group nested inside a tracked candidate's
	// extent matches it completely. A hull far larger than the bounds, or a Rehomed
	// above zero, is one tracked "list" that has been several different rectangles.
	Hull    image.Rectangle
	Rehomed int
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
	// id names this candidate for as long as it is tracked, and is never reused.
	//
	// A rectangle was doing this job and could not: callers asking "is this still the list
	// I was looking at" compared published extents, and a list's extent moves whenever its
	// rows are re-found a pixel lower. An identity that is decided by row correspondence
	// has to be REPORTED as an identity, or every caller re-derives it from geometry and
	// gets a different answer. Never reused, so a retired candidate cannot be mistaken for
	// the one that takes its place.
	id         int
	bounds     image.Rectangle
	rows       []image.Rectangle
	baseline   [][]desc
	seen       int
	excl       int
	shared     int
	quiet      int
	unreadable int
	last       int // the member marked on the last exclusive frame, or -1
	// cold is how many observations each member has gone without being found again. A
	// list's rows age for the same reason the list does; see forgetColdMembers.
	cold []int
	// entry is each member's own identity, never reused within this candidate, and
	// nextEntry the one the next member will be given.
	//
	// # Why an index was not enough
	//
	// Because a member's index is a position in a slice, and the slice changes. Forgetting
	// a member renumbers every member after it, so "the mark was on entry 2" stops being
	// about the same row — and `last`, `holders` and everything a caller is told have to be
	// renumbered in lockstep or they quietly describe a different row. That renumbering was
	// written, and a test had to be built to prove it did not invent a transfer nothing
	// moved, which is a sign the design was wrong rather than that the renumbering was
	// clever.
	//
	// It is also what the reader needs. A reader given rows by position loses its whole
	// history whenever the count changes — six rows this frame and seven the next, and
	// every baseline it has is dropped — and on 864 real frames that is most of them. An
	// identity is the only thing that lets "this row's unselected level" survive a frame in
	// which a different row was missing.
	entry      []int
	nextEntry  int
	holders    map[int]bool
	moves      int
	prompted   int
	unprompted int
	stale      int
	// qualifiedAt is the observation this candidate first met every requirement, or -1.
	qualifiedAt int
	// hull is the union of every extent it has had, and rehomed how many times that
	// extent changed area by more than a factor of two. See Evidence.Hull.
	hull    image.Rectangle
	rehomed int
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
	// nextID is the name the next candidate will be given. Monotonic, never reused.
	nextID int
	// retired counts candidates dropped for having gone MaxUnmatched observations
	// without being found again. Reported, because a population that is being pruned and
	// one that was never crowded look the same from a count of the survivors.
	retired int
	// navSeen says navigation has been observed at least once in this session.
	//
	// It decides whether responsiveness is a REQUIREMENT or merely a tie-break, which is
	// the difference between a session somebody is navigating and a session with no input
	// source at all. A caller that cannot see input must not have its silence read as "the
	// person pressed nothing here", so where nothing was ever observed the rule falls back
	// to what it was: a sole qualifier is named. See active().
	navSeen bool
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
	if navRecent {
		t.navSeen = true
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
	t.age()
	if t.identifiedAt < 0 {
		if g, _ := t.active(); g >= 0 {
			t.identifiedAt = t.frames
		}
	}
}

// age ages every candidate by one observation and retires the ones nothing has found again.
//
// # Why retiring is part of this repair and not a separate tidy-up
//
// Because the population was unbounded and nothing had noticed, and the reason nothing had
// noticed is the defect itself: matching by extent absorbed almost everything into a handful
// of buckets, so the count stayed small by being WRONG. Matching by rows refuses far more,
// which is the point of it, and an earlier experiment with a stricter rule accumulated
// candidates until the replay slowed several times over. A rule that refuses more has to say
// what becomes of what it refused.
//
// # And it is where evidence preservation is enforced
//
// A retired candidate's evidence goes with it. It is not merged into whatever is tracked at
// that place next, not consulted again, and not carried into the one that replaces it - the
// struct is dropped and a later list at the same place starts from nothing and serves its full
// MinObservations before it can be believed. That is the guarantee a bucket could not make: a
// bucket's counters were a sum over unrelated rectangles, and the only thing keeping an old
// list's transfers out of a new list's verdict was that nobody had asked.
//
// The cost is stated plainly: a list that goes away for MaxUnmatched observations and comes
// back has to earn its verdict again. On a screen that genuinely changed - a submenu opening,
// a menu closing - that is not a cost, it is the correct answer. On a list the detector simply
// loses for a long stretch it is lost evidence, and `Retired` is reported so the cost is
// countable rather than invisible.
func (t *Tracker) age() {
	kept := t.cands[:0]
	for _, c := range t.cands {
		c.stale++
		if c.stale > MaxUnmatched {
			t.retired++
			continue
		}
		kept = append(kept, c)
	}
	// The tail holds pointers the slice no longer owns; nil them so a retired candidate's
	// evidence is actually unreachable rather than merely unlisted.
	for i := len(kept); i < len(t.cands); i++ {
		t.cands[i] = nil
	}
	t.cands = kept
}

// Retired is how many candidates have been dropped for not being found again.
func (t *Tracker) Retired() int { return t.retired }

// update folds one frame's view of one list into whatever is already known about it.
func (t *Tracker) update(members []image.Rectangle, shades []Shade, used map[*candidate]bool) *candidate {
	ext := extent(members)
	c := t.match(members, used)
	if c == nil {
		t.nextID++
		c = &candidate{id: t.nextID, bounds: ext, rows: members, last: -1,
			holders: map[int]bool{}, qualifiedAt: -1, hull: ext}
		c.baseline = make([][]desc, len(members))
		c.cold = make([]int, len(members))
		c.entry = make([]int, len(members))
		for i := range members {
			c.nextEntry++
			c.entry[i] = c.nextEntry
		}
		t.cands = append(t.cands, c)
	}
	c.stale = 0
	// Members are matched by position, so a row the detector dropped this frame does not
	// shift every baseline onto a neighbour.
	//
	// ATTRIBUTION IS TOLERANT AND GEOMETRY IS STRICT, and the two halves of that are
	// separate statements for measured reasons.
	//
	// Attribution asks which member a row is so a shade can be scored against that
	// member's baseline. It is `memberAt`: the nearest member, within MemberMatch of the
	// list's own height. It has to stay tolerant because A SELECTED ROW IS REDRAWN. A game
	// draws the entry the person is on with a highlight, and the detector returns a box of
	// a different height for it — so the one row whose shade matters is the one least like
	// the member it belongs to. Measured, with attribution made strict as well: the real
	// menu's candidate recorded ONE transfer over 382 judged frames with 205 of them quiet,
	// against 31 presses in the recording, and coverage fell to 20 of 829 frames. Strictness
	// here does not refuse a wrong row, it refuses the selected one.
	//
	// Geometry is the other half. A member moves only to a row that `rowAt` called the same
	// row — a scale-free test against the SHORTER of the two, not a nearest-within-a-
	// fraction-of-the-whole — because a member that moves to wherever its nearest row was
	// drags the list after it. Measured, with the tolerant rule moving members: published
	// extents grew to 0.583..1.000 carrying twelve and thirteen entries where the menu has
	// nine, the reader held a stale claim over 50 frames against 3, and its conditional
	// accuracy fell from 93% to 51%. The bucket re-formed one member at a time.
	//
	// So a redrawn row is still read as its member, and it is not allowed to move it.
	// EACH MEMBER RE-FOUND AS THE SAME ROW moves to where it was just seen, and this is
	// what lets identity follow a list without becoming a bucket.
	//
	// Measured: frozen at birth, the real menu's candidate was matched on 389 of 864 frames
	// and did not qualify until observation 833, because a list's detected rows drift — a
	// row is found a few pixels lower, or with its label attached and so half as tall again
	// — and correspondence against a rectangle from four hundred frames ago decays until
	// the list stops being itself.
	//
	// It is still not a bucket, and the difference from matching by extent is the rate. A
	// member may only move to a row that was already the same row — within RowMatch of its
	// own height — so the list can migrate at most a fraction of a row per observation, and
	// only by being continuously re-found all the way. An extent could be replaced outright
	// by anything nested inside it, which is how a tracked list became a sliver in one frame.
	for i := range c.cold {
		c.cold[i]++
	}
	for _, m := range members {
		if j := c.rowAt(m); j >= 0 {
			c.rows[j] = m
			c.cold[j] = 0
		}
	}
	// AND THE LIST MAY GAIN AND LOSE ROWS, which is the rest of what it means for a
	// candidate to have a lifecycle. See admit and forgetColdMembers.
	c.admit(members)
	c.forgetColdMembers()
	// THE EXTENT IS THE MEMBERS', NOT THIS FRAME'S GROUP'S.
	//
	// It was the group's, and that is a second place the frame got to overwrite the list.
	// Callers restrict the reader to rows inside the published extent, so on a frame where
	// the detector found three rows of a nine-row menu the reader was handed three rows and
	// could not read any of them: too few to have a middle to deviate from. Measured, with
	// identity by rows and the extent still coming from the group, 263 of 315 published
	// frames were unreadable for that reason.
	//
	// The members ARE the identity now, so their extent is the list's extent, and it does
	// not shrink to whatever was found this time. RECORDED BEFORE IT MOVES, because the old
	// value is about to be gone: a hull far larger than the extent, or a rehoming, is one
	// tracked list that has been several different rectangles. See Evidence.Hull.
	own := extent(c.rows)
	if a, b := area(c.bounds), area(own); a > 0 && b > 0 && (b*2 < a || b > a*2) {
		c.rehomed++
	}
	c.hull = c.hull.Union(own)
	c.bounds = own

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
		// BY IDENTITY AND NOT BY POSITION. `marked` is an index into this frame's view of
		// the member slice; `last` is the entry the mark was on. Comparing the two as
		// positions reads a transfer whenever the slice changed under them.
		at := c.entry[marked[0]]
		if c.last >= 0 && at != c.last {
			c.moves++
			// Labelled, not caused. See Evidence.Prompted.
			if t.navRecent {
				c.prompted++
			} else {
				c.unprompted++
			}
		}
		c.last = at
		c.holders[at] = true
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
	// WHEN THIS CANDIDATE CROSSED THE BAR, recorded for every candidate and not only for
	// the one that won. `active` publishes a sole qualifier with no comparison to anything,
	// so the ORDER candidates cross in is the decision — and after the fact the counters
	// alone cannot say who was first.
	//
	// Here rather than in a loop over every candidate per frame, because a candidate's
	// verdict can only change when its counters do, and its counters only change here. The
	// loop was the same answer at O(candidates) per frame with an allocation each.
	if c.qualifiedAt < 0 && c.qualifies() {
		c.qualifiedAt = t.frames
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

// match finds the candidate already being tracked whose ROWS these rows are.
//
// # Why not the extent
//
// Because matching by extent made every tracked candidate a bucket. Measured on 864 real
// frames, every candidate that ever qualified had a hull - the union of every extent it had
// held - spanning the whole screen. `overlap` divides the intersection by the SMALLER area,
// which is right in grouping, where a row contains its label and the label is a duplicate of
// the row. Here it meant any group nested inside a tracked extent scored 1.0 and was taken for
// the same list; the candidate's bounds were then overwritten by the nested extent, so a
// tracked "list" was several different rectangles and every counter the decision reads was a
// sum over unrelated things. Drifting was what EARNED a candidate its verdict: a bucket sees a
// different member marked whenever it absorbs a different group.
//
// Two geometric repairs were tried on that corpus and both were rejected on measurement.
// Intersection over union fragments the real menu, whose grouping genuinely varies between
// three and seven rows frame to frame. A scale ratio on the extents improved coverage,
// accuracy and the reader's own score while taking correct-list identification to ZERO.
// [[ADR-164-a-list-is-the-same-list-when-its-rows-are]].
//
// # What this asks instead
//
// Whether these are the rows this candidate already knows, in the places it knows them. That
// is a sharpened form of the question `memberAt` asks of a single row to attribute a mark,
// which is why the idea was reachable at all: the member set is already the candidate's
// identity for every other purpose, and only matching had been using a rectangle.
//
// # The asymmetry is deliberate
//
// The share is of THIS FRAME'S rows, not of the candidate's members. A frame showing three
// rows of a six-row list is a detector dropout, which happens constantly and is accounted for
// everywhere else in this file. A frame showing three rows the candidate knows and nine it
// does not is a different structure - and that direction is the one that grows buckets, so it
// is the direction the share refuses.
//
// # Best, not first
//
// The old rule returned the first extent that overlapped enough, which made the answer depend
// on the order candidates happened to be created in. That order is frame order, and frame
// order is the sampling cadence. Taking the most corresponding candidate instead is the same
// answer at any stride.
func (t *Tracker) match(members []image.Rectangle, used map[*candidate]bool) *candidate {
	var best *candidate
	bestN := 0
	need := RowCorrespondence * float64(len(members))
	for _, c := range t.cands {
		if used[c] {
			continue
		}
		// A floor of two distinct rows was written here beside the share and then
		// removed: the mutation gate could not reach it. A group smaller than
		// minStack is never offered to this tracker at all, so the smallest frame
		// here has three rows and half of three already demands two.
		if n := c.corresponding(members); float64(n) < need {
			continue
		} else if n > bestN {
			best, bestN = c, n
		}
	}
	return best
}

// corresponding counts how many DISTINCT rows of this candidate appear among `members`.
//
// Distinct, because the count is a correspondence and not a tally: a sliver's three rows all
// landing in one member's neighbourhood is one row found again, not three, and counting them
// three times is exactly how a nested group used to pass for the list around it.
func (c *candidate) corresponding(members []image.Rectangle) int {
	var seen int
	hit := make([]bool, len(c.rows))
	for _, m := range members {
		if i := c.rowAt(m); i >= 0 && !hit[i] {
			hit[i] = true
			seen++
		}
	}
	return seen
}

// admit takes in a row that is plainly the NEXT ROW of this list and was not a member yet.
//
// # Why a list has to be allowed to grow
//
// Because the member set was fixed at birth, and a list is first seen through whatever the
// detector happened to find on one frame. On 864 real frames of a nine-entry menu the tracked
// candidate carried between three and seven members and never more, so four of the nine
// entries could not be read however well everything else worked: the published extent did not
// contain them, and the reader is given the rows inside the extent. The same gap refuses a
// list that SCROLLED - every row moves one pitch, every member follows the row that was below
// it, and the row that has just come into view at the top is a row no member can be.
//
// # Why this is not how a bucket forms
//
// A bucket formed by absorbing whatever was nested inside a rectangle. This admits a row only
// where the list's OWN regular spacing says its next row goes: one median pitch from the
// nearest member, to within the tolerance grouping itself uses. Accreting the screen one row at
// a time would mean the screen were periodic at this list's pitch all the way across, which is
// what it means for the list to continue.
//
// A second test, that the row must not sit on top of the member it would duplicate, was written
// here and removed: the mutation gate could not reach it. A row overlapping a member by half its
// own area is the SAME ROW as that member by `sameRow`, and `rowAt` has already claimed it two
// lines above.
//
// The pitch is measured from the members, so it is the list's own measure and not a constant.
func (c *candidate) admit(members []image.Rectangle) {
	pitch := c.pitch()
	if pitch <= 0 {
		return
	}
	for _, m := range members {
		if c.rowAt(m) >= 0 {
			continue
		}
		near, d := -1, math.MaxFloat64
		for i, k := range c.rows {
			if v := math.Abs(centreY(k) - centreY(m)); v < d {
				near, d = i, v
			}
		}
		if near < 0 || math.Abs(d-pitch) > pitchTolerance*pitch {
			continue
		}
		c.rows = append(c.rows, m)
		c.baseline = append(c.baseline, nil)
		c.cold = append(c.cold, 0)
		c.nextEntry++
		c.entry = append(c.entry, c.nextEntry)
	}
}

// pitch is the median spacing between this candidate's members, taken in order down the screen.
func (c *candidate) pitch() float64 {
	if len(c.rows) < 2 {
		return 0
	}
	ys := make([]float64, 0, len(c.rows))
	for _, r := range c.rows {
		ys = append(ys, centreY(r))
	}
	sort.Float64s(ys)
	gaps := make([]float64, 0, len(ys)-1)
	for i := 1; i < len(ys); i++ {
		gaps = append(gaps, ys[i]-ys[i-1])
	}
	return median(gaps)
}

// forgetColdMembers drops members nothing has found again for MaxUnmatched observations.
//
// # Why members age as well as whole candidates
//
// Because a member that is never found again still decides things. Its rectangle is part of
// the extent the belief publishes, so the list keeps claiming ground it no longer occupies and
// the reader keeps being handed rows from there; its baseline is still consulted; and it is
// still counted among the holders of the mark. A list that scrolled away from a row, or lost an
// entry, would otherwise stay anchored to it for the rest of the session.
//
// AND BECAUSE ITS EVIDENCE MUST NOT TRANSFER. The baseline goes with the member, so a row that
// appears at that place later is a new member with nothing yet learned about it rather than one
// inheriting an unselected level measured from a row that is gone. That is the guarantee
// Tracker.age makes for whole candidates, one level down - and here it needs saying twice,
// because `last` and `holders` are INDICES into this slice. Renumbering without moving them
// would compare this frame's member against a different row and read a transfer that nothing
// moved, which is a false transition offered to the graph.
//
// A candidate is never reduced below minStack members this way: fewer than that is not a list
// at all, and the honest end for one is to stop being found and be retired whole.
func (c *candidate) forgetColdMembers() {
	// How many may go at all, worked out before any do: a candidate reduced below minStack
	// members is not a list, and the honest end for one is to stop being found and be
	// retired whole. Counted here rather than tested inside the loop, where "how many are
	// left" is not yet known and the obvious expression for it is wrong.
	droppable := len(c.rows) - minStack
	if droppable <= 0 {
		return
	}
	keep := make([]image.Rectangle, 0, len(c.rows))
	base := make([][]desc, 0, len(c.rows))
	cold := make([]int, 0, len(c.rows))
	entry := make([]int, 0, len(c.rows))
	gone := map[int]bool{}
	var dropped int
	for i := range c.rows {
		if c.cold[i] > MaxUnmatched && dropped < droppable {
			dropped++
			gone[c.entry[i]] = true
			continue
		}
		keep = append(keep, c.rows[i])
		base = append(base, c.baseline[i])
		cold = append(cold, c.cold[i])
		entry = append(entry, c.entry[i])
	}
	if len(keep) == len(c.rows) {
		return
	}
	c.rows, c.baseline, c.cold, c.entry = keep, base, cold, entry
	// NOTHING IS RENUMBERED, because nothing is numbered by position. `last` and
	// `holders` hold entry identities, so a member leaving the slice moves no other
	// member's name and cannot turn this frame's reading into a transfer nothing made.
	// All that is left is to forget the one that went.
	if gone[c.last] {
		// The member that was holding the mark is gone, so there is nothing for a
		// transfer to be measured FROM and the next exclusive frame starts a holding
		// rather than continuing one.
		c.last = -1
	}
	for id := range gone {
		delete(c.holders, id)
	}
}

// memberAt finds which member of this candidate a region should be SCORED AS, by position.
//
// Deliberately more tolerant than `rowAt`, and the comment in Tracker.update says why: the
// selected row is redrawn, so the row whose shade decides everything is the row least like
// the member it belongs to. Nearest member within a fraction of the list's own height.
//
// That fraction is of the LIST, which only became a local bound when the list's extent became
// its members' extent. While the extent was whatever group matched this frame — up to the
// whole screen — a quarter of it was most of the screen, and this rule attributed almost
// anything to something.
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

// rowAt finds which of this candidate's known rows is the same row as r, found again, or -1.
//
// Nearest by centre among those that could be it at all, so a row between two members goes to
// the closer one rather than to whichever was declared first.
func (c *candidate) rowAt(r image.Rectangle) int {
	best, bestD := -1, math.MaxFloat64
	for i, k := range c.rows {
		if !sameRow(k, r) {
			continue
		}
		if d := math.Abs(centreY(k) - centreY(r)); d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

// sameRow reports whether two rectangles are one row of a list, seen twice.
//
// Both tolerances are fractions of the SMALLER of the two, which is what makes this refuse a
// nested sliver: a rectangle inside another is not near its centre in units of its own height.
func sameRow(a, b image.Rectangle) bool {
	h := math.Min(float64(a.Dy()), float64(b.Dy()))
	w := math.Min(float64(a.Dx()), float64(b.Dx()))
	if h <= 0 || w <= 0 {
		return false
	}
	if math.Abs(centreY(a)-centreY(b)) > RowMatch*h {
		return false
	}
	in := a.Intersect(b)
	return float64(in.Dx()) >= RowSideMatch*w
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
		ID: c.id, Bounds: c.bounds, Rows: len(c.rows), Frames: c.seen,
		Exclusive: c.excl, Shared: c.shared, Quiet: c.quiet, Unreadable: c.unreadable,
		Transfers: c.moves, Holders: len(c.holders),
		Prompted: c.prompted, Unprompted: c.unprompted,
		QualifiedAt: c.qualifiedAt, Hull: c.hull, Rehomed: c.rehomed,
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
	r, _, why, ok := t.ActiveList()
	return r, why, ok
}

// ActiveList is the same answer with the list's IDENTITY beside its place.
//
// The place alone cannot answer "is this still the list I was looking at", and that question is
// asked on every frame by the rule that decides whether the mark moved WITHIN one list. A list's
// extent moves whenever one of its rows is found a pixel lower, so comparing extents answers
// "no" to a list that never changed — measured, the whole 864-frame corpus retained one move.
func (t *Tracker) ActiveList() (image.Rectangle, int, string, bool) {
	i, why := t.active()
	if i < 0 {
		return image.Rectangle{}, 0, why, false
	}
	return t.cands[i].bounds, t.cands[i].id, why, true
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
		ev := Evidence{Rows: len(c.rows), Frames: c.seen, Exclusive: c.excl,
			Shared: c.shared, Quiet: c.quiet, Unreadable: c.unreadable,
			Transfers: c.moves, Holders: len(c.holders)}
		// A SOLE QUALIFIER IS NOT AUTOMATICALLY THE LIST BEING USED.
		//
		// # The measurement that forced this
		//
		// Over 864 real frames, scored against the labelled list's own extent rather
		// than against whether some candidate overlapped the selected row:
		//
		//	published by this arm          144 frames, 0 the right list, 144 wrong
		//	published by the arm below      48 frames, 45 the right list, 3 wrong
		//
		// 139 of the session's 185 selection readings came from this arm and not one of
		// them named the labelled row. The arm below, which is the only one that
		// consults input evidence, read 43 of its 46 correctly. The difference is not a
		// threshold: it is that this arm asked nothing at all. First past the post won,
		// and on a real screen the first candidate to behave like a list is a stack of
		// unrelated rectangles that happens to flicker.
		//
		// # Why the requirement is responsiveness and not a shape
		//
		// Because a shape rule cannot work and this file already says so: a menu is not
		// the biggest list, the brightest list, or the list with the most going on. What
		// separates the list being USED from a list that is merely there is that the
		// person's navigation reaches it — the same evidence the arm below compares, and
		// `Evidence.Prompted` is an increment of a move the shading had already proved,
		// so no input can manufacture one.
		//
		// # Why it is conditional on navigation having been observed
		//
		// Because a caller that cannot see input must not have its silence read as "the
		// person pressed nothing". `Tracker.Observe` and `ObserveShades` are that
		// caller, and where no navigation was ever observed this arm behaves exactly as
		// it did. The rule is: WHERE THERE IS INPUT EVIDENCE TO BE HAD, a published
		// active list must have some.
		//
		// Deleting the guard must fail
		// TestASoleQualifyingListIsNotActiveUntilNavigationReachesIt.
		if t.navSeen && c.prompted < MinTransfers {
			return -1, fmt.Sprintf("one list hands a mark around and the person's "+
				"navigation has never reached it: %d of its %d transfers "+
				"followed an observed input, against the %d it takes to say a "+
				"mark moves when somebody presses something. Navigation WAS "+
				"observed in this session, so a list nothing has been pressed "+
				"at is not the list being used (%s)",
				c.prompted, c.moves, MinTransfers, ev)
		}
		return passing[0], fmt.Sprintf("one list hands a mark around: %s", ev)
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
//
// # Why the comparison is a SHARE and not a count
//
// Because a count systematically favours whichever list moves most, and it does so by
// coincidence rather than by evidence. A list whose mark moves on its own schedule collects
// attributed moves in proportion to how often it moves: with presses spread over a session,
// the share of any list's moves that happen to land inside a response window is roughly the
// share of the session the windows cover, whatever that list is doing.
//
// Measured on 864 frames of a real pause menu, with the response window applied once at its
// stated bound:
//
//	the menu      3 of its 7 moves attributed   43%
//	the panel     5 of its 31 moves attributed  16%   <-- wins on COUNT
//
// 31 presses over 57 seconds and a 400ms window cover about 22% of the session, so the panel's
// 16% is at or below what coincidence alone produces and the menu's 43% is twice it. The count
// rule picked the panel. The share rule picks the menu, and it does so for a reason that holds
// at any session length rather than on this recording.
//
// The minimum count stays, and has to: a list with one move and one attribution has a share of
// 100% and has been seen responding once.
func (t *Tracker) responding(passing []int) (int, string) {
	// THE MOST RESPONSIVE AND THE NEXT MOST, by share, carrying both terms so the margin
	// can be compared without floating point.
	bestAt, secondAt := -1, -1
	for _, i := range passing {
		if t.cands[i].prompted < MinTransfers {
			continue
		}
		switch {
		case bestAt < 0 || t.moreResponsive(i, bestAt):
			bestAt, secondAt = i, bestAt
		case secondAt < 0 || t.moreResponsive(i, secondAt):
			secondAt = i
		}
	}
	if bestAt < 0 {
		// Nothing was attributable. Either no navigation was observed at all — a
		// session with no input source, or a person who pressed nothing — or the
		// presses never coincided with a mark moving anywhere.
		return -1, ""
	}
	second, secondOf := 0, 0
	if secondAt >= 0 {
		second, secondOf = t.cands[secondAt].prompted, t.cands[secondAt].moves
	}
	c := t.cands[bestAt]
	// THE MARGIN, as a cross-multiplied comparison of two shares. NavMargin times as
	// responsive, not NavMargin times as many attributions.
	if secondAt >= 0 && c.prompted*secondOf < NavMargin*second*c.moves {
		// More than one list answered. This is the case that must stay ambiguous: it
		// is exactly when picking would be guessing.
		return -1, ""
	}
	return bestAt, fmt.Sprintf("one of %d qualifying lists responds to navigation: "+
		"%d of its %d transfers followed an observed input, against %d of %d for the "+
		"next (%s)", len(passing), c.prompted, c.moves, second, secondOf,
		Evidence{Rows: len(c.rows), Frames: c.seen, Exclusive: c.excl,
			Shared: c.shared, Quiet: c.quiet, Unreadable: c.unreadable,
			Transfers: c.moves, Holders: len(c.holders)}.String())
}

// moreResponsive reports whether candidate i answered navigation a larger SHARE of the times
// its mark moved than candidate j did.
//
// Cross-multiplied rather than divided: these are small integers and a float comparison here
// would make the answer depend on rounding at exactly the margin where the rule abstains. Ties
// on share are broken by the larger number of attributions, so a list seen responding ten
// times out of twenty beats one seen responding once out of two — more evidence for the same
// share is more evidence.
func (t *Tracker) moreResponsive(i, j int) bool {
	a, b := t.cands[i], t.cands[j]
	if a.moves <= 0 || b.moves <= 0 {
		return a.prompted > b.prompted
	}
	left, right := a.prompted*b.moves, b.prompted*a.moves
	if left != right {
		return left > right
	}
	return a.prompted > b.prompted
}

// ActiveEntries names which ENTRY of the active list each of these rows is, or zero.
//
// # Why the tracker answers this and not the caller
//
// Because the member set is the list's identity, and only the tracker has it. A caller holding
// a frame's rows and a published extent can say which rows are in the list; it cannot say which
// of them is the entry it was told about last time, and the difference is everything the reader
// needs. A reader whose history is filed under positions loses all of it whenever the detector
// finds one row more or fewer. See Row.ID.
//
// # At most one row per entry
//
// Two rows nearest the same member is an ambiguity, not two sightings of one entry, and the
// nearer of them is the one named. The other is left unnamed and contributes no history, which
// is the honest answer: nothing here can say which of two rows is the entry.
//
// Tolerant, like attribution and for the same reason — a selected row is REDRAWN, so the one
// row whose shade decides the reading is the row least like the member it belongs to. Naming it
// does not move it; only Tracker.update does that, and only on a strict match.
func (t *Tracker) ActiveEntries(rows []Row) []int {
	out := make([]int, len(rows))
	i, _ := t.active()
	if i < 0 {
		return out
	}
	c := t.cands[i]
	nearest := make(map[int]int, len(rows))
	closest := make(map[int]float64, len(rows))
	for k, row := range rows {
		m := c.memberAt(row.Bounds)
		if m < 0 {
			continue
		}
		d := math.Abs(centreY(c.rows[m]) - centreY(row.Bounds))
		if _, seen := nearest[m]; !seen || d < closest[m] {
			nearest[m], closest[m] = k, d
		}
	}
	for m, k := range nearest {
		out[k] = c.entry[m]
	}
	return out
}

// ActiveEntryRanks is where each entry of the active list sits among its rows, counted from
// the top and starting at zero, and how many rows there are.
//
// Zero is a real rank, so a caller tells "rank 0" from "no rank" by whether it has an entry
// identity at all. That keeps the number the same one a move was always recorded with.
//
// # Why a rank as well as an identity
//
// Because an identity is for deciding and a rank is for saying. "The mark moved from entry 3 to
// entry 5 of a 6-entry list" is a sentence a person can check against their screen; the same
// sentence with identities in it — entry 1 to entry 14 — is not. So the decision about whether
// two readings are the same entry uses the identity, and what is written down for a person uses
// the rank.
//
// # And the rank is over the list's MEMBERS, not over this frame's rows
//
// Which is the whole point. A rank among the rows the detector happened to find moves whenever
// it misses one, so "entry 3" means a different row from one look to the next and a move
// recorded across that boundary names two rows the mark was never on. A rank among the members
// changes only when the list itself gains or loses one, which is rare and which
// ActiveEntryRanks reports by returning the count beside it.
func (t *Tracker) ActiveEntryRanks() (map[int]int, int) {
	i, _ := t.active()
	if i < 0 {
		return nil, 0
	}
	c := t.cands[i]
	order := make([]int, len(c.rows))
	for k := range order {
		order[k] = k
	}
	sort.SliceStable(order, func(a, b int) bool {
		return centreY(c.rows[order[a]]) < centreY(c.rows[order[b]])
	})
	out := make(map[int]int, len(order))
	for rank, k := range order {
		out[c.entry[k]] = rank
	}
	return out, len(c.rows)
}

// ActiveMissing is how many rows the active list HAS that the last observation did not show,
// how many it has in all, and whether a list is active at all.
//
// # The question this answers
//
// Whether the mark could be on a row nobody can see. The reader is given the rows of this
// frame that fall inside the list, so it can only ever name one of those — and if the list has
// a row that this look did not find, the mark may be on it and the reader will instead name the
// strongest row it WAS given. Measured on this corpus, every such claim was wrong.
//
// # Why it is not a remembered row COUNT
//
// Because a menu may legitimately change, and "this list once had nine rows" is then a false
// oracle: the ninth row may be gone rather than missed. What is reported instead is the
// membership as it stands with each member's own staleness behind it — a row that is still a
// member and was not seen on the last look. A row that genuinely left stops being a member
// after MaxUnmatched observations and stops counting here, and a list that has not been watched
// long enough to have a membership worth trusting cannot qualify in the first place.
//
// So the three cases the owner named are separated by the mechanism rather than by a threshold:
// a row temporarily missing is a member with a non-zero cold count; a list that genuinely
// changed loses those members to aging; and a list whose history is not yet trustworthy is not
// yet an active list.
func (t *Tracker) ActiveMissing() (missing, of int, ok bool) {
	i, _ := t.active()
	if i < 0 {
		return 0, 0, false
	}
	c := t.cands[i]
	for _, n := range c.cold {
		if n > 0 {
			missing++
		}
	}
	return missing, len(c.rows), true
}

// ActiveMembers is the rows the active list is made of, in order down the screen, or nil.
//
// What the list believes it contains, which is not what any one look found inside it.
// `InterfaceBelief.Where` is their extent and `Entries` is how many rows the DETECTOR found
// inside it this frame; neither says which rows the list believes it has, and that is the
// question behind every selection claim about a row the list does not contain.
func (t *Tracker) ActiveMembers() []image.Rectangle {
	i, _ := t.active()
	if i < 0 {
		return nil
	}
	c := t.cands[i]
	out := make([]image.Rectangle, len(c.rows))
	copy(out, c.rows)
	sort.SliceStable(out, func(a, b int) bool { return centreY(out[a]) < centreY(out[b]) })
	return out
}

// IdentifiedAt is the observation on which a single list first qualified, or -1.
func (t *Tracker) IdentifiedAt() int { return t.identifiedAt }

// area is a rectangle's area as a float, so a ratio of two of them cannot overflow.
func area(r image.Rectangle) float64 { return float64(r.Dx()) * float64(r.Dy()) }

// extent is the rectangle a set of rows occupies.
func extent(rows []image.Rectangle) image.Rectangle {
	out := rows[0]
	for _, r := range rows[1:] {
		out = out.Union(r)
	}
	return out
}
