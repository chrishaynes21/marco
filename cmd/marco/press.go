package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/voicelearn"
)

// runPress presses one key or one chord, exactly as named.
//
// "a key that's said" means the words arrive loosely worded: a plain key ("enter", "page up"), a
// punctuated chord ("ctrl+c"), or a spoken one ("control c", "control shift escape").
// `NormalizeChord` folds all three onto the canonical "mod+mod+key" spec the OS Key capability
// (and its backend's splitChord) already understands, so chords work for free.
//
// # Why this does NOT go through invoke.Decide, even though a product surface reaches it
//
// It is reachable from the product: the overlay's command line claims `press <key>` as one of its
// own verbs (`overlayVerb` in plugins/overlay/intake.go) and spawns `marco press …` directly, so
// this is not developer-only and saying it was would be wrong.
//
// But there is nothing here for the intake to decide. `invoke.Decide` answers ONE question — does
// Marco already know a durable behaviour by this name, or is this a request about the world for
// Director — and a key press is neither. It names no Play, resolves no identity, and has no
// provenance for `orchestrator.Authorize` to weigh (the door stops exactly one thing: a LEARNED
// play with intact provenance, and only to ask). Routing "press control c" through Decide would
// find no play by that name and hand the words to Director, which would go looking for something
// on screen called "press control c" — a strictly worse answer than the one a person asked for.
//
// It is the same shape as `marco run <file.marco>`: an imperative the person fully specified.
// Nothing is being looked up, so there is nothing to look it up wrongly.
//
// # What it DOES share with the intake, and must
//
// Cancellation. It runs under `withPanicStop`, which since the hooks and the cancellation were
// split apart watches [[internal/stopsignal]] on every run — so a `press` that holds a modifier
// down is reached by the same one "stop" as a Play, from any entrance and any process, and the
// host releases what it is holding on the way out. That is the property that actually matters for
// a direct effect: not "was it authorised" but "can it be stopped".
//
// Deleting the withPanicStop wrapper must fail TestADirectEffectIsStillCancellable.
func runPress(args []string) {
	spec := voicelearn.NormalizeChord(strings.Join(args, " "))
	if spec == "" {
		fmt.Fprintln(os.Stderr, `usage: marco press <key>   (e.g. "enter", "ctrl+c", "control shift escape")`)
		os.Exit(2)
	}
	host := oshost.New()
	err := withPanicStop(true, func(ctx context.Context) error {
		status, data, err := host.Invoke(runtime.HostCall{
			Act: "OS", Action: "Key", Input: runtime.Text(spec), Ctx: ctx, Out: os.Stdout,
		})
		if err != nil {
			return err
		}
		if status != "ok" {
			return fmt.Errorf("press %q: %s", spec, data.String())
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("[pressed] %s\n", spec)
}
