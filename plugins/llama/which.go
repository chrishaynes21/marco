package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Answering "which of these controls did they mean" with a local model.
//
// # What makes this a different question from the other two
//
// The other two are never told anything about the screen. This one cannot be answered without
// being told, so it carries the labels and roles of the controls that are actually in front of
// the person — and that is exactly why Marco asks it through its own switch
// ($MARCO_INTERPRETER) rather than through $MARCO_RESOLVER. See internal/resolver/which.go.
//
// Local is therefore more than a convenience here. With the default configuration the labels go
// to a model on the same machine and nothing leaves it.
//
//	→ {"input":"dark mode","which":{"application":"msedge","controls":[
//	     {"id":"e12","role":"combo_box","label":"Theme"}, …]}}
//	← {"control":"e12","because":"a theme setting is where a dark appearance is chosen"}
//	← {"control":""}                                                      (an abstention)
//
// # The prompt's job is to make abstention easy
//
// A model asked to pick one of eighty things will pick one. The useful answer to "open my tax
// return" on a Settings page is that none of these is it, and a model that cannot say so is
// worse than no model: its confident wrong proposals are indistinguishable from its confident
// right ones. So NONE is named first, repeated, and given examples.

type whichRequest struct {
	Application string    `json:"application"`
	Controls    []control `json:"controls"`
}

type control struct {
	ID    string `json:"id"`
	Role  string `json:"role"`
	Label string `json:"label"`
}

// choice is the answer. Two fields, and deliberately nowhere to put a coordinate, a keystroke,
// a role, a confidence or a permission.
type choice struct {
	Control string `json:"control"`
	Because string `json:"because,omitempty"`
}

func emitChoice(c choice) {
	b, _ := json.Marshal(c)
	os.Stdout.Write(append(b, '\n'))
}

const whichPrompt = "You are given a computer user's request and a list of the controls that " +
	"are actually on their screen right now. Say which ONE control the request means.\n" +
	"Reply with ONE JSON object and nothing else:\n" +
	`{"control":"<id>","because":"<short reason>"}` + "\n" +
	"Rules:\n" +
	"- If none of the listed controls is what the user means, reply exactly NONE. Saying NONE " +
	"is a GOOD answer and is often the right one. Never pick something merely because it is " +
	"the closest of a bad list.\n" +
	"- \"control\" MUST be an id copied exactly from the list. Never invent an id, a label or " +
	"a control that is not listed.\n" +
	"- Choose by what the control DOES for the user, not by shared words. \"dark mode\" means " +
	"the control that changes appearance even if it is called something else; a control that " +
	"merely shares a word with the request is not a match.\n" +
	"- If two or more listed controls fit equally well, reply exactly NONE. Do not pick one " +
	"of them: the user will be asked.\n" +
	"- \"because\" is one short clause about why that control is what the user wants. Do not " +
	"claim anything about whether it can be clicked or allowed.\n" +
	"Output JSON only, or NONE."

func whichUser(input, application string, controls []control) string {
	var b strings.Builder
	if application != "" {
		fmt.Fprintf(&b, "Application: %s\n", application)
	}
	b.WriteString("Controls on screen:\n")
	for _, c := range controls {
		fmt.Fprintf(&b, "  %s  %s  %q\n", c.ID, c.Role, c.Label)
	}
	// ABSTENTION AS AN ITEM IN THE LIST, not as a departure from the format.
	//
	// Measured: with NONE offered only as a rule in the instructions, a 3B model abstained
	// on 0 of 36 requests — including all five where the control genuinely was not on the
	// screen, each of which it answered confidently with something else. The model was not
	// disobeying so much as answering the question it had been given, which was "which of
	// these", with a list that had no way to say "not these".
	//
	// So the list has one now. Choosing it is picking an option rather than breaking a
	// format, which is a much easier thing for a small model to do.
	fmt.Fprintf(&b, "  %s  -  nothing here is what the user asked for\n", abstainID)
	fmt.Fprintf(&b, "\nUser's request: %q\n", input)
	fmt.Fprintf(&b, "Which control id? Answer %q if none of the listed controls is what "+
		"the user wants, or if two or more fit equally well.", abstainID)
	return b.String()
}

// abstainID is the id a model picks to say it does not know.
//
// Not an element id, and it cannot collide with one: element ids come from fusion and are
// "e12"-shaped. Were one ever to look like this, the engine discards an id it did not send,
// which is an abstention — the safe way round.
const abstainID = "none-of-these"

// which asks the model, and abstains on anything it cannot read.
func which(ctx context.Context, cfg config, input, application string,
	controls []control) choice {

	if input == "" || len(controls) == 0 {
		return choice{}
	}
	body, err := json.Marshal(chatReq{
		Model: cfg.Model,
		Messages: []message{
			{Role: "system", Content: whichPrompt},
			{Role: "user", Content: whichUser(input, application, controls)},
		},
		Temperature: 0, // choosing from a list, not writing
		MaxTokens:   160,
		Stream:      false,
	})
	if err != nil {
		return choice{}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return choice{}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.Key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.Key)
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return choice{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return choice{}
	}
	var cr chatResp
	if json.NewDecoder(resp.Body).Decode(&cr) != nil || len(cr.Choices) == 0 {
		return choice{}
	}
	reply := strings.TrimSpace(cr.Choices[0].Message.Content)
	if reply == "" || strings.EqualFold(reply, "NONE") ||
		strings.Contains(strings.ToLower(reply), abstainID) {
		return choice{}
	}
	var got choice
	if json.Unmarshal([]byte(extractJSON(reply)), &got) != nil {
		return choice{}
	}
	// Only an id that was offered. The engine checks this too, and both checks are worth
	// having: this one keeps a plugin honest about its own contract, and that one does not
	// trust the plugin.
	for _, c := range controls {
		if c.ID == got.Control {
			return got
		}
	}
	return choice{}
}
