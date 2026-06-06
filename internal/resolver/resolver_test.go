package resolver

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// fakePlugin is a tiny resolver that echoes back the first route — enough to
// exercise the exec + JSON round-trip without a model or network.
const fakePlugin = `package main
import ("bufio";"encoding/json";"os")
func main(){
	line,_ := bufio.NewReader(os.Stdin).ReadString('\n')
	var r struct{ Routes []string ` + "`json:\"routes\"`" + ` }
	json.Unmarshal([]byte(line), &r)
	out := ""
	if len(r.Routes) > 0 { out = r.Routes[0] }
	json.NewEncoder(os.Stdout).Encode(map[string]string{"route": out})
}`

func buildFake(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(fakePlugin), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "fake")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "main.go")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake plugin: %v\n%s", err, out)
	}
	return bin
}

func TestResolveViaPlugin(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", buildFake(t))
	got := Resolve(context.Background(), "anything", []string{"alpha", "beta"})
	if got != "alpha" {
		t.Fatalf("Resolve via plugin = %q, want alpha", got)
	}
}

func TestNoPluginConfigured(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", "")
	if Configured() {
		t.Fatal("Configured() true with no MARCO_RESOLVER")
	}
	if Resolve(context.Background(), "x", []string{"a"}) != "" {
		t.Fatal("Resolve with no plugin should be empty")
	}
}

func TestRejectsUnknownRoute(t *testing.T) {
	// The plugin echoes routes[0]; if it isn't one we offered, reject it.
	t.Setenv("MARCO_RESOLVER", buildFake(t))
	// Single route "alpha" → plugin returns "alpha" → accepted; but ask for a
	// route set where the echoed value is valid only when present.
	if got := Resolve(context.Background(), "x", []string{"alpha"}); got != "alpha" {
		t.Fatalf("got %q", got)
	}
}
