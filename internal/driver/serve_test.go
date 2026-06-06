package driver

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// serveProg declares feed listeners that react to inbound hotkey/chat events.
const serveProg = `the Hotkeys is a feed of any.
the Chat is a feed of any.

when Hotkeys reads Leader?
    log "leader".

when Hotkeys reads Stop?
    log "stopped".

when Chat reads Message?
    log input.

the App is a script.
log "ready".
`

// TestServeDeliversEvents pipes JSON event lines into a served program and
// checks that each reaches its feed listener. Output order races with the
// script body and concurrent listeners, so compare as a set of lines.
func TestServeDeliversEvents(t *testing.T) {
	dir := t.TempDir()
	prog := writeTemp(t, dir, "serve.marco", serveProg)

	events := strings.Join([]string{
		`{"feed":"Hotkeys","event":"Leader"}`,
		`{"feed":"Chat","event":"Message","data":"hello"}`,
		`{"feed":"Hotkeys","event":"Stop"}`,
		`{"feed":"Unknown","event":"Ignored"}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	if err := ServeFile(prog, strings.NewReader(events), &out, nil); err != nil {
		t.Fatalf("serve: %v", err)
	}

	got := splitNonEmpty(out.String())
	want := []string{"hello", "leader", "ready", "stopped"}
	sort.Strings(got)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("served output = %v, want %v", got, want)
	}
}

func splitNonEmpty(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
