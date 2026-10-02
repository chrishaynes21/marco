package resolver

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// The second question on the one plugin boundary, and the compatibility it must not break.

// buildPlugin compiles a one-file plugin and returns its path.
func buildPlugin(t *testing.T, source string) string {
	t.Helper()
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

// interpreting is a plugin that answers the interpret question by echoing the first verb offered,
// and reports back what it was sent so a test can assert the request.
const interpreting = `package main
import ("bufio";"encoding/json";"os")
func main(){
	line,_ := bufio.NewReader(os.Stdin).ReadString('\n')
	var r struct{
		Input string ` + "`json:\"input\"`" + `
		Routes []string ` + "`json:\"routes\"`" + `
		Interpret *struct{ Verbs []string ` + "`json:\"verbs\"`" + ` } ` + "`json:\"interpret\"`" + `
	}
	json.Unmarshal([]byte(line), &r)
	if r.Interpret == nil || len(r.Interpret.Verbs) == 0 {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"route":""})
		return
	}
	// The input is echoed back as the target, so a test can prove what crossed the boundary.
	json.NewEncoder(os.Stdout).Encode(map[string]any{"semantic": map[string]any{
		"kind": r.Interpret.Verbs[0], "target": r.Input, "ordinal": len(r.Routes),
	}})
}`

// routeOnly is a plugin written before the interpret question existed: it reads routes, answers a
// route, and knows nothing about anything else.
const routeOnly = `package main
import ("bufio";"encoding/json";"os")
func main(){
	line,_ := bufio.NewReader(os.Stdin).ReadString('\n')
	var r struct{ Routes []string ` + "`json:\"routes\"`" + ` }
	json.Unmarshal([]byte(line), &r)
	out := ""
	if len(r.Routes) > 0 { out = r.Routes[0] }
	json.NewEncoder(os.Stdout).Encode(map[string]string{"route": out})
}`

// A PLUGIN THAT ANSWERS THE QUESTION IS BELIEVED, WITHIN BOUNDS.
func TestInterpretViaPlugin(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", buildPlugin(t, interpreting))
	got, ok := Interpret(context.Background(), "shut that popup", []string{"dismiss", "invoke"})
	if !ok {
		t.Fatal("a plugin that answered produced no reading")
	}
	if got.Kind != "dismiss" {
		t.Errorf("kind = %q, want the first verb offered", got.Kind)
	}
	if got.Target != "shut that popup" {
		t.Errorf("target = %q; the plugin echoes the input, so this is what crossed", got.Target)
	}
	// AND NO ROUTES WERE SENT. The interpret question is not a route question, and a plugin
	// that answered one would be answering something nobody asked. The fixture reports the
	// count it received as the ordinal.
	if got.Ordinal != 0 {
		t.Errorf("the plugin was sent %d route(s) alongside an interpret question", got.Ordinal)
	}
}

// A ROUTE-ONLY PLUGIN IS NOT AN INTERPRETER, AND SAYS SO BY SAYING NOTHING.
//
// This is the compatibility claim the additive protocol rests on. A plugin built before the second
// question existed sees one field it does not recognise, answers `{"route":""}` exactly as it
// always did, and the caller correctly reads that as "nothing interpreted it" — which is the state
// an unconfigured plugin produces, and which the pipeline already handles.
//
// Breaking this would mean every existing plugin had to be rebuilt before anybody could run a
// Director, which is what a protocol version would have cost.
func TestARouteOnlyPluginDeclinesToInterpret(t *testing.T) {
	path := buildPlugin(t, routeOnly)
	t.Setenv("MARCO_RESOLVER", path)

	if _, ok := Interpret(context.Background(), "shut that popup", []string{"dismiss"}); ok {
		t.Error("a route-only plugin's `{\"route\":\"\"}` was read as an interpretation")
	}
	// AND IT STILL ANSWERS THE QUESTION IT KNOWS. The first question is untouched.
	if got := Resolve(context.Background(), "anything", []string{"alpha", "beta"}); got != "alpha" {
		t.Errorf("the route question now answers %q; the existing protocol must be "+
			"unchanged", got)
	}
}

// NO PLUGIN MEANS NO READING, AND IS NOT AN ERROR.
func TestInterpretWithNoPlugin(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", "")
	if _, ok := Interpret(context.Background(), "anything", []string{"invoke"}); ok {
		t.Error("an unconfigured resolver produced a reading")
	}
}

// AND NOTHING IS ASKED WITHOUT SOMETHING TO ASK ABOUT.
//
// An empty sentence or an empty vocabulary means the subprocess is never started. Not an
// optimisation: a plugin asked to choose from no verbs can only answer wrongly, and a Director that
// spawned a process to find that out would pay for it on every unparseable keystroke.
func TestInterpretAsksNothingWithoutAQuestion(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", buildPlugin(t, interpreting))
	if _, ok := Interpret(context.Background(), "", []string{"invoke"}); ok {
		t.Error("an empty request was interpreted")
	}
	if _, ok := Interpret(context.Background(), "do it", nil); ok {
		t.Error("a request was interpreted against no vocabulary")
	}
}

// A MALFORMED ANSWER IS DROPPED WHOLE.
//
// The shape rules, each one a case where keeping half the answer would act on a reading nothing
// produced. Driven through `clamp` directly, because what is being tested is the rule rather than
// the subprocess — and because each of these is one field of one answer.
func TestAMalformedAnswerIsDroppedWhole(t *testing.T) {
	long := make([]byte, MaxTargetPhrase+1)
	for i := range long {
		long[i] = 'a'
	}
	cases := []struct {
		why string
		in  Semantic
	}{
		{"no verb at all", Semantic{Target: "Save"}},
		{"a verb of only whitespace", Semantic{Kind: "   ", Target: "Save"}},
		{"a target longer than a control name", Semantic{Kind: "invoke", Target: string(long)}},
		{"a negative position", Semantic{Kind: "choose", Target: "result", Ordinal: -1}},
		{"a position past the tenth", Semantic{Kind: "choose", Target: "result",
			Ordinal: MaxOrdinal + 1}},
		{"a pointing and a name at once", Semantic{Kind: "dismiss", Target: "banner",
			Deictic: true}},
	}
	for _, c := range cases {
		if _, ok := clamp(c.in); ok {
			t.Errorf("an answer with %s was accepted: %+v\n\nThe alternative to dropping "+
				"it is guessing which half to keep, and the deterministic refusal is "+
				"still there to fall back on.", c.why, c.in)
		}
	}

	// AND A WELL-FORMED ONE SURVIVES, TRIMMED.
	got, ok := clamp(Semantic{Kind: " choose ", Target: "  result  ", Ordinal: 3})
	if !ok {
		t.Fatal("a well-formed answer was dropped")
	}
	if got.Kind != "choose" || got.Target != "result" || got.Ordinal != 3 {
		t.Errorf("a well-formed answer came back as %+v", got)
	}
}

// A CANCELLED REQUEST DOES NOT RUN THE PLUGIN.
//
// The caller's context reaches `exec.CommandContext`, which is how `stop` ends an interpretation
// that is waiting on a model rather than leaving a Director command open on a subprocess.
func TestInterpretHonoursACancelledContext(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", buildPlugin(t, interpreting))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, ok := Interpret(ctx, "shut that popup", []string{"dismiss"}); ok {
		t.Error("a cancelled interpretation produced a reading")
	}
}
