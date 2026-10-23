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
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	rt, err := d.Save(dr, want, strings.TrimSpace(app))
	if err != nil {
		return err.Error()
	}
	a.mu.Lock()
	c.saved = a.reg().Path(rt)
	where := c.saved
	a.mu.Unlock()
	a.trial.recorded(dr.Name, fmt.Sprintf("Saved %q from the recorder-first path "+
		"(%s): %d step(s), %d of which press something — %s", dr.Name,
		want, len(dr.Steps), dr.Pressable(), where),
		"Run it, or build an Act over it.", false, Pictured{})
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
	if saved != "" {
		out.Said = fmt.Sprintf("Saved %q — %s", c.name, saved)
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
		return fmt.Sprintf("%s-click at %s", button, where)
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
