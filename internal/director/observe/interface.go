package observe

import (
	"fmt"
	"image"
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
	// Stale says nothing has confirmed this belief since. A belief that is merely
	// unconfirmed must not read like one that was just seen, and on a live Director most
	// looks bring nothing: the shadow cadence lets an inference start every two seconds
	// while the session samples twice a second, so three looks in four are a property of
	// the cadence rather than of the screen.
	Stale bool `json:"stale,omitempty"`
	// Unconfirmed is how many consecutive looks have brought no evidence at all.
	Unconfirmed int `json:"looks_unconfirmed,omitempty"`
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
	// since that have carried none.
	looks   int
	missed  int
	moments []InterfaceMoment
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
	if !look.Observed {
		w.unconfirmed(look)
		return
	}
	w.looks++
	w.missed = 0
	before := w.belief

	var boxes []image.Rectangle
	var shades []selection.Shade
	for _, r := range look.Regions {
		if len(r.Shade) != selection.Features {
			continue
		}
		boxes = append(boxes, scaleRegion(r.Region))
		shades = append(shades, selection.Shade{r.Shade[0], r.Shade[1]})
	}
	if len(boxes) == 0 {
		w.settle(before, InterfaceBelief{
			Status: InterfaceUnknown, Selected: -1, Reading: "insufficient_evidence",
			Because:      "no shaded regions were proposed this look",
			Observations: w.looks, Sample: look.Sequence, At: look.At,
		}, "")
		return
	}

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

	var kept []selection.Row
	var keptShades []selection.Shade
	for i, r := range rows {
		if r.Bounds.In(live.Inset(-2)) {
			kept = append(kept, r)
			keptShades = append(keptShades, rowShades[i])
		}
	}
	next.Entries = len(kept)
	res := w.reader.ReadShades(kept, keptShades)
	next.Reading = string(res.Outcome)
	if res.Outcome == selection.Selected && res.Row >= 0 && res.Row < len(kept) {
		next.Selected = res.Row
		next.SelectedWhere = unscaleRect(kept[res.Row].Bounds)
	}
	next.Because = why + "; reading: " + res.Why
	w.settle(before, next, "")
}

// unconfirmed folds a look that brought no evidence.
//
// The belief is KEPT and marked, not replaced. Nothing was seen, so nothing contradicts it;
// what changed is only its age, and that is the part a consumer has to be told. Past
// interfaceForgetLooks it is dropped — with the reason, so "nobody has looked lately" cannot
// be read as "the screen is empty".
func (w *InterfaceWatch) unconfirmed(look InterfaceLook) {
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
		w.settle(before, InterfaceBelief{
			Status: InterfaceUnknown, Selected: -1, Reading: "insufficient_evidence",
			Observations: w.looks, Stale: true, Unconfirmed: w.missed,
			Sample: before.Sample, At: before.At,
			Because: fmt.Sprintf("no evidence has been observed for %d look(s): %s",
				w.missed, why),
		}, why)
		return
	}
	next.Because = fmt.Sprintf("%s (unconfirmed for %d look(s); the evidence under this "+
		"belief is sample %d)", why, w.missed, before.Sample)
	w.settle(before, next, why)
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
