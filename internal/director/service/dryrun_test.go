package service

// A dry run never reaches a live runtime, however the service was started.
//
// # The defect
//
// `ExecutePayload.DryRun` was sent by the client and never read by the server. `Server.execute`
// called `Handle` unconditionally, so `director execute --dry-run "click Save"` against an
// ordinary Director typed, clicked and focused for real. Only `director serve --dry-run` ever made
// a runtime dry, and the two facts were never compared.
//
// A flag that claims a safety property and does not have it is worse than no flag, because it is
// the one somebody reaches for when they are being careful. It was reached for in exactly that
// spirit during the Alpha 1 evaluation, which is how it was found — by reading, before running.
//
// # Why the test is here and not in cmd/director
//
// Because the hole was at the SERVER boundary. A CLI test proves the flag is sent; it cannot prove
// anybody acts on it. This drives the real `Server` over a real connection with a runtime that
// says it can reach the desktop, and asserts the phrase never arrives.

import (
	"strings"
	"testing"
)

// A DRY-RUN REQUEST NEVER REACHES A LIVE RUNTIME.
//
// Both halves. Against a live runtime the request is refused and `Handle` is never called; against
// a dry one it is served exactly as before, because that is the case where the flag tells the
// truth and a refusal would be pointless ceremony.
func TestADryRunNeverReachesALiveRuntime(t *testing.T) {
	for _, tc := range []struct {
		name string
		// wet is what the runtime says about itself.
		wet bool
		// handled is whether the phrase should reach Handle.
		handled bool
	}{
		{"a Director that can perform real input", true, false},
		{"a Director started with --dry-run", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt := newFakeRuntime()
			rt.wet = tc.wet
			_, dir := serve(t, rt)
			c := dial(t, dir)

			_, err := c.Execute("click Save", true, nil)

			rt.mu.Lock()
			calls := append([]string(nil), rt.calls...)
			rt.mu.Unlock()

			reached := len(calls) > 0
			if reached != tc.handled {
				t.Fatalf("the phrase reached the runtime = %v, want %v (calls: %v).\n"+
					"A dry run that reaches a live runtime performs real input "+
					"under a flag documented not to.", reached, tc.handled,
					calls)
			}
			if tc.handled {
				return
			}
			// AND THE PERSON IS TOLD WHY, with somewhere to go.
			if err == nil {
				t.Fatal("the dry run was refused silently; a caller cannot tell a " +
					"refusal from a preview that found nothing to do")
			}
			for _, want := range []string{"real input", "Nothing was done", "director plan"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal does not mention %q: %s", want,
						err.Error())
				}
			}
		})
	}
}

// AND AN ORDINARY REQUEST IS UNAFFECTED, which is the behaviour this must not change.
func TestAnOrdinaryRequestIsNotRefusedByTheDryRunGuard(t *testing.T) {
	rt := newFakeRuntime()
	rt.wet = true
	_, dir := serve(t, rt)
	c := dial(t, dir)

	if _, err := c.Execute("click Save", false, nil); err != nil {
		t.Fatalf("an ordinary request was refused: %v", err)
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if len(rt.calls) != 1 || rt.calls[0] != "click Save" {
		t.Errorf("the runtime saw %v, want one call to \"click Save\"", rt.calls)
	}
}
