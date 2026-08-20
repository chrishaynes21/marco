package observe

import (
	"fmt"
	"sort"
	"strings"
)

// What a person has told Marco, as a person can read it back.
//
// # Why this is not a memory browser
//
// The store holds everything Marco has ever recognised. Almost none of that is somebody's
// business: a subject it observed and never asked about is a guess, not a statement, and putting
// guesses in a list headed "what you told me" would invite people to correct things they never
// said. This surface shows INTENTIONAL judgements only — the ones a person actually answered —
// read through `Effective` so a withdrawn answer stops appearing as a truth rather than appearing
// as a truth with an asterisk.
//
// # Why revision needs a second door
//
// `ProposalLedger.Revise` reaches a judgement through the QUESTION that produced it, which works
// for as long as a session still holds that question. It stops working the moment the subject
// cannot be recognised again — and that is not a hypothetical: the Explorer record this milestone
// exists because of is exactly that case. Its stored envelope puts the group mostly above the
// window, so nothing Marco can ever see again will match it, so no session will ever recall the
// question, so the answer became uncorrectable through every path that existed.
//
// A judgement a person can no longer withdraw is worse than one Marco never stored. So the durable
// verbs below reach a judgement by the subject it is ABOUT. They enforce the same rule the ledger
// does — only a settled answer may be revised, and revision is a separate verb from answering —
// and they write through the same `Remember` the session path writes through, so there is still
// one place that decides what a judgement means.

// KnownJudgement is one thing a person has told Marco.
//
// Carries no evidence prose, no accessibility text and no geometry: a reader needs to know what
// they said and what it was about, and the description is built from counts and the closed
// vocabulary. The one free string is `Called`, which is the user's own word for the screen.
type KnownJudgement struct {
	Application string         `json:"application"`
	Subject     string         `json:"subject"`
	Kind        HypothesisKind `json:"kind"`
	// Judgement is what the answer currently amounts to, through the canonical reading.
	// Never `none` here: a withdrawn or unanswered record is not something you told Marco.
	Judgement Judgement `json:"judgement"`
	// Said is the sentence a person reads back.
	Said string `json:"said"`
	// About is the bounded structural description — counts and kinds, nothing quoted.
	About string `json:"about"`
	// Called is the user's own name for the screen, when they have given one.
	Called string `json:"called,omitempty"`
	// Locatable says whether Marco can currently point at what this refers to.
	//
	// Remembering a judgement and being able to find its subject are DIFFERENT things, and
	// collapsing them is how a person ends up unable to withdraw an answer about something
	// that has scrolled out of recognisability. A surface must show the judgement either way
	// and must not pretend the referent is visible.
	Locatable bool `json:"locatable"`
	// Answered is how many times a person has settled this.
	Answered int `json:"answered,omitempty"`
}

// WhatIsKnown is the canonical list of intentional judgements.
//
// `recognised` is the set of durable subject ids some live or recent session currently recalls.
// Locatability is a fact about NOW, and only perception knows it; this function will not guess at
// it from stored geometry, because stored geometry is what made the Explorer record unfindable in
// the first place.
func WhatIsKnown(subjects []RememberedSubject, recognised map[string]bool) []KnownJudgement {
	var out []KnownJudgement
	for _, s := range subjects {
		for _, k := range s.Knowledge {
			// THE filter, and it is the canonical reading rather than a status test.
			// Observed-only knowledge was never put to anybody, and a withdrawn answer is
			// one a person has told Marco to stop using.
			if !k.Active() {
				continue
			}
			out = append(out, KnownJudgement{
				Application: s.Application, Subject: s.ID, Kind: k.Kind,
				Judgement: k.Effective(),
				Said:      SaidAbout(k.Kind, k.Effective()),
				About:     DescribeSubject(s),
				Called:    s.Called,
				Locatable: recognised[s.ID],
				Answered:  k.Answered,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Application != out[j].Application {
			return out[i].Application < out[j].Application
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

// SaidAbout is the answer read back as a sentence.
//
// The user's own words are not available — they said yes or no to a question Marco phrased — so
// this states the CLAIM in the affirmative or the negative, and never re-asks. "You told Marco:
// these controls are not one set of choices" is something a person can check against their memory
// of answering; "possible_choice_group: contradicted" is not.
func SaidAbout(kind HypothesisKind, j Judgement) string {
	yes, no := claimsAbout(kind)
	if j == JudgementContradicted {
		return no
	}
	return yes
}

func claimsAbout(kind HypothesisKind) (yes, no string) {
	switch kind {
	case PossibleChoiceGroup:
		return "These controls belong together as one set of choices.",
			"These controls are NOT one set of choices."
	case PossibleMenuLikeState:
		return "This screen is a menu — a set of choices you pick from.",
			"This screen is NOT a menu."
	case PossibleSettingsLikeState:
		return "This screen is settings or options.",
			"This screen is NOT settings or options."
	case PossibleTextEntryState:
		return "This screen is somewhere you type — a search or entry screen.",
			"This screen is NOT somewhere you type."
	case PossibleReversiblePlace:
		return "This is somewhere you go on purpose and come back from.",
			"This is NOT somewhere you go on purpose."
	case PossibleTransitionAction:
		return "This is how you get to that screen.",
			"This is NOT how you get to that screen."
	case PossibleSelectionSequence:
		return "This short sequence is a deliberate way of choosing something.",
			"This short sequence is NOT a deliberate way of choosing something."
	}
	return "This means something.", "This does NOT mean anything."
}

// DescribeSubject is the bounded structural description of what a judgement is about.
//
// Counts and role names from the detector's vocabulary, and the interface terms that already
// passed the privacy classifier. No labels, no quoted text, no coordinates: a person needs enough
// to recognise which thing is meant, not a transcript of their screen.
func DescribeSubject(s RememberedSubject) string {
	var parts []string
	if n := s.Structure.Members; n > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", n, plural(n, "control", "controls")))
	}
	if role, n := dominantRole(s.Structure.Roles); role != "" && n > 0 {
		parts = append(parts, fmt.Sprintf("mostly %s", strings.ReplaceAll(role, "_", " ")))
	}
	if len(s.Structure.Terms) > 0 {
		words := make([]string, 0, len(s.Structure.Terms))
		for _, t := range s.Structure.Terms {
			words = append(words, string(t))
		}
		sort.Strings(words)
		parts = append(parts, "words like "+strings.Join(words, ", "))
	}
	if s.Sessions > 0 {
		parts = append(parts, fmt.Sprintf("seen on %d %s",
			s.Sessions, plural(s.Sessions, "visit", "visits")))
	}
	if len(parts) == 0 {
		return "a structure Marco recognised"
	}
	return strings.Join(parts, " · ")
}

func dominantRole(roles map[string]int) (string, int) {
	var best string
	var most int
	for role, n := range roles {
		if n > most || (n == most && role < best) {
			best, most = role, n
		}
	}
	return best, most
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// ── the durable verbs ─────────────────────────────────────────────────────────

// KnowledgeStore is durable knowledge as this surface reaches it.
//
// Narrower than Memory on purpose: nothing here can record a relationship, arm a demonstration or
// create authority. It reads what is stored and rewrites one interpretation of one subject.
type KnowledgeStore interface {
	Subjects() []RememberedSubject
	Subject(id string) (RememberedSubject, bool)
	Remember(application string, sig StructureSignature, k SemanticKnowledge) error
}

// ReviseKnown replaces a settled durable judgement with a new answer.
//
// The same rule the ledger enforces: only something already ANSWERED may be revised. A subject
// Marco merely observed, or one whose answer has already been withdrawn, is not a judgement to
// change — accepting either would make this a second way to answer, reachable without ever having
// been asked.
func ReviseKnown(store KnowledgeStore, subject string, kind HypothesisKind, r UserResponse) error {
	return rewrite(store, subject, kind, func(k SemanticKnowledge) (SemanticKnowledge, error) {
		if !r.Known() {
			return k, fmt.Errorf("%q is not an answer; say confirmed, contradicted or declined", r)
		}
		k.Status = KnowledgeStatusFor(r)
		return k, nil
	})
}

// RetractKnown withdraws a settled durable judgement, leaving no active judgement.
//
// What becomes durable is a WITHDRAWAL rather than an absence, because an absence would be undone
// by the next thing that recognised the subject. Everything else about the record — its structure,
// its visits, its name, its other interpretations — is untouched: withdrawing an answer is a claim
// about the answer, not about what was seen.
func RetractKnown(store KnowledgeStore, subject string, kind HypothesisKind) error {
	return rewrite(store, subject, kind, func(k SemanticKnowledge) (SemanticKnowledge, error) {
		k.Status = KnowledgeRetracted
		return k, nil
	})
}

func rewrite(store KnowledgeStore, subject string, kind HypothesisKind,
	apply func(SemanticKnowledge) (SemanticKnowledge, error)) error {

	if store == nil {
		return fmt.Errorf("there is no memory to change")
	}
	s, ok := store.Subject(subject)
	if !ok {
		return fmt.Errorf("nothing remembered is called %s", subject)
	}
	k, ok := s.Find(kind)
	if !ok || !k.Settled() {
		return fmt.Errorf("you have not answered anything about %s here, so there is "+
			"nothing to change; a question that was never answered is answered, not revised",
			kind)
	}
	next, err := apply(k)
	if err != nil {
		return err
	}
	// The evidence digest is CARRIED, not recomputed. It is the basis for deciding later
	// whether the evidence has changed shape, and stamping it with today's would silently
	// suppress the reconsideration that a decline or a withdrawal is meant to leave open.
	next.Kind, next.Evidence = k.Kind, k.Evidence
	next.Support, next.Contradictions = k.Support, k.Contradictions
	// Answered is left where it was: `upsert` accumulates it, and a correction is a person
	// settling this once more rather than a fresh independent answer.
	next.Answered = 0
	// THE durable write, and it is the one the session path uses.
	return store.Remember(s.Application, s.Structure, next)
}
