package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
)

// PluginAdvisor is the Advisor that consults an external plugin over a tiny JSON
// protocol on its stdio. The plugin (e.g. plugins/llama, running a local model) may
// use any model or none — this type only marshals a request and reads a reply, so
// the engine stays model-agnostic and zero-dependency. It is one Advisor among
// possible many; dispatch doesn't know or care that a model is behind it.
//
// Protocol (one JSON object each way):
//
//	→ {"mode":"converse","input":"make a command that mutes discord",
//	   "routes":["open-chest"],"app":"Discord"}
//	← {"intent":"teach","name":"mute discord","reply":"Show me how and I'll remember."}
//
// `"teach"` is the frozen WIRE VALUE of [dispatch.IntentLearn], not the product word. Learn is
// the word everywhere a person can see; out-of-tree resolvers already send this spelling, so the
// JSON keeps it.
//
// The `mode:"converse"` field lets ONE binary also serve the legacy resolver
// protocol; a resolver that doesn't understand converse just answers something the
// dispatch discards (Advise reports not-ok, and the deterministic fallback runs).
type PluginAdvisor struct{}

// PluginPath is the configured plugin ($MARCO_ASSISTANT, else $MARCO_RESOLVER — the
// same llama binary typically serves both, so wiring one enables the other).
func PluginPath() string {
	if p := os.Getenv("MARCO_ASSISTANT"); p != "" {
		return p
	}
	return os.Getenv("MARCO_RESOLVER")
}

// PluginConfigured reports whether a plugin is available.
func PluginConfigured() bool { return PluginPath() != "" }

// Default returns a PluginAdvisor when a plugin is configured, else nil — so a
// caller can wire one in a single line: dispatch.New(dispatch.Default()).
func Default() Advisor {
	if PluginConfigured() {
		return PluginAdvisor{}
	}
	return nil
}

type wireReq struct {
	Mode   string   `json:"mode"`
	Input  string   `json:"input"`
	Routes []string `json:"routes"`
	App    string   `json:"app"`
}

type wireResp struct {
	Intent string `json:"intent"`
	Route  string `json:"route"`
	Name   string `json:"name"`
	Reply  string `json:"reply"`
}

// Advise shells out to the plugin. Any failure, or an empty intent, is reported as
// not-ok so dispatch falls back to its deterministic policy.
func (PluginAdvisor) Advise(ctx context.Context, input string, routes []string, app string) (Decision, bool) {
	path := PluginPath()
	if path == "" {
		return Decision{}, false
	}
	reqBytes, err := json.Marshal(wireReq{Mode: "converse", Input: input, Routes: routes, App: app})
	if err != nil {
		return Decision{}, false
	}
	cmd := exec.CommandContext(ctx, path)
	cmd.Stdin = bytes.NewReader(append(reqBytes, '\n'))
	out, err := cmd.Output()
	if err != nil {
		return Decision{}, false
	}
	var r wireResp
	if json.Unmarshal(bytes.TrimSpace(out), &r) != nil {
		return Decision{}, false
	}
	if strings.TrimSpace(r.Intent) == "" {
		return Decision{}, false
	}
	return Decision{Intent: r.Intent, Route: r.Route, Name: r.Name, Reply: r.Reply}, true
}
