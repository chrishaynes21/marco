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
