// Command ocr is the Marco text/OCR resolver: a bridge host (see spec/Hosts.md)
// that fulfills a `Text` act with a single action, Find — locate on-screen TEXT and
// return where it is. It's the third anchor resolver, and unlike the engine's image
// and colour resolvers (which are wait GATES that confirm a static screen, then click
// the recorded coordinate) text is a LOCATOR: you reach for it precisely because the
// target MOVED (a reflowing UI — Discord's mute button, a web button), so it returns
// the live position of the word and the route clicks THERE.
//
// It speaks the bridge JSON protocol on stdio, one object per line:
//
//	→ {"act":"Text","action":"Find","input":{"Text":"Mute","X1":..,"Y1":..,"X2":..,"Y2":..,"Timeout":..}}
//	← {"status":"ok","data":{"X":640,"Y":480}}      // centre of the matched word
//	← {"status":"failed"}                           // not on screen (route falls back)
//	← {"status":"failed","error":"tesseract: ..."}  // OCR engine unavailable
//
// It also serves a teach-time Read action — OCR a captured button template (sent as a
// base64 PNG) and return the text on it, so a DEMONSTRATED anchor can be labelled and
// gain the same text locator a narrated "click the text X" gets. Read is invoked
// directly by the engine over the bridge during teach; it isn't a route capability:
//
//	→ {"act":"Text","action":"Read","input":{"Image":"<base64 png>"}}
//	← {"status":"ok","data":{"Text":"Start Game"}}  // label read off the button
//	← {"status":"failed"}                           // nothing readable (anchor stays gate-only)
//
// Wire it into a run with `--host Text=bridge:ocr` (the engine launches it on first
// use; teach uses the same $MARCO_OCR wiring). It lives in its own module so its OCR
// dependency never reaches the zero-dep engine; it reuses the engine's cross-platform
// screen capture via a local replace.
//
// Build: go -C plugins/ocr build -o ocr.exe .
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// request is one bridge call. Input is the Marco set as a JSON object (set fields
// map to object keys; numbers arrive as float64).
type request struct {
	Act    string         `json:"act"`
	Action string         `json:"action"`
	Input  map[string]any `json:"input"`
}

// response mirrors the bridge reply shape used by marco-macros.
type response struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
	Error  string `json:"error,omitempty"`
}

func main() {
	eng := newTesseract()
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
		status, data, err := handle(eng, req)
		resp := response{Status: status}
		switch {
		case err != nil:
			resp.Status, resp.Error = "failed", err.Error()
		case status == "ok":
			resp.Data = data
		}
		if err := enc.Encode(resp); err != nil {
			return // stdout gone (engine exited) — nothing more to do
		}
	}
}

// handle dispatches a bridge request to the Text act's actions.
func handle(eng ocrEngine, req request) (string, any, error) {
	switch strings.ToLower(req.Action) {
	case "find":
		return doFind(eng, req.Input)
	case "read":
		return doRead(eng, req.Input)
	default:
		return "failed", nil, fmt.Errorf("Text host has no action %q", req.Action)
	}
}
