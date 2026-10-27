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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// AND IT NOTICES A MOMENT INSIDE A GESTURE, which the ordinary cadence cannot.
//
// A drag's travel is 24 steps of 8ms — about 190ms, fixed however far the drag goes — and the
// stopped-drag scenario has that long to notice the press AND get a Stop request in before the
// release. Asking every 100ms spends half of it on the noticing, and the first desktop run of
// that scenario reported an ordering it could not settle for exactly that reason: the release
// arrived while Stop was still being pressed.
//
// So this is about the cadence and not about the bound, and it is the one wait in the harness
// where that is true. A failure here is a scenario that will keep coming out NOT RUN — which is
// the honest answer and not a useful one.
//
// The appended event lands 10ms in, so the old cadence returns at about 100ms and the tight one
// at about 15ms. The bound below sits between them with room for a loaded machine; it is a bound
// on the CADENCE being wrong, which is why it is not a sleep standing in for a verdict.
func TestTheHarnessNoticesAMouseEventInsideAGesture(t *testing.T) {
	log := filepath.Join(t.TempDir(), "fixture-events.jsonl")
	if err := os.WriteFile(log, []byte(`{"seq":1,"kind":"mouse","what":"left_down"}`+"\n"),
		0o600); err != nil {
		t.Fatal(err)
	}
	f := &fixtureRun{eventsFile: log}
	go func() {
		time.Sleep(10 * time.Millisecond)
		h, err := os.OpenFile(log, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return
		}
		defer h.Close()
		h.WriteString(`{"seq":2,"kind":"mouse","what":"left_up"}` + "\n")
	}()
	began := time.Now()
	got, arrived := f.waitForMouse(5*time.Second, 1, "left_up")
	took := time.Since(began)
	if !arrived {
		t.Fatalf("the release was never noticed, and it was written 10ms in")
	}
	if got.Seq != 2 {
		t.Errorf("it noticed #%d rather than the release at #2", got.Seq)
	}
	if took > 75*time.Millisecond {
		t.Errorf("it took %s to notice an event written 10ms in. A drag's travel is "+
			"about 190ms in total, so a scenario waiting this long has already "+
			"spent the gesture it meant to interrupt", took.Round(time.Millisecond))
	}
}
