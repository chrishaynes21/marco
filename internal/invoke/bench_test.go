package invoke_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// What the routing decision itself costs, separately from anything it decides to do.
//
// The phase was told to measure intake overhead and NOT to confuse it with foreground activation,
// Stage settling or perception — which dominate by orders of magnitude and are not this phase's
// business. This is the decision alone.
func BenchmarkDecideAnExactPlay(b *testing.B) {
	dir := b.TempDir()
	reg := routes.Registry{Dir: dir}
	for _, slug := range []string{"open-mouse-settings", "save-and-close", "enter-freeplay"} {
		if err := reg.Save(routes.Route{App: "settings", Focus: true, Slug: slug}, "script main...\n  do nothing.\n"); err != nil {
			b.Fatal(err)
		}
	}
	req := invoke.Request{Text: "Open Mouse Settings", Source: invoke.SourceSpoken, App: "discord"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if d := invoke.Decide(reg, req); d.Kind != invoke.KindPlay {
			b.Fatal(d.Kind)
		}
	}
}

func BenchmarkDecideAMiss(b *testing.B) {
	reg := routes.Registry{Dir: b.TempDir()}
	req := invoke.Request{Text: "turn bluetooth off", Source: invoke.SourceTyped}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if d := invoke.Decide(reg, req); d.Kind != invoke.KindDirector {
			b.Fatal(d.Kind)
		}
	}
}

func BenchmarkDecideAControlPhrase(b *testing.B) {
	reg := routes.Registry{Dir: b.TempDir()}
	req := invoke.Request{Text: "stop", Source: invoke.SourceSpoken}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if d := invoke.Decide(reg, req); d.Kind != invoke.KindControl {
			b.Fatal(d.Kind)
		}
	}
}
