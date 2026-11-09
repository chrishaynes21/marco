package orchestrator

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// A change of key takes the old one off and puts the new one on in one write.
func TestAChangeMovesTheShortcutInOneWrite(t *testing.T) {
	d := shortcutWorld(t)
	old := assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay}})
	if _, err := d.AssignShortcut(Assignment{Key: "f", ListenIn: "game",
		Steps: []routes.Route{freeplay}, Changing: &Slot{ListenIn: "game", Key: "e",
			Version: old.Version()}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.Reg.BindingAt("game", "e"); ok {
		t.Error("the old key is still there after the change")
	}
	if b, ok := d.Reg.BindingAt("game", "f"); !ok || !runs(b, freeplay.ID()) {
		t.Errorf("the new key does not run the play: %+v", b)
	}
}

// A change of where it is listened for, on the same key.
func TestAChangeOfWhereItIsListenedFor(t *testing.T) {
	d := shortcutWorld(t)
	old := assign(t, d, Assignment{Key: "c", ListenIn: "game", Steps: []routes.Route{copyAny}})
	if _, err := d.AssignShortcut(Assignment{Key: "c", ListenIn: "",
		Steps: []routes.Route{copyAny}, Changing: &Slot{ListenIn: "game", Key: "c",
			Version: old.Version()}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.Reg.BindingAt("game", "c"); ok {
		t.Error("the app shortcut is still there after moving it to everywhere")
	}
	if _, ok := d.Reg.BindingAt("", "c"); !ok {
		t.Error("the shortcut is not listened for everywhere")
	}
}

// A change whose old shortcut is no longer the version shown writes NOTHING: neither the removal
// nor the new key.
func TestAChangeOfAShortcutThatChangedWritesNothing(t *testing.T) {
	d := shortcutWorld(t)
	old := assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay}})
	// Somebody else changes it in between.
	if _, err := d.AssignShortcut(Assignment{Key: "e", ListenIn: "game",
		Steps: []routes.Route{freeplay}, Cmd: "enter freeplay with 3",
		Consent: old.Version()}); err != nil {
		t.Fatal(err)
	}
	_, err := d.AssignShortcut(Assignment{Key: "f", ListenIn: "game",
		Steps: []routes.Route{freeplay}, Changing: &Slot{ListenIn: "game", Key: "e",
			Version: old.Version()}})
	if err == nil {
		t.Fatal("a change went ahead against a version nobody was shown")
	}
	if _, ok := d.Reg.BindingAt("game", "f"); ok {
		t.Error("the new key was written although the change was refused")
	}
	if _, ok := d.Reg.BindingAt("game", "e"); !ok {
		t.Error("the old key was removed although the change was refused")
	}
}

// A change onto a key that is in use still needs consent to what is there.
func TestAChangeOntoATakenKeyNeedsConsent(t *testing.T) {
	d := shortcutWorld(t)
	old := assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay}})
	taken := assign(t, d, Assignment{Key: "f", ListenIn: "game", Steps: []routes.Route{copyAny}})
	change := Assignment{Key: "f", ListenIn: "game", Steps: []routes.Route{freeplay},
		Changing: &Slot{ListenIn: "game", Key: "e", Version: old.Version()}}
	if _, err := d.AssignShortcut(change); err == nil {
		t.Fatal("a change took a key in use with nobody agreeing")
	}
	if _, ok := d.Reg.BindingAt("game", "e"); !ok {
		t.Error("the refused change removed the old key")
	}
	change.Consent = taken.Version()
	if _, err := d.AssignShortcut(change); err != nil {
		t.Fatal(err)
	}
	if b, _ := d.Reg.BindingAt("game", "f"); !runs(b, freeplay.ID()) {
		t.Errorf("after consent `f should run the changed play: %+v", b)
	}
}

// A change whose old shortcut has gone is refused, rather than quietly becoming a new shortcut.
func TestAChangeOfAShortcutThatHasGoneIsRefused(t *testing.T) {
	d := shortcutWorld(t)
	old := assign(t, d, Assignment{Key: "e", ListenIn: "game", Steps: []routes.Route{freeplay}})
	if _, err := d.RemoveShortcut("game", "e", old.Version()); err != nil {
		t.Fatal(err)
	}
	if _, err := d.AssignShortcut(Assignment{Key: "f", ListenIn: "game",
		Steps: []routes.Route{freeplay}, Changing: &Slot{ListenIn: "game", Key: "e",
			Version: old.Version()}}); err == nil {
		t.Fatal("a change of a shortcut that had gone wrote a new one")
	}
	if _, ok := d.Reg.BindingAt("game", "f"); ok {
		t.Error("the refused change wrote the new key")
	}
}
