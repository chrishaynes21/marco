package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/nlu"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// runBind: `marco bind <key> "<route>"` — bind the leader hotkey `<key> to a
// route, scoped to the current foreground app. So `s in Notepad can run "say
// hello" while doing nothing elsewhere.
func runBind(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, `usage: marco bind <key> "<route>"`)
		os.Exit(2)
	}
	key := strings.ToLower(args[0])
	cmd := strings.TrimSpace(strings.Join(args[1:], " "))
	d := newDeps()
	app := winctx.Active()

	// A binding can chain steps with " then " ("say hi then wave"). Validate that
	// every step resolves to an existing route, so a typo can't be bound, then store
	// the raw command — each step is resolved + its args applied at run time.
	for _, step := range routes.SplitChain(cmd) {
		base, _, _ := routes.ParseInvocation(step)
		target := base
		if m := nlu.Resolve(base, d.Reg.Slugs()); m.Route != "" && (m.Exact || m.Score >= 0.6) {
			target = m.Route
		}
		if _, ok := d.Reg.Resolve(app, routes.Slug(target)); !ok {
			fmt.Fprintf(os.Stderr, "no route matches %q (teach it first)\n", step)
			os.Exit(1)
		}
	}
	if err := d.Reg.Bind(app, key, cmd); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	scope := app
	if scope == "" {
		scope = "everywhere"
	}
	fmt.Printf("bound `%s -> %s in %s\n", key, cmd, scope)
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

// runHotkey: `marco hotkey <key>` — run the route bound to the leader hotkey in
// the current foreground app, or do nothing if none is bound (silent no-op so an
// unbound key over a game is harmless).
func runHotkey(args []string) {
	if len(args) < 1 {
		return
	}
	d := newDeps()
	app := winctx.Active()
	cmd, ok := d.Reg.HotkeyCmd(app, strings.ToLower(args[0]))
	if !ok {
		return
	}
	// Run each chained step in order. A step pointing at a route that no longer
	// exists is skipped silently (a hotkey must never drop into teach-on-unknown).
	for _, step := range routes.SplitChain(cmd) {
		target, named, positional := resolveTarget(d, step)
		if _, ok := d.Reg.Resolve(app, target); !ok {
			continue
		}
		if err := dispatchDo(d, target, named, positional); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
