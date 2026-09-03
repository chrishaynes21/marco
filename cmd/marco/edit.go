package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// faviconPNG is the neon >m mark, served at /icon.png for the control center's favicon + logo.
//
//go:embed favicon.png
var faviconPNG []byte

// runEdit opens the Marco control center — a small local web app served from stdlib net/http
// (so the zero-dep engine rule holds). It opens on the named play's Edit view, where every step
// across the OS harness is editable (waits, coordinates, keys/hold/release, type, focus/launch,
// secrets, repeat blocks); a hamburger nav switches to Plays (browse / open / run / register),
// Bindings (leader hotkeys), and Help. Ctrl+C to stop.
//
//	marco edit "enter freeplay"
//
// uiView maps a `marco ui <arg>` argument to a known opening view, or "" (the default browser).
//
// # "plays" and "routes" are ONE view under two words
//
// The view IDENTIFIER stays `routes`: it is the DOM id, the nav key and the argument this command
// has always taken, and an identifier is not a product word. The Audience's word for what the view
// lists is now PLAY, so `marco ui plays` has to reach the same place — otherwise the tab a person
// can see says one thing and the only argument that opens it says another.
//
// `settings` and `config` are the same pairing, made for the same reason: the tab a person reads
// is called Settings now, and the DOM id and the argument that has always worked stay `config`.
//
// # EVERY VIEW THE PAGE HAS IS NAMEABLE HERE
//
// This used to accept five of them, and `learn` was not among the five — so the one capability the
// product is named for had no deep link at all. The overlay could not open it, `marco ui learn`
// silently landed on the Plays browser, and the only way in was to open the control centre and go
// looking for the tab. A view a person can see and cannot be SENT to is a view the rest of the
// product cannot point at.
//
// So the rule is structural rather than a list somebody remembers to extend:
// TestEveryViewInTheControlCentreCanBeOpenedByName reads the sections `editPage` actually renders
// and fails if one of them cannot be named here.
//
// Deleting the "plays" arm must fail TestMarcoUiPlaysOpensThePlaysView.
func uiView(args []string) string {
	switch v := strings.ToLower(strings.TrimSpace(strings.Join(args, " "))); v {
	case "plays":
		return "routes"
	case "settings":
		return "config"
	case "source", "steps":
		return "edit"
	// Cast is a TABLE ON the Advanced page rather than a view of its own, so the word opens the
	// page that holds it. Somebody who typed `marco ui cast` asked to see the cast; landing on
	// the default browser instead would be the command pretending not to understand.
	case "cast":
		return "advanced"
	case "help", "routes", "bindings", "config", "edit",
		"here", "learn", "activity", "advanced":
		return v
	default:
		return ""
	}
}

func runEdit(name, view string) {
	name = strings.TrimSpace(name)
	d := newDeps()
	ed := &editor{reg: d.Reg, app: appOf(d), view: view}
	// With a name, open on that play's Edit view; without one (marco ui / bare marco edit) the
	// control center lands on the Plays browser and no play is loaded yet — the step editor is a
	// tool for a play you already have, not the front door to the product.
	if name != "" {
		rt, ok := d.Reg.Resolve(appOf(d), name)
		if !ok {
			if rt, ok = findRouteByName(d.Reg, name); !ok {
				fmt.Fprintf(os.Stderr, "No play named %q. Known plays: run `marco plays`.\n", name)
				os.Exit(1)
			}
		}
		ed.rt, ed.path = rt, d.Reg.Path(rt)
		if err := ed.loadSrc(); err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", ed.path, err)
			os.Exit(1)
		}
	}

	ln, err := net.Listen("tcp", "localhost:0") // any free port
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	url := fmt.Sprintf("http://%s/", ln.Addr().String())

	mux := ed.serveMux()

	where := "all plays"
	if name != "" {
		where = fmt.Sprintf("%q", prettyRoute(ed.rt.Slug))
	}
	fmt.Printf("marco control center (%s) → %s  (Ctrl+C when done)\n", where, url)
	openBrowser(url)
	if err := http.Serve(ln, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// serveMux is EVERY DOOR THE CONTROL CENTRE HAS, and it is a function so a test can open them.
//
// It used to be built inline in runEdit, which also binds a port, prints, opens a browser and then
// blocks in http.Serve — so nothing could ask what this surface actually serves without starting
// it. That is the exact shape of the failure this project keeps finding: a handler written, a page
// written to call it, and no test able to enter where production enters, so the two agree only by
// somebody having remembered.
//
// Deleting a registration here must fail TestEveryDoorThePageKnocksOnIsAnswered.
func (ed *editor) serveMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store") // always serve the current page (no stale JS)
		fmt.Fprint(w, editPage)
	})
	mux.HandleFunc("/icon.png", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Write(faviconPNG)
	})
	mux.HandleFunc("/api/route", ed.handleRoute) // the active play's steps + source
	mux.HandleFunc("/api/save", ed.handleSave)   // write step edits back
	// /api/routes is the EDIT view's picker and keeps its shape exactly — untagged Go names,
	// registered plays only. /api/plays is the product listing: registered AND staged, with the
	// lifecycle words. Two endpoints because they answer two different questions; a picker that
	// offered a staged play would offer something `Resolve` cannot find.
	mux.HandleFunc("/api/routes", ed.handleRoutes)
	mux.HandleFunc("/api/plays", ed.handlePlays)
	mux.HandleFunc("/api/register", ed.handleRegister) // stage → askable
	mux.HandleFunc("/api/load", ed.handleLoad)         // switch which play is being edited
	mux.HandleFunc("/api/do", ed.handleDo)             // run a play for real
	mux.HandleFunc("/api/run", ed.handleRun)           // what became of a clicked Run
	mux.HandleFunc("/api/bindings", ed.handleBindings)
	mux.HandleFunc("/api/bind", ed.handleBind)
	mux.HandleFunc("/api/unbind", ed.handleUnbind)
	mux.HandleFunc("/api/scope", ed.handleScope)     // move a play between context/focus/global
	mux.HandleFunc("/api/delete", ed.handleDelete)   // forget a play
	mux.HandleFunc("/api/oconfig", ed.handleOConfig) // read/write the overlay settings
	// WHAT MARCO HAS DONE, and who could act. Both are READS of accounts that already exist —
	// see activity.go and cast.go; neither keeps a record of its own.
	mux.HandleFunc("/api/activity", ed.handleActivity)
	mux.HandleFunc("/api/cast", handleCast)
	// THE Learn panel. Its endpoints hold no state and decide nothing — see learnui.go.
	learnAPI(mux)
	// THE DOGFOOD LOOP: whether Marco may remember, what it just committed, and a way to
	// use it without leaving the page. See watchui.go.
	watchAPI(mux, ed)
	return mux
}

// editor holds one control-center session: the registry, the app context, and the play
// currently open in the Edit view (swappable via /api/load).
type editor struct {
	reg  routes.Registry
	app  string // foreground app when launched — the default scope for new bindings
	view string // initial view to open (help/routes/bindings/config/edit); "" = default
	mu   sync.Mutex
	rt   routes.Route
	path string
	src  string

	// runs is what became of each clicked Run. See runaccount.go — the control centre used to
	// report every run as a success the instant the process started.
	runs runAccount
}

// loadSrc reads the active play's source into e.src.
func (e *editor) loadSrc() error {
	b, err := os.ReadFile(e.path)
	if err != nil {
		return err
	}
	e.src = string(b)
	return nil
}

// scopeOf names a play's scope for display: focus (anywhere, switches), context (only in-app),
// or global (app-less, anywhere).
//
// ONE definition, in internal/plays. The copy that used to live here tested Focus BEFORE App and
// so disagreed with the folder the file actually goes in: an app-less play with the bit set read
// as "focus". Nothing produced that combination, which is exactly why it survived — and this
// handler builds Routes out of posted JSON, where it would stop being hypothetical.
func scopeOf(rt routes.Route) string { return string(plays.ScopeOf(rt)) }

// handleRoutes lists every registered play (name, app, scope) plus which one is open.
//
// The EDIT view's picker, unchanged on purpose: it offers plays to open in the step editor, so it
// must offer only what `Resolve` can reach. The product listing is handlePlays.
func (e *editor) handleRoutes(w http.ResponseWriter, _ *http.Request) {
	e.mu.Lock()
	cur := e.rt
	e.mu.Unlock()
	type row struct {
		Name, App, Scope string
		Current          bool
	}
	var rows []row
	for _, rt := range e.reg.List() {
		rows = append(rows, row{prettyRoute(rt.Slug), rt.App, scopeOf(rt), rt == cur})
	}
	writeJSON(w, rows)
}

// playRow is one play as the Plays view is shown it.
//
// Every word here comes out of internal/plays — kind, scope and lifecycle, each as a machine value
// AND as the sentence the product says. Nothing on this row is derived locally, because a second
// derivation is a second account: the Learn panel and this list once disagreed about the same file
// precisely because each computed its own wording.
//
// The JSON tags are EXPLICIT, unlike the older handlers' untagged Go names, so renaming a Go field
// cannot silently blank a column in the page.
type playRow struct {
	Name string `json:"name"`
	// Slug is the only safe handle for an action — see plays.Play.Slug.
	Slug         string `json:"slug"`
	App          string `json:"app"`
	Kind         string `json:"kind"`
	KindWord     string `json:"kindWord"`
	KindSays     string `json:"kindSays"`
	Scope        string `json:"scope"`
	ScopeWord    string `json:"scopeWord"`
	ScopeSays    string `json:"scopeSays"`
	Life         string `json:"life"`
	LifeWord     string `json:"lifeWord"`
	LifeSays     string `json:"lifeSays"`
	Registered   bool   `json:"registered"`
	Registerable bool   `json:"registerable"`
	Askable      bool   `json:"askable"`
	Activates    string `json:"activates"`
	Current      bool   `json:"current"`
}

// bindingRow is one hotkey that REACHES a play.
//
// Alongside the rows, never among them. A binding is a trigger — routes.Binding — and a play is a
// behaviour; a list that mixed them would invite forgetting a key in order to forget a macro.
type bindingRow struct {
	App string `json:"app"`
	Key string `json:"key"`
	Cmd string `json:"cmd"`
}

// handlePlays is the product listing: every play Marco has, registered and staged together, with
// the words the product uses for each.
//
// READ-ONLY on disk, and that is the property to protect — `plays.List` is os.ReadDir/ReadFile/Stat
// and nothing else, and nothing added here may change that.
//
// Deleting the staged half must fail TestThePlaysListingShowsRegisteredAndStagedPlaysDifferently.
func (e *editor) handlePlays(w http.ResponseWriter, _ *http.Request) {
	e.mu.Lock()
	cur := e.rt
	e.mu.Unlock()
	rows := []playRow{}
	for _, p := range plays.List(e.reg) {
		rows = append(rows, playRow{
			Name: p.Name, Slug: p.Slug, App: p.Application,
			Kind: string(p.Kind), KindWord: plays.KindWord(p.Kind), KindSays: plays.KindSays(p.Kind),
			Scope: string(p.Scope), ScopeWord: p.Scope.Word(), ScopeSays: p.Scope.Says(p.Application),
			Life: string(p.Life), LifeWord: p.Life.Word(), LifeSays: p.Life.Says(),
			Registered: p.Registered, Registerable: p.Life.Registerable(), Askable: p.Life.Askable(),
			Activates: p.Activates,
			// A STAGED play is never "open": e.rt names a registered location, and a staged
			// Route carries routes.LearnedFocus as an intention rather than a place.
			Current: p.Registered && p.Route == cur,
		})
	}
	binds := []bindingRow{}
	for _, b := range e.reg.Bindings() {
		cmd := b.Cmd
		if cmd == "" {
			cmd = b.Slug // legacy single-play binding
		}
		binds = append(binds, bindingRow{App: b.App, Key: b.Key, Cmd: cmd})
	}
	writeJSON(w, map[string]any{"plays": rows, "bindings": binds})
}

// unprefixRoutes strips the package name out of a registry error before a person reads it.
//
// `internal/routes` prefixes its errors "routes: " — correct for a log, wrong in a sentence shown
// beside a Register button, where it names a package the Audience has no word for.
func unprefixRoutes(s string) string {
	if rest, ok := strings.CutPrefix(s, "routes: "); ok {
		return rest
	}
	return s
}

// handleRegister moves a saved play to where the resolver looks, making it askable.
//
// # It acts on the SLUG, never on a display name
//
// A staged play's slug came from the phrase the Audience used when Learn saved it. `plays.Pretty`
// is lossy in the direction that matters — re-deriving a slug from a shown name can land on a
// different file, or on none — so the listing carries the slug and the button posts it back
// unchanged. See plays.Play.Slug.
//
// Deleting the Slug field and slugging req name instead must fail
// TestRegisteringActsOnTheSlugTheListingCarried.
func (e *editor) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct{ Slug, App string }
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Slug) == "" {
		http.Error(w, "bad request", 400)
		return
	}
	rt := routes.Route{App: req.App, Focus: routes.LearnedFocus, Slug: strings.TrimSpace(req.Slug)}
	if err := e.reg.Register(rt); err != nil {
		// The registry REFUSED, and the page must say so. A collision leaves the play exactly
		// where it was — saved, not askable — and reporting success here would be the one lie
		// this whole staging design exists to prevent.
		//
		// Deleting this arm must fail TestARefusedRegistrationStillShowsThePlayAsSaved.
		http.Error(w, unprefixRoutes(err.Error()), 409)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "name": prettyRoute(rt.Slug)})
}

// handleLoad switches the Edit view to another play by name, reloading its source.
func (e *editor) handleLoad(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name string }
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "bad request", 400)
		return
	}
	rt, ok := e.reg.Resolve(e.app, req.Name)
	if !ok {
		if rt, ok = findRouteByName(e.reg, req.Name); !ok {
			http.Error(w, "no such play", 404)
			return
		}
	}
	e.mu.Lock()
	e.rt, e.path = rt, e.reg.Path(rt)
	err := e.loadSrc()
	e.mu.Unlock()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "name": prettyRoute(rt.Slug)})
}

// doArgv is the command line a clicked Run launches, and it carries an IDENTITY.
//
// # Why not the shown name
//
// Two reasons, and the second one is not hypothetical.
//
// The shown name is derived from the slug by `plays.Pretty`, which undoes exactly one half of the
// fold `routes.Slug` applied — dashes back to spaces, and nothing else. A slug carrying anything
// else renders to a name that slugs to a DIFFERENT slug, so posting the name and letting the
// intake match it again can land on a neighbouring play or on none.
//
// And the name carries no SCOPE. `marco do "<phrase>"` resolves against the foreground
// application, which for this surface is the person's BROWSER — the control centre is a local web
// page. A play scoped to the app it drives is not reachable from there by name at all. The row
// already holds slug, app and scope; handing all three over is the surface saying which play it
// means instead of describing it in words that have to be guessed back.
//
// `--source` records that this arrived from the control centre. It is recorded and never
// consulted: see [[internal/invoke]].
//
// Deleting the `--play=` argument — spelling the play as a phrase again — must fail
// TestAClickedRunSpawnsAnExplicitIdentity.
func doArgv(rt routes.Route) []string {
	args := []string{"do", "--source=" + string(invoke.SourceControlCentre), "--play=" + rt.Slug}
	if rt.App != "" {
		args = append(args, "--app="+rt.App)
	}
	if rt.Focus {
		args = append(args, "--focus")
	}
	return args
}

// runSpawn starts a fresh marco process, like the overlay does — the engine stays out of this
// server's process.
//
// A package variable for the same reason `submitPhrase` in intake.go is one: a test has to be able
// to prove WHAT a clicked Run would launch without launching it. `marco do` performs real input.
// Production never reassigns it.
// It now hands back the process AND its combined output, because the caller has to read the
// engine's `[result] ` line to know what actually happened — see runaccount.go. Starting a child
// and walking away was how this surface came to report every run as a success.
var runSpawn = spawnMarco

func spawnMarco(args []string) (*exec.Cmd, io.Reader, error) {
	self, err := os.Executable()
	if err != nil {
		self = os.Args[0]
	}
	cmd := exec.Command(self, args...)
	// Both streams, into one reader. The result line goes to stdout and a play's own complaints
	// often go to stderr, and a person reading "failed" deserves whichever of them said why.
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return cmd, pipe, nil
}

// doTarget picks the registered play a Run names, out of the handle the row already carried.
//
// # A staged play is refused HERE, not by the absence of a button
//
// The Plays view offers no run button on a staged row, and that is a rendering decision — a
// rendering decision is not enforcement, because anything can post to this URL. `plays.Registered`
// is the enumeration of the plays anything can ASK FOR (every standing it can produce is
// `Life.Askable`), so a staged play is structurally unable to be found here rather than filtered
// out of a mixed list. See [[ADR-028-a-learned-play-is-a-file-with-a-past]].
//
// A second askability check over the same list would be belt and braces that can never fire — and
// would make the mutation that matters survive, which is how this was actually settled: swapping
// `Registered` for `List` left every test green while the redundant filter was here.
//
// The SCOPE disambiguates: the same slug can be registered under one app both in-context and as a
// focus play, and the row knows which one it is showing. Without one, the first match stands.
//
// Widening this to `plays.List` must fail TestAStagedPlayCannotBeRunThroughTheEndpoint.
func (e *editor) doTarget(slug, app, scope string) (routes.Route, bool) {
	if slug == "" {
		return routes.Route{}, false
	}
	var first routes.Route
	found := false
	for _, p := range plays.Registered(e.reg) {
		if p.Slug != slug || p.Application != app {
			continue
		}
		if scope != "" && string(p.Scope) == scope {
			return p.Route, true
		}
		if !found {
			first, found = p.Route, true
		}
	}
	return first, found
}

// handleDo runs a play for real (types/clicks). The row's own handle decides which one.
func (e *editor) handleDo(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name, Slug, App, Scope string }
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "bad request", 400)
		return
	}
	var (
		rt routes.Route
		ok bool
	)
	if slug := strings.TrimSpace(req.Slug); slug != "" {
		rt, ok = e.doTarget(slug, strings.TrimSpace(req.App), strings.TrimSpace(req.Scope))
	} else if name := strings.TrimSpace(req.Name); name != "" {
		// COMPATIBILITY with a name-only payload, which is what this endpoint used to take and
		// what anything outside this page may still post. It resolves the name to a real Route
		// here — so even the old shape leaves this handler as an identity rather than as words
		// that would be matched a second time downstream.
		if rt, ok = e.reg.Resolve(e.app, name); !ok {
			rt, ok = findRouteByName(e.reg, name)
		}
	}
	if !ok {
		http.Error(w, "no play Marco can run answers to that", 404)
		return
	}
	// THE ANSWER IS AN ID, NOT A VERDICT.
	//
	// This used to answer `{"ok":true}` the instant the process was started, and the page said
	// "running: X" for ever. A declined play, a stopped play, a play that refused because it
	// could not recognise the screen, and a play that worked all rendered the same. See
	// runaccount.go for why the id, and why not a blocking handler.
	//
	// Deleting the run id — answering ok again — must fail TestAClickedRunReportsWhatTheEngineSaid.
	id, err := e.runs.start(rt)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "run": id, "name": prettyRoute(rt.Slug)})
}

// handleRun answers "what became of the run I started".
//
// A read, and a cheap one: the page asks about one id until it is finished. It reports the
// ENGINE'S OWN WORD, from internal/outcome, which is the same vocabulary the HUD renders — so the
// two surfaces cannot describe one run differently without failing to compile.
func (e *editor) handleRun(w http.ResponseWriter, r *http.Request) {
	rec, ok := e.runs.get(strings.TrimSpace(r.URL.Query().Get("id")))
	if !ok {
		http.Error(w, "no such run", 404)
		return
	}
	writeJSON(w, map[string]any{
		"done":    rec.Done,
		"outcome": string(rec.Outcome),
		"play":    prettyRun(rec),
		"detail":  rec.Detail,
	})
}

// handleBindings lists every leader-hotkey binding (key → command, scoped to an app).
func (e *editor) handleBindings(w http.ResponseWriter, _ *http.Request) {
	type row struct{ App, Key, Cmd string }
	var rows []row
	for _, b := range e.reg.Bindings() {
		cmd := b.Cmd
		if cmd == "" {
			cmd = b.Slug // legacy single-play binding
		}
		rows = append(rows, row{b.App, b.Key, cmd})
	}
	writeJSON(w, map[string]any{"bindings": rows, "app": e.app})
}

// bindScope infers which app a new binding belongs to, out of the play its command starts with.
//
// The scope is the PLAY's app, so a hotkey only fires while that app is in front — the same key
// can drive different macros in different games (overloading). A global play gives "" and so a
// global binding.
//
// It splits the chain and strips the arguments the SAME way a press does — `routes.SplitChain`
// then `routes.ParseInvocation` — because the old hand-rolled " then " scan handed `findRouteByName`
// a step with its arguments still attached ("greet name:sam"), which matches nothing, so a bound
// play in an app was silently scoped global.
func bindScope(reg routes.Registry, cmd string) string {
	steps := routes.SplitChain(cmd)
	if len(steps) == 0 {
		return ""
	}
	base, _, _ := routes.ParseInvocation(steps[0])
	if rt, ok := findRouteByName(reg, base); ok {
		return rt.App
	}
	return ""
}

// handleBind binds `leader+key` to a command — after proving the key press could ever do anything.
//
// # It reported success without validating anything
//
// This handler used to store whatever was posted and answer `{"ok":true}`. `findRouteByName` was
// consulted only to guess the app, so a MISS was not a refusal: the binding landed as a global one
// pointing at a play that does not exist, the page said "bound", and the person pressed that key
// for ever while nothing happened. `pressHotkey` skips a step it cannot resolve — deliberately, so
// a key over a game is never a question — which is exactly what makes an unvalidated binding
// silent rather than noisy.
//
// # Validated by the code the press uses, not by a copy of its rule
//
// `bindKey` (bind.go) is that code. It walks `routes.SplitChain`, takes each step's base with
// `routes.ParseInvocation`, and demands `Reg.Resolve` — the EXACT resolve the press will make,
// with no fuzzy score — before it stores anything. It was written for this defect, one surface
// over. Calling it is the whole point: a second validation here would be a second rule, and the
// scope a binding LANDS in could drift from the scope a press LOOKS IN without anything noticing.
//
// The two call sites differ in one thing only, and it is not the rule: `marco bind` scopes to
// `winctx.Active()` — the app you are looking at when you type the command — while the control
// centre is a local web page, so the foreground app is the person's BROWSER and the scope has to
// come from the play instead (bindScope). They would become one call site the day binding takes
// its scope from the play everywhere; until then `bindKey` is the shared half, and it is the half
// that decides.
//
// `bindKey` prints its confirmation to stdout, which for `marco edit` is the terminal the person
// launched the control centre from. That is a log line, not the answer — the answer is this JSON.
//
// Deleting the bindKey call — storing the binding directly again — must fail
// TestABindingThePressCouldNeverResolveIsRefused.
func (e *editor) handleBind(w http.ResponseWriter, r *http.Request) {
	var req struct{ App, Key, Cmd string }
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Key == "" || strings.TrimSpace(req.Cmd) == "" {
		http.Error(w, "key and command required", 400)
		return
	}
	cmd := strings.TrimSpace(req.Cmd)
	app := strings.TrimSpace(req.App)
	if app == "" {
		app = bindScope(e.reg, cmd)
	}
	if err := bindKey(orchestrator.Deps{Reg: e.reg}, app, strings.ToLower(req.Key), cmd); err != nil {
		// 400 and the engine's own sentence — "no play matches …" is the one unknown-command
		// wording the rest of the product prefix-matches, and a person reading it beside the
		// field they typed into can act on it.
		http.Error(w, unprefixRoutes(err.Error()), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "app": app})
}

// handleUnbind removes a binding.
func (e *editor) handleUnbind(w http.ResponseWriter, r *http.Request) {
	var req struct{ App, Key string }
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Key == "" {
		http.Error(w, "bad request", 400)
		return
	}
	if err := e.reg.Unbind(req.App, strings.ToLower(req.Key)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// handleDelete forgets a play — registered or staged — together with everything that described it:
// its .marco, its recording, its anchor templates AND its provenance sidecar.
//
// # Why Unregister rather than Delete
//
// `reg.Delete` deliberately leaves the `.origin.json` behind, which is right for a MOVE (the past
// is about to be rewritten beside the new copy) and wrong for a forget: it left an orphaned sidecar
// that no command a person has could ever remove, and that a later unrelated play saved under the
// same slug would sit next to. `reg.Unregister` is the documented door for "this play is gone" and
// removes the provenance, the staged pair and the play in one call.
//
// Deleting the Unregister call — going back to Delete — must fail
// TestForgettingAPlayLeavesNoOrphanedProvenance.
func (e *editor) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name, Slug, App, Scope string
		Staged                 bool
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "bad request", 400)
		return
	}
	// The SLUG when the listing carried one; the shown name only as the compatibility path for
	// the older payload. See plays.Play.Slug for why re-deriving a slug can miss.
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = routes.Slug(req.Name)
	}
	if slug == "" {
		http.Error(w, "bad request", 400)
		return
	}
	rt := routes.Route{App: req.App, Focus: req.Scope == "focus", Slug: slug}
	// EACH ROW FORGETS ITS OWN FILES, and this is not a detail.
	//
	// The Plays list shows a registered play and a staged play of the same name as two rows,
	// because they are two files with two different standings — and that is the NORMAL position
	// for a staged play, since registration is refused on a name collision. Forgetting either row
	// through `reg.Unregister` reached both: Unregister is "Marco no longer has this play at all",
	// and it removes the registered play, its provenance AND the staged copy. So forgetting the
	// saved row deleted the working play, and following the registry's own advice — "rename the
	// learned play or remove the other one first" — destroyed the learned play it was telling you
	// to keep. Both were silent, both returned 200.
	//
	// Deleting either branch's own door must fail
	// TestForgettingAStagedPlayLeavesTheRegisteredOneAlone.
	var err error
	if req.Staged {
		// A staged play lives in `<app>/learned/`, so `reg.Has` — which asks about the
		// REGISTERED location — would answer no for every one of them. The listing's own
		// enumeration is the honest existence check.
		rt = routes.Route{App: req.App, Focus: routes.LearnedFocus, Slug: slug}
		if _, ok := plays.Find(e.reg, slug, req.App, true); !ok {
			http.Error(w, "no such play", 404)
			return
		}
		err = e.reg.DeleteStaged(rt)
	} else {
		if !e.reg.Has(rt) {
			http.Error(w, "no such play", 404)
			return
		}
		// The play AND its provenance, so forgetting leaves no sidecar describing a file that
		// is not there — and nothing in the staging directory, which is a different play.
		if err = e.reg.DeleteOrigin(rt); err == nil {
			err = e.reg.Delete(rt)
		}
	}
	if err != nil {
		http.Error(w, unprefixRoutes(err.Error()), 500)
		return
	}
	e.mu.Lock()
	if e.rt == rt { // the open play is gone — fall back to the browser
		e.rt, e.path, e.src = routes.Route{}, "", ""
	}
	e.mu.Unlock()
	writeJSON(w, map[string]any{"ok": true})
}

// handleScope moves a play between scopes — context (in-app), focus (anywhere, switches to the
// app), or global (app-less). It relocates the .marco, its recording AND its provenance to the new
// scope dir: save at the destination, carry the recording, delete the source. CurApp/CurScope
// identify which play (same slug can exist in several scopes); App is the destination app
// (context/focus need one).
//
// # Provenance travels with the file
//
// It did not, and that was a defect: the copy carried the source and the recording, `Delete` left
// the `.origin.json` behind, and so changing a learned play's scope silently stripped its past —
// it re-listed as Authored — while an unreachable sidecar stayed behind in the old directory.
// Moving a play is not supposed to change what it IS.
//
// Deleting the SaveWithOrigin arm must fail TestChangingScopeKeepsALearnedPlayLearned.
func (e *editor) handleScope(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name, Slug, CurApp, CurScope, Scope, App string }
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "bad request", 400)
		return
	}
	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		slug = routes.Slug(req.Name)
	}
	if slug == "" {
		http.Error(w, "bad request", 400)
		return
	}
	from := routes.Route{App: req.CurApp, Focus: req.CurScope == "focus", Slug: slug}
	if !e.reg.Has(from) {
		http.Error(w, "no such play", 404)
		return
	}
	app := strings.TrimSpace(req.App)
	if app == "" {
		app = from.App // keep the existing app unless the caller supplies a new one
	}
	var to routes.Route
	switch req.Scope {
	case "global":
		to = routes.Route{Slug: from.Slug}
	case "context":
		to = routes.Route{App: app, Slug: from.Slug}
	case "focus":
		to = routes.Route{App: app, Focus: true, Slug: from.Slug}
	default:
		http.Error(w, "scope must be context, focus, or global", 400)
		return
	}
	if to.App == "" && req.Scope != "global" {
		http.Error(w, req.Scope+" scope needs an app", 400)
		return
	}
	if to == from {
		writeJSON(w, map[string]any{"ok": true})
		return
	}
	if e.reg.Has(to) {
		http.Error(w, "a play already exists at that scope", 409)
		return
	}
	src, err := os.ReadFile(e.reg.Path(from))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := e.reg.Save(to, string(src)); err != nil {
		http.Error(w, unprefixRoutes(err.Error()), 500)
		return
	}
	if rec, ok := e.reg.LoadRecording(from); ok {
		_ = e.reg.SaveRecording(to, rec)
	}
	// THE PLAY'S PAST MOVES WITH IT, and it moves VERBATIM.
	//
	// This handler used to carry the `.marco` and the `.rec.json` and leave the `.origin.json`
	// behind, and `Delete` does not remove one — so changing a learned play's scope stripped its
	// provenance (it re-listed as Authored) and left an orphaned sidecar under the old scope for
	// the next play saved there. Now that the Plays surface shows kind and provenance, that is
	// visible the first time anybody touches the scope control.
	//
	// `MoveOrigin` copies the bytes rather than rebuilding them, which is the difference between
	// carrying the past and rewriting it: a play the person has edited stays `edited`.
	//
	// BEFORE Delete, not after: `locDir` falls back to the legacy loose location only while the
	// `.marco` is still there, so the old sidecar has to be reached while the play still is.
	//
	// Deleting this must fail TestChangingScopeKeepsALearnedPlayLearned.
	if err := e.reg.MoveOrigin(from, to); err != nil {
		http.Error(w, unprefixRoutes(err.Error()), 500)
		return
	}
	if err := e.reg.Delete(from); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	e.mu.Lock()
	if e.rt == from { // keep the open play pointing at its new home
		e.rt, e.path = to, e.reg.Path(to)
	}
	e.mu.Unlock()
	writeJSON(w, map[string]any{"ok": true})
}

// sleepRE matches a top-level wait step: `do OS's Sleep with <ms>.` (any indent). The captured
// group is the millisecond value the editor exposes.
var sleepRE = regexp.MustCompile(`(?m)^(\s*do OS's Sleep with )(\d+)(\.\s*)$`)

// pointDeclRE matches a Point local declaration: `the <name> is a Point with <args>.` The args
// carry the click's X,Y (and window-relative RelX,RelY). Captured: lead, name, " is a Point
// with ", args, trailing ".".
var pointDeclRE = regexp.MustCompile(`(?m)^(\s*the )(\w+)( is a Point with )([^\n.]*)(\.\s*)$`)

// clickPointRE matches a top-level click/move at a named Point, so the editor can attach that
// point's coordinates to the step.
var clickPointRE = regexp.MustCompile(`^do OS's (Click|Move) with (\w+)\.`)

// textActRE matches an OS action whose only argument is a string literal the editor exposes for
// in-place editing — the whole "one text arg" slice of the harness: a keypress (Key), a hold /
// release (KeyDown/KeyUp), typed text (Type), a named Secret, app focus/launch (Activate/Launch),
// and a navigation meaning (Navigate). Captured: the verb, then the literal.
//
// Navigate is here because a LEARNED play is made of almost nothing else, and "the demonstration
// went one row too far" is the single most likely correction anybody will want to make to one.
// A meaning Marco has no host for refuses at run time exactly as an unknown key name does.
var textActRE = regexp.MustCompile(`^do OS's (Key|KeyDown|KeyUp|Type|Secret|Activate|Launch|Navigate) with "(.*)"\.`)

// repeatRE matches a `repeat N times...` loop header (any indent). Captured: indent, count.
var repeatRE = regexp.MustCompile(`^(\s*)repeat (\d+) times\.\.\.\s*$`)

// showingRE matches a SCREEN CONDITION — `do Screen's Showing with "…"...` — which is the one
// sentence Marco has for "the play is where it said it would be". A learned play carries two of
// them: the entry condition it begins inside, and the postcondition it must reach to end `ok`.
// See [[ADR-030-a-play-says-where-it-begins]]. Captured: the screen's name.
//
// The capability is spelled ONCE, in `marcoexec`, which is where the generator reads it from — so
// the editor cannot come to recognise a sentence the Director no longer writes.
var showingRE = regexp.MustCompile(`^do ` + regexp.QuoteMeta(marcoexec.StartsOn) + ` with "(.*)"\.\.\.$`)

// activateTargetRE matches a press-a-control-by-name step: `do Theater's Activate with target1.`
// The captured local is the Target that carries the control's name.
var activateTargetRE = regexp.MustCompile(`^do Theater's Activate with (\w+)\.$`)

// targetDeclRE matches a Target local — `the target1 is a Target with Name "Mouse", Kind "button".`
// Captured: the local's name, then the control's name.
var targetDeclRE = regexp.MustCompile(`(?m)^\s*the (\w+) is a Target with Name "((?:[^"\\]|\\.)*)"`)

// pointVals holds a Point's editable coordinates.
type pointVals struct {
	X, Y, RelX, RelY int
	HasRel           bool
}

// step is one entry shown in the editor, in play order: an ACTION (read-only label), a WAIT
// (editable ms), or a CHECK. A click/move action also carries the Point it targets and that
// point's X,Y, which the editor exposes as editable coordinates.
//
// A CHECK is a screen condition — where the play says it begins, and where it says it must
// arrive. It is shown and it is not editable: those two lines are the play's claim about the
// world, not a step in the procedure, and the steps between them only mean anything in their
// company. Hiding them was how a learned play came to render as one unreadable row.
type step struct {
	Kind    string `json:"kind"` // "action" | "wait" | "check"
	Label   string `json:"label,omitempty"`
	Act     string `json:"act,omitempty"`   // editable subtype: click|move|key|keydown|keyup|type|secret|activate|launch|repeat
	Ms      int    `json:"ms,omitempty"`    // wait: milliseconds
	Count   int    `json:"count,omitempty"` // repeat: iteration count
	Point   string `json:"point,omitempty"` // click/move: the Point name (empty otherwise)
	X       int    `json:"x"`               // click/move: that point's coordinates
	Y       int    `json:"y"`
	Text    string `json:"text,omitempty"`  // key/keydown/keyup/type/secret/activate/launch: the literal
	Depth   int    `json:"depth,omitempty"` // 0 = top level, 1 = inside a repeat block (indent in the UI)
	CanDrag bool   `json:"canDrag"`         // a click/move at a point can be converted to a drag
	EndLine int    `json:"-"`               // repeat: last source line of the block (delete cascades to it)
	Line    int    `json:"line"`            // source line index (keys delete / wait / count / drag ops)
}

// bodyIndent is the indentation at which THIS play's steps live.
//
// # Why it is not just 4
//
// Because a play that says where it begins puts its whole procedure INSIDE that claim. Director
// generates exactly that shape — the entry condition at indent 4, `when ok?` at 8, and every step
// at 12 — for a structural reason it will not be giving up: there is no line after the block for
// control to fall through to, so the only path to an effect is through an `ok`. See
// [[ADR-030-a-play-says-where-it-begins]] and internal/director/marcoexec/play.go.
//
// The editor recognised indent 4 (and 8 inside a repeat) and nothing else, so EVERY learned play
// rendered as one row — the entry condition, unlabelled, with its actual steps invisible. The
// person could not see what Marco had learned, let alone change it, in the one surface built for
// changing plays. Phase 3 made that editing meaningful: an edited learned play stops being the
// artifact Director verified and runs locally against its own source, so it is an ordinary play
// and has to be editable like one.
//
// The rule is structural rather than a list of known shapes: if the play's first top-level
// statement is a screen condition whose `ok` arm opens immediately beneath it, the steps are two
// levels in. Anything else — every recorded and hand-written play — is the ordinary 4.
//
// Deleting the entry-condition arm must fail TestALearnedPlayShowsItsStepsInTheEditor.
func bodyIndent(src string) int {
	const top = 4
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || len(line)-len(strings.TrimLeft(line, " ")) != top {
			continue
		}
		if !showingRE.MatchString(t) {
			return top // an ordinary statement at the top level: the steps are here
		}
		for _, next := range lines[i+1:] {
			nt := strings.TrimSpace(next)
			if nt == "" {
				continue
			}
			if nt == "when ok?" && len(next)-len(strings.TrimLeft(next, " ")) == top+4 {
				return top + 8
			}
			break
		}
		return top
	}
	return top
}

// parseSteps walks the play body and returns the ordered step sequence the editor shows: the
// actions/waits at the play's body indent, plus one level of `repeat N times...` block — its
// header (editable count) and its indented body steps (Depth 1). A Find block's `when ok?/or?`
// arms and the point/anchor/target declarations stay hidden. Each editable action carries its
// subtype + fields.
//
// A play's screen conditions are shown as read-only CHECK rows: the entry condition above the
// steps it guards, the postcondition below them. They are the play's claim about the world, and a
// procedure read without them is a procedure read out of context.
func (e *editor) parseSteps() []step {
	pts := e.parsePoints()
	tgts := e.parseTargets()
	body := bodyIndent(e.src)
	var steps []step
	lines := strings.Split(e.src, "\n")
	repeatBodyIndent := -1 // indent of the current repeat block's body (-1 = not in one)
	repeatHdr := -1        // index into steps of the open repeat header (to extend its EndLine)
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		// A repeat block ends when indentation returns to (or above) its header level.
		if repeatBodyIndent >= 0 && indent < repeatBodyIndent {
			repeatBodyIndent, repeatHdr = -1, -1
		}
		inBody := repeatBodyIndent >= 0 && indent == repeatBodyIndent
		depth := 0
		if inBody {
			depth = 1
			steps[repeatHdr].EndLine = i // grow the block to cover this body line
		}
		// A `repeat N times...` header at the body indent: editable count, body follows indented.
		if m := repeatRE.FindStringSubmatch(line); m != nil && indent == body {
			n, _ := strconv.Atoi(m[2])
			steps = append(steps, step{Kind: "action", Act: "repeat", Count: n,
				Label: fmt.Sprintf("Repeat %d times", n), Line: i, EndLine: i})
			repeatBodyIndent, repeatHdr = body+4, len(steps)-1
			continue
		}
		// THE ENTRY CONDITION sits one level above the steps it guards, so it is the one line
		// worth showing from outside the body. Nothing else up there is a step.
		if body > 4 && indent == 4 {
			if m := showingRE.FindStringSubmatch(t); m != nil {
				steps = append(steps, step{Kind: "check", Line: i,
					Label: fmt.Sprintf("Starts on %q", unquoteLit(m[1]))})
			}
			continue
		}
		show := indent == body || inBody
		if !show {
			continue
		}
		if m := sleepRE.FindStringSubmatch(line); m != nil {
			ms, _ := strconv.Atoi(m[2])
			steps = append(steps, step{Kind: "wait", Ms: ms, Depth: depth, Line: i})
			continue
		}
		if m := showingRE.FindStringSubmatch(t); m != nil {
			steps = append(steps, step{Kind: "check", Depth: depth, Line: i,
				Label: fmt.Sprintf("Ends on %q", unquoteLit(m[1]))})
			continue
		}
		if strings.HasPrefix(t, "do ") {
			s := step{Kind: "action", Label: humanizeAction(t), Depth: depth, Line: i}
			// A click/move at a named point → attach coords for editing, allow drag conversion.
			if cm := clickPointRE.FindStringSubmatch(t); cm != nil {
				if pv, ok := pts[cm[2]]; ok {
					s.Act = strings.ToLower(cm[1]) // "click" | "move"
					s.Point, s.X, s.Y, s.CanDrag = cm[2], pv.X, pv.Y, true
				}
			} else if tm := textActRE.FindStringSubmatch(t); tm != nil {
				// A one-text-arg action (key/hold/release/type/secret/activate/launch/navigate)
				// → expose the literal for in-place editing.
				s.Act, s.Text = strings.ToLower(tm[1]), unquoteLit(tm[2])
			} else if am := activateTargetRE.FindStringSubmatch(t); am != nil {
				// Pressing a control BY NAME: the label says which control, not which local
				// happens to carry it. `target1` is an implementation detail of the sentence.
				if name, ok := tgts[am[1]]; ok {
					s.Label = fmt.Sprintf("Press %q", name)
				}
			}
			steps = append(steps, s)
		}
	}
	return steps
}

// parseTargets reads every Target declaration into a local-name → control-name map, so a step
// that presses a control by name can be labelled with the control.
func (e *editor) parseTargets() map[string]string {
	out := map[string]string{}
	for _, m := range targetDeclRE.FindAllStringSubmatch(e.src, -1) {
		out[m[1]] = unquoteLit(m[2])
	}
	return out
}

// parsePoints reads every Point declaration into a name→coordinates map.
func (e *editor) parsePoints() map[string]pointVals {
	out := map[string]pointVals{}
	for _, m := range pointDeclRE.FindAllStringSubmatch(e.src, -1) {
		out[m[2]] = parsePointArgs(m[4])
	}
	return out
}

// fieldRE pulls a `<Key> <int>` value out of a Point's arg list, using a word boundary so `X`
// does not also match inside `RelX`.
func fieldRE(key string) *regexp.Regexp { return regexp.MustCompile(`\b` + key + ` (-?\d+)`) }

var reX, reY, reRelX, reRelY = fieldRE("X"), fieldRE("Y"), fieldRE("RelX"), fieldRE("RelY")

func parsePointArgs(args string) pointVals {
	num := func(re *regexp.Regexp) (int, bool) {
		if m := re.FindStringSubmatch(args); m != nil {
			n, _ := strconv.Atoi(m[1])
			return n, true
		}
		return 0, false
	}
	var pv pointVals
	pv.X, _ = num(reX)
	pv.Y, _ = num(reY)
	if rx, ok := num(reRelX); ok {
		if ry, ok2 := num(reRelY); ok2 {
			pv.RelX, pv.RelY, pv.HasRel = rx, ry, true
		}
	}
	return pv
}

// quoteLit / unquoteLit round-trip a Marco string literal's inner text (escaping the backslash
// first so the quote-escape isn't double-processed), so an edited keypress or typed phrase stays
// a valid literal even if it contains a quote.
func quoteLit(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`)
}

func unquoteLit(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\"`, `"`), `\\`, `\`)
}

// humanizeAction turns a `do …` line into a short readable label.
func humanizeAction(line string) string {
	s := strings.TrimSuffix(strings.TrimSpace(line), "...")
	s = strings.TrimSuffix(s, ".")
	s = strings.TrimPrefix(s, "do ")
	switch {
	case strings.HasPrefix(s, "OS's Click"):
		return "Click"
	case strings.HasPrefix(s, "OS's Move"):
		return "Move cursor"
	case strings.HasPrefix(s, "OS's Type with "):
		return "Type " + strings.TrimPrefix(s, "OS's Type with ")
	case strings.HasPrefix(s, "OS's Key with "):
		return "Press " + strings.Trim(strings.TrimPrefix(s, "OS's Key with "), `"`)
	case strings.HasPrefix(s, "OS's KeyDown with "):
		return "Hold " + strings.Trim(strings.TrimPrefix(s, "OS's KeyDown with "), `"`)
	case strings.HasPrefix(s, "OS's KeyUp with "):
		return "Release " + strings.Trim(strings.TrimPrefix(s, "OS's KeyUp with "), `"`)
	case strings.HasPrefix(s, "OS's Activate with "):
		return "Focus " + strings.Trim(strings.TrimPrefix(s, "OS's Activate with "), `"`)
	case strings.HasPrefix(s, "OS's Launch with "):
		return "Launch " + strings.Trim(strings.TrimPrefix(s, "OS's Launch with "), `"`)
	case strings.HasPrefix(s, "OS's Secret with "):
		return "Secret " + strings.Trim(strings.TrimPrefix(s, "OS's Secret with "), `"`)
	case strings.HasPrefix(s, "OS's Navigate with "):
		return "Navigate " + strings.Trim(strings.TrimPrefix(s, "OS's Navigate with "), `"`)
	case strings.HasPrefix(s, "Theater's Activate with "):
		// The local's name, only as a fallback: parseSteps resolves it to the control's own
		// name when the declaration is there to read.
		return "Press a control"
	case strings.HasPrefix(s, "OS's Drag"):
		return "Drag"
	case strings.HasPrefix(s, "OS's Find") || strings.HasPrefix(s, "Text's Find") || strings.HasPrefix(s, "Vision's Locate"):
		return "Find target, then click"
	}
	return s
}

func (e *editor) handleRoute(w http.ResponseWriter, _ *http.Request) {
	e.mu.Lock()
	defer e.mu.Unlock()
	writeJSON(w, map[string]any{
		"loaded": e.rt.Slug != "",
		"view":   e.view,
		"name":   prettyRoute(e.rt.Slug),
		// The SLUG, beside the shown name, so the Edit view's Run can name the play it has open
		// instead of describing it — the identity the server already holds, handed to the page
		// that will hand it straight back. See doArgv.
		//
		// Deleting it must fail TestTheEditViewRunsThePlayItHasOpenByIdentity.
		"slug":   e.rt.Slug,
		"app":    e.rt.App,
		"scope":  scopeOf(e.rt),
		"path":   e.path,
		"steps":  e.parseSteps(),
		"source": e.src,
	})
}

// saveReq is the editor's save payload. Everything is keyed by SOURCE LINE INDEX (or point
// name), so operations don't shift when steps are deleted.
type saveReq struct {
	Waits   map[string]int    `json:"waits"`   // line → new ms (Sleep)
	Repeats map[string]int    `json:"repeats"` // line → new count (repeat header)
	Points  map[string][2]int `json:"points"`  // point name → [x, y]
	Texts   map[string]string `json:"texts"`   // line → new one-text-arg literal (key/type/…)
	Deletes []int             `json:"deletes"` // line indexes to remove (a repeat header cascades to its body)
	Drags   map[string][4]int `json:"drags"`   // line → [fromX, fromY, toX, toY] (click → drag)
	Adds    []addStep         `json:"adds"`    // new steps to insert
}

// addStep is one new command the editor inserts. After is the source-line index to insert it
// AFTER (-1 = at the end of the body, just before `this is ok!`). Act selects which fields matter;
// a "repeat" wraps a single inner action (Inner + the shared fields) in a `repeat Count times`.
type addStep struct {
	After int    `json:"after"`
	Act   string `json:"act"` // wait|click|move|key|keydown|keyup|type|secret|activate|launch|drag|repeat
	Ms    int    `json:"ms"`
	Count int    `json:"count"` // repeat: iterations
	Inner string `json:"inner"` // repeat: the inner action's act
	X     int    `json:"x"`
	Y     int    `json:"y"`
	ToX   int    `json:"toX"`
	ToY   int    `json:"toY"`
	Text  string `json:"text"`
}

// handleSave rebuilds the source from the edits and writes it back — IF Marco will still accept it.
//
// # The save goes through the one compile gate, and a refusal is a refusal
//
// This used to rebuild the text and hand it straight to `Registry.Save`, which is a `WriteFile`
// and asks nothing. So the visual editor could write a play that does not compile: deleting the
// header of a block, editing a literal into an unterminated string, or dropping the only line an
// arm had. Nothing said so. The person closed the editor, and found out the next time they asked
// for the play — by which point they had no idea which edit did it.
//
// `driver.CheckSource` is THE gate: it resolves `use` exactly as running the play would, off the
// one module list in the resolver, which is the whole of [[ADR-005-legal-marco-only]] and the
// reason `cmd/director/learnedplay.go` calls the same function rather than assembling modules of
// its own. A second, differently-configured gate here would be worse than none — it would accept
// what the runner refuses, or refuse what the runner accepts, and either way the editor would be
// arguing with the engine.
//
// Refused means REFUSED: the file on disk is untouched, `e.src` still holds what the person is
// editing, and the compiler's own sentence travels back so it can be read where the work is. The
// page keeps the edits rather than reloading over them.
//
// Deleting the CheckSource call must fail TestASaveThatWouldNotCompileIsRefused.
func (e *editor) handleSave(w http.ResponseWriter, r *http.Request) {
	var req saveReq
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "bad request", 400)
		return
	}
	updated := e.rebuild(req)
	if err := driver.CheckSource(updated); err != nil {
		// 409, not 500: nothing went wrong here. The edit is one Marco will not accept, and the
		// only thing that can resolve it is the person changing it.
		http.Error(w, "that edit is not a play Marco can run — "+err.Error(), http.StatusConflict)
		return
	}
	if err := e.reg.Save(e.rt, updated); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	e.src = updated
	writeJSON(w, map[string]any{"ok": true})
}

// anyDeclRE matches any local declaration name (`the <name> is a …`) so new points/drags get a
// name that doesn't collide with an existing one.
var anyDeclRE = regexp.MustCompile(`(?m)^\s*the (\w+) is a`)

// indentOf returns the leading spaces of a line, so an inserted step matches the body indent.
func indentOf(line string) string { return line[:len(line)-len(strings.TrimLeft(line, " "))] }

// endAddAnchor is the line an end-of-body insertion goes BEFORE, with the indent it takes.
//
// For an ordinary play that is the closing `this is ok!`. For a play that says where it must
// arrive it is the POSTCONDITION — everything from that line down is the verification, not the
// procedure, and `this is ok!` lives inside the postcondition's own `ok` arm. Anchoring on the
// text alone put a step added with "+ add step" on the far side of the screen check, so it ran
// after the play had already decided it had arrived.
//
// (-1, "") when the play has neither, and the caller appends at the body indent as before.
func endAddAnchor(lines []string, body int) (int, string) {
	fallback := -1
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if len(line)-len(strings.TrimLeft(line, " ")) == body &&
			(t == "this is ok!" || showingRE.MatchString(t)) {
			return i, indentOf(line)
		}
		if fallback < 0 && t == "this is ok!" {
			fallback = i
		}
	}
	if fallback >= 0 {
		return fallback, indentOf(lines[fallback])
	}
	return -1, ""
}

// freshName returns "<base><n>" for the smallest n not already used, and marks it used.
func freshName(used map[string]bool, base string) string {
	for n := 1; ; n++ {
		cand := base + strconv.Itoa(n)
		if !used[cand] {
			used[cand] = true
			return cand
		}
	}
}

// genAdd renders a new step's source line(s) at the given indent. Click/move/drag mint a fresh
// local (point/drag) declaration alongside the call so the insertion is self-contained.
func genAdd(a addStep, indent string, used map[string]bool) []string {
	switch a.Act {
	case "wait":
		return []string{fmt.Sprintf("%sdo OS's Sleep with %d.", indent, max(a.Ms, 0))}
	case "key":
		return []string{fmt.Sprintf(`%sdo OS's Key with "%s".`, indent, quoteLit(a.Text))}
	case "keydown":
		return []string{fmt.Sprintf(`%sdo OS's KeyDown with "%s".`, indent, quoteLit(a.Text))}
	case "keyup":
		return []string{fmt.Sprintf(`%sdo OS's KeyUp with "%s".`, indent, quoteLit(a.Text))}
	case "type":
		return []string{fmt.Sprintf(`%sdo OS's Type with "%s".`, indent, quoteLit(a.Text))}
	case "secret":
		return []string{fmt.Sprintf(`%sdo OS's Secret with "%s".`, indent, quoteLit(a.Text))}
	case "activate":
		return []string{fmt.Sprintf(`%sdo OS's Activate with "%s".`, indent, quoteLit(a.Text))}
	case "launch":
		return []string{fmt.Sprintf(`%sdo OS's Launch with "%s".`, indent, quoteLit(a.Text))}
	case "repeat":
		// A self-contained loop wrapping one inner action.
		inner := addStep{Act: a.Inner, Text: a.Text, X: a.X, Y: a.Y, ToX: a.ToX, ToY: a.ToY, Ms: a.Ms}
		body := genAdd(inner, indent+"    ", used)
		if len(body) == 0 { // unknown inner → a no-op wait so the block stays valid Marco
			body = []string{fmt.Sprintf("%sdo OS's Sleep with 0.", indent+"    ")}
		}
		return append([]string{fmt.Sprintf("%srepeat %d times...", indent, max(a.Count, 1))}, body...)
	case "click", "move":
		p := freshName(used, "p")
		verb := "Click"
		if a.Act == "move" {
			verb = "Move"
		}
		return []string{
			fmt.Sprintf("%sthe %s is a Point with X %d, Y %d.", indent, p, a.X, a.Y),
			fmt.Sprintf("%sdo OS's %s with %s.", indent, verb, p),
		}
	case "drag":
		d := freshName(used, "drag")
		return []string{
			fmt.Sprintf(`%sthe %s is a Drag with FromX %d, FromY %d, ToX %d, ToY %d, Button "left".`, indent, d, a.X, a.Y, a.ToX, a.ToY),
			fmt.Sprintf("%sdo OS's Drag with %s.", indent, d),
		}
	}
	return nil
}

// rebuild produces the new source in one line-keyed pass: drop deleted lines, rewrite edited
// Sleep waits and key/type literals, convert a click line to a Drag, insert any added steps after
// their anchor line (or before `this is ok!` for end-adds), then patch point coordinates by name.
// Keyed by source line so a delete doesn't disturb the other edits.
func (e *editor) rebuild(req saveReq) string {
	lines := strings.Split(e.src, "\n")
	del := make(map[int]bool, len(req.Deletes))
	for _, l := range req.Deletes {
		del[l] = true
	}
	// Deleting a `repeat N times...` header cascades to its indented body (an orphaned indented
	// body would be invalid Marco).
	for _, l := range req.Deletes {
		if l < 0 || l >= len(lines) {
			continue
		}
		if m := repeatRE.FindStringSubmatch(lines[l]); m != nil {
			hdr := len(m[1])
			for j := l + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "" {
					continue
				}
				if len(lines[j])-len(strings.TrimLeft(lines[j], " ")) <= hdr {
					break
				}
				del[j] = true
			}
		}
	}
	used := map[string]bool{}
	for _, m := range anyDeclRE.FindAllStringSubmatch(e.src, -1) {
		used[m[1]] = true
	}
	addsAfter := map[int][]addStep{}
	var endAdds []addStep
	for _, a := range req.Adds {
		if a.After < 0 {
			endAdds = append(endAdds, a)
		} else {
			addsAfter[a.After] = append(addsAfter[a.After], a)
		}
	}
	out := make([]string, 0, len(lines)+2*(len(req.Drags)+len(req.Adds)))
	flush := func(as []addStep, indent string) {
		for _, a := range as {
			out = append(out, genAdd(a, indent, used)...)
		}
	}
	endAt, endIndent := endAddAnchor(lines, bodyIndent(e.src))
	for i, line := range lines {
		// End-of-body adds land just before the play's ending — see endAddAnchor.
		if i == endAt && len(endAdds) > 0 {
			flush(endAdds, endIndent)
			endAdds = nil
		}
		if del[i] {
			flush(addsAfter[i], indentOf(line)) // a step added after a now-deleted step still lands here
			continue
		}
		if d, ok := req.Drags[strconv.Itoa(i)]; ok {
			indent := indentOf(line)
			name := freshName(used, "drag")
			out = append(out,
				fmt.Sprintf(`%sthe %s is a Drag with FromX %d, FromY %d, ToX %d, ToY %d, Button "left".`, indent, name, d[0], d[1], d[2], d[3]),
				fmt.Sprintf("%sdo OS's Drag with %s.", indent, name))
			flush(addsAfter[i], indent)
			continue
		}
		if m := sleepRE.FindStringSubmatch(line); m != nil {
			if ms, ok := req.Waits[strconv.Itoa(i)]; ok {
				line = m[1] + strconv.Itoa(max(ms, 0)) + m[3]
			}
		}
		if m := repeatRE.FindStringSubmatch(line); m != nil {
			if n, ok := req.Repeats[strconv.Itoa(i)]; ok {
				line = fmt.Sprintf("%srepeat %d times...", m[1], max(n, 1))
			}
		}
		if txt, ok := req.Texts[strconv.Itoa(i)]; ok {
			if tm := textActRE.FindStringSubmatch(strings.TrimSpace(line)); tm != nil {
				line = fmt.Sprintf(`%sdo OS's %s with "%s".`, indentOf(line), tm[1], quoteLit(txt))
			}
		}
		out = append(out, line)
		flush(addsAfter[i], indentOf(line))
	}
	flush(endAdds, "    ") // no `this is ok!` found → append at body indent
	return applyPoints(strings.Join(out, "\n"), req.Points)
}

// applyPoints rewrites the X,Y of each edited Point (keyed by name), shifting its window-
// relative RelX,RelY by the SAME delta so both stay consistent (moving the absolute click by Δ
// moves the window-relative click by Δ too — the window didn't move). Unedited points and any
// non-coordinate fields are left untouched.
func applyPoints(src string, edits map[string][2]int) string {
	if len(edits) == 0 {
		return src
	}
	return pointDeclRE.ReplaceAllStringFunc(src, func(m string) string {
		sub := pointDeclRE.FindStringSubmatch(m)
		name := sub[2]
		nv, ok := edits[name]
		if !ok {
			return m
		}
		old := parsePointArgs(sub[4])
		dx, dy := nv[0]-old.X, nv[1]-old.Y
		args := fmt.Sprintf("X %d, Y %d", nv[0], nv[1])
		if old.HasRel {
			args += fmt.Sprintf(", RelX %d, RelY %d", old.RelX+dx, old.RelY+dy)
		}
		return sub[1] + name + sub[3] + args + sub[5]
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// findRouteByName matches a play across all scopes by its pretty name or slug (case-insensitive).
func findRouteByName(reg routes.Registry, name string) (routes.Route, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, rt := range reg.List() {
		if strings.ToLower(prettyRoute(rt.Slug)) == n || strings.EqualFold(rt.Slug, n) {
			return rt, true
		}
	}
	return routes.Route{}, false
}

// openBrowser best-effort opens url in the default browser (Windows/macOS/Linux); a no-op
// failure just leaves the printed URL for the user to click.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

const editPage = `<!doctype html><html><head><meta charset="utf-8"><title>MARCO</title>
<link rel="icon" type="image/png" href="/icon.png">
<style>
 :root{--bg:#0A0A0A;--panel:#121316;--panel2:#17181c;--line:#1e2a2a;--accent:#00E5CC;--text:#DDD;
   --dim:#6A9A98;--run:#4DC94D;--err:#F56565;--listen:#5AB4F5;--amber:#E0B050;--mono:ui-monospace,"Cascadia Mono",Consolas,monospace}
 *{box-sizing:border-box}
 body{font:15px/1.45 var(--mono);margin:0;background:var(--bg);color:var(--text)}
 header{position:sticky;top:0;z-index:5;padding:12px 16px;background:linear-gradient(180deg,#111 0,#0b0b0b 100%);
   border-bottom:1px solid var(--line);display:flex;gap:14px;align-items:center}
 .burger{background:none;border:1px solid var(--line);color:var(--accent);font-size:18px;line-height:1;
   padding:5px 10px;border-radius:6px;cursor:pointer}
 .burger:hover{border-color:var(--accent);box-shadow:0 0 10px rgba(0,229,204,.25)}
 .logo{width:26px;height:26px;border-radius:6px;box-shadow:0 0 10px rgba(122,0,255,.5)}
 .brand{font-weight:700;letter-spacing:3px;color:var(--accent);text-shadow:0 0 10px rgba(0,229,204,.45)}
 .sub{color:var(--dim);letter-spacing:0;font-weight:400;margin-left:10px}
 .saved{margin-left:auto;color:var(--run);font-size:13px;text-shadow:0 0 8px rgba(77,201,77,.4)}
 nav{position:fixed;left:-240px;top:0;bottom:0;width:220px;background:var(--panel);border-right:1px solid var(--line);
   transition:left .18s ease;z-index:20;padding-top:56px}
 nav.open{left:0;box-shadow:0 0 40px rgba(0,0,0,.7)}
 nav a{display:flex;gap:10px;align-items:center;padding:13px 20px;color:var(--dim);cursor:pointer;
   border-left:3px solid transparent;letter-spacing:1px;text-transform:uppercase;font-size:13px}
 nav a:hover{color:var(--text);background:var(--panel2)}
 nav a.active{color:var(--accent);border-left-color:var(--accent);background:rgba(0,229,204,.07)}
 /* THE LINE BETWEEN NORMAL AND ADVANCED, drawn once and visible. Everything under it names a
    backstage thing on purpose; everything above it is the product. */
 nav .navgroup{margin:18px 0 2px;padding:8px 20px 4px;color:var(--dim);font-size:11px;letter-spacing:2px;
   text-transform:uppercase;border-top:1px solid var(--line);opacity:.75}
 table.cast{width:100%;border-collapse:collapse;font-size:13px}
 table.cast th{text-align:left;color:var(--dim);font-weight:400;font-size:11px;letter-spacing:1.5px;
   text-transform:uppercase;border-bottom:1px solid var(--line);padding:6px 8px}
 table.cast td{padding:7px 8px;border-bottom:1px solid var(--line);vertical-align:top;color:var(--text)}
 table.cast td.no{color:var(--err)}
 table.cast td.yes{color:var(--run)}
 .empty{color:var(--dim);font-size:13px;padding:10px 0}
 .act{display:flex;gap:10px;align-items:baseline;flex-wrap:wrap;padding:9px 12px;border:1px solid var(--line);
   border-radius:6px;margin:6px 0;background:var(--panel)}
 .act .when{color:var(--dim);font-size:12px;min-width:74px}
 .act .what{color:var(--text);flex:1;min-width:180px}
 .act .why{color:var(--dim);font-size:12px;flex-basis:100%}
 .out{font-size:11px;letter-spacing:1px;text-transform:uppercase;padding:2px 7px;border-radius:4px;border:1px solid var(--line)}
 .out.performed{color:var(--run);border-color:#153a1e}
 .out.clarify{color:var(--listen);border-color:#12283a}
 .out.refused{color:var(--amber);border-color:#3a3018}
 .out.unavailable{color:var(--dim)}
 .out.cancelled{color:var(--dim)}
 .out.failed{color:var(--err);border-color:#3a1e1e}
 #scrim{position:fixed;inset:0;background:rgba(0,0,0,.5);z-index:10;display:none}
 #scrim.on{display:block}
 main{padding:20px 18px;max-width:680px}
 h2{color:var(--accent);font-size:14px;letter-spacing:2px;text-transform:uppercase;margin:0 0 14px;
   border-bottom:1px solid var(--line);padding-bottom:8px}
 .hint{color:var(--dim);font-size:13px;margin:0 0 16px}
 .action{padding:8px 12px;border:1px solid var(--line);border-radius:6px;margin:6px 0;display:flex;gap:8px;align-items:center;background:var(--panel)}
 .action.depth1{margin-left:26px;border-left:2px solid var(--accent)}
 .num{color:#4a5a5a;min-width:22px}
 .lbl{display:flex;gap:8px;align-items:center}
 .xy,.to{color:var(--listen);display:inline-flex;align-items:center;gap:5px;font-size:13px}
 .coord{width:66px;padding:5px 6px;background:#0d0e10;border:1px solid var(--line);color:var(--accent);border-radius:5px;font:inherit;text-align:right}
 .dragbox{display:inline-flex;align-items:center;gap:6px}
 .mini{background:#12312e;color:var(--accent);padding:4px 9px;border:1px solid var(--line);border-radius:5px;cursor:pointer;font:13px var(--mono)}
 .mini:hover{border-color:var(--accent)}
 .mini.on{background:var(--accent);color:#04110f}
 .ops{margin-left:auto;display:inline-flex;gap:8px;align-items:center}
 .ops .del{margin-left:0}
 .del{margin-left:auto;background:#2a1414;color:var(--err);padding:4px 9px;border:1px solid #3a1e1e;border-radius:5px;cursor:pointer;font:inherit}
 .del:hover{background:#3a1c1c}
 .trash{font-size:14px;letter-spacing:.5px}
 .deleted{opacity:.4;text-decoration:line-through}
 .added{border-color:#1e3a2a;background:#0e1a12}
 .tin{width:150px;padding:5px 7px;background:#0d0e10;border:1px solid var(--line);color:var(--accent);border-radius:5px;font:inherit;text-align:left}
 .addfields{display:inline-flex;align-items:center;gap:6px;color:var(--dim);font-size:13px;flex-wrap:wrap}
 select.mini{background:#12312e;color:var(--accent);border:1px solid var(--line);border-radius:5px;padding:4px 6px;font:13px var(--mono)}
 .plus{background:#12312e;color:var(--accent)}
 .rep{color:var(--accent)}
 .rep input{width:60px;padding:5px 6px;background:#0d0e10;border:1px solid var(--line);color:var(--accent);border-radius:5px;font:inherit;text-align:right}
 .wait{display:flex;align-items:center;gap:8px;margin:3px 0 3px 30px;color:var(--amber)}
 .wait.depth1{margin-left:56px}
 .wait input{width:90px;padding:6px 8px;background:#0d0e10;border:1px solid var(--line);color:var(--amber);border-radius:6px;font:inherit;text-align:right}
 .keep{color:var(--run);font-size:12px;margin-left:6px}
 .bar{margin-top:18px;display:flex;gap:10px;align-items:center;flex-wrap:wrap}
 /* HIDDEN MEANS HIDDEN. the hidden attribute is only display:none in the browser's own stylesheet, so
    any author rule that sets display — .bar{display:flex} above — silently beats it. Every
    show(id,false) on a bar was a no-op: the Go there box and the sentence explaining why it was
    absent appeared together, which is the page contradicting itself. */
 [hidden]{display:none!important}
 button.go{padding:9px 18px;background:var(--accent);color:#04110f;border:0;border-radius:6px;cursor:pointer;font:inherit;font-weight:700;letter-spacing:1px}
 button.go:hover{box-shadow:0 0 14px rgba(0,229,204,.5)}
 details{margin-top:22px;color:var(--dim)}
 pre{background:#070708;padding:12px;border:1px solid var(--line);border-radius:6px;white-space:pre-wrap;font:13px var(--mono);color:#b8c4c2}
 .grouphead{color:var(--accent);font-size:12px;letter-spacing:2px;text-transform:uppercase;margin:18px 0 6px;opacity:.85}
 .rowcard{display:flex;gap:10px;align-items:center;flex-wrap:wrap;padding:10px 12px;border:1px solid var(--line);border-radius:6px;margin:6px 0;background:var(--panel)}
 .rowcard .nm{color:var(--text)}
 .tag{font-size:11px;letter-spacing:1px;text-transform:uppercase;padding:2px 7px;border-radius:4px;border:1px solid var(--line);color:var(--dim)}
 .tag.context{color:var(--accent);border-color:#12312e}
 .tag.focus{color:var(--listen);border-color:#12283a}
 .tag.global{color:var(--amber);border-color:#3a3018}
 .tag.cur{color:#04110f;background:var(--accent);border-color:var(--accent)}
 /* WHERE A PLAY CAME FROM. Learned reads apart from the two a person made themselves. */
 .tag.learned{color:#B48CF2;border-color:#2c2140}
 .tag.taught{color:var(--listen);border-color:#12283a}
 .tag.authored{color:var(--dim)}
 /* WHERE IT STANDS. Green only for askable; a saved play must never look like a ready one. */
 .tag.ready{color:var(--run);border-color:#153a1e}
 .tag.edited{color:var(--run);border-color:#3a3018}
 .tag.saved{color:var(--amber);border-color:#3a3018}
 .tag.unverified,.tag.stuck{color:var(--err);border-color:#3a1e1e}
 /* A TRIGGER, not the play: keyboard-shaped, and it keeps the key's own casing. */
 .tag.hot{color:var(--accent);border-color:#12312e;background:#0d0e10;text-transform:none;letter-spacing:0}
 .lifesays{color:var(--dim);font-size:12px;max-width:320px}
 .subhead{color:var(--amber);font-size:11px;letter-spacing:2px;text-transform:uppercase;margin:12px 0 4px 2px;opacity:.9}
 .spacer{margin-left:auto}
 .key{display:inline-block;min-width:20px;text-align:center;padding:2px 7px;border:1px solid var(--line);border-radius:4px;color:var(--accent);background:#0d0e10}
 input.field{padding:7px 9px;background:#0d0e10;border:1px solid var(--line);color:var(--text);border-radius:6px;font:inherit}
 .crow{display:flex;align-items:center;gap:12px;padding:9px 0;border-bottom:1px solid var(--line)}
 .crow label{color:var(--dim);min-width:160px;font-size:13px}
 .crow input{width:150px;padding:6px 8px;background:#0d0e10;border:1px solid var(--line);color:var(--accent);border-radius:6px;font:inherit}
 .crow select{background:#12312e;color:var(--accent);border:1px solid var(--line);border-radius:5px;padding:6px 8px;font:13px var(--mono)}
 kbd{padding:1px 6px;border:1px solid var(--line);border-bottom-width:2px;border-radius:4px;color:var(--accent);background:#0d0e10;font:12px var(--mono)}
 .help h3{color:var(--text);font-size:13px;letter-spacing:1px;text-transform:uppercase;margin:18px 0 6px}
 .help p,.help li{color:var(--dim);font-size:13px}
 .help ol{color:var(--dim);font-size:13px;padding-left:20px;margin:6px 0}
 .help li{margin:3px 0}
 .help b{color:var(--text)}
 .help h3{border-top:1px solid var(--line);padding-top:14px}
 .help kbd+kbd{margin-left:3px}
 a.plain{color:var(--accent);cursor:pointer;text-decoration:none}
 .banner{position:fixed;top:14px;left:50%;transform:translateX(-50%) translateY(-70px);background:var(--run);color:#04110f;
   padding:11px 22px;border-radius:8px;font-weight:700;letter-spacing:1px;box-shadow:0 6px 24px rgba(0,0,0,.55);
   opacity:0;transition:transform .25s ease,opacity .25s ease;z-index:50;pointer-events:none}
 .banner.show{transform:translateX(-50%) translateY(0);opacity:1}
 .banner.err{background:var(--err);color:#fff}
 /* A screen condition: shown, never edited. See the check step kind in edit.go. */
 .check{display:flex;align-items:center;gap:10px;padding:7px 12px;margin:3px 0;border-radius:7px;
   border:1px dashed #3a4a5a;background:rgba(255,255,255,.02);color:#9fb3c8;font-style:italic}
 .check .num{font-style:normal;opacity:.55;margin-right:8px}
 .check .why{margin-left:auto;font-size:11px;font-style:normal;opacity:.6;letter-spacing:.5px}
 /* The compiler's refusal, where the person is working — it stays until the next save. */
 .saveerr{margin:10px 0;padding:11px 14px;border-radius:8px;border:1px solid var(--err);
   background:rgba(255,60,60,.09);color:#ffbdbd;white-space:pre-wrap;font-size:13px;line-height:1.5}
 .saveerr b{color:#fff;display:block;margin-bottom:4px;letter-spacing:.5px}
</style></head><body>
<div id="banner" class="banner"></div>
<header>
  <button class="burger" onclick="toggleNav()">☰</button>
  <img src="/icon.png" alt="" class="logo">
  <span class="brand">MARCO<span class="sub" id="rt">…</span></span>
  <span id="saved" class="saved"></span>
</header>
<nav id="drawer">
  <a data-view="here" onclick="nav('here')">◉ Here</a>
  <a data-view="learn" onclick="nav('learn')">✦ Learn</a>
  <a data-view="routes" onclick="nav('routes')">▤ Plays</a>
  <a data-view="activity" onclick="nav('activity')">≡ Activity</a>
  <a data-view="bindings" onclick="nav('bindings')">⌨ Bindings</a>
  <a data-view="config" onclick="nav('config')">⚙ Settings</a>
  <a data-view="help" onclick="nav('help')">? Help</a>
  <div class="navgroup">Advanced</div>
  <a data-view="advanced" onclick="nav('advanced')">⚗ Advanced</a>
  <a data-view="edit" onclick="nav('edit')">◆ Marco source</a>
</nav>
<div id="scrim" onclick="closeNav()"></div>
<main>
 <section id="view-edit" hidden>
  <h2>Marco source — edit play</h2>
  <p class="hint">Every step is editable in place — wait (ms), coordinates (x, y), the key a
    <b>Press/Hold/Release</b> sends, <b>Type</b> text, a <b>Focus/Launch</b> app, a <b>Secret</b> name,
    or a <b>Repeat</b> count. <b>+</b> inserts after; <b>✕</b> deletes; <b>drag</b> turns a click
    into a click-drag. The last <b>settle · keep</b> wait lets the final action land before the run ends.</p>
  <div id="steps"></div>
  <div id="saveerr" class="saveerr" hidden></div>
  <div class="bar"><button class="go" onclick="save()">Save</button>
    <button type="button" class="mini" onclick="document.getElementById('steps').appendChild(addRow(-1))">+ add step</button>
    <button type="button" class="mini" id="editrun" hidden onclick="doOpenPlay()"
      title="runs for real (types/clicks)">▶ run</button>
    <span id="saved2"></span></div>
  <details><summary>Full play source</summary><pre id="src"></pre></details>
 </section>
 <section id="view-here" hidden>
  <h2>Here</h2>
  <p class="hint">Two things can be on here. <b>Watching</b> means Marco can see where you are.
    <b>Watching &amp; learning</b> adds permission to keep what it works out — learning always
    includes watching, so turning it on starts watching too, and stopping watching ends it.
    <b>Current</b> below is what Marco sees right now; <b>Just learned</b> is what it has
    actually written down.</p>
  <div id="hwake"></div>
  <div id="wbar" style="margin:0 0 12px;padding:12px;border:1px solid var(--line);border-radius:6px;background:var(--panel)">
   <div class="bar" style="margin:0">
    <div id="wstate" style="margin:0;flex:1;font-size:15px;font-weight:600">Not watching</div>
    <button id="wwatch" onclick="watchVerb('watch')"
      style="padding:6px 12px;background:transparent;color:var(--run);border:1px solid var(--line);border-radius:6px;cursor:pointer;font:inherit">Watch</button>
    <button id="wlearn" onclick="watchVerb('learn')"
      style="padding:6px 12px;background:transparent;color:var(--run);border:1px solid var(--line);border-radius:6px;cursor:pointer;font:inherit">Watch &amp; Learn</button>
    <button id="wunlearn" onclick="watchVerb('unlearn')" hidden
      style="padding:6px 12px;background:transparent;color:var(--dim);border:1px solid var(--line);border-radius:6px;cursor:pointer;font:inherit">Stop learning</button>
    <button id="wstop" onclick="watchVerb('stop')" hidden
      style="padding:6px 12px;background:transparent;color:var(--dim);border:1px solid var(--line);border-radius:6px;cursor:pointer;font:inherit">Stop watching</button>
   </div>
   <div id="wmeans" style="font-size:13px;margin-top:6px"></div>
   <div id="wsays" style="color:var(--dim);font-size:12px;margin-top:6px"></div>
  </div>
  <div id="gbox" hidden style="margin:0 0 12px;padding:12px;border:1px solid var(--line);border-radius:6px;background:var(--panel)">
   <div class="bar" style="margin:0 0 8px">
    <div class="grouphead" style="margin:0;flex:1">THE MAP</div>
    <div id="gcount" style="color:var(--dim);font-size:12px"></div>
   </div>
   <div id="ggraph" style="font-size:13px;line-height:1.9"></div>
   <div id="greach" style="font-size:13px;line-height:1.7;margin-top:12px;padding-top:10px;border-top:1px solid var(--line)"></div>
  </div>
  <div id="lherebar" style="margin:0 0 12px;padding:12px;border:1px solid var(--line);border-radius:6px;background:var(--panel)">
    <div class="bar" style="margin:0 0 8px">
      <div class="grouphead" style="margin:0;flex:1">NOW</div>
      <div style="color:var(--dim);font-size:12px">what Marco sees</div>
    </div>
    <div id="lherename" style="color:var(--accent);font-size:15px"></div>
    <div id="lherestatus" style="font-size:12px;margin-top:2px"></div>
    <div id="lheresays" style="color:var(--dim);font-size:12px;margin-top:4px"></div>
    <div id="lherewhy" style="color:var(--dim);font-size:12px;margin-top:2px"></div>
    <div class="bar" id="lherebtns" style="margin:10px 0 0"></div>
  </div>
  <div id="xbox" hidden style="margin:0 0 12px;padding:12px;border:1px solid var(--accent);border-radius:6px;background:var(--panel)">
   <div class="bar" style="margin:0 0 6px">
    <div class="grouphead" id="xhead" style="margin:0;flex:1;color:var(--accent)">READY TO TEST</div>
    <div style="color:var(--dim);font-size:12px">one thing at a time</div>
   </div>
   <div id="xedge" style="font-size:16px;color:var(--accent)"></div>
   <div id="xwhy" style="color:var(--dim);font-size:12px;margin-top:6px;line-height:1.6"></div>
   <div id="xsteps" style="font-size:13px;line-height:1.8;margin-top:10px"></div>
   <div class="bar" id="xbtns" style="margin:10px 0 0">
    <button class="go" id="xgo" onclick="testEdge()">Test what I learned</button>
    <button id="xstop" hidden onclick="stopAll()"
      style="padding:9px 14px;background:transparent;color:var(--dim);border:1px solid var(--line);border-radius:6px;cursor:pointer;font:inherit">Stop</button>
   </div>
  </div>
  <div id="lbox" hidden style="margin:0 0 12px;padding:12px;border:1px solid var(--line);border-radius:6px;background:var(--panel)">
   <div class="bar" style="margin:0 0 4px"><div class="grouphead" style="margin:0;flex:1">LAST RESULT</div><div style="color:var(--dim);font-size:12px;font-weight:400">what Marco wrote down</div></div>
   <div id="lnew" style="color:var(--accent);font-size:16px;margin:2px 0 0"></div>
   <div class="bar" id="ltrybar" style="margin:10px 0 0">
    <input class="field" id="ltryname" placeholder="ask Marco to do it" style="flex:1;min-width:240px">
    <button class="go" onclick="tryIt()">Go there</button>
   </div>
   <div id="lgap" hidden style="color:var(--dim);font-size:12px;margin-top:10px;line-height:1.6">Marco knows this place now. Knowing where somewhere is and being able to be <b>asked</b> for it are different things — open <b>Everything Marco has noticed</b> below and give what you just did a name, and it becomes something you can ask for.</div>
   <div id="ltryout" style="font-size:13px;margin-top:8px"></div>
   <div id="lmissed" style="color:var(--dim);font-size:12px;margin-top:6px"></div>
  </div>
  <details id="mbox" hidden style="margin:0 0 12px;padding:10px 12px;border:1px solid var(--line);border-radius:6px;background:var(--panel)">
   <summary style="cursor:pointer;color:var(--dim);font-size:12px;letter-spacing:.08em">EVERYTHING MARCO HAS NOTICED</summary>
   <div id="mlist" style="font-size:13px;line-height:1.6;margin-top:10px"></div>
   <div id="mgoal" hidden style="margin-top:12px;padding-top:10px;border-top:1px solid var(--line)">
    <div style="color:var(--dim);font-size:12px;margin-bottom:6px">Marco learns places and the
      ways between them by watching. What to <b>ask</b> for is still yours to name — give what you
      just did a name and it becomes something you can ask Marco to do.</div>
    <div class="bar" style="margin:0">
     <input class="field" id="mname" placeholder="open mouse settings" style="flex:1;min-width:240px">
     <button class="go" onclick="nameRecent()">Name what I just did</button>
    </div>
   </div>
   <div id="lolder" style="font-size:13px;line-height:1.7;margin-top:12px;padding-top:10px;border-top:1px solid var(--line)"></div>
   <div id="ltrailbox" hidden style="margin-top:12px;padding-top:10px;border-top:1px solid var(--line)">
     <div class="grouphead" style="margin-top:0">RECENT PLACES</div>
     <div id="ltrail" style="font-size:13px;line-height:1.7"></div>
   </div>
  </details>
  <div id="lplaces"></div>
 </section>
 <section id="view-learn" hidden>
  <h2>Learn</h2>
  <p class="hint">Name it, press <b>Start Learning</b>, then go and do the thing normally.
    Marco waits for you to leave this page — it never treats clicks on itself as part of what
    you are showing it. Press <b>Stop Learning</b> when you are done. To just watch what Marco
    sees, without showing it anything, open <a class="plain" onclick="nav('here')">Here</a>.</p>
  <div id="lwake"></div>
  <div class="bar">
    <input class="field" id="lname" placeholder="Open Mouse Settings" style="flex:1;min-width:260px">
    <button class="go" id="lstart" onclick="learnStart()">Start Learning</button>
    <button class="go" id="lstop" onclick="learnVerb('stop')" hidden>Stop Learning</button>
    <button class="go" id="ltry" onclick="learnVerb('try')" hidden>Try It</button>
    <button id="lcancel" onclick="learnVerb('cancel')" hidden
      style="padding:9px 14px;background:transparent;color:var(--dim);border:1px solid var(--line);border-radius:6px;cursor:pointer;font:inherit">cancel</button>
  </div>
  <div id="lready" hidden style="margin:0 0 12px;padding:10px 12px;border:1px solid var(--line);border-radius:6px;background:var(--panel);font-size:13px;line-height:1.8"></div>
  <div id="laskbar" hidden style="margin:0 0 12px;padding:12px;border:1px solid var(--line);border-radius:6px;background:var(--panel)">
    <div class="grouphead" style="margin-top:0">MARCO IS ASKING</div>
    <div id="lasking"></div>
  </div>
  <div id="lstage" class="grouphead"></div>
  <p id="lsaying" style="color:var(--text);font-size:14px;margin:6px 0 14px"></p>
  <p id="lstuck" hidden style="color:var(--err);font-size:13px;margin:-8px 0 14px"></p>
  <div id="lwaitbar" hidden style="margin:0 0 14px;padding:12px;border:1px solid var(--line);border-radius:6px;background:var(--panel)">
    <div class="grouphead" style="margin-top:0">WAITING FOR</div>
    <p id="lwaitwhat" style="color:var(--accent);font-size:14px;margin:4px 0 0"></p>
    <p id="lwaithere" style="color:var(--amber);font-size:13px;margin:8px 0 0"></p>
  </div>
  <div id="lnamebar" hidden style="margin-top:10px;padding:12px;border:1px solid var(--line);border-radius:6px;background:var(--panel)">
    <div class="grouphead" style="margin-top:0">MARCO IS ASKING ABOUT</div>
    <p id="lnamingwhat" style="color:var(--accent);font-size:14px;margin:4px 0 10px"></p>
    <div class="bar" style="margin-top:0">
      <input class="field" id="lcalled" placeholder="what you call this screen" style="flex:1;min-width:220px">
      <button class="go" onclick="learnName()">Save name</button>
      <button onclick="learnVerb('skip')"
        style="padding:9px 14px;background:transparent;color:var(--dim);border:1px solid var(--line);border-radius:6px;cursor:pointer;font:inherit">skip</button>
    </div>
  </div>
  <div id="lfacts"></div>
  <div id="lresult"></div>
  <details id="ldebug" hidden><summary>Debug</summary><pre id="ldebugbody"></pre></details>
 </section>
 <section id="view-routes" hidden><h2>Plays</h2>
  <p class="hint">A <b>play</b> is one thing Marco can do, written down as a small program you can
    read, edit and run. A play that was only <b>saved</b> is a file — nothing can ask for it by
    name until you <b>register</b> it.</p>
  <div id="routes"></div></section>
 <section id="view-bindings" hidden>
  <h2>Bindings — leader hotkeys</h2>
  <p class="hint">Press the leader (<kbd>` + "`" + `</kbd>) then a key to fire a play. A binding <b>scopes to its
    play's app</b>, so <kbd>` + "`" + `</kbd><kbd>e</kbd> can run one macro in Rocket League and another
    elsewhere. Global plays bind everywhere. A binding is a way in — it is not itself a play.</p>
  <div class="bar"><input class="field" id="bkey" placeholder="key (e.g. e)" style="width:110px">
    <input class="field" id="bcmd" placeholder="play (e.g. enter freeplay)" style="flex:1;min-width:160px">
    <button class="go" onclick="bindAdd()">Bind</button></div>
  <div id="bindings" style="margin-top:14px"></div>
 </section>
 <section id="view-config" hidden>
  <h2>Settings</h2>
  <p class="hint">How Marco behaves — the <b>leader</b> key, the voice <b>activation phrase</b>,
    theme, and where the HUD sits. Saved to the same file the overlay reads; changes apply
    <b>next time the overlay launches</b> (for live tweaks, use the overlay's own panel:
    leader → <b>config</b>). Hotkeys have their own tab:
    <a class="plain" onclick="nav('bindings')">Bindings</a>.</p>
  <div id="oconfig"></div>
  <div class="bar"><button class="go" onclick="saveConfigView()">Save</button>
    <span class="hint" id="ocpath" style="margin:0"></span></div>
 </section>
 <section id="view-activity" hidden>
  <h2>Activity</h2>
  <p class="hint">What Marco has done, most recent first, and how each one ended. Six words cover
    every ending: <b>performed</b> · <b>clarify</b> · <b>refused</b> · <b>unavailable</b> ·
    <b>cancelled</b> · <b>failed</b>. Hover a row for the reason Marco gave.</p>
  <div class="bar" style="margin:0 0 12px"><button class="mini" onclick="loadActivity()">refresh</button></div>
  <div id="activity"></div>
 </section>
 <section id="view-advanced" hidden>
  <h2>Advanced</h2>
  <p class="hint">The backstage view. Everything here names a part of Marco rather than something
    you asked it to do — useful when nothing happened and you want to know why. You never need
    this page to use Marco.</p>

  <div class="grouphead">CAST — WHO COULD ACT RIGHT NOW</div>
  <p class="hint">A play does not name who performs it. When you ask for one, Marco casts it: the
    first of these that can take the part gets it. If nothing here is available, a learned play
    will do nothing at all.</p>
  <div class="bar" style="margin:0 0 12px"><button class="mini" onclick="loadAdvanced()">refresh</button></div>
  <div id="cast"></div>

  <div class="grouphead">MARCO SOURCE</div>
  <p class="hint">Every play is a small program you can read and edit step by step — waits,
    coordinates, keys, typed text. That is the remedy when a screen loads slowly and a click
    lands too early: bump the wait before it. Open it from
    <a class="plain" onclick="nav('edit')">Marco source</a>, or from <b>edit</b> beside any play
    in the <a class="plain" onclick="nav('routes')">Plays</a> tab.</p>

  <div class="grouphead">DEEPER STILL</div>
  <p class="hint">What Marco perceived, the places and goals it holds, and how it got where it is,
    are answered by the command line rather than by this page:
    <b>marco director diagnose</b>, <b>marco director perception</b>, <b>marco director world</b>,
    <b>marco director history</b>. Each takes <b>--json</b>.</p>
 </section>
 <section id="view-help" hidden class="help"><h2>Help</h2>
  <p class="hint">Marco turns things you demonstrate once into small, editable programs that drive
    real mouse + keyboard. Each one is a <b>play</b>. Learn a command in the overlay, tune it here,
    fire it by name or a hotkey.</p>

  <h3>Where a play comes from</h3>
  <ul>
   <li><b>Authored</b> — you wrote it.</li>
   <li><b>Recorded</b> — you demonstrated it once with <b>learn</b> and Marco kept the recording.</li>
   <li><b>Learned</b> — Marco watched, worked out how to do it, and wrote it down.</li>
  </ul>

  <h3>The leader key</h3>
  <p>Everything in the overlay starts with the <b>leader</b> — the <kbd>` + "`" + `</kbd> key (change it in Settings). Tap it, then:</p>
  <ul>
   <li><kbd>` + "`" + `</kbd><kbd>m</kbd> — open the command line; type a play name (or a command) and press <kbd>Enter</kbd>.</li>
   <li><kbd>` + "`" + `</kbd><kbd>&lt;key&gt;</kbd> — run the play bound to that key (see Hotkeys).</li>
   <li><kbd>` + "`" + `</kbd> while a play is running — stop / cancel it.</li>
  </ul>

  <h3>Learn a new play</h3>
  <ol>
   <li>Focus the app or game you want to automate.</li>
   <li><kbd>` + "`" + `</kbd><kbd>m</kbd>, type <b>learn &lt;name&gt;</b>, <kbd>Enter</kbd>.</li>
   <li>Do the actions for real — clicks, keys, typing. Marco records them.</li>
   <li>Press the <b>leader</b> to finish; it saves and asks the scope (context / focus / global).</li>
  </ol>
  <p>Prefer to talk it through? <b>narrate learn &lt;name&gt;</b> (aliases <b>narrate teach</b>, <b>voice teach</b>) builds the play
    from spoken or typed phrases: "click this", "type hello", "press enter", "wait for this screen", "done".</p>

  <h3>Saved, then registered</h3>
  <p>A play Marco learns is <b>saved</b> first: a real file you can read and edit, which nothing can
    ask for by name yet. Registering it moves it where Marco looks when you ask — press
    <b>Register</b> beside it in the <b>Plays</b> tab. A name that is already taken is the usual
    reason a play is still only saved.</p>

  <h3>Run a play</h3>
  <ul>
   <li>Type its name in the command line, or say it out loud (voice).</li>
   <li>Bind it to a key and press <kbd>` + "`" + `</kbd><kbd>&lt;key&gt;</kbd>.</li>
   <li>In the <b>Plays</b> tab here: <b>▶ run</b> (performs real input) or <b>edit</b> to tune it.</li>
  </ul>

  <h3>Hotkeys — the Bindings tab</h3>
  <p>Enter a key + a play and press <b>Bind</b>; fire it with <kbd>` + "`" + `</kbd><kbd>&lt;key&gt;</kbd>. A binding
    <b>scopes to the play's app</b>, so the same key can mean different macros in different games —
    <kbd>` + "`" + `</kbd><kbd>e</kbd> = "enter freeplay" in Rocket League, something else elsewhere. Global plays bind
    everywhere. From the overlay: <b>bind &lt;key&gt; &lt;play&gt;</b> / <b>unbind &lt;key&gt;</b>.</p>

  <h3>Voice + the activation phrase</h3>
  <p>Voice is two-phase: say the <b>activation phrase</b> (the wake word, default <b>"marco"</b>) to arm
    the mic, then speak the command — or say both in one breath. Change the phrase in the <b>Settings</b>
    tab (it applies when the overlay next launches). Toggle the mic with <b>voice on</b> / <b>voice off</b>,
    <b>mute</b> / <b>unmute</b>, or <b>stop listening</b> / <b>listen</b>. Typed commands still work while muted.</p>

  <h3>Settings tab</h3>
  <p>Edit the <b>leader</b> key, the voice <b>activation phrase</b>, theme, HUD corner / width / opacity,
    and more. It writes the same overlay.json the overlay reads, so changes apply next launch. (The
    overlay's own in-HUD panel — leader → <b>config</b> — applies the non-text settings live.)</p>

  <h3>Here — what Marco can see</h3>
  <p>Press <b>Watch</b> and move around. Marco says which screen it thinks you are on and whether it
    recognises it; when it sees one it does not know, give it a name and it will remember. Naming a
    screen is what makes it worth remembering, and you can rename or unname one at any time.</p>

  <h3>Activity — what Marco has done</h3>
  <p>Every run, most recent first, with how it ended. There are six endings and no others:
    <b>performed</b>, <b>clarify</b> (Marco asked you something), <b>refused</b> (Marco said no),
    <b>unavailable</b> (nothing took it), <b>cancelled</b> (you stopped it), <b>failed</b>.</p>

  <h3>Advanced — and the step editor</h3>
  <p>You never need the Advanced pages to use Marco. They answer "why did nothing happen": the
    <b>Cast</b> table says whether anything on this machine can act at all, and <b>Marco source</b>
    is the step-by-step editor for one play.</p>
  <p>Every step is editable in place; <b>+</b> inserts a step after it, <b>✕</b> deletes, <b>Save</b> writes it back.</p>
  <ul>
   <li><b>Waits</b> — the ms between steps (the pacing that matters most).</li>
   <li><b>Click / move</b> — the x, y coordinates; <b>drag</b> turns a click into a click-and-drag.</li>
   <li><b>Press / Hold / Release</b> a key · <b>Type</b> text · <b>Focus / Launch</b> an app · <b>Secret</b> — the credential name.</li>
   <li><b>Repeat</b> — the loop count; its steps sit indented beneath it. Add one via <b>+ → repeat…</b>.</li>
   <li>The last <b>settle · keep</b> wait lets the final action land before the play ends — deleting it can make the last step flaky.</li>
  </ul>

  <h3>Play scopes</h3>
  <ul>
   <li><b>context</b> — runs only while its app is in front (menus, in-game actions).</li>
   <li><b>focus</b> — runs from anywhere and brings its app forward first.</li>
   <li><b>global</b> — app-less, runs anywhere and switches nothing (copy, paste, alt-tab…).</li>
  </ul>
  <p>Change a play's scope with the dropdown in the <b>Plays</b> tab — it moves the file to the new scope.</p>

  <h3>The OS harness — what a step can be</h3>
  <ul>
   <li><b>Click / Move / Drag</b> — the mouse at x, y (drag = press, glide, release).</li>
   <li><b>Press / Hold / Release</b> a key — a tap, a key-down held, a key-up.</li>
   <li><b>Type</b> text · <b>Secret</b> — types a stored credential (never written into the play).</li>
   <li><b>Focus</b> / <b>Launch</b> an app · <b>Wait</b> (ms) · <b>Repeat</b> N times.</li>
  </ul>

  <h3>Good to know (alpha)</h3>
  <ul>
   <li>A <b>recorded</b> play is pure <b>coordinates + recorded timings</b>. Record and replay it at the same resolution / DPI.</li>
   <li>If a screen loads slowly, bump the wait before that click here in the editor.</li>
   <li>Games that hide or lock the cursor (raw input — the pointer stays at screen-center) can't be driven by coordinates yet.</li>
  </ul>
 </section>
</main>
<script>
const LABELS={key:'Press',keydown:'Hold',keyup:'Release',type:'Type',secret:'Secret',activate:'Focus',launch:'Launch',navigate:'Navigate'};
let steps=[], adds=[];
function coord(v){ const i=document.createElement('input'); i.type='number'; i.className='coord'; i.value=v; return i; }
function numIn(v){ const i=document.createElement('input'); i.type='number'; i.className='coord'; i.value=v; return {el:i, val:()=>parseInt(i.value||'0',10)}; }
function txtIn(v){ const i=document.createElement('input'); i.className='tin'; i.value=v; return {el:i, val:()=>i.value}; }
function esc(s){ return (s||'').replace(/[&<>]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;'}[c])); }
// ops wraps a row's trailing controls in a right-aligned cluster so the ✕ lines up across rows.
function ops(...els){ const s=document.createElement('span'); s.className='ops'; els.forEach(e=> e && s.append(e)); return s; }
// ---- nav / drawer ----
function toggleNav(){ document.getElementById('drawer').classList.toggle('open'); document.getElementById('scrim').classList.toggle('on'); }
function closeNav(){ document.getElementById('drawer').classList.remove('open'); document.getElementById('scrim').classList.remove('on'); }
function nav(v){
  for(const s of document.querySelectorAll('main section')) s.hidden = (s.id!=='view-'+v);
  for(const a of document.querySelectorAll('nav a')) a.classList.toggle('active', a.dataset.view===v);
  closeNav();
  if(v==='edit') loadEdit(); else if(v==='routes') loadRoutes(); else if(v==='bindings') loadBindings();
  else if(v==='config') loadConfigView(); else if(v==='activity') loadActivity(); else if(v==='advanced') loadAdvanced();
  // HERE AND LEARN READ THE SAME ACCOUNT. Here is what Marco can see; Learn is what it is
  // acquiring. They are two questions about one running session, so one poll answers both — and
  // polling is a READ, which is why it may follow a tab around without starting anything.
  learnPolling(v==='learn' || v==='here');
  herePolling(v==='here');
}
// ---- Learn ----
//
// A WINDOW onto the Director's Learn lifecycle. Every button posts one verb and renders
// whatever comes back; nothing here decides when learning has finished, whether a candidate is
// ready, or whether a rehearsal may run. The server answers all of that.
//
// The panel never parses the sentence Marco says. It switches on the STAGE and on the three
// can_* flags, which are structured state — a UI that keyed off prose would break the first time
// somebody improved the wording.
// show toggles one control. The Learn panel's buttons are hidden rather than disabled: a
// control that cannot apply right now is noise, and a greyed-out Try It invites a person to
// wonder what they did wrong.
// renderPlaces lists the screens Marco knows, so a name can be corrected at any time.
//
// Authorship is not a mode. Somebody who realises a screen is misnamed fixes it here rather than
// waiting for Marco to raise the subject again — the absence of exactly this is what once made a
// mistaken name unrepairable without editing the store by hand.
// STATUSTONE colours the one word. Green only for known: a surface that made "new" and
// "known" look alike would hide the finding this panel exists to produce.
const STATUSTONE={known:'var(--run)', new:'var(--amber)', settling:'var(--dim)',
  ambiguous:'var(--amber)', contested:'var(--err)', degraded:'var(--err)',
  nowhere:'var(--dim)'};
function renderHere(v){
  const h=v.here;
  const watching=!!(h && h.status && h.status!=='nowhere');
  // THERE IS ONE WATCH CONTROL ON THIS PAGE, and it is the strip above. This panel used to
  // carry a second Watch button of its own — Light Mode — which meant somebody reading the
  // page saw two switches both called Watch, meaning different things, either of which made
  // recognition work. That is the confusion 38A.1 was reported for.
  //
  // The strip's Watch keeps an observation session running, which is what this panel needs,
  // so nothing here is lost by removing the duplicate. See TestTheHereViewHasOneWatchControl.

  const nameEl=document.getElementById('lherename');
  const statusEl=document.getElementById('lherestatus');
  const saysEl=document.getElementById('lheresays');
  const whyEl=document.getElementById('lherewhy');
  const btns=document.getElementById('lherebtns');
  if(!nameEl) return;

  // WHAT THE SCREEN SAYS IT IS, which is perception and not memory.
  //
  // Rendered as its own labelled line rather than in place of the name, because the name is
  // the canonical one and there is exactly one function that produces it. This is the other
  // half of the product distinction: watching, Marco may SEE Mouse; watching and learning, it
  // may REMEMBER Mouse. Shown only when it adds something the name does not already say.
  const says=(h && h.perceived) ? h.perceived : '';
  const shown=h ? (h.words || h.called || h.describes || '') : '';
  saysEl.textContent = (says && says!==shown) ? 'this screen says it is "'+says+'"' : '';

  if(!h){
    nameEl.textContent='—';
    statusEl.textContent='';
    whyEl.textContent='Marco is not watching anything. Press Watch, then move around.';
    btns.innerHTML='';
    show('ltrailbox', false);
    return;
  }
  // The NAME if somebody gave one, otherwise what it is made of. Never an identifier:
  // "which screen is this" is not answered by subj_543793ccc326.
  nameEl.textContent = h.words || h.called || h.describes || 'a screen';
  statusEl.innerHTML = '<span style="color:'+(STATUSTONE[h.status]||'var(--dim)')+
    ';font-weight:600;letter-spacing:.08em">'+esc((h.status||'').toUpperCase())+'</span>'+
    (h.application ? '<span style="color:var(--dim)"> in '+esc(h.application)+'</span>' : '');
  // WHY IT IS NOT THE PLACE YOU MEAN.
  //
  // The actual question when recognition fails: not "is this new" but "why is this not the
  // one I named a minute ago". The fields come from the matcher itself — CompareStructure is
  // ExplainStructure with the explanation discarded — so what is shown here and what the
  // Director decided are one walk of one set of rules.
  let whyText = h.why || '';
  const near = h.closest;
  if(near && (near.why||[]).length){
    const fields = near.why.map(d =>
      d.current || d.remembered
        ? d.field+' '+(d.current||'—')+' vs '+(d.remembered||'—')
        : d.field).join(' · ');
    whyText += (whyText ? '  ' : '') + 'Nearest: ' +
      (near.words || near.called || near.describes || 'a place Marco holds') +
      ' (' + near.verdict + ') — ' + fields;
  }
  whyEl.textContent = whyText;

  // NAMING, bound to the durable place HERE actually represents. Offered only when Marco
  // recognised one: renaming a screen it has not recognised would rename nothing, or
  // worse, whichever place it last had a handle for.
  let b='';
  if(h.handle){
    b += '<input class="field" id="lherecall" placeholder="call this screen…" '+
         'value="'+esc(h.called||'')+'" style="flex:1;min-width:200px">'+
         '<button class="go" onclick="nameHere()">'+(h.called?'Rename':'Name this screen')+'</button>';
    if(h.called){
      b += '<button onclick="unnameHere()" style="padding:9px 14px;background:transparent;'+
           'color:var(--dim);border:1px solid var(--line);border-radius:6px;cursor:pointer;font:inherit">Remove name</button>';
    }
  } else if(h.status==='new'){
    // A screen Marco can SEE clearly and does not know. Naming it is what makes it worth
    // remembering, so the field and the button say that — and the name is the licence:
    // pressing this with nothing typed establishes nothing. See Runtime.rememberHere.
    b = '<input class="field" id="lherecall" placeholder="call this screen…" '+
        'style="flex:1;min-width:200px">'+
        '<button class="go" onclick="rememberHere()">Remember this screen</button>';
  } else if(watching){
    b = '<span style="color:var(--dim);font-size:12px">Marco has not settled on what this '+
        'screen is yet.</span>';
  }
  // ONLY REBUILD WHEN THE BUTTONS ACTUALLY CHANGE.
  //
  // The panel re-reads every 700ms and this block used to be rewritten every time, which
  // destroys the input somebody is typing a name into: the field is removed mid-keystroke
  // and recreated empty, and focus lands back on the document. It reads as the box
  // "exiting out" about once a second, and it is impossible to type into.
  //
  // Exactly the same mistake as the screens list, made again in this block within an hour
  // of fixing it there — so the guard is the same one, over the values that decide the
  // markup. Everything else here is textContent, which does not disturb focus.
  const bsig = JSON.stringify([h.handle||'', h.called||'', h.status||'']);
  if(bsig !== LHEREBTNSIG){
    LHEREBTNSIG = bsig;
    // And when it does legitimately change, whatever was half-typed survives it.
    const a=document.activeElement;
    const keep=(a && a.id==='lherecall')
      ? {value:a.value, start:a.selectionStart, end:a.selectionEnd} : null;
    btns.innerHTML=b;
    if(keep){
      const el=document.getElementById('lherecall');
      if(el){
        el.value=keep.value;
        el.focus();
        try{ el.setSelectionRange(keep.start, keep.end); }catch(e){}
      }
    }
  }

  // THE WALK, in order, so "it thought the way back was a new screen" is visible at once.
  const trail=v.trail||[];
  show('ltrailbox', trail.length>0);
  const tr=document.getElementById('ltrail');
  if(tr && trail.length){
    tr.innerHTML = trail.map((s,i)=>
      (i?'<div style="color:var(--dim)">&nbsp;&nbsp;↓</div>':'')+
      '<div>'+esc(s.words||s.called||s.describes||'a screen')+
      ' <span style="color:'+(STATUSTONE[s.status]||'var(--dim)')+
      ';font-size:11px;letter-spacing:.08em">'+esc((s.status||'').toUpperCase())+'</span></div>'
    ).join('');
  }
}
// nameHere binds the naming actions to the durable place HERE represents.
//
// Read from the field at press time and sent with THAT handle, rather than "the current
// place": between rendering and pressing, the current place may have moved, and naming
// whichever screen Marco means rather than the one somebody is looking at is the exact
// failure ADR-069 exists to prevent.
async function nameHere(){
  const v=LHERE; if(!v || !v.handle) return;
  const el=document.getElementById('lherecall');
  learnPost('rename', {place: v.handle, called: (el ? el.value.trim() : '')});
}
async function unnameHere(){
  const v=LHERE; if(!v || !v.handle) return;
  learnPost('rename', {place: v.handle, called: ''});
}
// rememberHere makes a screen Marco does not know durable, under the name just typed.
//
// The name is the licence. A person looking at a screen and saying what it is called is the
// human semantic event that permits persisting it — the same one a learn session relies on —
// and pressing this with nothing typed establishes nothing.
async function rememberHere(){
  const el=document.getElementById('lherecall');
  const called=el ? el.value.trim() : '';
  if(!called){ banner('Say what you call this screen first', true); return; }
  learnPost('remember', {name: called});
}
let LHERE=null;
// LHEREBTNSIG is the last set of HERE actions actually drawn.
//
// Naming is the one place in this panel where somebody types something they cannot recover
// by pressing the button again, so it is the one place churn is least affordable.
let LHEREBTNSIG=null;
let LASKSIG=null;

// LPLACESIG is the last list actually drawn, so an unchanged list is left alone.
//
// The panel re-reads every 700ms and used to rebuild this list every time. Rebuilding replaces
// the very input somebody is typing a name into: the field is destroyed mid-keystroke, recreated
// with its old value, and focus lands back on the document. Live, that read as "I can't type into
// the call it… boxes, it exits out" — because it does, about once a second.
//
// Naming is the one place in this panel where a person types something they cannot get back by
// pressing the button again, so it is the one place churn is least affordable.
let LPLACESIG='';
function renderPlaces(places){
  const host=document.getElementById('lplaces');
  if(!host) return;
  // NOTHING CHANGED, so nothing is touched. The common case while somebody types.
  const sig=JSON.stringify(places);
  if(sig===LPLACESIG) return;
  LPLACESIG=sig;
  // And when it HAS changed, whatever was being typed survives the rebuild — text, focus
  // and caret. A list that legitimately updates must not cost somebody their half-typed name.
  const a=document.activeElement;
  const keep=(a && a.id && a.id.indexOf('pn_')===0)
    ? {id:a.id, value:a.value, start:a.selectionStart, end:a.selectionEnd} : null;
  const restore=()=>{
    if(!keep) return;
    const el=document.getElementById(keep.id);
    if(!el) return;
    el.value=keep.value;
    el.focus();
    try{ el.setSelectionRange(keep.start, keep.end); }catch(e){}
  };
  if(!places.length){ host.innerHTML=''; restore(); return; }
  let h='<div class="grouphead">SCREENS MARCO KNOWS</div>';
  for(const p of places){
    // THE CANONICAL NAME, and it says whose word it is. An inferred name shown as
    // "not named" is Marco knowing the answer and reporting that it does not — which is
    // what the panel did while the store already held an inferred name for that screen.
    const inferred = !p.called && p.words && p.words !== p.describes;
    const label=p.called ? esc(p.called)
      : inferred ? esc(p.words)+' <span class="tag" style="opacity:.7">marco</span>'
      : '<span style="color:var(--dim)">not named</span>';
    h+='<div class="rowcard">'+
       '<span class="nm">'+label+(p.here?' <span class="tag cur">here</span>':'')+'</span>'+
       '<span style="color:var(--dim);font-size:12px">'+esc(p.describes)+'</span>'+
       '<span class="spacer"></span>'+
       '<input class="field" style="width:170px" placeholder="call it…" '+
         'value="'+esc(p.called||'')+'" id="pn_'+esc(p.handle)+'">'+
       '<button class="go" onclick="renamePlace(\''+esc(p.handle)+'\')">save</button>'+
       (p.called?'<button onclick="unnamePlace(\''+esc(p.handle)+'\')" '+
         'style="padding:7px 10px;background:transparent;color:var(--dim);border:1px solid var(--line);border-radius:6px;cursor:pointer;font:inherit">remove</button>':'')+
       '</div>';
  }
  host.innerHTML=h;
  restore();
}
async function renamePlace(handle){
  const el=document.getElementById('pn_'+handle);
  learnPost2('rename', {place: handle, called: el? el.value.trim() : ''});
}
// Removing a name is a first-class action, not an empty save. The place survives; only the
// word goes, and the word becomes available to whichever screen the person actually meant.
async function unnamePlace(handle){ learnPost2('rename', {place: handle, called: ''}); }
async function learnPost2(v, body){
  try{
    const r=await fetch('/api/learn/'+v, {method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify(body)});
    learnRender(await r.json());
  }catch(e){ banner('Marco could not be reached', true); }
}
function show(id, on){ const e=document.getElementById(id); if(e) e.hidden=!on; }
let LTIMER=null;
// herePolling drives the dogfood strip. A SEPARATE timer from the Learn panel's on purpose: these
// are reads of ambient watching and of committed memory, and neither has anything to do with a
// demonstration's lifecycle. Held together they would have to poll at the same rate, and the feed
// wants a slower one than a session that is asking questions.
var HTIMER=null;
function herePolling(on){
  if(HTIMER){ clearInterval(HTIMER); HTIMER=null; }
  if(!on) return;
  watchRead(); learnedRead(); madeRead(); experimentRead(); mapRead();
  HTIMER=setInterval(()=>{ watchRead(); learnedRead(); madeRead(); experimentRead(); mapRead(); }, 1500);
}
function learnPolling(on){
  if(LTIMER){ clearInterval(LTIMER); LTIMER=null; }
  if(!on) return;
  learnRead();
  // A human-useful cadence, not an animation. The read is a copy of state the service already
  // holds, so this changes nothing about what Marco does — but there is no reason to ask faster
  // than a person can read.
  LTIMER=setInterval(learnRead, 700);
}
async function learnRead(){
  try{ learnRender(await (await fetch('/api/learn')).json()); }catch(e){}
}

// ---- the dogfood loop: watching, what committed, and a way to use it -------------------
//
// WCUR is where the feed got to. It is a sequence number from the Director, never a count of
// what this page has drawn: asking from zero every time would redraw the whole ring on every
// poll and could never tell "nothing happened" from "I have seen it already".
var WCUR=0, WSEEN=[];
// WATCHING and LEARNING are the last state the strip drew, so the panels below it can ask what
// Marco is doing without each keeping its own answer. One read decides; everything renders from it.
var WATCHING=false, LEARNING=false;
// WMAX is what this page keeps on screen. The Director's own ring is the real bound (256); this
// is the smaller one a person can actually read, and it is a display limit rather than a claim
// that nothing else happened.
const WMAX=12;
// WSTATES is the whole user-facing vocabulary of this strip: three states, one primary status
// each, and a sentence saying what Marco is doing rather than which permission is set.
//
// There is deliberately no fourth entry for "learning but not watching". That combination is not
// a product state — learning is a permission to remember what Marco SEES, so with nothing being
// seen it governs nothing — and the Director makes stopping watching stop learning with it. If it
// is ever reported anyway, the strip says the state is inconsistent rather than dressing it up as
// a valid one. See TestTheStripHasNoLearningWithoutWatchingState.
const WSTATES={
  off:   {dot:'○', tone:'var(--dim)',   title:'Not watching',
          means:'Marco is not looking at your screen.'},
  watch: {dot:'●', tone:'var(--run)',   title:'Watching',
          means:'Marco can see what you are doing, and is not saving anything new.'},
  learn: {dot:'●', tone:'var(--accent)',title:'Watching &amp; learning',
          means:'Marco can see what you are doing, and may write down the places it works out.'},
  broken:{dot:'⚠', tone:'var(--err)',   title:'Inconsistent',
          means:'Learning is on but Marco is not watching. That is not a state it should be able '+
                'to reach — press Stop watching, then start again.'},
  gone:  {dot:'○', tone:'var(--dim)',   title:'Not running',
          means:'The part of Marco that watches is not started. Press Watch to start it.'}
};
// watchState names the one product state from what the Director reported. One function, so the
// status line and the buttons can never disagree about what Marco is doing.
function watchState(s){
  if(!s || !s.available) return 'gone';
  if(s.learning && !s.watching) return 'broken';
  if(s.learning) return 'learn';
  if(s.watching) return 'watch';
  return 'off';
}
async function watchRead(){
  try{
    const s=await (await fetch('/api/watching')).json();
    const st=watchState(s), w=WSTATES[st];
    WATCHING = (st==='watch' || st==='learn');
    LEARNING = (st==='learn');
    document.getElementById('wstate').innerHTML =
      '<span style="color:'+w.tone+'">'+w.dot+'</span> '+w.title;
    document.getElementById('wmeans').innerHTML = w.means;
    // THE BUTTONS ARE THE LEGAL TRANSITIONS OUT OF THIS STATE, and nothing else is offered.
    // Watch and Watch & Learn are two ways IN; from watching there is one way further in and
    // one way out; from watching and learning there is a way back to watching and a way out.
    show('wwatch',   st==='off' || st==='gone');
    show('wlearn',   st==='off' || st==='gone' || st==='watch');
    document.getElementById('wlearn').innerHTML =
      st==='watch' ? 'Remember what I do' : 'Watch &amp; Learn';
    show('wunlearn', st==='learn');
    show('wstop',    st==='watch' || st==='learn' || st==='broken');
    // COUNTS, and only counts. This strip may say how much Marco has noticed; what it noticed
    // is a question for the store, and the answer to it arrives already worded in the feed.
    const bits=[];
    if(s.application) bits.push('watching '+s.application);
    if(s.candidates) bits.push(s.candidates+' thing'+(s.candidates==1?'':'s')+' seen more than once');
    if(s.learned) bits.push(s.learned+' learned by watching');
    document.getElementById('wsays').textContent=bits.join(' · ');
  }catch(e){}
}
// ---- what Marco made of what you just did -------------------------------------------
//
// The dogfood failure this answers: a person turned Watch & Learn on, walked
// Home -> Bluetooth & devices -> Mouse twice with real clicks, and the page said nothing at all.
// Not learned, not couldn't - nothing. The Director had the answer the whole time and said it
// only to a terminal. A mode indicator with no observable result is a light, not a product.
//
// Every line here is the Director's own verdict and the Director's own sentence. This page does
// not decide whether something was learned and cannot: it prints what the policy said.
const MADETONE={learned:'var(--run)', wait:'var(--dim)', never:'var(--err)'};
const MADEMARK={learned:'✓', wait:'·', never:'✗'};
function madeRow(v){
  // LEARNED IS ITS OWN WORD. A promoted candidate reports verdict 'never' with reason
  // 'already_known' - which is the policy being asked about a thing it has already admitted,
  // and rendering that as a refusal would tell somebody Marco threw away what it just learned.
  const state = v.learned ? 'learned' : (v.verdict==='promote' ? 'learned' : v.verdict);
  const tone = MADETONE[state] || 'var(--dim)';
  const mark = MADEMARK[state] || '·';
  const what = (v.did||'went') + (v.control ? ' “'+esc(v.control)+'”' : '');
  const times = v.seen===1 ? 'once' : v.seen+' times';
  const said = v.learned ? 'Marco remembers this way between two screens.' : (v.said||'');
  return '<div style="margin:0 0 8px">'+
    '<span style="color:'+tone+';font-weight:600">'+mark+'</span> '+
    '<span style="color:'+tone+'">'+esc(what)+'</span>'+
    '<span style="color:var(--dim)"> — '+times+'</span>'+
    (said ? '<div style="color:var(--dim);margin-left:16px">'+esc(said)+'</div>' : '')+
    '</div>';
}
// ---- one thought at a time --------------------------------------------------------
//
// The dogfood failure this answers: Marco was observing well and discovering routes, and a person
// watching it could not tell what it was focused on, what it was about to try, why, what it
// needed, whether it was waiting for them, or whether it had given their computer back. Every
// observation and discovery competed for the same space.
//
// So this panel shows ONE experiment and its whole lifecycle:
//
//   READY TO TEST -> TESTING -> the result, and the desktop given back
//
// The hypothesis is Marco's, in three parts, because "trying Mouse" is ambiguous between a goal,
// a target, a Place, an edge and a route. The reason is Marco's, out of evidence. Nothing here
// decides anything: the proposal comes from the Director and the press is what makes an attempt
// legitimate.
var XNOW=null, XBUSY=false;
// stopAll ends whatever Marco is doing, through the same cancellation the spoken "stop", the
// leader key and director stop all reach. One mechanism, four ways in.
async function stopAll(){
  try{ await fetch('/api/stop',{method:'POST'}); }catch(e){}
}
// ---- the map ----------------------------------------------------------------------
//
// Observe is where somebody watches Marco build a map of their computer. Not a log, not a
// recorder, not a stream of Director internals — a graph of PLACES and the CONNECTIONS between
// them, because that is what Marco actually learns.
//
// It answers four questions in order: where am I, what does Marco know around here, what did it
// just find, what can it reach. Everything on this page below the strip is one of those.
//
// Every word here comes from the Director, from the one naming function. This draws and decides
// nothing: a map that named a screen itself would be a second opinion about what a place is
// called, which is exactly what the rest of the product spent two dogfood sessions removing.
async function mapRead(){
  try{
    const m=await (await fetch('/api/map')).json();
    const box=document.getElementById('gbox');
    if(!box) return;
    const places=m.places||[], edges=m.edges||[], reach=m.reachable||[];
    show('gbox', !!m.available && (places.length>0 || m.known_places>0));
    document.getElementById('gcount').textContent =
      m.known_places ? (m.known_places+' place'+(m.known_places==1?'':'s')+', '+
        (m.known_edges||0)+' connection'+(m.known_edges==1?'':'s')) : '';
    // THE NEIGHBOURHOOD, as connections rather than as a list of names. An edge is the unit
    // of what Marco learns, so it is the unit the map draws: FROM, what you did, TO. Never
    // "origin" and "arrival" — those are the machinery's words and a person read them and
    // could not tell what they meant.
    const g=document.getElementById('ggraph');
    if(!edges.length){
      g.innerHTML = places.length
        ? '<span style="color:var(--dim)">Marco knows this screen and no way in or out of it '+
          'yet. Go somewhere from here and it will.</span>'
        : '<span style="color:var(--dim)">No map yet. Turn on Watch &amp; Learn and use your '+
          'computer normally.</span>';
    } else {
      g.innerHTML = edges.map(e => {
        const from = e.from===m.here, to = e.to===m.here;
        const mark = w => '<span style="color:'+(from||to?'var(--accent)':'var(--fg)')+'">'+esc(w)+'</span>';
        const you = w => '<b style="color:var(--accent)">'+esc(w)+'</b>';
        return '<div>'+(from?you(e.from_words):mark(e.from_words))+
          ' <span style="color:var(--dim)">&rarr;</span> '+
          (to?you(e.to_words):mark(e.to_words))+
          (e.observations>1?'<span style="color:var(--dim);font-size:11px"> seen '+
            e.observations+' times</span>':'')+'</div>';
      }).join('');
    }
    // WHAT MARCO THINKS IT CAN REACH, from the canonical planner and never from the picture.
    // A route it has walked and checked is a different claim from one it has only watched,
    // and the two are said apart rather than drawn alike.
    const r=document.getElementById('greach');
    r.innerHTML = reach.length
      ? '<div class="grouphead" style="margin:0 0 6px">I CAN GET TO</div>'+
        reach.slice(0,6).map(x =>
          '<div>'+esc(x.words)+
          '<span style="color:var(--dim);font-size:11px"> &middot; '+x.steps+' step'+
          (x.steps==1?'':'s')+(x.verified?', tried':', not tried yet')+'</span></div>').join('')
      : '';
  }catch(e){}
}
async function experimentRead(){
  // NOT WHILE ONE IS RUNNING. The panel is showing the attempt, and a poll that overwrote it
  // with the next proposal would take the story away mid-sentence.
  if(XBUSY) return;
  try{
    const v=await (await fetch('/api/experiment')).json();
    XNOW = (v && v.ready) ? v : null;
    renderExperiment();
  }catch(e){}
}
function renderExperiment(){
  const box=document.getElementById('xbox');
  if(!box) return;
  show('xbox', !!XNOW && !XBUSY);
  if(!XNOW || XBUSY) return;
  document.getElementById('xhead').textContent='READY TO TEST';
  // THE HYPOTHESIS, IN THE ORDER IT READS: from HERE, doing THIS, you arrive THERE.
  document.getElementById('xedge').innerHTML =
    esc(XNOW.from_words||'somewhere')+
    ' <span style="color:var(--dim)">&mdash;</span> '+esc(XNOW.action||'')+
    ' <span style="color:var(--dim)">&rarr;</span> '+esc(XNOW.to_words||'somewhere');
  document.getElementById('xwhy').textContent = XNOW.why||'';
  document.getElementById('xsteps').innerHTML='';
  show('xgo', true); show('xstop', false);
}
// testEdge asks Marco to try the connection it just described.
//
// It sends the edge BY IDENTITY, exactly as it was handed to this page. A surface that asked to
// test "Mouse" would be asking Marco to work out what it meant, and the whole point of the
// proposal is that Marco already decided and said so.
//
// The Director does the rest where it already happens: it takes the mutating slot, brings the
// application forward, plans over its own graph, takes authority and the actuation lease per step,
// verifies where it landed, and puts the desktop back. This page contributes a press.
async function testEdge(){
  const x=XNOW;
  if(!x || !x.edge || !x.edge.from || !x.edge.to) return;
  XBUSY=true;
  const box=document.getElementById('xbox');
  show('xbox', true);
  document.getElementById('xhead').textContent='TESTING';
  document.getElementById('xwhy').textContent='';
  show('xgo', false); show('xstop', true);
  document.getElementById('xsteps').innerHTML =
    '<div style="color:var(--dim)">&rarr; getting to '+esc(x.from_words||'the start')+'&hellip;</div>';
  try{
    const r=await fetch('/api/test',{method:'POST',headers:{'Content-Type':'application/json'},
      body:JSON.stringify({application:x.application, from:x.edge.from, to:x.edge.to})});
    renderAttempt(x, await r.json());
  }catch(e){
    document.getElementById('xsteps').innerHTML=
      '<div style="color:var(--err)">Marco could not be reached.</div>';
  }
  XBUSY=false;
  show('xgo', true); show('xstop', false);
  learnedRead(); madeRead();
}
// renderAttempt tells the story the attempt actually had.
//
// Every line is the Director's own account: whether it had to walk somewhere first, whether the
// thing being tested ran at all, where it landed, and what became of the window the person was
// using. Nothing is inferred here — in particular "it did not work" and "I never got as far as
// trying" are different lines, because a person must not read a positioning failure as a result
// about the connection.
function renderAttempt(x, v){
  document.getElementById('xhead').textContent='TESTED';
  const line=(mark, tone, text)=>
    '<div><span style="color:'+tone+'">'+mark+'</span> '+esc(text)+'</div>';
  let out='';
  if(v.positioned) out += line('✓','var(--run)','Got to '+(x.from_words||'the start'));
  if(!v.tried){
    out += line('✗','var(--err)', v.say || 'I did not get as far as trying it.');
  } else if(v.arrived){
    out += line('✓','var(--run)','Tried '+(x.action||'it'));
    out += line('✓','var(--run)','Arrived at '+(x.to_words||'the other screen'));
  } else {
    out += line('✓','var(--dim)','Tried '+(x.action||'it'));
    out += line('✗','var(--err)', v.say || 'That did not go where I expected.');
  }
  // AND THE DESKTOP. Never silent: "I put your computer back" and "I couldn't" are different
  // facts about somebody's afternoon, and the second is the one they need.
  const rest=v.restored;
  if(rest && rest.attempted){
    out += rest.restored
      ? line('✓','var(--run)','Put you back in '+(rest.application||'where you were'))
      : line('⚠','var(--err)', rest.say || 'I could not put you back where you were.');
  }
  document.getElementById('xsteps').innerHTML=out;
}
async function madeRead(){
  try{
    const v=await (await fetch('/api/made')).json();
    const seen=(v && v.seen) ? v.seen : [];
    const box=document.getElementById('mlist');
    if(!box) return;
    show('mbox', WATCHING);
    if(!seen.length){
      // NOT SILENCE. "Nothing yet" is an answer and "I learned nothing from that" is a
      // different one; a blank panel is neither, and a person reads it as broken.
      box.innerHTML='<span style="color:var(--dim)">Nothing yet — go and use another '+
        'window, and Marco will say what it made of each move.</span>';
      show('mgoal', false);
      return;
    }
    seen.sort((a,b)=> (b.seen||0)-(a.seen||0));
    box.innerHTML = seen.slice(0, 8).map(madeRow).join('');
    // AND THE NEXT ACTION, when there is anything worth naming.
    //
    // Ambient learning admits TOPOLOGY - places and the ways between them. It deliberately
    // creates no goal: admitWatched says so in as many words, because a name for an outcome
    // is a thing a person means, not a thing repetition implies. So the gap is shown and the
    // canonical way to close it is offered, rather than leaving somebody to wonder why there
    // is nothing to try.
    show('mgoal', LEARNING && seen.some(v => v.learned || v.verdict==='promote'));
  }catch(e){}
}
// nameRecent is "learn what I just did", through the request the command line makes with
// --recent. It is the one door that turns a walk into something you can ask for by name: it
// promotes the walk, keeps the demonstration and records the GOAL. This page builds none of
// that - it says the name and lets the Director do all of it.
async function nameRecent(){
  const el=document.getElementById('mname');
  const name=el ? el.value.trim() : '';
  if(!name){ banner('Say what to call it first', true); return; }
  await learnPost('recent', {name: name});
  if(el) el.value='';
  learnedRead();
}
async function watchVerb(v){
  try{ await fetch('/api/watching',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({verb:v})}); }catch(e){}
  await watchRead(); madeRead();
}
// learnedRead draws what the store COMMITTED, and nothing else. It cannot invent an event: every
// line here was announced by semanticmemory.Store after its own write succeeded, and arrives
// already worded by the only process holding the store — so this page never names a place.
async function learnedRead(){
  try{
    // READ FROM THE START OF THE RING, EVERY TIME, AND KEEP NOTHING.
    //
    // # Why a cursor was wrong here
    //
    // The Director resolves a place's name at READ time, deliberately: a Place is established
    // on one pass and NAMED on a later one, so an event that carried the words it had at the
    // instant of the write would say "about back, settings, 96 things on it" forever about a
    // screen that is now perfectly well called Home. That is ADR-111's own reasoning.
    //
    // This page defeated it. It asked from a cursor, got only what was new, and cached the
    // rendered line in WSEEN — so the description froze at the moment of the commit and the
    // name that arrived a few seconds later was never shown. Reported live, and it is the
    // whole of "naming propagation leaves the user confused": the naming worked and the page
    // was showing a snapshot of the instant before it.
    //
    // The cursor exists for a FOLLOWER, which must not reprint what it has already printed —
    // that is marco observe --follow, which keeps its own cursor in observefeed.go and is
    // untouched. This is a page that redraws wholesale; re-reading is what makes it current.
    //
    // Deleting the re-read must fail TestTheFeedRefreshesNamesRatherThanCachingThem.
    const v=await (await fetch('/api/learned?after=0')).json();
    if(!v.available) return;
    if(typeof v.newest==='number') WCUR=v.newest;
    if(v.missed){ document.getElementById('lmissed').textContent =
      v.missed+' earlier '+(v.missed==1?'change':'changes')+' scrolled past before this page read them.'; }
    // NEWEST FIRST. The Director returns the ring in the order it happened, oldest to newest —
    // and this page reads index 0 as the headline. Concatenating batches put the OLDEST of the
    // newest batch on top, which showed on a burst and hid on a quiet desktop.
    const all=(v.events||[]).slice().reverse();
    if(!all.length) return;
    WSEEN=all.slice(0,WMAX);
    show('lbox', true);
    // THE PRIMARY LINE IS THE MOST IMPORTANT THING THAT HAPPENED, not the most recent.
    //
    // Walking a familiar route is not a discovery, and a surface that put "saw way again"
    // over the top of "learned destination" would train somebody to stop reading it. The
    // demoted changes are still in the feed below and in Activity, where repeated evidence
    // belongs; they simply do not compete with new knowledge for the one line somebody
    // actually reads. See TestTheHeadlineIsNewKnowledgeRatherThanTheLatestEvent.
    // A movement is demoted for the same reason: Marco saw somebody go somewhere and cannot
    // say how, which must not sit above a way it actually learned. See ADR-122.
    const top = WSEEN.find(e => e.change!=='strengthened' && e.change!=='saw' &&
                             e.change!=='noticed') || WSEEN[0];
    document.getElementById('lnew').textContent=learnedLine(top);
    // AND GO THERE IS OFFERED ONLY FOR SOMETHING MARCO CAN BE ASKED FOR.
    //
    // # The failure this closes
    //
    // The box was prefilled from whatever the newest change described, and a a 'learned place'
    // describes a PLACE. So after Marco learned Home, the page offered "Home" and a button
    // saying Go there — and pressing it ran marco do "Home", which is not a request Marco
    // can answer. It went through intake, matched no play and no goal, reached the Director's
    // read-it-against-the-screen path and came back:
    //
    //     FAILED: I only understand click, focus, move, repeat and text editing so far
    //
    // A place is not a phrase. Knowing where somewhere is and being able to be asked for it
    // are different things, and the second one is a name a person gives — which is the whole
    // reason "Name what I just did" exists. Offering Go there without one sends somebody to a
    // dead end and blames the vocabulary.
    //
    // So the phrase comes from a GOAL and from nothing else, and when there is no goal the
    // gap is named rather than papered over.
    //
    // Deleting the kind test must fail TestGoThereIsOnlyOfferedForSomethingYouCanAskFor.
    const askable = WSEEN.find(e => e.kind==='goal' && e.description);
    show('ltrybar', !!askable);
    show('lgap', !askable);
    if(askable){
      // The phrase box is a SUGGESTION and it is only ever filled once, while it is empty:
      // overwriting what somebody was in the middle of typing because Marco learned something
      // else is the surface taking the keyboard off them.
      const box=document.getElementById('ltryname');
      if(!box.value.trim()) box.value=askable.description;
    }
    document.getElementById('lolder').innerHTML =
      WSEEN.filter(e=>e!==top).map(e=>'<div style="color:var(--dim)">'+esc(learnedLine(e))+'</div>').join('');
  }catch(e){}
}
// learnedLine is the four words, kept apart. Calling every change "learned" would train somebody
// to stop reading the feed — walking a familiar route is not a discovery. See ADR-111.
function learnedLine(e){
  const what={place:'place', edge:'way', goal:'destination', movement:'movement',
              affordance:'affordance'}[e.kind]||e.kind;
  // 'watched you go' is deliberately not a way. A movement is a crossing Marco saw and cannot
  // explain, because attribution refused to name a control it did not see; saying 'learned way'
  // there would offer a route the graph declined to claim. See ADR-120 and ADR-122.
  const how={learned:'learned '+what, strengthened:'saw '+what+' again',
             named:'named', rebound:what+' moved', saw:'watched you go',
             noticed:'noticed'}[e.change]||e.change;
  return how+'  '+(e.description||'');
}
// tryIt asks for the thing, through the same door a typed marco do uses. This page starts no
// play itself; it posts the words and then asks what became of them.
async function tryIt(){
  const text=document.getElementById('ltryname').value.trim();
  if(!text){ banner('Say what to try', true); return; }
  const out=document.getElementById('ltryout');
  out.textContent='asking Marco to '+text+'...';
  try{
    const r=await fetch('/api/try',{method:'POST',headers:{'Content-Type':'application/json'},
      body:JSON.stringify({text})});
    if(!r.ok){ out.textContent=await r.text(); return; }
    const j=await r.json();
    // THE ENGINE'S OWN WORD, not this page's guess. A declined play, a play that could not
    // recognise the screen and a play that worked are three different answers.
    watchRun(j.run, out);
  }catch(e){ out.textContent=String(e); }
}
function watchRun(id, out){
  let n=0;
  const t=setInterval(async()=>{
    if(++n>200){ clearInterval(t); return; }
    try{
      const s=await (await fetch('/api/run?id='+encodeURIComponent(id))).json();
      if(!s.done) return;
      clearInterval(t);
      out.textContent=s.outcome+(s.detail?' — '+s.detail:'');
    }catch(e){ clearInterval(t); }
  }, 500);
}
async function learnStart(){
  const name=document.getElementById('lname').value.trim();
  if(!name){ banner('Say what you want Marco to learn', true); return; }
  learnPost('start', {name});
}
async function learnVerb(v){ learnPost(v, {}); }
// learnAnswer settles one of Marco.s own questions. Addressed: WHICH question, in WHICH
// session -- answering "the current one" would settle whichever happened to be first.
async function learnAnswer(id, session, response){
  learnPost('answer', {id: id, session: session, response: response});
}
async function learnName(){
  const el=document.getElementById('lcalled');
  const called=el.value.trim();
  if(!called){ banner('Say what you call it, or skip', true); return; }
  el.value='';
  learnPost('name', {name: called});
}
// LREFUSAL is the last thing Marco refused to do, and WHY, held until something changes.
//
// A verb's answer used to be painted and then overwritten by the 700ms poll, so a refusal was on
// screen for under a second and the panel snapped back to the same question. Live, that read as
// "it's stuck on Want me to try it?" — the press WAS refused, every time, and there was no way to
// see it. A refusal nobody can read is a refusal that did not happen.
let LREFUSAL=null;
async function learnPost(v, body){
  try{
    const r=await fetch('/api/learn/'+v, {method:'POST', headers:{'Content-Type':'application/json'},
      body: JSON.stringify(body)});
    const s=await r.json();
    // Held, not flashed. Cleared by the next render that shows a different stage — which is
    // what "something changed" means here.
    LREFUSAL = (s.stage==='refused' && s.saying) ? {saying:s.saying, at:s.stage} : null;
    learnRender(s);
  }catch(e){ banner('Marco could not be reached', true); }
}
// LSTAGE is the lifecycle as words a person reads.
//
// The 'unavailable' label used to name the background service, in capitals, at a person who has no
// word for it and no idea what it means for them. What it means is that Marco cannot see, and
// pkg/playbill already had the register for that — "I'm not watching anything right now." — so
// this matches it rather than inventing a second way to say the same fact.
//
// Deleting the plain wording must fail TestNoNormalSurfaceNamesTheBackstage.
const LSTAGE={
  idle:'READY', unavailable:'NOT WATCHING YET',
  waiting_for_demonstration:'WAITING FOR YOU', learning:'LEARNING',
  finishing:'FINISHING UNDERSTANDING', needs_another_example:'NEEDS ANOTHER EXAMPLE',
  ready_to_try:'READY TO TRY', waiting_to_try:'WAITING TO TRY', trying:'TRYING',
  naming:'NAMING A SCREEN', understood:'UNDERSTOOD', refused:'REFUSED', stopped:'STOPPED',
};
// refusalWords is a refusal code as words, the way LSTAGE is a stage as words.
//
// # Why every one of them is spelled out
//
// The panel used to print the raw enum at the person: "Refused: several_routes". Two entries were
// then given English and the other twenty-six were left to a fallback that swaps underscores for
// spaces — which is not translation, it is the identifier with its punctuation combed. A person
// reading "no subject", "no tail", "not lowerable", "not assessable", "not armed" or "action not
// attributed" has been shown a machine word and told it is a sentence. All of those are reachable:
// they are declared in internal/director/learn/learn.go and raised by the coordinator.
//
// So the map is COMPLETE against that file, and TestEveryRefusalCodeHasPlainEnglish walks the
// declarations and fails if one of them is missing here. The fallback survives for exactly one
// case — a code added upstream before this map is extended — and the test is what stops that case
// being the normal one.
//
// Each line says what Marco could not do, in the second person where there is a person in it.
// Marco.s own account of the refusal is already on screen above this (#lsaying, from
// learn/say.go); this is the label beside it, and a label may be short without being a symbol.
//
// Deleting an entry must fail TestEveryRefusalCodeHasPlainEnglish.
const LREFUSED={
  no_observation:'Marco could not see anything while that happened',
  no_subject:'you never went to an application, so there was nothing to watch',
  nothing_changed:'the screen never changed',
  destination_not_recognised:'Marco would not recognise where you ended up',
  several_routes:'several ways there',
  route_not_remembered:'the way there was not remembered',
  not_armed:'Marco was never actually watching',
  demonstration_incomplete:'Marco did not see the whole thing',
  requires_text_entry:'it needs something typed, and Marco does not watch what you type',
  action_not_attributed:'Marco saw where you ended up but not what you did',
  not_assessable:'Marco could not make anything of what it saw',
  demonstrations_disagree:'the two times you showed it were not the same',
  evidence_insufficient:'showing it again would not settle it',
  application_changed:'you moved to a different application partway through',
  name_not_usable:'that name cannot be written down',
  goal_not_remembered:'that name already means getting somewhere else',
  examples_exhausted:'Marco has asked for another example as often as it will',
  rehearsal_declined:'you said not now',
  rehearsal_refused:'Marco would not try it',
  rehearsal_not_started:'you said yes and Marco never got permission to act',
  rehearsal_failed:'the try did not finish',
  not_lowerable:'Marco could not write this down as a play',
  name_refused:'that name for the screen was not accepted',
  save_failed:'the play could not be saved',
  play_not_registered:'it was saved, but nothing can ask for it yet',
  answer_timed_out:'nobody answered in time',
  no_tail:'part of Marco is not wired up on this machine',
};
function refusalWords(code){
  return LREFUSED[code] || String(code||'').replace(/_/g,' ');
}
// WAKING is true while the start request this page sent is still in flight.
//
// The read polls every 700ms and would otherwise repaint the button underneath somebody who has
// just pressed it, so the press has to be visible until it resolves. Starting the service is
// allowed to take a few seconds.
let WAKING=false;
// renderWake draws the one control that can start the part of Marco that watches.
//
// # Why a normal surface has to have this, and why it is a BUTTON
//
// Every read on this page connects without starting anything, which is correct and deliberate —
// see cmd/marco/intake.go's pendingQuestion for the reasoning: a read that silently paid for a
// service start would charge twenty seconds to somebody who only opened a tab, on every command,
// forever. But the consequence, until now, was that a cold machine showed Learn as unavailable
// AND hid the Start Learning button, so the one capability the product is named for was
// unreachable — right up until the person happened to ask for something Marco did not know, which
// is the only path that ever started it.
//
// A press is not a read. This is the person saying "start it", once, explicitly, and it is the
// only thing on this page that starts anything.
//
// Deleting this must fail TestANormalSurfaceCanStartTheServiceItDependsOn.
function renderWake(v){
  const down = v.available===false;
  const html = !down ? '' :
    '<div style="margin:0 0 12px;padding:12px;border:1px solid var(--line);border-radius:6px;background:var(--panel)">'+
    '<div style="color:var(--text);font-size:14px;margin-bottom:10px">'+
      (WAKING ? 'Starting…' : "I'm not watching anything right now.")+'</div>'+
    '<div style="color:var(--dim);font-size:12px;margin-bottom:10px">'+
      'Marco has to be watching before it can learn anything or tell you where you are.'+'</div>'+
    '<button class="go" onclick="wakeMarco()"'+(WAKING?' disabled':'')+'>Start Marco watching</button>'+
    '</div>';
  for(const id of ['lwake','hwake']){
    const el=document.getElementById(id);
    if(el && el.innerHTML!==html) el.innerHTML=html;
  }
}
// wakeMarco starts it, and says what happened either way.
async function wakeMarco(){
  if(WAKING) return;
  WAKING=true;
  renderWake({available:false});
  try{
    const r=await fetch('/api/learn/wake', {method:'POST'});
    const st=await r.json();
    WAKING=false;
    learnRender(st);
    banner(st.available===false ? 'Marco could not start watching' : '✓ Marco is watching',
      st.available===false);
  }catch(e){
    WAKING=false;
    banner('Marco could not be reached', true);
  }
}
function learnRender(v){
  const stage=v.stage||'idle';
  document.getElementById('lstage').textContent=LSTAGE[stage]||stage;
  document.getElementById('lsaying').textContent=v.saying||'';
  renderWake(v);
  show('lstart', !v.running && v.available!==false);
  show('lstop', !!v.can_stop);
  show('ltry', !!v.can_try);
  show('lcancel', !!v.can_cancel);
  show('lnamebar', !!v.can_name);
  // WHICH screen the question is about, in words. Marco may not ask somebody to name a
  // thing it cannot point at: two Settings pages once produced identical wording, the
  // wrong one got named, and the word was then reserved against the right one.
  const naming=document.getElementById('lnamingwhat');
  if(naming) naming.textContent = v.naming
    ? (v.naming.words ? v.naming.words+' — '+v.naming.describes : v.naming.describes)
    : 'a screen Marco cannot currently point at';
  // THE READINESS BLOCK, above everything else.
  //
  // The four facts somebody needs BEFORE they start clicking, because each of them has
  // silently wasted a live demonstration: the wrong window was latched and only said so at
  // the end; nothing was captured; a stale question held the single interruption slot so no
  // rehearsal could ever be offered. All four were knowable at the time and none was shown.
  // HOW MUCH OF THE DEMONSTRATED ROUTE IS VERIFIED.
  //
  // A demonstration of Home to Bluetooth to Mouse is two reusable legs, and both have to be
  // reviewed before the goal is reachable from where it starts. The second leg used to be
  // dropped silently, so a person was told the route was learned when one step of it had
  // never been tried. Verified 1 / 2 is the difference between that and the truth.
  function routeProgressLine(v){
    const total=v.required_edges|0;
    if(!total) return '';
    const done=v.verified_edges|0;
    const ok=t=>'<span style="color:var(--run)">'+t+'</span>';
    const part=t=>'<span style="color:var(--amber)">'+t+'</span>';
    const count=done===total?ok(done+' / '+total):part(done+' / '+total);
    // THE WAY THERE, and the facts table below uses the same words for the same value.
    //
    // It read "Transition", which is what this is underneath — an edge in the Director's graph,
    // from one place to another. That is a true description of the machinery and not of anything
    // the person did: they showed Marco how to get somewhere, and this line is how far Marco has
    // got with checking it. The two lines used to disagree — "route" here, "Transition" there —
    // which made one value look like two different things.
    //
    // Deleting the plain wording must fail TestNoNormalSurfaceNamesTheBackstage.
    let out='<div>The way there: '+esc(v.route||'')+'</div>'+
            '<div>Checked: '+count+'</div>';
      // 'trying' read as Marco ATTEMPTING the step. It means Marco has asked and is waiting
      // for an answer — and an open question sitting under the word 'trying' reads as stuck.
    (v.steps||[]).forEach(function(s,i){
      const mark=s.status==='verified'?ok('done'):
                 s.status==='offered'?part('waiting for your answer'):
                 s.status==='pending'?part('to do'):part(esc(s.status));
      out+='<div style="opacity:.85">  step '+(i+1)+' of '+total+': '+
           esc(s.from)+' → '+esc(s.to)+' — '+mark+
           (s.why?' <span style="opacity:.7">('+esc(s.why)+')</span>':'')+'</div>';
    });
    return out;
  }

  const ready=document.getElementById('lready');
  show('lready', !!v.available);
  if(ready){
    const yes=t=>'<span style="color:var(--run)">'+t+'</span>';
    const no=t=>'<span style="color:var(--amber)">'+t+'</span>';
    const q=v.questions_open|0;
    ready.innerHTML=
      '<div>Watching: '+(v.watching?yes(esc(v.watching)):no('nothing yet'))+'</div>'+
      // The old line paired the Theater's word for the thing with a mechanism word for what had
      // happened to it. The fact underneath is one a person can act on: Marco has decided
      // which window this demonstration is about, and if it decided wrong, everything you do next
      // is wasted. So the line says the fact. See learnView.TargetLocked for the live failure.
      //
      // Deleting the plain wording must fail TestNoNormalSurfaceNamesTheBackstage.
      '<div>Marco has settled on a window: '+(v.target_locked?yes('YES'):no('NOT YET'))+'</div>'+
      '<div>Captured actions: '+(v.captured|0)+'</div>'+
      '<div>Questions open: '+(q===0?yes('0'):no(String(q)))+'</div>'+
      routeProgressLine(v);
  }

  // WHERE MARCO THINKS YOU ARE, live, and whether it recognises it.
  //
  // The instrument for hardening place identity. Marco was minting several durable subjects
  // for one Settings page and nothing said so until a Learn run collapsed minutes later; this
  // is so somebody can walk an application and watch the answer change as they go.
  //
  // Every value is READ from the Director's own account. There is no place detector here and
  // there must not be one: a UI that decided for itself whether two screens matched would be
  // a second matcher, and the two would disagree about the very thing under investigation.
  LHERE = v.here || null;
  renderHere(v);

  // EVERY QUESTION MARCO IS WAITING ON, with a way to answer it.
  //
  // These are Marco's own — "are these one set?" — raised during a learn pass. They hold the
  // interruption budget, they block the rehearsal question behind them, and the panel used
  // to count them at you and offer nothing. A question nobody can answer is worse than a
  // question nobody is asked.
  //
  // A naming question gets no Yes/No: it wants a word, and the naming box below asks for it.
  const asking = v.asking || [];
  show('laskbar', asking.length > 0);
  const askHost = document.getElementById('lasking');
  if(askHost && asking.length){
    let ah = '';
    for(const q of asking){
      const btn = (label, resp, colour) =>
        '<button onclick="learnAnswer(\''+esc(q.id)+'\',\''+esc(q.session_id||'')+'\',\''+resp+'\')" '+
        'style="padding:6px 12px;background:transparent;color:'+colour+
        ';border:1px solid var(--line);border-radius:6px;cursor:pointer;font:inherit">'+label+'</button>';
      ah += '<div class="rowcard" style="align-items:flex-start">'+
        '<span style="flex:1;min-width:220px;color:var(--text);font-size:14px">'+
        esc(q.question||'Marco is asking about something')+'</span>';
      ah += q.naming
        ? '<span style="color:var(--dim);font-size:12px">answer below</span>'
        : btn('Yes','confirmed','var(--run)')+btn('No','contradicted','var(--err)')+
          btn('Not now','declined','var(--dim)');
      ah += '</div>';
    }
    // Same churn guard as HERE and the screens list. No input lives here today, but a
    // block rewritten every 700ms also drops keyboard focus off whichever button somebody
    // tabbed to — and this is the third place in one panel where rebuilding on every poll
    // has cost somebody an interaction.
    const asig = JSON.stringify(asking);
    if(asig !== LASKSIG){ LASKSIG = asig; askHost.innerHTML = ah; }
  }

  // WHICH screen a patient rehearsal is waiting for, and whether Marco thinks you are on a
  // different one. Marco used to say "I'll try it when we're back there" and then wait
  // forever, because "there" was a twin of the page you were standing on — one screen
  // recorded twice. Nothing said so, and the only way to find out was to read the store.
  const wp = v.waiting, we = v.elsewhere;
  show('lwaitbar', !!wp);
  if(wp){
    document.getElementById('lwaitwhat').textContent =
      wp.called ? wp.called+' — '+wp.describes : wp.describes;
    const h = document.getElementById('lwaithere');
    // TWO conditions, and the second one is this page.
    //
    // Input has no address: Marco refuses to send clicks unless the watched window is
    // genuinely in front, or they would land wherever you actually are. Reading this panel
    // means being in a browser, so watching it is itself what stops the rehearsal — which
    // reads as "it never fires" and is the reason somebody sat through ten identical
    // window_not_in_front refusals wondering what was wrong.
    const wrongScreen = we
      ? 'You look like you are on '+(we.called ? we.called+' — '+we.describes : we.describes)
        +', which Marco thinks is a different screen. It will not fire here. '
      : '';
    h.textContent = wrongScreen +
      'It also will not fire while you are looking at this page: it only sends input when ' +
      'the window it is watching is in front. Go there, leave it in front, and wait.';
  }
  // WHAT MARCO REFUSED, and why — held until the stage moves, plus whatever the Director
  // says it cannot get past. detail now arrives on a dead end too, not only on a refusal:
  // "a yes created no authority" is the sentence that explains a Try button doing nothing.
  const lines=[];
  if(LREFUSAL) lines.push(LREFUSAL.saying);
  // EXCEPT when Marco simply is not watching. The detail for that state is a socket error naming
  // an executable, and the block above already says the fact in words and offers the button that
  // fixes it. It stays in the Debug block, which is where a dial failure belongs.
  if(stage!=='unavailable') (v.detail||[]).forEach(d=>lines.push(d));
  const stuck=document.getElementById('lstuck');
  show('lstuck', lines.length>0);
  if(stuck) stuck.textContent=lines.join(' · ');

  renderPlaces(v.places||[]);
  document.getElementById('lname').disabled=!!v.running;

  // WHAT MARCO CURRENTLY UNDERSTANDS. Only the fields it actually has: an empty row would
  // read as "Marco knows nothing about this" when the truth is "not yet".
  const f=[];
  if(v.watching) f.push(['Watching', v.watching]);
  if(v.place) f.push(['Place', v.place]);
  if(v.running||v.captured) f.push(['Actions captured', String(v.captured||0)]);
  if(v.targets) f.push(['…that named a control', String(v.targets)]);
  if(v.unnamed) f.push(['…whose name is withheld', String(v.unnamed)]);
  if(v.offered) f.push(['Controls on offer', String(v.offered)]);
  if(v.route) f.push(['The way there', v.route]);
  if(v.goal) f.push(['Would be called', v.goal]);
  document.getElementById('lfacts').innerHTML=f.map(([k,x])=>
    '<div class="crow"><label>'+esc(k)+'</label><span style="color:var(--accent)">'+esc(x)+'</span></div>').join('');

  const out=[];
  // THE SAME LIFECYCLE WORDS A PLAYS ROW CARRIES, out of the same function.
  //
  // life / life_word / life_says come from learnui.go's addLifecycle, which reads
  // plays.AfterLearn — the same plays.Life a row in the Plays list renders. This panel used to
  // compose its own sentence and finish it by naming the tab the play could be found in: a
  // claim about DISCOVERY made from a fact about STORAGE, and false for a saved play that tab
  // could not contain. Nothing here may compose a standing sentence again.
  //
  // Deleting the life_word / life_says render must fail TestTheLearnPanelRendersTheLifecycleWords.
  if(v.life){
    const name=v.play?esc(v.play):'It';
    out.push('<p style="color:'+(v.life==='ready'?'var(--run)':'var(--amber)')+'">'+
      name+' — '+esc(v.life_word||'')+'. '+esc(v.life_says||'')+'</p>');
    // AND WHERE TO GO NEXT. A play that is only saved genuinely appears in the Plays tab now,
    // under the saved ones, with the button that finishes the job — so pointing there is a
    // claim the tab can keep.
    if(v.life!=='ready') out.push('<p class="hint">Find it in the '+
      '<a class="plain" onclick="nav(\'routes\')">Plays</a> tab, under the saved ones, '+
      'with a <b>Register</b> button beside it.</p>');
  }
  if(v.refused) out.push('<p style="color:var(--err)">Refused: '+esc(refusalWords(v.refused))+'</p>');
  document.getElementById('lresult').innerHTML=out.join('');

  const dbg=document.getElementById('ldebug');
  const detail=(v.detail||[]).join('\n');
  dbg.hidden=!detail;
  document.getElementById('ldebugbody').textContent=detail;
}
const BT=String.fromCharCode(96); // backtick, kept out of the Go raw string
// ---- config view (overlay settings) ----
const OCFG=[
 {k:'leader', label:'Leader key', type:'select', opts:[BT,'/','capslock','tab','f8']},
 {k:'wake', label:'Activation phrase', type:'text', ph:'marco'},
 {k:'voice', label:'Voice listening', type:'bool'},
 {k:'theme', label:'Theme', type:'select', opts:['default','dracula','solarized-dark','monokai','nord','tokyo-night','catppuccin-mocha','gruvbox-dark','rose-pine','light']},
 {k:'idle', label:'Opacity', type:'number', step:'0.05', min:'0.2', max:'1'},
 {k:'corner', label:'HUD corner', type:'select', opts:['top-right','top-left','bottom-right','bottom-left','top-center']},
 {k:'width', label:'HUD width', type:'number', step:'20'},
 {k:'maxLines', label:'Max log lines', type:'number'},
 {k:'monitor', label:'Monitor', type:'number'},
 {k:'border', label:'Accent border', type:'bool'},
 {k:'mini', label:'Mini mode', type:'bool'},
 {k:'metrics', label:'CPU / RAM widget', type:'bool'},
 {k:'coords', label:'Cursor coords', type:'bool'},
];
async function loadConfigView(){
  const r = await (await fetch('/api/oconfig')).json();
  const cfg = r.config||{};
  document.getElementById('ocpath').textContent = r.path||'';
  const box=document.getElementById('oconfig'); box.innerHTML='';
  for(const f of OCFG){
    const row=document.createElement('div'); row.className='crow';
    const lab=document.createElement('label'); lab.textContent=f.label; row.appendChild(lab);
    let inp;
    if(f.type==='select'){ inp=document.createElement('select');
      for(const o of f.opts){ const op=document.createElement('option'); op.value=o; op.textContent=(o===BT?BT+' (backtick)':o); if(o===cfg[f.k]) op.selected=true; inp.appendChild(op); } }
    else if(f.type==='bool'){ inp=document.createElement('select');
      for(const [v,l] of [['true','on'],['false','off']]){ const op=document.createElement('option'); op.value=v; op.textContent=l; if(String(!!cfg[f.k])===v) op.selected=true; inp.appendChild(op); } }
    else { inp=document.createElement('input'); inp.type=f.type; if(f.step)inp.step=f.step; if(f.min)inp.min=f.min; if(f.max)inp.max=f.max; if(f.ph)inp.placeholder=f.ph; inp.value = cfg[f.k]!==undefined?cfg[f.k]:''; }
    inp.dataset.k=f.k; inp.dataset.t=f.type; row.appendChild(inp); box.appendChild(row);
  }
}
async function saveConfigView(){
  const out={};
  for(const inp of document.querySelectorAll('#oconfig [data-k]')){
    const k=inp.dataset.k, t=inp.dataset.t;
    if(t==='bool') out[k] = inp.value==='true';
    else if(t==='number') out[k] = (k==='idle') ? parseFloat(inp.value||'0') : parseInt(inp.value||'0',10);
    else out[k]=inp.value;
  }
  const resp=await fetch('/api/oconfig',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(out)});
  banner(resp.ok ? '✓ Settings saved — relaunch the overlay to apply' : '✗ Save failed', !resp.ok);
}
// ---- edit view ----
const FINAL_WAIT_WARN='This is the final settle wait. Deleting it can make the last step (e.g. a click) fire and the play end before the app processes it, so it may not register. Delete anyway?';
function delBtn(rec,row,warn){
  const b=document.createElement('button'); b.type='button'; b.className='del'; b.textContent='✕';
  b.title = warn ? 'delete the final settle wait (not recommended)' : 'delete this step';
  b.onclick=()=>{ if(warn && !rec.del && !confirm(warn)) return; rec.del=!rec.del; row.classList.toggle('deleted', rec.del); };
  return b;
}
function plusBtn(afterLine,row){
  const b=document.createElement('button'); b.type='button'; b.className='mini plus'; b.title='add a step after this'; b.textContent='+';
  b.onclick=()=>{ row.after(addRow(afterLine)); };
  return b;
}
// addRow builds an editable "new step" row across the OS harness; get() returns the save payload.
function addRow(after){
  const rec={after};
  const row=document.createElement('div'); row.className='action added';
  const sel=document.createElement('select'); sel.className='mini';
  for(const [v,l] of [['wait','wait'],['key','press key'],['keydown','hold key'],['keyup','release key'],
      ['type','type text'],['click','click'],['move','move cursor'],['drag','drag'],
      ['activate','focus app'],['launch','launch app'],['secret','secret'],['repeat','repeat…']]){
    const o=document.createElement('option'); o.value=v; o.textContent=l; sel.appendChild(o);
  }
  const fields=document.createElement('span'); fields.className='addfields';
  const T=(ph,g)=>{ const k=txtIn(ph); fields.append(g, k.el); return k; };
  function render(){
    fields.textContent=''; const t=sel.value;
    if(t==='wait'){ const ms=numIn(200); fields.append('wait', ms.el, 'ms'); rec.get=()=>({after,act:'wait',ms:ms.val()}); }
    else if(t==='key'||t==='keydown'||t==='keyup'){ const k=T('e',LABELS[t].toLowerCase()); rec.get=()=>({after,act:t,text:k.val()}); }
    else if(t==='type'){ const k=T('hello','type'); rec.get=()=>({after,act:'type',text:k.val()}); }
    else if(t==='secret'){ const k=T('password','name'); rec.get=()=>({after,act:'secret',text:k.val()}); }
    else if(t==='activate'){ const k=T('Rocket League','app'); rec.get=()=>({after,act:'activate',text:k.val()}); }
    else if(t==='launch'){ const k=T('steam://run/252950','app'); rec.get=()=>({after,act:'launch',text:k.val()}); }
    else if(t==='drag'){ const x=numIn(0),y=numIn(0),tx=numIn(80),ty=numIn(0);
      fields.append('from', x.el, y.el, '→ to', tx.el, ty.el);
      rec.get=()=>({after,act:'drag',x:x.val(),y:y.val(),toX:tx.val(),toY:ty.val()}); }
    else if(t==='repeat'){ const cnt=numIn(3);
      const isel=document.createElement('select'); isel.className='mini';
      for(const [v,l] of [['key','press key'],['wait','wait'],['click','click']]){ const o=document.createElement('option'); o.value=v; o.textContent=l; isel.appendChild(o); }
      const inner=document.createElement('span'); inner.className='addfields';
      function ir(){ inner.textContent=''; const it=isel.value;
        if(it==='key'){ const k=txtIn('e'); inner.append('press', k.el); rec.get=()=>({after,act:'repeat',count:cnt.val(),inner:'key',text:k.val()}); }
        else if(it==='wait'){ const ms=numIn(100); inner.append('wait', ms.el, 'ms'); rec.get=()=>({after,act:'repeat',count:cnt.val(),inner:'wait',ms:ms.val()}); }
        else { const x=numIn(0),y=numIn(0); inner.append('x', x.el, 'y', y.el); rec.get=()=>({after,act:'repeat',count:cnt.val(),inner:'click',x:x.val(),y:y.val()}); } }
      isel.onchange=ir; ir();
      fields.append('repeat', cnt.el, 'times:', isel, inner); }
    else { const x=numIn(0),y=numIn(0); fields.append('x', x.el, 'y', y.el); rec.get=()=>({after,act:t,x:x.val(),y:y.val()}); }
  }
  sel.onchange=render; render();
  const rm=document.createElement('button'); rm.type='button'; rm.className='del'; rm.textContent='✕'; rm.title='discard';
  rm.onclick=()=>{ row.remove(); adds=adds.filter(a=>a!==rec); };
  const lead=document.createElement('span'); lead.className='lbl'; lead.innerHTML='<span class="num">+</span>';
  row.append(lead, sel, fields, rm);
  rec.node=row; adds.push(rec);
  return row;
}
// OPEN is the IDENTITY of the play the Edit view has loaded — slug, app and scope, exactly as a
// Plays row carries them — so this view's Run says which play it means. Null when nothing is open,
// and the Run button is hidden then: there is nothing to run.
let OPEN=null;
async function loadEdit(){
  const r = await (await fetch('/api/route')).json();
  const box=document.getElementById('steps'); box.innerHTML=''; steps=[]; adds=[];
  OPEN = r.loaded ? {name:r.name, slug:r.slug, app:r.app, scope:r.scope} : null;
  document.getElementById('editrun').hidden = !r.loaded;
  if(!r.loaded){
    document.getElementById('rt').textContent='';
    document.getElementById('src').textContent='';
    box.innerHTML='<p class="hint">Pick a play to edit:</p>';
    // /api/routes, not /api/plays: this picker opens a play in the step editor, so it may only
    // offer plays the editor can actually load — the registered ones.
    const rows = await (await fetch('/api/routes')).json();
    const groups={}; (rows||[]).forEach(x=>{ const k=x.App||''; (groups[k]=groups[k]||[]).push(x); });
    const apps=Object.keys(groups).sort((a,b)=> a===''?1 : b===''?-1 : a.localeCompare(b));
    if(!apps.length){ box.innerHTML+='<p class="hint">No plays yet.</p>'; return; }
    for(const app of apps){
      const h=document.createElement('div'); h.className='grouphead'; h.textContent=app||'Global'; box.appendChild(h);
      groups[app].forEach(x=>{
        const d=document.createElement('div'); d.className='rowcard';
        const nm=document.createElement('span'); nm.className='nm'; nm.textContent=x.Name; d.appendChild(nm);
        const sp=document.createElement('span'); sp.className='spacer'; d.appendChild(sp);
        const b=document.createElement('button'); b.className='mini'; b.textContent='edit ✎'; b.onclick=()=>openRoute(x.Name);
        d.appendChild(b); box.appendChild(d);
      });
    }
    return;
  }
  document.getElementById('rt').textContent = ' · ' + r.name + (r.app? (' ['+r.app+']'):'');
  document.getElementById('src').textContent = r.source;
  let n=0;
  for(let idx=0; idx<r.steps.length; idx++){
    const s=r.steps[idx];
    const finalWait = (idx===r.steps.length-1 && s.kind==='wait');
    const rec={line:s.line, kind:s.kind, point:s.point, act:s.act, del:false, dragOn:false};
    const row=document.createElement('div');
    if(s.kind==='check'){
      // A SCREEN CONDITION — where the play says it begins, and where it must arrive. Shown so a
      // learned play reads as what it is, and given no ✕ or + because deleting half a condition
      // is not an edit anybody means to make.
      n++;
      row.className='check' + (s.depth? ' depth1':'');
      row.innerHTML='<span class="num">'+n+'.</span><span>◉ '+esc(s.label)+'</span>'+
        '<span class="why">screen check</span>';
      steps.push(rec); box.appendChild(row); continue;
    }
    if(s.kind==='action'){
      n++;
      row.className='action' + (s.depth? ' depth1':'');
      if(s.act==='repeat'){ // loop header: editable count
        const lbl=document.createElement('span'); lbl.className='lbl rep';
        lbl.innerHTML='<span class="num">'+n+'.</span>↻ Repeat';
        rec.cnt=document.createElement('input'); rec.cnt.type='number'; rec.cnt.min='1'; rec.cnt.value=s.count;
        const w=document.createElement('span'); w.className='rep'; w.append(rec.cnt); w.append(' times');
        row.append(lbl, w, ops(plusBtn(s.line,row), delBtn(rec,row)));
      } else {
        let labelText = LABELS[s.act] || s.label;
        const label=document.createElement('span'); label.className='lbl';
        label.innerHTML='<span class="num">'+n+'.</span><span>'+esc(labelText)+'</span>';
        row.appendChild(label);
        if(LABELS[s.act] && s.act!=='click' && s.act!=='move'){ // one-text-arg action → editable literal
          rec.txt=txtIn(s.text); row.appendChild(rec.txt.el);
        } else if(s.point){
          rec.xi=coord(s.x); rec.yi=coord(s.y);
          const xy=document.createElement('span'); xy.className='xy'; xy.append('x', rec.xi, 'y', rec.yi); row.appendChild(xy);
          if(s.canDrag){
            rec.txi=coord(s.x+80); rec.tyi=coord(s.y);
            const to=document.createElement('span'); to.className='to'; to.style.display='none'; to.append('→ to', rec.txi, rec.tyi);
            const btn=document.createElement('button'); btn.type='button'; btn.className='mini'; btn.textContent='drag'; btn.title='make this a click-and-drag';
            btn.onclick=()=>{ rec.dragOn=!rec.dragOn; to.style.display=rec.dragOn?'inline-flex':'none'; btn.classList.toggle('on', rec.dragOn); };
            const wrap=document.createElement('span'); wrap.className='dragbox'; wrap.append(btn, to); row.appendChild(wrap);
          }
        }
        row.append(ops(plusBtn(s.line,row), delBtn(rec,row)));
      }
    } else {
      row.className='wait' + (s.depth? ' depth1':'');
      rec.ms=document.createElement('input'); rec.ms.type='number'; rec.ms.min='0'; rec.ms.step='50'; rec.ms.value=s.ms;
      let keep=null;
      if(finalWait){ keep=document.createElement('span'); keep.className='keep'; keep.textContent='settle · keep'; keep.title='auto-added so the last action registers'; }
      // settle-keep text sits INSIDE the delete button, so the ✕ stays outermost and lines up.
      row.append('⏱ wait', rec.ms, 'ms', ops(plusBtn(s.line,row), keep, delBtn(rec,row, finalWait?FINAL_WAIT_WARN:'')));
    }
    steps.push(rec); box.appendChild(row);
  }
}
async function save(){
  const waits={}, repeats={}, points={}, deletes=[], drags={}, texts={};
  for(const s of steps){
    if(s.del){ deletes.push(s.line); continue; }
    if(s.act==='repeat'){ repeats[s.line]=Math.max(1, parseInt(s.cnt.value||'1',10)); continue; }
    if(s.kind==='wait'){ waits[s.line]=Math.max(0, parseInt(s.ms.value||'0',10)); continue; }
    if(s.txt){ texts[s.line]=s.txt.val(); continue; }
    if(s.point){
      const x=parseInt(s.xi.value||'0',10), y=parseInt(s.yi.value||'0',10);
      if(s.dragOn && s.txi){ drags[s.line]=[x, y, parseInt(s.txi.value||'0',10), parseInt(s.tyi.value||'0',10)]; }
      else { points[s.point]=[x, y]; }
    }
  }
  const box=document.getElementById('saveerr');
  const resp = await fetch('/api/save',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({waits, repeats, points, deletes, drags, texts, adds:adds.map(a=>a.get())})});
  if(!resp.ok){
    // REFUSED, and nothing was written. The compiler's own sentence stays on the page — a toast
    // that vanishes in two seconds is no use for a message you have to act on — and the edits
    // stay in the form, because reloading here would throw away the work AND the mistake.
    const why=(await resp.text()).trim();
    box.innerHTML='<b>Not saved — Marco will not accept this play</b>';
    box.appendChild(document.createTextNode(why||'the edit was refused'));
    box.hidden=false; banner('✗ Not saved', true);
    return;
  }
  box.hidden=true; box.textContent='';
  banner('✓ Saved successfully');
  loadEdit();
}
function flash(msg){ const s=document.getElementById('saved'); s.textContent=msg; setTimeout(()=>s.textContent='', 2500); }
// banner shows a slide-down toast (green ok / red error), e.g. on save.
function banner(msg, err){
  const b=document.getElementById('banner');
  b.textContent=msg; b.className='banner show' + (err?' err':'');
  clearTimeout(b._t); b._t=setTimeout(()=>{ b.className='banner' + (err?' err':''); }, 2200);
}
// ---- plays view (grouped by app) ----
//
// THE PRODUCT LISTING, and it decides nothing. Every word on a row — where the play came from,
// where it applies, whether anything can ask for it — arrives already worded from internal/plays
// via /api/plays. A page that computed its own standing would be a second account of the same
// file, which is how a play was once announced as living in a tab that structurally could not
// contain it.
//
// The view id stays 'routes' throughout (nav key, section id, container id): it is an identifier,
// not a word anybody reads.
let PLAYS={plays:[], bindings:[]};
async function loadRoutes(){
  const r = await (await fetch('/api/plays')).json();
  PLAYS = {plays: r.plays||[], bindings: r.bindings||[]};
  const box=document.getElementById('routes'); box.innerHTML='';
  const rows = PLAYS.plays;
  if(!rows.length){ box.innerHTML='<p class="hint">No plays yet — learn one from the overlay: <kbd>` + "`" + `</kbd> then type <b>learn &lt;name&gt;</b>.</p>'; return; }
  // group: app name → its plays; "" (global) sorts last under "Global".
  const groups={};
  rows.forEach(p=>{ const k=p.app||''; (groups[k]=groups[k]||[]).push(p); });
  const apps=Object.keys(groups).sort((a,b)=> a===''?1 : b===''?-1 : a.localeCompare(b));
  for(const app of apps){
    const h=document.createElement('div'); h.className='grouphead'; h.textContent = app || 'Global';
    box.appendChild(h);
    // ASKABLE FIRST, and the rest under their own sub-heading. A list that mixed them would be
    // telling somebody a play is available when asking for it by name will not find it.
    const askable=groups[app].filter(p=>p.askable), staged=groups[app].filter(p=>!p.askable);
    askable.forEach(p=> box.appendChild(playRow(p)));
    if(staged.length){
      const s=document.createElement('div'); s.className='subhead';
      s.textContent='Saved — nothing can ask for these yet';
      box.appendChild(s);
      staged.forEach(p=> box.appendChild(playRow(p)));
    }
  }
}
// hotkeyFor is the leader key that REACHES this play, or ''.
//
// A binding is a way in and never a row of its own: forgetting a play and unbinding a key are
// different acts, and a list that showed a hotkey as a play would invite doing one meaning the
// other. A chained command (` + "`" + `a then b` + "`" + `) reaches every play it names.
function hotkeyFor(p){
  for(const b of PLAYS.bindings){
    if(b.app && p.app && b.app!==p.app) continue;
    for(const c of (b.cmd||'').toLowerCase().split(' then ')){
      const t=c.trim();
      if(t && (t===p.name.toLowerCase() || t===p.slug.toLowerCase())) return b.key;
    }
  }
  return '';
}
function tag(cls, text, title){
  const s=document.createElement('span'); s.className='tag '+cls; s.textContent=text;
  if(title) s.title=title;
  return s;
}
function playRow(p){
  const d=document.createElement('div'); d.className='rowcard';
  const nm=document.createElement('span'); nm.className='nm'; nm.textContent=p.name;
  d.appendChild(nm);
  if(p.current) d.appendChild(tag('cur','open'));
  d.appendChild(tag(p.kind, p.kindWord, p.kindSays));
  d.appendChild(tag(p.life, p.lifeWord, p.lifeSays));
  d.appendChild(tag(p.scope, p.scopeWord, p.scopeSays));
  const hk=hotkeyFor(p);
  if(hk) d.appendChild(tag('hot','⌨ '+hk, 'a hotkey that runs this play — the key is the way in, not the play'));
  const sp=document.createElement('span'); sp.className='spacer'; d.appendChild(sp);
  if(p.askable){
    const sc=document.createElement('select'); sc.className='mini'; sc.title=p.scopeSays;
    for(const s of ['context','focus','global']){ const o=document.createElement('option'); o.value=s; o.textContent=s; if(s===p.scope) o.selected=true; sc.appendChild(o); }
    sc.onchange=()=>changeScope(p, sc.value, sc);
    const e=document.createElement('button'); e.className='mini'; e.textContent='edit'; e.onclick=()=>openRoute(p.name);
    const run=document.createElement('button'); run.className='mini'; run.textContent='▶ run'; run.title='runs for real (types/clicks)'; run.onclick=()=>doPlay(p);
    d.append(sc, e, run);
  } else if(p.registerable){
    // NO run and NO scope switcher on a staged row. Asking for a staged play cannot work, and
    // offering the button would be a lie; its scope is decided by registering it.
    const rg=document.createElement('button'); rg.className='mini'; rg.textContent='Register'; rg.title=p.lifeSays;
    rg.onclick=()=>registerPlay(p);
    d.append(rg);
  } else {
    // NOT registerable, so there is no button — an offer Marco cannot keep is worse than none.
    // The sentence from internal/plays says why.
    const s=document.createElement('span'); s.className='lifesays'; s.textContent=p.lifeSays;
    d.append(s);
  }
  const del=document.createElement('button'); del.className='del trash'; del.style.marginLeft='0';
  del.textContent='🗑 forget'; del.title='forget this play'; del.onclick=()=>forgetPlay(p);
  d.append(del);
  return d;
}
// registerPlay moves a saved play where Marco looks when you ask for it.
//
// It posts the SLUG the listing carried, never the shown name — the slug came from the phrase used
// when the play was saved, and re-deriving one from a display name can land on a different play.
async function registerPlay(p){
  const resp=await fetch('/api/register',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({slug:p.slug, app:p.app})});
  if(!resp.ok){
    // REFUSED. The play is still exactly where it was — saved, not askable — and the list is
    // reloaded so it says so. A green banner here would be the one lie staging exists to prevent.
    banner((await resp.text()).trim()||('could not register '+p.name), true);
    loadRoutes();
    return;
  }
  banner('✓ '+p.name+' is ready — you can ask for it now');
  loadRoutes();
}
async function forgetPlay(p){
  if(!confirm('Forget the play "'+p.name+'"? This removes its file, its recording, and where it came from.')) return;
  const resp=await fetch('/api/delete',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({name:p.name, slug:p.slug, app:p.app, scope:p.scope, staged:!p.registered})});
  if(!resp.ok){ banner((await resp.text()).trim()||('could not forget '+p.name), true); return; }
  flash('forgot '+p.name); loadRoutes();
}
async function changeScope(p, scope, sel){
  if(scope===p.scope){ return; }
  let app = p.app;
  if((scope==='context'||scope==='focus') && !app){
    app = prompt('Which app should this play be scoped to? (its exe/window name, e.g. rocketleague)');
    if(!app){ sel.value=p.scope; return; }
  }
  const resp = await fetch('/api/scope',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({name:p.name, slug:p.slug, curApp:p.app, curScope:p.scope, scope, app})});
  if(!resp.ok){ sel.value=p.scope; flash((await resp.text()).trim()||'scope change failed'); return; }
  flash(p.name+' → '+scope); loadRoutes();
}
async function openRoute(name){
  const resp = await fetch('/api/load',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name})});
  if(!resp.ok){ flash('could not open '+name); return; }
  nav('edit'); // switch to the editor (loadEdit runs from nav)
}
// doPlay runs a play BY THE IDENTITY THE ROW ALREADY HOLDS — slug, app and scope — never by its
// shown name. The name is derived from the slug and that derivation is not reversible (a name with
// an apostrophe does not round-trip), so posting the name would ask the engine to guess back
// something this page already knows. See doArgv.
// doPlay presses Run and then WAITS TO BE TOLD WHAT HAPPENED.
//
// It used to flash "running: X" and stop there, because the server answered ok the moment the
// process existed. A play the door declined, a play somebody stopped and a play that worked all
// looked identical. The words below are the engine's own six outcomes, the same ones the HUD
// renders — see internal/outcome.
async function doPlay(p){
  const resp = await fetch('/api/do',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({slug:p.slug, app:p.app, scope:p.scope})});
  if(!resp.ok){ flash((await resp.text()).trim()||('could not run '+p.name)); return; }
  const started = await resp.json();
  flash('running: '+p.name);
  if(!started.run){ return; }
  await followRun(started.run, p.name);
}
// SAID is how each outcome reads to a person. Six words in, six sentences out — and every one of
// them says something different, which is the whole reason the vocabulary is six words and not
// "ok" and "not ok".
const SAID = {
  performed:   n => 'ran: '+n,
  clarify:     n => 'Marco asked about '+n+' — answer it',
  refused:     n => 'refused: '+n,
  unavailable: n => 'nothing could take '+n,
  cancelled:   n => 'stopped: '+n,
  failed:      n => 'failed: '+n,
};
async function followRun(id, name){
  // Polled rather than streamed: a play may run for a minute, and an HTTP handler held open for
  // its length is one a browser gives up on for a run that is going perfectly well.
  for(let i=0;i<600;i++){
    await new Promise(r=>setTimeout(r,300));
    let s;
    try { const r = await fetch('/api/run?id='+encodeURIComponent(id)); if(!r.ok) return; s = await r.json(); }
    catch(_) { return; }
    if(!s.done) continue;
    const say = SAID[s.outcome] || (n => n+': '+(s.outcome||'finished'));
    flash(say(s.play||name) + (s.detail ? ' — '+s.detail : ''));
    return;
  }
}
// doOpenPlay runs the play the Edit view has open. Also an identity: /api/route carries its slug.
function doOpenPlay(){ if(OPEN) doPlay(OPEN); }
// ---- activity view ----
//
// WHAT MARCO HAS DONE, and it holds nothing. Every row arrives already worded from /api/activity,
// which reads the Director's own account of its actions and this session's clicked runs. The page
// keeps no list of its own: a second record of what Marco has done is exactly the thing this
// campaign has been removing everywhere else.
//
// Deleting the outcome class must fail TestActivityRendersTheSixOutcomeWords.
const OUTWORD={
  performed:'performed', clarify:'clarify', refused:'refused',
  unavailable:'unavailable', cancelled:'cancelled', failed:'failed',
};
async function loadActivity(){
  const box=document.getElementById('activity');
  if(!box) return;
  let r;
  try{ r = await (await fetch('/api/activity')).json(); }
  catch(e){ box.innerHTML='<p class="empty">Marco could not be asked.</p>'; return; }
  const rows=r.entries||[];
  if(!rows.length){
    // AN HONEST EMPTY STATE, and it says which of the two empties this is.
    box.innerHTML='<p class="empty">'+esc(r.why || 'Marco has not done anything yet.')+'</p>';
    return;
  }
  box.innerHTML = rows.map(function(e){
    const w = OUTWORD[e.outcome] || 'failed';
    return '<div class="act">'+
      '<span class="when">'+esc(e.when||'')+'</span>'+
      '<span class="what">'+esc(e.what||'')+'</span>'+
      '<span class="out '+w+'">'+esc(w)+'</span>'+
      (e.from==='here'?'<span class="tag">from this page</span>':'')+
      (e.detail?'<span class="why">'+esc(e.detail)+'</span>':'')+
      '</div>';
  }).join('');
}
// ---- advanced view: the Cast ----
//
// WHO COULD ACT, asked of this machine and rendered exactly as it answers. An absent roster
// renders as an absent roster: a table of plausible-looking rows for a question Marco could not
// ask is the one thing this surface must never produce.
//
// Deleting the empty state must fail TestTheCastRendersAnHonestEmptyState.
async function loadAdvanced(){
  const box=document.getElementById('cast');
  if(!box) return;
  box.innerHTML='<p class="empty">asking…</p>';
  let r;
  try{ r = await (await fetch('/api/cast')).json(); }
  catch(e){ box.innerHTML='<p class="empty">Marco could not be asked.</p>'; return; }
  const rows=r.cast||[];
  let head='';
  if(r.provider) head='<p class="hint">Accessibility provider: <b>'+esc(r.provider)+'</b></p>';
  else head='<p class="hint" style="color:var(--err)">No accessibility provider was found, so '+
    'nothing here can act on a control. Build one with setup.ps1, or name one with $MARCO_UIA_BRIDGE.</p>';
  if(!rows.length){
    box.innerHTML=head+'<p class="empty">'+esc(r.why||'Marco could not ask.')+'</p>';
    return;
  }
  let h=head+'<table class="cast"><tr><th>Actor</th><th>Provider</th><th>Where</th>'+
    '<th>Available</th><th>Why not</th></tr>';
  for(const c of rows){
    h+='<tr><td>'+esc(c.actor)+'</td><td>'+esc(c.provider||'—')+'</td><td>'+esc(c.where||'—')+'</td>'+
       '<td class="'+(c.available?'yes':'no')+'">'+(c.available?'yes':'no')+'</td>'+
       '<td>'+esc(c.why||'')+'</td></tr>';
  }
  h+='</table>';
  if(r.last) h+='<p class="hint">Most recent refusal in this process: '+esc(r.last)+'</p>';
  box.innerHTML=h;
}
// ---- bindings view ----
async function loadBindings(){
  const r = await (await fetch('/api/bindings')).json();
  const box=document.getElementById('bindings'); box.innerHTML='';
  const rows = r.bindings||[];
  if(!rows.length){ box.innerHTML='<p class="hint">No bindings yet.</p>'; return; }
  // group by app so overloaded keys read clearly (` + "`" + `e in one game vs another). These are
  // TRIGGERS, listed under their own tab; the plays they reach live in the Plays tab.
  const groups={}; rows.forEach(b=>{ const k=b.App||''; (groups[k]=groups[k]||[]).push(b); });
  const apps=Object.keys(groups).sort((a,b)=> a===''?1 : b===''?-1 : a.localeCompare(b));
  for(const app of apps){
    const h=document.createElement('div'); h.className='grouphead'; h.textContent=app||'Global'; box.appendChild(h);
    groups[app].forEach(b=>{
      const d=document.createElement('div'); d.className='rowcard';
      const k=document.createElement('span'); k.className='key'; k.textContent=b.Key;
      const nm=document.createElement('span'); nm.className='nm'; nm.textContent=b.Cmd;
      const sp=document.createElement('span'); sp.className='spacer';
      const x=document.createElement('button'); x.className='del'; x.style.marginLeft='0'; x.textContent='✕'; x.onclick=()=>unbind(b.App,b.Key);
      d.append(k, nm, sp, x); box.appendChild(d);
    });
  }
}
async function bindAdd(){
  const key=document.getElementById('bkey').value.trim(), cmd=document.getElementById('bcmd').value.trim();
  if(!key||!cmd){ flash('key + play'); return; }
  const resp=await fetch('/api/bind',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({app:'', key, cmd})}); // app:'' → server scopes to the play's app
  // The SERVER's sentence — "no play matches …" names the step that could never fire, which is
  // the only thing that tells the person what to type instead.
  if(!resp.ok){ flash((await resp.text()).trim()||'bind failed'); return; }
  const res=await resp.json();
  document.getElementById('bkey').value=''; document.getElementById('bcmd').value='';
  loadBindings(); flash('bound '+key+(res.app?(' · '+res.app):' · global'));
}
async function unbind(app,key){
  const res=await (await fetch('/api/unbind',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({app,key})})).json();
  if(res.ok) loadBindings();
}
// ---- startup ----
//
// THE FRONT DOOR IS PLAYS, not the step editor. ` + "`" + `marco edit "<name>"` + "`" + ` opens that play for
// editing because a play was named; ` + "`" + `marco ui` + "`" + ` with nothing named lands on the list of what
// Marco can do, because a step editor with nothing loaded answers no question a person arrived
// with. The server can still pin a view (` + "`" + `marco ui help` + "`" + `, ` + "`" + `marco ui plays` + "`" + `).
//
// Changing the no-play fallback to 'edit' must fail TestTheControlCentreLandsOnPlaysWithNoPlayNamed.
async function init(){
  const r = await (await fetch('/api/route')).json();
  nav(r.view || (r.loaded ? 'edit' : 'routes'));
}
init();
</script></body></html>`
