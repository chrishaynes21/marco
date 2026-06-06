package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "help", "-h", "--help":
		usage(os.Stdout)
		return
	case "do":
		runAssistantDo(os.Args[2:])
		return
	case "teach":
		runAssistantTeach(os.Args[2:])
		return
	case "assistant":
		runAssistant(os.Args[2:])
		return
	case "routes":
		runRoutes()
		return
	case "forget":
		runForget(os.Args[2:])
		return
	case "secret":
		runSecret(os.Args[2:])
		return
	}
	if len(os.Args) < 3 {
		usage(os.Stderr)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		path, hosts, err := parseRunArgs(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			usage(os.Stderr)
			os.Exit(2)
		}
		// hosts != nil means a real host (windows/bridge) — give it an Esc
		// panic-stop. dryrun (nil) needs none.
		runErr := withPanicStop(hosts != nil, func(ctx context.Context) error {
			return driver.RunFileWithHostsCtx(ctx, path, os.Stdout, hosts)
		})
		if runErr != nil {
			fmt.Fprintln(os.Stderr, runErr)
			os.Exit(1)
		}
	case "serve":
		path, hosts, err := parseRunArgs(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			usage(os.Stderr)
			os.Exit(2)
		}
		if err := driver.ServeFile(path, os.Stdin, os.Stdout, hosts); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "test":
		if err := driver.RunTests(os.Args[2], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "contracts":
		if err := driver.PrintContracts(os.Args[2], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "check":
		jsonMode := false
		path := os.Args[2]
		if path == "--json" {
			if len(os.Args) < 4 {
				usage(os.Stderr)
				os.Exit(2)
			}
			jsonMode = true
			path = os.Args[3]
		}
		if err := driver.Check(path, os.Stdout, jsonMode); err != nil {
			os.Exit(1)
		}
	default:
		usage(os.Stderr)
		os.Exit(2)
	}
}

// parseRunArgs parses the args after `run`: an optional `--host <name>` flag
// followed by the program path. Recognized hosts:
//
//	dryrun  (default) — log foreign calls, resolve ok; deterministic.
//	windows | os      — native SendInput host; real keystrokes/mouse on Windows.
//
// The chosen host is registered as the default ("*") for every foreign act.
func parseRunArgs(args []string) (path string, hosts map[string]runtime.Host, err error) {
	hostName := "dryrun"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--host":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("--host needs a value")
			}
			hostName = args[i+1]
			i++
		default:
			if path != "" {
				return "", nil, fmt.Errorf("unexpected argument %q", args[i])
			}
			path = args[i]
		}
	}
	if path == "" {
		return "", nil, fmt.Errorf("missing <file.marco>")
	}
	switch {
	case hostName == "dryrun":
		return path, nil, nil
	case hostName == "windows" || hostName == "os":
		return path, map[string]runtime.Host{"*": oshost.New()}, nil
	case strings.HasPrefix(hostName, "bridge:"):
		exe := strings.TrimPrefix(hostName, "bridge:")
		if exe == "" {
			return "", nil, fmt.Errorf("bridge host needs a path: --host bridge:<exe>")
		}
		return path, map[string]runtime.Host{"*": bridgehost.New(exe)}, nil
	default:
		return "", nil, fmt.Errorf("unknown host %q (want dryrun, windows, bridge:<exe>)", hostName)
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `marco — a sentence-driven automation language and self-teaching assistant.

Assistant (teach by demonstration, then run by name):
  marco do "<name>"        run a route; if unknown, record it once and remember
  marco teach "<name>"     record/overwrite a route by demonstration
  marco assistant          interactive loop — say what you want, in plain words
  marco routes             list known routes
  marco forget "<name>"    delete a route
  marco secret set|list|rm <name>   manage stored passwords (OS credential store)

Run Marco programs:
  marco run   [--host dryrun|windows|bridge:<exe>] <file.marco>
  marco serve [--host dryrun|windows|bridge:<exe>] <file.marco>

Language tooling:
  marco check [--json] <file.marco>   static check + diagnostics
  marco test <file.marco>             run test blocks
  marco contracts <file.marco>        print inferred action contracts

Hosts: dryrun logs calls (default); windows performs real input; bridge:<exe>
delegates to an external program (e.g. AutoHotkey). Routes live in ./routes
(override with $MARCO_ROUTES). While teaching, type {{name}} for a password.
Press Esc to abort a running route. Set ANTHROPIC_API_KEY to let the assistant
fall back to Claude Haiku for loosely-phrased commands.
`)
}
