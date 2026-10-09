package program

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit/planner"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Decompose turns one request into an ordered program.
//
// Simple ordered decomposition and nothing more. It splits on the conjunctions people
// actually use to sequence instructions — "and then", "then", "and", ";", "," — parses
// each clause with the ordinary intent parser, and rejects the whole request if any
// clause is not something the Director implements.
//
// It does NOT infer. It does not reorder, insert a focus step it thinks is needed, or
// decide that "save the file" means Ctrl+S. Every step in the result corresponds to a
// clause the user actually said, in the order they said it, which is what makes
// `director plan` an honest preview rather than a guess about what might happen.
//
// parse is injected rather than imported so this package does not depend on the intent
// parser — the same reason everything else in the Director takes its collaborators.
func Decompose(request string, parse func(string) directorapi.Intent) (Program, error) {
	req := strings.TrimSpace(request)
	if req == "" {
		return Program{Status: StatusRejected}, fmt.Errorf("nothing was asked")
	}

	// Control flow is checked against the WHOLE request before splitting, because a
	// conjunction inside a conditional ("if it is open, click Save and close it")
	// would otherwise be split into clauses that each look unconditional.
	lower := strings.ToLower(" " + req + " ")
	for _, w := range controlFlow {
		if strings.Contains(lower, w) {
			return Program{Status: StatusRejected}, fmt.Errorf(
				"this reads as a condition (%q). The Director executes sequences, not "+
					"branches, so it is rejected rather than run unconditionally",
				strings.TrimSpace(w))
		}
	}

	clauses := Split(req)
	if len(clauses) > MaxSteps {
		return Program{Status: StatusRejected}, fmt.Errorf(
			"the request needs %d steps and the limit is %d; it is rejected rather than "+
				"shortened, because running part of it would do something you did not ask for",
			len(clauses), MaxSteps)
	}

	prog := Program{Goal: req, Status: StatusPlanned}
	for i, clause := range clauses {
		in := parse(clause)
		prog.Steps = append(prog.Steps, Step{
			ID:            StepID(fmt.Sprintf("s%d", i+1)),
			Operation:     in,
			Phrase:        clause,
			Verification:  requirementFor(in),
			FailurePolicy: Stop,
		})
	}
	if err := Validate(prog); err != nil {
		prog.Status = StatusRejected
		return prog, err
	}
	return prog, nil
}

// separators are the conjunctions that sequence instructions, longest first.
//
// Order matters: " and then " must be tried before " and " and before " then ", or a
// single conjunction becomes two empty clauses.
//
// The comma forms are listed because people write them: "add milk, eggs and bread, then change
// milk to oat milk". Without ", then " the comma is not part of the cut, so it stays on the end
// of the first clause and becomes part of what gets typed.
var separators = []string{
	", and then ", " and then ", ", then ", " then ", ", and ", " and ", "; ", ", ",
}

// fragment is a piece of the request with the separator that was cut from in front of it.
//
// THE SEPARATOR IS PART OF THE PERSON'S TEXT until something proves it was a conjunction.
// Rejoining with a hard-coded " and " rewrote it: "add milk, eggs and bread" came back as "add
// milk and eggs and bread", because the comma was cut as a separator and restored as the wrong
// word. The document then held a grocery list nobody had written.
//
// This is the quietest class of defect this project has: no step fails, nothing refuses, and the
// text that lands is a plausible version of what was asked for. It is only visible by reading
// the result against the request character for character.
type fragment struct {
	text string
	// sep is what was removed from in front of text, verbatim, and is put back exactly when
	// the fragment turns out to be the tail of the clause before it rather than a new one.
	sep string
}

// Split breaks a request into ordered clauses.
//
// The hard part is that "and" is not always a separator. "Type hello and goodbye"
// is one instruction whose TEXT contains "and", and splitting it would type "hello"
// and then fail to understand "goodbye". So a split is refused when it would cut
// inside a quoted run, and a clause that does not begin with a verb-like word is
// rejoined to the one before it — with the separator it was cut on, not a substitute.
func Split(request string) []string {
	frags := []fragment{{text: request}}
	for _, sep := range separators {
		var next []fragment
		for _, f := range frags {
			for i, p := range splitOutsideQuotes(f.text, sep) {
				// The first piece keeps whatever preceded the fragment it came from;
				// every later one was preceded by this separator.
				cut := f.sep
				if i > 0 {
					cut = sep
				}
				next = append(next, fragment{text: p, sep: cut})
			}
		}
		frags = next
	}

	var out []string
	for _, f := range frags {
		p := strings.TrimSpace(f.text)
		p = strings.TrimSuffix(p, ".")
		if p == "" {
			continue
		}
		// A fragment that does not start like an instruction belongs to the clause
		// before it. "type hello and goodbye" splits to ["type hello", "goodbye"],
		// and "goodbye" is not an instruction — it is the rest of the text.
		if len(out) > 0 && !startsLikeInstruction(p) {
			out[len(out)-1] = out[len(out)-1] + f.sep + p
			continue
		}
		out = append(out, p)
	}
	return out
}

// splitOutsideQuotes splits on sep, ignoring occurrences inside quotes.
//
// Text the user quoted is DATA. "type \"save and exit\" into the box" contains a
// conjunction that is part of what they want typed, and cutting there would type half
// of it and then try to execute the other half as an instruction.
// An APOSTROPHE is not a quote. "remember this field's value as email and then type
// ${email}" contains one, and treating it as an opening quote swallowed every
// conjunction after it — the whole request became a single clause whose "name" was
// "email and then type ${email}". So a single quote only delimits when it sits at a word
// boundary, which is what tells `'save and exit'` from `field's`.
func splitOutsideQuotes(s, sep string) []string {
	var out []string
	var quote byte // 0 when outside a quoted run, else the character that opened it
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote && closesQuote(s, i) {
				quote = 0
			}
			continue
		case c == '"':
			quote = c
			continue
		case c == '\'' && opensQuote(s, i):
			quote = c
			continue
		}
		if strings.HasPrefix(strings.ToLower(s[i:]), sep) {
			out = append(out, s[start:i])
			i += len(sep) - 1
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// opensQuote reports whether the quote at i begins a quoted run.
//
// At the start, or after a space or an opening bracket. An apostrophe inside a word —
// "field's", "don't", "window's" — never opens one.
func opensQuote(s string, i int) bool {
	if i == 0 {
		return true
	}
	switch s[i-1] {
	case ' ', '\t', '(', '[':
		return true
	}
	return false
}

// closesQuote reports whether the quote at i ends the run it is in.
//
// At the end, or before a space or ordinary punctuation. The trailing apostrophe of a
// plural possessive ("the users' names") therefore does not close a run that an
// apostrophe never opened.
func closesQuote(s string, i int) bool {
	if i == len(s)-1 {
		return true
	}
	switch s[i+1] {
	case ' ', '\t', ',', '.', ';', ':', '!', '?', ')', ']':
		return true
	}
	return false
}

// instructionStarters are the words a clause must begin with to be its own step.
//
// A closed list rather than "anything the parser accepts", because the parser is
// permissive by design and would happily read "goodbye" as a click target. Requiring a
// verb is what keeps the trailing half of a quoted phrase attached to its own step.
var instructionStarters = map[string]bool{
	"click": true, "press": true, "push": true, "tap": true, "activate": true,
	"choose": true, "select": true, "focus": true, "move": true, "open": true,
	"type": true, "enter": true, "write": true, "put": true, "clear": true,
	"empty": true, "add": true, "append": true, "replace": true, "copy": true,
	"paste": true, "undo": true, "redo": true, "submit": true, "commit": true,
	"remember": true, "forget": true, "read": true,
	"hit": true, "wait": true, "maximize": true, "maximise": true,
	"minimize": true, "minimise": true, "restore": true,
	// Instruction words the Director does NOT implement. Listed on purpose: a clause
	// beginning with one of these is a real instruction, so it becomes its own step
	// and validation rejects the whole request. Leaving them out would let
	// "click Save and scroll down" quietly rejoin into a click on a control labelled
	// "Save and scroll down" — harmless, because resolution then fails, but it tells
	// the user the wrong thing about why.
	"scroll": true, "drag": true, "close": true, "delete": true, "rename": true,
}

// starters is instructionStarters UNION the editing vocabulary, built once.
//
// THE HAND LIST WAS A SECOND COPY OF A FACT THAT ALREADY EXISTED, and it drifted. "Change" became
// a legal way to ask for a replacement; this file did not know the word; and "add milk, eggs and
// bread, then change milk to oat milk" rejoined into one instruction that appended the words
// "then change milk to oat milk" to the document. The list's own comment had predicted the shape
// of the failure — "harmless, because resolution then fails" — and it was not harmless, because
// the clause it swallowed was text and the step it joined was an append.
//
// Taking the union means a new editing phrasing teaches the splitter by existing. The hand list
// stays for the verbs that are not edits (click, focus, move) and for the deliberate entries
// naming instructions the Director does not implement.
var starters = func() map[string]bool {
	m := make(map[string]bool, len(instructionStarters)+16)
	for w := range instructionStarters {
		m[w] = true
	}
	for _, w := range planner.Verbs() {
		m[w] = true
	}
	return m
}()

func startsLikeInstruction(clause string) bool {
	fields := strings.Fields(strings.ToLower(clause))
	if len(fields) == 0 {
		return false
	}
	return starters[strings.Trim(fields[0], `".,!?`)]
}

// unprovable are the operations whose effect the World Model genuinely cannot see.
//
// A selection is not in the World State. A copy changes the clipboard, not the screen.
// Enter often leaves a trace — a newline, a cleared box — but a search submitted
// without changing the field leaves none, and there is no way to tell that apart from
// an Enter that went nowhere.
//
// Marking these best-effort is NOT a way to make a weak check pass. It is the honest
// statement that no check exists, and it is confined to this closed list: every other
// operation must prove itself or the program stops.
var unprovable = map[string]bool{
	"press_enter": true, "select_all": true, "copy_selection": true,
}

// requirementFor decides how strongly a step must prove itself.
func requirementFor(in directorapi.Intent) VerificationRequirement {
	if op, ok := in.Parameters["operation"].(string); ok && unprovable[op] {
		return VerifyBestEffort
	}
	return VerifyRequired
}
