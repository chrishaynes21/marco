package driver

import (
	"bytes"
	"errors"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/compile"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// stubHost returns a fixed result for every foreign invocation. Used to drive
// the failed / error resolution paths that the dryrun host can't produce.
type stubHost struct {
	status string
	data   runtime.Value
	err    error
}

func (h stubHost) Invoke(runtime.HostCall) (string, runtime.Value, error) {
	return h.status, h.data, h.err
}

func runWithHost(t *testing.T, src string, hosts map[string]runtime.Host) string {
	t.Helper()
	g, err := buildGraph(src, "", map[string]bool{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := compile.Compile(g, nil); err != nil {
		t.Fatalf("compile: %v", err)
	}
	var buf bytes.Buffer
	if err := runtime.RunWithHosts(g, &buf, hosts); err != nil {
		t.Fatalf("run: %v", err)
	}
	return buf.String()
}

const foreignBoomProg = `the OS is an act.
this exports Boom.

the App is a script.

do OS's Boom...
    when ok?
        log "ok".
    or?
        log that's error.
`

// A host returning status "failed" with an error payload resolves the calling
// frame as failed, and the error flows to `that's error` in the `or?` arm.
func TestForeignFailedWithError(t *testing.T) {
	host := stubHost{status: "failed", data: runtime.ErrVal(&runtime.Err{Message: "kaboom"})}
	got := runWithHost(t, foreignBoomProg, map[string]runtime.Host{"OS": host})
	if got != "[INFO] kaboom\n" {
		t.Fatalf("got %q, want %q", got, "[INFO] kaboom\n")
	}
}

// A host returning a non-nil Go error resolves the frame as failed with the
// error's message as `that's error`.
func TestForeignGoError(t *testing.T) {
	host := stubHost{err: errors.New("explode")}
	got := runWithHost(t, foreignBoomProg, map[string]runtime.Host{"OS": host})
	if got != "[INFO] explode\n" {
		t.Fatalf("got %q, want %q", got, "[INFO] explode\n")
	}
}

// A host returning "ok" with a value resolves the frame ok; the value is
// available as the frame's result via `that's <field>`-style access. Here we
// simply confirm the ok arm fires.
func TestForeignOK(t *testing.T) {
	host := stubHost{status: "ok", data: runtime.Absent()}
	got := runWithHost(t, foreignBoomProg, map[string]runtime.Host{"OS": host})
	if got != "[INFO] ok\n" {
		t.Fatalf("got %q, want %q", got, "[INFO] ok\n")
	}
}

// With no host registered for the act, foreign calls fall back to the dryrun
// host (logs the call, resolves ok).
func TestForeignDryrunFallback(t *testing.T) {
	got := runWithHost(t, foreignBoomProg, nil)
	if got != "[dryrun] OS's Boom\n[INFO] ok\n" {
		t.Fatalf("got %q, want %q", got, "[dryrun] OS's Boom\n[INFO] ok\n")
	}
}
