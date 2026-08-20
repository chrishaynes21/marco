package variables_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/variables"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The governing rules, as tests.
//
//	Variables remember MEANING, not pixels.
//	Every use is a fresh semantic resolution.
//	The remembered object is evidence, never authority.

// resolved is a resolution of the kind the Director produces before a capture.
func resolved() directorapi.Resolution {
	q := &directorapi.ElementQuery{
		Label: "Save", Role: directorapi.RoleButton,
		// The two fields that are true only of that instant. Capture must drop both.
		Window:  windowID("hwnd:4591094"),
		Ordinal: 2,
	}
	return directorapi.Resolution{
		Status: directorapi.ResolutionResolved,
		Target: &directorapi.ResolvedTarget{
			ElementID: "e42", WindowID: "hwnd:4591094",
			Point:       directorapi.Point{X: -1023, Y: 502},
			Role:        directorapi.RoleButton,
			Label:       "Save",
			NativeID:    "42.17.8",
			Confidence:  0.97,
			Query:       q,
			Explanation: "an exact label match on an enabled button",
		},
	}
}

func windowID(s string) *directorapi.WindowID {
	w := directorapi.WindowID(s)
	return &w
}

func world() *directorapi.WorldState {
	// ActiveWindow, not just Focused: FocusedWindow() reads the snapshot.s recorded
	// active window rather than scanning for a focused flag.
	active := directorapi.WindowID("hwnd:4591094")
	return &directorapi.WorldState{
		ActiveWindow: &active,
		Windows: []directorapi.Window{{
			ID: "hwnd:4591094", Application: "notepad",
			Title: "Untitled - Notepad", Focused: true,
		}},
	}
}

func TestCaptureKeepsTheQuestionAndDiscardsTheAnswer(t *testing.T) {
	// The whole design in one test. What made this control findable is kept; every
	// fact that was only true at that instant is thrown away.
	v, err := variables.Capture("save", variables.KindTarget, resolved(), world(), "remember this as save")
	if err != nil {
		t.Fatalf("capture: %v", err)
	}

	if v.Query == nil || v.Query.Label != "Save" {
		t.Fatalf("the semantic query was not kept: %+v", v.Query)
	}
	if v.Query.Window != nil {
		t.Fatal("the window handle was stored — that is a coordinate in disguise")
	}
	if v.Query.Ordinal != 0 {
		t.Fatal("an ordinal was stored; it describes a candidate list that will differ next time")
	}
	// Scoped to the APPLICATION, which survives a restart, rather than the window,
	// which does not.
	if v.Query.Application != "notepad" {
		t.Fatalf("application = %q, want notepad", v.Query.Application)
	}
}

func TestNoStoredFieldCanCarryATransientIdentity(t *testing.T) {
	// Checked against the SERIALISED form, because that is what survives a restart and
	// what a future field would silently join. A structural check on the Go type would
	// miss a field added later.
	v, err := variables.Capture("save", variables.KindTarget, resolved(), world(), "remember this")
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	blob := string(raw)
	for _, forbidden := range []string{
		"e42",     // element id
		"4591094", // window handle
		"42.17.8", // native runtime id
		"-1023",   // a coordinate
		"502",     // a coordinate
	} {
		if strings.Contains(blob, forbidden) {
			t.Fatalf("the stored variable carries %q — variables remember meaning, not pixels:\n%s",
				forbidden, blob)
		}
	}
}

func TestVariablesSurviveARestart(t *testing.T) {
	dir := t.TempDir()
	s, err := variables.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	v, _ := variables.Capture("save", variables.KindTarget, resolved(), world(), "remember this")
	if err := s.Put(v); err != nil {
		t.Fatalf("put: %v", err)
	}

	// A second Open is what a restart looks like from the store's point of view.
	again, err := variables.Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, ok := again.Get("save")
	if !ok {
		t.Fatal("the variable did not survive a restart")
	}
	if got.Query == nil || got.Query.Label != "Save" {
		t.Fatalf("the query did not survive: %+v", got.Query)
	}
	if got.Application != "notepad" {
		t.Fatalf("application = %q", got.Application)
	}
}

func TestAFutureSchemaIsRefusedRatherThanGuessedAt(t *testing.T) {
	// A newer Director may add a field whose ABSENCE changes meaning — a scope, an
	// expiry. Reading it under this build's rules would act on a variable the user
	// never agreed to.
	dir := t.TempDir()
	future := `{"version": 999, "variables": [{"name":"save","kind":"target"}]}`
	if err := os.WriteFile(filepath.Join(dir, "variables.json"), []byte(future), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := variables.Open(dir)
	if err == nil {
		t.Fatal("a future schema was loaded")
	}
	if !strings.Contains(err.Error(), "newer Director") {
		t.Fatalf("err = %v, want it to explain the version refusal", err)
	}
}

func TestAMissingStoreIsNotAnError(t *testing.T) {
	// A Director that nobody has told anything has no variables. That is a
	// normal state, not a fault.
	s, err := variables.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if len(s.All()) != 0 {
		t.Fatal("an empty store reported variables")
	}
}

func TestAnUnknownVariableIsNamedRatherThanGuessedAt(t *testing.T) {
	s, _ := variables.Open(t.TempDir())
	if _, ok := s.Get("save"); ok {
		t.Fatal("an unstored variable was found")
	}
	err := &variables.ErrUnknown{Name: "save"}
	if got := err.Error(); got != "Unknown variable: save" {
		t.Fatalf("message = %q", got)
	}
}

func TestAStaleVariableSaysTheWorldChangedNotThatItIsUnknown(t *testing.T) {
	// Two different problems. Unknown means "you never told me this"; stale means
	// "you did, and it is not here now" — which tells the user to re-capture rather
	// than to re-type.
	v, _ := variables.Capture("save", variables.KindTarget, resolved(), world(), "remember this")
	err := variables.StaleFor(v, directorapi.Resolution{Status: directorapi.ResolutionAbsent})
	if !strings.Contains(err.Error(), "cannot be found in the current world") {
		t.Fatalf("err = %v", err)
	}

	// Unobservable is NOT absence. Saying the button is gone when the application
	// merely did not expose its interior is a claim about the wrong thing.
	err = variables.StaleFor(v, directorapi.Resolution{Status: directorapi.ResolutionUnobservable})
	if !strings.Contains(err.Error(), "not evidence that it is gone") {
		t.Fatalf("err = %v, want it to distinguish unobservable from absent", err)
	}
}

func TestADuplicateNameIsRefusedRatherThanOverwritten(t *testing.T) {
	// A variable is knowledge the user built. Silently replacing it loses something
	// they cannot recover by retyping.
	s, _ := variables.Open(t.TempDir())
	v, _ := variables.Capture("save", variables.KindTarget, resolved(), world(), "remember this")
	if err := s.Put(v); err != nil {
		t.Fatalf("put: %v", err)
	}
	err := s.Put(v)
	if err == nil {
		t.Fatal("a duplicate name silently overwrote the existing variable")
	}
	var exists *variables.ErrExists
	if !asErr(err, &exists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	// Replacing is allowed when asked for explicitly.
	if err := s.Replace(v); err != nil {
		t.Fatalf("replace: %v", err)
	}
}

func TestRenameRefusesToClobber(t *testing.T) {
	s, _ := variables.Open(t.TempDir())
	a, _ := variables.Capture("save", variables.KindTarget, resolved(), world(), "remember this")
	b, _ := variables.Capture("store", variables.KindTarget, resolved(), world(), "remember this")
	_ = s.Put(a)
	_ = s.Put(b)

	if err := s.Rename("save", "store"); err == nil {
		t.Fatal("rename clobbered an existing variable")
	}
	if err := s.Rename("save", "keep"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, ok := s.Get("keep"); !ok {
		t.Fatal("the renamed variable is missing")
	}
	if _, ok := s.Get("save"); ok {
		t.Fatal("the old name still resolves")
	}
}

func TestForgetRemovesAndPersists(t *testing.T) {
	dir := t.TempDir()
	s, _ := variables.Open(dir)
	v, _ := variables.Capture("save", variables.KindTarget, resolved(), world(), "remember this")
	_ = s.Put(v)

	if err := s.Forget("save"); err != nil {
		t.Fatalf("forget: %v", err)
	}
	again, _ := variables.Open(dir)
	if _, ok := again.Get("save"); ok {
		t.Fatal("a forgotten variable came back after a restart")
	}
	if err := s.Forget("save"); err == nil {
		t.Fatal("forgetting an unknown variable succeeded")
	}
}

func TestNamesAreCaseInsensitiveAndValidated(t *testing.T) {
	// Spoken as often as typed: "remember this as Save" then "click $save" is one
	// variable, and case-sensitivity would turn a recognition capitalisation into a
	// missing variable.
	for _, in := range []string{"Save", "SAVE", "$save", " save "} {
		got, err := variables.NormalizeName(in)
		if err != nil || got != "save" {
			t.Fatalf("NormalizeName(%q) = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "9lives", "my var", "it", "that", "stop", "a-b"} {
		if _, err := variables.NormalizeName(bad); err == nil {
			t.Fatalf("%q was accepted as a name", bad)
		}
	}
}

func TestHistoryDistinguishesAWorkingVariableFromABrokenOne(t *testing.T) {
	// A variable that resolved yesterday and fails today is a different problem from
	// one that never worked. Only a record can tell them apart.
	dir := t.TempDir()
	s, _ := variables.Open(dir)
	v, _ := variables.Capture("save", variables.KindTarget, resolved(), world(), "remember this")
	_ = s.Put(v)

	s.RecordResolution("save", "Save")
	got, _ := s.Get("save")
	if got.History.Uses != 1 || got.History.LastResolvedAt == nil {
		t.Fatalf("a successful resolution was not recorded: %+v", got.History)
	}

	s.RecordFailure("save", "nothing matching Save is present")
	got, _ = s.Get("save")
	if got.History.LastFailure == "" || got.History.LastFailedAt == nil {
		t.Fatal("a failure was not recorded")
	}
	// And it survives, because the failure is the diagnosis.
	again, _ := variables.Open(dir)
	after, _ := again.Get("save")
	if after.History.LastFailure == "" {
		t.Fatal("the failure record did not persist")
	}
}

func TestTextVariablesCarryTheirProvenance(t *testing.T) {
	v, err := variables.CaptureText("customer", "Ada Lovelace", "selection", "remember the selected text")
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if v.Text != "Ada Lovelace" {
		t.Fatalf("text = %q", v.Text)
	}
	if v.Provenance.Source != "selection" {
		t.Fatalf("source = %q — a captured value must say where it came from", v.Provenance.Source)
	}
	// Text is a value, not a control. Asking for a query must refuse rather than
	// invent one.
	if _, err := variables.QueryFor(v); err == nil {
		t.Fatal("remembered text produced a control query")
	}
}

// asErr is errors.As without the import ceremony at each call site.
func asErr[T error](err error, target *T) bool {
	for err != nil {
		if v, ok := err.(T); ok {
			*target = v
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
