package meaning

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// aScreenWithSceneryAndContent has one operable control, one piece of scenery with the same
// label, and a text field holding the person's own writing.
func aScreenWithSceneryAndContent() *directorapi.WorldState {
	w := &directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{}}
	win := directorapi.WindowID("hwnd:1")
	add := func(id string, role directorapi.ElementRole, label, value string) {
		el := &directorapi.Element{
			ID: directorapi.ElementID(id), WindowID: win, Role: role,
			Label: label, Value: value,
			Bounds:  directorapi.Rect{X: 10, Y: 10, Width: 100, Height: 20},
			Enabled: true, Visible: true, Confidence: 0.9,
		}
		el.Provenance.Add(directorapi.ObservationReference{
			Observation: directorapi.ObservationID("obs:" + id),
			Source:      directorapi.SourceAccessibility,
		})
		w.Elements[el.ID] = el
	}
	add("e1", directorapi.RoleComboBox, "Theme", "Light")
	add("e2", directorapi.RoleText, "Theme", "")
	add("e3", directorapi.RoleTextField, "Display name", "Chris's private draft")
	add("e4", directorapi.RoleGroup, "Preferences", "")
	w.Windows = []directorapi.Window{{ID: win, Application: "msedge",
		Title: "a window title with a path in it", Focused: true, Visible: true}}
	w.ActiveWindow = &win
	return w
}

func TestOnlyOperableNamedControlsAreDescribed(t *testing.T) {
	got := Candidates(aScreenWithSceneryAndContent())
	ids := map[string]bool{}
	for _, c := range got {
		ids[c.ID] = true
	}
	if !ids["e1"] || !ids["e3"] {
		t.Errorf("an operable named control was left out: %+v", got)
	}
	// Scenery is left out: a proposal naming it would be rejected anyway, and sending it
	// is sending more of a person's screen than the question needs.
	if ids["e2"] {
		t.Error("static text was described to the interpreter")
	}
	if ids["e4"] {
		t.Error("a container was described to the interpreter")
	}
}

// AND THE PERSON'S OWN WRITING IS NOT SENT.
//
// The one thing a question like this must not leak. A text field's contents have no bearing on
// which control was meant, and there is nowhere in a Control to put them — which is the point:
// the shape of what is sent is the whole statement of what is sent.
func TestNothingButIdRoleAndLabelIsSent(t *testing.T) {
	const theirOwnWriting = "Chris's private draft"
	for _, c := range Candidates(aScreenWithSceneryAndContent()) {
		// CONTAINS, not equals. A field's contents appended to its label is the
		// shape this would actually take, and an equality check lets it straight
		// through.
		if strings.Contains(c.Label, theirOwnWriting) {
			t.Errorf("a text field's contents were described to the interpreter: %q",
				c.Label)
		}
		if c.Role == "" || c.ID == "" {
			t.Errorf("a candidate is missing its id or role: %+v", c)
		}
	}
	// Written as a shape check as well as a value check, so a field added later has to be
	// thought about: three fields, and each one named.
	sent := Candidates(aScreenWithSceneryAndContent())
	if len(sent) == 0 {
		t.Fatal("nothing was described, so this checked nothing")
	}
	first := sent[0]
	if first.ID == "" || first.Role == "" {
		t.Errorf("a candidate is missing its id or role: %+v", first)
	}
}

func TestWhatIsDescribedIsBoundedAndDeterministic(t *testing.T) {
	w := &directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{}}
	win := directorapi.WindowID("hwnd:1")
	for i := range HowManyControlsToOffer * 2 {
		id := directorapi.ElementID(string(rune('a'+i%26)) + string(rune('a'+i/26)))
		el := &directorapi.Element{
			ID: id, WindowID: win, Role: directorapi.RoleButton,
			Label:   "Button " + string(id),
			Bounds:  directorapi.Rect{X: 10, Y: 10, Width: 100, Height: 20},
			Enabled: true, Visible: true, Confidence: 0.9,
		}
		el.Provenance.Add(directorapi.ObservationReference{
			Observation: directorapi.ObservationID("obs:" + id),
			Source:      directorapi.SourceAccessibility,
		})
		w.Elements[id] = el
	}
	w.Windows = []directorapi.Window{{ID: win, Application: "x", Focused: true}}
	w.ActiveWindow = &win

	first := Candidates(w)
	if len(first) != HowManyControlsToOffer {
		t.Errorf("%d controls described, bound is %d", len(first), HowManyControlsToOffer)
	}
	for range 4 {
		again := Candidates(w)
		for i := range first {
			if again[i].ID != first[i].ID {
				t.Fatalf("two runs described the screen in different orders at %d: "+
					"%q then %q", i, first[i].ID, again[i].ID)
			}
		}
	}
}

const fakeInterpreter = `package main
import ("bufio";"encoding/json";"os")
func main(){
	bufio.NewReader(os.Stdin).ReadString('\n')
	json.NewEncoder(os.Stdout).Encode(map[string]string{"control": os.Getenv("FAKE_CONTROL")})
}`

func buildFake(t *testing.T) string {
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
		t.Fatalf("build fake: %v\n%s", err, out)
	}
	return bin
}

func TestAProposalMustBeAPropOfThisWorld(t *testing.T) {
	t.Setenv("MARCO_INTERPRETER", buildFake(t))
	w := aScreenWithSceneryAndContent()

	t.Setenv("FAKE_CONTROL", "e1")
	got, ok := Ask(context.Background(), w, "open dark mode")
	if !ok || got.ElementID != "e1" {
		t.Errorf("an honest proposal was discarded: %+v %v", got, ok)
	}

	// Scenery it was never offered, and a control that does not exist. Both refused, and
	// the second is refused against the WORLD rather than against the list that was sent
	// — the difference between checking the reply and checking reality.
	for _, said := range []string{"e2", "e4", "e99", "nothing"} {
		t.Setenv("FAKE_CONTROL", said)
		if got, ok := Ask(context.Background(), w, "open dark mode"); ok {
			t.Errorf("a proposal of %q was believed: %+v", said, got)
		}
	}
}

func TestWithoutAnInterpreterNothingIsAsked(t *testing.T) {
	t.Setenv("MARCO_INTERPRETER", "")
	if Available() {
		t.Error("an unset interpreter reports as available")
	}
	if _, ok := Ask(context.Background(), aScreenWithSceneryAndContent(),
		"open dark mode"); ok {
		t.Error("something proposed a control with no interpreter configured")
	}
}
