package widgetconfig

import (
	"os"
	"path/filepath"
	"testing"
)

// THE SKILLS' SETTINGS ARE READ AND WRITTEN like every other: the search provider and whether a
// search waits for its read-back. Leaving either out of the file's fields must fail this.
func TestTheSkillsSettingsAreReadAndWritten(t *testing.T) {
	p := filepath.Join(t.TempDir(), "overlay.json")
	t.Setenv("MARCO_OVERLAY_CONFIG", p)
	if err := os.WriteFile(p, []byte(`{"search":"bing","confirmSearch":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Load()
	if s.Search != "bing" || !s.ConfirmSearch {
		t.Fatalf("read %q %v", s.Search, s.ConfirmSearch)
	}
	s.ConfirmSearch = false
	if err := Write(s, "confirmSearch"); err != nil || Load().ConfirmSearch {
		t.Errorf("confirmSearch was not written: %v", err)
	}
	if !Known("search") || !Known("confirmSearch") {
		t.Error("the control centre cannot save them")
	}
}
