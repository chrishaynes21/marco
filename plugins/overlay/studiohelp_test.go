package main

// THE WIDGET'S `help` OPENS STUDIO'S HELP. See [[ADR-317-the-widgets-help-opens-studio]].

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// spawnsOf captures what the next spawns are asked to run, binary included, and starts nothing.
func spawnsOf(t *testing.T) func() []string {
	t.Helper()
	got := make(chan []string, 4)
	prev := startUIChild
	startUIChild = func(cmd *exec.Cmd) error {
		got <- cmd.Args
		return errors.New("not started: this test never spawns anything")
	}
	t.Cleanup(func() { startUIChild = prev })
	return func() []string {
		t.Helper()
		select {
		case a := <-got:
			return a
		case <-time.After(5 * time.Second):
			t.Fatal("nothing was started")
			return nil
		}
	}
}

func withStudio(t *testing.T, path string) {
	t.Helper()
	prev := studioBin
	studioBin = func() string { return path }
	t.Cleanup(func() { studioBin = prev })
}

func TestTheHelpWordOpensStudioOnItsHelp(t *testing.T) {
	noMarco(t)
	studio := filepath.Join(t.TempDir(), "studio.exe")
	withStudio(t, studio)
	next := spawnsOf(t)
	for _, word := range []string{"help", "h", "?", "manual"} {
		h := newModel()
		c, ok := lookupCommand(word)
		if !ok || c.Site != sitePanel {
			t.Fatalf("%q is not a panel word", word)
		}
		openPanel(h, c.Panel)
		if got := strings.Join(next(), " "); got != studio+" -view help" {
			t.Errorf("%q started %q, want Studio on its Help", word, got)
		}
	}
}

func TestTheHelpWordOpensTheControlCentreOnlyWithoutStudio(t *testing.T) {
	noMarco(t)
	withStudio(t, "")
	next := spawnsOf(t)
	h := newModel()
	startHelp(h)
	if got := next(); len(got) < 3 || got[1] != "ui" || got[2] != "help" {
		t.Fatalf("with no Studio, help started %q, want the control centre's help", got)
	}
	if logs := strings.Join(h.snapshot().logs, "\n"); !strings.Contains(logs, "Studio is not beside marco") {
		t.Errorf("the line does not say why it is the control centre: %q", logs)
	}
}

func TestStudioIsFoundBesideMarco(t *testing.T) {
	dir := t.TempDir()
	marco := filepath.Join(dir, "marco.exe")
	if err := os.WriteFile(marco, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MARCO_STUDIO", "")
	t.Setenv("MARCO_BIN", marco)
	if got := studioBin(); got != "" {
		t.Errorf("no studio.exe beside marco, and %q was found", got)
	}
	studio := filepath.Join(dir, "studio.exe")
	if err := os.WriteFile(studio, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := studioBin(); got != studio {
		t.Errorf("found %q, want the studio.exe beside marco (%s)", got, studio)
	}
	t.Setenv("MARCO_STUDIO", `C:\elsewhere\studio.exe`)
	if got := studioBin(); got != `C:\elsewhere\studio.exe` {
		t.Errorf("$MARCO_STUDIO did not win: %q", got)
	}
}
