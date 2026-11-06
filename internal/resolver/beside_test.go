package resolver

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// THE RESOLVER IS FOUND BESIDE THE EXECUTABLE WHEN NOTHING NAMED ONE, and an explicit setting —
// including an empty one, which is how a test or a person turns it off — always wins.
//
// Reading only $MARCO_RESOLVER again must fail this: a Director started from Studio's Open widget
// then has no model to ask (ADR-321).
func TestTheResolverIsFoundBesideTheExecutable(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip("this platform cannot say where the test binary is")
	}
	dir := filepath.Join(filepath.Dir(exe), "plugins", "llama")
	if _, err := os.Stat(filepath.Join(filepath.Dir(exe), "plugins")); err == nil {
		t.Skip("something already put a plugins directory beside the test binary")
	}
	name := "llama"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	plugin := filepath.Join(dir, name)
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(filepath.Dir(exe), "plugins")) })

	unset := func() {
		was, had := os.LookupEnv("MARCO_RESOLVER")
		os.Unsetenv("MARCO_RESOLVER")
		t.Cleanup(func() {
			if had {
				os.Setenv("MARCO_RESOLVER", was)
			}
		})
	}
	unset()
	if p := Path(); p != "" {
		t.Errorf("with nothing beside it and nothing set, the resolver is %q", p)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plugin, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if p := Path(); p != plugin {
		t.Errorf("the plugin beside the executable was not found: %q", p)
	}
	t.Setenv("MARCO_RESOLVER", "")
	if p := Path(); p != "" {
		t.Errorf("an empty setting did not turn it off: %q", p)
	}
	t.Setenv("MARCO_RESOLVER", `C:\elsewhere\resolver.exe`)
	if p := Path(); p != `C:\elsewhere\resolver.exe` {
		t.Errorf("an explicit setting lost to the plugin beside it: %q", p)
	}
}
