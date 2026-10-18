package demo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// THE SAVED SEQUENCES, BY THE NAME SOMEBODY GAVE THEM.
//
// A third directory in the same store, for the same reason the first two are separate: a
// demonstration is what happened, a learned procedure is a way of achieving a goal, and a
// sequence is a named routine. They have different lifetimes and different answers to "what is
// this a claim about", and one directory holding all three would need a discriminator field that
// means exactly what the directory name already means.
//
// # Why this is a file per name and not an index
//
// Because the file IS the artifact, readable on its own, and `director perform-workflow
// --workflow <path>` has always taken one. A named sequence is therefore the same artifact that
// command loads, in a place a name can find it — not a new format and not a wrapper around one.
//
// # The name is the person's, and the file name is not
//
// `safeName` already exists here and already flattens a name to something a filesystem accepts.
// The ARTIFACT keeps the name as typed, because that is what a person has to see when Marco
// lists what it knows; the file name is an implementation detail of where it was put. Two names
// that flatten to one file name would collide, and `Save` refuses rather than overwriting
// somebody's routine with a differently-capitalised one.
//
// Enforced by TestASavedSequenceIsFoundByItsName and TestTwoNamesThatFlattenToOneFileCollide.

const sequencesDir = "sequences"

// SaveSequence writes one named sequence, refusing to replace a different name's file.
func (s *Store) SaveSequence(a workflowstore.Artifact) error {
	if strings.TrimSpace(a.Name) == "" {
		return fmt.Errorf("a sequence with no name cannot be stored")
	}
	if err := os.MkdirAll(filepath.Join(s.dir, sequencesDir), 0o755); err != nil {
		return fmt.Errorf("demo: preparing %s: %w", sequencesDir, err)
	}
	// A DIFFERENT NAME IN THE SAME FILE is a collision, not an update. Replacing a
	// routine somebody saved because their new name flattens to the same characters is
	// the kind of loss that is noticed weeks later.
	if existing, ok := s.Sequence(a.Name); ok && !strings.EqualFold(existing.Name, a.Name) {
		return fmt.Errorf("%q and %q would be kept in the same place; "+
			"give one of them a more distinct name", a.Name, existing.Name)
	}
	body, err := a.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(sequencesDir, safeName(a.Name)), body, 0o644)
}

// Sequence reads one back by the name it was saved under, case-insensitively.
//
// Case-insensitively because a person asking for "my settings tour" is asking for the thing they
// called "My Settings Tour", and a store that distinguished them would be a store that lost
// things.
func (s *Store) Sequence(name string) (workflowstore.Artifact, bool) {
	raw, err := os.ReadFile(s.path(sequencesDir, safeName(name)))
	if err != nil {
		return workflowstore.Artifact{}, false
	}
	// THROUGH THE LOADER, so a file this build cannot vouch for is reported as absent
	// rather than half-read. `Unmarshal` refuses an unknown version for exactly this
	// reason, and a sequence that silently lost a field would replay as something else.
	a, err := workflowstore.Unmarshal(raw)
	if err != nil {
		return workflowstore.Artifact{}, false
	}
	return a, true
}

// SequenceProblem is why a named sequence could not be read, for a caller that has to say so.
//
// Separate from Sequence because most callers want "is there one", and the one that has to tell
// a person why their saved routine stopped working wants the sentence. A missing file is not a
// problem — it is the ordinary answer to a name nobody saved.
func (s *Store) SequenceProblem(name string) string {
	raw, err := os.ReadFile(s.path(sequencesDir, safeName(name)))
	if err != nil {
		return ""
	}
	if _, err := workflowstore.Unmarshal(raw); err != nil {
		return err.Error()
	}
	return ""
}

// Sequences lists what has been saved, by name, in alphabetical order.
func (s *Store) Sequences() []workflowstore.Artifact {
	entries, err := os.ReadDir(filepath.Join(s.dir, sequencesDir))
	if err != nil {
		return nil
	}
	var out []workflowstore.Artifact
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(s.dir, sequencesDir, e.Name()))
		if rerr != nil {
			continue
		}
		var a workflowstore.Artifact
		if json.Unmarshal(raw, &a) == nil && a.Name != "" {
			out = append(out, a)
		}
	}
	// By name, because that is how a person asks for them.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && strings.ToLower(out[j].Name) < strings.ToLower(out[j-1].Name); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// ForgetSequence removes one.
func (s *Store) ForgetSequence(name string) error {
	path := s.path(sequencesDir, safeName(name))
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("there is no saved sequence called %q", name)
	}
	return os.Remove(path)
}

// ── asking for one by name ────────────────────────────────────────────────────

// MostRunsOfASequence is the hard ceiling on a bounded repeat.
//
// Twenty-five, and the number matters less than the fact that there is one: a routine asked for
// "a hundred times" by a slip of the keyboard is a hundred times somebody's desktop is driven.
// The ordinary step ceiling in `policy` catches it too, and this catches it with a sentence
// about runs rather than about steps.
const MostRunsOfASequence = 25

// runPrefixes are the ways of asking for a saved routine by name.
//
// Short, and deliberately not extended with synonyms. The prefix is not what makes this safe —
// what makes it safe is that the REST of the phrase must be the name of something somebody
// saved. "run the installer" asks for a sequence only if there is a sequence called "the
// installer", and otherwise falls through to the ordinary request path untouched.
//
//	Named macros can use a separate invocation path, such as "Marco, run my settings tour."
//	Do not add broad goal patterns that hijack existing UI commands.
var runPrefixes = []string{"run ", "replay ", "run the ", "replay the "}

// countSuffix matches a bounded repeat on the end of a request: "3 times", "five more times".
var countSuffix = regexp.MustCompile(
	`\s+(\d+|one|two|three|four|five|six|seven|eight|nine|ten)\s*(?:more\s+)?times?$`)

var numberWords = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
}

// Asked reports the saved sequence a request asks to run, and how many times.
//
// The LOOKUP is part of the parse, which is the whole design. A prefix table that decided this
// on words alone would claim every sentence beginning with "run", and the ordinary request path
// would stop seeing them. Here, a request is a sequence invocation only when a sequence of that
// name exists — so adding this took nothing away.
//
// Enforced by TestRunningANameNobodySavedIsAnOrdinaryRequest.
func (s *Store) Asked(request string) (workflowstore.Artifact, int, bool) {
	rest := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(request), ".!?,"))
	lower := strings.ToLower(rest)
	// LONGEST PREFIX FIRST, so "run the tour" prefers "the tour" over " tour" only when
	// nothing is called "the tour". Both are tried, below.
	var tails []string
	for _, p := range runPrefixes {
		if strings.HasPrefix(lower, p) {
			tails = append(tails, strings.TrimSpace(rest[len(p):]))
		}
	}
	if len(tails) == 0 {
		return workflowstore.Artifact{}, 0, false
	}
	sort.Slice(tails, func(i, j int) bool { return len(tails[i]) < len(tails[j]) })
	for _, tail := range tails {
		// WITH THE COUNT STRIPPED FIRST, because "my settings tour 3 times" is a
		// request for a sequence called "my settings tour" — and then without, because
		// somebody is entitled to call a routine "my three times table".
		for _, attempt := range []struct {
			name  string
			times int
		}{
			{stripCount(tail), countIn(tail)},
			{tail, 1},
		} {
			if attempt.name == "" {
				continue
			}
			if a, ok := s.Sequence(attempt.name); ok {
				return a, attempt.times, true
			}
		}
	}
	return workflowstore.Artifact{}, 0, false
}

// stripCount is the phrase without its trailing repeat count.
func stripCount(s string) string {
	return strings.TrimSpace(countSuffix.ReplaceAllString(s, ""))
}

// countIn is the repeat count a phrase asked for, or one.
//
// One, never zero and never unbounded: "run my tour" means run it, and a default of "forever"
// is the thing an agent must not be able to acquire by omission.
func countIn(s string) int {
	m := countSuffix.FindStringSubmatch(strings.ToLower(s))
	if m == nil {
		return 1
	}
	if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
		return n
	}
	if n, ok := numberWords[m[1]]; ok {
		return n
	}
	return 1
}

// ExpandRequest answers whether a request asks for a saved sequence, and what program it is.
//
// The one method the execution pipeline needs, so that nothing there has to know about this
// store, this format or this parse. False means "not a sequence request", and the request goes
// on to the ordinary path exactly as it did before.
//
// Enforced by TestASequenceRequestBecomesAnOrdinaryProgram.
func (s *Store) ExpandRequest(request string, b goal.Binder) (goal.Expansion, bool, error) {
	a, times, ok := s.Asked(request)
	if !ok {
		return goal.Expansion{}, false, nil
	}
	ex, err := ExpandSequence(a, times, b)
	if err != nil {
		return goal.Expansion{}, true, err
	}
	return ex, true, nil
}
