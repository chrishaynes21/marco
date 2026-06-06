package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/nlu"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/osmod"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
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
	if m := nlu.Resolve(name, d.Reg.List()); m.Route != "" && (m.Exact || m.Score >= 0.75) {
		target = m.Route
	}
	if err := d.Do(target); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runRoutes() {
	list := newDeps().Reg.List()
	if len(list) == 0 {
		fmt.Println("No routes yet. Teach one with: marco teach \"<name>\"")
		return
	}
	fmt.Println("Known routes:")
	for _, r := range list {
		fmt.Println("  " + prettyRoute(r))
	}
}

func runForget(args []string) {
	name := strings.TrimSpace(strings.Join(args, " "))
	if name == "" {
		fmt.Fprintln(os.Stderr, `usage: marco forget "<name>"`)
		os.Exit(2)
	}
	d := newDeps()
	if !d.Reg.Has(name) {
		fmt.Fprintf(os.Stderr, "No route named %q.\n", name)
		os.Exit(1)
	}
	if err := d.Reg.Delete(name); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Forgot %q.\n", name)
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
			for _, r := range d.Reg.List() {
				fmt.Fprintln(os.Stdout, "  "+prettyRoute(r))
			}
			continue
		}

		m := nlu.Resolve(line, d.Reg.List())
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
			runDo(d, line) // unknown → teach
		}
	}
}

func runDo(d orchestrator.Deps, name string) {
	if err := d.Do(name); err != nil {
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
