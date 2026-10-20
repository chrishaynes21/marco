package main

// A DEMONSTRATED PLAY IS NOT RUN BY THE THING THAT CANNOT SUPERVISE IT.
//
// Through the PRODUCTION host map — `buildHosts`, the same one every `marco run` uses — because
// the thing being checked is the wiring and a test that built its own map would prove only that
// the refusal compiles. See the memory note "prove wiring by deleting it".

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

func TestADemonstratedPlayIsNotRunByMarcoItself(t *testing.T) {
	// THE PRODUCTION MAP, built by the production constructor.
	hosts := newDeps().Hosts
	h, ok := hosts["Demonstrated"]
	if !ok {
		t.Fatal("the Demonstrated act is not registered, so it falls through to the " +
			"catch-all — which answers it as an OS action nobody asked for")
	}
	_, _, err := h.Invoke(runtime.HostCall{Act: "Demonstrated", Action: "Press",
		Input: runtime.Absent(), Ctx: context.Background()})
	if err == nil {
		t.Fatal("marco answered a demonstrated press itself; nothing here can ground " +
			"a control, ask a person, or check an outcome")
	}
	// AND IT SAYS WHERE TO GO. A refusal that does not name the other door is a dead end
	// a person has to guess their way out of.
	for _, want := range []string{"demonstrated", "fresh look", "by name"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q: %v", want, err)
		}
	}
}
