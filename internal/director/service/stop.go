package service

import "context"

// What the one word "stop" has to reach, and why the answers are not all the same.
//
// # Three things can be under way, and they are three different kinds of thing
//
//	a COMMAND     — an executed phrase, or a performance. Registry.Cancel cuts its context.
//	a QUESTION    — a clarification the Director is waiting on. Clearing it abandons the
//	                paused program with it.
//	an EPISODE    — somebody demonstrating, with Marco watching. Nothing is being done to the
//	                desktop; an attempt is being built. ADR-066's Cancel throws it away.
//
// Before this, `cancelActive` knew about the first two. A Learn episode rides RequestObservation
// with no command behind it, so a leader key, a spoken "stop" or `marco director stop` during a
// demonstration answered "nothing is running" — while the demonstration carried on, still
// capturing, still holding the observation slot, and still refusing the next Start with "a learn
// session is already running".
//
// # Why an episode is NOT put in the command registry
//
// It would be the obvious fix and it would be wrong twice over.
//
// The registry's slot means "this is driving the desktop, and a second one must be refused". An
// episode drives nothing — the PERSON is driving, and Marco is watching. Claiming the slot would
// make every ordinary command refuse itself as busy for the length of a demonstration, which is
// the opposite of what a demonstration is.
//
// And the registry cancels by cutting a context, which is not what abandoning an episode means.
// ADR-066 is explicit that Cancel and Finish are separate operations with separate fields,
// separate methods and separate flags, "because routing one to the other silently destroys a
// demonstration a person has just given — and it would look like it worked". A second
// implementation of Cancel, reached only from the stop path, is the exact hazard that sentence
// describes: two bodies that must agree forever about what "throw it away" means.
//
// So the third arm sends the SAME request the `--cancel` verb sends. Not a copy of what that
// request does; the request itself. There is one implementation of Cancel and this is not it.
//
// A live REHEARSAL inside an episode is the other half of the distinction and gets the other
// mechanism: it is a PERFORMANCE, it drives real input, and it enters the command registry beside
// PERFORM — see cmd/director/commandslot.go. An episode is a session and a rehearsal is a
// performance; they are not the same kind of thing and they must not get the same mechanism.

// CommandRegistrar is a Runtime with desktop work of its own that must enter the command registry.
//
// SEPARATE FROM Runtime and asserted for rather than required, exactly as Performer is: a Director
// that only observes is a legitimate Director, and widening the Runtime interface would make every
// implementer claim something it does not do in order to keep compiling.
//
// It receives the service's own lifetime as well as the registry, so the work it begins is ended by
// a shutdown as well as by a stop.
type CommandRegistrar interface {
	UseCommands(serviceCtx context.Context, reg *Registry)
}

// Acquirer is a Runtime that can be in the middle of a Learn episode.
//
// Only the question is asked here — whether one is under way. Ending it goes back through the
// ordinary acquisition request, so this interface cannot become a second door onto the lifecycle.
type Acquirer interface {
	// LearningNow reports whether somebody is demonstrating right now.
	LearningNow() bool
}

// cancelLearning abandons a Learn episode through the one request that abandons one.
//
// Reported as accepted only when there was something to abandon AND the request came back clean. A
// stop that says it worked and did not is worse than one that says nothing is running: the person
// walks away from a session that is still capturing.
//
// The word is CANCEL and never Finish. That is settled product, not an implementation choice: a
// bare "stop" is the abort word everywhere in Marco, so the two controls that were both labelled
// Stop and did opposite things — abandon on the command line, keep-everything in the control
// centre — can no longer disagree. Finish keeps its own affordance under an honest name.
func (s *Server) cancelLearning() (CancelPayload, bool) {
	a, ok := s.runtime.(Acquirer)
	if !ok || !a.LearningNow() {
		return CancelPayload{}, false
	}
	if _, err := s.runtime.Observation(ObserveQuery{Learn: &ObserveLearn{Cancel: true}}); err != nil {
		return CancelPayload{Accepted: false, Message: err.Error()}, true
	}
	return CancelPayload{
		Accepted: true,
		Message: "stopped learning — nothing from that demonstration was kept. " +
			"To keep what you showed me, finish it instead of stopping it.",
	}, true
}
