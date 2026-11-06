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
// empty route, so the assistant simply falls back to its offline matcher / learn.
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
	// Interpret, when present, asks WHAT THE SENTENCE MEANT rather than which saved route
	// it named. Selected by the field rather than by Mode, because that is how the engine
	// asks — a question identified by a field can be added without every existing plugin
	// having to know a new mode word. See interpret.go.
	Interpret *interpretRequest `json:"interpret"`
	// Which, when present, asks WHICH OF THE CONTROLS ON SCREEN the request meant. The
	// only question that is told what is in front of the person. See which.go.
	Which *whichRequest `json:"which"`
	// Extract, when present, asks WHICH WORDS OF THE REQUEST are which argument's value.
	// Selected by its field like the two above, so an older plugin never sees it and a
	// newer one answers the questions it knows. See extract.go.
	Extract *extractRequest `json:"extract"`
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
	answer(ctx, cfg, req)
}

// answer asks the one question a request carries and prints the reply.
func answer(ctx context.Context, cfg config, req request) {
	// WHAT DID THE SENTENCE MEAN, asked first because it is identified by a field rather than
	// by a mode word, and a request carrying it is never also a route question.
	if req.Interpret != nil {
		emitSemantic(interpret(ctx, cfg, req.Input, req.Interpret.Verbs))
		return
	}
	// WHICH OF THESE CONTROLS DID THEY MEAN — the only question that is told what is on
	// screen, which is why Marco asks it through its own switch. Identified by its field,
	// like the one above, so a plugin that does not know it simply never sees it.
	if req.Which != nil {
		emitChoice(which(ctx, cfg, req.Input, req.Which.Application, req.Which.Controls))
		return
	}
	// WHICH WORDS ARE THIS ARGUMENT'S VALUE. Told the sentence and a list of names, and
	// nothing else — not the screen, and not what the routine does with them.
	if req.Extract != nil {
		emitValues(extractFor(ctx, cfg.forRoutines(), req.Input, req.Extract.Routine, req.Extract.Names))
		return
	}
	if req.Mode == "converse" {
		emitDecision(converse(ctx, cfg.forRoutines(), req))
		return
	}
	emit(resolve(ctx, cfg.forRoutines(), req))
}

func emit(route string) {
	b, _ := json.Marshal(response{Route: route})
	os.Stdout.Write(append(b, '\n'))
}

func emitDecision(d decision) {
	b, _ := json.Marshal(d)
	os.Stdout.Write(append(b, '\n'))
}
