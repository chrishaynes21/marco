// echo-bridge is a reference Marco host bridge (see spec/Hosts.md). It reads
// newline-delimited JSON requests on stdin and writes JSON responses on stdout,
// echoing each call's input back as the result data. It does no real OS work —
// it exists to demonstrate the bridge protocol and to exercise it in tests.
//
// Any language can implement a bridge by following this shape: read a line,
// parse {act, action, input}, do the effect, write {status, data}.
//
//	build: go build -o echo-bridge ./bridges/echo-bridge
//	use:   marco run --host bridge:./echo-bridge program.marco
package main

import (
	"bufio"
	"encoding/json"
	"os"
)

type request struct {
	Act    string `json:"act"`
	Action string `json:"action"`
	Input  any    `json:"input"`
}

type response struct {
	Status string `json:"status"`
	Data   any    `json:"data"`
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	out := json.NewEncoder(os.Stdout)
	for in.Scan() {
		var req request
		if err := json.Unmarshal(in.Bytes(), &req); err != nil {
			_ = out.Encode(response{Status: "failed", Data: map[string]any{"error": err.Error()}})
			continue
		}
		// Echo the input straight back as the result.
		_ = out.Encode(response{Status: "ok", Data: req.Input})
	}
}
