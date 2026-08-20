package uiaclient

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Decoding the object behind a control.
//
//	Do not make every control pretend to have a resource.
//	The field must be omitted when no trustworthy resource exists.
//
// This is the boundary where the bridge's account becomes the Director's. Everything the
// binding layer trusts about "which file is this?" arrives here, so the interesting cases
// are the ones where the bridge said something not quite good enough — and every one of
// them must come out as NOTHING rather than as a half-identity.

// bridgeSnapshot builds a host replaying a hand-written bridge reply.
func bridgeSnapshot(t *testing.T, elements string) directorapi.AccessibilitySnapshot {
	t.Helper()
	raw := `{"WindowId":"hwnd:1","WindowTitle":"live-1","App":"explorer","ProcessId":7,
	         "WindowX":0,"WindowY":0,"WindowW":800,"WindowH":600,
	         "Minimized":false,"Maximized":false,"WindowVisible":true,
	         "Partial":false,"Reason":"","ElapsedMs":5,"Elements":[` + elements + `]}`

	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("the fixture is not valid JSON: %v", err)
	}
	host := &fixtureHost{raw: []byte(raw)}
	snap, err := provider(host).Snapshot(context.Background(), "")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return snap
}

// item is one bridge element, optionally carrying a Resource.
func item(label, resource string) string {
	base := `{"Id":"uia:2","ParentId":"uia:1","Role":"list_item","ControlType":"ListItem",
	          "Label":"` + label + `","Value":"","Description":"",
	          "X":10,"Y":10,"W":200,"H":24,"Enabled":true,"Visible":true,
	          "Focused":false,"Selected":true,"Offscreen":false,
	          "AutomationId":"","ClassName":"","Depth":2,
	          "Patterns":"selectionitem","Expanded":"","Checked":""`
	if resource != "" {
		base += `,"Resource":` + resource
	}
	return base + "}"
}

// resource builds a bridge Resource block.
func resource(kind, path string) string {
	return `{"Kind":"` + kind + `","Path":"` + strings.ReplaceAll(path, `\`, `\\`) + `",
	         "ParsingName":"` + strings.ReplaceAll(path, `\`, `\\`) + `",
	         "DisplayName":"Alpha.txt","Source":"shell_folder_view",
	         "Confidence":1,"Link":false,
	         "Evidence":"the shell reports this item as selected | it is a file on disk"}`
}

func only(t *testing.T, snap directorapi.AccessibilitySnapshot, label string) directorapi.Observation {
	t.Helper()
	got := find(snap, label)
	if len(got) != 1 {
		t.Fatalf("found %d observations labelled %q, want one", len(got), label)
	}
	return got[0]
}

// ── what is carried ───────────────────────────────────────────────────────────

// TestAShellResourceReachesTheObservation.
func TestAShellResourceReachesTheObservation(t *testing.T) {
	snap := bridgeSnapshot(t, item("Alpha.txt",
		resource(directorapi.ResourceFile, `C:\tmp\live-1\Alpha.txt`)))

	o := only(t, snap, "Alpha.txt")
	if !o.Resource.Known() {
		t.Fatalf("the resource did not survive decoding: %+v", o.Resource)
	}
	if o.Resource.Path != `C:\tmp\live-1\Alpha.txt` {
		t.Errorf("path = %q", o.Resource.Path)
	}
	if o.Resource.Kind != directorapi.ResourceFile {
		t.Errorf("kind = %q, want file", o.Resource.Kind)
	}
	if o.Resource.Source != "shell_folder_view" {
		t.Errorf("source = %q", o.Resource.Source)
	}
	// The evidence arrives pipe-separated and becomes a list, so a reader sees the
	// clauses rather than one run-on sentence.
	if len(o.Resource.Evidence) != 2 {
		t.Errorf("evidence = %v, want two clauses", o.Resource.Evidence)
	}
}

// TestAFolderResourceIsAFolder.
func TestAFolderResourceIsAFolder(t *testing.T) {
	snap := bridgeSnapshot(t, item("Reports",
		resource(directorapi.ResourceFolder, `C:\tmp\live-1\Reports`)))

	o := only(t, snap, "Reports")
	if !o.Resource.IsFolder() {
		t.Fatalf("a folder decoded as %+v", o.Resource)
	}
	if o.Resource.IsFile() {
		t.Error("a folder also reports itself as a file")
	}
}

// ── what is refused ───────────────────────────────────────────────────────────

// TestAControlWithNoResourceGetsNone — the ordinary case, and the one that must not
// acquire a fiction.
func TestAControlWithNoResourceGetsNone(t *testing.T) {
	snap := bridgeSnapshot(t, item("Alpha.txt", ""))

	o := only(t, snap, "Alpha.txt")
	if o.Resource != nil {
		t.Fatalf("a control the bridge said nothing about acquired %+v", o.Resource)
	}
	if o.Resource.Known() {
		t.Error("a nil resource reports itself as known")
	}
}

// TestAResourceWithNoPathIsRefused — a virtual shell object.
func TestAResourceWithNoPathIsRefused(t *testing.T) {
	snap := bridgeSnapshot(t, item("Control Panel",
		`{"Kind":"file","Path":"","ParsingName":"::{26EE0668}","DisplayName":"Control Panel",
		  "Source":"shell_folder_view","Confidence":1,"Link":false,"Evidence":""}`))

	o := only(t, snap, "Control Panel")
	if o.Resource != nil {
		t.Fatalf("a shell object with no path became %+v", o.Resource)
	}
}

// TestAnUnmodelledResourceKindIsRefused.
//
// A bridge reporting a kind this build cannot check must not have it passed through: the
// binding layer would have to decide what to do with it, and deciding is guessing.
func TestAnUnmodelledResourceKindIsRefused(t *testing.T) {
	snap := bridgeSnapshot(t, item("Inbox",
		`{"Kind":"mail_folder","Path":"mapi://inbox","ParsingName":"mapi://inbox",
		  "DisplayName":"Inbox","Source":"shell_folder_view","Confidence":1,
		  "Link":false,"Evidence":""}`))

	o := only(t, snap, "Inbox")
	if o.Resource != nil {
		t.Fatalf("an unmodelled kind became %+v", o.Resource)
	}
}

// TestAnOlderBridgeStillDecodes.
//
//	Older serialized observations without resource identity must remain valid.
//
// A reply with no Resource field at all — every bridge before this milestone — must decode
// exactly as it did, with the resource absent rather than zero.
func TestAnOlderBridgeStillDecodes(t *testing.T) {
	snap := bridgeSnapshot(t, item("Save", ""))

	o := only(t, snap, "Save")
	if o.Resource != nil {
		t.Fatal("an old bridge's element acquired a resource")
	}
	if o.Label != "Save" || o.Role == "" {
		t.Errorf("the rest of the element did not survive: %+v", o)
	}
}

// TestADefaultedSourceAndConfidenceAreFilledHonestly.
//
// A bridge that reports a path and a kind but no provenance still produces a usable
// resource — the identity is what matters — and this layer says where it came from rather
// than leaving the field blank for a reader to interpret.
func TestADefaultedSourceAndConfidenceAreFilledHonestly(t *testing.T) {
	snap := bridgeSnapshot(t, item("Alpha.txt",
		`{"Kind":"file","Path":"C:\\tmp\\live-1\\Alpha.txt","ParsingName":"",
		  "DisplayName":"","Source":"","Confidence":0,"Link":false,"Evidence":""}`))

	o := only(t, snap, "Alpha.txt")
	if !o.Resource.Known() {
		t.Fatal("a path and a kind were not enough")
	}
	if o.Resource.Source == "" {
		t.Error("the resource does not say where it came from")
	}
	if o.Resource.Confidence == 0 {
		t.Error("the resource reports zero confidence in an identity it is asserting")
	}
}

// TestTheDecodedResourceCarriesNoCoordinates.
func TestTheDecodedResourceCarriesNoCoordinates(t *testing.T) {
	snap := bridgeSnapshot(t, item("Alpha.txt",
		resource(directorapi.ResourceFile, `C:\tmp\live-1\Alpha.txt`)))

	raw, err := json.Marshal(only(t, snap, "Alpha.txt").Resource)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{`"x"`, `"y"`, "bounds", "hwnd", "pidl"} {
		if strings.Contains(strings.ToLower(string(raw)), forbidden) {
			t.Errorf("the decoded resource contains %s: %s", forbidden, raw)
		}
	}
}

var _ runtime.Host = (*fixtureHost)(nil)
