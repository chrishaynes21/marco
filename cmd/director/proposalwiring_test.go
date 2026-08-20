package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// The proposal loop from the service request down to the sentence a person reads.
//
// The runner-level tests prove a question is asked and an answer is recorded. These prove the
// two ends nobody else covers: that the question reaches the CLI at all, and that an answer
// arriving through the real protocol request lands on the right hypothesis — including for a
// session that has already ended, which is the ordinary case rather than an edge one.

// answeredRegistry builds a registry holding one finished session with an open question.
func answeredRegistry(t *testing.T) (*observationRegistry, observe.Proposal) {
	t.Helper()

	var totals observe.ShadowTotals
	for _, s := range semanticSession() {
		totals.Add(s)
	}
	hs := observe.Hypotheses(totals, observe.DefaultHypothesisThresholds())
	var ledger observe.ProposalLedger
	ledger.Refresh(hs, 8, observe.DefaultProposalThresholds())

	open := ledger.Open()
	if len(open) == 0 {
		t.Fatal("the fixture produced no question, so this test cannot proceed")
	}

	g := newObservationRegistry()
	g.finished = append(g.finished, observesession.Result{
		Session: observe.Session{
			ID: "observe_1", State: observe.Completed, Application: "testgame",
		},
		Hypotheses: ledger.Annotate(hs),
		Proposals:  ledger,
		Stats:      observesession.Stats{SamplesTaken: 8, Shadow: totals},
	})
	return g, open[0]
}

// THE protocol-level wiring test: an answer travels the real request path.
//
// Deleting the Answer branch from Runtime.Observation, or the registry's Answer method, must
// fail this.
func TestAnAnswerTravelsTheProductionRequestPath(t *testing.T) {
	g, q := answeredRegistry(t)
	rt := &Runtime{observations: g}

	out, err := rt.Observation(service.ObserveQuery{
		ID: "observe_1",
		Answer: &service.ObserveAnswer{
			ProposalID: string(q.ID), Response: string(observe.ResponseConfirmed),
		},
	})
	if err != nil {
		t.Fatalf("Observation: %v", err)
	}
	view, ok := out.(observationView)
	if !ok {
		t.Fatalf("the handler returned %T", out)
	}
	if view.Answered == nil {
		t.Fatal("the reply does not say which question the answer landed on")
	}
	if view.Answered.ID != q.ID {
		t.Errorf("the answer landed on %s, not the question that was asked (%s)",
			view.Answered.ID, q.ID)
	}
	if view.Answered.Response != observe.ResponseConfirmed {
		t.Errorf("recorded response %q", view.Answered.Response)
	}

	// And it is durable: a later read of the same session shows the validation.
	after, _ := g.Snapshot("observe_1")
	var validated bool
	for _, h := range after.Hypotheses {
		if h.UserValidation != nil &&
			h.UserValidation.Response == observe.ResponseConfirmed {
			validated = true
		}
	}
	if !validated {
		t.Error("the confirmation did not reach the hypotheses a later read returns; the " +
			"answer was recorded somewhere nobody looks")
	}
}

// An unrecognised answer is refused rather than coerced into a default.
func TestAnUnknownAnswerIsRefusedByTheProtocol(t *testing.T) {
	g, q := answeredRegistry(t)
	rt := &Runtime{observations: g}

	_, err := rt.Observation(service.ObserveQuery{
		ID:     "observe_1",
		Answer: &service.ObserveAnswer{ProposalID: string(q.ID), Response: "maybe"},
	})
	if err == nil {
		t.Fatal("an answer outside the closed vocabulary was accepted. The only defaults " +
			"available are yes and no, and both put words in the user's mouth")
	}
	if !strings.Contains(err.Error(), "confirmed") {
		t.Errorf("the refusal does not say what the valid answers are: %v", err)
	}
}

// An answer for a question nobody asked is refused.
func TestAnAnswerToAnUnknownQuestionIsRefused(t *testing.T) {
	g, _ := answeredRegistry(t)
	rt := &Runtime{observations: g}

	if _, err := rt.Observation(service.ObserveQuery{
		ID: "observe_1",
		Answer: &service.ObserveAnswer{
			ProposalID: "q_nothing", Response: string(observe.ResponseConfirmed),
		},
	}); err == nil {
		t.Error("an answer was accepted for a question that was never put")
	}
}

// ── the sentence a person reads ───────────────────────────────────────────────

// The rendered report must put the question, and say how to answer it.
//
// A question somebody cannot see how to answer is a statement, and a loop whose question is
// buried under four screens of rectangles never gets an answer at all.
func TestTheRenderedReportPutsTheQuestionAndSaysHowToAnswer(t *testing.T) {
	g, q := answeredRegistry(t)
	view, _ := g.Snapshot("observe_1")

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshalling the view: %v", err)
	}
	out := renderObservationSession(raw)

	if !strings.Contains(out, "MARCO HAS A QUESTION") {
		t.Fatal("the report does not put the question to the reader at all")
	}
	if !strings.Contains(out, q.Question) {
		t.Error("the question sentence is missing from the report")
	}
	if !strings.Contains(out, "director answer") {
		t.Error("the report does not say how to answer")
	}
	// The distinction that matters most has to be visible where the user decides.
	if !strings.Contains(out, `"not now" is not "no"`) {
		t.Error("the report does not tell the user that declining is not disagreeing")
	}
	// No implementation identifiers in the question itself.
	for _, leak := range []string{"state_", "possible_"} {
		if strings.Contains(q.Question, leak) {
			t.Errorf("the question contains %q", leak)
		}
	}
}

// After an answer, the report explains what happened to the evidence.
func TestTheReportExplainsWhatAnAnswerDidToTheEvidence(t *testing.T) {
	g, q := answeredRegistry(t)
	rt := &Runtime{observations: g}
	if _, err := rt.Observation(service.ObserveQuery{
		ID: "observe_1",
		Answer: &service.ObserveAnswer{
			ProposalID: string(q.ID), Response: string(observe.ResponseContradicted),
		},
	}); err != nil {
		t.Fatalf("Observation: %v", err)
	}

	view, _ := g.Snapshot("observe_1")
	raw, _ := json.Marshal(view)
	out := renderObservationSession(raw)

	if !strings.Contains(out, "previously asked") {
		t.Error("the report does not show that a question was already answered")
	}
	if !strings.Contains(out, "AGAINST") {
		t.Error("the user's disagreement is not shown as evidence against the hypothesis")
	}
	// The observations must still be reported: the disagreement is the finding, not their
	// deletion.
	if !strings.Contains(out, "structure") && !strings.Contains(out, "recurrence") {
		t.Error("the observational support vanished from the report after the user " +
			"disagreed; 'I observed this and you told me I was wrong' is the sentence this " +
			"whole design exists to be able to say")
	}
}

// The CLI's own words map onto the closed vocabulary, and keep the three answers apart.
func TestTheAnswerWordsMapOntoTheClosedVocabulary(t *testing.T) {
	cases := map[string]observe.UserResponse{
		"yes": observe.ResponseConfirmed, "y": observe.ResponseConfirmed,
		"no": observe.ResponseContradicted, "wrong": observe.ResponseContradicted,
		"not-now": observe.ResponseDeclined, "later": observe.ResponseDeclined,
		"skip": observe.ResponseDeclined,
	}
	for word, want := range cases {
		got, ok := responseFor(word)
		if !ok || got != want {
			t.Errorf("%q → %q (ok=%v), want %q", word, got, ok, want)
		}
	}
	if _, ok := responseFor("maybe"); ok {
		t.Error("an unrecognised word was mapped to an answer")
	}
	// The three must stay distinct all the way to the command line.
	if responseForOrEmpty("no") == responseForOrEmpty("not-now") {
		t.Error("'no' and 'not-now' map to the same response at the CLI. Being busy is not " +
			"disagreeing, and a user who says 'later' has not told Marco it is wrong")
	}
}

func responseForOrEmpty(s string) observe.UserResponse {
	r, _ := responseFor(s)
	return r
}

// ── K: replay ─────────────────────────────────────────────────────────────────

// The same evidence, replayed from a trace, produces the same questions.
//
// Question identity is derived from the hypothesis fingerprint, which is derived from the
// accumulated evidence — so if a trace reproduces the evidence it must reproduce the questions.
// If it did not, an offline analysis would ask about things the live session never raised, and
// an answer given to one would not attach to the other.
func TestReplayProducesTheSameQuestions(t *testing.T) {
	samples := semanticSession()

	var live observe.ShadowTotals
	for _, s := range samples {
		live.Add(s)
	}
	var liveLedger observe.ProposalLedger
	liveLedger.Refresh(observe.Hypotheses(live, observe.DefaultHypothesisThresholds()),
		len(samples), observe.DefaultProposalThresholds())

	path := filepath.Join(t.TempDir(), "trace.jsonl")
	tr := &shadowTrace{path: path}
	for _, s := range samples {
		sample := s
		tr.record(&sample, 1)
	}
	slots, err := loadTrace(path)
	if err != nil {
		t.Fatalf("loadTrace: %v", err)
	}
	var replayed observe.ShadowTotals
	for _, s := range slots {
		replayed.Add(sampleFromSlot(s))
	}
	var replayLedger observe.ProposalLedger
	replayLedger.Refresh(observe.Hypotheses(replayed, observe.DefaultHypothesisThresholds()),
		len(slots), observe.DefaultProposalThresholds())

	if len(liveLedger.Proposals) == 0 {
		t.Fatal("the live fixture asked nothing; nothing to compare")
	}
	if len(replayLedger.Proposals) != len(liveLedger.Proposals) {
		t.Fatalf("live asked %d question(s), replay %d",
			len(liveLedger.Proposals), len(replayLedger.Proposals))
	}
	for i := range liveLedger.Proposals {
		l, r := liveLedger.Proposals[i], replayLedger.Proposals[i]
		if l.ID != r.ID {
			t.Errorf("question %d: live id %s, replay %s. An answer given to one would not "+
				"attach to the other", i, l.ID, r.ID)
		}
		if l.Question != r.Question {
			t.Errorf("question %d differs in wording", i)
		}
		if l.Evidence != r.Evidence {
			t.Errorf("question %d: evidence digest differs, so a decline recorded live "+
				"would be re-asked on replay", i)
		}
	}
}

// A proposal carries no application identity and no free-form text.
//
// The privacy surface of this milestone is small but it is not nothing: a question is stored,
// rendered and potentially shared, and the answer vocabulary is closed precisely so that
// somebody cannot type a correction containing whatever they like.
func TestAProposalCarriesNoFreeFormTextOrApplicationIdentity(t *testing.T) {
	g, q := answeredRegistry(t)
	view, _ := g.Snapshot("observe_1")
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	body := string(raw)

	for _, leak := range []string{"testgame", ".exe", "rocket", "schedule"} {
		if strings.Contains(strings.ToLower(body), leak) {
			// Application appears legitimately as session provenance; the question and
			// the proposal must not carry it.
			if strings.Contains(strings.ToLower(mustJSON(t, q)), leak) {
				t.Errorf("the proposal itself carries %q", leak)
			}
		}
	}
	// The response vocabulary is closed: there is nowhere to put arbitrary text.
	if !observe.ResponseConfirmed.Known() || observe.UserResponse("anything").Known() {
		t.Error("the response vocabulary is not closed")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	return string(b)
}

// ONE QUESTION, HOWEVER MANY SESSIONS RAISED IT.
//
// # The live failure
//
// A rehearsal proposal's identity is its route, and a ledger dedupes on it — so one session asks
// once. But the ledger belongs to a session and the question belongs to a route, which outlives
// it, while the panel aggregates every session. A two-edge learn ran pass after pass waiting for
// an answer and the person read:
//
//	Questions open: 3
//	I've watched getting from Bluetooth to Mouse twice … Shall I have a go?
//	I've watched getting from Bluetooth to Mouse twice … Shall I have a go?
//	I've watched getting from Bluetooth to Mouse twice … Shall I have a go?
//
// The copies must EXIST — the yes that creates authority is applied by the newest runner, which
// looks the proposal up in its own ledger, so a question only an older session holds is visible
// and unanswerable. They must simply be read as what they are: one question.
func TestOneQuestionIsShownOnceHoweverManySessionsRaisedIt(t *testing.T) {
	ref := observe.RelationshipRef{From: "subj_a", To: "subj_b"}
	copyIn := func(id observe.SessionID) observesession.Result {
		r := ref
		return observesession.Result{
			Session: observe.Session{ID: id, Application: "settings"},
			Proposals: observe.ProposalLedger{Proposals: []observe.Proposal{{
				// The SAME identity: it is the same question about the same route.
				ID: "r_sameroute", Ask: observe.AskRehearse, Relationship: &r,
				Question: "Shall I have a go?", Status: observe.ProposalOpen,
			}}},
		}
	}
	g := newObservationRegistry()
	g.finished = []observesession.Result{
		copyIn("observe_1"), copyIn("observe_2"), copyIn("observe_3"),
	}
	rt := &Runtime{observations: g}

	asking := rt.asking()
	if len(asking) != 1 {
		t.Fatalf("%d questions shown for one question raised by three passes: %+v", len(asking),
			asking)
	}
	// The NEWEST copy, because that is the one an answer can reach.
	if asking[0].SessionID != "observe_3" {
		t.Errorf("the question offered belongs to %s; an answer must go to the newest copy, "+
			"since the grant is applied by the newest runner against its own ledger",
			asking[0].SessionID)
	}
	if got := rt.openQuestions(); got != 1 {
		t.Errorf("the count says %d beside a list of %d — a person reads them together",
			got, len(asking))
	}
}

// An answer settles every copy, because it is one question.
//
// Settling only the copy that happened to be found leaves the others open, so the panel re-offers
// a question the person has already answered.
func TestAnAnswerSettlesEveryCopyOfTheQuestion(t *testing.T) {
	ref := observe.RelationshipRef{From: "subj_a", To: "subj_b"}
	copyIn := func(id observe.SessionID) observesession.Result {
		r := ref
		return observesession.Result{
			Session: observe.Session{ID: id, Application: "settings"},
			Proposals: observe.ProposalLedger{Proposals: []observe.Proposal{{
				ID: "r_sameroute", Ask: observe.AskRehearse, Relationship: &r,
				Question: "Shall I have a go?", Status: observe.ProposalOpen,
			}}},
		}
	}
	g := newObservationRegistry()
	g.finished = []observesession.Result{
		copyIn("observe_1"), copyIn("observe_2"), copyIn("observe_3"),
	}

	if _, ok := g.Answer("observe_3", "r_sameroute", observe.ResponseConfirmed); !ok {
		t.Fatal("the answer landed nowhere")
	}
	for i := range g.finished {
		for _, p := range g.finished[i].Proposals.Proposals {
			if p.Response == observe.ResponseNone {
				t.Errorf("%s still holds the question open after it was answered; the panel "+
					"will keep asking it", g.finished[i].Session.ID)
			}
		}
	}
	rt := &Runtime{observations: g}
	if got := rt.openQuestions(); got != 0 {
		t.Errorf("%d question(s) still open after the only question was answered", got)
	}
}

// A YES REACHES THE RUNNER EVEN WHEN A NEWER SESSION HAS STARTED.
//
// # The live failure
//
// Answering is two acts: recording what was said, and acting on it. The second belongs to a
// runner, because the runner owns the store, the bounds and the one ephemeral grant — and it was
// only ever done for a question in the NEWEST runner's own ledger, on the reasoning that questions
// are asked at the end of a session, so nothing is running by the time anybody answers.
//
// A learn episode runs bounded passes back to back. The newest runner is routinely a session that
// started after the question was raised, whose ledger never held it. Measured on
// Home → Bluetooth → Mouse:
//
//	the proposal:  answered, confirmed, evidence 9f4a56779b4f2389
//	the judgement: eligible, digest 9f4a56779b4f2389, inputs 1
//	the authority: none
//	step 1 of 2: Home → Bluetooth — trying, forever
//
// The yes was not refused. It was dropped — and no refusal was recorded either, because the code
// that records refusals never ran. That is the worst shape this system can fail in: a person says
// yes, nothing happens, and nothing anywhere says why.
func TestAYesReachesTheRunnerEvenWhenANewerSessionHasStarted(t *testing.T) {
	g, q := answeredRegistry(t)
	// A store, so the RUNNER's half of an answer is observable at all: recording what was
	// said happens in the ledger, acting on it happens against memory.
	store, why := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	g.memory = store

	// A NEWER session, exactly as a learn pass produces: its own runner, its own empty
	// ledger, and it becomes `last`.
	newer := observesession.New(sessionClock, dryTarget{}, &sameSampler{script: dryHold("a", 2)},
		nil).WithMemory(store)
	g.last = newer
	ledger := newer.Proposals()
	if _, known := ledger.Respond(q.ID, observe.ResponseConfirmed, 0); known {
		t.Fatal("the newer runner already knows the question, so this proves nothing")
	}
	before := len(store.Subjects())

	if _, ok := g.Answer("observe_1", q.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer landed nowhere")
	}

	// The MEANING reached the runner. For this question kind that is a durable write; for a
	// rehearsal question it is the grant, by the same call on the same runner.
	if got := len(store.Subjects()); got == before {
		t.Error("the answer was recorded and inert. A newer session had started, so the " +
			"runner that could act on it had never heard of the question and dropped it " +
			"silently — no effect, and no refusal saying why.")
	}
	// AND UNDER THE QUESTION.S OWN APPLICATION, not the answering runner.s.
	//
	// The newer runner has not sampled yet, so it has no application at all. Taking the
	// application from it judged a live yes in an empty namespace, found no candidate for the
	// route, and created no authority — and the review then reported the timeout to the
	// person as "Alright — I won.t try it".
	//
	// Deleting the application argument at the registry.s ApplyAnswer call must fail this.
	for _, s := range store.Subjects() {
		if s.Application != "testgame" {
			t.Errorf("the answer landed under application %q; the question was asked about "+
				"testgame, and the runner that applied it had none of its own", s.Application)
		}
	}
}

// A NAMING QUESTION STAYS ABOUT ITS OWN SCREEN WHEN A NEWER SESSION STARTS.
//
// Naming already binds to the subject the QUESTION named rather than to whatever is in front, and
// this proves the other half of that: the binding does not depend on the session that raised it
// still being the newest one. By the time somebody types a name, a learn episode has usually begun
// another pass.
func TestNamingSurvivesANewerSessionBecomingCurrent(t *testing.T) {
	store, why := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	asked, err := store.EstablishPlace("settings", namedPlace(observe.TermAudio))
	if err != nil {
		t.Fatalf("establishing the screen the question is about: %v", err)
	}
	other, err := store.EstablishPlace("settings", namedPlace(observe.TermDisplay))
	if err != nil {
		t.Fatalf("establishing another screen: %v", err)
	}
	g := newObservationRegistry().withMemory(store)
	g.finished = []observesession.Result{{
		Session: observe.Session{ID: "observe_1", Application: "settings"},
	}}
	q, ok := g.ProposeScreenName("settings", asked)
	if !ok {
		t.Fatalf("the naming question was not raised: %+v", q)
	}

	// A newer session becomes current, with its own runner and its own empty ledger.
	g.last = observesession.New(sessionClock, dryTarget{},
		&sameSampler{script: dryHold("a", 2)}, nil).WithMemory(store)
	g.finished = append(g.finished, observesession.Result{
		Session: observe.Session{ID: "observe_2", Application: "settings"},
	})
	// And the NEWER session raises its own naming question, about a different screen.
	// Each ledger allows one open naming question, so this is the shape two of them take:
	// one per session, the older one still waiting for its answer.
	if _, ok := g.ProposeScreenName("settings", other); !ok {
		t.Fatal("the newer session raised no naming question, so nothing here distinguishes " +
			"answering the question you were asked from answering the newest one")
	}

	name, err := observe.UserSuppliedScreenName("Home")
	if err != nil {
		t.Fatalf("naming: %v", err)
	}
	if _, err := g.AnswerName("", q.ID, name); err != nil {
		t.Fatalf("answering the naming question after a newer session started: %v", err)
	}

	for _, s := range store.Subjects() {
		switch s.ID {
		case asked:
			if s.Called != "Home" {
				t.Errorf("the screen the question was about is called %q", s.Called)
			}
		case other:
			if s.Called != "" {
				t.Errorf("a screen nobody was asked about was named %q. The answer bound "+
					"to something current rather than to the question.", s.Called)
			}
		}
	}
}
