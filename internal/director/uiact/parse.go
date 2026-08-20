package uiact

import (
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Parsing a semantic request.
//
// Conservative, in the same way the editing parser is conservative and for the same
// reason: a phrase this does not confidently recognise must fall through to the
// ordinary intent planner rather than be forced into a semantic action. "Expand on that
// point" is not a request to expand a tree node.
//
// What it deliberately does NOT claim: the phrases the editing parser owns — "undo",
// "select all", "copy", "paste". Those reach a richer implementation there, which can
// use a control's value API rather than a chord, and stealing them here would trade a
// verified text-state change for a keystroke. The two vocabularies overlap by design
// and the stronger one runs first.

// Request is one parsed semantic instruction.
type Request struct {
	Kind directorapi.SemanticActionKind
	// Target is the phrase naming the control ("the Downloads folder"), left for the
	// Director's ordinary target resolution. Empty means the verb addresses the
	// focused context, which is what "refresh" and "go back" mean.
	Target string
	// Ordinal is which one, from "the second result". 0 when unspecified.
	Ordinal int
	// Deictic marks "select this" / "expand that" — the user pointed rather than
	// named. Carried as a FLAG rather than as the word itself, because "this" is not a
	// label: resolving it means asking what holds focus or what was last mentioned,
	// which is the reference resolver's job. Passing "this" through as a target would
	// search the desktop for a control called "this".
	Deictic bool
}

// verbPhrases maps the words a person says to the verb they mean.
//
// Longest match wins, so "scroll to" is not read as "scroll". Ordered by length at
// lookup rather than here, so entries can be added in whatever grouping reads best.
var verbPhrases = map[string]directorapi.SemanticActionKind{
	"expand":   directorapi.SemanticExpand,
	"open up":  directorapi.SemanticExpand,
	"unfold":   directorapi.SemanticExpand,
	"collapse": directorapi.SemanticCollapse,
	"fold":     directorapi.SemanticCollapse,
	"close up": directorapi.SemanticCollapse,

	"toggle":   directorapi.SemanticToggle,
	"check":    directorapi.SemanticCheck,
	"tick":     directorapi.SemanticCheck,
	"uncheck":  directorapi.SemanticUncheck,
	"untick":   directorapi.SemanticUncheck,
	"deselect": directorapi.SemanticDeselect,
	"unselect": directorapi.SemanticDeselect,

	"invoke":   directorapi.SemanticInvoke,
	"press":    directorapi.SemanticInvoke,
	"activate": directorapi.SemanticInvoke,
	"push":     directorapi.SemanticInvoke,
	"tap":      directorapi.SemanticInvoke,
	"select":   directorapi.SemanticSelect,
	"choose":   directorapi.SemanticChoose,
	"pick":     directorapi.SemanticChoose,
	"open":     directorapi.SemanticOpen,

	"dismiss":           directorapi.SemanticDismiss,
	"submit":            directorapi.SemanticSubmit,
	"confirm":           directorapi.SemanticConfirm,
	"scroll to":         directorapi.SemanticScrollHere,
	"scroll here":       directorapi.SemanticScrollHere,
	"show context menu": directorapi.SemanticShowContextMenu,
	"context menu":      directorapi.SemanticShowContextMenu,
	"right click":       directorapi.SemanticShowContextMenu,

	"maximize": directorapi.SemanticMaximize,
	"maximise": directorapi.SemanticMaximize,
	"minimize": directorapi.SemanticMinimize,
	"minimise": directorapi.SemanticMinimize,
	"restore":  directorapi.SemanticRestore,
	"pin":      directorapi.SemanticPin,
	"unpin":    directorapi.SemanticUnpin,
	"close":    directorapi.SemanticClose,
}

// wholePhrases are the requests that take no target at all.
//
// Matched against the WHOLE phrase before anything else, so "refresh" is not read as a
// verb looking for a control called "" and "go back" is not read as a request to go to
// something called "back".
var wholePhrases = map[string]directorapi.SemanticActionKind{
	"refresh": directorapi.SemanticRefresh, "reload": directorapi.SemanticRefresh,
	"refresh it": directorapi.SemanticRefresh, "refresh this": directorapi.SemanticRefresh,
	"refresh the page": directorapi.SemanticRefresh, "reload the page": directorapi.SemanticRefresh,

	"back": directorapi.SemanticBack, "go back": directorapi.SemanticBack,
	"navigate back": directorapi.SemanticBack, "previous page": directorapi.SemanticBack,
	"forward": directorapi.SemanticForward, "go forward": directorapi.SemanticForward,
	"navigate forward": directorapi.SemanticForward,

	"next": directorapi.SemanticNext, "go to the next one": directorapi.SemanticNext,
	"next one": directorapi.SemanticNext,
	"previous": directorapi.SemanticPrevious, "previous one": directorapi.SemanticPrevious,
	"go to the previous one": directorapi.SemanticPrevious,

	"cut": directorapi.SemanticCut,

	"maximize": directorapi.SemanticMaximize, "maximise": directorapi.SemanticMaximize,
	"maximize it": directorapi.SemanticMaximize, "maximize the window": directorapi.SemanticMaximize,
	"minimize": directorapi.SemanticMinimize, "minimise": directorapi.SemanticMinimize,
	"minimize it": directorapi.SemanticMinimize, "minimize the window": directorapi.SemanticMinimize,
	"restore": directorapi.SemanticRestore, "restore it": directorapi.SemanticRestore,
	"restore the window": directorapi.SemanticRestore,

	"dismiss": directorapi.SemanticDismiss, "dismiss it": directorapi.SemanticDismiss,
	"dismiss this": directorapi.SemanticDismiss, "dismiss the popup": directorapi.SemanticDismiss,
	"submit": directorapi.SemanticSubmit, "submit it": directorapi.SemanticSubmit,
	"confirm": directorapi.SemanticConfirm, "confirm it": directorapi.SemanticConfirm,
	"ok": directorapi.SemanticConfirm, "yes": directorapi.SemanticConfirm,

	"show the context menu": directorapi.SemanticShowContextMenu,
	"open the context menu": directorapi.SemanticShowContextMenu,
}

// ordinals are the positions a choose can name.
var ordinals = map[string]int{
	"first": 1, "second": 2, "third": 3, "fourth": 4, "fifth": 5,
	"sixth": 6, "seventh": 7, "eighth": 8, "ninth": 9, "tenth": 10,
}

// Parse reads a semantic phrase, reporting false when it is not one.
//
// Matching happens in lower case for a simpler grammar, but the TARGET is recovered
// from the user's original words. A label is the user's data: "press File" names a menu
// called File, and handing "file" to the resolver would make every match rely on
// case-insensitivity that a future ranker is free to tighten. The editing parser
// preserves capitalisation for the same reason.
func Parse(phrase string) (Request, bool) {
	original := strings.TrimRight(strings.TrimSpace(phrase), " .!?,")
	if original == "" {
		return Request{}, false
	}
	s := strings.ToLower(original)

	if kind, ok := wholePhrases[s]; ok {
		return Request{Kind: kind}, true
	}

	// Longest verb first, so "scroll to the bottom item" is a scroll-here rather than
	// an unrecognised "scroll", and "close up the folder" is a collapse rather than a
	// close.
	best, bestLen := directorapi.SemanticActionKind(""), 0
	for verb, kind := range verbPhrases {
		if !strings.HasPrefix(s, verb+" ") {
			continue
		}
		if len(verb) > bestLen {
			best, bestLen = kind, len(verb)
		}
	}
	if bestLen == 0 {
		return Request{}, false
	}

	// The remainder in the USER's casing. Sliced by rune count rather than by byte
	// offset: lower-casing preserves the number of runes but not always the number of
	// bytes, and a byte index into the original would cut a multi-byte character in half.
	rest := strings.TrimSpace(string([]rune(original)[len([]rune(s[:bestLen])):]))

	// "Expand ON that point" is not a request to expand a control. A verb followed by a
	// preposition is being used figuratively or indirectly, and the direct-object
	// reading is a guess. Declining hands the phrase to the ordinary planner, which is
	// the conservative direction: a phrase this parser wrongly claims never reaches
	// anything else, while one it declines is still handled.
	if lower := strings.ToLower(rest); strings.HasPrefix(lower, "on ") ||
		strings.HasPrefix(lower, "about ") || strings.HasPrefix(lower, "for ") {
		return Request{}, false
	}

	target, deictic := cleanTarget(rest)
	req := Request{Kind: best, Target: target, Deictic: deictic}

	// An ordinal turns a select into a choose: "select the second result" names a
	// position among matches rather than a control called "second result".
	if n, remainder, ok := leadingOrdinal(req.Target); ok {
		req.Ordinal = n
		req.Target = remainder
		if req.Kind == directorapi.SemanticSelect {
			req.Kind = directorapi.SemanticChoose
		}
	}

	// A verb that needs a control, was given no name AND no pointing word is not a
	// semantic request. Handing it on rather than asking "expand what?" here keeps the
	// clarification in one place — the intent layer, which owns asking.
	if req.Kind.NeedsTarget() && req.Target == "" && !req.Deictic {
		return Request{}, false
	}
	return req, true
}

// leadingOrdinal strips "the second" from the front of a target phrase.
func leadingOrdinal(s string) (int, string, bool) {
	words := strings.Fields(s)
	if len(words) == 0 {
		return 0, s, false
	}
	if strings.EqualFold(words[0], "the") {
		words = words[1:]
	}
	if len(words) == 0 {
		return 0, s, false
	}
	n, ok := ordinals[strings.ToLower(words[0])]
	if !ok {
		return 0, s, false
	}
	return n, strings.Join(words[1:], " "), true
}

// cleanTarget strips the articles and filler a spoken target carries, and reports
// whether what remains was a pointing word rather than a name.
//
// Kept small on purpose: over-trimming would turn "the file menu" into "menu" and
// resolve against the wrong control. Only ONE leading article is removed, so "the the"
// stays odd rather than becoming silently reasonable.
func cleanTarget(s string) (string, bool) {
	s = strings.TrimSpace(s)
	// Determiners only. "on" is deliberately NOT here — it is a preposition, and
	// stripping it turned "expand on that point" into a request to expand something
	// called "that point". Parse rejects that shape before reaching this.
	for _, prefix := range []string{"the ", "that ", "this ", "a ", "an "} {
		if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
			s = strings.TrimSpace(s[len(prefix):])
			break
		}
	}
	switch strings.ToLower(s) {
	case "this", "that", "it", "here", "them", "these", "those",
		"this one", "that one", "the current one":
		return "", true
	}
	return s, false
}
