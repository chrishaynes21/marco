package uiaclient

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// These tests run against REAL UI Automation trees, recorded from real windows by
// plugins/uia/record-fixtures.ps1 and checked in under fixtures/. Nothing here
// touches a live desktop: the fixture is replayed through a fake host, so the whole
// chain — wire format, decoding, role normalisation, observation mapping — is
// exercised deterministically and in CI.
//
// Re-record with: powershell -File plugins/uia/record-fixtures.ps1

// fixtureHost replays a recorded snapshot as if the bridge had produced it. It
// decodes the fixture exactly the way bridgehost does (JSON → runtime.Value), so a
// mistake in the wire contract shows up here rather than only against live hardware.
type fixtureHost struct {
	raw       []byte
	status    string
	errMsg    string
	lastInput runtime.Value
	calls     int
}

func (h *fixtureHost) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	h.calls++
	h.lastInput = c.Input
	if h.status == "failed" {
		return "failed", runtime.ErrVal(&runtime.Err{Message: h.errMsg}), nil
	}
	var decoded any
	if err := json.Unmarshal(h.raw, &decoded); err != nil {
		return "failed", runtime.ErrVal(&runtime.Err{Message: err.Error()}), nil
	}
	return "ok", runtime.ValueFromJSON(decoded), nil
}

// loadFixture returns a host replaying the named fixture's accessibility snapshot.
func loadFixture(t *testing.T, name string) *fixtureHost {
	t.Helper()
	path := filepath.Join("..", "..", "..", "fixtures", name, "accessibility.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading fixture %s: %v\n"+
			"Record fixtures with: powershell -File plugins/uia/record-fixtures.ps1", name, err)
	}
	return &fixtureHost{raw: raw}
}

// fixedTime makes conversions byte-reproducible.
func provider(h runtime.Host) *Provider {
	p := New(h)
	p.now = func() time.Time { return time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC) }
	return p
}

// snapshot loads a fixture and converts it, failing the test on error.
func snapshot(t *testing.T, name string) directorapi.AccessibilitySnapshot {
	t.Helper()
	snap, err := provider(loadFixture(t, name)).Snapshot(context.Background(), "")
	if err != nil {
		t.Fatalf("Snapshot(%s): %v", name, err)
	}
	return snap
}

// find returns the observations whose label matches exactly.
func find(snap directorapi.AccessibilitySnapshot, label string) []directorapi.Observation {
	var out []directorapi.Observation
	for _, o := range snap.Observations {
		if o.Label == label {
			out = append(out, o)
		}
	}
	return out
}

// This is the vertical slice's perception step, end to end: a real Save dialog's
// accessibility tree becomes observations the Director can rank.
func TestSaveDialogFixture(t *testing.T) {
	snap := snapshot(t, "save-dialog")

	if len(snap.Observations) == 0 {
		t.Fatal("no observations decoded")
	}
	if snap.Partial {
		t.Errorf("the fixture should be a complete walk, got Partial: %s", snap.Reason)
	}
	if len(snap.Windows) != 1 {
		t.Fatalf("want 1 window, got %d", len(snap.Windows))
	}
	if got := snap.Windows[0].Title; got != "Save As" {
		t.Errorf("window title = %q, want \"Save As\"", got)
	}

	// The dialog contains SEVERAL things labelled "Save", which is exactly why the
	// Director ranks on role rather than matching text: an inert label, a menu, and
	// the button that is actually the target.
	saves := find(snap, "Save")
	if len(saves) < 2 {
		t.Fatalf("want several elements labelled Save (the ambiguity this fixture exists for), got %d", len(saves))
	}

	var button *directorapi.Observation
	roles := map[directorapi.ElementRole]int{}
	for i := range saves {
		roles[saves[i].Role]++
		if saves[i].Role == directorapi.RoleButton {
			button = &saves[i]
		}
	}
	if button == nil {
		t.Fatalf("no Save BUTTON among the Save-labelled elements (roles seen: %v)", roles)
	}
	if roles[directorapi.RoleText] == 0 {
		t.Error("the inert Save label should also be present — it is the decoy this fixture tests")
	}

	// The button must be usable and positioned, or it isn't a clickable target.
	if button.Enabled == nil || !*button.Enabled {
		t.Error("the Save button should be enabled")
	}
	if button.Bounds.Empty() {
		t.Error("the Save button must have real bounds — an empty rect is not clickable")
	}
	if button.Source != directorapi.SourceAccessibility {
		t.Errorf("source = %q, want accessibility", button.Source)
	}
	if button.NativeID == "" {
		t.Error("a UIA RuntimeId must survive as NativeID — identity matching depends on it")
	}

	// "Save As..." must NOT be confused with "Save"; it is a different element with
	// a different label, and only exact-label ranking keeps them apart.
	if got := find(snap, "Save As..."); len(got) != 1 {
		t.Errorf("want exactly one \"Save As...\" menu item, got %d", len(got))
	}
	if got := find(snap, "Cancel"); len(got) != 1 {
		t.Errorf("want exactly one Cancel button, got %d", len(got))
	}
}

// Two identical labels in different groups: label alone cannot decide, so the tree
// structure has to survive the conversion for ranking to have anything to work with.
func TestDuplicateLabelsFixture(t *testing.T) {
	snap := snapshot(t, "duplicate-labels")

	applies := find(snap, "Apply")
	if len(applies) != 2 {
		t.Fatalf("want exactly 2 Apply buttons, got %d", len(applies))
	}
	if applies[0].NativeID == applies[1].NativeID {
		t.Error("two distinct controls must have distinct native IDs")
	}
	if applies[0].ID == applies[1].ID {
		t.Error("two distinct controls must produce distinct observation IDs")
	}
	// They are geometrically distinct, which is what "the one on the left" needs.
	if applies[0].Bounds == applies[1].Bounds {
		t.Error("the two Apply buttons should have different bounds")
	}

	// Parent linkage is what lets the Director say "the Apply in the Video group".
	// Without it, the two are indistinguishable and it can only ask.
	if applies[0].ParentNativeID == "" || applies[1].ParentNativeID == "" {
		t.Fatal("parent linkage must survive — it is the only thing distinguishing the two")
	}
	if applies[0].ParentNativeID == applies[1].ParentNativeID {
		t.Error("the two Apply buttons belong to different groups")
	}

	byID := map[string]directorapi.Observation{}
	for _, o := range snap.Observations {
		byID[o.NativeID] = o
	}
	groups := map[string]bool{}
	for _, a := range applies {
		parent, ok := byID[a.ParentNativeID]
		if !ok {
			t.Fatalf("parent %q is not in the snapshot", a.ParentNativeID)
		}
		groups[parent.Label] = true
	}
	if !groups["Audio"] || !groups["Video"] {
		t.Errorf("want the Apply buttons under the Audio and Video groups, got %v", groups)
	}
}

// A disabled control is a resolution RESULT, not an absence. The Director should be
// able to say "Save is greyed out" — which requires the disabled state to survive
// and the element to still be present.
func TestDisabledButtonFixture(t *testing.T) {
	snap := snapshot(t, "disabled-button")

	saves := find(snap, "Save")
	if len(saves) != 1 {
		t.Fatalf("want exactly one Save button, got %d", len(saves))
	}
	save := saves[0]
	if save.Enabled == nil {
		t.Fatal("accessibility knows the enabled state; it must not come through as unreported")
	}
	if *save.Enabled {
		t.Error("the Save button in this fixture is disabled")
	}
	// Present, positioned, but not actionable — the distinction the Director reports.
	if save.Bounds.Empty() {
		t.Error("a disabled button still has bounds")
	}

	discards := find(snap, "Discard")
	if len(discards) != 1 || discards[0].Enabled == nil || !*discards[0].Enabled {
		t.Error("the Discard button should be present and enabled")
	}

	// The checkbox's state is its value, which is how "is autosave on?" is answered
	// without a second read.
	for _, o := range snap.Observations {
		if o.Role == directorapi.RoleCheckbox {
			if o.Value == "" {
				t.Error("a checkbox should carry its toggle state as its value")
			}
			return
		}
	}
	t.Error("expected a checkbox in this fixture")
}

// Accessibility genuinely knows these states, unlike OCR. Collapsing "reported
// false" into "not reported" would let a source's silence read as a denial.
func TestStateFieldsAreReportedNotInferred(t *testing.T) {
	snap := snapshot(t, "save-dialog")
	for _, o := range snap.Observations {
		if o.Enabled == nil || o.Visible == nil || o.Focused == nil || o.Selected == nil {
			t.Fatalf("element %q left a state field unreported; accessibility always knows them", o.Label)
		}
	}
}

// A provider is a replaceable subprocess. Letting it inject an arbitrary role string
// would let it inject one that planning or policy special-cases.
func TestUnknownRolesAreNotPassedThrough(t *testing.T) {
	if got := normaliseRole("button"); got != directorapi.RoleButton {
		t.Errorf("a known role should map, got %q", got)
	}
	for _, bogus := range []string{"", "Button", "ControlType.Button", "superuser", "admin_button"} {
		if got := normaliseRole(bogus); got != directorapi.RoleUnknown {
			t.Errorf("normaliseRole(%q) = %q, want unknown", bogus, got)
		}
	}

	// Every role the provider can emit must be in the map, or real elements silently
	// degrade to unknown.
	snap := snapshot(t, "save-dialog")
	unknown := 0
	for _, o := range snap.Observations {
		if o.Role == directorapi.RoleUnknown {
			unknown++
		}
	}
	if unknown > len(snap.Observations)/2 {
		t.Errorf("%d of %d elements mapped to unknown — the role map is out of step with the provider",
			unknown, len(snap.Observations))
	}
}

// Observation IDs derive from UIA's RuntimeId, so converting the same fixture twice
// produces identical IDs. Fixture reproducibility and provenance both depend on it.
func TestConversionIsDeterministic(t *testing.T) {
	a := snapshot(t, "save-dialog")
	b := snapshot(t, "save-dialog")

	if len(a.Observations) != len(b.Observations) {
		t.Fatalf("element counts differ: %d vs %d", len(a.Observations), len(b.Observations))
	}
	for i := range a.Observations {
		if a.Observations[i].ID != b.Observations[i].ID {
			t.Fatalf("observation %d id differs: %q vs %q", i, a.Observations[i].ID, b.Observations[i].ID)
		}
	}
	if a.Observations[0].ID == "" {
		t.Error("observation IDs must not be empty")
	}
}

func TestSnapshotSendsBoundsAndScope(t *testing.T) {
	h := loadFixture(t, "save-dialog")
	p := provider(h)
	p.MaxNodes = 250
	p.Timeout = 1500 * time.Millisecond

	if _, err := p.Snapshot(context.Background(), "hwnd:4242"); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	in := h.lastInput.AsSet()
	if in == nil {
		t.Fatal("input should be a set")
	}
	if v, _ := in.Get("Window"); v.AsText() != "hwnd:4242" {
		t.Errorf("scope not forwarded, got %q", v.AsText())
	}
	if v, _ := in.Get("MaxNodes"); func() bool { n, _ := v.AsNumber(); return int(n) != 250 }() {
		t.Error("MaxNodes not forwarded")
	}
	if v, _ := in.Get("TimeoutMs"); func() bool { n, _ := v.AsNumber(); return int(n) != 1500 }() {
		t.Error("TimeoutMs not forwarded")
	}
}

// An unscoped snapshot must not send an empty Window: the provider treats a present
// but unusable scope as an error, and an empty string would be exactly that.
func TestUnscopedSnapshotOmitsWindow(t *testing.T) {
	h := loadFixture(t, "save-dialog")
	if _, err := provider(h).Snapshot(context.Background(), ""); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if _, present := h.lastInput.AsSet().Get("Window"); present {
		t.Error("an unscoped snapshot must omit Window entirely, not send an empty one")
	}
}

// A failing provider must surface as an error, never as an empty world. "No Save
// button exists" and "I couldn't look" lead to very different decisions.
func TestProviderFailureIsAnErrorNotAnEmptyWorld(t *testing.T) {
	h := &fixtureHost{status: "failed", errMsg: "the window exposes no automation element"}
	snap, err := provider(h).Snapshot(context.Background(), "")
	if err == nil {
		t.Fatal("a failed provider must return an error")
	}
	if len(snap.Observations) != 0 {
		t.Error("a failed snapshot must not return partial observations")
	}
	if !errors.Is(err, err) || len(err.Error()) == 0 {
		t.Error("the error should carry the provider's message")
	}
}

// Available answers a yes/no question. Every kind of failure means the same thing to
// the Director — degrade and say so — so none of them should escape as an error.
func TestAvailableIsAlwaysAnAnswer(t *testing.T) {
	ok := &fixtureHost{raw: []byte(`{"Available":true,"Reason":""}`)}
	if !provider(ok).Available(context.Background()) {
		t.Error("want available")
	}

	no := &fixtureHost{raw: []byte(`{"Available":false,"Reason":"no foreground automation element"}`)}
	if provider(no).Available(context.Background()) {
		t.Error("want unavailable")
	}

	dead := &fixtureHost{status: "failed", errMsg: "bridge closed without responding"}
	if provider(dead).Available(context.Background()) {
		t.Error("a dead bridge is not available")
	}

	junk := &fixtureHost{raw: []byte(`"not an object"`)}
	if provider(junk).Available(context.Background()) {
		t.Error("an undecodable reply is not available")
	}
}

// A partial walk must stay flagged. Losing this turns "I stopped looking" into
// "there is nothing there" — the one confusion that makes a target search unsafe.
func TestPartialWalkStaysFlagged(t *testing.T) {
	h := &fixtureHost{raw: []byte(`{
		"WindowId":"hwnd:1","WindowTitle":"Big App","App":"chrome",
		"Partial":true,"Reason":"node limit (1500) reached",
		"Elements":[{"Id":"uia:1","Role":"button","Label":"OK","X":1,"Y":2,"W":3,"H":4,"Enabled":true,"Visible":true}]
	}`)}
	snap, err := provider(h).Snapshot(context.Background(), "")
	if err != nil {
		t.Fatalf("a partial walk is a usable result, not an error: %v", err)
	}
	if !snap.Partial {
		t.Fatal("Partial must survive the conversion")
	}
	if snap.Reason == "" {
		t.Error("a partial walk must say why it stopped")
	}
	if len(snap.Observations) != 1 {
		t.Error("a partial walk still returns what it did see")
	}
}
