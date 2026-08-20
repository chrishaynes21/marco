package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// serverReplying stands in for an OpenAI-compatible endpoint, returning reply as
// the assistant message and recording the request body it received.
func serverReplying(t *testing.T, reply string, gotBody *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if b, _ := io.ReadAll(r.Body); gotBody != nil {
			*gotBody = string(b)
		}
		json.NewEncoder(w).Encode(chatResp{Choices: []struct {
			Message message `json:"message"`
		}{{Message: message{Role: "assistant", Content: reply}}}})
	}))
}

func cfgFor(url string) config {
	return config{BaseURL: url, Model: "test", Timeout: 5 * time.Second}
}

func TestResolveExactSlug(t *testing.T) {
	var body string
	srv := serverReplying(t, "start-sea-of-thieves", &body)
	defer srv.Close()

	got := resolve(context.Background(), cfgFor(srv.URL),
		request{Input: "fire up the pirate game", Routes: []string{"start-sea-of-thieves", "open-chest"}})
	if got != "start-sea-of-thieves" {
		t.Fatalf("got %q, want start-sea-of-thieves", got)
	}
	// The prompt must carry the request and the candidate slugs.
	if !strings.Contains(body, "open-chest") || !strings.Contains(body, "pirate game") {
		t.Errorf("request body missing routes/input: %s", body)
	}
}

func TestResolveChattyReply(t *testing.T) {
	// A small model may wrap the answer in prose — we still extract the slug.
	srv := serverReplying(t, "Sure! You want: open-chest.", nil)
	defer srv.Close()
	got := resolve(context.Background(), cfgFor(srv.URL),
		request{Input: "crack the chest", Routes: []string{"start-sea-of-thieves", "open-chest"}})
	if got != "open-chest" {
		t.Fatalf("got %q, want open-chest", got)
	}
}

func TestResolveNoneIsEmpty(t *testing.T) {
	srv := serverReplying(t, "NONE", nil)
	defer srv.Close()
	if got := resolve(context.Background(), cfgFor(srv.URL),
		request{Input: "make me a sandwich", Routes: []string{"open-chest"}}); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestResolveUnknownSlugRejected(t *testing.T) {
	// A hallucinated slug that isn't in the list must not be returned.
	srv := serverReplying(t, "launch-rocket", nil)
	defer srv.Close()
	if got := resolve(context.Background(), cfgFor(srv.URL),
		request{Input: "go", Routes: []string{"open-chest"}}); got != "" {
		t.Fatalf("got %q, want empty (unknown slug)", got)
	}
}

func TestResolveServerDownIsEmpty(t *testing.T) {
	// Nothing listening → graceful empty, so the assistant just falls back.
	got := resolve(context.Background(), cfgFor("http://127.0.0.1:1"),
		request{Input: "go", Routes: []string{"open-chest"}})
	if got != "" {
		t.Fatalf("got %q, want empty when server is down", got)
	}
}

func TestResolveEmptyInputSkips(t *testing.T) {
	// Guard clauses: no routes or blank input never hit the network.
	if got := resolve(context.Background(), cfgFor("http://127.0.0.1:1"),
		request{Input: "", Routes: []string{"open-chest"}}); got != "" {
		t.Fatalf("blank input should yield empty, got %q", got)
	}
	if got := resolve(context.Background(), cfgFor("http://127.0.0.1:1"),
		request{Input: "go", Routes: nil}); got != "" {
		t.Fatalf("no routes should yield empty, got %q", got)
	}
}

// --- converse mode ----------------------------------------------------------

func converseVia(t *testing.T, reply string, routes []string) decision {
	t.Helper()
	srv := serverReplying(t, reply, nil)
	defer srv.Close()
	return converse(context.Background(), cfgFor(srv.URL),
		request{Mode: "converse", Input: "do the thing", Routes: routes, App: "Discord"})
}

func TestConverseTeach(t *testing.T) {
	d := converseVia(t, `{"intent":"teach","name":"mute discord","reply":"Show me how."}`, nil)
	if d.Intent != "teach" || d.Name != "mute discord" || d.Reply != "Show me how." {
		t.Fatalf("got %+v, want teach", d)
	}
}

func TestConverseRunCanonicalizesSlug(t *testing.T) {
	// The model echoes the slug with different casing; snap it to the real one.
	d := converseVia(t, `{"intent":"run","route":"Open-Chest","reply":"Opening."}`, []string{"open-chest"})
	if d.Intent != "run" || d.Route != "open-chest" {
		t.Fatalf("got %+v, want run open-chest", d)
	}
}

func TestConverseRunUnknownDowngradesToClarify(t *testing.T) {
	d := converseVia(t, `{"intent":"run","route":"ghost"}`, []string{"open-chest"})
	if d.Intent != "clarify" || d.Route != "" || d.Reply == "" {
		t.Fatalf("got %+v, want clarify with a reply and no route", d)
	}
}

func TestConverseExtractsWrappedJSON(t *testing.T) {
	// Small models often wrap the object in prose — we still parse it.
	d := converseVia(t, "Sure thing!\n{\"intent\":\"chat\",\"reply\":\"I automate clicks.\"}\nHope that helps.", nil)
	if d.Intent != "chat" || d.Reply != "I automate clicks." {
		t.Fatalf("got %+v, want chat", d)
	}
}

func TestConverseBadIntentIsEmpty(t *testing.T) {
	d := converseVia(t, `{"intent":"delete-everything","reply":"muahaha"}`, nil)
	if d.Intent != "" {
		t.Fatalf("got %+v, want empty intent (rejected)", d)
	}
}

func TestConverseServerDownIsEmpty(t *testing.T) {
	got := converse(context.Background(), cfgFor("http://127.0.0.1:1"),
		request{Mode: "converse", Input: "hi", Routes: []string{"open-chest"}})
	if got.Intent != "" {
		t.Fatalf("got %+v, want empty intent when server is down", got)
	}
}

func TestPickTokenBoundaries(t *testing.T) {
	routes := []string{"open-chest", "chest"}
	// "chest" must not match inside "open-chest"; exact wins.
	if got := pick("chest", routes); got != "chest" {
		t.Errorf("pick(chest) = %q, want chest", got)
	}
	if got := pick("open-chest", routes); got != "open-chest" {
		t.Errorf("pick(open-chest) = %q, want open-chest", got)
	}
	if got := pick("openchestx", routes); got != "" {
		t.Errorf("pick(openchestx) = %q, want empty", got)
	}
}
