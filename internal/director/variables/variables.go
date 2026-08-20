// Package variables is the Director's semantic memory.
//
// The governing rules:
//
//	Variables remember MEANING, not pixels.
//	Every use is a fresh semantic resolution.
//	The remembered object is evidence, never authority.
//
// "Remember this button as Save" does not store where Save was. It stores what Save
// MEANT — the query that found it, the role it had, the application it was in — so
// that "click $save" a week later, on a different monitor, after the window has moved
// and every element id has been reissued, resolves the same button again.
//
// That is the whole difference between this and a macro recorder. A recorder stores
// the answer; this stores the question. The answer is only ever valid for the instant
// it was computed, which is why nothing here holds an ElementID, an hwnd, a RuntimeId
// or a coordinate — see Variable for the enforcement.
package variables

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Kind is what sort of semantic object a variable names.
//
// Four, closed. A general object hierarchy would let a variable mean anything, and a
// variable that can mean anything cannot be resolved by a rule — it would need a
// resolver per shape, invented as each one appeared.
type Kind string

const (
	// KindTarget is a control: a button, a field, a menu item.
	KindTarget Kind = "target"
	// KindWindow is an application window.
	KindWindow Kind = "window"
	// KindRegion is an area of the screen, named semantically.
	KindRegion Kind = "region"
	// KindText is captured text plus where it came from.
	KindText Kind = "text"
)

// Variable is one remembered semantic object.
//
// Note what is ABSENT, deliberately and permanently: there is no ElementID, no
// WindowID, no hwnd, no RuntimeId, no Rect, no Point. Those are all answers computed
// against a world that no longer exists by the time the variable is used. A variable
// that carried one would resolve to the wrong control the first time the application
// repainted, and — worse — would look right while doing it.
type Variable struct {
	Name string `json:"name"`
	Kind Kind   `json:"kind"`

	// Query is the semantic description that found the object, and is what finds it
	// again. The whole point of the type.
	Query *directorapi.ElementQuery `json:"query,omitempty"`

	// Application is the app it was captured in, so "the Save in Notepad" stays
	// distinguishable from "the Save in the browser". Semantic, not a process id:
	// the application is still the same application after a restart.
	Application string `json:"application,omitempty"`
	// WindowTitle is the window's title at capture. Advisory only — titles change as
	// documents are edited — so it narrows a search but never decides it alone.
	WindowTitle string `json:"window_title,omitempty"`

	// Label and Role are what the object was, kept separately from Query so an
	// explanation can quote them without unpacking a query.
	Label string                  `json:"label,omitempty"`
	Role  directorapi.ElementRole `json:"role,omitempty"`

	// Text is the captured content, for KindText only.
	Text string `json:"text,omitempty"`

	Provenance Provenance `json:"provenance"`
	History    History    `json:"history"`
}

// Provenance is where a variable came from.
//
// Every variable must justify itself exactly as a perception element does. A
// remembered target the Director cannot account for is one it should not act on.
type Provenance struct {
	// CapturedAt is when the user said "remember this".
	CapturedAt time.Time `json:"captured_at"`
	// Source names how the object was identified at capture: which observation
	// source, and by what evidence.
	Source string `json:"source,omitempty"`
	// Phrase is the user's own words, which is what an explanation should quote back.
	Phrase string `json:"phrase,omitempty"`
	// Explanation is why the resolver chose this object when it was captured.
	Explanation string `json:"explanation,omitempty"`
	// Confidence is how sure the capture was, 0..1.
	Confidence float64 `json:"confidence"`
}

// History is what has happened to a variable since.
//
// Kept because a variable that resolved fine yesterday and fails today is a different
// problem from one that never resolved, and only a record can tell them apart.
type History struct {
	// Uses is how many times it has been referenced.
	Uses int `json:"uses"`
	// LastResolvedAt is when it last found its object.
	LastResolvedAt *time.Time `json:"last_resolved_at,omitempty"`
	// LastResolvedLabel is what it found then — evidence that the meaning still
	// tracks the same thing, and the first clue when it stops.
	LastResolvedLabel string `json:"last_resolved_label,omitempty"`
	// LastFailure is why the most recent lookup failed, empty if the last one worked.
	LastFailure string `json:"last_failure,omitempty"`
	// LastFailedAt is when that was.
	LastFailedAt *time.Time `json:"last_failed_at,omitempty"`
}

// Resolvable reports whether this variable carries enough to search with.
func (v Variable) Resolvable() bool {
	switch v.Kind {
	case KindText:
		// Text is its own value; there is nothing to look up.
		return true
	case KindWindow:
		return v.Application != "" || v.WindowTitle != ""
	}
	return v.Query != nil && v.Query.Constrained()
}

// Describe is a one-line human form.
func (v Variable) Describe() string {
	switch v.Kind {
	case KindText:
		return fmt.Sprintf("%s = text (%d characters)", v.Name, len(v.Text))
	case KindWindow:
		return fmt.Sprintf("%s = the %s window", v.Name, firstNonEmpty(v.Application, v.WindowTitle))
	}
	what := firstNonEmpty(v.Label, string(v.Role), "a control")
	if v.Application != "" {
		return fmt.Sprintf("%s = %q in %s", v.Name, what, v.Application)
	}
	return fmt.Sprintf("%s = %q", v.Name, what)
}

// namePattern is what a variable may be called.
//
// Letters, digits and underscore, not starting with a digit — the same shape every
// language uses, so a name never has to be quoted or escaped when it appears after a
// "$" in a phrase.
var namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// reserved are names a variable may not take.
//
// Each would make a phrase ambiguous between a variable reference and an ordinary
// word: "click $it" could mean the remembered thing or the previous step's target.
var reserved = map[string]bool{
	"it": true, "that": true, "this": true, "there": true,
	"here": true, "all": true, "none": true, "stop": true, "cancel": true,
}

// NormalizeName lower-cases a variable name and validates it.
//
// Case-INSENSITIVE, deliberately. These names are spoken as often as typed, and a user
// who says "remember this as Save" and later "click $save" means the same variable.
// Case-sensitivity would turn a speech-recognition capitalisation into a missing
// variable.
func NormalizeName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	name = strings.TrimPrefix(name, "$")
	if name == "" {
		return "", fmt.Errorf("a variable needs a name")
	}
	if !namePattern.MatchString(name) {
		return "", fmt.Errorf("%q is not a usable name: use letters, digits and underscores, "+
			"starting with a letter", raw)
	}
	lower := strings.ToLower(name)
	if reserved[lower] {
		return "", fmt.Errorf("%q is a word the Director already uses, so it cannot be a "+
			"variable name", raw)
	}
	return lower, nil
}

// Reference extracts a variable name from a phrase token like "$save".
func Reference(token string) (string, bool) {
	if !strings.HasPrefix(token, "$") {
		return "", false
	}
	name, err := NormalizeName(token)
	if err != nil {
		return "", false
	}
	return name, true
}

// ErrUnknown is a reference to a variable that was never stored.
//
// Named rather than generic, because the correct response is to say so and stop. A
// Director that guessed at an unknown variable would act on something the user never
// named.
type ErrUnknown struct{ Name string }

func (e *ErrUnknown) Error() string { return "Unknown variable: " + e.Name }

// ErrStale is a variable whose object is not in the current world.
//
// Distinct from unknown: the variable exists and means something, and the world has
// changed. That distinction is what tells a user to re-capture rather than re-type.
type ErrStale struct {
	Name   string
	Detail string
}

func (e *ErrStale) Error() string {
	msg := fmt.Sprintf("Variable %q refers to a target that cannot be found in the current world.", e.Name)
	if e.Detail != "" {
		msg += " " + e.Detail
	}
	return msg
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
