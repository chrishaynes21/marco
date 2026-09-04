package main

import (
	"sort"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// THE RECOGNITION TRACE: what happened to every reading, in codes rather than in hindsight.
//
// # Why this exists
//
// A clean dogfood produced `Home` and one unnamed Settings page out of a four-screen walk.
// Bluetooth, Mouse and System were not in the store — and the store is the only thing anybody
// could read, so every explanation for their absence was reconstructed from what survived.
// "Settlement is too slow" was the most plausible reconstruction. It was still a reconstruction.
//
// The question this answers is the one the store cannot: for each reading taken while somebody was
// moving, was there a world, was there a candidate, and if no Place came of it, WHICH gate refused
// it — in the vocabulary `observe.PlacesToEstablish` already speaks, recorded at the moment of the
// decision rather than inferred afterwards.
//
// # It decides nothing
//
// Nothing reads this to admit, refuse, promote or name anything. It is written by `record` after
// the decisions have been made, and read by a status renderer. A diagnostic that fed back into the
// thing it measures would be a policy nobody reviewed.
//
// # And it holds no text off the screen
//
// A state id, a subject id, a reason code and counts. The one word it carries is a candidate's
// SETTLED NAME, which is already an admitted, durable-memory-bound value — and it is here because
// "the screen was recognised and had no name yet" is one of the answers this has to give.

// maxRecognitionSteps bounds the trace. A ring, because the interesting part of a navigation burst
// is the end of it, and because a diagnostic that grows with how long somebody watched is what
// this subsystem refuses everywhere else.
const maxRecognitionSteps = 512

// maxRecognitionScreens bounds the report. A walk is a handful of screens; a session that drifted
// across forty is not what this is for.
const maxRecognitionScreens = 24

// recognitionStep is one reading and what became of it.
type recognitionStep struct {
	at          time.Time
	application string
	state       observe.ScreenStateID
	// readable says perception got past the window frame.
	readable bool
	// place is the durable subject this reading resolved to, empty when none.
	place string
	// candidate says a shape was offered — this screen COULD have become a Place — and
	// candidateName is the word it had settled on, empty when none had.
	candidate     bool
	candidateName string
	// refusal is why the current screen did not become a Place. Empty when none was refused.
	refusal observe.PlaceRefusal
	// readings, agreeing and episodes are the settlement arithmetic: how many inferences
	// landed in this state, how many agreed on ONE whole composition, and how many separate
	// visits it has had. `agreeing` is the number the threshold is actually applied to, and
	// the gap between it and `readings` is what a fast walk opens up.
	readings, agreeing, distinct, episodes int
	settled                                bool
	// sameRoles, worstDrift and drifted say HOW the distinct compositions differ.
	sameRoles  bool
	worstDrift int
	drifted    []string
	// nameSightings, nameRunnerUp and coherent are the naming tally and which settlement
	// path the state took.
	nameSightings, nameRunnerUp int
	coherent                    bool
	// nameSettled says the STATE has a word that passed its recurrence rule, which is a
	// different question from whether a candidate shape carries one.
	nameSettled bool
	// sinceInput is how long before this reading the session's most recent human input was
	// banked, so a destination lost because the NEXT press arrived first is visible as such.
	// Negative when no input has been seen at all.
	sinceInput time.Duration
	// established says this reading is the one that made the screen durable.
	established bool
}

// recognitionTrace is the bounded ring.
type recognitionTrace struct {
	mu    sync.Mutex
	steps []recognitionStep
}

func (t *recognitionTrace) add(s recognitionStep) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.steps = append(t.steps, s)
	if len(t.steps) > maxRecognitionSteps {
		t.steps = t.steps[len(t.steps)-maxRecognitionSteps:]
	}
}

func (t *recognitionTrace) all() []recognitionStep {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]recognitionStep{}, t.steps...)
}

func (t *recognitionTrace) reset() {
	t.mu.Lock()
	t.steps = nil
	t.mu.Unlock()
}

// recognitionReport folds the trace into one entry per screen.
//
// Per SCREEN rather than per reading, because the question somebody has is about a screen — "why
// is Bluetooth not here" — and forty chronological lines make them do the grouping themselves.
// The counts inside each entry are the chronology, compressed.
func recognitionReport(steps []recognitionStep) []service.RecognitionScreen {
	order := []string{}
	byState := map[string][]recognitionStep{}
	for _, s := range steps {
		key := s.application + "\x00" + string(s.state)
		if _, seen := byState[key]; !seen {
			order = append(order, key)
		}
		byState[key] = append(byState[key], s)
	}
	out := make([]service.RecognitionScreen, 0, len(order))
	for _, key := range order {
		group := byState[key]
		last := group[len(group)-1]
		screen := service.RecognitionScreen{
			Application: last.application, State: string(last.state),
			Readings: last.readings, Traced: len(group),
			Named: last.nameSettled,
			Place: last.place, Settled: last.settled,
			Agreeing: last.agreeing, Distinct: last.distinct, Visits: last.episodes,
			SameRoles: last.sameRoles, WorstDrift: last.worstDrift,
			NameSightings: last.nameSightings, NameRunnerUp: last.nameRunnerUp,
			Coherent:     last.coherent,
			Drifted:      last.drifted,
			SinceInputMS: last.sinceInput.Milliseconds(),
			VisibleMS:    last.at.Sub(group[0].at).Milliseconds(),
		}
		if last.sinceInput < 0 {
			screen.SinceInputMS = -1
		}
		for _, s := range group {
			if s.established {
				screen.Established = true
			}
		}
		// WHY NOT, counted. One reason per reading, in the order the gates are applied,
		// so a screen refused for two different reasons over its life says so.
		reasons := map[string]int{}
		for _, s := range group {
			switch {
			case !s.readable:
				reasons[string(observe.PlaceRefusal("perception_degraded"))]++
			case s.refusal != "":
				reasons[string(s.refusal)]++
			case !s.candidate && s.place == "":
				reasons["no_candidate"]++
			}
		}
		keys := make([]string, 0, len(reasons))
		for k := range reasons {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			screen.Refusals = append(screen.Refusals,
				service.RecognitionRefusal{Reason: k, Count: reasons[k]})
		}
		out = append(out, screen)
		if len(out) >= maxRecognitionScreens {
			break
		}
	}
	return out
}

// traceReading records what became of one reading.
//
// Called from `record` after every decision it makes, so what is written is the OUTCOME rather
// than an intention. Deliberately tolerant of a reading that produced nothing: "Marco looked and
// could not read the window" is one of the answers this exists to give, and skipping those would
// leave the trace showing only the readings that went well.
func (a *ambientObserver) traceReading(application string, look ambientLook, now time.Time) {
	a.mu.Lock()
	// WHEN SOMEBODY LAST DID SOMETHING, from the session's own log growing. The high-water
	// mark rather than the length, because the log is bounded and drops from the front.
	if seen := look.Dropped + len(look.Inputs); seen > a.inputSeen {
		a.inputSeen, a.lastInput = seen, now
	}
	since := time.Duration(-1)
	if !a.lastInput.IsZero() {
		since = now.Sub(a.lastInput)
	}
	established := a.lastEstablished
	a.lastEstablished = ""
	a.mu.Unlock()

	step := recognitionStep{
		at: now, application: application, state: look.State,
		readable:      look.Place.Readable(),
		place:         look.Place.Subject,
		candidate:     look.Shape != nil,
		refusal:       look.Refusal,
		readings:      look.Readings,
		agreeing:      look.Agreeing,
		distinct:      look.Distinct,
		sameRoles:     look.SameRoles,
		nameSightings: look.NameSightings,
		nameRunnerUp:  look.NameRunnerUp,
		coherent:      look.Coherent,
		worstDrift:    look.WorstDrift,
		drifted:       look.Drifted,
		episodes:      look.Visits,
		settled:       look.Settled,
		sinceInput:    since,
	}
	if look.Shape != nil {
		step.candidateName = look.Shape.Called
	}
	// NOT compared against look.Place.Subject. The reading was taken BEFORE the
	// establishment it caused, so that field is still empty on the one reading where this is
	// true — which made `Established` a flag that could essentially never fire, and every
	// screen printed "no Place" while the store held it.
	step.established = established != ""
	a.trace.add(step)
}
