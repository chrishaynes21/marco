package orchestrator

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// A replacing move that lands between a press's first read of the shortcut and its lock: the
// move invalidated the shortcut and published another routine. The press reads the shortcut
// AGAIN under the lock, finds it gone, and runs nothing — without that second read it would pin,
// and run, whatever the move published.
//
// Deleting the second read in ShortcutPressed must fail this.
func TestAPressThatLosesItsShortcutMidwayRunsNothing(t *testing.T) {
	d := shortcutWorld(t)
	assign(t, d, Assignment{Key: "c", Steps: []routes.Route{copyAny}})
	d.BetweenPressReads = func() {
		// What a replacing move does, in its order: invalidate, then publish.
		if _, err := d.dropShortcutsNaming(copyAny); err != nil {
			t.Fatal(err)
		}
		if err := d.Reg.Save(copyAny, strings.Replace(shortcutSrc, "nothing", "something",
			1)); err != nil {
			t.Fatal(err)
		}
	}
	p := d.ShortcutPressed("game", "c")
	if len(p.Steps) != 0 || p.Refused == "" {
		t.Fatalf("a press whose shortcut was invalidated midway resolved %+v", p.Steps)
	}
}
