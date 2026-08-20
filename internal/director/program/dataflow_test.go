package program_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// directorapiAct builds a minimal act intent, so a data-flow test can state exactly the
// program shape it is about rather than finding a phrase that happens to parse into it.
func directorapiAct(verb string, params map[string]any) directorapi.Intent {
	return directorapi.Intent{Kind: directorapi.IntentAct, Verb: verb, Parameters: params}
}

// Whole-program data flow, proved before the first step runs.
//
//	The validator proves the binding WILL exist if the earlier steps succeed.
//	Runtime execution still proves the capture actually did.

func TestACaptureMakesItsValueAvailableToLaterSteps(t *testing.T) {
	p, err := program.Decompose(
		"remember this field's value as email and then type ${email}", parse())
	if err != nil {
		t.Fatalf("a valid data flow was rejected: %v", err)
	}
	if len(p.Steps) != 2 {
		t.Fatalf("steps = %v, want 2", phrases(p))
	}
	c, binds := program.Binds(p.Steps[0].Operation)
	if !binds || c.Name != "email" {
		t.Fatalf("step 1 binds %+v, %v", c, binds)
	}
	ref, reads := program.Reads(p.Steps[1].Operation)
	if !reads || ref.Name != "email" {
		t.Fatalf("step 2 reads %+v, %v", ref, reads)
	}
}

func TestAnApostropheDoesNotSwallowTheRestOfTheRequest(t *testing.T) {
	// A possessive is not a quoted run. Treating "field's" as an opening quote made
	// every conjunction after it invisible, so the whole request collapsed into one
	// clause whose "name" was "email and then type ${email}".
	p, err := program.Decompose(
		"remember this window's title as title and then type ${title}", parse())
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	if len(p.Steps) != 2 {
		t.Fatalf("steps = %v, want 2", phrases(p))
	}
	// Text the user genuinely quoted is still data, and is still not split.
	one, err := program.Decompose(`type 'save and exit'`, parse())
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	if len(one.Steps) != 1 {
		t.Fatalf("a quoted conjunction was split: %v", phrases(one))
	}
}

func TestUseBeforeCaptureIsRejectedBeforeAnythingRuns(t *testing.T) {
	// The whole reason this check exists: discovering at step 3 that ${email} was never
	// captured, having already typed into two controls, has no undo.
	_, err := program.Decompose(
		"type ${email} and then remember this field's value as email", parse())
	if err == nil {
		t.Fatal("a value used before it was captured was accepted")
	}
	if got := err.Error(); !strings.Contains(got, `Value "email" is used before it is captured.`) {
		t.Fatalf("error = %q", got)
	}
}

func TestAValueNoStepCapturesIsRejected(t *testing.T) {
	_, err := program.Decompose("click Search and then type ${nobody}", parse())
	if err == nil {
		t.Fatal("a reference to a value nothing captures was accepted")
	}
	if !strings.Contains(err.Error(), "nobody") {
		t.Fatalf("error = %q, want it to name the value", err)
	}
}

func TestAStepCannotConsumeTheValueItCaptures(t *testing.T) {
	// A step that appeared to both capture and consume would have to decide whether the
	// read happened before or after the write.
	p := program.Program{Steps: []program.Step{captureStep("x"), useStep("x")}}
	if err := program.ValidateDataFlow(p); err != nil {
		t.Fatalf("capture-then-use was rejected: %v", err)
	}
	both := captureStep("x")
	both.Operation.Parameters[values.ParamInput] = values.ReferenceInput("x")
	if err := program.ValidateDataFlow(program.Program{Steps: []program.Step{both}}); err == nil {
		t.Fatal("a step that captured and consumed the same name was accepted")
	}
}

func TestCapturingTheSameNameTwiceIsRejected(t *testing.T) {
	// Values are immutable, so a second capture under the same name could only mean one
	// of them is silently discarded.
	p := program.Program{Steps: []program.Step{captureStep("customer"), captureStep("customer")}}
	err := program.ValidateDataFlow(p)
	if err == nil {
		t.Fatal("a duplicate capture was accepted")
	}
	if !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("error = %q, want it to say why", err)
	}
}

func TestAProgramMayNotCaptureMoreValuesThanTheLimit(t *testing.T) {
	// MaxSteps is 10 and MaxValues is 20, so the count limit is unreachable through
	// Decompose. Checked directly, because the limit is the validator's promise and not
	// an accident of another limit being smaller.
	var steps []program.Step
	for i := 0; i <= values.MaxValues; i++ {
		steps = append(steps, captureStep(string(rune('a'+i%26))+strings.Repeat("x", i)))
	}
	if err := program.ValidateDataFlow(program.Program{Steps: steps}); err == nil {
		t.Fatal("an unbounded number of captures was accepted")
	}
}

func TestValidationFailureMeansNoStepRan(t *testing.T) {
	// Decompose returns the rejected program rather than a partially built one, and its
	// status says so: nothing downstream may treat it as runnable.
	p, err := program.Decompose(
		"type ${email} and then remember this field's value as email", parse())
	if err == nil {
		t.Fatal("expected rejection")
	}
	if p.Status != program.StatusRejected {
		t.Fatalf("status = %q, want %q", p.Status, program.StatusRejected)
	}
}

func TestAValueReferenceIsNoLongerTreatedAsAnUnfilledPlaceholder(t *testing.T) {
	// "${" used to mark a template nobody filled in. It is now the value-reference
	// token, and a program using one is a real instruction.
	if _, err := program.Decompose(
		"remember the clipboard as clip and then type ${clip}", parse()); err != nil {
		t.Fatalf("a value reference was rejected as a placeholder: %v", err)
	}
	// The genuine placeholders are still refused.
	if _, err := program.Decompose("click Save and then type {{name}}", parse()); err == nil {
		t.Fatal("an unfilled template was accepted")
	}
}

func TestAnEnvironmentIsCreatedOnceAndReusedOnResume(t *testing.T) {
	// Idempotence is what makes a RESUME correct: the paused program brings its
	// environment back, and a second call must not replace it with an empty one.
	var ctx program.Context
	env := ctx.EnsureValues()
	if env == nil {
		t.Fatal("no environment was created")
	}
	if err := env.Bind("customer", mustValue(t, "Alice")); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if again := ctx.EnsureValues(); again != env {
		t.Fatal("a second call replaced the environment")
	}
	if !ctx.Values.Has("customer") {
		t.Fatal("the captured value was lost")
	}
}

func mustValue(t *testing.T, text string) values.Value {
	t.Helper()
	v, err := values.New(values.KindText, text, "test", values.VisibilityNormal)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	return v
}

// captureStep builds a step that binds name, without going through the parser.
func captureStep(name string) program.Step {
	return program.Step{
		Phrase:        "remember something as " + name,
		FailurePolicy: program.Stop,
		Operation: directorapiAct(intent.VerbCaptureValue, map[string]any{
			values.ParamCapture: values.Capture{Kind: values.CaptureClipboard, Name: name},
		}),
	}
}

// useStep builds a step that consumes name.
func useStep(name string) program.Step {
	return program.Step{
		Phrase:        "type ${" + name + "}",
		FailurePolicy: program.Stop,
		Operation: directorapiAct("edit", map[string]any{
			values.ParamInput: values.ReferenceInput(name),
		}),
	}
}
