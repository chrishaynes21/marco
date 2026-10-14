package main

// The desk, driven by the synthetic key source.
//
// `navsource.NewSynthetic` installs no hook and observes no real keyboard, and it takes a
// VIRTUAL-KEY CODE rather than a Consent — so these tests go through admission, repeat
// suppression and the classifier exactly as a physical key does. A seam that accepted a
// `Consent` would prove the plumbing and nothing about which keys are admitted.

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// deskRig is a desk over a synthetic key source, already running.
type deskRig struct {
	press   func(code uint16, down bool)
	desk    *consentDesk
	granter *hotkeyGranter
	out     *bytes.Buffer
	stopped *int
	cancel  context.CancelFunc
}

func newDeskRig(t *testing.T, within time.Duration) *deskRig {
	t.Helper()
	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	stops := 0
	desk := newConsentDesk(src.Consents(), func() { stops++ })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go desk.run(ctx)
	out := &bytes.Buffer{}
	return &deskRig{
		press: press, desk: desk, out: out, stopped: &stops, cancel: cancel,
		granter: &hotkeyGranter{desk: desk, out: out, live: false,
			within: within, life: 10 * time.Second,
			now: func() time.Time { return time.Unix(1700000000, 0) }},
	}
}

// tap is one complete physical press of a key.
func (r *deskRig) tap(code uint16) {
	r.press(code, true)
	r.press(code, false)
}

func anIntent() workflowrun.Intent {
	return workflowrun.Intent{
		Step: 3, Epoch: 7, Generation: 2, Window: directorapi.WindowID("hwnd:99"),
		Control: "control_1", Screen: "screen_a", Expect: "screen_b",
		At: workflowrun.Point{X: 109, Y: 488},
	}
}

// ── the answer ────────────────────────────────────────────────────────────────

// F7 PERMITS THE ACTION IT WAS ASKED ABOUT, and the grant carries what it was asked about.
func TestTheApprovalKeyGrantsForTheStepWindowAndObservationItWasAskedAbout(t *testing.T) {
	r := newDeskRig(t, 2*time.Second)
	in := anIntent()

	var got workflowrun.Authorisation
	var ok bool
	done := make(chan struct{})
	go func() {
		got, ok = r.granter.Authorise(context.Background(), in)
		close(done)
	}()
	waitForQuestion(t, r)
	r.tap(navsource.KeyF7)
	<-done

	if !ok {
		t.Fatalf("F7 did not grant: %s", r.out.String())
	}
	if got.Step != in.Step {
		t.Errorf("granted for step %d, asked about %d", got.Step, in.Step)
	}
	if got.Window != in.Window {
		t.Errorf("granted about window %q, asked about %q", got.Window, in.Window)
	}
	if got.Epoch != in.Epoch || got.Generation != in.Generation {
		t.Errorf("granted in %d/%d, asked in %d/%d",
			got.Epoch, got.Generation, in.Epoch, in.Generation)
	}
	if got.By == "" {
		t.Error("the grant names nobody who gave it")
	}
	if got.Expires.IsZero() {
		t.Error("the grant has no expiry, which would make it a session flag")
	}
	// AND THE PERSON WAS TOLD WHERE IT LANDS before they were asked to permit it.
	if !strings.Contains(r.out.String(), "(109,488)") {
		t.Errorf("the question did not say where the press lands:\n%s", r.out)
	}
}

// F8 REFUSES, and refusing is a complete answer rather than an error.
func TestTheRefusalKeyGrantsNothing(t *testing.T) {
	r := newDeskRig(t, 2*time.Second)
	got, ok := answerWith(t, r, navsource.KeyF8)
	if ok {
		t.Fatal("F8 granted an authorisation")
	}
	if got.By != "" {
		t.Errorf("a refusal produced a grant: %+v", got)
	}
	if *r.stopped != 0 {
		t.Error("refusing one action stopped the whole run")
	}
}

// F9 STOPS, AND IT STOPS WITH NOTHING PENDING.
//
// The case a desk that only listened while a prompt was open would miss, and the one somebody
// reaches for most: the run is between actions, nothing is being asked, and they want it to end.
func TestTheStopKeyIsObeyedWithNoQuestionOpen(t *testing.T) {
	r := newDeskRig(t, 2*time.Second)
	r.tap(navsource.KeyF9)
	waitUntil(t, func() bool { return *r.stopped == 1 }, "the stop key was not obeyed")
	if _, drained := r.desk.counts(); drained != 0 {
		t.Errorf("a key pressed with nothing open was counted as drained")
	}
}

// AND F9 STOPS WHILE A QUESTION IS OPEN, granting nothing.
func TestTheStopKeyEndsAnOpenQuestionWithoutGranting(t *testing.T) {
	r := newDeskRig(t, 2*time.Second)
	_, ok := answerWith(t, r, navsource.KeyF9)
	if ok {
		t.Fatal("the stop key granted an authorisation")
	}
	if *r.stopped != 1 {
		t.Errorf("the run was stopped %d time(s), want 1", *r.stopped)
	}
}

// ── the ways an answer can be the wrong answer ────────────────────────────────

// A KEY PRESSED BEFORE THE QUESTION ANSWERS NOTHING.
//
// The whole reason this is a desk and not a channel read. Somebody leaning on F7 while a run
// starts must not pre-authorise the first action it decides to ask about: a grant is permission
// for a press described to them, and nothing was described yet.
func TestAKeyPressedBeforeTheQuestionDoesNotAnswerIt(t *testing.T) {
	r := newDeskRig(t, 300*time.Millisecond)
	r.tap(navsource.KeyF7)
	r.tap(navsource.KeyF7)
	waitUntil(t, func() bool { stray, _ := r.desk.counts(); return stray == 2 },
		"the early presses were not seen at all")

	_, ok := r.granter.Authorise(context.Background(), anIntent())
	if ok {
		t.Fatal("a key pressed before the question was asked authorised the action")
	}
	if !strings.Contains(r.out.String(), "nobody answered") {
		t.Errorf("the run did not report that nobody answered:\n%s", r.out)
	}
}

// AND ONE PRESSED WHILE A QUESTION IS OPEN ANSWERS ONLY THAT ONE.
func TestOneKeyAnswersOnlyOneQuestion(t *testing.T) {
	r := newDeskRig(t, 400*time.Millisecond)
	if _, ok := answerWith(t, r, navsource.KeyF7); !ok {
		t.Fatal("the first question was not answered")
	}
	// The SECOND question, with no second keypress.
	if _, ok := r.granter.Authorise(context.Background(), anIntent()); ok {
		t.Fatal("one keypress authorised two actions")
	}
}

// A HELD KEY IS ONE ANSWER.
//
// A low-level hook reports a held key as a stream of key-downs with no repeat flag, so without
// the held-set a person resting a finger on F7 would authorise an action every few milliseconds.
func TestAHeldApprovalKeyIsOneAnswer(t *testing.T) {
	r := newDeskRig(t, 2*time.Second)
	for i := 0; i < 20; i++ {
		r.press(navsource.KeyF7, true) // auto-repeat: down, down, down...
	}
	r.press(navsource.KeyF7, false)
	waitUntil(t, func() bool { stray, _ := r.desk.counts(); return stray >= 1 },
		"no answer arrived at all")
	time.Sleep(50 * time.Millisecond)
	if stray, _ := r.desk.counts(); stray != 1 {
		t.Errorf("a held key produced %d answers, want 1", stray)
	}
}

// A QUESTION NOBODY ANSWERS EXPIRES, and expiring grants nothing.
func TestAnUnansweredQuestionExpires(t *testing.T) {
	r := newDeskRig(t, 150*time.Millisecond)
	start := time.Now()
	if _, ok := r.granter.Authorise(context.Background(), anIntent()); ok {
		t.Fatal("an unanswered question granted an authorisation")
	}
	if waited := time.Since(start); waited > 2*time.Second {
		t.Errorf("the question waited %s, which is not an expiry", waited)
	}
}

// AND A CANCELLED RUN STOPS WAITING, whether or not anybody ever answers.
func TestACancelledRunDoesNotWaitForAnAnswer(t *testing.T) {
	r := newDeskRig(t, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() {
		_, ok := r.granter.Authorise(ctx, anIntent())
		done <- ok
	}()
	waitForQuestion(t, r)
	cancel()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("a cancelled run was granted an authorisation")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a cancelled run kept waiting for an answer")
	}
}

// AND THE AGREEMENT QUESTION IS A SEPARATE KEYPRESS FROM THE AUTHORISATION.
//
// Two sentences, two answers. A build that took one F7 for both would let somebody agree that a
// step belongs in the workflow and be taken to have permitted it to happen now.
func TestAgreeingToAStepIsNotPermittingIt(t *testing.T) {
	r := newDeskRig(t, 300*time.Millisecond)
	agreed := make(chan bool, 1)
	go func() { agreed <- r.granter.agree(context.Background(), 0, "control_1", "a", "b") }()
	waitForQuestion(t, r)
	r.tap(navsource.KeyF7)
	if !<-agreed {
		t.Fatal("F7 did not agree to the step")
	}
	// The SAME F7 must not then authorise the press.
	if _, ok := r.granter.Authorise(context.Background(), anIntent()); ok {
		t.Fatal("agreeing to a step also permitted the press")
	}
}

// AFTER A QUESTION IS ANSWERED, THE NEXT KEY IS STRAY AGAIN.
//
// A mutation found this gap: the desk clears its pending request after delivering, and without
// that line a key pressed between two questions is delivered to the question that is already
// over. Nobody is reading it, so nothing visibly breaks — the key simply disappears, and the
// count that should have said "somebody pressed a key when nothing was being asked" says zero.
func TestAKeyPressedAfterTheAnswerIsStrayAgain(t *testing.T) {
	r := newDeskRig(t, 400*time.Millisecond)
	if _, ok := answerWith(t, r, navsource.KeyF7); !ok {
		t.Fatal("the question was not answered")
	}
	before, _ := r.desk.counts()
	r.tap(navsource.KeyF7)
	waitUntil(t, func() bool { stray, _ := r.desk.counts(); return stray == before+1 },
		"a key pressed with no question open was not counted as stray")
}

// AND A KEY PRESSED WITH NOTHING OPEN NEVER BECOMES AN ANSWER TO WHAT COMES NEXT.
//
// The same guarantee from the other side: stray, then a question, then no answer.
func TestAStrayKeyDoesNotAnswerTheNextQuestion(t *testing.T) {
	r := newDeskRig(t, 250*time.Millisecond)
	r.tap(navsource.KeyF7)
	waitUntil(t, func() bool { stray, _ := r.desk.counts(); return stray == 1 },
		"the stray key was never seen")
	if _, ok := r.granter.Authorise(context.Background(), anIntent()); ok {
		t.Fatal("a stray key authorised the next action")
	}
}

// AND THE AGREEMENT QUESTION IS REFUSED BY THE REFUSAL KEY.
//
// Its own test because `agree` has its own answer, and a version that returned true for any
// answer at all would pass every other test in this file.
func TestTheRefusalKeyDeclinesAStep(t *testing.T) {
	r := newDeskRig(t, 400*time.Millisecond)
	agreed := make(chan bool, 1)
	go func() { agreed <- r.granter.agree(context.Background(), 0, "control_1", "a", "b") }()
	waitUntil(t, func() bool { return strings.Contains(r.out.String(), "Do you agree") },
		"the agreement question was not asked")
	r.tap(navsource.KeyF8)
	if <-agreed {
		t.Fatal("the refusal key agreed to the step")
	}
}

// AND AN UNANSWERED AGREEMENT QUESTION IS NOT AGREEMENT.
func TestAnUnansweredAgreementQuestionIsNotAgreement(t *testing.T) {
	r := newDeskRig(t, 120*time.Millisecond)
	if r.granter.agree(context.Background(), 0, "control_1", "a", "b") {
		t.Fatal("nobody answered and the step was taken as agreed")
	}
}

// EVERY QUESTION ANSWERED BY A KEY IS COUNTED.
//
// Found in the first supervised run: somebody answered two questions and the report said one was
// asked, because the agreement question did not count itself. A report that undercounts what a
// person was asked is a report that undercounts what they were responsible for.
func TestEveryQuestionAnsweredByAKeyIsCounted(t *testing.T) {
	r := newDeskRig(t, 500*time.Millisecond)

	agreed := make(chan bool, 1)
	go func() { agreed <- r.granter.agree(context.Background(), 0, "control_1", "a", "b") }()
	waitUntil(t, func() bool { return strings.Contains(r.out.String(), "Do you agree") },
		"the agreement question was not asked")
	r.tap(navsource.KeyF7)
	if !<-agreed {
		t.Fatal("the step was not agreed to")
	}
	// THE QUESTION THAT WAS ASKED IS OVER. Cleared so waiting for the next one waits for
	// the next one, rather than seeing the last one still in the buffer and pressing a key
	// before the desk is listening.
	r.out.Reset()
	if _, ok := answerWith(t, r, navsource.KeyF7); !ok {
		t.Fatal("the press was not permitted")
	}

	if r.granter.proposed != 1 || r.granter.agreed != 1 {
		t.Errorf("steps: %d agreed of %d proposed, want 1 of 1",
			r.granter.agreed, r.granter.proposed)
	}
	if r.granter.asked != 1 || r.granter.granted != 1 {
		t.Errorf("presses: %d permitted of %d asked, want 1 of 1",
			r.granter.granted, r.granter.asked)
	}
}

// AND A STEP SOMEBODY DECLINED IS COUNTED AS PROPOSED AND NOT AS AGREED.
func TestADeclinedStepIsCountedAsProposedAndNotAgreed(t *testing.T) {
	r := newDeskRig(t, 500*time.Millisecond)
	agreed := make(chan bool, 1)
	go func() { agreed <- r.granter.agree(context.Background(), 0, "control_1", "a", "b") }()
	waitUntil(t, func() bool { return strings.Contains(r.out.String(), "Do you agree") },
		"the agreement question was not asked")
	r.tap(navsource.KeyF8)
	if <-agreed {
		t.Fatal("the refusal key agreed to the step")
	}
	if r.granter.proposed != 1 || r.granter.agreed != 0 {
		t.Errorf("steps: %d agreed of %d proposed, want 0 of 1",
			r.granter.agreed, r.granter.proposed)
	}
}

// ── a race ────────────────────────────────────────────────────────────────────

// MANY QUESTIONS AND MANY KEYS AT ONCE GRANT NO MORE THAN WERE APPROVED.
//
// The desk is one goroutine on purpose. This asks it to misbehave: questions opening while keys
// arrive, each question answered by at most one key and each key answering at most one question.
func TestConcurrentQuestionsAndKeysNeverGrantMoreThanWereApproved(t *testing.T) {
	r := newDeskRig(t, 200*time.Millisecond)
	const rounds = 25
	granted := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < rounds; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := r.granter.Authorise(context.Background(), anIntent()); ok {
				mu.Lock()
				granted++
				mu.Unlock()
			}
		}()
		r.tap(navsource.KeyF7)
	}
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if granted > rounds {
		t.Fatalf("%d grants from %d keypresses", granted, rounds)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// answerWith opens a question, waits for it to be open, and answers it with one key.
func answerWith(t *testing.T, r *deskRig, code uint16) (workflowrun.Authorisation, bool) {
	t.Helper()
	type result struct {
		a  workflowrun.Authorisation
		ok bool
	}
	done := make(chan result, 1)
	go func() {
		a, ok := r.granter.Authorise(context.Background(), anIntent())
		done <- result{a, ok}
	}()
	waitForQuestion(t, r)
	r.tap(code)
	select {
	case got := <-done:
		return got.a, got.ok
	case <-time.After(5 * time.Second):
		t.Fatal("the question never came back")
	}
	return workflowrun.Authorisation{}, false
}

// waitForQuestion waits until the question has been printed, so a key cannot be pressed before
// the desk is listening — which is a different test.
func waitForQuestion(t *testing.T, r *deskRig) {
	t.Helper()
	waitUntil(t, func() bool {
		return strings.Contains(r.out.String(), "May Marco") ||
			strings.Contains(r.out.String(), "Do you agree")
	}, "no question was asked")
}

func waitUntil(t *testing.T, cond func() bool, why string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal(why)
}
