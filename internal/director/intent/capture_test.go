package intent_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Both kinds of memory are spelled "remember". These tests are the parser's account of
// how it tells them apart, and of what it does when the phrase genuinely does not say.

func captureOf(t *testing.T, phrase string) values.Capture {
	t.Helper()
	in := intent.New().Parse(phrase)
	if in.Kind != directorapi.IntentAct {
		t.Fatalf("%q was not understood: %s", phrase, in.Ambiguity)
	}
	if in.Verb != intent.VerbCaptureValue {
		t.Fatalf("%q verb = %q, want %q", phrase, in.Verb, intent.VerbCaptureValue)
	}
	c, ok := in.Parameters[values.ParamCapture].(values.Capture)
	if !ok {
		t.Fatalf("%q carries no typed capture, only %#v", phrase, in.Parameters)
	}
	return c
}

func TestEveryCaptureFormParsesToItsOwnKind(t *testing.T) {
	for _, c := range []struct {
		phrase string
		kind   values.CaptureKind
		name   string
	}{
		{"remember the selected text as customer", values.CaptureSelectedText, "customer"},
		{"copy the selected text as query", values.CaptureSelectedText, "query"},
		{"remember this value as email", values.CaptureControlValue, "email"},
		{"remember this field's value as email", values.CaptureControlValue, "email"},
		{"read this field as customer", values.CaptureControlValue, "customer"},
		{"remember the value in the username field as username", values.CaptureControlValue, "username"},
		{"remember the clipboard as clip", values.CaptureClipboard, "clip"},
		{"remember what's on the clipboard as note", values.CaptureClipboard, "note"},
		{"remember the window title as title", values.CaptureWindowTitle, "title"},
		{"remember the active window title as title", values.CaptureWindowTitle, "title"},
		{"remember this window's title as source", values.CaptureWindowTitle, "source"},
		{`remember "John Smith" as customer`, values.CaptureLiteral, "customer"},
	} {
		got := captureOf(t, c.phrase)
		if got.Kind != c.kind {
			t.Errorf("%q kind = %s, want %s", c.phrase, got.Kind, c.kind)
		}
		if got.Name != c.name {
			t.Errorf("%q name = %q, want %q", c.phrase, got.Name, c.name)
		}
		if err := got.Validate(); err != nil {
			t.Errorf("%q produced an invalid capture: %v", c.phrase, err)
		}
	}
}

func TestACaptureIsNeverParsedAsATargetVariable(t *testing.T) {
	// The two memories have different lifetimes and different rules. Deciding later,
	// from leftover text, is how they would come to disagree.
	value := intent.New().Parse("remember this field's value as email")
	object := intent.New().Parse("remember this button as save")

	if value.Verb == object.Verb {
		t.Fatalf("both parsed to %q; the parser did not distinguish them", value.Verb)
	}
	if object.Verb != intent.VerbRemember {
		t.Errorf("remembering a control gave %q, want %q", object.Verb, intent.VerbRemember)
	}
	if _, isCapture := object.Parameters[values.ParamCapture]; isCapture {
		t.Error("remembering a control produced a value capture")
	}
	if _, isTarget := value.Parameters[intent.VariableName]; isTarget {
		t.Error("remembering a value produced a target variable")
	}
}

func TestABarePronounDoesNotSayWhichKindOfMemoryIsMeant(t *testing.T) {
	// "Remember this as customer" is exactly as likely to mean either. Picking one
	// silently would store the wrong kind of thing under a name the user will reuse.
	for _, phrase := range []string{
		"remember this as customer", "remember this as value", "remember that as thing",
		"remember it as x",
	} {
		in := intent.New().Parse(phrase)
		if in.Kind == directorapi.IntentAct {
			t.Errorf("%q was silently resolved to %q", phrase, in.Verb)
			continue
		}
		// A real question with both options in it, not a refusal dressed up as one.
		if !strings.Contains(in.Ambiguity, "control itself") ||
			!strings.Contains(in.Ambiguity, "value") {
			t.Errorf("%q asked %q, want both readings offered", phrase, in.Ambiguity)
		}
	}
}

func TestALiteralIsBoundExactlyAndNotInterpolated(t *testing.T) {
	// A literal that expanded ${...} would make quoting useless as a way of saying
	// "these exact characters".
	c := captureOf(t, `remember "${customer} and $save" as literal`)
	if c.Literal == nil || *c.Literal != "${customer} and $save" {
		t.Fatalf("literal = %v, want the exact characters", c.Literal)
	}
}

func TestALiteralCaptureNeedsNoTargetAndNoWorld(t *testing.T) {
	in := intent.New().Parse(`remember "John Smith" as customer`)
	if len(in.Targets) != 0 {
		t.Fatalf("a literal capture carries targets: %+v", in.Targets)
	}
	c := in.Parameters[values.ParamCapture].(values.Capture)
	if c.Kind.NeedsWorld() || c.Kind.NeedsTarget() {
		t.Fatal("a literal capture claims it needs to observe")
	}
}

func TestAControlReadResolvesTheFocusedControlOrTheNamedOne(t *testing.T) {
	focused := intent.New().Parse("remember this field's value as email")
	if len(focused.Targets) != 1 || focused.Targets[0].Query.Focused == nil {
		t.Fatalf("an unnamed control read did not target the focused one: %+v", focused.Targets)
	}
	named := intent.New().Parse("remember the value in the username field as username")
	if len(named.Targets) != 1 || named.Targets[0].Query.Label != "username field" {
		t.Fatalf("a named control read did not target it: %+v", named.Targets)
	}
}

func TestAValueReferenceInAnEditIsTypedRatherThanText(t *testing.T) {
	in := intent.New().Parse("type ${customer}")
	if in.Kind != directorapi.IntentAct {
		t.Fatalf("not understood: %s", in.Ambiguity)
	}
	input, ok := in.Parameters[values.ParamInput].(values.Input)
	if !ok {
		t.Fatalf("no typed input, only %#v", in.Parameters)
	}
	if !input.IsReference() || input.Reference.Name != "customer" {
		t.Fatalf("input = %+v, want a reference to customer", input)
	}
	// The unresolved token must not survive as text: a planner reading "${customer}"
	// out of Text would type those exact characters into the user's document.
	if strings.Contains(in.Text, "${") {
		t.Fatalf("the raw token survived in Text: %q", in.Text)
	}
}

func TestOrdinaryEditTextStaysLiteral(t *testing.T) {
	in := intent.New().Parse("type hello")
	input, ok := in.Parameters[values.ParamInput].(values.Input)
	if !ok {
		t.Fatal("no typed input")
	}
	if input.IsReference() {
		t.Fatal("plain text was read as a reference")
	}
	if in.Text != "hello" {
		t.Fatalf("Text = %q", in.Text)
	}
}

func TestAConcatenationIsRefusedAtParseTime(t *testing.T) {
	// Refused by name rather than typed literally with the braces in it.
	in := intent.New().Parse("type Dear ${customer},")
	if in.Kind == directorapi.IntentAct {
		t.Fatal("a concatenation was accepted")
	}
	if !strings.Contains(in.Ambiguity, "whole") {
		t.Fatalf("ambiguity = %q, want it to say a value may only be used whole", in.Ambiguity)
	}
}

func TestTheTwoNamespacesDoNotFallBackToEachOther(t *testing.T) {
	// `click $save` targets an object; `type ${save}` reads a value. Neither syntax may
	// resolve as the other, whichever namespace happens to contain the name.
	click := intent.New().Parse("click $save")
	if len(click.Targets) != 1 {
		t.Fatal("click lost its target")
	}
	if name, ok := intent.VariableTarget(click.Targets[0].Phrase); !ok || name != "save" {
		t.Fatalf("$save was not a target reference: %q", click.Targets[0].Phrase)
	}
	typed := intent.New().Parse("type ${save}")
	input := typed.Parameters[values.ParamInput].(values.Input)
	if !input.IsReference() || input.Reference.Name != "save" {
		t.Fatalf("${save} was not a value reference: %+v", input)
	}
	// And ${save} is not a target reference, nor $save a value one.
	if _, ok := intent.VariableTarget("${save}"); ok {
		t.Fatal("${save} was claimed by the object namespace")
	}
	if _, ok := values.ParseReference("$save"); ok {
		t.Fatal("$save was claimed by the value namespace")
	}
}
