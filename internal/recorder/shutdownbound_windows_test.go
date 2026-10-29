//go:build windows

package recorder

// A SHUTDOWN THAT CANNOT BE COMPLETED STILL RETURNS, AND SAYS WHAT IT LEFT BEHIND.
//
// # What was seen
//
// `plugins/studio`'s own suite, under the load of its headless-browser tests, intermittently left
// a played routine unfinished: the child had run the program — its last transcript line was the
// routine's own log — and then said nothing for thirty seconds, with no outcome word and the
// window's one run slot still claimed. Roughly one run in six; in isolation it finishes in half a
// second.
//
// The pump wake in `Stop` has been bounded since the 600-second timeouts that found it. The two
// channel receives EITHER SIDE of it were not, and the second of them waits for the app-switch
// poller — whose one blocking call is a foreground-window read, which blocks when the window's
// owning thread is not pumping. A desktop full of browser windows coming and going is exactly
// that desktop.
//
// # What this test establishes, and what it does not
//
// That neither wait can wedge a caller: a recording whose hook thread never reported starting,
// and one whose poller never comes back, both let `Stop` return. It does NOT establish why the
// poller was late — nothing here does, and the message exists so the next occurrence says which
// wait expired rather than needing the paragraph above again.
//
// No hooks are installed. The channels are what `Stop` waits on, so a recorder with those
// channels and nothing else is the whole of what the bounds are about — and installing real
// low-level hooks in a test is not a thing this package may do.
//
// See [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"
)

// aCleanPackage takes the capture state this package keeps, and puts it back afterwards.
//
// All of it is global — the session number, the buffer, the live channel, the wedged flag and the
// foreground the hooks tag presses with — because the hook callbacks are C callbacks and can
// reach nothing else. Every test here replaces a recording, so the order they run in would
// otherwise be part of what they assert.
func aCleanPackage(t *testing.T) {
	t.Helper()
	wasWedged, wasSession := recWedged.Load(), recSession.Load()
	wasFore := foreApp.Load()
	// AND THE CONFINEMENT, which is capture state like every other line here: a test that
	// starts a confined recording leaves `bound` holding a key and `confined` true, and
	// the next test to drive a hook body directly would then have its click gated by a
	// window chosen in somebody else's test. That is not hypothetical — it is how
	// `TestAStatedCoordinateModeOverridesInheritedAnchors` failed first.
	wasConfined, wasTarget := confined.Load(), targetHWnd.Load()
	wasTargetPID, wasGone := targetPID.Load(), gone.Load()
	wasApp := targetApp.Load()
	recMu.Lock()
	wasBuf, wasPub, wasBound := recBuf, recPub, bound
	recMu.Unlock()
	t.Cleanup(func() {
		recWedged.Store(wasWedged)
		recSession.Store(wasSession)
		confined.Store(wasConfined)
		targetHWnd.Store(wasTarget)
		targetPID.Store(wasTargetPID)
		gone.Store(wasGone)
		if wasApp != nil {
			targetApp.Store(wasApp)
		}
		recMu.Lock()
		recBuf, recPub, bound = wasBuf, wasPub, wasBound
		recMu.Unlock()
		// NOT IF THERE WAS NO SAMPLE YET: an atomic.Value refuses a nil, and
		// "nothing has been sampled" is the state a fresh process is in.
		if wasFore != nil {
			foreApp.Store(wasFore)
		}
	})
	recWedged.Store(false)
	confined.Store(false)
	gone.Store(false)
	targetHWnd.Store(0)
	targetPID.Store(0)
	targetApp.Store("")
	recMu.Lock()
	bound = nil
	recMu.Unlock()
}

// aRecordingThatWillNotFinishShuttingDown is a recorder waiting on channels nobody closes.
//
// `ready` is given or withheld per case; `stopped` and `pollStopped` are never closed, which is
// the state a thread stuck in a syscall leaves them in.
func aRecordingThatWillNotFinishShuttingDown(readyNow bool) *winRecorder {
	r := &winRecorder{
		done:        make(chan struct{}),
		stopped:     make(chan struct{}),
		ready:       make(chan struct{}),
		pollStopped: make(chan struct{}),
	}
	if readyNow {
		close(r.ready)
	}
	return r
}

// A HOOK THREAD THAT NEVER REPORTED STARTING DOES NOT WEDGE THE SHUTDOWN.
func TestAHookThreadThatNeverStartedDoesNotWedgeTheShutdown(t *testing.T) {
	// AND IT PUTS THE PACKAGE BACK, because abandoning a recording sets `recWedged`
	// and nothing in this process may record afterwards. Left set, it made
	// `TestAnUnconsumedArmDoesNotSurviveIntoTheNextRecording` SKIP on the refusal from
	// `Start` — a real test quietly not running, three files away, for hours.
	aCleanPackage(t)
	r := aRecordingThatWillNotFinishShuttingDown(false)
	done := make(chan struct{})
	go func() { defer close(done); r.Stop() }()

	// GENEROUSLY MORE THAN THE BOUND, and less than the thirty seconds that was the
	// symptom. A test that waited the bound exactly would fail on a slow machine for a
	// reason that is not the product's.
	select {
	case <-done:
	case <-time.After(hookShutdownBound + 20*time.Second):
		t.Fatal("Stop never returned for a recording whose hook thread never " +
			"reported that it had started, so a completed run reports nothing " +
			"at all and whoever spawned it waits for ever")
	}
	// AND A SECOND STOP IS A NO-OP rather than a second wait. `abandon` clears `done`,
	// which is what makes that true, and a caller that retries is a caller that would
	// otherwise pay the bound twice.
	again := make(chan struct{})
	go func() { defer close(again); r.Stop() }()
	select {
	case <-again:
	case <-time.After(2 * time.Second):
		t.Error("a second Stop waited again")
	}
}

// AND NEITHER DOES A POLLER THAT NEVER COMES BACK.
//
// The one the symptom points at. `ready` is closed and the pump still never answers, so this goes
// through the pump's own bound first and then through the poller's — which is the ordering a real
// shutdown has, and the reason the total is bounded rather than merely each part.
func TestAPollerThatNeverComesBackDoesNotWedgeTheShutdown(t *testing.T) {
	aCleanPackage(t) // see the note on the test above
	r := aRecordingThatWillNotFinishShuttingDown(true)
	done := make(chan struct{})
	started := time.Now()
	go func() { defer close(done); r.Stop() }()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("Stop never returned for a recording whose app-switch poller never " +
			"came back, which is the thirty-second silence a played routine " +
			"reported as nothing at all")
	}
	// AND IT DID NOT TAKE FOR EVER MINUS ONE. The bound is the point: a shutdown that
	// waits five minutes is a shutdown nobody distinguishes from a hang.
	if took := time.Since(started); took > 40*time.Second {
		t.Errorf("the shutdown took %s, which is not bounded by anything a person "+
			"would wait through", took)
	}
}

// A WORKER HELD ACROSS A TIMED-OUT SHUTDOWN CANNOT REACH A LATER RECORDING.
//
// # What the finding was
//
// `Stop` is bounded, so a shutdown that cannot complete gives up and returns — and the app-switch
// poller and the capture worker are still running. The poller's one blocking call is a
// foreground-window read, which does not return while the window's owning thread is not pumping;
// the capture worker is doing screen work. Either can come back AFTER another recording has
// started, and both publish through this package's global state: an old poller's app switch
// appears in somebody else's demonstration as a step they never performed, and an old capture
// writes a picture into whatever event is at its index now.
//
// # Both halves of the answer
//
// The workers are fenced by SESSION, which is what this exercises: a late publish from an
// abandoned recording is dropped.
//
// And a further recording is REFUSED outright, because the hooks cannot be fenced from Go at all.
// They are C callbacks with no context of their own, they gate on `recActive`, and the next
// `Start` would set that true again — so an abandoned recording's hooks, still installed because
// the thread that owns them never answered, would feed the new one. A session number cannot reach
// a C callback, so the honest consequence of hooks that are still installed is that recording is
// over until the process restarts, which is what the shutdown's own message already said.
//
// See [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].
func TestAWorkerHeldAcrossATimeoutCannotReachALaterRecording(t *testing.T) {
	aCleanPackage(t)

	// THE ABANDONED RECORDING. Its session is taken the way Start takes one, and its
	// shutdown gives up: nothing ever closes `stopped` or `pollStopped`.
	old := aRecordingThatWillNotFinishShuttingDown(true)
	old.session = recSession.Add(1)
	done := make(chan struct{})
	go func() { defer close(done); old.Stop() }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("the shutdown never returned")
	}

	// A LATER RECORDING IS REFUSED, and says what to do about it.
	err := New().Start()
	if err == nil {
		t.Fatal("a second recording started after a shutdown that could not take its " +
			"hooks off, so the abandoned recording's hooks would feed it")
	}
	for _, says := range []string{"cannot record again", "could not be taken off",
		"open it again"} {
		if !strings.Contains(err.Error(), says) {
			t.Errorf("the refusal does not say %q: %v", says, err)
		}
	}

	// AND THE OLD WORKER'S LATE PUBLISH IS DROPPED EVEN SO.
	//
	// Asserted separately, because the refusal above is what protects a real process and
	// the fence is what makes the workers correct: a build that only refused would be one
	// line from being wrong again the day somebody allowed recording to resume.
	//
	// The ORDERING this depends on — eligible first, replaced second, released last — is
	// what `TestAPausedWorkerPublishesIntoNothing` is about. Here it is enough that the
	// late publish goes nowhere.
	next := &winRecorder{}
	next.beginRecording() // as a later Start would
	recMu.Lock()
	recBuf = []RecordedEvent{{Kind: EvKey, KeyName: "a"}}
	recMu.Unlock()
	// WHAT THE POLLER DOES WHEN ITS READ FINALLY RETURNS, in the one line that matters.
	if old.recordAppSwitch("somebody-elses-window", time.Now()) {
		t.Error("an abandoned poller published an app switch into a later recording, " +
			"which is a step nobody performed in somebody else's demonstration")
	}
	recMu.Lock()
	n, kind := len(recBuf), recBuf[0].Kind
	recMu.Unlock()
	if n != 1 || kind != EvKey {
		t.Errorf("a later recording's buffer holds %d event(s) starting with %v after "+
			"an abandoned poller came back", n, kind)
	}
	// AND THE RECORDING OF THE CURRENT SESSION IS NOT FENCED OUT, or the check above is
	// satisfied by a fence that drops everything.
	if !next.recordAppSwitch("its-own-window", time.Now()) {
		t.Error("the current recording is fenced out of its own state")
	}
}

// A WORKER PAUSED AFTER ITS ELIGIBILITY CHECK PUBLISHES INTO NOTHING.
//
// # The finding
//
// The fence was read where a worker came back, and the write happened afterwards — two moments.
// A worker could pass its check, pause, and publish after a new recording had reset the buffer:
// an app switch appended to somebody else's demonstration, or a picture written onto whatever
// press is at that index now. Checking first and writing second is not a fence, however correct
// each half is on its own.
//
// # Why the order this test runs in is the whole of it
//
// Because the test it replaces changed the session BEFORE asking eligibility, which the broken
// shape passes too: it established only that the question had an answer. This one asks
// eligibility while the recording is still the worker's, starts the next recording, and releases
// the worker only then.
//
// There is no instant between the two halves to pause in any more — the fence is read under the
// lock that writes — so what a paused worker now meets is the lock, and on the other side of it
// the session has moved. What the test can still do is hold the worker where the gap used to be,
// which is what this is.
//
// See [[ADR-263-the-alpha-is-the-studio-stack-and-it-says-what-it-is]].
func TestAPausedWorkerPublishesIntoNothing(t *testing.T) {
	aCleanPackage(t)

	// ── THE RECORDING THE TWO WORKERS BELONG TO ──
	old := &winRecorder{}
	old.beginRecording()
	idx := recordIndexed(RecordedEvent{Kind: EvClick, X: 10, Y: 20})

	// ── AND EACH OF THEM IS ELIGIBLE, ASKED HERE AND NOT LATER ──
	//
	// The poller's eligibility is its sample being an application and being a change; the
	// capture's is holding an index into the buffer it was taken from. Both are true at
	// this instant, which is the instant the broken shape asked about.
	if !anAppSwitchToRecord("notepad", "") {
		t.Fatal("the poller's sample is not eligible, so this test cannot pause after " +
			"an eligibility check that passed")
	}
	if idx != 0 {
		t.Fatalf("the capture's index is %d, so this recording is not the one it was "+
			"taken from", idx)
	}

	// ── THE NEXT RECORDING STARTS WHILE BOTH ARE PAUSED ──
	next := &winRecorder{}
	next.beginRecording()
	if !next.recordAppSwitch("its-own-app", time.Now()) {
		t.Fatal("a live recording cannot publish its own app switch")
	}
	keep := recordIndexed(RecordedEvent{Kind: EvClick, X: 90, Y: 90})

	// ── AND NOW THEY ARE RELEASED ──
	if old.recordAppSwitch("somebody-elses-window", time.Now()) {
		t.Error("a paused poller published into the recording that replaced its own")
	}
	if old.attach(captured{idx: idx, img: []byte("a picture of an old press"),
		color: "0x00FF00", clickLX: 1, clickLY: 2, x: 77, y: 88, recenter: true}) {
		t.Error("a paused capture wrote onto the recording that replaced its own")
	}

	// ── WHAT THE LATER RECORDING HOLDS ──
	recMu.Lock()
	got := append([]RecordedEvent(nil), recBuf...)
	recMu.Unlock()
	if len(got) != keep+1 {
		t.Fatalf("the later recording holds %d event(s), want the %d it recorded "+
			"itself: %+v", len(got), keep+1, got)
	}
	ev := got[keep]
	if ev.Image != nil {
		t.Error("an abandoned capture put a picture of an old press onto a press in a " +
			"later recording")
	}
	if ev.Color != "" {
		t.Errorf("an abandoned capture wrote the colour %q onto a later recording's "+
			"press", ev.Color)
	}
	if ev.X != 90 || ev.Y != 90 {
		t.Errorf("an abandoned capture moved a later recording's click to %d,%d, so it "+
			"would be replayed somewhere nobody pressed", ev.X, ev.Y)
	}
	if ev.Kind == EvAppSwitch {
		t.Error("an abandoned poller's switch is where the later recording's press " +
			"should be")
	}
	if fore := foreground(); fore != "its-own-app" {
		t.Errorf("the foreground the hooks tag presses with is %q, so an abandoned "+
			"poller decided which application a later recording was in", fore)
	}

	// ── AND THE RECORDING THAT OWNS THE STATE IS NOT FENCED OUT ──
	//
	// Or every assertion above is satisfied by a publication that drops everything.
	if !next.attach(captured{idx: keep, img: []byte("its own picture"),
		color: "0x0000FF", clickLX: 3, clickLY: 4, x: 91, y: 92, recenter: true}) {
		t.Fatal("a live recording cannot attach a picture to its own press")
	}
	recMu.Lock()
	mine := recBuf[keep]
	recMu.Unlock()
	if string(mine.Image) != "its own picture" || mine.X != 91 || mine.Y != 92 ||
		mine.Color != "0x0000FF" || mine.ClickX != 3 || mine.ClickY != 4 {
		t.Errorf("a live recording's own capture did not land: %+v", mine)
	}

	// ── AND A DROPPED SAMPLE IS NOT REMEMBERED AS RECORDED ──
	//
	// `last` is what the next tick compares against. A poller that advanced it on a
	// publication that was refused would go quiet about that application: the switch was
	// dropped, and the next sample of it is "no change".
	if got := old.sampled("somebody-elses-window", "notepad", time.Now()); got != "notepad" {
		t.Errorf("an abandoned poller advanced its last-recorded application to %q "+
			"although nothing was recorded", got)
	}
	if got := next.sampled("another-app", "its-own-app", time.Now()); got != "another-app" {
		t.Errorf("a live poller did not advance its last-recorded application: %q", got)
	}
}

// AND A RECORDING IS NOT REPLACED WHILE A WORKER IS PUBLISHING.
//
// The other half of "under the same synchronization". The fence is read under `recMu`, which
// fences nothing unless the replacement takes that lock too — and it did not: `recSession.Add(1)`
// sat on the line BEFORE `recMu.Lock()`, so the session could move while a worker was inside a
// publication, and the buffer that worker was writing into could be emptied a moment later.
//
// A NEGATIVE ASSERTION WITH A WINDOW, which is the honest shape for "this does not happen": what
// it rejects completes immediately — an atomic add waits for nothing — so the failure is
// deterministic, and the pass is a window in which nothing happened.
func TestARecordingIsNotReplacedWhileAWorkerIsPublishing(t *testing.T) {
	aCleanPackage(t)

	recMu.Lock()
	was := recSession.Load()
	replaced := make(chan struct{})
	go func() {
		defer close(replaced)
		(&winRecorder{}).beginRecording()
	}()
	select {
	case <-replaced:
		recMu.Unlock()
		t.Fatal("a new recording replaced the session while the publication lock was " +
			"held, so a worker inside a publication can have the state it is " +
			"writing into taken out from under it")
	case <-time.After(100 * time.Millisecond):
	}
	if now := recSession.Load(); now != was {
		t.Errorf("the session moved from %d to %d while the publication lock was held",
			was, now)
	}
	recMu.Unlock()

	// AND IT COMPLETES ONCE THE LOCK IS FREE, or the wait above proved nothing: a
	// replacement that never happens is not a replacement that waited.
	select {
	case <-replaced:
	case <-time.After(10 * time.Second):
		t.Fatal("a recording could not be replaced after the publication lock was " +
			"released")
	}
	if recSession.Load() == was {
		t.Error("the session never moved, so nothing here is about a replacement")
	}
}

// AND THE FENCE IS READ UNDER THE LOCK, EVERYWHERE IT IS READ.
//
// The two tests above establish that a publication's fence and its write are one step, and that a
// replacement has to take the same lock. Neither can see WHERE in a function the fence is read:
// move `mineUnderLock` one line up, above `recMu.Lock()`, and both still pass while the gap the
// finding was about is open again. No test can pause a goroutine between two adjacent lines, so
// the rule is checked where it is written.
//
// The name is the rule — a reader that must hold the lock says so — and this is what makes that
// more than a comment. A deferred unlock is not an unlock yet: `attach` takes the lock and defers
// its release on the next line, which in source order would otherwise read as a release before
// the fence.
func TestTheSessionFenceIsReadUnderTheLock(t *testing.T) {
	const file = "recorder_windows.go"
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", file, err)
	}
	reads := 0
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		held := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			// A DEFERRED UNLOCK IS NOT AN UNLOCK YET. It runs when the function
			// returns, so the lock is still held by everything after it.
			if d, ok := n.(*ast.DeferStmt); ok {
				return !isCallTo(d.Call, "recMu", "Unlock")
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch {
			case isCallTo(call, "recMu", "Lock"):
				held = true
			case isCallTo(call, "recMu", "Unlock"):
				held = false
			case isCallTo(call, "r", "mineUnderLock"):
				reads++
				if !held {
					t.Errorf("%s reads the session fence without holding "+
						"recMu, so the recording can be replaced "+
						"between the check and the write", fn.Name.Name)
				}
			}
			return true
		})
	}
	// AND THERE IS A FENCE TO FIND. A rename, or a publication that stopped asking,
	// would otherwise satisfy this by having nothing to check.
	if reads < 2 {
		t.Errorf("%d publication(s) read the session fence; the poller and the capture "+
			"worker are two", reads)
	}
}

// isCallTo reports whether a call is `recv.name(...)`.
func isCallTo(call *ast.CallExpr, recv, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == recv
}

// A SAMPLE WORTH RECORDING, AND THREE THAT ARE NOT.
//
// The decision the poller makes twice, asked directly. The goroutine that asks it cannot be
// started in a test — `Start` installs real low-level hooks — so the decision is its own function
// and this is what holds it. A mutation taking the session fence out of ONE of the two call sites
// is what made that necessary.
//
// THE FENCE IS NO LONGER ONE OF THESE QUESTIONS, and that is the finding this file's later tests
// are about: asked here, it would be asked where the sample came back and acted on afterwards.
// It moved to `recordAppSwitch`, under the lock that writes. What is left here is what can be
// decided from the sample alone.
func TestAnAbandonedPollersSampleIsNotRecorded(t *testing.T) {
	// THE FIRST SAMPLE: nothing recorded yet, so any application is a change.
	if !anAppSwitchToRecord("notepad", "") {
		t.Error("the first sample of a live recording is not recorded, so a " +
			"demonstration would not say which application it began in")
	}
	// AND A LATER ONE, only when it differs.
	if !anAppSwitchToRecord("chrome", "notepad") {
		t.Error("a change of application is not recorded")
	}
	if anAppSwitchToRecord("notepad", "notepad") {
		t.Error("the same application is recorded again, so a demonstration fills up " +
			"with switches to where it already was")
	}
	if anAppSwitchToRecord("", "notepad") {
		t.Error("a sample that found no application is recorded as a switch to none")
	}
}
