package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

// Asking the resolver plugin WHICH PART OF A REQUEST is which argument's value.
//
// # Why this is the same plugin and the same protocol
//
// For the reason `Interpret` gives at length: there is already exactly one boundary between Marco
// and a model — one executable named by `$MARCO_RESOLVER`, one JSON object in on stdin, one out on
// stdout — and a second boundary would be a second thing to configure, a second thing to build, a
// second place for a key to live and a second answer to "is a model wired up".
//
// So this is a THIRD QUESTION on that protocol. The request gains one optional field and the
// response gains one; a plugin that only knows how to pick a route, or only routes and verbs, sees
// a field it does not recognise and answers as it always did, and the caller is told plainly that
// nothing extracted anything.
//
//	→ {"input":"write a note saying hello there","extract":{"names":["message"]}}
//	← {"values":[{"name":"message","start":22,"end":33}]}
//
// # What the plugin is TOLD, which is the words and a closed list of names
//
// Nothing about the screen, exactly as with `Interpret`: not a control, not a window, not an
// element, not a coordinate, not what Marco remembers. A value is a span of the person's own
// sentence, so the evidence a screen question would need is deliberately absent and a plugin that
// wanted to send a desktop somewhere has nothing here to send.
//
// And nothing about the routine beyond the NAMES of what it takes. Not its source, not its steps,
// not where it works — because the question is which words are which value, and a plugin that knew
// what the routine did could start answering a different question.
//
// # It rides $MARCO_RESOLVER, and that is how `meaning` stays untouched
//
// There are two variables and they are separate on purpose. `$MARCO_RESOLVER` names the plugin
// that answers which routine a request meant and what a sentence meant; `$MARCO_INTERPRETER` names
// the one that would answer which observed CONTROL was meant, and `internal/director/meaning` —
// the question held in shadow mode — is the only thing that reads it.
//
// This question is about words and takes the first variable. So configuring extraction cannot
// configure the shadow interpreter, and `cmd/director TestTheInterpreterIsNotWiredIntoAnything`
// stays green because nothing here goes near it.
//
// # OFFSETS, NEVER TEXT, AND THAT IS THE WHOLE SAFETY OF IT
//
// [Span] has no field for a value. There is nowhere for a plugin to put a word Marco did not send
// it, so an invented, paraphrased, translated, re-cased, trimmed or tidied value is not something
// this boundary refuses — it is something it cannot express. The only thing a plugin can get wrong
// is WHICH span, and that is visible in the confirmation beside the argument it was proposed for.
//
// `internal/director/values/provenance.go` is the shape being followed, and says why: everything
// there is *designed to be safe rather than redacted to be safe*, because a struct with a
// plaintext field has to be scrubbed at every boundary and will eventually be missed at one.
//
// See [[Extracting-argument-values-proposal-2026-10-30]].

// extractRequest is the question, as it arrives.
type extractRequest struct {
	Names []string `json:"names"`
	// Routine is the name of the routine the request was resolved to, so the words that NAME it —
	// however the recogniser or the person got them wrong — are not taken for a value. Empty when
	// nothing was chosen; an older plugin ignores it.
	Routine string `json:"routine,omitempty"`
}

// Span is where one argument's value is, in the request that was sent.
//
// AN ARGUMENT NAME AND TWO OFFSETS. No text, on purpose — see the note above. The offsets are BYTE
// offsets into the input as Marco sent it, half-open: `input[Start:End]`.
type Span struct {
	Name  string `json:"name"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// Value is the text this span names, copied out of the request Marco sent.
//
// THE ONE PLACE A VALUE COMES INTO EXISTENCE, and it comes out of the request rather than out of
// an answer. Verbatim: not trimmed, not collapsed, not normalised, because a value is used exactly
// as it was supplied and a span including a leading space includes it.
//
// Safe on any Span that came from [Extract], which validates the bounds before returning one; it
// answers "" for a span that did not, rather than panicking in whatever called it.
func (s Span) Value(input string) string {
	if s.Start < 0 || s.End > len(input) || s.Start >= s.End {
		return ""
	}
	return input[s.Start:s.End]
}

// MaxArgumentNames bounds how many names one question may ask about.
//
// A routine's arguments, not a vocabulary. Past this, something is asking a model to fill in a form
// rather than read a sentence, and a very long list is also how a prompt-injected request would
// try to smuggle instructions through a field that is meant to hold identifiers.
const MaxArgumentNames = 16

// MaxArgumentName bounds one name, which is a slug-shaped identifier and never prose.
const MaxArgumentName = 64

// ExtractTimeout is how long one extraction may take.
//
// The reasoning is `InterpretTimeout`'s exactly: a subprocess that may reach a network must not
// hold a request open, the honest failure is "nothing extracted anything" because that is also
// what an unconfigured plugin produces, and it is a variable only so that a test can shorten it —
// "a plugin that hangs becomes a deterministic refusal" is a property nobody would run at twenty
// seconds. Production never reassigns it.
var ExtractTimeout = 20 * time.Second

// Extract asks the plugin which spans of a request are which argument's value.
//
// False is the ORDINARY answer and never an error, as with `Interpret`: no plugin configured, a
// plugin that only understands routes, one that declined, a malformed reply, a timeout, a
// cancelled context. Every one of them means nothing was extracted and the person supplies the
// value themselves, which is why there is no error to return and nothing to branch on.
//
// # Every answer is validated, and a bad one is refused rather than repaired
//
// Each check below is something a plugin can get wrong, and each is a refusal of THAT span rather
// than of the whole answer — one unusable span should not cost the person the others. An answer
// with no usable span at all reports false, which is the same as not having asked.
func Extract(ctx context.Context, input string, names []string) ([]Span, bool) {
	return ExtractFor(ctx, input, "", names)
}

// ExtractFor is Extract told which routine the request was resolved to. A request whose routine
// name was misheard ("cloud paul stone" for the routine "cloud pull") otherwise reads to the model
// as if every word were the value.
func ExtractFor(ctx context.Context, input, routine string, names []string) ([]Span, bool) {
	path := Path()
	switch {
	case path == "", strings.TrimSpace(input) == "", len(names) == 0:
		return nil, false
	case len(names) > MaxArgumentNames:
		return nil, false
	}
	asked := map[string]bool{}
	for _, n := range names {
		if n == "" || len(n) > MaxArgumentName {
			return nil, false
		}
		asked[n] = true
	}
	// THE ROUTES AND VERBS FIELDS ARE DELIBERATELY ABSENT rather than empty-but-present:
	// this is neither a route question nor a verb question, and a plugin that answered one
	// would be answering something nobody asked.
	reqBytes, err := json.Marshal(struct {
		Input   string          `json:"input"`
		Extract *extractRequest `json:"extract"`
	}{Input: input, Extract: &extractRequest{Names: names, Routine: routine}})
	if err != nil {
		return nil, false
	}

	ctx, cancel := context.WithTimeout(ctx, ExtractTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path)
	cmd.Stdin = bytes.NewReader(append(reqBytes, '\n'))
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	var resp struct {
		Values []Span `json:"values"`
	}
	if json.Unmarshal(bytes.TrimSpace(out), &resp) != nil || len(resp.Values) == 0 {
		// A plugin that knows only routes or only verbs lands here: its reply
		// unmarshals with no `values`, and a reply with no extraction in it is not an
		// extraction.
		return nil, false
	}
	// HOW OFTEN EACH NAME WAS ANSWERED, COUNTED BEFORE ANY SPAN IS TAKEN.
	//
	// Two spans for one argument is an answer with no reading in it, and NEITHER is taken:
	// keeping the first would be this package picking on a model's behalf, and the first in a
	// list is not a judgement about anything. Counting is a pass of its own because the
	// decision is about the whole answer, and a loop that discovered the second span after
	// accepting the first would have to take one back.
	answered := map[string]int{}
	for _, s := range resp.Values {
		answered[s.Name]++
	}
	var ok []Span
	for _, s := range resp.Values {
		switch {
		case !asked[s.Name]:
			// A NAME NOBODY ASKED ABOUT. There is nowhere for that text to go,
			// and it is the shape an answer takes when a model has invented a
			// field rather than read the list it was given.
			continue
		case answered[s.Name] > 1:
			// TWO SPANS FOR ONE ARGUMENT, so that argument goes unanswered and
			// the person supplies it themselves.
			continue
		case s.Start < 0 || s.End > len(input) || s.Start >= s.End:
			// OUTSIDE THE REQUEST, or empty. An empty span is not a value; a
			// value that is empty is SUPPLIED, which is a different thing and
			// never reaches this question at all.
			continue
		case !utf8.RuneStart(input[s.Start]) ||
			(s.End < len(input) && !utf8.RuneStart(input[s.End])):
			// OFF A RUNE BOUNDARY, which would slice a character in half. A
			// model counting characters where Marco counts bytes is the
			// likeliest way to arrive here, and a half character is not text
			// anybody said.
			continue
		case strings.TrimSpace(input[s.Start:s.End]) == "":
			// ONLY WHITESPACE. Nobody means "type three spaces" by naming a
			// span, and this is what a miscount looks like from the outside.
			continue
		}
		ok = append(ok, s)
	}
	if len(ok) == 0 {
		return nil, false
	}
	return ok, true
}

// ExtractorConfigured reports whether a plugin is configured at all.
//
// The same answer `InterpreterConfigured` gives, named for this question so a caller reads what it
// is asking about. It says nothing about whether the plugin can answer it — a route-only plugin is
// configured and extracts nothing — which is why the sentence a surface shows distinguishes
// "nothing is configured" from "it was asked and said nothing".
func ExtractorConfigured() bool { return Path() != "" }
