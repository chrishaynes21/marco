// Package uiaclient is the typed Go side of the accessibility bridge. It drives
// plugins/uia (or any process speaking the same protocol) and turns its report into
// directorapi observations.
//
// It lives in internal/platform, not in the Director, because it knows two things
// the Director must not: that there is a subprocess, and what its wire format looks
// like. The Director sees only directorapi.AccessibilityProvider — which is what
// makes the eventual switch to an in-process implementation an optimisation rather
// than a rewrite.
//
// The mapping this package performs is not clerical. It is where a provider's
// dialect becomes the Director's vocabulary, and where a provider's silence is kept
// distinguishable from a provider's denial.
package uiaclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// accessibilityAct is the Marco act name the bridge is registered under.
const accessibilityAct = "Accessibility"

// Provider reads the accessibility tree through a Marco host.
type Provider struct {
	host runtime.Host
	// now is injectable so a fixture-driven test produces a byte-identical
	// snapshot rather than one stamped with the clock.
	now func() time.Time

	// MaxNodes, MaxDepth and Timeout bound every snapshot. They are sent to the
	// provider rather than enforced here, because the cost being bounded is the
	// provider's cross-process tree walk, not our decoding.
	MaxNodes int
	MaxDepth int
	Timeout  time.Duration
}

// Default bounds. A foreground-window walk is normally far under the node cap; the
// cap exists for the pathological case (a browser exposing tens of thousands of
// nodes) where an unbounded walk would stall the whole observe→act→observe loop.
const (
	defaultMaxNodes = 1500
	defaultMaxDepth = 40
	defaultTimeout  = 4 * time.Second
)

// New returns a provider driving the given Marco host.
func New(h runtime.Host) *Provider {
	return &Provider{
		host:     h,
		now:      time.Now,
		MaxNodes: defaultMaxNodes,
		MaxDepth: defaultMaxDepth,
		Timeout:  defaultTimeout,
	}
}

var _ directorapi.AccessibilityProvider = (*Provider)(nil)

// wireSnapshot mirrors the bridge's Snapshot response.
type wireSnapshot struct {
	WindowID      string `json:"WindowId"`
	WindowTitle   string `json:"WindowTitle"`
	App           string `json:"App"`
	ProcessID     int    `json:"ProcessId"`
	WindowX       int    `json:"WindowX"`
	WindowY       int    `json:"WindowY"`
	WindowW       int    `json:"WindowW"`
	WindowH       int    `json:"WindowH"`
	Minimized     bool   `json:"Minimized"`
	Maximized     bool   `json:"Maximized"`
	WindowVisible bool   `json:"WindowVisible"`
	Partial       bool   `json:"Partial"`
	Reason        string `json:"Reason"`
	ElapsedMs     int    `json:"ElapsedMs"`

	Elements []wireElement `json:"Elements"`

	// Shell is what a File Explorer window.s own view reported, when the bridge asked.
	// Omitted for every window that has no shell view, which is most of them.
	Shell *wireShell `json:"Shell,omitempty"`
}

// wireElement mirrors one element in the bridge's report.
type wireElement struct {
	ID           string `json:"Id"`
	ParentID     string `json:"ParentId"`
	Role         string `json:"Role"`
	ControlType  string `json:"ControlType"`
	Label        string `json:"Label"`
	Value        string `json:"Value"`
	Description  string `json:"Description"`
	X            int    `json:"X"`
	Y            int    `json:"Y"`
	W            int    `json:"W"`
	H            int    `json:"H"`
	Enabled      bool   `json:"Enabled"`
	Visible      bool   `json:"Visible"`
	Focused      bool   `json:"Focused"`
	Selected     bool   `json:"Selected"`
	Offscreen    bool   `json:"Offscreen"`
	AutomationID string `json:"AutomationId"`
	ClassName    string `json:"ClassName"`
	Depth        int    `json:"Depth"`

	// Patterns is the comma-separated list of accessibility patterns the control
	// implements, which is what the Director's capability ladder reads to know whether
	// a tree node can expand before trying to.
	//
	// A bridge that predates this field sends nothing, and the empty string is
	// deliberately NOT the same as "implements none" — see the observation builder,
	// where the difference decides whether a silent provider vetoes the strong rung.
	Patterns string `json:"Patterns"`
	// Expanded and Checked are three-valued: "true", "false", or "" for a control with
	// no such state. Empty must stay unknown: a tree node with no children is neither
	// expanded nor collapsed, and reading absence as "collapsed" would let an expand
	// that did nothing report success.
	Expanded string `json:"Expanded"`
	Checked  string `json:"Checked"`

	// Resource is the object behind this control, when the bridge could establish one.
	//
	// A POINTER, and omitted by a bridge that has none to report — which is almost every
	// control and every bridge older than this field. Absent and empty must stay
	// distinguishable: the binding layer refuses a control with no backing resource, and
	// an always-present zero value would make "the bridge is old" indistinguishable from
	// "this thing has no file behind it".
	Resource *wireResource `json:"Resource,omitempty"`
}

// wireResource mirrors the bridge's account of the object behind a control.
type wireResource struct {
	Kind        string  `json:"Kind"`
	Path        string  `json:"Path"`
	ParsingName string  `json:"ParsingName"`
	DisplayName string  `json:"DisplayName"`
	Source      string  `json:"Source"`
	Confidence  float64 `json:"Confidence"`
	Link        bool    `json:"Link"`
	// Evidence arrives as one string, pipe-separated, for the reason Patterns does: the
	// reader decodes flat fields and one shape is easier to keep honest than two.
	Evidence string `json:"Evidence"`
}

// Snapshot walks the accessibility tree. The zero WindowID means the foreground
// window, which is the normal case.
func (p *Provider) Snapshot(ctx context.Context, scope directorapi.WindowID) (directorapi.AccessibilitySnapshot, error) {
	in := runtime.NewSet()
	if scope != "" {
		in.Put("Window", runtime.Text(string(scope)))
	}
	in.Put("MaxNodes", runtime.Number(float64(p.maxNodes())))
	in.Put("MaxDepth", runtime.Number(float64(p.maxDepth())))
	in.Put("TimeoutMs", runtime.Number(float64(p.timeout().Milliseconds())))

	status, data, err := p.host.Invoke(runtime.HostCall{
		Act:    accessibilityAct,
		Action: "Snapshot",
		Input:  runtime.SetVal(in),
		Out:    io.Discard,
		Ctx:    ctx,
	})
	if err != nil {
		return directorapi.AccessibilitySnapshot{}, fmt.Errorf("uiaclient: snapshot: %w", err)
	}
	if status == "failed" {
		return directorapi.AccessibilitySnapshot{}, fmt.Errorf("uiaclient: snapshot failed: %s", errText(data))
	}

	var wire wireSnapshot
	if err := decode(data, &wire); err != nil {
		return directorapi.AccessibilitySnapshot{}, fmt.Errorf("uiaclient: decoding the snapshot: %w", err)
	}
	return p.convert(wire), nil
}

// Available reports whether the provider can serve requests right now.
//
// It answers false on any error rather than propagating one. The caller's question
// is "can I rely on this source?", and every negative answer — bridge missing,
// process dead, no automation element — means the same thing to the Director: build
// a degraded world and say so.
//
// The REASON the provider gave is not lost any more, it is just not what this caller asked for.
// See AskAvailability, which both this and the Theater's accessibility Actor go through, so one
// machine cannot hold two answers about whether it can act.
func (p *Provider) Available(ctx context.Context) bool {
	return AskAvailability(ctx, p.host).Available
}

// Focus moves keyboard focus to an element, without activating it.
//
// This is the one ACTION this provider performs, and it is here rather than among
// Marco's input primitives because focusing cannot be expressed as input: clicking a
// control activates it, and "focus the search box" must not press anything. Only the
// application itself can move focus without side effects, and the accessibility
// interface is how you ask it to.
func (p *Provider) Focus(ctx context.Context, window directorapi.WindowID, nativeID string) error {
	if nativeID == "" {
		return fmt.Errorf("uiaclient: focus needs the provider's own element id")
	}
	in := runtime.NewSet()
	// The Director's ElementID is its own; the provider knows the element by the
	// native id it originally reported, which is what NativeID preserved.
	in.Put("Element", runtime.Text(nativeID))
	if window != "" {
		in.Put("Window", runtime.Text(string(window)))
	}
	in.Put("MaxNodes", runtime.Number(float64(p.maxNodes())))

	status, data, err := p.host.Invoke(runtime.HostCall{
		Act:    accessibilityAct,
		Action: "Focus",
		Input:  runtime.SetVal(in),
		Out:    io.Discard,
		Ctx:    ctx,
	})
	if err != nil {
		return fmt.Errorf("uiaclient: focus: %w", err)
	}
	if status == "failed" {
		return fmt.Errorf("uiaclient: focus failed: %s", errText(data))
	}
	return nil
}

// convert turns the wire report into directorapi observations.
func (p *Provider) convert(wire wireSnapshot) directorapi.AccessibilitySnapshot {
	snap := directorapi.AccessibilitySnapshot{
		Timestamp:    p.now(),
		Partial:      wire.Partial,
		ResourceNote: wire.Shell.Note(),
		Reason:       wire.Reason,
	}

	windowID := directorapi.WindowID(wire.WindowID)
	// What the bridge reports it ACTUALLY walked. Independent of what was asked: if the
	// requested handle had died and the bridge fell back to the foreground window, this
	// names that window instead — which is the only signal a caller has that the answer
	// describes something else.
	snap.ObservedWindow, snap.ObservedApp = windowID, wire.App
	if wire.WindowID != "" {
		snap.Windows = []directorapi.Window{{
			ID:          windowID,
			Application: wire.App,
			Title:       wire.WindowTitle,
			Bounds: directorapi.Rect{
				X: wire.WindowX, Y: wire.WindowY,
				Width: wire.WindowW, Height: wire.WindowH,
			},
			// This provider is always asked about one window at a time and that
			// window is the foreground one unless a scope was given, so Focused
			// tracks visibility rather than being asserted. The window provider is
			// the authority on focus across all windows.
			Focused:   wire.WindowVisible && !wire.Minimized,
			Minimized: wire.Minimized,
			Maximized: wire.Maximized,
			Visible:   wire.WindowVisible,
		}}
	}

	snap.Observations = make([]directorapi.Observation, 0, len(wire.Elements))
	for _, e := range wire.Elements {
		snap.Observations = append(snap.Observations, p.observation(e, windowID, snap.Timestamp))
	}
	return snap
}

func (p *Provider) observation(e wireElement, window directorapi.WindowID, at time.Time) directorapi.Observation {
	enabled, visible := e.Enabled, e.Visible
	focused, selected := e.Focused, e.Selected

	return directorapi.Observation{
		// Deterministic, derived from UIA's own RuntimeId: the same element produces
		// the same observation ID every snapshot, which is what makes a recorded
		// fixture reproducible and provenance traceable across runs.
		ID:        directorapi.ObservationID("acc:" + e.ID),
		Source:    directorapi.SourceAccessibility,
		Timestamp: at,
		WindowID:  window,

		Role:        normaliseRole(e.Role),
		Label:       e.Label,
		Value:       e.Value,
		Description: e.Description,
		Bounds: directorapi.Rect{
			X: e.X, Y: e.Y, Width: e.W, Height: e.H,
		},

		// Set as pointers because accessibility genuinely KNOWS these, unlike OCR.
		// Fusion distinguishes "reported false" from "not reported", and collapsing
		// them here would let a source's silence read as a denial.
		Enabled:  &enabled,
		Visible:  &visible,
		Focused:  &focused,
		Selected: &selected,

		// Accessibility does not produce a per-element confidence: it is reporting
		// what the application declared, not guessing. Its standing on the evidence
		// ladder is carried separately by SourceRank.
		Confidence: 1.0,

		NativeID:       e.ID,
		ParentNativeID: e.ParentID,

		Attributes: attributesOf(e),
		Resource:   resourceOf(e),
	}
}

// resourceOf turns the bridge's account of the object behind a control into the typed
// form, or into nothing.
//
// FAILS CLOSED at every step. A resource with no path, an unrecognised kind, or a path
// the bridge could not canonicalise produces nil — the same answer as a bridge that
// reported nothing — because a half-established identity is exactly what the binding layer
// must not be handed.
func resourceOf(e wireElement) *directorapi.ResourceIdentity {
	r := e.Resource
	if r == nil || strings.TrimSpace(r.Path) == "" {
		return nil
	}
	switch r.Kind {
	case directorapi.ResourceFile, directorapi.ResourceFolder:
	default:
		// A kind this build cannot check. Refused rather than passed through: the
		// binding layer would have to guess what to do with it, and guessing is the
		// thing this whole path exists to remove.
		return nil
	}
	out := &directorapi.ResourceIdentity{
		Kind: r.Kind, Path: r.Path, ParsingName: r.ParsingName,
		DisplayName: r.DisplayName, Source: r.Source,
		Confidence: r.Confidence, Link: r.Link,
	}
	if out.Source == "" {
		out.Source = "accessibility_bridge"
	}
	if out.Confidence == 0 {
		out.Confidence = 1
	}
	for _, part := range strings.Split(r.Evidence, "|") {
		if part = strings.TrimSpace(part); part != "" {
			out.Evidence = append(out.Evidence, part)
		}
	}
	return out
}

// attributesOf carries the source-specific detail the common shape does not model.
//
// The pattern list and the two three-valued states go here rather than into new
// Observation fields for the reason Attributes exists: they are what THIS provider
// knows, other sources have no equivalent, and a core field would force every one of
// them to answer a question only accessibility can.
//
// Absence is meaningful and is preserved. A bridge that does not report patterns leaves
// the key out entirely, and the Director's ladder reads that as "unknown" rather than
// as "supports none" — the difference between offering the strong structural rung and
// falling every control on the desktop through to a click.
func attributesOf(e wireElement) map[string]any {
	attrs := map[string]any{
		"automation_id": e.AutomationID,
		"class_name":    e.ClassName,
		"control_type":  e.ControlType,
		"depth":         e.Depth,
		"offscreen":     e.Offscreen,
	}
	if e.Patterns != "" {
		attrs["patterns"] = e.Patterns
	}
	if b, ok := triState(e.Expanded); ok {
		attrs["expanded"] = b
	}
	if b, ok := triState(e.Checked); ok {
		attrs["checked"] = b
	}
	return attrs
}

// triState reads the bridge's three-valued string. The bool is what keeps "" — a
// control with no such state — from becoming false.
func triState(s string) (bool, bool) {
	switch s {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

// knownRoles is the closed set a provider may map into.
var knownRoles = map[string]directorapi.ElementRole{
	"button": directorapi.RoleButton, "text_field": directorapi.RoleTextField,
	"checkbox": directorapi.RoleCheckbox, "radio": directorapi.RoleRadio,
	"combo_box": directorapi.RoleComboBox, "list": directorapi.RoleList,
	"list_item": directorapi.RoleListItem, "menu": directorapi.RoleMenu,
	"menu_item": directorapi.RoleMenuItem, "tab": directorapi.RoleTab,
	"tab_list": directorapi.RoleTabList, "link": directorapi.RoleLink,
	"image": directorapi.RoleImage, "icon": directorapi.RoleIcon,
	"text": directorapi.RoleText, "heading": directorapi.RoleHeading,
	"dialog": directorapi.RoleDialog, "window": directorapi.RoleWindow,
	"pane": directorapi.RolePane, "group": directorapi.RoleGroup,
	"toolbar": directorapi.RoleToolbar, "scroll_bar": directorapi.RoleScrollBar,
	"title_bar": directorapi.RoleTitleBar,
	"slider":    directorapi.RoleSlider, "progress_bar": directorapi.RoleProgressBar,
	"table": directorapi.RoleTable, "row": directorapi.RoleRow,
	"cell": directorapi.RoleCell, "tree": directorapi.RoleTree,
	"tree_item": directorapi.RoleTreeItem, "toggle": directorapi.RoleToggle,
}

// normaliseRole maps a provider's role string into the Director's vocabulary,
// degrading to RoleUnknown rather than letting an unrecognised string through.
//
// This is a trust boundary, not a convenience. A provider is a subprocess that could
// be replaced, updated or (for a third-party skill) untrusted; letting it inject an
// arbitrary role would let it inject one that planning or policy special-cases.
func normaliseRole(s string) directorapi.ElementRole {
	if r, ok := knownRoles[s]; ok {
		return r
	}
	return directorapi.RoleUnknown
}

// decode re-marshals a bridge Value into a typed struct.
//
// The Value already came from JSON, so this round-trips it rather than walking the
// Value tree field by field: one marshal/unmarshal for a whole snapshot, against a
// hundred lines of manual extraction that would need updating with every field.
func decode(v runtime.Value, out any) error {
	raw, err := json.Marshal(runtime.JSONFromValue(v))
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func errText(v runtime.Value) string {
	if e := v.AsError(); e != nil {
		return e.Message
	}
	if s := v.AsText(); s != "" {
		return s
	}
	return "no detail"
}

func (p *Provider) maxNodes() int {
	if p.MaxNodes > 0 {
		return p.MaxNodes
	}
	return defaultMaxNodes
}

func (p *Provider) maxDepth() int {
	if p.MaxDepth > 0 {
		return p.MaxDepth
	}
	return defaultMaxDepth
}

func (p *Provider) timeout() time.Duration {
	if p.Timeout > 0 {
		return p.Timeout
	}
	return defaultTimeout
}

// wireShell is the bridge's account of a File Explorer view.
type wireShell struct {
	Available     bool   `json:"Available"`
	FolderPath    string `json:"FolderPath"`
	SelectedCount int    `json:"SelectedCount"`
	Refusal       string `json:"Refusal"`
}

// Note renders the view's account as one diagnostic sentence, empty when there is
// nothing worth saying.
//
// A refusal is worth saying and a success is not: an element that got a resource carries
// its own evidence, and repeating it at snapshot level would be noise on every walk. What
// a reader needs is the case where the bridge LOOKED and came back with nothing, which is
// otherwise indistinguishable from its never having looked.
func (s *wireShell) Note() string {
	if s == nil || s.Refusal == "" {
		return ""
	}
	if s.FolderPath != "" {
		return fmt.Sprintf("no backing resource: %s (the view is showing %s, %d item(s) selected)",
			s.Refusal, s.FolderPath, s.SelectedCount)
	}
	return "no backing resource: " + s.Refusal
}
