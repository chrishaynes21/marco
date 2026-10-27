//go:build windows

package main

// THE HARNESS CAN READ A MOVE RECORD.
//
// The fixture logs movement while a button is down — the travel of a drag, which is the one part
// of the gesture no account outside the application can see. A fixture that logs what nothing
// parses is a fixture that logs nothing, so this is the reading half.
//
// The rule about WHEN the fixture logs lives beside the C# itself, in `plugins/fixture`, because
// this module cannot hold it honestly: `go test` caches a result straight past a change to a file
// outside the module being tested, so a test here would go green against a broken fixture on
// every run but the first.

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTheHarnessReadsAMoveRecord(t *testing.T) {
	const line = `{"seq":7,"ms":120,"kind":"mouse","what":"move","control":"",` +
		`"mods":"","watched":"","on_control_x":40,"on_control_y":12,` +
		`"client_x":140,"client_y":92,"screen_x":740,"screen_y":492,` +
		`"window":{"x":600,"y":400,"w":1920,"h":1080},` +
		`"client":{"x":608,"y":431,"w":1904,"h":1041},"dpi":96}`

	var e fixtureEvent
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		t.Fatalf("the production record type refused a move record: %v", err)
	}
	if e.Kind != "mouse" || e.What != "move" {
		t.Errorf("the record came back as %s/%s", e.Kind, e.What)
	}
	if e.ScreenX != 740 || e.ScreenY != 492 {
		t.Errorf("the movement is at screen %d,%d, want 740,492", e.ScreenX,
			e.ScreenY)
	}
	if e.Window.W != 1920 || e.Window.H != 1080 {
		t.Errorf("the window stamped on the movement is %dx%d", e.Window.W,
			e.Window.H)
	}
	// AND IT READS AS A SENTENCE SOMEBODY CAN FOLLOW, which is what a failing scenario
	// hands a person. A record kind the printer did not know about would come out as a
	// line with holes in it exactly when somebody is reading it to find out what went
	// wrong.
	said := e.String()
	for _, want := range []string{"move", "740,492", "nothing"} {
		if !strings.Contains(said, want) {
			t.Errorf("the line for a movement is %q and does not mention %q",
				said, want)
		}
	}
}
