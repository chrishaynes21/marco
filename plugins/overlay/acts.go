package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The overlay is a bidirectional bridge host (see ../../spec/Hosts.md): it reads
// Overlay act requests on stdin, writes responses on stdout, and pushes feed
// events (Commands/Hotkeys) on the same stdout. A single writer goroutine
// serializes everything written back to the engine.

type request struct {
	Act    string `json:"act"`
	Action string `json:"action"`
	Input  any    `json:"input"`
}

type response struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
	Error  string `json:"error,omitempty"`
}

type event struct {
	Feed  string `json:"feed"`
	Event string `json:"event"`
	Data  any    `json:"data,omitempty"`
}

func okData(d any) response { return response{Status: "ok", Data: d} }
func fail(msg string) response {
	return response{Status: "failed", Error: msg}
}

// writeLoop encodes every response/event to stdout, one JSON object per line.
func writeLoop(out <-chan any) {
	enc := json.NewEncoder(os.Stdout)
	for v := range out {
		if err := enc.Encode(v); err != nil {
			return // engine gone
		}
	}
}

// readRequests pumps Overlay act calls from the engine and dispatches them. When
// stdin closes (engine exited) it asks the window to quit so the process ends.
func readRequests(h *model, out chan<- any) {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if json.Unmarshal(line, &req) != nil {
			continue
		}
		out <- dispatch(h, req)
	}
	h.requestQuit()
}

// dispatch fulfils one Overlay act call.
func dispatch(h *model, req request) response {
	mlogD("overlay: dispatch", "action", req.Action)
	switch req.Action {
	case "Show":
		h.show(true)
		bootHintOnce.Do(func() { // one-time nudge toward the web control center
			h.log("tip: type  ui  for the visual editor — routes · bindings · config · help")
		})
		return okData(nil)
	case "Hide":
		h.show(false)
		return okData(nil)
	case "Status":
		h.setStatus(asText(req.Input))
		return okData(nil)
	case "Log":
		h.log(asText(req.Input))
		return okData(nil)
	case "Clear":
		h.clear()
		return okData(nil)
	case "SetInput":
		h.setInput(asText(req.Input))
		return okData(nil)
	case "Heard":
		if voiceEnabled.Load() {
			h.setHeard(asText(req.Input))
		} else {
			h.setHeard("") // muted: don't show live mic text
		}
		return okData(nil)
	case "ListRoutes":
		names := listRoutes()
		h.setRoutes(names)
		return okData(names)
	case "Run", "RunVoice":
		// A finished spoken phrase arrives as RunVoice; drop it while voice is toggled off.
		if req.Action == "RunVoice" && !voiceEnabled.Load() {
			return okData(nil)
		}
		name := asText(req.Input)
		if name == "" {
			return fail("empty command")
		}
		fields := strings.Fields(name)
		// While a voice-teach session is live, every phrase IS narration — forward it
		// to the teach child (it handles "done"/"cancel" and exits) instead of running.
		if voiceActive.Load() {
			mlogD("overlay: forwarding to voice-teach", "phrase", name)
			writeVoicePhrase(name)
			return okData(nil)
		}
		// Voice-listening toggle (the overlay's mic config): "voice on|off|toggle", or the
		// natural aliases "mute"/"unmute" and "stop listening"/"listen". setVoice mirrors it to
		// the settings panel and persists it. Typed commands are never gated, so you can always
		// type `m voice on (or `m listen) to un-mute even after muting by voice.
		if on, matched := voiceToggleCmd(fields); matched {
			setVoice(on)
			if on {
				h.setStatus("voice on")
				h.log("voice on — say a command")
			} else {
				h.setStatus("voice off")
				h.log("voice off — type `m voice on to un-mute")
				h.setHeard("")
			}
			return okData(nil)
		}
		// "narrate teach <name>" (alias "voice teach") starts narration-driven
		// teaching: subsequent typed or spoken phrases ("click this", "type …", "wait
		// for this screen", "done") build the route. Same path whether you type each
		// phrase or speak it.
		if len(fields) >= 3 && (fields[0] == "narrate" || fields[0] == "voice") && fields[1] == "teach" {
			vname := strings.TrimSpace(strings.Join(fields[2:], " "))
			mlogI("overlay: narrate-teach starting", "name", vname)
			voiceActive.Store(true) // set before spawning so no early phrase leaks to Run
			h.setTeaching(true)
			h.log("narrate teach \"" + vname + "\" — type or say: click / anchor this / type … / wait for this screen / done")
			go func() {
				err := runNarrateTeach(h, vname)
				h.setTeaching(false)
				if err != nil {
					mlogE("overlay: narrate-teach failed", "name", vname, "err", err)
					h.log("narrate teach failed: " + vname)
				} else {
					mlogI("overlay: narrate-teach done", "name", vname)
				}
			}()
			return okData(nil)
		}
		// "exit" / "quit" closes the overlay (the window quit also stops serve).
		if len(fields) == 1 && (fields[0] == "exit" || fields[0] == "quit") {
			mlogI("overlay: quit requested")
			h.setStatus("bye")
			h.requestQuit()
			return okData(nil)
		}
		// "teach <name>" records a new route IN-PLACE (no console window): the SAME
		// interactive teach as the CLI. Demonstrate, press the leader to finish; the engine
		// streams its save / scope / simplify prompts into the HUD and the controller
		// answers them with a single y/n/s keypress (see runTeachInteractive).
		if len(fields) >= 1 && fields[0] == "teach" {
			tname := strings.TrimSpace(strings.Join(fields[1:], " "))
			if tname == "" {
				h.setStatus("teach needs a name — `m teach <name>")
				return fail("teach needs a name")
			}
			startTeach(h, tname)
			return okData(nil)
		}
		// "ui" (or bare "edit") opens the control center on the all-routes browser; "edit
		// <name>" opens it on that route's editor. It runs its own local web server, so launch
		// it detached rather than streaming it like a route — the HUD just notes it opened.
		if fields[0] == "ui" || fields[0] == "edit" {
			ename := strings.TrimSpace(strings.Join(fields[1:], " ")) // "" for the browser
			startEdit(h, ename)
			return okData(nil)
		}
		// A FINAL SPOKEN PHRASE goes to the Director, not to route lookup.
		//
		// This is the one line that connects voice to semantic desktop control. It sits
		// here — after the overlay's own vocabulary (voice on/off, teach, ui, exit),
		// which is the overlay configuring ITSELF and never a desktop intent — and
		// before route lookup, which becomes the fallback rather than the default.
		//
		// Typed commands are untouched. Someone typing `m my route` is naming a route
		// they taught; someone speaking is describing what they want to happen.
		//
		// Dispatch returns immediately. A spoken "stop" during a long command has to
		// reach this handler while that command is still running, and it cannot if the
		// handler is blocked waiting for it.
		if req.Action == "RunVoice" {
			mlogI("overlay: voice to director", "phrase", name)
			h.setHeard(name)
			dispatchVoice(h, name)
			return okData(nil)
		}
		// Verb commands; anything else is a route to run.
		args := []string{"do", name}
		switch {
		case len(fields) > 0 && (fields[0] == "bind" || fields[0] == "unbind"):
			args = fields
		case len(fields) >= 2 && fields[0] == "press":
			args = fields // press a key or chord (e.g. "press enter", "press control c")
		case len(fields) >= 2 && fields[0] == "forget":
			args = append([]string{"forget"}, fields[1:]...) // delete a route
		case len(fields) >= 4 && fields[0] == "rename":
			// "rename old name to new name" — split on " to " and pass as separate args.
			rest := strings.TrimSpace(strings.TrimPrefix(name, "rename "))
			if idx := strings.Index(rest, " to "); idx > 0 {
				args = []string{"rename", rest[:idx], "to", strings.TrimSpace(rest[idx+4:])}
			}
		case len(fields) >= 2 && fields[0] == "simplify":
			args = fields // re-simplify a route from its recording
		}
		mlogI("overlay: running", "name", name, "args", args)
		h.setStatus("running: " + name)
		_, outcome, route := runRecord(h, name, args...)
		if outcome == "failed" {
			// A run that never announced a [route] didn't resolve — it's an unknown
			// command, so offer to teach it in-HUD rather than just erroring. (Other
			// verbs like forget/rename aren't routes, so only the plain `do` path.)
			if args[0] == "do" && route == "" {
				mlogI("overlay: unknown command — offering teach", "name", name)
				offerTeach(h, name)
				return okData(nil)
			}
			mlogW2("overlay: command failed", "name", name)
			return fail("command failed: " + name)
		}
		return okData(nil)
	case "Hotkey":
		key := asText(req.Input)
		if key == "" {
			return okData(nil)
		}
		mlogD("overlay: hotkey", "key", key)
		if err, _, _ := runMarco(h, "hotkey", key); err != nil {
			mlogE("overlay: hotkey failed", "key", key, "err", err)
			return fail(err.Error())
		}
		return okData(nil)
	case "Active":
		return okData(activeApp())
	default:
		mlogW2("overlay: unknown action", "action", req.Action)
		return fail("unknown Overlay action: " + req.Action)
	}
}

// asText coerces a decoded JSON Value to text (Marco passes plain strings).
func asText(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case map[string]any:
		if s, ok := t["Text"].(string); ok {
			return s
		}
	}
	return ""
}

// ---- CLI seam: the same three commands web-ui uses ----

func marcoBin() string {
	if b := os.Getenv("MARCO_BIN"); b != "" {
		return b
	}
	return "marco"
}

// listRoutes runs `marco routes --json` and returns the route display names.
// routeInfo is one route with its scope. Scope is "context" (only in App), "focus"
// (App, from anywhere — switches to it), or "global" (app-less, anywhere).
type routeInfo struct {
	Name  string `json:"name"`
	App   string `json:"app"`
	Scope string `json:"scope"`
}

// listRoutesFull returns every route with its scope (name + app).
func listRoutesFull() []routeInfo {
	out, err := exec.Command(marcoBin(), "routes", "--json").Output()
	if err != nil {
		return nil
	}
	var rows []routeInfo
	if json.Unmarshal(out, &rows) != nil {
		return nil
	}
	return rows
}

// listRoutes returns just the route names (for autocomplete and the route ticker).
func listRoutes() []string {
	rows := listRoutesFull()
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, r.Name)
	}
	return names
}

// editState tracks the detached `marco edit` server so opening a new editor replaces the prior
// one (each edit server holds a port until it's killed).
var (
	editMu  sync.Mutex
	editCmd *exec.Cmd
)

// bootHintOnce logs the "open the visual editor" tip a single time, on the first Show.
var bootHintOnce sync.Once

// startEdit launches `marco edit <name>` detached — it serves the editor page and opens the
// browser itself, so the HUD just notes it rather than streaming it like a route. Any prior
// editor server is killed first so ports don't pile up.
func startEdit(h *model, name string) {
	// No name → open the control center on the all-routes browser (marco ui); a name opens it
	// on that route's editor (marco edit <name>).
	if name != "" {
		launchUI(h, []string{"edit", name}, `editing "`+name+`"`)
		return
	}
	launchUI(h, []string{"ui"}, "control center")
}

// startUI opens the control center on a specific tab (help/routes/bindings/config), e.g. the
// help command opens the Help view in the browser instead of an in-HUD panel.
func startUI(h *model, view string) {
	args, note := []string{"ui"}, "control center"
	if view != "" {
		args, note = append(args, view), view
	}
	launchUI(h, args, note)
}

// launchUI spawns the control center detached, replacing any prior instance so ports don't stack.
func launchUI(h *model, args []string, note string) {
	editMu.Lock()
	if editCmd != nil && editCmd.Process != nil {
		_ = editCmd.Process.Kill()
	}
	cmd := exec.Command(marcoBin(), args...)
	if err := cmd.Start(); err != nil {
		editMu.Unlock()
		mlogE("overlay: ui failed to start", "err", err)
		h.log("ui failed to open")
		return
	}
	editCmd = cmd
	editMu.Unlock()
	mlogI("overlay: ui", "args", strings.Join(args, " "))
	h.log(note + " — opened in your browser")
	go func() { _ = cmd.Wait() }() // reap when the user closes it
}

// runState tracks the in-flight CLI child so a stop key can cancel it.
var (
	runMu       sync.Mutex
	runCmd      *exec.Cmd
	runCanceled bool
)

// Interactive-prompt plumbing (named "teach*" for historical reasons, but it now
// serves ANY subcommand's prompts — teach save/scope/simplify, a `forget` delete
// confirm, a `simplify` confirm, …). The child's stdin is teachPipe; the global
// hook sets teachAsk when a prompt is up and the controller writes the y/n/s reply.
var (
	teachMu   sync.Mutex
	teachPipe io.WriteCloser // the active interactive child's stdin
	teachAsk  atomic.Bool    // a y/n/s prompt is waiting for an answer
)

// writeTeachAnswer forwards the submitted answer (e.g. "y", "n", "s", or "" for the
// bare-Enter default) plus a newline to the active child's stdin. Called from the
// controller's processor goroutine (NOT the hook thread), so the write is safe.
func writeTeachAnswer(s string) {
	teachMu.Lock()
	w := teachPipe
	teachMu.Unlock()
	if w != nil {
		_, _ = w.Write([]byte(s + "\n"))
	}
}

// Voice-teach plumbing: while voiceActive, the dispatcher pipes each phrase to the
// `marco teach --voice` child's stdin (one phrase per line) instead of running it.
var (
	voiceMu     sync.Mutex
	voicePipe   io.WriteCloser
	voiceActive atomic.Bool
)

// voiceEnabled gates whether finished spoken phrases run — the live mirror of cfg.Voice
// (settings panel + `voice on|off` command). On by default; MARCO_VOICE=off / a saved config
// starts muted. Typed commands are never gated, so you can always type `voice on` to un-mute
// even after muting by voice.
var voiceEnabled atomic.Bool

func init() { voiceEnabled.Store(cfgVoice()) }

// voiceToggleCmd recognises the mic-listening commands and returns the desired state. Beyond
// "voice on|off|toggle" it takes natural phrasings — "mute"/"unmute", "stop listening",
// "listen" — so speaking or typing "stop listening" mutes.
func voiceToggleCmd(fields []string) (on bool, matched bool) {
	switch strings.ToLower(strings.Join(fields, " ")) {
	case "voice", "voice toggle":
		return !voiceEnabled.Load(), true
	case "voice on", "unmute", "listen", "listen on", "start listening":
		return true, true
	case "voice off", "mute", "stop listening", "stop voice", "listen off", "quit listening":
		return false, true
	}
	return false, false
}

// writeVoicePhrase forwards one narration phrase to the voice-teach child's stdin.
func writeVoicePhrase(phrase string) {
	voiceMu.Lock()
	w := voicePipe
	voiceMu.Unlock()
	if w != nil {
		_, _ = w.Write([]byte(phrase + "\n"))
	}
}

// runNarrateTeach runs `marco teach --narrate <name>`, feeding it the phrases the
// dispatcher forwards (writeVoicePhrase) — typed or spoken — and streaming its
// per-step status into the HUD. The child saves on "done" / discards on "cancel"
// and exits, which ends the session here. Mirrors streamChild but with the
// narration stdin pipe.
// narrateLockPath returns the path of the lock file that signals the voice
// plugin to stay armed between narration phrases. Both the overlay and the
// voice plugin read MARCO_NARRATE_LOCK; if unset they share the same default.
func narrateLockPath() string {
	if p := os.Getenv("MARCO_NARRATE_LOCK"); p != "" {
		return p
	}
	return filepath.Join(os.TempDir(), "marco-narrate.lock")
}

func runNarrateTeach(h *model, name string) error {
	// Signal the voice plugin to re-arm between phrases for the duration of this
	// session. The file is removed in the defer below (normal exit) or simply
	// expires as stale if the overlay crashes.
	lock := narrateLockPath()
	_ = os.WriteFile(lock, nil, 0o600)
	defer os.Remove(lock)

	cmd := exec.Command(marcoBin(), "teach", "--narrate", name)
	cmd.Env = append(os.Environ(), "MARCO_NO_PANIC_STOP=1")
	inW, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	voiceMu.Lock()
	voicePipe = inW
	voiceMu.Unlock()
	defer func() {
		voiceActive.Store(false)
		voiceMu.Lock()
		voicePipe = nil
		voiceMu.Unlock()
		_ = inW.Close()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			if line := strings.TrimRight(sc.Text(), "\r"); line != "" {
				fmt.Fprintln(os.Stderr, line) // full detail to the serve console
				h.log(line)
			}
		}
	}()
	err = cmd.Run()
	pw.Close()
	<-done
	return err
}

// streamChild runs a marco subcommand, streams its stdout/stderr into the HUD,
// and makes ANY interactive prompt it raises answerable from the overlay: the
// child gets a stdin pipe (teachPipe) that the controller writes the y/n/s answer
// to, and prompts are detected on the partial (newline-less) line. Returns (err,
// canceled, route); route is the canonical name announced via "[route] <name>".
// trackCancel registers the child so Esc can kill it — on for runMarco's commands,
// off for teach (the recorder owns F12 instead).
func streamChild(h *model, trackCancel bool, args ...string) (error, bool, string) {
	cmd := exec.Command(marcoBin(), args...)
	// NO_PANIC_STOP: the overlay owns the global hook (dueling LL hooks + a route's
	// injected input deadlock). NO_TEACH: an unknown `do` errors instead of dropping
	// into teach-on-unknown. SIMPLIFY_SAVES: teach's [s] simplifies AND saves (the
	// HUD can't show the preview to re-confirm). All harmless to non-teach commands.
	cmd.Env = append(os.Environ(), "MARCO_NO_PANIC_STOP=1", "MARCO_NO_TEACH=1", "MARCO_SIMPLIFY_SAVES=1")
	if len(args) > 0 && args[0] == "teach" {
		// The leader key stops the demo. The overlay passes it through while recording so
		// it reaches the recorder's stop detection (see handleKey's recording gate).
		cmd.Env = append(cmd.Env, "MARCO_STOP_KEY="+cfgLeader())
	}
	inW, err := cmd.StdinPipe()
	if err != nil {
		return err, false, ""
	}
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	teachMu.Lock()
	teachPipe = inW
	teachMu.Unlock()
	defer func() {
		teachMu.Lock()
		teachPipe = nil
		teachMu.Unlock()
		teachAsk.Store(false)
		h.clearTeachPrompt()
		_ = inW.Close()
	}()

	route := ""
	done := make(chan struct{})
	go func() {
		defer close(done)
		rd := bufio.NewReader(pr)
		var line []byte
		flush := func() {
			s := strings.TrimRight(string(line), "\r")
			if s == "" {
				return
			}
			// The full route log (find/click/scores, errors, everything the child prints)
			// goes to the CONSOLE — the overlay.cmd window where serve runs — uncapped, so
			// the complete detail is always visible there even though the HUD only keeps a
			// short tail.
			fmt.Fprintln(os.Stderr, s)
			if r, ok := strings.CutPrefix(s, "[route] "); ok {
				route = strings.TrimSpace(r) // the resolved route name, not a log line
				return
			}
			// Unknown-`do` error: the overlay offers to teach instead (see the `do`
			// dispatch branch), so don't also log the raw engine error. Matched on the
			// do-specific hint so a failed `bind`/etc. still surfaces its own error.
			if strings.HasPrefix(s, "no route matches ") && strings.Contains(s, "marco teach") {
				return
			}
			h.log(s)
		}
		for {
			b, rerr := rd.ReadByte()
			if rerr != nil {
				flush()
				return
			}
			if b == '\n' {
				flush()
				line = line[:0]
				continue
			}
			line = append(line, b)
			// Prompts carry NO trailing newline (the engine blocks on stdin), so we
			// detect them on the partial line. A complete prompt has the bracketed
			// options "? [" AND ends with ": " (the answer cursor) — waiting for the
			// ": " captures the FULL menu, e.g. "...? [y]es / [n]o / [s]implify: ",
			// not just a truncated "? [y]", and re-arms cleanly for a re-ask.
			if !teachAsk.Load() && bytes.Contains(line, []byte("? [")) && bytes.HasSuffix(line, []byte(": ")) {
				h.setTeachPrompt(strings.TrimRight(string(line), "\r"))
				teachAsk.Store(true)
				line = line[:0]
			}
		}
	}()

	if trackCancel {
		runMu.Lock()
		runCmd, runCanceled = cmd, false
		runMu.Unlock()
	}

	err = cmd.Run()
	pw.Close()
	<-done

	canceled := false
	if trackCancel {
		runMu.Lock()
		canceled = runCanceled
		runCmd = nil
		runMu.Unlock()
	}
	return err, canceled, route
}

// runTeachInteractive runs `marco teach <name>` interactively (the recorder owns
// F12, so no Esc-cancel); its save/scope/simplify prompts are answered in the HUD.
func runTeachInteractive(h *model, name string) error {
	err, _, _ := streamChild(h, false, "teach", name)
	return err
}

// cancelRun kills the in-flight route, if any, marking it canceled (vs failed).
func cancelRun() {
	runMu.Lock()
	defer runMu.Unlock()
	if runCmd != nil && runCmd.Process != nil {
		runCanceled = true
		_ = runCmd.Process.Kill()
	}
}

// isRunning reports whether a cancelable route is currently executing, so the
// controller can make the leader key the panic/stop key while a route runs.
func isRunning() bool {
	runMu.Lock()
	defer runMu.Unlock()
	return runCmd != nil
}

// runRecord runs the command, sets the status, and appends it to the command
// history with its outcome (ok/canceled/failed) and how long it took. It also
// returns the canonical route the command resolved to ("" if none) so the caller
// can tell an unknown command from a route that failed at run time.
func runRecord(h *model, name string, args ...string) (time.Duration, string, string) {
	start := time.Now()
	err, canceled, route := runMarco(h, args...)
	d := time.Since(start)
	disp := name
	if route != "" {
		disp = route // show the canonical route a loose phrase resolved to
	}
	outcome := "ok"
	switch {
	case canceled:
		outcome = "canceled"
		h.setStatus("canceled: " + disp)
	case err != nil:
		outcome = "failed"
		h.setStatus("failed: " + disp)
	default:
		h.setStatus("ran: " + disp)
	}
	h.log(fmt.Sprintf("%s  %s  %s", outcomeIcon(outcome), disp, fmtDur(d)))
	h.addHistory(disp, outcome, d)
	return d, outcome, route
}

// recording is true while a demonstration teach child is running. The keyboard hook
// reads it to pass every key through to the recorder (and the app) — so the leader
// key reaches the recorder's stop detection and ends the demo, instead of opening
// the overlay command line.
var recording atomic.Bool

// startTeach kicks off an in-HUD demonstration teach for name (record → leader → save),
// keeping the panel awake and streaming the engine's prompts into the HUD. Shared by
// the explicit `m teach <name>` command and the unknown-command teach offer.
func startTeach(h *model, name string) {
	mlogI("overlay: teach starting", "name", name)
	h.setTeaching(true) // keep the HUD awake for the whole demonstration
	recording.Store(true)
	h.log("recording \"" + name + "\" — demonstrate now, press " + strings.ToUpper(cfgLeader()) + " to save")
	go func() {
		err := runTeachInteractive(h, name) // streams prompts; answered in-HUD
		recording.Store(false)
		h.setTeaching(false)
		if err != nil {
			mlogE("overlay: teach failed", "name", name, "err", err)
			h.log("teach failed: " + name)
		} else {
			mlogI("overlay: teach done", "name", name)
		}
	}()
}

// pendingTeach holds the route name an unknown-command teach offer would teach if
// accepted ("" = none). Set when `marco do <name>` resolves nothing; consumed by the
// controller when the user answers the in-HUD "Teach …?" prompt.
var (
	pendingTeachMu sync.Mutex
	pendingTeach   string
)

// offerTeach shows an in-HUD "Teach \"name\"? [y]es / [n]o" prompt for an unknown
// command, reusing the teach-prompt answering path (teachAsk). The controller starts
// a demonstration on yes (see actTeachSubmit) and clears it on no.
func offerTeach(h *model, name string) {
	pendingTeachMu.Lock()
	pendingTeach = name
	pendingTeachMu.Unlock()
	h.setStatus("unknown: " + name)
	h.setTeachPrompt(fmt.Sprintf("Teach %q? [y]es / [n]o: ", name))
	teachAsk.Store(true)
}

// takePendingTeach returns and clears the pending teach-offer name.
func takePendingTeach() string {
	pendingTeachMu.Lock()
	defer pendingTeachMu.Unlock()
	n := pendingTeach
	pendingTeach = ""
	return n
}

func outcomeIcon(outcome string) string {
	switch outcome {
	case "ok":
		return "ok"
	case "canceled":
		return "canceled"
	default:
		return "failed"
	}
}

func fmtDur(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// runMarco runs a marco subcommand, streaming output into the HUD and answering
// any prompt it raises (a `forget` delete confirm, a `simplify` confirm, …) from
// the overlay. Esc cancels it. Returns (err, canceled, route) — route is the
// canonical name the engine announced via a "[route] <name>" line.
func runMarco(h *model, args ...string) (error, bool, string) {
	return streamChild(h, true, args...)
}

// helpLines builds the help menu: the leader keys plus the known routes.
func helpLines() []string {
	lines := []string{
		"`m then type:",
		"  <route>            run it",
		"  teach <name>       record a new one (leader to save)",
		"  ui                 visual editor: routes · bindings · config · help",
		"  edit <route>       edit one route in the browser",
		"  simplify <route>   re-clean its steps",
		"  rename <old> to <new>  rename it",
		"  forget <route>     delete it",
		"  bind <key> <route> hotkey it for this app",
		"  voice on|off / config / help / exit",
		"`<key>  run the route bound to it   Esc  cancel",
		"",
	}
	// Three groups, matching the route folders: CONTEXT (this app, in-place), FOCUS
	// (an app's command you run from anywhere — it switches to that app), and GLOBAL
	// (app-less). Other apps' context routes aren't runnable here, so they're omitted.
	app := activeApp()
	var context, focus, global []routeInfo
	for _, r := range listRoutesFull() {
		switch r.Scope {
		case "global":
			global = append(global, r)
		case "focus":
			focus = append(focus, r) // any app's focus is reachable from here
		case "context":
			if strings.EqualFold(r.App, app) {
				context = append(context, r)
			}
		}
	}
	addGroup := func(title string, rs []routeInfo, tagApp bool) {
		lines = append(lines, title)
		if len(rs) == 0 {
			lines = append(lines, "  (none)")
			return
		}
		for i, r := range rs {
			if i >= 6 {
				lines = append(lines, "  …")
				break
			}
			label := r.Name
			if tagApp && !strings.EqualFold(r.App, app) {
				label += " (" + r.App + ")"
			}
			lines = append(lines, "  "+label)
		}
	}
	if len(context)+len(focus)+len(global) == 0 {
		lines = append(lines, "routes: (none yet — `m teach <name>)")
		return lines
	}
	if app != "" {
		addGroup("context — only in "+app+":", context, false)
	}
	addGroup("focus — switch to the app:", focus, true)
	addGroup("global — anywhere:", global, false)
	return lines
}

// activeApp runs `marco active` and returns the foreground app name.
func activeApp() string {
	out, err := exec.Command(marcoBin(), "active").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// argHints runs `marco args <phrase>` and returns the route's argument labels (one
// per line) to auto-pop as "name:" suggestions. Empty on no match / no args.
func argHints(phrase string) []string {
	out, err := exec.Command(marcoBin(), "args", phrase).Output()
	if err != nil {
		return nil
	}
	var hints []string
	for ln := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			hints = append(hints, ln)
		}
	}
	return hints
}
