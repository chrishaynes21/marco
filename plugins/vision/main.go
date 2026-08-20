// Command vision is the Marco vision resolver: a bridge host (see spec/Hosts.md) that
// fulfills a `Vision` act by running a learned UI-element detector (an ONNX model) over a
// screenshot. Where the image/colour/edge anchors are GEOMETRIC (match recorded pixels)
// and Text is OCR, Vision is SEMANTIC: it answers "where is the button / icon / menu item
// / prompt", finding controls that have no clean edges (an icon on a gradient) and giving
// each a class label — so a route can say "click the icon that does X" or resolve a moved
// control by what KIND of thing it is.
//
// It speaks the bridge JSON protocol on stdio, one object per line:
//
//	→ {"act":"Vision","action":"Detect","input":{"X1":..,"Y1":..,"X2":..,"Y2":..}}
//	← {"status":"ok","data":{"Elements":[{"Label":"button","X":..,"Y":..,"W":..,"H":..,"Score":..}]}}
//	→ {"act":"Vision","action":"Locate","input":{"Label":"button","X":..,"Y":..}}
//	← {"status":"ok","data":{"X":640,"Y":480}}   // centre of the best-matching element
//	← {"status":"failed"}                        // nothing matched (route falls back)
//	← {"status":"failed","error":"Vision has no model loaded"}  // detector unavailable
//
// It also serves a learn-time Identify action — run the detector on a captured button
// template (base64 PNG) and return the CLASS of the control under the click — so a
// DEMONSTRATED anchor records what KIND of thing it is. Like the OCR host's Read, it's
// invoked directly by the engine over the bridge during learn, not a route capability:
//
//	→ {"act":"Vision","action":"Identify","input":{"Image":"<base64 png>","ClickX":..,"ClickY":..}}
//	← {"status":"ok","data":{"Label":"icon"}}    // the control clicked
//	← {"status":"failed"}                         // no element there (anchor unaffected)
//
// Wire it into a run with `--host Vision=bridge:vision` (the engine launches it on first
// use). It lives in its own module so its ONNX dependency never reaches the zero-dep
// engine; it reuses the engine's cross-platform screen capture via a local replace.
//
// Build (default, dependency-free null detector — builds & runs everywhere):
//
//	go -C plugins/vision build -o vision.exe .
//
// Build (real detector — needs the ONNX binding (cgo, like voice), a runtime lib, a model):
//
//	go -C plugins/vision get github.com/yalue/onnxruntime_go
//	CGO_ENABLED=1 go -C plugins/vision build -tags onnxvision -o vision.exe .
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
)

// request is one bridge call. Input is the Marco set as a JSON object (numbers arrive as
// float64).
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
	// `vision detect <in.png> [out.png]` is a one-shot debug spike (annotated detections on
	// a screenshot file); any other invocation is the bridge host reading JSON on stdin.
	if len(os.Args) > 1 && os.Args[1] == "detect" {
		os.Exit(runDetect(os.Args[2:]))
	}
	det := newDetector()
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), bridgehost.MaxLine)
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
		status, data, err := handle(det, req)
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

	// Why the loop ended, when it was not EOF.
	//
	// A request too long to read stops the scanner, and simply falling out of the loop
	// exits the process without a word — which the engine sees as a closed pipe and
	// reports as a dead plugin. The frame that did it was a fullscreen game capture, and
	// nothing in that message would have pointed at its size.
	if err := in.Err(); err != nil {
		_ = enc.Encode(response{Status: "failed", Error: "unreadable request: " + err.Error()})
		fmt.Fprintf(os.Stderr, "%s: unreadable request (limit %d bytes): %v\n",
			filepath.Base(os.Args[0]), bridgehost.MaxLine, err)
		os.Exit(1)
	}
}

// handle dispatches a bridge request to the Vision act's actions.
func handle(det detector, req request) (string, any, error) {
	switch strings.ToLower(req.Action) {
	case "detect":
		return doDetect(det, req.Input)
	case "locate":
		return doLocate(det, req.Input)
	case "identify":
		return doIdentify(det, req.Input)
	default:
		return "failed", nil, fmt.Errorf("Vision host has no action %q", req.Action)
	}
}
