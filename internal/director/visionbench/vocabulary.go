package visionbench

import (
	"sort"
	"strings"
)

// The closed vocabulary an open-vocabulary detector is allowed to speak.
//
// A model like Grounding DINO will find whatever you name, and will name whatever you ask
// about. That is its strength and precisely its danger: asked an open question it returns
// confident prose, and a pipeline that turned prose into roles would let a model invent
// parts of the Director's own semantic vocabulary at run time.
//
// So the prompt is built from a fixed list, the reply is matched back against that list, and
// anything that does not match becomes UNKNOWN. A model cannot create a role here; it can
// only choose from ones that already exist, or fail to.
//
// # Versioned, because results must stay comparable
//
// Changing a prompt changes what a model finds. Two benchmark runs under different
// vocabularies are measuring different questions, and a report that did not say which would
// invite exactly the wrong comparison. Every report carries the version.

// VocabularyVersion identifies the prompt set a benchmark ran under.
//
// Bump it for ANY change to Terms or Synonyms. The digest in a report is what lets somebody
// six months later know whether two numbers are comparable.
const VocabularyVersion = "game-ui-v1"

// Terms are the phrases put to an open-vocabulary detector.
//
// Generic interface words, no game in any of them. A prompt naming "boost meter" would make
// the model find boost meters in wallpaper, and would make the benchmark a test of Rocket
// League rather than of the model.
var Terms = []string{
	"button",
	"menu",
	"menu item",
	"dialog",
	"panel",
	"tab",
	"icon",
	"text region",
	"numeric display",
	"meter",
	"progress bar",
	"grid",
	"grid cell",
	"heads up display",
	"overlay",
	"tooltip",
}

// synonyms map what a model says onto the DETECTOR CONTRACT.s own class vocabulary.
//
// Onto vision.Class, not onto Director roles — a distinction that cost a whole benchmark
// run. The adapter first mapped to roles like "pane" and "menu_item", and the acceptance
// filter downstream speaks vision classes, so twelve of thirteen detections were rejected as
// unknown and the challenger scored 0%% structural for a reason that had nothing to do with
// the model. Every backend must speak the one vocabulary the contract defines.
//
// The mapping is one-way and total: everything a model can return either lands on an
// existing class or lands on unknown. There is no branch that creates a class from a string.
var synonyms = map[string]string{
	// Direct, onto vision.Class.
	"button": "button", "menu": "menu", "menu item": "menu",
	"dialog": "panel", "panel": "panel", "tab": "button", "icon": "icon",
	"text region": "text", "numeric display": "text", "meter": "bar",
	"progress bar": "bar", "grid": "panel", "grid cell": "slot",
	"heads up display": "panel", "overlay": "panel", "tooltip": "panel",

	// Phrasings a model reaches for on its own.
	"menu option": "menu", "menu entry": "menu", "list item": "menu",
	"window panel": "panel", "window": "panel", "frame": "panel", "container": "panel",
	"gauge": "bar", "dial": "bar", "health bar": "bar",
	"loading bar": "bar", "slider": "bar",
	"inventory cell": "slot", "inventory slot": "slot", "slot": "slot",
	"cell": "slot", "tile": "slot",
	"label": "text", "caption": "text", "heading": "text", "title": "text",
	"number": "text", "counter": "text", "score": "text", "timer": "text",
	"push button": "button", "clickable": "button", "control": "button",
	"checkbox": "checkbox", "check box": "checkbox", "radio button": "radio",
	"toolbar": "panel", "hud": "panel", "popup": "panel", "modal": "panel",
	"image": "image", "picture": "image", "text field": "field", "input": "field",
}

// UnknownRole is what anything unrecognised becomes.
//
// Not dropped, deliberately. A model producing many unknowns is a finding — it means the
// vocabulary and the model disagree — and silently discarding them would make that look
// like a model that found nothing.
const UnknownRole = "unknown"

// Prompt renders the vocabulary as an open-vocabulary detector expects it.
//
// Deterministic: sorted, joined the same way every time, so the same vocabulary always
// produces the same prompt and therefore the same question. A prompt assembled from map
// iteration would quietly vary between runs.
func Prompt() string {
	terms := make([]string, len(Terms))
	copy(terms, Terms)
	sort.Strings(terms)
	return strings.Join(terms, " . ") + " ."
}

// NormaliseLabel maps a model's word onto a Director role, or unknown.
//
// # Why this refuses prose rather than parsing it
//
// An open-vocabulary model asked a loose question answers in sentences: "a button that
// probably opens the settings menu". Extracting "button" from that would work, and would be
// the beginning of trusting a model's narration. A label is a lookup or it is nothing.
func NormaliseLabel(label string) (string, bool) {
	clean := strings.ToLower(strings.TrimSpace(label))
	clean = strings.Trim(clean, ".,;:!?\"'")
	if clean == "" {
		return UnknownRole, false
	}
	// Grounding DINO concatenates the prompt tokens a box matched, so a single match on
	// "menu" comes back as "menu menu". Collapsing an adjacent repeat is not inventing
	// anything — it is one word twice — and without it every one-word match is refused.
	clean = collapseRepeats(clean)

	// Prose, not a label. A real class name is a word or two; anything longer is the
	// model describing rather than classifying.
	if len(strings.Fields(clean)) > 3 {
		return UnknownRole, false
	}
	if role, ok := synonyms[clean]; ok {
		return role, true
	}
	// Singular/plural is the one variation worth absorbing: a model saying "buttons" for
	// "button" is not inventing anything.
	if trimmed := strings.TrimSuffix(clean, "s"); trimmed != clean {
		if role, ok := synonyms[trimmed]; ok {
			return role, true
		}
	}
	return UnknownRole, false
}

// VocabularyDigest identifies the exact prompt configuration for a report.
//
// Version plus term count plus synonym count. Enough for a reader to notice that two runs
// were not asking the same question, without pretending to be a cryptographic commitment.
func VocabularyDigest() string {
	return VocabularyVersion + "/" + itoa(len(Terms)) + "t" + itoa(len(synonyms)) + "s"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for ; n > 0; n /= 10 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
	}
	return string(digits)
}

// collapseRepeats removes adjacent duplicate words.
//
// "menu menu" → "menu", "text region text region" → "text region". Only ADJACENT repeats,
// and only whole words: this is undoing a tokenizer artefact, not guessing at meaning.
func collapseRepeats(s string) string {
	words := strings.Fields(s)
	if len(words) < 2 {
		return s
	}
	// Try halves first: a doubled phrase repeats as a block.
	if len(words)%2 == 0 {
		half := len(words) / 2
		if strings.Join(words[:half], " ") == strings.Join(words[half:], " ") {
			return strings.Join(words[:half], " ")
		}
	}
	out := []string{words[0]}
	for _, w := range words[1:] {
		if w != out[len(out)-1] {
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}
