package resolver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var openPrograms = []ProgramChoice{{Label: "chatgpt", Name: "ChatGPT", Exe: "chatgpt"},
	{Label: "steam", Name: "Steam", Exe: "steamwebhelper"}}

// ONLY A PROGRAM THAT WAS OFFERED IS AN ANSWER, and every way of not answering is false.
func TestOnlyAnOfferedProgramIsAnswered(t *testing.T) {
	anExtractingPlugin(t)
	saw := filepath.Join(t.TempDir(), "saw.json")
	t.Setenv("FAKE_SAW", saw)
	replying(t, `{"program":"chatgpt"}`)
	if got, ok := WhichProgram(context.Background(), "chat cheap tea", openPrograms); !ok || got != "chatgpt" {
		t.Errorf("got %q %v", got, ok)
	}
	b, _ := os.ReadFile(saw)
	if !strings.Contains(string(b), `"program":{"programs":[`) || !strings.Contains(string(b), "chat cheap tea") {
		t.Errorf("the plugin was sent %s", b)
	}
	for _, reply := range []string{`{"program":"spotify"}`, `{"program":""}`, `{"route":"chatgpt"}`, `garbage`} {
		replying(t, reply)
		if got, ok := WhichProgram(context.Background(), "music", openPrograms); ok {
			t.Errorf("reply %s answered %q", reply, got)
		}
	}
	t.Setenv("MARCO_RESOLVER", "")
	if _, ok := WhichProgram(context.Background(), "chat", openPrograms); ok {
		t.Error("answered with no plugin")
	}
}
