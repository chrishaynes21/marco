package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// config is the runtime wiring, all from the environment so setup.ps1 (or the
// user) can point the plugin at any OpenAI-compatible endpoint without a rebuild.
type config struct {
	BaseURL string        // OpenAI-compatible base, e.g. http://localhost:11434/v1
	Model   string        // model tag, e.g. llama3.2:3b
	Key     string        // optional bearer token (OpenAI / authenticated servers)
	Timeout time.Duration // hard cap so a cold/slow model can't hang the assistant
}

func loadConfig() config {
	c := config{
		BaseURL: env("MARCO_LLM_URL", "http://localhost:11434/v1"),
		Model:   env("MARCO_LLM_MODEL", "llama3.2:3b"),
		Key:     os.Getenv("MARCO_LLM_KEY"),
		Timeout: 20 * time.Second,
	}
	// A local model's FIRST call pays a load cost; allow a generous, tunable cap.
	if ms, err := strconv.Atoi(os.Getenv("MARCO_LLM_TIMEOUT_MS")); err == nil && ms > 0 {
		c.Timeout = time.Duration(ms) * time.Millisecond
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	return c
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// --- OpenAI-compatible Chat Completions shapes (only the fields we use) -------

type chatReq struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
	Stream      bool      `json:"stream"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResp struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

// resolve asks the model to map req.Input to one of req.Routes and returns the
// matching slug, or "" when nothing is configured, nothing fits, or any error
// occurs — so the caller always degrades gracefully to its offline matcher.
func resolve(ctx context.Context, cfg config, req request) string {
	if len(req.Routes) == 0 || strings.TrimSpace(req.Input) == "" {
		return ""
	}
	body, err := json.Marshal(chatReq{
		Model: cfg.Model,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt(req)},
		},
		Temperature: 0, // deterministic classification, not creative writing
		MaxTokens:   32,
		Stream:      false,
	})
	if err != nil {
		return ""
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
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
	return pick(cr.Choices[0].Message.Content, req.Routes)
}

const systemPrompt = "You map a user's spoken request to ONE of their saved automation routes. " +
	"Reply with ONLY the single best-matching route slug, copied exactly, and nothing else. " +
	"If none clearly fits, reply exactly NONE."

func userPrompt(req request) string {
	var b strings.Builder
	b.WriteString("Saved routes (slugs):\n")
	for _, r := range req.Routes {
		fmt.Fprintf(&b, "- %s\n", r)
	}
	fmt.Fprintf(&b, "\nRequest: %q\n", req.Input)
	b.WriteString("Answer with one slug from the list above, or NONE.")
	return b.String()
}

// pick extracts a valid route slug from the model's free-text answer. Small local
// models don't always obey "reply with only the slug", so we clean the reply and
// accept it only if it exactly equals — or contains as a whole token — a known
// route. Anything else (including NONE) yields "", which the engine treats as no
// match. The engine re-verifies too, so a wrong slug can never run.
func pick(answer string, routes []string) string {
	a := strings.ToLower(strings.TrimSpace(answer))
	a = strings.Trim(a, "\"'`.\n\r ")
	if a == "" || a == "none" {
		return ""
	}
	// Exact match first (the well-behaved case).
	for _, r := range routes {
		if a == strings.ToLower(r) {
			return r
		}
	}
	// Fallback: the slug appears as a standalone token in a chattier reply
	// ("You want: start-sea-of-thieves"). Require word boundaries so a slug can't
	// match inside a larger word.
	for _, r := range routes {
		if containsToken(a, strings.ToLower(r)) {
			return r
		}
	}
	return ""
}

// --- converse mode: intent classification for dispatch -------------------

const conversePrompt = "You are Marco, a friendly assistant that automates clicks and keystrokes. " +
	"Decide what the user wants and reply with ONE JSON object and nothing else:\n" +
	`{"intent":"run|teach|chat|clarify","route":"","name":"","reply":""}` + "\n" +
	"Rules:\n" +
	"- run: they want to run one of their saved routes. Put its exact slug in \"route\" and a short confirmation in \"reply\".\n" +
	"- teach: they want to CREATE a new command (\"make/teach/record a command that ...\"). Put a short name in \"name\" and ask them to demonstrate it in \"reply\".\n" +
	"- chat: a greeting or a question about Marco. Answer in \"reply\".\n" +
	"- clarify: you are unsure which route they mean. Ask a short question in \"reply\".\n" +
	"Only ever use a slug from the provided list. Keep \"reply\" to one short sentence. Output JSON only."

func converseUser(req request) string {
	var b strings.Builder
	b.WriteString("Saved routes (slugs):\n")
	if len(req.Routes) == 0 {
		b.WriteString("(none yet)\n")
	}
	for _, r := range req.Routes {
		fmt.Fprintf(&b, "- %s\n", r)
	}
	if req.App != "" {
		fmt.Fprintf(&b, "Foreground app: %s\n", req.App)
	}
	fmt.Fprintf(&b, "User said: %q\n", req.Input)
	return b.String()
}

// converse asks the model to classify intent and returns a decision. Any failure
// yields an empty-intent decision, which dispatch reads as "no proposal" and
// falls back to its deterministic policy.
func converse(ctx context.Context, cfg config, req request) decision {
	if strings.TrimSpace(req.Input) == "" {
		return decision{}
	}
	body, err := json.Marshal(chatReq{
		Model: cfg.Model,
		Messages: []message{
			{Role: "system", Content: conversePrompt},
			{Role: "user", Content: converseUser(req)},
		},
		Temperature: 0,
		MaxTokens:   256, // room for a JSON object with a one-sentence reply
		Stream:      false,
	})
	if err != nil {
		return decision{}
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return decision{}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cfg.Key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.Key)
	}
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return decision{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return decision{}
	}
	var cr chatResp
	if json.NewDecoder(resp.Body).Decode(&cr) != nil || len(cr.Choices) == 0 {
		return decision{}
	}
	return parseDecision(cr.Choices[0].Message.Content, req.Routes)
}

// parseDecision extracts the JSON object a small model may wrap in prose, then
// sanitizes it: the intent must be one of the four we asked for, and a `run` route
// is snapped to a real slug (case-insensitively) or the intent is downgraded. The
// dispatch re-validates too, so this is best-effort, not a security boundary.
func parseDecision(content string, routes []string) decision {
	raw := extractJSON(content)
	if raw == "" {
		return decision{}
	}
	var d decision
	if json.Unmarshal([]byte(raw), &d) != nil {
		return decision{}
	}
	d.Intent = strings.ToLower(strings.TrimSpace(d.Intent))
	d.Route = strings.TrimSpace(d.Route)
	d.Name = strings.TrimSpace(d.Name)
	d.Reply = strings.TrimSpace(d.Reply)
	switch d.Intent {
	case "run":
		if slug := canonicalSlug(d.Route, routes); slug != "" {
			d.Route = slug
		} else {
			// Named a route we don't have — ask instead of guessing.
			d.Intent = "clarify"
			d.Route = ""
			if d.Reply == "" {
				d.Reply = "I'm not sure which command you mean — can you say it another way?"
			}
		}
	case "teach", "chat", "clarify":
		// keep
	default:
		return decision{}
	}
	return d
}

// extractJSON returns the substring from the first '{' to the last '}', or "".
func extractJSON(s string) string {
	i := strings.IndexByte(s, '{')
	j := strings.LastIndexByte(s, '}')
	if i < 0 || j < i {
		return ""
	}
	return s[i : j+1]
}

// canonicalSlug returns the route slug matching name case-insensitively, or "".
func canonicalSlug(name string, routes []string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	for _, r := range routes {
		if strings.ToLower(r) == name {
			return r
		}
	}
	return ""
}

// containsToken reports whether needle appears in haystack bounded by non-slug
// characters (slug chars being [a-z0-9-]).
func containsToken(haystack, needle string) bool {
	from := 0
	for {
		i := strings.Index(haystack[from:], needle)
		if i < 0 {
			return false
		}
		i += from
		if isBoundary(haystack, i-1) && isBoundary(haystack, i+len(needle)) {
			return true
		}
		from = i + 1
	}
}

func isBoundary(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return true
	}
	c := s[i]
	return !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-')
}
