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
	// ListID names the tracked list this belief is about, for as long as it is tracked,
	// and is never reused.
	//
	// Reported because callers were deriving it from `Where`, and an extent is not an
	// identity: a list whose rows are re-found a pixel lower publishes a different extent
	// and is the same list. Whether it is the same list is the tracker's answer to give.
	ListID int `json:"list_id,omitempty"`
	// SelectedRank is where SelectedEntry sits among the list's own rows, counted from the
	// top and starting at ZERO, and is meaningful only where SelectedEntry is non-zero.
	// ListEntries is how many rows the list has.
	//
	// Entries, beside them, is how many rows the DETECTOR found inside the list this
	// frame, which is a different number and moves constantly. These two are over the
	// list's members, so they change only when the list itself does — which is what makes
	// "entry 3 of 6" mean the same row from one look to the next.
	SelectedRank int `json:"selected_rank,omitempty"`
	ListEntries  int `json:"list_entries,omitempty"`
	// SelectedEntry is the IDENTITY of the entry Selected names, or zero.
	//
	// Selected is a position among the rows this frame's detector found inside the list,
	// and a position does not survive a frame: a detector that misses one row has
	// renumbered every row below it. Anything that compares one reading with the next —
	// whether the mark moved, whether it moved WITHIN one list — has to compare this
	// instead. [[ADR-165-an-entry-is-the-same-entry-when-nothing-says-otherwise]].
	SelectedEntry int `json:"selected_entry,omitempty"`
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
	// Navs is how many DIRECTIONAL navigation intents were observed in the interval
	// this look covers.
	//
	// A count, not the intents. Which direction was pressed is deliberately not carried:
	// knowing that "down" was pressed AND that a mark moved down would let the input
	// predict the visual outcome, and an outcome that was predicted is not one that was
	// observed. The selection tracker uses this only to ask whether the person was
	// navigating at all while some list handed its mark along, which is what separates
	// two lists that both genuinely behave like menus.
	Navs int
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
// Exported as InterfaceGrid for the replay's grouping diagnostic, which has to print the
// geometry the grouping was actually given. See BoxesForLook for why that matters.
const interfaceGrid = InterfaceGrid

// InterfaceGrid is interfaceGrid, for a diagnostic outside this package.
const InterfaceGrid = 10000

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
	// navDebt is how much of the response window is still open, and navAt the look it
	// was last refreshed at. Together they carry "the person pressed something recently"
	// across looks in TIME rather than in observations.
	navDebt time.Duration
	navAt   time.Time
	// settlingEntry, settlingAt and settlingPrompted are the ENTRY the reader is waiting
	// to confirm, when it was FIRST pointed at, and whether the response window was open
	// at that moment. See noteSettling, which is the only thing that writes them.
	//
	// An entry and not a row: the bank is read back a frame or more later, by which time a
	// position names whichever row has moved into that slot.
	settlingEntry    int
	settlingAt       time.Time
	settlingPrompted bool
	// lookAt is when the previous look was taken and interval the gap to the one before
	// it, so a press can be attributed over a window at least as long as the gap between
	// the looks that could have seen the response. See window().
	lookAt   time.Time
	interval time.Duration
	moments  []InterfaceMoment
	// pending is the moves the mark has made since somebody last drained them, and
	// pendingDropped how many the bound refused. See TakeSelectionMoves.
	pending        []SelectionMove
	pendingDropped int
	// heldDropped counts why a held sighting was let go, by cause.
	//
	// A move is only ever recorded between two sightings of one list, so every reset is a
	// move that cannot be. 349 correct readings produced 13 retained moves on this corpus
	// and the causes were not separated; a count of each is the difference between a
	// perception gap and a corpus that simply never repeats itself.
	heldDropped map[string]int
	// heldRank is the rank the held entry had when it was seen, for the move's prose.
	heldRank int
	// heldList, heldEntries and heldEntry are the list the mark was last SEEN in and
	// which entry of it; heldGap how many looks have brought no reading since. See
	// noteMove, which is the only thing that touches them.
	// heldList is the IDENTITY of the list the mark was last seen in, from the tracker,
	// and heldEntries its entry count. Not its extent: see noteMove.
	heldList    int
	heldEntries int
	heldEntry   int
	heldGap     int
}

// MaxPendingSelectionMoves bounds how many moves the watch holds between drains.
//
// Generous, because the caller drains every cycle and a cycle can produce one slow move and
// one per fast read. It exists so an uncollected watch grows a counter rather than a slice.
const MaxPendingSelectionMoves = 64

// MaxUnreadBetweenSightings is how many unread looks a held sighting survives.
//
// The mark is not read on every look, and a move is the gap between two sightings — so this is
// the point at which "the mark was on entry 2 and is now on entry 4" stops being an
// observation and becomes an inference about what happened in between. Past it the earlier
// sighting is dropped and the next one starts a fresh hold, which costs one move and keeps the
// record about what was seen.
//
// Four. On the 864-frame Trackmania corpus every single move the mark made was seen within one
// unread look of its predecessor, so this bound is never reached there; it is set above the
// measurement rather than at it because the measurement comes from a recording sampled every
// 66ms, and a live session's cadence is far coarser. A value of 0 would discard the
// reader-declines-on-arrival look that every real move passes through — which is the whole
// reason this is a gap and not a diff.
//
// # NOTHING EXERCISES THIS BOUND, and that is worth stating rather than implying
//
// Three fixtures were built to reach it and none could. To pass four unread looks the selection
// has to be unreadable while the list stays identified, and the only way to arrange that is
// several members marked at once — which runs straight into the measured dwell defect
// ('s TestAMarkThatDwellsIsStillRefusedAsShared): a run of shared frames teaches
// every member that its current look is its unselected level, and the reader then cannot read
// anything again. So there is no readable sighting on the far side of any blackout long enough
// to test this.
//
// It is therefore a safety net on a path the current reader cannot take. It stays because the
// code it guards reads "the mark was last seen on entry 2" and that claim has to expire
// somewhere; it is NOT verified, and if the dwell defect is ever repaired this is the first
// thing that becomes testable.
const MaxUnreadBetweenSightings = 4

// SelectionMove is one observed move of the mark from one entry of the identified list to
// another.
//
// # Why the watch emits these rather than letting a reader diff the belief
//
// Because most of the moves happen where a poller cannot see them. ADR-152's fast path reads
// the mark several times per cycle over remembered geometry, so between two beliefs a caller
// could observe, the mark may have moved four times — and a consumer that diffed what it
// happened to catch would be recording its own polling rate, which is the argument
// InterfaceMoment already makes for being a journal.
//
// # What it is NOT
//
// It is not a transition in any graph, and it carries no screen. The watch has no idea which
// screen it is looking at; grounding one of these is somebody else's job, and it is done
// where the screen model lives. See [SelectionTransition].
type SelectionMove struct {
	// Where is the list the move was seen in, window-relative and normalised, and Entries
	// how many entries were found in it. Between them, the only identity a list has.
	//
	// BOTH are part of it, and the entry count is the half that is easy to drop. "Entry 3"
	// means nothing except against a stable count of entries: a list that gained a row
	// between two looks has renumbered everything below it, so a move recorded across that
	// boundary would name two rows that are not the two the mark was on.
	Where   Region
	Entries int
	// From and To are the entry indices either side of the move, counted from the top.
	//
	// Both are real entries. A mark appearing out of nothing, or going unreadable, is not a
	// move between two of them, and recording those as moves from or to -1 would put a
	// reading failure in the same table as an observation.
	From, To int
	// Prompted says a directional navigation intent was observed within MaxResponseLatency
	// of this move being SEEN — of FirstSeen where the pixels could date it, and of At
	// where they could not.
	//
	// THE INPUT LABELS A MOVE; IT NEVER CREATES ONE, which is ADR-158's rule and is enforced
	// here by construction: this field is set on a move the shading already proved, and no
	// amount of pressing produces a SelectionMove.
	//
	// False means NO PRESS WAS ATTRIBUTABLE, which is not the same claim as "nobody pressed
	// anything". At a coarse cadence the window can close between looks. Both readings are
	// carried forward rather than resolved here; see SelectionTransition.Unprompted.
	Prompted bool
	// Tracked says a fast look saw this move rather than a discovery, and At is when those
	// pixels were read.
	//
	// Kept because the two are different evidence: a discovery found the list and measured
	// it, a fast read measured a list somebody else found up to a cycle earlier.
	Tracked bool
	// FromUnnamed is how many of the list's entries were geometry nobody could name, at the
	// look this move was seen on. Equal to Entries means nothing in the list has a name.
	FromUnnamed int
	// Gap is how many looks brought no reading between the two sightings this move is the
	// difference between.
	//
	// Zero is the mark being read on consecutive looks. One is the ordinary case for a real
	// move, because the reader declines on the look a mark arrives — for that one look the
	// row it left has not been seen unmarked yet, so nothing is exclusive. Larger values are
	// weaker evidence and are carried rather than hidden: a sighting four looks after the
	// last one leaves room for a move nobody saw.
	Gap    int
	Sample int
	At     time.Time
	// FirstSeen is when the pixels FIRST pointed at To, which is a look or more before At.
	//
	// # Two times for one move, and why neither is enough alone
	//
	// At is when the reader agreed. A change of winner is not published until a second
	// look confirms it, which refuses every one-frame blend and is correct — but it means
	// the time on this move is the time of the confirmation, not of the change. Anything
	// that joins a move to a cause over a window is then late by the confirmation delay,
	// and measured at a live 2s cadence that delay is larger than the whole window.
	//
	// FirstSeen is when the change happened, as nearly as pixels can say. It is NOT a
	// weaker reading of the same thing: the move is still only recorded on confirmation,
	// and a provisional sighting that was never confirmed produces nothing at all. What
	// the field adds is the moment, for the two consumers that need a moment — whether a
	// press was attributable, and which direction was in the window.
	//
	// Zero when the sighting could not be dated, which is the ordinary case for the look a
	// list is first identified on. Every consumer falls back to At.
	FirstSeen time.Time
	// ConfirmedAfterMS is At minus FirstSeen, stated rather than left to be subtracted —
	// the same reason TrackedAfterMS is stated. It is the measurement of the confirmation
	// delay, which was an assumption until this field existed.
	ConfirmedAfterMS int64
}

// TakeSelectionMoves drains the moves seen since the last drain, with how many the bound
// dropped.
//
// A DRAIN and not a journal, because the one caller that grounds these has to see each move
// exactly once. A reader that re-read the same list would count one move again on every look,
// and a graph built from that would read a mark that moved once as a list being navigated
// furiously for as long as the session lasted.
func (w *InterfaceWatch) TakeSelectionMoves() ([]SelectionMove, int) {
	moves, dropped := w.pending, w.pendingDropped
	w.pending, w.pendingDropped = nil, 0
	return moves, dropped
}

// noteMove records a move between the last entry the mark was SEEN on and this one.
//
// # Why this is not a diff of two consecutive beliefs
//
// Because measured against a real reading, consecutive beliefs almost never show a move. The
// selection reader declines on the look a mark arrives, which is the honest answer — the row
// it left has not been seen unmarked yet, so for one look two rows stand out and nothing is
// exclusive. On a scripted five-row walk the belief reads
//
//	entry 3, entry 3, unreadable, entry 4, entry 4
//
// and a rule comparing neighbours sees two changes, neither of them between two entries. It
// recorded nothing at all from fourteen moves.
//
// So the question asked here is the one a person would ask: WHERE WAS THE MARK LAST SEEN, and
// where is it now. The unreadable looks in between are carried on the move as its Gap rather
// than deleted, because a sighting four looks after the last one is weaker evidence than one
// on the very next look and a reader has to be able to tell.
//
// # What resets it
//
// Any change to the list the belief is about. A different list, a different entry count, or
// no identified list at all, and the held sighting is dropped rather than carried across: "the
// mark was on entry 2" is a fact about one list, and applying it to the next one would invent a
// move out of two unrelated readings. Each of those is its own case below.
func (w *InterfaceWatch) noteMove(_, next InterfaceBelief) {
	if next.Status != InterfaceProvisional {
		// No list is identified, so there is nothing for a mark to be an entry OF.
		w.drop("no list identified")
		w.heldEntry, w.heldRank, w.heldGap = 0, 0, 0
		w.heldList, w.heldEntries = 0, 0
		return
	}
	if next.ListID != w.heldList || next.ListEntries != w.heldEntries {
		// A DIFFERENT LIST, or the same one that has gained or lost a row.
		//
		// BOTH TERMS USED TO BE FRAME-LOCAL AND BOTH WERE WRONG IN THE SAME DIRECTION.
		// The first was the published EXTENT: a list's extent is the extent of its rows,
		// and a row found one pixel lower moves it, so two readings of a list nobody had
		// touched compared unequal. The second was `Entries`, how many rows the DETECTOR
		// found inside the list this frame — which on 864 real frames took nine different
		// values and changed on most frames. Between them the held sighting was dropped
		// almost every look and three moves were retained from 31 presses.
		//
		// Both are now over the list itself. The identity is the tracker's answer, and the
		// count is of the list's MEMBERS, which changes only when the list gains or loses
		// one. That is the real renumbering, and it is the one this guard is for:
		// `SelectedRank` is a position among the members, so a list that gained a row has
		// moved every rank below it and a move recorded across that boundary would name
		// two rows the mark was never on. `TestAListThatRenumbersRetainsNoMove` holds it.
		switch {
		case next.ListID != w.heldList:
			w.drop("a different list")
		default:
			w.drop("the list gained or lost a row")
		}
		w.heldList, w.heldEntries = next.ListID, next.ListEntries
		w.heldEntry, w.heldRank, w.heldGap = next.SelectedEntry, next.SelectedRank, 0
		// THE BANKED MOMENT IS NOT RESET HERE, and an earlier version did reset it.
		//
		// A settling row is an INDEX into one list, so the obvious thing is to drop it
		// with the held sighting. The mutation gate showed that deleting the reset
		// changed no outcome, and the reason is worth keeping: a move is only ever
		// emitted on a look that found the SAME list as the look before it, and on such
		// a look the reader's indices are already the new list's. The one guard that
		// does the work is in settlingOf, which refuses a moment banked for a different
		// row whatever made it stale — a precondition on the lookup rather than three
		// places remembering to clear it.
		return
	}
	if next.SelectedEntry == 0 {
		// The mark was not read this look, or was read on a row that is not an entry of
		// this list. Not a move to nowhere — an unread look, and the only thing it
		// changes is how good the next sighting's evidence is.
		w.heldGap++
		if w.heldGap > MaxUnreadBetweenSightings {
			w.drop("unread for too long between sightings")
			w.heldEntry, w.heldRank = 0, 0
		}
		return
	}
	from, fromRank, gap := w.heldEntry, w.heldRank, w.heldGap
	w.heldEntry, w.heldRank, w.heldGap = next.SelectedEntry, next.SelectedRank, 0
	if from == 0 || from == next.SelectedEntry {
		return
	}
	if len(w.pending) >= MaxPendingSelectionMoves {
		w.pendingDropped++
		return
	}
	at, sample := next.At, next.Sample
	if next.Tracked {
		at = next.TrackedAt
	}
	// WHEN THE PIXELS MOVED, AND WHETHER A PRESS EXPLAINS IT AS OF THEN.
	//
	// The fallback is the old behaviour exactly: an undated sighting leaves the move dated
	// at its confirmation and judged by the window as it stands now, which is what every
	// move did before FirstSeen existed.
	prompted := w.navDebt > 0
	firstSeen, lateMS := time.Time{}, int64(0)
	if seenAt, seenPrompted, known := w.settlingOf(next.SelectedEntry); known {
		firstSeen, prompted = seenAt, seenPrompted
		if !at.IsZero() && at.After(seenAt) {
			lateMS = at.Sub(seenAt).Milliseconds()
		}
	}
	w.pending = append(w.pending, SelectionMove{
		// THE LIST'S OWN COUNT AND THE RANKS WITHIN IT, not this frame's. A move says
		// "from entry 3 to entry 5 of a 6-entry list", and every number in that sentence
		// has to be about the list rather than about one look at it.
		Where: next.Where, Entries: next.ListEntries,
		From: fromRank, To: next.SelectedRank, Gap: gap,
		// THE WINDOW AS IT STOOD WHEN THE PIXELS MOVED, where they could be dated.
		//
		// It used to be the window as it stood at the CONFIRMATION, which is a look or
		// more later: the reader refuses a row it has seen once. That dated every move
		// late, and at a live 2s cadence the delay is larger than the whole window. The
		// repair is `selection.Result.Provisional` — the reader now says which row this
		// frame pointed at without publishing it, and the watch banks the moment. See
		// noteSettling for why a provisional sighting can carry a time and nothing else,
		// and settlingOf for why the earlier answer REPLACES this one rather than being
		// or-ed with it.
		Prompted:    prompted,
		Tracked:     next.Tracked,
		FromUnnamed: next.FromShapes,
		Sample:      sample, At: at,
		FirstSeen: firstSeen, ConfirmedAfterMS: lateMS,
	})
}

// drop records that a held sighting was let go, and why, when there was one to let go of.
func (w *InterfaceWatch) drop(why string) {
	if w.heldEntry == 0 {
		return
	}
	if w.heldDropped == nil {
		w.heldDropped = map[string]int{}
	}
	w.heldDropped[why]++
}

// HeldSightingsDropped is why a held sighting was let go, by cause, over the session.
//
// A move is only recorded between two sightings of one list, so each of these is a move that
// could not be. Instrumentation: it counts what the rule already did and decides nothing.
func (w *InterfaceWatch) HeldSightingsDropped() map[string]int {
	out := make(map[string]int, len(w.heldDropped))
	for k, v := range w.heldDropped {
		out[k] = v
	}
	return out
}

// settlingOf is when the pixels first pointed at this row, and whether the window was open
// then, for a row the reader has just CONFIRMED.
//
// # Why the window is REPLACED rather than widened
//
// Because the two answers disagree in both directions and the earlier one is the right one.
// A press 300ms before the pixels moved is attributable and was being missed, because by the
// time the reader agreed the window had closed — that is the gap this repairs. But a press
// that landed AFTER the pixels had already moved was being CREDITED, because the window it
// opened was still open when the reader agreed. That one is the input claiming a move it
// cannot have caused, and it is the same refusal the press ledger already makes one layer
// down: a slot later than the move is not its direction. Widening the test with an `or` would
// have kept it.
//
// So a dated sighting answers on its own, and only an undated one falls back to the debt as it
// stands now. Deleting the refusal must fail TestAPressAfterThePixelsMovedIsNotWhatMovedThem.
//
// # Why the row is checked here and nowhere else
//
// Because this is the lookup, and its contract is the moment for THIS row. An earlier version
// also cleared the banked moment wherever the identified list changed; the mutation gate showed
// those resets changed no outcome, because a move is only emitted on a look that found the same
// list as the look before it. A precondition on the lookup covers a stale index whatever made
// it stale, which three callers remembering to clear it does not.
//
// Deleting the row test must fail TestABankedMomentBelongsToOneRow.
func (w *InterfaceWatch) settlingOf(row int) (at time.Time, prompted, known bool) {
	// settlingEntry >= 0 implies settlingAt is set: noteSettling refuses an undated sighting
	// rather than banking one for this to reject.
	if row == 0 || row != w.settlingEntry {
		return time.Time{}, false, false
	}
	return w.settlingAt, w.settlingPrompted, true
}

// NewInterfaceWatch starts one, believing nothing.
func NewInterfaceWatch() *InterfaceWatch {
	return &InterfaceWatch{
		tracker:       selection.NewTracker(),
		reader:        selection.New(),
		heldEntry:     -1,
		settlingEntry: 0,
		belief:        InterfaceBelief{Status: InterfaceUnknown, Selected: -1, Reading: "insufficient_evidence"},
		change:        InterfaceChange{From: -1, To: -1, Why: "nothing observed yet"},
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
	// Deleting the look.Navs argument must fail TestTheActiveListIsTheOneThatRespondsToNavigation.
	w.tracker.ObserveNavigated(rows, rowShades, w.navRecent(look))

	live, listID, why, ok := w.tracker.ActiveList()
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
	next.ListID = listID
	// A DISCOVERY'S READING IS NOT A TRACKED ONE. Left set, the belief would keep claiming
	// its selection came from a fast look long after a slow one replaced it — and the two
	// timestamps would describe a read that is no longer the one being reported.
	next.Tracked, next.TrackedAt, next.TrackedAfterMS = false, time.Time{}, 0

	kept, keptShades := entriesInside(rows, rowShades, live)
	// EACH ROW NAMED AS AN ENTRY OF THE LIST, before the reader is asked anything. The
	// reader's only evidence about a row is how it compares with its own past, and its past
	// has to be filed under a name that survives a frame. Deleting this must fail
	// TestTheReaderKeepsARowsHistoryWhenAnotherRowGoesMissing.
	for i, id := range w.tracker.ActiveEntries(kept) {
		kept[i].ID = id
	}
	ranks, listEntries := w.tracker.ActiveEntryRanks()
	next.ListEntries = listEntries
	next.Entries = len(kept)
	for _, r := range kept {
		if unnamed[r.Bounds] {
			next.FromShapes++
		}
	}
	res := w.reader.ReadShades(kept, keptShades)
	// WHEN THE PIXELS MOVED, banked before the reading is settled. See noteSettling.
	//
	// Deleting this must fail TestAMoveSaysWhenThePixelsMovedAndNotOnlyWhenItWasBelieved.
	w.noteSettling(res, look.At)
	next.Reading = string(res.Outcome)
	if res.Outcome == selection.Selected && res.Row >= 0 && res.Row < len(kept) {
		next.Selected = res.Row
		next.SelectedEntry = res.Entry
		next.SelectedRank = ranks[res.Entry]
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
	// WHERE THE MARK WAS LAST SEEN, on every settle and not only the ones that moved it.
	//
	// Unconditional because the bookkeeping is about looks that brought NO reading as much
	// as about the ones that did: a look where the selection stays unread has to age the
	// held sighting, and a look where the identified list changes has to drop it. Called
	// only when the belief's own selection index changed, it would do neither — and the
	// index is unchanged on exactly the looks that matter here.
	//
	// This is also the one place every move passes through: the fast path and the discovery
	// path both settle here, and a record kept on either alone would miss most of a live
	// session's moves.
	//
	// Deleting this must fail TestEveryMoveOfTheMarkIsOfferedForGrounding.
	w.noteMove(before, next)
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

// GroupedStacksForLook is the stacks grouping made of this look, each as its own rows,
// window-relative and normalised.
//
// The stacks and not just the rows, because the question behind an incomplete list is whether
// the rows it is missing are in a DIFFERENT stack. A menu split into a six and a three is a
func GroupedStacksForLook(look InterfaceLook) [][]Region {
	if !look.Observed {
		return nil
	}
	boxes, _, _ := measurableBoxes(look)
	if len(boxes) == 0 {
		return nil
	}
	byGroup := map[int][]Region{}
	var order []int
	for _, r := range selection.GroupRows(boxes) {
		if _, seen := byGroup[r.Group]; !seen {
			order = append(order, r.Group)
		}
		byGroup[r.Group] = append(byGroup[r.Group], unscaleRect(r.Bounds))
	}
	out := make([][]Region, 0, len(order))
	for _, g := range order {
		out = append(out, byGroup[g])
	}
	return out
}

// GroupedRowsForLook is the rows grouping made of this look, window-relative and normalised.
//
// INSTRUMENTATION, and the companion to GroupedForLook, which gives the sizes alone. The sizes
// answer "was there a stack"; these answer "is the row somebody is looking at IN one", which is
// a different question and the one that matters when a list is published without the row the
// mark is on. A detector may propose a row, grouping may leave it out of every stack, and the
// list that is published then cannot contain it however well everything downstream works.
//
// Reports what the production rule produced. It does not group, re-group, or judge.
func GroupedRowsForLook(look InterfaceLook) []Region {
	if !look.Observed {
		return nil
	}
	boxes, _, _ := measurableBoxes(look)
	if len(boxes) == 0 {
		return nil
	}
	rows := selection.GroupRows(boxes)
	out := make([]Region, 0, len(rows))
	for _, r := range rows {
		out = append(out, unscaleRect(r.Bounds))
	}
	return out
}

// ActiveMembers is the rows the active list is made of, window-relative and normalised, or nil.
//
// The list's own answer about what it contains. `InterfaceBelief.Where` is their extent and
// `Entries` is how many rows the DETECTOR found inside it this frame; neither says which rows
// the list believes it has, and that is the question behind every selection claim about a row
// the list does not contain.
func (w *InterfaceWatch) ActiveMembers() []Region {
	members := w.tracker.ActiveMembers()
	if len(members) == 0 {
		return nil
	}
	out := make([]Region, 0, len(members))
	for _, m := range members {
		out = append(out, unscaleRect(m))
	}
	return out
}

// MaxResponseLatency is how long after a press a screen may still be repainting it.
//
// THE ONE LATENCY ASSUMPTION IN THE ATTRIBUTION, named and in one place. A menu animates for a
// couple of hundred milliseconds after a directional press; beyond that, a mark that moves is
// moving for its own reasons.
//
// It replaced a window counted in OBSERVATIONS, which looked cadence-independent and was the
// opposite. Two observations is 132ms when the frames are 66ms apart and 4.6s when the cycle is
// 2.3s, against a repaint that takes the same couple of hundred milliseconds either way. The
// short end credited 4 of a real menu's 31 transfers; the long end would credit an unrelated
// list's moves to the person.
//
// 400ms is deliberately generous for an animation and well inside the ~1.3s cadence at which a
// list is still identifiable at all, so the bound cannot quietly become "any time this session".
const MaxResponseLatency = 400 * time.Millisecond

// noteSettling banks the moment a row was first pointed at, for the move that may follow.
//
// # The whole of what provisional evidence is allowed to do
//
// It is allowed to carry a TIME. Nothing here touches the belief, the selection, the tracker or
// the graph: a provisional row is not published, is not counted, and does not appear in any
// report. What is kept is the moment it appeared and whether a press was attributable then —
// and both are read only once that same row has been CONFIRMED by the reader, which is the
// arithmetic that makes the narrow use safe rather than the convention.
//
// Three consequences follow from "only once that same row is confirmed", and they are the
// reasons this is not a quiet re-admission of the one-frame blend:
//
//   - a provisional sighting that is never confirmed is never read by anything;
//   - a blend names the row the selection is LEAVING, so it sets a DIFFERENT row here, and
//     the row that follows it finds its own later moment rather than inheriting the blend's;
//   - the published reading is byte-identical either way, so coverage and accuracy cannot
//     move. TestTheProvisionalFieldChangesNoReading holds that on a whole session.
//
// The window is snapshotted rather than recomputed later. navDebt decays as looks arrive, so
// asking "was it open then" after the fact would mean reconstructing a clock — and mixing a
// look's absolute time with an input producer's own milliseconds is a recorded trap. Taking the
// answer at the moment it is true needs no arithmetic at all.
func (w *InterfaceWatch) noteSettling(res selection.Result, at time.Time) {
	if at.IsZero() {
		// A MOMENT WITH NO TIME IS NOT A MOMENT, refused on the way in rather than on
		// the way out. A session with no clock can still point at rows, and banking
		// one would store a row index against a zero time for settlingOf to reject
		// later — two answers to one question. Refusing here makes the invariant
		// settlingEntry >= 0 implies settlingAt non-zero true by construction, which is
		// why settlingOf no longer tests for it: the mutation gate showed the second
		// test could not change an outcome.
		return
	}
	if res.Provisional < 0 {
		// Either this look published a row or it pointed at nothing. Neither is a new
		// provisional sighting, and the banked one is LEFT STANDING: the look that
		// confirms a row is the look after the one that proposed it, and clearing here
		// would throw the moment away exactly when it is about to be read.
		return
	}
	// A NEW ROW, OR A NEW RUN OF AGREEMENT ABOUT THE SAME ONE.
	//
	// The second half follows the reader rather than inventing a second policy, and without
	// it the moment can go badly stale. The reader clears its pending candidate on any frame
	// it cannot read — so "row 4, unreadable, unreadable, row 4, row 4" is, to the reader,
	// one lapsed sighting and then a fresh one, and ProvisionalN says so by starting over at
	// 1. Keeping the first moment would date the arrival from a sighting the reader had
	// already given up on, which at a coarse cadence is seconds earlier and may have been a
	// blend that went nowhere.
	//
	// Deleting the ProvisionalN test must fail
	// TestALapsedSightingIsNotTheMomentOfALaterArrival.
	if res.ProvisionalEntry != w.settlingEntry || res.ProvisionalN <= 1 {
		w.settlingEntry = res.ProvisionalEntry
		w.settlingAt = at
		w.settlingPrompted = w.navDebt > 0
	}
}

// navRecent reports whether the person was navigating recently enough that a transfer seen on
// this look could be the response.
//
// # Why the interval is the gap since the last look, OR the latency bound
//
// Both, whichever is longer, and each covers a case the other cannot.
//
// A look's own interval is the right window when looks are far apart: at a 2.3s cadence the
// press and the repaint are both inside one look, and the sample's inputs ARE the inputs of
// that interval, because the producer drains per cycle. Nothing needs carrying forward.
//
// The latency bound is the right window when looks are close together: at 66ms the press lands
// on one look and the mark has not moved yet, so the response shows up five or six looks later.
// Without the bound those transfers are attributed to nothing.
//
// Expressed as a decaying debt rather than a timestamp comparison, because the two clocks here
// must not be mixed: a look carries an absolute time and an InputEvent carries milliseconds from
// its producer's own start, and joining those two numbers is a recorded trap. Subtracting one
// look's time from another's is safe, and that is all this does.
func (w *InterfaceWatch) navRecent(look InterfaceLook) bool {
	// A LOOK'S OWN INTERVAL, remembered before anything uses it. The window a press opens
	// is at least as long as the gap between the looks that could see the response.
	if !look.At.IsZero() {
		if !w.lookAt.IsZero() {
			w.interval = look.At.Sub(w.lookAt)
		}
		w.lookAt = look.At
	}
	if look.Navs > 0 {
		w.Navigated(look.At, look.Navs)
		return true
	}
	return w.navOpen(look.At)
}

// Navigated opens the response window from a slot that carried directional navigation.
//
// # Why this is reachable from outside the watch
//
// Because the DETECTOR'S CADENCE must not decide whether a press is attributable.
// `InterfaceLook.Navs` is only read on a look that was OBSERVED, and live, an inference starts
// about every two seconds against a session sampling twice a second — so three slots in four
// bring no look at all. A press made during one of those slots never opened the window, while
// the fast reads taken during that same slot are exactly the ones that saw the mark move.
//
// Navigation evidence and screen evidence fail independently, which is the argument
// `ShadowTotals.add` already makes for folding the input account above its own early return.
// This is the same argument one layer in.
//
// The count is a count, for the reason InterfaceLook.Navs is: which direction was pressed must
// not reach anything that reads a mark, or the input could predict the outcome it is being
// correlated with.
//
// Deleting the call site in ShadowTotals.Observe must fail
// TestAPressDuringASkippedSlotStillAttributesTheMoveItExplains.
func (w *InterfaceWatch) Navigated(at time.Time, n int) {
	if n <= 0 {
		return
	}
	w.navDebt = w.window()
	w.navAt = at
}

// window is how long a press stays attributable: the latency bound, or the gap between looks,
// whichever is longer.
//
// # The half of ADR-158 that was written down and not implemented
//
// It says the attributable interval is "the gap since the last look OR the latency bound,
// whichever is longer", and gives the reason: a look's own interval is right when looks are far
// apart, because the press and the repaint are then both inside one look. The code only ever
// set the bound. At the recording's own 66ms that is invisible — the window spans six looks
// either way — and at a coarse cadence it is the whole difference.
//
// Measured, on the same 864 frames reprocessed at two rates. The gap between a press and the
// look at which the mark was first SEEN somewhere else:
//
//	every 66ms    4 moves, 128ms to 504ms      3 of 4 inside the 400ms bound
//	every 660ms   2 moves, 544ms and 1992ms    0 of 2 inside it
//
// The menu repaints just as fast either way. What changes is when Marco next looks, and a
// response cannot be seen before the look that sees it. A window shorter than the sampling
// interval attributes nothing at a live cadence, which is where every move will come from.
//
// # Why it is bounded above
//
// Because past some gap a look's interval stops licensing anything: if Marco last looked a
// minute ago, "something was pressed during that minute and the mark is now elsewhere" is not
// an attribution, it is a coincidence with a long name. TrackStaleness is the existing answer
// to how old an observation may be and still describe the same arrangement, and reusing it
// keeps one bound rather than inventing a second.
func (w *InterfaceWatch) window() time.Duration {
	// THE INTERVAL AND THE REPAINT, not whichever is larger.
	//
	// They are consecutive, not alternative. The press happened somewhere inside the
	// interval the look covers — the producer drains per cycle and says nothing about
	// where in it — and the screen then takes up to the latency bound to show it. So the
	// response may become visible as late as one whole interval plus one repaint after
	// the window opened.
	//
	// Measured, and the arithmetic is unforgiving: with the window set to the interval
	// ALONE, at a 2s cadence it expires at the exact instant the next look is taken, and
	// 0 of 26 transfers were attributed on a fixture where every single one of them was
	// the look after a press. A window equal to the gap it has to span reaches nothing.
	if w.interval <= 0 {
		return MaxResponseLatency
	}
	if span := w.interval + MaxResponseLatency; span < TrackStaleness {
		return span
	}
	// Past this a look's interval stops licensing anything: if Marco last looked a
	// minute ago, "something was pressed during that minute and the mark is now
	// elsewhere" is not an attribution, it is a coincidence with a long name.
	return TrackStaleness
}

// navOpen reports whether the response window is still open at this moment, ageing the debt by
// the time since it was last refreshed or read.
//
// Separated from navRecent so a FAST look can age the window without claiming to carry input
// evidence. A TrackLook has no navigation on it at all — the producer drains per cycle and a
// fast read is not a cycle — so for the tracking path the only honest question is whether the
// window a slow look opened is still open by the time these pixels were read.
func (w *InterfaceWatch) navOpen(at time.Time) bool {
	if w.navDebt <= 0 {
		return false
	}
	// A look with no time cannot age anything; treat it as no elapsed time rather than as
	// a jump to the epoch, which would expire the window on the first such look. Nor may
	// time run backwards: the fast reads for an interval are folded before that interval's
	// own look, so an out-of-order timestamp is ordinary and must not rewind the clock.
	if !at.IsZero() && !w.navAt.IsZero() {
		if elapsed := at.Sub(w.navAt); elapsed > 0 {
			w.navDebt -= elapsed
			w.navAt = at
		}
	}
	return w.navDebt > 0
}

// InterfaceCandidate is one list the tracker watched, and what it observed about it.
//
// The tracker's own counters, renamed only where this package already has a word for the
// thing. Nothing here is derived or re-judged: `Qualifies` and `Unmet` come from the same
// predicate the decision uses, so a report cannot disagree with the belief it explains.
type InterfaceCandidate struct {
	// ID names this tracked list for as long as it is tracked, and is never reused. It is
	// what InterfaceBelief.ListID refers to, so the belief and the table can be read
	// together.
	ID int `json:"id,omitempty"`
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
	// Prompted and Unprompted split Transfers by whether navigation was observed just
	// before the move was seen. The evidence that tells two real lists apart, and the
	// input only ever labels a move the shading already proved.
	Prompted   int `json:"prompted,omitempty"`
	Unprompted int `json:"unprompted,omitempty"`
	// Qualifies says this behaves like a list somebody is choosing from, and Unmet says
	// precisely what it falls short of when it does not.
	Qualifies bool                  `json:"qualifies"`
	Unmet     []selection.Shortfall `json:"unmet,omitempty"`
	// QualifiedAt is the observation this candidate first met every requirement, or -1.
	//
	// WHICH candidate got there first, as against Identified's WHEN. It matters because
	// the tracker publishes a sole qualifier with no comparison to anything, so the order
	// candidates cross the bar in IS the decision — and a report that cannot show that
	// order cannot explain a wrong one.
	QualifiedAt int `json:"qualified_at,omitempty"`
	// Hull is the union of every extent this candidate has ever had, and Rehomed how many
	// times that extent changed area by more than a factor of two.
	//
	// The temporal-identity diagnostic, and it cannot be recovered from Where: a
	// candidate's bounds are overwritten by every frame's extent, so the table shows
	// wherever it ended up. A hull far larger than the bounds is one tracked "list" that
	// has been several different rectangles.
	Hull    Region `json:"hull,omitzero"`
	Rehomed int    `json:"rehomed,omitempty"`
}

// maxInterfaceCandidates bounds what the report carries.
//
// A long session on a changing screen tracks more lists than a person will read about. The
// belief's `Because` states the total, so a truncated list cannot be mistaken for the whole
// one — and Tracker.Evidence sorts qualifying candidates first, so the ones a reader needs are
// the ones that survive the cut.
//
// The tracker also retires candidates nothing has found again, and `Retired` counts them: a
// population being pruned and one that was never crowded look identical from the survivors.
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
			ID:    e.ID,
			Where: unscaleRect(e.Bounds), Members: e.Rows,
			Frames: e.Frames, Judged: e.Judged(),
			Exclusive: e.Exclusive, Shared: e.Shared, Quiet: e.Quiet,
			Unreadable: e.Unreadable,
			Transfers:  e.Transfers, Holders: e.Holders,
			Prompted: e.Prompted, Unprompted: e.Unprompted,
			Qualifies: e.Qualifies, Unmet: e.Unmet(),
			QualifiedAt: e.QualifiedAt,
			Hull:        unscaleRect(e.Hull), Rehomed: e.Rehomed,
		})
	}
	return out
}

// ActiveMissing is how many rows the active list HAS that the last look did not show, and how
// many it has in all.
//
// REPORTED AND NOT ACTED ON, and the measurement behind that is in
// [[ADR-167-a-slow-gate-and-two-rules-that-measured-worse]]. Withholding the selection claim
// whenever this was non-zero raised accuracy from 96% to 98% and cut confidently wrong readings
// from 13 to 7 — and took retained graph moves from 13 to ONE, because a move needs two
// readings of one list and the withheld looks fall between them. A rule that buys precision by
// making the graph stop learning is not a trade worth taking.
//
// It is still worth saying. A claim about which entry is selected, made while the list has a row
// nobody could see, is a claim that could not have been right, and a report that cannot count
// those cannot tell a reader's error from a list's.
func (w *InterfaceWatch) ActiveMissing() (missing, of int) {
	m, o, _ := w.tracker.ActiveMissing()
	return m, o
}

// RetiredCandidates is how many tracked lists have been dropped for not being found again.
//
// Reported rather than inferred, because retirement is the half of the identity rule that
// nothing else can show: the survivors look the same whether the population is being pruned or
// was never crowded, and a retired candidate's evidence is deliberately gone rather than
// merged into whatever is tracked at that place next.
func (w *InterfaceWatch) RetiredCandidates() int { return w.tracker.Retired() }

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

// BoxesForLook is the rectangles a look offers the grouping, with its shading dropped.
//
// Exported beside GroupedForLook and for the same reason: a diagnostic that rebuilt what the
// grouping is given would be a second opinion about the tracker's input, and this repository
// has a memory note about exactly that ("two lists, one fact"). A replay whose input differs
// from production's input measures the replay.
func BoxesForLook(look InterfaceLook) []image.Rectangle {
	if !look.Observed {
		return nil
	}
	boxes, _, _ := measurableBoxes(look)
	return boxes
}

// entriesInside is the rows of the identified list, IN VISUAL ORDER, with their shading.
//
// # Why the sort is not cosmetic
//
// Because `Selected` is documented as the chosen entry "counted from the top", and without this
// it was not. GroupRows returns the stack's members top to bottom and then appends the aligned
// strays — a menu's last entry, set apart by a wider gap — in the order deduplication left
// them, which is largest-area first. Any list with a stray therefore handed the reader a
// sequence in which position 6 was somewhere in the middle of the screen.
//
// Measured on 864 frames of a real pause menu, against labels naming which of nine entries was
// selected: the belief read a nine-entry list on 139 frames and named the right ordinal on
// NONE of them. The reading was not wrong about which rectangle was marked; it was wrong about
// which entry that rectangle was, which is the only form of the answer a route can use.
//
// One statement of it, called from the discovery path and the tracking path, because two copies
// of "which rows belong to this list" is how the fast path and the slow path come to disagree
// about where the mark is.
func entriesInside(rows []selection.Row, shades []selection.Shade,
	live image.Rectangle) ([]selection.Row, []selection.Shade) {

	type entry struct {
		row   selection.Row
		shade selection.Shade
	}
	var in []entry
	// CONTAINED BY THE LIST, and a relaxation to "mostly inside" was tried and reverted.
	//
	// The reasoning for relaxing it was that a game draws the entry the person is on with a
	// highlight, so the detector returns a larger box for it and the SELECTED row would be
	// the one that pokes out of an extent that is actually the list's. That is a real shape
	// of failure and it is not reachable here: under identity by rows a member moves to the
	// row it was found as, so a row that grew a little is still that member and the extent
	// grows with it. Measured on 864 real frames, the relaxation was worth 4 namings of 35,
	// and no fixture could be built that it was load-bearing for — every swollen row that
	// was mostly inside the list also still corresponded to its member.
	// [[ADR-164-a-list-is-the-same-list-when-its-rows-are]].
	for i, r := range rows {
		if !r.Bounds.In(live.Inset(-2)) {
			continue
		}
		var sh selection.Shade
		if i < len(shades) {
			sh = shades[i]
		}
		in = append(in, entry{r, sh})
	}
	sort.SliceStable(in, func(a, b int) bool {
		ay := in[a].row.Bounds.Min.Y + in[a].row.Bounds.Max.Y
		by := in[b].row.Bounds.Min.Y + in[b].row.Bounds.Max.Y
		if ay != by {
			return ay < by
		}
		return in[a].row.Bounds.Min.X < in[b].row.Bounds.Min.X
	})
	outRows := make([]selection.Row, 0, len(in))
	outShades := make([]selection.Shade, 0, len(in))
	for _, e := range in {
		outRows = append(outRows, e.row)
		outShades = append(outShades, e.shade)
	}
	return outRows, outShades
}

// NavWindowOpenForTest reports whether a press would still be attributable at this moment.
//
// The window is internal state with no reader — the only consumer is noteMove, which asks about
// "now". A test about the window's BOUND cannot go through a move, because the thing under test
// is precisely that no move is attributed, and an absence has too many possible causes to be
// evidence on its own.
func NavWindowOpenForTest(w *InterfaceWatch, at time.Time) bool { return w.navOpen(at) }
