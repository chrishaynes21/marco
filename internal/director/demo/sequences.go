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
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
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
	// AND THE PICTURES OF A REVISION THIS ONE REPLACES GO FIRST.
	//
	// Before the artifact, for the reason `ForgetSequence` puts them first: a failure to
	// remove them must not leave crops of one recording's screens behind under the name
	// of a routine that is now a different recording. Nothing would read them again — the
	// read side is keyed by revision — and nothing would ever clean them up.
	//
	// This runs on EVERY save and not only where pictures are being written, which is the
	// case the name-keyed version got wrong: re-recording the same name without pictures
	// wrote nothing, cleared nothing, and left the old ones to be shown.
	if err := s.forgetOtherPreviews(a); err != nil {
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
	// AND THE PICTURES OF IT GO WITH IT.
	//
	// Before the artifact, so a failure to remove them refuses the whole thing rather
	// than leaving crops of somebody's screen behind under the name of a routine that no
	// longer exists. Nothing reads them, so they would never be noticed again.
	if err := s.ForgetPreviews(name); err != nil {
		return err
	}
	return os.Remove(path)
}

// ── asking for one by name ────────────────────────────────────────────────────

// MostRunsOfASequence is the hard ceiling on a bounded repeat.
//
// `program.MaxRuns` and not a number of its own, because two numbers for one question are two
// answers to it -- and this one was written as a literal 25 first, beside a literal 25 in the
// replay path and a third in the program validator.
//
// It is still checked HERE as well as there, for the sentence: a refusal at this point can say
// "runs of my settings tour" while the validator can only say how many steps that came to.
const MostRunsOfASequence = program.MaxRuns

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

// ── THE RAW DEMONSTRATION, BESIDE THE ROUTINE IT BELONGS TO ──
//
// A fourth directory, for the reason the other three are separate: an artifact is what Marco
// made of a demonstration and this is the demonstration. They are written together and read
// apart — `director perform-workflow` loads the artifact and must not be able to reach this,
// and a person asking what they actually did must not have to read an executable file.
//
// Same file name, different directory, so the pair is findable by name alone. The LINK is
// checked rather than assumed: `Evidence` returns nothing for a record whose name and instant
// do not match the artifact beside it, because a routine that was re-demonstrated leaves the
// old session's hands on disk and reading them as the new session's evidence is the one way
// this could lie about what somebody did.
const evidenceDir = "evidence"

// SaveEvidence writes the raw demonstration for a named routine.
//
// Failure here is NOT failure to save the routine. The caller writes the artifact first and
// reports this separately: a person who demonstrated something and got a working routine must
// not be told the demonstration failed because a companion file could not be written.
func (s *Store) SaveEvidence(e workflowstore.Evidence) error {
	if strings.TrimSpace(e.Name) == "" {
		return fmt.Errorf("evidence with no routine to belong to cannot be stored")
	}
	if err := os.MkdirAll(filepath.Join(s.dir, evidenceDir), 0o755); err != nil {
		return fmt.Errorf("demo: preparing %s: %w", evidenceDir, err)
	}
	body, err := workflowstore.MarshalEvidence(e)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(evidenceDir, safeName(e.Name)), body, 0o644)
}

// Evidence reads back the raw demonstration behind a saved routine.
//
// Takes the ARTIFACT rather than the name, so the link is checked on every read and a caller
// cannot accidentally ask for "whatever evidence is filed under this name".
func (s *Store) Evidence(a workflowstore.Artifact) (workflowstore.Evidence, bool) {
	raw, err := os.ReadFile(s.path(evidenceDir, safeName(a.Name)))
	if err != nil {
		return workflowstore.Evidence{}, false
	}
	e, err := workflowstore.UnmarshalEvidence(raw)
	if err != nil || !e.Describes(a) {
		return workflowstore.Evidence{}, false
	}
	return e, true
}
