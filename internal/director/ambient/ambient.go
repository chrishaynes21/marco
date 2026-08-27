// Package ambient is what Marco remembers from watching, and how little of it there is.
//
// # Observation time must not imply memory growth
//
// This is the property the whole ambient product rests on. Somebody leaves Marco watching for
// eight hours on one screen; the desktop is read thousands of times; and what Marco holds
// afterwards should be about the same size as what it held after five minutes, because nothing
// new happened.
//
// So nothing here is a log. Every fact is keyed on a durable semantic identity and carries a
// COUNT: a Place seen ten thousand times is one entry that says ten thousand, and a transition
// walked repeatedly is one edge with a tally. Growth tracks semantic novelty — how many different
// places and routes were seen — and never how long anybody watched.
//
// # And it is transient, deliberately
//
// This is not the Repertoire. Nothing in here is durable, nothing survives a Director restart,
// and losing it costs nothing that matters: the semantic memory is where knowledge lives, and
// ambient watching is not permission to write to it. What this buffer exists for is the present
// tense — what is on screen, what just happened, and enough recent shape that a future "learn
// what I just did" has something to read.
//
// # What it may hold
//
// Durable subject ids, counts, times and a closed provenance word. No labels, no window titles,
// no screen text, no coordinates, no screenshots. A transient buffer with weaker rules than the
// durable store would be the place a privacy boundary quietly stops applying, so it has the same
// ones.
package ambient

import (
	"sort"
	"sync"
	"time"
)

// How much is kept. Bounds are on DISTINCT things, because that is what can actually grow.
const (
	// MaxPlaces and MaxEdges bound how many different places and routes are remembered.
	//
	// A desktop with more than this many distinct screens in one watching session is
	// unusual, and the honest response to exceeding it is to forget the least recently seen
	// rather than to grow. Neither number bounds how LONG Marco watches, because repetition
	// costs nothing here — that is the point.
	MaxPlaces = 256
	MaxEdges  = 512
	// MaxMoves is the recent walk: the ordered tail, which is the only part of this buffer
	// that is a sequence rather than a tally.
	//
	// Short on purpose. It exists so a future "learn what I just did" can read back a route
	// somebody has only just walked, and a longer tail would be a history of the afternoon —
	// which is a different product with a different consent conversation.
	MaxMoves = 64
)

// Source is how a fact came to be observed. A closed vocabulary of two, and the distinction is
// load-bearing: a transition somebody walked and one Marco performed are different facts, and a
// future promotion policy that confused them would learn Marco's own behaviour back from itself.
type Source string

const (
	// ByHuman is somebody using their computer. The ordinary case for ambient watching.
	ByHuman Source = "human"
	// ByMarco is Marco's own performance, observed by the same substrate.
	ByMarco Source = "marco"
)

// Place is one screen, however many times it was seen.
type Place struct {
	Subject     string
	Application string
	Seen        int
	First, Last time.Time
}

// Edge is one observed transition, however many times it was walked.
type Edge struct {
	From, To    string
	Application string
	Seen        int
	// By counts each provenance separately rather than keeping the last one. A route
	// somebody walks and Marco later performs is the same route with two different kinds of
	// evidence behind it, and flattening them would lose which.
	By          map[Source]int
	First, Last time.Time
}

// Move is one entry in the recent walk.
//
// Kept as the name 36A gave it, and it is now a [Step]: the walk holds what somebody DID between
// two screens as well as which two they were. See action.go for why the middle of that sentence
// had to exist before "learn what I just did" could mean anything.
type Move = Step

// Buffer is everything ambient watching holds. Safe for concurrent use: the observation loop
// writes it and status reads it.
type Buffer struct {
	mu     sync.RWMutex
	places map[string]*Place
	edges  map[edgeKey]*Edge
	moves  []Step
	// order is the monotonic sequence stamped on every step. It counts steps RECORDED rather
	// than steps held, so a number the tail has forgotten is never reissued — which is what
	// makes a promotion watermark still mean something after the evidence behind it is gone.
	order int
	// consumed is the highest Order an explicit Learn has already promoted.
	//
	// A WATERMARK rather than a deletion. Erasing promoted steps would take the walk apart —
	// the step after a promoted one still needs its predecessor to know where it began — and
	// would make the recent trail lie about what just happened. This says only "these have
	// already become knowledge", which is the fact a second "learn what I just did" needs so
	// it does not learn the same afternoon twice.
	consumed int
	// dropped counts what the bounds discarded, so a full buffer reads as a full buffer
	// rather than as a quiet one.
	dropped int
}

type edgeKey struct{ from, to, application string }

// New is an empty buffer.
func New() *Buffer {
	return &Buffer{places: map[string]*Place{}, edges: map[edgeKey]*Edge{}}
}

// Saw records being on one screen.
//
// Idempotent in the only sense that matters: the ten-thousandth sighting of a place costs one
// increment and one timestamp, not an entry.
func (b *Buffer) Saw(application, subject string, at time.Time) {
	if subject == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if p, ok := b.places[subject]; ok {
		p.Seen++
		p.Last = at
		return
	}
	b.evictPlaceLocked()
	b.places[subject] = &Place{
		Subject: subject, Application: application, Seen: 1, First: at, Last: at,
	}
}

// Moved records a transition between two screens.
//
// The edge is a tally and the move is a tail entry: the first answers "how often does this
// happen", the second answers "what just happened", and one structure cannot do both without
// becoming a log.
func (b *Buffer) Moved(application, from, to string, by Source, at time.Time) {
	b.Walked(Step{From: from, To: to, Application: application, By: by, At: at})
}

// Walked records a transition and what the person did to make it.
//
// THE recording call. [Moved] is the shape of it that knows only where somebody went, kept
// because that is a real thing to observe — a screen change nothing was seen to cause is
// evidence, and refusing to record it would make the walk lie by omission.
//
// # It records the step even when the step cannot be learned
//
// An unrecognised endpoint, no attributed action, an action whose target had no name: every one
// of those produces a step that selection will later refuse. They are still recorded, because
// interpretation failure is not capture failure, and because "I saw you do something there and
// could not read it" is the answer somebody is owed instead of silence. See
// [SelectDemonstration], which is the only thing entitled to decide a step is not enough.
//
// The step is stamped with its Order here, inside the lock, so the sequence is total and matches
// the order the tail holds them in.
func (b *Buffer) Walked(s Step) {
	if s.From == "" || s.To == "" || s.From == s.To {
		return
	}
	// ADMISSION, not trust, for the vocabulary. A caller that invented an action word would
	// otherwise have put arbitrary text into a privacy-bounded record; a caller whose act is
	// unknown loses the ACT, never the step.
	kept := make([]Act, 0, len(s.Did))
	for _, a := range s.Did {
		if a.Kind.Known() {
			kept = append(kept, a)
		}
	}
	s.Did = kept
	s.FromShape, s.ToShape = s.FromShape.Clone(), s.ToShape.Clone()

	b.mu.Lock()
	defer b.mu.Unlock()

	key := edgeKey{from: s.From, to: s.To, application: s.Application}
	e, ok := b.edges[key]
	if !ok {
		b.evictEdgeLocked()
		e = &Edge{From: s.From, To: s.To, Application: s.Application,
			By: map[Source]int{}, First: s.At}
		b.edges[key] = e
	}
	e.Seen++
	e.By[s.By]++
	e.Last = s.At

	b.order++
	s.Order = b.order
	b.moves = append(b.moves, s)
	if len(b.moves) > MaxMoves {
		// The OLDEST goes. A tail that dropped the newest would answer "what just
		// happened" with what happened a while ago, which is the one question it has.
		b.moves = append(b.moves[:0], b.moves[len(b.moves)-MaxMoves:]...)
		b.dropped++
	}
}

// Promoted records that an explicit Learn has turned everything up to and including one step into
// durable knowledge.
//
// Monotone: a watermark never moves backwards, so an out-of-order call from a slower promotion
// cannot re-expose evidence a later one already consumed.
//
// Deleting this must fail TestTheSameAfternoonIsNotLearnedTwice.
func (b *Buffer) Promoted(through int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if through > b.consumed {
		b.consumed = through
	}
}

// evictPlaceLocked and evictEdgeLocked make room by forgetting the least recently seen.
//
// Least RECENTLY seen rather than least often: a screen somebody visited constantly this morning
// and not since is less useful now than one they saw once a minute ago, and ambient evidence is
// about the present tense.
func (b *Buffer) evictPlaceLocked() {
	if len(b.places) < MaxPlaces {
		return
	}
	var oldest string
	var when time.Time
	for id, p := range b.places {
		if oldest == "" || p.Last.Before(when) {
			oldest, when = id, p.Last
		}
	}
	delete(b.places, oldest)
	b.dropped++
}

func (b *Buffer) evictEdgeLocked() {
	if len(b.edges) < MaxEdges {
		return
	}
	var oldest edgeKey
	var when time.Time
	first := true
	for k, e := range b.edges {
		if first || e.Last.Before(when) {
			oldest, when, first = k, e.Last, false
		}
	}
	delete(b.edges, oldest)
	b.dropped++
}

// View is a snapshot of what is held, for status and diagnostics.
type View struct {
	Places []Place
	Edges  []Edge
	Recent []Step
	// Consumed is the highest Order an explicit Learn has already promoted. Selection reads
	// it; nothing else should need to.
	Consumed int
	Dropped  int
}

// Look reads the buffer without holding it.
//
// Ordered deterministically — most recently seen first — because a status surface that reordered
// itself between two reads of an unchanged desktop would look like something was happening.
func (b *Buffer) Look() View {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := View{Dropped: b.dropped, Consumed: b.consumed}
	for _, p := range b.places {
		out.Places = append(out.Places, *p)
	}
	for _, e := range b.edges {
		copied := *e
		copied.By = map[Source]int{}
		for k, v := range e.By {
			copied.By[k] = v
		}
		out.Edges = append(out.Edges, copied)
	}
	out.Recent = append(out.Recent, b.moves...)

	sort.Slice(out.Places, func(i, j int) bool {
		if out.Places[i].Last.Equal(out.Places[j].Last) {
			return out.Places[i].Subject < out.Places[j].Subject
		}
		return out.Places[i].Last.After(out.Places[j].Last)
	})
	sort.Slice(out.Edges, func(i, j int) bool {
		if out.Edges[i].Last.Equal(out.Edges[j].Last) {
			return out.Edges[i].From+out.Edges[i].To < out.Edges[j].From+out.Edges[j].To
		}
		return out.Edges[i].Last.After(out.Edges[j].Last)
	})
	return out
}

// Size is how many distinct things are held. The number that must not grow with time.
func (b *Buffer) Size() (places, edges, recent int) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.places), len(b.edges), len(b.moves)
}

// Forget empties the buffer. Used when watching stops: what Marco holds about the present tense
// is not something to keep after it stops paying attention.
func (b *Buffer) Forget() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.places = map[string]*Place{}
	b.edges = map[edgeKey]*Edge{}
	b.moves = nil
	b.dropped = 0
	// The SEQUENCE goes too, along with the watermark that is expressed in it. They mean
	// nothing without each other: keeping a watermark of 40 across a forgetting would suppress
	// the first forty steps of the next watching session, which are a different afternoon.
	b.order, b.consumed = 0, 0
}
