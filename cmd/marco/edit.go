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

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// runEdit opens a small local web editor for a saved route's TIMINGS and CLICK COORDINATES —
// the `do OS's Sleep with N.` waits between actions (the pacing that matters most now CV is off
// by default) and each click's X,Y. It resolves the route, serves an editor page, opens the
// browser, and writes the edits back to the route file. Self-contained (net/http is stdlib, so
// the zero-dep engine rule holds); Ctrl+C to stop.
//
//	marco edit "enter freeplay"
func runEdit(args []string) {
	name := strings.TrimSpace(strings.Join(args, " "))
	if name == "" {
		fmt.Fprintln(os.Stderr, `usage: marco edit "<route name>"`)
		os.Exit(2)
	}
	d := newDeps()
	rt, ok := d.Reg.Resolve(appOf(d), name)
	if !ok {
		// Fall back to a name/slug match across ALL scopes — you may be editing a route for an
		// app that isn't in the foreground.
		if rt, ok = findRouteByName(d.Reg, name); !ok {
			fmt.Fprintf(os.Stderr, "No route named %q. Known routes: run `marco routes`.\n", name)
			os.Exit(1)
		}
	}
	ed := &editor{reg: d.Reg, rt: rt, path: d.Reg.Path(rt)}
	src, err := os.ReadFile(ed.path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", ed.path, err)
		os.Exit(1)
	}
	ed.src = string(src)

	ln, err := net.Listen("tcp", "localhost:0") // any free port
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	url := fmt.Sprintf("http://%s/", ln.Addr().String())

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, editPage)
	})
	mux.HandleFunc("/api/route", ed.handleRoute)
	mux.HandleFunc("/api/save", ed.handleSave)

	fmt.Printf("editing %q → %s  (Ctrl+C when done)\n", prettyRoute(rt.Slug), url)
	openBrowser(url)
	if err := http.Serve(ln, mux); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// editor holds one route-editing session.
type editor struct {
	reg  routes.Registry
	rt   routes.Route
	path string
	src  string
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

// textActRE matches a top-level action whose only argument is a string literal the editor can
// expose for in-place editing: `do OS's Key with "…"` (a keypress) and `do OS's Type with "…"`.
var textActRE = regexp.MustCompile(`^do OS's (Key|Type) with "(.*)"\.`)

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
	Act     string `json:"act,omitempty"`   // action subtype the editor can edit: click|move|key|type
	Ms      int    `json:"ms,omitempty"`    // wait
	Point   string `json:"point,omitempty"` // action: the Point name it clicks (empty otherwise)
	X       int    `json:"x"`               // action: that point's coordinates (when Point != "")
	Y       int    `json:"y"`
	Text    string `json:"text,omitempty"` // key/type action: the editable string literal
	CanDrag bool   `json:"canDrag"`        // a click/move at a point can be converted to a drag
	Line    int    `json:"line"`           // source line index (keys delete / wait / drag ops)
}

// parseSteps walks the route body and returns the ordered action/wait sequence for display.
// Only TOP-LEVEL body lines (a `do …` action) are shown — nested `when ok?/or?` arms of a Find
// and the point/anchor declarations are context the editor hides. Waits become editable.
func (e *editor) parseSteps() []step {
	pts := e.parsePoints()
	var steps []step
	for i, line := range strings.Split(e.src, "\n") {
		t := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if m := sleepRE.FindStringSubmatch(line); m != nil {
			ms, _ := strconv.Atoi(m[2])
			steps = append(steps, step{Kind: "wait", Ms: ms, Line: i})
			continue
		}
		// A top-level action is a `do …` at the body indent (4 spaces). Deeper `do …` lines are
		// the found/fallback arms inside a Find block — part of that step, not separate.
		if indent == 4 && strings.HasPrefix(t, "do ") {
			s := step{Kind: "action", Label: humanizeAction(t), Line: i}
			// A click/move at a named point → attach coords for editing, allow drag conversion.
			if cm := clickPointRE.FindStringSubmatch(t); cm != nil {
				if pv, ok := pts[cm[2]]; ok {
					s.Act = strings.ToLower(cm[1]) // "click" | "move"
					s.Point, s.X, s.Y, s.CanDrag = cm[2], pv.X, pv.Y, true
				}
			} else if tm := textActRE.FindStringSubmatch(t); tm != nil {
				// A keypress / typed text → expose the literal for in-place editing.
				s.Act, s.Text = strings.ToLower(tm[1]), unquoteLit(tm[2]) // "key" | "type"
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
	case strings.HasPrefix(s, "OS's Find") || strings.HasPrefix(s, "Text's Find") || strings.HasPrefix(s, "Vision's Locate"):
		return "Find target, then click"
	}
	return s
}

func (e *editor) handleRoute(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"name":   prettyRoute(e.rt.Slug),
		"app":    e.rt.App,
		"path":   e.path,
		"steps":  e.parseSteps(),
		"source": e.src,
	})
}

// saveReq is the editor's save payload. Everything is keyed by SOURCE LINE INDEX (or point
// name), so operations don't shift when steps are deleted.
type saveReq struct {
	Waits   map[string]int    `json:"waits"`   // line → new ms
	Points  map[string][2]int `json:"points"`  // point name → [x, y]
	Texts   map[string]string `json:"texts"`   // line → new key/type literal
	Deletes []int             `json:"deletes"` // line indexes to remove (the step's line)
	Drags   map[string][4]int `json:"drags"`   // line → [fromX, fromY, toX, toY] (click → drag)
	Adds    []addStep         `json:"adds"`    // new steps to insert
}

// addStep is one new command the editor inserts. After is the source-line index to insert it
// AFTER (-1 = at the end of the body, just before `this is ok!`). Act selects which fields matter.
type addStep struct {
	After int    `json:"after"`
	Act   string `json:"act"` // wait | click | move | key | type | drag
	Ms    int    `json:"ms"`
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
	case "type":
		return []string{fmt.Sprintf(`%sdo OS's Type with "%s".`, indent, quoteLit(a.Text))}
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

const editPage = `<!doctype html><html><head><meta charset="utf-8"><title>marco · edit timings</title>
<style>
 body{font:15px system-ui;margin:0;background:#1e1e22;color:#e6e6e6}
 header{padding:14px 18px;background:#26262c;border-bottom:1px solid #34343c;display:flex;justify-content:space-between;align-items:center}
 #name{color:#9ad;font-weight:600}
 #saved{color:#5cb85c;font-size:13px}
 main{padding:18px;max-width:640px}
 .action{padding:8px 12px;border:1px solid #34343c;border-radius:6px;margin:6px 0;display:flex;gap:8px;align-items:center}
 .num{color:#666;min-width:22px}
 .lbl{display:flex;gap:8px;align-items:center}
 .xy,.to{color:#7aacc0;display:inline-flex;align-items:center;gap:5px;font-size:13px}
 .coord{width:64px;padding:5px 6px;background:#26262c;border:1px solid #3a3a44;color:#8dd;border-radius:5px;font:inherit;text-align:right}
 .dragbox{display:inline-flex;align-items:center;gap:6px}
 .mini{background:#3a4a5a;color:#cde;padding:4px 9px;border:0;border-radius:5px;cursor:pointer;font:13px system-ui}
 .mini.on{background:#3a6ea5;color:#fff}
 .del{margin-left:auto;background:#4a3030;color:#e6a0a0;padding:4px 9px;border:0;border-radius:5px;cursor:pointer;font:inherit}
 .del:hover{background:#6a4040}
 .deleted{opacity:.45;text-decoration:line-through}
 .added{border-color:#3a5a3a;background:#232a23}
 .tin{width:150px;padding:5px 7px;background:#26262c;border:1px solid #3a3a44;color:#8dd;border-radius:5px;font:inherit;text-align:left}
 .addfields{display:inline-flex;align-items:center;gap:6px;color:#7aacc0;font-size:13px}
 select.mini{background:#3a4a5a;color:#cde;border:0;border-radius:5px;padding:4px 6px;font:13px system-ui}
 .plus{background:#2f4a2f;color:#bfe0bf}
 .wait .del{margin-left:8px}
 .wait{display:flex;align-items:center;gap:8px;margin:2px 0 2px 30px;color:#e0b050}
 .wait input{width:90px;padding:6px 8px;background:#26262c;border:1px solid #3a3a44;color:#e0b050;border-radius:6px;font:inherit;text-align:right}
 .bar{margin-top:18px;display:flex;gap:10px;align-items:center}
 button{padding:9px 16px;background:#3a6ea5;color:#fff;border:0;border-radius:6px;cursor:pointer;font:inherit}
 button:hover{background:#4a7eb5}
 details{margin-top:22px;color:#aaa}
 pre{background:#141417;padding:12px;border-radius:6px;white-space:pre-wrap;font:13px ui-monospace,monospace;color:#cfcfcf}
 .hint{color:#888;font-size:13px;margin:0 0 14px}
</style></head><body>
<header><span>marco · edit timings — <span id="name">…</span></span><span id="saved"></span></header>
<main>
 <p class="hint">Edit each step in place — wait (ms), click/move coordinates (x, y), or the key
   a <b>Press</b> sends and the text a <b>Type</b> enters. <b>+</b> adds a step after this one;
   <b>✕</b> deletes; <b>drag</b> turns a click into a click-and-drag. With CV off, these
   coordinates + timings are the whole route.</p>
 <div id="steps"></div>
 <div class="bar"><button onclick="save()">Save</button>
   <button type="button" class="mini" onclick="document.getElementById('steps').appendChild(addRow(-1))">+ add step at end</button>
   <span id="saved2"></span></div>
 <details><summary>Full route source</summary><pre id="src"></pre></details>
</main>
<script>
let steps = [];  // existing rows (keyed by source line)
let adds  = [];  // new steps to insert: {after, node, get:()=>payload}
function coord(v){ const i=document.createElement('input'); i.type='number'; i.className='coord'; i.value=v; return i; }
function numIn(v){ const i=document.createElement('input'); i.type='number'; i.className='coord'; i.value=v; return {el:i, val:()=>parseInt(i.value||'0',10)}; }
function txtIn(v){ const i=document.createElement('input'); i.className='tin'; i.value=v; return {el:i, val:()=>i.value}; }
function delBtn(rec, row){
  const b=document.createElement('button'); b.type='button'; b.className='del'; b.title='delete this step'; b.textContent='✕';
  b.onclick=()=>{ rec.del=!rec.del; row.classList.toggle('deleted', rec.del); };
  return b;
}
function plusBtn(afterLine, row){
  const b=document.createElement('button'); b.type='button'; b.className='mini plus'; b.title='add a step after this'; b.textContent='+';
  b.onclick=()=>{ row.after(addRow(afterLine)); };
  return b;
}
// addRow builds an editable "new step" row; its type <select> swaps the relevant fields, and its
// get() returns the save payload. after<0 means append at the end of the body.
function addRow(after){
  const rec={after};
  const row=document.createElement('div'); row.className='action added';
  const sel=document.createElement('select'); sel.className='mini';
  for(const [v,l] of [['wait','wait'],['click','click'],['move','move cursor'],['key','press key'],['type','type text'],['drag','drag']]){
    const o=document.createElement('option'); o.value=v; o.textContent=l; sel.appendChild(o);
  }
  const fields=document.createElement('span'); fields.className='addfields';
  function render(){
    fields.textContent=''; const t=sel.value;
    if(t==='wait'){ const ms=numIn(200); fields.append('wait', ms.el, 'ms'); rec.get=()=>({after, act:'wait', ms:ms.val()}); }
    else if(t==='key'){ const k=txtIn('enter'); fields.append('press', k.el); rec.get=()=>({after, act:'key', text:k.val()}); }
    else if(t==='type'){ const k=txtIn('hello'); fields.append('type', k.el); rec.get=()=>({after, act:'type', text:k.val()}); }
    else if(t==='drag'){ const x=numIn(0),y=numIn(0),tx=numIn(80),ty=numIn(0);
      fields.append('from', x.el, y.el, '→ to', tx.el, ty.el);
      rec.get=()=>({after, act:'drag', x:x.val(), y:y.val(), toX:tx.val(), toY:ty.val()}); }
    else { const x=numIn(0),y=numIn(0); fields.append('x', x.el, 'y', y.el); rec.get=()=>({after, act:t, x:x.val(), y:y.val()}); }
  }
  sel.onchange=render; render();
  const rm=document.createElement('button'); rm.type='button'; rm.className='del'; rm.textContent='✕'; rm.title='discard';
  rm.onclick=()=>{ row.remove(); adds=adds.filter(a=>a!==rec); };
  const lead=document.createElement('span'); lead.className='lbl'; lead.innerHTML='<span class="num">+</span>';
  row.append(lead, sel, fields, rm);
  rec.node=row; adds.push(rec);
  return row;
}
async function load(){
  const r = await (await fetch('/api/route')).json();
  document.getElementById('name').textContent = r.name + (r.app? (' · '+r.app):'');
  document.getElementById('src').textContent = r.source;
  const box = document.getElementById('steps');
  box.innerHTML=''; steps=[]; adds=[]; let n=0;
  for(const s of r.steps){
    const rec = {line: s.line, kind: s.kind, point: s.point, del:false, dragOn:false};
    const row = document.createElement('div');
    if(s.kind==='action'){
      n++;
      row.className='action';
      let labelText=s.label; if(s.act==='key') labelText='Press'; if(s.act==='type') labelText='Type';
      const label=document.createElement('span'); label.className='lbl';
      label.innerHTML='<span class="num">'+n+'.</span><span>'+esc(labelText)+'</span>';
      row.appendChild(label);
      if(s.act==='key' || s.act==='type'){ // editable keypress / typed text
        rec.txt=txtIn(s.text); row.appendChild(rec.txt.el);
      } else if(s.point){ // a click/move at a known point → editable coordinates
        rec.xi=coord(s.x); rec.yi=coord(s.y);
        const xy=document.createElement('span'); xy.className='xy';
        xy.append('x', rec.xi, 'y', rec.yi); row.appendChild(xy);
        if(s.canDrag){ // convert to a press-drag-release
          rec.txi=coord(s.x+80); rec.tyi=coord(s.y);
          const to=document.createElement('span'); to.className='to'; to.style.display='none';
          to.append('→ to', rec.txi, rec.tyi);
          const btn=document.createElement('button'); btn.type='button'; btn.className='mini'; btn.textContent='drag';
          btn.title='make this a click-and-drag';
          btn.onclick=()=>{ rec.dragOn=!rec.dragOn; to.style.display=rec.dragOn?'inline-flex':'none'; btn.classList.toggle('on', rec.dragOn); };
          const wrap=document.createElement('span'); wrap.className='dragbox'; wrap.append(btn, to);
          row.appendChild(wrap);
        }
      }
      row.append(plusBtn(s.line,row), delBtn(rec,row));
    } else {
      row.className='wait';
      rec.ms=document.createElement('input'); rec.ms.type='number'; rec.ms.min='0'; rec.ms.step='50'; rec.ms.value=s.ms;
      row.append('⏱ wait', rec.ms, 'ms', plusBtn(s.line,row), delBtn(rec,row));
    }
    steps.push(rec); box.appendChild(row);
  }
}
async function save(){
  const waits={}, points={}, deletes=[], drags={}, texts={};
  for(const s of steps){
    if(s.del){ deletes.push(s.line); continue; }
    if(s.kind==='wait'){ waits[s.line]=Math.max(0, parseInt(s.ms.value||'0',10)); continue; }
    if(s.txt){ texts[s.line]=s.txt.val(); continue; }
    if(s.point){
      const x=parseInt(s.xi.value||'0',10), y=parseInt(s.yi.value||'0',10);
      if(s.dragOn && s.txi){ drags[s.line]=[x, y, parseInt(s.txi.value||'0',10), parseInt(s.tyi.value||'0',10)]; }
      else { points[s.point]=[x, y]; }
    }
  }
  const addList = adds.map(a=>a.get());
  const res = await (await fetch('/api/save',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({waits, points, deletes, drags, texts, adds:addList})})).json();
  document.getElementById('saved').textContent = res.ok ? 'saved' : 'save failed';
  setTimeout(()=>document.getElementById('saved').textContent='', 2500);
  load();
}
function esc(s){ return s.replace(/[&<>]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;'}[c])); }
load();
</script></body></html>`
