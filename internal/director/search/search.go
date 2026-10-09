// Package search is the Director looking for something that is not in front of it.
//
// Everything above this package acts on what it can see. Perception takes one reading of the
// current screen, resolution asks that reading a question, and a question the reading cannot
// answer is refused — honestly, and finally. There was no notion of looking somewhere else.
//
// That gap is the shape of most whole-task failures the project has measured. Each individual
// operation works; the task fails because the thing the next operation needs is not on screen
// yet, and nothing goes to find it.
//
// # A search is a wait with a move in it
//
// This is deliberately not a new mechanism. [wait/engine] already runs the loop:
//
//	observe → evaluate → satisfied? stop. otherwise pause, and observe again
//
// and documents why the pause is only a poll interval: "nothing here concludes that a condition
// holds because time passed". A wait is therefore a search over TIME with no moves — it looks at
// the same place repeatedly and waits for the place to change.
//
// A search replaces the pause with a MOVE: a bounded change to what can be seen. Everything else
// is the same machinery and the same rules — the same [conditions.WorldCondition] to say what is
// being looked for, the same three-valued evaluation, the same Unknown gate, the same hard bound.
// What is new is one honest answer the wait engine has no need for:
//
//	found        the condition held somewhere, and here is the world it held in
//	exhausted    every place this pattern can reach was looked at and read; it is not there
//	unreadable   some place could not be read, so "it is not there" is not established
//	stopped      cancelled, or the move itself failed
//
// `exhausted` is the point of the package. A single failed look says "not in this reading". An
// exhausted search says "not anywhere I could get to", which is a far stronger statement — and
// the only one that licenses telling somebody their text is not in the document.
//
// # Why the same pattern serves a game menu
//
// A search is made of a condition and a repeated move. Nothing in the loop knows that the move
// is a scroll or that the world came from the accessibility tree. Desktop text search is the
// first pattern because it is the one a real task earned; a game's menu is the next, and it
// differs only in which move is repeated and which provider answered.
//
// What it must NOT differ in is the gate below. A search performs moves WITHOUT BEING ASKED, so
// the set of moves it may make is closed, and adding to it is a decision with its own argument.
package search

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/wait/conditions"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/evaluation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Observer produces a fresh world. The same shape the wait engine and the execution pipeline
// use, so a search looks through exactly the path a command does — providers, fusion, identity.
type Observer func(ctx context.Context) (directorapi.WorldState, error)

// Performer carries out one looking move.
//
// Injected rather than imported, for the reason everything else in the Director takes its
// collaborators: performing an action needs the executor, and the executor is what calls this.
type Performer func(ctx context.Context, m Move) error

// Move is one step of a search pattern.
//
// It carries a real [directorapi.Action], so a move reaches the desktop by lowering to legal
// Marco like everything else does — [[ADR-005-legal-marco-only]] is not weakened by the
// Director deciding to look somewhere.
type Move struct {
	Action directorapi.Action
	// Phrase describes the move to a person ("scroll the list down one page"). Written here
	// rather than derived from the action because the sentence a search reports is about
	// LOOKING, and "scroll to the bottom" read out of an action describes an intention nobody
	// had.
	Phrase string
}

// Zero reports whether the move is absent.
func (m Move) Zero() bool { return m.Action == nil }

// Pattern is a bounded way of changing what can be seen.
//
// One repeated move and a budget, because that is what both known patterns are: scrolling a
// region a page at a time, and stepping through a menu. A pattern of several different moves
// would be a plan, and a plan about where to look is a thing nobody has needed yet.
type Pattern struct {
	// Step is the move repeated to look somewhere new.
	Step Move
	// Home puts the view back where the search found it. Zero when the pattern cannot restore
	// what it changed, which is reported rather than hidden — see [Result.Restored].
	Home Move
	// Budget is the most times Step may be performed. A search is always bounded: a loop that
	// kept scrolling would be the Director deciding on its own how much of somebody's
	// afternoon to spend.
	Budget int
	// Describe names the pattern for a person ("scrolling the Lines list").
	Describe string
}

// Outcome is what a search concluded. Four words, and the difference between the middle two is
// the whole reason this package exists.
type Outcome string

const (
	// Found: the condition held. World carries the reading it held in.
	Found Outcome = "found"
	// Exhausted: every place the pattern can reach was looked at AND READ, and the condition
	// held in none of them. The only outcome that licenses saying a thing is not there.
	Exhausted Outcome = "exhausted"
	// Unreadable: somewhere along the way a place could not be read, so absence is not
	// established. [[ADR-006-unknown-is-not-false]] — looking at a window Marco cannot read
	// is not the same as looking at a window that does not contain the thing.
	Unreadable Outcome = "unreadable"
	// Stopped: cancelled, or a move failed. Says nothing about where the thing is.
	Stopped Outcome = "stopped"
)

// Look is one place a search looked.
type Look struct {
	// After is the move that got here, empty for the first look.
	After string `json:"after,omitempty"`
	// State is what the condition made of this place.
	State evaluation.State `json:"state"`
	// Detail is the condition's own sentence about it.
	Detail string `json:"detail,omitempty"`
	// Relooked records that this place was read twice because the first reading did not
	// represent it. See [[ADR-137-a-thin-reading-is-not-an-answer]].
	Relooked bool `json:"relooked,omitempty"`
}

// Result is what a search concluded, and the account of how.
type Result struct {
	Outcome Outcome `json:"outcome"`
	// World is the reading the condition held in. Nil unless Found.
	World *directorapi.WorldState `json:"-"`
	// Looked is how many places were looked at, counting the one the search started in.
	Looked int `json:"looked"`
	// Read is how many of those could be read at all. Equal to Looked for an honest
	// Exhausted; lower is what makes the outcome Unreadable instead.
	Read int `json:"read"`
	// Moved is how many moves were performed.
	Moved int `json:"moved"`
	// Restored reports whether the view was put back. False with a non-Found outcome means
	// the view was LEFT WHERE IT LOOKED, which the person can see on their screen and so must
	// be able to read in the sentence.
	Restored bool `json:"restored"`
	// Reason is one sentence for a person.
	Reason string `json:"reason"`
	// Trail is every look, in order, for `director explain`.
	Trail []Look `json:"trail,omitempty"`
}

// Searcher looks for things.
type Searcher struct {
	observe Observer
	perform Performer
	// stop is the Director's own cancellation signal, polled alongside the context, for the
	// reason the wait engine holds both: a context is cancelled in-process, while a spoken
	// "stop" arrives in a different process entirely.
	stop func() bool
}

// New builds a searcher over an observer and a performer.
func New(observe Observer, perform Performer) *Searcher {
	return &Searcher{observe: observe, perform: perform}
}

// WithStopCheck attaches the Director's cancellation signal.
func (s *Searcher) WithStopCheck(stop func() bool) *Searcher {
	s.stop = stop
	return s
}

// ErrNotALookingMove is returned for a pattern whose move is not one a search may make unasked.
var ErrNotALookingMove = errors.New("not a move a search may make without being asked")

// Find looks for what the condition describes, moving as the pattern says, until it is found or
// the pattern runs out of places.
//
// The loop, and why each part is where it is:
//
//	look     observe and evaluate. The first look costs nothing extra: it is the reading the
//	         caller would have taken anyway, and a search that moved before looking would
//	         scroll past a thing already on screen.
//	found?   stop. The world it held in goes back to the caller, so the next step resolves
//	         against the reading the thing was actually seen in rather than a fresher one it
//	         may have left again.
//	room?    the budget, and whether the last move changed anything.
//	move     then look again.
func (s *Searcher) Find(ctx context.Context, what conditions.WorldCondition, p Pattern) (Result, error) {
	if what == nil {
		return Result{}, errors.New("a search needs something to look for")
	}
	if err := p.validate(); err != nil {
		return Result{}, err
	}

	res := Result{Outcome: Stopped}
	var previous *directorapi.WorldState

	for {
		if s.cancelled(ctx) {
			res.Reason = fmt.Sprintf("stopped after looking in %d place(s)", res.Looked)
			return s.restore(ctx, p, res), nil
		}

		world, look, err := s.look(ctx, what)
		if err != nil {
			res.Reason = "could not look: " + err.Error()
			return s.restore(ctx, p, res), nil
		}
		if res.Moved > 0 {
			look.After = p.Step.Phrase
		}
		res.Looked++
		res.Trail = append(res.Trail, look)
		if look.State != evaluation.Unknown {
			res.Read++
		}

		if look.State == evaluation.Satisfied {
			res.Outcome, res.World = Found, &world
			res.Reason = foundReason(what, p, res)
			// NOT restored. The thing is here, the next step is about to act on it, and
			// scrolling away from what was just found would undo the search.
			return res, nil
		}

		// THE VIEW STOPPED MOVING, so no further move will show anything new.
		//
		// [[ADR-007-no-progress-no-repetition]] in a third disguise. A region already at its
		// bottom accepts a page-down and reports success forever; without this the search
		// spends its whole budget re-reading one screen and then claims to have looked
		// everywhere. An unchanged world is not a place that was searched.
		if previous != nil && sameView(*previous, world) {
			res.Outcome = s.verdict(res)
			res.Reason = stuckReason(what, p, res)
			return s.restore(ctx, p, res), nil
		}
		previous = &world

		if res.Moved >= p.Budget {
			res.Outcome = s.verdict(res)
			res.Reason = budgetReason(what, p, res)
			return s.restore(ctx, p, res), nil
		}

		if err := s.perform(ctx, p.Step); err != nil {
			res.Reason = fmt.Sprintf("%s failed after %d place(s): %v",
				p.Step.Phrase, res.Looked, err)
			return s.restore(ctx, p, res), nil
		}
		res.Moved++
	}
}

// verdict decides between the two negative answers, and this is the sharpest rule here.
//
// "It is not there" is only honest if every place was actually read. One place that could not be
// read is one place the thing could have been, so the search has not established absence — it has
// established that it did not see it, which is a different sentence and a different word.
func (s *Searcher) verdict(res Result) Outcome {
	if res.Read == res.Looked {
		return Exhausted
	}
	return Unreadable
}

// look takes one reading and asks the condition about it.
//
// A READING THAT DID NOT REPRESENT THE PLACE IS NOT AN ANSWER ABOUT THE PLACE, so an Unknown is
// read once more before being believed. Measured upstream: twelve identical requests to one
// window gave nine answers and two refusals saying the application exposed only containers, so
// the thin reading was the accessibility tree caught mid-population.
// [[ADR-137-a-thin-reading-is-not-an-answer]].
//
// Free here for the same reason it is free there: looking has no effect, so nothing can be
// applied twice. It matters more here than there, because an Unknown costs a search its
// `exhausted` — with one unreadable place in five, a search that believed the first reading
// would almost never be able to say a thing is absent.
func (s *Searcher) look(ctx context.Context, what conditions.WorldCondition) (
	directorapi.WorldState, Look, error) {

	world, err := s.observe(ctx)
	if err != nil {
		return directorapi.WorldState{}, Look{}, err
	}
	ev := what.Evaluate(world)
	if ev.State != evaluation.Unknown {
		return world, Look{State: ev.State, Detail: ev.Explanation}, nil
	}

	fresh, ferr := s.observe(ctx)
	if ferr != nil {
		// The second reading failed outright, so the first one stands as the only thing
		// known about this place — and it was Unknown.
		return world, Look{State: ev.State, Detail: ev.Explanation, Relooked: true}, nil
	}
	again := what.Evaluate(fresh)
	return fresh, Look{State: again.State, Detail: again.Explanation, Relooked: true}, nil
}

// restore puts the view back, for every outcome but Found.
//
// A search that did not find anything has still moved somebody's screen. Leaving it scrolled to
// the bottom of a list, with nothing to show for it, is the Director changing what a person is
// looking at as a side effect of being wrong. So the view goes back where it was — and when the
// pattern cannot do that, the Result says so rather than letting the sentence imply nothing
// happened.
func (s *Searcher) restore(ctx context.Context, p Pattern, res Result) Result {
	if res.Moved == 0 || p.Home.Zero() {
		return res
	}
	// Deliberately not gated on cancellation. A cancelled search has already moved the view,
	// and "stop" is not a request to leave it somewhere else.
	if err := s.perform(context.WithoutCancel(ctx), p.Home); err == nil {
		res.Restored = true
		res.Reason += ", and the view was put back"
		return res
	}
	res.Reason += ", and the view was left where it looked"
	return res
}

func (s *Searcher) cancelled(ctx context.Context) bool {
	if ctx.Err() != nil {
		return true
	}
	return s.stop != nil && s.stop()
}

// validate holds the gate. A search performs moves WITHOUT BEING ASKED, so the set it may draw
// from is closed, and every addition is a decision with its own argument.
func (p Pattern) validate() error {
	if p.Budget <= 0 {
		return errors.New("a search needs a budget; an unbounded one is the Director deciding " +
			"for itself how long to spend looking")
	}
	if p.Step.Zero() {
		return errors.New("a search pattern needs a move")
	}
	if !MaySearchWith(p.Step.Action) {
		return fmt.Errorf("%w: %s", ErrNotALookingMove, p.Step.Phrase)
	}
	if !p.Home.Zero() && !MaySearchWith(p.Home.Action) {
		return fmt.Errorf("%w: %s", ErrNotALookingMove, p.Home.Phrase)
	}
	return nil
}

// MaySearchWith reports whether an action is one a search may perform of its own accord.
//
// # The criterion, which is not "harmless"
//
// A looking move changes WHAT CAN BE SEEN, or what is highlighted, and COMMITS NOTHING. Both
// halves matter. Scrolling qualifies on both: the view moves, no state the application cares
// about changes, and the move is reversible by scrolling back. A click does not qualify on
// either — it is the whole category of thing a person has to ask for.
//
// # The closed list, today
//
// One entry: a semantic scroll. That is the pattern a real task earned, and it is the only one
// whose safety argument has been made.
//
// The next one is a game's menu — step through the entries until the one being looked for is
// highlighted — and it is NOT in this list, because its argument is genuinely harder and has
// not been made. Arrow keys in a menu move a highlight, which commits nothing and is reversible;
// arrow keys in a list box change a SELECTION, which some applications treat as a choice, and
// in a game they may be the input the game is reading. Telling those apart needs to know what
// kind of thing is in front of Marco, which is exactly what the vision and game-pack work is
// for. Until then a search refuses to use them, and says which move it refused.
func MaySearchWith(a directorapi.Action) bool {
	sem, ok := a.(directorapi.SemanticAction)
	if !ok {
		return false
	}
	return sem.Kind == directorapi.SemanticScroll
}

// sameView reports whether two readings show the same thing.
//
// A fingerprint over what is on screen — each element's identity, role and where it sits —
// because that is precisely the question: did moving show me anything new. Labels alone are not
// enough (a list scrolled by one row has nearly the same labels) and ids alone are not enough
// (a virtualised list recycles them), so the bounds are in it.
//
// Element COUNT is not enough on its own either, and was the first thing tried: a list scrolled
// by a page has the same number of rows in it.
func sameView(a, b directorapi.WorldState) bool {
	return viewPrint(a) == viewPrint(b)
}

func viewPrint(w directorapi.WorldState) string {
	parts := make([]string, 0, len(w.Elements))
	for _, el := range w.Elements {
		if el == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s|%s|%s|%d,%d,%d,%d|%s",
			el.ID, el.Role, el.Label,
			el.Bounds.X, el.Bounds.Y, el.Bounds.Width, el.Bounds.Height,
			el.Value))
	}
	sort.Strings(parts)
	return strings.Join(parts, "\n")
}

func foundReason(what conditions.WorldCondition, p Pattern, res Result) string {
	if res.Moved == 0 {
		return fmt.Sprintf("%s, without having to look anywhere else", what.Description())
	}
	return fmt.Sprintf("%s, found by %s (%d place(s))", what.Description(), p.Describe, res.Looked)
}

func stuckReason(what conditions.WorldCondition, p Pattern, res Result) string {
	where := fmt.Sprintf("%s showed nothing new after %d place(s)", p.Describe, res.Looked)
	if res.Outcome == Unreadable {
		return fmt.Sprintf("%s, and %d of them could not be read, so %q may still be there",
			where, res.Looked-res.Read, what.Description())
	}
	return fmt.Sprintf("%s, so %s is as far as it goes: %s", where, p.Describe,
		notThere(what))
}

func budgetReason(what conditions.WorldCondition, p Pattern, res Result) string {
	where := fmt.Sprintf("looked in %d place(s) by %s", res.Looked, p.Describe)
	if res.Outcome == Unreadable {
		return fmt.Sprintf("%s, and %d could not be read, so %q may still be there",
			where, res.Looked-res.Read, what.Description())
	}
	return fmt.Sprintf("%s and %s", where, notThere(what))
}

func notThere(what conditions.WorldCondition) string {
	return fmt.Sprintf("did not find %s", what.Description())
}
