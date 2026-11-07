package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// ProgramChoice is one open program offered to the question: a label the answer must copy, how
// its window names it, and its executable.
type ProgramChoice struct {
	Label string `json:"label"`
	Name  string `json:"name"`
	Exe   string `json:"exe"`
}

// MaxPrograms bounds the list: a desktop, not a vocabulary.
const MaxPrograms = 64

// ProgramTimeout is how long one question may take. A variable so a test can shorten it;
// production never reassigns it.
var ProgramTimeout = 20 * time.Second

// WhichProgram asks the plugin which of the open programs somebody meant ("focus chat cheap tea"
// for ChatGPT), for the built-in skill Focus app (internal/skills).
//
// The answer is a label that was SENT, or false: no plugin, one that does not know the question,
// NONE, a timeout, or a label nobody offered. Only the request's words and the program names go to
// the plugin, never a window's title beyond the application's own name.
//
// Accepting a label that was not offered must fail TestOnlyAnOfferedProgramIsAnswered.
func WhichProgram(ctx context.Context, said string, programs []ProgramChoice) (string, bool) {
	path := Path()
	if path == "" || strings.TrimSpace(said) == "" || len(programs) == 0 || len(programs) > MaxPrograms {
		return "", false
	}
	offered := map[string]bool{}
	for _, p := range programs {
		offered[p.Label] = true
	}
	reqBytes, err := json.Marshal(struct {
		Input   string `json:"input"`
		Program struct {
			Programs []ProgramChoice `json:"programs"`
		} `json:"program"`
	}{Input: said, Program: struct {
		Programs []ProgramChoice `json:"programs"`
	}{programs}})
	if err != nil {
		return "", false
	}
	ctx, cancel := context.WithTimeout(ctx, ProgramTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path)
	Quiet(cmd)
	cmd.Stdin = bytes.NewReader(append(reqBytes, '\n'))
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	var resp struct {
		Program string `json:"program"`
	}
	if json.Unmarshal(bytes.TrimSpace(out), &resp) != nil || !offered[resp.Program] {
		// A plugin that knows only routes answers {"route": ""} here, which is no answer.
		return "", false
	}
	return resp.Program, true
}
