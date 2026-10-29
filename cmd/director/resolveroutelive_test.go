//go:build resolver

// This file is behind `-tags resolver`, and the reason is the gate's ledger rather than the
// test.
//
// It needs two things that are not in this repository and that a test may not start: the
// `plugins/llama` binary, and a model server on somebody's machine. So it declined when either
// was absent — and a conditional skip is the one thing `gate-skips.txt` CANNOT express. An
// undeclared skip fails the gate, and a declaration whose test ran and did not skip fails it
// too, so the same honest test was a finding on a machine with Ollama running and a finding on
// one without. Not a defect in either rule: the ledger is for a skip that is always a skip, and
// this is a third thing.
//
// `-tags corpus` is the precedent (see the full-frame perception corpus, CLAUDE.md): a test that
// needs something the repository does not contain is out of the default suite and is REQUIRED
// for a change to the thing it covers. So:
//
//	go test -tags resolver ./cmd/director/     # required for a resolver change
//
// It is still the only test that exercises the connection rather than a stand-in, and the gate
// now says plainly that it did not run it instead of excusing a skip that did not happen.

package main

// AND ONE REAL PARAPHRASE REACHES THE RESOLVER THROUGH DIRECTOR.
//
// # Why a stand-in Advisor is not enough
//
// Because the thing this whole slice is built on top of is a resolver that was complete, tested,
// configured by the installer, and reached by nothing a person uses. Every other test here hands
// the resolution a stand-in, which establishes the policy and NOTHING about whether a real
// request leaves this process. A suite of stand-ins is exactly what the dead resolver already had.
//
// So this one goes through `Runtime.ResolveRoute` — Director's own handler, with Director's
// library and Director's context decision — and lets it reach the real plugin and the real model
// server, with a phrase that shares only the application's name with the routine's.
//
// # What it covers and what it does not
//
// It covers the RESOLUTION path end to end: Director's library reading, the eligibility filter,
// the labels, the plugin call, and the identity that comes back. It does NOT cross the service
// socket — that transport is held by `internal/director/service`'s own tests, with a fake runtime,
// which is the right split: this is about whether the question reaches a model, and that is about
// whether a request reaches a runtime.
//
// # What it needs, and why it declines rather than passing
//
// A built `plugins/llama/llama.exe` and a model server answering on its default address. Neither
// is in this repository and neither is a thing a test may start: a model server is somebody's
// machine, and the plugin is a separate module. When either is absent this SKIPS, declared by
// name in `gate-skips.txt` with that reason — the alternative is a test that quietly passes on a
// machine where nothing was asked.

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/resolveroute"
)

// theRealResolverPlugin is the built plugin in this tree, or "" when it has not been built.
func theRealResolverPlugin(t *testing.T) string {
	t.Helper()
	here, err := os.Getwd()
	if err != nil {
		return ""
	}
	// cmd/director → plugins/llama. Named rather than searched for: a test that hunted
	// for an executable could find somebody else's.
	at := filepath.Join(filepath.Dir(filepath.Dir(here)), "plugins", "llama", "llama.exe")
	if _, err := os.Stat(at); err != nil {
		return ""
	}
	return at
}

// aModelServerIsAnswering reports whether anything is listening where the plugin looks.
//
// A TCP dial and not a request: whether the model answers well is what the test is about, and
// this only has to tell "nothing is running here" from "something is". A machine that has moved
// the server has set $MARCO_LLM_URL, and this declines rather than guessing at it.
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

func TestAParaphraseReachesTheResolverThroughDirector(t *testing.T) {
	plugin := theRealResolverPlugin(t)
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
	if dispatch.Default() == nil {
		t.Fatalf("no Advisor is configured although %s was set", plugin)
	}

	reg := aLibraryHere(t)
	want := "arkascended/focus/ark-graphics"
	r := &Runtime{}

	// A PARAPHRASE: it shares only the application's name with "ark graphics", and
	// "video quality" is not "graphics" in any letter. The deterministic arm cannot
	// answer it — an exact match is normalized equality — so the resolver IS consulted,
	// which is what the Source check below proves.
	//
	// CHOSEN BY PROBE, NOT BY TASTE, and the probe is worth keeping: a further paraphrase
	// ("make the visuals in the dinosaur game look nicer") is read by this model as a
	// request to teach a new command, three times out of three. That is the model's reach
	// and not a defect in any of this — it reached Ollama and came back — but it is why
	// this is a demonstration that the connection works and not a claim about how well
	// the model understands.
	const asked = "adjust ark video quality"
	out := r.ResolveRoute(service.ResolveRoutePayload{
		Request: asked, Target: "arkascended", Library: reg.Dir})

	if out.Mismatch != "" {
		t.Fatalf("the libraries disagreed: %s", out.Mismatch)
	}
	a := out.Answer
	if a.Source != dispatch.SourceAdvisor {
		t.Fatalf("%q was answered by %q, not by the resolver — resolver says %q; the "+
			"whole point of this test is that the request LEAVES this process",
			asked, a.Source, a.Resolver)
	}
	if !strings.Contains(a.Resolver, "answered this") {
		t.Errorf("the resolver's part is reported as %q", a.Resolver)
	}
	if a.Kind != "run" {
		t.Fatalf("the resolver made %q of %q, reply %q", a.Kind, asked, a.Reply)
	}
	// THE WHOLE IDENTITY, which is what this slice is for: not a name, not a slug, and
	// not a label — every part an invocation is built from.
	if a.ID != want || a.App != "arkascended" || a.Slug != "ark-graphics" || !a.Focus {
		t.Errorf("the resolver's answer is %+v, want the whole identity of %s", a, want)
	}
	// AND IT WAS READ AGAINST WHAT WAS NAMED, through Director's own context decision.
	if a.Context.Basis != resolveroute.Stated || a.Context.App != "arkascended" {
		t.Errorf("the request was read against %+v", a.Context)
	}
	t.Logf("asked %q -> %s (%s) via %s", asked, a.ID, a.Scope, a.Resolver)
}
