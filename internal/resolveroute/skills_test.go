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
		t.Errorf("a routine was answered as a skill: %+v", a)
	}
}
