package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/nlu"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/resolver"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/voicelearn"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// routesDir is where plays live on disk (override with $MARCO_ROUTES). The directory keeps its
// old name deliberately: renaming it would move every play a person already has.
func routesDir() string {
	if d := os.Getenv("MARCO_ROUTES"); d != "" {
		return d
	}
	return "routes"
}

// stopKeySpec is the gesture that ends a recording and aborts a running play
// (override with $MARCO_STOP_KEY, e.g. "esc", "home", "ctrl+f12"). Empty uses
// recorder.DefaultStopKey (F12), which keeps Esc free to record as a key.
func stopKeySpec() string { return os.Getenv("MARCO_STOP_KEY") }

// newDeps builds the orchestrator with the real recorder and the native OS host
// (plays drive and demonstrations capture real input).
func newDeps() orchestrator.Deps {
	hosts := map[string]runtime.Host{"*": oshost.New()}
	// The Screen act, so a play can say where it begins. Read-only: it looks and compares,
	// and cannot press anything. A play that asks and gets no answer refuses.
	hosts["Screen"] = newScreenHost()
	// Wire the OCR text resolver when $MARCO_OCR points at the plugin binary, so a
	// play's text anchor (do Text's Find) resolves by OCR. Launched lazily on first
	// use, so it costs nothing when no play needs it. Absent → text anchors fall
	// through to the OS host, which declines, and the click uses its recorded point.
	if ocr := strings.TrimSpace(os.Getenv("MARCO_OCR")); ocr != "" {
		hosts["Text"] = bridgehost.New(ocr)
	}
	// Wire the semantic vision resolver when $MARCO_VISION points at the plugin binary, so
	// a play's `do Vision's Locate/Detect` resolves UI elements by a learned detector.
	// Lazy like the OCR host; absent → Vision calls fall through and decline, and the click
	// uses its recorded point or another resolver.
	if vis := strings.TrimSpace(os.Getenv("MARCO_VISION")); vis != "" {
		hosts["Vision"] = bridgehost.New(vis)
	}
	// The Accessibility act, when a bridge is available, and the THEATER above it.
	//
	// The Theater is what a learned play actually asks: `do Theater's Activate with target1`.
	// It casts whichever actor can play the part on this machine — accessibility today,
	// something else later — so a play learned here still runs somewhere with a different
	// perception configuration. See [[ADR-068-the-theater-is-the-durable-semantic-world]].
	//
	// Wired unconditionally. A Theater with no actors is inert and SAYS so, which is the
	// honest answer for a machine that cannot act on a target; leaving the act unfulfilled
	// would make the same situation look like a broken program.
	var accessibility runtime.Host
	if uia := accessibilityBridge(); uia != "" {
		accessibility = bridgehost.New(uia)
		hosts["Accessibility"] = accessibility
	}
	hosts["Theater"] = newTheaterHost(hosts, accessibility)
	d := orchestrator.Deps{
		Reg:     routes.Registry{Dir: routesDir()},
		Rec:     recorder.New(),
		Hosts:   hosts,
		In:      os.Stdin,
		Out:     os.Stdout,
		App:     winctx.Active,
		StopKey: stopKeySpec(),
		// The overlay sets this: it can't show the simplified preview, so learn's
		// "[s]implify" simplifies AND saves in one step instead of re-confirming.
		SimplifySaves: os.Getenv("MARCO_SIMPLIFY_SAVES") != "",
		// Reserved demonstration key for "an argument goes here" → {{N}} (default F9).
		ArgKey: os.Getenv("MARCO_ARG_KEY"),
	}
	// THE door. Every invocation passes through it; only a learned play with intact
	// provenance is ever stopped there, and then only to ask. See
	// [[ADR-029-resolution-is-not-permission]].
	d.Authority = orchestrator.AskFirst{Deps: d}
	return d
}

// runAssistantDo is the product's entrance, and every other entrance comes through it.
//
// # Why the flags exist
//
//	--source  says HOW intent arrived, so an acceptance run can prove that typing and speaking
//	          take the same path. Recorded, never consulted: see [[internal/invoke]].
//	--play    is an identity the caller ALREADY holds — a clicked Run, a resolved binding. The
//	          words are then not read at all, because a display name is derived from a slug and
//	          the slug is not a general inverse of the name, so a second lookup can land
//	          somewhere else entirely.
//
// # What it stopped doing
//
// It no longer fuzzes the phrase to a slug before anything consults the registry. `resolveTarget`
// ran a 0.75-score matcher and then an optional external model IN FRONT OF the door, so an exact
// durable identity could lose to a fuzzy neighbour silently — measured: with both "open settings"
// and "open the settings" registered, asking for the second ran the first. A near miss is now a
// miss, and a miss belongs to Director, which can see and can ask.
//
// Deleting the runInvocation call must fail TestEveryEntranceRoutesThroughTheOneIntake.
func runAssistantDo(args []string) {
	req := invoke.Request{Source: invoke.SourceCLI}
	var explicit routes.Route
	var haveExplicit bool
	var words []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case strings.HasPrefix(a, "--source="):
			req.Source = invoke.Source(strings.TrimPrefix(a, "--source="))
		case strings.HasPrefix(a, "--play="):
			explicit.Slug = routes.Slug(strings.TrimPrefix(a, "--play="))
			haveExplicit = true
		case strings.HasPrefix(a, "--app="):
			explicit.App = strings.TrimPrefix(a, "--app=")
			req.App = explicit.App
		case a == "--focus":
			explicit.Focus = true
		default:
			words = append(words, a)
		}
	}
	req.Text = strings.TrimSpace(strings.Join(words, " "))
	if haveExplicit {
		req.Play = &explicit
	}
	if req.Text == "" && req.Play == nil {
		fmt.Fprintln(os.Stderr, `usage: marco do "<name>"`)
		os.Exit(2)
	}
	out, err := runInvocation(newDeps(), req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	announce(out)
	os.Exit(out.Exit())
}

// playJSON is the wire shape BOTH `marco routes --json` and `marco plays --json` emit.
//
// One struct, so the two commands can never drift into two spellings of the same fact.
//
// # The first four keys are a published contract
//
// name/slug/app/scope are parsed by consumers outside this module, which is why they survive a
// vocabulary change that renamed everything a person reads. Keys may be ADDED — a JSON decoder
// ignores what it does not know — and may never be renamed or removed.
//
// Renaming any of the first four must fail TestMarcoRoutesJSONKeepsItsPublishedKeys.
type playJSON struct {
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	App   string `json:"app"`
	Scope string `json:"scope"` // "context" | "focus" | "global"

	// Added with the product vocabulary. Every one of them is a rendering of something
	// internal/plays already decided — nothing here re-derives a kind, a scope or a standing.
	Kind       string `json:"kind"`                // "Authored" | "Recorded" | "Learned"
	Life       string `json:"life"`                // "ready"|"edited"|"unverified"|"saved"|"stuck"
	Registered bool   `json:"registered"`          // false ⇒ saved, and nothing can ask for it
	Activates  string `json:"activates,omitempty"` // brought forward before the play runs
}

// jsonOf renders a listing for a machine. No decisions of its own.
func jsonOf(list []plays.Play) []playJSON {
	out := make([]playJSON, 0, len(list))
	for _, p := range list {
		out = append(out, playJSON{
			Name: p.Name, Slug: p.Slug, App: p.Application, Scope: scopeName(p.Route),
			Kind: plays.KindWord(p.Kind), Life: string(p.Life),
			Registered: p.Registered, Activates: p.Activates,
		})
	}
	return out
}

func emitJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// runRoutes lists ONLY the plays anything can ask for — plays.Registered, never plays.List.
//
// # Why this is not `marco plays`
//
// Because its consumers are front ends, and a name a front end offers has to answer. The overlay
// and the resolver plugins read this list to decide what a phrase may resolve to; a staged play
// printed here would be Marco advertising a capability that `marco do` cannot find. `marco plays`
// is the product view and shows both groups, each labelled with where it stands.
//
// The verb keeps its name for the same reason its JSON keeps its keys: out-of-module callers say
// `marco routes` and nothing tells them not to.
//
// Widening this to plays.List must fail TestMarcoRoutesOffersOnlyPlaysThatCanAnswer.
func runRoutes(args []string) {
	list := plays.Registered(newDeps().Reg)
	if len(args) > 0 && args[0] == "--json" {
		emitJSON(jsonOf(list))
		return
	}
	if len(list) == 0 {
		fmt.Println("No plays yet. Learn one with: marco learn \"<name>\"")
		return
	}
	fmt.Println("Known plays:")
	for _, p := range list {
		fmt.Printf("  %-28s (%s)\n", p.Name, scopeDesc(p.Route))
	}
}

// runPlays is the product listing: everything Marco has, in the two groups the product shows.
//
// # Why it is not `marco routes` with a flag
//
// Because the two answer different questions and both answers are wanted. `marco routes` is what
// may be OFFERED; this is what a person HAS. A play that is saved and not yet askable is a real
// thing on disk that somebody can read, edit and register — and a listing that hid it would leave
// them with a file no command they know about mentions.
//
// The two groups are printed apart rather than mixed with a badge, because the difference is not a
// nuance: one of them answers when you ask for it and the other does not.
//
// Dropping the staged group must fail TestMarcoPlaysShowsTheSavedPlayMarcoRoutesMustNotOffer.
func runPlays(args []string) {
	list := plays.List(newDeps().Reg)
	if len(args) > 0 && args[0] == "--json" {
		emitJSON(jsonOf(list))
		return
	}
	var askable, staged []plays.Play
	for _, p := range list {
		if p.Registered {
			askable = append(askable, p)
		} else {
			staged = append(staged, p)
		}
	}
	if len(askable) == 0 && len(staged) == 0 {
		fmt.Println("No plays yet. Learn one with: marco learn \"<name>\"")
		return
	}
	if len(askable) > 0 {
		fmt.Println("Known plays:")
		for _, p := range askable {
			fmt.Printf("  %-28s %s · %s · %s\n",
				p.Name, plays.KindWord(p.Kind), p.Life.Word(), reachOf(p))
		}
	}
	if len(staged) == 0 {
		return
	}
	if len(askable) > 0 {
		fmt.Println()
	}
	fmt.Println("Saved, not askable yet:")
	for _, p := range staged {
		line := fmt.Sprintf("  %-28s %s · %s", p.Name, plays.KindWord(p.Kind), p.Life.Word())
		// The offer is made only where Marco can keep it. A stuck play — one edited since
		// its provenance was written — cannot be registered as learned, and naming the
		// command beside it would be an instruction that fails when followed.
		//
		// Deleting the Registerable guard must fail TestAStuckPlayIsNotOfferedRegistration.
		if p.Life.Registerable() {
			line += fmt.Sprintf("   (marco register %q)", p.Name)
		}
		fmt.Println(line)
	}
}

// runRegister makes a saved play askable: `marco register "<name>"`.
//
// It exists because `marco plays` names it. A listing that pointed at a command Marco does not
// have would be worse than no offer at all.
//
// # It acts on the slug, and takes the application from the row
//
// The slug is the only handle that identifies a play; the display name is that slug with its
// dashes back as spaces, so re-slugging what a person typed is how it becomes a handle again. The
// application comes from the staged listing rather than from the foreground window, because
// registering is not something you do WHILE the application is in front — and the foreground when
// you type this is a terminal.
//
// Deleting the staged lookup and building the Route from winctx.Active must fail
// TestRegisteringASavedPlayDoesNotDependOnWhatIsInFront.
func runRegister(args []string) {
	name := strings.TrimSpace(strings.Join(args, " "))
	if name == "" {
		fmt.Fprintln(os.Stderr, `usage: marco register "<name>"`)
		os.Exit(2)
	}
	reg := newDeps().Reg
	slug := routes.Slug(name)
	for _, p := range plays.Staged(reg) {
		if p.Slug != slug {
			continue
		}
		// routes.LearnedFocus, not p.Route.Focus: the scope a staged play registers into is
		// one decision taken in one place, and this is the caller that acts on it.
		rt := routes.Route{App: p.Application, Focus: routes.LearnedFocus, Slug: p.Slug}
		if err := reg.Register(rt); err != nil {
			// unprefixRoutes, the same strip the control centre applies to the same
			// call: "routes: " is a Go package name, and a person reading a refusal
			// about their own play has no use for it.
			fmt.Fprintln(os.Stderr, unprefixRoutes(err.Error()))
			os.Exit(1)
		}
		fmt.Printf("Registered %q. You can ask for it now.\n", p.Name)
		return
	}
	fmt.Fprintf(os.Stderr, "No saved play named %q. `marco plays` lists what is waiting.\n", name)
	os.Exit(1)
}

// scopeName is a play's bare scope: context | focus | global. One definition, in internal/plays.
func scopeName(rt routes.Route) string { return string(plays.ScopeOf(rt)) }

// reachOf is where a play answers from, as a listing row says it.
//
// FOCUS SAYS WHAT IT DOES. "From anywhere" is also true of a global play, and the difference —
// Marco brings the application forward first — is the capability worth having, so the row names
// it. See plays.Scope.Says, which makes the same point at sentence length.
//
// Collapsing the focus arm into the global one must fail TestFocusReadsDifferentlyFromContext.
func reachOf(p plays.Play) string {
	switch p.Scope {
	case plays.ScopeFocus:
		return "from anywhere (brings " + p.Application + " forward)"
	case plays.ScopeContext:
		return "only in " + p.Application
	default:
		return "anywhere"
	}
}

// scopeDesc is a human description of a play's reach, for listings.
func scopeDesc(rt routes.Route) string {
	switch {
	case rt.App == "":
		return "global — anywhere"
	case rt.Focus:
		// Naming the activation, not just the application: a focus play is the one that
		// brings its own application forward, and "chrome from anywhere" never said so.
		return "focus — from anywhere; brings " + rt.App + " forward"
	default:
		return "context — only in " + rt.App
	}
}

// runActive prints the current foreground app — the context a UI plugin shows
// and that scoped plays resolve against.
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
	// "forget all" / "delete all" wipes every play, after a confirm — so it's not
	// mistaken for a play literally named "all".
	if strings.EqualFold(name, "all") {
		all := d.Reg.List()
		if len(all) == 0 {
			fmt.Println("No plays to forget.")
			return
		}
		if !askYes(fmt.Sprintf("Forget ALL %d plays? This can't be undone. [y]es / [n]o: ", len(all))) {
			fmt.Println("Kept them.")
			return
		}
		n := 0
		for _, rt := range all {
			if err := d.Reg.Delete(rt); err == nil {
				n++
			}
		}
		fmt.Printf("Forgot %d plays.\n", n)
		return
	}
	rt, ok := d.Reg.Resolve(appOf(d), name)
	if !ok {
		fmt.Fprintf(os.Stderr, "No play named %q.\n", name)
		os.Exit(1)
	}
	// Confirm a destructive delete. On a non-interactive stdin (EOF) askYes returns
	// true, so piped/scripted `marco forget` still deletes; an interactive caller
	// (CLI or the overlay's prompt handshake) gets to decline.
	if !askYes(fmt.Sprintf("Forget %q? [y]es / [n]o: ", prettyRoute(rt.Slug))) {
		fmt.Printf("Kept %q.\n", prettyRoute(rt.Slug))
		return
	}
	if err := forgetPlay(d.Reg, rt); err != nil {
		fmt.Fprintln(os.Stderr, unprefixRoutes(err.Error()))
		os.Exit(1)
	}
	fmt.Printf("Forgot %q.\n", prettyRoute(rt.Slug))
}

// forgetPlay removes one registered play: its source, its recording, its anchors and its past.
//
// # Why the sidecar goes too
//
// `Delete` alone leaves the `.origin.json` behind — a record describing a play that is no longer
// there. Nothing a person has could then remove it, and `Origin` has to refuse it separately so
// the next play saved under that slug does not inherit a past it never had. The Plays surface
// removes the pair; two doors onto one act had better take the same things.
//
// # And why it stops there
//
// It does NOT reach `<app>/learned/`. A staged play of the same name is a different file with its
// own standing — and being unregisterable because of a collision with THIS play is the ordinary
// reason one exists. Forgetting this play is what clears that collision; it is not an instruction
// to destroy what was waiting.
//
// Split out of runForget so it can be proved rather than read: runForget itself reads stdin and
// calls os.Exit. Deleting the DeleteOrigin call must fail TestForgettingAPlayTakesItsPastWithIt.
func forgetPlay(reg routes.Registry, rt routes.Route) error {
	if err := reg.DeleteOrigin(rt); err != nil {
		return err
	}
	return reg.Delete(rt)
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
		fmt.Fprintf(os.Stderr, "No play named %q.\n", oldName)
		os.Exit(1)
	}
	newRt := routes.Route{App: rt.App, Focus: rt.Focus, Slug: routes.Slug(newName)}
	if d.Reg.Has(newRt) {
		fmt.Fprintf(os.Stderr, "A play named %q already exists.\n", newName)
		os.Exit(1)
	}
	if err := d.Reg.Rename(rt, newRt); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Renamed %q to %q.\n", prettyRoute(rt.Slug), prettyRoute(newRt.Slug))
}

func runAssistantLearn(args []string) {
	// --auto: non-interactive learn (record → simplify → save scoped to the
	// foreground app), so a front-end like the overlay can drive learning with no
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
		// No name given (e.g. bare "learn" from a console) — ask for one.
		fmt.Print("Name this command: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		name = strings.TrimSpace(line)
	}
	if name == "" {
		fmt.Fprintln(os.Stderr, `usage: marco learn "<name>"`)
		os.Exit(2)
	}
	if auto {
		if err := newDeps().LearnAuto(name); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if narrate {
		if err := runNarrateLearn(name); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := newDeps().Learn(name); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runArgs prints the full, ordered argument labels of the play a phrase resolves
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

// runNarrateLearn drives a narration-built play: each line on stdin is one phrase
// ("click this", "type hello", "wait for this screen", "done") — TYPED at a console
// or piped from the voice plugin's Final transcripts, same path either way.
func runNarrateLearn(name string) error {
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
	return newDeps().LearnVoice(name, voicelearn.NewOSEnv(), phrases, nil)
}

func runAssistantSimplify(args []string) {
	name := strings.TrimSpace(strings.Join(args, " "))
	if name == "" {
		fmt.Fprintln(os.Stderr, `usage: marco simplify "<name>"`)
		os.Exit(2)
	}
	d := newDeps()
	// Reuse an existing play for a close phrasing instead of failing on the
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
// The nlu resolver fuzzily maps what you type to one of your saved plays (or
// learns a new one). This is the seam where a future model-backed resolver
// plugs in — it only has to turn a line into a play name.
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
			fmt.Fprintln(os.Stdout, "  type a command to run it; unknown commands are learned by demonstration.")
			fmt.Fprintln(os.Stdout, "  'list' shows known plays. For a password in a play, type {{name}} while learning")
			fmt.Fprintln(os.Stdout, "  and set it with: marco secret set <name>")
			continue
		case "list":
			// The same projection `marco routes` prints, so the loop and the command
			// cannot come to different conclusions about the same play.
			for _, p := range plays.Registered(d.Reg) {
				fmt.Fprintf(os.Stdout, "  %-28s (%s)\n", p.Name, scopeDesc(p.Route))
			}
			continue
		}

		m := nlu.Resolve(line, d.Reg.Slugs())
		if m.Exact {
			runDo(d, m.Route) // exact play name — free offline fast path, no model call
			continue
		}
		// Conversational brain: the director's local-LLM Advisor turns a loose line
		// into run/teach/chat/clarify. Returns false (and we fall through) when no
		// brain is wired or it's unsure, so the classic fuzzy-confirm flow below —
		// including any legacy $MARCO_RESOLVER — is preserved unchanged.
		if converseTurn(d, line) {
			continue
		}
		switch {
		case m.Route != "" && m.Score >= 0.6:
			if askYes(fmt.Sprintf("Did you mean %q? [y]es / [n]o: ", prettyRoute(m.Route))) {
				runDo(d, m.Route)
			} else {
				runDo(d, line) // learn as a new command under the typed name
			}
		default:
			// Deterministic matcher unsure → optional external resolver plugin
			// ($MARCO_RESOLVER). A no-op when unset.
			if r := resolver.Resolve(context.Background(), line, d.Reg.Slugs()); r != "" &&
				askYes(fmt.Sprintf("Did you mean %q? [y]es / [n]o: ", prettyRoute(r))) {
				runDo(d, r)
			} else {
				runDo(d, line) // unknown → learn
			}
		}
	}
}

// runDo performs one command from the interactive loop, through the one intake.
//
// The REPL is a DEVELOPER SURFACE — nothing launches it, and `marco help` still lists it — but it
// does not get its own idea of what a phrase means. Its own matcher above ASKS before it
// substitutes anything ("Did you mean …?"), which is a confirmation rather than a silent guess,
// and whatever the person confirms arrives here and takes exactly the path every other entrance
// takes.
func runDo(d orchestrator.Deps, name string) {
	out, err := runInvocation(d, invoke.Request{Text: name, Source: invoke.SourceCLI})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	announce(out)
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

func prettyRoute(slug string) string { return plays.Pretty(slug) }

// accessibilityBridge is the accessibility plugin this machine can cast, or nothing.
//
// # Why discovery rather than an environment variable alone
//
// The Theater is wired unconditionally and casts whichever actor can play the part. Its only actor
// today is accessibility — so with no bridge the Theater is inert, and a learned play that says
// `do Theater's Activate` reaches a Theater with nobody in it.
//
// That is exactly what a live invocation hit: the play was saved, registered and resolvable, the
// Audience stood on the right screen, asked for it, and nothing happened. `$MARCO_UIA_BRIDGE` was
// never set, because nothing tells anybody to set it — and the Director next to it has always
// found the plugin by looking.
//
// So `marco` looks the same way the Director does: beside the executable first, so a packaged
// install works, then the working directory, so a build tree does. The environment variable still
// wins where somebody has said explicitly which bridge to use.
//
// Empty when there is no plugin, and the Theater then says it has no actor — which is the honest
// answer for a machine that cannot act, and different from a broken program.
//
// Deleting the discovery must fail TestALearnedPlayFindsAnActorWithoutBeingTold.
func accessibilityBridge() string {
	if uia := strings.TrimSpace(os.Getenv("MARCO_UIA_BRIDGE")); uia != "" {
		return uia
	}
	const rel = "plugins/uia/uia.exe"
	if exe, err := os.Executable(); err == nil {
		if candidate := filepath.Join(filepath.Dir(exe), rel); exists(candidate) {
			return candidate
		}
	}
	if p := filepath.FromSlash(rel); exists(p) {
		return p
	}
	return ""
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }
