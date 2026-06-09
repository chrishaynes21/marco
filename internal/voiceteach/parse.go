// Package voiceteach turns spoken (or typed) narration into a macro: each phrase is
// parsed into a Cmd, and a Session assembles the matching macroir Steps — "click
// this", "anchor this", "wait for this screen", "type hello", "press enter", "wait 2
// seconds", "activate notepad", plus the controls "undo" / "done" / "cancel". The
// parser is pure and OS-agnostic (unit-tested here); the Session reads the live
// cursor/screen through an Env so the OS-specific bits stay injectable.
package voiceteach

import (
	"strconv"
	"strings"
)

// Kind is the recognized command class for a narration phrase.
type Kind int

const (
	None       Kind = iota // not recognized — caller should ask the user to rephrase
	Click                  // click where the cursor is
	Anchor                 // capture an image of the target under the cursor, then click it
	WaitScreen             // wait until the current screen reappears (capture-now, find-at-run)
	Wait                   // wait Ms milliseconds
	Type                   // type Text
	Key                    // press Key
	Activate               // bring App to the front
	Undo                   // drop the last recorded step
	Done                   // finish and save
	Cancel                 // abandon without saving
)

// Cmd is a parsed narration phrase. Only the fields relevant to Kind are set.
type Cmd struct {
	Kind Kind
	Text string // Type
	Key  string // Key
	App  string // Activate
	Ms   int    // Wait
}

// Parse interprets one narration phrase. It's forgiving about phrasing (synonyms,
// trailing punctuation, case) and returns {Kind: None} for anything it doesn't
// recognize so the caller can prompt for a rephrase rather than guess.
func Parse(phrase string) Cmd {
	phrase = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(phrase), ".!,?"))
	low := strings.ToLower(phrase)
	if low == "" {
		return Cmd{Kind: None}
	}

	switch low {
	case "done", "save", "save it", "finish", "finished", "that's it", "thats it", "all done":
		return Cmd{Kind: Done}
	case "cancel", "never mind", "nevermind", "stop teaching", "forget it", "abort":
		return Cmd{Kind: Cancel}
	case "undo", "scratch that", "delete that", "remove that", "go back", "oops":
		return Cmd{Kind: Undo}
	case "click", "click this", "click here", "click that", "click it", "tap this", "tap that", "select this":
		return Cmd{Kind: Click}
	case "anchor this", "anchor", "anchor it", "anchor that", "remember this", "remember that", "save this image":
		return Cmd{Kind: Anchor}
	case "wait for this screen", "wait for the screen", "wait for screen", "wait for this",
		"wait for this to load", "wait for the page", "wait for it":
		return Cmd{Kind: WaitScreen}
	}

	// Prefix forms — longest/most-specific prefixes first so "switch to" beats "to".
	if rest, ok := cut(phrase, "type ", "say ", "write ", "input "); ok && rest != "" {
		if ph, ok := argPlaceholder(rest); ok {
			return Cmd{Kind: Type, Text: ph} // "type arg 1" → {{1}}, filled at run time
		}
		return Cmd{Kind: Type, Text: rest}
	}
	if rest, ok := cut(phrase, "press the ", "press ", "hit the ", "hit ", "tap the ", "key "); ok && rest != "" {
		return Cmd{Kind: Key, Key: normalizeKey(rest)}
	}
	if rest, ok := cut(phrase, "switch to ", "go to ", "activate ", "focus on ", "focus ", "open up ", "open "); ok && rest != "" {
		return Cmd{Kind: Activate, App: strings.ToLower(rest)}
	}
	if rest, ok := cut(phrase, "wait for ", "wait ", "pause for ", "pause ", "sleep for ", "sleep ", "hold for ", "hold "); ok {
		if ms, ok := parseDuration(rest); ok {
			return Cmd{Kind: Wait, Ms: ms}
		}
	}
	return Cmd{Kind: None}
}

// cut returns the case-preserved remainder after the first matching prefix.
func cut(phrase string, prefixes ...string) (rest string, ok bool) {
	low := strings.ToLower(phrase)
	for _, pre := range prefixes {
		if strings.HasPrefix(low, pre) {
			return strings.TrimSpace(phrase[len(pre):]), true
		}
	}
	return "", false
}

// parseDuration reads "2", "2 seconds", "500 ms", "1.5s", "half a second" loosely,
// returning milliseconds. Bare numbers are seconds.
func parseDuration(s string) (ms int, ok bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "a second" || s == "one second" || s == "a sec" {
		return 1000, true
	}
	if s == "half a second" || s == "a half second" {
		return 500, true
	}
	// Pull the leading number (allow a decimal).
	num := strings.TrimSpace(s)
	unit := ""
	if i := strings.IndexFunc(s, func(r rune) bool { return r != '.' && (r < '0' || r > '9') }); i > 0 {
		num, unit = strings.TrimSpace(s[:i]), strings.TrimSpace(s[i:])
	} else if i == 0 {
		return 0, false // doesn't start with a number
	}
	f, err := strconv.ParseFloat(num, 64)
	if err != nil || f < 0 {
		return 0, false
	}
	switch {
	case strings.HasPrefix(unit, "ms") || strings.HasPrefix(unit, "milli"):
		return int(f), true
	case strings.HasPrefix(unit, "min"):
		return int(f * 60000), true
	default: // "", "s", "sec", "seconds"
		return int(f * 1000), true
	}
}

// ordinals maps spoken ordinals to argument positions.
var ordinals = map[string]int{"first": 1, "second": 2, "third": 3, "fourth": 4, "fifth": 5}

// argPlaceholder recognizes a spoken reference to a positional argument — "arg 2",
// "argument 2", "the second argument" — and returns the codegen placeholder
// "{{2}}", which routes.ApplyArgs fills when the route is run with `… with a, b`.
func argPlaceholder(s string) (string, bool) {
	low := strings.ToLower(strings.TrimSpace(s))
	for _, pre := range []string{"argument ", "arg "} {
		if r, ok := strings.CutPrefix(low, pre); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(r)); err == nil && n >= 1 {
				return "{{" + strconv.Itoa(n) + "}}", true
			}
		}
	}
	low = strings.TrimPrefix(low, "the ")
	if r, ok := strings.CutSuffix(low, " argument"); ok {
		if n, ok := ordinals[strings.TrimSpace(r)]; ok {
			return "{{" + strconv.Itoa(n) + "}}", true
		}
	}
	return "", false
}

// normalizeKey lowercases the spoken key name and maps a few spoken synonyms to the
// names the OS Key capability expects.
func normalizeKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "return":
		return "enter"
	case "escape", "esc key":
		return "esc"
	case "space bar", "spacebar":
		return "space"
	case "control", "ctrl":
		return "ctrl"
	}
	return s
}
