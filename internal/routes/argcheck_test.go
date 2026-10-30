package routes

// WHAT A ROUTINE NEEDS, AND WHETHER IT HAS IT.
//
// The whole of this file is about one distinction and its consequences: **missing is not empty.**
// A routine that types what somebody supplied is the first thing in this product whose correctness
// depends on telling "they gave me nothing" from "they gave me nothing ON PURPOSE", and a length
// test gets the second one wrong in the direction that refuses what they meant.
//
// See [[Text-arguments-proposal-2026-10-30]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
)

// typesTheValue is one typed step as codegen leaves it: a press that handles its own failure.
//
// THE SHAPE MATTERS, because the fixtures below are checked BY THE COMPILER. A bare press at root
// leaves `failed` unhandled and will not compile whatever value is substituted into it — which is
// how the first draft of this file reported an escaping defect that was its own fixture.
func typesTheValue(ph string) string {
	return "do OS's Type with \"" + ph + "\"...\n" +
		"    when ok?\n        log \"done\".\n    or?\n        log that's error.\n"
}

// oneTextArg is a routine that types a value somebody supplies.
var oneTextArg = "use os.\n\nthe App is a script.\n\n" + typesTheValue("{{message}}")

// twoTextArgs declares them in the order the routine uses them, which is what a positional
// invocation zips onto.
var twoTextArgs = "use os.\n\nthe App is a script.\n\n" +
	typesTheValue("{{person}}") + typesTheValue("{{message}}")

// aTextAndASecret is the mixed case: one value typed, one read from the credential store.
var aTextAndASecret = "use os.\n\nthe App is a script.\n\n" +
	typesTheValue("{{user}}") + "do OS's Secret with \"log-in:password\".\n"

// AN ARGUMENT'S KIND IS PART OF WHAT A ROUTINE SAYS IT NEEDS.
//
// `ArgNames` returns both kinds in one list, so anything reading it as uniformly "text to type"
// would put a password in a text box. This is the same list with the kinds kept.
func TestARoutineSaysWhichValuesAreTypedAndWhichAreSecret(t *testing.T) {
	got := Arguments(aTextAndASecret)
	if len(got) != 2 {
		t.Fatalf("the routine accepts %+v", got)
	}
	if got[0] != (Argument{Name: "user"}) {
		t.Errorf("the typed value reads as %+v", got[0])
	}
	if got[1] != (Argument{Name: "password", Secret: true}) {
		t.Errorf("the secret reads as %+v", got[1])
	}
	// AND THE OLD LISTING IS UNCHANGED, because `marco args` and the overlay's labels are
	// built from it and an order that moved would reorder what people read.
	if strings.Join(ArgNames(aTextAndASecret), "|") != "user|password" {
		t.Errorf("the names came back as %q", ArgNames(aTextAndASecret))
	}
}

// A ROUTINE THAT TYPES SUPPLIED TEXT, INVOKED WITHOUT IT, IS REFUSED.
//
// What it did instead: `ApplyArgs` left the placeholder empty, the routine typed nothing, and the
// run reported `performed`. The product already refuses the mirror case — arguments given to a
// play that takes none — because "dropping them silently would let somebody watch the wrong thing
// happen and believe they had asked for it", and this is that sentence with the subject changed.
func TestARoutineNeedingTextIsRefusedWithoutIt(t *testing.T) {
	err := CheckArgs(oneTextArg, nil, nil)
	if err == nil {
		t.Fatal("a routine that types supplied text was allowed to run with none")
	}
	if !strings.Contains(err.Error(), `"message"`) {
		t.Errorf("the refusal does not name the argument: %v", err)
	}
}

// AND AN EMPTY STRING SOMEBODY SUPPLIED IS A VALUE.
//
// THE CASE A LENGTH TEST GETS WRONG. Clearing a field is a thing people do, and a routine that
// types nothing where they asked for nothing is correct — so the test is whether a value was
// SUPPLIED, which a map answers and a length never can.
func TestAnExplicitlyEmptyValueIsAValue(t *testing.T) {
	if err := CheckArgs(oneTextArg, map[string]string{"message": ""}, nil); err != nil {
		t.Fatalf("an empty value somebody gave on purpose was refused: %v", err)
	}
	// AND IT SUBSTITUTES AS NOTHING, which is what they asked for.
	got := ApplyArgs(oneTextArg, map[string]string{"message": ""}, nil)
	if !strings.Contains(got, `Type with ""`) {
		t.Errorf("the routine would type something: %q", got)
	}
}

// AND WHITESPACE IS TEXT SOMEBODY TYPED.
//
// Not trimmed and not collapsed. A leading space in a typed field is a leading space.
func TestAValueIsUsedExactlyAsSupplied(t *testing.T) {
	const padded = "  two  spaces  "
	if err := CheckArgs(oneTextArg, map[string]string{"message": padded}, nil); err != nil {
		t.Fatalf("a value that is mostly whitespace was refused: %v", err)
	}
	if got := ApplyArgs(oneTextArg, map[string]string{"message": padded}, nil); !strings.Contains(
		got, `Type with "`+padded+`"`) {
		t.Errorf("the value was altered on its way in: %q", got)
	}
}

// A VALUE FOR A NAME THE ROUTINE DOES NOT TAKE IS REFUSED, NOT DROPPED.
//
// Somebody expecting text to appear somewhere, and the routine has nowhere to put it. The refusal
// says what the routine does take, because that is the next thing they need.
func TestAValueForAnUnknownNameIsRefused(t *testing.T) {
	err := CheckArgs(oneTextArg, map[string]string{"message": "hi", "subject": "x"}, nil)
	if err == nil {
		t.Fatal("a value for an argument the routine does not have was accepted")
	}
	for _, want := range []string{`"subject"`, `"message"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %s: %v", want, err)
		}
	}
}

// AND MORE POSITIONAL VALUES THAN THE ROUTINE TAKES IS THE SAME ANSWER.
func TestMoreValuesThanArgumentsIsRefused(t *testing.T) {
	if err := CheckArgs(oneTextArg, nil, []string{"one", "two"}); err == nil {
		t.Fatal("two values were accepted for a routine that takes one")
	}
}

// A POSITIONAL VALUE COUNTS AS SUPPLYING THE NAME IT LANDS ON.
//
// `with chris` IS `person:chris`, and the check has to agree with the substitution exactly —
// which is why the zip has one owner. Two copies would disagree the first time either changed,
// and the disagreement would read as "it refused and then typed it anyway".
func TestAPositionalValueSuppliesTheNameItLandsOn(t *testing.T) {
	if err := CheckArgs(twoTextArgs, nil, []string{"chris", "hello"}); err != nil {
		t.Fatalf("positional values were not seen as supplying the names: %v", err)
	}
	got := ApplyArgs(twoTextArgs, nil, []string{"chris", "hello"})
	if !strings.Contains(got, `"chris"`) || !strings.Contains(got, `"hello"`) {
		t.Errorf("the values did not land on their names: %q", got)
	}
	// AND ONE SHORT IS STILL ONE SHORT.
	if err := CheckArgs(twoTextArgs, nil, []string{"chris"}); err == nil {
		t.Fatal("a routine taking two values ran with one")
	}
}

// A SECRET-TYPED ARGUMENT IS NOT REQUIRED HERE.
//
// It comes from the credential store, and a run that cannot find it refuses at the moment it is
// needed with the sentence that says how to set it. Requiring it here would refuse every routine
// with a password on a path that has nothing to do with passwords.
func TestASecretArgumentIsNotRequiredAsText(t *testing.T) {
	if err := CheckArgs(aTextAndASecret, map[string]string{"user": "chris"}, nil); err != nil {
		t.Fatalf("a routine with a secret was refused for not being given one: %v", err)
	}
	// AND THE TEXT ONE BESIDE IT IS STILL REQUIRED.
	if err := CheckArgs(aTextAndASecret, nil, nil); err == nil {
		t.Fatal("the typed value was not required because a secret sat beside it")
	}
}

// A VALUE THAT RUNS OVER MORE THAN ONE LINE IS REFUSED BEFORE ANYTHING RUNS.
//
// Because the substitution cannot write it down: `escapeArg` escapes a backslash and a quotation
// mark and nothing else, so a value carrying CR or LF produces a literal that does not lex and the
// routine fails at its own first line. That is the whole of the reason — a fact about the
// substitution as it stands, not a claim about what a demonstration can contain.
func TestAValueSpanningLinesIsRefused(t *testing.T) {
	for _, said := range []string{"line one\nline two", "line one\r\nline two",
		"trailing\n", "\rleading"} {
		err := CheckArgs(oneTextArg, map[string]string{"message": said}, nil)
		if err == nil {
			t.Errorf("a value spanning lines was accepted: %q", said)
			continue
		}
		if !strings.Contains(err.Error(), "more than one line") {
			t.Errorf("the refusal does not say why: %v", err)
		}
	}
}

// AND THE REST OF THE CHARACTER SPACE GOES THROUGH, written down so it still lexes.
//
// "Two replacements are enough" is exactly the kind of claim that is true until somebody pastes
// something, so a quotation mark, a backslash and text outside ASCII are exercised rather than
// assumed — and what they produce is checked as MARCO, by the compiler, not by reading the string.
func TestQuotesBackslashesAndUnicodeSurviveSubstitution(t *testing.T) {
	for _, said := range []string{
		`he said "hello"`,
		`a\backslash`,
		`both "and" \ together`,
		"café über été",
		"日本語 слово",
		"emoji \U0001F600",
		`a "quote\" that looks escaped`,
	} {
		if err := CheckArgs(oneTextArg, map[string]string{"message": said}, nil); err != nil {
			t.Errorf("%q was refused: %v", said, err)
			continue
		}
		filled := ApplyArgs(oneTextArg, map[string]string{"message": said}, nil)
		if err := driver.CheckSource(filled); err != nil {
			t.Errorf("%q produced source that will not compile: %v\n%s",
				said, err, filled)
		}
	}
}

// A SECRET-TYPED ARGUMENT MAY NOT BE SUPPLIED THROUGH A FLAG.
//
// One entrance may offer what another may not. A phrase carrying `password:value` has always
// reached `oshost.doSecret`, which uses it and REMEMBERS it in the credential store; whatever is
// thought of that, it is not something to acquire a second time through a flag that a window fills
// in and a command line carries. So the flag is refused, and the sentence says where the value is
// supposed to come from.
func TestASecretCannotBeSuppliedThroughAFlag(t *testing.T) {
	err := CheckFlagged(aTextAndASecret, []string{"user", "password"})
	if err == nil {
		t.Fatal("a secret was accepted from a flag, which is a second inline-secret " +
			"entrance")
	}
	if !strings.Contains(err.Error(), `"password"`) {
		t.Errorf("the refusal does not name it: %v", err)
	}
	if !strings.Contains(err.Error(), "credential store") {
		t.Errorf("the refusal does not say where the value comes from: %v", err)
	}
	// AND THE TEXT ONE BESIDE IT IS FINE THROUGH A FLAG.
	if err := CheckFlagged(aTextAndASecret, []string{"user"}); err != nil {
		t.Errorf("a typed value was refused from a flag: %v", err)
	}
	// AND NOTHING IS REFUSED WHEN NOTHING ARRIVED THAT WAY — the phrase path is
	// untouched, which is the half that keeps this from being a behaviour change.
	if err := CheckFlagged(aTextAndASecret, nil); err != nil {
		t.Errorf("a phrase carrying a secret was refused: %v", err)
	}
}
