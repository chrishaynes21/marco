// Package orchestrator implements the named-route teach loop: run a route by
// name if it exists, otherwise ask the user to demonstrate it, record the
// demonstration, simplify it, generate a Marco route, save it, and run it — so
// next time the same name just runs. OS-agnostic; the OS-specific work is the
// injected recorder and host.
package orchestrator

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
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
	// With CV/anchors OFF there is no wait-gate to absorb menu-transition time, so the ROUTE
	// must carry the real pauses: preserve the recorded timings faithfully instead of capping
	// every wait to 50ms (the cap only made sense when the CV poll did the real waiting).
	if cvOff() {
		o.MaxWaitMs = 24 * 60 * 60 * 1000 // effectively no cap — keep the exact recorded gaps
	}
	return o
}

// cvOff reports whether computer vision / anchors are OFF — the DEFAULT now (the anchor stack
// isn't reliable yet, so CV is a feature flag). Off unless explicitly enabled with $MARCO_CV=
// on/max (or $MARCO_ANCHORS=1/on); when off, teach preserves the recorded timings faithfully.
// Kept consistent with recorder.anchorsEnabled and oshost.cvOff.
func cvOff() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_CV"))) {
	case "max", "1", "on", "all", "true", "yes":
		return false // CV explicitly on
	case "0", "off", "false", "no", "none":
		return true // CV explicitly off
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_ANCHORS"))) {
	case "1", "on", "true", "yes":
		return false // anchors explicitly on
	}
	return true // default: CV off
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

// anchorKeyHint is the recording prompt's note about the anchor key (or "" when
// off): tap it, then click a moving target (a menu item) to match that click by
// image at run time, with the recorded coordinate as fallback.
func anchorKeyHint() string {
	if k := recorder.AnchorKey(); k != "" {
		return fmt.Sprintf(" (tap %s then click a moving target to match it by image)", strings.ToUpper(k))
	}
	return ""
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
	mlog.Debug("do: resolving", "name", name, "app", app)
	if rt, ok := d.Reg.Resolve(app, name); ok {
		mlog.Info("do: running", "route", rt.Slug, "scope", rt.App)
		return d.Run(rt)
	}
	mlog.Info("do: unknown — offering teach", "name", name)
	fmt.Fprintf(d.Out, "I don't know %q yet.\n", name)
	// Confirm before recording — a typo or misheard phrase shouldn't drop straight
	// into a demonstration. [n] (or no stdin) just stops here.
	if !d.confirm("Teach it now? [y]es / [n]o: ") {
		mlog.Info("do: teach declined", "name", name)
		return nil
	}
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
	fmt.Fprintf(d.Out, "(For a password, type {{name}} and set it with: marco secret set <name>.%s)%s\n", d.argKeyHint(argNames), anchorKeyHint())
	mlog.Debug("teach: starting recorder", "route", name, "stop_key", stop.Label())
	if err := d.Rec.Start(); err != nil {
		return fmt.Errorf("can't record on this platform: %w", err)
	}
	waitForStop(d.Rec, &stop)
	events := stripTrailingStop(d.Rec.Stop(), stop)
	mlog.Debug("teach: recording done", "route", name, "events", len(events))

	// The faithful recording is the default; "simplify further" re-runs the whole
	// pipeline from the raw events with typing-rhythm folding on — re-running from
	// events (not the folded steps) lets typing coalesce into Type steps before
	// loop-folding ever sees repeated letters.
	opts := d.teachOptions()
	opts.ArgNames = argNames
	steps := simplify.Simplify(events, opts)
	d.enrichAnchors(steps) // OCR each demonstrated anchor's button → text locator
	mlog.Info("teach: simplified", "route", name, "events", len(events), "steps", len(steps))
	if len(steps) == 0 {
		mlog.Warn("teach: nothing recorded", "route", name)
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
		d.enrichAnchors(steps) // re-simplify rebuilds steps from events → re-label anchors
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
	return d.saveTaught(name, argNames, steps, app, events)
}

// scopeMode is where a taught route is available and whether it switches windows.
type scopeMode int

const (
	scopeContext scopeMode = iota // only when <app> is in front; no window switch
	scopeFocus                    // anywhere; brings <app> to the front first
	scopeGlobal                   // anywhere; no window switch (app-agnostic actions)
)

// chooseScope asks where the route should work. Context (only here) is the default —
// the common case and the one that can't surprise you by switching windows. Focus
// makes it global and brings the app forward first; Global makes it work in place
// (typing, Ctrl+A, …). With no app context only Global makes sense, so it's returned
// without asking.
func (d Deps) chooseScope(app string) scopeMode {
	if app == "" {
		return scopeGlobal
	}
	for {
		fmt.Fprintf(d.Out, "Where? [c] only in %s / [f] in %s from anywhere / [g] anywhere: ", app, app)
		if d.In == nil {
			return scopeContext
		}
		switch strings.TrimSpace(strings.ToLower(readLine(d.In))) {
		case "", "c", "context", "here", "only here":
			return scopeContext
		case "f", "focus", "switch":
			return scopeFocus
		case "g", "global", "anywhere", "everywhere":
			return scopeGlobal
		}
		fmt.Fprintln(d.Out, "Please answer c (only here), f (anywhere + focus), or g (anywhere).")
	}
}

// saveTaught writes a freshly taught route under the chosen scope. Only the focus
// mode keeps the leading Activate (window switch); context and global strip it.
// events (the raw demonstration) is persisted beside the route when non-nil so it
// can be re-simplified later. Shared by demonstration and narration teach.
func (d Deps) saveTaught(name string, argNames []string, steps []macroir.Step, app string, events []recorder.RecordedEvent) error {
	mode := d.chooseScope(app)
	rt := routes.Route{Slug: routes.Slug(name)}
	codegenApp := "" // only focus prepends Activate(app) so the run switches first
	switch mode {
	case scopeContext:
		rt.App = app // routes/<app>/context — foreground-only, no switch
		steps = stripLeadingActivate(steps)
	case scopeFocus:
		rt.App, rt.Focus = app, true // routes/<app>/focus — switch to the app first
		codegenApp = app
	default: // scopeGlobal: routes/global — app-less, no switch
		steps = stripLeadingActivate(steps)
	}
	finalSrc, assets, err := codegen.Route(name, codegenApp, steps, d.Reg.ScopeDir(rt), argNames...)
	if err != nil {
		return err
	}
	mlog.Info("teach: saving", "route", name, "mode", mode, "path", d.Reg.Path(rt))
	if err := d.Reg.Save(rt, finalSrc); err != nil {
		mlog.Error("teach: save failed", "route", name, "err", err)
		return err
	}
	if err := d.Reg.WriteAssets(assets); err != nil {
		mlog.Error("teach: asset write failed", "route", name, "err", err)
		return err
	}
	// Keep the raw demonstration beside the route so it can be re-simplified later
	// (marco simplify). Best-effort: a failure here doesn't fail the teach.
	if events != nil {
		if data, mErr := json.Marshal(events); mErr == nil {
			_ = d.Reg.SaveRecording(rt, data)
		}
	}
	mlog.Info("teach: saved", "route", name, "steps", len(steps), "path", d.Reg.Path(rt))
	fmt.Fprintf(d.Out, "Learned %q (%s) → %s\n", name, scopeLabel(mode, app), d.Reg.Path(rt))
	return nil
}

// scopeLabel describes a saved route's reach for the "Learned …" line.
func scopeLabel(mode scopeMode, app string) string {
	switch mode {
	case scopeContext:
		return "only in " + app
	case scopeFocus:
		return "in " + app + " from anywhere"
	default:
		return "everywhere"
	}
}

// stripLeadingActivate drops a route's opening Activate (and the window-settle wait
// after it), so a context/global route doesn't switch windows when it runs.
func stripLeadingActivate(steps []macroir.Step) []macroir.Step {
	if len(steps) > 0 && steps[0].Kind == macroir.StepActivate {
		steps = steps[1:]
		if len(steps) > 0 && steps[0].Kind == macroir.StepWait {
			steps = steps[1:]
		}
	}
	return steps
}

// TeachAuto records a demonstration and saves it with NO interactive prompts —
// it simplifies with defaults and scopes the route to the app it was taught in
// (global only if no app context). This is what the overlay uses so teaching is
// driven entirely from the HUD (record → F12 → saved) without a console window.
func (d Deps) TeachAuto(name string) error {
	name, argNames := routes.SplitArgs(name)
	stop := recorder.ParseStopKey(d.StopKey)
	mlog.Debug("teach-auto: starting recorder", "route", name, "stop_key", stop.Label())
	fmt.Fprintf(d.Out, "Recording %q — do it now, press %s when done.%s%s\n", name, stop.Label(), d.argKeyHint(argNames), anchorKeyHint())
	if err := d.Rec.Start(); err != nil {
		return fmt.Errorf("can't record on this platform: %w", err)
	}
	waitForStop(d.Rec, &stop)
	events := stripTrailingStop(d.Rec.Stop(), stop)
	mlog.Debug("teach-auto: recording done", "route", name, "events", len(events))

	opts := d.teachOptions()
	opts.ArgNames = argNames
	steps := simplify.Simplify(events, opts)
	d.enrichAnchors(steps) // OCR each demonstrated anchor's button → text locator
	mlog.Info("teach-auto: simplified", "route", name, "events", len(events), "steps", len(steps))
	if len(steps) == 0 {
		mlog.Warn("teach-auto: nothing recorded", "route", name)
		fmt.Fprintln(d.Out, "Nothing was recorded — not saving.")
		return nil
	}
	app := firstActivateApp(steps)
	if app == "" {
		app = d.activeApp()
	}
	// Auto-teach defaults to a CONTEXT route (foreground-only) for the app it was taught
	// in — global if there's no app. Neither switches windows, so drop a leading Activate.
	rt := routes.Route{App: app, Slug: routes.Slug(name)} // Focus=false → context
	steps = stripLeadingActivate(steps)
	finalSrc, assets, err := codegen.Route(name, "", steps, d.Reg.ScopeDir(rt), argNames...)
	if err != nil {
		return err
	}
	mlog.Info("teach-auto: saving", "route", name, "app", app, "path", d.Reg.Path(rt))
	if err := d.Reg.Save(rt, finalSrc); err != nil {
		mlog.Error("teach-auto: save failed", "route", name, "err", err)
		return err
	}
	if err := d.Reg.WriteAssets(assets); err != nil {
		return err
	}
	if data, mErr := json.Marshal(events); mErr == nil {
		_ = d.Reg.SaveRecording(rt, data)
	}
	where := "everywhere"
	if app != "" {
		where = "in " + app
	}
	mlog.Info("teach-auto: saved", "route", name, "app", app, "steps", len(steps))
	fmt.Fprintf(d.Out, "Learned %q (%s), %d steps\n", name, where, len(steps))
	return nil
}

// TeachVoice builds a route by narration instead of demonstration: each phrase from
// the phrases channel is parsed and applied to a voiceteach.Session reading the live
// cursor/screen through env (status, if non-nil, gets a short line per step for a
// HUD). It saves on "done" (scoped to the app, like TeachAuto), discards on "cancel".
func (d Deps) TeachVoice(name string, env voiceteach.Env, phrases <-chan string, status func(string)) error {
	name, argNames := routes.SplitArgs(name)
	mlog.Info("narrate: session started", "route", name, "args", len(argNames))
	sess := voiceteach.New(env, argNames...)
	fmt.Fprintf(d.Out, "Narrate %q — say what to do (\"click this\", \"type …\", \"wait for this screen\"), then \"done\".\n", name)
	for phrase := range phrases {
		cmd := voiceteach.Parse(phrase)
		mlog.Debug("narrate: phrase", "route", name, "phrase", phrase, "kind", cmd.Kind)
		st, done, canceled := sess.Apply(cmd)
		fmt.Fprintf(d.Out, "  %s\n", st)
		if status != nil {
			status(st)
		}
		if canceled {
			mlog.Info("narrate: canceled", "route", name)
			fmt.Fprintln(d.Out, "Canceled — nothing saved.")
			return nil
		}
		if done {
			mlog.Debug("narrate: done received", "route", name)
			break
		}
	}
	steps := sess.Steps()
	if len(steps) == 0 {
		mlog.Warn("narrate: nothing recorded", "route", name)
		fmt.Fprintln(d.Out, "Nothing narrated — not saving.")
		return nil
	}
	app := firstActivateApp(steps)
	if app == "" {
		app = d.activeApp()
	}
	// Same save / scope prompts as demonstration teach (no simplify — narration has
	// no raw event stream to re-fold, and no recording is persisted).
	preview, _, err := codegen.Route(name, app, steps, "", argNames...)
	if err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "Narrated %d steps:\n--- route ---\n%s\n", len(steps), preview)
	if d.chooseSave(name, false) == actionDiscard {
		fmt.Fprintln(d.Out, "Discarded.")
		return nil
	}
	return d.saveTaught(name, argNames, steps, app, nil)
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
	d.enrichAnchors(steps) // re-derive text anchors lost when rebuilding from raw events
	if len(steps) == 0 {
		fmt.Fprintln(d.Out, "Nothing left after simplifying — leaving it as is.")
		return nil
	}
	// Preserve the route's scope. A focus route keeps switching to its app (codegen
	// prepends Activate); context/global don't, so strip any re-detected leading Activate.
	codegenApp := ""
	if rt.Focus {
		codegenApp = rt.App
	} else {
		steps = stripLeadingActivate(steps)
	}
	src, assets, err := codegen.Route(canon, codegenApp, steps, d.Reg.ScopeDir(rt))
	if err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "Simplified %q to %d steps:\n--- route ---\n%s\n", canon, len(steps), src)
	// The overlay can't show the preview to confirm against, so simplify there is
	// instant — SimplifySaves means "simplify and save" with no prompt.
	if !d.SimplifySaves && !d.confirm("Save the simplified version? [y]es / [n]o: ") {
		fmt.Fprintln(d.Out, "Left unchanged.")
		return nil
	}
	if err := d.Reg.Save(rt, src); err != nil {
		return err
	}
	if err := d.Reg.WriteAssets(assets); err != nil {
		return err
	}
	fmt.Fprintf(d.Out, "Simplified %q → %s\n", canon, d.Reg.Path(rt))
	return nil
}

// enrichAnchors labels each DEMONSTRATED anchor — a click the recorder captured a button
// template for — with what the available resolver hosts can recognise: the on-screen TEXT
// (the Text host's Read) and/or the element CLASS (the Vision host's Identify) under the
// click. Each recognised signal becomes a run-time `Find`/`Locate` fallback, so a moved
// control (text), or one with no text and no clean edge — an icon on a gradient (vision) —
// still resolves. This is the SINGLE teaching path: demonstrate once, the anchor gains
// whatever resolvers are wired. Best-effort and graceful — a host that's absent (no
// $MARCO_OCR / $MARCO_VISION) or can't read leaves that signal off and the anchor keeps
// whatever it has (image/colour/edge). Mutates steps in place, recursing loop bodies.
func (d Deps) enrichAnchors(steps []macroir.Step) {
	text, vision := d.Hosts["Text"], d.Hosts["Vision"]
	if text == nil && vision == nil {
		return
	}
	d.labelAnchors(text, vision, steps)
}

// labelAnchors walks steps (and nested loop bodies), labelling each anchored click via the
// wired hosts — OCR text into AnchorText, the detected element class into AnchorVision —
// skipping a signal already present (e.g. AnchorText from an earlier pass).
func (d Deps) labelAnchors(text, vision runtime.Host, steps []macroir.Step) {
	for i := range steps {
		s := &steps[i]
		if len(s.Steps) > 0 {
			d.labelAnchors(text, vision, s.Steps)
		}
		if s.Kind != macroir.StepClick || len(s.Template) == 0 {
			continue
		}
		if text != nil && s.AnchorText == "" {
			if t := readAnchorText(text, s.Template, s.AnchorClickX, s.AnchorClickY); t != "" {
				s.AnchorText = t
				mlog.Info("teach: labelled anchor by OCR", "text", t)
			}
		}
		if vision != nil && s.AnchorVision == "" {
			if label, box, ok := readAnchorVision(vision, s.Template, s.AnchorClickX, s.AnchorClickY); ok {
				s.AnchorVision = label
				mlog.Info("teach: labelled anchor by vision", "label", label)
				// Re-crop the template to the DETECTED button box — a pixel-tight anchor the
				// geometric crop can't produce (it fuses stacked menu buttons). Keeps the
				// recorded screen click; only the template image and the click-within-template
				// move. Best-effort: a bad/oversized box leaves the template untouched.
				if cropped, ncx, ncy, ok := cropTemplate(s.Template, box, s.AnchorClickX, s.AnchorClickY); ok {
					s.Template = cropped
					s.AnchorClickX, s.AnchorClickY = ncx, ncy
					mlog.Info("teach: re-cropped anchor to detected button", "box", fmt.Sprintf("%dx%d", box[2], box[3]))
				}
			}
		}
	}
}

// readAnchorText OCRs one template via the Text host's Read action, returning the
// recognised label or "" on any miss (no text on the button, host failed/declined, or
// the OCR engine is absent). clickX,clickY locate the click WITHIN the template so Read
// labels the button under the click, not the whole crop; (0,0) lets Read fall back to
// the template centre. The bridge never errors the Go call — a failure arrives as a
// non-"ok" status — so a missing tesseract degrades to "no text anchor", never a failed
// teach.
func readAnchorText(host runtime.Host, template []byte, clickX, clickY int) string {
	in := runtime.NewSet()
	in.Put("Image", runtime.Text(base64.StdEncoding.EncodeToString(template)))
	in.Put("ClickX", runtime.Number(float64(clickX)))
	in.Put("ClickY", runtime.Number(float64(clickY)))
	status, data, err := host.Invoke(runtime.HostCall{
		Act:    "Text",
		Action: "Read",
		Input:  runtime.SetVal(in),
		Out:    io.Discard,
		Ctx:    context.Background(),
	})
	if err != nil || status != "ok" || !data.IsSet() {
		return ""
	}
	if v, ok := data.AsSet().Get("Text"); ok {
		return strings.TrimSpace(v.AsText())
	}
	return ""
}

// readAnchorVision identifies the clicked control via the Vision host's Identify action,
// returning its CLASS label ("button", "icon", …) and its BOX (X,Y,W,H, image-local) under
// the click. ok=false on any miss (no element there, host failed/declined, or no model).
// Same graceful contract as readAnchorText: the bridge never errors the Go call, so a
// missing model degrades to "no vision anchor".
func readAnchorVision(host runtime.Host, template []byte, clickX, clickY int) (label string, box [4]int, ok bool) {
	in := runtime.NewSet()
	in.Put("Image", runtime.Text(base64.StdEncoding.EncodeToString(template)))
	in.Put("ClickX", runtime.Number(float64(clickX)))
	in.Put("ClickY", runtime.Number(float64(clickY)))
	status, data, err := host.Invoke(runtime.HostCall{
		Act:    "Vision",
		Action: "Identify",
		Input:  runtime.SetVal(in),
		Out:    io.Discard,
		Ctx:    context.Background(),
	})
	if err != nil || status != "ok" || !data.IsSet() {
		return "", box, false
	}
	set := data.AsSet()
	if v, k := set.Get("Label"); k {
		label = strings.TrimSpace(v.AsText())
	}
	if label == "" {
		return "", box, false
	}
	box = [4]int{setNum(set, "X"), setNum(set, "Y"), setNum(set, "W"), setNum(set, "H")}
	return label, box, true
}

// setNum reads an integer field from a set (numbers arrive as float64), 0 if absent.
func setNum(set *runtime.Set, name string) int {
	if v, ok := set.Get(name); ok {
		if n, ok := v.AsNumber(); ok {
			return int(n)
		}
	}
	return 0
}

// cropTemplate re-crops a template PNG to box (X,Y,W,H, in the template's own pixels) — the
// vision-detected control — returning the new PNG and the click position WITHIN it. ok=false
// when the template isn't decodable, the box is degenerate, or it barely shrinks the
// template (no isolation gained), so the original is kept.
func cropTemplate(template []byte, box [4]int, clickX, clickY int) (out []byte, ncx, ncy int, ok bool) {
	img, err := png.Decode(bytes.NewReader(template))
	if err != nil {
		return nil, 0, 0, false
	}
	b := img.Bounds()
	r := image.Rect(box[0], box[1], box[0]+box[2], box[1]+box[3]).Add(b.Min).Intersect(b)
	if r.Dx() < minCropPx || r.Dy() < minCropPx {
		return nil, 0, 0, false
	}
	// Only worth it if the box actually isolates something smaller than the template.
	if r.Dx() >= b.Dx()-2 && r.Dy() >= b.Dy()-2 {
		return nil, 0, 0, false
	}
	sub := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(sub, sub.Bounds(), img, r.Min, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, sub); err != nil {
		return nil, 0, 0, false
	}
	return buf.Bytes(), clickX - (r.Min.X - b.Min.X), clickY - (r.Min.Y - b.Min.Y), true
}

// minCropPx is the smallest side a vision-cropped template may have — below it the detection
// is too tight to be a real control.
const minCropPx = 10

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
