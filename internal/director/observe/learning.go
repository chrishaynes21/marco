package observe

import (
	"fmt"
	"sort"
	"strings"
)

// Asking whether Marco should LEARN a habit it has noticed, which is a different question from
// asking what a screen is.
//
// # The two questions, and why they are not one
//
//	"I think this is a settings screen. Is that right?"        — about MEANING
//	"I've seen you move between these screens repeatedly.
//	 Do you want me to learn how you do that?"                 — about BEHAVIOUR
//
// They are asked over different evidence, they mean different things, and — most importantly —
// their answers mean different things. `no` to the first says Marco's interpretation is WRONG.
// `no` to the second says the user does not want this learned, and leaves the observation
// entirely intact: the transition really did happen, five times, across three sessions, and a
// preference is not a correction. Overloading one on the other would let "don't bother" quietly
// delete evidence.
//
// # What a yes buys, and what it does not
//
// A pending LEARNING REQUEST. Not a procedure, not a capability, not an action, not a route, and
// not a recorder that quietly starts watching more closely. Marco records that the user is
// willing, and stops there — obtaining a clean demonstration is a separate workflow with its own
// consent, because it may need to observe more than passive discovery does.
//
//	Marco has learned enough to notice a habit. It has not learned enough to imitate one.
//
// # Why the evidence bar is high
//
// This question costs more than a semantic one. "Is this a settings screen?" is answerable in a
// second and wrong answers are cheap; "do you want me to learn this?" invites the user into a
// workflow, and asking it about a transition Marco cannot actually characterise is asking them
// to invest in a guess. So the policy below is deliberately conservative and every refusal has a
// NAMED reason, because "Marco never asked" is otherwise impossible to debug.

// AskKind is what a question is FOR.
//
// The smallest typed distinction that keeps two kinds of question apart, and it exists so that
// response semantics are decided by TYPE rather than inferred from wording. A reader — or a
// future code path — must never have to parse the sentence to know what `no` meant.
type AskKind string

const (
	// AskSemantic is "is my interpretation of this thing correct?".
	AskSemantic AskKind = "confirm_semantic_interpretation"
	// AskLearnRelationship is "do you want me to learn how you make this transition?".
	AskLearnRelationship AskKind = "learn_observed_relationship"
	// AskSecondDemonstration is "you already asked me to learn this; I need another example".
	//
	// Semantically different from both of the others, and typed apart for the same reason
	// they are: the answer means something different. `no` here declines to demonstrate
	// again — it is not a correction of an interpretation and not a withdrawal of the
	// original request to learn.
	AskSecondDemonstration AskKind = "provide_second_demonstration"
	// AskRehearse is "shall I try this myself, once, one step at a time?".
	//
	// The only question whose yes creates authority to act. Typed apart from every other
	// kind because the consequence is different in kind, not in degree: a yes to learning
	// authorised WATCHING, and reaching one from the other is exactly the confusion this
	// vocabulary exists to make impossible.
	AskRehearse AskKind = "rehearse_candidate"
	// AskNameScreen is "what do you call this screen?".
	//
	// The only question here whose answer is a STRING the user writes rather than a yes or a
	// no, and it is typed apart for exactly that reason. Every other ask is closed
	// vocabulary; this one carries a word into durable memory.
	//
	// Asked only when something concrete needs it: a verified play about to be written down
	// cannot say `do Screen's Showing with "…"` until somebody has said what "…" is. Marco
	// does not name screens for the sake of a tidier memory.
	AskNameScreen AskKind = "name_this_screen"
)

// RelationshipRef names the durable edge a learning question is about.
//
// Durable subject ids, never session-local state references and never "the current screen". The
// user may answer minutes later, from somewhere else entirely, and the answer belongs to the
// edge that was asked about.
type RelationshipRef struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// LearningStatus is what the user has said about learning one relationship.
type LearningStatus string

const (
	// LearningPending is "yes, learn this" — recorded, and not yet acted on.
	//
	// The whole of what a yes buys. Nothing downstream reads this as permission to execute,
	// because nothing executable exists for it to point at.
	LearningPending LearningStatus = "pending"
	// LearningRefused is "no, do not learn this".
	//
	// A PREFERENCE, and never a contradiction of the observation. The transition was still
	// observed; the user simply does not want it learned.
	LearningRefused LearningStatus = "refused"
	// LearningDeclined is "not now" — suppressed until the evidence changes shape.
	LearningDeclined LearningStatus = "declined"
	// LearningFulfilled is a request the user agreed to and has now satisfied: a complete
	// demonstration exists.
	//
	// Distinct from pending so a finished request stops arming a capture. Without it every
	// later session would watch the same route again forever, which is the collection loop
	// this design refuses.
	LearningFulfilled LearningStatus = "fulfilled"
)

// LearningRequest is what the user said about learning one relationship, and what was true when
// they said it.
//
// Held ON the relationship rather than in a registry of its own: it is one per edge, it is
// meaningless without its endpoints, and hanging it off the record gives it the same referential
// integrity and the same lifecycle for free. `RememberedSubject` already carries `Knowledge`
// beside `Structure` for exactly this reason.
//
// It is not, and must never become, an entry in anything executable.
type LearningRequest struct {
	Status LearningStatus `json:"status"`
	// Evidence is the digest of the relationship evidence the question was asked ON, so a
	// later session can tell whether the claim has changed shape since.
	Evidence string `json:"evidence"`
	// Observations and Sessions snapshot what was true at the moment of asking, for
	// explainability. Counts only.
	Observations int `json:"observations,omitempty"`
	Sessions     int `json:"sessions,omitempty"`
}

// Pending reports whether Marco is waiting to be shown this.
func (r *LearningRequest) Pending() bool { return r != nil && r.Status == LearningPending }

// LearningThresholds decide when an observed habit has earned a question.
//
// Discrete, integer, and expressed as comparisons against the observation count rather than as
// ratios. There is deliberately no confidence number: a float invented to answer "should I ask"
// becomes the thing everybody reads instead of the evidence, and the evidence here is legible.
type LearningThresholds struct {
	// MinSessions is the independent corroboration below which nothing is proposed.
	//
	// Two. A habit seen ten times in one sitting and a habit seen four times on three
	// separate days are different claims, and only the second has survived a restart, a
	// different window generation, and whatever else changed in between. Ten repetitions
	// inside one session can be one long fiddle with a menu.
	MinSessions int
	// MinObservations is how often the change must have been seen at all.
	//
	// Three. Two is a coincidence with one witness.
	MinObservations int
	// MinIntentShare is the fraction of observations the DOMINANT intent must precede,
	// expressed as a numerator over MinIntentShareOf.
	//
	// A majority. Below that Marco has no characteristic answer to "how did you do it",
	// which is the thing the question offers to learn.
	MinIntentShare   int
	MinIntentShareOf int
	// MaxUnattributedShare is how much of the evidence may have had NO navigation before it,
	// over MaxUnattributedShareOf.
	//
	// Half. A change that mostly happens on its own is not a habit — it is a timer, a scene
	// load, or a network event, and offering to learn "how you do that" would be offering to
	// learn something the user does not do.
	MaxUnattributedShare   int
	MaxUnattributedShareOf int
	// MinRunVariance guards ordered-run AMBIGUITY: if at least this many distinct runs were
	// seen and none of them recurred, the navigation has no characteristic shape.
	MinRunVariance int
}

// DefaultLearningThresholds are the conservative defaults.
func DefaultLearningThresholds() LearningThresholds {
	return LearningThresholds{
		MinSessions: 2, MinObservations: 3,
		MinIntentShare: 1, MinIntentShareOf: 2,
		MaxUnattributedShare: 1, MaxUnattributedShareOf: 2,
		MinRunVariance: 3,
	}
}

// LearningRefusal is a CLOSED reason a relationship was not proposed.
//
// Closed and enumerated because the alternative is silence. "Marco did not ask" has a dozen
// explanations and a reader cannot distinguish them; this is the difference between a policy
// somebody can tune and a black box somebody works around.
type LearningRefusal string

const (
	RefusalInsufficientSessions     LearningRefusal = "insufficient_sessions"
	RefusalInsufficientObservations LearningRefusal = "insufficient_observations"
	RefusalNavigationTooWeak        LearningRefusal = "navigation_too_weak"
	RefusalTooMuchUnattributed      LearningRefusal = "too_much_unattributed"
	RefusalConditionalOnly          LearningRefusal = "conditional_only"
	RefusalRunsInconsistent         LearningRefusal = "runs_inconsistent"
	RefusalEndpointUnresolved       LearningRefusal = "endpoint_unresolved"
	RefusalAlreadyDeclined          LearningRefusal = "already_declined"
	RefusalAlreadyRefused           LearningRefusal = "already_refused"
	RefusalLearningPending          LearningRefusal = "learning_pending"
	RefusalAnotherQuestionOpen      LearningRefusal = "another_question_open"
	RefusalAlreadyAsked             LearningRefusal = "already_asked"
)

// Topology is the durable relationship graph for one application, with its endpoints.
//
// One value rather than two lookups, so the analysis core holds no store handle and makes no
// second round trip per edge. Subjects is keyed by RememberedSubject.ID.
type Topology struct {
	Relationships []RememberedRelationship
	Subjects      map[string]RememberedSubject
}

// LearningAssessment is one relationship, judged.
type LearningAssessment struct {
	Relationship RelationshipRef `json:"relationship"`
	// Eligible says the question may be put; Refusals says why not, when it may not.
	Eligible bool              `json:"eligible"`
	Refusals []LearningRefusal `json:"refusals,omitempty"`
	// Observations, Sessions and the navigation summary, for the report.
	Observations    int               `json:"observations"`
	Sessions        int               `json:"sessions"`
	Preceded        map[NavIntent]int `json:"preceded,omitempty"`
	Unattributed    int               `json:"unattributed,omitempty"`
	ConditionalOnly int               `json:"conditional_only,omitempty"`
	Runs            []NavSequence     `json:"runs,omitempty"`
	// Question is what would be, or was, asked.
	Question string `json:"question,omitempty"`
}

// AssessLearning judges one relationship against the policy.
//
// Every failing rule is collected rather than short-circuited, because a reader tuning the policy
// needs to know whether an edge failed on one count or five. Order is stable.
func AssessLearning(rel RememberedRelationship, top Topology,
	th LearningThresholds) LearningAssessment {

	if th.MinSessions == 0 {
		th = DefaultLearningThresholds()
	}
	e := rel.Evidence()
	out := LearningAssessment{
		Relationship:    RelationshipRef{From: rel.From, To: rel.To},
		Observations:    rel.Observations,
		Sessions:        rel.Sessions,
		Preceded:        rel.Preceded,
		Unattributed:    rel.Unattributed,
		ConditionalOnly: rel.ConditionalOnly,
		Runs:            rel.Sequences,
	}

	// CROSS-SESSION CORROBORATION, checked before anything else because it is the rule most
	// likely to be quietly dropped in favour of "but there are twenty observations".
	if rel.Sessions < th.MinSessions {
		out.Refusals = append(out.Refusals, RefusalInsufficientSessions)
	}
	if rel.Observations < th.MinObservations {
		out.Refusals = append(out.Refusals, RefusalInsufficientObservations)
	}
	// A change that mostly happens on its own is not something the user does.
	if rel.Unattributed*th.MaxUnattributedShareOf >
		rel.Observations*th.MaxUnattributedShare {
		out.Refusals = append(out.Refusals, RefusalTooMuchUnattributed)
	}
	// There must be a characteristic answer to "how". A scatter of one-off intents is not one.
	if dominantIntentSupport(rel)*th.MinIntentShareOf <
		rel.Observations*th.MinIntentShare {
		out.Refusals = append(out.Refusals, RefusalNavigationTooWeak)
	}
	// ADR-013 does not stop at the durable boundary, and it does not stop here either. An
	// edge whose every attributed observation rests on keys that also mean "walk forwards"
	// is not evidence of deliberate navigation.
	if e.ConditionalEvidenceOnly() {
		out.Refusals = append(out.Refusals, RefusalConditionalOnly)
	}
	// Ordered runs: a repeated shape is supporting evidence, and a scatter of one-offs is
	// evidence of ambiguity. Neither is a procedure, and neither is treated as one.
	if runsAreScattered(rel.Sequences, th.MinRunVariance) {
		out.Refusals = append(out.Refusals, RefusalRunsInconsistent)
	}
	// ENDPOINT QUALITY. A screen with no readable name is fine — the question says "this
	// screen". A screen whose every interpretation the user has REJECTED is not: Marco's
	// model of it is wrong, and building an invitation on top of it asks the user to act on
	// a misunderstanding.
	for _, id := range []string{rel.From, rel.To} {
		s, ok := top.Subjects[id]
		if !ok || allContradicted(s) {
			out.Refusals = append(out.Refusals, RefusalEndpointUnresolved)
			break
		}
	}
	// What the user has already said about learning THIS edge.
	switch {
	case rel.Learning.Pending():
		out.Refusals = append(out.Refusals, RefusalLearningPending)
	case rel.Learning != nil && rel.Learning.Status == LearningRefused:
		out.Refusals = append(out.Refusals, RefusalAlreadyRefused)
	case rel.Learning != nil && rel.Learning.Status == LearningDeclined &&
		rel.Learning.Evidence == RelationshipDigest(rel):
		// Declined, and nothing has changed shape since. See RelationshipDigest.
		out.Refusals = append(out.Refusals, RefusalAlreadyDeclined)
	}

	out.Eligible = len(out.Refusals) == 0
	out.Question = LearningQuestion(rel, top)
	return out
}

// dominantIntentSupport is the highest single-intent support on an edge.
func dominantIntentSupport(rel RememberedRelationship) int {
	best := 0
	for _, n := range rel.Preceded {
		if n > best {
			best = n
		}
	}
	return best
}

// runsAreScattered reports navigation with no characteristic shape.
//
// The distinction Part 7 of the design turns on: `down → down → confirm` seen four times
// suggests repeatable behaviour; four different runs seen once each suggests the navigation
// around this change is incidental. Only the second is a refusal — an edge with NO runs at all
// is judged on its intent support instead, because a single-keypress transition legitimately has
// nothing sequential to say.
func runsAreScattered(runs []NavSequence, minVariance int) bool {
	if minVariance <= 0 || len(runs) < minVariance {
		return false
	}
	for _, r := range runs {
		if r.Count > 1 {
			return false
		}
	}
	return true
}

// allContradicted reports a subject whose every interpretation the user rejected.
func allContradicted(s RememberedSubject) bool {
	if len(s.Knowledge) == 0 {
		return false
	}
	for _, k := range s.Knowledge {
		if k.Status != KnowledgeContradicted {
			return false
		}
	}
	return true
}

// RelationshipDigest is the SHAPE of an edge's evidence, for the material-change rule.
//
// # What is in it, and what is deliberately not
//
// Not the raw counts. Observations grow every time the user walks the same path, and a digest
// that moved with them would bring a declined question back within minutes — the exact nagging
// the decline exists to prevent, and a lesson this repository has already paid for twice (see
// EvidenceDigest and LiveRecorder).
//
// What IS in it is the set of things whose change means "this is a different claim now":
//
//   - which intents precede the change at all;
//   - whether it is mostly unattributed;
//   - whether it rests only on context-admitted keys;
//   - which ordered runs have been seen;
//   - a BANDED session count.
//
// The band is the compromise the design asks for. "A new independent session corroborates" is
// listed as a material change, and taken literally it would re-ask every single session. Banding
// it — few, several, many — means a declined question comes back when the corroboration has
// genuinely stepped up, and not merely because the user played again.
func RelationshipDigest(rel RememberedRelationship) string {
	intents := make([]string, 0, len(rel.Preceded))
	for intent := range rel.Preceded {
		intents = append(intents, string(intent))
	}
	sort.Strings(intents)

	runs := make([]string, 0, len(rel.Sequences))
	for _, r := range rel.Sequences {
		runs = append(runs, joinSequence(r.Intents))
	}
	sort.Strings(runs)

	mostlyUnattributed := rel.Unattributed*2 > rel.Observations
	return digest("", "edge="+rel.From+">"+rel.To,
		"intents="+strings.Join(intents, ","),
		"runs="+strings.Join(runs, "|"),
		fmt.Sprintf("unattributed=%t", mostlyUnattributed),
		fmt.Sprintf("conditional=%t", rel.Evidence().ConditionalEvidenceOnly()),
		"sessions="+sessionBand(rel.Sessions))
}

// sessionBand reduces a session count to the steps at which corroboration meaningfully changes.
func sessionBand(n int) string {
	switch {
	case n >= 5:
		return "many"
	case n >= 3:
		return "several"
	}
	return "few"
}

// LearningProposalIdentity derives a learning question's identity from the DURABLE edge.
//
// Not from session-local states, not from the hypothesis kind, and not from the wording. Two
// sessions that both notice the same edge must produce the same question identity, or the user
// gets asked again every time they play.
func LearningProposalIdentity(from, to string) ProposalID {
	return ProposalID("l_" + digest("", "learn", from+">"+to))
}

// LearningQuestion renders the invitation, using the best names available.
//
// # Why the wording adapts
//
// Because a remembered subject id is not something to show a person, and "the screen with four
// controls whose text says settings" is not something to say out loud either. Where an endpoint
// carries a confirmed interpretation, the question names it; where it does not, the question
// says "this screen" and is still perfectly answerable — the user is looking at their own
// memory of playing, not at Marco's record.
//
// What the wording must never do is imply that Marco already knows how. "Do you want me to learn
// how you do that" is an offer; "shall I do that for you" is a claim, and the gap between them is
// this entire milestone.
func LearningQuestion(rel RememberedRelationship, top Topology) string {
	from := subjectName(top.Subjects[rel.From])
	to := subjectName(top.Subjects[rel.To])
	// Two DIFFERENT screens that happen to carry the same confirmed interpretation would read
	// as "from the settings screen to the settings screen", which is worse than saying less:
	// it describes a transition nobody made and invites the user to correct a claim Marco did
	// not make. The second one loses its name and the sentence stays true.
	if from != "" && from == to {
		to = ""
	}
	switch {
	case from != "" && to != "":
		return fmt.Sprintf("I've seen you go from %s to %s several times. "+
			"Do you want me to learn how you do that?", from, to)
	case from != "":
		return fmt.Sprintf("I've seen you go from %s to another screen several times. "+
			"Do you want me to learn how you do that?", from)
	case to != "":
		return fmt.Sprintf("I've seen you reach %s from the same place several times. "+
			"Do you want me to learn how you do that?", to)
	}
	return "I've seen you make the same move between two screens several times. " +
		"Do you want me to learn how you do it?"
}

// subjectName is the plainest true description of a remembered subject, or empty.
//
// Only a CONFIRMED interpretation earns a name. An observed-only guess is Marco talking to
// itself, and putting it in a question would present its own hypothesis back to the user as
// though it were settled — which is how a system starts believing its own output.
func subjectName(s RememberedSubject) string {
	for _, k := range s.Knowledge {
		if k.Status != KnowledgeConfirmed {
			continue
		}
		switch k.Kind {
		case PossibleSettingsLikeState:
			return "the settings screen"
		case PossibleTextEntryState:
			return "the screen you type on"
		case PossibleMenuLikeState:
			return "that menu"
		case PossibleReversiblePlace:
			return "that screen"
		}
	}
	return ""
}

// ReviewRelationships puts learning questions to the user, where the policy allows.
//
// # Where this sits, and why it is second
//
// AFTER the semantic review, sharing the same open-question budget. If Marco is still asking
// what a screen IS, it has no business asking whether to learn a route involving it — the user
// would be answering a question about behaviour before agreeing on the vocabulary. The ordering
// is enforced by nothing more elaborate than call order and one shared MaxOpen, which is the
// smallest policy that produces the right behaviour.
//
// Returns an assessment per relationship, eligible or not, so a diagnostic can explain silence.
func (l *ProposalLedger) ReviewRelationships(top Topology, inference int,
	th ProposalThresholds, lt LearningThresholds) []LearningAssessment {

	if th.MaxOpen == 0 && th.MaxProposals == 0 {
		th = DefaultProposalThresholds()
	}
	out := make([]LearningAssessment, 0, len(top.Relationships))
	// Deterministic order: the best-corroborated edge gets the interruption, and two
	// identical sessions ask the same question.
	rels := append([]RememberedRelationship{}, top.Relationships...)
	sort.Slice(rels, func(i, j int) bool {
		if rels[i].Sessions != rels[j].Sessions {
			return rels[i].Sessions > rels[j].Sessions
		}
		if rels[i].Observations != rels[j].Observations {
			return rels[i].Observations > rels[j].Observations
		}
		return rels[i].From+rels[i].To < rels[j].From+rels[j].To
	})

	for _, rel := range rels {
		a := AssessLearning(rel, top, lt)
		id := LearningProposalIdentity(rel.From, rel.To)
		digest := RelationshipDigest(rel)

		if existing := l.find(id); existing != nil {
			// Already asked this session. A declined question comes back only when the
			// evidence has changed SHAPE, by exactly the rule semantic questions use.
			if existing.Status == ProposalDeclined && existing.Evidence != digest {
				existing.Evidence = digest
				existing.Status = ProposalOpen
				existing.Response = ResponseNone
				existing.AskedAtInference = inference
				existing.Asked++
			} else {
				a.Eligible = false
				a.Refusals = append(a.Refusals, RefusalAlreadyAsked)
			}
			out = append(out, a)
			continue
		}
		if !a.Eligible {
			out = append(out, a)
			continue
		}
		if l.openCount() >= th.MaxOpen {
			a.Eligible = false
			a.Refusals = append(a.Refusals, RefusalAnotherQuestionOpen)
			out = append(out, a)
			continue
		}
		if len(l.Proposals) >= th.MaxProposals {
			l.Suppressed++
			a.Eligible = false
			a.Refusals = append(a.Refusals, RefusalAnotherQuestionOpen)
			out = append(out, a)
			continue
		}
		l.Proposals = append(l.Proposals, Proposal{
			ID: id, Ask: AskLearnRelationship, Question: a.Question,
			Relationship: &RelationshipRef{From: rel.From, To: rel.To},
			Evidence:     digest, Status: ProposalOpen,
			AskedAtInference: inference, Asked: 1,
			Support: []EvidenceSource{FromNavigation, FromRecurrence},
		})
		out = append(out, a)
	}
	return out
}

// openCount is how many questions are awaiting an answer.
func (l *ProposalLedger) openCount() int {
	n := 0
	for _, p := range l.Proposals {
		if p.Status == ProposalOpen {
			n++
		}
	}
	return n
}

// DescribeLearningAssessment renders one judged relationship for a person.
//
// Prints the authority line unconditionally. The single most likely misreading of this record is
// that a pending request means Marco can do something.
func DescribeLearningAssessment(a LearningAssessment, from, to RememberedSubject) []string {
	out := []string{
		"from: " + describeSubject(from, a.Relationship.From),
		"to:   " + describeSubject(to, a.Relationship.To),
		fmt.Sprintf("observed: %d transition(s) across %d session(s)",
			a.Observations, a.Sessions),
	}
	intents := make([]NavIntent, 0, len(a.Preceded))
	for intent := range a.Preceded {
		intents = append(intents, intent)
	}
	sort.Slice(intents, func(i, j int) bool {
		if a.Preceded[intents[i]] != a.Preceded[intents[j]] {
			return a.Preceded[intents[i]] > a.Preceded[intents[j]]
		}
		return intents[i] < intents[j]
	})
	for _, intent := range intents {
		out = append(out, fmt.Sprintf("navigation evidence: %s %d/%d",
			intent, a.Preceded[intent], a.Observations))
	}
	out = append(out, fmt.Sprintf("unattributed: %d/%d", a.Unattributed, a.Observations))
	if a.ConditionalOnly > 0 {
		out = append(out, fmt.Sprintf("context-admitted (weaker): %d", a.ConditionalOnly))
	}
	for _, r := range a.Runs {
		out = append(out, fmt.Sprintf("ordered evidence: %s observed %d time(s)",
			describeIntents(r.Intents), r.Count))
	}
	if a.Eligible {
		out = append(out, "question: "+a.Question)
	} else {
		reasons := make([]string, 0, len(a.Refusals))
		for _, r := range a.Refusals {
			reasons = append(reasons, string(r))
		}
		out = append(out, "not asked: "+strings.Join(reasons, ", "))
	}
	out = append(out, "authority: none. Marco has not learned how to make this happen and "+
		"cannot perform it")
	return out
}

// PlaceWords is what to call a remembered place in a sentence somebody reads.
//
// # Why a rehearsal question needs this
//
// Two open questions once read identically — "I've watched this move twice…" — because both
// resolved their endpoints to nothing and fell back to "this move". They were different routes,
// from two different Homes, and the Audience had no way to tell which one they were answering.
//
// The order is whose word it is. What somebody CALLED a place beats what Marco worked out about
// it, and what Marco worked out beats a description of what it is made of. The description is
// the floor: it always says something, so a question can always name where it means.
//
// Never an id. A subject id in front of a person is Marco failing to answer and hoping nobody
// notices.
func PlaceWords(s RememberedSubject) string {
	if n := strings.TrimSpace(string(s.Called)); n != "" {
		return n
	}
	// WHAT MARCO WORKED OUT, before what Marco can only describe.
	//
	// The rung this function's own doc always promised and did not have. A Place Marco has
	// grounded evidence for is called that; the structural description below is the floor,
	// and asking somebody to read "about back, settings, 148 things on it" and work out
	// which screen it is was making them speak Marco's diagnostics.
	//
	// It loses to `Called` unconditionally. The Audience's word wins.
	//
	// Deleting this must fail TestAnInferredNameBeatsTheStructuralDescription.
	if n := strings.TrimSpace(s.Semantic); n != "" {
		return n
	}
	if w := subjectName(s); w != "" {
		return w
	}
	// AND THE FLOOR IS A SENTENCE, NOT AN INVENTORY.
	//
	// It used to be `DescribeStructure` — "about back, settings, 96 things on it". That is
	// what this function's own note above calls making somebody speak Marco's diagnostics, and
	// two dogfood sessions ran into it: first as a subject id, then as this. Reported verbatim
	// on reading it in the control centre: "I have no clue what that is telling me."
	//
	// An unnamed place is a real state and it is said plainly. What the screen is MADE of is
	// still available — `DescribeStructure` is unchanged and `KnownPlace.Describes` carries it
	// beside the name, which is where a list that has to tell two unnamed screens apart reads
	// it. What changed is only what a SENTENCE calls a place nobody has named.
	//
	// One representation, everywhere, so two surfaces cannot describe the same nameless screen
	// differently. Deleting this must fail TestAnUnnamedPlaceIsCalledTheSameThingEverywhere.
	return Unnamed
}

// Unnamed is what every surface calls a place nobody has named.
//
// A constant rather than a literal, because the whole value of it is that there is exactly one.
const Unnamed = "Unnamed place"

// DescribeStructure says what a screen is made of, in plain words.
//
// The ONE description. Surfaces that need to point at a place a person has not named — the Learn
// panel, the recent-places trail, a rehearsal question — all read this, so two of them cannot
// describe the same screen differently.
func DescribeStructure(sig StructureSignature) string {
	var parts []string
	if len(sig.Terms) > 0 {
		words := make([]string, 0, len(sig.Terms))
		for _, t := range sig.Terms {
			words = append(words, string(t))
		}
		sort.Strings(words)
		parts = append(parts, "about "+strings.Join(words, ", "))
	}
	n := 0
	for _, v := range sig.Roles {
		n += v
	}
	if n > 0 {
		parts = append(parts, fmt.Sprintf("%d things on it", n))
	}
	if len(parts) == 0 {
		return "a screen Marco recognises"
	}
	return strings.Join(parts, ", ")
}

// PlaceWordsAsking is what to call a place in a question that has to tell two of them apart.
//
// # Why a label is not always enough
//
// `PlaceWords` calls every place nobody has named the same thing, which is right for a label and
// wrong for a QUESTION. A rehearsal asks "shall I try getting from X to Y", and if X and Y are
// both "Unnamed place" the sentence describes nothing and cannot be answered — measured by
// TestTwoRoutesFromDifferentPlacesAskDifferentQuestions, which caught exactly that.
//
// So a question keeps the label and adds what the screen is MADE of, which is the only thing that
// distinguishes two places nobody has named. It is still one naming function underneath: this
// composes PlaceWords rather than deciding a name of its own.
//
// Deleting the description must fail TestTwoRoutesFromDifferentPlacesAskDifferentQuestions.
func PlaceWordsAsking(s RememberedSubject) string {
	w := PlaceWords(s)
	if w != Unnamed {
		return w
	}
	if d := DescribeStructure(s.Structure); d != "" {
		return w + " (" + d + ")"
	}
	return w
}
