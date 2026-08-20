// web-ui is a reference Marco UI plugin: a local control panel that drives the
// headless `marco` engine. Like the resolver and bridge, the UI lives OUTSIDE
// marco — a separate program (its own module, stdlib only) that shells out to
// the marco CLI:
//
//	GET  /api/routes  → `marco routes --json`     (all routes + scope)
//	GET  /api/status  → `marco active` + running   (foreground app + in-flight routes)
//	POST /api/do      → `marco do "<name>"`         (run a route)
//
// The page lists every route, highlights the one currently running, and takes
// commands by typing or by voice (browser Web Speech API). marco core stays
// headless and zero-dependency.
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
	"sort"
	"strings"
	"sync"
)

func marcoBin() string {
	if b := os.Getenv("MARCO_BIN"); b != "" {
		return b
	}
	return "marco"
}

// running tracks routes currently executing, by name (count handles concurrent
// runs of the same route).
var (
	mu      sync.Mutex
	running = map[string]int{}
)

func mark(name string, delta int) {
	mu.Lock()
	defer mu.Unlock()
	running[name] += delta
	if running[name] <= 0 {
		delete(running, name)
	}
}

func runningList() []string {
	mu.Lock()
	defer mu.Unlock()
	out := make([]string, 0, len(running))
	for k := range running {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func main() {
	addr := os.Getenv("MARCO_UI_ADDR")
	if addr == "" {
		addr = "localhost:8765"
	}
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// The page is compiled into the binary and changes when the binary does, so a
		// browser holding an old copy is showing a Director that no longer exists. That
		// cost an entire round of "the buttons are not there" once already.
		w.Header().Set("Cache-Control", "no-store, must-revalidate")
		fmt.Fprint(w, accountPage)
	})
	http.HandleFunc("/api/playbill", handlePlaybill)
	http.HandleFunc("/api/answer", handleAnswer)
	http.HandleFunc("/api/name", handleName)
	http.HandleFunc("/api/stop", handleStop)
	http.HandleFunc("/api/knows", handleKnows)
	http.HandleFunc("/api/correct", handleCorrect)
	// "Show me what this refers to". A read: it points, and changes nothing.
	http.HandleFunc("/api/showme", handleShowMe)
	// Show Sight. A READ: it starts nothing and answers nothing, which is what makes it safe
	// to leave a panel polling it while a person is being asked a question.
	http.HandleFunc("/api/sight", handleSight)
	http.HandleFunc("/api/point", handlePoint)
	http.HandleFunc("/api/routes", func(w http.ResponseWriter, _ *http.Request) {
		out, err := exec.Command(marcoBin(), "routes", "--json").Output()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if len(strings.TrimSpace(string(out))) == 0 {
			out = []byte("[]")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	})
	http.HandleFunc("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		app, _ := exec.Command(marcoBin(), "active").Output()
		writeJSON(w, map[string]any{
			"app":     strings.TrimSpace(string(app)),
			"running": runningList(),
		})
	})
	http.HandleFunc("/api/do", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Name) == "" {
			http.Error(w, "missing name", 400)
			return
		}
		mark(req.Name, 1)
		out, err := exec.Command(marcoBin(), "do", req.Name).CombinedOutput()
		mark(req.Name, -1)
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
 header{padding:14px 18px;background:#26262c;border-bottom:1px solid #34343c;display:flex;justify-content:space-between;align-items:center}
 #app{color:#9ad}
 #busy{color:#e0b050;font-size:13px}
 main{padding:18px;max-width:680px}
 .row{display:flex;gap:8px;margin-bottom:14px}
 input{flex:1;padding:9px 12px;background:#26262c;border:1px solid #3a3a44;color:#e6e6e6;border-radius:6px}
 button{padding:9px 14px;background:#3a6ea5;color:#fff;border:0;border-radius:6px;cursor:pointer}
 button:hover{background:#4a7eb5}
 #mic.live{background:#a53a3a}
 .route{display:flex;justify-content:space-between;align-items:center;padding:8px 12px;border:1px solid #34343c;border-radius:6px;margin-bottom:6px}
 .route.running{border-color:#e0b050;background:#2c2922}
 .scope{color:#888;font-size:13px}
 .tag{color:#e0b050;font-size:12px;margin-left:8px}
 pre{background:#141417;padding:12px;border-radius:6px;white-space:pre-wrap;min-height:40px}
</style></head><body>
<header><span>marco — <span id="app">…</span></span><span id="busy"></span></header>
<main>
 <div class="row">
   <input id="cmd" placeholder='say a command, e.g. "open chest"' autofocus>
   <button id="mic" title="speak a command">🎤</button>
   <button onclick="run(cmd.value)">Run</button>
 </div>
 <div id="routes"></div>
 <pre id="out"></pre>
</main>
<script>
let runningNow = [];
async function refresh(){
  const rs = await (await fetch('/api/routes')).json();
  const st = await (await fetch('/api/status')).json();
  runningNow = st.running || [];
  document.getElementById('app').textContent = st.app || '(no foreground app)';
  document.getElementById('busy').textContent = runningNow.length ? ('running: ' + runningNow.join(', ')) : '';
  document.getElementById('routes').innerHTML = rs.map(r => {
    const on = runningNow.includes(r.name);
    return '<div class="route'+(on?' running':'')+'"><span>'+r.name+
      ' <span class="scope">'+(r.app?('· '+r.app):'· everywhere')+'</span>'+
      (on?'<span class="tag">▶ running</span>':'')+'</span>'+
      '<button onclick="run(\''+r.name.replace(/'/g,"\\'")+'\')">run</button></div>';
  }).join('') || '<div class="scope">No routes yet — learn one: marco learn "name"</div>';
}
async function run(name){
  if(!name) return;
  document.getElementById('out').textContent = 'running "'+name+'"…';
  refresh();
  const res = await (await fetch('/api/do',{method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({name})})).json();
  document.getElementById('out').textContent = res.output || (res.ok?'(done)':'(failed)');
  refresh();
}
cmd.addEventListener('keydown', e => { if(e.key==='Enter') run(cmd.value); });

// Voice: browser Web Speech API (Chrome/Edge). Hidden if unsupported.
const SR = window.SpeechRecognition || window.webkitSpeechRecognition;
const mic = document.getElementById('mic');
if(SR){
  const rec = new SR(); rec.lang='en-US'; rec.interimResults=false;
  mic.onclick = ()=>{ try{ rec.start(); mic.classList.add('live'); }catch(_){} };
  rec.onend = ()=> mic.classList.remove('live');
  rec.onresult = e => { const t=e.results[0][0].transcript; cmd.value=t; run(t); };
} else { mic.style.display='none'; }

refresh(); setInterval(refresh, 2000);
</script></body></html>`
