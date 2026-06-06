// claude-resolver is a reference Marco resolver plugin: it reads one JSON
// request on stdin, asks Claude Haiku which saved route best matches the
// request, and writes one JSON response on stdout. It is a SEPARATE Go module so
// the Anthropic SDK never becomes a dependency of marco itself.
//
//	build: cd plugins/claude-resolver && go build -o claude-resolver .
//	use:   set MARCO_RESOLVER to the built binary, e.g.
//	         export MARCO_RESOLVER=$PWD/plugins/claude-resolver/claude-resolver
//	         export ANTHROPIC_API_KEY=sk-...
//	       then `marco assistant` falls back to Haiku when its local matcher is unsure.
//
// Protocol (matches internal/resolver):
//
//	→ {"input":"fire up sea of thieves","routes":["start-sea-of-thieves","open-chest"]}
//	← {"route":"start-sea-of-thieves"}      (empty route = no match)
//
// Any language can implement this contract; this one happens to use Go + the SDK.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

type request struct {
	Input  string   `json:"input"`
	Routes []string `json:"routes"`
}

type response struct {
	Route string `json:"route"`
}

func main() {
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	var req request
	if json.Unmarshal([]byte(line), &req) != nil {
		emit("")
		return
	}
	emit(resolve(req))
}

func emit(route string) {
	b, _ := json.Marshal(response{Route: route})
	os.Stdout.Write(append(b, '\n'))
}

func resolve(req request) string {
	if os.Getenv("ANTHROPIC_API_KEY") == "" || len(req.Routes) == 0 || strings.TrimSpace(req.Input) == "" {
		return ""
	}
	client := anthropic.NewClient() // reads ANTHROPIC_API_KEY
	resp, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5_20251001,
		MaxTokens: 32,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt(req))),
		},
	})
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(t.Text)
		}
	}
	ans := strings.ToLower(strings.TrimSpace(b.String()))
	for _, r := range req.Routes {
		if ans == r {
			return r
		}
	}
	return ""
}

func prompt(req request) string {
	var b strings.Builder
	b.WriteString("You map a user's request to one of their saved automation routes.\n")
	b.WriteString("Available routes (slugs):\n")
	for _, r := range req.Routes {
		fmt.Fprintf(&b, "- %s\n", r)
	}
	fmt.Fprintf(&b, "Request: %q\n", req.Input)
	b.WriteString("Reply with ONLY the single best-matching route slug, exactly as written above, " +
		"or NONE if none clearly fits. No other text.")
	return b.String()
}
