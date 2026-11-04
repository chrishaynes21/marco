package main

// THE CONTROL CENTRE'S CONFIG VIEW, through the real handler, over a settings file of the test's
// own. S5: it shows what the widget runs with and writes only what it was sent.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func widgetSettingsFile(t *testing.T, contents string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "overlay.json")
	if contents != "" {
		if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("MARCO_OVERLAY_CONFIG", p)
	t.Setenv("MARCO_OVERLAY_LEADER", "")
	return p
}

func postSettings(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	(&editor{}).handleOConfig(w, httptest.NewRequest(http.MethodPost, "/api/oconfig", strings.NewReader(body)))
	return w
}

func settingsOnDisk(t *testing.T, p string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	return m
}

// A save of the activation phrase writes the activation phrase. The leader, which the widget took
// from its environment, is not written into the file (where it would then win), and a field no
// surface knows survives.
//
// Writing every setting must fail this.
func TestTheControlCentreSavesOnlyWhatWasSent(t *testing.T) {
	p := widgetSettingsFile(t, `{"theme":"nord","futureThing":[1,2]}`)
	t.Setenv("MARCO_OVERLAY_LEADER", "capslock")
	if w := postSettings(t, `{"wake":"  jarvis "}`); w.Code != http.StatusOK {
		t.Fatalf("POST = %d: %s", w.Code, w.Body.String())
	}
	m := settingsOnDisk(t, p)
	if m["wake"] != "jarvis" || m["theme"] != "nord" {
		t.Errorf("after saving the activation phrase the file is %v", m)
	}
	if _, wrote := m["leader"]; wrote {
		t.Errorf("a save of the activation phrase wrote a leader: %v", m)
	}
	if ft, ok := m["futureThing"].([]any); !ok || len(ft) != 2 {
		t.Errorf("a field no surface knows did not survive: %v", m)
	}
}

// The view shows the leader the widget listens for, from its environment when the file says
// nothing, and the file's when it does: the widget's precedence.
func TestTheControlCentreShowsTheRealLeader(t *testing.T) {
	for _, c := range []struct{ file, env, want string }{
		{``, "capslock", "capslock"},
		{`{"leader":"tab"}`, "capslock", "tab"},
		{`{"leader":7}`, "capslock", "capslock"}, // a leader of the wrong type keeps the one before it
		{``, "", "`"},
	} {
		widgetSettingsFile(t, c.file)
		t.Setenv("MARCO_OVERLAY_LEADER", c.env)
		w := httptest.NewRecorder()
		(&editor{}).handleOConfig(w, httptest.NewRequest(http.MethodGet, "/api/oconfig", nil))
		var got struct {
			Config struct {
				Leader string `json:"leader"`
			} `json:"config"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Config.Leader != c.want {
			t.Errorf("file %q, env %q: shown %q, want %q", c.file, c.env, got.Config.Leader, c.want)
		}
	}
}

// What cannot be saved is not, and says why: a setting nobody knows, a value of the wrong type,
// and a file that cannot be read (which is left exactly as it was).
func TestTheControlCentreRefusesWhatItCannotSave(t *testing.T) {
	p := widgetSettingsFile(t, `{"wake":"marco"}`)
	for _, body := range []string{`{"nonsense":1}`, `{"width":"wide"}`, `not json`} {
		if w := postSettings(t, body); w.Code != http.StatusBadRequest {
			t.Errorf("POST %s = %d: %s", body, w.Code, w.Body.String())
		}
	}
	if m := settingsOnDisk(t, p); len(m) != 1 || m["wake"] != "marco" {
		t.Errorf("a refused save changed the file: %v", m)
	}
	const broken = `{"wake":`
	if err := os.WriteFile(p, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	w := postSettings(t, `{"theme":"nord"}`)
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "not a settings file Marco can read") {
		t.Errorf("a save over an unreadable file = %d: %s", w.Code, w.Body.String())
	}
	if b, _ := os.ReadFile(p); string(b) != broken {
		t.Errorf("the unreadable file was replaced with %s", b)
	}
}

// The activation phrase the launcher reads is the shared reader's.
func TestTheActivationPhraseIsReadThroughTheOneOwner(t *testing.T) {
	widgetSettingsFile(t, `{"wake":" jarvis "}`)
	said, err := captureStdout(t, func() error { runWake(); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(said) != "jarvis" {
		t.Errorf("runWake said %q", said)
	}
}
