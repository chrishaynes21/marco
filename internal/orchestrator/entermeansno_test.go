package orchestrator

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// F6: ENTER MEANS NO. A bare Enter at "Save this as …?" discards, and only an explicit yes saves.
//
// Taking Enter as save again must fail this.
func TestEnterAtTheSaveQuestionDiscards(t *testing.T) {
	for _, c := range []struct {
		answer string
		want   learnAction
	}{{"\n", actionDiscard}, {"y\n", actionSave}, {"YES\n", actionSave}, {"n\n", actionDiscard}} {
		var out bytes.Buffer
		d := Deps{In: strings.NewReader(c.answer), Out: &out}
		if got := d.chooseSave("say hello", false); got != c.want {
			t.Errorf("%q at the save question: %v, want %v", c.answer, got, c.want)
		}
		if !strings.Contains(out.String(), "[y]es / [N]o") {
			t.Errorf("the question does not show its default: %q", out.String())
		}
	}
}

// And a bare Enter at Simplify's "Save the simplified version?" leaves the routine as it was:
// that question replaces a routine, and only an explicit yes does.
func TestEnterAtTheSimplifyQuestionLeavesTheRoutine(t *testing.T) {
	reg := routes.Registry{Dir: t.TempDir()}
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvKey, KeyName: "h", Down: true, T: at(0)},
		{Kind: recorder.EvKey, KeyName: "e", Down: true, T: at(140)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(900)},
	}
	var out bytes.Buffer
	d := Deps{Reg: reg, Rec: &fakeRecorder{events: events}, In: strings.NewReader("y\nc\n"),
		Out: &out, App: func() string { return "editor" }, StopKey: "esc"}
	if err := d.Learn("type he"); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	rt := routes.Route{App: "editor", Slug: "type-he"}
	before, _ := os.ReadFile(reg.Path(rt))
	var out2 bytes.Buffer
	d2 := Deps{Reg: reg, Rec: &fakeRecorder{}, In: strings.NewReader("\n"), Out: &out2,
		App: func() string { return "editor" }}
	if err := d2.SimplifyRoute("type he"); err != nil {
		t.Fatalf("%v\n%s", err, out2.String())
	}
	after, _ := os.ReadFile(reg.Path(rt))
	if string(after) != string(before) || !strings.Contains(out2.String(), "Left unchanged") {
		t.Errorf("Enter at the simplify question changed the play:\n%s", out2.String())
	}
}
