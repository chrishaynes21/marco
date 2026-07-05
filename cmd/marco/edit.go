package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// runEdit opens the Marco control center — a small local web app served from stdlib net/http
// (so the zero-dep engine rule holds). It opens on the named route's Edit view, where every step
// across the OS harness is editable (waits, coordinates, keys/hold/release, type, focus/launch,
// secrets, repeat blocks); a hamburger nav switches to Routes (browse / open / run), Bindings
// (leader hotkeys), and Help. Ctrl+C to stop.
//
//	marco edit "enter freeplay"
func runEdit(args []string) {
	name := strings.TrimSpace(strings.Join(args, " "))
	d := newDeps()
	ed := &editor{reg: d.Reg, app: appOf(d)}
	// With a name, open on that route's Edit view; without one (marco ui / bare marco edit) the
	// control center lands on the all-routes browser and no route is loaded yet.
	if name != "" {
		rt, ok := d.Reg.Resolve(appOf(d), name)
		if !ok {
			if rt, ok = findRouteByName(d.Reg, name); !ok {
				fmt.Fprintf(os.Stderr, "No route named %q. Known routes: run `marco routes`.\n", name)
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

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store") // always serve the current page (no stale JS)
		fmt.Fprint(w, editPage)
	})
	mux.HandleFunc("/api/route", ed.handleRoute)   // the active route's steps + source
	mux.HandleFunc("/api/save", ed.handleSave)     // write step edits back
	mux.HandleFunc("/api/routes", ed.handleRoutes) // every route (the Routes view)
	mux.HandleFunc("/api/load", ed.handleLoad)     // switch which route is being edited
	mux.HandleFunc("/api/do", ed.handleDo)         // run a route for real
	mux.HandleFunc("/api/bindings", ed.handleBindings)
	mux.HandleFunc("/api/bind", ed.handleBind)
	mux.HandleFunc("/api/unbind", ed.handleUnbind)
	mux.HandleFunc("/api/scope", ed.handleScope)   // move a route between context/focus/global
	mux.HandleFunc("/api/delete", ed.handleDelete) // delete a route

	where := "all routes"
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

// editor holds one control-center session: the registry, the app context, and the route
// currently open in the Edit view (swappable via /api/load).
type editor struct {
	reg  routes.Registry
	app  string // foreground app when launched — the default scope for new bindings
	mu   sync.Mutex
	rt   routes.Route
	path string
	src  string
}

// loadSrc reads the active route's source into e.src.
func (e *editor) loadSrc() error {
	b, err := os.ReadFile(e.path)
	if err != nil {
		return err
	}
	e.src = string(b)
	return nil
}

// scopeOf names a route's scope for display: focus (anywhere, switches), context (only in-app),
// or global (app-less, anywhere).
func scopeOf(rt routes.Route) string {
	switch {
	case rt.Focus:
		return "focus"
	case rt.App != "":
		return "context"
	default:
		return "global"
	}
}

// handleRoutes lists every route (name, app, scope) plus which one is open — the Routes view.
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

// handleLoad switches the Edit view to another route by name, reloading its source.
func (e *editor) handleLoad(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name string }
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "bad request", 400)
		return
	}
	rt, ok := e.reg.Resolve(e.app, req.Name)
	if !ok {
		if rt, ok = findRouteByName(e.reg, req.Name); !ok {
			http.Error(w, "no such route", 404)
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

// handleDo runs a route for real (types/clicks) by spawning a fresh marco process, like the
// overlay and web-ui do — the engine stays out of this server's process.
func (e *editor) handleDo(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name string }
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "bad request", 400)
		return
	}
	self, err := os.Executable()
	if err != nil {
		self = os.Args[0]
	}
	if err := exec.Command(self, "do", req.Name).Start(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// handleBindings lists every leader-hotkey binding (key → command, scoped to an app).
func (e *editor) handleBindings(w http.ResponseWriter, _ *http.Request) {
	type row struct{ App, Key, Cmd string }
	var rows []row
	for _, b := range e.reg.Bindings() {
		cmd := b.Cmd
		if cmd == "" {
			cmd = b.Slug // legacy single-route binding
		}
		rows = append(rows, row{b.App, b.Key, cmd})
	}
	writeJSON(w, map[string]any{"bindings": rows, "app": e.app})
}

// handleBind binds `leader+key` to a command. The scope is the ROUTE's app, so a hotkey only
// fires while that app is in front — the same key can drive different macros in different games
// (overloading). When App isn't given, it's inferred from the command's first route (a global
// route → a global binding). Pass App explicitly to override.
func (e *editor) handleBind(w http.ResponseWriter, r *http.Request) {
	var req struct{ App, Key, Cmd string }
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Key == "" || strings.TrimSpace(req.Cmd) == "" {
		http.Error(w, "key and command required", 400)
		return
	}
	app := strings.TrimSpace(req.App)
	if app == "" {
		first := strings.TrimSpace(req.Cmd)
		if i := strings.Index(strings.ToLower(first), " then "); i >= 0 {
			first = strings.TrimSpace(first[:i]) // scope to the first route in a chain
		}
		if rt, ok := findRouteByName(e.reg, first); ok {
			app = rt.App // "" for a global route → a global binding
		}
	}
	if err := e.reg.Bind(app, strings.ToLower(req.Key), strings.TrimSpace(req.Cmd)); err != nil {
		http.Error(w, err.Error(), 500)
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

// handleDelete removes a route (its .marco, recording, and any anchor templates). CurApp/Scope
// identify which one when the same slug exists in several scopes.
func (e *editor) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name, App, Scope string }
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "bad request", 400)
		return
	}
	rt := routes.Route{App: req.App, Focus: req.Scope == "focus", Slug: routes.Slug(req.Name)}
	if !e.reg.Has(rt) {
		http.Error(w, "no such route", 404)
		return
	}
	if err := e.reg.Delete(rt); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	e.mu.Lock()
	if e.rt == rt { // the open route is gone — fall back to the browser
		e.rt, e.path, e.src = routes.Route{}, "", ""
	}
	e.mu.Unlock()
	writeJSON(w, map[string]any{"ok": true})
}

// handleScope moves a route between scopes — context (in-app), focus (anywhere, switches to the
// app), or global (app-less). It relocates the .marco (and its recording) to the new scope dir:
// save at the destination, carry the recording, delete the source. CurApp/CurScope identify which
// route (same slug can exist in several scopes); App is the destination app (context/focus need one).
func (e *editor) handleScope(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name, CurApp, CurScope, Scope, App string }
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "bad request", 400)
		return
	}
	from := routes.Route{App: req.CurApp, Focus: req.CurScope == "focus", Slug: routes.Slug(req.Name)}
	if !e.reg.Has(from) {
		http.Error(w, "no such route", 404)
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
		http.Error(w, "a route already exists at that scope", 409)
		return
	}
	src, err := os.ReadFile(e.reg.Path(from))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := e.reg.Save(to, string(src)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if rec, ok := e.reg.LoadRecording(from); ok {
		_ = e.reg.SaveRecording(to, rec)
	}
	if err := e.reg.Delete(from); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	e.mu.Lock()
	if e.rt == from { // keep the open route pointing at its new home
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
// release (KeyDown/KeyUp), typed text (Type), a named Secret, and app focus/launch
// (Activate/Launch). Captured: the verb, then the literal.
var textActRE = regexp.MustCompile(`^do OS's (Key|KeyDown|KeyUp|Type|Secret|Activate|Launch) with "(.*)"\.`)

// repeatRE matches a `repeat N times...` loop header (any indent). Captured: indent, count.
var repeatRE = regexp.MustCompile(`^(\s*)repeat (\d+) times\.\.\.\s*$`)

// pointVals holds a Point's editable coordinates.
type pointVals struct {
	X, Y, RelX, RelY int
	HasRel           bool
}

// step is one entry shown in the editor, in route order: an ACTION (read-only label) or a WAIT
// (editable ms). A click/move action also carries the Point it targets and that point's X,Y,
// which the editor exposes as editable coordinates.
type step struct {
	Kind    string `json:"kind"` // "action" | "wait"
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

// parseSteps walks the route body and returns the ordered step sequence the editor shows: the
// top-level actions/waits, plus one level of `repeat N times...` block — its header (editable
// count) and its indented body steps (Depth 1). A Find block's `when ok?/or?` arms and the
// point/anchor declarations stay hidden. Each editable action carries its subtype + fields.
func (e *editor) parseSteps() []step {
	pts := e.parsePoints()
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
		// A top-level `repeat N times...` header: editable count, body follows indented.
		if m := repeatRE.FindStringSubmatch(line); m != nil && indent == 4 {
			n, _ := strconv.Atoi(m[2])
			steps = append(steps, step{Kind: "action", Act: "repeat", Count: n,
				Label: fmt.Sprintf("Repeat %d times", n), Line: i, EndLine: i})
			repeatBodyIndent, repeatHdr = 8, len(steps)-1
			continue
		}
		show := indent == 4 || inBody
		if !show {
			continue
		}
		if m := sleepRE.FindStringSubmatch(line); m != nil {
			ms, _ := strconv.Atoi(m[2])
			steps = append(steps, step{Kind: "wait", Ms: ms, Depth: depth, Line: i})
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
				// A one-text-arg action (key/hold/release/type/secret/activate/launch) → expose
				// the literal for in-place editing.
				s.Act, s.Text = strings.ToLower(tm[1]), unquoteLit(tm[2])
			}
			steps = append(steps, s)
		}
	}
	return steps
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
		"name":   prettyRoute(e.rt.Slug),
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

// handleSave rebuilds the source from the edits and writes it back.
func (e *editor) handleSave(w http.ResponseWriter, r *http.Request) {
	var req saveReq
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "bad request", 400)
		return
	}
	updated := e.rebuild(req)
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
	for i, line := range lines {
		// End-of-body adds land just before the closing `this is ok!`.
		if strings.TrimSpace(line) == "this is ok!" && len(endAdds) > 0 {
			flush(endAdds, indentOf(line))
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

// findRouteByName matches a route across all scopes by its pretty name or slug (case-insensitive).
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
 button.go{padding:9px 18px;background:var(--accent);color:#04110f;border:0;border-radius:6px;cursor:pointer;font:inherit;font-weight:700;letter-spacing:1px}
 button.go:hover{box-shadow:0 0 14px rgba(0,229,204,.5)}
 details{margin-top:22px;color:var(--dim)}
 pre{background:#070708;padding:12px;border:1px solid var(--line);border-radius:6px;white-space:pre-wrap;font:13px var(--mono);color:#b8c4c2}
 .grouphead{color:var(--accent);font-size:12px;letter-spacing:2px;text-transform:uppercase;margin:18px 0 6px;opacity:.85}
 .rowcard{display:flex;gap:10px;align-items:center;padding:10px 12px;border:1px solid var(--line);border-radius:6px;margin:6px 0;background:var(--panel)}
 .rowcard .nm{color:var(--text)}
 .tag{font-size:11px;letter-spacing:1px;text-transform:uppercase;padding:2px 7px;border-radius:4px;border:1px solid var(--line);color:var(--dim)}
 .tag.context{color:var(--accent);border-color:#12312e}
 .tag.focus{color:var(--listen);border-color:#12283a}
 .tag.global{color:var(--amber);border-color:#3a3018}
 .tag.cur{color:#04110f;background:var(--accent);border-color:var(--accent)}
 .spacer{margin-left:auto}
 .key{display:inline-block;min-width:20px;text-align:center;padding:2px 7px;border:1px solid var(--line);border-radius:4px;color:var(--accent);background:#0d0e10}
 input.field{padding:7px 9px;background:#0d0e10;border:1px solid var(--line);color:var(--text);border-radius:6px;font:inherit}
 kbd{padding:1px 6px;border:1px solid var(--line);border-bottom-width:2px;border-radius:4px;color:var(--accent);background:#0d0e10;font:12px var(--mono)}
 .help h3{color:var(--text);font-size:13px;letter-spacing:1px;text-transform:uppercase;margin:18px 0 6px}
 .help p,.help li{color:var(--dim);font-size:13px}
 a.plain{color:var(--accent);cursor:pointer;text-decoration:none}
 .banner{position:fixed;top:14px;left:50%;transform:translateX(-50%) translateY(-70px);background:var(--run);color:#04110f;
   padding:11px 22px;border-radius:8px;font-weight:700;letter-spacing:1px;box-shadow:0 6px 24px rgba(0,0,0,.55);
   opacity:0;transition:transform .25s ease,opacity .25s ease;z-index:50;pointer-events:none}
 .banner.show{transform:translateX(-50%) translateY(0);opacity:1}
 .banner.err{background:var(--err);color:#fff}
</style></head><body>
<div id="banner" class="banner"></div>
<header>
  <button class="burger" onclick="toggleNav()">☰</button>
  <span class="brand">MARCO<span class="sub" id="rt">…</span></span>
  <span id="saved" class="saved"></span>
</header>
<nav id="drawer">
  <a data-view="edit" class="active" onclick="nav('edit')">◆ Edit</a>
  <a data-view="routes" onclick="nav('routes')">▤ Routes</a>
  <a data-view="bindings" onclick="nav('bindings')">⌨ Bindings</a>
  <a data-view="help" onclick="nav('help')">? Help</a>
</nav>
<div id="scrim" onclick="closeNav()"></div>
<main>
 <section id="view-edit">
  <h2>Edit route</h2>
  <p class="hint">Every step is editable in place — wait (ms), coordinates (x, y), the key a
    <b>Press/Hold/Release</b> sends, <b>Type</b> text, a <b>Focus/Launch</b> app, a <b>Secret</b> name,
    or a <b>Repeat</b> count. <b>+</b> inserts after; <b>✕</b> deletes; <b>drag</b> turns a click
    into a click-drag. The last <b>settle · keep</b> wait lets the final action land before the run ends.</p>
  <div id="steps"></div>
  <div class="bar"><button class="go" onclick="save()">Save</button>
    <button type="button" class="mini" onclick="document.getElementById('steps').appendChild(addRow(-1))">+ add step</button>
    <span id="saved2"></span></div>
  <details><summary>Full route source</summary><pre id="src"></pre></details>
 </section>
 <section id="view-routes" hidden><h2>Routes</h2><div id="routes"></div></section>
 <section id="view-bindings" hidden>
  <h2>Bindings — leader hotkeys</h2>
  <p class="hint">Press the leader (<kbd>` + "`" + `</kbd>) then a key to fire a route. A binding <b>scopes to its
    route's app</b>, so <kbd>` + "`" + `</kbd><kbd>e</kbd> can run one macro in Rocket League and another
    elsewhere. Global routes bind everywhere.</p>
  <div class="bar"><input class="field" id="bkey" placeholder="key (e.g. e)" style="width:110px">
    <input class="field" id="bcmd" placeholder="route (e.g. enter freeplay)" style="flex:1;min-width:160px">
    <button class="go" onclick="bindAdd()">Bind</button></div>
  <div id="bindings" style="margin-top:14px"></div>
 </section>
 <section id="view-help" hidden class="help"><h2>Help</h2>
  <h3>Overlay hotkeys</h3>
  <ul><li><kbd>` + "`" + `</kbd> leader, then <kbd>m</kbd> opens the command line; type a route + Enter.</li>
   <li><kbd>` + "`" + `</kbd> then a bound key (e.g. <kbd>0</kbd>) runs its macro.</li>
   <li><kbd>` + "`" + `</kbd> while a route runs = stop/cancel, and the leader also finishes a teach.</li></ul>
  <h3>Commands (type or say)</h3>
  <ul><li><b>teach &lt;name&gt;</b> — record a new route. <b>edit &lt;name&gt;</b> — open this editor.</li>
   <li><b>bind &lt;key&gt; &lt;route&gt;</b> / <b>unbind &lt;key&gt;</b> — hotkeys.</li>
   <li><b>voice on|off</b>, <b>mute</b>, <b>stop listening</b> — the mic.</li></ul>
  <h3>The settle wait</h3>
  <p>New routes end with a short <b>settle</b> wait so the final click registers before the run ends. Bump it for slow screens; deleting it can make the last step flaky.</p>
  <h3>OS harness</h3>
  <p>Steps map to the OS host: click / move / drag, press / hold / release a key, type, focus / launch an app, a secret, a wait, and repeat-N-times blocks. CV is off in alpha — coordinates + timings drive everything.</p>
 </section>
</main>
<script>
const LABELS={key:'Press',keydown:'Hold',keyup:'Release',type:'Type',secret:'Secret',activate:'Focus',launch:'Launch'};
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
}
// ---- edit view ----
const FINAL_WAIT_WARN='This is the final settle wait. Deleting it can make the last step (e.g. a click) fire and the route end before the app processes it, so it may not register. Delete anyway?';
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
async function loadEdit(){
  const r = await (await fetch('/api/route')).json();
  const box=document.getElementById('steps'); box.innerHTML=''; steps=[]; adds=[];
  if(!r.loaded){
    document.getElementById('rt').textContent='';
    document.getElementById('src').textContent='';
    box.innerHTML='<p class="hint">Pick a route to edit:</p>';
    const rows = await (await fetch('/api/routes')).json();
    const groups={}; (rows||[]).forEach(x=>{ const k=x.App||''; (groups[k]=groups[k]||[]).push(x); });
    const apps=Object.keys(groups).sort((a,b)=> a===''?1 : b===''?-1 : a.localeCompare(b));
    if(!apps.length){ box.innerHTML+='<p class="hint">No routes yet.</p>'; return; }
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
  const res = await (await fetch('/api/save',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({waits, repeats, points, deletes, drags, texts, adds:adds.map(a=>a.get())})})).json();
  if(res.ok){ banner('✓ Saved successfully'); } else { banner('✗ Save failed', true); }
  loadEdit();
}
function flash(msg){ const s=document.getElementById('saved'); s.textContent=msg; setTimeout(()=>s.textContent='', 2500); }
// banner shows a slide-down toast (green ok / red error), e.g. on save.
function banner(msg, err){
  const b=document.getElementById('banner');
  b.textContent=msg; b.className='banner show' + (err?' err':'');
  clearTimeout(b._t); b._t=setTimeout(()=>{ b.className='banner' + (err?' err':''); }, 2200);
}
// ---- routes view (grouped by app) ----
async function loadRoutes(){
  const rows = await (await fetch('/api/routes')).json();
  const box=document.getElementById('routes'); box.innerHTML='';
  // group: app name → its routes; "" (global) sorts last under "Global".
  const groups={};
  (rows||[]).forEach(r=>{ const k=r.App||''; (groups[k]=groups[k]||[]).push(r); });
  const apps=Object.keys(groups).sort((a,b)=> a===''?1 : b===''?-1 : a.localeCompare(b));
  if(!apps.length){ box.innerHTML='<p class="hint">No routes yet — teach one from the overlay: <kbd>` + "`" + `</kbd> then type <b>teach &lt;name&gt;</b>.</p>'; return; }
  for(const app of apps){
    const h=document.createElement('div'); h.className='grouphead'; h.textContent = app || 'Global';
    box.appendChild(h);
    groups[app].forEach(r=> box.appendChild(routeRow(r)));
  }
}
function routeRow(r){
  const d=document.createElement('div'); d.className='rowcard';
  const nm=document.createElement('span'); nm.className='nm'; nm.textContent=r.Name;
  d.appendChild(nm);
  if(r.Current){ const c=document.createElement('span'); c.className='tag cur'; c.textContent='open'; d.appendChild(c); }
  const sp=document.createElement('span'); sp.className='spacer'; d.appendChild(sp);
  // scope switcher
  const sc=document.createElement('select'); sc.className='mini'; sc.title='scope';
  for(const s of ['context','focus','global']){ const o=document.createElement('option'); o.value=s; o.textContent=s; if(s===r.Scope) o.selected=true; sc.appendChild(o); }
  sc.onchange=()=>changeScope(r, sc.value, sc);
  const e=document.createElement('button'); e.className='mini'; e.textContent='edit'; e.onclick=()=>openRoute(r.Name);
  const run=document.createElement('button'); run.className='mini'; run.textContent='▶ run'; run.title='runs for real (types/clicks)'; run.onclick=()=>doRoute(r.Name);
  const del=document.createElement('button'); del.className='del trash'; del.style.marginLeft='0'; del.textContent='🗑 delete'; del.title='delete route'; del.onclick=()=>delRoute(r);
  d.append(sc, e, run, del);
  return d;
}
async function delRoute(r){
  if(!confirm('Delete route "'+r.Name+'"? This removes its .marco and its recording.')) return;
  const resp=await fetch('/api/delete',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({name:r.Name, app:r.App, scope:r.Scope})});
  if(!resp.ok){ flash('delete failed'); return; }
  flash('deleted '+r.Name); loadRoutes();
}
async function changeScope(r, scope, sel){
  if(scope===r.Scope){ return; }
  let app = r.App;
  if((scope==='context'||scope==='focus') && !app){
    app = prompt('Which app should this route be scoped to? (its exe/window name, e.g. rocketleague)');
    if(!app){ sel.value=r.Scope; return; }
  }
  const resp = await fetch('/api/scope',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({name:r.Name, curApp:r.App, curScope:r.Scope, scope, app})});
  if(!resp.ok){ sel.value=r.Scope; flash((await resp.text()).trim()||'scope change failed'); return; }
  flash(r.Name+' → '+scope); loadRoutes();
}
async function openRoute(name){
  const resp = await fetch('/api/load',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name})});
  if(!resp.ok){ flash('could not open '+name); return; }
  nav('edit'); // switch to the editor (loadEdit runs from nav)
}
async function doRoute(name){
  const res = await (await fetch('/api/do',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name})})).json();
  flash(res.ok ? ('running: '+name) : 'run failed');
}
// ---- bindings view ----
async function loadBindings(){
  const r = await (await fetch('/api/bindings')).json();
  const box=document.getElementById('bindings'); box.innerHTML='';
  const rows = r.bindings||[];
  if(!rows.length){ box.innerHTML='<p class="hint">No bindings yet.</p>'; return; }
  // group by app so overloaded keys read clearly (` + "`" + `e in one game vs another)
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
  if(!key||!cmd){ flash('key + route'); return; }
  const resp=await fetch('/api/bind',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({app:'', key, cmd})}); // app:'' → server scopes to the route's app
  if(!resp.ok){ flash('bind failed'); return; }
  const res=await resp.json();
  document.getElementById('bkey').value=''; document.getElementById('bcmd').value='';
  loadBindings(); flash('bound '+key+(res.app?(' · '+res.app):' · global'));
}
async function unbind(app,key){
  const res=await (await fetch('/api/unbind',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({app,key})})).json();
  if(res.ok) loadBindings();
}
// ---- startup: land on the open route's editor, or the all-routes browser (marco ui) ----
async function init(){
  const r = await (await fetch('/api/route')).json();
  nav(r.loaded ? 'edit' : 'routes');
}
init();
</script></body></html>`
