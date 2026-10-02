package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// Answering "what did that sentence mean" with Claude.
//
// The sibling of the route question in main.go, and the same contract: one JSON object in, one JSON
// object out, every failure degrading to no reading at all.
//
// # What leaves the machine
//
// The user's sentence, and the list of verbs Marco can perform. Nothing about their screen: not a
// control, not a window title, not a label, not what Marco remembers. Marco resolves the target
// itself, locally, against a world it observed — so this plugin is told what somebody asked for and
// never what they are looking at.
//
// That boundary is the engine's, not this plugin's: `internal/resolver.Interpret` has no parameter
// for screen evidence, so there is nothing here that could be forwarded even by mistake. It is
// restated here because this is the half that reaches a network, and somebody reading only this
// file should be able to see what it sends.
//
//	→ {"input":"shut that pop-up","interpret":{"verbs":["invoke","dismiss",…]}}
//	← {"semantic":{"kind":"dismiss"}}

// interpretRequest is the question, as it arrives.
type interpretRequest struct {
	Verbs []string `json:"verbs"`
}

// semantic is the answer: a verb from the list, a phrase, a position, a flag. There is nowhere in
// it for an element, a coordinate, a window or a keystroke.
type semantic struct {
	Kind    string `json:"kind"`
	Target  string `json:"target,omitempty"`
	Ordinal int    `json:"ordinal,omitempty"`
	Deictic bool   `json:"deictic,omitempty"`
}

type interpretResponse struct {
	Semantic *semantic `json:"semantic,omitempty"`
}

func emitSemantic(s *semantic) {
	b, _ := json.Marshal(interpretResponse{Semantic: s})
	os.Stdout.Write(append(b, '\n'))
}

// interpret asks Claude what one sentence meant. nil means no reading.
func interpret(input string, verbs []string) *semantic {
	if os.Getenv("ANTHROPIC_API_KEY") == "" || len(verbs) == 0 ||
		strings.TrimSpace(input) == "" {
		return nil
	}
	client := anthropic.NewClient() // reads ANTHROPIC_API_KEY
	resp, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5_20251001,
		MaxTokens: 256,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(interpretPrompt(input, verbs))),
		},
	})
	if err != nil {
		return nil
	}
	var b strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(t.Text)
		}
	}
	return parseSemantic(b.String(), verbs)
}

func interpretPrompt(input string, verbs []string) string {
	var b strings.Builder
	b.WriteString("Translate a computer user's request into ONE action from a fixed list.\n")
	b.WriteString("Allowed verbs:\n")
	for _, v := range verbs {
		fmt.Fprintf(&b, "- %s\n", v)
	}
	fmt.Fprintf(&b, "\nThe user asked: %q\n\n", input)
	b.WriteString("Reply with ONE JSON object and nothing else:\n")
	b.WriteString(`{"kind":"","target":"","ordinal":0,"deictic":false}` + "\n\n")
	b.WriteString("Rules:\n")
	b.WriteString("- \"kind\" must be copied exactly from the allowed verbs above. " +
		"If none of them fits, reply exactly NONE.\n")
	b.WriteString("- \"target\" is the user's own words for the thing to act on, with filler " +
		"removed: \"hit the save button please\" -> \"Save\". Leave it empty when the action " +
		"is about the whole window or the current view (going back, refreshing, undoing).\n")
	b.WriteString("- \"ordinal\" is a position the user named (\"the third result\" -> 3), " +
		"else 0.\n")
	b.WriteString("- \"deictic\" is true only when the user pointed instead of naming " +
		"(\"close this\"). Never together with a target.\n")
	b.WriteString("- You cannot see their screen and must not guess what is on it. " +
		"Something else resolves the target; invent no control name they did not say.\n")
	b.WriteString("- One action only. If the request contains several, describe the first.\n")
	return b.String()
}

// parseSemantic extracts and sanitises the reply. Best-effort cleaning, never a boundary: Marco
// re-validates the verb against its own vocabulary and refuses anything outside it.
func parseSemantic(content string, verbs []string) *semantic {
	if strings.EqualFold(strings.Trim(strings.TrimSpace(content), "\"'`. \n\r"), "none") {
		return nil
	}
	i := strings.IndexByte(content, '{')
	j := strings.LastIndexByte(content, '}')
	if i < 0 || j < i {
		return nil
	}
	var s semantic
	if json.Unmarshal([]byte(content[i:j+1]), &s) != nil {
		return nil
	}
	name := strings.ToLower(strings.TrimSpace(s.Kind))
	s.Kind = ""
	for _, v := range verbs {
		if strings.ToLower(v) == name {
			s.Kind = v
			break
		}
	}
	if s.Kind == "" {
		return nil
	}
	s.Target = strings.TrimSpace(s.Target)
	if s.Deictic && s.Target != "" {
		// The named half is what the user actually said; keep it.
		s.Deictic = false
	}
	if s.Ordinal < 0 {
		s.Ordinal = 0
	}
	return &s
}
