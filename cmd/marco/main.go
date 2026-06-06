package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "do":
		runAssistantDo(os.Args[2:])
		return
	case "teach":
		runAssistantTeach(os.Args[2:])
		return
	case "assistant":
		runAssistant(os.Args[2:])
		return
	case "secret":
		runSecret(os.Args[2:])
		return
	}
	if len(os.Args) < 3 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		path, hosts, err := parseRunArgs(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			usage()
			os.Exit(2)
		}
		if err := driver.RunFileWithHosts(path, os.Stdout, hosts); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "serve":
		path, hosts, err := parseRunArgs(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			usage()
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
				usage()
				os.Exit(2)
			}
			jsonMode = true
			path = os.Args[3]
		}
		if err := driver.Check(path, os.Stdout, jsonMode); err != nil {
			os.Exit(1)
		}
	default:
		usage()
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

func usage() {
	fmt.Fprintln(os.Stderr, "usage: marco run   [--host dryrun|windows|bridge:<exe>] <file.marco>")
	fmt.Fprintln(os.Stderr, "       marco serve [--host dryrun|windows|bridge:<exe>] <file.marco>")
	fmt.Fprintln(os.Stderr, "       marco do \"<name>\"      run a named route, or teach it if unknown")
	fmt.Fprintln(os.Stderr, "       marco teach \"<name>\"   record/overwrite a named route")
	fmt.Fprintln(os.Stderr, "       marco assistant         interactive loop: type a command per line")
	fmt.Fprintln(os.Stderr, "       marco secret set|list|rm <name>   manage stored passwords")
	fmt.Fprintln(os.Stderr, "       marco test <file.marco>")
	fmt.Fprintln(os.Stderr, "       marco contracts <file.marco>")
	fmt.Fprintln(os.Stderr, "       marco check [--json] <file.marco>")
}
