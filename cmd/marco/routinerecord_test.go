package main

// A ROUTINE'S RUN IS SOMETHING YOU CAN CITE.
//
// Until this, `marco do` on a saved routine announced one word and left nothing behind, so the
// only account of a run was whoever still had the terminal open. These hold the record it writes
// now — and more carefully, the things it must NOT write: a routine presses what it was shown and
// looks at nothing afterwards, so every word and every number about evidence has to be absent
// rather than zero.
//
// # Why nothing is pressed
//
// Every route here names an application that is not running, so the preparation fails and the
// chain stops before any press. `marco do` performs real input and some of these run the real
// host; a case that reached a click would be clicking the owner's desktop. See CLAUDE.md.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runs"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// aRunOf performs one saved routine the way `marco do` does, and returns what it said, the
// outcome it reported and the reference it cited.
//
// Through `performOnePlay`, which is the one door every performance goes through — a test that
// called `runRoute` would be testing the performance and not the account, and the account is what is new.
func aRunOf(t *testing.T, d orchestrator.Deps, rt routes.Route) (said string, out Outcome,
	ref runref.Reference) {

	t.Helper()
	// NO KEYBOARD HOOK. `withPanicStop` installs a real one otherwise, and a test has no
	// business putting a global hook on somebody's desktop.
	t.Setenv("MARCO_NO_PANIC_STOP", "1")
	said, _ = captureStdout(t, func() error {
		var err error
		out, err = performOnePlay(d, playInvocation{rt: rt})
		_ = err
		return nil
	})
	for _, line := range strings.Split(said, "\n") {
		if r, ok := runref.Parse(line); ok {
			ref = r
		}
	}
	return said, out, ref
}

// aLibraryWith writes one routine and returns the world that can perform it.
func aLibraryWith(t *testing.T, src string) (orchestrator.Deps, routes.Route) {
	t.Helper()
	t.Setenv("MARCO_HOME", t.TempDir())
	reg, rt := aSavedRoute(t, src)
	t.Setenv("MARCO_ROUTES", reg.Dir)
	return orchestrator.Deps{Reg: reg, Out: &strings.Builder{},
		Hosts: map[string]runtime.Host{"*": oshost.New()}}, rt
}

// recordAt reads a run's record back as the fields a reader sees, and the raw JSON beside it.
func recordAt(t *testing.T, id string) (map[string]any, string) {
	t.Helper()
	raw, err := os.ReadFile(runs.Path(id))
	if err != nil {
		t.Fatalf("the record of %s could not be read: %v", id, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("the record of %s is not readable JSON: %v", id, err)
	}
	return m, string(raw)
}

// A RUN CITES ITSELF BEFORE IT SENDS ANYTHING, AND LEAVES A RECORD UNDER THAT REFERENCE.
//
// The citation is printed before the first input on purpose: the run it is most needed for is one
// that then went wrong in a way that left nothing behind.
func TestARoutinesRunCitesItselfAndLeavesARecord(t *testing.T) {
	d, rt := aLibraryWith(t, aGuardedRoutine)
	said, out, ref := aRunOf(t, d, rt)

	if ref.ID == "" {
		t.Fatalf("the run cited no reference. It said:\n%s", said)
	}
	if ref.Path != runs.Path(ref.ID) {
		t.Errorf("the citation points at %q, and the record is at %q",
			ref.Path, runs.Path(ref.ID))
	}
	// AND THE CITATION CARRIES NO OUTCOME. `runref.Parse` strips a parenthesised word
	// only when it is one of the SEVEN SUPERVISED ones, so a routine's word on that line
	// would be left inside the path. The outcome arrives on `[result]`, from the package
	// that owns a routine's words.
	if ref.Outcome != "" {
		t.Errorf("the citation carries the outcome %q; a routine's word is not one of "+
			"the supervised seven and would be read as part of the path", ref.Outcome)
	}
	m, raw := recordAt(t, ref.ID)
	for field, want := range map[string]any{
		"kind":     kindRoutine,
		"routine":  rt.Slug,
		"scope":    "focus",
		"contract": string(orchestrator.Guarded),
		"mode":     modeUnstated,
		"outcome":  string(out),
	} {
		if m[field] != want {
			t.Errorf("the record's %s is %v, want %v", field, m[field], want)
		}
	}
	if m["application"] != anAbsentApplication {
		t.Errorf("the record's application is %v", m["application"])
	}
	for _, field := range []string{"build", "source_digest", "library", "started",
		"ended", "because"} {
		if s, _ := m[field].(string); s == "" {
			t.Errorf("the record says nothing for %s:\n%s", field, raw)
		}
	}
	// AND THE SENTENCE IS THE RUN'S OWN, carried rather than paraphrased.
	if because, _ := m["because"].(string); !strings.Contains(because, rt.Slug) {
		t.Errorf("the record's reason is %q and does not name what stopped", because)
	}
}

// AND THE RECORD CITES THE SOURCE THAT ACTUALLY RAN.
//
// The digest is of the SAVED file, not of the text with arguments substituted into it: somebody
// holding this record and the library can look at exactly what ran, and nothing a person typed
// ends up in a durable file.
func TestTheRecordCitesTheSourceThatRan(t *testing.T) {
	d, rt := aLibraryWith(t, aGuardedRoutine)
	_, _, ref := aRunOf(t, d, rt)
	m, _ := recordAt(t, ref.ID)
	if got, want := m["source_digest"], routes.DigestOf(aGuardedRoutine); got != want {
		t.Errorf("the record cites source %v, and what ran digests to %v", got, want)
	}
}

// AND NOTHING IN A ROUTINE'S RECORD IS ABOUT EVIDENCE.
//
// The rule ADR-253 exists for, asserted on the KEYS and not on their values. Three zeroes read as
// a run that asked those questions and found nothing; the truth is that a routine presses what it
// was shown and looks at nothing afterwards, so the questions were never asked. And `verified` may
// never appear at all: it is a claim about evidence this run never gathered.
func TestARoutinesRecordSaysNothingAboutEvidence(t *testing.T) {
	d, rt := aLibraryWith(t, aGuardedRoutine)
	_, _, ref := aRunOf(t, d, rt)
	m, raw := recordAt(t, ref.ID)
	for _, field := range []string{"verified", "contradicted", "unconfirmed"} {
		if _, present := m[field]; present {
			t.Errorf("a routine's record carries %q, which nothing on this path "+
				"ever asked:\n%s", field, raw)
		}
	}
	if strings.Contains(raw, "verified") {
		t.Errorf("the word `verified` is in a routine's record:\n%s", raw)
	}
}

// AND A LEGACY ROUTINE'S RECORD SAYS SO.
//
// Which contract the source states is what decides whether an account of what did not happen
// stops the run, and a record that did not say it would leave somebody comparing two runs under
// two different rules with nothing to tell them apart.
func TestALegacyRoutinesRecordSaysWhichContractItRanUnder(t *testing.T) {
	d, rt := aLibraryWith(t, aLegacyRoutine)
	_, out, ref := aRunOf(t, d, rt)
	m, _ := recordAt(t, ref.ID)
	if m["contract"] != string(orchestrator.Legacy) {
		t.Errorf("a route stating no basis recorded its contract as %v", m["contract"])
	}
	// AND IT STILL REPORTS WHAT IT ALWAYS DID: a legacy route was saved under a contract
	// that says a click is attempted and the run carries on.
	if out != OutcomePerformed {
		t.Errorf("a legacy routine reported %q; that contract is not this one's "+
			"business", out)
	}
}

// TWO RUNS OF ONE ROUTINE ARE TWO RECORDS.
//
// The acceptance check — *both repeated runs produce distinct references whose reports match
// their respective outcomes* — at the command that produces them. One reference for two runs
// would mean the second run described itself in the first one's record.
func TestTwoRunsOfOneRoutineAreTwoRecords(t *testing.T) {
	d, rt := aLibraryWith(t, aGuardedRoutine)
	_, firstOut, first := aRunOf(t, d, rt)
	_, secondOut, second := aRunOf(t, d, rt)

	if first.ID == "" || second.ID == "" {
		t.Fatalf("a run cited nothing: %q and %q", first.ID, second.ID)
	}
	if first.ID == second.ID {
		t.Fatalf("both runs cite %q", first.ID)
	}
	one, _ := recordAt(t, first.ID)
	two, _ := recordAt(t, second.ID)
	if one["outcome"] != string(firstOut) {
		t.Errorf("the first record says %v and the first run reported %q",
			one["outcome"], firstOut)
	}
	if two["outcome"] != string(secondOut) {
		t.Errorf("the second record says %v and the second run reported %q",
			two["outcome"], secondOut)
	}
	if one["started"] == two["started"] && one["run"] == two["run"] {
		t.Error("the two records are one record")
	}
}

// AND A RUN THAT NEVER FINISHED IS STILL RETRIEVABLE AS ONE THAT DID NOT.
//
// The killed-child case at the command. The reservation is made and nothing finalises it, which is
// what a `marco.exe` killed mid-run leaves behind: a record saying the attempt began, with no
// `ended`, and never a completed or successful one.
func TestARunThatNeverFinishedSaysSoAndNotThatItWorked(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	t.Setenv("MARCO_ROUTES", t.TempDir())
	rt := routes.Route{Slug: "a-routine", App: anAbsentApplication, Focus: true}

	var ref runref.Reference
	said, _ := captureStdout(t, func() error {
		res := citeRoutineRun(time.Now(), rt)
		if res.ID == "" {
			t.Fatal("nothing was reserved")
		}
		return nil
	})
	for _, line := range strings.Split(said, "\n") {
		if r, ok := runref.Parse(line); ok {
			ref = r
		}
	}
	m, raw := recordAt(t, ref.ID)
	if m["outcome"] != runs.Unfinished {
		t.Errorf("an interrupted attempt reads as %v, want %q", m["outcome"],
			runs.Unfinished)
	}
	if _, ended := m["ended"]; ended {
		t.Errorf("an interrupted attempt says when it ended:\n%s", raw)
	}
	if m["kind"] != kindRoutine {
		t.Errorf("an interrupted attempt does not say which kind it is: %v", m["kind"])
	}
	// AND IT NEVER READS AS A RUN THAT WORKED.
	if strings.Contains(raw, string(OutcomePerformed)) {
		t.Errorf("an interrupted attempt reads as performed:\n%s", raw)
	}
}

// AND THE RECORD SAYS HOW MANY INPUT ACTIONS THE RUN ASKED FOR.
//
// Zero here, truthfully: the preparation named an application that is not running, so the chain
// stopped before any press. Zero is a real answer and is written. A host that cannot be asked at
// all is a different fact — see the test below.
func TestTheRecordSaysHowManyInputActionsWereAskedFor(t *testing.T) {
	d, rt := aLibraryWith(t, aGuardedRoutine)
	_, _, ref := aRunOf(t, d, rt)
	m, raw := recordAt(t, ref.ID)
	n, present := m["input_actions"]
	if !present {
		t.Fatalf("the real host can say what was asked for and the record does "+
			"not:\n%s", raw)
	}
	if n != float64(0) {
		t.Errorf("a run that stopped at its preparation asked for %v input actions", n)
	}
}

// AND A HOST THAT CANNOT SAY LEAVES THE FIELD OUT.
//
// Nil and not zero. Writing zero for a host nobody can ask would turn "nobody knows" into
// "nothing happened", in the one field the record has for answering "did it touch my machine".
func TestAHostThatCannotSayHowMuchItSentLeavesTheFieldOut(t *testing.T) {
	d, rt := aLibraryWith(t, aGuardedRoutine)
	d.Hosts = map[string]runtime.Host{"*": &aHostThatWillNotDeliver{}}
	_, _, ref := aRunOf(t, d, rt)
	m, raw := recordAt(t, ref.ID)
	if _, present := m["input_actions"]; present {
		t.Errorf("a host that cannot be asked produced a count anyway:\n%s", raw)
	}
}

// aHostThatLooksForItsOwnRecord answers like an absent application, and on its FIRST call counts
// the records already in the store.
//
// Which is the whole of what "before the first input" means, measured from inside the run rather
// than from the order of some printed lines.
type aHostThatLooksForItsOwnRecord struct {
	asked   int
	atFirst int
}

func (h *aHostThatLooksForItsOwnRecord) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	h.asked++
	if h.asked == 1 {
		entries, _ := os.ReadDir(runs.Dir())
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".json") {
				h.atFirst++
			}
		}
	}
	return "failed", runtime.Absent(), nil
}

func (h *aHostThatLooksForItsOwnRecord) Unperformed() []string {
	return []string{"the " + anAbsentApplication + " window could not be brought to the front"}
}

// THE REFERENCE IS CLAIMED BEFORE THE FIRST INPUT.
//
// Not "the citation is printed early" — that is a line of output and a test of it would pass over
// a reservation made afterwards. What matters is that the RECORD IS ALREADY THERE when the run
// starts touching the machine, because the run this exists for is the one that dies at its first
// press and leaves nothing else behind.
//
// Measured from inside the run, by the host, on the first thing it is asked to do.
func TestTheReferenceIsClaimedBeforeTheFirstInput(t *testing.T) {
	d, rt := aLibraryWith(t, aGuardedRoutine)
	watcher := &aHostThatLooksForItsOwnRecord{}
	d.Hosts = map[string]runtime.Host{"*": watcher}
	_, _, ref := aRunOf(t, d, rt)

	if watcher.asked == 0 {
		t.Fatal("the host was never asked for anything; this case measures nothing")
	}
	if watcher.atFirst != 1 {
		t.Errorf("when the run asked for its first action there were %d records in "+
			"the store, want 1 — its own, claimed before anything was sent",
			watcher.atFirst)
	}
	if ref.ID == "" {
		t.Error("the run cited nothing")
	}
}

// aRoutineWithAnArgument states the window it was recorded in and takes a word.
const aRoutineWithAnArgument = `use os.

the ARoutine is an actor.

this can Run.
this's Run does...
    do OS's Type with "{{who}}".
    do OS's Activate with "` + anAbsentApplication + `"...
        when ok?
            this is that!
        or?
            this is that!

the play...
    do ARoutine's Run.
`

// AND THE DIGEST IS OF THE SAVED FILE, NOT OF WHAT A PERSON TYPED INTO IT.
//
// Two reasons, and both matter. A digest of the substituted text cites source that is nowhere on
// disk, so nobody can look at what ran; and the substituted text holds whatever the person passed
// on the command line, which would make a durable file out of their arguments.
func TestTheDigestIsOfTheFileAndNotOfWhatWasTypedIntoIt(t *testing.T) {
	t.Setenv("MARCO_NO_PANIC_STOP", "1")
	t.Setenv("MARCO_HOME", t.TempDir())
	reg := routes.Registry{Dir: t.TempDir()}
	t.Setenv("MARCO_ROUTES", reg.Dir)
	rt := routes.Route{Slug: "a-routine", App: anAbsentApplication, Focus: true}
	if err := os.MkdirAll(filepath.Dir(reg.Path(rt)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := reg.Save(rt, aRoutineWithAnArgument); err != nil {
		t.Fatal(err)
	}
	d := orchestrator.Deps{Reg: reg, Out: &strings.Builder{},
		Hosts: map[string]runtime.Host{"*": &aHostThatWillNotDeliver{}}}

	const secretish = "correct-horse-battery-staple"
	var ref runref.Reference
	said, _ := captureStdout(t, func() error {
		_, _ = performOnePlay(d, playInvocation{rt: rt,
			named: map[string]string{"who": secretish}})
		return nil
	})
	for _, line := range strings.Split(said, "\n") {
		if r, ok := runref.Parse(line); ok {
			ref = r
		}
	}
	if ref.ID == "" {
		t.Fatalf("the run cited nothing. It said:\n%s", said)
	}
	m, raw := recordAt(t, ref.ID)
	if got, want := m["source_digest"], routes.DigestOf(aRoutineWithAnArgument); got != want {
		t.Errorf("the record cites source %v; the SAVED file digests to %v", got, want)
	}
	if strings.Contains(raw, secretish) {
		t.Errorf("what the person typed is in the durable record:\n%s", raw)
	}
}

// anIncompleteRoutine states the window it was recorded in and is MISSING a term the check needs.
//
// `Dpi` is absent — not present and empty, absent, which is what a route from a build that could
// not read it actually looks like. ADR-251 makes this a contract of its own precisely because it
// refuses rather than pressing, naming what it lacks.
const anIncompleteRoutine = `use os.

the ARoutine is an actor.

this can Run.
this's Run does...
    the p1 is a Point with X 842, Y 274, RelX 722, RelY 210, RelTo "window", WinW 1920, WinH 1080, Process "something.exe".
    do OS's Click with p1...
        when ok?
            this is ok!
        or?
            this is that!

the App is a script.

do ARoutine's Run...
    when ok?
        log "a routine: done".
    or?
        log that's error.
`

// AN INCOMPLETE ROUTINE'S REFUSAL STILL FAILS THE COMMAND, AND ITS RECORD SAYS WHICH CONTRACT.
//
// The third contract in ADR-251, at the command. It is here because of what the contract rule
// looks like: `legacy` is the one that leaves without consulting the host's account, and the
// plausible-looking form — anything that is not `guarded` leaves — would let an incomplete
// route's refusal pass as a successful run. Nothing in this package covered that, so the mutation
// survived; this is what closes it.
func TestAnIncompleteRoutinesRefusalStillFailsTheCommand(t *testing.T) {
	d, rt := aLibraryWith(t, anIncompleteRoutine)
	d.Hosts = map[string]runtime.Host{"*": oshost.New()}
	_, out, ref := aRunOf(t, d, rt)

	if out == OutcomePerformed {
		t.Error("a routine whose press refused for a missing measurement reported " +
			"that it performed")
	}
	m, _ := recordAt(t, ref.ID)
	if m["contract"] != string(orchestrator.Incomplete) {
		t.Errorf("the record calls it %v, and a press stating a window while lacking "+
			"a term is its own contract", m["contract"])
	}
	// AND THE REASON NAMES WHAT WAS LACKING, carried from the run's own account.
	if because, _ := m["because"].(string); because == "" {
		t.Error("the record says nothing about why it stopped")
	}
}
