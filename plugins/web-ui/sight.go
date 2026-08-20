package main

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
)

// "Show Sight" — what Marco can see, in the browser.
//
// # What it answers
//
// Four things, and no more: which application is being watched, what Marco can perceive WITH, what
// it takes the current screen to be, and what it is currently referring to. Not an accessibility
// tree. A person asking "what does Marco see?" is asking whether it has understood where they are,
// and a tree dump answers a question nobody has while looking like comprehension.
//
// # It is a read, and that is the whole promise
//
// `director sight` starts no session, answers no proposal, writes no memory and recognises nothing
// as a result of being asked. Turning the panel on must not change what Marco believes, or the
// inspection is measuring itself — so the toggle lives entirely in the browser and the handler is
// a plain read of state the service already holds.
//
// # Why the sources are named
//
// "Perceiving: yes" would be true of a Director reading an accessibility tree and of one reading
// pixels, and those are different claims about how far to trust a highlight. Each source arrives
// with its real state, and an off one arrives with the reason. Nothing here can turn one on.

// handleSight reports what Marco is currently seeing.
func handleSight(w http.ResponseWriter, r *http.Request) {
	args := []string{"sight", "--json"}
	if r.URL.Query().Get("view") == "debug" {
		args = append(args, "--deep")
	}
	out, err := exec.Command(directorBin(), args...).Output()
	if err != nil {
		writeJSON(w, map[string]any{"reach": "absent"})
		return
	}
	var v struct {
		Application string `json:"application"`
		Say         string `json:"say"`
		About       string `json:"about"`
		Place       string `json:"place"`
		Source      string `json:"perception_source"`
		Sources     []struct {
			Name   string `json:"name"`
			On     bool   `json:"on"`
			Reason string `json:"reason"`
		} `json:"perception_sources"`
		Question       string           `json:"question"`
		Interpretation string           `json:"interpretation"`
		Boxes          []map[string]any `json:"boxes"`
	}
	if json.Unmarshal(out, &v) != nil {
		writeJSON(w, map[string]any{"reach": "absent"})
		return
	}
	sources := make([]map[string]any, 0, len(v.Sources))
	for _, s := range v.Sources {
		sources = append(sources,
			map[string]any{"name": s.Name, "on": s.On, "reason": s.Reason})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{
		"reach": "present", "watching": v.Application, "say": v.Say, "about": v.About,
		"place": v.Place, "grounding": v.Source, "sources": sources,
		"question": v.Question, "interpretation": v.Interpretation,
		// A COUNT, never the rectangles. A page that held desktop coordinates would be a
		// second place able to decide where Marco is pointing, and the one with the least
		// evidence — see the note in internal/director/observe/referent.go.
		"locatable": len(v.Boxes) > 0,
	})
}

// handlePoint points at what Marco means.
//
// # A question is pointed at BY NAME
//
// With a question id, this resolves that question's own subject and refuses when it cannot. Without
// one it resolves whatever Marco is currently referring to, which is the honest answer for a Sight
// panel and the wrong one under a question — the general path deliberately skips a screen-shaped
// subject so it does not consume the choice, so a button that used it beneath a screen question
// would highlight some other group under that sentence. Found live on Windows Settings.
//
// The same command `director show-me` runs, with `--watch 0` so that inspecting a question does not
// quietly begin three minutes of observation. When Marco cannot point, the sentence it returns says
// so — the page never converts "no boxes" into a guess.
func handlePoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Question string `json:"question"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	args := []string{"show-me", "--watch", "0", "--json"}
	if q := strings.TrimSpace(req.Question); q != "" {
		args = append(args, "--question", q)
	}
	out, err := exec.Command(directorBin(), args...).Output()
	if err != nil {
		writeJSON(w, map[string]any{"said": "I couldn't work out where that is right now."})
		return
	}
	var v struct {
		Say   string           `json:"say"`
		Boxes []map[string]any `json:"boxes"`
	}
	if json.Unmarshal(out, &v) != nil {
		writeJSON(w, map[string]any{"said": "I couldn't work out where that is right now."})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{"said": v.Say, "shown": len(v.Boxes)})
}
