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
	name := strings.TrimSpace(strings.Join(args[1:], " "))
	d := newDeps()
	app := winctx.Active()

	// Resolve the spoken/typed name to an existing route slug (confident match),
	// else slugify as-is.
	target := name
	if m := nlu.Resolve(name, d.Reg.Slugs()); m.Route != "" && (m.Exact || m.Score >= 0.6) {
		target = m.Route
	}
	slug := routes.Slug(target)
	if _, ok := d.Reg.Resolve(app, slug); !ok {
		fmt.Fprintf(os.Stderr, "no route matches %q (teach it first)\n", name)
		os.Exit(1)
	}
	if err := d.Reg.Bind(app, key, slug); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	scope := app
	if scope == "" {
		scope = "everywhere"
	}
	fmt.Printf("bound `%s -> %s in %s\n", key, slug, scope)
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
	slug, ok := d.Reg.HotkeySlug(app, strings.ToLower(args[0]))
	if !ok {
		return
	}
	if _, ok := d.Reg.Resolve(app, slug); !ok {
		return // binding points at a route that no longer exists
	}
	if err := dispatchDo(d, slug, nil, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
