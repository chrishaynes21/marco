package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// THE SCREEN'S QUESTIONS KEEP THEIR MODEL; THE LIBRARY'S GET THE ONE THAT FOUND THE VALUE; A
// NAMED MODEL IS USED FOR BOTH. Changing the interpreter's default in passing must fail this
// (Experiment-072), and so must losing the override.
func TestTheDefaultModelDependsOnTheQuestion(t *testing.T) {
	t.Setenv("MARCO_LLM_MODEL", "")
	cfg := loadConfig()
	if cfg.Model != "llama3.2:3b" {
		t.Errorf("the screen's questions default to %q", cfg.Model)
	}
	if m := cfg.forRoutines().Model; m != "llama3.1:8b" {
		t.Errorf("the library's questions default to %q", m)
	}
	t.Setenv("MARCO_LLM_MODEL", "qwen2.5:7b")
	cfg = loadConfig()
	if cfg.Model != "qwen2.5:7b" || cfg.forRoutines().Model != "qwen2.5:7b" {
		t.Errorf("a named model is not used for both: %q, %q", cfg.Model, cfg.forRoutines().Model)
	}
}

// THE VALUE QUESTION NAMES THE ROUTINE, and asks for the words naming it first, so a misheard
// name is not copied into the value. Without a routine it is the question it always was.
func TestTheValueQuestionNamesTheRoutine(t *testing.T) {
	got := extractUser("cloud paul stone", "cloud pull", []string{"resource"})
	for _, want := range []string{`The request is for the routine "cloud pull"`, `"routine_words"`, "- resource"} {
		if !strings.Contains(got, want) {
			t.Errorf("the question lacks %q:\n%s", want, got)
		}
	}
	if plain := extractUser("cloud pull stone", "", []string{"resource"}); strings.Contains(plain, "routine") {
		t.Errorf("with no routine the question still mentions one:\n%s", plain)
	}
}

// AND EACH QUESTION IS ASKED WITH ITS MODEL, through the plugin's own routing: a fake model server
// records the model every request names. Asking the library's questions with the screen's model
// again, or the screen's with the library's, must fail this.
func TestEachQuestionIsAskedWithItsModel(t *testing.T) {
	t.Setenv("MARCO_LLM_MODEL", "")
	var models []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		models = append(models, body.Model)
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"NONE"}}]}`)
	}))
	defer srv.Close()
	t.Setenv("MARCO_LLM_URL", srv.URL)
	cfg := loadConfig()
	for _, c := range []struct {
		req  request
		want string
	}{
		{request{Input: "cloud paul stone", Extract: &extractRequest{Names: []string{"resource"}, Routine: "cloud pull"}}, "llama3.1:8b"},
		{request{Input: "cloud paul stone", Routes: []string{"cloud-pull"}}, "llama3.1:8b"},
		{request{Input: "turn it up", Interpret: &interpretRequest{Verbs: []string{"press"}}}, "llama3.2:3b"},
	} {
		models = nil
		answer(context.Background(), cfg, c.req)
		if len(models) == 0 || models[0] != c.want {
			t.Errorf("%+v was asked with %q, want %q", c.req, models, c.want)
		}
	}
}
