package main

// A SECRET THAT COULD NOT BE LOOKED UP MUST NOT REPORT A RUN THAT PERFORMED.
//
// `doSecret` had two outcomes that went out through `fail` and reached no account at all: a
// secret the store has no value for, and a store that would not answer. The chain did its job in
// both cases — the arm ended the routine and nothing after it ran — and then the COMMAND said
// `performed`, because the rule that fails a run reads `Unperformed` and the account was empty.
// A person was told their routine had run while the field the password was going into was empty.
//
// # Why this is at the command and with the production host
//
// Because the rule is here. `runRoute` turns a non-empty account into a failed run, and a test
// with a host of its own would be a test of the rule against a stand-in that was written to
// satisfy it. The host is the real one, with the real `doSecret` in it.
//
// # Why the store is substituted, and why the value it holds is EMPTY
//
// A test may not put a value in somebody's credential manager to see what a run does with it,
// and may not read whatever happens to be in there either — that is a different machine's answer
// every time. So `oshost.NewWithSecrets` takes the store, and the one case that NEEDS a value
// stores an empty one: a secret that is found and is empty submits no characters at all, so the
// control reaches the step after the secret without typing anything into whatever has focus on
// the machine running the tests.
//
// Nothing here presses anything either. The routine's second link is a guarded press with no
// `Activate` in front of it, so it refuses for want of a bound window — which is what makes
// "the step after the secret was reached" observable without sending input.
//
// The gesture and the account are held in `internal/oshost` by
// `TestASecretThatCouldNotBeLookedUpIsInTheAccount`; the chain's shape is `internal/codegen`'s.
//
// See [[ADR-261-typed-text-is-a-sequence-of-submissions]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// aRoutineThatNeedsASecret is a guarded chain whose FIRST link is the secret and whose second is
// a press — so what the secret stopped is observable as a press that was never asked for.
//
// Written out rather than generated, which is this package's pattern for a source it needs a
// particular shape of: the shape itself — one `do` per capability, each calling the next from
// inside its own success — is `internal/codegen`'s claim and is tested there.
const aRoutineThatNeedsASecret = `use os.

the ARoutine is an actor.

this can Run.
this's Run does...
    do OS's Secret with "a-routine:password"...
        when ok?
            do ARoutine's Step2.
            this is that!
        or?
            this is that!

this can Step2.
this's Step2 does...
    the p1 is a Point with X 842, Y 274, RelX 722, RelY 210, RelTo "window", WinW 1920, WinH 1080, Dpi 144, Process "something.exe".
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

// aLibraryWithSecrets is `aLibraryWith` with the credential store supplied.
func aLibraryWithSecrets(t *testing.T, src string, store map[string]string) (orchestrator.Deps,
	routes.Route) {

	t.Helper()
	t.Setenv("MARCO_HOME", t.TempDir())
	reg, rt := aSavedRoute(t, src)
	t.Setenv("MARCO_ROUTES", reg.Dir)
	return orchestrator.Deps{Reg: reg, Out: &strings.Builder{},
		Hosts: map[string]runtime.Host{"*": oshost.NewWithSecrets(aTestStore(store))}}, rt
}

// aTestStore is a credential store with whatever a case put in it, and nothing else.
type aTestStore map[string]string

func (s aTestStore) Set(name, value string) error { s[name] = value; return nil }
func (s aTestStore) Get(name string) (string, bool, error) {
	v, ok := s[name]
	return v, ok, nil
}
func (s aTestStore) Delete(name string) error { delete(s, name); return nil }
func (s aTestStore) List() ([]string, error)  { return nil, nil }

// A MISSING SECRET FAILS THE COMMAND, KEEPS THE REASON, AND STOPS WHAT FOLLOWS.
func TestAMissingSecretFailsTheCommandAndStopsTheRoutine(t *testing.T) {
	d, rt := aLibraryWithSecrets(t, aRoutineThatNeedsASecret, map[string]string{})
	said, out, ref := aRunOf(t, d, rt)

	if out != OutcomeFailed {
		t.Errorf("a routine that stopped at a secret nobody stored reported %q", out)
	}
	m, raw := recordAt(t, ref.ID)
	because, _ := m["because"].(string)
	switch {
	case because == "":
		t.Errorf("the record says nothing about why it stopped:\n%s", raw)
	case !strings.Contains(because, "a-routine:password"):
		t.Errorf("the record says %q and does not name the secret, which is the "+
			"only thing that says which one to go and set", because)
	case !strings.Contains(because, "marco secret set"):
		t.Errorf("the record says %q and does not say how to set it", because)
	}
	// AND NOTHING AFTER IT WAS ASKED FOR, read off the record's own count of the input
	// actions this run asked the machine for: the secret is one, and the press after it
	// would have been the second.
	if n, _ := m["input_actions"].(float64); n != 1 {
		t.Errorf("the run asked the machine for %v input action(s), want 1 — the "+
			"secret, and nothing after it:\n%s", m["input_actions"], raw)
	}
	if strings.Contains(said, "a routine: done") {
		t.Errorf("the routine reported that it finished:\n%s", said)
	}
}

// AND A STORE THAT WOULD NOT ANSWER IS THE SAME, WITH ITS OWN WORDS.
//
// A locked store, a service that is not running, a profile the process cannot read. It is not a
// missing secret — there may well be a value in there — so the sentence says the store would not
// answer rather than telling somebody to set something they have already set.
func TestAStoreThatWouldNotAnswerFailsTheCommand(t *testing.T) {
	d, rt := aLibraryWithSecrets(t, aRoutineThatNeedsASecret, nil)
	d.Hosts = map[string]runtime.Host{"*": oshost.NewWithSecrets(aStoreThatWillNotAnswer{})}
	said, out, ref := aRunOf(t, d, rt)

	if out != OutcomeFailed {
		t.Errorf("a routine that stopped at a store that would not answer reported "+
			"%q", out)
	}
	m, raw := recordAt(t, ref.ID)
	because, _ := m["because"].(string)
	if !strings.Contains(because, "would not answer") {
		t.Errorf("the record says %q and does not say the store would not answer:\n%s",
			because, raw)
	}
	if strings.Contains(because, "marco secret set") {
		t.Errorf("the record tells somebody to set a secret that may already be "+
			"set: %q", because)
	}
	if n, _ := m["input_actions"].(float64); n != 1 {
		t.Errorf("the run asked for %v input action(s), want 1", m["input_actions"])
	}
	if strings.Contains(said, "a routine: done") {
		t.Errorf("the routine reported that it finished:\n%s", said)
	}
}

// AND THE CONTROL: A SECRET THAT WAS FOUND LETS THE ROUTINE GO ON.
//
// Without which both cases above are satisfied by a command that fails every routine containing
// a secret. The value stored is EMPTY on purpose — see the note at the top of this file — so the
// step after the secret is reached with nothing typed anywhere.
//
// That step is a guarded press with no `Activate` in front of it, so it refuses for want of a
// bound window and sends nothing. The run still fails, and what it fails ABOUT is the point: the
// account names the window rather than the secret, which is how this says the secret went
// through.
func TestASecretThatWasFoundLetsTheRoutineReachTheNextStep(t *testing.T) {
	d, rt := aLibraryWithSecrets(t, aRoutineThatNeedsASecret,
		map[string]string{"a-routine:password": ""})
	_, _, ref := aRunOf(t, d, rt)

	m, raw := recordAt(t, ref.ID)
	because, _ := m["because"].(string)
	if strings.Contains(because, "a-routine:password") {
		t.Errorf("a secret that was found is still what the run stopped at: %q", because)
	}
	if !strings.Contains(because, "window") {
		t.Errorf("the run stopped at %q, and the step after the secret is a press "+
			"that refuses for want of a window — so this does not establish "+
			"that the secret went through:\n%s", because, raw)
	}
	// AND THE STEP AFTER IT WAS ASKED FOR, which is the whole claim: two input actions
	// asked for rather than one.
	if n, _ := m["input_actions"].(float64); n != 2 {
		t.Errorf("the run asked the machine for %v input action(s), want 2 — the "+
			"secret and the press after it:\n%s", m["input_actions"], raw)
	}
}

// aStoreThatWillNotAnswer is a credential store in a state a machine can really be in.
type aStoreThatWillNotAnswer struct{ aTestStore }

func (aStoreThatWillNotAnswer) Get(string) (string, bool, error) {
	return "", false, errTheStoreIsLocked
}

var errTheStoreIsLocked = errStore("the credential store is not available")

type errStore string

func (e errStore) Error() string { return string(e) }
