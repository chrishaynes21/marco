package main

// WHAT A CONFIRMATION MEANS WHEN TWO ROUTINES ANSWER TO ONE NAME.
//
// # The defect this is about
//
// The adapters reduced Director's answer to its SLUG. The REPL then confirmed that slug and sent
// it through the door as words — and the door resolves words again, by the registry's own
// priority order. With `settings` saved for two applications, or under two scopes of one, the
// routine performed is whichever the second lookup lands on and not the one Director proposed and
// the person agreed to. The comment there said the whole identity was in the JSON for anything
// that needed it; it was not.
//
// So the identity travels, and what is held here is that it arrives: the exact invocation a
// confirmation produces, against a library with namesakes in it.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/resolveroute"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// twoRoutinesCalledSettings writes namesakes into a library and points this process at it.
//
// Three of them, which is the shape that makes the old behaviour wrong in two different ways: two
// applications with one of each, and one application with the name under two scopes.
func twoRoutinesCalledSettings(t *testing.T) routes.Registry {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	reg := routes.Registry{Dir: dir}
	for _, rt := range []routes.Route{
		{App: "arkascended", Slug: "settings"},
		{App: "arkascended", Focus: true, Slug: "settings"},
		{App: "discord", Focus: true, Slug: "settings"},
	} {
		src := "// Auto-generated.\n// Generation: 1.\nuse os.\n\nthe A is an actor.\n\n" +
			"this can Run.\nthis's Run does...\n    do OS's Key with \"a\".\n" +
			"    this is ok!\n\nthe App is a script.\n\ndo A's Run...\n" +
			"    when ok?\n        log \"done\".\n    or?\n        log that's error.\n"
		if err := reg.Save(rt, src); err != nil {
			t.Fatal(err)
		}
	}
	return reg
}

// AND THE CONFIRMED INVOCATION IS THE ONE THAT WAS PROPOSED.
//
// Through the two steps the adapter actually takes: Director's answer becomes a proposal, and the
// proposal becomes a request. Both halves matter — dropping the identity in either one produces a
// request that gets resolved again.
func TestAConfirmedProposalInvokesTheExactRouteDirectorNamed(t *testing.T) {
	twoRoutinesCalledSettings(t)

	for _, want := range []struct {
		say string
		id  string
		rt  routes.Route
	}{
		{"the in-place one in Ark", "arkascended/context/settings",
			routes.Route{App: "arkascended", Slug: "settings"}},
		{"the one that brings Ark forward", "arkascended/focus/settings",
			routes.Route{App: "arkascended", Focus: true, Slug: "settings"}},
		{"the one in Discord", "discord/focus/settings",
			routes.Route{App: "discord", Focus: true, Slug: "settings"}},
	} {
		// DIRECTOR'S ANSWER, with every part of the identity in it.
		a := resolveroute.Answer{Kind: "run", ID: want.id, App: want.rt.App,
			Slug: want.rt.Slug, Focus: want.rt.Focus, Name: "settings",
			Scope: string(orchestrator.ScopeAlreadyOf(want.rt))}
		p := proposalOf(a)

		if p.Route != want.rt {
			t.Errorf("%s: the proposal is for %+v, want %+v", want.say, p.Route,
				want.rt)
		}
		if p.ID != want.id {
			t.Errorf("%s: the proposal's identity is %q", want.say, p.ID)
		}
		// AND THE REQUEST IT BECOMES CARRIES THE ROUTE AND NOT THE WORDS, which is
		// what reaches the intake's explicit-identity arm where nothing is looked
		// up. Through the production function rather than a copy of it here: a copy
		// would pass while the real one sent words.
		req := requestFor(p)
		if req.Play == nil || *req.Play != want.rt {
			t.Errorf("%s: the request names %+v", want.say, req.Play)
		}
		if strings.TrimSpace(req.Text) != "" {
			t.Errorf("%s: the request carries words as well, which the door would "+
				"resolve: %q", want.say, req.Text)
		}
	}
}

// AND THE DOOR PERFORMS THE ROUTE IT WAS GIVEN, NOT THE ONE A NAME RESOLVES TO.
//
// The other half, and the one that shows why the slug alone was wrong: the registry's priority
// order answers "settings" with the in-place routine for the foreground application, so sending
// the WORDS would perform that one whichever of the three was proposed.
func TestResolvingTheNameAgainLandsSomewhereElse(t *testing.T) {
	reg := twoRoutinesCalledSettings(t)

	// WHAT THE WORDS RESOLVE TO, with Ark in front: the in-place one.
	byWords, ok := reg.Resolve("arkascended", "settings")
	if !ok {
		t.Fatal("the library does not answer to the name at all")
	}
	// AND THAT IS NOT WHAT DIRECTOR WOULD HAVE PROPOSED for two of the three, which is
	// the whole defect: a confirmation of either of those used to perform this instead.
	for _, other := range []routes.Route{
		{App: "arkascended", Focus: true, Slug: "settings"},
		{App: "discord", Focus: true, Slug: "settings"},
	} {
		if byWords == other {
			t.Fatalf("the name resolves to %+v, so this test cannot show the "+
				"difference", byWords)
		}
		// AND THE EXPLICIT IDENTITY REACHES EXACTLY THAT ROUTE. `Decide`'s
		// explicit-identity arm is what the confirmed proposal now takes.
		pinned := other
		d := invoke.Decide(reg, nil, invoke.Request{Play: &pinned,
			App: "arkascended", Source: invoke.SourceCLI})
		if d.Kind != invoke.KindPlay {
			t.Fatalf("an explicit identity was not routed as a play: %+v", d)
		}
		if d.Play != other {
			t.Errorf("the door would perform %+v, want the %+v that was agreed to",
				d.Play, other)
		}
		if !d.Explicit {
			t.Errorf("the door did not treat %+v as an identity it was given", other)
		}
	}
}
