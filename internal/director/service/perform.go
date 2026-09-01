package service

import (
	"context"
	"fmt"
	"strings"
)

// Carrying out a learned outcome, as a COMMAND.
//
// # Why this is not just another read on the observation door
//
// PERFORM arrives on ObserveQuery beside Point, Reach, Learn and the rest, and every one of those
// is a read. Perform is the single field there that drives real input — and it was routed exactly
// like its read-only neighbours: straight into Runtime.Observation, with no registry entry, no
// cancellable context and no lifetime the service knew about.
//
// What that cost, measured:
//
//   - `director status` said nothing was running while a play was typing and clicking.
//   - `director stop` answered "nothing is running", because Registry.Cancel had no active
//     command to find — CANCEL_ACTIVE could not reach a performance at all.
//   - a second mutating request was accepted concurrently, so two things drove one desktop.
//   - rehearse.Live.Perform checks ctx.Err() before EVERY step and has a CancelledAttempt
//     terminal and a RefusalCancelled refusal ready. All of it was dead code: the only context
//     ever handed in was context.Background().
//
// So a performance enters the SAME registry an executed phrase does, at the same layer, for the
// same three reasons: it becomes visible, it becomes refusable, and it becomes stoppable. There is
// no second registry and no second lifecycle — Begin/Finish here is the one in server.go's
// execute, spelled once more for the one other request that can move the desktop.
//
// Deleting the Begin/Finish pair must fail TestStoppingAPerformanceReportsItAsCancelled and
// TestAPerformanceIsVisibleToStatusAndRefusesAConcurrentCommand.

// Performer is a Runtime that can carry out something Marco has learned.
//
// SEPARATE FROM Runtime on purpose, and asserted for rather than required. A Director that only
// observes is a legitimate Director — the CLI's own stub is one — and widening the Runtime
// interface would have made every implementer claim an ability to drive input in order to keep
// compiling. Asked for by name, a Director either offers it or honestly does not.
//
// The context is the point of the method. A performance that could not be handed one could not be
// stopped, whatever the registry knew about it.
type Performer interface {
	PerformGoal(ctx context.Context, q PerformQuery) (PerformView, error)
}

// performGoal runs one learned outcome under a registry command.
//
// The response shape is unchanged — always a PerformView on ResponsePerception — because both
// clients (`director perform` and `marco do`'s bridge) already read exactly that, and a busy or
// cancelled performance is something they must RENDER rather than something that breaks their
// decoding. The refusal vocabulary is where the difference lives.
func (s *Server) performGoal(requestID string, q PerformQuery, send func(ResponseEnvelope)) {
	performer, ok := s.runtime.(Performer)
	if !ok {
		send(NewResponse(requestID, ResponsePerception, PerformView{
			Goal:    q.Name,
			Refusal: "no_performer",
			Say:     "this Director cannot carry out learned outcomes",
		}))
		return
	}

	// The phrase a person reads in `director status` and in the busy message. The Audience's
	// own words, not the subject id: an id in a status line is unreadable, and the words are
	// what they asked with.
	phrase := strings.TrimSpace(q.Name)
	if phrase == "" {
		phrase = strings.TrimSpace(q.Subject)
	}

	cmd, ctx, err := s.registry.Begin(s.ctx, requestID, phrase)
	if err != nil {
		var busy *ErrBusy
		if asBusy(err, &busy) {
			// REFUSED, NOT QUEUED. Two things driving one desktop is the state the
			// mutating slot exists to prevent, and a performance that waited its turn
			// would start against a screen the other command had already changed.
			send(NewResponse(requestID, ResponsePerception, PerformView{
				Goal: q.Name, Application: q.Application, Refusal: "busy",
				Say: busyMessage(busy.Active),
			}))
			return
		}
		send(NewResponse(requestID, ResponseError, ErrorPayload{
			Code: "begin_failed", Message: err.Error(),
		}))
		return
	}

	view, err := performer.PerformGoal(ctx, q)
	view.Command = cmd.ID
	if err != nil {
		s.registry.Finish(cmd.ID, CommandFailed, len(view.Steps), err.Error())
		send(NewResponse(requestID, ResponseError,
			ErrorPayload{Code: "perform", Message: err.Error()}))
		return
	}
	s.registry.Finish(cmd.ID, performState(view), performedSteps(view), performReason(view))
	send(NewResponse(requestID, ResponsePerception, view))
}

// performState classifies a finished performance for the command record.
//
// CANCELLED IS ITS OWN STATE. "You stopped it" and "it failed" are different facts about the same
// half-finished walk, and a history that rendered them alike would tell somebody their play is
// broken when they are the one who stopped it.
//
// Arrival — not completion of the plan — is what counts as done, exactly as PerformGoal decides
// it: a walk whose every step verified and which ended somewhere else is `unverified`, not
// `completed`. See Runtime.confirmArrival.
func performState(v PerformView) CommandState {
	switch {
	case v.Refusal == string(performCancelled):
		return CommandCancelled
	case v.Arrived && v.Refusal == "":
		return CommandCompleted
	case v.Refusal == "":
		return CommandUnverified
	default:
		return CommandFailed
	}
}

// performCancelled is the one word for an Audience-ended performance, borrowed from the walker
// rather than minted here so the two cannot drift. rehearse.CancelledAttempt and
// rehearse.RefusalCancelled both render as it.
//
// Spelled as a literal because internal/director/service must not import the walker: the service
// is the door, and a door that knew the rehearsal vocabulary as types would be a second place that
// decides what a refusal means. cmd/director's TestTheCancelledWordIsTheWalkersWord holds the
// literal against the walker's constants; TestACancelledPerformanceIsRecordedAsCancelled holds
// this mapping.
const performCancelled = CommandState("cancelled")

// performedSteps counts what the desktop actually saw confirmed.
func performedSteps(v PerformView) int {
	n := 0
	for _, s := range v.Steps {
		if s.Verified {
			n++
		}
	}
	return n
}

// performReason is the one sentence the command record keeps.
func performReason(v PerformView) string {
	if say := strings.TrimSpace(v.Say); say != "" {
		return say
	}
	if v.Refusal != "" {
		return v.Refusal
	}
	return fmt.Sprintf("performed %q", v.Goal)
}

// Tester is a Runtime that can try one connection it learned by watching.
//
// Its own interface beside Performer, asserted for rather than required, for the reason Performer
// is: a Director that only observes is a legitimate Director, and widening one interface would
// make every implementer claim an ability it does not have.
type Tester interface {
	TestEdge(ctx context.Context, q TestQuery) (PerformView, error)
}

// testEdge runs one experiment under a registry command.
//
// # The same door a performance takes, deliberately
//
// An experiment drives real input, so every reason a performance enters the registry applies
// unchanged: it becomes visible to `director status`, it refuses a concurrent mutating request,
// and it becomes stoppable. What differs is only what the person asked for — "check what you
// learned" rather than "go there" — and that difference belongs in the request and in the words,
// not in a second lifecycle.
//
// The response is a PerformView because an experiment's report IS a performance's report, plus
// what became of the desktop. One shape for a client to render; `Testing` says which act it was.
func (s *Server) testEdge(requestID string, q TestQuery, send func(ResponseEnvelope)) {
	tester, ok := s.runtime.(Tester)
	if !ok {
		send(NewResponse(requestID, ResponsePerception, PerformView{
			Refusal: "no_performer",
			Say:     "this Director cannot try what it has learned",
		}))
		return
	}
	phrase := "checking what I learned"
	cmd, ctx, err := s.registry.Begin(s.ctx, requestID, phrase)
	if err != nil {
		var busy *ErrBusy
		if asBusy(err, &busy) {
			send(NewResponse(requestID, ResponsePerception, PerformView{
				Application: q.Application, Refusal: "busy",
				Say: busyMessage(busy.Active),
			}))
			return
		}
		send(NewResponse(requestID, ResponseError, ErrorPayload{
			Code: "begin_failed", Message: err.Error(),
		}))
		return
	}
	view, err := tester.TestEdge(ctx, q)
	view.Command = cmd.ID
	if err != nil {
		s.registry.Finish(cmd.ID, CommandFailed, len(view.Steps), err.Error())
		send(NewResponse(requestID, ResponseError,
			ErrorPayload{Code: "test", Message: err.Error()}))
		return
	}
	s.registry.Finish(cmd.ID, performState(view), performedSteps(view), performReason(view))
	send(NewResponse(requestID, ResponsePerception, view))
}
