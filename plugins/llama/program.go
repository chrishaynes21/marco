package main

// WHICH OF THE OPEN PROGRAMS DID THEY MEAN (the built-in skill Focus app, internal/skills).
//
// "focus chat cheap tea" names no program by its executable or its window title, and somebody
// should not have to know that Steam's window belongs to steamwebhelper.exe. So the engine asks:
// here is what was said, here are the programs that are open, which one is it. The answer is one
// of the labels it sent or nothing, and the engine discards anything else — the model chooses
// among windows that exist and can never name one that does not.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// programRequest is the question: the open programs, each a label the answer must copy.
type programRequest struct {
	Programs []programChoice `json:"programs"`
	// Routines are the person's saved routine names, which are NOT applications: words that
	// sound more like one of them are answered NONE, so a misheard routine name ("open sosh al"
	// for "open social") is left for the routine resolver.
	Routines []string `json:"routines,omitempty"`
}

type programChoice struct {
	Label string `json:"label"` // slug-shaped, what the answer copies: "chatgpt"
	Name  string `json:"name"`  // how its window names it: "ChatGPT", "Google Chrome"
	Exe   string `json:"exe"`   // its program: "chatgpt", "steamwebhelper"
}

func emitProgram(label string) {
	b, _ := json.Marshal(struct {
		Program string `json:"program"`
	}{label})
	os.Stdout.Write(append(b, '\n'))
}

// programPrompt was tuned against llama3.1:8b on a real desktop's list (ADR-327): the first
// wording refused "vs code" and "my browser" and took the nonsense word "zorblax" for Trackmania.
// The rules below are what made it answer the first kind and refuse the second.
const programPrompt = "The user wants to switch to an application that is already open on their " +
	"computer. You are given what they said and the open applications, each with a label.\n" +
	"Rules:\n" +
	"- A nickname, an abbreviation or part of the name counts: \"vs code\" is Visual Studio Code, " +
	"\"chrome\" is Google Chrome.\n" +
	"- Saying what kind of application it is counts when exactly one listed application is that " +
	"kind: \"my browser\" is the web browser, \"music\" is the music player.\n" +
	"- Speech recognition may have misheard the name: words that SOUND like an application's " +
	"name count (\"chat cheap tea\" is ChatGPT).\n" +
	"- If what they said is not the name, a nickname, a sound-alike or the kind of one listed " +
	"application, reply NONE. Never guess, and never pick an application just because it is in " +
	"the list.\n" +
	"Reply with ONLY the label, copied exactly, or NONE, and nothing else."

func programUser(said string, ps []programChoice, routines []string) string {
	var b strings.Builder
	if len(routines) > 0 {
		b.WriteString("Saved commands (NOT applications; if what they said sounds more like one " +
			"of these, reply NONE):\n")
		for _, r := range routines {
			fmt.Fprintf(&b, "- %s\n", r)
		}
		b.WriteString("\n")
	}
	b.WriteString("Open applications:\n")
	for _, p := range ps {
		fmt.Fprintf(&b, "- %s: %s (%s.exe)\n", p.Label, p.Name, p.Exe)
	}
	fmt.Fprintf(&b, "\nThey said: %q\n", said)
	b.WriteString("Answer with one label from the list above, or NONE.")
	return b.String()
}

// program asks the model and answers a label it was sent, or "".
func program(ctx context.Context, cfg config, said string, ps []programChoice, routines []string) string {
	if strings.TrimSpace(said) == "" || len(ps) == 0 {
		return ""
	}
	body, err := json.Marshal(chatReq{
		Model: cfg.Model,
		Messages: []message{
			{Role: "system", Content: programPrompt},
			{Role: "user", Content: programUser(said, ps, routines)},
		},
		Temperature: 0, // choosing from a list, not writing
		MaxTokens:   32,
		Stream:      false,
	})
	if err != nil {
		return ""
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/chat/completions",
		bytes.NewReader(body))
	if err != nil {
		return ""
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.Key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.Key)
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var cr chatResp
	if json.NewDecoder(resp.Body).Decode(&cr) != nil || len(cr.Choices) == 0 {
		return ""
	}
	labels := make([]string, 0, len(ps))
	for _, p := range ps {
		labels = append(labels, p.Label)
	}
	return pick(cr.Choices[0].Message.Content, labels)
}
