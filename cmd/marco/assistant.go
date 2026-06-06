package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/nlu"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/osmod"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/resolver"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// routesDir is where named routes live (override with $MARCO_ROUTES).
func routesDir() string {
	if d := os.Getenv("MARCO_ROUTES"); d != "" {
		return d
	}
	return "routes"
}

// newDeps builds the orchestrator with the real recorder and the native OS host
// (routes drive and demonstrations capture real input).
func newDeps() orchestrator.Deps {
	return orchestrator.Deps{
		Reg:   routes.Registry{Dir: routesDir(), OS: osmod.Source},
		Rec:   recorder.New(),
		Hosts: map[string]runtime.Host{"*": oshost.New()},
		In:    os.Stdin,
		Out:   os.Stdout,
		App:   winctx.Active,
	}
}

func runAssistantDo(args []string) {
	name := strings.TrimSpace(strings.Join(args, " "))
	if name == "" {
		fmt.Fprintln(os.Stderr, `usage: marco do "<name>"`)
		os.Exit(2)
	}
	d := newDeps()
	// Resolve to an existing route so close phrasings reuse it instead of
	// teaching a duplicate. `do` is non-interactive, so only act on confident
	// matches; otherwise pass the phrase through (Do teaches if unknown).
	target := name
	if m := nlu.Resolve(name, d.Reg.Slugs()); m.Route != "" && (m.Exact || m.Score >= 0.75) {
		target = m.Route
	} else if r := resolver.Resolve(context.Background(), name, d.Reg.Slugs()); r != "" {
		target = r // optional model fallback for loose phrasing
	}
	if err := dispatchDo(d, target); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runRoutes(args []string) {
	list := newDeps().Reg.List()
	if len(args) > 0 && args[0] == "--json" {
		// Machine-readable, for a UI plugin: [{ "name", "slug", "app" }]
		type jr struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
			App  string `json:"app"`
		}
		out := make([]jr, 0, len(list))
		for _, rt := range list {
			out = append(out, jr{Name: prettyRoute(rt.Slug), Slug: rt.Slug, App: rt.App})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return
	}
	if len(list) == 0 {
		fmt.Println("No routes yet. Teach one with: marco teach \"<name>\"")
		return
	}
	fmt.Println("Known routes:")
	for _, rt := range list {
		scope := "everywhere"
		if rt.App != "" {
			scope = "in " + rt.App
		}
		fmt.Printf("  %-28s (%s)\n", prettyRoute(rt.Slug), scope)
	}
}

// runActive prints the current foreground app — the context a UI plugin shows
// and that scoped routes resolve against.
func runActive() {
	if a := winctx.Active(); a != "" {
		fmt.Println(a)
	}
}

func runForget(args []string) {
	name := strings.TrimSpace(strings.Join(args, " "))
	if name == "" {
		fmt.Fprintln(os.Stderr, `usage: marco forget "<name>"`)
		os.Exit(2)
	}
	d := newDeps()
	rt, ok := d.Reg.Resolve(appOf(d), name)
	if !ok {
		fmt.Fprintf(os.Stderr, "No route named %q.\n", name)
		os.Exit(1)
	}
	if err := d.Reg.Delete(rt); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Forgot %q.\n", prettyRoute(rt.Slug))
}

func runAssistantTeach(args []string) {
	name := strings.TrimSpace(strings.Join(args, " "))
	if name == "" {
		fmt.Fprintln(os.Stderr, `usage: marco teach "<name>"`)
		os.Exit(2)
	}
	if err := newDeps().Teach(name); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runAssistant is the interactive loop: each line is interpreted as a command.
// The nlu resolver fuzzily maps what you type to one of your saved routes (or
// teaches a new one). This is the seam where a future model-backed resolver
// plugs in — it only has to turn a line into a route name.
func runAssistant(_ []string) {
	d := newDeps()
	fmt.Fprintln(os.Stdout, "marco assistant — say what you want (e.g. \"open chest\"). 'list', 'help', 'quit'.")
	for {
		fmt.Fprint(os.Stdout, "> ")
		raw, ok := readStdinLine()
		if !ok && raw == "" {
			return
		}
		line := strings.TrimSpace(raw)
		switch line {
		case "":
			continue
		case "quit", "exit":
			return
		case "help":
			fmt.Fprintln(os.Stdout, "  type a command to run it; unknown commands are taught by demonstration.")
			fmt.Fprintln(os.Stdout, "  'list' shows known routes. For a password in a route, type {{name}} while teaching")
			fmt.Fprintln(os.Stdout, "  and set it with: marco secret set <name>")
			continue
		case "list":
			for _, rt := range d.Reg.List() {
				scope := "everywhere"
				if rt.App != "" {
					scope = "in " + rt.App
				}
				fmt.Fprintf(os.Stdout, "  %-28s (%s)\n", prettyRoute(rt.Slug), scope)
			}
			continue
		}

		m := nlu.Resolve(line, d.Reg.Slugs())
		switch {
		case m.Exact:
			runDo(d, m.Route)
		case m.Route != "" && m.Score >= 0.6:
			if askYes(fmt.Sprintf("Did you mean %q? [Y/n] ", prettyRoute(m.Route))) {
				runDo(d, m.Route)
			} else {
				runDo(d, line) // teach as a new command under the typed name
			}
		default:
			// Deterministic matcher unsure → optional external resolver plugin
			// ($MARCO_RESOLVER). A no-op when unset.
			if r := resolver.Resolve(context.Background(), line, d.Reg.Slugs()); r != "" &&
				askYes(fmt.Sprintf("Did you mean %q? [Y/n] ", prettyRoute(r))) {
				runDo(d, r)
			} else {
				runDo(d, line) // unknown → teach
			}
		}
	}
}

func runDo(d orchestrator.Deps, name string) {
	if err := dispatchDo(d, name); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

// askYes prompts and reads one line; empty/"y"/"yes" is affirmative.
func askYes(prompt string) bool {
	fmt.Fprint(os.Stdout, prompt)
	a, _ := readStdinLine()
	a = strings.TrimSpace(strings.ToLower(a))
	return a == "" || a == "y" || a == "yes"
}

// readStdinLine reads one line from stdin without buffering ahead (so it never
// steals input from the orchestrator's own prompts on the same stdin). ok is
// false only on EOF with no bytes read.
func readStdinLine() (line string, ok bool) {
	var b []byte
	var one [1]byte
	for {
		n, err := os.Stdin.Read(one[:])
		if n > 0 {
			if one[0] == '\n' {
				return strings.TrimRight(string(b), "\r"), true
			}
			b = append(b, one[0])
		}
		if err != nil {
			return strings.TrimRight(string(b), "\r"), len(b) > 0
		}
	}
}

func prettyRoute(slug string) string { return strings.ReplaceAll(slug, "-", " ") }
