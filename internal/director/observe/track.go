package observe

import (
	"fmt"
	"image"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// SLOW DISCOVERY, FAST TRACKING: the half that decides what a fast read may conclude.
//
// # The measurement this exists because of
//
// [[Experiment-024-watching-a-player-navigate-a-real-menu]]: a player pressed a direction
// twelve times in 3.35 seconds, and all twelve landed inside TWO inferences. The session
// sampled 94 times in 244 seconds — 2.6s each, against a configured 500ms, because a
// ScreenParser inference blocks the cycle it runs in. No confirmation rule can reconstruct
// twelve visual states that were never sampled.
//
// The split is therefore not an optimisation, it is the only way the question can be asked.
// Discovering that there is a list of nine entries at these coordinates is expensive and rarely
// changes. Discovering which of them holds the mark is cheap — 597µs for nine rows, measured —
// and changes constantly.
//
// # The authority boundary, which is the whole of this file
//
//	Only a DISCOVERY may identify an interface. Tracking may only move the mark inside one.
//
// A fast look carries no roles, no labels and no semantic evidence. It cannot mint a list,
// cannot name one, cannot change which list is being watched, and cannot change how many
// entries that list is thought to have. It may move `Selected`, and it may fail.
//
// That is the same shape as the boundary the whole subsystem rests on: a cheap sensor may
// update a value, never establish a structure.
//
// # And a fast read is never presented as contemporaneous with the discovery
//
// Two timestamps, not one. `At` is when the geometry was discovered; `TrackedAt` is when the
// pixels were read. [[ADR-152-slow-discovery-fast-tracking]] names this as the defect to avoid
// rather than the feature to build, and `InterfaceBelief.Tracked` is what makes it impossible
// to read one for the other.

// TrackStaleness is how old a discovery's geometry may be when fast pixels are read against it.
//
// Three seconds, from the measured discovery cadence: inferences arrived every 2.6s on the live
// run, so a bound below that would refuse every fast look taken in an ordinary gap between two
// discoveries, and the feature would never run. Much above it and a fast read is being compared
// against an arrangement nobody has confirmed for longer than it takes to walk a menu.
//
// It is a bound on the GEOMETRY's age, not on the belief's. A belief may be older; what must be
// recent is the discovery that said where the rows are.
const TrackStaleness = 3 * time.Second

// TrackLook is one fast visual read over geometry a slow discovery found.
//
// It carries measurements and provenance and nothing else. There is no field for a role, a
// label or a name, and that absence is deliberate in the same way ShadowShape's is: a type that
// cannot carry a structural claim cannot be used to make one.
type TrackLook struct {
	// Discovery is the sample number of the inference whose geometry these rows came from.
	//
	// Provenance, so a belief can say "entry 3 of the list found at sample 41, read at
	// sample 58" rather than implying the two happened together.
	Discovery int `json:"discovery_sample"`
	// DiscoveredAt is when that inference ran; At is when THESE pixels were read.
	DiscoveredAt time.Time `json:"discovered_at,omitzero"`
	At           time.Time `json:"at,omitzero"`
	// Generation is the window lifecycle epoch the pixels belonged to. Geometry from two
	// generations describes two different worlds.
	Generation uint64 `json:"generation,omitempty"`
	// Rows is the remembered geometry, window-relative and normalised, in discovery order.
	// Shades is parallel to it, with a nil entry where a row could not be measured.
	Rows   []Region    `json:"rows,omitempty"`
	Shades [][]float64 `json:"shades,omitempty"`
	// Refused is why no pixels were read, empty when they were.
	//
	// Marco-authored. It is the channel the freshness checks the SAMPLER can make arrive on
	// — the window moved, the window was replaced, the capture failed — because those are
	// questions only the layer holding the live window can answer, and a look that silently
	// carried no shades would be indistinguishable from a screen that went black.
	Refused string `json:"refused,omitempty"`
	// CaptureMS is what the pixels cost. The number the cadence is chosen from, and the one
	// ADR-152 was written without.
	CaptureMS int64 `json:"capture_ms,omitempty"`
	// ReadUS is what measuring the rows cost, in microseconds, because in milliseconds it
	// rounds to nothing — which is the point.
	ReadUS int64 `json:"read_us,omitempty"`
}

// Fresh reports whether this look's geometry is recent enough to read pixels against.
func (l TrackLook) Fresh() bool {
	if l.DiscoveredAt.IsZero() || l.At.IsZero() {
		return false
	}
	age := l.At.Sub(l.DiscoveredAt)
	return age >= 0 && age <= TrackStaleness
}

// TrackingStats is what the fast path actually did, and is mostly a list of reasons it did not.
//
// Every refusal has its own number. "Tracking produced no readings" has five causes that send a
// reader to five different places — nothing identified a list, the window moved, the geometry
// went stale, the rows could not be measured, the rows were not the list's — and one counter for
// all of them would answer none of them. That is the same argument ShadowLabels makes.
type TrackingStats struct {
	// Offered is every fast look that reached the belief; Read is the ones that produced a
	// reading, and Moves the ones that saw the mark somewhere new.
	Offered int `json:"offered,omitempty"`
	Read    int `json:"read,omitempty"`
	Moves   int `json:"selection_moves,omitempty"`
	// NoBelief is looks taken while no list was identified. NOT a failure of tracking: it
	// is tracking correctly declining to be the thing that identifies one.
	NoBelief int `json:"no_list_identified,omitempty"`
	// Stale is looks whose discovery was older than TrackStaleness.
	Stale int `json:"stale_geometry,omitempty"`
	// Elsewhere is looks whose rows did not land inside the list the belief is about. The
	// candidate-identity check: geometry about a different part of the screen is not a
	// reading of this list, however fresh it is.
	Elsewhere int `json:"geometry_elsewhere,omitempty"`
	// Unmeasured is looks where no row could be measured at all.
	Unmeasured int `json:"unmeasured,omitempty"`
	// Refused is looks the sampler refused before reading pixels, by reason.
	//
	// Bounded, and the reasons are Marco's own words — the window moved, the window was
	// replaced, the capture failed. A map rather than a closed vocabulary because the
	// platform's reasons are not a vocabulary this package owns.
	Refused    int            `json:"refused,omitempty"`
	RefusedWhy map[string]int `json:"refused_why,omitempty"`
	// First and Last are the ends of the fast-look series, so the ACHIEVED cadence can be
	// reported rather than the configured one. "The interval is 250ms" is a setting; how
	// often a read actually happened is the question.
	First time.Time `json:"first,omitzero"`
	Last  time.Time `json:"last,omitzero"`
	// CaptureMSTotal and MaxCaptureMS are the cost of the pixels; ReadUSTotal the cost of
	// measuring them. Both totals, so a mean can be taken against Offered without this
	// type having to keep a sample.
	CaptureMSTotal int64 `json:"capture_ms_total,omitempty"`
	MaxCaptureMS   int64 `json:"capture_ms_max,omitempty"`
	ReadUSTotal    int64 `json:"read_us_total,omitempty"`
}

// maxRefusalReasons bounds the refusal map, so a platform that produces a new string per
// failure cannot grow a session result without limit.
const maxRefusalReasons = 8

// Cadence is the MEASURED mean interval between fast looks, zero when there were too few.
//
// From the timestamps rather than from the configuration, because the configured interval is
// what was asked for and this is what the machine did. The two differed by a factor of five on
// the slow path, which is the finding that made this whole file necessary.
func (t TrackingStats) Cadence() time.Duration {
	if t.Offered < 2 || t.First.IsZero() || t.Last.IsZero() {
		return 0
	}
	span := t.Last.Sub(t.First)
	if span <= 0 {
		return 0
	}
	return span / time.Duration(t.Offered-1)
}

// MeanCaptureMS is what a fast look's pixels cost on average.
func (t TrackingStats) MeanCaptureMS() float64 {
	if t.Offered == 0 {
		return 0
	}
	return float64(t.CaptureMSTotal) / float64(t.Offered)
}

// MeanReadUS is what measuring a look's rows cost on average.
func (t TrackingStats) MeanReadUS() float64 {
	if t.Read == 0 {
		return 0
	}
	return float64(t.ReadUSTotal) / float64(t.Read)
}

func (t *TrackingStats) refuse(why string) {
	t.Refused++
	if why == "" {
		why = "no reason given"
	}
	if t.RefusedWhy == nil {
		t.RefusedWhy = map[string]int{}
	}
	if _, seen := t.RefusedWhy[why]; !seen && len(t.RefusedWhy) >= maxRefusalReasons {
		return
	}
	t.RefusedWhy[why]++
}

// Track folds one fast visual read into the belief.
//
// Every early return below is an authority boundary or a freshness check, and each one is
// counted rather than silent: a fast path that quietly did nothing would be indistinguishable
// from one that was never wired, which is the failure this repository has recorded eight times.
func (w *InterfaceWatch) Track(look TrackLook) {
	t := &w.tracking
	t.Offered++
	if t.First.IsZero() {
		t.First = look.At
	}
	t.Last = look.At
	t.CaptureMSTotal += look.CaptureMS
	if look.CaptureMS > t.MaxCaptureMS {
		t.MaxCaptureMS = look.CaptureMS
	}

	if look.Refused != "" {
		t.refuse(look.Refused)
		return
	}
	// ONLY A DISCOVERY MAY IDENTIFY AN INTERFACE.
	//
	// Not a performance decision and not a threshold. A fast look has no roles and no
	// semantic evidence; it cannot tell a list from nine rectangles that happen to be in a
	// row. Letting one establish a list would make the cheapest sensor in the system the one
	// that decides what is on screen.
	//
	// Deleting this must fail TestATrackedLookCannotIdentifyAList.
	if w.belief.Status != InterfaceProvisional {
		t.NoBelief++
		return
	}
	// AND THE GEOMETRY MUST BE RECENT. A fast read against an arrangement nobody has
	// confirmed lately is a confident answer about rows that may not be there, which is
	// worse than no answer.
	//
	// Deleting this must fail TestAFastReadAgainstStaleGeometryIsRefused.
	if !look.Fresh() {
		t.Stale++
		return
	}

	var boxes []image.Rectangle
	var shades []selection.Shade
	for i, r := range look.Rows {
		if i >= len(look.Shades) || len(look.Shades[i]) != selection.Features {
			continue
		}
		boxes = append(boxes, scaleRegion(r))
		shades = append(shades, selection.Shade{look.Shades[i][0], look.Shades[i][1]})
	}
	if len(boxes) == 0 {
		t.Unmeasured++
		return
	}

	// THE SAME GROUPING AND THE SAME READER as the slow path, on purpose. A second
	// implementation of "which row is marked" would eventually disagree with the first, and
	// the disagreement would show up as a selection that changed when nothing moved.
	//
	// The reader's baselines are SHARED with the slow path, which is the point: a row's
	// baseline is built from the frames in which it was not chosen, and a fast path with its
	// own reader would start from nothing every time a discovery refreshed.
	rows := selection.GroupRows(boxes)
	shadeFor := map[image.Rectangle]selection.Shade{}
	for i, b := range boxes {
		shadeFor[b] = shades[i]
	}
	live := scaleRegion(w.belief.Where)
	var kept []selection.Row
	var keptShades []selection.Shade
	for _, r := range rows {
		if !r.Bounds.In(live.Inset(-2)) {
			continue
		}
		kept = append(kept, r)
		keptShades = append(keptShades, shadeFor[r.Bounds])
	}
	// CANDIDATE IDENTITY. Rows that do not land inside the list this belief is about are
	// geometry about a different part of the screen, and reading them would move a mark
	// inside a list they are not in.
	//
	// Deleting this must fail TestAFastReadOfTheWrongPartOfTheScreenIsRefused.
	if len(kept) == 0 {
		t.Elsewhere++
		return
	}

	start := time.Now()
	res := w.reader.ReadShades(kept, keptShades)
	t.ReadUSTotal += time.Since(start).Microseconds()
	t.Read++

	before := w.belief
	next := before
	// WHAT A FAST LOOK MAY CHANGE, and it is this list and no more: the mark, where the mark
	// is, the reader's own word for what it saw, and the fact that a fast look is what saw
	// it. Status, Where, Entries, FromShapes, Observations and After are all untouched —
	// they are the discovery's, and a tracking look is not a discovery.
	next.Selected, next.SelectedWhere = -1, Region{}
	if res.Outcome == selection.Selected && res.Row >= 0 && res.Row < len(kept) {
		next.Selected = res.Row
		next.SelectedWhere = unscaleRect(kept[res.Row].Bounds)
	}
	next.Reading = string(res.Outcome)
	// THE TWO TIMESTAMPS. A fast observation presented as contemporaneous with an old
	// discovery is the defect ADR-152 names; these three fields are what make that
	// impossible to do by accident.
	next.Tracked = true
	next.TrackedAt = look.At
	next.TrackedAfterMS = look.At.Sub(look.DiscoveredAt).Milliseconds()
	next.Because = fmt.Sprintf("tracked: %s — read %dms after the list was found at "+
		"sample %d", res.Why, next.TrackedAfterMS, look.Discovery)
	if next.Selected != before.Selected {
		t.Moves++
	}
	// The belief was confirmed by pixels, so it is not stale and the forget counter does
	// not advance. See InterfaceWatch.trackedThisLook.
	next.Stale, next.Unconfirmed, next.Empty = false, 0, 0
	w.trackedThisLook = true
	w.empty = 0
	w.settle(before, next, "")
}

// Tracking is what the fast path did this session.
func (w *InterfaceWatch) Tracking() TrackingStats { return w.tracking }
