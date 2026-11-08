package main

// ↑ AND ↓ ON THE COMMAND LINE, LIKE A SHELL'S HISTORY.
//
// ↑ puts the most recent command back on the line, editable, and each further ↑ walks back; ↓
// walks forward, and past the newest puts back whatever was being typed before the first ↑ (and
// the proposal it was open on, if Director's values were on it). Enter on a recalled line is an
// ordinary Enter: the line is read, reviewed and run as if it had been typed.
//
// # What is kept
//
// A command is kept AS IT RAN, values and all: "cloud pull stone" comes back as "cloud pull stone"
// and "repeat open social five times" with its count, so ↑ then Enter runs the same thing again
// (the owner reversed the first rule, which kept a routine's name and asked for its value afresh).
// Focus app comes back as the program it went to ("focus ChatGPT"), never what was heard, and a
// search with its query. Any other typed line (the widget's own words, a request for Director) is
// kept as typed; a spoken one is kept only when
// it named a routine, so what the microphone heard in passing is not walked through. Kept in this
// process only, never written anywhere and never logged.

import (
	"strings"
)

// recallKeeps is how many commands ↑ can walk back through.
const recallKeeps = 100

// remember keeps one command for ↑. The same command twice running is kept once.
func (h *model) remember(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if n := len(h.recall); n > 0 && h.recall[n-1] == line {
		return
	}
	h.recall = append(h.recall, line)
	if len(h.recall) > recallKeeps {
		h.recall = h.recall[len(h.recall)-recallKeeps:]
	}
}

// rememberAsRoutine replaces a command kept as said with the routine it ran as, when the line
// had no reading before it ran (an engine too slow to answer): the values stay out either way.
func (h *model) rememberAsRoutine(said, routine string) {
	said, routine = strings.TrimSpace(said), strings.TrimSpace(routine)
	if routine == "" {
		return
	}
	h.mu.Lock()
	for i := len(h.recall) - 1; i >= 0; i-- {
		if h.recall[i] == said {
			h.recall[i] = routine
			break
		}
	}
	h.mu.Unlock()
}

// recallOf is what a line is kept as, from the engine's reading of it: the command AS IT RAN, values
// and all, so ↑ and Enter run it again (the owner: "same for all things that accept args, pressing
// up should recall the last command"). A routine is its line as typed, "cloud pull stone" or
// "repeat open social five times"; Focus app is the program it went to, never what was heard.
//
// Keeping a routine by its name only must fail TestARecalledRoutineComesBackWithItsValues.
func recallOf(line string, r lineReading) string {
	if r.Kind == "skill" && r.Slug == focusSkill {
		// FOCUS APP IS KEPT WITH THE PROGRAM IT BROUGHT FORWARD, "focus ChatGPT", so ↑ goes
		// back to it (the owner's ask): which window to bring forward is the whole request, not a
		// value from last time. The PROGRAM, never what was heard. Still being found, it is kept
		// by its name and replaced once the run says which program (focusRan).
		for _, p := range r.Parts {
			if p.Label == "program" && strings.TrimSpace(p.Value) != "" {
				return "focus " + p.Value
			}
		}
	}
	if r.Kind == "skill" && r.Slug == searchSkill && strings.TrimSpace(r.Values["query"]) != "" {
		// AND A SEARCH WITH ITS QUERY, "search the web for ark wiki", for the same reason.
		return "search the web for " + strings.TrimSpace(r.Values["query"])
	}
	if r.Kind == "skill" && strings.TrimSpace(r.Name) != "" {
		return r.Name // "Search the web" with nothing yet, whose reading asks for the query
	}
	return strings.TrimSpace(line)
}

// recallOlder is ↑: the command before the one on the line, or the newest when the walk begins.
//
// Starting the walk anywhere but the newest must fail TestUpAndDownWalkTheCommandsLikeAShell.
func (h *model) recallOlder() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.editing || len(h.recall) == 0 {
		return
	}
	switch {
	case !h.recalling:
		h.recallDraft, h.recallDraftProposal = h.input, h.proposal
		h.recalling, h.recallAt = true, len(h.recall)-1
	case h.recallAt > 0:
		h.recallAt--
	default:
		return // the oldest: ↑ stays on it
	}
	h.proposal = nil // a recalled line is not the routine Director's values were for
	h.input, h.cursorBack = " "+h.recall[h.recallAt], 0
}

// recallNewer is ↓: the command after the one on the line, and past the newest, the line as it was
// before ↑.
//
// Not putting the draft back must fail TestUpAndDownWalkTheCommandsLikeAShell.
func (h *model) recallNewer() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.editing || !h.recalling {
		return
	}
	if h.recallAt < len(h.recall)-1 {
		h.recallAt++
		h.input, h.cursorBack = " "+h.recall[h.recallAt], 0
		return
	}
	h.input, h.proposal, h.cursorBack = h.recallDraft, h.recallDraftProposal, 0
	h.endRecallLocked()
}

// endRecall forgets where the walk was: a line opened afresh, or Enter.
func (h *model) endRecall() { h.mu.Lock(); h.endRecallLocked(); h.mu.Unlock() }

func (h *model) endRecallLocked() {
	h.recalling, h.recallAt, h.recallDraft, h.recallDraftProposal = false, 0, "", nil
}

func (h *model) recalled() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.recall...)
}
