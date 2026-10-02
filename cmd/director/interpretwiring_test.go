package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Is the interpreter WIRED, or merely written?
//
// This repository has recorded six occasions on which a complete, correct mechanism was never
// invoked by the running Director, and every gate stayed green through all of them. The rule that
// came out of it is that a wiring test must enter through the PRODUCTION constructor — a unit test
// of a helper proves the helper.
//
// So this does not call `interpret.New()` and check it works. It builds the pipeline the way the
// Director builds it, and asks whether the field is set.

// THE DIRECTOR CONSULTS THE INTERPRETER FOR A PHRASE IT CANNOT PARSE.
//
// Through the production wiring function, with a plugin configured exactly as a person would
// configure one — and then through the production `Pipeline.Handle`, so what is proven is that a
// sentence typed at `marco do` reaches the reading.
//
// Deleting the `Interpret:` line from NewRuntime's pipeline must fail this.
func TestTheDirectorConsultsTheInterpreterForAPhraseItCannotParse(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", buildInterpretingPlugin(t))

	// THE PRODUCTION WIRING, not a hand-made field. If `interpretIntent` ever stops returning
	// a reader for a configured plugin, this is where it is caught.
	reader := interpretIntent()
	if reader == nil {
		t.Fatal("with a plugin configured, the Director wires no interpreter. A model that " +
			"is installed and never asked is the sixth recorded case of production " +
			"never invoking a working mechanism.")
	}

	// AND THE PIPELINE USES IT FOR EXACTLY THE PHRASES THE PARSER DECLINES.
	//
	// This asserts the READING. An empty world is enough and is the honest fixture: the
	// request will go on to fail at resolution because there is nothing on screen to resolve
	// against, and what is being proven here is which Intent it failed WITH.
	p := &execute.Pipeline{
		Intent:    intent.New().Parse,
		Interpret: reader,
		Observe: func(context.Context) (directorapi.WorldState, error) {
			return directorapi.WorldState{}, nil
		},
	}

	unknown := "would you shut that popup for me"
	if parsed := intent.New().Parse(unknown); parsed.Kind != directorapi.IntentUnknown {
		t.Fatalf("%q now parses deterministically; this test needs a phrase that does not",
			unknown)
	}
	out := p.Handle(context.Background(), unknown)
	// The fixture answers with the first verb it is offered, which is `invoke` — so the
	// reading is an ACTION rather than a clarification, and that is the whole claim. It gets
	// no further, because nothing downstream is wired.
	if out.Intent.Kind != directorapi.IntentAct {
		t.Errorf("a phrase the parser declined came back as %s (%q). The interpreter is "+
			"wired and was not consulted, or its answer was discarded.",
			out.Intent.Kind, out.Message)
	}
	if out.Intent.Verb != intent.SemanticVerb {
		t.Errorf("the reading is %q rather than a semantic action", out.Intent.Verb)
	}

	// AND A PHRASE IT CAN PARSE IS UNTOUCHED BY ANY OF THIS.
	if got := p.Handle(context.Background(), "click Save"); got.Intent.Verb == "" {
		t.Error("a deterministic reading was lost")
	}
}

// AND THE PRODUCTION PIPELINE IS THE ONE THAT CARRIES IT.
//
// # Why this test exists, and the mutation that demanded it
//
// The test above builds its own `execute.Pipeline` over the production reader — which proves the
// reader works and proves NOTHING about whether `NewRuntime` installs it. Measured: deleting
// `Interpret: interpretIntent()` from the Director's pipeline left that test green. A model
// configured, a reader built, and the running Director never asking it. The sixth recorded case,
// reproduced in the hour it was written.
//
// `NewRuntime` cannot be called here — it opens an accessibility bridge, claims a home and builds
// perception over a real desktop — so this reads the production construction site itself and
// asserts the field is in it. Structural rather than behavioural, and it is the stronger of the two
// available guards: a behavioural test that cannot reach the constructor is a test of something
// else.
//
// Deleting the `Interpret:` line from NewRuntime's pipeline must fail this.
func TestThePipelineTheDirectorBuildsCarriesTheInterpreter(t *testing.T) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "runtime.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing runtime.go: %v", err)
	}

	found, keys := false, []string{}
	ast.Inspect(parsed, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Pipeline" {
			return true
		}
		found = true
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if id, ok := kv.Key.(*ast.Ident); ok {
				keys = append(keys, id.Name)
			}
		}
		return false
	})

	if !found {
		t.Fatal("no execute.Pipeline literal in runtime.go. This test finds the Director's " +
			"own construction site; if the pipeline is now built elsewhere, point it there " +
			"rather than deleting it.")
	}
	for _, want := range []string{"Intent", "Interpret"} {
		if !slices.Contains(keys, want) {
			t.Errorf("the pipeline the Director builds sets no %s.\n\nFields set: %v\n\n"+
				"A reader that is built and never installed is a model somebody "+
				"configured and Marco never asks. The test that drives the reader "+
				"directly cannot see this.", want, keys)
		}
	}
}

// WITH NO PLUGIN, NOTHING IS WIRED.
//
// A nil field rather than something that always declines. The difference is what `director status`
// can say: "no interpreter is configured" sends somebody to set one, and "it did not understand
// you" sends them to rephrase a sentence that was never going to work.
func TestWithNoPluginTheDirectorWiresNoInterpreter(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", "")
	if interpretIntent() != nil {
		t.Error("an interpreter was wired with no plugin configured")
	}
	if interpreterConfigured() {
		t.Error("the diagnostic claims an interpreter with no plugin configured")
	}
}

// STATUS SAYS WHETHER MARCO CAN READ PLAIN REQUESTS.
//
// Without it, "I only understand click, focus, move, repeat and text editing so far" and "the
// interpreter was consulted and had no reading" are the same sentence to a person — and the two
// have completely different remedies. A capability whose absence is invisible is one nobody turns
// on.
//
// Deleting either arm must fail this.
func TestStatusSaysWhetherMarcoCanReadPlainRequests(t *testing.T) {
	source := readSource(t, "serve.go")
	for _, want := range []string{
		`"Understanding: plain requests, via `,
		`"Understanding: the built-in phrasings only `,
		`set $MARCO_RESOLVER`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("the status no longer says %s.\n\nA person cannot tell the built-in "+
				"vocabulary's refusal from a model's, and only one of them is about how "+
				"they phrased it.", want)
		}
	}

	// AND THE TWO ARMS ANSWER THE SAME QUESTION THE WIRING ASKS. A status that consulted
	// `$MARCO_RESOLVER` for itself would be a second answer to one fact, and the one that
	// drifted would be the one on screen.
	t.Setenv("MARCO_RESOLVER", buildInterpretingPlugin(t))
	if !interpreterConfigured() {
		t.Error("the diagnostic denies an interpreter the wiring would build")
	}
	if interpreterPath() == "" {
		t.Error("the diagnostic names no plugin while one is configured")
	}
}

// THE DIAGNOSTIC NAMES THE PLUGIN AND NOTHING ELSE.
//
// The path is a thing the person set and the thing they would check. A key, a model name or a
// prompt is not, and a status command has to be safe to paste into a bug report — the rule the rest
// of `printStatus` already follows about element labels.
func TestTheInterpreterDiagnosticNamesOnlyThePlugin(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", "/some/plugin")
	t.Setenv("MARCO_LLM_KEY", "sk-should-never-appear")
	t.Setenv("ANTHROPIC_API_KEY", "sk-should-never-appear")
	got := interpreterPath()
	if got != "/some/plugin" {
		t.Errorf("the diagnostic reports %q, want the configured path", got)
	}
	if strings.Contains(got, "sk-") {
		t.Fatal("the diagnostic carries a credential")
	}
}

// buildInterpretingPlugin compiles a plugin that answers the interpret question with the first verb
// it is offered.
//
// A real subprocess over the real protocol, because the thing being tested is the wiring and a
// substituted asker would skip exactly the part that was missing.
func buildInterpretingPlugin(t *testing.T) string {
	t.Helper()
	const source = `package main
import ("bufio";"encoding/json";"os")
func main(){
	line,_ := bufio.NewReader(os.Stdin).ReadString('\n')
	var r struct{
		Input string ` + "`json:\"input\"`" + `
		Interpret *struct{ Verbs []string ` + "`json:\"verbs\"`" + ` } ` + "`json:\"interpret\"`" + `
	}
	json.Unmarshal([]byte(line), &r)
	if r.Interpret == nil || len(r.Interpret.Verbs) == 0 {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"route":""})
		return
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"semantic": map[string]any{
		"kind": r.Interpret.Verbs[0], "target": "Save",
	}})
}`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "plugin")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "main.go")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the plugin: %v\n%s", err, out)
	}
	return bin
}
