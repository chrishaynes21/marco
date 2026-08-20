package dispatch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var demoRoutes = []string{"open-chest", "mute-discord"}

// funcAdvisor adapts a function to the Advisor interface for tests.
type funcAdvisor func(input string) (Decision, bool)

func (f funcAdvisor) Advise(_ context.Context, input string, _ []string, _ string) (Decision, bool) {
	return f(input)
}

func decide(t *testing.T, d Dispatcher, input string) Decision {
	t.Helper()
	return d.Decide(context.Background(), input, demoRoutes, "")
}

// --- deterministic-only (no Advisor) ----------------------------------------

func TestExactRunsWithoutAdvisor(t *testing.T) {
	got := decide(t, Dispatcher{}, "open chest")
	if got.Intent != IntentRun || got.Route != "open-chest" {
		t.Fatalf("got %+v, want run open-chest", got)
	}
}

func TestNearMatchClarifies(t *testing.T) {
	// "mute discord app" fuzzily matches mute-discord (~0.67) but isn't exact.
	got := decide(t, Dispatcher{}, "mute discord app")
	if got.Intent != IntentClarify || got.Route != "mute-discord" {
		t.Fatalf("got %+v, want clarify mute-discord", got)
	}
	if got.Reply == "" {
		t.Error("clarify should carry a reply")
	}
}

func TestUnknownTeaches(t *testing.T) {
	got := decide(t, Dispatcher{}, "order a pizza")
	if got.Intent != IntentTeach || got.Name != "order a pizza" {
		t.Fatalf("got %+v, want teach 'order a pizza'", got)
	}
}

func TestEmptyIsNone(t *testing.T) {
	if got := decide(t, Dispatcher{}, "   "); got.Intent != IntentNone {
		t.Fatalf("got %+v, want none", got)
	}
}

// --- with an Advisor --------------------------------------------------------

func TestAdvisorTeachPassesThrough(t *testing.T) {
	adv := funcAdvisor(func(string) (Decision, bool) {
		return Decision{Intent: IntentTeach, Name: "do laundry", Reply: "Show me how."}, true
	})
	got := decide(t, New(adv), "some novel request")
	if got.Intent != IntentTeach || got.Name != "do laundry" || got.Reply != "Show me how." {
		t.Fatalf("got %+v, want advisor's teach", got)
	}
}

func TestAdvisorRunValidated(t *testing.T) {
	adv := funcAdvisor(func(string) (Decision, bool) {
		return Decision{Intent: IntentRun, Route: "open-chest", Reply: "Opening it."}, true
	})
	got := decide(t, New(adv), "crack it open")
	if got.Intent != IntentRun || got.Route != "open-chest" {
		t.Fatalf("got %+v, want run open-chest", got)
	}
}

func TestAdvisorHallucinatedRouteRejected(t *testing.T) {
	// Advisor names a route the user doesn't have → discard it, fall back to
	// deterministic (this input is unknown → teach).
	adv := funcAdvisor(func(string) (Decision, bool) {
		return Decision{Intent: IntentRun, Route: "launch-rockets"}, true
	})
	got := decide(t, New(adv), "zzz nonsense")
	if got.Intent != IntentTeach {
		t.Fatalf("got %+v, want teach (rejected hallucinated route)", got)
	}
}

func TestAdvisorNotOkFallsBack(t *testing.T) {
	adv := funcAdvisor(func(string) (Decision, bool) { return Decision{}, false })
	got := decide(t, New(adv), "mute discord app")
	if got.Intent != IntentClarify {
		t.Fatalf("got %+v, want deterministic clarify", got)
	}
}

func TestExactSkipsAdvisor(t *testing.T) {
	called := false
	adv := funcAdvisor(func(string) (Decision, bool) {
		called = true
		return Decision{Intent: IntentChat, Reply: "hi"}, true
	})
	got := decide(t, New(adv), "open chest")
	if got.Intent != IntentRun {
		t.Fatalf("got %+v, want run", got)
	}
	if called {
		t.Error("advisor consulted on an exact match (should be the free fast path)")
	}
}

// --- PluginAdvisor over a fake plugin binary --------------------------------

// fakePlugin echoes a converse decision, so we exercise the exec + JSON round-trip
// without a model. It only answers converse requests.
const fakePlugin = `package main
import ("bufio";"encoding/json";"os")
func main(){
	line,_ := bufio.NewReader(os.Stdin).ReadString('\n')
	var r map[string]any
	json.Unmarshal([]byte(line), &r)
	out := map[string]string{}
	if r["mode"] == "converse" { out = map[string]string{"intent":"chat","reply":"hello there"} }
	json.NewEncoder(os.Stdout).Encode(out)
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

func TestPluginAdvisorRoundTrip(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", "")
	t.Setenv("MARCO_ASSISTANT", buildFake(t))
	if !PluginConfigured() {
		t.Fatal("PluginConfigured false with MARCO_ASSISTANT set")
	}
	got := New(Default()).Decide(context.Background(), "hey marco", demoRoutes, "")
	if got.Intent != IntentChat || got.Reply != "hello there" {
		t.Fatalf("got %+v, want chat 'hello there'", got)
	}
}

func TestDefaultNilWhenUnconfigured(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", "")
	t.Setenv("MARCO_ASSISTANT", "")
	if Default() != nil {
		t.Fatal("Default() should be nil with no plugin configured")
	}
}
