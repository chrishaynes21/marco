package service

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
)

// Serving the demonstration commands.
//
// The service TRANSPORTS what the demo package concluded and has no opinion about it: it
// does not summarise a proposal, re-score an extraction, or decide whether something is
// worth learning. The same rule the perception and explanation requests follow, for the
// same reason — a second opinion between the caller and the thing that decided is how two
// answers to one question come about.
//
// The one judgement it does make is the routing, and an unknown action is an ERROR rather
// than a default. The available defaults are "do nothing", which looks like the recorder
// ignoring the user, and "start recording", which is worse.

// demonstration answers one DEMONSTRATION request.
func (s *Server) demonstration(p DemonstrationPayload) (DemonstrationResponse, error) {
	switch p.Action {
	case DemoStart:
		d, err := s.runtime.StartDemonstration()
		if err != nil {
			return DemonstrationResponse{}, err
		}
		return DemonstrationResponse{
			Recording: d,
			Message: "recording. Perform the task once, then stop — every verified step " +
				"is kept as semantics, and nothing about where you clicked.",
		}, nil

	case DemoStop:
		d, err := s.runtime.StopDemonstration()
		if err != nil {
			return DemonstrationResponse{}, err
		}
		return DemonstrationResponse{Demonstration: d, Message: stopMessage(d)}, nil

	case DemoAbandon:
		reason := p.Reason
		if reason == "" {
			reason = "the user abandoned it"
		}
		d, err := s.runtime.AbandonDemonstration(reason)
		if err != nil {
			return DemonstrationResponse{}, err
		}
		return DemonstrationResponse{Demonstration: d, Message: "discarded"}, nil

	case DemoActive:
		d := s.runtime.ActiveDemonstration()
		if d == nil {
			return DemonstrationResponse{Message: "nothing is being recorded"}, nil
		}
		return DemonstrationResponse{Recording: d,
			Message: fmt.Sprintf("recording %s: %d step(s) so far", d.ID, len(d.Steps))}, nil

	case DemoList:
		list, err := s.runtime.Demonstrations()
		if err != nil {
			return DemonstrationResponse{}, err
		}
		return DemonstrationResponse{
			Demonstrations: list, Recording: s.runtime.ActiveDemonstration(),
		}, nil

	case DemoShow:
		d, err := s.runtime.Demonstration(demo.ID(p.ID))
		if err != nil {
			return DemonstrationResponse{}, err
		}
		return DemonstrationResponse{Demonstration: d}, nil

	case DemoExtract, DemoProposal:
		out, err := s.runtime.ExtractProcedure(demo.ID(p.ID))
		if err != nil {
			return DemonstrationResponse{}, err
		}
		return DemonstrationResponse{Extraction: &out, Message: out.Refusal}, nil

	case DemoExplain:
		x, err := s.explainProcedure(p)
		if err != nil {
			return DemonstrationResponse{}, err
		}
		return DemonstrationResponse{Explanation: &x}, nil

	case DemoApprove:
		l, err := s.runtime.ApproveProcedure(demo.ID(p.ID), "the user")
		if err != nil {
			return DemonstrationResponse{}, err
		}
		return DemonstrationResponse{Learned: l, Message: fmt.Sprintf(
			"%q is installed and will serve %s from the next request",
			l.Name, l.Goal.Describe())}, nil

	case DemoForget:
		if err := s.runtime.ForgetProcedure(p.Name); err != nil {
			return DemonstrationResponse{}, err
		}
		return DemonstrationResponse{Message: fmt.Sprintf(
			"%q is forgotten. It stays available to the running service until it "+
				"restarts, because a procedure that vanished mid-session would make one "+
				"request behave differently from the next.", p.Name)}, nil

	case DemoLearned:
		return DemonstrationResponse{Procedures: s.runtime.LearnedProcedures()}, nil
	}
	return DemonstrationResponse{}, fmt.Errorf(
		"%q is not something that can be done with a demonstration", p.Action)
}

// explainProcedure answers "why does this procedure have this shape?".
//
// From a LEARNED procedure by name when it has been approved — the decisions stored with
// it are the ones the user agreed to — and from a fresh extraction otherwise. The
// distinction matters: re-extracting an approved procedure would explain what a newer
// extractor would produce today rather than what is installed.
func (s *Server) explainProcedure(p DemonstrationPayload) (demo.Explanation, error) {
	if p.Name != "" {
		for _, l := range s.runtime.LearnedProcedures() {
			if strings.EqualFold(l.Name, p.Name) {
				return demo.Explanation{
					Demonstration: l.Source,
					Procedure:     l.Name,
					Goal:          filterDecisions(l.Decisions, demo.VerdictRecovered),
					Parameters:    filterDecisions(l.Decisions, demo.VerdictParameter),
					Constants:     filterDecisions(l.Decisions, demo.VerdictConstant),
					Refusals:      filterDecisions(l.Decisions, demo.VerdictRefused),
				}, nil
			}
		}
		return demo.Explanation{}, fmt.Errorf("no learned procedure is called %q", p.Name)
	}
	out, err := s.runtime.ExtractProcedure(demo.ID(p.ID))
	if err != nil {
		return demo.Explanation{}, err
	}
	return demo.Explain(out), nil
}

// filterDecisions selects the decisions with one verdict, preserving order.
func filterDecisions(ds []demo.Decision, v demo.Verdict) []demo.Decision {
	var out []demo.Decision
	for _, d := range ds {
		if d.Verdict == v {
			out = append(out, d)
		}
	}
	return out
}

// stopMessage says what became of a finished session, in the terms the user needs.
//
// A refusal is reported HERE, at the moment the recording ends, rather than being
// discovered later when they ask for a procedure — by which time they have forgotten what
// was in it.
func stopMessage(d *demo.Demonstration) string {
	switch {
	case d == nil:
		return ""
	case d.Status == demo.Refused:
		return d.Refusal
	case len(d.Steps) == 0:
		return "nothing was recorded: no verified semantic action happened while it ran"
	}
	return fmt.Sprintf("recorded %d step(s). Review the procedure it suggests with: "+
		"director extract %s", len(d.Steps), d.ID)
}
