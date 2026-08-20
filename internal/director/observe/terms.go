package observe

import (
	"sort"
	"strings"
)

// Reading the words an interface shows, without keeping the words.
//
// # Why a closed vocabulary and not the text
//
// Text on screen is the strongest semantic evidence available: four aligned buttons could be
// anything, and four aligned buttons under the word SETTINGS are a settings screen in any
// application ever written. But an OCR pass over a game window also reads chat, player names,
// item names and whatever a friend just typed — and a hypothesis record is durable, inspectable
// and eventually shareable.
//
// So the same trade the navigation producer makes is made here: classify to MEANING at the
// boundary and discard the rest. What crosses into the discovery path is a term from the closed
// vocabulary below, never the label it came from. A username matches nothing in this table and
// therefore cannot become semantic evidence — not by policy, but because there is nowhere to
// put it. See [[ADR-013-navigation-is-meaning-not-keys]] for the same argument about keys.
//
// # Why this is not a game dictionary
//
// Every term below names a concept that ordinary interfaces have: settings, audio, search,
// back. None of them names a game, a game's menu, or a game's layout. The test to apply when
// adding one is whether a word processor could plausibly have it — `settings` passes, `boost`
// and `stealth` do not, and neither does any phrase that only makes sense in one title.
//
// A term is EVIDENCE, never an identity. "The word SETTINGS was observed in 4 of the 4
// inferences this screen was active" is a measurement; that the screen IS settings is a
// hypothesis, formed elsewhere, with contradictions attached.

// InterfaceTerm is a generic interface concept observed as text.
type InterfaceTerm string

const (
	TermSettings      InterfaceTerm = "settings"
	TermControls      InterfaceTerm = "controls"
	TermAudio         InterfaceTerm = "audio"
	TermDisplay       InterfaceTerm = "display"
	TermSensitivity   InterfaceTerm = "sensitivity"
	TermLanguage      InterfaceTerm = "language"
	TermNotifications InterfaceTerm = "notifications"
	TermAccount       InterfaceTerm = "account"
	TermSocial        InterfaceTerm = "social"
	TermInvite        InterfaceTerm = "invite"
	TermSearch        InterfaceTerm = "search"
	TermResume        InterfaceTerm = "resume"
	TermQuit          InterfaceTerm = "quit"
	TermBack          InterfaceTerm = "back"
	TermApply         InterfaceTerm = "apply"
	TermCancel        InterfaceTerm = "cancel"
	TermHelp          InterfaceTerm = "help"
)

// vocabulary maps a single WORD to the concept it evidences.
//
// Word-level and exact. Substring matching is what turns "backpack" into `back` and
// "researcher" into `search`, and a false term is worse than a missing one because it becomes
// support for a hypothesis nobody can trace back to a mistake.
//
// Synonyms collapse into one concept deliberately: `controller`, `bindings` and `keybinds` are
// the same interface idea under three house styles, and a hypothesis should not be weaker
// because an application chose the third.
var vocabulary = map[string]InterfaceTerm{
	"settings": TermSettings, "options": TermSettings, "preferences": TermSettings,
	"configuration": TermSettings, "config": TermSettings,

	"controls": TermControls, "controller": TermControls, "gamepad": TermControls,
	"bindings": TermControls, "keybinds": TermControls, "keybindings": TermControls,
	"input": TermControls, "keyboard": TermControls, "mouse": TermControls,

	"audio": TermAudio, "sound": TermAudio, "volume": TermAudio, "music": TermAudio,

	"display": TermDisplay, "video": TermDisplay, "graphics": TermDisplay,
	"resolution": TermDisplay, "brightness": TermDisplay,

	"sensitivity": TermSensitivity, "deadzone": TermSensitivity,

	"language": TermLanguage, "locale": TermLanguage,

	"notifications": TermNotifications, "alerts": TermNotifications,

	"account": TermAccount, "profile": TermAccount, "sign": TermAccount,
	"login": TermAccount, "logout": TermAccount,

	"friends": TermSocial, "party": TermSocial, "social": TermSocial,
	"players": TermSocial, "roster": TermSocial,

	"invite": TermInvite, "invitation": TermInvite,

	"search": TermSearch, "find": TermSearch, "filter": TermSearch,

	"resume": TermResume, "continue": TermResume, "unpause": TermResume,

	"quit": TermQuit, "exit": TermQuit, "disconnect": TermQuit,

	"back": TermBack, "return": TermBack,

	"apply": TermApply, "save": TermApply, "confirm": TermApply, "accept": TermApply,

	"cancel": TermCancel, "discard": TermCancel,

	"help": TermHelp, "tutorial": TermHelp, "support": TermHelp,
}

// Known reports whether a term is in the closed vocabulary.
//
// Everything that enters the discovery path is checked against this, for the same reason
// NavIntent is: the underlying type is a string, so admission rather than typing is what keeps
// arbitrary text out.
func (t InterfaceTerm) Known() bool { _, ok := knownTerms[t]; return ok }

var knownTerms = func() map[InterfaceTerm]bool {
	out := map[InterfaceTerm]bool{}
	for _, term := range vocabulary {
		out[term] = true
	}
	return out
}()

// InterfaceTerms returns the vocabulary, sorted.
func InterfaceTerms() []InterfaceTerm {
	out := make([]InterfaceTerm, 0, len(knownTerms))
	for t := range knownTerms {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// SemanticEvidence is what one inference's readable interface said about itself.
//
// Counts and closed-vocabulary terms. No text, no labels, no digests — nothing here can be
// used to reconstruct what was on screen, which is what makes it safe to keep for the life of a
// hypothesis.
type SemanticEvidence struct {
	// Terms are the generic concepts observed, deduplicated within the inference.
	//
	// Deduplicated on purpose: a settings screen with eight rows all containing the word
	// "audio" is one piece of evidence that the screen concerns audio, not eight.
	Terms []InterfaceTerm `json:"terms,omitempty"`
	// PlaceName is what the Place appears to be CALLED, when one Actor.s evidence says so
	// unambiguously. Admitted through AdmittedPlaceName under the demonstration licence;
	// empty whenever the evidence disagrees, is absent, or nobody asked Marco to learn.
	//
	// A word, never a count and never a shape. See
	// [[ADR-076-a-place-may-say-what-it-appears-to-be-called]].
	PlaceName string `json:"place_name,omitempty"`
	// EditableFields counts controls perception reported as text-editable.
	//
	// A count, never the contents. This is the only honest basis for a text-entry
	// hypothesis: geometry cannot distinguish a search box from a label with a border, and
	// guessing from shape is exactly the failure this whole layer is arranged to avoid.
	EditableFields int `json:"editable_fields,omitempty"`
	// Observed says perception actually had interface text to classify this inference.
	//
	// # The distinction this exists for
	//
	// "Marco looked and found no interface concepts" and "Marco could not look" are
	// different facts, and only the first is evidence. Without this field they are the
	// same value — an empty Terms slice — and two things go wrong:
	//
	//   - Cross-session matching reads a remembered subject's terms against an empty
	//     current set and concludes the screen DIFFERS, when all that happened is that
	//     OCR was unavailable. A missing reading becomes a positive disagreement.
	//   - The per-state term ratio divides by every inference, including the ones that
	//     never had text to read. Scoped OCR runs on roughly one inference in six, so a
	//     term present on EVERY reading scores about 0.17 against a 0.50 threshold and
	//     can never qualify. The discriminator is unreachable in a live session.
	//
	// Both were live defects, found by auditing this path rather than by a failing test —
	// which is why the fix is a representation change rather than a threshold change.
	Observed bool `json:"observed,omitempty"`
}

// Empty reports whether this inference offered no semantic evidence at all.
func (s SemanticEvidence) Empty() bool {
	return !s.Observed && len(s.Terms) == 0 && s.EditableFields == 0
}

// SemanticEvidenceFrom reads one sample's entities into closed-vocabulary evidence.
//
// The raw label is read here and kept nowhere. This function is the boundary: everything
// upstream of it may hold text, and nothing downstream of it can.
// Observed is set when at least one entity carried a label the classifier released in the
// clear. That is exactly "there was interface text to read": if a screen showed readable text
// and no concept matched, the empty result is a real finding; if nothing readable reached this
// function at all, the answer is UNKNOWN and must not be reported as an absence of concepts.
//
// Deriving it from the entities rather than from a flag the caller passes is deliberate. It
// covers both sources without either side having to remember: accessibility supplies labels on
// every sample, scoped OCR only on a label pass, and a screen exposing neither yields unknown by
// construction rather than by a caller getting an argument right.
func SemanticEvidenceFrom(entities []EntitySnapshot) SemanticEvidence {
	var out SemanticEvidence
	seen := map[InterfaceTerm]bool{}
	for _, e := range entities {
		if e.Role.TextEditable() {
			out.EditableFields++
		}
		// Only labels the privacy classifier released in the clear are read. A redacted
		// label is not consulted, not matched, and not counted — the structural allowlist
		// that decided to withhold it is not overridable from here.
		if e.Label.Redacted || e.Label.Text == "" {
			continue
		}
		out.Observed = true
		for _, term := range termsIn(e.Label.Text) {
			if seen[term] {
				continue
			}
			seen[term] = true
			out.Terms = append(out.Terms, term)
		}
	}
	// Sorted so a fixture replays identically and two sessions can be compared.
	sort.Slice(out.Terms, func(i, j int) bool { return out.Terms[i] < out.Terms[j] })
	return out
}

// ScreenTextEvidence classifies text read for the SCREEN rather than for any control.
//
// # Why this exists beside SemanticEvidenceFrom
//
// A detector on a surface with no accessibility finds a heading — the words CONTROLLER
// SETTINGS above four boxes — and those words are what make the four boxes mean something.
// They are also the single most dangerous text on the screen to keep: a text region on a
// desktop is where a chat line, a document title and a person's name live, and unlike a
// button's name there is no structural claim that it describes the interface.
//
// So a text region takes the same vocabulary and a stricter deal. A control's name may be
// released in the clear as a SESSION-LOCAL safe label (see Classify) because a structural role
// vouches for it; a text region's words may only ever become closed-vocabulary terms, and the
// string is gone by the time this function returns. There is no path from here to plaintext,
// which is why the caller may hand it arbitrary screen text without having to be careful.
//
// Observed is set when there was anything at all to read, matched or not — "Marco looked at
// the heading and recognised no concept" is a fact about the screen; "Marco never read it" is
// not, and the two must not be the same empty set. See SemanticEvidence.Observed.
func ScreenTextEvidence(texts []string) SemanticEvidence {
	var out SemanticEvidence
	seen := map[InterfaceTerm]bool{}
	for _, text := range texts {
		if strings.TrimSpace(text) == "" {
			continue
		}
		out.Observed = true
		for _, term := range termsIn(text) {
			if seen[term] {
				continue
			}
			seen[term] = true
			out.Terms = append(out.Terms, term)
		}
	}
	sort.Slice(out.Terms, func(i, j int) bool { return out.Terms[i] < out.Terms[j] })
	return out
}

// Merge folds another reading of the same inference into this one.
//
// Two sources may describe one screen — accessibility naming controls, and a detector's boxes
// read by scoped OCR — and neither is complete. The union is the honest answer: a term is
// evidence that a concept was present, and it was no less present for having been found by the
// other source.
//
// Observed is OR-ed, deliberately. If either source had text to classify then Marco looked,
// and an unavailable second source must not turn a real reading back into UNKNOWN.
// EditableFields takes the larger: it is a count of what is on screen, not a sum over sources
// that would double the same text box.
func (s SemanticEvidence) Merge(other SemanticEvidence) SemanticEvidence {
	out := SemanticEvidence{
		Observed:       s.Observed || other.Observed,
		EditableFields: s.EditableFields,
	}
	if other.EditableFields > out.EditableFields {
		out.EditableFields = other.EditableFields
	}
	seen := map[InterfaceTerm]bool{}
	for _, t := range append(append([]InterfaceTerm{}, s.Terms...), other.Terms...) {
		if seen[t] {
			continue
		}
		seen[t] = true
		out.Terms = append(out.Terms, t)
	}
	sort.Slice(out.Terms, func(i, j int) bool { return out.Terms[i] < out.Terms[j] })
	// TWO ACTORS AGREEING IS ONE NAME. TWO DISAGREEING IS NONE.
	//
	// Merging is where Actor evidence meets, so it is where disagreement has to be settled
	// honestly rather than by whichever ran last. Accessibility saying "Mouse" while OCR says
	// "Bluetooth & devices" is Marco not knowing, and silence is the correct report.
	//
	// Deleting this must fail TestActorsDisagreeingProduceNoName.
	switch {
	case other.PlaceName == "", s.PlaceName == other.PlaceName:
		out.PlaceName = s.PlaceName
	case s.PlaceName == "":
		out.PlaceName = other.PlaceName
	}
	return out
}

// termsIn matches one label's WORDS against the vocabulary.
func termsIn(label string) []InterfaceTerm {
	var out []InterfaceTerm
	for _, word := range strings.FieldsFunc(strings.ToLower(label), notLetter) {
		if term, ok := vocabulary[word]; ok {
			out = append(out, term)
		}
	}
	return out
}

// notLetter splits on everything that is not a letter, so "AUDIO/VIDEO", "Audio:" and
// "audio_settings" all yield the words they contain.
func notLetter(r rune) bool {
	return !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z')
}

// admissibleTerms filters evidence down to the closed vocabulary and bounds it.
func admissibleTerms(in SemanticEvidence) SemanticEvidence {
	// REBUILT, so every field has to be carried deliberately.
	//
	// That is the point — nothing reaches a screen state by default — and it is also the trap:
	// `PlaceName` was added upstream, passed its own tests, and arrived here to be silently
	// dropped by a constructor that only knew about terms. The chain looked connected from
	// both ends.
	//
	// The name needs no filtering of its own: AdmittedPlaceName already applied the licence
	// and the shape filter before it was ever put on the evidence.
	//
	// Deleting `PlaceName` here must fail TestAPlaceEstablishedThroughTheProductionPassIsNamed.
	out := SemanticEvidence{
		EditableFields: in.EditableFields, Observed: in.Observed, PlaceName: in.PlaceName,
	}
	for _, t := range in.Terms {
		if !t.Known() {
			continue
		}
		out.Terms = append(out.Terms, t)
		if len(out.Terms) >= MaxTermsPerInference {
			break
		}
	}
	return out
}

// MaxTermsPerInference bounds one inference's semantic evidence.
const MaxTermsPerInference = 12
