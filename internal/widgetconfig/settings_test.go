package widgetconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func settingsAt(t *testing.T, contents string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "overlay.json")
	if contents != "" {
		if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("MARCO_OVERLAY_CONFIG", p)
	for _, k := range []string{"THEME", "IDLE", "MONITOR", "SIZE", "LEADER", "FONT", "MAXLINES"} {
		t.Setenv("MARCO_OVERLAY_"+k, "")
	}
	t.Setenv("MARCO_VOICE", "")
	return p
}

func onDisk(t *testing.T, p string) map[string]json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	return m
}

// Defaults, then the environment, then the file, key by key: one key of the wrong type keeps the
// value before it and does not spoil the others.
func TestTheSettingsAreReadTheWayTheWidgetReadsThem(t *testing.T) {
	settingsAt(t, `{"theme":"nord","width":"wide","leader":"tab","voice":false,"future":{"x":1}}`)
	t.Setenv("MARCO_OVERLAY_SIZE", "500x300")
	t.Setenv("MARCO_OVERLAY_LEADER", "capslock")
	s := Load()
	if s.Theme != "nord" || s.Width != 500 || s.Height != 300 || s.Leader != "tab" || s.Voice ||
		s.Wake != "marco" || s.MaxLines != 5 {
		t.Errorf("read as %+v", s)
	}
	if Leader() != s.Leader {
		t.Errorf("Leader() %q is not the settings' %q", Leader(), s.Leader)
	}
}

// Write changes the named keys and nothing else: not the other known ones, not a field nobody
// here knows, and not their values as written.
//
// Writing the whole struct back must fail this.
func TestASaveKeepsWhatItDidNotChange(t *testing.T) {
	p := settingsAt(t, `{"wake":"jarvis","leader":"tab","future":{"x":[1,"two"]}}`)
	s := Load()
	s.Theme, s.Leader = "dracula", "f8" // the leader changed in memory, and is not named
	if err := Write(s, "theme"); err != nil {
		t.Fatal(err)
	}
	m := onDisk(t, p)
	if string(m["theme"]) != `"dracula"` || string(m["leader"]) != `"tab"` || string(m["wake"]) != `"jarvis"` {
		t.Errorf("after writing the theme the file is %s", fmt.Sprint(m))
	}
	var future map[string][]any
	if err := json.Unmarshal(m["future"], &future); err != nil || len(future["x"]) != 2 || future["x"][1] != "two" {
		t.Errorf("a field nobody knows came back as %s", m["future"])
	}
	if len(m) != 4 {
		t.Errorf("the file holds %d fields, want 4: %v", len(m), m)
	}
}

// A file that is not there is made; one that cannot be read is not overwritten; an unknown key
// writes nothing at all.
func TestWhatCannotBeWrittenIsNot(t *testing.T) {
	p := settingsAt(t, "")
	if err := Write(Defaults(), "leader"); err != nil {
		t.Fatal(err)
	}
	if m := onDisk(t, p); len(m) != 1 || string(m["leader"]) != "\"`\"" {
		t.Errorf("a first save wrote %v", m)
	}
	if err := Write(Defaults(), "theme", "nonsense"); err == nil {
		t.Error("an unknown setting was written")
	}
	if m := onDisk(t, p); len(m) != 1 {
		t.Errorf("a refused write changed the file: %v", m)
	}
	for _, broken := range []string{`{"wake":`, `[1,2]`, `"text"`} {
		if err := os.WriteFile(p, []byte(broken), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Write(Defaults(), "theme"); err == nil {
			t.Errorf("a write over %s succeeded", broken)
		}
		if b, _ := os.ReadFile(p); string(b) != broken {
			t.Errorf("%s was replaced with %s", broken, b)
		}
	}
}

// Two surfaces saving different settings at the same moment both land: the read-modify-write is
// one claim on the file, so neither writes back a copy read before the other's write.
//
// Dropping the lock must fail this (it does not every time; it is run many times over).
func TestTwoSavesAtOnceBothLand(t *testing.T) {
	p := settingsAt(t, `{"future":1}`)
	keys := []string{"theme", "corner", "font", "wake"}
	for round := 0; round < 20; round++ {
		var wg sync.WaitGroup
		for i, k := range keys {
			wg.Add(1)
			go func(i int, k string) {
				defer wg.Done()
				var s Settings
				*(s.fields()[k].(*string)) = fmt.Sprintf("r%d-%d", round, i)
				if err := Write(s, k); err != nil {
					t.Error(err)
				}
			}(i, k)
		}
		wg.Wait()
		m := onDisk(t, p)
		for i, k := range keys {
			if want := fmt.Sprintf(`"r%d-%d"`, round, i); string(m[k]) != want {
				t.Fatalf("round %d: %s is %s, want %s: a save was lost", round, k, m[k], want)
			}
		}
		if string(m["future"]) != "1" {
			t.Fatalf("round %d: the unknown field is %s", round, m["future"])
		}
	}
}
