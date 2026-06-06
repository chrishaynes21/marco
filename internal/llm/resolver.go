// Package llm is the optional model-backed resolver: when the deterministic
// matcher (internal/nlu) is unsure, Claude Haiku maps a loosely-phrased request
// to one of the user's saved routes. It is inert without an ANTHROPIC_API_KEY —
// no key means no client and no network call, so the assistant works fully
// offline by default and only gets smarter when a key is present.
package llm

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// Available reports whether a Claude API key is configured.
func Available() bool { return os.Getenv("ANTHROPIC_API_KEY") != "" }

// Resolve asks Claude Haiku which saved route best matches input. It returns the
// matching route slug, or "" when nothing fits, the key is unset, or any error
// occurs — so the caller always degrades gracefully to teaching a new route.
func Resolve(ctx context.Context, input string, routes []string) string {
	if !Available() || len(routes) == 0 || strings.TrimSpace(input) == "" {
		return ""
	}
	client := anthropic.NewClient() // reads ANTHROPIC_API_KEY
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5_20251001,
		MaxTokens: 32,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt(input, routes))),
		},
	})
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(t.Text)
		}
	}
	ans := strings.ToLower(strings.TrimSpace(b.String()))
	for _, r := range routes {
		if ans == r {
			return r
		}
	}
	return ""
}

func prompt(input string, routes []string) string {
	var b strings.Builder
	b.WriteString("You map a user's request to one of their saved automation routes.\n")
	b.WriteString("Available routes (slugs):\n")
	for _, r := range routes {
		fmt.Fprintf(&b, "- %s\n", r)
	}
	fmt.Fprintf(&b, "Request: %q\n", input)
	b.WriteString("Reply with ONLY the single best-matching route slug, exactly as written above, " +
		"or NONE if none clearly fits. No other text.")
	return b.String()
}
