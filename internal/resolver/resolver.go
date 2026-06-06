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
// Protocol (one JSON object each way, over the plugin's stdio):
//
//	→ {"input":"fire up sea of thieves","routes":["start-sea-of-thieves","open-chest"]}
//	← {"route":"start-sea-of-thieves"}      (empty route = no match)
//
// The plugin path comes from $MARCO_RESOLVER. Unset → deterministic only.
package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
)

type request struct {
	Input  string   `json:"input"`
	Routes []string `json:"routes"`
}

type response struct {
	Route string `json:"route"`
}

// Path returns the configured resolver-plugin executable ($MARCO_RESOLVER).
func Path() string { return os.Getenv("MARCO_RESOLVER") }

// Configured reports whether a resolver plugin is set.
func Configured() bool { return Path() != "" }

// Resolve runs the plugin to pick the route best matching input. It returns the
// matching route slug, or "" when no plugin is configured, nothing fits, or any
// error occurs — so the caller always degrades gracefully to teaching.
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
