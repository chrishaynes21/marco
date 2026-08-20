package binding_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Backing-resource identity, and the caption that is not one.
//
//	A selected filesystem item in Windows Explorer exposes a canonical backing path.
//	Do not treat a caption as file identity.
//
// The live rename scenario stopped exactly here: Explorer's list item was a control
// captioned "Alpha.txt" with nothing behind it, and the binding layer refused. These fix
// what the resource must be for it to stop refusing — and, just as importantly, what still
// gets refused afterwards.

// shellItem builds an element as the bridge now reports one: a control that a source
// positively established the object behind.
func shellItem(id, caption, path, kind string) *directorapi.Element {
	el := obj(id, directorapi.RoleListItem, caption, "")
	el.Resource = &directorapi.ResourceIdentity{
		Kind: kind, Path: path, ParsingName: path, DisplayName: caption,
		Source: "shell_folder_view", Confidence: 1,
		Evidence: []string{"the shell reports this item as selected"},
	}
	return el
}

// ── what is now accepted ──────────────────────────────────────────────────────

// TestAShellResourceIdentifiesAFile is the case the whole milestone exists for.
func TestAShellResourceIdentifiesAFile(t *testing.T) {
	w := world(0, focused(shellItem("e1", "Alpha.txt",
		`C:\tmp\live-1\Alpha.txt`, directorapi.ResourceFile)))

	b, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem != nil {
		t.Fatalf("an Explorer item with a shell path did not bind: %s", problem.Message)
	}
	if b.Resolved != binding.KindFile {
		t.Errorf("resolved kind = %s, want file", b.Resolved)
	}
	if b.Resource != `C:\tmp\live-1\Alpha.txt` {
		t.Errorf("resource = %q, want the shell's path", b.Resource)
	}
	if !b.Decisive() {
		t.Error("the binding has no decisive evidence")
	}
	if b.Identity == nil || b.Identity.Source != "shell_folder_view" {
		t.Errorf("the binding does not record how the identity was obtained: %+v", b.Identity)
	}
	// And the shell's own account travels with it, so a reader can tell a path that came
	// from a source that knew from one inferred off an attribute.
	var sawShell bool
	for _, e := range b.Evidence {
		if strings.HasPrefix(e.Kind, "resource_") || e.Kind == "shell" {
			sawShell = true
		}
	}
	if !sawShell {
		t.Errorf("no shell evidence reached the binding: %+v", b.Evidence)
	}
}

// TestAHiddenExtensionDoesNotChangeTheIdentity.
//
// With "hide extensions for known file types" on, Explorer captions the item "Alpha". The
// path still ends in .txt, because the path comes from the shell and not from the caption
// — which is the entire point.
func TestAHiddenExtensionDoesNotChangeTheIdentity(t *testing.T) {
	w := world(0, focused(shellItem("e1", "Alpha",
		`C:\tmp\live-1\Alpha.txt`, directorapi.ResourceFile)))

	b, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem != nil {
		t.Fatalf("resolve: %s", problem.Message)
	}
	if b.Resource != `C:\tmp\live-1\Alpha.txt` {
		t.Errorf("resource = %q; a hidden extension must not reach the identity", b.Resource)
	}
	if b.Label != "Alpha" {
		t.Errorf("label = %q; the caption is still what is shown", b.Label)
	}
}

// TestSameCaptionInDifferentFoldersAreDifferentObjects.
func TestSameCaptionInDifferentFoldersAreDifferentObjects(t *testing.T) {
	first := shellItem("e1", "Alpha.txt", `C:\tmp\one\Alpha.txt`, directorapi.ResourceFile)
	second := shellItem("e2", "Alpha.txt", `C:\tmp\two\Alpha.txt`, directorapi.ResourceFile)

	a, problem := resolver().Resolve("this file", binding.KindFile, world(0, focused(first)))
	if problem != nil {
		t.Fatalf("resolve: %s", problem.Message)
	}
	b, problem := resolver().Resolve("this file", binding.KindFile, world(0, focused(second)))
	if problem != nil {
		t.Fatalf("resolve: %s", problem.Message)
	}
	if a.Resource == b.Resource {
		t.Fatal("two files with the same caption in different folders got the same identity")
	}
}

// TestAShellFolderIsAFolderNotAFile.
//
// The kind comes from the SHELL, not from the shape of the name: a folder called
// "Reports.txt" is a folder, and a caption-derived rule would call it a file.
func TestAShellFolderIsAFolderNotAFile(t *testing.T) {
	w := world(0, focused(shellItem("e1", "Reports.txt",
		`C:\tmp\live-1\Reports.txt`, directorapi.ResourceFolder)))

	if b, problem := resolver().Resolve("this file", binding.KindFile, w); problem == nil {
		t.Fatalf("a folder bound as a file: %+v", b)
	} else if problem.Reason != binding.ReasonWrongKind {
		t.Errorf("reason = %s, want wrong_kind", problem.Reason)
	}

	b, problem := resolver().Resolve("this folder", binding.KindFolder, w)
	if problem != nil {
		t.Fatalf("a folder did not bind as a folder: %s", problem.Message)
	}
	if b.Resource != `C:\tmp\live-1\Reports.txt` {
		t.Errorf("resource = %q", b.Resource)
	}
}

// TestAFileWithNoExtensionIsStillAFile — the mirror case.
func TestAFileWithNoExtensionIsStillAFile(t *testing.T) {
	w := world(0, focused(shellItem("e1", "Reports",
		`C:\tmp\live-1\Reports`, directorapi.ResourceFile)))

	b, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem != nil {
		t.Fatalf("an extensionless file did not bind: %s", problem.Message)
	}
	if b.Resolved != binding.KindFile {
		t.Errorf("resolved kind = %s, want file", b.Resolved)
	}
}

// TestAShellResourceIsFoundThroughTheSelectionWhenFocusIsElsewhere — the two fixes from
// the previous milestone and this one, together, which is the shape the live run needs.
func TestAShellResourceIsFoundThroughTheSelectionWhenFocusIsElsewhere(t *testing.T) {
	w := world(0,
		focused(obj("e1", directorapi.RoleButton, "Sort", "")),
		selected(shellItem("e2", "Alpha.txt", `C:\tmp\live-1\Alpha.txt`, directorapi.ResourceFile)),
	)

	b, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem != nil {
		t.Fatalf("a selected shell item was not reachable past a focused toolbar: %s",
			problem.Message)
	}
	if b.Resource != `C:\tmp\live-1\Alpha.txt` {
		t.Errorf("resource = %q", b.Resource)
	}
}

// ── what is still refused ─────────────────────────────────────────────────────

// TestACaptionOnlyItemIsStillRefused is the regression that keeps this milestone honest.
func TestACaptionOnlyItemIsStillRefused(t *testing.T) {
	// Exactly what Explorer looked like before the bridge learned to ask the shell.
	w := world(0, focused(obj("e1", directorapi.RoleListItem, "Alpha.txt", "")))

	b, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem == nil {
		t.Fatalf("a caption bound as a file: %+v", b)
	}
	if problem.Reason != binding.ReasonWrongKind {
		t.Errorf("reason = %s, want wrong_kind", problem.Reason)
	}
}

// TestAVirtualShellItemDoesNotMasqueradeAsAFile.
//
// The bridge reports no resource for an item with no file behind it, and an element with
// no resource is a control. A virtual item that arrived with a KIND and no path is refused
// by the same rule.
func TestAVirtualShellItemDoesNotMasqueradeAsAFile(t *testing.T) {
	el := obj("e1", directorapi.RoleListItem, "Control Panel", "")
	el.Resource = &directorapi.ResourceIdentity{
		Kind: directorapi.ResourceFile, DisplayName: "Control Panel",
		Source: "shell_folder_view",
		// No path. A shell object that is not on disk has none, and the absence is the
		// answer rather than something to fill in.
	}
	if b, problem := resolver().Resolve("this file", binding.KindFile,
		world(0, focused(el))); problem == nil {
		t.Fatalf("a virtual shell item bound as a file: %+v", b)
	}
}

// TestAnUncheckableResourceKindIsRefused — a bridge reporting a kind this build does not
// model must not be believed into a file.
func TestAnUncheckableResourceKindIsRefused(t *testing.T) {
	el := obj("e1", directorapi.RoleListItem, "Mailbox", "")
	el.Resource = &directorapi.ResourceIdentity{
		Kind: "mail_folder", Path: `mapi://inbox`, Source: "shell_folder_view",
	}
	if b, problem := resolver().Resolve("this file", binding.KindFile,
		world(0, focused(el))); problem == nil {
		t.Fatalf("an unmodelled resource kind bound as a file: %+v", b)
	}
}

// TestMultipleSelectedShellItemsAreAmbiguous.
func TestMultipleSelectedShellItemsAreAmbiguous(t *testing.T) {
	w := world(0,
		focused(obj("e1", directorapi.RoleButton, "Sort", "")),
		selected(shellItem("e2", "Alpha.txt", `C:\tmp\live-1\Alpha.txt`, directorapi.ResourceFile)),
		selected(shellItem("e3", "Bravo.txt", `C:\tmp\live-1\Bravo.txt`, directorapi.ResourceFile)),
	)

	b, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem == nil {
		t.Fatalf("two selected files bound to %+v", b)
	}
	if problem.Reason != binding.ReasonAmbiguous {
		t.Errorf("reason = %s, want ambiguous", problem.Reason)
	}
}

// TestADisappearedItemIsNotBound — an item the bridge no longer reports cannot be bound,
// and the world simply does not contain it.
func TestADisappearedItemIsNotBound(t *testing.T) {
	before := world(0, focused(shellItem("e1", "Alpha.txt",
		`C:\tmp\live-1\Alpha.txt`, directorapi.ResourceFile)))
	b, problem := resolver().Resolve("this file", binding.KindFile, before)
	if problem != nil {
		t.Fatalf("resolve: %s", problem.Message)
	}

	// The item is gone; something else holds the view.
	after := world(1, focused(obj("e9", directorapi.RoleButton, "Sort", "")))
	out := resolver().Revalidate(b, after)
	if out.OK {
		t.Fatal("a binding to an item that disappeared was revalidated")
	}
	if out.Problem.Reason != binding.ReasonGone {
		t.Errorf("reason = %s, want object_gone", out.Problem.Reason)
	}
}

// ── revalidation against the shell path ───────────────────────────────────────

// TestRevalidationReIdentifiesByTheShellPath.
func TestRevalidationReIdentifiesByTheShellPath(t *testing.T) {
	before := world(0,
		focused(shellItem("e1", "Alpha.txt", `C:\tmp\live-1\Alpha.txt`, directorapi.ResourceFile)),
		selected(shellItem("e2", "Bravo.txt", `C:\tmp\live-1\Bravo.txt`, directorapi.ResourceFile)),
	)
	b, problem := resolver().Resolve("this file", binding.KindFile, before)
	if problem != nil {
		t.Fatalf("resolve: %s", problem.Message)
	}

	// The tree was rebuilt: new element ids, same files, same selection.
	after := world(1,
		focused(shellItem("e7", "Alpha.txt", `C:\tmp\live-1\Alpha.txt`, directorapi.ResourceFile)),
	)
	out := resolver().Revalidate(b, after)
	if !out.OK {
		t.Fatalf("the same file was not re-identified: %s", out.Problem.Message)
	}
	if out.Binding.Resource != `C:\tmp\live-1\Alpha.txt` {
		t.Errorf("re-identified as %q", out.Binding.Resource)
	}
}

// TestRevalidationRefusesWhenFocusMovedToBravo — the case the live scenario's distractor
// exists for.
func TestRevalidationRefusesWhenFocusMovedToBravo(t *testing.T) {
	before := world(0, focused(shellItem("e1", "Alpha.txt",
		`C:\tmp\live-1\Alpha.txt`, directorapi.ResourceFile)))
	b, _ := resolver().Resolve("this file", binding.KindFile, before)

	after := world(1,
		shellItem("e1", "Alpha.txt", `C:\tmp\live-1\Alpha.txt`, directorapi.ResourceFile),
		focused(shellItem("e2", "Bravo.txt", `C:\tmp\live-1\Bravo.txt`, directorapi.ResourceFile)),
	)
	out := resolver().Revalidate(b, after)
	if out.OK {
		t.Fatal("the binding survived focus moving to the distractor")
	}
	if out.Problem.Reason != binding.ReasonChanged {
		t.Errorf("reason = %s, want object_changed", out.Problem.Reason)
	}
}

// TestRevalidationRefusesWhenTheWindowNavigatedElsewhere.
//
// A same-named file in the folder the view moved to is NOT the bound object. Re-identifying
// by path is what makes this a refusal rather than a silent substitution.
func TestRevalidationRefusesWhenTheWindowNavigatedElsewhere(t *testing.T) {
	before := world(0, focused(shellItem("e1", "Alpha.txt",
		`C:\tmp\live-1\Alpha.txt`, directorapi.ResourceFile)))
	b, _ := resolver().Resolve("this file", binding.KindFile, before)

	after := world(1, focused(shellItem("e1", "Alpha.txt",
		`C:\tmp\somewhere-else\Alpha.txt`, directorapi.ResourceFile)))
	out := resolver().Revalidate(b, after)
	if out.OK {
		t.Fatal("a same-named file in another folder satisfied the binding")
	}
}

// TestRevalidationRefusesWhenOnlyTheLabelStillMatches.
func TestRevalidationRefusesWhenOnlyTheLabelStillMatches(t *testing.T) {
	before := world(0, focused(shellItem("e1", "Alpha.txt",
		`C:\tmp\live-1\Alpha.txt`, directorapi.ResourceFile)))
	b, _ := resolver().Resolve("this file", binding.KindFile, before)

	// Same caption, no resource, different native id — a control that merely reads the
	// same.
	after := world(1, focused(obj("e5", directorapi.RoleListItem, "Alpha.txt", "")))
	out := resolver().Revalidate(b, after)
	if out.OK {
		t.Fatal("a caption that still matches was accepted as the same object")
	}
}

// ── serialisation ─────────────────────────────────────────────────────────────

// TestAShellBindingSerialisesWithoutCoordinatesOrHandles.
func TestAShellBindingSerialisesWithoutCoordinatesOrHandles(t *testing.T) {
	w := world(0, focused(shellItem("e1", "Alpha.txt",
		`C:\tmp\live-1\Alpha.txt`, directorapi.ResourceFile)))
	b, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem != nil {
		t.Fatalf("resolve: %s", problem.Message)
	}

	for _, form := range []any{b, b.Snapshot()} {
		raw, err := json.Marshal(form)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		lower := strings.ToLower(string(raw))
		for _, forbidden := range []string{`"x":`, `"y":`, "bounds", "click_point", "pidl", "comobject"} {
			if strings.Contains(lower, forbidden) {
				t.Errorf("%T contains %s: %s", form, forbidden, raw)
			}
		}
		// And the identity DID survive, so this is not passing by carrying nothing.
		if !strings.Contains(string(raw), `Alpha.txt`) {
			t.Errorf("%T lost the resource: %s", form, raw)
		}
	}
}

// TestTheSnapshotCarriesTheShellIdentity.
func TestTheSnapshotCarriesTheShellIdentity(t *testing.T) {
	w := world(0, focused(shellItem("e1", "Alpha.txt",
		`C:\tmp\live-1\Alpha.txt`, directorapi.ResourceFile)))
	b, _ := resolver().Resolve("this file", binding.KindFile, w)

	snap := b.Snapshot()
	if snap.Identity == nil {
		t.Fatal("the snapshot lost the account of how the identity was obtained")
	}
	if snap.Identity.Source != "shell_folder_view" {
		t.Errorf("source = %q", snap.Identity.Source)
	}
	if !snap.Identified() {
		t.Error("a snapshot with a shell path reports itself as unidentifiable")
	}

	// And it does not alias the live binding.
	b.Identity.Evidence = append(b.Identity.Evidence, "added later")
	if len(snap.Identity.Evidence) == len(b.Identity.Evidence) {
		t.Error("the snapshot's evidence changed when the binding's did")
	}
}
