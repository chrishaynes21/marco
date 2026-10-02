package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// Asking the resolver plugin what a sentence MEANT, rather than which saved play it named.
//
// # Why this is the same plugin and the same protocol
//
// Because there is already exactly one boundary between Marco and a model: one executable named
// by `$MARCO_RESOLVER`, one JSON object in on stdin, one JSON object out on stdout. A second
// boundary would be a second thing to configure, a second thing to build, a second place for an
// API key to live, and a second answer to "is a model wired up".
//
// So this is an additional QUESTION on the existing protocol rather than a new protocol. The
// request gains one optional field and the response gains one; a plugin that only knows how to
// pick a route sees a field it does not recognise, answers `{"route":""}` as it always did, and
// the caller is told plainly that nothing interpreted the sentence.
//
//	→ {"input":"shut that pop-up","interpret":{"verbs":["invoke","dismiss",…]}}
//	← {"semantic":{"kind":"dismiss"}}
//
//	→ {"input":"press the third result","interpret":{"verbs":[…]}}
//	← {"semantic":{"kind":"choose","target":"result","ordinal":3}}
//
// # What the plugin is TOLD, which is the user's own words and a closed list
//
// Nothing about the screen. Not a control label, not a window title, not an element, not a
// coordinate, not what Marco remembers. The model is being asked what a sentence means — not
// which thing on the desktop it points at — so the evidence it would need in order to answer the
// second question is deliberately absent, and a plugin that wanted to send the screen somewhere
// has nothing here to send.
//
// That is what keeps grounding where it belongs: the target travels on as the user's PHRASE, and
// Marco's own resolver decides what it refers to against a world it observed itself.
//
// # What the plugin may SAY, which is four fields
//
// A verb from the list it was given, a target phrase, a position, and whether the user pointed
// rather than named. There is no field for an element id, a window, a coordinate, a keystroke, a
// file path or a play — so the narrowest statement of what this boundary permits is not a rule
// written down here, it is the shape of [Semantic].

// interpretRequest is the question, carried on the existing request object.
type interpretRequest struct {
	// Verbs is the closed vocabulary the answer must come from, supplied by the caller so
	// this package never holds a copy of it. A list that drifted from the one the Director
	// can actually perform would invite a verb that parses and refuses.
	Verbs []string `json:"verbs"`
}

// Semantic is what a plugin made of one sentence.
//
// Shape only. This package does not know the verb vocabulary and does not judge `Kind` against it
// — that is the Director's, and doing it here would be a second opinion about what Marco can do.
// What is enforced here is that the value is small, bounded and carries nothing but these four
// things.
type Semantic struct {
	// Kind is the semantic verb, as one of the words the request offered.
	Kind string `json:"kind"`
	// Target is the user's own phrase for the control, left for the Director's resolver.
	// Empty means the verb addresses whatever has focus, which is what "go back" means.
	Target string `json:"target,omitempty"`
	// Ordinal is which one, from "the third result". 0 when unspecified.
	Ordinal int `json:"ordinal,omitempty"`
	// Deictic says the user pointed rather than named — "dismiss this".
	Deictic bool `json:"deictic,omitempty"`
}

// MaxTargetPhrase bounds how long a target phrase may be.
//
// A control's name, not a sentence. Past this the plugin is not naming a thing on screen, it is
// passing prose through a field the resolver will search for verbatim — and a very long phrase is
// also the shape a prompt-injected answer takes.
const MaxTargetPhrase = 120

// MaxOrdinal bounds "the Nth one", matching the deterministic parser's own ordinal table.
const MaxOrdinal = 10

// InterpretTimeout is how long one interpretation may take.
//
// # Why there is a timeout here at all
//
// Because this runs a subprocess that may reach a network, and the caller's context is the one
// that belongs to the whole request. A model that hangs must not hold a Director command open
// indefinitely, and the honest failure is "nothing interpreted it" — which the caller already
// handles, because that is what an unconfigured plugin produces.
//
// Generous rather than tight: a local model on a cold cache is slow, and interpreting a sentence
// happens before anything touches the desktop, so the cost of waiting is a person waiting rather
// than a half-finished action.
//
// A VARIABLE rather than a constant purely so a test can shorten it. "A plugin that hangs becomes
// a deterministic refusal" is one of the properties that must hold before any model is trusted, and
// at twenty seconds it is a property nobody would run. Production never reassigns it — the same
// seam, and the same justification, as `dialPerformer` and `pendingQuestion`.
var InterpretTimeout = 20 * time.Second

// Interpret asks the plugin what one sentence meant, reporting false when nothing did.
//
// False is the ORDINARY answer and never an error: no plugin configured, a plugin that only
// understands routes, a plugin that declined, a malformed reply, a timeout, a cancelled context.
// Every one of them means the deterministic reading stands, which is why there is no error to
// return and nothing for a caller to branch on.
//
// The context is the CALLER's, so a stop reaches the subprocess: `exec.CommandContext` kills it.
// The timeout above is a bound on top of that, never instead of it.
func Interpret(ctx context.Context, input string, verbs []string) (Semantic, bool) {
	path := Path()
	if path == "" || strings.TrimSpace(input) == "" || len(verbs) == 0 {
		return Semantic{}, false
	}
	// The ROUTES field is deliberately absent rather than empty-but-present: this is not a
	// route question, and a plugin that answered one would be answering something nobody
	// asked. A route-only plugin reads no routes, finds nothing to pick from, and says so.
	reqBytes, err := json.Marshal(struct {
		Input     string            `json:"input"`
		Interpret *interpretRequest `json:"interpret"`
	}{Input: input, Interpret: &interpretRequest{Verbs: verbs}})
	if err != nil {
		return Semantic{}, false
	}

	ctx, cancel := context.WithTimeout(ctx, InterpretTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path)
	cmd.Stdin = bytes.NewReader(append(reqBytes, '\n'))
	out, err := cmd.Output()
	if err != nil {
		return Semantic{}, false
	}
	var resp struct {
		Semantic *Semantic `json:"semantic"`
	}
	if json.Unmarshal(bytes.TrimSpace(out), &resp) != nil || resp.Semantic == nil {
		// A route-only plugin lands here: its `{"route":"…"}` unmarshals with no
		// `semantic`, and a reply with no interpretation in it is not an interpretation.
		return Semantic{}, false
	}
	return clamp(*resp.Semantic)
}

// clamp enforces the SHAPE of an answer, and refuses rather than repairing.
//
// # Why a bad answer is dropped whole
//
// Because the alternative is guessing which half of it to keep. An answer naming a verb outside
// the list it was given, or a position of 400, or a target phrase the length of a paragraph, is an
// answer from something that did not do what it was asked — and taking the verb while discarding
// the position would act on a reading nothing produced.
//
// Dropping it is safe by construction: the caller's deterministic reading is still there, and its
// refusal sentence is still the one the person sees.
func clamp(s Semantic) (Semantic, bool) {
	s.Kind = strings.TrimSpace(s.Kind)
	s.Target = strings.TrimSpace(s.Target)
	if s.Kind == "" {
		return Semantic{}, false
	}
	if len([]rune(s.Target)) > MaxTargetPhrase {
		return Semantic{}, false
	}
	if s.Ordinal < 0 || s.Ordinal > MaxOrdinal {
		return Semantic{}, false
	}
	// A POINTED-AT TARGET AND A NAMED ONE ARE DIFFERENT CLAIMS. "Dismiss this" has no name
	// and "dismiss the update banner" is not a pointing; an answer claiming both describes no
	// request a person made, and the construction downstream would silently prefer one.
	if s.Deictic && s.Target != "" {
		return Semantic{}, false
	}
	return s, true
}
