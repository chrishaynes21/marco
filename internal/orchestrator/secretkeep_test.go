package orchestrator

// EVERY RE-RENDER KEEPS THE CREDENTIAL THE ROUTINE WAS READING.
//
// # What these assert, and what they deliberately do not
//
// THE IDENTIFIER, never the word `Secret`. `sign-in:password` and `password` are two different
// entries in somebody's credential store, and a routine that moved from one to the other still
// says `do OS's Secret …`, still shows the same step list, and still reviews as correct. The
// only thing that changed is which password comes back, and nothing comes back.
//
// Five operations re-render a saved routine, and they do not all have the same answer. Three can
// reproduce the identifier because they render the STORED STEPS, which hold the placeholder, and
// keep the route name. Two cannot: a rename changes the route name by construction, and a
// re-simplification renders the DEMONSTRATION, which never held the placeholder at all. Those two
// refuse.
//
// NOTHING HERE TOUCHES A CREDENTIAL STORE. These compare generated text. The one test that needs
// evidence about a RUN is in `cmd/marco`, against a substituted store — see
// `TestAnEditedRoutineStillAsksForTheCredentialItAlwaysAskedFor`.
//
// See [[ADR-287-a-re-render-keeps-the-credential-it-was-reading]].

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// theCredential is the name `aRoutineAskingFor(t, "password")` reads, as `codegen` writes it.
const theCredential = `OS's Secret with "type-my-address:password"`

// ── THE ONE THE EDITOR COULD NOT EVEN OPEN ────────────────────────────────────

// A ROUTINE THAT READS A CREDENTIAL CAN BE EDITED AT ALL, AND STILL READS THE SAME ONE.
//
// # Why "at all"
//
// Because the step editor renders the stored steps and compares the result against the file,
// refusing when they differ — which is the right rule and was firing for the wrong reason. The
// render wrote the bare `password` where the file says `type-my-address:password`, so every
// routine in the library that reads a credential answered *its saved source is not what its
// stored steps render to under today's rules*, and could not be edited.
func TestARoutineThatReadsACredentialCanBeEditedAndKeepsIt(t *testing.T) {
	d, rt := aRoutineAskingFor(t, "password")
	before, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), theCredential) {
		t.Fatalf("the fixture does not read a route-qualified credential:\n%s", before)
	}

	// THE WAIT AT STEP 3 BECOMES 800ms — an edit that has nothing to do with the
	// credential, which is the point: it must not move one.
	e, err := d.PrepareEdit(rt, []StepEdit{{Op: EditWait, At: 2, To: 800}})
	if err != nil {
		t.Fatalf("a play that reads a credential cannot be edited: %v", err)
	}
	if e.Refused != "" {
		t.Fatalf("the edit was refused: %s", e.Refused)
	}
	if !strings.Contains(e.Now, theCredential) {
		t.Fatalf("the edited version does not read the credential the play reads "+
			"now:\n%s", e.Now)
	}
	if !strings.Contains(e.Now, "800") {
		t.Fatalf("the edit did not land:\n%s", e.Now)
	}
	if err := d.SaveEdit(e); err != nil {
		t.Fatalf("saving the edit: %v", err)
	}
	after, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), theCredential) {
		t.Fatalf("the saved play reads a different credential:\n%s", after)
	}
	// AND IT CAN BE EDITED AGAIN, which is the same check one round later.
	if _, err := d.PrepareEdit(rt, []StepEdit{{Op: EditWait, At: 2, To: 900}}); err != nil {
		t.Fatalf("the edited play cannot be edited again: %v", err)
	}
}

// ── AND THE OTHER FOUR ──
//
// A rebuild under today's rules, a re-simplification, a move and a rename are in
// `rename_test.go`, beside the move engine they go through:
// `TestARebuildUnderTodaysRulesKeepsTheCredential`,
// `TestARebuildThatCannotKeepTheCredentialIsRefused`,
// `TestAMoveKeepsTheCredentialItWasReading` and
// `TestARenameThatWouldMoveACredentialNameIsRefused`.

// ── AND THE RULE ITSELF, UNDER THE SURFACES ───────────────────────────────────

// WHAT A RE-RENDER DECLARES IS READ OFF THE SOURCE, AND BOTH SHAPES SURVIVE.
//
// A credential can be read under the route-qualified name or under a bare global one, and which
// it is was decided when the routine was saved. Declaring is what qualifies it, so declaring
// every secret placeholder would be the same defect pointing the other way: a routine that reads
// the global `token` would start reading `type-my-address:token`.
func TestWhatARenderDeclaresComesFromWhatTheSourceReads(t *testing.T) {
	steps := []macroir.Step{
		{Kind: macroir.StepType, Text: "{{password}}"},
		{Kind: macroir.StepType, Text: "{{token}}"},
		{Kind: macroir.StepType, Text: "{{item}}"},
	}
	// THE SOURCE READS ONE QUALIFIED AND ONE BARE.
	src := `do OS's Secret with "type-my-address:password".
do OS's Secret with "token".
do OS's Type with "{{item}}".`

	base, args := renderArgsFor("type my address", src, steps)
	if base != "type my address" {
		t.Fatalf("the base is %q", base)
	}
	has := map[string]bool{}
	for _, a := range args {
		has[a] = true
	}
	if !has["password"] {
		t.Error("the qualified credential is not declared, so a render would write " +
			"the bare name — a different entry in the store")
	}
	if has["token"] {
		t.Error("the bare credential is declared, so a render would qualify it — " +
			"also a different entry in the store")
	}
	if !has["item"] {
		t.Error("the value somebody supplies is not declared, so a render would " +
			"turn it into a credential-store read")
	}
}

// AND A NAME THE SOURCE READS UNDER NEITHER SHAPE IS NOT DECLARED, so the comparison catches it
// rather than this guessing.
func TestACredentialTheSourceDoesNotReadIsNotDeclared(t *testing.T) {
	steps := []macroir.Step{{Kind: macroir.StepType, Text: "{{password}}"}}
	_, args := renderArgsFor("type my address", `do OS's Secret with "elsewhere:password".`,
		steps)
	for _, a := range args {
		if a == "password" {
			t.Fatal("a credential this source does not read under either shape was " +
				"declared, which is this code deciding which password a " +
				"play should use")
		}
	}
}

// AND THE COMPARISON NAMES WHAT WOULD BE LOST AND WHAT WOULD TAKE ITS PLACE.
func TestTheRefusalNamesBothCredentials(t *testing.T) {
	why := secretsKept(`do OS's Secret with "a:password".`,
		`do OS's Secret with "b:password".`, "renaming it")
	for _, want := range []string{`"a:password"`, `"b:password"`, "renaming it",
		"Nothing has been changed"} {
		if !strings.Contains(why, want) {
			t.Errorf("the refusal does not say %q: %q", want, why)
		}
	}
	// AND SAYS SO WHEN THERE WOULD BE NONE AT ALL.
	gone := secretsKept(`do OS's Secret with "a:password".`, `do OS's Type with "x".`,
		"rebuilding it")
	if !strings.Contains(gone, "no credential at all") {
		t.Errorf("a rebuild that drops the credential entirely says: %q", gone)
	}
	// AND NOTHING WHEN NOTHING MOVED.
	if why := secretsKept(`do OS's Secret with "a:password".`,
		`do OS's Secret with "a:password".`, "x"); why != "" {
		t.Errorf("an unchanged credential was refused: %q", why)
	}
	if why := secretsKept(`do OS's Type with "x".`, `do OS's Type with "y".`, "x"); why != "" {
		t.Errorf("a play with no credential was refused: %q", why)
	}
}
