// Package bridgehost implements an out-of-process Host (see spec/Hosts.md): it
// drives a subprocess in any language over newline-delimited JSON on stdio. This
// is how a host written in AutoHotkey, Python, Go, etc. fulfills a foreign act
// surface — Marco describes intent, the bridge process performs the OS effect.
//
// Protocol (one JSON object per line). Marco → bridge requests:
//
//	→ {"act":"OS","action":"Key","input":"e"}
//	← {"status":"ok","data":null}
//	← {"status":"failed","error":"unknown key"}
//
// The bridge may ALSO push events back to Marco (bidirectional; e.g. a global
// hotkey or a typed command from an overlay HUD) by writing feed lines on the
// same stdout, interleaved with responses:
//
//	← {"feed":"Hotkeys","event":"Stop"}
//	← {"feed":"Commands","event":"Run","data":"login to facebook"}
//
// A single reader goroutine demuxes the two by shape: a line carrying "feed" is
// an event (surfaced on Events()), anything else is the response to the in-flight
// request. Requests are serialized (one at a time) so the bridge needs no
// request/response correlation.
package bridgehost

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// request is the wire form sent to the bridge.
type request struct {
	Act    string `json:"act"`
	Action string `json:"action"`
	Input  any    `json:"input"`
}

// response is the wire form read back from the bridge for a request.
type response struct {
	Status string `json:"status"`
	Data   any    `json:"data"`
	Error  string `json:"error"`
}

// wireEvent is the wire form of a host→Marco event line (mirrors the stdin event
// shape in internal/driver/serve.go).
type wireEvent struct {
	Feed  string `json:"feed"`
	Event string `json:"event"`
	Data  any    `json:"data"`
}

// Host drives a bridge subprocess. It implements runtime.Host and, so an
// event-pushing bridge can feed a `marco serve` run, runtime.EventSource.
type Host struct {
	mu       sync.Mutex
	path     string
	args     []string
	started  bool
	startErr error
	cmd      *exec.Cmd
	enc      *json.Encoder
	startFn  func() (io.Writer, io.Reader, func() error, error) // overridable for tests

	respCh chan response      // reader goroutine → Invoke (one in-flight call)
	events chan runtime.Event // reader goroutine → serve event loop
}

// New returns a bridge host that launches path (with args) on first use.
func New(path string, args ...string) *Host {
	h := &Host{
		path:   path,
		args:   args,
		respCh: make(chan response, 1),
		events: make(chan runtime.Event, 64),
	}
	h.startFn = h.startProcess
	return h
}

func (h *Host) startProcess() (io.Writer, io.Reader, func() error, error) {
	cmd := exec.Command(h.path, h.args...)
	// Inherit the parent's stderr so the bridge child's LOGS reach the console. A nil
	// Stderr would send them to the null device (Go's default) — which is why the
	// overlay/macros bridges' debug output, and the route logs they relay, vanished.
	// stdout stays the JSON protocol pipe; only stderr carries logs.
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, err
	}
	h.cmd = cmd
	stop := func() error {
		stdin.(io.Closer).Close()
		return cmd.Wait()
	}
	return stdin, stdout, stop, nil
}

// ensureStarted launches the subprocess (once) and starts the demux reader. It
// is idempotent and safe to call from both Invoke and Events; callers hold h.mu.
func (h *Host) ensureStarted() error {
	if h.started {
		return h.startErr // sticky: a failed launch must keep failing, not nil-deref enc
	}
	if h.respCh == nil {
		h.respCh = make(chan response, 1)
	}
	if h.events == nil {
		h.events = make(chan runtime.Event, 64)
	}
	w, r, _, err := h.startFn()
	if err != nil {
		// Can't launch: close the event channel so a serve fan-in sees this
		// source end instead of waiting on it forever. Mark started (with the
		// error) so we don't retry, don't double-close, and every later Invoke
		// degrades to failed instead of dereferencing a nil encoder.
		h.started = true
		h.startErr = err
		close(h.events)
		return err
	}
	h.enc = json.NewEncoder(w)
	go h.readLoop(r)
	h.started = true
	return nil
}

// readLoop is the single demultiplexer: feed lines go to events, everything else
// is the response to the current request. On EOF it closes both channels so a
// pending Invoke unblocks (as failed) and serve sees the event source end.
func (h *Host) readLoop(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var probe struct {
			Feed string `json:"feed"`
		}
		if json.Unmarshal(line, &probe) == nil && probe.Feed != "" {
			var we wireEvent
			if json.Unmarshal(line, &we) == nil && we.Feed != "" {
				h.events <- runtime.Event{
					Feed:    we.Feed,
					Message: we.Event,
					Payload: runtime.ValueFromJSON(we.Data),
				}
			}
			continue
		}
		var resp response
		_ = json.Unmarshal(line, &resp) // a malformed response surfaces as ok/empty
		h.respCh <- resp
	}
	close(h.events)
	close(h.respCh)
}

func (h *Host) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.ensureStarted(); err != nil {
		return "failed", runtime.ErrVal(&runtime.Err{Message: err.Error()}), nil
	}
	req := request{Act: c.Act, Action: c.Action, Input: runtime.JSONFromValue(c.Input)}
	if err := h.enc.Encode(&req); err != nil {
		return "failed", runtime.ErrVal(&runtime.Err{Message: "bridge write: " + err.Error()}), nil
	}
	resp, ok := <-h.respCh
	if !ok {
		return "failed", runtime.ErrVal(&runtime.Err{Message: "bridge closed without responding"}), nil
	}
	if resp.Status == "" {
		resp.Status = "ok"
	}
	if resp.Status == "failed" && resp.Error != "" {
		return "failed", runtime.ErrVal(&runtime.Err{Message: resp.Error}), nil
	}
	return resp.Status, runtime.ValueFromJSON(resp.Data), nil
}

// Events implements runtime.EventSource: events pushed by the bridge subprocess.
// Calling it starts the subprocess (and reader) so events flow even before the
// first Invoke — a serve fan-in uses this to bring an event-pushing host online.
func (h *Host) Events() <-chan runtime.Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	_ = h.ensureStarted()
	return h.events
}

// Close shuts down the bridge subprocess if it was started.
func (h *Host) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cmd != nil && h.cmd.Process != nil {
		return h.cmd.Process.Kill()
	}
	return nil
}
