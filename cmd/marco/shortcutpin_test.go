package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// A PRESS DOES NOT RUN WHAT WAS PUBLISHED AT THE IDENTITY AFTER IT RESOLVED.
//
// The press reads the shortcut and pins the routine's bytes under its lock; something saved there
// between that and the run — the window a replacing move used to leave — is refused at the run.
//
// Dropping the Pinned field from pressHotkey's request must fail this.
func TestAPressDoesNotRunWhatWasPublishedAfterItResolved(t *testing.T) {
	d, _ := hotkeyWorld(t)
	noRecording(t)
	save := routes.Route{Slug: "save"}
	if err := d.Reg.Save(save, runnableSrc); err != nil {
		t.Fatal(err)
	}
	bindOK(t, d, "discord", "", "s", "save")

	prev := betweenPressAndRun
	betweenPressAndRun = func() {
		// A DIFFERENT routine, published at the same identity.
		if err := d.Reg.Save(save, strings.Replace(runnableSrc, `"done"`, `"other"`, 1)); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { betweenPressAndRun = prev })

	said := press(t, d, "discord", "s")
	if strings.Contains(said, "[result] performed") {
		t.Fatalf("the press ran a play published after it resolved its shortcut:\n%s", said)
	}
	if !strings.Contains(said, "changed between the shortcut being pressed and the run") {
		t.Errorf("the refusal does not say what happened:\n%s", said)
	}
}
