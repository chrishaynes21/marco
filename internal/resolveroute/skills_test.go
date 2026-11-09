package resolveroute

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/resolver"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// A BUILT-IN SKILL THE RESOLVER NAMES IS SAID TO BE ONE, WITH ITS EXPLICIT LINE: "switch me back
// to Discord" becomes `focus Discord` for the command line to read back, and no surface can take
// it for a routine. Dropping Builtin or Line from the answer must fail this.
func TestASkillTheResolverNamesIsAnsweredAsTheSkillsLine(t *testing.T) {
	const said = "switch me back to Discord"
	skill := Candidate{Route: routes.Route{Slug: "focus-app"}, ID: "builtin/focus-app",
		Name: "Focus app", Scope: "built in", Runnable: true, Builtin: true,
		Takes: []routes.Argument{{Name: "app"}}}
	routine := aCandidate("ark-graphics", "arkascended", true)
	e := &recordingExtractor{ok: true, spans: []resolver.Span{{Name: "app", Start: 18, End: 25}}}
	r := Resolver{Reg: routes.Registry{Dir: t.TempDir()},
		Advisor: &recordingAdvisor{ok: true, offers: dispatch.Decision{Intent: dispatch.IntentRun,
			Route: "focus-app", Source: dispatch.SourceAdvisor}},
		Extract: e.extract}
	a := r.Resolve(context.Background(), said, []Candidate{routine, skill}, at("arkascended"))
	if a.Kind != dispatch.IntentRun || a.ID != "builtin/focus-app" || !a.Builtin {
		t.Fatalf("the skill was not answered as one: %+v", a)
	}
	if a.Line != "focus Discord" || a.SourceAt != "" {
		t.Errorf("line %q, version %q; want the explicit line and no version", a.Line, a.SourceAt)
	}

	// AND A ROUTINE IS NEVER ANSWERED AS A SKILL.
	r.Advisor = &recordingAdvisor{ok: true, offers: dispatch.Decision{Intent: dispatch.IntentRun,
		Route: "ark-graphics", Source: dispatch.SourceAdvisor}}
	if a := r.Resolve(context.Background(), "graphics", []Candidate{routine, skill},
		at("arkascended")); a.Builtin || a.Line != "" {
		t.Errorf("a play was answered as a skill: %+v", a)
	}
}

// A WEB SEARCH THE RESOLVER CHOSE FOR "search …" SEARCHES FOR WHAT WAS TYPED: the owner's
// "search broken titan builds" searched for "titan builds", the model reading "search broken" as a
// mishearing of the skill's name. The words after "search" or "search for" are the query; a line
// that begins otherwise keeps the model's reading. See ADR-343.
func TestASearchTheResolverChoseIsForTheWordsAsTyped(t *testing.T) {
	search := Candidate{Route: routes.Route{Slug: "search-the-web"}, ID: "builtin/search-the-web",
		Name: "Search the web", Scope: "built in", Runnable: true, Builtin: true,
		Takes: []routes.Argument{{Name: "query"}}}
	for _, c := range []struct {
		said       string
		start, end int // what the model read as the query
		line       string
	}{
		{"search broken titan builds", 14, 26, "search the web for broken titan builds"},
		{"Search for Broken Titan builds", 18, 30, "search the web for Broken Titan builds"},
		{"look up titan builds on google", 8, 20, "search the web for titan builds"},
		{"search for", 0, 0, "search the web"},
	} {
		e := &recordingExtractor{ok: true}
		if c.end > 0 {
			e.spans = []resolver.Span{{Name: "query", Start: c.start, End: c.end}}
		}
		r := Resolver{Reg: routes.Registry{Dir: t.TempDir()},
			Advisor: &recordingAdvisor{ok: true, offers: dispatch.Decision{Intent: dispatch.IntentRun,
				Route: "search-the-web", Source: dispatch.SourceAdvisor}},
			Extract: e.extract}
		a := r.Resolve(context.Background(), c.said, []Candidate{search}, at("destiny2"))
		if a.Line != c.line {
			t.Errorf("%q is reviewed as %q, want %q", c.said, a.Line, c.line)
		}
	}
}

// AND ONLY A SEARCH: a line beginning "search" that the resolver answered with another skill keeps
// the model's reading of its value.
func TestOnlyASearchTakesTheWordsAfterSearch(t *testing.T) {
	focus := Candidate{Route: routes.Route{Slug: "focus-app"}, ID: "builtin/focus-app",
		Name: "Focus app", Scope: "built in", Runnable: true, Builtin: true,
		Takes: []routes.Argument{{Name: "app"}}}
	const said = "search for my discord window"
	e := &recordingExtractor{ok: true, spans: []resolver.Span{{Name: "app", Start: 14, End: 21}}}
	r := Resolver{Reg: routes.Registry{Dir: t.TempDir()},
		Advisor: &recordingAdvisor{ok: true, offers: dispatch.Decision{Intent: dispatch.IntentRun,
			Route: "focus-app", Source: dispatch.SourceAdvisor}},
		Extract: e.extract}
	if a := r.Resolve(context.Background(), said, []Candidate{focus}, at("destiny2")); a.Line != "focus discord" {
		t.Errorf("%q is reviewed as %q, want focus discord", said, a.Line)
	}
}
