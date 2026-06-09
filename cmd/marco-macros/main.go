// Command marco-macros is the macros layer: a bridge process (see spec/Hosts.md)
// that fulfills the OS act surface — Key/Type/Click/Move/Sleep/Color/Focus/
// Repeat/Spam/StopSpam/Roll/EightBall/Name/Find/Secret/Activate/Launch — by
// driving the native input backend in internal/oshost. It speaks the bridge JSON
// protocol on stdio, so the engine talks to macros as a separable layer:
//
//	marco serve --host OS=bridge:marco-macros overlay.marco
//
// It reuses the very same host the in-process `--host windows` path uses, so the
// OS effects have one implementation, exercised two ways. Requests are handled
// one at a time (the bridge protocol is serialized); the stateful background
// spam loop in oshost returns immediately and persists across calls until
// StopSpam, which is why the overlay's game hotkeys use Spam, not the blocking
// Repeat.
//
// Build: go build -o marco-macros ./cmd/marco-macros
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"

	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

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

func main() {
	host := oshost.New()
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(os.Stdout)

	for in.Scan() {
		line := in.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			_ = enc.Encode(response{Status: "failed", Error: "bad request: " + err.Error()})
			continue
		}
		status, data, err := host.Invoke(runtime.HostCall{
			Act:    req.Act,
			Action: req.Action,
			Input:  runtime.ValueFromJSON(req.Input),
			Out:    os.Stderr, // keep stdout clean for the JSON protocol
			Ctx:    context.Background(),
		})
		resp := response{Status: status}
		switch {
		case err != nil:
			resp.Status, resp.Error = "failed", err.Error()
		case status == "failed":
			resp.Status = "failed"
			if data.IsError() {
				resp.Error = data.AsError().Message
			}
		default:
			resp.Data = runtime.JSONFromValue(data)
		}
		if err := enc.Encode(resp); err != nil {
			return // stdout gone (engine exited) — nothing more to do
		}
	}
}
