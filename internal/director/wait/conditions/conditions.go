// Package conditions is what the Director can wait FOR.
//
// The governing rule of the whole wait layer:
//
//	The Director never waits because time passed.
//	It waits because the world has not yet provided sufficient evidence to continue.
//
// A sleep is a guess dressed as a fact. "It usually takes 500ms" is true until the
// machine is loaded, the network is slow, or the animation is longer than it was on the
// developer's desktop — and when it is false, the Director acts into a half-drawn screen
// and reports confidently on what it found there. Worse, a sleep that is too LONG is
// invisible: everything works, slowly, forever.
//
// A condition is falsifiable. It names something observable, it is answered from
// evidence, and its answer can be Unknown — which a duration cannot be.
//
// Every condition here is SEMANTIC. None of them names a platform API, a window handle
// or a poll interval: "the dialog appears", "the Save button is enabled", "the page
// stops changing". That is what lets the same condition be re-evaluated during replay
// against a screen that has since moved.
package conditions

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/wait/evaluation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ID identifies a condition kind, stable and machine-readable.
type ID string

const (
	IDElementExists   ID = "element_exists"
	IDElementMissing  ID = "element_missing"
	IDElementEnabled  ID = "element_enabled"
	IDElementDisabled ID = "element_disabled"
	IDElementFocused  ID = "element_focused"

	IDWindowExists       ID = "window_exists"
	IDWindowClosed       ID = "window_closed"
	IDWindowForeground   ID = "window_foreground"
	IDWindowTitleMatches ID = "window_title_contains"

	IDTextAppears    ID = "text_appears"
	IDTextDisappears ID = "text_disappears"

	IDRegionStable        ID = "region_stable"
	IDRegionChanged       ID = "region_changed"
	IDRegionStillChanging ID = "region_still_changing"

	IDVerificationSatisfied ID = "verification_satisfied"
)

// Condition is something the Director can wait for.
//
// Deliberately tiny. A condition is a DESCRIPTION of a desired state, not the machinery
// for detecting it — evaluation needs a world, an observation cycle and sometimes a
// region watcher, none of which belong in the thing a planner writes down.
type Condition interface {
	ID() ID
	Description() string
}

// WorldCondition is a condition answerable from a World State alone.
//
// Most are. Separated from the others so the engine can answer them without a capture,
// which matters: a wait that took a screenshot to check whether a window had closed
// would be expensive for no reason.
type WorldCondition interface {
	Condition
	Evaluate(w directorapi.WorldState) evaluation.Result
}

// ── element conditions ────────────────────────────────────────────────────────

// ElementQuery names an element the way a user would, not by id.
//
// By QUERY rather than by ElementID, deliberately. An id is meaningful only within one
// session and one identity history; a condition that named one could not survive a
// replay, and "wait until the Save button is enabled" would silently become "wait until
// element e42 is enabled" — a different and much weaker statement.
type ElementQuery struct {
	Label string                  `json:"label,omitempty"`
	Role  directorapi.ElementRole `json:"role,omitempty"`
	// Window narrows the search. Empty means anywhere in the world.
	Window directorapi.WindowID `json:"window,omitempty"`
}

func (q ElementQuery) describe() string {
	parts := []string{}
	if q.Label != "" {
		parts = append(parts, fmt.Sprintf("%q", q.Label))
	}
	if q.Role != "" {
		parts = append(parts, string(q.Role))
	}
	if len(parts) == 0 {
		return "an element"
	}
	return strings.Join(parts, " ")
}

// find returns the elements matching the query, and whether the world could answer at
// all.
//
// The second return is the Unknown gate. A world with no elements is not a world where
// the button is absent — it is a world the Director could not read, and the two must
// not produce the same answer.
func (q ElementQuery) find(w directorapi.WorldState) ([]*directorapi.Element, bool) {
	if len(w.Elements) == 0 {
		return nil, false
	}
	want := strings.ToLower(strings.TrimSpace(q.Label))
	var out []*directorapi.Element
	for _, el := range w.Elements {
		if q.Window != "" && el.WindowID != q.Window {
			continue
		}
		if q.Role != "" && el.Role != q.Role {
			continue
		}
		if want != "" && !strings.Contains(strings.ToLower(el.Label), want) {
			continue
		}
		out = append(out, el)
	}
	return out, true
}

// ElementExists waits for a control to appear.
type ElementExists struct{ Query ElementQuery }

func (c ElementExists) ID() ID { return IDElementExists }
func (c ElementExists) Description() string {
	return "until " + c.Query.describe() + " exists"
}

func (c ElementExists) Evaluate(w directorapi.WorldState) evaluation.Result {
	found, readable := c.Query.find(w)
	if !readable {
		return unreadable(c.Query)
	}
	if len(found) == 0 {
		return evaluation.Deny(0.8, c.Query.describe()+" is not there yet")
	}
	return evaluation.Satisfy(found[0].Confidence,
		c.Query.describe()+" is present", elementEvidence(found[0]))
}

// ElementMissing waits for a control to go away.
//
// The asymmetry with ElementExists matters. "It is not in the world" is only evidence of
// absence if the world is READABLE — otherwise it is evidence of nothing, and a wait
// that treated an unreadable application as "the dialog closed" would proceed into a
// dialog that is still open.
type ElementMissing struct{ Query ElementQuery }

func (c ElementMissing) ID() ID { return IDElementMissing }
func (c ElementMissing) Description() string {
	return "until " + c.Query.describe() + " is gone"
}

func (c ElementMissing) Evaluate(w directorapi.WorldState) evaluation.Result {
	found, readable := c.Query.find(w)
	if !readable {
		return unreadable(c.Query)
	}
	if len(found) > 0 {
		return evaluation.Deny(0.8, c.Query.describe()+" is still there",
			elementEvidence(found[0]))
	}
	// Coverage gates the confidence: a world the Director can barely see into is a
	// weak place to conclude that something is absent from it.
	return evaluation.Satisfy(absenceConfidence(w), c.Query.describe()+" is gone")
}

// ElementEnabled waits for a control to become operable.
//
// STRUCTURAL STATE FIRST. An accessibility source reporting a button enabled is a fact;
// a visual pass concluding the same from colour is an inference, and it is used only
// when nothing structural said anything. That ordering is what stops a wait finishing
// because a greyed-out button happened to be over a light background.
type ElementEnabled struct{ Query ElementQuery }

func (c ElementEnabled) ID() ID { return IDElementEnabled }
func (c ElementEnabled) Description() string {
	return "until " + c.Query.describe() + " is enabled"
}

func (c ElementEnabled) Evaluate(w directorapi.WorldState) evaluation.Result {
	return enabledState(c.Query, w, true)
}

// ElementDisabled waits for a control to become inoperable.
type ElementDisabled struct{ Query ElementQuery }

func (c ElementDisabled) ID() ID { return IDElementDisabled }
func (c ElementDisabled) Description() string {
	return "until " + c.Query.describe() + " is disabled"
}

func (c ElementDisabled) Evaluate(w directorapi.WorldState) evaluation.Result {
	return enabledState(c.Query, w, false)
}

func enabledState(q ElementQuery, w directorapi.WorldState, want bool) evaluation.Result {
	found, readable := q.find(w)
	if !readable {
		return unreadable(q)
	}
	if len(found) == 0 {
		// The control is not there. UNKNOWN rather than unsatisfied: a button that does
		// not exist is not a disabled button, and reporting it as one would let a wait
		// for "disabled" finish the instant the dialog closed.
		return evaluation.Unknowable(q.describe() + " is not present, so whether it is " +
			"enabled cannot be answered")
	}

	el := found[0]
	word := "enabled"
	if !want {
		word = "disabled"
	}

	// Where the state came from decides how much it is worth. StateEvidence records
	// that; without it the Director would treat a colour inference and a tree fact as
	// interchangeable.
	confidence := el.Confidence
	source := directorapi.SourceAccessibility
	if fact, ok := el.StateEvidence[directorapi.StateEnabled]; ok {
		confidence = fact.Confidence
		source = fact.Source
	}

	if el.Enabled == want {
		return evaluation.Satisfy(confidence,
			fmt.Sprintf("%s is %s (per %s)", q.describe(), word, source),
			elementEvidence(el))
	}
	return evaluation.Deny(confidence,
		fmt.Sprintf("%s is not %s yet", q.describe(), word), elementEvidence(el))
}

// ElementFocused waits for keyboard focus to land on a control.
type ElementFocused struct{ Query ElementQuery }

func (c ElementFocused) ID() ID { return IDElementFocused }
func (c ElementFocused) Description() string {
	return "until " + c.Query.describe() + " has keyboard focus"
}

func (c ElementFocused) Evaluate(w directorapi.WorldState) evaluation.Result {
	found, readable := c.Query.find(w)
	if !readable {
		return unreadable(c.Query)
	}
	if len(found) == 0 {
		return evaluation.Unknowable(c.Query.describe() + " is not present")
	}
	for _, el := range found {
		if el.Focused {
			return evaluation.Satisfy(el.Confidence,
				c.Query.describe()+" has focus", elementEvidence(el))
		}
	}
	return evaluation.Deny(0.8, c.Query.describe()+" does not have focus yet",
		elementEvidence(found[0]))
}

// ── window conditions ─────────────────────────────────────────────────────────

// WindowExists waits for a window to appear.
type WindowExists struct {
	Window directorapi.WindowID
	// Title matches instead when the window has no stable id yet — a dialog that does
	// not exist cannot be named by handle.
	Title string
}

func (c WindowExists) ID() ID { return IDWindowExists }
func (c WindowExists) Description() string {
	return "until the window " + c.describeTarget() + " exists"
}

func (c WindowExists) describeTarget() string {
	if c.Title != "" {
		return fmt.Sprintf("titled %q", c.Title)
	}
	return string(c.Window)
}

func (c WindowExists) Evaluate(w directorapi.WorldState) evaluation.Result {
	if len(w.Windows) == 0 {
		return evaluation.Unknowable("no windows were observed at all, so whether this " +
			"one exists cannot be answered")
	}
	if win, ok := matchWindow(w, c.Window, c.Title); ok {
		return evaluation.Satisfy(0.95, "the window "+c.describeTarget()+" is open",
			windowEvidence(win))
	}
	return evaluation.Deny(0.85, "the window "+c.describeTarget()+" has not appeared")
}

// WindowClosed waits for a window to go away.
//
// GONE, not merely unfocused. Those look similar to a user and are entirely different
// to the Director: a window that lost focus is still there, still holds unsaved work,
// and still receives the keystrokes a wait-then-type would send.
type WindowClosed struct {
	Window directorapi.WindowID
	Title  string
}

func (c WindowClosed) ID() ID { return IDWindowClosed }
func (c WindowClosed) Description() string {
	return "until the window " + WindowExists(c).describeTarget() + " closes"
}

func (c WindowClosed) Evaluate(w directorapi.WorldState) evaluation.Result {
	if len(w.Windows) == 0 {
		// No windows observed at all is not "the window closed" — it is a Director that
		// could not enumerate windows. Concluding closure here would let a wait finish
		// the moment observation failed.
		return evaluation.Unknowable("no windows were observed at all, so this cannot be " +
			"read as the window having closed")
	}
	if win, ok := matchWindow(w, c.Window, c.Title); ok {
		detail := "the window is still open"
		if !win.Focused {
			// Said explicitly, because losing focus is the thing most easily mistaken
			// for closing.
			detail = "the window is still open (it lost focus, which is not the same thing)"
		}
		return evaluation.Deny(0.9, detail, windowEvidence(win))
	}
	return evaluation.Satisfy(0.9, "the window is gone from the window list")
}

// WindowForeground waits for a window to come to the front.
type WindowForeground struct {
	Window directorapi.WindowID
	Title  string
}

func (c WindowForeground) ID() ID { return IDWindowForeground }
func (c WindowForeground) Description() string {
	return "until the window " + WindowExists(c).describeTarget() + " is in front"
}

func (c WindowForeground) Evaluate(w directorapi.WorldState) evaluation.Result {
	if len(w.Windows) == 0 {
		return evaluation.Unknowable("no windows were observed")
	}
	win, ok := matchWindow(w, c.Window, c.Title)
	if !ok {
		return evaluation.Unknowable("the window is not in the window list, so whether it " +
			"is in front cannot be answered")
	}
	if win.Focused || (w.ActiveWindow != nil && *w.ActiveWindow == win.ID) {
		return evaluation.Satisfy(0.95, "the window is in front", windowEvidence(win))
	}
	return evaluation.Deny(0.9, "the window is open but not in front", windowEvidence(win))
}

// WindowTitleContains waits for a window's title to include some text — the closest
// thing to "the page finished loading" that the window system alone can express.
type WindowTitleContains struct {
	Window directorapi.WindowID
	Text   string
}

func (c WindowTitleContains) ID() ID { return IDWindowTitleMatches }
func (c WindowTitleContains) Description() string {
	return fmt.Sprintf("until a window title contains %q", c.Text)
}

func (c WindowTitleContains) Evaluate(w directorapi.WorldState) evaluation.Result {
	if len(w.Windows) == 0 {
		return evaluation.Unknowable("no windows were observed")
	}
	want := strings.ToLower(c.Text)
	for i := range w.Windows {
		win := w.Windows[i]
		if c.Window != "" && win.ID != c.Window {
			continue
		}
		if strings.Contains(strings.ToLower(win.Title), want) {
			return evaluation.Satisfy(0.9,
				fmt.Sprintf("the title %q contains %q", win.Title, c.Text),
				windowEvidence(&w.Windows[i]))
		}
	}
	return evaluation.Deny(0.85, fmt.Sprintf("no window title contains %q yet", c.Text))
}

// ── text conditions ───────────────────────────────────────────────────────────

// TextAppears waits for a string to become visible.
//
// It does NOT require OCR. A label the accessibility tree already reports is text that
// has appeared, and demanding a screen capture to confirm it would be slower, less
// reliable, and would make the condition unusable wherever OCR is unavailable. OCR is
// consulted only when structure has nothing to say.
type TextAppears struct {
	Text string
	// Window narrows the search.
	Window directorapi.WindowID
}

func (c TextAppears) ID() ID { return IDTextAppears }
func (c TextAppears) Description() string {
	return fmt.Sprintf("until the text %q appears", c.Text)
}

func (c TextAppears) Evaluate(w directorapi.WorldState) evaluation.Result {
	return textPresence(c.Text, c.Window, w, true)
}

// TextDisappears waits for a string to stop being visible.
type TextDisappears struct {
	Text   string
	Window directorapi.WindowID
}

func (c TextDisappears) ID() ID { return IDTextDisappears }
func (c TextDisappears) Description() string {
	return fmt.Sprintf("until the text %q is gone", c.Text)
}

func (c TextDisappears) Evaluate(w directorapi.WorldState) evaluation.Result {
	return textPresence(c.Text, c.Window, w, false)
}

func textPresence(text string, window directorapi.WindowID,
	w directorapi.WorldState, want bool) evaluation.Result {

	if len(w.Elements) == 0 && len(w.Observations) == 0 {
		return evaluation.Unknowable("nothing was observed, so whether the text is on " +
			"screen cannot be answered")
	}
	want2 := strings.ToLower(strings.TrimSpace(text))

	// Structure first: a label the tree reports IS visible text, and it is the stronger
	// evidence.
	for _, el := range w.Elements {
		if window != "" && el.WindowID != window {
			continue
		}
		if strings.Contains(strings.ToLower(el.Label), want2) ||
			strings.Contains(strings.ToLower(el.Value), want2) {
			if want {
				return evaluation.Satisfy(el.Confidence,
					fmt.Sprintf("%q appears in a %s labelled %q", text, el.Role, el.Label),
					elementEvidence(el))
			}
			return evaluation.Deny(el.Confidence,
				fmt.Sprintf("%q is still on screen, in a %s", text, el.Role),
				elementEvidence(el))
		}
	}

	// Then the raw observations, which is where OCR text lands.
	for _, o := range w.Observations {
		if window != "" && o.WindowID != window {
			continue
		}
		hay := strings.ToLower(o.Label + " " + o.Text + " " + o.Value)
		if strings.Contains(hay, want2) {
			if want {
				return evaluation.Satisfy(o.Confidence,
					fmt.Sprintf("%q was read on screen by %s", text, o.Source),
					evaluation.EvidenceReference{
						Kind: "text", Observation: o.ID, Detail: o.Text,
					})
			}
			return evaluation.Deny(o.Confidence,
				fmt.Sprintf("%q is still on screen, read by %s", text, o.Source))
		}
	}

	if want {
		return evaluation.Deny(absenceConfidence(w),
			fmt.Sprintf("%q has not appeared", text))
	}
	return evaluation.Satisfy(absenceConfidence(w),
		fmt.Sprintf("%q is no longer on screen", text))
}

// ── helpers ───────────────────────────────────────────────────────────────────

// unreadable is the Unknown answer for a world with no elements in it.
func unreadable(q ElementQuery) evaluation.Result {
	return evaluation.Unknowable("no elements were observed at all, so whether " +
		q.describe() + " is there cannot be answered — this is blindness, not absence")
}

// absenceConfidence is how much a conclusion of ABSENCE is worth in this world.
//
// Gated on coverage, because absence is the one conclusion that gets weaker the less
// the Director can see. An application exposing eight anonymous panes will report every
// button as missing, confidently and wrongly.
func absenceConfidence(w directorapi.WorldState) float64 {
	c := w.Confidence.Coverage
	if c <= 0 {
		return 0.3
	}
	return 0.4 + 0.5*c
}

func matchWindow(w directorapi.WorldState, id directorapi.WindowID, title string) (*directorapi.Window, bool) {
	want := strings.ToLower(strings.TrimSpace(title))
	for i := range w.Windows {
		if id != "" && w.Windows[i].ID == id {
			return &w.Windows[i], true
		}
		if id == "" && want != "" && strings.Contains(strings.ToLower(w.Windows[i].Title), want) {
			return &w.Windows[i], true
		}
	}
	return nil, false
}

func elementEvidence(el *directorapi.Element) evaluation.EvidenceReference {
	ref := evaluation.EvidenceReference{
		Kind: "element", Element: el.ID, Window: el.WindowID,
		Detail: fmt.Sprintf("%s %q", el.Role, el.Label),
	}
	if el.Provenance.Len() > 0 {
		ref.Observation = el.Provenance.Sources[0].Observation
	}
	return ref
}

func windowEvidence(win *directorapi.Window) evaluation.EvidenceReference {
	return evaluation.EvidenceReference{
		Kind: "window", Window: win.ID,
		Detail: fmt.Sprintf("%q, focused=%v", win.Title, win.Focused),
	}
}
