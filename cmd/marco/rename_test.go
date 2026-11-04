package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aDemonstratedRoutine is a routine saved the way a recording is kept, with its demonstration and
// a step account that types {{value}} — the state the reviewed rename re-renders from. A routine
// with no demonstration is refused by it (see TestTheCLIRenameNeedsTheDemonstration).
func aDemonstratedRoutine(t *testing.T, d orchestrator.Deps, name, value string) routes.Route {
	t.Helper()
	steps := []macroir.Step{{Kind: macroir.StepActivate, Text: "notepad"},
		{Kind: macroir.StepType, Text: "{{" + value + "}}"}, {Kind: macroir.StepWait, Ms: 250}}
	dr := &orchestrator.Draft{Name: name, Args: []string{value}, Steps: steps, App: "notepad",
		Mode: orchestrator.Unstated}
	rt, err := d.Save(dr, orchestrator.ScopeAppAnywhere, "notepad")
	if err != nil {
		t.Fatalf("saving the fixture: %v", err)
	}
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rec, err := json.Marshal([]recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "notepad", T: t0},
		{Kind: recorder.EvKey, KeyName: "a", Down: true, T: t0.Add(10 * time.Millisecond), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "a", Down: false, T: t0.Add(20 * time.Millisecond), App: "notepad"},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: t0.Add(30 * time.Millisecond), App: "notepad"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Reg.SaveRecording(rt, rec); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(steps)
	if err := d.Reg.SaveSteps(rt, routes.StoredSteps{Source: routes.DigestOf(string(src)),
		Demonstration: routes.DigestOf(string(rec)), Steps: raw}); err != nil {
		t.Fatal(err)
	}
	return rt
}

// `marco rename` is the reviewed rename: what it will do is said, ENTER MEANS NO, and a yes is the
// one move Studio's Rename makes. The routine's values, its shortcuts and the routine itself all
// arrive at the new name and nothing is left at the old one.
//
// Going back to RenameInPlace must fail this (the question would not be asked, and Enter would
// rename).
func TestTheCLIRenameIsTheReviewedOne(t *testing.T) {
	d, _ := hotkeyWorld(t)
	noRecording(t)
	theLeaderIs(t, "`")
	rt := aDemonstratedRoutine(t, d, "cloud pull", "resource")
	bindOK(t, d, "notepad", "notepad", "c", "cloud pull")

	answering(t, "") // Enter
	var out bytes.Buffer
	if code := renameReviewed(d, rt, "gather", &out); code != 0 || !strings.Contains(out.String(), "Nothing was renamed") {
		t.Fatalf("Enter at the question: %d\n%s", code, out.String())
	}
	if !d.Reg.Has(rt) {
		t.Fatal("Enter renamed it")
	}
	for _, want := range []string{`Rename "cloud pull" (` + rt.ID() + `) to "gather"`,
		"It keeps asking for its values: with resource", "Its shortcuts move with it: `c in notepad",
		"Enter means no"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the question does not say %q:\n%s", want, out.String())
		}
	}

	answering(t, "y")
	out.Reset()
	if code := renameReviewed(d, rt, "gather", &out); code != 0 {
		t.Fatalf("a yes: %d\n%s", code, out.String())
	}
	to := routes.Route{App: rt.App, Focus: rt.Focus, Slug: "gather"}
	if d.Reg.Has(rt) || !d.Reg.Has(to) {
		t.Fatalf("after the rename: old %v, new %v\n%s", d.Reg.Has(rt), d.Reg.Has(to), out.String())
	}
	if p := d.ShortcutPressed("notepad", "c"); !p.Found || len(p.Binding.Routes) != 1 || p.Binding.Routes[0] != to.ID() {
		t.Errorf("the shortcut after the rename: %+v", p.Binding)
	}
	src, err := os.ReadFile(d.Reg.Path(to))
	if err != nil || !strings.Contains(string(src), "{{resource}}") {
		t.Errorf("the renamed routine lost its value: %s", src)
	}
	// AND IT IS THE ROUTINE RE-RENDERED UNDER ITS NEW NAME, not the old file moved: what it calls
	// itself is what it is filed under.
	if got := orchestrator.NameIn(string(src), to.Slug); got != "gather" {
		t.Errorf("the renamed routine calls itself %q", got)
	}
}

// What the reviewed rename refuses, the CLI refuses: a name already taken, and a name that tries
// to change the values. Nothing is asked and nothing changes.
func TestTheCLIRenameRefusesWhatTheReviewRefuses(t *testing.T) {
	d, _ := hotkeyWorld(t)
	noRecording(t)
	rt := aDemonstratedRoutine(t, d, "cloud pull", "resource")
	other := aDemonstratedRoutine(t, d, "gather", "resource")
	answering(t, "y", "y")
	for _, name := range []string{"gather", "harvest with item"} {
		var out bytes.Buffer
		if code := renameReviewed(d, rt, name, &out); code != 1 || strings.Contains(out.String(), "Rename it?") {
			t.Errorf("renaming to %q: %d\n%s", name, code, out.String())
		}
	}
	if !d.Reg.Has(rt) || !d.Reg.Has(other) {
		t.Error("a refused rename changed the library")
	}
}

// A routine with no demonstration cannot be renamed by the reviewed rename, which re-renders from
// it, and the CLI now says so rather than moving the files itself. This is the one thing
// `marco rename` did before R7 and does not now; Studio's Rename has always refused it.
func TestTheCLIRenameNeedsTheDemonstration(t *testing.T) {
	d, _ := hotkeyWorld(t)
	rt := routes.Route{App: "notepad", Slug: "hand-written"}
	if err := d.Reg.Save(rt, typesSrc("hand written", "x")); err != nil {
		t.Fatal(err)
	}
	answering(t, "y")
	var out bytes.Buffer
	if code := renameReviewed(d, rt, "other", &out); code != 1 || !strings.Contains(out.String(), "no longer has the demonstration") {
		t.Errorf("renaming a routine with no demonstration: %d\n%s", code, out.String())
	}
	if !d.Reg.Has(rt) {
		t.Error("it was renamed anyway")
	}
}
