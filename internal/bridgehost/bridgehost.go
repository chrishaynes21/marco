// Package bridgehost implements an out-of-process Host (see spec/Hosts.md): it
// drives a subprocess in any language over newline-delimited JSON on stdio. This
// is how a host written in AutoHotkey, Python, Node, etc. fulfills a foreign act
// surface — Marco describes intent, the bridge process performs the OS effect.
//
// Protocol (one JSON object per line):
//
//	→ {"act":"OS","action":"Key","input":"e"}
//	← {"status":"ok","data":null}
//	← {"status":"failed","error":"unknown key"}
//
// Calls are serialized (one in-flight request at a time) so a single bridge
// process needs no request/response correlation.
package bridgehost

import (
	"bufio"
	"encoding/json"
	"io"
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

// response is the wire form read back from the bridge.
type response struct {
	Status string `json:"status"`
	Data   any    `json:"data"`
	Error  string `json:"error"`
}

// Host drives a bridge subprocess. It implements runtime.Host.
type Host struct {
	mu      sync.Mutex
	path    string
	args    []string
	started bool
	cmd     *exec.Cmd
	enc     *json.Encoder
	scan    *bufio.Scanner
	startFn func() (io.Writer, io.Reader, func() error, error) // overridable for tests
}

// New returns a bridge host that launches path (with args) on first use.
func New(path string, args ...string) *Host {
	h := &Host{path: path, args: args}
	h.startFn = h.startProcess
	return h
}

func (h *Host) startProcess() (io.Writer, io.Reader, func() error, error) {
	cmd := exec.Command(h.path, h.args...)
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

func (h *Host) ensureStarted() error {
	if h.started {
		return nil
	}
	w, r, _, err := h.startFn()
	if err != nil {
		return err
	}
	h.enc = json.NewEncoder(w)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	h.scan = sc
	h.started = true
	return nil
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
	if !h.scan.Scan() {
		err := h.scan.Err()
		msg := "bridge closed without responding"
		if err != nil {
			msg = "bridge read: " + err.Error()
		}
		return "failed", runtime.ErrVal(&runtime.Err{Message: msg}), nil
	}
	var resp response
	if err := json.Unmarshal(h.scan.Bytes(), &resp); err != nil {
		return "failed", runtime.ErrVal(&runtime.Err{Message: "bridge bad response: " + err.Error()}), nil
	}
	if resp.Status == "" {
		resp.Status = "ok"
	}
	if resp.Status == "failed" && resp.Error != "" {
		return "failed", runtime.ErrVal(&runtime.Err{Message: resp.Error}), nil
	}
	return resp.Status, runtime.ValueFromJSON(resp.Data), nil
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
