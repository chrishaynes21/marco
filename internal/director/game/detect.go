package game

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Recognising what is in front.
//
//	Detection should combine process, executable, window title and semantic
//	observations. Not just executable names.
//
// Four signals, and the reason for four is that each is wrong on its own. An executable
// name is renamed by mod loaders and launchers. A window title is set by the game and also
// by every streaming overlay that decorates one. A process name is shared by a hundred
// Unity games. And observations are the strongest signal of all — a window containing a
// control called "Palbox" is not ambiguous — but they arrive only once the game is running
// and drawing something a provider understood.
//
// So a pack scores what it recognises, says what it recognised, and the framework compares
// the numbers. What this file provides is the arithmetic and the vocabulary, so twelve
// packs do not each invent their own idea of what 0.7 means.

// Signal weights.
//
// Chosen so that no SINGLE weak signal reaches the threshold and any two do, and so that an
// interface match alone is enough. A game recognised only by its process name is a guess;
// one recognised by its process name and its window title is not; and one whose interface
// contains a control this pack knows by name is not a guess at all.
const (
	// SignalProcess is the process or executable name matching.
	//
	// Sized with SignalTitle so the two COMBINE past the threshold and neither reaches it
	// alone: 0.4 then 0.4 + 0.6×0.35 = 0.61. That is the property the comment above
	// claims, and it is arithmetic rather than intention — a test asserts both halves.
	SignalProcess = 0.4
	// SignalTitle is the window title matching.
	SignalTitle = 0.35
	// SignalInterface is an observation only this application produces.
	SignalInterface = 0.6
	// SignalEntity is an entity a pack's own observer contributed, which is the
	// strongest thing there is: the pack recognised the interface well enough to model
	// it.
	SignalEntity = 0.7

	// MatchThreshold is the score at which a pack should report Matched.
	//
	// Above any one weak signal and at or below any combination of two, so "the process
	// name looks right" is never a detection on its own — which is the failure mode that
	// gives a user's Minecraft another game's procedures.
	MatchThreshold = 0.6
)

// Matcher accumulates detection signals and produces a Detection.
//
// A small builder rather than a function with eight parameters, because what a pack wants
// to write is a list of things it recognised, and what a reader wants to see is that same
// list in the evidence.
type Matcher struct {
	score    float64
	evidence []string
	mode     string
	version  string
}

// NewMatcher returns an empty matcher.
func NewMatcher() *Matcher { return &Matcher{} }

// Signal records one recognised thing.
//
// Weights COMBINE probabilistically rather than adding, so three weak signals approach
// certainty without exceeding it and no accumulation of guesses reaches 1.0. A pack that
// listed the same signal twice gains almost nothing, which is the correct treatment of
// saying the same thing twice.
func (m *Matcher) Signal(weight float64, evidence string) *Matcher {
	if weight <= 0 {
		return m
	}
	if weight > 1 {
		weight = 1
	}
	m.score += (1 - m.score) * weight
	if evidence != "" {
		m.evidence = append(m.evidence, evidence)
	}
	return m
}

// Mode records which part of the application is in front.
func (m *Matcher) Mode(mode string) *Matcher {
	if mode != "" {
		m.mode = mode
	}
	return m
}

// Version records the application version, when the pack can tell.
func (m *Matcher) Version(v string) *Matcher {
	if v != "" {
		m.version = v
	}
	return m
}

// Result produces the detection.
//
// A pack below the threshold reports Matched=false WITH its evidence, so "why did it not
// detect my game?" is answerable: the evidence says what it did recognise, and the score
// says it was not enough.
func (m *Matcher) Result() Detection {
	return Detection{
		Matched:     m.score >= MatchThreshold,
		Confidence:  m.score,
		Mode:        m.mode,
		GameVersion: m.version,
		Evidence:    append([]string{}, m.evidence...),
	}
}

// ── the signals themselves ────────────────────────────────────────────────────

// MatchProcess records a process or executable name match, case-insensitively.
//
// Compares the BASE NAME of the executable as well as the process name, because the two
// are the same string on Windows and different on a launcher that reports a full path.
func (m *Matcher) MatchProcess(p Process, names ...string) *Matcher {
	candidates := []string{p.Name, filepath.Base(p.Executable)}
	for _, want := range names {
		for _, got := range candidates {
			if got == "" || got == "." {
				continue
			}
			if strings.EqualFold(got, want) {
				return m.Signal(SignalProcess,
					fmt.Sprintf("the process is %s", got))
			}
		}
	}
	return m
}

// MatchTitle records a window-title match.
//
// CONTAINS rather than equals: a game's window title carries a version, a world name, or a
// streaming overlay's decoration, and a pack that demanded the exact string would detect
// nothing on a real desktop.
func (m *Matcher) MatchTitle(w directorapi.WorldState, fragments ...string) *Matcher {
	title := activeTitle(w)
	if title == "" {
		return m
	}
	lower := strings.ToLower(title)
	for _, f := range fragments {
		if f == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(f)) {
			return m.Signal(SignalTitle,
				fmt.Sprintf("the window is titled %q", title))
		}
	}
	return m
}

// MatchLabels records that the interface contains controls this pack knows.
//
// ALL of them, not any: a single label is a coincidence, and the point of this signal is
// that a particular COMBINATION of controls appears in one application and nowhere else.
func (m *Matcher) MatchLabels(w directorapi.WorldState, labels ...string) *Matcher {
	if len(labels) == 0 {
		return m
	}
	for _, want := range labels {
		if !hasLabel(w, want) {
			return m
		}
	}
	return m.Signal(SignalInterface,
		fmt.Sprintf("the interface contains %s", quoteAll(labels)))
}

// MatchEntities records that this pack's own observers modelled what is on screen.
func (m *Matcher) MatchEntities(w directorapi.WorldState, source string) *Matcher {
	n := 0
	for _, el := range w.Elements {
		if el.Entity.Known() && strings.EqualFold(el.Entity.Source, source) {
			n++
		}
	}
	if n == 0 {
		return m
	}
	return m.Signal(SignalEntity,
		fmt.Sprintf("%s recognised %d element(s) of this interface", source, n))
}

// ── helpers ───────────────────────────────────────────────────────────────────

func activeTitle(w directorapi.WorldState) string {
	if w.ActiveWindow == nil {
		if len(w.Windows) > 0 {
			return w.Windows[0].Title
		}
		return ""
	}
	for _, win := range w.Windows {
		if win.ID == *w.ActiveWindow {
			return win.Title
		}
	}
	return ""
}

func hasLabel(w directorapi.WorldState, want string) bool {
	for _, el := range w.Elements {
		if strings.EqualFold(el.Label, want) {
			return true
		}
	}
	return false
}

func quoteAll(labels []string) string {
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		out = append(out, fmt.Sprintf("%q", l))
	}
	return strings.Join(out, " and ")
}
