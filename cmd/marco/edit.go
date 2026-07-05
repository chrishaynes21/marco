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
var clickPointRE = regexp.MustCompile(`^do OS's (?:Click|Move) with (p\w+)\.`)

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
	Ms      int    `json:"ms,omitempty"`    // wait
	Point   string `json:"point,omitempty"` // action: the Point name it clicks (empty otherwise)
	X       int    `json:"x"`               // action: that point's coordinates (when Point != "")
	Y       int    `json:"y"`
	CanDrag bool   `json:"canDrag"` // a click/move at a point can be converted to a drag
	Line    int    `json:"line"`    // source line index (keys delete / wait / drag ops)
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
				if pv, ok := pts[cm[1]]; ok {
					s.Point, s.X, s.Y, s.CanDrag = cm[1], pv.X, pv.Y, true
				}
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
	Deletes []int             `json:"deletes"` // line indexes to remove (the step's line)
	Drags   map[string][4]int `json:"drags"`   // line → [fromX, fromY, toX, toY] (click → drag)
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

// dragNumRE finds existing `the dragN is a Drag` locals so a new conversion gets a fresh number.
var dragNumRE = regexp.MustCompile(`the drag(\d+) is a Drag`)

// rebuild produces the new source: drop deleted lines, rewrite edited Sleep waits, convert a
// click line to a Drag (a decl + call using the given from/to coords), then patch surviving
// point coordinates by name. Line-based so a delete doesn't disturb the other edits.
func (e *editor) rebuild(req saveReq) string {
	lines := strings.Split(e.src, "\n")
	del := make(map[int]bool, len(req.Deletes))
	for _, l := range req.Deletes {
		del[l] = true
	}
	dn := 0
	for _, m := range dragNumRE.FindAllStringSubmatch(e.src, -1) {
		if n, _ := strconv.Atoi(m[1]); n > dn {
			dn = n
		}
	}
	out := make([]string, 0, len(lines)+len(req.Drags))
	for i, line := range lines {
		if del[i] {
			continue
		}
		if d, ok := req.Drags[strconv.Itoa(i)]; ok {
			indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
			dn++
			out = append(out,
				fmt.Sprintf(`%sthe drag%d is a Drag with FromX %d, FromY %d, ToX %d, ToY %d, Button "left".`, indent, dn, d[0], d[1], d[2], d[3]),
				fmt.Sprintf("%sdo OS's Drag with drag%d.", indent, dn))
			continue
		}
		if m := sleepRE.FindStringSubmatch(line); m != nil {
			if ms, ok := req.Waits[strconv.Itoa(i)]; ok {
				line = m[1] + strconv.Itoa(max(ms, 0)) + m[3]
			}
		}
		out = append(out, line)
	}
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
 <p class="hint">Edit the wait between steps (ms) and click coordinates (x, y); <b>✕</b> deletes a
   step; <b>drag</b> turns a click into a click-and-drag (set the "to" point). Save writes it
   back. With CV off, these coordinates + timings are the whole route.</p>
 <div id="steps"></div>
 <div class="bar"><button onclick="save()">Save</button><span id="saved2"></span></div>
 <details><summary>Full route source</summary><pre id="src"></pre></details>
</main>
<script>
let steps = [];  // per-row edit state
function coord(v){ const i=document.createElement('input'); i.type='number'; i.className='coord'; i.value=v; return i; }
function delBtn(rec, row){
  const b=document.createElement('button'); b.type='button'; b.className='del'; b.title='delete this step'; b.textContent='✕';
  b.onclick=()=>{ rec.del=!rec.del; row.classList.toggle('deleted', rec.del); };
  return b;
}
async function load(){
  const r = await (await fetch('/api/route')).json();
  document.getElementById('name').textContent = r.name + (r.app? (' · '+r.app):'');
  document.getElementById('src').textContent = r.source;
  const box = document.getElementById('steps');
  box.innerHTML=''; steps=[]; let n=0;
  for(const s of r.steps){
    const rec = {line: s.line, kind: s.kind, point: s.point, del:false, dragOn:false};
    const row = document.createElement('div');
    if(s.kind==='action'){
      n++;
      row.className='action';
      const label=document.createElement('span'); label.className='lbl';
      label.innerHTML='<span class="num">'+n+'.</span><span>'+esc(s.label)+'</span>';
      row.appendChild(label);
      if(s.point){ // a click/move at a known point → editable coordinates
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
      row.appendChild(delBtn(rec,row));
    } else {
      row.className='wait';
      rec.ms=document.createElement('input'); rec.ms.type='number'; rec.ms.min='0'; rec.ms.step='50'; rec.ms.value=s.ms;
      row.append('⏱ wait', rec.ms, 'ms', delBtn(rec,row));
    }
    steps.push(rec); box.appendChild(row);
  }
}
async function save(){
  const waits={}, points={}, deletes=[], drags={};
  for(const s of steps){
    if(s.del){ deletes.push(s.line); continue; }
    if(s.kind==='wait'){ waits[s.line]=Math.max(0, parseInt(s.ms.value||'0',10)); continue; }
    if(s.point){
      const x=parseInt(s.xi.value||'0',10), y=parseInt(s.yi.value||'0',10);
      if(s.dragOn && s.txi){ drags[s.line]=[x, y, parseInt(s.txi.value||'0',10), parseInt(s.tyi.value||'0',10)]; }
      else { points[s.point]=[x, y]; }
    }
  }
  const res = await (await fetch('/api/save',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({waits, points, deletes, drags})})).json();
  document.getElementById('saved').textContent = res.ok ? 'saved' : 'save failed';
  setTimeout(()=>document.getElementById('saved').textContent='', 2500);
  load();
}
function esc(s){ return s.replace(/[&<>]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;'}[c])); }
load();
</script></body></html>`
