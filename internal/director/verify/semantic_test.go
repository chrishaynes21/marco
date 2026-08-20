package verify_test

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/verify"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Verification's half of the milestone:
//
//	Every action needs its own semantic verification.
//	Verification must never rely solely on visual change.
//
// Each verb is proved by ITS OWN evidence: an expand by the node reporting itself
// expanded or by its children appearing, a dismiss by the thing being gone. "Something
// changed" is corroboration, never a verdict.

var base = time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

// worldOf builds a world at a given tick, from elements.
func worldOf(tick int, els ...*directorapi.Element) directorapi.WorldState {
	m := map[directorapi.ElementID]*directorapi.Element{}
	for _, e := range els {
		m[e.ID] = e
	}
	return directorapi.WorldState{
		Timestamp: base.Add(time.Duration(tick) * time.Second),
		Elements:  m,
		Windows: []directorapi.Window{{
			ID: "hwnd:1", Application: "explorer", Title: "Files",
			Focused: true, Visible: true,
		}},
	}
}

func node(id, label string, attrs map[string]any) *directorapi.Element {
	return &directorapi.Element{
		ID: directorapi.ElementID(id), Label: label, Role: directorapi.RoleTreeItem,
		WindowID: "hwnd:1", Enabled: true, Visible: true, Confidence: 1,
		Attributes: attrs,
	}
}

func child(id, parent string) *directorapi.Element {
	p := directorapi.ElementID(parent)
	return &directorapi.Element{
		ID: directorapi.ElementID(id), Label: id, Role: directorapi.RoleTreeItem,
		WindowID: "hwnd:1", Enabled: true, Visible: true, Confidence: 1, ParentID: &p,
	}
}

func target(id, label string) directorapi.ResolvedTarget {
	return directorapi.ResolvedTarget{
		ElementID: directorapi.ElementID(id), Label: label,
		Role: directorapi.RoleTreeItem, WindowID: "hwnd:1", Confidence: 1,
	}
}

func expand(kind directorapi.SemanticActionKind) directorapi.SemanticAction {
	return directorapi.SemanticAction{
		Kind: kind,
		Target: directorapi.ElementReference{
			Query: &directorapi.ElementQuery{Label: "Explorer"},
		},
	}
}

func TestAnExpandIsProvedByTheControlsOwnReportedState(t *testing.T) {
	before := worldOf(0, node("e1", "Explorer", map[string]any{"expanded": false}))
	after := worldOf(1, node("e1", "Explorer", map[string]any{"expanded": true}))

	res := verify.New().Verify(expand(directorapi.SemanticExpand), target("e1", "Explorer"), before, after)
	if !res.Success {
		t.Fatalf("an expand that reports itself expanded was not verified: %s", res.Reason)
	}
}

// TestAnExpandIsProvedByItsChildrenWhenTheStateIsNotReported is what carries the
// applications whose providers never report the state — which is most trees.
func TestAnExpandIsProvedByItsChildrenWhenTheStateIsNotReported(t *testing.T) {
	before := worldOf(0, node("e1", "Explorer", nil))
	after := worldOf(1, node("e1", "Explorer", nil), child("kid1", "e1"), child("kid2", "e1"))

	res := verify.New().Verify(expand(directorapi.SemanticExpand), target("e1", "Explorer"), before, after)
	if !res.Success {
		t.Fatalf("children appearing under the node did not verify the expand: %s", res.Reason)
	}
}

// TestTheControlsOwnDenialOutranksAnythingElse is the rule that keeps a busy screen from
// verifying an action that did not happen.
func TestTheControlsOwnDenialOutranksAnythingElse(t *testing.T) {
	// The node still says collapsed, and meanwhile a pile of unrelated elements
	// appeared — a notification, a repaint, anything.
	before := worldOf(0, node("e1", "Explorer", map[string]any{"expanded": false}))
	after := worldOf(1,
		node("e1", "Explorer", map[string]any{"expanded": false}),
		child("noise1", "somewhere"), child("noise2", "somewhere"))

	res := verify.New().Verify(expand(directorapi.SemanticExpand), target("e1", "Explorer"), before, after)
	if res.Success {
		t.Fatal("an expand was verified while the control reported itself still collapsed; " +
			"the application answering the exact question must outrank movement elsewhere")
	}
}

func TestAnExpandWithNoEvidenceAtAllFails(t *testing.T) {
	before := worldOf(0, node("e1", "Explorer", nil))
	after := worldOf(1, node("e1", "Explorer", nil))

	res := verify.New().Verify(expand(directorapi.SemanticExpand), target("e1", "Explorer"), before, after)
	if res.Success {
		t.Fatal("an expand verified with nothing to show for it")
	}
}

func TestACollapseIsProvedByChildrenDisappearing(t *testing.T) {
	before := worldOf(0, node("e1", "Explorer", nil), child("kid1", "e1"))
	after := worldOf(1, node("e1", "Explorer", nil))

	res := verify.New().Verify(expand(directorapi.SemanticCollapse), target("e1", "Explorer"), before, after)
	if !res.Success {
		t.Fatalf("a collapse whose children vanished was not verified: %s", res.Reason)
	}
}

func TestACheckIsProvedByTheCheckedState(t *testing.T) {
	before := worldOf(0, node("e1", "Remember me", map[string]any{"checked": false}))
	after := worldOf(1, node("e1", "Remember me", map[string]any{"checked": true}))

	act := directorapi.SemanticAction{Kind: directorapi.SemanticCheck,
		Target: directorapi.ElementReference{Query: &directorapi.ElementQuery{Label: "Remember me"}}}
	res := verify.New().Verify(act, target("e1", "Remember me"), before, after)
	if !res.Success {
		t.Fatalf("a checked box was not verified: %s", res.Reason)
	}
}

// TestAnUnreadableStateIsInconclusiveRatherThanFailed keeps the Director honest in the
// direction that matters: the box may well be ticked, and claiming it is not would be
// as wrong as claiming it is.
func TestAnUnreadableStateIsInconclusiveRatherThanFailed(t *testing.T) {
	before := worldOf(0, node("e1", "Remember me", nil))
	after := worldOf(1, node("e1", "Remember me", nil))

	act := directorapi.SemanticAction{Kind: directorapi.SemanticCheck,
		Target: directorapi.ElementReference{Query: &directorapi.ElementQuery{Label: "Remember me"}}}
	res := verify.New().Verify(act, target("e1", "Remember me"), before, after)
	if !res.Inconclusive {
		t.Errorf("a state that could not be read produced success=%v rather than an "+
			"inconclusive verdict: %s", res.Success, res.Reason)
	}
}

func TestAToggleIsProvedByTheStateBeingTheOtherOne(t *testing.T) {
	before := worldOf(0, node("e1", "Sidebar", map[string]any{"checked": false}))
	after := worldOf(1, node("e1", "Sidebar", map[string]any{"checked": true}))

	act := directorapi.SemanticAction{Kind: directorapi.SemanticToggle,
		Target: directorapi.ElementReference{Query: &directorapi.ElementQuery{Label: "Sidebar"}}}
	res := verify.New().Verify(act, target("e1", "Sidebar"), before, after)
	if !res.Success {
		t.Fatalf("a toggle that flipped was not verified: %s", res.Reason)
	}
}

func TestAToggleThatDidNotFlipFails(t *testing.T) {
	before := worldOf(0, node("e1", "Sidebar", map[string]any{"checked": true}))
	after := worldOf(1, node("e1", "Sidebar", map[string]any{"checked": true}))

	act := directorapi.SemanticAction{Kind: directorapi.SemanticToggle,
		Target: directorapi.ElementReference{Query: &directorapi.ElementQuery{Label: "Sidebar"}}}
	res := verify.New().Verify(act, target("e1", "Sidebar"), before, after)
	if res.Success {
		t.Fatal("a toggle verified while the state never changed")
	}
}

func TestADismissIsProvedByTheThingBeingGone(t *testing.T) {
	dialog := &directorapi.Element{
		ID: "d1", Label: "Save changes?", Role: directorapi.RoleDialog,
		WindowID: "hwnd:1", Enabled: true, Visible: true, Confidence: 1,
	}
	before := worldOf(0, dialog)
	after := worldOf(1)

	act := directorapi.SemanticAction{Kind: directorapi.SemanticDismiss}
	res := verify.New().Verify(act, target("d1", "Save changes?"), before, after)
	if !res.Success {
		t.Fatalf("a dialog that disappeared did not verify the dismiss: %s", res.Reason)
	}
}

func TestARefreshThatChangedNothingIsNotClaimedAsSuccess(t *testing.T) {
	before := worldOf(0, node("e1", "Row", nil))
	after := worldOf(1, node("e1", "Row", nil))

	act := directorapi.SemanticAction{Kind: directorapi.SemanticRefresh}
	res := verify.New().Verify(act, directorapi.ResolvedTarget{}, before, after)
	if res.Success {
		t.Fatal("a refresh that changed nothing reported success")
	}
	// The reason must say what it means: "back" at the start of the history does
	// nothing and reports nothing, and that is not a fault to be retried.
	if res.Reason == "" {
		t.Error("the failure carries no explanation")
	}
}

func TestAWindowVerbIsProvedByTheWindowsState(t *testing.T) {
	before := worldOf(0)
	after := worldOf(1)
	after.Windows[0].Maximized = true

	act := directorapi.SemanticAction{Kind: directorapi.SemanticMaximize}
	res := verify.New().Verify(act,
		directorapi.ResolvedTarget{WindowID: "hwnd:1"}, before, after)
	if !res.Success {
		t.Fatalf("a maximized window did not verify the maximize: %s", res.Reason)
	}
}

// TestAMinimizedWindowIsReportedMinimizedEvenWhenAlsoMaximized: a window manager
// remembers what a minimized window will restore TO, so both flags can be set at once.
func TestAMinimizedWindowIsReportedMinimizedEvenWhenAlsoMaximized(t *testing.T) {
	before := worldOf(0)
	after := worldOf(1)
	after.Windows[0].Minimized, after.Windows[0].Maximized = true, true

	act := directorapi.SemanticAction{Kind: directorapi.SemanticMinimize}
	res := verify.New().Verify(act,
		directorapi.ResolvedTarget{WindowID: "hwnd:1"}, before, after)
	if !res.Success {
		t.Fatalf("a minimized window was not verified as minimized: %s", res.Reason)
	}
}

// TestAnAfterStateThatIsNotAfterProvesNothing — comparing a snapshot against itself
// would "verify" anything.
func TestAnAfterStateThatIsNotAfterProvesNothing(t *testing.T) {
	w := worldOf(0, node("e1", "Explorer", map[string]any{"expanded": true}))
	res := verify.New().Verify(expand(directorapi.SemanticExpand), target("e1", "Explorer"), w, w)
	if !res.Inconclusive {
		t.Error("a world compared against itself produced a verdict")
	}
}
