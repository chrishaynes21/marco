package observe

import (
	"sort"
	"strings"
)

// What survives a session, and what a stored record is allowed to claim.
//
// # A stored record is not a belief
//
// Loading memory reconstructs EVIDENCE and PROVENANCE, never `known = true`. A subject Marco
// merely observed and a subject the user confirmed are different things, and a store that
// flattened them would let yesterday's guess come back today wearing a human signature.
//
// # What is deliberately not stored
//
// No screenshots. No raw OCR text — only terms from the closed vocabulary in terms.go, which
// have already passed the privacy classifier. No key identities. No window generations, process
// ids, absolute coordinates, session references or proposal ids: those are ephemeral, and a
// durable record built on them would be wrong by tomorrow.
//
// The recursive shape guard is `TestNothingCapturedCanReachDurableMemory`, which applies the
// same rule to this type graph that the input trace guards apply to theirs.

// KnowledgeStatus is what is known about one interpretation of a remembered subject.
//
// It preserves the distinction Part 9 of the design turns on: observation and confirmation are
// not the same, and neither is a decline.
type KnowledgeStatus string

const (
	// KnowledgeObserved is a hypothesis that was formed and never put to the user.
	KnowledgeObserved KnowledgeStatus = "observed"
	// KnowledgeConfirmed is one the user was asked about and agreed with.
	KnowledgeConfirmed KnowledgeStatus = "confirmed"
	// KnowledgeContradicted is one the user was asked about and rejected.
	//
	// As durable as a confirmation, and for the same reason: a correction the system
	// forgets overnight is a correction the user has to give again every day.
	KnowledgeContradicted KnowledgeStatus = "contradicted"
	// KnowledgeDeclined is one the user chose not to answer.
	//
	// Stored so the question is not put again, and NOT stored as a denial. It is also not
	// permanent: materially new evidence makes it askable again.
	KnowledgeDeclined KnowledgeStatus = "declined"
	// KnowledgeRetracted is an answer the user has since withdrawn.
	//
	// # Why withdrawing is itself durable
	//
	// Because the alternative is that the old answer comes back. If a retraction were
	// forgotten on restart, a person who corrected themselves would find their correction
	// undone overnight — which is the same failure durability exists to prevent, pointed the
	// other way.
	//
	// It says "do not use my previous answer", NOT "Marco never saw this". The subject, its
	// structure, its visits, its name and every relationship it takes part in are untouched:
	// a person withdrawing a judgement is making a claim about their own reliability, not
	// about what was observed.
	KnowledgeRetracted KnowledgeStatus = "retracted"
)

// Judgement is what a person's answers currently amount to, once revision is taken into account.
//
// THE canonical reading, and the only one. Every consumer asks this rather than switching on
// `Status` itself, because a consumer that understood revision history would be a second place
// deciding what an answer means — and the two would disagree the first time a status was added.
type Judgement string

const (
	// JudgementNone is no active judgement: never asked, declined, or withdrawn.
	JudgementNone Judgement = "none"
	// JudgementConfirmed and JudgementContradicted are a person's live position.
	JudgementConfirmed    Judgement = "confirmed"
	JudgementContradicted Judgement = "contradicted"
)

// Effective is what this record currently claims a person believes.
//
// `observed` is deliberately none: a record existing is not somebody agreeing, and reading it as
// agreement would manufacture a validation nobody gave. `declined` and `retracted` are none for
// the same reason from the other direction — one is "I can't tell", the other is "don't use my
// answer", and neither is evidence for either side.
func (k SemanticKnowledge) Effective() Judgement {
	switch k.Status {
	case KnowledgeConfirmed:
		return JudgementConfirmed
	case KnowledgeContradicted:
		return JudgementContradicted
	}
	return JudgementNone
}

// Active reports whether a person's judgement currently governs.
func (k SemanticKnowledge) Active() bool { return k.Effective() != JudgementNone }

// SemanticKnowledge is what is known about one interpretation of a subject.
type SemanticKnowledge struct {
	Kind   HypothesisKind  `json:"kind"`
	Status KnowledgeStatus `json:"status"`
	// Evidence is the structural evidence digest at the moment this was settled.
	//
	// The basis for deciding later whether the evidence has changed SHAPE — the same
	// materiality rule the within-session proposal policy uses, so a decline is lifted by
	// new kinds of evidence and never by time passing.
	Evidence string `json:"evidence"`
	// Support and Contradictions are COUNTS of the evidence sources that were present,
	// kept for explainability.
	//
	// Counts and source names, never the prose. A durable record needs to support "you
	// confirmed this when it had structural and text support and nothing against it"; it
	// does not need, and must not keep, a transcript of a session.
	Support        []EvidenceSource `json:"support,omitempty"`
	Contradictions []EvidenceSource `json:"contradictions,omitempty"`
	// Answered counts how many times a person has settled this, so a record that has been
	// confirmed repeatedly reads differently from one confirmed once.
	Answered int `json:"answered,omitempty"`
}

// Settled reports whether a person has answered this.
func (k SemanticKnowledge) Settled() bool {
	return k.Status == KnowledgeConfirmed || k.Status == KnowledgeContradicted
}

// RememberedSubject is one structure Marco has seen before, and what is known about it.
type RememberedSubject struct {
	// ID is a stable, content-derived handle for diagnostics and for addressing a record.
	// It is NOT the identity test — CompareStructure is. Two records could in principle share an
	// id's inputs and still be distinguished by the matcher's tolerances.
	ID string `json:"id"`
	// Application namespaces the record. Provenance, and a conservative boundary — see the
	// note in the store.
	Application string `json:"application"`
	// Structure is the identity-bearing evidence.
	Structure StructureSignature `json:"structure"`
	// Knowledge is what is known, one entry per interpretation.
	Knowledge []SemanticKnowledge `json:"knowledge,omitempty"`
	// Learned is WHERE the evidence for this subject came from, in the closed vocabulary.
	//
	// PROVENANCE, never a requirement. A target learned while the accessibility bridge was
	// running says so; nothing that casts an Actor reads it, and a later session resolves the
	// target however it can. The distinction is structural — see EvidenceSource, and note
	// that no field here names a capability.
	//
	// It is also what keeps a correction able to win: an Audience-authored claim and a
	// perception-authored one are different kinds of thing, and merging them would let an
	// observed label outrank the person who corrected it.
	Learned EvidenceSource `json:"learned,omitempty"`
	// Called is what the USER calls this screen, empty until somebody says.
	//
	// The one durable string in this system that comes from a person rather than from
	// perception, and the distinction is the whole privacy boundary: an OCR line, an
	// accessibility label or a window title may never land here, because none of them was
	// offered. A name is offered.
	//
	// It exists so a play can say `do Screen's Showing with "the pause menu"` — a sentence a
	// reader understands — while Director keeps the fingerprint backstage. See
	// [[ADR-031-the-user-names-the-stage]].
	Called string `json:"called,omitempty"`
	// Semantic is what the Place APPEARS to be called, from what an Actor perceived.
	//
	// # Why this is not Called
	//
	// `Called` means "a person offered this word". Writing an inference there would make Marco
	// indistinguishable from the Audience in its own records — and every later reader, including
	// the ones that decide whether a play may say a name out loud, would have no way to tell what
	// somebody said from what Marco guessed. The privacy boundary above is only a boundary while
	// exactly one field is on the human side of it.
	//
	// So this sits beside it, carries its own provenance, and loses to it everywhere a name is
	// presented. The Audience's word wins; this is what Marco says when they have not given one.
	//
	// Admitted under the demonstration licence, through observe.AdmittedPlaceName, which is the
	// same gate a semantic target's label rides and for the same reason.
	Semantic string `json:"semantic,omitempty"`
	// SemanticFrom is which Actor's evidence produced it — never a runtime handle, a provider
	// node id or a session. Two Actors agreeing must strengthen ONE hypothesis rather than
	// producing two competing names, so this records who agreed rather than storing a name each.
	SemanticFrom EvidenceSource `json:"semantic_from,omitempty"`

	// Sessions counts how many separate sessions have matched this subject. Bounded
	// growth: a subject observed in a thousand samples is still one record.
	Sessions int `json:"sessions,omitempty"`
}

// Find returns what is known about one interpretation.
func (r RememberedSubject) Find(kind HypothesisKind) (SemanticKnowledge, bool) {
	for _, k := range r.Knowledge {
		if k.Kind == kind {
			return k, true
		}
	}
	return SemanticKnowledge{}, false
}

// Recollection is what a lookup produced: the verdict, and the record when there was one.
type Recollection struct {
	Verdict MatchVerdict
	Subject RememberedSubject
}

// Memory is durable semantic knowledge, as the session path sees it.
//
// An interface so the analysis core keeps no file handle and the runner keeps no store: the
// composition root supplies the real one. `observe` has no os import and must not gain one — a
// package that could open a file could eventually write a screenshot to it.
type Memory interface {
	// Recall looks up what is known about evidence like this.
	Recall(application string, sig StructureSignature) Recollection
	// Remember records a settled interpretation. Called on semantic EVENTS — an answer, or
	// evidence that has changed shape — and never per sample.
	Remember(application string, sig StructureSignature, k SemanticKnowledge) error
	// RememberRelationships records observed transitions between subjects already stored.
	//
	// One call per session with every edge, rather than one per edge: the store writes its
	// whole file atomically, so a batch is one write where n edges would be n. It is also
	// the honest granularity — the evidence for an edge is complete only when the session
	// that produced it is.
	//
	// Endpoints are RememberedSubject ids the CALLER has already resolved through Recall.
	// The store checks they exist and refuses otherwise; it does not match signatures a
	// second time.
	RememberRelationships(application string, obs []RelationshipObservation) (
		RelationshipUpdate, error)
	// Topology is the durable relationship graph for one application, with its endpoints.
	//
	// A READ, and the only way this package sees relationships at all. It returns the
	// subjects alongside the edges because a learning question needs both — the evidence to
	// judge and the names to say — and two round trips per edge would be two chances for the
	// two halves to describe different moments.
	Topology(application string) Topology
	// RememberLearning records what the user said about learning one relationship.
	//
	// A PREFERENCE about an edge, never a change to the edge's evidence. The store refuses a
	// request whose relationship it does not hold.
	RememberLearning(application string, ref RelationshipRef, req LearningRequest) error
	// RememberFollowUp records what the user said when asked for a SECOND demonstration.
	RememberFollowUp(application string, ref RelationshipRef, req LearningRequest) error
	// FulfilLearning marks a demonstration request as satisfied, by lineage sequence.
	//
	// The thing that stops the collection loop: a request that stayed pending after its
	// demonstration completed would arm a capture in every later session, forever.
	FulfilLearning(application string, ref RelationshipRef, sequence int) error
}

// KnowledgeStatusFor maps an answer onto what is durably known.
//
// One-to-one, and `declined` stays its own thing here exactly as it does everywhere else: a
// decision not to answer is stored so the question is not repeated, never as a denial.
func KnowledgeStatusFor(r UserResponse) KnowledgeStatus {
	switch r {
	case ResponseConfirmed:
		return KnowledgeConfirmed
	case ResponseContradicted:
		return KnowledgeContradicted
	case ResponseDeclined:
		return KnowledgeDeclined
	}
	return KnowledgeObserved
}

// KnowledgeFrom builds the durable record for an answered proposal.
//
// The response vocabulary maps onto the knowledge vocabulary one-to-one, and `declined` stays
// its own thing at every layer: it is not a denial here either.
func KnowledgeFrom(h Hypothesis, r UserResponse) SemanticKnowledge {
	k := SemanticKnowledge{
		Kind: h.Kind, Status: KnowledgeObserved, Evidence: EvidenceDigest(h), Answered: 1,
	}
	switch r {
	case ResponseConfirmed:
		k.Status = KnowledgeConfirmed
	case ResponseContradicted:
		k.Status = KnowledgeContradicted
	case ResponseDeclined:
		k.Status = KnowledgeDeclined
		// A decline is not an answer about the world, so it does not count as one.
		k.Answered = 0
	default:
		k.Answered = 0
	}
	k.Support = sourcesOf(h.Support)
	k.Contradictions = sourcesOf(h.Contradictions)
	return k
}

func sourcesOf(in []Evidence) []EvidenceSource {
	seen := map[EvidenceSource]bool{}
	var out []EvidenceSource
	for _, e := range in {
		if !seen[e.Source] {
			seen[e.Source] = true
			out = append(out, e.Source)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// RecalledValidation turns remembered knowledge into validation for a hypothesis.
//
// Only ESTABLISHED equivalence may do this. A candidate match is structural agreement with
// nothing distinctive to confirm it, and inheriting a user's answer on that basis is exactly the
// false memory this whole design is arranged against.
//
// Observation-only knowledge never becomes validation either: it was never answered, and a
// record existing is not a person agreeing.
func RecalledValidation(rec Recollection, kind HypothesisKind) (*UserValidation, bool) {
	if !rec.Verdict.Established() {
		return nil, false
	}
	k, ok := rec.Subject.Find(kind)
	if !ok || !k.Settled() {
		return nil, false
	}
	resp := ResponseConfirmed
	if k.Status == KnowledgeContradicted {
		resp = ResponseContradicted
	}
	return &UserValidation{Response: resp, Evidence: k.Evidence}, true
}

// CandidateStore is where a completed demonstration is kept.
//
// Separate from Memory rather than folded into it, because the two answer different questions and
// a consumer that wants one should not be handed the other. Nothing here can execute a candidate;
// see the note on ProcedureCandidate.
type CandidateStore interface {
	// RememberCandidate keeps one completed demonstration. The store refuses a candidate
	// whose relationship it does not hold.
	RememberCandidate(application string, c ProcedureCandidate) error
	// Candidates returns what has been demonstrated for one application.
	Candidates(application string) []ProcedureCandidate
}

// Name is what this place is CALLED — the Audience's word if they gave one, otherwise Marco's.
//
// # Why a play may say either
//
// A play refers to a screen by name: `do Screen's Showing with "Home"`. That sentence needs a word
// a reader understands and a resolver can look up; it does not need the word to have come from a
// person. `Called` still wins, because the Audience's word always does.
//
// The distinction between them is kept in the RECORD — see Called and Semantic — precisely so it
// can be reported honestly while both remain usable. Collapsing the two fields would lose that;
// refusing to use the inferred one made Marco demand a name for a screen it had already correctly
// named, with no way to decline.
//
// Empty when neither exists, which is a real answer: a play cannot say where it starts.
//
// Deleting the Semantic branch must fail TestAPlayMaySayTheNameMarcoWorkedOut.
func (r RememberedSubject) Name() string {
	if n := strings.TrimSpace(r.Called); n != "" {
		return n
	}
	return strings.TrimSpace(r.Semantic)
}
