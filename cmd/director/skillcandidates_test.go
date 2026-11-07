package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// THE BUILT-IN SKILLS ARE OFFERED ONLY TO A CALLER THAT ASKED FOR THEM, and an answer naming one
// says it is one. Studio does not ask: its Play button runs routines, and a skill is not one.
//
// Not appending them, or appending them for everybody, must fail this.
func TestASkillIsOfferedOnlyToACallerThatAsked(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", "")
	_ = aLibraryHere(t)
	r := &Runtime{}
	without := r.ResolveRoute(service.ResolveRoutePayload{Request: "focus app", Target: "arkascended"})
	with := r.ResolveRoute(service.ResolveRoutePayload{Request: "focus app", Target: "arkascended",
		Skills: true})
	if without.Answer.Builtin || without.Answer.ID == "builtin/focus-app" {
		t.Errorf("a caller that did not ask was answered with a skill: %+v", without.Answer)
	}
	if with.Answer.Offered != without.Answer.Offered+2 {
		t.Errorf("offered %d with the skills and %d without", with.Answer.Offered, without.Answer.Offered)
	}
	if !with.Answer.Builtin || with.Answer.ID != "builtin/focus-app" || with.Answer.Line != "focus app" {
		t.Errorf("the skill was not answered as one: %+v", with.Answer)
	}
}
