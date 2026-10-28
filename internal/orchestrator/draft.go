package orchestrator

// RECORD, REVIEW, THEN SAVE — three acts, not one.
//
// `Learn` and `LearnAuto` record and save in a single call. That is right for a terminal and for
// a HUD with one key: you press the stop gesture and the thing exists. It is wrong for a window
// with buttons, because it collapses decisions a person expects to make separately:
//
//	Stop    end the capture and SHOW ME what you got
//	Cancel  throw it away
//	Save    keep it, as this
//
// Routing a window's Stop button through `LearnAuto` would make Stop and Save the same event by
// construction — there would be nothing to cancel, because it would already be on disk. So this
// is a separate entrance: `Record` returns a draft and writes nothing, and `Save` is the only
// thing here that touches the store.
//
// # The owner's window is not part of the recording, anywhere
//
// `stripTrailingOwner` removes the tail that ends a recording. That is enough when the only
// contact with Marco is pressing Stop, and it is not enough for a window a person can glance at
// while demonstrating. A glance leaves an app switch in the middle of the stream, and
// `simplify.foldActivates` drops the step immediately before a switch as the gesture that
// performed it — so an unshielded click made just before looking at Marco is read as the taskbar
// press that brought Marco forward, and is lost. The same switch also decides which app the
// routine is filed under.
//
// Neither is acceptable for a window that invites being looked at, so a draft excludes the
// owner's own events wherever they occur. Interacting with the recorder is never the thing being
// demonstrated.
//
// Exclusion is by the process that owned the window at the instant of the event, never by the
// app name a poller sampled: a name can be stale, and the one case where it is stale is a click
// made in the GAME shortly after looking at Marco — exactly the click this exists to save. App
// switches are the one exception, because a switch carries no process at all.
//
// And taking a switch out leaves its RETURN behind, announcing an app that is already in front.
// `simplify.foldActivates` would collapse that redundant activate — but only after it has
// already deleted the step beside it as the gesture that performed the switch, so the glance
// eats the demonstrated click either way. `withoutRedundantReturns` does the collapse first.

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/simplify"
)

// Scope is where a saved routine is allowed to work. The same three the interactive prompt
// offers, named so a caller can state one instead of being asked.
type Scope string

const (
	// ScopeApp works only while its application is in front, and switches nothing.
	ScopeApp Scope = "app"
	// ScopeAppAnywhere works from anywhere and brings its application forward first.
	ScopeAppAnywhere Scope = "app-anywhere"
	// ScopeEverywhere works anywhere and switches nothing — for actions that are not
	// about one application.
	ScopeEverywhere Scope = "everywhere"
)

// mode converts a stated scope into the internal one, and says whether it needs an app.
func (s Scope) mode() scopeMode {
	switch s {
	case ScopeAppAnywhere:
		return scopeFocus
	case ScopeEverywhere:
		return scopeGlobal
	default:
		return scopeContext
	}
}

// Draft is a recording that has been taken and not yet saved.
//
// It holds both what was recorded and what was made of it, because review needs both: the steps
// are what would run, and the raw events are what a re-simplification would be built from. The
// draft itself is never executable — nothing runs until `Save` has written legal Marco.
type Draft struct {
	// Name is what the person called it, and Args the positional arguments its name declared.
	Name string
	Args []string
	// Steps is what was made of the recording. What would run, once saved.
	Steps []macroir.Step
	// Events is the raw demonstration, kept so a draft can be re-simplified and so the
	// saved routine can carry its own recording. Never executed.
	Events []recorder.RecordedEvent
	// App is the application the demonstration was made in, as observed. A proposal for
	// the scope and nothing more: `Save` takes the scope it is given.
	App string
	// StoppedBy says how the capture ended, which review may want to show.
	StoppedBy string
	// Unrepresentable is why this recording cannot be saved, or "" when it can.
	//
	// A REFUSAL CARRIED ON THE DRAFT RATHER THAN RETURNED INSTEAD OF IT. The person is
	// about to review what was recorded, and "nothing happened" would be a worse answer
	// than the steps plus the reason they cannot be kept: the gesture that could not be
	// written down is one they performed, and they are the only one who can perform it
	// again differently. So review shows both and `Save` refuses.
	//
	// Set from `simplify.Simplify`, which is the only thing that knows. See
	// `simplify.modifierTrouble` for what qualifies and why it is a refusal and not a
	// best guess.
	Unrepresentable string
	// Mode is what the recording was TOLD to capture, or Unstated when nothing was.
	//
	// It travels on the draft because review happens before the save: a person deciding
	// whether to keep a recording is entitled to know what was captured, and a window
	// that asked the environment again at that moment would be answering about the
	// process rather than about the recording in front of them. `Save` writes this and
	// nothing re-derives it. See Deps.Mode.
	Mode RecordedAs
}

// Pressable counts the steps that send input, which is what makes a draft worth saving.
func (d *Draft) Pressable() int {
	n := 0
	for _, s := range d.Steps {
		switch s.Kind {
		case macroir.StepClick, macroir.StepDrag, macroir.StepKey,
			macroir.StepKeyDown, macroir.StepKeyUp, macroir.StepType:
			n++
		}
	}
	return n
}

// Capture is a recording that has begun: the recorder is watching, and nothing has been made of
// what it has seen yet.
//
// It exists so that STARTING can fail separately from finishing. A window with a Record button
// has to be told at once that the recorder would not start, because the alternative is a window
// that says "recording", takes somebody's whole demonstration, and only says it never began when
// they press Stop. `Record` below is still the one-call form, for the callers that block.
type Capture struct {
	d        Deps
	name     string
	argNames []string
	stop     recorder.StopKey
	// mode is what this capture told its recorder to capture. See Deps.Mode.
	mode RecordedAs
}

// Begin starts the recorder and returns the capture in flight.
//
// The only failure is the recorder refusing to start, which is a platform fact and is the one
// thing a caller can act on before a person has demonstrated anything.
func (d Deps) Begin(name string) (*Capture, error) {
	name, argNames := routes.SplitArgs(name)
	stop := recorder.ParseStopKey(d.StopKey)
	mlog.Debug("draft: starting recorder", "route", name, "stop_key", stop.Label())
	// ── WHAT TO CAPTURE, STATED BEFORE THE HOOKS GO ON ──
	//
	// Before `Start`, because the hooks read it and a mode stated afterwards would be
	// stated after the first press could already have been captured. A recorder that
	// cannot be told keeps doing what it did, which is the whole of "other callers are
	// preserved": every caller that states nothing sends `Inherited` and the environment
	// decides, exactly as it always has.
	mode := d.recordedAs()
	if s, ok := d.Rec.(recorder.Stated); ok {
		s.Capture(mode.capture())
	}
	if err := d.Rec.Start(); err != nil {
		return nil, fmt.Errorf("can't record on this platform: %w", err)
	}
	return &Capture{d: d, name: name, argNames: argNames, stop: stop, mode: mode}, nil
}

// Name is what this capture will be called, after the argument names have been taken off it.
func (c *Capture) Name() string { return c.name }

// Record captures a demonstration and returns it UNSAVED.
//
// It writes nothing. A caller that wants the draft kept calls `Save`; a caller that does not
// drops it, and that is the whole of Cancel.
//
// The capture ends on the stop gesture or on `Deps.StopWhen`, whichever comes first — see
// `waitForStop` for why both stay live.
func (d Deps) Record(name string) (*Draft, error) {
	c, err := d.Begin(name)
	if err != nil {
		return nil, err
	}
	return c.Finish(), nil
}

// Finish waits for the capture to end and makes a draft of what was recorded.
//
// It cannot fail: everything after the recorder started is arithmetic over events that have
// already happened. A recording of nothing is a draft with no steps, which `Save` refuses — and
// that refusal belongs there, where somebody is deciding whether to keep it.
func (c *Capture) Finish() *Draft {
	d, name, argNames, stop := c.d, c.name, c.argNames, c.stop
	byButton := waitForStop(d.Rec, &stop, d.StopWhen)
	events := stripTrailingStop(d.Rec.Stop(), stop)
	// ── THE OWNER'S WINDOW, UNDER WHICHEVER OF THE TWO RULES THIS DEPS ASKED FOR ──
	//
	// Everywhere for a surface that is glanced at, which is what the window needs and why
	// the tail alone is not enough there; off the tail of a button-ended recording for
	// everything else, which is what the terminal and the widget have always done. Both
	// were decided on purpose and neither is this function's to pick. See
	// Deps.OwnerEverywhere.
	if d.Owner != "" || d.OwnerPID != 0 {
		switch {
		case !d.OwnerTailOnly:
			events = withoutRedundantReturns(
				withoutOwner(events, d.Owner, d.OwnerPID))
		case byButton:
			events = stripTrailingOwner(events, d.Owner, d.OwnerPID)
		}
	}

	// THE MODE THIS CAPTURE RAN UNDER, not this Deps' now. The same value today — the
	// capture took it from the Deps — and the one that stays right if a draft ever outlives
	// the window state that made it, which is what `Draft.Mode` is for one field down.
	opts := d.learnOptionsFor(c.mode)
	opts.ArgNames = argNames
	steps, refused := simplify.Simplify(events, opts)
	d.enrichAnchors(steps)

	app := firstActivateApp(steps)
	if app == "" {
		app = d.activeApp()
	}
	how := "the stop key"
	if byButton {
		how = "the Stop button"
	}
	mlog.Info("draft: recorded", "route", name, "events", len(events),
		"steps", len(steps), "app", app, "stopped_by", how)
	return &Draft{
		Name: name, Args: argNames, Steps: steps, Events: events,
		App: app, StoppedBy: how,
		Unrepresentable: reasonOf(refused),
		// THE MODE THIS CAPTURE WAS STARTED UNDER, carried from the capture and
		// not asked of the environment again. `Begin` is where it was stated and
		// where the recorder was told; a second reading here could disagree with
		// what the hooks were actually running under.
		Mode: c.mode,
	}
}

// Save writes a draft as legal Marco, under the scope it is given.
//
// The scope is STATED rather than inferred, which is the other half of separating Save from
// Stop: a window asks where a routine should work and then says so, where the terminal prompts
// for it and the HUD guessed. `app` is the application the scope is about — the draft's own
// observed app unless the caller names another — and is ignored by `ScopeEverywhere`.
// Target is the route a draft WOULD be saved as, without saving anything.
//
// Exported because a caller that has to ask "is there already a routine there?" must ask about
// the identity a save would actually write. Deriving it a second time in the caller would be a
// second answer to which routine this is, and the answers would differ the day either side
// changed — the shape of defect this project keeps finding. `Save` calls this, so there is one
// derivation and a collision check and a save cannot disagree.
//
// It says nothing about whether that route exists; that is the caller's question to ask.
func (d Deps) Target(dr *Draft, scope Scope, app string) (routes.Route, error) {
	if dr == nil {
		return routes.Route{}, fmt.Errorf("there is no draft to save")
	}
	if app == "" {
		app = dr.App
	}
	mode := scope.mode()
	if app == "" && mode != scopeGlobal {
		return routes.Route{}, fmt.Errorf("%q needs an application and none was "+
			"observed or given", scope)
	}
	rt := routes.Route{Slug: routes.Slug(dr.Name)}
	switch mode {
	case scopeContext:
		rt.App = app
	case scopeFocus:
		rt.App, rt.Focus = app, true
	}
	return rt, nil
}

func (d Deps) Save(dr *Draft, scope Scope, app string) (routes.Route, error) {
	return d.saveWith(dr, scope, app, true)
}

// saveWith is Save, told whether it may write over what is there.
//
// ONE PREAMBLE, because `Keep` needs every line of it and a second copy would be a second answer
// to which application a draft is about. That is not hypothetical: `Keep` was written with its
// own Target call and WITHOUT the `app == "" -> dr.App` fallback three lines below, so a focus
// routine recorded from inside its own application came out with no Activate — a routine that
// types into whatever happened to be in front. `TestAFocusPlayActivatesEvenWhenTheDemonstrationDidNot`
// said so, which is what it is for.
func (d Deps) saveWith(dr *Draft, scope Scope, app string, over bool) (routes.Route, error) {
	if dr == nil {
		return routes.Route{}, fmt.Errorf("there is no draft to save")
	}
	if len(dr.Steps) == 0 {
		return routes.Route{}, fmt.Errorf("nothing was recorded, so there is " +
			"nothing to save")
	}
	// THE ONE WAY A DRAFT BECOMES A ROUTINE, so the one place a refusal has to be
	// answered. A gesture that cannot be written down faithfully is not saved as
	// something else: see Draft.Unrepresentable.
	if dr.Unrepresentable != "" {
		return routes.Route{}, fmt.Errorf("this recording cannot be saved as it "+
			"stands: %s", dr.Unrepresentable)
	}
	if app == "" {
		app = dr.App
	}
	mode := scope.mode()
	if app == "" {
		// NO APPLICATION TO BE ABOUT. Said rather than silently rescoped: a
		// caller that asked for an app scope and gets a global routine would
		// have been told its routine works somewhere it does not.
		if mode != scopeGlobal {
			return routes.Route{}, fmt.Errorf("%q needs an application and "+
				"none was observed or given", scope)
		}
	}
	rt, err := d.Target(dr, scope, app)
	if err != nil {
		return routes.Route{}, err
	}
	return rt, d.write(dr, mode, app, rt, over)
}

// write renders a draft and puts it on disk, either over what is there or only where nothing is.
//
// # Two ways to write one file, and the difference is consent
//
// `over` true is what `Save` has always done and what a REPLACEMENT needs: the path is expected
// to be occupied and the new version takes its place. Through `Registry.Replace`, which is atomic
// — a write that fails part way would otherwise leave somebody's working routine truncated and
// the thing it replaced gone, which is the one case where there is an original to lose.
//
// `over` false is a FRESH save, and it refuses a path that is already occupied. That is not the
// same question as the collision check `Keep` runs: the check is two instants earlier and
// produces the sentence a person reads, and this is what makes its answer true at the moment of
// writing. Another save landing in the gap between them used to be overwritten by a caller that
// had been told the path was free. See Registry.Create.
func (d Deps) write(dr *Draft, mode scopeMode, app string, rt routes.Route, over bool) error {
	src, assets, err := d.render(dr, mode, app, rt)
	if err != nil {
		return err
	}
	put := d.Reg.Create
	if over {
		put = d.Reg.Replace
	}
	if err := put(rt, src); err != nil {
		mlog.Error("draft: save failed", "route", dr.Name, "over", over, "err", err)
		return err
	}
	if err := d.Reg.WriteAssets(assets); err != nil {
		return err
	}
	// AND THE RAW DEMONSTRATION BESIDE IT, for review and re-simplification. Not an
	// execution path: nothing reads it to run anything. Best-effort, as the other
	// save paths treat it.
	if raw, mErr := json.Marshal(dr.Events); mErr == nil {
		_ = d.Reg.SaveRecording(rt, raw)
	}
	mlog.Info("draft: saved", "route", dr.Name, "app", app, "steps", len(dr.Steps),
		"path", d.Reg.Path(rt))
	return nil
}

// withoutOwner removes the recorder's own window from a demonstration, wherever it appears.
//
// By PROCESS, read at the instant of each event, and never by the app name a poller sampled —
// a name can be stale, and the case where it is stale is a click made in the target application
// shortly after looking at Marco, which is precisely what must survive.
//
// App switches are the exception and are matched by name, because a switch carries no process at
// all. Removing them is what lets an unshielded click survive a mid-recording glance: with the
// switch gone, the click before it is no longer the step immediately before a switch, and
// `simplify.foldActivates` has no reason to read it as the gesture that performed one. It is
// also what keeps the routine filed under the application it was demonstrated in.
//
// # A CLICK IS ONE ACT WITH TWO EDGES, AND THEY CAN BE ATTRIBUTED TO DIFFERENT PROCESSES
//
// This is the whole of why the clicks are paired BEFORE anything is excluded, and it is a
// defect this had, found in the owner's first mouse-driven trial.
//
// The press that RAISES a window arrives while the previous window is still in front — clicking
// a window is what causes the activation, so the foreground at the instant of the press is the
// old one. `internal/recorder` says so itself and refuses to compute a window offset in that
// case. So a click on Marco's window from the application has its press attributed to the
// APPLICATION and its release to MARCO, and the click back has its press attributed to MARCO and
// its release to the APPLICATION.
//
// Excluding edge by edge therefore kept half of each click. The surviving halves were a press on
// Marco and a release in the application with no press between them — which `internal/simplify`
// reads, correctly by every rule it has, as a DRAG from one to the other. A drag cannot be saved
// in a routine whose presses are checked, so looking at Marco during a recording and clicking
// back made the whole recording unsaveable. Reported from the live trial as
// `drag from -740, 348 to 50, 374`: the press of one click joined to the release of the next.
//
// So the pair decides together, and a click EITHER of whose edges is the owner's is the owner's.
// That is right about intent as well as about mechanism: a press in the application whose release
// is in Marco IS the click that raised Marco, and a press in Marco whose release is in the
// application IS the click that went back. Both are somebody working the window rather than
// showing Marco anything.
//
// An edge with no partner is decided on its own, and kept unless it is the owner's: an event this
// cannot place is not shown to belong to anybody, and guessing the other way deletes
// demonstrations. It is never completed by borrowing an edge from a different click.
func withoutOwner(events []recorder.RecordedEvent, owner string,
	ownerPID uint32) []recorder.RecordedEvent {

	if owner == "" && ownerPID == 0 {
		return events
	}
	// PAIRED FIRST, so no decision below can take one edge of a click and leave the
	// other.
	partner := pairClicks(events)
	out := make([]recorder.RecordedEvent, 0, len(events))
	for i := range events {
		e := events[i]
		if owner != "" && e.Kind == recorder.EvAppSwitch && e.KeyName == owner {
			continue
		}
		if e.Kind == recorder.EvClick {
			if ownersClick(events, i, partner[i], ownerPID) {
				continue
			}
			out = append(out, e)
			continue
		}
		if ownerPID != 0 && e.PID == ownerPID {
			continue
		}
		out = append(out, e)
	}
	return out
}

// pairClicks matches every press to its OWN release, per button, in order.
//
// Returns one entry per event: the index of the other edge of that click, or -1 for an event
// that is not a click or whose partner never arrived.
//
// Per button, because buttons interleave — a left press, a right press, a left release and a
// right release are two clicks and not one of each other's halves. In order, and never across a
// second press of the same button: a button cannot be released twice without being pressed again,
// so the release after a press is that press's own, and a press followed by another press of the
// same button is a press whose release was lost. Taking the later release for it would be
// inventing a gesture out of two different clicks, which is exactly the defect being fixed.
func pairClicks(events []recorder.RecordedEvent) []int {
	partner := make([]int, len(events))
	for i := range partner {
		partner[i] = -1
	}
	// open is the press of each button still waiting for its release, by index.
	open := map[string]int{}
	for i := range events {
		e := events[i]
		if e.Kind != recorder.EvClick {
			continue
		}
		button := strings.ToLower(strings.TrimSpace(e.Button))
		if e.Down {
			// AND A SECOND PRESS LEAVES THE FIRST UNMATCHED, rather than
			// reaching past it. The nearest press is the one the next release
			// belongs to.
			open[button] = i
			continue
		}
		if at, waiting := open[button]; waiting {
			partner[at], partner[i] = i, at
			delete(open, button)
		}
		// AN UNMATCHED RELEASE STAYS UNMATCHED. Its press was never recorded, and
		// the press before it belongs to a click of its own.
	}
	return partner
}

// ownersClick reports whether this click belongs to the window driving the recording.
//
// Asked of the PAIR: either edge being the owner's makes the click theirs, because the two edges
// of one click can be attributed to two processes and the click is still one act. See
// withoutOwner.
func ownersClick(events []recorder.RecordedEvent, i, with int, ownerPID uint32) bool {
	if ownerPID == 0 {
		return false
	}
	if events[i].PID == ownerPID {
		return true
	}
	return with >= 0 && events[with].PID == ownerPID
}

// withoutRedundantReturns drops an app switch naming the application already in front.
//
// Such a switch is created by the exclusion above rather than by the demonstration: the
// recorder's poller emits a switch only when its sample CHANGES, so two switches naming the same
// app cannot both reach here until something between them has been taken out. A glance at Marco
// and the return from it are one `studio` sample and one `arkascended` sample — and with the
// `studio` gone, the `arkascended` announces a switch to a window that never left the front.
//
// It has to happen BEFORE `simplify`, because `simplify.foldActivates` does these two things in
// the order that loses the demonstration. It first drops the step immediately before an activate
// as the gesture that performed the switch, and only then collapses an activate that re-focuses
// the app in effect — so the redundant return takes the demonstrated click beside it on the way
// out, and is then removed itself, leaving no trace of what it cost. Collapsing it first leaves
// the gesture rule nothing to fire on.
//
// This is the second shape of the same failure and it is the ORDINARY one. The stale-name case
// needs the person to click within one poll of returning to the game; this one is what a glance
// looks like whenever the poller samples both sides of it, which over a visit long enough to
// read something is every time. It was missed because the fixture omitted the return.
//
// `simplify` itself is untouched and this runs only where a window is being excluded, so the
// draft path is the only path whose stream changes. `LearnAuto` keeps the behaviour it shipped
// with — it strips only the TAIL, so a mid-visit glance leaves its switch in the stream and the
// click before it still goes, exactly as it did before any of this. That is a real limitation of
// the legacy path and it is left alone on purpose: Studio's recording goes through the draft.
func withoutRedundantReturns(events []recorder.RecordedEvent) []recorder.RecordedEvent {
	out := make([]recorder.RecordedEvent, 0, len(events))
	// inFront is the app named by the last switch that survived. Empty until one does:
	// a recording that opens with no switch at all has nothing to be redundant with.
	inFront := ""
	for _, e := range events {
		if e.Kind == recorder.EvAppSwitch {
			if e.KeyName == inFront {
				continue
			}
			inFront = e.KeyName
		}
		out = append(out, e)
	}
	return out
}

// ScopeFor is the only scope a draft can honestly be saved under, or "" when the choice is still
// the person's.
//
// A press that asked to be checked against the window it was made in needs that window brought
// forward, and `ScopeApp` and `ScopeEverywhere` both promise to switch nothing: their routes
// carry no `Activate`, so nothing binds a window and every such press refuses. The choice is
// therefore not a choice, and saying so is better than offering three options two of which
// produce a routine that cannot press anything.
//
// A draft with no guarded press keeps all three, because it is saved under the contract it always
// was.
func ScopeFor(dr *Draft) Scope {
	if dr == nil || !guarded(dr.Steps) {
		return ""
	}
	return ScopeAppAnywhere
}

// guarded reports whether any step asked to be checked against the window it was recorded in.
func guarded(steps []macroir.Step) bool {
	for _, s := range steps {
		if s.RelTo != "" {
			return true
		}
		if len(s.Steps) > 0 && guarded(s.Steps) {
			return true
		}
	}
	return false
}

// Unrunnable is a SAVED routine whose presses ask to be checked against the window they were made
// in, under a scope that brings no window forward — so every one of them refuses.
//
// It exists because the guarded contract arrived before this rule did: a routine saved in between
// is on disk now, reads as a routine, and cannot press anything. Nothing detects that at run
// time except the refusal itself, one press at a time, which is a person watching a routine do
// nothing and being told why four times.
type Unrunnable struct {
	Route routes.Route
	// Name is what it was called, as the route itself records it.
	Name string
	// Resavable says its own demonstration is still stored beside it, so it can be saved
	// again under the scope its presses need without anybody re-demonstrating anything.
	Resavable bool
}

// Unrunnable lists the saved routines in that state.
func (d Deps) Unrunnable() []Unrunnable {
	var out []Unrunnable
	for _, rt := range d.Reg.List() {
		if rt.Focus {
			continue // brings its application forward, so its presses have a window
		}
		raw, err := os.ReadFile(d.Reg.Path(rt))
		if err != nil || !strings.Contains(string(raw), `RelTo "`) {
			continue
		}
		out = append(out, Unrunnable{Route: rt, Name: NameIn(string(raw), rt.Slug),
			Resavable: d.Reg.HasRecording(rt)})
	}
	return out
}

// Resave writes one of them again under the scope its presses need, from its own demonstration.
//
// The demonstration is re-simplified rather than the source rewritten, because the source is
// generated and rewriting generated Marco by hand is how two things that have to agree stop
// agreeing. The old file is removed only once the new one is written.
//
// WHAT THE ROUTINE STATES ABOUT ITS OWN RECORDING SURVIVES, and that is not only a comment: the
// stated mode decides the simplification as well as the line. See the note beside the read below.
func (d Deps) Resave(u Unrunnable) (routes.Route, error) {
	if !u.Resavable {
		return routes.Route{}, fmt.Errorf("%q no longer has the demonstration it "+
			"was made from, so it cannot be saved again — record it again",
			u.Name)
	}
	raw, ok := d.Reg.LoadRecording(u.Route)
	if !ok {
		return routes.Route{}, fmt.Errorf("%q no longer has the demonstration it "+
			"was made from, so it cannot be saved again — record it again",
			u.Name)
	}
	var events []recorder.RecordedEvent
	if err := json.Unmarshal(raw, &events); err != nil {
		return routes.Route{}, fmt.Errorf("%q: its demonstration could not be "+
			"read back: %w", u.Name, err)
	}
	// ── WHAT THE ROUTINE SAYS IT WAS RECORDED AS, READ OFF THE FILE BEING REPLACED ──
	//
	// `Unstated` here was a defect, and a quiet one in both directions. The saved line went
	// missing, so a routine that stated coordinates came back out of a repair stating
	// nothing — a repair that edits a claim about somebody's afternoon out of their file.
	// And because the mode is also what keeps the recorded gaps, a Studio launched with
	// `$MARCO_CV=on` rebuilt that same routine with every pause capped to 50ms and no
	// anchor to wait on in their place. The routine then presses a menu before it has
	// drawn, having been repaired.
	//
	// Read from the source rather than taken from this Deps or from the caller's
	// `Unrunnable`, for the reason `ModeOf` exists: the saved Marco is the only durable
	// account of how a recording was made, and anything else is a second account to
	// disagree with it.
	src, err := os.ReadFile(d.Reg.Path(u.Route))
	if err != nil {
		return routes.Route{}, fmt.Errorf("%q could not be read back, so what it "+
			"states about its own recording cannot be carried into the repair: %w",
			u.Name, err)
	}
	// AND AN UNKNOWN WORD STAYS UNSTATED rather than being echoed, which `ModeOf` is what
	// does: a repair is not a place to start passing on a claim nothing verified.
	dr := d.DraftOf(u.Name, events, u.Route.App, "a repair", ModeOf(string(src)))
	name := dr.Name
	app := u.Route.App
	if app == "" {
		app = firstActivateApp(dr.Steps)
	}
	rt, err := d.Save(dr, ScopeAppAnywhere, app)
	if err != nil {
		return rt, err
	}
	if rt != u.Route {
		// THE OLD ONE GOES, AND ONLY NOW. Two routines of the same name, one of
		// which cannot press anything, is worse than the one that could not.
		if err := d.Reg.Delete(u.Route); err != nil {
			mlog.Error("resave: the old routine is still there", "route",
				u.Route.Slug, "err", err)
		}
	}
	mlog.Info("resave: saved again so its presses have a window", "route", name,
		"was", string(ScopeApp), "now", string(ScopeAppAnywhere),
		"recorded_as", string(dr.Mode))
	return rt, nil
}

// NameIn recovers what a generated route was called, from the line codegen writes at the top of
// it, falling back to the slug with its dashes read as spaces.
//
// The header is the only record of the name: `routes.Route` carries a slug, and a slug has lost
// the spaces. The fallback is honest rather than exact — "ark-graphics" instead of "ark
// graphics" — and it only decides what the regenerated file calls itself.
func NameIn(src, slug string) string {
	const opens = "marco learn \""
	i := strings.Index(src, opens)
	if i < 0 {
		return strings.ReplaceAll(slug, "-", " ")
	}
	rest := src[i+len(opens):]
	if j := strings.Index(rest, "\""); j >= 0 && j > 0 {
		return rest[:j]
	}
	return strings.ReplaceAll(slug, "-", " ")
}

// ── WHAT A SAVED ROUTINE SAYS ABOUT HOW IT WAS RECORDED ───────────────────────

// RecordedAs is what a recording was told to capture, in the word a saved routine states.
//
// # Why it is stated and never inferred
//
// Because it cannot be inferred. A routine with no pictures in it may have been recorded in
// coordinate mode on purpose, or recorded with the anchor stack on against a flat menu that
// offered nothing distinctive to anchor to. Those are two different facts about somebody's
// afternoon and the file looks identical. So the mode is a claim the thing that started the
// recording makes, or it is absent — and absent is read as absent.
//
// # And the word is not the engineering switch
//
// `$MARCO_CV` and `$MARCO_ANCHORS` are settings of a process, read once when it starts. This is a
// statement about ONE recording, and it beats the environment: see recorder.Mode for the defect
// that made it necessary, which is one demonstration saving or being refused depending on a
// variable nobody in the window could see.
type RecordedAs string

const (
	// Unstated is a recording whose caller said nothing about what to capture, which is
	// every caller but Studio's coordinate mode and every routine already on disk.
	//
	// The zero value on purpose: a `Deps` built without thinking about this behaves as
	// every `Deps` did before it existed.
	Unstated RecordedAs = ""
	// Coordinates is the coordinate and the clock and nothing else: no picture of the
	// control pressed, no colour under it, and the anchor key left as an ordinary key.
	//
	// It is what makes a recording's saveability a property of the recording. A press
	// found by matching a picture cannot be stopped at, so `codegen.Unpropagatable`
	// refuses a whole guarded routine that contains one — and with anchors inherited ON,
	// whether a recording contains one depends on the pixels that happened to be under
	// the pointer.
	Coordinates RecordedAs = "coordinates"
)

// recordedAs is the mode this Deps states, with the default named rather than left blank.
func (d Deps) recordedAs() RecordedAs {
	if d.Mode == Coordinates {
		return Coordinates
	}
	return Unstated
}

// capture is this mode as the recorder is told it.
//
// Two vocabularies on purpose, and the conversion is HERE, in one function. `recorder.Mode` is
// about hooks and pictures; `RecordedAs` is the word a saved routine carries and a run record
// prints. A recorder that knew the saved word would be a recorder answering a question about a
// file format.
func (m RecordedAs) capture() recorder.Mode {
	if m == Coordinates {
		return recorder.Coordinates
	}
	return recorder.Inherited
}

// modeStated is the comment line codegen writes and this reads back.
//
// One constant shared by the writer and the reader would be better, and it cannot be: `codegen`
// may not import this package. Held by test instead — TestTheModeASaveWritesIsTheModeThisReads
// generates a route in each mode and requires this to read back what was stated.
const modeStated = "// Recorded as: "

// ModeOf reads how a saved route states it was recorded, or Unstated when it does not say.
//
// From the SOURCE, for the same reason `ContractOf` is: the saved Marco is the only durable
// account of a route, and a second record of it beside the file is a second record to disagree
// with it. Unstated covers every route saved before this, every route whose generator was told
// nothing, and every route somebody edited the line out of — all three of which are "this file
// does not say", which is the honest answer and not a default.
func ModeOf(src string) RecordedAs {
	for _, line := range strings.Split(src, "\n") {
		rest, found := strings.CutPrefix(strings.TrimSpace(line), modeStated)
		if !found {
			continue
		}
		switch RecordedAs(strings.TrimSuffix(strings.TrimSpace(rest), ".")) {
		case Coordinates:
			return Coordinates
		}
		// A WORD THIS DOES NOT KNOW IS NOT A MODE. A newer build's word, or a
		// hand-typed one, reads as "unstated" rather than being passed on: a reader
		// that echoed whatever it found would put an unrecognised string into a run
		// record as though this had verified it.
		return Unstated
	}
	return Unstated
}

// ── WHAT A SAVED ROUTINE SAYS ABOUT ITS OWN PRESSES ───────────────────────────

// Contract is which of the three things a saved route says about how its presses are checked.
//
// The BASIS is the contract and everything else is a measurement — see
// [[ADR-251-a-press-checks-the-window-it-was-recorded-in]] — so these are three states a route
// is IN, read off the saved Marco, rather than three ways a recording happened to go.
type Contract string

const (
	// Legacy is a route whose presses state no window: they are pressed at the
	// coordinate they were recorded at, having checked nothing, exactly as every route
	// saved before any of this did.
	Legacy Contract = "legacy"
	// Guarded is a route whose every press states the window it was made in and
	// everything needed to check it.
	Guarded Contract = "guarded"
	// Incomplete is a route whose press states the window and is missing a term the
	// check needs. It will REFUSE rather than press, naming what it lacks — which is
	// the point of it, and the reason it is a state of its own instead of being filed
	// under either neighbour.
	Incomplete Contract = "incomplete"
)

// measuredTerms is every term a guarded press needs beside its basis, as the saved Marco spells
// them.
//
// The same list `internal/oshost` requires at the press, and it has to stay the same list: a
// route this called complete and the guard then refused would be a listing that contradicts the
// run. Held by test rather than by comment — TestEveryTermTheGuardNeedsIsOneThisReads generates a
// guarded route, removes one term at a time, and requires each removal to read as Incomplete.
var measuredTerms = []string{"RelX", "RelY", "WinW", "WinH", "Dpi", "Process"}

// draggedTerms is that list for a DRAG, which is two points in one window: two pairs of offsets,
// and one set of window terms because there is one window — see
// [[ADR-259-a-guarded-drag-is-two-points-in-one-window]].
//
// Its own list rather than the press's, because the saved Marco spells a drag's offsets
// differently and `internal/oshost` asks for them by those names: `FromRelX`/`FromRelY` for the
// start and `ToRelX`/`ToRelY` for the end, each pair required and each refused when absent. A
// drag measured against the press's list would be a listing that contradicts the run in both
// directions — and `FromRelX 193` ENDS IN `RelX 193`, so the press's list would have read a drag
// with one end measured and the other missing as complete.
//
// WHAT ITS ABSENCE DID: a guarded drag read as LEGACY, because the only shape this knew about was
// a Point. Studio showed a routine that checks the window it was recorded in to a person as one
// that checks nothing, and the four desktop drag scenarios all failed on the precondition before
// anything about a drag was exercised. That is what the desktop gate is for.
var draggedTerms = []string{"FromRelX", "FromRelY", "ToRelX", "ToRelY",
	"WinW", "WinH", "Dpi", "Process"}

// everyMeasuredTerm is the order a missing-term list is reported in, and the union of the two
// lists above with each term named once.
//
// A list of its own so that reporting cannot quietly drop a term: a shape's list is what decides
// whether something is MISSING, and this is what decides whether a missing thing is SAID. A term
// added to a shape and not to this would be read, found absent, and never named.
// `TestTheReportedTermsAreTheUnionOfTheShapes` holds the two together.
var everyMeasuredTerm = []string{"RelX", "RelY", "FromRelX", "FromRelY", "ToRelX", "ToRelY",
	"WinW", "WinH", "Dpi", "Process"}

// measuredShapeOf is which set a declaration declares, and therefore which terms the guard will
// ask of it. "" for a line that is not a guarded action's declaration at all.
//
// By the SHAPE and not by the terms present, because the question "is this complete" has no
// answer until something says what complete would be. A reader that collected whatever
// measurements it found would call every shape complete.
func measuredShapeOf(line string) []string {
	switch {
	case strings.Contains(line, "is a Point with"):
		return measuredTerms
	case strings.Contains(line, "is a Drag with"):
		return draggedTerms
	}
	return nil
}

// statesTerm is whether a declaration states this term — the term and its value, and not the tail
// of a longer name.
//
// A term in a saved set always follows `with ` or `, `, which is what makes the distinction
// readable: `, FromRelX 193` does not state `RelX`, and a check written as "the name followed by
// a space appears somewhere in the line" says it does. That cost nothing while a Point was the
// only guarded shape and no two of its terms shared an ending; it is load-bearing now.
func statesTerm(line, term string) bool {
	return strings.Contains(line, ", "+term+" ") ||
		strings.Contains(line, "with "+term+" ")
}

// ContractOf reads a saved route's contract, and what any incomplete press is missing.
//
// Read from the SOURCE, because that is where the contract lives: the saved Marco is the only
// executable truth about a route, and a second record of it beside the file is a second record to
// disagree with it. The same reason `cmd/marco` asks the source whether a run should be checked
// for presses that did not happen.
func ContractOf(src string) (Contract, []string) {
	out, missing := Legacy, map[string]bool{}
	for _, line := range strings.Split(src, "\n") {
		want := measuredShapeOf(line)
		if want == nil || !strings.Contains(line, `RelTo "`) {
			continue
		}
		out = Guarded
		for _, term := range want {
			if !statesTerm(line, term) {
				missing[term] = true
			}
		}
	}
	if len(missing) == 0 {
		return out, nil
	}
	lacks := make([]string, 0, len(missing))
	for _, term := range everyMeasuredTerm {
		if missing[term] {
			lacks = append(lacks, term)
		}
	}
	return Incomplete, lacks
}

// DraftOf is a draft made from a demonstration this process did not capture.
//
// # Why this exists apart from Capture.Finish
//
// Because a demonstration and a CAPTURE are different things. `Finish` has a recorder to wait
// for, a stop to classify, Studio's own window to strip out of the stream and anchors to enrich;
// all of that is about having been the process whose hooks were installed. What is left when none
// of it applies is the part that turns events into a routine — the learn options, `simplify`, and
// the four fields a draft carries — and there are two callers for that.
//
// `Resave` is one: it rebuilds a routine from the demonstration saved beside it, which happened in
// some process that has long since exited. `plugins/e2e` is the other: it seeds a routine into a
// test library from events it measured off a real window, so that the half of the product after
// the save — generation, the file, the contract, the executor, the input path, the guard, the run
// record — can be played against a real application without a person's hands.
//
// Said once rather than twice, because the alternative is a harness with its own idea of what a
// draft is. A seeded routine is worth playing only if it is byte-for-byte the routine the product
// would have written from the same demonstration, and that is not a property anybody can keep by
// remembering to.
//
// IT IS NOT A RECORDER. Nothing here captures anything; `events` is a demonstration that already
// happened, and a caller that invents one is seeding rather than recording — a distinction
// `plugins/e2e` carries all the way into its report.
func (d Deps) DraftOf(name string, events []recorder.RecordedEvent, app, stoppedBy string,
	mode RecordedAs) *Draft {
	return d.draftFor(RebuildCurrent, name, events, app, stoppedBy, mode)
}

// ── WHICH RULES A REBUILD IS RENDERED UNDER ───────────────────────────────────

// Rebuild is which rules a review's version was rendered under.
//
// # Why a bounded word and not a set of options
//
// Because a caller handing over simplifier settings is a caller deciding what somebody's routine
// becomes, from outside the one place that knows what the choices MEAN. There are two choices, a
// person picks between them by name, and the review says which one it is showing. Anything else
// is a dial nobody can describe on a card.
type Rebuild string

const (
	// RebuildCurrent renders the demonstration under today's generator rules and
	// nothing else — the same simplification the original save used.
	//
	// The zero value on purpose: a caller that says nothing gets the rebuild that
	// changes the least.
	RebuildCurrent Rebuild = "current"
	// RebuildSimplifyTyping is that, plus folding a typed run into one step across
	// longer pauses.
	//
	// IT CHANGES TIMING, which is why it is a separate choice with its own sentence
	// rather than an improvement applied quietly. See `(Rebuild) Caution`.
	RebuildSimplifyTyping Rebuild = "simplify-typing"
)

// Offered is the two choices, in the order a surface should show them.
func Offered() []Rebuild { return []Rebuild{RebuildCurrent, RebuildSimplifyTyping} }

// known reports whether this is one of the two.
func (r Rebuild) known() bool { return r == RebuildCurrent || r == RebuildSimplifyTyping }

// Choice is the words on the control that asks for this rebuild.
func (r Rebuild) Choice() string {
	if r == RebuildSimplifyTyping {
		return "Simplify typing pauses"
	}
	return "Rebuild with current rules"
}

// Caution is what this rebuild CHANGES about the routine, beyond the generator's own rules, in
// the words a person reads before choosing it.
//
// # Why the simplified one has to say this
//
// Because "simplify" sounds like a tidy-up and is not one. Folding a typed run into a single step
// removes the waits BETWEEN those keystrokes — up to a second of them — and a routine that typed
// into a box which needed that second to catch up will now type into it faster than it did when
// somebody watched it work. That is a behaviour change, chosen on purpose, and a person is owed
// it in a sentence rather than in a diff of two sources they are being asked to compare.
//
// What it does NOT change is also worth stating, because the sentence above invites the question:
// a wait that was not inside a typed run stays exactly as it was, and so do which button each
// press used, which keys were held across what, and the routine stopping at the first action that
// did not happen.
func (r Rebuild) Caution() string {
	if r != RebuildSimplifyTyping {
		return ""
	}
	return "This folds each run of typing into one step, which removes the pauses " +
		"between those keystrokes — up to a second of them. Everything else is " +
		"unchanged: waits outside a typed run, which button each press used, which " +
		"keys were held across what, and the routine stopping at the first action " +
		"that did not happen."
}

// draftFor is a draft of one stored demonstration, under one of the two sets of rules.
//
// # The mode comes from the SAVED FILE for both of them
//
// `simplify` depends on the stated mode as much as the comment does: with no anchor to wait on,
// the recorded gaps are the only waiting a routine has, so a routine whose file states
// `coordinates` must keep them. Simplifying under the ENVIRONMENT's idea of the mode is the
// defect [[ADR-254-what-a-recording-captures-is-stated]] fixed in `Resave` — a Studio launched
// with `$MARCO_CV=on` capping every recorded gap to 50ms and putting nothing in its place, which
// is a menu pressed before it has drawn — and `SimplifyRoute` had it until this slice. So both
// callers reach the simplification through here, with the mode read off the file.
func (d Deps) draftFor(kind Rebuild, name string, events []recorder.RecordedEvent,
	app, stoppedBy string, mode RecordedAs) *Draft {

	base, argNames := routes.SplitArgs(name)
	opts := d.learnOptionsFor(mode)
	opts.ArgNames = argNames
	if kind == RebuildSimplifyTyping {
		// THE ONE OPTION THE CHOICE CHANGES, set here rather than taken from a
		// caller: see Rebuild.
		opts.TypingRhythmMs = simplify.AggressiveTypingRhythmMs
	}
	steps, refused := simplify.Simplify(events, opts)
	// AND THE ANCHORS ARE RE-LABELLED, because rebuilding from raw events loses the text
	// a previous pass read off each anchored press. `Capture.Finish` does this and the old
	// `SimplifyRoute` did it; `DraftOf` did not, so a repair quietly dropped them. A no-op
	// unless a Text or Vision host is wired, which is every coordinate-mode routine.
	d.enrichAnchors(steps)
	return &Draft{Name: base, Args: argNames,
		Steps: steps, Events: events, Unrepresentable: reasonOf(refused),
		App: app, StoppedBy: stoppedBy, Mode: mode}
}

// reasonOf is a refusal as the sentence a draft carries, and "" for no refusal.
//
// One line, and it exists so that the two places that build a draft cannot differ about what an
// absent refusal looks like — an `err.Error()` on a nil error panics, and a `!= nil` check
// written twice is a check that gets written once.
func reasonOf(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// render is the source a draft becomes, and the asset files with it. NOTHING IS WRITTEN.
//
// # Why it is its own function with two callers
//
// Because the second caller's whole promise is that what a person reviewed is what gets written.
// `Save` used to decide this inline, and a review built from a copy of those four lines would be
// a second answer to "what does this draft become" — which is the shape of defect this repository
// keeps finding: `codegen` and `oshost` holding two lists of the same terms, a recorder and a
// reader holding two ideas of one mode. Here it would be worse than a disagreement about a
// listing, because the disagreement would be between the source somebody approved and the source
// that replaced theirs.
//
// The scope decides two things — whether the application is brought forward, and therefore
// whether a leading `Activate` belongs in the steps — and a regeneration passes the scope the
// route ALREADY has, which is how it cannot quietly rescope somebody's routine.
func (d Deps) render(dr *Draft, mode scopeMode, app string, rt routes.Route) (string,
	map[string][]byte, error) {

	steps := dr.Steps
	codegenApp := ""
	switch mode {
	case scopeContext:
		steps = stripLeadingActivate(steps)
	case scopeFocus:
		codegenApp = app
	default:
		steps = stripLeadingActivate(steps)
	}
	// THE DRAFT'S OWN MODE, not this Deps'. A draft can outlive the window state that
	// made it — recorded, reviewed, saved under another name minutes later — and the
	// recording is what the line describes.
	return codegen.RouteAs(codegen.Recorded{Mode: string(dr.Mode)},
		dr.Name, codegenApp, steps, d.Reg.ScopeDir(rt), dr.Args...)
}

// ScopeAlreadyOf is the scope a SAVED routine is already under, read off where it lives.
//
// The route's own place is the durable account of its scope — `routes/<app>/focus` brings the
// application forward, `routes/<app>/context` works only while it is already in front, and
// `routes/global` is about no application — so this is a reading of the library rather than a
// choice. A regeneration uses it to write the routine back exactly where and as it was.
func ScopeAlreadyOf(rt routes.Route) Scope {
	switch {
	case rt.App == "":
		return ScopeEverywhere
	case rt.Focus:
		return ScopeAppAnywhere
	}
	return ScopeApp
}
