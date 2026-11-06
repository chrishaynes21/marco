package main

// WHAT A TYPED LINE MEANS, READ BACK BEFORE ENTER RUNS IT (S6 of
// docs/Surfaces-alignment-proposal-2026-11-02.md, §5.4).
//
// `marco line --json -- <line>` answers what the widget's command line would run, without running
// anything: which routine, which values, which are missing, and the read-back a person sees before
// Enter. The widget draws the answer and decides nothing about it. What it then runs is the
// answer's IDENTITY and VERSION: `marco do --play=… --app=… [--focus] --expect=<source_at>
// --arg=name=value…`, so the words are not read again and a routine saved since the read-back
// refuses at the door.
//
// # The order, and why each step is here
//
//  1. The whole line as a routine's name (`Registry.Resolve`, exact). AN EXACT NAME WINS: with
//     routines `cloud pull` and `cloud pull stone`, the line `cloud pull stone` is the second one
//     and has no values. This is the intake's whole-phrase-first order, kept.
//  2. The invocation grammar the intake already has (`routes.ParseInvocation`): `name:value`
//     tokens, or ` with a, b`. Unchanged.
//  3. NEW, and only on this command line: the line as a routine's name followed by its value,
//     `cloud pull stone`. The longest leading run of words that names a routine taking an argument
//     is the routine. Only for a routine that takes ONE value: with two or more, the line is
//     ASKED ABOUT and nothing is guessed, because nothing in `greet sam see you at 8` says where
//     the name ends and the message begins.
//  4. Nothing: the line goes to the intake as words, as it always has, and Director reads it.
//
// Step 3 is not added to `marco do`. A spoken or scripted phrase keeps the intake's own grammar,
// and this command line is the one place the read-back is on screen before anything runs.
//
// # Values Director proposed
//
// A line read with a routine's identity and the values Director proposed for it
// (`--play/--app/--focus`, `--proposed=name=value`, `--proposed-at=<digest>`) is read through
// `PrepareProposed`, the same contract Studio's review uses. So a value left as proposed is labelled
// as read out of what was said, one the person changed is labelled as theirs, and a proposal made
// for an earlier revision of the routine refuses. A line that no longer names that routine drops the
// proposals: they were about that routine and no other.
//
// Deleting step 1's precedence must fail TestAnExactNameWinsOverATemplate.

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// lineReading is `marco line --json`.
type lineReading struct {
	Line string `json:"line"`
	// Kind is "routine" when the line names one, and "none" when it goes to the intake as
	// words.
	Kind  string `json:"kind"`
	ID    string `json:"id,omitempty"`
	App   string `json:"app,omitempty"`
	Slug  string `json:"slug,omitempty"`
	Focus bool   `json:"focus,omitempty"`
	Name  string `json:"name,omitempty"`
	// Form is how the line named it: exact, named, with, bare.
	Form string `json:"form,omitempty"`
	// Needs is whether the routine takes a value anybody types.
	Needs bool `json:"needs,omitempty"`
	// Values are what would run, by argument name, and From how each arrived.
	Values map[string]string `json:"values,omitempty"`
	From   map[string]string `json:"from,omitempty"`
	// Parts is the read-back, one per argument, in the routine's order.
	Parts   []linePart `json:"parts,omitempty"`
	Missing []string   `json:"missing,omitempty"`
	// Pending is what to show, muted, after what has been typed: `‹resource›`.
	Pending string `json:"pending,omitempty"`
	// Readback is the whole read-back as one line, for the history.
	Readback string `json:"readback,omitempty"`
	// Says is why it cannot run as it stands: a value needed, values that must be named, the
	// contract's own refusal. Empty when it can.
	Says     string `json:"says,omitempty"`
	Runnable bool   `json:"runnable,omitempty"`
	// SourceAt is the version the read-back was read from. It travels as `--expect`.
	SourceAt string `json:"source_at,omitempty"`
}

// linePart is one argument in the read-back.
type linePart struct {
	Label  string `json:"label"`
	Value  string `json:"value,omitempty"`
	From   string `json:"from,omitempty"`
	Secret bool   `json:"secret,omitempty"`
	Said   string `json:"said,omitempty"` // "(read out of what you said)", "(as you corrected it)"
}

// lineProposal is what Director proposed, carried with the identity it was proposed for.
type lineProposal struct {
	Route    routes.Route
	Values   map[string]string
	At       string
	Carrying bool
}

// runLine: `marco line --json [--play=<slug> [--app=<app>] [--focus] --proposed=name=value…
// --proposed-at=<digest>] -- <line>`.
func runLine(args []string) {
	line, p, ok := lineArgs(args)
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: marco line --json [--play=<slug> [--app=<app>] [--focus] "+
			"--proposed=name=value… --proposed-at=<digest>] -- <line>")
		os.Exit(2)
	}
	_ = json.NewEncoder(os.Stdout).Encode(readLine(newDeps(), winctx.Active(), line, p))
}

// lineArgs reads runLine's arguments. Everything after `--` is the line, so nothing typed can be
// read as a flag.
func lineArgs(args []string) (line string, p lineProposal, ok bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return strings.TrimSpace(strings.Join(args[i+1:], " ")), p, true
		case a == "--json":
		case strings.HasPrefix(a, "--play="):
			p.Route.Slug, p.Carrying = routes.Slug(strings.TrimPrefix(a, "--play=")), true
		case strings.HasPrefix(a, "--app="):
			p.Route.App = strings.TrimPrefix(a, "--app=")
		case a == "--focus":
			p.Route.Focus = true
		case strings.HasPrefix(a, "--proposed="):
			name, value, found := strings.Cut(strings.TrimPrefix(a, "--proposed="), "=")
			if !found || strings.TrimSpace(name) == "" {
				return "", p, false
			}
			if p.Values == nil {
				p.Values = map[string]string{}
			}
			p.Values[strings.ToLower(strings.TrimSpace(name))] = value
		case strings.HasPrefix(a, "--proposed-at="):
			p.At = strings.TrimPrefix(a, "--proposed-at=")
		default:
			return "", p, false
		}
	}
	return "", p, false
}

// readLine is the whole reading, with the registry and the foreground handed in.
func readLine(d orchestrator.Deps, app, line string, p lineProposal) lineReading {
	line = strings.TrimSpace(line)
	out := lineReading{Line: line, Kind: "none"}
	if line == "" {
		return out
	}
	rt, form, named, positional, ok := lineRoutine(d, app, line)
	if !ok {
		return out
	}
	raw, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		return out
	}
	src := string(raw)
	out.Kind, out.ID, out.App, out.Slug, out.Focus, out.Form = "routine", rt.ID(), rt.App, rt.Slug, rt.Focus, form
	out.Name = orchestrator.NameIn(src, rt.Slug)
	if out.Name == "" {
		out.Name = plays.Pretty(rt.Slug)
	}
	args := routes.Arguments(src)
	var typed []routes.Argument
	for _, a := range args {
		if !a.Secret {
			typed = append(typed, a)
		}
	}
	out.Needs = len(typed) > 0

	// A LINE OF WORDS AFTER A ROUTINE THAT TAKES SEVERAL VALUES IS ASKED ABOUT.
	if form == "bare" && len(typed) > 1 {
		out.Says = fmt.Sprintf("%s values needed — say which: %s", countWord(len(typed)),
			out.Name+" "+namedSlots(typed))
		out.Pending = namedSlots(typed)
		return out
	}
	values, why := valuesFor(src, args, typed, form, named, positional)
	if why != "" {
		out.Says = why
		return out
	}
	// THE PROPOSALS ARE FOR ONE ROUTINE, BY ITS WHOLE IDENTITY. A line read with proposals that
	// names another routine (a context routine where they were for its focus namesake, which
	// shares the application and the name) is refused rather than read with the proposals
	// dropped: dropped, the values in the line would read back as typed.
	//
	// Dropping them silently again must fail TestAProposalForAFocusRoutineIsNotReadOnItsNamesake.
	proposed, at := map[string]string(nil), ""
	if p.Carrying {
		if p.Route.ID() != rt.ID() {
			out.Says = "this line is " + rt.ID() + ", not " + p.Route.ID() +
				", the routine those values were proposed for, so nothing would run"
			return out
		}
		proposed, at = p.Values, p.At
	}
	for _, a := range typed {
		if _, ok := values[strings.ToLower(a.Name)]; !ok {
			out.Missing = append(out.Missing, a.Name)
		}
	}
	if len(out.Missing) > 0 {
		out.Pending = pendingSlots(out.Missing, len(typed) == 1 && (form == "exact" || form == "bare"))
		out.Says = neededWord(out.Missing) + " · " + strings.TrimSpace(out.Name+" "+
			pendingSlots(out.Missing, len(typed) == 1))
		out.SourceAt = routes.DigestOf(src)
		out.Parts = partsOf(args, values, nil)
		return out
	}
	i, err := d.PrepareProposed(rt, values, proposed, at)
	if err != nil {
		out.Says = err.Error()
		return out
	}
	out.SourceAt = i.SourceAt
	out.Values, out.From = i.Values, i.From
	out.Parts = partsOf(args, i.Values, i.From)
	out.Readback = readbackOf(out.Name, out.Parts)
	if !i.Runnable() {
		out.Says = i.Why
		return out
	}
	out.Runnable = true
	return out
}

// lineRoutine finds the routine a line names, in the order the file comment gives.
func lineRoutine(d orchestrator.Deps, app, line string) (rt routes.Route, form string,
	named map[string]string, positional []string, ok bool) {

	if rt, ok := d.Reg.Resolve(app, line); ok {
		return rt, "exact", nil, nil, true
	}
	if base, n, pos := routes.ParseInvocation(line); base != line {
		if rt, ok := d.Reg.Resolve(app, base); ok {
			if len(n) > 0 {
				return rt, "named", n, nil, true
			}
			return rt, "with", nil, pos, true
		}
		// A line with a label or a ` with ` in it that names nothing is not read as
		// words after a routine's name: the label would become part of the value.
		return routes.Route{}, "", nil, nil, false
	}
	words := strings.Fields(line)
	for n := len(words) - 1; n >= 1; n-- {
		base := strings.Join(words[:n], " ")
		rt, ok := d.Reg.Resolve(app, base)
		if !ok || !takesText(d, rt) {
			continue
		}
		// THE VALUE IS THE REST OF THE LINE AS TYPED, spaces and all, from the first
		// character after the routine's last word.
		rest := line
		for _, w := range words[:n] {
			rest = strings.TrimLeft(rest, " \t")
			rest = rest[len(w):]
		}
		return rt, "bare", nil, []string{strings.TrimSpace(rest)}, true
	}
	return routes.Route{}, "", nil, nil, false
}

// takesText is whether a routine takes a value anybody types.
func takesText(d orchestrator.Deps, rt routes.Route) bool {
	raw, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		return false
	}
	for _, a := range routes.Arguments(string(raw)) {
		if !a.Secret {
			return true
		}
	}
	return false
}

// valuesFor puts the line's values against the routine's names. Positional values go in the
// routine's own order, as the intake's grammar puts them (`routes.ArgNames`), and one that would
// land on a value the credential store answers is refused rather than dropped.
func valuesFor(src string, args, typed []routes.Argument, form string, named map[string]string,
	positional []string) (map[string]string, string) {

	values := map[string]string{}
	for k, v := range named {
		values[strings.ToLower(k)] = v
	}
	if form == "bare" && len(typed) == 1 {
		values[strings.ToLower(typed[0].Name)] = positional[0]
		return values, ""
	}
	if len(positional) > 0 {
		order := routes.ArgNames(src)
		if len(positional) > len(order) {
			return nil, fmt.Sprintf("%d values were given and this routine takes %d — name them: %s",
				len(positional), len(typed), namedSlots(typed))
		}
		secret := map[string]bool{}
		for _, a := range args {
			secret[strings.ToLower(a.Name)] = a.Secret
		}
		for i, v := range positional {
			name := strings.ToLower(order[i])
			if secret[name] {
				return nil, fmt.Sprintf("%s is read from the credential store, so it is not "+
					"typed here — name the values instead: %s", order[i], namedSlots(typed))
			}
			values[name] = v
		}
	}
	return values, ""
}

// partsOf is the read-back, one part per argument in the routine's order.
func partsOf(args []routes.Argument, values, from map[string]string) []linePart {
	var parts []linePart
	for _, a := range args {
		k := strings.ToLower(a.Name)
		if a.Secret {
			parts = append(parts, linePart{Label: a.Name, Secret: true, Said: "from the credential store"})
			continue
		}
		v, ok := values[k]
		if !ok {
			continue
		}
		parts = append(parts, linePart{Label: a.Name, Value: v, From: from[k], Said: saidClause(from[k])})
	}
	return parts
}

// saidClause is the contract's provenance clause, compact: only for a value somebody could take
// for their own and is not, or was not.
func saidClause(from string) string {
	switch from {
	case routes.FromExtracted:
		return "(read out of what you said)"
	case routes.FromEdited:
		return "(as you corrected it)"
	}
	return ""
}

// readbackOf is the read-back on one line: `Cloud pull · resource: stone`.
func readbackOf(name string, parts []linePart) string {
	var b strings.Builder
	b.WriteString(capitalised(name))
	for _, p := range parts {
		b.WriteString(" · " + p.Label + ": ")
		switch {
		case p.Secret:
			b.WriteString(p.Said)
			continue
		case p.Value == "":
			b.WriteString("nothing")
		default:
			b.WriteString(p.Value)
		}
		if p.Said != "" {
			b.WriteString(" " + p.Said)
		}
	}
	return b.String()
}

func capitalised(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// namedSlots is `name:‹name› message:‹message›`.
func namedSlots(args []routes.Argument) string {
	names := make([]string, 0, len(args))
	for _, a := range args {
		names = append(names, a.Name)
	}
	return pendingSlots(names, false)
}

// pendingSlots is the values still to type: `‹resource›` for a routine's one value, and
// `name:‹name›` for each when there are several (or labels have been used).
func pendingSlots(names []string, bare bool) string { return routes.Slots(names, bare) }

// neededWord is `Resource needed`, `Name and message needed`.
func neededWord(names []string) string {
	var s string
	switch len(names) {
	case 1:
		s = names[0]
	case 2:
		s = names[0] + " and " + names[1]
	default:
		s = strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
	return capitalised(s) + " needed"
}

func countWord(n int) string {
	if w := [...]string{"", "One", "Two", "Three", "Four", "Five"}; n < len(w) {
		return w[n]
	}
	return fmt.Sprint(n)
}

// typedArgs is how many values a routine takes as text, not counting the ones the credential store
// answers; -1 when its source cannot be read.
func typedArgs(d orchestrator.Deps, rt routes.Route) int {
	raw, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		return -1
	}
	n := 0
	for _, a := range routes.Arguments(string(raw)) {
		if !a.Secret {
			n++
		}
	}
	return n
}
