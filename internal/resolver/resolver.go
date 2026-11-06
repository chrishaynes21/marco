// Package resolver is the zero-dependency client for an optional, external
// natural-language resolver "plugin". When the deterministic matcher
// (internal/nlu) is unsure, Marco shells out to a configured program over a tiny
// JSON protocol to map a loosely-phrased request to one of the user's routes.
//
// Keeping the model out of core is deliberate: Marco itself has no third-party
// dependencies. The plugin (e.g. plugins/claude-resolver) is a separate module
// that may use whatever SDK it likes — same "play nicely with other languages"
// boundary as the bridge host.
//
// Protocol (one JSON object each way, over the plugin's stdio). There are two questions, and a
// plugin may answer either or both — the field it is asked by says which:
//
//	WHICH PLAY DID THEY MEAN
//	→ {"input":"fire up sea of thieves","routes":["start-sea-of-thieves","open-chest"]}
//	← {"route":"start-sea-of-thieves"}      (empty route = no match)
//
//	WHAT DID THAT SENTENCE MEAN                                        (see interpret.go)
//	→ {"input":"shut that pop-up","interpret":{"verbs":["invoke","dismiss",…]}}
//	← {"semantic":{"kind":"dismiss"}}       (no "semantic" = no interpretation)
//
// The second question is ADDITIVE and was added without a protocol version: a plugin that only
// knows the first sees one unrecognised field, answers `{"route":""}`, and the caller correctly
// reads that as "nothing interpreted it". Neither question is ever asked with the other's field
// present, so no plugin has to decide which it was being asked.
//
// Neither question is ever told what is on screen. See the note on Interpret.
//
// The plugin path comes from $MARCO_RESOLVER. Unset → deterministic only.
package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type request struct {
	Input  string   `json:"input"`
	Routes []string `json:"routes"`
}

type response struct {
	Route string `json:"route"`
}

// Path returns the resolver-plugin executable: $MARCO_RESOLVER when it is SET — an empty value
// meaning none, which is how a test or a person turns it off — and otherwise the local plugin
// beside this executable (Beside).
//
// # Why it is looked for
//
// Because nothing else set it. setup.ps1 pinned $MARCO_RESOLVER into overlay.cmd only, so a
// Director started from Studio's Open widget had no model to ask, and a misheard spoken request
// ("cloud paul stone") was answered by the deterministic arms alone, which cannot recover a
// name. The plugin is looked for where every other optional piece is: beside marco.exe, as the
// widget looks for voice.exe. See ADR-321.
func Path() string {
	if p, set := os.LookupEnv("MARCO_RESOLVER"); set {
		return strings.TrimSpace(p)
	}
	return Beside()
}

// Beside is plugins/llama/llama.exe beside this executable, or "" when it is not there.
func Beside() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	name := "llama"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	at := filepath.Join(filepath.Dir(exe), "plugins", "llama", name)
	if fi, err := os.Stat(at); err != nil || fi.IsDir() {
		return ""
	}
	return at
}

// Configured reports whether a resolver plugin is set.
func Configured() bool { return Path() != "" }

// Resolve runs the plugin to pick the route best matching input. It returns the
// matching route slug, or "" when no plugin is configured, nothing fits, or any
// error occurs — so the caller always degrades gracefully to learning.
func Resolve(ctx context.Context, input string, routes []string) string {
	path := Path()
	if path == "" || len(routes) == 0 || input == "" {
		return ""
	}
	reqBytes, err := json.Marshal(request{Input: input, Routes: routes})
	if err != nil {
		return ""
	}
	cmd := exec.CommandContext(ctx, path)
	cmd.Stdin = bytes.NewReader(append(reqBytes, '\n'))
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	var resp response
	if json.Unmarshal(bytes.TrimSpace(out), &resp) != nil {
		return ""
	}
	// Accept only an exact, known route slug.
	for _, r := range routes {
		if resp.Route == r {
			return r
		}
	}
	return ""
}
