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

// runPress presses a single key or a key chord — the spoken/typed key name(s).
// "a key that's said" means the phrase arrives loosely worded: a plain key
// ("enter", "page up"), a punctuated chord ("ctrl+c"), or a spoken chord
// ("control c", "control shift escape"). NormalizeChord folds all three into the
// canonical "mod+mod+key" spec the OS Key capability (and its backend's splitChord)
// already understands, so chords work for free. Real input, like `marco do`, under
// the Esc/stop-key panic-stop.
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
