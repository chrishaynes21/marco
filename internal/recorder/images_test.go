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
	// A RELEASE IS MEASURED TOO, which is what the hook stamps now.
	//
	// It carried nothing here while the only two-edged gesture was a click, whose release
	// lands where its press did. A drag ends somewhere else, so both edges declare the
	// rectangle they are about — and a fixture describing the old stream is exactly the
	// hazard `attribution_windows_test.go`'s header names: a consumer tested against a
	// desktop that cannot happen.
	release := func(pid uint32, x, y int) RecordedEvent {
		return RecordedEvent{Kind: EvClick, Down: false, Button: "left", X: x, Y: y,
			PID: pid, WinRel: true, RelX: x - 100, RelY: y - 50,
			RelTo: "window", WinW: 1920, WinH: 1080, Dpi: 96}
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

	// EVERY EDGE, because every edge declared the window it was about. The lookups are
	// still two: one per distinct program, not one per event.
	want := []string{
		"arkascended.exe", "arkascended.exe",
		"arkascended.exe", "arkascended.exe",
		"chrome.exe", "chrome.exe",
	}
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

// AND WHAT IS NAMED IS WHATEVER DECLARED A WINDOW, NOT WHATEVER IS A PRESS.
//
// This read "only a press is named, because only a press measured a window", which was true of
// the stream and never of the condition: `resolveImages` asks whether an event claimed a
// rectangle. Both edges of a click claim one now, so both are named — and the discrimination
// being tested is the one that matters, which is that an event claiming NOTHING gets no name.
// A program name beside such an event would describe a window nothing was measured in, and the
// guarded contract reads its terms together or not at all.
func TestOnlyWhatDeclaredAWindowCarriesTheProgramName(t *testing.T) {
	events := aRecordingAcrossTwoPrograms()
	// AND ONE EVENT THAT MEASURED NOTHING, in the middle of a recording where
	// everything else did. Its process is readable and its basis is empty, so the only
	// thing that can leave it unnamed is the condition this is about.
	events = append(events, RecordedEvent{Kind: EvClick, Down: true, Button: "left",
		X: 70, Y: 70, PID: 4812})
	resolveImages(events, func(uint32) string { return "arkascended.exe" })

	for i := range events[:len(events)-1] {
		if events[i].Process == "" {
			t.Errorf("event %d declared the window it was about and was not "+
				"named", i)
		}
	}
	if last := events[len(events)-1]; last.Process != "" {
		t.Errorf("an event that declared no window is named %q, so a name now "+
			"describes a window nothing measured", last.Process)
	}
}

// AND A PROGRAM THAT HAS ALREADY GONE LEAVES THE NAME MISSING, AND MISSING IS WHAT IT STAYS.
//
// Not filled from the app poller's last sample, which is the one answer available and the wrong
// one: the point of these terms is that somebody measured them.
//
// And the press does NOT give up its basis over it, which was the first answer here. A press
// without a basis is an ordinary point and an ordinary point is pressed at its recorded
// coordinate having checked anything at all — so a failed lookup would have been quietly turning
// the safeguards off, on a recording the person is about to be offered. The press keeps saying
// which window it was about, the name stays missing, and guarded replay is unavailable for it
// until it is demonstrated again.
func TestAProgramThatHasGoneLeavesTheNameMissing(t *testing.T) {
	events := aRecordingAcrossTwoPrograms()
	resolveImages(events, func(uint32) string { return "" })

	for i := range events {
		if events[i].Process != "" {
			t.Errorf("event %d was named %q by a lookup that answered nothing",
				i, events[i].Process)
		}
	}
	e := events[0]
	if e.RelTo != "window" {
		t.Errorf("the press stopped saying which window it was about (%q) "+
			"because its program could not be named, which runs it "+
			"unchecked", e.RelTo)
	}
	// AND EVERYTHING THAT WAS MEASURED IS STILL THERE. Only the name is missing.
	if e.WinW != 1920 || e.WinH != 1080 || e.Dpi != 96 || !e.WinRel ||
		e.RelX != 742 || e.RelY != 224 {
		t.Errorf("a failed name lookup disturbed what WAS measured: %+v", e)
	}
}

// AND A PRESS WHOSE OWNER THE HOOK COULD NOT READ IS NOT LOOKED UP AT ALL.
//
// Process id zero means the hook could not read the owner. There is nothing to ask about, and
// asking anyway would be a handle open against process zero on every such event. The name is
// missing for the same reason as above, and stays missing for the same reason.
func TestAPressWithNoProcessIsNotLookedUp(t *testing.T) {
	events := []RecordedEvent{{Kind: EvClick, Down: true, WinRel: true, X: 1, Y: 1,
		RelX: 1, RelY: 1, RelTo: "window", WinW: 800, WinH: 600, Dpi: 96}}
	asked := 0
	resolveImages(events, func(uint32) string { asked++; return "something.exe" })

	if asked != 0 {
		t.Errorf("%d lookup(s) for a press whose owner could not be read", asked)
	}
	if events[0].Process != "" {
		t.Errorf("a press with no readable owner was named %q",
			events[0].Process)
	}
	if events[0].RelTo != "window" {
		t.Errorf("the press gave up its basis, so it will run unchecked: %+v",
			events[0])
	}
}

// AND A PRESS WITH NO OFFSET IS STILL NAMED, BECAUSE IT IS STILL ABOUT A WINDOW.
//
// The lookup follows the BASIS and not the offset. A press whose offset could not be placed
// inside the window it was attributed to — a click that changed the foreground, from inside the
// hook — is missing one measurement, and reading the program would add a second one it did not
// have to be missing. Review names each missing measurement, so a lookup skipped here becomes a
// person told their program could not be identified when it could.
func TestAPressWithNoOffsetIsStillNamed(t *testing.T) {
	events := []RecordedEvent{{Kind: EvClick, Down: true, X: 842, Y: 274, PID: 4812,
		// THE WINDOW WAS MEASURED AND THE PRESS WAS NOT IN IT: a basis, a
		// rectangle, a scale, and no offset.
		RelTo: "window", WinW: 1920, WinH: 1080, Dpi: 144}}
	resolveImages(events, func(uint32) string { return "arkascended.exe" })

	if events[0].Process != "arkascended.exe" {
		t.Errorf("the press was named %q; the program was readable and only "+
			"its offset was not", events[0].Process)
	}
}
