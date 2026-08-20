// Package timeline turns successive perception cycles into an ordered event log.
//
// # Why this exists on the DIRECTOR side
//
// A front-end that wants to show "what is Marco doing right now" needs events, and the
// service protocol is request/response: responses stream only inside a command exchange,
// so nothing publishes "vision contributed" or "fusion rejected an anonymous icon". The
// obvious shortcut is to let the front-end poll perception twice and subtract. That would
// put derivation in the client, and then every client — the overlay, a future web view, a
// harness — would derive it slightly differently, and the Director would no longer be the
// authority on its own history.
//
// So the diffing happens here, once, beside the thing being diffed. A client renders.
//
// # This adds no reasoning
//
// Every event is a restatement of a number the fusion report or the world already carried:
// a count changed, a source stopped reporting, an element persisted long enough to be worth
// mentioning. Nothing here decides anything, nothing here is consulted by planning, and
// removing it would change no behaviour — only what a HUD can show.
package timeline

import (
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Kind names what happened. A closed set: a client renders these and nothing else, so a
// new kind is a deliberate addition rather than a string that appeared.
type Kind string

const (
	// KindCycle is one completed observation round.
	KindCycle Kind = "cycle"
	// KindSourceContributed is a source that produced evidence after producing none.
	KindSourceContributed Kind = "source_contributed"
	// KindSourceSilent is a source that produced evidence and has stopped.
	//
	// The counterpart matters more than it looks: a source that quietly stops is
	// invisible in a snapshot, because "0 observations" looks the same whether it never
	// worked or stopped working a minute ago.
	KindSourceSilent Kind = "source_silent"
	// KindSourceDegraded is a source fusion expected and did not get.
	KindSourceDegraded Kind = "source_degraded"
	// KindTextAttached is text evidence that found structure to attach to.
	KindTextAttached Kind = "text_attached"
	// KindRejected is evidence that produced no element and reinforced none.
	KindRejected Kind = "rejected"
	// KindConflict is two sources disagreeing about one field.
	KindConflict Kind = "conflict"
	// KindElementStable is an element that has persisted long enough to be trusted as
	// a thing rather than a flicker.
	KindElementStable Kind = "element_stable"
	// KindElementLost is a previously stable element that is no longer present.
	//
	// Only ever emitted for something that was PROMOTED. A transient element vanishing
	// is not news; a thing the Director had come to rely on vanishing is.
	KindElementLost Kind = "element_lost"
	// KindAppChanged is the foreground application changing.
	KindAppChanged Kind = "app_changed"
	// KindWindowChanged is the active window changing under the same application.
	KindWindowChanged Kind = "window_changed"
)

// Event is one thing that happened, ready to render.
//
// Seq is monotonic and gap-free, so a client that polls with a cursor can tell the
// difference between "nothing happened" and "I missed some" — the second is what makes a
// dropped Director event detectable rather than silent.
type Event struct {
	Seq  uint64    `json:"seq"`
	At   time.Time `json:"at"`
	Kind Kind      `json:"kind"`

	// Source is the provider this concerns, where one does.
	Source string `json:"source,omitempty"`
	// Count is the quantity the event is about: observations in a cycle, evidence
	// rejected, elements promoted.
	Count int `json:"count,omitempty"`
	// Detail is a short rendered phrase. Never a label, never user content — see the
	// privacy note on Recorder.Observe.
	Detail string `json:"detail,omitempty"`
	// Duration is how long the cycle took, for KindCycle.
	Duration time.Duration `json:"duration,omitempty"`
}

// stablePromoteAfter is how many consecutive cycles an element must appear in before it
// is announced as stable.
//
// Three rather than two: two consecutive cycles is a coincidence a busy screen produces
// constantly, and a HUD that announced every one of them would scroll uselessly.
const stablePromoteAfter = 3

// Recorder folds cycles into events and keeps a bounded log.
//
// Not safe for concurrent use; the caller holds the lock it already holds to record a
// cycle. That is deliberate — a second lock here would be a second thing to get wrong,
// and the only writer is the observe path.
type Recorder struct {
	events []Event
	next   uint64
	limit  int

	// seen counts consecutive cycles per element, for stability promotion.
	seen map[directorapi.ElementID]int
	// firstSeen is when the current unbroken run of an element began, so a world
	// payload can report an AGE rather than only a cycle count.
	firstSeen map[directorapi.ElementID]time.Time
	// promoted remembers what has already been announced, so an element that stays
	// stable is announced once rather than every cycle.
	promoted map[directorapi.ElementID]bool

	lastBySource map[string]int
	lastApp      string
	lastWindow   string
	started      bool
}

// New builds a recorder holding at most limit events.
func New(limit int) *Recorder {
	if limit <= 0 {
		limit = 256
	}
	return &Recorder{
		limit:        limit,
		seen:         map[directorapi.ElementID]int{},
		firstSeen:    map[directorapi.ElementID]time.Time{},
		promoted:     map[directorapi.ElementID]bool{},
		lastBySource: map[string]int{},
	}
}

// Observe folds one completed cycle into the log.
//
// # Privacy
//
// No event carries a label, a value, a window title or any other content read from the
// screen. Everything here is a count, a source name, a role or a duration. That is not a
// restriction this package has to enforce carefully — it is a property of what it is given
// to say — and it means the event log is safe to render on a stream, which is precisely
// where a HUD ends up.
func (r *Recorder) Observe(cycle observation.Cycle, report fusion.Report, w *directorapi.WorldState) {
	now := cycle.StartedAt
	if now.IsZero() {
		now = time.Now()
	}

	r.emit(Event{
		At: now, Kind: KindCycle,
		Count:    len(cycle.Observations),
		Duration: cycle.Duration(),
	})

	// Sources, sorted so a cycle always produces the same event order. Map iteration
	// would make the log non-deterministic, and a log that reorders itself is one
	// nobody can diff against a previous run.
	bySource := map[string]int{}
	for src, n := range report.BySource {
		bySource[string(src)] = n
	}
	for _, src := range union(r.lastBySource, bySource) {
		was, is := r.lastBySource[src], bySource[src]
		switch {
		case was == 0 && is > 0 && r.started:
			r.emit(Event{At: now, Kind: KindSourceContributed, Source: src, Count: is})
		case was > 0 && is == 0:
			r.emit(Event{At: now, Kind: KindSourceSilent, Source: src})
		}
	}
	r.lastBySource = bySource

	for _, d := range report.Degraded {
		r.emit(Event{At: now, Kind: KindSourceDegraded,
			Source: string(d.Source), Detail: d.Reason})
	}
	// FilledLabel specifically: text that gave a previously unnamed element its name.
	// That is the event worth showing — "OCR read a label" — as distinct from text that
	// merely corroborated a name another source already had.
	if n := report.Text.FilledLabel; n > 0 {
		r.emit(Event{At: now, Kind: KindTextAttached, Count: n})
	}
	if report.Rejected > 0 {
		r.emit(Event{At: now, Kind: KindRejected, Count: report.Rejected})
	}
	for _, c := range report.Conflicts {
		r.emit(Event{At: now, Kind: KindConflict, Detail: c.Field})
	}

	r.observeWorld(now, w)
	r.started = true
}

// observeWorld emits the world-scoped events: focus moves and element stability.
func (r *Recorder) observeWorld(now time.Time, w *directorapi.WorldState) {
	if w == nil {
		return
	}

	app := ""
	if w.ActiveApp != nil {
		app = w.ActiveApp.Name
	}
	appChanged := app != r.lastApp
	if appChanged && r.started {
		r.emit(Event{At: now, Kind: KindAppChanged, Detail: app})
	}
	r.lastApp = app

	win := ""
	if w.ActiveWindow != nil {
		win = string(*w.ActiveWindow)
	}
	// Only when the app did NOT change: a new app naturally brings a new window, and
	// reporting both would double every switch. Tested against appChanged captured
	// BEFORE lastApp is updated — comparing against the updated field is always true,
	// which silently disables the guard.
	if win != r.lastWindow && !appChanged && r.started {
		r.emit(Event{At: now, Kind: KindWindowChanged})
	}
	r.lastWindow = win

	// Stability. Counted over CONSECUTIVE cycles: an element that vanishes and returns
	// starts again, because the question is whether a thing is holding still, not
	// whether it has ever been seen.
	present := make(map[directorapi.ElementID]bool, len(w.Elements))
	ids := make([]directorapi.ElementID, 0, len(w.Elements))
	for id := range w.Elements {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	promoted := 0
	for _, id := range ids {
		present[id] = true
		r.seen[id]++
		if _, known := r.firstSeen[id]; !known {
			r.firstSeen[id] = now
		}
		if r.seen[id] == stablePromoteAfter && !r.promoted[id] {
			r.promoted[id] = true
			promoted++
		}
	}
	lost := 0
	for id := range r.seen {
		if present[id] {
			continue
		}
		// Only a PROMOTED element is worth reporting as lost. A transient one
		// vanishing is not news; a thing the Director had come to rely on is.
		if r.promoted[id] {
			lost++
		}
		delete(r.seen, id)
		delete(r.firstSeen, id)
		delete(r.promoted, id)
	}
	if promoted > 0 {
		// One event for the batch rather than one per element: a window opening
		// promotes forty controls at once, and forty identical lines is not a feed.
		r.emit(Event{At: now, Kind: KindElementStable, Count: promoted})
	}
	if lost > 0 {
		r.emit(Event{At: now, Kind: KindElementLost, Count: lost})
	}
}

func (r *Recorder) emit(e Event) {
	r.next++
	e.Seq = r.next
	r.events = append(r.events, e)
	if len(r.events) > r.limit {
		r.events = r.events[len(r.events)-r.limit:]
	}
}

// Stability is how long one element has been continuously present.
//
// Cycles counts the CURRENT unbroken run: an element that vanished and came back starts
// again, because the question a HUD is asking is whether a thing is holding still now.
type Stability struct {
	Cycles    int
	FirstSeen time.Time
	Promoted  bool
}

// Stability reports the current run for one element.
func (r *Recorder) Stability(id directorapi.ElementID) Stability {
	return Stability{
		Cycles:    r.seen[id],
		FirstSeen: r.firstSeen[id],
		Promoted:  r.promoted[id],
	}
}

// Oldest is the lowest sequence number still retained, or 0 when the log is empty.
//
// This is what makes loss DETECTABLE rather than silent: a client that asks from cursor C
// and gets back an oldest of more than C+1 knows the log rolled past it. Without it, a
// client cannot tell "nothing happened" from "everything happened and I missed it".
func (r *Recorder) Oldest() uint64 {
	if len(r.events) == 0 {
		return 0
	}
	return r.events[0].Seq
}

// Newest is the highest sequence number issued.
func (r *Recorder) Newest() uint64 { return r.next }

// Since returns events after the given sequence number, oldest first, and the newest
// sequence issued.
//
// The second return is what lets a client detect loss: if the oldest event it gets back is
// newer than cursor+1, the log rolled over and something was missed. A HUD may drop its own
// frames; it may not silently drop the Director's events.
func (r *Recorder) Since(cursor uint64, limit int) ([]Event, uint64) {
	if limit <= 0 || limit > r.limit {
		limit = r.limit
	}
	out := make([]Event, 0, limit)
	for _, e := range r.events {
		if e.Seq > cursor {
			out = append(out, e)
		}
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, r.next
}

// union returns every key of both maps, sorted.
func union(a, b map[string]int) []string {
	set := map[string]bool{}
	for k := range a {
		set[k] = true
	}
	for k := range b {
		set[k] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
