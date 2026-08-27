package ambient

import "time"

// Choosing which part of the recent past somebody meant.
//
// # The question this answers, and the one it refuses to
//
// "Learn what I just did" names an outcome and points at the past. It does not say where the
// demonstration started, how far back it goes, or which of two things somebody did in the last
// minute they meant. Something has to decide, and the decision has to be honest about the cases
// where the evidence does not support one.
//
// So this is a PURE function over the recent trail. It reads a snapshot, applies boundaries and
// sufficiency rules, and returns one of four outcomes. It writes nothing, establishes nothing,
// and cannot: the licences that make evidence durable live in the promotion step, one layer up,
// and there is nothing reachable from here that could reach them.
//
// # Why not simply "the last N moves"
//
// Because the trail is a bounded tail and not a demonstration. Sixty-four steps is however much
// of the afternoon happened to fit, and reading it as one intention would learn a walk to lunch.
// What separates a demonstration from the trail around it is SEMANTIC boundaries — the person
// changed application, Marco did something itself, a stretch of time went by, an earlier Learn
// already took this — and those are what the backward walk below stops at.
//
// # It minimises questions and does not abolish honesty
//
// The clean case answers with no questions at all: one application, one coherent walk, every
// endpoint recognisable, every action nameable. Where the evidence genuinely supports two
// readings the answer is [Ambiguous] and somebody is asked, once. It is never an arbitrary pick
// from history.

// RecentGap is how long a pause has to be before what came after it is a different intention.
//
// # Why there is a time boundary here at all, when the buffer has none
//
// The buffer bounds by SIZE and not by time on purpose: an application that takes forty seconds
// to open a page must still be learnable, and an expiry short enough to keep the buffer tidy is
// short enough to make slow software impossible to show Marco. That reasoning is about RETENTION.
//
// This is about INTENTION, which is a different question. Somebody who did something, went and
// made a coffee, came back and did something else did two things — and the second one is what
// they mean by "what I just did". Five minutes is generous against every load screen anybody has
// measured here and short against a coffee.
//
// It is SECONDARY to the size bound and never overrides it: nothing is forgotten because of this
// constant, only excluded from one selection. The evidence stays, and a later demonstration that
// crosses the same stretch reads it exactly as before.
const RecentGap = 5 * time.Minute

// Outcome is what selection concluded, from a closed vocabulary.
type Outcome string

const (
	// Selected is one coherent demonstration, sufficient to promote.
	Selected Outcome = "selected"
	// Nothing is no recent evidence at all in the application asked about. Not a failure:
	// it is what watching an idle desktop honestly produces.
	Nothing Outcome = "nothing_recent"
	// Insufficient is evidence that exists and does not say enough. The shortfall says
	// which, because "I could not see what you clicked" and "I do not recognise the screen
	// you ended on" send somebody to two different places.
	Insufficient Outcome = "insufficient"
	// Ambiguous is two readings the evidence supports equally. A question, never a guess.
	Ambiguous Outcome = "ambiguous"
	// NotYours is a recent trail that is Marco's own performance rather than the person's.
	//
	// Its own outcome rather than Nothing, because the honest sentence is different: there
	// IS something recent and it is not something you did, and telling somebody there is
	// nothing would be telling them their demonstration was not seen.
	NotYours Outcome = "not_yours"
)

// Shortfall says what an insufficient demonstration was missing, from a closed vocabulary.
type Shortfall string

const (
	// ShortNoSteps is a run that was cut to nothing by the boundaries.
	ShortNoSteps Shortfall = "no_steps"
	// ShortUnknownPlace is an endpoint Marco can neither recognise nor describe.
	ShortUnknownPlace Shortfall = "unrecognised_screen"
	// ShortNoAction is a screen change with nothing attributed to the person.
	//
	// The ordinary honest cause is that the change had no cause anybody pressed — a timer,
	// a notification, a page finishing loading on its own.
	ShortNoAction Shortfall = "no_action_observed"
	// ShortUnnamedTarget is an action Marco saw land on a control it could not name.
	//
	// The commonest real shortfall on a desktop, and the one whose sentence matters most: a
	// press on a list item resolves to a role and no name under passive watching, which is
	// the privacy boundary working rather than perception failing.
	ShortUnnamedTarget Shortfall = "control_not_named"
)

// There is deliberately no "crossed applications" shortfall.
//
// A walk that leaves one program does not FAIL — it ends. The backward walk stops at the boundary
// and what is inside it is a perfectly good demonstration, so a refusal word for the case would
// name a state selection never reaches. This file had one for a while, and it was dead: a
// vocabulary entry nothing can produce is a reader being told about a decision that is not made
// anywhere, which is the shape of the unreachable discriminator this project has now met twice.

// Request is what a retrospective Learn asks of the trail.
type Request struct {
	// Application narrows the search. Empty means whichever program the trail ends in,
	// which is the ordinary case: somebody says "learn what I just did" while still looking
	// at where they did it.
	Application string
}

// There is deliberately no clock on a Request.
//
// Selection reads only the trail's own timestamps, and every boundary it applies is a gap BETWEEN
// two things somebody did. Nothing here compares anything to the present, and the reason is a
// product decision rather than a simplification: somebody who demonstrated something, was
// interrupted for twenty minutes, came back and said "learn what I just did" means the thing they
// demonstrated. It is still the last thing they did. A rule that measured staleness from now
// would refuse them, and the honest bound on how old this evidence can be is already the one
// watching has — the buffer is transient, and stopping forgets all of it.

// Demonstration is one selected walk, ready to be promoted or refused.
type Demonstration struct {
	Application string
	// Steps are the legs, in the order they were walked.
	Steps []Step
	// From and To are the ends of the walk.
	From, To string
	// Through is the highest Order in the walk, which is what the watermark is set to when
	// it is promoted.
	Through int
}

// Result is everything selection concluded, so a caller can explain itself.
type Result struct {
	Outcome Outcome
	// Demonstration is set only for Selected.
	Demonstration Demonstration
	// Why is set only for Insufficient.
	Why Shortfall
	// Alternative is the other reading, set only for Ambiguous. It exists so the question a
	// person is asked can name both rather than say "that was unclear".
	Alternative Demonstration
	// Considered is how many steps the backward walk looked at before the boundaries cut it.
	// A diagnostic: "nothing recent" over a trail of forty steps means the boundaries were
	// the reason, not the watching.
	Considered int
}

// SelectDemonstration chooses the one thing somebody meant by "what I just did".
//
// # The algorithm, and why every part of it is a refusal to guess
//
//  1. Walk BACKWARDS from the most recent step. What somebody just did ends at the present; a
//     forward walk would have to know where to start, which is the thing being decided.
//
//  2. Stop at the first semantic boundary — a different program, a step Marco performed itself, a
//     step an earlier Learn already promoted, a pause longer than [RecentGap], or a screen
//     already in the run. The last of those is what keeps a walk SIMPLE: coming back to where you
//     started ends one journey and begins another, and a run that contained both would be two
//     intentions read as one.
//
//  3. Refuse rather than weaken. Every endpoint must be recognisable or describable, every step
//     must have an action attributed to the person, and every action must say what it landed on.
//     There is no score here and nothing that can be nearly enough: the alternative to refusing is
//     a play that does the wrong thing on a screen somebody was not watching.
//
//  4. Ask when two readings survive. If the destination was reached more than once in the recent
//     window, by different routes, "what I just did" genuinely has two answers.
//
// Deterministic in its arguments, including the clock. Nothing here reads time, memory, the
// store, or anything else that could differ between two calls with the same view.
func SelectDemonstration(v View, req Request) Result {
	application := req.Application
	if application == "" {
		application = trailingApplication(v.Recent)
	}
	if application == "" {
		return Result{Outcome: Nothing}
	}

	// NAMING AN APPLICATION MEANS "the last thing I did IN THAT PROGRAM".
	//
	// Somebody who demonstrated something in Settings, switched to their mail for a minute
	// and then said `learn "open mouse settings" --recent` has not stopped meaning the
	// Settings walk. Starting the backward walk at the very end of the trail would meet the
	// mail step first, cut on the application boundary, and answer "I haven't seen you go
	// anywhere" — which is both wrong and the kind of wrong that reads as broken perception.
	//
	// So the walk starts at the last step in the named program. Once inside it, a step in
	// another program still ENDS the demonstration: what is skipped is the tail after it, not
	// a gap in the middle, so an interleaved walk is never spliced together.
	trail := v.Recent
	if req.Application != "" {
		trail = trailingIn(trail, application)
	}
	run, considered, cut := recentRun(View{Recent: trail, Consumed: v.Consumed}, application)
	out := Result{Considered: considered}
	switch {
	case len(run) == 0 && cut == cutMarco:
		// SOMETHING IS THERE AND IT IS NOT YOURS. Checked before the empty case,
		// because the two produce different sentences and only one of them is true.
		out.Outcome = NotYours
		return out
	case len(run) == 0:
		out.Outcome = Nothing
		return out
	}

	d := Demonstration{Application: application, Steps: run,
		From: run[0].From, To: run[len(run)-1].To, Through: run[len(run)-1].Order}
	if why, ok := insufficient(run); !ok {
		out.Outcome, out.Why = Insufficient, why
		return out
	}
	if alt, ambiguous := otherWayTo(v, application, d); ambiguous {
		out.Outcome, out.Demonstration, out.Alternative = Ambiguous, d, alt
		return out
	}
	out.Outcome, out.Demonstration = Selected, d
	return out
}

// cut says which boundary ended the backward walk, for the sentences that depend on it.
type cut int

const (
	cutStart cut = iota
	cutApplication
	cutMarco
	cutPromoted
	cutGap
	cutRevisit
)

// recentRun is the backward walk and the boundaries that stop it.
//
// Returns the run in FORWARD order, how many steps were considered, and which boundary ended it.
func recentRun(v View, application string) ([]Step, int, cut) {
	var reversed []Step
	// visited is every screen the run passes through, so the walk stays SIMPLE. See the
	// revisit boundary below.
	visited := map[string]bool{}
	considered := 0
	ending := cutStart
	last := time.Time{}

	for i := len(v.Recent) - 1; i >= 0; i-- {
		s := v.Recent[i]
		considered++
		if !sameApplication(s.Application, application) {
			// A DIFFERENT PROGRAM ends the demonstration rather than being folded
			// into it. Plays are scoped to one application, and a walk that crossed
			// two would be learned as a thing Marco cannot represent. See ADR-094.
			ending = cutApplication
			break
		}
		if s.By != ByHuman {
			// MARCO'S OWN WORK IS NOT YOUR DEMONSTRATION. Without this the play that
			// ran while watching was on would be offered back as something the person
			// had just shown Marco, which is how a system learns its own behaviour
			// from itself.
			//
			// Deleting this must fail TestWhatMarcoDidIsNotWhatYouDemonstrated.
			ending = cutMarco
			break
		}
		if s.Order <= v.Consumed {
			// ALREADY LEARNED. See Buffer.Promoted for why this is a watermark
			// rather than a deletion.
			ending = cutPromoted
			break
		}
		if !last.IsZero() && last.Sub(s.At) > RecentGap {
			ending = cutGap
			break
		}
		if visited[s.From] {
			// THE WALK STAYS SIMPLE: no screen twice.
			//
			// What this cuts is a round trip. Home to Bluetooth, back to Home, then to
			// Display: taking all three would learn a walk that visits Home twice and
			// ends somewhere it has already been through, which is two intentions
			// spliced into one. What is kept is the longest recent stretch that never
			// goes anywhere twice — deterministic, explainable in a sentence, and never
			// a pick from the middle of history.
			//
			// The check is on the step's SOURCE, not on both ends. In a contiguous
			// backward walk the destination is always the junction with what has
			// already been taken, so checking it would cut every walk at its first
			// step.
			//
			// Deleting this must fail TestARoundTripIsNotOneDemonstration.
			ending = cutRevisit
			break
		}
		if len(reversed) == 0 {
			visited[s.To] = true
		}
		visited[s.From] = true
		reversed = append(reversed, s)
		last = s.At
	}

	// Reversed in place. The trail is at most MaxMoves long, so this is a few dozen swaps.
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	return reversed, considered, ending
}

// insufficient is the sufficiency rule, and it is the same one for every step.
//
// # Why there is no confidence score
//
// Because a score exists to avoid saying no. Every condition below is binary and every one of
// them is something a future walk genuinely needs: an endpoint it can recognise on arrival, an
// action attributed to the person rather than to the passage of time, and a name for whatever
// that action landed on. A demonstration missing any of them cannot be reproduced safely, and
// dressing that up as 0.7 would produce a play that fails in front of somebody.
func insufficient(run []Step) (Shortfall, bool) {
	if len(run) == 0 {
		return ShortNoSteps, false
	}
	for _, s := range run {
		if !s.Placed() {
			return ShortUnknownPlace, false
		}
		if len(s.Did) == 0 {
			return ShortNoAction, false
		}
		for _, a := range s.Did {
			if !a.Representable() {
				return ShortUnnamedTarget, false
			}
		}
	}
	return "", true
}

// otherWayTo asks whether the recent past reached the same destination another way.
//
// # The case this exists for
//
// Somebody opens the Mouse page from Home through Bluetooth. A minute later they open it again
// from Home through Devices. Both are recent, both are theirs, both end where they are standing,
// and "learn what I just did" has two answers. Selecting the more recent one silently would be
// choosing arbitrary history — and it would be WRONG about half the time, because the one
// somebody means is the one they consider the right way, not the one they did last.
//
// So it is a question. Scanned over the same window the run was cut from, so a route walked an
// hour ago does not make today ambiguous.
//
// Deleting this must fail TestTwoWaysToTheSamePlaceIsAQuestion.
func otherWayTo(v View, application string, chosen Demonstration) (Demonstration, bool) {

	inChosen := map[int]bool{}
	for _, s := range chosen.Steps {
		inChosen[s.Order] = true
	}
	// The window is the CHOSEN RUN's own stretch of activity, not the present. Something
	// walked an hour before this demonstration is not a competing reading of it, and
	// measuring from now would make the same past ambiguous or not depending on how long
	// somebody took to type the command.
	var from time.Time
	if len(chosen.Steps) > 0 {
		from = chosen.Steps[0].At
	}
	var alt []Step
	// Backwards again, over everything the chosen run did not take.
	for i := len(v.Recent) - 1; i >= 0; i-- {
		s := v.Recent[i]
		if inChosen[s.Order] || s.Order <= v.Consumed {
			continue
		}
		if !sameApplication(s.Application, application) || s.By != ByHuman {
			continue
		}
		if !from.IsZero() && !s.At.IsZero() && from.Sub(s.At) > RecentGap {
			continue
		}
		if s.To != chosen.To {
			continue
		}
		// A step arriving at the same destination, by a different route. Its own leg is
		// enough to say the past is ambiguous; the alternative is reported so the
		// question can name it.
		if s.From == stepFrom(chosen.Steps, s.To) {
			continue
		}
		alt = []Step{s}
		break
	}
	if len(alt) == 0 {
		return Demonstration{}, false
	}
	return Demonstration{Application: application, Steps: alt,
		From: alt[0].From, To: alt[0].To, Through: alt[0].Order}, true
}

// stepFrom is where the chosen walk came from when it arrived at to, empty if it never did.
func stepFrom(steps []Step, to string) string {
	for _, s := range steps {
		if s.To == to {
			return s.From
		}
	}
	return ""
}

// trailingIn cuts the trail at the last step belonging to one program.
//
// Everything after it is a different program's, and the caller named this one. See the note at
// the call site for why the tail is dropped rather than the foreign steps being skipped over.
func trailingIn(recent []Step, application string) []Step {
	for i := len(recent) - 1; i >= 0; i-- {
		if sameApplication(recent[i].Application, application) {
			return recent[:i+1]
		}
	}
	return nil
}

// trailingApplication is whichever program the trail ends in.
func trailingApplication(recent []Step) string {
	for i := len(recent) - 1; i >= 0; i-- {
		if recent[i].Application != "" {
			return recent[i].Application
		}
	}
	return ""
}

// sameApplication compares program names the way every other surface here does.
func sameApplication(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return a != ""
}
