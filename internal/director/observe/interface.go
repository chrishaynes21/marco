package observe

import (
	"fmt"
	"image"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// What interface is in front of this session, and what is chosen in it.
//
// # Why this is a belief and not a finding
//
// A session already answers "what did the world contain". This answers something narrower
// and more perishable: what is on screen NOW, and what changed since the last look. It is
// kept here, folded at the shadow call site, because every bit of evidence under it is
// shadow evidence — regions a detector proposed, and two numbers describing how each one is
// painted. [[ADR-003-evidence-authority-by-source]] lets vision propose structure and
// regions; it does not let it establish that anything can be acted on, and nothing here
// claims otherwise.
//
// [[ADR-102-a-detector-earns-its-place-where-perception-fails]] is why it is worth having at
// all. A detector is admitted by the ABSENCE of better evidence, and a game has no
// accessibility tree — the case that decision anticipated.
//
// # What it may never say
//
// That the entries are all of them. The grouping that produced them is a guess about
// arrangement, and on the measured menu it found seven of nine: the last two sat in their
// own candidates. A belief that reported "a menu of seven" would be wrong in a way nobody
// could detect downstream, so `Complete` exists, is always false, and is the field a
// consumer has to read before counting anything.
//
// That an entry can be pressed. Not here, not by this evidence.
// [[ADR-004-vision-cannot-establish-actionability]].

// InterfaceStatus is how much is known about the interface.
type InterfaceStatus string

const (
	// InterfaceUnknown — nothing has been seen behaving like a list being used. The
	// ordinary early state, and the state on any screen nobody is navigating.
	InterfaceUnknown InterfaceStatus = "unknown"
	// InterfaceAmbiguous — more than one list behaves like one being used. An answer, not
	// a failure: picking between them would be a guess.
	InterfaceAmbiguous InterfaceStatus = "ambiguous"
	// InterfaceProvisional — one list has been seen handing a mark between its members.
	// Provisional because that is the strongest thing this evidence can support.
	InterfaceProvisional InterfaceStatus = "provisional"
)

// InterfaceBelief is what the session currently believes it is looking at.
type InterfaceBelief struct {
	Status InterfaceStatus `json:"status"`
	// Where the list is, window-relative and normalised. Zero unless provisional.
	Where Region `json:"where,omitzero"`
	// Entries is how many candidate entries were found in it. NOT how many it has.
	Entries int `json:"entries"`
	// Complete is always false and says so in the payload, so a reader counting entries
	// has to pass the field that tells them not to.
	Complete bool `json:"entries_known_complete"`
	// FromShapes is how many of those entries are rectangles nobody could name.
	//
	// The warning this field exists to keep answerable: a detected region is not proof of a
	// menu entry, and a list assembled entirely out of unnamed geometry is a weaker claim
	// than one assembled out of named controls — even though the mark on it reads the same
	// way. Equal to Entries means nothing in this list has a name.
	FromShapes int `json:"entries_from_unnamed_geometry,omitempty"`
	// Selected is the chosen entry, counted from the top, or -1 when the reader declined.
	Selected int `json:"selected"`
	// SelectedWhere is that entry's region, so a caller never has to index anything.
	SelectedWhere Region `json:"selected_where,omitzero"`
	// Reading is the reader's own word: selected, none_selected or insufficient_evidence.
	Reading string `json:"reading"`
	// Because is the evidence the status rests on, in words.
	Because string `json:"because"`
	// Observations is how many looks this belief is built from, and After is how many it
	// took before a list could be named at all. Both count looks that CARRIED EVIDENCE: a
	// slot the detector sat out is not a look at anything.
	Observations int `json:"observations"`
	After        int `json:"identified_after,omitempty"`

	// Sample and At are the look this belief rests on — not necessarily the look that just
	// happened.
	Sample int       `json:"sample,omitempty"`
	At     time.Time `json:"at,omitzero"`
	// Tracked says the SELECTION was read by a fast look over remembered geometry rather
	// than by the discovery that identified the list.
	//
	// The field that stops a fast observation being read as contemporaneous with an old
	// discovery, which [[ADR-152-slow-discovery-fast-tracking]] names as the defect to
	// avoid. TrackedAt is when those pixels were read; At is when the geometry was found;
	// TrackedAfterMS is the gap, stated rather than left to be subtracted.
	Tracked        bool      `json:"selection_from_tracking,omitempty"`
	TrackedAt      time.Time `json:"selection_read_at,omitzero"`
	TrackedAfterMS int64     `json:"selection_read_after_ms,omitempty"`

	// Stale says nothing has confirmed this belief since. A belief that is merely
	// unconfirmed must not read like one that was just seen, and on a live Director most
	// looks bring nothing: the shadow cadence lets an inference start every two seconds
	// while the session samples twice a second, so three looks in four are a property of
	// the cadence rather than of the screen.
	Stale bool `json:"stale,omitempty"`
	// Unconfirmed is how many consecutive looks have brought no evidence at all.
	Unconfirmed int `json:"looks_unconfirmed,omitempty"`
	// Empty is how many consecutive looks DID see the screen and found nothing shaded on it.
	//
	// Separate from Unconfirmed because they are different evidence and have different
	// consequences: nothing was looked at, against something was looked at and was bare.
	// One number for both would make a detector that keeps dropping out indistinguishable
	// from a menu that keeps closing.
	Empty int `json:"looks_with_nothing_on_screen,omitempty"`
}

// InterfaceChange is what moved since the previous observation.
//
// Separate from the belief because "what is true now" and "what just happened" are different
// questions, and a consumer that polls needs the second to be told rather than diffed — the
// same argument live.go makes for the whole event stream.
type InterfaceChange struct {
	// InterfaceChanged says the identified list itself is different, or newly known, or
	// newly unknown.
	InterfaceChanged bool `json:"interface_changed,omitempty"`
	// SelectionMoved says the chosen entry is not the one it was.
	SelectionMoved bool `json:"selection_moved,omitempty"`
	// From and To are the entry indices either side of a move, or -1.
	From int `json:"from"`
	To   int `json:"to"`
	// Why says it in words, including when nothing changed.
	Why string `json:"why"`
}

// InterfaceLook is one chance to see, offered to the watch whether or not anything was seen.
//
// Observed is the field the type exists for. A detector that sat out its slot has not reported
// an empty screen, and folding the two together would make a sampling cadence look like a menu
// that keeps vanishing — [[ADR-006-unknown-is-not-false]], at the point where a live Director
// differs from a replay most sharply: replaying a recording, every look carries a frame.
type InterfaceLook struct {
	Sequence int
	At       time.Time
	Observed bool
	// Why says what happened instead, when nothing was observed.
	Why string
	// Regions is what the detector proposed. Empty WITH Observed true is evidence about an
	// empty screen; empty with Observed false is no evidence at all.
	Regions []ShadowRegion
	// Shapes is geometry the detector proposed and could not name, from the SAME look.
	//
	// It is folded into the grouping beside the named regions, because the grouping's
	// question is about arrangement and arrangement does not need a word. What it must
	// never do is let a belief claim more than unnamed geometry can support, which is why
	// the belief counts how many of its entries came from here. See shape.go.
	Shapes []ShadowShape
}

// InterfaceMoment is a look at which the belief changed, kept so a session can say WHEN.
//
// A journal rather than a counter because the question a live session has to answer is "what
// did it emit, and when" — and a client that polled for the current belief and wrote down what
// it saw would be reporting its own polling rate.
type InterfaceMoment struct {
	Sample int             `json:"sample"`
	At     time.Time       `json:"at,omitzero"`
	Status InterfaceStatus `json:"status"`
	// Selected is the entry index, or -1.
	Selected int    `json:"selected"`
	Stale    bool   `json:"stale,omitempty"`
	Why      string `json:"why"`
}

// maxInterfaceMoments bounds the journal. The most recent are kept: a long session's early
// moments are a history, and what a reader is looking at is what just happened.
const maxInterfaceMoments = 64

// interfaceForgetLooks is how many consecutive looks may bring nothing before the belief is
// dropped rather than merely marked stale.
//
// The same order as the number of looks it takes to earn one (selection.MinObservations): a
// belief is held no longer than it cost to acquire. Past that the honest answer is that nobody
// has looked lately — which is still not the answer that the menu has gone.
const interfaceForgetLooks = 10

// interfaceEmptyLooks is how many consecutive looks may SEE the screen and find nothing
// shaded on it before the belief is given up.
//
// Two, and it is the same persistence rule the screen-change gate uses rather than a second
// policy: one look is not enough for a claim one look cannot make. Measured in
// Experiment-024, during fifty seconds in which the player held still and the screen did not
// change, the detector reported 29 → 0 → 13 → 13 → 18 → 0 → 0 → 1 → 1 → 1 → 0 admitted
// regions. A belief that a single zero destroyed would have been destroyed four times on an
// unchanging screen, and each destruction is a semantic change to unknown that nothing
// observed.
//
// It is deliberately NOT interfaceForgetLooks. These two absences are different evidence: a
// look that brought nothing has established nothing about the screen
// ([[ADR-006-unknown-is-not-false]]), while a look that saw the screen and found nothing on it
// is evidence that there is nothing there — weaker than ten looks' worth, stronger than none.
const interfaceEmptyLooks = 2

// interfaceGrid is the integer space normalised regions are judged in.
//
// The grouping and tracking rules are geometric and were written against pixels. Regions
// arrive window-relative and normalised, so they are scaled into a fixed grid rather than
// against any particular window size — which also means a belief does not change merely
// because somebody resized a window.
const interfaceGrid = 10000

// InterfaceWatch folds shadow regions into a belief about the interface, look by look.
type InterfaceWatch struct {
	tracker *selection.Tracker
	reader  *selection.Reader
	belief  InterfaceBelief
	change  InterfaceChange
	// looks counts the looks that carried evidence; missed counts the consecutive ones
	// since that have carried none; empty counts the consecutive ones that saw the screen
	// and found nothing shaded on it.
	looks  int
	missed int
	empty  int
	// seen is the sequence of the last look that carried evidence — the one `looks` last
	// counted.
	//
	// Kept because a DROPPED belief has to be able to say what it last saw, and cannot read
	// it off itself: the drop rewrites the belief's own Sample to the look it gave up at,
	// and then goes on rewriting it every look after that, so a message built from the
	// previous belief would ratchet forward and end up naming the look before this one. A
	// test caught that on the first attempt at this.
	seen int
	// tracking is the fast path's own account. Unexported for the reason the belief is: a
	// reader of a session gets the answer, never the machinery.
	tracking TrackingStats
	// trackedThisLook says a fast read confirmed this belief during the interval the next
	// look covers. Cleared by every look, set by Track.
	//
	// It exists because of what happens without it. The fast reads for an interval are
	// folded first, then the cycle's own look — and on the cycle the detector sat out, that
	// look is "nothing was observed", which marked the belief stale and overwrote the
	// reason the fast read had just written. A belief being confirmed four times a second
	// by pixels was reporting itself as unconfirmed.
	trackedThisLook bool
	moments         []InterfaceMoment
}

// NewInterfaceWatch starts one, believing nothing.
func NewInterfaceWatch() *InterfaceWatch {
	return &InterfaceWatch{
		tracker: selection.NewTracker(),
		reader:  selection.New(),
		belief:  InterfaceBelief{Status: InterfaceUnknown, Selected: -1, Reading: "insufficient_evidence"},
		change:  InterfaceChange{From: -1, To: -1, Why: "nothing observed yet"},
	}
}

// Observe folds one cycle's shadow regions in.
//
// Regions WITHOUT shading are dropped rather than defaulted: a region nobody measured and a
// region measured as black must not look alike, and a zero would read as the darkest thing
// on screen.
func (w *InterfaceWatch) Observe(look InterfaceLook) {
	tracked := w.trackedThisLook
	w.trackedThisLook = false
	if !look.Observed {
		w.unconfirmed(look, tracked)
		return
	}
	w.looks++
	w.missed = 0
	w.seen = look.Sequence
	before := w.belief

	boxes, shades, unnamed := measurableBoxes(look)
	if len(boxes) == 0 {
		w.emptyLook(before, look)
		return
	}
	// Something measurable was seen, so whatever run of bare looks was accumulating is over.
	w.empty = 0

	// Grouping works on boxes alone, so the shades are carried alongside by position.
	rows := selection.GroupRows(boxes)
	shadeFor := map[image.Rectangle]selection.Shade{}
	for i, b := range boxes {
		shadeFor[b] = shades[i]
	}
	rowShades := make([]selection.Shade, len(rows))
	for i, r := range rows {
		rowShades[i] = shadeFor[r.Bounds]
	}
	w.tracker.ObserveShades(rows, rowShades)

	live, why, ok := w.tracker.Active()
	next := InterfaceBelief{
		Selected: -1, Reading: "insufficient_evidence",
		Because: why, Observations: w.looks, After: w.tracker.IdentifiedAt(),
		Sample: look.Sequence, At: look.At,
	}
	if !ok {
		next.Status = InterfaceUnknown
		if n := qualifyingLists(w.tracker); n > 1 {
			next.Status = InterfaceAmbiguous
		}
		w.settle(before, next, "")
		return
	}

	next.Status = InterfaceProvisional
	next.Where = unscaleRect(live)
	// A DISCOVERY'S READING IS NOT A TRACKED ONE. Left set, the belief would keep claiming
	// its selection came from a fast look long after a slow one replaced it — and the two
	// timestamps would describe a read that is no longer the one being reported.
	next.Tracked, next.TrackedAt, next.TrackedAfterMS = false, time.Time{}, 0

	var kept []selection.Row
	var keptShades []selection.Shade
	for i, r := range rows {
		if r.Bounds.In(live.Inset(-2)) {
			kept = append(kept, r)
			keptShades = append(keptShades, rowShades[i])
		}
	}
	next.Entries = len(kept)
	for _, r := range kept {
		if unnamed[r.Bounds] {
			next.FromShapes++
		}
	}
	res := w.reader.ReadShades(kept, keptShades)
	next.Reading = string(res.Outcome)
	if res.Outcome == selection.Selected && res.Row >= 0 && res.Row < len(kept) {
		next.Selected = res.Row
		next.SelectedWhere = unscaleRect(kept[res.Row].Bounds)
	}
	next.Because = why + "; reading: " + res.Why
	// AND WHAT THE LIST IS MADE OF, in the sentence a person reads.
	//
	// A belief whose entries are all unnamed geometry is a real belief and a weaker one,
	// and the difference has to survive into prose — a reader who sees "entry 3 of 9 is
	// selected" and has to go looking for a count to discover that none of the nine has a
	// name has been told something misleading by omission.
	if next.FromShapes > 0 {
		next.Because += fmt.Sprintf("; %d of %d entries are geometry nobody could name",
			next.FromShapes, next.Entries)
	}
	w.settle(before, next, "")
}

// emptyLook folds a look that SAW the screen and found nothing shaded on it.
//
// # Why one of these does not end a belief
//
// Because the detector blinks. Experiment-024 watched a player hold still for fifty seconds in
// front of an unchanging menu while the detector reported 29 → 0 → 13 → 13 → 18 → 0 → 0 → 1 →
// 1 → 1 → 0 admitted regions. Under the previous rule each of those zeros replaced a
// provisional belief with `unknown` immediately — four semantic changes to a state that nothing
// observed, on a screen that did not change.
//
// So the same persistence rule the screen-change gate applies is applied here: a bare look is
// UNCERTAINTY, and only a second consecutive one is evidence the list has gone.
// [[ADR-153-a-blackout-is-not-a-destination]].
//
// It is not a higher threshold on anything. The first bare look is reported, counted, and
// visible in the belief as Empty; what it does not do is destroy a belief built from ten.
func (w *InterfaceWatch) emptyLook(before InterfaceBelief, look InterfaceLook) {
	w.empty++
	// Held only while there IS something to hold. A belief that was already unknown has
	// nothing to protect, and keeping the first bare look quiet there would suppress the
	// ordinary reason an unknown interface is unknown.
	if before.Status == InterfaceProvisional && w.empty < interfaceEmptyLooks {
		next := before
		next.Stale = true
		next.Empty = w.empty
		// Sample and At are NOT moved. They name the evidence this belief rests on, and
		// a bare look is not evidence for it — the same rule `unconfirmed` follows, and
		// the reason its message can say which sample to go and look at.
		next.Because = fmt.Sprintf("a look saw the screen and found nothing shaded on it "+
			"(%d of the %d it takes to give up; the evidence under this belief is "+
			"sample %d)", w.empty, interfaceEmptyLooks, before.Sample)
		w.settle(before, next, "a look found nothing on the screen")
		return
	}
	w.settle(before, InterfaceBelief{
		Status: InterfaceUnknown, Selected: -1, Reading: "insufficient_evidence",
		Because: fmt.Sprintf("%d consecutive look(s) saw the screen and found nothing "+
			"shaded on it", w.empty),
		Empty:        w.empty,
		Observations: w.looks, Sample: look.Sequence, At: look.At,
	}, "")
}

// unconfirmed folds a look that brought no evidence.
//
// The belief is KEPT and marked, not replaced. Nothing was seen, so nothing contradicts it;
// what changed is only its age, and that is the part a consumer has to be told. Past
// interfaceForgetLooks it is dropped — with the reason, so "nobody has looked lately" cannot
// be read as "the screen is empty".
func (w *InterfaceWatch) unconfirmed(look InterfaceLook, tracked bool) {
	// A FAST READ IS A CONFIRMATION, JUST NOT A DISCOVERY.
	//
	// The detector sat this slot out; the pixels did not. Marking the belief stale here
	// would report a belief confirmed four times a second as unconfirmed, and letting
	// `missed` grow would have interfaceForgetLooks drop a belief the fast path is actively
	// holding up. What it does NOT do is count as a look that carried structural evidence —
	// `looks` is untouched, because how much looking identified the list is the discovery's
	// number.
	//
	// Deleting this must fail TestAFastReadMovesTheMarkInsideAKnownList.
	if tracked && w.belief.Status == InterfaceProvisional {
		w.missed = 0
		return
	}
	w.missed++
	before := w.belief
	why := look.Why
	if why == "" {
		why = "nothing was observed this look"
	}

	next := before
	next.Stale = true
	next.Unconfirmed = w.missed
	if w.missed >= interfaceForgetLooks {
		// Unconditionally, including when the belief is already unknown: the reason has to
		// keep saying that nobody has looked. The first version only rewrote the belief on
		// the way down, so the look after a drop replaced "dropped for want of evidence"
		// with the ordinary stale line and the drop became unexplainable.
		//
		// STAMPED WITH THE LOOK IT WAS GIVEN UP AT, not with the evidence it gave up.
		// Elsewhere Sample and At name the evidence a belief rests on, and a belief that is
		// merely stale keeps naming it — that is what makes its message able to say which
		// sample to go and look at. A DROPPED belief rests on nothing, so there is no such
		// look to name, and naming the old one puts the moment on the timeline at the time
		// of the last thing seen rather than the time the answer changed. `emptyLook`'s
		// give-up path already stamps the look; these two are the same event with different
		// evidence behind them and must read the same way.
		//
		// Found by writing down what a live run would be asked to report: the protocol's
		// last phase asks whether the belief is lost, for the right reason, and WHEN, and
		// the journal's answer to "when" was up to tens of seconds early.
		//
		// Deleting this must fail TestABeliefIsGivenUpAtTheLookThatGaveUpOnIt.
		w.settle(before, InterfaceBelief{
			Status: InterfaceUnknown, Selected: -1, Reading: "insufficient_evidence",
			Observations: w.looks, Stale: true, Unconfirmed: w.missed,
			Sample: look.Sequence, At: look.At,
			// AND WHAT IT LAST SAW, because the two halves of the answer are the time it
			// gave up and the evidence it gave up on, and a reader with only the first
			// cannot tell a detector that stopped from a menu nobody opened.
			Because: fmt.Sprintf("no evidence has been observed for %d look(s): %s%s",
				w.missed, why, w.lastSeen()),
		}, why)
		return
	}
	next.Because = fmt.Sprintf("%s (unconfirmed for %d look(s); the evidence under this "+
		"belief is sample %d)", why, w.missed, before.Sample)
	w.settle(before, next, why)
}

// lastSeen names the last look that brought evidence, as a clause to hang on a reason, or
// nothing at all when no look ever has — in which case there is no sighting to point at and a
// sentence claiming one would be inventing it.
func (w *InterfaceWatch) lastSeen() string {
	if w.seen == 0 {
		return " (no look has brought any evidence yet)"
	}
	return fmt.Sprintf(" (the last look that brought evidence was sample %d)", w.seen)
}

// settle records the new belief and what moved to get to it.
//
// quiet is what to say when nothing moved: a look that brought no evidence still has something
// to report about itself.
func (w *InterfaceWatch) settle(before, next InterfaceBelief, quiet string) {
	c := InterfaceChange{From: before.Selected, To: next.Selected}
	switch {
	case before.Status != next.Status:
		c.InterfaceChanged = true
		c.Why = fmt.Sprintf("the interface went from %s to %s", before.Status, next.Status)
	case next.Status == InterfaceProvisional && before.Where != next.Where:
		c.InterfaceChanged = true
		c.Why = "a different list is now the one being used"
	}
	if before.Selected != next.Selected {
		c.SelectionMoved = true
		if c.Why == "" {
			switch {
			case next.Selected < 0:
				c.Why = "the selection can no longer be read"
			case before.Selected < 0:
				c.Why = fmt.Sprintf("entry %d is now selected", next.Selected)
			default:
				c.Why = fmt.Sprintf("the selection moved from entry %d to entry %d",
					before.Selected, next.Selected)
			}
		}
	}
	if c.Why == "" {
		c.Why = "nothing changed"
		if quiet != "" {
			c.Why = quiet
		}
	}
	w.belief, w.change = next, c
	// Journalled only when something actually moved. A moment per look would be a record of
	// the sampling rate; what a reader needs is the times the answer was different.
	if c.InterfaceChanged || c.SelectionMoved || len(w.moments) == 0 {
		w.moments = append(w.moments, InterfaceMoment{
			Sample: next.Sample, At: next.At, Status: next.Status,
			Selected: next.Selected, Stale: next.Stale, Why: c.Why,
		})
		if len(w.moments) > maxInterfaceMoments {
			w.moments = w.moments[len(w.moments)-maxInterfaceMoments:]
		}
	}
}

// Belief is what the session currently thinks it is looking at.
func (w *InterfaceWatch) Belief() InterfaceBelief { return w.belief }

// Change is what moved at the most recent look.
func (w *InterfaceWatch) Change() InterfaceChange { return w.change }

// Moments is the bounded journal of looks at which the answer was different.
func (w *InterfaceWatch) Moments() []InterfaceMoment {
	return append([]InterfaceMoment(nil), w.moments...)
}

// measurableBoxes is every rectangle on a look that can take part in a selection reading,
// with its shading, and which of them arrived without a name.
//
// ONE statement of this, used by Observe and by the replay diagnostic. It was inline in
// Observe, and a diagnostic that rebuilt it would have been a second opinion about what the
// tracker was given - which is the shape of "two lists, one fact" in this repository: a
// spectest that passed while production refused the same play, because two copies of one rule
// drifted. A replay whose input differs from production's input measures the replay.
//
// Regions WITHOUT shading are dropped rather than defaulted: a region nobody measured and a
// region measured as black must not look alike, and a zero would read as the darkest thing on
// screen.
//
// The unnamed geometry comes after the named structure and on equal terms within the grouping.
// Equal terms is the point: the grouping asks whether these rectangles are arranged like a list
// and whether one is painted unlike its neighbours, and neither question has ever needed a
// word. What it does not get is equal standing in the ANSWER - see FromShapes.
func measurableBoxes(look InterfaceLook) ([]image.Rectangle, []selection.Shade,
	map[image.Rectangle]bool) {

	var boxes []image.Rectangle
	var shades []selection.Shade
	// unnamed holds the boxes that came from shapes, so the belief can say how much of what
	// it is about has a name. A set rather than a parallel slice: grouping reorders and
	// discards, and an index into the input would not survive it.
	unnamed := map[image.Rectangle]bool{}
	for _, r := range look.Regions {
		if len(r.Shade) != selection.Features {
			continue
		}
		boxes = append(boxes, scaleRegion(r.Region))
		shades = append(shades, selection.Shade{r.Shade[0], r.Shade[1]})
	}
	for _, sh := range look.Shapes {
		if !sh.Shaded() {
			continue
		}
		b := scaleRegion(sh.Region)
		boxes = append(boxes, b)
		shades = append(shades, selection.Shade{sh.Shade[0], sh.Shade[1]})
		unnamed[b] = true
	}
	return boxes, shades, unnamed
}

// GroupedForLook reports the SIZE OF EVERY GROUP grouping proposes for one look, largest
// first.
//
// Exported for the replay diagnostic, and it answers the first question about a failed run.
// Experiment-025 reported "no list has been seen handing a mark between its members" after
// ninety-two presses at a menu, and nothing said which stage actually failed. Grouping is
// upstream of the one that spoke, and a session that cannot describe its output sends a reader
// to the selection rule for what may be a grouping finding.
//
// Sizes rather than a count, because the shape of the distribution IS the finding. Twelve
// groups of one and a group of three is a fragmented screen; one group of six is a menu. Both
// render as "13 groups" and they are opposite diagnoses — and only one of them can be fixed by
// the selection rule, because the tracker refuses any group below its minimum outright.
//
// It runs the same builder and the same GroupRows call Observe runs, and nothing else — no
// belief, no tracker, no state.
func GroupedForLook(look InterfaceLook) []int {
	if !look.Observed {
		return nil
	}
	boxes, _, _ := measurableBoxes(look)
	if len(boxes) == 0 {
		return nil
	}
	size := map[int]int{}
	for _, g := range selection.GroupRows(boxes) {
		size[g.Group]++
	}
	out := make([]int, 0, len(size))
	for _, n := range size {
		out = append(out, n)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(out)))
	return out
}

// InterfaceCandidate is one list the tracker watched, and what it observed about it.
//
// The tracker's own counters, renamed only where this package already has a word for the
// thing. Nothing here is derived or re-judged: `Qualifies` and `Unmet` come from the same
// predicate the decision uses, so a report cannot disagree with the belief it explains.
type InterfaceCandidate struct {
	// Where it sits, normalised and window-relative like every other region here.
	Where Region `json:"where"`
	// Members is how many rows the tracker knows this list to have.
	Members int `json:"members"`
	// Frames is how many observations it survived; Judged, how many of those it could
	// actually be assessed on. The threshold is compared against Judged, and the gap
	// between the two is a statement about the looks rather than about the list.
	Frames int `json:"frames"`
	Judged int `json:"judged"`
	// Exclusive, Shared and Quiet are frames with exactly one member marked, with more
	// than one, and with none. Unreadable is frames where too few members could be
	// measured to say.
	Exclusive  int `json:"exclusive"`
	Shared     int `json:"shared"`
	Quiet      int `json:"quiet"`
	Unreadable int `json:"unreadable"`
	// Transfers is the mark moving to a different member; Holders, how many distinct
	// members have ever held it. These two are the claim a menu makes and a heads-up
	// display does not.
	Transfers int `json:"transfers"`
	Holders   int `json:"holders"`
	// Qualifies says this behaves like a list somebody is choosing from, and Unmet says
	// precisely what it falls short of when it does not.
	Qualifies bool                  `json:"qualifies"`
	Unmet     []selection.Shortfall `json:"unmet,omitempty"`
}

// maxInterfaceCandidates bounds what the report carries.
//
// A tracker never prunes: a candidate whose extent stops matching anything simply stops being
// updated, and a long session on a changing screen accumulates them. The belief's `Because`
// states the total, so a truncated list cannot be mistaken for the whole one — and
// Tracker.Evidence sorts qualifying candidates first, so the ones a reader needs are the ones
// that survive the cut.
const maxInterfaceCandidates = 12

// Candidates is every list the tracker watched, most convincing first.
//
// Instrumentation, and deliberately nothing more. It reports the counters the classification
// already keeps; it does not classify, re-measure, or offer a candidate a reader could act on.
// A candidate here is not a control, not an affordance and not durable — the same line
// ShadowShape holds.
func (w *InterfaceWatch) Candidates() []InterfaceCandidate {
	ev := w.tracker.Evidence()
	if len(ev) > maxInterfaceCandidates {
		ev = ev[:maxInterfaceCandidates]
	}
	out := make([]InterfaceCandidate, 0, len(ev))
	for _, e := range ev {
		out = append(out, InterfaceCandidate{
			Where: unscaleRect(e.Bounds), Members: e.Rows,
			Frames: e.Frames, Judged: e.Judged(),
			Exclusive: e.Exclusive, Shared: e.Shared, Quiet: e.Quiet,
			Unreadable: e.Unreadable,
			Transfers:  e.Transfers, Holders: e.Holders,
			Qualifies: e.Qualifies, Unmet: e.Unmet(),
		})
	}
	return out
}

// qualifyingLists counts the candidates that behave like one being used.
func qualifyingLists(t *selection.Tracker) int {
	n := 0
	for _, e := range t.Evidence() {
		if e.Qualifies {
			n++
		}
	}
	return n
}

func scaleRegion(r Region) image.Rectangle {
	return image.Rect(
		int(r.X*interfaceGrid), int(r.Y*interfaceGrid),
		int((r.X+r.Width)*interfaceGrid), int((r.Y+r.Height)*interfaceGrid))
}

func unscaleRect(b image.Rectangle) Region {
	return Region{
		X: float64(b.Min.X) / interfaceGrid, Y: float64(b.Min.Y) / interfaceGrid,
		Width:  float64(b.Dx()) / interfaceGrid,
		Height: float64(b.Dy()) / interfaceGrid,
	}
}
