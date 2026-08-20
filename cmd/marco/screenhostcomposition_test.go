package main

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/platform/screenhost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// COMPOSITION-ROOT HARD GATE (M2).
//
// `screenhost` and its `Recognition` interface are proved read-only by
// screenhost.TestTheScreenHostCannotAct. That guard stops at the package boundary. It cannot see
// what the COMPOSITION ROOT injects: `cmd/marco.newScreenHost` could hand the Screen host a broad,
// effect-capable dependency without touching the screenhost package at all, and every existing
// test would stay green.
//
// This gate closes that. It drives the real root (`newScreenHost`) and pins the injected backing
// to the narrow read type — one that is a `screenhost.Recognition` and is NOT a `runtime.Host`
// (the actuation interface) and exposes no actuation/mutation method by any name.
//
// The invariant: `Screen's Showing` may observe semantic state; nothing reachable from its
// production dependency may actuate the desktop.
func TestProductionScreenHostIsBackedOnlyByANarrowReader(t *testing.T) {
	// A temp memory path so the test reads no real store and is deterministic. Open never returns
	// nil (a missing file is a fresh store), so the root always injects a *liveScreens.
	t.Setenv("MARCO_MEMORY", filepath.Join(t.TempDir(), "memory.json"))

	h := newScreenHost() // THE real composition root.
	if h == nil {
		t.Fatal("newScreenHost returned nil")
	}

	// Read the unexported `world` field's DYNAMIC type without extracting its value, and pin it to
	// the narrow reader. If the root is ever changed to inject a broader object, this fails.
	world := reflect.ValueOf(h).Elem().Field(0)
	if world.IsNil() {
		t.Fatal("the production Screen host has no backing; it is supposed to read semantic memory")
	}
	backingType := world.Elem().Type()
	if backingType != reflect.TypeOf((*liveScreens)(nil)) {
		t.Fatalf("the production Screen host is backed by %s, not the narrow *liveScreens reader; "+
			"a different backing may be able to do more than look", backingType)
	}

	// Compile-time: the backing is a Recognition. Runtime: it is NOT an actuator. runtime.Host
	// (Invoke) is THE effect interface — a Screen backing that also satisfies it is exactly the
	// "broader runtime object that can reach execution" this gate exists to forbid.
	var _ screenhost.Recognition = (*liveScreens)(nil)
	var backing any = (*liveScreens)(nil)
	if _, isHost := backing.(runtime.Host); isHost {
		t.Fatal("the production Screen host backing implements runtime.Host — it can be asked to actuate")
	}

	// And it exposes none of the actuation/mutation method names, by any interface it satisfies.
	rt := reflect.TypeOf((*liveScreens)(nil))
	for _, forbidden := range []string{"Invoke", "Press", "Key", "Click", "Type", "Activate",
		"Run", "Execute", "NameSubject", "Remember", "Register", "Grant", "Save"} {
		if _, has := rt.MethodByName(forbidden); has {
			t.Errorf("the production Screen host backing can %s; asking where you are must not be "+
				"a way of doing something", forbidden)
		}
	}
}
