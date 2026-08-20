package observe

import "fmt"

// Asking the user what a screen is called, and binding the answer to the screen that was asked
// about rather than to whatever is in front of them when they get round to answering.
//
// # Why this question exists at all
//
// Because a verified play cannot say where it begins until somebody has said what "there" is
// called. `do Screen's Showing with "…"` is the play's first executable sentence and the string
// inside it is resolved against durable memory at run time — so it is not a label, it is the
// condition. There is no name Director may invent that would make that sentence true.
//
// # And why it is DEMAND-DRIVEN
//
// Marco does not name screens for the sake of a tidier memory. It asks when, and only when,
// something concrete is blocked on the answer: a rehearsed, verified procedure that is ready to
// become readable Marco and cannot, because `JudgeLowering` refused with `screen_unnamed`. A pass
// over memory looking for unnamed subjects would be a collection loop wearing a question's
// clothes.
//
// See [[ADR-031-the-user-names-the-stage]].

// SubjectRef names the DURABLE remembered subject a question is about.
//
// # Why a typed reference and not a string in the sentence
//
// Because the answer arrives late. By the time somebody types "the pause menu" the screen has
// changed, possibly several times, possibly into a different application; a handler that read
// "the current subject" would name the wrong thing and there would be nothing in the record to
// say so. This is the same reasoning `RelationshipRef` already carries for learning questions,
// and the same durable identity semantic memory uses across a restart.
//
// The application travels WITH the id. A name is scoped to one program — `settings` in one game
// is not `settings` in another — so an answer that arrived while a different application was in
// front must still land in the right namespace.
type SubjectRef struct {
	Application string `json:"application"`
	ID          string `json:"id"`
}

// Known reports whether this reference could identify anything.
func (r SubjectRef) Known() bool { return r.Application != "" && r.ID != "" }

// ScreenNamer is the durable write a naming answer performs, and the whole of it.
//
// One method, and it cannot act. A handler holding this can put a word next to a remembered
// screen; it cannot press a key, run a route, grant authority or register anything — see
// [[ADR-029-resolution-is-not-permission]] for why the narrow interface is the point rather than
// a convenience.
type ScreenNamer interface {
	NameSubject(application, id string, called ScreenName) error
}

// ScreenNameProposalIdentity derives a naming question's identity from the durable subject.
//
// Not from the wording, not from the candidate that happened to need it, and not from the session.
// Two lowering attempts a week apart that are blocked on the same screen are the SAME question,
// and a user who has already been asked must not be asked again because a different play wanted
// the same name.
func ScreenNameProposalIdentity(application, id string) ProposalID {
	return ProposalID("n_" + digest("", "name_screen", application+"/"+id))
}

// ScreenNameQuestion is the sentence a person reads.
//
// It says why it is asking, because the request is otherwise strange: Marco has been happily
// watching this screen for a week without needing a word for it. It names no identifier, quotes
// nothing it has read off the screen, and offers no suggestion — a suggested name is Marco's own
// perception handed back as though the user had said it, which is exactly the leak the type
// system spends `UserSuppliedScreenName` preventing.
func ScreenNameQuestion(s RememberedSubject) string {
	if known := subjectName(s); known != "" {
		return fmt.Sprintf("I can write down how you get out of %s, but the play has to say "+
			"where it starts and I don't have a word for this screen. What do you call it?",
			known)
	}
	return "I can write down how you do this, but the play has to say where it starts and " +
		"I don't have a word for this screen. What do you call it?"
}

// ReviewScreenName puts the naming question for one durable subject, where the policy allows.
//
// # Idempotent by identity
//
// Repeated lowering attempts are the ORDINARY case — every `director learned` recomputes every
// judgement — so this must be safe to call on a loop. One open question per durable subject,
// keyed on the subject and not on the attempt.
//
// Returns the proposal and whether one was newly PUT. A subject that already has a name, an
// already-open question, and a full interruption budget all return false, and only the first of
// those is a reason to stop caring about the subject.
func (l *ProposalLedger) ReviewScreenName(ref SubjectRef, top Topology,
	th ProposalThresholds) (Proposal, bool) {

	if th.MaxOpen == 0 && th.MaxProposals == 0 {
		th = DefaultProposalThresholds()
	}
	if !ref.Known() {
		return Proposal{}, false
	}
	s, ok := top.Subjects[ref.ID]
	if !ok {
		// A screen memory no longer holds cannot be named. Asking would invite the user to
		// name something that will not be there when the answer arrives.
		return Proposal{}, false
	}
	if s.Called != "" {
		// Already named. Not a refusal and not an error — the demand simply is not there.
		return Proposal{}, false
	}
	id := ScreenNameProposalIdentity(ref.Application, ref.ID)
	if existing := l.find(id); existing != nil {
		return *existing, false
	}
	if l.openCount() >= th.MaxOpen || len(l.Proposals) >= th.MaxProposals {
		l.Suppressed++
		return Proposal{}, false
	}
	p := Proposal{
		ID: id, Ask: AskNameScreen, Question: ScreenNameQuestion(s),
		Screen: &SubjectRef{Application: ref.Application, ID: ref.ID},
		Status: ProposalOpen, Asked: 1,
		Support: []EvidenceSource{FromStructure},
	}
	l.Proposals = append(l.Proposals, p)
	return p, true
}

// NamingQuestion returns the OPEN naming question with this identity.
//
// Deliberately a read. The durable write has to succeed before the question is settled — a name
// memory refused is a question still waiting for an answer — so the caller looks the question up,
// writes, and only then calls SettleName. See Runner.NameScreen.
func (l ProposalLedger) NamingQuestion(id ProposalID) (Proposal, bool) {
	for _, p := range l.Proposals {
		if p.ID != id {
			continue
		}
		if p.Ask != AskNameScreen || p.Screen == nil || !p.Screen.Known() {
			return Proposal{}, false
		}
		if p.Status != ProposalOpen {
			return Proposal{}, false
		}
		return p, true
	}
	return Proposal{}, false
}

// SettleName records the name the user gave, against the question that asked for it.
//
// Called only AFTER the durable write succeeded. An invalid name, a name already taken in the
// same application, or a store that refused to write all leave the question open — closing a
// question the user has not successfully answered would lose the only prompt that would let them
// try again.
func (l *ProposalLedger) SettleName(id ProposalID, name ScreenName, inference int) (Proposal, bool) {
	if name == "" {
		return Proposal{}, false
	}
	for i := range l.Proposals {
		p := &l.Proposals[i]
		if p.ID != id || p.Ask != AskNameScreen {
			continue
		}
		if p.Status != ProposalOpen {
			return *p, false
		}
		p.Named = name
		p.Response = ResponseConfirmed
		p.Status = ProposalAnswered
		p.AnsweredAtInference = inference
		return *p, true
	}
	return Proposal{}, false
}

// Naming is AUTHORSHIP, and authorship is reversible.
//
// # What went wrong before this existed
//
// A person was asked to name a screen, answered with the word they had in mind for a DIFFERENT
// screen, and then could not take it back. Withdrawing the answer removed the judgement and left
// the name; uniqueness then reserved that word forever, so the screen they had actually meant
// could never receive it. The only repair was editing semantic memory by hand.
//
// That is not a missing feature. It is the difference between asking somebody a question and
// making them live with their first answer — and Marco asks these questions when it is least
// sure, which is exactly when a person is most likely to answer about the wrong thing.
//
// # The three transitions, all on ONE identity
//
//	unnamed → named      the ordinary case
//	named   → renamed    a correction
//	named   → unnamed    a retraction
//
// None of them mints a subject and none of them deletes one. What the Audience calls a place is a
// property OF that place, and changing it must not change which place it is — a rename that
// created a new subject would orphan every route, goal and target that pointed at the old one.
//
// See [[ADR-069-a-name-is-authored-and-can-be-taken-back]].

// ScreenAuthor is the Audience's authority over what places are called.
//
// Deliberately separate from ScreenNamer, which is what the naming QUESTION uses. That one exists
// to answer a proposal; this one is a person deciding to change their mind, with no question
// pending and none required.
type ScreenAuthor interface {
	ScreenNamer
	// UnnameSubject takes the Audience's name back, leaving the place itself untouched.
	//
	// The subject survives with everything else it holds — its identity, its judgements, the
	// routes that point at it. Only the word goes, and the word becomes available again.
	UnnameSubject(application, id string) error
}
