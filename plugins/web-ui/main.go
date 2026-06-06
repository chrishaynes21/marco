// web-ui is a reference Marco UI plugin: a tiny local control panel that drives
// the headless `marco` engine. Like the resolver and bridge, the UI lives
// OUTSIDE marco — it's a separate program (its own module, stdlib only) that
// shells out to the marco CLI:
//
//	GET  /api/routes  → `marco routes --json`
//	GET  /api/active  → `marco active`
//	POST /api/do      → `marco do "<name>"`
//
// So marco core stays headless and zero-dependency; any front-end (this web UI,
// the AutoHotkey overlay, a tray app) speaks the same CLI seam.
//
//	build: go -C plugins/web-ui build -o web-ui .
//	use:   set MARCO_BIN to the marco binary (or have `marco` on PATH), then run
//	       web-ui and open http://localhost:8765
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func marcoBin() string {
	if b := os.Getenv("MARCO_BIN"); b != "" {
		return b
	}
	return "marco"
}

func main() {
	addr := os.Getenv("MARCO_UI_ADDR")
	if addr == "" {
		addr = "localhost:8765"
	}
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, page)
	})
	http.HandleFunc("/api/routes", func(w http.ResponseWriter, _ *http.Request) {
		out, err := exec.Command(marcoBin(), "routes", "--json").Output()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if len(strings.TrimSpace(string(out))) == 0 {
			out = []byte("[]")
		}
		w.Write(out)
	})
	http.HandleFunc("/api/active", func(w http.ResponseWriter, _ *http.Request) {
		out, _ := exec.Command(marcoBin(), "active").Output()
		writeJSON(w, map[string]string{"app": strings.TrimSpace(string(out))})
	})
	http.HandleFunc("/api/do", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Name string `json:"name"` }
		if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Name) == "" {
			http.Error(w, "missing name", 400)
			return
		}
		out, err := exec.Command(marcoBin(), "do", req.Name).CombinedOutput()
		writeJSON(w, map[string]any{"ok": err == nil, "output": string(out)})
	})

	fmt.Printf("marco web UI → http://%s  (driving %q)\n", addr, marcoBin())
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

const page = `<!doctype html><html><head><meta charset="utf-8"><title>marco</title>
<style>
 body{font:15px system-ui;margin:0;background:#1e1e22;color:#e6e6e6}
 header{padding:14px 18px;background:#26262c;border-bottom:1px solid #34343c}
 #app{color:#9ad}
 main{padding:18px;max-width:680px}
 .row{display:flex;gap:8px;margin-bottom:14px}
 input{flex:1;padding:9px 12px;background:#26262c;border:1px solid #3a3a44;color:#e6e6e6;border-radius:6px}
 button{padding:9px 14px;background:#3a6ea5;color:#fff;border:0;border-radius:6px;cursor:pointer}
 button:hover{background:#4a7eb5}
 .route{display:flex;justify-content:space-between;align-items:center;padding:8px 12px;border:1px solid #34343c;border-radius:6px;margin-bottom:6px}
 .scope{color:#888;font-size:13px}
 pre{background:#141417;padding:12px;border-radius:6px;white-space:pre-wrap;min-height:40px}
</style></head><body>
<header>marco — <span id="app">…</span></header>
<main>
 <div class="row"><input id="cmd" placeholder='say a command, e.g. "open chest"' autofocus>
   <button onclick="run(cmd.value)">Run</button></div>
 <div id="routes"></div>
 <pre id="out"></pre>
</main>
<script>
async function refresh(){
  const rs = await (await fetch('/api/routes')).json();
  document.getElementById('routes').innerHTML = rs.map(r =>
    '<div class="route"><span>'+r.name+' <span class="scope">'+(r.app?('· '+r.app):'· everywhere')+
    '</span></span><button onclick="run(\''+r.name.replace(/'/g,"\\'")+'\')">run</button></div>').join('')
    || '<div class="scope">No routes yet — teach one: marco teach "name"</div>';
  const a = await (await fetch('/api/active')).json();
  document.getElementById('app').textContent = a.app || '(no foreground app)';
}
async function run(name){
  if(!name) return;
  document.getElementById('out').textContent = 'running "'+name+'"…';
  const res = await (await fetch('/api/do',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({name})})).json();
  document.getElementById('out').textContent = res.output || (res.ok?'(done)':'(failed)');
  refresh();
}
cmd.addEventListener('keydown', e => { if(e.key==='Enter') run(cmd.value); });
refresh(); setInterval(refresh, 3000);
</script></body></html>`
