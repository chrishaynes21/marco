package dispatch

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// THE ADVISOR IS THE PLUGIN BESIDE THE EXECUTABLE WHEN NOTHING NAMED ONE: the same rule as
// resolver.Path, because the advisor and the extractor are the same binary. Reading
// $MARCO_RESOLVER alone here again must fail this.
func TestTheAdvisorIsThePluginBesideTheExecutable(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip("this platform cannot say where the test binary is")
	}
	root := filepath.Join(filepath.Dir(exe), "plugins")
	if _, err := os.Stat(root); err == nil {
		t.Skip("something already put a plugins directory beside the test binary")
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	name := "llama"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.MkdirAll(filepath.Join(root, "llama"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "llama", name), nil, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MARCO_ASSISTANT", "")
	was, had := os.LookupEnv("MARCO_RESOLVER")
	os.Unsetenv("MARCO_RESOLVER")
	t.Cleanup(func() {
		if had {
			os.Setenv("MARCO_RESOLVER", was)
		}
	})
	if !PluginConfigured() || Default() == nil {
		t.Errorf("the plugin beside the executable is not the advisor: %q", PluginPath())
	}
}
