package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/nlu"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/resolver"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/voiceteach"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// routesDir is where named routes live (override with $MARCO_ROUTES).
func routesDir() string {
	if d := os.Getenv("MARCO_ROUTES"); d != "" {
		return d
	}
	return "routes"
}

// stopKeySpec is the gesture that ends a recording and aborts a running route
// (override with $MARCO_STOP_KEY, e.g. "esc", "home", "ctrl+f12"). Empty uses
// recorder.DefaultStopKey (F12), which keeps Esc free to record as a key.
func stopKeySpec() string { return os.Getenv("MARCO_STOP_KEY") }

// newDeps builds the orchestrator with the real recorder and the native OS host
// (routes drive and demonstrations capture real input).
func newDeps() orchestrator.Deps {
	hosts := map[string]runtime.Host{"*": oshost.New()}
	// Wire the OCR text resolver when $MARCO_OCR points at the plugin binary, so a
	// route's text anchor (do Text's Find) resolves by OCR. Launched lazily on first
	// use, so it costs nothing when no route needs it. Absent → text anchors fall
	// through to the OS host, which declines, and the click uses its recorded point.
	if ocr := strings.TrimSpace(os.Getenv("MARCO_OCR")); ocr != "" {
		hosts["Text"] = bridgehost.New(ocr)
	}
	// Wire the semantic vision resolver when $MARCO_VISION points at the plugin binary, so
	// a route's `do Vision's Locate/Detect` resolves UI elements by a learned detector.
	// Lazy like the OCR host; absent → Vision calls fall through and decline, and the click
	// uses its recorded point or another resolver.
	if vis := strings.TrimSpace(os.Getenv("MARCO_VISION")); vis != "" {
		hosts["Vision"] = bridgehost.New(vis)
	}
	return orchestrator.Deps{
		Reg:     routes.Registry{Dir: routesDir()},
		Rec:     recorder.New(),
		Hosts:   hosts,
		In:      os.Stdin,
		Out:     os.Stdout,
		App:     winctx.Active,
		StopKey: stopKeySpec(),
		// The overlay sets this: it can't show the simplified preview, so teach's
		// "[s]implify" simplifies AND saves in one step instead of re-confirming.
		SimplifySaves: os.Getenv("MARCO_SIMPLIFY_SAVES") != "",
		// Reserved demonstration key for "an argument goes here" → {{N}} (default F9).
		ArgKey: os.Getenv("MARCO_ARG_KEY"),
	}
}

func runAssistantDo(args []string) {
	name := strings.TrimSpace(strings.Join(args, " "))
	if name == "" {
		fmt.Fprintln(os.Stderr, `usage: marco do "<name>"`)
		os.Exit(2)
	}
	d := newDeps()
	// A command may chain several steps with " then " — run them in order, stopping
	// at the first failure. Each step keeps its own "… with a, b"/"name:" args.
	for _, step := range routes.SplitChain(name) {
		target, named, positional := resolveTarget(d, step)
		mlog.Info("do: dispatching", "target", target, "app", appOf(d))
		if err := dispatchDo(d, target, named, positional); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

// resolveTarget peels a single command's args ("name:value" / "… with a, b") and
// resolves its phrase to an existing route slug, preferring a confident nlu match,
// then the optional model resolver, else the phrase as-is (Do teaches if unknown).
func resolveTarget(d orchestrator.Deps, command string) (target string, named map[string]string, positional []string) {
	base, named, positional := routes.ParseInvocation(command)
	mlog.Debug("do: parsed invocation", "input", command, "base", base, "named_count", len(named), "positional_count", len(positional))
	target = base
	if m := nlu.Resolve(base, d.Reg.Slugs()); m.Route != "" && (m.Exact || m.Score >= 0.75) {
		mlog.Debug("do: nlu match", "input", base, "route", m.Route, "exact", m.Exact, "score", m.Score)
		target = m.Route
	} else if r := resolver.Resolve(context.Background(), base, d.Reg.Slugs()); r != "" {
		mlog.Debug("do: resolver match", "input", base, "route", r)
		target = r // optional model fallback for loose phrasing
	}
	return target, named, positional
}

func runRoutes(args []string) {
	list := newDeps().Reg.List()
	if len(args) > 0 && args[0] == "--json" {
		// Machine-readable, for a UI plugin: [{ "name", "slug", "app", "scope" }]
		type jr struct {
			Name  string `json:"name"`
			Slug  string `json:"slug"`
			App   string `json:"app"`
			Scope string `json:"scope"` // "context" | "focus" | "global"
		}
		out := make([]jr, 0, len(list))
		for _, rt := range list {
			out = append(out, jr{Name: prettyRoute(rt.Slug), Slug: rt.Slug, App: rt.App, Scope: scopeName(rt)})
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
		fmt.Printf("  %-28s (%s)\n", prettyRoute(rt.Slug), scopeDesc(rt))
	}
}

// scopeName is a route's bare scope: context | focus | global.
func scopeName(rt routes.Route) string {
	switch {
	case rt.App == "":
		return "global"
	case rt.Focus:
		return "focus"
	default:
		return "context"
	}
}

// scopeDesc is a human description of a route's reach, for listings.
func scopeDesc(rt routes.Route) string {
	switch {
	case rt.App == "":
		return "global — anywhere"
	case rt.Focus:
		return "focus — " + rt.App + " from anywhere"
	default:
		return "context — only in " + rt.App
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
	// "forget all" / "delete all" wipes every route, after a confirm — so it's not
	// mistaken for a route literally named "all".
	if strings.EqualFold(name, "all") {
		all := d.Reg.List()
		if len(all) == 0 {
			fmt.Println("No routes to forget.")
			return
		}
		if !askYes(fmt.Sprintf("Forget ALL %d routes? This can't be undone. [y]es / [n]o: ", len(all))) {
			fmt.Println("Kept them.")
			return
		}
		n := 0
		for _, rt := range all {
			if err := d.Reg.Delete(rt); err == nil {
				n++
			}
		}
		fmt.Printf("Forgot %d routes.\n", n)
		return
	}
	rt, ok := d.Reg.Resolve(appOf(d), name)
	if !ok {
		fmt.Fprintf(os.Stderr, "No route named %q.\n", name)
		os.Exit(1)
	}
	// Confirm a destructive delete. On a non-interactive stdin (EOF) askYes returns
	// true, so piped/scripted `marco forget` still deletes; an interactive caller
	// (CLI or the overlay's prompt handshake) gets to decline.
	if !askYes(fmt.Sprintf("Forget %q? [y]es / [n]o: ", prettyRoute(rt.Slug))) {
		fmt.Printf("Kept %q.\n", prettyRoute(rt.Slug))
		return
	}
	if err := d.Reg.Delete(rt); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Forgot %q.\n", prettyRoute(rt.Slug))
}

func runRename(args []string) {
	phrase := strings.TrimSpace(strings.Join(args, " "))
	before, after, found := strings.Cut(phrase, " to ")
	oldName, newName := strings.TrimSpace(before), strings.TrimSpace(after)
	if !found || oldName == "" || newName == "" {
		fmt.Fprintln(os.Stderr, `usage: marco rename "old name" to "new name"`)
		os.Exit(2)
	}
	d := newDeps()
	rt, ok := d.Reg.Resolve(appOf(d), oldName)
	if !ok {
		fmt.Fprintf(os.Stderr, "No route named %q.\n", oldName)
		os.Exit(1)
	}
	newRt := routes.Route{App: rt.App, Focus: rt.Focus, Slug: routes.Slug(newName)}
	if d.Reg.Has(newRt) {
		fmt.Fprintf(os.Stderr, "A route named %q already exists.\n", newName)
		os.Exit(1)
	}
	if err := d.Reg.Rename(rt, newRt); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Renamed %q to %q.\n", prettyRoute(rt.Slug), prettyRoute(newRt.Slug))
}

func runAssistantTeach(args []string) {
	// --auto: non-interactive teach (record → simplify → save scoped to the
	// foreground app), so a front-end like the overlay can drive teaching with no
	// prompts and no console window.
	auto, narrate := false, false
	var rest []string
	for _, a := range args {
		switch a {
		case "--auto":
			auto = true
		case "--narrate", "--voice": // narrate from typed stdin lines OR the mic
			narrate = true
		default:
			rest = append(rest, a)
		}
	}
	name := strings.TrimSpace(strings.Join(rest, " "))
	if name == "" && !auto {
		// No name given (e.g. bare "teach" from a console) — ask for one.
		fmt.Print("Name this command: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		name = strings.TrimSpace(line)
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, `usage: marco teach "<name>"`)
		os.Exit(2)
	}
	if auto {
		if err := newDeps().TeachAuto(name); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if narrate {
		if err := runNarrateTeach(name); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := newDeps().Teach(name); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runArgs prints the full, ordered argument labels of the route a phrase resolves
// to, one per line — the overlay weaves them into the "with" clause as colored
// hints in front of each value (so "say hello with chris" reads "with name: chris").
// Prints nothing (and exits 0) when there's no confident match or no args, so a
// front-end treats empty as "no hint".
func runArgs(cliArgs []string) {
	phrase := strings.TrimSpace(strings.Join(cliArgs, " "))
	if phrase == "" {
		return
	}
	d := newDeps()
	base, _, _ := routes.ParseInvocation(phrase)
	target := base
	if m := nlu.Resolve(base, d.Reg.Slugs()); m.Route != "" && (m.Exact || m.Score >= 0.75) {
		target = m.Route
	}
	rt, ok := d.Reg.Resolve(appOf(d), target)
	if !ok {
		return
	}
	src, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		return
	}
	for _, name := range routes.ArgNames(string(src)) {
		fmt.Println(name)
	}
}

// runNarrateTeach drives a narration-built route: each line on stdin is one phrase
// ("click this", "type hello", "wait for this screen", "done") — TYPED at a console
// or piped from the voice plugin's Final transcripts, same path either way.
func runNarrateTeach(name string) error {
	phrases := make(chan string)
	go func() {
		defer close(phrases)
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				phrases <- line
			}
		}
	}()
	return newDeps().TeachVoice(name, voiceteach.NewOSEnv(), phrases, nil)
}

func runAssistantSimplify(args []string) {
	name := strings.TrimSpace(strings.Join(args, " "))
	if name == "" {
		fmt.Fprintln(os.Stderr, `usage: marco simplify "<name>"`)
		os.Exit(2)
	}
	d := newDeps()
	// Reuse an existing route for a close phrasing instead of failing on the
	// exact words (same confident-match rule as `do`).
	target := name
	if m := nlu.Resolve(name, d.Reg.Slugs()); m.Route != "" && (m.Exact || m.Score >= 0.75) {
		target = m.Route
	}
	if err := d.SimplifyRoute(target); err != nil {
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
				fmt.Fprintf(os.Stdout, "  %-28s (%s)\n", prettyRoute(rt.Slug), scopeDesc(rt))
			}
			continue
		}

		m := nlu.Resolve(line, d.Reg.Slugs())
		switch {
		case m.Exact:
			runDo(d, m.Route)
		case m.Route != "" && m.Score >= 0.6:
			if askYes(fmt.Sprintf("Did you mean %q? [y]es / [n]o: ", prettyRoute(m.Route))) {
				runDo(d, m.Route)
			} else {
				runDo(d, line) // teach as a new command under the typed name
			}
		default:
			// Deterministic matcher unsure → optional external resolver plugin
			// ($MARCO_RESOLVER). A no-op when unset.
			if r := resolver.Resolve(context.Background(), line, d.Reg.Slugs()); r != "" &&
				askYes(fmt.Sprintf("Did you mean %q? [y]es / [n]o: ", prettyRoute(r))) {
				runDo(d, r)
			} else {
				runDo(d, line) // unknown → teach
			}
		}
	}
}

func runDo(d orchestrator.Deps, name string) {
	base, named, positional := routes.ParseInvocation(name)
	mlog.Debug("assistant: parsed invocation", "input", name, "base", base, "named_count", len(named), "positional_count", len(positional))
	if err := dispatchDo(d, base, named, positional); err != nil {
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
