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
}

// Begin starts the recorder and returns the capture in flight.
//
// The only failure is the recorder refusing to start, which is a platform fact and is the one
// thing a caller can act on before a person has demonstrated anything.
func (d Deps) Begin(name string) (*Capture, error) {
	name, argNames := routes.SplitArgs(name)
	stop := recorder.ParseStopKey(d.StopKey)
	mlog.Debug("draft: starting recorder", "route", name, "stop_key", stop.Label())
	if err := d.Rec.Start(); err != nil {
		return nil, fmt.Errorf("can't record on this platform: %w", err)
	}
	return &Capture{d: d, name: name, argNames: argNames, stop: stop}, nil
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
	// THE OWNER'S WINDOW COMES OUT WHEREVER IT IS, not only at the end. See the
	// package note above for why the tail alone is not enough here.
	if d.Owner != "" || d.OwnerPID != 0 {
		events = withoutRedundantReturns(withoutOwner(events, d.Owner, d.OwnerPID))
	}

	opts := d.learnOptions()
	opts.ArgNames = argNames
	steps := simplify.Simplify(events, opts)
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
	}
}

// Save writes a draft as legal Marco, under the scope it is given.
//
// The scope is STATED rather than inferred, which is the other half of separating Save from
// Stop: a window asks where a routine should work and then says so, where the terminal prompts
// for it and the HUD guessed. `app` is the application the scope is about — the draft's own
// observed app unless the caller names another — and is ignored by `ScopeEverywhere`.
func (d Deps) Save(dr *Draft, scope Scope, app string) (routes.Route, error) {
	if dr == nil {
		return routes.Route{}, fmt.Errorf("there is no draft to save")
	}
	if len(dr.Steps) == 0 {
		return routes.Route{}, fmt.Errorf("nothing was recorded, so there is " +
			"nothing to save")
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
	steps := dr.Steps
	rt := routes.Route{Slug: routes.Slug(dr.Name)}
	codegenApp := ""
	switch mode {
	case scopeContext:
		rt.App = app
		steps = stripLeadingActivate(steps)
	case scopeFocus:
		rt.App, rt.Focus = app, true
		codegenApp = app
	default:
		steps = stripLeadingActivate(steps)
	}
	src, assets, err := codegen.Route(dr.Name, codegenApp, steps,
		d.Reg.ScopeDir(rt), dr.Args...)
	if err != nil {
		return rt, err
	}
	if err := d.Reg.Save(rt, src); err != nil {
		mlog.Error("draft: save failed", "route", dr.Name, "err", err)
		return rt, err
	}
	if err := d.Reg.WriteAssets(assets); err != nil {
		return rt, err
	}
	// AND THE RAW DEMONSTRATION BESIDE IT, for review and re-simplification. Not an
	// execution path: nothing reads it to run anything. Best-effort, as the other
	// save paths treat it.
	if raw, mErr := json.Marshal(dr.Events); mErr == nil {
		_ = d.Reg.SaveRecording(rt, raw)
	}
	mlog.Info("draft: saved", "route", dr.Name, "scope", string(scope),
		"app", app, "steps", len(steps), "path", d.Reg.Path(rt))
	return rt, nil
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
	name, argNames := routes.SplitArgs(u.Name)
	opts := d.learnOptions()
	opts.ArgNames = argNames
	dr := &Draft{Name: name, Args: argNames,
		Steps: simplify.Simplify(events, opts), Events: events,
		App: u.Route.App, StoppedBy: "a repair"}
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
		"was", string(ScopeApp), "now", string(ScopeAppAnywhere))
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

// ContractOf reads a saved route's contract, and what any incomplete press is missing.
//
// Read from the SOURCE, because that is where the contract lives: the saved Marco is the only
// executable truth about a route, and a second record of it beside the file is a second record to
// disagree with it. The same reason `cmd/marco` asks the source whether a run should be checked
// for presses that did not happen.
func ContractOf(src string) (Contract, []string) {
	out, missing := Legacy, map[string]bool{}
	for _, line := range strings.Split(src, "\n") {
		if !strings.Contains(line, "is a Point with") || !strings.Contains(line, `RelTo "`) {
			continue
		}
		out = Guarded
		for _, term := range measuredTerms {
			// THE TERM AND ITS VALUE, not the word anywhere in the line. A
			// declaration ending `Process "arkascended.exe"` contains the word
			// `Dpi` nowhere, and one containing it as part of a longer name is
			// not this term.
			if !strings.Contains(line, term+" ") {
				missing[term] = true
			}
		}
	}
	if len(missing) == 0 {
		return out, nil
	}
	lacks := make([]string, 0, len(missing))
	for _, term := range measuredTerms {
		if missing[term] {
			lacks = append(lacks, term)
		}
	}
	return Incomplete, lacks
}
