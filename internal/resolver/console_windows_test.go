package resolver

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A PLUGIN IS STARTED WITH NO CONSOLE WINDOW (Windows only). Director runs console-less, so a
// console child it starts gets a new window: every resolution flashed a terminal, which is what
// the owner saw the first time a misheard request reached the model.
func TestAPluginIsGivenNoConsoleWindow(t *testing.T) {
	cmd := exec.Command("cmd.exe")
	Quiet(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow ||
		cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatalf("the plugin would get a console window: %+v", cmd.SysProcAttr)
	}
}

// AND EVERY PLACE THAT STARTS ONE DOES IT: each exec.Command in this package and in
// internal/dispatch (the advisor, the same binary) is followed by Quiet. Read as source, because the
// spawn is buried in functions that need a plugin to reach. Dropping one Quiet must fail this.
func TestEveryPluginStartIsQuiet(t *testing.T) {
	for _, dir := range []string{".", filepath.Join("..", "dispatch")} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(string(b), "\n")
			for i, l := range lines {
				s := strings.TrimSpace(l)
				if !strings.HasPrefix(s, "cmd := exec.Command") {
					continue
				}
				if i+1 >= len(lines) || !strings.Contains(lines[i+1], "Quiet(cmd)") {
					t.Errorf("%s:%d starts a plugin without Quiet: %s", f, i+1, s)
				}
			}
		}
	}
}
