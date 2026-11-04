package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A DIRECTOR ALREADY RUNNING IS LEFT AS IT IS. It was not started for the widget, so the widget
// must neither start a second one nor tell the first whom it is for. DIRECTOR_BIN names nothing,
// so anything that tried to start one would fail loudly.
func TestStartingTheDirectorLeavesARunningOneAlone(t *testing.T) {
	serveStub(t, execute.Outcome{Status: directorapi.ResultDone})
	t.Setenv("DIRECTOR_BIN", filepath.Join(t.TempDir(), "no-such-director.exe"))

	var code int
	out, _ := capture(t, func() { code = directorStart(true, uint64(os.Getpid())) })
	if code != 0 {
		t.Fatalf("exit %d with a Director running: %s", code, out)
	}
	if !strings.Contains(out, `"started":false`) || !strings.Contains(out, `"running":true`) {
		t.Fatalf("a running Director was not reported as found and left alone: %s", out)
	}
}

// AND ONE IT STARTS IS TOLD WHOM IT IS FOR, which is the only thing that makes it stop when the
// widget has gone. Without --for, it is served exactly as a request would start it.
func TestADirectorStartedForTheWidgetIsToldWhomItIsFor(t *testing.T) {
	got := directorStartArgs(4242)
	if i := slices.Index(got, "--for"); i < 0 || i+1 >= len(got) || got[i+1] != "4242" {
		t.Fatalf("serve args %q do not name the process it is started for", got)
	}
	if got := directorStartArgs(0); !slices.Equal(got, []string{"serve", "--quiet"}) {
		t.Fatalf("with no owner, serve args are %q, not the ones a request starts it with", got)
	}
}
