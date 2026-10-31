package routes

import (
	"fmt"
	"regexp"
	"sort"
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

// SplitArgs splits a LEARN name (or positional invocation) on " with ": "say hello
// with name" → ("say hello", ["name"]); "dm with person, message" → ("dm",
// ["person","message"]). At learn time the list is the declared arg names; in the
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
	byPos := zipPositional(src, positional)
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

// ── WHAT A ROUTINE NEEDS, AND WHETHER IT HAS IT ──────────────────────────────

// Argument is one value a routine accepts, and where that value comes from.
//
// TWO KINDS, AND A SURFACE MUST TELL THEM APART. `ArgNames` returns both in one list, so anything
// that read it as uniformly "text to type" would put a password in a text box. A secret-typed
// argument is not typed by whoever presses Play: `oshost.doSecret` reads it from the credential
// store, and when the store does not have it the run refuses and says how to set it.
type Argument struct {
	Name string
	// Secret is whether it is read from the credential store rather than supplied as text.
	Secret bool
}

// How a value for an argument ARRIVED. Four words, and they are the only four.
//
// # Why this is written down rather than described per surface
//
// `resolveroute.ResolverSentence` exists because "the configured resolver answered this" about a
// free offline match is the false impression that whole sentence was written to prevent. With one
// value read out of a phrase by a grammar and another proposed by a model, ONE sentence about the
// request would be that same false impression per argument — so each value says how it arrived,
// and a person can see which of the two it is before they press anything.
//
// Here rather than in either package that needs them, because `internal/orchestrator` and
// `internal/resolveroute` do not import one another and both import this: a vocabulary kept in two
// places is two vocabularies the first time either changes.
//
// AND NONE OF THESE WORDS CARRIES THE VALUE. The provenance says where a value came from; the
// confirmation says what it is. `internal/director/values/provenance.go` is the shape being
// followed, being *designed to be safe rather than redacted to be safe*.
const (
	// FromTyped is a value given as a flag or typed into a field.
	FromTyped = "typed"
	// FromParsed is a value the deterministic grammar read out of a phrase.
	FromParsed = "parsed"
	// FromExtracted is a value a model proposed by naming a span of the request.
	FromExtracted = "extracted"
	// FromEdited is a proposal the person corrected, which is THEIRS now and must not go
	// on being credited to the model.
	FromEdited = "edited"
)

// Arguments is what a routine accepts, with each one's kind.
//
// The same order `ArgNames` has always reported — placeholders first, then route-qualified secret
// arguments — because the overlay's labels and `marco args` are built from it and an order that
// changed would reorder what people read.
func Arguments(src string) []Argument {
	var out []Argument
	seen := map[string]bool{}
	add := func(name string, secret bool) {
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, Argument{Name: name, Secret: secret})
		}
	}
	for _, m := range phRe.FindAllStringSubmatch(src, -1) {
		add(m[1], false)
	}
	for _, m := range secretArgRe.FindAllStringSubmatch(src, -1) {
		add(m[1], true)
	}
	return out
}

// zipPositional maps positional values onto the declared argument names, in source order.
//
// ONE OWNER, because `ApplyArgs` does this to fill the placeholders and a check that answers
// "was a value supplied" has to agree with it exactly. Two copies would disagree the first time
// either changed, and the disagreement would read as "it refused and then typed it anyway".
func zipPositional(src string, positional []string) map[string]string {
	out := map[string]string{}
	for i, name := range ArgNames(src) {
		if i < len(positional) {
			out[strings.ToLower(name)] = positional[i]
		}
	}
	return out
}

// ValuesNeeded is the refusal that a routine types text and nobody supplied it.
//
// # Why this one refusal has a type
//
// Every other thing `CheckArgs` says is the same wherever it is said: a name the routine does not
// take is wrong at every entrance, and so is a value with a line break in it. THIS one has an
// answer that depends on WHERE THE REQUEST CAME FROM. A surface that can show what a routine would
// type before typing it asks the person for the value; one that cannot has to say so and send them
// somewhere that can — and neither of those sentences belongs in this package, which knows about
// routines and nothing about windows.
//
// So the refusal is recognisable rather than re-worded. Matching on the sentence would work until
// somebody improved the wording, and the thing that broke would be a surface going quiet.
//
// The NAMES are carried and the values are not, for the reason the whole of `CheckArgs` gives.
type ValuesNeeded struct {
	// Names is the arguments nothing supplied a value for, in the routine's own order.
	Names []string
}

func (e *ValuesNeeded) Error() string {
	return fmt.Sprintf("this routine types text that has to be supplied, and no value "+
		"was given for %s", joinArgs(e.Names))
}

// CheckArgs says why a routine cannot be run with these values, or nil when it can.
//
// # Missing is not empty
//
// The test is whether a value was SUPPLIED, which a map answers and a length never can. An empty
// string somebody supplied on purpose is a value — clearing a field is a thing people do, and a
// routine that types nothing where they asked for nothing is correct. `len(v) == 0` would refuse
// exactly the case they meant.
//
// And a value is used as supplied: not trimmed, not collapsed. A leading space in a typed field is
// text somebody typed.
//
// # And the names are checked both ways
//
// A value for a name the routine does not take is REFUSED rather than dropped: it is somebody
// expecting text to appear somewhere, and the routine has nowhere to put it. That is the rule
// `performLearned` already applies to a play with no arguments at all, generalised — and its
// reason is the same: dropping them silently would let somebody watch the wrong thing happen and
// believe they had asked for it.
//
// A SECRET-typed argument is NOT required here. It comes from the credential store, and a run that
// cannot find it refuses at the moment it is needed with the sentence that says how to set it.
// Whether a value may be OFFERED for one is a question about the entrance that offered it, not
// about this routine, so it is not answered here.
//
// # No value appears in anything this returns
//
// The argument is named; its text is not. These sentences reach logs, terminals and run records,
// and a value that is somebody's message or address does not belong in any of them.
func CheckArgs(src string, named map[string]string, positional []string) error {
	args := Arguments(src)
	known := map[string]bool{}
	for _, a := range args {
		known[strings.ToLower(a.Name)] = true
	}
	var unknown []string
	for k := range named {
		if !known[strings.ToLower(k)] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("this routine does not take %s, so there is nowhere for that "+
			"text to go%s", joinArgs(unknown), takesWhat(args))
	}
	if len(positional) > len(args) {
		return fmt.Errorf("%d values were given and this routine takes %d%s",
			len(positional), len(args), takesWhat(args))
	}
	byPos := zipPositional(src, positional)
	// ── AND A VALUE THIS SUBSTITUTION CANNOT WRITE DOWN ──
	//
	// `escapeArg` escapes a backslash and a quotation mark and nothing else, so a value
	// carrying a carriage return or a line feed produces a Marco string literal that does
	// not lex: `lex: unterminated string literal`, and the routine fails at its own first
	// line having pressed nothing.
	//
	// THAT IS THE WHOLE OF THE REASON, and it is a fact about the substitution as it
	// stands rather than a claim about demonstrations. Whether a routine should be able to
	// type several lines is a separate question with its own answer; whoever lifts this
	// refusal should know they are lifting an escaping limit.
	//
	// Refused rather than escaped, in this increment, because an escape nobody has
	// exercised is how the two replacements above came to look sufficient.
	for _, a := range args {
		v, ok := named[strings.ToLower(a.Name)]
		if !ok {
			v, ok = byPos[strings.ToLower(a.Name)]
		}
		if ok && strings.ContainsAny(v, "\r\n") {
			return fmt.Errorf("the value given for %q runs over more than one line, "+
				"and a routine cannot type that yet", a.Name)
		}
	}
	var missing []string
	for _, a := range args {
		if a.Secret {
			continue
		}
		k := strings.ToLower(a.Name)
		if _, ok := named[k]; ok {
			continue
		}
		if _, ok := byPos[k]; ok {
			continue
		}
		missing = append(missing, a.Name)
	}
	if len(missing) > 0 {
		return &ValuesNeeded{Names: missing}
	}
	return nil
}

// takesWhat names what a routine does accept, for a sentence that has just said what it does not.
func takesWhat(args []Argument) string {
	var names []string
	for _, a := range args {
		if a.Secret {
			names = append(names, a.Name+" (from the credential store)")
			continue
		}
		names = append(names, a.Name)
	}
	if len(names) == 0 {
		return " — it takes none"
	}
	return " — it takes " + joinArgs(names)
}

// joinArgs is a list in a sentence, with each name quoted so two of them cannot read as one.
func joinArgs(in []string) string {
	quoted := make([]string, 0, len(in))
	for _, s := range in {
		quoted = append(quoted, fmt.Sprintf("%q", s))
	}
	switch len(quoted) {
	case 0:
		return ""
	case 1:
		return quoted[0]
	}
	return strings.Join(quoted[:len(quoted)-1], ", ") + " and " + quoted[len(quoted)-1]
}

// CheckFlagged says why a value may not be offered through an explicit flag, or nil when it may.
//
// # One entrance may offer what another may not
//
// A phrase carrying `password:value` has always reached `oshost.doSecret`, which uses it AND
// REMEMBERS it in the credential store. Whatever is thought of that, it is not something to
// acquire a second time through a flag that a window fills in and a command line carries — so a
// value offered for a SECRET-typed argument through a flag is refused, and the sentence says where
// that value is supposed to come from.
//
// The phrase path is untouched. This is about the entrance, which is why it takes the names that
// arrived as flags rather than the values: nothing here needs to see one.
func CheckFlagged(src string, flagged []string) error {
	if len(flagged) == 0 {
		return nil
	}
	secret := map[string]bool{}
	for _, a := range Arguments(src) {
		if a.Secret {
			secret[strings.ToLower(a.Name)] = true
		}
	}
	var refused []string
	for _, name := range flagged {
		if secret[strings.ToLower(name)] {
			refused = append(refused, name)
		}
	}
	if len(refused) == 0 {
		return nil
	}
	sort.Strings(refused)
	return fmt.Errorf("%s cannot be given here: it is read from the credential store, so "+
		"set it once with `marco secret set` and this routine will find it",
		joinArgs(refused))
}
