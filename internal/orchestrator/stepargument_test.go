package orchestrator

// THE TEXT A STEP TYPES BECOMES SOMETHING YOU SUPPLY.
//
// The third edit operation, and the reason it exists is discoverability rather than capability:
// this was already possible two ways and both were invisible — name the recording "… with
// message" so the phrase declares an argument, and tap a reserved key (f9) at the moment in the
// demonstration where the text goes. Neither can be found from any surface.
//
// As an edit it needs no foreknowledge and no second demonstration, and it works on routines
// saved before any of it existed. What it produces is the placeholder everything downstream is
// already built on, which is why these tests end by asking `routes` what the edited routine
// takes rather than by comparing source text.
//
// See [[ADR-276-a-routine-may-be-told-what-to-type]] and
// [[ADR-277-a-request-may-propose-the-text-a-routine-types]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// typingSteps is a step list with one Type step in the middle of it.
func typingSteps() []macroir.Step {
	return []macroir.Step{
		{Kind: macroir.StepActivate, Text: "notepad"},
		{Kind: macroir.StepType, Text: "hello there"},
		{Kind: macroir.StepKey, Key: "enter"},
		{Kind: macroir.StepWait, Ms: 50},
	}
}

// edited applies one operation to typingSteps and returns the new list, or the refusal.
func edited(t *testing.T, ops ...StepEdit) ([]macroir.Step, []string, string) {
	t.Helper()
	out, says, why, _, err := applyEdits(typingSteps(), ops)
	if err != nil {
		return nil, says, err.Error()
	}
	return out, says, why
}

// THE LITERAL BECOMES A PLACEHOLDER, AND NOTHING ELSE MOVES.
func TestTheTextAStepTypesBecomesSomethingSupplied(t *testing.T) {
	out, says, why := edited(t, StepEdit{Op: EditArgument, At: 1, Name: "message"})
	if why != "" {
		t.Fatalf("it refused: %s", why)
	}
	if len(out) != 4 {
		t.Fatalf("the list is now %d steps", len(out))
	}
	if out[1].Text != "{{message}}" {
		t.Errorf("the step types %q", out[1].Text)
	}
	// AND IT IS STILL A TYPE STEP, in the same place, with everything else untouched.
	if out[1].Kind != macroir.StepType {
		t.Errorf("the step is now a %q", out[1].Kind)
	}
	if out[0].Text != "notepad" || out[2].Key != "enter" || out[3].Ms != 50 {
		t.Errorf("the rest of the play moved: %+v", out)
	}
	// AND THE REVIEW SAYS WHAT IT WAS, which is what makes keeping it an informed choice:
	// the text is about to stop being part of the routine.
	if len(says) != 1 || !strings.Contains(says[0], "message") ||
		!strings.Contains(says[0], "hello there") {
		t.Errorf("the review says %v", says)
	}
}

// AND WHAT THE ROUTINE TAKES IS WHAT `routes` SAYS IT TAKES.
//
// The claim that matters, and it is deliberately asked of the reader every surface uses rather
// than of the source text: `routes.Arguments` is what Studio's field beside Play is built from,
// what `CheckArgs` refuses against, and what a request's proposal is asked about. A placeholder
// this produced that `routes` could not read back would be a step typing two braces and a word.
func TestAnEditedRoutineTakesTheNameItWasGiven(t *testing.T) {
	out, _, why := edited(t, StepEdit{Op: EditArgument, At: 1, Name: "message"})
	if why != "" {
		t.Fatalf("it refused: %s", why)
	}
	args := routes.Arguments(out[1].Text)
	if len(args) != 1 {
		t.Fatalf("routes reads %+v from %q", args, out[1].Text)
	}
	if args[0].Name != "message" {
		t.Errorf("it is called %q", args[0].Name)
	}
	if args[0].Secret {
		t.Error("a supplied text argument is reported as a secret")
	}
}

// A NAME IS LOWERCASED, because every reader downstream compares them that way.
func TestASuppliedNameIsLowercased(t *testing.T) {
	out, _, why := edited(t, StepEdit{Op: EditArgument, At: 1, Name: "  Message  "})
	if why != "" {
		t.Fatalf("it refused: %s", why)
	}
	if out[1].Text != "{{message}}" {
		t.Errorf("the step types %q", out[1].Text)
	}
}

// AND A NAME `routes` COULD NOT READ BACK IS REFUSED.
//
// The one that would be silent: a name with a space or a brace in it writes a literal that LOOKS
// like a placeholder and is never filled, so the routine types the braces and the person's name
// at them. `phRe` is the pattern that decides, so this refuses exactly what it will not read.
func TestANameThatWouldNotBeReadBackIsRefused(t *testing.T) {
	for _, name := range []string{
		"", "   ", "the message", "message!", "{{message}}", "mes.sage",
		"mes/sage", "mes:sage", "mes\tsage",
	} {
		out, _, why := edited(t, StepEdit{Op: EditArgument, At: 1, Name: name})
		if why == "" {
			t.Errorf("%q was accepted, giving %q", name, out[1].Text)
			continue
		}
		if !strings.Contains(why, "not a name") {
			t.Errorf("%q was refused as %q", name, why)
		}
	}
}

// AND A NAME THE GENERATOR READS FROM THE CREDENTIAL STORE IS REFUSED.
//
// THE WORST AVAILABLE SURPRISE, which is why it is refused by name rather than explained in a
// tooltip. `codegen` resolves a placeholder called `password`, `token`, `pin`, `key` and a few
// others from the credential store instead of typing it — so somebody asking for text they supply
// and happening to choose one of those words would get a routine that types whatever is saved
// there. They cannot be expected to know the list.
func TestANameTheGeneratorMeansAsACredentialIsRefused(t *testing.T) {
	for _, name := range []string{"password", "pass", "passwd", "pwd", "pin", "secret",
		"token", "otp", "apikey", "api-key", "key", "PASSWORD", "Token"} {
		out, _, why := edited(t, StepEdit{Op: EditArgument, At: 1, Name: name})
		if why == "" {
			t.Errorf("%q was accepted, giving %q", name, out[1].Text)
			continue
		}
		if !strings.Contains(why, "credential store") {
			t.Errorf("%q was refused as %q", name, why)
		}
	}
	// AND AN ORDINARY NAME IS STILL FINE, so the refusal is about those words and not
	// about naming things.
	if _, _, why := edited(t, StepEdit{Op: EditArgument, At: 1,
		Name: "passphrase-to-type"}); why != "" {
		t.Errorf("an ordinary name was refused: %s", why)
	}
}

// AND A STEP THAT TYPES NOTHING HAS NOTHING TO SUPPLY.
func TestOnlyAStepThatTypesTextCanBeSupplied(t *testing.T) {
	for _, at := range []int{0, 2, 3} {
		_, _, why := edited(t, StepEdit{Op: EditArgument, At: at, Name: "message"})
		if why == "" {
			t.Errorf("step %d was made supplied and it types no text", at+1)
			continue
		}
		if !strings.Contains(why, "does not type any text") {
			t.Errorf("step %d was refused as %q", at+1, why)
		}
	}
}

// AND A STEP WHOSE TEXT IS ALREADY SUPPLIED IS NOT SUPPLIED AGAIN.
//
// A second pass would bury one placeholder inside another and the routine would type the braces.
func TestTextAlreadySuppliedIsNotSuppliedAgain(t *testing.T) {
	steps := typingSteps()
	steps[1].Text = "{{message}}"
	_, _, why, _, err := applyEdits(steps, []StepEdit{{Op: EditArgument, At: 1, Name: "other"}})
	if err == nil && why == "" {
		t.Fatal("a placeholder was made into a placeholder again")
	}
	said := why
	if err != nil {
		said = err.Error()
	}
	if !strings.Contains(said, "already supplied") {
		t.Errorf("it was refused as %q", said)
	}
}

// AND ONE OPERATION PER STEP, which is the rule the other two already follow.
func TestAStepIsNotBothSuppliedAndSomethingElse(t *testing.T) {
	for _, c := range []struct {
		why string
		ops []StepEdit
	}{
		{"supplied then removed", []StepEdit{
			{Op: EditArgument, At: 1, Name: "message"}, {Op: EditRemove, At: 1}}},
		{"removed then supplied", []StepEdit{
			{Op: EditRemove, At: 1}, {Op: EditArgument, At: 1, Name: "message"}}},
		// AND NOT "SUPPLIED AND GIVEN A DURATION", which cannot be asked: a step is
		// a Type or a Wait and never both, so one of the two operations is refused
		// for the step's kind before any collision could arise. A case for it would
		// pass on the wrong refusal and read as though the collision were checked.
	} {
		_, _, why, _, err := applyEdits(typingSteps(), c.ops)
		if err == nil && why == "" {
			t.Errorf("%s was accepted", c.why)
		}
	}
}

// AND A POSITION THAT IS NOT IN THE LIST IS REFUSED, like every other operation's.
func TestSupplyingAStepThatIsNotThereIsRefused(t *testing.T) {
	for _, at := range []int{-1, 4, 99} {
		_, _, _, _, err := applyEdits(typingSteps(),
			[]StepEdit{{Op: EditArgument, At: at, Name: "message"}})
		if err == nil {
			t.Errorf("step %d was accepted", at+1)
		}
	}
}

// AND A PLACEHOLDER IN A STEP LIST IS A DECLARATION, EXCEPT A CREDENTIAL'S NAME.
//
// `suppliedIn` is what lets an edited routine be edited AGAIN: the panel re-renders the stored
// steps to check they still reproduce the saved source, and `codegen` keeps a `{{name}}` in a
// typed literal only when the name is declared. Two rules in it, and both are about what must
// NOT be declared.
func TestAPlaceholderInTheStepsDeclaresItselfExceptACredential(t *testing.T) {
	typed := func(text string) macroir.Step {
		return macroir.Step{Kind: macroir.StepType, Text: text}
	}
	for _, c := range []struct {
		why   string
		steps []macroir.Step
		known []string
		want  string
	}{
		{"an ordinary placeholder", []macroir.Step{typed("{{message}}")}, nil,
			"message"},
		{"one among fixed text", []macroir.Step{typed("Dear {{name}}, hello")}, nil,
			"name"},
		{"two of them", []macroir.Step{typed("{{greeting}} {{name}}")}, nil,
			"greeting name"},
		{"one the name already declared", []macroir.Step{typed("{{message}}")},
			[]string{"message"}, ""},
		{"the same one twice", []macroir.Step{typed("{{m}}"), typed("{{m}}")}, nil, "m"},
		// A CREDENTIAL'S NAME IS LEFT UNDECLARED. Declaring one changes what
		// `codegen` renders — a global secret becomes a route-qualified one — so a
		// routine already on disk with `{{password}}` typed in it would stop
		// matching its own source and become uneditable. `EditArgument` refuses
		// those names, so nothing this editor writes can need it.
		{"a credential's name", []macroir.Step{typed("{{password}}")}, nil, ""},
		{"a credential among others",
			[]macroir.Step{typed("{{token}} {{message}}")}, nil, "message"},
		// AND TEXT ON A STEP THAT DOES NOT TYPE. `Activate` carries an application
		// name in the same field, and reading it would declare an argument for a
		// routine that has none.
		{"an application name on an activate",
			[]macroir.Step{{Kind: macroir.StepActivate, Text: "{{notepad}}"}}, nil, ""},
		{"nothing at all", nil, nil, ""},
	} {
		got := strings.Join(suppliedIn(c.steps, c.known), " ")
		if got != c.want {
			t.Errorf("%s declares %q, want %q", c.why, got, c.want)
		}
	}
}

// AND THE OTHER TWO OPERATIONS STILL DO WHAT THEY DID.
//
// The control. A third case added to a switch is a chance to have changed the other two, and the
// list this is applied to is shared.
func TestTheOtherTwoOperationsAreUnchanged(t *testing.T) {
	out, _, why := edited(t, StepEdit{Op: EditWait, At: 3, To: 250})
	if why != "" || len(out) != 4 || out[3].Ms != 250 {
		t.Errorf("setting a wait gave %+v (%s)", out, why)
	}
	out, _, why = edited(t, StepEdit{Op: EditRemove, At: 2})
	if why != "" || len(out) != 3 {
		t.Fatalf("removing a step gave %+v (%s)", out, why)
	}
	if out[2].Kind != macroir.StepWait {
		t.Errorf("the wrong step was removed: %+v", out)
	}
	// AND A WORD NEITHER OF THEM KNOWS IS STILL REFUSED.
	if _, _, _, _, err := applyEdits(typingSteps(),
		[]StepEdit{{Op: EditOp("tidy"), At: 1}}); err == nil {
		t.Error("an operation nothing knows was accepted")
	}
}
