// web-ui is a reference Marco UI plugin: a local control panel that drives the
// headless `marco` engine. Like the resolver and bridge, the UI lives OUTSIDE
// marco — a separate program (its own module, stdlib only) that shells out to
// the marco CLI:
//
//	GET  /api/routes  → `marco plays --json`       (every Play, with slug + scope)
//	GET  /api/status  → `marco active` + running   (foreground app + in-flight Plays)
//	POST /api/do      → `marco do --source=web …`  (run a Play, by identity or by phrase)
//
// marco core stays headless and zero-dependency.
//
// # Status: DEVELOPER SURFACE, not the product (Phase 3, 2026-08-20)
//
// Nothing launches this. `setup.ps1 -WebUI` still builds it because the Director panels it
// carries — Knows, Correct, Show-me, Sight, Answer — are real work that the control centre has
// not yet harvested. It is kept for that, and it is not a front door: it drives the DEVELOPER
// `director` CLI directly rather than the product's own service client, so it can and does show
// things the normal product does not.
//
// Do not treat it as a template for a new surface. `cmd/marco/learnui.go` is the model: it holds
// no state and renders what the coordinator returns.
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
	// POST /api/do — one invocation, and it says WHICH KIND it is.
	//
	// This surface has two ways in and they are not the same thing, which is the distinction the
	// one intake was built on (internal/invoke). Pressing **run** beside a listed Play is an
	// EXPLICIT IDENTITY: this page was handed a slug by `marco plays --json` and must hand that
	// same slug back, because a display name is derived from a slug and turning it back into words
	// for something else to guess at can land on a different Play entirely. Typing or speaking into
	// the box is a PHRASE, and a phrase Marco has no exact answer for belongs to Director.
	//
	// Until Phase 3 this handler sent `marco do <display name>` for both, with no --source and no
	// --play — the last legacy-shaped invocation in the repository, and the reason a Run button
	// here could reach Director while the same button in the control centre could not.
	http.HandleFunc("/api/do", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"` // what to show the person while it runs
			Slug string `json:"slug"` // set by the run button: the identity this page already holds
			App  string `json:"app"`  // the scope that slug lives in, if any
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "unreadable request", 400)
			return
		}
		label := strings.TrimSpace(req.Name)
		args := []string{"do", "--source=web"}
		switch {
		case strings.TrimSpace(req.Slug) != "":
			args = append(args, "--play="+strings.TrimSpace(req.Slug))
			if a := strings.TrimSpace(req.App); a != "" {
				args = append(args, "--app="+a)
			}
			if label == "" {
				label = strings.TrimSpace(req.Slug)
			}
		case label != "":
			args = append(args, label)
		default:
			http.Error(w, "missing name", 400)
			return
		}
		mark(label, 1)
		out, err := exec.Command(marcoBin(), args...).CombinedOutput()
		mark(label, -1)
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

// The old single-page UI that used to be compiled in beside this one was deleted in Phase 3.
//
// It was a complete second front end — its own route list, its own run button, its own voice
// handler — held in a `const page` that nothing served: `/` has rendered `accountPage` (page.go)
// since the Director panels landed. Two separate audits read its `marco do "<display name>"` as
// the last legacy-shaped invocation in the repository and recommended fixing it. It could not be
// reached from a browser at all.
//
// That is the more expensive kind of dead code: not a function nobody calls, but a whole surface
// that reads as the product and is not it. Deleting it is what makes the remaining answer to
// "how does this page run a play" a single one.
