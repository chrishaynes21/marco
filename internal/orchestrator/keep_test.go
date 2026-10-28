package orchestrator

// ONE SET OF RULES, HELD AT EVERY SURFACE THAT KEEPS A RECORDING.
//
// Three divergences were measured before this, and each of them is a case here:
//
//	a collision      `marco learn` of a name twice REPLACED the first routine, silently
//	a scope          two of its three answers saved a routine that refuses every press
//	a mode           neither path stated what it captured, and still does not, on purpose
//
// Through the PRODUCTION entrances — `Deps.Learn` and `Deps.LearnAuto`, the ones `marco learn`
// and the widget call — rather than through `Keep` alone. A test of the rule that did not enter
// where a person does would have passed for the whole time the rule was missing: both of the
// surfaces had `Save` underneath them and neither reached these checks.
//
// See [[ADR-266-one-set-of-rules-for-keeping-a-recording]].

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aPlainDemonstration types one key in an application and ends with the stop gesture.
//
// Nothing in it checks a window, so every scope is open to it — which is what makes it the
// control for the scope cases and the fixture for the collision ones.
func aPlainDemonstration(key string) []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: at(0)},
		{Kind: recorder.EvKey, KeyName: key, Down: true, T: at(10), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: key, Down: false, T: at(20), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(30), App: "notepad"},
	}
}

// aWindowCheckingDemonstration clicks where the press asks to be checked against the window it
// was made in — so it can only be saved to a scope that brings that window forward.
func aWindowCheckingDemonstration() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			App: "arkascended", PID: 4242, T: at(100), WinRel: true,
			RelX: 722, RelY: 210, RelTo: "window", WinW: 1920, WinH: 1080,
			Dpi: 144, Process: "arkascended.exe"},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 842, Y: 274,
			App: "arkascended", PID: 4242, T: at(105)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(200), App: "arkascended"},
	}
}

// aTerminal is the Deps `marco learn` builds, with the answers typed in and nothing else changed.
func aTerminal(reg routes.Registry, events []recorder.RecordedEvent, answers string,
	out *bytes.Buffer, app string) Deps {

	return Deps{
		Reg: reg, Rec: &fakeRecorder{events: events},
		In:      strings.NewReader(answers),
		Out:     out,
		App:     func() string { return app },
		StopKey: "esc",
	}
}

// ── A COLLISION KEEPS WHAT IS THERE ──────────────────────────────────────────

// AND LEARNING A NAME TWICE DOES NOT REPLACE THE FIRST ONE BY ITSELF.
//
// Measured before this: the second learn wrote over the first, the library kept one routine, and
// the line printed said `Learned` as though it were a fresh save. Re-recording is what somebody
// testing a routine does all afternoon, which makes the likeliest casualty the routine they were
// comparing against.
func TestLearningANameTwiceAsksBeforeItReplacesAnything(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "notepad", Slug: "the-same-name"}

	var first bytes.Buffer
	d := aTerminal(reg, aPlainDemonstration("a"), "y\nc\n", &first, "notepad")
	if err := d.Learn("the same name"); err != nil {
		t.Fatalf("the first learn: %v\n%s", err, first.String())
	}
	was := sourceAt(t, reg, rt)
	if !strings.Contains(was, `"a"`) {
		t.Fatalf("the first recording did not save what it demonstrated:\n%s", was)
	}

	// ── ENTER MEANS NO ──
	//
	// The two answers do not cost the same. No costs a second attempt; yes costs a
	// routine and the demonstration beside it, neither of which comes back. A person
	// pressing Enter through a prompt they did not read must not lose work by it.
	var declined bytes.Buffer
	d = aTerminal(reg, aPlainDemonstration("z"), "y\nc\n\n", &declined, "notepad")
	if err := d.Learn("the same name"); err != nil {
		t.Fatalf("the second learn: %v\n%s", err, declined.String())
	}
	if now := sourceAt(t, reg, rt); now != was {
		t.Errorf("Enter replaced the routine that was already there:\n%s", now)
	}
	// AND IT NAMED THE EXACT ROUTE BEING REPLACED, which is what somebody is agreeing
	// to: the application, the scope and the slug, not just a name.
	for _, says := range []string{"already a routine saved there", "notepad/context/the-same-name",
		"[y]es / [N]o", "Left alone"} {
		if !strings.Contains(declined.String(), says) {
			t.Errorf("the question does not say %q:\n%s", says, declined.String())
		}
	}

	// ── AND AN EXPLICIT YES REPLACES IT, AND SAYS THAT IS WHAT IT DID ──
	var agreed bytes.Buffer
	d = aTerminal(reg, aPlainDemonstration("z"), "y\nc\ny\n", &agreed, "notepad")
	if err := d.Learn("the same name"); err != nil {
		t.Fatalf("the third learn: %v\n%s", err, agreed.String())
	}
	now := sourceAt(t, reg, rt)
	if !strings.Contains(now, `"z"`) {
		t.Errorf("an explicit yes did not replace the routine:\n%s", now)
	}
	if !strings.Contains(agreed.String(), "Replaced") {
		t.Errorf("the save that replaced one does not say so:\n%s", agreed.String())
	}
	// AND STILL ONE ROUTINE, so this is a replacement and not a second file somewhere.
	if got := reg.List(); len(got) != 1 {
		t.Errorf("the library holds %+v, want the one routine", got)
	}
}

// AND THE WIDGET'S UNATTENDED PATH REFUSES, BECAUSE THERE IS NOBODY TO ASK.
//
// Record, press F12, saved: no prompt and no review. Of the three surfaces this is the one where a
// silent overwrite is worst, so it is the one that will not do it at all.
func TestAutoLearnRefusesACollisionAndKeepsWhatIsThere(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "notepad", Slug: "the-same-name"}

	var first bytes.Buffer
	d := aTerminal(reg, aPlainDemonstration("a"), "", &first, "notepad")
	if err := d.LearnAuto("the same name"); err != nil {
		t.Fatalf("the first auto-learn: %v\n%s", err, first.String())
	}
	was := sourceAt(t, reg, rt)

	var second bytes.Buffer
	d = aTerminal(reg, aPlainDemonstration("z"), "", &second, "notepad")
	// NOT AN ERROR. The recording happened and nothing is wrong with the machine; what
	// could not happen is the save, and the caller's exit code is not the place to say so.
	if err := d.LearnAuto("the same name"); err != nil {
		t.Fatalf("the second auto-learn returned an error: %v\n%s", err, second.String())
	}
	if now := sourceAt(t, reg, rt); now != was {
		t.Errorf("the unattended path replaced a routine with nobody having looked at "+
			"it:\n%s", now)
	}
	for _, says := range []string{"Not saving", "already a routine saved there"} {
		if !strings.Contains(second.String(), says) {
			t.Errorf("the refusal does not say %q:\n%s", says, second.String())
		}
	}
	if strings.Contains(second.String(), "Learned") {
		t.Errorf("it said it learned something and wrote nothing:\n%s", second.String())
	}
}

// AND IF IT CHANGED WHILE THE QUESTION WAS OPEN, NOTHING IS WRITTEN.
//
// Somebody agreed to replace ONE routine. Between the question and the answer that routine can be
// deleted, or saved again from a terminal, and the consent was not to whatever is there by then.
// So the target is re-derived and re-checked at the instant of writing rather than trusted from
// the look that produced the question.
//
// The interleaving is real: the reader below removes the routine as the answer is read, which is
// the same instant somebody else saving would land in.
func TestAReplacementRefusesWhenTheRoutineChangedWhileTheQuestionWasOpen(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "notepad", Slug: "the-same-name"}

	var first bytes.Buffer
	d := aTerminal(reg, aPlainDemonstration("a"), "y\nc\n", &first, "notepad")
	if err := d.Learn("the same name"); err != nil {
		t.Fatalf("the first learn: %v\n%s", err, first.String())
	}

	var out bytes.Buffer
	// THE ROUTINE GOES AS THE "yes" IS TYPED. The first two answers are the save and the
	// scope; the third is the consent, and reading it is when the library changes.
	answers := &readerThatActs{
		lines: []string{"y", "c", "y"},
		on:    2,
		act:   func() { _ = reg.Delete(rt) },
	}
	d = aTerminal(reg, aPlainDemonstration("z"), "", &out, "notepad")
	d.In = answers
	err := d.Learn("the same name")

	if err == nil {
		t.Fatalf("it replaced a routine that had gone while the question was open:\n%s",
			out.String())
	}
	if !strings.Contains(err.Error(), "to replace any more") {
		t.Errorf("the refusal does not say what changed: %v", err)
	}
	// AND NOTHING WAS WRITTEN IN ITS PLACE, which is the point: the consent was to a
	// replacement, and an ordinary save is a different thing nobody agreed to.
	if got := reg.List(); len(got) != 0 {
		t.Errorf("the library holds %+v, want nothing — the replacement was refused", got)
	}
}

// readerThatActs hands back one line at a time and runs act() as the given line is read.
//
// Byte at a time, because that is how the orchestrator's own reader reads: it takes no
// read-ahead so it can share stdin with an interactive caller. The side effect fires on the first
// byte of the chosen line, which is inside the window between the question being printed and the
// answer being complete.
type readerThatActs struct {
	lines []string
	on    int
	act   func()
	at    int
	buf   string
	fired bool
}

func (r *readerThatActs) Read(p []byte) (int, error) {
	if r.buf == "" {
		if r.at >= len(r.lines) {
			return 0, io.EOF
		}
		if r.at == r.on && !r.fired {
			r.fired = true
			r.act()
		}
		r.buf = r.lines[r.at] + "\n"
		r.at++
	}
	p[0] = r.buf[0]
	r.buf = r.buf[1:]
	return 1, nil
}

// AND CONSENT IS TO A VERSION, NOT TO A PATH.
//
// A routine's identity is its place in the library, and a place does not change when its contents
// do. `Keep` checked that the route still existed with the same id, which catches it being
// DELETED while the question is open and misses it being REPLACED: another save landing there
// leaves a newer routine at the identity somebody agreed to overwrite, and it was overwritten.
// They agreed to lose the version they were shown.
//
// The interleaving is real again: the contents change under the same identity as the "yes" is
// read, which is the instant another writer would land in.
func TestAReplacementRefusesWhenTheContentsChangedUnderTheSameIdentity(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "notepad", Slug: "the-same-name"}

	var first bytes.Buffer
	d := aTerminal(reg, aPlainDemonstration("a"), "y\nc\n", &first, "notepad")
	if err := d.Learn("the same name"); err != nil {
		t.Fatalf("the first learn: %v\n%s", err, first.String())
	}

	// SOMEBODY ELSE SAVES OVER IT WHILE THE QUESTION IS OPEN. Same place, same id, and a
	// routine nobody in this flow has ever seen.
	newer := "// somebody else's routine, saved while the question was on screen\n" +
		mustRead(t, reg, rt)
	var out bytes.Buffer
	answers := &readerThatActs{
		lines: []string{"y", "c", "y"},
		on:    2,
		act:   func() { _ = reg.Replace(rt, newer) },
	}
	d = aTerminal(reg, aPlainDemonstration("z"), "", &out, "notepad")
	d.In = answers
	err := d.Learn("the same name")

	if err == nil {
		t.Fatalf("it replaced a routine that had been saved again while the question "+
			"was open:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "saved again since you were shown it") {
		t.Errorf("the refusal does not say what changed: %v", err)
	}
	// AND THE NEWER ROUTINE IS EXACTLY AS ITS OWN WRITER LEFT IT.
	if now := mustRead(t, reg, rt); now != newer {
		t.Errorf("the newer routine was written over:\n%s", now)
	}

	// ── AND THE DEMONSTRATION IS HALF OF THE VERSION ──
	//
	// A routine and the recording beside it are both replaced by a save, and both are
	// lost by one. A routine re-recorded from the same steps has the same source and a
	// different demonstration, so checking the source alone would call that the version
	// somebody was shown — and the thing they would lose is the recording, which is the
	// only thing a rebuild can be made from.
	var over bytes.Buffer
	d = aTerminal(reg, aPlainDemonstration("y"), "y\nc\n", &over, "notepad")
	at := d.Collision(d.DraftOf("the same name", aPlainDemonstration("y"), "notepad",
		"a test", Unstated), ScopeApp, "notepad")
	if !at.Exists || at.Recording == "" {
		t.Fatalf("the collision does not say which demonstration it is about: %+v", at)
	}
	theirRec := []byte(`[{"kind":"somebody else's demonstration"}]`)
	answers = &readerThatActs{
		lines: []string{"y", "c", "y"},
		on:    2,
		act:   func() { _ = reg.SaveRecording(rt, theirRec) },
	}
	var recOut bytes.Buffer
	d = aTerminal(reg, aPlainDemonstration("y"), "", &recOut, "notepad")
	d.In = answers
	err = d.Learn("the same name")
	if err == nil {
		t.Fatalf("it replaced a routine whose demonstration had been written again "+
			"while the question was open:\n%s", recOut.String())
	}
	if !strings.Contains(err.Error(), "saved again since you were shown it") {
		t.Errorf("the refusal does not say what changed: %v", err)
	}
	if rec, ok := reg.LoadRecording(rt); !ok || string(rec) != string(theirRec) {
		t.Errorf("their demonstration was written over: ok=%v %s", ok, rec)
	}
}

// AND A FRESH SAVE DOES NOT WRITE OVER A ROUTINE THAT ARRIVED AFTER THE CHECK.
//
// The collision check and the write are two instants. `Keep` asked, was told the path was free,
// and then called a create-or-truncate write — so another writer landing in the gap was
// overwritten by a caller that had consented to nothing about it. The gap is as long as whatever
// happens in between, which on an interactive surface is as long as somebody takes to answer.
//
// The create is exclusive now, which closes the gap rather than narrowing it: the test and the
// write are one operation. This drives that through the production entrance by having the other
// writer land while the SCOPE answer is being read, which is after the first collision look.
func TestAFreshSaveDoesNotOverwriteAConcurrentSave(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "notepad", Slug: "ordinary"}

	theirs := "// somebody else's routine, created after the collision check\nuse os.\n"
	theirRec := []byte(`[{"kind":"theirs"}]`)
	var out bytes.Buffer
	answers := &readerThatActs{
		lines: []string{"y", "c"},
		// AS THE SCOPE IS ANSWERED, which is between the save being asked for and
		// the write happening.
		on: 1,
		act: func() {
			if err := reg.Create(rt, theirs); err != nil {
				panic(err)
			}
			if err := reg.SaveRecording(rt, theirRec); err != nil {
				panic(err)
			}
		},
	}
	d := aTerminal(reg, aPlainDemonstration("a"), "", &out, "notepad")
	d.In = answers

	// NOT AN ERROR FROM Learn: the collision is the person's to answer, and what it
	// answered is "leave it alone". What matters is the file.
	if err := d.Learn("ordinary"); err != nil {
		t.Fatalf("learning: %v\n%s", err, out.String())
	}
	if now := mustRead(t, reg, rt); now != theirs {
		t.Errorf("the routine created after the collision check was written over:\n%s",
			now)
	}
	if rec, ok := reg.LoadRecording(rt); !ok || string(rec) != string(theirRec) {
		t.Errorf("their demonstration was written over: ok=%v %s", ok, rec)
	}
	// AND IT SAID SO, rather than reporting a save that did not happen.
	if strings.Contains(out.String(), "Learned") {
		t.Errorf("it said it learned something and wrote nothing:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "already a routine saved there") &&
		!strings.Contains(out.String(), "while this one was being kept") {
		t.Errorf("nothing said why it was not saved:\n%s", out.String())
	}
}

// AND IF THE GAP IS HIT FOR REAL, THEIR FILE IS STILL THEIRS.
//
// The test above drives the realistic window — between the question and the answer — and the
// collision check is what catches it there. This one is about the gap the check CANNOT cover: the
// instants between `Keep` looking and `Keep` writing, which no prompt sits inside and no seam can
// stand in.
//
// So it is raced, many times, with another writer creating the target while the save is running.
// It cannot fail wrongly: the assertion is only ever made about an iteration where the other
// writer's create SUCCEEDED, and in that iteration those bytes are the ones nobody consented to
// lose. With a create-or-truncate write some of those iterations lose them.
func TestWhenTheGapIsRacedTheOtherWritersFileSurvives(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	const theirs = "// somebody else's routine\nuse os.\n"
	rt := routes.Route{App: "notepad", Slug: "ordinary"}

	raced := 0
	for i := 0; i < 240; i++ {
		reg := routes.Registry{Dir: t.TempDir()}
		var out bytes.Buffer
		d := aTerminal(reg, aPlainDemonstration("a"), "y\nc\n", &out, "notepad")

		made := make(chan bool, 1)
		started := make(chan struct{})
		go func() {
			<-started
			// NO SLEEP AND NO CLOCK. The stagger is however long the scheduler
			// takes to get here, which is what makes some iterations land inside
			// the window and some outside it; a test that waited a fixed time
			// would be a test of the wait.
			made <- reg.Create(rt, theirs) == nil
		}()
		close(started)
		_ = d.Learn("ordinary")
		if !<-made {
			continue // they lost the create; this iteration says nothing
		}
		raced++
		if now := mustRead(t, reg, rt); now != theirs {
			t.Fatalf("iteration %d: the other writer created the routine and it was "+
				"written over:\n%s", i, now)
		}
		if rec, ok := reg.LoadRecording(rt); ok {
			t.Fatalf("iteration %d: a demonstration was written beside a routine "+
				"this save did not write: %s", i, rec)
		}
	}
	// AND THE RACE HAPPENED, or everything above is an assertion about nothing.
	if raced == 0 {
		t.Skip("the other writer never won a create, so nothing was raced")
	}
	t.Logf("the other writer created the target in %d of 240 iterations", raced)
}

// AND TWO SAVES OF ONE ROUTINE SERIALISE, WITH THE STALE ONE REFUSING.
//
// The last gap in the replacement, and the one atomic writing cannot close. Reading the digests,
// rendering, and writing are three operations: another save landing between the first and the
// last had its version overwritten by a caller whose comparison was true when it was made and was
// not true any more. `Registry.Replace` makes nobody read a half-written routine and says nothing
// about that.
//
// So the route is held across the whole of validate-and-write, and this stands in the one instant
// that exists for standing in — `AfterValidating`, inside the lock, after the comparison and
// before anything is written. Deterministic, not raced: the second save CANNOT proceed while the
// first holds the route, so there is nothing here that depends on timing to be true.
//
// Through `Keep`, because that is where the interleaving is. It is the shared backend every
// surface goes through, and there is no prompt inside the critical section for a surface-level
// test to stand in.
func TestTwoSavesOfOneRoutineSerialiseAndTheStaleConsentRefuses(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "notepad", Slug: "the-same-name"}

	var out bytes.Buffer
	base := aTerminal(reg, aPlainDemonstration("a"), "y\nc\n", &out, "notepad")
	if err := base.Learn("the same name"); err != nil {
		t.Fatalf("the first save: %v\n%s", err, out.String())
	}
	v1 := mustRead(t, reg, rt)

	draftOf := func(d Deps, key string) *Draft {
		return d.DraftOf("the same name", aPlainDemonstration(key), "notepad",
			"a test", Unstated)
	}
	consentFor := func(d Deps, dr *Draft) Consent {
		at := d.Collision(dr, ScopeApp, "notepad")
		if !at.Exists || at.Source == "" {
			t.Fatalf("the collision does not say which version it is about: %+v", at)
		}
		return Consent{ID: at.ID, Source: at.Source, Recording: at.Recording}
	}

	// ── THE FIRST SAVE, PAUSED AFTER ITS VERSION WAS CHECKED AND INSIDE THE LOCK ──
	validated, resume := make(chan struct{}), make(chan struct{})
	first := aTerminal(reg, nil, "", &out, "notepad")
	first.AfterValidating = func() {
		close(validated)
		<-resume
	}
	drFirst := draftOf(first, "b")
	okFirst := consentFor(first, drFirst)
	doneFirst := make(chan error, 1)
	go func() {
		_, err := first.Keep(drFirst, ScopeApp, "notepad", okFirst)
		doneFirst <- err
	}()
	<-validated

	// ── AND A SECOND SAVE ATTEMPTED WHILE IT IS PAUSED ──
	//
	// Its consent is read NOW, so it is entirely valid: the first save has validated and
	// written nothing, and v1 is what is on disk. Nothing about this save is wrong except
	// that it is second.
	second := aTerminal(reg, nil, "", &out, "notepad")
	drSecond := draftOf(second, "c")
	okSecond := consentFor(second, drSecond)
	if okSecond.Source != okFirst.Source {
		t.Fatalf("the two saves consented to different versions, so this says nothing "+
			"about serialising: %q and %q", okFirst.Source, okSecond.Source)
	}
	doneSecond := make(chan error, 1)
	go func() {
		_, err := second.Keep(drSecond, ScopeApp, "notepad", okSecond)
		doneSecond <- err
	}()

	// IT MUST NOT GET PAST THE LOCK. The invariant is the file rather than the clock: the
	// wait is only how long this is prepared to be wrong for, and what it asserts is that
	// nothing was written while the first save held the route.
	select {
	case err := <-doneSecond:
		t.Fatalf("the second save finished while the first held the route: %v", err)
	case <-time.After(250 * time.Millisecond):
	}
	if now := mustRead(t, reg, rt); now != v1 {
		t.Fatalf("something was written while the first save was paused:\n%s", now)
	}

	// ── THE FIRST ONE FINISHES ──
	close(resume)
	if err := <-doneFirst; err != nil {
		t.Fatalf("the first save refused: %v", err)
	}
	afterFirst := mustRead(t, reg, rt)
	if afterFirst == v1 {
		t.Fatal("the first save wrote nothing, so the rest of this says nothing")
	}

	// ── AND THE SECOND ONE REVALIDATES AGAINST WHAT IT ACTUALLY LEFT ──
	//
	// Its consent was to v1, which has gone. It refuses, and the version the first save
	// wrote is exactly as that save left it. This is the overwrite the lock exists for:
	// without it the second save validated against v1 while the first was still deciding,
	// and wrote over it afterwards.
	errSecond := <-doneSecond
	if errSecond == nil {
		t.Fatal("the second save went ahead with consent to a version that had gone")
	}
	if !strings.Contains(errSecond.Error(), "saved again since you were shown it") {
		t.Errorf("the refusal does not say what changed: %v", errSecond)
	}
	if now := mustRead(t, reg, rt); now != afterFirst {
		t.Errorf("the first save's version was written over:\n%s", now)
	}

	// ── THE CONTROL: A REPLACEMENT WHOSE CONSENT IS CURRENT STILL GOES THROUGH ──
	//
	// Without which every refusal above is satisfied by a save that has stopped working.
	third := aTerminal(reg, nil, "", &out, "notepad")
	drThird := draftOf(third, "d")
	if _, err := third.Keep(drThird, ScopeApp, "notepad",
		consentFor(third, drThird)); err != nil {
		t.Fatalf("a replacement with current consent was refused: %v", err)
	}
	if now := mustRead(t, reg, rt); now == afterFirst {
		t.Error("the replacement with current consent wrote nothing")
	}
	// AND THE LOCK WAS GIVEN BACK EVERY TIME, which is what that control also proves:
	// a hold that leaked would have made this one wait out `SaveWithin` and refuse.
	free, err := reg.Lock(rt, 50*time.Millisecond)
	if err != nil {
		t.Errorf("the route is still held after every save finished: %v", err)
	}
	// AND GIVEN BACK AGAIN. A hold left open keeps the file open, which on Windows stops
	// the directory being removed — which is how this test first failed, in its own
	// cleanup rather than in an assertion.
	free.Release()
}

// AND HOLDING ONE ROUTINE DOES NOT HOLD ITS NAMESAKE.
//
// A route IS its scope and its slug, so `settings` saved for one application and `settings` saved
// to bring that application forward are two routines and two files. One lock for the slug would
// make a save of either wait for the other, which is a save queueing behind an unrelated one —
// and at the end of the wait, refusing.
//
// The rendezvous file is therefore beside the route rather than at the top of the library, which
// is the same reason `Routine.ID` carries the whole identity and not the name.
func TestHoldingOneRoutineDoesNotHoldItsNamesake(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	inPlace := routes.Route{App: "notepad", Slug: "settings"}
	anywhere := routes.Route{App: "notepad", Focus: true, Slug: "settings"}

	var out bytes.Buffer
	for _, where := range []struct {
		answers string
		rt      routes.Route
	}{{"y\nc\n", inPlace}, {"y\nf\n", anywhere}} {
		d := aTerminal(reg, aPlainDemonstration("a"), where.answers, &out, "notepad")
		if err := d.Learn("settings"); err != nil {
			t.Fatalf("saving %+v: %v\n%s", where.rt, err, out.String())
		}
		if !reg.Has(where.rt) {
			t.Fatalf("%+v was not saved: %v\n%s", where.rt, reg.List(), out.String())
		}
	}

	// ── ONE OF THEM IS HELD ──
	held, err := reg.Lock(inPlace, time.Second)
	if err != nil {
		t.Fatalf("holding the in-place routine: %v", err)
	}
	defer held.Release()

	// ── AND THE OTHER ONE SAVES, WITHOUT WAITING FOR IT ──
	//
	// Measured against the bound rather than against a stopwatch: a save that had to wait
	// for the namesake would be refused at `SaveWithin`, and this has to finish well
	// inside it.
	d := aTerminal(reg, nil, "", &out, "notepad")
	dr := d.DraftOf("settings", aPlainDemonstration("z"), "notepad", "a test", Unstated)
	at := d.Collision(dr, ScopeAppAnywhere, "notepad")
	if !at.Exists || at.ID != RouteID(anywhere) {
		t.Fatalf("the collision is %+v, want the anywhere-in-application routine", at)
	}
	began := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(began)
		_, err := d.Keep(dr, ScopeAppAnywhere, "notepad", Consent{ID: at.ID,
			Source: at.Source, Recording: at.Recording})
		done <- err
	}()
	<-began
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("saving the namesake was refused while the other was held: %v",
				err)
		}
	case <-time.After(SaveWithin / 2):
		t.Fatalf("saving %s waited for %s, which is a different routine",
			RouteID(anywhere), RouteID(inPlace))
	}
	// AND THE ONE THAT WAS HELD IS UNTOUCHED, which is the other half: the two routines
	// are separate files and a save of one is not a save of the other.
	if !strings.Contains(mustRead(t, reg, inPlace), `"a"`) {
		t.Errorf("the held routine was written:\n%s", mustRead(t, reg, inPlace))
	}
}

// AND A PATH THAT IS NOT A ROUTINE IS NOT A PATH THAT IS FREE.
//
// A directory where the routine should be. It is occupied, a save must not write through it, and
// the demonstration beside it must not land either.
//
// WHAT THIS DOES NOT COVER, said because the probe behind it is worth keeping: `Collision` now
// treats only "not found" as absence, where every `Stat` error used to read as free. On Windows
// that branch cannot be reached by anything a test can arrange — a directory in the routine's
// place makes `Stat` SUCCEED (this case), and a file where a parent directory should be reports
// path-not-found, which `os.IsNotExist` answers true for. Both were measured. So the guard is
// correct, its mutation is recorded as unfalsifiable rather than as a kill, and it stays for the
// same reason `Watching.has` keeps its own: it is the honest expression of the rule at the point
// where the answer is "nobody knows".
func TestAPathThatIsNotARoutineIsNotReportedAsFree(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "notepad", Slug: "ordinary"}
	// A DIRECTORY WHERE THE ROUTINE SHOULD BE, which is the cheapest unreadable path
	// there is and needs no permissions a test cannot arrange: `Stat` succeeds, and
	// `ReadFile` and the write both refuse.
	if err := os.MkdirAll(reg.Path(rt), 0o755); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	d := aTerminal(reg, aPlainDemonstration("a"), "y\nc\n", &out, "notepad")
	if err := d.Learn("ordinary"); err != nil {
		t.Fatalf("learning: %v\n%s", err, out.String())
	}
	// IT IS STILL A DIRECTORY, which is the whole assertion: nothing was written through
	// it or around it.
	st, err := os.Stat(reg.Path(rt))
	if err != nil || !st.IsDir() {
		t.Errorf("the path was written over: %v %+v", err, st)
	}
	if rec, ok := reg.LoadRecording(rt); ok {
		t.Errorf("a demonstration was written beside a routine that was not "+
			"saved: %s", rec)
	}
	if strings.Contains(out.String(), "Learned") {
		t.Errorf("it said it learned something:\n%s", out.String())
	}
}

// mustRead is a saved routine's source, and a failure when it is not there.
func mustRead(t *testing.T, reg routes.Registry, rt routes.Route) string {
	t.Helper()
	raw, err := os.ReadFile(reg.Path(rt))
	if err != nil {
		t.Fatalf("reading %s: %v", reg.Path(rt), err)
	}
	return string(raw)
}

// ── A SCOPE THE RECORDING CANNOT BE PERFORMED UNDER ──────────────────────────

// AND NO SURFACE WRITES A ROUTINE THAT WOULD REFUSE EVERY PRESS.
//
// Measured before this: answering "only here" or "anywhere" for a recording whose presses check
// their own window saved exactly that, and `Unrunnable` — the same reader Studio greys the Play
// button with — reported it immediately.
func TestNoSurfaceSavesARoutineThatRefusesEveryPress(t *testing.T) {
	t.Setenv("MARCO_CV", "off")

	// THE TERMINAL IS NOT EVEN OFFERED THE SCOPES THAT WOULD NOT WORK, and is told why:
	// a person who expected "only here" needs to know why they are not being asked for it.
	for _, answers := range []struct {
		say string
		in  string
	}{
		{"asked for only-here", "y\nc\n"},
		{"asked for anywhere", "y\ng\n"},
		{"answered nothing", "y\n\n"},
	} {
		reg := routes.Registry{Dir: t.TempDir()}
		var out bytes.Buffer
		d := aTerminal(reg, aWindowCheckingDemonstration(), answers.in, &out, "arkascended")
		if err := d.Learn("ark graphics"); err != nil {
			t.Fatalf("%s: %v\n%s", answers.say, err, out.String())
		}
		if got := d.Unrunnable(); len(got) != 0 {
			t.Errorf("%s: saved a routine that refuses every press: %+v\n%s",
				answers.say, got, out.String())
		}
		// AND IT WENT WHERE IT CAN ACTUALLY RUN, which is the only scope left.
		if !reg.Has(routes.Route{App: "arkascended", Focus: true, Slug: "ark-graphics"}) {
			t.Errorf("%s: it is not saved where it can run: %+v\n%s",
				answers.say, reg.List(), out.String())
		}
		if !strings.Contains(out.String(), "checks the window it was made in") {
			t.Errorf("%s: nothing said why the other scopes were not offered:\n%s",
				answers.say, out.String())
		}
	}

	// AND THE UNATTENDED PATH CHOOSES A SCOPE THE DRAFT SUPPORTS.
	//
	// It defaulted to the application's in-place scope, which for this recording is the
	// one that cannot work. Nobody sees this one before it is written, so there is nothing
	// to correct it afterwards.
	reg := routes.Registry{Dir: t.TempDir()}
	var auto bytes.Buffer
	d := aTerminal(reg, aWindowCheckingDemonstration(), "", &auto, "arkascended")
	if err := d.LearnAuto("ark graphics"); err != nil {
		t.Fatalf("auto: %v\n%s", err, auto.String())
	}
	if got := d.Unrunnable(); len(got) != 0 {
		t.Errorf("the unattended path saved a routine that refuses every press: %+v\n%s",
			got, auto.String())
	}
	if !reg.Has(routes.Route{App: "arkascended", Focus: true, Slug: "ark-graphics"}) {
		t.Errorf("the unattended path saved it somewhere it cannot run: %+v\n%s",
			reg.List(), auto.String())
	}

	// ── THE CONTROL: AN ORDINARY RECORDING STILL GOES WHERE IT IS ASKED ──
	//
	// Without which every assertion above is satisfied by a surface that has stopped
	// honouring the scope anybody chooses.
	for _, want := range []struct {
		in string
		rt routes.Route
	}{
		{"y\nc\n", routes.Route{App: "notepad", Slug: "ordinary"}},
		{"y\nf\n", routes.Route{App: "notepad", Focus: true, Slug: "ordinary"}},
		{"y\ng\n", routes.Route{Slug: "ordinary"}},
	} {
		reg := routes.Registry{Dir: t.TempDir()}
		var out bytes.Buffer
		d := aTerminal(reg, aPlainDemonstration("a"), want.in, &out, "notepad")
		if err := d.Learn("ordinary"); err != nil {
			t.Fatalf("%q: %v\n%s", want.in, err, out.String())
		}
		if !reg.Has(want.rt) {
			t.Errorf("%q saved it as %+v, want %+v\n%s", want.in, reg.List(),
				want.rt, out.String())
		}
		if !strings.Contains(out.String(), "Learned") {
			t.Errorf("%q did not say it saved anything:\n%s", want.in, out.String())
		}
	}
}

// ── WHAT WAS CAPTURED IS PRESERVED, DELIBERATELY ─────────────────────────────

// AND NEITHER PATH STARTS STATING A MODE IT NEVER STATED.
//
// This is shared ORCHESTRATION and shared SAVE RULES, and it is NOT an identical capture
// configuration. Studio states coordinate mode to the recorder; the terminal and the widget state
// nothing, which leaves the hooks doing what the environment says — the behaviour every caller
// before Studio had. Moving them onto the shared pipeline must not change what they capture, and
// a routine that suddenly declared a mode would be claiming something about how it was recorded
// that nobody arranged.
func TestTheTerminalStatesNoModeAndStillDoesNot(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	for _, how := range []struct {
		say string
		run func(Deps) error
		in  string
	}{
		{"learn", func(d Deps) error { return d.Learn("ordinary") }, "y\nc\n"},
		{"auto", func(d Deps) error { return d.LearnAuto("ordinary") }, ""},
	} {
		reg := routes.Registry{Dir: t.TempDir()}
		var out bytes.Buffer
		d := aTerminal(reg, aPlainDemonstration("a"), how.in, &out, "notepad")
		told := &statedRecorder{Recorder: d.Rec}
		d.Rec = told
		if err := how.run(d); err != nil {
			t.Fatalf("%s: %v\n%s", how.say, err, out.String())
		}
		// THE RECORDER WAS TOLD `Inherited`, which is what "states nothing" means to the
		// hooks and is what these callers have always sent.
		if told.mode != recorder.Inherited {
			t.Errorf("%s: the recorder was told %v, want the inherited mode these "+
				"callers have always sent", how.say, told.mode)
		}
		// AND THE SAVED ROUTINE STATES NOTHING, which `ModeOf` reads back as unstated.
		src := sourceAt(t, reg, routes.Route{App: "notepad", Slug: "ordinary"})
		if strings.Contains(src, modeStated) {
			t.Errorf("%s: the saved routine states a mode it was never recorded "+
				"under:\n%s", how.say, src)
		}
		if got := ModeOf(src); got != Unstated {
			t.Errorf("%s: the saved routine reads back as %q, want unstated",
				how.say, got)
		}
	}
}

// statedRecorder records what capture mode it was told, and is otherwise the recorder it wraps.
type statedRecorder struct {
	recorder.Recorder
	mode recorder.Mode
	told bool
}

func (s *statedRecorder) Capture(m recorder.Mode) { s.mode, s.told = m, true }

// ── AND CANCEL WRITES NOTHING ────────────────────────────────────────────────

// WHICH IS THE WHOLE OF WHAT A DRAFT IS FOR.
//
// Stop and Save are two acts because there is something to cancel: the draft was never on disk,
// so dropping it is the entire operation. Three ways to end up with nothing written, and none of
// them may leave a file behind.
func TestNothingIsWrittenWhenNothingIsKept(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	for _, how := range []struct {
		say    string
		events []recorder.RecordedEvent
		in     string
	}{
		{"answered no at the save prompt", aPlainDemonstration("a"), "n\n"},
		{"nothing was demonstrated", []recorder.RecordedEvent{
			{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(10)}}, "y\nc\n"},
	} {
		reg := routes.Registry{Dir: t.TempDir()}
		var out bytes.Buffer
		d := aTerminal(reg, how.events, how.in, &out, "notepad")
		if err := d.Learn("ordinary"); err != nil {
			t.Fatalf("%s: %v\n%s", how.say, err, out.String())
		}
		if got := reg.List(); len(got) != 0 {
			t.Errorf("%s: wrote %+v\n%s", how.say, got, out.String())
		}
		if entries, err := os.ReadDir(reg.Dir); err == nil && len(entries) != 0 {
			t.Errorf("%s: left %d thing(s) in the library", how.say, len(entries))
		}
	}
}

// sourceAt is a saved routine's source, or a failure naming where it looked.
func sourceAt(t *testing.T, reg routes.Registry, rt routes.Route) string {
	t.Helper()
	raw, err := os.ReadFile(reg.Path(rt))
	if err != nil {
		t.Fatalf("reading %s: %v", reg.Path(rt), err)
	}
	return string(raw)
}

// AND A REBUILD TAKES THE SAME LOCK, OR THE LOCK PROTECTS NOTHING.
//
// A lock protects the writers that take it. `Keep` took it and `Deps.Replace` — the path behind
// Studio's Simplify and its generation rebuilds — did not, so the two could still lose each
// other's work: the rebuild read the digests while a `Keep` was deciding, and wrote over what
// that `Keep` committed, having compared against the version before it. Not a smaller gap than
// the one the lock closed; the same gap, from the side that had not joined the protocol.
//
// Deterministic, like the two-`Keep` case: `Keep` pauses inside its lock after validating, and
// the rebuild CANNOT proceed, so nothing here depends on timing to be true.
func TestARebuildWaitsForASaveAndThenRefusesStaleConsent(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	d, rt := aSavedRoutineWithItsRecording(t)
	statingNoGeneration(t, d, rt)
	v1 := mustReadFile(t, d.Reg.Path(rt))

	// A REVIEW IS PREPARED AND LEFT OPEN, which is a person looking at the comparison.
	p, err := d.Prepare(rt, RebuildCurrent)
	if err != nil {
		t.Fatalf("preparing a version: %v", err)
	}
	if !p.Replaceable() {
		t.Fatalf("the preparation offers nothing to replace with (refused: %q)",
			p.Refused)
	}

	// ── AND A SAVE OF THE SAME ROUTINE IS IN FLIGHT, PAUSED AFTER IT VALIDATED ──
	saving := d
	validated, resume := make(chan struct{}), make(chan struct{})
	saving.AfterValidating = func() {
		close(validated)
		<-resume
	}
	dr := saving.DraftOf(p.Name, aPlainDemonstration("k"), "arkascended", "a test",
		Unstated)
	at := saving.Collision(dr, ScopeAppAnywhere, "arkascended")
	if !at.Exists {
		t.Fatalf("there is nothing at the target to replace: %+v", at)
	}
	doneSave := make(chan error, 1)
	go func() {
		_, err := saving.Keep(dr, ScopeAppAnywhere, "arkascended",
			Consent{ID: at.ID, Source: at.Source, Recording: at.Recording})
		doneSave <- err
	}()
	<-validated

	// ── THE REBUILD IS TAKEN WHILE THAT SAVE HOLDS THE ROUTE ──
	//
	// Its digests are still current: the save has validated and written nothing. The only
	// thing wrong with this rebuild is that it is second.
	doneRebuild := make(chan error, 1)
	go func() { doneRebuild <- d.Replace(p) }()
	select {
	case err := <-doneRebuild:
		t.Fatalf("the rebuild wrote while a save held the route: %v", err)
	case <-time.After(250 * time.Millisecond):
	}
	if now := mustReadFile(t, d.Reg.Path(rt)); now != v1 {
		t.Fatalf("something was written while the save was paused:\n%s", now)
	}

	// ── THE SAVE FINISHES ──
	close(resume)
	if err := <-doneSave; err != nil {
		t.Fatalf("the save refused: %v", err)
	}
	afterSave := mustReadFile(t, d.Reg.Path(rt))
	if afterSave == v1 {
		t.Fatal("the save wrote nothing, so the rest of this says nothing")
	}

	// ── AND THE REBUILD REVALIDATES AGAINST WHAT THE SAVE LEFT ──
	errRebuild := <-doneRebuild
	if errRebuild == nil {
		t.Fatal("the rebuild replaced a routine that had been saved since the review " +
			"was built")
	}
	if !strings.Contains(errRebuild.Error(), "saved again since the version you reviewed") {
		t.Errorf("the refusal does not say what changed: %v", errRebuild)
	}
	if now := mustReadFile(t, d.Reg.Path(rt)); now != afterSave {
		t.Errorf("the save's version was written over by the rebuild:\n%s", now)
	}

	// ── THE CONTROL: A REBUILD PREPARED AGAINST WHAT IS THERE NOW GOES THROUGH ──
	//
	// Without which every refusal above is satisfied by a rebuild that has stopped
	// working. It also proves the holds were given back: a leaked one would have made
	// this wait out SaveWithin and refuse.
	fresh, err := d.Prepare(rt, RebuildCurrent)
	if err != nil {
		t.Fatalf("preparing again: %v", err)
	}
	if !fresh.Replaceable() {
		t.Skipf("the routine as the save left it offers nothing to rebuild (refused: "+
			"%q), so the control cannot be made", fresh.Refused)
	}
	if err := d.Replace(fresh); err != nil {
		t.Fatalf("a rebuild prepared against what is there now was refused: %v", err)
	}
	// AND WHAT IS ON DISK IS THE VERSION THAT WAS REVIEWED.
	//
	// Asserted against the review rather than against "it changed": a rebuild of a
	// routine the save had already written under current rules can legitimately come out
	// byte-identical, and the promise is that what somebody looked at is what got
	// written.
	if now := mustReadFile(t, d.Reg.Path(rt)); now != fresh.Now {
		t.Errorf("what was written is not the version that was reviewed:\n%s", now)
	}
}

// mustReadFile is one file's contents, and a failure naming it.
func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(raw)
}
