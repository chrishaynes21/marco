package main

// THE WORDS OF THE CAPTURE IN FRONT OF YOU, AND NOT OF THE ONE BEFORE IT.
//
// # The condition
//
// The boundary look was given text the way the observation loop is given it: `offer` the frame,
// `ready()` in the same breath, and take whatever OCR has FINISHED. For a loop that looks ten
// times a second and will look again, that is right — the queue is one deep, the newest frame
// wins, and a reading a few looks old is what the admission rule was built for.
//
// At an authorisation boundary it is the defect. The sequence:
//
//	the loop reads frame A           a reading of A is finished and waiting
//	a person is asked to authorise   nothing looks; the epoch cannot move
//	the panel becomes B              same shape, the approved point still inside something
//	the boundary captures B          offers it, and drains A
//	A's reading is admissible        the epoch never moved, so it is current by the rule
//	A's words agree with A           the text rule has nothing to refuse
//	the click is sent                to the panel that replaced the one somebody approved
//
// Two panels of one shape agree by arrangement — ADR-194 measured 0.721 against a threshold of
// 0.55 — so on a screen whose identity words have been separating, the words are the whole of
// the evidence, and the previous frame's words are the one answer that cannot be evidence about
// this one.
//
// # What these drive
//
// The production fold, `freshFrom`: the real observer, the real stabiliser, the real
// `readingEyes` over an engine this file decides the timing of, the real `freshOf`, the real
// `worldMoved`, and — for the input assertions — the real `workflowrun.Runner` with that
// boundary installed and a notebook where the mouse would be.
//
// The reader is gated rather than stubbed because what is being tested is WHEN the evidence
// arrives. A stub that always answers cannot describe a boundary that clicked before its
// answer came.

import (
	"context"
	"image"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/platform/marcorunner"
	"github.com/chaynes-simpleclouds/marco/internal/platform/recordhost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ── an engine whose timing this file decides ──────────────────────────────────

// gatedReader is OCR that answers when it is told to, and otherwise does not.
//
// Every call announces itself on `calls` and then waits. A test that serves the call decides
// what the frame says; a test that leaves it there decides that the words have not arrived yet,
// which is the state this whole file is about.
type gatedReader struct {
	calls chan *ocrAsk
}

// ocrAsk is one reading waiting for somebody to decide what it found.
type ocrAsk struct {
	reply chan []string
}

func (g *gatedReader) Recognize(ctx context.Context, in ocr.ImageInput) ([]ocr.Result, error) {
	ask := &ocrAsk{reply: make(chan []string, 1)}
	select {
	case g.calls <- ask:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	// AND FROM HERE IT IS DEAF, WHICH IS WHAT THE REAL ONE IS.
	//
	// `bridgehost.Invoke` writes a request line and reads the answer, and no context
	// can interrupt it once the line is written. So a boundary whose budget expires
	// still has a reading coming, with nobody left waiting for it — which is the state
	// the buffered channel in `readNow` exists for, and a cooperative fixture would
	// never produce it.
	return laidOut(<-ask.reply), nil
}

// laidOut turns words into engine results, one box each.
//
// The boxes have to be real: `recognise` drops a result with an empty rectangle, so a lazy
// fixture would produce a reading that said nothing and the tests below would pass for the
// wrong reason.
func laidOut(words []string) []ocr.Result {
	out := make([]ocr.Result, 0, len(words))
	for i, w := range words {
		out = append(out, ocr.Result{
			Text:       w,
			Bounds:     image.Rect(10, 10+i*20, 150, 25+i*20),
			Confidence: 0.9,
		})
	}
	return out
}

// serving answers every reading with these words, for as long as the test runs.
func serving(t *testing.T, g *gatedReader, words ...string) {
	t.Helper()
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		for {
			select {
			case <-done:
				return
			case ask := <-g.calls:
				select {
				case ask.reply <- words:
				case <-done:
					return
				}
			}
		}
	}()
}

// answerOne answers exactly one reading, and leaves every later one waiting.
func answerOne(t *testing.T, g *gatedReader, words ...string) {
	t.Helper()
	go func() {
		select {
		case ask := <-g.calls:
			ask.reply <- words
		case <-time.After(5 * time.Second):
		}
	}()
}

// readerOver starts a reader over a gated engine, with its worker running.
//
// The worker matters: the loop's pipeline has to be alive and have something in it, because the
// claim is that the boundary does not take what is in it.
func readerOver(t *testing.T, g *gatedReader) *readingEyes {
	t.Helper()
	e := &readingEyes{
		eng:  g,
		work: make(chan readingFrame, 1),
		done: make(chan observe.ScreenReading, 4),
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go e.run(ctx)
	return e
}

// quietGoroutines is the goroutine count once it has stopped moving.
//
// A bare count is not a baseline. Tests run one after another in one binary and the previous
// one's cleanup cancels contexts whose goroutines exit when they get round to it — so a count
// taken immediately can be two or three above the steady state, which is exactly enough to hide
// the one goroutine the assertion below is looking for.
func quietGoroutines(t *testing.T) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	was, still := goruntime.NumGoroutine(), 0
	for {
		time.Sleep(20 * time.Millisecond)
		now := goruntime.NumGoroutine()
		if now == was {
			if still++; still >= 3 {
				return now
			}
		} else {
			was, still = now, 0
		}
		if time.Now().After(deadline) {
			t.Fatalf("the goroutine count never settled (%d, then %d), so there "+
				"is no baseline to compare against", was, now)
		}
	}
}

// aFrame is a window's worth of pixels. What is in them is the engine's business, and this
// engine is told what it found.
func aFrame() image.Image { return image.NewRGBA(image.Rect(0, 0, 400, 300)) }

// theWordsOfTheScreen and theWordsOfAnother are three words each, which is the fewest that can
// be a claim — see observe.TextDisagreementWords. They share nothing, so one reading of the
// second is allowed to refuse to believe this is the first.
var (
	theWordsOfTheScreen = []string{"AUDIO", "GRAPHICS", "VIDEO"}
	theWordsOfAnother   = []string{"INVENTORY", "CRAFTING", "TRIBE"}
)

// askWithWords is `askBoundary` with a reader: the production fold, a budget, and the real
// decision.
func askWithWords(t *testing.T, o *fastobserve.Observer, eyes *readingEyes,
	planned workflowrun.Sighting, regions []observe.Region,
	within time.Duration) (bool, string, freshEvidence) {

	t.Helper()
	now := worldNow{Alive: true, State: "valid", Generation: 3,
		Front: observe.FrontTarget, Application: "thegame"}
	now.Fresh = freshFrom(context.Background(), o, eyes, boundaryLook(regions),
		aFrame(), theControl, within)
	now.Epoch, now.Screen = now.Fresh.Epoch, now.Fresh.Screen
	ok, why := worldMoved("thegame", workflowstore.NormaliseScope("thegame"), now, planned)
	return ok, why, now.Fresh
}

// settledWithWords settles a screen the way a run with --reading settles one: eight looks, each
// read, so the screen has a corroborated vocabulary of its own.
//
// That vocabulary is what makes words load-bearing at all — see freshEvidence.Needed. A screen
// nobody has read cannot be contradicted by a reading, so demanding one would refuse presses
// for evidence the rule could never have used.
func settledWithWords(t *testing.T, regions []observe.Region) (
	*fastobserve.Observer, workflowrun.Sighting) {

	t.Helper()
	o, planned := settledOn(t, regions,
		observe.TextOf(directorapi.SourceOCR, theWordsOfTheScreen))
	if !o.SaidBy(o.Screen()).Said() {
		t.Fatalf("%s has no vocabulary, so nothing in this file is about text",
			o.Screen())
	}
	return o, planned
}

// A PANEL WHOSE WORDS HAVE NOT ARRIVED IS NOT PRESSED, AND THE PREVIOUS FRAME'S WILL NOT DO.
//
// The arrangement is IDENTICAL to the standing screen's, which is the hard case: every term
// above the text one agrees, the stabiliser settles the look onto the screen the plan was made
// on, and the only thing that could say this is a different panel is what it says. A reading of
// the loop's last frame is finished and waiting in the queue the boundary used to drain, and it
// agrees with that screen — so a boundary that drains is a boundary that clears.
func TestAPanelWhoseWordsHaveNotArrivedIsNotPressed(t *testing.T) {
	g := &gatedReader{calls: make(chan *ocrAsk, 4)}
	eyes := readerOver(t, g)
	shape := aPanel(0.30, 12)
	o, planned := settledWithWords(t, shape)

	// ── THE LOOP'S LAST FRAME, READ AND WAITING ──
	//
	// Exactly the state the defect needed: a finished reading, stamped with the
	// epoch standing now, saying what the standing screen says.
	answerOne(t, g, theWordsOfTheScreen...)
	eyes.offer(readingFrame{img: aFrame(), look: 7,
		epoch: o.Epoch(), generation: 3})
	deadline := time.Now().Add(5 * time.Second)
	for len(eyes.done) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the loop's reading never finished, so the stale-evidence " +
				"state this test is about was never set up")
		}
		time.Sleep(time.Millisecond)
	}

	// AND NOW NOBODY ANSWERS FOR THE CAPTURE THE BOUNDARY TAKES.
	base := quietGoroutines(t)
	ok, why, e := askWithWords(t, o, eyes, planned, shape, 120*time.Millisecond)
	if ok {
		t.Fatalf("a press was cleared on a screen words separate, with no reading "+
			"of the look in front of it: %+v", e)
	}
	if !strings.Contains(why, "arrangement alone") {
		t.Errorf("the refusal is %q, which is not the missing-words one", why)
	}
	if !e.Needed {
		t.Error("the screen has a corroborated vocabulary and the boundary says " +
			"words are not needed on it, so this test would pass with the " +
			"whole term removed")
	}
	if e.Said {
		t.Error("no reading of that capture was ever produced and the boundary " +
			"reports that the stabiliser read one")
	}
	// AND THE STALE READING IS STILL SITTING THERE, which is the mechanism and not
	// the outcome: a boundary that drained it would have had words, agreed, and
	// cleared the press.
	if len(eyes.done) != 1 {
		t.Errorf("the boundary consumed the loop's finished reading (%d left in "+
			"the queue), which is a reading of the frame BEFORE the one it "+
			"just took", len(eyes.done))
	}
	if n := eyes.tooLate.Load(); n != 1 {
		t.Errorf("%d reading(s) were counted as too late, want 1: a budget that "+
			"expired is the evidence somebody reading the report needs", n)
	}

	// ── AND THE ABANDONED READING DOES NOT STAY ABANDONED ──
	//
	// Giving up on the answer is not giving up on the work: a bridge call cannot be
	// interrupted once it has been written, so the reading finishes after its budget
	// has expired, with nobody waiting for it. `readNow` answers on a BUFFERED
	// channel for exactly that reason — on an unbuffered one every expired boundary
	// would park a goroutine on the send for the rest of the run, and a run refusing
	// presses for want of words is the run that produces most of them.
	select {
	case ask := <-g.calls:
		ask.reply <- theWordsOfTheScreen
	default:
		t.Fatal("the boundary never asked for a reading, so there is nothing " +
			"abandoned to account for")
	}
	deadline = time.Now().Add(2 * time.Second)
	for goruntime.NumGoroutine() > base {
		if time.Now().After(deadline) {
			t.Errorf("the reading that finished after its budget left a "+
				"goroutine behind (%d against %d before the boundary), "+
				"so a long run accumulates one per refused press",
				goruntime.NumGoroutine(), base)
			break
		}
		time.Sleep(time.Millisecond)
	}
}

// AND NOTHING REACHES THE HOST. The real runner, with that boundary installed.
//
// The test above asserts the decision; this asserts the consequence, because a refusal the
// runner ignored would satisfy it. One authorised step, one same-shape panel underneath it,
// nobody answering for the pixels — and the notebook where the mouse would be has to be empty.
func TestAStepIsNotPressedWhileItsOwnWordsAreUnavailable(t *testing.T) {
	g := &gatedReader{calls: make(chan *ocrAsk, 4)}
	eyes := readerOver(t, g)
	shape := aPanel(0.49, 12)
	o, seen := settledWithWords(t, shape)

	book, s, r := aRunnerWatching(t, func(ctx context.Context, at workflowrun.Sighting,
		point observe.PointerAt) (bool, string) {

		now := worldNow{Alive: true, State: "valid", Generation: 3,
			Front: observe.FrontTarget, Application: "thegame"}
		now.Fresh = freshFrom(ctx, o, eyes, boundaryLook(shape), aFrame(), point,
			100*time.Millisecond)
		now.Epoch, now.Screen = now.Fresh.Epoch, now.Fresh.Screen
		return worldMoved("thegame", workflowstore.NormaliseScope("thegame"), now, seen)
	})
	got := r.Advance(context.Background(), s, theStepsLook())

	if lines := book.Lines(); len(lines) != 0 {
		t.Fatalf("%d call(s) reached the host with no reading of the capture "+
			"under the pointer:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if got.Outcome != workflowrun.OutcomeRefused {
		t.Errorf("the step ended %q (%s)", got.Outcome, got.Because)
	}
	if !strings.Contains(got.Because, "arrangement alone") {
		t.Errorf("the refusal does not come from the missing words: %s", got.Because)
	}
}

// AND WHEN THE WORDS DO ARRIVE, THEY REFUSE THE PANEL THEY DESCRIBE.
//
// The other half of the same condition, and the reason waiting is worth anything: the evidence
// the boundary waits for is evidence that can refuse. Same arrangement, three words that screen
// has never said, and the refusal comes from the stabiliser's own text rule rather than from
// anything this file measures.
func TestWordsThatArriveLateStillRefuseTheSameShape(t *testing.T) {
	g := &gatedReader{calls: make(chan *ocrAsk, 4)}
	eyes := readerOver(t, g)
	shape := aPanel(0.30, 12)
	o, planned := settledWithWords(t, shape)

	// A SLOW ANSWER THAT SAYS SOMETHING ELSE. Inside the budget, so the boundary has
	// it; after the capture, which is the ordinary case for OCR.
	serving(t, g, theWordsOfAnother...)
	was := o.Stability()
	ok, why, e := askWithWords(t, o, eyes, planned, shape, 2*time.Second)
	if ok {
		t.Fatalf("a panel of the same shape reading %v was cleared to press on a "+
			"plan made against %q (%+v)", theWordsOfAnother, planned.Screen, e)
	}
	if !e.Said {
		t.Fatal("the reading arrived inside its budget and the boundary reports " +
			"that no words reached the stabiliser")
	}
	now := o.Stability()
	if now.TextRefusedMerge == was.TextRefusedMerge &&
		now.TextRefusedReuse == was.TextRefusedReuse {
		t.Errorf("the refusal (%s) did not come from the stabiliser's text rule, "+
			"so something else in this test is doing the work", why)
	}
	if !strings.Contains(why, "did not agree") {
		t.Errorf("the refusal is %q; a look the text rule would not merge is a "+
			"look that did not agree", why)
	}
}

// AND AN UNCHANGED PANEL WHOSE WORDS ARRIVE IS PRESSED.
//
// The positive control, through the real runner: without it every refusal above is satisfied by
// a boundary that refuses everything, and the whole file would be asserting that Marco never
// clicks. Same screen, same shape, the words it has always said, answered inside the budget —
// and one move and one click arrive at the host.
func TestAnUnchangedPanelWhoseWordsArriveIsPressed(t *testing.T) {
	g := &gatedReader{calls: make(chan *ocrAsk, 4)}
	eyes := readerOver(t, g)
	// THE CONTROL THE STEP WAS RE-ACQUIRED AT IS UNDER THIS PANEL. The runner hands
	// the boundary the point it is about to press, and a panel with nothing there
	// would refuse on the target term and prove nothing about text.
	shape := aPanel(0.49, 12)
	o, seen := settledWithWords(t, shape)
	serving(t, g, theWordsOfTheScreen...)

	var said, needed bool
	book, s, r := aRunnerWatching(t, func(ctx context.Context, at workflowrun.Sighting,
		point observe.PointerAt) (bool, string) {

		now := worldNow{Alive: true, State: "valid", Generation: 3,
			Front: observe.FrontTarget, Application: "thegame"}
		now.Fresh = freshFrom(ctx, o, eyes, boundaryLook(shape), aFrame(), point,
			2*time.Second)
		now.Epoch, now.Screen = now.Fresh.Epoch, now.Fresh.Screen
		said, needed = now.Fresh.Said, now.Fresh.Needed
		return worldMoved("thegame", workflowstore.NormaliseScope("thegame"), now, seen)
	})
	got := r.Advance(context.Background(), s, theStepsLook())

	if got.Outcome != workflowrun.OutcomePressed {
		t.Fatalf("the step ended %q (%s), so the boundary refused a screen that "+
			"had not changed and said what it has always said",
			got.Outcome, got.Because)
	}
	if !needed || !said {
		t.Errorf("needed=%v said=%v: the control has to run with the rule in "+
			"force, or it is not a control for it", needed, said)
	}
	lines := book.Lines()
	if len(lines) != 2 {
		t.Fatalf("%d call(s) reached the host, want 2 (a move and a click):\n%s",
			len(lines), strings.Join(lines, "\n"))
	}
	if !strings.HasPrefix(lines[1], "OS's Click") {
		t.Errorf("the second host call is %q, which is not a click", lines[1])
	}
	if n := eyes.tooLate.Load(); n != 0 {
		t.Errorf("%d reading(s) were counted as too late on a run where every "+
			"one was answered", n)
	}
	// AND THE BOUNDARY WAS ASKED TWICE, which is what one authorised click costs:
	// the runner confirms before the hover and again after the dwell, because the
	// dwell is a gap like any other. Counted, so the report can say what the rule
	// is spending.
	if n := eyes.atBoundary.Load(); n != 2 {
		t.Errorf("%d boundary reading(s) were counted, want 2 — one per "+
			"confirmation before the press", n)
	}
}

// AND A SCREEN NOBODY HAS READ IS STILL PRESSED ON ITS PIXELS.
//
// The limit of the rule, and the reason it is a rule about the screen rather than about the
// run. Words may do exactly one thing — refuse a merge — and a screen with no corroborated
// vocabulary cannot be contradicted by a reading at all. So on such a screen there is nothing
// to withhold: demanding words would refuse every press for evidence the stabiliser could
// never have used, and a reader that is slow or silent would become a reason not to act.
//
// Which is also why nothing is asked for here. The reader is running and would not answer, the
// budget is a second and a half twice over, and this test finishes immediately.
func TestAScreenNobodyHasReadIsStillPressedOnItsPixels(t *testing.T) {
	g := &gatedReader{calls: make(chan *ocrAsk, 4)}
	eyes := readerOver(t, g)
	shape := aPanel(0.49, 12)
	// SETTLED ON PIXELS ALONE, which is every run without --reading and the opening
	// looks of every run with it.
	o, seen := settledOn(t, shape, observe.ScreenText{})
	if o.SaidBy(o.Screen()).Said() {
		t.Fatalf("%s has a vocabulary, so this is not the unread case", o.Screen())
	}

	book, s, r := aRunnerWatching(t, func(ctx context.Context, at workflowrun.Sighting,
		point observe.PointerAt) (bool, string) {

		now := worldNow{Alive: true, State: "valid", Generation: 3,
			Front: observe.FrontTarget, Application: "thegame"}
		now.Fresh = freshFrom(ctx, o, eyes, boundaryLook(shape), aFrame(), point,
			boundaryTextWait)
		now.Epoch, now.Screen = now.Fresh.Epoch, now.Fresh.Screen
		if now.Fresh.Needed {
			t.Error("words are reported load-bearing for a screen nobody has " +
				"read, so a run with a slow reader cannot press anything")
		}
		return worldMoved("thegame", workflowstore.NormaliseScope("thegame"), now, seen)
	})
	if got := r.Advance(context.Background(), s, theStepsLook()); got.Outcome !=
		workflowrun.OutcomePressed {
		t.Fatalf("the step ended %q (%s) on an unchanged screen with no "+
			"vocabulary", got.Outcome, got.Because)
	}
	if n := len(book.Lines()); n != 2 {
		t.Errorf("%d call(s) reached the host, want 2", n)
	}
	// AND NOTHING WAS ASKED OF OCR. A boundary that waited for words it could not
	// have used would have spent its whole budget here, twice.
	if n := eyes.atBoundary.Load(); n != 0 {
		t.Errorf("%d reading(s) were asked for on a screen whose words cannot "+
			"refuse anything", n)
	}
}

// AND A BOUNDARY WAITING FOR WORDS IS CANCELLABLE.
//
// A bounded wait that a Ctrl-C cannot interrupt is a new place for a run to hang, and this one
// sits between a person authorising an action and the action. So the budget is a ceiling and the
// context is the floor: the run's own cancellation ends the wait at once, and the press is
// refused for the reason it should be — nobody answered for these pixels.
func TestABoundaryWaitingForWordsIsCancellable(t *testing.T) {
	g := &gatedReader{calls: make(chan *ocrAsk, 4)}
	eyes := readerOver(t, g)
	shape := aPanel(0.30, 12)
	o, planned := settledWithWords(t, shape)

	// NOBODY ANSWERING, A BUDGET LONGER THAN THIS TEST SHOULD TAKE, and a
	// cancellation while it waits.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	began := time.Now()
	now := worldNow{Alive: true, State: "valid", Generation: 3,
		Front: observe.FrontTarget, Application: "thegame"}
	now.Fresh = freshFrom(ctx, o, eyes, boundaryLook(shape), aFrame(), theControl,
		30*time.Second)
	now.Epoch, now.Screen = now.Fresh.Epoch, now.Fresh.Screen
	took := time.Since(began)

	if took > 5*time.Second {
		t.Errorf("the boundary waited %s after the run was cancelled, so a "+
			"cancellation does not reach the wait", took)
	}
	ok, why := worldMoved("thegame", workflowstore.NormaliseScope("thegame"),
		now, planned)
	if ok {
		t.Fatal("a press was cleared on a cancelled boundary with no words")
	}
	if !strings.Contains(why, "arrangement alone") {
		t.Errorf("the refusal is %q, which is not the missing-words one", why)
	}
}

// ── the runner, assembled once ───────────────────────────────────────────────

// aRunnerWatching is the real coordinator with a notebook where the mouse would be, one step
// authorised, and the boundary under test installed as its confirmation.
func aRunnerWatching(t *testing.T, confirm func(context.Context, workflowrun.Sighting,
	observe.PointerAt) (bool, string)) (*recordhost.Host, *workflowstore.SessionBinding,
	*workflowrun.Runner) {

	t.Helper()
	art := oneClick(t)
	book := recordhost.New()
	var marco directorapi.MarcoRunner = marcorunner.New(map[string]runtime.Host{
		"OS": book,
	})
	s := workflowstore.NewSessionBinding()
	if err := s.ConfirmStep(0, "SIMULATED fixture"); err != nil {
		t.Fatal(err)
	}
	grant := &permits{epoch: 5, generation: 1}
	r := workflowrun.New(art, nil, answers{"now_1": "screen_a"}, grant).
		WithExecutor(marcoexec.New(marco), false).
		WithConfirm(confirm)
	return book, s, r
}

// theStepsLook is the observation the step is advanced on, with its control re-acquired.
func theStepsLook() workflowrun.Sighting {
	return workflowrun.Sighting{
		Screen: "now_1", Epoch: 5, Generation: 1,
		Window: "hwnd:1", Front: true, At: time.Now(),
		Bounds: directorapi.Rect{X: 100, Y: 50, Width: 1000, Height: 500},
		Outcomes: []observe.Reacquisition{{
			Target: "control_1", Screen: "now_1",
			Result: observe.ReacquireMatched, Controls: 1,
			Where:   observe.Region{X: 0.4, Y: 0.5, Width: 0.1, Height: 0.04},
			Because: "a fixture outcome",
		}},
	}
}
