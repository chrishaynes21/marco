// Package activate is the one answer to "press this control".
//
// # Why this package exists
//
// Because there were two answers, and they had drifted.
//
// A control is activated from two places: a live rehearsal, which the Director drives while
// somebody watches, and a saved play, which the Theater performs later. Both mean the same thing
// — make this control do its thing — and both had their own implementation.
//
// One of them knew that a Windows Settings navigation item is a SELECTION item and not a button,
// and would fall back to Select. The other asked for Invoke and gave up. That difference was
// found by four failed live runs, fixed in the rehearsal path, and left unfixed on the path that
// runs saved plays — so a route somebody had successfully learned would have failed the first time
// it ran on its own, for a reason already understood and already solved.
//
// # What is shared, and what is not
//
// The two callers reach a control through different layers: a rehearsal compiles a
// `marcoexec.Operation` and runs it through Marco; the Theater's Actor asks the Accessibility act
// directly. Sharing the CALL would mean forcing one of them through the other's machinery.
//
// What they must share is the semantics: which ways there are, in what order, and the rule that
// only a capability REFUSING for lack of the pattern licenses trying the next one. That is what
// lives here. Each caller supplies its own Attempt.
//
// # This package holds no policy about who may act
//
// It presses what it is given. Authority, scope, foreground safety and one-attempt bounds all
// belong to the caller, and nothing here can be used to get around them.
package activate

import (
	"context"
	"fmt"
	"strings"
)

// Way is one way a control can be made to do its thing.
//
// The values are the Accessibility act's own capability names, because that is what both callers
// end up asking for and a second vocabulary would need translating twice.
type Way string

const (
	// Invoke is a button doing its one thing.
	Invoke Way = "Invoke"
	// Select is a list or navigation item becoming the chosen one.
	//
	// THE one that mattered live. Windows Settings is built almost entirely from selection
	// items, so a ladder without this fails every navigation step on the most obvious
	// application anybody would demonstrate something in.
	Select Way = "Select"
	// Expand is a disclosure opening.
	Expand Way = "Expand"
	// Toggle is a checkbox or switch changing state.
	Toggle Way = "Toggle"
)

// Ladder is every way to activate a control, in the order they are tried.
//
// Ordered by how specific each pattern is about what pressing MEANS. Toggle is last and
// deliberately so: it is the only one whose effect depends on what the control was already doing,
// so trying it earlier could switch something off that somebody meant to press — and the
// demonstration would be reproduced backwards.
//
// Changing this order changes what Marco does on every machine. It is one list on purpose.
var Ladder = []Way{Invoke, Select, Expand, Toggle}

// Attempt performs one way and says whether the control simply lacks that pattern.
//
// `unsupported` is the load-bearing return. "This control has no such pattern" invites another
// way; "this control has the pattern and it went wrong" does not, and retrying that under a
// different name would be Marco pressing a control repeatedly in different ways until something
// happened — which is exactly what an authorised attempt must never do.
type Attempt func(ctx context.Context, w Way) (unsupported bool, err error)

// Activate presses a control the way that control can be pressed.
//
// Returns which way worked. An error means every way was refused, or one of them was implemented
// and failed — the two are distinguished by the caller's own Attempt, and only the first walks the
// whole ladder.
//
// Deleting the unsupported check — continuing on any failure — must fail
// TestARealFailureStopsTheLadder.
func Activate(ctx context.Context, attempt Attempt) (Way, error) {
	if attempt == nil {
		return "", fmt.Errorf("nothing to activate with")
	}
	var last error
	for _, w := range Ladder {
		unsupported, err := attempt(ctx, w)
		if err == nil {
			return w, nil
		}
		if !unsupported {
			// Implemented, and it went wrong. That is a real failure and the attempt
			// stops here.
			return "", err
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("this control cannot be activated")
	}
	return "", last
}

// Unsupported reports whether a host's message means "this control lacks that pattern".
//
// The `unsupported:` prefix is the accessibility host's own contract — its comment says it exists
// "so the Director can fall back and RECORD that it did" — and recognising it is the one thing
// both callers must agree on. A second copy of this check is a second thing to keep in agreement
// with a C# file.
func Unsupported(message string) bool {
	return strings.Contains(message, "unsupported:")
}
