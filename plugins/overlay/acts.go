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

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/invoke"
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
			h.log("tip: type  ui  for the visual editor — plays · bindings · config · help")
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
		// Trimmed before the empty test, not after: a phrase of pure whitespace used to
		// pass it and then index fields[0] on an empty slice.
		name := strings.TrimSpace(asText(req.Input))
		if name == "" {
			return fail("empty command")
		}
		fields := strings.Fields(name)
		// While a voice-learn session is live, every phrase IS narration — forward it
		// to the learn child (it handles "done"/"cancel" and exits) instead of running.
		if voiceActive.Load() {
			mlogD("overlay: forwarding to voice-learn", "phrase", name)
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
		// "narrate learn <name>" (aliases "narrate teach", "voice teach") starts narration-driven
		// learning: subsequent typed or spoken phrases ("click this", "type …", "wait
		// for this screen", "done") build the route. Same path whether you type each
		// phrase or speak it.
		if len(fields) >= 3 && (fields[0] == "narrate" || fields[0] == "voice") &&
			(fields[1] == "learn" || fields[1] == "teach") {
			vname := strings.TrimSpace(strings.Join(fields[2:], " "))
			mlogI("overlay: narrate-learn starting", "name", vname)
			voiceActive.Store(true) // set before spawning so no early phrase leaks to Run
			h.setLearnSession(true)
			h.log("narrate learn \"" + vname + "\" — type or say: click / anchor this / type … / wait for this screen / done")
			go func() {
				err := runNarrateLearn(h, vname)
				h.setLearnSession(false)
				if err != nil {
					mlogE("overlay: narrate-learn failed", "name", vname, "err", err)
					h.log("narrate learn failed: " + vname)
				} else {
					mlogI("overlay: narrate-learn done", "name", vname)
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
		// "learn <name>" records a new route IN-PLACE (no console window): the SAME
		// interactive learn as the CLI. Demonstrate, press the leader to finish; the engine
		// streams its save / scope / simplify prompts into the HUD and the controller
		// answers them with a single y/n/s keypress (see runTeachInteractive).
		// LEARN is the word a person types; `teach` still answers, undocumented, because it is
		// what they have been typing. TEACH is reserved for Marco guiding a person.
		//
		// Deleting the alias must fail TestTheLearnWordAnswersToItsOldName.
		if len(fields) >= 1 && (fields[0] == "learn" || fields[0] == "teach") {
			tname := strings.TrimSpace(strings.Join(fields[1:], " "))
			if tname == "" {
				h.setStatus("learn needs a name — `m learn <name>")
				return fail("learn needs a name")
			}
			startLearn(h, tname)
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
		// THE OVERLAY'S OWN VERBS, and nothing else, stay here. bind / unbind / press /
		// forget / rename / simplify are instructions about Marco's catalogue, not about
		// the desktop; they are `marco` subcommands and were never invocations.
		if verb, ok := overlayVerb(name, fields); ok {
			mlogI("overlay: running", "name", name, "args", verb)
			h.setStatus("running: " + name)
			r := runRecord(h, name, verb...)
			if r.err != nil {
				mlogW2("overlay: command failed", "name", name)
				return fail("command failed: " + name)
			}
			return okData(nil)
		}

		// EVERYTHING PAST HERE IS ONE REQUEST, whichever way it arrived.
		//
		// The only difference between typing and speaking is which word goes in
		// --source, and the engine records that word without ever reading it to decide
		// anything. See intake.go for the whole of why.
		source := invoke.SourceTyped
		if req.Action == "RunVoice" {
			source = invoke.SourceSpoken
			h.setHeard(name) // the HUD still shows what was heard
		}

		// A CONTROL WORD IS RECOGNISED HERE FOR IMMEDIACY, AND FOR NOTHING ELSE.
		//
		// `intent.IsControlPhrase` is the ONE definition — the same function the engine's
		// intake and the Director's own phrase routing use. A second list in the overlay
		// would be a second answer to "did they say stop", and the two would drift the
		// first time a word was added to either.
		//
		// Recognising it locally kills the child NOW, so a long play stops the moment the
		// word lands instead of after a process spawn and a socket dial. The phrase still
		// goes through the intake, because that is what reaches the Director — the thing
		// actually driving a learned play, which treats a dropped client as "the work
		// continues". This local half decides nothing: it cannot route, refuse or consume
		// the phrase.
		//
		// Deleting this must fail TestASpokenStopKillsTheChildImmediately.
		if intent.IsControlPhrase(name) {
			mlogI("overlay: control phrase — cancelling locally too", "phrase", name)
			killChildRun()
		}

		h.setStatus("running: " + name)
		dispatchIntake(h, name, source)
		return okData(nil)
	case "Hotkey":
		key := asText(req.Input)
		if key == "" {
			return okData(nil)
		}
		mlogD("overlay: hotkey", "key", key)
		// A hotkey is an EXPLICIT IDENTITY, not a phrase: `marco hotkey` resolves the
		// binding to a play once and hands the intake that identity, so the words are
		// never read back. Nothing about that decision belongs here.
		if r := runMarco(h, "hotkey", key); r.err != nil {
			mlogE("overlay: hotkey failed", "key", key, "err", r.err)
			return fail(r.err.Error())
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

// Interactive-prompt plumbing (no longer named for one subcommand: it now
// serves ANY subcommand's prompts — learn save/scope/simplify, a `forget` delete
// confirm, a `simplify` confirm, …). The child's stdin is promptPipe; the global
// hook sets promptAsk when a prompt is up and the controller writes the y/n/s reply.
var (
	promptMu   sync.Mutex
	promptPipe io.WriteCloser // the active interactive child's stdin
	promptAsk  atomic.Bool    // a y/n/s prompt is waiting for an answer
)

// writePromptAnswer forwards the submitted answer (e.g. "y", "n", "s", or "" for the
// bare-Enter default) plus a newline to the active child's stdin. Called from the
// controller's processor goroutine (NOT the hook thread), so the write is safe.
func writePromptAnswer(s string) {
	promptMu.Lock()
	w := promptPipe
	promptMu.Unlock()
	if w != nil {
		_, _ = w.Write([]byte(s + "\n"))
	}
}

// Voice-learn plumbing: while voiceActive, the dispatcher pipes each phrase to the
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

// writeVoicePhrase forwards one narration phrase to the voice-learn child's stdin.
func writeVoicePhrase(phrase string) {
	voiceMu.Lock()
	w := voicePipe
	voiceMu.Unlock()
	if w != nil {
		_, _ = w.Write([]byte(phrase + "\n"))
	}
}

// runNarrateLearn runs `marco teach --narrate <name>`, feeding it the phrases the
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

func runNarrateLearn(h *model, name string) error {
	// Signal the voice plugin to re-arm between phrases for the duration of this
	// session. The file is removed in the defer below (normal exit) or simply
	// expires as stale if the overlay crashes.
	lock := narrateLockPath()
	_ = os.WriteFile(lock, nil, 0o600)
	defer os.Remove(lock)

	cmd := exec.Command(marcoBin(), "learn", "--narrate", name)
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

// noPlayMatches is the prefix of the engine's unknown-command error. It is PRODUCED
// in the ROOT module by cmd/marco/panicstop.go and cmd/marco/bind.go; nothing links
// the two literals at compile time because the overlay is a separate Go module, so
// they must be changed together. Deleting or editing it must fail TestNoPlayMatchesPrefix.
const noPlayMatches = "no play matches "

// streamChild runs a marco subcommand, streams its stdout/stderr into the HUD,
// and makes ANY interactive prompt it raises answerable from the overlay: the
// child gets a stdin pipe (promptPipe) that the controller writes the y/n/s answer
// to, and prompts are detected on the partial (newline-less) line.
//
// It returns a childRun: what the engine ANNOUNCED (`[route] ` and `[result] `) plus
// whether the process errored or was killed here. The two announced lines are the whole
// reason this returns a struct — the outcome is read, not guessed from an exit code.
//
// trackCancel registers the child so the leader key can kill it — on for runMarco's
// commands, off for learn (the recorder owns F12 instead) and off for a control phrase,
// which is the thing doing the cancelling.
func streamChild(h *model, trackCancel bool, args ...string) childRun {
	cmd := exec.Command(marcoBin(), args...)
	// NO_PANIC_STOP: the overlay owns the global hook (dueling LL hooks + a route's
	// injected input deadlock). NO_TEACH: an unknown `do` errors instead of dropping
	// into learn-on-unknown. SIMPLIFY_SAVES: learn's [s] simplifies AND saves (the
	// HUD can't show the preview to re-confirm). All harmless to non-learn commands.
	cmd.Env = append(os.Environ(), "MARCO_NO_PANIC_STOP=1", "MARCO_NO_TEACH=1", "MARCO_SIMPLIFY_SAVES=1")
	if len(args) > 0 && args[0] == "teach" {
		// The leader key stops the demo. The overlay passes it through while recording so
		// it reaches the recorder's stop detection (see handleKey's recording gate).
		cmd.Env = append(cmd.Env, "MARCO_STOP_KEY="+cfgLeader())
	}
	inW, err := cmd.StdinPipe()
	if err != nil {
		return childRun{err: err}
	}
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	promptMu.Lock()
	promptPipe = inW
	promptMu.Unlock()
	defer func() {
		promptMu.Lock()
		promptPipe = nil
		promptMu.Unlock()
		promptAsk.Store(false)
		h.clearPrompt()
		_ = inW.Close()
	}()

	route, result := "", ""
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
			// "[route] " is WIRE PROTOCOL, not user text: the engine announces the
			// resolved play on it and cmd/marco has a live test pinning the producer
			// side. It is deliberately NOT renamed with the product vocabulary.
			if r, ok := strings.CutPrefix(s, routePrefix); ok {
				route = strings.TrimSpace(r) // the resolved play name, not a log line
				return
			}
			// "[result] " is the same kind of line and says what BECAME of it. Both are
			// consumed rather than logged: they are for this program, not for a person
			// reading the HUD, which renders the outcome as its own row.
			if v, ok := strings.CutPrefix(s, resultPrefix); ok {
				result = strings.TrimSpace(v)
				return
			}
			// Unknown-`do` error: the overlay offers to learn instead (see the `do`
			// dispatch branch), so don't also log the raw engine error. Matched on the
			// do-specific hint so a failed `bind`/etc. still surfaces its own error. Both
			// spellings are accepted: the engine says `marco learn`, older builds said `marco teach`.
			if strings.HasPrefix(s, noPlayMatches) &&
				(strings.Contains(s, "marco learn") || strings.Contains(s, "marco teach")) {
				return
			}
			// A LONG COMMAND HAS TO BE LEGIBLE WHILE IT RUNS. The Director's rendered
			// events stream through this same child now that spoken phrases take the
			// ordinary path, so the status line follows them — "heard: …", then
			// "[3/5] …", then the question if one is asked. The Director's own wording
			// is used as-is; re-describing it here would be a second vocabulary.
			if st, ok := directorStatusLine(s); ok {
				h.setStatus(st)
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
			if !promptAsk.Load() && bytes.Contains(line, []byte("? [")) && bytes.HasSuffix(line, []byte(": ")) {
				h.setPrompt(strings.TrimRight(string(line), "\r"))
				promptAsk.Store(true)
				line = line[:0]
			}
		}
	}()

	if trackCancel {
		claimRunSlot(cmd)
	}

	err = cmd.Run()
	pw.Close()
	<-done

	killed := false
	if trackCancel {
		killed = releaseRunSlot(cmd)
	}
	return childRun{err: err, killed: killed, route: route, result: result}
}

// claimRunSlot and releaseRunSlot are the cancellable-run registration.
//
// There is ONE slot, and that is deliberate: the leader key cancels "the thing that is
// running", and a person who presses it means the most recent thing they started.
//
// # Why release checks that the slot is still ours
//
// Because dispatch is asynchronous for BOTH sources now, a second invocation can start
// while the first is still going — which is the whole reason it is asynchronous. Without
// the check, the child that finished FIRST would clear a LATER child's registration, and
// the leader key would then find nothing to cancel while something was plainly still
// running: it would open the command line instead of stopping the desktop.
//
// Deleting the identity check must fail TestAFinishedChildDoesNotDeregisterALaterOne.
func claimRunSlot(cmd *exec.Cmd) {
	runMu.Lock()
	defer runMu.Unlock()
	runCmd, runCanceled = cmd, false
}

// releaseRunSlot gives the slot back and reports whether THIS child was the one killed.
func releaseRunSlot(cmd *exec.Cmd) bool {
	runMu.Lock()
	defer runMu.Unlock()
	if runCmd != cmd {
		return false
	}
	killed := runCanceled
	runCmd = nil
	return killed
}

// runTeachInteractive runs `marco teach <name>` interactively (the recorder owns
// F12, so no Esc-cancel); its save/scope/simplify prompts are answered in the HUD.
func runTeachInteractive(h *model, name string) error {
	return streamChild(h, false, "learn", name).err
}

// killChildRun kills the in-flight child, if any, marking it cancelled (vs failed).
//
// This is HALF a cancellation: it is the immediate, local half. It stops the process the
// overlay spawned, which for an ordinary play IS the thing performing — and for a learned
// play is only a client holding a socket.
func killChildRun() {
	runMu.Lock()
	defer runMu.Unlock()
	if runCmd != nil && runCmd.Process != nil {
		runCanceled = true
		_ = runCmd.Process.Kill()
	}
}

// cancelRun is the WHOLE cancellation: kill the child for immediacy, and tell the
// Director to stop because it is the one that may still be driving the desktop.
//
// # Why the second half was missing and why it mattered
//
// Killing the child looks like it stops everything, and for a recorded play it does. For a
// learned play the child is a client and the Director is the performer — and the Director
// deliberately treats a dropped client as "not a cancellation: the work continues", so
// that a front end which crashed cannot abort a replay. The consequence was that pressing
// the panic key made the HUD go quiet while the desktop carried on being driven, with
// nothing left on screen offering to stop it.
//
// Deleting the stopTheDirector call must fail TestCancelAlsoStopsTheDirector.
func cancelRun() {
	killChildRun()
	stopTheDirector()
}

// isRunning reports whether a cancelable route is currently executing, so the
// controller can make the leader key the panic/stop key while a route runs.
func isRunning() bool {
	runMu.Lock()
	defer runMu.Unlock()
	return runCmd != nil
}

// runRecord runs the command, sets the status, and appends it to the command history with
// its outcome and how long it took.
//
// The outcome is the engine's, read off `[result] `; only a subcommand that announces none
// falls back to the exit code. That is the whole of what changed here, and it is why a
// refusal no longer renders as a success — see outcome.go.
func runRecord(h *model, name string, args ...string) childRun {
	return runRecordTracked(h, name, true, args...)
}

// runRecordTracked is runRecord with the cancellable-slot registration made explicit, for
// the one caller that must not claim it: a control phrase is the thing doing the
// cancelling, not a thing to cancel.
func runRecordTracked(h *model, name string, track bool, args ...string) childRun {
	start := time.Now()
	r := streamChild(h, track, args...)
	r.dur = time.Since(start)
	disp := name
	if r.route != "" {
		disp = r.route // show the canonical play a loose phrase resolved to
	}
	out := r.outcome()
	h.setStatus(out.status(disp))
	h.log(fmt.Sprintf("%s  %s  %s", out, disp, fmtDur(r.dur)))
	h.addHistory(disp, string(out), r.dur)
	return r
}

// recording is true while a demonstration learn child is running. The keyboard hook
// reads it to pass every key through to the recorder (and the app) — so the leader
// key reaches the recorder's stop detection and ends the demo, instead of opening
// the overlay command line.
var recording atomic.Bool

// startLearn kicks off an in-HUD demonstration learn for name (record → leader → save),
// keeping the panel awake and streaming the engine's prompts into the HUD. Shared by
// the explicit `m learn <name>` command and the unknown-command learn offer.
func startLearn(h *model, name string) {
	mlogI("overlay: learn starting", "name", name)
	h.setLearnSession(true) // keep the HUD awake for the whole demonstration
	recording.Store(true)
	h.log("recording \"" + name + "\" — demonstrate now, press " + strings.ToUpper(cfgLeader()) + " to save")
	go func() {
		err := runTeachInteractive(h, name) // streams prompts; answered in-HUD
		recording.Store(false)
		h.setLearnSession(false)
		if err != nil {
			mlogE("overlay: learn failed", "name", name, "err", err)
			h.log("learn failed: " + name)
		} else {
			mlogI("overlay: learn done", "name", name)
		}
	}()
}

// pendingPrompt holds the route name an unknown-command learn offer would learn if
// accepted ("" = none). Set when `marco do <name>` resolves nothing; consumed by the
// controller when the user answers the in-HUD "Learn …?" prompt.
var (
	pendingPromptMu sync.Mutex
	pendingPrompt   string
)

// offerTeach shows an in-HUD "Learn \"name\"? [y]es / [n]o" prompt for an unknown
// command, reusing the prompt answering path (promptAsk). The controller starts
// a demonstration on yes (see actTeachSubmit) and clears it on no.
func offerTeach(h *model, name string) {
	pendingPromptMu.Lock()
	pendingPrompt = name
	pendingPromptMu.Unlock()
	h.setStatus("unknown: " + name)
	h.setPrompt(fmt.Sprintf("Learn %q? [y]es / [n]o: ", name))
	promptAsk.Store(true)
}

// takePendingTeach returns and clears the pending learn-offer name.
func takePendingTeach() string {
	pendingPromptMu.Lock()
	defer pendingPromptMu.Unlock()
	n := pendingPrompt
	pendingPrompt = ""
	return n
}

func fmtDur(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// runMarco runs a marco subcommand, streaming output into the HUD and answering
// any prompt it raises (a `forget` delete confirm, a `simplify` confirm, …) from
// the overlay. The leader key cancels it.
func runMarco(h *model, args ...string) childRun {
	return streamChild(h, true, args...)
}

// helpLines builds the help menu: the leader keys plus the known plays.
func helpLines() []string {
	lines := []string{
		"`m then type:",
		"  <play>             run it",
		"  learn <name>       record a new one (leader to save)",
		"  ui                 visual editor: plays · bindings · config · help",
		"  edit <play>        edit one play in the browser",
		"  simplify <play>    re-clean its steps",
		"  rename <old> to <new>  rename it",
		"  forget <play>      delete it",
		"  bind <key> <play>  hotkey it for this app",
		"  voice on|off / config / help / exit",
		"`<key>  run the play bound to it   Esc  cancel",
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
		lines = append(lines, "plays: (none yet — `m learn <name>)")
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
