// Package orchestrator implements the named-route teach loop: run a route by
// name if it exists, otherwise ask the user to demonstrate it, record the
// demonstration, simplify it, generate a Marco route, save it, and run it — so
// next time the same name just runs. OS-agnostic; the OS-specific work is the
// injected recorder and host.
package orchestrator

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/simplify"
	"github.com/chaynes-simpleclouds/marco/internal/voiceteach"
)

// Deps are the orchestrator's collaborators (injectable for testing).
type Deps struct {
	Reg   routes.Registry
	Rec   recorder.Recorder
	Hosts map[string]runtime.Host // route execution hosts (e.g. {"*": oshost.New()})
	In    io.Reader               // prompts / confirmations
	Out   io.Writer               // user-facing output
	App   func() string           // current foreground app; nil disables context-awareness
	// StopKey is the gesture that ends a recording (e.g. "f12", "esc",
	// "ctrl+f12"). Empty uses recorder.DefaultStopKey. Keeping it off Esc lets a
	// macro record Esc as an ordinary key.
	StopKey string
	// SimplifySaves makes the teach "[s]implify" choice simplify AND save in one
	// step, with no re-confirmation. It's for a front-end that can't show the
	// simplified preview (the overlay HUD) — re-confirming a result you can't see
	// is pointless. The CLI leaves it false so the preview-then-confirm stays.
	SimplifySaves bool
	// ArgKey is the reserved demonstration key that drops a positional argument
	// placeholder ("an argument goes here" → {{N}}); empty uses simplify.DefaultArgKey
	// (F9). Set from $MARCO_ARG_KEY by the binary. "off" disables it.
	ArgKey string
}

// teachOptions are the simplify defaults for a demonstration, honoring the
// configured arg key.
func (d Deps) teachOptions() simplify.Options {
	o := simplify.DefaultOptions()
	if k := strings.TrimSpace(d.ArgKey); k != "" {
		o.ArgKey = strings.ToLower(k)
	}
	return o
}

// argKeyHint is the recording prompt's note about the arg key (or "" when off),
// listing the declared arg names when the teach phrase had a "with …" clause.
func (d Deps) argKeyHint(argNames []string) string {
	k := d.teachOptions().ArgKey
	if k == "" || k == "off" {
		return ""
	}
	if len(argNames) > 0 {
		return fmt.Sprintf(" (tap %s where each arg goes, in order: %s)", strings.ToUpper(k), strings.Join(argNames, ", "))
	}
	return fmt.Sprintf(" (tap %s where a value should be an argument)", strings.ToUpper(k))
}

// activeApp returns the foreground app name, or "" when unavailable.
func (d Deps) activeApp() string {
	if d.App == nil {
		return ""
	}
	return d.App()
}

// Do runs the route best matching name in the current app context; if there
// isn't one, it teaches it, then offers to run.
func (d Deps) Do(name string) error {
	app := d.activeApp()
	if rt, ok := d.Reg.Resolve(app, name); ok {
		return d.Run(rt)
	}
	fmt.Fprintf(d.Out, "I don't know %q yet.\n", name)
	if err := d.Teach(name); err != nil {
		return err
	}
	if rt, ok := d.Reg.Resolve(d.activeApp(), name); ok && d.confirm("Run it now? [y]es / [n]o: ") {
		return d.Run(rt)
	}
	return nil
}

// Teach records a demonstration of name, simplifies it into a route, and saves
// it. The user performs the actions and presses Esc to finish.
func (d Deps) Teach(name string) error {
	// "say hello with name, count" declares named args; the route is the part before
	// "with". Reassign name to the route so the rest of the flow (slug, codegen, save)
	// is unchanged.
	name, argNames := routes.SplitArgs(name)
	stop := recorder.ParseStopKey(d.StopKey)
	fmt.Fprintf(d.Out, "Show me how to %q. Do it now, then press %s when finished.\n", name, stop.Label())
	fmt.Fprintf(d.Out, "(For a password, type {{name}} and set it with: marco secret set <name>.%s)\n", d.argKeyHint(argNames))
	if err := d.Rec.Start(); err != nil {
		return fmt.Errorf("can't record on this platform: %w", err)
	}
	waitForStop(d.Rec, &stop)
	events := stripTrailingStop(d.Rec.Stop(), stop)

	// The faithful recording is the default; "simplify further" re-runs the whole
	// pipeline from the raw events with typing-rhythm folding on — re-running from
	// events (not the folded steps) lets typing coalesce into Type steps before
	// loop-folding ever sees repeated letters.
	opts := d.teachOptions()
	opts.ArgNames = argNames
	steps := simplify.Simplify(events, opts)
	if len(steps) == 0 {
		fmt.Fprintln(d.Out, "Nothing was recorded — not saving.")
		return nil
	}
	// The route's primary app — for scope and the context comment — is the app it
	// actually starts working in (the recorder's first detected app switch). When
	// the recorder supplied no app info (e.g. a non-Windows backend), fall back to
	// the foreground app now.
	app := firstActivateApp(steps)
	if app == "" {
		app = d.activeApp()
	}
	// Preview with relative template names; the saved copy embeds the real
	// per-scope directory once scope is chosen below.
	src, _, err := codegen.Route(name, app, steps, "", argNames...)
	if err != nil {
		return err
	}
	if app != "" {
		fmt.Fprintf(d.Out, "(context: %s)\n", app)
	}
	fmt.Fprintf(d.Out, "Recorded %d steps:\n--- route ---\n%s\n", len(steps), src)

	// Save / discard / simplify-further. Simplify is a one-shot (it folds as far
	// as it goes), so once it's run the prompt drops back to just save/discard.
	for {
		action := d.chooseSave(name, opts.TypingRhythmMs == 0)
		if action == actionDiscard {
			fmt.Fprintln(d.Out, "Discarded.")
			return nil
		}
		if action == actionSave {
			break
		}
		// actionSimplify (only offered while not yet simplified)
		opts.TypingRhythmMs = simplify.AggressiveTypingRhythmMs
		steps = simplify.Simplify(events, opts)
		if src, _, err = codegen.Route(name, app, steps, "", argNames...); err != nil {
			return err
		}
		fmt.Fprintf(d.Out, "Simplified to %d steps:\n--- route ---\n%s\n", len(steps), src)
		// On a front-end that can't show the preview (the overlay), "simplify" means
		// "simplify and save" — don't re-prompt for a result the user can't see.
		if d.SimplifySaves {
			break
		}
	}
	// Scope: app-only is the sensible default and the happy path (just say yes), so
	// ask to keep it there; "no" opts into global. Empty/Enter/yes → scoped to the
	// app it was taught in; no → available everywhere.
	scope := app
	if app != "" && !d.confirm(fmt.Sprintf("Make it available only in %s? [y]es / [n]o: ", app)) {
		scope = ""
	}
	// Regenerate with the real scope directory so image-template paths resolve,
	// then save the route and write the template files beside it.
	finalSrc, assets, gErr := codegen.Route(name, app, steps, d.Reg.ScopeDir(scope), argNames...)
	if gErr != nil {
		return gErr
	}
	if err := d.Reg.Save(scope, name, finalSrc); err != nil {
		return err
	}
	if err := d.Reg.WriteAssets(assets); err != nil {
		return err
	}
	// Keep the raw demonstration beside the route so it can be re-simplified
	// later (marco simplify). Best-effort: a failure here doesn't fail the teach.
	if data, mErr := json.Marshal(events); mErr == nil {
		_ = d.Reg.SaveRecording(scope, name, data)
	}
	rt := routes.Route{App: scope, Slug: routes.Slug(name)}
	where := "everywhere"
	if scope != "" {
		where = "in " + scope
	}
	fmt.Fprintf(d.Out, "Learned %q (%s) → %s\n", name, where, d.Reg.Path(rt))
	return nil
}

// TeachAuto records a demonstration and saves it with NO interactive prompts —
// it simplifies with defaults and scopes the route to the app it was taught in
// (global only if no app context). This is what the overlay uses so teaching is
// driven entirely from the HUD (record → F12 → saved) without a console window.
func (d Deps) TeachAuto(name string) error {
	name, argNames := routes.SplitArgs(name)
	stop := recorder.ParseStopKey(d.StopKey)
	fmt.Fprintf(d.Out, "Recording %q — do it now, press %s when done.%s\n", name, stop.Label(), d.argKeyHint(argNames))
	if err := d.Rec.Start(); err != nil {
		return fmt.Errorf("can't record on this platform: %w", err)
	}
	waitForStop(d.Rec, &stop)
	events := stripTrailingStop(d.Rec.Stop(), stop)

	opts := d.teachOptions()
	opts.ArgNames = argNames
	steps := simplify.Simplify(events, opts)
	if len(steps) == 0 {
		fmt.Fprintln(d.Out, "Nothing was recorded — not saving.")
		return nil
	}
	app := firstActivateApp(steps)
	if app == "" {
		app = d.activeApp()
	}
	scope := app // auto: scope to the app it was taught in
	finalSrc, assets, err := codegen.Route(name, app, steps, d.Reg.ScopeDir(scope), argNames...)
	if err != nil {
		return err
	}
	if err := d.Reg.Save(scope, name, finalSrc); err != nil {
		return err
	}
	if err := d.Reg.WriteAssets(assets); err != nil {
		return err
	}
	if data, mErr := json.Marshal(events); mErr == nil {
		_ = d.Reg.SaveRecording(scope, name, data)
	}
	where := "everywhere"
	if scope != "" {
		where = "in " + scope
	}
	fmt.Fprintf(d.Out, "Learned %q (%s), %d steps\n", name, where, len(steps))
	return nil
}

// TeachVoice builds a route by narration instead of demonstration: each phrase from
// the phrases channel is parsed and applied to a voiceteach.Session reading the live
// cursor/screen through env (status, if non-nil, gets a short line per step for a
// HUD). It saves on "done" (scoped to the app, like TeachAuto), discards on "cancel".
func (d Deps) TeachVoice(name string, env voiceteach.Env, phrases <-chan string, status func(string)) error {
	name, argNames := routes.SplitArgs(name)
	sess := voiceteach.New(env, argNames...)
	fmt.Fprintf(d.Out, "Narrate %q — say what to do (\"click this\", \"type …\", \"wait for this screen\"), then \"done\".\n", name)
	for phrase := range phrases {
		st, done, canceled := sess.Apply(voiceteach.Parse(phrase))
		fmt.Fprintf(d.Out, "  %s\n", st)
		if status != nil {
			status(st)
		}
		if canceled {
			fmt.Fprintln(d.Out, "Canceled — nothing saved.")
			return nil
		}
		if done {
			break
		}
	}
	steps := sess.Steps()
	if len(steps) == 0 {
		fmt.Fprintln(d.Out, "Nothing narrated — not saving.")
		return nil
	}
	app := firstActivateApp(steps)
	if app == "" {
		app = d.activeApp()
	}
	scope := app
	src, assets, err := codegen.Route(name, app, steps, d.Reg.ScopeDir(scope), argNames...)
	if err != nil {
		return err
	}
	if err := d.Reg.Save(scope, name, src); err != nil {
		return err
	}
	if err := d.Reg.WriteAssets(assets); err != nil {
		return err
	}
	where := "everywhere"
	if scope != "" {
		where = "in " + scope
	}
	fmt.Fprintf(d.Out, "Learned %q (%s), %d steps\n", name, where, len(steps))
	return nil
}

// Run executes a specific route.
func (d Deps) Run(rt routes.Route) error {
	return driver.RunFileWithHosts(d.Reg.Path(rt), d.Out, d.Hosts)
}

// SimplifyRoute re-simplifies an already-saved route from its stored
// demonstration — folding it as far as it goes (typing rhythm included) — shows
// the result, and overwrites the route in place if confirmed. A route with no
// stored recording (taught before recordings were kept, or hand-written) can't
// be re-simplified; re-teach it instead. Re-simplifying always starts from the
// raw events, never the already-folded steps, so nothing is lost to an earlier
// pass (e.g. repeated letters that a loop-fold would otherwise have swallowed).
func (d Deps) SimplifyRoute(name string) error {
	rt, ok := d.Reg.Resolve(d.activeApp(), name)
	if !ok {
		return fmt.Errorf("I don't know %q", name)
	}
	data, ok := d.Reg.LoadRecording(rt)
	if !ok {
		return fmt.Errorf("I don't have a recording for %q, so I can't re-simplify it — re-teach it with: marco teach %q", name, name)
	}
	var events []recorder.RecordedEvent
	if err := json.Unmarshal(data, &events); err != nil {
		return fmt.Errorf("the recording for %q is unreadable: %w", name, err)
	}
	// Canonical name → the same file resolves and overwrites, even if the caller
	// passed a loose phrasing that matched.
	canon := strings.ReplaceAll(rt.Slug, "-", " ")
	opts := d.teachOptions()
	opts.TypingRhythmMs = simplify.AggressiveTypingRhythmMs
	steps := simplify.Simplify(events, opts)
	if len(steps) == 0 {
		fmt.Fprintln(d.Out, "Nothing left after simplifying — leaving it as is.")
		return nil
	}
	app := firstActivateApp(steps)
	if app == "" {
		app = rt.App
	}
	src, assets, err := codegen.Route(canon, app, steps, d.Reg.ScopeDir(rt.App))
	if err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "Simplified %q to %d steps:\n--- route ---\n%s\n", canon, len(steps), src)
	if !d.confirm("Save the simplified version? [y]es / [n]o: ") {
		fmt.Fprintln(d.Out, "Left unchanged.")
		return nil
	}
	if err := d.Reg.Save(rt.App, canon, src); err != nil {
		return err
	}
	if err := d.Reg.WriteAssets(assets); err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "Simplified %q → %s\n", canon, d.Reg.Path(rt))
	return nil
}

// firstActivateApp returns the app of the route's leading Activate step (the app
// the recorder saw in front when the demonstration began), or "" if the steps
// don't start by activating an app.
func firstActivateApp(steps []macroir.Step) string {
	if len(steps) > 0 && steps[0].Kind == macroir.StepActivate {
		return steps[0].Text
	}
	return ""
}

// waitForStop blocks until the stop gesture is seen on the recorder's event
// stream, or the stream closes.
func waitForStop(rec recorder.Recorder, stop *recorder.StopKey) {
	ch := rec.Events()
	if ch == nil {
		return
	}
	for ev := range ch {
		if stop.Triggered(ev) {
			return
		}
	}
}

// stripTrailingStop removes the stop-gesture keypresses (and any trailing moves)
// the stop leaves at the end of the recording.
func stripTrailingStop(events []recorder.RecordedEvent, stop recorder.StopKey) []recorder.RecordedEvent {
	end := len(events)
	for end > 0 {
		e := events[end-1]
		isStop := e.Kind == recorder.EvKey && stop.Has(e.KeyName)
		isMove := e.Kind == recorder.EvMove
		if isStop || isMove {
			end--
			continue
		}
		break
	}
	return events[:end]
}

// teachAction is the user's choice at the save prompt.
type teachAction int

const (
	actionSave teachAction = iota
	actionDiscard
	actionSimplify
)

// chooseSave presents the save menu and returns the chosen action. The
// "simplify further" option is offered only when canSimplify is true (it's a
// one-shot). It re-prompts on unrecognized input; an empty line (or no input)
// means save.
func (d Deps) chooseSave(name string, canSimplify bool) teachAction {
	for {
		if canSimplify {
			fmt.Fprintf(d.Out, "Save this as %q? [y]es / [n]o / [s]implify: ", name)
		} else {
			fmt.Fprintf(d.Out, "Save this as %q? [y]es / [n]o: ", name)
		}
		if d.In == nil {
			return actionSave
		}
		switch strings.TrimSpace(strings.ToLower(readLine(d.In))) {
		case "", "y", "yes":
			return actionSave
		case "n", "no":
			return actionDiscard
		case "s", "simplify", "simplify further":
			if canSimplify {
				return actionSimplify
			}
		}
		if canSimplify {
			fmt.Fprintln(d.Out, "Please answer y (save), n (discard), or s (simplify).")
		} else {
			fmt.Fprintln(d.Out, "Please answer y (save) or n (discard).")
		}
	}
}

// confirm prints a prompt and reports whether the reply is affirmative
// (empty/"y"/"yes"). It reads one line without buffering ahead, so it can share
// In (e.g. os.Stdin) with an interactive caller without stealing its input.
func (d Deps) confirm(prompt string) bool {
	fmt.Fprint(d.Out, prompt)
	if d.In == nil {
		return false
	}
	line := strings.TrimSpace(strings.ToLower(readLine(d.In)))
	return line == "" || line == "y" || line == "yes"
}

// readLine reads up to and including a newline from r, one byte at a time (no
// read-ahead), and returns the line without the trailing CR/LF.
func readLine(r io.Reader) string {
	var b []byte
	var one [1]byte
	for {
		n, err := r.Read(one[:])
		if n > 0 {
			if one[0] == '\n' {
				break
			}
			if one[0] != '\r' {
				b = append(b, one[0])
			}
		}
		if err != nil {
			break
		}
	}
	return string(b)
}
