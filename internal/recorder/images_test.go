package recorder

// WHERE THE PROCESS IMAGE COMES FROM, AND WHEN.
//
// The name of the program a press landed in is the one term of the five that cannot be read at
// the press. Reading it means opening a process handle, and that is the work that makes Windows
// drop the hooks — the invariant in CLAUDE.md, learned the hard way. So the press records the
// process ID, which is two window-data reads, and the NAME is resolved once the hooks are gone.
//
// That ordering is the whole design, and it has one consequence worth testing rather than
// assuming: by then a process may be gone, and the honest answer is nothing.

import (
	"testing"
)

// aRecordingAcrossTwoPrograms is two presses in a game and one in a browser, with their
// releases, which carry the same process and no window.
func aRecordingAcrossTwoPrograms() []RecordedEvent {
	press := func(pid uint32, x, y int) RecordedEvent {
		return RecordedEvent{Kind: EvClick, Down: true, Button: "left", X: x, Y: y,
			PID: pid, WinRel: true, RelX: x - 100, RelY: y - 50,
			RelTo: "window", WinW: 1920, WinH: 1080, Dpi: 96}
	}
	release := func(pid uint32, x, y int) RecordedEvent {
		return RecordedEvent{Kind: EvClick, Down: false, Button: "left", X: x, Y: y,
			PID: pid}
	}
	return []RecordedEvent{
		press(4812, 842, 274), release(4812, 842, 274),
		press(4812, 496, 283), release(4812, 496, 283),
		press(7330, 300, 120), release(7330, 300, 120),
	}
}

// EACH PRESS IS NAMED, AND EACH PROGRAM IS ASKED ABOUT ONCE.
//
// One lookup per distinct process, not one per click: a menu is thirty presses in one program,
// and thirty handle opens for one answer are thirty chances to be told a different one.
func TestEachPressIsNamedAndEachProgramAskedOnce(t *testing.T) {
	events := aRecordingAcrossTwoPrograms()
	var asked []uint32
	resolveImages(events, func(pid uint32) string {
		asked = append(asked, pid)
		switch pid {
		case 4812:
			return "arkascended.exe"
		case 7330:
			return "chrome.exe"
		}
		return ""
	})

	want := []string{"arkascended.exe", "", "arkascended.exe", "", "chrome.exe", ""}
	for i, w := range want {
		if events[i].Process != w {
			t.Errorf("event %d is named %q, want %q", i, events[i].Process, w)
		}
	}
	if len(asked) != 2 {
		t.Errorf("%d lookup(s) for two programs across six events: %v", len(asked),
			asked)
	}
}

// AND ONLY A PRESS IS NAMED, BECAUSE ONLY A PRESS MEASURED A WINDOW.
//
// The release carries the same process — both edges are stamped, which is what keeps a strip
// walking backwards from losing the press behind an unattributed release — but it records no
// geometry at all. A program name beside it would describe a window nothing was measured in, and
// the guarded contract reads these five terms together or not at all.
func TestOnlyAPressCarriesTheProgramName(t *testing.T) {
	events := aRecordingAcrossTwoPrograms()
	resolveImages(events, func(uint32) string { return "arkascended.exe" })

	for i := range events {
		if events[i].Down {
			continue
		}
		if events[i].Process != "" {
			t.Errorf("release %d is named %q, and it measured no window", i,
				events[i].Process)
		}
	}
}

// AND A PROGRAM THAT HAS ALREADY GONE TAKES THE WHOLE MEASUREMENT WITH IT.
//
// Not filled from the app poller's last sample, which is the one answer available and the wrong
// one: the point of these five terms is that somebody measured them. And not left as a window
// with a nameless owner either, which is what this did first — because a press that states a
// basis is asking replay to CHECK it, replay requires every term, and a point carrying four of
// five is a routine recorded honestly that can never run.
//
// So the basis comes off and the press is saved as an ordinary point, pressed by the rule every
// route before this was saved with. It is the same all-or-none rule the press itself applies
// (`newClick`), reaching the one term that cannot be read while the hooks are up.
func TestAProgramThatHasGoneUnmeasuresTheWindow(t *testing.T) {
	events := aRecordingAcrossTwoPrograms()
	resolveImages(events, func(uint32) string { return "" })

	for i := range events {
		if events[i].Process != "" {
			t.Errorf("event %d was named %q by a lookup that answered nothing",
				i, events[i].Process)
		}
	}
	e := events[0]
	if e.RelTo != "" || e.WinW != 0 || e.WinH != 0 || e.Dpi != 0 {
		t.Errorf("the press still claims a window it cannot name the owner of: "+
			"%+v", e)
	}
	// AND IT IS STILL AN ORDINARY POINT. The offset and the coordinate are what the
	// legacy path runs on, and losing those would make the press unreplayable rather
	// than unguarded.
	if !e.WinRel || e.RelX != 742 || e.RelY != 224 || e.X != 842 || e.Y != 274 {
		t.Errorf("the press lost its coordinate as well as its basis: %+v", e)
	}
}

// AND A PRESS WHOSE OWNER THE HOOK COULD NOT READ IS NOT LOOKED UP AT ALL.
//
// Process id zero means the hook could not read the owner. There is nothing to ask about, and
// asking anyway would be a handle open against process zero on every such event. It is
// unmeasured for the same reason as above: no owner, no guarded press.
func TestAPressWithNoProcessIsNotLookedUp(t *testing.T) {
	events := []RecordedEvent{{Kind: EvClick, Down: true, WinRel: true, X: 1, Y: 1,
		RelX: 1, RelY: 1, RelTo: "window", WinW: 800, WinH: 600, Dpi: 96}}
	asked := 0
	resolveImages(events, func(uint32) string { asked++; return "something.exe" })

	if asked != 0 {
		t.Errorf("%d lookup(s) for a press whose owner could not be read", asked)
	}
	if events[0].Process != "" || events[0].RelTo != "" {
		t.Errorf("a press with no process kept its window: %+v", events[0])
	}
}
