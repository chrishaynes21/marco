package gate

import (
	"fmt"
	"os"
	"strings"
)

// Declared is a skip this gate tolerates: a package, a test, and the reason somebody wrote.
//
// # Why a reason is required and not optional
//
// Because the list is the gate's own statement of what it does not cover, and a bare test name is
// not one. Every entry here is a sentence a reader can disagree with — "this needs a game
// running", "this is a tool, not a check" — and an entry nobody could write a reason for is an
// entry that should be a failure instead.
type Declared struct {
	Package string
	Test    string
	Why     string
	Line    int
	// Broken marks a declared skip that is a DEFECT rather than an expectation: a test
	// that no longer establishes what it was written to establish.
	//
	// Written `BROKEN:` at the front of the reason. The distinction matters because the two
	// kinds of entry mean opposite things about coverage — "this needs a game running" is
	// a limit somebody chose, and "the script this gate is built on stopped producing the
	// condition" is a mutation gate that has quietly died. Both are tolerated by a gate
	// that must still be able to pass; only one of them is somebody's decision.
	//
	// So they are counted and printed apart, on a green run as well, and an entry here is
	// a parked finding rather than a settled answer.
	Broken bool
}

// brokenMark begins the reason of a declared skip that is a defect rather than a decision.
const brokenMark = "BROKEN:"

// Broken are the declared skips that are defects, for a report that counts them apart.
func Broken(declared []Declared) []Declared {
	var out []Declared
	for _, d := range declared {
		if d.Broken {
			out = append(out, d)
		}
	}
	return out
}

// Skipped is a test that declined to run, and what it said.
type Skipped struct {
	Package string
	Test    string
	Says    string
}

// ReadLedger reads the declared skips, or says which line it could not read.
//
// The format is one skip per line:
//
//	<package> <TestName> — the reason this gate tolerates it
//
// `#` begins a comment and blank lines are ignored. A line that names a test and gives no reason
// is an error rather than an entry, because that is the shape a silent exemption would take.
func ReadLedger(path string) ([]Declared, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []Declared
	for i, line := range strings.Split(string(raw), "\n") {
		text := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Fields(text)
		if len(fields) < 2 {
			return nil, fmt.Errorf("%s:%d: %q names no test", path, i+1, text)
		}
		why := strings.TrimSpace(strings.TrimPrefix(
			strings.TrimSpace(strings.Join(fields[2:], " ")), "—"))
		why = strings.TrimSpace(strings.TrimPrefix(why, "-"))
		if why == "" {
			return nil, fmt.Errorf("%s:%d: %s %s is tolerated with no reason given, "+
				"and a skip nobody can give a reason for is a failure",
				path, i+1, fields[0], fields[1])
		}
		out = append(out, Declared{Package: fields[0], Test: fields[1], Why: why,
			Line: i + 1, Broken: strings.HasPrefix(why, brokenMark)})
	}
	return out, nil
}

// Unexpected are the skips no entry in the ledger declares.
//
// The finding the gate exists for: a test that stopped running for a reason nobody wrote down.
func Unexpected(skipped []Skipped, declared []Declared) []Skipped {
	ok := map[string]bool{}
	for _, d := range declared {
		ok[d.Package+" "+d.Test] = true
	}
	var out []Skipped
	for _, s := range skipped {
		if !ok[s.Package+" "+s.Test] {
			out = append(out, s)
		}
	}
	return out
}

// Stale are the declared skips that did not skip, in packages this run actually covered.
//
// # Why a ledger entry that no longer applies is a finding too
//
// Because the entries are read as the gate's coverage, and one that has quietly started passing
// makes the gate look narrower than it is — or, worse, names a test that no longer exists, in
// which case the exemption is doing nothing and the next skip of a RENAMED test arrives
// undeclared. A list of excuses nobody prunes stops describing anything.
//
// `covered` is the set of packages whose tests ran, because an entry for a package this run did
// not reach has not been shown to be anything.
func Stale(declared []Declared, skipped []Skipped, covered map[string]bool) []Declared {
	did := map[string]bool{}
	for _, s := range skipped {
		did[s.Package+" "+s.Test] = true
	}
	var out []Declared
	for _, d := range declared {
		if covered[d.Package] && !did[d.Package+" "+d.Test] {
			out = append(out, d)
		}
	}
	return out
}
