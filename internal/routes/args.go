package routes

import (
	"regexp"
	"strconv"
	"strings"
)

// phRe matches a {{name}} placeholder (named arg, or positional {{1}}) left in a
// route's source by codegen; secrets were already converted to Secret calls, so any
// remaining placeholder is an argument.
var phRe = regexp.MustCompile(`\{\{([A-Za-z0-9_-]+)\}\}`)

// namedRe matches a "key:value" invocation token (the value may be empty or run on
// into following tokens).
var namedRe = regexp.MustCompile(`^([A-Za-z][\w-]*):(.*)$`)

// SplitArgs splits a TEACH name (or positional invocation) on " with ": "say hello
// with name" → ("say hello", ["name"]); "dm with person, message" → ("dm",
// ["person","message"]). At teach time the list is the declared arg names; in the
// positional run form it's the values. No " with " → (command, nil).
func SplitArgs(command string) (route string, args []string) {
	i := strings.Index(strings.ToLower(command), " with ")
	if i < 0 {
		return strings.TrimSpace(command), nil
	}
	route = strings.TrimSpace(command[:i])
	for a := range strings.SplitSeq(command[i+len(" with "):], ",") {
		if a = strings.TrimSpace(a); a != "" {
			args = append(args, a)
		}
	}
	return route, args
}

// chainRe splits a command on the " then " keyword (case-insensitive, surrounded by
// whitespace) — the separator that chains several commands under one trigger.
var chainRe = regexp.MustCompile(`(?i)\s+then\s+`)

// SplitChain splits a command into its sequential steps on " then ": "say hi then
// wave then say bye" → ["say hi", "wave", "say bye"]. A command with no " then " is a
// single step. Each step still carries its own " with …"/"name:" args, parsed later
// by ParseInvocation. Used to run or bind several commands as one.
func SplitChain(command string) []string {
	var out []string
	for _, p := range chainRe.Split(command, -1) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{strings.TrimSpace(command)}
	}
	return out
}

// ParseInvocation parses a run command into the route phrase and its arguments,
// preferring NAMED "key:value" tokens — "say hello name:chris" → ("say hello",
// {name:chris}, nil); "dm person:sam message:hi there" → ("dm", {person:sam,
// message:"hi there"}, nil). A value runs until the next "key:" token, so it can
// contain spaces. With no named tokens it falls back to positional " with a, b".
func ParseInvocation(command string) (route string, named map[string]string, positional []string) {
	named = map[string]string{}
	var routeToks []string
	curKey := ""
	for tok := range strings.FieldsSeq(command) {
		if m := namedRe.FindStringSubmatch(tok); m != nil && !strings.HasPrefix(m[2], "//") {
			curKey = strings.ToLower(m[1])
			named[curKey] = m[2]
		} else if curKey != "" {
			named[curKey] = strings.TrimSpace(named[curKey] + " " + tok)
		} else {
			routeToks = append(routeToks, tok)
		}
	}
	if len(named) > 0 {
		return strings.Join(routeToks, " "), named, nil
	}
	r, pos := SplitArgs(command)
	return r, nil, pos
}

// ApplyArgs fills placeholders in a route's source: {{name}} from named (by name,
// case-insensitive), {{N}} from positional (1-based). A positional invocation also
// fills NAMED placeholders — the values map onto the route's declared arg names in
// order, so "say hello with chris" lands "chris" in {{name}} (i.e. `with chris` ≡
// `name:chris`). Explicit "name:value" tokens win over the positional mapping. Each
// value is escaped for the Marco string literal it sits in. An unfilled placeholder
// becomes empty. Source with no placeholders is returned unchanged, so it's safe to
// call on every run.
func ApplyArgs(src string, named map[string]string, positional []string) string {
	// Zip positional values onto the declared arg names (source order), so a value
	// given without a "key:" still reaches its named placeholder.
	byPos := map[string]string{}
	for i, name := range ArgNames(src) {
		if i < len(positional) {
			byPos[strings.ToLower(name)] = positional[i]
		}
	}
	return phRe.ReplaceAllStringFunc(src, func(m string) string {
		key := strings.ToLower(m[2 : len(m)-2])
		if v, ok := named[key]; ok {
			return escapeArg(v)
		}
		if n, err := strconv.Atoi(key); err == nil && n >= 1 && n <= len(positional) {
			return escapeArg(positional[n-1])
		}
		if v, ok := byPos[key]; ok {
			return escapeArg(v)
		}
		return ""
	})
}

// secretArgRe matches a route-qualified secret call — do OS's Secret with
// "route:argname" — capturing the arg name. Global secrets (no ":") are pre-set, not
// invocation args, so they're skipped.
var secretArgRe = regexp.MustCompile(`OS's Secret with "[^"]*:([A-Za-z0-9_-]+)"`)

// ArgNames returns the argument names a route accepts — both {{name}} placeholders
// and route-qualified secret args (password/pin/…) — first-seen order. The overlay
// auto-pops these as "name:" labels.
func ArgNames(src string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	for _, m := range phRe.FindAllStringSubmatch(src, -1) {
		add(m[1])
	}
	for _, m := range secretArgRe.FindAllStringSubmatch(src, -1) {
		add(m[1])
	}
	return out
}

func escapeArg(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
