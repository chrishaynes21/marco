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

// Answering "what did that sentence mean" with a local model.
//
// # What this question is, and what makes it safe to answer locally
//
// Marco's Director can perform a fixed set of semantic verbs against whatever control a
// description resolves to. This maps one English sentence onto one of those verbs plus a
// description of the thing it is about — and that is the whole job. It does not decide what is on
// screen, does not choose a control, does not see the desktop and is never told anything about it.
//
// So the model's entire input is: the user's own sentence, and the list of verbs Marco can
// perform. Its entire output is one verb from that list, a phrase, a position and a flag. Marco
// then resolves the phrase itself, against a world it observed, and refuses honestly when nothing
// matches.
//
// That is why this is a good job for a small local model: the task is closed-vocabulary
// classification with a short free-text span, which a 3B instruct model does well, and nothing
// about anybody's screen ever leaves the machine.
//
// Protocol (the second question in internal/resolver):
//
//	→ {"input":"shut that pop-up","interpret":{"verbs":["invoke","dismiss",…]}}
//	← {"semantic":{"kind":"dismiss"}}
//
// Every failure degrades to no `semantic` field at all, which Marco reads as "nothing interpreted
// it" and answers with its own deterministic refusal.

// interpretRequest is the question, as it arrives.
type interpretRequest struct {
	Verbs []string `json:"verbs"`
}

// semantic is the answer. Four fields, and there is deliberately nowhere to put anything else:
// no element, no coordinate, no window, no keystroke, no file.
type semantic struct {
	Kind    string `json:"kind"`
	Target  string `json:"target,omitempty"`
	Ordinal int    `json:"ordinal,omitempty"`
	Deictic bool   `json:"deictic,omitempty"`
}

// interpretResponse is what goes back when there is a reading.
type interpretResponse struct {
	Semantic *semantic `json:"semantic,omitempty"`
}

func emitSemantic(s *semantic) {
	b, _ := json.Marshal(interpretResponse{Semantic: s})
	os.Stdout.Write(append(b, '\n'))
}

const interpretPrompt = "You translate a computer user's request into ONE action from a fixed " +
	"list. Reply with ONE JSON object and nothing else:\n" +
	`{"kind":"","target":"","ordinal":0,"deictic":false}` + "\n" +
	"Rules:\n" +
	"- \"kind\" MUST be copied exactly from the allowed verbs. If none of them fits what the " +
	"user asked for, reply exactly NONE.\n" +
	"- \"target\" is the user's own words for the control or thing to act on, with filler words " +
	"removed: \"hit the save button please\" -> \"Save\". Leave it empty when the action is " +
	"about the whole window or the current view (going back, refreshing, undoing).\n" +
	"- \"ordinal\" is a position when the user named one (\"the third result\" -> 3), else 0.\n" +
	"- \"deictic\" is true ONLY when the user pointed instead of naming (\"close this\", " +
	"\"expand that\"). Never set it together with a target.\n" +
	"- Do NOT invent a control name the user did not say. Do NOT guess what is on their screen: " +
	"you cannot see it, and something else resolves the target.\n" +
	"- One action only. If the request contains several, describe the FIRST one.\n" +
	"Output JSON only."

func interpretUser(input string, verbs []string) string {
	var b strings.Builder
	b.WriteString("Allowed verbs:\n")
	for _, v := range verbs {
		fmt.Fprintf(&b, "- %s\n", v)
	}
	fmt.Fprintf(&b, "\nUser asked: %q\n", input)
	b.WriteString("Reply with the JSON object, or NONE.")
	return b.String()
}

// interpret asks the model what one sentence meant. nil means no reading.
func interpret(ctx context.Context, cfg config, input string, verbs []string) *semantic {
	if strings.TrimSpace(input) == "" || len(verbs) == 0 {
		return nil
	}
	body, err := json.Marshal(chatReq{
		Model: cfg.Model,
		Messages: []message{
			{Role: "system", Content: interpretPrompt},
			{Role: "user", Content: interpretUser(input, verbs)},
		},
		Temperature: 0, // classification, not writing
		MaxTokens:   128,
		Stream:      false,
	})
	if err != nil {
		return nil
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.Key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.Key)
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var cr chatResp
	if json.NewDecoder(resp.Body).Decode(&cr) != nil || len(cr.Choices) == 0 {
		return nil
	}
	return parseSemantic(cr.Choices[0].Message.Content, verbs)
}

// parseSemantic extracts and sanitises the model's answer.
//
// # Why this cleans up rather than trusting
//
// A small local model does not reliably obey "reply with only JSON", and it will occasionally
// answer with a verb that is close to one in the list rather than one from it. So the reply is dug
// out of whatever prose surrounds it, the verb is snapped to the allowed list case-insensitively,
// and an answer that still does not fit is dropped.
//
// Marco re-validates every one of these against its own vocabulary — this is best-effort
// cleaning, never a boundary. A verb this let through that Marco cannot perform is refused there.
func parseSemantic(content string, verbs []string) *semantic {
	if strings.EqualFold(strings.Trim(strings.TrimSpace(content), "\"'`. \n\r"), "none") {
		return nil
	}
	raw := extractJSON(content)
	if raw == "" {
		return nil
	}
	var s semantic
	if json.Unmarshal([]byte(raw), &s) != nil {
		return nil
	}
	s.Kind = canonicalVerb(s.Kind, verbs)
	if s.Kind == "" {
		return nil
	}
	s.Target = strings.TrimSpace(s.Target)
	// A POINTED-AT TARGET AND A NAMED ONE ARE DIFFERENT CLAIMS, and a model asked not to set
	// both will sometimes set both anyway. The named one is kept, because it is the half the
	// user actually said.
	if s.Deictic && s.Target != "" {
		s.Deictic = false
	}
	if s.Ordinal < 0 {
		s.Ordinal = 0
	}
	return &s
}

// canonicalVerb returns the allowed verb matching name case-insensitively, or "".
func canonicalVerb(name string, verbs []string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	for _, v := range verbs {
		if strings.ToLower(v) == name {
			return v
		}
	}
	return ""
}
