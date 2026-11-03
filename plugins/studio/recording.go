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
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// studioCaptures is what this window records, stated ONCE.
//
// Used both to tell the recorder and to tell the person, because two literals would be two
// statements of one fact — and the day they disagreed, the window would be describing a recording
// it had not started. See StartRecording for why it is stated at all.
const studioCaptures = orchestrator.Coordinates

// capture is one recorder-first recording: in flight, or a draft waiting to be looked at.
type capture struct {
	name string
	// window is the chosen window as somebody reading the card would name it, and shot
	// is the capture in flight, which is what can be ASKED whether it is paused.
	//
	// The page redraws on a poll and does not read the event channel, so it cannot see
	// the boundary markers the terminal prints from: it asks instead. See
	// orchestrator.Capture.Paused.
	window string
	shot   *orchestrator.Capture
	// mode is what the recorder was told to capture when this one started.
	//
	// Kept beside the draft rather than read back off it, because the page asks about a
	// recording that is still running and has no draft yet — and "what is being captured"
	// is the question somebody has while they are demonstrating, not afterwards.
	mode orchestrator.RecordedAs
	// again is the saved routine this is a second recording of, or nil.
	//
	// It carries NO authority to replace anything. It is what the window has to know to
	// say which routine somebody set out to re-record — and to notice when the
	// demonstration they then made was in a different application, which would quietly
	// make a second routine rather than the new version of the one they chose. See
	// again.go.
	again *reRecording
	// stop is closed by the Stop button, and `ended` makes closing it idempotent — a
	// second press of a button whose first press is still being served must not panic.
	stop  chan struct{}
	ended sync.Once
	// done closes when `Record` has returned, which is when there is a draft to review.
	done chan struct{}
	// wasDraft is the recording as it was DEMONSTRATED, kept once it has been
	// simplified so there is a way back, and nil until then.
	//
	// The original rather than the previous version: simplifying twice must still have
	// one thing to return to, and that thing is what the person did.
	wasDraft *orchestrator.Draft
	// wasSupplied is `supplied` as it stood for `wasDraft`, kept WITH it.
	//
	// Because a draft can be marked BEFORE it is simplified — a briskly typed run folds
	// into one step without Simplify, so there is something to pick on an ordinary
	// recording — and then the thing kept as the way back is a draft with marks ON it.
	// Restoring the draft and clearing the marks would leave the card silent about
	// placeholders that are in the steps in front of somebody, which is the same class of
	// wrongness as warning about a mark that is gone, only quieter. One pair, swapped
	// together. See Acts.UseOriginalDraft.
	wasSupplied []string
	// dropped is the marks a simplification took off, for the review to name beside Save.
	// NOT `wasSupplied`: that belongs to the FIRST simplification's way back, and a value
	// marked on an already-simplified draft and then simplified away is not in it.
	dropped []string
	// supplied is the values marked as supplied on the draft under review, in the order
	// they were marked.
	//
	// KEPT SO THE CARD CAN SAY WHAT SIMPLIFY WOULD COST. Simplifying re-derives the steps
	// from the EVENTS, which throws the marks away — there is no honest way to carry a
	// position across a list that folded — so the caption names them while they are still
	// there. Cleared by anything that re-derives. See supplyCaption.
	supplied []string
	// simplified is whether the draft under review is a simplification rather than the
	// recording as it was made. SHOWN, because a person approving a list of steps is owed
	// the fact that something has been folded since they made it.
	simplified bool
	// draft is what was recorded, unsaved. Nil while recording and after a failure.
	draft *orchestrator.Draft

	// saved is where it went, once Save has written it. Empty until then.
	saved string
	// partial is the sentence for a save that WROTE THE ROUTINE and could not write the
	// step account beside it, and "" for every other outcome.
	//
	// Its own field and not a refusal, because the two have opposite consequences. A
	// refusal means nothing is on disk and the card goes on offering Save and Replace; this
	// means the routine is there, runs, and cannot have its steps edited until it is
	// rebuilt. Returned as a refusal it would have sent the card to ask what was in the
	// way — and what is in the way is the routine this save just wrote, so the window
	// would have offered to replace it. See orchestrator.ErrStepsNotWritten.
	partial string
}

// Recording is what the page shows about a recorder-first recording, during and after.
type Recording struct {
	Name   string `json:"name,omitempty"`
	Active bool   `json:"active"`
	// Again is the identity of the saved routine this is a second recording of, and
	// AgainName what that routine calls itself. Both empty for an ordinary recording.
	//
	// The identity rather than the name, because a name is not unique: `settings` can be
	// saved for two applications under two scopes, and a window that remembered the word
	// would offer to replace whichever one it found first.
	Again     string `json:"again,omitempty"`
	AgainName string `json:"again_name,omitempty"`
	// AgainScope is where that routine works, so the review opens on its scope rather
	// than on the first option in a list. A PREFERENCE, not a decision: a scope the new
	// recording cannot be performed under is still refused by `keep`, and a forced scope
	// still withdraws the choice entirely. See Recording.Scope.
	AgainScope string `json:"again_scope,omitempty"`
	// AgainSays is what a re-recording means for what happens at Save, in a sentence.
	//
	// Said during the recording AND at review, because the thing it has to head off is
	// an expectation: somebody who pressed *Record again* on a routine may reasonably
	// think Save will replace it. It will not. Save refuses a collision and offers the
	// choice, exactly as it does for any other recording — see replace.go.
	AgainSays string `json:"again_says,omitempty"`
	// AgainElsewhere is set when the demonstration was made in a DIFFERENT application
	// from the routine being re-recorded.
	//
	// Because then an ordinary Save writes a SECOND routine rather than a new version of
	// the one somebody chose, and nothing refuses it: the identity a save writes is
	// built from the application that was observed, so a different application is a
	// different route and there is no collision to refuse. That is safe and it is not
	// what the person asked for, so it is said.
	AgainElsewhere string `json:"again_elsewhere,omitempty"`
	// Mode is what this recording was told to capture, in the word the saved routine
	// will state, and Captures is that in a sentence.
	//
	// Shown BESIDE RECORD and again at review, not only in the saved file. The reason is
	// the defect it closes: what a recording captured used to depend on an environment
	// variable the window inherited, so two recordings of the same thing could differ and
	// the window said nothing either time. A mode nobody can see is a mode nobody can
	// check. See orchestrator.RecordedAs.
	Mode     string `json:"mode,omitempty"`
	Captures string `json:"captures,omitempty"`
	// Everywhere is WHERE a recording captures and how it ends, and is a release
	// requirement rather than a nicety. The name is older than the sentence and is kept
	// so the field, the element and the test still answer to one word; what it says is
	// now that recording is bounded to the chosen window. See everywhereSentence.
	Everywhere string `json:"everywhere,omitempty"`
	// Review says a draft is waiting: the capture has ended and nothing is on disk.
	// This is the state the Stop button exists to reach.
	Review bool `json:"review,omitempty"`
	// Application is what was OBSERVED, and a proposal for the scope rather than the
	// scope itself. `Save` is told which scope to use; see SaveDraft.
	Application string `json:"application,omitempty"`
	// StoppedBy is how the capture ended — the button, or the key — because both stay
	// live and a person is owed the difference when the one they did not press won.
	StoppedBy string `json:"stopped_by,omitempty"`
	// Away says the person left the window being recorded, how often and for how long.
	//
	// It is on the review card because the draft is SHORTER than the demonstration they
	// gave and nothing else here explains why — and because the clock was changed: the
	// gap where the absence was is a quarter of a second now, which is right for the
	// routine and is not what happened. See orchestrator.Draft.AwaySentence.
	Away string `json:"away,omitempty"`
	// Exit says which keys somebody used to leave the window on their way back here, and
	// that they are not in the demonstration.
	//
	// IT IS THE ONE THING REVIEW SAYS WAS TAKEN OUT, which is why it is said at all: the
	// boundary refuses every other half-gesture rather than trimming it, and the only
	// reason this one is trimmed is that it is how the recording was ENDED. A person who
	// disagrees — who meant that Alt — can only disagree if the card tells them. See
	// orchestrator.Draft.ExitSentence.
	Exit string `json:"exit,omitempty"`
	// Window is the window this recording is of, named as the person chose it.
	Window string `json:"window,omitempty"`
	// Paused says the person is outside that window RIGHT NOW, and for how long.
	//
	// # Why this is not optional
	//
	// Because a confined recording goes silent the moment somebody leaves the window, and
	// silence looks exactly like a recording that is still going. Somebody who alt-tabs
	// to check something and carries on demonstrating gets a routine missing half its
	// steps, with nothing having told them — and the review sentence afterwards is no
	// substitute, because by then the demonstration they have to give again is over.
	Paused string `json:"paused,omitempty"`
	// Simplified says the steps below are a SIMPLIFICATION rather than the recording as
	// it was made, and Original whether there is something to go back to.
	//
	// SAID, BECAUSE SOMEBODY APPROVING A LIST OF STEPS IS OWED IT. A review that folded
	// typing since they demonstrated it, and showed the result as though it were what
	// they did, would be a card quietly disagreeing with their own memory of it.
	Simplified bool `json:"simplified,omitempty"`
	Original   bool `json:"original,omitempty"`
	// Pressable is how many steps send input, which is what makes a draft worth saving.
	Pressable int `json:"pressable,omitempty"`
	// Steps is the draft in plain words, one line each, in order.
	Steps []string `json:"steps,omitempty"`
	// Supply is the steps whose text could be made something the person supplies, and
	// SupplySays is what the control says about itself and about what Simplify would cost.
	//
	// TWO READINGS OF ONE LIST, index-aligned by construction: both are built from
	// `dr.Steps` in the same call, and `Supply` carries a position into `Steps` rather
	// than a sentence of its own, so the chooser and the list cannot describe two
	// different routines. Held by TestTheReviewOffersTheStepsItIsShowing.
	Supply     []SupplyChoice `json:"supply,omitempty"`
	SupplySays string         `json:"supply_says,omitempty"`
	// Marked names the values this draft will ask for instead of typing, and Unmarked the
	// ones a simplification took off. Both exist so the review can say what Simplify costs
	// at the moment somebody approves it -- beside the button, and again beside Save once
	// it has happened -- rather than only inside the chooser's own caption. The words are
	// the person's own value names, because "your marks" is not something anybody can check.
	Marked   []string `json:"marked,omitempty"`
	Unmarked []string `json:"unmarked,omitempty"`
	// TypedRuns is the runs of steps that could be declared TYPED TEXT, and TypedSays is
	// what that costs. See typeddraft.go for the defect it exists for.
	TypedRuns []TypedChoice `json:"typed_runs,omitempty"`
	TypedSays string        `json:"typed_says,omitempty"`
	// Basis is a term for the step list below, handed back with any operation against it.
	//
	// POSITIONS ALONE ARE NOT CONSENT. Simplify re-derives this list, going back to the
	// original replaces it, and each of the two edit controls changes it — so a press
	// that arrives after one of them refers to steps that have moved. The saved routine’s
	// panel has refused on a moved list since step editing existed; this card did not,
	// until the second edit control made it obvious that the window is the other writer.
	// See orchestrator.Draft.Shown.
	Basis string `json:"basis,omitempty"`
	// StopKey names the stop gesture, so the button is not the only way out and the other
	// way is written where somebody can see it — which is BEFORE Record as well as during
	// a recording, because the card that holds it while one is running is behind the
	// application being demonstrated in. There is always one: see stopLabel.
	StopKey string `json:"stop_key,omitempty"`
	// ArgKey names the key that marks a value the routine should ASK FOR rather than type,
	// and ArgSays is what to do with it. Empty when the key is off.
	//
	// BESIDE THE STOP KEY BECAUSE IT IS THE SAME KIND OF FACT and was missing for the same
	// reason: a reserved key that works, that nothing on screen mentions. A routine taking
	// an argument was reachable three ways and announced none of them, so the capability
	// was indistinguishable from an absent one — which is how it was reported. Said
	// BEFORE Record as well as during a recording: the band is behind the application being
	// demonstrated in, so the card is the only surface somebody can read the key on in time
	// to use it. See argSentence.
	ArgKey  string `json:"arg_key,omitempty"`
	ArgSays string `json:"arg_says,omitempty"`
	Said    string `json:"said,omitempty"`
	Failed  bool   `json:"failed,omitempty"`
	// Saved is the path the routine went to, once Save has written one.
	Saved string `json:"saved,omitempty"`
	// Partial says the routine IS saved and runs, and that the steps were not written
	// down beside it — so it cannot be step-edited until it has been rebuilt. The third
	// answer a save has, and rendered as a problem rather than as a success. See
	// capture.partial and orchestrator.ErrStepsNotWritten.
	Partial string `json:"partial,omitempty"`
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
	// DefaultScope is the scope the chooser OPENS on when the choice is still the person's:
	// `orchestrator.DefaultScope`, the one default every surface uses (From anywhere → this
	// application). The page used to type its own, "only in", which disagreed with the ruling
	// and could drift from the terminal's. Empty when there is no default to offer — no
	// application known — and then the page states no preference either.
	DefaultScope string `json:"default_scope,omitempty"`
	// Unsupported says this recording cannot be saved as a routine at all, and why.
	//
	// Different from Incomplete, which is a routine that saves and then refuses: this one
	// does not save. A step whose failure cannot be made to stop the routine — a press
	// found by matching a picture, a repeated block — would leave whatever follows it
	// running after a press that did not happen, which is the guard's promise broken by
	// the one step nobody could wrap. Said here because a generator error arriving at a
	// button press is a worse way to learn it.
	Unsupported string `json:"unsupported,omitempty"`
}

// nameOr is `s`, or `fallback` when there is nothing to call it.
func nameOr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// AWindow is one window the page offers, and `ID` is what comes back when somebody picks it.
type AWindow struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Windows lists the windows a recording could be of, for the control on the record card.
//
// # Why the page has to ask at all
//
// Because a recording is of ONE window and the window has to be chosen before capture starts.
// The first version inferred it — the first foreground window that was not Studio's became the
// target — and that is whatever came forward first, so an updater or a notification toast
// became the thing being recorded while the person demonstrated into a window nothing was
// watching. A choice somebody was never shown is not one they can correct.
//
// Asked fresh every time the card is drawn: a list of windows is the one thing that cannot be
// cached, because the person is about to open the application they mean to demonstrate in.
func (a *Acts) Windows() []AWindow {
	d := orchestrator.Deps{OwnerPID: uint32(os.Getpid())}
	var out []AWindow
	for _, p := range d.Pickable() {
		out = append(out, AWindow{ID: strconv.FormatUint(uint64(p.HWnd), 10),
			Label: p.Label()})
	}
	return out
}

// StartRecording begins a recorder-first capture under `name`, of the window `window`.
//
// Returns "" when the recorder is running, or the sentence to show the person. Nothing is
// brought forward and nothing counts down: the person switches to whatever they are
// demonstrating when they like, and that switch is recorded as the step that performs it.
func (a *Acts) StartRecording(name, window string) string {
	return a.beginRecording(name, window, nil)
}

// beginRecording is StartRecording, and `RecordAgain` through the same door.
//
// One function because of the five refusals below. A second entrance that repeated them would be
// five rules in two places, and a second entrance that skipped them would let somebody start a
// recording while a run was in flight — which is a runner pressing things into a draft as the
// person's own. `again` is nil for an ordinary recording and is what makes this one a
// re-recording of something saved; see again.go.
func (a *Acts) beginRecording(name, window string, again *reRecording) string {
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
	c := &capture{name: name, stop: make(chan struct{}), done: make(chan struct{}),
		again: again}
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
		// ANNOUNCED IN THIS WINDOW'S STORE, so the widget's shortcuts are held off while
		// somebody demonstrates here (internal/recordingnow).
		RecordingHome: a.home,
		// ── AND WHAT TO CAPTURE, SAID OUT LOUD AND NOT INHERITED ──
		//
		// This window records plain coordinates and the real pauses. That is not
		// a preference, it is what the path IS: it does not look at the screen,
		// which is the whole reason it works on a menu drawn over a live 3D world
		// where recognition has no answer.
		//
		// It used to be left to `$MARCO_CV` / `$MARCO_ANCHORS`, read once when
		// Studio started. So a window launched from a shell with anchors on
		// captured a picture of any click whose crop happened to be distinctive —
		// and a press found by matching a picture cannot be stopped at, so
		// `codegen.Unpropagatable` refused the whole recording at Save. The same
		// demonstration saved or was refused depending on a variable nobody in
		// the window could see and the pixels under the pointer. Twice in a row,
		// two different answers, no way to tell why.
		//
		// Stated here, where the recording is started, so it is this window's
		// claim and not the shell's. See orchestrator.Deps.Mode.
		Mode: studioCaptures,
	}
	// AND WHAT THE PAGE WILL BE TOLD IS A COPY OF WHAT THE RECORDER IS TOLD.
	//
	// Not the constant a second time. The mutation gate for this slice took the `Mode`
	// out of the backend above and left a window still promising coordinates over a
	// recording that had gone back to inheriting them — two statements of one fact, and
	// the page was the one that lied. So the fact is stated once, to the thing that acts
	// on it, and the sentence somebody reads is taken from there.
	a.mu.Lock()
	c.mode = d.Mode
	a.mu.Unlock()

	// ── AND WHICH WINDOW, CHOSEN BEFORE THE RECORDER STARTS ──
	//
	// Refused here, so the sentence reaches the button that asked rather than a window
	// that said "recording" and took nothing. `Begin` refuses a confined recording with
	// no window of its own accord; this is the same refusal one step earlier, with the
	// list the page offered in hand so it can say what went wrong.
	if window != "" {
		chosen, err := d.TheWindow(window)
		if err != nil {
			a.mu.Lock()
			a.capture = nil
			a.mu.Unlock()
			return err.Error()
		}
		d.Window = chosen.Target()
		a.mu.Lock()
		c.window = chosen.Label()
		a.mu.Unlock()
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
	a.mu.Lock()
	c.shot = shot
	a.mu.Unlock()
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

// SimplifyDraft shows a simplified version of the recording under review, and writes nothing.
//
// # Why this is on the review and not only on the saved routine
//
// A saved routine has offered it since [[ADR-273-a-routine-writes-down-what-it-does]]'s slice, and
// `marco learn`'s terminal review has offered it for longer. STUDIO'S REVIEW DID NOT, so the one
// surface where somebody looks at a demonstration they just made — with the steps in front of
// them and Save under their hand — was the one that could not simplify it. They had to save it
// first and then simplify the saved routine, which is two writes to get the version they wanted.
//
// The rules are not this window's: [orchestrator.Deps.SimplifiedDraft] is what the terminal review
// asks, re-deriving from the EVENTS with typing folded harder.
//
// NOTHING IS WRITTEN. The draft under review is swapped for the simplified one and the original is
// kept beside it, so [Acts.UseOriginalDraft] puts it back; Save persists whichever is being shown,
// which is the rule it already followed.
func (a *Acts) SimplifyDraft() string {
	dr, cp, why := a.draftToSave()
	if why != "" {
		return why
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	next := d.SimplifiedDraft(dr)
	switch {
	case next == nil:
		return "that recording cannot be simplified"
	case next.Unrepresentable != "":
		// THE REASON RATHER THAN A REFUSAL. Simplifying re-runs the same pipeline
		// over the same events, so what gets refused is a property of the
		// DEMONSTRATION — and a reason that can only appear by surprise is the one
		// worth not swallowing. The terminal review says the same thing.
		return "a simplified version cannot be saved as it stands, so it is not " +
			"offered: " + next.Unrepresentable
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if cp.wasDraft == nil {
		// THE FIRST SIMPLIFICATION KEEPS THE ORIGINAL, and a second one does not
		// overwrite it: simplifying twice must still have one way back, to what was
		// demonstrated rather than to the previous simplification.
		//
		// WITH ITS MARKS, because that draft may carry some: a briskly typed run
		// folds without Simplify, so a value can be marked before this is ever
		// pressed. See capture.wasSupplied.
		cp.wasDraft, cp.wasSupplied = cp.draft, cp.supplied
	}
	cp.draft = next
	cp.simplified = true
	if len(cp.supplied) > 0 {
		cp.dropped = cp.supplied
	}
	// AND THE MARKS ARE GONE WITH THE LIST THEY WERE ON. `SimplifiedDraft` re-derives
	// from the EVENTS, so a value somebody marked as supplied is typed text again — and
	// a position cannot honestly be carried across a list that folded. The caption said
	// so before this was pressed; clearing it here is what makes that true rather than
	// a warning about something that did not happen. See supplyCaption.
	cp.supplied = nil
	return ""
}

// UseOriginalDraft puts the recording as it was demonstrated back under review.
//
// The way back the ruling asks for, and it is the ORIGINAL rather than one step back: simplifying
// twice leaves one thing to return to, which is what was recorded.
func (a *Acts) UseOriginalDraft() string {
	_, cp, why := a.draftToSave()
	if why != "" {
		return why
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if cp.wasDraft == nil {
		return "this recording has not been simplified, so there is nothing to go back to"
	}
	// THE ORIGINAL COMES BACK WITH WHATEVER WAS MARKED ON IT, which is usually nothing:
	// the marks a simplification threw away were applied to a list derived from this one,
	// and this is the list before any of that. Usually, and not always — a briskly typed
	// run folds without Simplify, so somebody can have marked a value and then pressed it
	// — and a draft restored with its placeholders and no caption about them is a card
	// that says nothing about the steps on it. See capture.wasSupplied.
	cp.draft, cp.supplied = cp.wasDraft, cp.wasSupplied
	cp.wasDraft, cp.wasSupplied, cp.simplified, cp.dropped = nil, nil, false, nil
	return ""
}

// SaveDraft persists the reviewed recording, under the scope it is GIVEN.
//
// `scope` is "app", "app-anywhere" or "everywhere", and `app` is the application the scope is
// about — empty meaning the one that was observed. Stated rather than inferred, because this
// window can ask the question and a routine quietly rescoped to everywhere works in places
// nobody was told about.
// A collision is REFUSED here. See replace.go for why the policy sits in this window and not in
// the orchestrator, which overwrites on purpose for every other caller.
func (a *Acts) SaveDraft(scope, app string) string {
	dr, cp, why := a.draftToSave()
	if why != "" {
		return why
	}
	return a.keep(cp, dr, scope, app, orchestrator.Consent{})
}

// wantedScope reads the page's word for a scope, or says what the choices are.
//
// Refused rather than defaulted: a scope nobody recognised becoming "only in this application"
// is the window choosing where somebody's routine works and not telling them.
func wantedScope(scope string) (orchestrator.Scope, string) {
	return orchestrator.WantedScope(scope)
}

// Recording is the capture in flight, or the draft waiting to be looked at.
func (a *Acts) Recording() Recording {
	a.mu.Lock()
	c := a.capture
	a.mu.Unlock()
	if c == nil {
		// NOTHING IS BEING RECORDED, AND THE LINE BESIDE RECORD IS STILL AN ANSWER.
		//
		// What a recording STARTED NOW would capture, so the page can say it before
		// anybody presses anything — which is where it matters, because the choice
		// being described is one somebody is about to make. The same field a running
		// capture fills with its own mode, so the page needs no second question and
		// no sentence of its own.
		//
		// AND THE STOP KEY BEFORE THERE IS A RECORDING TO STOP. It was reported only
		// once one was running, which is the state where the card holding it is
		// behind whatever somebody is demonstrating in. See everywhereSentence.
		//
		// AND THE ARG KEY, WHICH IS ONLY USEFUL HERE. It is pressed during the
		// demonstration, which means it has to be known before Record — and
		// the name field it refers to is two rows above this sentence, so this
		// is the one card where both halves can be acted on at once.
		return Recording{Mode: string(studioCaptures),
			StopKey:    stopLabel(),
			Captures:   capturesSentence(studioCaptures),
			Everywhere: everywhereSentence(stopLabel()),
			ArgKey:     argKeyLabel(),
			ArgSays:    argSentence(argKeyLabel(), nil)}
	}
	out := Recording{Name: c.name, StopKey: stopLabel()}
	// WHAT IT IS CAPTURING, FROM THE CAPTURE AND NOT FROM THIS WINDOW'S INTENTIONS.
	//
	// `c.mode` is what the recorder was actually told when this one started, so a Studio
	// whose Deps stopped stating a mode would show nothing here rather than go on
	// promising coordinates. The sentence is the window's; the word is the recording's.
	out.Mode = string(c.mode)
	out.Captures = capturesSentence(c.mode)
	// AND WHERE, WHICH DOES NOT CHANGE WHILE ONE IS RUNNING.
	//
	// Answered during a recording as well as before one, though only the record card
	// draws it and that card is hidden while one is running — the band has its own
	// sentences, which say the window, the pause and the way back. This used to claim the
	// disclosure was on screen throughout "because the card is on screen the whole time
	// somebody is away doing the thing", which `showing($('captureCard'), idle && ...)`
	// has never done. A field the page may ask for in any state is worth answering in any
	// state; what is not worth doing is saying where it appears and being wrong.
	out.Everywhere = everywhereSentence(out.StopKey)
	// AND THE ARG KEY, NAMING THE VALUES THIS RECORDING DECLARED.
	//
	// From the name the person typed, split the same way `Deps.Begin` splits it
	// (`routes.SplitArgs`) rather than by looking for " with " here — one reading of
	// one phrase, or the band lists arguments the recorder is not filling. The order is the
	// whole point of saying it: the Nth press of the key fills the Nth name, and nothing
	// afterwards can tell that two of them went the wrong way round.
	out.ArgKey = argKeyLabel()
	_, declared := routes.SplitArgs(c.name)
	out.ArgSays = argSentence(out.ArgKey, declared)
	// AND WHICH ROUTINE THIS IS A SECOND RECORDING OF, when it is one.
	if g := c.again; g != nil {
		out.Again, out.AgainName, out.AgainScope = g.id, g.name, g.scope
		out.AgainSays = fmt.Sprintf("This is a second recording of %q. Saving it "+
			"will NOT replace that routine: Save refuses to write over one, "+
			"and then offers you Replace by name.", g.name)
	}
	select {
	case <-c.done:
	default:
		out.Active = true
		// ── WHAT THE CAPTURE IS DOING RIGHT NOW ──
		//
		// The window it is of, and whether it is admitting anything. Asked here
		// because the page redraws on this poll and cannot see the boundary
		// markers — `waitForStop` prints those, which suits the terminal and the
		// HUD that streams it. See orchestrator.Capture.Paused.
		a.mu.Lock()
		shot, window := c.shot, c.window
		a.mu.Unlock()
		out.Window = window
		if shot != nil {
			if paused, since := shot.Paused(); paused {
				out.Paused = fmt.Sprintf("Paused: you have left %s. "+
					"Nothing you do elsewhere is being recorded. "+
					"(%s)", nameOr(window, "the window"),
					since.Round(time.Second))
			}
		}
		return out
	}
	a.mu.Lock()
	dr, saved, partial := c.draft, c.saved, c.partial
	a.mu.Unlock()
	out.Saved, out.Partial = saved, partial
	if dr == nil {
		out.Failed = true
		out.Said = "nothing was recorded"
		return out
	}
	out.Review = saved == ""
	out.Application, out.StoppedBy = dr.App, dr.StoppedBy
	// AND WHETHER THE DEMONSTRATION WAS MADE WHERE THE ROUTINE LIVES.
	//
	// Only knowable once there is a draft, because the application is OBSERVED. See
	// againElsewhere for why a mismatch is worth a sentence: the save would succeed,
	// and it would write a second routine rather than the new version somebody meant.
	out.AgainElsewhere = againElsewhere(c.again, dr.App)
	out.Simplified, out.Original = c.simplified, c.wasDraft != nil
	out.Away = dr.AwaySentence()
	out.Exit = dr.ExitSentence()
	out.Pressable = dr.Pressable()
	out.Steps = inWords(dr.Steps)
	// AND WHICH OF THEM COULD ASK FOR THEIR TEXT INSTEAD OF TYPING IT.
	//
	// From the same step list as the sentences above, in the same call, so a position the
	// chooser offers is a position in the list on screen. See supplydraft.go for why the
	// control is one row under the list rather than a box on every step.
	out.Supply = supplyChoices(dr.Steps)
	out.SupplySays = supplyCaption(dr.Steps, c.supplied)
	out.Marked = append([]string(nil), c.supplied...)
	// WHAT THE LAST SIMPLIFICATION TOOK OFF AND HAS NOT BEEN MARKED AGAIN.
	if c.simplified {
		for _, name := range c.dropped {
			if !slices.Contains(c.supplied, name) {
				out.Unmarked = append(out.Unmarked, name)
			}
		}
	}
	// AND WHICH RUNS OF THEM COULD BE DECLARED TYPED TEXT, from the same list in the same
	// call, for the same reason: a range the chooser offers is a range of the steps on
	// screen.
	out.TypedRuns = typedChoices(dr.Steps)
	out.TypedSays = typedCaption(out.TypedRuns)
	// AND THE TERM FOR THE LIST ALL THREE OF THEM COUNT AGAINST. Taken here, beside the
	// steps it describes, so the page cannot be holding a term for one list and positions
	// into another.
	out.Basis = dr.Shown()
	// AND A GESTURE THAT CANNOT BE WRITTEN DOWN IS SAID HERE, BESIDE THE STEPS.
	//
	// Before the pressable check, because it is the more specific statement about the same
	// recording, and the steps are left in the payload on purpose: the person is entitled
	// to see what was understood of what they did. `Save` refuses this draft as well, so
	// this sentence is the explanation and not the enforcement — a window that only said
	// it, or a save that only refused it, would each be half an answer.
	if dr.Unrepresentable != "" {
		out.Failed = true
		out.Said = dr.Unrepresentable + "."
		return out
	}
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
		// WHAT TO DO INSTEAD IS NOT SPELT OUT ANY MORE, and that is the honest shape
		// of it. It used to end "record it again using ordinary left-clicks and the
		// keyboard", which named the wrong thing even then — a right-click was on the
		// list and left-clicks were the advice — and right and middle presses are
		// saved now. What remains unsaveable is a press found by matching a picture or
		// by reading words, and a repeated block, and none of those is a hand
		// somebody chose to use: telling them to click differently would send them to
		// change the one thing that is not the problem.
		out.Unsupported = fmt.Sprintf("This recording contains %s. A routine whose "+
			"presses are checked against the window they were made in has to "+
			"stop when one of them does not happen, and this version cannot "+
			"make %s do that — so it cannot be saved yet.", why, why)
	}
	// THE SHARED DEFAULT, for the chooser to open on. See Recording.DefaultScope.
	if def, _ := orchestrator.DefaultScope(dr); def != "" {
		out.DefaultScope = string(def)
	}
	// AND HOW IT WILL BE PERFORMED, when that is not a choice. See Recording.Scope.
	if only := orchestrator.ScopeFor(dr); only != "" {
		out.Scope = string(only)
		where := dr.App
		if where == "" {
			where = "the application it was recorded in"
		}
		// AND WHY IT IS NOT A CHOICE, which is the half a withdrawn control cannot
		// say for itself. This was "Runs in %s. Marco brings it forward before
		// playback." — true, and silent about the thing a person is looking for when
		// the chooser they expected is missing. The other two scopes bring no window
		// forward, so a press checked against the window it was made in would have
		// nothing to be checked against. See orchestrator.ScopeFor and
		// WhyNotThisScope, which refuses the same pairing at the write.
		out.Brings = fmt.Sprintf("Runs in %s, which Marco brings forward. Its presses "+
			"are checked against that window, so the other two choices would "+
			"leave every one of them refusing.", where)
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
	case macroir.StepMove:
		// A MOVE IS A PRESS-SHAPED STEP WITH NO BUTTON, and it had no sentence.
		//
		// It fell through to its own kind name, `move`, which the page's verb table
		// does not open -- so the editor drew it as a row of two numbers with an
		// empty action column, and the one word it did have was thrown away, because
		// a point row replaces the value half with the boxes. `PointIn` has always
		// offered it the same edit a click gets.
		where := fmt.Sprintf("%d, %d", s.X, s.Y)
		if s.WinRel {
			where = fmt.Sprintf("%d, %d in the window", s.RelX, s.RelY)
		}
		return fmt.Sprintf("move the pointer to %s%s", where, inWindow(s))
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

// capturesSentence is what a mode means, for somebody about to demonstrate something.
//
// The word `coordinates` is what the saved file states and is not what a person needs read to
// them. Unstated says so plainly rather than describing one of the things it might have been:
// that is the honest shape of a blank, and this window never produces one — a recording started
// here always states a mode, so an empty sentence here means the statement stopped happening.
func capturesSentence(m orchestrator.RecordedAs) string {
	switch m {
	case orchestrator.Coordinates:
		return "Where you press and how long you pause, and nothing about how the " +
			"screen looked. It does not take a picture of anything."
	}
	return "what this recording captures is not being stated, so it depends on how " +
		"Marco was started"
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
// A DRAG IS ASKED THE SAME QUESTION, because it declares the same contract now. It was a
// `StepClick`-only test while a drag measured nothing, and leaving it that way would have made
// a drag the one gesture whose missing measurements are invisible at review — refused at Save
// with a sentence, after the person had read a page that said nothing was wrong.
func notMeasured(s macroir.Step) []string {
	if s.RelTo == "" || (s.Kind != macroir.StepClick && s.Kind != macroir.StepDrag) {
		return nil
	}
	var out []string
	if !s.WinRel {
		// The same missing fact in both cases, said in the words of the gesture it
		// is about: a drag has two ends and "where the press went" would be a
		// sentence about something the person did not do.
		if s.Kind == macroir.StepDrag {
			out = append(out, "where in the window the drag began and ended")
		} else {
			out = append(out, "where in the window the press went")
		}
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
		"where in the window the drag began and ended",
		"how big the window was",
		"what scale it was drawn at",
		"which program owned it",
	} {
		if lacks[one] {
			missing = append(missing, one)
		}
	}
	// TWO THINGS, AND THE SCREEN SAYS THE REST OF IT.
	//
	// This was one 50-word sentence carrying four: how many presses, which measurements
	// were absent, what saving one of these would mean, and what to do about it. The page
	// says the third itself — `This routine won't run.` immediately above this, `Save
	// anyway` on the button below it — and then said REFUSE here as well, so the person
	// read one fact under a heading that had just made it. What is left is the count and
	// the named measurement, and then the repair.
	//
	// THE NAMED MEASUREMENT STAYS, and a first attempt at this cut it. Three tests exist to
	// stop exactly that: `TestReviewNamesWhicheverMeasurementFailed` walks the four things
	// that can be absent and requires each to be named, because they are not
	// interchangeable — "what scale it was drawn at" on a Windows that cannot report one
	// means recording it again will NOT help, and the other three mean it will. A person
	// deciding whether to keep this needs to know which they have.
	//
	// So what goes is what the screen carries, never what only this sentence can say.
	repair := "Record it again with the program in front of you."
	if lacks["what scale it was drawn at"] && !canReadWindowScale() {
		repair = "This computer's Windows cannot report a window's scale, so a " +
			"routine that checks the window it was made in cannot be made " +
			"on this machine."
	}
	// AND IT AGREES WITH ITSELF. A `%d press(es)` and a verb that does not bend with it
	// is how `1 press lack` reached the screen in this file's last copy pass; and the
	// things that can be absent are CLAUSES — "which program owned it", "what scale it
	// was drawn at" — so they are introduced rather than used as the object of a verb,
	// which is how "1 press lacks which program owned it" reads.
	press, are, measurement := "press", "is", "a measurement"
	if presses > 1 {
		press, are, measurement = "presses", "are", "measurements"
	}
	return fmt.Sprintf("%d %s %s missing %s: %s. %s",
		presses, press, are, measurement, andAll(missing), repair)
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
	// ID is the route's whole identity, and it is the term the page hands back when
	// somebody asks for one of these to be saved again.
	//
	// By NAME it was the first that answered, which for `settings` saved under two
	// applications is whichever this listing happened to put first — so the button beside
	// one row saved the other row's routine and said the first one's name while doing it.
	// The same defect the routine list had, in the same file. See Routine.ID.
	ID        string `json:"id"`
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
		out = append(out, Unrunnable{ID: routeID(u.Route), Name: u.Name,
			App: u.Route.App, Resavable: u.Resavable})
	}
	return out
}

// Resave writes one of them again from its own demonstration, under the scope its presses need.
//
// Explicit, and named for what it does: nothing repairs itself behind somebody's back, and a
// routine quietly rescoped is the thing `wantedScope` refuses to do at Save.
func (a *Acts) Resave(id string) string {
	a.mu.Lock()
	recording := a.recordingLocked()
	a.mu.Unlock()
	if recording {
		return "Marco is recording a demonstration; one thing at a time"
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	for _, u := range d.Unrunnable() {
		// EXACTLY THE ONE BESIDE THE BUTTON THEY PRESSED. By name it was the first
		// that answered, and two routines of one name in two applications are two
		// routines: the button beside the second row saved the first row's routine
		// and reported the first one's name. See Unrunnable.ID.
		if routeID(u.Route) != id {
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
	return fmt.Sprintf("there is no routine %q that needs saving again — press "+
		"Refresh and choose it again", id)
}

// everywhereSentence is what a recording takes in and how it ends, said before it starts.
//
// # It used to say the opposite of what the recorder does
//
// It said Marco watches the keyboard and mouse EVERYWHERE, that anything done in the meantime
// becomes part of the routine, and — in the clause that did the damage — that "recording only a
// chosen window is not something Marco can do yet". All three were true of the hooks when this was
// written. None of them survived [[ADR-268-record-only-the-chosen-window]]: `Deps.Watch`'s zero
// value is `OneWindow`, Studio sets no `Watch`, and `StartRecording` refuses a confined recording
// that was not told which window. The recording Studio makes is of ONE CHOSEN WINDOW.
//
// So the largest thing on the record card denied the chooser two rows under it, and the band three
// states later contradicted it three times over — the window named, the pause shown while it
// happens, the keys that came back trimmed. All of that exists only because confinement does. It
// cost a person either the recording they did not make, or a demonstration given with every other
// application closed for no reason.
//
// # And it says the way out while somebody can still read it
//
// The stop shortcut existed only in the recording band, which is BEHIND the application being
// demonstrated in for the whole of the time it is on screen, and there as the smallest text in it
// after a red Cancel. The one moment a person can learn the key is before they press Record, and
// that is the one card that never named it. So the key is in this sentence, and it comes from
// `recorder.ParseStopKey` rather than from `$MARCO_STOP_KEY` directly: an unset override is not no
// shortcut, it is F12 (`recorder.DefaultStopKey`), and a window that read the variable raw would
// go quiet about the key in the default installation — which is every installation.
//
// From the backend rather than written into the page, like the sentence above it, so the day the
// rule moves again the sentence moves with it rather than going quietly stale. Held by
// `TestTheWindowSaysWhatARecordingTakesInBeforeRecord`.
func everywhereSentence(stopKey string) string {
	return "Records actions in the window you choose. Recording pauses outside it. " +
		"Return to Studio and press Stop, or press " + stopKey + "."
}

// argKeyLabel is the configured "an argument goes here" key, as a person would read it, or ""
// when there is none.
//
// From `orchestrator.ArgKeyFor` and NOT from `$MARCO_ARG_KEY`, for the reason `stopLabel` below
// is written the way it is: an unset override is not "no key", it is F9
// (`simplify.DefaultArgKey`), and a window that read the variable raw would go silent about the
// key in the default installation, which is every installation. That exact bug shipped here once
// already, about the stop key, three rows further up the same card.
func argKeyLabel() string {
	return strings.ToUpper(orchestrator.ArgKeyFor(os.Getenv("MARCO_ARG_KEY")))
}

// argSentence is how to mark a value the routine should ASK FOR instead of typing, said on the
// card where a person can still act on it.
//
// # Why this is a release requirement and not a nicety
//
// Because the capability was COMPLETE AND INVISIBLE, which is the shape this repository keeps
// finding. A routine can take arguments three ways: name it `cloud pull stone with item` and the
// phrase declares one; press the reserved key during the demonstration and a placeholder is
// dropped where the value goes (`simplify.go`, the `argKey` branch); or turn a typed step into a
// supplied one afterwards. Studio wires all three — `Deps.Begin` splits the name through
// `routes.SplitArgs`, `Deps.ArgKey` is passed from the environment and falls back to F9, and the
// step edit has been there since [[ADR-273-a-routine-writes-down-what-it-does]] — and said
// nothing about any of them anywhere. `marco learn` prints the key in its prompt
// (`orchestrator.argKeyHint`); the window that has a Record button did not, so the owner of this
// repository looked for the feature and concluded it did not exist.
//
// # Two states, two sentences, one field
//
// Before a recording there is no name yet, so the card says what the key is FOR and how to name
// the value — the `with` clause, which is the other half nobody could find. While one is
// running the name is known, so the band names the values in the order the key will fill them,
// which is the only thing that stops a second press going to the wrong argument. The same field
// in both, like `Everywhere` above: a page with two sentences of its own is a page that chooses
// which state it is in.
func argSentence(key string, argNames []string) string {
	switch {
	case key == "":
		return ""
	case len(argNames) > 0:
		return "Press " + key + " where each value goes, in order: " +
			strings.Join(argNames, ", ") + "."
	default:
		return "Press " + key + " during the demonstration where the routine " +
			"should ask for a value instead of typing it. Add " +
			"“with item” to the name above and the value is called item."
	}
}

// stopLabel is the configured stop gesture, as a person would read it.
//
// ONE ANSWER FOR BOTH CARDS. `Recording` reported the raw `$MARCO_STOP_KEY` on one path and the
// parsed label on the other, so the same installation named F12 during a recording and nothing at
// all before one — the unset variable read as "there is no shortcut" in the one state where
// somebody still had time to learn it.
func stopLabel() string {
	return recorder.ParseStopKey(os.Getenv("MARCO_STOP_KEY")).Label()
}
