package observe_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Asking, and what an answer is allowed to change.
//
// The risk this layer carries is not that it fails to ask. It is that it asks too often, asks
// about things it has no business being confident about, or treats an answer as settlement. Each
// test here is one of those.

// supported builds a hypothesis that has earned a question.
func supported(kind observe.HypothesisKind, terms ...observe.InterfaceTerm) observe.Hypothesis {
	return observe.Hypothesis{
		Kind:   kind,
		Status: observe.StatusSupported,
		Subject: observe.Subject{
			Kind: observe.SubjectState, Ref: "state_2",
			Fingerprint: observe.Fingerprint{
				Roles: map[string]int{"button": 5}, Terms: terms, TermsKnown: true,
				Members: 5, Recurrence: 3,
			},
		},
		Episodes: 3,
		Observed: "a recurring screen of grouped controls",
		Support: []observe.Evidence{
			{Source: observe.FromStructure, Statement: "five controls presented as a set"},
			{Source: observe.FromRecurrence, Statement: "recurred three separate times"},
		},
		Validation: "open and close it deliberately",
	}
}

func ask(t *testing.T, l *observe.ProposalLedger, hs ...observe.Hypothesis) []observe.Hypothesis {
	t.Helper()
	return l.Refresh(hs, 1, observe.DefaultProposalThresholds())
}

// ── A: a supported hypothesis earns exactly one question ──────────────────────

func TestASupportedHypothesisBecomesOneQuestion(t *testing.T) {
	var l observe.ProposalLedger
	ask(t, &l, supported(observe.PossibleSettingsLikeState, observe.TermSettings))

	open := l.Open()
	if len(open) != 1 {
		t.Fatalf("%d open question(s), want 1", len(open))
	}
	q := open[0]
	if q.Question == "" {
		t.Fatal("the proposal carries no question to put to anybody")
	}
	// The sentence a person reads must not contain implementation identifiers, and must
	// not imply Marco has learned to DO anything.
	for _, leak := range []string{"state_", "group_", "possible_", "shadow_"} {
		if strings.Contains(q.Question, leak) {
			t.Errorf("the question contains %q: %s", leak, q.Question)
		}
	}
	if !strings.Contains(strings.ToLower(q.Question), "?") {
		t.Errorf("the question is not phrased as one: %s", q.Question)
	}
	for _, overclaim := range []string{"I have learned", "I can now", "I know how"} {
		if strings.Contains(q.Question, overclaim) {
			t.Errorf("the question claims a capability: %s", q.Question)
		}
	}
}

// ── B and C: evidence that has not earned an interruption ─────────────────────

func TestATentativeHypothesisIsNotWorthInterruptingFor(t *testing.T) {
	h := supported(observe.PossibleMenuLikeState)
	h.Status = observe.StatusTentative

	var l observe.ProposalLedger
	ask(t, &l, h)
	if got := len(l.Open()); got != 0 {
		t.Errorf("%d question(s) from a tentative hypothesis; asking about evidence that "+
			"has not recurred teaches the user that Marco's questions are noise", got)
	}
}

func TestAContestedHypothesisIsNotPutAsThoughItWereEstablished(t *testing.T) {
	h := supported(observe.PossibleMenuLikeState)
	h.Status = observe.StatusContested
	h.Contradictions = []observe.Evidence{
		{Source: observe.FromNavigation, Statement: "the same change happened by itself twice"},
	}

	var l observe.ProposalLedger
	ask(t, &l, h)
	if got := len(l.Open()); got != 0 {
		t.Errorf("%d question(s) from a contested hypothesis. 'Is this a settings screen?' "+
			"presupposes the evidence agrees, and here it does not", got)
	}
}

// ── D: yes ────────────────────────────────────────────────────────────────────

// A confirmation strengthens the hypothesis it was asked about, and erases nothing.
func TestConfirmationValidatesTheHypothesisAndKeepsTheObservations(t *testing.T) {
	h := supported(observe.PossibleSettingsLikeState, observe.TermSettings)
	var l observe.ProposalLedger
	ask(t, &l, h)

	q := l.Open()[0]
	if _, ok := l.Respond(q.ID, observe.ResponseConfirmed, 5); !ok {
		t.Fatal("the answer was not accepted")
	}

	got := l.Annotate([]observe.Hypothesis{h})[0]
	if got.Status != observe.StatusValidated {
		t.Errorf("status %q after a confirmation, want validated", got.Status)
	}
	if got.UserValidation == nil || got.UserValidation.Response != observe.ResponseConfirmed {
		t.Fatalf("the confirmation is not recorded on the hypothesis: %+v", got.UserValidation)
	}
	// The observational support is still there, and the user is a distinct source.
	var fromUser, fromStructure int
	for _, e := range got.Support {
		switch e.Source {
		case observe.FromUser:
			fromUser++
		case observe.FromStructure:
			fromStructure++
		}
	}
	if fromUser != 1 {
		t.Errorf("user evidence appears %d times, want 1", fromUser)
	}
	if fromStructure != 1 {
		t.Error("the observational support was lost when the user confirmed")
	}
}

// A confirmation does NOT clear a contradiction.
//
// The one place a human answer could overwrite a measurement, and it must not. ADR-014's rule is
// contradiction-first; a user saying yes does not un-observe the evidence that disagreed, and a
// status that hid that would make the whole contradiction model decorative.
func TestConfirmationDoesNotClearAContradiction(t *testing.T) {
	h := supported(observe.PossibleMenuLikeState)
	h.Contradictions = []observe.Evidence{
		{Source: observe.FromRecurrence, Statement: "seen in only one visit"},
	}
	v := &observe.UserValidation{Response: observe.ResponseConfirmed}

	got := h.WithValidation(v)
	if got.Status == observe.StatusValidated {
		t.Error("a confirmation promoted a hypothesis that still carries a contradiction. " +
			"The user agreeing does not un-observe the evidence that disagreed")
	}
	if len(got.Contradictions) != 1 {
		t.Errorf("%d contradiction(s) after confirmation, want the original 1 kept",
			len(got.Contradictions))
	}
}

// ── E: no ─────────────────────────────────────────────────────────────────────

// A contradiction is recorded as such, and the supporting observations survive it.
//
// "I observed this repeatedly and you told me I was wrong" is the most useful thing Marco can
// report about its own model, and it is unsayable if the answer deletes the evidence.
func TestContradictionIsRecordedWithoutDeletingTheSupport(t *testing.T) {
	h := supported(observe.PossibleSettingsLikeState, observe.TermSettings)
	var l observe.ProposalLedger
	ask(t, &l, h)
	q := l.Open()[0]
	l.Respond(q.ID, observe.ResponseContradicted, 6)

	got := l.Annotate([]observe.Hypothesis{h})[0]
	if got.Status != observe.StatusContested {
		t.Errorf("status %q after the user said it was wrong, want contested", got.Status)
	}
	if len(got.Support) < 2 {
		t.Errorf("the observational support was deleted by the answer: %v", got.Support)
	}
	var userContra bool
	for _, c := range got.Contradictions {
		if c.Source == observe.FromUser {
			userContra = true
		}
	}
	if !userContra {
		t.Error("the user's disagreement is not in the contradictions")
	}
	if got.UserValidation.Response != observe.ResponseContradicted {
		t.Error("the validation record does not say what the user said")
	}
}

// ── F: not now ────────────────────────────────────────────────────────────────

// THE distinction this milestone turns on. A decline is not a denial.
//
// Collapsing them would let "I'm busy" quietly become "you are wrong" — incorrect, and the kind
// of error nobody would ever notice, because both produce a system that stops asking.
func TestDeclineSuppressesTheQuestionAndIsNotEvidence(t *testing.T) {
	h := supported(observe.PossibleMenuLikeState)
	var l observe.ProposalLedger
	ask(t, &l, h)
	q := l.Open()[0]
	l.Respond(q.ID, observe.ResponseDeclined, 4)

	if got := len(l.Open()); got != 0 {
		t.Errorf("%d question(s) still open after a decline", got)
	}

	got := l.Annotate([]observe.Hypothesis{h})[0]
	if got.Status != observe.StatusSupported {
		t.Errorf("status %q after a DECLINE, want the evidence untouched at supported. "+
			"A decision not to answer is not evidence about the screen", got.Status)
	}
	if len(got.Contradictions) != 0 {
		t.Errorf("a decline produced %d contradiction(s); it is not a no",
			len(got.Contradictions))
	}
	for _, e := range got.Support {
		if e.Source == observe.FromUser {
			t.Error("a decline was recorded as user support; it is not a yes either")
		}
	}
	if got.UserValidation == nil || got.UserValidation.Response != observe.ResponseDeclined {
		t.Error("the decline is not recorded at all; the proposal policy needs it")
	}
}

// ── G: no spam ────────────────────────────────────────────────────────────────

// Re-analysing the same evidence must not re-ask.
func TestRepeatedAnalysisDoesNotAskTwice(t *testing.T) {
	h := supported(observe.PossibleSettingsLikeState, observe.TermSettings)
	var l observe.ProposalLedger
	for i := 0; i < 20; i++ {
		ask(t, &l, h)
	}
	if got := len(l.Proposals); got != 1 {
		t.Fatalf("%d proposals after twenty analyses of the same evidence, want 1", got)
	}
	if got := l.Proposals[0].Asked; got != 1 {
		t.Errorf("the question was put %d times", got)
	}
}

// An answered question is never re-asked, and never re-answered.
func TestAnAnsweredQuestionIsNotReopened(t *testing.T) {
	h := supported(observe.PossibleMenuLikeState)
	var l observe.ProposalLedger
	ask(t, &l, h)
	q := l.Open()[0]
	l.Respond(q.ID, observe.ResponseConfirmed, 3)

	for i := 0; i < 10; i++ {
		ask(t, &l, h)
	}
	if got := len(l.Open()); got != 0 {
		t.Errorf("%d question(s) reopened after being answered", got)
	}
	if _, ok := l.Respond(q.ID, observe.ResponseContradicted, 9); ok {
		t.Error("an answered question accepted a second, different answer; the first answer " +
			"was the one given to the question that was actually asked")
	}
}

// ── H: material change ────────────────────────────────────────────────────────

// A declined question comes back only when the evidence changes SHAPE.
//
// Not when time passes, and not when the same thing is seen again. More of the same evidence is
// exactly the case where re-asking is nagging; a new KIND of evidence is a genuinely different
// question.
func TestADeclinedQuestionReturnsOnlyWhenTheEvidenceChangesShape(t *testing.T) {
	h := supported(observe.PossibleSettingsLikeState, observe.TermSettings)
	var l observe.ProposalLedger
	ask(t, &l, h)
	l.Respond(l.Open()[0].ID, observe.ResponseDeclined, 4)

	// More of the same: more episodes, same kinds of evidence. Must stay quiet.
	stronger := h
	stronger.Episodes = 40
	stronger.Subject.Fingerprint.Recurrence = 40
	for i := 0; i < 10; i++ {
		ask(t, &l, stronger)
	}
	if got := len(l.Open()); got != 0 {
		t.Fatalf("%d question(s) reopened by more of the same evidence. Seeing it again is "+
			"not learning something new, and re-asking on that basis is nagging", got)
	}

	// A new KIND of evidence: the interface started saying something it had not before.
	changed := h
	changed.Subject.Fingerprint.Terms = []observe.InterfaceTerm{
		observe.TermSettings, observe.TermControls,
	}
	ask(t, &l, changed)
	if got := len(l.Open()); got != 1 {
		t.Errorf("%d open question(s) after the evidence changed shape, want 1", got)
	}
	if got := l.Proposals[0].Asked; got != 2 {
		t.Errorf("the question records %d askings, want 2", got)
	}
}

// ── J: ephemeral ids ──────────────────────────────────────────────────────────

// Renumbering a state must not create a second question about the same thing.
//
// `state_4` today is `state_9` tomorrow and can change within one session. A question keyed on
// it would be asked again every time the tracker renumbered, which is both spam and a lie about
// there being two different things.
func TestRenumberingAStateDoesNotDuplicateTheQuestion(t *testing.T) {
	h := supported(observe.PossibleSettingsLikeState, observe.TermSettings)
	var l observe.ProposalLedger
	ask(t, &l, h)

	renumbered := h
	renumbered.Subject.Ref = "state_11"
	ask(t, &l, renumbered)

	if got := len(l.Proposals); got != 1 {
		t.Errorf("%d proposals after the same hypothesis was renumbered, want 1. Question "+
			"identity is keyed on a session-local counter", got)
	}
}

// Two genuinely different subjects are two questions.
//
// The control for the test above: identity must not be SO coarse that everything collapses into
// one question, which would pass the deduplication test by never asking about anything twice.
func TestDifferentSubjectsAreDifferentQuestions(t *testing.T) {
	a := supported(observe.PossibleSettingsLikeState, observe.TermSettings)
	b := supported(observe.PossibleSettingsLikeState, observe.TermSocial)
	b.Subject.Fingerprint.Members = 9
	b.Subject.Fingerprint.Roles = map[string]int{"button": 9}

	if observe.ProposalIdentity(a) == observe.ProposalIdentity(b) {
		t.Error("two structurally different screens produced the same question identity")
	}
}

// ── the interruption budget ───────────────────────────────────────────────────

// Marco asks one thing at a time.
//
// The cost of asking is not the cost of a dialog, it is the cost of breaking somebody's
// attention — and that is paid per interruption, not per question.
func TestOnlyOneQuestionIsOpenAtATime(t *testing.T) {
	var l observe.ProposalLedger
	ask(t, &l,
		supported(observe.PossibleSettingsLikeState, observe.TermSettings),
		supported(observe.PossibleMenuLikeState),
		supported(observe.PossibleTextEntryState),
	)
	if got := len(l.Open()); got != 1 {
		t.Errorf("%d questions open at once, want 1", got)
	}
}

// An unrecognised answer is refused rather than coerced.
func TestAnUnknownAnswerIsRefused(t *testing.T) {
	var l observe.ProposalLedger
	ask(t, &l, supported(observe.PossibleMenuLikeState))
	q := l.Open()[0]

	if _, ok := l.Respond(q.ID, observe.UserResponse("maybe"), 2); ok {
		t.Error("an answer outside the closed vocabulary was accepted. The only defaults " +
			"available are yes and no, and both would put words in the user's mouth")
	}
	if got := len(l.Open()); got != 1 {
		t.Error("the question was closed by an answer that was not understood")
	}
}
