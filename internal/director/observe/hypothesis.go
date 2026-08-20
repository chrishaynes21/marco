package observe

import (
	"fmt"
	"sort"
	"strings"
)

// Saying what the evidence suggests, in a way that survives being wrong.
//
// # What this layer is for
//
// Everything beneath it measures: this screen recurred four times, these five controls appear
// together, `pause` preceded this change three times out of three, the word SETTINGS was read
// in every inference of this screen. None of that is an interpretation, and none of it is
// useful to a person on its own.
//
// This is where measurements become a sentence somebody could act on — and the entire design
// problem is that such a sentence is much easier to produce than to justify. A generator that
// only accumulates support will eventually believe everything, so contradiction is modelled
// first-class, kept itemised rather than netted off into a score, and allowed to hold a
// hypothesis at `contested` forever.
//
// # Nothing here knows what application is running
//
// Not the executable, not the window title, not the game. The generator's whole input is
// ShadowTotals, which contains role counts, normalised geometry, recurrence counts,
// closed-vocabulary interface terms and navigation intents. Identical evidence from an unknown
// `game.exe` produces identical hypotheses, and TestHypothesesDoNotDependOnApplicationIdentity
// is what keeps that true.
//
// Application identity remains on the session for provenance — which session watched what — and
// is not reachable from here.
//
// # The bar
//
// "recurring navigable state; semantics unknown" is a better outcome than a confident
// "controller settings" that happens to be wrong. Geometry alone never names a screen; text
// alone never names a screen; a single episode never establishes anything. Where the evidence
// runs out, so does the claim.

// HypothesisKind is the closed vocabulary of generic interpretations.
//
// Every value begins "possible", as [[Insight]]'s does, because a discovery layer that asserts
// has stopped being falsifiable. The vocabulary is small on purpose: seven interpretations that
// can each be grounded in evidence Marco actually has, rather than an ontology of things it
// would be nice to recognise.
type HypothesisKind string

const (
	// PossibleChoiceGroup is several similar controls presented together as a set.
	//
	// Named for what the evidence shows — an arrangement — rather than "navigable choice
	// group", because navigation through it is separate evidence that may be absent. A
	// mouse-driven settings list is a choice group nobody was seen to navigate.
	PossibleChoiceGroup HypothesisKind = "possible_choice_group"
	// PossibleMenuLikeState is a recurring screen dominated by such a group.
	PossibleMenuLikeState HypothesisKind = "possible_menu_like_state"
	// PossibleSettingsLikeState is a menu-like screen whose text recurs from the
	// settings family of interface concepts.
	PossibleSettingsLikeState HypothesisKind = "possible_settings_like_state"
	// PossibleTextEntryState is a screen offering somewhere to type.
	PossibleTextEntryState HypothesisKind = "possible_text_entry_state"
	// PossibleTransitionAction is an intent that repeatedly preceded one change.
	PossibleTransitionAction HypothesisKind = "possible_transition_action"
	// PossibleReversiblePlace is a screen entered and left with navigation evidence in
	// both directions — the shape of somewhere you go and come back from.
	PossibleReversiblePlace HypothesisKind = "possible_reversible_place"
	// PossibleSelectionSequence is an ordered run, such as two moves and a confirm, that
	// preceded a change.
	PossibleSelectionSequence HypothesisKind = "possible_selection_sequence"
)

// SubjectKind says what a hypothesis is about.
type SubjectKind string

const (
	SubjectState      SubjectKind = "screen_state"
	SubjectTransition SubjectKind = "state_transition"
	SubjectGroup      SubjectKind = "structural_group"
	// SubjectTarget is a THING the person can act on — a button, a list item — identified
	// by what it is called, what sort of thing it is, and where it lives.
	//
	// Unlike every kind above it, a target is not a composition. A screen is recognised by
	// what it is made of; a target is recognised by its name, in a place. That is why the
	// matcher gives it its own branch rather than a tolerance — see CompareStructure.
	//
	// It carries no provider handle. See [[ADR-068-the-theater-is-the-durable-semantic-world]].
	SubjectTarget SubjectKind = "target"
)

// Subject is the thing a hypothesis concerns.
//
// # Ref is ephemeral and says so
//
// `state_3` is a counter in one session. The same screen is `state_1` in the next run and
// `state_7` in the one after, because states are minted in the order they are first seen. Ref
// exists so a reader can cross-reference this hypothesis against the state table printed above
// it in the SAME report, and for nothing else.
//
// Fingerprint is the part that could mean something later: composition, geometry and semantic
// terms, all of which describe the screen rather than name it. Cross-session identity is NOT
// solved here — no matcher exists — but the representation is arranged so that solving it later
// does not require rewriting every hypothesis that was already recorded, and so that nobody can
// accidentally persist `state_3` as if it meant something.
type Subject struct {
	Kind SubjectKind `json:"kind"`
	// Ref is the session-local identifier. Ephemeral: valid only within this session.
	Ref string `json:"session_ref"`
	// To is the destination reference, for a transition subject.
	To string `json:"session_ref_to,omitempty"`
	// Fingerprint describes the subject generically.
	Fingerprint Fingerprint `json:"fingerprint"`
}

// Fingerprint describes a subject without naming it or its session.
type Fingerprint struct {
	// Roles is the structural composition: how many of each detected role.
	Roles map[string]int `json:"roles,omitempty"`
	// Terms are the generic interface concepts that recurred here.
	Terms []InterfaceTerm `json:"terms,omitempty"`
	// TermsKnown says perception had text to classify for this subject at all.
	//
	// False means UNKNOWN, not "no concepts". A matcher that read an empty Terms slice as
	// an absence would treat an unavailable OCR pass as positive evidence that a screen
	// differs from a remembered one.
	TermsKnown bool `json:"terms_known,omitempty"`
	// Envelope is the normalised, window-relative bounding box, when the subject has one.
	Envelope *Region `json:"envelope,omitempty"`
	// Members is how many tracked structures make up the subject.
	Members int `json:"members,omitempty"`
	// Recurrence is how many separate episodes the subject was observed across.
	Recurrence int `json:"recurrence,omitempty"`
}

// EvidenceSource names an independent way of knowing.
//
// Independence is the property that matters: two facts from the same source are one
// observation seen twice, and a hypothesis supported by structure AND text AND navigation is
// materially stronger than one supported three times by geometry.
type EvidenceSource string

const (
	FromStructure  EvidenceSource = "structure"
	FromRecurrence EvidenceSource = "recurrence"
	FromText       EvidenceSource = "text"
	FromNavigation EvidenceSource = "navigation"
	FromAccessible EvidenceSource = "accessibility"
	// FromUser is the person Marco asked. Deliberate feedback, and a source of its own:
	// an answer is not an observation and must never be counted as one.
	FromUser EvidenceSource = "user"
)

// Evidence is one measured fact, with the denominator that makes it readable.
type Evidence struct {
	Source EvidenceSource `json:"source"`
	// Statement is Marco-authored prose. It never contains an application name, a window
	// title, a label, or anything read off the screen — only counts and closed vocabulary.
	Statement string `json:"statement"`
	// Support and Of are the fraction this rests on. Of is 0 when the fact is a count
	// rather than a ratio.
	Support int `json:"support,omitempty"`
	Of      int `json:"of,omitempty"`
}

// HypothesisStatus is how far the evidence goes.
//
// Four values and no float. A single confidence number is exactly the flattening that makes
// contradictions disappear — 0.62 tells a reader nothing about whether it is 0.62 because the
// evidence is thin or because half of it points the other way, and those call for opposite
// responses. The counts stay itemised in Support and Contradictions; this only says which
// regime the hypothesis is in.
//
// `validated` is the only one that involves a person, and it is still not "true": it means the
// observations agreed and the one person who was asked agreed with them.
type HypothesisStatus string

const (
	// StatusTentative is consistent evidence that has not recurred enough to lean on.
	StatusTentative HypothesisStatus = "tentative"
	// StatusSupported is recurrence across independent episodes with no material
	// contradiction, from more than one kind of evidence where more than one is available.
	StatusSupported HypothesisStatus = "supported"
	// StatusContested is real support AND real evidence against. It is a terminal state,
	// not a waypoint: more supporting observations do not clear a contradiction.
	StatusContested HypothesisStatus = "contested"
	// StatusValidated is supported evidence that the user was asked about and confirmed.
	//
	// The strongest thing this layer can say, and it is still not "true": it means the
	// observations agreed and the one person who was asked agreed with them. It requires
	// NO contradictions — a confirmation does not clear one, because a user saying yes does
	// not un-observe the times the evidence disagreed. See ADR-014's contradiction-first
	// rule, which this deliberately does not weaken.
	StatusValidated HypothesisStatus = "validated"
)

// Hypothesis is one cautious interpretation, with everything needed to doubt it.
type Hypothesis struct {
	Kind    HypothesisKind   `json:"kind"`
	Subject Subject          `json:"subject"`
	Status  HypothesisStatus `json:"status"`

	// Observed is the plain evidence sentence, so the finding and the guess read
	// separately.
	Observed string `json:"observed"`
	// Support and Contradictions are itemised, never netted against each other.
	Support        []Evidence `json:"support,omitempty"`
	Contradictions []Evidence `json:"contradictions,omitempty"`
	// Unattributed is changes with no navigation observed before them. Carried on the
	// hypothesis itself rather than left in the transition table, because a reader deciding
	// whether to believe "pause opens this" needs to see the times it opened by itself.
	Unattributed int `json:"unattributed,omitempty"`
	// Episodes is the independent-recurrence count this rests on.
	Episodes int `json:"episodes,omitempty"`
	// Validation is what a person could do to settle it. Required, for the same reason
	// [[Insight]] requires it: a hypothesis with no test is an assertion.
	Validation string `json:"validation"`
	// UserValidation is what the user said when Marco asked about this interpretation.
	//
	// Nil until a question has been answered. Held SEPARATELY from Support and
	// Contradictions so the observational record and the human record can be read apart:
	// "I observed this repeatedly and you confirmed it" and "I observed this repeatedly and
	// you told me I was wrong" are both things Marco must be able to say, and neither is
	// expressible if the answer is folded into the evidence it agreed or disagreed with.
	UserValidation *UserValidation `json:"user_validation,omitempty"`
}

// WithValidation returns a copy carrying the user's answer, folded into its evidence.
//
// A confirmation becomes SUPPORT with its own provenance and a contradiction becomes a
// CONTRADICTION, so both appear where a reader is already looking — but the observational
// entries either side of them are untouched. Nothing is rewritten and nothing is deleted; the
// answer is another voice in the record rather than the final word on it.
//
// A decline changes nothing at all. It is a decision not to answer, and treating it as evidence
// would let "I'm busy" become "you are wrong".
func (h Hypothesis) WithValidation(v *UserValidation) Hypothesis {
	if v == nil || v.Response == ResponseNone {
		return h
	}
	// Idempotent. Annotate runs over a Result more than once — once when the session ends
	// and again each time an answer arrives — and appending the same user evidence twice
	// would inflate the record with a second voice that does not exist.
	if h.UserValidation != nil && h.UserValidation.Response == v.Response {
		return h
	}
	h.UserValidation = v
	switch v.Response {
	case ResponseConfirmed:
		h.Support = append(append([]Evidence{}, h.Support...), Evidence{
			Source:    FromUser,
			Statement: "asked about this interpretation directly, the user confirmed it",
		})
		// Promotion happens only with a clean record. A confirmation does not clear a
		// contradiction: the user agreeing does not un-observe the evidence that
		// disagreed, and a status that hid that would be the one place in this system
		// where a human answer overwrote a measurement.
		if len(h.Contradictions) == 0 {
			h.Status = StatusValidated
		}
	case ResponseContradicted:
		h.Contradictions = append(append([]Evidence{}, h.Contradictions...), Evidence{
			Source: FromUser,
			Statement: "asked about this interpretation directly, the user said it was " +
				"wrong; the observations below stand and are reported unchanged",
		})
		// Contested, by the same contradiction-first rule everything else obeys. The
		// supporting observations remain listed: one answer does not delete them, it
		// disagrees with them, and the disagreement is the finding.
		h.Status = StatusContested
	}
	return h
}

// Sources reports the distinct kinds of evidence supporting this.
func (h Hypothesis) Sources() []EvidenceSource {
	seen := map[EvidenceSource]bool{}
	var out []EvidenceSource
	for _, e := range h.Support {
		if !seen[e.Source] {
			seen[e.Source] = true
			out = append(out, e.Source)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// HypothesisThresholds bound generation.
type HypothesisThresholds struct {
	// MinEpisodes is the independent recurrence below which nothing is `supported`.
	//
	// Two. One episode is an anecdote — the whole reason screen states are promoted on
	// recurrence rather than on first sighting is that a single observation cannot tell a
	// screen from a transition frame, and the same logic applies one layer up.
	MinEpisodes int
	// MinGroupMembers is how many controls make an arrangement worth calling a set.
	MinGroupMembers int
	// MinUniformity is how regular a group's spacing must be before its arrangement is
	// treated as deliberate rather than coincidental.
	MinUniformity float64
	// MinTermRatio is the share of a state's inferences a term must appear in.
	MinTermRatio float64
	// MinNavigationSupport is the share of a change's observations an intent must precede.
	MinNavigationSupport float64
	// MinNavigationObservations is how often a change must have been seen at all.
	MinNavigationObservations int
	// MaxHypotheses bounds the output.
	MaxHypotheses int
}

// DefaultHypothesisThresholds are the provisional defaults.
func DefaultHypothesisThresholds() HypothesisThresholds {
	return HypothesisThresholds{
		MinEpisodes:               2,
		MinGroupMembers:           3,
		MinUniformity:             0.50,
		MinTermRatio:              0.50,
		MinNavigationSupport:      0.60,
		MinNavigationObservations: 2,
		MaxHypotheses:             24,
	}
}

// settingsFamily is the terms that evidence a configuration screen.
//
// A FAMILY rather than a single word, because applications disagree about which one they put
// at the top: one says OPTIONS, another CONTROLS, a third SENSITIVITY. Requiring any one of
// them exactly would make the hypothesis a fact about house style.
var settingsFamily = map[InterfaceTerm]bool{
	TermSettings: true, TermControls: true, TermAudio: true, TermDisplay: true,
	TermSensitivity: true, TermLanguage: true, TermNotifications: true,
}

// entryFamily is the terms that evidence a screen you look someone or something up in.
var entryFamily = map[InterfaceTerm]bool{
	TermSearch: true, TermInvite: true, TermSocial: true, TermAccount: true,
}

// Hypotheses derives cautious interpretations from a session's discovery evidence.
//
// Deterministic: same totals, same hypotheses, same order. A fixture must replay exactly, and a
// change in output must mean a change in evidence rather than a change in map iteration.
func Hypotheses(t ShadowTotals, th HypothesisThresholds) []Hypothesis {
	var out []Hypothesis
	out = append(out, choiceGroups(t, th)...)
	out = append(out, menuLikeStates(t, th)...)
	out = append(out, textEntryStates(t, th)...)
	out = append(out, transitionActions(t, th)...)
	out = append(out, reversiblePlaces(t, th)...)
	out = append(out, selectionSequences(t, th)...)

	sort.SliceStable(out, func(i, j int) bool {
		if rank(out[i].Status) != rank(out[j].Status) {
			return rank(out[i].Status) > rank(out[j].Status)
		}
		if out[i].Episodes != out[j].Episodes {
			return out[i].Episodes > out[j].Episodes
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Subject.Ref < out[j].Subject.Ref
	})
	if th.MaxHypotheses > 0 && len(out) > th.MaxHypotheses {
		out = out[:th.MaxHypotheses]
	}
	return out
}

func rank(s HypothesisStatus) int {
	switch s {
	case StatusValidated:
		return 3
	case StatusSupported:
		return 2
	case StatusContested:
		return 1
	}
	return 0
}

// classify decides the regime from the itemised evidence.
//
// The rules, in order, and the order matters: a contradiction is checked FIRST and is not
// outvoted by any amount of support. That is the whole difference between a hypothesis
// generator and a confirmation-bias engine.
func classify(episodes int, support, contra []Evidence, th HypothesisThresholds) HypothesisStatus {
	if len(contra) > 0 {
		return StatusContested
	}
	if episodes < th.MinEpisodes {
		return StatusTentative
	}
	// More than one INDEPENDENT kind of evidence. Two structural facts about the same
	// rectangle are one observation described twice.
	sources := map[EvidenceSource]bool{}
	for _, e := range support {
		sources[e.Source] = true
	}
	if len(sources) < 2 {
		return StatusTentative
	}
	return StatusSupported
}

// ── structures ────────────────────────────────────────────────────────────────

// choiceGroups reports recurring sets of controls presented together.
func choiceGroups(t ShadowTotals, th HypothesisThresholds) []Hypothesis {
	var out []Hypothesis
	for _, g := range t.Groups {
		if len(g.Members) < th.MinGroupMembers {
			continue
		}
		st := stateByID(t, g.State)
		env := g.Envelope

		support := []Evidence{{
			Source:  FromStructure,
			Support: len(g.Members),
			Statement: fmt.Sprintf(
				"%d controls of the same kind sat together in one region, evenly spaced "+
					"(uniformity %.2f)", len(g.Members), g.Uniformity),
		}}
		if g.Episodes > 1 {
			support = append(support, Evidence{
				Source: FromRecurrence, Support: g.Episodes,
				Statement: fmt.Sprintf(
					"the arrangement reappeared in %d separate visits to this screen",
					g.Episodes),
			})
		}
		if g.Nameable == len(g.Members) {
			support = append(support, Evidence{
				Source:  FromStructure,
				Support: g.Nameable, Of: len(g.Members),
				Statement: "every member is a role whose name may be read, so this is " +
					"structure a person could be told about",
			})
		}

		var contra []Evidence
		if g.Uniformity < th.MinUniformity {
			contra = append(contra, Evidence{
				Source: FromStructure,
				Statement: fmt.Sprintf(
					"the spacing is irregular (uniformity %.2f); scattered controls that "+
						"happen to share a region look the same to a detector", g.Uniformity),
			})
		}
		if g.Episodes < th.MinEpisodes {
			contra = append(contra, Evidence{
				Source: FromRecurrence,
				Statement: "it was only ever seen in one visit, so nothing distinguishes a " +
					"structure from a one-off layout",
			})
		}

		out = append(out, Hypothesis{
			Kind: PossibleChoiceGroup,
			Subject: Subject{
				Kind: SubjectGroup, Ref: g.ID,
				Fingerprint: Fingerprint{
					Roles: copyRoles(g.Roles), Envelope: &env,
					Members: len(g.Members), Recurrence: g.Episodes,
					Terms: termsOf(st, th), TermsKnown: st.TermObservations > 0,
				},
			},
			Episodes: g.Episodes,
			Observed: fmt.Sprintf("%d controls recurred together at %s across %d visit(s)",
				len(g.Members), describeRegion(g.Envelope), g.Episodes),
			Support: support, Contradictions: contra,
			Status: classify(g.Episodes, support, contra, th),
			Validation: "interact with one member and watch the others; a set of choices " +
				"usually responds together, while unrelated controls that share a region do not",
		})
	}
	return out
}

// menuLikeStates reports recurring screens dominated by a choice group, and the settings-like
// specialisation when the text agrees.
// dominantGroup is the largest structural group on a screen, if it has one.
//
// One definition, used by every hypothesis that describes a screen, because it feeds the
// fingerprint — see stateFingerprint for why that has to be true.
func dominantGroup(t ShadowTotals, id ScreenStateID) (StructuralGroup, bool) {
	groups := groupsIn(t, id)
	if len(groups) == 0 {
		return StructuralGroup{}, false
	}
	best := groups[0]
	for _, g := range groups {
		if len(g.Members) > len(best.Members) {
			best = g
		}
	}
	return best, true
}

// stateFingerprint is what a SCREEN looks like, independent of which interpretation is
// describing it.
//
// # Why this is one function and not four
//
// It used to be four, and two of them left `Members` unset. That is not a cosmetic
// inconsistency: `Members` is part of durable identity, and `CompareStructure` allows it to
// differ by one — so a screen described by `possible_menu_like_state` (members 4) and the SAME
// screen described by `possible_reversible_place` (members 0) compared as DIFFERENT, and
// `Remember` stored them as two separate subjects.
//
// The consequences ran downstream of that quietly. One screen occupying two records means the
// question a person answered about one of them does not suppress the question about the other;
// it means recall returns whichever record the current hypothesis order happens to produce a
// matching signature for; and it means a relationship endpoint resolves or does not resolve
// depending on which interpretation named it. Found by tracing the relationship path rather than
// by any failing test, because every existing test compared a signature with itself.
//
// The rule, stated so it cannot drift again: a subject's identity is a property of the SUBJECT.
// Two hypotheses about one screen disagree about what it MEANS and must not disagree about what
// it IS.
// # Why a screen has no member count
//
// It used to borrow one from its dominant structural group, and that was a correctness defect
// rather than an approximation. A group is made of tracks persistent in EXACTLY ONE state, so
// while a session has only ever seen one place, the chrome that place shares with its neighbours
// is still persistent-in-one-state and counts; the moment a second place appears, that shared
// structure becomes ambient and belongs to neither, and the group collapses.
//
// Measured on one surface: the same place, same roles, same terms, reported 24 members observed
// alone and 12 observed alongside its neighbour. `MemberTolerance` is 1. So a demonstration
// standing on its own starting place could never establish it — memory answered `different` about
// a screen it had stored — and waiting did not converge, because only going somewhere ELSE
// changes the count.
//
// A screen is not its dominant group. The count was never a property of the place; it was a
// property of how much of the session had happened, which is the one thing durable identity may
// not depend on. Groups keep theirs, where it is intrinsic: a group IS its members.
func stateFingerprint(t ShadowTotals, st ScreenState, th HypothesisThresholds) Fingerprint {
	return Fingerprint{
		Roles: copyRoles(st.Roles), Terms: termsOf(st, th),
		TermsKnown: st.TermObservations > 0, Recurrence: st.Episodes,
	}
}

func menuLikeStates(t ShadowTotals, th HypothesisThresholds) []Hypothesis {
	var out []Hypothesis
	for _, st := range t.States {
		best, ok := dominantGroup(t, st.ID)
		if !ok {
			continue
		}
		if len(best.Members) < th.MinGroupMembers {
			continue
		}

		support := []Evidence{{
			Source: FromStructure, Support: len(best.Members),
			Statement: fmt.Sprintf("this screen is dominated by %d controls presented as a set",
				len(best.Members)),
		}}
		if st.Episodes > 1 {
			support = append(support, Evidence{
				Source: FromRecurrence, Support: st.Episodes,
				Statement: fmt.Sprintf("the screen recurred %d separate times, for %d "+
					"observations in total", st.Episodes, st.Inferences),
			})
		}

		var contra []Evidence
		if st.Episodes < th.MinEpisodes {
			contra = append(contra, Evidence{
				Source: FromRecurrence,
				Statement: "the screen was seen in only one visit; a transition frame and a " +
					"screen are indistinguishable until one of them comes back",
			})
		}

		terms := termsOf(st, th)
		out = append(out, Hypothesis{
			Kind: PossibleMenuLikeState,
			Subject: Subject{
				Kind: SubjectState, Ref: string(st.ID),
				Fingerprint: stateFingerprint(t, st, th),
			},
			Episodes: st.Episodes,
			Observed: fmt.Sprintf("a recurring screen of %d grouped controls, seen in %d "+
				"visit(s) over %d observation(s)", len(best.Members), st.Episodes, st.Inferences),
			Support: support, Contradictions: contra,
			Status: classify(st.Episodes, support, contra, th),
			Validation: "open and close this screen deliberately; a menu should appear and " +
				"disappear as a whole, while a layout of the underlying scene will not",
		})

		if h, ok := settingsLike(st, best, terms, stateFingerprint(t, st, th), th); ok {
			out = append(out, h)
		}
	}
	return out
}

// settingsLike is the specialisation, and it requires TEXT.
//
// This is the line the whole milestone turns on. Four aligned buttons are a choice group in any
// application; that they configure something is a claim geometry cannot support at all, and the
// only honest basis for it is that the interface said so — repeatedly, across separate visits,
// in words from a vocabulary that belongs to interfaces generally.
func settingsLike(st ScreenState, g StructuralGroup, terms []InterfaceTerm, fingerprint Fingerprint,
	th HypothesisThresholds) (Hypothesis, bool) {

	var family []InterfaceTerm
	for _, term := range terms {
		if settingsFamily[term] {
			family = append(family, term)
		}
	}
	if len(family) == 0 {
		return Hypothesis{}, false
	}

	support := []Evidence{{
		Source: FromStructure, Support: len(g.Members),
		Statement: fmt.Sprintf("%d controls presented as a set, which is the shape a "+
			"configuration screen takes", len(g.Members)),
	}}
	var contra []Evidence
	for _, term := range family {
		seen, episodes := st.Terms[term], st.TermEpisodes[term]
		support = append(support, Evidence{
			Source: FromText, Support: seen, Of: st.Inferences,
			Statement: fmt.Sprintf("the interface showed the concept %q in %d of this "+
				"screen's %d observations, across %d visit(s)",
				term, seen, st.Inferences, episodes),
		})
		// THE transient-overlay contradiction. A word that appeared once and never came
		// back with the screen is not a property of the screen — it is a notification, a
		// tooltip, or a misread, and it must not become durable semantic identity.
		if episodes < th.MinEpisodes && st.Episodes >= th.MinEpisodes {
			contra = append(contra, Evidence{
				Source: FromText, Support: episodes, Of: st.Episodes,
				Statement: fmt.Sprintf("the concept %q appeared in only %d of the %d visits "+
					"to this screen, so it may have come from something transient rather "+
					"than from the screen itself", term, episodes, st.Episodes),
			})
		}
	}
	if st.Episodes < th.MinEpisodes {
		contra = append(contra, Evidence{
			Source:    FromRecurrence,
			Statement: "the screen itself was seen in only one visit",
		})
	}

	return Hypothesis{
		Kind: PossibleSettingsLikeState,
		Subject: Subject{
			Kind: SubjectState, Ref: string(st.ID),
			Fingerprint: fingerprint,
		},
		Episodes: st.Episodes,
		Observed: fmt.Sprintf("a recurring screen of grouped controls whose text repeatedly "+
			"used configuration concepts (%s)", joinTerms(family)),
		Support: support, Contradictions: contra,
		Status: classify(st.Episodes, support, contra, th),
		Validation: "change one of the values here and check it persists after leaving and " +
			"returning; a settings screen keeps what it was told",
	}, true
}

// textEntryStates reports screens offering somewhere to type.
//
// Requires an ACCESSIBILITY role, never geometry. A bordered rectangle and a search box are the
// same picture, and a hypothesis that guessed from shape would be wrong on every heads-up
// display in every game.
func textEntryStates(t ShadowTotals, th HypothesisThresholds) []Hypothesis {
	var out []Hypothesis
	for _, st := range t.States {
		if st.EditableFields == 0 {
			continue
		}
		terms := termsOf(st, th)
		support := []Evidence{{
			Source: FromAccessible, Support: st.EditableFields,
			Statement: fmt.Sprintf("%d control(s) here reported themselves as text-editable",
				st.EditableFields),
		}}
		var lookup []InterfaceTerm
		for _, term := range terms {
			if entryFamily[term] {
				lookup = append(lookup, term)
			}
		}
		if len(lookup) > 0 {
			support = append(support, Evidence{
				Source: FromText, Support: len(lookup),
				Statement: fmt.Sprintf("the screen's text used look-up concepts (%s)",
					joinTerms(lookup)),
			})
		}
		var contra []Evidence
		if st.Episodes < th.MinEpisodes {
			contra = append(contra, Evidence{
				Source:    FromRecurrence,
				Statement: "the screen was seen in only one visit",
			})
		}
		out = append(out, Hypothesis{
			Kind: PossibleTextEntryState,
			Subject: Subject{
				Kind: SubjectState, Ref: string(st.ID),
				Fingerprint: stateFingerprint(t, st, th),
			},
			Episodes: st.Episodes,
			Observed: fmt.Sprintf("a screen with %d editable field(s), seen in %d visit(s)",
				st.EditableFields, st.Episodes),
			Support: support, Contradictions: contra,
			Status: classify(st.Episodes, support, contra, th),
			Validation: "focus the field and type; a real entry control shows what was typed. " +
				"Marco does not record what that is, and would take it as a parameter at the " +
				"moment a capability ran rather than learning it from you now",
		})
	}
	return out
}

// ── navigation ────────────────────────────────────────────────────────────────

// transitionActions reports intents that repeatedly preceded one change.
//
// The language is fixed at "preceded". Not "causes", not "opens", not "triggers" — a 3.5-second
// sampling interval cannot order two events inside it, the player may have pressed three keys,
// and the change may have been a cutscene ending. What can be said is what was seen, and how
// often it held.
func transitionActions(t ShadowTotals, th HypothesisThresholds) []Hypothesis {
	var out []Hypothesis
	for _, tr := range t.Transitions {
		if tr.Count < th.MinNavigationObservations {
			continue
		}
		intent, n := tr.Dominant()
		if intent == "" {
			continue
		}
		ratio := float64(n) / float64(tr.Count)
		if ratio < th.MinNavigationSupport {
			continue
		}

		statement := fmt.Sprintf("%q preceded this change in %d of its %d observations",
			intent, n, tr.Count)
		// A partial caveat, stated in the support itself rather than as a contradiction.
		// Some observations resting on a context judgement does not undo the ones that did
		// not, and contesting the whole edge for one of them would be too blunt.
		if tr.ConditionalOnly > 0 && tr.ConditionalOnly < tr.Attributed() {
			statement += fmt.Sprintf(" (%d of them from keys that are only navigation "+
				"while a set of choices is on screen)", tr.ConditionalOnly)
		}
		support := []Evidence{{
			Source: FromNavigation, Support: n, Of: tr.Count, Statement: statement,
		}}
		toState := stateByID(t, tr.To)
		if toState.Episodes > 1 {
			support = append(support, Evidence{
				Source: FromRecurrence, Support: toState.Episodes,
				Statement: fmt.Sprintf("the destination screen recurred %d separate times",
					toState.Episodes),
			})
		}

		var contra []Evidence
		// THE contradiction that keeps this honest: the same change happening with nothing
		// observed before it. Five transitions with pause before four is a much weaker
		// claim than four out of four, and a generator that reported only the four would
		// have deleted its own counter-example.
		if tr.Unattributed > 0 {
			contra = append(contra, Evidence{
				Source: FromNavigation, Support: tr.Unattributed, Of: tr.Count,
				Statement: fmt.Sprintf("the same change happened %d time(s) with no "+
					"navigation observed before it at all", tr.Unattributed),
			})
		}
		for _, other := range competing(tr, intent) {
			contra = append(contra, Evidence{
				Source: FromNavigation, Support: tr.Preceded[other], Of: tr.Count,
				Statement: fmt.Sprintf("%q also preceded this change, in %d of %d "+
					"observations", other, tr.Preceded[other], tr.Count),
			})
		}
		// THE conditional contradiction. When EVERY attributed observation rests on keys
		// that are only navigation in context, the edge has no unambiguous evidence at all
		// — the whole claim depends on Marco having judged the screen correctly, from an
		// observation up to one sampling interval earlier. That is real evidence and it may
		// not be promoted to `supported`.
		if tr.Attributed() > 0 && tr.ConditionalOnly == tr.Attributed() {
			contra = append(contra, Evidence{
				Source: FromNavigation, Support: tr.ConditionalOnly, Of: tr.Attributed(),
				Statement: "every one of these observations rests on a key that also means " +
					"movement during play, admitted because the screen looked like a set of " +
					"choices at the time; none of them is unambiguous navigation",
			})
		}

		out = append(out, Hypothesis{
			Kind: PossibleTransitionAction,
			Subject: Subject{
				Kind: SubjectTransition, Ref: string(tr.From), To: string(tr.To),
				Fingerprint: Fingerprint{
					Roles: copyRoles(toState.Roles), Terms: termsOf(toState, th),
					TermsKnown: toState.TermObservations > 0, Recurrence: tr.Count,
				},
			},
			Episodes:     tr.Count,
			Unattributed: tr.Unattributed,
			Observed: fmt.Sprintf("%q preceded this change in %d of %d observations",
				intent, n, tr.Count),
			Support: support, Contradictions: contra,
			Status: classify(tr.Count, support, contra, th),
			Validation: fmt.Sprintf("perform %q deliberately from the origin screen and "+
				"check the same change follows", intent),
		})
	}
	return out
}

// reversiblePlaces reports screens entered and left with navigation evidence both ways.
//
// The strongest thing this layer can currently say about a screen, and the one closest to being
// useful: somewhere the player goes deliberately and comes back from is a candidate destination
// for "take me there", in a way that a screen merely observed to exist is not.
func reversiblePlaces(t ShadowTotals, th HypothesisThresholds) []Hypothesis {
	var out []Hypothesis
	for _, in := range t.Transitions {
		if in.From == in.To {
			continue
		}
		back, ok := transitionBetween(t, in.To, in.From)
		if !ok {
			continue
		}
		// One direction per pair, chosen deterministically, or every place is reported
		// twice with the roles reversed.
		if in.From > in.To {
			continue
		}
		enter, enterN := in.Dominant()
		leave, leaveN := back.Dominant()
		if enter == "" || leave == "" {
			continue
		}
		if in.Count < th.MinNavigationObservations || back.Count < th.MinNavigationObservations {
			continue
		}

		st := stateByID(t, in.To)
		support := []Evidence{
			{
				Source: FromNavigation, Support: enterN, Of: in.Count,
				Statement: fmt.Sprintf("%q preceded arriving in %d of %d observations",
					enter, enterN, in.Count),
			},
			{
				Source: FromNavigation, Support: leaveN, Of: back.Count,
				Statement: fmt.Sprintf("%q preceded leaving in %d of %d observations",
					leave, leaveN, back.Count),
			},
		}
		if st.Episodes > 1 {
			support = append(support, Evidence{
				Source: FromRecurrence, Support: st.Episodes,
				Statement: fmt.Sprintf("the screen was visited %d separate times", st.Episodes),
			})
		}
		var contra []Evidence
		if in.Unattributed+back.Unattributed > 0 {
			contra = append(contra, Evidence{
				Source:  FromNavigation,
				Support: in.Unattributed + back.Unattributed, Of: in.Count + back.Count,
				Statement: fmt.Sprintf("%d of these changes happened with no navigation "+
					"observed before them", in.Unattributed+back.Unattributed),
			})
		}
		// "Somewhere you go and come back from" is the strongest claim this layer makes,
		// so it is held to the stricter standard: if either direction rests entirely on
		// context-admitted keys, the round trip is not established by unambiguous evidence.
		if conditionalOnly(in) || conditionalOnly(back) {
			contra = append(contra, Evidence{
				Source: FromNavigation,
				Statement: "at least one direction was observed only through keys that also " +
					"mean movement during play, admitted because the screen looked like a " +
					"set of choices at the time",
			})
		}

		out = append(out, Hypothesis{
			Kind: PossibleReversiblePlace,
			Subject: Subject{
				Kind: SubjectState, Ref: string(in.To),
				Fingerprint: stateFingerprint(t, st, th),
			},
			Episodes:     st.Episodes,
			Unattributed: in.Unattributed + back.Unattributed,
			Observed: fmt.Sprintf("the player went here after %q and left after %q, "+
				"%d time(s) in and %d time(s) out", enter, leave, in.Count, back.Count),
			Support: support, Contradictions: contra,
			Status: classify(st.Episodes, support, contra, th),
			Validation: fmt.Sprintf("perform %q and then %q deliberately and check the "+
				"screen appears and disappears to match", enter, leave),
		})
	}
	return out
}

// selectionSequences reports ordered runs that preceded a change.
func selectionSequences(t ShadowTotals, th HypothesisThresholds) []Hypothesis {
	var out []Hypothesis
	for _, tr := range t.Transitions {
		for _, seq := range tr.Sequences {
			if len(seq.Intents) < 2 || seq.Count < th.MinNavigationObservations {
				continue
			}
			st := stateByID(t, tr.To)
			support := []Evidence{{
				Source: FromNavigation, Support: seq.Count, Of: tr.Count,
				Statement: fmt.Sprintf("the ordered run %s preceded this change %d time(s) "+
					"out of %d", joinIntents(seq.Intents), seq.Count, tr.Count),
			}}
			var contra []Evidence
			if tr.Unattributed > 0 {
				contra = append(contra, Evidence{
					Source: FromNavigation, Support: tr.Unattributed, Of: tr.Count,
					Statement: fmt.Sprintf("the same change happened %d time(s) with no "+
						"navigation before it", tr.Unattributed),
				})
			}
			out = append(out, Hypothesis{
				Kind: PossibleSelectionSequence,
				Subject: Subject{
					Kind: SubjectTransition, Ref: string(tr.From), To: string(tr.To),
					Fingerprint: Fingerprint{
						Roles: copyRoles(st.Roles), Terms: termsOf(st, th),
						Recurrence: seq.Count,
					},
				},
				Episodes:     seq.Count,
				Unattributed: tr.Unattributed,
				Observed: fmt.Sprintf("%s preceded this change %d time(s)",
					joinIntents(seq.Intents), seq.Count),
				Support: support, Contradictions: contra,
				Status: classify(seq.Count, support, contra, th),
				Validation: "repeat the same run from the same screen and check it arrives " +
					"in the same place; a selection sequence should be repeatable",
			})
		}
	}
	return out
}

// ── helpers ───────────────────────────────────────────────────────────────────

func stateByID(t ShadowTotals, id ScreenStateID) ScreenState {
	for _, st := range t.States {
		if st.ID == id {
			return st
		}
	}
	return ScreenState{}
}

func groupsIn(t ShadowTotals, id ScreenStateID) []StructuralGroup {
	var out []StructuralGroup
	for _, g := range t.Groups {
		if g.State == id {
			out = append(out, g)
		}
	}
	return out
}

func transitionBetween(t ShadowTotals, from, to ScreenStateID) (ScreenTransition, bool) {
	for _, tr := range t.Transitions {
		if tr.From == from && tr.To == to {
			return tr, true
		}
	}
	return ScreenTransition{}, false
}

// termsOf returns the terms that appeared in enough of a state's observations to be about the
// state rather than about a moment.
// termsOf returns the terms that appeared in enough of a state's READINGS to be about the state
// rather than about a moment.
//
// The denominator is TermObservations — the inferences that actually had text to classify — and
// not the inference count. Scoped OCR runs on roughly one inference in six, so measuring against
// every inference caps a perfectly stable term at about 0.17 against a 0.50 threshold and makes
// the semantic discriminator unreachable in any real session.
//
// Zero readings means the answer is UNKNOWN, and the caller must distinguish that from a screen
// that was read and carried no concepts; see TermsKnown on the fingerprint.
func termsOf(st ScreenState, th HypothesisThresholds) []InterfaceTerm {
	if st.TermObservations == 0 {
		return nil
	}
	var out []InterfaceTerm
	for term, seen := range st.Terms {
		if float64(seen)/float64(st.TermObservations) >= th.MinTermRatio {
			out = append(out, term)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// conditionalOnly reports whether an edge's every attributed observation rested on keys that
// are only navigation in context.
func conditionalOnly(tr ScreenTransition) bool {
	return tr.Attributed() > 0 && tr.ConditionalOnly == tr.Attributed()
}

// competing is every correlated intent on an edge except the dominant one.
func competing(tr ScreenTransition, dominant NavIntent) []NavIntent {
	var out []NavIntent
	for intent := range tr.Preceded {
		if intent != dominant {
			out = append(out, intent)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if tr.Preceded[out[i]] != tr.Preceded[out[j]] {
			return tr.Preceded[out[i]] > tr.Preceded[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

func copyRoles(in map[string]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func joinTerms(in []InterfaceTerm) string {
	parts := make([]string, 0, len(in))
	for _, t := range in {
		parts = append(parts, string(t))
	}
	return strings.Join(parts, ", ")
}

func joinIntents(in []NavIntent) string {
	parts := make([]string, 0, len(in))
	for _, i := range in {
		parts = append(parts, string(i))
	}
	return strings.Join(parts, " → ")
}
