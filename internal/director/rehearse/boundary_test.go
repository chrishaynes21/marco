package rehearse_test

import (
	"go/build"
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
)

// What is allowed to reach a host, and what is structurally incapable of it.
//
// # The shape of the guarantee
//
// Roadmap 20 proved that everything Marco LEARNS lives in a package that cannot act. That is
// still true and is still checked there. This milestone adds the first package that CAN reach a
// host — through an injected interface — so the guarantee here is different in kind:
//
//	rehearse cannot act by itself.  It holds no host, opens no window, and imports
//	                               nothing that could. Whoever constructs the runner
//	                               decides what is at the other end.
//	rehearse cannot be reached by   observe and observesession do not import it, so no
//	the learning layer.             amount of evidence can find its way to a boundary.
//	the recorder cannot act.        It imports nothing that can perform input.

// forbidden are packages that can affect the machine, by import path fragment.
var forbidden = []struct{ fragment, why string }{
	{"internal/oshost", "the OS host: keyboard, mouse, clipboard, secrets"},
	{"internal/recorder", "installs low-level input hooks"},
	{"internal/driver", "drives input"},
	{"internal/winctx", "window activation and focus"},
	{"internal/screen", "screen capture and input geometry"},
	{"internal/platform", "platform adapters, including every real host"},
	{"internal/director/execute", "the execution pipeline"},
	{"internal/director/goal", "goal execution"},
	{"internal/director/plan", "action planning"},
	{"internal/director/target", "choosing what to act ON"},
	{"os/exec", "starting processes"},
}

func walkImports(path string, seen map[string]bool, depth int) error {
	if depth > 12 || seen[path] {
		return nil
	}
	seen[path] = true
	p, err := build.Import(path, ".", 0)
	if err != nil {
		return nil
	}
	for _, imp := range p.Imports {
		if !strings.Contains(imp, "chaynes-simpleclouds/marco") && !strings.Contains(imp, "/") {
			continue
		}
		if err := walkImports(imp, seen, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// The dry path holds no ability to act. It is handed one.
//
// `marcoexec` is deliberately NOT forbidden here — it is the boundary, and reaching it is the
// point. What it cannot reach is a HOST: marcoexec takes a `directorapi.MarcoRunner` and the
// composition root supplies it, which is why the same code can end at a keyboard or at a
// notebook without changing a line.
func TestTheDryPathCannotReachAHostByItself(t *testing.T) {
	const pkg = "github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	reachable := map[string]bool{}
	if err := walkImports(pkg, reachable, 0); err != nil {
		t.Fatalf("walking imports: %v", err)
	}
	for path := range reachable {
		for _, f := range forbidden {
			if strings.Contains(path, f.fragment) {
				t.Errorf("rehearse reaches %s\n\treason it is forbidden: %s\n\t"+
					"the dry path must be HANDED a runner, never able to build one",
					path, f.why)
			}
		}
	}
}

// The recording host cannot perform input.
//
// Part of the guarantee is that the thing at the bottom is incapable, not merely well behaved.
// A test host that imported the OS host would be one typo away from being one.
func TestTheRecordingHostIsIncapableOfActing(t *testing.T) {
	const pkg = "github.com/chaynes-simpleclouds/marco/internal/platform/recordhost"
	reachable := map[string]bool{}
	if err := walkImports(pkg, reachable, 0); err != nil {
		t.Fatalf("walking imports: %v", err)
	}
	for path := range reachable {
		for _, f := range forbidden {
			// It lives under internal/platform itself, which is where hosts belong. What
			// matters is that it reaches no OTHER platform adapter.
			if f.fragment == "internal/platform" && strings.HasSuffix(path, "recordhost") {
				continue
			}
			if strings.Contains(path, f.fragment) {
				t.Errorf("the recording host reaches %s (%s)", path, f.why)
			}
		}
	}
}

// Nothing the learning layer holds can reach the dry path.
//
// The inverse of the import check above, and the one that matters for authority: a
// ProcedureCandidate, an assessment, a judgement or a yes cannot find a boundary, because the
// packages they live in do not know this one exists.
func TestTheLearningLayerCannotReachTheDryPath(t *testing.T) {
	for _, pkg := range []string{
		"github.com/chaynes-simpleclouds/marco/internal/director/observe",
		"github.com/chaynes-simpleclouds/marco/internal/director/observesession",
	} {
		reachable := map[string]bool{}
		if err := walkImports(pkg, reachable, 0); err != nil {
			t.Fatalf("walking imports: %v", err)
		}
		for path := range reachable {
			if strings.Contains(path, "internal/director/rehearse") {
				t.Errorf("%s reaches the dry rehearsal path.\n\t"+
					"Evidence must not be able to find a boundary: only a CLAIMED grant "+
					"handed across by the composition root may", pkg)
			}
		}
	}
}

// A dry step is an engineering artefact, not evidence.
//
// It has no method or field by which it could report on the world, and nothing in the learning
// loop reads it. The alternative — a dry run that quietly counted — would mean Marco could
// convince itself a procedure works without ever having tried it.
func TestAStepEmissionIsNotEvidence(t *testing.T) {
	rt := reflect.TypeOf(rehearse.StepEmission{})
	for _, name := range []string{
		"Execute", "Run", "Replay", "Perform", "Apply", "Invoke", "Verify", "Confirm",
		"Promote", "Record", "Remember", "Store",
	} {
		if _, ok := rt.MethodByName(name); ok {
			t.Errorf("StepEmission has a %s method", name)
		}
		if _, ok := reflect.PointerTo(rt).MethodByName(name); ok {
			t.Errorf("*StepEmission has a %s method", name)
		}
	}
	// And it holds nothing captured. Same rule as every other record in this system.
	for _, field := range []string{"keycode", "scancode", "rawkey", "screenshot", "pixels",
		"image", "title", "label", "text", "password", "secret", "coordinate", "window"} {
		for i := 0; i < rt.NumField(); i++ {
			if strings.Contains(strings.ToLower(rt.Field(i).Name), field) {
				t.Errorf("StepEmission.%s could hold captured content", rt.Field(i).Name)
			}
		}
	}
	// The attempt cannot act either.
	at := reflect.TypeOf(rehearse.Attempt{})
	for _, name := range []string{"Execute", "Run", "Replay", "Perform", "Send", "Press"} {
		if _, ok := reflect.PointerTo(at).MethodByName(name); ok {
			t.Errorf("*Attempt has a %s method", name)
		}
	}
}
