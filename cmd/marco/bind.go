package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// runBind: `marco bind <key> "<play>"` — bind the leader hotkey `<key> to a
// play, scoped to the current foreground app. So `s in Notepad can run "say
// hello" while doing nothing elsewhere.
func runBind(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, `usage: marco bind <key> "<play>"`)
		os.Exit(2)
	}
	if err := bindKey(newDeps(), winctx.Active(), strings.ToLower(args[0]),
		strings.TrimSpace(strings.Join(args[1:], " "))); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// bindKey is the whole of what binding does, with the two things a test cannot have — the real
// registry and the real foreground window — handed in.
//
// Split out for the same reason `pressHotkey` is: a test that re-implements the validation is
// testing its own copy. The scope a binding LANDS in and the scope a press RESOLVES in have to be
// the same one, and nothing could see them diverge while both lived behind an os.Exit.
func bindKey(d orchestrator.Deps, app, key, cmd string) error {
	// A binding can chain steps with " then " ("say hi then wave"). Validate that
	// every step resolves to an existing play, so a typo can't be bound, then store
	// the raw command — each step is resolved + its args applied at run time.
	//
	// VALIDATED THE WAY IT WILL BE RESOLVED, which it was not.
	//
	// Binding used to accept a step on a 0.6 FUZZY score and then store the raw words, while
	// pressing the key resolves them EXACTLY. So `marco bind e "enter freeplayy"` scored 0.93,
	// was accepted, was reported as bound — and then did nothing at all, forever, because the
	// press could not resolve it and a hotkey skips a step it cannot resolve rather than
	// dropping into learn. A binding that reports success and can never fire is worse than one
	// that refuses.
	//
	// Deleting the exact check must fail TestABindingIsValidatedTheWayItWillBeResolved.
	for _, step := range routes.SplitChain(cmd) {
		base, _, _ := routes.ParseInvocation(step)
		if _, ok := d.Reg.Resolve(app, base); !ok {
			// "no play matches " is the engine's one unknown-command prefix, shared with
			// panicstop.go and PREFIX-MATCHED by plugins/overlay (acts.go, const
			// noPlayMatches). Changing the wording on one side only breaks the overlay's
			// offer-to-learn silently — it simply stops recognising the error.
			//
			// Deleting the shared spelling must fail TestTheUnknownCommandErrorIsOnePrefix.
			return fmt.Errorf("no play matches %q (learn it first)", step)
		}
	}
	// BOUND IN THE SCOPE THE PRESS WILL LOOK IN. `pressHotkey` resolves with the foreground
	// application, so a binding stored under any other scope is one that can never fire.
	//
	// Deleting the shared `app` must fail TestAHotkeyResolvesInTheScopeItWasBoundIn.
	if err := d.Reg.Bind(app, key, cmd); err != nil {
		return err
	}
	scope := app
	if scope == "" {
		scope = "everywhere"
	}
	fmt.Printf("bound `%s -> %s in %s\n", key, cmd, scope)
	return nil
}

// runUnbind: `marco unbind <key>` — remove the hotkey for the current app.
func runUnbind(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: marco unbind <key>")
		os.Exit(2)
	}
	key := strings.ToLower(args[0])
	d := newDeps()
	if err := d.Reg.Unbind(winctx.Active(), key); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("unbound `%s\n", key)
}

// exitProcess ends the process with a code.
//
// A package variable so a test can enter `pressHotkey` — production code — without the test
// binary exiting underneath it. Production never reassigns it.
var exitProcess = os.Exit

// runHotkey: `marco hotkey <key>` — run the play bound to the leader hotkey in
// the current foreground app, or do nothing if none is bound (silent no-op so an
// unbound key over a game is harmless).
func runHotkey(args []string) {
	if len(args) < 1 {
		return
	}
	pressHotkey(newDeps(), winctx.Active(), strings.ToLower(args[0]))
}

// pressHotkey is the whole of what a press does, with the two things a test cannot have — the
// real registry and the real foreground window — handed in.
//
// Split out so a test can ENTER IT rather than re-implement it. The three lines below (HotkeyCmd,
// ParseInvocation, Resolve) are exactly the wiring that must not drift, and a test that copies
// them into itself and then calls `invoke.Decide` directly proves only that the copy works. See
// the memory note "prove wiring by deleting it".
func pressHotkey(d orchestrator.Deps, app, key string) {
	cmd, ok := d.Reg.HotkeyCmd(app, key)
	if !ok {
		return
	}
	// A BINDING ALREADY HOLDS AN IDENTITY, so it is resolved ONCE and performed as that play.
	//
	// It used to put its own command text back through the phrase matcher on every press —
	// `resolveTarget`, the 0.75-score matcher plus an optional external model — so a hotkey
	// that had been bound to a particular play could quietly start running a different one as
	// the registry filled up. A binding is an invocation mapping, not a second chance to guess.
	//
	// Each chained step is resolved here, in this loop, and handed to the one intake as an
	// EXPLICIT identity: `invoke.Decide` then performs that play without reading any words.
	//
	// A step pointing at a play that no longer exists is still skipped silently — a hotkey must
	// never drop into learn-on-unknown, and it must not become a question for Director either.
	// Pressing a key over a game is not a request for interpretation.
	//
	// Deleting the identity and passing text must fail TestAHotkeyPerformsTheBoundPlayItself.
	for _, step := range routes.SplitChain(cmd) {
		base, named, positional := routes.ParseInvocation(step)
		rt, ok := d.Reg.Resolve(app, base)
		if !ok {
			continue
		}
		out, err := runInvocation(d, invoke.Request{
			Source: invoke.SourceHotkey, App: app, Play: &rt,
			Named: named, Positional: positional,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		announce(out)
		if out != OutcomePerformed {
			// A chain stops at the first step that did not perform. Nothing further is
			// attempted, and the outcome travels out as this process.s exit code.
			exitProcess(out.Exit())
			return
		}
	}
}
