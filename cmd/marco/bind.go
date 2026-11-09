package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recordingnow"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// LEADER SHORTCUTS FROM THE TERMINAL — and from the widget, whose `bind` and `unbind` words spawn
// these commands. Every rule is `orchestrator`'s (shortcuts.go); this file only COLLECTS the
// answers: which key, which routine, listened for where, and whether to replace what is there.
//
// "Listened for where" is the foreground at the moment somebody types, unless they say:
// `--everywhere`, or `--in=<app>`. That is the widget's meaning of `bind`, kept (R4).

// whereFlags reads `--everywhere` / `--in=<app>` off the front of args.
func whereFlags(args []string) (listenIn string, stated bool, rest []string, err error) {
	for len(args) > 0 && strings.HasPrefix(args[0], "--") {
		switch a := args[0]; {
		case a == "--everywhere":
			if stated {
				return "", false, nil, fmt.Errorf("say --everywhere or --in=<app>, not both")
			}
			listenIn, stated = "", true
		case strings.HasPrefix(a, "--in="):
			if stated {
				return "", false, nil, fmt.Errorf("say --everywhere or --in=<app>, not both")
			}
			listenIn, stated = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(a, "--in="))), true
			if listenIn == "" {
				return "", false, nil, fmt.Errorf("--in needs an application")
			}
		default:
			return listenIn, stated, args, nil
		}
		args = args[1:]
	}
	return listenIn, stated, args, nil
}

// runBind: `marco bind [--everywhere|--in=<app>] <key> "<routine>[ then …]"`, or
// `marco bind --reassign [--everywhere|--in=<app>] <key>` for a shortcut set up by name.
func runBind(args []string) {
	reassign := false
	if len(args) > 0 && args[0] == "--reassign" {
		reassign, args = true, args[1:]
	}
	listenIn, stated, args, err := whereFlags(args)
	if err == nil && len(args) > 0 && args[0] == "--reassign" {
		reassign, args = true, args[1:]
	}
	if err != nil || (reassign && len(args) != 1) || (!reassign && len(args) < 2) {
		if err != nil {
			fmt.Fprintln(os.Stderr, "marco: "+err.Error())
		}
		fmt.Fprintln(os.Stderr, `usage: marco bind [--everywhere|--in=<app>] <key> "<play>"`)
		fmt.Fprintln(os.Stderr, `       marco bind --reassign [--everywhere|--in=<app>] <key>`)
		os.Exit(2)
	}
	fg := winctx.Active()
	if !stated {
		listenIn = fg
	}
	d := newDeps()
	key := strings.ToLower(args[0])
	if reassign {
		err = reassignKey(d, fg, listenIn, key, os.Stdout)
	} else {
		err = bindKey(d, fg, listenIn, key, strings.TrimSpace(strings.Join(args[1:], " ")),
			os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// configuredLeader is the leader key the widget is using, which cannot itself be bound. Read the
// way the widget reads it, by the one reader Studio uses too (internal/widgetconfig).
var configuredLeader = widgetconfig.Leader

// resolveSteps resolves each step of cmd EXACTLY, once, in the foreground's context — the same
// lookup a typed name gets — and returns the routines. This is the only place a shortcut's words
// are ever looked up; afterwards it holds identities.
//
// Deleting the exact check must fail TestABindingIsValidatedTheWayItWillBeResolved.
func resolveSteps(d orchestrator.Deps, fg, cmd string) ([]routes.Route, error) {
	var steps []routes.Route
	for _, step := range routes.SplitChain(cmd) {
		base, _, _ := routes.ParseInvocation(step)
		rt, ok := d.Reg.Resolve(fg, base)
		if !ok {
			// "no play matches " is the engine's one unknown-command prefix, shared with
			// panicstop.go and PREFIX-MATCHED by plugins/overlay (acts.go, const
			// noPlayMatches). Changing the wording on one side only breaks the overlay's
			// offer-to-learn silently — it simply stops recognising the error.
			//
			// Deleting the shared spelling must fail TestTheUnknownCommandErrorIsOnePrefix.
			return nil, fmt.Errorf("no play matches %q (learn it first)", step)
		}
		steps = append(steps, rt)
	}
	return steps, nil
}

// bindKey is the whole of `marco bind <key> <routine>`, with the registry, the foreground, the
// answers and the output handed in so a test can enter it.
func bindKey(d orchestrator.Deps, fg, listenIn, key, cmd string, out io.Writer) error {
	steps, err := resolveSteps(d, fg, cmd)
	if err != nil {
		return err
	}
	return assignWithConsent(d, orchestrator.Assignment{Key: key, ListenIn: listenIn,
		Steps: steps, Cmd: cmd, Leader: configuredLeader()}, out)
}

// reassignKey turns a shortcut set up by name into one that names its routines — after showing
// which routines those are and asking. The words and any configured values are kept as they were.
func reassignKey(d orchestrator.Deps, fg, listenIn, key string, out io.Writer) error {
	st := d.ShortcutAt(listenIn, key)
	if !st.Exists {
		return fmt.Errorf("there is no shortcut on `%s %s", key, where(listenIn))
	}
	if !st.Binding.Legacy() {
		return fmt.Errorf("`%s %s already names its play: %s", key, where(listenIn), st.Says)
	}
	steps, err := resolveSteps(d, fg, st.Binding.Command())
	if err != nil {
		return fmt.Errorf("`%s %s was set up by name as %q, and that name does not "+
			"reach a play here: %v", key, where(listenIn), st.Binding.Command(), err)
	}
	ids := make([]string, len(steps))
	for i, rt := range steps {
		ids[i] = rt.ID()
	}
	fmt.Fprintf(out, "`%s %s was set up by name as %q. Here that is %s.\n", key,
		where(listenIn), st.Binding.Command(), strings.Join(ids, " then "))
	if !confirmNo(out, "Make the shortcut run exactly that? [y]es / [N]o: ") {
		fmt.Fprintln(out, "Left as it is. It still does not run until it is reassigned.")
		return nil
	}
	b, err := d.AssignShortcut(orchestrator.Assignment{Key: key, ListenIn: listenIn,
		Steps: steps, Cmd: st.Binding.Command(), Leader: configuredLeader(),
		Consent: st.Version})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "reassigned `%s %s -> %s (%s)\n", b.Key, where(b.App), b.Cmd,
		strings.Join(b.Routes, ", "))
	return nil
}

// assignWithConsent assigns, and when something is already on that key asks with Enter meaning NO,
// carrying the version it showed as the consent.
func assignWithConsent(d orchestrator.Deps, a orchestrator.Assignment, out io.Writer) error {
	if why := d.WhyNotShortcut(a); why != "" {
		return fmt.Errorf("not bound: %s", why)
	}
	if st := d.ShortcutAt(a.ListenIn, a.Key); st.Exists {
		fmt.Fprintln(out, st.Says+".")
		if !confirmNo(out, "Replace it? [y]es / [N]o: ") {
			fmt.Fprintln(out, "Left as it is.")
			return nil
		}
		a.Consent = st.Version
	}
	b, err := d.AssignShortcut(a)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "bound `%s -> %s in %s (%s)\n", b.Key, b.Cmd, scopeWord(b.App),
		strings.Join(b.Routes, ", "))
	return nil
}

// askLine reads one answer. A variable so a test can answer.
var askLine = func() string { s, _ := readStdinLine(); return s }

// confirmNo asks a yes/no question where Enter, end of input and anything else mean NO.
func confirmNo(out io.Writer, prompt string) bool {
	fmt.Fprint(out, prompt)
	switch strings.TrimSpace(strings.ToLower(askLine())) {
	case "y", "yes":
		return true
	}
	return false
}

func where(app string) string {
	if app == "" {
		return "everywhere"
	}
	return "in " + app
}

// scopeWord is the bind confirmation's "in <app>" / "in everywhere" — the spelling this line has
// always had, which the widget shows as it arrives.
func scopeWord(app string) string {
	if app == "" {
		return "everywhere"
	}
	return app
}

// runUnbind: `marco unbind [--everywhere|--in=<app>] <key>` — take a shortcut off, saying what it was.
func runUnbind(args []string) {
	listenIn, stated, args, err := whereFlags(args)
	if err != nil || len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: marco unbind [--everywhere|--in=<app>] <key>")
		os.Exit(2)
	}
	if !stated {
		listenIn = winctx.Active()
	}
	if err := unbindKey(newDeps(), listenIn, strings.ToLower(args[0]), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// unbindKey removes exactly the shortcut it just read, so one that changed in between is refused.
func unbindKey(d orchestrator.Deps, listenIn, key string, out io.Writer) error {
	st := d.ShortcutAt(listenIn, key)
	if !st.Exists {
		fmt.Fprintf(out, "there is no shortcut on `%s %s\n", key, where(listenIn))
		return nil
	}
	b, err := d.RemoveShortcut(listenIn, key, st.Version)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "unbound `%s %s (it ran %s)\n", b.Key, where(b.App), b.Command())
	return nil
}

// runBindings: `marco bindings [--here] [--armed-in=<app>] [--json]` — every shortcut, or what
// each key fires here. --armed-in is the widget's: the application that was in front when its
// leader was pressed, so the list is of the context the key will be checked against (see
// pressArmed), not of whatever is in front by the time this process reads it.
func runBindings(args []string) {
	app, here, carried, asJSON, ok := bindingsArgs(args, winctx.Active)
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: marco bindings [--here] [--armed-in=<app>] [--json]")
		os.Exit(2)
	}
	listShortcuts(newDeps(), app, here, carried, asJSON, os.Stdout)
}

// bindingsArgs reads `marco bindings`'s arguments. The application listed is the one the leader
// carried, else with --here the one in front (read only then), else none.
func bindingsArgs(args []string, front func() string) (app string, here, armed, asJSON, ok bool) {
	for _, a := range args {
		switch {
		case a == "--here":
			here = true
		case a == "--json":
			asJSON = true
		case strings.HasPrefix(a, armedFlag):
			app, armed = strings.TrimPrefix(a, armedFlag), true
		default:
			return "", false, false, false, false
		}
	}
	if armed {
		here = true
	} else if here {
		app = front()
	}
	return app, here, armed, asJSON, true
}

// listShortcuts writes the list. carried is whether app is the one a leader was pressed in, which a
// JSON listing echoes as `armed_in`: that echo is how the widget knows this engine checks a press
// against it (pressArmed), since an engine older than the check either refuses `--armed-in` or has
// no `bindings` at all. Dropping the echo must fail TestAListingSaysItWasForTheArmedApplication.
func listShortcuts(d orchestrator.Deps, app string, here, carried, asJSON bool, out io.Writer) {
	rows := d.ListShortcuts(app, here)
	held := heldOff()
	if asJSON {
		if rows == nil {
			rows = []orchestrator.ListedShortcut{}
		}
		doc := map[string]any{"app": app, "shortcuts": rows}
		if carried {
			doc["armed_in"] = app
		}
		if held != "" {
			doc["held"] = held
		}
		_ = json.NewEncoder(out).Encode(doc)
		return
	}
	if held != "" {
		fmt.Fprintln(out, held)
	}
	if len(rows) == 0 {
		if here {
			fmt.Fprintf(out, "no shortcuts %s\n", where(app))
		} else {
			fmt.Fprintln(out, "no shortcuts")
		}
		return
	}
	for _, r := range rows {
		mark := ""
		switch r.State {
		case "needs-reassigning":
			mark = "  [needs reassigning]"
		case "stale":
			mark = "  [play gone]"
		case "wider":
			mark = "  [wider than its play]"
		}
		if r.Shadowed {
			mark += "  [hidden here by an app shortcut]"
		}
		fmt.Fprintf(out, "`%-3s %-18s %s%s\n", r.Key, where(r.ListenIn), r.Cmd, mark)
	}
}

// exitProcess ends the process with a code.
//
// A package variable so a test can enter `pressHotkey` — production code — without the test
// binary exiting underneath it. Production never reassigns it.
var exitProcess = os.Exit

// heldOff is why every shortcut is held off right now, or "". It is the press's own reading (see
// pressHotkey), so a list never says a key runs while a press of it would refuse.
//
// The press's sentence, without its "nothing was performed": a list is read before anything is
// pressed. TestTheArmListSaysARecordingHoldsShortcutsOff holds that the press still says it.
func heldOff() string {
	const performed = " — nothing was performed"
	switch m, st := recordingnow.InProgress(recordingHome()); st {
	case recordingnow.Recording:
		return strings.TrimSuffix(m.Says(), performed)
	case recordingnow.Unknown:
		return "Marco cannot tell whether a recording is in progress, so shortcuts are held off"
	}
	return ""
}

// runHotkey: `marco hotkey <key>` — run the shortcut on the leader key in the current foreground
// app. A key with nothing on it performs nothing and says so; it announces no outcome, because
// nothing was invoked.
//
// `--armed-in=<app>` is the widget's: the application that was in front when its leader was
// pressed. With it, the press is checked against the foreground it reads itself (pressArmed).
func runHotkey(args []string) {
	key, armedIn, carried := hotkeyArgs(args)
	if key == "" {
		return
	}
	pressArmed(newDeps(), winctx.Active(), armedIn, carried, key)
}

// hotkeyArgs reads `marco hotkey`'s arguments: the key, lower-cased, and the application its
// leader was pressed in if one was carried.
func hotkeyArgs(args []string) (key, armedIn string, carried bool) {
	for _, a := range args {
		if v, ok := strings.CutPrefix(a, armedFlag); ok {
			armedIn, carried = v, true
		} else if key == "" {
			key = strings.ToLower(a)
		}
	}
	return key, armedIn, carried
}

// armedFlag carries the application a leader was pressed in, from the widget to the engine.
const armedFlag = "--armed-in="

// pressArmed is a press that knows which application its leader was pressed in.
//
// THE KEY IS FOR THE APPLICATION THE LEADER WAS PRESSED IN. The widget lists what each key does
// there, and a person who reads `e  cloud pull stone` and presses e means that routine. If
// another application has come to the front in between, the same key can be bound to something
// else there, and each lookup on its own would be correct while the press ran a routine nobody
// was shown. So the foreground this process reads is compared with the one the leader carried,
// BEFORE any binding is selected, and a change refuses and asks for the leader again. A press
// with no context carried (a script calling `marco hotkey`) is a press in whatever is in front.
//
// Deleting the comparison must fail TestAKeyPressedAfterTheFocusMovedRunsNothing.
func pressArmed(d orchestrator.Deps, front, armedIn string, carried bool, key string) {
	if carried && front != armedIn {
		fmt.Fprintln(os.Stdout, contextMoved(armedIn, front, key))
		announce(OutcomeRefused)
		exitProcess(OutcomeRefused.Exit())
		return
	}
	pressHotkey(d, front, key)
}

// contextMoved is what a press says when the application in front is not the one its leader was
// pressed in.
func contextMoved(armedIn, front, key string) string {
	named := func(app string) string {
		if app == "" {
			return "nothing that could be read"
		}
		return app
	}
	leader := configuredLeader()
	return widgetconfig.Pressed(leader, key) + " was pressed for " + named(armedIn) + ", and " +
		named(front) + " is in front now — nothing was performed; press " +
		widgetconfig.Named(leader) + " again"
}

// betweenPressAndRun is the instant after a press resolved its routines and before it runs them,
// where a test can publish something else at the identity. A no-op in production.
var betweenPressAndRun = func() {}

// recordingHome is where a recording in progress is announced. A variable so a test can state one.
var recordingHome = marcopaths.Home

// pressHotkey is the whole of what a press does, with the registry and the foreground handed in.
//
// Split out so a test can ENTER IT rather than re-implement it. See the memory note "prove wiring
// by deleting it".
func pressHotkey(d orchestrator.Deps, app, key string) {
	// ── A RECORDING IN PROGRESS HOLDS EVERY SHORTCUT OFF ──
	//
	// First, before anything is looked up: a shortcut injects real input, and pressed during a
	// demonstration it would type into the window being recorded. Only if one is BOUND — an
	// unbound key over a game stays silent. See internal/recordingnow.
	//
	// Deleting this must fail TestARecordingHoldsShortcutsOff.
	p := d.ShortcutPressed(app, key)
	if !p.Found {
		// ── NOTHING ON IT IS SAID, AND NOTHING IS ANNOUNCED ──
		//
		// It used to be silent, which left the widget nothing to say about a key that did
		// nothing. No `[result] ` line: no routine was asked to do anything, so none of the
		// seven words is true of it. The widget reads "exited cleanly, announced nothing" as
		// exactly this. The foreground named is the one this press read.
		//
		// Deleting the sentence must fail TestTheArmListIsWhatAPressWouldFire.
		fmt.Fprintln(os.Stdout, nothingOn(app, key))
		return
	}
	// UNKNOWN IS REFUSED TOO. A press that cannot tell whether somebody is recording does not
	// find out by typing into their window.
	switch m, st := recordingnow.InProgress(recordingHome()); st {
	case recordingnow.Recording:
		fmt.Fprintln(os.Stdout, m.Says())
		announce(OutcomeRefused)
		exitProcess(OutcomeRefused.Exit())
		return
	case recordingnow.Unknown:
		fmt.Fprintln(os.Stdout, recordingnow.UnknownSays())
		announce(OutcomeRefused)
		exitProcess(OutcomeRefused.Exit())
		return
	}
	// ── A SHORTCUT NAMES ITS ROUTINES, AND NOTHING ELSE IS LOOKED UP ──
	//
	// It used to store words and resolve them on every press, by name, with the foreground's
	// four-scope priority — so a key bound to one routine ran a namesake once one existed, and a
	// new routine saved under an old name inherited the key. Now each step is the identity
	// recorded at assignment, performed through the one intake as an EXPLICIT identity, or the
	// press refuses and says why: set up by name and not reassigned, or a routine that is gone.
	//
	// Deleting the identity and passing text must fail TestAHotkeyPerformsTheBoundPlayItself.
	if p.Refused != "" {
		fmt.Fprintln(os.Stdout, p.Refused)
		announce(OutcomeRefused)
		exitProcess(OutcomeRefused.Exit())
		return
	}
	betweenPressAndRun()
	for _, step := range p.Steps {
		// THE STEP'S WORDS GIVE ITS CONFIGURED VALUES ONLY. The routine is the identity.
		_, named, positional := routes.ParseInvocation(step.Text)
		rt := step.Route
		// PINNED to the bytes the owner read under the routine's lock while the key still
		// named it, so a routine published at this identity since is refused, not run.
		//
		// Dropping the pin must fail TestAPressDoesNotRunWhatWasPublishedAfterItResolved.
		out, err := runInvocation(d, invoke.Request{
			Source: invoke.SourceHotkey, App: app, Play: &rt,
			Named: named, Positional: positional, Pinned: step.Pin,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		announce(out)
		if out != OutcomePerformed {
			// A chain stops at the first step that did not perform. Nothing further is
			// attempted, and the outcome travels out as this process's exit code.
			exitProcess(out.Exit())
			return
		}
	}
}

// nothingOn is what a press of a key with no shortcut on it says.
func nothingOn(app, key string) string {
	if app == "" {
		return "nothing on " + widgetconfig.Pressed(configuredLeader(), key)
	}
	return "nothing on " + widgetconfig.Pressed(configuredLeader(), key) + " in " + app
}
