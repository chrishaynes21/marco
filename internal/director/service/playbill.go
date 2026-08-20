package service

import (
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// THE publication site.
//
// # Why the server finishes the playbill and the runtime starts it
//
// Because they know different halves and neither can see the other's. The Runtime owns
// perception, the observation session, memory and the learning lifecycle — everything
// under CURRENT, SEEING, THINKING and LEARNING. The Server owns command lifecycle,
// confirmations and clarifications — everything under DOING, and two of the three kinds
// of pending question.
//
// The alternative was to give the Runtime a back-reference to the command registry. That
// would let the layer that observes the desktop see the layer that drives it, which is
// the coupling ADR-010 exists to prevent, for the sake of one status field.
//
// # Deleting the runtime call must break a test
//
// This function is the ONLY path from the Director to a presentation. If the
// `s.runtime.Playbill` line below is removed, every surface still renders — showing an
// idle, unobserving, believing-nothing Director, forever, with no error anywhere. That
// is the exact failure mode the wiring-test rule in this repository was written for, so
// there is a mutation test that removes it: see TestRemovingTheRuntimeCallEmptiesTheWatchSurface.
//
// # It grants nothing and can grant nothing
//
// Everything below is a read of state that already existed. There is no branch here that
// starts, authorises, confirms, cancels or performs, and there is no argument a caller
// could pass that would produce one.

// playbillFor assembles the account and admits it.
func (s *Server) playbillFor(p PlaybillPayload) PlaybillResponse {
	started := time.Now()

	// THE runtime call. Perception, observation, recall and the learning lifecycle.
	v := s.runtime.Playbill(p)

	v.Version = playbill.Version
	v.Reach = playbill.Present
	v.TakenAt = started
	v.UptimeMS = time.Since(s.startedAt).Milliseconds()

	// THE command half. The server's own state, never asked of the runtime.
	v.Doing = s.doing()
	if q := s.pendingQuestion(); q != nil {
		// A confirmation or clarification OUTRANKS a passive observation question.
		// Both block something the person asked for; a proposal is Marco being curious
		// while they play, and interrupting the first with the second would be Marco
		// talking over itself.
		v.Question = q
	}
	if v.Why == "" {
		v.Why = v.Doing.Because
	}

	if p.Diagnostics && v.Diagnostics != nil {
		v.Diagnostics.ComposeMS = time.Since(started).Milliseconds()
	}
	// Normalise, then bound, then admit. Normalising first so an honest partial account
	// is not refused for the sections it had nothing to say about; Admitted last and
	// never Admit-and-hope, because a playbill that fails its own privacy check is
	// REPLACED by one that says so rather than shipped with a logged warning.
	return PlaybillResponse{View: v.Normalise().Bound().WithDigest().Admitted()}
}

// doing is the command half of the account.
//
// Derived from the registry the executor already writes to. Nothing here is a second
// record of what is running, because a second record is a record that can disagree.
func (s *Server) doing() playbill.Doing {
	if active, ok := s.registry.Active(); ok {
		out := playbill.Doing{
			Phase: playbill.Performing, What: active.Phrase, Live: true,
			Step: active.Iteration, Steps: active.Total,
			RunningMS: time.Since(active.StartedAt).Milliseconds(),
		}
		if s.runtime.Confirmations() != nil {
			if _, waiting := s.runtime.Confirmations().Pending(); waiting {
				// Blocked on a person. Distinct from running, and the distinction is
				// the whole reason a person looks at this surface during a long command.
				out.Phase = playbill.AwaitingPermission
			}
		}
		return out
	}

	recent := s.registry.Recent(1)
	if len(recent) == 0 {
		return playbill.Doing{Phase: playbill.NotDoing}
	}
	last := recent[0]
	// Only the last few seconds. A terminal state shown forever would have somebody
	// reading yesterday's failure as today's, and "idle" is the honest answer once the
	// moment has passed.
	if last.CompletedAt != nil && time.Since(*last.CompletedAt) > lastOutcomeWindow {
		return playbill.Doing{Phase: playbill.NotDoing}
	}
	return playbill.Doing{
		Phase: phaseOf(last.State), What: last.Phrase,
		Because: oneLine(last.Reason), Live: true,
		RunningMS: last.Duration().Milliseconds(),
	}
}

// lastOutcomeWindow is how long a finished command stays on the surface.
//
// Matched to how long somebody takes to glance up after speaking, not to how long the
// information stays true. It stays true forever; it stops being what is HAPPENING almost
// immediately.
const lastOutcomeWindow = 12 * time.Second

// phaseOf maps a command state onto the presentation's phase vocabulary.
//
// A mapping and not a rename: `timed_out` becomes `unverified` because a timeout is the
// absence of confirmation rather than evidence that nothing happened, and telling a
// person "that didn't work" when it may well have is the more dangerous of the two
// mistakes. `internal_error` becomes `failed` and keeps its reason.
func phaseOf(s CommandState) playbill.Phase {
	switch s {
	case CommandCompleted:
		return playbill.Succeeded
	case CommandUnverified, CommandTimedOut:
		return playbill.Unverified
	case CommandBlocked:
		return playbill.Refused
	case CommandCancelled:
		return playbill.Cancelled
	case CommandFailed, CommandInternalError:
		return playbill.Failed
	case CommandRunning, CommandPending:
		return playbill.Performing
	}
	return playbill.NotDoing
}

// pendingQuestion is the confirmation or clarification waiting on a person.
//
// Both route through paths that already exist — CONFIRM and CLARIFY — and the Via field
// says which. A presentation that renders this calls the ordinary request; it does not
// get a shortcut, and there is nothing here that would let it build one.
func (s *Server) pendingQuestion() *playbill.Question {
	if b := s.runtime.Confirmations(); b != nil {
		if c, ok := b.Pending(); ok {
			return &playbill.Question{
				ID:    c.ID,
				Asks:  confirmationSentence(c),
				Wants: playbill.WantsChoice,
				// The broker's own vocabulary. A presentation offers these two and
				// nothing else, and neither of them is a default.
				Answers: []string{"yes", "no"},
				Via:     playbill.ViaConfirm,
			}
		}
	}
	if c, ok := s.pending.Get(); ok {
		q := &playbill.Question{
			ID:    string(c.CommandID),
			Asks:  oneLine(c.Question),
			Wants: playbill.WantsChoice,
			Via:   playbill.ViaClarify,
		}
		for i := range c.Candidates {
			// The candidate's ORDINAL, not its label. A clarification candidate is a
			// control on somebody's screen and its label is arbitrary observed content,
			// which this representation has no admission for — so a surface renders
			// "the first one / the second one" and the ordinary CLARIFY request carries
			// the choice. The person is looking at their own screen and can see which.
			q.Answers = append(q.Answers, "option-"+ordinal(i+1))
		}
		return q
	}
	return nil
}

// confirmationSentence renders the broker's question in the presentation's register.
//
// Built from the CLOSED fields — scope, action, effect — rather than from the target's
// label, which is observed content. The result is deliberately less specific than the
// terminal prompt: a person answering in the overlay is looking at their own screen and
// already knows what is on it.
func confirmationSentence(c ConfirmationPayload) string {
	parts := []string{"May I"}
	switch {
	case c.Action != "":
		parts = append(parts, c.Action)
	case c.Effect != "":
		parts = append(parts, c.Effect)
	default:
		parts = append(parts, "go ahead")
	}
	if c.Reason != "" {
		parts = append(parts, "— "+c.Reason)
	}
	return oneLine(strings.Join(parts, " ") + "?")
}

// oneLine flattens a multi-line reason to its first line.
//
// The Director's failure reasons are sometimes several lines with a remedy under them,
// which is right for a terminal and wrong for a field the admission guard refuses on
// sight. Taking the first line rather than joining them keeps the sentence a sentence.
func oneLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if n := []rune(s); len(n) > playbill.MaxSentence {
		s = string(n[:playbill.MaxSentence-1]) + "…"
	}
	return s
}

// ordinal renders a small position as a closed-vocabulary word.
//
// Bounded on purpose: a clarification with more candidates than this is a question a
// person cannot answer by looking anyway, and the ordinary CLARIFY path still carries
// every one of them.
func ordinal(n int) string {
	if n < 1 || n > 9 {
		return "other"
	}
	return string(rune('0' + n))
}
