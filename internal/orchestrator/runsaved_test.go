package orchestrator_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
)

// How these tests get a saved play to run, now that there is only one invocation spine.
//
// # What used to be here, and why it went
//
// Every test in this package used to enter through `orchestrator.Deps.Do` — resolve a phrase, put
// it through the authority door, run it. `Deps.Do` had no production callers by the time anybody
// looked: the shipped path is `invoke.Decide` → `cmd/marco/intake.go`'s `runInvocation` →
// `performOnePlay`, which calls this package's `Classify` and `Authorize` and then runs the play
// through `cmd/marco/panicstop.go`'s `runRoute`. So the densest authority coverage in the
// repository was proving things about a function nothing called. It was retired in Phase 3.
//
// # Why this helper is not a replacement for it
//
// Because a helper that resolved, authorized AND ran would be the same mistake in test clothing:
// a second spine, kept alive by the tests that mirror it, drifting from the one that ships.
// `cmd/marco/authoritybypass_test.go` is the record of what that costs — a bypass lived on the
// real `marco do` path while every authority test passed, because every authority test entered
// somewhere else.
//
// So the work is split, and the split is the point:
//
//	the DOOR          Classify / Authorize / AskFirst, called directly, in authority_test.go.
//	                  Those ARE the production units; intake.go calls exactly them.
//	the PLAY          this helper: run the saved file through the ordinary driver. The
//	                  screen-guard and destination tests are about what the GENERATED SOURCE says
//	                  and what the Screen host answers, and neither of those has anything to do
//	                  with how an invocation reached the file.
//	the WIRING        cmd/marco, beside `runInvocation`, where the product actually goes.
//
// This helper therefore claims one thing only: it ran the file. It does not consult the door, and
// a test that needs the door must say so out loud by calling `Authorize` itself.
func runSavedPlay(t *testing.T, d orchestrator.Deps, phrase string) {
	t.Helper()
	rt, ok := d.Reg.Resolve(d.App(), phrase)
	if !ok {
		t.Fatalf("no play answers to %q in %q", phrase, d.App())
	}
	if err := driver.RunFileWithHosts(d.Reg.Path(rt), d.Out, d.Hosts); err != nil {
		t.Fatalf("running %q: %v", phrase, err)
	}
}
