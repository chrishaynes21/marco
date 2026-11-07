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
// A line naming a ROUTINE is kept as the routine's name, never with its values: "cloud pull stone"
// comes back as "cloud pull", whose reading then asks for the value afresh (‹resource›), so a
// recalled run never carries what somebody said last time. A repeat comes back with its count
// ("repeat open social 5 times"), which is part of the request and not a value. Any other typed line
// (the widget's own words, a request for Director) is kept as typed; a spoken one is kept only when
// it named a routine, so what the microphone heard in passing is not walked through. Kept in this
// process only, never written anywhere and never logged.

import (
	"strconv"
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

// recallOf is what a line is kept as, from the engine's reading of it: a routine's name (and a
// repeat's count) and nothing it was given, or the line as typed when it names no routine.
//
// Keeping a routine's values must fail TestARecalledRoutineAsksForItsValueAgain.
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
	if r.Kind == "skill" && strings.TrimSpace(r.Name) != "" {
		return r.Name // "Search the web", whose reading asks for the query again
	}
	if r.Kind != "routine" || strings.TrimSpace(r.Name) == "" {
		return line
	}
	if r.Times > 0 {
		return "repeat " + r.Name + " " + strconv.Itoa(r.Times) + " times"
	}
	return r.Name
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
	h.input = " " + h.recall[h.recallAt]
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
		h.input = " " + h.recall[h.recallAt]
		return
	}
	h.input, h.proposal = h.recallDraft, h.recallDraftProposal
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
