package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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

// runAssistant is the interactive loop: each line is a command name. This is the
// seam where a future natural-language / voice front-end plugs in — it only has
// to produce the command-name string.
func runAssistant(_ []string) {
	d := newDeps()
	fmt.Fprintln(os.Stdout, "marco assistant — type a command (e.g. \"open chest\"), or 'quit'.")
	sc := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprint(os.Stdout, "> ")
		if !sc.Scan() {
			return
		}
		name := strings.TrimSpace(sc.Text())
		switch name {
		case "":
			continue
		case "quit", "exit":
			return
		case "list":
			for _, r := range d.Reg.List() {
				fmt.Fprintln(os.Stdout, "  "+r)
			}
			continue
		}
		if err := d.Do(name); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}
