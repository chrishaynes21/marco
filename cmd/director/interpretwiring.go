package main

import (
	"context"

	"github.com/chaynes-simpleclouds/marco/internal/director/interpret"
	"github.com/chaynes-simpleclouds/marco/internal/resolver"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Where the model is allowed to help, and the one line that says so.
//
// # Why this is its own file
//
// Because "is a model involved in deciding what to do to somebody's computer" is the kind of
// question that should be answerable by reading one short file, and because the wiring is the only
// place the answer lives. `internal/director/interpret` states what an interpretation may contain;
// this states where one is consulted from. Between them there is nowhere else to look.
//
// # The whole of what is wired
//
// One optional function on the pipeline, consulted only after the deterministic parser returned
// `IntentUnknown`, producing one typed operation from a closed vocabulary. No tool loop, no second
// turn, no access to the screen, no authority, and no path to the desktop — see the package note
// on `interpret` for why each of those is a shape rather than a rule.

// interpretIntent is the interpreter the production pipeline is given, or nil.
//
// Nil when no plugin is configured, which is the default. A nil field is how a Director without a
// model behaves exactly as it did before this existed — and it is also how `director status` can
// say "no interpreter is configured" rather than reporting that Marco did not understand.
func interpretIntent() func(context.Context, string) (directorapi.Intent, bool) {
	in := interpret.New()
	if in == nil {
		return nil
	}
	return in.Intent
}

// interpreterConfigured reports whether a model-backed reading is available, for diagnostics.
//
// It asks the same question the wiring above asks, through the same package, so a status line
// cannot claim an interpreter the pipeline does not have. A second check against
// `$MARCO_RESOLVER` here would be a second answer to one fact.
func interpreterConfigured() bool { return interpret.New() != nil }

// interpreterPath is which plugin would be consulted, for diagnostics. Empty when none is.
//
// The PATH, which is a thing the person set, and never a key, a model name or a prompt.
func interpreterPath() string { return resolver.Path() }
