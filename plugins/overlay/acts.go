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
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
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
			h.log("tip: type  plays  to see what Marco can do, or  ui  for the control centre")
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
		// IS THIS ONE OF THE OVERLAY'S OWN WORDS? Asked ONCE, of the one table
		// (commands.go), and the answer is what every arm below tests against. Asking word
		// by word is how this chain came to disagree with the two other places the same
		// vocabulary was written down.
		//
		// A yes here does not say which arm handles it — `bind`, `forget` and the rest are
		// answered further down by overlayVerb, which adds the arity rules. It says only
		// that these words are instructions ABOUT Marco and must not be read as desktop
		// intent: `forget my play` sent to the intake is Director hunting the screen for
		// something to forget.
		local, isLocal := overlayCommand(fields)
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
		if len(fields) >= 3 && isLocal && (local.Word == cmdNarrate || local.Word == cmdVoice) &&
			isWord(fields[1], cmdLearn) {
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
		if len(fields) == 1 && isLocal && local.Word == cmdExit {
			mlogI("overlay: quit requested")
			h.setStatus("bye")
			h.requestQuit()
			return okData(nil)
		}
		// "learn <name>" records a new route IN-PLACE (no console window): the SAME
		// interactive learn as the CLI. Demonstrate, press the leader to finish; the engine
		// streams its save / scope / simplify prompts into the HUD and the controller
		// answers them with a single y/n/s keypress (see runLearnInteractive).
		// LEARN is the word a person types; `teach` still answers, undocumented, because it is
		// what they have been typing. TEACH is reserved for Marco guiding a person.
		//
		// Deleting the alias must fail TestTheLearnWordAnswersToItsOldName.
		if isLocal && local.Word == cmdLearn {
			tname := strings.TrimSpace(strings.Join(fields[1:], " "))
			if tname == "" {
				h.setStatus("learn needs a name — `m learn <name>")
				return fail("learn needs a name")
			}
			startLearn(h, tname)
			return okData(nil)
		}
		// `ui` TAKES A VIEW AND `edit` TAKES A PLAY, and conflating them was a bug a person
		// met on their first guess.
		//
		// Both words used to land on `marco edit "<argument>"`, so `ui plays` — the most
		// obvious thing to type, and the word the product now uses for what that view lists
		// — answered: No play named "plays". The overlay then logged that it had opened in
		// the browser anyway, because the claim was made at spawn time. Two wrong answers to
		// one request, and the second one hid the first.
		//
		// The overlay does NOT keep a list of view names. `marco ui <view>` owns that
		// vocabulary (cmd/marco/edit.go, uiView) and it is still growing; a copy here would
		// be a fourth list to drift, and it would refuse views that exist. What the overlay
		// owns is which ARGUMENT SPACE the word names, and that is all this decides.
		//
		// Deleting the split must fail TestUiOpensAViewAndEditOpensAPlay.
		if isLocal && (local.Word == cmdUI || local.Word == cmdEdit) {
			arg := strings.TrimSpace(strings.Join(fields[1:], " ")) // "" = the default view
			if local.Word == cmdUI {
				startUI(h, arg)
			} else {
				startEdit(h, arg)
			}
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
		// Recognising it locally stops the running play NOW, so a long play stops the
		// moment the word lands instead of after a process spawn and a socket dial. The
		// phrase still goes through the intake, because that is what reaches the Director
		// — the thing actually driving a learned play, which treats a dropped client as
		// "the work continues". This local half decides nothing: it cannot route, refuse
		// or consume the phrase.
		//
		// It ASKS before it insists. See stopRunningPlays: the difference between asking
		// and killing is the difference between a released key and one still held down on
		// the desktop after the thing holding it has gone.
		//
		// Deleting this must fail TestASpokenStopReachesTheRunningPlayImmediately.
		if intent.IsControlPhrase(name) {
			mlogI("overlay: control phrase — stopping locally too", "phrase", name)
			stopRunningPlays()
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
		launchUI(h, []string{"edit", name}, `the editor for "`+name+`"`)
		return
	}
	launchUI(h, []string{"ui"}, "the control centre")
}

// startUI opens the control centre, on a named view when one was asked for.
//
// The view name is passed THROUGH, unexamined. `marco ui <view>` owns which views exist
// (cmd/marco/edit.go, uiView) and that list is still growing; a copy of it here would be a
// fourth place the product writes its vocabulary down, and it would start refusing views
// that exist the first time one was added on the other side.
func startUI(h *model, view string) {
	args, note := []string{"ui"}, "the control centre"
	if view != "" {
		args, note = append(args, view), "the control centre — "+view
	}
	launchUI(h, args, note)
}

// launchUI spawns the control center detached, replacing any prior instance so ports don't stack.
//
// # Why a browser server needs the hook guard
//
// This was the ONE overlay spawn that did not set MARCO_NO_PANIC_STOP, and the gap was not
// harmless because the control centre is not a leaf. It serves a page with a Run button, and a
// clicked Run makes `cmd/marco/edit.go` spawn a play — inheriting this environment. So the
// GRANDCHILD installed its own WH_KEYBOARD_LL and WH_MOUSE_LL underneath the overlay's live hook,
// while the play it was running injected input through them. Dueling low-level hooks are a
// load-bearing invariant in CLAUDE.md: Windows drops a hook whose thread is slow to answer, and
// the symptom is the leader key quietly ceasing to work rather than anything that looks like a
// crash.
//
// The overlay owns the global hook for the whole desktop. Every child it starts is told so, and
// this one had been missed because it is spawned by a different function from the other three.
//
// Deleting the environment must fail TestTheControlCentreIsSpawnedUnderTheHookGuard.
// startUIChild starts the control centre, and is a package variable for the same reason
// spawnIntakeChild is one: WHAT the overlay asked to be opened has to be checkable without a
// marco binary present, and on this surface a real marco performs real input.
//
// Production never reassigns it. The environment is already on the command by the time this
// is called, so a test that swaps it still sees exactly what production built — which is the
// only version of the claim that would have caught the missing hook guard.
var startUIChild = func(cmd *exec.Cmd) error { return cmd.Start() }

// uiStartupWindow is how long the control centre gets to prove it is alive.
//
// It has to outlast the ordinary failures — a missing play, a bad argument, no marco on the
// PATH — which are all decided in milliseconds, and stay short enough that somebody who
// pressed Enter is still watching the line when it answers. A var only so a test need not
// wait in real time; production never sets it.
var uiStartupWindow = 1500 * time.Millisecond

func launchUI(h *model, args []string, note string) {
	editMu.Lock()
	if editCmd != nil && editCmd.Process != nil {
		_ = editCmd.Process.Kill()
	}
	cmd := exec.Command(marcoBin(), args...)
	cmd.Env = append(os.Environ(), "MARCO_NO_PANIC_STOP=1")
	if err := startUIChild(cmd); err != nil {
		editMu.Unlock()
		mlogE("overlay: ui failed to start", "err", err)
		h.log("could not open " + note)
		return
	}
	editCmd = cmd
	editMu.Unlock()
	mlogI("overlay: ui", "args", strings.Join(args, " "))
	h.log("opening " + note + "…")

	// SUCCESS IS REPORTED WHEN IT IS KNOWN, AND NOT BEFORE.
	//
	// This said "opened in your browser" immediately after Start(), which reports only that
	// a process was created. Every real failure happens after that: `marco ui nonsense`,
	// `marco edit <a play that does not exist>` (exit 1 with a message the person never
	// sees, because it goes to the child's stderr), a port that will not bind. The HUD
	// cheerfully claimed the browser had opened in all three cases, so the surface whose
	// whole job is to say what is happening was the one thing lying about it.
	//
	// A local web server that is still running after the startup window IS the success
	// condition — there is nothing else to wait for, and the overlay cannot see the
	// browser. An exit inside the window is a failure and says so; an exit after it is the
	// person closing the control centre, which is not news.
	//
	// Deleting this must fail TestTheHudDoesNotClaimTheBrowserOpened.
	go func() {
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }() // reap either way, so it cannot linger as a zombie
		select {
		case err := <-done:
			mlogW2("overlay: the control centre stopped before it was up", "err", err, "args", args)
			h.log("could not open " + note)
		case <-time.After(uiStartupWindow):
			h.log(note + " — open in your browser")
			<-done // still reaped; the person closes it in their own time
		}
	}()
}

// runs is every marco child the overlay currently has in flight, and whether a stop has
// already been asked of it. See trackRun for why it is all of them and not one.
var (
	runMu sync.Mutex
	runs  = map[*exec.Cmd]bool{}
)

// stopGrace is how long a child gets to stop ITSELF before the overlay stops being polite.
//
// # What the number has to cover
//
// Up to one `stopsignal.PollInterval` (100ms) for the child to notice the raised
// generation, and then an honest unwind: cancel the frame tree, run `finally`, release a
// held key, put a held mouse button down. Those are single FFI calls costing milliseconds
// each. Two seconds is more than an order of magnitude of headroom on a loaded machine.
//
// # Why not the runtime's own five seconds
//
// `internal/runtime`'s cleanupBudget is 5s, and this being shorter is deliberate rather
// than an oversight. That budget bounds a WEDGED `finally` before abandoning it where it
// stands; a wedged cleanup is exactly the case where somebody who pressed stop wants the
// process gone rather than politely waited for. Waiting out another program's worst case
// is how "stop" turns into "hang".
//
// Both directions of the error are real and neither is subtle. Too short and this is the
// old kill wearing a delay: the child is terminated mid-unwind and the key stays down,
// which is the whole defect. Too long and a child that cannot hear us carries on typing
// while the person watches it. Somebody who does not want to wait says stop again — the
// second one does not wait (see stopRunningPlays).
//
// A var only so a test does not have to wait in real time; production never sets it.
var stopGrace = 2 * time.Second

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
// `marco learn --narrate` child's stdin (one phrase per line) instead of running it.
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

// runNarrateLearn runs `marco learn --narrate <name>`, feeding it the phrases the
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
// trackCancel registers the child so a stop can reach it — on for runMarco's commands, off
// for learn (the recorder owns F12 instead) and off for a control phrase, which is the
// thing doing the cancelling.
func streamChild(h *model, trackCancel bool, args ...string) childRun {
	cmd := exec.Command(marcoBin(), args...)
	// NO_PANIC_STOP: the overlay owns the global hook (dueling LL hooks + a route's
	// injected input deadlock). SIMPLIFY_SAVES: learn's [s] simplifies AND saves (the
	// HUD can't show the preview to re-confirm). Both harmless to the other subcommands.
	cmd.Env = append(os.Environ(), "MARCO_NO_PANIC_STOP=1", "MARCO_SIMPLIFY_SAVES=1")
	// THE GUARD NAMES THE VERB THE OVERLAY SPAWNS, and nothing else. It is about what the
	// CHILD is doing, not about which word a person typed into the HUD: `teach` is still
	// accepted up there as an alias, and it becomes `learn` before it ever reaches here.
	//
	// It once tested `args[0] == "teach"` alone. When the spawned verb became `learn` the
	// guard silently stopped matching, the child no longer got the stop key, and the leader
	// key would have stopped ending a demonstration — with nothing failing to say so,
	// because the variable is read by a different process in a different module.
	//
	// Deleting this must fail TestALearnChildIsGivenTheStopKey.
	if len(args) > 0 && args[0] == "learn" {
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
			if v, ok := strings.CutPrefix(s, outcome.ResultPrefix); ok {
				result = strings.TrimSpace(v)
				return
			}
			// Unknown-`do` error: the overlay offers to learn instead (see the `do`
			// dispatch branch), so don't also log the raw engine error. Matched on the
			// do-specific hint so a failed `bind`/etc. still surfaces its own error. The
			// hint is the engine's own wording — `marco learn` — and the two modules ship
			// together, so there is one spelling to match rather than a history of them.
			if strings.HasPrefix(s, noPlayMatches) && strings.Contains(s, "marco learn") {
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

	// START, THEN REGISTER, THEN WAIT — rather than `cmd.Run()` with the registration
	// before it. Two reasons, and the second one is a real bug the first was hiding.
	//
	// `Run` is Start followed by Wait, so registering before it published a *exec.Cmd whose
	// .Process field was still nil and was about to be written by this goroutine while the
	// action loop read it. That is a data race on the one field the stop path dereferences.
	//
	// And it was a race with a consequence, not only a theoretical one: a stop that landed
	// in that window found a nil Process and did nothing at all, so the very first instant
	// of a play — the instant somebody who mis-spoke wants back — was the instant a stop
	// was silently dropped. Registering after Start closes both.
	if err := cmd.Start(); err != nil {
		pw.Close()
		<-done
		return childRun{err: err}
	}
	if trackCancel {
		trackRun(cmd)
	}

	err = cmd.Wait()
	pw.Close()
	<-done

	stopped := false
	if trackCancel {
		stopped = untrackRun(cmd)
	}
	return childRun{err: err, killed: stopped, route: route, result: result}
}

// trackRun and untrackRun are the in-flight-child registration.
//
// # Why every child, and not one slot
//
// This used to be a single slot, overwritten unconditionally. The reasoning was that the
// leader key cancels "the thing that is running" and a person means the most recent thing
// they started — but dispatch is asynchronous for typed AND spoken input by design (see
// intake.go), so two invocations a second apart are both really running, and the second
// one's claim erased the first one's handle. That child then existed with its handle in
// nobody's hand: nothing could reach it, and `isRunning` reported the truth about only one
// of the two things driving the desktop.
//
// The other honest option was to REFUSE a second invocation while one is running. That is
// the wrong place for a refusal. What happens to a request is the door's decision
// (internal/invoke), and a front end quietly dropping one would be a second router — the
// exact defect this phase removed from the spoken path. It also refuses things a person
// legitimately wants: start a long play, then ask Director something while it runs.
//
// So: track them all. That is also what the word now MEANS. The graceful half of a stop is
// a broadcast (see stopRunningPlays), so one stop already ends every play running against
// this store whether the overlay holds its handle or not; a registration naming only the
// newest child would describe something narrower than the word does.
//
// # What the registration is still for, now that the broadcast does the main job
//
//   - the timeout fallback needs a handle to terminate a child that did not hear us;
//   - `isRunning` makes the leader key the panic key, and it must be true while ANY child
//     is in flight, not only the newest — otherwise the key opens the command line while
//     something is plainly still running;
//   - the value remembers that a stop was asked of THIS child, so a death without an
//     announced `[result] ` renders as cancelled rather than failed.
//
// Deleting the map (going back to one slot) must fail TestASecondRunDoesNotStrandTheFirst.
func trackRun(cmd *exec.Cmd) {
	runMu.Lock()
	defer runMu.Unlock()
	runs[cmd] = false
}

// untrackRun deregisters a finished child and reports whether a stop had been asked of it.
func untrackRun(cmd *exec.Cmd) bool {
	runMu.Lock()
	defer runMu.Unlock()
	stopped := runs[cmd]
	delete(runs, cmd)
	return stopped
}

// runLearnInteractive runs `marco learn <name>` interactively (the recorder owns
// F12, so no Esc-cancel); its save/scope/simplify prompts are answered in the HUD.
func runLearnInteractive(h *model, name string) error {
	return streamChild(h, false, "learn", name).err
}

// stopRunningPlays is the LOCAL half of one stop: it asks, and only then insists.
//
// # This used to be a kill, and that is why a key stayed down
//
// It was `Process.Kill`. TerminateProcess runs no deferred function, so the child's
// ReleaseHeld and its cursor restore never ran: stop a play mid-keystroke and the key
// stayed down on the desktop after the thing holding it had gone. Every other cancellation
// route in the product had been given a graceful path; this one — the one an Audience
// actually uses — still killed.
//
// # The two halves, in this order
//
//  1. RAISE THE STOP GENERATION. This is a cancellation rather than a kill: a play watching
//     the generation cancels its frame tree, stops driving the desktop, unwinds through
//     `finally`, and the host releases what it was holding. It is also the ONLY half that
//     reaches a child the overlay has no handle on — a `marco do` somebody started in a
//     terminal, or the grandchild a clicked Run spawns out of the control centre.
//  2. ARM THE FALLBACK. A child that cannot hear the signal — an older marco.exe with no
//     watcher, or one wedged in a blocking host call — must still die, or "stop" becomes
//     "nothing happened". So each tracked child gets stopGrace and is terminated if it is
//     still there afterwards.
//
// A SECOND STOP DOES NOT WAIT. If a child has already been asked once and the person says
// stop again, they have watched the polite path not work and they mean it; waiting out a
// second grace period would be the overlay insisting it knows better.
//
// Deleting the raise must fail TestAStopAsksTheStoreBeforeItKillsAnything.
func stopRunningPlays() {
	// The broadcast first, and synchronously. It is one small file write, it is off the
	// hook thread (the hook pushes an action; see intake.go), and it has to be ordered
	// BEFORE the grace period starts or the grace is being counted from before anybody was
	// asked anything.
	if err := stopsignal.Raise(stopsignal.Home()); err != nil {
		// A store we cannot write to means the graceful half will not arrive at all. Say so
		// in the log, and carry on to the fallback, which is now the only thing that will
		// stop anything.
		mlogW2("overlay: could not raise the stop signal", "err", err)
	}

	runMu.Lock()
	var impatient, asked []*exec.Cmd
	for cmd, alreadyAsked := range runs {
		if alreadyAsked {
			impatient = append(impatient, cmd)
			continue
		}
		runs[cmd] = true
		asked = append(asked, cmd)
	}
	runMu.Unlock()

	for _, cmd := range impatient {
		terminate(cmd, "asked twice")
	}
	if len(asked) == 0 {
		return
	}
	// The grace is read HERE, at the moment of asking, not inside the goroutine. A child is
	// owed the grace period that was in force when somebody asked it to stop; going back to
	// the variable later would mean a value changed in between silently retimed a countdown
	// already running.
	grace := stopGrace
	// The waiting happens on its own goroutine because the caller may be the action loop
	// the HUD draws from, and freezing the window for two seconds is not what stop should
	// feel like. Nothing waits on this goroutine: a child that stops by itself simply
	// deregisters and the sweep finds nothing to do.
	go func() {
		time.Sleep(grace)
		runMu.Lock()
		var deaf []*exec.Cmd
		for _, cmd := range asked {
			if _, still := runs[cmd]; still {
				deaf = append(deaf, cmd)
			}
		}
		runMu.Unlock()
		for _, cmd := range deaf {
			terminate(cmd, "did not stop when asked")
		}
	}()
}

// terminate is the fallback, and it is a named function rather than an inline Kill so that
// it reads as what it is: the thing this file exists to stop doing by default. Every call
// says in the log why politeness was abandoned, because a killed child is a child whose
// `finally` did not run, and somebody debugging a key left held down needs to see that.
func terminate(cmd *exec.Cmd, why string) {
	if cmd.Process == nil {
		return
	}
	mlogW2("overlay: terminating a child", "why", why, "pid", cmd.Process.Pid)
	_ = cmd.Process.Kill()
}

// cancelRun is the WHOLE cancellation: stop the local plays, and tell the Director to stop
// because it is the one that may still be driving the desktop.
//
// # Why the second half was missing and why it mattered
//
// Stopping the child looks like it stops everything, and for a recorded play it does. For a
// learned play the child is a client and the Director is the performer — and the Director
// deliberately treats a dropped client as "not a cancellation: the work continues", so
// that a front end which crashed cannot abort a replay. The consequence was that pressing
// the panic key made the HUD go quiet while the desktop carried on being driven, with
// nothing left on screen offering to stop it.
//
// Deleting the stopTheDirector call must fail TestCancelAlsoStopsTheDirector.
func cancelRun() {
	stopRunningPlays()
	stopTheDirector()
}

// isRunning reports whether any cancelable route is currently executing, so the
// controller can make the leader key the panic/stop key while a route runs.
func isRunning() bool {
	runMu.Lock()
	defer runMu.Unlock()
	return len(runs) > 0
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
	h.setStatus(statusLine(out, disp))
	h.log(fmt.Sprintf("%s  %s  %s", out, disp, fmtDur(r.dur)))
	h.addHistory(disp, string(out), r.dur)
	return r
}

// inConfig is true while the settings editor owns the keyboard.
//
// Declared here rather than beside the hook that reads it, for the same reason `recording`
// below is: the key capture underneath the settings editor is Windows-only, but the thing
// that OPENS it is not — a person types `m config on any platform the HUD draws on.
var inConfig atomic.Bool

// openPanel shows the panel a word asked for.
//
// # Why the controller does not do this itself
//
// It did, and that switch was one of the three places the HUD wrote its own vocabulary
// down (see commands.go). Splitting the decision from the doing leaves the controller with
// one question — "is this a panel word" — answered by the table, and puts the doing here,
// where a test can reach it: controller_windows.go is Windows-only and its switch runs on a
// goroutine fed by a low-level keyboard hook.
//
// # Why it is synchronous even though the listing shells out
//
// panelPlays asks the engine what plays exist, which is two short `marco` reads. This runs
// on the action-processor goroutine, never the hook thread (CLAUDE.md), and the only thing
// queued behind it is whatever the person types NEXT — they have just pressed Enter. The
// alternative, a goroutine per panel, would make "the word opened the panel" a race in
// every test that checks it.
//
// Deleting an arm here must fail TestEveryPanelWordOpensItsPanel.
func openPanel(h *model, k panelKind) {
	switch k {
	case panelHelpBrowser:
		// The FULL manual, in the browser. Deliberately not the same thing as the in-HUD
		// listing: one is a reference you sit and read, the other is a glance you take
		// without leaving the application you are in.
		startUI(h, "help")
	case panelConfig:
		inConfig.Store(true)
		h.openConfig(configLines())
	case panelHere:
		// HERE: what Marco sees, believes, is learning and needs. Click-through, because a
		// person is meant to keep using another application while it is up.
		h.openWatch()
	case panelPlays:
		// The in-HUD answer to "what can I do", generated from the table and grouped by
		// where each play applies. See commandListing.
		h.showHelp(commandListing())
	case panelDiagnostics:
		// The evidence underneath what Here said, and the one mode that captures the
		// mouse. Entered by name, never implicitly, and Esc releases it.
		h.openDiagnostics()
	case panelPerception:
		// The frozen per-element sample. Its own word because it is expensive, it is a
		// point in time rather than a live view, and it answers a narrower question.
		refreshInsightDeep(h)
	}
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
		err := runLearnInteractive(h, name) // streams prompts; answered in-HUD
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

// offerLearn shows an in-HUD "Learn \"name\"? [y]es / [n]o" prompt for an unknown
// command, reusing the prompt answering path (promptAsk). The controller starts
// a demonstration on yes (see actPromptSubmit) and clears it on no.
func offerLearn(h *model, name string) {
	pendingPromptMu.Lock()
	pendingPrompt = name
	pendingPromptMu.Unlock()
	h.setStatus("unknown: " + name)
	h.setPrompt(fmt.Sprintf("Learn %q? [y]es / [n]o: ", name))
	promptAsk.Store(true)
}

// takePendingLearn returns and clears the pending learn-offer name.
func takePendingLearn() string {
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

// The hand-written help menu that used to live here is gone, and its replacement is
// commandListing() in commands.go.
//
// It had had NO CALLER for as long as the history shows: `helpOn` was never set true, so
// three render branches in view.go were unreachable and the only in-product listing of what
// Marco can do could not be reached at all. It also spelled the command words out as prose,
// which is how the HUD came to accept four words it never highlighted — nothing could check
// a paragraph against a switch statement. The listing is now generated from the one table
// every site reads, and the word `plays` opens it.
//
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
