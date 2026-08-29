package observe

import "fmt"

// IS THE PRIMARY READING ENOUGH TO INTERPRET THIS INTERFACE?
//
// # What this adds, and what it deliberately does not
//
// The judgement itself already existed and is not moved: [ReachOfState] decides how far into a
// window an observation got, on arrangement rather than richness, and [Place] carries the
// answer. 37D did not replace it. Running it over the seven real desktop readings 37C captured
// — Settings at two widths, a second Settings Place, Explorer, a browser at two widths and
// Paint — classifies all seven as content reached, which is what they were. See
// TestTheCapturedDesktopIsSufficient.
//
// What was missing is a NAME for the three-way answer and a reason a person can read.
//
// The three states were already structurally distinct and were being reported as two booleans
// nobody had joined up:
//
//	nothing to describe        Place.Placed is false — the sensors did not report
//	the frame and no page      Place.Reach is ReachShell
//	the page                   Place.Reach is ReachContent
//
// Collapsing any pair of those is the mistake this repository has now made twice at two
// different levels: ADR-031 forbids sending a perception failure to the page, and [Reach]
// exists because "I could not read it" was being reported as "I do not recognise it".
//
// # What it is not allowed to consider
//
// Not element count. Not acquisition time — Explorer's tree takes 1.5 seconds to read and is
// the richest reading in the corpus. Not the application's name. Not whether memory recognises
// the Place: an unknown application with a healthy reading is SUFFICIENT and unknown, and
// those are different facts about different things. Not whether any optional sensor exists —
// this describes the primary reading, and it would describe it identically on a machine with
// no vision plugin installed.
//
// # It names no sensor
//
// [Incomplete] says the primary reading did not represent the interface. It does not say to
// run a detector, which detector, or on what budget. That decision needs to weigh cost against
// what is actually missing, and it belongs to whatever schedules additional perception — not
// to the thing that noticed. A classifier that returned `UseScreenParser` would have decided
// the architecture of every future sensor by accident.
type SufficiencyState string

const (
	// Sufficient is a reading that represents the interface well enough to interpret.
	//
	// It is not a claim that the reading is COMPLETE. Nothing here can know what the
	// application meant to show. It is the narrower and honest statement: nothing about
	// this reading says the interface went unread.
	Sufficient SufficiencyState = "sufficient"
	// Incomplete is the window, and space where its content should be with nothing in it.
	Incomplete SufficiencyState = "incomplete"
	// Unobservable is no usable reading at all — the sensors did not report, or reported
	// too little to describe a screen.
	//
	// Distinct from [Incomplete] on purpose. A provider that failed and a provider that
	// succeeded and returned a frame are different facts with different remedies, and the
	// second is the one that says something about the application.
	Unobservable SufficiencyState = "unobservable"
)

// SufficiencyReason is why, from a closed set.
//
// Bounded deliberately. An open evidence trace grows until nobody reads it, and the consumer
// this exists for has to branch on the answer rather than print it.
type SufficiencyReason string

const (
	// ReasonContentReached is the ordinary case: structures are spread through the space
	// the window gives its content.
	ReasonContentReached SufficiencyReason = "content_reached"
	// ReasonPopulatedPanel is a window with an empty region and somewhere else with things
	// in it — a mail client with nothing selected, an editor with no file open, Paint's
	// blank canvas beside its ribbon. Read perfectly well; one part happens to be empty.
	ReasonPopulatedPanel SufficiencyReason = "populated_panel"
	// ReasonTooLittleToJudge is an observation too thin to reason about arrangement. It is
	// a REFUSAL to judge, not a finding of health, and it resolves to Sufficient because
	// the caller's other checks still have to pass.
	ReasonTooLittleToJudge SufficiencyReason = "too_little_to_judge"
	// ReasonClientAreaUnpopulated is the live Settings failure: a large region covering the
	// space the content belongs in, with almost nothing observed inside it, and nowhere
	// else in the window with anything in it either.
	ReasonClientAreaUnpopulated SufficiencyReason = "client_area_unpopulated"
	// ReasonNothingObserved is no signature at all — no state settled, or no structures.
	ReasonNothingObserved SufficiencyReason = "nothing_observed"
)

// Sufficiency is the assessment, its reason, and the evidence behind it.
//
// This is the seam a later escalation policy consumes. It carries the vacancy so that policy
// can weigh HOW MUCH of the window is unaccounted for without re-deriving anything, and it
// carries no sensor name so that what to do about it stays an open question.
type Sufficiency struct {
	State  SufficiencyState
	Reason SufficiencyReason
	// Vacancy is the empty space, when emptiness is why. Zero otherwise.
	Vacancy Vacancy
}

// Enough reports whether the reading may be interpreted as a screen.
func (s Sufficiency) Enough() bool { return s.State == Sufficient }

// SufficiencyOf reads the assessment already made, and names it.
//
// It derives — it does not decide. Every input is a field [PlaceNow] has already filled from
// [ReachOfState], so there is exactly one place where a reading is judged and this is not it.
// A second opinion here would be a second thing to keep in step, which is the whole failure
// mode 37D was asked to avoid.
func SufficiencyOf(p Place) Sufficiency {
	if !p.Placed {
		return Sufficiency{State: Unobservable, Reason: ReasonNothingObserved}
	}
	if p.Reach == ReachShell {
		return Sufficiency{State: Incomplete,
			Reason: ReasonClientAreaUnpopulated, Vacancy: p.Vacancy}
	}
	reason := p.Reason
	if reason == "" {
		reason = ReasonContentReached
	}
	return Sufficiency{State: Sufficient, Reason: reason}
}

// Describe says what happened in words an owner can act on.
//
// Deliberately not a score. "0.42 below threshold 0.5" tells a person nothing they can do
// something about; "accessibility described the window but nothing inside it" tells them the
// application is not exposing its content, which is a fact about their machine.
func (s Sufficiency) Describe() string {
	switch s.Reason {
	case ReasonContentReached:
		return "accessibility described the window and the page in it"
	case ReasonPopulatedPanel:
		return "accessibility described the window; part of it is empty and the rest is not"
	case ReasonTooLittleToJudge:
		return "too little was observed to say how far the reading got"
	case ReasonClientAreaUnpopulated:
		if s.Vacancy.Found() {
			return fmt.Sprintf(
				"accessibility described the window but not the content: %.0f%% of it "+
					"came back as one region with %d of %d structures inside it",
				s.Vacancy.Share*100, s.Vacancy.Inside, s.Vacancy.Structures)
		}
		return "accessibility described the window but not the content in it"
	case ReasonNothingObserved:
		return "nothing was observed to describe this screen with"
	}
	return string(s.State)
}
