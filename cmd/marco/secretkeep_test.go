package main

// AN EDITED ROUTINE ASKS THE CREDENTIAL STORE FOR THE SAME THING IT ALWAYS ASKED FOR.
//
// # Why this one is a RUN and not a comparison of two sources
//
// Because every other test of this correction reads generated text, and the thing that matters
// is what the machine asks for. `sign-in:password` and `password` are two different entries, and
// a routine that moved from one to the other still says `do OS's Secret …` in every review. The
// only place the difference becomes visible is the lookup.
//
// So the store is SUBSTITUTED and it WRITES DOWN what it was asked for. `oshost.NewWithSecrets`
// is the seam, and the host around it is the production one with the production `doSecret` in it.
//
// # What this does not do
//
// It does not touch anybody's credential manager — a test may not put a value in one to see what
// a run does with it, and may not read whatever happens to be in there either. The value stored
// is EMPTY, which submits no characters at all, so nothing is typed into whatever has focus on
// the machine running the tests. And nothing is pressed: the routine's steps after the secret are
// a key and a wait, performed through the same host, with no window asked for and no pointer
// moved.
//
// See [[ADR-287-a-re-render-keeps-the-credential-it-was-reading]].

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// aListeningStore is a credential store that answers from what a case put in it and remembers
// every name it was asked for.
//
// THE RECORD IS THE EVIDENCE. Asserting that the run SUCCEEDED would say the lookup found
// something; asserting the name says WHICH something, which is the whole question.
type aListeningStore struct {
	have  map[string]string
	asked *[]string
}

func (s aListeningStore) Set(name, value string) error { s.have[name] = value; return nil }
func (s aListeningStore) Get(name string) (string, bool, error) {
	*s.asked = append(*s.asked, name)
	v, ok := s.have[name]
	return v, ok, nil
}
func (s aListeningStore) Delete(name string) error { delete(s.have, name); return nil }
func (s aListeningStore) List() ([]string, error)  { return nil, nil }

// aSavedRoutineReadingACredential is a routine whose typed step takes a password from the store,
// saved the way one is saved — with its demonstration and its step account beside it, so it can
// be edited afterwards.
func aSavedRoutineReadingACredential(t *testing.T, asked *[]string,
	have map[string]string) (orchestrator.Deps, routes.Route) {

	t.Helper()
	t.Setenv("MARCO_HOME", t.TempDir())
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	reg := routes.Registry{Dir: dir}
	d := orchestrator.Deps{Reg: reg, Out: &strings.Builder{},
		Hosts: map[string]runtime.Host{"*": oshost.NewWithSecrets(
			aListeningStore{have: have, asked: asked})}}

	steps := []macroir.Step{
		{Kind: macroir.StepType, Text: "{{password}}"},
		{Kind: macroir.StepWait, Ms: 10},
	}
	dr := &orchestrator.Draft{Name: "sign in to the portal", Args: []string{"password"},
		Steps: steps, Mode: orchestrator.Unstated}
	rt, err := d.Save(dr, orchestrator.ScopeEverywhere, "")
	if err != nil {
		t.Fatalf("saving the fixture: %v", err)
	}
	// A REAL DEMONSTRATION, so a rebuild has something to render and refuses for the
	// reason this is about rather than for want of a recording. It does NOT hold the
	// placeholder — a secret's value never enters a recording — which is exactly why a
	// rebuild from it cannot keep the credential.
	base := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	rec, err := json.Marshal([]recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: base},
		{Kind: recorder.EvKey, KeyName: "a", Down: true, App: "notepad",
			T: base.Add(10 * time.Millisecond)},
		{Kind: recorder.EvKey, KeyName: "a", Down: false, App: "notepad",
			T: base.Add(20 * time.Millisecond)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, App: "notepad",
			T: base.Add(30 * time.Millisecond)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.SaveRecording(rt, rec); err != nil {
		t.Fatalf("writing the fixture's demonstration: %v", err)
	}
	src, rerr := os.ReadFile(reg.Path(rt))
	if rerr != nil {
		t.Fatal(rerr)
	}
	raw, err := json.Marshal(steps)
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.SaveSteps(rt, routes.StoredSteps{
		Source:        routes.DigestOf(string(src)),
		Demonstration: routes.DigestOf(string(rec)),
		Steps:         raw,
	}); err != nil {
		t.Fatalf("writing the fixture's step account: %v", err)
	}
	return d, rt
}

// THE RUN ASKS FOR THE SAME NAME BEFORE AND AFTER AN EDIT.
func TestAnEditedRoutineStillAsksForTheCredentialItAlwaysAskedFor(t *testing.T) {
	var asked []string
	// THE VALUE IS EMPTY ON PURPOSE: found, and submits no characters.
	have := map[string]string{"sign-in-to-the-portal:password": ""}
	d, rt := aSavedRoutineReadingACredential(t, &asked, have)

	src, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), `OS's Secret with "sign-in-to-the-portal:password"`) {
		t.Fatalf("the fixture does not read a route-qualified credential:\n%s", src)
	}

	// ── BEFORE ──
	if _, out, _ := aRunOf(t, d, rt); out != OutcomePerformed {
		t.Fatalf("the routine reported %q before any edit was made, so this cannot "+
			"say what an edit changed", out)
	}
	if len(asked) != 1 || asked[0] != "sign-in-to-the-portal:password" {
		t.Fatalf("the run asked the credential store for %v", asked)
	}

	// ── THE EDIT: the wait becomes 800ms, which has nothing to do with the password ──
	e, err := d.PrepareEdit(rt, []orchestrator.StepEdit{
		{Op: orchestrator.EditWait, At: 1, To: 800}})
	if err != nil {
		t.Fatalf("a routine that reads a credential cannot be edited: %v", err)
	}
	if e.Refused != "" {
		t.Fatalf("the edit was refused: %s", e.Refused)
	}
	if err := d.SaveEdit(e); err != nil {
		t.Fatalf("saving the edit: %v", err)
	}

	// ── AFTER ──
	asked = nil
	if _, out, _ := aRunOf(t, d, rt); out != OutcomePerformed {
		t.Errorf("the routine reported %q after an edit that did not touch the "+
			"password, which is what a redirected lookup looks like from "+
			"outside: nothing is found and the run stops", out)
	}
	if len(asked) != 1 {
		t.Fatalf("the edited routine asked the credential store for %v", asked)
	}
	if asked[0] != "sign-in-to-the-portal:password" {
		t.Fatalf("the edited routine asks the credential store for %q, and it asked "+
			"for %q before the edit. Those are two different entries, so it "+
			"signs in with nothing", asked[0],
			"sign-in-to-the-portal:password")
	}
}

// AND A ROUTINE WHOSE CREDENTIAL COULD NOT BE KEPT IS NEVER WRITTEN, SO NO RUN EVER ASKS WRONG.
//
// The other half: the operations that cannot preserve the identifier refuse, so there is no
// saved routine for a run to get wrong. Asserted at the run because that is where the cost
// would land.
func TestARefusedRebuildLeavesTheRoutineAskingForWhatItAlwaysAskedFor(t *testing.T) {
	var asked []string
	have := map[string]string{"sign-in-to-the-portal:password": ""}
	d, rt := aSavedRoutineReadingACredential(t, &asked, have)

	// A RE-SIMPLIFICATION RENDERS THE DEMONSTRATION, which never held the placeholder.
	p, err := d.Prepare(rt, orchestrator.RebuildSimplifyTyping)
	if err != nil {
		t.Fatalf("preparing the rebuild: %v", err)
	}
	if p.Refused == "" {
		t.Fatalf("a rebuild that drops the credential was offered, and would "+
			"write:\n%s", p.Now)
	}
	if !strings.Contains(p.Refused, "sign-in-to-the-portal:password") {
		t.Errorf("the refusal does not name the credential: %q", p.Refused)
	}

	// AND THE ROUTINE STILL RUNS, ASKING FOR WHAT IT ALWAYS ASKED FOR.
	if _, out, _ := aRunOf(t, d, rt); out != OutcomePerformed {
		t.Fatalf("the routine reported %q after a REFUSED rebuild", out)
	}
	if len(asked) != 1 || asked[0] != "sign-in-to-the-portal:password" {
		t.Fatalf("the run asked the credential store for %v", asked)
	}
}
