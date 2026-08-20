// Package values is the Director's program-local semantic data.
//
// The governing distinction:
//
//	Semantic target variables answer "WHICH OBJECT?".
//	Captured values answer "WHAT INFORMATION?".
//
//	Objects always re-resolve. Values never re-resolve.
//
// A target variable stores a query and proves its object again from the current world
// every single time. A captured value is the opposite: it is a fact observed once, at
// a moment, and it does not change afterwards. If the field it came from is edited a
// second later, the value does not follow — that is what makes it usable as data.
//
// They are also scoped differently, and deliberately so. Target variables are user
// knowledge and persist. Values belong to ONE running program and vanish when it ends,
// because a value that outlived its program would be a fact about a screen nobody is
// looking at any more, silently reused in a context it was never captured for.
package values

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Kind is what sort of information a value holds.
//
// Typed rather than collapsed into string, because the source changes what the value
// MEANS and what may safely be done with it. A window title is public and cheap to
// re-read; a control value may be a password. Flattening them would lose exactly the
// distinction that decides whether it can be logged.
type Kind string

const (
	// KindText is literal text the user supplied or selected.
	KindText Kind = "text"
	// KindWindowTitle came from a window's title.
	KindWindowTitle Kind = "window_title"
	// KindClipboard was read from the system clipboard.
	KindClipboard Kind = "clipboard"
	// KindControlValue was read from a control's value.
	KindControlValue Kind = "control_value"
)

// Visibility is how freely a value may be shown.
type Visibility string

const (
	// VisibilityNormal may appear in traces and explanations.
	VisibilityNormal Visibility = "normal"
	// VisibilitySensitive is shown redacted by default but is not a credential —
	// a customer name, an address. Length and type may still be reported.
	VisibilitySensitive Visibility = "sensitive"
	// VisibilitySecret is credential material. Never printed, never logged, never
	// traced, never explained. Only the host ever sees the plaintext.
	VisibilitySecret Visibility = "secret"
)

// Redacted is what a non-normal value renders as, everywhere.
const Redacted = "<secret>"

// Value is one captured fact.
//
// IMMUTABLE by construction: every field is set at capture and there is no method that
// changes one. Callers receive copies. That is not a stylistic choice — a mutable value
// would let a later step silently alter what an earlier step captured, and the program
// would produce a result no single step asked for.
//
// Note what a value CANNOT hold: there is no ElementID, no WindowID, no handle, no
// Rect and no Point. A value is DATA, never IDENTITY. Identity belongs to target
// variables, which re-resolve it; a value that carried identity would be a stale handle
// wearing a different name.
type Value struct {
	kind       Kind
	text       string
	visibility Visibility
	source     string
	capturedAt time.Time
	// verified records that the capture proved something was actually there. A value
	// that was never verified must not exist — see New.
	verified bool
	// prov is the recorded account of the capture, for explanations. Metadata only:
	// see Provenance, which has nowhere to put content.
	prov Provenance
}

// WithProvenance returns a copy carrying the recorded account of its capture.
//
// A COPY rather than a setter, because Value is immutable and the whole design leans on
// that. The capture builds the value, then attaches what it knows about how it got it;
// nothing afterwards can alter either.
func (v Value) WithProvenance(p Provenance) Value {
	v.prov = p
	if p.Source != "" && v.source == "" {
		v.source = p.Source
	}
	return v
}

// Provenance is how this value was captured.
func (v Value) Provenance() Provenance { return v.prov }

// New builds a value.
//
// It REFUSES an unverified capture. "Unknown is not empty" is the whole reason: a
// selection that could not be read and a selection that is genuinely empty produce the
// same empty string, and only one of them is a fact. Binding the first as "" would let
// a program type nothing into a field and report success.
func New(kind Kind, text, source string, vis Visibility) (Value, error) {
	if kind == "" {
		return Value{}, fmt.Errorf("values: a captured value needs a type")
	}
	if vis == "" {
		vis = VisibilityNormal
	}
	return Value{
		kind: kind, text: text, visibility: vis, source: source,
		capturedAt: time.Now(), verified: true,
	}, nil
}

// ErrUnknown reports that a capture could not determine the value.
//
// Distinct from an empty value, and the distinction is the point. This stops a program;
// an empty-but-verified value does not.
type ErrUnknown struct {
	Name   string
	Source string
	Reason string
}

func (e *ErrUnknown) Error() string {
	return fmt.Sprintf("could not capture %q from %s: %s — nothing was stored, because "+
		"an unreadable value is not an empty one", e.Name, e.Source, e.Reason)
}

// Kind returns the value's type.
func (v Value) Kind() Kind { return v.kind }

// Visibility returns how freely it may be shown.
func (v Value) Visibility() Visibility { return v.visibility }

// Source names where it came from.
func (v Value) Source() string { return v.source }

// CapturedAt is when it was observed.
func (v Value) CapturedAt() time.Time { return v.capturedAt }

// Len is the character count, safe to report for any visibility.
func (v Value) Len() int { return len(v.text) }

// Empty reports whether the captured value is the empty string.
//
// A verified empty value is a real fact: the field WAS empty. That is different from
// never having read it, which produces no Value at all.
func (v Value) Empty() bool { return v.text == "" }

// Plaintext returns the actual content.
//
// The ONLY accessor that returns it, deliberately narrow so every call site is
// findable. It exists for one purpose: handing the value to the host that will type or
// set it. Nothing that logs, traces, explains or serialises may call it.
func (v Value) Plaintext() string { return v.text }

// String is the safe rendering, and is what fmt uses everywhere by default.
//
// Implementing String() is the load-bearing part: a secret accidentally passed to a
// log line, an error message or a %v format renders as <secret> rather than as itself.
// Relying on every call site to remember would mean one forgotten format verb leaks a
// password.
func (v Value) String() string {
	switch v.visibility {
	case VisibilitySecret, VisibilitySensitive:
		return Redacted
	}
	return v.text
}

// Describe is a one-line summary for explanations, never containing a secret.
func (v Value) Describe() string {
	if v.visibility != VisibilityNormal {
		return fmt.Sprintf("%s %s (%d characters)", v.kind, Redacted, v.Len())
	}
	if v.Empty() {
		return string(v.kind) + " (empty, and verified as empty)"
	}
	return fmt.Sprintf("%s %q", v.kind, truncate(v.text, 40))
}

// namePattern is what a value may be called. Same shape as a variable name, so a user
// does not have to remember two rules.
var namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// NormalizeName validates and lower-cases a value name.
func NormalizeName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	name = strings.TrimPrefix(name, "$")
	name = strings.TrimPrefix(name, "{")
	name = strings.TrimSuffix(name, "}")
	if name == "" {
		return "", fmt.Errorf("a value needs a name")
	}
	if !namePattern.MatchString(name) {
		return "", fmt.Errorf("%q is not a usable name: use letters, digits and "+
			"underscores, starting with a letter", raw)
	}
	return strings.ToLower(name), nil
}

// Reference is a program-local value named in a request: the `${customer}` in
// "type ${customer}".
//
// A TYPE rather than a bare string, and that is the whole of Part 4. A reference that
// stayed embedded in a phrase would have to be substituted into it later, and textual
// substitution is how "type ${customer}" becomes "type " when the capture failed —
// silently typing nothing instead of refusing. Carried as structured data, an
// unresolvable reference has nowhere to degrade to.
type Reference struct {
	Name string `json:"name"`
}

// String renders the reference the way the user wrote it.
func (r Reference) String() string { return "${" + r.Name + "}" }

// valueRef matches the ${name} form.
//
// Braces, so that an object reference and a value reference are DIFFERENT tokens the
// parser can tell apart without knowing what has been captured. `$save` asks which
// object; `${customer}` asks what information. Deciding between them by looking up the
// name would make the meaning of a phrase depend on program state, so the same words
// would mean different things at different moments.
var valueRef = regexp.MustCompile(`^\$\{([^}]*)\}$`)

// ParseReference reads a "${name}" token.
//
// The anchors are load-bearing: only a token that is ENTIRELY a reference is one.
// "hello ${name}" is not a reference with some text around it, it is a concatenation,
// and this milestone does not do concatenation — see ParseInput, which refuses it by
// name rather than quietly interpolating.
func ParseReference(token string) (Reference, bool) {
	m := valueRef.FindStringSubmatch(strings.TrimSpace(token))
	if m == nil {
		return Reference{}, false
	}
	name, err := NormalizeName(m[1])
	if err != nil {
		return Reference{}, false
	}
	return Reference{Name: name}, true
}

// ContainsReference reports whether a phrase holds a ${...} reference anywhere.
func ContainsReference(s string) bool { return strings.Contains(s, "${") }

// TextCompatible reports whether this value may be used as text in an edit.
//
// An explicit rule rather than a call to String(), because "can this be typed?" is a
// question about the value's KIND and VISIBILITY, not about how it prints. The default
// branch refuses: a kind added later must state its own answer here, and until it does
// it cannot be silently stringified into somebody's document.
func (v Value) TextCompatible() error {
	if v.visibility == VisibilitySecret {
		// Refused before it can be lowered. A secret reaching a text operation would be
		// embedded in generated Marco source, which is exactly what the named-secret
		// mechanism exists to avoid.
		return fmt.Errorf("a secret value cannot be used as text; credentials are typed " +
			"through Marco's named secret mechanism, never as literal source")
	}
	switch v.kind {
	case KindText, KindWindowTitle, KindControlValue:
		return nil
	case KindClipboard:
		// Only plain text is ever bound as a clipboard value — a picture on the
		// clipboard produces Unknown rather than an empty string — so a bound clipboard
		// value is text by construction.
		return nil
	}
	return fmt.Errorf("a %s value has no defined text conversion", v.kind)
}

// emailish is the one content pattern worth upgrading on.
//
// Not an attempt to detect private data in general, which cannot be done and would give
// false confidence if attempted. It catches the single most common case of a field
// value that is personal, and it only ever raises the visibility.
var emailish = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Classify decides how freely a captured value may be shown.
//
// Conservative by SOURCE first, because the source is known before the content is read
// and is the more reliable signal: anything typed into a control may be personal,
// whatever it happens to say. Content can only raise the level, never lower it — a
// window title that looks like an email is still a window title, but a field value that
// looks like an email is not going to be printed either way.
//
// Nothing here ever returns Secret. A secret is not something to be classified after
// reading; it is something to REFUSE to read, which is where that decision lives.
func Classify(k Kind, text string) Visibility {
	base := VisibilityNormal
	switch k {
	case KindControlValue:
		// Whatever a person typed into a field. Public far more often than not, but the
		// exceptions are the ones that matter and they are not distinguishable by
		// looking.
		base = VisibilitySensitive
	case KindClipboard:
		// Matching directorapi.ClipboardState, which already treats the clipboard as
		// sensitive: it routinely holds whatever the user last copied, and that is
		// exactly as likely to be a password as a URL.
		base = VisibilitySensitive
	}
	if base == VisibilityNormal && emailish.MatchString(strings.TrimSpace(text)) {
		return VisibilitySensitive
	}
	return base
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
