package observe

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// What a session keeps, and what it refuses to.
//
// A game window shows the player's account name, their party, their friends list, whatever
// Spotify is playing, and any notification Windows feels like putting on top. A session
// that stored "the text that was visible" would be storing all of it, and would then write
// it to disk under a filename the person never looks at.
//
// So plaintext is the exception. A label is kept in the clear only when it looks like the
// name of a control — short, wordy, no digits-and-punctuation soup — and everything else is
// reduced to a role, a length and a digest. A digest is enough to notice that a label
// CHANGED, which is what temporal analysis actually needs; the words themselves are almost
// never the point.

// Digest is a stable, non-reversible identifier for a piece of evidence.
//
// Short on purpose: 12 hex characters is enough to tell two labels apart across a few
// hundred samples and far too little to attack. It exists so "this changed" can be observed
// without "this said" being retained.
type Digest string

// digestOf reduces text to a digest.
func digestOf(parts ...string) Digest {
	h := sha256.New()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return Digest(hex.EncodeToString(h.Sum(nil))[:12])
}

// SafeLabel is a label as a session is willing to keep it.
//
// Either Text is set (classified safe) or it is empty and only the digest, length and
// confidence survive. Both carry the same Digest, so a label that becomes unreadable, or
// becomes private, still compares correctly against its earlier self.
type SafeLabel struct {
	// Text is the plaintext, present only when the label classified as safe.
	Text string `json:"text,omitempty"`
	// Digest identifies the label whether or not the text was kept.
	Digest Digest `json:"digest"`
	// Length is the original character count, kept because a label growing from 4 to 40
	// characters is a real observation even when neither is readable.
	Length int `json:"length"`
	// Confidence is what the reader believed.
	Confidence float64 `json:"confidence"`
	// Redacted says the text was deliberately withheld, so a reader can tell it apart
	// from a control that simply had no name.
	Redacted bool `json:"redacted,omitempty"`
}

// Empty reports whether there was no label at all.
func (l SafeLabel) Empty() bool { return l.Digest == "" }

// Describe renders a label for a person.
func (l SafeLabel) Describe() string {
	switch {
	case l.Empty():
		return "(unnamed)"
	case l.Text != "":
		return fmt.Sprintf("%q", l.Text)
	default:
		return fmt.Sprintf("(withheld, %d chars, %s)", l.Length, l.Digest)
	}
}

// LabelPolicy decides which labels may be kept in the clear.
type LabelPolicy struct {
	// MaxWords and MaxLength bound what a control's name looks like. A sentence is not
	// a button, and a paragraph that landed inside a box is certainly not one.
	MaxWords  int
	MaxLength int
	// MinConfidence is the floor below which nothing is kept in the clear, because a
	// reading nobody believes is not worth the privacy cost of storing.
	MinConfidence float64
}

// DefaultLabelPolicy is the conservative default.
func DefaultLabelPolicy() LabelPolicy {
	return LabelPolicy{MaxWords: 5, MaxLength: 40, MinConfidence: 0.6}
}

// privateMarkers are shapes that mean "this is about a person".
//
// Not an attempt at a complete list, and it does not need to be: this is the SECOND line.
// The first is that plaintext requires a positive classification, so anything unusual is
// withheld by default and these merely catch things that would otherwise look ordinary.
var privateMarkers = []string{
	"@", "#", "http://", "https://", "www.", ".com", ".net", ".gg",
	"friend", "party", "invite", "chat", "whisper", "clan", "guild",
	"logged in", "signed in", "account", "profile", "steam", "epic", "discord",
	"playing", "listening", "spotify", "now playing",
}

// LabelContext is what is known about the thing a label belongs to.
//
// The FIRST gate, and the important one. What decides whether text may be kept in the clear
// is not what the text looks like but what it is attached to: the name written on a button
// is a fact about the interface, and text that happens to sit inside a picture is a fact
// about the person using it.
type LabelContext struct {
	// Role is the element's semantic kind.
	Role directorapi.ElementRole
	// Sources names the providers that contributed to the element.
	Sources []string
}

// eligible reports whether a role's name may be kept in the clear.
//
// ONE policy, and it is not this package's. `directorapi.ElementRole.NameablePlaintext` is the
// canonical allowlist, shared with the scoped label reader, the shadow diagnostic and the
// vision benchmark's nameable-role coverage — because four copies of an allowlist is four
// places to widen it, and only one of them looks like a privacy decision from the outside.
func eligible(role directorapi.ElementRole) bool { return role.NameablePlaintext() }

// Classify decides how a label may be stored.
//
// Two stages, and the order matters:
//
//  1. STRUCTURAL ELIGIBILITY. Is this the name of a control? If not, nothing else is
//     considered and the text is withheld. This is a property of the element, not of the
//     string, which is what makes it robust against text nobody anticipated.
//  2. SHAPE. Even a control's name is refused if it looks like an identifier, a token or a
//     tagged handle — defence in depth, not the primary defence.
//
// The default is privacy. An unrecognised role, an unknown role, or no context at all
// yields a digest and nothing more.
//
// # What this costs
//
// It is not free. The detector currently in use emits a single class that maps to RoleIcon,
// so a game's menu labels are now withheld too — the same "RESUME GAME" that reads perfectly
// is stored as a digest, because nothing structural vouches for it being a button. That is
// the honest trade: a classifier that could keep those would also keep a friends list. The
// remedy is a detector with a real class vocabulary, not a looser rule here.
func Classify(text string, confidence float64, ctx LabelContext, p LabelPolicy) SafeLabel {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return SafeLabel{}
	}
	label := SafeLabel{
		Digest:     digestOf(strings.ToLower(trimmed)),
		Length:     len([]rune(trimmed)),
		Confidence: confidence,
		Redacted:   true,
	}
	if !eligible(ctx.Role) {
		return label
	}
	if safeLabelText(trimmed, confidence, p) {
		label.Text = trimmed
		label.Redacted = false
	}
	return label
}

// AdmittedTargetLabel decides whether a control's name may travel on a semantic TARGET —
// the enrichment attached to an input event the person themselves made.
//
// The same two-stage classifier Classify applies, sharing its shape filter verbatim, with
// one deliberately wider first stage: during an explicit Learn demonstration, a role a
// person can ACTIVATE — a list item, a tree item, a link — may carry its name too, for the
// one control their own input landed on.
//
// # Why the demonstration licence is sound where a general widening is not
//
// The plaintext allowlist is conservative because a list item's text is very often a fact
// about the person — a friend, a file, a message. What changes under a demonstration is not
// the text but the PROVENANCE of the question: the person explicitly asked Marco to learn
// this and then activated that control themselves. Retaining the name of the one thing they
// aimed at is the same shape as [[ADR-047]] — an explicit human semantic event licenses
// persisting the one identity it is about — and it reaches nothing else on the screen: the
// gate admits only what an event's own resolution touched, never a sweep.
//
// The shape filter is unchanged and unconditional, so a friend tag, a token or a filename
// with an extension is refused whatever the licence says. One policy site, beside the
// classifier it extends; a second copy elsewhere is a privacy decision nobody reviewed.
func AdmittedTargetLabel(role directorapi.ElementRole, demonstration bool, text string,
	confidence float64) string {

	trimmed := strings.TrimSpace(text)
	if trimmed == "" || len([]rune(trimmed)) > MaxTargetLabelLength {
		return ""
	}
	if !eligible(role) && !(demonstration && role.Clickable()) {
		return ""
	}
	if !safeLabelText(trimmed, confidence, DefaultLabelPolicy()) {
		return ""
	}
	return trimmed
}

// safeLabelText reports whether text looks like the name of a control and nothing else.
func safeLabelText(s string, confidence float64, p LabelPolicy) bool {
	if confidence < p.MinConfidence {
		return false
	}
	if len([]rune(s)) > p.MaxLength {
		return false
	}
	if len(strings.Fields(s)) > p.MaxWords {
		return false
	}
	lower := strings.ToLower(s)
	for _, marker := range privateMarkers {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	// Mostly letters. A control's name is words; an account tag is brackets, digits and
	// punctuation, and a server identifier is more of the same.
	letters, meaningful := 0, 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		meaningful++
		if unicode.IsLetter(r) {
			letters++
		}
	}
	if meaningful == 0 {
		return false
	}
	if float64(letters)/float64(meaningful) < 0.7 {
		return false
	}
	// Brackets and pipes are how games mark clans, parties and tags.
	if strings.ContainsAny(s, "[]{}<>|\\/*") {
		return false
	}

	// Word shape. Added because the whole-string ratio above happily accepted
	// "TTVX-FINAL-SECRET-6b8d-XYZZYPLOV": one token, 84% letters, under the length cap,
	// matching no marker. Nothing about that is a control's name, and the thing that
	// gives it away is the SHAPE OF ITS WORDS rather than the mix of its characters.
	//
	// Real labels are ordinary words: "RESUME", "SETTINGS", "MODE". Identifiers, tokens,
	// session keys and server names are long unbroken runs, or mix letters and digits
	// inside a single word. Both are refused.
	for _, word := range strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == '-' || r == '_'
	}) {
		if len([]rune(word)) > maxLabelWordLength {
			return false
		}
		hasLetter, hasDigit := false, false
		for _, r := range word {
			if unicode.IsLetter(r) {
				hasLetter = true
			}
			if unicode.IsDigit(r) {
				hasDigit = true
			}
		}
		if hasLetter && hasDigit {
			return false
		}
	}
	return true
}

// maxLabelWordLength is the longest single word a control's name may contain.
//
// "Antidisestablishmentarianism" is 28 characters and would fail; no button says that. The
// things this length actually excludes are identifiers and tokens.
const maxLabelWordLength = 15

// Region is where something is, relative to the window rather than the desktop.
//
// Relative on purpose. Desktop coordinates change when a window moves, so comparing them
// across samples would report that everything moved whenever the player dragged the window;
// and a desktop coordinate is also a small fact about the person's monitor layout.
type Region struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// relativeTo converts desktop bounds into window-relative ones.
func relativeTo(box, window directorapi.Rect) Region {
	if window.Width <= 0 || window.Height <= 0 {
		return Region{}
	}
	return Region{
		X:      float64(box.X-window.X) / float64(window.Width),
		Y:      float64(box.Y-window.Y) / float64(window.Height),
		Width:  float64(box.Width) / float64(window.Width),
		Height: float64(box.Height) / float64(window.Height),
	}
}

// nearlyEqual reports whether two regions are the same within tolerance.
//
// Jitter is not movement. A detector's box wanders a pixel or two between frames, and a
// session that called that a transition would report hundreds of them per minute and bury
// the handful that mattered.
func (r Region) nearlyEqual(other Region, tolerance float64) bool {
	return absF(r.X-other.X) <= tolerance &&
		absF(r.Y-other.Y) <= tolerance &&
		absF(r.Width-other.Width) <= tolerance &&
		absF(r.Height-other.Height) <= tolerance
}

func absF(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// EntitySnapshot is one thing seen in one sample, as a session keeps it.
//
// Carries no ElementID, no provider-native id, no handle and no desktop coordinate. Those
// are all either unstable across samples or a detail about the machine rather than about
// what was on screen.
type EntitySnapshot struct {
	// Identity is what this package uses to recognise the same thing again. Derived
	// from role, label and structural position — never from coordinates alone.
	Identity Digest `json:"identity"`
	// Role is the semantic kind: button, text, group, icon and so on.
	Role directorapi.ElementRole `json:"role"`
	// Label is the name, kept safely.
	Label SafeLabel `json:"label"`
	// Confidence is fusion's belief in this element.
	Confidence float64 `json:"confidence"`
	// Sources names the providers that contributed, so a reader can see whether
	// something was structural or only seen.
	Sources []string `json:"sources,omitempty"`
	// Region is where it sat, relative to the window.
	Region Region `json:"region"`
	// Enabled and Focused are state flags, kept because they change.
	Enabled bool `json:"enabled"`
	Focused bool `json:"focused,omitempty"`
	// Actionable says whether policy would let the Director act on it. Recorded as an
	// OBSERVATION about the evidence; this package never acts on anything.
	Actionable bool `json:"actionable"`
	// Chrome says this belongs to the WINDOW rather than to the page — a title bar, a
	// scroll bar, or anything inside one. Classified from the accessibility hierarchy while
	// it is still available; a region carries no parent, so identity could never work it
	// out downstream.
	//
	// A label, not a removal. The entity is still observed, still actionable and still
	// shown; exactly one consumer reads this, and it is the durable place signature.
	Chrome bool `json:"chrome,omitempty"`
	// GridPosition is set for cells of a detected grid.
	GridPosition string `json:"grid_position,omitempty"`
}

// identityOf derives a stable identity for an element.
//
// Role, label and grid position — the things that stay the same when a window moves. A
// coordinate appears only as a coarse quadrant, and only to separate two identically named
// controls in different parts of a screen; using precise position would make every element
// a new element as soon as anything reflowed.
func identityOf(role directorapi.ElementRole, label SafeLabel, gridPosition string, r Region) Digest {
	return digestOf(
		string(role),
		string(label.Digest),
		gridPosition,
		quadrant(r),
	)
}

// quadrant reduces a region to a coarse ninth of the window.
func quadrant(r Region) string {
	col := int(r.X * 3)
	row := int(r.Y * 3)
	return fmt.Sprintf("%d%d", clampQ(row), clampQ(col))
}

func clampQ(v int) int {
	switch {
	case v < 0:
		return 0
	case v > 2:
		return 2
	}
	return v
}

// GridSnapshot is a detected grid, as a session keeps it.
type GridSnapshot struct {
	Identity Digest  `json:"identity"`
	Rows     int     `json:"rows"`
	Columns  int     `json:"columns"`
	Cells    int     `json:"cells"`
	Region   Region  `json:"region"`
	Fill     float64 `json:"fill"`
}

// FrameSummary is what a sample's capture cost and covered.
type FrameSummary struct {
	Application      string        `json:"application"`
	WindowGeneration uint64        `json:"window_generation"`
	Width            int           `json:"width"`
	Height           int           `json:"height"`
	CaptureDuration  time.Duration `json:"capture_duration"`
	// Skipped counts work deliberately not done — regions too small to read, or past a
	// budget. Reported because a skipped region must never read as an empty one.
	Skipped int `json:"skipped,omitempty"`
}

// ProviderSummary is what each source contributed to one sample.
type ProviderSummary struct {
	Name         string        `json:"name"`
	Observations int           `json:"observations"`
	Duration     time.Duration `json:"duration"`
	Failed       bool          `json:"failed,omitempty"`
	Reason       string        `json:"reason,omitempty"`

	// State is what the provider concluded about the target, in the outcome vocabulary.
	// Empty for a provider that reported no outcome.
	State string `json:"state,omitempty"`
	// Expected and Observed are the two window generations the guard compared: what the
	// Director intended this provider to observe, and what the provider could prove it
	// did. BOTH are reported, because a verdict with only one number in view is a verdict
	// a reader has to take on faith.
	Expected uint64 `json:"expected_generation,omitempty"`
	Observed uint64 `json:"observed_generation,omitempty"`
	// Proven is the guard's verdict. False on a target-scoped provider means its evidence
	// did not enter belief, whatever its observation count says.
	Proven bool `json:"proven,omitempty"`
	// Global marks evidence that makes no window claim and is exempt by construction.
	Global bool `json:"global,omitempty"`
	// Quarantined is how many observations this provider produced that were refused —
	// collected, retained for diagnostics, and never believed. Reported apart from
	// Observations, which counts only what was ADMITTED.
	Quarantined int `json:"quarantined,omitempty"`
}

// Phase timings for one sample, so "observation is slow" can be attributed.
type Phases struct {
	Validate  time.Duration `json:"validate"`
	Capture   time.Duration `json:"capture"`
	Detect    time.Duration `json:"detect"`
	Labels    time.Duration `json:"labels"`
	Fuse      time.Duration `json:"fuse"`
	Snapshot  time.Duration `json:"snapshot"`
	Analyse   time.Duration `json:"analyse"`
	Total     time.Duration `json:"total"`
	LabelsRun int           `json:"labels_run"`
	LabelsCap int           `json:"labels_capped,omitempty"`
}

// Sample is one bounded observation.
type Sample struct {
	Sequence         int               `json:"sequence"`
	Timestamp        time.Time         `json:"timestamp"`
	WindowGeneration uint64            `json:"window_generation"`
	Frame            FrameSummary      `json:"frame"`
	Providers        []ProviderSummary `json:"providers,omitempty"`
	Entities         []EntitySnapshot  `json:"entities,omitempty"`
	Grids            []GridSnapshot    `json:"grids,omitempty"`
	Phases           Phases            `json:"phases"`
	// Digest summarises the whole sample, so two identical scenes compare in one step.
	Digest Digest `json:"digest"`
	// Shadow is what an EXPERIMENTAL provider found this cycle, when one was running.
	//
	// Nil for an ordinary session. It rides here rather than on its own channel because a
	// separate channel is exactly what went unconsumed last time: a Sample already flows
	// from the sampler through the session accumulator, the terminal Result, the service
	// protocol and the CLI, so anything carried on it cannot be dropped in isolation.
	//
	// Reporting only. Nothing here is evidence and nothing here reaches belief.
	Shadow *ShadowSample `json:"shadow,omitempty"`

	// Structure is the AUTHORITATIVE composition this frame presented — the fused world's
	// structural elements, in window-relative geometry — and whether anything looked.
	//
	// Set by the composition root, which is the only place a desktop rectangle and a window
	// frame are both in scope. Unobserved on a sample from a source that produces no fused
	// world; `StructureOf` then falls back to the detector, which is the surface the
	// detector exists for.
	//
	// It carries no labels, no text and no identities: a screen is a composition, and
	// segmentation reads role and region and nothing else.
	Structure StructuralView `json:"structure,omitzero"`
}

// digest computes a sample's overall digest from its entity identities.
func (s *Sample) computeDigest() {
	parts := make([]string, 0, len(s.Entities)+len(s.Grids))
	for _, e := range s.Entities {
		parts = append(parts, string(e.Identity))
	}
	for _, g := range s.Grids {
		parts = append(parts, string(g.Identity))
	}
	s.Digest = digestOf(parts...)
}

// RelativeTo converts desktop bounds into window-relative ones.
//
// Exported for the snapshot adapter at the composition root, which is the only place a
// desktop rectangle and a window frame are both in scope. Everything downstream of it works
// in relative geometry, so this is the one conversion point.
func RelativeTo(box, window directorapi.Rect) Region { return relativeTo(box, window) }

// IdentityOf derives a stable identity for an element.
//
// Exported for the same reason as RelativeTo. Kept as one function rather than left to each
// caller, because two callers deriving identity differently would silently split one thing
// into two across a timeline.
func IdentityOf(role directorapi.ElementRole, label SafeLabel, gridPosition string, r Region) Digest {
	return identityOf(role, label, gridPosition, r)
}

// ── what a Place appears to be called ─────────────────────────────────────────

// PlaceNameEvidence is one element's claim about what the Place is called.
//
// Provider-neutral by construction: a role, a word, how sure the Actor was, and two facts about
// where it sits. No runtime id, no handle, no rectangle — see [[ADR-076]] for why a Place's name
// may never depend on any of those.
type PlaceNameEvidence struct {
	Role  directorapi.ElementRole
	Label string
	// Confidence is the Actor's own, and it meets the same bar a target label meets.
	Confidence float64
	// Selected says the Actor reports this as the chosen one of its siblings.
	Selected bool
	// InsideValueChooser says an ancestor is a control for PICKING A VALUE rather than for
	// going somewhere — a combo box, a spinner. Measured live: Settings Home reports two
	// selected items, `Home` in the navigation pane and `Dark` inside the Color-mode combo
	// box. One says where you are; the other says what a setting is set to.
	InsideValueChooser bool
	// Trail is the navigation trail this word appears in, when it appears in one.
	//
	// # Section is not Place
	//
	// A selected navigation item names the SECTION, and on a sub-page that is not where you
	// are. Measured on the Settings Mouse page: the rail reports `Bluetooth & devices`
	// selected, and two sibling buttons under one parent read `Bluetooth & devices` and
	// `Mouse`. On Home the same parent holds one button, `Home`.
	//
	// The trail is self-identifying: it is the set of sibling labels CONTAINING the selected
	// item.s word. No geometry, no ordering, and no application named anywhere.
	Trail []string
}

// AdmittedPlaceName is the one word a Place may be called, from what an Actor perceived.
//
// # Measured, not invented
//
// Five live applications were dumped before this rule existed:
//
//	Settings   title "Settings" on every page      selected list_item "Home"        ← only signal
//	Chrome     title "… - Google Chrome"           selected tab "Marco - Marco Director"
//	Discord    title "@BeeTeaSea - Discord"        selected tree_item "Direct Messages"
//	VS Code    title "… - Visual Studio Code"      THREE selected items
//	Spotify    title "Trojans • Atlas Genius"      none
//
// The window title is not the signal. In most applications it is `<place> - <app>`, so using it
// means stripping an application suffix — which is exactly the application-baking a Place name
// must not do — and in Settings it is the application name on every page, discriminating nothing.
//
// The SELECTED navigable item is the signal. It is short, noun-like, and it is what actually
// changes as somebody walks around.
//
// # Why silence is a legitimate answer
//
// VS Code offers three selections at once and Spotify none. Ranking them would have named VS Code
// `Explorer (Ctrl+Shift+E)` — a keyboard hint, not a place. So exactly one admissible candidate
// names the Place; zero or several name nothing, and the structural description carries that one
// Place. A missing name costs a line of diagnostics. A confident wrong one is trusted.
//
// # The licence
//
// The same one a semantic target rides: an explicit Learn demonstration. A Place's name is read
// off somebody's screen, and passive observation has no business writing that down — see
// AdmittedTargetLabel for the argument, which is the same argument. The shape filter is
// unconditional either way, so a friend tag or a filename is refused whatever the licence says.
//
// One policy site, beside the one it extends. A second copy elsewhere is a privacy decision
// nobody reviewed.
//
// Deleting the single-candidate rule must fail TestSeveralSelectedItemsNameNothing.
func AdmittedPlaceName(evidence []PlaceNameEvidence, demonstration bool) string {
	if !demonstration {
		return ""
	}
	found := ""
	for _, e := range evidence {
		if !e.Selected || e.InsideValueChooser {
			continue
		}
		// A DESTINATION, not a control. A button is something you press; it never reports
		// itself as the selected one of its siblings, which is why `Back` and `Close`
		// cannot reach this rule at all — but the role check says so rather than relying
		// on that.
		if !e.Role.Navigable() {
			continue
		}
		trimmed := strings.TrimSpace(e.Label)
		if trimmed == "" || len([]rune(trimmed)) > MaxTargetLabelLength {
			continue
		}
		if !safeLabelText(trimmed, e.Confidence, DefaultLabelPolicy()) {
			continue
		}
		// THE SECTION IS NOT THE PLACE.
		//
		// A selected navigation item names the section, and on a sub-page that is not
		// where you are. When this word appears in a trail, the trail says which.
		//
		// One entry: the section IS the page — the section root, and they are legitimately
		// equal. Two: the other entry is the page. More than two is a deeper trail than
		// anything measured, and guessing which entry is the page there would be inventing
		// a rule; no name is the honest answer.
		//
		// Order is never read, because the fused world is a map and has none.
		//
		// Deleting the trail branch must fail TestASubPageIsNotNamedAfterItsSection.
		word := trimmed
		if len(e.Trail) > 0 {
			other := ""
			for _, t := range e.Trail {
				if s := strings.TrimSpace(t); s != "" && s != trimmed {
					if other != "" {
						return "" // three or more: unmeasured, fail closed
					}
					other = s
				}
			}
			if other != "" {
				if !safeLabelText(other, e.Confidence, DefaultLabelPolicy()) {
					return ""
				}
				word = other
			}
		}
		if found != "" && found != word {
			// Two different answers. Marco does not know which, and saying so is the
			// honest report.
			return ""
		}
		found = word
	}
	return found
}
