package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/nlu"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/osmod"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
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
	if err := newDeps().Do(name); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
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
	sc := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprint(os.Stdout, "> ")
		if !sc.Scan() {
			return
		}
		line := strings.TrimSpace(sc.Text())
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
			if askYes(sc, fmt.Sprintf("Did you mean %q? [Y/n] ", prettyRoute(m.Route))) {
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

// askYes reads a line via the REPL's scanner; empty/"y"/"yes" is affirmative.
func askYes(sc *bufio.Scanner, prompt string) bool {
	fmt.Fprint(os.Stdout, prompt)
	if !sc.Scan() {
		return false
	}
	a := strings.TrimSpace(strings.ToLower(sc.Text()))
	return a == "" || a == "y" || a == "yes"
}

func prettyRoute(slug string) string { return strings.ReplaceAll(slug, "-", " ") }
