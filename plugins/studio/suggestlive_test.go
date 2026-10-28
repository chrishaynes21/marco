package main

// AND ONE REAL PARAPHRASE REACHES OLLAMA AND COMES BACK AS AN IDENTITY.
//
// # Why a stand-in Advisor is not enough
//
// Because the thing this slice is built on top of is a resolver that was complete, tested,
// configured by the installer, and reached by nothing a person uses. Every test beside this one
// hands `SuggestRoutine` an Advisor of its own, which establishes the policy — which routines are
// offered, under what labels, which arm answered — and establishes NOTHING about whether a real
// request ever leaves this process. A suite of stand-ins is exactly what the dead resolver
// already had.
//
// So this one asks the real plugin, which asks the real model server, with a phrase that shares
// no word with the routine's name, and asserts the whole identity that comes back.
//
// # What it needs, and why it says so rather than passing
//
// A built `plugins/llama/llama.exe` and a model server answering on its default address. Neither
// is in this repository and neither is a thing a test may start: a model server is somebody's
// machine, and the plugin is a separate module. When either is absent this SKIPS, declared by
// name in `gate-skips.txt` with that reason — which is the gate's own rule, and the alternative
// is a test that quietly passes on a machine where nothing was asked.
//
// It is also the one test here that costs real time: a cold model load was measured at a few
// seconds, and the first converse call of a session was once observed to produce nothing at all.
// That degraded safely to the deterministic arm, which is why the bound below is generous rather
// than tight.

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
)

// theRealResolver is the built plugin in this tree, or "" when it has not been built.
func theRealResolver(t *testing.T) string {
	t.Helper()
	here, err := os.Getwd()
	if err != nil {
		return ""
	}
	// plugins/studio → plugins/llama, which is where its own build.cmd-less `go build`
	// puts it. Named rather than searched for: a test that hunted for an executable could
	// find somebody else's.
	at := filepath.Join(filepath.Dir(here), "llama", "llama.exe")
	if _, err := os.Stat(at); err != nil {
		return ""
	}
	return at
}

// aModelServerIsAnswering reports whether anything is listening where the plugin looks.
//
// A TCP dial and not a request: whether the model answers well is what the test is about, and
// this only has to tell "nothing is running here" from "something is". The address is the
// plugin's own default; a machine that has moved it has set $MARCO_LLM_URL, and this will skip
// rather than guess at it.
func aModelServerIsAnswering() bool {
	if os.Getenv("MARCO_LLM_URL") != "" {
		return false
	}
	c, err := net.DialTimeout("tcp", "localhost:11434", 2*time.Second)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func TestAParaphraseReachesTheResolverAndComesBackAsAnIdentity(t *testing.T) {
	plugin := theRealResolver(t)
	if plugin == "" {
		t.Skip("plugins/llama/llama.exe is not built, so the real resolver cannot be asked")
	}
	if !aModelServerIsAnswering() {
		t.Skip("no model server is answering on the plugin's default address, so the " +
			"real resolver cannot be asked")
	}
	// A GENEROUS CAP. The plugin's own default is 20s and a cold model load is most of a
	// first call; this test is about the connection, not about latency.
	t.Setenv("MARCO_LLM_TIMEOUT_MS", "60000")
	t.Setenv("MARCO_ASSISTANT", plugin)
	t.Setenv("MARCO_RESOLVER", plugin)

	a := aSavedRoutine(t, string(orchestrator.ScopeAppAnywhere), nil)
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d routine(s): %+v", len(rs), rs)
	}
	want := rs[0]
	// AND THE REAL ADVISOR, not this package's seam.
	a.advisor = nil
	if dispatch.Default() == nil {
		t.Fatalf("no Advisor is configured although %s was set", plugin)
	}

	// A PARAPHRASE: it shares only the application's name with "ark graphics", and
	// "video quality" is not "graphics" in any letter. The deterministic arm cannot
	// answer it — an exact match is normalized equality — so the Advisor IS consulted,
	// which is what the Source check below proves.
	//
	// CHOSEN BY PROBE, NOT BY TASTE, and the probe is worth recording: a FURTHER
	// paraphrase ("make the visuals in the dinosaur game look nicer") is read by this
	// model as a request to teach a new command, three times out of three. That is the
	// model's reach and not a defect in any of this — it reached Ollama and came back —
	// but it is why this test is a demonstration that the connection WORKS and is not a
	// claim about how well the model understands. Seven phrasings were tried; six
	// resolved identically three times each.
	const asked = "adjust ark video quality"
	said := a.SuggestRoutine(asked, want.App)

	if said.Source != dispatch.SourceAdvisor {
		t.Fatalf("%q was answered by %q, not by the resolver — resolver says %q; the "+
			"whole point of this test is that the request LEAVES this process",
			asked, said.Source, said.Resolver)
	}
	if !strings.Contains(said.Resolver, "answered this") {
		t.Errorf("the resolver's part is reported as %q", said.Resolver)
	}
	if said.Kind != dispatch.IntentRun {
		t.Fatalf("the resolver made %q of %q, reply %q", said.Kind, asked, said.Reply)
	}
	// THE WHOLE IDENTITY, which is the thing this slice is for: not a name, not a slug,
	// and not a label — the scope and the slug together, exactly as a card carries it and
	// exactly what Play is given.
	if said.ID != want.ID {
		t.Errorf("the resolver's suggestion is %q, want the whole identity %q",
			said.ID, want.ID)
	}
	if said.Target != want.App {
		t.Errorf("the request was read against %q, want %q", said.Target, want.App)
	}
	t.Logf("asked %q -> %s (%s) via %s", asked, said.ID, said.Scope, said.Resolver)
}
