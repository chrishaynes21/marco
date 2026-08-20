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
