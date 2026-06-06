package driver

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/compile"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// blockHost's action blocks until the call's context is canceled — standing in
// for a long-running OS effect (sleep, spam) that a panic-stop must interrupt.
type blockHost struct{}

func (blockHost) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	<-c.Ctx.Done()
	return "ok", runtime.Absent(), nil
}

func TestPanicStopAbortsRun(t *testing.T) {
	src := `the OS is an act.
this exports Wait.

the App is a script.

do OS's Wait...
    when ok?
        log "after".
    or?
        log "aborted".
`
	g, err := buildGraph(src, "", map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if err := compile.Compile(g, nil); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel() // simulate the Esc panic-stop
	}()

	var out bytes.Buffer
	if err := runtime.RunWithHostsContext(ctx, g, &out, map[string]runtime.Host{"*": blockHost{}}); err != nil {
		t.Fatalf("run: %v", err)
	}
	// The post-Wait line must not run — the route was aborted mid-call.
	if strings.Contains(out.String(), "after") {
		t.Fatalf("route was not aborted; output:\n%s", out.String())
	}
}

func TestNoCancelRunsToEnd(t *testing.T) {
	// Same program, but the host returns immediately and ctx is never canceled:
	// the run completes and logs "after".
	src := `the OS is an act.
this exports Wait.

the App is a script.

do OS's Wait...
    when ok?
        log "after".
    or?
        log "aborted".
`
	g, _ := buildGraph(src, "", map[string]bool{})
	if err := compile.Compile(g, nil); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	// dryrun host (nil → DryRunHost) returns ok immediately.
	if err := runtime.RunWithHosts(g, &out, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "after") {
		t.Fatalf("expected completion; output:\n%s", out.String())
	}
}
