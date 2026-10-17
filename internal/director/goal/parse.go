package goal

import (
	"strings"
)

// Parsing a goal.
//
//	Natural language → Goal → Procedure → Semantic Program → Execution
//
// Deterministic and conservative, like every other parser in the Director. A phrase this
// does not confidently recognise falls through to the ordinary planner, which still
// handles "click Save" — so the cost of declining is nothing, and the cost of claiming
// wrongly is a five-step procedure aimed at the wrong thing.
//
// The vocabulary is small on purpose. These are the requests the milestone names, and a
// goal that parsed but had no procedure would refuse at expansion for a reason no reader
// could see.

// pattern is one way of saying a goal.
//
// Matched as a PREFIX against the lower-cased phrase, longest first, so "save as" is not
// read as "save" and "close without saving" is not read as "close".
type pattern struct {
	prefix string
	kind   Kind
	// nameFollows: what comes after the separator is the new name, not a target.
	nameFollows bool
	// targetFollows: what comes after the prefix is the thing to act on.
	targetFollows bool
}

// patterns is the goal vocabulary, in no particular order — length decides precedence.
var patterns = []pattern{
	// Whole-phrase goals: no target, no name.
	{prefix: "save as", kind: SaveAs, nameFollows: true},
	{prefix: "save this file", kind: Save},
	{prefix: "save this document", kind: Save},
	{prefix: "save the file", kind: Save},
	{prefix: "save it", kind: Save},
	{prefix: "save this", kind: Save},
	{prefix: "save", kind: Save},

	{prefix: "close without saving", kind: CloseWithoutSaving},
	{prefix: "close and discard", kind: CloseWithoutSaving},
	{prefix: "discard changes and close", kind: CloseWithoutSaving},

	{prefix: "print this document", kind: Print},
	{prefix: "print this", kind: Print},
	{prefix: "print it", kind: Print},
	{prefix: "print", kind: Print},

	{prefix: "open settings", kind: OpenSettings},
	{prefix: "open the settings", kind: OpenSettings},
	{prefix: "open preferences", kind: OpenSettings},

	{prefix: "create a new tab", kind: CreateTab},
	{prefix: "create a tab", kind: CreateTab},
	{prefix: "new tab", kind: CreateTab},
	{prefix: "open a new tab", kind: CreateTab},

	// Goals that name a new thing.
	{prefix: "create a new folder called", kind: CreateFolder, nameFollows: true},
	{prefix: "create a folder called", kind: CreateFolder, nameFollows: true},
	{prefix: "create a new folder named", kind: CreateFolder, nameFollows: true},
	{prefix: "create a folder named", kind: CreateFolder, nameFollows: true},
	{prefix: "make a new folder called", kind: CreateFolder, nameFollows: true},
	{prefix: "make a folder called", kind: CreateFolder, nameFollows: true},
	{prefix: "new folder called", kind: CreateFolder, nameFollows: true},
	// AND THE SAME REQUESTS WITHOUT A NAME, which are the same request with a question
	// attached.
	//
	// Measured live: "make a new folder called Reports" ran the Explorer procedure, and
	// "make a new folder" answered *"I only understand click, focus, move, repeat and
	// text editing so far"* — because no pattern matched, so it was never a goal, so
	// nothing ever asked what to call it. Marco has the goal, has the Explorer procedure,
	// and was missing one word.
	//
	// Matching them makes the goal recognised and the name MISSING, which is a state this
	// package already models and already has the sentence for: `Procedure.Missing` reports
	// it, `Refusal.Question` renders it as "create a folder — what should it be called?",
	// and the program layer already pauses on a clarification with the goal's values
	// intact. Nothing new was needed except the admission that the request was understood.
	//
	// Safe beside the longer patterns because the match is longest-prefix: "make a new
	// folder called Reports" still takes the name with it.
	{prefix: "create a new folder", kind: CreateFolder, nameFollows: true},
	{prefix: "create a folder", kind: CreateFolder, nameFollows: true},
	{prefix: "make a new folder", kind: CreateFolder, nameFollows: true},
	{prefix: "make a folder", kind: CreateFolder, nameFollows: true},
	{prefix: "new folder", kind: CreateFolder, nameFollows: true},

	// Goals that name a thing to act on.
	{prefix: "rename", kind: Rename, targetFollows: true},
	{prefix: "duplicate", kind: Duplicate, targetFollows: true},
	{prefix: "delete", kind: Delete, targetFollows: true},
	{prefix: "download", kind: Download, targetFollows: true},
	{prefix: "move", kind: Move, targetFollows: true},
}

// separators introduce the second half of a two-part goal: the new name, or the
// destination. Longest first for the same reason as the prefixes.
var separators = []string{" to be called ", " to be named ", " called ", " named ", " into ", " to "}

// Parse reads a goal from a phrase, reporting false when it is not one.
func Parse(phrase string) (Goal, bool) {
	original := strings.TrimRight(strings.TrimSpace(phrase), " .!?,")
	if original == "" {
		return Goal{}, false
	}
	lower := strings.ToLower(original)

	best, bestLen := pattern{}, 0
	for _, p := range patterns {
		if lower != p.prefix && !strings.HasPrefix(lower, p.prefix+" ") {
			continue
		}
		if len(p.prefix) > bestLen {
			best, bestLen = p, len(p.prefix)
		}
	}
	if bestLen == 0 {
		return Goal{}, false
	}

	g := Goal{Kind: best.kind, Phrase: original, Parameters: map[string]string{}}
	// The remainder in the USER's casing: a name is the user's data, and "Reports" is
	// not "reports" when it becomes a folder on their disk.
	rest := strings.TrimSpace(string([]rune(original)[len([]rune(lower[:bestLen])):]))

	switch {
	case best.nameFollows && rest != "":
		// Everything after the prefix is the name — "create a folder called Reports".
		g.Parameters[ParamName] = trimQuotes(rest)

	case best.targetFollows && rest != "":
		// "rename this file to Budget" — the separator splits the target from the name.
		target, tail, sep := splitOnSeparator(rest)
		g.Context.Target, g.Context.TargetIsImplicit = cleanTarget(target)
		if tail != "" {
			switch best.kind {
			case Move:
				// "move this file to Downloads" — the tail is where it goes.
				g.Parameters[ParamDestination] = trimQuotes(tail)
			default:
				g.Parameters[ParamName] = trimQuotes(tail)
			}
		}
		_ = sep
	}

	// SaveAs takes its name after "save as", with no target.
	if best.kind == SaveAs && rest != "" {
		g.Parameters[ParamName] = trimQuotes(rest)
	}
	return g, true
}

// splitOnSeparator finds the first separator and splits around it.
//
// FIRST rather than last: "rename Budget to Q3 to Q4" is a phrase nobody means, and
// taking the first keeps "rename X to Y" reading the way it is said.
func splitOnSeparator(s string) (head, tail, sep string) {
	lower := strings.ToLower(s)
	bestAt, bestSep := -1, ""
	for _, candidate := range separators {
		if i := strings.Index(lower, candidate); i >= 0 {
			if bestAt < 0 || i < bestAt || (i == bestAt && len(candidate) > len(bestSep)) {
				bestAt, bestSep = i, candidate
			}
		}
	}
	if bestAt < 0 {
		return strings.TrimSpace(s), "", ""
	}
	return strings.TrimSpace(s[:bestAt]),
		strings.TrimSpace(s[bestAt+len(bestSep):]),
		strings.TrimSpace(bestSep)
}

// cleanTarget strips determiners and reports whether what remains was a pointing word.
//
// "Rename this file to Budget" points at whatever is selected. Passing "this file"
// through as a label would search the desktop for a control called "this file"; marking
// it implicit lets resolution use the selection, which is what the user meant.
func cleanTarget(s string) (string, bool) {
	s = strings.TrimSpace(s)
	for _, prefix := range []string{"the ", "this ", "that ", "a ", "an "} {
		if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
			s = strings.TrimSpace(s[len(prefix):])
			break
		}
	}
	switch strings.ToLower(s) {
	case "", "this", "that", "it", "file", "document", "image", "folder", "one", "thing":
		// A bare noun with no name is a pointing word too: "rename this file" and
		// "rename the file" both mean the one that is selected.
		return "", true
	}
	return trimQuotes(s), false
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return strings.TrimSpace(s[1 : len(s)-1])
		}
	}
	return s
}

// Supplying returns the request the person would have made had they said the missing part.
//
// # Why reconstruct the phrase rather than patch the Goal
//
// Because `Parse` owns the patterns, and a second path that filled a parameter directly would
// be a second definition of what "create a folder called X" means. The patterns are the
// vocabulary; this uses them.
//
// And it CHECKS ITSELF. The reconstructed phrase is parsed back, and the answer is only
// accepted if the parameter landed where it was meant to. So a requirement with no phrasing
// here, or an answer that breaks the pattern, returns false and the clarification is honestly
// unresumable rather than silently turned into a different request.
//
// Measured live: "make a new folder" asked "what should it be called?" and an answer of
// "Reports" was re-run as a request to find a control called Reports — because an answer the
// clarification parser does not recognise is treated as a new command, which is right for
// everything except an answer that was asked for.
//
// Enforced by TestAnAnsweredQuestionBecomesTheRequestThePersonMeant.
func Supplying(phrase string, missing Requirement, answer string) (string, bool) {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return "", false
	}
	joiner, ok := joiners[missing]
	if !ok {
		return "", false
	}
	full := strings.TrimRight(strings.TrimSpace(phrase), " .!?,") + joiner + answer
	g, parsed := Parse(full)
	if !parsed {
		return "", false
	}
	want := paramFor[missing]
	if want == "" || g.Param(want) != trimQuotes(answer) {
		return "", false
	}
	return full, true
}

// joiners are the words that attach an answer to the request it answers.
//
// One per requirement that can be answered with a value. A requirement absent here cannot be
// resumed this way, which is the honest state for `RequiresTarget`: "which one?" is answered by
// pointing or by choosing from a list, not by appending a word to the sentence.
var joiners = map[Requirement]string{
	RequiresName:        " called ",
	RequiresDestination: " to ",
}

// paramFor is the parameter each answerable requirement fills, so the self-check knows what to
// look for.
var paramFor = map[Requirement]string{
	RequiresName:        ParamName,
	RequiresDestination: ParamDestination,
}
