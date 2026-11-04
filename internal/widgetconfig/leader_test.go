package widgetconfig

import (
	"os"
	"path/filepath"
	"testing"
)

// The widget's precedence: default, then the environment, then the file when it states one.
func TestTheLeaderIsReadTheWayTheWidgetReadsIt(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "overlay.json")
	t.Setenv("MARCO_OVERLAY_CONFIG", p)
	t.Setenv("MARCO_OVERLAY_LEADER", "")
	if got := Leader(); got != DefaultLeader {
		t.Errorf("no file, no environment: %q, want the default", got)
	}
	t.Setenv("MARCO_OVERLAY_LEADER", "tab")
	if got := Leader(); got != "tab" {
		t.Errorf("the environment was not used: %q", got)
	}
	if err := os.WriteFile(p, []byte(`{"theme":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Leader(); got != "tab" {
		t.Errorf("a file that states no leader overrode the environment: %q", got)
	}
	if err := os.WriteFile(p, []byte(`{"leader":"capslock","future":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Leader(); got != "capslock" {
		t.Errorf("the file's leader was not used: %q", got)
	}
	if err := os.WriteFile(p, []byte(`{broken`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Leader(); got != "tab" {
		t.Errorf("an unreadable file should leave the value before it: %q", got)
	}
}

func TestAShortcutReadsWithItsLeader(t *testing.T) {
	for _, c := range []struct{ leader, key, want string }{
		{"`", "e", "`e"},
		{"capslock", "e", "CapsLock then e"},
		{"f13", "c", "F13 then c"},
		{"tab", "c", "Tab then c"},
	} {
		if got := Pressed(c.leader, c.key); got != c.want {
			t.Errorf("Pressed(%q, %q) = %q, want %q", c.leader, c.key, got, c.want)
		}
	}
}
