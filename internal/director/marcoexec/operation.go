// Package marcoexec is the Director's ONLY route to a desktop effect.
//
// It owns the whole lowering: a typed Operation in, legal Marco source out, then
// Marco's own lexer, parser, graph builder, compiler and runtime. Nothing else in the
// Director may construct Marco source, and nothing in the Director may reach a host.
//
//	Director decides what should happen.
//	Legal Marco describes the effect.
//	Marco's compiler and runtime are the only path by which the effect
//	reaches the computer.
//
// Why one package. Marco string construction spread across action packages would be
// several encoders with several sets of bugs, and the escaping rules alone are subtle
// enough to justify a single implementation (see encode.go — strconv.Quote produces
// invalid Marco). More importantly, the compile-before-mutation guarantee is only a
// guarantee if there is exactly one place where a Director intention becomes an
// effect. Two places is a bypass waiting to be written.
package marcoexec

import (
	"fmt"
	"strings"
)

// Kind names a semantic desktop effect. One per thing the Director can do, not one
// per Marco capability — Click covers left and right, which lower differently.
type Kind string

const (
	KindClick Kind = "click"
	KindMove  Kind = "move"
	KindDrag  Kind = "drag"
	KindKey   Kind = "key"
	KindType  Kind = "type"
	// KindNavigate is one or more NAVIGATION MEANINGS, in order — "confirm", "down".
	//
	// Not KindKey with a chord, and the difference is the whole point. A chord is a
	// decision about a device and an application; the Director learned that somebody
	// CONFIRMED, and it has no standing to decide that confirming is Enter. `OS's
	// Navigate` carries the meaning to the host, which owns the binding — the acting
	// half of [[ADR-013-navigation-is-meaning-not-keys]].
	KindNavigate     Kind = "navigate"
	KindTypeSecret   Kind = "type_secret"
	KindActivate     Kind = "activate"
	KindLaunch       Kind = "launch"
	KindMoveWindow   Kind = "move_window"
	KindWindowState  Kind = "window_state"
	KindFocus        Kind = "focus"
	KindSetValue     Kind = "set_value"
	KindGetValue     Kind = "get_value"
	KindClipboardGet Kind = "clipboard_get"
	KindClipboardSet Kind = "clipboard_set"

	// The STRUCTURAL control effects. Each asks the application to perform the change
	// through its own accessibility API rather than synthesising the input that
	// usually causes it — so there is no pointer to aim, nothing to obscure the
	// target, and a refusal when the control cannot do it at all.
	//
	// One Kind per effect, not one per pattern: KindToggle is "flip this", and whether
	// that reaches TogglePattern or SelectionItemPattern is the host's business.
	KindInvoke         Kind = "invoke"
	KindExpand         Kind = "expand"
	KindCollapse       Kind = "collapse"
	KindToggle         Kind = "toggle"
	KindSelect         Kind = "select"
	KindDeselect       Kind = "deselect"
	KindScrollIntoView Kind = "scroll_into_view"
)

// controlKinds are the operations that act on one control, named by the accessibility
// source's own element id and carried by the Control set.
//
// A set rather than a switch list repeated in four places: Capabilities, Modules,
// Validate and body all need the same answer, and three of them agreeing while the
// fourth drifts is how a capability ends up lowering without being validated.
var controlKinds = map[Kind]string{
	KindFocus:          "Accessibility's Focus",
	KindSetValue:       "Accessibility's SetValue",
	KindGetValue:       "Accessibility's GetValue",
	KindInvoke:         "Accessibility's Invoke",
	KindExpand:         "Accessibility's Expand",
	KindCollapse:       "Accessibility's Collapse",
	KindToggle:         "Accessibility's Toggle",
	KindSelect:         "Accessibility's Select",
	KindDeselect:       "Accessibility's Deselect",
	KindScrollIntoView: "Accessibility's ScrollIntoView",
}

// ControlKind reports whether a kind acts on one accessibility control.
func (k Kind) ControlKind() bool { _, ok := controlKinds[k]; return ok }

// Point is an absolute virtual-desktop coordinate. Negative values are ordinary — a
// monitor left of or above the primary one has them, and they must survive lowering
// unchanged.
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Rect is a window rectangle.
type Rect struct {
	X, Y, W, H int
}

// Operation is one semantic desktop effect, typed.
//
// Callers build this. They never build source: the lowering is this package's, and a
// caller that assembled its own string would be a second encoder with its own
// escaping bugs and its own path around the compiler.
type Operation struct {
	Kind Kind `json:"kind"`

	// At is the target point for Click and Move; From/To are Drag's endpoints.
	At   Point `json:"at,omitempty"`
	From Point `json:"from,omitempty"`
	To   Point `json:"to,omitempty"`
	// Button is "left", "right" or "middle". Empty means left.
	Button string `json:"button,omitempty"`
	// Count is how many clicks. 0 or 1 is a single click.
	Count int `json:"count,omitempty"`

	// Chord is the key or chord for KindKey; Hold is how long to hold it, in ms.
	Chord string `json:"chord,omitempty"`
	Hold  int    `json:"hold,omitempty"`

	// Intents is the ordered navigation meanings for KindNavigate.
	//
	// ORDERED and never deduplicated: `down, down, confirm` is three distinct presses
	// and two of them are the same word. A set would lose the second `down`, and the
	// selection would stop one row short of where the demonstration went.
	Intents []string `json:"intents,omitempty"`

	// Text is the literal text for KindType and KindClipboardSet.
	Text string `json:"text,omitempty"`
	// SensitiveText says Text came from a sensitive captured value, so the retained
	// source must be redacted even though the executed source cannot be. Never
	// serialised as part of the operation's identity — see Sensitive.
	SensitiveText bool `json:"-"`

	// SecretRef NAMES a credential. The value is never here, never lowered into
	// source, and never returned — Marco's OS's Secret fetches it inside the host.
	// That is the entire reason the capability exists.
	SecretRef string `json:"secret_ref,omitempty"`

	// App is the application to activate; Target is the path/URL to launch.
	App    string `json:"app,omitempty"`
	Target string `json:"target,omitempty"`

	// Window is "hwnd:<handle>"; Bounds and State are MoveWindow's and
	// WindowState's payloads.
	Window string `json:"window,omitempty"`
	Bounds Rect   `json:"bounds,omitempty"`
	State  string `json:"state,omitempty"`

	// Element is the accessibility source's own id, for the control operations.
	Element string `json:"element,omitempty"`
	// Called is the control's own LABEL, for a control named durably rather than by id.
	//
	// The difference matters for exactly one reason: a saved play outlives the tree it was
	// learned from. An Element identifies a control in the tree as it stands now and means
	// nothing after a redraw; a label is what the person saw, and it is still true tomorrow.
	// So a direct operation carries an Element and a LEARNED PLAY carries this.
	//
	// Exactly one of the two is set. See Validate, which refuses both and refuses neither.
	Called   string `json:"called,omitempty"`
	Value    string `json:"value,omitempty"`
	MaxNodes int    `json:"max_nodes,omitempty"`
}

// Secret reports whether this operation carries credential material.
//
// Drives redaction everywhere: diagnostics, the action graph, `director lower`. It is
// a property of the OPERATION rather than a flag a caller remembers to set, so a new
// call site cannot forget it.
func (o Operation) Secret() bool { return o.Kind == KindTypeSecret }

// Sensitive marks an operation whose Text came from a sensitive captured value.
//
// Distinct from Secret, and the difference is what each one permits. A SECRET never
// enters the source at all — OS's Secret takes a name, and the value is fetched inside
// the host. A SENSITIVE value must be in the source, because there is no other way to
// put a customer's email into a field; what it must not do is become HISTORY.
//
// So this flag changes exactly one thing: what is RETAINED after execution. The program
// that runs holds the real text; the copy kept for diagnostics does not.
func (o Operation) Sensitive() bool { return o.SensitiveText }

// Capabilities returns the act-and-capability pairs this operation invokes, in order.
//
// The single table mapping Director effects onto Marco's declared vocabulary. Every
// name here is exported by internal/osmod/os.marco or internal/uiamod/accessibility.marco;
// anything else fails the compiler, which is the point.
func (o Operation) Capabilities() ([]string, bool) {
	switch o.Kind {
	case KindClick:
		if o.Button != "" && o.Button != "left" {
			// OS's Click takes EITHER a Point (always left) OR a button name (always
			// at the cursor). A right-click at a point is therefore Move then Click —
			// two capabilities, which is what the language actually offers.
			caps := []string{"OS's Move"}
			for i := 0; i < o.clicks(); i++ {
				caps = append(caps, "OS's Click")
			}
			return caps, true
		}
		caps := make([]string, 0, o.clicks())
		for i := 0; i < o.clicks(); i++ {
			caps = append(caps, "OS's Click")
		}
		return caps, true
	case KindMove:
		return []string{"OS's Move"}, true
	case KindDrag:
		return []string{"OS's Drag"}, true
	case KindKey:
		return []string{"OS's Key"}, true
	case KindNavigate:
		caps := make([]string, 0, len(o.Intents))
		for range o.Intents {
			caps = append(caps, "OS's Navigate")
		}
		return caps, len(caps) > 0
	case KindType:
		return []string{"OS's Type"}, true
	case KindTypeSecret:
		return []string{"OS's Secret"}, true
	case KindActivate:
		return []string{"OS's Activate"}, true
	case KindLaunch:
		return []string{"OS's Launch"}, true
	case KindMoveWindow:
		return []string{"OS's MoveWindow"}, true
	case KindWindowState:
		return []string{"OS's WindowState"}, true
	case KindClipboardGet:
		return []string{"OS's ClipboardGet"}, true
	case KindClipboardSet:
		return []string{"OS's ClipboardSet"}, true
	}
	if cap, ok := controlKinds[o.Kind]; ok {
		return []string{cap}, true
	}
	return nil, false
}

func (o Operation) clicks() int {
	if o.Count <= 0 {
		return 1
	}
	return o.Count
}

// Modules returns the `use` imports this operation needs.
func (o Operation) Modules() []string {
	if o.Kind.ControlKind() {
		return []string{"accessibility"}
	}
	return []string{"os"}
}

// Describe is a one-line human form, safe to log.
//
// Never includes a secret's value — there is none to include, since only the NAME
// ever reaches this package.
func (o Operation) Describe() string {
	switch o.Kind {
	case KindClick:
		return fmt.Sprintf("%s click ×%d at (%d,%d)", o.buttonName(), o.clicks(), o.At.X, o.At.Y)
	case KindMove:
		return fmt.Sprintf("move the pointer to (%d,%d)", o.At.X, o.At.Y)
	case KindNavigate:
		return "navigate: " + strings.Join(o.Intents, ", ")
	case KindDrag:
		return fmt.Sprintf("%s drag from (%d,%d) to (%d,%d)", o.buttonName(), o.From.X, o.From.Y, o.To.X, o.To.Y)
	case KindKey:
		return "press " + o.Chord
	case KindType:
		return fmt.Sprintf("type %d characters", len(o.Text))
	case KindTypeSecret:
		// The NAME is redacted too, not just the value. A credential name can itself be
		// sensitive ("prod-root-password"), and a diagnostic dump shared with someone
		// else would carry it. The operation stays identifiable without it.
		return "type a saved credential (<redacted>)"
	case KindActivate:
		return "activate " + o.App
	case KindLaunch:
		return "launch " + o.Target
	case KindMoveWindow:
		return fmt.Sprintf("move %s to (%d,%d %dx%d)", o.Window, o.Bounds.X, o.Bounds.Y, o.Bounds.W, o.Bounds.H)
	case KindWindowState:
		return fmt.Sprintf("set %s to %s", o.Window, o.State)
	case KindFocus:
		return "focus " + o.Element
	case KindSetValue:
		return fmt.Sprintf("set the value of %s", o.Element)
	case KindGetValue:
		return "read the value of " + o.Element
	case KindClipboardGet:
		return "read the clipboard"
	case KindClipboardSet:
		return fmt.Sprintf("put %d characters on the clipboard", len(o.Text))
	case KindInvoke:
		return "invoke " + o.controlName()
	case KindExpand:
		return "expand " + o.Element
	case KindCollapse:
		return "collapse " + o.Element
	case KindToggle:
		return "toggle " + o.Element
	case KindSelect:
		return "select " + o.Element
	case KindDeselect:
		return "deselect " + o.Element
	case KindScrollIntoView:
		return "scroll " + o.Element + " into view"
	}
	return string(o.Kind)
}

func (o Operation) buttonName() string {
	if o.Button == "" {
		return "left"
	}
	return o.Button
}

// Validate checks an operation before anything is generated.
//
// Runs first so an unsupported or malformed operation fails with a Director-level
// message rather than as a Marco syntax error nobody can act on. It does not replace
// the compiler and is not allowed to: the compiler still gets the final say on
// whether Marco can express this.
func (o Operation) Validate() error {
	if _, ok := o.Capabilities(); !ok {
		return fmt.Errorf("marcoexec: %q is not an operation the Director can express in Marco", o.Kind)
	}
	switch o.Kind {
	case KindKey:
		if strings.TrimSpace(o.Chord) == "" {
			return fmt.Errorf("marcoexec: a key operation needs a chord")
		}
	case KindNavigate:
		if len(o.Intents) == 0 {
			return fmt.Errorf("marcoexec: a navigate operation needs at least one meaning")
		}
		for _, in := range o.Intents {
			if !NavigationMeanings[in] {
				// Refused HERE as well as at the host. A meaning the Director cannot
				// name is a program that never runs, rather than a program that runs
				// and asks a live desktop to interpret a typo.
				return fmt.Errorf("marcoexec: %q is not a navigation meaning", in)
			}
		}
	case KindTypeSecret:
		if strings.TrimSpace(o.SecretRef) == "" {
			return fmt.Errorf("marcoexec: a secret operation needs the credential's name")
		}
	case KindActivate:
		if strings.TrimSpace(o.App) == "" {
			return fmt.Errorf("marcoexec: activate needs an application")
		}
	case KindLaunch:
		if strings.TrimSpace(o.Target) == "" {
			return fmt.Errorf("marcoexec: launch needs something to launch")
		}
	case KindClick, KindDrag:
		if b := o.Button; b != "" && b != "left" && b != "right" && b != "middle" {
			return fmt.Errorf("marcoexec: %q is not a mouse button Marco knows", b)
		}
	case KindMoveWindow:
		if err := checkWindow(o.Window); err != nil {
			return err
		}
		if o.Bounds.W <= 0 || o.Bounds.H <= 0 {
			// The host rejects this too, but rejecting here means the window is never
			// touched rather than touched and then failed.
			return fmt.Errorf("marcoexec: a window move needs a positive width and height, got %dx%d",
				o.Bounds.W, o.Bounds.H)
		}
	case KindWindowState:
		if err := checkWindow(o.Window); err != nil {
			return err
		}
		switch o.State {
		case "normal", "maximized", "minimized":
		default:
			return fmt.Errorf("marcoexec: %q is not a window state Marco knows", o.State)
		}
	}
	// Every control operation needs to say WHICH control, in exactly one of the two ways
	// there are. Without either, the program would compile and then act on the foreground
	// window's idea of "nothing", which is a failure the host would have to invent a message
	// for. With both, there would be two answers and the host would silently prefer one.
	if o.Kind.ControlKind() {
		el, called := strings.TrimSpace(o.Element), strings.TrimSpace(o.Called)
		switch {
		case el == "" && called == "":
			return fmt.Errorf(
				"marcoexec: %s needs either the source's element id or the control's label",
				o.Kind)
		case el != "" && called != "":
			return fmt.Errorf(
				"marcoexec: %s was given both an element id and a label (%q); a control is "+
					"identified one way or the other, never both", o.Kind, called)
		}
		if o.Window != "" {
			if err := checkWindow(o.Window); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkWindow rejects a handle that is not in the form the host parses.
//
// Caught here rather than at the host, because a malformed handle that reaches oshost
// is a failed act on a live desktop, while one caught here is an operation that never
// ran.
func checkWindow(w string) error {
	if strings.TrimSpace(w) == "" {
		return fmt.Errorf("marcoexec: this operation needs a window")
	}
	if !strings.HasPrefix(strings.ToLower(w), "hwnd:") {
		return fmt.Errorf("marcoexec: a window must look like %q, got %q", "hwnd:<handle>", w)
	}
	return nil
}

// NavigationMeanings is the closed vocabulary KindNavigate accepts.
//
// The same seven words the observation side produces, and deliberately NOT `point`: a pointer
// press needs a position, a position is not a meaning, and `OS's Click` with a Point is the
// capability that already exists for it. Naming `point` here would invite a coordinate into a
// vocabulary whose whole value is that it holds none.
var NavigationMeanings = map[string]bool{
	"up": true, "down": true, "left": true, "right": true,
	"confirm": true, "back": true, "pause": true,
}

// controlName is how a description refers to the control an operation acts on.
//
// The LABEL when there is one, because that is the word a person would use, and the source's id
// otherwise. A diagnostic that printed a UIA RuntimeId at somebody who asked what Marco was about
// to do would be answering a different question.
func (o Operation) controlName() string {
	if o.Called != "" {
		// Quoted for a reader, not for Marco: this string never becomes source. The
		// package note about strconv.Quote applies to encoding, which is encode.go.
		return fmt.Sprintf("%q", o.Called)
	}
	return o.Element
}
