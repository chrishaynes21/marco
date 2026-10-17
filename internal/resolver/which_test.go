package resolver

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeInterpreter answers the "which control" question with whatever it is told to answer,
// through an environment variable — so one built binary can play an honest plugin, a plugin
// that names something it was never offered, and one that writes an essay.
const fakeInterpreter = `package main
import ("bufio";"encoding/json";"os")
func main(){
	bufio.NewReader(os.Stdin).ReadString('\n')
	json.NewEncoder(os.Stdout).Encode(map[string]string{
		"control": os.Getenv("FAKE_CONTROL"),
		"because": os.Getenv("FAKE_BECAUSE"),
	})
}`

func buildFakeInterpreter(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(fakeInterpreter), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "fake")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "main.go")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake interpreter: %v\n%s", err, out)
	}
	return bin
}

func someControls() []Control {
	return []Control{
		{ID: "e12", Role: "combo_box", Label: "Theme"},
		{ID: "e18", Role: "checkbox", Label: "Send notifications"},
	}
}

func TestAnInterpreterChoosesFromWhatItWasOffered(t *testing.T) {
	t.Setenv("MARCO_INTERPRETER", buildFakeInterpreter(t))
	t.Setenv("FAKE_CONTROL", "e12")
	t.Setenv("FAKE_BECAUSE", "a theme setting is where a dark appearance is chosen")

	got := WhichControl(context.Background(), "dark mode", "msedge", someControls())
	if got.Abstained() {
		t.Fatal("an honest answer was discarded")
	}
	if got.Control != "e12" {
		t.Errorf("chose %q", got.Control)
	}
	if !strings.Contains(got.Because, "theme setting") {
		t.Errorf("the reason did not survive: %q", got.Because)
	}
}

// AND IT CANNOT NAME A CONTROL THAT WAS NOT OBSERVED.
//
// The whole of that refusal, and it is a check rather than a hope. A plugin that returns
// anything else has abstained, whatever it thinks it said.
func TestAnIdThatWasNeverOfferedIsAnAbstention(t *testing.T) {
	t.Setenv("MARCO_INTERPRETER", buildFakeInterpreter(t))
	for _, said := range []string{"e99", "Theme", "the combo box", "e1", "", "none-of-these"} {
		t.Setenv("FAKE_CONTROL", said)
		got := WhichControl(context.Background(), "dark mode", "msedge", someControls())
		if !got.Abstained() {
			t.Errorf("a plugin naming %q was believed: %+v", said, got)
		}
	}
}

func TestNoInterpreterMeansTheQuestionIsNeverAsked(t *testing.T) {
	t.Setenv("MARCO_INTERPRETER", "")
	if InterpreterConfigured() {
		t.Error("an unset interpreter reports as configured")
	}
	if got := WhichControl(context.Background(), "dark mode", "msedge",
		someControls()); !got.Abstained() {
		t.Errorf("something answered with no plugin configured: %+v", got)
	}
	// AND AN UNSET INTERPRETER IS NOT A SEPARATE SWITCH FROM THE OTHER QUESTIONS BY
	// ACCIDENT. A plugin somebody configured to pick between their saved play names has
	// not agreed to be sent the labels of whatever is in front of them.
	t.Setenv("MARCO_RESOLVER", buildFakeInterpreter(t))
	t.Setenv("FAKE_CONTROL", "e12")
	if got := WhichControl(context.Background(), "dark mode", "msedge",
		someControls()); !got.Abstained() {
		t.Errorf("$MARCO_RESOLVER was asked what is on screen: %+v", got)
	}
}

func TestAnythingThatGoesWrongIsAnAbstention(t *testing.T) {
	t.Setenv("MARCO_INTERPRETER", filepath.Join(t.TempDir(), "no-such-program"))
	if got := WhichControl(context.Background(), "dark mode", "msedge",
		someControls()); !got.Abstained() {
		t.Errorf("a missing plugin produced an answer: %+v", got)
	}
	// Nothing to choose between, and no question worth asking.
	t.Setenv("MARCO_INTERPRETER", buildFakeInterpreter(t))
	t.Setenv("FAKE_CONTROL", "e12")
	if got := WhichControl(context.Background(), "dark mode", "msedge", nil); !got.Abstained() {
		t.Errorf("an empty screen produced an answer: %+v", got)
	}
	if got := WhichControl(context.Background(), "", "msedge",
		someControls()); !got.Abstained() {
		t.Errorf("an empty request produced an answer: %+v", got)
	}
}

// AND THE SENTENCE IT SENDS BACK IS BOUNDED, because it is going into a person's account of
// why something was chosen and is not a place for an essay.
func TestTheReasonIsBounded(t *testing.T) {
	t.Setenv("MARCO_INTERPRETER", buildFakeInterpreter(t))
	t.Setenv("FAKE_CONTROL", "e12")
	t.Setenv("FAKE_BECAUSE", strings.Repeat("because ", 200))
	got := WhichControl(context.Background(), "dark mode", "msedge", someControls())
	if len(got.Because) > 220 {
		t.Errorf("a %d-character reason was passed through", len(got.Because))
	}
	if !strings.HasSuffix(got.Because, "…") {
		t.Errorf("a truncated reason does not say it was truncated: %q", got.Because)
	}
}
