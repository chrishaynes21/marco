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
	case "press":
		runPress(os.Args[2:])
		return
	case "teach":
		runAssistantTeach(os.Args[2:])
		return
	case "simplify":
		runAssistantSimplify(os.Args[2:])
		return
	case "edit":
		// `marco edit "<route>"` opens the control center on that route's editor.
		runEdit(strings.TrimSpace(strings.Join(os.Args[2:], " ")), "")
		return
	case "ui":
		// `marco ui` opens the all-routes browser; `marco ui <view>` opens a specific tab
		// (help / routes / bindings / config), e.g. the overlay's help command runs `marco ui help`.
		runEdit("", uiView(os.Args[2:]))
		return
	case "wake":
		// Print the voice activation phrase from overlay.json (the launcher reads it into
		// $MARCO_VOICE_WAKE so the Config tab's edit takes effect on relaunch).
		runWake()
		return
	case "assistant":
		runAssistant(os.Args[2:])
		return
	case "games":
		// Which games Marco understands, and what it may do in them. A pure client, like
		// `marco director`: the capability packs live in the Director service, which is the
		// process that observes the desktop.
		runGames(os.Args[2:])
		return
	case "director":
		// The stable client entry point for a front-end (the overlay, and through it
		// voice). A pure client: the Director service owns all the state.
		runDirector(os.Args[2:])
		return
	case "dispatch":
		// Classify a phrase into one ROUTE-LEVEL decision (run/teach/chat/clarify)
		// without acting. This is NOT the Director (which plans desktop actions from a
		// world model) — it only picks among the routes you already have. A front-end
		// uses `marco dispatch "<phrase>" --json` to converse. See internal/dispatch.
		runDispatch(os.Args[2:])
		return
	case "routes":
		runRoutes(os.Args[2:])
		return
	case "active":
		runActive()
		return
	case "bind":
		runBind(os.Args[2:])
		return
	case "unbind":
		runUnbind(os.Args[2:])
		return
	case "hotkey":
		runHotkey(os.Args[2:])
		return
	case "forget":
		runForget(os.Args[2:])
		return
	case "rename":
		runRename(os.Args[2:])
		return
	case "args":
		runArgs(os.Args[2:])
		return
	case "secret":
		runSecret(os.Args[2:])
		return
	case "diag":
		runDiag(os.Args[2:])
		return
	case "vision":
		runVision(os.Args[2:])
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
		runtime.LogTimestamps = true // serve logs are watched live → stamp them
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

// parseRunArgs parses the args after `run`/`serve`: zero or more `--host <spec>`
// flags and the program path. A spec is one of:
//
//	dryrun  (default) — log foreign calls, resolve ok; deterministic.
//	windows | os      — native SendInput host; real keystrokes/mouse on Windows.
//	bridge:<exe>      — delegate to an external program over JSON stdio.
//
// A bare spec selects the default host for every foreign act ("*"). Prefix it
// with `Act=` to bind one act to its own layer — e.g. route OS effects to the
// macros bridge and the HUD to the overlay bridge in a single run:
//
//	marco serve --host OS=bridge:marco-macros --host Overlay=bridge:overlay overlay.marco
//
// Acts sharing an identical spec share a single host instance (one subprocess).
// A nil hosts map means "all dryrun" (no real host; no panic-stop).
func parseRunArgs(args []string) (path string, hosts map[string]runtime.Host, err error) {
	type binding struct{ act, spec string }
	var bindings []binding
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--host":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("--host needs a value")
			}
			spec := args[i+1]
			i++
			act := "*"
			// `Act=spec` binds one act; an act name is a bare identifier (bridge
			// specs carry ':' but not a leading `name=`).
			if eq := strings.IndexByte(spec, '='); eq > 0 && isActName(spec[:eq]) {
				act, spec = spec[:eq], spec[eq+1:]
			}
			bindings = append(bindings, binding{act, spec})
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
	hosts = map[string]runtime.Host{}
	cache := map[string]runtime.Host{} // spec → shared instance
	for _, b := range bindings {
		h, ok := cache[b.spec]
		if !ok {
			if h, err = hostFromSpec(b.spec); err != nil {
				return "", nil, err
			}
			cache[b.spec] = h
		}
		if h != nil { // dryrun contributes no host (it is the default)
			hosts[b.act] = h
		}
	}
	if len(hosts) == 0 {
		return path, nil, nil
	}
	return path, hosts, nil
}

// hostFromSpec builds a host from one --host spec, or (nil, nil) for dryrun.
func hostFromSpec(spec string) (runtime.Host, error) {
	switch {
	case spec == "dryrun":
		return nil, nil
	case spec == "windows" || spec == "os":
		return oshost.New(), nil
	case strings.HasPrefix(spec, "bridge:"):
		exe := strings.TrimPrefix(spec, "bridge:")
		if exe == "" {
			return nil, fmt.Errorf("bridge host needs a path: --host bridge:<exe>")
		}
		return bridgehost.New(exe), nil
	default:
		return nil, fmt.Errorf("unknown host %q (want dryrun, windows, bridge:<exe>)", spec)
	}
}

// isActName reports whether s is a bare act identifier (or "*"), used to tell a
// `Act=spec` host binding from a plain spec.
func isActName(s string) bool {
	if s == "*" {
		return true
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case i > 0 && (r >= '0' && r <= '9'):
		default:
			return false
		}
	}
	return s != ""
}

func usage(w io.Writer) {
	fmt.Fprint(w, `marco — a sentence-driven automation language and self-teaching assistant.

Assistant (teach by demonstration, then run by name):
  marco do "<name>"        run a route; if unknown, record it once and remember
  marco press <key>        press a key or chord (e.g. enter, ctrl+c, control shift esc)
  marco teach "<name>"     record/overwrite a route by demonstration
  marco simplify "<name>"  re-simplify a saved route as far as it goes
  marco ui                 open the control center (all routes, bindings, help) in a browser
  marco edit "<name>"      open the control center on one route's visual editor
  marco assistant          interactive loop — say what you want, in plain words
  marco dispatch "<phrase>" [--json]  classify a phrase (run/teach/chat/clarify)
  marco routes [--json]    list known routes (--json for a UI plugin)
  marco active             print the foreground app (context)
  marco forget "<name>"    delete a route
  marco rename "old" to "new"  rename a route
  marco secret set|list|rm <name>   manage stored passwords (OS credential store)

Run Marco programs:
  marco run   [--host [Act=]dryrun|windows|bridge:<exe>]... <file.marco>
  marco serve [--host [Act=]dryrun|windows|bridge:<exe>]... <file.marco>

Language tooling:
  marco check [--json] <file.marco>   static check + diagnostics
  marco test <file.marco>             run test blocks
  marco contracts <file.marco>        print inferred action contracts

Vision (semantic UI detector — needs $MARCO_VISION + a model):
  marco vision detect <png> [out]     box every UI element it finds on a screenshot

Hosts: dryrun logs calls (default); windows performs real input; bridge:<exe>
delegates to an external program (e.g. AutoHotkey). Prefix a spec with Act= to
give one act its own layer (e.g. OS=bridge:marco-macros Overlay=bridge:overlay);
a bridge may also push events back to drive serve. Routes live in ./routes
(override with $MARCO_ROUTES). While teaching, type {{name}} for a password.
The stop key ends a recording and aborts a running route — default F12, so Esc
is free to record as an ordinary key; change it with $MARCO_STOP_KEY (e.g. "esc",
"home", "ctrl+f12"). Set $MARCO_RESOLVER to a resolver-plugin
executable to let the assistant fall back to it (e.g. the Claude one in
plugins/claude-resolver) for loosely-phrased commands. The engine is headless;
UIs are plugins that drive it (see plugins/web-ui). Marco itself has no deps.

Conversation: dispatch decides run/teach/chat/clarify from a plain phrase. It
works offline (deterministic matcher) and gets smarter when a brain plugin is set
via $MARCO_ASSISTANT (falls back to $MARCO_RESOLVER) — e.g. the local model in
plugins/llama. A front-end uses "marco dispatch --json" to converse.
`)
}
