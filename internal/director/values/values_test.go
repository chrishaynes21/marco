package values_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/values"
)

// The governing rules, as tests.
//
//	Objects always re-resolve. Values never re-resolve.
//	Values are immutable. Values are data, never identity.
//	Unknown is not empty. Secrets never leak.

const secretText = "hunter2-correct-horse"

func normal(t *testing.T, text string) values.Value {
	t.Helper()
	v, err := values.New(values.KindText, text, "selection", values.VisibilityNormal)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	return v
}

func secret(t *testing.T) values.Value {
	t.Helper()
	v, err := values.New(values.KindControlValue, secretText, "password field", values.VisibilitySecret)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	return v
}

func TestAValueIsTypedRatherThanCollapsedToString(t *testing.T) {
	// The source changes what the value MEANS and what may safely be done with it. A
	// window title is public; a control value may be a password.
	for _, k := range []values.Kind{
		values.KindText, values.KindWindowTitle, values.KindClipboard, values.KindControlValue,
	} {
		v, err := values.New(k, "x", "src", values.VisibilityNormal)
		if err != nil {
			t.Fatalf("new %s: %v", k, err)
		}
		if v.Kind() != k {
			t.Fatalf("kind = %s, want %s", v.Kind(), k)
		}
	}
	if _, err := values.New("", "x", "src", values.VisibilityNormal); err == nil {
		t.Fatal("an untyped value was accepted")
	}
}

func TestAVerifiedEmptyValueIsRealButUnknownIsNot(t *testing.T) {
	// The distinction the whole package turns on. A field that WAS empty is a fact; a
	// field that could not be read is not, and binding it as "" would let a program
	// type nothing and report success.
	empty := normal(t, "")
	if !empty.Empty() {
		t.Fatal("an empty capture did not report itself empty")
	}
	env := values.NewEnvironment()
	if err := env.Bind("field", empty); err != nil {
		t.Fatalf("a verified empty value was refused: %v", err)
	}

	// Unknown produces no Value at all — there is no constructor for one.
	unknown := &values.ErrUnknown{Name: "customer", Source: "selection", Reason: "nothing is selected"}
	if !strings.Contains(unknown.Error(), "not an empty one") {
		t.Fatalf("err = %q, want it to distinguish unknown from empty", unknown)
	}
}

func TestAValueCannotBeChangedAfterCapture(t *testing.T) {
	// "If a field changes after capture, the stored value does not."
	env := values.NewEnvironment()
	if err := env.Bind("customer", normal(t, "Alice")); err != nil {
		t.Fatalf("bind: %v", err)
	}

	// Rebinding is refused: a name that could be rebound would let a later step change
	// what an earlier one captured — a mutable value through the namespace.
	if err := env.Bind("customer", normal(t, "Bob")); err == nil {
		t.Fatal("a captured value was rebound")
	}
	got, _ := env.Get("customer")
	if got.Plaintext() != "Alice" {
		t.Fatalf("value = %q, want the original capture", got.Plaintext())
	}

	// And the copy handed out cannot be altered — Value has no mutating method and no
	// exported field, so this is structural rather than conventional.
	got2, _ := env.Get("customer")
	if got2.Plaintext() != "Alice" {
		t.Fatal("a second read differed from the first")
	}
}

func TestASecretNeverRendersItself(t *testing.T) {
	// The load-bearing one. String() is what every %v, every log line and every error
	// message reaches for, so redaction has to live in the type rather than at each
	// call site — one forgotten format verb would leak a password.
	s := secret(t)

	for what, got := range map[string]string{
		"String":       s.String(),
		"Describe":     s.Describe(),
		"fmt %v":       fmt.Sprintf("%v", s),
		"fmt %s":       fmt.Sprintf("%s", s),
		"inside a msg": fmt.Sprintf("typing %v into the field", s),
	} {
		if strings.Contains(got, secretText) {
			t.Fatalf("%s leaked the secret: %q", what, got)
		}
		if !strings.Contains(got, values.Redacted) {
			t.Fatalf("%s = %q, want it redacted", what, got)
		}
	}

	// Only the host-facing accessor returns it, and it is the single findable call site.
	if s.Plaintext() != secretText {
		t.Fatal("Plaintext did not return the value the host needs")
	}
}

func TestASecretIsNotSerialisedEvenBySnapshot(t *testing.T) {
	env := values.NewEnvironment()
	if err := env.Bind("password", secret(t)); err != nil {
		t.Fatalf("bind: %v", err)
	}
	raw, err := json.Marshal(env.Describe())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), secretText) {
		t.Fatalf("the snapshot serialised the secret:\n%s", raw)
	}
	// Type, source and LENGTH stay reportable — enough to diagnose without revealing.
	if !strings.Contains(string(raw), `"length"`) {
		t.Fatal("the snapshot dropped the length, which is safe and useful")
	}
}

func TestSensitiveValuesAreRedactedToo(t *testing.T) {
	// Not a credential, but not for printing either — a customer name, an address.
	v, err := values.New(values.KindText, "Alice Smith", "selection", values.VisibilitySensitive)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if strings.Contains(v.String(), "Alice") {
		t.Fatalf("a sensitive value rendered itself: %q", v.String())
	}
	if v.Len() != len("Alice Smith") {
		t.Fatal("the length should still be reportable")
	}
}

func TestValueReferencesAreADifferentTokenFromObjectReferences(t *testing.T) {
	// ${customer} asks WHAT INFORMATION; $save asks WHICH OBJECT. Different tokens, so
	// the parser can tell them apart without knowing what has been captured — deciding
	// by lookup would make a phrase mean different things at different moments.
	ref, ok := values.ParseReference("${customer}")
	if !ok || ref.Name != "customer" {
		t.Fatalf("ParseReference = %+v, %v", ref, ok)
	}
	for _, notAValue := range []string{"$save", "customer", "${}", "${2bad}", "$ {x}"} {
		if _, ok := values.ParseReference(notAValue); ok {
			t.Fatalf("%q was read as a value reference", notAValue)
		}
	}
}

func TestAnUnknownValueIsNamedAsAValue(t *testing.T) {
	// A user who typed ${save} for $save needs to be told which namespace they missed.
	env := values.NewEnvironment()
	_, err := env.Resolve("customer")
	if err == nil {
		t.Fatal("an uncaptured value resolved")
	}
	if got := err.Error(); got != "Unknown value: customer" {
		t.Fatalf("message = %q", got)
	}
}

func TestTheNamespacesDoNotCollide(t *testing.T) {
	// A value called "save" and a target variable called "save" are different things
	// and must both be usable. The environment knows nothing about variables, which is
	// what guarantees it.
	env := values.NewEnvironment()
	if err := env.Bind("save", normal(t, "the text")); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if !env.Has("save") {
		t.Fatal("the value is missing")
	}
	// ${save} is the value; $save remains an object reference this package cannot see.
	if _, ok := values.ParseReference("$save"); ok {
		t.Fatal("an object reference was claimed by the value namespace")
	}
}

func TestAnEnvironmentIsBoundedAndRejectsRatherThanTruncates(t *testing.T) {
	env := values.NewEnvironment()
	for i := 0; i < values.MaxValues; i++ {
		if err := env.Bind(fmt.Sprintf("v%d", i), normal(t, "x")); err != nil {
			t.Fatalf("bind %d: %v", i, err)
		}
	}
	if err := env.Bind("overflow", normal(t, "x")); err == nil {
		t.Fatal("an unbounded number of values was accepted")
	}

	big := strings.Repeat("a", values.MaxTextLength+1)
	fresh := values.NewEnvironment()
	err := fresh.Bind("huge", normal(t, big))
	if err == nil {
		t.Fatal("an oversized value was accepted")
	}
	if !strings.Contains(err.Error(), "rather than shortened") {
		t.Fatalf("err = %v, want it to say it rejects rather than truncates", err)
	}
}

func TestAnEnvironmentHasNoPersistence(t *testing.T) {
	// Values belong to one program. There is deliberately no Save, no path and no
	// loader — a persisted value would be a fact about a screen nobody is looking at,
	// reused silently in a context it was never captured for.
	env := values.NewEnvironment()
	_ = env.Bind("customer", normal(t, "Alice"))

	// A new program gets a new environment, and knows nothing.
	next := values.NewEnvironment()
	if next.Has("customer") {
		t.Fatal("a value survived into a new program")
	}
	if _, err := next.Resolve("customer"); err == nil {
		t.Fatal("a value from a finished program still resolved")
	}
}

func TestAValueCarriesNoDesktopIdentity(t *testing.T) {
	// Values are DATA, never identity. Checked against the serialised snapshot, since
	// that is what would carry a field added later.
	env := values.NewEnvironment()
	_ = env.Bind("title", mustNew(t, values.KindWindowTitle, "Untitled - Notepad", "window"))
	raw, _ := json.Marshal(env.Describe())
	for _, forbidden := range []string{"hwnd", "element_id", "runtime_id", "bounds", "point"} {
		if strings.Contains(strings.ToLower(string(raw)), forbidden) {
			t.Fatalf("a value carries %q — values are data, never identity:\n%s", forbidden, raw)
		}
	}
}

func mustNew(t *testing.T, k values.Kind, text, src string) values.Value {
	t.Helper()
	v, err := values.New(k, text, src, values.VisibilityNormal)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	return v
}
