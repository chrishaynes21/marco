package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AN EMPTY THEATER CAN BE ASKED ABOUT BEFORE A PLAY FAILS.
//
// # The silence this ends
//
// `no_actor_available` is the honest refusal for a machine with nothing that can act, and it is
// only reachable by RUNNING a play. So the first time anybody discovered the Theater was empty
// was when a play they had just taught, verified and saved did nothing at all — and the refusal
// arrived as a `failed` status inside a play, which reads as a broken play rather than as a
// machine that cannot act.
//
// `marco director diagnose` is where a person goes when nothing happened. This is the report it
// now carries: the provider path that was found (or was not), and the roster.
//
// Mutations this kills: dropping the roster from theaterDiagnostics; reporting a bridge path
// without asking whether anything can actually act on it; printing the same text whether or not
// a provider was found, which is the exact ambiguity the report exists to remove.
func TestDiagnoseSaysWhetherAnythingCanAct(t *testing.T) {
	t.Setenv("MARCO_UIA_BRIDGE", "")
	inTempCwd(t, t.TempDir())

	// A machine with no provider. Not a hypothetical: it is every fresh install that
	// shipped without one, which is what this round is fixing.
	empty := theaterDiagnostics()
	if empty.Bridge != "" {
		t.Fatalf("with no provider anywhere, diagnose reported a bridge at %q", empty.Bridge)
	}
	if len(empty.Roster) != 0 {
		t.Errorf("with no provider, the roster is %+v; the Theater has nobody in it and "+
			"the report should say so", empty.Roster)
	}
	out := render(printTheater, empty)
	if !strings.Contains(out, "NOT FOUND") {
		t.Errorf("diagnose does not say the provider is missing:\n%s", out)
	}

	// The same machine once a provider exists. The report must CHANGE — a diagnostic that
	// reads the same either way answers nothing.
	bridge := filepath.Join(t.TempDir(), "uia.exe")
	if err := os.WriteFile(bridge, nil, 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	t.Setenv("MARCO_UIA_BRIDGE", bridge)

	got := theaterDiagnostics()
	if got.Bridge != bridge {
		t.Errorf("diagnose reported the bridge as %q, want %q", got.Bridge, bridge)
	}
	if len(got.Roster) != 1 || got.Roster[0].Name != "accessibility" || !got.Roster[0].Available {
		t.Fatalf("with a provider wired, the roster is %+v; want one ready accessibility "+
			"actor. The roster reads the same Available() predicate casting reads, so a "+
			"roster that disagrees here is a Theater that will refuse a play.", got.Roster)
	}
	out = render(printTheater, got)
	if strings.Contains(out, "NOT FOUND") || !strings.Contains(out, bridge) {
		t.Errorf("diagnose does not name the provider it found:\n%s", out)
	}
	if !strings.Contains(out, "ready") {
		t.Errorf("diagnose does not say the actor can act:\n%s", out)
	}
}

// render captures what a printer wrote to stdout.
func render(print func(theaterReport), r theaterReport) string {
	old := os.Stdout
	pr, pw, err := os.Pipe()
	if err != nil {
		return "render: " + err.Error()
	}
	os.Stdout = pw
	done := make(chan string, 1)
	go func() {
		b := make([]byte, 0, 4096)
		chunk := make([]byte, 1024)
		for {
			n, err := pr.Read(chunk)
			b = append(b, chunk[:n]...)
			if err != nil {
				break
			}
		}
		done <- string(b)
	}()
	print(r)
	_ = pw.Close()
	os.Stdout = old
	return <-done
}
