package llm

import (
	"context"
	"strings"
	"testing"
)

func TestInertWithoutKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	if Available() {
		t.Fatal("Available() true with no key")
	}
	// No key → no client, no network, empty result.
	if got := Resolve(context.Background(), "fire up the pirate game", []string{"start-sea-of-thieves"}); got != "" {
		t.Fatalf("Resolve without key = %q, want \"\"", got)
	}
}

func TestResolveGuards(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-not-real")
	// Empty inputs short-circuit before any network call.
	if Resolve(context.Background(), "", []string{"a"}) != "" {
		t.Error("empty input should return \"\"")
	}
	if Resolve(context.Background(), "x", nil) != "" {
		t.Error("no routes should return \"\"")
	}
}

func TestPromptShape(t *testing.T) {
	p := prompt("fire up sea of thieves", []string{"start-sea-of-thieves", "open-chest"})
	for _, want := range []string{
		"start-sea-of-thieves", "open-chest",
		`"fire up sea of thieves"`, "NONE",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q:\n%s", want, p)
		}
	}
}
