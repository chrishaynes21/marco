package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
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
	switch req.Action {
	case "Show":
		h.show(true)
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
		h.setHeard(asText(req.Input))
		return okData(nil)
	case "ListRoutes":
		names := listRoutes()
		h.setRoutes(names)
		return okData(names)
	case "Run":
		name := asText(req.Input)
		if name == "" {
			return fail("empty command")
		}
		fields := strings.Fields(name)
		// While a voice-teach session is live, every phrase IS narration — forward it
		// to the teach child (it handles "done"/"cancel" and exits) instead of running.
		if voiceActive.Load() {
			writeVoicePhrase(name)
			return okData(nil)
		}
		// "narrate teach <name>" (alias "voice teach") starts narration-driven
		// teaching: subsequent typed or spoken phrases ("click this", "type …", "wait
		// for this screen", "done") build the route. Same path whether you type each
		// phrase or speak it.
		if len(fields) >= 3 && (fields[0] == "narrate" || fields[0] == "voice") && fields[1] == "teach" {
			vname := strings.TrimSpace(strings.Join(fields[2:], " "))
			voiceActive.Store(true) // set before spawning so no early phrase leaks to Run
			h.setTeaching(true)
			h.log("narrate teach \"" + vname + "\" — type or say: click / anchor this / type … / wait for this screen / done")
			go func() {
				err := runNarrateTeach(h, vname)
				h.setTeaching(false)
				if err != nil {
					h.log("narrate teach failed: " + vname)
				}
			}()
			return okData(nil)
		}
		// "exit" / "quit" closes the overlay (the window quit also stops serve).
		if len(fields) == 1 && (fields[0] == "exit" || fields[0] == "quit") {
			h.setStatus("bye")
			h.requestQuit()
			return okData(nil)
		}
		// "teach <name>" records a new route IN-PLACE (no console window): the SAME
		// interactive teach as the CLI. Demonstrate, press F12 to finish; the engine
		// streams its save / scope / simplify prompts into the HUD and the controller
		// answers them with a single y/n/s keypress (see runTeachInteractive).
		if len(fields) >= 1 && fields[0] == "teach" {
			tname := strings.TrimSpace(strings.Join(fields[1:], " "))
			if tname == "" {
				h.setStatus("teach needs a name — `m teach <name>")
				return fail("teach needs a name")
			}
			h.setTeaching(true) // keep the HUD awake for the whole demonstration
			h.log("recording \"" + tname + "\" — demonstrate now, press F12 to save")
			go func() {
				err := runTeachInteractive(h, tname) // streams prompts; answered in-HUD
				h.setTeaching(false)
				if err != nil {
					h.log("teach failed: " + tname)
				}
			}()
			return okData(nil)
		}
		// Verb commands; anything else is a route to run.
		args := []string{"do", name}
		switch {
		case len(fields) > 0 && (fields[0] == "bind" || fields[0] == "unbind"):
			args = fields
		case len(fields) >= 2 && (fields[0] == "forget" || fields[0] == "delete" || fields[0] == "rm"):
			args = append([]string{"forget"}, fields[1:]...) // delete a route
		case len(fields) >= 2 && fields[0] == "simplify":
			args = fields // re-simplify a route from its recording
		}
		h.setStatus("running: " + name)
		if _, outcome := runRecord(h, name, args...); outcome == "failed" {
			return fail("command failed: " + name)
		}
		return okData(nil)
	case "Hotkey":
		key := asText(req.Input)
		if key == "" {
			return okData(nil)
		}
		if err, _, _ := runMarco(h, "hotkey", key); err != nil {
			return fail(err.Error())
		}
		return okData(nil)
	case "Active":
		return okData(activeApp())
	default:
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
func listRoutes() []string {
	out, err := exec.Command(marcoBin(), "routes", "--json").Output()
	if err != nil {
		return nil
	}
	var rows []struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(out, &rows) != nil {
		return nil
	}
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, r.Name)
	}
	return names
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
func runNarrateTeach(h *model, name string) error {
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
			if r, ok := strings.CutPrefix(s, "[route] "); ok {
				route = strings.TrimSpace(r) // the resolved route name, not a log line
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

// runRecord runs the command, sets the status, and appends it to the command
// history with its outcome (ok/canceled/failed) and how long it took.
func runRecord(h *model, name string, args ...string) (time.Duration, string) {
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
	h.addHistory(disp, outcome, d)
	return d, outcome
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
		"  teach <name>       record a new one (F12 to save)",
		"  simplify <route>   re-clean its steps",
		"  forget <route>     delete it",
		"  bind <key> <route> hotkey it for this app",
		"  config / help / exit",
		"`<key>  run the route bound to it   Esc  cancel",
		"",
		"routes:",
	}
	routes := listRoutes()
	if len(routes) == 0 {
		lines = append(lines, "  (none yet — `m teach <name>)")
	}
	for i, r := range routes {
		if i >= 8 {
			lines = append(lines, "  …")
			break
		}
		lines = append(lines, "  "+r)
	}
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
	for _, ln := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			hints = append(hints, ln)
		}
	}
	return hints
}
