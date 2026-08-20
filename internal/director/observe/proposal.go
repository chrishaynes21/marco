package observe

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Asking the user a question, and treating the answer as evidence rather than as truth.
//
// # Where a question is allowed to come from
//
// A hypothesis, and nothing else. Marco does not look at the screen and invent a question; it
// looks at what it has already concluded, decides whether the evidence is good enough to be
// worth interrupting somebody for, and asks about THAT. The chain stays intact — evidence,
// structure, correlation, hypothesis, question — so every question can be traced back to the
// observations that prompted it, and a bad question is a diagnosable defect rather than a
// mystery.
//
// # Why answers are evidence and not settlement
//
// A confirmation is the strongest single signal this system will ever get, and it is still one
// person answering one question, possibly while distracted, possibly about a screen they were
// not really looking at. So a `yes` is recorded as support with its own provenance and does NOT
// erase the observations that disagreed; a `no` is recorded as a contradiction and does NOT
// delete the observations that agreed. Marco must be able to say "I observed this repeatedly and
// you told me I was wrong", because that sentence is the most useful thing it could possibly
// report about its own model.
//
// # Three answers, and the third is not the second
//
// `declined` means the user chose not to answer. It is NOT evidence that the hypothesis is
// false. Collapsing it into `no` would let "I'm busy" quietly become "you are wrong", which is
// both incorrect and the kind of error nobody would ever notice.
//
// # This milestone ends here
//
// A validated hypothesis is the output. Not a capability, not an action, not a saved macro.
// "I know what this probably is" is not "I know how to accomplish a goal here", and the gap
// between them is where every premature automation lives.

// UserResponse is the closed vocabulary of answers.
//
// An enum rather than free text, deliberately. The user's answer to Marco's own question is
// deliberate feedback and is far less sensitive than passive input capture — but a free-form
// field on an evidence type is how a note becomes a place to put anything, and a correction in
// prose is a different milestone with its own design. See [[ADR-013-navigation-is-meaning-not-keys]]
// for the same argument applied to keys.
type UserResponse string

const (
	// ResponseNone is an unanswered proposal.
	ResponseNone UserResponse = ""
	// ResponseConfirmed is "yes, that is what this is".
	ResponseConfirmed UserResponse = "confirmed"
	// ResponseContradicted is "no, your interpretation is wrong".
	ResponseContradicted UserResponse = "contradicted"
	// ResponseDeclined is "not now" — a decision not to answer, and NOT a denial.
	ResponseDeclined UserResponse = "declined"
)

// Known reports whether a response is in the closed vocabulary.
func (r UserResponse) Known() bool {
	switch r {
	case ResponseConfirmed, ResponseContradicted, ResponseDeclined:
		return true
	}
	return false
}

// Answered reports whether this response settles the question semantically.
//
// A decline does not. That is the whole point of keeping it separate.
func (r UserResponse) Answered() bool {
	return r == ResponseConfirmed || r == ResponseContradicted
}

// ProposalID identifies a QUESTION, not a screen.
//
// Derived from the hypothesis kind and its semantic fingerprint — composition, recurring terms,
// member count — and never from `state_4`, which is a counter that means something different in
// the next session and can change within one. A question about "the recurring screen of five
// controls whose text says settings" must be the same question whether the tracker happens to
// call it state_2 or state_9.
type ProposalID string

// UserValidation is what the user said, with enough provenance to explain it later.
type UserValidation struct {
	Response UserResponse `json:"response"`
	// AskedAtInference and AnsweredAtInference locate the exchange in the session.
	AskedAtInference    int `json:"asked_at_inference,omitempty"`
	AnsweredAtInference int `json:"answered_at_inference,omitempty"`
	// Evidence is the digest of the evidence set the question was asked ON.
	//
	// Provenance, and the thing that makes a stale answer detectable: if the hypothesis
	// has since changed materially, the answer was given about a different claim.
	Evidence string `json:"evidence"`
}

// ProposalStatus is where a question is in its life.
type ProposalStatus string

const (
	// ProposalOpen has been asked and not answered.
	ProposalOpen ProposalStatus = "open"
	// ProposalAnswered has a confirmation or a contradiction.
	ProposalAnswered ProposalStatus = "answered"
	// ProposalDeclined was declined, and is suppressed until the evidence changes shape.
	ProposalDeclined ProposalStatus = "declined"
)

// Proposal is one question put to the user about one hypothesis.
type Proposal struct {
	ID   ProposalID     `json:"id"`
	Kind HypothesisKind `json:"kind"`
	// Ask is what this question is FOR, and it decides what the answer MEANS.
	//
	// Empty means AskSemantic, so every record written before this field existed reads
	// correctly. The distinction is typed rather than inferred from the wording, because a
	// code path that had to parse a sentence to know what `no` meant would eventually get it
	// wrong — and the two `no`s have opposite consequences. See learning.go.
	Ask AskKind `json:"ask,omitempty"`
	// Question is the sentence a person reads. Generic, hedged, and free of implementation
	// identifiers — see questionFor.
	Question string  `json:"question"`
	Subject  Subject `json:"subject"`
	// Relationship is the durable edge a learning question is about, nil for a semantic one.
	//
	// Durable subject ids. A delayed answer must still refer to the edge that was asked
	// about, even when the user is now looking at something else entirely.
	Relationship *RelationshipRef `json:"relationship,omitempty"`
	// Screen is the durable subject an AskNameScreen question is about, nil for every other
	// kind.
	//
	// The same argument as Relationship, and it matters more here: this question's answer is
	// a WORD that gets written next to one remembered screen forever. Binding it to whatever
	// is in front of the user when they answer would put their word on the wrong thing, in
	// the wrong application, with nothing in the record to say it happened.
	Screen *SubjectRef `json:"screen,omitempty"`
	// Named is what the user called the screen, empty for every other kind of question.
	//
	// The ONE piece of free text this whole record may carry, typed rather than a string so
	// that arriving here is a deliberate act. See ScreenName.
	Named ScreenName `json:"named,omitempty"`
	// Evidence is the structural digest the question was asked on, and the basis for
	// deciding later whether anything has materially changed.
	Evidence string         `json:"evidence"`
	Status   ProposalStatus `json:"status"`
	Response UserResponse   `json:"response,omitempty"`
	// Retracted says a previous answer was WITHDRAWN by the user.
	//
	// Distinct from a decline, which is "I cannot tell", and from never having been asked.
	// Carried so a surface can say "you withdrew your answer" rather than silently showing
	// a question that looks untouched.
	Retracted bool `json:"retracted,omitempty"`
	// AskedAtInference and AnsweredAtInference locate the exchange in the session.
	AskedAtInference    int `json:"asked_at_inference"`
	AnsweredAtInference int `json:"answered_at_inference,omitempty"`
	// Asked counts how many times this question has been put. More than one means the
	// evidence changed shape after a decline.
	Asked int `json:"asked"`
	// Recognised says this record came from DURABLE MEMORY rather than from a question put
	// in this session, and RecognisedAs is the remembered subject it matched.
	//
	// Kept apart from an ordinary answer so a report can say WHY nothing was asked. A
	// question that silently fails to appear is indistinguishable from a question the
	// policy declined to ask, and "I recognised this from before" is the explanation the
	// user is owed.
	Recognised   bool   `json:"recognised,omitempty"`
	RecognisedAs string `json:"recognised_as,omitempty"`
	// Support and Contradictions are the evidence SOURCES present when the question was
	// put — closed vocabulary, never prose. Carried so an answer can be remembered with
	// its provenance without re-deriving the hypothesis.
	Support        []EvidenceSource `json:"support,omitempty"`
	Contradictions []EvidenceSource `json:"contradictions,omitempty"`
}

// Validation renders this proposal as validation evidence for a hypothesis.
func (p Proposal) Validation() *UserValidation {
	if p.Response == ResponseNone {
		return nil
	}
	return &UserValidation{
		Response: p.Response, Evidence: p.Evidence,
		AskedAtInference: p.AskedAtInference, AnsweredAtInference: p.AnsweredAtInference,
	}
}

// ProposalThresholds bound when Marco is allowed to interrupt.
type ProposalThresholds struct {
	// MaxOpen bounds unanswered questions at once. A queue of questions is a queue of
	// interruptions, and somebody who ignored the first is not helped by the fourth.
	MaxOpen int
	// MaxProposals bounds the ledger for a session.
	MaxProposals int
}

// DefaultProposalThresholds are the provisional defaults.
//
// One open question at a time. This is a passive observer interrupting somebody who is playing a
// game: the cost of asking is not the cost of a dialog, it is the cost of breaking their
// attention, and it is not paid per question but per interruption.
func DefaultProposalThresholds() ProposalThresholds {
	return ProposalThresholds{MaxOpen: 1, MaxProposals: 32}
}

// ProposalLedger is one session's record of what was asked and what came back.
//
// Session-scoped and plain data: no goroutines, no clock, no I/O. It is carried into the
// terminal Result so an answer can still arrive after the session has ended — which is the
// ORDINARY case, not an edge case, because a three-minute session usually finishes before
// somebody reads the question.
type ProposalLedger struct {
	Proposals []Proposal `json:"proposals,omitempty"`
	// Suppressed counts questions withheld because they had already been asked or declined
	// on the same evidence. Reported rather than silent: "Marco asked nothing" and "Marco
	// had nothing to ask" are different, and only one of them is reassuring.
	Suppressed int `json:"suppressed,omitempty"`
}

// Review is THE production entry point: it attaches validation to hypotheses and asks about the
// ones that have earned a question.
//
// One call doing both, deliberately. They are the two halves of the same idea — what the user
// has already said about a hypothesis, and whether it is time to say anything — and splitting
// them would create a second call site that could be forgotten independently. Deleting this call
// must break the production path; see TestTheProductionSessionPathProposesQuestions.
func (l *ProposalLedger) Review(hs []Hypothesis, inference int,
	th ProposalThresholds) []Hypothesis {

	if th.MaxOpen == 0 && th.MaxProposals == 0 {
		th = DefaultProposalThresholds()
	}
	out := make([]Hypothesis, 0, len(hs))
	for _, h := range hs {
		id := ProposalIdentity(h)
		evidence := EvidenceDigest(h)

		// Anything already known about this question is attached FIRST, so a hypothesis
		// carries the user's answer whether or not it is currently ask-eligible.
		if existing := l.find(id); existing != nil {
			out = append(out, h.WithValidation(existing.Validation()))
			continue
		}
		if !l.eligible(h, th) {
			out = append(out, h)
			continue
		}
		if len(l.Proposals) >= th.MaxProposals {
			l.Suppressed++
			out = append(out, h)
			continue
		}
		l.Proposals = append(l.Proposals, Proposal{
			ID: id, Kind: h.Kind, Question: questionFor(h), Subject: h.Subject,
			Evidence: evidence, Status: ProposalOpen,
			AskedAtInference: inference, Asked: 1,
			Support:        sourcesOf(h.Support),
			Contradictions: sourcesOf(h.Contradictions),
		})
		out = append(out, h)
	}
	return out
}

// eligible decides whether a hypothesis has earned an interruption.
//
// The policy is expressed over the EXISTING discrete status model rather than a new confidence
// number, because a float invented to answer "should I ask" would immediately become the thing
// everybody reads instead of the evidence — and the whole hypothesis layer is arranged so that
// the evidence stays legible.
func (l *ProposalLedger) eligible(h Hypothesis, th ProposalThresholds) bool {
	// Only a supported hypothesis. `tentative` has not recurred enough to lean on, and
	// asking about it would train the user to expect noise. `contested` must never be put
	// as though it were established — the honest question there is not "is this a settings
	// screen?" but a discussion of the contradiction, and this milestone does not have the
	// vocabulary for that conversation.
	if h.Status != StatusSupported {
		return false
	}
	// A hypothesis resting entirely on context-admitted navigation is already `contested`
	// by ADR-013's rule, so this is belt and braces — but the rule is worth stating where
	// somebody deciding whether to interrupt a person can see it.
	if len(h.Contradictions) > 0 {
		return false
	}
	// One interruption at a time.
	open := 0
	for _, p := range l.Proposals {
		if p.Status == ProposalOpen {
			open++
		}
	}
	return open < th.MaxOpen
}

// Respond records an answer against the question it was asked about.
//
// Keyed on the proposal's own identity, NOT on whatever is on screen now. The user may answer
// minutes later, after the screen has changed several times and the session has ended; the
// answer belongs to the question that was put, and attaching it to the current state would be
// the confirmation equivalent of a stale ordinal.
func (l *ProposalLedger) Respond(id ProposalID, r UserResponse, inference int) (Proposal, bool) {
	if !r.Known() {
		return Proposal{}, false
	}
	for i := range l.Proposals {
		if l.Proposals[i].ID != id {
			continue
		}
		p := &l.Proposals[i]
		// A NAMING question is not answerable with yes, no or not-now, and the generic path
		// refuses it rather than recording something meaningless against it. This is what
		// keeps the closed vocabulary closed: `AskNameScreen` has its own typed response,
		// and every other question keeps the three words it always had.
		if p.Ask == AskNameScreen {
			return Proposal{}, false
		}
		// An answered question is not re-answered. The first answer is the one given in
		// response to the question that was actually asked.
		if p.Status == ProposalAnswered {
			return *p, false
		}
		p.Response = r
		p.AnsweredAtInference = inference
		if r == ResponseDeclined {
			// A decline suppresses the question and touches no semantic evidence.
			p.Status = ProposalDeclined
		} else {
			p.Status = ProposalAnswered
		}
		return *p, true
	}
	return Proposal{}, false
}

// Clone returns an independent copy.
//
// The ledger outlives the session it was built in — a Result is handed out while answers may
// still arrive — so a snapshot must not share its backing array with the live one.
func (l ProposalLedger) Clone() ProposalLedger {
	out := ProposalLedger{Suppressed: l.Suppressed}
	if len(l.Proposals) > 0 {
		out.Proposals = append([]Proposal{}, l.Proposals...)
	}
	return out
}

// Annotate attaches what the user has said to each hypothesis, asking nothing.
//
// A value receiver and no mutation: this is the read the terminal Result takes, and a report
// being generated must not be able to put a new question to somebody.
func (l ProposalLedger) Annotate(hs []Hypothesis) []Hypothesis {
	out := make([]Hypothesis, 0, len(hs))
	for _, h := range hs {
		if p := l.lookup(ProposalIdentity(h)); p != nil {
			h = h.WithValidation(p.Validation())
		}
		out = append(out, h)
	}
	return out
}

// lookup finds a proposal by identity without allowing mutation.
func (l ProposalLedger) lookup(id ProposalID) *Proposal {
	for i := range l.Proposals {
		if l.Proposals[i].ID == id {
			p := l.Proposals[i]
			return &p
		}
	}
	return nil
}

// Open returns the questions awaiting an answer.
func (l ProposalLedger) Open() []Proposal {
	var out []Proposal
	for _, p := range l.Proposals {
		if p.Status == ProposalOpen {
			out = append(out, p)
		}
	}
	return out
}

// find returns the live record for a question identity, if the evidence still matches.
//
// # How a declined question comes back
//
// By the evidence changing SHAPE — a new interface term, a different structural composition —
// and not by time passing and not by seeing the same thing again. That distinction is the whole
// difference between a system that asks once more when it has genuinely learned something and
// one that nags.
//
// It is also the lesson `LiveRecorder` already paid for: keying materiality on a count that
// grows every sample republished an unchanged claim forever. Episode counts grow; the structural
// digest does not.
func (l *ProposalLedger) find(id ProposalID) *Proposal {
	for i := range l.Proposals {
		if l.Proposals[i].ID == id {
			return &l.Proposals[i]
		}
	}
	return nil
}

// Reask reopens a declined question whose evidence has since changed shape.
//
// Called from Review's caller path via Refresh; separated so the rule is testable on its own.
func (l *ProposalLedger) Reask(h Hypothesis, inference int) bool {
	p := l.find(ProposalIdentity(h))
	if p == nil || p.Status != ProposalDeclined {
		return false
	}
	if digest := EvidenceDigest(h); digest != p.Evidence {
		p.Evidence = digest
		p.Status = ProposalOpen
		p.Response = ResponseNone
		p.AskedAtInference = inference
		p.Asked++
		return true
	}
	return false
}

// RecallFrom seeds the ledger with what durable memory already knows.
//
// # Why memory becomes a ledger entry rather than a special case
//
// A remembered answer IS an answered question — just one answered in an earlier session. Seeding
// it into the ledger means every existing behaviour applies unchanged: the hypothesis gets its
// validation, the proposal policy sees the question as settled and does not ask, the report shows
// it under "previously asked", and a remembered DECLINE is reopened by exactly the same
// material-change rule as a decline given five minutes ago.
//
// The alternative — a parallel "memory" path beside the proposal path — would be two mechanisms
// that have to agree about suppression forever.
//
// Only ESTABLISHED matches are seeded. A candidate match is structural agreement with nothing
// distinctive to confirm it, and inheriting a user's answer on that basis is the false memory
// this design is most concerned about.
func (l *ProposalLedger) RecallFrom(hs []Hypothesis, application string, m Memory) []Hypothesis {
	if m == nil {
		return hs
	}
	for _, h := range hs {
		id := ProposalIdentity(h)
		if l.find(id) != nil {
			// This session has already asked or recalled it; the live record wins.
			continue
		}
		rec := m.Recall(application, SignatureOf(h))
		if !rec.Verdict.Established() {
			continue
		}
		k, ok := rec.Subject.Find(h.Kind)
		if !ok {
			// The subject is recognised and nothing is known about THIS interpretation.
			// Recognition alone is not an answer, so the question stays askable.
			continue
		}
		if k.Status == KnowledgeObserved {
			// Observed-only knowledge was never put to anybody. A record existing is not
			// a person agreeing, and seeding it would manufacture a validation nobody gave.
			continue
		}
		p := Proposal{
			ID: id, Kind: h.Kind, Question: questionFor(h), Subject: h.Subject,
			Evidence: k.Evidence, Recognised: true, RecognisedAs: rec.Subject.ID,
			Support: k.Support, Contradictions: k.Contradictions,
		}
		// THE effective judgement, and it is asked of the record rather than switched on
		// here. A superseded answer must not govern, and a consumer that read `Status`
		// directly would be a second place deciding what an answer means.
		//
		// Deleting this in favour of a Status switch must fail
		// TestARevisedAnswerIsWhatALaterSessionRecalls.
		switch k.Effective() {
		case JudgementConfirmed:
			p.Status, p.Response = ProposalAnswered, ResponseConfirmed
		case JudgementContradicted:
			p.Status, p.Response = ProposalAnswered, ResponseContradicted
		default:
			// No active judgement — declined, or withdrawn. Both leave the question
			// reconsiderable when the evidence changes SHAPE, which is the bounded policy
			// that already exists; neither resurfaces on a timer.
			p.Status, p.Response = ProposalDeclined, ResponseNone
			if k.Status == KnowledgeDeclined {
				p.Response = ResponseDeclined
			}
			p.Retracted = k.Status == KnowledgeRetracted
		}
		l.Proposals = append(l.Proposals, p)
	}
	return hs
}

// Refresh reopens declined questions whose evidence has changed shape, then reviews.
//
// The order matters: a declined question is reconsidered BEFORE the open-question budget is
// spent on a new one, because a question the user already engaged with once is a better use of
// an interruption than a fresh one.
func (l *ProposalLedger) Refresh(hs []Hypothesis, inference int,
	th ProposalThresholds) []Hypothesis {

	for _, h := range hs {
		l.Reask(h, inference)
	}
	return l.Review(hs, inference, th)
}

// ProposalIdentity derives a question's identity from what it is ABOUT.
//
// Kind plus the semantic fingerprint. Session-local references are excluded on purpose: they are
// counters, they differ between runs, and a question keyed on one would be asked again every
// time the tracker renumbered.
// Identity is deliberately STRUCTURAL — kind, role composition, member count — and deliberately
// excludes the interface terms, which belong to the evidence instead.
//
// That split was a defect first. With terms in the identity, a screen that started saying one
// more word became a NEW question, so a declined "is this a settings screen?" came straight back
// the moment OCR read another label — bypassing the suppression entirely and producing exactly
// the nagging the decline exists to prevent.
//
// The cost is that two structurally identical screens classified the same way collapse into one
// question. That is acceptable and arguably correct: if Marco cannot tell them apart from
// structure, "a recurring screen of five controls" IS the same question about both, and asking
// it twice would be asking the user to distinguish something Marco has not distinguished.
func ProposalIdentity(h Hypothesis) ProposalID {
	return ProposalID("q_" + digest(h.Kind, identityKey(h.Subject)))
}

// EvidenceDigest is the structural shape of the evidence behind a hypothesis.
//
// Deliberately NOT a function of counts. Episodes, observations and support totals all grow as a
// session runs, and a digest that moved with them would make every question materially new every
// few seconds.
//
// # Corrected 2026-08-09: the evidence SOURCES were removed
//
// This digest originally included the set of support and contradiction sources, and
// [[ADR-015-a-question-is-evidence-not-settlement]] described it that way. Implementation
// disproved it. The source composition GROWS within a session: `possible_choice_group` carries
// `[structure]` when it is first supported and `[recurrence structure]` once a second episode
// accumulates. So one hypothesis has different digests at different moments of one session.
//
// Within a session that was survivable. Across sessions it is not: durable memory records the
// digest at the moment the user ANSWERED, and a later session compares it against the digest at
// the moment it first RECALLS — a different point in the accumulation. The two never agree, so
// every declined question returned on every restart, which is precisely the nagging the decline
// exists to prevent.
//
// What remains is genuinely structural: the interpretation, what the subject IS, and what the
// interface says about itself. Those are the things whose change means "this is a different
// claim now" — and the interface terms were already the ADR's own worked example of material
// change. A hypothesis that gains a contradiction does not need to be caught here, because a
// contradiction makes it `contested` and therefore ineligible to be asked at all.
func EvidenceDigest(h Hypothesis) string {
	terms := make([]string, 0, len(h.Subject.Fingerprint.Terms))
	for _, t := range h.Subject.Fingerprint.Terms {
		terms = append(terms, string(t))
	}
	sort.Strings(terms)
	return digest(h.Kind, identityKey(h.Subject), "terms="+strings.Join(terms, ","))
}

// identityKey renders the STRUCTURAL part of a subject: what the thing is, not what supports it.
//
// Session-local references are excluded — they are counters, and a question keyed on one would
// be asked again every time the tracker renumbered.
func identityKey(s Subject) string {
	f := s.Fingerprint
	roles := make([]string, 0, len(f.Roles))
	for r, n := range f.Roles {
		roles = append(roles, fmt.Sprintf("%s:%d", r, n))
	}
	sort.Strings(roles)
	return fmt.Sprintf("subject=%s|roles=%s|members=%d",
		s.Kind, strings.Join(roles, ","), f.Members)
}

func digest(kind HypothesisKind, parts ...string) string {
	h := sha256.New()
	h.Write([]byte(kind))
	for _, p := range parts {
		h.Write([]byte{0})
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// questionFor renders a hypothesis as a sentence a person can answer.
//
// # What the wording has to do
//
// Say what Marco thinks, say that it is unsure, and say nothing that implies it has learned to
// DO anything. "I think this may be a settings screen" invites a correction; "I've learned your
// settings screen" invites a complaint.
//
// No implementation identifiers. `state_7` is meaningless to the person being asked and actively
// misleading — it looks like a name for something, and it is a counter.
func questionFor(h Hypothesis) string {
	switch h.Kind {
	case PossibleSettingsLikeState:
		return "I keep seeing a screen that looks like settings or options — " +
			"a group of choices, with words like that on it. Is that what it is?"
	case PossibleMenuLikeState:
		return "I keep seeing a screen that looks like a menu — a set of choices " +
			"you pick from. Is that what it is?"
	case PossibleTextEntryState:
		return "I keep seeing a screen with somewhere to type on it. Is that a " +
			"search or entry screen?"
	case PossibleChoiceGroup:
		return "I keep seeing a group of controls that appear together, as though " +
			"they were one set of options. Do they belong together?"
	case PossibleReversiblePlace:
		return "There's a screen you seem to open and then come back from. Is that " +
			"somewhere you go on purpose?"
	case PossibleTransitionAction:
		return "One thing you do seems to lead to a particular screen. Is that " +
			"how you get there?"
	case PossibleSelectionSequence:
		return "You seem to repeat the same short sequence to get somewhere. Is " +
			"that a deliberate way of choosing something?"
	}
	return "I've noticed something recurring on screen but I'm not sure what it is. " +
		"Does it mean anything to you?"
}

// ── revision ──────────────────────────────────────────────────────────────────

// Changing your mind, and why it is a different operation from answering.
//
// `Respond` refuses to answer an answered question, and that refusal is worth keeping: it stops
// a double-submitted click, a stale panel and a replayed request from quietly overwriting what
// somebody actually said. What it must NOT do is take away the right to correct yourself.
//
// So revision is EXPLICIT. `Answer` stays one-shot; `Revise` and `Retract` are separate verbs a
// caller has to mean. An old button pressed twice is not a revision — a person choosing to change
// their answer is.
//
//	Answer(Q, no)      → recorded
//	Answer(Q, yes)     → refused, "that question was already answered"
//	Revise(Q, yes)     → allowed
//	Retract(Q)         → allowed
//
// Neither touches perception. A subject, its structure, its visits, its name and every
// relationship it takes part in survive a retraction untouched: withdrawing a judgement is a
// claim about the answer's reliability, not about what was observed.

// Revise replaces a settled answer with a new one.
//
// Refuses a question that was never answered — there is nothing to revise, and accepting it would
// make `Revise` a second way to answer, which is exactly the door `Respond` closes.
func (l *ProposalLedger) Revise(id ProposalID, r UserResponse, inference int) (Proposal, bool) {
	if !r.Known() {
		return Proposal{}, false
	}
	for i := range l.Proposals {
		p := &l.Proposals[i]
		if p.ID != id {
			continue
		}
		if p.Ask == AskNameScreen {
			// A name is not one of three words, here as everywhere else.
			return Proposal{}, false
		}
		if p.Status == ProposalOpen {
			// Never answered. `Answer` is the operation for that, and letting this
			// stand in would make an accidental revision indistinguishable from a
			// first answer.
			return Proposal{}, false
		}
		p.Response = r
		p.Retracted = false
		p.AnsweredAtInference = inference
		if r == ResponseDeclined {
			p.Status = ProposalDeclined
		} else {
			p.Status = ProposalAnswered
		}
		return *p, true
	}
	return Proposal{}, false
}

// Retract withdraws a previous answer, leaving no active judgement.
//
// The proposition becomes reconsiderable on the ordinary rule — when its evidence changes shape —
// rather than immediately. Somebody who has just said "ignore what I told you" has not asked to
// be asked again this second.
func (l *ProposalLedger) Retract(id ProposalID, inference int) (Proposal, bool) {
	for i := range l.Proposals {
		p := &l.Proposals[i]
		if p.ID != id {
			continue
		}
		if p.Status == ProposalOpen {
			return Proposal{}, false
		}
		p.Response = ResponseNone
		p.Retracted = true
		p.Status = ProposalDeclined
		p.AnsweredAtInference = inference
		return *p, true
	}
	return Proposal{}, false
}

// KnowledgeStatusForRevision maps a revision onto what becomes durable.
//
// Separate from KnowledgeStatusFor because a RETRACTION is not one of the three answers: it is
// the absence of one, made durable so it survives a restart. Without that, withdrawing a
// judgement would be undone overnight — the same failure durability exists to prevent, pointed
// the other way.
func KnowledgeStatusForRevision(p Proposal) KnowledgeStatus {
	if p.Retracted {
		return KnowledgeRetracted
	}
	return KnowledgeStatusFor(p.Response)
}
