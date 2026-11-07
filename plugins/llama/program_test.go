package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

var open = []programChoice{
	{Label: "chatgpt", Name: "ChatGPT", Exe: "chatgpt"},
	{Label: "steam", Name: "Steam", Exe: "steamwebhelper"},
}

// THE ANSWER IS ONE OF THE PROGRAMS SENT, or nothing: a model naming a program that is not open,
// or NONE, chooses nothing.
func TestTheProgramAnswerIsOneThatIsOpen(t *testing.T) {
	var body string
	srv := serverReplying(t, "chatgpt", &body)
	got := program(context.Background(), cfgFor(srv.URL), "chat cheap tea", open, nil)
	srv.Close()
	if got != "chatgpt" {
		t.Errorf("got %q", got)
	}
	if !strings.Contains(body, "steamwebhelper.exe") || !strings.Contains(body, "chat cheap tea") {
		t.Errorf("the question did not carry the programs and what was said: %s", body)
	}
	for _, reply := range []string{"NONE", "spotify", ""} {
		srv := serverReplying(t, reply, nil)
		if got := program(context.Background(), cfgFor(srv.URL), "music", open, nil); got != "" {
			t.Errorf("reply %q chose %q", reply, got)
		}
		srv.Close()
	}
}

// THE QUESTION IS SELECTED BY ITS FIELD, and answered as {"program": …}.
func TestTheProgramQuestionIsAnswered(t *testing.T) {
	srv := serverReplying(t, "steam", nil)
	defer srv.Close()
	r, w, _ := os.Pipe()
	saved := os.Stdout
	os.Stdout = w
	answer(context.Background(), cfgFor(srv.URL), request{Input: "the game launcher",
		Program: &programRequest{Programs: open}})
	os.Stdout = saved
	w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	var got struct{ Program string }
	if json.Unmarshal(buf.Bytes(), &got) != nil || got.Program != "steam" {
		t.Errorf("answered %q", buf.String())
	}
}
