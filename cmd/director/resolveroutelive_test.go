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
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/resolver"
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

// AND THE REAL PLUGIN ANSWERS THE EXTRACTION QUESTION WITH SPANS THAT VALIDATE.
//
// # What this establishes, and the much larger thing it does not
//
// ONE THING: that the third question on this protocol reaches the plugin on this machine, that
// the plugin reaches the model, and that what comes back is a span Marco accepts. That is a
// CONNECTION PROBE. Every rule about what a span may be is held without a model, in
// `internal/resolver` and `internal/resolveroute`, and Director's own reach is held in the default
// suite by `TestDirectorsOwnHandlerReachesTheExtractionQuestion`, which asks a plugin this
// repository builds.
//
// IT IS NOT A COMPREHENSION BENCHMARK AND MUST NOT BE REPORTED AS ONE. Whether a model reads
// sentences WELL is a measurement with a corpus and a denominator — see
// [[Experiment-069-rewording-closes-the-spelling-gap-and-not-the-meaning-gap]], which is what that
// kind of claim costs. One sentence answered correctly is the most misleading possible evidence
// about it: the route half of this same plugin reads a second paraphrase as a request to teach a
// new command, three times out of three, and that is the model's reach rather than a defect in
// anything here.
//
// # Why the route question is not in the way
//
// It asks `resolver.Extract` directly rather than through `Runtime.ResolveRoute`, on purpose. A
// request carrying a value can never be an exact match, so going through Director would put the
// model's ROUTE reading in front of the thing being probed — and a probe that fails because a 3B
// model offered to teach a new command has established nothing about extraction either way.
func TestTheRealPluginAnswersTheExtractionQuestion(t *testing.T) {
	plugin := theRealResolverPlugin(t)
	if plugin == "" {
		t.Skip("plugins/llama/llama.exe is not built, so the real resolver cannot be asked")
	}
	if !aModelServerIsAnswering() {
		t.Skip("no model server is answering on the plugin's default address, so the " +
			"real resolver cannot be asked")
	}
	t.Setenv("MARCO_LLM_TIMEOUT_MS", "60000")
	t.Setenv("MARCO_RESOLVER", plugin)
	if !resolver.ExtractorConfigured() {
		t.Fatalf("no extractor is configured although %s was set", plugin)
	}
	// A GENEROUS CAP for the same reason the test above has one: a cold model load is most
	// of a first call, and this is about the connection and not about latency.
	was := resolver.ExtractTimeout
	resolver.ExtractTimeout = 90 * time.Second
	t.Cleanup(func() { resolver.ExtractTimeout = was })

	// CHOSEN TO BE EASY, and said so: the value is at the end, after a word that introduces
	// it, in the shape a person would actually type. Making it hard would turn this into
	// the benchmark it must not be.
	const asked = "write a note saying hello there"
	spans, ok := resolver.Extract(context.Background(), asked, []string{"message"})
	if !ok {
		t.Fatalf("the plugin was asked %q and nothing usable came back — either it "+
			"did not reach the model, or the model answered with words that are "+
			"not in the sentence", asked)
	}
	var got []string
	for _, sp := range spans {
		v := sp.Value(asked)
		got = append(got, sp.Name+"="+v)
		// THE VALUE IS THE REQUEST'S OWN WORDS. Structurally true — the span has no
		// field for text — and asserted anyway, because this is the one test where
		// the words on the other side of the boundary are a model's.
		if !strings.Contains(asked, v) {
			t.Errorf("%q is not in the request", v)
		}
		if strings.TrimSpace(v) == "" {
			t.Errorf("%s came back as whitespace", sp.Name)
		}
	}
	t.Logf("asked %q -> %v (a connection probe, not a measurement of comprehension)",
		asked, got)
}
