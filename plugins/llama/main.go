// llama is a Marco resolver plugin backed by a LOCAL language model. It reads one
// JSON request on stdin, asks a small instruct model which saved route best
// matches a loosely-phrased request, and writes one JSON response on stdout.
//
// It speaks the OpenAI-compatible Chat Completions protocol
// (POST <base>/chat/completions), which means the SAME plugin drives:
//
//   - a local llama via Ollama    (default: http://localhost:11434/v1, no key)
//   - LM Studio / llama.cpp server (point MARCO_LLM_URL at it)
//   - OpenAI's cloud API           (MARCO_LLM_URL=https://api.openai.com/v1 + MARCO_LLM_KEY)
//
// Local is the default on purpose: no key, no cost, nothing leaves the machine,
// and no public API to slam. It is a SEPARATE Go module so no model SDK becomes a
// dependency of marco itself — the engine stays zero-dependency. Only net/http is
// used, so there is nothing to `go get`.
//
// Protocol (matches internal/resolver — drop-in for $MARCO_RESOLVER):
//
//	→ {"input":"fire up sea of thieves","routes":["start-sea-of-thieves","open-chest"]}
//	← {"route":"start-sea-of-thieves"}      (empty route = no match)
//
// Every failure mode (model down, missing, slow, garbled reply) degrades to an
// empty route, so the assistant simply falls back to its offline matcher / teach.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
)

type request struct {
	// Mode selects the protocol: "" (or "resolve") is the legacy route matcher;
	// "converse" is the richer intent classifier dispatch uses.
	Mode   string   `json:"mode"`
	Input  string   `json:"input"`
	Routes []string `json:"routes"`
	App    string   `json:"app"` // foreground app name, context for converse
}

type response struct {
	Route string `json:"route"`
}

// decision is the converse-mode reply (matches internal/dispatch's wire shape).
type decision struct {
	Intent string `json:"intent"`
	Route  string `json:"route"`
	Name   string `json:"name"`
	Reply  string `json:"reply"`
}

func main() {
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	var req request
	if json.Unmarshal([]byte(line), &req) != nil {
		emit("")
		return
	}
	cfg := loadConfig()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()
	if req.Mode == "converse" {
		emitDecision(converse(ctx, cfg, req))
		return
	}
	emit(resolve(ctx, cfg, req))
}

func emit(route string) {
	b, _ := json.Marshal(response{Route: route})
	os.Stdout.Write(append(b, '\n'))
}

func emitDecision(d decision) {
	b, _ := json.Marshal(d)
	os.Stdout.Write(append(b, '\n'))
}
