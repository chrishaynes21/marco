package values_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/values"
)

// The data-flow half of the package, as tests.
//
//	Exactly one of literal or reference. A value is read when it is USED.
//	Concatenation is refused, not interpolated. A cleared program has no values.

func TestAnInputIsEitherLiteralOrReferenceButNeverBoth(t *testing.T) {
	if err := values.LiteralInput("hello").Validate(); err != nil {
		t.Fatalf("a literal was refused: %v", err)
	}
	if err := values.ReferenceInput("customer").Validate(); err != nil {
		t.Fatalf("a reference was refused: %v", err)
	}
	// Both and neither are reachable by building the struct directly, and both are
	// silent — neither would fail until the text was already going into a control.
	lit := "hello"
	both := values.Input{Literal: &lit, Reference: &values.Reference{Name: "customer"}}
	if err := both.Validate(); err == nil {
		t.Fatal("an input that was both literal and reference was accepted")
	}
	if err := (values.Input{}).Validate(); err == nil {
		t.Fatal("an empty input was accepted")
	}
}

func TestAWholeTokenIsAReferenceAndAnythingElseIsNot(t *testing.T) {
	in, err := values.ParseInput("${customer}")
	if err != nil || !in.IsReference() || in.Reference.Name != "customer" {
		t.Fatalf("ParseInput(${customer}) = %+v, %v", in, err)
	}
	// $save is the OBJECT namespace and must stay ordinary text here; neither syntax
	// may fall back to the other.
	for _, literal := range []string{"$save", "$5.00", "hello", "save"} {
		in, err := values.ParseInput(literal)
		if err != nil {
			t.Fatalf("ParseInput(%q) errored: %v", literal, err)
		}
		if in.IsReference() {
			t.Fatalf("%q was read as a value reference", literal)
		}
	}
}

func TestConcatenationIsRefusedRatherThanInterpolated(t *testing.T) {
	// Interpolating would be easy, and would immediately raise the question of what
	// "Dear ${name}," means when the capture is empty, unknown, or secret — three
	// answers this milestone does not have.
	for _, phrase := range []string{"Dear ${customer},", "${first} ${last}", "x${y}"} {
		_, err := values.ParseInput(phrase)
		if err == nil {
			t.Fatalf("%q was accepted", phrase)
		}
		var te *values.ErrTransformation
		if !asTransformation(err, &te) {
			t.Fatalf("%q gave %T, want a transformation refusal", phrase, err)
		}
	}
}

func asTransformation(err error, out **values.ErrTransformation) bool {
	t, ok := err.(*values.ErrTransformation)
	if ok {
		*out = t
	}
	return ok
}

func TestAReferenceIsReadWhenItIsUsedNotWhenItIsParsed(t *testing.T) {
	env := values.NewEnvironment()
	in := values.ReferenceInput("customer")

	// Before the capture, the same input refuses. That is the whole reason it is not a
	// string substitution: substitution has nothing to fail with.
	if _, err := in.Resolve(env); err == nil {
		t.Fatal("a reference resolved before anything was captured")
	}
	if err := env.Bind("customer", normal(t, "Alice")); err != nil {
		t.Fatalf("bind: %v", err)
	}
	got, err := in.Resolve(env)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Plaintext() != "Alice" {
		t.Fatalf("value = %q", got.Plaintext())
	}
}

func TestASecretValueCannotFlowIntoText(t *testing.T) {
	// A secret reaching a text operation would be embedded in generated Marco source,
	// which is exactly what the named-secret mechanism exists to avoid.
	env := values.NewEnvironment()
	if err := env.Bind("password", secret(t)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	_, err := values.ReferenceInput("password").Resolve(env)
	if err == nil {
		t.Fatal("a secret value was accepted as text")
	}
	if strings.Contains(err.Error(), secretText) {
		t.Fatalf("the refusal leaked the secret: %v", err)
	}
}

func TestAnUnknownFutureKindIsNotSilentlyStringified(t *testing.T) {
	// The default branch refuses. A kind added later must state its own text rule here
	// before it can be typed into somebody's document.
	v, err := values.New(values.Kind("geometry"), "1,2", "somewhere", values.VisibilityNormal)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := v.TextCompatible(); err == nil {
		t.Fatal("an unrecognised kind was accepted as text")
	}
}

func TestEveryCapturedKindThatShouldBeTextIs(t *testing.T) {
	for _, k := range []values.Kind{
		values.KindText, values.KindWindowTitle, values.KindClipboard, values.KindControlValue,
	} {
		v := mustNew(t, k, "x", "src")
		if err := v.TextCompatible(); err != nil {
			t.Errorf("%s is not text-compatible: %v", k, err)
		}
	}
}

func TestClassificationIsConservativeBySourceAndOnlyRaisedByContent(t *testing.T) {
	// The source is known before the content is read and is the more reliable signal:
	// anything typed into a control may be personal, whatever it happens to say.
	if got := values.Classify(values.KindControlValue, "hello"); got != values.VisibilitySensitive {
		t.Errorf("a control value classified %s, want sensitive", got)
	}
	if got := values.Classify(values.KindClipboard, "hello"); got != values.VisibilitySensitive {
		t.Errorf("clipboard classified %s, want sensitive", got)
	}
	if got := values.Classify(values.KindWindowTitle, "Untitled - Notepad"); got != values.VisibilityNormal {
		t.Errorf("a window title classified %s, want normal", got)
	}
	// Content raises, never lowers.
	if got := values.Classify(values.KindText, "someone@example.com"); got != values.VisibilitySensitive {
		t.Errorf("an email classified %s, want sensitive", got)
	}
	if got := values.Classify(values.KindText, "John Smith"); got != values.VisibilityNormal {
		t.Errorf("a literal name classified %s, want normal", got)
	}
	// Classification NEVER produces a secret. A secret is refused at the read, not
	// discovered after it.
	for _, k := range []values.Kind{
		values.KindText, values.KindWindowTitle, values.KindClipboard, values.KindControlValue,
	} {
		if values.Classify(k, "hunter2") == values.VisibilitySecret {
			t.Errorf("%s was classified secret; secrets are refused, not classified", k)
		}
	}
}

func TestACaptureCarriesExactlyTheFieldsItsKindNeeds(t *testing.T) {
	lit := "John Smith"
	ok := values.Capture{Kind: values.CaptureLiteral, Name: "customer", Literal: &lit}
	if err := ok.Validate(); err != nil {
		t.Fatalf("a valid literal capture was refused: %v", err)
	}
	// A literal with nothing to capture, and a desktop read carrying literal text, are
	// both incoherent — letting either through binds whichever field the executor
	// happened to read.
	if err := (values.Capture{Kind: values.CaptureLiteral, Name: "x"}).Validate(); err == nil {
		t.Fatal("a literal capture with no literal was accepted")
	}
	bad := values.Capture{Kind: values.CaptureClipboard, Name: "x", Literal: &lit}
	if err := bad.Validate(); err == nil {
		t.Fatal("a clipboard capture carrying literal text was accepted")
	}
	if err := (values.Capture{Kind: "invented", Name: "x"}).Validate(); err == nil {
		t.Fatal("an unknown capture kind was accepted")
	}
	if err := (values.Capture{Kind: values.CaptureClipboard, Name: "2bad"}).Validate(); err == nil {
		t.Fatal("an invalid name was accepted")
	}
}

func TestOnlyControlReadsNeedATargetAndOnlyLiteralsSkipTheWorld(t *testing.T) {
	// Running an accessibility walk for `remember "John Smith" as customer` would be
	// pure cost, and resolving a control for a clipboard read would be meaningless.
	for _, c := range []struct {
		kind   values.CaptureKind
		target bool
		world  bool
	}{
		{values.CaptureSelectedText, true, true},
		{values.CaptureControlValue, true, true},
		{values.CaptureClipboard, false, true},
		{values.CaptureWindowTitle, false, true},
		{values.CaptureLiteral, false, false},
	} {
		if got := c.kind.NeedsTarget(); got != c.target {
			t.Errorf("%s NeedsTarget = %v, want %v", c.kind, got, c.target)
		}
		if got := c.kind.NeedsWorld(); got != c.world {
			t.Errorf("%s NeedsWorld = %v, want %v", c.kind, got, c.world)
		}
		if c.kind.Produces() == "" {
			t.Errorf("%s produces no value kind", c.kind)
		}
	}
}

func TestClearingAnEnvironmentEndsItRatherThanEmptyingIt(t *testing.T) {
	// Relying on the environment becoming unreachable would satisfy the garbage
	// collector and not the guarantee: a secret would stay in memory for an unbounded
	// time, and anything holding a pointer could still read it.
	env := values.NewEnvironment()
	if err := env.Bind("password", secret(t)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if n := env.Clear(); n != 1 {
		t.Fatalf("Clear discarded %d values, want 1", n)
	}
	if !env.Cleared() || env.Count() != 0 {
		t.Fatal("the environment did not report itself finished")
	}
	if _, err := env.Resolve("password"); err == nil {
		t.Fatal("a value survived the end of its program")
	}
	// A cleared environment is not quietly reusable: binding into it would attach a
	// value to a program that is over.
	if err := env.Bind("customer", normal(t, "Alice")); err == nil {
		t.Fatal("a finished program accepted a new capture")
	}
	raw, _ := json.Marshal(env.Describe())
	if strings.Contains(string(raw), secretText) {
		t.Fatalf("a cleared environment still serialises the secret:\n%s", raw)
	}
}

func TestAPlanPreviewShowsTheReferenceAndNotItsValue(t *testing.T) {
	// The value does not exist when a plan is previewed, and printing it later would
	// leak whatever it turned out to be.
	if got := values.ReferenceInput("customer").Describe(); got != "${customer}" {
		t.Fatalf("Describe = %q", got)
	}
	if got := values.LiteralInput("hello").Describe(); got != `"hello"` {
		t.Fatalf("Describe = %q", got)
	}
}
