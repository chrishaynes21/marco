package verify_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/verify"
)

// Verification must belong to the bound object.
//
//	Do not accept only: a matching label, a destination file existing somewhere, the
//	focused item now displaying the requested text, or a generic "changed"
//	observation.
//
// The fixtures below are a controlled filesystem — no real files are touched, which is
// what makes it possible to test the mistakes as well as the success.

// fakeFS answers Inspect from a map. A path absent from the map does not exist; a path in
// unreadable cannot be answered at all, which is a different thing.
type fakeFS struct {
	files      map[string]string // path → content digest
	unreadable map[string]bool
}

func fs(files map[string]string) *fakeFS {
	return &fakeFS{files: files, unreadable: map[string]bool{}}
}

func (f *fakeFS) Inspect(path string) (verify.Identity, bool) {
	if f.unreadable[path] {
		return verify.Identity{}, false
	}
	digest, ok := f.files[path]
	return verify.Identity{Resource: path, Exists: ok, ContentDigest: digest}, true
}

const (
	targetPath = `C:\tmp\Report.txt`
	renamed    = `C:\tmp\Budget.txt`
	decoyPath  = `C:\tmp\Report2.txt`
	thirdFile  = `C:\tmp\Notes.txt`
)

func boundFile(path, digest string) verify.Identity {
	return verify.Identity{Resource: path, ContentDigest: digest, Exists: true,
		Label: "Report.txt", NativeID: "uia:2"}
}

func renameOf(digest string) verify.RenameCorrelation {
	return verify.RenameCorrelation{
		Bound: boundFile(targetPath, digest), RequestedName: "Budget.txt",
		Distractors: []string{decoyPath, thirdFile},
		Origin: verify.Origin{
			Goal: "rename", Procedure: "explorer rename", StepIndex: 4,
			StepID: "s4", ActionNode: "n12",
		},
	}
}

// ── the intended result ───────────────────────────────────────────────────────

// TestTheIntendedResourcePasses.
func TestTheIntendedResourcePasses(t *testing.T) {
	after := fs(map[string]string{
		renamed: "sha:aaa", decoyPath: "sha:bbb", thirdFile: "sha:ccc",
	})

	c := verify.CorrelateRename(renameOf("sha:aaa"), after)

	if c.Result != verify.Correlated {
		t.Fatalf("result = %s (%s); the intended file became the requested name and "+
			"nothing else moved", c.Result, c.Reason)
	}
	if c.Method != verify.MethodResource {
		t.Errorf("method = %s, want resource identity", c.Method)
	}
	if c.Expected.Resource != renamed {
		t.Errorf("expected resource = %q, want %q", c.Expected.Resource, renamed)
	}
	if c.Confidence != 1 {
		t.Errorf("confidence = %v, want 1", c.Confidence)
	}
	if len(c.Mismatching) != 0 {
		t.Errorf("a correlated rename reported mismatches: %v", c.Mismatching)
	}
}

// TestAContentPreservingRenamePasses.
func TestAContentPreservingRenamePasses(t *testing.T) {
	after := fs(map[string]string{
		renamed: "sha:aaa", decoyPath: "sha:bbb", thirdFile: "sha:ccc",
	})
	req := renameOf("sha:aaa")
	req.RequireContent = true

	c := verify.CorrelateRename(req, after)

	if c.Result != verify.Correlated {
		t.Fatalf("result = %s (%s)", c.Result, c.Reason)
	}
	if c.Method != verify.MethodContent {
		t.Errorf("method = %s, want content identity", c.Method)
	}
}

// ── the mistakes ──────────────────────────────────────────────────────────────

// TestRenamingTheDistractorFails is the failure the whole correlation exists to catch:
// the requested name appeared, and it appeared on the wrong file.
func TestRenamingTheDistractorFails(t *testing.T) {
	after := fs(map[string]string{
		// Report2.txt became Budget.txt. Report.txt was never touched.
		renamed: "sha:bbb", targetPath: "sha:aaa", thirdFile: "sha:ccc",
	})

	c := verify.CorrelateRename(renameOf("sha:aaa"), after)

	if c.Result == verify.Correlated {
		t.Fatal("a rename of the item NEXT TO the intended one was accepted; the " +
			"destination exists and that is exactly why it is not enough")
	}
	joined := strings.Join(c.Mismatching, " | ")
	if !strings.Contains(joined, targetPath) {
		t.Errorf("the mismatch does not say the original is still there: %s", joined)
	}
	if !strings.Contains(joined, decoyPath) {
		t.Errorf("the mismatch does not say which thirdFile item moved: %s", joined)
	}
	// And it says where to look.
	if !strings.Contains(c.Describe(), "explorer rename") {
		t.Errorf("the account does not name the procedure that produced it: %s", c.Describe())
	}
}

// TestDestinationOnlyEvidenceFails — a file with the right name existing somewhere is not
// evidence that anything was renamed.
func TestDestinationOnlyEvidenceFails(t *testing.T) {
	after := fs(map[string]string{
		// Budget.txt was already there; nothing happened at all.
		renamed: "sha:zzz", targetPath: "sha:aaa", decoyPath: "sha:bbb", thirdFile: "sha:ccc",
	})

	c := verify.CorrelateRename(renameOf("sha:aaa"), after)

	if c.Result == verify.Correlated {
		t.Fatal("the mere existence of a file with the requested name was accepted as " +
			"a successful rename")
	}
	if c.Confidence >= 1 {
		t.Errorf("confidence = %v on evidence that establishes nothing", c.Confidence)
	}
}

// TestLabelOnlyEvidenceIsInconclusive — an object with no file behind it cannot be
// correlated at all, and saying so is different from saying the rename failed.
func TestLabelOnlyEvidenceIsInconclusive(t *testing.T) {
	req := verify.RenameCorrelation{
		Bound:         verify.Identity{Label: "Report.txt", Exists: true},
		RequestedName: "Budget.txt",
	}

	c := verify.CorrelateRename(req, fs(map[string]string{renamed: "sha:aaa"}))

	if c.Result != verify.CorrelationInconclusive {
		t.Fatalf("result = %s; a caption is not identity, and treating its absence as "+
			"failure is as wrong as treating its presence as success", c.Result)
	}
	if c.Method != verify.MethodNone {
		t.Errorf("method = %s, want none", c.Method)
	}
	if !strings.Contains(c.Reason, "label") {
		t.Errorf("the reason does not explain what was missing: %s", c.Reason)
	}
}

// TestAlteredContentFailsWhenContentCorrelationIsRequired — a replacement with the right
// name is not the object that was bound.
func TestAlteredContentFailsWhenContentCorrelationIsRequired(t *testing.T) {
	after := fs(map[string]string{
		renamed: "sha:DIFFERENT", decoyPath: "sha:bbb", thirdFile: "sha:ccc",
	})
	req := renameOf("sha:aaa")
	req.RequireContent = true

	c := verify.CorrelateRename(req, after)

	if c.Result == verify.Correlated {
		t.Fatal("a different object carrying the requested name was accepted")
	}
	if !strings.Contains(strings.Join(c.Mismatching, " | "), "content differs") {
		t.Errorf("the mismatch does not name the content: %v", c.Mismatching)
	}
}

// TestAnUnreadableDistractorRefusesRatherThanAssumes.
func TestAnUnreadableDistractorRefusesRatherThanAssumes(t *testing.T) {
	after := fs(map[string]string{renamed: "sha:aaa", decoyPath: "sha:bbb", thirdFile: "sha:ccc"})
	after.unreadable[thirdFile] = true

	c := verify.CorrelateRename(renameOf("sha:aaa"), after)

	if c.Result == verify.Correlated {
		t.Fatal("a decoyPath that could not be read was assumed untouched")
	}
}

// TestNoInspectorIsInconclusiveRatherThanAPass.
func TestNoInspectorIsInconclusiveRatherThanAPass(t *testing.T) {
	c := verify.CorrelateRename(renameOf("sha:aaa"), nil)
	if c.Result != verify.CorrelationInconclusive {
		t.Fatalf("result = %s; with nothing able to read the file, nothing is established",
			c.Result)
	}
	if c.Result.OK() {
		t.Fatal("an inconclusive correlation reported itself as verified")
	}
}

// TestAMismatchNamesWhereItCameFrom.
//
//	A verification mismatch must identify the originating goal, procedure, step, and
//	action node when available.
func TestAMismatchNamesWhereItCameFrom(t *testing.T) {
	after := fs(map[string]string{targetPath: "sha:aaa", decoyPath: "sha:bbb", thirdFile: "sha:ccc"})

	c := verify.CorrelateRename(renameOf("sha:aaa"), after)
	if c.Result.OK() {
		t.Fatal("a rename that did not happen was accepted")
	}
	where := c.Origin.Describe()
	for _, want := range []string{"rename", "explorer rename", "step 4", "n12"} {
		if !strings.Contains(where, want) {
			t.Errorf("the origin does not mention %q: %s", want, where)
		}
	}
}
