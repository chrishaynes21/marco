package main

// RECORD → STOP → REVIEW → SAVE, with a visible button at each end.
//
// # What this is, and what it is not
//
// This is the RECORDER-FIRST path: `internal/recorder` watches the keyboard and mouse,
// `internal/simplify` makes steps of what it saw, and `internal/codegen` writes legal Marco. It
// is the widget recorder Marco shipped with, reached from a window for the first time.
//
// It is NOT the path in learn.go. That one spawns `director demonstrate-visual`, which looks at
// the screen, correlates each press to a control it can see, and refuses to write a coordinate
// as a control's identity. Both exist on purpose and they answer different questions:
//
//	demonstrate-visual   what did they press, and can Marco recognise it again
//	this                 what did they DO, in order, with the real pauses
//
// The second is the one the owner asked for — "a cleaner way to end basic recordings like this"
// — and it is the one that works on a menu drawn over a live 3D world, where recognition has no
// answer. See [[ADR-248-replaying-a-demonstrated-layout]].
//
// # Why this runs in Studio's own process
//
// Every other child Studio starts is a child: the runner, the visual recorder, the performer.
// This one is not, and the reason is the whole point of the backend it drives.
//
// `orchestrator.Record` returns a draft and WRITES NOTHING. That is what makes Stop, Cancel and
// Save three separate acts rather than one — there is something to cancel because nothing is on
// disk yet. A draft held in a child process could not do that: it would have to be written
// somewhere for the parent to review it, which is the thing it exists not to do, or it would
// need a protocol of its own, which is a second Studio interface. So the draft lives here, in
// the field below, until somebody saves it or drops it.
//
// The cost is that the low-level hooks are installed in the window's own process. That is a real
// cost and it is why `internal/recorder` has the discipline it has — callbacks return fast, the
// pump BLOCKS rather than polls, and shutdown wakes the pump from the installing thread. The
// overlay has run hooks inside a GUI process for two milestones on the same terms.
//
// # Why Studio names itself to the recorder
//
// Because pressing a button in a window means bringing that window forward and clicking in it,
// and the hooks record that like any other click. Left in, it becomes a step: every replay would
// raise Studio and press Stop. Worse, the switch that brought Studio forward decides which
// application the routine is filed under, and a glance at Studio mid-demonstration costs the
// click before it.
//
// So Studio passes its own process id — `os.Getpid`, which is the authoritative answer and needs
// no guessing — and its own app name from `winctx.Self`, which is the same transform the app
// poller's samples come from. The backend does the rest, and
// [[ADR-249-ending-a-recording-is-not-part-of-the-recording]] is why it takes five rules to do.

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// capture is one recorder-first recording: in flight, or a draft waiting to be looked at.
type capture struct {
	name string
	// stop is closed by the Stop button, and `ended` makes closing it idempotent — a
	// second press of a button whose first press is still being served must not panic.
	stop  chan struct{}
	ended sync.Once
	// done closes when `Record` has returned, which is when there is a draft to review.
	done chan struct{}
	// draft is what was recorded, unsaved. Nil while recording and after a failure.
	draft *orchestrator.Draft

	// saved is where it went, once Save has written it. Empty until then.
	saved string
}

// Recording is what the page shows about a recorder-first recording, during and after.
type Recording struct {
	Name   string `json:"name,omitempty"`
	Active bool   `json:"active"`
	// Review says a draft is waiting: the capture has ended and nothing is on disk.
	// This is the state the Stop button exists to reach.
	Review bool `json:"review,omitempty"`
	// Application is what was OBSERVED, and a proposal for the scope rather than the
	// scope itself. `Save` is told which scope to use; see SaveDraft.
	Application string `json:"application,omitempty"`
	// StoppedBy is how the capture ended — the button, or the key — because both stay
	// live and a person is owed the difference when the one they did not press won.
	StoppedBy string `json:"stopped_by,omitempty"`
	// Pressable is how many steps send input, which is what makes a draft worth saving.
	Pressable int `json:"pressable,omitempty"`
	// Steps is the draft in plain words, one line each, in order.
	Steps []string `json:"steps,omitempty"`
	// StopKey names the optional shortcut, so the button is not the only way out and the
	// other way is written where somebody can see it.
	StopKey string `json:"stop_key,omitempty"`
	Said    string `json:"said,omitempty"`
	Failed  bool   `json:"failed,omitempty"`
	// Saved is the path the routine went to, once Save has written one.
	Saved string `json:"saved,omitempty"`
	// Incomplete says some press here asked to be checked against the window it was
	// made in and a measurement it needs is missing, so a routine saved from this will
	// REFUSE to press rather than press somewhere nobody checked.
	//
	// Saving is still allowed: the draft is a true record of what somebody did, and
	// throwing it away would lose the demonstration as well as the measurement. What is
	// not allowed is saving it quietly as something that runs — so the page is told, the
	// Save message repeats it, and the saved route refuses.
	Incomplete bool `json:"incomplete,omitempty"`
	// Missing is that in words: how much of the recording cannot be checked, which
	// measurements are absent, and whether recording it again would change that.
	Missing string `json:"missing,omitempty"`
	// Scope is the one scope this recording can be saved under, when the choice is not
	// the person's, and Brings is how to say it: "Runs in Ark. Marco brings it forward
	// before playback."
	//
	// # Why a choice is withdrawn rather than defaulted
	//
	// A press that asked to be checked against the window it was made in needs that
	// window brought forward, and two of the three scopes promise to switch nothing —
	// their routes carry no Activate, so nothing binds a window and every press
	// refuses. Offering them would be offering two ways to save a routine that cannot
	// press anything. Empty for a recording with no guarded press, which keeps all
	// three.
	Scope  string `json:"scope,omitempty"`
	Brings string `json:"brings,omitempty"`
	// Unsupported says this recording cannot be saved as a routine at all, and why.
	//
	// Different from Incomplete, which is a routine that saves and then refuses: this one
	// does not save. A step whose failure cannot be made to stop the routine — a
	// right-click, a drag — would leave whatever follows it running after a press that
	// did not happen, which is the guard's promise broken by the one step nobody could
	// wrap. Said here because a generator error arriving at a button press is a worse way
	// to learn it.
	Unsupported string `json:"unsupported,omitempty"`
}

// StartRecording begins a recorder-first capture under `name`.
//
// Returns "" when the recorder is running, or the sentence to show the person. Nothing is
// brought forward and nothing counts down: the person switches to whatever they are
// demonstrating when they like, and that switch is recorded as the step that performs it.
func (a *Acts) StartRecording(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "say what to call it first — the name is what you will look for later"
	}
	// THE SAME NAME RULE AS A DEMONSTRATION, for the same reason: both become
	// declarations in the play that composes them. Said here rather than discovered
	// from a compile error two steps later.
	for _, c := range a.Saved() {
		if strings.EqualFold(c.Name, name) {
			return fmt.Sprintf("there is already an Act called %q, and a "+
				"recording cannot share its name — call this something "+
				"else", c.Name)
		}
	}

	a.mu.Lock()
	switch {
	case a.closing:
		a.mu.Unlock()
		return "Studio is closing, so nothing new is started"
	case a.busy:
		a.mu.Unlock()
		return "a run is in flight; one thing at a time on one screen"
	case a.recordingLocked():
		a.mu.Unlock()
		return "something is already being recorded"
	// A DRAFT NOBODY HAS DEALT WITH YET, which is one that was recorded and NOT
	// SAVED. The `saved` half matters: without it a successful Save left the window
	// holding its own finished draft and refusing to record anything else, so Studio
	// recorded exactly once per launch unless somebody pressed Cancel over a routine
	// they had just kept.
	case a.capture != nil && a.capture.draft != nil && a.capture.saved == "":
		a.mu.Unlock()
		return "there is a recording waiting to be saved or cancelled — deal " +
			"with that one first"
	}
	c := &capture{name: name, stop: make(chan struct{}), done: make(chan struct{})}
	a.capture = c
	a.mu.Unlock()

	// AND THE BACKEND, WITH STUDIO NAMED TO IT. See the note at the top of this file
	// for why both terms are passed and why the process id is the one that holds.
	d := orchestrator.Deps{
		Reg:      a.reg(),
		Rec:      a.recorderNow(),
		Out:      &strings.Builder{},
		App:      winctx.Active,
		StopKey:  os.Getenv("MARCO_STOP_KEY"),
		ArgKey:   os.Getenv("MARCO_ARG_KEY"),
		Owner:    winctx.Self(),
		OwnerPID: uint32(os.Getpid()),
		StopWhen: c.stop,
	}
	// THE RECORDER IS STARTED HERE, SYNCHRONOUSLY, so a refusal reaches the button that
	// asked for it. Started in a goroutine it would reach the person at Stop instead —
	// a window that said "recording", took their whole demonstration, and then told
	// them it had never begun. See orchestrator.Begin.
	shot, err := d.Begin(name)
	if err != nil {
		a.mu.Lock()
		a.capture = nil
		a.mu.Unlock()
		return err.Error()
	}
	// AND THE TRIAL HAS BEGUN. Noted once the recorder is watching, so the journal
	// records a recording that started rather than one that was asked for.
	//
	// THE SAME TWO NOTES THE VISUAL RECORDER WRITES, and no application or duration
	// because this path chooses neither. It does mean the journal does not say WHICH
	// recorder a note came from, which is a real gap and is recorded as one: the
	// alternative was a new note kind and a reader to match, and no record at all
	// while that was built. The sentence on the closing note says which path it was.
	a.trial.recording(name, "", 0)
	go func() {
		dr := shot.Finish()
		a.mu.Lock()
		c.draft = dr
		close(c.done)
		a.mu.Unlock()
	}()
	return ""
}

// StopRecording ends the capture and opens review.
//
// It does NOT save. That is the whole reason this is a separate button from the one below: a
// person who has just finished showing Marco something is entitled to look at what it got before
// deciding whether to keep it. Routing this through `LearnAuto` would have made Stop and Save one
// event, because the routine would already be on disk.
func (a *Acts) StopRecording() string {
	a.mu.Lock()
	c := a.capture
	a.mu.Unlock()
	if c == nil {
		return "nothing is being recorded"
	}
	select {
	case <-c.done:
		return "that recording has already finished"
	default:
	}
	// CLOSED ONCE, however many times the button is pressed. The backend races this
	// against the stop gesture and takes whichever arrives first.
	c.ended.Do(func() { close(c.stop) })
	// AND WE WAIT FOR THE DRAFT, because "Stop" means review is open when it returns.
	// Bounded: the recorder unhooks from its own thread and a stop that never lands is
	// a window that would otherwise hang on a button press.
	select {
	case <-c.done:
	case <-time.After(5 * time.Second):
		return "the recorder did not finish within five seconds of being asked"
	}
	// AND NOTHING TO REPORT. A capture that has begun cannot fail: starting is the only
	// thing that can, and that was answered at the button. See orchestrator.Finish.
	return ""
}

// CancelRecording throws the recording away.
//
// Whether it is still running or already in review: both are the same answer, because neither has
// written anything. Nothing is deleted here — there is nothing to delete, and that is the claim
// worth testing.
func (a *Acts) CancelRecording() string {
	a.mu.Lock()
	c := a.capture
	a.capture = nil
	saved := ""
	if c != nil {
		saved = c.saved
	}
	a.mu.Unlock()
	if c == nil {
		return "there is nothing to cancel"
	}
	if saved != "" {
		// ALREADY KEPT, so this is putting the card away and not throwing
		// anything out. Said as nothing rather than as a cancellation: a journal
		// line reading "cancelled" beside a routine that is on disk would be the
		// evidence lying about what happened.
		return ""
	}
	// AND A CAPTURE STILL RUNNING IS ENDED ON THE WAY OUT, or the hooks outlive the
	// recording that installed them. Not waited for: the draft it produces has already
	// been let go of, and the goroutine writes only to its own `capture`.
	c.ended.Do(func() { close(c.stop) })
	a.trial.recorded(c.name, "Cancelled — the recorder-first recording was thrown "+
		"away and nothing was written.", "", false, Pictured{})
	return ""
}

// SaveDraft persists the reviewed recording, under the scope it is GIVEN.
//
// `scope` is "app", "app-anywhere" or "everywhere", and `app` is the application the scope is
// about — empty meaning the one that was observed. Stated rather than inferred, because this
// window can ask the question and a routine quietly rescoped to everywhere works in places
// nobody was told about.
func (a *Acts) SaveDraft(scope, app string) string {
	a.mu.Lock()
	c := a.capture
	a.mu.Unlock()
	if c == nil {
		return "there is no recording to save"
	}
	select {
	case <-c.done:
	default:
		return "that recording is still running — press Stop first"
	}
	a.mu.Lock()
	dr, already := c.draft, c.saved
	a.mu.Unlock()
	if dr == nil {
		return "there is no recording to save"
	}
	if already != "" {
		// SAVED ONCE. A second Save would write the same routine over itself
		// under whatever scope was selected the second time, which is a routine
		// quietly rescoped by a double-click.
		return "that recording has already been saved — " + already
	}
	want, why := wantedScope(scope)
	if why != "" {
		return why
	}
	// ── AND A SCOPE ITS PRESSES CANNOT BE PERFORMED UNDER IS REFUSED, NOT REWRITTEN ──
	//
	// Quietly saving it as something else would be this window deciding where somebody's
	// routine works — the thing `wantedScope` exists to prevent — and saving it as asked
	// would write a routine that refuses every press. So it says which scope it needs
	// and why, and the person asks again.
	if only := orchestrator.ScopeFor(dr); only != "" && want != only {
		where := dr.App
		if where == "" {
			where = "the application it was recorded in"
		}
		return fmt.Sprintf("every press in this recording checks the window it was "+
			"made in, and %q brings no window forward — so there would be "+
			"nothing for those presses to be checked against and every one of "+
			"them would refuse. Save it to run in %s with Marco bringing it "+
			"forward.", want, where)
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	rt, err := d.Save(dr, want, strings.TrimSpace(app))
	if err != nil {
		return err.Error()
	}
	a.mu.Lock()
	c.saved = a.reg().Path(rt)
	where := c.saved
	a.mu.Unlock()
	note := fmt.Sprintf("Saved %q from the recorder-first path (%s): %d step(s), "+
		"%d of which press something — %s", dr.Name, want, len(dr.Steps),
		dr.Pressable(), where)
	next := "Run it, or build an Act over it."
	// AND THE JOURNAL SAYS WHEN WHAT WAS SAVED CANNOT RUN. A trial note reading
	// "Saved, run it" about a routine that will refuse every press is a note that will
	// be believed later by somebody reading back what happened.
	if why := whyItWillNotRun(dr.Steps); why != "" {
		note += ". " + why
		next = "Record it again — this one will refuse to press."
	}
	a.trial.recorded(dr.Name, note, next, false, Pictured{})
	return ""
}

// wantedScope reads the page's word for a scope, or says what the choices are.
//
// Refused rather than defaulted: a scope nobody recognised becoming "only in this application"
// is the window choosing where somebody's routine works and not telling them.
func wantedScope(scope string) (orchestrator.Scope, string) {
	switch orchestrator.Scope(strings.TrimSpace(scope)) {
	case orchestrator.ScopeApp:
		return orchestrator.ScopeApp, ""
	case orchestrator.ScopeAppAnywhere:
		return orchestrator.ScopeAppAnywhere, ""
	case orchestrator.ScopeEverywhere:
		return orchestrator.ScopeEverywhere, ""
	}
	return "", fmt.Sprintf("say where it should work: %q, %q or %q",
		orchestrator.ScopeApp, orchestrator.ScopeAppAnywhere,
		orchestrator.ScopeEverywhere)
}

// Recording is the capture in flight, or the draft waiting to be looked at.
func (a *Acts) Recording() Recording {
	a.mu.Lock()
	c := a.capture
	a.mu.Unlock()
	if c == nil {
		return Recording{}
	}
	out := Recording{Name: c.name,
		StopKey: recorder.ParseStopKey(os.Getenv("MARCO_STOP_KEY")).Label()}
	select {
	case <-c.done:
	default:
		out.Active = true
		return out
	}
	a.mu.Lock()
	dr, saved := c.draft, c.saved
	a.mu.Unlock()
	out.Saved = saved
	if dr == nil {
		out.Failed = true
		out.Said = "nothing was recorded"
		return out
	}
	out.Review = saved == ""
	out.Application, out.StoppedBy = dr.App, dr.StoppedBy
	out.Pressable = dr.Pressable()
	out.Steps = inWords(dr.Steps)
	if out.Pressable == 0 {
		out.Failed = true
		out.Said = "nothing in this recording presses anything, so there is " +
			"nothing to save. Record it again and use the mouse or the " +
			"keyboard on the thing you are showing Marco."
		return out
	}
	out.Said = fmt.Sprintf("%d step(s), %d of which press something.",
		len(dr.Steps), out.Pressable)
	// AND WHETHER WHAT IT MEASURED IS ENOUGH TO RUN ON. Before Save, not after: the
	// point of a review is that the person decides knowing this.
	if why := whyItWillNotRun(dr.Steps); why != "" {
		out.Incomplete, out.Missing = true, why
	}
	// AND WHETHER IT CAN BE SAVED AT ALL. Before Save, for the same reason as above.
	if why := codegen.Unpropagatable(dr.Steps); why != "" {
		out.Unsupported = fmt.Sprintf("This recording contains %s. A routine whose "+
			"presses are checked against the window they were made in has to "+
			"stop when one of them does not happen, and this version cannot "+
			"make %s do that — so it cannot be saved yet. Record it again "+
			"using ordinary left-clicks and the keyboard.", why, why)
	}
	// AND HOW IT WILL BE PERFORMED, when that is not a choice. See Recording.Scope.
	if only := orchestrator.ScopeFor(dr); only != "" {
		out.Scope = string(only)
		where := dr.App
		if where == "" {
			where = "the application it was recorded in"
		}
		out.Brings = fmt.Sprintf("Runs in %s. Marco brings it forward before "+
			"playback.", where)
	}
	if saved != "" {
		out.Said = fmt.Sprintf("Saved %q — %s", c.name, saved)
		if out.Incomplete {
			out.Said += ". It will refuse to press until it is recorded again."
		}
	}
	return out
}

// inWords is the draft as a person reviewing it reads it, one line per step.
//
// Plain words and not the generated Marco, deliberately. The source depends on the scope, and
// the scope is chosen in the same panel this is shown in — so a preview rendered before that
// choice would be a preview of something else. Showing the Marco for the scope CURRENTLY
// selected, regenerated when it changes, is the better review and is the next thing to build
// here; it is not this, and a preview that drifts from what gets saved is worse than words.
func inWords(steps []macroir.Step) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, oneStep(s))
	}
	return out
}

// oneStep is one step in words.
func oneStep(s macroir.Step) string {
	switch s.Kind {
	case macroir.StepActivate:
		return "bring " + s.Text + " to the front"
	case macroir.StepClick:
		where := fmt.Sprintf("%d, %d", s.X, s.Y)
		if s.WinRel {
			// AND WHERE IT WAS INSIDE THE WINDOW, when that is what was
			// recorded. The basis matters to somebody reading this: a point
			// in the window survives the window moving and a point on the
			// desktop does not.
			where = fmt.Sprintf("%d, %d in the window", s.RelX, s.RelY)
		}
		button := s.Button
		if button == "" {
			button = "left"
		}
		return fmt.Sprintf("%s-click at %s%s", button, where, inWindow(s))
	case macroir.StepDrag:
		return fmt.Sprintf("drag from %d, %d to %d, %d", s.X, s.Y, s.X2, s.Y2)
	case macroir.StepKey:
		if s.Text != "" {
			return fmt.Sprintf("type %q", s.Text)
		}
		if s.Count > 1 {
			return fmt.Sprintf("press %s %d times", s.Key, s.Count)
		}
		return "press " + s.Key
	case macroir.StepKeyDown:
		return "hold " + s.Key
	case macroir.StepKeyUp:
		return "let go of " + s.Key
	case macroir.StepType:
		return fmt.Sprintf("type %q", s.Text)
	case macroir.StepWait:
		return fmt.Sprintf("wait %dms", s.Ms)
	}
	// AND ANYTHING ELSE BY ITS OWN NAME, rather than silently as nothing. A step this
	// has no sentence for is still a step that will run, and a review that omitted it
	// would be a review of a shorter routine than the one being saved.
	return string(s.Kind)
}

// recorderNow is the input recorder, or whatever a test put in its place.
func (a *Acts) recorderNow() recorder.Recorder {
	if a.recording != nil {
		return a.recording()
	}
	return recorder.New()
}

// recordingLocked says something is being recorded, by EITHER recorder. The caller holds a.mu.
//
// One predicate, because there are now two kinds of recording and four places that have to
// refuse while one is running — a run, a Scene deleted, everything forgotten, and a second
// recording started. Each of those asked `a.learn != nil && !a.learn.over` directly, so a second
// kind of recording would have been invisible to all of them: a run started while the hooks are
// watching presses Marco's own clicks into somebody's draft, and the draft would record them as
// the person's.
func (a *Acts) recordingLocked() bool {
	if a.learn != nil && !a.learn.over {
		return true
	}
	if c := a.capture; c != nil {
		select {
		case <-c.done:
		default:
			return true
		}
	}
	return false
}

// inWindow is what the press measured about its window, as a reviewer reads it, or "" when it
// measured nothing.
//
// Shown because it is what the saved route will carry and what a run will check against. A
// review that listed only the coordinate would be hiding the terms that decide whether the
// coordinate is ever trusted — and a person looking at a routine recorded in a window they have
// since resized is owed the chance to see that before they keep it.
func inWindow(s macroir.Step) string {
	if s.RelTo == "" {
		return ""
	}
	var said []string
	if s.WinW > 0 && s.WinH > 0 {
		said = append(said, fmt.Sprintf("%d×%d", s.WinW, s.WinH))
	}
	if s.Dpi > 0 {
		said = append(said, fmt.Sprintf("%d dpi", s.Dpi))
	}
	if s.Process != "" {
		said = append(said, s.Process)
	}
	// ── AND WHAT IT ASKED FOR AND DID NOT GET ──
	//
	// A press that declared the window and is missing a measurement will REFUSE at run
	// time rather than press, and this is where somebody finds that out while they are
	// still in front of the screen. Named term by term, because "this one cannot be
	// checked" tells a person nothing about whether doing it again would help and the
	// name of the missing measurement does.
	if lacks := notMeasured(s); len(lacks) > 0 {
		said = append(said, "cannot be checked — "+andAll(lacks)+
			" was not measured")
	}
	if len(said) == 0 {
		return ""
	}
	return " (" + strings.Join(said, ", ") + ")"
}

// notMeasured is what a press asked to be checked against and did not get, shortest name first
// in the order the guard reads them.
//
// A press states the window it is about whether or not the window could be measured, because the
// basis is what the press MEANS and not one of the measurements. That is deliberate and it is
// what makes this function necessary: the alternative — dropping the basis when a measurement
// failed — turned the press into an ordinary point, and an ordinary point is pressed at its
// recorded coordinate having checked nothing. A failed measurement means guarded replay is
// unavailable for that press, so the person has to be told, here, before they keep it.
func notMeasured(s macroir.Step) []string {
	if s.Kind != macroir.StepClick || s.RelTo == "" {
		return nil
	}
	var out []string
	if !s.WinRel {
		out = append(out, "where in the window the press went")
	}
	if s.WinW <= 0 || s.WinH <= 0 {
		out = append(out, "how big the window was")
	}
	if s.Dpi <= 0 {
		out = append(out, "what scale it was drawn at")
	}
	if s.Process == "" {
		out = append(out, "which program owned it")
	}
	return out
}

// whyItWillNotRun is the draft-level account of an incomplete recording: how much of it cannot be
// checked, which measurements are missing, and whether recording it again would change that.
//
// The last part is the one somebody actually needs. A window that would not describe itself this
// time, or a program that had exited before the recording closed, is a moment that passed — do it
// again with the thing running in front of you. A Windows with no `GetDpiForWindow` is not a
// moment; no press on this machine will ever carry a scale, so a routine that checks the window
// it was made in is not available here at all. Telling somebody to try again when nothing about
// trying again can change the answer is the worse of those two mistakes.
func whyItWillNotRun(steps []macroir.Step) string {
	presses, lacks := 0, map[string]bool{}
	for _, s := range steps {
		l := notMeasured(s)
		if len(l) == 0 {
			continue
		}
		presses++
		for _, one := range l {
			lacks[one] = true
		}
	}
	if presses == 0 {
		return ""
	}
	var missing []string
	for _, one := range []string{
		"where in the window the press went",
		"how big the window was",
		"what scale it was drawn at",
		"which program owned it",
	} {
		if lacks[one] {
			missing = append(missing, one)
		}
	}
	repair := "Record it again with the program running and in front of you, and " +
		"the measurements should arrive."
	if lacks["what scale it was drawn at"] && !canReadWindowScale() {
		repair = "This computer's Windows is too old to report a window's scale, " +
			"so recording it again will not change that: a routine that " +
			"checks the window it was made in cannot be made on this machine."
	}
	press := "press"
	if presses > 1 {
		press = "presses"
	}
	return fmt.Sprintf("%d %s here asked to be checked against the window they "+
		"were made in, and %s was not measured. Saving this keeps the "+
		"recording, and it will REFUSE to press rather than press somewhere "+
		"nobody checked. %s", presses, press, andAll(missing), repair)
}

// andAll is a list a person reads: "a", "a and b", "a, b and c".
func andAll(of []string) string {
	switch len(of) {
	case 0:
		return ""
	case 1:
		return of[0]
	}
	return strings.Join(of[:len(of)-1], ", ") + " and " + of[len(of)-1]
}

// canReadWindowScale is whether this Windows can be asked a window's DPI at all.
//
// A variable so both sentences above can be read by a test. Only one of them is reachable on any
// given machine — this one can read a scale, so the "record it again" branch is the only live one
// here — and the branch that cannot be reached is the branch that would be wrong if nobody
// checked it: telling somebody to demonstrate it again when nothing about demonstrating it again
// can change the answer.
var canReadWindowScale = winctx.CanReadWindowScale

// Unrunnable is what the page shows about a saved routine whose presses can never be checked.
type Unrunnable struct {
	Name      string `json:"name"`
	App       string `json:"app,omitempty"`
	Resavable bool   `json:"resavable,omitempty"`
}

// Unrunnable lists the saved routines whose presses ask to be checked against the window they
// were made in, under a scope that brings no window forward.
//
// They exist because the guarded contract arrived before the rule that a guarded recording is
// saved to bring its application forward. A routine saved in between is on disk, reads like a
// routine, and cannot press anything — and the only thing that says so at run time is the
// refusal itself, one press at a time, which is a person watching a routine do nothing while
// being told why four times.
func (a *Acts) Unrunnable() []Unrunnable {
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	var out []Unrunnable
	for _, u := range d.Unrunnable() {
		out = append(out, Unrunnable{Name: u.Name, App: u.Route.App,
			Resavable: u.Resavable})
	}
	return out
}

// Resave writes one of them again from its own demonstration, under the scope its presses need.
//
// Explicit, and named for what it does: nothing repairs itself behind somebody's back, and a
// routine quietly rescoped is the thing `wantedScope` refuses to do at Save.
func (a *Acts) Resave(name string) string {
	a.mu.Lock()
	recording := a.recordingLocked()
	a.mu.Unlock()
	if recording {
		return "Marco is recording a demonstration; one thing at a time"
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	for _, u := range d.Unrunnable() {
		if !strings.EqualFold(u.Name, name) {
			continue
		}
		rt, err := d.Resave(u)
		if err != nil {
			return err.Error()
		}
		a.trial.recorded(u.Name, fmt.Sprintf("Saved %q again so its presses have "+
			"a window to be checked against: it now brings %s forward before "+
			"playback — %s", u.Name, rt.App, a.reg().Path(rt)),
			"Run it.", false, Pictured{})
		return ""
	}
	return fmt.Sprintf("there is no routine called %q that needs saving again", name)
}
